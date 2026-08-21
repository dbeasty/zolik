import { forwardRef, type ReactNode } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';
import { Gesture, GestureDetector } from 'react-native-gesture-handler';
import Animated, {
  runOnJS,
  useAnimatedStyle,
  useSharedValue,
  withSpring,
} from 'react-native-reanimated';

import { CardView } from '@/src/components/CardView';
import { useDropPulseStyle } from '@/src/hooks/useDropPulse';
import { colors, shared } from '@/src/theme';

export type StagingGroup = {
  entries: { index: number; card: string }[];
};

type Props = {
  groups: StagingGroup[];
  // True for the whole span a hand card is being dragged, so this area
  // pulses as an eligible drop target the moment the drag starts — the same
  // "you can drop it here" cue MeldTable gives table melds.
  dragActive?: boolean;
  // Paired with the hand index each card came from (not just the card
  // string) so removing the right one works even when the hand holds
  // duplicate cards, e.g. from a second physical deck.
  onRemove: (index: number) => void;
  // Reorders the cards within one staged group — positions within that
  // group's entries array, not hand indices, since that's what determines
  // the order the group is eventually sent to the server as `lay_meld`
  // (which matters for a run, where order is the whole point).
  onReorderGroup: (groupIndex: number, from: number, to: number) => void;
  onCancelGroup: (groupIndex: number) => void;
  onAddGroup: () => void;
  canAddGroup: boolean;
  onLayAll: () => void;
  canLayAll: boolean;
  layCount: number;
  // Lets the parent screen measure each group's card-row rect, so a
  // dragged hand card's drop position (not just "somewhere in the staging
  // area") can be resolved to a specific group + index within it — see
  // measureGroupRowZones in the game screen.
  onGroupRowRef?: (groupIndex: number, el: View | null) => void;
  // Which group + position within it a card being dragged is currently
  // over — drawn as a thin marker between the two cards it would land
  // between (or at an end), the same "show me where before I let go" cue
  // MeldTable's run front/end markers give table melds. Null means no drag
  // in progress, or it's not over any group right now.
  insertHover?: { groupIndex: number; pos: number } | null;
};

// Drop target for building a new meld: drag cards here (or tap them in your
// hand — either path lands in the parent's `groups` state) to stage a
// group. A hand that needs, say, a run *and* a set laid down in the same
// turn doesn't have to finish one before starting the other — "+ Add
// another run or set" opens a second box so both can be built side by
// side, each with its own Cancel. One "Lay meld" button at the bottom lays
// every group that currently has cards in it, in one tap.
//
// Always rendered at this same full shape (hint text + Cancel + Add + Lay
// meld, all present but disabled when empty) rather than collapsing to a
// thin strip when nothing is staged — a size/shape that changes based on
// content moves every drop target below it while a drag is in flight,
// which makes aiming a drop nondeterministic. Buttons disable via opacity
// instead of unmounting, the same treatment the rest of this screen's
// controls (Draw deck, Take discard, ...) already use.
export const MeldStagingArea = forwardRef<View, Props>(function MeldStagingArea(
  {
    groups,
    dragActive,
    onRemove,
    onReorderGroup,
    onCancelGroup,
    onAddGroup,
    canAddGroup,
    onLayAll,
    canLayAll,
    layCount,
    onGroupRowRef,
    insertHover,
  },
  ref,
) {
  const pulseStyle = useDropPulseStyle(!!dragActive);

  return (
    <Animated.View testID="staging-zone" ref={ref} style={[styles.box, dragActive && pulseStyle]}>
      {groups.map((group, i) => (
        <GroupBox
          key={i}
          index={i}
          showLabel={groups.length > 1}
          entries={group.entries}
          onRemove={onRemove}
          onReorder={(from, to) => onReorderGroup(i, from, to)}
          onCancel={() => onCancelGroup(i)}
          onRowRef={onGroupRowRef}
          hoverPos={insertHover?.groupIndex === i ? insertHover.pos : null}
        />
      ))}
      <Pressable
        testID="add-group-button"
        style={[shared.button, shared.buttonSecondary, styles.addButton, !canAddGroup && styles.disabled]}
        onPress={onAddGroup}
        disabled={!canAddGroup}
      >
        <Text style={shared.buttonTextSecondary}>+ Add another run or set</Text>
      </Pressable>
      <Pressable
        testID="lay-all-button"
        style={[shared.button, styles.layAllButton, !canLayAll && styles.disabled]}
        onPress={onLayAll}
        disabled={!canLayAll}
      >
        <Text style={shared.buttonText}>Lay meld{layCount > 0 ? ` (${layCount})` : ''}</Text>
      </Pressable>
    </Animated.View>
  );
});

