import { expect, test } from '@playwright/test';

import { dragLocatorTo, waitForGameLoaded } from '../helpers/drag';
import { loginAs } from '../helpers/login';
import { seedGame } from '../helpers/seed';

/**
 * The browser end of Phase 2.3 (docs/extensibility-plan.md).
 *
 * An offer describes a *shape* ("3 or more cards from your hand") because
 * enumerating every legal combination of a 13-card hand is combinatorial. So
 * the one question the offer list structurally cannot answer is the one a
 * player assembling a meld actually asks: are *these* cards a meld, and what
 * are they worth?
 *
 * Each client used to answer that with its own copy of the rules — the
 * terminal client carried a second card-scoring table purely to render a live
 * "natural value" readout. Now the client sends the candidate and the engine
 * answers, so the number shown while choosing is computed by the same code
 * that judges the submission.
 */

test.describe('meld preview', () => {
  test('the staging area reports what the server says the cards are worth', async ({
    page,
    request,
  }) => {
    const game = await seedGame(request, {
      hand: ['KS', 'KD', 'KH', '4C'],
      phase: 'meld',
    });
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    // Nothing staged: nothing to preview.
    await expect(page.getByTestId('staging-preview')).toHaveCount(0);

    // Stage the three kings.
    const staging = page.getByTestId('staging-zone');
    for (let i = 0; i < 3; i++) {
      await dragLocatorTo(page, page.getByTestId('hand-card-0'), staging);
      await page.waitForTimeout(200);
    }

    const preview = page.getByTestId('staging-preview');
    await expect(preview).toBeVisible();
    // Three kings: a valid set worth 30 natural points. Both facts come from
    // the engine — no client here knows a king is worth 10.
    await expect(preview).toContainText(/valid set/i);
    await expect(preview).toContainText('30');
  });

  test('an incomplete selection is priced but not called a meld', async ({ page, request }) => {
    // The readout exists to be watched *while* choosing, so a half-built meld
    // has to report a running total rather than going blank until the shape
    // happens to be legal.
    const game = await seedGame(request, {
      hand: ['KS', '4C', '9H', '2D'],
      phase: 'meld',
    });
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    const staging = page.getByTestId('staging-zone');
    await dragLocatorTo(page, page.getByTestId('hand-card-0'), staging);
    await page.waitForTimeout(300);
    await dragLocatorTo(page, page.getByTestId('hand-card-0'), staging);
    await page.waitForTimeout(300);

    const preview = page.getByTestId('staging-preview');
    await expect(preview).toBeVisible();
    await expect(preview).not.toContainText(/valid/i);
    // K(10) + 4 = 14 — the engine's scoring, not the client's.
    await expect(preview).toContainText('14');
  });

  test('the floor shown is the one the ruleset actually enforces', async ({ page, request }) => {
    // A house-rule override has to reach the readout, or a player is told
    // they have cleared a floor the server will refuse them on.
    const game = await seedGame(
      request,
      { hand: ['KS', 'KD', 'KH', '4C'], phase: 'meld' },
      { rulesProfile: 'continental', initialMeldMinimum: 70 },
    );
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    const staging = page.getByTestId('staging-zone');
    for (let i = 0; i < 3; i++) {
      await dragLocatorTo(page, page.getByTestId('hand-card-0'), staging);
      await page.waitForTimeout(200);
    }

    const preview = page.getByTestId('staging-preview');
    await expect(preview).toBeVisible();
    // 30 points against the overridden 70-point floor, not continental's
    // shipped 35.
    await expect(preview).toContainText('70');
    await expect(preview).toContainText('✗');
  });

  test('previewing changes nothing — it is not a move', async ({ page, request }) => {
    // A preview dry-runs a real lay_meld inside the engine. If it did not
    // clone first, watching the readout would quietly remove cards from the
    // player's own hand.
    const game = await seedGame(request, {
      hand: ['KS', 'KD', 'KH', '4C'],
      phase: 'meld',
    });
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    const staging = page.getByTestId('staging-zone');
    for (let i = 0; i < 3; i++) {
      await dragLocatorTo(page, page.getByTestId('hand-card-0'), staging);
      await page.waitForTimeout(200);
    }
    await expect(page.getByTestId('staging-preview')).toBeVisible();

    // The hand count is the server's, and it is unchanged: staging is a
    // client-side arrangement, and previewing it consumed nothing. Nothing
    // reached the table either.
    await expect(page.getByText(/Your hand \(4\)/)).toBeVisible();
    await expect(page.getByText('TABLE MELDS')).toHaveCount(0);

    // The cards are still there to actually play.
    await page.getByTestId('lay-all-button').click();
    await expect(page.getByText('TABLE MELDS')).toBeVisible();
  });
});
