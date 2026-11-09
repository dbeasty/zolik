import { expect, test, type APIRequestContext, type Page } from '@playwright/test';

import { API_BASE } from '../helpers/env';
import { seedIntroSeen } from '../helpers/login';

/**
 * Things a person reads off the table, each of which once read wrong:
 * a game called by its internal id, a bot's name cut to five letters on a
 * phone, a spectator told about "your hand", a bad link that never stopped
 * connecting.
 */

async function guest(request: APIRequestContext) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `said-${Math.random().toString(36).slice(2, 8)}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

async function signIn(page: Page, g: any) {
  await page.addInitScript((s) => window.localStorage.setItem('zolik_session', JSON.stringify(s)), {
    accessToken: g.accessToken,
    refreshToken: g.refreshToken,
    userId: g.userId,
    username: g.guestName,
    isGuest: true,
    guestId: g.guestId,
    guestKey: g.guestKey,
  });
  await seedIntroSeen(page);
}

async function startedTable(request: APIRequestContext, host: any, moduleId: string, bots: number) {
  const auth = { Authorization: `Bearer ${host.accessToken}` };
  const created = await request.post(`${API_BASE}/matches`, { headers: auth, data: { moduleId, options: {} } });
  expect(created.ok(), await created.text()).toBeTruthy();
  const { matchId } = await created.json();
  for (let i = 0; i < bots; i++) {
    const bot = await request.post(`${API_BASE}/matches/${matchId}/add-bot`, { headers: auth });
    expect(bot.ok(), await bot.text()).toBeTruthy();
  }
  const started = await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });
  expect(started.ok(), await started.text()).toBeTruthy();
  return matchId as string;
}

test('an open table names its game in words, not by its id', async ({ page, request }) => {
  const host = await guest(request);
  await signIn(page, host);
  const created = await request.post(`${API_BASE}/matches`, {
    headers: { Authorization: `Bearer ${host.accessToken}` },
    data: { moduleId: 'marias', variation: 'licitovany', options: {} },
  });
  const { matchId } = await created.json();
  await page.goto(`/lobby/table?matchId=${matchId}`);
  const line = page.getByTestId('table-module');
  await expect(line).toHaveText(/Mariáš/, { timeout: 30_000 });
  await expect(line).not.toHaveText(/marias|licitovany/);
});

test("a bot's name is readable on a phone, whole", async ({ page, request }) => {
  await page.setViewportSize({ width: 393, height: 852 });
  const host = await guest(request);
  await signIn(page, host);
  const matchId = await startedTable(request, host, 'prsi', 3);
  await page.goto(`/match/${matchId}`);
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
  const seats = page.locator('[data-testid^="seat-bot:"]');
  await expect(seats).toHaveCount(3, { timeout: 15_000 });

  // Every line of text in a bot's tile is drawn whole: nothing is ellipsized.
  // The names are the longest thing on the tile and were being cut to "Stead…".
  const cut = await seats.evaluateAll((tiles) =>
    tiles.flatMap((tile) =>
      [...tile.querySelectorAll<HTMLElement>('div[dir="auto"], span')]
        .filter(
          (el) =>
            el.children.length === 0 &&
            getComputedStyle(el).textOverflow === 'ellipsis' &&
            el.scrollWidth > el.clientWidth + 1,
        )
        .map((el) => `${el.textContent} (${el.clientWidth} of ${el.scrollWidth})`),
    ),
  );
  expect(cut).toEqual([]);
});

test('a spectator is not shown a hand or controls of their own', async ({ browser, request }) => {
  const host = await guest(request);
  const matchId = await startedTable(request, host, 'prsi', 1);

  const seated = await browser.newContext();
  const seatedPage = await seated.newPage();
  await signIn(seatedPage, host);
  await seatedPage.goto(`/match/${matchId}`);
  await expect(seatedPage.getByTestId('action-bar')).toBeVisible({ timeout: 30_000 });
  await expect(seatedPage.locator(`[data-testid="zone-hand:${host.userId}"]`)).toBeVisible();

  const watching = await browser.newContext();
  const page = await watching.newPage();
  await signIn(page, await guest(request));
  await page.goto(`/match/${matchId}`);
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
  // The table itself is there to watch...
  await expect(page.getByTestId('seat-strip')).toBeVisible({ timeout: 15_000 });
  await expect(page.getByTestId('zone-discard')).toBeVisible();
  // ...and nothing talks to them as if they were playing.
  await expect(page.getByTestId('action-bar')).toHaveCount(0);
  await expect(page.getByText('Your hand', { exact: true })).toHaveCount(0);

  await seated.close();
  await watching.close();
});

test('a cut-off match link says the table is gone and offers a way out', async ({ page, request }) => {
  await signIn(page, await guest(request));
  await page.goto('/match/6aa0e923');
  await expect(page.getByTestId('match-gone')).toHaveText('That table no longer exists', { timeout: 30_000 });
  await expect(page.getByTestId('match-connecting')).toBeHidden();
  await page.getByTestId('match-gone-leave').click();
  await expect(page.getByTestId('games-list')).toBeVisible({ timeout: 15_000 });
});

test("the hand's title and its controls never draw over each other on a phone", async ({ page, request }) => {
  await page.setViewportSize({ width: 393, height: 852 });
  const host = await guest(request);
  await signIn(page, host);
  const matchId = await startedTable(request, host, 'canasta', 3);
  // Bulgarian: the longest words for both of them. "Your hand" and
  // "Auto-arrange" overlapped in 11 of the 24 languages.
  await page.addInitScript(() => window.localStorage.setItem('zolik_locale', 'bg'));
  await page.goto(`/match/${matchId}`);
  const box = page.locator(`[data-testid="hand-auto-arrange-hand:${host.userId}"]`);
  await expect(box).toBeVisible({ timeout: 30_000 });
  const gap = await box.evaluate((el) => {
    const panel = el.closest('[data-testid^="zone-hand"]')!;
    const title = panel.querySelector('div[dir="auto"]')!.getBoundingClientRect();
    const r = el.getBoundingClientRect();
    const sameLine = r.top < title.bottom && title.top < r.bottom;
    return { title: panel.querySelector('div[dir="auto"]')!.textContent, sameLine, clear: r.left - title.right };
  });
  expect(gap.title).toBe('Твоята ръка');
  if (gap.sameLine) expect(gap.clear).toBeGreaterThanOrEqual(0);
});
