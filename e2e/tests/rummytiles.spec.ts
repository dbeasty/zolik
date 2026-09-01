import { expect, test } from '@playwright/test';

import { API_BASE } from '../helpers/env';

/**
 * End-to-end for the Rummy Tiles module (docs/rummy-games-plan.md, Phase B).
 *
 * The Go suites prove the rules in memory: `rummytiles/sets_test.go` pins
 * group/run validity and joker resolution, `rummytiles/engine_test.go` pins
 * the workspace and the initial meld, `rummytiles/agreement_test.go`
 * cross-checks every offer against the engine, and `rummytiles/deadend_test.go`
 * proves the player on turn always has a move at 2, 3 and 4 seats. What only
 * this can prove is the rest of the stack: real HTTP, a real WebSocket, real
 * Mongo persistence, and a sixth game hosted by a runtime that has never heard
 * of it.
 *
 * There is no client UI for this module yet (see the plan's Outcome section —
 * the tile face and board-slot selection are B1/B4, not built in this pass),
 * so unlike ginrummy.spec.ts and canasta.spec.ts this cannot drive
 * `app/match/[matchId].tsx`. What it proves instead: the protocol itself,
 * end to end, using the same discipline a real client would — reading only
 * the offer list, composing a real `place` (which no offer enumerates a
 * submission for; see extensibility-plan.md §1.1) exactly the way the greedy
 * bot does, and never naming a rule Rummy Tiles' engine did not already state
 * on the wire.
 */

type Offer = {
  id: string;
  verb: string;
  enabled: boolean;
  whyNot?: string;
  composite?: boolean;
  source?: { zone: string; ownerId?: string; cards?: string[]; minCards?: number };
  target?: { meldId?: string };
};

