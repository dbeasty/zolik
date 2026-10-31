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
#   ZOLIK_IOS_STORE=1 scripts/build-ios.sh            App Store .ipa
#   ZOLIK_IOS_STORE=1 ZOLIK_IOS_UPLOAD=1 …            …and upload it
#
# A store build is archived and exported for App Store Connect into
# client-react-native/ios/build/store. It differs from a device build in what
# the app may do: no plain-http loads (app.config.ts, ZOLIK_STORE_BUILD), and
# a fresh native project, so nothing a dev build put in Info.plist rides
# along. (The dev client's own Release build phase strips its Bonjour entry.) Its build number is the UTC time it was built
# (ZOLIK_IOS_BUILD_NUMBER overrides), which only ever goes up, as App Store
# Connect requires. Uploading needs the app record to exist in App Store
# Connect and uses the Xcode account's credentials.
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
STORE="${ZOLIK_IOS_STORE:-0}"
OUT="$RN/ios/build"

export PATH="/opt/homebrew/bin:$PATH"
# CocoaPods refuses to run without a UTF-8 locale.
export LANG=en_US.UTF-8

if [ "${ZOLIK_IOS_SKIP_CORE:-0}" != 1 ]; then
  "$ROOT/scripts/build-mobile-core.sh" ios
fi
test -d "$RN/modules/zolik-nearby/ios/Zolikcore.xcframework" || { echo "no game core; run without ZOLIK_IOS_SKIP_CORE" >&2; exit 1; }

if [ "$STORE" = 1 ]; then
  [ -z "$DEVICE" ] || { echo "a store build is not installed on a phone; drop ZOLIK_IOS_DEVICE" >&2; exit 1; }
  export ZOLIK_STORE_BUILD=1
  export ZOLIK_IOS_BUILD_NUMBER="${ZOLIK_IOS_BUILD_NUMBER:-$(date -u +%Y%m%d%H%M)}"
  echo "store build, build number $ZOLIK_IOS_BUILD_NUMBER"
  # CI=1 so --clean does not stop to ask about uncommitted files.
  (cd "$RN" && CI=1 npx expo prebuild --platform ios --no-install --clean)
else
  (cd "$RN" && npx expo prebuild --platform ios --no-install)
fi
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
export EXPO_PUBLIC_ZOLIK_BASE_URL=https://jokerless.com
export EXPO_PUBLIC_ZOLIK_VERSION="$ZOLIK_VERSION"
export EXPO_PUBLIC_ZOLIK_COMMIT="$ZOLIK_COMMIT"
if [ "$STORE" = 1 ]; then
  ARCHIVE="$OUT/store/Jokerless.xcarchive"
  rm -rf "$OUT/store"
  (
    cd "$RN/ios"
    xcodebuild -workspace Jokerless.xcworkspace -scheme Jokerless \
      -configuration Release -destination generic/platform=iOS \
      -derivedDataPath "$OUT" -archivePath "$ARCHIVE" \
      -allowProvisioningUpdates \
      DEVELOPMENT_TEAM="$TEAM" CODE_SIGN_STYLE=Automatic \
      -quiet archive
  )
else
  if [ -n "$DEVICE" ]; then
    DEST="platform=iOS,name=$DEVICE"
  else
    DEST="generic/platform=iOS"
  fi
  (
    cd "$RN/ios"
    xcodebuild -workspace Jokerless.xcworkspace -scheme Jokerless \
      -configuration Release -destination "$DEST" -derivedDataPath "$OUT" \
      -allowProvisioningUpdates -allowProvisioningDeviceRegistration \
      DEVELOPMENT_TEAM="$TEAM" CODE_SIGN_STYLE=Automatic \
      -quiet build
  )
fi
echo "$PODS" > "$JSI_STAMP"

if [ "$STORE" = 1 ]; then
  # Export signs the archive again for distribution, with the push
  # entitlement in its production form. "upload" sends it to App Store
  # Connect as part of the export; "export" leaves the .ipa here.
  DESTINATION=export
  [ "${ZOLIK_IOS_UPLOAD:-0}" = 1 ] && DESTINATION=upload
  cat > "$OUT/store/ExportOptions.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>method</key><string>app-store-connect</string>
  <key>destination</key><string>$DESTINATION</string>
  <key>teamID</key><string>$TEAM</string>
  <key>signingStyle</key><string>automatic</string>
  <key>uploadSymbols</key><true/>
  <key>manageAppVersionAndBuildNumber</key><false/>
</dict>
</plist>
PLIST
  xcodebuild -exportArchive -archivePath "$ARCHIVE" \
    -exportOptionsPlist "$OUT/store/ExportOptions.plist" \
    -exportPath "$OUT/store" -allowProvisioningUpdates -quiet
  if [ "$DESTINATION" = upload ]; then
    echo "uploaded $ZOLIK_VERSION ($ZOLIK_IOS_BUILD_NUMBER) to App Store Connect"
  else
    echo "ipa: $OUT/store/Jokerless.ipa ($ZOLIK_VERSION, build $ZOLIK_IOS_BUILD_NUMBER)"
  fi
  exit 0
fi

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
