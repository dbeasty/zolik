/**
 * The German pack's artwork, as data.
 *
 * Everything a German-suited card is drawn from — the four suit signs, the
 * three court figures, the frame an ace and a court sit in — is a list of
 * filled and stroked paths built here, by plain functions with no React in
 * them. The components (`GermanSuit`, `GermanCourt`, `GermanFace`) only turn
 * the lists into `<Path>`s.
 *
 * It is data for the reason `src/cards/vector/faces.ts` is: nothing is parsed
 * at runtime, a test can ask what a card is made of without rendering it, and
 * the same lists can be written out as a plain `.svg` to look at the whole
 * pack on one sheet.
 *
 * This is original artwork, drawn for this client in the manner of the
 * double-ended Central European packs and traced from none of them. The free
 * German-suited set that exists (the xskat deck on Wikimedia Commons) is
 * CC BY-SA 3.0, which cannot be folded into an AGPL program the way the LGPL
 * French deck can — and it was drawn to be legally safe rather than to be
 * looked at.
 */

export type Shape = {
  d: string;
  fill?: string;
  stroke?: string;
  /** Stroke width, in the drawing's own units. */
  sw?: number;
  opacity?: number;
  /** An SVG transform, for a suit sign placed inside a larger drawing. */
  transform?: string;
};

export const GERMAN_SUITS = ['H', 'D', 'C', 'S'] as const;

/**
 * The pack's own colours. `fill` is the sign, `line` its outline and the
 * colour its index is printed in, `light` and `deep` the highlight and the
 * shaded side that make a flat sign read as a printed one.
 */
export const GERMAN_SUIT_COLORS: Record<string, { fill: string; line: string; light: string; deep: string }> = {
  D: { fill: '#EDB62F', line: '#6B4305', light: '#FFE79A', deep: '#C4820F' },
  C: { fill: '#C9853A', line: '#452610', light: '#F0BE7C', deep: '#7A4A21' },
  S: { fill: '#46A23C', line: '#17501F', light: '#9BD66A', deep: '#23742C' },
};

const SKIN = '#F4CFA8';
const SKIN_SHADE = '#D99E72';
const BLUSH = '#E58E73';
const HAIR = '#6A4322';
const HAIR_GREY = '#8C8478';
const GOLD = '#EDB62F';
const GOLD_LINE = '#6B4305';
const ERMINE = '#FBF6E9';

/** What a figure wears besides its suit's colour: a hat, a lapel, a cuff. */
const ACCENTS: Record<string, string> = {
  H: '#2F4E8F',
  D: '#A8382C',
  C: '#3C8A3A',
  S: '#D9A320',
};

/** A circle as a path, so a shape is only ever a path. */
function circle(cx: number, cy: number, r: number): string {
  return `M${cx - r} ${cy} A${r} ${r} 0 1 0 ${cx + r} ${cy} A${r} ${r} 0 1 0 ${cx - r} ${cy} Z`;
}

/**
 * A `#rgb` or `#rrggbb` colour moved towards black by `k` (0 to 1). The
 * heart's red belongs to the skin, so its outline and shaded side are worked
 * out from whatever the skin says rather than fixed here. Anything that is
 * not a hex colour comes back unchanged.
 */
export function darken(color: string, k: number): string {
  const m = /^#([0-9a-f]{3}|[0-9a-f]{6})$/i.exec(color);
  if (!m) return color;
  const hex = m[1].length === 3 ? m[1].replace(/./g, '$&$&') : m[1];
  const part = (i: number) =>
    Math.round(parseInt(hex.slice(i, i + 2), 16) * (1 - k))
      .toString(16)
      .padStart(2, '0');
  return `#${part(0)}${part(2)}${part(4)}`;
}

/**
 * A suit sign, in a 100×100 box. Each is a silhouette with an outline, a
 * shaded right-hand side and a highlight on the left — the light comes from
 * the top left on every sign, which is what makes four different pictures
 * look like one pack.
 */
