import { expect, test, type Page } from '@playwright/test';

import { handCards } from '../helpers/drag';
import { API_BASE } from '../helpers/env';
import { selectOnly } from '../helpers/hand';

/**
 * Playing a card by selecting it, then tapping where it goes.
 *
 * `drag-and-drop.spec.ts` proves a card can be dragged onto a target; this
 * proves the same targets are reachable without a drag at all — select a
 * card, and every legal place it could land (yours or an opponent's) lights
 * up and takes a tap. On a phone, where an opponent's meld usually sits below
 * a scrolling fold, this is the only realistic way to reach it: the shell
 * derives no more here than it does for a drag — it is told which offers are
 * enabled and where they land, and lights up exactly that.
 */

type Ctx = import('@playwright/test').APIRequestContext;

async function tableWithBots(request: Ctx, moduleId: string, seats: number) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `tap-${Math.random().toString(36).slice(2, 10)}` },
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
  await page.setViewportSize({ width: 1280, height: 1400 });
  await page.addInitScript(
    (s) => {
      window.localStorage.setItem('zolik_session', JSON.stringify(s));
    },
    {
      accessToken: host.accessToken,
      refreshToken: host.refreshToken,
      userId: host.userId,
      username: 'tap',
      isGuest: true,
    },
  );
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
  const zone = (body.view?.zones ?? []).find((z: any) => z.kind === 'hand' && z.ownerId === userId);
  return (zone?.cards ?? []).map((c: any) => c.card);
}

async function meldGroups(request: Ctx, matchId: string, userId: string): Promise<Record<string, string[]>> {
  const body = await board(request, matchId, userId);
  const out: Record<string, string[]> = {};
  for (const z of body.view?.zones ?? []) {
    for (const g of z.groups ?? []) out[g.id] = g.cards ?? [];
  }
  return out;
}

/** A live lay-off onto an existing meld — see `drag-and-drop.spec.ts` for why the verb is checked. */
async function layOffOffer(request: Ctx, matchId: string, userId: string) {
  const body = await board(request, matchId, userId);
  return (
    (body.legalActions ?? []).find(
      (o: any) => o.enabled && o.verb === 'lay_off' && o.target?.meldId && (o.source?.cards ?? []).length > 0,
    ) ?? null
  );
}

/**
 * The hand with one copy of a card gone.
 *
 * One copy, not every match: two decks are in play here, so a hand can hold
 * "AD" twice and playing one of them must leave the other.
 */
function withoutOne(hand: string[], card: string): string[] {
  const at = hand.indexOf(card);
  return at < 0 ? hand : [...hand.slice(0, at), ...hand.slice(at + 1)];
}

async function offerFor(request: Ctx, matchId: string, userId: string, verb: string) {
  const body = await board(request, matchId, userId);
  const offer = (body.legalActions ?? []).find((o: any) => o.verb === verb && o.enabled);
  return { offer, cards: (offer?.source?.cards ?? []) as string[] };
}

