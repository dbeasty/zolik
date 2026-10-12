import { useEffect, useRef } from 'react';
import { Platform } from 'react-native';
import { Gesture, PointerType, type PanGesture } from 'react-native-gesture-handler';
import { runOnJS } from 'react-native-reanimated';

/**
 * How long a finger has to rest on a card before it is picked up.
 *
 * Long enough that a flick to scroll the board never lifts anything, short
 * enough that a person who meant to drag is not left waiting. The same order
 * as a phone's own press-and-hold.
 */
export const HOLD_TO_DRAG_MS = 300;

/**
 * How far a finger may wander during the hold and still be holding. Past it
 * the touch was a swipe, and the swipe belongs to the page.
 */
const HOLD_SLOP = 10;

export type CardPanHandlers = {
  /** Distance a mouse or pen must move before a press becomes a drag. */
  minDistance: number;
  /** Touch-down, whether or not it ever becomes a drag. */
  onBegin?: (x: number, y: number) => void;
  /** The drag is live: the card is in hand. */
  onStart: () => void;
  onUpdate: (x: number, y: number, dx: number, dy: number) => void;
  /** Always, drag or not, completed or cancelled. */
  onFinalize: () => void;
  /**
   * A finger held a card long enough to pick it up, then let go without
   * moving. On a phone that is the only long-press there is, so it is also
   * the way to ask what a card's mark means.
   */
  onHeldInPlace?: () => void;
};

/**
 * The gesture that picks a card up — from the hand or off the board.
 *
 * ## Why a finger has to hold first
 *
 * On a phone browser the board scrolls, and it scrolls under the same finger
 * that would drag a card. Gesture-handler on web can only tell the browser
 * once, at touch-down, which way it may scroll (`touch-action: pan-y`), and the
 * browser then takes every vertical swipe for itself. The board is *above* the
 * hand, so every drag aimed at a meld was a vertical swipe: it scrolled the
 * page, the browser cancelled the pointer, and the card snapped back. A phone
 * could rearrange its hand sideways and do nothing else.
 *
 * So a touch is a scroll until it has rested on a card for `HOLD_TO_DRAG_MS`.
 * Then the card lifts, and from there the finger owns it: every `touchmove`
 * for the rest of that touch is cancelled before the browser can turn it into
 * a scroll (see `holdScroll`), and the card goes wherever the finger does —
 * onto a meld, along the hand, off a meld and onto another.
 *
 * A mouse is unchanged: it never scrolls by dragging, so a press that moves
 * `minDistance` is a drag straight away, the way it always was.
 *
 * Native keeps the plain movement-threshold pan it had; this is about the
 * browser's scroll arbitration, which native does not have.
 */
