import { expect, test } from '@playwright/test';

import { API_BASE, asViewer, type Viewer } from '../helpers/env';

/**
 * End-to-end for the Siete y Medio module.
 *
 * The Go suites prove the rules in memory: `sietemedio/engine_test.go` pins
 * each clause, `agreement_test.go` cross-checks every offer against the
 * engine, and `conformance_test.go` plays whole matches from the offers alone.
 * What only this can prove is the rest of the stack — real HTTP, real
 * WebSockets, real persistence — and the three things about this game that
 * are the runtime's problem rather than the rules':
 *
 *  1. **The bank is a seat, and it moves.** Every chip a player wins comes out
 *     of another player's stack, so the whole table's chips are a constant —
 *     a sharper invariant than a game against a house can state, and one that
 *     only survives a real round trip if the settlement does.
 *  2. **A stake is bounded by somebody else's chips.** *La banca responde*:
 *     the top of the stake range is what the banker can still cover, which is
 *     a number no client could work out and so has to arrive on the wire.
 *  3. **The hidden information runs the ordinary way round.** Each seat's
 *     first card is that seat's own, and the banker's is hidden from the whole
 *     table until their turn. That is per-viewer filtering of a kind blackjack
 *     never exercised, where the one secret was kept from everybody at once.
 *
 * The play-through is written the way a UI shell would be: it reads the offer
 * list and nothing else. It names no rank, no total and no rule of siete y
 * medio. Where it mentions "stake" or "stand", it is asserting that a real
 * match reached that state — not encoding the rule that got it there.
 */

type Offer = {
  id: string;
  verb: string;
  enabled: boolean;
  manual?: boolean;
  whyNot?: string;
  params?: {
    name: string;
    kind?: string;
    min?: number;
    max?: number;
    default?: number;
    choices?: { value: string }[];
  }[];
};

type Zone = {
  id: string;
  kind: string;
  ownerId?: string;
  cards?: { card: string }[];
  count: number;
};

type Fact = { labelKey: string; value?: string; params?: Record<string, unknown> };

type MatchState = {
  type: string;
  matchId: string;
  moduleId: string;
  status: string;
  deck?: string;
  winnerId?: string;
  winners?: string[];
  players: { id: string; name: string; isAI: boolean }[];
  view: {
    zones: Zone[];
    seats?: { playerId: string; active?: boolean; labelKeys?: string[]; facts?: Fact[] }[];
    header?: Fact[];
  };
  legalActions: Offer[];
  rounds?: {
    rounds: { number: number; scores: { playerId: string; delta: number; total: number }[] }[];
  };
};

type Ctx = import('@playwright/test').APIRequestContext;

const STARTING_STACK = 200;
const MIN_STAKE = 5;

