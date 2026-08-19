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
  onPress?: () => void;
  compact?: boolean;
};

export function CardView({ card, selected, justDrawn, onPress, compact }: Props) {
  const d = parseCard(card);
  const content = (
    // Ring wrapper is always present at a fixed size (border color just
    // toggles transparent<->success) so highlighting a card never nudges
    // its neighbors' layout — a card that shifts mid-gesture is exactly
    // what broke double-tap-to-discard before this was fixed.
    <View style={[styles.ring, justDrawn && styles.justDrawnRing]}>
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