export function suitShapes(suit: string, red: string): Shape[] {
  const c = GERMAN_SUIT_COLORS[suit];
  switch (suit) {
    case 'H': {
      const line = darken(red, 0.45);
      return [
        {
          d: 'M50 93 C36 80 7 61 7 34 C7 19 18 8 32 8 C40 8 47 12 50 20 C53 12 60 8 68 8 C82 8 93 19 93 34 C93 61 64 80 50 93 Z',
          fill: red,
          stroke: line,
          sw: 3.5,
        },
        {
          d: 'M50 93 C64 80 93 61 93 34 C93 19 82 8 68 8 C60 8 53 12 50 20 C62 24 66 50 50 93 Z',
          fill: '#000000',
          opacity: 0.2,
        },
        { d: 'M19 35 C18 25 24 18 33 17', stroke: '#FFFFFF', sw: 5, opacity: 0.6 },
      ];
    }
    case 'D':
      // A hawk bell: the loop it hangs from, the round body, a band about
      // its middle and the slot across its foot with a hole at each end.
      return [
        { d: circle(50, 13, 8), stroke: c.line, sw: 5 },
        { d: circle(50, 57, 36), fill: c.fill, stroke: c.line, sw: 3.5 },
        { d: 'M50 21 A36 36 0 0 1 50 93 A47 47 0 0 0 50 21 Z', fill: c.deep, opacity: 0.75 },
        { d: 'M15 50 Q50 39 85 50 L86 60 Q50 49 14 60 Z', fill: c.line },
        { d: 'M24 53.5 Q50 44.5 76 53.5', stroke: c.light, sw: 1.8, opacity: 0.85 },
        { d: 'M33 79 Q50 71 67 79', stroke: c.line, sw: 4 },
        { d: circle(33, 79, 4.2), fill: c.line },
        { d: circle(67, 79, 4.2), fill: c.line },
        { d: 'M23 40 C26 32 33 27 41 25', stroke: c.light, sw: 5 },
      ];
    case 'C':
      // An acorn hanging from its stalk: a cross-hatched cup over the nut.
      return [
        { d: 'M50 22 C50 14 53 8 61 4', stroke: c.line, sw: 6 },
        { d: 'M29 46 C29 70 39 88 50 97 C61 88 71 70 71 46 Z', fill: c.fill, stroke: c.line, sw: 3.5 },
        { d: 'M50 97 C61 88 71 70 71 46 L58 46 C59 68 56 84 50 97 Z', fill: c.deep, opacity: 0.7 },
        { d: 'M37 56 C37.5 66 40 75 44 82', stroke: c.light, sw: 4.5 },
        {
          d: 'M21 50 C18 31 33 19 50 19 C67 19 82 31 79 50 C70 56 30 56 21 50 Z',
          fill: c.deep,
          stroke: c.line,
          sw: 3.5,
        },
        {
          d: 'M31 31 L43 50 M43 24 L57 51 M57 24 L69 47 M69 31 L57 50 M57 24 L43 51 M43 24 L31 47',
          stroke: c.light,
          sw: 2,
          opacity: 0.6,
        },
      ];
    case 'S':
      // A leaf, point up, on a curling stalk.
      return [
        { d: 'M50 68 C50 82 47 90 41 96', stroke: c.line, sw: 6 },
        {
          d: 'M50 4 C60 20 92 36 92 60 C92 76 78 85 65 81 C58 79 53 75 50 69 C47 75 42 79 35 81 C22 85 8 76 8 60 C8 36 40 20 50 4 Z',
          fill: c.fill,
          stroke: c.line,
          sw: 3.5,
        },
        {
          d: 'M50 4 C60 20 92 36 92 60 C92 76 78 85 65 81 C58 79 53 75 50 69 Z',
          fill: c.deep,
          opacity: 0.7,
        },
        { d: 'M50 15 L50 70', stroke: c.line, sw: 3.2 },
        {
          d: 'M50 36 L38 25 M50 36 L62 25 M50 51 L29 38 M50 51 L71 38 M50 64 L28 57 M50 64 L72 57',
          stroke: c.line,
          sw: 2.4,
        },
        { d: 'M20 62 C19 50 27 40 35 33', stroke: c.light, sw: 4, opacity: 0.85 },
      ];
  }
  return [];
}

