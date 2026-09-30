import { useEffect, useState } from 'react';
import { ActivityIndicator, Text, View } from 'react-native';

import type { MatchModule } from '@/src/api/matchTypes';
import type {
  Leaderboard,
  LeaderboardKind,
  LeaderboardScope,
  LifetimeStats,
  TallyView,
} from '@/src/api/types';
import { Screen } from '@/src/components/Screen';
import { SignInRequired } from '@/src/components/SignInRequired';
import { LeaderboardTable } from '@/src/components/stats/LeaderboardTable';
import {
  Empty,
  Headline,
  Line,
  Section,
  SplitTable,
  Toggle,
} from '@/src/components/stats/StatsParts';
import { useSession } from '@/src/context/SessionContext';
import { useLocale } from '@/src/hooks/useLocale';
import { t } from '@/src/lib/i18n';
import {
  SCOPES,
  avgRankText,
  difficultySplits,
  moduleLabel,
  played,
  playerCountSplits,
  scopeBlurb,
  splits,
  streakText,
  winPercentText,
} from '@/src/lib/stats';
import { colors } from '@/src/theme';

/**
 * A player's lifetime record, and the board it puts them on.
 *
 * Reached from the "More" screen behind the face in the corner, and — like the
 * score table beside it there — only with an account: what is shown here is
 * kept *with* an account, and a guest has nowhere for it to be kept. The gate
 * is `SignInRequired`, the same one the score table uses, so the two cannot
 * come to disagree about who is allowed in.
 *
 * The two data-backed halves load independently and fail independently, so a
 * server that is down for one does not blank the other.
 *
 * Everything a figure *means* — whether a bucket is worth a row, what an
 * unplayed one reads as, how a bot persona is named — is decided in
 * `src/lib/stats.ts` and tested there. This file lays out the answers.
 */
