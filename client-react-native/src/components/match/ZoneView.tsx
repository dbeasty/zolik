import { useMemo, useState } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import type { Zone } from '@/src/api/matchTypes';
import { CardGlance } from '@/src/components/match/CardGlance';
import { CardView } from '@/src/components/CardView';
import { Panel, type Measurable } from '@/src/components/match/Panel';
import { useMetrics } from '@/src/hooks/useMetrics';
import { groupElementId, zoneElementId } from '@/src/lib/drops';
import type { Metrics } from '@/src/lib/layout';
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
 * leak, because nothing was sent. (Whether such a zone is drawn at all is
 * decided upstream, by `drawableZones` in `src/lib/board.ts` — this file just
 * renders whatever it's handed.)
 *
 * It is also where a dragged card can be let go of. Zones and the groups
 * inside them register themselves as drop regions under the ids the offers
 * name them by, and light up when the card in hand is one they would take.
 * This file decides none of that — it is told which of its ids are live.
 *
 * Drawn inside a `Panel`, which is what gives every zone a minimize control
 * for free. A panel that is a live drop target is held open regardless of
 * whether the player minimized it — see `forceOpen` below — because a target
 * a preference can hide is a target a player can lose a card into.
 */

type Props = {
  zone: Zone;
  /** Cards the player has selected, for a zone they can act from. */
  selected?: string[];
  onPressCard?: (card: string, index: number) => void;
  compact?: boolean;
  /** Sized to its contents, so small zones can sit beside each other. */
  inline?: boolean;
  /** A softer look for a panel drawn inside another panel — see `Panel`'s own `nested`. */
  nested?: boolean;
  /** Names this panel by something other than the zone's own label — an owner's name, typically. */
  title?: string;
  /** A second line under `title` — falls back to the zone's own label when `title` is supplied. */
  subtitle?: string;
  /** Stable id for remembering whether this panel is put away. Omit for a panel that may not be minimized. */
  panelId?: string;
  minimized?: boolean;
  onToggleMinimized?: () => void;
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
  nested,
  title: titleOverride,
  subtitle: subtitleOverride,
  panelId,
  minimized,
  onToggleMinimized,
  registerDrop,
  activeDrops,
  hoveredDrop,
  pressableDrops,
  onPressDrop,
}: Props) {
  const metrics = useMetrics();
  const styles = useMemo(() => zoneStyles(metrics), [metrics]);

  const zoneLabel = label(zone.labelKey) || zone.id;
  const title = titleOverride || zoneLabel;
  const subtitle = titleOverride ? subtitleOverride ?? zoneLabel : subtitleOverride;

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

  // A minimized panel still has to open for a drag aimed at it, or at one of
  // its own groups — a target a preference can hide is a target a card can be
  // lost into.
  const groupsLive = (zone.groups ?? []).some((g) => {
    const gid = groupElementId(g.id);
    return (activeDrops?.has(gid) ?? false) || (pressableDrops?.has(gid) ?? false);
  });
  const forceOpen = zoneLive || groupsLive;

  // Stands in for a drag once what to send is already chosen and only where
  // it goes is still open — the same thing a group's own press overlay does,
  // but for an offer that names the whole zone rather than one group inside
  // it (composing a brand-new meld on an empty spread; discarding onto a
  // pile). Rendered first among the zone's real content, so a group's own
  // overlay — painted after, and so on top — still wins a tap inside it: the
  // zone-wide target only ever catches what no group claimed.
  const zonePressable = pressableDrops?.has(zoneId) ?? false;

  // What a put-away panel says about itself on its collapsed header — kind
  // decides the shape, same as it decides the layout above. A stack needs
  // none of this: its count is the whole of what it ever shows, open or not.
  const groups = zone.groups ?? [];
  const summary =
    zone.kind === 'pile' && cards.length
      ? <CardGlance cards={[cards[cards.length - 1].card]} max={1} testID={`zone-summary-${zone.id}`} />
      : zone.kind === 'spread' && groups.length
        ? (
            <View style={styles.summaryRow} testID={`zone-summary-${zone.id}`}>
              <Text style={styles.summaryCount}>{groups.length}</Text>
              <CardGlance cards={groups.map((g) => g.cards[g.cards.length - 1])} max={4} />
            </View>
          )
        : zone.kind === 'hand' && cards.length
          ? <CardGlance cards={cards.map((c) => c.card)} max={6} testID={`zone-summary-${zone.id}`} />
          : undefined;

  return (
    <Panel
      panelId={panelId}
      title={title}
      subtitle={subtitle}
      inline={inline}
      nested={nested}
      live={zoneLive}
      hovered={hoveredDrop === zoneId}
      minimized={minimized}
      onToggleMinimized={onToggleMinimized}
      forceOpen={forceOpen}
      testID={`zone-${zone.id}`}
      innerRef={(n) => registerDrop?.(zoneId, n)}
      count={foldable ? undefined : zone.count}
      countTestID={foldable ? undefined : `zone-count-${zone.id}`}
      summary={summary}
      accessory={
        foldable ? (
          // A bordered badge with an outline chevron — visually its own
          // thing, not a second copy of the minimize control's solid ▾/▸
          // sitting right next to it. Two adjacent triangles meaning two
          // different things was a coin flip for whoever was looking at it.
          <Pressable
            testID={`zone-toggle-${zone.id}`}
            accessibilityRole="button"
            accessibilityState={{ expanded: open }}
            accessibilityLabel={open ? `Hide the rest of ${zoneLabel}` : `Show all of ${zoneLabel}`}
            onPress={() => setOpen((was) => !was)}
            style={styles.toggle}
          >
            <Text style={styles.toggleText} testID={`zone-count-${zone.id}`}>
              {zone.count} {open ? '⌃' : '⌄'}
            </Text>
          </Pressable>
        ) : undefined
      }
    >
      <View style={styles.content}>
        {zonePressable ? (
          <Pressable
            testID={`zone-press-${zone.id}`}
            style={StyleSheet.absoluteFill}
            onPress={(e) => onPressDrop?.(zoneId, e.nativeEvent.pageX)}
          />
        ) : null}

        {zone.kind === 'stack' ? <StackBack count={zone.count} compact={compact} metrics={metrics} /> : null}

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
                  accessibilityLabel={groupOpen ? 'Collapse this group' : 'Show all cards in this group'}
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
    </Panel>
  );
}

