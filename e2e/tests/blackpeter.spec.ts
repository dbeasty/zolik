import { expect, test, type Page } from '@playwright/test';

import { API_BASE, asViewer } from '../helpers/env';

/**
 * End-to-end for the Black Peter module (docs/blackpeter-rules.md).
 *
 * The Go suite proves the rules in memory: the 51-card pack, pairs going down,
 * the turn passing left, one loser a round. What only this can prove is the
 * rest of the stack, and the one thing Black Peter added to the shell: taking
 * a card out of another player's face-down hand by pointing at it.
 */

type Offer = {
  id: string;
  verb: string;
  enabled: boolean;
  whyNot?: string;
  source?: { zone?: string; ownerId?: string; zoneId?: string };
  target?: { zoneId?: string };
};

type Zone = { id: string; kind: string; ownerId?: string; cards?: { card: string; badgeKeys?: string[] }[]; count: number };

type MatchState = {
  moduleId: string;
  status: string;
  players: { id: string }[];
  view: { zones: Zone[] };
  legalActions: Offer[];
};

type Ctx = import('@playwright/test').APIRequestContext;
type User = { userId: string; accessToken: string; refreshToken: string; username?: string };

async function guest(request: Ctx): Promise<User> {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `peter-${Math.random().toString(36).slice(2, 10)}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

async function startMatch(request: Ctx, seats: number, options: Record<string, number> = {}) {
  const users: User[] = [];
  for (let i = 0; i < seats; i++) users.push(await guest(request));
  const auth = { Authorization: `Bearer ${users[0].accessToken}` };
  const created = await request.post(`${API_BASE}/matches`, { headers: auth, data: { moduleId: 'blackpeter', options } });
  expect(created.ok(), await created.text()).toBeTruthy();
  const { matchId } = await created.json();
  for (const u of users.slice(1)) {
    const joined = await request.post(`${API_BASE}/matches/${matchId}/join`, {
      headers: { Authorization: `Bearer ${u.accessToken}` },
    });
    expect(joined.ok(), await joined.text()).toBeTruthy();
  }
  const started = await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });
  expect(started.ok(), await started.text()).toBeTruthy();
  return { matchId, users };
}

async function stateFor(request: Ctx, matchId: string, user: User): Promise<MatchState> {
  const res = await request.get(`${API_BASE}/matches/${matchId}`, asViewer(user));
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

async function tableWithBots(request: Ctx, seats: number, options: Record<string, number> = {}) {
  const host = await guest(request);
  const auth = { Authorization: `Bearer ${host.accessToken}` };
  const created = await request.post(`${API_BASE}/matches`, { headers: auth, data: { moduleId: 'blackpeter', options } });
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

async function signIn(page: Page, host: User) {
  await page.addInitScript((s) => {
    window.localStorage.setItem('zolik_session', JSON.stringify(s));
  }, {
    accessToken: host.accessToken,
    refreshToken: host.refreshToken,
    userId: host.userId,
    username: host.username ?? 'peter',
    isGuest: true,
  });
}

test.describe('black peter', () => {
  test('the server offers Black Peter for two to six', async ({ request }) => {
    const { modules } = await (await request.get(`${API_BASE}/modules`)).json();
    const bp = modules.find((m: { id: string }) => m.id === 'blackpeter');
    expect(bp, 'blackpeter should be a hosted module').toBeTruthy();
    expect([bp.minPlayers, bp.maxPlayers]).toEqual([2, 6]);
    expect(bp.options.map((o: { name: string }) => o.name)).toEqual(expect.arrayContaining(['pairs', 'rounds']));
  });

  test('one seat may take a card, from the hand on its left, and nobody sees another hand', async ({ request }) => {
    const { matchId, users } = await startMatch(request, 3);
    const ids = users.map((u) => u.userId);
    let asked = 0;
    let cardsInHands = 0;
    for (const viewer of users) {
      const state = await stateFor(request, matchId, viewer);
      expect(state.moduleId).toBe('blackpeter');
      const draw = state.legalActions.find((o) => o.id === 'draw');
      expect(draw, 'every seat holding cards sees the draw').toBeTruthy();
      if (draw!.enabled) {
        asked++;
        // The source is another player's hand, drawn as a hand zone.
        expect(draw!.source?.ownerId).not.toBe(viewer.userId);
        expect(ids).toContain(draw!.source?.ownerId);
        expect(draw!.source?.zoneId).toBe(`hand:${draw!.source?.ownerId}`);
        expect(draw!.target?.zoneId).toBe(`hand:${viewer.userId}`);
      } else {
        expect(draw!.whyNot).toBe('NOT_YOUR_TURN');
      }
      for (const z of state.view.zones.filter((z) => z.kind === 'hand')) {
        if (z.ownerId === viewer.userId) {
          expect(z.cards!.length).toBe(z.count);
          cardsInHands += z.count;
          // Nobody holds a pair: they went down in the deal.
          const ranks = z.cards!.filter((c) => c.card !== 'JS').map((c) => c.card[0]);
          expect(new Set(ranks).size).toBe(ranks.length);
        } else {
          expect(z.cards ?? []).toHaveLength(0);
        }
      }
      const pairs = state.view.zones.find((z) => z.id === 'pairs')!;
      expect(pairs.count % 2).toBe(0);
      if (viewer === users[users.length - 1]) expect(cardsInHands + pairs.count).toBe(51);
    }
    expect(asked).toBe(1);
  });

  test('a whole match plays out over real WebSockets, pointing at cards', async ({ page, request }) => {
    test.setTimeout(240_000);
    const { matchId, users } = await startMatch(request, 3, { rounds: 3, pauseBetweenRounds: 0 });
    const wsBase = API_BASE.replace(/^http/, 'ws');

    const result = await page.evaluate(
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
        const latest = (seat: Seat) => {
          for (let i = seat.inbox.length - 1; i >= 0; i--) {
            if (seat.inbox[i].type === 'match_state') return seat.inbox[i];
          }
          return null;
        };
        const seats = await Promise.all(tokens.map(open));
        for (let i = 0; i < 200 && !seats.every(latest); i++) await new Promise((r) => setTimeout(r, 50));

        const errors: string[] = [];
        let draws = 0;
        for (let step = 0; step < 3000; step++) {
          const idx = seats.findIndex((s) => (latest(s)?.legalActions ?? []).some((o: any) => o.enabled));
          if (idx === -1) break;
          const state = latest(seats[idx]);
          if (state.status !== 'active') break;
          const offer = state.legalActions.find((o: any) => o.enabled);
          const action: any = { offerId: offer.id, verb: offer.verb };
          if (offer.verb === 'draw') {
            const from = state.view.zones.find((z: any) => z.id === offer.source.zoneId);
            // Point at the last back, as a player tapping the end of the fan.
            action.params = { pick: String(from.count - 1) };
            draws++;
          }
          const before = seats.map((s) => s.inbox.length);
          seats[idx].ws.send(JSON.stringify(action));
          for (let i = 0; i < 200; i++) {
            if (seats.every((s, k) => s.inbox.length > before[k])) break;
            await new Promise((r) => setTimeout(r, 15));
          }
          for (const s of seats) {
            const last = s.inbox[s.inbox.length - 1];
            if (last?.type === 'error') errors.push(`${last.code}: ${last.message}`);
          }
        }
        const final = seats.map(latest).find(Boolean) ?? {};
        for (const s of seats) s.ws.close();
        return { draws, errors, status: final.status };
      },
      { wsBase, matchId, tokens: users.map((u) => u.accessToken) },
    );

    expect(result.errors, `socket errors: ${result.errors.join('; ')}`).toEqual([]);
    expect(result.status).toBe('completed');
    expect(result.draws).toBeGreaterThan(0);

    const persisted: any = await stateFor(request, matchId, users[0]);
    const rounds = persisted.rounds?.rounds ?? [];
    expect(rounds).toHaveLength(3);
    for (const r of rounds) {
      // One mark a round: exactly one seat was left with Black Peter.
      expect(r.scores.filter((s: { shown?: number }) => s.shown === 1)).toHaveLength(1);
    }
  });

  test('on the board, the hand on your left is a fan of backs you take a card from', async ({ page, request }) => {
    test.setTimeout(180_000);
    const { matchId, host } = await tableWithBots(request, 3, { rounds: 1 });
    await signIn(page, host);
    await page.goto(`/match/${matchId}`);
    await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
    await expect(page.getByTestId('match-module')).toHaveAttribute('data-module', 'blackpeter');

    // Wait for the human's turn: the take control comes alive.
    const take = page.getByTestId('offer-draw');
    await expect(take).toBeVisible({ timeout: 60_000 });
    await expect(take).not.toHaveAttribute('aria-disabled', 'true', { timeout: 90_000 });

    const before = await stateFor(request, matchId, host);
    const offer = before.legalActions.find((o) => o.id === 'draw' && o.enabled)!;
    const fromZone = offer.source!.zoneId!;
    const fromCount = before.view.zones.find((z) => z.id === fromZone)!.count;

    // Every card in that hand is drawn as its own back, and each is a button.
    const backs = page.locator(`[data-testid^="card-${fromZone}-pick-"]`);
    await expect(backs).toHaveCount(fromCount);
    await expect(backs.first()).toHaveAttribute('aria-label', new RegExp(`1 .*${fromCount}`));

    // Take the last one, as a finger would.
    await page.getByTestId(`card-${fromZone}-pick-${fromCount - 1}`).click();

    await expect
      .poll(async () => (await stateFor(request, matchId, host)).view.zones.find((z) => z.id === fromZone)?.count ?? 0, {
        timeout: 15_000,
      })
      .toBeLessThan(fromCount);
  });
});