// The card row is *always* rendered, just with placeholder content when
// empty — never conditionally mounted/unmounted — so a group's height
// never changes as cards are added or removed within it. A height change
// here shifts the hand row below it, which used to break double-tap-to-
// discard: the second tap would land on whatever card ended up under the
// finger after the reflow, not the one that was actually double-tapped.
function GroupBox({
  index,
  showLabel,
  entries,
  onRemove,
  onReorder,
  onCancel,
  onRowRef,
  hoverPos,
}: {
  index: number;
  showLabel: boolean;
  entries: { index: number; card: string }[];
  onRemove: (index: number) => void;
  onReorder: (from: number, to: number) => void;
  onCancel: () => void;
  onRowRef?: (groupIndex: number, el: View | null) => void;
  hoverPos?: number | null;
}) {
  const empty = entries.length === 0;
  // Interleaves an insert marker at hoverPos (before the entry at that
  // position, or trailing if hoverPos === entries.length) rather than
  // conditionally rendering it via a ternary — there can be a marker
  // before *and* the cards still need their own keys, so building the
  // child list explicitly is clearer than nesting fragments per-position.
  const rowChildren: ReactNode[] = [];
  if (!empty) {
    entries.forEach(({ index: handIndex, card }, position) => {
      if (hoverPos === position) {
        rowChildren.push(<View key={`marker-${position}`} style={styles.insertMarker} />);
      }
      rowChildren.push(
        <DraggableStagedCard
          key={`${card}-${handIndex}`}
          card={card}
          position={position}
          count={entries.length}
          onReorder={onReorder}
          onRemove={() => onRemove(handIndex)}
          testID={`staged-card-${index}-${position}`}
        />,
      );
    });
    if (hoverPos === entries.length) {
      rowChildren.push(<View key="marker-end" style={styles.insertMarker} />);
    }
  }
  return (
    <View style={index > 0 ? styles.groupSpacing : undefined}>
      {showLabel ? <Text style={styles.groupLabel}>Run/set {index + 1}</Text> : null}
      <View style={styles.cardRow} ref={(el) => onRowRef?.(index, el)}>
        {empty ? (
          <Text style={styles.hint}>Drag cards here — or select them below — to build a meld</Text>
        ) : (
          rowChildren
        )}
      </View>
      <Pressable
        testID={`cancel-group-${index}`}
        style={[shared.button, shared.buttonSecondary, styles.cancelButton, empty && styles.disabled]}
        onPress={onCancel}
        disabled={empty}
      >
        <Text style={shared.buttonTextSecondary}>Cancel</Text>
      </Pressable>
    </View>
  );
}

const STAGE_CARD_WIDTH = 52;
const STAGE_CARD_MARGIN = 6;
const STAGE_SLOT = STAGE_CARD_WIDTH + STAGE_CARD_MARGIN;
// Same "clear most of a slot before committing" threshold HandRow uses for
// hand reordering — small enough that an ordinary tap doesn't misfire as a
// reorder, large enough that a deliberate drag still commits well before
// the finger reaches the neighboring card's center.
const REORDER_COMMIT_RATIO = 0.75;

