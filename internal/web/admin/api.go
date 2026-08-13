package admin

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/SouichiroTsujimoto/unagi/internal/feature/article"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/engagement"
	"github.com/labstack/echo/v4"
)

func (h *Handler) PublishArticle(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	item, err := h.articles.GetByID(c.Request().Context(), id)
	if errors.Is(err, article.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}
	if err != nil {
		return err
	}
	published, err := h.articles.Publish(c.Request().Context(), id, item.RevisionID, item.PublishedAt)
	if err != nil {
		return err
	}
	if wantsJSON(c) {
		return c.JSON(http.StatusOK, published)
	}
	return c.Redirect(http.StatusSeeOther, "/admin")
}

func (h *Handler) UnpublishArticle(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	item, err := h.articles.Unpublish(c.Request().Context(), id)
	if errors.Is(err, article.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}
	if err != nil {
		return err
	}
	if wantsJSON(c) {
		return c.JSON(http.StatusOK, item)
	}
	return c.Redirect(http.StatusSeeOther, "/admin")
}

func (h *Handler) ListStickers(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	stickers, err := h.engagement.ListStickersByArticleID(c.Request().Context(), id)
	if errors.Is(err, engagement.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}
	if err != nil {
		h.log.Error("list stickers", "err", err, "article_id", id)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	return c.JSON(http.StatusOK, map[string]any{"stickers": stickers})
}

func (h *Handler) DeleteStickers(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	var body struct {
		IDs []int64 `json:"ids"`
		All bool    `json:"all"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid json")
	}

	var n int64
	if body.All {
		n, err = h.engagement.DeleteAllStickers(c.Request().Context(), id)
	} else {
		n, err = h.engagement.DeleteStickersByIDs(c.Request().Context(), id, body.IDs)
	}
	if errors.Is(err, engagement.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}
	if errors.Is(err, engagement.ErrInvalidInput) {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid input")
	}
	if err != nil {
		h.log.Error("delete stickers", "err", err, "article_id", id)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	return c.JSON(http.StatusOK, map[string]any{"deleted": n})
}

func (h *Handler) ListComments(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	comments, err := h.engagement.ListCommentsByArticleID(c.Request().Context(), id)
	if errors.Is(err, engagement.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}
	if err != nil {
		h.log.Error("list comments", "err", err, "article_id", id)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	return c.JSON(http.StatusOK, map[string]any{"comments": comments})
}

func (h *Handler) UpdateComment(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	commentID, err := strconv.ParseInt(c.Param("commentID"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid comment id")
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid json")
	}
	comment, err := h.engagement.SetCommentStatus(c.Request().Context(), id, commentID, body.Status)
	if errors.Is(err, engagement.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}
	if errors.Is(err, engagement.ErrInvalidInput) {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid input")
	}
	if err != nil {
		h.log.Error("update comment", "err", err, "article_id", id, "comment_id", commentID)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	return c.JSON(http.StatusOK, comment)
}

func (h *Handler) DeleteComments(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	var body struct {
		IDs []int64 `json:"ids"`
		All bool    `json:"all"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid json")
	}

	var n int64
	if body.All {
		n, err = h.engagement.DeleteAllComments(c.Request().Context(), id)
	} else {
		n, err = h.engagement.DeleteCommentsByIDs(c.Request().Context(), id, body.IDs)
	}
	if errors.Is(err, engagement.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}
	if errors.Is(err, engagement.ErrInvalidInput) {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid input")
	}
	if err != nil {
		h.log.Error("delete comments", "err", err, "article_id", id)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	return c.JSON(http.StatusOK, map[string]any{"deleted": n})
}
