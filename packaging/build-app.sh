#!/bin/bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

ARCH="${ARCH:-arm64}"
OUT="build"
APP="$OUT/max-vpn.app"

echo "==> building frontend"
( cd app/frontend && npm install --silent && npm run build )

export CGO_CFLAGS="-O2 -g -mmacosx-version-min=11.0"
export CGO_LDFLAGS="-mmacosx-version-min=11.0"

echo "==> building app binary ($ARCH)"
rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"

if [ "$ARCH" = "universal" ]; then
  GOARCH=arm64 go build -o "$OUT/max-vpn-arm64" ./app
  GOARCH=amd64 go build -o "$OUT/max-vpn-amd64" ./app
  lipo -create -output "$APP/Contents/MacOS/max-vpn" "$OUT/max-vpn-arm64" "$OUT/max-vpn-amd64"
  rm -f "$OUT/max-vpn-arm64" "$OUT/max-vpn-amd64"
else
  GOARCH="$ARCH" go build -o "$APP/Contents/MacOS/max-vpn" ./app
fi

cp packaging/Info.plist "$APP/Contents/Info.plist"
[ -f packaging/icon.icns ] && cp packaging/icon.icns "$APP/Contents/Resources/icon.icns" || true

echo "==> built $APP"
