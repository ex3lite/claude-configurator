package tui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ex3lite/claude-configurator/internal/catalog"
	"github.com/ex3lite/claude-configurator/internal/config"
)

func TestStageAndSaveModel(t *testing.T) {
	m := testModel(t)
	press(m, special(tea.KeyEnter))
	press(m, special(tea.KeyEnter))
	selectChoice(t, m, "claude-fable-5[1m]")
	press(m, special(tea.KeyEnter))
	if got, _ := config.Get(m.drafts[config.Global], "model"); got != "claude-fable-5[1m]" {
		t.Fatalf("staged model = %v", got)
	}
	press(m, textKey("j"))
	press(m, special(tea.KeyEnter))
	selectChoice(t, m, "claude-sonnet-5")
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
	m.focus = 1
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
	for _, size := range [][2]int{{120, 30}, {90, 25}, {60, 20}, {48, 12}, {40, 10}} {
		m.width, m.height = size[0], size[1]
		content := m.View().Content
		if !strings.Contains(content, "Claude") && !strings.Contains(content, "CLAUDE") {
			t.Fatalf("%dx%d render missing title: %q", size[0], size[1], content)
		}
		if got := len(strings.Split(content, "\n")); got > size[1] {
			t.Fatalf("%dx%d rendered %d lines", size[0], size[1], got)
		}
		for _, line := range strings.Split(content, "\n") {
			if width := lipgloss.Width(line); width > size[0] {
				t.Fatalf("%dx%d rendered a %d-column line: %q", size[0], size[1], width, line)
			}
		}
	}
}

func TestStagedActionBarFitsNarrowTerminal(t *testing.T) {
	m := testModelWithSystemLanguage(t, "ru_RU.UTF-8")
	config.Set(m.drafts[config.Global], "model", "fable")
	for _, size := range [][2]int{{76, 24}, {60, 20}, {52, 18}} {
		m.width, m.height = size[0], size[1]
		content := m.View().Content
		for _, line := range strings.Split(content, "\n") {
			if width := lipgloss.Width(line); width > size[0] {
				t.Fatalf("%dx%d staged view has a %d-column line: %q", size[0], size[1], width, line)
			}
		}
	}
}

