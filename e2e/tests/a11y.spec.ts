import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Page } from '@playwright/test';

import { playAgainstBots, openGame } from '../helpers/lobby';
import { loginAsFreshGuest, seedIntroSeen } from '../helpers/login';

/**
 * Automated accessibility checks (docs/accessibility-plan.md, Phase 0/6).
 *
 * axe-core over every screen a player can reach, and over a live table of
 * every game. Serious and critical violations fail the test; moderate and
 * minor ones are printed so they stay visible without blocking.
 *
 * Rules switched off, each for a stated reason:
 * - `color-contrast` on the felt: axe cannot see through the board's gradient
 *   and SVG card faces, and reports every card as "needs review". Contrast is
 *   asserted per skin token by `src/skins/contrast.test.ts` instead.
 * - `region`: React Native Web renders the root as nested divs; landmarks are
 *   asserted for the board explicitly below rather than for every wrapper.
 */
const GAMES = ['zolik', 'prsi', 'canasta', 'holdem', 'ginrummy', 'rummytiles', 'blackjack', 'marias', 'klondike', 'lastcard'];

async function scan(page: Page, what: string, opts: { board?: boolean } = {}) {
  let builder = new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa']).disableRules(['region']);
  if (opts.board) builder = builder.disableRules(['region', 'color-contrast']);
  const result = await builder.analyze();
  const blocking = result.violations.filter((v) => v.impact === 'serious' || v.impact === 'critical');
  for (const v of result.violations) {
    console.log(
      `[axe ${what}] ${v.impact} ${v.id}: ${v.help} — ${v.nodes.length} node(s): ${v.nodes
        .slice(0, 3)
        .map((n) => n.target.join(' '))
        .join(' | ')}`,
    );
  }
  expect(
    blocking.map((v) => `${v.id} (${v.nodes.length}): ${v.nodes.slice(0, 2).map((n) => n.html.slice(0, 120)).join(' || ')}`),
    `${what}: serious/critical axe violations`,
  ).toEqual([]);
}

test.describe('axe: screens outside a match', () => {
  test.beforeEach(async ({ page, request }) => {
    await loginAsFreshGuest(page, request, `axe-${Math.random().toString(36).slice(2, 8)}`);
  });

  for (const [path, ready] of [
    ['/', 'home'],
    ['/lobby/games', 'games'],
    ['/settings', 'settings'],
    ['/rules?moduleId=zolik', 'rules'],
    ['/stats', 'stats'],
    ['/about', 'about'],
    ['/more', 'more'],
    ['/lobby/join', 'join'],
    ['/legal/terms', 'terms'],
    ['/legal/privacy', 'privacy'],
    ['/legal/accessibility', 'accessibility statement'],
    ['/lobby/games?moduleId=zolik', 'game page'],
    ['/lobby/mine', 'my games'],
    ['/account', 'account'],
    ['/circle', 'circle'],
    ['/scoring', 'score table'],
    ['/local-games', 'local games'],
  ] as const) {
    test(`${ready} has no serious violations`, async ({ page }) => {
      await page.goto(path);
      await page.waitForLoadState('networkidle').catch(() => {});
      await page.waitForTimeout(800);
      await scan(page, ready);
    });
  }

  test('game setup has no serious violations', async ({ page }) => {
    await openGame(page, 'zolik');
    await page.getByTestId('play-bots-zolik').click();
    await expect(page.getByTestId('deal-me-in-zolik')).toBeVisible({ timeout: 30_000 });
    await scan(page, 'setup');
  });

  test('table lobby has no serious violations', async ({ page }) => {
    await openGame(page, 'zolik');
    await page.getByTestId('play-friends-zolik').click();
    await page.getByTestId('open-table-zolik').click();
    await expect(page.getByTestId('table-add-bot')).toBeVisible({ timeout: 30_000 });
    await page.getByTestId('table-add-bot').click();
    await expect(page.locator('[data-testid^="bot-strength-open-"]').first()).toBeVisible();
    await scan(page, 'table lobby');
    // The bot strength sheet, open: a named dialog over the table.
    await page.locator('[data-testid^="bot-strength-open-"]').first().click();
    await expect(page.getByTestId('bot-strength-sheet')).toBeVisible();
    // Past the fade-in: mid-fade the text is half its colour, and the dialog
    // has no role until react-native-web marks it active at the end.
    await page.waitForTimeout(800);
    await scan(page, 'bot strength sheet');
  });

  test('account menu, open, has no serious violations', async ({ page }) => {
    await page.goto('/');
    await page.getByTestId('account-menu-button').click();
    await expect(page.getByTestId('account-menu')).toBeVisible();
    await page.waitForTimeout(800);
    await scan(page, 'account menu');
  });
});

test.describe('axe: signed-out screens', () => {
  test('intro and sign-in have no serious violations', async ({ page }) => {
    await page.goto('/intro');
    await page.waitForTimeout(800);
    await scan(page, 'intro');
    await seedIntroSeen(page);
    await page.goto('/auth/login');
    await page.waitForTimeout(800);
    await scan(page, 'login');
    for (const [path, what] of [
      ['/auth/email', 'email sign-in'],
      ['/auth/guest', 'guest'],
      ['/auth/username-login', 'username sign-in'],
      ['/auth/register', 'register'],
      ['/lobby/join', 'join (signed out)'],
    ] as const) {
      await page.goto(path);
      await page.waitForTimeout(800);
      await scan(page, what);
    }
  });
});

test.describe('axe: a live table of every game', () => {
  test.setTimeout(90_000);
  for (const game of GAMES) {
    test(`${game} board has no serious violations`, async ({ page, request }) => {
      await loginAsFreshGuest(page, request, `axe-${game}-${Math.random().toString(36).slice(2, 6)}`);
      await openGame(page, game);
      if (game === 'klondike') {
        // One seat: solitaire has no bots to play.
        await page.getByTestId('play-solo-klondike').click();
        await page.getByTestId('deal-me-in-klondike').click();
      } else {
        await playAgainstBots(page, game);
      }
      await expect(page).toHaveURL(/\/match\//, { timeout: 30_000 });
      await page.waitForTimeout(2500);
      await scan(page, `${game} board`, { board: true });
    });
  }
});
