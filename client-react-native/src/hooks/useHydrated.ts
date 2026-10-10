import { useEffect, useState } from 'react';
import { Platform } from 'react-native';

/**
 * What a width-dependent screen should believe about the window on its first
 * paint.
 *
 * The web build is pre-rendered at a window size of zero, so the HTML a browser
 * receives is always the narrow layout. A browser that is wider then renders
 * the wide layout during hydration, React finds the two disagree
 * ("Minified React error #418"), throws the pre-rendered tree away and rebuilds
 * it — a console error on every desktop visit, and the page drawn twice.
 *
 * Reading the real width only once mounted makes the first client render match
 * the pre-rendered one, and the layout then settles on the real width in the
 * next frame. Off the web there is no pre-render, so the real width is used at
 * once.
 */
export function useHydrated(): boolean {
  const [hydrated, setHydrated] = useState(Platform.OS !== 'web');
  useEffect(() => setHydrated(true), []);
  return hydrated;
}

/** The width-dependent answer to give: the real one once hydrated, narrow before. */
export function narrowOnFirstPaint(hydrated: boolean, narrow: boolean): boolean {
  return hydrated ? narrow : true;
}
