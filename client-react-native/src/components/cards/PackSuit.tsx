import Svg, { Path } from 'react-native-svg';

import { GERMAN_PACK, suitLine, type Shape, type SuitPack } from '@/src/components/cards/germanArt';

/**
 * A pictorial pack's suit sign.
 *
 * The French suits are one colour each and one closed shape (`Suit`). The
 * German and Spanish ones are pictures: a bell has a slot and a band, an acorn
 * a cup on a nut, a cup a stem and a foot, a coin a struck rosette. They are
 * drawn in their own traditional colours rather than the French two, because
 * four colours is how a hand from either pack is read at a glance, and the only
 * thing a player fanning ten cards has time for.
 *
 * The heart — and the Spanish cup, which is the same code — takes the skin's
 * red, so it matches the rest of the board under every skin. The other three
 * are the pack's own colours and no skin changes them: a skin may retint the
 * table, not the cards' suits (see `src/skins/types.ts`).
 *
 * Which pack is drawn comes in as `pack` and defaults to the German one, so a
 * caller that has never heard of the Spanish forty is unchanged. The drawings
 * themselves are data in `germanArt.ts` and `spanishArt.ts`; this file only
 * puts them on screen.
 */

/** The colour a pack's index is printed in: dark enough to read on stock. */
export function packInk(suit: string, red: string, pack: SuitPack = GERMAN_PACK): string {
  return suitLine(suit, red, pack);
}

/** A list of shapes from a pack's art, as paths inside whatever `Svg` holds them. */
export function PackShapes({ shapes }: { shapes: readonly Shape[] }) {
  return (
    <>
      {shapes.map((s, i) => (
        <Path
          key={i}
          d={s.d}
          fill={s.fill ?? 'none'}
          stroke={s.stroke}
          strokeWidth={s.sw}
          strokeLinecap="round"
          strokeLinejoin="round"
          opacity={s.opacity}
          transform={s.transform}
        />
      ))}
    </>
  );
}

type Props = { suit: string; size: number; red: string; pack?: SuitPack };

export function PackSuit({ suit, size, red, pack = GERMAN_PACK }: Props) {
  if (!(pack.suits as readonly string[]).includes(suit)) return null;
  return (
    <Svg width={size} height={size} viewBox="0 0 100 100">
      <PackShapes shapes={pack.sign(suit, red)} />
    </Svg>
  );
}
