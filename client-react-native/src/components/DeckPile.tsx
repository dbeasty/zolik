import { StyleSheet, Text } from 'react-native';
import { Gesture, GestureDetector } from 'react-native-gesture-handler';
import Animated, {
  runOnJS,
  useAnimatedStyle,
  useSharedValue,
  type SharedValue,
} from 'react-native-reanimated';

import type { DragPreview, DropZone } from '@/src/components/HandRow';
import { colors } from '@/src/theme';

type Props = {
  count: number;
  canDraw: boolean;
  onDraw: () => void;
  // Measures the hand row's current screen-space rect, live, so proximity
  // and the drop test always reflect the real on-screen layout rather than
  // a value cached from a scroll/layout event (same live-measurement
  // approach the rest of this screen's drag-drop already uses).
  measureHandZone: (cb: (zone: DropZone | null) => void) => void;
  dragPreview: DragPreview;
  // Vertical span, in screen coordinates, the flip progresses over: from
  // where the drag started (still face-down) down to the top of the hand
  // row (fully peeled). Recomputed at the start of every drag rather than
  // cached, so it stays correct if the layout shifts between drags.
  flip: SharedValue<number>;
};

function pointInZone(x: number, y: number, zone: DropZone): boolean {
  return x >= zone.x && x <= zone.x + zone.width && y >= zone.y && y <= zone.y + zone.height;
}

// Tap still draws immediately, same as before — this only adds an
// alternative drag-to-draw gesture on top via Gesture.Race, so whichever of
// the two resolves first (a short tap vs. a real pan) wins and the other is
// cancelled by the gesture handler itself.
export function DeckPile({ count, canDraw, onDraw, measureHandZone, dragPreview, flip }: Props) {
  const rangeStartY = useSharedValue(0);
  const rangeEndY = useSharedValue(0);

  function beginDrag(startX: number, startY: number, grabX: number, grabY: number) {
    dragPreview.active.value = true;
    dragPreview.offsetX.value = grabX;
    dragPreview.offsetY.value = grabY;
    dragPreview.x.value = startX;
    dragPreview.y.value = startY;
    flip.value = 0;
    rangeStartY.value = startY;
    measureHandZone((zone) => {
      rangeEndY.value = zone ? zone.y : startY + 200;
    });
  }

  function endDrag(absoluteX: number, absoluteY: number) {
    dragPreview.active.value = false;
    measureHandZone((zone) => {
      if (zone && pointInZone(absoluteX, absoluteY, zone)) {
        onDraw();
      }
    });
  }

  const pan = Gesture.Pan()
    .minDistance(10)
    .enabled(canDraw)
    .onStart((e) => {
      runOnJS(beginDrag)(e.absoluteX, e.absoluteY, e.x, e.y);
    })
    .onUpdate((e) => {
      dragPreview.x.value = e.absoluteX;
      dragPreview.y.value = e.absoluteY;
      const span = rangeEndY.value - rangeStartY.value;
      const progress = span > 0 ? (e.absoluteY - rangeStartY.value) / span : 0;
      flip.value = Math.max(0, Math.min(1, progress));
    })
    .onEnd((e) => {
      runOnJS(endDrag)(e.absoluteX, e.absoluteY);
    })
    .onFinalize(() => {
      dragPreview.active.value = false;
    });

  const tap = Gesture.Tap()
    .enabled(canDraw)
    .onEnd((_e, success) => {
      if (success) runOnJS(onDraw)();
    });

  const gesture = Gesture.Race(pan, tap);

  const animatedStyle = useAnimatedStyle(() => ({
    opacity: dragPreview.active.value ? 0.3 : canDraw ? 1 : 0.4,
  }));

  return (
    <GestureDetector gesture={gesture}>
      <Animated.View testID="deck-pile" style={[styles.deckBack, animatedStyle]}>
        <Text style={styles.deckBackText}>{count}</Text>
      </Animated.View>
    </GestureDetector>
  );
}

const styles = StyleSheet.create({
  deckBack: {
    width: 52,
    height: 72,
    backgroundColor: colors.accentDim,
    borderRadius: 6,
    borderWidth: 2,
    borderColor: colors.border,
    alignItems: 'center',
    justifyContent: 'center',
  },
  deckBackText: {
    color: '#fff',
    fontSize: 16,
    fontWeight: '700',
  },
});
