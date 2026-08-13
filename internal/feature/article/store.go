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

// List returns publicly visible articles, newest first.
func (a *Articles) List(ctx context.Context, now time.Time) ([]Article, error) {
	if now.IsZero() {
		now = time.Now()
	}
	now = now.In(jst)

	var rows []dbArticle
	if err := a.db.NewSelect().
		Model(&rows).
		Where("status = ?", StatusPublished).
		Where("published_revision_id IS NOT NULL").
		Where("(published_at IS NULL OR published_at <= ?)", now).
		OrderExpr("published_at DESC NULLS LAST, id DESC").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("list articles: %w", err)
	}
	return a.hydrateMany(ctx, rows, false)
}

// Get returns a publicly visible article by slug.
func (a *Articles) Get(ctx context.Context, slug string, now time.Time) (Article, error) {
	if now.IsZero() {
		now = time.Now()
	}
	now = now.In(jst)

	var row dbArticle
	err := a.db.NewSelect().
		Model(&row).
		Where("slug = ?", slug).
		Where("status = ?", StatusPublished).
		Where("published_revision_id IS NOT NULL").
		Where("(published_at IS NULL OR published_at <= ?)", now).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return Article{}, ErrNotFound
	}
	if err != nil {
		return Article{}, fmt.Errorf("get article: %w", err)
	}
	return a.hydrateOne(ctx, row, true)
}

// ListByTopic returns publicly visible articles for a topic name.
func (a *Articles) ListByTopic(ctx context.Context, topic string, now time.Time) ([]Article, error) {
	if now.IsZero() {
		now = time.Now()
	}
	now = now.In(jst)

	var rows []dbArticle
	err := a.db.NewSelect().
		Model(&rows).
		Join("JOIN article_topics at ON at.article_id = a.id").
		Join("JOIN topics t ON t.id = at.topic_id").
		Where("t.name = ?", topic).
		Where("a.status = ?", StatusPublished).
		Where("a.published_revision_id IS NOT NULL").
		Where("(a.published_at IS NULL OR a.published_at <= ?)", now).
		OrderExpr("a.published_at DESC NULLS LAST, a.id DESC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list by topic: %w", err)
	}
	return a.hydrateMany(ctx, rows, false)
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

// Create inserts a draft article with an initial revision.
func (a *Articles) Create(ctx context.Context, in SaveInput) (Article, error) {
	in, err := normalizeSaveInput(in)
	if err != nil {
		return Article{}, err
	}
	exists, err := a.db.NewSelect().Model((*dbArticle)(nil)).Where("slug = ?", in.Slug).Exists(ctx)
	if err != nil {
		return Article{}, err
	}
	if exists {
		return Article{}, ErrSlugExists
	}

	now := time.Now().UTC()
	var created Article
	err = a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		row := &dbArticle{
			Slug:      in.Slug,
			Status:    StatusDraft,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if _, err := tx.NewInsert().Model(row).Exec(ctx); err != nil {
			return fmt.Errorf("insert article: %w", err)
		}
		rev := &dbRevision{
			ArticleID: row.ID,
			Title:     in.Title,
			Emoji:     in.Emoji,
			Type:      in.Type,
			BodyMD:    in.BodyMD,
			Summary:   summarize(in.BodyMD),
			CreatedAt: now,
		}
		if _, err := tx.NewInsert().Model(rev).Exec(ctx); err != nil {
			return fmt.Errorf("insert revision: %w", err)
		}
		if err := replaceTopics(ctx, tx, row.ID, in.Topics); err != nil {
			return err
		}
		if !in.PublishedAt.IsZero() {
			row.PublishedAt = sql.NullTime{Time: in.PublishedAt.UTC(), Valid: true}
			if _, err := tx.NewUpdate().Model(row).Column("published_at").WherePK().Exec(ctx); err != nil {
				return err
			}
		}
		created = Article{
			ID:         row.ID,
			Slug:       row.Slug,
			Status:     row.Status,
			RevisionID: rev.ID,
			Title:      rev.Title,
			Emoji:      rev.Emoji,
			Type:       rev.Type,
			Topics:     append([]string(nil), in.Topics...),
			BodyMD:     rev.BodyMD,
			Summary:    rev.Summary,
			CreatedAt:  row.CreatedAt,
			UpdatedAt:  row.UpdatedAt,
		}
		if row.PublishedAt.Valid {
			created.PublishedAt = row.PublishedAt.Time.In(jst)
		}
		return nil
	})
	if err != nil {
		return Article{}, err
	}
	return created, nil
}

