import { useEffect, useMemo, useState } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';
import { useRouter } from 'expo-router';

import { useSession } from '@/src/context/SessionContext';
import { useMetrics } from '@/src/hooks/useMetrics';
import type { LifetimeStats, TallyView } from '@/src/api/types';
import type { Metrics } from '@/src/lib/layout';
import { useSkin } from '@/src/hooks/useSkin';
import type { Skin } from '@/src/skins/types';

import { Panel } from './Panel';
import { t } from '@/src/lib/i18n';
import { recordFigures, streakText } from '@/src/lib/stats';

/**
 * The player's own record, beside the match that just changed it.
 *
 * A guest has none, and that is a decision rather than a gap: a guest identity
 * lives on one device and is keyed on a display name anybody may claim, so a
 * lifetime record for one would merge strangers' histories. So the panel holds
 * an invitation in its place — and the invitation is worth making, because
 * signing in does not start a record from zero. The server re-attributes every
 * match the device has already played, so a guest who registers here keeps the
 * one they have just finished.
 *
 * The figures come from `/users/me/stats` and are read, never computed: win
 * rate, averages and streaks are all derived on the server so a fix to an
 * average can never require rewriting history.
 */
type Props = {
  /** Which game to show a per-game record for, beside the overall one. */
  moduleId?: string;
};

export function LifetimeRecord({ moduleId }: Props) {
  const { session, client } = useSession();
  const router = useRouter();
  const metrics = useMetrics();
  const skin = useSkin();
  const styles = useMemo(() => recordStyles(metrics, skin), [metrics, skin]);

  const [stats, setStats] = useState<LifetimeStats | null>(null);
  const [failed, setFailed] = useState(false);

  const isGuest = !session || session.isGuest;

  useEffect(() => {
    if (isGuest) return;
    let live = true;
    const load = () =>
      client
        .getStats()
        .then((s) => live && setStats(s))
        .catch(() => live && setFailed(true));

    load();

    // And once more, shortly.
    //
    // A finished match is recorded asynchronously and deliberately: the player
    // who just won should see the final board without waiting on bookkeeping,
    // and a bookkeeping failure must never fail the move that won. Which means
    // the first read of this panel can land before the match it is sitting
    // under has been counted, and show a record that is one match stale — the
    // one match its reader is most interested in.
    //
    // Two bounded reads rather than a poll: the write is a single insert that
    // has already been dispatched, so it is either there by now or something
    // has gone wrong that retrying will not fix.
    const again = setTimeout(load, 2500);
    return () => {
      live = false;
      clearTimeout(again);
    };
  }, [client, isGuest]);

  if (isGuest) {
    return (
      <Panel title={t('record.title')} forceOpen testID="lifetime-record">
        <Text style={styles.invite} testID="lifetime-guest-invite">
          {t('record.guest')}
        </Text>
        <Pressable
          testID="lifetime-sign-in"
          onPress={() => router.push('/auth/login')}
          style={styles.button}
        >
          <Text style={styles.buttonText}>{t('record.signInToKeep')}</Text>
        </Pressable>
      </Panel>
    );
  }

  if (failed) {
    return (
      <Panel title={t('record.title')} forceOpen testID="lifetime-record">
        <Text style={styles.quiet} testID="lifetime-unavailable">
          {t('record.failed')}
        </Text>
      </Panel>
    );
  }

  if (!stats) {
    return (
      <Panel title={t('record.title')} forceOpen testID="lifetime-record">
        <Text style={styles.quiet}>{t('record.loading')}</Text>
      </Panel>
    );
  }

  const perGame = moduleId ? stats.byModule?.[moduleId] : undefined;

  return (
    <Panel title={t('record.title')} forceOpen testID="lifetime-record">
      <Tally tally={stats.overall} styles={styles} testID="lifetime-overall" />
      {/* The streak is a sentence of its own rather than a fifth figure: as
          "Streak: 1 loss" beside Played and Won it read as a loss count, and
          55 played, 29 won, 1 loss does not add up. */}
      <Text style={styles.streak} testID="lifetime-streak">
        {streakText(stats.currentStreak)}
      </Text>

      {/* Kept separate from the overall figures on purpose: a rummy penalty
          total and a poker stack are not comparable numbers, so an average
          across both would be noise. */}
      {perGame ? (
        <View style={styles.perGame} testID="lifetime-this-game">
          <Text style={styles.perGameTitle}>{t('record.atThisGame')}</Text>
          <Tally tally={perGame} styles={styles} />
        </View>
      ) : null}
    </Panel>
  );
}

/** One bucket's figures; which ones, and in what order, is recordFigures'. */
function Tally({
  tally,
  styles,
  testID,
}: {
  tally: TallyView;
  styles: ReturnType<typeof recordStyles>;
  testID?: string;
}) {
  return (
    <View style={styles.grid} testID={testID}>
      {recordFigures(tally).map((f) => (
        <Figure
          key={f.key}
          label={f.label}
          value={f.value}
          styles={styles}
          testID={testID ? `${testID}-${f.key}` : undefined}
        />
      ))}
    </View>
  );
}

function Figure({
  label,
  value,
  styles,
  testID,
}: {
  label: string;
  value: string;
  styles: ReturnType<typeof recordStyles>;
  testID?: string;
}) {
  return (
    <View style={styles.figure}>
      <Text style={styles.figureValue} testID={testID}>
        {value}
      </Text>
      <Text style={styles.figureLabel}>{label}</Text>
    </View>
  );
}

/*
 * Deliberately absent: the best score posted at this game.
 *
 * TallyView carries it, but it is stored the way everything downstream of a
 * module is stored — higher-is-better — so a rummy best of 98 penalty points is
 * held as -98, and printing it reads "Best -98". Standing.Shown solves this for
 * a scoreboard because the module fills it in; a lifetime record has no module
 * to ask, and there is nothing in the statistics data that says which way a
 * given game counts.
 *
 * Doing it properly means the descriptor declaring a game's scoring direction,
 * which is a real fact only a module knows and would fix every screen that
 * shows a stored score rather than this one figure. Until then this shows the
 * counts, which read the same way in every game.
 */

function recordStyles(m: Metrics, s: Skin) {
  const colors = s.colors;
  return StyleSheet.create({
    grid: { flexDirection: 'row', flexWrap: 'wrap', gap: m.narrow ? 12 : 20 },
    figure: { minWidth: m.narrow ? 60 : 72 },
    figureValue: { color: colors.text, fontSize: m.narrow ? 18 : 20, fontWeight: '700' },
    figureLabel: { color: colors.muted, fontSize: Math.max(10, m.panel.bodyFont - 2) },
    streak: { color: colors.muted, fontSize: m.panel.bodyFont, marginTop: 8 },
    perGame: { marginTop: 12, borderTopWidth: 1, borderTopColor: colors.border, paddingTop: 10 },
    perGameTitle: {
      color: colors.muted,
      fontSize: Math.max(10, m.panel.bodyFont - 2),
      marginBottom: 6,
      fontWeight: '600',
    },
    invite: { color: colors.text, fontSize: m.panel.bodyFont, lineHeight: m.panel.bodyFont + 6 },
    quiet: { color: colors.muted, fontSize: m.panel.bodyFont },
    button: {
      alignSelf: 'flex-start',
      marginTop: 10,
      backgroundColor: colors.accentButton,
      paddingVertical: 10,
      paddingHorizontal: 16,
      borderRadius: 8,
    },
    buttonText: { color: colors.onAccent, fontSize: 14, fontWeight: '600' },
  });
}
