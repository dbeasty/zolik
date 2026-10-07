import { router, useLocalSearchParams } from 'expo-router';
import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';

import type { MatchModule } from '@/src/api/matchTypes';
import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { useLocale } from '@/src/hooks/useLocale';
import { formatApiError } from '@/src/lib/apiError';
import { choiceLabel, moduleLabel, optionLabel, variationLabel } from '@/src/lib/gameLabels';
import { loadGameSetup, saveGameSetup } from '@/src/lib/gameSetupStore';
import { t } from '@/src/lib/i18n';
import { factText } from '@/src/lib/labels';
import { colors, shared } from '@/src/theme';

type Mode = 'table' | 'bots';

/** '' leaves a seat on the table's Opponents setting; the rest are the server's module.Skill ids. */
const SEAT_SKILLS = ['', 'easy', 'medium', 'hard', 'ai'] as const;

/**
 * One game's settings, all of them open, and the button that starts it.
 *
 * Rendered entirely from `/modules`: this screen has no list of options in
 * it. `MELD_MINS` and `DISCARD_LOCK_ROUNDS` were once client constants, which
 * meant adding a knob meant editing a client. Here the form is whatever the
 * module declares, and a game registered tomorrow gets the right one.
 *
 * Everything is shown at once. When the setups of every game were stacked on
 * one screen they had to be folded away, or a player who came to press one
 * button met seven games' worth of controls; this screen is one game, reached
 * by choosing to start one, so the controls are the point of being here.
 *
 * Two ways in, from the game's own page, and one difference between them. A
 * table for people is opened and then filled from the table lobby — bots are
 * added there, one at a time, beside the people invited — so the bot count
 * belongs only to a game against bots, which deals straight away.
 */
