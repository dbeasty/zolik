import { APIRequestContext, Page } from '@playwright/test';

import { API_BASE } from './env';
import type { SeededGame } from './seed';

/**
 * Marks this device as having already seen the first-run intro screen (see
 * client-react-native's `src/lib/introStore.ts` — key 'zolik_seen_intro'),
 * so a test that navigates to `/` lands on the main menu instead of the
 * one-time intro. Called by `loginAs`/`loginAsFreshGuest` below, and by any
 * spec that visits `/` without going through either — see
 * `openInGerman`-style seeding in `localisation.spec.ts` for the same idea
 * applied to locale.
 */
export async function seedIntroSeen(page: Page) {
  await page.addInitScript(() => {
    window.localStorage.setItem('zolik_seen_intro', '1');
  });
}

// Seeds the web app's localStorage session (see client-react-native's
// SessionContext — key 'zolik_session', shape matches PlayerSession) before
// any page script runs, so SessionProvider's bootstrap effect finds an
// already-logged-in guest instead of bouncing to the login screen. Avoids
// driving the actual guest-login UI flow for every single test.
export async function loginAs(page: Page, game: SeededGame) {
  const session = {
    accessToken: game.token,
    refreshToken: game.refreshToken,
    userId: game.userId,
    username: game.username,
    isGuest: true,
  };
  await page.addInitScript((s) => {
    window.localStorage.setItem('zolik_session', JSON.stringify(s));
  }, session);
  await seedIntroSeen(page);
}

export type GuestIdentity = {
  userId: string;
  username: string;
  accessToken: string;
  refreshToken: string;
};

/**
 * Guest login with no game attached — for specs about the waiting room and
 * lobby invites, where two independent *people* matter more than any one
 * game. Drives the real /auth/guest endpoint (not a shortcut) so the
 * session this seeds is exactly what the app itself would have produced,
 * guestId included — the field the waiting room and the invite flow are
 * actually built on.
 */
export async function loginAsFreshGuest(
  page: Page,
  request: APIRequestContext,
  guestName: string,
): Promise<GuestIdentity> {
  const res = await request.post(`${API_BASE}/auth/guest`, { data: { guestName } });
  if (!res.ok()) throw new Error(`guest login failed: ${res.status()} ${await res.text()}`);
  const guest = await res.json();

  const session = {
    accessToken: guest.accessToken,
    refreshToken: guest.refreshToken,
    userId: guest.userId,
    username: guest.guestName ?? guestName,
    isGuest: true,
    guestId: guest.guestId,
    claimableMatches: guest.claimableMatches ?? 0,
  };
  await page.addInitScript((s) => {
    window.localStorage.setItem('zolik_session', JSON.stringify(s));
  }, session);
  await seedIntroSeen(page);

  return {
    userId: guest.userId,
    username: session.username,
    accessToken: guest.accessToken,
    refreshToken: guest.refreshToken,
  };
}
