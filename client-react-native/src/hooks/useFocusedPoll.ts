import { useIsFocused } from 'expo-router';
import { useEffect, useRef, type RefObject } from 'react';

/**
 * Runs `poll` now and every `intervalMs`, but only while this screen is the
 * focused one.
 *
 * expo-router Stack screens stay mounted under the screens pushed over them.
 * The lobby's join and table screens polled their match regardless, and when
 * the host started it each one called `router.replace` to the board — from
 * underneath Rules or Settings, replacing whatever the player was actually
 * looking at. Polling stops on blur and starts again (with an immediate poll)
 * on return.
 *
 * The returned ref is true only while focused. A poll already in flight when
 * the screen loses focus must check it before navigating, since its answer
 * can arrive after the blur.
 */
export function useFocusedPoll(
  poll: () => unknown,
  intervalMs: number,
  enabled: boolean,
): RefObject<boolean> {
  const focused = useIsFocused();
  const focusedRef = useRef(focused);
  focusedRef.current = focused;

  useEffect(() => {
    if (!enabled || !focused) return undefined;
    void poll();
    const timer = setInterval(() => void poll(), intervalMs);
    return () => clearInterval(timer);
  }, [enabled, focused, poll, intervalMs]);

  return focusedRef;
}
