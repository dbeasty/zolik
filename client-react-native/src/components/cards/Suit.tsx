import Svg, { Circle, G, Path } from 'react-native-svg';

/**
 * A suit, drawn rather than typed.
 *
 * Every card on the board until now spelled its suit with the Unicode
 * character — `♠`, `♥` — which works, costs nothing, and has one flaw that
 * only shows at size: the shape is whatever font the platform reached for.
 * iOS, Android and three browsers each draw a different spade, none of them
 * the one on a printed card, and all of them go soft the moment a card is
 * drawn bigger than the 52px it was designed at.
 *
 * These are paths in a 100×100 box, so one shape serves a 9px corner index
 * and a 60px centre pip on a desktop, identically and crisply, the same way
 * `Avatar` already draws a face. Vector for the same three reasons, the last
 * of which is the point here: a card that scales up is the whole large-screen
 * half of this look.
 */

type Props = {
  /** 'H' | 'D' | 'C' | 'S'; anything else draws nothing. */
  suit: string;
  size: number;
  color: string;
  /** Turned over, for a pip below the waist — see `src/lib/pips.ts`. */
  flipped?: boolean;
};

/**
 * The three suits that are one closed shape. Clubs are three circles and a
 * stem, which is a shape but not a *path* worth writing by hand, so they are
 * assembled below instead.
 *
 * Hearts and spades are the same silhouette turned over — a heart is a spade
 * upside down with the stem taken off — which is not a coincidence and is
 * why their curves match here.
 */
const PATHS: Record<string, string> = {
  H: 'M50 91 C50 91 7 60 7 34 C7 18 19 8 32 8 C41 8 47 13 50 20 C53 13 59 8 68 8 C81 8 93 18 93 34 C93 60 50 91 50 91 Z',
  D: 'M50 5 C50 5 66 30 90 50 C66 70 50 95 50 95 C50 95 34 70 10 50 C34 30 50 5 50 5 Z',
  S: 'M50 7 C50 7 7 41 7 63 C7 75 16 83 27 83 C35 83 41 79 45 73 C45 79 42 88 33 94 L67 94 C58 88 55 79 55 73 C59 79 65 83 73 83 C84 83 93 75 93 63 C93 41 50 7 50 7 Z',
};

/** The club's stem, drawn under its three lobes. */
const CLUB_STEM = 'M43 62 C45 76 42 87 33 94 L67 94 C58 87 55 76 57 62 Z';

export function Suit({ suit, size, color, flipped }: Props) {
  if (!PATHS[suit] && suit !== 'C') return null;
  return (
    <Svg width={size} height={size} viewBox="0 0 100 100">
      {/* Rotated about the centre of the box, so a flipped pip occupies
          exactly the box an upright one does — the same discipline every
          highlight on this board keeps, one level down. */}
      <G transform={flipped ? 'rotate(180 50 50)' : undefined}>
        {suit === 'C' ? (
          <>
            <Path d={CLUB_STEM} fill={color} />
            <Circle cx="50" cy="26" r="21" fill={color} />
            <Circle cx="26" cy="58" r="21" fill={color} />
            <Circle cx="74" cy="58" r="21" fill={color} />
          </>
        ) : (
          <Path d={PATHS[suit]} fill={color} />
        )}
      </G>
    </Svg>
  );
}
