import { expect, test, type Page } from '@playwright/test';

import { API_BASE } from '../helpers/env';
import { seedIntroSeen } from '../helpers/login';

/**
 * Every game on every screen size a person will use it on.
 *
 * `every-game.spec.ts` proves each registered game plays over the socket. This
 * proves each one can be *looked at*: the match screen mounts, nothing throws
 * in the page, the page never grows a horizontal scrollbar, and the rules
 * screen renders — on a phone and on a desktop. Phone layout is where a new
 * game most often breaks (a wide scoreboard, a fixed-width row), and nothing
 * else walks the whole registry at 375px.
 *
 * Driven from `/modules`, so a game added tomorrow is covered the day it is
 * registered.
 */

type Ctx = import('@playwright/test').APIRequestContext;
type Module = {
  id: string;
  label: string;
  minPlayers: number;
  maxPlayers: number;
  variations?: { id: string; maxPlayers?: number }[];
};

const SIZES = [
  { name: 'phone', width: 375, height: 667 },
  { name: 'tablet', width: 768, height: 1024 },
  { name: 'desktop', width: 1280, height: 800 },
];

async function fetchModules(): Promise<Module[]> {
  const res = await fetch(`${API_BASE}/modules`);
  const body = await res.json();
  return Array.isArray(body) ? body : body.modules;
}

