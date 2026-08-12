package terminal

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestNotifyDevReloadPostsWhenEnvSet(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		hits.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	t.Setenv(DevReloadNotifyEnv, srv.URL)
	NotifyDevReload()

	if hits.Load() != 1 {
		t.Fatalf("notify hits = %d, want 1", hits.Load())
	}
}

func TestNotifyDevReloadNoopWithoutEnv(t *testing.T) {
	t.Setenv(DevReloadNotifyEnv, "")
	NotifyDevReload() // must not panic
}

func TestDevReloadScriptURL(t *testing.T) {
	t.Setenv(DevReloadScriptEnv, " http://127.0.0.1:8199/dev-reload.js ")
	if got := DevReloadScriptURL(); got != "http://127.0.0.1:8199/dev-reload.js" {
		t.Fatalf("DevReloadScriptURL = %q", got)
	}
}
