import { expect, test } from '@playwright/test';

import { API_BASE } from '../helpers/env';
import { loginAsFreshGuest } from '../helpers/login';
import { openGameSetup } from '../helpers/lobby';
import { waitForOfferEnabled } from '../helpers/turn';

/**
 * Stepping back through a game that has stopped.
 *
 * The Go suite proves the fold is deterministic for every module, and proves
 * it against the module's own state. What it cannot show is the thing a player
 * actually meets: that the board on the last frame of a replay is the board
 * they left the table on, drawn by the same screen, in a real browser.
 *
 * So this spec plays a few real moves and then checks the two ends of the
 * replay against the table itself — the deal at one end, and at the other, the
 * position the game is sitting in right now.
 */
test.describe('a stopped game can be stepped through', () => {
  test('the replay opens on the deal and ends on the board the table is on', async ({
    page,
    request,
  }) => {
    test.setTimeout(180_000);

    const me = await loginAsFreshGuest(
      page,
      request,
      `e2e-replay-${Math.random().toString(36).slice(2, 8)}`,
    );

    await page.goto('/lobby/games');
    await expect(page.getByTestId('games-list')).toBeVisible({ timeout: 30_000 });
    await openGameSetup(page, 'prsi');
    await page.getByTestId('play-bots-prsi').click();
    await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 45_000 });

    const matchId = new URL(page.url()).pathname.split('/').pop()!;

    // A few real moves, so there is something to step through. Drawing is the
    // one move always available to whoever is on turn in Prší, and the point
    // here is a non-empty action log rather than good play.
    for (let i = 0; i < 3; i++) {
      await waitForOfferEnabled(page, 'offer-draw');
      await page.getByTestId('offer-draw').click();
    }

    // Leaving the table closes its socket, which is what lets the board come
    // to rest — and it has to come to rest before the comparison below means
    // anything, or the bots play on between the two reads and the replay is
    // legitimately a move or two ahead of the snapshot.
    //
    // Waiting for stillness rather than for status "suspended": a table only
    // suspends when it is waiting on the seat that left, so one that is on a
    // bot's turn when the human walks away never reaches that status and the
    // wait times out. Two identical reads in a row is the honest question —
    // is anything still happening? — and it is the same answer either way.
    //
    // "In a row" has to mean further apart than a bot takes to think, though
    // (BOT_THINK_MAX_MS, 1.8 s by default). expect.poll calls straight away,
    // so a bare comparison checks two reads microseconds apart, calls a bot
    // mid-think "still", and the replay fetched later is a move ahead of the
    // snapshot — a bot's hand one card off on the last frame.
    const STILL_FOR_MS = 2_500;
    await page.goto('/lobby/mine');
    const zoneCounts = async () => {
      const res = await request.get(`${API_BASE}/matches/${matchId}`);
      if (!res.ok()) return 'unreadable';
      const body = (await res.json()) as { view: { zones: { id: string; count: number }[] } };
      return JSON.stringify(body.view.zones.map((z) => [z.id, z.count]));
    };
    let settled = await zoneCounts();
    let settledAt = Date.now();
    await expect
      .poll(
        async () => {
          const again = await zoneCounts();
          if (again !== settled) {
            settled = again;
            settledAt = Date.now();
            return false;
          }
          return Date.now() - settledAt >= STILL_FOR_MS;
        },
        {
          timeout: 60_000,
          intervals: [500],
          message: 'the board should stop moving once nobody is at the table',
        },
      )
      .toBe(true);
    const liveCounts = Object.fromEntries(
      (JSON.parse(settled) as [string, number][]).map(([id, count]) => [id, count]),
    );

    // The way in is the stored-games list, as a player finds it.
    const replayButton = page.getByTestId(`mine-replay-${matchId}`);
    await expect(replayButton).toBeVisible({ timeout: 30_000 });
    await replayButton.click();

    await expect(page.getByTestId('replay-screen')).toBeVisible({ timeout: 30_000 });

    // It opens on the deal, and says so.
    const step = page.getByTestId('replay-step');
    await expect(step).toContainText('1 of', { timeout: 15_000 });
    // Nothing has happened yet, so there is nowhere back to go.
    await expect(page.getByTestId('replay-first')).toHaveAttribute('aria-disabled', 'true');
    await expect(page.getByTestId('replay-prev')).toHaveAttribute('aria-disabled', 'true');

    // Forward one, and the caption names a mover rather than the deal.
    await page.getByTestId('replay-next').click();
    await expect(step).toContainText('2 of', { timeout: 15_000 });
    await expect(page.getByTestId('replay-prev')).not.toHaveAttribute('aria-disabled', 'true');

    // And back again: stepping is reversible, which is the whole point.
    await page.getByTestId('replay-prev').click();
    await expect(step).toContainText('1 of', { timeout: 15_000 });

    // Jump to the end. This is the assertion the whole feature rests on: the
    // last frame of the fold is the position the table is actually in.
    await page.getByTestId('replay-last').click();
    await expect(page.getByTestId('replay-next')).toHaveAttribute('aria-disabled', 'true', {
      timeout: 15_000,
    });

    const replayed = await request.get(`${API_BASE}/matches/${matchId}/replay?from=0&limit=300`, {
      headers: { Authorization: `Bearer ${me.accessToken}` },
    });
    expect(replayed.ok(), await replayed.text()).toBeTruthy();
    const rep = (await replayed.json()) as {
      total: number;
      truncated?: boolean;
      frames: { index: number; view: { zones: { id: string; count: number }[] } }[];
    };
    expect(rep.truncated ?? false, 'a game played minutes ago should still fold').toBe(false);

    const last = rep.frames[rep.frames.length - 1]!;
    const lastCounts = Object.fromEntries(last.view.zones.map((z) => [z.id, z.count]));
    for (const [zoneId, count] of Object.entries(liveCounts)) {
      expect(lastCounts[zoneId], `zone ${zoneId} on the last frame`).toBe(count);
    }
  });

  /**
   * A replay is for the people who played it. The server holds this line —
   * there is a Go test on the same rule — but the route is new surface, and a
   * client that could reach somebody else's game is worth catching here too.
   */
  test('somebody who was not at the table cannot replay it', async ({ browser, request }) => {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();
    try {
      const host = await request.post(`${API_BASE}/auth/guest`, {
        data: { guestName: `replay-host-${Math.random().toString(36).slice(2, 8)}` },
      });
      expect(host.ok(), await host.text()).toBeTruthy();
      const { accessToken } = (await host.json()) as { accessToken: string };

      const created = await request.post(`${API_BASE}/matches`, {
        headers: { Authorization: `Bearer ${accessToken}` },
        data: { moduleId: 'prsi' },
      });
      expect(created.ok(), await created.text()).toBeTruthy();
      const { matchId } = (await created.json()) as { matchId: string };

      const stranger = await loginAsFreshGuest(
        page,
        request,
        `e2e-nosy-${Math.random().toString(36).slice(2, 8)}`,
      );
      const refused = await request.get(`${API_BASE}/matches/${matchId}/replay`, {
        headers: { Authorization: `Bearer ${stranger.accessToken}` },
      });
      expect(refused.status(), await refused.text()).toBe(403);
      expect(((await refused.json()) as { code: string }).code).toBe('NOT_AT_THIS_TABLE');
    } finally {
      await ctx.close();
    }
  });
});
