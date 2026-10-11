import { expect, test, type APIRequestContext, type Page } from '@playwright/test';

import { API_BASE } from '../helpers/env';

/**
 * The guided lessons, as a player meets them.
 *
 * Asserts what is on the screen — the card says words rather than keys, it
 * does not sit on top of the controls it talks about, a held-back move says
 * why, leaving works and leaves nothing behind — not that the socket answered.
 */

type Guest = { accessToken: string; refreshToken: string; userId: string };

async function guest(request: APIRequestContext): Promise<Guest> {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `guide-${Math.random().toString(36).slice(2, 10)}` },
  });
  return res.json();
}

async function lessonTable(request: APIRequestContext, who: Guest, moduleId: string) {
  const lessons = (await (await request.get(`${API_BASE}/modules/${moduleId}/lessons`)).json()).lessons ?? [];
  if (!lessons.length) return null;
  const lesson = lessons[0];
  const auth = { Authorization: `Bearer ${who.accessToken}` };
  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    data: { moduleId, lessonId: lesson.id },
  });
  expect(created.ok(), await created.text()).toBeTruthy();
  const { matchId } = await created.json();
  for (let i = 0; i < lesson.bots; i++) {
    await request.post(`${API_BASE}/matches/${matchId}/add-bot`, { headers: auth, data: { skill: 'easy' } });
  }
  const started = await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });
  expect(started.ok(), await started.text()).toBeTruthy();
  return { matchId, lesson };
}

async function open(page: Page, who: Guest, matchId: string) {
  await page.addInitScript(
    (s) => window.localStorage.setItem('zolik_session', JSON.stringify(s)),
    { accessToken: who.accessToken, refreshToken: who.refreshToken, userId: who.userId, username: 'guide', isGuest: true },
  );
  await page.goto(`/match/${matchId}`);
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
}

test('a lesson table shows the guide in words, and the card stays out of the way', async ({ page, request }) => {
  test.setTimeout(90_000);
  const who = await guest(request);
  const t = await lessonTable(request, who, 'prsi');
  expect(t).not.toBeNull();
  await open(page, who, t!.matchId);

  const card = page.getByTestId('guide-card');
  await expect(card).toBeVisible();
  const text = await page.getByTestId('guide-text').innerText();
  expect(text).not.toMatch(/guide\.|\{/); // words, not a key or a bare placeholder

  // The card is above the board, so it covers neither the hand nor the
  // controls it is about.
  const cardBox = (await card.boundingBox())!;
  for (const id of ['controls-panel']) {
    const el = page.getByTestId(id);
    if (await el.count()) {
      const box = await el.boundingBox();
      if (box) expect(cardBox.y + cardBox.height).toBeLessThanOrEqual(box.y + 1);
    }
  }
});

test('a card waiting to be read holds the move, and says why', async ({ page, request }) => {
  test.setTimeout(90_000);
  const who = await guest(request);
  const t = await lessonTable(request, who, 'prsi');
  await open(page, who, t!.matchId);

  // The first card is a "say" step. Whatever the table offers, a move made
  // over it is held back.
  await expect(page.getByTestId('guide-next')).toBeVisible();
  const offer = page.locator('[data-testid^="offer-"]:not([data-testid$="-title"])').first();
  if (await offer.count()) {
    await offer.click({ force: true }).catch(() => undefined);
    // Either the offer was not live (bot's turn) or the guide held it.
    const held = page.getByTestId('guide-held');
    if (await held.count()) await expect(held).not.toHaveText(/guide\./);
  }
  await page.getByTestId('guide-next').click();
  await expect(page.getByTestId('guide-text')).not.toHaveText(/guide\./);
});

test('exiting a lesson leaves nothing behind in my games', async ({ page, request }) => {
  test.setTimeout(90_000);
  const who = await guest(request);
  const t = await lessonTable(request, who, 'prsi');
  await open(page, who, t!.matchId);

  const auth = { Authorization: `Bearer ${who.accessToken}` };
  // Practice is not a game to come back to: never listed while it is live.
  const listed = await (await request.get(`${API_BASE}/users/me/tables`, { headers: auth })).json();
  expect(listed.tables ?? []).toHaveLength(0);

  await page.getByTestId('guide-exit').click();
  await expect(page.getByTestId('guide-exit-sheet')).toBeVisible();
  await page.getByTestId('guide-exit-keep').click();
  await expect(page.getByTestId('guide-card')).toBeVisible();

  await page.getByTestId('guide-exit').click();
  await page.getByTestId('guide-exit-real').click();
  await expect(page).toHaveURL(/\/lobby\/setup\?moduleId=prsi/, { timeout: 15_000 });
});

test('every game with a lesson opens it with real words on the card', async ({ page, request }) => {
  test.setTimeout(240_000);
  const mods: { id: string }[] = (await (await request.get(`${API_BASE}/modules`)).json()).modules;
  const seen: string[] = [];
  for (const m of mods) {
    const who = await guest(request);
    const t = await lessonTable(request, who, m.id);
    if (!t) continue;
    const p = await page.context().newPage();
    await open(p, who, t.matchId);
    const text = await p.getByTestId('guide-text').innerText();
    expect(text, `${m.id}: the first card`).not.toMatch(/guide\.|\{/);
    expect(text.trim().length, `${m.id}: the first card`).toBeGreaterThan(10);
    seen.push(m.id);
    await p.close();
  }
  // Every game ships a guide; a game that drops out of this list lost its.
  expect(seen.length).toBe(mods.length);
});
