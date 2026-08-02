#!/bin/sh

set -eu

repository="shurrman/vault-manager"

fail() {
	printf 'vault-manager installer: %s\n' "$*" >&2
	exit 1
}

command -v curl >/dev/null 2>&1 || fail "curl is required"
command -v tar >/dev/null 2>&1 || fail "tar is required"

tag=${VAULT_MANAGER_VERSION:-}
if [ -z "$tag" ]; then
	latest_url=$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/${repository}/releases/latest")
	tag=${latest_url##*/}
fi
case "$tag" in
	v*) ;;
	*) tag="v${tag}" ;;
esac
printf '%s\n' "$tag" | grep -Eq '^v[0-9]+\.[0-9]+$' || fail "invalid release version: ${tag}"
version=${tag#v}

case "$(uname -s)" in
	Linux) os=linux ;;
	Darwin) os=darwin ;;
	*) fail "unsupported operating system: $(uname -s)" ;;
esac

case "$(uname -m)" in
	x86_64 | amd64) arch=amd64 ;;
	aarch64 | arm64) arch=arm64 ;;
	*) fail "unsupported architecture: $(uname -m)" ;;
esac

asset="vault-manager_${version}_${os}_${arch}.tar.gz"
base_url=${VAULT_MANAGER_DOWNLOAD_BASE_URL:-"https://github.com/${repository}/releases/download/${tag}"}
install_dir=${VAULT_MANAGER_INSTALL_DIR:-}
if [ -z "$install_dir" ]; then
	if [ -d /usr/local/bin ] && [ -w /usr/local/bin ]; then
		install_dir=/usr/local/bin
	else
		install_dir=${HOME:?HOME is required}/.local/bin
	fi
fi

temp_dir=$(mktemp -d 2>/dev/null || mktemp -d -t vault-manager)
cleanup() {
	rm -rf "$temp_dir"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM

archive="${temp_dir}/${asset}"
checksums="${temp_dir}/checksums.txt"
curl -fsSL "${base_url}/${asset}" -o "$archive"
curl -fsSL "${base_url}/checksums.txt" -o "$checksums"

expected=$(awk -v asset="$asset" '$2 == asset { print $1; exit }' "$checksums")
[ -n "$expected" ] || fail "${asset} is missing from checksums.txt"
if command -v sha256sum >/dev/null 2>&1; then
	actual=$(sha256sum "$archive" | awk '{ print $1 }')
elif command -v shasum >/dev/null 2>&1; then
	actual=$(shasum -a 256 "$archive" | awk '{ print $1 }')
else
	fail "sha256sum or shasum is required"
fi
[ "$actual" = "$expected" ] || fail "SHA-256 verification failed for ${asset}"

tar -xzf "$archive" -C "$temp_dir"
[ -f "${temp_dir}/vault-manager" ] || fail "archive does not contain vault-manager"
mkdir -p "$install_dir"
if command -v install >/dev/null 2>&1; then
	install -m 0755 "${temp_dir}/vault-manager" "${install_dir}/vault-manager"
else
	cp "${temp_dir}/vault-manager" "${install_dir}/vault-manager"
	chmod 0755 "${install_dir}/vault-manager"
fi

printf 'Installed vault-manager %s to %s/vault-manager\n' "$version" "$install_dir"
case ":${PATH}:" in
	*":${install_dir}:"*) ;;
	*) printf 'Add %s to PATH to run vault-manager by name.\n' "$install_dir" ;;
esac
