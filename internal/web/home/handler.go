package home

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/SouichiroTsujimoto/unigo-template/internal/feature/account"
)

type Handler struct {
	accounts *account.Accounts
	log      *slog.Logger
}

func New(accounts *account.Accounts, log *slog.Logger) *Handler {
	return &Handler{
		accounts: accounts,
		log:      log,
	}
}

func (handler *Handler) Show(c echo.Context) error {
	accounts, err := handler.accounts.List(c.Request().Context())
	if err != nil {
		handler.log.Error("list accounts", "err", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}

	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
	if err := Page(accounts).Render(c.Request().Context(), c.Response()); err != nil {
		handler.log.Error("render home", "err", err)
		return err
	}
	return nil
}

func (handler *Handler) List(c echo.Context) error {
	accounts, err := handler.accounts.List(c.Request().Context())
	if err != nil {
		handler.log.Error("list accounts", "err", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	return c.JSON(http.StatusOK, accounts)
}

func (handler *Handler) Create(c echo.Context) error {
	var body struct {
		Email string `json:"email"`
	}
	if err := c.Bind(&body); err != nil {
		handler.log.Error("bind create account", "err", err)
		return echo.NewHTTPError(http.StatusBadRequest, "invalid json")
	}

	created, err := handler.accounts.Create(c.Request().Context(), body.Email)
	if err != nil {
		if message, ok := accountErrorMessage(err); ok {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": message})
		}
		handler.log.Error("create account", "err", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	return c.JSON(http.StatusCreated, created)
}

func (handler *Handler) Delete(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid account id")
	}

	if err := handler.accounts.Delete(c.Request().Context(), id); err != nil {
		if errors.Is(err, account.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "account not found"})
		}
		handler.log.Error("delete account", "err", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	return c.NoContent(http.StatusNoContent)
}

func accountsJSON(accounts []account.Account) string {
	if accounts == nil {
		accounts = []account.Account{}
	}
	data, err := json.Marshal(accounts)
	if err != nil {
		return "[]"
	}
	return string(data)
}

func accountErrorMessage(err error) (string, bool) {
	if errors.Is(err, account.ErrEmailRequired) {
		return "メールアドレスを入力してください。", true
	}
	if errors.Is(err, account.ErrEmailInvalid) {
		return "有効なメールアドレスを入力してください。", true
	}
	if errors.Is(err, account.ErrEmailExists) {
		return "このメールアドレスは登録済みです。", true
	}
	return "", false
}
