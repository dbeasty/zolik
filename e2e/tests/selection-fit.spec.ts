import { expect, test, type Page } from '@playwright/test';

import { handCards } from '../helpers/drag';
import { API_BASE } from '../helpers/env';
import { selectedCodes } from '../helpers/hand';
import { waitForOfferEnabled } from '../helpers/turn';

/**
 * A control refuses a selection it cannot send, rather than sending a
 * trimmed or guessed version of it.
 *
 * The bug this guards against: a discard takes exactly one card, and picking
 * two used to leave the button enabled — pressing it discarded whichever card
 * a `slice` happened to keep, silently, while the other stayed selected
 * looking chosen when it had already been spent. `src/lib/drops.ts`'s `fits`
 * is the fix; this is the one thing only a browser can check, that the
 * *rendered* control actually goes dark and says why.
 */

type Ctx = import('@playwright/test').APIRequestContext;

async function tableWithBots(request: Ctx, moduleId: string, seats = 2) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `fit-${Math.random().toString(36).slice(2, 10)}` },
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
  await page.addInitScript(
    (s) => {
      window.localStorage.setItem('zolik_session', JSON.stringify(s));
    },
    {
      accessToken: host.accessToken,
      refreshToken: host.refreshToken,
      userId: host.userId,
      username: 'fit',
      isGuest: true,
    },
  );
  await page.goto(`/match/${matchId}`);
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
}

async function board(request: Ctx, matchId: string, userId: string) {
  const res = await request.get(`${API_BASE}/matches/${matchId}?as=${userId}`);
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

async function serverHand(request: Ctx, matchId: string, userId: string): Promise<string[]> {
  const body = await board(request, matchId, userId);
  const zone = (body.view?.zones ?? []).find((z: any) => z.kind === 'hand' && z.ownerId === userId);
  return (zone?.cards ?? []).map((c: any) => c.card);
}

test.describe('a control refuses what it cannot send', () => {
  test('discard greys out and says why when two cards are picked, and comes back for one', async ({
    page,
    request,
  }) => {
    test.setTimeout(60_000);
    const { matchId, host } = await tableWithBots(request, 'zolik');
    await openMatch(page, host, matchId);
    await handCards(page);

    // A turn starts with a draw; the drawn card lands selected on its own —
    // exactly one, which is what discard takes.
    await waitForOfferEnabled(page, 'offer-draw:deck');
    await page.getByTestId('offer-draw:deck').click();
    const discard = page.getByTestId('offer-discard');
    await expect(discard).not.toHaveAttribute('aria-disabled', 'true');
    await expect(page.getByTestId('why-discard')).toHaveCount(0);
    const handSize = (await serverHand(request, matchId, host.userId)).length;

    // The drawn card is picked *for* the player, not *by* them — touching a
    // different card replaces it rather than joining it, so the drawn card
    // they actually mean to keep has to be chosen first before a second one
    // can be added to it.
    const unselected = page.locator('[data-testid^="card-hand:"]:not([aria-selected="true"])');
    await unselected.first().click();
    await expect(page.locator('[data-testid^="card-hand:"][aria-selected="true"]')).toHaveCount(1);
    await unselected.first().click();
    await expect(page.locator('[data-testid^="card-hand:"][aria-selected="true"]')).toHaveCount(2);

    await expect(discard).toHaveAttribute('aria-disabled', 'true');
    await expect(page.getByTestId('why-discard')).toHaveText('Select just one card');

    // The server never saw a discard for either card.
    const untouched = await serverHand(request, matchId, host.userId);
    expect(untouched.length).toBe(handSize);

    // Deselecting one brings it back.
    await page.locator('[data-testid^="card-hand:"][aria-selected="true"]').first().click();
    await expect(discard).not.toHaveAttribute('aria-disabled', 'true');
    await expect(page.getByTestId('why-discard')).toHaveCount(0);

    // And it sends exactly the one card that stayed selected — never a
    // different, guessed one.
    const before = await serverHand(request, matchId, host.userId);
    const [stillSelected] = await selectedCodes(page);
    await discard.click();

    await expect.poll(async () => (await serverHand(request, matchId, host.userId)).length).toBe(
      before.length - 1,
    );
    // Counted, not just "not contains": two decks are in play, so the hand
    // may hold another copy of the same code, and that copy staying behind is
    // correct — one fewer of it is what "exactly this one" actually means.
    const after = await serverHand(request, matchId, host.userId);
    const countOf = (hand: string[], code: string) => hand.filter((c) => c === code).length;
    expect(countOf(after, stillSelected)).toBe(countOf(before, stillSelected) - 1);
  });
});
