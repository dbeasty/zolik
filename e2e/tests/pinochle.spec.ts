import { expect, test, type Page } from '@playwright/test';

import { API_BASE, asViewer } from '../helpers/env';
import { selectOnly } from '../helpers/hand';

/**
 * End-to-end for the Pinochle module (docs/pinochle-rules.md).
 *
 * The Go suites prove the rules in memory: the meld table, the follow-beat-
 * trump obligations, the auction and the settlement, and the shared contract
 * suite plays whole matches of every variation from offers alone. What only
 * this can prove is the rest of the stack — real HTTP, real WebSockets, real
 * persistence — for a pack in which every card exists twice, a widow, a
 * two-handed stock, and the web shell drawing all of it.
 *
 * The play-throughs read the offer list and nothing else; they name no rule
 * of Pinochle beyond which verb to prefer.
 */

type Offer = {
  id: string;
  verb: string;
  enabled: boolean;
  source?: { cards?: string[]; minCards?: number; submit?: string[] };
  params?: { name: string; kind?: string; min?: number; default?: number; choices?: { value: string }[] }[];
  facts?: { labelKey: string }[];
};

type Zone = {
  id: string;
  kind: string;
  ownerId?: string;
  cards?: { card: string; by?: string }[];
  groups?: { kind?: string; cards: string[] }[];
  count: number;
  arrange?: string;
};

type MatchState = {
  moduleId: string;
  status: string;
  players: { id: string }[];
  view: { zones: Zone[]; seats?: { playerId: string; side?: string }[] };
  legalActions: Offer[];
  standings?: { playerId: string; score: number }[];
};

type Ctx = import('@playwright/test').APIRequestContext;
type User = { userId: string; accessToken: string; refreshToken: string; username?: string };

