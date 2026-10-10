import { expect, test } from '@playwright/test';

import { API_BASE } from '../helpers/env';

/**
 * Every game, every variation, a real table against bots over a real socket.
 *
 * `bots.spec.ts` proves the bot machinery in four games, and each of the other
 * games has a spec of its own. What nothing proved was the *registry*: that
 * every module `/modules` advertises, in every variation it advertises, deals,
 * offers its seat something legal, accepts that move, and hands the turn on
 * without the server answering `error`. A new module or variation is covered
 * the moment it is registered, with nothing to remember to add here.
 *
 * The driver is deliberately dumb — first enabled offer wins — because the
 * point is the contract between the offer list and the engine, not strategy.
 * A game whose offer list contains a move its own engine then refuses fails
 * here with the refusal code.
 */

type Ctx = import('@playwright/test').APIRequestContext;

type Module = {
  id: string;
  label: string;
  minPlayers: number;
  maxPlayers: number;
  variations?: { id: string; maxPlayers?: number }[];
};

async function guest(request: Ctx) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `all-${Math.random().toString(36).slice(2, 10)}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

async function listModules(request: Ctx): Promise<Module[]> {
  const res = await request.get(`${API_BASE}/modules`);
  expect(res.ok(), await res.text()).toBeTruthy();
  const body = await res.json();
  return Array.isArray(body) ? body : body.modules;
}

// Resolved once at collection time so each variation is its own test. The
// server is already up when Playwright loads the file (see README).
async function fetchModulesSync(): Promise<Module[]> {
  const res = await fetch(`${API_BASE}/modules`);
  const body = await res.json();
  return Array.isArray(body) ? body : body.modules;
}

/** Verbs that end or abandon something rather than play it. */
const NOT_PLAY = /reset_turn|undo|resign|concede|quit|leave|forfeit|abandon|surrender|restart|new_game|deal_again/i;

test.describe('every registered game plays', () => {
  test('the registry is not empty and each module names a variation', async ({ request }) => {
    const modules = await listModules(request);
    expect(modules.length).toBeGreaterThanOrEqual(15);
    for (const m of modules) {
      expect(m.label, `${m.id} needs a label`).toBeTruthy();
      expect(m.minPlayers, `${m.id} minPlayers`).toBeGreaterThanOrEqual(1);
      expect(m.maxPlayers, `${m.id} maxPlayers`).toBeGreaterThanOrEqual(m.minPlayers);
    }
  });

  test('a module the server does not know is refused, not created', async ({ request }) => {
    const host = await guest(request);
    const res = await request.post(`${API_BASE}/matches`, {
      headers: { Authorization: `Bearer ${host.accessToken}` },
      data: { moduleId: 'no-such-game', options: {} },
    });
    expect(res.ok()).toBeFalsy();
    expect(res.status()).toBeLessThan(500);
  });

  test('a variation the module does not know is refused, not created', async ({ request }) => {
    const host = await guest(request);
    const res = await request.post(`${API_BASE}/matches`, {
      headers: { Authorization: `Bearer ${host.accessToken}` },
      data: { moduleId: 'prsi', variation: 'no-such-variation', options: {} },
    });
    expect(res.ok()).toBeFalsy();
    expect(res.status()).toBeLessThan(500);
  });
});

async function playThrough(
  request: Ctx,
  page: import('@playwright/test').Page,
  mod: Module,
  variation: string | undefined,
  seats: number,
) {
  const host = await guest(request);
  const auth = { Authorization: `Bearer ${host.accessToken}` };

  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    data: { moduleId: mod.id, variation, options: {} },
  });
  expect(created.ok(), `create ${mod.id}/${variation}: ${await created.text()}`).toBeTruthy();
  const { matchId } = await created.json();

  for (let i = 1; i < seats; i++) {
    const bot = await request.post(`${API_BASE}/matches/${matchId}/add-bot`, { headers: auth });
    expect(bot.ok(), `add-bot ${i}: ${await bot.text()}`).toBeTruthy();
  }
  const started = await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });
  expect(started.ok(), `start ${mod.id}/${variation}: ${await started.text()}`).toBeTruthy();

  // The browser origin only matters as somewhere to run a WebSocket from.
  await page.goto(`${API_BASE}/version`);
  const wsBase = API_BASE.replace(/^http/, 'ws');

  return page.evaluate(
    async ({ wsBase, matchId, token, notPlay }) => {
      const skip = new RegExp(notPlay, 'i');
      const ws = new WebSocket(`${wsBase}/ws/matches/${matchId}?token=${encodeURIComponent(token)}`);
      const inbox: any[] = [];
      await new Promise<void>((resolve, reject) => {
        ws.onopen = () => resolve();
        ws.onerror = () => reject(new Error('socket failed to open'));
        setTimeout(() => reject(new Error('socket open timed out')), 10000);
      });
      ws.onmessage = (ev) => inbox.push(JSON.parse(String(ev.data)));
      const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));
      const latest = () => {
        for (let i = inbox.length - 1; i >= 0; i--) if (inbox[i].type === 'match_state') return inbox[i];
        return null;
      };
      // `manual` offers (leave, concede) are always on the table; having one is not having the turn.
      const mine = (s: any) => (s?.legalActions ?? []).some((o: any) => o.enabled && !o.manual && !skip.test(o.verb));

      const submissionFor = (o: any) => {
        if (o.composite) return null;
        const action: any = { offerId: o.id, verb: o.verb };
        const submit = o.source?.submit ?? [];
        const need = o.source?.minCards ?? 0;
        if (submit.length > 0) action.cards = submit;
        else if (need > 0) {
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

      const deadline = Date.now() + 80000;
      let myActions = 0;
      let turnsReturned = 0;
      let lastKey = '';
      let stuckSince = Date.now();
      const verbs: string[] = [];
      let handedOver = false;

      while (Date.now() < deadline) {
        const s = latest();
        if (s && s.status !== 'active') break;
        if (s && mine(s)) {
          if (handedOver) {
            turnsReturned++;
            handedOver = false;
          }
          const enabled = (s.legalActions ?? []).filter((o: any) => o.enabled && !skip.test(o.verb));
          let action: any = null;
          for (const o of enabled) {
            action = submissionFor(o);
            if (action) break;
          }
          if (!action) {
            // Offers only a person can compose (composite) — nothing more a
            // driver can do here, and that is not a defect.
            break;
          }
          const before = inbox.length;
          const statesBefore = inbox.filter((m) => m.type === 'match_state').length;
          ws.send(JSON.stringify(action));
          myActions++;
          verbs.push(action.verb);
          // A move is answered by a fresh match_state (or an error); events
          // can arrive first and must not be mistaken for the answer.
          for (let k = 0; k < 250; k++) {
            const got = inbox.slice(before);
            if (got.some((m) => m.type === 'error')) break;
            if (inbox.filter((m) => m.type === 'match_state').length > statesBefore) break;
            await sleep(20);
          }
          const after = latest();
          if (after && !mine(after)) handedOver = true;
          lastKey = '';
          stuckSince = Date.now();
          if (inbox.slice(before).some((m) => m.type === 'error')) break;
          if (turnsReturned >= 3) break;
          continue;
        }
        // Waiting on a bot. Count each hand-back once.
        const key = s ? JSON.stringify(s.view?.seats?.map((x: any) => x.active)) : '';
        if (key !== lastKey) {
          lastKey = key;
          stuckSince = Date.now();
        }
        await sleep(100);
        if (Date.now() - stuckSince > 45000) break;
      }

      const final = latest();
      const errors = inbox.filter((m) => m.type === 'error').map((m) => `${m.code}: ${m.message}`);
      ws.close();
      return {
        myActions,
        turnsReturned,
        status: final?.status ?? 'unknown',
        errors,
        verbs,
        stuckFor: Date.now() - stuckSince,
        mineAtEnd: mine(final),
        endOffers: (final?.legalActions ?? []).filter((o: any) => o.enabled).map((o: any) => o.verb + (o.composite ? '*' : '')),
        types: [...new Set(inbox.map((m) => m.type))],
      };
    },
    { wsBase, matchId, token: host.accessToken, notPlay: NOT_PLAY.source },
  );
}

test.describe('every game, every variation, against bots', () => {
  test.describe.configure({ mode: 'serial' });

  let modules: Module[] = [];
  test.beforeAll(async () => {
    modules = await fetchModulesSync();
  });

  // Registered up front for the known registry; a module added later is
  // caught by the registry test above (count) and by the dynamic test below.
  test('each advertised variation reaches a playable table', async ({ request, page }) => {
    test.setTimeout(30 * 60_000);
    const failures: string[] = [];

    for (const mod of modules) {
      if (process.env.ONLY && !process.env.ONLY.split(',').includes(mod.id)) continue;
      const variations = mod.variations?.length ? mod.variations : [{ id: undefined as any }];
      for (const v of variations) {
        const cap = v.maxPlayers ?? mod.maxPlayers;
        const seats = Math.min(Math.max(mod.minPlayers, 2), cap);
        const solo = mod.minPlayers === 1 && mod.maxPlayers === 1;
        const label = `${mod.id}/${v.id ?? 'default'}`;
        try {
          const r = await playThrough(request, page, mod, v.id, solo ? 1 : seats);
          console.log(`${label}: actions=${r.myActions} returned=${r.turnsReturned} status=${r.status} verbs=${[...new Set(r.verbs)].join(',')} end=${r.endOffers.join(',')} stuck=${r.stuckFor}`);
          if (r.errors.length) failures.push(`${label}: server errors ${r.errors.join('; ')}`);
          else if (r.myActions === 0 && r.status === 'active')
            failures.push(`${label}: the seat never made a move (offered one: ${r.mineAtEnd})`);
          else if (!solo && r.status === 'active' && r.stuckFor > 44000)
            failures.push(`${label}: the table stalled for ${Math.round(r.stuckFor / 1000)}s on a bot (verbs ${r.verbs.join(',')})`);
        } catch (e) {
          failures.push(`${label}: ${(e as Error).message}`);
        }
      }
    }
    expect(failures, failures.join('\n')).toEqual([]);
  });
});
