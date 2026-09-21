#!/bin/sh
# Tests the Bluetooth on-air framing (length-prefixed messages cut into
# chunks) on both platforms, with no device and no radio:
#
#   ios      NearbyFraming.swift, compiled with swiftc
#   android  NearbyBle.kt's Reassembler and frameChunks, as a JUnit test
#            (needs client-react-native/android from `expo prebuild`)
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
MOD="$ROOT/client-react-native/modules/zolik-nearby"
OUT=$(mktemp -d)
trap 'rm -rf "$OUT"' EXIT

swiftc -o "$OUT/framing" "$MOD/ios/NearbyFraming.swift" "$MOD/ios/FramingCheck/main.swift"
"$OUT/framing"

if [ -d "$ROOT/client-react-native/android" ]; then
  (cd "$ROOT/client-react-native/android" && ./gradlew -q :zolik-nearby:testReleaseUnitTest)
  echo "android framing ok"
else
  echo "android: skipped (run expo prebuild -p android first)"
fi
