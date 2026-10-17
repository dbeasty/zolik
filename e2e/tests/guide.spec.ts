import AxeBuilder from '@axe-core/playwright';
import { expect, test, type APIRequestContext, type Page } from '@playwright/test';

import { API_BASE } from '../helpers/env';
import { loginAsFreshGuest } from '../helpers/login';

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

  // The first card is a "say" step: a move the table would take is held back
  // while it is up. Wait for the player's turn, so the move pressed is a live
  // one — a dead one opens the table's own "why not" sheet instead, which says
  // nothing about the guide.
  await expect(page.getByTestId('guide-next')).toBeVisible();
  await expect(page.getByTestId('turn-step')).toBeVisible({ timeout: 30_000 });
  // Draw, not Play: Play waits for a card to be picked and sends nothing.
  await page.getByTestId('offer-draw').click();
  const held = page.getByTestId('guide-held');
  await expect(held).toBeVisible();
  await expect(held).not.toHaveText(/guide\.|\{/);
  await expect(page.getByTestId('why-sheet-backdrop')).toHaveCount(0);
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

// Serious and critical axe violations, over the whole page. The felt's
// contrast is left to the skin tests, as in a11y.spec.ts.
async function axe(page: Page, what: string) {
  const result = await new AxeBuilder({ page })
    .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'])
    .disableRules(['region', 'color-contrast'])
    .analyze();
  const blocking = result.violations.filter((v) => v.impact === 'serious' || v.impact === 'critical');
  expect(
    blocking.map((v) => `${v.id} (${v.nodes.length}): ${v.nodes.slice(0, 2).map((n) => n.html.slice(0, 120)).join(' || ')}`),
    `${what}: serious/critical axe violations`,
  ).toEqual([]);
}

test('the guide card and its exit sheet have no serious accessibility violations', async ({ page, request }) => {
  test.setTimeout(90_000);
  const who = await guest(request);
  const t = await lessonTable(request, who, 'prsi');
  await open(page, who, t!.matchId);
  await expect(page.getByTestId('guide-card')).toBeVisible();
  await axe(page, 'lesson table');

  await page.getByTestId('guide-exit').click();
  await expect(page.getByTestId('guide-exit-sheet')).toBeVisible();
  await axe(page, 'guide exit sheet');
});

test('the tour walks every screen it names, in words, and can be left', async ({ page, request }) => {
  test.setTimeout(90_000);
  await loginAsFreshGuest(page, request, `tour-${Math.random().toString(36).slice(2, 8)}`);
  await page.goto('/');
  // Screens stay mounted under a stack, so read only the one on show.
  const shown = (id: string) => page.getByTestId(id).filter({ visible: true }).first();

  await shown('tour-start').click();
  const paths = [/\/$/, /\/lobby\/games/, /\/lobby\/setup/, /\/lobby\/games/, /\/more/, /\/settings/];
  const said = new Set<string>();
  for (const [i, path] of paths.entries()) {
    await expect(page).toHaveURL(path, { timeout: 15_000 });
    const text = shown('tour-text');
    await expect(text).toBeVisible();
    const words = await text.innerText();
    expect(words, `tour step ${i + 1}`).not.toMatch(/tour\.|\{/);
    said.add(words);
    if (i === 0) await axe(page, 'tour banner');
    if (i < paths.length - 1) await shown('tour-next').click();
  }
  // Six steps, six different things said: none repeats its neighbour's text.
  expect(said.size).toBe(paths.length);

  await shown('tour-exit').click();
  await expect(page.getByTestId('tour-banner').filter({ visible: true })).toHaveCount(0);
  // Left is left: a reload does not bring it back.
  await page.reload();
  await expect(page.locator('[data-testid^="skin-choice-"]').filter({ visible: true }).first()).toBeVisible({ timeout: 15_000 });
  await expect(page.getByTestId('tour-banner').filter({ visible: true })).toHaveCount(0);
});
