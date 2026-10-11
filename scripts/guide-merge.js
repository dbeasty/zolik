#!/usr/bin/env node
/**
 * Folds translations of one guide topic into its JSON file.
 *
 *   node scripts/guide-merge.js <topic> <dir>
 *
 * `<topic>` is a file in client-react-native/src/lib/locales/guide (without
 * .json) whose `en` block is the source. `<dir>` holds one `<locale>.json` per
 * language: a flat `{ key: translated text }` for exactly that topic's keys.
 *
 * Refuses a translation that misses a key, adds one, leaves a blank, or drops
 * or renames a `{placeholder}`: those are the failures that otherwise ship as
 * a half-translated screen. Languages not in `<dir>` are left as they were,
 * so a topic can be translated in batches.
 */
const fs = require('fs');
const path = require('path');

const [topic, dir] = process.argv.slice(2);
if (!topic || !dir) {
  console.error('usage: guide-merge.js <topic> <dir>');
  process.exit(2);
}
const file = path.join(__dirname, '..', 'client-react-native', 'src', 'lib', 'locales', 'guide', `${topic}.json`);
const doc = JSON.parse(fs.readFileSync(file, 'utf8'));
const en = doc.en;
if (!en) {
  console.error(`${topic}.json has no "en" block`);
  process.exit(2);
}
const placeholders = (s) => (s.match(/\{(\w+)\}/g) || []).sort().join();

let bad = 0;
for (const f of fs.readdirSync(dir).filter((n) => n.endsWith('.json')).sort()) {
  const locale = f.replace(/\.json$/, '');
  if (locale === 'en') continue;
  const tr = JSON.parse(fs.readFileSync(path.join(dir, f), 'utf8'));
  const problems = [];
  for (const k of Object.keys(en)) {
    if (typeof tr[k] !== 'string' || tr[k].trim() === '') problems.push(`missing or blank: ${k}`);
    else if (placeholders(tr[k]) !== placeholders(en[k])) problems.push(`placeholders differ: ${k}`);
  }
  for (const k of Object.keys(tr)) if (!(k in en)) problems.push(`unknown key: ${k}`);
  if (problems.length) {
    bad++;
    console.error(`${locale}: refused\n  ${problems.join('\n  ')}`);
    continue;
  }
  doc[locale] = Object.fromEntries(Object.keys(en).map((k) => [k, tr[k]]));
  console.log(`${locale}: merged`);
}
const order = ['en', ...Object.keys(doc).filter((l) => l !== 'en').sort()];
fs.writeFileSync(file, JSON.stringify(Object.fromEntries(order.map((l) => [l, doc[l]])), null, 2) + '\n');
process.exit(bad ? 1 : 0);
