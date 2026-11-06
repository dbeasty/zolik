import { router } from 'expo-router';

/**
 * A screen's own Back: the screen behind it, or the home screen when there is
 * none. A page opened straight from a link, a reload or a new tab has nothing
 * on the stack, and a bare `router.back()` there does nothing at all — the
 * player presses Back and stays put. The header's ← arrow does the same
 * (src/desktop/NavArrows.tsx); this is that, for the in-page buttons.
 */
export function goBack(): void {
  if (router.canGoBack()) router.back();
  else router.replace('/');
}
