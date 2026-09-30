import { expect, test } from '@playwright/test';

import { loginAsFreshAccount, loginAsFreshGuest } from '../helpers/login';

/**
 * The stats & leaderboard screen.
 *
 * The defect this file is written against is specific and worth naming: the
 * screen used to render `JSON.stringify(response, null, 2)` for both halves,
 * so a player with a record was shown the raw shape of it. Every test here
 * therefore asserts on *rendered* content — a heading, a table row, a named
 * empty state — and asserts that no JSON punctuation survives anywhere on the
 * page. A suite that only checked the fetch succeeded would have stayed green
 * through the entire bug.
 *
 * The screen needs an account (see account-gate.spec.ts, which owns the gate
 * itself), so every test that looks at its content signs one in first.
 *
 * Locators use `exact: true` for the same reason sign-in.spec.ts does: React
 * Native Web renders these as plain text nodes and a substring match collides
 * with the prose around them.
 */

const suffix = () => Math.random().toString(36).slice(2, 8);
const freshEmail = (tag: string) => `e2e-${tag}-${suffix()}@example.com`;

/** Nothing on this screen may ever be a serialised payload again. */
async function expectNoRawJson(page: import('@playwright/test').Page) {
  const body = (await page.getByTestId('your-record').textContent()) ?? '';
  const board = (await page.getByTestId('leaderboard-section').textContent()) ?? '';
  for (const text of [body, board]) {
    expect(text).not.toContain('{');
    expect(text).not.toContain('"entries"');
    expect(text).not.toContain('null');
  }
}

test.describe('stats & leaderboard', () => {
  test('without an account the screen is the gate, never a payload', async ({ page, request }) => {
    await page.goto('/stats');
    await expect(page.getByTestId('sign-in-required')).toBeVisible({ timeout: 10_000 });

    await loginAsFreshGuest(page, request, `e2e-stats-guest-${suffix()}`);
    await page.goto('/stats');
    await expect(page.getByTestId('sign-in-required')).toBeVisible({ timeout: 10_000 });
    await expect(page.getByTestId('your-record')).toHaveCount(0);
  });

  test('a fresh account sees an empty record in words, not a wall of zeroes', async ({
    page,
    request,
  }) => {
    await loginAsFreshAccount(page, request, freshEmail('stats'));
    await page.goto('/stats');

    // A brand-new account has played nothing. That is an absence, and the
    // screen says so rather than printing 0/0/0/0 and a 0% win rate — see the
    // note at the top of src/lib/stats.ts about what a flat zero claims.
    await expect(page.getByTestId('stats-none-yet')).toBeVisible({ timeout: 15_000 });
    await expect(page.getByTestId('stats-none-yet')).toContainText('No finished matches yet');
    await expect(page.getByTestId('sign-in-required')).toHaveCount(0);

    // The board beside it loads on its own: either somebody is ranked, or the
    // named empty state says so in words.
    await expect(
      page.getByTestId('leaderboard-table').or(page.getByTestId('leaderboard-empty')),
    ).toBeVisible({ timeout: 10_000 });

    await expectNoRawJson(page);
  });

  test('the leaderboard toggles re-query and stay rendered', async ({ page, request }) => {
    await loginAsFreshAccount(page, request, freshEmail('stats-board'));
    await page.goto('/stats');
    const section = page.getByTestId('leaderboard-section');
    await expect(section).toBeVisible({ timeout: 10_000 });

    // Overall is the default, and its blurb is what explains why the scoped
    // boards below do not add up to it.
    await expect(section).toContainText('Every match, whoever was at the table.');

    await page.getByTestId('leaderboard-scope-vs_ai').click();
    await expect(section).toContainText('Matches with at least one bot in them.');
    await expect(
      page.getByTestId('leaderboard-table').or(page.getByTestId('leaderboard-empty')),
    ).toBeVisible({ timeout: 10_000 });

    // Bots are ranked on their own board, never mixed into the human one.
    await page.getByTestId('leaderboard-kind-ai').click();
    await expect(
      page.getByTestId('leaderboard-table').or(page.getByTestId('leaderboard-empty')),
    ).toBeVisible({ timeout: 10_000 });

    await expectNoRawJson(page);
  });
});

test.describe('recording a live game', () => {
  test('is a scorepad behind "More", not a way to play on the menu', async ({ page, request }) => {
    await loginAsFreshAccount(page, request, freshEmail('live'));
    await page.goto('/');

    // The menu is the list of games. The scorepad is not one, and neither is
    // the stats screen; both live behind the face in the corner.
    await expect(page.getByTestId('games-list')).toBeVisible({ timeout: 10_000 });
    for (const text of ['Offline score table', 'Record a live game', 'Stats & leaderboard']) {
      await expect(page.getByText(text, { exact: true })).toHaveCount(0);
    }

    await page.goto('/more');
    const entry = page.getByTestId('more-score-table');
    await expect(entry).toHaveText('Record a live game', { timeout: 10_000 });
    await entry.click();
    await expect(page).toHaveURL(/\/scoring/, { timeout: 10_000 });

    // Says what it is, and that it does not feed the record on the stats
    // screen — the natural reading of a scorecard filed next to a win rate.
    await expect(page.getByText(/playing with real cards/).locator('visible=true')).toBeVisible();
    await expect(
      page.getByText(/Nothing it records counts towards your stats/).locator('visible=true'),
    ).toBeVisible();
    await expect(page.getByText('Start scorecard', { exact: true }).locator('visible=true')).toBeVisible();
  });
});

