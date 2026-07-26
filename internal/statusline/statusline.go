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
	colors, err := colorsFor(options)
	if err != nil {
		return "", err
	}

	model := data.Model.DisplayName
	if model == "" {
		model = data.Model.ID
	}
	if model == "" {
		model = "Claude"
	}
	segments := []segment{{text: shorten(model, 28), compact: shorten(model, 14), role: "accent"}}

	if project := projectName(data); project != "" {
		segments = append(segments, segment{text: project, role: "project", priority: 4})
	}
	branch := options.Branch
	if branch == "" {
		branch = data.Worktree.Branch
	}
	if branch != "" {
		segments = append(segments, segment{text: "git:" + shorten(branch, 24), role: "git", priority: 3})
	}
	if remaining, ok := contextRemaining(data); ok {
		segments = append(segments, segment{
			text:     fmt.Sprintf("ctx:%d%% left", percent(remaining)),
			compact:  fmt.Sprintf("ctx:%d%%", percent(remaining)),
			role:     usageRole(remaining),
			priority: 1,
		})
	}
	segments = append(segments, segment{text: options.Now.Format("15:04"), role: "muted", priority: 5})
	if data.RateLimits.FiveHour != nil {
		segments = append(segments, rateSegment("5h", *data.RateLimits.FiveHour, options.Now))
	}
	if data.RateLimits.SevenDay != nil {
		segments = append(segments, rateSegment("7d", *data.RateLimits.SevenDay, options.Now))
	}

	segments, plain := fit(segments, options.Columns)
	first := paintSegments(colors, segments)
	if plain != "" {
		first = plain
	}
	if second := secondaryLine(data, colors, options.Columns); second != "" {
		return first + "\n" + second, nil
	}
	return first, nil
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

func rateSegment(label string, window RateWindow, now time.Time) segment {
	remaining := clamp(100 - window.UsedPercentage)
	reset := ""
	if window.ResetsAt > 0 {
		reset = " · ↻" + until(time.Unix(window.ResetsAt, 0).Sub(now))
	}
	return segment{
		text:    fmt.Sprintf("%s:%d%% left%s", label, percent(remaining), reset),
		compact: fmt.Sprintf("%s:%d%%", label, percent(remaining)),
		role:    usageRole(remaining),
	}
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
	case "claude":
		return map[string]string{
			"accent":  "\x1b[1;38;2;217;119;87m",
			"project": "\x1b[38;2;139;128;120m",
			"git":     "\x1b[38;2;103;143;105m",
			"context": "\x1b[38;2;139;111;177m",
			"success": "\x1b[38;2;103;143;105m",
			"warning": "\x1b[38;2;198;132;67m",
			"danger":  "\x1b[38;2;194;65;59m",
			"muted":   "\x1b[38;2;124;119;116m",
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
		return nil, fmt.Errorf("unknown status-line theme %q: use auto, claude, ansi, or mono", options.Theme)
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

func until(duration time.Duration) string {
	if duration <= 0 {
		return "now"
	}
	minutes := int(math.Ceil(duration.Minutes()))
	days := minutes / (24 * 60)
	hours := minutes / 60 % 24
	minutes %= 60
	switch {
	case days > 0:
		return fmt.Sprintf("%dd%dh", days, hours)
	case hours > 0:
		return fmt.Sprintf("%dh%dm", hours, minutes)
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
