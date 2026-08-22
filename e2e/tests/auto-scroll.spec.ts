import { expect, test } from '@playwright/test';

import { waitForGameLoaded } from '../helpers/drag';
import { loginAs } from '../helpers/login';
import { seedGame } from '../helpers/seed';

// Reproduces the "many melds on the table push the hand row off-screen"
// scenario: dragging a hand card up toward the top edge of the screen
// should auto-scroll the page (client-react-native/app/game/[gameId].tsx's
// handleAutoScroll) so a meld/staging area that's scrolled out of view
// becomes reachable without letting go of the drag.
test('dragging a hand card to the top edge auto-scrolls the page toward the melds', async ({
  page,
  request,
}) => {
  // Six 4-card melds is enough to make the "MELDS" box taller than a
  // trimmed-down viewport, pushing the hand row below the fold at the
  // page's initial (scrolled-to-top) position.
  await page.setViewportSize({ width: 480, height: 640 });

  const game = await seedGame(request, {
    hand: ['7S', '2S', '3S', '4S', '9C', 'KD'],
    melds: {
      ai: [
        ['3H', '4H', '5H', '6H'],
        ['3D', '4D', '5D', '6D'],
        ['3C', '4C', '5C', '6C'],
        ['3S', '4S', '5S', '6S'],
        ['7H', '8H', '9H', 'TH'],
        ['7D', '8D', '9D', 'TD'],
      ],
    },
    phase: 'meld',
    roundReqMet: true,
  });
  await loginAs(page, game);
  await page.goto(`/game/${game.gameId}`);
  await waitForGameLoaded(page);

  const scrollView = page.getByTestId('game-scroll-view');
  await expect(scrollView).toBeVisible();

  // Scroll all the way down so the hand row is on screen — same starting
  // point a player reaches by scrolling down to grab a card, with the
  // melds/staging area now above the fold.
  await scrollView.evaluate((el) => {
    el.scrollTop = el.scrollHeight;
  });
  const startTop = await scrollView.evaluate((el) => el.scrollTop);
  expect(startTop).toBeGreaterThan(0);

  const card = page.getByTestId('hand-card-0');
  await expect(card).toBeVisible();
  const cardBox = (await card.boundingBox())!;

  // Pick up the card, then drag it up to just inside the auto-scroll edge
  // band (AUTO_SCROLL_EDGE = 90px from the top) and hold it there with a
  // stream of small pointer moves — the auto-scroll only fires on drag
  // "hover" updates, which need real, continuing pointermove events (a
  // motionless held pointer never re-fires onUpdate). Mirrors the
  // load-bearing pause/step pattern in helpers/drag.ts.
  await page.mouse.move(cardBox.x + cardBox.width / 2, cardBox.y + cardBox.height / 2);
  await page.mouse.down();
  await page.waitForTimeout(200);

  const edgeX = cardBox.x + cardBox.width / 2;
  const edgeY = 40; // well inside the 90px top auto-scroll band
  await page.mouse.move(edgeX, edgeY, { steps: 15 });

  // Jiggle at the edge for ~1.2s (well past several 60ms hover-throttle
  // ticks) so handleAutoScroll gets repeated opportunities to nudge the
  // ScrollView upward.
  for (let i = 0; i < 15; i++) {
    await page.mouse.move(edgeX, edgeY + (i % 2), { steps: 1 });
    await page.waitForTimeout(80);
  }

  const midTop = await scrollView.evaluate((el) => el.scrollTop);
  expect(midTop).toBeLessThan(startTop);

  await page.mouse.up();
  await page.waitForTimeout(400);
});