export default function GameSetupScreen() {
  const params = useLocalSearchParams<{ moduleId?: string; mode?: string }>();
  const moduleId = String(params.moduleId ?? '');
  const asked: Mode = params.mode === 'bots' ? 'bots' : 'table';
  const { client, session } = useSession();
  useLocale();

  const [mod, setMod] = useState<MatchModule | null>(null);
  const [error, setError] = useState('');
  const [startError, setStartError] = useState('');
  const [busy, setBusy] = useState(false);
  const [variation, setVariation] = useState<string | undefined>();
  const [options, setOptions] = useState<Record<string, number>>({});
  const [bots, setBots] = useState<number | undefined>();
  // One entry per bot seat: '' follows the table's Opponents setting, anything
  // else is that seat's own strength. Not remembered between visits — a mix of
  // strengths is a choice about this game, not a preference.
  const [seatSkills, setSeatSkills] = useState<string[]>([]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const m = (await client.modules()).find((x) => x.id === moduleId);
        if (cancelled) return;
        if (!m) {
          router.replace('/');
          return;
        }
        const saved = await loadGameSetup(m.id);
        if (cancelled) return;
        // Only trust a saved pick that still matches this module's current
        // descriptor: a variation or option the server has since renamed or
        // dropped falls back to the module's own default rather than
        // resurrecting a stale choice.
        const savedVariation =
          saved?.variation && m.variations?.some((x) => x.id === saved.variation)
            ? saved.variation
            : undefined;
        const variationId = savedVariation ?? m.variations?.[0]?.id;
        const spec = m.variations?.find((x) => x.id === variationId);
        const o: Record<string, number> = { ...(spec?.defaults ?? {}) };
        for (const opt of m.options ?? []) {
          if (saved?.options && opt.name in saved.options) o[opt.name] = saved.options[opt.name];
        }
        setVariation(variationId);
        setOptions(o);
        setBots(saved?.bots);
        setMod(m);
      } catch (e) {
        if (!cancelled) setError(formatApiError(e));
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [client, moduleId]);

  const pickVariation = (m: MatchModule, id: string) => {
    setVariation(id);
    // A variation is a named ruleset, so switching it moves the knobs with it.
    const spec = m.variations?.find((x) => x.id === id);
    setOptions({ ...(spec?.defaults ?? {}) });
  };

  // A one-seat game has no table to fill and nobody to invite: it is always
  // dealt straight away, with no bots.
  const mode: Mode = mod && mod.maxPlayers === 1 ? 'bots' : asked;

  const start = useCallback(async () => {
    if (!mod) return;
    if (!session) {
      setStartError(t('lobby.games.signInFirst'));
      return;
    }
    setBusy(true);
    setStartError('');
    try {
      // Remembered for next time regardless of how the match turns out — it
      // is a preference about this screen, not a fact about the match.
      await saveGameSetup(mod.id, { variation, options, bots });
      const { matchId } = await client.createMatch(mod.id, variation, options);

      if (mode === 'table') {
        // Open the table and let the host fill it: invite someone out of the
        // waiting room, add bots, then deal.
        router.replace(`/lobby/table?matchId=${encodeURIComponent(matchId)}`);
        return;
      }

      // As many bots as asked for, clamped to the seats this module has. The
      // screen does not know what a legal table is for any game — the
      // descriptor does, and the clamp is the only thing reading it.
      const seats = botCount(mod, bots);
      for (let i = 0; i < seats; i++) {
        await client.addBot(matchId, seatSkills[i] || undefined);
      }
      await client.startMatch(matchId);
      router.replace(`/match/${matchId}`);
    } catch (e) {
      setStartError(formatApiError(e));
    } finally {
      setBusy(false);
    }
  }, [client, session, mod, mode, variation, options, bots, seatSkills]);

  if (error) {
    return (
      <Screen title={t('nav.games')}>
        <Text testID="games-error" style={shared.error}>
          {error}
        </Text>
      </Screen>
    );
  }
  if (!mod) {
    return (
      <Screen title={t('nav.games')}>
        <ActivityIndicator color={colors.accent} />
      </Screen>
    );
  }

  const spec = mod.variations?.find((v) => v.id === variation);
  const choices = botChoices(mod);

  return (
    <Screen>
      <ScrollView style={{ flex: 1 }} keyboardShouldPersistTaps="handled" testID="games-list">
        <View testID={`module-${mod.id}`}>
          <View style={styles.header}>
            <Text style={[shared.title, { flexShrink: 1 }]}>
              {mod.maxPlayers === 1
                ? moduleLabel(mod)
                : mode === 'bots'
                  ? t('setup.titleBots', { game: moduleLabel(mod) })
                  : t('setup.titleTable', { game: moduleLabel(mod) })}
            </Text>
            {/* The rules as these settings make them: a ruleset and its
                options change what the written rules say. One template
                literal, not a `+` chain — expo-router's typed routes only see
                through the former. */}
            <Pressable
              testID={`setup-rules-${mod.id}`}
              accessibilityRole="link"
              onPress={() =>
                router.push(
                  `/rules?moduleId=${encodeURIComponent(mod.id)}&variation=${encodeURIComponent(variation ?? '')}&options=${encodeURIComponent(JSON.stringify(options))}`,
                )
              }
              style={styles.rulesLink}
            >
              <Text style={styles.rulesLinkText}>{t('nav.rules')} ›</Text>
            </Pressable>
          </View>

          {(mod.variations ?? []).length > 1 ? (
            <View testID={`setup-section-${mod.id}-variation`} style={styles.row}>
              {(mod.variations ?? []).map((v) => (
                <Pressable
                  key={v.id}
                  testID={`variation-${mod.id}-${v.id}`}
                  onPress={() => pickVariation(mod, v.id)}
                  style={[styles.pill, variation === v.id && styles.pillOn]}
                >
                  <Text style={styles.pillText}>{variationLabel(mod.id, v)}</Text>
                </Pressable>
              ))}
            </View>
          ) : null}

          {/* What this ruleset is, in the module's own words. */}
          {spec?.summary?.map((f, i) => (
            <Text key={i} style={styles.summary}>
              · {factText(f)}
            </Text>
          ))}

          {mode === 'bots' && choices.length > 1 ? (
            <View testID={`setup-section-${mod.id}-bots`} style={styles.option}>
              <Text style={styles.optionLabel}>{t('lobby.games.bots')}</Text>
              <View style={styles.row}>
                {choices.map((n) => (
                  <Pressable
                    key={n}
                    testID={`bots-${mod.id}-${n}`}
                    onPress={() => setBots(n)}
                    style={[styles.pill, botCount(mod, bots) === n && styles.pillOn]}
                  >
                    <Text style={styles.pillText}>{n}</Text>
                  </Pressable>
                ))}
              </View>
            </View>
          ) : null}

          {mode === 'bots' && (mod.options ?? []).some((o) => o.name === 'botSkill')
            ? Array.from({ length: botCount(mod, bots) }, (_, i) => (
                <View key={i} testID={`setup-section-${mod.id}-bot-${i}`} style={styles.option}>
                  <Text style={styles.optionLabel}>{t('setup.botSeat', { n: i + 1 })}</Text>
                  <View style={styles.row}>
                    {SEAT_SKILLS.map((s) => (
                      <Pressable
                        key={s || 'table'}
                        testID={`bot-skill-${mod.id}-${i}-${s || 'table'}`}
                        onPress={() =>
                          setSeatSkills((prev) => {
                            const next = [...prev];
                            next[i] = s;
                            return next;
                          })
                        }
                        style={[styles.pill, (seatSkills[i] ?? '') === s && styles.pillOn]}
                      >
                        <Text style={styles.pillText}>
                          {s ? t(`setup.botSkill.${s}`) : t('setup.botSkillTable')}
                        </Text>
                      </Pressable>
                    ))}
                  </View>
                </View>
              ))
            : null}

          {(mod.options ?? []).map((opt) => (
            <View key={opt.name} style={styles.option}>
              <Text style={styles.optionLabel}>{optionLabel(mod.id, opt)}</Text>
              <View style={styles.row}>
                {opt.choices.map((c) => (
                  <Pressable
                    key={c.value}
                    testID={`option-${mod.id}-${opt.name}-${c.value}`}
                    onPress={() => setOptions((prev) => ({ ...prev, [opt.name]: c.value }))}
                    style={[styles.pill, options[opt.name] === c.value && styles.pillOn]}
                  >
                    <Text style={styles.pillText}>{choiceLabel(mod.id, opt.name, c)}</Text>
                  </Pressable>
                ))}
              </View>
            </View>
          ))}
        </View>
      </ScrollView>

      {/* Outside the scroll, so the button is always in reach however many
          options the game declares. */}
      <View style={styles.footer}>
        {startError ? (
          <Text
            testID={`start-error-${mod.id}`}
            accessibilityRole="alert"
            accessibilityLiveRegion="polite"
            style={styles.startError}
          >
            {startError}
          </Text>
        ) : null}
        <Pressable
          testID={mode === 'bots' ? `deal-me-in-${mod.id}` : `open-table-${mod.id}`}
          accessibilityRole="button"
          disabled={busy}
          onPress={start}
          style={[shared.button, { marginBottom: 0 }, busy && styles.busy]}
        >
          <Text style={shared.buttonText}>
            {mode === 'bots' ? t('setup.dealMeIn') : t('setup.openTable')}
          </Text>
        </Pressable>
      </View>
    </Screen>
  );
}

