package tui

import (
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"os"
	"slices"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ex3lite/claude-configurator/internal/catalog"
	"github.com/ex3lite/claude-configurator/internal/config"
)

type screen uint8

const (
	browse screen = iota
	editText
	editChoice
	editList
	search
	showDiff
	showHelp
	confirmDanger
	confirmQuit
	confirmReload
)

type inputPurpose uint8

const (
	settingValue inputPurpose = iota
	listItem
	searchValue
)

type Model struct {
	workspace *config.Workspace
	drafts    map[config.Scope]map[string]any
	scope     config.Scope
	version   string

	width, height int
	dark          bool
	noColor       bool
	focus         int // 0: main menu, 1: settings inside the selected category
	category      int
	selected      int
	query         string
	status        string
	screen        screen
	languageMode  uiLanguage
	language      uiLanguage
	preferences   string

	editSpec      catalog.Spec
	choice        int
	choiceForList bool
	listSelected  int
	input         []rune
	inputCursor   int
	inputPurpose  inputPurpose
	inputListEdit int
	returnScreen  screen

	pendingSpec  catalog.Spec
	pendingValue any
	dangerText   string
}

func New(workspace *config.Workspace, scope config.Scope, version string) *Model {
	languageMode, preferencesPath := loadLanguagePreference()
	m := &Model{
		workspace:    workspace,
		scope:        scope,
		version:      version,
		dark:         true,
		noColor:      os.Getenv("NO_COLOR") != "",
		screen:       browse,
		focus:        0,
		languageMode: languageMode,
		language:     resolveLanguage(languageMode),
		preferences:  preferencesPath,
		drafts:       make(map[config.Scope]map[string]any, 3),
	}
	m.resetDrafts()
	return m
}

func (m *Model) Init() tea.Cmd { return nil }

func (m *Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.BackgroundColorMsg:
		m.dark = msg.IsDark()
		return m, nil
	case tea.KeyPressMsg:
		if msg.Keystroke() == "ctrl+c" {
			if m.anyDirty() {
				m.screen = confirmQuit
				return m, nil
			}
			return m, tea.Quit
		}
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch m.screen {
	case editText, search:
		m.handleTextKey(msg)
	case editChoice:
		m.handleChoiceKey(msg)
	case editList:
		m.handleListKey(msg)
	case showDiff:
		m.handleDiffKey(msg)
	case showHelp:
		if msg.Key().Code == tea.KeyEscape || msg.String() == "?" || msg.String() == "q" {
			m.screen = browse
		}
	case confirmDanger:
		if isYes(msg) {
			m.setValue(m.pendingSpec, m.pendingValue)
			m.screen = browse
		} else if isNo(msg) {
			m.screen = browse
			m.status = m.tr("status.cancelled")
		}
	case confirmQuit:
		if isYes(msg) {
			return m, tea.Quit
		} else if isNo(msg) {
			m.screen = browse
		}
	case confirmReload:
		if isYes(msg) {
			m.reload()
		} else if isNo(msg) {
			m.screen = browse
		}
	default:
		return m.handleBrowseKey(msg)
	}
	return m, nil
}

func (m *Model) handleBrowseKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.Key().Code == tea.KeySpace {
		if m.focus == 1 {
			if spec, ok := m.currentSpec(); ok && spec.Kind == catalog.Boolean {
				m.toggle(spec)
			}
		}
		return m, nil
	}
	key := msg.String()
	switch key {
	case "q":
		if m.anyDirty() {
			m.screen = confirmQuit
			return m, nil
		}
		return m, tea.Quit
	case "?":
		m.screen = showHelp
	case "/":
		m.openInput(searchValue, catalog.Spec{}, m.query, browse)
		m.screen = search
	case "esc":
		if m.query != "" {
			m.query = ""
			m.selected = 0
		}
		m.focus = 0
	case "left":
		if m.focus == 1 {
			m.focus = 0
			m.selected = 0
		}
	case "g":
		m.switchScope(config.Global)
	case "p":
		m.switchScope(config.Project)
	case "l":
		m.switchScope(config.Local)
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "enter":
		if m.focus == 0 {
			m.focus = 1
			m.selected = 0
		} else {
			m.openEditor()
		}
	case "u", "backspace", "delete":
		if m.focus != 1 {
			break
		}
		if spec, ok := m.currentSpec(); ok {
			if spec.App {
				m.setLanguage(languageAuto)
				break
			}
			config.Unset(m.drafts[m.scope], spec.Path)
			m.status = m.tr("status.inherit", m.specLabel(spec.ID, spec.Label))
		}
	case "s":
		if !m.dirty(m.scope) {
			m.status = m.tr("status.nothing_to_save", m.scopeLabel(string(m.scope)))
		} else {
			m.screen = showDiff
		}
	case "r":
		if m.anyDirty() {
			m.screen = confirmReload
		} else {
			m.reload()
		}
	}
	return m, nil
}