/** Presses whatever is live until `verb` becomes available — see `drag-and-drop.spec.ts` for why this waits rather than giving up. */
async function playUntilOffered(
  page: Page,
  request: Ctx,
  matchId: string,
  userId: string,
  verb: string,
  budgetMs = 90_000,
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

const card = (page: Page, i: number) => page.locator('[data-testid^="card-hand:"]').nth(i);

test.describe('playing a card by tapping instead of dragging', () => {
  test('a selected card shows every place it may go, groups it does not belong to stay dark, and a tap plays it', async ({
    page,
    request,
  }) => {
    // Four seats so the human has a bot partner: in Canasta every meld on the
    // table belongs to the partnership, which is exactly the "someone else's
    // meld" shape the report was about — a two-player game would only ever
    // offer a lay-off onto the human's own melds.
    test.setTimeout(150_000);
    const { matchId, host } = await tableWithBots(request, 'canasta', 4);
    await openMatch(page, host, matchId);
    await handCards(page);

    const offer = await playUntilOffered(page, request, matchId, host.userId, 'lay_off');
    test.skip(!offer, 'no lay-off came up before the deal moved on');

    const meldId = offer.target.meldId as string;
    const eligible = (offer.source?.cards ?? []) as string[];
    const before = await serverHand(request, matchId, host.userId);
    const meldsBefore = await meldGroups(request, matchId, host.userId);
    const dragged = before.find((c) => eligible.includes(c));
    expect(dragged, 'the hand holds a card this lay-off accepts').toBeTruthy();

    // Select it — a tap, not a drag — and *only* it.
    //
    // Both halves matter, and this used to do neither. Clicking the n-th card
    // on screen where n was an index into the server's hand clicked a
    // different card entirely, because the fan is in the player's own order,
    // not the server's. And the card drawn on the way here is already
    // selected, so a plain tap would have left two cards picked — an offer
    // that accepts one of them accepts neither pair, and the target this test
    // is about would stay dark for a reason that has nothing to do with what
    // it is testing.
    await selectOnly(page, [dragged!]);

    // Its own target lights up as a pressable overlay...
    const group = page.getByTestId(`group-${meldId}`);
    await expect(group).toBeVisible();
    await expect(page.getByTestId(`group-press-${meldId}`)).toBeVisible();

    // ...and a meld this card does *not* fit does not, proving the highlight
    // is the offer list at work and not "everything lit because something is
    // selected". "Fit" has to be read off the same offer list the screen
    // reads, not assumed: Canasta's capture-the-discard-pile offer also names
    // a meld target and also lists hand cards as its source (see
    // `drag-and-drop.spec.ts`'s own `layOffOffer` comment), so a card that
    // fits exactly one *lay-off* can still legitimately light a second meld
    // through a different offer entirely — that is correct, not a bug, and a
    // meld picked without checking for it is not a fair "should stay dark"
    // example.
    const fullBoard = await board(request, matchId, host.userId);
    const liveMeldIds = new Set(
      (fullBoard.legalActions ?? [])
        .filter((o: any) => o.enabled && o.target?.meldId && (o.source?.minCards ?? 0) > 0)
        .filter((o: any) => {
          const allowed = o.source?.cards ?? [];
          return allowed.length === 0 || allowed.includes(dragged);
        })
        .map((o: any) => o.target.meldId),
    );
    const otherMeldId = Object.keys(meldsBefore).find((id) => id !== meldId && !liveMeldIds.has(id));
    if (otherMeldId) {
      await expect(page.getByTestId(`group-press-${otherMeldId}`)).toHaveCount(0);
    }

    // Tap it.
    await page.getByTestId(`group-press-${meldId}`).click();

    await expect
      .poll(async () => (await meldGroups(request, matchId, host.userId))[meldId]?.length ?? 0, {
        timeout: 10_000,
      })
      .toBe((meldsBefore[meldId]?.length ?? 0) + 1);

    const after = await meldGroups(request, matchId, host.userId);
    expect(after[meldId]).toContain(dragged);
    // Every other meld on the table is untouched.
    for (const [id, cards] of Object.entries(meldsBefore)) {
      if (id === meldId) continue;
      expect(after[id]).toEqual(cards);
    }

    const handAfter = await serverHand(request, matchId, host.userId);
    expect(handAfter.sort()).toEqual(withoutOne(before, dragged).sort());
  });

  test('nothing selected lights nothing up', async ({ page, request }) => {
    const { matchId, host } = await tableWithBots(request, 'zolik', 2);
    await openMatch(page, host, matchId);
    await handCards(page);

    await expect(page.locator('[data-testid^="group-press-"]')).toHaveCount(0);
    await expect(page.locator('[data-testid^="zone-press-"]')).toHaveCount(0);
  });

  test('a card the offer list does not accept there stays put on a tap at an unlit target', async ({
    page,
    request,
  }) => {
    const { matchId, host } = await tableWithBots(request, 'zolik', 2);
    await openMatch(page, host, matchId);
    const cards = await handCards(page);
    test.skip(cards.length === 0, 'no cards in hand');

    // Select any card, then tap the draw pile — never a valid destination
    // for a card already in hand.
    await card(page, 0).click();
    await expect(card(page, 0)).toHaveAttribute('aria-selected', 'true');

    await expect(page.locator('[data-testid="zone-press-draw"]')).toHaveCount(0);

    const before = await serverHand(request, matchId, host.userId);
    await page.getByTestId('zone-draw').click({ force: true });
    await page.waitForTimeout(500);
    const after = await serverHand(request, matchId, host.userId);
    expect(after).toEqual(before);
  });
});
