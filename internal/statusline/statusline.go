package statusline

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

type Data struct {
	CWD         string `json:"cwd"`
	SessionName string `json:"session_name"`
	Model       struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	} `json:"model"`
	Workspace struct {
		CurrentDir string `json:"current_dir"`
		ProjectDir string `json:"project_dir"`
		Repo       struct {
			Name string `json:"name"`
		} `json:"repo"`
	} `json:"workspace"`
	ContextWindow struct {
		UsedPercentage      *float64 `json:"used_percentage"`
		RemainingPercentage *float64 `json:"remaining_percentage"`
	} `json:"context_window"`
	RateLimits struct {
		FiveHour *RateWindow `json:"five_hour"`
		SevenDay *RateWindow `json:"seven_day"`
	} `json:"rate_limits"`
	FastMode *bool `json:"fast_mode"`
	Effort   struct {
		Level string `json:"level"`
	} `json:"effort"`
	Thinking struct {
		Enabled *bool `json:"enabled"`
	} `json:"thinking"`
	Vim struct {
		Mode string `json:"mode"`
	} `json:"vim"`
	Agent struct {
		Name string `json:"name"`
	} `json:"agent"`
	OutputStyle struct {
		Name string `json:"name"`
	} `json:"output_style"`
	Worktree struct {
		Branch string `json:"branch"`
	} `json:"worktree"`
}

type RateWindow struct {
	UsedPercentage float64 `json:"used_percentage"`
	ResetsAt       int64   `json:"resets_at"`
}

type Options struct {
	Theme     string
	Columns   int
	Now       time.Time
	Branch    string
	NoColor   bool
	ColorTerm string
	Term      string
	Language  string
}

type segment struct {
	text     string
	compact  string
	role     string
	priority int
}

func Run(in io.Reader, out io.Writer, theme string, now time.Time) error {
	var data Data
	if err := json.NewDecoder(io.LimitReader(in, 1<<20)).Decode(&data); err != nil {
		return fmt.Errorf("read Claude Code status JSON: %w", err)
	}
	columns, _ := strconv.Atoi(os.Getenv("COLUMNS"))
	cwd := data.Workspace.CurrentDir
	if cwd == "" {
		cwd = data.CWD
	}
	branch := data.Worktree.Branch
	if branch == "" {
		branch = gitBranch(cwd)
	}
	rendered, err := Render(data, Options{
		Theme:     theme,
		Columns:   columns,
		Now:       now,
		Branch:    branch,
		NoColor:   os.Getenv("NO_COLOR") != "",
		ColorTerm: os.Getenv("COLORTERM"),
		Term:      os.Getenv("TERM"),
	})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, rendered)
	return err
}

func Render(data Data, options Options) (string, error) {
	if options.Columns <= 0 {
		options.Columns = 120
	}
	if options.Now.IsZero() {
		options.Now = time.Now()
	}
	if options.Language == "" || options.Language == "auto" {
		options.Language = systemLanguage()
	}
	colors, err := colorsFor(options)
	if err != nil {
		return "", err
	}
	icons := options.Theme == "nerd"

	model := data.Model.DisplayName
	if model == "" {
		model = data.Model.ID
	}
	if model == "" {
		model = "Claude"
	}
	if icons {
		model = "󰚩 " + model
	}
	segments := []segment{{text: shorten(model, 28), compact: shorten(model, 14), role: "accent"}}

	if project := projectName(data); project != "" {
		if icons {
			project = " " + project
		}
		segments = append(segments, segment{text: project, role: "project", priority: 4})
	}
	branch := options.Branch
	if branch == "" {
		branch = data.Worktree.Branch
	}
	if branch != "" {
		prefix := "git:"
		if icons {
			prefix = " "
		}
		segments = append(segments, segment{text: prefix + shorten(branch, 24), role: "git", priority: 3})
	}
	if remaining, ok := contextRemaining(data); ok {
		prefix := "ctx:"
		if icons {
			prefix = "󰍛 "
		}
		segments = append(segments, segment{
			text:     fmt.Sprintf("%s%d%% left", prefix, percent(remaining)),
			compact:  fmt.Sprintf("%s%d%%", prefix, percent(remaining)),
			role:     usageRole(remaining),
			priority: 1,
		})
	}
	clock := options.Now.Format("15:04")
	if icons {
		clock = " " + clock
	}
	segments = append(segments, segment{text: clock, role: "muted", priority: 5})

	segments, plain := fit(segments, options.Columns)
	first := paintSegments(colors, segments)
	if plain != "" {
		first = plain
	}
	lines := []string{first}
	lines = append(lines, limitLines(data, colors, options)...)
	if second := secondaryLine(data, colors, options.Columns); second != "" {
		lines = append(lines, second)
	}
	return strings.Join(lines, "\n"), nil
}

