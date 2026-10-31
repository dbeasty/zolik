import { router, useLocalSearchParams } from 'expo-router';
import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';

import { FormError } from '@/src/a11y/Field';
import { heading } from '@/src/a11y/props';
import { RadioGroup, radioProps } from '@/src/a11y/RadioGroup';
import { Tip } from '@/src/a11y/Tip';
import type { MatchModule } from '@/src/api/matchTypes';
import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { LearnButton } from '@/src/guide/LearnButton';
import { useLocale } from '@/src/hooks/useLocale';
import { formatApiError } from '@/src/lib/apiError';
import { choiceLabel, moduleLabel, optionHelp, optionLabel, variationLabel } from '@/src/lib/gameLabels';
import { botChoices, botCount, dealWithBots, resolveSetup } from '@/src/lib/dealSetup';
import { loadGameSetup, saveGameSetup } from '@/src/lib/gameSetupStore';
import { t } from '@/src/lib/i18n';
import { factText } from '@/src/lib/labels';
import { colors, shared } from '@/src/theme';

type Mode = 'table' | 'bots';

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
        // The same reading of last time's setup that Play now deals with.
        const resolved = resolveSetup(m, saved);
        setVariation(resolved.variation);
        setOptions(resolved.options);
        setBots(resolved.bots);
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

      if (mode === 'table') {
        // Open the table and let the host fill it: invite someone out of the
        // waiting room, add bots, then deal.
        const { matchId } = await client.createMatch(mod.id, variation, options);
        router.replace(`/lobby/table?matchId=${encodeURIComponent(matchId)}`);
        return;
      }

      const matchId = await dealWithBots(client, mod, { variation, options, bots });
      router.replace(`/match/${matchId}`);
    } catch (e) {
      setStartError(formatApiError(e));
    } finally {
      setBusy(false);
    }
  }, [client, session, mod, mode, variation, options, bots]);

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
        <ActivityIndicator aria-label={t('a11y.loading')} color={colors.accent} />
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
            <Text style={[shared.title, { flexShrink: 1 }]} {...heading(2)}>
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
              aria-label={t('a11y.actionFor', { action: t('nav.rules'), what: moduleLabel(mod) })}
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
            // A radio group with no visible heading of its own — the pills
            // are the rulesets — so it is named for a screen reader alone.
            <RadioGroup
              testID={`setup-section-${mod.id}-variation`}
              style={styles.row}
              label={t('a11y.setup.variation')}
            >
              {(mod.variations ?? []).map((v) => (
                <Pressable
                  key={v.id}
                  testID={`variation-${mod.id}-${v.id}`}
                  {...radioProps(variation === v.id)}
                  onPress={() => pickVariation(mod, v.id)}
                  style={[styles.pill, variation === v.id && styles.pillOn]}
                >
                  <Text style={styles.pillText}>{variationLabel(mod.id, v)}</Text>
                </Pressable>
              ))}
            </RadioGroup>
          ) : null}

          {/* What this ruleset is, in the module's own words. */}
          {spec?.summary?.map((f, i) => (
            <Text key={i} style={styles.summary}>
              · {factText(f)}
            </Text>
          ))}

          {mode === 'bots' && choices.length > 1 ? (
            <View testID={`setup-section-${mod.id}-bots`} style={styles.option}>
              <Text style={styles.optionLabel} nativeID={`setup-${mod.id}-bots-label`}>
                {t('lobby.games.bots')}
              </Text>
              <RadioGroup
                style={styles.row}
                label={t('lobby.games.bots')}
                labelledBy={`setup-${mod.id}-bots-label`}
              >
                {choices.map((n) => (
                  <Pressable
                    key={n}
                    testID={`bots-${mod.id}-${n}`}
                    {...radioProps(botCount(mod, bots) === n)}
                    onPress={() => setBots(n)}
                    style={[styles.pill, botCount(mod, bots) === n && styles.pillOn]}
                  >
                    <Text style={styles.pillText}>{n}</Text>
                  </Pressable>
                ))}
              </RadioGroup>
            </View>
          ) : null}

          {(mod.options ?? []).map((opt) => {
            const labelId = `setup-${mod.id}-${opt.name}-label`;
            // The option's own explanation, where the module gives one — the
            // same words wherever the option is described — as the tooltip on
            // its name and the description a screen reader hears with it.
            const help = optionHelp(mod.id, opt);
            return (
              <View key={opt.name} style={styles.option}>
                <Tip text={help} focusable={!!help} label={optionLabel(mod.id, opt)} style={styles.optionName}>
                  {/* The ⓘ beside the name, not inside it: the name is also
                      the radio group's label, and stays exactly the words. */}
                  <View style={styles.optionNameRow}>
                    <Text style={styles.optionLabel} nativeID={labelId}>
                      {optionLabel(mod.id, opt)}
                    </Text>
                    {help ? (
                      <Text style={styles.optionLabel} aria-hidden>
                        {' ⓘ'}
                      </Text>
                    ) : null}
                  </View>
                </Tip>
                <RadioGroup style={styles.row} label={optionLabel(mod.id, opt)} labelledBy={labelId}>
                  {opt.choices.map((c) => (
                    <Pressable
                      key={c.value}
                      testID={`option-${mod.id}-${opt.name}-${c.value}`}
                      {...radioProps(options[opt.name] === c.value)}
                      onPress={() => setOptions((prev) => ({ ...prev, [opt.name]: c.value }))}
                      style={[styles.pill, options[opt.name] === c.value && styles.pillOn]}
                    >
                      <Text style={styles.pillText}>{choiceLabel(mod.id, opt.name, c)}</Text>
                    </Pressable>
                  ))}
                </RadioGroup>
              </View>
            );
          })}
        </View>
      </ScrollView>

      {/* Outside the scroll, so the button is always in reach however many
          options the game declares. */}
      <View style={styles.footer}>
        {startError ? (
          <FormError testID={`start-error-${mod.id}`} style={styles.startError} message={startError} />
        ) : null}
        <Pressable
          testID={mode === 'bots' ? `deal-me-in-${mod.id}` : `open-table-${mod.id}`}
          accessibilityRole="button"
          aria-busy={busy}
          disabled={busy}
          onPress={start}
          style={[shared.button, { marginBottom: 0 }, busy && styles.busy]}
        >
          <Text style={shared.buttonText}>
            {mode === 'bots' ? t('setup.dealMeIn') : t('setup.openTable')}
          </Text>
        </Pressable>
        {/* A lesson for exactly the variation and options selected above. */}
        <LearnButton moduleId={mod.id} variation={variation} options={options} compact />
      </View>
    </Screen>
  );
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
  optionNameRow: { flexDirection: 'row', alignItems: 'baseline' },
  optionLabel: { color: colors.muted, fontSize: 12, fontWeight: '700' },
  // Only as wide as the name, so the tooltip anchors on the words.
  optionName: { alignSelf: 'flex-start' },
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
