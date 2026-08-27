#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

eye_user=panoptes-eye
eye_binary=/opt/panoptes/eye
eye_state=/var/lib/panoptes/eye
eye_env=/etc/panoptes/eye.env
eye_unit=/etc/systemd/system/panoptes-eye.service
eye_dropin_dir=/etc/systemd/system/panoptes-eye.service.d
eye_dropin=$eye_dropin_dir/docker.conf
eye_link=/usr/local/bin/eye

die() {
	printf '%s\n' "$*" >&2
	exit 1
}

usage() {
	cat <<'EOF'
usage:
  install.sh [options]

options:
  --endpoint HOST:PORT
  --seal-file PATH
  --epithet NAME
  --binary PATH
  --from-source
  --docker
EOF
}

require_root() {
	if [[ ${EUID} -ne 0 ]]; then
		die "install.sh must be run as root"
	fi
}

require_cmd() {
	command -v "$1" >/dev/null || die "missing command: $1"
}

take_flag() {
	local name=$1
	shift
	if [[ $# -lt 1 ]]; then
		die "missing value for $name"
	fi
	printf '%s' "$1"
}

prompt_if_empty() {
	local name=$1
	local prompt=$2
	local value=${3-}
	if [[ -n $value ]]; then
		printf '%s' "$value"
		return
	fi
	if [[ -t 0 ]]; then
		local typed=
		read -r -p "$prompt" typed
		printf '%s' "$typed"
		return
	fi
	die "$name is required"
}

release_base=https://github.com/Kirragami/panoptes/releases/latest/download

source_version() {
	local ver=dev
	if command -v git >/dev/null && git -C "$root" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
		ver=$(git -C "$root" describe --tags --always --dirty 2>/dev/null || printf 'dev')
	fi
	printf '%s' "$ver"
}

linux_arch() {
	case $(uname -m) in
	x86_64)
		printf 'amd64'
		;;
	aarch64 | arm64)
		printf 'arm64'
		;;
	*)
		die "unsupported architecture: $(uname -m)"
		;;
	esac
}

build_eye() {
	local output=$1
	[[ -f $root/main.go ]] || die "Eye source not found; pass --binary or omit --from-source to download a release"
	require_cmd go
	(
		cd "$root"
		CGO_ENABLED=0 go build -trimpath \
			-ldflags "-s -w -X github.com/Kirragami/panoptes/eye/cli.version=$(source_version)" \
			-o "$output" .
	)
}

download_release_eye() {
	local output=$1
	[[ $(uname -s) == Linux ]] || die "install is only published for linux"
	require_cmd curl
	require_cmd sha256sum
	local arch name sums want got
	arch=$(linux_arch)
	name=eye-linux-$arch
	sums=$workdir/checksums.txt
	curl -fsSL --max-time 60 "$release_base/checksums.txt" -o "$sums"
	curl -fsSL --max-time 60 "$release_base/$name" -o "$output"
	want=$(awk -v asset="$name" '{
		file = $NF
		sub(/^\*/, "", file)
		n = split(file, parts, "/")
		file = parts[n]
		if (file == asset) {
			print $1
			exit
		}
	}' "$sums")
	[[ -n $want ]] || die "checksum for $name was not published"
	got=$(sha256sum "$output" | awk '{print $1}')
	if [[ ${got,,} != "${want,,}" ]]; then
		die "downloaded $name checksum does not match"
	fi
	chmod 0755 "$output"
}

obtain_eye() {
	local output=$1
	if [[ -n $binary ]]; then
		printf '%s' "$binary"
		return
	fi
	if [[ $from_source -eq 1 ]]; then
		build_eye "$output"
		printf '%s' "$output"
		return
	fi
	download_release_eye "$output"
	printf '%s' "$output"
}

install_binary() {
	local source=$1
	local dest=$2
	[[ -f $source ]] || die "binary not found: $source"
	install -d -m 0755 "$(dirname "$dest")"
	install -m 0755 -o root -g root "$source" "$dest"
}

nologin_shell() {
	if [[ -x /usr/sbin/nologin ]]; then
		printf '/usr/sbin/nologin'
		return
	fi
	printf '/sbin/nologin'
}

ensure_user() {
	local name=$1
	local home=$2
	if getent passwd "$name" >/dev/null; then
		return
	fi
	useradd --system --user-group --home-dir "$home" --shell "$(nologin_shell)" "$name"
}

