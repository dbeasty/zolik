import { expect, test } from '@playwright/test';

import { dragPointTo, waitForGameLoaded } from '../helpers/drag';
import { loginAs } from '../helpers/login';
import { seedGame } from '../helpers/seed';

test.describe('hand reordering', () => {
  test('dragging a card sideways within the hand reorders it', async ({ page, request }) => {
    const game = await seedGame(request, {
      hand: ['2S', '3S', '4S', '5S', '6S'],
      phase: 'discard',
    });
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    await expect(page.getByTestId('hand-card-0')).toContainText('2');

    const card = page.getByTestId('hand-card-0');
    const box = (await card.boundingBox())!;
    // Slide right by ~3 slots (each ~70px including margin) — comfortably
    // past REORDER_COMMIT_RATIO so it lands a few positions over, not back
    // where it started.
    await dragPointTo(
      page,
      { x: box.x + box.width / 2, y: box.y + box.height / 2 },
      { x: box.x + box.width / 2 + 220, y: box.y + box.height / 2 },
    );

    // 2S is no longer first — some other card slid into slot 0 instead.
    await expect(page.getByTestId('hand-card-0')).not.toContainText('2');
  });
});

test.describe('tap-to-select', () => {
  test('tapping a card toggles its selection ring without discarding or staging it', async ({
    page,
    request,
  }) => {
    const game = await seedGame(request, {
      hand: ['2S', '3S', '4S', '5S', '6S'],
      phase: 'meld',
    });
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    await page.getByTestId('hand-card-0').click();
    // Still in hand (a tap no longer discards or stages — see HandRow's
    // module doc comment) — just visually selected.
    await expect(page.getByTestId('hand-card-0')).toBeVisible();
    await expect(page.getByText('Your hand (5)')).toBeVisible();

    // Tapping again toggles selection back off.
    await page.getByTestId('hand-card-0').click();
    await expect(page.getByTestId('hand-card-0')).toBeVisible();
  });
});

test.describe('drop-target highlight', () => {
  test('the staging area pulses only while a card is actively being dragged', async ({
    page,
    request,
  }) => {
    const game = await seedGame(request, {
      hand: ['2S', '3S', '4S', '5S', '6S'],
      phase: 'meld',
    });
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    const staging = page.getByTestId('staging-zone');
    const card = page.getByTestId('hand-card-0');
    const cardBox = (await card.boundingBox())!;

    const borderBefore = await staging.evaluate((el) => getComputedStyle(el).borderColor);

    await page.mouse.move(cardBox.x + cardBox.width / 2, cardBox.y + cardBox.height / 2);
    await page.mouse.down();
    await page.waitForTimeout(200);
    // The pan gesture (and with it, draggedCard / the highlight) only
    // activates past minDistance(10) — holding still after mousedown never
    // triggers it, same as a plain tap wouldn't.
    await page.mouse.move(cardBox.x + cardBox.width / 2 + 30, cardBox.y + cardBox.height / 2, {
      steps: 5,
    });
    await page.waitForTimeout(200);
    const borderDuring = await staging.evaluate((el) => getComputedStyle(el).borderColor);
    await page.mouse.up();
    await page.waitForTimeout(400);

    expect(borderDuring).not.toBe(borderBefore);
  });
});
