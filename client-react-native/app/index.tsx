import { router } from 'expo-router';
import { useCallback, useEffect, useRef, useState } from 'react';
import { ActivityIndicator, Pressable, Text, View } from 'react-native';
import type { StyleProp, ViewStyle } from 'react-native';

import { Avatar } from '@/src/components/avatars/Avatar';
import { avatarFor } from '@/src/components/avatars/catalogue';
import { BuildFooter } from '@/src/components/BuildFooter';
import { Screen } from '@/src/components/Screen';
import { ZOLIK_BASE_URL } from '@/src/config';
import { useSession } from '@/src/context/SessionContext';
import { useLobbySocket } from '@/src/hooks/useLobbySocket';
import { useWaitingLobbyStatus } from '@/src/hooks/useWaitingLobbyStatus';
import type { PlayerSession, WaitingPlayer } from '@/src/api/types';
import { reasonText } from '@/src/lib/i18n';
import { consumePendingInvite } from '@/src/lib/pendingInvite';
import { colors, shared } from '@/src/theme';

function MenuButton({
  label,
  onPress,
  secondary,
  style,
}: {
  label: string;
  onPress: () => void;
  secondary?: boolean;
  /** Only for spacing — the button's own look is not negotiable, which is
   *  the point: every button on this screen is this one. */
  style?: StyleProp<ViewStyle>;
}) {
  return (
    <Pressable
      style={[shared.button, secondary && shared.buttonSecondary, style]}
      onPress={onPress}
    >
      <Text style={[shared.buttonText, secondary && shared.buttonTextSecondary]}>
        {label}
      </Text>
    </Pressable>
  );
}

export default function MainMenu() {
  const { session, loading } = useSession();
  useFollowPendingInvite(!!session && !loading);

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
          label="Play"
          onPress={() => {
            if (!session) {
              router.push('/auth/guest');
              return;
            }
            router.push('/lobby/games');
          }}
        />
        <MenuButton
          label="Join a table"
          secondary
          onPress={() => {
            if (!session) {
              router.push('/auth/guest');
              return;
            }
            router.push('/lobby/join');
          }}
        />
        {/* Settings, sign-out, the account and the second-tier screens are
            not here: they are behind the face in the top corner, which is
            where a player looks for themselves. See `AccountMenu`. What is
            left is the two things this screen exists to do — and, for
            somebody with no session yet, the two ways to get one. */}
        {!session ? (
          <>
            <MenuButton label="Sign in" onPress={() => router.push('/auth/login')} />
            <MenuButton
              label="Continue as guest"
              secondary
              onPress={() => router.push('/auth/guest')}
            />
          </>
        ) : null}
      </View>

      <BuildFooter />
    </Screen>
  );
}

/**
 * Resume an invite that was interrupted by signing in.
 *
 * Every account sign-in path — email code, username, OAuth callback, the
 * legacy registration — finishes with `router.replace('/')`, so the menu is
 * where they all come back to and therefore the only place one hook covers
 * them all. (Guest sign-in does its own, because it lands on the game picker
 * instead; see app/auth/guest.tsx.)
 *
 * Consumed, never merely read: the note is deleted before the navigation it
 * causes, so an invite followed once cannot ambush somebody on their next
 * visit to the menu. Nothing happens without a session, which is what keeps
 * this from firing during the moment before storage has been read.
 */
function useFollowPendingInvite(ready: boolean) {
  // Once per mount. The effect's dependencies settle more than once while the
  // session loads, and consuming twice would race the storage delete.
  const followed = useRef(false);

  useEffect(() => {
    if (!ready || followed.current) return;
    followed.current = true;
    let live = true;
    consumePendingInvite().then((code) => {
      if (live && code) router.replace(`/join/${encodeURIComponent(code)}`);
    });
    return () => {
      live = false;
    };
  }, [ready]);
}

/**
 * "The main page would be the waiting room and would give us status of the
 * players available" — this is that status, right on the menu.
 *
 * Two things a person has to be able to tell apart here, which the first cut
 * of this card ran together: *who is waiting* and *whether they themselves
 * are waiting*. So the roster is the body of the card in both states — real
 * faces and names, because "3 players waiting" answers a smaller question
 * than "who?" — and the button underneath does one thing only, which is to
 * put you in that list or take you back out. It is the same MenuButton as the
 * menu below it, deliberately: a control that publishes your availability
 * should not look like a different species of thing from "Play".
 *
 * Being available *is* an active WebSocket connection (useLobbySocket) that
 * makes this device inviteable; browsing the pool beforehand is a read-only
 * poll (useWaitingLobbyStatus) that commits to nothing. Only one of the two is
 * ever enabled at a time, driven by the `available` toggle below — the two
 * hooks themselves are unchanged from how the old dedicated waiting-room
 * screen used them.
 */
