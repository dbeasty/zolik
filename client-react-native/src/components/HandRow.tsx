import { useRef } from 'react';
import { StyleSheet, View } from 'react-native';
import { Gesture, GestureDetector, ScrollView } from 'react-native-gesture-handler';
import Animated, {
  runOnJS,
  useAnimatedStyle,
  useSharedValue,
  withSpring,
} from 'react-native-reanimated';

import { CardView } from '@/src/components/CardView';
import { moveCardToIndex } from '@/src/lib/cards';

export type DropZone = { x: number; y: number; width: number; height: number };

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
  compact?: boolean;
};

const CARD_WIDTH = 52;
const CARD_WIDTH_COMPACT = 44;
const CARD_MARGIN = 6;
const DOUBLE_TAP_MS = 300;

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
  onDrop: (from: number, to: number) => void;
}) {
  const translateX = useSharedValue(0);
  const translateY = useSharedValue(0);
  const dragging = useSharedValue(false);
  const lastTapAt = useRef(0);

  // Plain JS double-tap detection layered on top of the single tap that
  // already works for selection — avoids depending on gesture-handler's
  // built-in multi-tap recognizer.
  function handleTap() {
    const now = Date.now();
    if (now - lastTapAt.current < DOUBLE_TAP_MS) {
      lastTapAt.current = 0;
      if (onDoubleTap) onDoubleTap(index);
      return;
    }
    lastTapAt.current = now;
    onToggle(index);
  }

  const tap = Gesture.Tap().onEnd((_e, success) => {
    if (success) runOnJS(handleTap)();
  });

  function handleDragEnd(translationX: number, absoluteX: number, absoluteY: number) {
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

  const pan = Gesture.Pan()
    .minDistance(10)
    .onStart(() => {
      dragging.value = true;
    })
    .onUpdate((e) => {
      translateX.value = e.translationX;
      translateY.value = e.translationY;
    })
    .onEnd((e) => {
      runOnJS(handleDragEnd)(e.translationX, e.absoluteX, e.absoluteY);
    })
    .onFinalize(() => {
      translateX.value = withSpring(0, { damping: 20, stiffness: 300 });
      translateY.value = withSpring(0, { damping: 20, stiffness: 300 });
      dragging.value = false;
    });

  const gesture = Gesture.Race(pan, tap);

  const animatedStyle = useAnimatedStyle(() => ({
    transform: [{ translateX: translateX.value }, { translateY: translateY.value }],
    zIndex: dragging.value ? 10 : 0,
    opacity: dragging.value ? 0.9 : 1,
  }));

  return (
    <GestureDetector gesture={gesture}>
      <Animated.View style={animatedStyle}>
        <CardView card={card} selected={selected} compact={compact} />
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
