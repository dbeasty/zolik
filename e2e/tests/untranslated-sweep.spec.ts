import { expect, test, type APIRequestContext, type Page } from '@playwright/test';

import { API_BASE } from '../helpers/env';

/**
 * Every key the app can put on screen without wording for it.
 *
 * `localisation.spec.ts` asks "is any English showing?", which catches a
 * hardcoded sentence. It cannot catch the *other* leak: a key the server sends
 * that no bundle knows. Those do not render as English-from-the-bundle — they
 * render as `humanise()` output, so `zolik.offer.layMeld` becomes "Lay meld",
 * which is English nobody ever wrote and no search for an English string will
 * find. Four buttons sat on the Czech board that way.
 *
 * With `zolik_i18n_debug`, `t()` stops falling back and renders `_TX_<key>_`
 * instead. This drives the app in Czech with that on and collects every marker
 * it can reach, which turns "some of it is still English" into the list of
 * keys to write words for.
 *
 * It is a gate, not just a report: the expectation at the end is that the list
 * is empty, so a module that starts offering a new action fails here rather
 * than shipping a English button to twenty-three languages.
 */

test.describe.configure({ timeout: 90_000 });

const MARKER = /_TX_([A-Za-z0-9_.[\]-]+)_/g;

/** Keys legitimately absent, with the reason. */
const ALLOWED = [
  // Game, variation, option and choice names the server owns. `gameLabels.ts`
  // deliberately has no keys for the ones that read the same everywhere —
  // "Texas Hold'em", "Vegas Strip", "3:2", "500" — and falls through to the
  // label the server sent. See its header.
  /^module\./,
  /^variation\./,
  /^option\./,
  /^choice\./,
];

function collect(text: string, into: Set<string>) {
  for (const m of text.matchAll(MARKER)) {
    const key = m[1];
    if (!ALLOWED.some((re) => re.test(key))) into.add(key);
  }
}

/** Loads a route in Czech with the marker armed. */
async function openInCzech(page: Page, path: string) {
  await page.addInitScript(() => {
    window.localStorage.setItem('zolik_locale', 'cs');
    window.localStorage.setItem('zolik_i18n_debug', '1');
  });
  await page.goto(path);
  await expect(page.locator('body')).not.toHaveText('');
}

async function guest(request: APIRequestContext, who: string) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `${who}-${Math.random().toString(36).slice(2, 8)}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

/** A started table of bots, so the board has live offers on it. */
async function seatedMatch(request: APIRequestContext, moduleId: string, seats: number) {
  const host = await guest(request, `sweep-${moduleId}`);
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

async function openMatchInCzech(page: Page, host: Record<string, string>, matchId: string) {
  await page.addInitScript(
    (s) => {
      window.localStorage.setItem('zolik_locale', 'cs');
      window.localStorage.setItem('zolik_i18n_debug', '1');
      window.localStorage.setItem('zolik_session', JSON.stringify(s));
    },
    {
      accessToken: host.accessToken,
      refreshToken: host.refreshToken,
      userId: host.userId,
      username: 'sweep',
      isGuest: true,
    },
  );
  await page.goto(`/match/${matchId}`);
}

/** Fails naming the keys, so a red run is a work list rather than a hunt. */
function expectNothingMissing(missing: Set<string>, where: string) {
  const found = [...missing].sort();
  expect(found, `${where}: on screen with no wording in Czech`).toEqual([]);
}

test.describe('no screen can reach a key it has no words for', () => {
  test('the menus and the account panel', async ({ page }) => {
    const missing = new Set<string>();

    for (const path of [
      '/',
      '/more',
      '/about',
      '/settings',
      '/auth/login',
      '/auth/guest',
      '/auth/email',
      '/auth/register',
      '/auth/username-login',
      '/lobby/join',
      '/scoring',
      '/stats',
      '/legal/terms',
      '/legal/privacy',
    ]) {
      await openInCzech(page, path);
      collect(await page.evaluate(() => document.body.innerText), missing);
    }

    await openInCzech(page, '/');
    await page.getByTestId('account-menu-button').click();
    await expect(page.getByTestId('account-menu')).toBeVisible();
    collect(await page.evaluate(() => document.body.innerText), missing);

    expectNothingMissing(missing, 'the menus');
  });

  test('the game picker, where every module describes itself', async ({ page, request }) => {
    const missing = new Set<string>();
    const host = await guest(request, 'sweep-lobby');
    await page.addInitScript(
      (s) => {
        window.localStorage.setItem('zolik_session', JSON.stringify(s));
      },
      {
        accessToken: host.accessToken,
        refreshToken: host.refreshToken,
        userId: host.userId,
        username: 'sweep',
        isGuest: true,
      },
    );
    await openInCzech(page, '/lobby/games');
    await expect(page.getByTestId('games-list')).toBeVisible();
    collect(await page.evaluate(() => document.body.innerText), missing);
    expectNothingMissing(missing, 'the game picker');
  });

  // A live board per game — where the offers are, and where the four English
  // buttons were. Two seats is enough for a turn to exist; Canasta is
  // partnership play and needs four. One test each, so a red run names the
  // game as well as the keys.
  for (const [moduleId, seats] of [
    ['zolik', 2],
    ['prsi', 2],
    ['canasta', 4],
    ['holdem', 2],
    ['ginrummy', 2],
    ['rummytiles', 2],
    ['blackjack', 2],
  ] as const) {
    test(`a table of ${moduleId}`, async ({ page, request }) => {
      const missing = new Set<string>();
      const { matchId, host } = await seatedMatch(request, moduleId, seats);
      await openMatchInCzech(page, host, matchId);
      // The board paints in stages and the action bar is the last of them,
      // which is the part this test exists for. Waited for by name rather
      // than by a sleep: under parallel workers a fixed wait is sometimes
      // long enough and sometimes not, which reads as a leak appearing and
      // disappearing — the worst possible signal from a test like this.
      await expect(page.getByTestId('action-bar')).toBeVisible({ timeout: 20_000 });
      collect(await page.evaluate(() => document.body.innerText), missing);

      // The why-sheet carries the rule and the remedy behind a refusal — a
      // whole vocabulary that never appears until something is refused.
      const why = page.getByText('proč').first();
      if (await why.count()) {
        await why.click({ timeout: 2000 }).catch(() => {});
        await page.waitForTimeout(400);
        collect(await page.evaluate(() => document.body.innerText), missing);
      }

      expectNothingMissing(missing, `a table of ${moduleId}`);
    });
  }
});
