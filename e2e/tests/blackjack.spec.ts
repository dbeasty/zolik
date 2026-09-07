import { expect, test } from '@playwright/test';

import { API_BASE } from '../helpers/env';

/**
 * End-to-end for the Blackjack module.
 *
 * The Go suites prove the rules in memory: `blackjack/engine_test.go` pins
 * each clause, `blackjack/agreement_test.go` cross-checks every offer against
 * the engine, and `blackjack/conformance_test.go` plays whole matches from the
 * offers alone. What only this can prove is the rest of the stack — real HTTP,
 * real WebSockets, real Mongo persistence — and the two things about this game
 * that are new to the runtime rather than to the rules:
 *
 *  1. **Two phases wait on everybody at once.** Betting and insurance are
 *     simultaneous at a real table, and this is the first module that models
 *     them that way. A runtime that assumed one seat on turn would deadlock
 *     here, and only a real socket fan-out shows it does not.
 *  2. **The opponent is not a seat.** The dealer is a zone nobody owns, played
 *     by a rule, and the hole card is the only hidden thing in the game.
 *
 * The play-through is written the way a UI shell would be: it reads the offer
 * list and nothing else. It names no rank, no total and no rule of blackjack.
 * Where it mentions "bet" or "stand", it is asserting that a real match
 * reached that state — not encoding the rule that got it there.
 */

type Offer = {
  id: string;
  verb: string;
  enabled: boolean;
  whyNot?: string;
  params?: { name: string; kind?: string; min?: number; max?: number; default?: number; choices?: { value: string }[] }[];
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
  winners?: string[];
  players: { id: string; name: string; isAI: boolean }[];
  view: { zones: Zone[]; seats?: { playerId: string; active?: boolean }[] };
  legalActions: Offer[];
  rounds?: { rounds: { number: number; scores: { playerId: string; delta: number; total: number }[] }[] };
};

type Ctx = import('@playwright/test').APIRequestContext;

