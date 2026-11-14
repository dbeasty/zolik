import { useIsFocused } from 'expo-router';
import { useEffect, useRef, type RefObject } from 'react';
import { Platform } from 'react-native';

/**
 * Listens for keydown on the web document, but only while this screen is the
 * focused one.
 *
 * expo-router Stack screens stay mounted under the screens pushed over them,
 * so a plain `document` listener keeps answering keys typed on Rules or
 * Settings. On the match screen that meant Enter sent the lead move, D drew,
 * T took the discard and U undid — all from a screen that showed none of it.
 *
 * The handler is read through a ref so callers can reassign it every render
 * without re-subscribing. Focus is checked both by attaching only while
 * focused and again inside the listener, since a blur and a keydown can land
 * in the same frame before the effect cleanup runs.
 */
export function useFocusedKeydown(handler: RefObject<(e: KeyboardEvent) => void>) {
  const focused = useIsFocused();
  const focusedRef = useRef(focused);
  focusedRef.current = focused;

  useEffect(() => {
    if (!focused || Platform.OS !== 'web' || typeof document === 'undefined') return undefined;
    const listener = (e: KeyboardEvent) => {
      if (!focusedRef.current) return;
      handler.current?.(e);
    };
    document.addEventListener('keydown', listener);
    return () => document.removeEventListener('keydown', listener);
  }, [focused, handler]);
}
