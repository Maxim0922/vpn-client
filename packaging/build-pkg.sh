#!/bin/bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

OUT="build"
STAGE="$OUT/pkgroot"
PKG="$OUT/max-vpn.pkg"
VERSION="0.1.0"
IDENT="de.max-tsx.maxvpn.pkg"

ARCH="${ARCH:-arm64}" packaging/build-app.sh
CGO_LDFLAGS="-Wl,-no_warn_duplicate_libraries" \
  go build -tags with_gvisor,with_utls -o "$OUT/maxvpnd" ./cmd/maxvpnd

rm -rf "$STAGE"
mkdir -p "$STAGE/Applications"
mkdir -p "$STAGE/Library/PrivilegedHelperTools"
mkdir -p "$STAGE/Library/LaunchDaemons"

cp -R "$OUT/max-vpn.app" "$STAGE/Applications/"
install -m 0755 "$OUT/maxvpnd" "$STAGE/Library/PrivilegedHelperTools/maxvpnd"
install -m 0644 packaging/de.max-tsx.maxvpn.daemon.plist "$STAGE/Library/LaunchDaemons/"

chmod +x packaging/scripts/postinstall

COMPONENTS="$OUT/components.plist"
pkgbuild --analyze --root "$STAGE" "$COMPONENTS"
plutil -replace 0.BundleIsRelocatable -bool NO "$COMPONENTS"

pkgbuild \
  --root "$STAGE" \
  --component-plist "$COMPONENTS" \
  --scripts packaging/scripts \
  --identifier "$IDENT" \
  --version "$VERSION" \
  --install-location / \
  "$PKG"

echo "==> built $PKG"
echo "    install with: sudo installer -pkg $PKG -target /"
