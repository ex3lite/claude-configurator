#!/bin/sh
set -eu

repository="ex3lite/claude-configurator"
requested_version="${1:-latest}"
install_dir="${CLAUDE_CONFIG_INSTALL_DIR:-${HOME}/.local/bin}"

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
if command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "${temp_dir}/${archive}" | awk '{print $1}')"
else
  actual="$(shasum -a 256 "${temp_dir}/${archive}" | awk '{print $1}')"
fi
if [ "$actual" != "$expected" ]; then
  echo "Checksum verification failed" >&2
  exit 1
fi

tar -xzf "${temp_dir}/${archive}" -C "$temp_dir"
mkdir -p "$install_dir"
install -m 0755 "${temp_dir}/claude-config" "${install_dir}/claude-config"
ln -sf claude-config "${install_dir}/claude-configurator"
ln -sf claude-config "${install_dir}/ccfg"

echo "Installed claude-config ${release_tag} to ${install_dir}"
case ":${PATH}:" in
  *":${install_dir}:"*) ;;
  *) echo "Add ${install_dir} to PATH to run claude-config." ;;
esac
