import { expect, type Page } from '@playwright/test';

import { API_BASE } from './env';
import { selectOnly } from './hand';
import { seedIntroSeen } from './login';

/**
 * Real seats at a real table, for specs that watch one seat in the browser
 * while the test plays the others: guests, a started table, a socket a seat,
 * and the watched seat playing through the controls on its own screen — the
 * server keeps one live connection a seat, so that is the only way it can.
 */

export type Ctx = import('@playwright/test').APIRequestContext;
export type User = { userId: string; accessToken: string; refreshToken: string; guestId: string; guestKey: string; name: string };

export async function guest(request: Ctx, name: string): Promise<User> {
  const res = await request.post(`${API_BASE}/auth/guest`, { data: { guestName: name } });
  expect(res.ok(), await res.text()).toBeTruthy();
  const g = await res.json();
  return { ...g, name: g.guestName ?? name };
}

export async function newTable(request: Ctx, moduleId: string, users: User[], options: Record<string, number>) {
  const auth = { Authorization: `Bearer ${users[0].accessToken}` };
  const created = await request.post(`${API_BASE}/matches`, { headers: auth, data: { moduleId, options } });
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
  return matchId as string;
}

/** One seat's socket, keeping the latest state the server sent it. */
export class Seat {
  latest: any = null;
  /** How many states this seat has received: a move has landed once every seat's has moved on. */
  received = 0;
  errors: string[] = [];
  private ws?: WebSocket;
  constructor(
    readonly matchId: string,
    readonly user: User,
    /** Played in the browser rather than over a socket of its own; see `refresh`. */
    readonly page?: Page,
    private request?: Ctx,
  ) {
    if (page) return;
    const base = API_BASE.replace(/^http/, 'ws');
    this.ws = new WebSocket(`${base}/ws/matches/${matchId}?token=${encodeURIComponent(user.accessToken)}`);
    this.ws.onmessage = (ev) => {
      const msg = JSON.parse(String(ev.data));
      if (msg.type === 'match_state') {
        this.latest = msg;
        this.received++;
      }
      if (msg.type === 'error') this.errors.push(`${msg.code}: ${msg.message}`);
    };
  }
  /** The browser seat's state, read over HTTP since its socket is the page's. */
  async refresh() {
    if (!this.page) return;
    const r = await this.request!.get(`${API_BASE}/matches/${this.matchId}`, {
      headers: { Authorization: `Bearer ${this.user.accessToken}` },
    });
    this.latest = await r.json();
  }
  async ready() {
    await this.refresh();
    for (let i = 0; i < 200 && !this.latest; i++) await sleep(50);
    expect(this.latest, `${this.user.name} never got a state`).toBeTruthy();
  }
  enabled(id: string) {
    return (this.latest?.legalActions ?? []).find((o: any) => o.id === id && o.enabled);
  }
  async send(action: any) {
    if (!this.page) {
      this.ws!.send(JSON.stringify(action));
      return;
    }
    // The way a player does it: pick the card, press the control.
    if (action.cards?.length) await selectOnly(this.page, action.cards);
    await this.page.getByTestId(`offer-${action.offerId}`).click();
    // A card that asks a question — the colour a wild names — asks it now;
    // answer as the move meant to.
    for (const [name, value] of Object.entries((action.params ?? {}) as Record<string, string>)) {
      const sheet = this.page.getByTestId(`choice-${name}`);
      if (await sheet.isVisible({ timeout: 1500 }).catch(() => false)) {
        await this.page.getByTestId(`choice-${name}-${value}`).click();
      }
    }
  }
  close() {
    this.ws?.close();
  }
}

export const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

/** Opens A's own view of the match in the browser. */
export async function watchAs(page: Page, user: User, matchId: string) {
  await page.addInitScript((s) => {
    window.localStorage.setItem('zolik_session', JSON.stringify(s));
  }, {
    accessToken: user.accessToken,
    refreshToken: user.refreshToken,
    userId: user.userId,
    username: user.name,
    isGuest: true,
    guestId: user.guestId,
    guestKey: user.guestKey,
    claimableMatches: 0,
  });
  await seedIntroSeen(page);
  await page.goto(`/match/${matchId}`);
  await expect(page.locator('[data-testid^="card-hand:"]').first()).toBeVisible({ timeout: 30_000 });
}

/** Sends one move and waits until every socket seat has the state after it. */
export async function move(seats: Seat[], seat: Seat, action: any) {
  const sockets = seats.filter((s) => !s.page);
  const before = sockets.map((s) => s.received);
  await seat.send(action);
  for (let i = 0; i < 250 && sockets.some((s, k) => s.received <= before[k]); i++) await sleep(20);
  expect(seat.errors, `the server refused ${JSON.stringify(action)}`).toEqual([]);
  for (const s of seats) await s.refresh();
}