func projectName(data Data) string {
	if data.Workspace.Repo.Name != "" {
		return shorten(data.Workspace.Repo.Name, 24)
	}
	path := data.Workspace.ProjectDir
	if path == "" {
		path = data.Workspace.CurrentDir
	}
	if path == "" {
		path = data.CWD
	}
	name := filepath.Base(filepath.Clean(path))
	if name == "." || name == string(filepath.Separator) {
		return ""
	}
	return shorten(name, 24)
}

func contextRemaining(data Data) (float64, bool) {
	if data.ContextWindow.RemainingPercentage != nil {
		return clamp(*data.ContextWindow.RemainingPercentage), true
	}
	if data.ContextWindow.UsedPercentage != nil {
		return clamp(100 - *data.ContextWindow.UsedPercentage), true
	}
	return 0, false
}

type limitLine struct {
	plain    string
	rendered string
}

func limitLines(data Data, colors map[string]string, options Options) []string {
	type rateItem struct {
		label  string
		window RateWindow
	}
	var windows []rateItem
	if data.RateLimits.FiveHour != nil && data.RateLimits.FiveHour.ResetsAt > 0 {
		windows = append(windows, rateItem{"5h", *data.RateLimits.FiveHour})
	}
	if data.RateLimits.SevenDay != nil && data.RateLimits.SevenDay.ResetsAt > 0 {
		windows = append(windows, rateItem{"7d", *data.RateLimits.SevenDay})
	}
	if len(windows) == 0 {
		message, compact := availabilityText(options.Language)
		if options.Theme == "nerd" {
			message, compact = " "+message, " "+compact
		}
		if options.Columns < lipgloss.Width(message) {
			message = compact
		}
		return []string{paint(colors, "muted", shorten(message, options.Columns))}
	}

	full := make([]limitLine, 0, len(windows))
	for _, item := range windows {
		full = append(full, renderLimit(item.label, item.window, options, colors, true))
	}
	if len(full) == 2 {
		plain := full[0].plain + "  │  " + full[1].plain
		if lipgloss.Width(plain) <= options.Columns {
			return []string{full[0].rendered + paint(colors, "muted", "  │  ") + full[1].rendered}
		}
	}

	lines := make([]string, 0, len(windows))
	for i, item := range windows {
		line := full[i]
		if lipgloss.Width(line.plain) > options.Columns {
			line = renderLimit(item.label, item.window, options, colors, false)
		}
		if lipgloss.Width(line.plain) > options.Columns {
			line.plain = shorten(line.plain, options.Columns)
			line.rendered = paint(colors, usageRole(100-item.window.UsedPercentage), line.plain)
		}
		lines = append(lines, line.rendered)
	}
	return lines
}

