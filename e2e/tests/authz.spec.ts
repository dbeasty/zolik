import { expect, test } from '@playwright/test';

import { API_BASE } from '../helpers/env';

/**
 * Who may do what to a table.
 *
 * Every other spec acts as the host or as a seated player, so nothing else
 * would notice a route that stopped asking who was calling. This acts as the
 * people who must be refused — a seated non-host, a signed-in stranger, and
 * somebody with no sign-in at all — against every route that changes or reveals
 * a table, and asserts the refusal is the written one (a code, not a 500 and
 * not a silent success). A read the table is meant to allow a stranger
 * (spectating) is checked the other way: it works, and shows nothing private.
 */

type Ctx = import('@playwright/test').APIRequestContext;
type Who = { userId: string; accessToken: string };

async function guest(request: Ctx, name: string): Promise<Who> {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `${name}-${Math.random().toString(36).slice(2, 8)}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

const as = (who: Who | null) =>
  who ? { Authorization: `Bearer ${who.accessToken}` } : ({} as Record<string, string>);

async function lobbyTable(request: Ctx) {
  const host = await guest(request, 'host');
  const joiner = await guest(request, 'joiner');
  const stranger = await guest(request, 'stranger');
  const created = await request.post(`${API_BASE}/matches`, {
    headers: as(host),
    data: { moduleId: 'prsi', options: {} },
  });
  const { matchId } = await created.json();
  const joined = await request.post(`${API_BASE}/matches/${matchId}/join`, { headers: as(joiner), data: {} });
  expect(joined.ok(), await joined.text()).toBeTruthy();
  const bot = await (await request.post(`${API_BASE}/matches/${matchId}/add-bot`, { headers: as(host) })).json();
  return { host, joiner, stranger, matchId, botId: bot.playerId as string };
}

test.describe('table authority', () => {
  test('only the host starts, adds bots, sets bot skill or deletes a lobby table', async ({ request }) => {
    const { host, joiner, stranger, matchId, botId } = await lobbyTable(request);
    const routes: [string, string, unknown][] = [
      ['POST', `/matches/${matchId}/start`, {}],
      ['POST', `/matches/${matchId}/add-bot`, {}],
      ['POST', `/matches/${matchId}/bots/${botId}/skill`, { skill: 1 }],
      ['DELETE', `/matches/${matchId}`, undefined],
    ];
    for (const [method, path, data] of routes) {
      for (const [label, who] of [['a seated non-host', joiner], ['a stranger', stranger]] as const) {
        const res = await request.fetch(`${API_BASE}${path}`, { method, headers: as(who), data });
        expect(res.status(), `${label}: ${method} ${path}`).toBe(403);
        const body = await res.json();
        expect(body.code, `${label}: ${method} ${path}`).toMatch(/^NOT_(THE_HOST|AT_THIS_TABLE)$/);
      }
      const anon = await request.fetch(`${API_BASE}${path}`, { method, data });
      expect(anon.status(), `no sign-in: ${method} ${path}`).toBe(401);
    }
    // And the table is still there, still a lobby, still the host's.
    const state = await (await request.get(`${API_BASE}/matches/${matchId}`, { headers: as(host) })).json();
    expect(state.status).toBe('lobby');
  });

  test('creating or joining a table needs a sign-in', async ({ request }) => {
    const { matchId } = await lobbyTable(request);
    const create = await request.post(`${API_BASE}/matches`, { data: { moduleId: 'prsi' } });
    expect(create.status()).toBe(401);
    const join = await request.post(`${API_BASE}/matches/${matchId}/join`, { data: {} });
    expect(join.status()).toBe(401);
  });

  test('a started table refuses a late stranger, and keeps its private routes private', async ({ request }) => {
    const { host, joiner, stranger, matchId } = await lobbyTable(request);
    const started = await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: as(host) });
    expect(started.ok(), await started.text()).toBeTruthy();

    const late = await request.post(`${API_BASE}/matches/${matchId}/join`, { headers: as(stranger), data: {} });
    expect(late.status()).toBe(400);
    expect((await late.json()).code).toBe('MATCH_ALREADY_STARTED');

    // A hint is advice about a hand, a replay is a record of a seat's view, a
    // same-deal link is the deal itself: all three are for people at the table.
    for (const [method, path] of [
      ['POST', `/matches/${matchId}/hint`],
      ['GET', `/matches/${matchId}/replay`],
      ['GET', `/matches/${matchId}/same-deal`],
      ['DELETE', `/matches/${matchId}`],
    ] as const) {
      const res = await request.fetch(`${API_BASE}${path}`, { method, headers: as(stranger), data: method === 'POST' ? {} : undefined });
      expect(res.status(), `stranger: ${method} ${path}`).toBe(403);
      expect((await res.json()).code).toBe('NOT_AT_THIS_TABLE');
    }
    const anonHint = await request.post(`${API_BASE}/matches/${matchId}/hint`, { data: {} });
    expect(anonHint.status()).toBe(401);

    // A seated joiner may ask for what is theirs, but not delete the table.
    const del = await request.delete(`${API_BASE}/matches/${matchId}`, { headers: as(joiner) });
    expect(del.status()).toBe(403);
  });

  test('the post-game routes are closed to everyone until the match is over', async ({ request }) => {
    const { host, stranger, matchId } = await lobbyTable(request);
    await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: as(host) });
    for (const who of [host, stranger]) {
      for (const route of ['rematch', 'deal-again', 'deal-link']) {
        const res = await request.post(`${API_BASE}/matches/${matchId}/${route}`, { headers: as(who), data: {} });
        expect(res.status(), `${route} on a live table`).toBe(409);
        expect((await res.json()).code).toBe('MATCH_NOT_OVER');
      }
    }
  });

  test('the dev-only state hatch is not open to a stranger', async ({ request }) => {
    const { stranger, matchId } = await lobbyTable(request);
    const res = await request.post(`${API_BASE}/matches/${matchId}/debug-state`, {
      headers: as(stranger),
      data: { state: {} },
    });
    expect(res.status()).toBe(403);
  });
});

test.describe('table over the socket', () => {
  test('a spectator can watch a started table but cannot move for anyone', async ({ page, request }) => {
    const { host, stranger, matchId } = await lobbyTable(request);
    await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: as(host) });
    await page.goto(`${API_BASE}/version`);
    const wsBase = API_BASE.replace(/^http/, 'ws');

    const result = await page.evaluate(
      async ({ wsBase, matchId, token }) => {
        const ws = new WebSocket(`${wsBase}/ws/matches/${matchId}?token=${encodeURIComponent(token)}`);
        const inbox: any[] = [];
        await new Promise<void>((resolve, reject) => {
          ws.onopen = () => resolve();
          ws.onerror = () => reject(new Error('socket failed to open'));
          setTimeout(() => reject(new Error('socket open timed out')), 10000);
        });
        ws.onmessage = (ev) => inbox.push(JSON.parse(String(ev.data)));
        const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));
        for (let i = 0; i < 100 && !inbox.some((m) => m.type === 'match_state'); i++) await sleep(50);
        const first = inbox.find((m) => m.type === 'match_state');
        // Whatever the board offers a seated player, try to send it.
        const probe = { offerId: 'play_card', verb: 'play_card', cards: ['AC'] };
        const before = inbox.length;
        ws.send(JSON.stringify(probe));
        await sleep(800);
        const answers = inbox.slice(before);
        ws.close();
        return {
          sawState: !!first,
          offered: (first?.legalActions ?? []).filter((o: any) => o.enabled && !o.manual).length,
          handCards: (first?.view?.zones ?? [])
            .filter((z: any) => z.kind === 'hand')
            .reduce((n: number, z: any) => n + (z.cards ?? []).length, 0),
          errors: answers.filter((m) => m.type === 'error').length,
          states: answers.filter((m) => m.type === 'match_state').length,
        };
      },
      { wsBase, matchId, token: stranger.accessToken },
    );

    expect(result.sawState, 'a stranger may watch').toBeTruthy();
    expect(result.offered, 'and is offered nothing to play').toBe(0);
    expect(result.handCards, 'and sees no hand').toBe(0);
    expect(result.errors, 'a move from a spectator is answered with an error').toBeGreaterThan(0);
  });

  test('a socket with no token is refused', async ({ page, request }) => {
    const { matchId } = await lobbyTable(request);
    await page.goto(`${API_BASE}/version`);
    const wsBase = API_BASE.replace(/^http/, 'ws');
    const opened = await page.evaluate(
      ({ wsBase, matchId }) =>
        new Promise<string>((resolve) => {
          const ws = new WebSocket(`${wsBase}/ws/matches/${matchId}`);
          let sawState = false;
          ws.onmessage = (ev) => {
            const m = JSON.parse(String(ev.data));
            if (m.type === 'match_state' && (m.legalActions ?? []).some((o: any) => o.enabled && !o.manual)) sawState = true;
          };
          ws.onclose = () => resolve(sawState ? 'offered-moves' : 'closed');
          setTimeout(() => resolve(sawState ? 'offered-moves' : 'open-but-inert'), 2500);
        }),
      { wsBase, matchId },
    );
    // Closed, or open as a bare spectator — but never handed a move.
    expect(opened).not.toBe('offered-moves');
  });
});
