import { expect, test, type Page } from '@playwright/test';

import { handCards } from '../helpers/drag';
import { API_BASE, asViewer, type Viewer } from '../helpers/env';
import { selectedCodes } from '../helpers/hand';
import { waitForOfferEnabled } from '../helpers/turn';

/**
 * Asking for a hint sets up the move a strong bot would make in this seat,
 * and makes nothing: the cards are picked, the control is ringed, and the
 * player still presses it. A table that turned hints off shows no button.
 */

type Ctx = import('@playwright/test').APIRequestContext;

async function zolikTable(request: Ctx, options: Record<string, number> = {}) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `hint-${Math.random().toString(36).slice(2, 10)}` },
  });
  const host = await res.json();
  const auth = { Authorization: `Bearer ${host.accessToken}` };
  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    data: { moduleId: 'zolik', options },
  });
  expect(created.ok(), await created.text()).toBeTruthy();
  const { matchId } = await created.json();
  await request.post(`${API_BASE}/matches/${matchId}/add-bot`, { headers: auth });
  await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });
  return { matchId, host };
}

async function openMatch(page: Page, host: any, matchId: string) {
  await page.addInitScript(
    (s) => {
      window.localStorage.setItem('zolik_session', JSON.stringify(s));
    },
    {
      accessToken: host.accessToken,
      refreshToken: host.refreshToken,
      userId: host.userId,
      username: 'hint',
      isGuest: true,
    },
  );
  await page.goto(`/match/${matchId}`);
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
}

async function handSize(request: Ctx, matchId: string, viewer: Viewer): Promise<number> {
  const body = await (await request.get(`${API_BASE}/matches/${matchId}`, asViewer(viewer))).json();
  const zone = (body.view?.zones ?? []).find((z: any) => z.kind === 'hand' && z.ownerId === viewer.userId);
  return (zone?.cards ?? []).length;
}

test('a hint sets up a move without making it', async ({ page, request }) => {
  test.setTimeout(60_000);
  const { matchId, host } = await zolikTable(request);
  await openMatch(page, host, matchId);
  await handCards(page);

  // Once drawn, the hint is a move made with cards from the hand.
  await waitForOfferEnabled(page, 'offer-draw:deck');
  await page.getByTestId('offer-draw:deck').click();
  await expect.poll(() => handSize(request, matchId, host)).toBe(14);

  await page.getByTestId('hint-button').click();
  await expect(page.getByTestId('hint-line')).toBeVisible();
  await expect(page.getByTestId('hint-line')).toContainText('Suggested');
  expect((await selectedCodes(page)).length).toBeGreaterThan(0);

  // Nothing was played.
  expect(await handSize(request, matchId, host)).toBe(14);
});

test('a table without hints shows no hint button', async ({ page, request }) => {
  test.setTimeout(60_000);
  const { matchId, host } = await zolikTable(request, { hints: 0 });
  await openMatch(page, host, matchId);
  await waitForOfferEnabled(page, 'offer-draw:deck');
  await expect(page.getByTestId('turn-step')).toBeVisible();
  await expect(page.getByTestId('hint-button')).toHaveCount(0);
});
