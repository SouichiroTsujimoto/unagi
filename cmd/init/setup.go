package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

const skeletonModule = "github.com/SouichiroTsujimoto/unagi"

type options struct {
	Root string

	// Non-interactive overrides (tests / scripting).
	Module  string
	SkipUI  bool
	SkipCSS bool
}

type answers struct {
	Module string
}

func run(opts options) error {
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return err
	}

	current, err := readModulePath(filepath.Join(root, "go.mod"))
	if err != nil {
		return err
	}

	ans := answers{Module: current}
	if opts.Module != "" {
		ans.Module = opts.Module
	}

	if !opts.SkipUI {
		if err := prompt(&ans); err != nil {
			return err
		}
	}

	ans.Module = strings.TrimSpace(ans.Module)
	if ans.Module == "" {
		return fmt.Errorf("module path is required")
	}

	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	fmt.Println(title.Render("unigo-template init"))

	if current != ans.Module {
		if err := renameModule(root, current, ans.Module); err != nil {
			return fmt.Errorf("rename module: %w", err)
		}
		fmt.Printf("Module: %s → %s\n", current, ans.Module)
	} else {
		fmt.Printf("Module: %s\n", ans.Module)
	}

	if !opts.SkipCSS {
		installed, err := installTailwindTools(root)
		if err != nil {
			return err
		}
		if installed {
			fmt.Println("CSS: Tailwind CSS + daisyUI (installed into tools/)")
		} else {
			fmt.Println("CSS: Tailwind CSS + daisyUI (tools/ already present; skipped install)")
		}
	}

	// Regenerate *_templ.go before tidy so imports match the new module path.
	if err := runIn(root, "go", "tool", "templ", "generate", "-path", "internal/web"); err != nil {
		return err
	}
	if err := runIn(root, "go", "mod", "tidy"); err != nil {
		return err
	}
	if !opts.SkipCSS {
		if err := runIn(root, "just", "css"); err != nil {
			return err
		}
	}

	fmt.Println("Done. Next: just run")
	return nil
}

func prompt(ans *answers) error {
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Go module path").
				Description("Set the module path for this app (used in go.mod and imports).").
				Placeholder(skeletonModule).
				Value(&ans.Module).
				Validate(func(v string) error {
					v = strings.TrimSpace(v)
					if v == "" {
						return fmt.Errorf("module path is required")
					}
					if strings.ContainsAny(v, " \t") {
						return fmt.Errorf("module path must not contain spaces")
					}
					return nil
				}),
		),
	)
	return form.Run()
}

// installTailwindTools installs Standalone CLI into tools/.
// Returns true when a fresh install ran, false when tools/tailwindcss already existed.
func installTailwindTools(root string) (bool, error) {
	bin := filepath.Join(root, "tools", "tailwindcss")
	if info, err := os.Stat(bin); err == nil && !info.IsDir() && info.Mode().Perm()&0o111 != 0 {
		return false, nil
	}

	cmd := exec.Command("bash", "-lc", "curl -sL daisyui.com/fast | bash -s -- tools && rm -f tools/input.css tools/output.css")
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("install tailwind/daisyui tools: %w", err)
	}
	return true, nil
}

func runIn(root, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}
