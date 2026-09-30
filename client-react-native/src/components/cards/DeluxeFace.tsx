import { StyleSheet, Text, View } from 'react-native';

import { Court } from '@/src/components/cards/Court';
import { Suit } from '@/src/components/cards/Suit';
import type { CardDisplay } from '@/src/lib/cards';
import { isCourt, pipsFor } from '@/src/lib/pips';

/**
 * A card face drawn the way a card is printed.
 *
 * The board already had two faces: `plain` (a rank and one big pip) and
 * `rich` (two corner indices and a centre pip). Both say what the card is.
 * Neither looks like a card — a five of clubs with a single club in the
 * middle is a *label* for the five of clubs, and at the size a desktop screen
 * can now afford to draw one, the difference between a label and a card is
 * the whole of what there is to look at.
 *
 * So this face draws the three things a real one has and the others don't:
 *
 *  - **Indices in both corners**, rank over suit, the bottom one upside down.
 *  - **The proper pip arrangement** for the rank — the layouts in
 *    `src/lib/pips.ts`, including the seven's odd high pip.
 *  - **A mirrored figure** on the courts and the joker (`Court`), and one
 *    large centre pip on an ace.
 *
 * Every measurement is a fraction of the card it is handed, which is what
 * makes it the *large screen* half of this work as much as the pretty half:
 * the same face is right at 52 pixels across on a phone and at 90 on a
 * monitor, because nothing in it is a constant in pixels.
 *
 * Size in, drawing out — it never reads `useMetrics` itself. `CardView` owns
 * how big a card is and passes it down, so a face can never disagree with the
 * box it is drawn in, and a skin can never change that box (see
 * `src/skins/types.ts`).
 */

type Props = {
  card: CardDisplay;
  /** The card's own box, border included — what `CardView` laid out. */
  width: number;
  height: number;
  ink: string;
  red: string;
  /** Crowns, caps and collars. Falls back to the figure's own colour. */
  courtAccent?: string;
  /**
   * The colour this card's own face is filled with — which is not always the
   * skin's `cardBg`: a selected card and a joker each have their own. The
   * court figure needs it to punch its features back out of the silhouette
   * (see `Court`), so it has to be the fill this card actually has rather
   * than the one most cards have.
   */
  stock: string;
};

/**
 * Where the pips live inside the card: inset from the sides far enough to
 * clear the corner indices, and from the top and bottom by less — the same
 * proportions a printed card uses, give or take the millimetre nobody can
 * see at this size.
 */
const FIELD = { left: 0.15, right: 0.85, top: 0.06, bottom: 0.94 };

export function DeluxeFace({ card, width, height, ink, red, courtAccent, stock }: Props) {
  const color = card.isRed ? red : ink;
  const suit = card.suit;

  // Everything below is a fraction of the card, never a constant.
  const indexRank = Math.max(7, Math.round(width * 0.26));
  const indexSuit = Math.max(5, Math.round(width * 0.17));
  const pipSize = Math.max(4, Math.round(width * 0.19));
  const acePip = Math.round(width * 0.44);
  const inset = Math.max(2, Math.round(width * 0.045));

  const fieldLeft = width * FIELD.left;
  const fieldTop = height * FIELD.top;
  const fieldWidth = width * (FIELD.right - FIELD.left);
  const fieldHeight = height * (FIELD.bottom - FIELD.top);

  const pips = pipsFor(card.rank);
  const figure = isCourt(card.rank) || card.isJoker;

  const index = (
    <>
      {/* Rank over suit, tight together, the way an index is set. A joker
          has no suit to put under its rank, so it gets the rank alone. */}
      <Text
        style={[
          styles.indexRank,
          {
            fontSize: card.isJoker ? Math.round(indexRank * 0.62) : indexRank,
            lineHeight: Math.round(indexRank * 1.02),
            color,
          },
        ]}
      >
        {card.rank}
      </Text>
      {card.isJoker ? null : <Suit suit={suit} size={indexSuit} color={color} />}
    </>
  );

  return (
    <View style={StyleSheet.absoluteFill} pointerEvents="none">
      {figure ? (
        <View style={[styles.field, { left: fieldLeft, top: fieldTop, width: fieldWidth, height: fieldHeight }]}>
          <Court
            rank={card.isJoker ? 'JKR' : card.rank}
            width={fieldWidth}
            height={fieldHeight}
            color={card.isJoker ? red : color}
            accent={courtAccent}
            stock={stock}
          />
        </View>
      ) : pips.length ? (
        // The rank's own arrangement. Each pip is placed by its centre, which
        // is why every one of them is offset by half its own size.
        <View style={[styles.field, { left: fieldLeft, top: fieldTop, width: fieldWidth, height: fieldHeight }]}>
          {pips.map((p, i) => (
            <View
              key={`${p.x}-${p.y}-${i}`}
              style={{
                position: 'absolute',
                left: p.x * fieldWidth - pipSize / 2,
                top: p.y * fieldHeight - pipSize / 2,
              }}
            >
              <Suit suit={suit} size={pipSize} color={color} flipped={p.flip} />
            </View>
          ))}
        </View>
      ) : (
        // An ace: one pip, large, dead centre. The only card whose face is a
        // single shape, and the reason `pipsFor` returns nothing for it.
        <View style={styles.centre}>
          <Suit suit={suit} size={acePip} color={color} />
        </View>
      )}

      {/* The indices, last so they sit over anything that reaches the
          corners. Both of them, the bottom one turned over — a card that
          reads from one end only is the thing this face is here to stop
          being. */}
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
