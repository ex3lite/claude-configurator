#!/bin/sh
set -eu

repo_dir="$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"
cd "$repo_dir"

if ! command -v vhs >/dev/null 2>&1; then
  echo "vhs is required: https://github.com/charmbracelet/vhs" >&2
  exit 1
fi
if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required to build the official status-line JSON fixture." >&2
  exit 1
fi
if ! command -v fc-list >/dev/null 2>&1; then
  echo "fontconfig is required to verify the screenshot font." >&2
  exit 1
fi

screenshot_font=""
for candidate in \
  "MesloLGS Nerd Font Mono" \
  "UbuntuMono Nerd Font Mono" \
  "UbuntuSansMono Nerd Font Mono"; do
  if fc-list : family | grep -Fqi "$candidate"; then
    screenshot_font="$candidate"
    break
  fi
done
if [ -z "$screenshot_font" ]; then
  echo "A supported Nerd Font is required. Run the installer and accept MesloLGS Nerd Font." >&2
  exit 1
fi

demo_version="$(awk '/^## v/{print $2; exit}' CHANGELOG.md)"
go build -ldflags "-s -w -X main.version=${demo_version}" -o bin/claude-config ./cmd/claude-config

tui_tape="$(mktemp)"
status_tape="$(mktemp)"
trap 'rm -f "$tui_tape" "$status_tape"' EXIT INT TERM
sed "s|__NERD_FONT__|${screenshot_font}|g" docs/tui.tape >"$tui_tape"
sed "s|__NERD_FONT__|${screenshot_font}|g" docs/statusline.tape >"$status_tape"

record() {
  tape="$1"
  shift
  attempt=1
  while [ "$attempt" -le 3 ]; do
    if vhs "$tape"; then
      sleep 1
      ready=1
      for screenshot in "$@"; do
        [ -s "$screenshot" ] || ready=0
      done
      [ "$ready" -eq 1 ] && return 0
    fi
    attempt=$((attempt + 1))
  done
  echo "VHS did not generate all expected screenshots after 3 attempts." >&2
  return 1
}

rm -f docs/tui-main.png docs/tui-models.png docs/statusline-limits.png
record "$tui_tape" docs/tui-main.png docs/tui-models.png
record "$status_tape" docs/statusline-limits.png
for screenshot in docs/tui-main.png docs/tui-models.png docs/statusline-limits.png; do
  if [ ! -s "$screenshot" ]; then
    echo "Screenshot was not generated: ${screenshot}" >&2
    exit 1
  fi
done

warm_pixels() {
  ffmpeg -v error -i "$1" -f rawvideo -pix_fmt rgb24 - |
    hexdump -v -e '3/1 "%u " "\n"' |
    awk '($1 > 150 && $2 >= 50 && $2 <= 170 && $3 < 140 && $1-$2 > 35) { count++ } END { print count+0 }'
}

green_pixels() {
  ffmpeg -v error -i "$1" -f rawvideo -pix_fmt rgb24 - |
    hexdump -v -e '3/1 "%u " "\n"' |
    awk '($2 > 110 && $2-$1 > 10 && $2-$3 > 0) { count++ } END { print count+0 }'
}

if [ "${CLAUDE_CONFIG_CAPTURE_LIVE:-auto}" != "0" ] && command -v claude >/dev/null 2>&1; then
  if ! "${repo_dir}/scripts/update-live-screenshot.sh" "$screenshot_font"; then
    echo "Live Claude Code screenshot was not refreshed; keeping the last verified image." >&2
  fi
fi
for screenshot in docs/tui-main.png docs/statusline-limits.png docs/claude-cli-statusline.png; do
  if [ ! -s "$screenshot" ] ||
    [ "$(warm_pixels "$screenshot")" -lt 500 ]; then
    echo "Screenshot color check failed: Claude orange is missing from ${screenshot}." >&2
    exit 1
  fi
done
for screenshot in docs/statusline-limits.png docs/claude-cli-statusline.png; do
  if [ "$(green_pixels "$screenshot")" -lt 500 ]; then
    echo "Screenshot color check failed: status green is missing from ${screenshot}." >&2
    exit 1
  fi
done
echo "Updated docs/tui-main.png, docs/tui-models.png, and docs/statusline-limits.png using ${screenshot_font}."