/** A suit sign placed inside a larger drawing, `size` units across at (x, y). */
export function placedSuit(suit: string, red: string, x: number, y: number, size: number, flipped = false): Shape[] {
  const transform = `translate(${x} ${y}) scale(${size / 100})${flipped ? ' rotate(180 50 50)' : ''}`;
  return suitShapes(suit, red).map((s) => ({ ...s, transform }));
}

/** The colour a suit's panel is washed with. */
export function suitTint(suit: string, red: string): string {
  return suit === 'H' ? red : GERMAN_SUIT_COLORS[suit]?.fill ?? red;
}

/** The colour a suit's line work — its frame, its index — is printed in. */
export function suitLine(suit: string, red: string): string {
  return suit === 'H' ? red : GERMAN_SUIT_COLORS[suit]?.line ?? red;
}

/**
 * The room a corner index takes out of the panel, in the panel's own units:
 * how far in from the side and how far down from the top. `GermanFace` works
 * it out from the index it actually prints, because a Czech "Sv" is wider
 * than a German "O".
 */
export type Notch = { w: number; h: number };

/**
 * The printed frame a court or an ace sits in: a rule and a hairline inside
 * it, for a panel 100 wide and `span` tall, washed with a little of the
 * suit's own colour.
 *
 * The frame steps in around the two corner indices rather than running
 * behind them — the index is type laid over the card, and a rule struck
 * through a letter is the first thing that makes a drawn card look drawn.
 */
export function frameShapes(span: number, color: string, notch: Notch, tint?: string): Shape[] {
  const box = (inset: number, r: number) => {
    const a = inset;
    const b = 100 - inset;
    const c = span - inset;
    const nw = notch.w + inset;
    const nh = notch.h + inset;
    const mw = 100 - nw;
    const mh = span - nh;
    return (
      `M${nw + r} ${a} L${b - r} ${a} Q${b} ${a} ${b} ${a + r} ` +
      `L${b} ${mh - r} Q${b} ${mh} ${b - r} ${mh} L${mw + r} ${mh} Q${mw} ${mh} ${mw} ${mh + r} ` +
      `L${mw} ${c - r} Q${mw} ${c} ${mw - r} ${c} L${a + r} ${c} Q${a} ${c} ${a} ${c - r} ` +
      `L${a} ${nh + r} Q${a} ${nh} ${a + r} ${nh} L${nw - r} ${nh} Q${nw} ${nh} ${nw} ${nh - r} ` +
      `L${nw} ${a + r} Q${nw} ${a} ${nw + r} ${a} Z`
    );
  };
  return [
    { d: box(1.5, 3.5), fill: tint, opacity: 0.09 },
    { d: box(1.5, 3.5), stroke: color, sw: 1.8, opacity: 0.8 },
    { d: box(4.6, 2), stroke: color, sw: 0.7, opacity: 0.6 },
  ];
}

/**
 * The eso: one large sign in a roundel, with a flourish above and below it.
 * A German ace is a panel rather than a pip, and this is the panel.
 */
