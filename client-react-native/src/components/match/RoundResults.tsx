import { memo, useMemo } from 'react';
import { Animated, ScrollView, StyleSheet, Text, View } from 'react-native';

import { useArrival } from '@/src/hooks/useArrival';
import { useMetrics } from '@/src/hooks/useMetrics';
import { factText, label, playerName, shownScore } from '@/src/lib/labels';
import type { Metrics } from '@/src/lib/layout';
import type { Fact, MatchPlayer, RoundLog, RoundScore, Standing } from '@/src/api/matchTypes';
import { colors } from '@/src/theme';

import { Panel } from './Panel';

/**
 * What the match did, round by round.
 *
 * The screen this replaces said "Match over" and named a winner, and that was
 * everything: a seven-deal match ended with no account of the six deals that
 * were not the last one. The board below still showed the final position, which
 * made it worse rather than better — the position a match ends in is the least
 * informative thing about it, and the settlements that decided it had been
 * wiped off the table one by one as each new round was dealt.
 *
 * It renders numbers the server computed and labels the server named, and
 * derives nothing. Which column is which, what a round is called, who took one,
 * what a delta was made of — every one of those is a fact about a game, and this
 * component is not allowed to know any game.
 */
type Props = {
  log: RoundLog;
  players: MatchPlayer[];
  standings?: Standing[];
  viewerId: string;
};

export const RoundResults = memo(function RoundResults({
  log,
  players,
  standings,
  viewerId,
}: Props) {
  const metrics = useMetrics();
  const styles = useMemo(() => roundStyles(metrics), [metrics]);

  // Columns follow the standings when there are any, so the table reads
  // best-first — and fall back to the seating when there are not.
  const columns = useMemo(() => {
    if (standings?.length) return standings.map((s) => s.playerId);
    return players.map((p) => p.id);
  }, [standings, players]);

  // A fact that is true of every round is not a fact about a round. Some games
  // rotate what a round demands and some fix it once for the match, and a table
  // that repeated the fixed one down eleven rows would be saying nothing eleven
  // times. Which of the two a given game is doing is not knowable here, so this
  // does not ask: it hoists whatever turns out to be identical throughout, and
  // leaves whatever varies where it varies.
  const { shared, perRound } = useMemo(() => splitSharedFacts(log), [log]);

  // Re-played per round, so each result announces itself rather than only the
  // first one of the match.
  const arrival = useArrival(log.rounds.length);

  if (!log.rounds.length) return null;

  const byId = new Map(standings?.map((s) => [s.playerId, s]));
  const unit = standings?.[0]?.labelKey;

  return (
    <Animated.View style={arrival}>
      <Panel
        title="Results"
        subtitle={shared.length ? shared.map((f) => factText(f, players)).join(' · ') : undefined}
        forceOpen
        testID="round-results"
        // Tinted only between rounds. At the end of a match the banner above
        // is already doing the announcing, and two tinted boxes stacked on one
        // another read as noise rather than emphasis.
        style={log.paused ? styles.paused : undefined}
      >
      <ScrollView horizontal showsHorizontalScrollIndicator style={styles.scroller}>
        <View>
          <View style={[styles.row, styles.headRow]}>
            <Text style={[styles.cell, styles.labelCell, styles.head]}>{label(log.labelKey)}</Text>
            {columns.map((id) => (
              <Text
                key={id}
                numberOfLines={1}
                style={[styles.cell, styles.head, id === viewerId && styles.mine]}
              >
                {playerName(players, id)}
              </Text>
            ))}
          </View>

          {log.rounds.map((r, i) => {
            const scores = new Map(r.scores.map((s) => [s.playerId, s]));
            const took = new Set(r.winners ?? []);
            // The row that just landed, picked out so a reader arriving at a
            // table of eleven can see which line is the news.
            const newest = i === log.rounds.length - 1;
            return (
              <View
                key={r.number}
                style={[styles.row, newest && styles.newest]}
                testID={`round-${r.number}`}
              >
                <View style={[styles.cell, styles.labelCell]}>
                  <Text style={styles.roundNumber}>{r.number}</Text>
                  {(perRound.get(r.number) ?? []).map((f, i) => (
                    <Text key={`${f.labelKey}-${i}`} style={styles.roundFact}>
                      {factText(f, players)}
                    </Text>
                  ))}
                </View>
                {columns.map((id) => {
                  const s = scores.get(id);
                  return (
                    <View key={id} style={styles.cell}>
                      <Text
                        testID={`round-${r.number}-${id}`}
                        style={[styles.delta, took.has(id) && styles.tookIt]}
                      >
                        {s ? formatDelta(s) : '—'}
                      </Text>
                      {s ? <Text style={styles.total}>{runningTotal(s)}</Text> : null}
                    </View>
                  );
                })}
              </View>
            );
          })}

          {standings?.length ? (
            <View style={[styles.row, styles.totalRow]}>
              <Text style={[styles.cell, styles.labelCell, styles.head]}>
                {unit ? label(unit) : 'Total'}
              </Text>
              {columns.map((id) => {
                const s = byId.get(id);
                return (
                  <Text key={id} testID={`round-total-${id}`} style={[styles.cell, styles.grand]}>
                    {s ? shownScore(s) : '—'}
                  </Text>
                );
              })}
            </View>
          ) : null}
        </View>
        </ScrollView>
      </Panel>
    </Animated.View>
  );
});

