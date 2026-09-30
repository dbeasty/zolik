#!/usr/bin/env node
/**
 * Turns the vendored card art into a TypeScript module the client can draw.
 *
 * The art is Chris Aguilar's Vectorized Playing Cards (LGPL 3.0) — see
 * `client-react-native/src/cards/vector/NOTICE.txt`. It arrives as 55 SVG
 * files, and three things have to happen before a React Native client can use
 * them:
 *
 *  1. **No XML at runtime.** `SvgXml` would parse a card on every mount, on
 *     three platforms, with a parser whose web build we do not control. The
 *     shapes never change, so they are turned into data here, once, and the
 *     client walks an array.
 *  2. **The stock comes out.** Every face is drawn on a white rounded rect.
 *     `CardView` already draws the stock — with the skin's gradient, the
 *     selection tint and the joker's own fill — so the art's own white rect
 *     is dropped and the card shows through.
 *  3. **Colours become tokens.** The whole deck is drawn in five flat
 *     colours. Emitting them as `ink` / `red` / `gold` / `navy` / `stock`
 *     rather than as hex is what lets a skin restate the deck in its own
 *     palette without touching a single path (see `src/skins/types.ts`).
 *
 * Nothing here is run by the app or by CI: it is run by hand when the art
 * changes, and its output is committed. Regenerate with
 *
 *     node scripts/gen-card-faces.js
 *
 * The generated file stays LGPL, like the art it is derived from — which is
 * why it lives under `src/cards/vector/` next to the licence rather than
 * among the AGPL sources in `src/components/`.
 */

const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..', 'client-react-native', 'src', 'cards', 'vector');
const ART = path.join(ROOT, 'art');
const OUT = path.join(ROOT, 'faces.ts');

/**
 * The deck's five colours, named for what they do on a card rather than for
 * what they are: `gold` is the crowns and the filigree, `navy` the line work
 * in the courts' robes. A skin maps these; it does not know the hexes.
 *
 * `#ffff00` is the third joker's yellow, which is the same ink as the courts'
 * `#e2cf00` as far as anything downstream is concerned.
 */
const COLORS = {
  '#000': 'ink',
  '#000000': 'ink',
  '#d40000': 'red',
  '#e2cf00': 'gold',
  '#ff0': 'gold',
  '#ffff00': 'gold',
  '#131f67': 'navy',
  '#fff': 'stock',
  '#ffffff': 'stock',
  // One path on the king of diamonds is #100f08 rather than black. It is a
  // stray in the source art, not a sixth colour — a near-black nobody could
  // pick out of the line work beside it — so it is ink like the rest.
  '#100f08': 'ink',
};

/** `s13.svg` is the king of spades; `j03.svg` is the joker we ship. */
const SUITS = { c: 'C', d: 'D', h: 'H', s: 'S' };
const RANKS = ['A', '2', '3', '4', '5', '6', '7', '8', '9', '10', 'J', 'Q', 'K'];
const JOKER_FILE = 'j03';

function keyFor(name) {
  if (name.startsWith('j')) return name === JOKER_FILE ? 'JKR' : null;
  const suit = SUITS[name[0]];
  const rank = RANKS[Number(name.slice(1)) - 1];
  if (!suit || !rank) throw new Error(`unrecognised card file: ${name}.svg`);
  return `${rank}${suit}`;
}

/**
 * The fill a path actually has.
 *
 * Not simply `fill=`: the art states most of its colours in a `style`
 * attribute, and the paths that state nothing at all are black by SVG's own
 * default — which is most of the line work on every card. Treating "no fill"
 * as "not drawn" loses the courts entirely, so the default is explicit here.
 */
