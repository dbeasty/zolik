/**
 * Pure helpers for the sign-in flows.
 *
 * The browser round trip is the part of authentication most likely to break
 * quietly — a callback URL that parses wrong looks exactly like a user who
 * changed their mind. Keeping the parsing and labelling here, free of React
 * and of the network, means it can be tested directly.
 */

import type { AuthProvider } from '@/src/api/types';
import { t } from '@/src/lib/i18n';

/** What came back on the deep link the provider's browser tab returned to. */
export type CallbackResult =
  | { status: 'success'; code: string }
  | { status: 'cancelled' }
  | { status: 'error'; reason: string };

/**
 * Reads the outcome out of the URL the in-app browser handed back.
 *
 * Both spellings are checked because deep links are not consistent about it:
 * `zolik://auth?code=x` puts the parameters in the query, while some platforms
 * deliver `zolik://auth#code=x`. Missing both means the person dismissed the
 * browser, which is a cancellation rather than a failure — nothing went wrong
 * and nothing should be reported as if it had.
 */
export function parseAuthCallback(url: string | null | undefined): CallbackResult {
  if (!url) return { status: 'cancelled' };

  const params = new Map<string, string>();
  for (const separator of ['?', '#']) {
    const at = url.indexOf(separator);
    if (at === -1) continue;
    // A query section runs only up to a following fragment, not to the end of
    // the string — a url can carry both, and without this bound the query's
    // last value swallows the entire fragment as one long string.
    const stop = separator === '?' ? url.indexOf('#', at + 1) : -1;
    const section = stop === -1 ? url.slice(at + 1) : url.slice(at + 1, stop);
    for (const pair of section.split('&')) {
      if (!pair) continue;
      const eq = pair.indexOf('=');
      const rawKey = eq === -1 ? pair : pair.slice(0, eq);
      const rawValue = eq === -1 ? '' : pair.slice(eq + 1);
      const key = safeDecode(rawKey);
      // First writer wins: a query parameter is not overwritten by a fragment
      // one of the same name.
      if (key && !params.has(key)) params.set(key, safeDecode(rawValue));
    }
  }

  const code = params.get('code');
  if (code) return { status: 'success', code };

  const error = params.get('error');
  if (error) {
    // The person pressing "cancel" at the provider is not an error worth
    // showing them; every provider spells it one of these ways.
    if (error === 'access_denied' || error === 'user_cancelled_login') {
      return { status: 'cancelled' };
    }
    return { status: 'error', reason: error };
  }
  return { status: 'cancelled' };
}

function safeDecode(value: string): string {
  try {
    return decodeURIComponent(value.replace(/\+/g, ' '));
  } catch {
    // A malformed escape must not throw out of a parse whose whole job is to
    // report what happened.
    return value;
  }
}

/** Turns a server-side failure code into something worth reading. */
export function authErrorMessage(reason: string): string {
  switch (reason) {
    case 'already_linked':
      return t('auth.error.alreadyLinked');
    case 'expired':
      return t('auth.error.expired');
    case 'already_completed':
      return t('auth.error.alreadyUsed');
    case 'exchange_failed':
    case 'sign_in_failed':
      return t('auth.error.notCompleted');
    case 'unknown_provider':
      return t('auth.error.unknownProvider');
    case 'no_code':
      return t('auth.error.noCode');
    default:
      return t('auth.error.failed');
  }
}

/**
 * Orders the sign-in methods for display: the ones people recognise first,
 * guest play last.
 *
 * Guest going last is a deliberate nudge rather than a technical need. It is
 * the option that loses your statistics, so it should not be the one under
 * your thumb — but it stays plainly available, because being forced to make an
 * account before playing a card game is worse.
 */
export function orderProviders(providers: AuthProvider[]): AuthProvider[] {
  const rank = (p: AuthProvider) => {
    if (p.kind === 'oauth') return 0;
    if (p.kind === 'email') return 1;
    return 2;
  };
  return [...providers].sort((a, b) => {
    const byKind = rank(a) - rank(b);
    return byKind !== 0 ? byKind : a.displayName.localeCompare(b.displayName);
  });
}

/** The label on a provider's button. */
export function providerButtonLabel(p: AuthProvider): string {
  switch (p.kind) {
    case 'guest':
      return t('auth.provider.guest');
    case 'email':
      return t('auth.provider.email');
    default:
      return t('auth.provider.continueWith', { provider: p.displayName });
  }
}

/**
 * The line offering to keep a guest's play, or null when there is nothing to
 * keep.
 *
 * Returning null rather than an encouraging zero matters: "sign in to keep
 * your 0 games" is an advert, and one that undersells the actual offer to
 * somebody who has just installed the app.
 */
export function claimPrompt(matches: number | undefined): string | null {
  if (!matches || matches < 1) return null;
  // `.1` carries the one-game wording (see i18n `t`).
  return t('auth.claim.prompt', { n: matches });
}

/** How a completed sign-in reports what it rescued. */
export function claimedMessage(claimed: number): string | null {
  if (!claimed || claimed < 1) return null;
  return t('auth.claim.added', { n: claimed });
}