/**
 * A face-down pile: the count is the only thing that matters about it.
 *
 * Sized to match the ring-wrapped card it sits beside exactly — the discard
 * pile beside it shows a real CardView, ring and all, and a guessed box here
 * drifted out of sync with that the moment either one's metrics changed.
 */
function StackBack({ count, compact, metrics }: { count: number; compact?: boolean; metrics: Metrics }) {
  const styles = useMemo(() => zoneStyles(metrics), [metrics]);
  if (count <= 0) return <Text style={styles.hidden}>empty</Text>;
  return (
    <View style={[styles.back, compact ? styles.backCompact : styles.backFull]}>
      <Text style={styles.backText}>{count}</Text>
    </View>
  );
}

function zoneStyles(m: Metrics) {
  const ringBorderAndPadding = 2 * (m.card.ringPadding + m.card.ringBorder);
  // A standalone card's ring is sized to its content, which includes the
  // card's own trailing gap (there for spacing in a fanned hand) even when
  // nothing follows it — so matching the ring's true rendered width means
  // counting that gap too, not just the ring's own border and padding.
  const ringOuterWidth = ringBorderAndPadding + m.card.gap;
  const ringOuterHeight = ringBorderAndPadding;

  return StyleSheet.create({
    // Wide enough to be worth aiming at, since it is the only way to look under
    // the top card.
    toggle: {
      paddingHorizontal: 6,
      paddingVertical: 2,
      borderRadius: 6,
      borderWidth: 1,
      borderColor: colors.border,
    },
    // Wraps everything but the header, so a zone-wide press overlay
    // (`zone-press-<id>`) fills exactly this and never the header above it —
    // the minimize chevron and the pile's own show-all toggle live there and
    // must stay reachable even while the zone is a live target.
    content: { position: 'relative' },
    summaryRow: { flexDirection: 'row', alignItems: 'center', gap: 4, flexShrink: 1, minWidth: 0 },
    summaryCount: { color: colors.muted, fontSize: m.panel.bodyFont },
    toggleText: { color: colors.accentButton, fontSize: m.panel.bodyFont, fontWeight: '700' },
    cards: { flexDirection: 'row', flexWrap: 'wrap', gap: 4, marginTop: 6 },
    // A vertical stack instead of cards.row's horizontal fan — see the comment
    // where this is used. alignItems keeps the column hugging the cards'
    // width instead of stretching to the group box's, which matters once
    // groups sit side by side (below) rather than each spanning full width.
    stackedCards: { flexDirection: 'column', alignItems: 'flex-start', marginTop: 6 },
    // Pulls every card but the first up into the one above it, leaving just
    // its top corner (rank + suit) showing.
    stackedOverlap: { marginTop: -(m.card.compactHeight + ringOuterHeight - m.stackedCorner) },
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
      borderRadius: 6,
      borderWidth: 1,
      borderColor: colors.border,
      backgroundColor: colors.accentDim,
      alignItems: 'center',
      justifyContent: 'center',
    },
    backCompact: {
      width: m.card.compactWidth + ringOuterWidth,
      height: m.card.compactHeight + ringOuterHeight,
    },
    backFull: {
      width: m.card.width + ringOuterWidth,
      height: m.card.height + ringOuterHeight,
    },
    backText: { color: colors.text, fontWeight: '700' },
  });
}