function fillOf(attrs) {
  const style = /style="([^"]*)"/.exec(attrs);
  const fromStyle = style && /(?:^|;)\s*fill:\s*([^;]+)/.exec(style[1]);
  const fromAttr = /\sfill="([^"]*)"/.exec(attrs);
  const raw = ((fromStyle && fromStyle[1]) || (fromAttr && fromAttr[1]) || '#000')
    .trim()
    .toLowerCase();
  // Nothing in this deck is stroked (every `stroke:` in the art is `none`), so
  // a path with no fill paints nothing at all and is left out rather than
  // being drawn in some guessed colour.
  if (raw === 'none') return null;
  const token = COLORS[raw];
  if (!token) throw new Error(`unmapped colour ${raw} — add it to COLORS or fix the art`);
  return token;
}

const attr = (attrs, name) => {
  const m = new RegExp(`\\s${name}="([^"]*)"`).exec(attrs);
  return m ? m[1] : null;
};

/**
 * Walks one card's SVG into a tree of groups and paths.
 *
 * The art is svgo-normalised, so the only elements left are `svg`, `g`,
 * `path` and the one `rect` of stock — which is why a tag scanner is enough
 * and this file carries no XML dependency. A shape this parser does not know
 * throws rather than being skipped: a silently dropped path is a card with a
 * hole in it, found much later and at much greater cost than a failed build.
 */
function parse(svg) {
  const viewBox = attr(/<svg([^>]*)>/.exec(svg)[1], 'viewBox');
  if (!viewBox) throw new Error('no viewBox');
  const stack = [{ k: [] }];
  const tag = /<(\/?)(svg|g|path|rect)([^>]*?)(\/?)>/g;
  let m;
  let dropped = 0;
  while ((m = tag.exec(svg))) {
    const [, close, name, attrs, selfClose] = m;
    if (name === 'svg') continue;
    if (close) {
      if (stack.length > 1) stack.pop();
      continue;
    }
    if (name === 'rect') {
      // The stock. Dropped on purpose — see the file comment.
      dropped += 1;
      continue;
    }
    if (name === 'path') {
      const d = attr(attrs, 'd');
      if (!d) throw new Error('path with no d');
      const c = fillOf(attrs);
      // A transform can sit on the path as well as on a group above it: svgo
      // pushes a group's transform down onto its children whenever that is
      // shorter. Both spellings mean the same thing and both have to be read,
      // or the card is drawn thousands of units off the page.
      const t = attr(attrs, 'transform');
      if (c) stack[stack.length - 1].k.push(t ? { d, c, t } : { d, c });
      continue;
    }
    // A group. Its transform is the only thing about it that matters; a
    // group with none is still kept, because flattening one would mean
    // composing matrices here instead of letting the renderer do it.
    const node = { g: attr(attrs, 'transform') || '', k: [] };
    stack[stack.length - 1].k.push(node);
    if (!selfClose) stack.push(node);
  }
  return { viewBox, nodes: stack[0].k, dropped };
}

/**
 * Refuses a card whose art would land outside the card.
 *
 * The deck is drawn in a space thousands of units across and pulled back into
 * the 63×88 viewBox by a matrix on the group above it. Optimising the SVGs is
 * therefore one rounding decision away from a disaster that is *invisible in
 * the file*: an earlier pass ran svgo at one decimal, which rounded some of
 * those matrices away entirely, and eight cards came out blank on a real
 * table while the other forty-five looked perfect. Spot-checking four of them
 * caught nothing.
 *
 * So: a path with no transform above it has to be drawn in card units. If its
 * coordinates run to the hundreds, the matrix that was supposed to bring them
 * back is gone, and this fails the build instead of shipping a blank card.
 */
function offViewBox(file, nodes) {
  (function walk(ns, transformed) {
    for (const n of ns) {
      if (!('d' in n)) {
        walk(n.k, transformed || !!n.g);
        continue;
      }
      if (transformed || n.t) continue;
      const far = (n.d.match(/-?\d+(\.\d+)?/g) || []).some((v) => Math.abs(Number(v)) > 200);
      if (far) {
        throw new Error(
          `${file}: a path with no transform above it is drawn far outside the ` +
            `63×88 card — the art's matrix was lost, probably by over-eager ` +
            `optimisation. Re-run svgo with ./art/svgo.config.mjs.`,
        );
      }
    }
  })(nodes, false);
}

