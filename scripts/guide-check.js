#!/usr/bin/env node
/**
 * Checks the English guide text against the lessons the server ships.
 *
 *   node scripts/guide-check.js
 *
 * Reads client-react-native/src/guide/lessonKeys.json (regenerate it first:
 * `cd server && go run ./cmd/dump-lessons > ../client-react-native/src/guide/lessonKeys.json`)
 * and the `en` block of every topic in src/lib/locales/guide. Fails when a key
 * a lesson can show has no English wording, or a topic words a key no lesson
 * uses (apart from the guide's own `guide.common.*`, `guide.exit.*`,
 * `guide.entry.*` and `tour.*`).
 */
const fs = require('fs');
const path = require('path');

const root = path.join(__dirname, '..', 'client-react-native', 'src');
const keys = JSON.parse(fs.readFileSync(path.join(root, 'guide', 'lessonKeys.json'), 'utf8'));
const dir = path.join(root, 'lib', 'locales', 'guide');

const words = {};
for (const f of fs.readdirSync(dir).filter((n) => n.endsWith('.json'))) {
  const topic = f.replace(/\.json$/, '');
  for (const k of Object.keys(JSON.parse(fs.readFileSync(path.join(dir, f), 'utf8')).en || {})) words[k] = topic;
}

const own = /^(guide\.(common|exit|entry)\.|tour\.)/;
const missing = keys.filter((k) => !words[k]);
const unused = Object.keys(words).filter((k) => !own.test(k) && !keys.includes(k));
if (missing.length) console.error('lessons use keys with no English wording:\n  ' + missing.join('\n  '));
if (unused.length) console.error('English wording no lesson uses:\n  ' + unused.join('\n  '));
process.exit(missing.length || unused.length ? 1 : 0);