export function aceShapes(suit: string, red: string, span: number, notch: Notch): Shape[] {
  const line = suitLine(suit, red);
  const cy = span / 2;
  const flourish = (y: number, dir: 1 | -1): Shape[] => [
    {
      d: `M50 ${y} C44 ${y - 7 * dir} 33 ${y - 8 * dir} 27 ${y - 2 * dir} C24 ${y + 2 * dir} 28 ${y + 6 * dir} 32 ${y + 3 * dir} M50 ${y} C56 ${y - 7 * dir} 67 ${y - 8 * dir} 73 ${y - 2 * dir} C76 ${y + 2 * dir} 72 ${y + 6 * dir} 68 ${y + 3 * dir}`,
      stroke: line,
      sw: 1.5,
      opacity: 0.85,
    },
    {
      d: `M50 ${y - 5 * dir} L53 ${y - 10 * dir} L50 ${y - 15 * dir} L47 ${y - 10 * dir} Z`,
      fill: line,
      opacity: 0.85,
    },
  ];
  return [
    ...frameShapes(span, line, notch, suitTint(suit, red)),
    { d: circle(50, cy, 40), stroke: line, sw: 1.6, opacity: 0.8 },
    { d: circle(50, cy, 36.5), stroke: line, sw: 0.7, opacity: 0.6 },
    ...flourish(cy - 50, 1),
    ...flourish(cy + 50, -1),
    ...placedSuit(suit, red, 22, cy - 28, 56),
  ];
}

export type CourtInks = {
  /** The figure's line work — the card's ink. */
  ink: string;
  /** The heart's red, from the skin. */
  red: string;
  /** The card's own stock: shirts, collars and the whites of things. */
  stock: string;
};

/** How far from the centre line each figure's coat reaches at the waist. */
const REACH: Record<string, number> = { K: 39, Q: 33, J: 26 };

/** The face every figure shares, so the three read as one family. */
function faceShapes(ink: string): Shape[] {
  return [
    { d: circle(38.2, 34.5, 2.8), fill: SKIN, stroke: ink, sw: 1.3 },
    { d: circle(61.8, 34.5, 2.8), fill: SKIN, stroke: ink, sw: 1.3 },
    {
      d: 'M38.5 29 C38.5 20 43.5 15.5 50 15.5 C56.5 15.5 61.5 20 61.5 29 L61.5 36 C61.5 45 56 50.5 50 50.5 C44 50.5 38.5 45 38.5 36 Z',
      fill: SKIN,
      stroke: ink,
      sw: 1.8,
    },
    { d: circle(43, 40, 2.6), fill: BLUSH, opacity: 0.4 },
    { d: circle(57, 40, 2.6), fill: BLUSH, opacity: 0.4 },
    { d: circle(45.2, 33.6, 1.5), fill: ink },
    { d: circle(54.8, 33.6, 1.5), fill: ink },
    { d: 'M42.6 30.4 Q45.2 28.8 47.6 30.1 M52.4 30.1 Q54.8 28.8 57.4 30.4', stroke: ink, sw: 1.1 },
    { d: 'M50 34.5 L48.8 39.8 L51.4 40', stroke: SKIN_SHADE, sw: 1.3 },
  ];
}

const MOUTH = 'M46.6 44.2 Q50 46.2 53.4 44.2';

/**
 * One half of a court card: a figure from the waist up, in a box 100 wide
 * whose bottom edge is `waist`. `GermanCourt` draws it twice, the second
 * turned about the waist.
 *
 * The German pack tells its two lower courts apart by where the suit sign
 * is, not by what they wear: the svršek (Ober) holds his up by his head, the
 * spodek (Unter) holds his down at his side. That rule is what a player
 * looks for, so it is what the drawing is built around; the feathered cap,
 * the brimmed hat and the crown only back it up. The král keeps his sign up
 * beside his crown and a sceptre in his other hand.
 *
 * Signs sit on the figure's right of the panel, clear of the corner index,
 * which owns the top left of every card.
 */
