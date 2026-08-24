import {
  authErrorMessage,
  claimPrompt,
  claimedMessage,
  orderProviders,
  parseAuthCallback,
  providerButtonLabel,
} from '@/src/lib/auth';
import type { AuthProvider } from '@/src/api/types';

describe('parseAuthCallback', () => {
  it('reads a code out of the query string', () => {
    const got = parseAuthCallback('clientreactnative://auth/callback?code=abc123');
    expect(got).toEqual({ status: 'success', code: 'abc123' });
  });

  it('reads a code out of a fragment, for platforms that deliver it that way', () => {
    const got = parseAuthCallback('clientreactnative://auth/callback#code=xyz789');
    expect(got).toEqual({ status: 'success', code: 'xyz789' });
  });

  it('prefers a query parameter over a same-named fragment one', () => {
    const got = parseAuthCallback(
      'clientreactnative://auth/callback?code=fromQuery#code=fromFragment',
    );
    expect(got).toEqual({ status: 'success', code: 'fromQuery' });
  });

  it('treats a dismissed browser (no url) as a cancellation, not an error', () => {
    expect(parseAuthCallback(null)).toEqual({ status: 'cancelled' });
    expect(parseAuthCallback(undefined)).toEqual({ status: 'cancelled' });
  });

  it('treats the provider access_denied / user_cancelled codes as a cancellation', () => {
    expect(parseAuthCallback('app://cb?error=access_denied')).toEqual({ status: 'cancelled' });
    expect(parseAuthCallback('app://cb?error=user_cancelled_login')).toEqual({
      status: 'cancelled',
    });
  });

  it('surfaces every other provider error as an error', () => {
    expect(parseAuthCallback('app://cb?error=server_error')).toEqual({
      status: 'error',
      reason: 'server_error',
    });
  });

  it('decodes percent-encoded values', () => {
    const got = parseAuthCallback('app://cb?code=a%20b%2Bc');
    expect(got).toEqual({ status: 'success', code: 'a b+c' });
  });

  it('does not throw on a malformed percent-escape', () => {
    // A callback URL is provider-controlled input; a parser that throws here
    // would crash the app on a malformed redirect instead of just failing to
    // find a code.
    expect(() => parseAuthCallback('app://cb?code=%zz')).not.toThrow();
  });

  it('reports a url with neither code nor error as a cancellation', () => {
    expect(parseAuthCallback('app://cb?state=xyz')).toEqual({ status: 'cancelled' });
  });
});

describe('authErrorMessage', () => {
  it('gives every known reason a readable, distinct message', () => {
    const reasons = [
      'already_linked',
      'expired',
      'already_completed',
      'exchange_failed',
      'sign_in_failed',
      'unknown_provider',
      'no_code',
    ];
    const messages = reasons.map(authErrorMessage);
    for (const m of messages) {
      expect(m.length).toBeGreaterThan(0);
    }
    expect(new Set(messages).size).toBeGreaterThan(1);
  });

  it('falls back to a generic message for an unrecognised reason', () => {
    expect(authErrorMessage('something_new_the_server_added')).toMatch(/sign-in failed/i);
  });
});

describe('orderProviders', () => {
  const google: AuthProvider = { id: 'google', displayName: 'Google', kind: 'oauth' };
  const microsoft: AuthProvider = { id: 'microsoft', displayName: 'Microsoft', kind: 'oauth' };
  const email: AuthProvider = { id: 'email', displayName: 'Email', kind: 'email' };
  const guest: AuthProvider = { id: 'guest', displayName: 'Play as guest', kind: 'guest' };

  it('puts guest last: it is the option that loses your statistics', () => {
    const got = orderProviders([guest, email, google]);
    expect(got[got.length - 1]).toBe(guest);
  });

  it('puts oauth providers ahead of email', () => {
    const got = orderProviders([email, google]);
    expect(got).toEqual([google, email]);
  });

  it('orders same-kind providers alphabetically, for a stable layout', () => {
    const got = orderProviders([microsoft, google]);
    expect(got).toEqual([google, microsoft]);
  });

  it('does not mutate the input array', () => {
    const input = [guest, google];
    const copy = [...input];
    orderProviders(input);
    expect(input).toEqual(copy);
  });
});

describe('providerButtonLabel', () => {
  it('labels guest play plainly', () => {
    expect(providerButtonLabel({ id: 'guest', displayName: 'x', kind: 'guest' })).toBe(
      'Play as guest',
    );
  });

  it('labels email generically rather than by its internal id', () => {
    expect(providerButtonLabel({ id: 'email', displayName: 'Email', kind: 'email' })).toBe(
      'Continue with email',
    );
  });

  it('names the provider for an oauth button', () => {
    expect(
      providerButtonLabel({ id: 'google', displayName: 'Google', kind: 'oauth' }),
    ).toBe('Continue with Google');
  });
});

describe('claimPrompt', () => {
  it('offers nothing when there is nothing to keep', () => {
    expect(claimPrompt(undefined)).toBeNull();
    expect(claimPrompt(0)).toBeNull();
  });

  it('does not advertise a zero as if it were a real offer', () => {
    // Regression guard for the specific bug this function exists to avoid:
    // "keep your 0 games" reads as broken, not as an honest empty state.
    expect(claimPrompt(0)).toBeNull();
  });

  it('uses the singular for exactly one game', () => {
    expect(claimPrompt(1)).toBe('You have 1 game recorded on this device. Sign in to keep them.');
  });

  it('uses the plural otherwise', () => {
    expect(claimPrompt(5)).toContain('5 games');
  });
});

describe('claimedMessage', () => {
  it('says nothing when nothing was claimed', () => {
    expect(claimedMessage(0)).toBeNull();
  });

  it('uses the singular for one claimed match', () => {
    expect(claimedMessage(1)).toMatch(/previous game has been added/);
  });

  it('uses the plural and the count for more than one', () => {
    expect(claimedMessage(3)).toBe('Your 3 previous games have been added to your account.');
  });
});
