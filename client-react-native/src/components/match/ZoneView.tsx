import { useState } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import type { Zone } from '@/src/api/matchTypes';
import { CardView } from '@/src/components/CardView';
import type { Measurable } from '@/src/hooks/useDropRegistry';
import { groupElementId, zoneElementId } from '@/src/lib/drops';
import { label } from '@/src/lib/labels';
import { colors, dropArmed } from '@/src/theme';

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
  /** Sized to its contents, so small zones can sit beside each other. */
  inline?: boolean;
  /** Publishes this zone and its groups as places a card may be dropped. */
  registerDrop?: (elementId: string, node: Measurable | null) => void;
  /** Element ids that would accept the card currently being dragged. */
  activeDrops?: ReadonlySet<string>;
  /** The one the pointer is over right now. */
  hoveredDrop?: string | null;
  /**
   * Element ids that may be resolved with a press rather than a drag —
   * standing in for a drag when what to send has already been chosen and only
   * where it goes is still open. Disjoint in practice from a live drag: this
   * is populated between drags, not during one.
   */
  pressableDrops?: ReadonlySet<string>;
  onPressDrop?: (elementId: string, pageX: number) => void;
};

export function ZoneView({
  zone,
  selected,
  onPressCard,
  compact,
  inline,
  registerDrop,
  activeDrops,
  hoveredDrop,
  pressableDrops,
  onPressDrop,
}: Props) {
  const title = label(zone.labelKey) || zone.id;
  const zoneId = zoneElementId(zone.id);
  const zoneLive = activeDrops?.has(zoneId) ?? false;

  const cards = zone.cards ?? [];
  /**
   * A pile is about its top card; the rest is history.
   *
   * Only some games send the rest at all — a Canasta or Prší discard pile
   * arrives as a single card, because in those games what is underneath is not
   * public. Where the whole pile *is* sent, showing all of it turns a small
   * zone into a wall of cards that pushes the rest of the board off screen, so
   * it is folded down to the top card and can be opened.
   */
  const foldable = zone.kind === 'pile' && cards.length > 1;
  const [open, setOpen] = useState(false);
  const shown = foldable && !open ? cards.slice(-1) : cards;
  const buried = cards.length - shown.length;

  /** Which melds the player has tapped open, to see past the stacked corners. */
  const [expandedGroups, setExpandedGroups] = useState<ReadonlySet<string>>(new Set());
  const toggleGroup = (id: string) =>
    setExpandedGroups((was) => {
      const next = new Set(was);
      next.has(id) ? next.delete(id) : next.add(id);
      return next;
    });

  return (
    <View
      ref={(n) => registerDrop?.(zoneId, n as unknown as Measurable | null)}
      style={[
        styles.zone,
        inline && styles.inline,
        zoneLive && styles.live,
        hoveredDrop === zoneId && styles.hovered,
      ]}
      testID={`zone-${zone.id}`}
    >
      <View style={styles.headerRow}>
        <Text style={styles.title}>{title}</Text>
        {foldable ? (
          <Pressable
            testID={`zone-toggle-${zone.id}`}
            accessibilityRole="button"
            accessibilityState={{ expanded: open }}
            accessibilityLabel={open ? `Hide the rest of ${title}` : `Show all of ${title}`}
            onPress={() => setOpen((was) => !was)}
            style={styles.toggle}
          >
            <Text style={styles.toggleText} testID={`zone-count-${zone.id}`}>
              {open ? `${zone.count} ▴` : `${zone.count} ▾`}
            </Text>
          </Pressable>
        ) : (
          <Text style={styles.count} testID={`zone-count-${zone.id}`}>
            {zone.count}
          </Text>
        )}
      </View>

      {zone.kind === 'stack' ? <StackBack count={zone.count} /> : null}

      {/* Groups first: a spread's cards belong to its groups, and rendering
          both would show every card twice. */}
      {(zone.groups ?? []).length > 0 ? (
        <View style={styles.groups}>
          {(zone.groups ?? []).map((g) => {
            const groupId = groupElementId(g.id);
            const groupLive = activeDrops?.has(groupId) ?? false;
            const groupPressable = pressableDrops?.has(groupId) ?? false;
            const groupOpen = expandedGroups.has(g.id);
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
                {/* Stacked, not fanned — a meld's cards overlap top to
                    bottom instead of spreading left to right, so a run of
                    seven costs one card's width instead of seven. Every
                    card but the last shows only its top corner (rank and
                    suit are both up there), which is enough to read the
                    group at a glance; the last card sits in full. This also
                    narrows the group box itself, so several fit side by
                    side (see styles.groups) instead of each claiming a full
                    row the way a horizontal fan did. Tapping the meld undoes
                    the overlap so every card shows in full — that tap is
                    disabled while the meld is a live drop target, so it
                    never steals a press meant to resolve the drop. */}
                <Pressable
                  disabled={groupPressable}
                  onPress={() => toggleGroup(g.id)}
                  accessibilityRole="button"
                  accessibilityState={{ expanded: groupOpen }}
                  accessibilityLabel={groupOpen ? 'Collapse this meld' : 'Show all cards in this meld'}
                  testID={`group-toggle-${g.id}`}
                >
                  <View style={styles.stackedCards}>
                    {g.cards.map((c, i) => (
                      <View
                        key={`${g.id}-${c}-${i}`}
                        style={i > 0 && !groupOpen && styles.stackedOverlap}
                      >
                        <CardView card={c} compact stacked={!groupOpen} />
                      </View>
                    ))}
                  </View>
                </Pressable>
                {(g.badgeKeys ?? []).map((b) => (
                  <Text key={b} style={styles.badge}>
                    {label(b)}
                  </Text>
                ))}
                {/* Stands in for a drag once what to send is already chosen —
                    a target lit up this way, and not by a card in flight, is
                    one a press can resolve as well as a drop can. */}
                {groupPressable ? (
                  <Pressable
                    testID={`group-press-${g.id}`}
                    style={StyleSheet.absoluteFill}
                    onPress={(e) => onPressDrop?.(groupId, e.nativeEvent.pageX)}
                  />
                ) : null}
              </View>
            );
          })}
        </View>
      ) : (
        <View style={styles.cards}>
          {/* Indices are into the whole pile, not into what is on screen, so a
              card keeps the same name whether the pile is open or folded. */}
          {shown.map((c, i) => (
            <CardView
              key={`${zone.id}-${c.card}-${buried + i}`}
              card={c.card}
              compact={compact}
              selected={selected?.includes(c.card)}
              onPress={onPressCard ? () => onPressCard(c.card, buried + i) : undefined}
              testID={`card-${zone.id}-${buried + i}`}
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
  // Wide enough to be worth aiming at, since it is the only way to look under
  // the top card.
  toggle: { paddingHorizontal: 6, paddingVertical: 2, marginLeft: 8 },
  toggleText: { color: colors.accentButton, fontSize: 12, fontWeight: '700' },
  // A zone that sizes to its contents rather than filling the row. The draw
  // and discard piles are a couple of cards wide and belong side by side; only
  // a hand or a spread earns a line of its own.
  inline: { flexGrow: 0, flexShrink: 0, marginBottom: 0, minWidth: 96 },
  cards: { flexDirection: 'row', flexWrap: 'wrap', gap: 4, marginTop: 6 },
  // A vertical stack instead of cards.row's horizontal fan — see the comment
  // where this is used. alignItems keeps the column hugging the cards'
  // width instead of stretching to the group box's, which matters once
  // groups sit side by side (below) rather than each spanning full width.
  stackedCards: { flexDirection: 'column', alignItems: 'flex-start', marginTop: 6 },
  // Pulls every card but the first up into the one above it, leaving just
  // its top corner (rank + suit) showing. Tuned to a compact CardView's
  // rendered height (60 + the ring's own border/padding) minus enough room
  // for that corner to stay legible.
  stackedOverlap: { marginTop: -40 },
  // Row + wrap rather than one meld per line: stackedCards narrows each
  // group to about one card's width, so several now fit across before
  // wrapping instead of each claiming a full-width row on its own.
  groups: { flexDirection: 'row', flexWrap: 'wrap', gap: 6, marginTop: 4 },
  group: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
    padding: 4,
  },
  // Only the border colour changes, never its width: a region that grew when
  // it lit up would move every region after it in the middle of the drag,
  // which moves the very measurements the drop is tested against. dropArmed
  // keeps to that too — it only touches colour, style and fill.
  live: { borderColor: colors.accent },
  hovered: dropArmed,
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
