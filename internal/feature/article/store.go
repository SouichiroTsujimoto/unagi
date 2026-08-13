package article

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/uptrace/bun"
)

const topicNamesExpr = `COALESCE((
	SELECT array_agg(t.name ORDER BY t.name)
	FROM article_topics AS at
	JOIN topics AS t ON t.id = at.topic_id
	WHERE at.article_id = a.id
), ARRAY[]::text[]) AS topic_names`

func (a *Articles) selectPublic(model any, now time.Time) *bun.SelectQuery {
	return a.db.NewSelect().
		Model(model).
		ColumnExpr("?TableColumns").
		ColumnExpr(topicNamesExpr).
		Relation("Revision").
		Where("a.status = ?", StatusPublished).
		Where("a.published_revision_id IS NOT NULL").
		Where("(a.published_at IS NULL OR a.published_at <= ?)", now)
}

func publicNow(now time.Time) time.Time {
	if now.IsZero() {
		now = time.Now()
	}
	return now.In(jst)
}

// List returns publicly visible articles, newest first.
func (a *Articles) List(ctx context.Context, now time.Time) ([]Article, error) {
	now = publicNow(now)

	var rows []dbArticle
	if err := a.selectPublic(&rows, now).
		OrderExpr("a.published_at DESC NULLS LAST, a.id DESC").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("list articles: %w", err)
	}
	return a.publicArticles(ctx, rows, false)
}

// Get returns a publicly visible article by slug.
func (a *Articles) Get(ctx context.Context, slug string, now time.Time) (Article, error) {
	now = publicNow(now)

	var row dbArticle
	err := a.selectPublic(&row, now).
		Where("a.slug = ?", slug).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return Article{}, ErrNotFound
	}
	if err != nil {
		return Article{}, fmt.Errorf("get article: %w", err)
	}
	return a.publicArticle(ctx, row, true)
}

// ListByTopic returns publicly visible articles for a topic name.
func (a *Articles) ListByTopic(ctx context.Context, topic string, now time.Time) ([]Article, error) {
	now = publicNow(now)

	var rows []dbArticle
	err := a.selectPublic(&rows, now).
		Where("EXISTS (SELECT 1 FROM article_topics AS at JOIN topics AS t ON t.id = at.topic_id WHERE at.article_id = a.id AND t.name = ?)", topic).
		OrderExpr("a.published_at DESC NULLS LAST, a.id DESC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list by topic: %w", err)
	}
	return a.publicArticles(ctx, rows, false)
}

// Topics returns topic names that have at least one currently public article.
func (a *Articles) Topics(ctx context.Context, now time.Time) ([]string, error) {
	if now.IsZero() {
		now = time.Now()
	}
	now = now.In(jst)

	var names []string
	err := a.db.NewSelect().
		ColumnExpr("DISTINCT t.name").
		TableExpr("topics AS t").
		Join("JOIN article_topics at ON at.topic_id = t.id").
		Join("JOIN articles a ON a.id = at.article_id").
		Where("a.status = ?", StatusPublished).
		Where("a.published_revision_id IS NOT NULL").
		Where("(a.published_at IS NULL OR a.published_at <= ?)", now).
		OrderExpr("t.name ASC").
		Scan(ctx, &names)
	if err != nil {
		return nil, fmt.Errorf("list topics: %w", err)
	}
	return names, nil
}

// TopicExists reports whether a topic name is known (even if currently empty of public posts).
func (a *Articles) TopicExists(ctx context.Context, topic string) (bool, error) {
	return a.db.NewSelect().
		Model((*dbTopic)(nil)).
		Where("name = ?", topic).
		Exists(ctx)
}

// ListAll returns all articles for admin (latest revision).
func (a *Articles) ListAll(ctx context.Context) ([]Article, error) {
	var rows []dbArticle
	if err := a.db.NewSelect().
		Model(&rows).
		OrderExpr("updated_at DESC, id DESC").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("list all articles: %w", err)
	}
	out := make([]Article, 0, len(rows))
	for _, row := range rows {
		item, err := a.hydrateAdmin(ctx, row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// GetByID returns an article for admin by id (latest revision).
func (a *Articles) GetByID(ctx context.Context, id int64) (Article, error) {
	var row dbArticle
	err := a.db.NewSelect().Model(&row).Where("id = ?", id).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return Article{}, ErrNotFound
	}
	if err != nil {
		return Article{}, fmt.Errorf("get article by id: %w", err)
	}
	return a.hydrateAdmin(ctx, row)
}

// GetBySlugAdmin returns an article for admin by slug (latest revision).
func (a *Articles) GetBySlugAdmin(ctx context.Context, slug string) (Article, error) {
	var row dbArticle
	err := a.db.NewSelect().Model(&row).Where("slug = ?", slug).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return Article{}, ErrNotFound
	}
	if err != nil {
		return Article{}, fmt.Errorf("get article by slug: %w", err)
	}
	return a.hydrateAdmin(ctx, row)
}