/**
 * Separates the facts every round carries from the ones that vary.
 *
 * Identity is the whole rendered fact — its key and its params — because two
 * rounds demanding "two sets" say the same thing and two rounds demanding
 * different numbers do not, and only the params tell them apart.
 */
function splitSharedFacts(log: RoundLog): {
  shared: Fact[];
  perRound: Map<number, Fact[]>;
} {
  const perRound = new Map<number, Fact[]>();
  if (log.rounds.length < 2) {
    for (const r of log.rounds) perRound.set(r.number, r.facts ?? []);
    return { shared: [], perRound };
  }

  const idOf = (f: Fact) => JSON.stringify([f.labelKey, f.value ?? null, f.params ?? null]);
  const counts = new Map<string, { fact: Fact; seen: number }>();
  for (const r of log.rounds) {
    // A round listing the same fact twice must not make it look universal.
    const once = new Set<string>();
    for (const f of r.facts ?? []) {
      const id = idOf(f);
      if (once.has(id)) continue;
      once.add(id);
      const entry = counts.get(id);
      if (entry) entry.seen += 1;
      else counts.set(id, { fact: f, seen: 1 });
    }
  }

  const universal = new Set<string>();
  const shared: Fact[] = [];
  for (const [id, { fact, seen }] of counts) {
    if (seen === log.rounds.length) {
      universal.add(id);
      shared.push(fact);
    }
  }
  for (const r of log.rounds) {
    perRound.set(
      r.number,
      (r.facts ?? []).filter((f) => !universal.has(idOf(f))),
    );
  }
  return { shared, perRound };
}

/** A round's own movement, signed so a reader can see which way it went. */
function formatDelta(s: RoundScore): string {
  const n = s.shown ?? s.delta;
  return n > 0 ? `+${n}` : String(n);
}

function runningTotal(s: RoundScore): string {
  return String(s.shownTotal ?? s.total);
}

function roundStyles(m: Metrics) {
  return StyleSheet.create({
    scroller: { marginTop: 4 },
    // The same tint the end-of-match banner uses, for the same reason: the
    // thing it has to beat is being mistaken for nothing having happened.
    paused: { backgroundColor: 'rgba(61, 139, 253, 0.10)', borderColor: colors.accent },
    newest: { backgroundColor: 'rgba(61, 139, 253, 0.07)', borderRadius: 4 },
    row: { flexDirection: 'row', alignItems: 'flex-start' },
    headRow: { borderBottomWidth: 1, borderBottomColor: colors.border, paddingBottom: 4 },
    totalRow: { borderTopWidth: 1, borderTopColor: colors.border, marginTop: 4, paddingTop: 6 },
    cell: {
      minWidth: m.narrow ? 68 : 88,
      paddingVertical: 4,
      paddingHorizontal: 6,
      alignItems: 'flex-start',
    },
    // The left-hand column is wider: it carries the round's own number and
    // whatever the module said was true of it.
    labelCell: { minWidth: m.narrow ? 92 : 132 },
    head: { color: colors.muted, fontSize: m.panel.bodyFont, fontWeight: '600' },
    mine: { color: colors.text },
    roundNumber: { color: colors.text, fontSize: m.panel.bodyFont, fontWeight: '600' },
    roundFact: { color: colors.muted, fontSize: Math.max(10, m.panel.bodyFont - 3) },
    delta: { color: colors.text, fontSize: m.panel.bodyFont },
    // Whoever took the round, marked on the cell rather than in a legend.
    tookIt: { color: colors.gold, fontWeight: '700' },
    total: { color: colors.muted, fontSize: Math.max(10, m.panel.bodyFont - 3) },
    grand: { color: colors.text, fontSize: m.panel.bodyFont, fontWeight: '700' },
  });
}
