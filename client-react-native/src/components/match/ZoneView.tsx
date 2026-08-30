import { useMemo, useState } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import type { Zone } from '@/src/api/matchTypes';
import { CardBack } from '@/src/components/CardBack';
import { CardGlance } from '@/src/components/match/CardGlance';
import { CardView } from '@/src/components/CardView';
import { Panel, type Measurable } from '@/src/components/match/Panel';
import { SettleIn } from '@/src/components/match/SettleIn';
import { useMetrics } from '@/src/hooks/useMetrics';
import { useSkin } from '@/src/hooks/useSkin';
import { groupElementId, zoneElementId } from '@/src/lib/drops';
import type { Metrics } from '@/src/lib/layout';
import { label } from '@/src/lib/labels';
import type { Skin } from '@/src/skins/types';

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
  /**
   * Places the cards in flight would be refused. Drawn as refusing, which is
   * not the same as unlit: an unlit target and a forbidden one looked
   * identical, and which of the two it is is the whole question a player is
   * asking mid-drag.
   */
  refusedDrops?: ReadonlySet<string>;
  /** The one the pointer is over right now. */
  hoveredDrop?: string | null;
  /**
   * Which of `hoveredDrop`'s ordered positions a card in flight currently
   * means, for a target with a choice of more than one — a run's group shows
   * this live, before a player lets go, rather than only revealing where a
   * card landed after. An index into an ordered list, not a position's name:
   * this file draws the slice, it does not need to know what either end of
   * it is called.
   */
  hoveredPosition?: { index: number; count: number } | null;
  /**
   * Element ids that may be resolved with a press rather than a drag —
   * standing in for a drag when what to send has already been chosen and only
   * where it goes is still open. Disjoint in practice from a live drag: this
   * is populated between drags, not during one.
   */
  pressableDrops?: ReadonlySet<string>;
  onPressDrop?: (elementId: string, pageY: number) => void;
  /**
   * Group ids (a meld's own id, not an element id) that could be *aimed at*
   * right now — pointed at before any card is picked, so a move can be made
   * target-first as well as cards-first. Populated only while nothing is
   * selected; see the match screen's `armableGroups`.
   */
  armableGroups?: ReadonlySet<string>;
  /** The one currently aimed at, if any. */
  armedGroupId?: string | null;
  onAimGroup?: (groupId: string) => void;
  /**
   * How long this zone's newest card should hold its entrance, keyed by the
   * zone's own element id — set while a flight is landing here, so the card
   * doesn't greet its own arrival. Absent or zero means enter at once, which
   * is exactly what happened before flights existed.
   */
  entranceDelays?: ReadonlyMap<string, number>;
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
  refusedDrops,
  hoveredDrop,
  hoveredPosition,
  pressableDrops,
  onPressDrop,
  armableGroups,
  armedGroupId,
  onAimGroup,
  entranceDelays,
}: Props) {
  const metrics = useMetrics();
  const skin = useSkin();
  const styles = useMemo(() => zoneStyles(metrics, skin), [metrics, skin]);

  const zoneLabel = label(zone.labelKey) || zone.id;
  const title = titleOverride || zoneLabel;
  const subtitle = titleOverride ? subtitleOverride ?? zoneLabel : subtitleOverride;

  const zoneId = zoneElementId(zone.id);
  const zoneLive = activeDrops?.has(zoneId) ?? false;
  const zoneRefused = refusedDrops?.has(zoneId) ?? false;
  const entranceDelay = entranceDelays?.get(zoneId) ?? 0;

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
      refused={zoneRefused}
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
            onPress={(e) => onPressDrop?.(zoneId, e.nativeEvent.pageY)}
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
            const groupRefused = refusedDrops?.has(groupId) ?? false;
            const groupPressable = pressableDrops?.has(groupId) ?? false;
            const groupArmable = armableGroups?.has(g.id) ?? false;
            const groupArmed = armedGroupId === g.id;
            const groupOpen = expandedGroups.has(g.id);
            // Only meaningful while this exact group is the one being
            // hovered — `hoveredPosition` is a fact about `hoveredDrop`, not
            // about every group on the board.
            const hoveredSlice = hoveredDrop === groupId ? hoveredPosition : null;
            return (
              <View
                key={g.id}
                ref={(n) => registerDrop?.(groupId, n as unknown as Measurable | null)}
                style={[
                  styles.group,
                  groupArmed && styles.armed,
                  groupLive && styles.live,
                  groupRefused && styles.refused,
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
                    never steals a press meant to resolve the drop.
                    While nothing is selected, the same tap also arms this
                    meld as a target for the cards picked next — the two
                    are the same gesture wanting the same thing, so aiming
                    costs no tap of its own. */}
                <Pressable
                  disabled={groupPressable}
                  onPress={() => {
                    toggleGroup(g.id);
                    if (groupArmable) onAimGroup?.(g.id);
                  }}
                  accessibilityRole="button"
                  accessibilityState={{ expanded: groupOpen }}
                  // Flat, not nested in `accessibilityState` — see `CardView`'s
                  // own `aria-selected`: this version of react-native-web drops
                  // `accessibilityState.selected` on the web build silently,
                  // where the flat aria spelling reaches the DOM.
                  aria-selected={groupArmed}
                  accessibilityLabel={groupOpen ? 'Collapse this group' : 'Show all cards in this group'}
                  testID={`group-toggle-${g.id}`}
                >
                  <View style={styles.stackedCards}>
                    {g.cards.map((c, i) => (
                      <View
                        key={`${g.id}-${c}-${i}`}
                        style={i > 0 && !groupOpen && styles.stackedOverlap}
                      >
                        {/* Keyed by card and position, so a card laid off
                            onto this group mounts fresh — and the mount is
                            the entrance. The delay only ever reaches a card
                            mounting right now, which is exactly the one a
                            flight is bringing. */}
                        <SettleIn kind="settle" delay={entranceDelay}>
                          <CardView card={c} compact stacked={!groupOpen} />
                        </SettleIn>
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
                    onPress={(e) => onPressDrop?.(groupId, e.nativeEvent.pageY)}
                  />
                ) : null}
                {/* Which of more than one place a card in flight would land
                    if let go right now — a slice of the group's own height,
                    since the group is drawn as a stack running top to bottom
                    (see the comment above). Purely a highlight: it changes
                    which slice is tinted, never the group's own size, the
                    same discipline `styles.live`/`hovered` already keep. */}
                {hoveredSlice ? (
                  <View
                    pointerEvents="none"
                    testID={`group-slice-${g.id}`}
                    style={[
                      styles.slice,
                      {
                        top: `${(100 * hoveredSlice.index) / hoveredSlice.count}%`,
                        height: `${100 / hoveredSlice.count}%`,
                      },
                    ]}
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
            // The key is the card and its place, so a new top card is a new
            // element — and a new element's mount is its entrance: the top of
            // a pile flips over as if peeled off a deck, anything else
            // settles into place.
            <SettleIn
              key={`${zone.id}-${c.card}-${buried + i}`}
              kind={zone.kind === 'pile' && buried + i === cards.length - 1 ? 'flip' : 'settle'}
              delay={buried + i === cards.length - 1 ? entranceDelay : 0}
            >
              <CardView
                card={c.card}
                faceDown={c.faceDown}
                compact={compact}
                selected={selected?.includes(c.card)}
                onPress={onPressCard && !c.faceDown ? () => onPressCard(c.card, buried + i) : undefined}
                testID={`card-${zone.id}-${buried + i}`}
              />
            </SettleIn>
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
  const skin = useSkin();
  const styles = useMemo(() => zoneStyles(metrics, skin), [metrics, skin]);
  if (count <= 0) return <Text style={styles.hidden}>empty</Text>;

  if (!skin.deckStack) {
    return (
      <View style={[styles.back, compact ? styles.backCompact : styles.backFull]}>
        <Text style={styles.backText}>{count}</Text>
      </View>
    );
  }

  // A deck rather than a box: two darkened edges peeking out under a real
  // card back, and the count on a chip. Same outer size as the flat box to
  // the pixel — the offsets spend the slack the ring arithmetic below
  // already reserves, so the zone measures identically either way.
  const w = compact ? metrics.card.compactWidth : metrics.card.width;
  const h = compact ? metrics.card.compactHeight : metrics.card.height;
  return (
    <View style={[styles.deck, compact ? styles.backCompact : styles.backFull]}>
      <View style={[styles.deckUnder, styles.deckUnderFar, { left: 6, top: 6, width: w, height: h }]} />
      <View style={[styles.deckUnder, { left: 4, top: 4, width: w, height: h }]} />
      <View style={[styles.deckUnder, styles.deckUnderNear, { left: 2, top: 2, width: w, height: h }]} />
      {/* Shadow on the top card only — the pile below it reads as solid, so
          one cast edge under the whole stack is what a real deck throws. */}
      <View style={skin.card.shadow && styles.deckTopShadow}>
        <CardBack width={w} height={h} />
      </View>
      <View style={[styles.deckCount, { width: w }]} pointerEvents="none">
        <View style={styles.deckCountPill}>
          <Text style={styles.backText}>{count}</Text>
        </View>
      </View>
    </View>
  );
}

function zoneStyles(m: Metrics, s: Skin) {
  const colors = s.colors;
  const dropArmed = s.dropArmed;
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
      // So `slice` below positions against this box and not some further-out
      // ancestor — a plain layout change, not a drop-state style, so it's
      // fine on the group at rest as well as lit up.
      position: 'relative',
    },
    // Only the border colour changes, never its width: a region that grew when
    // it lit up would move every region after it in the middle of the drag,
    // which moves the very measurements the drop is tested against. dropArmed
    // keeps to that too — it only touches colour, style and fill.
    live: { borderColor: colors.accent },
    // A place these cards may not go. Dimmed and outlined in the refusal
    // colour rather than merely left alone, and — like `live` — only colour,
    // style and opacity, never width: a region that changed size mid-drag
    // would move the measurements the drop is tested against.
    refused: { borderColor: colors.danger, borderStyle: 'dashed', opacity: 0.55 },
    hovered: dropArmed,
    // Which slice of the group a card in flight would land in, right now —
    // sized and positioned in `top`/`height` percentages set inline per
    // hover, so this only ever supplies the look. Absolutely positioned
    // inside `group` (now `position: 'relative'`), so it overlays without
    // adding to the group's own measured size.
    slice: {
      position: 'absolute',
      left: 0,
      right: 0,
      backgroundColor: 'rgba(251, 191, 36, 0.22)',
      borderRadius: 6,
    },
    // A target pointed at before any card was picked — colour only, same
    // discipline as `live`/`hovered`: this box must not change size just
    // because it was tapped.
    armed: { borderColor: colors.gold, backgroundColor: 'rgba(251, 191, 36, 0.10)' },
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
    // The deck variant of `back`: same outer box, no fill of its own — the
    // card back and its under-edges paint the inside.
    deck: {
      marginTop: 6,
      position: 'relative',
    },
    deckUnder: {
      position: 'absolute',
      borderRadius: 6,
      backgroundColor: s.card.back.colors[1],
      borderWidth: 1,
      borderColor: 'rgba(0, 0, 0, 0.4)',
      opacity: 0.8,
    },
    deckUnderNear: { opacity: 0.9 },
    // The third, deepest edge — the offsets (2, 4, 6) spend exactly the
    // slack the ring arithmetic reserves, so the box still measures the
    // same as the flat variant to the pixel.
    deckUnderFar: { opacity: 0.65 },
    // Shadow, never size — same discipline as CardView's cardShadow.
    deckTopShadow: {
      shadowColor: '#000',
      shadowOffset: { width: 0, height: 3 },
      shadowOpacity: 0.35,
      shadowRadius: 5,
      elevation: 5,
    },
    deckCount: {
      position: 'absolute',
      bottom: 5,
      left: 0,
      alignItems: 'center',
    },
    deckCountPill: {
      backgroundColor: 'rgba(0, 0, 0, 0.55)',
      paddingHorizontal: 7,
      paddingVertical: 1,
      borderRadius: 9,
    },
  });
}
