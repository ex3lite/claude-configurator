#!/bin/sh
set -eu

repository="ex3lite/claude-configurator"
requested_version="${1:-latest}"
install_dir="${CLAUDE_CONFIG_INSTALL_DIR:-${HOME}/.local/bin}"

has_nerd_font() {
  if command -v fc-list >/dev/null 2>&1 &&
    fc-list : family 2>/dev/null | grep -Eiq 'Nerd Font|NerdFont|(^|[ ,])(NF|NFM|NFP)([ ,]|$)'; then
    return 0
  fi
  for font_dir in \
    "${HOME}/Library/Fonts" \
    "${HOME}/.local/share/fonts" \
    "${HOME}/.local/share/fonts/claude-configurator" \
    "/Library/Fonts"; do
    [ -d "$font_dir" ] || continue
    if find "$font_dir" -type f \( -iname '*NerdFont*' -o -iname '*Nerd Font*' \) -print -quit 2>/dev/null |
      grep -q .; then
      return 0
    fi
  done
  return 1
}

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

install_nerd_font() {
  if ! command -v unzip >/dev/null 2>&1; then
    echo "Cannot install MesloLGS Nerd Font automatically: unzip is required." >&2
    return 1
  fi
  font_release_url="$(curl -fsSL -o /dev/null -w '%{url_effective}' \
    "https://github.com/ryanoasis/nerd-fonts/releases/latest")"
  font_tag="${font_release_url##*/}"
  font_base_url="https://github.com/ryanoasis/nerd-fonts/releases/download/${font_tag}"
  curl -fsSL "${font_base_url}/Meslo.zip" -o "${temp_dir}/Meslo.zip"
  curl -fsSL "${font_base_url}/SHA-256.txt" -o "${temp_dir}/nerd-font-checksums.txt"
  font_expected="$(awk '$2 == "Meslo.zip" {print $1}' "${temp_dir}/nerd-font-checksums.txt")"
  font_actual="$(sha256_file "${temp_dir}/Meslo.zip")"
  if [ -z "$font_expected" ] || [ "$font_actual" != "$font_expected" ]; then
    echo "MesloLGS Nerd Font checksum verification failed." >&2
    return 1
  fi

  font_unpack="${temp_dir}/meslo"
  mkdir -p "$font_unpack"
  unzip -q "${temp_dir}/Meslo.zip" -d "$font_unpack"
  if [ "$target_os" = "darwin" ]; then
    font_install_dir="${HOME}/Library/Fonts"
  else
    font_install_dir="${HOME}/.local/share/fonts/claude-configurator"
  fi
  mkdir -p "$font_install_dir"
  installed=0
  for font in "${font_unpack}"/MesloLGSNerdFontMono-*.ttf; do
    [ -f "$font" ] || continue
    install -m 0644 "$font" "$font_install_dir/"
    installed=1
  done
  if [ "$installed" -ne 1 ]; then
    echo "MesloLGS Nerd Font files were not found in the verified archive." >&2
    return 1
  fi
  if command -v fc-cache >/dev/null 2>&1; then
    fc-cache -f "$font_install_dir" >/dev/null 2>&1 || true
  fi
  echo "Installed MesloLGS Nerd Font Mono to ${font_install_dir}"
  echo "Restart the terminal and select “MesloLGS Nerd Font Mono” as its font."
}

offer_nerd_font() {
  if has_nerd_font; then
    echo "Nerd Font detected. Claude Icons will be unlocked in the status-bar themes."
    return
  fi
  font_choice="${CLAUDE_CONFIG_INSTALL_NERD_FONT:-ask}"
  if [ "$font_choice" = "ask" ] && [ -r /dev/tty ]; then
    printf 'Install the recommended MesloLGS Nerd Font for the Claude Icons theme? [y/N] ' >/dev/tty
    IFS= read -r font_answer </dev/tty || font_answer=""
    case "$font_answer" in
      y|Y|yes|YES) font_choice="1" ;;
      *) font_choice="0" ;;
    esac
  fi
  case "$font_choice" in
    1|true|yes) install_nerd_font ;;
    *)
      echo "Nerd Font not installed. Set CLAUDE_CONFIG_INSTALL_NERD_FONT=1 to install it non-interactively."
      ;;
  esac
}

case "$(uname -s)" in
  Darwin) target_os="darwin" ;;
  Linux) target_os="linux" ;;
  *) echo "Unsupported operating system: $(uname -s)" >&2; exit 1 ;;
esac

case "$(uname -m)" in
  x86_64|amd64) target_arch="amd64" ;;
  arm64|aarch64) target_arch="arm64" ;;
  *) echo "Unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

if [ "$requested_version" = "latest" ]; then
  release_url="$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/${repository}/releases/latest")"
  release_tag="${release_url##*/}"
else
  release_tag="$requested_version"
fi
case "$release_tag" in
  v*) release_version="${release_tag#v}" ;;
  *) release_version="$release_tag"; release_tag="v${release_tag}" ;;
esac

archive="claude-configurator_${release_version}_${target_os}_${target_arch}.tar.gz"
base_url="https://github.com/${repository}/releases/download/${release_tag}"
temp_dir="$(mktemp -d)"
trap 'rm -rf "$temp_dir"' EXIT INT TERM

curl -fsSL "${base_url}/${archive}" -o "${temp_dir}/${archive}"
curl -fsSL "${base_url}/checksums.txt" -o "${temp_dir}/checksums.txt"
expected="$(awk -v file="$archive" '$2 == file {print $1}' "${temp_dir}/checksums.txt")"
if [ -z "$expected" ]; then
  echo "Checksum for ${archive} was not found" >&2
  exit 1
fi
actual="$(sha256_file "${temp_dir}/${archive}")"
if [ "$actual" != "$expected" ]; then
  echo "Checksum verification failed" >&2
  exit 1
fi

tar -xzf "${temp_dir}/${archive}" -C "$temp_dir"
unpacked_version="$("${temp_dir}/claude-config" --version)"
if [ "$unpacked_version" != "$release_version" ]; then
  echo "Downloaded binary reports ${unpacked_version}; expected ${release_version}" >&2
  exit 1
fi

mkdir -p "$install_dir"
install -m 0755 "${temp_dir}/claude-config" "${install_dir}/claude-config"
ln -sf claude-config "${install_dir}/claude-configurator"
ln -sf claude-config "${install_dir}/ccfg"

installed_version="$("${install_dir}/claude-config" --version)"
if [ "$installed_version" != "$release_version" ]; then
  echo "Installed binary verification failed: expected ${release_version}, got ${installed_version}" >&2
  exit 1
fi

echo "Installed and verified claude-config ${release_tag} in ${install_dir}"
offer_nerd_font
case ":${PATH}:" in
  *":${install_dir}:"*) ;;
  *)
    quoted_install_dir="$(printf '%s' "$install_dir" | sed "s/'/'\\\\''/g")"
    echo
    echo "The install directory is not on PATH in this shell."
    echo "Run this exact command now:"
    printf "  export PATH='%s':\"\$PATH\"\n" "$quoted_install_dir"
    echo "Add the same line to your shell profile if you want it to persist."
    ;;
esac