func normalizeSaveInput(in SaveInput) (SaveInput, error) {
	in.Slug = strings.TrimSpace(in.Slug)
	in.Title = strings.TrimSpace(in.Title)
	in.Emoji = strings.TrimSpace(in.Emoji)
	in.BodyMD = strings.TrimSpace(in.BodyMD) + "\n"
	if in.Title == "" {
		return SaveInput{}, fmt.Errorf("%w: title is required", ErrInvalidInput)
	}
	if err := validateSlug(in.Slug); err != nil {
		return SaveInput{}, err
	}
	typ, err := normalizeType(in.Type)
	if err != nil {
		return SaveInput{}, err
	}
	in.Type = typ
	topics, err := normalizeTopics(in.Topics)
	if err != nil {
		return SaveInput{}, err
	}
	in.Topics = topics
	return in, nil
}

func replaceTopics(ctx context.Context, tx bun.Tx, articleID int64, topics []string) error {
	if _, err := tx.NewDelete().Model((*dbArticleTopic)(nil)).Where("article_id = ?", articleID).Exec(ctx); err != nil {
		return err
	}
	for _, name := range topics {
		var topic dbTopic
		err := tx.NewSelect().Model(&topic).Where("name = ?", name).Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			topic = dbTopic{Name: name, Slug: topicSlug(name)}
			if _, err := tx.NewInsert().Model(&topic).Exec(ctx); err != nil {
				return fmt.Errorf("insert topic: %w", err)
			}
		} else if err != nil {
			return err
		}
		link := &dbArticleTopic{ArticleID: articleID, TopicID: topic.ID}
		if _, err := tx.NewInsert().Model(link).Exec(ctx); err != nil {
			return fmt.Errorf("link topic: %w", err)
		}
	}
	return nil
}

func (a *Articles) publicArticles(ctx context.Context, rows []dbArticle, withHTML bool) ([]Article, error) {
	out := make([]Article, 0, len(rows))
	for _, row := range rows {
		item, err := a.publicArticle(ctx, row, withHTML)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func (a *Articles) publicArticle(ctx context.Context, row dbArticle, withHTML bool) (Article, error) {
	if row.Revision == nil {
		return Article{}, ErrNotFound
	}
	rev := row.Revision
	topics := row.TopicNames
	if topics == nil {
		topics = []string{}
	}
	item := Article{
		ID:         row.ID,
		Slug:       row.Slug,
		Status:     row.Status,
		RevisionID: rev.ID,
		OGVersion:  row.OGVersion,
		OGTemplate: row.OGTemplate,
		Title:      rev.Title,
		Emoji:      rev.Emoji,
		Type:       rev.Type,
		Topics:     topics,
		Published:  true,
		BodyMD:     rev.BodyMD,
		Summary:    rev.Summary,
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
	}
	if row.PublishedAt.Valid {
		item.PublishedAt = row.PublishedAt.Time.In(jst)
	}
	if withHTML {
		html, err := a.RenderHTML(ctx, rev.BodyMD)
		if err != nil {
			return Article{}, err
		}
		item.HTML = html
	}
	return item, nil
}

func (a *Articles) hydrateAdmin(ctx context.Context, row dbArticle) (Article, error) {
	var rev dbRevision
	err := a.db.NewSelect().
		Model(&rev).
		Where("article_id = ?", row.ID).
		OrderExpr("id DESC").
		Limit(1).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return Article{}, ErrNotFound
	}
	if err != nil {
		return Article{}, err
	}
	topics, err := a.topicsFor(ctx, row.ID)
	if err != nil {
		return Article{}, err
	}
	item := Article{
		ID:         row.ID,
		Slug:       row.Slug,
		Status:     row.Status,
		RevisionID: rev.ID,
		OGVersion:  row.OGVersion,
		OGTemplate: row.OGTemplate,
		Title:      rev.Title,
		Emoji:      rev.Emoji,
		Type:       rev.Type,
		Topics:     topics,
		Published:  row.Status == StatusPublished && row.PublishedRevisionID.Valid,
		BodyMD:     rev.BodyMD,
		Summary:    rev.Summary,
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
	}
	if row.PublishedAt.Valid {
		item.PublishedAt = row.PublishedAt.Time.In(jst)
	}
	return item, nil
}

func (a *Articles) topicsFor(ctx context.Context, articleID int64) ([]string, error) {
	var names []string
	err := a.db.NewSelect().
		ColumnExpr("t.name").
		TableExpr("topics AS t").
		Join("JOIN article_topics at ON at.topic_id = t.id").
		Where("at.article_id = ?", articleID).
		OrderExpr("t.name ASC").
		Scan(ctx, &names)
	if err != nil {
		return nil, fmt.Errorf("load topics: %w", err)
	}
	return names, nil
}

// FormatMarkdown builds Zenn-compatible Markdown from an article.
func FormatMarkdown(a Article) string {
	topics := make([]string, 0, len(a.Topics))
	for _, t := range a.Topics {
		topics = append(topics, fmt.Sprintf("%q", t))
	}
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "title: %q\n", a.Title)
	fmt.Fprintf(&b, "emoji: %q\n", a.Emoji)
	fmt.Fprintf(&b, "type: %q\n", a.Type)
	fmt.Fprintf(&b, "topics: [%s]\n", strings.Join(topics, ", "))
	b.WriteString("---\n\n")
	b.WriteString(strings.TrimSpace(a.BodyMD))
	b.WriteByte('\n')
	return b.String()
}
