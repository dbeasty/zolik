import { expect, test } from '@playwright/test';

import { tapCard } from '../helpers/drag';
import { API_BASE } from '../helpers/env';
import { cardByCode, handCodes } from '../helpers/hand';
import { openGame, openGameSetup } from '../helpers/lobby';
import { loginAsFreshGuest } from '../helpers/login';
import { waitForOfferEnabled } from '../helpers/turn';

/**
 * End-to-end for Last Card (docs/lastcard-plan.md, phase 3).
 *
 * The Go suites prove the rules in memory — the deck, the call and the catch,
 * the challenge, the stacking, the scoring — and play whole matches from
 * offers alone. What only this can prove is the rest of the stack: a match
 * over real WebSockets with every house rule switched on, and a board in a
 * real browser that draws this pack's own faces, which no other game uses.
 *
 * The play-through reads the offer list and nothing else.
 */

type Offer = {
  id: string;
  verb: string;
  enabled: boolean;
  source?: { cards?: string[]; minCards?: number; submit?: string[] };
  params?: { name: string; choices?: { value: string }[] }[];
};

type MatchState = {
  moduleId: string;
  deck?: string;
  status: string;
  view: { zones: { id: string; kind: string; ownerId?: string; cards?: { card: string }[]; count: number }[] };
  legalActions: Offer[];
  rounds?: { rounds: { number: number; winners: string[]; scores: { delta: number; total: number; lines?: { points: number }[] }[] }[] };
};

type Ctx = import('@playwright/test').APIRequestContext;

const CODE = /^([CTVA]-([0-9]|S|R|D)|W4?)$/;

async function guest(request: Ctx) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `lastcard-${Math.random().toString(36).slice(2, 10)}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

/** A table of real players, opened and started; the test drives every seat. */
async function startMatch(request: Ctx, seats: number, options: Record<string, number> = {}) {
  const users = [];
  for (let i = 0; i < seats; i++) users.push(await guest(request));
  const auth = { Authorization: `Bearer ${users[0].accessToken}` };
  const created = await request.post(`${API_BASE}/matches`, { headers: auth, data: { moduleId: 'lastcard', options } });
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

async function stateFor(request: Ctx, matchId: string, user: { accessToken: string }): Promise<MatchState> {
  const res = await request.get(`${API_BASE}/matches/${matchId}`, {
    headers: { Authorization: `Bearer ${user.accessToken}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

