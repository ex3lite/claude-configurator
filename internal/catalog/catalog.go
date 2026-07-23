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
	Purpose     string
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
	{ID: "main-model", Path: "model", Label: "Main model", Category: "Models", Kind: Enum, Options: []string{"default", "best", "fable", "sonnet", "opus", "haiku", "sonnet[1m]", "opus[1m]", "opusplan", "claude-fable-5[1m]", "claude-sonnet-5"}, AllowCustom: true, Description: "Model used when a new Claude Code session starts. Aliases follow the model recommended by your provider; full IDs pin a version.", Purpose: "Choose the capability, speed, and cost profile for day-to-day work without changing every launch command."},
	{ID: "subagent-model", Path: "env.CLAUDE_CODE_SUBAGENT_MODEL", Label: "Subagent model", Category: "Models", Kind: Enum, Options: []string{"fable", "sonnet", "opus", "haiku", "sonnet[1m]", "opus[1m]", "claude-fable-5[1m]", "claude-sonnet-5"}, AllowCustom: true, Description: "Model used by all subagents, agent teams, and workflow agents. It overrides model choices made by individual agents.", Purpose: "Keep the main session on a powerful model while delegated work uses a faster model, or give every agent Fable-level autonomy."},
	{ID: "advisor-model", Path: "advisorModel", Label: "Advisor model", Category: "Models", Kind: Enum, Options: []string{"best", "fable", "opus", "sonnet", "haiku", "claude-fable-5[1m]", "claude-sonnet-5"}, AllowCustom: true, Description: "Model used by the server-side advisor when Claude asks for a second opinion on a difficult decision.", Purpose: "Separate the model reviewing a decision from the model doing the main work."},
	{ID: "fallback-models", Path: "fallbackModel", Label: "Fallback models", Category: "Models", Kind: List, Options: []string{"fable", "sonnet", "opus", "haiku", "sonnet[1m]", "opus[1m]", "claude-fable-5[1m]", "claude-sonnet-5"}, AllowCustom: true, MaxItems: 3, Description: "Ordered fallback chain of up to three unique aliases or model IDs.", Purpose: "Keep work moving when the preferred model cannot serve a request and Claude Code needs another allowed model."},

	{ID: "effort", Path: "effortLevel", Label: "Reasoning effort", Category: "Reasoning", Kind: Enum, Options: []string{"low", "medium", "high", "xhigh"}, Description: "Persistent adaptive reasoning effort for models that support it.", Purpose: "Lower levels answer faster and cheaper; higher levels spend more reasoning on difficult tasks."},
	{ID: "always-thinking", Path: "alwaysThinkingEnabled", Label: "Extended thinking", Category: "Reasoning", Kind: Boolean, Description: "Enable extended thinking by default on supported models.", Purpose: "Improve complex reasoning when extra latency and token use are acceptable."},
	{ID: "auto-compact", Path: "autoCompactEnabled", Label: "Auto compact", Category: "Reasoning", Kind: Boolean, Description: "Summarize and compact a conversation automatically near its context limit.", Purpose: "Prevent long sessions from stopping at the limit, with the trade-off that a summary can omit detail."},

	{ID: "agent-teams", Path: "env.CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS", Label: "Agent teams", Category: "Agents", Kind: Enum, Options: []string{"0", "1"}, Description: "Enable the experimental feature that lets several Claude agents coordinate on one task.", Purpose: "Use it when independent parts of a task can run in parallel; experimental behavior can still change."},
	{ID: "teammate-mode", Path: "teammateMode", Label: "Teammate display", Category: "Agents", Kind: Enum, Options: []string{"in-process", "auto", "tmux", "iterm2"}, Description: "Choose where agent-team teammates are displayed.", Purpose: "Keep agents inline for a simple terminal, or use separate panes when you want to monitor each teammate."},
	{ID: "agent-view", Path: "disableAgentView", Label: "Disable agent view", Category: "Agents", Kind: Boolean, Description: "Disable background agents, the agent view, and the on-demand supervisor.", Purpose: "Use this only when the background-agent interface is distracting or incompatible with your terminal."},

	{ID: "permission-mode", Path: "permissions.defaultMode", Label: "Default permission mode", Category: "Permissions", Kind: Enum, Options: []string{"default", "acceptEdits", "auto", "plan", "dontAsk", "bypassPermissions", "delegate"}, Description: "Default tool-approval behavior for new sessions.", Purpose: "Balance speed and safety by deciding when Claude must ask before it acts.", Danger: map[string]string{"bypassPermissions": "This skips permission prompts and can allow destructive actions."}},
	{ID: "permission-allow", Path: "permissions.allow", Label: "Allow rules", Category: "Permissions", Kind: List, Description: "Tool rules that run without a confirmation prompt. Lists merge across scopes.", Purpose: "Reduce prompt fatigue for commands and paths you already trust."},
	{ID: "permission-ask", Path: "permissions.ask", Label: "Ask rules", Category: "Permissions", Kind: List, Description: "Tool rules that always require your confirmation. Lists merge across scopes.", Purpose: "Keep sensitive actions visible even when a broader rule would otherwise allow them."},
	{ID: "permission-deny", Path: "permissions.deny", Label: "Deny rules", Category: "Permissions", Kind: List, Description: "Tool rules Claude Code must always reject. Lists merge across scopes.", Purpose: "Block commands, tools, or paths that Claude must never access."},

	{ID: "sandbox", Path: "sandbox.enabled", Label: "Sandbox", Category: "Safety", Kind: Boolean, Description: "Run supported shell commands inside Claude Code's filesystem and network isolation.", Purpose: "Limit the damage an unexpected shell command can do to the machine.", Danger: map[string]string{"false": "Disabling the sandbox removes an important command-isolation layer."}},
	{ID: "sandbox-auto-allow", Path: "sandbox.autoAllowBashIfSandboxed", Label: "Auto-allow sandboxed Bash", Category: "Safety", Kind: Boolean, Description: "Automatically approve Bash when Claude Code can run the command inside the sandbox.", Purpose: "Keep isolated workflows fast without removing the isolation boundary."},
	{ID: "sandbox-unsandboxed", Path: "sandbox.allowUnsandboxedCommands", Label: "Allow unsandboxed commands", Category: "Safety", Kind: Boolean, Description: "Allow a command to request execution outside the sandbox.", Purpose: "Needed only for tools that cannot work in isolation; enabling it gives those commands broader system access.", Danger: map[string]string{"true": "Unsandboxed commands can access resources outside the isolation boundary."}},
	{ID: "checkpointing", Path: "fileCheckpointingEnabled", Label: "File checkpointing", Category: "Safety", Kind: Boolean, Description: "Snapshot files edited by Claude so /rewind can restore an earlier state.", Purpose: "Make mistaken edits recoverable without depending on a Git commit."},

	{ID: "ui-language", Path: "@app.language", Label: "Interface language", Category: "Interface", Kind: Enum, Options: []string{"auto", "en", "ru", "zh-CN"}, Description: "Language used by Claude Configurator. Auto follows the operating system language.", Purpose: "Make the configurator readable without maintaining a separate language setting on every machine.", App: true},
	{ID: "theme", Path: "theme", Label: "Theme", Category: "Interface", Kind: Enum, Options: []string{"auto", "dark", "light", "dark-daltonized", "light-daltonized", "dark-ansi", "light-ansi"}, Description: "Built-in Claude Code color theme. Auto follows the terminal background; ANSI variants use the terminal palette.", Purpose: "Keep text legible and choose color-blind-friendly or terminal-native colors when needed."},
	{ID: "tui", Path: "tui", Label: "TUI renderer", Category: "Interface", Kind: Enum, Options: []string{"fullscreen", "default"}, Description: "Choose the fullscreen or classic Claude Code terminal renderer.", Purpose: "Fullscreen reduces flicker; the classic renderer keeps compatibility with normal terminal scrollback."},
	{ID: "view", Path: "viewMode", Label: "Transcript view", Category: "Interface", Kind: Enum, Options: []string{"default", "verbose", "focus"}, Description: "Control how much tool and agent detail is shown in the transcript.", Purpose: "Reduce noise for focused work or expose more detail while debugging agent behavior."},
	{ID: "output-style", Path: "outputStyle", Label: "Output style", Category: "Interface", Kind: String, Options: []string{"default", "Explanatory", "Learning"}, Description: "Built-in or custom style that changes how Claude presents its answers.", Purpose: "Choose concise output, more explanation, or a learning-oriented response without changing the model."},
	{ID: "editor-mode", Path: "editorMode", Label: "Editor mode", Category: "Interface", Kind: Enum, Options: []string{"normal", "vim"}, Description: "Choose normal input bindings or Vim-style modal editing.", Purpose: "Use Vim mode when its navigation matches your muscle memory."},
	{ID: "language", Path: "language", Label: "Claude response language", Category: "Interface", Kind: String, Description: "Preferred language for Claude responses, dictation, and terminal session titles.", Purpose: "Avoid repeating the same language instruction in every prompt."},
	{ID: "reduced-motion", Path: "prefersReducedMotion", Label: "Reduced motion", Category: "Interface", Kind: Boolean, Description: "Reduce non-essential spinners, shimmer, and terminal animation.", Purpose: "Improve accessibility and make remote or slow terminals feel calmer."},

	{ID: "auto-memory", Path: "autoMemoryEnabled", Label: "Auto memory", Category: "Behavior", Kind: Boolean, Description: "Allow Claude Code to save useful project context automatically.", Purpose: "Carry project knowledge into later sessions without explaining it again."},
	{ID: "git-instructions", Path: "includeGitInstructions", Label: "Git instructions", Category: "Behavior", Kind: Boolean, Description: "Include Claude Code's built-in commit and pull-request workflow guidance.", Purpose: "Keep it unless the project already supplies its own Git workflow rules."},
	{ID: "updates", Path: "autoUpdatesChannel", Label: "Update channel", Category: "Behavior", Kind: Enum, Options: []string{"stable", "latest"}, Description: "Choose the Claude Code automatic update channel.", Purpose: "Stable favors predictability; latest delivers new features and fixes sooner."},
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
