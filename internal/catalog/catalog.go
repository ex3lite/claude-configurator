package catalog

import "fmt"

type Kind uint8

const (
	String Kind = iota
	Boolean
	Enum
	List
)

type Spec struct {
	ID          string
	Path        string
	Label       string
	Category    string
	Description string
	Kind        Kind
	Options     []string
	AllowCustom bool
	MaxItems    int
	Danger      map[string]string
	App         bool
}

func (s Spec) Dangerous(value any) (string, bool) {
	if len(s.Danger) == 0 {
		return "", false
	}
	message, ok := s.Danger[fmt.Sprint(value)]
	return message, ok
}

var Specs = []Spec{
	{ID: "main-model", Path: "model", Label: "Main model", Category: "Models", Kind: Enum, Options: []string{"default", "best", "sonnet", "opus", "haiku", "sonnet[1m]", "opus[1m]", "opusplan", "claude-fable-5[1m]", "claude-sonnet-5"}, AllowCustom: true, Description: "Default model for new Claude Code sessions. Stable aliases follow the latest model available from your provider."},
	{ID: "subagent-model", Path: "env.CLAUDE_CODE_SUBAGENT_MODEL", Label: "Subagent model", Category: "Models", Kind: Enum, Options: []string{"sonnet", "opus", "haiku", "sonnet[1m]", "opus[1m]", "claude-sonnet-5"}, AllowCustom: true, Description: "Model for all subagents, agent teams, and workflow agents. Overrides per-agent model choices."},
	{ID: "advisor-model", Path: "advisorModel", Label: "Advisor model", Category: "Models", Kind: Enum, Options: []string{"best", "opus", "sonnet", "haiku", "claude-sonnet-5"}, AllowCustom: true, Description: "Model used by the server-side advisor tool."},
	{ID: "fallback-models", Path: "fallbackModel", Label: "Fallback models", Category: "Models", Kind: List, Options: []string{"sonnet", "opus", "haiku", "sonnet[1m]", "opus[1m]", "claude-sonnet-5"}, AllowCustom: true, MaxItems: 3, Description: "Ordered fallback chain. Claude Code accepts up to three unique aliases or model IDs."},

	{ID: "effort", Path: "effortLevel", Label: "Reasoning effort", Category: "Reasoning", Kind: Enum, Options: []string{"low", "medium", "high", "xhigh"}, Description: "Persistent adaptive reasoning effort for supported models."},
	{ID: "always-thinking", Path: "alwaysThinkingEnabled", Label: "Extended thinking", Category: "Reasoning", Kind: Boolean, Description: "Enable extended thinking by default."},
	{ID: "auto-compact", Path: "autoCompactEnabled", Label: "Auto compact", Category: "Reasoning", Kind: Boolean, Description: "Compact conversations automatically near the context limit."},

	{ID: "agent-teams", Path: "env.CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS", Label: "Agent teams", Category: "Agents", Kind: Enum, Options: []string{"0", "1"}, Description: "Enable the experimental agent teams feature."},
	{ID: "teammate-mode", Path: "teammateMode", Label: "Teammate display", Category: "Agents", Kind: Enum, Options: []string{"in-process", "auto", "tmux", "iterm2"}, Description: "How agent-team teammates are displayed."},
	{ID: "agent-view", Path: "disableAgentView", Label: "Disable agent view", Category: "Agents", Kind: Boolean, Description: "Disable background agents, agent view, and the on-demand supervisor."},

	{ID: "permission-mode", Path: "permissions.defaultMode", Label: "Default permission mode", Category: "Permissions", Kind: Enum, Options: []string{"default", "acceptEdits", "auto", "plan", "dontAsk", "bypassPermissions", "delegate"}, Description: "Default permission behavior for new sessions.", Danger: map[string]string{"bypassPermissions": "This skips permission prompts and can allow destructive actions."}},
	{ID: "permission-allow", Path: "permissions.allow", Label: "Allow rules", Category: "Permissions", Kind: List, Description: "Tool rules that are allowed without prompting. Rules merge across scopes."},
	{ID: "permission-ask", Path: "permissions.ask", Label: "Ask rules", Category: "Permissions", Kind: List, Description: "Tool rules that always require confirmation. Rules merge across scopes."},
	{ID: "permission-deny", Path: "permissions.deny", Label: "Deny rules", Category: "Permissions", Kind: List, Description: "Tool rules that are always denied. Rules merge across scopes."},

	{ID: "sandbox", Path: "sandbox.enabled", Label: "Sandbox", Category: "Safety", Kind: Boolean, Description: "Run supported shell commands in the Claude Code sandbox.", Danger: map[string]string{"false": "Disabling the sandbox removes an important command-isolation layer."}},
	{ID: "sandbox-auto-allow", Path: "sandbox.autoAllowBashIfSandboxed", Label: "Auto-allow sandboxed Bash", Category: "Safety", Kind: Boolean, Description: "Automatically allow Bash commands when Claude Code can sandbox them."},
	{ID: "sandbox-unsandboxed", Path: "sandbox.allowUnsandboxedCommands", Label: "Allow unsandboxed commands", Category: "Safety", Kind: Boolean, Description: "Allow commands to request execution outside the sandbox.", Danger: map[string]string{"true": "Unsandboxed commands can access resources outside the isolation boundary."}},
	{ID: "checkpointing", Path: "fileCheckpointingEnabled", Label: "File checkpointing", Category: "Safety", Kind: Boolean, Description: "Snapshot edited files so /rewind can restore them."},

	{ID: "ui-language", Path: "@app.language", Label: "Interface language", Category: "Interface", Kind: Enum, Options: []string{"auto", "en", "ru", "zh-CN"}, Description: "Language used by Claude Configurator. Auto follows the operating system language.", App: true},
	{ID: "theme", Path: "theme", Label: "Theme", Category: "Interface", Kind: String, Options: []string{"auto", "dark", "light", "dark-daltonized", "light-daltonized", "dark-ansi", "light-ansi"}, Description: "Claude Code color theme. Custom theme references are accepted."},
	{ID: "tui", Path: "tui", Label: "TUI renderer", Category: "Interface", Kind: Enum, Options: []string{"fullscreen", "default"}, Description: "Choose the fullscreen or classic Claude Code renderer."},
	{ID: "view", Path: "viewMode", Label: "Transcript view", Category: "Interface", Kind: Enum, Options: []string{"default", "verbose", "focus"}, Description: "Amount of tool detail displayed in the transcript."},
	{ID: "output-style", Path: "outputStyle", Label: "Output style", Category: "Interface", Kind: String, Options: []string{"default", "Explanatory", "Learning"}, Description: "Built-in or custom assistant output style."},
	{ID: "editor-mode", Path: "editorMode", Label: "Editor mode", Category: "Interface", Kind: Enum, Options: []string{"normal", "vim"}, Description: "Input editor key bindings."},
	{ID: "language", Path: "language", Label: "Claude response language", Category: "Interface", Kind: String, Description: "Preferred language for Claude responses, dictation, and terminal session titles."},
	{ID: "reduced-motion", Path: "prefersReducedMotion", Label: "Reduced motion", Category: "Interface", Kind: Boolean, Description: "Reduce non-essential terminal animation."},

	{ID: "auto-memory", Path: "autoMemoryEnabled", Label: "Auto memory", Category: "Behavior", Kind: Boolean, Description: "Allow Claude Code to save useful project context automatically."},
	{ID: "git-instructions", Path: "includeGitInstructions", Label: "Git instructions", Category: "Behavior", Kind: Boolean, Description: "Include Claude Code's built-in Git workflow instructions."},
	{ID: "updates", Path: "autoUpdatesChannel", Label: "Update channel", Category: "Behavior", Kind: Enum, Options: []string{"stable", "latest"}, Description: "Claude Code automatic update channel."},
}

func Categories() []string {
	var categories []string
	for _, spec := range Specs {
		if len(categories) == 0 || categories[len(categories)-1] != spec.Category {
			categories = append(categories, spec.Category)
		}
	}
	return categories
}
