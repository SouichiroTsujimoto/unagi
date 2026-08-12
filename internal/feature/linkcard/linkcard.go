package linkcard

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/uptrace/bun"
)

const (
	ProviderOGP     = "ogp"
	ProviderYouTube = "youtube"
	ProviderX       = "x"
	ProviderAmazon  = "amazon"
	ProviderGitHub  = "github"

	successTTL = 7 * 24 * time.Hour
	failureTTL = 1 * time.Hour
)

// Cards resolves bare URLs into embed/link-card HTML fragments.
type Cards struct {
	db   *bun.DB
	http *http.Client

	mu  sync.Mutex
	mem map[string]memEntry
}

type memEntry struct {
	card      Card
	expiresAt time.Time
}

// Card is a resolved preview or embed.
type Card struct {
	URL         string
	Provider    string
	Title       string
	Description string
	ImageURL    string
	SiteName    string
	HTML        string
	OK          bool
}

type dbCache struct {
	bun.BaseModel `bun:"table:link_card_cache,alias:lc"`

	URLHash     string    `bun:",pk"`
	URL         string    `bun:",notnull"`
	Provider    string    `bun:",notnull"`
	Title       string    `bun:",notnull"`
	Description string    `bun:",notnull"`
	ImageURL    string    `bun:"image_url,notnull"`
	SiteName    string    `bun:"site_name,notnull"`
	HTML        string    `bun:",notnull"`
	OK          bool      `bun:",notnull"`
	FetchedAt   time.Time `bun:",notnull"`
	ExpiresAt   time.Time `bun:",notnull"`
}

func New(db *bun.DB) *Cards {
	return &Cards{
		db:   db,
		http: newHTTPClient(),
		mem:  make(map[string]memEntry),
	}
}

// ExpandMarkdown replaces bare URL lines and @[card](url) with HTML embeds.
// Cache hits and instant providers (YouTube) render immediately; other URLs become
// daisyUI skeleton placeholders for client-side hydration.
func (c *Cards) ExpandMarkdown(ctx context.Context, body string) (string, error) {
	return c.expandMarkdown(ctx, body, false)
}

// ExpandMarkdownSync resolves every card before returning (admin preview).
func (c *Cards) ExpandMarkdownSync(ctx context.Context, body string) (string, error) {
	return c.expandMarkdown(ctx, body, true)
}

func (c *Cards) expandMarkdown(ctx context.Context, body string, sync bool) (string, error) {
	if c == nil || strings.TrimSpace(body) == "" {
		return body, nil
	}
	parts := splitMarkdownFences(body)
	var b strings.Builder
	for _, part := range parts {
		if part.code {
			b.WriteString(part.text)
			continue
		}
		b.WriteString(c.expandProse(ctx, part.text, sync))
	}
	return b.String(), nil
}

type mdPart struct {
	text string
	code bool
}

func splitMarkdownFences(body string) []mdPart {
	lines := strings.Split(body, "\n")
	var out []mdPart
	var buf strings.Builder
	inCode := false
	fence := ""

	flush := func(code bool) {
		out = append(out, mdPart{text: buf.String(), code: code})
		buf.Reset()
	}
	write := func(i int, line string) {
		buf.WriteString(line)
		if i < len(lines)-1 {
			buf.WriteByte('\n')
		}
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(strings.TrimRight(line, "\r"))
		if !inCode {
			if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
				if buf.Len() > 0 {
					flush(false)
				}
				inCode = true
				fence = trimmed[:3]
				write(i, line)
				continue
			}
			write(i, line)
			continue
		}
		write(i, line)
		if fence != "" && strings.HasPrefix(trimmed, fence) {
			flush(true)
			inCode = false
			fence = ""
		}
	}
	if buf.Len() > 0 {
		flush(inCode)
	}
	if len(out) == 0 {
		return []mdPart{{text: body, code: false}}
	}
	return out
}

var (
	cardDirectiveRE = mustCompile(`(?m)^@[card]\((https?://[^)\s]+)\)\s*$`)
	bareURLLineRE   = mustCompile(`(?m)^(https?://[^\s<>]+|<https?://[^>\s]+>)\s*$`)
)

