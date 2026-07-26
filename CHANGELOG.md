# Changelog

## v0.5.0 — 2026-07-26

- Added dedicated 5-hour and 7-day limit rows with progress bars, used and
  remaining percentages, reset dates converted to the device's local time
  without a redundant timezone suffix, and localized countdowns.
- Added a dedicated model-role row for the live primary model plus effective
  subagent and advisor models inherited from global, project, and local scopes.
- Added explicit waiting text when Claude has not supplied `rate_limits`;
  missing windows never receive invented values or dates.
- Added automatic Nerd Font detection and a Claude icon status-line theme.
- Added optional, checksum-verified MesloLGS Nerd Font installation to the
  macOS, Linux, and Windows installers.
- Replaced mock documentation artwork with reproducible VHS screenshots of the
  real TUI, model picker, status-line renderer, and a privacy-safe live Claude
  Code input bar.
- Added automatic screenshot refreshes on `main`, with a color regression
  check and an optional live Claude Code capture when the CLI is installed.
- Strengthened the Claude clay, green, and violet palette and added a full-row
  selection state so the TUI no longer reads as monochrome.
- Added separate English, Russian, and Simplified Chinese screenshot sets so
  every localized README displays the matching interface language.

## v0.4.0 — 2026-07-26

- Added a localized startup check for the latest stable GitHub Release.
- Added explicit consent before download or replacement.
- Added platform-specific archive selection and SHA-256 verification against
  the release `checksums.txt`.
- Added safe executable replacement and automatic restart on macOS, Linux, and
  Windows.
- Added `--no-update` and `CLAUDE_CONFIG_NO_UPDATE=1` opt-outs; `--help`,
  `--version`, and development builds remain network-free.
- Added typed controls for nested-agent depth, total and concurrent subagent
  limits, read-only tool concurrency, interactive `/init`, and shared task
  lists.
- Added a built-in Claude-style status-line renderer with Auto, Claude
  true-color, ANSI, and monochrome themes.
- Added live model, project, Git branch, context remaining, 5-hour/7-day
  allowance, and reset countdowns from Claude Code's official status JSON.

## v0.3.0 — 2026-07-23

- Reworked the TUI around Claude Code's warm clay palette with clearer
  title/subtitle hierarchy and responsive spacing.
- Added persistent Save and hotkey actions; status messages no longer hide the
  controls.
- Added explicit **Default / inherit** choices and visible reset actions that
  remove the current scope's key.
- Added the current `fable` alias to every model picker, including subagents.
- Replaced manual theme input with built-in and discovered custom-theme
  pickers.
- Added localized “what it controls / why you may need it” explanations for
  every setting.

## v0.2.0 — 2026-07-23

- Hierarchical main menu: Enter opens a category; Esc or Left returns.
- Model pickers with stable aliases, Fable/Sonnet presets, and explicit custom
  provider IDs.
- Auto-localized English, Russian, and Simplified Chinese TUI with a persistent
  interface-language preference.
- Refined responsive layout, scope controls, breadcrumbs, panels, and model
  selection modal.

## v0.1.0 — 2026-07-23

- Interactive global, project, and local settings editor.
- Main, subagent, advisor, and fallback model controls.
- Reasoning, agents, permissions, sandbox, interface, and behavior settings.
- Staged diff, safe writes, conflict detection, backups, and Git worktree support.
- macOS, Linux, and Windows release binaries.
