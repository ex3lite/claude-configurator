#!/bin/sh
set -eu

repo_dir="$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"
input="${repo_dir}/docs/screenshots/en/tui-main.png"
output="${repo_dir}/docs/social-preview.png"

if ! command -v ffmpeg >/dev/null 2>&1; then
  echo "ffmpeg is required to render the social preview." >&2
  exit 1
fi
if [ ! -s "$input" ]; then
  echo "English TUI screenshot not found: ${input}" >&2
  exit 1
fi

ffmpeg -y -v error -i "$input" \
  -vf "crop=1400:700:0:0,scale=1280:640:flags=lanczos" \
  -frames:v 1 "$output"
test -s "$output"
