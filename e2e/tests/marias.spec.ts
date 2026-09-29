import { expect, test } from '@playwright/test';

import { API_BASE } from '../helpers/env';

/**
 * End-to-end for the Mariáš module (docs/marias-plan.md, step 3).
 *
 * The Go suites prove the rules in memory: `marias/engine_test.go` pins the
 * auction, the doubling round, the play obligations and the settlement, and
 * the shared contract suite plays whole matches from offers alone. What only
 * this can prove is the rest of the stack — real HTTP, real WebSockets, real
 * persistence — for the first trick-taking game, and the first to lay its
 * trick out by seat.
 *
 * Like the other game specs, the play-through reads the offer list and
 * nothing else; it names no rule of Mariáš.
 */

type Offer = {
  id: string;
  verb: string;
  enabled: boolean;
  source?: { cards?: string[]; minCards?: number; submit?: string[] };
};

type Zone = {
  id: string;
  kind: string;
  ownerId?: string;
  cards?: { card: string; by?: string }[];
  count: number;
  shared?: boolean;
  arrange?: string;
};

type MatchState = {
  moduleId: string;
  status: string;
  players: { id: string }[];
  view: { zones: Zone[]; header?: { labelKey: string }[] };
  legalActions: Offer[];
};

type Ctx = import('@playwright/test').APIRequestContext;

async function guest(request: Ctx) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `marias-${Math.random().toString(36).slice(2, 10)}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

/**
 * Opens a Mariáš table with three real players and starts it. Real players
 * rather than bots, so the whole match is driven — and timed — by the test.
 */
async function startMatch(request: Ctx, options: Record<string, number> = {}) {
  const users = [await guest(request), await guest(request), await guest(request)];
  const auth = { Authorization: `Bearer ${users[0].accessToken}` };
  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    data: { moduleId: 'marias', options },
  });
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

async function stateFor(request: Ctx, matchId: string, user: { userId: string; accessToken: string }): Promise<MatchState> {
  const res = await request.get(`${API_BASE}/matches/${matchId}?as=${encodeURIComponent(user.userId)}`, {
    headers: { Authorization: `Bearer ${user.accessToken}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

test.describe('marias', () => {
  test('the server offers Mariáš for exactly three players', async ({ request }) => {
    const { modules } = await (await request.get(`${API_BASE}/modules`)).json();
    const marias = modules.find((m: { id: string }) => m.id === 'marias');
    expect(marias, 'marias should be a hosted module').toBeTruthy();
    expect(marias.minPlayers).toBe(3);
    expect(marias.maxPlayers).toBe(3);
    expect(marias.variations.map((v: { id: string }) => v.id)).toContain('voleny');
  });

  test('only the chooser is asked anything, and nobody sees another hand', async ({ request }) => {
    const { matchId, users } = await startMatch(request);
    const asked: string[] = [];
    for (const viewer of users) {
      const state = await stateFor(request, matchId, viewer);
      expect(state.moduleId).toBe('marias');
      if (state.legalActions.some((o) => o.enabled)) asked.push(viewer.userId);
      for (const z of state.view.zones.filter((z) => z.kind === 'hand')) {
        if (z.ownerId === viewer.userId) {
          expect(z.cards!.length).toBe(z.count);
        } else {
          expect(z.cards ?? []).toHaveLength(0);
        }
      }
    }
    expect(asked).toHaveLength(1);
  });

  test('a whole match plays out over real WebSockets, trick by trick', async ({ page, request }) => {
    test.setTimeout(240_000);
    const { matchId, users } = await startMatch(request, { deals: 9, pauseBetweenRounds: 0 });
    const wsBase = API_BASE.replace(/^http/, 'ws');

    const result = await page.evaluate(
      async ({ wsBase, matchId, tokens, ids }) => {
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

        const submissionFor = (o: any) => {
          const action: any = { offerId: o.id, verb: o.verb };
          const need = o.source?.minCards ?? 0;
          if (o.source?.submit?.length) action.cards = o.source.submit;
          else if (need > 0) {
            const cards = o.source?.cards ?? [];
            if (cards.length < need) return null;
            action.cards = cards.slice(0, need);
          }
          return action;
        };
        // Doubling is left out: with no limit, a driver that always doubled
        // would double for ever. "pass" closes the round.
        const order = ['play_card', 'discard', 'choose_trump', 'announce', 'good', 'pass', 'continue'];

        const verbs: Record<string, number> = {};
        const errors: string[] = [];
        let arrangedOk = true;
        let tricksSeen = 0;
        let moves = 0;
        for (let step = 0; step < 3000; step++) {
          const idx = seats.findIndex((s) => (latest(s)?.legalActions ?? []).some((o: any) => o.enabled));
          if (idx === -1) break;
          const state = latest(seats[idx]);
          if (state.status !== 'active') break;

          // Every card in the trick names a seated player, so the shell can
          // lay it out by seat.
          const trick = state.view.zones.find((z: any) => z.arrange === 'bySeat');
          if (trick?.cards?.length) {
            tricksSeen++;
            if (!trick.cards.every((c: any) => ids.includes(c.by))) arrangedOk = false;
          }

          const enabled = state.legalActions.filter((o: any) => o.enabled && order.includes(o.verb));
          enabled.sort((a: any, b: any) => order.indexOf(a.verb) - order.indexOf(b.verb));
          const action = enabled.map(submissionFor).find(Boolean);
          if (!action) break;

          const before = seats.map((s) => s.inbox.length);
          seats[idx].ws.send(JSON.stringify(action));
          verbs[action.verb] = (verbs[action.verb] ?? 0) + 1;
          moves++;
          for (let i = 0; i < 200; i++) {
            if (seats.every((s, k) => s.inbox.length > before[k])) break;
            await new Promise((r) => setTimeout(r, 20));
          }
          for (const s of seats) {
            const last = s.inbox[s.inbox.length - 1];
            if (last?.type === 'error') errors.push(`${last.code}: ${last.message}`);
          }
        }
        const final = seats.map(latest).find(Boolean) ?? {};
        for (const s of seats) s.ws.close();
        return { moves, verbs, errors, arrangedOk, tricksSeen, status: final.status };
      },
      { wsBase, matchId, tokens: users.map((u) => u.accessToken), ids: users.map((u) => u.userId) },
    );

    expect(result.errors, `socket errors: ${result.errors.join('; ')}`).toEqual([]);
    expect(result.status).toBe('completed');
    expect(result.arrangedOk).toBe(true);
    expect(result.tricksSeen).toBeGreaterThan(0);
    for (const verb of ['choose_trump', 'announce', 'discard', 'play_card']) {
      expect(result.verbs[verb] ?? 0, `${verb} was never played`).toBeGreaterThan(0);
    }

    // And it was recorded: nine deals in the round log, every one zero-sum.
    const persisted = await stateFor(request, matchId, users[0]);
    expect(persisted.status).toBe('completed');
    const rounds = (persisted as any).rounds?.rounds ?? [];
    expect(rounds).toHaveLength(9);
    for (const r of rounds) {
      const sum = r.scores.reduce((a: number, s: { delta: number }) => a + s.delta, 0);
      expect(sum, `deal ${r.number} is not zero-sum`).toBe(0);
    }
  });
});
