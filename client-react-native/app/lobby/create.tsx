import { router } from 'expo-router';
import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, Pressable, Text, View } from 'react-native';

import { Screen } from '@/src/components/Screen';
import { useRulesConfig } from '@/src/context/RulesConfigContext';
import { storage, useSession } from '@/src/context/SessionContext';
import type { LobbyPlayer, RulesProfile } from '@/src/api/types';
import { colors, shared } from '@/src/theme';

const DIFFICULTIES = ['easy', 'medium', 'hard'] as const;
const LAST_PROFILE_KEY = 'zolik_last_rules_profile';
const LAST_MELD_MIN_KEY = 'zolik_last_meld_min';
const LAST_DISCARD_LOCK_ROUND_KEY = 'zolik_last_discard_lock_round';

const PROFILES: { value: RulesProfile; label: string }[] = [
  { value: 'continental', label: 'Continental' },
  { value: 'zolik_classic', label: 'Žolík Classic' },
];

async function loadLastProfile(): Promise<RulesProfile> {
  const stored = await storage.getItem(LAST_PROFILE_KEY);
  return stored === 'continental' || stored === 'zolik_classic' ? stored : 'zolik_classic';
}

// Meld-min/discard-lock preferences are remembered per rules profile — each
// profile has its own sensible default (e.g. 35/round-3 for Continental, 0/0
// for Žolík Classic), so a value picked while playing one profile must never
// leak into a freshly created game under a different profile.
function scopedKey(key: string, profile: RulesProfile): string {
  return `${key}:${profile}`;
}

async function loadLastNumericSetting(
  key: string,
  allowed: readonly number[],
): Promise<number | undefined> {
  const stored = await storage.getItem(key);
  if (stored == null) return undefined;
  const parsed = Number(stored);
  return allowed.includes(parsed) ? parsed : undefined;
}

const PROFILE_RULES_TITLE: Record<string, string> = {
  continental: 'Continental Rummy rules',
  zolik_classic: 'Žolík Classic rules',
  custom: 'Custom house rules',
};

export default function CreateLobbyScreen() {
  const { client, session } = useSession();
  const rulesInfo = useRulesConfig();
  // "0" is a UI-only "off" choice, prepended to the server's real options.
  const meldMins = [0, ...rulesInfo.initialMeldMinOptions];
  const discardLockRounds = [0, ...rulesInfo.discardDrawMinRoundOptions];
  const [gameId, setGameId] = useState('');
  const [joinCode, setJoinCode] = useState('');
  const [players, setPlayers] = useState<LobbyPlayer[]>([]);
  const [aiDiff, setAiDiff] = useState<(typeof DIFFICULTIES)[number]>('medium');
  const [profile, setProfile] = useState<RulesProfile>('zolik_classic');
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
      if (info.rulesProfile != null) setProfile(info.rulesProfile);
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
        const lastProfile = await loadLastProfile();
        const lastMeldMin = await loadLastNumericSetting(
          scopedKey(LAST_MELD_MIN_KEY, lastProfile),
          meldMins,
        );
        const lastDiscardLockRound = await loadLastNumericSetting(
          scopedKey(LAST_DISCARD_LOCK_ROUND_KEY, lastProfile),
          discardLockRounds,
        );
        if (cancelled) return;
        setProfile(lastProfile);
        if (lastMeldMin != null) setInitialMin(lastMeldMin);
        if (lastDiscardLockRound != null) setDiscardLockRound(lastDiscardLockRound);
        const { gameId: id, joinCode: code } = await client.createGame(
          lastProfile,
          lastMeldMin,
          lastDiscardLockRound,
        );
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

  async function selectProfile(next: RulesProfile) {
    if (next === profile) return;
    setProfile(next);
    try {
      await client.updateGameSettings(gameId, { rulesProfile: next });
      await storage.setItem(LAST_PROFILE_KEY, next);
      // The server resets initialMeldMinimum/discardDrawMinRound to the new
      // profile's defaults — pick those up on the next poll rather than
      // guessing them here.
      await poll();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Update failed');
    }
  }

  async function cycleMeldMin() {
    const idx = meldMins.indexOf(initialMin);
    const next = meldMins[(idx + 1) % meldMins.length];
    setInitialMin(next);
    try {
      await client.updateGameSettings(gameId, { initialMeldMinimum: next });
      await storage.setItem(scopedKey(LAST_MELD_MIN_KEY, profile), String(next));
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Update failed');
    }
  }

  async function cycleDiscardLock() {
    const idx = discardLockRounds.indexOf(discardLockRound);
    const next = discardLockRounds[(idx + 1) % discardLockRounds.length];
    setDiscardLockRound(next);
    try {
      await client.updateGameSettings(gameId, { discardDrawMinRound: next });
      await storage.setItem(scopedKey(LAST_DISCARD_LOCK_ROUND_KEY, profile), String(next));
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
      <Text style={shared.status}>Players ({players.length}/{rulesInfo.maxPlayers})</Text>
      {players.map((p, i) => (
        <Text key={p.id} style={{ color: colors.text, marginBottom: 4 }}>
          {i + 1}. {p.name}
          {p.id === session?.userId ? ' (you)' : ''}
          {p.isAI ? ' 🤖' : ''}
        </Text>
      ))}

      <View style={{ marginTop: 16 }}>
        <Text style={shared.status}>Rules: {PROFILES.find((p) => p.value === profile)?.label ?? profile}</Text>
        <View style={{ flexDirection: 'row', gap: 8, marginVertical: 8 }}>
          {PROFILES.map((p) => (
            <Pressable
              key={p.value}
              style={[
                shared.button,
                p.value === profile ? null : shared.buttonSecondary,
                { flex: 1, marginBottom: 0 },
              ]}
              disabled={!isHost}
              onPress={() => selectProfile(p.value)}
            >
              <Text style={p.value === profile ? shared.buttonText : shared.buttonTextSecondary}>
                {p.label}
              </Text>
            </Pressable>
          ))}
        </View>

        <Text style={shared.status}>
          AI difficulty: {aiDiff.charAt(0).toUpperCase() + aiDiff.slice(1)}
        </Text>
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
              <Text style={d === aiDiff ? shared.buttonText : shared.buttonTextSecondary}>
                {d.charAt(0).toUpperCase() + d.slice(1)}
              </Text>
            </Pressable>
          ))}
        </View>
        <View style={[shared.card, { marginTop: 8, paddingVertical: 12 }]}>
          <Text style={{ color: colors.text, fontWeight: '700', fontSize: 14, marginBottom: 10 }}>
            {PROFILE_RULES_TITLE[profile] ?? 'House rules'}
          </Text>
          <View style={{ flexDirection: 'row', gap: 8 }}>
            <RuleChip
              label="Meld value"
              value={initialMin > 0 ? String(initialMin) : 'Off'}
              onPress={isHost ? cycleMeldMin : undefined}
            />
            <RuleChip
              label="Discard pickup"
              value={discardLockRound > 1 ? `Round ${discardLockRound}` : 'Open'}
              onPress={isHost ? cycleDiscardLock : undefined}
            />
          </View>
          {profile === 'zolik_classic' ? (
            <Text style={{ color: colors.muted, fontSize: 11, marginTop: 8 }}>
              13-card deal · 3+ card runs · any card may be taken from the discard pile · at least
              one joker-free run required to go down · a joker can only be discarded to end the
              hand.
            </Text>
          ) : null}
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