func TestPanelUsesRequestedDimensions(t *testing.T) {
	m := testModel(t)
	m.noColor = true
	panel := m.panel("TITLE", "body", 25, 18, true)
	if width := lipgloss.Width(panel); width != 25 {
		t.Fatalf("panel width = %d, want 25", width)
	}
	if height := lipgloss.Height(panel); height != 18 {
		t.Fatalf("panel height = %d, want 18", height)
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

func TestLocalizedRenderingFitsTerminal(t *testing.T) {
	for _, language := range []string{"ru_RU.UTF-8", "zh_CN.UTF-8"} {
		t.Run(language, func(t *testing.T) {
			m := testModelWithSystemLanguage(t, language)
			for _, size := range [][2]int{{120, 30}, {90, 25}, {64, 24}, {52, 18}} {
				m.width, m.height = size[0], size[1]
				for _, line := range strings.Split(m.View().Content, "\n") {
					if width := lipgloss.Width(line); width > size[0] {
						t.Fatalf("%dx%d rendered a %d-column line: %q", size[0], size[1], width, line)
					}
				}
			}
		})
	}
}

func TestEditorFitsEightyColumns(t *testing.T) {
	m := testModel(t)
	m.noColor = true
	m.width, m.height = 80, 24
	press(m, special(tea.KeyEnter))
	press(m, special(tea.KeyEnter))
	for _, line := range strings.Split(m.View().Content, "\n") {
		if width := lipgloss.Width(line); width > m.width {
			t.Fatalf("editor line width = %d, want <= %d: %q", width, m.width, line)
		}
	}
}

func TestNavigationDrillsIntoCategory(t *testing.T) {
	m := testModel(t)
	if m.focus != 0 {
		t.Fatalf("initial focus = %d, want main menu", m.focus)
	}
	press(m, special(tea.KeyRight))
	if m.focus != 0 || m.category != 0 {
		t.Fatal("right arrow changed the main menu selection")
	}
	press(m, special(tea.KeyDown))
	if m.category != 1 {
		t.Fatalf("category = %d, want 1", m.category)
	}
	press(m, special(tea.KeyEnter))
	if m.focus != 1 {
		t.Fatal("Enter did not open the selected category")
	}
	press(m, special(tea.KeyLeft))
	if m.focus != 0 || m.category != 1 {
		t.Fatal("left arrow did not return to the same main-menu category")
	}
}

func TestModelUsesChoicePickerAndExplicitCustomOption(t *testing.T) {
	m := testModel(t)
	press(m, special(tea.KeyEnter))
	press(m, special(tea.KeyEnter))
	if m.screen != editChoice {
		t.Fatalf("model editor screen = %v, want editChoice", m.screen)
	}
	if m.choiceOptions()[0] != inheritChoice ||
		!slices.Contains(m.choiceOptions(), "fable") ||
		!slices.Contains(m.choiceOptions(), "sonnet") ||
		!slices.Contains(m.choiceOptions(), "claude-fable-5[1m]") {
		t.Fatalf("model options = %#v", m.choiceOptions())
	}
	selectChoice(t, m, customChoice)
	m.width, m.height = 52, 18
	if content := m.View().Content; !strings.Contains(content, "Custom model ID") {
		t.Fatalf("selected custom option is hidden in a small terminal: %q", content)
	}
	press(m, special(tea.KeyEnter))
	if m.screen != editText {
		t.Fatalf("custom model screen = %v, want editText", m.screen)
	}
}

func TestSubagentPickerCanResetToInheritanceAndSelectFable(t *testing.T) {
	m := testModel(t)
	m.focus = 1
	m.selected = 1
	config.Set(m.drafts[config.Global], "env.CLAUDE_CODE_SUBAGENT_MODEL", "sonnet")

	press(m, special(tea.KeyEnter))
	if m.screen != editChoice {
		t.Fatalf("subagent editor screen = %v, want editChoice", m.screen)
	}
	if m.choiceOptions()[0] != inheritChoice || !slices.Contains(m.choiceOptions(), "fable") {
		t.Fatalf("subagent options = %#v", m.choiceOptions())
	}
	selectChoice(t, m, inheritChoice)
	press(m, special(tea.KeyEnter))
	if _, ok := config.Get(m.drafts[config.Global], "env.CLAUDE_CODE_SUBAGENT_MODEL"); ok {
		t.Fatal("default/inherit choice did not remove the scoped value")
	}

	press(m, special(tea.KeyEnter))
	selectChoice(t, m, "fable")
	press(m, special(tea.KeyEnter))
	if got, _ := config.Get(m.drafts[config.Global], "env.CLAUDE_CODE_SUBAGENT_MODEL"); got != "fable" {
		t.Fatalf("staged subagent model = %v, want fable", got)
	}
}

func TestThemeUsesBuiltInChoicePicker(t *testing.T) {
	m := testModel(t)
	themeDir := filepath.Join(filepath.Dir(m.workspace.Paths.Global), "themes")
	if err := os.MkdirAll(themeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(themeDir, "dracula.json"), []byte(`{"base":"dark"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	m.category = 5 // Interface
	m.focus = 1
	for i, spec := range m.visibleSpecs() {
		if spec.ID == "theme" {
			m.selected = i
			break
		}
	}
	press(m, special(tea.KeyEnter))
	if m.screen != editChoice {
		t.Fatalf("theme editor screen = %v, want editChoice", m.screen)
	}
	for _, option := range []string{inheritChoice, "auto", "dark", "light", "dark-daltonized", "light-ansi", "custom:dracula"} {
		if !slices.Contains(m.choiceOptions(), option) {
			t.Fatalf("theme options missing %q: %#v", option, m.choiceOptions())
		}
	}
	if slices.Contains(m.choiceOptions(), customChoice) {
		t.Fatalf("theme unexpectedly requires manual input: %#v", m.choiceOptions())
	}
	selectChoice(t, m, "dark")
	press(m, special(tea.KeyEnter))
	if got, _ := config.Get(m.drafts[config.Global], "theme"); got != "dark" {
		t.Fatalf("staged theme = %v, want dark", got)
	}
}

func TestFooterAlwaysShowsActionsAlongsideStatus(t *testing.T) {
	m := testModelWithSystemLanguage(t, "ru_RU.UTF-8")
	m.noColor = true
	m.status = "Проверочный статус"
	content := m.View().Content
	for _, text := range []string{"Сохранить", "Клавиши", "Проверочный статус"} {
		if !strings.Contains(content, text) {
			t.Fatalf("footer missing %q: %q", text, content)
		}
	}

	m.focus = 1
	content = m.View().Content
	for _, text := range []string{"Изменить", "Сбросить → наследовать", "Назад"} {
		if !strings.Contains(content, text) {
			t.Fatalf("settings footer missing %q: %q", text, content)
		}
	}
}

func TestDetailExplainsPurposeAndReset(t *testing.T) {
	m := testModelWithSystemLanguage(t, "ru_RU.UTF-8")
	m.noColor = true
	m.focus = 1
	config.Set(m.drafts[config.Global], "model", "fable")
	content := m.renderDetail(64)
	for _, text := range []string{"ЧТО ЭТО МЕНЯЕТ", "ЗАЧЕМ ЭТО НУЖНО", "Сбросить → наследовать"} {
		if !strings.Contains(content, text) {
			t.Fatalf("detail missing %q: %q", text, content)
		}
	}
}

func TestEverySettingHasLocalizedPurpose(t *testing.T) {
	for _, spec := range catalog.Specs {
		if spec.Purpose == "" {
			t.Errorf("%s has no English purpose", spec.ID)
		}
		for _, language := range []uiLanguage{languageRU, languageZH} {
			if translations[language]["spec."+spec.ID+".purpose"] == "" {
				t.Errorf("%s has no %s purpose", spec.ID, language)
			}
		}
	}
}

func TestFallbackModelsUseChoicePicker(t *testing.T) {
	m := testModel(t)
	m.focus = 1
	m.selected = 3
	press(m, special(tea.KeyEnter))
	if m.screen != editList {
		t.Fatalf("fallback editor screen = %v, want editList", m.screen)
	}
	press(m, textKey("a"))
	if m.screen != editChoice {
		t.Fatalf("fallback add screen = %v, want editChoice", m.screen)
	}
	selectChoice(t, m, "sonnet")
	press(m, special(tea.KeyEnter))
	items := m.ownList(m.editSpec)
	if len(items) != 1 || items[0] != "sonnet" {
		t.Fatalf("fallback models = %#v", items)
	}
}

func TestExistingProviderModelOpensAsCustom(t *testing.T) {
	m := testModel(t)
	m.focus = 1
	config.Set(m.drafts[config.Global], "model", "gateway/team-opus")
	press(m, special(tea.KeyEnter))
	if got := m.choiceOptions()[m.choice]; got != customChoice {
		t.Fatalf("selected choice = %q, want custom", got)
	}
	press(m, special(tea.KeyEnter))
	if got := string(m.input); got != "gateway/team-opus" {
		t.Fatalf("custom model input = %q", got)
	}
}

func TestAutoLanguageAndPersistedInterfaceChoice(t *testing.T) {
	m := testModelWithSystemLanguage(t, "ru_RU.UTF-8")
	if m.languageMode != languageAuto || m.language != languageRU {
		t.Fatalf("language mode/resolved = %q/%q", m.languageMode, m.language)
	}
	if content := m.View().Content; !strings.Contains(content, "ГЛАВНОЕ МЕНЮ") {
		t.Fatalf("auto-localized view is not Russian: %q", content)
	}

	m.category = 5 // Interface
	m.focus = 1
	for i, spec := range m.visibleSpecs() {
		if spec.ID == "ui-language" {
			m.selected = i
			break
		}
	}
	press(m, special(tea.KeyEnter))
	selectChoice(t, m, "zh-CN")
	press(m, special(tea.KeyEnter))
	if m.languageMode != languageZH || m.language != languageZH {
		t.Fatalf("selected language = %q/%q", m.languageMode, m.language)
	}
	raw, err := os.ReadFile(m.preferences)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"language": "zh-CN"`) {
		t.Fatalf("preferences = %s", raw)
	}
	if m.dirty(config.Global) {
		t.Fatal("interface language dirtied Claude Code settings")
	}
}

func TestCategoryMenuScrollsToSelection(t *testing.T) {
	m := testModel(t)
	m.width, m.height = 90, 25
	m.category = len(catalog.Categories()) - 1
	content := m.View().Content
	if !strings.Contains(content, "Behavior") {
		t.Fatalf("selected final category is not visible: %q", content)
	}
}

func testModel(t *testing.T) *Model {
	return testModelWithSystemLanguage(t, "en_US.UTF-8")
}

func testModelWithSystemLanguage(t *testing.T, language string) *Model {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	project := filepath.Join(root, "project")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "AppData", "Local"))
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANGUAGE", "")
	t.Setenv("LANG", language)
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

func selectChoice(t *testing.T, m *Model, value string) {
	t.Helper()
	index := slices.Index(m.choiceOptions(), value)
	if index < 0 {
		t.Fatalf("choice %q not found in %#v", value, m.choiceOptions())
	}
	m.choice = index
}