func renderLimit(label string, window RateWindow, options Options, colors map[string]string, full bool) limitLine {
	now := options.Now
	used := clamp(window.UsedPercentage)
	remaining := clamp(100 - window.UsedPercentage)
	resetAt := time.Unix(window.ResetsAt, 0).In(now.Location())
	reset := formatReset(resetAt, now, options.Language)
	countdown := until(resetAt.Sub(now), options.Language)
	words := wordsFor(options.Language)
	if options.Theme == "nerd" {
		label = " " + label
	}
	role := usageRole(remaining)

	if !full {
		plain := fmt.Sprintf("%s  %d%% %s · %d%% %s · ↻ %s · %s",
			label, percent(used), words.used, percent(remaining), words.left,
			formatResetCompact(resetAt, now, options.Language), countdown)
		return limitLine{plain: plain, rendered: paint(colors, role, plain)}
	}

	bar := progressBar(used, 10)
	usage := fmt.Sprintf("%d%% %s · %d%% %s", percent(used), words.used, percent(remaining), words.left)
	timing := fmt.Sprintf(" · %s %s · %s %s", words.resets, reset, words.in, countdown)
	plain := label + "  " + bar + "  " + usage + timing
	rendered := paint(colors, "accent", label) + "  " +
		paint(colors, role, bar) + "  " +
		paint(colors, role, usage) +
		paint(colors, "muted", timing)
	return limitLine{plain: plain, rendered: rendered}
}

func progressBar(used float64, width int) string {
	filled := int(math.Round(clamp(used) * float64(width) / 100))
	return strings.Repeat("▓", filled) + strings.Repeat("░", width-filled)
}

func secondaryLine(data Data, colors map[string]string, columns int) string {
	var items []string
	if data.SessionName != "" && data.SessionName != projectName(data) {
		items = append(items, "session:"+shorten(data.SessionName, 28))
	}
	if data.Agent.Name != "" {
		items = append(items, "agent:"+shorten(data.Agent.Name, 24))
	}
	if data.Effort.Level != "" {
		items = append(items, "effort:"+data.Effort.Level)
	}
	if data.Thinking.Enabled != nil && *data.Thinking.Enabled {
		items = append(items, "thinking:on")
	}
	if data.FastMode != nil && *data.FastMode {
		items = append(items, "fast:on")
	}
	if data.Vim.Mode != "" {
		items = append(items, "vim:"+data.Vim.Mode)
	}
	if data.OutputStyle.Name != "" && data.OutputStyle.Name != "default" {
		items = append(items, "style:"+data.OutputStyle.Name)
	}
	if len(items) == 0 {
		return ""
	}
	line := "↳ " + strings.Join(items, " · ")
	line = shorten(line, max(columns, 1))
	return paint(colors, "muted", line)
}

func fit(segments []segment, columns int) ([]segment, string) {
	for visibleWidth(segments) > columns {
		index, priority := -1, 0
		for i, item := range segments {
			if item.priority > priority {
				index, priority = i, item.priority
			}
		}
		if index < 0 {
			break
		}
		segments = append(segments[:index], segments[index+1:]...)
	}
	if visibleWidth(segments) <= columns {
		return segments, ""
	}
	for i := range segments {
		if segments[i].compact != "" {
			segments[i].text = segments[i].compact
		}
	}
	if visibleWidth(segments) <= columns {
		return segments, ""
	}
	var text []string
	for _, item := range segments {
		text = append(text, item.text)
	}
	return nil, shorten(strings.Join(text, " | "), columns)
}

func visibleWidth(segments []segment) int {
	var text []string
	for _, item := range segments {
		text = append(text, item.text)
	}
	return lipgloss.Width(strings.Join(text, " | "))
}

func paintSegments(colors map[string]string, segments []segment) string {
	rendered := make([]string, 0, len(segments))
	for _, item := range segments {
		rendered = append(rendered, paint(colors, item.role, item.text))
	}
	return strings.Join(rendered, paint(colors, "muted", " | "))
}

func paint(colors map[string]string, role, text string) string {
	if colors[role] == "" {
		return text
	}
	return colors[role] + text + "\x1b[0m"
}

