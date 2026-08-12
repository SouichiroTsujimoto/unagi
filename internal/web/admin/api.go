package admin

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/SouichiroTsujimoto/unagi/internal/feature/adminauth"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/article"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/engagement"
	"github.com/labstack/echo/v4"
)

func (h *Handler) BeginSetup(c echo.Context) error {
	need, err := h.auth.NeedsBootstrap(c.Request().Context())
	if err != nil {
		return err
	}
	if !need {
		return echo.NewHTTPError(http.StatusForbidden, "bootstrap closed")
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid json")
	}
	if err := h.auth.VerifyBootstrapToken(body.Token); err != nil {
		return echo.NewHTTPError(http.StatusForbidden, "invalid bootstrap token")
	}
	options, err := h.auth.BeginRegistration(c.Request().Context())
	if err != nil {
		h.log.Error("begin setup registration", "err", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	return c.JSON(http.StatusOK, options)
}

func (h *Handler) FinishSetup(c echo.Context) error {
	need, err := h.auth.NeedsBootstrap(c.Request().Context())
	if err != nil {
		return err
	}
	if !need {
		return echo.NewHTTPError(http.StatusForbidden, "bootstrap closed")
	}
	token := c.Request().Header.Get("X-Bootstrap-Token")
	if err := h.auth.VerifyBootstrapToken(token); err != nil {
		return echo.NewHTTPError(http.StatusForbidden, "invalid bootstrap token")
	}
	displayName := c.QueryParam("name")
	if displayName == "" {
		displayName = "Primary"
	}
	if _, err := h.auth.FinishRegistration(c.Request().Context(), c.Request().Body, displayName); err != nil {
		h.log.Error("finish setup registration", "err", err)
		return echo.NewHTTPError(http.StatusBadRequest, "registration failed")
	}
	codes, err := h.auth.IssueRecoveryCodes(c.Request().Context(), 8)
	if err != nil {
		return err
	}
	_, rawToken, err := h.auth.CreateSession(c.Request().Context())
	if err != nil {
		return err
	}
	h.setSessionCookie(c, rawToken)
	return c.JSON(http.StatusOK, map[string]any{"ok": true, "recoveryCodes": codes})
}

func (h *Handler) BeginLogin(c echo.Context) error {
	options, err := h.auth.BeginLogin(c.Request().Context())
	if errors.Is(err, adminauth.ErrUnauthorized) {
		return echo.NewHTTPError(http.StatusUnauthorized, "no credentials")
	}
	if err != nil {
		h.log.Error("begin login", "err", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	return c.JSON(http.StatusOK, options)
}

func (h *Handler) FinishLoginAPI(c echo.Context) error {
	_, raw, err := h.auth.FinishLogin(c.Request().Context(), c.Request().Body)
	if err != nil {
		h.log.Error("finish login", "err", err)
		return echo.NewHTTPError(http.StatusUnauthorized, "login failed")
	}
	h.setSessionCookie(c, raw)
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) BeginRegisterPasskey(c echo.Context) error {
	options, err := h.auth.BeginRegistration(c.Request().Context())
	if err != nil {
		h.log.Error("begin register", "err", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	return c.JSON(http.StatusOK, options)
}

func (h *Handler) FinishRegisterPasskey(c echo.Context) error {
	name := c.QueryParam("name")
	if name == "" {
		name = "Passkey"
	}
	cred, err := h.auth.FinishRegistration(c.Request().Context(), c.Request().Body, name)
	if err != nil {
		h.log.Error("finish register", "err", err)
		return echo.NewHTTPError(http.StatusBadRequest, "registration failed")
	}
	return c.JSON(http.StatusCreated, map[string]any{"id": cred.ID, "displayName": cred.DisplayName})
}

func (h *Handler) DeletePasskey(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	if err := h.auth.DeleteCredential(c.Request().Context(), id); err != nil {
		if errors.Is(err, adminauth.ErrLastCredential) {
			return echo.NewHTTPError(http.StatusBadRequest, "cannot delete last credential")
		}
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) Recover(c echo.Context) error {
	var body struct {
		Code string `json:"code"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid json")
	}
	if err := h.auth.ConsumeRecoveryCode(c.Request().Context(), body.Code); err != nil {
		return echo.NewHTTPError(http.StatusForbidden, "invalid recovery code")
	}
	options, err := h.auth.BeginRegistration(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, options)
}

func (h *Handler) CreateArticle(c echo.Context) error {
	in, err := bindSaveInput(c)
	if err != nil {
		return err
	}
	created, err := h.articles.Create(c.Request().Context(), in)
	if errors.Is(err, article.ErrSlugExists) {
		return echo.NewHTTPError(http.StatusConflict, "slug exists")
	}
	if errors.Is(err, article.ErrInvalidInput) || errors.Is(err, article.ErrInvalidSlug) {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err != nil {
		h.log.Error("create article", "err", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	return c.JSON(http.StatusCreated, created)
}

func (h *Handler) SaveArticle(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	in, err := bindSaveInput(c)
	if err != nil {
		return err
	}
	saved, err := h.articles.SaveRevision(c.Request().Context(), id, in)
	if errors.Is(err, article.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}
	if errors.Is(err, article.ErrSlugExists) {
		return echo.NewHTTPError(http.StatusConflict, "slug exists")
	}
	if errors.Is(err, article.ErrInvalidInput) || errors.Is(err, article.ErrInvalidSlug) {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err != nil {
		h.log.Error("save article", "err", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	return c.JSON(http.StatusOK, saved)
}

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
	var body struct {
		PublishedAt string `json:"publishedAt"`
	}
	_ = c.Bind(&body)
	publishedAt := item.PublishedAt
	if body.PublishedAt != "" {
		if t, err := time.ParseInLocation("2006-01-02 15:04", body.PublishedAt, time.FixedZone("Asia/Tokyo", 9*60*60)); err == nil {
			publishedAt = t
		}
	}
	published, err := h.articles.Publish(c.Request().Context(), id, item.RevisionID, publishedAt)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, published)
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
	return c.JSON(http.StatusOK, item)
}

func (h *Handler) Preview(c echo.Context) error {
	var body struct {
		BodyMD string `json:"bodyMd"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid json")
	}
	html, err := h.articles.RenderHTMLSync(c.Request().Context(), body.BodyMD)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "render failed")
	}
	return c.JSON(http.StatusOK, map[string]string{"html": html})
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

func bindSaveInput(c echo.Context) (article.SaveInput, error) {
	var body struct {
		Slug        string   `json:"slug"`
		Title       string   `json:"title"`
		Emoji       string   `json:"emoji"`
		Type        string   `json:"type"`
		Topics      []string `json:"topics"`
		BodyMD      string   `json:"bodyMd"`
		PublishedAt string   `json:"publishedAt"`
	}
	if err := c.Bind(&body); err != nil {
		return article.SaveInput{}, echo.NewHTTPError(http.StatusBadRequest, "invalid json")
	}
	in := article.SaveInput{
		Slug:   body.Slug,
		Title:  body.Title,
		Emoji:  body.Emoji,
		Type:   body.Type,
		Topics: body.Topics,
		BodyMD: body.BodyMD,
	}
	if body.PublishedAt != "" {
		t, err := time.ParseInLocation("2006-01-02 15:04", body.PublishedAt, time.FixedZone("Asia/Tokyo", 9*60*60))
		if err != nil {
			t2, err2 := time.ParseInLocation("2006-01-02", body.PublishedAt, time.FixedZone("Asia/Tokyo", 9*60*60))
			if err2 != nil {
				return article.SaveInput{}, echo.NewHTTPError(http.StatusBadRequest, "invalid publishedAt")
			}
			t = t2
		}
		in.PublishedAt = t
	}
	return in, nil
}
