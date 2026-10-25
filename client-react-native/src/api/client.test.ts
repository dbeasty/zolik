import { ApiError, COOKIE_HELD_REFRESH, ZolikClient, apiErrorFromResponse, socketBase } from '@/src/api/client';

describe('apiErrorFromResponse', () => {
  it('parses a JSON refusal with code and Retry-After', () => {
    const err = apiErrorFromResponse(
      '{"code":"SERVER_BUSY","message":"SERVER_BUSY"}',
      503,
      { get: (name) => (name === 'Retry-After' ? '7' : null) },
    );
    expect(err).toBeInstanceOf(ApiError);
    expect(err.code).toBe('SERVER_BUSY');
    expect(err.status).toBe(503);
    expect(err.retryAfterMs).toBe(7000);
  });

  it('keeps raw text when the body is not JSON', () => {
    const err = apiErrorFromResponse('plain failure', 500, { get: () => null });
    expect(err.message).toBe('plain failure');
    expect(err.code).toBeUndefined();
  });
});

describe('socketBase', () => {
  it.each([
    ['https://play.limidus.com', 'wss://play.limidus.com'],
    ['https://play.limidus.com/', 'wss://play.limidus.com'],
    ['http://192.168.1.20:47800', 'ws://192.168.1.20:47800'],
    ['HTTP://10.0.2.2:8090/api?x=1', 'ws://10.0.2.2:8090'],
    ['192.168.1.20:47800', 'ws://192.168.1.20:47800'],
  ])('%s -> %s', (base, want) => {
    expect(socketBase(base)).toBe(want);
  });

  it('is what the match socket is opened on', () => {
    const c = new ZolikClient('https://play.limidus.com');
    c.bindSession({ accessToken: 'a b', refreshToken: 'r', userId: 'u' } as never);
    expect(c.matchSocketUrl('m/1')).toBe('wss://play.limidus.com/ws/matches/m%2F1?token=a%20b');
  });
});

describe('token refresh', () => {
  type Call = { path: string; auth?: string; body?: string };

  const reply = (status: number, body: unknown = {}) => ({
    status,
    ok: status >= 200 && status < 300,
    headers: { get: () => null },
    text: async () => JSON.stringify(body),
  });

  /** A server whose access tokens are `a0`, `a1`, … and which, like the real
   *  one, retires each refresh token the moment it is exchanged. */
  function server(refresh: (call: Call) => ReturnType<typeof reply> | Promise<never>) {
    const calls: Call[] = [];
    let current = 'a1';
    const transport = {
      fetch: async (path: string, init: { headers?: Record<string, string>; body?: string }) => {
        const call = { path, auth: init.headers?.Authorization, body: init.body };
        calls.push(call);
        await Promise.resolve();
        if (path === '/auth/refresh') return refresh(call);
        return call.auth === `Bearer ${current}` ? reply(200, { ok: true }) : reply(401);
      },
      socketUrl: (p: string) => p,
      openSocket: () => {
        throw new Error('unused');
      },
    };
    const client = new ZolikClient('http://x', transport as never);
    const expired = jest.fn();
    const updated = jest.fn();
    client.bindSession(
      { accessToken: 'a0', refreshToken: 'r0', userId: 'g' } as never,
      updated,
      expired,
    );
    return { client, calls, expired, updated, setCurrent: (t: string) => (current = t) };
  }

  it('shares one refresh between requests that were rejected together', async () => {
    const spent = new Set<string>();
    const s = server((call) => {
      const { refreshToken } = JSON.parse(call.body ?? '{}');
      if (spent.has(refreshToken)) return reply(401);
      spent.add(refreshToken);
      return reply(200, { accessToken: 'a1', refreshToken: 'r1' });
    });

    await Promise.all([s.client.getMe(), s.client.getMe(), s.client.getMe()]);

    expect(s.calls.filter((c) => c.path === '/auth/refresh')).toHaveLength(1);
    expect(s.expired).not.toHaveBeenCalled();
    // The third argument is the offline pass, which this server did not
    // rotate: a refresh that does not mention one leaves the one in hand.
    expect(s.updated).toHaveBeenCalledWith('a1', 'r1', undefined);
  });

  it('keeps the session when the refresh cannot reach the server', async () => {
    const s = server(() => Promise.reject(new TypeError('Failed to fetch')));
    await expect(s.client.getMe()).rejects.toThrow('Failed to fetch');
    expect(s.expired).not.toHaveBeenCalled();
  });

  it('keeps the session when the server is restarting', async () => {
    const s = server(() => reply(502));
    await expect(s.client.getMe()).rejects.toMatchObject({ status: 502 });
    expect(s.expired).not.toHaveBeenCalled();
  });

  it('signs out when the server refuses the refresh token', async () => {
    const s = server(() => reply(401));
    await expect(s.client.getMe()).rejects.toMatchObject({ status: 401 });
    expect(s.expired).toHaveBeenCalledTimes(1);
  });
});

