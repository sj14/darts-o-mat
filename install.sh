#!/usr/bin/env bash
# Installs darts-o-cam, the program for the computer the cameras are connected to, or with
# --server the server darts-o-mat: finds out which package fits this computer (system,
# processor, and on Linux the package manager), downloads it from the latest release,
# checks it against the release's checksums and installs it.
#
#   curl -fsSL https://raw.githubusercontent.com/sj14/darts-o-mat/main/install.sh | bash
#   curl -fsSL https://raw.githubusercontent.com/sj14/darts-o-mat/main/install.sh | bash -s -- --server
#
# Run it again later to update: it installs the latest release over the installed one.
#
# Options (after `bash -s --` when piped): --server for the server instead of darts-o-cam,
# --version 1.2.3 for another release than the latest, --dry-run to only say what would
# be done, -h for this text.
#
# darts-o-cam: on macOS the app goes into ~/Applications. On Linux a .deb or .rpm (with
# sudo) if the system has dpkg or rpm, else the .tar.gz into ~/.local.
# Server: on Linux a .deb or .rpm (with sudo; it comes with a systemd service), else, and
# on macOS, the program into ~/.local/bin.
# Windows: not with this script, download the .zip from the release page.
set -eu

repo="sj14/darts-o-mat"
# DARTS_O_CAM_RELEASES points to another place with the same layout, e.g. a mirror (or the
# test, install_test.sh).
releases="${DARTS_O_CAM_RELEASES:-https://github.com/$repo/releases}"
app=darts-o-cam
version=latest
dry_run=0

say() { printf '%s\n' "$*"; }
die() { printf 'Error: %s\n' "$*" >&2; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }

usage() {
	sed -n '2,/^set -eu/p' "$0" | sed '$d' | sed 's/^# \{0,1\}//'
	exit 0
}

while [ $# -gt 0 ]; do
	case $1 in
	--server) app=darts-o-mat; shift ;;
	--version) [ $# -ge 2 ] || die "--version needs a number, e.g. --version 1.2.3"; version=${2#v}; shift 2 ;;
	--dry-run) dry_run=1; shift ;;
	-h | --help) usage ;;
	*) die "unknown option $1 (see -h)" ;;
	esac
done

# What is this computer?
case $(uname -s) in
Darwin) os=macos ;;
Linux) os=linux ;;
MINGW* | MSYS* | CYGWIN*) die "this is Windows: download the .zip for Windows from $releases/latest, unpack it and double click $app.exe" ;;
*) die "unsupported system $(uname -s); see $releases/latest" ;;
esac
case $(uname -m) in
x86_64 | amd64) arch=amd64 ;;
arm64 | aarch64) arch=arm64 ;;
*) die "unsupported processor $(uname -m): $app is built for 64-bit Intel/AMD and ARM (a 32-bit system on a Raspberry Pi needs the 64-bit version of its system)" ;;
esac
# A terminal started under Rosetta on an Apple silicon Mac says x86_64: the Mac is ARM.
if [ "$os" = macos ] && [ "$arch" = amd64 ] && [ "$(sysctl -n sysctl.proc_translated 2>/dev/null || true)" = 1 ]; then
	arch=arm64
fi

# Which package?
case $os in
macos)
	if [ "$app" = darts-o-mat ]; then
		# The server's archives are named after Go's system names.
		file="darts-o-mat_darwin_$arch.tar.gz"
		how="unpack into ~/.local/bin"
	else
		file="darts-o-cam_macos_$arch.zip"
		how="unpack into ~/Applications"
	fi
	;;
linux)
	if have dpkg; then
		file="${app}_linux_$arch.deb"
		if have apt-get; then how="sudo apt-get install"; else how="sudo dpkg -i"; fi
	elif have rpm; then
		file="${app}_linux_$arch.rpm"
		if have dnf; then how="sudo dnf install"
		elif have zypper; then how="sudo zypper install"
		elif have yum; then how="sudo yum install"
		else how="sudo rpm -U"; fi
	else
		file="${app}_linux_$arch.tar.gz"
		if [ "$app" = darts-o-mat ]; then how="unpack into ~/.local/bin"; else how="unpack into ~/.local"; fi
	fi
	;;
esac
# Installed already? Then this is an update.
installed=0
case $file in
*.zip) { [ -d "$HOME/Applications/darts-o-cam.app" ] || [ -d /Applications/darts-o-cam.app ]; } && installed=1 ;;
*.deb) dpkg -s "$app" >/dev/null 2>&1 && installed=1 ;;
*.rpm) rpm -q "$app" >/dev/null 2>&1 && installed=1 ;;
*.tar.gz) [ -x "$HOME/.local/bin/$app" ] && installed=1 ;;
esac
if [ "$installed" = 1 ]; then action=Updating; else action=Installing; fi
if [ "$version" = latest ]; then base="$releases/latest/download"; else base="$releases/download/v$version"; fi

