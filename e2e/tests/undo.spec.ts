import { expect, test } from '@playwright/test';

import { dragLocatorTo, waitForGameLoaded } from '../helpers/drag';
import { loginAs } from '../helpers/login';
import { seedGame } from '../helpers/seed';

test.describe('undo', () => {
  test('Undo turn reverts every lay-off since the draw at once, not just the last one', async ({
    page,
    request,
  }) => {
    // canUndoTurn needs a TurnMeldSnapshot, which the server only takes at
    // the moment of the draw action (see rules.ValidateDraw) and restores
    // the state to exactly as it was right after that draw — not before it
    // (the pickup itself is Undo *take discard*'s job, not Undo turn's). A
    // state seeded straight into the meld phase skips the draw entirely, so
    // this test starts at 'draw' and takes the discard for real, then lays
    // off onto two separate melds, so Undo turn has two stacked actions to
    // unwind at once — something a single Undo lay-off couldn't do.
    const game = await seedGame(request, {
      hand: ['7S', '2S', '3S', 'KD'],
      melds: { ai: [['7H', '7C', '7D'], ['2H', '2C', '2D']] },
      phase: 'draw',
      discardPile: ['9H'],
      roundReqMet: true,
    });
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    await page.getByTestId('discard-top-card').click();
    await expect(page.getByText('Your hand (5)')).toBeVisible();

    const meldRows = page.locator('[data-testid^="meld-row-"]');
    await expect(meldRows).toHaveCount(2);
    // 7S lays off onto the 7s set; hand-card-0 is still 7S at this point
    // (the pickup appended 9H to the end, not the front).
    await dragLocatorTo(page, page.getByTestId('hand-card-0'), meldRows.nth(0));
    await expect(page.getByText('Your hand (4)')).toBeVisible();
    // 2S (now hand-card-0 again) lays off onto the 2s set.
    await dragLocatorTo(page, page.getByTestId('hand-card-0'), meldRows.nth(1));
    await expect(page.getByText('Your hand (3)')).toBeVisible();
    await expect(meldRows.nth(0).locator('[data-testid^="meld-card-"]')).toHaveCount(4);
    await expect(meldRows.nth(1).locator('[data-testid^="meld-card-"]')).toHaveCount(4);

    await page.getByText('Undo turn').click();
    await page.waitForTimeout(400);

    // Back to right after the draw — both lay-offs undone at once, but the
    // pickup itself (9H still in hand, discard pile still empty) stands.
    await expect(page.getByText('Your hand (5)')).toBeVisible();
    await expect(meldRows.nth(0).locator('[data-testid^="meld-card-"]')).toHaveCount(3);
    await expect(meldRows.nth(1).locator('[data-testid^="meld-card-"]')).toHaveCount(3);
  });

  test('Undo lay-off reverts just the most recent lay-off', async ({ page, request }) => {
    const game = await seedGame(request, {
      hand: ['7S', '2S', '3S', '9C', 'KD'],
      melds: { ai: [['7H', '7C', '7D']] },
      phase: 'meld',
      roundReqMet: true,
    });
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    const meldRow = page.locator('[data-testid^="meld-row-"]').first();
    await dragLocatorTo(page, page.getByTestId('hand-card-0'), meldRow);
    await expect(page.getByText('Your hand (4)')).toBeVisible();

    await page.getByText('Undo lay-off').click();
    await page.waitForTimeout(400);

    await expect(page.getByText('Your hand (5)')).toBeVisible();
    await expect(meldRow.locator('[data-testid^="meld-card-"]')).toHaveCount(3);
  });

  test('Undo meld reverts a freshly laid new meld back to hand', async ({ page, request }) => {
    const game = await seedGame(request, {
      hand: ['7H', '7C', '7D', '2S', '3S', '9C', 'KD'],
      phase: 'meld',
    });
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    const staging = page.getByTestId('staging-zone');
    await dragLocatorTo(page, page.getByTestId('hand-card-0'), staging);
    await dragLocatorTo(page, page.getByTestId('hand-card-0'), staging);
    await dragLocatorTo(page, page.getByTestId('hand-card-0'), staging);
    await page.getByTestId('lay-all-button').click();
    await expect(page.getByText('Your hand (4)')).toBeVisible();
    await expect(page.getByText('TABLE MELDS')).toBeVisible();

    await page.getByText('Undo meld').click();
    await page.waitForTimeout(400);

    await expect(page.getByText('Your hand (7)')).toBeVisible();
    await expect(page.getByText('TABLE MELDS')).not.toBeVisible();
  });

  test('Undo take discard reverts the pickup and re-locks the draw phase', async ({ page, request }) => {
    const game = await seedGame(request, {
      hand: ['2S', '3S', '4S', '9C', 'KD'],
      phase: 'draw',
      discardPile: ['7H'],
    });
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    await page.getByTestId('discard-top-card').click();
    await expect(page.getByText('Your hand (6)')).toBeVisible();

    await page.getByText('Undo take discard').click();
    await page.waitForTimeout(400);

    await expect(page.getByText('Your hand (5)')).toBeVisible();
  });
});
