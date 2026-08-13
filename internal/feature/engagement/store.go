package engagement

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SouichiroTsujimoto/unagi/internal/feature/article"
)

// GetSnapshot returns stickers and visible comments for a published article.
func (e *Engagement) GetSnapshot(ctx context.Context, slug string, now time.Time, viewer *Viewer) (Snapshot, error) {
	art, err := e.articles.Get(ctx, slug, now)
	if errors.Is(err, article.ErrNotFound) {
		return Snapshot{}, ErrNotFound
	}
	if err != nil {
		return Snapshot{}, err
	}
	if viewer != nil {
		normalized := *viewer
		normalized.AvatarURL = highResolutionXAvatarURL(viewer.AvatarURL)
		viewer = &normalized
	}

	stickers, err := e.listStickers(ctx, art.ID)
	if err != nil {
		return Snapshot{}, err
	}
	comments, err := e.listComments(ctx, art.ID, viewerXUserID(viewer))
	if err != nil {
		return Snapshot{}, err
	}
	snap := Snapshot{
		Stickers:      stickers,
		Comments:      comments,
		AllowedEmoji:  append([]string(nil), AllowedEmoji...),
		LoginPath:     LoginPath,
		Authenticated: viewer != nil,
		Viewer:        viewer,
	}
	if viewer != nil {
		snap.LogoutPath = "/auth/x/logout"
	}
	return snap, nil
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

	createdAt := now
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	row := &dbSticker{
		ArticleID: art.ID,
		Kind:      KindEmoji,
		Value:     emoji,
		X:         x,
		Y:         y,
		CreatedAt: createdAt.UTC(),
	}
	if _, err := e.db.NewInsert().Model(row).Exec(ctx); err != nil {
		return Sticker{}, fmt.Errorf("insert sticker: %w", err)
	}
	return stickerFromRow(*row), nil
}

