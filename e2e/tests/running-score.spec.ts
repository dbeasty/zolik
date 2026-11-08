import { expect, test, type Page } from '@playwright/test';

import { API_BASE } from '../helpers/env';

/**
 * The running score: a trick game shows every seat's points while the hand is
 * still being played, not only once it has been settled.
 *
 * The Go suites pin the facts each module sends (running_score_test.go and
 * the view tests). What only this proves is that they reach the board: a
 * table of real players is driven a few tricks into a hand over WebSockets,
 * then the first player opens it in the web shell and every seat tile has to
 * carry its number.
 */

type Ctx = import('@playwright/test').APIRequestContext;
type User = { userId: string; accessToken: string; refreshToken: string; username?: string };

async function guest(request: Ctx): Promise<User> {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `score-${Math.random().toString(36).slice(2, 10)}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

async function startMatch(request: Ctx, moduleId: string, seats: number, variation: string, options: Record<string, number>) {
  const users: User[] = [];
  for (let i = 0; i < seats; i++) users.push(await guest(request));
  const auth = { Authorization: `Bearer ${users[0].accessToken}` };
  const created = await request.post(`${API_BASE}/matches`, { headers: auth, data: { moduleId, variation, options } });
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

async function signIn(page: Page, user: User) {
  await page.addInitScript((s) => {
    window.localStorage.setItem('zolik_session', JSON.stringify(s));
  }, {
    accessToken: user.accessToken,
    refreshToken: user.refreshToken,
    userId: user.userId,
    username: user.username ?? 'score',
    isGuest: true,
  });
}

const games = [
  // Sedma's leader may carry a trick on; ending it promptly gets tricks taken.
  { moduleId: 'sedma', seats: 4, variation: '', key: 'sedma.seat.points', text: /\d+ points/, order: ['end_trick', 'play_card', 'continue'] },
  { moduleId: 'marias', seats: 3, variation: '', key: 'marias.seat.points', text: /\d+ points/, order: ['play_card', 'discard', 'choose_trump', 'announce', 'good', 'pass', 'continue'] },
  { moduleId: 'pinochle', seats: 4, variation: 'partnership', key: 'pinochle.seat.taken', text: /Taken \d+/, order: ['play_card', 'bury', 'pass_cards', 'name_trump', 'meld', 'draw', 'bid', 'pass', 'continue'] },
] as const;

test.describe('running score', () => {
  for (const g of games) {
    test(`${g.moduleId} shows every seat's points mid-hand`, async ({ page, request }) => {
      test.setTimeout(180_000);
      const { matchId, users } = await startMatch(request, g.moduleId, g.seats, g.variation, { pauseBetweenRounds: 0 });
      const wsBase = API_BASE.replace(/^http/, 'ws');

      // Play until a few cards have gone and every playing seat carries the
      // running score, then stop: the board is left mid-hand.
      const scored: string[] = await page.evaluate(
        async ({ wsBase, matchId, tokens, key, order, seats }) => {
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
          const all = await Promise.all(tokens.map(open));
          for (let i = 0; i < 200 && !all.every(latest); i++) await new Promise((r) => setTimeout(r, 50));
          let played = 0;
          const withScore = (state: any) =>
            (state?.view?.seats ?? []).filter((s: any) => (s.facts ?? []).some((f: any) => f.labelKey === key)).map((s: any) => s.playerId);
          try {
            for (let step = 0; step < 2000; step++) {
              const idx = all.findIndex((s) => (latest(s)?.legalActions ?? []).some((o: any) => o.enabled));
              if (idx === -1) return [];
              const state = latest(all[idx]);
              if (state.status !== 'active') return [];
              // Two rounds of cards played, so at least one trick is in.
              const shown = withScore(latest(all[0]));
              if (played >= seats * 2 && shown.length >= seats - 1) return shown;

              const live = state.legalActions.filter((o: any) => o.enabled && order.includes(o.verb));
              const bidOffer = live.find((o: any) => o.verb === 'bid');
              const standing = bidOffer?.facts?.some((f: any) => f.labelKey === 'pinochle.fact.standingBid');
              const enabled = live.filter((o: any) => !(o.verb === 'bid' && standing));
              enabled.sort((a: any, b: any) => order.indexOf(a.verb) - order.indexOf(b.verb));
              const action = enabled.map(submissionFor).find(Boolean);
              if (!action) return [];
              const before = all.map((s) => s.inbox.length);
              all[idx].ws.send(JSON.stringify(action));
              if (action.verb === 'play_card') played++;
              for (let i = 0; i < 200; i++) {
                if (all.every((s, k) => s.inbox.length > before[k])) break;
                await new Promise((r) => setTimeout(r, 15));
              }
            }
            return [];
          } finally {
            for (const s of all) s.ws.close();
          }
        },
        { wsBase, matchId, tokens: users.map((u) => u.accessToken), key: g.key, order: [...g.order], seats: g.seats },
      );
      expect(scored.length, `no seat carried ${g.key} mid-hand`).toBeGreaterThanOrEqual(g.seats - 1);

      await signIn(page, users[0]);
      await page.goto(`/match/${matchId}`);
      await expect(page.getByTestId('match-screen')).toBeVisible({ timeout: 30_000 });
      await expect(page.getByTestId('match-module')).toHaveAttribute('data-module', g.moduleId);
      // Every seat that plays this hand shows its number on its own tile —
      // the other side's as well as the viewer's.
      for (const id of scored) {
        await expect(page.getByTestId(`seat-${id}`)).toContainText(g.text, { timeout: 15_000 });
      }
      expect(scored).toContain(users[1].userId);
    });
  }
});
