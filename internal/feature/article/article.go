package article

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/uptrace/bun"
)

var (
	ErrNotFound            = errors.New("article not found")
	ErrSlugExists          = errors.New("slug already exists")
	ErrInvalidSlug         = errors.New("invalid slug")
	ErrInvalidInput        = errors.New("invalid input")
	ErrCannotUnpublishLast = errors.New("cannot unpublish")
)

var (
	frontmatterRE = regexp.MustCompile(`(?s)\A---\r?\n(.*?)\r?\n---\r?\n?(.*)`)
	slugRE        = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,98}[a-z0-9])?$`)
)

const (
	StatusDraft     = "draft"
	StatusPublished = "published"
)

// Article is the public/admin view of a post at a specific revision.
type Article struct {
	ID          int64
	Slug        string
	Status      string
	RevisionID  int64
	Title       string
	Emoji       string
	Type        string
	Topics      []string
	Published   bool
	PublishedAt time.Time
	BodyMD      string
	HTML        string
	Summary     string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type dbArticle struct {
	bun.BaseModel `bun:"table:articles,alias:a"`

	ID                  int64         `bun:",pk,autoincrement"`
	Slug                string        `bun:",notnull,unique"`
	Status              string        `bun:",notnull"`
	PublishedRevisionID sql.NullInt64 `bun:"published_revision_id"`
	PublishedAt         sql.NullTime  `bun:"published_at"`
	SourcePath          string        `bun:"source_path"`
	SourceHash          string        `bun:"source_hash"`
	CreatedAt           time.Time     `bun:",notnull"`
	UpdatedAt           time.Time     `bun:",notnull"`
}

type dbRevision struct {
	bun.BaseModel `bun:"table:article_revisions,alias:r"`

	ID        int64     `bun:",pk,autoincrement"`
	ArticleID int64     `bun:",notnull"`
	Title     string    `bun:",notnull"`
	Emoji     string    `bun:",notnull"`
	Type      string    `bun:",notnull"`
	BodyMD    string    `bun:"body_md,notnull"`
	Summary   string    `bun:",notnull"`
	CreatedAt time.Time `bun:",notnull"`
}

type dbTopic struct {
	bun.BaseModel `bun:"table:topics,alias:t"`

	ID   int64  `bun:",pk,autoincrement"`
	Name string `bun:",notnull,unique"`
	Slug string `bun:",notnull,unique"`
}

type dbArticleTopic struct {
	bun.BaseModel `bun:"table:article_topics,alias:at"`

	ArticleID int64 `bun:",pk"`
	TopicID   int64 `bun:",pk"`
}

// MarkdownExpander expands bare URLs / @[card](...) into embed HTML before markdown render.
type MarkdownExpander interface {
	ExpandMarkdown(ctx context.Context, body string) (string, error)
}

// SyncMarkdownExpander can resolve embeds synchronously (admin preview).
type SyncMarkdownExpander interface {
	MarkdownExpander
	ExpandMarkdownSync(ctx context.Context, body string) (string, error)
}

// Articles is the article store backed by Bun.
type Articles struct {
	db        *bun.DB
	embeds    MarkdownExpander
	mediaBase string
}

func New(db *bun.DB) *Articles {
	return &Articles{db: db}
}

// SetEmbeds attaches a link-card / embed expander used when rendering HTML.
func (a *Articles) SetEmbeds(embeds MarkdownExpander) {
	a.embeds = embeds
}

// SetMediaPublicBase rewrites Markdown `/images/...` paths to the Storage public base before HTML render.
func (a *Articles) SetMediaPublicBase(base string) {
	a.mediaBase = strings.TrimRight(strings.TrimSpace(base), "/")
}

// RenderHTML expands embeds (when configured) then converts Markdown to sanitized HTML.
// Link cards use cache/instant providers only; pending cards hydrate client-side.
func (a *Articles) RenderHTML(ctx context.Context, body string) (string, error) {
	body = RewriteImageURLs(body, a.mediaBase)
	if a != nil && a.embeds != nil {
		expanded, err := a.embeds.ExpandMarkdown(ctx, body)
		if err == nil {
			body = expanded
		}
	}
	return Render(body)
}

// RenderHTMLSync resolves every link card before returning HTML.
func (a *Articles) RenderHTMLSync(ctx context.Context, body string) (string, error) {
	body = RewriteImageURLs(body, a.mediaBase)
	if a != nil && a.embeds != nil {
		if sync, ok := a.embeds.(SyncMarkdownExpander); ok {
			expanded, err := sync.ExpandMarkdownSync(ctx, body)
			if err == nil {
				body = expanded
			}
		} else {
			expanded, err := a.embeds.ExpandMarkdown(ctx, body)
			if err == nil {
				body = expanded
			}
		}
	}
	return Render(body)
}

// SaveInput is the editable content for create/update.
type SaveInput struct {
	Slug        string
	Title       string
	Emoji       string
	Type        string
	Topics      []string
	BodyMD      string
	PublishedAt time.Time
}

// Path returns the canonical article path.
func (a Article) Path() string {
	return path.Join("/articles", a.Slug)
}

// TopicPath returns the topic filter path.
func TopicPath(topic string) string {
	return path.Join("/tags", url.PathEscape(topic))
}

// IsPublic reports whether the article should be shown at now.
func (a Article) IsPublic(now time.Time) bool {
	if a.Status != StatusPublished || !a.Published {
		return false
	}
	if a.PublishedAt.IsZero() {
		return true
	}
	return !a.PublishedAt.After(now.In(jst))
}

// ValidateSlug reports whether slug is a valid article filename stem.
func ValidateSlug(slug string) error {
	return validateSlug(slug)
}

func validateSlug(slug string) error {
	if !slugRE.MatchString(slug) {
		return ErrInvalidSlug
	}
	return nil
}

func summarize(body string) string {
	plain := stripMD(body)
	plain = strings.Join(strings.Fields(plain), " ")
	if plain == "" {
		return ""
	}
	runes := []rune(plain)
	if len(runes) > 120 {
		return string(runes[:120]) + "…"
	}
	return plain
}

func stripMD(body string) string {
	var b strings.Builder
	lines := strings.Split(body, "\n")
	inFence := false
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "```") || strings.HasPrefix(trim, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if strings.HasPrefix(trim, ":::") {
			continue
		}
		line = regexp.MustCompile(`!\[[^\]]*\]\([^)]+\)`).ReplaceAllString(line, "")
		line = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`).ReplaceAllString(line, "$1")
		line = strings.TrimLeftFunc(line, func(r rune) bool {
			return r == '#' || r == '>' || r == '-' || r == '*' || r == '|' || unicode.IsSpace(r)
		})
		line = strings.ReplaceAll(line, "`", "")
		line = strings.ReplaceAll(line, "**", "")
		line = strings.ReplaceAll(line, "__", "")
		line = strings.ReplaceAll(line, "*", "")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(strings.TrimSpace(line))
		if b.Len() > 200 {
			break
		}
	}
	return b.String()
}

func normalizeTopics(topics []string) ([]string, error) {
	out := make([]string, 0, len(topics))
	seen := make(map[string]struct{})
	for _, topic := range topics {
		topic = strings.TrimSpace(topic)
		if topic == "" {
			continue
		}
		if _, ok := seen[topic]; ok {
			continue
		}
		seen[topic] = struct{}{}
		out = append(out, topic)
	}
	if len(out) > 5 {
		return nil, fmt.Errorf("%w: topics at most 5", ErrInvalidInput)
	}
	return out, nil
}

func normalizeType(v string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		v = "tech"
	}
	switch v {
	case "tech", "idea":
		return v, nil
	default:
		return "", fmt.Errorf("%w: invalid type %q", ErrInvalidInput, v)
	}
}

func topicSlug(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	name = strings.ReplaceAll(name, " ", "-")
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
			continue
		}
		// Keep unicode letters for Japanese topics by percent-encoding-ish slug:
		// store raw name-based slug via URL path escape without slashes.
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteString(url.PathEscape(string(r)))
		}
	}
	s := b.String()
	if s == "" {
		return "topic"
	}
	return s
}
