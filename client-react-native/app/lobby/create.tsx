import { router } from 'expo-router';
import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, Pressable, Text, View } from 'react-native';

import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import type { LobbyPlayer } from '@/src/api/types';
import { colors, shared } from '@/src/theme';

const DIFFICULTIES = ['easy', 'medium', 'hard'] as const;
const MELD_MINS = [0, 35, 50];

export default function CreateLobbyScreen() {
  const { client, session } = useSession();
  const [gameId, setGameId] = useState('');
  const [joinCode, setJoinCode] = useState('');
  const [players, setPlayers] = useState<LobbyPlayer[]>([]);
  const [aiDiff, setAiDiff] = useState<(typeof DIFFICULTIES)[number]>('medium');
  const [initialMin, setInitialMin] = useState(35);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(true);

  const isHost = Boolean(
    session?.userId && players.length > 0 && players[0].id === session.userId,
  );

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
    let cancelled = false;
    (async () => {
      try {
        const min = initialMin > 0 ? initialMin : undefined;
        const { gameId: id, joinCode: code } = await client.createGame(min);
        if (cancelled) return;
        setGameId(id);
        setJoinCode(code);
        setBusy(false);
      } catch (e) {
        if (!cancelled) {
          setError(e instanceof Error ? e.message : 'Create failed');
          setBusy(false);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- run once on mount
  }, []);

  useEffect(() => {
    if (!gameId) return;
    poll();
    const t = setInterval(poll, 2000);
    return () => clearInterval(t);
  }, [gameId, poll]);

  async function addAI() {
    setError('');
    try {
      await client.addAI(gameId, aiDiff);
      await poll();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Add AI failed');
    }
  }

  async function startGame() {
    setError('');
    try {
      await client.startGame(gameId);
      await poll();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Start failed');
    }
  }

  function cycleMeldMin() {
    const idx = MELD_MINS.indexOf(initialMin);
    setInitialMin(MELD_MINS[(idx + 1) % MELD_MINS.length]);
  }

  if (busy) {
    return (
      <Screen title="New game">
        <ActivityIndicator color={colors.accent} />
      </Screen>
    );
  }

  return (
    <Screen title="Lobby" subtitle={joinCode ? `Join code: ${joinCode}` : undefined} scroll>
      {error ? <Text style={shared.error}>{error}</Text> : null}
      <Text style={shared.status}>Players ({players.length}/8)</Text>
      {players.map((p, i) => (
        <Text key={p.id} style={{ color: colors.text, marginBottom: 4 }}>
          {i + 1}. {p.name}
          {p.id === session?.userId ? ' (you)' : ''}
          {p.isAI ? ' 🤖' : ''}
        </Text>
      ))}

      <View style={{ marginTop: 16 }}>
        <Text style={shared.status}>AI difficulty: {aiDiff}</Text>
        <View style={{ flexDirection: 'row', gap: 8, marginVertical: 8 }}>
          {DIFFICULTIES.map((d) => (
            <Pressable
              key={d}
              style={[shared.button, shared.buttonSecondary, { flex: 1, marginBottom: 0 }]}
              onPress={() => setAiDiff(d)}
            >
              <Text style={shared.buttonTextSecondary}>{d}</Text>
            </Pressable>
          ))}
        </View>
        <Pressable style={[shared.button, shared.buttonSecondary]} onPress={cycleMeldMin}>
          <Text style={shared.buttonTextSecondary}>
            Initial meld minimum: {initialMin || 'default'}
          </Text>
        </Pressable>
        {isHost ? (
          <>
            <Pressable style={shared.button} onPress={addAI}>
              <Text style={shared.buttonText}>Add AI</Text>
            </Pressable>
            <Pressable
              style={shared.button}
              onPress={startGame}
              disabled={players.length < 4}
            >
              <Text style={shared.buttonText}>
                Start game {players.length < 4 ? `(need ${4 - players.length} more)` : ''}
              </Text>
            </Pressable>
          </>
        ) : (
          <Text style={shared.status}>Waiting for host to start…</Text>
        )}
      </View>
    </Screen>
  );
}