describe('signing in keeps the offline pass', () => {
  /** A server that answers every sign-in shape with one pass, so a route that
   *  drops it shows up as an undefined rather than as a different value. */
  function transportFor(signInPath: string) {
    return {
      fetch: async (path: string) => ({
        status: 200,
        ok: true,
        headers: { get: () => null },
        text: async () =>
          JSON.stringify(
            path === signInPath
              ? { accessToken: 'a', refreshToken: 'r', offlinePass: 'pass', username: 'ada' }
              : { id: 'u1', username: 'ada' },
          ),
      }),
      socketUrl: (p: string) => p,
      openSocket: () => {
        throw new Error('unused');
      },
    };
  }

  // Every one of these is a way into a signed-in session, and a table with no
  // internet can only seat the account when the pass came along the way in.
  it('on a username sign-in', async () => {
    const client = new ZolikClient('http://x', transportFor('/auth/login') as never);
    await expect(client.login('ada', 'pw')).resolves.toMatchObject({ offlinePass: 'pass' });
  });

  it('on registering', async () => {
    const client = new ZolikClient('http://x', transportFor('/auth/register') as never);
    await expect(client.register('ada', 'pw')).resolves.toMatchObject({ offlinePass: 'pass' });
  });

  it('on a mailed code', async () => {
    const client = new ZolikClient('http://x', transportFor('/auth/email/verify') as never);
    const outcome = await client.verifyEmailCode('ada@example.com', '123456');
    expect(outcome.session).toMatchObject({ offlinePass: 'pass' });
  });
});

describe('cookie session (web)', () => {
  type Sent = { path: string; headers: Record<string, string>; body: Record<string, unknown>; credentials?: string };

  const reply = (body: unknown) => ({
    status: 200,
    ok: true,
    headers: { get: () => null },
    text: async () => JSON.stringify(body),
  });

  /** A server in cookie mode: the refresh token never comes back in a body. */
  function cookieClient() {
    const sent: Sent[] = [];
    const transport = {
      fetch: async (
        path: string,
        init: { headers?: Record<string, string>; body?: string; credentials?: string },
      ) => {
        sent.push({
          path,
          headers: init.headers ?? {},
          body: JSON.parse(init.body ?? '{}'),
          credentials: init.credentials,
        });
        if (path === '/auth/guest') {
          return reply({ accessToken: 'a0', refreshToken: '', guestId: 'g', guestKey: 'g.mac', guestName: 'G' });
        }
        if (path === '/auth/refresh') return reply({ accessToken: 'a1', refreshToken: '' });
        return reply({});
      },
      socketUrl: (p: string) => p,
      openSocket: () => {
        throw new Error('unused');
      },
    };
    return { client: new ZolikClient('http://x', transport as never, { cookieSession: true }), sent };
  }

  it('asks for cookies on the auth routes only', async () => {
    const { client, sent } = cookieClient();
    await client.guestLogin('G');
    await client.getMe();
    const [auth, other] = sent;
    expect(auth.headers['X-Zolik-Session']).toBe('cookie');
    expect(auth.credentials).toBe('include');
    expect(other.headers['X-Zolik-Session']).toBeUndefined();
    expect(other.credentials).toBeUndefined();
  });

  it('holds a marker, never sends it, and still renews', async () => {
    const { client, sent } = cookieClient();
    const s = await client.guestLogin('G');
    expect(s.refreshToken).toBe(COOKIE_HELD_REFRESH);
    expect(s.guestKey).toBe('g.mac');

    await client.refreshTokens();
    const refresh = sent.find((c) => c.path === '/auth/refresh')!;
    expect(refresh.body.refreshToken).toBeUndefined();
    expect(client.accessToken).toBe('a1');
    expect(client.refreshToken).toBe(COOKIE_HELD_REFRESH);

    await client.logout();
    expect(sent.find((c) => c.path === '/auth/logout')!.body.refreshToken).toBeUndefined();
  });

  it('hands over a token stored before cookies, once', async () => {
    const { client, sent } = cookieClient();
    client.bindSession({ accessToken: 'a0', refreshToken: 'stored', userId: 'g' } as never);
    await client.refreshTokens();
    expect(sent[0].body.refreshToken).toBe('stored');
    expect(client.refreshToken).toBe(COOKIE_HELD_REFRESH);
  });
});
