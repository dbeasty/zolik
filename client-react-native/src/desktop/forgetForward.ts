import { IS_DESKTOP } from '@/src/config';

/**
 * Takes away what Forward would return to, in the Mac app's window history.
 *
 * `router.dismissTo` goes home by walking the window's history back, so the
 * screen it left is still ahead: after signing in, Forward showed the sign-in
 * form again. Pushing the entry the window is on drops everything after it, as
 * following a link does. Once the walk has landed, which is not at once.
 */
export function forgetForward(arrivedAt = '/', tries = 40): void {
  if (!IS_DESKTOP) return;
  if (window.location.pathname === arrivedAt) {
    window.history.pushState(window.history.state, '', window.location.href);
  } else if (tries > 0) {
    setTimeout(() => forgetForward(arrivedAt, tries - 1), 50);
  }
}
