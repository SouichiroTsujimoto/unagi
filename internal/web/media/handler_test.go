package media_test

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/SouichiroTsujimoto/unagi/internal/db"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/media"
	webmedia "github.com/SouichiroTsujimoto/unagi/internal/web/media"
	"github.com/labstack/echo/v4"
)

func TestHandlerSignAndComplete(t *testing.T) {
	database := db.OpenTest(t)
	store, err := media.NewLocalStore(filepath.Join(t.TempDir(), "objects"))
	if err != nil {
		t.Fatal(err)
	}
	lib := media.New(database, store)
	h := webmedia.New(lib, slog.New(slog.NewTextHandler(io.Discard, nil)))
	e := echo.New()

	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/admin/media/sign", strings.NewReader(`{"filename":"dot.png","contentType":"image/png","sizeBytes":`+strconv.Itoa(buf.Len())+`}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	if err := h.SignUpload(e.NewContext(req, rec)); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("sign status=%d body=%s", rec.Code, rec.Body.String())
	}
	var signed struct {
		ObjectKey string `json:"objectKey"`
		SignedURL string `json:"signedUrl"`
		URL       string `json:"url"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &signed); err != nil {
		t.Fatal(err)
	}
	if signed.ObjectKey == "" || signed.URL != "/images/"+signed.ObjectKey {
		t.Fatalf("signed=%+v", signed)
	}

	if err := store.Put(req.Context(), signed.ObjectKey, bytes.NewReader(buf.Bytes()), "image/png", int64(buf.Len())); err != nil {
		t.Fatal(err)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/admin/media/complete", strings.NewReader(`{"objectKey":"`+signed.ObjectKey+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	if err := h.CompleteUpload(e.NewContext(req, rec)); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("complete status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/admin/media/sign", strings.NewReader(`{"filename":"x.txt","contentType":"text/plain","sizeBytes":4}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	err = h.SignUpload(e.NewContext(req, rec))
	httpErr, ok := err.(*echo.HTTPError)
	if !ok || httpErr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %#v", err)
	}
}
