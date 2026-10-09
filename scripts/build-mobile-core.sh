#!/bin/sh
# Builds the embedded game server (server/mobile/zolikcore) for the iOS and
# Android apps, and drops it where the zolik-nearby Expo module links it:
#
#   client-react-native/modules/zolik-nearby/ios/Zolikcore.xcframework
#   client-react-native/modules/zolik-nearby/android/maven/  (a flat Maven repo)
#
# Neither is checked in. Run this before `expo prebuild`, `expo run:*` or an
# EAS build: server/go.mod replaces kdb with the sibling ../kdb checkout, so an
# EAS worker cannot compile Go itself and uploads these prebuilt instead.
#
#   scripts/build-mobile-core.sh            both platforms
#   scripts/build-mobile-core.sh ios        just one
#   scripts/build-mobile-core.sh android
#
# Needs gomobile (go install golang.org/x/mobile/cmd/gomobile@latest &&
# gomobile init), Xcode for iOS, and the Android NDK for Android.
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
MOD="$ROOT/client-react-native/modules/zolik-nearby"
WHICH="${1:-all}"

command -v gomobile >/dev/null || { echo "gomobile not found: go install golang.org/x/mobile/cmd/gomobile@latest" >&2; exit 1; }
[ -f "$ROOT/../kdb/go/go.mod" ] || { echo "the kdb sibling checkout is missing at $ROOT/../kdb" >&2; exit 1; }

# The web client is compiled into the core too, so that a phone hosting a
# table can serve it to browsers in the room: somebody without the app opens
# the host's link and plays in their browser. It is exported here, the same way
# server/Dockerfile does it, so the page and the phone's server are always one
# build. A stray export already in dist/ is refused rather than shipped,
# because nothing would say which commit it came from.
#
#   ZOLIK_MOBILE_WEB=0 scripts/build-mobile-core.sh   leaves it out
WEBDIST="$ROOT/server/internal/webui/dist"
if [ "$(ls -A "$WEBDIST" | grep -v '^.gitkeep$' || true)" ]; then
  echo "server/internal/webui/dist holds a web export; clear it first" >&2
  exit 1
fi
clean_web() {
  find "$WEBDIST" -mindepth 1 ! -name .gitkeep -exec rm -rf {} + 2>/dev/null || true
}
trap clean_web EXIT
if [ "${ZOLIK_MOBILE_WEB:-1}" != 0 ]; then
  eval "$("$ROOT/scripts/version.sh" --export)"
  (
    cd "$ROOT/client-react-native"
    rm -rf dist
    EXPO_PUBLIC_ZOLIK_API_SAME_ORIGIN=1 \
      EXPO_PUBLIC_ZOLIK_VERSION="$ZOLIK_VERSION" \
      EXPO_PUBLIC_ZOLIK_COMMIT="$ZOLIK_COMMIT" \
      npx expo export --platform web
  )
  test -f "$ROOT/client-react-native/dist/index.html" || { echo "expo export produced no dist/index.html" >&2; exit 1; }
  cp -R "$ROOT/client-react-native/dist/." "$WEBDIST/"
  echo "web: $(du -sh "$WEBDIST" | cut -f1) compiled into the core"
fi

# Stamped the same way the Docker image is, so the footer and the About
# screen say which build the phone's own server is.
eval "$("$ROOT/scripts/version.sh" --export)"
STAMP="-X zolik/server/internal/buildinfo.Version=$ZOLIK_VERSION -X zolik/server/internal/buildinfo.Commit=$ZOLIK_COMMIT"

cd "$ROOT/server"

if [ "$WHICH" = all ] || [ "$WHICH" = ios ]; then
  rm -rf "$MOD/ios/Zolikcore.xcframework"
  gomobile bind -target=ios,iossimulator -trimpath -ldflags="-s -w $STAMP" -o "$MOD/ios/Zolikcore.xcframework" ./mobile/zolikcore
  echo "ios: $(du -sh "$MOD/ios/Zolikcore.xcframework/ios-arm64" | cut -f1) arm64"
fi

if [ "$WHICH" = all ] || [ "$WHICH" = android ]; then
  : "${ANDROID_HOME:=$HOME/Library/Android/sdk}"
  : "${ANDROID_NDK_HOME:=$(ls -d "$ANDROID_HOME"/ndk/* 2>/dev/null | sort -V | tail -1)}"
  export ANDROID_HOME ANDROID_NDK_HOME
  # A flat Maven repository (group/artifact/version/artifact-version.aar plus
  # a pom), because the Android Gradle plugin refuses a bare local .aar in a
  # library module. The version is fixed: the artifact is rebuilt in place.
  REPO="$MOD/android/maven/com/jokerless/zolikcore/1.0.0"
  rm -rf "$MOD/android/maven"
  mkdir -p "$REPO"
  # arm64 and arm for phones; amd64 for an x86_64 emulator image. The page
  # size flag is not optional: Play rejects native code that is not aligned
  # for Android 15's 16 KB pages, and Go's linker does not do it by default.
  gomobile bind -target=android/arm64,android/arm,android/amd64 -androidapi 24 -javapkg com.jokerless \
    -trimpath -ldflags="-s -w $STAMP -extldflags=-Wl,-z,max-page-size=16384" \
    -o "$REPO/zolikcore-1.0.0.aar" ./mobile/zolikcore
  rm -f "$REPO/zolikcore-1.0.0-sources.jar"
  cat > "$REPO/zolikcore-1.0.0.pom" <<'POM'
<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.jokerless</groupId>
  <artifactId>zolikcore</artifactId>
  <version>1.0.0</version>
  <packaging>aar</packaging>
</project>
POM
  echo "android: $(du -sh "$REPO/zolikcore-1.0.0.aar" | cut -f1) aar"
fi
