import { expect, test, type Page } from '@playwright/test';

import { handCards } from '../helpers/drag';
import { API_BASE } from '../helpers/env';

/**
 * No two live controls that read the same.
 *
 * A control is labelled from its verb, which is fine until a module offers the
 * same verb twice at once — Žolíky draws from the deck and from the discard
 * pile, and both said "Draw". Every offer was individually correct; the
 * collision existed only on screen, which is why nothing caught it until
 * somebody played the game.
 *
 * The Go suite holds every module to this at the source
 * (TestOffersOnScreenTogetherCanBeToldApart). This checks the half that only a
 * browser can: that what is *rendered* is distinct, labels and the facts
 * printed under them together.
 */

type Ctx = import('@playwright/test').APIRequestContext;

async function tableWithBots(request: Ctx, moduleId: string, seats = 2) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `labels-${Math.random().toString(36).slice(2, 10)}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  const host = await res.json();
  const auth = { Authorization: `Bearer ${host.accessToken}` };

  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    data: { moduleId, options: {} },
  });
  expect(created.ok(), await created.text()).toBeTruthy();
  const { matchId } = await created.json();

  for (let i = 1; i < seats; i++) {
    await request.post(`${API_BASE}/matches/${matchId}/add-bot`, { headers: auth });
  }
  await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });
  return { matchId, host };
}

async function openMatch(page: Page, host: any, matchId: string) {
  await page.addInitScript((s) => {
    window.localStorage.setItem('zolik_session', JSON.stringify(s));
  }, {
    accessToken: host.accessToken,
    refreshToken: host.refreshToken,
    userId: host.userId,
    username: 'labels',
    isGuest: true,
  });
  await page.goto(`/match/${matchId}`);
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
}

/** What every control that can be pressed right now says. */
async function liveLabels(page: Page): Promise<string[]> {
  return page
    .locator('[data-testid^="offer-"]:not([aria-disabled="true"])')
    .evaluateAll((els) => els.map((e) => (e.textContent ?? '').replace(/\s+/g, ' ').trim()));
}

test.describe('telling the controls apart', () => {
  test('the two ways to draw do not both say the same thing', async ({ page, request }) => {
    const { matchId, host } = await tableWithBots(request, 'zolik');
    await openMatch(page, host, matchId);
    await handCards(page);

    const deck = page.getByTestId('offer-draw:deck');
    const pile = page.getByTestId('offer-draw:discard');
    await expect(deck).toBeVisible();
    await expect(pile).toBeVisible();

    const deckText = ((await deck.textContent()) ?? '').trim();
    const pileText = ((await pile.textContent()) ?? '').trim();

    expect(deckText).not.toBe(pileText);
    // And the one that takes from the pile says so, rather than leaving a
    // player to find out by pressing it.
    expect(pileText.toLowerCase()).toContain('discard');
  });

  test('no two pressable controls read alike, over a played-out hand', async ({ page, request }) => {
    // Žolíky because it is the game with two draws; the collisions it does not
    // have — a Canasta meld per rank, a lay-off per meld — are held to the same
    // rule by the Go suite, on positions this could not reliably reach.
    const { matchId, host } = await tableWithBots(request, 'zolik');
    await openMatch(page, host, matchId);
    await handCards(page);

    const deadline = Date.now() + 30_000;
    let checked = 0;
    while (Date.now() < deadline && checked < 6) {
      const labels = await liveLabels(page);
      if (labels.length > 1) {
        expect(new Set(labels).size, `duplicate controls on screen: ${labels.join(' / ')}`).toBe(
          labels.length,
        );
        checked++;
      }
      const live = page.locator('[data-testid^="offer-"]:not([aria-disabled="true"])').first();
      if (await live.count()) {
        try {
          await live.click({ timeout: 5000 });
        } catch {
          /* the board moved under us; the next pass re-reads it */
        }
      }
      await page.waitForTimeout(500);
    }
    expect(checked, 'never saw two controls live at once').toBeGreaterThan(0);
  });
});
