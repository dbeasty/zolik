import { Pressable, StyleSheet, Text, View } from 'react-native';

import { parseCard } from '@/src/lib/cards';
import { colors } from '@/src/theme';

type Props = {
  card: string;
  selected?: boolean;
  // Marks the card(s) just picked up from the deck or discard pile this
  // turn, so it's obvious which card is new — independent of `selected`
  // (a drawn card can be both, e.g. right after tapping it to stage a meld).
  justDrawn?: boolean;
  // The floating copy of a card currently being dragged — a distinct ring
  // color from justDrawn/selected so "this is what my finger is holding
  // right now" reads as its own state, not a reused one.
  dragging?: boolean;
  onPress?: () => void;
  compact?: boolean;
  // Passed straight through to the outer wrapper (data-testid on web) — set
  // by the caller, which knows the card's context (hand index, staged
  // group, table meld), since CardView itself doesn't.
  testID?: string;
};

export function CardView({ card, selected, justDrawn, dragging, onPress, compact, testID }: Props) {
  const d = parseCard(card);
  const content = (
    // Ring wrapper is always present at a fixed size (border color just
    // toggles transparent<->success/accent) so highlighting a card never
    // nudges its neighbors' layout — a card that shifts mid-gesture is
    // exactly what broke double-tap-to-discard before this was fixed.
    <View
      testID={testID}
      style={[styles.ring, justDrawn && styles.justDrawnRing, dragging && styles.draggingRing]}
    >
      <View
        style={[
          styles.card,
          compact && styles.compact,
          selected && styles.selected,
          d.isJoker && styles.joker,
        ]}
      >
        <Text style={[styles.rank, d.isRed && styles.red]}>{d.rank}</Text>
        <Text style={[styles.suit, d.isRed && styles.red]}>{d.suitSymbol}</Text>
      </View>
    </View>
  );
  if (onPress) {
    return (
      <Pressable onPress={onPress} style={({ pressed }) => pressed && styles.pressed}>
        {content}
      </Pressable>
    );
  }
  return content;
}

const styles = StyleSheet.create({
  ring: {
    borderRadius: 8,
    borderWidth: 2,
    borderColor: 'transparent',
    padding: 1,
  },
  justDrawnRing: {
    borderColor: colors.success,
  },
  draggingRing: {
    borderColor: colors.accent,
  },
  card: {
    width: 52,
    height: 72,
    backgroundColor: colors.cardBg,
    borderRadius: 6,
    borderWidth: 2,
    borderColor: colors.cardBorder,
    padding: 4,
    marginRight: 6,
    justifyContent: 'space-between',
  },
  compact: {
    width: 44,
    height: 60,
  },
  selected: {
    borderColor: colors.gold,
    backgroundColor: '#fffbeb',
  },
  joker: {
    backgroundColor: '#fef3c7',
  },
  pressed: {
    opacity: 0.85,
  },
  rank: {
    fontSize: 14,
    fontWeight: '700',
    color: '#1e293b',
  },
  suit: {
    fontSize: 18,
    alignSelf: 'center',
    color: '#1e293b',
  },
  red: {
    color: '#dc2626',
  },
});
