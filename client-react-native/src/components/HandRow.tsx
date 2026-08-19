import { useRef } from 'react';
import { StyleSheet, View } from 'react-native';
import { Gesture, GestureDetector, ScrollView } from 'react-native-gesture-handler';
import Animated, {
  runOnJS,
  useAnimatedStyle,
  useSharedValue,
  type SharedValue,
} from 'react-native-reanimated';

import { CardView } from '@/src/components/CardView';
import { moveCardToIndex } from '@/src/lib/cards';

export type DropZone = { x: number; y: number; width: number; height: number };

// Shared drag-preview state, owned by the screen that renders HandRow so it
// can also render the floating overlay outside HandRow's own ScrollView
// (which would otherwise clip the dragged card instead of letting it float
// over other content like the discard pile).
export type DragPreview = {
  x: SharedValue<number>;
  y: SharedValue<number>;
  active: SharedValue<boolean>;
  draggingIndex: SharedValue<number>;
};

export function useDragPreview(): DragPreview {
  const x = useSharedValue(0);
  const y = useSharedValue(0);
  const active = useSharedValue(false);
  const draggingIndex = useSharedValue(-1);
  return { x, y, active, draggingIndex };
}

type Props = {
  cards: string[];
  selected: Set<number>;
  onToggle: (index: number) => void;
  onReorder: (newOrder: string[]) => void;
  onDoubleTap?: (index: number) => void;
  // Called on every render with the current screen-space rect of a drop
  // target (e.g. the discard pile). Returning null disables drop detection.
  getDropZone?: () => DropZone | null;
  onDropOnZone?: (index: number) => void;
  onDragCardChange?: (card: string | null) => void;
  dragPreview: DragPreview;
  compact?: boolean;
};

const CARD_WIDTH = 52;
const CARD_WIDTH_COMPACT = 44;
const CARD_MARGIN = 6;
const DOUBLE_TAP_MS = 350;

function pointInZone(x: number, y: number, zone: DropZone): boolean {
  return x >= zone.x && x <= zone.x + zone.width && y >= zone.y && y <= zone.y + zone.height;
}

export function HandRow({
  cards,
  selected,
  onToggle,
  onReorder,
  onDoubleTap,
  getDropZone,
  onDropOnZone,
  onDragCardChange,
  dragPreview,
  compact,
}: Props) {
  const slot = (compact ? CARD_WIDTH_COMPACT : CARD_WIDTH) + CARD_MARGIN;

  return (
    <ScrollView horizontal showsHorizontalScrollIndicator style={styles.row}>
      {cards.map((c, i) => (
        <DraggableCard
          key={`${c}-${i}`}
          card={c}
          index={i}
          count={cards.length}
          slot={slot}
          selected={selected.has(i)}
          compact={compact}
          onToggle={onToggle}
          onDoubleTap={onDoubleTap}
          getDropZone={getDropZone}
          onDropOnZone={onDropOnZone}
          onDragCardChange={onDragCardChange}
          dragPreview={dragPreview}
          onDrop={(from, to) => onReorder(moveCardToIndex(cards, from, to))}
        />
      ))}
      <View style={styles.spacer} />
    </ScrollView>
  );
}

function DraggableCard({
  card,
  index,
  count,
  slot,
  selected,
  compact,
  onToggle,
  onDoubleTap,
  getDropZone,
  onDropOnZone,
  onDragCardChange,
  dragPreview,
  onDrop,
}: {
  card: string;
  index: number;
  count: number;
  slot: number;
  selected: boolean;
  compact?: boolean;
  onToggle: (index: number) => void;
  onDoubleTap?: (index: number) => void;
  getDropZone?: () => DropZone | null;
  onDropOnZone?: (index: number) => void;
  onDragCardChange?: (card: string | null) => void;
  dragPreview: DragPreview;
  onDrop: (from: number, to: number) => void;
}) {
  const lastTapAt = useRef(0);

  // Plain RN Pressable (via CardView's onPress) handles taps — it's the
  // same touchable every other button in this app uses, unlike
  // gesture-handler's own Tap gesture which wasn't registering reliably.
  // Double-tap is just JS timing on top of it: two presses within the
  // window count as one discard, not two toggles.
  function handlePress() {
    const now = Date.now();
    if (now - lastTapAt.current < DOUBLE_TAP_MS) {
      lastTapAt.current = 0;
      if (onDoubleTap) onDoubleTap(index);
      return;
    }
    lastTapAt.current = now;
    onToggle(index);
  }

  function handleDragStart() {
    onDragCardChange?.(card);
  }

  function handleDragEnd(translationX: number, absoluteX: number, absoluteY: number) {
    onDragCardChange?.(null);
    const zone = getDropZone?.();
    if (zone && onDropOnZone && pointInZone(absoluteX, absoluteY, zone)) {
      onDropOnZone(index);
      return;
    }
    const deltaSlots = Math.round(translationX / slot);
    const target = Math.max(0, Math.min(count - 1, index + deltaSlots));
    if (target !== index) {
      onDrop(index, target);
    }
  }

  // minDistance means a plain tap never engages the pan, so it always falls
  // through to the Pressable underneath.
  const pan = Gesture.Pan()
    .minDistance(10)
    .onStart((e) => {
      dragPreview.draggingIndex.value = index;
      dragPreview.active.value = true;
      dragPreview.x.value = e.absoluteX;
      dragPreview.y.value = e.absoluteY;
      runOnJS(handleDragStart)();
    })
    .onUpdate((e) => {
      dragPreview.x.value = e.absoluteX;
      dragPreview.y.value = e.absoluteY;
    })
    .onEnd((e) => {
      runOnJS(handleDragEnd)(e.translationX, e.absoluteX, e.absoluteY);
    })
    .onFinalize(() => {
      dragPreview.active.value = false;
      dragPreview.draggingIndex.value = -1;
    });

  const animatedStyle = useAnimatedStyle(() => ({
    opacity: dragPreview.draggingIndex.value === index ? 0.3 : 1,
  }));

  return (
    <GestureDetector gesture={pan}>
      <Animated.View style={animatedStyle}>
        <CardView card={card} selected={selected} compact={compact} onPress={handlePress} />
      </Animated.View>
    </GestureDetector>
  );
}

const styles = StyleSheet.create({
  row: {
    flexGrow: 0,
    marginVertical: 8,
  },
  spacer: {
    width: 8,
  },
});