function WaitingStatusCard({ session }: { session: PlayerSession }) {
  const [available, setAvailable] = useState(false);

  const { players: idlePlayers, loaded: idleLoaded } = useWaitingLobbyStatus(!available);

  const onInvited = useCallback((matchId: string, _joinCode: string) => {
    router.replace(`/lobby/join?matchId=${encodeURIComponent(matchId)}`);
  }, []);
  const { players: livePlayers, status, attempts, retryNow } = useLobbySocket(available, onInvited);

  if (!available) {
    return (
      <View style={[shared.card, { marginTop: 12 }]} testID="home-waiting-status">
        {!idleLoaded ? (
          <Text style={shared.status}>Checking who's around…</Text>
        ) : (
          <WaitingList
            players={idlePlayers}
            heading={
              idlePlayers.length === 1
                ? '1 player is waiting to play'
                : `${idlePlayers.length} players are waiting to play`
            }
            empty="Nobody is waiting to play right now. Put yourself on the list and you'll be the first anyone sees."
          />
        )}
        <MenuButton
          label={availableLabel}
          secondary
          style={cardButton}
          onPress={() => setAvailable(true)}
        />
      </View>
    );
  }

  // Your own row is dropped: the list answers "who might I end up playing
  // with", and you are not one of them.
  const others = livePlayers.filter((p) => p.playerId !== session.userId);

  if (status === 'open') {
    return (
      <View style={[shared.card, { marginTop: 12 }]} testID="home-waiting-status">
        <View testID="waiting-status-open">
          <Text style={{ color: colors.success, fontWeight: '600', marginBottom: 4 }}>
            You're waiting to play
          </Text>
          <Text style={[shared.status, { marginTop: 0, marginBottom: 10 }]}>
            Anyone opening a table can pick you up — they don't need a code from you.
          </Text>
          <WaitingList
            players={others}
            heading={
              others.length === 1
                ? '1 other player is waiting too'
                : `${others.length} other players are waiting too`
            }
            empty="Nobody else is waiting yet. Hosts can still see you and invite you."
          />
        </View>
        <MenuButton
          label={stopLabel}
          secondary
          style={cardButton}
          onPress={() => setAvailable(false)}
        />
      </View>
    );
  }

  if (status === 'busy') {
    return (
      <View style={[shared.card, { marginTop: 12 }]} testID="home-waiting-status">
        <View testID="waiting-status-busy">
          <Text style={{ color: colors.gold, fontWeight: '600', marginBottom: 4 }}>
            {reasonText('SERVER_BUSY')}
          </Text>
          <Text style={shared.status}>
            Attempt {attempts}. The server is not taking new waiting-room connections right now.
          </Text>
        </View>
        <MenuButton label="Try again now" secondary style={cardButton} onPress={retryNow} />
        <Pressable style={{ marginTop: 10 }} onPress={() => setAvailable(false)}>
          <Text style={shared.status}>{stopLabel}</Text>
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
        <MenuButton label="Try again now" secondary style={cardButton} onPress={retryNow} />
        <Pressable style={{ marginTop: 10 }} onPress={() => setAvailable(false)}>
          <Text style={shared.status}>{stopLabel}</Text>
        </Pressable>
      </View>
    );
  }

  return (
    <View style={[shared.card, { marginTop: 12 }]} testID="home-waiting-status">
      <View testID="waiting-status-connecting">
        <ActivityIndicator color={colors.accent} style={{ marginBottom: 8 }} />
        <Text style={shared.status}>Adding you to the waiting list…</Text>
        <Text style={[shared.status, { marginTop: 4, fontSize: 12 }]}>
          If this doesn't finish in a few seconds, check the server address below is reachable
          from this device.
        </Text>
        <Text style={[shared.status, { marginTop: 4 }]}>Server: {ZOLIK_BASE_URL}</Text>
      </View>
      <Pressable style={{ marginTop: 10 }} onPress={() => setAvailable(false)}>
        <Text style={shared.status}>{stopLabel}</Text>
      </Pressable>
    </View>
  );
}

/**
 * The two halves of the toggle, written out once each.
 *
 * "Find players" — what this button used to say — named a screen to go
 * looking at, and there is no such screen: the tap does not search for
 * anybody, it publishes *you*, to everybody. Naming that effect instead
 * settles the one question the old card left a person holding, which was what
 * pressing it was about to do to them.
 */
const availableLabel = 'Make me available to play';
const stopLabel = 'Stop waiting';

/** A MenuButton sitting last inside a card, where the card supplies the
 *  bottom margin the menu stack normally wants. */
const cardButton = { marginTop: 12, marginBottom: 0 } as const;

/** How many faces fit before the card costs more room than it earns. */
const maxNamesShown = 8;

/**
 * The pool as people rather than as a number.
 *
 * Each is drawn under the face they are waiting behind, which is the face
 * they will still be sitting behind a moment later — so spotting a friend in
 * the list is possible at all. The count this replaces could tell you the
 * room was not empty, and nothing else about it.
 */
function WaitingList({
  players,
  heading,
  empty,
}: {
  players: WaitingPlayer[];
  heading: string;
  empty: string;
}) {
  if (players.length === 0) {
    return <Text style={[shared.status, { marginTop: 0 }]}>{empty}</Text>;
  }

  const shown = players.slice(0, maxNamesShown);
  return (
    <View testID="home-waiting-list">
      <Text style={{ color: colors.text, fontWeight: '600', marginBottom: 8 }}>{heading}</Text>
      {shown.map((p) => (
        <View
          key={p.playerId}
          testID={`home-waiting-player-${p.playerId}`}
          style={{ flexDirection: 'row', alignItems: 'center', gap: 8, marginBottom: 6 }}
        >
          <Avatar spec={avatarFor(p.playerId, false, p.avatar)} size={24} />
          <Text style={{ color: colors.text, flexShrink: 1 }} numberOfLines={1}>
            {p.username}
            {p.isGuest ? ' (guest)' : ''}
          </Text>
        </View>
      ))}
      {players.length > shown.length ? (
        <Text style={[shared.status, { marginTop: 2 }]}>+{players.length - shown.length} more</Text>
      ) : null}
    </View>
  );
}
