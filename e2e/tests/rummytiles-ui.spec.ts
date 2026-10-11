import { expect, test, type Page } from '@playwright/test';

import { API_BASE, asViewer } from '../helpers/env';
import { selectedCodes, selectOnly } from '../helpers/hand';

type Ctx = import('@playwright/test').APIRequestContext;

async function table(request: Ctx) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `tiles-${Math.random().toString(36).slice(2, 10)}` },
  });
  const host = await res.json();
  const auth = { Authorization: `Bearer ${host.accessToken}` };
  const created = await request.post(`${API_BASE}/matches`, { headers: auth, data: { moduleId: 'rummytiles', options: {} } });
  expect(created.ok(), await created.text()).toBeTruthy();
  const { matchId } = await created.json();
  await request.post(`${API_BASE}/matches/${matchId}/add-bot`, { headers: auth });
  await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });
  return { matchId, host };
}

async function open(page: Page, host: any, matchId: string) {
  await page.addInitScript(
    (s) => window.localStorage.setItem('zolik_session', JSON.stringify(s)),
    { accessToken: host.accessToken, refreshToken: host.refreshToken, userId: host.userId, username: 'tiles', isGuest: true },
  );
  await page.goto(`/match/${matchId}`);
  await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
}

async function seed(request: Ctx, matchId: string, host: any, hand: string[], opts: { initial?: boolean; sets?: any[] } = {}) {
  const body = await (await request.get(`${API_BASE}/matches/${matchId}`, asViewer(host))).json();
  const bot = body.players.find((p: any) => p.id !== host.userId).id;
  const state = {
    status: 'active', variation: body.variation ?? '', players: [host.userId, bot], current: host.userId,
    hands: { [host.userId]: hand, [bot]: ['1-R', '2-K', '4-O', '6-B', '8-R', '9-O', '12-K'] },
    pool: ['3-B', '3-K', '5-B', '5-K', '6-K', '7-K', '8-K', '9-K', '10-K', '11-K'],
    sets: opts.sets ?? [], workspace: { sets: opts.sets ?? [] }, initialMeld: { [host.userId]: !!opts.initial, [bot]: true },
    nextSetId: 10, roundNumber: 1, scores: { [host.userId]: 0, [bot]: 0 }, targetScore: 100, roundLimit: 0, seed: 7,
  };
  const res = await request.post(`${API_BASE}/matches/${matchId}/debug-state`, {
    headers: { Authorization: `Bearer ${host.accessToken}` }, data: { state },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
}

async function tableSets(request: Ctx, matchId: string, host: any): Promise<string[][]> {
  const b = await (await request.get(`${API_BASE}/matches/${matchId}`, asViewer(host))).json();
  const zone = b.view.zones.find((z: any) => z.kind === 'spread' && z.id === 'table');
  return (zone?.groups ?? []).map((g: any) => g.cards);
}

const HAND = ['10-R', '11-R', '12-R', '2-B', '5-O', '9-K', '1-B', '7-O', '13-K', '4-B'];

test.describe('Rummy Tiles in the real UI', () => {
  test('tiles show their faces: number and shape, in the hand', async ({ page, request }) => {
    test.setTimeout(60_000);
    const { matchId, host } = await table(request);
    await open(page, host, matchId);
    const faces = await expect
      .poll(async () => page.evaluate(() => [...document.querySelectorAll('[data-testid^="card-hand:"]')].map((c) => (c as HTMLElement).innerText.trim())))
      .toHaveLength(14)
      .then(() => page.evaluate(() => [...document.querySelectorAll('[data-testid^="card-hand:"]')].map((c) => (c as HTMLElement).innerText.trim())));
    for (const f of faces) {
      expect(f, 'a tile face reads as its number and a colour shape').toMatch(/^(\d{1,2}\s*[●■▲◆]|JKR)/);
      expect(f).not.toContain('?');
    }
  });

  test('hint selects the tiles and Place then works', async ({ page, request }) => {
    test.setTimeout(60_000);
    const { matchId, host } = await table(request);
    await seed(request, matchId, host, HAND);
    await open(page, host, matchId);
    await page.getByTestId('hint-button').click();
    await expect(page.getByTestId('hint-line')).toBeVisible();
    await expect.poll(async () => (await selectedCodes(page)).sort()).toEqual(['10-R', '11-R', '12-R']);
    const place = page.getByTestId('offer-place');
    await expect(place).toBeEnabled();
    await expect(page.getByText('Select just one card')).toHaveCount(0);
    await place.click();
    await expect.poll(() => tableSets(request, matchId, host)).toEqual([['10-R', '11-R', '12-R']]);
    // The Add button names its target as tiles, not as raw codes.
    await expect(page.getByText('10-R,11-R,12-R')).toHaveCount(0);
    // What the player sees: the tiles are on the table with their faces.
    await expect(page.getByTestId('card-s10-0')).toContainText('10');
  });

  test('picking three tiles by hand and pressing Place works', async ({ page, request }) => {
    test.setTimeout(60_000);
    const { matchId, host } = await table(request);
    await seed(request, matchId, host, HAND);
    await open(page, host, matchId);
    await selectOnly(page, ['10-R', '11-R', '12-R']);
    await page.getByTestId('offer-place').click();
    await expect.poll(() => tableSets(request, matchId, host)).toEqual([['10-R', '11-R', '12-R']]);
  });

  test('a tile moves from one meld to another through the tray', async ({ page, request }) => {
    test.setTimeout(60_000);
    await page.setViewportSize({ width: 1280, height: 1800 });
    const { matchId, host } = await table(request);
    await seed(request, matchId, host, ['2-B', '5-O', '9-K'], {
      initial: true,
      sets: [
        { id: 's1', kind: 'run', cards: ['5-R', '6-R', '7-R', '8-R'] },
        { id: 's2', kind: 'group', cards: ['8-B', '8-K', '8-O'] },
      ],
    });
    await open(page, host, matchId);
    // Open the run, pick its last tile: it goes to the tray.
    await page.getByTestId('group-toggle-s1').click();
    await page.getByTestId('card-s1-3').click();
    await expect(page.getByText('Now tap the meld to move them to.')).toBeVisible();
    await page.getByTestId('group-press-s1').click();
    await expect(page.getByTestId('zone-count-tray')).toHaveText('1');
    await expect(page.getByTestId('card-tray-0')).toContainText('8');
    // Pick it up from the tray and drop it on the group.
    await page.getByTestId('lift-8-R').click();
    await page.getByTestId('group-press-s2').click();
    await expect.poll(() => tableSets(request, matchId, host)).toEqual([
      ['5-R', '6-R', '7-R'],
      ['8-B', '8-K', '8-O', '8-R'],
    ]);
    // Nothing is left loose (Done still wants a tile played from the hand, as the rules say).
    await expect(page.getByTestId('zone-count-tray')).toHaveCount(0);
  });
});
