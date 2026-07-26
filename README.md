# Claude Configurator

[English](README.md) · [Русский](README.ru.md) · [简体中文](README.zh-CN.md)

[![CI](https://github.com/ex3lite/claude-configurator/actions/workflows/ci.yml/badge.svg)](https://github.com/ex3lite/claude-configurator/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/ex3lite/claude-configurator)](https://github.com/ex3lite/claude-configurator/releases)
[![License: MIT](https://img.shields.io/badge/license-MIT-22c55e.svg)](LICENSE)

A fast terminal UI for editing Claude Code settings globally or per project. It
works locally, does not use prompts, and never sends your configuration
anywhere.

![Claude Configurator TUI](docs/screenshot.svg)

## Features

- Global, shared project, and local project scopes.
- Pickers for main, subagent, advisor, and fallback models; no raw typing for
  normal model selection.
- Current `fable`, `best`, `sonnet`, `opus`, and `haiku` aliases, plus an
  explicit **Default / inherit** action in every scoped picker.
- Reasoning, agents, permissions, sandbox, interface, and behavior settings.
- Typed controls for nested-agent depth, total/concurrent subagent caps, tool
  concurrency, interactive `/init`, and shared task lists.
- Claude-style status bar themes with live context, 5-hour/7-day allowance,
  and reset countdowns.
- Auto-localized TUI in English, Russian, or Simplified Chinese.
- Claude Code-inspired warm palette, clear title/subtitle hierarchy, and
  plain-language “what it controls / why you may need it” explanations.
- Persistent action bar with Save, hotkeys, inherited-value/source display,
  staged changes, and diff before save.
- Conflict detection, automatic backups, and protection against invalid JSON.
- Consent-based self-updates from verified GitHub Release assets.
- Git repository and worktree-aware paths.
- One native binary for macOS, Linux, and Windows.

## Install

### macOS or Linux

```sh
curl -fsSL https://raw.githubusercontent.com/ex3lite/claude-configurator/main/scripts/install.sh | sh
```

The installer verifies the release checksum and installs `claude-config`,
`claude-configurator`, and `ccfg` into `~/.local/bin`.

### Windows PowerShell

```powershell
irm https://raw.githubusercontent.com/ex3lite/claude-configurator/main/scripts/install.ps1 | iex
```

### Go

```sh
go install github.com/ex3lite/claude-configurator/cmd/claude-config@latest
```

Prebuilt archives and checksums are also available on the
[Releases page](https://github.com/ex3lite/claude-configurator/releases).

## Use

```text
claude-config
claude-config --scope global|project|local
claude-config --project /path/to/project
claude-config --no-update
claude-config statusline --theme auto|claude|ansi|mono
claude-config --help
claude-config --version
```

### Scopes

| Scope | File | Purpose |
|---|---|---|
| Global | `~/.claude/settings.json` | Your defaults for every project |
| Project | `.claude/settings.json` | Shared repository settings |
| Local | `.claude/settings.local.json` | Personal repository overrides |

Claude Code precedence is managed settings → CLI overrides → local → project →
global. Claude Configurator edits the last three levels and never changes
managed policy.

### Fable main model with Sonnet subagents

Select the global scope with `g`, open **Models**, and choose **Fable 5 · 1M**
for the main model and **Sonnet 5** for subagents. The resulting settings are:

```json
{
  "model": "claude-fable-5[1m]",
  "env": {
    "CLAUDE_CODE_SUBAGENT_MODEL": "claude-sonnet-5"
  }
}
```

Use `p` instead to apply the same values only to the current project. The
subagent setting applies to all subagents, agent teams, and workflow agents,
and overrides per-agent model choices. Restart already-running Claude Code
sessions after saving.

The picker includes Claude Code's current `default`, `best`, `fable`, `sonnet`,
`opus`, `haiku`, 1M-context, and `opusplan` aliases. Fable appears both as the
recommended `fable` alias and as a pinned preset. **Custom model ID…** remains
the last explicit option for gateways and provider-specific deployments;
ordinary model selection never opens a string field. See the
[official model configuration](https://code.claude.com/docs/en/model-config).

### Inheritance, reset, and save

The first item in a scoped picker is **Default / inherit**. It removes that
JSON key from the selected scope, so the value falls back to project, global,
managed, or Claude's own default resolution. It does not write a fake string.
The same action is always visible as **[U] Reset to inherit** in the details
panel and action bar.

Changes remain staged until **[S] Save**. Save is always visible, shows the
number of changed settings, and opens a diff before writing the file.

### Claude Code theme

Open **Interface → Theme** to choose Auto, dark, light, daltonized, or terminal
ANSI themes from a picker. Existing custom themes from `~/.claude/themes/*.json`
are added to the same list automatically; theme selection does not require
typing a string.

### Claude CLI controls

Open **Claude CLI** to configure the current
[official environment controls](https://code.claude.com/docs/en/env-vars):

| Setting | Claude default | Requirement |
|---|---:|---|
| Nested subagent depth | `1` | Claude Code 2.1.217+ |
| Total subagents per session | `200` | Claude Code 2.1.212+ |
| Concurrent subagents | `20` | Claude Code 2.1.217+ |
| Read-only tool/subagent concurrency | `10` | Current Claude Code |
| Interactive `/init` | Off | Enable with `CLAUDE_CODE_NEW_INIT=1` |
| Shared task list | Separate | Give sessions the same task-list ID |

Numeric settings use validated presets with an explicit custom-number option.
They are stored as strings under `env`, as Claude Code expects. Resetting a
setting removes the current scope's environment key and restores inheritance.

### Claude-style status bar

Choose **Claude CLI → Status bar theme** to install the built-in renderer:

![Claude-style status bar](docs/statusline.svg)

```json
{
  "statusLine": {
    "type": "command",
    "command": "claude-config statusline --theme auto",
    "refreshInterval": 60
  }
}
```

The bar uses Claude Code's
[official status-line JSON](https://code.claude.com/docs/en/statusline) rather
than scraping the terminal. It shows the model, project, Git branch, remaining context, local
time, and real 5-hour/7-day remaining allowance with time until reset. A second
line adds session, agent, effort, thinking, fast, Vim, and output-style state
when present.

Themes are **Auto**, Claude clay true color, terminal ANSI, and monochrome.
Auto honors `NO_COLOR`. The layout drops secondary segments first on narrow
terminals. Rate-limit fields are available only to Claude.ai Pro/Max
subscribers after the first API response; missing windows are simply omitted.
Resetting **Status bar theme** removes the entire `statusLine` override from
the selected scope so the lower scope is inherited.

### Interface language

The TUI starts in **Auto** mode and follows the operating system language.
Open **Interface → Interface language** to choose Auto, English, Русский, or
简体中文. This preference is saved in the operating system's user configuration
directory for Claude Configurator and is not written to Claude Code settings.

### Automatic updates

Release binaries check the latest stable
[GitHub Release](https://github.com/ex3lite/claude-configurator/releases) when
the TUI starts. If a newer version exists, a localized dialog asks before
anything is downloaded. Accepting it downloads only the archive for the
current operating system and `checksums.txt`, verifies SHA-256, replaces the
running binary safely, and restarts Claude Configurator. Choosing **Later**
continues with the installed version.

The updater follows published releases, never the repository's `main` branch.
`--help` and `--version` do not access the network. Use `--no-update` for one
launch or set `CLAUDE_CONFIG_NO_UPDATE=1` to disable checks in scripts and
offline environments. Builds produced by `go install` report `dev` and stay
under Go's package-management flow instead of replacing themselves.

### Keyboard

| Key | Action |
|---|---|
| `↑/↓`, `j/k` | Select an item in the current screen |
| `Enter` | Open a category or edit a setting |
| `Esc`, `←` | Return to the main menu |
| `g`, `p`, `l` | Global, project, local scope |
| `Space` | Toggle a boolean |
| `/` | Search |
| `u` | Remove this scope's value and inherit |
| `s` | Review diff and save |
| `r` | Reload from disk |
| `?` | Help |
| `q` | Quit |

## Safety and privacy

- Existing files are checked again before save; external changes block the
  write instead of being overwritten.
- Invalid JSON is never replaced. The error includes the file, line, and
  column.
- Existing unknown settings are preserved.
- Backups are stored under the operating system's user cache directory in
  `claude-configurator/backups`; the latest 10 backups per file are retained.
- Global and local files default to owner-only permissions.
- Dangerous settings such as `bypassPermissions` require a second
  confirmation.
- No telemetry, analytics, or account access. The only automatic network
  request is the startup check for public GitHub Release metadata; settings
  files and their values never leave the computer.

## Troubleshooting

- Settings are not active: restart Claude Code and check `/status`; managed
  settings or command-line flags may have higher precedence.
- Startup reports invalid JSON: fix the exact location printed by
  `claude-config`, then run `claude doctor`.
- A save is blocked: another process changed the file; press `r`, review the
  new value, and apply your change again.
- Colors are unwanted: start with `NO_COLOR=1 claude-config`.
- Status limits are absent: make one Claude request first; Claude exposes these
  fields only for supported Claude.ai Pro/Max subscriptions.
- The status bar does not start: ensure `claude-config` is on the `PATH` seen
  by Claude Code, then select the theme again.
- An update cannot be installed: ensure the directory containing
  `claude-config` is writable, or rerun the installation script.

## Development

Requires Go 1.25+.

```sh
go test -race ./...
go vet ./...
go run ./cmd/claude-config
```

Claude Configurator is an independent community project and is not affiliated
with or endorsed by Anthropic. Claude is a trademark of Anthropic.
