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
    const meldBox = (await meldRow.boundingBox())!;
    // Aim at the left edge of the row so it reads as "front".
    const frontTarget = { x: meldBox.x + 10, y: meldBox.y + meldBox.height / 2 };

    const card = page.getByTestId('hand-card-0');
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
});
