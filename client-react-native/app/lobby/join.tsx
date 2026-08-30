import { router, useLocalSearchParams } from 'expo-router';
import { useCallback, useEffect, useState } from 'react';
import { Pressable, Text, TextInput, View } from 'react-native';

import type { MatchState } from '@/src/api/matchTypes';
import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { formatApiError } from '@/src/lib/apiError';
import { colors, shared } from '@/src/theme';

/**
 * Joining a table somebody else opened, by code or by invitation.
 *
 * Identical for every game, because there is nothing game-specific about
 * waiting in a lobby: a code, a roster, and a host who starts it. The version
 * this replaces polled a Žolíky game and jumped to the Žolíky screen; this one
 * polls a match and hands over to the one screen that plays all of them.
 */
export default function JoinMatchScreen() {
  const { client } = useSession();
  // A host's invite lands the player here with the match already decided —
  // they were seated server-side the moment the host picked them, so there is
  // no code to type and nothing left to do but watch the table fill up, same
  // as anyone who joined by code.
  const { matchId: invitedMatchId } = useLocalSearchParams<{ matchId?: string }>();
  const [code, setCode] = useState('');
  const [matchId, setMatchId] = useState(invitedMatchId ?? '');
  const [state, setState] = useState<MatchState | null>(null);
  const [error, setError] = useState('');

  const poll = useCallback(async () => {
    if (!matchId) return;
    try {
      const m = await client.getMatch(matchId);
      setState(m);
      // The host started it. Everything from here is the shell's job.
      if (m.status !== 'lobby') router.replace(`/match/${matchId}`);
    } catch (e) {
      setError(formatApiError(e, 'Could not read the table'));
    }
  }, [client, matchId]);

  useEffect(() => {
    if (!matchId) return;
    poll();
    const t = setInterval(poll, 2000);
    return () => clearInterval(t);
  }, [matchId, poll]);

  async function join() {
    setError('');
    const trimmed = code.trim();
    if (!trimmed) {
      setError('Enter a join code or match ID');
      return;
    }
    try {
      setMatchId(await client.joinMatch(trimmed));
    } catch (e) {
      setError(formatApiError(e, 'Join failed'));
    }
  }

  if (!matchId) {
    return (
      <Screen title="Join a table" scroll>
        <TextInput
          testID="join-code"
          style={shared.input}
          placeholder="Join code or match ID"
          placeholderTextColor={colors.muted}
          autoCapitalize="characters"
          value={code}
          onChangeText={setCode}
        />
        {error ? <Text style={shared.error}>{error}</Text> : null}
        <Pressable testID="join-submit" style={shared.button} onPress={join}>
          <Text style={shared.buttonText}>Join</Text>
        </Pressable>
      </Screen>
    );
  }

  return (
    <Screen title="Waiting for the host" scroll>
      <Text testID="lobby-joined" style={shared.status}>
        Joined {state?.moduleId ? `a game of ${state.moduleId}` : 'the table'} — waiting to start
      </Text>
      {error ? <Text style={shared.error}>{error}</Text> : null}

      <Text style={[shared.status, { marginTop: 12 }]}>
        Players ({state?.players.length ?? 0})
      </Text>
      {(state?.players ?? []).map((p, i) => (
        <View key={p.id} testID={`lobby-player-${p.id}`}>
          <Text style={{ color: colors.text, marginBottom: 4 }}>
            {i + 1}. {p.name}
            {p.isAI ? ' 🤖' : ''}
          </Text>
        </View>
      ))}
    </Screen>
  );
}
