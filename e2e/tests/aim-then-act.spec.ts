import { expect, test, type Page } from '@playwright/test';

import { handCards } from '../helpers/drag';
import { API_BASE } from '../helpers/env';
import { clearHandSelection, selectOnly } from '../helpers/hand';

/**
 * Playing a card by picking the target first, then the cards, then pressing
 * the control — the other order from `tap-to-play.spec.ts`'s "select cards,
 * then tap where they go".
 *
 * Both orders end at the same offer through the same submission; what
 * differs is which is chosen first. Tapping a meld with *nothing* selected
 * used to either do nothing (a lone offer's own button ignores a bare meld
 * tap) or, for a folded control with more than one target sharing a label,
 * hit `onAmbiguous` and send whatever the offer's own list defaulted to —
 * a card nobody chose. Now it arms that meld as the target and sends nothing
 * until a selection actually settles it, which is the fix for both: aiming a
 * folded "Lay off" at a specific meld before any card is picked, and never
 * guessing which of several eligible cards was meant.
 */

type Ctx = import('@playwright/test').APIRequestContext;

async function tableWithBots(request: Ctx, moduleId: string, seats: number) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `aim-${Math.random().toString(36).slice(2, 10)}` },
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
  // Wide enough to hold a four-handed board and the hand at once, matching
  // `tap-to-play.spec.ts` — arming is board-and-offer logic, not adaptive
  // layout, so it needs no narrower a viewport than that suite already trusts.
  await page.setViewportSize({ width: 1280, height: 1400 });
  await page.addInitScript(
    (s) => {
      window.localStorage.setItem('zolik_session', JSON.stringify(s));
    },
    {
      accessToken: host.accessToken,
      refreshToken: host.refreshToken,
      userId: host.userId,
      username: 'aim',
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

async function meldGroups(request: Ctx, matchId: string, userId: string): Promise<Record<string, string[]>> {
  const body = await board(request, matchId, userId);
  const out: Record<string, string[]> = {};
  for (const z of body.view?.zones ?? []) {
    for (const g of z.groups ?? []) out[g.id] = g.cards ?? [];
  }
  return out;
}

async function serverHand(request: Ctx, matchId: string, userId: string): Promise<string[]> {
  const body = await board(request, matchId, userId);
  const zone = (body.view?.zones ?? []).find((z: any) => z.kind === 'hand' && z.ownerId === userId);
  return (zone?.cards ?? []).map((c: any) => c.card);
}

/** A live lay-off: one that names a meld and lists cards it would take — see `drag-and-drop.spec.ts`'s own `layOffOffer` for why the verb is checked rather than just the shape. */
async function layOffOffer(request: Ctx, matchId: string, userId: string) {
  const body = await board(request, matchId, userId);
  return (
    (body.legalActions ?? []).find(
      (o: any) => o.enabled && o.verb === 'lay_off' && o.target?.meldId && (o.source?.cards ?? []).length > 0,
    ) ?? null
  );
}

/** Presses whatever is live until `verb` becomes offered — see `tap-to-play.spec.ts` for why this waits rather than giving up. */
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
    const body = await board(request, matchId, userId);
    const offer = (body.legalActions ?? []).find((o: any) => o.verb === verb && o.enabled);
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

/** The control that sends a resolved offer: a folded group control if one exists for it, otherwise its own lone button. */
function controlFor(page: Page, offer: any) {
  const groupKey = offer.labelKey ?? `verb.${offer.verb}`;
  return page.getByTestId(`offer-group:${groupKey}`).or(page.getByTestId(`offer-${offer.id}`));
}

/**
 * A clean slate before aiming: nothing selected, and no *other* ambiguous
 * "which of these melds did you mean" still pending from earlier.
 *
 * `playUntilOffered` presses whatever is live to get here, which can include
 * a folded control that turned out ambiguous — the same recovery a real
 * player touching their hand gets for free, since any tap on a card clears
 * it. `clearHandSelection` alone only touches a card if one is already
 * selected, which would leave a *pending-but-unselected* ambiguity untouched,
 * so the deselect-then-reselect-then-deselect below guarantees the touch
 * happens regardless of what, if anything, was selected to begin with.
 */
async function resetPending(page: Page) {
  await clearHandSelection(page);
  const first = page.locator('[data-testid^="card-hand:"]').first();
  await first.click();
  await first.click();
  await expect(page.locator('[data-testid^="card-hand:"][aria-selected="true"]')).toHaveCount(0);
}

test.describe('aiming at a target before choosing the cards', () => {
  test('a meld tapped with nothing selected arms it and sends nothing', async ({ page, request }) => {
    test.setTimeout(150_000);
    const { matchId, host } = await tableWithBots(request, 'canasta', 4);
    await openMatch(page, host, matchId);
    await handCards(page);

    const offer = await playUntilOffered(page, request, matchId, host.userId, 'lay_off');
    test.skip(!offer, 'no lay-off came up before the deal moved on');

    const meldId = offer.target.meldId as string;
    const before = await meldGroups(request, matchId, host.userId);
    const handBefore = await serverHand(request, matchId, host.userId);

    await resetPending(page);

    const toggle = page.getByTestId(`group-toggle-${meldId}`);
    await toggle.click();
    await expect(toggle).toHaveAttribute('aria-selected', 'true');

    // Nothing was sent: the board and the hand are exactly as they were.
    await page.waitForTimeout(500);
    expect(await meldGroups(request, matchId, host.userId)).toEqual(before);
    expect(await serverHand(request, matchId, host.userId)).toEqual(handBefore);
  });

  test('arm the meld, pick the cards, press the control — it lands only there', async ({ page, request }) => {
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
    const chosen = before.find((c) => eligible.includes(c));
    expect(chosen, 'the hand holds a card this lay-off accepts').toBeTruthy();

    // The meld, then the cards — the order the folded button's own hint
    // could not resolve on its own before arming existed.
    await resetPending(page);
    await page.getByTestId(`group-toggle-${meldId}`).click();
    await selectOnly(page, [chosen!]);

    const control = controlFor(page, offer);
    await expect(control).toBeEnabled();
    await control.click();

    await expect
      .poll(async () => (await meldGroups(request, matchId, host.userId))[meldId]?.length ?? 0, {
        timeout: 10_000,
      })
      .toBe((meldsBefore[meldId]?.length ?? 0) + 1);

    const after = await meldGroups(request, matchId, host.userId);
    expect(after[meldId]).toContain(chosen);
    // Every other meld on the table — including any other one the same
    // control could have meant — is untouched.
    for (const [id, cards] of Object.entries(meldsBefore)) {
      if (id === meldId) continue;
      expect(after[id]).toEqual(cards);
    }
  });

  test('cards first still sends in one tap, arming or no arming', async ({ page, request }) => {
    test.setTimeout(150_000);
    const { matchId, host } = await tableWithBots(request, 'canasta', 4);
    await openMatch(page, host, matchId);
    await handCards(page);

    const offer = await playUntilOffered(page, request, matchId, host.userId, 'lay_off');
    test.skip(!offer, 'no lay-off came up before the deal moved on');

    const meldId = offer.target.meldId as string;
    const eligible = (offer.source?.cards ?? []) as string[];
    const meldsBefore = await meldGroups(request, matchId, host.userId);
    const before = await serverHand(request, matchId, host.userId);
    const chosen = before.find((c) => eligible.includes(c));
    expect(chosen).toBeTruthy();

    await selectOnly(page, [chosen!]);
    await page.getByTestId(`group-press-${meldId}`).click();

    await expect
      .poll(async () => (await meldGroups(request, matchId, host.userId))[meldId]?.length ?? 0, {
        timeout: 10_000,
      })
      .toBe((meldsBefore[meldId]?.length ?? 0) + 1);
  });
});
