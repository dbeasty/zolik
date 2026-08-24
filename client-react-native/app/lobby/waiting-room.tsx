import { router } from 'expo-router';
import { useCallback, useEffect } from 'react';
import { ActivityIndicator, Pressable, Text, View } from 'react-native';

import { Screen } from '@/src/components/Screen';
import { ZOLIK_BASE_URL } from '@/src/config';
import { useLobbySocket } from '@/src/hooks/useLobbySocket';
import { useSession } from '@/src/context/SessionContext';
import { colors, shared } from '@/src/theme';

/**
 * The waiting room: connect here to be visible to a host looking to pick up
 * players directly, without a join code.
 *
 * This is deliberately just a socket and a count — the pool itself is
 * rendered on the host's create-lobby screen, where the "invite" action
 * actually lives. A waiting player has nothing to do here but wait, so the
 * only thing worth showing is that the connection is live and how many
 * other people share it, plus the one event that ends the wait: getting
 * picked up.
 *
 * The connection status is rendered honestly rather than as a single
 * spinner: "still connecting for the first time," "connected," and "lost
 * the connection and retrying" are different situations, and collapsing
 * them into one indistinguishable "…" is what made a wrong server address
 * on a second device look identical to a working-but-slow connection during
 * testing. Every state names the server this device is actually trying to
 * reach, so a mismatched address (the most common reason two devices don't
 * see each other) is diagnosable from this screen alone.
 */
export default function WaitingRoomScreen() {
  const { session, loading } = useSession();

  const onInvited = useCallback((gameId: string, _joinCode: string) => {
    router.replace(`/lobby/join?gameId=${encodeURIComponent(gameId)}`);
  }, []);

  // enabled only once a session is actually known, not merely "not yet
  // present" — see the loading guard below for why that distinction matters.
  const { players, status, attempts, retryNow } = useLobbySocket(
    Boolean(session) && !loading,
    onInvited,
  );

  useEffect(() => {
    // SessionProvider restores a persisted session asynchronously on boot
    // (SecureStore/localStorage is never synchronous), so on a fresh page
    // load `session` reads as null for a moment before `loading` turns
    // false — not because nobody is signed in, but because the answer just
    // hasn't arrived yet. Redirecting on that first, transient null bounced
    // an actually-signed-in guest straight back to the sign-in screen on
    // every hard reload of this route.
    if (!loading && !session) router.replace('/auth/login');
  }, [session, loading]);

  if (loading || !session) {
    return (
      <Screen title="Waiting room">
        <ActivityIndicator color={colors.accent} />
      </Screen>
    );
  }

  const others = players.filter((p) => p.playerId !== session.userId).length;

  return (
    <Screen
      title="Waiting room"
      subtitle="A host can pick you up directly — no join code needed"
      scroll
    >
      <ConnectionCard status={status} attempts={attempts} onRetry={retryNow} others={others} />

      <Text style={[shared.status, { marginTop: 16 }]} testID="waiting-room-server">
        Server: {ZOLIK_BASE_URL}
      </Text>
      <Text style={[shared.status, { marginTop: 8 }]}>
        Leave this screen at any time — you'll stop appearing in the waiting
        room the moment you do.
      </Text>
    </Screen>
  );
}

function ConnectionCard({
  status,
  attempts,
  onRetry,
  others,
}: {
  status: 'connecting' | 'open' | 'reconnecting';
  attempts: number;
  onRetry: () => void;
  others: number;
}) {
  if (status === 'open') {
    return (
      <View style={shared.card} testID="waiting-room-status-open">
        <Text style={{ color: colors.success, fontWeight: '600', marginBottom: 4 }}>
          You're visible to hosts
        </Text>
        <Text style={shared.status}>
          {others === 0
            ? 'No one else is waiting right now.'
            : others === 1
              ? '1 other player is also waiting.'
              : `${others} other players are also waiting.`}
        </Text>
      </View>
    );
  }

  if (status === 'reconnecting') {
    return (
      <View style={shared.card} testID="waiting-room-status-reconnecting">
        <Text style={{ color: colors.gold, fontWeight: '600', marginBottom: 4 }}>
          Connection lost — reconnecting…
        </Text>
        <Text style={shared.status}>
          Attempt {attempts}. This can happen if your device's network changed, or the server
          restarted.
        </Text>
        <Pressable
          style={[shared.buttonSecondary, { marginTop: 12, marginBottom: 0 }]}
          onPress={onRetry}
        >
          <Text style={shared.buttonTextSecondary}>Try again now</Text>
        </Pressable>
      </View>
    );
  }

  return (
    <View style={shared.card} testID="waiting-room-status-connecting">
      <ActivityIndicator color={colors.accent} style={{ marginBottom: 8 }} />
      <Text style={shared.status}>Connecting…</Text>
      <Text style={[shared.status, { marginTop: 4, fontSize: 12 }]}>
        If this doesn't finish in a few seconds, double-check the server address below is
        reachable from this device.
      </Text>
    </View>
  );
}
