import { expect, test, type APIRequestContext, type Page } from '@playwright/test';

import { API_BASE, WEB_BASE } from '../helpers/env';
import { loginAsFreshAccount, seedIntroSeen } from '../helpers/login';

/**
 * A table on a phone, played from a browser in the room, and a game that
 * reaches the cloud only because its players said so.
 *
 * Runs against `scripts/phone-e2e-stack.sh`: a cloud, and `cmd/phonehost` -
 * the very package the apps embed - enrolled as a fresh account and serving
 * the web client on its room listener. Skipped without it.
 *
 *  1. The phone's owner opens a table (over the phone's own loopback, as the
 *     app does).
 *  2. Somebody in the room opens the shared link in a browser - no app, no
 *     account - picks a name and sits down. The page talks to the phone.
 *  3. The game is played to the end; the browser guest says yes to keeping
 *     their seat and leaves with a claim link to the cloud.
 *  4. Nothing has reached the cloud. The owner saves the game; the room
 *     cannot see or save the phone's games.
 *  5. The game arrives in the cloud credited to the owner, with the guest's
 *     seat as a guest. The guest, later, signs in on the cloud and opens their
 *     link: the game is theirs.
 */

const OWN = process.env.ZOLIK_E2E_PHONE_OWN ?? '';
const ROOM = process.env.ZOLIK_E2E_PHONE_ROOM ?? '';
const OWNER = process.env.ZOLIK_E2E_PHONE_OWNER ?? '';
const OWNER_PASS = process.env.ZOLIK_E2E_PHONE_OWNER_PASS ?? '';
const OWNER_TOKEN = process.env.ZOLIK_E2E_PHONE_OWNER_TOKEN ?? '';

test.skip(!OWN || !ROOM || !OWNER_PASS, 'needs scripts/phone-e2e-stack.sh');
test.describe.configure({ timeout: 240_000 });

type Ctx = APIRequestContext;

async function postOk(request: Ctx, url: string, token: string, data: unknown = {}) {
  const res = await request.post(url, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
    data,
  });
  expect(res.ok(), `${url}: ${res.status()} ${await res.text()}`).toBeTruthy();
  return res.json().catch(() => ({}));
}

async function getJson(request: Ctx, url: string, token = '') {
  const res = await request.get(url, { headers: token ? { Authorization: `Bearer ${token}` } : {} });
  return { status: res.status(), body: res.ok() ? await res.json() : null };
}

/** Plays every seat whose token is given, over the phone's sockets, to the end. */
async function playToTheEnd(page: Page, wsBase: string, matchId: string, tokens: string[]) {
  return page.evaluate(
    async ({ wsBase, matchId, tokens }) => {
      type Seat = { ws: WebSocket; inbox: any[] };
      const open = async (token: string): Promise<Seat> => {
        const ws = new WebSocket(`${wsBase}/ws/matches/${matchId}?token=${encodeURIComponent(token)}`);
        const inbox: any[] = [];
        await new Promise<void>((resolve, reject) => {
          ws.onopen = () => resolve();
          ws.onerror = () => reject(new Error('socket failed to open'));
          setTimeout(() => reject(new Error('socket open timed out')), 10000);
        });
        ws.onmessage = (ev) => inbox.push(JSON.parse(String(ev.data)));
        return { ws, inbox };
      };
      const latest = (s: Seat) => {
        for (let i = s.inbox.length - 1; i >= 0; i--) if (s.inbox[i].type === 'match_state') return s.inbox[i];
        return null;
      };
      const seats = await Promise.all(tokens.map(open));
      for (let i = 0; i < 200 && !seats.every(latest); i++) await new Promise((r) => setTimeout(r, 50));

      const submissionFor = (o: any) => {
        const action: any = { offerId: o.id, verb: o.verb };
        for (const p of o.params ?? []) {
          if (p.kind === 'int') {
            const v = p.default >= p.min && p.default <= p.max ? p.default : p.min;
            action.params = { ...(action.params ?? {}), [p.name]: String(v) };
          } else if ((p.choices ?? []).length > 0) {
            action.params = { ...(action.params ?? {}), [p.name]: p.choices[0].value };
          } else {
            return null;
          }
        }
        return action;
      };
      const order = ['bet', 'decline_insurance', 'stand', 'hit', 'double', 'split', 'surrender', 'continue'];

      let status = 'active';
      for (let step = 0; step < 2000; step++) {
        const states = seats.map(latest);
        status = states.find((s) => s)?.status ?? status;
        if (states.some((s) => s && s.status === 'completed')) {
          status = 'completed';
          break;
        }
        const idx = seats.findIndex((s) => (latest(s)?.legalActions ?? []).some((o: any) => o.enabled));
        if (idx === -1) {
          await new Promise((r) => setTimeout(r, 50));
          continue;
        }
        const seat = seats[idx];
        const enabled = latest(seat).legalActions.filter((o: any) => o.enabled);
        enabled.sort((a: any, b: any) => order.indexOf(a.verb) - order.indexOf(b.verb));
        let action: any = null;
        for (const o of enabled) {
          action = submissionFor(o);
          if (action) break;
        }
        if (!action) break;
        const before = seat.inbox.length;
        seat.ws.send(JSON.stringify(action));
        for (let i = 0; i < 200 && seat.inbox.length === before; i++) await new Promise((r) => setTimeout(r, 20));
      }
      for (const s of seats) s.ws.close();
      return status;
    },
    { wsBase, matchId, tokens },
  );
}

