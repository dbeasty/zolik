#!/usr/bin/env node
/**
 * The parity checks from `i18n.test.ts`, runnable without a node_modules.
 *
 * `jest` asserts all of this and is the authority; this exists so the person
 * translating can run it after every file instead of after all twenty-two,
 * when a dropped `{n}` is one of five hundred lines rather than one of eleven
 * thousand.
 */

const fs = require('fs');
const path = require('path');

const dir = path.join(__dirname, '..', 'client-react-native', 'src', 'lib', 'locales');

/** Reads a bundle without a TS toolchain: strip the export, evaluate the object. */
function readBundle(file) {
  const src = fs.readFileSync(path.join(dir, file), 'utf8');
  const start = src.indexOf('= {');
  return eval(`({${src.slice(start + 3, src.lastIndexOf('};'))}})`);
}

const en = readBundle('en.ts');
const reference = Object.keys(en);
const placeholders = (s) => (s.match(/\{(\w+)\}/g) ?? []).sort().join(',');

const files = fs
  .readdirSync(dir)
  .filter((f) => f.endsWith('.ts') && f !== 'en.ts')
  .sort();

let failed = 0;
for (const file of files) {
  const code = file.replace('.ts', '');
  const bundle = readBundle(file);
  const keys = Object.keys(bundle);
  const problems = [];

  const missing = reference.filter((k) => !(k in bundle));
  const extra = keys.filter((k) => !(k in en));
  if (missing.length) problems.push(`${missing.length} missing: ${missing.slice(0, 5).join(', ')}`);
  if (extra.length) problems.push(`${extra.length} extra: ${extra.slice(0, 5).join(', ')}`);

  for (const key of reference) {
    const value = bundle[key];
    if (value === undefined) continue;
    if (value.trim() === '') problems.push(`${key}: empty`);
    if (placeholders(en[key]) !== placeholders(value)) {
      problems.push(`${key}: placeholders en(${placeholders(en[key])}) vs ${code}(${placeholders(value)})`);
    }
  }

  // A Cyrillic \u0435 inside a Slovak word renders identically to a Latin e,
  // passes every check above, and is invisible in review — it is exactly the
  // kind of thing that survives to production. Real Bulgarian and Greek are
  // exempt; every other bundle is Latin-script and has no business carrying
  // one of these.
  if (code !== 'bg' && code !== 'el') {
    const strays = new Set();
    for (const value of Object.values(bundle)) {
      for (const ch of value) {
        const cp = ch.codePointAt(0);
        if ((cp >= 0x0400 && cp <= 0x04ff) || (cp >= 0x0370 && cp <= 0x03ff)) strays.add(ch);
      }
    }
    if (strays.size) problems.push(`non-Latin lookalike character(s): ${[...strays].join(' ')}`);
  }

  // A translation identical to the English is usually a key that was pasted
  // rather than translated. Proper nouns and bare `{value}` passthroughs are
  // legitimately identical, so this reports a count rather than failing.
  const untouched = reference.filter((k) => bundle[k] === en[k] && !/^\{\w+\}$/.test(en[k]));

  if (problems.length) {
    failed += 1;
    console.log(`✗ ${code}`);
    for (const p of problems.slice(0, 12)) console.log(`    ${p}`);
    if (problems.length > 12) console.log(`    …and ${problems.length - 12} more`);
  } else {
    console.log(`✓ ${code}  ${keys.length} keys, ${untouched.length} identical to English`);
  }
}

console.log(`\n${files.length} locales checked, ${failed} with problems.`);
process.exit(failed ? 1 : 0);
