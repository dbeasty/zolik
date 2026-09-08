import type { Locator, Page } from '@playwright/test';

/**
 * The only file in the harness that knows what anything on screen is called.
 *
 * A soak client presses the same controls a person presses, so when a screen
 * is renamed exactly one file here goes red rather than every scenario. The
 * vocabulary is deliberately small — sign in, pick a game, open or join a
 * table, press what is offered, leave — because that is the whole of what a
 * player does, repeated for hours.
 *
 * Nothing here names a game. The list of games comes from the lobby, which
 * renders itself from `/modules`, so a module registered tomorrow is soaked
 * tomorrow without this file being edited.
 */

/** How long we are willing to wait for a screen to appear at all. */
const SCREEN_TIMEOUT = 45_000;

export async function signInAsGuest(page: Page, name: string): Promise<void> {
  // Through the front door rather than by seeding localStorage the way the
  // e2e helpers do. A soak run has hours to spare and only does this once per
  // client, and it means `/auth/guest`, the session bootstrap and the redirect
  // into the lobby are all on the soaked path instead of being stepped over.
  await page.goto('/');
  await page.getByText('Play', { exact: true }).click();

  const field = page.getByPlaceholder('Display name');
  await field.waitFor({ state: 'visible', timeout: SCREEN_TIMEOUT });
  await field.fill(name);
  await page.getByText('Continue', { exact: true }).click();

  await page.getByTestId('games-list').waitFor({ state: 'visible', timeout: SCREEN_TIMEOUT });
}

export async function openGamesScreen(page: Page): Promise<void> {
  await page.goto('/lobby/games');
  await page.getByTestId('games-list').waitFor({ state: 'visible', timeout: SCREEN_TIMEOUT });
}

/** The games this server hosts, read off the picker the way a player reads them. */
export async function gamesOnOffer(page: Page): Promise<string[]> {
  return page
    .locator('[data-testid^="play-bots-"]')
    .evaluateAll((els) =>
      els.map((e) => (e.getAttribute('data-testid') ?? '').replace('play-bots-', '')).filter(Boolean),
    );
}

/**
 * Seats this client at a table of bots and deals.
 *
 * The bot count is picked from the pills the module itself declares, so a game
 * that seats six is soaked at six sometimes and at two others — table size is
 * a real dimension of load and picking one number here would flatten it.
 */
export async function startAgainstBots(page: Page, moduleId: string, rng: () => number): Promise<void> {
  const pills = await page
    .locator(`[data-testid^="bots-${moduleId}-"]`)
    .evaluateAll((els) => els.map((e) => e.getAttribute('data-testid') ?? ''));
  if (pills.length) {
    await page.getByTestId(pills[Math.floor(rng() * pills.length)]).click();
  }
  await page.getByTestId(`play-bots-${moduleId}`).click();
  await page.getByTestId('match-screen').waitFor({ state: 'visible', timeout: SCREEN_TIMEOUT });
}

/** Opens a table for other people and returns its join code. */
export async function openTable(page: Page, moduleId: string): Promise<string> {
  await page.getByTestId(`play-friends-${moduleId}`).click();
  const code = page.getByTestId('table-join-code');
  await code.waitFor({ state: 'visible', timeout: SCREEN_TIMEOUT });
  return ((await code.textContent()) ?? '').trim();
}

/** Joins a table by the code its host read out. */
export async function joinByCode(page: Page, code: string): Promise<void> {
  await page.goto('/lobby/join');
  const field = page.getByTestId('join-code');
  await field.waitFor({ state: 'visible', timeout: SCREEN_TIMEOUT });
  await field.fill(code);
  await page.getByTestId('join-submit').click();
  await page.getByTestId('lobby-joined').waitFor({ state: 'visible', timeout: SCREEN_TIMEOUT });
}

/** Seats one more bot at the host's table. */
export async function addBotAtTable(page: Page): Promise<boolean> {
  const add = page.getByTestId('table-add-bot');
  if (!(await add.isVisible().catch(() => false))) return false;
  if (!(await add.click({ timeout: 5_000 }).then(() => true).catch(() => false))) return false;
  await page.waitForTimeout(400);
  return true;
}

/** How many seats are taken, read off the roster the host is looking at. */
export async function seatedAtTable(page: Page): Promise<number> {
  return page.locator('[data-testid^="seated-"]').count();
}

/**
 * Deals, and says whether it took.
 *
 * Start is not greyed out when a table is short of players — the server is the
 * one that knows a game's minimum, and it answers by refusing, which the screen
 * prints. So the caller's move when this returns false is to seat another bot
 * and ask again, which is exactly what the host would do.
 */
export async function startTable(page: Page, timeout = 12_000): Promise<boolean> {
  await page.getByTestId('table-start').click({ timeout: 10_000 });
  return page
    .getByTestId('match-screen')
    .waitFor({ state: 'visible', timeout })
    .then(() => true)
    .catch(() => false);
}