say "System:  $os, $arch"
say "Package: $file ($version release)"
say "$action: $how"
if [ "$dry_run" = 1 ]; then
	say "(only showing what would be done: --dry-run)"
	exit 0
fi

have curl || have wget || die "curl or wget is needed to download the package"
download() { # download URL FILE
	if have curl; then curl -fsSL -o "$2" "$1"; else wget -q -O "$2" "$1"; fi
}
sha256() {
	if have sha256sum; then sha256sum "$1" | cut -d' ' -f1; else shasum -a 256 "$1" | cut -d' ' -f1; fi
}
as_root() { # as root nothing is needed, otherwise sudo asks for the password
	if [ "$(id -u)" = 0 ]; then "$@"; elif have sudo; then sudo "$@"; else die "this needs administrator rights, but sudo isn't installed: run the script as root"; fi
}

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

say "Downloading $base/$file"
download "$base/$file" "$tmp/$file" || die "the download failed; see $releases"
download "$base/SHA256SUMS" "$tmp/SHA256SUMS" || die "the checksums of the release couldn't be downloaded"
want=$(grep "[ *]$file\$" "$tmp/SHA256SUMS" | cut -d' ' -f1)
[ -n "$want" ] || die "no checksum for $file in the release"
[ "$(sha256 "$tmp/$file")" = "$want" ] || die "the checksum of $file doesn't match: not installing it"

case $file in
*.zip)
	apps="$HOME/Applications"
	mkdir -p "$apps"
	rm -rf "$tmp/app" "$apps/darts-o-cam.app"
	mkdir "$tmp/app"
	ditto -x -k "$tmp/$file" "$tmp/app"
	mv "$tmp/app/darts-o-cam.app" "$apps/"
	say "Installed $apps/darts-o-cam.app. Open it from there (macOS asks for camera access on the first start; if it is open already, quit and open it again)."
	;;
*.deb)
	if have apt-get; then as_root apt-get install -y "$tmp/$file"; else as_root dpkg -i "$tmp/$file"; fi
	;;
*.rpm)
	if have dnf; then as_root dnf install -y "$tmp/$file"
	elif have zypper; then as_root zypper --non-interactive install --allow-unsigned-rpm "$tmp/$file"
	elif have yum; then as_root yum install -y "$tmp/$file"
	else as_root rpm -U "$tmp/$file"; fi
	;;
*.tar.gz)
	mkdir "$tmp/unpacked"
	tar xzf "$tmp/$file" -C "$tmp/unpacked"
	root=$(echo "$tmp"/unpacked/*)
	mkdir -p "$HOME/.local/bin"
	if [ "$app" = darts-o-mat ]; then
		# The server's archive is a folder with the program, LICENSE and README.
		cp "$root/darts-o-mat" "$HOME/.local/bin/"
		say "Installed ~/.local/bin/darts-o-mat."
	else
		mkdir -p "$HOME/.local/share"
		cp -R "$root/usr/bin/." "$HOME/.local/bin/"
		cp -R "$root/usr/share/." "$HOME/.local/share/"
		say "Installed into ~/.local (the program in ~/.local/bin, the menu entry in ~/.local/share)."
	fi
	case ":$PATH:" in *":$HOME/.local/bin:"*) ;; *) say "Add ~/.local/bin to your PATH to start it as $app." ;; esac
	;;
esac

if [ "$os" = linux ]; then
	# A running service keeps the old program until it is restarted.
	if have systemctl && systemctl is-active --quiet "$app" 2>/dev/null; then
		as_root systemctl restart "$app"
		say "The $app service was restarted."
	fi
fi
if [ "$app" = darts-o-mat ]; then
	case $file in
	*.deb | *.rpm) say "Start it as a service: sudo systemctl enable --now darts-o-mat (the games are kept in /var/lib/darts-o-mat)." ;;
	*) say "Start it with: darts-o-mat (the games are kept in the file darts-o-mat.db in the folder you start it in)." ;;
	esac
	say "Then open http://localhost:3000 in the browser."
	if [ "$installed" = 1 ]; then say "Done: darts-o-mat is up to date. Your games and accounts are kept."; else say "Done."; fi
else
	if [ "$os" = linux ]; then
		say "Start it from the menu (darts-o-cam) or, without a desktop, with: sudo systemctl enable --now darts-o-cam"
		say "Your user needs access to the cameras, usually through the group video: sudo usermod -aG video \$USER (then log in again)."
	fi
	if [ "$installed" = 1 ]; then say "Done: darts-o-cam is up to date. Your cameras and settings are kept."; else say "Done. Next: add and calibrate the cameras in darts-o-cam."; fi
fi
