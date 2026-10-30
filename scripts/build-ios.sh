#!/bin/sh
# Builds the iPhone app (client-react-native) as a Release build signed for
# devices, into client-react-native/ios/build, and can put it on a phone:
#
#   1. the game core (build-mobile-core.sh ios), which also compiles in the
#      web client a phone hosting a table serves to browsers in the room;
#   2. the native project (expo prebuild) and its pods;
#   3. the app, signed by Xcode's automatic signing for the team.
#
#   scripts/build-ios.sh                              build only
#   ZOLIK_IOS_DEVICE="DavJa iPhone" scripts/build-ios.sh  build, install, launch
#   ZOLIK_IOS_SKIP_CORE=1 …                           reuse the last core
#   ZOLIK_IOS_TEAM=… …                                another signing team
#
# ZOLIK_IOS_DEVICE is a name or id from `xcrun devicectl list devices`; a phone
# the team has not seen yet is registered to it on the way. Needs Xcode signed
# in to the team's account, gomobile and the ../kdb checkout.
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
RN="$ROOT/client-react-native"
# Limidus Corp, the paid team. A free Personal Team cannot sign the push
# entitlement expo-notifications adds.
TEAM="${ZOLIK_IOS_TEAM:-U763YTDQ4R}"
DEVICE="${ZOLIK_IOS_DEVICE:-}"
OUT="$RN/ios/build"

export PATH="/opt/homebrew/bin:$PATH"
# CocoaPods refuses to run without a UTF-8 locale.
export LANG=en_US.UTF-8

if [ "${ZOLIK_IOS_SKIP_CORE:-0}" != 1 ]; then
  "$ROOT/scripts/build-mobile-core.sh" ios
fi
test -d "$RN/modules/zolik-nearby/ios/Zolikcore.xcframework" || { echo "no game core; run without ZOLIK_IOS_SKIP_CORE" >&2; exit 1; }

(cd "$RN" && npx expo prebuild --platform ios --no-install)
(cd "$RN/ios" && pod install)
PODS="$(cd "$RN/ios/Pods" && pwd -P)"

# expo-modules-jsi compiles itself with SwiftPM inside its own package
# directory, and SwiftPM keeps the header paths it first resolved from
# PODS_ROOT even when a later build passes another one. Worktrees share one
# node_modules, so a cache another checkout left points at that checkout's
# Pods, and once that checkout is gone the build fails on "'hermes/hermes.h'
# file not found". The cache is dropped whenever it was not built for these
# Pods; it rebuilds in about a minute.
JSI="$(cd "$RN/node_modules/expo-modules-jsi/apple" && pwd -P)"
JSI_STAMP="$JSI/.DerivedData/zolik-pods-root"
if [ -d "$JSI/.DerivedData" ] && [ "$(cat "$JSI_STAMP" 2>/dev/null)" != "$PODS" ]; then
  echo "expo-modules-jsi: dropping a cache built for other Pods"
  rm -rf "$JSI/.DerivedData"
fi

eval "$("$ROOT/scripts/version.sh" --export)"
if [ -n "$DEVICE" ]; then
  DEST="platform=iOS,name=$DEVICE"
else
  DEST="generic/platform=iOS"
fi
(
  cd "$RN/ios"
  EXPO_PUBLIC_ZOLIK_BASE_URL=https://jokerless.com \
    EXPO_PUBLIC_ZOLIK_VERSION="$ZOLIK_VERSION" \
    EXPO_PUBLIC_ZOLIK_COMMIT="$ZOLIK_COMMIT" \
    xcodebuild -workspace Jokerless.xcworkspace -scheme Jokerless \
      -configuration Release -destination "$DEST" -derivedDataPath "$OUT" \
      -allowProvisioningUpdates -allowProvisioningDeviceRegistration \
      DEVELOPMENT_TEAM="$TEAM" CODE_SIGN_STYLE=Automatic \
      -quiet build
)
echo "$PODS" > "$JSI_STAMP"

APP="$OUT/Build/Products/Release-iphoneos/Jokerless.app"
echo "app: $APP ($(du -sh "$APP" | cut -f1))"

if [ -n "$DEVICE" ]; then
  # The first install over a fresh tunnel sometimes times out; a second try
  # gets through.
  xcrun devicectl device install app --device "$DEVICE" "$APP" >/dev/null ||
    xcrun devicectl device install app --device "$DEVICE" "$APP" >/dev/null
  xcrun devicectl device process launch --device "$DEVICE" com.jokerless.app >/dev/null
  echo "installed and launched on $DEVICE"
fi
