import { router, useLocalSearchParams } from 'expo-router';
import { useCallback, useEffect, useRef, useState } from 'react';
import { ActivityIndicator, Pressable, Text } from 'react-native';

import type { MatchState } from '@/src/api/matchTypes';
import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { formatApiError } from '@/src/lib/apiError';
import { clearPendingDestination, savePendingDestination } from '@/src/lib/pendingDestination';
import { shared } from '@/src/theme';
import { t } from '@/src/lib/i18n';

/**
 * Where a shared link lands: `/join/ABC123`.
 *
 * This is the whole of the Zoom-shaped promise — somebody is sent a URL, they
 * open it, and they are at the table. Nothing is typed and nothing is read out
 * loud, so this screen's job is to make the steps between those two facts
 * invisible when they succeed and legible when they do not.
 *
 * There are only three of them:
 *
 *  1. **No session yet.** Overwhelmingly the common case, because an invite
 *     arrives in a chat on a device that has never played. The code is put
 *     aside (`pendingDestination`) and the guest screen takes over — a display name
 *     and a face, which is the same thing Zoom asks for and no more.
 *  2. **A session.** Take the seat and go where a seated player belongs: the
 *     table screen for the host, the waiting-for-the-host screen for everyone
 *     else. The join call is idempotent server-side, so re-opening a link you
 *     already followed is a no-op rather than a second seat.
 *  3. **A refusal.** Full, already dealt, or no such table. These are ordinary
 *     outcomes of a link that has been sitting in a chat for an hour, so they
 *     are worded and offered a way onward, not shown as a failed request.
 *
 * The screen deliberately does not preview the table before joining. A seat is
 * cheap, un-joining is a page away, and an extra confirm step between "I
 * clicked the link" and "I am in" is exactly the friction the feature removes.
 */
export default function JoinByLinkScreen() {
  const { client, session, loading } = useSession();
  const { code } = useLocalSearchParams<{ code: string }>();
  const joinCode = String(code ?? '').trim();

  const [error, setError] = useState('');
  const [table, setTable] = useState<MatchState | null>(null);
  // One attempt per mount. Without it, React's development double-invoke and
  // every incidental re-render would each fire a join for the same seat.
  const attempted = useRef(false);

  const follow = useCallback(async () => {
    if (!joinCode) {
      setError(t('join.missingCode'));
      return;
    }

    if (!session) {
      // Held rather than passed as a query parameter: the sign-in screens
      // navigate to fixed destinations, and threading a code through every one
      // of them — including the OAuth round trip, which leaves the app
      // entirely — is how it gets dropped.
      await savePendingDestination(`/join/${encodeURIComponent(joinCode)}`);
      router.replace('/auth/guest');
      return;
    }

    try {
      // Read before joining, purely so the waiting screen can name the game
      // while the seat is being taken. A failure here is not fatal: joining is
      // the thing that matters, and it will produce the real refusal.
      try {
        setTable(await client.getMatch(joinCode));
      } catch {
        /* the join below is the authority on whether this table exists */
      }

      const matchId = await client.joinMatch(joinCode);
      // The note has done its job. Cleared before navigating, so a table that
      // refuses the next visitor does not follow them around.
      await clearPendingDestination();

      const seated = await client.getMatch(matchId, session.userId);
      if (seated.status !== 'lobby') {
        router.replace(`/match/${matchId}`);
        return;
      }
      // A host following their own link is sent to their own table, not to a
      // screen telling them to wait for themselves.
      router.replace(
        seated.hostId === session.userId
          ? `/lobby/table?matchId=${encodeURIComponent(matchId)}`
          : `/lobby/join?matchId=${encodeURIComponent(matchId)}`,
      );
    } catch (e) {
      await clearPendingDestination();
      setError(formatApiError(e, 'That table could not be joined'));
    }
  }, [client, joinCode, session]);

  useEffect(() => {
    // The session is read from storage asynchronously; acting before it has
    // settled would send a returning player through guest sign-in.
    if (loading || attempted.current) return;
    attempted.current = true;
    follow();
  }, [follow, loading]);

  if (error) {
    return (
      <Screen title={t('nav.join')} scroll>
        <Text testID="invite-error" style={shared.error}>
          {error}
        </Text>
        <Text style={shared.status}>
          {t('join.staleLink')}
        </Text>
        <Pressable
          testID="invite-error-join"
          style={shared.button}
          onPress={() => router.replace('/lobby/join')}
        >
          <Text style={shared.buttonText}>{t('join.enterCode')}</Text>
        </Pressable>
        <Pressable testID="invite-error-home" onPress={() => router.replace('/')}>
          <Text style={shared.status}>{t('join.backToMenu')}</Text>
        </Pressable>
      </Screen>
    );
  }

  return (
    <Screen title={t('nav.joining')} scroll>
      <ActivityIndicator testID="invite-joining" />
      <Text style={[shared.status, { marginTop: 12 }]}>
        {table?.moduleId ? t('join.takingSeatAt', { game: table.moduleId }) : t('join.takingSeat')}
      </Text>
    </Screen>
  );
}
