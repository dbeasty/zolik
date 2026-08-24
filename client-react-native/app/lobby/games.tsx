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
        for (const m of body.modules ?? []) {
          const first = m.variations?.[0];
          if (first) {
            v[m.id] = first.id;
            o[m.id] = { ...(first.defaults ?? {}) };
          }
        }
        setVariation(v);
        setOptions(o);
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

        // Enough bots to reach the module's own minimum. The screen does not
        // know what that number is for any particular game.
        for (let i = 1; i < mod.minPlayers; i++) {
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
    [session, variation, options],
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

            <View style={styles.actions}>
              <Pressable
                testID={`play-bots-${mod.id}`}
                disabled={busy === mod.id}
                onPress={() => start(mod, true)}
                style={[styles.button, busy === mod.id && styles.buttonBusy]}
              >
                <Text style={styles.buttonText}>Play against bots</Text>
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
