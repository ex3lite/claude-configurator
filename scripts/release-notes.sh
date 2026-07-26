#!/bin/sh
set -eu

tag="${1:-}"
repository="https://github.com/ex3lite/claude-configurator"

if ! printf '%s\n' "$tag" |
  grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$'; then
  echo "Expected a semantic version tag such as v0.7.0, got: ${tag:-<empty>}" >&2
  exit 1
fi

highlights="$(
  awk -v tag="$tag" '
    $1 == "##" && $2 == tag { found = 1; next }
    found && $1 == "##" { exit }
    found && !printed && NF == 0 { next }
    found { print; printed = 1 }
    END { if (!found) exit 2 }
  ' CHANGELOG.md
)" || {
  echo "CHANGELOG.md has no section for ${tag}" >&2
  exit 1
}
if [ -z "$highlights" ]; then
  echo "CHANGELOG.md section for ${tag} is empty" >&2
  exit 1
fi

cat <<EOF
## Highlights

${highlights}

## Install

**macOS / Linux**

\`\`\`sh
curl -fsSL https://raw.githubusercontent.com/ex3lite/claude-configurator/main/scripts/install.sh | sh
\`\`\`

**Windows PowerShell**

\`\`\`powershell
irm https://raw.githubusercontent.com/ex3lite/claude-configurator/main/scripts/install.ps1 | iex
\`\`\`

## Upgrade

Restart Claude Configurator and accept the verified update, or run the installer
again. Existing Claude Code settings are preserved.

## Screenshots

![Claude Configurator ${tag}](https://raw.githubusercontent.com/ex3lite/claude-configurator/${tag}/docs/screenshots/en/tui-main.png)

## Verification

- Tests, vet, and a native CLI build passed on macOS, Linux, and Windows.
- Every published archive passed checksum, \`--version\`, and \`--help\` smoke checks.
- SHA-256 values are available in [\`checksums.txt\`](${repository}/releases/download/${tag}/checksums.txt).
EOF
