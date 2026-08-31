import { useEffect, useRef, type ReactNode } from 'react';
import { Animated, type StyleProp, type ViewStyle } from 'react-native';

import { useReducedMotion } from '@/src/hooks/useReducedMotion';
import { ARRIVAL_EASING, ms } from '@/src/lib/motion';

/**
 * A card (or anything) arriving where it now is, with just enough motion to
 * be noticed.
 *
 * The board is server-driven: a card doesn't travel here, it is simply *in
 * the next state* somewhere it wasn't. These three entrances put the travel
 * back at the destination — the one place that knows something new appeared —
 * without any cross-board geometry to measure or get wrong:
 *
 * - 'deal'   — dealt into a fan: rises into place, staggered by `delay`.
 * - 'flip'   — turned face-up onto a pile: swings in around its vertical
 *              edge, the peel of a card coming off the deck.
 * - 'settle' — added to a group on the table: lands a touch too big and
 *              settles down, the way a placed card stops moving.
 *
 * Mount-only by design: callers key the element by what it shows (they
 * already do, for reconciliation), so "this card changed" *is* "this element
 * remounted". Transform and opacity only — layout is untouched, so nothing a
 * drag is hit-tested against ever moves because of an entrance.
 *
 * Same house rules as `useArrival`: React Native's own `Animated`, native
 * driver, still under reduce-motion, identical in a browser — where the web
 * client and its end-to-end tests actually run.
 */

type Kind = 'deal' | 'flip' | 'settle';

type Props = {
  kind?: Kind;
  /** Milliseconds to hold before entering — the deal's stagger. */
  delay?: number;
  style?: StyleProp<ViewStyle>;
  children: ReactNode;
};

export function SettleIn({ kind = 'settle', delay = 0, style, children }: Props) {
  const progress = useRef(new Animated.Value(0)).current;
  const stillness = useReducedMotion();

  useEffect(() => {
    if (stillness) {
      progress.setValue(1);
      return;
    }
    const anim = Animated.timing(progress, {
      toValue: 1,
      duration: kind === 'flip' ? ms(240) : ms(280),
      delay,
      easing: ARRIVAL_EASING,
      useNativeDriver: true,
    });
    anim.start();
    return () => anim.stop();
    // Mount-only (plus the reduce-motion answer arriving): an entrance that
    // replayed on prop changes would twitch on every re-render of the board.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [stillness]);

  const entrance =
    kind === 'flip'
      ? {
          opacity: progress.interpolate({ inputRange: [0, 0.35, 1], outputRange: [0, 1, 1] }),
          transform: [
            { perspective: 600 },
            { rotateY: progress.interpolate({ inputRange: [0, 1], outputRange: ['70deg', '0deg'] }) },
          ],
        }
      : kind === 'deal'
        ? {
            opacity: progress,
            transform: [
              { translateY: progress.interpolate({ inputRange: [0, 1], outputRange: [14, 0] }) },
              { scale: progress.interpolate({ inputRange: [0, 1], outputRange: [0.92, 1] }) },
            ],
          }
        : {
            opacity: progress,
            transform: [
              { translateY: progress.interpolate({ inputRange: [0, 1], outputRange: [-4, 0] }) },
              { scale: progress.interpolate({ inputRange: [0, 1], outputRange: [1.1, 1] }) },
            ],
          };

  return <Animated.View style={[style, entrance]}>{children}</Animated.View>;
}