export default function StatsScreen() {
  const { client, session } = useSession();
  useLocale();

  const signedIn = !!session && !session.isGuest;

  const [stats, setStats] = useState<LifetimeStats | null>(null);
  const [statsError, setStatsError] = useState('');
  const [statsLoading, setStatsLoading] = useState(signedIn);

  const [kind, setKind] = useState<LeaderboardKind>('user');
  const [scope, setScope] = useState<LeaderboardScope>('overall');
  const [board, setBoard] = useState<Leaderboard | null>(null);
  const [boardError, setBoardError] = useState('');
  const [boardLoading, setBoardLoading] = useState(true);

  // The module list is only ever used to put a name on a `byModule` key. A
  // failure here is not worth reporting: `moduleLabel` humanises the id, so the
  // table renders either way.
  const [modules, setModules] = useState<MatchModule[]>([]);
  useEffect(() => {
    if (!signedIn) return;
    let cancelled = false;
    client
      .modules()
      .then((m) => {
        if (!cancelled) setModules(m);
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [client, signedIn]);

  useEffect(() => {
    // No guest branch: without an account the screen renders the gate below
    // instead, and there is nothing to fetch for it.
    if (!signedIn) {
      setStats(null);
      setStatsLoading(false);
      return;
    }
    let cancelled = false;
    setStatsLoading(true);
    setStatsError('');
    client
      .getStats()
      .then((s) => {
        if (!cancelled) setStats(s);
      })
      .catch((e) => {
        if (!cancelled) setStatsError(e instanceof Error ? e.message : t('error.generic'));
      })
      .finally(() => {
        if (!cancelled) setStatsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [client, signedIn, session?.userId]);

  useEffect(() => {
    if (!signedIn) return;
    let cancelled = false;
    setBoardLoading(true);
    setBoardError('');
    client
      .getLeaderboard({ kind, scope, limit: 25 })
      .then((b) => {
        if (!cancelled) setBoard(b);
      })
      .catch((e) => {
        if (!cancelled) setBoardError(e instanceof Error ? e.message : t('error.generic'));
      })
      .finally(() => {
        if (!cancelled) setBoardLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [client, kind, scope, signedIn]);

  if (!signedIn) return <SignInRequired title={t('more.stats')} />;

  return (
    <Screen title={t('more.stats')} scroll>
      <Section title={t('record.title')} testID="your-record">
        {statsLoading ? (
          <ActivityIndicator color={colors.accent} />
        ) : statsError ? (
          <Empty testID="stats-error">{t('stats.recordFailed', { reason: statsError })}</Empty>
        ) : stats ? (
          <YourRecord stats={stats} modules={modules} />
        ) : null}
      </Section>

      <Section title={t('stats.leaderboard')} note={scopeBlurb(scope)} testID="leaderboard-section">
        <Toggle
          testID="leaderboard-kind"
          value={kind}
          onChange={setKind}
          options={[
            { id: 'user', label: t('stats.kind.players') },
            { id: 'ai', label: t('stats.kind.bots') },
          ]}
        />
        <Toggle
          testID="leaderboard-scope"
          value={scope}
          onChange={setScope}
          options={SCOPES.map((s) => ({ id: s.id, label: t(s.labelKey) }))}
        />
        {boardLoading ? (
          <ActivityIndicator color={colors.accent} style={{ marginTop: 12 }} />
        ) : boardError ? (
          <Empty testID="leaderboard-error">{t('stats.boardFailed', { reason: boardError })}</Empty>
        ) : board && board.entries.length > 0 ? (
          <LeaderboardTable entries={board.entries} youId={session?.userId} />
        ) : (
          <Empty testID="leaderboard-empty">
            {kind === 'ai' ? t('stats.boardEmptyBots') : t('stats.boardEmptyPlayers')}
          </Empty>
        )}
      </Section>
    </Screen>
  );
}

/**
 * A registered player's lifetime record.
 *
 * The splits below the headline are dropped when empty rather than rendered as
 * a row of zeroes — see the note in `src/lib/stats.ts` about what a flat zero
 * claims about a table size somebody has simply never played.
 */
function YourRecord({ stats, modules }: { stats: LifetimeStats; modules: MatchModule[] }) {
  if (!played(stats.overall)) {
    return <Empty testID="stats-none-yet">{t('stats.noneYet')}</Empty>;
  }

  const opponents = splits(
    { vsHumans: stats.vsHumans, vsAI: stats.vsAI },
    (k) => (k === 'vsHumans' ? t('stats.scope.vsHumans') : t('stats.scope.vsBots')),
    // Declared order, not alphabetical: humans first because that is the
    // half most people came to look at.
    (a, b) => (a === 'vsHumans' ? -1 : b === 'vsHumans' ? 1 : 0),
  );
  const games = splits(stats.byModule, (k) => moduleLabel(k, modules));
  const sizes = playerCountSplits(stats.byPlayerCount);
  const bots = difficultySplits(stats.byAIDifficulty);

  return (
    <View>
      <Headline tally={stats.overall} />

      <View style={{ marginTop: 12 }}>
        <Line label={t('record.winRate')} value={winPercentText(stats.overall)} testID="stats-winrate" />
        <Line label={t('stats.avgFinish')} value={avgRankText(stats.overall)} testID="stats-avgrank" />
        <Line
          label={t('stats.currentStreak')}
          value={streakText(stats.currentStreak)}
          tone={streakTone(stats.currentStreak)}
          testID="stats-streak"
        />
        <Line
          label={t('stats.bestStreak')}
          value={
            stats.longestWinStreak > 0
              ? t('stats.bestStreakValue', { n: stats.longestWinStreak })
              : '—'
          }
          testID="stats-beststreak"
        />
        {stats.overall.bestScore !== null ? (
          <Line
            label={t('stats.bestScore')}
            value={String(stats.overall.bestScore)}
            testID="stats-bestscore"
          />
        ) : null}
      </View>

      {opponents.length > 0 ? (
        <SubTable
          title={t('stats.split.opponents')}
          // The overlap surprises people whose two rows add up to more than
          // their overall record, so it is stated where they see it.
          note={t('stats.split.opponentsNote')}
          rows={opponents}
          testID="split-opponents"
        />
      ) : null}
      {games.length > 1 ? (
        <SubTable title={t('stats.split.games')} rows={games} testID="split-games" />
      ) : null}
      {sizes.length > 1 ? (
        <SubTable title={t('stats.split.tableSize')} rows={sizes} testID="split-sizes" />
      ) : null}
      {bots.length > 0 ? (
        <SubTable title={t('stats.split.bots')} rows={bots} testID="split-bots" />
      ) : null}
    </View>
  );
}

/** Green for a winning run, red for a losing one, plain after a draw. */
function streakTone(streak: number): string | undefined {
  if (streak > 0) return colors.success;
  if (streak < 0) return colors.danger;
  return undefined;
}

function SubTable({
  title,
  note,
  rows,
  testID,
}: {
  title: string;
  note?: string;
  rows: { key: string; label: string; tally: TallyView }[];
  testID?: string;
}) {
  return (
    <View style={{ marginTop: 20 }}>
      <Text style={{ color: colors.text, fontSize: 15, fontWeight: '600', marginBottom: 4 }}>
        {title}
      </Text>
      {note ? (
        <Text style={{ color: colors.muted, fontSize: 12, marginBottom: 8 }}>{note}</Text>
      ) : null}
      <SplitTable rows={rows} testID={testID} />
    </View>
  );
}
