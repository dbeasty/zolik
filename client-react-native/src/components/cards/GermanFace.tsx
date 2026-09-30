import { StyleSheet, Text, View } from 'react-native';
import Svg from 'react-native-svg';

import { GermanShapes, GermanSuit, germanInk } from '@/src/components/cards/GermanSuit';
import { aceShapes, courtPanel, placedSuit, type Notch, type Shape } from '@/src/components/cards/germanArt';
import type { CardDisplay } from '@/src/lib/cards';
import { germanIndex } from '@/src/lib/deck';
import { isCourt, pipsFor } from '@/src/lib/pips';

/**
 * A German-suited card face: `DeluxeFace`'s layout, with the German pack's
 * suits, courts and indices.
 *
 * Everything that makes the deluxe face read as a printed card carries over
 * unchanged — indices in both corners with the bottom one turned, the pip
 * arrangements in `src/lib/pips.ts` for the 7 to the 10, one large sign on
 * the ace, a double-ended figure on the courts — so a Mariáš hand sits on the
 * board at the same size and in the same grammar as a rummy hand. Every
 * measurement is a fraction of the card handed in; `CardView` owns the box,
 * and draws the plain and stacked German faces itself.
 *
 * The middle of the card is one drawing, 100 units wide, made of the shapes
 * in `germanArt.ts`; the indices are type laid over it, because which letter
 * a svršek wears is the player's language's business.
 */

type Props = {
  card: CardDisplay;
  width: number;
  height: number;
  ink: string;
  red: string;
  stock: string;
};

const FIELD = { left: 0.15, right: 0.85, top: 0.06, bottom: 0.94 };

/** How wide a pip is, in the field's 100 units. */
const PIP = 27;

/**
 * How far the two pip columns are drawn in towards the spine. A German sign
 * is a picture with an outline, wider for its height than a French pip, and
 * at the French spacing the top-left one touches the index beside it.
 */
const COLUMNS = 0.84;

export function GermanFace({ card, width, height, ink, red, stock }: Props) {
  const suit = card.suit;
  const color = germanInk(suit, red);
  // The deluxe face's proportions, so a German card and a French one are
  // printed at the same weight and the pips clear the index the same way.
  const indexRank = Math.max(7, Math.round(width * 0.24));
  const indexSuit = Math.max(6, Math.round(width * 0.17));
  const inset = Math.max(2, Math.round(width * 0.045));

  const fieldWidth = width * (FIELD.right - FIELD.left);
  const fieldHeight = height * (FIELD.bottom - FIELD.top);
  const field = { left: width * FIELD.left, top: height * FIELD.top, width: fieldWidth, height: fieldHeight };
  const label = germanIndex(card.rank);
  // Two letters (a Czech "Sv") set a size down, so they fit the column a
  // single letter does — "10" included, which is the widest index and the one
  // the top-left pip sits nearest.
  const fontSize = label.length > 1 ? Math.round(indexRank * (card.rank === '10' ? 0.72 : 0.8)) : indexRank;
  const lineHeight = Math.round(indexRank * 1.02);

  const span = fieldWidth > 0 ? Math.round((fieldHeight / fieldWidth) * 100) : 0;
  let shapes: Shape[] = [];
  if (span > 0) {
    const pips = pipsFor(card.rank);
    if (pips.length) {
      shapes = pips.flatMap((p) => placedSuit(suit, red, 50 + (p.x - 0.5) * COLUMNS * 100 - PIP / 2, p.y * span - PIP / 2, PIP, p.flip));
    } else {
      // A court's frame and an ace's step in around the index. How far is
      // measured from the index actually printed: bold capitals run to about
      // two thirds of their size across.
      const unit = 100 / fieldWidth;
      const gap = width * 0.02;
      const column = Math.max(indexSuit, label.length * fontSize * 0.65);
      const notch: Notch = {
        w: Math.max(6, (inset + column + gap - field.left) * unit),
        h: Math.max(6, (inset + lineHeight + indexSuit + gap - field.top) * unit),
      };
      shapes = isCourt(card.rank)
        ? courtPanel(card.rank, suit, span, notch, { ink, red, stock })
        : aceShapes(suit, red, span, notch);
    }
  }

  const index = (
    <>
      <Text style={[styles.indexRank, { fontSize, lineHeight, color }]}>{label}</Text>
      <GermanSuit suit={suit} size={indexSuit} red={red} />
    </>
  );

  return (
    <View style={StyleSheet.absoluteFill} pointerEvents="none">
      {span > 0 ? (
        <View style={[styles.field, field]}>
          <Svg width={fieldWidth} height={fieldHeight} viewBox={`0 0 100 ${span}`}>
            <GermanShapes shapes={shapes} />
          </Svg>
        </View>
      ) : null}
      <View style={[styles.indexTL, { left: inset, top: inset }]}>{index}</View>
      <View style={[styles.indexBR, { right: inset, bottom: inset }]}>{index}</View>
    </View>
  );
}

const styles = StyleSheet.create({
  field: { position: 'absolute' },
  indexTL: { position: 'absolute', alignItems: 'center' },
  indexBR: { position: 'absolute', alignItems: 'center', transform: [{ rotate: '180deg' }] },
  indexRank: { fontWeight: '700', textAlign: 'center' },
});
