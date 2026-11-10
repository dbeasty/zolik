#!/bin/sh
# Puts the Mac app on a disk image for download, the standard way:
#
#   scripts/macos-dmg.sh <Jokerless.app> <out.dmg>
#
#   1. opening the image shows the license (COPYRIGHT and LICENSE), and the
#      volume mounts only after Agree;
#   2. its window shows the app, an arrow and a link to Applications, with no
#      toolbar, so installing is one drag;
#   3. the volume has the app's icon.
#
# The image comes out unsigned; build-macos.sh signs and notarizes it, which
# has to happen after the license goes in, because that changes the file.
#
# Finder writes the window's layout, so this mounts a volume for a moment and
# needs permission to control Finder (macOS asks the first time).
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
[ $# = 2 ] || { echo "usage: $0 <Jokerless.app> <out.dmg>" >&2; exit 1; }
APP=$1
DMG=$2

WORK=$(mktemp -d "${TMPDIR:-/tmp}/jokerless-dmg.XXXXXX")
# Under /Volumes, so Finder treats it as a disk, and named per run, so it is
# found by path even if a Jokerless volume is already open.
MNT="/Volumes/jokerless-dmg-$$"
STAGE="$WORK/stage"
RW="$WORK/rw.dmg"
cleanup() {
  [ ! -d "$MNT" ] || hdiutil detach -quiet -force "$MNT" || true
  rm -rf "$WORK"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM

mkdir -p "$STAGE/.background"
ditto "$APP" "$STAGE/Jokerless.app"
ln -s /Applications "$STAGE/Applications"

swift "$ROOT/scripts/macos-dmg-background.swift" "$WORK/bg.png" "$WORK/bg@2x.png"
tiffutil -cathidpicheck "$WORK/bg.png" "$WORK/bg@2x.png" -out "$STAGE/.background/background.tiff" 2>/dev/null

# Room for the .DS_Store Finder is about to write. The spare space compresses
# to nothing in the final image.
SIZE_MB=$(($(du -sm "$STAGE" | cut -f1) + 20))
hdiutil create -quiet -volname Jokerless -srcfolder "$STAGE" -size "${SIZE_MB}m" -fs HFS+ -format UDRW "$RW"
hdiutil attach -quiet -noautoopen -mountpoint "$MNT" "$RW"
# Positions match macos-dmg-background.swift. The window is 28 points taller
# than the picture, for its title bar.
osascript - "$MNT" <<'APPLESCRIPT'
on run argv
  tell application "Finder"
    set vol to item (POSIX file (item 1 of argv) as alias)
    open vol
    set win to container window of vol
    set current view of win to icon view
    set toolbar visible of win to false
    set statusbar visible of win to false
    set pathbar visible of win to false
    set bounds of win to {200, 120, 800, 548}
    set opts to icon view options of win
    set arrangement of opts to not arranged
    set icon size of opts to 128
    set text size of opts to 13
    set background picture of opts to file ".background:background.tiff" of vol
    set position of item "Jokerless.app" of vol to {150, 170}
    set position of item "Applications" of vol to {450, 170}
    update vol without registering applications
    delay 2
    close win
  end tell
end run
APPLESCRIPT

# Finder writes .DS_Store when it gets to it, and a volume detached before then
# opens with the default layout.
i=0
until [ -s "$MNT/.DS_Store" ]; do
  i=$((i + 1))
  [ "$i" -le 30 ] || { echo "Finder did not save the window layout" >&2; exit 1; }
  sleep 1
done
# After Finder is done, which otherwise removes the file and clears the flag.
# The volume shows .VolumeIcon.icns only once its root is flagged as having a
# custom icon.
cp "$APP/Contents/Resources/AppIcon.icns" "$MNT/.VolumeIcon.icns"
SetFile -a C "$MNT"
sync
hdiutil detach -quiet "$MNT"

mkdir -p "$(dirname -- "$DMG")"
rm -f "$DMG"
hdiutil convert -quiet "$RW" -format ULFO -o "$DMG"
node "$ROOT/scripts/macos-dmg-license.mjs" > "$WORK/license.plist"
hdiutil udifrez -quiet -xml "$WORK/license.plist" '' "$DMG"
hdiutil imageinfo "$DMG" | grep -q 'Software License Agreement: true' ||
  { echo "the disk image did not take the license" >&2; exit 1; }
