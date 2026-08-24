import { router } from 'expo-router';
import { ActivityIndicator, Pressable, Text, View } from 'react-native';

import { Screen } from '@/src/components/Screen';
import { ZOLIK_BASE_URL } from '@/src/config';
import { useSession } from '@/src/context/SessionContext';
import { useWaitingLobbyStatus } from '@/src/hooks/useWaitingLobbyStatus';
import type { WaitingPlayer } from '@/src/api/types';
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
  // Status only — this does not connect to the waiting room itself, and so
  // does not make this device visible to any host. That only happens when
  // someone actually taps "Find players" below (see useLobbySocket, and
  // useWaitingLobbyStatus's own doc comment for why the two are separate).
  const { players: waitingPlayers, loaded: waitingLoaded } = useWaitingLobbyStatus(Boolean(session));

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

      {session ? (
        <WaitingStatusCard players={waitingPlayers} loaded={waitingLoaded} />
      ) : null}

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
 * players available" — this is that status, right on the menu, without
 * requiring a tap into a separate screen first.
 *
 * It stops short of joining the pool itself, on purpose: a count is
 * information, showing up as inviteable to a stranger's game is a decision,
 * and folding the second into just glancing at the app would surprise
 * people who opened it only to check something. "Find players" is the
 * explicit, single action that actually joins — see useLobbySocket.
 */
function WaitingStatusCard({ players, loaded }: { players: WaitingPlayer[]; loaded: boolean }) {
  return (
    <View style={[shared.card, { marginTop: 12 }]} testID="home-waiting-status">
      {!loaded ? (
        <Text style={shared.status}>Checking who's around…</Text>
      ) : players.length === 0 ? (
        <Text style={shared.status}>No one is waiting to play right now.</Text>
      ) : (
        <>
          <Text style={{ color: colors.text, fontWeight: '600', marginBottom: 4 }}>
            {players.length === 1 ? '1 player waiting to play' : `${players.length} players waiting to play`}
          </Text>
          <Text style={shared.status}>
            {players
              .slice(0, 5)
              .map((p) => p.username)
              .join(', ')}
            {players.length > 5 ? `, +${players.length - 5} more` : ''}
          </Text>
        </>
      )}
      <Pressable
        style={[shared.buttonSecondary, { marginTop: 12, marginBottom: 0 }]}
        onPress={() => router.push('/lobby/waiting-room')}
      >
        <Text style={shared.buttonTextSecondary}>Find players</Text>
      </Pressable>
    </View>
  );
}