// A staged card within a run/set: tap removes it (back to hand), drag left
// or right reorders it within its own group. Order matters here in a way it
// doesn't in the hand — a run is laid down in the order these cards are
// sent, so being able to fix "6,5,7" into "5,6,7" without canceling and
// restarting the whole group is the point.
function DraggableStagedCard({
  card,
  position,
  count,
  onReorder,
  onRemove,
  testID,
}: {
  card: string;
  position: number;
  count: number;
  onReorder: (from: number, to: number) => void;
  onRemove: () => void;
  testID?: string;
}) {
  const dragging = useSharedValue(false);
  // Follows the finger live during the drag (translateX in on-screen
  // pixels), rather than only fading the card and waiting for release to
  // show anything — a plain opacity dim with no motion read as "did this
  // even register?" until the drop landed.
  const translateX = useSharedValue(0);

  function commit(translationX: number) {
    const slots = translationX / STAGE_SLOT;
    const deltaSlots =
      Math.abs(slots) < REORDER_COMMIT_RATIO
        ? 0
        : Math.sign(slots) * Math.round(Math.abs(slots) - (REORDER_COMMIT_RATIO - 0.5));
    const target = Math.max(0, Math.min(count - 1, position + deltaSlots));
    if (target !== position) onReorder(position, target);
  }

  const pan = Gesture.Pan()
    .minDistance(10)
    .onStart(() => {
      dragging.value = true;
    })
    .onUpdate((e) => {
      translateX.value = e.translationX;
    })
    .onEnd((e) => {
      runOnJS(commit)(e.translationX);
    })
    .onFinalize(() => {
      dragging.value = false;
      translateX.value = withSpring(0, { damping: 16, stiffness: 220 });
    });

  const tap = Gesture.Tap().onEnd((_e, success) => {
    if (success) runOnJS(onRemove)();
  });

  const gesture = Gesture.Race(pan, tap);

  const animatedStyle = useAnimatedStyle(() => ({
    opacity: dragging.value ? 0.85 : 1,
    cursor: (dragging.value ? 'grabbing' : 'grab') as string,
    transform: [
      { translateX: translateX.value },
      { translateY: dragging.value ? -6 : 0 },
      { scale: dragging.value ? 1.08 : 1 },
    ],
    zIndex: dragging.value ? 10 : 0,
    shadowColor: '#000',
    shadowOpacity: dragging.value ? 0.35 : 0,
    shadowRadius: dragging.value ? 6 : 0,
    shadowOffset: { width: 0, height: dragging.value ? 3 : 0 },
    elevation: dragging.value ? 6 : 0,
  }));

  return (
    <GestureDetector gesture={gesture}>
      <Animated.View style={[animatedStyle, { userSelect: 'none' } as object]}>
        <CardView card={card} testID={testID} />
      </Animated.View>
    </GestureDetector>
  );
}

const styles = StyleSheet.create({
  box: {
    borderWidth: 2,
    borderColor: colors.accentDim,
    borderStyle: 'dashed',
    borderRadius: 10,
    padding: 12,
    marginTop: 8,
  },
  groupSpacing: {
    marginTop: 14,
    paddingTop: 14,
    borderTopWidth: 1,
    borderTopColor: colors.border,
  },
  groupLabel: {
    color: colors.muted,
    fontSize: 11,
    fontWeight: '700',
    marginBottom: 6,
  },
  cardRow: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    alignItems: 'center',
    justifyContent: 'center',
    minHeight: 72,
  },
  hint: {
    color: colors.muted,
    fontSize: 12,
    textAlign: 'center',
  },
  insertMarker: {
    width: 3,
    borderRadius: 2,
    marginHorizontal: 2,
    alignSelf: 'stretch',
    backgroundColor: colors.gold,
  },
  cancelButton: {
    marginTop: 10,
    marginBottom: 0,
  },
  addButton: {
    marginTop: 12,
    marginBottom: 0,
  },
  layAllButton: {
    marginTop: 10,
    marginBottom: 0,
  },
  disabled: {
    opacity: 0.4,
  },
});
