#!/usr/bin/env node
/**
 * Finds user-facing English still hardcoded in the client.
 *
 * The i18n tests prove every *key* is worded in every language. They cannot
 * prove a string reached a key at all — a sentence typed straight into JSX is
 * invisible to them, and that is exactly what produces a screen half in the
 * player's language and half in English.
 *
 * This reads the source rather than the running app because the alternative is
 * catching each leak only on the screen that happens to be exercised. It is a
 * heuristic, so it errs toward reporting: an entry that is genuinely not
 * player-facing goes in `ALLOWED` below, with the reason, and stops being
 * noise for everyone after.
 */

const fs = require('fs');
const path = require('path');

const root = path.join(__dirname, '..', 'client-react-native');
const dirs = ['app', 'src/components', 'src/hooks'];

/** Props whose value a player reads. */
const TEXT_PROPS = [
  'title',
  'subtitle',
  'label',
  'placeholder',
  'accessibilityLabel',
  'accessibilityHint',
  'heading',
  'message',
  'confirmLabel',
  'cancelLabel',
  'emptyText',
];

/**
 * Strings that look player-facing and are not. Each needs a reason: the list
 * is a place to record "we looked at this", not a place to hide failures.
 */
const ALLOWED = new Set([
  // Skin and avatar names are proper nouns chosen by the designer, shown
  // identically in every language — see `src/skins` and `src/components/avatars`.
  'Amber', 'Violet', 'Teal', 'Coral', 'Slate', 'Moss', 'Casino', 'Classic',
  // The product name, in both spellings the app uses.
  'Zolik', 'Žolíky',
]);

const IGNORE_FILE = /\.test\.tsx?$|\/__tests__\//;

function walk(dir, out = []) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) walk(full, out);
    else if (/\.tsx$/.test(entry.name) && !IGNORE_FILE.test(full)) out.push(full);
  }
  return out;
}

