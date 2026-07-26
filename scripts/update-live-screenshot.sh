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
demo_dir="${capture_root}/demo-project"
mkdir -p "${demo_dir}/bin"
git -C "$demo_dir" init -q -b main
cp "${repo_dir}/docs/demo-settings.json" "${demo_dir}/settings.json"
cp "${repo_dir}/bin/claude-config" "${demo_dir}/bin/claude-config"

tape="${capture_root}/claude-cli.tape"
full="${capture_root}/full.png"
cropped="${capture_root}/cropped.png"
vhs_log="${capture_root}/vhs.log"
sed \
  -e "s|__NERD_FONT__|${font}|g" \
  -e "s|__OUTPUT_GIF__|${capture_root}/capture.gif|g" \
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
  exit 1
fi
test -s "$full"
ffmpeg -y -v error -i "$full" \
  -vf "crop=iw:265:0:ih-265,pad=iw:ih+32:0:0:color=0x1f1f1d" \
  -frames:v 1 "$cropped"
test -s "$cropped"
mv "$cropped" "${repo_dir}/docs/claude-cli-statusline.png"
echo "Updated docs/claude-cli-statusline.png from a real Claude Code session in demo-project."
