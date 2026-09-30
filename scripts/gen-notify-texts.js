#!/usr/bin/env node
/**
 * Writes server/internal/notify/texts.json from the client's locale bundles.
 *
 * An OS push is worded by the server, because the phone or the browser shows
 * it without the app running, so the server has no bundle of its own to ask.
 * Rather than keep a second copy of those sentences in Go, this lifts them out
 * of the client bundles — every key starting `notify.push.` (the push's own
 * sentences) or `module.` (game names, which the push puts in them) — for
 * every locale, English filling any gap.
 *
 * `src/lib/notifyTexts.test.ts` fails when the committed file no longer
 * matches the bundles, so rewording a push in the app and forgetting to run
 * this is caught rather than shipped.
 *
 * No dependencies, and the bundles are read the way `check-locales.js` reads
 * them, so it runs without a node_modules.
 *
 *   node scripts/gen-notify-texts.js
 */

const fs = require('fs');
const path = require('path');

const localesDir = path.join(__dirname, '..', 'client-react-native', 'src', 'lib', 'locales');
const outFile = path.join(__dirname, '..', 'server', 'internal', 'notify', 'texts.json');

/** The keys a push can need. */
const wanted = (key) => key.startsWith('notify.push.') || key.startsWith('module.');

/** Reads a bundle without a TS toolchain: strip the export, evaluate the object. */
function readBundle(file) {
  const src = fs.readFileSync(path.join(localesDir, file), 'utf8');
  const start = src.indexOf('= {');
  return eval(`({${src.slice(start + 3, src.lastIndexOf('};'))}})`);
}

/** Every locale's push sentences, keys sorted so the file diffs cleanly. */
function build() {
  const files = fs
    .readdirSync(localesDir)
    .filter((f) => f.endsWith('.ts'))
    .sort();
  const en = readBundle('en.ts');
  const keys = Object.keys(en).filter(wanted).sort();
  const out = {};
  for (const file of files) {
    const bundle = file === 'en.ts' ? en : readBundle(file);
    const texts = {};
    for (const key of keys) texts[key] = bundle[key] || en[key];
    out[file.replace(/\.ts$/, '')] = texts;
  }
  return out;
}

/** The file's exact contents, so the test can compare bytes. */
function render() {
  return `${JSON.stringify(build(), null, 2)}\n`;
}

module.exports = { build, render, outFile };

if (require.main === module) {
  fs.writeFileSync(outFile, render());
  console.log(`wrote ${path.relative(process.cwd(), outFile)}`);
}
