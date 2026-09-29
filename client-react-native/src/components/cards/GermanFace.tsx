import { StyleSheet, Text, View } from 'react-native';
import Svg, { Rect } from 'react-native-svg';

import { GermanCourt } from '@/src/components/cards/GermanCourt';
import { GermanSuit, germanInk } from '@/src/components/cards/GermanSuit';
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

export function GermanFace({ card, width, height, ink, red, stock }: Props) {
  const suit = card.suit;
  const color = germanInk(suit, red);
  // The deluxe face's proportions, so a German card and a French one are
  // printed at the same weight and the pips clear the index the same way.
  const indexRank = Math.max(7, Math.round(width * 0.24));
  const indexSuit = Math.max(6, Math.round(width * 0.17));
  const pipSize = Math.max(5, Math.round(width * 0.19));
  const acePip = Math.round(width * 0.5);
  const inset = Math.max(2, Math.round(width * 0.045));

  const fieldWidth = width * (FIELD.right - FIELD.left);
  const fieldHeight = height * (FIELD.bottom - FIELD.top);
  const field = { left: width * FIELD.left, top: height * FIELD.top, width: fieldWidth, height: fieldHeight };
  const pips = pipsFor(card.rank);
  const label = germanIndex(card.rank);

  const index = (
    <>
      <Text
        style={[
          styles.indexRank,
          {
            // Two letters (a Czech "Sv") set a size down, so they fit the
            // column a single letter does — "10" included, which is the widest
            // index and the one the top-left pip sits nearest.
            fontSize: label.length > 1 ? Math.round(indexRank * (card.rank === '10' ? 0.72 : 0.8)) : indexRank,
            lineHeight: Math.round(indexRank * 1.02),
            color,
          },
        ]}
      >
        {label}
      </Text>
      <GermanSuit suit={suit} size={indexSuit} red={red} />
    </>
  );

  return (
    <View style={StyleSheet.absoluteFill} pointerEvents="none">
      {isCourt(card.rank) ? (
        <View style={[styles.field, field]}>
          <GermanCourt
            rank={card.rank}
            suit={suit}
            width={fieldWidth}
            height={fieldHeight}
            ink={ink}
            red={red}
            stock={stock}
          />
        </View>
      ) : pips.length ? (
        <View style={[styles.field, field]}>
          {pips.map((p, i) => (
            <View
              key={`${p.x}-${p.y}-${i}`}
              style={{
                position: 'absolute',
                left: p.x * fieldWidth - pipSize / 2,
                top: p.y * fieldHeight - pipSize / 2,
              }}
            >
              <GermanSuit suit={suit} size={pipSize} red={red} flipped={p.flip} />
            </View>
          ))}
        </View>
      ) : (
        // The eso: one large sign in a printed frame, the way a mariášky ace
        // is a panel rather than a pip.
        <View style={styles.centre}>
          <View style={[styles.field, field]}>
            <Svg width={fieldWidth} height={fieldHeight}>
              <Rect
                x={2}
                y={2}
                width={Math.max(0, fieldWidth - 4)}
                height={Math.max(0, fieldHeight - 4)}
                rx={4}
                fill="none"
                stroke={color}
                strokeWidth={1.4}
                opacity={0.55}
              />
            </Svg>
          </View>
          <GermanSuit suit={suit} size={acePip} red={red} />
        </View>
      )}
      <View style={[styles.indexTL, { left: inset, top: inset }]}>{index}</View>
      <View style={[styles.indexBR, { right: inset, bottom: inset }]}>{index}</View>
    </View>
  );
}

const styles = StyleSheet.create({
  field: { position: 'absolute' },
  centre: {
    position: 'absolute',
    top: 0,
    bottom: 0,
    left: 0,
    right: 0,
    alignItems: 'center',
    justifyContent: 'center',
  },
  indexTL: { position: 'absolute', alignItems: 'center' },
  indexBR: { position: 'absolute', alignItems: 'center', transform: [{ rotate: '180deg' }] },
  indexRank: { fontWeight: '700', textAlign: 'center' },
});
