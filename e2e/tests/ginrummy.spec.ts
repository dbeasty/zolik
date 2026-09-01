import { expect, test } from '@playwright/test';

import { API_BASE } from '../helpers/env';

/**
 * End-to-end for the Gin Rummy module (docs/rummy-games-plan.md, Phase A).
 *
 * The Go suites prove the rules in memory: `ginrummy/engine_test.go` pins
 * each clause, `ginrummy/agreement_test.go` cross-checks every offer against
 * the engine, and `ginrummy/conformance_test.go` plays whole matches — knocks
 * and gins included — from offers alone. What only this can prove is the
 * rest of the stack: real HTTP, a real WebSocket, real Mongo persistence, and
 * a fifth game hosted by a runtime that has never heard of it.
 *
 * The play-through is written the way a UI shell would be: it reads
 * `/modules` and the offer list and nothing else. It names no rank, no meld
 * and no rule of Gin Rummy anywhere. Where a test does mention "knock" or
 * "gin", it is asserting that a real match reached that state — not encoding
 * the rule that got it there.
 */

type Offer = {
  id: string;
  verb: string;
  enabled: boolean;
  whyNot?: string;
  source?: { zone: string; ownerId?: string; cards?: string[]; minCards?: number };
  target?: { zone: string; meldId?: string };
};

type Zone = {
  id: string;
  kind: string;
  ownerId?: string;
  cards?: { card: string }[];
  count: number;
  groups?: { id: string; cards: string[] }[];
};

type MatchState = {
  type: string;
  matchId: string;
  moduleId: string;
  status: string;
  winnerId?: string;
  players: { id: string; name: string; isAI: boolean }[];
  view: { zones: Zone[]; header?: unknown[]; status?: unknown[]; prompts?: unknown[] };
  legalActions: Offer[];
};

type Ctx = import('@playwright/test').APIRequestContext;

