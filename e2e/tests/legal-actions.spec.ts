import { expect, test } from '@playwright/test';

import { dragLocatorTo, waitForGameLoaded } from '../helpers/drag';
import { loginAs } from '../helpers/login';
import { seedGame } from '../helpers/seed';

/**
 * The browser end of Phase 1 (docs/extensibility-plan.md).
 *
 * The Go suites already prove the offers are correct and sufficient
 * (server/internal/rules/offers_agreement_test.go and
 * server/internal/game/offer_driven_play_test.go). What only a real browser
 * can prove is the last link: that the controls a player actually sees are
 * driven by those offers, so that a rule change on the server changes the UI
 * with no client edit.
 *
 * Every assertion below is about an affordance whose client-side derivation
 * this phase deleted — the expressions are listed in the plan's §1.4 table.
 */

test.describe('legal actions drive the UI', () => {
  test('the discard pile is inert and explains itself while the ruleset locks it', async ({
    page,
    request,
  }) => {
    // Continental locks discard pickup until table round 3; a freshly dealt
    // game is on round 1. This used to be re-derived client-side as
    // `discardDrawMinRound > 1 && round < discardDrawMinRound`; now the
    // server sends draw:discard disabled with whyNot=DISCARD_LOCKED and the
    // client only renders it.
    const game = await seedGame(
      request,
      {
        hand: ['7S', '2S', '3S', 'KD'],
        phase: 'draw',
        discardPile: ['9H'],
      },
      { rulesProfile: 'continental' },
    );
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    // The engine's own reason, rendered as text next to the pile.
    await expect(page.getByText(/locked/i)).toBeVisible();

    // ...and clicking the pile does nothing: the hand size is unchanged.
    const handBefore = await page.getByText(/Your hand \(\d+\)/).textContent();
    await page.getByTestId('discard-top-card').click();
    await page.waitForTimeout(400);
    await expect(page.getByText(/Your hand \(\d+\)/)).toHaveText(handBefore ?? '');

    // The deck, by contrast, is offered — proving the pile's inertness is
    // the lock and not a dead screen.
    await page.getByTestId('deck-pile').click();
    await expect(page.getByText(/Your hand \(5\)/)).toBeVisible();
  });

  test('the same pile is live under a ruleset that does not lock it', async ({ page, request }) => {
    // Same UI, same code path, different server ruleset — and the client
    // needed no change to tell them apart. This is the pair that makes the
    // previous test meaningful rather than a screenshot of a broken screen.
    const game = await seedGame(request, {
      hand: ['7S', '2S', '3S', 'KD'],
      phase: 'draw',
      discardPile: ['9H'],
    }); // default profile: zolik_classic, no lock
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    await expect(page.getByText(/locked/i)).toHaveCount(0);
    await page.getByTestId('discard-top-card').click();
    await expect(page.getByText(/Your hand \(5\)/)).toBeVisible();
  });

  test('lay-off is refused before going down and offered after, with no client rule', async ({
    page,
    request,
  }) => {
    // The deleted expression:
    //   canLayOff = isMyTurn && phase === 'meld' && roundReqMet[userId]
    //
    // Asserted through the per-meld control rather than a drag. A drag is the
    // wrong instrument here: when a meld is not a valid drop target the card
    // falls through to the next zone under the pointer — the discard pile —
    // which ends the turn and makes any follow-up assertion ambiguous about
    // *why* the lay-off did not happen. The button is bound directly to the
    // `lay_off:<meldId>` offer, so its presence is exactly the thing under
    // test.
    const seed = {
      hand: ['7S', '2S', '3S', 'KD'],
      melds: { ai: [['7H', '7C', '7D']] },
      phase: 'meld' as const,
      roundReqMet: false,
    };
    const game = await seedGame(request, seed);
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    const meldRow = page.locator('[data-testid^="meld-row-"]').first();
    await expect(meldRow.locator('[data-testid^="meld-card-"]')).toHaveCount(3);
    const meldId = (await meldRow.getAttribute('data-testid'))!.replace('meld-row-', '');

    // Select 7S — it would extend the 7s set, so the only thing standing
    // between this player and a lay-off is that they are not down yet.
    await page.getByTestId('hand-card-0').click();
    await expect(page.getByTestId(`lay-off-${meldId}`)).toHaveCount(0);

    // Flip only the server-side fact. No client code knows what changed.
    await game.reseed({ ...seed, roundReqMet: true });
    await expect(page.getByTestId(`lay-off-${meldId}`)).toBeVisible();

    // ...and the control the server started offering actually works. The
    // hand selection survives the state push, so it is not re-clicked here —
    // clicking a selected card toggles it off.
    await page.getByTestId(`lay-off-${meldId}`).click();
    await expect(meldRow.locator('[data-testid^="meld-card-"]')).toHaveCount(4);
  });

  test('"Swap joker here" appears only where a card in hand takes the joker\'s place', async ({
    page,
    request,
  }) => {
    // The deleted guess: `cards.some(c => c.startsWith('JOKER'))`, which
    // offered the swap on any meld holding a joker — including melds where
    // nothing in hand could actually replace it. The server now answers per
    // meld, per card.
    const game = await seedGame(request, {
      // 4S takes the joker's slot in 2S-3S-JOKER-5S. Nothing here fits the
      // second meld's joker.
      hand: ['4S', 'KD', '9C'],
      melds: { ai: [['2S', '3S', 'JOKER1', '5S'], ['TH', 'JH', 'JOKER2', 'KH']] },
      phase: 'meld',
      roundReqMet: true,
    });
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    // Selecting 4S should reveal the swap control on the first meld only.
    await page.getByTestId('hand-card-0').click();

    const rows = page.locator('[data-testid^="meld-row-"]');
    await expect(rows).toHaveCount(2);
    await expect(rows.nth(0).getByText(/Swap joker/i)).toBeVisible();
    await expect(rows.nth(1).getByText(/Swap joker/i)).toHaveCount(0);
  });

  test('undo controls appear exactly when the server offers them', async ({ page, request }) => {
    // The four canUndo* booleans this phase replaced. The pickup is made for
    // real (not seeded) because the undo window only opens on the draw
    // action itself — see rules.ValidateDraw.
    const game = await seedGame(request, {
      hand: ['7S', '2S', '3S', 'KD'],
      phase: 'draw',
      discardPile: ['9H'],
      roundReqMet: true,
    });
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    // Nothing has happened yet, so nothing is undoable.
    await expect(page.getByText('Undo take discard')).toHaveCount(0);

    await page.getByTestId('discard-top-card').click();
    await expect(page.getByText(/Your hand \(5\)/)).toBeVisible();

    // The pickup opened the window; the control is now offered.
    await expect(page.getByText('Undo take discard')).toBeVisible();

    await page.getByText('Undo take discard').click();
    await expect(page.getByText(/Your hand \(4\)/)).toBeVisible();
    // ...and closed again once used.
    await expect(page.getByText('Undo take discard')).toHaveCount(0);
  });

  test('the rules panel reports the ruleset the server resolved', async ({ page, request }) => {
    // rulesSummaryLines used to be a copy of rules/profiles.go keyed on the
    // profile name, so a house-rule override displayed the shipped profile's
    // numbers while the engine enforced different ones. Create a game with
    // an override and check the panel shows the override.
    const game = await seedGame(
      request,
      { hand: ['7S', '2S', '3S', 'KD'], phase: 'meld' },
      { rulesProfile: 'continental', initialMeldMinimum: 70 },
    );
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    await page.getByText(/rules/i).first().click();

    // Continental's own constants...
    await expect(page.getByText('12 cards')).toBeVisible();
    await expect(page.getByText('4 cards')).toBeVisible();
    // ...and the house-rule override, not the profile's shipped 35.
    await expect(page.getByText(/70\+ points/)).toBeVisible();
  });
});
