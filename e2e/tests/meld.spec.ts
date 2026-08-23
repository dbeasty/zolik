import { expect, test } from '@playwright/test';

import { dragLocatorTo, waitForGameLoaded } from '../helpers/drag';
import { loginAs } from '../helpers/login';
import { seedGame } from '../helpers/seed';

test.describe('building a new meld by drag', () => {
  test('dragging three hand cards onto the staging area stages them, then Lay meld lays it down', async ({
    page,
    request,
  }) => {
    const game = await seedGame(request, {
      hand: ['7H', '7C', '7D', '2S', '3S', '4S', '9C', 'KD'],
      phase: 'meld',
    });
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    const staging = page.getByTestId('staging-zone');
    // Drag hand-card-0 (7H) three times — after each drop the staged card
    // leaves the hand array, so the next 7 always re-lands on index 0.
    await dragLocatorTo(page, page.getByTestId('hand-card-0'), staging);
    await dragLocatorTo(page, page.getByTestId('hand-card-0'), staging);
    await dragLocatorTo(page, page.getByTestId('hand-card-0'), staging);

    await expect(page.getByTestId('staged-card-0-0')).toBeVisible();
    await expect(page.getByTestId('staged-card-0-1')).toBeVisible();
    await expect(page.getByTestId('staged-card-0-2')).toBeVisible();
    // Staging is purely a client-side grouping step — the cards are still
    // physically in hand (just hidden from the visible row) until Lay meld
    // actually sends lay_meld to the server, so "Your hand (N)" doesn't
    // shrink yet.
    await expect(page.getByText('Your hand (8)')).toBeVisible();

    const layButton = page.getByTestId('lay-all-button');
    await expect(layButton).toBeEnabled();
    await layButton.click();

    // Only now do the 3 cards actually leave the hand server-side, and the
    // meld shows up under "TABLE MELDS" — three 7s is a valid set.
    await expect(page.getByText('Your hand (5)')).toBeVisible();
    await expect(page.getByText('TABLE MELDS')).toBeVisible();
    await expect(page.getByTestId('staged-card-0-0')).not.toBeVisible();
  });

  test('staging a set and a run as two separate groups lays both down in one "Lay meld" tap', async ({
    page,
    request,
  }) => {
    const game = await seedGame(request, {
      hand: ['7H', '7C', '7D', '2S', '3S', '4S', '9C', 'KD'],
      phase: 'meld',
    });
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    const staging = page.getByTestId('staging-zone');
    // First group: the three 7s (a set). Card 0 keeps re-landing at index 0
    // as each staged card leaves the visible hand row.
    await dragLocatorTo(page, page.getByTestId('hand-card-0'), staging);
    await dragLocatorTo(page, page.getByTestId('hand-card-0'), staging);
    await dragLocatorTo(page, page.getByTestId('hand-card-0'), staging);
    await expect(page.getByTestId('staged-card-0-2')).toBeVisible();

    // Open a second group box, then stage the 2S/3S/4S run into it — with
    // group 0 already full, a drop anywhere in the staging box (not over a
    // specific group row) falls back to the last group, which is now the
    // freshly-added empty one.
    await page.getByTestId('add-group-button').click();
    await dragLocatorTo(page, page.getByTestId('hand-card-0'), staging);
    await dragLocatorTo(page, page.getByTestId('hand-card-0'), staging);
    await dragLocatorTo(page, page.getByTestId('hand-card-0'), staging);
    await expect(page.getByTestId('staged-card-1-2')).toBeVisible();

    // Both groups still just client-side staging — nothing has left the
    // hand server-side yet.
    await expect(page.getByText('Your hand (8)')).toBeVisible();

    const layButton = page.getByTestId('lay-all-button');
    await expect(layButton).toBeEnabled();
    await expect(layButton).toContainText('Lay meld (2)');
    await layButton.click();

    // Both groups laid down together: 6 cards left the hand, and two melds
    // show up under "TABLE MELDS".
    await expect(page.getByText('Your hand (2)')).toBeVisible();
    await expect(page.locator('[data-testid^="meld-row-"]')).toHaveCount(2);
    await expect(page.getByTestId('staged-card-0-0')).not.toBeVisible();
  });

  test('Cancel on a staged group returns its cards to the hand', async ({ page, request }) => {
    const game = await seedGame(request, {
      hand: ['7H', '7C', '7D', '2S', '3S', '4S', '9C', 'KD'],
      phase: 'meld',
    });
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    await dragLocatorTo(page, page.getByTestId('hand-card-0'), page.getByTestId('staging-zone'));
    await expect(page.getByTestId('staged-card-0-0')).toBeVisible();
    // hand-card-0 is now 7C (7H left the visible row for the staging area) —
    // confirm the 7 that's still shown in the hand isn't the one we staged.
    await expect(page.getByTestId('hand-card-0')).toContainText('7');

    await page.getByTestId('cancel-group-0').click();
    await expect(page.getByTestId('staged-card-0-0')).not.toBeVisible();
    // 7H is back in the (now unstaged) hand row.
    await expect(page.getByTestId('hand-card-0')).toContainText('7');
  });
});

