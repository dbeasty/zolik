import Svg, { Path } from 'react-native-svg';

import { GERMAN_SUITS, suitLine, suitShapes, type Shape } from '@/src/components/cards/germanArt';

/**
 * The four German suits — hearts, bells, acorns and leaves.
 *
 * The French suits are one colour each and one closed shape (`Suit`). The
 * German ones are pictures: a bell has a slot and a band, an acorn a cup on a
 * nut, a leaf its veins. They are drawn in their own traditional colours —
 * red, gold, brown and green — rather than the French two, because four
 * colours is how a German-suited hand is read at a glance, and the only thing
 * a player fanning ten cards has time for.
 *
 * The heart takes the skin's red, so it matches the rest of the board under
 * every skin. The other three are the pack's own colours and no skin changes
 * them: a skin may retint the table, not the cards' suits (see
 * `src/skins/types.ts`).
 *
 * The drawings themselves are data in `germanArt.ts`; this file only puts
 * them on screen.
 */

/** The colour a German suit's index is printed in: dark enough to read on stock. */
export function germanInk(suit: string, red: string): string {
  return suitLine(suit, red);
}

/** A list of shapes from `germanArt.ts`, as paths inside whatever `Svg` holds them. */
export function GermanShapes({ shapes }: { shapes: readonly Shape[] }) {
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

type Props = { suit: string; size: number; red: string };

export function GermanSuit({ suit, size, red }: Props) {
  if (!(GERMAN_SUITS as readonly string[]).includes(suit)) return null;
  return (
    <Svg width={size} height={size} viewBox="0 0 100 100">
      <GermanShapes shapes={suitShapes(suit, red)} />
    </Svg>
  );
}
