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
#   CONFIG=release scripts/build-macos.sh  release build (Apple silicon + Intel)
#   ZOLIK_MACOS_SKIP_WEB=1 …               reuse the last web export
#   ZOLIK_MACOS_SKIP_CORE=1 …              reuse the last core
#   CONFIG=release ZOLIK_MACOS_DEVELOPER_ID=1 scripts/build-macos.sh
#                                          signed for any Mac, and notarized
#
# A plain build is ad hoc signed and opens only on the Mac that built it.
# ZOLIK_MACOS_DEVELOPER_ID=1 signs with the team's Developer ID Application
# certificate instead, under the hardened runtime, sends the app to Apple's
# notary service, staples the ticket and zips the result for download as
# build/Jokerless-<version>.zip. Notarizing uses credentials stored once with
#   xcrun notarytool store-credentials jokerless-notary
# (ZOLIK_MACOS_NOTARY_PROFILE names another profile).
#
# Needs Xcode, gomobile (see build-mobile-core.sh) and the ../kdb checkout.
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
MAC="$ROOT/client-macos"
OUT="$MAC/build"
CONFIG="${CONFIG:-debug}"
DEVELOPER_ID="${ZOLIK_MACOS_DEVELOPER_ID:-0}"
NOTARY_PROFILE="${ZOLIK_MACOS_NOTARY_PROFILE:-jokerless-notary}"
mkdir -p "$OUT"

if [ "$DEVELOPER_ID" = 1 ]; then
  # A debug build carries the e2e control folder; it must never leave this Mac.
  [ "$CONFIG" = release ] || { echo "a Developer ID build must be CONFIG=release" >&2; exit 1; }
  SIGN_ID=$(security find-identity -v -p codesigning | sed -n 's/.*"\(Developer ID Application: [^"]*\)".*/\1/p' | head -1)
  [ -n "$SIGN_ID" ] || { echo "no Developer ID Application certificate in the keychain" >&2; exit 1; }
  # Checked before the long build rather than after it.
  xcrun notarytool history --keychain-profile "$NOTARY_PROFILE" >/dev/null ||
    { echo "no notary credentials: xcrun notarytool store-credentials $NOTARY_PROFILE" >&2; exit 1; }
fi

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

# A release build is what gets downloaded, so it runs on Intel Macs too (the
# core is already built for both). A debug build stays native, for speed.
ARCHS=""
[ "$CONFIG" != release ] || ARCHS="--arch arm64 --arch x86_64"
BIN="$(cd "$MAC" && swift build -c "$CONFIG" $ARCHS --show-bin-path)/Jokerless"
# SwiftPM does not notice a rebuilt xcframework, so the binary is always
# relinked rather than left holding the previous core.
rm -f "$BIN"
(cd "$MAC" && swift build -c "$CONFIG" $ARCHS --product Jokerless)

APP="$OUT/Jokerless.app"
rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
cp "$BIN" "$APP/Contents/MacOS/Jokerless"
cp -R "$OUT/web" "$APP/Contents/Resources/web"
cp "$MAC/Resources/bridge.js" "$APP/Contents/Resources/bridge.js"
cp "$MAC/Resources/e2e.js" "$APP/Contents/Resources/e2e.js"
# The Dock and Finder icon: the phones' icon, as an .icns (Info.plist names it).
ICONSET="$OUT/AppIcon.iconset"
rm -rf "$ICONSET"
mkdir -p "$ICONSET"
SRC_ICON="$ROOT/client-react-native/assets/images/icon.png"
for size in 16 32 128 256 512; do
  sips -z "$size" "$size" "$SRC_ICON" --out "$ICONSET/icon_${size}x${size}.png" >/dev/null
  sips -z "$((size * 2))" "$((size * 2))" "$SRC_ICON" --out "$ICONSET/icon_${size}x${size}@2x.png" >/dev/null
done
iconutil -c icns "$ICONSET" -o "$APP/Contents/Resources/AppIcon.icns"
rm -rf "$ICONSET"
sed -e "s/__VERSION__/$ZOLIK_VERSION/g" -e "s/__COMMIT__/$ZOLIK_COMMIT/g" \
  "$MAC/Resources/Info.plist" > "$APP/Contents/Info.plist"
if [ "$DEVELOPER_ID" != 1 ]; then
  # Ad hoc signing is enough to run here.
  codesign --force --sign - --entitlements "$MAC/Resources/Jokerless.entitlements" "$APP" >/dev/null
  echo "app: $APP ($(du -sh "$APP" | cut -f1))"
  exit 0
fi

# The notary service accepts only the hardened runtime with a secure timestamp.
codesign --force --sign "$SIGN_ID" --options runtime --timestamp \
  --entitlements "$MAC/Resources/Jokerless.entitlements" "$APP"
codesign --verify --strict --deep "$APP"
ZIP="$OUT/Jokerless-$ZOLIK_VERSION.zip"
rm -f "$ZIP"
ditto -c -k --keepParent "$APP" "$ZIP"
xcrun notarytool submit "$ZIP" --keychain-profile "$NOTARY_PROFILE" --wait
# The stapled ticket lets the app open the first time without asking Apple.
# The zip is rebuilt so the download carries it.
xcrun stapler staple "$APP"
rm -f "$ZIP"
ditto -c -k --keepParent "$APP" "$ZIP"
spctl --assess --type execute --verbose "$APP"
echo "app: $APP ($(du -sh "$APP" | cut -f1)), signed by $SIGN_ID and notarized"
echo "download: $ZIP ($(du -sh "$ZIP" | cut -f1))"