/** A string a player could read: has a letter, and is not an identifier or token. */
function looksHuman(s) {
  const t = s.trim();
  if (t.length < 2) return false;
  if (!/[A-Za-z]/.test(t)) return false;
  if (ALLOWED.has(t)) return false;
  // testIDs, style tokens, route paths, keys, urls, single lowercase words
  // used as enum values — none of these are read as prose.
  if (/^[a-z0-9]+([-_.][a-z0-9]+)*$/.test(t)) return false;
  if (/^[A-Z0-9_]+$/.test(t)) return false;
  if (/^[#/.]/.test(t) || /^https?:/.test(t)) return false;
  if (/^\{.*\}$/.test(t)) return false;
  // Colours, not words.
  if (/^(rgba?|hsla?)\(/.test(t) || /^#[0-9a-f]{3,8}$/i.test(t)) return false;
  // Fragments of TypeScript that the `>...<` scan picks up out of a generic
  // or a comparison — `Set<string>`, `x >= left && x`. Prose does not contain
  // these operators, so requiring their absence costs nothing.
  if (/[<>=&|]{1,2}/.test(t) && !/[.?!,]/.test(t)) return false;
  if (/^(string|number|boolean|null|undefined|void|any)\b/.test(t)) return false;
  return /[A-Za-z]{2,}/.test(t);
}

const findings = [];

for (const dir of dirs) {
  const abs = path.join(root, dir);
  if (!fs.existsSync(abs)) continue;
  for (const file of walk(abs)) {
    const rel = path.relative(root, file);
    const src = fs.readFileSync(file, 'utf8');
    const lines = src.split('\n');

    lines.forEach((line, i) => {
      const n = i + 1;
      const code = line.replace(/\/\/.*$/, '');
      if (/^\s*\*/.test(line) || /^\s*\/\//.test(line)) return;

      // <Text ...>Some words</Text> on one line. The multi-line case is
      // handled after this loop, against the whole file.
      for (const m of code.matchAll(/>([^<>{}\n]+)</g)) {
        // Same guard as the wrapped-text pass: these characters mean the
        // brackets were an arrow and a comparison, not a pair of tags.
        if (/[;=()[\]|&/\\]/.test(m[1])) continue;
        // `() => Promise<void>` — the `>` that opened this was the tail of an
        // arrow, so what follows is a type, not a line of prose.
        if (code[m.index - 1] === '=') continue;
        if (looksHuman(m[1])) findings.push({ rel, n, kind: 'jsx-text', text: m[1].trim() });
      }

      // title="Some words" / placeholder='...'
      for (const prop of TEXT_PROPS) {
        const re = new RegExp(`\\b${prop}=(?:"([^"]+)"|'([^']+)')`, 'g');
        for (const m of code.matchAll(re)) {
          const v = m[1] ?? m[2];
          if (looksHuman(v)) findings.push({ rel, n, kind: `prop:${prop}`, text: v.trim() });
        }
      }

      // title={'Some words'} / label={"..."} — braces around a bare literal
      for (const prop of TEXT_PROPS) {
        const re = new RegExp(`\\b${prop}=\\{\\s*(?:"([^"]+)"|'([^']+)')\\s*\\}`, 'g');
        for (const m of code.matchAll(re)) {
          const v = m[1] ?? m[2];
          if (looksHuman(v)) findings.push({ rel, n, kind: `prop:${prop}`, text: v.trim() });
        }
      }

      // A sentence handed straight to a function — `setError('…')`,
      // `setNotice('…')`. Invisible to every pattern above, because it never
      // touches JSX at all, and it is exactly where the messages a player
      // sees when something breaks tend to live.
      for (const m of code.matchAll(/\b(set[A-Z]\w*|alert|throw new Error)\(\s*'([^']{4,})'/g)) {
        const v = m[2];
        if (looksHuman(v) && /\s/.test(v)) {
          findings.push({ rel, n, kind: 'call-argument', text: v.trim() });
        }
      }

      // Ternaries and returns that yield a bare sentence: ? 'Sign in' : 'Sign out'
      for (const m of code.matchAll(/[?:]\s*'([^']{4,})'/g)) {
        const v = m[1];
        if (looksHuman(v) && /\s/.test(v)) {
          findings.push({ rel, n, kind: 'inline-literal', text: v.trim() });
        }
      }
    });

    // A sentence long enough to wrap is exactly the kind that carries the most
    // meaning, and the line-at-a-time scan above cannot see it: prettier splits
    // `<Text>Attempt {n}. The server is …</Text>` across three lines and no
    // single one of them is a complete `>text<`. Scanning the whole file for
    // text nodes that may span newlines is what catches those.
    // Comments are not player-facing, and a `{/* … */}` block is full of
    // exactly the prose this scan is hunting for. Blanking them first (keeping
    // the line count intact) is cheaper than trying to exclude them later.
    const scanSrc = src
      .replace(/\{\/\*[\s\S]*?\*\/\}/g, (c) => c.replace(/[^\n]/g, ' '))
      .replace(/\/\*[\s\S]*?\*\//g, (c) => c.replace(/[^\n]/g, ' '));

    for (const m of scanSrc.matchAll(/>((?:[^<>{}]|\{[^{}]*\})*?)</g)) {
      const raw = m[1];
      if (!raw.includes('\n')) continue; // single-line case already covered
      // A JSX text node is prose. Any of these characters means the `>` and
      // `<` that bracketed it were an arrow function and a comparison, not a
      // pair of tags — `useMemo(() => f(x), [x]); return (` and friends.
      if (/[;=()[\]|&/\\]/.test(raw.replace(/\{[^{}]*\}/g, ''))) continue;
      // Collapse the wrapping back into one sentence, and drop the `{expr}`
      // holes so what is left is only the words a translator would own.
      const text = raw.replace(/\{[^{}]*\}/g, '\u0001').replace(/\s+/g, ' ').trim();
      const words = text.replace(/\u0001/g, '').trim();
      if (!looksHuman(words) || !/\s/.test(words)) continue;
      const n = src.slice(0, m.index).split('\n').length;
      findings.push({ rel, n, kind: 'jsx-text-wrapped', text: text.replace(/\u0001/g, '{}') });
    }
  }
}

// One report per file, deduplicated by text+line.
const seen = new Set();
const byFile = new Map();
for (const f of findings) {
  const id = `${f.rel}:${f.n}:${f.text}`;
  if (seen.has(id)) continue;
  seen.add(id);
  if (!byFile.has(f.rel)) byFile.set(f.rel, []);
  byFile.get(f.rel).push(f);
}

const files = [...byFile.keys()].sort();
let total = 0;
for (const file of files) {
  console.log(`\n${file}`);
  for (const f of byFile.get(file).sort((a, b) => a.n - b.n)) {
    console.log(`  ${String(f.n).padStart(4)}  [${f.kind}]  ${f.text}`);
    total += 1;
  }
}
console.log(`\n${total} hardcoded player-facing string(s) in ${files.length} file(s).`);
process.exit(total ? 1 : 0);