test.describe('last card', () => {
  test('the server offers Last Card for two to ten, dealt from its own pack', async ({ request }) => {
    const { modules } = await (await request.get(`${API_BASE}/modules`)).json();
    const lc = modules.find((m: { id: string }) => m.id === 'lastcard');
    expect(lc, 'lastcard should be a hosted module').toBeTruthy();
    expect(lc.minPlayers).toBe(2);
    expect(lc.maxPlayers).toBe(10);
    expect(lc.deck).toBe('lastcard');
    expect(lc.options.map((o: { name: string }) => o.name)).toEqual(
      expect.arrayContaining(['targetScore', 'lastCardCall', 'drawFourChallenge', 'stacking', 'drawUntilPlayable', 'sevenZero']),
    );
  });

  test('the setup offers no AI opponent, since the game ships no trained network', async ({ page, request }) => {
    await loginAsFreshGuest(page, request, 'Last Card setup');
    await openGame(page, 'lastcard');
    await openGameSetup(page, 'lastcard', 'bots');
    await page.getByTestId('bots-lastcard-1').click();
    // One strength for the whole table; per-seat strength is set at the table.
    await expect(page.getByTestId('bot-skill-lastcard-0-hard')).toHaveCount(0);
    await expect(page.getByTestId('option-lastcard-botSkill-3')).toBeVisible();
    await expect(page.getByTestId('option-lastcard-botSkill-4')).toHaveCount(0);

    // Where a game does ship one, both controls still offer it.
    await openGame(page, 'holdem');
    await openGameSetup(page, 'holdem', 'bots');
    await expect(page.getByTestId('option-holdem-botSkill-4')).toBeVisible();
  });

  test('every seat sees its own hand, in the pack’s own codes, and nobody else’s', async ({ request }) => {
    const { matchId, users } = await startMatch(request, 3);
    for (const viewer of users) {
      const state = await stateFor(request, matchId, viewer);
      expect(state.deck).toBe('lastcard');
      for (const z of state.view.zones.filter((z) => z.kind === 'hand')) {
        if (z.ownerId === viewer.userId) {
          expect(z.cards!.length).toBe(z.count);
          for (const c of z.cards!) expect(c.card).toMatch(CODE);
        } else {
          expect(z.cards ?? []).toHaveLength(0);
        }
      }
    }
  });

  test('a whole match to 200 plays out over real WebSockets with every house rule on', async ({ page, request }) => {
    test.setTimeout(240_000);
    const { matchId, users } = await startMatch(request, 3, {
      targetScore: 200,
      stacking: 2,
      drawUntilPlayable: 1,
      sevenZero: 1,
      pauseBetweenRounds: 1,
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
        const seats = await Promise.all(tokens.map(open));
        for (let i = 0; i < 200 && !seats.every(latest); i++) await new Promise((r) => setTimeout(r, 50));

        const submissionFor = (o: any) => {
          const action: any = { offerId: o.id, verb: o.verb };
          const need = o.source?.minCards ?? 0;
          if (need > 0) {
            const cards = o.source?.cards ?? [];
            if (cards.length < need) return null;
            action.cards = cards.slice(0, need);
          }
          for (const p of o.params ?? []) {
            const v = p.choices?.length ? p.choices[0].value : String(p.default ?? p.min ?? 0);
            action.params = { ...(action.params ?? {}), [p.name]: v };
          }
          return action;
        };
        // Answer what is asked first, then play, then draw.
        const order = ['continue', 'catch', 'call', 'accept', 'play_card', 'pass', 'draw'];

        const verbs: Record<string, number> = {};
        const errors: string[] = [];
        for (let step = 0; step < 6000; step++) {
          const idx = seats.findIndex((s) => (latest(s)?.legalActions ?? []).some((o: any) => o.enabled));
          if (idx === -1) break;
          const state = latest(seats[idx]);
          if (state.status !== 'active') break;
          const enabled = state.legalActions.filter((o: any) => o.enabled && order.includes(o.verb));
          enabled.sort((a: any, b: any) => order.indexOf(a.verb) - order.indexOf(b.verb));
          const action = enabled.map(submissionFor).find(Boolean);
          if (!action) break;

          const before = seats.map((s) => s.inbox.length);
          seats[idx].ws.send(JSON.stringify(action));
          verbs[action.verb] = (verbs[action.verb] ?? 0) + 1;
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
        return { verbs, errors, status: final.status };
      },
      { wsBase, matchId, tokens: users.map((u) => u.accessToken) },
    );

    expect(result.errors, `socket errors: ${result.errors.join('; ')}`).toEqual([]);
    expect(result.status).toBe('completed');
    for (const verb of ['play_card', 'draw', 'call']) {
      expect(result.verbs[verb] ?? 0, `${verb} was never played`).toBeGreaterThan(0);
    }

    // The score sheet: one winner a deal, whose lines add up to what they
    // scored, and a match winner past the target.
    const persisted = await stateFor(request, matchId, users[0]);
    const rounds = persisted.rounds?.rounds ?? [];
    // Every deal but the last ended in the pause, and the table went on from
    // it. (One big deal can reach the target on its own, with no pause at all.)
    if (rounds.length > 1) {
      expect(result.verbs['continue'] ?? 0, 'continue was never played').toBeGreaterThan(0);
    }
    expect(rounds.length).toBeGreaterThan(0);
    for (const r of rounds) {
      expect(r.winners).toHaveLength(1);
      for (const s of r.scores) {
        if (!s.lines?.length) continue;
        expect(s.lines.reduce((a, l) => a + l.points, 0), `deal ${r.number}`).toBe(s.delta);
      }
    }
    const last = rounds[rounds.length - 1];
    expect(Math.max(...last.scores.map((s) => s.total))).toBeGreaterThanOrEqual(200);
  });

  test('the board draws this pack’s own faces, and a card tapped and played lands on the pile', async ({ page, request }) => {
    test.setTimeout(120_000);
    const me = await loginAsFreshGuest(page, request, 'Last Card tester');
    await openGame(page, 'lastcard');
    await openGameSetup(page, 'lastcard', 'bots');
    await page.getByTestId('bots-lastcard-1').click();
    await page.getByTestId('option-lastcard-targetScore-0').click();
    await page.getByTestId('deal-me-in-lastcard').click();
    await page.waitForURL(/\/match\//, { timeout: 30_000 });
    const matchId = page.url().split('/match/')[1].split(/[?#]/)[0];

    await waitForOfferEnabled(page, 'offer-draw', 60_000);
    const state = await stateFor(request, matchId, me);
    const mine = state.view.zones.find((z) => z.kind === 'hand' && z.ownerId === me.userId)!;

    // What a player sees: every card in hand is one of this pack's codes, and
    // every one is drawn — an illustrated face, not an empty card.
    const codes = await handCodes(page);
    expect(codes.length).toBe(mine.count);
    for (const c of codes) expect(c).toMatch(CODE);
    const drawn = await page.evaluate(() =>
      [...document.querySelectorAll('[data-testid^="card-hand:"]')].map(
        (card) => card.querySelectorAll('svg path').length,
      ),
    );
    expect(drawn.length).toBe(mine.count);
    for (const paths of drawn) expect(paths).toBeGreaterThan(3);

    // Play a card the server says fits — a coloured one, so no colour has to
    // be named — or draw if nothing does.
    const play = state.legalActions.find((o) => o.id === 'play_card');
    const card = (play?.enabled ? play.source?.cards ?? [] : []).find((c) => c !== 'W' && c !== 'W4');
    const pileBefore = state.view.zones.find((z) => z.id === 'discard')!.count;
    if (card) {
      await tapCard(page, cardByCode(page, card));
      await page.getByTestId('offer-play_card').click();
      await expect
        .poll(async () => (await stateFor(request, matchId, me)).view.zones.find((z) => z.id === 'discard')!.count, {
          timeout: 15_000,
        })
        .toBeGreaterThan(pileBefore);
    } else {
      await page.getByTestId('offer-draw').click();
      await expect
        .poll(async () => (await stateFor(request, matchId, me)).view.zones.find((z) => z.kind === 'hand' && z.ownerId === me.userId)!.count, {
          timeout: 15_000,
        })
        .toBeGreaterThan(mine.count);
    }
    // And the header says which colour is in play, in words.
    await expect(page.getByText(/Colour in play/)).toBeVisible();
  });
});

test.describe('last card, lowest total wins', () => {
  test('everyone pays for their own hand, and the lowest total takes the match', async ({ request }) => {
    test.setTimeout(240_000);
    const { matchId, users } = await startMatch(request, 3, { targetScore: 200, scoring: 1, pauseBetweenRounds: 0 });
    const wsBase = API_BASE.replace(/^http/, 'ws');
    const sockets = users.map((u) => new WebSocket(`${wsBase}/ws/matches/${matchId}?token=${encodeURIComponent(u.accessToken)}`));
    const latest: any[] = users.map(() => null);
    sockets.forEach((ws, i) => {
      ws.onmessage = (ev) => {
        const msg = JSON.parse(String(ev.data));
        if (msg.type === 'match_state') latest[i] = msg;
      };
    });
    for (let i = 0; i < 200 && latest.some((l) => !l); i++) await new Promise((r) => setTimeout(r, 50));

    // Any legal move will do; the point is the arithmetic at the end of each deal.
    const order = ['continue', 'accept', 'catch', 'call', 'play_card', 'pass', 'draw'];
    for (let step = 0; step < 8000; step++) {
      const i = latest.findIndex((l) => (l?.legalActions ?? []).some((o: any) => o.enabled));
      if (i === -1 || latest[i].status !== 'active') break;
      const o = latest[i].legalActions
        .filter((x: any) => x.enabled && order.includes(x.verb))
        .sort((a: any, b: any) => order.indexOf(a.verb) - order.indexOf(b.verb))[0];
      const action: any = { offerId: o.id, verb: o.verb };
      if ((o.source?.minCards ?? 0) > 0) action.cards = o.source.cards.slice(0, 1);
      for (const p of o.params ?? []) action.params = { ...(action.params ?? {}), [p.name]: p.defaultChoice ?? p.choices[0].value };
      const before = latest.map((l) => l);
      sockets[i].send(JSON.stringify(action));
      for (let k = 0; k < 200 && latest.every((l, j) => l === before[j]); k++) await new Promise((r) => setTimeout(r, 10));
    }
    for (const ws of sockets) ws.close();

    const final = await stateFor(request, matchId, users[0]);
    expect(final.status).toBe('completed');
    const rounds = final.rounds?.rounds ?? [];
    expect(rounds.length).toBeGreaterThan(0);
    for (const r of rounds) {
      for (const s of r.scores) {
        // The deal's winner pays nothing; everyone else pays their own lines.
        if (r.winners.includes((s as any).playerId)) expect(s.delta).toBe(0);
        if (s.lines?.length) expect(s.lines.reduce((a, l) => a + l.points, 0)).toBe(s.delta);
      }
    }
    const totals = rounds[rounds.length - 1].scores.map((s) => s.total);
    expect(Math.max(...totals), 'someone reached the target').toBeGreaterThanOrEqual(200);
    const standings = (final as any).standings as { playerId: string; rank: number; shown?: number }[];
    const lowest = Math.min(...totals);
    expect(standings.find((s) => s.rank === 1)?.shown, 'the winner holds the lowest total').toBe(lowest);
  });
});
