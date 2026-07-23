//go:build darwin

package tui

import (
	"os/exec"
	"strings"
)

func platformLocale() string {
	out, err := exec.Command("/usr/bin/defaults", "read", "-g", "AppleLocale").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
