import { expect, test } from '@playwright/test';

import { openGame, playAgainstBots } from '../helpers/lobby';
import { loginAsFreshGuest } from '../helpers/login';

/**
 * The piles stay where they are while the turn goes round.
 *
 * On a phone the board is taller than the window and sits scrolled to the
 * hand. Two things used to move the draw and discard piles on every turn,
 * which on an iPhone read as the stock flickering and half hidden:
 *
 * - the seat whose turn it is grew a "to play" line and a thicker border, so
 *   the seat strip above the piles changed height each move;
 * - the boxes below the piles carry lines only on the viewer's turn, so the
 *   board got shorter when the turn passed, the scroll clamped, and
 *   everything above slid down.
 *
 * So, across real turns against a bot at an iPhone's size: no seat tile
 * changes height, and the board never gets shorter within the deal. Sampled
 * every frame, not after the fact, because the jump lasted as long as a turn.
 */

test.use({ viewport: { width: 393, height: 852 }, hasTouch: true });
test.setTimeout(120_000);

test('Last Card on a phone: the seats and the board keep their size as turns pass', async ({ page, request }) => {
  await loginAsFreshGuest(page, request, `steady-${Math.random().toString(36).slice(2, 6)}`);
  await openGame(page, 'lastcard');
  await playAgainstBots(page, 'lastcard');
  await expect(page).toHaveURL(/\/match\//, { timeout: 30_000 });
  await expect(page.getByTestId('zone-draw')).toBeVisible({ timeout: 30_000 });

  await page.evaluate(() => {
    type Sample = { turn: string; seats: Record<string, number>; board: number };
    const w = window as unknown as { __samples: Sample[] };
    w.__samples = [];
    const tick = () => {
      // Only while a hand is being played: a settlement is a different board.
      if (document.querySelector('[data-testid="offer-draw"]')) {
        const seats: Record<string, number> = {};
        for (const el of document.querySelectorAll<HTMLElement>('[data-testid^="seat-"]')) {
          const id = el.dataset.testid!;
          if (/^seat-(active|team)-/.test(id) || id === 'seat-strip') continue;
          seats[id] = Math.round(el.getBoundingClientRect().height);
        }
        const active = document.querySelector<HTMLElement>('[data-testid^="seat-active-"]');
        const board = document.querySelector<HTMLElement>('[data-testid="match-screen"]')?.firstElementChild;
        w.__samples.push({
          turn: active?.dataset.testid ?? '',
          seats,
          board: Math.round(board?.getBoundingClientRect().height ?? 0),
        });
      }
      requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);
  });

  // Play by drawing: it keeps every turn short and never ends the deal early.
  const end = Date.now() + 25_000;
  while (Date.now() < end) {
    for (const id of ['offer-accept', 'offer-pass', 'offer-draw']) {
      const control = page.getByTestId(id);
      if ((await control.count()) && (await control.getAttribute('aria-disabled')) !== 'true') {
        await control.click({ timeout: 1000 }).catch(() => undefined);
        break;
      }
    }
    await page.waitForTimeout(500);
  }

  const samples = await page.evaluate(
    () => (window as unknown as { __samples: { turn: string; seats: Record<string, number>; board: number }[] }).__samples,
  );
  const turns = samples.filter((s, i) => i > 0 && s.turn !== samples[i - 1].turn).length;
  expect(turns, 'the turn went round several times while sampling').toBeGreaterThanOrEqual(6);

  // A seat's own content can change (a "last card" call adds a line), so the
  // check is narrower than "never": its height does not follow the turn.
  const bySeatAndTurn = new Map<string, Set<number>>();
  for (const s of samples) {
    for (const [seat, h] of Object.entries(s.seats)) {
      const key = `${seat} while ${s.turn === `seat-active-${seat.slice('seat-'.length)}` ? 'to play' : 'waiting'}`;
      bySeatAndTurn.set(key, (bySeatAndTurn.get(key) ?? new Set()).add(h));
    }
  }
  for (const seat of new Set(samples.flatMap((s) => Object.keys(s.seats)))) {
    const playing = [...(bySeatAndTurn.get(`${seat} while to play`) ?? [])];
    const waiting = [...(bySeatAndTurn.get(`${seat} while waiting`) ?? [])];
    if (!playing.length || !waiting.length) continue;
    expect(
      playing.some((h) => waiting.includes(h)),
      `${seat} is the same height on its turn (${playing}) as off it (${waiting})`,
    ).toBe(true);
  }

  let tallest = 0;
  for (const s of samples) {
    expect(s.board, 'the board never gets shorter within the deal').toBeGreaterThanOrEqual(tallest);
    tallest = Math.max(tallest, s.board);
  }
});
