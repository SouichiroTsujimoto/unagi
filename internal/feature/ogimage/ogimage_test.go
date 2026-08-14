package ogimage

import (
	"bytes"
	"image"
	"image/png"
	"strings"
	"testing"

	"github.com/SouichiroTsujimoto/unagi/internal/braille"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/article"
	"golang.org/x/image/font"
)

func TestRenderPNG(t *testing.T) {
	images, err := New()
	if err != nil {
		t.Fatal(err)
	}
	item := article.Article{
		Slug:       "hello-unagi",
		RevisionID: 12,
		OGVersion:  3,
		Title:      "Kea DHCPのIPアドレスリース数のメトリクス取得が難しかった話",
		Emoji:      "🥞",
		Topics:     []string{"Go", "DHCP", "ネットワーク", "ignored"},
	}
	body, err := images.RenderPNG(item)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(body, []byte("\x89PNG\r\n\x1a\n")) {
		t.Fatal("missing PNG signature")
	}
	config, err := png.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if config.Width != Width || config.Height != Height {
		t.Fatalf("dimensions=%dx%d", config.Width, config.Height)
	}
	if got := Path(item); got != "/og/articles/hello-unagi/12-3-7.png" {
		t.Fatalf("path=%q", got)
	}
	if len(body) > 1_000_000 {
		t.Fatalf("OGP PNG is too large: %d bytes", len(body))
	}

	rendered, err := png.Decode(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	pattern := braille.NoiseGrid(item.Slug, 5, 5)
	offX, offY := patternPoint(t, pattern, false)
	onX, onY := patternPoint(t, pattern, true)
	assertPixel(t, rendered, 100, 100, darkBackground)
	assertPixel(t, rendered, offX, offY, darkBackground)
	assertPixel(t, rendered, onX, onY, accent)

	item.OGTemplate = article.OGTemplateEditorial
	body, err = images.RenderPNG(item)
	if err != nil {
		t.Fatal(err)
	}
	rendered, err = png.Decode(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	assertPixel(t, rendered, 100, 100, accent)
	assertPixel(t, rendered, offX, offY, accent)
	assertPixel(t, rendered, onX, onY, ink)
}

func TestRenderVeryLongTitle(t *testing.T) {
	images, err := New()
	if err != nil {
		t.Fatal(err)
	}
	item := article.Article{
		Title:  strings.Repeat("非常に長い記事タイトル", 20),
		Topics: []string{"Go"},
	}
	if _, err := images.RenderPNG(item); err != nil {
		t.Fatal(err)
	}
}

func TestWrapKeepsASCIIWordsTogether(t *testing.T) {
	images, err := New()
	if err != nil {
		t.Fatal(err)
	}
	face, err := images.face(44)
	if err != nil {
		t.Fatal(err)
	}
	defer face.Close()

	const prefix = "Vercel Go Runtime +"
	maxWidth := font.MeasureString(face, prefix+" Supa").Ceil()
	lines := wrap(face, "Vercel Go Runtime + Supabaseで、技術ブログを無料で動かす", maxWidth)
	if len(lines) < 2 {
		t.Fatalf("lines=%q", lines)
	}
	if lines[0] != prefix {
		t.Fatalf("first line=%q want %q", lines[0], prefix)
	}
	if !strings.HasPrefix(lines[1], "Supabase") {
		t.Fatalf("second line split Supabase: %q", lines[1])
	}
}

func assertPixel(t *testing.T, rendered image.Image, x, y int, want interface {
	RGBA() (uint32, uint32, uint32, uint32)
}) {
	t.Helper()
	gotR, gotG, gotB, gotA := rendered.At(x, y).RGBA()
	wantR, wantG, wantB, wantA := want.RGBA()
	if gotR != wantR || gotG != wantG || gotB != wantB || gotA != wantA {
		t.Fatalf("pixel (%d,%d)=(%d,%d,%d,%d) want (%d,%d,%d,%d)", x, y, gotR, gotG, gotB, gotA, wantR, wantG, wantB, wantA)
	}
}

func patternPoint(t *testing.T, pattern braille.DotGrid, want bool) (int, int) {
	t.Helper()
	for i, on := range pattern.On {
		if on == want {
			return 874 + (i%pattern.Cols)*58, 226 + (i/pattern.Cols)*58
		}
	}
	t.Fatalf("pattern has no dot with state %t", want)
	return 0, 0
}
