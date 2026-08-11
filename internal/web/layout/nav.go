package layout

import "github.com/SouichiroTsujimoto/unigo-template/internal/terminal"

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
