package layout

import "github.com/SouichiroTsujimoto/unagi/internal/terminal"

func devReloadScriptURL() string {
	return terminal.DevReloadScriptURL()
}

func navClass(active, name string) string {
	base := "text-[15px] transition-colors"
	if active == name {
		return base + " text-base-content font-medium"
	}
	return base + " text-base-content/45 hover:text-base-content"
}

func containerClass(wide bool) string {
	base := "mx-auto w-full px-6 pt-16 pb-24 sm:pt-20"
	if wide {
		return base + " max-w-2xl"
	}
	return base + " max-w-lg"
}

func ogType(v string) string {
	if v == "" {
		return "website"
	}
	return v
}
