#!/usr/bin/env bash
# Tests install.sh without a network and without touching the system: the commands it
# asks about (uname, dpkg, ...) are stand-ins, and the "release" is a folder of files.
#   bash install_test.sh
# The stand-in commands are written with single quotes on purpose.
# shellcheck disable=SC2016
set -u
here=$(cd "$(dirname "$0")" && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
fails=0

fail() { printf 'FAIL: %s\n' "$*"; fails=$((fails + 1)); }
check() { # check NAME OUTPUT PATTERN
	case $2 in *"$3"*) ;; *) fail "$1: expected \"$3\" in:"$'\n'"$2" ;; esac
}

# system DIR SYSTEM MACHINE [stand-in...] makes a PATH of its own: only the tools the
# script needs and a uname that says SYSTEM and MACHINE, so a system without dpkg really
# has none. A stand-in "name" records its arguments in $work/calls, "name=text" prints
# text.
system() {
	local dir=$1 os=$2 machine=$3 tool name p
	shift 3
	mkdir -p "$dir"
	for tool in curl wget tar mktemp rm mkdir mv cp cut grep sed id ditto sha256sum shasum cat dirname echo chmod ln; do
		p=$(command -v "$tool" 2>/dev/null) && [ -x "$p" ] && ln -sf "$p" "$dir/$tool"
	done
	printf '#!/bin/sh\ncase "$1" in -m) echo %s;; *) echo %s;; esac\n' "$machine" "$os" >"$dir/uname"
	chmod +x "$dir/uname"
	for name in "$@"; do
		case $name in
		*=*) printf '#!/bin/sh\necho "%s"\n' "${name#*=}" >"$dir/${name%%=*}"; name=${name%%=*} ;;
		*) printf '#!/bin/sh\necho "%s $*" >> "%s/calls"\n' "$name" "$work" >"$dir/$name" ;;
		esac
		chmod +x "$dir/$name"
	done
}

releases="https://example.invalid"
run() { # run DIR ARGS... -> $output and $status of the script
	local dir=$1
	shift
	output=$(PATH=$dir HOME="$work/home" DARTS_O_CAM_RELEASES="$releases" "$(command -v bash)" "$here/install.sh" "$@" 2>&1)
	status=$?
}
plan() { # plan NAME EXPECTED... DIR: what --dry-run says
	run "$work/$1" --dry-run
	shift
	local want
	for want in "$@"; do check "$want" "$output" "$want"; done
}

system "$work/mac-arm" Darwin arm64 sysctl=0
plan mac-arm "macos, arm64" "darts-o-cam_macos_arm64.zip" "unpack into ~/Applications"
system "$work/mac-intel" Darwin x86_64 sysctl=0
plan mac-intel "macos, amd64" "darts-o-cam_macos_amd64.zip"
# A terminal under Rosetta on an Apple silicon Mac says x86_64, but the Mac is ARM.
system "$work/rosetta" Darwin x86_64 sysctl=1
plan rosetta "macos, arm64"
system "$work/deb" Linux x86_64 dpkg apt-get
plan deb "darts-o-cam_linux_amd64.deb" "sudo apt-get install"
system "$work/deb-only" Linux aarch64 dpkg
plan deb-only "darts-o-cam_linux_arm64.deb" "sudo dpkg -i"
system "$work/rpm-dnf" Linux aarch64 rpm dnf
plan rpm-dnf "darts-o-cam_linux_arm64.rpm" "sudo dnf install"
system "$work/rpm-zypper" Linux x86_64 rpm zypper
plan rpm-zypper "darts-o-cam_linux_amd64.rpm" "sudo zypper install"
system "$work/rpm-only" Linux x86_64 rpm
plan rpm-only "sudo rpm -U"
system "$work/tar" Linux x86_64
plan tar "darts-o-cam_linux_amd64.tar.gz" "unpack into ~/.local" "Installing"

