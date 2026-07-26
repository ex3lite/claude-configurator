package statusline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
)

func TestRenderLimitsAndResponsiveLayout(t *testing.T) {
	now := time.Date(2026, 7, 26, 10, 0, 0, 0, time.UTC)
	contextLeft, thinking := 97.0, true
	var data Data
	data.Model.DisplayName = "Fable 5"
	data.ConfiguredModels = ModelRoles{Subagent: "claude-sonnet-5", Advisor: "opus"}
	data.Workspace.Repo.Name = "demo-project"
	data.ContextWindow.RemainingPercentage = &contextLeft
	data.RateLimits.FiveHour = &RateWindow{UsedPercentage: 36, ResetsAt: now.Add(time.Hour + 42*time.Minute).Unix()}
	data.RateLimits.SevenDay = &RateWindow{UsedPercentage: 16, ResetsAt: now.Add(3*24*time.Hour + 4*time.Hour).Unix()}
	data.Effort.Level = "xhigh"
	data.Thinking.Enabled = &thinking

	rendered, err := Render(data, Options{Theme: "mono", Columns: 160, Now: now, Branch: "main", Language: "en"})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"Main: Fable 5", "Subagents: Sonnet 5", "Advisor: Opus",
		"demo-project", "git:main", "ctx:97% left",
		"5h", "▓▓▓▓░░░░░░", "36% used · 64% left",
		"resets today, 11:42 · in 1h 42m",
		"7d", "▓▓░░░░░░░░", "16% used · 84% left",
		"resets Jul 29, 14:00 · in 3d 4h",
		"effort:xhigh", "thinking:on",
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("status line missing %q: %q", expected, rendered)
		}
	}

	narrow, err := Render(data, Options{Theme: "mono", Columns: 48, Now: now, Branch: "main", Language: "en"})
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(narrow, "\n") {
		if width := lipgloss.Width(line); width > 48 {
			t.Fatalf("narrow line width = %d: %q", width, line)
		}
	}

	noColor, err := Render(data, Options{Theme: "claude", Columns: 160, Now: now, NoColor: true, Language: "en"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(noColor, "\x1b[") {
		t.Fatalf("NO_COLOR output contains ANSI escapes: %q", noColor)
	}

	var unavailable Data
	unavailable.Model.DisplayName = "Fable 5"
	unavailable.RateLimits.FiveHour = &RateWindow{UsedPercentage: 36}
	missing, err := Render(unavailable, Options{Theme: "mono", Columns: 100, Now: now, Language: "en"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(missing, "available after the first Pro/Max response") {
		t.Fatalf("missing limits are not explained: %q", missing)
	}

	irkutsk := time.FixedZone("IRKT", 8*60*60)
	localNow := time.Date(2026, 7, 26, 10, 0, 0, 0, irkutsk)
	data.RateLimits.FiveHour.ResetsAt = localNow.Add(time.Hour + 42*time.Minute).Unix()
	localized, err := Render(data, Options{
		Theme: "nerd", Columns: 200, Now: localNow, Branch: "main", Language: "ru", NoColor: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"󰚩 Основная: Fable 5", "Сабагенты: Sonnet 5", "Advisor: Opus",
		" main", " 5h",
		"36% исп. · 64% ост.",
		"сброс сегодня, 11:42 · через 1 ч 42 мин",
	} {
		if !strings.Contains(localized, expected) {
			t.Fatalf("localized status line missing %q: %q", expected, localized)
		}
	}
}

func TestConfiguredModelsFollowScopePrecedence(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_STATUSLINE_SUBAGENT_MODEL", "")
	t.Setenv("CLAUDE_CONFIG_STATUSLINE_ADVISOR_MODEL", "")
	t.Setenv("CLAUDE_CODE_SUBAGENT_MODEL", "")

	writeSettings := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeSettings(filepath.Join(home, ".claude", "settings.json"),
		`{"env":{"CLAUDE_CODE_SUBAGENT_MODEL":"haiku"},"advisorModel":"sonnet"}`)
	writeSettings(filepath.Join(project, ".claude", "settings.json"),
		`{"env":{"CLAUDE_CODE_SUBAGENT_MODEL":"claude-sonnet-5"},"advisorModel":"opus"}`)

	roles := configuredModels(home, project)
	if roles.Subagent != "claude-sonnet-5" || roles.Advisor != "opus" {
		t.Fatalf("configured models = %#v", roles)
	}
}

func TestModelRolesShowClaudeDefault(t *testing.T) {
	var data Data
	data.Model.DisplayName = "Fable 5"
	rendered, err := Render(data, Options{Theme: "mono", Columns: 100, Language: "ru"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered, "Сабагенты: По умолчанию Claude") ||
		!strings.Contains(rendered, "Advisor: По умолчанию Claude") {
		t.Fatalf("default model roles are not explicit: %q", rendered)
	}
}
