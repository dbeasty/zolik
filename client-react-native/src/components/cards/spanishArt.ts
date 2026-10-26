/**
 * The Spanish pack's artwork, as data.
 *
 * The four Spanish signs — oros (coins), copas (cups), espadas (swords) and
 * bastos (clubs) — drawn the way `germanArt.ts` draws its four: a silhouette
 * with an outline, a shaded right-hand side and a highlight on the left, in a
 * 100×100 box, with the light coming from the top left on every sign. That is
 * what makes four different pictures look like one pack.
 *
 * Everything else a Spanish card needs is already in `germanArt.ts` and is
 * used from there rather than copied: the frame a court or an ace sits in, the
 * ace's roundel, and the double-ended court figure. The two packs differ in
 * their signs and their colours; they do not differ in how a court card is
 * laid out, and two copies of a figure would drift the first time one of them
 * was touched. `SuitPack` is the seam.
 *
 * What that costs is worth saying plainly: a real baraja española prints its
 * sota, caballo and rey as single full-length figures, not as the double-ended
 * courts drawn here. The signs, the colours, the indices and the forty cards
 * are the pack; the figures are this client's own, in the same hand as its
 * German ones.
 *
 * Suits map to the server's French codes as the French pack's own suits
 * descend from these: H→copas, D→oros, C→bastos, S→espadas. So "KD" is the
 * rey de oros, and nothing on the wire had to learn a new notation.
 *
 * Original artwork, drawn for this client and traced from no pack.
 */

import { circle, type Shape, type SuitPack } from '@/src/components/cards/germanArt';

export const SPANISH_SUITS = ['H', 'D', 'C', 'S'] as const;

/**
 * The pack's own colours — gold coins, steel swords, wooden clubs. Copas has
 * no entry for the reason the German pack's hearts has none: a cup is printed
 * red, and the red belongs to the skin, so its outline and its shaded side are
 * worked out from whatever the skin says.
 */
export const SPANISH_SUIT_COLORS: Record<string, { fill: string; line: string; light: string; deep: string }> = {
  D: { fill: '#EDB62F', line: '#6B4305', light: '#FFE79A', deep: '#C4820F' },
  S: { fill: '#7D97BC', line: '#223049', light: '#CBDAED', deep: '#47628C' },
  C: { fill: '#9A6B3A', line: '#3E2612', light: '#D6A878', deep: '#6B4624' },
};

/** What a figure wears besides its suit's colour: a hat, a lapel, a cuff. */
export const SPANISH_ACCENTS: Record<string, string> = {
  H: '#2F4E8F',
  D: '#A8382C',
  C: '#3C8A3A',
  S: '#D9A320',
};