# The server (--server): its own file names, and no .zip or menu entry.
plan_server() { # plan_server NAME EXPECTED... DIR: what --server --dry-run says
	run "$work/$1" --server --dry-run
	shift
	local want
	for want in "$@"; do check "server: $want" "$output" "$want"; done
}
plan_server mac-arm "macos, arm64" "darts-o-mat_darwin_arm64.tar.gz" "unpack into ~/.local/bin"
plan_server mac-intel "macos, amd64" "darts-o-mat_darwin_amd64.tar.gz"
plan_server deb "darts-o-mat_linux_amd64.deb" "sudo apt-get install"
plan_server deb-only "darts-o-mat_linux_arm64.deb" "sudo dpkg -i"
plan_server rpm-dnf "darts-o-mat_linux_arm64.rpm" "sudo dnf install"
plan_server rpm-zypper "darts-o-mat_linux_amd64.rpm" "sudo zypper install"
plan_server tar "darts-o-mat_linux_amd64.tar.gz" "unpack into ~/.local" "Installing"

system "$work/arm32" Linux armv7l
run "$work/arm32" --dry-run
check arm32 "$output" "unsupported processor armv7l"
[ "$status" -ne 0 ] || fail "armv7l: expected an error"
system "$work/win" MINGW64_NT-10.0 x86_64
run "$work/win" --dry-run
check windows "$output" "this is Windows"
[ "$status" -ne 0 ] || fail "windows: expected an error"
run "$work/win" --server --dry-run
check server-windows "$output" "double click darts-o-mat.exe"
[ "$status" -ne 0 ] || fail "server on Windows: expected an error"
run "$work/tar" --nonsense
check "unknown option" "$output" "unknown option"
run "$work/tar" --version 1.2.3 --dry-run
check version "$output" "Package: darts-o-cam_linux_amd64.tar.gz (1.2.3 release)"

# A release as a folder: files and their checksums under latest/download/.
releases="file://$work/rel"
download="$work/rel/latest/download"
mkdir -p "$download" "$work/home"
sums() { (cd "$download" && { sha256sum -- * 2>/dev/null || shasum -a 256 -- *; } >SHA256SUMS); }

# Linux .tar.gz into ~/.local, then again as an update.
pkg="$work/pkg/darts-o-cam_1.0.0_linux_amd64"
mkdir -p "$pkg/usr/bin" "$pkg/usr/share/applications" "$pkg/usr/lib/systemd/system"
printf '#!/bin/sh\necho 1.0.0\n' >"$pkg/usr/bin/darts-o-cam"
chmod +x "$pkg/usr/bin/darts-o-cam"
echo "[Desktop Entry]" >"$pkg/usr/share/applications/darts-o-cam.desktop"
echo "[Unit]" >"$pkg/usr/lib/systemd/system/darts-o-cam.service"
tar czf "$download/darts-o-cam_linux_amd64.tar.gz" -C "$work/pkg" darts-o-cam_1.0.0_linux_amd64
sums
run "$work/tar"
check tar-install "$output" "Installed into ~/.local"
[ -x "$work/home/.local/bin/darts-o-cam" ] || fail "tar: the program isn't in ~/.local/bin"
[ -f "$work/home/.local/share/applications/darts-o-cam.desktop" ] || fail "tar: no menu entry"
[ ! -e "$work/home/.local/share/systemd" ] && [ ! -e "$work/home/.local/lib" ] || fail "tar: the systemd unit must not be installed"
run "$work/tar" --dry-run
check tar-update "$output" "Updating"
run "$work/tar"
check tar-update "$output" "up to date"

# A package that doesn't match its checksum is not installed.
echo tampered >>"$download/darts-o-cam_linux_amd64.tar.gz"
rm -rf "$work/home/.local"
run "$work/tar"
check checksum "$output" "doesn't match"
if [ "$status" -eq 0 ] || [ -e "$work/home/.local/bin/darts-o-cam" ]; then fail "a package with a wrong checksum was installed"; fi

# .deb: apt-get gets the downloaded file (stand-ins record the calls).
rm "$download/darts-o-cam_linux_amd64.tar.gz"
echo deb >"$download/darts-o-cam_linux_amd64.deb"
sums
system "$work/deb-real" Linux x86_64 dpkg apt-get sudo
rm -f "$work/calls"
run "$work/deb-real"
if [ "$(id -u)" = 0 ]; then want=apt-get; else want=sudo; fi
calls=$(cat "$work/calls" 2>/dev/null)
check deb-install "$calls" "$want"
check deb-install "$calls" "install -y"
check deb-install "$calls" "darts-o-cam_linux_amd64.deb"

