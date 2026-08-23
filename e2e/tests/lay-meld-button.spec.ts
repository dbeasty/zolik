import { expect, test } from '@playwright/test';

import { dragLocatorTo, waitForGameLoaded } from '../helpers/drag';
import { loginAs } from '../helpers/login';
import { seedGame } from '../helpers/seed';

/**
 * "Lay meld" must become usable as soon as there is something to lay.
 *
 * The button is gated on the server's `lay_meld` offer plus "is anything
 * staged". Both halves have to be right, and the second is easy to get wrong
 * because staging is a purely client-side arrangement the server never sees.
 */

async function stage(page: import('@playwright/test').Page, times: number) {
  const staging = page.getByTestId('staging-zone');
  for (let i = 0; i < times; i++) {
    await dragLocatorTo(page, page.getByTestId('hand-card-0'), staging);
    await page.waitForTimeout(200);
  }
}

test.describe('lay meld button', () => {
  test('is usable once cards are staged, in the meld phase', async ({ page, request }) => {
    const game = await seedGame(request, {
      hand: ['KS', 'KD', 'KH', '4C'],
      phase: 'meld',
    });
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    const lay = page.getByTestId('lay-all-button');
    // Nothing staged: the resting primary blue, dimmed.
    await expect(lay).toBeVisible();
    await expect(lay).toHaveCSS('opacity', '0.4');

    await stage(page, 3);

    // Something to lay: undimmed *and* a visibly lighter blue. The colour
    // change is the point — a dimmed blue on this dark background still
    // reads as "a blue button", so opacity alone didn't tell the player the
    // button had come alive.
    await expect(lay).toHaveCSS('opacity', '1');
    await expect(lay).toHaveCSS('background-color', 'rgb(96, 165, 250)');
    await lay.click();
    await expect(page.getByText('TABLE MELDS')).toBeVisible();
  });

  test('becomes usable after drawing, in a game that starts in the draw phase', async ({
    page,
    request,
  }) => {
    // The realistic path a player takes. A deal opens in the draw phase, and
    // staging is blocked until the server offers a meld (see stageCardsAt), so
    // dragging cards in beforehand does nothing. Once they draw, staging and
    // "Lay meld" must both start working.
    const game = await seedGame(request, {
      hand: ['KS', 'KD', 'KH', '4C'],
      phase: 'draw',
      discardPile: ['9H'],
    });
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    // Before drawing: dragging into the box stages nothing, and the box says
    // why rather than sitting inert.
    await stage(page, 1);
    await expect(page.getByText(/Your hand \(4\)/)).toBeVisible();
    await expect(page.getByTestId('staging-preview')).toContainText(/draw a card/i);

    await page.getByTestId('deck-pile').click();
    await expect(page.getByText(/Your hand \(5\)/)).toBeVisible();

    // After drawing: staging works and the meld goes down.
    await stage(page, 3);
    await page.getByTestId('lay-all-button').click();
    await expect(page.getByText('TABLE MELDS')).toBeVisible();
  });

  test('explains itself rather than sitting dead while melding is not allowed', async ({
    page,
    request,
  }) => {
    // Staged cards plus a server that will not accept them yet is exactly the
    // state a player finds confusing. Whatever the button does, the screen has
    // to say why — a disabled control with no reason is the failure mode the
    // whole offer mechanism exists to remove.
    const game = await seedGame(request, {
      hand: ['KS', 'KD', 'KH', '4C'],
      phase: 'draw',
      discardPile: ['9H'],
    });
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    await stage(page, 3);

    // The engine's own reason, rendered — not a bare greyed-out button. And
    // specific: "draw a card before melding" rather than "not available",
    // because the server distinguishes that case (rules.ErrMustDrawFirst).
    const preview = page.getByTestId('staging-preview');
    await expect(preview).toBeVisible();
    await expect(preview).toContainText(/draw a card before melding/i);
  });
});
