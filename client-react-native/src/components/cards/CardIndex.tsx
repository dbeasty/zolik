import { useMemo } from 'react';
import { StyleSheet, Text, View } from 'react-native';

import { Suit } from '@/src/components/cards/Suit';
import { useMetrics } from '@/src/hooks/useMetrics';
import { useSkin } from '@/src/hooks/useSkin';
import { parseCard } from '@/src/lib/cards';
import type { Metrics } from '@/src/lib/layout';
import type { Skin } from '@/src/skins/types';

/**
 * A card drawn as nothing but its index — the rank-and-suit mark printed in
 * a playing card's corner, which is what the word means to a printer.
 *
 * The smallest a card can be drawn and still *be* that card: the rank and the
 * suit, side by side, on a sliver of stock. It is the corner of a card with
 * the card taken away — which is all anybody reads off a card they are not
 * about to play, and is why a real player holding a dozen of them fans them
 * until only the corners show.
 *
 * It exists for the places where a whole card cannot be afforded. A group of
 * cards on a phone used to be drawn as a column of overlapped card corners,
 * each costing about twenty pixels and the last one a whole card's height:
 * seven of them came to 176px, and eight groups came to most of the board. As
 * indices the same seven cost 98px and say exactly the same thing — every
 * card, in order, none of them summarised or dropped.
 *
 * Lossless is the point. The alternative that was tried first was a *summary*
 * tile — "A" with three suit pips for a set of aces, the two ends for a run —
 * which reads better and is shorter still, and which this client may not have:
 * knowing that a run is its ends, or that a set repeats its rank, is knowing
 * what a set and a run are, and the shell is not allowed to know any game's
 * words (see `src/lib/__tests__/shell.test.ts`, which enforces exactly that).
 * An index knows only what a card is, which is a fact about a deck.
 */

type Props = {
  card: string;
  /** Sized to sit inside a compact zone rather than a full-size one. */
  compact?: boolean;
  testID?: string;
};

export function CardIndex({ card, compact, testID }: Props) {
  const metrics = useMetrics();
  const skin = useSkin();
  const styles = useMemo(() => indexStyles(metrics, skin, !!compact), [metrics, skin, compact]);
  const d = parseCard(card);
  const pip = Math.max(7, Math.round(indexWidth(metrics, !!compact) * 0.3));

  return (
    <View style={styles.box} testID={testID}>
      <Text style={[styles.rank, d.isJoker && styles.jokerRank, d.isRed && styles.red]} numberOfLines={1}>
        {d.rank}
      </Text>
      {d.isJoker ? (
        <Text style={[styles.star, { fontSize: pip }]}>★</Text>
      ) : (
        <Suit suit={d.suit} size={pip} color={d.isRed ? skin.card.red : skin.card.ink} />
      )}
    </View>
  );
}

function indexWidth(m: Metrics, compact: boolean): number {
  return compact ? m.card.compactWidth : m.card.width;
}

/**
 * How tall one index is, and so how much of the screen a column of them costs.
 * Exported because a caller laying several out needs the number before it has
 * drawn any of them — the same reason every other size on this board lives in
 * one place.
 */
export function indexHeight(m: Metrics, compact: boolean): number {
  return Math.max(14, Math.round((compact ? m.card.compactHeight : m.card.height) * 0.28));
}

function indexStyles(m: Metrics, s: Skin, compact: boolean) {
  const width = indexWidth(m, compact);
  const height = indexHeight(m, compact);
  return StyleSheet.create({
    box: {
      width,
      height,
      flexDirection: 'row',
      alignItems: 'center',
      justifyContent: 'center',
      gap: 2,
      borderRadius: 3,
      borderWidth: 1,
      borderColor: s.colors.cardBorder,
      backgroundColor: s.colors.cardBg,
    },
    rank: {
      color: s.card.ink,
      fontWeight: '700',
      fontSize: Math.max(9, Math.round(height * 0.62)),
      lineHeight: Math.max(10, Math.round(height * 0.72)),
    },
    // "JKR" is three characters where every other rank is one or two.
    jokerRank: { fontSize: Math.max(7, Math.round(height * 0.44)) },
    star: { color: s.card.red, fontWeight: '700' },
    red: { color: s.card.red },
  });
}