func colorsFor(options Options) (map[string]string, error) {
	theme := options.Theme
	if theme == "" {
		theme = "auto"
	}
	if options.NoColor {
		theme = "mono"
	} else if theme == "auto" {
		switch {
		case strings.EqualFold(options.Term, "dumb"):
			theme = "mono"
		case strings.Contains(strings.ToLower(options.ColorTerm), "truecolor") ||
			strings.Contains(strings.ToLower(options.ColorTerm), "24bit"):
			theme = "claude"
		default:
			theme = "ansi"
		}
	}
	switch theme {
	case "claude", "nerd":
		return map[string]string{
			"accent":  "\x1b[1;38;2;240;138;104m",
			"project": "\x1b[38;2;190;181;173m",
			"git":     "\x1b[1;38;2;138;203;136m",
			"context": "\x1b[1;38;2;199;160;255m",
			"success": "\x1b[1;38;2;138;203;136m",
			"warning": "\x1b[1;38;2;255;180;84m",
			"danger":  "\x1b[1;38;2;255;107;99m",
			"muted":   "\x1b[38;2;183;174;167m",
		}, nil
	case "ansi":
		return map[string]string{
			"accent":  "\x1b[1;33m",
			"project": "\x1b[90m",
			"git":     "\x1b[32m",
			"context": "\x1b[35m",
			"success": "\x1b[32m",
			"warning": "\x1b[33m",
			"danger":  "\x1b[31m",
			"muted":   "\x1b[90m",
		}, nil
	case "mono":
		return map[string]string{}, nil
	default:
		return nil, fmt.Errorf("unknown status-line theme %q: use auto, nerd, claude, ansi, or mono", options.Theme)
	}
}

func usageRole(remaining float64) string {
	switch {
	case remaining < 20:
		return "danger"
	case remaining < 50:
		return "warning"
	default:
		return "success"
	}
}

func percent(value float64) int {
	return int(math.Round(clamp(value)))
}

func clamp(value float64) float64 {
	return math.Max(0, math.Min(100, value))
}

type statusWords struct {
	used   string
	left   string
	resets string
	in     string
}

func wordsFor(language string) statusWords {
	switch language {
	case "ru":
		return statusWords{"исп.", "ост.", "сброс", "через"}
	case "zh-CN":
		return statusWords{"已用", "剩余", "重置", "还有"}
	default:
		return statusWords{"used", "left", "resets", "in"}
	}
}

func availabilityText(language string) (string, string) {
	switch language {
	case "ru":
		return "лимиты: ждём данные Claude.ai · появятся после первого ответа Pro/Max",
			"лимиты: недоступны в этой сессии"
	case "zh-CN":
		return "限额：正在等待 Claude.ai 数据 · Pro/Max 首次响应后显示",
			"限额：此会话中不可用"
	default:
		return "limits: waiting for Claude.ai usage data · available after the first Pro/Max response",
			"limits: unavailable in this session"
	}
}

func formatReset(reset, now time.Time, language string) string {
	timePart := reset.Format("15:04")
	var datePart string
	switch {
	case sameDate(reset, now):
		switch language {
		case "ru":
			datePart = "сегодня, " + timePart
		case "zh-CN":
			datePart = "今天 " + timePart
		default:
			datePart = "today, " + timePart
		}
	case sameDate(reset, now.AddDate(0, 0, 1)):
		switch language {
		case "ru":
			datePart = "завтра, " + timePart
		case "zh-CN":
			datePart = "明天 " + timePart
		default:
			datePart = "tomorrow, " + timePart
		}
	default:
		datePart = calendarDate(reset, now, language) + ", " + timePart
	}
	return datePart + " (" + timezoneLabel(reset) + ")"
}

func formatResetCompact(reset, now time.Time, language string) string {
	full := formatReset(reset, now, language)
	return strings.NewReplacer(", ", " ", " (", " ").Replace(full[:len(full)-1])
}

