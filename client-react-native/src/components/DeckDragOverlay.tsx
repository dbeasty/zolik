import { StyleSheet, Text } from 'react-native';
import Animated, { interpolate, useAnimatedStyle, type SharedValue } from 'react-native-reanimated';

import type { DragPreview } from '@/src/components/HandRow';
import { colors } from '@/src/theme';

type Props = {
  dragPreview: DragPreview;
  // Where within the root component to position the overlay (see
  // measureOverlayOrigin in the game screen) — same trick the hand-drag
  // overlay already uses to stay glued to the finger regardless of header
  // height or safe-area insets.
  originX: SharedValue<number>;
  originY: SharedValue<number>;
  flip: SharedValue<number>;
};

// Floats the card being dragged off the deck above the whole screen and
// flips it face-up as it nears the hand, via the classic two-face rotateY +
// backfaceVisibility technique, so the reveal reads as a physical "peel"
// rather than a fade/swap. The front face shows a plain checkmark, not the
// drawn card's value — the player doesn't actually know what they drew
// until the server responds to draw_card, so revealing any specific rank/
// suit here would be showing them a card they don't have yet.
export function DeckDragOverlay({ dragPreview, originX, originY, flip }: Props) {
  const wrapStyle = useAnimatedStyle(() => ({
    position: 'absolute',
    left: dragPreview.x.value - dragPreview.offsetX.value - originX.value,
    top: dragPreview.y.value - dragPreview.offsetY.value - originY.value,
    opacity: dragPreview.active.value ? 1 : 0,
    zIndex: 1000,
    elevation: 1000,
  }));

  const backStyle = useAnimatedStyle(() => ({
    transform: [{ perspective: 800 }, { rotateY: `${interpolate(flip.value, [0, 1], [0, 180])}deg` }],
  }));

  const frontStyle = useAnimatedStyle(() => ({
    transform: [{ perspective: 800 }, { rotateY: `${interpolate(flip.value, [0, 1], [180, 360])}deg` }],
  }));

  return (
    <Animated.View style={wrapStyle} pointerEvents="none">
      <Animated.View style={[styles.face, backStyle]}>
        <Text style={styles.backText}>🂠</Text>
      </Animated.View>
      <Animated.View style={[styles.face, styles.front, frontStyle]}>
        <Text style={styles.frontText}>✓</Text>
      </Animated.View>
    </Animated.View>
  );
}

const styles = StyleSheet.create({
  face: {
    width: 52,
    height: 72,
    borderRadius: 6,
    borderWidth: 2,
    alignItems: 'center',
    justifyContent: 'center',
    backfaceVisibility: 'hidden',
    backgroundColor: colors.accentDim,
    borderColor: colors.border,
  },
  front: {
    position: 'absolute',
    top: 0,
    left: 0,
    backgroundColor: 'rgba(74, 222, 128, 0.15)',
    borderColor: colors.success,
  },
  backText: {
    fontSize: 28,
    color: '#fff',
  },
  frontText: {
    fontSize: 22,
    fontWeight: '700',
    color: colors.success,
  },
});
