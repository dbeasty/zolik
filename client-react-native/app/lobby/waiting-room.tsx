import { router } from 'expo-router';
import { useCallback, useEffect } from 'react';
import { ActivityIndicator, Text, View } from 'react-native';

import { Screen } from '@/src/components/Screen';
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
 */
export default function WaitingRoomScreen() {
  const { session } = useSession();

  const onInvited = useCallback((gameId: string, _joinCode: string) => {
    router.replace(`/lobby/join?gameId=${encodeURIComponent(gameId)}`);
  }, []);

  const { players, connected } = useLobbySocket(Boolean(session), onInvited);

  useEffect(() => {
    if (!session) router.replace('/auth/login');
  }, [session]);

  if (!session) {
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
      <View style={shared.card}>
        {connected ? (
          <>
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
          </>
        ) : (
          <>
            <ActivityIndicator color={colors.accent} style={{ marginBottom: 8 }} />
            <Text style={shared.status}>Connecting…</Text>
          </>
        )}
      </View>

      <Text style={[shared.status, { marginTop: 16 }]}>
        Leave this screen at any time — you'll stop appearing in the waiting
        room the moment you do.
      </Text>
    </Screen>
  );
}
