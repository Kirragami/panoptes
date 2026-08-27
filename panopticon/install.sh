#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

panopticon_user=panopticon
panopticon_binary=/opt/panoptes/panopticon
panopticon_state=/var/lib/panoptes/panopticon
panopticon_env=/etc/panoptes/panopticon.env
panopticon_unit=/etc/systemd/system/panopticon.service
panopticon_link=/usr/local/bin/panopticon
panopticon_chronicle=$panopticon_state/chronicle.db

die() {
	printf '%s\n' "$*" >&2
	exit 1
}

usage() {
	cat <<'EOF'
usage:
  install.sh [options]

options:
  --tls-cert PATH
  --tls-key PATH
  --panel-addr ADDR
  --panel-password-file PATH
  --firebase PATH
  --binary PATH
  --from-source
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

prompt_password() {
	local first=
	local second=
	if [[ -t 0 ]]; then
		read -r -s -p "Panel password: " first
		printf '\n' >&2
		read -r -s -p "Confirm panel password: " second
		printf '\n' >&2
		[[ $first == "$second" ]] || die "passwords do not match"
		[[ -n $first ]] || die "panel password is required"
		printf '%s' "$first"
		return
	fi
	die "panel password is required; pass --panel-password-file"
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

build_panopticon() {
	local output=$1
	[[ -f $root/main.go ]] || die "Panopticon source not found; pass --binary or omit --from-source to download a release"
	require_cmd go
	(
		cd "$root"
		CGO_ENABLED=0 go build -trimpath \
			-ldflags "-s -w -X github.com/Kirragami/panoptes/panopticon/cli.version=$(source_version)" \
			-o "$output" .
	)
}

download_release_panopticon() {
	local output=$1
	[[ $(uname -s) == Linux ]] || die "install is only published for linux"
	require_cmd curl
	require_cmd sha256sum
	local arch name sums want got
	arch=$(linux_arch)
	name=panopticon-linux-$arch
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

obtain_panopticon() {
	local output=$1
	if [[ -n $binary ]]; then
		printf '%s' "$binary"
		return
	fi
	if [[ $from_source -eq 1 ]]; then
		build_panopticon "$output"
		printf '%s' "$output"
		return
	fi
	download_release_panopticon "$output"
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

readable_by_user() {
	local name=$1
	local path=$2
	su -s /bin/sh "$name" -c "test -r $(printf '%q' "$path")"
}

executable_by_user() {
	local name=$1
	local path=$2
	su -s /bin/sh "$name" -c "test -x $(printf '%q' "$path")"
}

trim() {
	local s=$1
	s="${s#"${s%%[![:space:]]*}"}"
	s="${s%"${s##*[![:space:]]}"}"
	printf '%s' "$s"
}

grant_ancestors_exec() {
	local user=$1
	local path=$2
	local dir
	dir=$(dirname -- "$path")
	while [[ $dir != / ]]; do
		if [[ -d $dir ]] && ! executable_by_user "$user" "$dir"; then
			chgrp "$user" "$dir"
			chmod g+x "$dir"
		fi
		dir=$(dirname -- "$dir")
	done
}

grant_read_to_user() {
	local user=$1
	local path=$2
	local resolved
	resolved=$(readlink -f -- "$path")
	[[ -n $resolved && -e $resolved ]] || return 1
	grant_ancestors_exec "$user" "$path"
	grant_ancestors_exec "$user" "$resolved"
	chgrp "$user" "$resolved"
	chmod g+r "$resolved"
}

ensure_readable_by_user() {
	local user=$1
	local path=$2
	if readable_by_user "$user" "$path"; then
		return
	fi
	grant_read_to_user "$user" "$path" || die "$path is not readable by $user; could not chmod"
	if ! readable_by_user "$user" "$path"; then
		die "$path is not readable by $user after chmod"
	fi
}

reject_home_path() {
	local path=$1
	case $path in
	/home/*)
		die "$path is under /home; ProtectHome blocks it"
		;;
	esac
}

write_env_line() {
	local name=$1
	local value=$2
	if [[ $value == *$'\n'* ]]; then
		die "$name cannot contain a newline"
	fi
	printf '%s=%s\n' "$name" "$value"
}

systemd_unescape() {
	local s=$1
	local i=0
	local out=
	local c=
	while ((i < ${#s})); do
		c=${s:i:1}
		if [[ $c == '$' && ${s:i:2} == '$$' ]]; then
			out+='$'
			i=$((i + 2))
		elif [[ $c == '%' && ${s:i:2} == '%%' ]]; then
			out+='%'
			i=$((i + 2))
		else
			out+=$c
			i=$((i + 1))
		fi
	done
	printf '%s' "$out"
}

load_env_value() {
	local file=$1
	local wanted=$2
	local line name value
	[[ -f $file ]] || return 0
	while IFS= read -r line || [[ -n $line ]]; do
		[[ -z $line || $line == \#* ]] && continue
		name=${line%%=*}
		value=${line#*=}
		if [[ $name == "$wanted" ]]; then
			systemd_unescape "$value"
			return
		fi
	done <"$file"
}

mint_hex() {
	if command -v openssl >/dev/null; then
		openssl rand -hex 32
		return
	fi
	od -An -N32 -tx1 /dev/urandom | tr -d ' \n'
	printf '\n'
}

mint_session_key() {
	if command -v openssl >/dev/null; then
		openssl rand -base64 32 | tr -d '\n'
		printf '\n'
		return
	fi
	require_cmd base64
	dd if=/dev/urandom bs=32 count=1 status=none | base64 | tr -d '\n'
	printf '\n'
}

write_unit() {
	cat >"$panopticon_unit" <<EOF
[Unit]
Description=Panoptes Panopticon
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=$panopticon_user
Group=$panopticon_user
EnvironmentFile=$panopticon_env
ExecStart=$panopticon_binary
WorkingDirectory=$panopticon_state
Restart=on-failure
RestartSec=5
UMask=0077
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
ReadWritePaths=$panopticon_state

[Install]
WantedBy=multi-user.target
EOF
}

tls_cert=
tls_key=
panel_addr=:8443
password_file=
firebase=
binary=
from_source=0

while [[ $# -gt 0 ]]; do
	case $1 in
	-h | --help)
		usage
		exit 0
		;;
	--tls-cert)
		tls_cert=$(take_flag "$1" "${2-}")
		shift 2
		;;
	--tls-cert=*)
		tls_cert=${1#*=}
		shift
		;;
	--tls-key)
		tls_key=$(take_flag "$1" "${2-}")
		shift 2
		;;
	--tls-key=*)
		tls_key=${1#*=}
		shift
		;;
	--panel-addr)
		panel_addr=$(take_flag "$1" "${2-}")
		shift 2
		;;
	--panel-addr=*)
		panel_addr=${1#*=}
		shift
		;;
	--panel-password-file)
		password_file=$(take_flag "$1" "${2-}")
		shift 2
		;;
	--panel-password-file=*)
		password_file=${1#*=}
		shift
		;;
	--firebase)
		firebase=$(take_flag "$1" "${2-}")
		shift 2
		;;
	--firebase=*)
		firebase=${1#*=}
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
	*)
		die "unknown option: $1"
		;;
	esac
done

require_root
require_cmd install
require_cmd useradd

tls_cert=$(trim "$(prompt_if_empty tls-cert "TLS certificate path: " "$tls_cert")")
tls_key=$(trim "$(prompt_if_empty tls-key "TLS key path: " "$tls_key")")
[[ -n $panel_addr ]] || panel_addr=:8443
if [[ -z $firebase ]]; then
	firebase=$(load_env_value "$panopticon_env" PANOPTICON_FIREBASE_CREDENTIALS)
fi
firebase=$(trim "$(prompt_if_empty firebase "Firebase credentials path: " "$firebase")")
reject_home_path "$tls_cert"
reject_home_path "$tls_key"
reject_home_path "$firebase"
[[ -f $tls_cert ]] || die "TLS certificate not found: $tls_cert"
[[ -f $tls_key ]] || die "TLS key not found: $tls_key"
[[ -f $firebase ]] || die "Firebase credentials not found: $firebase"

workdir=$(mktemp -d)
trap 'rm -rf "$workdir"' EXIT

built=$(obtain_panopticon "$workdir/panopticon")
[[ -x $built ]] || die "Panopticon binary is not executable: $built"

password_hash=
if [[ -n $password_file ]]; then
	[[ -f $password_file ]] || die "password file not found: $password_file"
	password_hash=$(tr -d '\r\n' <"$password_file" | "$built" hash-password)
elif [[ -t 0 ]]; then
	password_hash=$(prompt_password | "$built" hash-password)
else
	password_hash=$(load_env_value "$panopticon_env" PANOPTICON_PANEL_PASSWORD_HASH)
	[[ -n $password_hash ]] || die "panel password is required; pass --panel-password-file"
fi
[[ -n $password_hash ]] || die "failed to hash panel password"

edict_token=$(load_env_value "$panopticon_env" PANOPTICON_EDICT_TOKEN)
if [[ -z $edict_token ]]; then
	edict_token=$(mint_hex)
fi

session_key=$(load_env_value "$panopticon_env" PANOPTICON_PANEL_SESSION_KEY)
if [[ -z $session_key ]]; then
	session_key=$(mint_session_key)
fi

ensure_user "$panopticon_user" "$panopticon_state"
install -d -m 0700 -o "$panopticon_user" -g "$panopticon_user" "$panopticon_state"
install -d -m 0755 /etc/panoptes
install_binary "$built" "$panopticon_binary"
ln -sfn "$panopticon_binary" "$panopticon_link"

ensure_readable_by_user "$panopticon_user" "$tls_cert"
ensure_readable_by_user "$panopticon_user" "$tls_key"
ensure_readable_by_user "$panopticon_user" "$firebase"

{
	write_env_line PANOPTICON_TLS_CERT_FILE "$tls_cert"
	write_env_line PANOPTICON_TLS_KEY_FILE "$tls_key"
	write_env_line PANOPTICON_CHRONICLE "$panopticon_chronicle"
	write_env_line PANOPTICON_PANEL_ADDR "$panel_addr"
	write_env_line PANOPTICON_PANEL_PASSWORD_HASH "$password_hash"
	write_env_line PANOPTICON_PANEL_SESSION_KEY "$session_key"
	write_env_line PANOPTICON_EDICT_TOKEN "$edict_token"
	write_env_line PANOPTICON_FIREBASE_CREDENTIALS "$firebase"
} >"$panopticon_env"
chmod 0600 "$panopticon_env"
chown root:root "$panopticon_env"

write_unit
require_cmd systemctl
systemctl daemon-reload
systemctl enable panopticon.service
systemctl restart panopticon.service
printf '%s\n' "Panopticon installed at $panopticon_binary"