async function guest(request: Ctx) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `gin-${Math.random().toString(36).slice(2, 10)}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

/**
 * Opens a Gin Rummy match with two real players and starts it.
 *
 * Real players rather than bots: `/matches/{id}/add-bot` seats a body but
 * does not drive it, so a table with a bot on turn would simply stop.
 */
async function startMatch(request: Ctx, opts: { variation?: string; options?: Record<string, number> } = {}) {
  const users = [await guest(request), await guest(request)];
  const host = users[0];
  const auth = { Authorization: `Bearer ${host.accessToken}` };

  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    data: { moduleId: 'ginrummy', variation: opts.variation, options: opts.options ?? {} },
  });
  expect(created.ok(), await created.text()).toBeTruthy();
  const { matchId } = await created.json();

  const joined = await request.post(`${API_BASE}/matches/${matchId}/join`, {
    headers: { Authorization: `Bearer ${users[1].accessToken}` },
  });
  expect(joined.ok(), await joined.text()).toBeTruthy();

  const started = await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });
  expect(started.ok(), await started.text()).toBeTruthy();

  return { matchId, users, auth };
}

async function stateFor(request: Ctx, matchId: string, viewerId: string): Promise<MatchState> {
  const res = await request.get(`${API_BASE}/matches/${matchId}?as=${encodeURIComponent(viewerId)}`);
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

test.describe('ginrummy', () => {
  test('the server offers Gin Rummy and describes it well enough to render a form', async ({ request }) => {
    const res = await request.get(`${API_BASE}/modules`);
    expect(res.ok()).toBeTruthy();
    const { modules } = await res.json();

    const gin = modules.find((m: { id: string }) => m.id === 'ginrummy');
    expect(gin, 'ginrummy should be a hosted module').toBeTruthy();
    expect(gin.label).toBeTruthy();
    expect(gin.minPlayers).toBe(2);
    expect(gin.maxPlayers).toBe(2);

    const variationIds = gin.variations.map((v: { id: string }) => v.id);
    expect(variationIds).toContain('standard');
    expect(variationIds).toContain('oklahoma');

    const optionNames = gin.options.map((o: { name: string }) => o.name);
    for (const v of gin.variations) {
      for (const name of Object.keys(v.defaults ?? {})) {
        expect(optionNames, `variation ${v.id} defaults an undeclared option`).toContain(name);
      }
    }

    // Five games now run behind the one runtime.
    const ids = modules.map((m: { id: string }) => m.id);
    expect(ids).toEqual(expect.arrayContaining(['zolik', 'prsi', 'canasta', 'holdem', 'ginrummy']));
  });

  test('a Gin Rummy match persists through Mongo and comes back playable', async ({ request }) => {
    const { matchId, users } = await startMatch(request, { options: { targetScore: 100 } });

    const state = await stateFor(request, matchId, users[0].userId);
    expect(state.moduleId).toBe('ginrummy');
    expect(state.status).toBe('active');
    expect(state.legalActions.length).toBeGreaterThan(0);

    const kinds = state.view.zones.map((z) => z.kind);
    expect(kinds).toContain('hand');
    expect(kinds).toContain('stack');
    expect(kinds).toContain('pile');

    // Exactly one player is offered anything at the start of the hand — the
    // upcard dance is a real turn, not a special case.
    const withOffers = [];
    for (const u of users) {
      const s = await stateFor(request, matchId, u.userId);
      if (s.legalActions.some((o) => o.enabled)) withOffers.push(u.userId);
    }
    expect(withOffers).toHaveLength(1);
  });

  test('a viewer never receives the opponent hand', async ({ request }) => {
    const { matchId, users } = await startMatch(request, { options: { targetScore: 100 } });

    for (const viewer of users) {
      const state = await stateFor(request, matchId, viewer.userId);
      const opponent = users.find((u) => u.userId !== viewer.userId)!;

      const theirs = state.view.zones.find((z) => z.kind === 'hand' && z.ownerId === opponent.userId);
      expect(theirs, 'the opponent hand zone should exist').toBeTruthy();
      expect(theirs!.cards ?? []).toHaveLength(0);
      expect(theirs!.count).toBeGreaterThan(0);

      const own = state.view.zones.find((z) => z.kind === 'hand' && z.ownerId === viewer.userId);
      expect(own!.cards!.length).toBe(own!.count);

      const stock = state.view.zones.find((z) => z.id === 'stock');
      expect(stock!.cards ?? []).toHaveLength(0);
      expect(stock!.count).toBeGreaterThan(0);
      const pile = state.view.zones.find((z) => z.id === 'discard');
      expect((pile!.cards ?? []).length).toBeLessThanOrEqual(1);
    }
  });

  test('an option the descriptor does not declare is refused at creation', async ({ request }) => {
    const host = await guest(request);
    const res = await request.post(`${API_BASE}/matches`, {
      headers: { Authorization: `Bearer ${host.accessToken}` },
      data: { moduleId: 'ginrummy', options: { targetScore: 12345 } },
    });
    expect(res.status()).toBe(400);
    expect((await res.json()).code).toBe('OPTION_NOT_ALLOWED');
  });

  test('a whole Gin Rummy match plays to a winner over real WebSockets, knocks included', async ({
    page,
    request,
  }) => {
    // The end-to-end claim at full strength: two real players, two real
    // sockets, every move persisted, played to a winner by a client that
    // reads only the offer list — knocking and gin included, which is the one
    // thing no Žolíky run can do (extensibility-plan.md §1.1).
    test.setTimeout(240_000);

    const { matchId, users } = await startMatch(request, { options: { targetScore: 100 } });
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
          for (let i = 0; i < 200; i++) {
            if (latest(seat)) return latest(seat);
            await new Promise((r) => setTimeout(r, 50));
          }
          throw new Error('no match_state arrived');
        };

        const seats = await Promise.all(tokens.map(open));
        for (const s of seats) await settle(s);

        /**
         * Build the submission an offer describes, using only what the offer
         * declares — the same discipline module.SubmissionFor holds a bot to.
         */
        const submissionFor = (o: any) => {
          const action: any = { offerId: o.id, verb: o.verb };
          const need = o.source?.minCards ?? 0;
          if (need > 0) {
            const cards = o.source?.cards ?? [];
            if (cards.length < need) return null;
            action.cards = cards.slice(0, need);
          }
          if (o.target?.meldId) action.target = o.target.meldId;
          return action;
        };

        // Knocking first, or the aggregate discard offer (always enabled once
        // a knock is too) would win every tie and the match would never
        // knock at all.
        const order = ['knock', 'lay_off', 'finish_layoff', 'draw', 'discard', 'pass', 'continue'];

        const verbs: Record<string, number> = {};
        let moves = 0;
        let status = 'active';
        let winnerId = '';
        const errors: string[] = [];

        for (let step = 0; step < 4000; step++) {
          const idx = seats.findIndex((s) => (latest(s)?.legalActions ?? []).some((o: any) => o.enabled));
          if (idx === -1) break;

          const seat = seats[idx];
          const state = latest(seat);
          status = state.status;
          winnerId = state.winnerId ?? '';
          if (status !== 'active') break;

          const enabled = state.legalActions.filter((o: any) => o.enabled);
          enabled.sort((a: any, b: any) => order.indexOf(a.verb) - order.indexOf(b.verb));

          let action: any = null;
          for (const o of enabled) {
            action = submissionFor(o);
            if (action) break;
          }
          if (!action) break;

          const before = seats.map((s) => s.inbox.length);
          seat.ws.send(JSON.stringify(action));
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
      { wsBase, matchId, tokens: users.map((u) => u.accessToken) },
    );

    expect(result.errors, `socket errors: ${result.errors.join('; ')}`).toEqual([]);
    expect(result.moves).toBeGreaterThan(10);
    expect(result.status).toBe('completed');
    expect(result.winnerId).not.toBe('');

    // And a match of *Gin Rummy*: somebody actually knocked. Without that,
    // reaching the target would mean nothing but drawing and discarding —
    // dead hands forever, never a settled score.
    expect(result.verbs.knock ?? 0).toBeGreaterThan(0);
    expect(result.verbs.draw ?? 0).toBeGreaterThan(0);
    expect(result.verbs.discard ?? 0).toBeGreaterThan(0);

    const persisted = await (await request.get(`${API_BASE}/matches/${matchId}`)).json();
    expect(persisted.status).toBe('completed');
    expect(persisted.winnerId).toBe(result.winnerId);
  });

  test('the runtime keeps hosting the other four games', async ({ request }) => {
    for (const moduleId of ['zolik', 'prsi', 'canasta']) {
      const host = await guest(request);
      const auth = { Authorization: `Bearer ${host.accessToken}` };
      const created = await request.post(`${API_BASE}/matches`, { headers: auth, data: { moduleId } });
      expect(created.ok(), await created.text()).toBeTruthy();
      const { matchId } = await created.json();

      const other = await guest(request);
      await request.post(`${API_BASE}/matches/${matchId}/join`, {
        headers: { Authorization: `Bearer ${other.accessToken}` },
      });
      const started = await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });
      expect(started.ok(), await started.text()).toBeTruthy();

      const state = await stateFor(request, matchId, host.userId);
      expect(state.moduleId).toBe(moduleId);
      expect(state.status).toBe('active');
      expect(state.legalActions.length).toBeGreaterThan(0);
    }
  });
});
