import { expect, test } from '@playwright/test';

import { dragLocatorTo, dragPointTo, waitForGameLoaded } from '../helpers/drag';
import { loginAs } from '../helpers/login';
import { seedGame } from '../helpers/seed';

test.describe('staging drop position', () => {
  test('dropping a card on the front half of the staged row inserts it first, not appended', async ({
    page,
    request,
  }) => {
    const game = await seedGame(request, {
      // 6S/8S staged first (out of run order); 7S then dropped between them.
      hand: ['6S', '8S', '7S', '2C', '3C', '9C', 'KD'],
      phase: 'meld',
    });
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    const staging = page.getByTestId('staging-zone');
    await dragLocatorTo(page, page.getByTestId('hand-card-0'), staging); // 6S -> [6S]
    await dragLocatorTo(page, page.getByTestId('hand-card-0'), staging); // 8S -> [6S, 8S]

    await expect(page.getByTestId('staged-card-0-0')).toContainText('6');
    await expect(page.getByTestId('staged-card-0-1')).toContainText('8');

    // hand-card-0 is now 7S. Drop it on the LEFT edge of the staged row —
    // between 6S and 8S — rather than appending it after 8S.
    const target = page.getByTestId('staged-card-0-0');
    await target.scrollIntoViewIfNeeded();
    const targetBox = (await target.boundingBox())!;
    const card = page.getByTestId('hand-card-0');
    await card.scrollIntoViewIfNeeded();
    const cardBox = (await card.boundingBox())!;

    await dragPointTo(
      page,
      { x: cardBox.x + cardBox.width / 2, y: cardBox.y + cardBox.height / 2 },
      // Just past the right edge of the first staged card (6S) — the
      // boundary between "insert before 6S" and "insert between 6S/8S".
      { x: targetBox.x + targetBox.width + 2, y: targetBox.y + targetBox.height / 2 },
    );

    // 7S landed between 6S and 8S, in rank order — not appended at the end.
    await expect(page.getByTestId('staged-card-0-0')).toContainText('6');
    await expect(page.getByTestId('staged-card-0-1')).toContainText('7');
    await expect(page.getByTestId('staged-card-0-2')).toContainText('8');
  });
});

test.describe('multi-select drag to stage', () => {
  test('dragging one of several selected cards stages the whole selection at once', async ({
    page,
    request,
  }) => {
    const game = await seedGame(request, {
      hand: ['4H', '5H', '6H', '2C', '3C', '9C', 'KD'],
      phase: 'meld',
    });
    await loginAs(page, game);
    await page.goto(`/game/${game.gameId}`);
    await waitForGameLoaded(page);

    // Select 4H, 5H, 6H (hand-card-0/1/2) via tap.
    await page.getByTestId('hand-card-0').click();
    await page.getByTestId('hand-card-1').click();
    await page.getByTestId('hand-card-2').click();

    // Drag just one of the selected cards onto the staging area — the
    // whole selection should move together, same as multi-select lay-off.
    await dragLocatorTo(page, page.getByTestId('hand-card-0'), page.getByTestId('staging-zone'));

    await expect(page.getByTestId('staged-card-0-0')).toBeVisible();
    await expect(page.getByTestId('staged-card-0-1')).toBeVisible();
    await expect(page.getByTestId('staged-card-0-2')).toBeVisible();
    await expect(page.getByText('Your hand (7)')).toBeVisible();

    const layButton = page.getByTestId('lay-all-button');
    await expect(layButton).toBeEnabled();
    await layButton.click();
    await expect(page.getByText('Your hand (4)')).toBeVisible();
    await expect(page.getByText('TABLE MELDS')).toBeVisible();
  });
});