func (m *Model) move(delta int) {
	if m.focus == 0 && m.query == "" {
		m.moveCategory(delta)
		return
	}
	items := m.visibleSpecs()
	if len(items) == 0 {
		m.selected = 0
		return
	}
	m.selected = (m.selected + delta + len(items)) % len(items)
}

func (m *Model) moveCategory(delta int) {
	if m.query != "" {
		return
	}
	categories := catalog.Categories()
	m.category = (m.category + delta + len(categories)) % len(categories)
	m.selected = 0
}

func (m *Model) switchScope(scope config.Scope) {
	m.scope = scope
	m.status = m.tr("status.scope", m.scopeLabel(string(scope)))
}

func (m *Model) openEditor() {
	spec, ok := m.currentSpec()
	if !ok {
		return
	}
	m.editSpec = spec
	switch spec.Kind {
	case catalog.Boolean:
		m.toggle(spec)
	case catalog.Enum:
		m.openChoice(spec, false, -1)
	case catalog.List:
		m.listSelected = 0
		m.screen = editList
	default:
		value := ""
		if current, _, ok := m.effective(spec); ok {
			value = fmt.Sprint(current)
		}
		m.openInput(settingValue, spec, value, browse)
	}
}

func (m *Model) openChoice(spec catalog.Spec, forList bool, listIndex int) {
	m.editSpec = spec
	m.choiceForList = forList
	m.inputListEdit = listIndex
	m.choice = 0
	var current any
	var ok bool
	if forList {
		items := m.ownList(spec)
		if listIndex >= 0 && listIndex < len(items) {
			current, ok = items[listIndex], true
		}
	} else {
		current, _, ok = m.effective(spec)
	}
	if ok {
		if index := slices.Index(spec.Options, fmt.Sprint(current)); index >= 0 {
			m.choice = index
		} else if spec.AllowCustom {
			m.choice = len(spec.Options)
		}
	}
	m.screen = editChoice
}

func (m *Model) choiceOptions() []string {
	options := append([]string(nil), m.editSpec.Options...)
	if m.editSpec.AllowCustom {
		options = append(options, customChoice)
	}
	return options
}

func (m *Model) toggle(spec catalog.Spec) {
	current := false
	if value, _, ok := m.effective(spec); ok {
		current, _ = value.(bool)
	}
	m.propose(spec, !current)
}

func (m *Model) handleChoiceKey(msg tea.KeyPressMsg) {
	options := m.choiceOptions()
	switch msg.String() {
	case "esc", "q":
		if m.choiceForList {
			m.screen = editList
		} else {
			m.screen = browse
		}
	case "up", "k":
		m.choice = (m.choice - 1 + len(options)) % len(options)
	case "down", "j":
		m.choice = (m.choice + 1) % len(options)
	case "enter", " ":
		option := options[m.choice]
		if option == customChoice {
			value := ""
			if m.choiceForList {
				items := m.ownList(m.editSpec)
				if m.inputListEdit >= 0 && m.inputListEdit < len(items) {
					value = fmt.Sprint(items[m.inputListEdit])
				}
				m.openInput(listItem, m.editSpec, value, editList)
			} else {
				if current, _, ok := m.effective(m.editSpec); ok &&
					slices.Index(m.editSpec.Options, fmt.Sprint(current)) < 0 {
					value = fmt.Sprint(current)
				}
				m.openInput(settingValue, m.editSpec, value, browse)
			}
			return
		}
		if m.choiceForList {
			m.commitListValue(option)
			return
		}
		m.propose(m.editSpec, option)
		if m.screen != confirmDanger {
			m.screen = browse
		}
	}
}

func (m *Model) handleListKey(msg tea.KeyPressMsg) {
	items := m.ownList(m.editSpec)
	switch msg.String() {
	case "esc", "q":
		m.screen = browse
	case "up", "k":
		if len(items) > 0 {
			m.listSelected = (m.listSelected - 1 + len(items)) % len(items)
		}
	case "down", "j":
		if len(items) > 0 {
			m.listSelected = (m.listSelected + 1) % len(items)
		}
	case "a":
		if m.editSpec.MaxItems > 0 && len(items) >= m.editSpec.MaxItems {
			m.status = m.tr("status.max_items", m.specLabel(m.editSpec.ID, m.editSpec.Label), m.editSpec.MaxItems)
			return
		}
		if len(m.editSpec.Options) > 0 {
			m.openChoice(m.editSpec, true, -1)
		} else {
			m.inputListEdit = -1
			m.openInput(listItem, m.editSpec, "", editList)
		}
	case "e", "enter":
		if len(items) > 0 {
			if len(m.editSpec.Options) > 0 {
				m.openChoice(m.editSpec, true, m.listSelected)
			} else {
				m.inputListEdit = m.listSelected
				m.openInput(listItem, m.editSpec, fmt.Sprint(items[m.listSelected]), editList)
			}
		}
	case "d", "delete", "backspace":
		if len(items) > 0 {
			items = append(items[:m.listSelected], items[m.listSelected+1:]...)
			config.Set(m.drafts[m.scope], m.editSpec.Path, items)
			if m.listSelected >= len(items) && m.listSelected > 0 {
				m.listSelected--
			}
			m.status = m.tr("status.item_removed")
		}
	}
}

