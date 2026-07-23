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
	focus         int
	category      int
	selected      int
	query         string
	status        string
	screen        screen

	editSpec      catalog.Spec
	choice        int
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
	m := &Model{
		workspace: workspace,
		scope:     scope,
		version:   version,
		dark:      true,
		noColor:   os.Getenv("NO_COLOR") != "",
		screen:    browse,
		focus:     1,
		drafts:    make(map[config.Scope]map[string]any, 3),
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
			m.status = "Change cancelled"
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
		if spec, ok := m.currentSpec(); ok && spec.Kind == catalog.Boolean {
			m.toggle(spec)
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
	case "tab", "shift+tab":
		m.focus = 1 - m.focus
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
	case "left", "h":
		m.moveCategory(-1)
	case "right":
		m.moveCategory(1)
	case "enter":
		m.openEditor()
	case "u", "backspace", "delete":
		if spec, ok := m.currentSpec(); ok {
			config.Unset(m.drafts[m.scope], spec.Path)
			m.status = spec.Label + " now inherits from a lower scope"
		}
	case "s":
		if !m.dirty(m.scope) {
			m.status = "Nothing to save in " + string(m.scope)
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
	m.status = "Editing " + string(scope) + " settings"
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
		m.choice = 0
		if current, _, ok := m.effective(spec); ok {
			if index := slices.Index(spec.Options, fmt.Sprint(current)); index >= 0 {
				m.choice = index
			}
		}
		m.screen = editChoice
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

func (m *Model) toggle(spec catalog.Spec) {
	current := false
	if value, _, ok := m.effective(spec); ok {
		current, _ = value.(bool)
	}
	m.propose(spec, !current)
}

func (m *Model) handleChoiceKey(msg tea.KeyPressMsg) {
	switch msg.String() {
	case "esc", "q":
		m.screen = browse
	case "up", "k", "left", "h":
		m.choice = (m.choice - 1 + len(m.editSpec.Options)) % len(m.editSpec.Options)
	case "down", "j", "right", "l":
		m.choice = (m.choice + 1) % len(m.editSpec.Options)
	case "enter", " ":
		m.propose(m.editSpec, m.editSpec.Options[m.choice])
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
			m.status = fmt.Sprintf("%s accepts at most %d items", m.editSpec.Label, m.editSpec.MaxItems)
			return
		}
		m.inputListEdit = -1
		m.openInput(listItem, m.editSpec, "", editList)
	case "e", "enter":
		if len(items) > 0 {
			m.inputListEdit = m.listSelected
			m.openInput(listItem, m.editSpec, fmt.Sprint(items[m.listSelected]), editList)
		}
	case "d", "delete", "backspace":
		if len(items) > 0 {
			items = append(items[:m.listSelected], items[m.listSelected+1:]...)
			config.Set(m.drafts[m.scope], m.editSpec.Path, items)
			if m.listSelected >= len(items) && m.listSelected > 0 {
				m.listSelected--
			}
			m.status = "Item removed"
		}
	}
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
		m.screen = browse
		return
	}
	if value == "" {
		m.status = "Value cannot be empty; use u to inherit instead"
		return
	}
	if m.inputPurpose == settingValue {
		m.propose(m.editSpec, value)
		if m.screen != confirmDanger {
			m.screen = browse
		}
		return
	}

	items := m.ownList(m.editSpec)
	for i, item := range items {
		if fmt.Sprint(item) == value && i != m.inputListEdit {
			m.status = "Duplicate items are ignored"
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
	m.status = "List updated"
	m.screen = editList
}

func (m *Model) propose(spec catalog.Spec, value any) {
	if message, dangerous := spec.Dangerous(value); dangerous {
		m.pendingSpec = spec
		m.pendingValue = value
		m.dangerText = message
		m.screen = confirmDanger
		return
	}
	m.setValue(spec, value)
}

func (m *Model) setValue(spec catalog.Spec, value any) {
	config.Set(m.drafts[m.scope], spec.Path, value)
	m.status = spec.Label + " staged"
}

func (m *Model) handleDiffKey(msg tea.KeyPressMsg) {
	switch msg.String() {
	case "esc", "n", "q":
		m.screen = browse
	case "enter", "y", "s":
		err := m.workspace.Save(m.scope, m.drafts[m.scope], "")
		if err != nil {
			if errors.Is(err, config.ErrConflict) {
				m.status = "Save blocked: the file changed on disk. Reload before editing again."
			} else {
				m.status = "Save failed: " + err.Error()
			}
			m.screen = browse
			return
		}
		m.drafts[m.scope] = config.Clone(m.workspace.Docs[m.scope].Data)
		m.status = "Saved. Restart running Claude Code sessions to apply startup settings."
		m.screen = browse
	}
}

func (m *Model) reload() {
	if err := m.workspace.Reload(); err != nil {
		m.status = "Reload failed: " + err.Error()
		m.screen = browse
		return
	}
	m.resetDrafts()
	m.status = "Settings reloaded from disk"
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
		haystack := strings.ToLower(spec.Label + " " + spec.Path + " " + spec.Description)
		if strings.Contains(haystack, query) {
			specs = append(specs, spec)
		}
	}
	return specs
}

func (m *Model) diffLines() []string {
	var lines []string
	for _, spec := range catalog.Specs {
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
	if m.width < 48 || m.height < 12 {
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
			content = m.renderConfirm("Dangerous setting", m.dangerText, "Enter/y apply · Esc/n cancel", true)
		case confirmQuit:
			content = m.renderConfirm("Discard staged changes?", "Unsaved changes exist in one or more scopes.", "Enter/y discard · Esc/n stay", true)
		case confirmReload:
			content = m.renderConfirm("Reload from disk?", "All staged changes will be discarded.", "Enter/y reload · Esc/n stay", true)
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
	return title + "\n\nTerminal is too small.\nResize to at least 48×12.\n\nq quit"
}

func (m *Model) renderBrowser() string {
	header := m.renderHeader()
	footer := m.renderFooter()
	bodyHeight := max(8, m.height-lipgloss.Height(header)-lipgloss.Height(footer)-2)
	var body string
	switch {
	case m.width >= 110:
		categoryWidth := 19
		settingsWidth := 40
		detailWidth := max(42, m.width-categoryWidth-settingsWidth-4)
		body = lipgloss.JoinHorizontal(lipgloss.Top,
			m.panel("CATEGORIES", m.renderCategories(bodyHeight-2), categoryWidth, bodyHeight, m.focus == 0),
			m.panel("SETTINGS", m.renderSettings(bodyHeight-2), settingsWidth, bodyHeight, m.focus == 1),
			m.panel("DETAIL", m.renderDetail(detailWidth-4), detailWidth, bodyHeight, false),
		)
	case m.width >= 78:
		tabs := m.renderCategoryTabs()
		settingsWidth := 38
		detailWidth := max(36, m.width-settingsWidth-2)
		panels := lipgloss.JoinHorizontal(lipgloss.Top,
			m.panel("SETTINGS", m.renderSettings(bodyHeight-3), settingsWidth, bodyHeight-1, true),
			m.panel("DETAIL", m.renderDetail(detailWidth-4), detailWidth, bodyHeight-1, false),
		)
		body = tabs + "\n" + panels
	default:
		listHeight := max(5, bodyHeight/2)
		body = m.renderCategoryTabs() + "\n" +
			m.panel("SETTINGS", m.renderSettings(listHeight-2), m.width, listHeight, true) + "\n" +
			m.panel("DETAIL", m.renderDetail(m.width-4), m.width, bodyHeight-listHeight-1, false)
	}
	return header + "\n" + body + "\n" + footer
}

func (m *Model) renderHeader() string {
	title := m.style("#006D77", "#67E8F9").Bold(true).Render("CLAUDE CONFIG")
	versionText := m.version
	if versionText != "dev" && !strings.HasPrefix(versionText, "v") {
		versionText = "v" + versionText
	}
	version := m.muted().Render(" " + versionText)
	scope := m.style("#6D28D9", "#C4B5FD").Bold(true).Render(strings.ToUpper(string(m.scope)))
	path := m.workspace.Paths.For(m.scope)
	dirty := ""
	if m.dirty(m.scope) {
		dirty = m.warning().Render("  ● staged")
	}
	left := title + version + "   " + scope + dirty
	right := m.muted().Render(truncate(path, max(10, m.width-lipgloss.Width(left)-2)))
	gap := max(1, m.width-lipgloss.Width(left)-lipgloss.Width(right))
	return left + strings.Repeat(" ", gap) + right
}

func (m *Model) renderFooter() string {
	keys := "↑↓ navigate  Enter edit  Space toggle  / search  u inherit  s save  ? help  q quit"
	if m.width < 110 {
		keys = "↑↓ move  Enter edit  / search  u inherit  s save  ? help  q quit"
	}
	if m.status != "" {
		keys = m.status
	}
	return m.muted().Width(max(1, m.width)).Render(truncate(keys, m.width))
}

func (m *Model) renderCategories(height int) string {
	categories := catalog.Categories()
	lines := make([]string, 0, len(categories))
	for i, category := range categories {
		prefix := "  "
		style := m.muted()
		if i == m.category {
			prefix = "› "
			style = m.selectedStyle()
		}
		lines = append(lines, style.Render(prefix+category))
	}
	return fitLines(lines, height)
}

func (m *Model) renderCategoryTabs() string {
	if m.query != "" {
		return m.style("#6D28D9", "#C4B5FD").Bold(true).Render("Search: " + m.query)
	}
	var tabs []string
	for i, category := range catalog.Categories() {
		style := m.muted().Padding(0, 1)
		if i == m.category {
			style = m.selectedStyle().Padding(0, 1)
		}
		tabs = append(tabs, style.Render(category))
	}
	return truncate(strings.Join(tabs, " "), m.width)
}

func (m *Model) renderSettings(height int) string {
	specs := m.visibleSpecs()
	if len(specs) == 0 {
		return m.muted().Render("No matching settings")
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
		text := "<inherit>"
		if ok {
			text = compactValue(value)
			if source != string(m.scope) {
				text += "  ↳"
			}
		}
		changed := " "
		old, oldOK := config.Get(m.workspace.Docs[m.scope].Data, spec.Path)
		next, nextOK := config.Get(m.drafts[m.scope], spec.Path)
		if oldOK != nextOK || !config.Equal(old, next) {
			changed = "●"
		}
		cursor := " "
		if i == m.selected {
			cursor = "›"
		}
		line := fmt.Sprintf("%s%s %-17s %s", cursor, changed, truncate(spec.Label, 17), truncate(text, 10))
		style := m.muted()
		if i == m.selected {
			style = m.selectedStyle()
		} else if changed == "●" {
			style = m.warning()
		}
		lines = append(lines, style.Render(truncate(line, 33)))
	}
	return fitLines(lines, height)
}

func (m *Model) renderDetail(width int) string {
	spec, ok := m.currentSpec()
	if !ok {
		return "No setting selected"
	}
	own, ownOK := config.Get(m.drafts[m.scope], spec.Path)
	effective, source, effectiveOK := m.effective(spec)
	target := "<inherit>"
	if ownOK {
		target = formatValue(own)
	}
	resolved := "<Claude default>"
	if effectiveOK {
		resolved = formatValue(effective)
	}
	lines := []string{
		m.accent().Bold(true).Render(spec.Label),
		m.muted().Render(spec.Path),
		"",
		m.muted().Render("THIS SCOPE"),
		wrap(target, width),
		"",
		m.muted().Render("EFFECTIVE"),
		wrap(resolved, width),
	}
	if source != "" {
		lines = append(lines, m.muted().Render("source: "+source))
	}
	lines = append(lines, "", wrap(spec.Description, width))
	if len(spec.Options) > 0 {
		lines = append(lines, "", m.muted().Render("Suggestions: "+strings.Join(spec.Options, ", ")))
	}
	if spec.Kind == catalog.List && strings.HasPrefix(spec.Path, "permissions.") {
		lines = append(lines, "", m.muted().Render("Permission arrays merge across scopes."))
	}
	return strings.Join(lines, "\n")
}

func (m *Model) renderTextEditor() string {
	title := m.editSpec.Label
	description := m.editSpec.Description
	if m.inputPurpose == searchValue {
		title = "Search settings"
		description = "Match labels, JSON paths, and descriptions."
	} else if m.inputPurpose == listItem {
		title = "Edit " + m.editSpec.Label
	}
	cursor := m.inputCursor
	before := string(m.input[:cursor])
	after := string(m.input[cursor:])
	value := before + m.accent().Reverse(true).Render(" ") + after
	body := wrap(description, m.modalContentWidth()) + "\n\n" +
		m.inputStyle().Render(value) + "\n\n" +
		m.muted().Render("Enter apply · Esc cancel · ←→ move")
	if len(m.editSpec.Options) > 0 && m.inputPurpose == settingValue {
		body += "\n" + m.muted().Render("Suggestions: "+strings.Join(m.editSpec.Options, ", "))
	}
	return m.modal(title, body, false)
}

func (m *Model) renderChoice() string {
	var lines []string
	for i, option := range m.editSpec.Options {
		style := m.muted()
		prefix := "  "
		if i == m.choice {
			style = m.selectedStyle()
			prefix = "› "
		}
		lines = append(lines, style.Render(prefix+option))
	}
	body := wrap(m.editSpec.Description, m.modalContentWidth()) + "\n\n" +
		strings.Join(lines, "\n") + "\n\n" +
		m.muted().Render("↑↓ choose · Enter apply · Esc cancel")
	return m.modal(m.editSpec.Label, body, false)
}

func (m *Model) renderList() string {
	items := m.ownList(m.editSpec)
	var lines []string
	if len(items) == 0 {
		lines = append(lines, m.muted().Render("No values in this scope"))
	} else {
		for i, item := range items {
			style := m.muted()
			prefix := "  "
			if i == m.listSelected {
				style = m.selectedStyle()
				prefix = "› "
			}
			lines = append(lines, style.Render(prefix+fmt.Sprint(item)))
		}
	}
	body := wrap(m.editSpec.Description, m.modalContentWidth()) + "\n\n" +
		strings.Join(lines, "\n") + "\n\n" +
		m.muted().Render("a add · Enter/e edit · d delete · Esc done")
	if m.editSpec.MaxItems > 0 {
		body += "\n" + m.muted().Render(fmt.Sprintf("Maximum: %d items", m.editSpec.MaxItems))
	}
	return m.modal(m.editSpec.Label, body, false)
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
		rendered = append(rendered, m.muted().Render(fmt.Sprintf("… %d more changes", hidden)))
	}
	body := strings.Join(rendered, "\n") + "\n\n" +
		m.muted().Render("Enter/y/s save · Esc/n cancel") + "\n" +
		m.muted().Render(m.workspace.Paths.For(m.scope))
	return m.modal("Save "+string(m.scope)+" settings?", body, false)
}

func (m *Model) renderHelp() string {
	body := `Navigation
  ↑/↓ or j/k    move
  ←/→           category
  Tab           switch focus
  g / p / l     global / project / local

Editing
  Enter         edit value
  Space         toggle boolean
  u             unset and inherit
  /             search
  s             review and save
  r             reload from disk
  q             quit

Values marked ↳ are inherited. ● marks staged changes.`
	return m.modal("Keyboard help", body+"\n\n"+m.muted().Render("Esc or ? close"), false)
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
	return m.panelStyle(active).
		Width(max(1, width-2)).
		Height(max(1, height-2)).
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

func compactValue(value any) string {
	switch typed := value.(type) {
	case []any:
		return fmt.Sprintf("%d items", len(typed))
	default:
		return truncate(formatValue(value), 13)
	}
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