async function guest(request: Ctx) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `sm-${Math.random().toString(36).slice(2, 10)}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

/**
 * Opens a Siete y Medio match with three real players and starts it.
 *
 * Three rather than two, which is the minimum: with three seats one banks and
 * two stake, so "every seat except the banker is waited on at once" is a
 * property with something to say. Real players rather than bots, as every
 * module spec here does — this suite wants to drive every decision itself.
 */
async function startMatch(
  request: Ctx,
  opts: { variation?: string; options?: Record<string, number> } = {},
) {
  const users = [await guest(request), await guest(request), await guest(request)];
  const host = users[0];
  const auth = { Authorization: `Bearer ${host.accessToken}` };

  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    data: { moduleId: 'sietemedio', variation: opts.variation, options: opts.options ?? {} },
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

async function stateFor(request: Ctx, matchId: string, viewer: Viewer): Promise<MatchState> {
  const res = await request.get(`${API_BASE}/matches/${matchId}`, asViewer(viewer));
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

/** Whoever the board says is holding the bank this round. */
function bankerOf(state: MatchState): string {
  const seat = (state.view.seats ?? []).find((s) => (s.labelKeys ?? []).includes('sietemedio.seat.bank'));
  expect(seat, 'the board should say who holds the bank').toBeTruthy();
  return seat!.playerId;
}

test.describe('siete y medio', () => {
  test('the server offers Siete y Medio and describes it well enough to render a form', async ({
    request,
  }) => {
    const res = await request.get(`${API_BASE}/modules`);
    expect(res.ok()).toBeTruthy();
    const { modules } = await res.json();

    const sm = modules.find((m: { id: string }) => m.id === 'sietemedio');
    expect(sm, 'sietemedio should be a hosted module').toBeTruthy();
    expect(sm.label).toBeTruthy();
    expect(sm.minPlayers).toBe(2);
    expect(sm.maxPlayers).toBe(8);
    // The pack reaches a client as a name, so it can draw oros and copas
    // without being told that this game exists.
    expect(sm.deck).toBe('spanish');

    const variationIds = sm.variations.map((v: { id: string }) => v.id);
    expect(variationIds).toEqual(expect.arrayContaining(['mexicana', 'cantina', 'abierta']));

    // The three things the sources disagree about are declared options, so a
    // lobby can render a table's whole rule set without knowing what any of
    // it means.
    const optionNames = sm.options.map((o: { name: string }) => o.name);
    expect(optionNames).toEqual(
      expect.arrayContaining(['ties', 'sevenAndAHalfPays', 'bank', 'firstCardDown', 'minStake']),
    );
    for (const v of sm.variations) {
      for (const name of Object.keys(v.defaults ?? {})) {
        expect(optionNames, `variation ${v.id} defaults an undeclared option`).toContain(name);
      }
    }
  });

  test('the table opens by asking every seat but the banker for a stake at once', async ({
    request,
  }) => {
    const { matchId, users } = await startMatch(request, {
      options: { rounds: 6, startingStack: STARTING_STACK, minStake: MIN_STAKE },
    });

    const opening = await stateFor(request, matchId, users[0]);
    expect(opening.moduleId).toBe('sietemedio');
    expect(opening.status).toBe('active');
    expect(opening.deck).toBe('spanish');
    const banker = bankerOf(opening);

    // The property that is this game's own: several seats awaited at once, and
    // the one holding the bank waiting rather than staking.
    for (const u of users) {
      const state = await stateFor(request, matchId, u);
      const active = (state.view.seats ?? []).filter((s) => s.active).map((s) => s.playerId);
      expect(active.sort()).toEqual(
        users
          .map((x) => x.userId)
          .filter((id) => id !== banker)
          .sort(),
      );
      // Every board agrees who is banking, whoever is looking at it.
      expect(bankerOf(state)).toBe(banker);
      // And the header names them, so a client need not scan the seats.
      const header = (state.view.header ?? []).find((f) => f.labelKey === 'sietemedio.header.bank');
      expect(header?.params?.playerId).toBe(banker);
    }

    // The banker is offered nothing but the manual way out of the table.
    const bankerSeat = users.find((u) => u.userId === banker)!;
    const bankerState = await stateFor(request, matchId, bankerSeat);
    const live = bankerState.legalActions.filter((o) => o.enabled && !o.manual);
    expect(live, 'the banker covers the stakes rather than making one').toEqual([]);
  });

  test('the stake a seat is offered is bounded by what the bank can cover', async ({
    page,
    request,
  }) => {
    const { matchId, users } = await startMatch(request, {
      // Every seat holds the same stack, so a range topped out below it can
      // only be the bank's limit rather than the seat's own chips.
      options: { rounds: 6, startingStack: STARTING_STACK, minStake: MIN_STAKE },
    });
    const wsBase = API_BASE.replace(/^http/, 'ws');

    const opening = await stateFor(request, matchId, users[0]);
    const banker = bankerOf(opening);
    const punters = users.filter((u) => u.userId !== banker);

    const state = await stateFor(request, matchId, punters[0]);
    const stake = state.legalActions.find((o) => o.enabled && (o.params ?? []).length > 0);
    expect(stake, 'the opening offer should carry a numeric parameter').toBeTruthy();
    const spec = stake!.params![0];
    expect(spec.kind).toBe('int');
    expect(spec.min).toBe(MIN_STAKE);
    expect(spec.default).toBeGreaterThanOrEqual(spec.min!);
    expect(spec.max).toBeGreaterThanOrEqual(spec.min!);
    // La banca responde. At the standard table a 7½ collects double, so the
    // bank's whole stack answers half as much again in stakes — which is a
    // number about *another seat's* chips, and the reason this range is
    // pushed rather than left to a client to work out.
    expect(
      spec.max,
      'the stake should be capped below the seat’s own chips by what the bank can answer for',
    ).toBeLessThan(STARTING_STACK);

    // The engine agrees with its own offer at both ends of the range it
    // declared, and refuses the one chip past it by naming the bank rather
    // than the seat's own stack.
    //
    // The over-sized stake goes first, while the bank's cover is untouched:
    // once a seat has taken the whole of it, the seats behind are no longer
    // asked at all, which is a different refusal about a different thing.
    const result = await submit(page, wsBase, matchId, [
      {
        token: punters[1].accessToken,
        action: {
          offerId: stake!.id,
          verb: stake!.verb,
          params: { [spec.name]: String(STARTING_STACK) },
        },
      },
      {
        token: punters[0].accessToken,
        action: { offerId: stake!.id, verb: stake!.verb, params: { [spec.name]: String(spec.max) } },
      },
    ]);

    expect(result[0], 'a stake past the bank should be refused, and say so').toBe(
      'BANK_CANNOT_COVER',
    );
    expect(result[1], 'the top of the declared range should be accepted').toBe('');
  });

  test('each seat sees its own first card and nobody else’s', async ({ request, page }) => {
    const { matchId, users } = await startMatch(request, {
      options: { rounds: 6, startingStack: STARTING_STACK, minStake: MIN_STAKE },
    });
    const wsBase = API_BASE.replace(/^http/, 'ws');

    const opening = await stateFor(request, matchId, users[0]);
    const banker = bankerOf(opening);

    // Stake every seat that can, so there is a deal to look at.
    await placeStakes(page, wsBase, matchId, users.map((u) => u.accessToken));

    for (const viewer of users) {
      const state = await stateFor(request, matchId, viewer);

      for (const u of users) {
        const hand = state.view.zones.find((z) => z.id === `hand:${u.userId}`)!;
        expect(hand, `hand for ${u.userId}`).toBeTruthy();
        expect(hand.ownerId).toBe(u.userId);
        const shown = (hand.cards ?? []).length;

        if (u.userId === viewer.userId) {
          // Your own hand is yours to see, all of it.
          expect(shown, 'a seat should see its own hand').toBe(hand.count);
          expect(hand.count).toBeGreaterThan(0);
        } else {
          // Everybody else's opening card is theirs. The count still says it
          // is there, which is how this protocol hides a card: a client draws
          // the difference face down and is told nothing.
          expect(shown, `${viewer.userId} should not see ${u.userId}'s first card`).toBe(
            hand.count - 1,
          );
        }
      }

      // The banker's hand is hidden from the whole table — the banker apart —
      // until their turn, which is the one piece of ceremony in this game.
      if (viewer.userId !== banker) {
        const bank = state.view.zones.find((z) => z.id === `hand:${banker}`)!;
        expect(bank.cards ?? []).toHaveLength(0);
        expect(bank.count).toBeGreaterThan(0);
      }

      // A seat whose hand is part-hidden is given the total of what is on
      // show, under a key that says so — sending the real one would hand back
      // the card the zone just took out.
      const others = (state.view.seats ?? []).filter((s) => s.playerId !== viewer.userId);
      for (const s of others) {
        const keys = (s.facts ?? []).map((f) => f.labelKey);
        expect(keys, `${s.playerId}'s total should not be spelled out`).not.toContain(
          'sietemedio.seat.total',
        );
      }

      // The pack is a count and nothing else.
      const pack = state.view.zones.find((z) => z.id === 'pack')!;
      expect(pack.cards ?? []).toHaveLength(0);
      expect(pack.count).toBeGreaterThan(0);
    }
  });

  test('an option the descriptor does not declare is refused at creation', async ({ request }) => {
    const host = await guest(request);
    const res = await request.post(`${API_BASE}/matches`, {
      headers: { Authorization: `Bearer ${host.accessToken}` },
      data: { moduleId: 'sietemedio', options: { minStake: 7 } },
    });
    expect(res.status()).toBe(400);
    expect((await res.json()).code).toBe('OPTION_NOT_ALLOWED');
  });

  test('a whole match plays to a winner over real WebSockets, chips and all', async ({
    page,
    request,
  }) => {
    test.setTimeout(240_000);

    const { matchId, users } = await startMatch(request, {
      // The Cantina table, where the bank goes round: over six rounds every
      // seat banks twice, so the settlement is exercised from both sides of
      // it rather than from whichever seat won the cut.
      variation: 'cantina',
      options: { rounds: 6, startingStack: STARTING_STACK, minStake: MIN_STAKE },
    });
    const wsBase = API_BASE.replace(/^http/, 'ws');

    const result = await page.evaluate(
      async ({ wsBase, matchId, tokens }) => {
        type Seat = { ws: WebSocket; inbox: any[] };

        const open = async (token: string): Promise<Seat> => {
          const ws = new WebSocket(
            `${wsBase}/ws/matches/${matchId}?token=${encodeURIComponent(token)}`,
          );
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

        // Standing is preferred over asking for a card so a driver with no
        // sense of the arithmetic does not simply draw itself past 7½ every
        // round and finish the match with nothing settled either way.
        const order = ['stake', 'stand', 'hit', 'continue'];

        const verbs: Record<string, number> = {};
        const bankers = new Set<string>();
        let moves = 0;
        let status = 'active';
        let winnerId = '';
        const errors: string[] = [];

        for (let step = 0; step < 4000; step++) {
          // A manual offer — getting up from the table — is open all game and
          // is nobody's turn; a driver never makes it (module.ActionOffer.Manual).
          const live = (o: any) => o.enabled && !o.manual;
          const idx = seats.findIndex((s) => (latest(s)?.legalActions ?? []).some(live));
          if (idx === -1) break;

          const seat = seats[idx];
          const state = latest(seat);
          status = state.status;
          winnerId = state.winnerId ?? '';
          if (status !== 'active') break;

          for (const s of state.view?.seats ?? []) {
            if ((s.labelKeys ?? []).includes('sietemedio.seat.bank')) bankers.add(s.playerId);
          }

          const enabled = state.legalActions.filter(live);
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
          bankers: [...bankers],
          status: final.status ?? status,
          winnerId: final.winnerId ?? winnerId,
        };
      },
      { wsBase, matchId, tokens: users.map((u) => u.accessToken) },
    );

    expect(result.errors, `socket errors: ${result.errors.join('; ')}`).toEqual([]);
    expect(result.moves).toBeGreaterThan(10);
    expect(result.status).toBe('completed');

    // A match of siete y medio: stakes were put up, hands were played out, and
    // the table stopped between rounds for everybody to see the settlement.
    expect(result.verbs.stake ?? 0).toBeGreaterThan(0);
    expect(result.verbs.stand ?? 0).toBeGreaterThan(0);
    expect(result.verbs.continue ?? 0).toBeGreaterThan(0);

    // At this table the deal goes round, so the bank was held by more than the
    // seat that won the cut — the one thing about this game that no other
    // module here would show.
    expect(result.bankers.length, `the bank never moved: ${result.bankers.join(', ')}`).toBeGreaterThan(1);

    const persisted = await stateFor(request, matchId, users[0]);
    expect(persisted.status).toBe('completed');

    // The round table survived the trip through the store, and its arithmetic
    // still holds: every seat's running total is its deltas added up.
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
      // There is no house here: a round is a transfer between seats, so the
      // whole table's deltas have to cancel. The invariant a game against a
      // dealer cannot state, checked where it is hardest to hold — after a
      // round trip through the store.
      const sum = round.scores.reduce((n, s) => n + s.delta, 0);
      expect(sum, `round ${round.number} moved ${sum} chips into or out of the table`).toBe(0);
    }

    // And the whole table still holds exactly what it sat down with.
    const totals = Object.values(running).reduce((n, v) => n! + (v ?? 0), 0);
    expect(totals).toBe(users.length * STARTING_STACK);
  });
});