async function guest(request: Ctx) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `ui-${Math.random().toString(36).slice(2, 10)}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

async function seatedTable(request: Ctx, mod: Module, variation: string | undefined) {
  const host = await guest(request);
  const auth = { Authorization: `Bearer ${host.accessToken}` };
  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    data: { moduleId: mod.id, variation, options: {} },
  });
  expect(created.ok(), await created.text()).toBeTruthy();
  const { matchId } = await created.json();
  const solo = mod.maxPlayers === 1;
  for (let i = 1; i < (solo ? 1 : Math.max(2, mod.minPlayers)); i++) {
    const bot = await request.post(`${API_BASE}/matches/${matchId}/add-bot`, { headers: auth });
    expect(bot.ok(), await bot.text()).toBeTruthy();
  }
  const started = await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });
  expect(started.ok(), await started.text()).toBeTruthy();
  return { matchId, host };
}

async function signIn(page: Page, host: any) {
  await page.addInitScript((s) => {
    window.localStorage.setItem('zolik_session', JSON.stringify(s));
  }, {
    accessToken: host.accessToken,
    refreshToken: host.refreshToken,
    userId: host.userId,
    username: host.username ?? 'ui',
    isGuest: true,
  });
  await seedIntroSeen(page);
}

/** Console noise the dev bundle always makes; anything else is a finding. */
const BENIGN = [
  /React DevTools/i,
  /Download the React/i,
  /\[perf\]/i,
  /Reanimated/i,
  /Require cycle/i,
  /shadow\*/i,
  /pointerEvents is deprecated/i,
  /Running application/i,
  /\[object Object\]/i,
  /Development-level warnings/i,
  /Performance optimizations/i,
  /Failed to load resource.*(404|favicon)/i,
  /useNativeDriver/i,
  /props\.pointerEvents/i,
  /accessibilityRole|accessibilityLabel|accessibilityState|accessibilityHint/i,
];

function watchPage(page: Page) {
  const problems: string[] = [];
  page.on('pageerror', (e) => problems.push(`pageerror: ${e.message}`));
  page.on('console', (m) => {
    if (m.type() !== 'error') return;
    const text = m.text();
    if (BENIGN.some((re) => re.test(text))) return;
    problems.push(`console.error: ${text.slice(0, 300)}`);
  });
  return problems;
}

/**
 * Content a person cannot reach on this screen width.
 *
 * `documentElement.scrollWidth` is no use here: the app's root clips, so a row
 * that is 600px wide on a 220px window still reports a document that fits.
 * Instead: every visible element whose right edge is past the window, unless
 * something between it and the root is itself a scroller or a clip (a hand
 * fan, a carousel, a table that scrolls sideways on purpose).
 */
const overflow = (page: Page) =>
  page.evaluate(() => {
    const W = window.innerWidth;
    const cut: string[] = [];
    for (const el of Array.from(document.body.querySelectorAll<HTMLElement>('*'))) {
      const r = el.getBoundingClientRect();
      if (r.width < 1 || r.height < 1 || r.right <= W + 1) continue;
      const cs = getComputedStyle(el);
      if (cs.display === 'none' || cs.visibility === 'hidden' || cs.opacity === '0') continue;
      if (cs.position === 'fixed') continue;
      let covered = false;
      for (let a: HTMLElement | null = el.parentElement; a && a !== document.body; a = a.parentElement) {
        const ox = getComputedStyle(a).overflowX;
        if (a.id === 'root') break;
        if (ox !== 'visible') {
          covered = true;
          break;
        }
      }
      if (covered) continue;
      const id = el.getAttribute('data-testid') || el.tagName.toLowerCase();
      cut.push(`${id} (${Math.round(r.left)}→${Math.round(r.right)})`);
      if (cut.length >= 4) break;
    }
    return { cut, innerWidth: W };
  });

test.describe('the overflow probe', () => {
  // A probe that cannot fail proves nothing — at 220px the app genuinely fits,
  // so the only way to know the check bites is to hand it something that does not.
  test('sees an element past the edge, and ignores one inside a scroller', async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 667 });
    await page.setContent(`<div data-testid="wide" style="width:900px;height:20px">x</div>`);
    expect((await overflow(page)).cut.join()).toContain('wide');
    await page.setContent(
      `<div style="overflow-x:auto;width:300px"><div data-testid="inside" style="width:900px;height:20px">x</div></div>`,
    );
    expect((await overflow(page)).cut).toEqual([]);
  });
});

test.describe('every game renders on every screen size', () => {
  test.describe.configure({ mode: 'serial' });

  let modules: Module[] = [];
  test.beforeAll(async () => {
    modules = await fetchModules();
  });

  for (const size of SIZES) {
    test(`${size.name}: each variation's match screen mounts cleanly and fits the width`, async ({
      page,
      request,
    }) => {
      test.setTimeout(20 * 60_000);
      await page.setViewportSize({ width: size.width, height: size.height });
      const failures: string[] = [];
      const problems = watchPage(page);

      for (const mod of modules) {
        const variations = mod.variations?.length ? mod.variations : [{ id: undefined as any }];
        for (const v of variations) {
          const label = `${mod.id}/${v.id ?? 'default'}@${size.name}`;
          problems.length = 0;
          try {
            const { matchId, host } = await seatedTable(request, mod, v.id);
            await page.context().clearCookies();
            await signIn(page, host);
            await page.goto(`/match/${matchId}`);
            await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
            await expect(page.getByTestId('match-module')).toHaveText(new RegExp(mod.id));
            // A board: at least one zone drawn, and the controls bar.
            await expect
              .poll(() => page.locator('[data-testid^="zone-"]').count(), { timeout: 15_000 })
              .toBeGreaterThan(0);
            await expect(page.getByTestId('action-bar')).toBeVisible({ timeout: 15_000 });
            // Let the first paint settle, then measure.
            await page.waitForTimeout(600);
            const o = await overflow(page);
            if (o.cut.length) failures.push(`${label}: past the ${o.innerWidth}px edge: ${o.cut.join(', ')}`);
            if (problems.length) failures.push(`${label}: ${[...new Set(problems)].join(' | ')}`);
          } catch (e) {
            failures.push(`${label}: ${(e as Error).message.split('\n')[0]}`);
          }
        }
      }
      expect(failures, failures.join('\n')).toEqual([]);
    });
  }

  test('the rules screen opens for every game, and at phone width fits', async ({ page, request }) => {
    test.setTimeout(10 * 60_000);
    const failures: string[] = [];
    const problems = watchPage(page);
    const host = await guest(request);
    await signIn(page, host);
    await page.setViewportSize({ width: 375, height: 667 });

    for (const mod of modules) {
      problems.length = 0;
      try {
        await page.goto(`/rules?moduleId=${encodeURIComponent(mod.id)}`);
        await expect(page.getByText(/rules/i).first()).toBeVisible({ timeout: 15_000 });
        await expect(page.locator('body')).not.toContainText(/undefined|\[object Object\]|\{\{/, { timeout: 5_000 });
        const o = await overflow(page);
        if (o.cut.length) failures.push(`${mod.id}: rules past the ${o.innerWidth}px edge: ${o.cut.join(', ')}`);
        if (problems.length) failures.push(`${mod.id}: ${[...new Set(problems)].join(' | ')}`);
      } catch (e) {
        failures.push(`${mod.id}: ${(e as Error).message.split('\n')[0]}`);
      }
    }
    expect(failures, failures.join('\n')).toEqual([]);
  });

  test('the main menu lists every registered game', async ({ page, request }) => {
    const host = await guest(request);
    await signIn(page, host);
    await page.goto('/');
    await expect(page.getByTestId('games-list')).toBeVisible({ timeout: 15_000 });
    const missing: string[] = [];
    for (const mod of modules) {
      // Poker is the one menu entry for the whole holdem family.
      const visible = await page.getByTestId(`game-${mod.id}`).count();
      if (!visible) missing.push(mod.id);
    }
    expect(missing, `games with no menu entry: ${missing.join(', ')}`).toEqual([]);
  });
});
