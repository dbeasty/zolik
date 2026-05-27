import { Pressable, StyleSheet, Text, View } from 'react-native';

import { parseCard } from '@/src/lib/cards';
import { colors } from '@/src/theme';

type Props = {
  card: string;
  selected?: boolean;
  onPress?: () => void;
  compact?: boolean;
};

export function CardView({ card, selected, onPress, compact }: Props) {
  const d = parseCard(card);
  const content = (
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
