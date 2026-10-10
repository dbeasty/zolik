#!/bin/sh
# Builds the Mac app (client-macos) into client-macos/build/Jokerless.app:
#
#   1. the web client, exported the same way the phones ship it, which the
#      app shows in its web view and serves to browsers in the room;
#   2. the game core (server/mobile/zolikcore) for macOS, as
#      client-macos/build/Zolikcore.xcframework;
#   3. the Swift shell, linked against both, and the .app bundle around it.
#
#   scripts/build-macos.sh                 everything, debug build
#   CONFIG=release scripts/build-macos.sh  release build
#   ZOLIK_MACOS_SKIP_WEB=1 …               reuse the last web export
#   ZOLIK_MACOS_SKIP_CORE=1 …              reuse the last core
#
# Needs Xcode, gomobile (see build-mobile-core.sh) and the ../kdb checkout.
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
MAC="$ROOT/client-macos"
OUT="$MAC/build"
CONFIG="${CONFIG:-debug}"
mkdir -p "$OUT"

eval "$("$ROOT/scripts/version.sh" --export)"

if [ "${ZOLIK_MACOS_SKIP_WEB:-0}" != 1 ]; then
  (
    cd "$ROOT/client-react-native"
    rm -rf dist
    # Same-origin, because the same export is what a Mac hosting a table
    # serves to browsers in the room. Inside the app the page is told where
    # the server is by the app itself (window.__ZOLIK_DESKTOP__).
    PATH="/opt/homebrew/bin:$PATH" \
      EXPO_PUBLIC_ZOLIK_API_SAME_ORIGIN=1 \
      EXPO_PUBLIC_ZOLIK_VERSION="$ZOLIK_VERSION" \
      EXPO_PUBLIC_ZOLIK_COMMIT="$ZOLIK_COMMIT" \
      npx expo export --platform web
  )
  test -f "$ROOT/client-react-native/dist/index.html" || { echo "expo export produced no dist/index.html" >&2; exit 1; }
  rm -rf "$OUT/web"
  cp -R "$ROOT/client-react-native/dist" "$OUT/web"
  echo "web: $(du -sh "$OUT/web" | cut -f1)"
fi

if [ "${ZOLIK_MACOS_SKIP_CORE:-0}" != 1 ]; then
  command -v gomobile >/dev/null || { echo "gomobile not found: go install golang.org/x/mobile/cmd/gomobile@latest" >&2; exit 1; }
  STAMP="-X zolik/server/internal/buildinfo.Version=$ZOLIK_VERSION -X zolik/server/internal/buildinfo.Commit=$ZOLIK_COMMIT"
  # The same floor as LSMinimumSystemVersion: without it the C parts of the
  # core target the SDK's own macOS, and the app would not start on older ones.
  rm -rf "$OUT/Zolikcore.xcframework"
  (cd "$ROOT/server" && gomobile bind -target=macos -macosversion=13.0 -trimpath -ldflags="-s -w $STAMP" \
    -o "$OUT/Zolikcore.xcframework" ./mobile/zolikcore)
  echo "core: $(du -sh "$OUT/Zolikcore.xcframework" | cut -f1)"
fi

BIN="$(cd "$MAC" && swift build -c "$CONFIG" --show-bin-path)/Jokerless"
# SwiftPM does not notice a rebuilt xcframework, so the binary is always
# relinked rather than left holding the previous core.
rm -f "$BIN"
(cd "$MAC" && swift build -c "$CONFIG" --product Jokerless)

APP="$OUT/Jokerless.app"
rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
cp "$BIN" "$APP/Contents/MacOS/Jokerless"
cp -R "$OUT/web" "$APP/Contents/Resources/web"
cp "$MAC/Resources/bridge.js" "$APP/Contents/Resources/bridge.js"
cp "$MAC/Resources/e2e.js" "$APP/Contents/Resources/e2e.js"
cp "$ROOT/client-react-native/assets/images/icon.png" "$APP/Contents/Resources/AppIcon.png"
sed -e "s/__VERSION__/$ZOLIK_VERSION/g" -e "s/__COMMIT__/$ZOLIK_COMMIT/g" \
  "$MAC/Resources/Info.plist" > "$APP/Contents/Info.plist"
# Ad hoc signing is enough to run here; a release is signed and notarized
# with the team's certificate instead.
codesign --force --sign - --entitlements "$MAC/Resources/Jokerless.entitlements" "$APP" >/dev/null
echo "app: $APP ($(du -sh "$APP" | cut -f1))"
