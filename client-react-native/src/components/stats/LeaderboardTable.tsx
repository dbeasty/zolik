import { StyleSheet, Text, View } from 'react-native';

import type { LeaderboardEntry } from '@/src/api/types';
import { subjectName, winPercentText } from '@/src/lib/stats';
import { colors } from '@/src/theme';

/**
 * The ranked board.
 *
 * `youId` is the signed-in account's id, used only to mark one row. It is
 * matched against `subject.id` rather than against the displayed name, because
 * two people can share a display name and exactly one of them is you.
 */
export function LeaderboardTable({
  entries,
  youId,
}: {
  entries: LeaderboardEntry[];
  youId?: string;
}) {
  return (
    <View style={styles.table} testID="leaderboard-table">
      <View style={styles.row}>
        <Text style={[styles.rank, styles.head]}>#</Text>
        <Text style={[styles.name, styles.head]}>Player</Text>
        <Text style={[styles.num, styles.head]}>Played</Text>
        <Text style={[styles.num, styles.head]}>Won</Text>
        <Text style={[styles.num, styles.head]}>Win %</Text>
      </View>
      {entries.map((e) => {
        const you = !!youId && e.subject.kind === 'user' && e.subject.id === youId;
        return (
          <View
            key={`${e.subject.kind}:${e.subject.id}`}
            style={[styles.row, you && styles.youRow]}
            testID={you ? 'leaderboard-row-you' : `leaderboard-row-${e.rank}`}
          >
            <Text style={[styles.rank, you && styles.youText]}>{e.rank}</Text>
            <Text style={[styles.name, you && styles.youText]} numberOfLines={1}>
              {subjectName(e.subject)}
              {you ? ' (you)' : ''}
            </Text>
            <Text style={[styles.num, you && styles.youText]}>{e.tally.matches}</Text>
            <Text style={[styles.num, you && styles.youText]}>{e.tally.wins}</Text>
            <Text style={[styles.num, you && styles.youText]}>{winPercentText(e.tally)}</Text>
          </View>
        );
      })}
    </View>
  );
}

const styles = StyleSheet.create({
  table: {
    backgroundColor: colors.surface,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 12,
    paddingHorizontal: 12,
    paddingVertical: 4,
  },
  row: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingVertical: 9,
    borderBottomWidth: StyleSheet.hairlineWidth,
    borderBottomColor: colors.border,
  },
  // Your own row is tinted rather than only bolded: on a board of twenty rows,
  // weight alone is not findable at a glance.
  youRow: {
    backgroundColor: 'rgba(122, 184, 255, 0.12)',
    marginHorizontal: -12,
    paddingHorizontal: 12,
  },
  youText: {
    color: colors.accentButton,
    fontWeight: '700',
  },
  head: {
    color: colors.muted,
    fontSize: 11,
    fontWeight: '600',
    textTransform: 'uppercase',
  },
  rank: {
    width: 28,
    color: colors.muted,
    fontSize: 14,
    fontVariant: ['tabular-nums'],
  },
  name: {
    flex: 1,
    color: colors.text,
    fontSize: 14,
    paddingRight: 8,
  },
  num: {
    width: 52,
    textAlign: 'right',
    color: colors.text,
    fontSize: 14,
    fontVariant: ['tabular-nums'],
  },
});
