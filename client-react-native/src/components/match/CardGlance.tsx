import { useMemo } from 'react';
import { StyleSheet, Text, View } from 'react-native';

import { useMetrics } from '@/src/hooks/useMetrics';
import { parseCard } from '@/src/lib/cards';
import type { Metrics } from '@/src/lib/layout';
import { colors } from '@/src/theme';

/**
 * A tiny, game-blind digest of some cards — rank and suit as text, capped,
 * with a `+n` tail for the rest.
 *
 * Built for a panel's collapsed rail: a minimized hand or pile still says
 * what's in it, without drawing a full `CardView` for each one. Uses the same
 * `parseCard` a real card does, so this reading and a card's own can never
 * disagree about what a string means — it's the same reading, laid out
 * smaller, not a second cheaper one.
 */

type Props = {
  cards: string[];
  /** How many to draw before folding the rest into a `+n` tail. */
  max?: number;
  testID?: string;
};

export function CardGlance({ cards, max = 6, testID = 'card-glance' }: Props) {
  const metrics = useMetrics();
  const styles = useMemo(() => glanceStyles(metrics), [metrics]);

  if (!cards.length) return null;

  const shown = cards.slice(0, max);
  const rest = cards.length - shown.length;

  return (
    <View style={styles.row} testID={testID}>
      {shown.map((card, i) => {
        const d = parseCard(card);
        return (
          <Text key={`${card}-${i}`} style={[styles.one, d.isRed && styles.red]} testID={`glance-${card}-${i}`}>
            {d.rank}
            {d.suitSymbol}
          </Text>
        );
      })}
      {rest > 0 ? <Text style={styles.tail}>+{rest}</Text> : null}
    </View>
  );
}

function glanceStyles(m: Metrics) {
  return StyleSheet.create({
    row: { flexDirection: 'row', flexShrink: 1, gap: 9, alignItems: 'center', flexWrap: 'nowrap', overflow: 'hidden' },
    // Noticeably bigger than the panel's own title — this is meant to read
    // as the actual content of a collapsed panel, not as fine print next to
    // the label. The title says whose hand it is; this is the hand.
    one: { color: colors.text, fontSize: m.panel.titleFont + 4, fontWeight: '700' },
    red: { color: '#f87171' },
    tail: { color: colors.muted, fontSize: m.panel.titleFont + 2, fontWeight: '600' },
  });
}