func (m *Model) commitListValue(value string) {
	items := m.ownList(m.editSpec)
	for i, item := range items {
		if fmt.Sprint(item) == value && i != m.inputListEdit {
			m.status = m.tr("status.duplicate")
			m.screen = editList
			return
		}
	}
	if m.inputListEdit >= 0 && m.inputListEdit < len(items) {
		items[m.inputListEdit] = value
	} else {
		items = append(items, value)
		m.listSelected = len(items) - 1
	}
	config.Set(m.drafts[m.scope], m.editSpec.Path, items)
	m.status = m.tr("status.list_updated")
	m.screen = editList
}

func (m *Model) openInput(purpose inputPurpose, spec catalog.Spec, value string, returnTo screen) {
	m.inputPurpose = purpose
	m.editSpec = spec
	m.input = []rune(value)
	m.inputCursor = len(m.input)
	m.returnScreen = returnTo
	m.screen = editText
}

func (m *Model) handleTextKey(msg tea.KeyPressMsg) {
	key := msg.Key()
	switch key.Code {
	case tea.KeyEscape:
		if m.inputPurpose == searchValue {
			m.screen = browse
		} else {
			m.screen = m.returnScreen
		}
	case tea.KeyEnter:
		m.commitInput()
	case tea.KeyBackspace:
		if m.inputCursor > 0 {
			m.input = append(m.input[:m.inputCursor-1], m.input[m.inputCursor:]...)
			m.inputCursor--
		}
	case tea.KeyDelete:
		if m.inputCursor < len(m.input) {
			m.input = append(m.input[:m.inputCursor], m.input[m.inputCursor+1:]...)
		}
	case tea.KeyLeft:
		if m.inputCursor > 0 {
			m.inputCursor--
		}
	case tea.KeyRight:
		if m.inputCursor < len(m.input) {
			m.inputCursor++
		}
	case tea.KeyHome:
		m.inputCursor = 0
	case tea.KeyEnd:
		m.inputCursor = len(m.input)
	default:
		if key.Text != "" {
			runes := []rune(key.Text)
			m.input = append(m.input[:m.inputCursor], append(runes, m.input[m.inputCursor:]...)...)
			m.inputCursor += len(runes)
		}
	}
}

func (m *Model) commitInput() {
	value := strings.TrimSpace(string(m.input))
	if m.inputPurpose == searchValue {
		m.query = value
		m.selected = 0
		m.focus = 1
		m.screen = browse
		return
	}
	if value == "" {
		m.status = m.tr("status.empty")
		return
	}
	if m.inputPurpose == settingValue {
		m.propose(m.editSpec, value)
		if m.screen != confirmDanger {
			m.screen = browse
		}
		return
	}

	m.commitListValue(value)
}

func (m *Model) propose(spec catalog.Spec, value any) {
	if message, dangerous := spec.Dangerous(value); dangerous {
		m.pendingSpec = spec
		m.pendingValue = value
		m.dangerText = m.dangerMessage(spec, value, message)
		m.screen = confirmDanger
		return
	}
	m.setValue(spec, value)
}

func (m *Model) dangerMessage(spec catalog.Spec, value any, fallback string) string {
	if message := translations[m.language]["danger."+spec.ID+"."+fmt.Sprint(value)]; message != "" {
		return message
	}
	return fallback
}

func (m *Model) setValue(spec catalog.Spec, value any) {
	if spec.App {
		m.setLanguage(uiLanguage(fmt.Sprint(value)))
		return
	}
	config.Set(m.drafts[m.scope], spec.Path, value)
	m.status = m.tr("status.staged", m.specLabel(spec.ID, spec.Label))
}

func (m *Model) setLanguage(language uiLanguage) {
	if !validLanguage(language) {
		return
	}
	if err := saveLanguagePreference(m.preferences, language); err != nil {
		m.status = m.tr("app.language.save_failed", err)
		return
	}
	m.languageMode = language
	m.language = resolveLanguage(language)
	m.status = m.tr("app.language.saved")
}

func (m *Model) handleDiffKey(msg tea.KeyPressMsg) {
	switch msg.String() {
	case "esc", "n", "q":
		m.screen = browse
	case "enter", "y", "s":
		err := m.workspace.Save(m.scope, m.drafts[m.scope], "")
		if err != nil {
			if errors.Is(err, config.ErrConflict) {
				m.status = m.tr("status.conflict")
			} else {
				m.status = m.tr("status.save_failed", err)
			}
			m.screen = browse
			return
		}
		m.drafts[m.scope] = config.Clone(m.workspace.Docs[m.scope].Data)
		m.status = m.tr("status.saved")
		m.screen = browse
	}
}

