import Svg, { G, Path, Circle as SvgCircle, Rect } from 'react-native-svg';

/**
 * The figure on a jack, a queen, a king or a joker.
 *
 * Drawn the way a printed court card is drawn, which is the only thing that
 * makes one recognisable as a court card at a glance: a half-figure in the
 * top half of the panel, and the *same* half-figure rotated 180° in the
 * bottom, so the card reads from either end of the table. That mirroring is
 * the court equivalent of the rule the pips keep (see `src/lib/pips.ts`), and
 * for the same reason — a card printed one way up is a poster.
 *
 * The joker belongs here rather than beside it: it is a figure card of no
 * suit, mirrored about the same waist as the other three. Giving it a star
 * instead — which is what the board did before — made the one wild card in
 * the deck look like a card with its face missing.
 *
 * Stylised rather than illustrated, on purpose. A real court card is a dense
 * woodcut, and a woodcut redrawn at 40 pixels across is a smudge. What
 * survives at this size is the silhouette: what is on the head, a face under
 * it, and a robe below. That is enough to tell a king from a jack across a
 * table, which is all a court figure has ever had to do.
 *
 * **The design space is 100 wide and as tall as the panel actually is.** Not
 * a square: an SVG viewBox that doesn't match its box is letterboxed inside
 * it, so a 100×100 court drawn into a tall panel shrinks to fit the *width*
 * and centres — which put both mirrored halves in a band across the middle of
 * the card, overlapping each other, with bare stock above and below.
 *
 * The figures are therefore drawn for the shape a half actually is, which is
 * `HALF` below: a card is about 1.39 times as tall as it is wide, the pip
 * field keeps most of that, and half of it comes out near enough 100 × 87 —
 * upright, not the wide-and-short 100 × 50 a square viewBox implies. Getting
 * this wrong does not crop anything or throw; it just quietly draws a small
 * figure at the top of a large empty panel, which is why it is written down.
 */

type Props = {
  /** 'J' | 'Q' | 'K' | 'JKR'. Anything else draws nothing. */
  rank: string;
  width: number;
  height: number;
  /** The figure's line and fill — the card's ink, or its red. */
  color: string;
  /** The crown, the cap band, the collar trim. Falls back to `color`. */
  accent?: string;
  /**
   * The card's own stock, under the figure.
   *
   * Not decoration: it is what keeps the figure from fusing into one blob.
   * Every part of a court is drawn in the same ink, so a face touching a
   * beard touching a collar is a single silhouette with no features in it —
   * which is exactly what the first cut of this looked like. Drawing the face
   * *in the stock colour* over the shapes behind it punches it back out, the
   * way the white of the card does on a printed one.
   */
  stock: string;
};

const FIGURES = ['J', 'Q', 'K', 'JKR'];

/**
 * How tall a half is in design units, for a panel 100 wide.
 *
 * Derived, not chosen: a card is 72/52 as tall as it is wide, the pip field
 * takes 88% of the height and 70% of the width, and half of the result is
 * `0.5 * (72/52) * (0.88/0.70) * 100` — about 87. Every figure below is drawn
 * to fill this; a panel whose own half differs (a compact card rounds
 * differently) makes up the difference with `robe` rather than by squashing
 * the figure.
 */
const HALF = 87;

/**
 * Where each figure's shoulders meet the bottom of its half, so a panel
 * slightly taller than `HALF` is filled by carrying the robe down rather than
 * by leaving a band of bare stock above the waist.
 *
 * Per rank because the shoulders are different widths, and a skirt wider than
 * the shoulders above it is a figure wearing a table.
 */
const SKIRT: Record<string, { x: number; width: number }> = {
  K: { x: 20, width: 60 },
  Q: { x: 22, width: 56 },
  J: { x: 26, width: 48 },
  JKR: { x: 28, width: 44 },
};

/**
 * Each figure fills the top `HALF` of the design space and is drawn twice by
 * `Court` below. Written half-height rather than full so the two copies
 * cannot drift apart: there is one figure, and the other is it.
 *
 * The three shapes each of them is built from are the same three, in the same
 * order — what is on the head, the face, the body — because what tells a king
 * from a jack at 40 pixels is which of those is which, not how any one of
 * them is drawn.
 */
