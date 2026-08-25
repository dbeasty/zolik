import { expect, test } from '@playwright/test';

import { API_BASE } from '../helpers/env';

/**
 * End-to-end for the Canasta module (docs/canasta-plan.md).
 *
 * The Go suites prove the rules in memory: `canasta/engine_test.go` pins each
 * clause, `canasta/agreement_test.go` cross-checks every offer against the
 * engine, and `canasta/conformance_test.go` plays whole matches from offers
 * alone. What only this can prove is the rest of the stack — real HTTP, a real
 * WebSocket, real Mongo persistence, and a third game hosted by a runtime that
 * has never heard of it.
 *
 * The play-through is written the way a UI shell would be: it reads `/modules`
 * and the offer list and nothing else. It names no rank, no meld and no rule of
 * Canasta anywhere. Where a test does mention one, it is asserting that the
 * *runtime* stayed generic — not encoding a rule.
 */

type Offer = {
  id: string;
  verb: string;
  enabled: boolean;
  whyNot?: string;
  source?: { zone: string; ownerId?: string; cards?: string[]; minCards?: number; maxCards?: number };
  target?: { zone: string; meldId?: string };
};

type Zone = {
  id: string;
  kind: string;
  ownerId?: string;
  cards?: { card: string }[];
  count: number;
  groups?: { id: string; cards: string[]; badgeKeys?: string[] }[];
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
    data: { guestName: `can-${Math.random().toString(36).slice(2, 10)}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

/**
 * Opens a Canasta match with `seats` real players and starts it.
 *
 * Real players rather than bots: `/matches/{id}/add-bot` seats a body but does
 * not drive it (extensibility-plan.md §3.x), so a table with a bot on turn
 * would simply stop.
 */
async function startMatch(
  request: Ctx,
  seats: number,
  opts: { variation?: string; options?: Record<string, number> } = {},
) {
  const users = [];
  for (let i = 0; i < seats; i++) users.push(await guest(request));
  const host = users[0];
  const auth = { Authorization: `Bearer ${host.accessToken}` };

  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    data: { moduleId: 'canasta', variation: opts.variation, options: opts.options ?? {} },
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

  return { matchId, users, auth };
}

async function stateFor(request: Ctx, matchId: string, viewerId: string): Promise<MatchState> {
  const res = await request.get(`${API_BASE}/matches/${matchId}?as=${encodeURIComponent(viewerId)}`);
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

test.describe('canasta', () => {
  test('the server offers Canasta and describes it well enough to render a form', async ({ request }) => {
    const res = await request.get(`${API_BASE}/modules`);
    expect(res.ok()).toBeTruthy();
    const { modules } = await res.json();

    const canasta = modules.find((m: { id: string }) => m.id === 'canasta');
    expect(canasta, 'canasta should be a hosted module').toBeTruthy();
    expect(canasta.label).toBeTruthy();
    expect(canasta.minPlayers).toBe(2);
    expect(canasta.maxPlayers).toBe(4);

    // Two shipped rulesets, each declaring a starting value for every option —
    // so a lobby can show what it is about to create without asking the server
    // to resolve anything.
    const variationIds = canasta.variations.map((v: { id: string }) => v.id);
    expect(variationIds).toContain('classic');
    expect(variationIds).toContain('modern_american');

    const optionNames = canasta.options.map((o: { name: string }) => o.name);
    for (const v of canasta.variations) {
      for (const name of Object.keys(v.defaults ?? {})) {
        expect(optionNames, `variation ${v.id} defaults an undeclared option`).toContain(name);
      }
    }

    // Three games now run behind the one runtime.
    const ids = modules.map((m: { id: string }) => m.id);
    expect(ids).toEqual(expect.arrayContaining(['zolik', 'prsi', 'canasta']));
  });

  test('a Canasta match persists through Mongo and comes back playable', async ({ request }) => {
    const { matchId, users } = await startMatch(request, 2, { options: { targetScore: 500 } });

    // Read it back cold through a fresh HTTP request: nothing is cached in the
    // process, so this is the round trip through the database.
    const state = await stateFor(request, matchId, users[0].userId);
    expect(state.moduleId).toBe('canasta');
    expect(state.status).toBe('active');
    expect(state.legalActions.length).toBeGreaterThan(0);

    // The board arrived as generic zones. The runtime never learned what any of
    // them mean — a "spread" is a rummy table here and was a Prší nothing.
    const kinds = state.view.zones.map((z) => z.kind);
    expect(kinds).toContain('hand');
    expect(kinds).toContain('stack');
    expect(kinds).toContain('pile');
    expect(kinds).toContain('spread');

    // Exactly one player is offered anything — which is how the runtime works
    // out whose turn it is without a turn field to read.
    const withOffers = [];
    for (const u of users) {
      const s = await stateFor(request, matchId, u.userId);
      if (s.legalActions.some((o) => o.enabled)) withOffers.push(u.userId);
    }
    expect(withOffers).toHaveLength(1);
  });

  test('a viewer never receives another player cards', async ({ request }) => {
    // The one contract term with a security consequence, checked through the
    // real serialisation rather than in memory.
    const { matchId, users } = await startMatch(request, 4, { options: { targetScore: 500 } });

    for (const viewer of users) {
      const state = await stateFor(request, matchId, viewer.userId);
      const others = users.filter((u) => u.userId !== viewer.userId);

      for (const other of others) {
        const theirs = state.view.zones.find((z) => z.kind === 'hand' && z.ownerId === other.userId);
        expect(theirs, 'the opponent hand zone should exist').toBeTruthy();
        expect(theirs!.cards ?? []).toHaveLength(0);
        expect(theirs!.count).toBeGreaterThan(0);
      }

      const own = state.view.zones.find((z) => z.kind === 'hand' && z.ownerId === viewer.userId);
      expect(own!.cards!.length).toBe(own!.count);

      // The stock is face down and the pile shows only its top card, so a
      // client cannot reason about cards its player has not seen.
      const stock = state.view.zones.find((z) => z.id === 'draw');
      expect(stock!.cards ?? []).toHaveLength(0);
      expect(stock!.count).toBeGreaterThan(0);
      const pile = state.view.zones.find((z) => z.id === 'discard');
      expect((pile!.cards ?? []).length).toBeLessThanOrEqual(1);
    }
  });

  test('four seats are dealt into two partnerships', async ({ request }) => {
    // Partnerships are the fact that made Canasta a module rather than a
    // profile of the rummy engine, and they have to survive the wire: a
    // four-handed table shows two shared meld spreads, not four private ones.
    const { matchId, users } = await startMatch(request, 4, { options: { targetScore: 500 } });
    const state = await stateFor(request, matchId, users[0].userId);

    const spreads = state.view.zones.filter((z) => z.kind === 'spread' && z.id.startsWith('melds:'));
    expect(spreads, 'four players make two partnerships').toHaveLength(2);
    // A meld spread belongs to a partnership, so it names no owner — unlike
    // the four hand zones, which each do.
    for (const s of spreads) expect(s.ownerId ?? '').toBe('');
    expect(state.view.zones.filter((z) => z.kind === 'hand')).toHaveLength(4);
  });

  test('an option the descriptor does not declare is refused at creation', async ({ request }) => {
    const host = await guest(request);
    const res = await request.post(`${API_BASE}/matches`, {
      headers: { Authorization: `Bearer ${host.accessToken}` },
      data: { moduleId: 'canasta', options: { targetScore: 12345 } },
    });
    expect(res.status()).toBe(400);
    expect((await res.json()).code).toBe('OPTION_NOT_ALLOWED');
  });

  test('a variation the descriptor does not ship is refused too', async ({ request }) => {
    const host = await guest(request);
    const res = await request.post(`${API_BASE}/matches`, {
      headers: { Authorization: `Bearer ${host.accessToken}` },
      data: { moduleId: 'canasta', variation: 'bolivia' },
    });
    // 404 rather than 400: the runtime treats a named ruleset that does not
    // exist the same way it treats a module that does not exist — a missing
    // resource, not a malformed request. See match/handlers.go's
    // writeModuleError. Bolivia is a real canasta variant this module does not
    // ship, which is exactly the mistake a client would actually make.
    expect(res.status()).toBe(404);
    expect((await res.json()).code).toBe('UNKNOWN_VARIATION');
  });

  test('a whole Canasta match plays to a winner over real WebSockets', async ({ page, request }) => {
    // The end-to-end claim at full strength: two real players, two real
    // sockets, every move persisted, played to a winner by a client that reads
    // only the offer list.
    //
    // This is also the claim Žolíky cannot make. Going out there needs a meld
    // *shape* the offer protocol deliberately does not enumerate
    // (extensibility-plan.md §1.1), so its offer-only driver takes real turns
    // but never finishes. A Canasta meld is n of one rank, so the offers carry
    // concrete cards and a shell with no rules in it can play the game out.
    test.setTimeout(240_000);

    const { matchId, users } = await startMatch(request, 2, { options: { targetScore: 500 } });
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
         * declares. This is the whole discipline: `minCards` cards from the
         * front of the list the offer says it will accept, plus the meld it
         * says to aim at. Nothing here knows what a canasta is.
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

        // A preference order is a UI choice, not a rule: build the table, take
        // the pile when it is offered, and discard only because a turn has to
        // end somewhere.
        const order = ['lay_meld', 'lay_off', 'take_pile', 'draw', 'discard'];

        const verbs: Record<string, number> = {};
        let moves = 0;
        let status = 'active';
        let winnerId = '';
        let deals = 0;
        let sawCanastaBadge = false;
        let sawRedThrees = false;
        const errors: string[] = [];

        for (let step = 0; step < 1500; step++) {
          const idx = seats.findIndex((s) => (latest(s)?.legalActions ?? []).some((o: any) => o.enabled));
          if (idx === -1) break;

          const seat = seats[idx];
          const state = latest(seat);
          status = state.status;
          winnerId = state.winnerId ?? '';
          if (status !== 'active') break;

          // Observations a UI would make, gathered as we go rather than
          // re-derived: badges and the red-three zone are pushed facts.
          for (const z of state.view?.zones ?? []) {
            if (z.id?.startsWith('redThrees:')) sawRedThrees = true;
            for (const g of z.groups ?? []) {
              if ((g.badgeKeys ?? []).some((b: string) => b.includes('Canasta'))) sawCanastaBadge = true;
            }
          }
          const dealFact = (state.view?.header ?? []).find((f: any) => f.labelKey === 'header.deal');
          if (dealFact) deals = Math.max(deals, Number(dealFact.value) || 0);

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

          // Wait for the broadcast to reach every seat, so the next iteration
          // reads fresh state rather than the state it just acted on.
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
          deals,
          sawCanastaBadge,
          sawRedThrees,
          status: final.status ?? status,
          winnerId: final.winnerId ?? winnerId,
        };
      },
      { wsBase, matchId, tokens: users.map((u) => u.accessToken) },
    );

    // Nothing the offers advertised was refused. An offer the engine then
    // rejects is a control that does nothing, and no client-side check would
    // catch it.
    expect(result.errors, `socket errors: ${result.errors.join('; ')}`).toEqual([]);

    // A whole match, not a token move.
    expect(result.moves).toBeGreaterThan(20);
    expect(result.status).toBe('completed');
    expect(result.winnerId).not.toBe('');

    // And a match of *Canasta*: cards were melded and extended, not just drawn
    // and thrown away. Without melds nobody can go out, so a run that reached a
    // winner on draws and discards alone would mean the target was reached on
    // hand penalties — a shuffling exercise, not this game.
    expect(result.verbs.lay_meld ?? 0).toBeGreaterThan(0);
    expect(result.verbs.discard ?? 0).toBeGreaterThan(0);
    expect(result.verbs.draw ?? 0).toBeGreaterThan(0);

    // The database agrees with what the sockets reported.
    const persisted = await (await request.get(`${API_BASE}/matches/${matchId}`)).json();
    expect(persisted.status).toBe('completed');
    expect(persisted.winnerId).toBe(result.winnerId);
  });

  test('the runtime keeps hosting the other two games', async ({ request }) => {
    // A third module is only interesting if it cost the first two nothing. The
    // rummy engine and Prší, unchanged, still run behind the same envelope and
    // the same socket.
    for (const moduleId of ['zolik', 'prsi']) {
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
