#!/bin/sh
set -eu

repo_dir="$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"
font="${1:-MesloLGS Nerd Font Mono}"

for command in claude vhs ffmpeg git; do
  if ! command -v "$command" >/dev/null 2>&1; then
    echo "Skipping live Claude Code screenshot: ${command} is not installed." >&2
    exit 2
  fi
done

capture_root="$(mktemp -d)"
trap 'rm -rf "$capture_root"' EXIT INT TERM

capture_locale() {
  language="$1"
  locale="$2"
  limits_wait="$3"
  locale_root="${capture_root}/${language}"
  demo_dir="${locale_root}/demo-project"
  output_dir="${repo_dir}/docs/screenshots/${language}"
  tape="${locale_root}/claude-cli.tape"
  full="${locale_root}/full.png"
  cropped="${locale_root}/cropped.png"
  vhs_log="${locale_root}/vhs.log"
  mkdir -p "${demo_dir}/bin" "${demo_dir}/.claude" "$output_dir"
  git -C "$demo_dir" init -q -b main
  cp "${repo_dir}/docs/demo-settings.json" "${demo_dir}/.claude/settings.json"
  sed '/"advisorModel": "opus",/d' \
    "${repo_dir}/docs/demo-settings.json" >"${demo_dir}/settings.json"
  cp "${repo_dir}/bin/claude-config" "${demo_dir}/bin/claude-config"

  sed \
    -e "s|__NERD_FONT__|${font}|g" \
    -e "s|__LOCALE__|${locale}|g" \
    -e "s|__LIMITS_WAIT__|${limits_wait}|g" \
    -e "s|__OUTPUT_GIF__|${locale_root}/capture.gif|g" \
    -e "s|__DEMO_DIR__|${demo_dir}|g" \
    -e "s|__BINARY_DIR__|${demo_dir}/bin|g" \
    -e "s|__SETTINGS_FILE__|${demo_dir}/settings.json|g" \
    -e "s|__FULL_SCREENSHOT__|${full}|g" \
    "${repo_dir}/docs/claude-cli.tape" >"$tape"

  if ! vhs "$tape" >"$vhs_log" 2>&1; then
    sed -E \
      -e 's/[[:alnum:]._%+-]+@[[:alnum:].-]+/[redacted-email]/g' \
      -e 's#(/Users/)[^/]+#/Users/[redacted]#g' \
      "$vhs_log" >&2
    return 1
  fi
  test -s "$full"
  ffmpeg -y -v error -i "$full" \
    -vf "crop=iw:165:0:ih-236,pad=iw:ih+32:0:0:color=0x1c1e1b" \
    -frames:v 1 "$cropped"
  test -s "$cropped"
  mv "$cropped" "${output_dir}/claude-cli-statusline.png"
}

capture_locale en "en_US.UTF-8" "limits:"
capture_locale ru "ru_RU.UTF-8" "лимиты:"
capture_locale zh-CN "zh_CN.UTF-8" "限额："
echo "Updated localized live Claude Code screenshots in isolated demo-project repositories."
