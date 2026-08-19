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

type Props = {
  cards: string[];
  selected: Set<number>;
  onToggle: (index: number) => void;
  onReorder: (newOrder: string[]) => void;
  onDoubleTap?: (index: number) => void;
  compact?: boolean;
};

const CARD_WIDTH = 52;
const CARD_WIDTH_COMPACT = 44;
const CARD_MARGIN = 6;

export function HandRow({ cards, selected, onToggle, onReorder, onDoubleTap, compact }: Props) {
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
  onDrop: (from: number, to: number) => void;
}) {
  const translateX = useSharedValue(0);
  const dragging = useSharedValue(false);

  // Double-tap discards the card directly. It's tried first so a genuine
  // double-tap doesn't also fire as two single taps; Exclusive falls back
  // to the single tap (select) when only one tap happens.
  const doubleTap = Gesture.Tap()
    .numberOfTaps(2)
    .onEnd((_e, success) => {
      if (success && onDoubleTap) runOnJS(onDoubleTap)(index);
    });

  // A tap (no meaningful movement) toggles selection for meld/discard
  // actions. A drag past the distance threshold reorders the hand instead —
  // Gesture.Race lets whichever one actually happens win, no mode switch.
  const singleTap = Gesture.Tap().onEnd((_e, success) => {
    if (success) runOnJS(onToggle)(index);
  });

  const taps = onDoubleTap ? Gesture.Exclusive(doubleTap, singleTap) : singleTap;

  const pan = Gesture.Pan()
    .minDistance(10)
    .onStart(() => {
      dragging.value = true;
    })
    .onUpdate((e) => {
      translateX.value = e.translationX;
    })
    .onEnd((e) => {
      const deltaSlots = Math.round(e.translationX / slot);
      const target = Math.max(0, Math.min(count - 1, index + deltaSlots));
      if (target !== index) {
        runOnJS(onDrop)(index, target);
      }
    })
    .onFinalize(() => {
      translateX.value = withSpring(0, { damping: 20, stiffness: 300 });
      dragging.value = false;
    });

  const gesture = Gesture.Race(pan, taps);

  const animatedStyle = useAnimatedStyle(() => ({
    transform: [{ translateX: translateX.value }],
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
