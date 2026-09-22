import { ApiError, ZolikClient, apiErrorFromResponse, socketBase } from '@/src/api/client';

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
    expect(s.updated).toHaveBeenCalledWith('a1', 'r1');
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
