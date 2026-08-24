import { router, useLocalSearchParams } from 'expo-router';
import { useCallback, useEffect, useState } from 'react';
import { Pressable, Text, TextInput, View } from 'react-native';

import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import type { LobbyPlayer } from '@/src/api/types';
import { colors, shared } from '@/src/theme';

export default function JoinLobbyScreen() {
  const { client } = useSession();
  // A host's invite lands the player here with the game already decided —
  // they were seated server-side the moment the host picked them, so there
  // is no code to type and nothing left to do but watch the lobby fill up,
  // same as anyone who joined by code.
  const { gameId: invitedGameId } = useLocalSearchParams<{ gameId?: string }>();
  const [code, setCode] = useState('');
  const [gameId, setGameId] = useState(invitedGameId ?? '');
  const [players, setPlayers] = useState<LobbyPlayer[]>([]);
  const [error, setError] = useState('');
  const [joined, setJoined] = useState(Boolean(invitedGameId));

  const poll = useCallback(async () => {
    if (!gameId) return;
    try {
      const info = await client.getLobby(gameId);
      setPlayers(info.players);
      if (info.status === 'active') {
        router.replace(`/game/${gameId}`);
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Poll failed');
    }
  }, [client, gameId]);

  useEffect(() => {
    if (!gameId) return;
    poll();
    const t = setInterval(poll, 2000);
    return () => clearInterval(t);
  }, [gameId, poll]);

  async function join() {
    setError('');
    const trimmed = code.trim();
    if (!trimmed) {
      setError('Enter a join code or game id');
      return;
    }
    try {
      const id = await client.joinGame(trimmed);
      setGameId(id);
      setJoined(true);
      await poll();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Join failed');
    }
  }

  return (
    <Screen title="Join game" scroll>
      {!joined ? (
        <>
          <TextInput
            style={shared.input}
            placeholder="Join code or game ID"
            placeholderTextColor="#8b9cb3"
            autoCapitalize="characters"
            value={code}
            onChangeText={setCode}
          />
          {error ? <Text style={shared.error}>{error}</Text> : null}
          <Pressable style={shared.button} onPress={join}>
            <Text style={shared.buttonText}>Join</Text>
          </Pressable>
        </>
      ) : (
        <View testID="lobby-joined">
          <Text style={shared.status}>Joined — waiting for host to start</Text>
          {error ? <Text style={shared.error}>{error}</Text> : null}
          <Text style={[shared.status, { marginTop: 12 }]}>
            Players ({players.length}/8)
          </Text>
          {players.map((p, i) => (
            <Text key={p.id} testID={`lobby-player-${p.id}`} style={{ color: colors.text, marginBottom: 4 }}>
              {i + 1}. {p.name}
              {p.isAI ? ' 🤖' : ''}
            </Text>
          ))}
        </View>
      )}
    </Screen>
  );
}
