import { router, useLocalSearchParams } from 'expo-router';
import { useCallback, useEffect, useState } from 'react';
import { Pressable, ScrollView, Text, View } from 'react-native';

import type { MatchState } from '@/src/api/matchTypes';
import type { WaitingPlayer } from '@/src/api/types';
import { Avatar } from '@/src/components/avatars/Avatar';
import { avatarFor } from '@/src/components/avatars/catalogue';
import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { formatApiError } from '@/src/lib/apiError';
import { colors, shared } from '@/src/theme';

/**
 * The host's table before it starts: who is seated, who can be pulled in, and
 * the button that deals.
 *
 * This is the second half of what used to be the "new game" screen, with the
 * rummy taken out. That screen decided the ruleset *and* ran the lobby, and
 * every knob on it was a Žolíky knob; choosing the game now happens in the
 * picker, which reads `/modules`, and what is left here is true of any game:
 * a join code, a roster, bots, and the waiting room.
 */
export default function TableScreen() {
  const { client, session } = useSession();
  const { matchId } = useLocalSearchParams<{ matchId: string }>();

  const [state, setState] = useState<MatchState | null>(null);
  const [waiting, setWaiting] = useState<WaitingPlayer[]>([]);
  const [invitingId, setInvitingId] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  const id = String(matchId ?? '');
  const isHost = !!state?.hostId && state.hostId === session?.userId;

  const poll = useCallback(async () => {
    if (!id) return;
    try {
      const m = await client.getMatch(id, session?.userId);
      setState(m);
      if (m.status !== 'lobby') router.replace(`/match/${id}`);
    } catch (e) {
      setError(formatApiError(e, 'Could not read the table'));
    }
    // Best-effort: a host who cannot currently see the waiting room should
    // still be able to run their table. Its absence is not an error worth
    // showing.
    try {
      setWaiting(await client.getWaitingLobby());
    } catch {
      /* the waiting room is optional infrastructure */
    }
  }, [client, id, session?.userId]);

  useEffect(() => {
    if (!id) return;
    poll();
    const t = setInterval(poll, 2000);
    return () => clearInterval(t);
  }, [id, poll]);

  async function invite(playerId: string) {
    setInvitingId(playerId);
    setError('');
    try {
      await client.invitePlayer(id, playerId);
      await poll();
    } catch (e) {
      setError(formatApiError(e, 'Invite failed'));
    } finally {
      setInvitingId('');
    }
  }

  async function addBot() {
    setBusy(true);
    setError('');
    try {
      await client.addBot(id);
      await poll();
    } catch (e) {
      setError(formatApiError(e, 'Could not add a bot'));
    } finally {
      setBusy(false);
    }
  }

  async function start() {
    setBusy(true);
    setError('');
    try {
      await client.startMatch(id);
      router.replace(`/match/${id}`);
    } catch (e) {
      setError(formatApiError(e, 'Could not start'));
      setBusy(false);
    }
  }

  const players = state?.players ?? [];
  const seatedIds = players.map((p) => p.id);
  const available = waiting.filter((p) => !seatedIds.includes(p.playerId));

  return (
    <Screen title="Your table" scroll>
      <ScrollView testID="table-screen">
        <Text testID="table-module" style={shared.status}>
          {state?.moduleId ?? '…'}
          {state?.variation ? ` · ${state.variation}` : ''}
        </Text>

        {state?.joinCode ? (
          <Text style={{ color: colors.text, fontSize: 18, marginTop: 8 }}>
            Join code: <Text testID="table-join-code" style={{ fontWeight: '700' }}>{state.joinCode}</Text>
          </Text>
        ) : null}

        <Text style={[shared.status, { marginTop: 12 }]}>Players ({players.length})</Text>
        {players.map((p, i) => (
          <Text key={p.id} testID={`seated-${p.id}`} style={{ color: colors.text, marginBottom: 4 }}>
            {i + 1}. {p.name}
            {p.isAI ? ' 🤖' : ''}
            {p.id === state?.hostId ? ' ★' : ''}
          </Text>
        ))}

        {error ? <Text style={shared.error}>{error}</Text> : null}

        {isHost ? (
          <>
            <WaitingPlayersPanel
              available={available}
              invitingId={invitingId}
              onInvite={invite}
            />
            <Pressable testID="table-add-bot" style={shared.button} onPress={addBot} disabled={busy}>
              <Text style={shared.buttonText}>Add a bot</Text>
            </Pressable>
            <Pressable testID="table-start" style={shared.button} onPress={start} disabled={busy}>
              <Text style={shared.buttonText}>Start</Text>
            </Pressable>
          </>
        ) : (
          <Text style={shared.status}>Waiting for the host to start…</Text>
        )}
      </ScrollView>
    </Screen>
  );
}

/**
 * The host's view into the waiting room: people available right now, seatable
 * with one tap and no join code. Deliberately not shown to a non-host — invite
 * is a host-only action on the server, so a player who cannot use it has no
 * reason to see who is waiting.
 */
function WaitingPlayersPanel({
  available,
  invitingId,
  onInvite,
}: {
  available: WaitingPlayer[];
  invitingId: string;
  onInvite: (playerId: string) => void;
}) {
  return (
    <View style={[shared.card, { marginTop: 12, marginBottom: 12 }]} testID="waiting-players-panel">
      <Text style={{ color: colors.text, fontWeight: '700', fontSize: 14, marginBottom: 10 }}>
        Waiting to play ({available.length})
      </Text>
      {available.length === 0 ? (
        // Rendered explicitly rather than hiding the whole panel: a host
        // seeing nothing here should be able to tell "no one is waiting right
        // now" from "this is broken" at a glance, especially when comparing
        // notes with someone on a second device who insists they are waiting.
        <Text style={shared.status}>
          No one is waiting right now. Anyone who opens “Find players” shows up here.
        </Text>
      ) : (
        available.map((p) => (
          <View
            key={p.playerId}
            testID={`waiting-player-${p.playerId}`}
            style={{
              flexDirection: 'row',
              alignItems: 'center',
              justifyContent: 'space-between',
              marginBottom: 8,
            }}
          >
            {/* The face they are waiting under, which is the face they will
                be sitting behind a moment later — the host picks a person
                out of the pool, not a row of text. */}
            <View style={{ flexDirection: 'row', alignItems: 'center', gap: 8, flexShrink: 1 }}>
              <Avatar spec={avatarFor(p.playerId, false, p.avatar)} size={28} />
              <Text style={{ color: colors.text }} numberOfLines={1}>
                {p.username}
                {p.isGuest ? ' (guest)' : ''}
              </Text>
            </View>
            <Pressable
              testID={`invite-${p.playerId}`}
              style={[shared.button, { marginBottom: 0, paddingVertical: 8, paddingHorizontal: 14 }]}
              onPress={() => onInvite(p.playerId)}
              disabled={invitingId !== ''}
            >
              <Text style={shared.buttonText}>{invitingId === p.playerId ? '…' : 'Invite'}</Text>
            </Pressable>
          </View>
        ))
      )}
    </View>
  );
}
