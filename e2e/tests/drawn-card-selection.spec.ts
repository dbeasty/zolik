import { expect, test, type Page } from '@playwright/test';

import { handCards } from '../helpers/drag';
import { API_BASE } from '../helpers/env';
import { waitForOfferEnabled } from '../helpers/turn';
import { cardByCode, clearHandSelection, handCodes, selectedCodes } from '../helpers/hand';

/**
 * What the card you just drew does to the card you tap next.
 *
 * A drawn card lands selected, so that playing it is one tap. The cost, until
 * this was fixed, was the commonest turn in the game: draw, then play some
 * *other* card. A tap adds to the selection rather than replacing it — that is
 * how several cards are gathered into a meld — so the player ended up with two
 * cards picked, an offer that takes exactly one matched neither, and nothing
 * lit up. Nothing on screen said the cure was to go and unpick a card they had
 * never picked; the board just looked broken.
 *
 * The rule now is one sentence: **a selection the player didn't make is
 * replaced by the first card they touch.** These tests are the two halves of
 * it, plus the deliberate multi-select it must not break.
 */

type Ctx = import('@playwright/test').APIRequestContext;

async function tableWithBots(request: Ctx, moduleId: string, seats = 2) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `drawn-${Math.random().toString(36).slice(2, 10)}` },
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
  await page.setViewportSize({ width: 1280, height: 1400 });
  await page.addInitScript(
    (s) => {
      window.localStorage.setItem('zolik_session', JSON.stringify(s));
    },
    {
      accessToken: host.accessToken,
      refreshToken: host.refreshToken,
      userId: host.userId,
      username: 'drawn',
      isGuest: true,
    },
  );
  await page.goto(`/match/${matchId}`);
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
}

/** Draws from the deck and returns the code of the card that arrived selected. */
async function drawOne(page: Page): Promise<string> {
  const before = await handCards(page);
  await waitForOfferEnabled(page, 'offer-draw:deck');
  await page.getByTestId('offer-draw:deck').click();
  await expect(page.locator('[data-testid^="card-hand:"]')).toHaveCount(before.length + 1, {
    timeout: 10_000,
  });
  // The drawn card is the one the app picks for you — that is the feature
  // this whole file is about, so assert it rather than assume it.
  await expect.poll(async () => (await selectedCodes(page)).length).toBe(1);
  return (await selectedCodes(page))[0];
}

