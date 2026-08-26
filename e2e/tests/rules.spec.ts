import { expect, test, type Page } from '@playwright/test';

import { API_BASE } from '../helpers/env';

/**
 * Written rules, reflecting the table actually being looked at.
 *
 * `GET /modules/{id}/rules` resolves a module's rules against a variation and
 * option overrides (internal/module/rules.go, one implementation per game) —
 * the point being that the sentences on screen change when the pills do, not
 * just the descriptor's static blurb. This drives that from both places a
 * player can reach the rules screen: the picker, where the pills live, and a
 * running match, where the table's own settings do.
 */

type Ctx = import('@playwright/test').APIRequestContext;

async function guest(request: Ctx) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `rules-${Math.random().toString(36).slice(2, 10)}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

async function signIn(page: Page, host: { accessToken: string; refreshToken: string; userId: string; username?: string }) {
  await page.addInitScript((s) => {
    window.localStorage.setItem(
      'zolik_session',
      JSON.stringify({ ...s, isGuest: true, username: s.username ?? 'e2e' }),
    );
  }, host);
}

test.describe('the game picker rules link', () => {
  test('reflects the meld-floor option and the selected variation', async ({ page }) => {
    await page.goto('/lobby/games');
    await expect(page.getByTestId('games-list')).toBeVisible({ timeout: 20_000 });

    // Default: zolik_classic, meld floor off.
    await page.getByTestId('rules-zolik').click();
    await expect(page.getByTestId('rules-screen')).toBeVisible({ timeout: 20_000 });
    await expect(page.getByTestId('rules-screen')).toContainText('no minimum point value');

    // Back to the picker, turn the meld floor on, and check the sentence
    // that appears is the *option's* sentence, not the descriptor's default.
    await page.goto('/lobby/games');
    await expect(page.getByTestId('games-list')).toBeVisible({ timeout: 20_000 });
    await page.getByTestId('option-zolik-initialMeldMinimum-50').click();
    await page.getByTestId('rules-zolik').click();
    await expect(page.getByTestId('rules-screen')).toBeVisible({ timeout: 20_000 });
    await expect(page.getByTestId('rules-screen')).toContainText('at least 50 natural points');

    // Back to the picker, switch variation, and check the rotating-contract
    // sentence that only Continental has.
    await page.goto('/lobby/games');
    await expect(page.getByTestId('games-list')).toBeVisible({ timeout: 20_000 });
    await page.getByTestId('variation-zolik-continental').click();
    await page.getByTestId('rules-zolik').click();
    await expect(page.getByTestId('rules-screen')).toBeVisible({ timeout: 20_000 });
    await expect(page.getByTestId('rules-screen')).toContainText('each deal requires its own combination');
  });
});

test.describe('the in-match rules link', () => {
  test('opens the rules screen for the table actually being played', async ({ page, request }) => {
    const host = await guest(request);
    const auth = { Authorization: `Bearer ${host.accessToken}` };

    const created = await request.post(`${API_BASE}/matches`, {
      headers: auth,
      data: { moduleId: 'zolik', variation: 'zolik_classic', options: {} },
    });
    expect(created.ok(), await created.text()).toBeTruthy();
    const { matchId } = await created.json();

    const bot = await request.post(`${API_BASE}/matches/${matchId}/add-bot`, { headers: auth });
    expect(bot.ok(), await bot.text()).toBeTruthy();
    const started = await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });
    expect(started.ok(), await started.text()).toBeTruthy();

    await signIn(page, host);
    await page.goto(`/match/${matchId}`);
    await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 20_000 });

    await page.getByTestId('match-rules').click();
    await expect(page.getByTestId('rules-screen')).toBeVisible({ timeout: 20_000 });
    await expect(page.getByTestId('rules-section-0')).toBeVisible();
  });
});