test('a browser in the room plays at the phone, and keeps the game only by saying yes', async ({
  browser,
  request,
}) => {
  // 1. The owner's own app opens a table on the phone.
  const host = await postOk(request, `${OWN}/auth/offline-pass`, '', { offlinePass: OWNER_PASS });
  expect(host.userId).toBe(`user:${OWNER}`);
  const { matchId } = await postOk(request, `${OWN}/matches`, host.accessToken, {
    moduleId: 'blackjack',
    variation: 'atlantic',
    options: { rounds: 5, startingStack: 200, minBet: 5 },
  });
  const table = await getJson(request, `${OWN}/matches/${matchId}`, host.accessToken);
  const joinCode: string = table.body.joinCode;
  expect(joinCode).toBeTruthy();

  // 2. Somebody in the room follows the link, in a browser, with no account.
  const roomContext = await browser.newContext();
  const page = await roomContext.newPage();
  await seedIntroSeen(page);
  await page.goto(`${ROOM}/join/${joinCode}`);
  await expect(page).toHaveURL(/\/auth\/guest/);
  await page.getByPlaceholder('Display name').fill('Bea');
  await page.getByText('Continue', { exact: true }).click();
  await expect(page.getByTestId('lobby-joined')).toBeVisible({ timeout: 20_000 });
  await expect(page.locator('[data-testid^="lobby-player-"]').filter({ hasText: 'Bea' })).toHaveCount(1);
  // The link this page offers names the phone, so it works for the next
  // person in the room too.
  await expect(page.getByTestId('invite-url')).toHaveText(`${ROOM}/join/${joinCode}`);

  // What the browser holds is a session on the phone, not on the cloud.
  // A second person in the room sits down while Bea waits: Bea is told.
  const otherContext = await browser.newContext();
  const other = await otherContext.newPage();
  await seedIntroSeen(other);
  await other.goto(`${ROOM}/join/${joinCode}`);
  await other.getByPlaceholder('Display name').fill('Cyd');
  await other.getByText('Continue', { exact: true }).click();
  await expect(other.getByTestId('lobby-joined')).toBeVisible({ timeout: 20_000 });
  await expect(page.getByTestId('arrival-banner')).toContainText('Cyd joined the table', { timeout: 10_000 });
  const cyd = await other.evaluate(() => JSON.parse(window.localStorage.getItem('zolik_session') ?? '{}'));
  await otherContext.close();

  const bea = await page.evaluate(() => JSON.parse(window.localStorage.getItem('zolik_session') ?? '{}'));
  expect(bea.isGuest).toBe(true);
  expect(bea.seatReceipt, 'the phone signs a receipt for the seat').toBeTruthy();

  await postOk(request, `${OWN}/matches/${matchId}/start`, host.accessToken);
  await expect(page).toHaveURL(new RegExp(`/match/${matchId}`), { timeout: 20_000 });

  // 3. The game is played to its end.
  const status = await playToTheEnd(page, ROOM.replace(/^http/, 'ws'), matchId, [
    host.accessToken,
    bea.accessToken,
    cyd.accessToken,
  ]);
  expect(status).toBe('completed');
  // The test's own socket for Bea's seat displaced the page's ("opened in
  // another tab"); coming back is what a player would do.
  await page.reload();

  const prompt = page.getByTestId('save-game');
  await expect(prompt).toBeVisible({ timeout: 20_000 });
  await expect(prompt).toContainText('Keep this game for a Jokerless account?');
  await page.getByTestId('save-game-yes').click();
  const linkText = page.getByTestId('save-game-claim-link');
  await expect(linkText).toBeVisible();
  const claimLink = (await linkText.textContent())!.trim();
  expect(claimLink.startsWith(`${WEB_BASE}/claim#r=`), claimLink).toBeTruthy();

  // Cyd keeps their seat out of it; every seat has now answered.
  await postOk(request, `${ROOM}/matches/${matchId}/save-consent`, cyd.accessToken, { save: false });

  // 4. Nothing has left the phone. The room cannot list or save its games;
  //    the owner's own app can.
  expect((await request.get(`${ROOM}/local/saves`)).status()).toBe(404);
  expect((await request.post(`${ROOM}/local/saves/${matchId}/save`)).status()).toBe(404);
  const waiting = await getJson(request, `${OWN}/local/saves`);
  const game = waiting.body.games.find((g: any) => g.matchId === matchId);
  expect(game.state).toBe('pending');
  expect(game.seats.find((s: any) => s.name === 'Bea').consent).toBe(true);
  expect((await getJson(request, `${API_BASE}/matches/${matchId}`, OWNER_TOKEN)).status).not.toBe(200);

  const saved = await postOk(request, `${OWN}/local/saves/${matchId}/save`, '');
  expect(saved.state).toBe('saved');

  // 5. It reaches the cloud, credited to the owner and, as a guest, to Bea.
  await expect
    .poll(async () => (await getJson(request, `${API_BASE}/matches/${matchId}`, OWNER_TOKEN)).status, {
      timeout: 150_000,
      intervals: [2_000],
    })
    .toBe(200);
  const inCloud = (await getJson(request, `${API_BASE}/matches/${matchId}`, OWNER_TOKEN)).body;
  const names = inCloud.players.map((p: any) => p.name);
  expect(names).toContain('Bea');
  expect(names.some((n: string) => /owner/.test(n))).toBeTruthy();
  // Cyd said no: an anonymous seat, with no name.
  expect(names).not.toContain('Cyd');

  // And the phone lets the bundle go once it sees the game in its owner's
  // history.
  await expect
    .poll(async () => (await getJson(request, `${OWN}/local/saves`)).body.games.find((g: any) => g.matchId === matchId)?.state, {
      timeout: 90_000,
      intervals: [2_000],
    })
    .toBe('uploaded');

  // Later, online, Bea signs in on the cloud and opens the link she kept.
  const cloudContext = await browser.newContext();
  const cloudPage = await cloudContext.newPage();
  const account = await loginAsFreshAccount(cloudPage, request, `bea-${Date.now()}@phone.test`);
  await cloudPage.goto(claimLink);
  await expect(cloudPage.getByTestId('claim-done')).toContainText('Done', { timeout: 20_000 });
  await expect(cloudPage).not.toHaveURL(/#r=/);

  await expect
    .poll(
      async () => {
        const res = await getJson(request, `${API_BASE}/users/me/stats`, account.accessToken);
        return res.body?.overall?.matches ?? 0;
      },
      { timeout: 30_000, intervals: [1_000] },
    )
    .toBe(1);

  await roomContext.close();
  await cloudContext.close();
});