func (c *Cards) expandProse(ctx context.Context, prose string, sync bool) string {
	prose = cardDirectiveRE.ReplaceAllStringFunc(prose, func(line string) string {
		m := cardDirectiveRE.FindStringSubmatch(line)
		if m == nil {
			return line
		}
		return c.replaceURL(ctx, strings.TrimSpace(m[1]), sync)
	})
	prose = bareURLLineRE.ReplaceAllStringFunc(prose, func(line string) string {
		raw := strings.TrimSpace(line)
		raw = strings.TrimPrefix(raw, "<")
		raw = strings.TrimSuffix(raw, ">")
		return c.replaceURL(ctx, raw, sync)
	})
	return prose
}

func (c *Cards) replaceURL(ctx context.Context, raw string, sync bool) string {
	if !sync {
		if card, ok := c.Lookup(ctx, raw); ok {
			if card.OK && strings.TrimSpace(card.HTML) != "" {
				return card.HTML + "\n"
			}
			esc := htmlEscape(raw)
			return fmt.Sprintf("<p><a href=\"%s\" rel=\"noopener noreferrer\">%s</a></p>\n", esc, esc)
		}
		if u, err := url.Parse(raw); err == nil && detectProvider(u) == ProviderYouTube {
			card, err := resolveYouTube(raw, u)
			if err == nil && card.OK {
				hash := hashURL(normalizeURL(u))
				_ = c.save(ctx, hash, card, successTTL)
				c.memPut(hash, card, successTTL)
				return card.HTML + "\n"
			}
		}
		return renderPendingCard(raw) + "\n"
	}

	card, err := c.Resolve(ctx, raw)
	if err != nil || !card.OK || strings.TrimSpace(card.HTML) == "" {
		esc := htmlEscape(raw)
		return fmt.Sprintf("<p><a href=\"%s\" rel=\"noopener noreferrer\">%s</a></p>\n", esc, esc)
	}
	return card.HTML + "\n"
}

// Lookup returns a cached card without hitting the network.
func (c *Cards) Lookup(ctx context.Context, raw string) (Card, bool) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil {
		return Card{}, false
	}
	if err := validateFetchURL(u); err != nil {
		return Card{}, false
	}
	normalized := normalizeURL(u)
	hash := hashURL(normalized)
	if card, ok := c.memGet(hash); ok {
		return card, true
	}
	if card, ok, err := c.dbGet(ctx, hash); err == nil && ok {
		c.memPut(hash, card, successTTL)
		return card, true
	}
	return Card{}, false
}

// Resolve fetches or builds a card for url.
func (c *Cards) Resolve(ctx context.Context, raw string) (Card, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil {
		return Card{}, err
	}
	if err := validateFetchURL(u); err != nil {
		return Card{}, err
	}
	normalized := normalizeURL(u)
	hash := hashURL(normalized)

	if card, ok := c.memGet(hash); ok {
		return card, nil
	}
	if card, ok, err := c.dbGet(ctx, hash); err == nil && ok {
		remain := successTTL
		c.memPut(hash, card, remain)
		return card, nil
	}

	card, err := c.resolveFresh(ctx, normalized)
	if err != nil {
		fail := Card{URL: normalized, Provider: ProviderOGP, OK: false}
		_ = c.save(ctx, hash, fail, failureTTL)
		c.memPut(hash, fail, failureTTL)
		return fail, err
	}
	ttl := successTTL
	if !card.OK {
		ttl = failureTTL
	}
	_ = c.save(ctx, hash, card, ttl)
	c.memPut(hash, card, ttl)
	return card, nil
}

func (c *Cards) resolveFresh(ctx context.Context, raw string) (Card, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return Card{}, err
	}
	switch detectProvider(u) {
	case ProviderYouTube:
		return resolveYouTube(raw, u)
	case ProviderX:
		return c.resolveX(ctx, raw, u)
	case ProviderAmazon:
		return c.resolveAmazon(ctx, raw, u)
	case ProviderGitHub:
		return c.resolveGitHub(ctx, raw, u)
	default:
		return c.resolveOGP(ctx, raw)
	}
}

