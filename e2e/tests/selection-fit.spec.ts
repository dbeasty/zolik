import { expect, test, type Page } from '@playwright/test';

import { handCards, tapCard } from '../helpers/drag';
import { API_BASE } from '../helpers/env';
import { selectedCodes } from '../helpers/hand';
import { waitForOfferEnabled } from '../helpers/turn';

/**
 * A control refuses a selection it cannot send, rather than sending a
 * trimmed or guessed version of it.
 *
 * The bug this guards against: a discard takes exactly one card, and picking
 * two used to leave the button enabled — pressing it discarded whichever card
 * a `slice` happened to keep, silently, while the other stayed selected
 * looking chosen when it had already been spent. `src/lib/drops.ts`'s `fits`
 * is the fix; this is the one thing only a browser can check, that the
 * *rendered* control actually goes dark and says why.
 */

type Ctx = import('@playwright/test').APIRequestContext;

async function tableWithBots(request: Ctx, moduleId: string, seats = 2) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `fit-${Math.random().toString(36).slice(2, 10)}` },
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

/**
 * A two-handed Samba table seeded straight into "this side has its canastas,
 * and this player's whole hand is black threes".
 *
 * Seeded rather than played toward: only six black threes exist in the whole
 * Samba shoe, split across every seat, so a hand boiling down to exactly
 * those six is far rarer than a retry loop could stand — the same reasoning,
 * and the same `debug-state` blob shape, as
 * `canasta-black-three-going-out.spec.ts`, which explains the fields at
 * length. The second seat is a real guest who never gets a turn.
 */
async function sambaGoingOutTable(request: Ctx) {
  const guest = async () => {
    const res = await request.post(`${API_BASE}/auth/guest`, {
      data: { guestName: `fit-${Math.random().toString(36).slice(2, 10)}` },
    });
    expect(res.ok(), await res.text()).toBeTruthy();
    return res.json();
  };
  const host = await guest();
  const other = await guest();
  const auth = { Authorization: `Bearer ${host.accessToken}` };

  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    // High enough that going out cannot end the match — the point is the
    // control, not the scoreboard.
    data: { moduleId: 'canasta', variation: 'samba', options: { targetScore: 10000 } },
  });
  expect(created.ok(), await created.text()).toBeTruthy();
  const { matchId } = await created.json();

  const joined = await request.post(`${API_BASE}/matches/${matchId}/join`, {
    headers: { Authorization: `Bearer ${other.accessToken}` },
  });
  expect(joined.ok(), await joined.text()).toBeTruthy();
  const started = await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });
  expect(started.ok(), await started.text()).toBeTruthy();

  // Two natural canastas of ranks nothing here can confuse with a three, so
  // the side already qualifies to go out (Samba needs two).
  const melds = ['4', '5'].map((rank) => ({
    id: `t0-${rank}`,
    teamId: 0,
    rank,
    cards: [`${rank}H`, `${rank}D`, `${rank}S`, `${rank}C`, `${rank}H`, `${rank}D`, `${rank}S`],
  }));

  const seeded = await request.post(`${API_BASE}/matches/${matchId}/debug-state`, {
    headers: auth,
    data: {
      state: {
        status: 'active',
        variation: 'samba',
        players: [host.userId, other.userId],
        turnOrder: [host.userId, other.userId],
        current: host.userId,
        // Already drawn, so nothing lands pre-picked and every selection
        // below is the player's own.
        phase: 'meld',
        teams: [
          { id: 0, players: [host.userId], score: 0, melds, redThrees: [], hasMelded: true },
          { id: 1, players: [other.userId], score: 0, melds: [], redThrees: [], hasMelded: false },
        ],
        teamOf: { [host.userId]: 0, [other.userId]: 1 },
        drawPile: Array(20).fill('8S'),
        discardPile: ['9H'],
        hands: {
          [host.userId]: ['3C', '3C', '3C', '3S', '3S', '3S'],
          [other.userId]: ['8H', '9H', 'TH'],
        },
        frozen: false,
        laidThisTurn: 0,
        meldsAtTurnStart: true,
        tookPileThisTurn: false,
        handSize: 15,
        targetScore: 10000,
        canastasToGoOut: 2,
        dealNumber: 0,
        dealer: 0,
        pause: true,
        openDiscard: false,
        winnerTeam: 0,
        seed: 1,
      },
    },
  });
  expect(seeded.ok(), await seeded.text()).toBeTruthy();

  return { matchId, host };
}

