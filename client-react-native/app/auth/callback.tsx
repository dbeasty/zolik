import { router, useGlobalSearchParams } from 'expo-router';
import { useEffect, useRef, useState } from 'react';
import { ActivityIndicator, Text } from 'react-native';

import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { authErrorMessage, claimedMessage } from '@/src/lib/auth';
import { shared, colors } from '@/src/theme';
import { t } from '@/src/lib/i18n';
import { useLocale } from '@/src/hooks/useLocale';

/**
 * Where the browser lands if the OAuth redirect actually navigates the app,
 * rather than being captured in-flight by `WebBrowser.openAuthSessionAsync`
 * (the native path, which resolves before this route is ever visited).
 *
 * On web there is no such capture — the provider's redirect is a normal
 * page navigation — so this screen is the web build's real landing point for
 * every browser sign-in. It re-derives the outcome from the URL rather than
 * assuming SessionContext's in-flight promise is still around to resolve it,
 * because on web that promise lives in the tab that is being replaced.
 */
export default function AuthCallbackScreen() {
  useLocale();
  const params = useGlobalSearchParams<{ code?: string; error?: string }>();
  const { completeOAuth } = useSession();
  // The outcome, not its words: the stored language can finish loading after
  // this screen has mounted, and a sentence rendered then stayed in English
  // under a translated title. Worded at render time, it follows the locale.
  const [outcome, setOutcome] = useState<
    | { kind: 'signingIn' }
    | { kind: 'failed'; reason: string }
    | { kind: 'thrown'; text: string }
    | { kind: 'signedIn'; claimed: number }
  >({ kind: 'signingIn' });
  const message =
    outcome.kind === 'failed'
      ? authErrorMessage(outcome.reason)
      : outcome.kind === 'thrown'
        ? outcome.text || t('error.signIn')
        : outcome.kind === 'signedIn'
          ? (claimedMessage(outcome.claimed) ?? t('auth.callback.signedIn'))
          : t('a11y.auth.signingIn');
  const ran = useRef(false);

  useEffect(() => {
    // Effects can fire twice under React's strict double-invoke; the
    // exchange code is single-use, so a second call would only ever fail —
    // guard it rather than let that failure overwrite a success already in
    // flight.
    if (ran.current) return;
    ran.current = true;

    if (params.error) {
      setOutcome({ kind: 'failed', reason: String(params.error) });
      setTimeout(() => router.replace('/auth/login'), 1500);
      return;
    }
    const code = params.code ? String(params.code) : '';
    if (!code) {
      router.replace('/');
      return;
    }
    // Through the same tail as every other sign-in: the claim prompt is
    // cleared and a guest identity the server just retired is forgotten,
    // rather than sent again on the next guest sign-in.
    completeOAuth(code)
      .then((done) => {
        setOutcome({ kind: 'signedIn', claimed: done.claimedMatches });
        router.replace('/');
      })
      .catch((e) => {
        setOutcome({ kind: 'thrown', text: e instanceof Error ? e.message : '' });
        setTimeout(() => router.replace('/auth/login'), 1500);
      });
  }, [params.code, params.error, completeOAuth]);

  return (
    <Screen title={t('nav.signingIn')}>
      <ActivityIndicator aria-label={t('a11y.loading')} color={colors.accent} style={{ marginBottom: 16 }} />
      {/* Polite: the message changes from "signing you in" to the outcome,
          and a screen reader should hear the outcome. */}
      <Text style={shared.status} aria-live="polite">
        {message}
      </Text>
    </Screen>
  );
}
