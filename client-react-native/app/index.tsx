import { router } from 'expo-router';
import { useCallback, useState } from 'react';
import { ActivityIndicator, Pressable, Text, View } from 'react-native';

import { Screen } from '@/src/components/Screen';
import { ZOLIK_BASE_URL } from '@/src/config';
import { useSession } from '@/src/context/SessionContext';
import { useLobbySocket } from '@/src/hooks/useLobbySocket';
import { useWaitingLobbyStatus } from '@/src/hooks/useWaitingLobbyStatus';
import type { PlayerSession } from '@/src/api/types';
import { colors, shared } from '@/src/theme';

function MenuButton({
  label,
  onPress,
  secondary,
}: {
  label: string;
  onPress: () => void;
  secondary?: boolean;
}) {
  return (
    <Pressable
      style={[shared.button, secondary && shared.buttonSecondary]}
      onPress={onPress}
    >
      <Text style={[shared.buttonText, secondary && shared.buttonTextSecondary]}>
        {label}
      </Text>
    </Pressable>
  );
}

export default function MainMenu() {
  const { session, loading, logout } = useSession();

  if (loading) {
    return (
      <Screen>
        <ActivityIndicator color="#3d8bfd" />
      </Screen>
    );
  }

  return (
    <Screen
      title="Žolíky"
      subtitle={`Continental Rummy · ${ZOLIK_BASE_URL}`}
      scroll
    >
      {session ? (
        <Text style={shared.status}>Playing as {session.username}</Text>
      ) : (
        <Text style={shared.status}>Sign in or continue as guest to play online.</Text>
      )}

      {session ? <WaitingStatusCard session={session} /> : null}

      <View style={{ marginTop: 16 }}>
        <MenuButton
          label="New game"
          onPress={() => {
            if (!session) {
              router.push('/auth/guest');
              return;
            }
            router.push('/lobby/create');
          }}
        />
        <MenuButton
          label="Join game"
          secondary
          onPress={() => {
            if (!session) {
              router.push('/auth/guest');
              return;
            }
            router.push('/lobby/join');
          }}
        />
        <MenuButton
          label="Offline score table"
          secondary
          onPress={() => router.push('/scoring')}
        />
        <MenuButton label="Stats & leaderboard" secondary onPress={() => router.push('/stats')} />

        {session ? (
          <>
            {session.isGuest ? (
              <MenuButton
                label="Sign in to keep your stats"
                secondary
                onPress={() => router.push('/auth/login')}
              />
            ) : (
              <MenuButton label="Account" secondary onPress={() => router.push('/account')} />
            )}
            <MenuButton label="Sign out" secondary onPress={() => logout()} />
          </>
        ) : (
          <>
            <MenuButton label="Sign in" onPress={() => router.push('/auth/login')} />
            <MenuButton
              label="Continue as guest"
              secondary
              onPress={() => router.push('/auth/guest')}
            />
          </>
        )}
      </View>
    </Screen>
  );
}

/**
 * "The main page would be the waiting room and would give us status of the
 * players available" — this is that status, right on the menu, and now
 * "Find players" toggles availability in place instead of navigating to a
 * separate screen. Being available *is* an active WebSocket connection
 * (useLobbySocket) that makes this device inviteable; browsing the count
 * beforehand is a read-only poll (useWaitingLobbyStatus) that commits to
 * nothing. Only one of the two is ever enabled at a time, driven by the
 * `available` toggle below — the two hooks themselves are unchanged from
 * how the old dedicated waiting-room screen used them.
 */
function WaitingStatusCard({ session }: { session: PlayerSession }) {
  const [available, setAvailable] = useState(false);

  const { players: idlePlayers, loaded: idleLoaded } = useWaitingLobbyStatus(!available);

  const onInvited = useCallback((gameId: string, _joinCode: string) => {
    router.replace(`/lobby/join?gameId=${encodeURIComponent(gameId)}`);
  }, []);
  const { players: livePlayers, status, attempts, retryNow } = useLobbySocket(available, onInvited);

  if (!available) {
    return (
      <View style={[shared.card, { marginTop: 12 }]} testID="home-waiting-status">
        {!idleLoaded ? (
          <Text style={shared.status}>Checking who's around…</Text>
        ) : idlePlayers.length === 0 ? (
          <Text style={shared.status}>No one is waiting to play right now.</Text>
        ) : (
          <>
            <Text style={{ color: colors.text, fontWeight: '600', marginBottom: 4 }}>
              {idlePlayers.length === 1
                ? '1 player waiting to play'
                : `${idlePlayers.length} players waiting to play`}
            </Text>
            <Text style={shared.status}>
              {idlePlayers
                .slice(0, 5)
                .map((p) => p.username)
                .join(', ')}
              {idlePlayers.length > 5 ? `, +${idlePlayers.length - 5} more` : ''}
            </Text>
          </>
        )}
        <Pressable
          style={[shared.buttonSecondary, { marginTop: 12, marginBottom: 0 }]}
          onPress={() => setAvailable(true)}
        >
          <Text style={shared.buttonTextSecondary}>Find players</Text>
        </Pressable>
      </View>
    );
  }

  const others = livePlayers.filter((p) => p.playerId !== session.userId).length;

  if (status === 'open') {
    return (
      <View style={[shared.card, { marginTop: 12 }]} testID="home-waiting-status">
        <View testID="waiting-status-open">
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
        <Pressable
          style={[shared.buttonSecondary, { marginTop: 12, marginBottom: 0 }]}
          onPress={() => setAvailable(false)}
        >
          <Text style={shared.buttonTextSecondary}>Stop</Text>
        </Pressable>
      </View>
    );
  }

  if (status === 'reconnecting') {
    return (
      <View style={[shared.card, { marginTop: 12 }]} testID="home-waiting-status">
        <View testID="waiting-status-reconnecting">
          <Text style={{ color: colors.gold, fontWeight: '600', marginBottom: 4 }}>
            Connection lost — reconnecting…
          </Text>
          <Text style={shared.status}>
            Attempt {attempts}. This can happen if your device's network changed, or the server
            restarted.
          </Text>
          <Text style={[shared.status, { marginTop: 4 }]}>Server: {ZOLIK_BASE_URL}</Text>
        </View>
        <Pressable
          style={[shared.buttonSecondary, { marginTop: 12, marginBottom: 0 }]}
          onPress={retryNow}
        >
          <Text style={shared.buttonTextSecondary}>Try again now</Text>
        </Pressable>
        <Pressable style={{ marginTop: 10 }} onPress={() => setAvailable(false)}>
          <Text style={shared.status}>Stop</Text>
        </Pressable>
      </View>
    );
  }

  return (
    <View style={[shared.card, { marginTop: 12 }]} testID="home-waiting-status">
      <View testID="waiting-status-connecting">
        <ActivityIndicator color={colors.accent} style={{ marginBottom: 8 }} />
        <Text style={shared.status}>Connecting…</Text>
        <Text style={[shared.status, { marginTop: 4, fontSize: 12 }]}>
          If this doesn't finish in a few seconds, check the server address below is reachable
          from this device.
        </Text>
        <Text style={[shared.status, { marginTop: 4 }]}>Server: {ZOLIK_BASE_URL}</Text>
      </View>
      <Pressable style={{ marginTop: 10 }} onPress={() => setAvailable(false)}>
        <Text style={shared.status}>Stop</Text>
      </Pressable>
    </View>
  );
}
