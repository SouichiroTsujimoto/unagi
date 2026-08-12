package terminal

import (
	"net/http"
	"os"
	"strings"
	"time"
)

// Dev reload hub contract between cmd/dev (listener) and the app server
// (notifier) / templ pages (script src). Kept in terminal so runtime code
// can honor it without importing cmd/dev.
const (
	DevReloadNotifyEnv = "UNIGO_DEV_RELOAD_NOTIFY"
	DevReloadScriptEnv = "UNIGO_DEV_RELOAD_SCRIPT"
)

// DevReloadScriptURL returns the development auto-reload script URL when set.
func DevReloadScriptURL() string {
	return strings.TrimSpace(os.Getenv(DevReloadScriptEnv))
}

// NotifyDevReload tells the development reload hub that listen succeeded.
// No-op when UNIGO_DEV_RELOAD_NOTIFY is unset (production / plain bin/server).
func NotifyDevReload() {
	url := strings.TrimSpace(os.Getenv(DevReloadNotifyEnv))
	if url == "" {
		return
	}
	// Keep this short and synchronous so Air's next cycle cannot race ahead of
	// the browser reload signal. Failure is non-fatal (hub may be restarting).
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Post(url, "text/plain", nil)
	if err != nil {
		return
	}
	_ = resp.Body.Close()
}