/**
 * The bot counts a module can be opened with: enough to reach its minimum
 * table at the low end, one short of its maximum at the high end — because
 * one of the seats is the host's. A one-seat game has exactly one answer:
 * none.
 */
function botChoices(mod: MatchModule): number[] {
  if (mod.maxPlayers <= 1) return [0];
  const out: number[] = [];
  for (let n = Math.max(1, mod.minPlayers - 1); n <= Math.max(1, mod.maxPlayers - 1); n++) {
    out.push(n);
  }
  return out;
}

/**
 * How many bots to seat: what the host picked, clamped to the range above.
 * A clamp rather than a plain read, so a count left over from a game whose
 * range has since moved can never post a seat the server is going to refuse.
 */
function botCount(mod: MatchModule, picked?: number): number {
  const choices = botChoices(mod);
  const lo = choices[0];
  const hi = choices[choices.length - 1];
  return Math.min(hi, Math.max(lo, picked ?? lo));
}

const styles = StyleSheet.create({
  header: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'flex-start', gap: 12 },
  rulesLink: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 6,
    paddingHorizontal: 10,
    paddingVertical: 5,
    marginTop: 4,
  },
  rulesLinkText: { color: colors.accentButton, fontSize: 12, fontWeight: '700' },
  summary: { color: colors.muted, fontSize: 13, marginTop: 2 },
  row: { flexDirection: 'row', flexWrap: 'wrap', gap: 6, marginTop: 6 },
  option: { marginTop: 14 },
  optionLabel: { color: colors.muted, fontSize: 12, fontWeight: '700' },
  pill: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 6,
    paddingHorizontal: 12,
    paddingVertical: 7,
  },
  pillOn: { borderColor: colors.accent, backgroundColor: colors.accentDim },
  pillText: { color: colors.text, fontSize: 13 },
  footer: {
    borderTopWidth: 1,
    borderTopColor: colors.border,
    paddingTop: 12,
    paddingBottom: 16,
    backgroundColor: colors.bg,
  },
  busy: { opacity: 0.5 },
  startError: { color: colors.danger, marginBottom: 10 },
});
