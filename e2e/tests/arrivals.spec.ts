import { expect, test } from '@playwright/test';

import { API_BASE } from '../helpers/env';
import { loginAsFreshGuest } from '../helpers/login';

/**
 * Somebody sitting down is news to the people already at the table.
 *
 * The host opens a table and waits on its screen; a friend joins from
 * elsewhere. The host is told, in a banner over whatever they are looking
 * at, without having to spot a new name in the list. A friend joining does
 * not tell the friend about themselves.
 */
test('the host waiting at a table is told when a friend sits down', async ({ page, request }) => {
  const host = await loginAsFreshGuest(page, request, `Host ${Date.now() % 10000}`);
  const created = await request.post(`${API_BASE}/matches`, {
    headers: { Authorization: `Bearer ${host.accessToken}` },
    data: { moduleId: 'prsi' },
  });
  expect(created.ok(), await created.text()).toBeTruthy();
  const { matchId } = await created.json();

  await page.goto(`/lobby/table?matchId=${matchId}`);
  await expect(page.getByTestId('table-screen')).toBeVisible({ timeout: 30_000 });
  await expect(page.getByTestId('arrival-banner')).toHaveCount(0);

  const friend = await request.post(`${API_BASE}/auth/guest`, { data: { guestName: 'Bea Arrives' } });
  const bea = await friend.json();
  const joined = await request.post(`${API_BASE}/matches/${matchId}/join`, {
    headers: { Authorization: `Bearer ${bea.accessToken}` },
  });
  expect(joined.ok(), await joined.text()).toBeTruthy();

  const banner = page.getByTestId('arrival-banner');
  await expect(banner).toBeVisible({ timeout: 10_000 });
  await expect(banner).toContainText('Bea Arrives joined the table');
  // Shown once, though both the cloud's message and the player list said it.
  await expect(banner).toHaveCount(1);
  // And it goes away by itself.
  await expect(banner).toHaveCount(0, { timeout: 10_000 });
});