func (m *Model) reload() {
	if err := m.workspace.Reload(); err != nil {
		m.status = m.tr("status.reload_failed", err)
		m.screen = browse
		return
	}
	m.resetDrafts()
	m.status = m.tr("status.reloaded")
	m.screen = browse
}

func (m *Model) resetDrafts() {
	for _, scope := range []config.Scope{config.Global, config.Project, config.Local} {
		m.drafts[scope] = config.Clone(m.workspace.Docs[scope].Data)
	}
}

func (m *Model) ownList(spec catalog.Spec) []any {
	value, ok := config.Get(m.drafts[m.scope], spec.Path)
	if !ok {
		return nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	return append([]any(nil), items...)
}

func (m *Model) effective(spec catalog.Spec) (any, string, bool) {
	if spec.App {
		return string(m.languageMode), m.tr("app.source"), true
	}
	scopes := []config.Scope{config.Global}
	if m.scope == config.Project || m.scope == config.Local {
		scopes = append(scopes, config.Project)
	}
	if m.scope == config.Local {
		scopes = append(scopes, config.Local)
	}
	merge := strings.HasPrefix(spec.Path, "permissions.") && spec.Kind == catalog.List
	if merge {
		var combined []any
		var sources []string
		seen := make(map[string]bool)
		for _, scope := range scopes {
			value, ok := config.Get(m.drafts[scope], spec.Path)
			if !ok {
				continue
			}
			items, ok := value.([]any)
			if !ok {
				continue
			}
			for _, item := range items {
				key := formatValue(item)
				if !seen[key] {
					combined = append(combined, item)
					seen[key] = true
				}
			}
			sources = append(sources, string(scope))
		}
		if len(sources) > 0 {
			return combined, strings.Join(sources, "+"), true
		}
		return nil, "", false
	}
	for i := len(scopes) - 1; i >= 0; i-- {
		if value, ok := config.Get(m.drafts[scopes[i]], spec.Path); ok {
			return value, string(scopes[i]), true
		}
	}
	return nil, "", false
}

func (m *Model) dirty(scope config.Scope) bool {
	return !config.Equal(m.drafts[scope], m.workspace.Docs[scope].Data)
}

func (m *Model) anyDirty() bool {
	return m.dirty(config.Global) || m.dirty(config.Project) || m.dirty(config.Local)
}

func (m *Model) currentSpec() (catalog.Spec, bool) {
	items := m.visibleSpecs()
	if len(items) == 0 {
		return catalog.Spec{}, false
	}
	if m.selected >= len(items) {
		m.selected = len(items) - 1
	}
	return items[m.selected], true
}

func (m *Model) visibleSpecs() []catalog.Spec {
	query := strings.ToLower(strings.TrimSpace(m.query))
	category := catalog.Categories()[m.category]
	var specs []catalog.Spec
	for _, spec := range catalog.Specs {
		if query == "" {
			if spec.Category == category {
				specs = append(specs, spec)
			}
			continue
		}
		haystack := strings.ToLower(
			spec.Label + " " + m.specLabel(spec.ID, spec.Label) + " " +
				spec.Path + " " + spec.Description + " " +
				m.specDescription(spec.ID, spec.Description),
		)
		if strings.Contains(haystack, query) {
			specs = append(specs, spec)
		}
	}
	return specs
}

func (m *Model) diffLines() []string {
	var lines []string
	for _, spec := range catalog.Specs {
		if spec.App {
			continue
		}
		old, oldOK := config.Get(m.workspace.Docs[m.scope].Data, spec.Path)
		next, nextOK := config.Get(m.drafts[m.scope], spec.Path)
		if oldOK == nextOK && config.Equal(old, next) {
			continue
		}
		if oldOK {
			lines = append(lines, "- "+spec.Path+" = "+formatValue(old))
		}
		if nextOK {
			lines = append(lines, "+ "+spec.Path+" = "+formatValue(next))
		} else {
			lines = append(lines, "+ "+spec.Path+" = <inherit>")
		}
	}
	return lines
}

func formatValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case []any:
		raw, _ := json.Marshal(typed)
		return string(raw)
	default:
		raw, _ := json.Marshal(typed)
		return string(raw)
	}
}

func isYes(msg tea.KeyPressMsg) bool {
	return msg.String() == "y" || msg.String() == "enter"
}

func isNo(msg tea.KeyPressMsg) bool {
	return msg.String() == "n" || msg.String() == "esc" || msg.String() == "q"
}

func (m *Model) View() tea.View {
	var content string
	if m.width < 48 || m.height < 16 {
		content = m.renderTiny()
	} else {
		switch m.screen {
		case editText, search:
			content = m.renderTextEditor()
		case editChoice:
			content = m.renderChoice()
		case editList:
			content = m.renderList()
		case showDiff:
			content = m.renderDiff()
		case showHelp:
			content = m.renderHelp()
		case confirmDanger:
			content = m.renderConfirm(m.tr("confirm.danger.title"), m.dangerText, m.tr("confirm.danger.help"), true)
		case confirmQuit:
			content = m.renderConfirm(m.tr("confirm.quit.title"), m.tr("confirm.quit.text"), m.tr("confirm.quit.help"), true)
		case confirmReload:
			content = m.renderConfirm(m.tr("confirm.reload.title"), m.tr("confirm.reload.text"), m.tr("confirm.reload.help"), true)
		default:
			content = m.renderBrowser()
		}
	}
	view := tea.NewView(limitLines(content, m.height))
	view.AltScreen = true
	view.WindowTitle = "Claude Configurator"
	return view
}

