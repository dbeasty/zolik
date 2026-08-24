import { router, useGlobalSearchParams } from 'expo-router';
import { useEffect, useRef, useState } from 'react';
import { ActivityIndicator, Text } from 'react-native';

import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { authErrorMessage, claimedMessage } from '@/src/lib/auth';
import { shared, colors } from '@/src/theme';

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
  const params = useGlobalSearchParams<{ code?: string; error?: string }>();
  const { client, setSession } = useSession();
  const [message, setMessage] = useState('Signing you in…');
  const ran = useRef(false);

  useEffect(() => {
    // Effects can fire twice under React's strict double-invoke; the
    // exchange code is single-use, so a second call would only ever fail —
    // guard it rather than let that failure overwrite a success already in
    // flight.
    if (ran.current) return;
    ran.current = true;

    if (params.error) {
      setMessage(authErrorMessage(String(params.error)));
      setTimeout(() => router.replace('/auth/login'), 1500);
      return;
    }
    const code = params.code ? String(params.code) : '';
    if (!code) {
      router.replace('/');
      return;
    }
    client
      .exchangeOAuthCode(code)
      .then(async (outcome) => {
        await setSession(outcome.session);
        const claimed = claimedMessage(outcome.claimedMatches);
        setMessage(claimed ?? 'Signed in.');
        router.replace('/');
      })
      .catch((e) => {
        setMessage(e instanceof Error ? e.message : 'Sign-in failed');
        setTimeout(() => router.replace('/auth/login'), 1500);
      });
  }, [params.code, params.error, client, setSession]);

  return (
    <Screen title="Signing in">
      <ActivityIndicator color={colors.accent} style={{ marginBottom: 16 }} />
      <Text style={shared.status}>{message}</Text>
    </Screen>
  );
}
