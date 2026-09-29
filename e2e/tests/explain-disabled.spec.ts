import { expect, test, type Page } from '@playwright/test';

import { API_BASE } from '../helpers/env';
import { waitForOfferEnabled } from '../helpers/turn';

/**
 * A control that cannot be pressed still answers a press — with why.
 *
 * Players pressed a greyed-out control, got nothing, and were left not knowing
 * what to do next. The reason line under it could be tapped, but nobody taps a
 * grey sentence when the button above it is what they wanted. This checks the
 * thing only a browser can: the button itself, marked off for assistive tech,
 * opens the sheet with the reason, the rule, and a working way out.
 */

type Ctx = import('@playwright/test').APIRequestContext;

async function zolikTableWithBot(request: Ctx) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `why-${Math.random().toString(36).slice(2, 10)}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  const host = await res.json();
  const auth = { Authorization: `Bearer ${host.accessToken}` };

  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    data: { moduleId: 'zolik', options: {} },
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
      username: 'why',
      isGuest: true,
    },
  );
  await page.goto(`/match/${matchId}`);
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
}

async function handSize(request: Ctx, matchId: string, userId: string): Promise<number> {
  const res = await request.get(`${API_BASE}/matches/${matchId}?as=${userId}`);
  expect(res.ok(), await res.text()).toBeTruthy();
  const body = await res.json();
  const zone = (body.view?.zones ?? []).find((z: any) => z.kind === 'hand' && z.ownerId === userId);
  return (zone?.cards ?? []).length;
}

test('pressing a disabled control explains it, and its remedy works', async ({ page, request }) => {
  test.setTimeout(60_000);
  const { matchId, host } = await zolikTableWithBot(request);
  await openMatch(page, host, matchId);

  // The start of this player's turn: a draw is due, so melding is refused.
  await waitForOfferEnabled(page, 'offer-draw:deck');
  const meld = page.getByTestId('offer-lay_meld');
  await expect(meld).toHaveAttribute('aria-disabled', 'true');
  const before = await handSize(request, matchId, host.userId);

  // `force`, because the point is to press a control that is marked off.
  await meld.click({ force: true });
  const sheet = page.getByTestId('why-sheet');
  await expect(sheet).toBeVisible();
  await expect(page.getByTestId('why-reason')).not.toBeEmpty();
  await expect(sheet.locator('[data-testid^="why-rule-"]').first()).toBeVisible();

  // The way out is a working control, not advice.
  await page.getByTestId('why-remedy-action').click();
  await expect(sheet).toBeHidden();
  await expect.poll(() => handSize(request, matchId, host.userId)).toBe(before + 1);
});

test('pressing a control the selection does not fit says what to pick', async ({ page, request }) => {
  test.setTimeout(60_000);
  const { matchId, host } = await zolikTableWithBot(request);
  await openMatch(page, host, matchId);

  await waitForOfferEnabled(page, 'offer-draw:deck');
  await page.getByTestId('offer-draw:deck').click();

  // Drawn, so melding is allowed — but nothing that makes a meld is picked.
  const meld = page.getByTestId('offer-lay_meld');
  await expect(meld).toHaveAttribute('aria-disabled', 'true');
  await meld.click({ force: true });
  await expect(page.getByTestId('why-sheet')).toBeVisible();
  await expect(page.getByTestId('why-reason')).not.toBeEmpty();
  await page.getByTestId('why-close').click();
  await expect(page.getByTestId('why-sheet')).toBeHidden();
});
