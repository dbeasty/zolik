import { router } from 'expo-router';
import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, Pressable, Text, View } from 'react-native';

import { Screen } from '@/src/components/Screen';
import { storage, useSession } from '@/src/context/SessionContext';
import type { LobbyPlayer, ModuleDescriptor, OptionSpec } from '@/src/api/types';
import { rulesSummaryLines } from '@/src/lib/cards';
import type { OptionValues } from '@/src/lib/lobbyOptions';
import {
  defaultsFor,
  labelFor,
  lastOptionKey,
  nextChoice,
  restoreChoice,
} from '@/src/lib/lobbyOptions';
import { colors, shared } from '@/src/theme';

/**
 * The new-game lobby, rendered from the module descriptor the server serves at
 * GET /module.
 *
 * This screen used to own four separate copies of rule knowledge: the list of
 * variations, the option space (`MELD_MINS = [0, 35, 50, 70]`,
 * `DISCARD_LOCK_ROUNDS = [0, 1, 2, 3]`), a table of display names, and a
 * hand-written paragraph restating one profile's constants. Adding a knob or a
 * third variation meant editing all of it, and a value the server would reject
 * looked perfectly selectable. Now every one of those comes off the descriptor
 * — see docs/extensibility-plan.md Phase 2.1.
 */

const DIFFICULTIES = ['easy', 'medium', 'hard'] as const;
const LAST_PROFILE_KEY = 'zolik_last_rules_profile';

export default function CreateLobbyScreen() {
  const { client, session } = useSession();
  const [gameId, setGameId] = useState('');
  const [joinCode, setJoinCode] = useState('');
  const [players, setPlayers] = useState<LobbyPlayer[]>([]);
  const [aiDiff, setAiDiff] = useState<(typeof DIFFICULTIES)[number]>('medium');
  const [descriptor, setDescriptor] = useState<ModuleDescriptor | null>(null);
  const [profileId, setProfileId] = useState('');
  const [options, setOptions] = useState<OptionValues>({});
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(true);

  const isHost = Boolean(
    session?.userId && players.length > 0 && players[0].id === session.userId,
  );
  const profile = descriptor?.profiles.find((p) => p.id === profileId);
  const minPlayers = descriptor?.minPlayers ?? 2;

  const poll = useCallback(async () => {
    if (!gameId) return;
    try {
      const info = await client.getLobby(gameId);
      setPlayers(info.players);
      if (info.rulesProfile != null) setProfileId(info.rulesProfile);
      // The lobby endpoint reports what the server actually resolved, which is
      // the authority — a rejected or clamped value corrects itself here
      // rather than lingering in the UI.
      setOptions((prev) => ({
        ...prev,
        ...(info.initialMeldMinimum != null ? { initialMeldMinimum: info.initialMeldMinimum } : {}),
        ...(info.discardDrawMinRound != null
          ? { discardDrawMinRound: info.discardDrawMinRound }
          : {}),
      }));
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
        const desc = await client.getModuleDescriptor();
        if (cancelled) return;
        setDescriptor(desc);

        // Restore the host's last choices, but only where the server still
        // declares them: a variation or a value that has been retired must not
        // resurrect itself from this device's storage.
        const storedProfile = await storage.getItem(LAST_PROFILE_KEY);
        const chosen =
          desc.profiles.find((p) => p.id === storedProfile) ?? desc.profiles[0];

        const values = defaultsFor(chosen, desc.options);
        for (const o of desc.options) {
          const restored = restoreChoice(o, await storage.getItem(lastOptionKey(o.name)));
          if (restored != null) values[o.name] = restored;
        }
        if (cancelled) return;

        setProfileId(chosen?.id ?? '');
        setOptions(values);

        const { gameId: id, joinCode: code } = await client.createGame(chosen?.id, values);
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

  async function selectProfile(next: string) {
    if (next === profileId || !descriptor) return;
    setProfileId(next);
    // Switching variation resets every knob to that profile's own defaults —
    // server-side, and mirrored here so the chips do not show the previous
    // profile's numbers until the next poll lands.
    setOptions(defaultsFor(descriptor.profiles.find((p) => p.id === next), descriptor.options));
    try {
      await client.updateGameSettings(gameId, { rulesProfile: next });
      await storage.setItem(LAST_PROFILE_KEY, next);
      await poll();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Update failed');
    }
  }

  async function cycleOption(option: OptionSpec) {
    const next = nextChoice(option, options[option.name]);
    setOptions((prev) => ({ ...prev, [option.name]: next }));
    try {
      await client.updateGameSettings(gameId, { [option.name]: next });
      await storage.setItem(lastOptionKey(option.name), String(next));
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

  // The ruleset as it currently stands: the profile's defaults with this
  // lobby's overrides on top. Rendered by the same summary function the
  // in-game rules panel uses, so the lobby and the table never disagree.
  const previewRules = profile ? { ...profile.rules, ...options } : undefined;

  return (
    <Screen title="Lobby" subtitle={joinCode ? `Join code: ${joinCode}` : undefined} scroll>
      {error ? <Text style={shared.error}>{error}</Text> : null}
      <Text style={shared.status}>
        Players ({players.length}/{descriptor?.maxPlayers ?? 8})
      </Text>
      {players.map((p, i) => (
        <Text key={p.id} style={{ color: colors.text, marginBottom: 4 }}>
          {i + 1}. {p.name}
          {p.id === session?.userId ? ' (you)' : ''}
          {p.isAI ? ' 🤖' : ''}
        </Text>
      ))}

      <View style={{ marginTop: 16 }}>
        <Text style={shared.status}>Rules: {profile?.label ?? profileId}</Text>
        <View style={{ flexDirection: 'row', gap: 8, marginVertical: 8 }}>
          {(descriptor?.profiles ?? []).map((p) => (
            <Pressable
              key={p.id}
              testID={`profile-${p.id}`}
              style={[
                shared.button,
                p.id === profileId ? null : shared.buttonSecondary,
                { flex: 1, marginBottom: 0 },
              ]}
              disabled={!isHost}
              onPress={() => selectProfile(p.id)}
            >
              <Text style={p.id === profileId ? shared.buttonText : shared.buttonTextSecondary}>
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
            {profile ? `${profile.label} rules` : 'House rules'}
          </Text>
          <View style={{ flexDirection: 'row', gap: 8 }}>
            {(descriptor?.options ?? []).map((o) => (
              <RuleChip
                key={o.name}
                testID={`option-${o.name}`}
                label={o.label}
                value={labelFor(o, options[o.name])}
                onPress={isHost ? () => cycleOption(o) : undefined}
              />
            ))}
          </View>
          {previewRules
            ? rulesSummaryLines(previewRules, profile?.contract).map((line) => (
                <Text
                  key={line.label}
                  style={{ color: colors.muted, fontSize: 11, marginTop: 6 }}
                >
                  {line.label}: {line.value}
                </Text>
              ))
            : null}
        </View>

        {isHost ? (
          <>
            <Pressable style={shared.button} onPress={addAI}>
              <Text style={shared.buttonText}>Add AI</Text>
            </Pressable>
            <Pressable
              style={shared.button}
              onPress={startGame}
              disabled={players.length < minPlayers}
            >
              <Text style={shared.buttonText}>
                Start game{' '}
                {players.length < minPlayers ? `(need ${minPlayers - players.length} more)` : ''}
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
  testID,
}: {
  label: string;
  value: string;
  onPress?: () => void;
  testID?: string;
}) {
  return (
    <Pressable
      testID={testID}
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
