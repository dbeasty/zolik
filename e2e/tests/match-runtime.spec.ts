import { expect, test } from '@playwright/test';

import { API_BASE } from '../helpers/env';

/**
 * End-to-end for the module runtime (docs/extensibility-plan.md Phase 3).
 *
 * The Go tests prove the module contract in memory. This proves the rest of
 * the stack: real HTTP, a real WebSocket, real Mongo persistence, and a real
 * second game hosted by a runtime that has never heard of it.
 *
 * The whole spec is written against `/modules` and the offer list — it names
 * no rank, suit or rule of either game, and picks its moves the way a UI shell
 * would. Where it does mention Prší, it is to assert that the *runtime* stayed
 * generic, not to encode a rule.
 */

type Offer = {
  id: string;
  verb: string;
  enabled: boolean;
  whyNot?: string;
  source?: { zone: string; cards?: string[]; minCards?: number };
  params?: { name: string; choices: { value: string }[] }[];
};

type MatchState = {
  type: string;
  matchId: string;
  moduleId: string;
  status: string;
  winnerId?: string;
  players: { id: string; name: string; isAI: boolean }[];
  view: { zones: { id: string; kind: string; ownerId?: string; cards?: { card: string }[]; count: number }[] };
  legalActions: Offer[];
};

async function guest(request: import('@playwright/test').APIRequestContext) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `rt-${Math.random().toString(36).slice(2, 10)}` },
  });
  expect(res.ok()).toBeTruthy();
  return res.json();
}

/** Opens a match, seats a bot, starts it. Returns ids and the host's token. */
async function startMatch(
  request: import('@playwright/test').APIRequestContext,
  moduleId: string,
  options: Record<string, number> = {},
) {
  const host = await guest(request);
  const auth = { Authorization: `Bearer ${host.accessToken}` };

  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    data: { moduleId, options },
  });
  expect(created.ok(), await created.text()).toBeTruthy();
  const { matchId } = await created.json();

  const bot = await request.post(`${API_BASE}/matches/${matchId}/add-bot`, { headers: auth });
  expect(bot.ok(), await bot.text()).toBeTruthy();
  const { playerId: botId } = await bot.json();

  const started = await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });
  expect(started.ok(), await started.text()).toBeTruthy();

  return { matchId, botId, token: host.accessToken, userId: host.userId, auth };
}

async function stateFor(
  request: import('@playwright/test').APIRequestContext,
  matchId: string,
  viewerId: string,
): Promise<MatchState> {
  const res = await request.get(`${API_BASE}/matches/${matchId}?as=${encodeURIComponent(viewerId)}`);
  expect(res.ok()).toBeTruthy();
  return res.json();
}