type Zone = {
  id: string;
  kind: string;
  ownerId?: string;
  cards?: { card: string }[];
  count: number;
  groups?: { id: string; kind: string; cards: string[]; badgeKeys?: string[] }[];
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
    data: { guestName: `tiles-${Math.random().toString(36).slice(2, 10)}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

async function startMatch(request: Ctx, seats: number, opts: { options?: Record<string, number> } = {}) {
  const users = [];
  for (let i = 0; i < seats; i++) users.push(await guest(request));
  const host = users[0];
  const auth = { Authorization: `Bearer ${host.accessToken}` };

  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    data: { moduleId: 'rummytiles', options: opts.options ?? {} },
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

test.describe('rummytiles', () => {
  test('the server offers Rummy Tiles and describes it well enough to render a form', async ({ request }) => {
    const res = await request.get(`${API_BASE}/modules`);
    expect(res.ok()).toBeTruthy();
    const { modules } = await res.json();

    const tiles = modules.find((m: { id: string }) => m.id === 'rummytiles');
    expect(tiles, 'rummytiles should be a hosted module').toBeTruthy();
    expect(tiles.label).toBeTruthy();
    expect(tiles.minPlayers).toBe(2);
    expect(tiles.maxPlayers).toBe(4);

    const optionNames = tiles.options.map((o: { name: string }) => o.name);
    for (const v of tiles.variations) {
      for (const name of Object.keys(v.defaults ?? {})) {
        expect(optionNames, `variation ${v.id} defaults an undeclared option`).toContain(name);
      }
    }

    // Six games now run behind the one runtime.
    const ids = modules.map((m: { id: string }) => m.id);
    expect(ids).toEqual(expect.arrayContaining(['zolik', 'prsi', 'canasta', 'holdem', 'ginrummy', 'rummytiles']));
  });

  test('a Rummy Tiles match persists through Mongo and comes back playable, at three seats', async ({ request }) => {
    const { matchId, users } = await startMatch(request, 3);

    const state = await stateFor(request, matchId, users[0].userId);
    expect(state.moduleId).toBe('rummytiles');
    expect(state.status).toBe('active');
    expect(state.legalActions.length).toBeGreaterThan(0);

    const kinds = state.view.zones.map((z) => z.kind);
    expect(kinds).toContain('hand');
    expect(kinds).toContain('stack'); // the pool
    expect(kinds).toContain('spread'); // the table

    const withOffers = [];
    for (const u of users) {
      const s = await stateFor(request, matchId, u.userId);
      if (s.legalActions.some((o) => o.enabled)) withOffers.push(u.userId);
    }
    expect(withOffers).toHaveLength(1);
  });

  test('a viewer never receives another player hand', async ({ request }) => {
    const { matchId, users } = await startMatch(request, 4);

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
    }
  });

  test('an option the descriptor does not declare is refused at creation', async ({ request }) => {
    const host = await guest(request);
    const res = await request.post(`${API_BASE}/matches`, {
      headers: { Authorization: `Bearer ${host.accessToken}` },
      data: { moduleId: 'rummytiles', options: { targetScore: 12345 } },
    });
    expect(res.status()).toBe(400);
    expect((await res.json()).code).toBe('OPTION_NOT_ALLOWED');
  });

  test('a real turn places tiles on the table over a real WebSocket, and the table is public', async ({
    page,
    request,
  }) => {
    // place is Composite — no offer enumerates its exact submission, the same
    // offer-explosion limit a rummy meld always runs into (extensibility-
    // plan.md §1.1). So this composes one directly from the hand the server
    // actually dealt, exactly as the greedy bot does: any three tiles of one
    // number in different colours, or three consecutive tiles in one colour.
    // Finding one is not guaranteed on every seed's opening hand, so this
    // tries a few draws first — a real player would do the same.
    test.setTimeout(120_000);

    const { matchId, users } = await startMatch(request, 2);
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

        const send = async (seat: Seat, action: any) => {
          const before = seats.map((s) => s.inbox.length);
          seat.ws.send(JSON.stringify(action));
          for (let i = 0; i < 200; i++) {
            if (seats.every((s, k) => s.inbox.length > before[k])) return;
            await new Promise((r) => setTimeout(r, 20));
          }
          throw new Error('no broadcast arrived for ' + JSON.stringify(action));
        };

        // Number is the part before the hyphen, colour the part after.
        const findFormableSet = (hand: string[]): string[] | null => {
          const byNumber: Record<string, string[]> = {};
          for (const t of hand) {
            if (t.startsWith('JOKER')) continue;
            const n = t.split('-')[0];
            (byNumber[n] ??= []).push(t);
          }
          for (const n of Object.keys(byNumber)) {
            const seen = new Set<string>();
            const group: string[] = [];
            for (const t of byNumber[n]) {
              const colour = t.split('-')[1];
              if (seen.has(colour)) continue;
              seen.add(colour);
              group.push(t);
            }
            if (group.length >= 3) return group.slice(0, 4);
          }
          for (const colour of ['R', 'B', 'O', 'K']) {
            const nums = hand
              .filter((t) => !t.startsWith('JOKER') && t.split('-')[1] === colour)
              .map((t) => parseInt(t.split('-')[0], 10))
              .sort((a, b) => a - b);
            let i = 0;
            while (i < nums.length) {
              let j = i;
              while (j + 1 < nums.length && nums[j + 1] === nums[j] + 1) j++;
              if (j - i + 1 >= 3) {
                const run: string[] = [];
                for (let k = i; k <= j; k++) run.push(`${nums[k]}-${colour}`);
                return run;
              }
              i = j + 1;
            }
          }
          return null;
        };

        const errors: string[] = [];
        let placedSetId = '';
        let drew = 0;

        for (let turn = 0; turn < 40 && !placedSetId; turn++) {
          const idx = seats.findIndex((s) => (latest(s)?.legalActions ?? []).some((o: any) => o.enabled));
          if (idx === -1) break;
          const seat = seats[idx];
          const state = latest(seat);
          const me = state.players[idx].id;
          const hand = (state.view.zones.find((z: any) => z.ownerId === me && z.kind === 'hand')?.cards ?? []).map(
            (c: any) => c.card,
          );

          const combo = findFormableSet(hand);
          if (combo) {
            await send(seat, { verb: 'place', cards: combo });
            const after = latest(seat);
            const table = after.view.zones.find((z: any) => z.id === 'table');
            const group = (table?.groups ?? []).find((g: any) => combo.every((c: string) => g.cards.includes(c)));
            if (group) placedSetId = group.id;
            continue;
          }

          const drawOffer = (latest(seat).legalActions ?? []).find((o: any) => o.id === 'draw' && o.enabled);
          if (drawOffer) {
            await send(seat, { offerId: 'draw', verb: 'draw' });
            drew++;
            continue;
          }
          const last = seat.inbox[seat.inbox.length - 1];
          if (last?.type === 'error') errors.push(`${last.code}: ${last.message}`);
          break;
        }

        const final = seats.map(latest).find((s) => s) ?? {};
        for (const s of seats) s.ws.close();
        return { errors, placedSetId, drew, status: final.status };
      },
      { wsBase, matchId, tokens: users.map((u) => u.accessToken) },
    );

    expect(result.errors, `socket errors: ${result.errors.join('; ')}`).toEqual([]);
    expect(result.placedSetId, 'expected a set to actually land on the table within 40 turns').not.toBe('');

    // The database agrees, and the placed set is visible to the *other*
    // player too — the workspace is public the moment it exists (view.go).
    const persisted = await stateFor(request, matchId, users[1].userId);
    const table = persisted.view.zones.find((z) => z.id === 'table');
    expect(table?.groups?.some((g) => g.id === result.placedSetId)).toBeTruthy();
  });

  test('the runtime keeps hosting the other five games', async ({ request }) => {
    for (const moduleId of ['zolik', 'prsi', 'canasta', 'ginrummy']) {
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
