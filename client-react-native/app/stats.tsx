import { router } from 'expo-router';
import { useEffect, useState } from 'react';
import { ActivityIndicator, Pressable, Text, View } from 'react-native';

import type { MatchModule } from '@/src/api/matchTypes';
import type {
  Leaderboard,
  LeaderboardKind,
  LeaderboardScope,
  LifetimeStats,
  TallyView,
} from '@/src/api/types';
import { Screen } from '@/src/components/Screen';
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
import { colors, shared } from '@/src/theme';

/**
 * Your lifetime record, and the board it puts you on.
 *
 * The two halves load independently and fail independently, which is the whole
 * shape of this screen. The leaderboard is public and the personal record needs
 * an account, so a signed-out visitor sees a full board and an invitation, a
 * guest sees a board and an explanation of why they have no record of their
 * own, and a server that is down for one of the two does not blank the other.
 *
 * Everything a figure *means* — whether a bucket is worth a row, what an
 * unplayed one reads as, how a bot persona is named — is decided in
 * `src/lib/stats.ts` and tested there. This file lays out the answers.
 */
export default function StatsScreen() {
  const { client, session } = useSession();

  const registered = !!session && !session.isGuest;

  const [stats, setStats] = useState<LifetimeStats | null>(null);
  const [statsError, setStatsError] = useState('');
  const [statsLoading, setStatsLoading] = useState(registered);

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
  }, [client]);

  useEffect(() => {
    if (!registered) {
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
        if (!cancelled) setStatsError(e instanceof Error ? e.message : 'Could not load your stats');
      })
      .finally(() => {
        if (!cancelled) setStatsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [client, registered, session?.userId]);

  useEffect(() => {
    let cancelled = false;
    setBoardLoading(true);
    setBoardError('');
    client
      .getLeaderboard({ kind, scope, limit: 25 })
      .then((b) => {
        if (!cancelled) setBoard(b);
      })
      .catch((e) => {
        if (!cancelled) {
          setBoardError(e instanceof Error ? e.message : 'Could not load the leaderboard');
        }
      })
      .finally(() => {
        if (!cancelled) setBoardLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [client, kind, scope]);

  return (
    <Screen title="Stats & leaderboard" scroll>
      <Section title="Your record" testID="your-record">
        {statsLoading ? (
          <ActivityIndicator color={colors.accent} />
        ) : !session ? (
          <SignInPrompt reason="Sign in to keep a lifetime record of the games you play." />
        ) : session.isGuest ? (
          <SignInPrompt
            reason={
              // Not a limitation to apologise for — it is why the guest option
              // is safe to offer. Said plainly, with the fix attached.
              'Guest games are recorded, but a guest name is per-device and two people can hold the same one — so there is no lifetime record until you sign in. Sign in and the games this device has already played come with you.'
            }
          />
        ) : statsError ? (
          <Empty testID="stats-error">Could not load your stats: {statsError}</Empty>
        ) : stats ? (
          <YourRecord stats={stats} modules={modules} />
        ) : null}
      </Section>

      <Section
        title="Leaderboard"
        note={scopeBlurb(scope)}
        testID="leaderboard-section"
      >
        <Toggle
          testID="leaderboard-kind"
          value={kind}
          onChange={setKind}
          options={[
            { id: 'user', label: 'Players' },
            { id: 'ai', label: 'Bots' },
          ]}
        />
        <Toggle
          testID="leaderboard-scope"
          value={scope}
          onChange={setScope}
          options={SCOPES.map((s) => ({ id: s.id, label: s.label }))}
        />
        {boardLoading ? (
          <ActivityIndicator color={colors.accent} style={{ marginTop: 12 }} />
        ) : boardError ? (
          <Empty testID="leaderboard-error">Could not load the leaderboard: {boardError}</Empty>
        ) : board && board.entries.length > 0 ? (
          <LeaderboardTable
            entries={board.entries}
            youId={registered ? session?.userId : undefined}
          />
        ) : (
          <Empty testID="leaderboard-empty">
            {kind === 'ai'
              ? 'No bot has finished a match under these rules yet.'
              : 'Nobody is ranked here yet. Finish a match and this is where it shows up.'}
          </Empty>
        )}
      </Section>
    </Screen>
  );
}

function SignInPrompt({ reason }: { reason: string }) {
  return (
    <View style={[shared.card, { marginBottom: 0 }]} testID="stats-signin-prompt">
      <Text style={[shared.status, { marginTop: 0, marginBottom: 12 }]}>{reason}</Text>
      <Pressable style={[shared.button, { marginBottom: 0 }]} onPress={() => router.push('/auth/login')}>
        <Text style={shared.buttonText}>Sign in</Text>
      </Pressable>
    </View>
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
    return (
      <Empty testID="stats-none-yet">
        No finished matches yet. Play one and your record starts here.
      </Empty>
    );
  }

  const opponents = splits(
    { vsHumans: stats.vsHumans, vsAI: stats.vsAI },
    (k) => (k === 'vsHumans' ? 'vs humans' : 'vs bots'),
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
        <Line label="Win rate" value={winPercentText(stats.overall)} testID="stats-winrate" />
        <Line
          label="Average finish"
          value={avgRankText(stats.overall)}
          testID="stats-avgrank"
        />
        <Line
          label="Current streak"
          value={streakText(stats.currentStreak)}
          tone={
            stats.currentStreak > 0
              ? colors.success
              : stats.currentStreak < 0
                ? colors.danger
                : undefined
          }
          testID="stats-streak"
        />
        <Line
          label="Best winning streak"
          value={stats.longestWinStreak > 0 ? `${stats.longestWinStreak} in a row` : '—'}
          testID="stats-beststreak"
        />
        {stats.overall.bestScore !== null ? (
          <Line
            label="Best score"
            value={String(stats.overall.bestScore)}
            testID="stats-bestscore"
          />
        ) : null}
      </View>

      {opponents.length > 0 ? (
        <SubTable
          title="Who you played"
          // The overlap surprises people whose two rows add up to more than
          // their overall record, so it is stated where they see it.
          note="A table with both a person and a bot at it counts in both rows."
          rows={opponents}
          testID="split-opponents"
        />
      ) : null}
      {games.length > 1 ? (
        <SubTable title="By game" rows={games} testID="split-games" />
      ) : null}
      {sizes.length > 1 ? (
        <SubTable title="By table size" rows={sizes} testID="split-sizes" />
      ) : null}
      {bots.length > 0 ? (
        <SubTable title="Against bots" rows={bots} testID="split-bots" />
      ) : null}
    </View>
  );
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
