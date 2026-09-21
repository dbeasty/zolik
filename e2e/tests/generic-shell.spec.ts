import { expect, test, type Page } from '@playwright/test';

import { API_BASE } from '../helpers/env';
import { openGameSetup } from '../helpers/lobby';

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

/**
 * What a setup section is painted with — the mark the panel puts on the
 * control a player asked for. Read as the browser computes it rather than as
 * a class name, because the mark is the thing being claimed.
 */
const TRANSPARENT = 'rgba(0, 0, 0, 0)';

async function wash(page: Page, testId: string) {
  return page.getByTestId(testId).evaluate((el) => getComputedStyle(el).backgroundColor);
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
  { moduleId: 'ginrummy', label: 'Gin Rummy', seats: 2, options: { targetScore: 100 } },
  { moduleId: 'blackjack', label: 'Blackjack', seats: 3, options: { rounds: 5, startingStack: 200 } },
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

  test('a finished match says so, and says who won', async ({ page, request }) => {
    /**
     * The bug this is written from, in the shape it was reported: "I folded
     * and I think the game got stuck."
     *
     * The match had ended on that move — it was the last hand of a fixed-length
     * table — and the screen's entire response was a twelve-pixel word next to
     * a dot that stayed green, four controls greying out, and a line at the
     * bottom of a scrolling page reading "Winner", with no name after it. A
     * finished match was indistinguishable from a hung one, which is what the
     * player quite reasonably called it.
     *
     * So: one hand, two seats, and a single press of whatever the server
     * offers — the shortest route to a match that ends on the human's own
     * move. Nothing here names a verb; the control pressed is whichever one
     * the offer list put first.
     */
    test.setTimeout(180_000);
    const { matchId, host } = await tableWithBots(request, 'holdem', 2, {
      variation: 'timed',
      options: { handLimit: 5 },
    });
    await signIn(page, host);

    await page.goto(`/match/${matchId}`);
    await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });

    for (let i = 0; i < 24; i++) {
      if (await page.getByTestId('match-over').isVisible()) break;
      if ((await playAFewMoves(page, 1)) === 0) break;
    }

    const state = await (await request.get(`${API_BASE}/matches/${matchId}?as=${host.userId}`)).json();
    expect(state.status, 'the shortest table this game offers should have ended').toBe('completed');

    // Said, rather than left to be inferred from a control that stopped
    // working.
    await expect(page.getByTestId('match-over')).toBeVisible();
    await expect(page.getByTestId('match-over-title')).toHaveText('Match over');

    // And whose win it was, by name — the part that used to be missing. Read
    // from the server's own answer, so the screen cannot agree with a wrong
    // one.
    const winnerId = (state.winners ?? [])[0];
    expect(winnerId, 'a completed match should name a winner').toBeTruthy();
    const winnerName = state.players.find((p: any) => p.id === winnerId)?.name;
    await expect(page.getByTestId('match-over-outcome')).toHaveText(
      winnerId === host.userId ? 'You won.' : `${winnerName} won.`,
    );

    // The module's own closing lines name people too, instead of trailing off
    // into a bare label with the subject of the sentence dropped.
    await expect(
      page.locator('[data-testid^="status-"]').filter({ hasText: winnerName }).first(),
    ).toBeVisible();

    // The way back out. A table of bots can be set up again from here, which
    // is the one thing a player who just finished one wants.
    await page.getByTestId('match-over-again').click();
    await expect(page.getByTestId('match-over')).toBeHidden({ timeout: 60_000 });
    await expect(page.getByTestId('action-bar')).toBeVisible();
    expect(page.url(), 'playing again should open a different match').not.toContain(matchId);
  });

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

    // The value sits in a typed field now, not a label — a player who knows
    // the figure they want can enter it directly instead of nudging a stepper.
    //
    // Every read below retries. The field re-syncs its text from the shared
    // value one render after a press, so a one-shot inputValue() straight
    // after a click can see the previous figure — which is how this test once
    // took a stale 41 for "max" and then called the correct clamp to 1000 an
    // overshoot. The range comes from the field's own label ("min–max"), the
    // same bounds the engine handed the control.
    const value = page.getByTestId('param-amount-value');
    const [min, max] = ((await value.getAttribute('aria-label')) ?? '').split('–').map(Number);
    expect(Number.isFinite(min) && Number.isFinite(max) && min <= max, 'the field names its range').toBe(
      true,
    );
    const amount = async () => Number((await value.inputValue()) || '0');

    // The raise button names the figure it sends, and follows every way of
    // setting it. It used to read "Raise" over a slider set to 483 — the
    // player had to trust the press read the right number. Checked against
    // the title's own text, the thing on screen, not the field beside it.
    const title = page.getByTestId('offer-raise-title');
    const namesAmount = async (n: number, how: string) =>
      expect(title, `the raise button should name ${n} after ${how}`).toHaveText(
        new RegExp(`\\b${n}$`),
      );
    const before = await amount();
    await namesAmount(before, 'dealing');

    await page.getByTestId('param-amount-up').click();
    await expect
      .poll(amount, { message: 'the stepper should move within the engine range' })
      .toBe(Math.min(before + 1, max));
    await namesAmount(Math.min(before + 1, max), 'the stepper');

    // A quick choice ("½ Pot", "Pot") moves the button too. Their test ids
    // end in the value they jump to, so a numeric suffix picks them out from
    // the stepper's own controls.
    const quick = (
      await page
        .getByTestId('param-amount')
        .locator('[data-testid^="param-amount-"]')
        .evaluateAll((els) => els.map((e) => e.getAttribute('data-testid') ?? ''))
    )
      .map((id) => Number(id.slice('param-amount-'.length)))
      .filter((n) => Number.isInteger(n));
    expect(quick.length, 'the raise should offer quick choices').toBeGreaterThan(0);
    await page.getByTestId(`param-amount-${quick[0]}`).click();
    await expect(value).toHaveValue(String(quick[0]));
    await namesAmount(quick[0], 'a quick choice');

    // And the top of the range is reachable in one press, because a player who
    // wants everything in should not have to hold a button down.
    await page.getByTestId('param-amount-max').click();
    await expect(value).toHaveValue(String(max));
    await namesAmount(max, 'max');

    // Dragging the slider to its left end lands on the minimum, and the button
    // says so — the case in the report: slider moved, button unchanged.
    const slider = page.getByTestId('param-amount-slider');
    const box = await slider.boundingBox();
    if (!box) throw new Error('slider has no box');
    await page.mouse.move(box.x + box.width - 2, box.y + box.height / 2);
    await page.mouse.down();
    await page.mouse.move(box.x + 1, box.y + box.height / 2, { steps: 8 });
    await page.mouse.up();
    await expect(value).toHaveValue(String(min));
    await namesAmount(min, 'dragging the slider');

    // Typing an exact figure works too, and the engine still gets the last
    // word: the field clamps to the range it was given on commit.
    await value.fill(String(max + 1000));
    await value.press('Enter');
    await expect(value, 'typing past the range should clamp to it, not overshoot').toHaveValue(
      String(max),
    );
    await namesAmount(max, 'typing past the range');
  });

  test('the lobby lists every hosted game without naming one', async ({ page, request }) => {
    // Adding a fifth game is a server-only change: the picker is rendered from
    // /modules, so a module that registers itself appears here with its
    // variations, its options and its player range.
    const host = await guest(request);
    await signIn(page, host);
    await page.goto('/lobby/games');
    await expect(page.getByTestId('games-list')).toBeVisible({ timeout: 30_000 });

    for (const id of ['zolik', 'prsi', 'canasta', 'holdem', 'ginrummy', 'blackjack']) {
      await expect(page.getByTestId(`module-${id}`)).toBeVisible();
    }

    // A card arrives closed, saying what it is set to rather than how to set it.
    await expect(page.getByTestId('setup-digest-holdem')).toBeVisible();
    await expect(page.getByTestId('option-holdem-bigBlind-20')).toHaveCount(0);

    // Options come from the descriptor, so a knob nobody typed into this
    // client is nonetheless rendered — once the setup is open.
    await openGameSetup(page, 'holdem');
    await openGameSetup(page, 'canasta');
    await expect(page.getByTestId('option-holdem-bigBlind-20')).toBeVisible();
    await expect(page.getByTestId('option-canasta-targetScore-500')).toBeVisible();
    // And a game with two shipped rulesets offers both.
    await expect(page.getByTestId('variation-holdem-freezeout')).toBeVisible();
    await expect(page.getByTestId('variation-holdem-timed')).toBeVisible();
  });

  test('a way in to the setup arrives at the control it names', async ({ page, request }) => {
    // A card's header has three ways in, and two of them name something: the
    // table size and the digest each state a value, so pressing one reads as
    // "change that". Opening the panel at the top and leaving the player to
    // find the control they just named answers a different question — the more
    // so on Žolíky, whose seat count is the last of nine rows.
    const host = await guest(request);
    await signIn(page, host);
    await page.goto('/lobby/games');
    await expect(page.getByTestId('games-list')).toBeVisible({ timeout: 30_000 });

    // The table size opens the setup at the seat count — in view, not merely
    // in the DOM somewhere below the fold.
    await page.getByTestId('players-zolik').click();
    const seats = page.getByTestId('setup-section-zolik-bots');
    await expect(seats).toBeInViewport({ ratio: 1 });
    // And marked, because a panel scrolled to roughly the right place still
    // leaves a player scanning nine rows of controls for the one they named.
    expect(await wash(page, 'setup-section-zolik-bots')).not.toBe(TRANSPARENT);

    // The digest names the ruleset, so it moves to the rulesets — it does not
    // close the panel the player is reading.
    await page.getByTestId('setup-digest-press-zolik').click();
    await expect(page.getByTestId('setup-toggle-zolik')).toHaveAttribute('aria-expanded', 'true');
    await expect(page.getByTestId('setup-section-zolik-variation')).toBeInViewport({ ratio: 1 });
    // One mark at a time: the seat count is no longer the answer.
    expect(await wash(page, 'setup-section-zolik-variation')).not.toBe(TRANSPARENT);
    expect(await wash(page, 'setup-section-zolik-bots')).toBe(TRANSPARENT);

    // Pressing the way in you are already at is the way back out.
    await page.getByTestId('setup-digest-press-zolik').click();
    await expect(page.getByTestId('setup-toggle-zolik')).toHaveAttribute('aria-expanded', 'false');

    // A chip that names something the module does not have is just another way
    // to open the panel: Gin Rummy seats exactly two, so it draws no seat row
    // at all, and its table size must still open the setup rather than nothing.
    await page.getByTestId('players-ginrummy').click();
    await expect(page.getByTestId('setup-toggle-ginrummy')).toHaveAttribute(
      'aria-expanded',
      'true',
    );
    await expect(page.getByTestId('setup-section-ginrummy-bots')).toHaveCount(0);
    await expect(page.getByTestId('variation-ginrummy-oklahoma')).toBeVisible();
  });

  test('the lobby starts a game and hands it to the shell', async ({ page, request }) => {
    test.setTimeout(120_000);
    const host = await guest(request);
    await signIn(page, host);
    await page.goto('/lobby/games');
    await expect(page.getByTestId('games-list')).toBeVisible({ timeout: 30_000 });

    // Pick the short Canasta target so the lobby is exercising real options.
    await openGameSetup(page, 'canasta');
    await page.getByTestId('option-canasta-targetScore-500').click();
    await page.getByTestId('play-bots-canasta').click();

    await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 45_000 });
    await expect(page.getByTestId('match-module')).toHaveText(/canasta/);
    await expect(page.getByTestId('action-bar')).toBeVisible();
  });

  test('the host says how many bots sit down', async ({ page, request }) => {
    // This button used to seat exactly enough bots to reach the module's
    // minimum, so every quick game was a two-hander. The count is a choice
    // now — and its range is the descriptor's, not this screen's: Prší seats
    // 2–6, so five bots are offered and a sixth is not.
    test.setTimeout(120_000);
    const host = await guest(request);
    await signIn(page, host);
    await page.goto('/lobby/games');
    await expect(page.getByTestId('games-list')).toBeVisible({ timeout: 30_000 });

    await openGameSetup(page, 'prsi');
    await expect(page.getByTestId('bots-prsi-5')).toBeVisible();
    await expect(page.getByTestId('bots-prsi-6')).toHaveCount(0);

    await page.getByTestId('bots-prsi-3').click();
    await page.getByTestId('play-bots-prsi').click();

    await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 45_000 });
    const matchId = new URL(page.url()).pathname.split('/').pop();
    const state = await request.get(`${API_BASE}/matches/${matchId}`);
    expect(state.ok(), await state.text()).toBeTruthy();
    const { players } = (await state.json()) as { players: { isAI: boolean }[] };
    expect(players, 'three bots and the host').toHaveLength(4);
    expect(players.filter((p) => p.isAI)).toHaveLength(3);
  });
});

