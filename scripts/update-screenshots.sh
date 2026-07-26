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

temp_root="$(mktemp -d)"
trap 'rm -rf "$temp_root"' EXIT INT TERM

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

render_locale() {
  language="$1"
  locale="$2"
  main_wait="$3"
  settings_wait="$4"
  status_wait="$5"
  output_dir="docs/screenshots/${language}"
  demo_root="${temp_root}/demo-${language}"
  tui_tape="${temp_root}/tui-${language}.tape"
  status_tape="${temp_root}/status-${language}.tape"
  mkdir -p "$output_dir"

  sed \
    -e "s|__NERD_FONT__|${screenshot_font}|g" \
    -e "s|__LOCALE__|${locale}|g" \
    -e "s|__MAIN_WAIT__|${main_wait}|g" \
    -e "s|__SETTINGS_WAIT__|${settings_wait}|g" \
    -e "s|__DEMO_ROOT__|${demo_root}|g" \
    -e "s|__OUTPUT_DIR__|${output_dir}|g" \
    -e "s|__OUTPUT_GIF__|${temp_root}/tui-${language}.gif|g" \
    docs/tui.tape >"$tui_tape"
  sed \
    -e "s|__NERD_FONT__|${screenshot_font}|g" \
    -e "s|__LOCALE__|${locale}|g" \
    -e "s|__STATUS_WAIT__|${status_wait}|g" \
    -e "s|__OUTPUT_DIR__|${output_dir}|g" \
    -e "s|__OUTPUT_GIF__|${temp_root}/status-${language}.gif|g" \
    docs/statusline.tape >"$status_tape"

  rm -f "${output_dir}/tui-main.png" "${output_dir}/tui-models.png" "${output_dir}/statusline-limits.png"
  record "$tui_tape" "${output_dir}/tui-main.png" "${output_dir}/tui-models.png"
  record "$status_tape" "${output_dir}/statusline-limits.png"
}

render_locale en "en_US.UTF-8" "MAIN MENU" "SETTINGS" "used"
render_locale ru "ru_RU.UTF-8" "ГЛАВНОЕ МЕНЮ" "НАСТРОЙКИ" "исп."
render_locale zh-CN "zh_CN.UTF-8" "主菜单" "设置" "已用"
rm -f docs/tui-main.png docs/tui-models.png docs/statusline-limits.png docs/claude-cli-statusline.png

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
for language in en ru zh-CN; do
  output_dir="docs/screenshots/${language}"
  for screenshot in \
    "${output_dir}/tui-main.png" \
    "${output_dir}/tui-models.png" \
    "${output_dir}/statusline-limits.png" \
    "${output_dir}/claude-cli-statusline.png"; do
    if [ ! -s "$screenshot" ]; then
      echo "Screenshot was not generated: ${screenshot}" >&2
      exit 1
    fi
  done
  for screenshot in \
    "${output_dir}/tui-main.png" \
    "${output_dir}/statusline-limits.png" \
    "${output_dir}/claude-cli-statusline.png"; do
    if [ "$(warm_pixels "$screenshot")" -lt 500 ]; then
      echo "Screenshot color check failed: Claude orange is missing from ${screenshot}." >&2
      exit 1
    fi
  done
  for screenshot in \
    "${output_dir}/statusline-limits.png" \
    "${output_dir}/claude-cli-statusline.png"; do
    if [ "$(green_pixels "$screenshot")" -lt 500 ]; then
      echo "Screenshot color check failed: status green is missing from ${screenshot}." >&2
      exit 1
    fi
  done
done
echo "Updated localized English, Russian, and Simplified Chinese screenshots using ${screenshot_font}."
