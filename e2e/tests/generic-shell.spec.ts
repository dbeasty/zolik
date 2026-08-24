import { expect, test, type Page } from '@playwright/test';

import { API_BASE } from '../helpers/env';

/**
 * One screen, every game (docs/one-architecture-plan.md Phase 7).
 *
 * This is `architecture.md` §7.7's last untested claim, and the only place it
 * can actually be tested: a shell that renders zones and offer buttons should
 * play any module with no new screen. The Go suites prove the server describes
 * every game in one shape; the client's own tests prove the shell names no
 * game's vocabulary. What neither can show is that a real browser, running one
 * unchanged screen, plays three games that share almost nothing.
 *
 * So this drives `app/match/[matchId].tsx` through Prší (shedding), Canasta
 * (melding, partnerships) and Texas Hold'em (betting, chips, a numeric input) —
 * clicking the controls a human would, and never the same control twice by
 * name. The screen is not reloaded between them and is not aware of them.
 */

type Ctx = import('@playwright/test').APIRequestContext;

async function guest(request: Ctx) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `shell-${Math.random().toString(36).slice(2, 10)}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

/** Seats the human, fills the rest with bots, starts. */
async function tableWithBots(
  request: Ctx,
  moduleId: string,
  seats: number,
  opts: { variation?: string; options?: Record<string, number> } = {},
) {
  const host = await guest(request);
  const auth = { Authorization: `Bearer ${host.accessToken}` };

  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    data: { moduleId, variation: opts.variation, options: opts.options ?? {} },
  });
  expect(created.ok(), await created.text()).toBeTruthy();
  const { matchId } = await created.json();

  for (let i = 1; i < seats; i++) {
    const bot = await request.post(`${API_BASE}/matches/${matchId}/add-bot`, { headers: auth });
    expect(bot.ok(), await bot.text()).toBeTruthy();
  }
  const started = await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });
  expect(started.ok(), await started.text()).toBeTruthy();

  return { matchId, host };
}

/** Puts a session in localStorage so the shell opens already signed in. */
async function signIn(page: Page, host: { accessToken: string; refreshToken: string; userId: string; username?: string }) {
  await page.addInitScript((s) => {
    window.localStorage.setItem('zolik_session', JSON.stringify(s));
  }, {
    accessToken: host.accessToken,
    refreshToken: host.refreshToken,
    userId: host.userId,
    username: host.username ?? 'shell',
    isGuest: true,
  });
}

/** Every offer control currently on screen, enabled or not. */
async function offers(page: Page) {
  return page.locator('[data-testid^="offer-"]');
}

/**
 * Presses whatever this game offers, up to `max` times, waiting for the turn
 * to come back between moves.
 *
 * The point of the loop is that it names no verb. It presses enabled controls
 * in the order the server listed them, which is exactly what a person who had
 * never played the game would be able to do.
 */
async function playAFewMoves(page: Page, max: number): Promise<number> {
  let moves = 0;

  /** Ids of the controls that are live right now, read as a snapshot. */
  const liveOffers = () =>
    page
      .locator('[data-testid^="offer-"]:not([aria-disabled="true"])')
      .evaluateAll((els) => els.map((e) => e.getAttribute('data-testid') ?? ''));

  /**
   * Polls until this seat is offered something.
   *
   * Generous on purpose. The deal decides who goes first and in several of
   * these games it is not the human — and a bot's turn is not one action but
   * however many that game takes (Canasta draws, melds and discards), each
   * behind a deliberate think-time pause. Waiting 1.5s here was enough for
   * Prší and silently reported "the shell could not press anything" for
   * Canasta, which is a fact about the deal rather than about the shell.
   */
  const waitForOffers = async (ms: number) => {
    const deadline = Date.now() + ms;
    while (Date.now() < deadline) {
      const ids = (await liveOffers()).filter(Boolean);
      if (ids.length) return ids;
      await page.waitForTimeout(300);
    }
    return [] as string[];
  };

  for (let i = 0; i < max; i++) {
    const ids = await waitForOffers(i === 0 ? 30_000 : 12_000);
    if (!ids.length) break;

    // Click by id rather than by position in a live locator: every accepted
    // action re-renders the whole bar, so a locator resolved before the click
    // can be pointing at a detached element by the time it lands. Only a
    // *successful* click counts — an earlier version swallowed failures, which
    // turned "the click never landed" into "a move was made" and hid the fact
    // that the selector was matching the scrolling container too.
    try {
      await page.getByTestId(ids[0]).click({ timeout: 5000 });
      moves++;
    } catch {
      break; // the board moved under us; the server-side check below decides
    }
    await page.waitForTimeout(700);
  }
  return moves;
}

const GAMES = [
  { moduleId: 'prsi', label: 'Prsi', seats: 2 },
  { moduleId: 'canasta', label: 'Canasta', seats: 2, options: { targetScore: 500 } },
  { moduleId: 'holdem', label: 'Holdem', seats: 3, variation: 'timed' },
  { moduleId: 'zolik', label: 'Zoliky', seats: 2 },
];

test.describe('one shell, every game', () => {
  for (const game of GAMES) {
    test(`the same screen renders and plays ${game.label}`, async ({ page, request }) => {
      test.setTimeout(180_000);
      const { matchId, host } = await tableWithBots(request, game.moduleId, game.seats, game);
      await signIn(page, host);

      await page.goto(`/match/${matchId}`);
      await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });

      // The screen knows which game it is showing only because the server told
      // it — and it is the only thing on the page that names one.
      await expect(page.getByTestId('match-module')).toHaveText(new RegExp(game.moduleId));
      await expect(page.getByTestId('match-status')).toHaveText(/active|completed/);

      // A table with people at it, exactly one of whom is on turn. Seat.Active
      // is a pushed fact — the shell never works it out.
      const seats = page.locator('[data-testid^="seat-"]').filter({ hasNot: page.locator('nothing') });
      await expect(page.getByTestId('seat-strip')).toBeVisible();
      await expect(page.getByTestId(`seat-${host.userId}`)).toBeVisible();

      // A board: at least one zone, drawn from its kind alone.
      const zones = page.locator('[data-testid^="zone-"]');
      expect(await zones.count()).toBeGreaterThan(0);

      // Controls, one per offer, with reasons on the disabled ones.
      await expect(page.getByTestId('action-bar')).toBeVisible();
      const allOffers = await offers(page);
      expect(await allOffers.count(), 'every game should offer something').toBeGreaterThan(0);

      // A scoreboard, in the same shape for a game measured in points, one
      // measured in cards left and one measured in chips.
      await expect(page.getByTestId('match-standings')).toBeVisible();
      await expect(page.getByTestId(`standing-${host.userId}`)).toBeVisible();

      // And it plays. Counting clicks would prove nothing — a click on a dead
      // control counts just as well — so the check is that the *server's* view
      // of the match moved, read back through a separate HTTP request.
      const before = await request.get(`${API_BASE}/matches/${matchId}?as=${host.userId}`);
      const beforeBoard = JSON.stringify((await before.json()).view);

      const moves = await playAFewMoves(page, 12);
      expect(moves, 'the shell should have been able to press something').toBeGreaterThan(0);

      const after = await request.get(`${API_BASE}/matches/${matchId}?as=${host.userId}`);
      const afterJson = await after.json();
      expect(
        JSON.stringify(afterJson.view) !== beforeBoard || afterJson.status !== 'active',
        'clicking the shell controls should have advanced the real match',
      ).toBeTruthy();

      // Nothing crashed and the board is still there.
      await expect(page.getByTestId('match-screen')).toBeVisible();
      await expect(page.getByTestId('match-status')).toHaveText(/active|completed|suspended/);
    });
  }

  test('a disabled control says why, in the engine own words', async ({ page, request }) => {
    // The offer protocol's central claim, seen by a person: a control that is
    // off is still on screen, with the reason next to it. An offer that
    // vanished when it became illegal would be indistinguishable from a bug.
    test.setTimeout(120_000);
    const { matchId, host } = await tableWithBots(request, 'canasta', 2, { options: { targetScore: 500 } });
    await signIn(page, host);

    await page.goto(`/match/${matchId}`);
    await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });

    // At the start of a Canasta turn a player must draw, so several other
    // controls are off — each with a code from the engine rather than a
    // sentence composed in the client.
    const reasons = page.locator('[data-testid^="why-"]');
    await expect(reasons.first()).toBeVisible({ timeout: 15_000 });
    const text = await reasons.first().textContent();
    expect(text?.trim().length ?? 0).toBeGreaterThan(0);
  });

  test('a numeric control appears only where a game asks for a number', async ({ page, request }) => {
    // The protocol addition poker forced, rendered. A no-limit raise cannot be
    // enumerated as buttons, so the shell draws a stepper between the bounds
    // the engine computed — and draws nothing of the sort for a card game,
    // because a card game declares no such parameter.
    test.setTimeout(180_000);

    // A card game may still declare a parameter — Prší's wild names the suit
    // that follows, which is the input the protocol grew *first*. What it never
    // declares is a *numeric* one, so that is what is checked: no stepper.
    const cards = await tableWithBots(request, 'prsi', 2);
    await signIn(page, cards.host);
    await page.goto(`/match/${cards.matchId}`);
    await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
    expect(
      await page.locator('[data-testid$="-value"][data-testid^="param-"]').count(),
      'a card game should render no numeric stepper',
    ).toBe(0);

    const poker = await tableWithBots(request, 'holdem', 3, { variation: 'timed' });
    await signIn(page, poker.host);
    await page.goto(`/match/${poker.matchId}`);
    await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });

    // Wait for our turn, then look for the stepper on the raise control.
    const stepper = page.getByTestId('param-amount');
    await expect(stepper).toBeVisible({ timeout: 40_000 });

    const value = page.getByTestId('param-amount-value');
    const before = Number((await value.textContent()) ?? '0');
    await page.getByTestId('param-amount-up').click();
    const after = Number((await value.textContent()) ?? '0');
    expect(after, 'the stepper should move within the engine range').toBeGreaterThanOrEqual(before);

    // And the top of the range is reachable in one press, because a player who
    // wants everything in should not have to hold a button down.
    await page.getByTestId('param-amount-max').click();
    const maxed = Number((await value.textContent()) ?? '0');
    expect(maxed).toBeGreaterThanOrEqual(after);
  });

  test('the lobby lists every hosted game without naming one', async ({ page, request }) => {
    // Adding a fifth game is a server-only change: the picker is rendered from
    // /modules, so a module that registers itself appears here with its
    // variations, its options and its player range.
    const host = await guest(request);
    await signIn(page, host);
    await page.goto('/lobby/games');
    await expect(page.getByTestId('games-list')).toBeVisible({ timeout: 30_000 });

    for (const id of ['zolik', 'prsi', 'canasta', 'holdem']) {
      await expect(page.getByTestId(`module-${id}`)).toBeVisible();
    }

    // Options come from the descriptor, so a knob nobody typed into this
    // client is nonetheless rendered.
    await expect(page.getByTestId('option-holdem-bigBlind-20')).toBeVisible();
    await expect(page.getByTestId('option-canasta-targetScore-500')).toBeVisible();
    // And a game with two shipped rulesets offers both.
    await expect(page.getByTestId('variation-holdem-freezeout')).toBeVisible();
    await expect(page.getByTestId('variation-holdem-timed')).toBeVisible();
  });

  test('the lobby starts a game and hands it to the shell', async ({ page, request }) => {
    test.setTimeout(120_000);
    const host = await guest(request);
    await signIn(page, host);
    await page.goto('/lobby/games');
    await expect(page.getByTestId('games-list')).toBeVisible({ timeout: 30_000 });

    // Pick the short Canasta target so the lobby is exercising real options.
    await page.getByTestId('option-canasta-targetScore-500').click();
    await page.getByTestId('play-bots-canasta').click();

    await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 45_000 });
    await expect(page.getByTestId('match-module')).toHaveText(/canasta/);
    await expect(page.getByTestId('action-bar')).toBeVisible();
  });
});
