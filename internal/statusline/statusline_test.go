package statusline

import (
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
	data.Workspace.Repo.Name = "kakadu-ai"
	data.ContextWindow.RemainingPercentage = &contextLeft
	data.RateLimits.FiveHour = &RateWindow{UsedPercentage: 36, ResetsAt: now.Add(time.Hour + 42*time.Minute).Unix()}
	data.RateLimits.SevenDay = &RateWindow{UsedPercentage: 16, ResetsAt: now.Add(3*24*time.Hour + 4*time.Hour).Unix()}
	data.Effort.Level = "xhigh"
	data.Thinking.Enabled = &thinking

	rendered, err := Render(data, Options{Theme: "mono", Columns: 160, Now: now, Branch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"Fable 5", "kakadu-ai", "git:main", "ctx:97% left",
		"5h:64% left · ↻1h42m", "7d:84% left · ↻3d4h",
		"effort:xhigh", "thinking:on",
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("status line missing %q: %q", expected, rendered)
		}
	}

	narrow, err := Render(data, Options{Theme: "mono", Columns: 48, Now: now, Branch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(narrow, "\n") {
		if width := lipgloss.Width(line); width > 48 {
			t.Fatalf("narrow line width = %d: %q", width, line)
		}
	}

	noColor, err := Render(data, Options{Theme: "claude", Columns: 160, Now: now, NoColor: true})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(noColor, "\x1b[") {
		t.Fatalf("NO_COLOR output contains ANSI escapes: %q", noColor)
	}
}
