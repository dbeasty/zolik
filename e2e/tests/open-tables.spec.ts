import { expect, test } from '@playwright/test';

import { openGame, openTableFor } from '../helpers/lobby';
import { loginAsFreshGuest } from '../helpers/login';

/**
 * Tables open to anyone: a host lists their lobby, and a stranger who opens
 * the same game finds it on the game's page and sits down — no code, no
 * invite. Two browser contexts, because the subject is two people who have
 * never exchanged anything.
 */

const tag = () => Math.random().toString(36).slice(2, 8);

test.describe('open tables', () => {
  test('a listed table is offered on the game page, and Join seats the stranger', async ({ browser, request }) => {
    const hostCtx = await browser.newContext();
    const guestCtx = await browser.newContext();
    const hostPage = await hostCtx.newPage();
    const guestPage = await guestCtx.newPage();
    try {
      await loginAsFreshGuest(hostPage, request, `e2e-host-${tag()}`);
      const guest = await loginAsFreshGuest(guestPage, request, `e2e-stranger-${tag()}`);

      await openGame(hostPage, 'prsi');
      await openTableFor(hostPage, 'prsi');
      await expect(hostPage.getByTestId('table-screen')).toBeVisible({ timeout: 20_000 });
      const code = (await hostPage.getByTestId('table-join-code').innerText()).trim();

      // Not listed until the host says so.
      const toggle = hostPage.getByTestId('listed-toggle');
      await expect(toggle).toHaveAttribute('aria-checked', 'false');
      await toggle.click();
      await expect(toggle).toHaveAttribute('aria-checked', 'true', { timeout: 10_000 });

      await openGame(guestPage, 'prsi');
      const join = guestPage.getByTestId(`open-table-join-${code}`);
      await expect(join).toBeVisible({ timeout: 15_000 });
      await join.click();

      // Seated at the host's table, as the host sees it.
      await expect(hostPage.getByTestId(`seated-${guest.userId}`)).toBeVisible({ timeout: 15_000 });

      // Withdrawn, it is off the page for everybody else.
      await toggle.click();
      await expect(toggle).toHaveAttribute('aria-checked', 'false', { timeout: 10_000 });
    } finally {
      await hostCtx.close();
      await guestCtx.close();
    }
  });

  test('a table opened the ordinary way is not offered to strangers', async ({ browser, request }) => {
    const hostCtx = await browser.newContext();
    const guestCtx = await browser.newContext();
    const hostPage = await hostCtx.newPage();
    const guestPage = await guestCtx.newPage();
    try {
      await loginAsFreshGuest(hostPage, request, `e2e-host-${tag()}`);
      await loginAsFreshGuest(guestPage, request, `e2e-stranger-${tag()}`);

      // A game no other spec lists a table of, so "none" means this one.
      await openGame(hostPage, 'canasta');
      await openTableFor(hostPage, 'canasta');
      await expect(hostPage.getByTestId('table-screen')).toBeVisible({ timeout: 20_000 });
      const code = (await hostPage.getByTestId('table-join-code').innerText()).trim();

      await openGame(guestPage, 'canasta');
      await expect(guestPage.getByTestId('open-tables-card')).toBeVisible({ timeout: 15_000 });
      await expect(guestPage.getByTestId(`open-table-join-${code}`)).toHaveCount(0);
    } finally {
      await hostCtx.close();
      await guestCtx.close();
    }
  });
});