async function board(request: Ctx, matchId: string, userId: string) {
  const res = await request.get(`${API_BASE}/matches/${matchId}?as=${userId}`);
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

/**
 * Plays whatever is live until a pickup off the discard pile is offered to
 * `userId`, or gives up after `budgetMs`. A fresh deal has nothing on the
 * pile yet, so this needs at least one discard to have happened first —
 * usually the host's own, on whichever turn the loop first finds something
 * pressable.
 */
async function waitForDiscardPickupOffered(
  page: Page,
  request: Ctx,
  matchId: string,
  userId: string,
  budgetMs = 30_000,
): Promise<boolean> {
  const deadline = Date.now() + budgetMs;
  while (Date.now() < deadline) {
    const body = await board(request, matchId, userId);
    const offer = (body.legalActions ?? []).find((o: any) => o.id === 'draw:discard');
    if (offer?.enabled) return true;
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
  return false;
}

/** The rank+suit prefix of a card code, sans the trailing suit letter — enough to tell two codes apart by more than just their copy in a two-deck hand. */
function rankPart(code: string): string {
  return code.replace(/[SHDC]$/, '');
}

test.describe('the card you just drew', () => {
  test('arrives already selected, so playing it is one tap', async ({ page, request }) => {
    const { matchId, host } = await tableWithBots(request, 'zolik');
    await openMatch(page, host, matchId);
    await handCards(page);

    const drawn = await drawOne(page);
    expect(await selectedCodes(page)).toEqual([drawn]);
  });

  test('steps aside when you tap a different card', async ({ page, request }) => {
    const { matchId, host } = await tableWithBots(request, 'zolik');
    await openMatch(page, host, matchId);
    await handCards(page);

    const drawn = await drawOne(page);

    // Tap some other card — the one the player actually means to play.
    const other = (await page.evaluate(() =>
      [...document.querySelectorAll('[data-testid^="card-hand:"]')].map(
        (c) => c.closest('[aria-label]')?.getAttribute('aria-label') ?? '',
      ),
    )).find((c) => c !== drawn);
    expect(other, 'the hand holds a card other than the drawn one').toBeTruthy();

    await cardByCode(page, other!).click();

    // Exactly one card picked, and it is the one just tapped. Before the fix
    // this was two, and every one-card offer went dark.
    await expect.poll(async () => await selectedCodes(page)).toEqual([other!]);
  });

  test('is simply unpicked when you tap it, rather than reselected', async ({ page, request }) => {
    const { matchId, host } = await tableWithBots(request, 'zolik');
    await openMatch(page, host, matchId);
    await handCards(page);

    await drawOne(page);
    await page.locator('[data-testid^="card-hand:"][aria-selected="true"]').click();

    // "Not that one" means what it says.
    await expect.poll(async () => await selectedCodes(page)).toEqual([]);
  });

  test('does not stop you gathering several cards deliberately', async ({ page, request }) => {
    const { matchId, host } = await tableWithBots(request, 'zolik');
    await openMatch(page, host, matchId);
    await handCards(page);

    await drawOne(page);

    // Once the selection is the player's own, taps accumulate again — which
    // is how a meld gets built, and the thing the replace rule must not cost.
    await clearHandSelection(page);
    const codes = await page.evaluate(() =>
      [...document.querySelectorAll('[data-testid^="card-hand:"]')].map(
        (c) => c.closest('[aria-label]')?.getAttribute('aria-label') ?? '',
      ),
    );
    // Two distinct codes, so clicking "the card labelled X" cannot land twice
    // on the same card.
    const distinct = [...new Set(codes)].slice(0, 2);
    test.skip(distinct.length < 2, 'hand has no two distinct cards');

    await cardByCode(page, distinct[0]).click();
    await cardByCode(page, distinct[1]).click();

    await expect
      .poll(async () => (await selectedCodes(page)).slice().sort())
      .toEqual(distinct.slice().sort());
  });
});

test.describe('a card picked up off the discard pile', () => {
  test('stays selected when you tap a different card, unlike a plain draw', async ({
    page,
    request,
  }) => {
    const { matchId, host } = await tableWithBots(request, 'zolik');
    await openMatch(page, host, matchId);
    await handCards(page);

    const offered = await waitForDiscardPickupOffered(page, request, matchId, host.userId);
    test.skip(!offered, 'never reached a position where a discard pickup was offered');

    const before = await handCodes(page);
    await page.getByTestId('offer-draw:discard').click();
    await expect(page.locator('[data-testid^="card-hand:"]')).toHaveCount(before.length + 1, {
      timeout: 10_000,
    });
    await expect.poll(async () => (await selectedCodes(page)).length).toBe(1);
    const drawn = (await selectedCodes(page))[0]!;

    // A card whose rank and suit both differ from the pickup, so no offer
    // could possibly be ready for the pair — this is specifically the case a
    // plain draw replaces on (see the tests above) and a pickup must not.
    const other = (await handCodes(page)).find((c) => rankPart(c) !== rankPart(drawn));
    test.skip(!other, 'hand holds no card of a different rank to tap');

    await cardByCode(page, other!).click();

    // Unlike stepping aside for a plain draw, the pickup is one the module
    // itself marked as owed to a meld this turn — see `zolik.badge.owedToMeld`
    // — so it rides along rather than being bumped by the first other card
    // touched. Both stay picked; only tapping the pickup itself would drop it.
    await expect
      .poll(async () => (await selectedCodes(page)).slice().sort())
      .toEqual([drawn, other!].sort());
  });
});
