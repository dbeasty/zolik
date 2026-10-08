import { useMemo } from 'react';
import { StyleSheet, Text, View } from 'react-native';

import { Suit } from '@/src/components/cards/Suit';
import { useMetrics } from '@/src/hooks/useMetrics';
import { useSkin } from '@/src/hooks/useSkin';
import { INKS, isLastCardCode, parseLastCard } from '@/src/components/cards/lastCardArt';
import { parseCard } from '@/src/lib/cards';
import { CARD_INDEX_BORDER, cardIndexBox, type CardIndexBox } from '@/src/lib/layout';
import type { Skin } from '@/src/skins/types';

/**
 * A card drawn as nothing but its index — the rank-and-suit mark printed in
 * a playing card's corner, which is what the word means to a printer.
 *
 * The smallest a card can be drawn and still *be* that card: the rank and the
 * suit, side by side, on a sliver of stock. It is the corner of a card with
 * the card taken away — which is all anybody reads off a card they are not
 * about to play.
 *
 * It exists for the places where a whole card cannot be afforded. A closed
 * group of cards on a phone was a column of overlapped card corners, each
 * costing about twenty pixels and the last one a whole card's height; as
 * indices the same cards cost about a third of that and say exactly the same
 * thing — every card, in order, none of them summarised or dropped.
 *
 * Lossless is the point. A *summary* tile — one rank and its suits for a
 * group that repeats a rank, the two ends for a sequence — reads better and
 * is shorter still, and the board may not draw one: knowing what a group's
 * ends are is knowing what the game's groups are, and the shell is not
 * allowed to know any game's words (see `src/lib/__tests__/shell.test.ts`).
 * An index knows only what a card is, which is a fact about a deck.
 *
 * Sizes come from `cardIndexBox`, never from the skin; the skin only colours.
 */

type Props = {
  card: string;
  testID?: string;
};

export function CardIndex({ card, testID }: Props) {
  const metrics = useMetrics();
  const skin = useSkin();
  const box = useMemo(() => cardIndexBox(metrics), [metrics]);
  const styles = useMemo(() => indexStyles(box, skin), [box, skin]);
  const d = parseCard(card);

  // A Last Card index is its number and its colour's shape, both in that
  // colour — the shape is a glyph, so it is type like the rest of the index.
  const lc = isLastCardCode(card) ? parseLastCard(card) : null;
  if (lc) {
    const color = lc.colour ? INKS[lc.colour].deep : skin.card.ink;
    return (
      <View style={styles.box} testID={testID}>
        <Text style={[styles.rank, { color }]} numberOfLines={1}>
          {d.rank}
          {d.suitSymbol}
        </Text>
      </View>
    );
  }

  return (
    <View style={styles.box} testID={testID}>
      <Text style={[styles.rank, d.isJoker && styles.jokerRank, d.isRed && styles.red]} numberOfLines={1}>
        {d.rank}
      </Text>
      {d.isJoker ? (
        <Text style={styles.star}>★</Text>
      ) : (
        <Suit suit={d.suit} size={box.markSize} color={d.isRed ? skin.card.red : skin.card.ink} />
      )}
    </View>
  );
}

function indexStyles(b: CardIndexBox, s: Skin) {
  const line = b.height - 2 * CARD_INDEX_BORDER;
  return StyleSheet.create({
    box: {
      width: b.width,
      height: b.height,
      flexDirection: 'row',
      alignItems: 'center',
      justifyContent: 'center',
      gap: 2,
      borderRadius: 3,
      borderWidth: CARD_INDEX_BORDER,
      borderColor: s.colors.cardBorder,
      backgroundColor: s.colors.cardBg,
      overflow: 'hidden',
    },
    rank: {
      color: s.card.ink,
      fontWeight: '700',
      fontSize: b.rankFont,
      lineHeight: line,
    },
    // "JKR" is three characters where every other rank is one or two.
    jokerRank: { fontSize: Math.max(7, Math.round(b.rankFont * 0.7)) },
    star: { color: s.card.red, fontWeight: '700', fontSize: b.markSize, lineHeight: line },
    red: { color: s.card.red },
  });
}
