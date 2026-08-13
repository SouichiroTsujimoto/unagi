package article

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

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
			Slug:       in.Slug,
			Status:     StatusDraft,
			OGVersion:  1,
			OGTemplate: DefaultOGTemplate,
			CreatedAt:  now,
			UpdatedAt:  now,
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
			OGVersion:  row.OGVersion,
			OGTemplate: row.OGTemplate,
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
		row.OGVersion++
		if in.PublishedAt.IsZero() {
			row.PublishedAt = sql.NullTime{}
		} else {
			row.PublishedAt = sql.NullTime{Time: in.PublishedAt.UTC(), Valid: true}
		}
		if _, err := tx.NewUpdate().
			Model(&row).
			Column("slug", "updated_at", "published_at", "og_version").
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
			OGVersion:  row.OGVersion,
			OGTemplate: row.OGTemplate,
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

// BumpOGVersion changes the public OGP URL so the image is rendered again.
func (a *Articles) BumpOGVersion(ctx context.Context, articleID int64) (Article, error) {
	res, err := a.db.NewUpdate().
		Model((*dbArticle)(nil)).
		Set("og_version = og_version + 1").
		Where("id = ?", articleID).
		Exec(ctx)
	if err != nil {
		return Article{}, fmt.Errorf("bump OGP version: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return Article{}, err
	}
	if n == 0 {
		return Article{}, ErrNotFound
	}
	return a.GetByID(ctx, articleID)
}

// SetOGTemplate selects the article OGP design and invalidates its cached image.
func (a *Articles) SetOGTemplate(ctx context.Context, articleID int64, template string) (Article, error) {
	switch template {
	case OGTemplateEditorial, OGTemplateDotDark:
	default:
		return Article{}, fmt.Errorf("%w: invalid OGP template %q", ErrInvalidInput, template)
	}

	res, err := a.db.NewUpdate().
		Model((*dbArticle)(nil)).
		Set("og_template = ?", template).
		Set("og_version = og_version + 1").
		Where("id = ?", articleID).
		Where("og_template <> ?", template).
		Exec(ctx)
	if err != nil {
		return Article{}, fmt.Errorf("set OGP template: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return Article{}, err
	}
	if n == 0 {
		var exists bool
		exists, err = a.db.NewSelect().
			Model((*dbArticle)(nil)).
			Where("id = ?", articleID).
			Exists(ctx)
		if err != nil {
			return Article{}, err
		}
		if !exists {
			return Article{}, ErrNotFound
		}
	}
	return a.GetByID(ctx, articleID)
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