systemd_escape() {
	local value=$1
	value=${value//%/%%}
	value=${value//\$/\$\$}
	printf '%s' "$value"
}

write_env_line() {
	local name=$1
	local value=$2
	if [[ $value == *$'\n'* ]]; then
		die "$name cannot contain a newline"
	fi
	printf '%s=%s\n' "$name" "$(systemd_escape "$value")"
}

write_unit() {
	cat >"$eye_unit" <<EOF
[Unit]
Description=Panoptes Eye
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=$eye_user
Group=$eye_user
EnvironmentFile=$eye_env
ExecStart=$eye_binary
WorkingDirectory=$eye_state
Restart=on-failure
RestartSec=5
UMask=0077
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
ReadWritePaths=$eye_state

[Install]
WantedBy=multi-user.target
EOF
}

endpoint=
seal_file=
epithet=
binary=
from_source=0
docker=0

while [[ $# -gt 0 ]]; do
	case $1 in
	-h | --help)
		usage
		exit 0
		;;
	--endpoint)
		endpoint=$(take_flag "$1" "${2-}")
		shift 2
		;;
	--endpoint=*)
		endpoint=${1#*=}
		shift
		;;
	--seal-file)
		seal_file=$(take_flag "$1" "${2-}")
		shift 2
		;;
	--seal-file=*)
		seal_file=${1#*=}
		shift
		;;
	--epithet)
		epithet=$(take_flag "$1" "${2-}")
		shift 2
		;;
	--epithet=*)
		epithet=${1#*=}
		shift
		;;
	--binary)
		binary=$(take_flag "$1" "${2-}")
		shift 2
		;;
	--binary=*)
		binary=${1#*=}
		shift
		;;
	--from-source)
		from_source=1
		shift
		;;
	--docker)
		docker=1
		shift
		;;
	*)
		die "unknown option: $1"
		;;
	esac
done

require_root
require_cmd install
require_cmd useradd

endpoint=$(prompt_if_empty endpoint "Panopticon endpoint (host:port): " "$endpoint")
[[ $endpoint == *:* ]] || die "endpoint must be host:port"

workdir=$(mktemp -d)
trap 'rm -rf "$workdir"' EXIT

built=$(obtain_eye "$workdir/eye")
[[ -f $built ]] || die "Eye binary not found: $built"

ensure_user "$eye_user" "$eye_state"
install -d -m 0700 -o "$eye_user" -g "$eye_user" "$eye_state"
install -d -m 0755 /etc/panoptes
install_binary "$built" "$eye_binary"
ln -sfn "$eye_binary" "$eye_link"

{
	write_env_line PANOPTICON_ENDPOINT "$endpoint"
	write_env_line PANOPTES_STATE_DIR "$eye_state"
	if [[ -n $epithet ]]; then
		write_env_line EYE_EPITHET "$epithet"
	fi
} >"$eye_env"
chmod 0600 "$eye_env"
chown root:root "$eye_env"

if [[ -n $epithet ]]; then
	printf '%s\n' "$epithet" >"$eye_state/epithet"
	chmod 0600 "$eye_state/epithet"
	chown "$eye_user:$eye_user" "$eye_state/epithet"
fi

if [[ ! -f $eye_state/brand ]]; then
	seal_file=$(prompt_if_empty seal-file "Seal file: " "$seal_file")
	[[ -f $seal_file ]] || die "Seal file not found: $seal_file"
	install -m 0600 -o "$eye_user" -g "$eye_user" "$seal_file" "$eye_state/seal"
fi

write_unit

if [[ $docker -eq 1 ]] && getent group docker >/dev/null; then
	install -d -m 0755 "$eye_dropin_dir"
	cat >"$eye_dropin" <<'EOF'
[Service]
SupplementaryGroups=docker
EOF
else
	if [[ $docker -eq 1 ]]; then
		printf '%s\n' "docker group is absent; skipping socket grant" >&2
	fi
	rm -f "$eye_dropin"
	rmdir "$eye_dropin_dir" 2>/dev/null || true
fi

require_cmd systemctl
systemctl daemon-reload
systemctl enable panoptes-eye.service
systemctl restart panoptes-eye.service
printf '%s\n' "Eye installed at $eye_binary"
