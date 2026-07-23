package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ex3lite/claude-configurator/internal/config"
)

func TestStageAndSaveModel(t *testing.T) {
	m := testModel(t)
	press(m, special(tea.KeyEnter))
	typeText(m, "claude-fable-5[1m]")
	press(m, special(tea.KeyEnter))
	if got, _ := config.Get(m.drafts[config.Global], "model"); got != "claude-fable-5[1m]" {
		t.Fatalf("staged model = %v", got)
	}
	press(m, textKey("j"))
	press(m, special(tea.KeyEnter))
	typeText(m, "claude-sonnet-5")
	press(m, special(tea.KeyEnter))
	if got, _ := config.Get(m.drafts[config.Global], "env.CLAUDE_CODE_SUBAGENT_MODEL"); got != "claude-sonnet-5" {
		t.Fatalf("staged subagent model = %v", got)
	}
	if content := m.View().Content; !utf8.ValidString(content) {
		t.Fatal("staged render contains invalid UTF-8")
	}
	press(m, textKey("s"))
	if m.screen != showDiff {
		t.Fatalf("screen = %v, want showDiff", m.screen)
	}
	press(m, special(tea.KeyEnter))
	raw, err := os.ReadFile(m.workspace.Paths.Global)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"model": "claude-fable-5[1m]"`) {
		t.Fatalf("settings = %s", raw)
	}
	if !strings.Contains(string(raw), `"CLAUDE_CODE_SUBAGENT_MODEL": "claude-sonnet-5"`) {
		t.Fatalf("settings = %s", raw)
	}
	if m.dirty(config.Global) {
		t.Fatal("scope remained dirty after save")
	}
}

func TestDangerousToggleRequiresConfirmation(t *testing.T) {
	m := testModel(t)
	m.category = 4 // Safety
	m.selected = 0 // sandbox.enabled
	press(m, textKey(" "))
	if m.screen != browse {
		t.Fatalf("enabling sandbox opened confirmation")
	}
	press(m, textKey(" "))
	if m.screen != confirmDanger {
		t.Fatalf("screen = %v, want confirmDanger", m.screen)
	}
	press(m, textKey("n"))
	if got, _ := config.Get(m.drafts[config.Global], "sandbox.enabled"); got != true {
		t.Fatalf("cancel changed value to %v", got)
	}
}

func TestScopeSearchAndUnset(t *testing.T) {
	m := testModel(t)
	press(m, textKey("l"))
	if m.scope != config.Local {
		t.Fatalf("scope = %s", m.scope)
	}
	press(m, textKey("/"))
	typeText(m, "subagent")
	press(m, special(tea.KeyEnter))
	specs := m.visibleSpecs()
	if len(specs) != 1 || specs[0].Path != "env.CLAUDE_CODE_SUBAGENT_MODEL" {
		t.Fatalf("search results = %#v", specs)
	}
	config.Set(m.drafts[config.Local], specs[0].Path, "sonnet")
	press(m, textKey("u"))
	if _, ok := config.Get(m.drafts[config.Local], specs[0].Path); ok {
		t.Fatal("unset did not remove local value")
	}
}

func TestResponsiveRendering(t *testing.T) {
	m := testModel(t)
	for _, size := range [][2]int{{120, 30}, {90, 25}, {60, 20}, {40, 10}} {
		m.width, m.height = size[0], size[1]
		content := m.View().Content
		if !strings.Contains(content, "Claude") && !strings.Contains(content, "CLAUDE") {
			t.Fatalf("%dx%d render missing title: %q", size[0], size[1], content)
		}
		if got := len(strings.Split(content, "\n")); got > size[1] {
			t.Fatalf("%dx%d rendered %d lines", size[0], size[1], got)
		}
	}
}

func TestNoColorRendering(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	m := testModel(t)
	m.width, m.height = 90, 25
	if content := m.View().Content; strings.Contains(content, "\x1b[3") {
		t.Fatalf("NO_COLOR render contains ANSI color sequence: %q", content)
	}
}

func TestEditorFitsEightyColumns(t *testing.T) {
	m := testModel(t)
	m.noColor = true
	m.width, m.height = 80, 24
	press(m, special(tea.KeyEnter))
	for _, line := range strings.Split(m.View().Content, "\n") {
		if width := lipgloss.Width(line); width > m.width {
			t.Fatalf("editor line width = %d, want <= %d: %q", width, m.width, line)
		}
	}
}

func testModel(t *testing.T) *Model {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	project := filepath.Join(root, "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	workspace, err := config.LoadWorkspace(home, project)
	if err != nil {
		t.Fatal(err)
	}
	m := New(workspace, config.Global, "test")
	m.width, m.height = 120, 30
	return m
}

func press(m *Model, key tea.KeyPressMsg) {
	_, _ = m.Update(key)
}

func textKey(text string) tea.KeyPressMsg {
	runes := []rune(text)
	code := rune(0)
	if len(runes) == 1 {
		code = runes[0]
	}
	return tea.KeyPressMsg(tea.Key{Text: text, Code: code})
}

func special(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code})
}

func typeText(m *Model, text string) {
	press(m, textKey(text))
}
