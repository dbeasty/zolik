import { router, useLocalSearchParams } from 'expo-router';
import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, Pressable, Text } from 'react-native';

import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { formatApiError } from '@/src/lib/apiError';
import { moduleName, variationName } from '@/src/lib/gameLabels';
import { routeForMatch } from '@/src/lib/matchRoute';
import { clearPendingDestination, savePendingDestination } from '@/src/lib/pendingDestination';
import { shared } from '@/src/theme';
import { t } from '@/src/lib/i18n';

/**
 * Where a sent deal lands: `/deal/<token>`.
 *
 * Somebody finished a game of solitaire and sent its cards on. This says which
 * game that is and deals it at a table of the reader's own — the same cards,
 * with nothing about the game they came from. The token is sealed by the
 * server, so this screen cannot tell which match it was either, and does not
 * need to.
 *
 * Unlike a table invite it asks for one press. Following a join link puts you
 * in a seat that was already waiting; following this one starts a game, and a
 * game is something to choose to start.
 */
export default function SentDealScreen() {
  const { client, session, loading } = useSession();
  const { token: raw } = useLocalSearchParams<{ token: string }>();
  const token = String(raw ?? '').trim();

  const [deal, setDeal] = useState<{ moduleId: string; variation?: string } | null>(null);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let live = true;
    client
      .sentDeal(token)
      .then((d) => live && setDeal(d))
      .catch((e) => live && setError(formatApiError(e, t('deal.stale'))));
    return () => {
      live = false;
    };
  }, [client, token]);

  const play = useCallback(async () => {
    if (!session) {
      // Held, as a join link is, so signing in as a guest brings them back here.
      await savePendingDestination(`/deal/${encodeURIComponent(token)}`);
      router.replace('/auth/guest');
      return;
    }
    setBusy(true);
    try {
      const next = await client.playSentDeal(token);
      await clearPendingDestination();
      router.replace(routeForMatch(next.status, next.hostId === session.userId, next.matchId));
    } catch (e) {
      setError(formatApiError(e, t('deal.stale')));
      setBusy(false);
    }
  }, [client, session, token]);

  if (error) {
    return (
      <Screen scroll>
        <Text testID="deal-error" style={shared.error}>
          {error}
        </Text>
        <Pressable testID="deal-error-home" onPress={() => router.replace('/')}>
          <Text style={shared.status}>{t('join.backToMenu')}</Text>
        </Pressable>
      </Screen>
    );
  }

  if (!deal || loading) {
    return (
      <Screen scroll>
        <ActivityIndicator testID="deal-loading" />
      </Screen>
    );
  }

  const game = moduleName(deal.moduleId);
  return (
    <Screen scroll>
      <Text testID="deal-game" style={shared.title}>
        {deal.variation ? `${game} · ${variationName(deal.moduleId, deal.variation)}` : game}
      </Text>
      <Text style={[shared.status, { marginBottom: 16 }]}>{t('deal.body', { game })}</Text>
      <Pressable
        testID="deal-play"
        accessibilityRole="button"
        disabled={busy}
        onPress={play}
        style={[shared.button, busy && { opacity: 0.6 }]}
      >
        <Text style={shared.buttonText}>{t('deal.play')}</Text>
      </Pressable>
    </Screen>
  );
}
