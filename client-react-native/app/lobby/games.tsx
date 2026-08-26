import { router } from 'expo-router';
import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';

import type { MatchModule } from '@/src/api/matchTypes';
import { Screen } from '@/src/components/Screen';
import { ZOLIK_BASE_URL } from '@/src/config';
import { useSession } from '@/src/context/SessionContext';
import { factText, label } from '@/src/lib/labels';
import { colors } from '@/src/theme';

/**
 * The game picker, rendered entirely from `/modules`.
 *
 * This screen has no list of games in it. Adding a fifth is a server-only
 * change — register the module and it appears here with its variations, its
 * options and its player range, because that is what a descriptor is for.
 *
 * The same is true of the options below it: `MELD_MINS` and
 * `DISCARD_LOCK_ROUNDS` were once client constants, which meant adding a knob
 * meant editing a client. Here the form is whatever the module declares.
 *
 * The bot count is the one control on this screen that is not a module option,
 * and deliberately so: how many seats a table has is not a house rule any game
 * declares, it is the descriptor's own player range. So the picker is built
 * from that range, and a game registered tomorrow gets the right one.
 */
export default function GamesScreen() {
  const { session } = useSession();
  const [modules, setModules] = useState<MatchModule[]>([]);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState('');

  // Which variation and options each module is configured with right now,
  // seeded from the variation's own declared defaults.
  const [variation, setVariation] = useState<Record<string, string>>({});
  const [options, setOptions] = useState<Record<string, Record<string, number>>>({});
  // How many bots "Play against bots" seats, per module. Held here rather
  // than derived at the click, because it is a choice the host makes before
  // pressing anything — and, like everything else on this screen, its range
  // comes from the module's own descriptor.
  const [bots, setBots] = useState<Record<string, number>>({});

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const res = await fetch(`${ZOLIK_BASE_URL}/modules`);
        const body = (await res.json()) as { modules: MatchModule[] };
        if (cancelled) return;
        setModules(body.modules ?? []);

        const v: Record<string, string> = {};
        const o: Record<string, Record<string, number>> = {};
        const b: Record<string, number> = {};
        for (const m of body.modules ?? []) {
          const first = m.variations?.[0];
          if (first) {
            v[m.id] = first.id;
            o[m.id] = { ...(first.defaults ?? {}) };
          }
          b[m.id] = botCount(m);
        }
        setVariation(v);
        setOptions(o);
        setBots(b);
      } catch (e) {
        if (!cancelled) setError(String(e));
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const pickVariation = (moduleId: string, mod: MatchModule, id: string) => {
    setVariation((prev) => ({ ...prev, [moduleId]: id }));
    const spec = mod.variations?.find((x) => x.id === id);
    // Re-seed the options from the variation's declared defaults: a variation
    // is a named ruleset, so switching it should move the knobs with it.
    setOptions((prev) => ({ ...prev, [moduleId]: { ...(spec?.defaults ?? {}) } }));
  };

  const start = useCallback(
    async (mod: MatchModule, withBot: boolean) => {
      if (!session?.accessToken) {
        setError('Sign in first');
        return;
      }
      setBusy(mod.id);
      setError('');
      const auth = {
        Authorization: `Bearer ${session.accessToken}`,
        'Content-Type': 'application/json',
      };
      try {
        const created = await fetch(`${ZOLIK_BASE_URL}/matches`, {
          method: 'POST',
          headers: auth,
          body: JSON.stringify({
            moduleId: mod.id,
            variation: variation[mod.id],
            options: options[mod.id] ?? {},
          }),
        });
        if (!created.ok) throw new Error(await created.text());
        const { matchId } = (await created.json()) as { matchId: string };

        if (!withBot) {
          // Open the table and let the host fill it: invite someone out of the
          // waiting room, add bots, then deal.
          router.push(`/lobby/table?matchId=${encodeURIComponent(matchId)}`);
          return;
        }

        // As many bots as the host asked for, clamped to the seats this
        // module actually has. The screen still does not know what a legal
        // table is for any particular game — the descriptor does, and the
        // clamp is the only thing reading it.
        const seats = botCount(mod, bots[mod.id]);
        for (let i = 0; i < seats; i++) {
          await fetch(`${ZOLIK_BASE_URL}/matches/${matchId}/add-bot`, {
            method: 'POST',
            headers: auth,
          });
        }
        const started = await fetch(`${ZOLIK_BASE_URL}/matches/${matchId}/start`, {
          method: 'POST',
          headers: auth,
        });
        if (!started.ok) throw new Error(await started.text());
        router.push(`/match/${matchId}`);
      } catch (e) {
        setError(String(e));
      } finally {
        setBusy('');
      }
    },
    [session, variation, options, bots],
  );

  if (!modules.length && !error) {
    return (
      <Screen title="Games">
        <ActivityIndicator color={colors.accent} />
      </Screen>
    );
  }

  return (
    <Screen title="Games" subtitle="Everything this server can host">
      <ScrollView testID="games-list">
        {error ? (
          <Text testID="games-error" style={styles.error}>
            {error}
          </Text>
        ) : null}

        {modules.map((mod) => (
          <View key={mod.id} style={styles.card} testID={`module-${mod.id}`}>
            <Text style={styles.name}>{mod.label}</Text>
            <Text style={styles.meta}>
              {mod.minPlayers === mod.maxPlayers
                ? `${mod.minPlayers} players`
                : `${mod.minPlayers}–${mod.maxPlayers} players`}
            </Text>

            {(mod.variations ?? []).length > 1 ? (
              <View style={styles.row}>
                {(mod.variations ?? []).map((v) => (
                  <Pressable
                    key={v.id}
                    testID={`variation-${mod.id}-${v.id}`}
                    onPress={() => pickVariation(mod.id, mod, v.id)}
                    style={[styles.pill, variation[mod.id] === v.id && styles.pillOn]}
                  >
                    <Text style={styles.pillText}>{v.label}</Text>
                  </Pressable>
                ))}
              </View>
            ) : null}

            {/* What this ruleset is, in the module's own words. */}
            {(mod.variations ?? [])
              .find((v) => v.id === variation[mod.id])
              ?.summary?.map((f, i) => (
                <Text key={i} style={styles.summary}>
                  · {factText(f)}
                </Text>
              ))}

            {(mod.options ?? []).map((opt) => (
              <View key={opt.name} style={styles.option}>
                <Text style={styles.optionLabel}>{opt.label}</Text>
                <View style={styles.row}>
                  {opt.choices.map((c) => {
                    const on = (options[mod.id] ?? {})[opt.name] === c.value;
                    return (
                      <Pressable
                        key={c.value}
                        testID={`option-${mod.id}-${opt.name}-${c.value}`}
                        onPress={() =>
                          setOptions((prev) => ({
                            ...prev,
                            [mod.id]: { ...(prev[mod.id] ?? {}), [opt.name]: c.value },
                          }))
                        }
                        style={[styles.pill, on && styles.pillOn]}
                      >
                        <Text style={styles.pillText}>{c.label}</Text>
                      </Pressable>
                    );
                  })}
                </View>
              </View>
            ))}

            {botChoices(mod).length > 1 ? (
              <View style={styles.option}>
                <Text style={styles.optionLabel}>Bots</Text>
                <View style={styles.row}>
                  {botChoices(mod).map((n) => {
                    const on = botCount(mod, bots[mod.id]) === n;
                    return (
                      <Pressable
                        key={n}
                        testID={`bots-${mod.id}-${n}`}
                        onPress={() => setBots((prev) => ({ ...prev, [mod.id]: n }))}
                        style={[styles.pill, on && styles.pillOn]}
                      >
                        <Text style={styles.pillText}>{n}</Text>
                      </Pressable>
                    );
                  })}
                </View>
              </View>
            ) : null}

            <View style={styles.actions}>
              <Pressable
                testID={`play-bots-${mod.id}`}
                disabled={busy === mod.id}
                onPress={() => start(mod, true)}
                style={[styles.button, busy === mod.id && styles.buttonBusy]}
              >
                <Text style={styles.buttonText}>
                  {botCount(mod, bots[mod.id]) === 1
                    ? 'Play against a bot'
                    : `Play against ${botCount(mod, bots[mod.id])} bots`}
                </Text>
              </Pressable>
              <Pressable
                testID={`play-friends-${mod.id}`}
                disabled={busy === mod.id}
                onPress={() => start(mod, false)}
                style={[styles.button, styles.secondary, busy === mod.id && styles.buttonBusy]}
              >
                <Text style={styles.buttonText}>Open a table</Text>
              </Pressable>
            </View>
          </View>
        ))}
      </ScrollView>
    </Screen>
  );
}

