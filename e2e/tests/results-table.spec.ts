import { expect, test, type Page } from '@playwright/test';

import { API_BASE } from '../helpers/env';

/**
 * The results table, read as a reader reads it.
 *
 * Two things a table of numbers has to do before it says anything: tell the
 * reader which column is theirs, and put every cell of a column under that
 * column's heading. Both had been left to the layout engine — the head row
 * measured itself against a name and the rows below it against a number, so a
 * long name pushed one heading off the numbers it belonged to and every
 * heading after it as well — and neither is a thing a unit test on a
 * stylesheet can see. So this measures the rendered boxes.
 */

type Ctx = import('@playwright/test').APIRequestContext;

async function guest(request: Ctx) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    // A deliberately long one: the misalignment this guards against only
    // appears when a heading is wider than the numbers under it.
    data: { guestName: `results-table-reader-${Math.random().toString(36).slice(2, 8)}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

/**
 * Seats the human, fills the rest with bots, starts.
 *
 * Hold'em with a pause between rounds, because it reaches a settled table in a
 * few presses where a rummy deal takes a hundred — and the table under test
 * names no game.
 */
async function table(request: Ctx, bots = 3) {
  const host = await guest(request);
  const auth = { Authorization: `Bearer ${host.accessToken}` };
  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    data: {
      moduleId: 'holdem',
      variation: 'timed',
      options: { handLimit: 5, pauseBetweenRounds: 1 },
    },
  });
  expect(created.ok(), await created.text()).toBeTruthy();
  const { matchId } = await created.json();
  for (let i = 0; i < bots; i++) {
    const bot = await request.post(`${API_BASE}/matches/${matchId}/add-bot`, { headers: auth });
    expect(bot.ok(), await bot.text()).toBeTruthy();
  }
  const started = await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });
  expect(started.ok(), await started.text()).toBeTruthy();
  return { matchId, host };
}

async function signIn(page: Page, host: any) {
  await page.addInitScript((s) => window.localStorage.setItem('zolik_session', JSON.stringify(s)), {
    accessToken: host.accessToken,
    refreshToken: host.refreshToken,
    userId: host.userId,
    username: host.username ?? 'results-table',
    isGuest: true,
  });
}

/** Presses whatever the table offers, short of agreeing to start the next round. */
async function playUntilPaused(page: Page, request: Ctx, matchId: string, userId: string) {
  for (let i = 0; i < 160; i++) {
    const state = await (await request.get(`${API_BASE}/matches/${matchId}?as=${userId}`)).json();
    if (state.rounds?.paused) return state;
    const ids = await page
      .locator('[data-testid^="offer-"]:not([aria-disabled="true"])')
      .evaluateAll((els) => els.map((e) => e.getAttribute('data-testid') ?? '').filter(Boolean));
    const pick = ids.find((id) => !id.includes('continue'));
    if (!pick) {
      await page.waitForTimeout(400);
      continue;
    }
    try {
      await page.getByTestId(pick).click({ timeout: 4000 });
    } catch {
      // The board moved under us; the server state read above decides.
    }
    await page.waitForTimeout(250);
  }
  return null;
}

test('the results table names the reader and lines its columns up', async ({ page, request }) => {
  test.setTimeout(240_000);
  await page.setViewportSize({ width: 900, height: 900 });

  const { matchId, host } = await table(request);
  await signIn(page, host);
  await page.goto(`/match/${matchId}`);
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });

  const state = await playUntilPaused(page, request, matchId, host.userId);
  expect(state, 'a table with the pause switched on should have stopped between rounds').toBeTruthy();
  await expect(page.getByTestId('round-results')).toBeVisible({ timeout: 30_000 });

  const columns: string[] = (state.rounds.rounds.at(-1).scores as any[]).map((s) => s.playerId);

  // Every column's heading, and the boxes its cells actually occupy. The cell
  // is the parent of the element carrying the testID: react-native-web renders
  // a cell as a box with the text inside it.
  const seen = await page.evaluate(
    ({ ids, me }) => {
      const box = (el: Element | null) => {
        if (!el) return null;
        const r = (el as HTMLElement).getBoundingClientRect();
        return { left: Math.round(r.left), right: Math.round(r.right), width: Math.round(r.width) };
      };
      const cellOf = (testId: string) => {
        const el = document.querySelector(`[data-testid="${testId}"]`);
        return el ? box(el.parentElement) : null;
      };
      const panel = document.querySelector('[data-testid="round-results"]') as HTMLElement | null;
      return {
        columns: ids.map((id) => ({
          id,
          mine: id === me,
          head: (document.querySelector(`[data-testid="round-head-${id}"]`) as HTMLElement | null)
            ?.innerText,
          headBox: box(document.querySelector(`[data-testid="round-head-${id}"]`)),
          roundBox: cellOf(`round-1-${id}`),
          totalBox: cellOf(`round-total-${id}`),
        })),
        marks: (panel?.innerText.match(/\(you\)/g) ?? []).length,
      };
    },
    { ids: columns, me: host.userId },
  );

  const mine = seen.columns.find((c) => c.mine);
  expect(mine?.head, "the reader's own column should say so").toContain('(you)');
  expect(seen.marks, 'exactly one column is the reader').toBe(1);

  for (const c of seen.columns) {
    expect(c.headBox, `column ${c.id} should have a heading`).toBeTruthy();
    expect(c.roundBox, `column ${c.id} should have a round cell`).toBeTruthy();
    expect(
      [c.roundBox!.left, c.roundBox!.right],
      `column ${c.id}: its round cell sits under its own heading`,
    ).toEqual([c.headBox!.left, c.headBox!.right]);
    if (c.totalBox) {
      expect(
        [c.totalBox.left, c.totalBox.right],
        `column ${c.id}: its total sits under its own heading`,
      ).toEqual([c.headBox!.left, c.headBox!.right]);
    }
  }

  // And one width for all of them, rather than each as wide as its own name.
  const widths = new Set(seen.columns.map((c) => c.headBox!.width));
  expect([...widths], 'every column is the same width').toHaveLength(1);
});
