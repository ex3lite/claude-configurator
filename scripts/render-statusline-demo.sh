#!/bin/sh
set -eu

repo_dir="$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"
now="$(date +%s)"

jq -n --argjson now "$now" '{
  model: {display_name: "Fable 5"},
  workspace: {
    current_dir: "/tmp/demo-project",
    project_dir: "/tmp/demo-project",
    repo: {name: "demo-project"}
  },
  worktree: {branch: "main"},
  context_window: {remaining_percentage: 72},
  rate_limits: {
    five_hour: {used_percentage: 38, resets_at: ($now + 6120)},
    seven_day: {used_percentage: 21, resets_at: ($now + 327600)}
  },
  effort: {level: "xhigh"},
  thinking: {enabled: true}
}' | COLUMNS="${COLUMNS:-180}" "$repo_dir/bin/claude-config" statusline --theme nerd