export function courtShapes(rank: string, suit: string, waist: number, inks: CourtInks): Shape[] {
  const { ink, red, stock } = inks;
  const coat = suit === 'H' ? red : GERMAN_SUIT_COLORS[suit]?.fill ?? ink;
  const coatDeep = suit === 'H' ? darken(red, 0.28) : GERMAN_SUIT_COLORS[suit]?.deep ?? ink;
  const accent = ACCENTS[suit] ?? ink;
  const w = waist;
  const neck: Shape = { d: 'M45 46 L45 59 L55 59 L55 46 Z', fill: SKIN_SHADE };

  switch (rank) {
    case 'K':
      return [
        // The robe, shaded down its right-hand side.
        {
          d: `M11 ${w} L11 78 C11 66 22 58 38 55 L62 55 C78 58 89 66 89 78 L89 ${w} Z`,
          fill: coat,
          stroke: ink,
          sw: 2,
        },
        { d: `M70 58.5 C82 62 89 68 89 78 L89 ${w} L74 ${w} Z`, fill: coatDeep, opacity: 0.75 },
        { d: `M23 70 L23 ${w} M77 70 L77 ${w}`, stroke: ink, sw: 1.4 },
        // A gold orphrey down the front, set with stones.
        { d: `M45 64 L55 64 L55 ${w} L45 ${w} Z`, fill: GOLD, stroke: ink, sw: 1.2 },
        { d: 'M50 70.5 L52.4 73.5 L50 76.5 L47.6 73.5 Z', fill: accent },
        { d: circle(50, 81, 1.5), fill: ink },
        neck,
        // An ermine collar across the shoulders.
        {
          d: 'M24 63 C32 54 44 53 50 59 C56 53 68 54 76 63 C68 71 56 71 50 66 C44 71 32 71 24 63 Z',
          fill: ERMINE,
          stroke: ink,
          sw: 1.6,
        },
        {
          d: 'M31 61.5 L31 64.5 M38 58.5 L38 61.5 M38 65.5 L38 67.5 M62 58.5 L62 61.5 M62 65.5 L62 67.5 M69 61.5 L69 64.5',
          stroke: ink,
          sw: 1.7,
        },
        // Hair to the collar, then the head, then a full beard over both.
        { d: 'M38 22 C33 32 34 46 40 54 L44 50 L40 28 Z', fill: HAIR_GREY, stroke: ink, sw: 1.2 },
        { d: 'M62 22 C67 32 66 46 60 54 L56 50 L60 28 Z', fill: HAIR_GREY, stroke: ink, sw: 1.2 },
        ...faceShapes(ink),
        {
          d: 'M38.8 37 C38.8 51 44 59.5 50 59.5 C56 59.5 61.2 51 61.2 37 C58.5 41.5 54.5 43 50 43 C45.5 43 41.5 41.5 38.8 37 Z',
          fill: HAIR_GREY,
          stroke: ink,
          sw: 1.4,
        },
        { d: 'M43.6 42.6 Q47 40 50 42.4 Q53 40 56.4 42.6 Q53 45.4 50 43.8 Q47 45.4 43.6 42.6 Z', fill: ERMINE, stroke: ink, sw: 0.8 },
        { d: 'M47.4 47.4 Q50 48.8 52.6 47.4', stroke: ink, sw: 1.1 },
        // The crown.
        {
          d: 'M36.5 17.5 L34.5 4 L43 11.5 L50 2 L57 11.5 L65.5 4 L63.5 17.5 Z',
          fill: GOLD,
          stroke: ink,
          sw: 1.6,
        },
        { d: 'M36 17 L64 17 L64 23 L36 23 Z', fill: GOLD, stroke: ink, sw: 1.6 },
        { d: circle(50, 20, 1.7), fill: accent },
        { d: circle(43, 20, 1.2), fill: ink },
        { d: circle(57, 20, 1.2), fill: ink },
        { d: circle(34.5, 4, 1.7), fill: accent, stroke: ink, sw: 0.7 },
        { d: circle(50, 2.6, 1.7), fill: accent, stroke: ink, sw: 0.7 },
        { d: circle(65.5, 4, 1.7), fill: accent, stroke: ink, sw: 0.7 },
        // The sceptre, in his right hand.
        { d: `M18.4 30 L21.6 30 L21.6 ${w} L18.4 ${w} Z`, fill: GOLD, stroke: ink, sw: 1 },
        { d: circle(20, 25.5, 5), fill: GOLD, stroke: ink, sw: 1.4 },
        { d: 'M15.2 25.5 L24.8 25.5', stroke: ink, sw: 1 },
        { d: 'M20 20.5 L20 13.5 M17.2 16.5 L22.8 16.5', stroke: ink, sw: 1.7 },
        { d: circle(20, 63, 4.6), fill: SKIN, stroke: ink, sw: 1.4 },
        ...placedSuit(suit, red, 73, 4, 24),
      ];
    case 'Q':
      return [
        {
          d: `M17 ${w} L17 77 C17 67 26 60 39 56 L61 56 C74 60 83 67 83 77 L83 ${w} Z`,
          fill: coat,
          stroke: ink,
          sw: 2,
        },
        { d: `M67 58.5 C77 62 83 68 83 77 L83 ${w} L70 ${w} Z`, fill: coatDeep, opacity: 0.75 },
        // His other arm, down at his side.
        { d: `M28 68 L28 ${w}`, stroke: ink, sw: 1.4 },
        { d: `M50 67 L50 ${w}`, stroke: ink, sw: 1.4 },
        { d: circle(50, 73, 1.7), fill: GOLD, stroke: ink, sw: 0.8 },
        { d: circle(50, 80, 1.7), fill: GOLD, stroke: ink, sw: 0.8 },
        neck,
        // A shirt front between two turned-back lapels.
        { d: 'M42 55 L50 68 L58 55 Z', fill: stock, stroke: ink, sw: 1.2 },
        { d: 'M37.5 56.5 L42 55 L50 68 L45.5 73 Z', fill: accent, stroke: ink, sw: 1.2 },
        { d: 'M62.5 56.5 L58 55 L50 68 L54.5 73 Z', fill: accent, stroke: ink, sw: 1.2 },
        // The arm that holds the sign up.
        {
          d: 'M67 59.5 C80 61 90 51 89.5 38 L80.5 37 C80.5 46 75 50 63 54.5 Z',
          fill: coat,
          stroke: ink,
          sw: 2,
        },
        { d: 'M80 37.5 L90 38.5 L90.4 33.5 L80.4 32.5 Z', fill: accent, stroke: ink, sw: 1.2 },
        { d: circle(85.2, 29.4, 4.4), fill: SKIN, stroke: ink, sw: 1.4 },
        { d: 'M38 23 C34 32 35 44 40.5 50 L40 28 Z', fill: HAIR, stroke: ink, sw: 1.2 },
        { d: 'M62 23 C66 32 65 44 59.5 50 L60 28 Z', fill: HAIR, stroke: ink, sw: 1.2 },
        ...faceShapes(ink),
        { d: 'M44.4 42 Q47.4 39.8 50 41.6 Q52.6 39.8 55.6 42 Q52.6 44 50 42.8 Q47.4 44 44.4 42 Z', fill: HAIR, stroke: ink, sw: 0.7 },
        { d: 'M47.2 46 Q50 47.4 52.8 46', stroke: ink, sw: 1.1 },
        // A brimmed hat with a band and a plume: the svršek's. The plume is
        // kept short, clear of a two-letter index.
        { d: 'M37.5 18.5 C36.5 5 63.5 5 62.5 18.5 Z', fill: ink },
        { d: 'M37.4 14 L62.6 14 L62.8 18.6 L37.2 18.6 Z', fill: accent },
        { d: 'M28 19.5 C28 16 72 16 72 19.5 C72 23.5 28 23.5 28 19.5 Z', fill: ink },
        { d: 'M42 11.5 C35 12.5 29 9 26.5 3 C33 2.5 40 4.5 44 8.5 Z', fill: accent, stroke: ink, sw: 1.1 },
        { d: 'M41 10 C36 9.6 31.5 7.4 28.4 4.2', stroke: ink, sw: 0.7, opacity: 0.7 },
        ...placedSuit(suit, red, 73.2, 2, 24),
      ];
    case 'J':
      return [
        {
          d: `M24 ${w} L24 78 C24 68 31 61 41 57 L59 57 C69 61 76 68 76 78 L76 ${w} Z`,
          fill: coat,
          stroke: ink,
          sw: 2,
        },
        { d: `M63 59 C70.5 62.5 76 69 76 78 L76 ${w} L66 ${w} Z`, fill: coatDeep, opacity: 0.75 },
        // His other arm, down at his side.
        { d: `M33 70 L33 ${w}`, stroke: ink, sw: 1.4 },
        // A jerkin laced down the front.
        { d: `M50 63 L50 ${w}`, stroke: ink, sw: 1.2 },
        { d: 'M46 68 L54 72 M54 68 L46 72 M46 75.5 L54 79.5 M54 75.5 L46 79.5', stroke: ink, sw: 1.1 },
        neck,
        // A soft white collar.
        {
          d: 'M37 58.5 C41 65 46.5 65.5 50 61.5 C53.5 65.5 59 65 63 58.5 L57.5 54.5 L42.5 54.5 Z',
          fill: stock,
          stroke: ink,
          sw: 1.3,
        },
        // The arm that holds the sign down at his side.
        { d: 'M66 60.5 C72 56 78 53 84 51.5 L86.5 58.5 C81 60 77 63.5 72.5 69 Z', fill: coat, stroke: ink, sw: 2 },
        { d: 'M82.4 52 L86.8 50.8 L89 57.4 L84.8 58.8 Z', fill: accent, stroke: ink, sw: 1.1 },
        { d: circle(89.4, 54.8, 3.9), fill: SKIN, stroke: ink, sw: 1.4 },
        // Hair cut square to the jaw.
        { d: 'M38 22 C34 30 34.5 42 38 48 L42 47 L40 28 Z', fill: HAIR, stroke: ink, sw: 1.2 },
        { d: 'M62 22 C66 30 65.5 42 62 48 L58 47 L60 28 Z', fill: HAIR, stroke: ink, sw: 1.2 },
        ...faceShapes(ink),
        { d: MOUTH, stroke: ink, sw: 1.2 },
        // A soft cap with a feather: no brim, no points.
        { d: 'M36 20 C31 9 44 3 55 4.5 C66 6 69.5 13 64.5 20 Z', fill: accent, stroke: ink, sw: 1.6 },
        { d: 'M35.8 18.5 L64.2 18.5 L64.2 23 L35.8 23 Z', fill: ink },
        { d: 'M60 10.5 C65 3 74 1.5 81 2.5 C78 9 70 13.5 62 14 Z', fill: ERMINE, stroke: ink, sw: 1.1 },
        { d: 'M62 12.6 C68 9.6 74 6 79 3.4', stroke: ink, sw: 0.7, opacity: 0.7 },
        ...placedSuit(suit, red, 77, 60.5, 21.5),
      ];
  }
  return [];
}