export function useCardPan(handlers: CardPanHandlers): PanGesture {
  const h = useRef(handlers);
  h.current = handlers;
  // Per-touch bookkeeping, outlived by nothing: reset on every touch-down.
  const s = useRef({
    x0: 0,
    y0: 0,
    touch: false,
    active: false,
    moved: false,
    timer: null as ReturnType<typeof setTimeout> | null,
  });

  useEffect(() => holdScroll.retain(), []);
  useEffect(
    () => () => {
      if (s.current.timer) clearTimeout(s.current.timer);
      if (s.current.active) holdScroll.unlock();
    },
    [],
  );

  if (Platform.OS !== 'web') {
    // Worklets: they capture the handlers as they are this render, which is
    // why they are read off the argument and not the ref.
    const { onBegin, onStart, onUpdate, onFinalize } = handlers;
    const pan = Gesture.Pan()
      .minDistance(handlers.minDistance)
      .onBegin((e) => {
        if (onBegin) runOnJS(onBegin)(e.absoluteX, e.absoluteY);
      })
      .onStart(() => {
        runOnJS(onStart)();
      })
      .onUpdate((e) => {
        runOnJS(onUpdate)(e.absoluteX, e.absoluteY, e.translationX, e.translationY);
      })
      .onFinalize(() => {
        runOnJS(onFinalize)();
      });
    pan.config.touchAction = 'pan-y';
    return pan;
  }

  // Web: callbacks run on the JS thread here, so plain closures over refs.
  const clear = () => {
    if (s.current.timer) clearTimeout(s.current.timer);
    s.current.timer = null;
  };
  const pan = Gesture.Pan()
    .runOnJS(true)
    .manualActivation(true)
    .onBegin((e) => {
      h.current.onBegin?.(e.absoluteX, e.absoluteY);
    })
    .onTouchesDown((e, manager) => {
      const t = e.allTouches[0];
      if (!t || e.numberOfTouches > 1) return;
      clear();
      s.current = {
        ...s.current,
        x0: t.absoluteX,
        y0: t.absoluteY,
        touch: e.pointerType === PointerType.TOUCH,
        active: false,
        moved: false,
      };
      if (!s.current.touch) return;
      s.current.timer = setTimeout(() => {
        s.current.timer = null;
        holdScroll.lock();
        if (typeof navigator !== 'undefined') navigator.vibrate?.(12);
        manager.activate();
      }, HOLD_TO_DRAG_MS);
    })
    .onTouchesMove((e, manager) => {
      const t = e.allTouches[0];
      if (!t) return;
      const d = Math.hypot(t.absoluteX - s.current.x0, t.absoluteY - s.current.y0);
      if (s.current.active) {
        if (d > HOLD_SLOP) s.current.moved = true;
        return;
      }
      if (!s.current.touch) {
        if (d >= h.current.minDistance) manager.activate();
        return;
      }
      // Moved before the hold was up: a swipe, and the page's to scroll.
      if (d > HOLD_SLOP) {
        clear();
        manager.fail();
      }
    })
    .onTouchesUp(() => clear())
    .onTouchesCancelled(() => clear())
    .onStart(() => {
      s.current.active = true;
      h.current.onStart();
    })
    .onUpdate((e) => {
      h.current.onUpdate(e.absoluteX, e.absoluteY, e.translationX, e.translationY);
    })
    .onFinalize(() => {
      clear();
      const heldInPlace = s.current.active && s.current.touch && !s.current.moved;
      s.current.active = false;
      holdScroll.unlock();
      h.current.onFinalize();
      if (heldInPlace) h.current.onHeldInPlace?.();
    });
  // Vertical swipes still scroll — until a hold claims the touch.
  pan.config.touchAction = 'pan-y';
  // A long-press on a card image is otherwise "save image" on Android — the
  // very gesture that now means "pick it up". No chained setter on a Pan.
  pan.config.enableContextMenu = false;
  return pan;
}

/**
 * Keeps the page from scrolling under a card that has been picked up.
 *
 * `touch-action` cannot do it: the browser reads it once, at touch-down, when
 * nobody knew yet that this touch would become a drag. What can still stop a
 * scroll after that is cancelling the `touchmove` events themselves — but only
 * from a listener that was already registered, and registered non-passive,
 * when the touch began; one added mid-touch is too late to be asked. So one
 * listener sits on the document for as long as anything draggable is mounted,
 * and does nothing at all unless a card is in hand.
 */
const holdScroll = (() => {
  let users = 0;
  let locked = false;
  const swallow = (e: TouchEvent) => {
    if (locked && e.cancelable) e.preventDefault();
  };
  return {
    retain(): () => void {
      if (Platform.OS !== 'web' || typeof document === 'undefined') return () => {};
      if (users++ === 0) document.addEventListener('touchmove', swallow, { passive: false });
      return () => {
        if (--users === 0) {
          locked = false;
          document.removeEventListener('touchmove', swallow);
        }
      };
    },
    lock() {
      locked = true;
    },
    unlock() {
      locked = false;
    },
  };
})();
