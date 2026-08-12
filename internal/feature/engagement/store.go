package engagement

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"

	"github.com/SouichiroTsujimoto/unagi/internal/feature/article"
)

// GetSnapshot returns stickers and visible comments for a published article.
func (e *Engagement) GetSnapshot(ctx context.Context, slug string, now time.Time) (Snapshot, error) {
	art, err := e.articles.Get(ctx, slug, now)
	if errors.Is(err, article.ErrNotFound) {
		return Snapshot{}, ErrNotFound
	}
	if err != nil {
		return Snapshot{}, err
	}

	stickers, err := e.listStickers(ctx, art.ID)
	if err != nil {
		return Snapshot{}, err
	}
	comments, err := e.listComments(ctx, art.ID)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{
		Stickers:      stickers,
		Comments:      comments,
		AllowedEmoji:  append([]string(nil), AllowedEmoji...),
		LoginPath:     LoginPath,
		Authenticated: false,
	}, nil
}

// AddEmojiSticker places an anonymous emoji sticker on a published article.
func (e *Engagement) AddEmojiSticker(ctx context.Context, slug string, now time.Time, in AddEmojiInput) (Sticker, error) {
	emoji, err := normalizeEmoji(in.Emoji)
	if err != nil {
		return Sticker{}, err
	}
	x, err := normalizeCoord(in.X, "x")
	if err != nil {
		return Sticker{}, err
	}
	y, err := normalizeCoord(in.Y, "y")
	if err != nil {
		return Sticker{}, err
	}
	if len(in.VisitorHash) != 32 {
		return Sticker{}, fmt.Errorf("%w: visitor hash required", ErrInvalidInput)
	}

	art, err := e.articles.Get(ctx, slug, now)
	if errors.Is(err, article.ErrNotFound) {
		return Sticker{}, ErrNotFound
	}
	if err != nil {
		return Sticker{}, err
	}

	total, err := e.db.NewSelect().Model((*dbSticker)(nil)).Where("article_id = ?", art.ID).Count(ctx)
	if err != nil {
		return Sticker{}, fmt.Errorf("count stickers: %w", err)
	}
	if total >= MaxStickersPerArticle {
		return Sticker{}, fmt.Errorf("%w: board full", ErrLimitExceeded)
	}

	visitorCount, err := e.db.NewSelect().
		Model((*dbSticker)(nil)).
		Where("article_id = ?", art.ID).
		Where("visitor_hash = ?", in.VisitorHash).
		Count(ctx)
	if err != nil {
		return Sticker{}, fmt.Errorf("count visitor stickers: %w", err)
	}
	if visitorCount >= MaxStickersPerVisitorArticle {
		return Sticker{}, fmt.Errorf("%w: visitor limit", ErrLimitExceeded)
	}

	createdAt := now
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	row := &dbSticker{
		ArticleID:   art.ID,
		Kind:        KindEmoji,
		Value:       emoji,
		X:           x,
		Y:           y,
		VisitorHash: append([]byte(nil), in.VisitorHash...),
		CreatedAt:   createdAt.UTC(),
	}
	if _, err := e.db.NewInsert().Model(row).Exec(ctx); err != nil {
		return Sticker{}, fmt.Errorf("insert sticker: %w", err)
	}
	return stickerFromRow(*row), nil
}

// AddAvatarSticker will place an X avatar sticker once OAuth exists.
func (e *Engagement) AddAvatarSticker(ctx context.Context, slug string, now time.Time) (Sticker, error) {
	if err := e.requireLoginForPublished(ctx, slug, now); err != nil {
		return Sticker{}, err
	}
	return Sticker{}, ErrLoginRequired
}

// AddComment will create a comment once OAuth exists.
func (e *Engagement) AddComment(ctx context.Context, slug string, now time.Time, body string) (Comment, error) {
	if _, err := normalizeCommentBody(body); err != nil {
		return Comment{}, err
	}
	if err := e.requireLoginForPublished(ctx, slug, now); err != nil {
		return Comment{}, err
	}
	return Comment{}, ErrLoginRequired
}

func (e *Engagement) requireLoginForPublished(ctx context.Context, slug string, now time.Time) error {
	_, err := e.articles.Get(ctx, slug, now)
	if errors.Is(err, article.ErrNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	return ErrLoginRequired
}

func (e *Engagement) listStickers(ctx context.Context, articleID int64) ([]Sticker, error) {
	var rows []dbSticker
	err := e.db.NewSelect().
		Model(&rows).
		Where("article_id = ?", articleID).
		OrderExpr("id ASC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list stickers: %w", err)
	}
	out := make([]Sticker, 0, len(rows))
	for _, row := range rows {
		out = append(out, stickerFromRow(row))
	}
	return out, nil
}

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

func (e *Engagement) listComments(ctx context.Context, articleID int64) ([]Comment, error) {
	var rows []dbComment
	err := e.db.NewSelect().
		Model(&rows).
		Where("article_id = ?", articleID).
		Where("status = ?", CommentStatusVisible).
		OrderExpr("created_at ASC, id ASC").
		Limit(MaxCommentsPerArticle).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	out := make([]Comment, 0, len(rows))
	for _, row := range rows {
		out = append(out, commentFromRow(row))
	}
	return out, nil
}

func stickerFromRow(row dbSticker) Sticker {
	return Sticker{
		ID:          row.ID,
		Kind:        row.Kind,
		Value:       row.Value,
		X:           row.X,
		Y:           row.Y,
		Username:    row.Username,
		DisplayName: row.DisplayName,
		CreatedAt:   row.CreatedAt,
	}
}

func commentFromRow(row dbComment) Comment {
	return Comment{
		ID:          row.ID,
		Body:        row.Body,
		Username:    row.Username,
		DisplayName: row.DisplayName,
		AvatarURL:   row.AvatarURL,
		CreatedAt:   row.CreatedAt,
	}
}

// SeedVisibleComment inserts a visible comment for tests.
func (e *Engagement) SeedVisibleComment(ctx context.Context, articleID int64, body, username, displayName, avatarURL string, at time.Time) (Comment, error) {
	body, err := normalizeCommentBody(body)
	if err != nil {
		return Comment{}, err
	}
	if at.IsZero() {
		at = time.Now()
	}
	row := &dbComment{
		ArticleID:   articleID,
		Body:        body,
		Status:      CommentStatusVisible,
		Username:    username,
		DisplayName: displayName,
		AvatarURL:   avatarURL,
		CreatedAt:   at.UTC(),
		UpdatedAt:   at.UTC(),
	}
	if _, err := e.db.NewInsert().Model(row).Exec(ctx); err != nil {
		return Comment{}, fmt.Errorf("insert comment: %w", err)
	}
	return commentFromRow(*row), nil
}

// ArticleIDBySlug resolves a published article id for tests.
func (e *Engagement) ArticleIDBySlug(ctx context.Context, slug string, now time.Time) (int64, error) {
	art, err := e.articles.Get(ctx, slug, now)
	if errors.Is(err, article.ErrNotFound) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	return art.ID, nil
}
