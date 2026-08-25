import { StyleSheet, Text, View } from 'react-native';

import type { Zone } from '@/src/api/matchTypes';
import { CardView } from '@/src/components/CardView';
import type { Measurable } from '@/src/hooks/useDropRegistry';
import { groupElementId, zoneElementId } from '@/src/lib/drops';
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
 *
 * It is also where a dragged card can be let go of. Zones and the groups
 * inside them register themselves as drop regions under the ids the offers
 * name them by, and light up when the card in hand is one they would take.
 * This file decides none of that — it is told which of its ids are live.
 */

type Props = {
  zone: Zone;
  /** Cards the player has selected, for a zone they can act from. */
  selected?: string[];
  onPressCard?: (card: string, index: number) => void;
  compact?: boolean;
  /** Publishes this zone and its groups as places a card may be dropped. */
  registerDrop?: (elementId: string, node: Measurable | null) => void;
  /** Element ids that would accept the card currently being dragged. */
  activeDrops?: ReadonlySet<string>;
  /** The one the pointer is over right now. */
  hoveredDrop?: string | null;
};

export function ZoneView({
  zone,
  selected,
  onPressCard,
  compact,
  registerDrop,
  activeDrops,
  hoveredDrop,
}: Props) {
  const title = label(zone.labelKey) || zone.id;
  const zoneId = zoneElementId(zone.id);
  const zoneLive = activeDrops?.has(zoneId) ?? false;

  return (
    <View
      ref={(n) => registerDrop?.(zoneId, n as unknown as Measurable | null)}
      style={[styles.zone, zoneLive && styles.live, hoveredDrop === zoneId && styles.hovered]}
      testID={`zone-${zone.id}`}
    >
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
          {(zone.groups ?? []).map((g) => {
            const groupId = groupElementId(g.id);
            const groupLive = activeDrops?.has(groupId) ?? false;
            return (
              <View
                key={g.id}
                ref={(n) => registerDrop?.(groupId, n as unknown as Measurable | null)}
                style={[
                  styles.group,
                  groupLive && styles.live,
                  hoveredDrop === groupId && styles.hovered,
                ]}
                testID={`group-${g.id}`}
              >
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
            );
          })}
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

      {/* An empty spread that can be dropped on says so, because otherwise the
          first meld of the game has an invisible target. */}
      {zoneLive && !(zone.cards ?? []).length && !(zone.groups ?? []).length ? (
        <Text style={styles.dropHere} testID={`drop-here-${zone.id}`}>
          Drop here
        </Text>
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
  // Only the border colour changes, never its width: a region that grew when
  // it lit up would move every region after it in the middle of the drag,
  // which moves the very measurements the drop is tested against.
  live: { borderColor: colors.accent },
  hovered: { borderColor: colors.gold, backgroundColor: colors.accentDim },
  badge: { color: colors.gold, fontSize: 10, marginTop: 2 },
  hidden: { color: colors.muted, fontSize: 11, marginTop: 6, fontStyle: 'italic' },
  dropHere: { color: colors.gold, fontSize: 11, marginTop: 6, fontStyle: 'italic' },
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
