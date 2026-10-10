import { expect, test } from '@playwright/test';

import { API_BASE } from '../helpers/env';

/**
 * What a person may be called, and what they may sign in with.
 *
 * A name is signed into the token, listed in the lobby and drawn at every table
 * its owner sits down at, so what the server accepts is what everyone else is
 * shown. These pin the three ways it used to go wrong: a name of nothing but
 * spaces or zero-width characters (a seat with nothing written on it), a
 * right-to-left override that reverses what is drawn after it, and a ten-
 * thousand-character name carried in every token and every seat list. And the
 * one that was an outage rather than an oddity: a passphrase longer than 72
 * bytes was an `internal server error` at registration.
 */

type Ctx = import('@playwright/test').APIRequestContext;

const rand = () => Math.random().toString(36).slice(2, 9);

async function guestNamed(request: Ctx, guestName: string) {
  const res = await request.post(`${API_BASE}/auth/guest`, { data: { guestName } });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

const INVISIBLE = /[​⁠﻿‪-‮⁦-⁩\u0000-\u001f]/;

test.describe('guest names', () => {
  test('a name is cleaned before anyone else is shown it', async ({ request }) => {
    const g = await guestNamed(request, `  ‮Bob   the\u0000Builder ${rand()} `);
    expect(g.guestName).not.toMatch(INVISIBLE);
    expect(g.guestName).toMatch(/^Bob theBuilder \w+$/);
  });

  test('a name that would show as nothing is replaced by a visible one', async ({ request }) => {
    for (const blank of ['   ', '​​​', '‮', '⁠﻿']) {
      const g = await guestNamed(request, blank);
      expect(g.guestName.trim().length, `name for ${JSON.stringify(blank)}`).toBeGreaterThan(0);
      expect(g.guestName).not.toMatch(INVISIBLE);
    }
  });

  test('a ten-thousand-character name is cut to what a seat can hold', async ({ request }) => {
    const g = await guestNamed(request, 'x'.repeat(10_000));
    expect([...g.guestName].length).toBeLessThanOrEqual(24);
    // The token carries the name, so a cut name means a token that is small.
    expect(g.accessToken.length).toBeLessThan(2_000);
  });

  test('a cleaned name is what the other player is shown at the table', async ({ request }) => {
    const host = await guestNamed(request, '‮eliv\u0000l');
    const created = await request.post(`${API_BASE}/matches`, {
      headers: { Authorization: `Bearer ${host.accessToken}` },
      data: { moduleId: 'prsi', options: {} },
    });
    const { matchId } = await created.json();
    const state = await (await request.get(`${API_BASE}/matches/${matchId}`)).json();
    const names: string[] = state.players.map((p: any) => p.name);
    expect(names.length).toBeGreaterThan(0);
    for (const n of names) expect(n).not.toMatch(INVISIBLE);
  });
});

test.describe('username/password accounts', () => {
  const register = (request: Ctx, username: string, password: string) =>
    request.post(`${API_BASE}/auth/register`, { data: { username, password } });
  const login = (request: Ctx, username: string, password: string) =>
    request.post(`${API_BASE}/auth/login`, { data: { username, password } });

  test('a long passphrase registers and every byte of it counts at sign-in', async ({ request }) => {
    const name = `long-${rand()}`;
    const phrase = 'correct horse battery staple '.repeat(5);
    const reg = await register(request, name, phrase);
    expect(reg.status(), await reg.text()).toBe(200);
    expect((await login(request, name, phrase)).status()).toBe(200);
    expect((await login(request, name, phrase + '!')).status()).toBe(401);
  });

  test('an absurdly long password is refused with a 400, not a 500', async ({ request }) => {
    const res = await register(request, `big-${rand()}`, 'p'.repeat(5_000));
    expect(res.status()).toBe(400);
  });

  test('a username of nothing visible is refused', async ({ request }) => {
    for (const blank of ['', '   ', '​​', '‮']) {
      const res = await register(request, blank, 'secretpass1');
      expect(res.status(), `username ${JSON.stringify(blank)}`).toBe(400);
    }
  });

  test('"name " is not a second account beside "name", and signing in forgives stray spaces', async ({ request }) => {
    const name = `ann-${rand()}`;
    expect((await register(request, name, 'secretpass1')).status()).toBe(200);
    expect((await register(request, `${name} `, 'secretpass2')).status()).not.toBe(200);
    expect((await register(request, `‮${name}`, 'secretpass2')).status()).not.toBe(200);
    expect((await login(request, ` ${name} `, 'secretpass1')).status()).toBe(200);
  });

  test('a registered username comes back clean, and no longer than a seat can hold', async ({ request }) => {
    const res = await register(request, `  ${rand()}${'n'.repeat(500)}  `, 'secretpass1');
    expect(res.status(), await res.text()).toBe(200);
    const body = await res.json();
    const shown = body.username ?? body.guestName;
    expect([...String(shown)].length).toBeLessThanOrEqual(32);
  });

  test('renaming goes through the same cleaning, and says so when a name is refused', async ({ request }) => {
    const name = `ren-${rand()}`;
    const reg = await (await register(request, name, 'secretpass1')).json();
    const patch = (username: string) =>
      request.patch(`${API_BASE}/users/me`, {
        headers: { Authorization: `Bearer ${reg.accessToken}` },
        data: { username },
      });

    for (const blank of ['', '   ', '\u200b', '\u202e']) {
      expect((await patch(blank)).status(), `rename to ${JSON.stringify(blank)}`).toBe(400);
    }
    expect((await patch('x'.repeat(200))).status()).toBe(400);

    const fresh = `new-${rand()}`;
    const ok = await patch(`  \u202e${fresh}  `);
    expect(ok.status(), await ok.text()).toBe(200);
    const me = await (
      await request.get(`${API_BASE}/users/me`, { headers: { Authorization: `Bearer ${reg.accessToken}` } })
    ).json();
    expect(me.username).toBe(fresh);
  });
});

test.describe('request size', () => {
  test('a body past the limit is refused before it is stored or hashed', async ({ request }) => {
    const res = await request.post(`${API_BASE}/auth/register`, {
      headers: { 'content-type': 'application/json' },
      data: JSON.stringify({ username: `big-${rand()}`, password: 'secretpass1', pad: 'y'.repeat(9 * 1024 * 1024) }),
    });
    expect(res.status()).toBeGreaterThanOrEqual(400);
    expect(res.status()).toBeLessThan(500);
  });

  test('the scorepad, which needs no sign-in, keeps to its own small limit', async ({ request }) => {
    const res = await request.post(`${API_BASE}/scoring-sessions`, {
      headers: { 'content-type': 'application/json' },
      data: JSON.stringify({ players: ['A', 'B'], pad: 'y'.repeat(64 * 1024) }),
    });
    expect(res.status()).toBe(400);
  });
});

