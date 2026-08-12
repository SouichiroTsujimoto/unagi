package css

import "testing"

func TestParseThemesBuiltInFlags(t *testing.T) {
	t.Parallel()

	src := `
@plugin "../../../tools/daisyui.mjs" {
	themes: cyberpunk --default, black --prefersdark;
	logs: false;
}
`
	light, dark := parseThemes(src)
	if light != "cyberpunk" || dark != "black" {
		t.Fatalf("got light=%q dark=%q", light, dark)
	}
}

func TestParseThemesCustomBlocks(t *testing.T) {
	t.Parallel()

	src := `
@plugin "../../../tools/daisyui.mjs" {
	themes: false;
}
@plugin "../../../tools/daisyui-theme.mjs" {
	name: "brand-light";
	default: true;
	color-scheme: light;
}
@plugin "../../../tools/daisyui-theme.mjs" {
	name: "brand-dark";
	prefersdark: true;
	color-scheme: dark;
}
`
	light, dark := parseThemes(src)
	if light != "brand-light" || dark != "brand-dark" {
		t.Fatalf("got light=%q dark=%q", light, dark)
	}
}

func TestParseThemesIgnoresCommentExamples(t *testing.T) {
	t.Parallel()

	src := `
/* Example: themes: cyberpunk --default, black --prefersdark; */
@plugin "../../../tools/daisyui.mjs" {
	themes: light --default, dark --prefersdark;
	logs: false;
}
`
	light, dark := parseThemes(src)
	if light != "light" || dark != "dark" {
		t.Fatalf("got light=%q dark=%q (comment example must be ignored)", light, dark)
	}
}

func TestEmbeddedStylesThemes(t *testing.T) {
	t.Parallel()

	if ThemeLight() == "" || ThemeDark() == "" {
		t.Fatal("expected non-empty theme names from embedded styles.css")
	}
}