// AddAvatarSticker places an X avatar sticker for a signed-in reader.
func (e *Engagement) AddAvatarSticker(ctx context.Context, slug string, now time.Time, author Author, x, y float64) (Sticker, error) {
	if err := validateAuthor(author); err != nil {
		return Sticker{}, err
	}
	if strings.TrimSpace(author.AvatarURL) == "" {
		return Sticker{}, fmt.Errorf("%w: avatar required", ErrInvalidInput)
	}
	x, err := normalizeCoord(x, "x")
	if err != nil {
		return Sticker{}, err
	}
	y, err = normalizeCoord(y, "y")
	if err != nil {
		return Sticker{}, err
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

	createdAt := now
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	xUserID := author.XUserID
	row := &dbSticker{
		ArticleID:   art.ID,
		Kind:        KindAvatar,
		Value:       highResolutionXAvatarURL(author.AvatarURL),
		X:           x,
		Y:           y,
		XUserID:     &xUserID,
		Username:    author.Username,
		DisplayName: author.DisplayName,
		CreatedAt:   createdAt.UTC(),
	}
	if _, err := e.db.NewInsert().Model(row).Exec(ctx); err != nil {
		return Sticker{}, fmt.Errorf("insert avatar sticker: %w", err)
	}
	return stickerFromRow(*row), nil
}

// AddComment creates a visible comment for a signed-in reader.
func (e *Engagement) AddComment(ctx context.Context, slug string, now time.Time, author Author, body string) (Comment, error) {
	if err := validateAuthor(author); err != nil {
		return Comment{}, err
	}
	body, err := normalizeCommentBody(body)
	if err != nil {
		return Comment{}, err
	}

	art, err := e.articles.Get(ctx, slug, now)
	if errors.Is(err, article.ErrNotFound) {
		return Comment{}, ErrNotFound
	}
	if err != nil {
		return Comment{}, err
	}

	total, err := e.db.NewSelect().Model((*dbComment)(nil)).Where("article_id = ?", art.ID).Count(ctx)
	if err != nil {
		return Comment{}, fmt.Errorf("count comments: %w", err)
	}
	if total >= MaxCommentsPerArticle {
		return Comment{}, fmt.Errorf("%w: comments full", ErrLimitExceeded)
	}

	userCount, err := e.db.NewSelect().
		Model((*dbComment)(nil)).
		Where("article_id = ?", art.ID).
		Where("x_user_id = ?", author.XUserID).
		Count(ctx)
	if err != nil {
		return Comment{}, fmt.Errorf("count user comments: %w", err)
	}
	if userCount >= MaxCommentsPerUserArticle {
		return Comment{}, fmt.Errorf("%w: user comment limit", ErrLimitExceeded)
	}

	createdAt := now
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	xUserID := author.XUserID
	row := &dbComment{
		ArticleID:   art.ID,
		Body:        body,
		Status:      CommentStatusVisible,
		XUserID:     &xUserID,
		Username:    author.Username,
		DisplayName: author.DisplayName,
		AvatarURL:   highResolutionXAvatarURL(author.AvatarURL),
		CreatedAt:   createdAt.UTC(),
		UpdatedAt:   createdAt.UTC(),
	}
	if _, err := e.db.NewInsert().Model(row).Exec(ctx); err != nil {
		return Comment{}, fmt.Errorf("insert comment: %w", err)
	}
	out := commentFromRow(*row, author.XUserID)
	return out, nil
}

// DeleteOwnComment removes a comment owned by the signed-in reader.
func (e *Engagement) DeleteOwnComment(ctx context.Context, slug string, now time.Time, author Author, commentID int64) error {
	if err := validateAuthor(author); err != nil {
		return err
	}
	if commentID <= 0 {
		return fmt.Errorf("%w: comment id required", ErrInvalidInput)
	}

	art, err := e.articles.Get(ctx, slug, now)
	if errors.Is(err, article.ErrNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	res, err := e.db.NewDelete().
		Model((*dbComment)(nil)).
		Where("article_id = ?", art.ID).
		Where("id = ?", commentID).
		Where("x_user_id = ?", author.XUserID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete own comment: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func viewerXUserID(viewer *Viewer) string {
	if viewer == nil {
		return ""
	}
	return viewer.XUserID
}

func validateAuthor(author Author) error {
	if strings.TrimSpace(author.XUserID) == "" || strings.TrimSpace(author.Username) == "" {
		return ErrLoginRequired
	}
	return nil
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

func (e *Engagement) listComments(ctx context.Context, articleID int64, viewerXUserID string) ([]Comment, error) {
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
		out = append(out, commentFromRow(row, viewerXUserID))
	}
	return out, nil
}

func stickerFromRow(row dbSticker) Sticker {
	value := row.Value
	if row.Kind == KindAvatar {
		value = highResolutionXAvatarURL(value)
	}
	return Sticker{
		ID:          row.ID,
		Kind:        row.Kind,
		Value:       value,
		X:           row.X,
		Y:           row.Y,
		Username:    row.Username,
		DisplayName: row.DisplayName,
		CreatedAt:   row.CreatedAt,
	}
}

func commentFromRow(row dbComment, viewerXUserID string) Comment {
	out := Comment{
		ID:          row.ID,
		Body:        row.Body,
		Username:    row.Username,
		DisplayName: row.DisplayName,
		AvatarURL:   highResolutionXAvatarURL(row.AvatarURL),
		CreatedAt:   row.CreatedAt,
	}
	if viewerXUserID != "" && row.XUserID != nil && *row.XUserID == viewerXUserID {
		out.Mine = true
	}
	return out
}

func adminCommentFromRow(row dbComment) AdminComment {
	return AdminComment{
		ID:          row.ID,
		Body:        row.Body,
		Status:      row.Status,
		Username:    row.Username,
		DisplayName: row.DisplayName,
		AvatarURL:   highResolutionXAvatarURL(row.AvatarURL),
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
	return commentFromRow(*row, ""), nil
}

// SeedCommentWithUser inserts a comment attributed to an X user for tests.
func (e *Engagement) SeedCommentWithUser(ctx context.Context, articleID int64, author Author, body, status string, at time.Time) (AdminComment, error) {
	body, err := normalizeCommentBody(body)
	if err != nil {
		return AdminComment{}, err
	}
	if status == "" {
		status = CommentStatusVisible
	}
	if at.IsZero() {
		at = time.Now()
	}
	xUserID := author.XUserID
	row := &dbComment{
		ArticleID:   articleID,
		Body:        body,
		Status:      status,
		XUserID:     &xUserID,
		Username:    author.Username,
		DisplayName: author.DisplayName,
		AvatarURL:   author.AvatarURL,
		CreatedAt:   at.UTC(),
		UpdatedAt:   at.UTC(),
	}
	if _, err := e.db.NewInsert().Model(row).Exec(ctx); err != nil {
		return AdminComment{}, fmt.Errorf("insert comment: %w", err)
	}
	return adminCommentFromRow(*row), nil
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