/**
 * The bot counts a module can be opened with: enough to reach its minimum
 * table at the low end, one short of its maximum at the high end — because
 * one of the seats is the host's.
 */
function botChoices(mod: MatchModule): number[] {
  const out: number[] = [];
  for (let n = Math.max(1, mod.minPlayers - 1); n <= Math.max(1, mod.maxPlayers - 1); n++) {
    out.push(n);
  }
  return out;
}

/**
 * How many bots to seat: what the host picked, clamped to the range above.
 * A clamp rather than a plain read, so a count left over from a module list
 * that reloaded — or a game whose range moved under it — can never post a
 * seat the server is going to refuse.
 */
function botCount(mod: MatchModule, picked?: number): number {
  const choices = botChoices(mod);
  const lo = choices[0];
  const hi = choices[choices.length - 1];
  return Math.min(hi, Math.max(lo, picked ?? lo));
}

const styles = StyleSheet.create({
  card: {
    backgroundColor: colors.surface,
    borderRadius: 12,
    borderWidth: 1,
    borderColor: colors.border,
    padding: 14,
    marginBottom: 12,
  },
  name: { color: colors.text, fontSize: 18, fontWeight: '700' },
  meta: { color: colors.muted, fontSize: 12, marginTop: 2 },
  summary: { color: colors.muted, fontSize: 12, marginTop: 2 },
  row: { flexDirection: 'row', flexWrap: 'wrap', gap: 6, marginTop: 6 },
  option: { marginTop: 8 },
  optionLabel: { color: colors.muted, fontSize: 12, fontWeight: '700' },
  pill: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 6,
    paddingHorizontal: 10,
    paddingVertical: 5,
  },
  pillOn: { borderColor: colors.accent, backgroundColor: colors.accentDim },
  pillText: { color: colors.text, fontSize: 12 },
  actions: { flexDirection: 'row', gap: 8, marginTop: 12 },
  button: {
    backgroundColor: colors.accentButton,
    borderRadius: 8,
    paddingVertical: 10,
    paddingHorizontal: 14,
  },
  secondary: { backgroundColor: colors.muted },
  buttonBusy: { opacity: 0.5 },
  buttonText: { color: colors.onAccent, fontWeight: '700', fontSize: 13 },
  error: { color: colors.danger, marginBottom: 10 },
});