const files = fs
  .readdirSync(ART)
  .filter((f) => f.endsWith('.svg'))
  .sort();

const faces = {};
let paths = 0;
for (const file of files) {
  const name = path.basename(file, '.svg');
  const key = keyFor(name);
  // j01 and j02 are the deck's other two jokers. The game deals one joker
  // face, so the others are kept in `art/` — unused, but there to be switched
  // to without going back to the source deck — and simply not generated.
  if (!key) continue;
  const { viewBox, nodes, dropped } = parse(fs.readFileSync(path.join(ART, file), 'utf8'));
  if (dropped > 1) throw new Error(`${file}: expected at most one stock rect, saw ${dropped}`);
  const count = (function walk(ns) {
    return ns.reduce((n, x) => n + (x.d ? 1 : walk(x.k)), 0);
  })(nodes);
  if (!count) throw new Error(`${file}: no paths survived`);
  offViewBox(file, nodes);
  paths += count;
  faces[key] = { viewBox, nodes };
}

const expected = 53; // 52 cards and the one joker
if (Object.keys(faces).length !== expected) {
  throw new Error(`generated ${Object.keys(faces).length} faces, expected ${expected}`);
}

/**
 * The colours the deck was actually printed in — what a skin gets by saying
 * nothing. Derived from the same table the art is read through, so the
 * unskinned deck cannot drift from what is in the SVGs.
 */
const PRINTED = {
  ink: '#000000',
  red: '#d40000',
  gold: '#e2cf00',
  navy: '#131f67',
  stock: '#ffffff',
};
for (const [token, hex] of Object.entries(PRINTED)) {
  if (COLORS[hex] !== token) throw new Error(`PRINTED.${token} is not the deck's ${token}`);
}

const header = `/**
 * The card faces, as data. GENERATED — do not edit.
 *
 * Run \`node scripts/gen-card-faces.js\` to rebuild this from the SVGs in
 * \`./art\`. That script is also where the shape of this file is explained.
 *
 * Vector Playing Cards 3.0
 * https://totalnonsense.com/open-source-vector-playing-cards/
 * Copyright 2011,2019 - Chris Aguilar - conjurenation@gmail.com
 * Licensed under: LGPL 3.0 - https://www.gnu.org/licenses/lgpl-3.0.html
 *
 * This file is a derivative of that artwork and is under the same licence,
 * not Zolik's AGPL — see ./NOTICE.txt and ./LICENSE.LGPL-3.0.txt.
 */

import type { CardPalette } from '@/src/skins/types';

/** The five inks the whole deck is drawn in. A skin says what each one is. */
export type FaceColor = 'ink' | 'red' | 'gold' | 'navy' | 'stock';

/** A filled path, or a group that transforms the ones inside it. */
export type FaceNode =
  | { d: string; c: FaceColor; t?: string }
  | { g: string; k: FaceNode[] };

export type Face = {
  /** Always "0 0 63 88" — a card is 63mm by 88mm, and the art is drawn to it. */
  viewBox: string;
  nodes: FaceNode[];
};

/** ${'What the deck looks like unskinned.'} */
export const PRINTED: CardPalette = {
${Object.entries(PRINTED)
  .map(([k, v]) => `  ${k}: '${v}',`)
  .join('\n')}
};

/** Keyed the way \`parseCard\` names a card: "KS", "10D", "AH", "JKR". */
export const FACES: Record<string, Face> = `;

fs.writeFileSync(OUT, `${header}${JSON.stringify(faces)};\n`);
console.log(
  `wrote ${path.relative(process.cwd(), OUT)} — ${Object.keys(faces).length} faces, ` +
    `${paths} paths, ${(fs.statSync(OUT).size / 1024).toFixed(0)} KB`,
);
