import { expect, test, type Page } from '@playwright/test';

import { API_BASE } from '../helpers/env';
import { selectOnly } from '../helpers/hand';
import { seedIntroSeen } from '../helpers/login';

/**
 * The status box between the board and the hand, end to end, for the games
 * whose moves chain: what each move did must reach the player who is about to
 * act, in words, before they act.
 *
 * Three real seats. B and C are driven over real WebSockets; A plays in a real
 * browser, through the controls on her screen — the server keeps one live
 * connection a seat, so A's moves are made the way a player makes them. After every move B or C makes, the box
 * on A's screen must say that move, with what it did: a Draw Two must say who
 * draws two and misses a turn, a Reverse which way play now goes, a wild the
 * colour it named. The driver plays the action cards first, so every one of
 * them comes up; a match that ends before they all have is dealt again.
 *
 * Every expectation is the English line a player reads, built from the move
 * that was sent — not from anything the server sent back.
 */

type Ctx = import('@playwright/test').APIRequestContext;
type User = { userId: string; accessToken: string; refreshToken: string; guestId: string; guestKey: string; name: string };

async function guest(request: Ctx, name: string): Promise<User> {
  const res = await request.post(`${API_BASE}/auth/guest`, { data: { guestName: name } });
  expect(res.ok(), await res.text()).toBeTruthy();
  const g = await res.json();
  return { ...g, name: g.guestName ?? name };
}

async function newTable(request: Ctx, moduleId: string, users: User[], options: Record<string, number>) {
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
class Seat {
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
  }
  close() {
    this.ws?.close();
  }
}

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