/** A `#rrggbb` moved towards black by `k`, for the red the skin hands us. */
function shade(color: string, k: number): string {
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
 * A Spanish suit sign, in a 100×100 box.
 *
 * Each is built the same way: the silhouette first, then the shaded side over
 * it, then the highlight, then the detail that tells a player what it is — the
 * rosette struck on a coin, the knots on a club, the fuller down a blade.
 */
export function spanishSign(suit: string, red: string): Shape[] {
  const c = SPANISH_SUIT_COLORS[suit];
  switch (suit) {
    // Copas: a footed goblet. The one sign printed in the skin's red, so a
    // Spanish hand matches the rest of the board under every skin.
    case 'H': {
      const line = shade(red, 0.45);
      const deep = shade(red, 0.25);
      return [
        // The bowl, and the band round its rim.
        { d: 'M25 23 L75 23 C75 49 65 62 50 62 C35 62 25 49 25 23 Z', fill: red, stroke: line, sw: 2 },
        { d: 'M70 25 C70 48 62 59 52 61.6 C62 57 66 44 66 25 Z', fill: deep, opacity: 0.8 },
        { d: 'M31 27 C31 44 35 54 42 58.5', stroke: '#FFFFFF', sw: 2.6, opacity: 0.35 },
        { d: 'M21 15 L79 15 L79 24 L21 24 Z', fill: red, stroke: line, sw: 1.6 },
        { d: 'M21 19.5 L79 19.5', stroke: line, sw: 0.8, opacity: 0.6 },
        // Stem, knop and foot.
        { d: 'M45.5 62 L54.5 62 L54.5 78 L45.5 78 Z', fill: red, stroke: line, sw: 1.6 },
        { d: circle(50, 70, 5.4), fill: red, stroke: line, sw: 1.4 },
        { d: 'M28 92 C28 83 38 78.5 50 78.5 C62 78.5 72 83 72 92 Z', fill: red, stroke: line, sw: 1.8 },
        { d: 'M60 80 C68 82 72 86 72 92 L62 92 Z', fill: deep, opacity: 0.7 },
      ];
    }
    // Oros: a struck coin, rim and rosette.
    case 'D':
      return [
        { d: circle(50, 50, 42), fill: c.fill, stroke: c.line, sw: 2 },
        { d: circle(57, 57, 33), fill: c.deep, opacity: 0.45 },
        { d: circle(41, 41, 26), fill: c.light, opacity: 0.55 },
        { d: circle(50, 50, 34), stroke: c.line, sw: 1.3, opacity: 0.7 },
        { d: circle(50, 50, 30.5), stroke: c.line, sw: 0.7, opacity: 0.5 },
        // A six-petalled rosette, which is what a coin of oros is struck with.
        {
          d:
            'M50 29 L55 42 L68 37 L60 50 L68 63 L55 58 L50 71 L45 58 L32 63 L40 50 L32 37 L45 42 Z',
          fill: c.line,
          opacity: 0.8,
        },
        { d: circle(50, 50, 5.2), fill: c.fill, stroke: c.line, sw: 1.1 },
      ];
    // Bastos: a knotted cudgel, leaning as it is printed.
    case 'C':
      return [
        {
          d:
            'M22 91 C17 86 18 78 25 75 L63 21 C67 12 78 9 85 15 C92 21 90 33 81 35 L39 87 C34 93 27 95 22 91 Z',
          fill: c.fill,
          stroke: c.line,
          sw: 2,
        },
        // The shaded under-side, and the lit upper edge.
        { d: 'M36 84 L78 32 C84 31 88 27 89 22 C91 30 88 34 81 35 L39 87 C34 93 27 95 22 91 C28 93 33 89 36 84 Z', fill: c.deep, opacity: 0.8 },
        { d: 'M27 79 L66 24', stroke: c.light, sw: 3, opacity: 0.55 },
        // Knots, which is what tells a club from a sword at a glance.
        { d: circle(74, 30, 4.6), fill: c.deep, stroke: c.line, sw: 1 },
        { d: circle(55, 53, 4.2), fill: c.deep, stroke: c.line, sw: 1 },
        { d: circle(36, 76, 3.8), fill: c.deep, stroke: c.line, sw: 1 },
      ];
    // Espadas: a straight blade, point up, with the curved guard of the pack.
    case 'S':
      return [
        { d: 'M50 5 L59 25 L59 57 L41 57 L41 25 Z', fill: c.fill, stroke: c.line, sw: 1.8 },
        { d: 'M50 8 L59 26 L59 57 L50 57 Z', fill: c.deep, opacity: 0.55 },
        { d: 'M46 14 L46 55', stroke: c.light, sw: 2.2, opacity: 0.7 },
        { d: 'M50 11 L50 56', stroke: c.line, sw: 0.7, opacity: 0.45 },
        // The guard.
        { d: 'M19 62 C29 55 71 55 81 62 C71 69 29 69 19 62 Z', fill: c.deep, stroke: c.line, sw: 1.7 },
        { d: 'M24 61 C34 57 66 57 76 61', stroke: c.light, sw: 1.4, opacity: 0.55 },
        // Grip and pommel.
        { d: 'M44.5 68 L55.5 68 L55.5 84 L44.5 84 Z', fill: c.deep, stroke: c.line, sw: 1.5 },
        { d: 'M47.5 70 L47.5 82 M52.5 70 L52.5 82', stroke: c.line, sw: 0.7, opacity: 0.6 },
        { d: circle(50, 89, 6.4), fill: c.fill, stroke: c.line, sw: 1.5 },
        { d: circle(47.6, 86.8, 2.4), fill: c.light, opacity: 0.7 },
      ];
  }
  return [];
}

/** The Spanish-suited forty, as a pack the shared printing can take. */
export const SPANISH_PACK: SuitPack = {
  id: 'spanish',
  suits: SPANISH_SUITS,
  colors: SPANISH_SUIT_COLORS,
  accents: SPANISH_ACCENTS,
  sign: spanishSign,
};
