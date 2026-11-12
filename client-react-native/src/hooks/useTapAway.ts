import { useRef } from 'react';
import type { GestureResponderEvent, ViewProps } from 'react-native';

/**
 * How far a press may wander and still count as a tap. Past it, the press
 * was a scroll or a drag, and neither is a request to put the cards down.
 */
export const TAP_SLOP = 10;

type ResponderProps = Pick<
  ViewProps,
  | 'onStartShouldSetResponder'
  | 'onResponderGrant'
  | 'onResponderRelease'
  | 'onResponderTerminate'
  | 'onResponderTerminationRequest'
>;

/** Whether a press that went down at `from` and came up at `to` was a tap. */
export function isTap(from: { x: number; y: number }, to: { x: number; y: number }) {
  return Math.hypot(to.x - from.x, to.y - from.y) <= TAP_SLOP;
}

/**
 * Responder props for a surface whose bare taps mean "never mind": spread on
 * a View, a tap that lands on it and nowhere more particular calls `onTap`.
 *
 * "Nowhere more particular" is the responder system's own rule: a press is
 * offered to the deepest view first, so every button, card and drop target
 * inside answers its own taps and this one only hears what nothing else
 * wanted — the felt between panels, a heading, an empty margin. A region
 * that should keep its near misses (the hand, the action bar) spreads
 * `swallowTaps` so they stop there.
 *
 * Off when there is nothing to undo, so an idle board claims no presses.
 */
export function useTapAway(onTap: () => void, enabled: boolean): ResponderProps {
  const start = useRef<{ x: number; y: number } | null>(null);
  if (!enabled) return {};
  const at = (e: GestureResponderEvent) => ({ x: e.nativeEvent.pageX, y: e.nativeEvent.pageY });
  return {
    onStartShouldSetResponder: () => true,
    onResponderGrant: (e) => {
      start.current = at(e);
    },
    onResponderRelease: (e) => {
      const from = start.current;
      start.current = null;
      if (from && isTap(from, at(e))) onTap();
    },
    // A scroll that takes the press over was never a tap.
    onResponderTerminationRequest: () => true,
    onResponderTerminate: () => {
      start.current = null;
    },
  };
}

/**
 * Keeps a tap on this region's empty space from reaching a `useTapAway`
 * surface behind it. A tap between two cards in the hand, or just beside a
 * button, is a near miss — not a decision to drop the selection.
 */
export const swallowTaps: ResponderProps = { onStartShouldSetResponder: () => true };