/**
 * A whole court panel, 100 wide and `span` tall: the frame, the figure, the
 * same figure turned about the waist, and the belt that joins them.
 */
export function courtPanel(rank: string, suit: string, span: number, notch: Notch, inks: CourtInks): Shape[] {
  const reach = REACH[rank];
  if (!reach) return [];
  const waist = span / 2;
  const half = courtShapes(rank, suit, waist, inks);
  const turn = `rotate(180 50 ${waist})`;
  return [
    ...frameShapes(span, suitLine(suit, inks.red), notch, suitTint(suit, inks.red)),
    ...half,
    ...half.map((s) => ({ ...s, transform: s.transform ? `${turn} ${s.transform}` : turn })),
    {
      d: `M${50 - reach - 2} ${waist - 2.6} L${50 + reach + 2} ${waist - 2.6} L${50 + reach + 2} ${waist + 2.6} L${50 - reach - 2} ${waist + 2.6} Z`,
      fill: GOLD,
      stroke: inks.ink,
      sw: 1.2,
    },
    { d: `M50 ${waist - 4.4} L54.4 ${waist} L50 ${waist + 4.4} L45.6 ${waist} Z`, fill: ACCENTS[suit] ?? GOLD_LINE, stroke: inks.ink, sw: 1 },
  ];
}
