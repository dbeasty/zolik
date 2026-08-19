import { router } from 'expo-router';
import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, Pressable, Text, View } from 'react-native';

import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import type { LobbyPlayer } from '@/src/api/types';
import { colors, shared } from '@/src/theme';

const DIFFICULTIES = ['easy', 'medium', 'hard'] as const;
const MELD_MINS = [35, 50, 70];
const DISCARD_LOCK_ROUNDS = [1, 2, 3];

export default function CreateLobbyScreen() {
  const { client, session } = useSession();
  const [gameId, setGameId] = useState('');
  const [joinCode, setJoinCode] = useState('');
  const [players, setPlayers] = useState<LobbyPlayer[]>([]);
  const [aiDiff, setAiDiff] = useState<(typeof DIFFICULTIES)[number]>('medium');
  const [initialMin, setInitialMin] = useState(35);
  // Continental Rummy: discard-pile pickup only opens up from round 3.
  const [discardLockRound, setDiscardLockRound] = useState(3);
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
      if (info.initialMeldMinimum != null) setInitialMin(info.initialMeldMinimum);
      if (info.discardDrawMinRound != null) setDiscardLockRound(info.discardDrawMinRound);
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
        const { gameId: id, joinCode: code } = await client.createGame(initialMin);
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

  async function cycleMeldMin() {
    const idx = MELD_MINS.indexOf(initialMin);
    const next = MELD_MINS[(idx + 1) % MELD_MINS.length];
    setInitialMin(next);
    try {
      await client.updateGameSettings(gameId, { initialMeldMinimum: next });
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Update failed');
    }
  }

  async function cycleDiscardLock() {
    const idx = DISCARD_LOCK_ROUNDS.indexOf(discardLockRound);
    const next = DISCARD_LOCK_ROUNDS[(idx + 1) % DISCARD_LOCK_ROUNDS.length];
    setDiscardLockRound(next);
    try {
      await client.updateGameSettings(gameId, { discardDrawMinRound: next });
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Update failed');
    }
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
              style={[
                shared.button,
                d === aiDiff ? null : shared.buttonSecondary,
                { flex: 1, marginBottom: 0 },
              ]}
              onPress={() => setAiDiff(d)}
            >
              <Text style={d === aiDiff ? shared.buttonText : shared.buttonTextSecondary}>{d}</Text>
            </Pressable>
          ))}
        </View>
        <View style={[shared.card, { marginTop: 8, paddingVertical: 12 }]}>
          <Text style={{ color: colors.text, fontWeight: '700', fontSize: 14, marginBottom: 10 }}>
            Continental Rummy rules
          </Text>
          <View style={{ flexDirection: 'row', gap: 8 }}>
            <RuleChip label="Meld value" value={String(initialMin)} onPress={isHost ? cycleMeldMin : undefined} />
            <RuleChip
              label="Discard pickup"
              value={`R${discardLockRound}`}
              onPress={isHost ? cycleDiscardLock : undefined}
            />
          </View>
        </View>

        {isHost ? (
          <>
            <Pressable style={shared.button} onPress={addAI}>
              <Text style={shared.buttonText}>Add AI</Text>
            </Pressable>
            <Pressable
              style={shared.button}
              onPress={startGame}
              disabled={players.length < 2}
            >
              <Text style={shared.buttonText}>
                Start game {players.length < 2 ? `(need ${2 - players.length} more)` : ''}
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

function RuleChip({
  label,
  value,
  onPress,
}: {
  label: string;
  value: string;
  onPress?: () => void;
}) {
  return (
    <Pressable
      onPress={onPress}
      disabled={!onPress}
      style={{
        flex: 1,
        backgroundColor: colors.surface,
        borderWidth: 1,
        borderColor: colors.border,
        borderRadius: 8,
        paddingVertical: 8,
        paddingHorizontal: 6,
        alignItems: 'center',
        opacity: onPress ? 1 : 0.7,
      }}
    >
      <Text style={{ color: colors.muted, fontSize: 11 }}>{label}</Text>
      <Text style={{ color: colors.text, fontSize: 15, fontWeight: '700', marginTop: 2 }}>
        {value}
      </Text>
    </Pressable>
  );
}
