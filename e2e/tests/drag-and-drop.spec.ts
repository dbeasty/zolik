import { expect, test, type Page } from '@playwright/test';

import { dragLocatorTo, handCards } from '../helpers/drag';
import { API_BASE } from '../helpers/env';

/**
 * Playing a card by dragging it onto the board.
 *
 * The claim under test is not "a card can be dragged" — it is that the shell
 * works out *where a card may be dropped* from the offer list alone, and so
 * does it for every game without being told about any of them. The screen this
 * replaced knew that a card dropped on the pile was a discard and a card
 * dropped on a meld was a lay-off; those were rummy rules restated on the
 * client, where they could disagree with the server.
 *
 * So these run the same gesture against two games that share no rule, and
 * check the *server's* copy of the board afterwards rather than the screen's —
 * a client that only moved a card in its own head would pass every visual
 * assertion.
 *
 * Cards are chosen by asking the server which ones an offer accepts, and
 * located by index into the hand it sent. The rendered order matches it as
 * long as nothing has been rearranged, and nothing here rearranges: what a
 * card looks like on screen ("7♥") is not what it is called on the wire
 * ("7H"), and teaching this file to translate would be teaching it what a suit
 * is.
 */

type Ctx = import('@playwright/test').APIRequestContext;

async function tableWithBots(request: Ctx, moduleId: string, seats = 2) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `dnd-${Math.random().toString(36).slice(2, 10)}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  const host = await res.json();
  const auth = { Authorization: `Bearer ${host.accessToken}` };

  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    data: { moduleId, options: {} },
  });
  expect(created.ok(), await created.text()).toBeTruthy();
  const { matchId } = await created.json();

  for (let i = 1; i < seats; i++) {
    await request.post(`${API_BASE}/matches/${matchId}/add-bot`, { headers: auth });
  }
  await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });
  return { matchId, host };
}

async function openMatch(page: Page, host: any, matchId: string) {
  // Tall enough to hold a four-handed board and the hand at once. A drag is
  // two points on one screen: `dragLocatorTo` deliberately does not scroll the
  // target into view, because scrolling to it would move the source out from
  // under the pointer, so both have to be visible together.
  await page.setViewportSize({ width: 1280, height: 1400 });
  await page.addInitScript((s) => {
    window.localStorage.setItem('zolik_session', JSON.stringify(s));
  }, {
    accessToken: host.accessToken,
    refreshToken: host.refreshToken,
    userId: host.userId,
    username: 'dnd',
    isGuest: true,
  });
  await page.goto(`/match/${matchId}`);
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
}

async function board(request: Ctx, matchId: string, userId: string) {
  const res = await request.get(`${API_BASE}/matches/${matchId}?as=${userId}`);
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

async function serverHand(request: Ctx, matchId: string, userId: string): Promise<string[]> {
  const body = await board(request, matchId, userId);
  const zone = (body.view?.zones ?? []).find(
    (z: any) => z.kind === 'hand' && z.ownerId === userId,
  );
  return (zone?.cards ?? []).map((c: any) => c.card);
}

/** Every meld group on the table, by the id an offer would name it under. */
async function meldGroups(
  request: Ctx,
  matchId: string,
  userId: string,
): Promise<Record<string, string[]>> {
  const body = await board(request, matchId, userId);
  const out: Record<string, string[]> = {};
  for (const z of body.view?.zones ?? []) {
    for (const g of z.groups ?? []) out[g.id] = g.cards ?? [];
  }
  return out;
}

/**
 * A live lay-off: one that names a meld and lists cards it would take.
 *
 * The verb is checked, not just the shape. Canasta's capture of the discard
 * pile also names a meld as its target and also lists hand cards as its
 * source, so "an enabled offer with a target meld" finds two different moves —
 * and this test is about the one that puts a card from your hand onto a meld.
 */
async function layOffOffer(request: Ctx, matchId: string, userId: string) {
  const body = await board(request, matchId, userId);
  return (
    (body.legalActions ?? []).find(
      (o: any) =>
        o.enabled && o.verb === 'lay_off' && o.target?.meldId && (o.source?.cards ?? []).length > 0,
    ) ?? null
  );
}

/** The enabled offer for a verb, and the cards it says it will take. */
async function offerFor(request: Ctx, matchId: string, userId: string, verb: string) {
  const body = await board(request, matchId, userId);
  const offer = (body.legalActions ?? []).find((o: any) => o.verb === verb && o.enabled);
  return { offer, cards: (offer?.source?.cards ?? []) as string[] };
}

const card = (page: Page, i: number) => page.locator('[data-testid^="card-hand:"]').nth(i);

/**
 * The hand with one copy of a card gone.
 *
 * One copy, not every match: two decks are in play in Žolíky, so a hand can
 * hold "7H" twice and playing one of them must leave the other.
 */
function withoutOne(hand: string[], card: string): string[] {
  const at = hand.indexOf(card);
  return at < 0 ? hand : [...hand.slice(0, at), ...hand.slice(at + 1)];
}

/**
 * Presses whatever is live until `verb` becomes available.
 *
 * Waits rather than giving up when nothing is pressable, because "nothing is
 * pressable" usually means it is a bot's turn and the turn is coming back. An
 * earlier version returned immediately in that case, which made a test that
 * only ran when the deal happened to start on the human — it skipped silently
 * about half the time, and a test that skips is a test that is not run.
 */
async function playUntilOffered(
  page: Page,
  request: Ctx,
  matchId: string,
  userId: string,
  verb: string,
  budgetMs = 30_000,
) {
  const deadline = Date.now() + budgetMs;
  while (Date.now() < deadline) {
    const { offer } = await offerFor(request, matchId, userId, verb);
    if (offer) return offer;
    const live = page.locator('[data-testid^="offer-"]:not([aria-disabled="true"])').first();
    if (await live.count()) {
      try {
        await live.click({ timeout: 5000 });
      } catch {
        /* the board moved under us; the next pass re-reads it */
      }
    }
    await page.waitForTimeout(500);
  }
  return null;
}

test.describe('dropping a card on the board', () => {
  test('a Prsi card dragged onto the discard pile is played, on the server', async ({
    page,
    request,
  }) => {
    const { matchId, host } = await tableWithBots(request, 'prsi');
    await openMatch(page, host, matchId);
    await handCards(page);

    const offer = await playUntilOffered(page, request, matchId, host.userId, 'play_card');
    test.skip(!offer, 'never reached a turn where a card could be played');

    const { cards: playable } = await offerFor(request, matchId, host.userId, 'play_card');
    const before = await serverHand(request, matchId, host.userId);
    const index = before.findIndex((c) => playable.includes(c));
    expect(index).toBeGreaterThanOrEqual(0);

    const dragged = before[index];
    await dragLocatorTo(page, card(page, index), page.getByTestId('zone-discard'));

    // The server is the witness, and specifically that *this* card left — a
    // hand that merely got shorter could have got shorter for another reason.
    await expect
      .poll(async () => (await serverHand(request, matchId, host.userId)).join(','), {
        timeout: 10_000,
      })
      .toBe(withoutOne(before, dragged).join(','));
  });

  test('a Zolik card dragged onto the discard pile is discarded, sharing no rule with Prsi', async ({
    page,
    request,
  }) => {
    const { matchId, host } = await tableWithBots(request, 'zolik');
    await openMatch(page, host, matchId);
    await handCards(page);

    // Rummy makes you draw before you may discard. This test does not know
    // that — it presses whatever control is live until the engine offers
    // discarding, which is the same way it would find out in any other game.
    const offer = await playUntilOffered(page, request, matchId, host.userId, 'discard');
    test.skip(!offer, 'never reached a position where discarding was legal');

    const { cards: discardable } = await offerFor(request, matchId, host.userId, 'discard');
    const before = await serverHand(request, matchId, host.userId);
    const index = before.findIndex((c) => discardable.includes(c));
    expect(index).toBeGreaterThanOrEqual(0);

    const dragged = before[index];
    await dragLocatorTo(page, card(page, index), page.getByTestId('zone-discard'));

    await expect
      .poll(async () => (await serverHand(request, matchId, host.userId)).join(','), {
        timeout: 10_000,
      })
      .toBe(withoutOne(before, dragged).join(','));
  });

  test('a Canasta card dragged onto a meld extends that meld and not another', async ({
    page,
    request,
  }) => {
    // Longer than the suite default, and deliberately so: this one has to play
    // a real hand of Canasta far enough for a partnership to have melded
    // before there is anything to drop a card on. The alternative — asserting
    // against a position dealt by hand — would prove the drop worked against a
    // board no game had actually produced.
    test.setTimeout(150_000);

    // Four seats, so the human has a bot partner: a lay-off goes onto the
    // *partnership's* melds, and in a two-player game every meld on the table
    // belongs to the opponent. That is a Canasta rule the shell does not know
    // — it is here only to reach a position where a meld can be dropped on.
    const { matchId, host } = await tableWithBots(request, 'canasta', 4);
    await openMatch(page, host, matchId);
    await handCards(page);

    // Reaching a lay-off takes a while and cannot be asked for: the
    // partnership has to have melded (the bot partner does that on its own),
    // and the human has to be holding a card of a rank already on the table.
    // So this plays the hand — pressing whatever is live, waiting through the
    // bots' turns — until the engine offers one.
    let offer: any = null;
    const deadline = Date.now() + 90_000;
    while (Date.now() < deadline && !offer) {
      offer = await layOffOffer(request, matchId, host.userId);
      if (offer) break;
      const live = page.locator('[data-testid^="offer-"]:not([aria-disabled="true"])').first();
      if (await live.count()) {
        try {
          await live.click({ timeout: 5000 });
        } catch {
          /* the board moved under us; the next pass re-reads it */
        }
      }
      await page.waitForTimeout(500);
    }
    test.skip(!offer, 'no lay-off came up before the deal moved on');

    const meldId = offer.target.meldId as string;
    const eligible = (offer.source?.cards ?? []) as string[];
    const before = await serverHand(request, matchId, host.userId);
    const meldsBefore = await meldGroups(request, matchId, host.userId);
    const index = before.findIndex((c) => eligible.includes(c));
    expect(index).toBeGreaterThanOrEqual(0);
    const dragged = before[index];

    // Dropped on the *group*, not on the zone that holds it. A partnership has
    // several melds in one spread, and which one the card joins is decided by
    // where the pointer let go.
    await dragLocatorTo(page, card(page, index), page.getByTestId(`group-${meldId}`));

    await expect
      .poll(async () => (await meldGroups(request, matchId, host.userId))[meldId]?.length ?? 0, {
        timeout: 10_000,
      })
      .toBe((meldsBefore[meldId]?.length ?? 0) + 1);

    const after = await meldGroups(request, matchId, host.userId);
    expect(after[meldId]).toContain(dragged);
    // Every other meld on the table is untouched — the drop meant one of them.
    for (const [id, cards] of Object.entries(meldsBefore)) {
      if (id === meldId) continue;
      expect(after[id]).toEqual(cards);
    }
  });

  test('a card the engine will not take is refused, and goes back where it came from', async ({
    page,
    request,
  }) => {
    const { matchId, host } = await tableWithBots(request, 'zolik');
    await openMatch(page, host, matchId);
    const shown = await handCards(page);

    // Rummy refuses every discard until you have drawn, so on this first turn
    // nothing in hand has a home. This is the half of the feature that is
    // about *not* acting, and the half a happy-path test would miss.
    const { offer } = await offerFor(request, matchId, host.userId, 'discard');
    test.skip(!!offer, 'this deal allowed an immediate discard');

    const before = await serverHand(request, matchId, host.userId);
    await dragLocatorTo(page, card(page, 0), page.getByTestId('zone-discard'));
    await page.waitForTimeout(800);

    expect(await serverHand(request, matchId, host.userId)).toEqual(before);
    // And it is back in its own place, not quietly rearranged as a
    // consolation prize for a drop that was refused.
    expect(await handCards(page)).toEqual(shown);
  });
});
