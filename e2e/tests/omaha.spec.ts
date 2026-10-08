import { expect, test, type Page } from '@playwright/test';

import { API_BASE, asViewer } from '../helpers/env';

/**
 * End-to-end for Pot-Limit Omaha, Poker's third variation
 * (docs/new-games-plan.md, O1).
 *
 * `holdem/omaha_test.go` proves the rules in memory: exactly two from the hand
 * and three from the board, the pot-limit arithmetic, and a bot that plays to
 * the end at every table size. What only this can prove is the screen: that a
 * player sees four cards and a raise control whose top is the pot — named
 * "Pot", with no "All-in" button that would put in a fraction of the stack.
 */

type Ctx = import('@playwright/test').APIRequestContext;
type Session = { accessToken: string; refreshToken: string; userId: string; username?: string };

async function guest(request: Ctx): Promise<Session> {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `omaha-${Math.random().toString(36).slice(2, 10)}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

async function signIn(page: Page, s: Session) {
  await page.addInitScript((session) => {
    window.localStorage.setItem('zolik_session', JSON.stringify(session));
  }, { ...s, username: s.username ?? 'omaha', isGuest: true });
}

test.describe('pot-limit omaha', () => {
  test('a player holds four cards and can raise no further than the pot', async ({ page, request }) => {
    test.setTimeout(150_000);
    const host = await guest(request);
    const auth = { Authorization: `Bearer ${host.accessToken}` };
    const created = await request.post(`${API_BASE}/matches`, {
      headers: auth,
      data: { moduleId: 'holdem', variation: 'omaha', options: { handLimit: 30, startingStack: 1000, bigBlind: 20 } },
    });
    expect(created.ok(), await created.text()).toBeTruthy();
    const { matchId } = await created.json();
    for (let i = 0; i < 2; i++) {
      const bot = await request.post(`${API_BASE}/matches/${matchId}/add-bot`, { headers: auth });
      expect(bot.ok(), await bot.text()).toBeTruthy();
    }
    expect((await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth })).ok()).toBeTruthy();

    await signIn(page, host);
    await page.goto(`/match/${matchId}`);
    await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });

    // Four face-up cards in my hand.
    await expect
      .poll(async () => page.locator(`[data-testid^="card-hole:${host.userId}-"]`).count(), { timeout: 30_000 })
      .toBe(4);

    // Wait for a raise to come round, going on past hands the bots end.
    await expect
      .poll(
        async () => {
          if (await page.getByTestId('param-amount').isVisible()) return true;
          const next = page.locator('[data-testid="offer-continue"]:not([aria-disabled="true"])');
          if (await next.isVisible()) await next.click().catch(() => {});
          return false;
        },
        { timeout: 120_000, intervals: [500] },
      )
      .toBe(true);

    // The server's own range, read the way the screen reads it.
    const res = await request.get(`${API_BASE}/matches/${matchId}`, asViewer(host));
    const state = await res.json();
    const raise = state.legalActions.find((o: { verb: string }) => o.verb === 'raise');
    const amount = raise.params.find((p: { name: string }) => p.name === 'amount');
    const me = state.view.seats.find((s: { playerId: string }) => s.playerId === host.userId);
    const stack = Number(me.facts.find((f: { labelKey: string }) => f.labelKey === 'holdem.seat.stack').value);
    expect(amount.max, 'a pot-limit raise is capped below a 50-big-blind stack').toBeLessThan(stack);

    // The top of the range is the button named "Pot", and nothing says "All-in".
    const top = page.getByTestId(`param-amount-${amount.max}`);
    await expect(top).toBeVisible();
    await expect(top).toHaveText(/Pot/);
    await expect(page.getByTestId('param-amount')).not.toContainText('All-in');
  });
});
