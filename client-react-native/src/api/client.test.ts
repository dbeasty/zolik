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
