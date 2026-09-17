import { useMemo } from 'react';
import { StyleSheet, View } from 'react-native';
import Svg, { G, Path } from 'react-native-svg';

import { FACES, type Face, type FaceColor, type FaceNode } from '@/src/cards/vector/faces';
import type { CardDisplay } from '@/src/lib/cards';
import type { CardPalette } from '@/src/skins/types';

/**
 * A real printed card, drawn from a real printed deck.
 *
 * Everything on this face — the courts, the pips, the corner indices, the
 * joker — is Chris Aguilar's Vectorized Playing Cards, engraved from a deck
 * that has looked like this since the 1880s. The deck is LGPL 3.0 and is
 * vendored under `src/cards/vector/`, which is also where its licence and
 * the mandatory attribution live. `scripts/gen-card-faces.js` turns the SVGs
 * into the data this component walks.
 *
 * This replaced a face drawn by hand (`DeluxeFace`), and the reason is worth
 * writing down, because it is not "the old one was ugly". The old face drew
 * a court figure from primitives — a circle for a head, two dots for eyes, a
 * polygon crown — and the honest ceiling on that approach is a figure that
 * reads as *a drawing of* a king. Below about 60 pixels it reads as a smudge
 * with a hat. The engraved deck has the opposite property: it was cut for
 * print at 63mm and survives being shrunk, because the line weights were
 * chosen by somebody solving that exact problem a century before us.
 *
 * Three things this face does *not* own, all of them deliberate:
 *
 *  - **The stock.** The art's own white rounded rect is stripped at
 *    generation time. `CardView` draws the card — its gradient wash, its
 *    selection fill, the joker's own tint — and this face is drawn over it.
 *    A face that painted its own background would flatten all three.
 *  - **The size.** Width and height come in from `CardView`, like every
 *    other face. A skin cannot change the box (see `src/skins/types.ts`).
 *  - **The colours,** except by name. The deck is drawn in five flat inks
 *    and the skin says what each one is, which is how the same engraving
 *    serves an ivory-and-gold Heirloom table and the deck's own red-and-navy
 *    without a second copy of the art.
 */

type Props = {
  card: CardDisplay;
  /** The card's own box, border already subtracted — what `CardView` laid out. */
  width: number;
  height: number;
  /** What the deck's five inks are on this skin. */
  palette: CardPalette;
};

/**
 * Which face a card wears.
 *
 * The deck is keyed exactly the way `parseCard` names a card, so this is a
 * lookup rather than a mapping — but not an assertion: a card code the deck
 * has no face for draws nothing and lets `CardView` fall through, which is a
 * blank card rather than a crash on a table somebody is playing at.
 */
export function faceFor(card: CardDisplay): Face | undefined {
  return FACES[card.isJoker ? 'JKR' : `${card.rank}${card.suit}`];
}

function draw(nodes: readonly FaceNode[], palette: CardPalette, prefix: string) {
  return nodes.map((node, i) => {
    const key = `${prefix}${i}`;
    if ('d' in node) {
      // A path can carry its own transform as well as inherit one — svgo moves
      // a group's matrix onto its children when that is shorter to write, so
      // both spellings occur in the same deck.
      return (
        <Path
          key={key}
          d={node.d}
          transform={node.t || undefined}
          fill={palette[node.c as FaceColor]}
        />
      );
    }
    return (
      <G key={key} transform={node.g || undefined}>
        {draw(node.k, palette, `${key}.`)}
      </G>
    );
  });
}

export function VectorFace({ card, width, height, palette }: Props) {
  const face = faceFor(card);
  // Rebuilt only when the card or the skin's inks change — a hand of
  // seventeen cards re-renders on every drag, and the courts carry single
  // paths of forty thousand characters.
  const art = useMemo(
    () => (face ? draw(face.nodes, palette, '') : null),
    [face, palette],
  );
  if (!face) return null;
  return (
    // Absolutely filled, like every other face, and for a reason that cost an
    // afternoon: `CardView` paints the skin's gradient wash as its own
    // absolutely-positioned layer *before* the face. A face laid out in the
    // normal flow paints underneath that layer no matter what order it is
    // written in — positioned elements win over in-flow siblings — so the
    // card came out blank. Taking the face out of the flow also keeps it clear
    // of the card's padding, which would otherwise shrink the art.
    <View style={StyleSheet.absoluteFill} pointerEvents="none">
      {/* `preserveAspectRatio="none"`: a card here is 52×72 and the art is
          drawn at 63×88, the same shape to within eight tenths of a percent.
          Letterboxing that difference would inset the art on one axis and
          leave the corner indices at different distances from the two edges.
          Stretching it by 0.8% is invisible, and the card fills its box. */}
      <Svg width={width} height={height} viewBox={face.viewBox} preserveAspectRatio="none">
        {art}
      </Svg>
    </View>
  );
}
