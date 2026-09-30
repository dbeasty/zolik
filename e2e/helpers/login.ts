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
    guestKey: guest.guestKey,
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

/**
 * A *registered* account, seeded the same way as a guest but through the
 * passwordless email flow.
 *
 * Needed by any spec about lifetime statistics: a guest deliberately has no
 * durable record — a guest name is per-device and two people can hold the same
 * one, so crediting one would merge strangers (see the server's
 * `stats.SubjectGuest`). Only a registered subject appears on a leaderboard or
 * has a `/users/me/stats` to fetch, so "signed in" is a precondition of the
 * feature rather than a convenience here.
 *
 * Uses GET /auth/dev/last-code, the dev-only stand-in for reading an inbox
 * that sign-in.spec.ts already depends on.
 */
export async function loginAsFreshAccount(
  page: Page,
  request: APIRequestContext,
  email: string,
): Promise<GuestIdentity> {
  const start = await request.post(`${API_BASE}/auth/email/start`, { data: { email } });
  if (!start.ok()) throw new Error(`email start failed: ${start.status()} ${await start.text()}`);

  const codeRes = await request.get(
    `${API_BASE}/auth/dev/last-code?email=${encodeURIComponent(email)}`,
  );
  if (!codeRes.ok()) throw new Error(`no code for ${email}: ${codeRes.status()}`);
  const { code } = await codeRes.json();

  const verify = await request.post(`${API_BASE}/auth/email/verify`, { data: { email, code } });
  if (!verify.ok()) throw new Error(`verify failed: ${verify.status()} ${await verify.text()}`);
  const body = await verify.json();
  const s = body.session ?? body;

  const session = {
    accessToken: s.accessToken,
    refreshToken: s.refreshToken,
    userId: s.userId,
    username: s.username,
    isGuest: false,
  };
  await page.addInitScript((v) => {
    window.localStorage.setItem('zolik_session', JSON.stringify(v));
  }, session);
  await seedIntroSeen(page);

  return {
    userId: session.userId,
    username: session.username,
    accessToken: session.accessToken,
    refreshToken: session.refreshToken,
  };
}
