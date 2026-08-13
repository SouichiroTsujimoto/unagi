package web

import "testing"

func TestStaticCacheControl(t *testing.T) {
	t.Parallel()

	if got := staticCacheControl("wuhu1sland-1.webp"); got != staticImageCacheControl {
		t.Fatalf("webp=%q", got)
	}
	if got := staticCacheControl("vendor/foo.png"); got != staticImageCacheControl {
		t.Fatalf("png=%q", got)
	}
	if got := staticCacheControl("app.css"); got != "" {
		t.Fatalf("css should not be long-cached, got %q", got)
	}
	if got := staticCacheControl("article-share.js"); got != "" {
		t.Fatalf("js should not be long-cached, got %q", got)
	}
}