func calendarDate(value, now time.Time, language string) string {
	monthsEN := [...]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	monthsRU := [...]string{"янв.", "февр.", "мар.", "апр.", "мая", "июн.", "июл.", "авг.", "сент.", "окт.", "нояб.", "дек."}
	year := ""
	if value.Year() != now.Year() {
		year = fmt.Sprintf(" %d", value.Year())
	}
	switch language {
	case "ru":
		return fmt.Sprintf("%d %s%s", value.Day(), monthsRU[value.Month()-1], year)
	case "zh-CN":
		if year != "" {
			return fmt.Sprintf("%d年%d月%d日", value.Year(), value.Month(), value.Day())
		}
		return fmt.Sprintf("%d月%d日", value.Month(), value.Day())
	default:
		return fmt.Sprintf("%s %d%s", monthsEN[value.Month()-1], value.Day(), year)
	}
}

func sameDate(left, right time.Time) bool {
	left = left.In(right.Location())
	return left.Year() == right.Year() && left.YearDay() == right.YearDay()
}

func timezoneLabel(value time.Time) string {
	_, offset := value.Zone()
	if offset == 0 {
		return "UTC"
	}
	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}
	hours, minutes := offset/3600, offset%3600/60
	if minutes == 0 {
		return fmt.Sprintf("UTC%s%d", sign, hours)
	}
	return fmt.Sprintf("UTC%s%d:%02d", sign, hours, minutes)
}

func systemLanguage() string {
	names := []string{os.Getenv("LC_ALL"), os.Getenv("LC_MESSAGES"), os.Getenv("LANGUAGE")}
	if runtime.GOOS == "darwin" {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		if output, err := exec.CommandContext(ctx, "/usr/bin/defaults", "read", "-g", "AppleLocale").Output(); err == nil {
			names = append(names, strings.TrimSpace(string(output)))
		}
	}
	names = append(names, os.Getenv("LANG"))
	for _, name := range names {
		normalized := strings.ToLower(strings.ReplaceAll(name, "_", "-"))
		switch {
		case strings.HasPrefix(normalized, "ru"):
			return "ru"
		case strings.HasPrefix(normalized, "zh"):
			return "zh-CN"
		case normalized != "" && normalized != "c" && normalized != "c.utf-8" && normalized != "posix":
			return "en"
		}
	}
	return "en"
}

func until(duration time.Duration, language string) string {
	if duration <= 0 {
		switch language {
		case "ru":
			return "сейчас"
		case "zh-CN":
			return "现在"
		default:
			return "now"
		}
	}
	minutes := int(math.Ceil(duration.Minutes()))
	days := minutes / (24 * 60)
	hours := minutes / 60 % 24
	minutes %= 60
	if language == "ru" {
		switch {
		case days > 0:
			return fmt.Sprintf("%d д %d ч", days, hours)
		case hours > 0:
			return fmt.Sprintf("%d ч %d мин", hours, minutes)
		default:
			return fmt.Sprintf("%d мин", minutes)
		}
	}
	if language == "zh-CN" {
		switch {
		case days > 0:
			return fmt.Sprintf("%d天%d小时", days, hours)
		case hours > 0:
			return fmt.Sprintf("%d小时%d分钟", hours, minutes)
		default:
			return fmt.Sprintf("%d分钟", minutes)
		}
	}
	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh", days, hours)
	case hours > 0:
		return fmt.Sprintf("%dh %dm", hours, minutes)
	default:
		return fmt.Sprintf("%dm", minutes)
	}
}

func shorten(value string, width int) string {
	if width <= 0 || lipgloss.Width(value) <= width {
		return value
	}
	if width == 1 {
		return "…"
	}
	runes := []rune(value)
	for len(runes) > 0 && lipgloss.Width(string(runes)) > width-1 {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

func gitBranch(cwd string) string {
	if cwd == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	output, err := exec.CommandContext(ctx, "git", "-C", cwd, "branch", "--show-current").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}
