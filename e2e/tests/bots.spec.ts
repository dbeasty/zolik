import { expect, test } from '@playwright/test';

import { API_BASE } from '../helpers/env';

/**
 * Bots, for every game (docs/one-architecture-plan.md Phase 6).
 *
 * `POST /matches/{id}/add-bot` used to seat a body that never moved: driving
 * one was rummy-only work living in the rummy runtime, so a new module got a
 * lobby, a socket, and a table that stopped on the first bot's turn. The
 * runtime now drives every bot from the module's own offer list.
 *
 * The load-bearing test is `the turn comes back`. Checking that a human has
 * moves at the start of a match proves nothing — they usually do, because the
 * deal puts them first. What can only happen if a bot really moved is the turn
 * *returning* after the human uses theirs, and that is what is asserted, over a
 * real socket, in four games — one of which the bot machinery has never been
 * told about.
 */

type Ctx = import('@playwright/test').APIRequestContext;

async function guest(request: Ctx) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `bot-${Math.random().toString(36).slice(2, 10)}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

/** One human, one bot, started. */
async function humanVersusBot(
  request: Ctx,
  moduleId: string,
  opts: { variation?: string; options?: Record<string, number> } = {},
) {
  const host = await guest(request);
  const auth = { Authorization: `Bearer ${host.accessToken}` };

  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    data: { moduleId, variation: opts.variation, options: opts.options ?? {} },
  });
  expect(created.ok(), await created.text()).toBeTruthy();
  const { matchId } = await created.json();

  const bot = await request.post(`${API_BASE}/matches/${matchId}/add-bot`, { headers: auth });
  expect(bot.ok(), await bot.text()).toBeTruthy();
  const { playerId: botId } = await bot.json();

  const started = await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });
  expect(started.ok(), await started.text()).toBeTruthy();

  return { matchId, botId, host };
}

async function stateFor(request: Ctx, matchId: string, viewerId: string) {
  const res = await request.get(`${API_BASE}/matches/${matchId}?as=${encodeURIComponent(viewerId)}`);
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

const GAMES = [
  { moduleId: 'prsi', label: 'Prsi', prefer: ['play_card', 'pass', 'draw'] },
  {
    moduleId: 'canasta', label: 'Canasta', options: { targetScore: 500 },
    prefer: ['draw', 'lay_meld', 'lay_off', 'discard'],
  },
  { moduleId: 'holdem', label: 'Holdem', variation: 'timed', prefer: ['check', 'call', 'fold'] },
  { moduleId: 'zolik', label: 'Zoliky', prefer: ['draw', 'discard'] },
];

test.describe('bots play every game', () => {
  for (const game of GAMES) {
    test(`a ${game.label} bot moves, so the turn comes back`, async ({ page, request }) => {
      test.setTimeout(120_000);
      const { matchId, botId, host } = await humanVersusBot(request, game.moduleId, game);
      const wsBase = API_BASE.replace(/^http/, 'ws');

      const result = await page.evaluate(
        async ({ wsBase, matchId, token, prefer, botId }) => {
          const ws = new WebSocket(`${wsBase}/ws/matches/${matchId}?token=${encodeURIComponent(token)}`);
          const inbox: any[] = [];
          await new Promise<void>((resolve, reject) => {
            ws.onopen = () => resolve();
            ws.onerror = () => reject(new Error('socket failed to open'));
            setTimeout(() => reject(new Error('socket open timed out')), 10000);
          });
          ws.onmessage = (ev) => inbox.push(JSON.parse(String(ev.data)));

          const latest = () => {
            for (let i = inbox.length - 1; i >= 0; i--) {
              if (inbox[i].type === 'match_state') return inbox[i];
            }
            return null;
          };
          const settle = async (ms: number) => {
            const deadline = Date.now() + ms;
            while (Date.now() < deadline) {
              if (latest()) return latest();
              await new Promise((r) => setTimeout(r, 50));
            }
            return latest();
          };

          /** Waits until this seat is offered something, or the match ends. */
          const waitForMyTurn = async (ms: number) => {
            const deadline = Date.now() + ms;
            while (Date.now() < deadline) {
              const s = latest();
              if (s && s.status !== 'active') return { state: s, mine: false };
              if (s && (s.legalActions ?? []).some((o: any) => o.enabled)) {
                return { state: s, mine: true };
              }
              await new Promise((r) => setTimeout(r, 100));
            }
            return { state: latest(), mine: false };
          };

          const submissionFor = (o: any) => {
            if (o.composite) return null; // a shape only a person can compose
            const action: any = { offerId: o.id, verb: o.verb };
            const need = o.source?.minCards ?? 0;
            if (need > 0) {
              const cards = o.source?.cards ?? [];
              if (cards.length < need) return null;
              action.cards = cards.slice(0, need);
            }
            if (o.target?.meldId) action.target = o.target.meldId;
            for (const p of o.params ?? []) {
              const v = p.kind === 'int' ? String(p.default ?? p.min ?? 0) : p.choices?.[0]?.value;
              if (v === undefined) return null;
              action.params = { ...(action.params ?? {}), [p.name]: v };
            }
            return action;
          };

          await settle(15000);

          // Wait for our first turn — the bot may legitimately be first.
          let turn = await waitForMyTurn(30000);
          const botMovedFirst = (latest()?.view?.seats ?? []).length > 0;

          // Use our turn, taking as many actions as it takes to hand it over.
          let myActions = 0;
          for (let i = 0; i < 30 && turn.mine; i++) {
            const enabled = (turn.state.legalActions ?? []).filter((o: any) => o.enabled);
            enabled.sort(
              (a: any, b: any) =>
                (prefer.indexOf(a.verb) + 1 || 99) - (prefer.indexOf(b.verb) + 1 || 99),
            );
            let action: any = null;
            for (const o of enabled) {
              action = submissionFor(o);
              if (action) break;
            }
            if (!action) break;

            const before = inbox.length;
            ws.send(JSON.stringify(action));
            myActions++;
            for (let k = 0; k < 100; k++) {
              if (inbox.length > before) break;
              await new Promise((r) => setTimeout(r, 20));
            }
            const s = latest();
            if (!s || s.status !== 'active') break;
            if (!(s.legalActions ?? []).some((o: any) => o.enabled)) break; // handed over
            turn = { state: s, mine: true };
          }

          // The whole point: the turn must come back, which needs the bot to
          // have taken its own.
          const back = await waitForMyTurn(45000);
          const errors = inbox.filter((m) => m.type === 'error').map((m) => `${m.code}: ${m.message}`);
          const final = latest();
          const botSeat = (final?.view?.seats ?? []).find((s: any) => s.playerId === botId);
          const botHand = (final?.view?.zones ?? []).find(
            (z: any) => z.kind === 'hand' && z.ownerId === botId,
          );

          ws.close();
          return {
            myActions,
            botMovedFirst,
            turnCameBack: back.mine,
            status: final?.status ?? 'unknown',
            errors,
            botHasSeat: !!botSeat,
            botCardsVisible: (botHand?.cards ?? []).length,
            standings: final?.standings ?? [],
          };
        },
        { wsBase, matchId, token: host.accessToken, prefer: game.prefer, botId },
      );

      expect(result.errors, `socket errors: ${result.errors.join('; ')}`).toEqual([]);
      expect(result.myActions, 'the human should have had a turn').toBeGreaterThan(0);
      // Either the turn came back to us, or the match finished — both require
      // the bot to have acted. What must not happen is the table simply
      // stopping on the bot's turn, which is what it used to do.
      expect(
        result.turnCameBack || result.status === 'completed',
        `the table stopped on the bot's turn (status ${result.status})`,
      ).toBeTruthy();

      expect(result.botHasSeat, 'the bot should occupy a seat').toBeTruthy();
      expect(result.botCardsVisible, "the bot's cards must stay hidden").toBe(0);
    });
  }

  test('every game reports a scoreboard in the same shape', async ({ request }) => {
    // One screen shows who is ahead at rummy, canasta and poker. None of them
    // measure the same thing, so the shape is what has to be shared.
    for (const game of GAMES) {
      const { matchId, host } = await humanVersusBot(request, game.moduleId, game);
      const state = await stateFor(request, matchId, host.userId);

      const standings = state.standings ?? [];
      expect(standings.length, `${game.label} should keep a scoreboard`).toBeGreaterThan(0);
      for (const s of standings) {
        expect(s.playerId).toBeTruthy();
        expect(s.rank).toBeGreaterThanOrEqual(1);
        expect(s.labelKey, 'a score with no unit cannot be labelled').toBeTruthy();
      }
      expect(standings[0].rank).toBe(1);
    }
  });

  test('every game seats its players and marks exactly one active', async ({ request }) => {
    // Seat.Active is what the runtime drives bots from, so a module whose view
    // disagreed with its own offers would have its bots playing the wrong seat.
    for (const game of GAMES) {
      const { matchId, host } = await humanVersusBot(request, game.moduleId, game);
      const state = await stateFor(request, matchId, host.userId);
      const seats = state.view?.seats ?? [];
      expect(seats.length, `${game.label} should seat its players`).toBe(2);

      const active = seats.filter((s: any) => s.active);
      expect(active.length, `${game.label} should have exactly one active seat`).toBe(1);
    }
  });
});
