import { Pressable, StyleSheet, Text, View } from 'react-native';

import { parseCard } from '@/src/lib/cards';
import { colors } from '@/src/theme';

/**
 * How much room a card takes.
 *
 * Exported because the hand draws a gap the exact size of a card, to show
 * where a dragged one will land, and a gap that is nearly card-sized is worse
 * than no gap at all — the cards visibly shuffle by a few pixels as it moves.
 */
export const CARD_METRICS = {
  width: 52,
  height: 72,
  /** Space after each card, part of what a slot occupies. */
  gap: 6,
  /** The ring drawn around every card, whether or not it is coloured in. */
  ringPadding: 1,
  ringBorder: 2,
  compactWidth: 44,
  compactHeight: 60,
};

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
  // A card in a stacked meld shows only its top corner — the rest is under
  // the next card in the pile. The rank and suit need to share that corner
  // side by side, rather than the rank on top and the suit centered below,
  // or the suit is exactly what the overlap crops off.
  stacked?: boolean;
  // Passed straight through to the outer wrapper (data-testid on web) — set
  // by the caller, which knows the card's context (hand index, staged
  // group, table meld), since CardView itself doesn't.
  testID?: string;
};

export function CardView({ card, selected, justDrawn, dragging, onPress, compact, stacked, testID }: Props) {
  const d = parseCard(card);
  const content = (
    // Ring wrapper is always present at a fixed size (border color just
    // toggles transparent<->success/accent) so highlighting a card never
    // nudges its neighbors' layout — a card that shifts mid-gesture is
    // exactly what broke double-tap-to-discard before this was fixed.
    <View
      testID={testID}
      // Selectedness said out loud rather than only drawn. A gold border is
      // invisible to a screen reader, and it is also the only evidence a test
      // could otherwise check — which would mean asserting on a hex colour,
      // and those change for design reasons that have nothing to do with
      // whether the card is picked.
      //
      // `aria-selected` rather than `accessibilityState={{selected}}`: React
      // Native has taken the aria spelling since 0.71 and maps it back to
      // accessibilityState on iOS and Android, while react-native-web forwards
      // it to the DOM. The older spelling reaches native but this version of
      // react-native-web drops it, so it would leave the web build silently
      // saying nothing.
      aria-selected={!!selected}
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
        {stacked ? (
          <View style={styles.corner}>
            <Text style={[styles.rank, d.isJoker && styles.jokerRank, d.isRed && styles.red]}>
              {d.rank}
            </Text>
            <Text style={[styles.suitInline, d.isRed && styles.red]}>{d.suitSymbol}</Text>
          </View>
        ) : (
          <>
            <Text style={[styles.rank, d.isJoker && styles.jokerRank, d.isRed && styles.red]}>
              {d.rank}
            </Text>
            <Text style={[styles.suit, d.isRed && styles.red]}>{d.suitSymbol}</Text>
          </>
        )}
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
    borderWidth: CARD_METRICS.ringBorder,
    borderColor: 'transparent',
    padding: CARD_METRICS.ringPadding,
  },
  justDrawnRing: {
    borderColor: colors.success,
  },
  draggingRing: {
    borderColor: colors.accent,
  },
  card: {
    width: CARD_METRICS.width,
    height: CARD_METRICS.height,
    backgroundColor: colors.cardBg,
    borderRadius: 6,
    borderWidth: 2,
    borderColor: colors.cardBorder,
    padding: 4,
    marginRight: CARD_METRICS.gap,
    justifyContent: 'space-between',
  },
  compact: {
    width: CARD_METRICS.compactWidth,
    height: CARD_METRICS.compactHeight,
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
  // "JKR" is 3 characters vs. 1-2 for every other rank, so it needs its own
  // (smaller) size to stay inside the card instead of overflowing the edge.
  jokerRank: {
    fontSize: 11,
  },
  suit: {
    fontSize: 18,
    alignSelf: 'center',
    color: '#1e293b',
  },
  corner: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 2,
  },
  suitInline: {
    fontSize: 14,
    color: '#1e293b',
  },
  red: {
    color: '#dc2626',
  },
});
