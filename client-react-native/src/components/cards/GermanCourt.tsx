import Svg, { G, Path, Circle as SvgCircle, Rect } from 'react-native-svg';

import { GERMAN_SUIT_COLORS, GermanSuitGlyph } from '@/src/components/cards/GermanSuit';

/**
 * The figure on a German spodek, svršek or král.
 *
 * Built exactly as `Court` builds the French figures — a half-figure in the
 * top of the panel and the same half turned over below it, drawn for the
 * panel's real proportions — because Czech mariášky are double-ended too,
 * and that file already explains why each of those choices is the one that
 * survives being 40 pixels across. What differs is who is standing there.
 *
 * The German pack tells its two lower courts apart by *where the suit sign
 * is*, not by what they wear: the svršek (Ober) holds his sign up by his
 * head, the spodek (Unter) holds his low at his waist. That rule is the
 * heart of this drawing — it is what a Czech player looks for — and the
 * headgear only backs it up: a hunter's feathered cap on the spodek, a
 * brimmed hat on the svršek, a crown on the král, who also keeps his sign up
 * beside it.
 *
 * Original, stylised figures in the manner of the Tell pattern, not traced
 * from any printed pack (docs/marias-plan.md §3.4).
 */

type Props = {
  /** 'J' (spodek) | 'Q' (svršek) | 'K' (král). Anything else draws nothing. */
  rank: string;
  suit: string;
  width: number;
  height: number;
  /** The figure's line and fill — the card's ink. */
  ink: string;
  /** The heart's red, from the skin. */
  red: string;
  /** The card's own stock, which punches the features out of the figure. */
  stock: string;
};

const FIGURES = ['J', 'Q', 'K'];

/** How tall a half is in design units for a panel 100 wide — see `Court`. */
const HALF = 87;

/** Where the shoulders meet the bottom of the half, per figure. */
const SKIRT: Record<string, { x: number; width: number }> = {
  K: { x: 20, width: 60 },
  Q: { x: 24, width: 52 },
  J: { x: 26, width: 48 },
};

function Figure({
  rank,
  suit,
  ink,
  red,
  stock,
  waist,
}: {
  rank: string;
  suit: string;
  ink: string;
  red: string;
  stock: string;
  waist: number;
}) {
  // The suit's own colour dresses the figure — coat, shoulders, the band of
  // a hat — outlined in ink, as a Tell-pattern court is printed in colour
  // rather than in silhouette. Hearts take the skin's red, like the sign.
  const trim = suit === 'H' ? red : GERMAN_SUIT_COLORS[suit]?.fill ?? ink;
  const face = (
    <G>
      <SvgCircle cx="50" cy="44" r="13" fill={stock} stroke={ink} strokeWidth="2.6" />
      <SvgCircle cx="45" cy="42" r="1.8" fill={ink} />
      <SvgCircle cx="55" cy="42" r="1.8" fill={ink} />
      <Path d="M45 50 C47 52 53 52 55 50" stroke={ink} strokeWidth="1.6" fill="none" />
    </G>
  );
  const skirt = SKIRT[rank];
  const robe =
    skirt && waist > HALF - 1 ? (
      <G>
        <Rect x={skirt.x} y={HALF - 2} width={skirt.width} height={waist - (HALF - 2)} fill={trim} stroke={ink} strokeWidth={2} />
        <Rect x={49} y={HALF - 2} width={2} height={waist - (HALF - 2)} fill={ink} opacity={0.6} />
      </G>
    ) : null;

  switch (rank) {
    case 'K':
      return (
        <G>
          {robe}
          <Path d="M20 87 C23 71 34 64 50 64 C66 64 77 71 80 87 Z" fill={trim} stroke={ink} strokeWidth="2" />
          {/* A fur collar, in the stock, over the shoulders. */}
          <Path d="M30 72 C38 66 62 66 70 72 C62 76 38 76 30 72 Z" fill={stock} opacity={0.85} />
          <Path d="M39 46 C39 64 43 70 50 70 C57 70 61 64 61 46 Z" fill={ink} />
          {face}
          <Rect x="30" y="27" width="40" height="6" fill={ink} />
          <Path d="M31 28 L33 8 L42 19 L50 5 L58 19 L67 8 L69 28 Z" fill="#E0B232" stroke={ink} strokeWidth="1.5" />
          {/* His sign, up beside the crown — on the right, clear of the
              corner index, which owns the top-left of every card. */}
          <GermanSuitGlyph suit={suit} red={red} x={74} y={6} size={22} />
        </G>
      );
    case 'Q':
      return (
        <G>
          {robe}
          <Path d="M24 87 C27 70 37 63 50 63 C63 63 73 70 76 87 Z" fill={trim} stroke={ink} strokeWidth="2" />
          {/* A doublet's trim down the front. */}
          <Path d="M50 64 L50 87" stroke={ink} strokeWidth="2.5" />
          {face}
          {/* A brimmed hat with a plume: the svršek's. */}
          <Rect x="22" y="27" width="56" height="5" rx="2" fill={ink} />
          <Path d="M34 28 C34 12 66 12 66 28 Z" fill={ink} />
          <Rect x="34" y="22" width="32" height="4" fill={trim} />
          <Path d="M38 16 C26 8 20 0 22 -4 C28 4 34 9 42 13 Z" fill={trim} />
          {/* The svršek holds his sign high, by his head. */}
          <GermanSuitGlyph suit={suit} red={red} x={74} y={4} size={22} />
        </G>
      );
    case 'J':
      return (
        <G>
          {robe}
          <Path d="M26 87 C30 68 37 62 50 62 C63 62 70 68 74 87 Z" fill={trim} stroke={ink} strokeWidth="2" />
          <Path d="M34 70 A5 5 0 0 0 44 70 A5 5 0 0 0 54 70 A5 5 0 0 0 64 70" fill="none" stroke={ink} strokeWidth="2" />
          {face}
          {/* A soft hunter's cap with a feather: no brim, no points. */}
          <Rect x="32" y="29" width="36" height="5" fill={ink} />
          <Path d="M33 30 C33 12 67 12 67 30 Z" fill={ink} />
          <Path d="M36 22 C24 15 19 5 22 0 C28 8 33 14 40 18 Z" fill={trim} stroke={ink} strokeWidth="1.2" />
          {/* The spodek holds his sign low, at his waist. */}
          <GermanSuitGlyph suit={suit} red={red} x={74} y={HALF - 26} size={22} />
        </G>
      );
    default:
      return null;
  }
}

export function GermanCourt({ rank, suit, width, height, ink, red, stock }: Props) {
  if (!FIGURES.includes(rank) || width <= 0 || height <= 0) return null;
  const span = Math.round((height / width) * 100);
  const waist = span / 2;
  const trim = suit === 'H' ? red : GERMAN_SUIT_COLORS[suit]?.line ?? ink;
  return (
    <Svg width={width} height={height} viewBox={`0 0 100 ${span}`}>
      <Rect x="3" y="3" width="94" height={Math.max(0, span - 6)} rx="4" fill="none" stroke={trim} strokeWidth="1.6" opacity={0.55} />
      <Path d={`M3 ${waist} L97 ${waist}`} stroke={ink} strokeWidth="1" opacity={0.3} />
      <Figure rank={rank} suit={suit} ink={ink} red={red} stock={stock} waist={waist} />
      <G transform={`rotate(180 50 ${waist})`}>
        <Figure rank={rank} suit={suit} ink={ink} red={red} stock={stock} waist={waist} />
      </G>
    </Svg>
  );
}