/**
 * Puts a stake up for every seat that is asked for one, over real sockets, so
 * a test that wants to look at a dealt table can get to one without knowing
 * how staking works beyond "the offer says how much".
 */
async function placeStakes(
  page: import('@playwright/test').Page,
  wsBase: string,
  matchId: string,
  tokens: string[],
) {
  await page.evaluate(
    async ({ wsBase, matchId, tokens }) => {
      const sockets = await Promise.all(
        tokens.map(async (token) => {
          const ws = new WebSocket(
            `${wsBase}/ws/matches/${matchId}?token=${encodeURIComponent(token)}`,
          );
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
        // The banker is offered no stake at all, which is not a failure here:
        // it is the seat the others are staking against.
        const offer = (latest(s)?.legalActions ?? []).find(
          (o: any) => o.enabled && (o.params ?? []).length > 0,
        );
        if (!offer) continue;
        const p = offer.params[0];
        s.ws.send(
          JSON.stringify({
            offerId: offer.id,
            verb: offer.verb,
            params: { [p.name]: String(p.default ?? p.min) },
          }),
        );
      }

      // Let the deal land on every socket before anybody looks at the board.
      await new Promise((r) => setTimeout(r, 1500));
      for (const s of sockets) s.ws.close();
    },
    { wsBase, matchId, tokens },
  );
}

/**
 * Sends one action per seat over a real socket, in order, and reports the
 * error code each came back with — the empty string for one that was
 * accepted.
 *
 * Actions only travel over the socket on this server, so a test that wants to
 * watch a particular refusal has to open one. In order rather than at once,
 * because what the bank can still cover depends on what has already been put
 * up, and a race between two stakes would make the refusal under test
 * intermittent.
 */
async function submit(
  page: import('@playwright/test').Page,
  wsBase: string,
  matchId: string,
  steps: { token: string; action: Record<string, unknown> }[],
): Promise<string[]> {
  return page.evaluate(
    async ({ wsBase, matchId, steps }) => {
      const codes: string[] = [];
      for (const step of steps) {
        const ws = new WebSocket(
          `${wsBase}/ws/matches/${matchId}?token=${encodeURIComponent(step.token)}`,
        );
        const inbox: any[] = [];
        await new Promise<void>((resolve, reject) => {
          ws.onopen = () => resolve();
          ws.onerror = () => reject(new Error('socket failed to open'));
          setTimeout(() => reject(new Error('socket open timed out')), 10000);
        });
        ws.onmessage = (ev) => inbox.push(JSON.parse(String(ev.data)));

        // Wait for the board before acting on it, the way a client does.
        for (let i = 0; i < 200; i++) {
          if (inbox.some((m) => m.type === 'match_state')) break;
          await new Promise((r) => setTimeout(r, 50));
        }
        const before = inbox.length;
        ws.send(JSON.stringify(step.action));
        for (let i = 0; i < 200; i++) {
          if (inbox.length > before) break;
          await new Promise((r) => setTimeout(r, 20));
        }
        const error = inbox.slice(before).find((m) => m.type === 'error');
        codes.push(error ? String(error.code) : '');
        ws.close();
      }
      return codes;
    },
    { wsBase, matchId, steps },
  );
}
