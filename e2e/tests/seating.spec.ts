import { expect, test } from '@playwright/test';

import { API_BASE, WEB_BASE } from '../helpers/env';

/**
 * End-to-end for arranging a table before it is dealt.
 *
 * "Form teams" is one mechanism, not two: in a game with sides the turn
 * alternates between them, so a side is a position in the seating and
 * reordering the seats is the whole of choosing a partner. What these check is
 * that the arrangement survives the round trip — through HTTP, through the
 * database, and into the deal the module actually makes.
 */

type Ctx = Parameters<Parameters<typeof test>[1]>[0]['request'];

async function guest(request: Ctx, tag: string) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `seat-${tag}-${Math.random().toString(36).slice(2, 8)}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

/** A Samba lobby with `seats` real players, host first, not yet dealt. */
async function lobby(request: Ctx, seats: number) {
  const users = [];
  for (let i = 0; i < seats; i++) users.push(await guest(request, String(i)));
  const host = users[0];
  const auth = { Authorization: `Bearer ${host.accessToken}` };

  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    data: { moduleId: 'canasta', variation: 'samba', options: { targetScore: 1000 } },
  });
  expect(created.ok(), await created.text()).toBeTruthy();
  const { matchId } = await created.json();

  for (const u of users.slice(1)) {
    const joined = await request.post(`${API_BASE}/matches/${matchId}/join`, {
      headers: { Authorization: `Bearer ${u.accessToken}` },
    });
    expect(joined.ok(), await joined.text()).toBeTruthy();
  }
  return { matchId, users, auth };
}

const stateOf = async (request: Ctx, matchId: string) =>
  (await request.get(`${API_BASE}/matches/${matchId}`)).json();

test.describe('arranging the table', () => {
  test('a lobby says who is playing with whom, and the deal agrees', async ({ request }) => {
    const { matchId, users } = await lobby(request, 4);
    const ids = users.map((u) => u.userId);

    // Before anyone rearranges anything: partners sit opposite, so the sides
    // are seats 0+2 and 1+3.
    const before = await stateOf(request, matchId);
    expect(before.sides, 'a partnership game should say what the sides are').toEqual([
      [ids[0], ids[2]],
      [ids[1], ids[3]],
    ]);
  });

  test('moving a seat changes who your partner is', async ({ request }) => {
    const { matchId, users, auth } = await lobby(request, 4);
    const ids = users.map((u) => u.userId);

    // The host wants the fourth player as a partner. Partners sit opposite, so
    // that seat swaps with the third — and this is the whole feature: saying
    // where somebody sits *is* saying who they play with.
    const wanted = [ids[0], ids[1], ids[3], ids[2]];
    const res = await request.post(`${API_BASE}/matches/${matchId}/seats`, {
      headers: auth,
      data: { order: wanted },
    });
    expect(res.ok(), await res.text()).toBeTruthy();
    expect((await res.json()).sides).toEqual([
      [ids[0], ids[3]],
      [ids[1], ids[2]],
    ]);

    // Read back cold, so this is the round trip through the database rather
    // than the handler's own answer.
    const after = await stateOf(request, matchId);
    expect(after.players.map((p: { id: string }) => p.id)).toEqual(wanted);
    expect(after.sides).toEqual([
      [ids[0], ids[3]],
      [ids[1], ids[2]],
    ]);

    // And the deal honours it: the seat order the host arranged is the order
    // the hands are dealt in.
    const started = await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });
    expect(started.ok(), await started.text()).toBeTruthy();

    const dealt = await stateOf(request, matchId);
    expect(dealt.status).toBe('active');
    // Sides are a lobby fact; once dealt, the board carries the partnerships.
    expect(dealt.sides).toBeFalsy();
    const seats = dealt.view.zones
      .filter((z: { kind: string; ownerId?: string }) => z.kind === 'hand' && z.ownerId)
      .map((z: { ownerId: string }) => z.ownerId);
    // The viewer's own hand comes first in their view, so compare as a set of
    // the same four seats rather than by position.
    expect(new Set(seats)).toEqual(new Set(wanted));
  });

  test('an order that is not a permutation of the table is refused', async ({ request }) => {
    const { matchId, users, auth } = await lobby(request, 4);
    const ids = users.map((u) => u.userId);

    for (const [label, order] of [
      ['a seat left out', [ids[0], ids[1], ids[2]]],
      ['somebody twice', [ids[0], ids[1], ids[2], ids[2]]],
      ['a stranger', [ids[0], ids[1], ids[2], 'nobody']],
    ] as [string, string[]][]) {
      const res = await request.post(`${API_BASE}/matches/${matchId}/seats`, {
        headers: auth,
        data: { order },
      });
      expect(res.ok(), `${label} should be refused`).toBeFalsy();
      expect((await res.json()).code).toBe('BAD_SEATING');
    }

    // Refused, not half-applied.
    const after = await stateOf(request, matchId);
    expect(after.players.map((p: { id: string }) => p.id)).toEqual(ids);
  });

  test('only the host arranges, and only before the deal', async ({ request }) => {
    const { matchId, users, auth } = await lobby(request, 4);
    const ids = users.map((u) => u.userId);
    const order = [ids[0], ids[1], ids[3], ids[2]];

    const asPlayer = await request.post(`${API_BASE}/matches/${matchId}/seats`, {
      headers: { Authorization: `Bearer ${users[1].accessToken}` },
      data: { order },
    });
    expect(asPlayer.ok(), 'a player who is not the host should not reseat the table').toBeFalsy();
    expect((await asPlayer.json()).code).toBe('NOT_THE_HOST');

    expect((await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth })).ok()).toBeTruthy();

    // Afterwards the seating is what the hands were dealt against, so moving it
    // would reassign cards already held.
    const afterDeal = await request.post(`${API_BASE}/matches/${matchId}/seats`, {
      headers: auth,
      data: { order },
    });
    expect(afterDeal.ok(), 'a dealt table should not be reseated').toBeFalsy();
    expect((await afterDeal.json()).code).toBe('MATCH_ALREADY_STARTED');
  });

  test('a game where everybody plays for themselves offers no sides', async ({ request }) => {
    // Prší has no partnerships, so there is nothing to preview and the lobby
    // should say so by omission rather than by inventing a team per seat.
    const host = await guest(request, 'prsi');
    const auth = { Authorization: `Bearer ${host.accessToken}` };
    const created = await request.post(`${API_BASE}/matches`, {
      headers: auth,
      data: { moduleId: 'prsi' },
    });
    expect(created.ok(), await created.text()).toBeTruthy();
    const { matchId } = await created.json();

    const other = await guest(request, 'prsi2');
    await request.post(`${API_BASE}/matches/${matchId}/join`, {
      headers: { Authorization: `Bearer ${other.accessToken}` },
    });

    expect((await stateOf(request, matchId)).sides).toBeFalsy();
  });

  // The one check here that opens a browser, because it is the one thing the
  // API cannot show: `sides` is a lobby field and disappears at the deal, so
  // everything above stops watching exactly where a player starts caring.
  // Somebody who joined a table they did not arrange, or came back to one a
  // day later, could read the whole board and not know who they were playing
  // with.
  test('the board keeps saying who your partner is', async ({ page, request }) => {
    const { matchId, users, auth } = await lobby(request, 4);
    const ids = users.map((u) => u.userId);
    expect((await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth })).ok()).toBeTruthy();

    const dealt = await stateOf(request, matchId);
    const nameOf = (id: string) =>
      dealt.players.find((p: { id: string; name: string }) => p.id === id).name;

    await page.addInitScript((session) => {
      window.localStorage.setItem('zolik_session', JSON.stringify(session));
    }, {
      accessToken: users[0].accessToken,
      refreshToken: users[0].refreshToken,
      userId: ids[0],
      username: nameOf(ids[0]),
      isGuest: true,
    });
    await page.goto(`${WEB_BASE}/match/${matchId}`);
    await expect(page.getByTestId('seat-strip')).toBeVisible();

    // Partners sit opposite, so seat 2 is the host's. Their tile says so in
    // the second person; the host's own names the partner.
    await expect(page.getByTestId(`seat-team-${ids[2]}`)).toHaveText('Your team');
    await expect(page.getByTestId(`seat-team-${ids[0]}`)).toHaveText(`Team: ${nameOf(ids[2])}`);

    // The opponents are a pair too, and saying so is the difference between a
    // board with sides on it and a board that only marks the viewer's.
    await expect(page.getByTestId(`seat-team-${ids[1]}`)).toHaveText(`Team: ${nameOf(ids[3])}`);
    await expect(page.getByTestId(`seat-team-${ids[3]}`)).toHaveText(`Team: ${nameOf(ids[1])}`);
  });

  test('a board where everybody plays for themselves marks no teams', async ({ page, request }) => {
    const { matchId, users, auth } = await lobby(request, 2);
    expect((await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth })).ok()).toBeTruthy();

    await page.addInitScript((session) => {
      window.localStorage.setItem('zolik_session', JSON.stringify(session));
    }, {
      accessToken: users[0].accessToken,
      refreshToken: users[0].refreshToken,
      userId: users[0].userId,
      username: 'solo',
      isGuest: true,
    });
    await page.goto(`${WEB_BASE}/match/${matchId}`);
    await expect(page.getByTestId('seat-strip')).toBeVisible();
    // Heads-up Canasta is two sides of one, and a partnership badge on a seat
    // with no partner is noise wearing the clothes of information.
    await expect(page.locator('[data-testid^="seat-team-"]')).toHaveCount(0);
  });
});