test.describe('the legacy path is gone', () => {
  test('a second player joins by code and lands on the same screen', async ({ page, request }) => {
    // The join flow is the one part of the lobby that is not "open a table
    // against bots", and it is new: it used to poll a Žolíky game and jump to
    // the Žolíky screen. Now it polls a match and hands over to the one screen.
    test.setTimeout(120_000);

    const host = await guest(request);
    const created = await request.post(`${API_BASE}/matches`, {
      headers: { Authorization: `Bearer ${host.accessToken}` },
      data: { moduleId: 'prsi' },
    });
    expect(created.ok(), await created.text()).toBeTruthy();
    const { matchId, joinCode } = await created.json();
    expect(joinCode, 'a table should have a code to read out').toBeTruthy();

    const joiner = await guest(request);
    await signIn(page, joiner);
    await page.goto('/lobby/join');
    await page.getByTestId('join-code').fill(joinCode);
    await page.getByTestId('join-submit').click();

    await expect(page.getByTestId('lobby-joined')).toBeVisible({ timeout: 20_000 });
    await expect(page.getByTestId(`lobby-player-${joiner.userId}`)).toBeVisible();

    // The host starts it, and the joiner's lobby hands over to the shell by
    // itself — no per-game screen to choose between.
    const started = await request.post(`${API_BASE}/matches/${matchId}/start`, {
      headers: { Authorization: `Bearer ${host.accessToken}` },
    });
    expect(started.ok(), await started.text()).toBeTruthy();

    await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
    await expect(page.getByTestId('match-module')).toHaveText(/prsi/);
  });

  test('the retired endpoints are actually retired', async ({ request }) => {
    // The point of the deletion, checked rather than assumed. These are the
    // routes the bespoke client used; nothing answers on them now.
    const host = await guest(request);
    const auth = { Authorization: `Bearer ${host.accessToken}` };

    for (const [method, path, why] of [
      ['POST', '/games', 'creating a rummy game'],
      ['GET', '/games/abc123', 'reading one'],
      ['GET', '/module', 'the single-module descriptor, replaced by /modules'],
    ] as const) {
      const res =
        method === 'POST'
          ? await request.post(`${API_BASE}${path}`, { headers: auth, data: {} })
          : await request.get(`${API_BASE}${path}`);
      expect(res.status(), `${method} ${path} (${why}) should be gone`).toBe(404);
    }

    // /rules was the rummy ruleset endpoint, and that is gone too — but the
    // URL is not free. The client has a rules screen at /rules
    // (client-react-native/app/rules.tsx), and since the server began shipping
    // the web client from the same origin (92543d8), the exported rules.html
    // answers that path with a 200 whatever the request asks for. So the check
    // is not "404" but "nothing here speaks the old API": the web page when a
    // bundle is embedded, a plain 404 when the server runs from source without
    // one. Rules are served per module now, at /modules/{id}/rules.
    const rules = await request.get(`${API_BASE}/rules`, {
      headers: { Accept: 'application/json' },
    });
    const rulesType = rules.headers()['content-type'] ?? '';
    expect(rulesType, 'GET /rules must not answer with a ruleset').not.toContain('json');
    if (rules.status() !== 404) {
      expect(rules.status(), 'GET /rules is either gone or the rules screen').toBe(200);
      expect(rulesType, 'GET /rules is the client rules screen').toContain('text/html');
    }
    const perModule = await request.get(`${API_BASE}/modules/zolik/rules`);
    expect(perModule.ok(), 'the per-module rules endpoint replaced it').toBeTruthy();

    // And what replaced them answers, for four games.
    const modules = await request.get(`${API_BASE}/modules`);
    expect(modules.ok()).toBeTruthy();
    const ids = (await modules.json()).modules.map((m: { id: string }) => m.id);
    expect(ids).toEqual(expect.arrayContaining(['zolik', 'prsi', 'canasta', 'holdem']));
  });

  test('a finished match is recorded, whatever game it was', async ({ page, request }) => {
    // Statistics used to be rummy arithmetic, so only Žolíky had any. This is
    // the end-to-end proof that is no longer true: play a whole Prší match out
    // over real sockets and ask the server for its recorded result.
    test.setTimeout(180_000);

    const a = await guest(request);
    const b = await guest(request);
    const created = await request.post(`${API_BASE}/matches`, {
      headers: { Authorization: `Bearer ${a.accessToken}` },
      data: { moduleId: 'prsi' },
    });
    expect(created.ok(), await created.text()).toBeTruthy();
    const { matchId } = await created.json();
    await request.post(`${API_BASE}/matches/${matchId}/join`, {
      headers: { Authorization: `Bearer ${b.accessToken}` },
    });
    await request.post(`${API_BASE}/matches/${matchId}/start`, {
      headers: { Authorization: `Bearer ${a.accessToken}` },
    });

    const wsBase = API_BASE.replace(/^http/, 'ws');
    const status = await page.evaluate(
      async ({ wsBase, matchId, tokens }) => {
        const open = async (token: string) => {
          const ws = new WebSocket(`${wsBase}/ws/matches/${matchId}?token=${encodeURIComponent(token)}`);
          const inbox: any[] = [];
          await new Promise<void>((resolve, reject) => {
            ws.onopen = () => resolve();
            ws.onerror = () => reject(new Error('socket failed'));
            setTimeout(() => reject(new Error('open timed out')), 10000);
          });
          ws.onmessage = (ev) => inbox.push(JSON.parse(String(ev.data)));
          return { ws, inbox };
        };
        const latest = (s: any) => {
          for (let i = s.inbox.length - 1; i >= 0; i--) {
            if (s.inbox[i].type === 'match_state') return s.inbox[i];
          }
          return null;
        };

        const seats = await Promise.all(tokens.map(open));
        for (let i = 0; i < 100 && !seats.every(latest); i++) {
          await new Promise((r) => setTimeout(r, 50));
        }

        let last = 'active';
        for (let step = 0; step < 500; step++) {
          const idx = seats.findIndex((s) => (latest(s)?.legalActions ?? []).some((o: any) => o.enabled));
          if (idx === -1) break;
          const st = latest(seats[idx]);
          last = st.status;
          if (last !== 'active') break;

          const o = st.legalActions.filter((x: any) => x.enabled)[0];
          const action: any = { offerId: o.id, verb: o.verb };
          // `submit` first: `cards` is the pool to pick from, not the move.
          if (o.source?.submit?.length) {
            action.cards = o.source.submit;
          } else if (o.source?.minCards && (o.source?.cards ?? []).length >= o.source.minCards) {
            action.cards = o.source.cards.slice(0, o.source.minCards);
          }
          for (const p of o.params ?? []) {
            action.params = { ...(action.params ?? {}), [p.name]: p.choices?.[0]?.value ?? String(p.min ?? 0) };
          }
          const before = seats.map((s) => s.inbox.length);
          seats[idx].ws.send(JSON.stringify(action));
          for (let i = 0; i < 120; i++) {
            if (seats.every((s, k) => s.inbox.length > before[k])) break;
            await new Promise((r) => setTimeout(r, 20));
          }
        }
        const final = seats.map(latest).find((s) => s);
        for (const s of seats) s.ws.close();
        return final?.status ?? last;
      },
      { wsBase, matchId, tokens: [a.accessToken, b.accessToken] },
    );

    expect(status).toBe('completed');

    // Recording is asynchronous by design — the player who just won should not
    // wait on bookkeeping — so give it a moment.
    let body: any = null;
    for (let i = 0; i < 40; i++) {
      const res = await request.get(`${API_BASE}/matches/${matchId}/result`);
      if (res.ok()) {
        body = await res.json();
        break;
      }
      await new Promise((r) => setTimeout(r, 250));
    }
    expect(body, 'a finished match should be recorded').toBeTruthy();
    expect(body.moduleId).toBe('prsi');
    expect((body.participants ?? []).length).toBe(2);
    // The scoreboard is in the shape no game owns: a rank and a score with a
    // unit, whatever the game measures.
    for (const p of body.participants) {
      expect(p.rank).toBeGreaterThanOrEqual(1);
      expect(p.playerId).toBeTruthy();
    }
  });
});