func (m *Model) renderTiny() string {
	title := m.style("#006D77", "#67E8F9").Bold(true).Render("Claude Configurator")
	return title + "\n\n" + m.tr("tiny.text") + "\n\n" + m.tr("tiny.quit")
}

func (m *Model) renderBrowser() string {
	header := m.renderHeader()
	footer := m.renderFooter()
	bodyHeight := max(8, m.height-lipgloss.Height(header)-lipgloss.Height(footer)-2)
	var body string
	switch {
	case m.focus == 0 && m.query == "":
		body = m.panel(
			m.tr("panel.categories"),
			m.renderCategoryMenu(m.width-4, bodyHeight-4),
			m.width,
			bodyHeight,
			true,
		)
	case m.width >= 78:
		settingsWidth := min(46, max(36, m.width*42/100))
		detailWidth := m.width - settingsWidth
		body = lipgloss.JoinHorizontal(lipgloss.Top,
			m.panel(m.tr("panel.settings"), m.renderSettings(settingsWidth-4, bodyHeight-4), settingsWidth, bodyHeight, true),
			m.panel(m.tr("panel.detail"), m.renderDetail(detailWidth-4), detailWidth, bodyHeight, false),
		)
	default:
		if bodyHeight < 18 {
			body = m.panel(
				m.tr("panel.settings"),
				m.renderSettings(m.width-4, bodyHeight-4),
				m.width,
				bodyHeight,
				true,
			)
		} else {
			listHeight := max(7, bodyHeight/2-1)
			body = m.panel(m.tr("panel.settings"), m.renderSettings(m.width-4, listHeight-4), m.width, listHeight, true) + "\n" +
				m.panel(m.tr("panel.detail"), m.renderDetail(m.width-4), m.width, bodyHeight-listHeight-1, false)
		}
	}
	return header + "\n" + body + "\n" + footer
}

func (m *Model) renderHeader() string {
	title := m.style("#006D77", "#67E8F9").Bold(true).Render("◆ CLAUDE CONFIG")
	versionText := m.version
	if versionText != "dev" && !strings.HasPrefix(versionText, "v") {
		versionText = "v" + versionText
	}
	version := m.muted().Render(" " + versionText)
	path := m.workspace.Paths.For(m.scope)
	dirty := ""
	if m.dirty(m.scope) {
		dirty = "  " + m.warning().Render(m.tr("header.staged"))
	}
	left := title + version + dirty
	right := m.muted().Render(truncate(path, max(10, m.width-lipgloss.Width(left)-2)))
	gap := max(1, m.width-lipgloss.Width(left)-lipgloss.Width(right))
	first := left + strings.Repeat(" ", gap) + right

	var scopes []string
	for _, item := range []struct {
		key   string
		scope config.Scope
	}{{"G", config.Global}, {"P", config.Project}, {"L", config.Local}} {
		label := item.key + " " + m.scopeLabel(string(item.scope))
		style := m.muted().Padding(0, 1)
		if item.scope == m.scope {
			style = m.scopeStyle().Padding(0, 1)
		}
		scopes = append(scopes, style.Render(label))
	}
	breadcrumb := m.renderBreadcrumb()
	scopeLine := strings.Join(scopes, " ")
	secondGap := max(1, m.width-lipgloss.Width(scopeLine)-lipgloss.Width(breadcrumb))
	second := scopeLine + strings.Repeat(" ", secondGap) + breadcrumb
	rule := m.muted().Render(strings.Repeat("─", max(1, m.width)))
	return first + "\n" + second + "\n" + rule
}

func (m *Model) renderFooter() string {
	keys := m.tr("footer.categories")
	if m.focus == 1 || m.query != "" {
		keys = m.tr("footer.settings")
	}
	if m.status != "" {
		keys = "  " + m.status
	}
	return m.muted().Width(max(1, m.width)).Render(truncate(keys, m.width))
}

func (m *Model) renderBreadcrumb() string {
	if m.query != "" {
		return m.style("#6D28D9", "#C4B5FD").Bold(true).Render(m.tr("breadcrumb.search", m.query))
	}
	menu := m.muted().Render(m.tr("breadcrumb.menu"))
	if m.focus == 0 {
		return menu
	}
	category := catalog.Categories()[m.category]
	return menu + m.muted().Render("  /  ") +
		m.accent().Bold(true).Render(m.categoryLabel(category))
}

