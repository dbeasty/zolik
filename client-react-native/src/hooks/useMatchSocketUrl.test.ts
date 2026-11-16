/**
 * A token renewal must not reopen the match socket: the old memo depended on
 * the token's value, so every fifteen-minute refresh dropped a working
 * connection and showed "Connecting…".
 */
import React from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';

import { useMatchSocketUrl } from '@/src/hooks/useMatchSocketUrl';

type S = { accessToken: string; userId: string } | null;

describe('useMatchSocketUrl', () => {
  let token = 'tok-1';
  const client = { matchSocketUrl: jest.fn((id: string) => `ws://x/ws/matches/${id}?token=${token}`) };
  const seen: (string | null)[] = [];
  function Probe({ session, settling }: { session: S; settling: boolean }) {
    seen.push(useMatchSocketUrl(client, 'm1', session, settling));
    return null;
  }
  let r: ReactTestRenderer;
  const render = (session: S, settling = false) =>
    act(() => {
      if (r) r.update(React.createElement(Probe, { session, settling }));
      else r = create(React.createElement(Probe, { session, settling }));
    });

  beforeEach(() => {
    seen.length = 0;
    client.matchSocketUrl.mockClear();
    token = 'tok-1';
  });

  it('keeps the same address across a token renewal', () => {
    render({ accessToken: 'tok-1', userId: 'u1' });
    const first = seen[seen.length - 1];
    expect(first).toContain('token=tok-1');
    token = 'tok-2';
    render({ accessToken: 'tok-2', userId: 'u1' });
    expect(seen[seen.length - 1]).toBe(first);
    expect(client.matchSocketUrl).toHaveBeenCalledTimes(1);
  });

  it('opens nothing without a token or while settling, and a new viewer gets a new address', () => {
    render(null);
    expect(seen[seen.length - 1]).toBeNull();
    render({ accessToken: 'tok-1', userId: 'u1' }, true);
    expect(seen[seen.length - 1]).toBeNull();
    render({ accessToken: 'tok-1', userId: 'u1' });
    expect(seen[seen.length - 1]).toContain('token=tok-1');
    token = 'tok-9';
    render({ accessToken: 'tok-9', userId: 'u2' });
    expect(seen[seen.length - 1]).toContain('token=tok-9');
  });
});
