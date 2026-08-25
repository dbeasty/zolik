import { useMemo } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import { useMetrics } from '@/src/hooks/useMetrics';
import { parseCard } from '@/src/lib/cards';
import type { CardMetrics } from '@/src/lib/layout';
import { colors } from '@/src/theme';

/**
 * How much room a card takes, at scale 1 — the size the sizes in
 * `src/lib/layout.ts` are derived from. Exported because a couple of call
 * sites need the *unscaled* metric for something other than drawing a card
 * (see `useDropRegistry`'s HIT_SLOP comment); anything that draws needs the
 * scaled numbers from `useMetrics().card` instead, not this.
 */
export const CARD_METRICS = {
  width: 52,
  height: 72,
  gap: 6,
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

/** Every dimension a card's own render needs, computed once per card size. */
function cardStyles(m: CardMetrics) {
  return StyleSheet.create({
    ring: {
      borderRadius: 8,
      borderWidth: m.ringBorder,
      borderColor: 'transparent',
      padding: m.ringPadding,
    },
    justDrawnRing: { borderColor: colors.success },
    draggingRing: { borderColor: colors.accent },
    card: {
      width: m.width,
      height: m.height,
      backgroundColor: colors.cardBg,
      borderRadius: 6,
      borderWidth: 2,
      borderColor: colors.cardBorder,
      padding: 4,
      marginRight: m.gap,
      justifyContent: 'space-between',
    },
    compact: {
      width: m.compactWidth,
      height: m.compactHeight,
    },
    selected: {
      borderColor: colors.gold,
      backgroundColor: '#fffbeb',
    },
    joker: { backgroundColor: '#fef3c7' },
    pressed: { opacity: 0.85 },
    rank: {
      fontSize: m.rankFont,
      fontWeight: '700',
      color: '#1e293b',
    },
    // "JKR" is 3 characters vs. 1-2 for every other rank, so it needs its own
    // (smaller) size to stay inside the card instead of overflowing the edge.
    jokerRank: { fontSize: m.jokerRankFont },
    suit: {
      fontSize: m.suitFont,
      alignSelf: 'center',
      color: '#1e293b',
    },
    corner: {
      flexDirection: 'row',
      alignItems: 'center',
      gap: 2,
    },
    suitInline: {
      fontSize: m.suitInlineFont,
      color: '#1e293b',
    },
    red: { color: '#dc2626' },
  });
}

export function CardView({ card, selected, justDrawn, dragging, onPress, compact, stacked, testID }: Props) {
  const metrics = useMetrics();
  const d = parseCard(card);
  // Recomputed only when the card's own size changes (a resize, or a device
  // rotation) — every other render of a card reuses the same style objects.
  const styles = useMemo(() => cardStyles(metrics.card), [metrics.card]);

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
