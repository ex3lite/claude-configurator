package tui

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func nerdFontAvailable() bool {
	switch strings.ToLower(os.Getenv("CLAUDE_CONFIG_NERD_FONT")) {
	case "1", "true", "yes":
		return true
	case "0", "false", "no":
		return false
	}

	if path, err := exec.LookPath("fc-list"); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		output, runErr := exec.CommandContext(ctx, path, ":", "family").Output()
		cancel()
		if runErr == nil && isNerdFontName(string(output)) {
			return true
		}
	}

	home, _ := os.UserHomeDir()
	dirs := []string{
		filepath.Join(home, "Library", "Fonts"),
		filepath.Join(home, ".local", "share", "fonts"),
		"/Library/Fonts",
		"/usr/local/share/fonts",
		"/usr/share/fonts",
	}
	if runtime.GOOS == "windows" {
		dirs = []string{
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "Windows", "Fonts"),
			filepath.Join(os.Getenv("WINDIR"), "Fonts"),
		}
	}
	for _, root := range dirs {
		if root == "" {
			continue
		}
		found := false
		_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if !entry.IsDir() && isNerdFontName(entry.Name()) {
				found = true
				return fs.SkipAll
			}
			return nil
		})
		if found {
			return true
		}
	}
	return false
}

func isNerdFontName(name string) bool {
	name = strings.ToLower(name)
	if strings.Contains(name, "nerd font") || strings.Contains(name, "nerdfont") {
		return true
	}
	for _, field := range strings.FieldsFunc(name, func(r rune) bool {
		return r == ' ' || r == ',' || r == '-' || r == '_'
	}) {
		if field == "nf" || field == "nfm" || field == "nfp" {
			return true
		}
	}
	return false
}