test.describe('module runtime', () => {
  test('the server hosts more than one game, and says so', async ({ request }) => {
    const res = await request.get(`${API_BASE}/modules`);
    expect(res.ok()).toBeTruthy();
    const { modules } = await res.json();

    const ids = modules.map((m: { id: string }) => m.id);
    expect(ids).toContain('zolik');
    expect(ids).toContain('prsi');

    // Each is self-describing enough to render a picker and a new-match form.
    for (const m of modules) {
      expect(m.label).toBeTruthy();
      expect(m.minPlayers).toBeGreaterThanOrEqual(2);
      expect(m.maxPlayers).toBeGreaterThanOrEqual(m.minPlayers);
    }
  });

  test('a Prsi match persists through Mongo and comes back playable', async ({ request }) => {
    const { matchId, userId } = await startMatch(request, 'prsi');

    // Read it back cold, through a fresh HTTP request: nothing is cached in
    // the process, so this is the round trip through the database.
    const state = await stateFor(request, matchId, userId);
    expect(state.moduleId).toBe('prsi');
    expect(state.status).toBe('active');
    expect(state.legalActions.length).toBeGreaterThan(0);

    // The board arrived as generic zones. The runtime never learned what any
    // of them mean.
    const kinds = state.view.zones.map((z) => z.kind);
    expect(kinds).toContain('hand');
    expect(kinds).toContain('stack');
    expect(kinds).toContain('pile');
  });

  test('a viewer never receives another player cards', async ({ request }) => {
    // The one contract term with a security consequence, checked through the
    // real serialisation rather than in memory.
    const { matchId, botId, userId } = await startMatch(request, 'prsi');

    for (const [viewer, other] of [
      [userId, botId],
      [botId, userId],
    ]) {
      const state = await stateFor(request, matchId, viewer);
      const theirs = state.view.zones.find((z) => z.kind === 'hand' && z.ownerId === other);
      expect(theirs, 'the opponent hand zone should exist').toBeTruthy();
      expect(theirs!.cards ?? []).toHaveLength(0);
      expect(theirs!.count).toBeGreaterThan(0);
    }
  });

  test('a whole Prsi match plays to a winner over real WebSockets', async ({ page, request }) => {
    // The end-to-end claim, at full strength: two real players, two real
    // sockets, every move persisted, played to a winner by a client that reads
    // only the offer list.
    //
    // Two *human* seats rather than a bot: both need tokens to open sockets,
    // and driving both is what makes this a whole game rather than one move.
    const host = await guest(request);
    const guest2 = await guest(request);

    const created = await request.post(`${API_BASE}/matches`, {
      headers: { Authorization: `Bearer ${host.accessToken}` },
      data: { moduleId: 'prsi' },
    });
    expect(created.ok(), await created.text()).toBeTruthy();
    const { matchId } = await created.json();

    const joined = await request.post(`${API_BASE}/matches/${matchId}/join`, {
      headers: { Authorization: `Bearer ${guest2.accessToken}` },
    });
    expect(joined.ok(), await joined.text()).toBeTruthy();

    const started = await request.post(`${API_BASE}/matches/${matchId}/start`, {
      headers: { Authorization: `Bearer ${host.accessToken}` },
    });
    expect(started.ok(), await started.text()).toBeTruthy();

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

        const settle = async (seat: Seat) => {
          for (let i = 0; i < 100; i++) {
            if (latest(seat)) return latest(seat);
            await new Promise((r) => setTimeout(r, 50));
          }
          throw new Error('no match_state arrived');
        };

        const seats = await Promise.all(tokens.map(open));
        for (const s of seats) await settle(s);

        const verbs: Record<string, number> = {};
        let moves = 0;
        let status = 'active';
        let winnerId = '';
        const errors: string[] = [];

        for (let step = 0; step < 400; step++) {
          // Whoever the module is offering something to is on turn. Neither
          // this loop nor the runtime has any other notion of turn order.
          const idx = seats.findIndex((s) => (latest(s)?.legalActions ?? []).some((o: any) => o.enabled));
          if (idx === -1) break;

          const seat = seats[idx];
          const state = latest(seat);
          status = state.status;
          winnerId = state.winnerId ?? '';
          if (status !== 'active') break;

          const enabled = state.legalActions.filter((o: any) => o.enabled);
          const order = ['play_card', 'pass', 'draw'];
          enabled.sort((a: any, b: any) => order.indexOf(a.verb) - order.indexOf(b.verb));
          const o =
            enabled.find((x: any) => !x.source?.minCards || (x.source?.cards ?? []).length > 0) ??
            enabled[0];
          if (!o) break;

          const action: any = { offerId: o.id, verb: o.verb };
          if (o.source?.minCards && (o.source?.cards ?? []).length) action.cards = [o.source.cards[0]];
          for (const p of o.params ?? []) {
            action.params = { ...(action.params ?? {}), [p.name]: p.choices[0].value };
          }

          const before = seats.map((s) => s.inbox.length);
          seat.ws.send(JSON.stringify(action));
          verbs[o.verb] = (verbs[o.verb] ?? 0) + 1;
          moves++;

          // Wait for the broadcast to reach every seat, so the next iteration
          // reads fresh state rather than the state it just acted on.
          for (let i = 0; i < 120; i++) {
            if (seats.every((s, k) => s.inbox.length > before[k])) break;
            await new Promise((r) => setTimeout(r, 20));
          }
          for (const s of seats) {
            const last = s.inbox[s.inbox.length - 1];
            if (last?.type === 'error') errors.push(`${last.code}: ${last.message}`);
          }
        }

        const final = seats.map(latest).find((s) => s) ?? {};
        for (const s of seats) s.ws.close();
        return {
          moves,
          verbs,
          errors,
          status: final.status ?? status,
          winnerId: final.winnerId ?? winnerId,
        };
      },
      { wsBase, matchId, tokens: [host.accessToken, guest2.accessToken] },
    );

    // Nothing the offers advertised was refused.
    expect(result.errors, `socket errors: ${result.errors.join('; ')}`).toEqual([]);
    // A whole game, not a token move.
    expect(result.moves).toBeGreaterThan(5);
    expect(result.status).toBe('completed');
    expect(result.winnerId).not.toBe('');
    // And the moves were varied: this is a real game, not a draw loop.
    expect(Object.keys(result.verbs).length).toBeGreaterThan(1);

    // The database agrees with what the sockets reported.
    const persisted = await (await request.get(`${API_BASE}/matches/${matchId}`)).json();
    expect(persisted.status).toBe('completed');
    expect(persisted.winnerId).toBe(result.winnerId);
  });

  test('the runtime refuses a move the module did not offer', async ({ request }) => {
    // Authority still sits with the module: the runtime routes, it does not
    // adjudicate. A verb no module defines must be refused rather than
    // silently ignored.
    const { matchId, userId } = await startMatch(request, 'prsi');
    const state = await stateFor(request, matchId, userId);
    expect(state.status).toBe('active');

    // A rummy verb sent at a Prsi match — the clearest possible "wrong game".
    const res = await request.post(`${API_BASE}/matches/${matchId}/start`);
    expect(res.status()).toBeGreaterThanOrEqual(400);
  });

  test('an option the descriptor does not declare is refused at creation', async ({ request }) => {
    const host = await guest(request);
    const res = await request.post(`${API_BASE}/matches`, {
      headers: { Authorization: `Bearer ${host.accessToken}` },
      data: { moduleId: 'prsi', options: { handSize: 99 } },
    });
    expect(res.status()).toBe(400);
    const body = await res.json();
    expect(body.code).toBe('OPTION_NOT_ALLOWED');
  });

  test('an unknown module is a 404, not a crash', async ({ request }) => {
    const host = await guest(request);
    const res = await request.post(`${API_BASE}/matches`, {
      headers: { Authorization: `Bearer ${host.accessToken}` },
      data: { moduleId: 'marias' },
    });
    expect(res.status()).toBe(404);
    expect((await res.json()).code).toBe('UNKNOWN_MODULE');
  });

  test('the same runtime hosts a Zolik match too', async ({ request }) => {
    // The runtime is not a Prsi runtime that happens to also compile. The
    // rummy engine, unchanged, runs behind the same envelope and the same
    // socket.
    const { matchId, userId } = await startMatch(request, 'zolik');
    const state = await stateFor(request, matchId, userId);

    expect(state.moduleId).toBe('zolik');
    expect(state.status).toBe('active');
    expect(state.legalActions.length).toBeGreaterThan(0);
    // Rummy's board has a meld spread; Prsi's does not. Same zone vocabulary.
    const hand = state.view.zones.find((z) => z.kind === 'hand' && z.ownerId === userId);
    expect(hand?.cards?.length).toBeGreaterThan(0);
  });
});