function Figure({
  rank,
  color,
  accent,
  stock,
  waist,
}: {
  rank: string;
  color: string;
  accent: string;
  stock: string;
  waist: number;
}) {
  // Filled with the card's stock and outlined in its ink, and drawn *after*
  // whatever sits behind it — see `stock` above.
  //
  // With features, which is not decoration either: an outlined circle with
  // nothing in it does not read as a face, it reads as a hole punched in the
  // figure. Two eyes and a mouth are the fewest marks that fix that. They are
  // well under a pixel on a phone, where they blur into a grey suggestion of
  // features — which is still a face, and still better than a hole.
  const face = (
    <G>
      <SvgCircle cx="50" cy="44" r="13" fill={stock} stroke={color} strokeWidth="2.6" />
      <SvgCircle cx="45" cy="42" r="1.8" fill={color} />
      <SvgCircle cx="55" cy="42" r="1.8" fill={color} />
      <Path d="M45 50 C47 52 53 52 55 50" stroke={color} strokeWidth="1.6" fill="none" />
    </G>
  );
  const skirt = SKIRT[rank];
  // The robe, carried from the shoulders down to the waist wherever the panel
  // is taller than a figure. Drawn first, so the shoulders' own curve lies
  // over its top edge and the two read as one shape rather than a body with a
  // seam across it.
  const robe =
    skirt && waist > HALF - 1 ? (
      <G>
        <Rect x={skirt.x} y={HALF - 2} width={skirt.width} height={waist - (HALF - 2)} fill={color} />
        {/* A placket down the middle of it. Without this the robe is a
            rectangle of solid ink, and the bottom third of every court card
            is a black slab — which is what the figure looked like before the
            robe was clothing. */}
        <Rect x={49} y={HALF - 2} width={2} height={waist - (HALF - 2)} fill={stock} opacity={0.5} />
      </G>
    ) : null;

  switch (rank) {
    case 'K':
      return (
        <G>
          {robe}
          <Path d="M20 87 C23 71 34 64 50 64 C66 64 77 71 80 87 Z" fill={color} />
          {/* The beard, behind the face and emerging below it — the whole of
              how a king reads as a king and not a queen once the crown is
              four pixels tall. */}
          <Path d="M39 46 C39 64 43 70 50 70 C57 70 61 64 61 46 Z" fill={color} />
          {face}
          {/* The collar, in the stock: a king whose beard, shoulders and robe
              are all one colour is one silhouette, and this is the line that
              tells them apart. */}
          <Path d="M38 68 L50 78 L62 68" stroke={stock} strokeWidth="2.2" fill="none" />
          <Rect x="27" y="27" width="46" height="6" fill={color} />
          {/* Three points and a band — the one crown anybody draws from memory. */}
          <Path d="M28 28 L31 5 L41 18 L50 2 L59 18 L69 5 L72 28 Z" fill={accent} />
        </G>
      );
    case 'Q':
      return (
        <G>
          {robe}
          <Path d="M22 87 C26 69 36 63 50 63 C64 63 74 69 78 87 Z" fill={color} />
          <Path d="M40 66 L50 76 L60 66" stroke={stock} strokeWidth="2.2" fill="none" />
          {/* Hair as one shape behind the face, which the face then punches a
              clear disc out of: a frame, rather than two curls stuck to the
              sides of a head. */}
          <Path d="M31 44 C31 26 69 26 69 44 C69 62 62 68 50 68 C38 68 31 62 31 44 Z" fill={color} />
          {face}
          <Rect x="32" y="27" width="36" height="5" fill={color} />
          {/* A lower, rounder tiara — the silhouette difference that survives
              being small, where a detailed crown does not. */}
          <Path d="M33 28 C33 7 45 14 50 3 C55 14 67 7 67 28 Z" fill={accent} />
        </G>
      );
    case 'J':
      return (
        <G>
          {robe}
          {/* A ruff rather than a robe — a jack's collar, scalloped. */}
          <Path d="M26 87 C30 68 37 62 50 62 C63 62 70 68 74 87 Z" fill={color} />
          <Path d="M34 70 A5 5 0 0 0 44 70 A5 5 0 0 0 54 70 A5 5 0 0 0 64 70" fill="none" stroke={stock} strokeWidth="2" />
          {face}
          <Rect x="31" y="29" width="38" height="5" fill={color} />
          {/* A soft cap with a feather: no points at all, which is what tells
              a jack from the two crowned ranks at a distance. */}
          <Path d="M32 30 C32 11 68 11 68 30 Z" fill={accent} />
          <Path d="M66 23 C78 16 83 6 80 1 C74 9 69 15 62 19 Z" fill={color} />
        </G>
      );
    case 'JKR':
      return (
        <G>
          {robe}
          {/* The ruff a jester wears, pointed rather than scalloped. */}
          <Path d="M28 87 L36 70 L43 81 L50 68 L57 81 L64 70 L72 87 Z" fill={color} />
          {face}
          <Rect x="26" y="31" width="48" height="5" fill={color} />
          {/* Three drooping points, a bell on each. A jester's cap is the one
              headdress that is unmistakable in silhouette, which is the whole
              job at this size. */}
          <Path d="M50 9 C34 9 27 20 27 32 L73 32 C73 20 66 9 50 9 Z" fill={accent} />
          <Path d="M29 29 C20 26 14 18 13 10" stroke={accent} strokeWidth="4" fill="none" />
          <Path d="M71 29 C80 26 86 18 87 10" stroke={accent} strokeWidth="4" fill="none" />
          <SvgCircle cx="12" cy="9" r="5" fill={color} />
          <SvgCircle cx="88" cy="9" r="5" fill={color} />
          <SvgCircle cx="50" cy="5" r="5" fill={color} />
        </G>
      );
    default:
      return null;
  }
}

export function Court({ rank, width, height, color, accent, stock }: Props) {
  if (!FIGURES.includes(rank) || width <= 0 || height <= 0) return null;
  const trim = accent ?? color;
  // The panel's own shape, in design units: 100 across and however many that
  // makes it tall. This is what stops the figure being letterboxed — see the
  // file comment.
  const span = Math.round((height / width) * 100);
  const waist = span / 2;
  return (
    <Svg width={width} height={height} viewBox={`0 0 100 ${span}`}>
      {/* The panel the figure sits in, and the waist it is mirrored about —
          both hairlines, both the figure's own colour at low opacity, so the
          frame reads as printing on the card rather than as a border on a
          box. */}
      <Rect
        x="3"
        y="3"
        width="94"
        height={Math.max(0, span - 6)}
        rx="4"
        fill="none"
        stroke={trim}
        strokeWidth="1.6"
        opacity={0.55}
      />
      <Path d={`M3 ${waist} L97 ${waist}`} stroke={color} strokeWidth="1" opacity={0.3} />
      <Figure rank={rank} color={color} accent={trim} stock={stock} waist={waist} />
      {/* The same figure, turned over. */}
      <G transform={`rotate(180 50 ${waist})`}>
        <Figure rank={rank} color={color} accent={trim} stock={stock} waist={waist} />
      </G>
    </Svg>
  );
}