async function openMatch(page: Page, host: any, matchId: string) {
  await page.addInitScript(
    (s) => {
      window.localStorage.setItem('zolik_session', JSON.stringify(s));
    },
    {
      accessToken: host.accessToken,
      refreshToken: host.refreshToken,
      userId: host.userId,
      username: 'fit',
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

/**
 * How a control is painted: whether it is filled, and how much room it takes.
 *
 * A control that cannot be pressed is drawn as the outline of a button rather
 * than a faded one, so the filled controls on screen are exactly the ones that
 * can be pressed. The size is measured alongside because that is the half of
 * the rule that is easy to break: a border that appeared only on greying out
 * would reflow the whole row every time an offer came and went.
 */
async function paint(page: Page, testId: string) {
  return page.evaluate((id) => {
    const el = document.querySelector(`[data-testid="${id}"]`) as HTMLElement;
    const cs = getComputedStyle(el);
    const r = el.getBoundingClientRect();
    return {
      fill: cs.backgroundColor,
      border: cs.borderTopWidth,
      stroke: cs.borderTopStyle,
      width: Math.round(r.width),
      height: Math.round(r.height),
    };
  }, testId);
}

test.describe('a control refuses what it cannot send', () => {
  test('discard greys out and says why when two cards are picked, and comes back for one', async ({
    page,
    request,
  }) => {
    test.setTimeout(60_000);
    const { matchId, host } = await tableWithBots(request, 'zolik');
    await openMatch(page, host, matchId);
    await handCards(page);

    // A turn starts with a draw; the drawn card lands selected on its own —
    // exactly one, which is what discard takes.
    await waitForOfferEnabled(page, 'offer-draw:deck');
    await page.getByTestId('offer-draw:deck').click();
    const discard = page.getByTestId('offer-discard');
    await expect(discard).not.toHaveAttribute('aria-disabled', 'true');
    await expect(page.getByTestId('why-discard')).toHaveCount(0);
    const pressable = await paint(page, 'offer-discard');
    const handSize = (await serverHand(request, matchId, host.userId)).length;

    // The drawn card is picked *for* the player, not *by* them — touching a
    // different card replaces it rather than joining it, so the drawn card
    // they actually mean to keep has to be chosen first before a second one
    // can be added to it.
    const unselected = page.locator('[data-testid^="card-hand:"]:not([aria-selected="true"])');
    await tapCard(page, unselected.first());
    await expect(page.locator('[data-testid^="card-hand:"][aria-selected="true"]')).toHaveCount(1);
    await tapCard(page, unselected.first());
    await expect(page.locator('[data-testid^="card-hand:"][aria-selected="true"]')).toHaveCount(2);

    await expect(discard).toHaveAttribute('aria-disabled', 'true');
    await expect(page.getByTestId('why-discard')).toHaveText('Select just one card');

    // And it is drawn as the outline of a button rather than a faded one, in a
    // box the same size: the outline is carried by every control at every
    // moment and only its colour changes, so the row does not shift under the
    // thumb that is still picking cards. (The slot does widen here — the
    // reason line below the button is longer than the button — which is why
    // this pins the height and the border rather than the width.)
    const refusing = await paint(page, 'offer-discard');
    expect(refusing.fill, 'a control that cannot be pressed should not be filled').toBe(
      'rgba(0, 0, 0, 0)',
    );
    expect(pressable.fill, 'a control that can be pressed should be filled').not.toBe(
      'rgba(0, 0, 0, 0)',
    );
    expect({ h: refusing.height, border: refusing.border }).toEqual({
      h: pressable.height,
      border: pressable.border,
    });
    // Dashed here, where the browser draws it properly at a rounded corner —
    // it says "not a real button yet" more plainly than any colour can. Native
    // gets the same outline solid; see the note on `ghost` in `OfferBar`.
    expect(refusing.stroke, 'a refusing control should read as a dashed outline').toBe('dashed');
    expect(pressable.stroke).toBe('solid');

    // The server never saw a discard for either card.
    const untouched = await serverHand(request, matchId, host.userId);
    expect(untouched.length).toBe(handSize);

    // Deselecting one brings it back.
    await tapCard(page, page.locator('[data-testid^="card-hand:"][aria-selected="true"]').first());
    await expect(discard).not.toHaveAttribute('aria-disabled', 'true');
    await expect(page.getByTestId('why-discard')).toHaveCount(0);
    expect((await paint(page, 'offer-discard')).fill, 'and it fills back in').toBe(pressable.fill);

    // And it sends exactly the one card that stayed selected — never a
    // different, guessed one.
    const before = await serverHand(request, matchId, host.userId);
    const [stillSelected] = await selectedCodes(page);
    await discard.click();

    await expect.poll(async () => (await serverHand(request, matchId, host.userId)).length).toBe(
      before.length - 1,
    );
    // Counted, not just "not contains": two decks are in play, so the hand
    // may hold another copy of the same code, and that copy staying behind is
    // correct — one fewer of it is what "exactly this one" actually means.
    const after = await serverHand(request, matchId, host.userId);
    const countOf = (hand: string[], code: string) => hand.filter((c) => c === code).length;
    expect(countOf(after, stillSelected)).toBe(countOf(before, stillSelected) - 1);
  });

  /**
   * The same rule from the other side: too few, rather than too many.
   *
   * `fits` answers "are these cards ones this offer could ever take", and it
   * says so in as many words that it has no opinion on whether there are
   * *enough of them yet*. A drag stages a partial answer and is meant to; a
   * button is not, and asked `fits` alone anyway. So a control whose offer
   * needs six cards stayed filled over a one-card pick, and pressing it did
   * nothing whatsoever: `submissionFor` refuses a submission under the
   * offer's own floor and returns null, and the press died there — no move,
   * no refusal, nothing on screen. `readyWith` is the fix.
   *
   * Six because that is the widest floor any of the four games produces:
   * Samba deals three decks, so a hand can come down to all six black
   * threes, and going out on them is all of them or none (the server side of
   * that is `canasta-black-three-going-out.spec.ts`, which proved the wire
   * honoured this move while the button above it was doing nothing at all —
   * which is exactly how this survived two rounds of fixing).
   */
  test('a meld that takes the whole hand greys out for a partial pick, then sends all six', async ({
    page,
    request,
  }) => {
    test.setTimeout(60_000);
    const { matchId, host } = await sambaGoingOutTable(request);
    await openMatch(page, host, matchId);
    await handCards(page);

    const meld = page.getByTestId('offer-lay_meld:3');
    // Nothing picked yet: the offer names its own submission, so it is one
    // tap, and that path was never the broken one.
    await expect(meld).not.toHaveAttribute('aria-disabled', 'true');
    const pressable = await paint(page, 'offer-lay_meld:3');

    const cards = page.locator('[data-testid^="card-hand:"]');
    await expect(cards).toHaveCount(6);
    await tapCard(page, cards.first());
    await expect(page.locator('[data-testid^="card-hand:"][aria-selected="true"]')).toHaveCount(1);

    await expect(meld).toHaveAttribute('aria-disabled', 'true');
    await expect(page.getByTestId('why-lay_meld:3')).toHaveText('Select 6 card(s)');
    const refusing = await paint(page, 'offer-lay_meld:3');
    expect(refusing.fill, 'a control that cannot be pressed should not be filled').toBe(
      'rgba(0, 0, 0, 0)',
    );
    expect(refusing.stroke).toBe('dashed');
    expect({ h: refusing.height, border: refusing.border }).toEqual({
      h: pressable.height,
      border: pressable.border,
    });

    // Pressing it anyway is the failure this guards: not a refusal, but a
    // move that never left the screen. The hand is still whole afterwards.
    await meld.click({ force: true });
    await page.waitForTimeout(250);
    expect(await serverHand(request, matchId, host.userId)).toHaveLength(6);

    // Picking the rest brings it back, and then it really does go out.
    for (let i = 1; i < 6; i++) await tapCard(page, cards.nth(i));
    await expect(page.locator('[data-testid^="card-hand:"][aria-selected="true"]')).toHaveCount(6);
    await expect(meld).not.toHaveAttribute('aria-disabled', 'true');
    expect((await paint(page, 'offer-lay_meld:3')).fill).toBe(pressable.fill);

    await meld.click();
    await expect
      .poll(async () => (await serverHand(request, matchId, host.userId)).length)
      .toBe(0);
  });
});