async function guest(request: Ctx) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `bj-${Math.random().toString(36).slice(2, 10)}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

/**
 * Opens a Blackjack match with two real players and starts it.
 *
 * Real players rather than bots, as every module spec here does:
 * `/matches/{id}/add-bot` seats a body, and this suite wants to drive every
 * decision itself.
 */
async function startMatch(
  request: Ctx,
  opts: { variation?: string; options?: Record<string, number> } = {},
) {
  const users = [await guest(request), await guest(request)];
  const host = users[0];
  const auth = { Authorization: `Bearer ${host.accessToken}` };

  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    data: { moduleId: 'blackjack', variation: opts.variation, options: opts.options ?? {} },
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

test.describe('blackjack', () => {
  test('the server offers Blackjack and describes it well enough to render a form', async ({ request }) => {
    const res = await request.get(`${API_BASE}/modules`);
    expect(res.ok()).toBeTruthy();
    const { modules } = await res.json();

    const bj = modules.find((m: { id: string }) => m.id === 'blackjack');
    expect(bj, 'blackjack should be a hosted module').toBeTruthy();
    expect(bj.label).toBeTruthy();
    expect(bj.minPlayers).toBe(2);
    expect(bj.maxPlayers).toBe(7);

    const variationIds = bj.variations.map((v: { id: string }) => v.id);
    expect(variationIds).toEqual(expect.arrayContaining(['vegas', 'atlantic', 'single']));

    // Every house rule this game has is a declared option, so the lobby can
    // render a table's whole rule set without knowing what any of it means.
    const optionNames = bj.options.map((o: { name: string }) => o.name);
    expect(optionNames).toEqual(
      expect.arrayContaining(['decks', 'minBet', 'dealerHitsSoft17', 'blackjackPays', 'maxSplits']),
    );
    for (const v of bj.variations) {
      for (const name of Object.keys(v.defaults ?? {})) {
        expect(optionNames, `variation ${v.id} defaults an undeclared option`).toContain(name);
      }
    }

    const ids = modules.map((m: { id: string }) => m.id);
    expect(ids).toEqual(
      expect.arrayContaining(['zolik', 'prsi', 'canasta', 'holdem', 'ginrummy', 'rummytiles', 'blackjack']),
    );
  });

  test('the table opens by asking every seat for a stake at once', async ({ request }) => {
    const { matchId, users } = await startMatch(request, { options: { rounds: 5 } });

    // The property that is new to the runtime: more than one seat awaited, and
    // the board saying so rather than a client working it out.
    for (const u of users) {
      const state = await stateFor(request, matchId, u.userId);
      expect(state.moduleId).toBe('blackjack');
      expect(state.status).toBe('active');
      expect(state.legalActions.some((o) => o.enabled)).toBeTruthy();

      const active = (state.view.seats ?? []).filter((s) => s.active).map((s) => s.playerId);
      expect(active.sort()).toEqual(users.map((x) => x.userId).sort());
    }

    // And the stake is a declared range, not a list of amounts.
    const state = await stateFor(request, matchId, users[0].userId);
    const bet = state.legalActions.find((o) => o.enabled && (o.params ?? []).length > 0);
    expect(bet, 'the opening offer should carry a numeric parameter').toBeTruthy();
    const spec = bet!.params![0];
    expect(spec.kind).toBe('int');
    expect(spec.min).toBeGreaterThan(0);
    expect(spec.max).toBeGreaterThanOrEqual(spec.min!);
    expect(spec.default).toBeGreaterThanOrEqual(spec.min!);
  });

  test('the hole card never reaches a viewer, and nothing else is hidden', async ({ request, page }) => {
    const { matchId, users } = await startMatch(request, { options: { rounds: 5, insurance: 0 } });
    const wsBase = API_BASE.replace(/^http/, 'ws');

    // Stake both seats so there is a deal to look at.
    await placeBets(page, wsBase, matchId, users.map((u) => u.accessToken));

    for (const viewer of users) {
      const state = await stateFor(request, matchId, viewer.userId);

      const dealer = state.view.zones.find((z) => z.id === 'dealer')!;
      expect(dealer, 'the dealer should be a zone nobody owns').toBeTruthy();
      expect(dealer.ownerId).toBeFalsy();
      // Two cards dealt, one of them shown: the count says there is another,
      // and the card itself is simply not sent.
      expect(dealer.count).toBe(2);
      expect(dealer.cards ?? []).toHaveLength(1);

      // Player cards are face up in this game, so every box shows its own.
      for (const u of users) {
        const box = state.view.zones.find((z) => z.id === `box:${u.userId}`)!;
        expect(box, `box for ${u.userId}`).toBeTruthy();
        const shown = (box.groups ?? []).reduce((n, g) => n + g.cards.length, 0);
        expect(shown).toBe(box.count);
        expect(shown).toBeGreaterThan(0);
      }

      // The shoe is a count and nothing else.
      const shoe = state.view.zones.find((z) => z.id === 'shoe')!;
      expect(shoe.cards ?? []).toHaveLength(0);
      expect(shoe.count).toBeGreaterThan(0);
    }
  });

  test('an option the descriptor does not declare is refused at creation', async ({ request }) => {
    const host = await guest(request);
    const res = await request.post(`${API_BASE}/matches`, {
      headers: { Authorization: `Bearer ${host.accessToken}` },
      data: { moduleId: 'blackjack', options: { decks: 3 } },
    });
    expect(res.status()).toBe(400);
    expect((await res.json()).code).toBe('OPTION_NOT_ALLOWED');
  });

  test('a whole match plays to a winner over real WebSockets, chips and all', async ({ page, request }) => {
    test.setTimeout(240_000);

    const { matchId, users } = await startMatch(request, {
      variation: 'atlantic',
      options: { rounds: 5, startingStack: 200, minBet: 5 },
    });
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
         * Build the submission an offer describes, from what the offer itself
         * declares — the same discipline module.SubmissionFor holds a bot to.
         * The one input this game has is a number, and its value comes from
         * the offer's own range.
         */
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

    // A match of blackjack: stakes were placed, hands were played out, and the
    // table stopped between rounds for everybody to see the settlement.
    expect(result.verbs.bet ?? 0).toBeGreaterThan(0);
    expect(result.verbs.stand ?? 0).toBeGreaterThan(0);
    expect(result.verbs.continue ?? 0).toBeGreaterThan(0);

    // The round table survived the trip through Mongo, and its arithmetic
    // still holds: every seat's running total is its deltas added up.
    const persisted = await stateFor(request, matchId, users[0].userId);
    expect(persisted.status).toBe('completed');
    const rounds = persisted.rounds?.rounds ?? [];
    expect(rounds.length).toBeGreaterThan(0);
    const running: Record<string, number | undefined> = {};
    for (const round of rounds) {
      for (const score of round.scores) {
        if (running[score.playerId] === undefined) {
          running[score.playerId] = score.total;
          continue;
        }
        expect(score.total).toBe(running[score.playerId]! + score.delta);
        running[score.playerId] = score.total;
      }
    }
  });
});

/**
 * Puts a stake up for every seat, over real sockets, so a test that wants to
 * look at a dealt table can get to one without knowing how betting works
 * beyond "the offer says how much".
 */
async function placeBets(
  page: import('@playwright/test').Page,
  wsBase: string,
  matchId: string,
  tokens: string[],
) {
  await page.evaluate(
    async ({ wsBase, matchId, tokens }) => {
      const sockets = await Promise.all(
        tokens.map(async (token) => {
          const ws = new WebSocket(`${wsBase}/ws/matches/${matchId}?token=${encodeURIComponent(token)}`);
          const inbox: any[] = [];
          await new Promise<void>((resolve, reject) => {
            ws.onopen = () => resolve();
            ws.onerror = () => reject(new Error('socket failed to open'));
            setTimeout(() => reject(new Error('socket open timed out')), 10000);
          });
          ws.onmessage = (ev) => inbox.push(JSON.parse(String(ev.data)));
          return { ws, inbox };
        }),
      );

      const latest = (s: { inbox: any[] }) => {
        for (let i = s.inbox.length - 1; i >= 0; i--) {
          if (s.inbox[i].type === 'match_state') return s.inbox[i];
        }
        return null;
      };

      for (const s of sockets) {
        for (let i = 0; i < 200 && !latest(s); i++) await new Promise((r) => setTimeout(r, 50));
        const offer = (latest(s)?.legalActions ?? []).find((o: any) => o.enabled && (o.params ?? []).length > 0);
        if (!offer) continue;
        const p = offer.params[0];
        s.ws.send(JSON.stringify({ offerId: offer.id, verb: offer.verb, params: { [p.name]: String(p.default ?? p.min) } }));
      }

      // Let the deal land on every socket before anybody looks at the board.
      await new Promise((r) => setTimeout(r, 1500));
      for (const s of sockets) s.ws.close();
    },
    { wsBase, matchId, tokens },
  );
}