/** Opens A's own view of the match in the browser. */
async function watchAs(page: Page, user: User, matchId: string) {
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
async function move(seats: Seat[], seat: Seat, action: any) {
  const sockets = seats.filter((s) => !s.page);
  const before = sockets.map((s) => s.received);
  await seat.send(action);
  for (let i = 0; i < 250 && sockets.some((s, k) => s.received <= before[k]); i++) await sleep(20);
  expect(seat.errors, `the server refused ${JSON.stringify(action)}`).toEqual([]);
  for (const s of seats) await s.refresh();
}

/** The box sits under the pile and over the hand: where the eye goes. */
async function expectBoxBetweenTableAndHand(page: Page, viewerId: string) {
  const pile = await page.locator('[data-testid^="card-discard-"]:not([data-testid*="concealed"])').last().boundingBox();
  const box = await page.getByTestId('move-announcements').boundingBox();
  const hand = await page.getByTestId(`zone-hand:${viewerId}`).boundingBox();
  expect(pile && box && hand, 'pile, box and hand all on screen').toBeTruthy();
  expect(box!.y, 'the box is below the pile').toBeGreaterThanOrEqual(pile!.y + pile!.height - 1);
  expect(box!.y + box!.height, 'the box is above the hand').toBeLessThanOrEqual(hand!.y + 1);
}

// ---------------------------------------------------------------------------
// Last Card
// ---------------------------------------------------------------------------

const SHAPE: Record<string, string> = { C: '●', T: '◆', V: '▲', A: '■' };
const COLOUR: Record<string, string> = { C: '● Coral', T: '◆ Teal', V: '▲ Violet', A: '■ Amber' };
const ACTION: Record<string, string> = { S: 'Skip', R: 'Reverse', D: 'Draw Two' };

/** A Last Card code as a sentence names it: "●7", "◆ Skip", "Wild Draw Four". */
function lastCardText(code: string) {
  if (code === 'W') return 'Wild';
  if (code === 'W4') return 'Wild Draw Four';
  const [c, f] = code.split('-');
  return ACTION[f] ? `${SHAPE[c]} ${ACTION[f]}` : `${SHAPE[c]}${f}`;
}

/** The order the driver plays cards in: every kind of action first. */
function lastCardRank(code: string) {
  if (code.endsWith('-D')) return 0;
  if (code.endsWith('-S')) return 1;
  if (code.endsWith('-R')) return 2;
  if (code === 'W4') return 3;
  if (code === 'W') return 4;
  return 5;
}

test('every Last Card move is said, with what it did, between the pile and the hand', async ({ page, request }) => {
  test.setTimeout(420_000);
  const users = [await guest(request, 'Ada'), await guest(request, 'Bo'), await guest(request, 'Cy')];
  const [A] = users;
  const name = (id: string) => users.find((u) => u.userId === id)?.name ?? id;
  // Every line the strip can say about the others' moves, by what they did.
  const seen = new Set<string>();
  const required = ['drawTwo', 'skip', 'reverse', 'wild', 'drawFour', 'accepted', 'drew', 'called', 'caught', 'played', 'wildShowsColour'];

  for (let table = 0; table < 6 && required.some((r) => !seen.has(r)); table++) {
    const matchId = await newTable(request, 'lastcard', users, {
      targetScore: 0,
      lastCardCall: 1,
      drawFourChallenge: 1,
      stacking: 0,
      pauseBetweenRounds: 0,
    });
    await watchAs(page, A, matchId);
    const seats = users.map((u) => (u === A ? new Seat(matchId, u, page, request) : new Seat(matchId, u)));
    await Promise.all(seats.map((s) => s.ready()));

    for (let step = 0; step < 600; step++) {
      const seat = seats.find((s) => (s.latest?.legalActions ?? []).some((o: any) => o.enabled));
      if (!seat || seat.latest.status !== 'active') {
        break;
      }
      const me = seat.user;
      const watched = me.userId !== A.userId;
      const order: string[] = seat.latest.view.seats.map((x: any) => x.playerId);
      const header = seat.latest.view.header ?? [];
      const anticlockwise = header.some((f: any) => f.labelKey === 'lastcard.header.anticlockwise');
      const nextOf = (id: string, dir = anticlockwise ? -1 : 1) =>
        order[(((order.indexOf(id) + dir) % order.length) + order.length) % order.length];

      // Choose: answer what is asked, then call (Bo never does, so he can be
      // caught), then the action cards first.
      let action: any;
      let expected: { key: string; text: RegExp } | undefined;
      const call = seat.enabled('call');
      const play = seat.enabled('play_card');
      if (seat.enabled('accept')) {
        action = { offerId: 'accept', verb: 'accept' };
        expected = { key: 'accepted', text: new RegExp(`${me.name} took 4 cards and missed a turn`) };
      } else if (seat.enabled('catch')) {
        action = { offerId: 'catch', verb: 'catch' };
        expected = { key: 'caught', text: new RegExp(`${me.name} caught \\w+ without a “Last card!” — \\w+ draws 2`) };
      } else if (call && me.name !== 'Bo') {
        action = { offerId: 'call', verb: 'call' };
        expected = { key: 'called', text: new RegExp(`${me.name}: “Last card!”`) };
      } else if (play) {
        const cards: string[] = [...play.source.cards].sort((x: string, y: string) => lastCardRank(x) - lastCardRank(y));
        const card = cards[0];
        action = { offerId: 'play_card', verb: 'play_card', cards: [card] };
        const colour = play.params?.[0]?.defaultChoice ?? play.params?.[0]?.choices?.[0]?.value;
        if (card === 'W' || card === 'W4') action.params = { colour };
        const said = `${me.name} played ${lastCardText(card)}`;
        const next = nextOf(me.userId);
        const lastPlay = seat.latest.view.zones.find((z: any) => z.ownerId === me.userId)?.count === 1;
        if (lastPlay) {
          expected = undefined; // the deal ends; "went out" is checked below
        } else if (card.endsWith('-D')) {
          expected = { key: 'drawTwo', text: new RegExp(`${said} — ${name(next)} draws 2 and misses a turn`) };
        } else if (card.endsWith('-S') || (card.endsWith('-R') && order.length === 2)) {
          expected = { key: 'skip', text: new RegExp(`${said} — ${name(next)} misses a turn`) };
        } else if (card.endsWith('-R')) {
          const way = anticlockwise ? 'clockwise ↻' : 'anticlockwise ↺';
          expected = { key: 'reverse', text: new RegExp(`${said} — play now goes ${way}`) };
        } else if (card === 'W4') {
          expected = {
            key: 'drawFour',
            text: new RegExp(`${said} — the colour is now ${COLOUR[colour]}; ${name(next)} takes four or challenges`),
          };
        } else if (card === 'W') {
          expected = { key: 'wild', text: new RegExp(`${said} — the colour is now ${COLOUR[colour]}`) };
        } else {
          expected = { key: 'played', text: new RegExp(`${said}$`, 'm') };
        }
      } else if (seat.enabled('pass')) {
        action = { offerId: 'pass', verb: 'pass' };
        expected = { key: 'kept', text: new RegExp(`${me.name} kept the card they drew`) };
      } else {
        action = { offerId: 'draw', verb: 'draw' };
        expected = { key: 'drew', text: new RegExp(`${me.name} drew a card|${me.name} had nothing left to draw`) };
      }

      await move(seats, seat, action);

      // A reads every other seat's move, in words, between the pile and the hand.
      if (watched && expected) {
        await expect(page.getByTestId('move-announcements'), `after ${me.name} ${JSON.stringify(action)}`).toContainText(
          expected.text,
          { timeout: 10_000 },
        );
        if (seen.size === 0) await expectBoxBetweenTableAndHand(page, A.userId);
        // A wild on the pile wears the colour it named, on the card itself.
        if ((expected.key === 'wild' || expected.key === 'drawFour') && action.params?.colour) {
          await expectWildShowsColour(page, action.params.colour);
          seen.add('wildShowsColour');
        }
        seen.add(expected.key);
      }
    }

    // The deal's end is said too.
    await expect(page.getByTestId('move-announcements')).toContainText(/went out and scores \d+/, { timeout: 10_000 });
    for (const s of seats) s.close();
  }

  console.log('checked on screen:', [...seen].sort().join(', '));
  expect(required.filter((r) => !seen.has(r)), 'kinds of move never seen and checked').toEqual([]);
});

/** Each colour's ink, as the card faces draw it (lastCardArt.ts INKS.main). */
const MAIN_INK: Record<string, string> = { C: '#e8603c', T: '#14a193', V: '#7a5bd8', A: '#e5a019' };

/**
 * The wild on top of the pile shows the colour it named: the other three
 * quarters of its wheel fade, and the named colour's shape sits at the hub.
 */
async function expectWildShowsColour(page: Page, colour: string) {
  const top = page.locator('[data-testid^="card-discard-"]:not([data-testid*="concealed"])').last();
  await expect
    .poll(
      async () =>
        top.evaluate((el, ink) => {
          const paths = [...el.querySelectorAll('path')];
          const fill = (p: Element) => (p.getAttribute('fill') ?? '').toLowerCase();
          const faded = paths.filter((p) => p.getAttribute('opacity') === '0.2' && fill(p) !== ink).length;
          const named = paths.some((p) => fill(p) === ink && p.getAttribute('opacity') !== '0.2');
          return faded >= 3 && named;
        }, MAIN_INK[colour]),
      { timeout: 10_000 },
    )
    .toBe(true);
}

// ---------------------------------------------------------------------------
// Prší
// ---------------------------------------------------------------------------

/** The order the driver plays Prší cards in: the cards that do something first. */
function prsiRank(code: string) {
  if (code.startsWith('7')) return 0;
  if (code.startsWith('A')) return 1;
  if (code.startsWith('Q')) return 2;
  return 3;
}

test('every Prší move is said, with what it asks of whom, between the pile and the hand', async ({ page, request }) => {
  test.setTimeout(420_000);
  const users = [await guest(request, 'Ada'), await guest(request, 'Bo'), await guest(request, 'Cy')];
  const [A] = users;
  const name = (id: string) => users.find((u) => u.userId === id)?.name ?? id;
  const seen = new Set<string>();
  const required = ['seven', 'ace', 'queen', 'took', 'missed', 'drew', 'played'];

  for (let table = 0; table < 12 && required.some((r) => !seen.has(r)); table++) {
    const matchId = await newTable(request, 'prsi', users, {});
    await watchAs(page, A, matchId);
    const seats = users.map((u) => (u === A ? new Seat(matchId, u, page, request) : new Seat(matchId, u)));
    await Promise.all(seats.map((s) => s.ready()));

    for (let step = 0; step < 400; step++) {
      const seat = seats.find((s) => (s.latest?.legalActions ?? []).some((o: any) => o.enabled));
      if (!seat || seat.latest.status !== 'active') break;
      const me = seat.user;
      const watched = me.userId !== A.userId;
      const order: string[] = seat.latest.view.seats.map((x: any) => x.playerId);
      const next = order[(order.indexOf(me.userId) + 1) % order.length];

      let action: any;
      let expected: { key: string; text: RegExp } | undefined;
      const play = seat.enabled('play_card');
      if (play) {
        const card = [...play.source.cards].sort((x: string, y: string) => prsiRank(x) - prsiRank(y))[0];
        action = { offerId: 'play_card', verb: 'play_card', cards: [card] };
        const suit = play.params?.[0]?.defaultChoice ?? play.params?.[0]?.choices?.[0]?.value;
        if (card.startsWith('Q')) action.params = { suit };
        const lastPlay = seat.latest.view.zones.find((z: any) => z.ownerId === me.userId)?.count === 1;
        const said = `${me.name} played`;
        if (lastPlay) expected = { key: 'played', text: new RegExp(`${said} .*, their last card, and wins`) };
        else if (card.startsWith('7'))
          expected = { key: 'seven', text: new RegExp(`${said} .* — ${name(next)} answers with a seven or takes \\d+`) };
        else if (card.startsWith('A'))
          expected = { key: 'ace', text: new RegExp(`${said} .* — ${name(next)} answers with an ace or misses a turn`) };
        else if (card.startsWith('Q')) expected = { key: 'queen', text: new RegExp(`${said} .* — the suit is now \\w+`) };
        else expected = { key: 'played', text: new RegExp(`${said} \\S+$`, 'm') };
      } else if (seat.enabled('pass')) {
        action = { offerId: 'pass', verb: 'pass' };
        expected = { key: 'missed', text: new RegExp(`${me.name} misses a turn`) };
      } else {
        const owed = (seat.latest.view.prompts ?? []).find((f: any) => f.labelKey === 'prompt.mustDrawOrAnswerSeven');
        action = { offerId: 'draw', verb: 'draw' };
        expected = owed
          ? { key: 'took', text: new RegExp(`${me.name} took \\d+ cards`) }
          : { key: 'drew', text: new RegExp(`${me.name} drew a card`) };
      }

      await move(seats, seat, action);
      if (watched && expected) {
        await expect(page.getByTestId('move-announcements'), `after ${me.name} ${JSON.stringify(action)}`).toContainText(
          expected.text,
          { timeout: 10_000 },
        );
        if (seen.size === 0) await expectBoxBetweenTableAndHand(page, A.userId);
        seen.add(expected.key);
      }
    }
    for (const s of seats) s.close();
  }
  console.log('checked on screen:', [...seen].sort().join(', '));
  expect(required.filter((r) => !seen.has(r)), 'kinds of move never seen and checked').toEqual([]);
});
