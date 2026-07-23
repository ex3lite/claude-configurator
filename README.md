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
- Reasoning, agents, permissions, sandbox, interface, and behavior settings.
- Auto-localized TUI in English, Russian, or Simplified Chinese.
- Search, inherited-value/source display, staged changes, and diff before save.
- Conflict detection, automatic backups, and protection against invalid JSON.
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

The picker includes Claude Code's stable `default`, `best`, `sonnet`, `opus`,
`haiku`, 1M-context, and `opusplan` aliases. **Custom model ID…** remains the
last explicit option for gateways and provider-specific deployments; ordinary
model selection never opens a string field. See the
[official model configuration](https://code.claude.com/docs/en/model-config).

### Interface language

The TUI starts in **Auto** mode and follows the operating system language.
Open **Interface → Interface language** to choose Auto, English, Русский, or
简体中文. This preference is saved in the operating system's user configuration
directory for Claude Configurator and is not written to Claude Code settings.

### Keyboard

| Key | Action |
|---|---|
| `↑/↓`, `j/k` | Select an item in the current screen |
| `Enter` | Open a category or edit a setting |
| `Esc`, `←` | Return to the main menu |
| `g`, `p`, `l` | Global, project, local scope |
| `Space` | Toggle a boolean |
| `/` | Search |
| `u` | Unset and inherit |
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
- No telemetry, analytics, account access, or runtime network requests.

## Troubleshooting

- Settings are not active: restart Claude Code and check `/status`; managed
  settings or command-line flags may have higher precedence.
- Startup reports invalid JSON: fix the exact location printed by
  `claude-config`, then run `claude doctor`.
- A save is blocked: another process changed the file; press `r`, review the
  new value, and apply your change again.
- Colors are unwanted: start with `NO_COLOR=1 claude-config`.

## Development

Requires Go 1.25+.

```sh
go test -race ./...
go vet ./...
go run ./cmd/claude-config
```

Claude Configurator is an independent community project and is not affiliated
with or endorsed by Anthropic. Claude is a trademark of Anthropic.
