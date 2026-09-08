import { ReactNode } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import type { TallyView } from '@/src/api/types';
import { avgRankText, winPercentText } from '@/src/lib/stats';
import { colors, shared } from '@/src/theme';

/**
 * The pieces the stats screen is built from.
 *
 * All presentational: everything here takes numbers already decided on by
 * `src/lib/stats.ts` and puts them somewhere. Nothing in this file works out
 * what a figure means, which is what keeps the interesting logic testable
 * without rendering anything.
 */

/** A section heading with an optional aside — the "why these numbers" line. */
export function Section({
  title,
  note,
  children,
  testID,
}: {
  title: string;
  note?: string;
  children: ReactNode;
  testID?: string;
}) {
  return (
    <View style={{ marginBottom: 24 }} testID={testID}>
      <Text style={styles.sectionTitle}>{title}</Text>
      {note ? <Text style={styles.sectionNote}>{note}</Text> : null}
      {children}
    </View>
  );
}

/**
 * The four headline counts.
 *
 * Laid out as a wrapping row of fixed-basis tiles rather than a 4-column grid,
 * because a narrow phone in a large font size cannot fit four of these across
 * and silently squeezing them is how a "12" becomes a "1".
 */
export function Headline({ tally }: { tally: TallyView }) {
  return (
    <View style={styles.headlineRow} testID="stats-headline">
      <Tile label="Played" value={String(tally.matches)} testID="stats-played" />
      <Tile label="Won" value={String(tally.wins)} tone={colors.success} testID="stats-won" />
      <Tile label="Lost" value={String(tally.losses)} testID="stats-lost" />
      <Tile label="Drawn" value={String(tally.draws)} testID="stats-drawn" />
    </View>
  );
}

function Tile({
  label,
  value,
  tone,
  testID,
}: {
  label: string;
  value: string;
  tone?: string;
  testID?: string;
}) {
  return (
    <View style={styles.tile} testID={testID}>
      <Text style={[styles.tileValue, tone ? { color: tone } : null]}>{value}</Text>
      <Text style={styles.tileLabel}>{label}</Text>
    </View>
  );
}

/** A label and a value on one line — the shape most of this screen is. */
export function Line({
  label,
  value,
  tone,
  testID,
}: {
  label: string;
  value: string;
  tone?: string;
  testID?: string;
}) {
  return (
    <View style={styles.line} testID={testID}>
      <Text style={styles.lineLabel} numberOfLines={2}>
        {label}
      </Text>
      <Text style={[styles.lineValue, tone ? { color: tone } : null]}>{value}</Text>
    </View>
  );
}

/**
 * A table of buckets: one row per split, with its record.
 *
 * The columns are played / won / win rate / average finish. Average finish
 * earns its place next to the win rate because the two disagree in the way
 * that matters: a player who is second in every six-handed game has a 0% win
 * rate and is not a bad player, and only the rank column says so.
 */
export function SplitTable({
  rows,
  testID,
}: {
  rows: { key: string; label: string; tally: TallyView }[];
  testID?: string;
}) {
  return (
    <View style={styles.table} testID={testID}>
      <View style={[styles.row, styles.headRow]}>
        {/* The label column has no heading of its own — what the rows are is
            said by the section title above the table. */}
        <View style={styles.cellLabel} />
        <Text style={[styles.cellNum, styles.headText]}>Played</Text>
        <Text style={[styles.cellNum, styles.headText]}>Won</Text>
        <Text style={[styles.cellNum, styles.headText]}>Win %</Text>
        <Text style={[styles.cellNum, styles.headText]}>Finish</Text>
      </View>
      {rows.map((r) => (
        <View key={r.key} style={styles.row} testID={`split-row-${r.key}`}>
          <Text style={styles.cellLabel} numberOfLines={2}>
            {r.label}
          </Text>
          <Text style={styles.cellNum}>{r.tally.matches}</Text>
          <Text style={styles.cellNum}>{r.tally.wins}</Text>
          <Text style={styles.cellNum}>{winPercentText(r.tally)}</Text>
          <Text style={styles.cellNum}>{avgRankText(r.tally)}</Text>
        </View>
      ))}
    </View>
  );
}

/** A segmented control. Used for the leaderboard's scope and kind. */
export function Toggle<T extends string>({
  options,
  value,
  onChange,
  testID,
}: {
  options: { id: T; label: string }[];
  value: T;
  onChange: (id: T) => void;
  testID?: string;
}) {
  return (
    <View style={styles.toggle} testID={testID}>
      {options.map((o) => {
        const on = o.id === value;
        return (
          <Pressable
            key={o.id}
            onPress={() => onChange(o.id)}
            style={[styles.toggleItem, on && styles.toggleItemOn]}
            testID={`${testID ?? 'toggle'}-${o.id}`}
            accessibilityRole="button"
            accessibilityState={{ selected: on }}
          >
            <Text style={[styles.toggleText, on && styles.toggleTextOn]}>{o.label}</Text>
          </Pressable>
        );
      })}
    </View>
  );
}

/** What a section says when there is genuinely nothing in it yet. */
export function Empty({ children, testID }: { children: ReactNode; testID?: string }) {
  return (
    <View style={[shared.card, { marginBottom: 0 }]} testID={testID}>
      <Text style={[shared.status, { marginTop: 0 }]}>{children}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  sectionTitle: {
    color: colors.text,
    fontSize: 17,
    fontWeight: '700',
    marginBottom: 4,
  },
  sectionNote: {
    color: colors.muted,
    fontSize: 13,
    marginBottom: 10,
  },
  headlineRow: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: 8,
  },
  tile: {
    flexGrow: 1,
    flexBasis: 72,
    backgroundColor: colors.surface,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 12,
    paddingVertical: 12,
    alignItems: 'center',
  },
  tileValue: {
    color: colors.text,
    fontSize: 22,
    fontWeight: '700',
  },
  tileLabel: {
    color: colors.muted,
    fontSize: 12,
    marginTop: 2,
  },
  line: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: 12,
    paddingVertical: 7,
    borderBottomWidth: StyleSheet.hairlineWidth,
    borderBottomColor: colors.border,
  },
  lineLabel: {
    color: colors.muted,
    fontSize: 14,
    flexShrink: 1,
  },
  lineValue: {
    color: colors.text,
    fontSize: 14,
    fontWeight: '600',
  },
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
    paddingVertical: 8,
    borderBottomWidth: StyleSheet.hairlineWidth,
    borderBottomColor: colors.border,
  },
  headRow: {
    borderBottomColor: colors.border,
  },
  headText: {
    color: colors.muted,
    fontSize: 11,
    fontWeight: '600',
    textTransform: 'uppercase',
  },
  cellLabel: {
    flex: 1,
    color: colors.text,
    fontSize: 14,
    paddingRight: 8,
  },
  cellNum: {
    width: 52,
    textAlign: 'right',
    color: colors.text,
    fontSize: 14,
    fontVariant: ['tabular-nums'],
  },
  toggle: {
    flexDirection: 'row',
    backgroundColor: colors.surface,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 10,
    padding: 3,
    marginBottom: 8,
  },
  toggleItem: {
    flex: 1,
    paddingVertical: 8,
    borderRadius: 8,
    alignItems: 'center',
  },
  toggleItemOn: {
    backgroundColor: colors.accentButton,
  },
  toggleText: {
    color: colors.muted,
    fontSize: 13,
    fontWeight: '600',
  },
  toggleTextOn: {
    color: colors.onAccent,
  },
});