func (m *Model) renderCategoryMenu(width, height int) string {
	var lines []string
	categories := catalog.Categories()
	stride := 3
	if width >= 76 {
		stride = 2
	}
	visible := max(1, height/stride)
	start := 0
	if m.category >= visible {
		start = m.category - visible + 1
	}
	end := min(len(categories), start+visible)
	for i := start; i < end; i++ {
		category := categories[i]
		marker := "◇"
		style := m.muted()
		if i == m.category {
			marker = "◆"
			style = m.selectedStyle()
		}
		title := fmt.Sprintf("%s  %s  ·  %s",
			marker,
			m.categoryLabel(category),
			m.tr("category.settings_count", m.categoryCount(category)),
		)
		lines = append(lines, style.Width(max(1, width)).Render(truncate(title, width)))
		lines = append(lines, m.muted().Render("   "+truncate(m.categoryDescription(category), max(4, width-3))))
		if stride == 3 {
			lines = append(lines, "")
		}
	}
	return fitLines(lines, height)
}

func (m *Model) categoryCount(category string) int {
	count := 0
	for _, spec := range catalog.Specs {
		if spec.Category == category {
			count++
		}
	}
	return count
}

func (m *Model) renderSettings(width, height int) string {
	specs := m.visibleSpecs()
	if len(specs) == 0 {
		return m.muted().Render(m.tr("empty.no_matches"))
	}
	start := 0
	if m.selected >= height {
		start = m.selected - height + 1
	}
	end := min(len(specs), start+height)
	var lines []string
	for i := start; i < end; i++ {
		spec := specs[i]
		value, source, ok := m.effective(spec)
		text := m.tr("value.inherit")
		if ok {
			text = m.compactValue(spec, value)
			if !spec.App && source != string(m.scope) {
				text += "  ↳"
			}
		}
		changed := " "
		if !spec.App {
			old, oldOK := config.Get(m.workspace.Docs[m.scope].Data, spec.Path)
			next, nextOK := config.Get(m.drafts[m.scope], spec.Path)
			if oldOK != nextOK || !config.Equal(old, next) {
				changed = "●"
			}
		}
		cursor := " "
		if i == m.selected {
			cursor = "›"
		}
		labelWidth := min(23, max(12, width/2))
		valueWidth := max(6, width-labelWidth-5)
		label := truncate(m.specLabel(spec.ID, spec.Label), labelWidth)
		label = lipgloss.NewStyle().Width(labelWidth).Render(label)
		line := cursor + changed + " " + label + " " + truncate(text, valueWidth)
		style := m.muted()
		if i == m.selected {
			style = m.selectedStyle()
		} else if changed == "●" {
			style = m.warning()
		}
		lines = append(lines, style.Width(max(1, width)).Render(truncate(line, width)))
	}
	return fitLines(lines, height)
}

func (m *Model) renderDetail(width int) string {
	spec, ok := m.currentSpec()
	if !ok {
		return m.tr("empty.no_matches")
	}
	var own any
	var ownOK bool
	if spec.App {
		own, ownOK = string(m.languageMode), true
	} else {
		own, ownOK = config.Get(m.drafts[m.scope], spec.Path)
	}
	effective, source, effectiveOK := m.effective(spec)
	target := m.tr("value.inherit")
	if ownOK {
		target = m.displayValue(spec, own)
	}
	resolved := m.tr("value.claude_default")
	if effectiveOK {
		resolved = m.displayValue(spec, effective)
	}
	if spec.App && m.languageMode == languageAuto {
		resolved += "\n" + m.tr("app.language.resolved", m.optionLabel(spec.ID, string(m.language)))
	}
	path := spec.Path
	if spec.App {
		path = "claude-configurator/config.json"
	}
	lines := []string{
		m.accent().Bold(true).Render(m.specLabel(spec.ID, spec.Label)),
		m.muted().Render(path),
		"",
		m.muted().Render(m.tr("detail.this_scope")),
		wrap(target, width),
		"",
		m.muted().Render(m.tr("detail.effective")),
		wrap(resolved, width),
	}
	if source != "" {
		lines = append(lines, m.muted().Render(m.tr("detail.source", m.sourceLabel(source))))
	}
	lines = append(lines, "", wrap(m.specDescription(spec.ID, spec.Description), width))
	if len(spec.Options) > 0 {
		var options []string
		for _, option := range spec.Options {
			options = append(options, m.optionLabel(spec.ID, option))
		}
		lines = append(lines, "", m.muted().Render(wrap(m.tr("detail.suggestions", strings.Join(options, ", ")), width)))
	}
	if spec.Kind == catalog.List && strings.HasPrefix(spec.Path, "permissions.") {
		lines = append(lines, "", m.muted().Render(m.tr("detail.permissions_merge")))
	}
	if spec.App {
		lines = append(lines, "", m.muted().Render(m.tr("detail.app_storage", m.preferences)))
	}
	return strings.Join(lines, "\n")
}

