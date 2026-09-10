#!/usr/bin/env node
/**
 * The lobby's words come from the server, not from a bundle.
 *
 * `/modules` is the one endpoint that sends English prose rather than keys, so
 * `src/lib/gameLabels.ts` re-keys it client-side. That leaves two ways for
 * English to reach a Czech screen, and neither is visible to a source scan:
 *
 *  1. **A label with no key.** The fallback renders the server's English and
 *     nothing complains. "500 (short)" sat in the Canasta options this way.
 *  2. **A key two modules disagree about.** `choice.targetScore.500` is
 *     "500 (short)" in Canasta and plain "500" in Gin Rummy and Rummy Tiles.
 *     One bare key cannot be both, so whichever wording is written, one game
 *     is wrong — and it reads as a translation that is merely *odd*, which is
 *     the kind of bug nobody reports. The fix is a module-scoped key; this
 *     names the ones that need one.
 *
 * Needs a running server, so it is advisory: unreachable is a skip, not a
 * failure. `npm run i18n` covers the checks that need nothing.
 */

const fs = require('fs');
const path = require('path');

// Same default as the e2e suite's `API_BASE`, so both talk to one dev stack.
const BASE = process.env.ZOLIK_E2E_API_BASE || 'http://127.0.0.1:8090';
const LOCALES = path.join(__dirname, '..', 'client-react-native', 'src', 'lib', 'locales');

/** Ratios, bare numbers, ranges — "3:2", "500", "2–4". Nothing to translate. */
const UNIVERSAL = /^[\s\d:+\-–—/.,()%]*$/;

/**
 * Names, which stay as they are.
 *
 * A game is called what it is called: a Czech player looks for "Prší", and a
 * German one also looks for "Texas Hold'em" — translating either would make
 * the game harder to find, not easier. Same for the places variations are
 * named after. Listed rather than guessed at, so a genuinely translatable
 * label added tomorrow is reported instead of quietly assumed to be a name.
 */
const PROPER = new Set([
  // Games.
  'Žolíky', 'Prší', 'Canasta', 'Blackjack', 'Gin Rummy', 'Rummy Tiles',
  'Texas Hold’em', "Texas Hold'em",
  // Variations, which are named after places or after the game itself.
  'Žolík Classic', 'Continental', 'Oklahoma', 'Atlantic City', 'Vegas Strip',
  // Samba is the name of the game as well as of the dance; it is not "Samba mode".
  'Samba',
]);

function keysOfEnglish() {
  const src = fs.readFileSync(path.join(LOCALES, 'en.ts'), 'utf8');
  return new Set([...src.matchAll(/^\s*'([^']+)':/gm)].map((m) => m[1]));
}

async function main() {
  let modules;
  try {
    const res = await fetch(`${BASE}/modules`);
    modules = await res.json();
  } catch (err) {
    console.log(`server labels: skipped (${BASE} unreachable — start the dev stack to run this)`);
    return;
  }

  const known = keysOfEnglish();
  const unkeyed = [];
  /** bare key -> Map(label -> [moduleId]), to spot two games meaning different things. */
  const byBareKey = new Map();

  const note = (moduleId, bare, scoped, label) => {
    if (UNIVERSAL.test(label) || PROPER.has(label)) return;
    if (!known.has(scoped) && !known.has(bare)) unkeyed.push(`${scoped}  = ${JSON.stringify(label)}`);
  };

  for (const mod of modules.modules ?? modules) {
    const id = mod.id;
    if (!known.has(`module.${id}`) && !PROPER.has(mod.label)) unkeyed.push(`module.${id}  = ${JSON.stringify(mod.label)}`);
    for (const v of mod.variations ?? []) note(id, `variation.${id}.${v.id}`, `variation.${id}.${v.id}`, v.label);
    for (const opt of mod.options ?? []) {
      note(id, `option.${opt.name}`, `option.${id}.${opt.name}`, opt.label);
      for (const c of opt.choices ?? []) {
        const bare = `choice.${opt.name}.${c.value}`;
        note(id, bare, `choice.${id}.${opt.name}.${c.value}`, c.label);
        // Recorded only for modules still relying on the bare key. A module
        // with its own scoped wording has already opted out of the shared one,
        // so it cannot be in conflict with it — that is what resolving a
        // collision looks like, and the check has to be able to see it or it
        // reports work already done.
        if (known.has(`choice.${id}.${opt.name}.${c.value}`)) continue;
        if (!byBareKey.has(bare)) byBareKey.set(bare, new Map());
        const seen = byBareKey.get(bare);
        if (!seen.has(c.label)) seen.set(c.label, []);
        seen.get(c.label).push(id);
      }
    }
  }

  const collisions = [...byBareKey].filter(([, seen]) => seen.size > 1);

  for (const [bare, seen] of collisions) {
    console.log(`✗ ${bare} means different things:`);
    for (const [label, ids] of seen) console.log(`    ${JSON.stringify(label)} in ${ids.join(', ')}`);
    console.log(`    → give the odd one out its own key: choice.<moduleId>.${bare.slice('choice.'.length)}`);
  }
  for (const line of unkeyed) console.log(`✗ no wording anywhere: ${line}`);

  const bad = collisions.length + unkeyed.length;
  console.log(
    bad === 0
      ? `✓ server labels: ${byBareKey.size} choices, none colliding, none unworded`
      : `\n${bad} problem(s) in the labels the server sends.`,
  );
  process.exitCode = bad === 0 ? 0 : 1;
}

main();
