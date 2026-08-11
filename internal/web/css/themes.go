package css

import (
	_ "embed"
	"regexp"
	"strings"
	"sync"
)

//go:embed styles.css
var stylesFile string

var (
	themesOnce sync.Once
	lightName  string
	darkName   string
)

var (
	themesLineRE = regexp.MustCompile(`(?m)^\s*themes:\s*([^;]+);`)
	themeTokenRE = regexp.MustCompile(`([A-Za-z0-9_-]+)((?:\s+--[A-Za-z0-9_-]+)*)`)
	nameRE       = regexp.MustCompile(`(?m)^\s*name:\s*"([^"]+)"\s*;`)
	defaultRE    = regexp.MustCompile(`(?m)^\s*default:\s*true\s*;`)
	prefersDarkRE = regexp.MustCompile(`(?m)^\s*prefersdark:\s*true\s*;`)
)

// ThemeLight is the daisyUI theme marked --default (or default: true).
func ThemeLight() string {
	loadThemes()
	return lightName
}

// ThemeDark is the daisyUI theme marked --prefersdark (or prefersdark: true).
func ThemeDark() string {
	loadThemes()
	return darkName
}

func loadThemes() {
	themesOnce.Do(func() {
		lightName, darkName = parseThemes(stylesFile)
	})
}

func parseThemes(src string) (light, dark string) {
	light, dark = "light", "dark"
	src = stripCSSComments(src)

	if m := themesLineRE.FindStringSubmatch(src); len(m) == 2 {
		for _, tok := range themeTokenRE.FindAllStringSubmatch(m[1], -1) {
			name, flags := tok[1], tok[2]
			if strings.Contains(flags, "--default") {
				light = name
			}
			if strings.Contains(flags, "--prefersdark") {
				dark = name
			}
		}
	}

	// Custom @plugin daisyui-theme blocks may set default / prefersdark.
	for _, block := range themePluginBlocks(src) {
		nm := nameRE.FindStringSubmatch(block)
		if len(nm) != 2 {
			continue
		}
		if defaultRE.MatchString(block) {
			light = nm[1]
		}
		if prefersDarkRE.MatchString(block) {
			dark = nm[1]
		}
	}
	return light, dark
}

func stripCSSComments(src string) string {
	var b strings.Builder
	b.Grow(len(src))
	for i := 0; i < len(src); {
		if i+1 < len(src) && src[i] == '/' && src[i+1] == '*' {
			j := strings.Index(src[i+2:], "*/")
			if j < 0 {
				break
			}
			i += 2 + j + 2
			b.WriteByte(' ')
			continue
		}
		b.WriteByte(src[i])
		i++
	}
	return b.String()
}

func themePluginBlocks(src string) []string {
	const marker = "daisyui-theme"
	var blocks []string
	for {
		i := strings.Index(src, marker)
		if i < 0 {
			break
		}
		rest := src[i:]
		open := strings.Index(rest, "{")
		if open < 0 {
			break
		}
		depth := 0
		end := -1
		for j := open; j < len(rest); j++ {
			switch rest[j] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					end = j
				}
			}
			if end >= 0 {
				break
			}
		}
		if end < 0 {
			break
		}
		blocks = append(blocks, rest[open:end+1])
		src = rest[end+1:]
	}
	return blocks
}