# The server: the Linux .tar.gz has the tree of the packages like the camera's, the macOS
# one is a folder with the program; a .deb is what apt-get gets.
rm -rf "$work/home/.local"
srv="$work/srv/darts-o-mat_1.0.0_linux_amd64"
mkdir -p "$srv/usr/bin" "$srv/usr/share/doc/darts-o-mat" "$srv/usr/lib/systemd/system"
printf '#!/bin/sh\necho 1.0.0\n' >"$srv/usr/bin/darts-o-mat"
chmod +x "$srv/usr/bin/darts-o-mat"
echo license >"$srv/usr/share/doc/darts-o-mat/LICENSE"
echo "[Unit]" >"$srv/usr/lib/systemd/system/darts-o-mat.service"
tar czf "$download/darts-o-mat_linux_amd64.tar.gz" -C "$work/srv" darts-o-mat_1.0.0_linux_amd64
sums
run "$work/tar" --server
check server-tar "$output" "Installed into ~/.local"
check server-tar "$output" "http://localhost:3000"
[ -x "$work/home/.local/bin/darts-o-mat" ] || fail "server tar: the program isn't in ~/.local/bin"
[ -f "$work/home/.local/share/doc/darts-o-mat/LICENSE" ] || fail "server tar: no LICENSE"
[ ! -e "$work/home/.local/bin/darts-o-cam" ] && [ ! -e "$work/home/.local/lib" ] && [ ! -e "$work/home/.local/share/systemd" ] || fail "server tar: only the program and its documents are installed, not the service"
run "$work/tar" --server
check server-update "$output" "up to date"
check server-update "$output" "games and accounts are kept"
rm "$download/darts-o-mat_linux_amd64.tar.gz"
# macOS: a folder with the program, LICENSE and README.
rm -rf "$work/home/.local"
mac="$work/macsrv/darts-o-mat_1.0.0_darwin_amd64"
mkdir -p "$mac"
printf '#!/bin/sh\necho 1.0.0\n' >"$mac/darts-o-mat"
chmod +x "$mac/darts-o-mat"
echo license >"$mac/LICENSE"
tar czf "$download/darts-o-mat_darwin_amd64.tar.gz" -C "$work/macsrv" darts-o-mat_1.0.0_darwin_amd64
sums
system "$work/mac-srv" Darwin x86_64 sysctl=0
run "$work/mac-srv" --server
check server-mac "$output" "Installed ~/.local/bin/darts-o-mat"
[ -x "$work/home/.local/bin/darts-o-mat" ] || fail "server on macOS: the program isn't in ~/.local/bin"
[ ! -e "$work/home/.local/share" ] || fail "server on macOS: only the program is installed"
rm "$download/darts-o-mat_darwin_amd64.tar.gz"
echo deb >"$download/darts-o-mat_linux_amd64.deb"
sums
rm -f "$work/calls"
run "$work/deb-real" --server
calls=$(cat "$work/calls" 2>/dev/null)
check server-deb "$calls" "install -y"
check server-deb "$calls" "darts-o-mat_linux_amd64.deb"
check server-deb "$output" "systemctl enable --now darts-o-mat"
rm "$download/darts-o-mat_linux_amd64.deb"

# macOS: the app is replaced in ~/Applications (only where ditto exists).
if command -v ditto >/dev/null 2>&1; then
	rm "$download/darts-o-cam_linux_amd64.deb"
	mkdir -p "$work/mac/darts-o-cam.app/Contents"
	zipapp() { # zipapp VERSION
		echo "$1" >"$work/mac/darts-o-cam.app/Contents/version"
		rm -f "$download/darts-o-cam_macos_amd64.zip"
		(cd "$work/mac" && ditto -c -k --keepParent darts-o-cam.app "$download/darts-o-cam_macos_amd64.zip")
		sums
	}
	system "$work/mac-real" Darwin x86_64 sysctl=0
	zipapp v1
	run "$work/mac-real"
	[ "$(cat "$work/home/Applications/darts-o-cam.app/Contents/version" 2>/dev/null)" = v1 ] || fail "macOS: app not installed: $output"
	zipapp v2
	run "$work/mac-real"
	check mac-update "$output" "up to date"
	[ "$(cat "$work/home/Applications/darts-o-cam.app/Contents/version")" = v2 ] || fail "macOS: the update didn't replace the app"
fi

if [ "$fails" -eq 0 ]; then echo "install.sh: all checks passed"; else echo "install.sh: $fails checks failed"; exit 1; fi
