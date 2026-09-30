import Svg, { Circle, G, Path, Rect } from 'react-native-svg';

/**
 * The four German suits — hearts, bells, acorns and leaves — drawn as paths.
 *
 * The French suits are one colour each and one closed shape (`Suit`). The
 * German ones are pictures: a bell has a slot and a band, an acorn a cap on a
 * nut, a leaf a vein. They are drawn in their own traditional colours — red,
 * gold, brown and green — rather than the French two, because four colours is
 * how a German-suited hand is read at a glance, and the only thing a player
 * fanning ten cards has time for.
 *
 * The heart takes the skin's red, so it matches the rest of the board under
 * every skin. The other three are the pack's own colours and no skin changes
 * them: a skin may retint the table, not the cards' suits (see
 * `src/skins/types.ts`).
 *
 * Designed in a 100×100 box. `GermanSuitGlyph` is the shape as an SVG group,
 * for placing inside another drawing (a court figure holds one); `GermanSuit`
 * wraps it in its own `Svg` for use on its own.
 */

/** The pack's own colours: a main fill and a darker line for detail. */
export const GERMAN_SUIT_COLORS: Record<string, { fill: string; line: string }> = {
  D: { fill: '#E0A526', line: '#7A4E00' },
  C: { fill: '#A86F3A', line: '#4F2E12' },
  S: { fill: '#3E9A34', line: '#1E5E18' },
};

/** The colour a German suit's index is printed in: dark enough to read on stock. */
export function germanInk(suit: string, red: string): string {
  return suit === 'H' ? red : GERMAN_SUIT_COLORS[suit]?.line ?? red;
}

const HEART =
  'M50 91 C50 91 7 60 7 34 C7 18 19 8 32 8 C41 8 47 13 50 20 C53 13 59 8 68 8 C81 8 93 18 93 34 C93 60 50 91 50 91 Z';

type GlyphProps = {
  /** 'H' | 'D' | 'C' | 'S'; anything else draws nothing. */
  suit: string;
  /** The heart's colour — the skin's red. */
  red: string;
  /** Where, and how big, inside the parent drawing's own units. */
  x?: number;
  y?: number;
  size?: number;
  /** Turned over, for a pip below the waist. */
  flipped?: boolean;
};

export function GermanSuitGlyph({ suit, red, x = 0, y = 0, size = 100, flipped }: GlyphProps) {
  const k = size / 100;
  const turn = flipped ? ' rotate(180 50 50)' : '';
  const c = GERMAN_SUIT_COLORS[suit];
  let shape = null;
  switch (suit) {
    case 'H':
      shape = <Path d={HEART} fill={red} />;
      break;
    case 'D':
      // A round sleigh bell: the loop it hangs from, the body, the band
      // round its middle and the slot across its foot.
      shape = (
        <G>
          <Circle cx="50" cy="14" r="8" fill="none" stroke={c.line} strokeWidth="5" />
          <Circle cx="50" cy="56" r="37" fill={c.fill} stroke={c.line} strokeWidth="3" />
          <Path d="M14 48 Q50 38 86 48 L86 58 Q50 48 14 58 Z" fill={c.line} />
          <Path d="M32 80 Q50 70 68 80" stroke={c.line} strokeWidth="5" fill="none" />
          <Circle cx="50" cy="84" r="4.5" fill={c.line} />
        </G>
      );
      break;
    case 'C':
      // An acorn: a stalk, a scaled cap, and the nut below it.
      shape = (
        <G>
          <Rect x="46" y="4" width="8" height="18" rx="3" fill={c.line} />
          <Path d="M29 48 C29 80 40 96 50 96 C60 96 71 80 71 48 Z" fill={c.fill} stroke={c.line} strokeWidth="3" />
          <Path d="M20 52 C20 30 33 20 50 20 C67 20 80 30 80 52 Z" fill={c.line} />
          <Path d="M28 42 L72 42 M32 33 L68 33" stroke={c.fill} strokeWidth="2.5" opacity={0.7} />
        </G>
      );
      break;
    case 'S':
      // A leaf, point up, with its midrib running on into the stalk.
      shape = (
        <G>
          <Path d="M50 3 C80 20 92 54 50 88 C8 54 20 20 50 3 Z" fill={c.fill} stroke={c.line} strokeWidth="3" />
          <Path d="M50 12 L50 97" stroke={c.line} strokeWidth="4.5" />
          <Path d="M50 38 L67 26 M50 38 L33 26 M50 58 L71 44 M50 58 L29 44" stroke={c.line} strokeWidth="3" />
        </G>
      );
      break;
    default:
      return null;
  }
  return <G transform={`translate(${x} ${y}) scale(${k})${turn}`}>{shape}</G>;
}

type Props = { suit: string; size: number; red: string; flipped?: boolean };

export function GermanSuit({ suit, size, red, flipped }: Props) {
  if (!['H', 'D', 'C', 'S'].includes(suit)) return null;
  return (
    <Svg width={size} height={size} viewBox="0 0 100 100">
      <GermanSuitGlyph suit={suit} red={red} flipped={flipped} />
    </Svg>
  );
}
