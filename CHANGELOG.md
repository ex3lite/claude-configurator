# Changelog

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