func (m *Model) displayValue(spec catalog.Spec, value any) string {
	switch typed := value.(type) {
	case string:
		return m.optionLabel(spec.ID, typed)
	case bool:
		if typed {
			return m.tr("value.on")
		}
		return m.tr("value.off")
	default:
		return formatValue(value)
	}
}

func (m *Model) compactValue(spec catalog.Spec, value any) string {
	if items, ok := value.([]any); ok {
		return m.tr("value.items", len(items))
	}
	return truncate(m.displayValue(spec, value), 18)
}

func (m *Model) sourceLabel(source string) string {
	parts := strings.Split(source, "+")
	for i, part := range parts {
		switch part {
		case string(config.Global), string(config.Project), string(config.Local):
			parts[i] = m.scopeLabel(part)
		}
	}
	return strings.Join(parts, "+")
}

func (m *Model) renderTextEditor() string {
	title := m.specLabel(m.editSpec.ID, m.editSpec.Label)
	description := m.specDescription(m.editSpec.ID, m.editSpec.Description)
	if m.inputPurpose == searchValue {
		title = m.tr("search.title")
		description = m.tr("search.description")
	} else if m.editSpec.AllowCustom {
		title = m.tr("editor.custom_model")
	} else if m.inputPurpose == listItem {
		title = m.tr("editor.edit", m.specLabel(m.editSpec.ID, m.editSpec.Label))
	}
	cursor := m.inputCursor
	before := string(m.input[:cursor])
	after := string(m.input[cursor:])
	value := before + m.accent().Reverse(true).Render(" ") + after
	body := wrap(description, m.modalContentWidth()) + "\n\n" +
		m.inputStyle().Render(value) + "\n\n" +
		m.muted().Render(m.tr("editor.apply"))
	if len(m.editSpec.Options) > 0 && m.inputPurpose == settingValue {
		var options []string
		for _, option := range m.editSpec.Options {
			options = append(options, m.optionLabel(m.editSpec.ID, option))
		}
		body += "\n" + m.muted().Render(
			wrap(m.tr("detail.suggestions", strings.Join(options, ", ")), m.modalContentWidth()),
		)
	}
	return m.modal(title, body, false)
}

func (m *Model) renderChoice() string {
	options := m.choiceOptions()
	var lines []string
	visible := max(3, m.height-10)
	start := 0
	if m.choice >= visible {
		start = m.choice - visible + 1
	}
	end := min(len(options), start+visible)
	for i := start; i < end; i++ {
		option := options[i]
		style := m.muted()
		prefix := "  "
		if i == m.choice {
			style = m.selectedStyle()
			prefix = "› "
		}
		label := m.optionLabel(m.editSpec.ID, option)
		raw := option
		if option == customChoice || raw == label {
			raw = ""
		}
		line := prefix + label
		if raw != "" {
			line += "  ·  " + raw
		}
		lines = append(lines, style.Width(max(1, m.modalContentWidth())).Render(
			truncate(line, m.modalContentWidth()),
		))
	}
	body := wrap(m.specDescription(m.editSpec.ID, m.editSpec.Description), m.modalContentWidth()) + "\n\n" +
		strings.Join(lines, "\n") + "\n\n" +
		m.muted().Render(m.tr("choice.help"))
	return m.modal(m.specLabel(m.editSpec.ID, m.editSpec.Label), body, false)
}

func (m *Model) renderList() string {
	items := m.ownList(m.editSpec)
	var lines []string
	if len(items) == 0 {
		lines = append(lines, m.muted().Render(m.tr("list.empty")))
	} else {
		for i, item := range items {
			style := m.muted()
			prefix := "  "
			if i == m.listSelected {
				style = m.selectedStyle()
				prefix = "› "
			}
			value := m.optionLabel(m.editSpec.ID, fmt.Sprint(item))
			lines = append(lines, style.Render(prefix+value))
		}
	}
	body := wrap(m.specDescription(m.editSpec.ID, m.editSpec.Description), m.modalContentWidth()) + "\n\n" +
		strings.Join(lines, "\n") + "\n\n" +
		m.muted().Render(m.tr("list.help"))
	if m.editSpec.MaxItems > 0 {
		body += "\n" + m.muted().Render(m.tr("list.maximum", m.editSpec.MaxItems))
	}
	return m.modal(m.specLabel(m.editSpec.ID, m.editSpec.Label), body, false)
}

func (m *Model) renderDiff() string {
	lines := m.diffLines()
	limit := max(3, m.height-13)
	hidden := 0
	if len(lines) > limit {
		hidden = len(lines) - limit
		lines = lines[:limit]
	}
	var rendered []string
	for _, line := range lines {
		style := m.muted()
		if strings.HasPrefix(line, "+") {
			style = m.success()
		} else if strings.HasPrefix(line, "-") {
			style = m.danger()
		}
		rendered = append(rendered, style.Render(wrap(line, m.modalContentWidth())))
	}
	if hidden > 0 {
		rendered = append(rendered, m.muted().Render(m.tr("diff.more", hidden)))
	}
	body := strings.Join(rendered, "\n") + "\n\n" +
		m.muted().Render(m.tr("diff.help")) + "\n" +
		m.muted().Render(m.workspace.Paths.For(m.scope))
	return m.modal(m.tr("diff.title", m.scopeLabel(string(m.scope))), body, false)
}

