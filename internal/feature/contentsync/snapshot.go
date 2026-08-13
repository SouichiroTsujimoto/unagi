package contentsync

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/SouichiroTsujimoto/unagi/internal/feature/article"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/media"
)

var mdImageRE = regexp.MustCompile(`!\[[^\]]*\]\(([^)]+)\)`)

const maxArticles = 200

type Snapshot struct {
	Repository string      `json:"repository"`
	CommitSHA  string      `json:"commit_sha"`
	RunID      string      `json:"run_id"`
	Articles   []ArticleIn `json:"articles"`
	Images     []ImageIn   `json:"images"`
}

type ArticleIn struct {
	Path     string `json:"path"`
	Markdown string `json:"markdown"`
}

type ImageIn struct {
	Path        string `json:"path"`
	SHA256      string `json:"sha256"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
}

type preparedImage struct {
	ImageIn
	ObjectKey string
}

type preparedArticle struct {
	article.SyncContent
}

func prepareSnapshot(snap Snapshot, allowedRepo string) ([]preparedArticle, []preparedImage, error) {
	if err := validateEnvelope(snap, allowedRepo); err != nil {
		return nil, nil, err
	}
	images, byPath, err := prepareImages(snap.Images)
	if err != nil {
		return nil, nil, err
	}
	articles, err := prepareArticles(snap.Articles, byPath)
	if err != nil {
		return nil, nil, err
	}
	return articles, images, nil
}

func validateEnvelope(snap Snapshot, allowedRepo string) error {
	if strings.TrimSpace(snap.RunID) == "" {
		return fmt.Errorf("%w: run_id is required", ErrInvalidSnapshot)
	}
	if !validCommitSHA(snap.CommitSHA) {
		return fmt.Errorf("%w: commit_sha is required", ErrInvalidSnapshot)
	}
	repo := strings.TrimSpace(snap.Repository)
	if repo == "" {
		return fmt.Errorf("%w: repository is required", ErrInvalidSnapshot)
	}
	if !strings.EqualFold(repo, allowedRepo) {
		return fmt.Errorf("%w: %s", ErrForbiddenRepo, repo)
	}
	if len(snap.Articles) > maxArticles {
		return fmt.Errorf("%w: too many articles", ErrInvalidSnapshot)
	}
	return nil
}

func validCommitSHA(v string) bool {
	v = strings.TrimSpace(strings.ToLower(v))
	if len(v) < 7 || len(v) > 40 {
		return false
	}
	for _, r := range v {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

func prepareImages(images []ImageIn) ([]preparedImage, map[string]preparedImage, error) {
	out := make([]preparedImage, 0, len(images))
	byPath := make(map[string]preparedImage, len(images))
	seenKey := make(map[string]string, len(images))
	for _, img := range images {
		p := strings.TrimSpace(img.Path)
		if !validImagePath(p) {
			return nil, nil, fmt.Errorf("%w: invalid image path %q", ErrInvalidSnapshot, img.Path)
		}
		if img.Size <= 0 || img.Size > media.MaxUploadBytes {
			return nil, nil, fmt.Errorf("%w: image %s exceeds 5 MiB or is empty", ErrInvalidSnapshot, p)
		}
		key, err := media.ContentAddressedKey(img.SHA256, img.ContentType)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: image %s: %v", ErrInvalidSnapshot, p, err)
		}
		if prev, ok := seenKey[key]; ok && prev != p {
			return nil, nil, fmt.Errorf("%w: duplicate object key for %s and %s", ErrInvalidSnapshot, prev, p)
		}
		seenKey[key] = p
		prepared := preparedImage{ImageIn: img, ObjectKey: key}
		prepared.Path = p
		prepared.SHA256 = strings.ToLower(strings.TrimSpace(img.SHA256))
		if _, ok := byPath[p]; ok {
			return nil, nil, fmt.Errorf("%w: duplicate image path %s", ErrInvalidSnapshot, p)
		}
		byPath[p] = prepared
		out = append(out, prepared)
	}
	return out, byPath, nil
}

func validImagePath(p string) bool {
	if p == "" || strings.Contains(p, "..") || strings.HasPrefix(p, "/") {
		return false
	}
	if path.Dir(p) != "images" {
		return false
	}
	ext := strings.ToLower(path.Ext(p))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif":
		return path.Base(p) == path.Base(p) && path.Base(p) != "." && path.Base(p) != "/"
	default:
		return false
	}
}

func prepareArticles(articles []ArticleIn, images map[string]preparedImage) ([]preparedArticle, error) {
	out := make([]preparedArticle, 0, len(articles))
	seenSlug := make(map[string]string, len(articles))
	for _, in := range articles {
		p := strings.TrimSpace(in.Path)
		slug, err := slugFromPath(p)
		if err != nil {
			return nil, err
		}
		if prev, ok := seenSlug[slug]; ok {
			return nil, fmt.Errorf("%w: duplicate slug %s (%s and %s)", ErrInvalidSnapshot, slug, prev, p)
		}
		seenSlug[slug] = p
		if strings.ContainsRune(in.Markdown, 0) || !utf8.ValidString(in.Markdown) {
			return nil, fmt.Errorf("%w: %s is not valid UTF-8", ErrInvalidSnapshot, p)
		}
		parsed, err := article.Parse(slug, []byte(in.Markdown))
		if err != nil {
			return nil, fmt.Errorf("%w: parse %s: %v", ErrInvalidSnapshot, p, err)
		}
		body, err := rewriteAndCheckImages(parsed.BodyMD, images)
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %v", ErrInvalidSnapshot, p, err)
		}
		sum := sha256.Sum256([]byte(in.Markdown))
		out = append(out, preparedArticle{SyncContent: article.SyncContent{
			Slug:       slug,
			SourcePath: p,
			SourceHash: hex.EncodeToString(sum[:]),
			Title:      parsed.Title,
			Emoji:      parsed.Emoji,
			Type:       parsed.Type,
			Topics:     parsed.Topics,
			BodyMD:     body,
		}})
	}
	return out, nil
}

func slugFromPath(p string) (string, error) {
	if p == "" || strings.Contains(p, "..") || strings.HasPrefix(p, "/") {
		return "", fmt.Errorf("%w: invalid article path %q", ErrInvalidSnapshot, p)
	}
	if path.Dir(p) != "articles" || !strings.HasSuffix(p, ".md") {
		return "", fmt.Errorf("%w: article path must be articles/<slug>.md", ErrInvalidSnapshot)
	}
	slug := strings.TrimSuffix(path.Base(p), ".md")
	if err := article.ValidateSlug(slug); err != nil {
		return "", fmt.Errorf("%w: %s: %v", ErrInvalidSnapshot, p, err)
	}
	return slug, nil
}

func rewriteAndCheckImages(body string, images map[string]preparedImage) (string, error) {
	var firstErr error
	rewritten := mdImageRE.ReplaceAllStringFunc(body, func(raw string) string {
		if firstErr != nil {
			return raw
		}
		m := mdImageRE.FindStringSubmatch(raw)
		if len(m) != 2 {
			return raw
		}
		src := strings.TrimSpace(m[1])
		local, ok := localImagePath(src)
		if !ok {
			return raw
		}
		img, found := images[local]
		if !found {
			firstErr = fmt.Errorf("missing image %s", local)
			return raw
		}
		return strings.Replace(raw, m[1], "/images/"+img.ObjectKey, 1)
	})
	if firstErr != nil {
		return "", firstErr
	}
	return rewritten, nil
}

func localImagePath(src string) (string, bool) {
	src = strings.TrimSpace(src)
	if src == "" {
		return "", false
	}
	if u, err := url.Parse(src); err == nil && u.Scheme != "" {
		return "", false
	}
	src = strings.TrimPrefix(src, "./")
	src = path.Clean(src)
	src = strings.TrimPrefix(src, "/")
	if strings.HasPrefix(src, "images/") && !strings.Contains(src, "..") {
		return src, true
	}
	return "", false
}
