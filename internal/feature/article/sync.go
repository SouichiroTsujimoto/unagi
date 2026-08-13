package article

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

// SyncMeta is the Git identity of an article row.
type SyncMeta struct {
	Slug       string
	SourcePath string
	SourceHash string
}

// ListSyncMeta returns slug/source fields for every article.
func (a *Articles) ListSyncMeta(ctx context.Context) ([]SyncMeta, error) {
	var rows []dbArticle
	if err := a.db.NewSelect().Model(&rows).Column("slug", "source_path", "source_hash").Scan(ctx); err != nil {
		return nil, fmt.Errorf("list sync meta: %w", err)
	}
	out := make([]SyncMeta, 0, len(rows))
	for _, row := range rows {
		out = append(out, SyncMeta{Slug: row.Slug, SourcePath: row.SourcePath, SourceHash: row.SourceHash})
	}
	return out, nil
}

// SyncContent is one article from a Git snapshot, already parsed and rewritten.
type SyncContent struct {
	Slug       string
	SourcePath string
	SourceHash string
	Title      string
	Emoji      string
	Type       string
	Topics     []string
	BodyMD     string
}

// SnapshotCounts is the result of applying a full article snapshot.
type SnapshotCounts struct {
	Created   int
	Updated   int
	Unchanged int
	Deleted   int
}

// ApplySnapshotTx upserts the snapshot and physically deletes articles that
// disappeared. It does not change publish status or published_at. A published
// article whose content changed gets a new published revision.
func (a *Articles) ApplySnapshotTx(ctx context.Context, tx bun.Tx, items []SyncContent) (SnapshotCounts, error) {
	var rows []dbArticle
	if err := tx.NewSelect().Model(&rows).Scan(ctx); err != nil {
		return SnapshotCounts{}, fmt.Errorf("list articles for sync: %w", err)
	}
	bySlug := make(map[string]dbArticle, len(rows))
	for _, row := range rows {
		bySlug[row.Slug] = row
	}

	now := time.Now().UTC()
	seen := make(map[string]struct{}, len(items))
	var counts SnapshotCounts
	for _, item := range items {
		seen[item.Slug] = struct{}{}
		row, ok := bySlug[item.Slug]
		if !ok {
			if err := syncCreate(ctx, tx, item, now); err != nil {
				return SnapshotCounts{}, err
			}
			counts.Created++
			continue
		}
		if row.SourceHash == item.SourceHash && row.SourcePath == item.SourcePath {
			counts.Unchanged++
			continue
		}
		if err := syncUpdate(ctx, tx, row, item, now); err != nil {
			return SnapshotCounts{}, err
		}
		counts.Updated++
	}

	var deleteIDs []int64
	for slug, row := range bySlug {
		if _, ok := seen[slug]; !ok {
			deleteIDs = append(deleteIDs, row.ID)
		}
	}
	if len(deleteIDs) > 0 {
		if _, err := tx.NewDelete().Model((*dbArticle)(nil)).Where("id IN (?)", bun.In(deleteIDs)).Exec(ctx); err != nil {
			return SnapshotCounts{}, fmt.Errorf("delete missing articles: %w", err)
		}
		counts.Deleted = len(deleteIDs)
	}
	return counts, nil
}

func syncCreate(ctx context.Context, tx bun.Tx, in SyncContent, now time.Time) error {
	row := &dbArticle{
		Slug:       in.Slug,
		Status:     StatusDraft,
		OGVersion:  1,
		OGTemplate: DefaultOGTemplate,
		SourcePath: in.SourcePath,
		SourceHash: in.SourceHash,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if _, err := tx.NewInsert().Model(row).Exec(ctx); err != nil {
		return fmt.Errorf("insert article %s: %w", in.Slug, err)
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
		return fmt.Errorf("insert revision %s: %w", in.Slug, err)
	}
	return replaceTopics(ctx, tx, row.ID, in.Topics)
}

func syncUpdate(ctx context.Context, tx bun.Tx, row dbArticle, in SyncContent, now time.Time) error {
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
		return fmt.Errorf("insert revision %s: %w", in.Slug, err)
	}
	row.SourcePath = in.SourcePath
	row.SourceHash = in.SourceHash
	row.UpdatedAt = now
	row.OGVersion++
	cols := []string{"source_path", "source_hash", "updated_at", "og_version"}
	if row.Status == StatusPublished && row.PublishedRevisionID.Valid {
		row.PublishedRevisionID = sql.NullInt64{Int64: rev.ID, Valid: true}
		cols = append(cols, "published_revision_id")
	}
	if _, err := tx.NewUpdate().Model(&row).Column(cols...).WherePK().Exec(ctx); err != nil {
		return fmt.Errorf("update article %s: %w", in.Slug, err)
	}
	return replaceTopics(ctx, tx, row.ID, in.Topics)
}
