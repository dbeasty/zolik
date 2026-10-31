import { useCallback, useState } from 'react';
import type { ViewStyle } from 'react-native';

/**
 * Keeps a scrolling board's content at the tallest it has been, so it can
 * grow but never shrink.
 *
 * On a phone the match board is taller than the window and sits scrolled to
 * its foot, by the hand. Boxes under the piles carry lines that exist only on
 * the viewer's turn ("your turn", what the table is waiting on you for), so
 * the board got shorter every time the turn passed. A shorter board clamps
 * the scroll, and everything above the box that shrank — the draw pile, the
 * discard pile — slid down and back up again on every move, which read as the
 * table flickering. With a floor under the content, the room a box gives up
 * stays at the foot of the board instead, and nothing above it moves.
 *
 * `resetKey` names what the floor was measured for: when it changes (a new
 * deal, the window turned) the board goes back to its natural height.
 */
export function useNoShrink(resetKey: string): {
  onContentSizeChange: (width: number, height: number) => void;
  style: ViewStyle | undefined;
} {
  const [floor, setFloor] = useState({ key: resetKey, height: 0 });
  const height = floor.key === resetKey ? floor.height : 0;
  const onContentSizeChange = useCallback(
    (_width: number, h: number) => {
      const next = Math.ceil(h);
      setFloor((f) => (f.key !== resetKey || next > f.height ? { key: resetKey, height: next } : f));
    },
    [resetKey],
  );
  return { onContentSizeChange, style: height > 0 ? { minHeight: height } : undefined };
}
