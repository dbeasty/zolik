import { StyleSheet, Text, View } from 'react-native';

import type { Zone } from '@/src/api/matchTypes';
import { CardView } from '@/src/components/CardView';
import { label } from '@/src/lib/labels';
import { colors } from '@/src/theme';

/**
 * One area of the board, laid out by its *kind* rather than its meaning.
 *
 * Four kinds cover every game here, which is the finding worth recording: a
 * fanned hand, a face-down stack, a face-up pile and a spread of groups
 * describe a rummy table, a shedding pile, a canasta partnership's melds and a
 * poker board without any of them being special-cased. A game with no melds
 * simply never sends a spread.
 *
 * A hidden zone arrives with a count and no cards. That is the whole anti-cheat
 * surface and it needs no cooperation from this file — there is nothing to
 * leak, because nothing was sent.
 */

type Props = {
  zone: Zone;
  /** Cards the player has selected, for a zone they can act from. */
  selected?: string[];
  onPressCard?: (card: string, index: number) => void;
  compact?: boolean;
};

export function ZoneView({ zone, selected, onPressCard, compact }: Props) {
  const title = label(zone.labelKey) || zone.id;

  return (
    <View style={styles.zone} testID={`zone-${zone.id}`}>
      <View style={styles.headerRow}>
        <Text style={styles.title}>{title}</Text>
        <Text style={styles.count} testID={`zone-count-${zone.id}`}>
          {zone.count}
        </Text>
      </View>

      {zone.kind === 'stack' ? <StackBack count={zone.count} /> : null}

      {/* Groups first: a spread's cards belong to its groups, and rendering
          both would show every card twice. */}
      {(zone.groups ?? []).length > 0 ? (
        <View style={styles.groups}>
          {(zone.groups ?? []).map((g) => (
            <View key={g.id} style={styles.group} testID={`group-${g.id}`}>
              <View style={styles.cards}>
                {g.cards.map((c, i) => (
                  <CardView key={`${g.id}-${c}-${i}`} card={c} compact />
                ))}
              </View>
              {(g.badgeKeys ?? []).map((b) => (
                <Text key={b} style={styles.badge}>
                  {label(b)}
                </Text>
              ))}
            </View>
          ))}
        </View>
      ) : (
        <View style={styles.cards}>
          {(zone.cards ?? []).map((c, i) => (
            <CardView
              key={`${zone.id}-${c.card}-${i}`}
              card={c.card}
              compact={compact}
              selected={selected?.includes(c.card)}
              onPress={onPressCard ? () => onPressCard(c.card, i) : undefined}
              testID={`card-${zone.id}-${i}`}
            />
          ))}
        </View>
      )}

      {/* A zone with a count and nothing to show is somebody else's hand, or a
          pile whose contents are not in play. Saying so beats an empty box. */}
      {zone.count > 0 && !(zone.cards ?? []).length && !(zone.groups ?? []).length && zone.kind !== 'stack' ? (
        <Text style={styles.hidden}>{zone.count} hidden</Text>
      ) : null}
    </View>
  );
}

/** A face-down pile: the count is the only thing that matters about it. */
function StackBack({ count }: { count: number }) {
  if (count <= 0) return <Text style={styles.hidden}>empty</Text>;
  return (
    <View style={styles.back}>
      <Text style={styles.backText}>{count}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  zone: {
    backgroundColor: colors.surface,
    borderRadius: 10,
    borderWidth: 1,
    borderColor: colors.border,
    padding: 8,
    marginBottom: 8,
  },
  headerRow: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center' },
  title: { color: colors.muted, fontSize: 12, fontWeight: '700' },
  count: { color: colors.muted, fontSize: 12 },
  cards: { flexDirection: 'row', flexWrap: 'wrap', gap: 4, marginTop: 6 },
  groups: { gap: 6, marginTop: 4 },
  group: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
    padding: 4,
  },
  badge: { color: colors.gold, fontSize: 10, marginTop: 2 },
  hidden: { color: colors.muted, fontSize: 11, marginTop: 6, fontStyle: 'italic' },
  back: {
    marginTop: 6,
    width: 44,
    height: 62,
    borderRadius: 6,
    borderWidth: 1,
    borderColor: colors.border,
    backgroundColor: colors.accentDim,
    alignItems: 'center',
    justifyContent: 'center',
  },
  backText: { color: colors.text, fontWeight: '700' },
});
