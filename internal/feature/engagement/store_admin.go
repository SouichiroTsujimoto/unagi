package engagement

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SouichiroTsujimoto/unagi/internal/feature/article"
	"github.com/uptrace/bun"
)

// ListStickersByArticleID returns stickers for an article in placement order (admin).
func (e *Engagement) ListStickersByArticleID(ctx context.Context, articleID int64) ([]Sticker, error) {
	if _, err := e.articles.GetByID(ctx, articleID); err != nil {
		if errors.Is(err, article.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return e.listStickers(ctx, articleID)
}

// DeleteStickersByIDs removes selected stickers that belong to the article.
func (e *Engagement) DeleteStickersByIDs(ctx context.Context, articleID int64, ids []int64) (int64, error) {
	if _, err := e.articles.GetByID(ctx, articleID); err != nil {
		if errors.Is(err, article.ErrNotFound) {
			return 0, ErrNotFound
		}
		return 0, err
	}
	uniq := uniquePositiveIDs(ids)
	if len(uniq) == 0 {
		return 0, fmt.Errorf("%w: ids required", ErrInvalidInput)
	}
	res, err := e.db.NewDelete().
		Model((*dbSticker)(nil)).
		Where("article_id = ?", articleID).
		Where("id IN (?)", bun.In(uniq)).
		Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("delete stickers: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return n, nil
}

// DeleteAllStickers removes every sticker on the article board.
func (e *Engagement) DeleteAllStickers(ctx context.Context, articleID int64) (int64, error) {
	if _, err := e.articles.GetByID(ctx, articleID); err != nil {
		if errors.Is(err, article.ErrNotFound) {
			return 0, ErrNotFound
		}
		return 0, err
	}
	res, err := e.db.NewDelete().
		Model((*dbSticker)(nil)).
		Where("article_id = ?", articleID).
		Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("delete all stickers: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return n, nil
}

// ListCommentsByArticleID returns all comments including hidden ones (admin).
func (e *Engagement) ListCommentsByArticleID(ctx context.Context, articleID int64) ([]AdminComment, error) {
	if _, err := e.articles.GetByID(ctx, articleID); err != nil {
		if errors.Is(err, article.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var rows []dbComment
	err := e.db.NewSelect().
		Model(&rows).
		Where("article_id = ?", articleID).
		OrderExpr("created_at ASC, id ASC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	out := make([]AdminComment, 0, len(rows))
	for _, row := range rows {
		out = append(out, adminCommentFromRow(row))
	}
	return out, nil
}

// SetCommentStatus updates a comment's visibility.
func (e *Engagement) SetCommentStatus(ctx context.Context, articleID, commentID int64, status string) (AdminComment, error) {
	status = strings.TrimSpace(status)
	if status != CommentStatusVisible && status != CommentStatusHidden {
		return AdminComment{}, fmt.Errorf("%w: invalid status", ErrInvalidInput)
	}
	if _, err := e.articles.GetByID(ctx, articleID); err != nil {
		if errors.Is(err, article.ErrNotFound) {
			return AdminComment{}, ErrNotFound
		}
		return AdminComment{}, err
	}
	var row dbComment
	err := e.db.NewSelect().
		Model(&row).
		Where("article_id = ?", articleID).
		Where("id = ?", commentID).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return AdminComment{}, ErrNotFound
	}
	if err != nil {
		return AdminComment{}, err
	}
	now := time.Now().UTC()
	row.Status = status
	row.UpdatedAt = now
	if _, err := e.db.NewUpdate().Model(&row).Column("status", "updated_at").WherePK().Exec(ctx); err != nil {
		return AdminComment{}, fmt.Errorf("update comment status: %w", err)
	}
	return adminCommentFromRow(row), nil
}

// DeleteCommentsByIDs removes selected comments that belong to the article.
func (e *Engagement) DeleteCommentsByIDs(ctx context.Context, articleID int64, ids []int64) (int64, error) {
	if _, err := e.articles.GetByID(ctx, articleID); err != nil {
		if errors.Is(err, article.ErrNotFound) {
			return 0, ErrNotFound
		}
		return 0, err
	}
	uniq := uniquePositiveIDs(ids)
	if len(uniq) == 0 {
		return 0, fmt.Errorf("%w: ids required", ErrInvalidInput)
	}
	res, err := e.db.NewDelete().
		Model((*dbComment)(nil)).
		Where("article_id = ?", articleID).
		Where("id IN (?)", bun.In(uniq)).
		Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("delete comments: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return n, nil
}

// DeleteAllComments removes every comment on the article.
func (e *Engagement) DeleteAllComments(ctx context.Context, articleID int64) (int64, error) {
	if _, err := e.articles.GetByID(ctx, articleID); err != nil {
		if errors.Is(err, article.ErrNotFound) {
			return 0, ErrNotFound
		}
		return 0, err
	}
	res, err := e.db.NewDelete().
		Model((*dbComment)(nil)).
		Where("article_id = ?", articleID).
		Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("delete all comments: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return n, nil
}

func uniquePositiveIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