async function guest(request: Ctx): Promise<User> {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `pinochle-${Math.random().toString(36).slice(2, 10)}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

/** A table of real players, so the whole match is driven by the test. */
async function startMatch(request: Ctx, seats: number, variation: string, options: Record<string, number> = {}) {
  const users: User[] = [];
  for (let i = 0; i < seats; i++) users.push(await guest(request));
  const auth = { Authorization: `Bearer ${users[0].accessToken}` };
  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    data: { moduleId: 'pinochle', variation, options },
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
  return { matchId, users };
}

async function stateFor(request: Ctx, matchId: string, user: User): Promise<MatchState> {
  const res = await request.get(`${API_BASE}/matches/${matchId}`, asViewer(user));
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

/** One person and bots: what a player opening a table from the menu gets. */
async function tableWithBots(request: Ctx, seats: number, variation: string, options: Record<string, number> = {}) {
  const host = await guest(request);
  const auth = { Authorization: `Bearer ${host.accessToken}` };
  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    data: { moduleId: 'pinochle', variation, options },
  });
  expect(created.ok(), await created.text()).toBeTruthy();
  const { matchId } = await created.json();
  for (let i = 1; i < seats; i++) {
    const bot = await request.post(`${API_BASE}/matches/${matchId}/add-bot`, { headers: auth });
    expect(bot.ok(), await bot.text()).toBeTruthy();
  }
  const started = await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });
  expect(started.ok(), await started.text()).toBeTruthy();
  return { matchId, host };
}

async function signIn(page: Page, host: User) {
  await page.addInitScript((s) => {
    window.localStorage.setItem('zolik_session', JSON.stringify(s));
  }, {
    accessToken: host.accessToken,
    refreshToken: host.refreshToken,
    userId: host.userId,
    username: host.username ?? 'pinochle',
    isGuest: true,
  });
}

test.describe('pinochle', () => {
  test('the server offers Pinochle in four forms, each with its own table size', async ({ request }) => {
    const { modules } = await (await request.get(`${API_BASE}/modules`)).json();
    const pinochle = modules.find((m: { id: string }) => m.id === 'pinochle');
    expect(pinochle, 'pinochle should be a hosted module').toBeTruthy();
    expect(pinochle.minPlayers).toBe(2);
    expect(pinochle.maxPlayers).toBe(4);
    const seats = Object.fromEntries(
      pinochle.variations.map((v: { id: string; minPlayers: number; maxPlayers: number }) => [v.id, [v.minPlayers, v.maxPlayers]]),
    );
    expect(seats).toEqual({
      partnership: [4, 4],
      cutthroat: [3, 3],
      twohanded: [2, 2],
      doubledeck: [4, 4],
    });
    expect(pinochle.options.map((o: { name: string }) => o.name)).toEqual(
      expect.arrayContaining(['scoring', 'minBid', 'target', 'passCards', 'meldNeeds', 'allPass']),
    );
  });

  test('a cutthroat table refuses a fourth player', async ({ request }) => {
    const users = [await guest(request), await guest(request), await guest(request), await guest(request)];
    const auth = { Authorization: `Bearer ${users[0].accessToken}` };
    const created = await request.post(`${API_BASE}/matches`, { headers: auth, data: { moduleId: 'pinochle', variation: 'cutthroat' } });
    expect(created.ok(), await created.text()).toBeTruthy();
    const { matchId } = await created.json();
    const outcomes: boolean[] = [];
    for (const u of users.slice(1)) {
      const joined = await request.post(`${API_BASE}/matches/${matchId}/join`, { headers: { Authorization: `Bearer ${u.accessToken}` } });
      outcomes.push(joined.ok());
    }
    expect(outcomes).toEqual([true, true, false]);
  });

  test('only one seat is asked to bid, every bid starts at the minimum, and nobody sees another hand', async ({ request }) => {
    const { matchId, users } = await startMatch(request, 4, 'partnership', { minBid: 300 });
    const asked: string[] = [];
    for (const viewer of users) {
      const state = await stateFor(request, matchId, viewer);
      expect(state.moduleId).toBe('pinochle');
      const bid = state.legalActions.find((o) => o.id === 'bid');
      expect(bid, 'every seat sees the bid control').toBeTruthy();
      if (bid!.enabled) {
        asked.push(viewer.userId);
        expect(bid!.params?.[0]).toMatchObject({ kind: 'int', min: 300, default: 300 });
      }
      for (const z of state.view.zones.filter((z) => z.kind === 'hand')) {
        if (z.ownerId === viewer.userId) {
          expect(z.cards!.length).toBe(12);
        } else {
          expect(z.cards ?? []).toHaveLength(0);
          expect(z.count).toBe(12);
        }
      }
      // Partners sit opposite: seat i and seat i+2 share a side.
      const sides = (state.view.seats ?? []).map((s) => s.side);
      expect(sides).toEqual(['0', '1', '0', '1']);
    }
    expect(asked).toHaveLength(1);
  });

  for (const [variation, seats, options, verbsPlayed] of [
    ['partnership', 4, { target: 1000, passCards: 3 }, ['bid', 'pass', 'name_trump', 'pass_cards', 'play_card']],
    ['cutthroat', 3, { target: 1000 }, ['bid', 'pass', 'name_trump', 'bury', 'play_card']],
    ['twohanded', 2, { target: 1000 }, ['play_card', 'meld', 'draw']],
    ['doubledeck', 4, { target: 1000 }, ['bid', 'pass', 'name_trump', 'play_card']],
  ] as const) {
    test(`a whole ${variation} match plays out over real WebSockets`, async ({ page, request }) => {
      test.setTimeout(420_000);
      const { matchId, users } = await startMatch(request, seats, variation, { ...options, pauseBetweenRounds: 0 });
      const wsBase = API_BASE.replace(/^http/, 'ws');

      const result = await page.evaluate(
        async ({ wsBase, matchId, tokens, ids }) => {
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
          const seatsOpen = await Promise.all(tokens.map(open));
          for (let i = 0; i < 200 && !seatsOpen.every(latest); i++) await new Promise((r) => setTimeout(r, 50));

          const submissionFor = (o: any) => {
            const action: any = { offerId: o.id, verb: o.verb };
            const need = o.source?.minCards ?? 0;
            if (o.source?.submit?.length) action.cards = o.source.submit;
            else if (need > 0) {
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
          // The first caller of each auction opens at the minimum and everyone
          // else passes, so every hand has a bid and the auction is short.
          // A two-handed winner melds when it can.
          const order = ['play_card', 'bury', 'pass_cards', 'name_trump', 'meld', 'draw', 'bid', 'pass', 'continue'];

          const verbs: Record<string, number> = {};
          const errors: string[] = [];
          let arrangedOk = true;
          let duplicatesSeen = false;
          let meldSeen = false;
          let moves = 0;
          for (let step = 0; step < 8000; step++) {
            const idx = seatsOpen.findIndex((s) => (latest(s)?.legalActions ?? []).some((o: any) => o.enabled));
            if (idx === -1) break;
            const state = latest(seatsOpen[idx]);
            if (state.status !== 'active') break;

            const trick = state.view.zones.find((z: any) => z.arrange === 'bySeat');
            if (trick?.cards?.length && !trick.cards.every((c: any) => ids.includes(c.by))) arrangedOk = false;
            const hand = state.view.zones.find((z: any) => z.kind === 'hand' && z.cards?.length);
            if (hand) {
              const codes = hand.cards.map((c: any) => c.card);
              if (new Set(codes).size < codes.length) duplicatesSeen = true;
            }
            if (state.view.zones.some((z: any) => z.id.startsWith('meld:') && z.groups?.length)) meldSeen = true;

            const live = state.legalActions.filter((o: any) => o.enabled && order.includes(o.verb));
            const bidOffer = live.find((o: any) => o.verb === 'bid');
            const standing = bidOffer?.facts?.some((f: any) => f.labelKey === 'pinochle.fact.standingBid');
            const enabled = live.filter((o: any) => !(o.verb === 'bid' && standing));
            enabled.sort((a: any, b: any) => order.indexOf(a.verb) - order.indexOf(b.verb));
            const action = enabled.map(submissionFor).find(Boolean);
            if (!action) break;

            const before = seatsOpen.map((s) => s.inbox.length);
            seatsOpen[idx].ws.send(JSON.stringify(action));
            verbs[action.verb] = (verbs[action.verb] ?? 0) + 1;
            moves++;
            for (let i = 0; i < 200; i++) {
              if (seatsOpen.every((s, k) => s.inbox.length > before[k])) break;
              await new Promise((r) => setTimeout(r, 15));
            }
            for (const s of seatsOpen) {
              const last = s.inbox[s.inbox.length - 1];
              if (last?.type === 'error') errors.push(`${last.code}: ${last.message}`);
            }
          }
          const final = seatsOpen.map(latest).find(Boolean) ?? {};
          for (const s of seatsOpen) s.ws.close();
          return { moves, verbs, errors, arrangedOk, duplicatesSeen, meldSeen, status: final.status };
        },
        { wsBase, matchId, tokens: users.map((u) => u.accessToken), ids: users.map((u) => u.userId) },
      );

      expect(result.errors, `socket errors: ${result.errors.join('; ')}`).toEqual([]);
      expect(result.status).toBe('completed');
      expect(result.arrangedOk).toBe(true);
      expect(result.duplicatesSeen, 'a pack with every card twice should deal some hand a pair').toBe(true);
      expect(result.meldSeen, 'some meld should have been laid on the table').toBe(true);
      for (const verb of verbsPlayed) {
        expect(result.verbs[verb] ?? 0, `${verb} was never played`).toBeGreaterThan(0);
      }

      // Recorded: a round log whose last totals are the standings, and in
      // which partners always share one score.
      const persisted: any = await stateFor(request, matchId, users[0]);
      expect(persisted.status).toBe('completed');
      const rounds = persisted.rounds?.rounds ?? [];
      expect(rounds.length).toBeGreaterThan(0);
      const last = rounds[rounds.length - 1];
      const top = Math.max(...last.scores.map((s: { total: number }) => s.total));
      expect(top).toBeGreaterThanOrEqual(1000);
      if (seats === 4) {
        for (const r of rounds) {
          const t = r.scores.map((s: { total: number }) => s.total);
          expect(t[0], `round ${r.number}: partners disagree`).toBe(t[2]);
          expect(t[1], `round ${r.number}: partners disagree`).toBe(t[3]);
        }
      }
    });
  }

  test('the web table bids, names trumps, shows the meld and plays a card', async ({ page, request }) => {
    test.setTimeout(240_000);
    // Everyone else is a bot; the human is asked to bid sooner or later, and
    // a minimum the bots will not go past makes the human's opening bid win.
    const { matchId, host } = await tableWithBots(request, 4, 'partnership', { minBid: 600, allPass: 1 });
    await signIn(page, host);
    await page.goto(`/match/${matchId}`);
    await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
    await expect(page.getByTestId('match-module')).toHaveAttribute('data-module', 'pinochle');

    // Wait for the auction to reach the human (a redeal may come first).
    const bid = page.getByTestId('offer-bid');
    await expect(bid).toBeVisible({ timeout: 60_000 });
    await expect(bid).not.toHaveAttribute('aria-disabled', 'true', { timeout: 90_000 });
    await expect(bid).toContainText('600');
    await bid.click();

    // The bots pass, so the human names trumps.
    const trump = page.getByTestId('offer-trump');
    await expect(trump).toBeVisible({ timeout: 60_000 });
    await expect(trump).not.toHaveAttribute('aria-disabled', 'true', { timeout: 60_000 });
    await trump.click();
    // A choice is asked for; take whichever the sheet offers first.
    const choice = page.locator('[data-testid^="choice-"]').first();
    if (await choice.isVisible({ timeout: 5_000 }).catch(() => false)) await choice.click();

    // The meld is on the table and the human leads.
    await expect.poll(async () => {
      const s = await stateFor(request, matchId, host);
      return s.view.zones.filter((z) => z.id.startsWith('meld:')).length + (s.legalActions.find((o) => o.id === 'play')?.enabled ? 100 : 0);
    }, { timeout: 60_000 }).toBeGreaterThanOrEqual(100);
    const state = await stateFor(request, matchId, host);
    for (const z of state.view.zones.filter((z) => z.id.startsWith('meld:'))) {
      await expect(page.getByTestId(`zone-${z.id}`)).toBeVisible();
    }
    // Trumps are drawn in the header as the suit itself, the one the server named.
    const trumpFact = (state.view as any).header.find((f: any) => f.labelKey === 'pinochle.header.trump');
    const trumpSuit = String(trumpFact.value).replace('suit.', '');
    const pill = page.getByTestId(`suit-in-play-${trumpSuit}`);
    await expect(pill).toBeVisible();
    await expect(pill).toContainText(/\S/);

    // Select the first legal card — by its code, since the pack holds two
    // of it — press Play, and confirm the server moved.
    const legal = state.legalActions.find((o) => o.id === 'play')!.source!.cards!;
    const handBefore = state.view.zones.find((z) => z.ownerId === host.userId && z.kind === 'hand')!.count;
    await selectOnly(page, [legal[0]]);
    const playButton = page.getByTestId('offer-play');
    await expect(playButton).not.toHaveAttribute('aria-disabled', 'true');
    await playButton.click();
    await expect.poll(async () => {
      const s = await stateFor(request, matchId, host);
      return s.view.zones.find((z) => z.ownerId === host.userId && z.kind === 'hand')!.count;
    }, { timeout: 30_000 }).toBe(handBefore - 1);
    await expect(page.getByTestId(`zone-trick`)).toBeVisible();
  });
});
