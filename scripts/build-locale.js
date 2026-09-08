#!/usr/bin/env node
/**
 * Writes one locale bundle from a JSON of translations, using the English
 * bundle as the template.
 *
 * The English file is walked line by line: its section comments and blank
 * lines are copied through, and each `'key': 'value',` line is re-emitted with
 * the translated value. That is the point — every bundle then has the same
 * keys in the same order under the same headings, so `git diff` between two
 * languages shows wording and nothing else, and a reviewer who does not read
 * Latvian can still see that a key landed in the right section.
 *
 * The comments stay in English on purpose. They address whoever is editing the
 * file, not whoever is reading the app.
 *
 * This is an authoring aid, not a build step: once written, the `.ts` files are
 * the source of truth and are edited directly. Nothing imports the JSON, and
 * `check-locales.js` reads the `.ts` files. Reach for this when seeding a whole
 * new language; for a single key, edit the twenty-four files and let the test
 * tell you which one you missed.
 *
 * Usage: node scripts/build-locale.js <code> <translations.json> <"Header doc">
 */

const fs = require('fs');
const path = require('path');

const [code, jsonPath, header] = process.argv.slice(2);
if (!code || !jsonPath) {
  console.error('usage: build-locale.js <code> <translations.json> [header]');
  process.exit(1);
}

const root = path.join(__dirname, '..', 'client-react-native', 'src', 'lib', 'locales');
const template = fs.readFileSync(path.join(root, 'en.ts'), 'utf8');
const translations = JSON.parse(fs.readFileSync(jsonPath, 'utf8'));

/** Quotes for TS, preferring single quotes and switching only when forced. */
function quote(value) {
  if (!value.includes("'")) return `'${value.replace(/\\/g, '\\\\')}'`;
  if (!value.includes('"')) return `"${value.replace(/\\/g, '\\\\')}"`;
  return `'${value.replace(/\\/g, '\\\\').replace(/'/g, "\\'")}'`;
}

// Prettier wraps a long entry onto its own line rather than running past ~110
// columns, so a value may sit on the line after its key. Emitting that same
// shape is what keeps the files diffable against each other.
const WIDTH = 110;

const lines = template.split('\n');
const out = [];
const seen = new Set();
let inBody = false;

for (let i = 0; i < lines.length; i += 1) {
  const line = lines[i];
  if (!inBody) {
    if (/^export const en: Record<string, string> = \{$/.test(line)) {
      out.push(
        (header ? `/**\n * ${header.split('\n').join('\n * ')}\n */\n\n` : '') +
          `export const ${code}: Record<string, string> = {`,
      );
      inBody = true;
    }
    continue; // The English file's own header doc does not belong here.
  }

  const match = line.match(/^  ((?:'(?:[^'\\]|\\.)*')|(?:"(?:[^"\\]|\\.)*")):(.*)$/);
  if (!match) {
    out.push(line);
    continue;
  }

  // Swallow a value that continues onto following lines, so it is replaced
  // rather than left behind under the translated key.
  let rest = match[2];
  while (!/,\s*$/.test(rest) && i + 1 < lines.length) {
    i += 1;
    rest += lines[i];
  }

  const key = JSON.parse(
    match[1].startsWith('"') ? match[1] : `"${match[1].slice(1, -1).replace(/\\'/g, "'").replace(/"/g, '\\"')}"`,
  );
  if (!(key in translations)) {
    console.error(`${code}: missing translation for ${key}`);
    process.exitCode = 1;
    continue;
  }
  seen.add(key);

  const entry = `  ${match[1]}: ${quote(translations[key])},`;
  out.push(entry.length <= WIDTH ? entry : `  ${match[1]}:\n    ${quote(translations[key])},`);
}

const extra = Object.keys(translations).filter((k) => !seen.has(k));
if (extra.length) {
  console.error(`${code}: ${extra.length} translation(s) for keys English does not have: ${extra.join(', ')}`);
  process.exitCode = 1;
}

fs.writeFileSync(path.join(root, `${code}.ts`), out.join('\n'));
console.log(`${code}: ${seen.size} keys`);