test.describe('laying off onto a table meld by drag', () => {
  test('dragging a matching card onto an opponent set lays it off', async ({ page, request }) => {
    const game = await seedGame(request, {
      hand: ['7S', '2S', '3S', '4S', '9C', 'KD'],
      melds: { ai: [['7H', '7C', '7D']] },
      phase: 'meld',
      roundReqMet: true,
    });
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    const meldRow = page.locator('[data-testid^="meld-row-"]').first();
    await expect(meldRow).toBeVisible();

    await dragLocatorTo(page, page.getByTestId('hand-card-0'), meldRow);

    await expect(page.getByText('Your hand (5)')).toBeVisible();
    // The set now holds 4 cards instead of 3.
    await expect(meldRow.locator('[data-testid^="meld-card-"]')).toHaveCount(4);
  });

  test('dragging a card onto the front half of a run extends its low end', async ({ page, request }) => {
    const game = await seedGame(request, {
      hand: ['5H', '2S', '3S', '4S', '9C', 'KD'],
      melds: { ai: [['6H', '7H', '8H', '9H']] },
      phase: 'meld',
      roundReqMet: true,
    });
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    const meldRow = page.locator('[data-testid^="meld-row-"]').first();
    // Raw page.mouse coordinates don't auto-scroll like locator actions do
    // (see dragLocatorTo in helpers/drag.ts) — scroll the card into view
    // before reading either box's position, so both are measured in their
    // final, settled scroll position.
    const card = page.getByTestId('hand-card-0');
    await card.scrollIntoViewIfNeeded();
    const meldBox = (await meldRow.boundingBox())!;
    // Aim at the left edge of the row so it reads as "front".
    const frontTarget = { x: meldBox.x + 10, y: meldBox.y + meldBox.height / 2 };

    const cardBox = (await card.boundingBox())!;
    await page.mouse.move(cardBox.x + cardBox.width / 2, cardBox.y + cardBox.height / 2);
    await page.mouse.down();
    await page.waitForTimeout(200);
    await page.mouse.move(frontTarget.x, frontTarget.y, { steps: 15 });
    await page.waitForTimeout(200);
    await page.mouse.up();
    await page.waitForTimeout(400);

    await expect(page.getByText('Your hand (5)')).toBeVisible();
    await expect(meldRow.locator('[data-testid^="meld-card-"]')).toHaveCount(5);
    // 5H extending the front should now be the first card shown.
    await expect(meldRow.locator('[data-testid^="meld-card-"]').first()).toContainText('5');
  });
});

test.describe('multi-select lay-off', () => {
  test('tapping two cards then clicking "Lay off N here" lays off both together', async ({
    page,
    request,
  }) => {
    const game = await seedGame(request, {
      hand: ['TS', 'JS', '2S', '3S', '9C', 'KD'],
      melds: { ai: [['6S', '7S', '8S', '9S']] },
      phase: 'meld',
      roundReqMet: true,
    });
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    // TS and JS (hand-card-0, hand-card-1) both extend the spade run's high
    // end — a run, so the per-meld action is "End ▶" rather than a single
    // generic "Lay off N here" (that label is set-only, see MeldTable).
    await page.getByTestId('hand-card-0').click();
    await page.getByTestId('hand-card-1').click();

    const meldRow = page.locator('[data-testid^="meld-row-"]').first();
    const layButton = meldRow.locator('[data-testid^="lay-off-end-"]');
    await expect(layButton).toBeVisible();
    await layButton.click();
    await page.waitForTimeout(400);

    await expect(page.getByText('Your hand (4)')).toBeVisible();
    await expect(meldRow.locator('[data-testid^="meld-card-"]')).toHaveCount(6);
  });
});

