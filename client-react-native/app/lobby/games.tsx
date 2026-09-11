import { router } from 'expo-router';
import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';

import type { MatchModule } from '@/src/api/matchTypes';
import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { formatApiError } from '@/src/lib/apiError';
import { orderModules } from '@/src/lib/gameOrder';
import { loadGameSetup, saveGameSetup } from '@/src/lib/gameSetupStore';
import { factText, label } from '@/src/lib/labels';
import { colors } from '@/src/theme';
import { choiceLabel, moduleLabel, optionLabel, variationLabel } from '@/src/lib/gameLabels';
import { t } from '@/src/lib/i18n';

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
  const { client, session } = useSession();
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
        const fetched = await client.modules();
        if (cancelled) return;

        // Personalize the order by what this player actually plays; a
        // player with no match history yet has nothing to personalize with,
        // so `playCounts` stays undefined and orderModules falls back to the
        // general popularity ranking on its own.
        let playCounts: Record<string, number> | undefined;
        try {
          const stats = await client.getStats();
          playCounts = {};
          for (const [id, tally] of Object.entries(stats.byModule ?? {})) {
            playCounts[id] = tally.matches;
          }
        } catch {
          // Stats are a personalization nicety, not a requirement — the
          // popularity default below still gives a sensible order.
        }
        if (cancelled) return;

        const list = orderModules(fetched, playCounts);
        setModules(list);

        const v: Record<string, string> = {};
        const o: Record<string, Record<string, number>> = {};
        const b: Record<string, number> = {};
        for (const m of list) {
          const saved = await loadGameSetup(m.id);
          // Only trust a saved pick that still matches this module's current
          // descriptor: a variation or option the server has since renamed or
          // dropped should fall back to the module's own default rather than
          // resurrect a stale choice.
          const savedVariation =
            saved?.variation && m.variations?.some((x) => x.id === saved.variation)
              ? saved.variation
              : undefined;
          const variationId = savedVariation ?? m.variations?.[0]?.id;
          const spec = m.variations?.find((x) => x.id === variationId);
          if (variationId) {
            v[m.id] = variationId;
            o[m.id] = { ...(spec?.defaults ?? {}) };
            for (const opt of m.options ?? []) {
              if (saved?.options && opt.name in saved.options) {
                o[m.id][opt.name] = saved.options[opt.name];
              }
            }
          }
          b[m.id] = saved?.bots ?? botCount(m);
        }
        setVariation(v);
        setOptions(o);
        setBots(b);
      } catch (e) {
        if (!cancelled) setError(formatApiError(e));
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [client]);

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
        setError(t('lobby.games.signInFirst'));
        return;
      }
      setBusy(mod.id);
      setError('');
      try {
        // Remember this pick for next time, regardless of how the match turns
        // out — it is a setup-screen preference, not a fact about the match.
        await saveGameSetup(mod.id, {
          variation: variation[mod.id],
          options: options[mod.id],
          bots: bots[mod.id],
        });

        const { matchId } = await client.createMatch(
          mod.id,
          variation[mod.id],
          options[mod.id] ?? {},
        );

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
          await client.addBot(matchId);
        }
        await client.startMatch(matchId);
        router.push(`/match/${matchId}`);
      } catch (e) {
        setError(formatApiError(e));
      } finally {
        setBusy('');
      }
    },
    [client, session, variation, options, bots],
  );

  if (!modules.length && !error) {
    return (
      <Screen title={t('nav.games')}>
        <ActivityIndicator color={colors.accent} />
      </Screen>
    );
  }

  return (
    <Screen title={t('nav.games')} subtitle={t('lobby.games.subtitle')}>
      <ScrollView testID="games-list">
        {error ? (
          <Text testID="games-error" style={styles.error}>
            {error}
          </Text>
        ) : null}

        {modules.map((mod) => (
          <View key={mod.id} style={styles.card} testID={`module-${mod.id}`}>
            <View style={styles.headerRow}>
              <View>
                <Text style={styles.name}>{moduleLabel(mod)}</Text>
                <Text style={styles.meta}>
                  {mod.minPlayers === mod.maxPlayers
                    ? t('lobby.games.players', { n: mod.minPlayers })
                    : t('lobby.games.playerRange', { min: mod.minPlayers, max: mod.maxPlayers })}
                </Text>
              </View>
              <Pressable
                testID={`rules-${mod.id}`}
                onPress={() =>
                  // One continuous template literal, not a concatenation:
                  // expo-router's typed routes validate an href against its
                  // known route patterns via a template-literal type, which
                  // only sees through an actual template literal — a `+`
                  // chain of them infers as plain `string` and fails to typecheck.
                  router.push(
                    `/rules?moduleId=${encodeURIComponent(mod.id)}&variation=${encodeURIComponent(variation[mod.id] ?? '')}&options=${encodeURIComponent(JSON.stringify(options[mod.id] ?? {}))}`,
                  )
                }
                style={styles.rulesLink}
              >
                <Text style={styles.rulesLinkText}>{t('nav.rules')}</Text>
              </Pressable>
            </View>

            {(mod.variations ?? []).length > 1 ? (
              <View style={styles.row}>
                {(mod.variations ?? []).map((v) => (
                  <Pressable
                    key={v.id}
                    testID={`variation-${mod.id}-${v.id}`}
                    onPress={() => pickVariation(mod.id, mod, v.id)}
                    style={[styles.pill, variation[mod.id] === v.id && styles.pillOn]}
                  >
                    <Text style={styles.pillText}>{variationLabel(mod.id, v)}</Text>
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
                <Text style={styles.optionLabel}>{optionLabel(mod.id, opt)}</Text>
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
                        <Text style={styles.pillText}>{choiceLabel(mod.id, opt.name, c)}</Text>
                      </Pressable>
                    );
                  })}
                </View>
              </View>
            ))}

            {botChoices(mod).length > 1 ? (
              <View style={styles.option}>
                <Text style={styles.optionLabel}>{t('lobby.games.bots')}</Text>
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
                    ? t('lobby.games.playBot')
                    : t('lobby.games.playBots', { n: botCount(mod, bots[mod.id]) })}
                </Text>
              </Pressable>
              <Pressable
                testID={`play-friends-${mod.id}`}
                disabled={busy === mod.id}
                onPress={() => start(mod, false)}
                style={[styles.button, styles.secondary, busy === mod.id && styles.buttonBusy]}
              >
                <Text style={styles.buttonText}>{t('lobby.games.openTable')}</Text>
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
  headerRow: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'flex-start' },
  name: { color: colors.text, fontSize: 18, fontWeight: '700' },
  meta: { color: colors.muted, fontSize: 12, marginTop: 2 },
  rulesLink: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 6,
    paddingHorizontal: 10,
    paddingVertical: 5,
  },
  rulesLinkText: { color: colors.accent, fontSize: 12, fontWeight: '700' },
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
