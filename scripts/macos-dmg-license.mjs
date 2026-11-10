#!/usr/bin/env node
// Writes the software license agreement of the Mac disk image: the resources
// `hdiutil udifrez -xml` stores in the image, which make macOS show the text
// with Agree and Disagree before it mounts the volume.
//
//   node scripts/macos-dmg-license.mjs > license.plist
//
// The text is the repository's own COPYRIGHT and LICENSE, word for word and
// line for line (hence the fixed-width font, which keeps the licence's layout),
// so it cannot drift from what the source tree says.
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const ROOT = join(dirname(fileURLToPath(import.meta.url)), '..');

/** The resources are ASCII or RTF escapes, because the format predates UTF-8. */
function rtf(text) {
  const body = text
    .replace(/\r\n/g, '\n')
    .replace(/[\\{}]/g, (c) => `\\${c}`)
    .replace(/[^\x00-\x7f]/g, (c) => `\\u${c.charCodeAt(0)}?`)
    .replace(/\n/g, '\\line\n');
  // 8.5 points: the largest size at which the dialog fits 78 columns unwrapped.
  return `{\\rtf1\\ansi\\ansicpg1252\\deff0{\\fonttbl{\\f0\\fmodern Menlo;}}\\f0\\fs17\n${body}}\n`;
}

function u16(...values) {
  const out = Buffer.alloc(values.length * 2);
  values.forEach((v, i) => out.writeUInt16BE(v, i * 2));
  return out;
}

/** A list of Pascal strings: a count, then each string after its length byte. */
function stringList(strings) {
  return Buffer.concat([
    u16(strings.length),
    ...strings.map((s) => {
      const ascii = Buffer.from(s, 'ascii');
      if (ascii.length > 255) throw new Error(`string too long for STR#: ${s}`);
      return Buffer.concat([Buffer.from([ascii.length]), ascii]);
    }),
  ]);
}

function resource(data) {
  return `    <dict>
      <key>Attributes</key><string>0x0000</string>
      <key>Data</key><data>${data.toString('base64')}</data>
      <key>ID</key><string>5000</string>
      <key>Name</key><string>English</string>
    </dict>`;
}

const read = (name) => readFileSync(join(ROOT, name), 'utf8').trimEnd();

const text = `Jokerless for Mac is built from Zolik, whose source code is at
https://github.com/dbeasty/zolik

Playing online is also subject to the Terms of Use:
https://jokerless.com/legal/terms

---

${read('COPYRIGHT')}

--------------------------------------------------------------------------

${read('LICENSE')}
`;

// One language, English, as the default: default language, language count,
// then the language's region code, resource offset and two-byte-text flag.
const languages = u16(0, 1, 0, 0, 0);
// The dialog's language name, buttons and prompt, in the order macOS expects.
const labels = stringList([
  'English',
  'Agree',
  'Disagree',
  'Print',
  'Save...',
  'To install Jokerless, read the license and press Agree. If you do not agree, press Disagree and the disk image will not open.',
]);

process.stdout.write(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>LPic</key>
  <array>
${resource(languages)}
  </array>
  <key>RTF </key>
  <array>
${resource(Buffer.from(rtf(text), 'ascii'))}
  </array>
  <key>STR#</key>
  <array>
${resource(labels)}
  </array>
</dict>
</plist>
`);