test.describe('swap joker', () => {
  test('selecting the natural card and clicking "Swap joker here" swaps it in', async ({
    page,
    request,
  }) => {
    // The deck has two copies of every card (see rules.DeckCountForPlayers),
    // so an 8C in hand and an 8C's worth of joker on the table isn't a
    // conflict — and debug-state doesn't enforce card-supply conservation
    // anyway, since it bypasses rules validation entirely.
    const game = await seedGame(request, {
      hand: ['8C', '2S', '3S', 'KD'],
      melds: { ai: [['6C', '7C', 'JOKER1', '9C']] },
      phase: 'meld',
      roundReqMet: true,
    });
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    await page.getByTestId('hand-card-0').click();

    const meldRow = page.locator('[data-testid^="meld-row-"]').first();
    const swapButton = meldRow.locator('[data-testid^="swap-joker-"]');
    await expect(swapButton).toBeVisible();
    await swapButton.click();
    await page.waitForTimeout(400);

    // 8C replaces the joker in the run; the joker comes back to hand
    // (displayed as "JKR" — see parseCard).
    await expect(meldRow.getByText('JKR')).not.toBeVisible();
    await expect(page.getByText('JKR')).toBeVisible();
  });
  // Reported live (game 6a8a940102b43b43b9512844): the AI held the heart run
  // 8-9-T-J-Q-K with jokers standing in for the 8H and the TH, the player
  // held the real TH, dragged it onto that meld — and nothing happened at
  // all. No joker, no error, no move.
  //
  // Two jokers is what makes the shape bite, and why this needs its own
  // spec rather than a tweak to the one above. With a single joker the
  // server can usually take the card as a plain lay-off instead (it
  // re-resolves the wild onto a free end and drops the natural into the
  // vacated slot), so the meld is a lay-off target and the drop lands
  // either way. Here it cannot: the second joker leaves no end to re-resolve
  // onto, so lay-off is refused outright (INVALID_MELD) and the swap is the
  // only thing this meld will take.
  //
  // The screen used to measure a meld as a drop target only while *some*
  // lay-off was legal somewhere on the table, so a table like this had no
  // drop zones at all and the drag resolved onto nothing. The "Swap joker
  // here" button path (tested above) kept working throughout, which is why
  // the bug read as "sometimes the swap just doesn't happen."
  test('dragging the natural card onto a swap-only meld reclaims the joker', async ({
    page,
    request,
  }) => {
    const game = await seedGame(request, {
      // The run reads 8-9-T-J-Q-K with the 8H and the TH both played as
      // jokers. Nothing in hand extends it (its ends want a 7H or an AH),
      // so the only move this meld offers is handing back a joker.
      hand: ['TH', '2S', '3S', 'KD'],
      melds: { ai: [['JOKER1', '9H', 'JOKER2', 'JH', 'QH', 'KH']] },
      phase: 'meld',
      roundReqMet: true,
    });
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    const meldRow = page.locator('[data-testid^="meld-row-"]').first();
    const meldCards = meldRow.locator('[data-testid^="meld-card-"]');
    await expect(meldCards).toHaveCount(6);
    await expect(meldCards.filter({ hasText: 'JKR' })).toHaveCount(2);

    // No tap-to-select first: the drag alone has to carry the intent, which
    // is how a player actually reaches for a joker.
    await dragLocatorTo(page, page.getByTestId('hand-card-0'), meldRow);

    // One joker has left the meld for the hand. The meld keeps its six cards
    // and the hand its four — a swap trades one card for one — so the counts
    // alone would not notice this move at all; what changed is *which* cards
    // sit where.
    await expect(meldCards).toHaveCount(6);
    await expect(meldCards.filter({ hasText: 'JKR' })).toHaveCount(1);
    await expect(meldCards.filter({ hasText: '10' })).toHaveCount(1);
    await expect(page.getByText('Your hand (4)')).toBeVisible();
    await expect(page.locator('[data-testid^="hand-card-"]').filter({ hasText: 'JKR' })).toHaveCount(
      1,
    );
  });
});