func (m *Model) renderHelp() string {
	return m.modal(
		m.tr("help.title"),
		m.helpText()+"\n\n"+m.muted().Render(m.tr("help.close")),
		false,
	)
}

func (m *Model) renderConfirm(title, text, footer string, dangerous bool) string {
	body := wrap(text, m.modalContentWidth()) + "\n\n" + m.muted().Render(footer)
	return m.modal(title, body, dangerous)
}

func (m *Model) modal(title, body string, dangerous bool) string {
	width := m.modalWidth()
	style := m.panelStyle(false)
	if dangerous {
		style = style.BorderForeground(m.color("#B45309", "#F59E0B"))
		title = m.warning().Bold(true).Render(title)
	} else {
		title = m.accent().Bold(true).Render(title)
	}
	box := style.Width(width - 2).Render(title + "\n\n" + body)
	top := max(0, (m.height-lipgloss.Height(box))/3)
	left := max(0, (m.width-lipgloss.Width(box))/2)
	return strings.Repeat("\n", top) + lipgloss.NewStyle().MarginLeft(left).Render(box)
}

func (m *Model) panel(title, body string, width, height int, active bool) string {
	titleStyle := m.muted().Bold(true)
	if active {
		titleStyle = m.accent().Bold(true)
	}
	body = limitLines(body, max(0, height-4))
	return m.panelStyle(active).
		Width(max(4, width)).
		Height(max(4, height)).
		Render(titleStyle.Render(title) + "\n\n" + body)
}

func (m *Model) panelStyle(active bool) lipgloss.Style {
	style := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	if active {
		return style.BorderForeground(m.color("#006D77", "#22D3EE"))
	}
	return style.BorderForeground(m.color("#CBD5E1", "#475569"))
}

func (m *Model) inputStyle() lipgloss.Style {
	return m.panelStyle(true).Width(max(24, m.modalContentWidth()-2)).Padding(0, 1)
}

func (m *Model) modalWidth() int {
	return min(88, max(44, m.width-8))
}

func (m *Model) modalContentWidth() int {
	return max(36, m.modalWidth()-4)
}

func (m *Model) selectedStyle() lipgloss.Style {
	style := m.style("#5B21B6", "#DDD6FE").Bold(true)
	if !m.noColor {
		style = style.Background(m.color("#EDE9FE", "#4C1D95"))
	}
	return style
}

func (m *Model) scopeStyle() lipgloss.Style {
	style := m.style("#0F766E", "#A5F3FC").Bold(true)
	if !m.noColor {
		style = style.Background(m.color("#CCFBF1", "#164E63"))
	}
	return style
}

func (m *Model) accent() lipgloss.Style {
	return m.style("#006D77", "#67E8F9")
}

func (m *Model) muted() lipgloss.Style {
	return m.style("#475569", "#94A3B8")
}

func (m *Model) warning() lipgloss.Style {
	return m.style("#B45309", "#FBBF24")
}

func (m *Model) danger() lipgloss.Style {
	return m.style("#B91C1C", "#FB7185")
}

func (m *Model) success() lipgloss.Style {
	return m.style("#047857", "#34D399")
}

func (m *Model) style(light, dark string) lipgloss.Style {
	style := lipgloss.NewStyle()
	if m.noColor {
		return style
	}
	return style.Foreground(lipgloss.LightDark(m.dark)(lipgloss.Color(light), lipgloss.Color(dark)))
}

func (m *Model) color(light, dark string) color.Color {
	if m.noColor {
		return nil
	}
	return lipgloss.LightDark(m.dark)(lipgloss.Color(light), lipgloss.Color(dark))
}

func truncate(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(text) <= width {
		return text
	}
	var out strings.Builder
	for len(text) > 0 {
		r, size := utf8.DecodeRuneInString(text)
		if lipgloss.Width(out.String()+string(r)+"…") > width {
			break
		}
		out.WriteRune(r)
		text = text[size:]
	}
	return out.String() + "…"
}

func wrap(text string, width int) string {
	if width < 8 {
		return truncate(text, width)
	}
	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		if paragraph == "" {
			lines = append(lines, "")
			continue
		}
		var line string
		for _, word := range strings.Fields(paragraph) {
			if line == "" {
				line = word
				continue
			}
			if lipgloss.Width(line)+1+lipgloss.Width(word) <= width {
				line += " " + word
			} else {
				lines = append(lines, truncate(line, width))
				line = word
			}
		}
		lines = append(lines, truncate(line, width))
	}
	return strings.Join(lines, "\n")
}

func fitLines(lines []string, height int) string {
	if height <= 0 {
		return ""
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func limitLines(text string, height int) string {
	if height <= 0 {
		return ""
	}
	lines := strings.Split(text, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}
