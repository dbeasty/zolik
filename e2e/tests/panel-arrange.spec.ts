import { expect, test, type Page } from '@playwright/test';

import { handCards } from '../helpers/drag';
import { API_BASE } from '../helpers/env';

/**
 * Moving and resizing the board's panels, on the board itself.
 *
 * Everything here is judged by where things are drawn, not by what was
 * stored: the first version of this kept a correct arrangement in storage
 * while the window it was tried in drew one column regardless, and every
 * check on the stored value passed.
 */

type Ctx = import('@playwright/test').APIRequestContext;

async function tableWithBot(request: Ctx, moduleId: string) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `arrange-${Math.random().toString(36).slice(2, 10)}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  const host = await res.json();
  const auth = { Authorization: `Bearer ${host.accessToken}` };
  const created = await request.post(`${API_BASE}/matches`, { headers: auth, data: { moduleId, options: {} } });
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
      username: 'arrange',
      isGuest: true,
    },
  );
  await page.goto(`/match/${matchId}`);
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
  await handCards(page);
}

const box = async (page: Page, testId: string) => {
  const b = await page.getByTestId(testId).boundingBox();
  expect(b, `${testId} is drawn`).toBeTruthy();
  return b!;
};

/** Pressed, held, carried in steps and let go — the way a mouse drags. */
async function drag(page: Page, testId: string, to: { x: number; y: number }) {
  const from = await box(page, testId);
  await page.mouse.move(from.x + from.width / 2, from.y + from.height / 2);
  await page.mouse.down();
  await page.mouse.move(to.x, to.y, { steps: 12 });
  await page.mouse.up();
}

test.describe('arranging the board', () => {
  test.use({ viewport: { width: 1280, height: 900 } });

  test('a panel dragged by its handle into the empty column is drawn beside the hand, and stays there', async ({
    page,
    request,
  }) => {
    const { matchId, host } = await tableWithBot(request, 'ginrummy');
    await openMatch(page, host, matchId);

    await page.getByTestId('layout-open').click();
    await expect(page.getByTestId('layout-bar')).toBeVisible();
    // Every region has a handle, and the second column is there to drop into.
    await expect(page.getByTestId('region-grip-table')).toBeVisible();
    await expect(page.getByTestId('layout-empty-column')).toBeVisible();

    const empty = await box(page, 'layout-empty-column');
    await drag(page, 'region-grip-table', { x: empty.x + empty.width / 2, y: empty.y + 40 });
    await page.getByTestId('layout-done').click();
    await expect(page.getByTestId('layout-bar')).toHaveCount(0);

    // Side by side: the table starts to the right of where the hand ends.
    const table = await box(page, 'region-table');
    const hand = await box(page, 'region-hand');
    expect(table.x).toBeGreaterThanOrEqual(hand.x + hand.width - 1);

    await page.reload();
    await handCards(page);
    const tableAgain = await box(page, 'region-table');
    const handAgain = await box(page, 'region-hand');
    expect(tableAgain.x).toBeGreaterThanOrEqual(handAgain.x + handAgain.width - 1);
  });

  test('the bar between the columns resizes them, and the width is kept', async ({ page, request }) => {
    const { matchId, host } = await tableWithBot(request, 'ginrummy');
    await openMatch(page, host, matchId);

    await page.getByTestId('layout-open').click();
    await page.getByTestId('layout-preset-beside').click();
    await page.getByTestId('layout-done').click();

    const before = await box(page, 'board-column-0');
    const row = await box(page, 'board-columns');
    // Half and half to start with.
    expect(Math.abs(before.width - (row.width - 16) / 2)).toBeLessThan(4);

    const divider = await box(page, 'board-divider');
    await drag(page, 'board-divider', { x: row.x + row.width * 0.4, y: divider.y + 100 });
    const after = await box(page, 'board-column-0');
    expect(after.width).toBeLessThan(before.width - 60);
    expect(Math.abs(after.width / (row.width - 16) - 0.4)).toBeLessThan(0.03);

    await page.reload();
    await handCards(page);
    const kept = await box(page, 'board-column-0');
    expect(Math.abs(kept.width - after.width)).toBeLessThan(4);

    // The keyboard too: a step right is a wider left column.
    await page.getByTestId('board-divider').focus();
    await page.keyboard.press('ArrowRight');
    await expect
      .poll(async () => (await box(page, 'board-column-0')).width)
      .toBeGreaterThan(kept.width + 20);
  });

  test('a window too narrow for two columns says so, and the arrows reorder the one it draws', async ({
    page,
    request,
  }) => {
    await page.setViewportSize({ width: 820, height: 900 });
    const { matchId, host } = await tableWithBot(request, 'ginrummy');
    await openMatch(page, host, matchId);

    await page.getByTestId('layout-open').click();
    await expect(page.getByTestId('layout-hint')).toContainText(/wider window/i);
    // No column to the side to move to, so no arrow offers to.
    await expect(page.getByTestId('layout-move-hand-right')).toHaveCount(0);

    const tableY = (await box(page, 'region-table')).y;
    expect((await box(page, 'region-hand')).y).toBeGreaterThan(tableY);
    // Up past the moves panel, then past the table.
    await page.getByTestId('layout-move-hand-up').click();
    await page.getByTestId('layout-move-hand-up').click();
    await expect.poll(async () => (await box(page, 'region-hand')).y).toBeLessThan((await box(page, 'region-table')).y);
  });
});