/**
 * Puts this client in the waiting room and leaves it there.
 *
 * The one scenario with no match in it, and the reason it is here: being
 * available is a *socket*, held open for as long as the screen is, and a
 * waiting room that leaks a room entry per visit does not show up in any run
 * where every client is busy playing.
 */
export async function waitInTheLobby(page: Page): Promise<boolean> {
  await page.goto('/');
  const find = page.getByText('Make me available to play', { exact: true });
  if (!(await find.isVisible().catch(() => false))) return false;
  await find.click();
  return page
    .getByTestId('waiting-status-open')
    .waitFor({ state: 'visible', timeout: 30_000 })
    .then(() => true)
    .catch(() => false);
}

/** Waits for a table this client joined to be dealt by its host. */
export async function waitForDeal(page: Page, timeout: number): Promise<boolean> {
  return page
    .getByTestId('match-screen')
    .waitFor({ state: 'visible', timeout })
    .then(() => true)
    .catch(() => false);
}

/** The match this page is looking at, from the address bar. */
export function matchIdInUrl(page: Page): string {
  return page.url().match(/\/match\/([^/?#]+)/)?.[1] ?? '';
}

export function matchOverBanner(page: Page): Locator {
  return page.getByTestId('match-over');
}

/** Sets up the same table again — only offered when the opponents were all bots. */
export async function playAgain(page: Page): Promise<boolean> {
  const again = page.getByTestId('match-over-again');
  if (!(await again.isVisible().catch(() => false))) return false;
  await again.click();
  await matchOverBanner(page).waitFor({ state: 'hidden', timeout: SCREEN_TIMEOUT }).catch(() => {});
  return true;
}

export async function leaveMatch(page: Page): Promise<void> {
  const leave = page.getByTestId('match-over-leave');
  if (await leave.isVisible().catch(() => false)) {
    await leave.click();
  } else {
    await page.goto('/lobby/games');
  }
  await page.getByTestId('games-list').waitFor({ state: 'visible', timeout: SCREEN_TIMEOUT });
}

/** Presses one of the controls the action bar is showing. Reports whether it landed. */
export async function pressOffer(page: Page, offerId: string): Promise<boolean> {
  // By id rather than through a locator resolved earlier: every accepted action
  // re-renders the whole bar, so a handle taken a moment ago can be pointing at
  // a detached element by the time the click arrives.
  return page
    .getByTestId(`offer-${offerId}`)
    .click({ timeout: 5_000 })
    .then(() => true)
    .catch(() => false);
}

/**
 * The controls that can be pressed right now, as ids.
 *
 * "Live" here means the client's own answer, not the server's: a control is
 * greyed out both when the engine refuses it and when the current selection
 * would not go, and from the outside those are the same fact — this seat
 * cannot press that. The distinction only matters to the caller once nothing
 * at all is live, which is where the server gets asked.
 */
export async function liveOfferIds(page: Page): Promise<string[]> {
  return page
    .locator('[data-testid^="offer-"]:not([aria-disabled="true"])')
    .evaluateAll((els) =>
      els
        .map((e) => (e.getAttribute('data-testid') ?? '').replace(/^offer-/, ''))
        // The folded groups and the glance strip share the prefix and are not
        // offers; only a control named for a single offer is one.
        .filter((id) => id && !id.startsWith('group:') && !id.startsWith('glance')),
    );
}

/** Whether a control is on screen and live — the client's own readiness included. */
export async function offerIsLive(page: Page, offerId: string): Promise<boolean> {
  const el = page.getByTestId(`offer-${offerId}`);
  if (!(await el.isVisible().catch(() => false))) return false;
  return (await el.getAttribute('aria-disabled')) !== 'true';
}

export function handCards(page: Page): Locator {
  return page.locator('[data-testid^="card-hand:"]');
}

/** Empties the selection, so what follows starts from nothing picked. */
export async function clearSelection(page: Page): Promise<void> {
  const selected = page.locator('[data-testid^="card-hand:"][aria-selected="true"]');
  for (let guard = 0; guard < 20; guard++) {
    if ((await selected.count()) === 0) return;
    // One at a time, re-reading between clicks: each click re-renders the fan,
    // so a list of handles captured up front goes stale.
    await selected.first().click({ timeout: 3_000 }).catch(() => {});
  }
}

/**
 * Picks `count` cards out of the hand, starting at `from`.
 *
 * Which cards is deliberately not clever. A soak client is not trying to play
 * well — it is trying to keep the server, the socket and the screen busy — and
 * a control that will not take the cards offered simply stays unready, which
 * the caller reads and moves on from. Rotating the starting index means a hand
 * that refuses the first card is not asked the same question forever.
 */
export async function selectCards(page: Page, count: number, from: number): Promise<number> {
  await clearSelection(page);
  const cards = handCards(page);
  const total = await cards.count();
  if (total === 0) return 0;

  let picked = 0;
  for (let i = 0; i < count && i < total; i++) {
    const card = cards.nth((from + i) % total);
    if (await card.click({ timeout: 3_000 }).then(() => true).catch(() => false)) picked++;
  }
  return picked;
}