func detectProvider(u *url.URL) string {
	host := strings.ToLower(u.Hostname())
	host = strings.TrimPrefix(host, "www.")
	switch {
	case host == "youtu.be",
		host == "youtube.com",
		host == "m.youtube.com",
		host == "music.youtube.com",
		host == "youtube-nocookie.com":
		return ProviderYouTube
	case host == "twitter.com",
		host == "mobile.twitter.com",
		host == "x.com",
		host == "mobile.x.com":
		return ProviderX
	case isAmazonHost(host):
		return ProviderAmazon
	case host == "github.com",
		host == "gist.github.com",
		host == "raw.githubusercontent.com":
		return ProviderGitHub
	default:
		return ProviderOGP
	}
}

func isAmazonHost(host string) bool {
	if host == "amzn.to" || host == "amzn.asia" || host == "a.co" {
		return true
	}
	if strings.HasPrefix(host, "amazon.") {
		return true
	}
	if strings.Contains(host, ".amazon.") {
		return true
	}
	return false
}

func normalizeURL(u *url.URL) string {
	u2 := *u
	if detectProvider(u) != ProviderGitHub {
		u2.Fragment = ""
	}
	return u2.String()
}

func hashURL(raw string) string {
	// v2: invalidate early Amazon/GitHub cache entries that used weak fallbacks.
	sum := sha256.Sum256([]byte("v2|" + raw))
	return hex.EncodeToString(sum[:])
}

func (c *Cards) memGet(hash string) (Card, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ent, ok := c.mem[hash]
	if !ok || time.Now().After(ent.expiresAt) {
		if ok {
			delete(c.mem, hash)
		}
		return Card{}, false
	}
	return ent.card, true
}

func (c *Cards) memPut(hash string, card Card, ttl time.Duration) {
	if ttl <= 0 {
		ttl = failureTTL
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.mem[hash] = memEntry{card: card, expiresAt: time.Now().Add(ttl)}
}

func (c *Cards) dbGet(ctx context.Context, hash string) (Card, bool, error) {
	if c.db == nil {
		return Card{}, false, nil
	}
	var row dbCache
	err := c.db.NewSelect().Model(&row).Where("url_hash = ?", hash).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return Card{}, false, nil
	}
	if err != nil {
		return Card{}, false, err
	}
	if time.Now().After(row.ExpiresAt) {
		return Card{}, false, nil
	}
	return Card{
		URL:         row.URL,
		Provider:    row.Provider,
		Title:       row.Title,
		Description: row.Description,
		ImageURL:    row.ImageURL,
		SiteName:    row.SiteName,
		HTML:        row.HTML,
		OK:          row.OK,
	}, true, nil
}

func (c *Cards) save(ctx context.Context, hash string, card Card, ttl time.Duration) error {
	if c.db == nil {
		return nil
	}
	now := time.Now().UTC()
	row := &dbCache{
		URLHash:     hash,
		URL:         card.URL,
		Provider:    card.Provider,
		Title:       card.Title,
		Description: card.Description,
		ImageURL:    card.ImageURL,
		SiteName:    card.SiteName,
		HTML:        card.HTML,
		OK:          card.OK,
		FetchedAt:   now,
		ExpiresAt:   now.Add(ttl),
	}
	_, err := c.db.NewInsert().
		Model(row).
		On("CONFLICT (url_hash) DO UPDATE").
		Set("url = EXCLUDED.url").
		Set("provider = EXCLUDED.provider").
		Set("title = EXCLUDED.title").
		Set("description = EXCLUDED.description").
		Set("image_url = EXCLUDED.image_url").
		Set("site_name = EXCLUDED.site_name").
		Set("html = EXCLUDED.html").
		Set("ok = EXCLUDED.ok").
		Set("fetched_at = EXCLUDED.fetched_at").
		Set("expires_at = EXCLUDED.expires_at").
		Exec(ctx)
	return err
}
