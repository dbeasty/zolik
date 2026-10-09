import { router, useLocalSearchParams } from 'expo-router';
import { useEffect, useRef, useState } from 'react';
import { ActivityIndicator, Platform, Pressable, Text } from 'react-native';

import { FormError } from '@/src/a11y/Field';
import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { formatApiError } from '@/src/lib/apiError';
import { t } from '@/src/lib/i18n';
import { savePendingDestination } from '@/src/lib/pendingDestination';
import { colors, shared } from '@/src/theme';

/**
 * Where an AI client's sign-in lands: `/oauth/consent?req=…`.
 *
 * The server's /oauth/authorize validated the client and sent the person here;
 * this screen is the authentication (they are signed in as themselves) and the
 * consent (they choose). Nothing about the agent's powers is negotiable — it
 * can play tables and nothing else — so the screen says so and asks one
 * question.
 *
 * The answer is a redirect back to the client, produced by the server so the
 * `redirect_uri` that is followed is the one it vouched for, never one this
 * screen was handed in a query string.
 */
export default function OAuthConsentScreen() {
  const { client, session, loading } = useSession();
  const { req } = useLocalSearchParams<{ req: string }>();
  const request = String(req ?? '');

  const [info, setInfo] = useState<{ clientName: string; redirectHost: string } | null>(null);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const started = useRef(false);

  useEffect(() => {
    if (loading || started.current) return;
    started.current = true;
    if (!request) {
      setError(t('oauth.expired'));
      return;
    }
    if (!session) {
      // Held aside while they sign in, then replayed — see pendingDestination.
      savePendingDestination(`/oauth/consent?req=${encodeURIComponent(request)}`).then(() =>
        router.replace('/auth/guest'),
      );
      return;
    }
    client
      .oauthRequestInfo(request)
      .then(setInfo)
      .catch(() => setError(t('oauth.expired')));
  }, [client, loading, request, session]);

  async function answer(approve: boolean) {
    setBusy(true);
    setError('');
    try {
      const { redirectUrl } = await client.oauthApprove(request, approve);
      if (Platform.OS === 'web') window.location.assign(redirectUrl);
      else router.replace('/');
    } catch (e) {
      setBusy(false);
      setError(formatApiError(e, t('oauth.expired')));
    }
  }

  const name = info?.clientName ?? '';
  return (
    <Screen title={name ? t('oauth.heading', { name }) : t('oauth.working')} scroll>
      {error ? (
        <FormError testID="oauth-error" message={error} />
      ) : null}
      {info ? (
        <>
          <Text style={shared.status}>{t('oauth.explain', { name })}</Text>
          <Text style={{ color: colors.muted, fontSize: 13, marginBottom: 16 }}>
            {t('oauth.returnsTo', { host: info.redirectHost })}
          </Text>
          <Pressable role="button" testID="oauth-allow" style={shared.button} onPress={() => answer(true)} disabled={busy}>
            <Text style={shared.buttonText}>{t('oauth.allow')}</Text>
          </Pressable>
          <Pressable role="button" testID="oauth-deny" style={shared.button} onPress={() => answer(false)} disabled={busy}>
            <Text style={shared.buttonText}>{t('oauth.deny')}</Text>
          </Pressable>
        </>
      ) : error ? null : (
        <ActivityIndicator aria-label={t('a11y.loading')} />
      )}
    </Screen>
  );
}
