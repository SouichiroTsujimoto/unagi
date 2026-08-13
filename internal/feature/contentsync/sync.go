package contentsync

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SouichiroTsujimoto/unagi/internal/feature/article"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/media"
	"github.com/uptrace/bun"
)

const (
	advisoryLockKey int64 = 872314001
	Repository            = "SouichiroTsujimoto/unagi-content"
)

type Config struct {
	Secret string
}

type Sync struct {
	db       *bun.DB
	articles *article.Articles
	media    *media.Library
	cfg      Config
}

func New(db *bun.DB, articles *article.Articles, library *media.Library, cfg Config) (*Sync, error) {
	cfg.Secret = strings.TrimSpace(cfg.Secret)
	if articles == nil || db == nil {
		return nil, fmt.Errorf("contentsync: db and articles are required")
	}
	return &Sync{db: db, articles: articles, media: library, cfg: cfg}, nil
}

func (s *Sync) Configured() bool {
	return s != nil && s.cfg.Secret != ""
}

func (s *Sync) Repository() string {
	if s == nil {
		return ""
	}
	return Repository
}

func (s *Sync) Secret() string {
	if s == nil {
		return ""
	}
	return s.cfg.Secret
}

type Result struct {
	CommitSHA    string `json:"commit_sha"`
	RunID        string `json:"run_id"`
	Created      int    `json:"created"`
	Updated      int    `json:"updated"`
	Unchanged    int    `json:"unchanged"`
	Deleted      int    `json:"deleted"`
	ArticleCount int    `json:"article_count"`
}

type Upload struct {
	Path        string `json:"path"`
	ObjectKey   string `json:"object_key"`
	SignedURL   string `json:"signed_url"`
	ContentType string `json:"content_type"`
}

type dbSyncRun struct {
	bun.BaseModel `bun:"table:content_sync_runs,alias:csr"`

	ID           int64     `bun:",pk,autoincrement"`
	RunID        string    `bun:"run_id,notnull"`
	CommitSHA    string    `bun:"commit_sha,notnull"`
	Repository   string    `bun:"repository,notnull"`
	AppliedAt    time.Time `bun:"applied_at,notnull"`
	ArticleCount int       `bun:"article_count,notnull"`
}

func (s *Sync) DryRun(ctx context.Context, snap Snapshot) (Result, error) {
	articles, _, err := prepareSnapshot(snap, Repository)
	if err != nil {
		return Result{}, err
	}
	existing, err := s.articles.ListSyncMeta(ctx)
	if err != nil {
		return Result{}, err
	}
	bySlug := make(map[string]article.SyncMeta, len(existing))
	for _, item := range existing {
		bySlug[item.Slug] = item
	}
	var counts article.SnapshotCounts
	seen := make(map[string]struct{}, len(articles))
	for _, item := range articles {
		seen[item.Slug] = struct{}{}
		cur, ok := bySlug[item.Slug]
		if !ok {
			counts.Created++
			continue
		}
		if cur.SourceHash == item.SourceHash && cur.SourcePath == item.SourcePath {
			counts.Unchanged++
			continue
		}
		counts.Updated++
	}
	for slug := range bySlug {
		if _, ok := seen[slug]; !ok {
			counts.Deleted++
		}
	}
	return Result{
		CommitSHA:    strings.TrimSpace(snap.CommitSHA),
		RunID:        strings.TrimSpace(snap.RunID),
		Created:      counts.Created,
		Updated:      counts.Updated,
		Unchanged:    counts.Unchanged,
		Deleted:      counts.Deleted,
		ArticleCount: len(articles),
	}, nil
}

func (s *Sync) PlanUploads(ctx context.Context, snap Snapshot) ([]Upload, error) {
	if s.media == nil {
		return nil, fmt.Errorf("contentsync: media library is required")
	}
	_, images, err := prepareSnapshot(snap, Repository)
	if err != nil {
		return nil, err
	}
	var uploads []Upload
	for _, img := range images {
		exists, err := s.media.Exists(ctx, img.ObjectKey)
		if err != nil {
			return nil, err
		}
		if exists {
			continue
		}
		signed, err := s.media.BeginKeyedUpload(ctx, img.ObjectKey, img.ContentType, img.Size)
		if err != nil {
			return nil, err
		}
		uploads = append(uploads, Upload{
			Path:        img.Path,
			ObjectKey:   img.ObjectKey,
			SignedURL:   signed.SignedURL,
			ContentType: signed.ContentType,
		})
	}
	if uploads == nil {
		uploads = []Upload{}
	}
	return uploads, nil
}

func (s *Sync) Apply(ctx context.Context, snap Snapshot) (Result, error) {
	articles, images, err := prepareSnapshot(snap, Repository)
	if err != nil {
		return Result{}, err
	}
	if err := s.ensureImagesStored(ctx, images); err != nil {
		return Result{}, err
	}

	items := make([]article.SyncContent, 0, len(articles))
	for _, a := range articles {
		items = append(items, a.SyncContent)
	}

	var result Result
	err = s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(?)", advisoryLockKey); err != nil {
			return fmt.Errorf("advisory lock: %w", err)
		}
		if err := insertRun(ctx, tx, snap, len(items)); err != nil {
			return err
		}
		counts, err := s.articles.ApplySnapshotTx(ctx, tx, items)
		if err != nil {
			return err
		}
		result = Result{
			CommitSHA:    strings.TrimSpace(snap.CommitSHA),
			RunID:        strings.TrimSpace(snap.RunID),
			Created:      counts.Created,
			Updated:      counts.Updated,
			Unchanged:    counts.Unchanged,
			Deleted:      counts.Deleted,
			ArticleCount: len(items),
		}
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	for _, img := range images {
		if err := s.media.Upsert(ctx, media.Media{
			ObjectKey:   img.ObjectKey,
			ContentType: img.ContentType,
			SizeBytes:   img.Size,
			SHA256:      img.SHA256,
			CreatedAt:   time.Now().UTC(),
		}); err != nil {
			return Result{}, err
		}
	}
	return result, nil
}

func (s *Sync) ensureImagesStored(ctx context.Context, images []preparedImage) error {
	if len(images) == 0 {
		return nil
	}
	if s.media == nil {
		return fmt.Errorf("contentsync: media library is required")
	}
	for _, img := range images {
		ok, err := s.media.Exists(ctx, img.ObjectKey)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%w: %s", ErrMissingImage, img.Path)
		}
	}
	return nil
}

func insertRun(ctx context.Context, tx bun.Tx, snap Snapshot, articleCount int) error {
	run := &dbSyncRun{
		RunID:        strings.TrimSpace(snap.RunID),
		CommitSHA:    strings.TrimSpace(snap.CommitSHA),
		Repository:   strings.TrimSpace(snap.Repository),
		AppliedAt:    time.Now().UTC(),
		ArticleCount: articleCount,
	}
	_, err := tx.NewInsert().Model(run).Exec(ctx)
	if err == nil {
		return nil
	}
	if isUniqueViolation(err) {
		return ErrDuplicateRun
	}
	return fmt.Errorf("insert sync run: %w", err)
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate key") || strings.Contains(msg, "unique constraint")
}