// SaveRevision appends a new revision and updates article metadata.
func (a *Articles) SaveRevision(ctx context.Context, id int64, in SaveInput) (Article, error) {
	in, err := normalizeSaveInput(in)
	if err != nil {
		return Article{}, err
	}
	now := time.Now().UTC()
	var saved Article
	err = a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var row dbArticle
		if err := tx.NewSelect().Model(&row).Where("id = ?", id).Scan(ctx); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if in.Slug != row.Slug {
			exists, err := tx.NewSelect().Model((*dbArticle)(nil)).Where("slug = ? AND id <> ?", in.Slug, id).Exists(ctx)
			if err != nil {
				return err
			}
			if exists {
				return ErrSlugExists
			}
			row.Slug = in.Slug
		}
		rev := &dbRevision{
			ArticleID: row.ID,
			Title:     in.Title,
			Emoji:     in.Emoji,
			Type:      in.Type,
			BodyMD:    in.BodyMD,
			Summary:   summarize(in.BodyMD),
			CreatedAt: now,
		}
		if _, err := tx.NewInsert().Model(rev).Exec(ctx); err != nil {
			return fmt.Errorf("insert revision: %w", err)
		}
		row.UpdatedAt = now
		if in.PublishedAt.IsZero() {
			row.PublishedAt = sql.NullTime{}
		} else {
			row.PublishedAt = sql.NullTime{Time: in.PublishedAt.UTC(), Valid: true}
		}
		if _, err := tx.NewUpdate().
			Model(&row).
			Column("slug", "updated_at", "published_at").
			WherePK().
			Exec(ctx); err != nil {
			return err
		}
		if err := replaceTopics(ctx, tx, row.ID, in.Topics); err != nil {
			return err
		}
		saved = Article{
			ID:         row.ID,
			Slug:       row.Slug,
			Status:     row.Status,
			RevisionID: rev.ID,
			Title:      rev.Title,
			Emoji:      rev.Emoji,
			Type:       rev.Type,
			Topics:     append([]string(nil), in.Topics...),
			Published:  row.Status == StatusPublished && row.PublishedRevisionID.Valid,
			BodyMD:     rev.BodyMD,
			Summary:    rev.Summary,
			CreatedAt:  row.CreatedAt,
			UpdatedAt:  row.UpdatedAt,
		}
		if row.PublishedAt.Valid {
			saved.PublishedAt = row.PublishedAt.Time.In(jst)
		}
		return nil
	})
	if err != nil {
		return Article{}, err
	}
	return saved, nil
}

// Publish marks a revision as the published one.
func (a *Articles) Publish(ctx context.Context, articleID, revisionID int64, publishedAt time.Time) (Article, error) {
	now := time.Now().UTC()
	if publishedAt.IsZero() {
		publishedAt = now
	}
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var rev dbRevision
		if err := tx.NewSelect().Model(&rev).Where("id = ? AND article_id = ?", revisionID, articleID).Scan(ctx); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		var row dbArticle
		if err := tx.NewSelect().Model(&row).Where("id = ?", articleID).Scan(ctx); err != nil {
			return err
		}
		row.Status = StatusPublished
		row.PublishedRevisionID = sql.NullInt64{Int64: revisionID, Valid: true}
		if !row.PublishedAt.Valid {
			if publishedAt.IsZero() {
				publishedAt = now
			}
			row.PublishedAt = sql.NullTime{Time: publishedAt.UTC(), Valid: true}
		}
		row.UpdatedAt = now
		_, err := tx.NewUpdate().
			Model(&row).
			Column("status", "published_revision_id", "published_at", "updated_at").
			WherePK().
			Exec(ctx)
		return err
	})
	if err != nil {
		return Article{}, err
	}
	return a.GetByID(ctx, articleID)
}

// Unpublish moves an article back to draft without deleting revisions.
func (a *Articles) Unpublish(ctx context.Context, articleID int64) (Article, error) {
	now := time.Now().UTC()
	res, err := a.db.NewUpdate().
		Model((*dbArticle)(nil)).
		Set("status = ?", StatusDraft).
		Set("published_revision_id = NULL").
		Set("updated_at = ?", now).
		Where("id = ?", articleID).
		Exec(ctx)
	if err != nil {
		return Article{}, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return Article{}, ErrNotFound
	}
	return a.GetByID(ctx, articleID)
}

// ListRevisions returns revisions newest first.
func (a *Articles) ListRevisions(ctx context.Context, articleID int64) ([]dbRevision, error) {
	var revs []dbRevision
	err := a.db.NewSelect().
		Model(&revs).
		Where("article_id = ?", articleID).
		OrderExpr("id DESC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list revisions: %w", err)
	}
	return revs, nil
}

// ExportMarkdown returns Zenn-compatible Markdown for an article's latest revision.
func (a *Articles) ExportMarkdown(ctx context.Context, id int64) (string, error) {
	item, err := a.GetByID(ctx, id)
	if err != nil {
		return "", err
	}
	return FormatMarkdown(item), nil
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

func (a *Articles) hydrateMany(ctx context.Context, rows []dbArticle, withHTML bool) ([]Article, error) {
	out := make([]Article, 0, len(rows))
	for _, row := range rows {
		item, err := a.hydrateOne(ctx, row, withHTML)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func (a *Articles) hydrateOne(ctx context.Context, row dbArticle, withHTML bool) (Article, error) {
	if !row.PublishedRevisionID.Valid {
		return Article{}, ErrNotFound
	}
	var rev dbRevision
	if err := a.db.NewSelect().Model(&rev).Where("id = ?", row.PublishedRevisionID.Int64).Scan(ctx); err != nil {
		return Article{}, fmt.Errorf("load revision: %w", err)
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
