import { expect, test } from '@playwright/test';

import { dragLocatorTo } from '../helpers/drag';
import { cardByCode, selectedCodes, selectOnly } from '../helpers/hand';
import { Seat, guest, move, newTable, watchAs } from '../helpers/seats';

/**
 * Two things a player does with their own hands, in the browser, at a real
 * Last Card table:
 *
 *  - A wild asks which colour it names — whether it is dragged onto the pile
 *    or picked and played with the button. A drag has no control beside it
 *    to answer on, so the question comes up as a sheet; the answer is what
 *    the server gets, and the card on the pile wears it.
 *  - A drawn card is picked for the player while the turn is theirs, and let
 *    go when the turn ends — a draw that ended it, or a card drawn and kept.
 *
 * Ada plays in the browser; Bo is played over a socket. The deal is whatever
 * it is, so the test plays on — tables dealt again if need be — until each
 * case has come up and been checked.
 */

const isWild = (c: string) => c === 'W' || c === 'W4';
const topCard = '[data-testid^="card-discard-"]:not([data-testid*="concealed"])';

test('a wild asks for its colour however it is played, and a drawn card is let go when the turn ends', async ({
  page,
  request,
}) => {
  test.setTimeout(420_000);
  const users = [await guest(request, 'Ada'), await guest(request, 'Bo')];
  const [A] = users;
  const seen = new Set<string>();
  const required = ['dragAsked', 'pressAsked', 'drawEndedTurn', 'keptLetGo'];

  for (let table = 0; table < 10 && required.some((r) => !seen.has(r)); table++) {
    const matchId = await newTable(request, 'lastcard', users, {
      targetScore: 0,
      lastCardCall: 0,
      drawFourChallenge: 0,
    });
    await watchAs(page, A, matchId);
    const seats = users.map((u) => (u === A ? new Seat(matchId, u, page, request) : new Seat(matchId, u)));
    await Promise.all(seats.map((s) => s.ready()));
    const [ada, bo] = seats;

    for (let step = 0; step < 300; step++) {
      const seat = await nextToMove(seats);
      if (!seat) break;

      if (seat === bo) {
        // Bo keeps his wilds for nothing: plain cards only, else draw.
        const play = bo.enabled('play_card');
        const card = (play?.source?.cards ?? []).find((c: string) => !isWild(c));
        if (card) await move(seats, bo, { offerId: 'play_card', verb: 'play_card', cards: [card] });
        else if (bo.enabled('pass')) await move(seats, bo, { offerId: 'pass', verb: 'pass' });
        else await move(seats, bo, { offerId: 'draw', verb: 'draw' });
        continue;
      }

      const play = ada.enabled('play_card');
      const cards: string[] = play?.source?.cards ?? [];
      const wild = cards.find(isWild);
      const colour = wild === 'W4' ? 'A' : 'T';

      if (wild && (!seen.has('dragAsked') || !seen.has('pressAsked'))) {
        const byDrag = !seen.has('dragAsked');
        if (byDrag) {
          await dragLocatorTo(page, cardByCode(page, wild), page.locator(topCard).last());
        } else {
          await selectOnly(page, [wild]);
          await page.getByTestId('offer-play_card').click();
        }
        // The question comes up, and nothing has gone yet.
        await expect(page.getByTestId('choice-colour')).toBeVisible({ timeout: 10_000 });
        await ada.refresh();
        expect(ada.latest.view.zones.find((z: any) => z.id === 'discard').cards.at(-1).card).not.toBe(wild);
        await page.getByTestId(`choice-colour-${colour}`).click();
        await expect(page.getByTestId('choice-colour')).toHaveCount(0);
        // The answer is what the server got, and the card on the pile wears it.
        await expect
          .poll(async () => {
            await ada.refresh();
            const top = ada.latest.view.zones.find((z: any) => z.id === 'discard').cards.at(-1);
            return `${top.card}:${top.as ?? ''}`;
          })
          .toBe(`${wild}:${colour}`);
        seen.add(byDrag ? 'dragAsked' : 'pressAsked');
        continue;
      }

      // A plain card goes by the button, so no question is asked of it.
      const plain = cards.find((c) => !isWild(c));
      if (plain) {
        await selectOnly(page, [plain]);
        await page.getByTestId('offer-play_card').click();
        await expect(page.getByTestId('choice-colour')).toHaveCount(0);
        const held = ada.latest.view.zones.find((z: any) => z.ownerId === A.userId).count;
        await expect
          .poll(async () => (await ada.refresh(), ada.latest.view.zones.find((z: any) => z.ownerId === A.userId).count))
          .toBeLessThan(held);
        continue;
      }

      // Nothing to play: draw.
      const before = ada.latest.view.zones.find((z: any) => z.ownerId === A.userId).count;
      await page.getByTestId('offer-draw').click();
      await expect.poll(async () => (await ada.refresh(), ada.latest.view.zones.find((z: any) => z.ownerId === A.userId).count)).toBeGreaterThan(before);
      if (ada.enabled('pass')) {
        // The drawn card fits: it is picked for her while the turn is hers…
        await expect.poll(() => selectedCodes(page)).toHaveLength(1);
        // …and let go once she keeps it and the turn moves on.
        await page.getByTestId('offer-pass').click();
        await expect.poll(() => selectedCodes(page), { timeout: 10_000 }).toEqual([]);
        seen.add('keptLetGo');
      } else {
        // The draw ended the turn: the card is not left picked.
        await page.waitForTimeout(600);
        expect(await selectedCodes(page)).toEqual([]);
        seen.add('drawEndedTurn');
      }
    }
    for (const s of seats) s.close();
  }
  console.log('checked:', [...seen].sort().join(', '));
  expect(required.filter((r) => !seen.has(r)), 'cases that never came up').toEqual([]);
});

/**
 * Whoever the table is waiting on — polled for a moment, since the seat that
 * just moved in the browser and the one on a socket hear about it at
 * slightly different times. Null once the deal is over.
 */
async function nextToMove(seats: Seat[]): Promise<Seat | null> {
  for (let i = 0; i < 60; i++) {
    for (const s of seats) await s.refresh();
    if (seats.some((s) => s.latest?.status && s.latest.status !== 'active')) return null;
    const seat = seats.find((s) => (s.latest?.legalActions ?? []).some((o: any) => o.enabled));
    if (seat) return seat;
    await new Promise((r) => setTimeout(r, 50));
  }
  return null;
}