/**
 * A record with things actually in it.
 *
 * Served from a fixture rather than played out: the empty-state tests above
 * already prove the real fetch path end to end, and what is unproven by them is
 * the *layout* — which buckets earn a row, what order they come in, and what a
 * never-played one does. Reaching a five-way split by playing would take dozens
 * of seeded matches and would pin none of those decisions any harder.
 *
 * The shape is the server's `statsResponse` (server/internal/stats/handlers.go).
 */
function tally(over: Record<string, unknown>) {
  return {
    matches: 0,
    wins: 0,
    losses: 0,
    draws: 0,
    scoreSum: 0,
    rankSum: 0,
    winRate: 0,
    avgScore: 0,
    avgRank: 0,
    bestScore: null,
    worstScore: null,
    ...over,
  };
}

const POPULATED = {
  gamesPlayed: 12,
  gamesWon: 5,
  gamesLost: 6,
  gamesDrawn: 1,
  subject: { kind: 'user', id: 'me', name: 'Dana' },
  overall: tally({
    matches: 12,
    wins: 5,
    losses: 6,
    draws: 1,
    winRate: 5 / 12,
    avgRank: 2.1,
    bestScore: 480,
  }),
  vsHumans: tally({ matches: 7, wins: 4, losses: 3, winRate: 4 / 7, avgRank: 1.9 }),
  vsAI: tally({ matches: 9, wins: 3, losses: 5, draws: 1, winRate: 1 / 3, avgRank: 2.4 }),
  byModule: {
    zolik: tally({ matches: 8, wins: 4, winRate: 0.5, avgRank: 1.8 }),
    canasta: tally({ matches: 4, wins: 1, winRate: 0.25, avgRank: 2.5 }),
  },
  byPlayerCount: {
    // Out of numeric order on purpose: the screen must sort by table size, not
    // by the string the key happens to be.
    '4': tally({ matches: 5, wins: 1, winRate: 0.2, avgRank: 2.8 }),
    '2': tally({ matches: 7, wins: 4, winRate: 4 / 7, avgRank: 1.4 }),
    // Never played. Must not appear as a row of zeroes.
    '6': tally({}),
  },
  byAIDifficulty: {
    'hard:miroslav': tally({ matches: 2, wins: 0, losses: 2, winRate: 0, avgRank: 2.5 }),
    easy: tally({ matches: 4, wins: 3, winRate: 0.75, avgRank: 1.3 }),
  },
  currentStreak: -2,
  longestWinStreak: 3,
  longestLossStreak: 2,
  recentMatches: [],
};

test.describe('a record with something in it', () => {
  test('renders the headline and the splits, and drops what was never played', async ({
    page,
    request,
  }) => {
    await page.route('**/users/me/stats', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(POPULATED),
      }),
    );
    await loginAsFreshAccount(
      page,
      request,
      `e2e-stats-full-${Math.random().toString(36).slice(2, 8)}@example.com`,
    );
    await page.goto('/stats');

    const record = page.getByTestId('your-record');
    await expect(page.getByTestId('stats-headline')).toBeVisible({ timeout: 15_000 });
    await expect(page.getByTestId('stats-played')).toContainText('12');
    await expect(page.getByTestId('stats-won')).toContainText('5');
    await expect(page.getByTestId('stats-winrate')).toContainText('42%');
    await expect(page.getByTestId('stats-avgrank')).toContainText('2.1');
    // Signed streak: negative is a losing run, and it says so in words.
    await expect(page.getByTestId('stats-streak')).toContainText('2 losses in a row');
    await expect(page.getByTestId('stats-bestscore')).toContainText('480');

    // The overlap is stated where somebody noticing 7 + 9 > 12 would look.
    await expect(record).toContainText('counts in both rows');

    // Table sizes sort numerically, and the unplayed one is absent entirely —
    // not a 0% row claiming they lose every 6-handed game.
    await expect(page.getByTestId('split-row-2')).toBeVisible();
    await expect(page.getByTestId('split-row-4')).toBeVisible();
    await expect(page.getByTestId('split-row-6')).toHaveCount(0);
    const sizes = (await page.getByTestId('split-sizes').textContent()) ?? '';
    expect(sizes.indexOf('2 players')).toBeLessThan(sizes.indexOf('4 players'));

    // Bots are weakest-first, and a persona key reads as a name, not an id.
    const bots = (await page.getByTestId('split-bots').textContent()) ?? '';
    expect(bots).toContain('Miroslav (hard)');
    expect(bots.indexOf('Easy')).toBeLessThan(bots.indexOf('Miroslav (hard)'));

    // A genuinely-played bucket that was lost every time *does* read 0%.
    await expect(page.getByTestId('split-row-hard:miroslav')).toContainText('0%');

    // And the module list gives the game its real name.
    await expect(page.getByTestId('split-games')).toContainText('Žolíky');

    await expectNoRawJson(page);
  });
});
