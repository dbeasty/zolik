import { useMemo } from 'react';
import { ScrollView, StyleSheet, Text, View } from 'react-native';

import type { MatchPlayer, Seat, Standing } from '@/src/api/matchTypes';
import { Panel } from '@/src/components/match/Panel';
import { useMetrics } from '@/src/hooks/useMetrics';
import type { Metrics } from '@/src/lib/layout';
import { factText, label, playerName, shownScore } from '@/src/lib/labels';
import { colors } from '@/src/theme';

/**
 * The table: who is playing, whose turn it is, and their own numbers.
 *
 * Every number here comes from the server pre-resolved. This component adds
 * them up, compares them and interprets them exactly never — it lays out
 * whatever facts a seat carries, which is why the same strip shows a Prší card
 * count, a Canasta partnership score and a poker stack without knowing that any
 * of those exist.
 *
 * Standing (rank + running score) lives in the same tile as the name it
 * belongs to rather than in a scoreboard of its own further down the screen —
 * a rank is a property of a player, not a separate list a reader has to match
 * back up to one.
 *
 * On a narrow screen the strip stops scrolling and wraps instead — a
 * horizontal scroller with no scrollbar hint was hiding the fourth seat off
 * the right edge of a phone, on the one screen where knowing who's at the
 * table is the point.
 */

type Props = {
  seats: Seat[];
  players: MatchPlayer[];
  viewerId: string;
  standings?: Standing[];
  panelId?: string;
  minimized?: boolean;
  onToggleMinimized?: () => void;
};

export function SeatStrip({ seats, players, viewerId, standings, panelId, minimized, onToggleMinimized }: Props) {
  const metrics = useMetrics();
  const styles = useMemo(() => seatStyles(metrics), [metrics]);

  if (!seats.length) return null;

  const tiles = seats.map((seat) => {
    const isMe = seat.playerId === viewerId;
    const player = players.find((p) => p.id === seat.playerId);
    const standing = standings?.find((s) => s.playerId === seat.playerId);
    return (
      <View
        key={seat.playerId}
        testID={`seat-${seat.playerId}`}
        style={[styles.seat, metrics.narrow && styles.seatNarrow, seat.active && styles.active, isMe && styles.mine]}
      >
        <View style={styles.nameRow}>
          {standing ? <Text style={styles.rank}>{standing.rank}</Text> : null}
          <Text style={styles.name} numberOfLines={1}>
            {playerName(players, seat.playerId)}
          </Text>
          {player?.isAI ? <Text style={styles.badge}>BOT</Text> : null}
        </View>

        {standing ? (
          <Text testID={`standing-${seat.playerId}`} style={styles.score}>
            {shownScore(standing)} {label(standing.labelKey)}
          </Text>
        ) : null}

        {/* What the ranking was decided on, where the module says so — a
            scoreboard that shows only a total cannot explain a tiebreak. */}
        {(standing?.facts ?? []).map((f, i) => (
          <Text key={`${f.labelKey}-${i}`} style={styles.fact}>
            {factText(f, players)}
          </Text>
        ))}

        {/* Turn is a pushed fact, not something worked out from offers. */}
        {seat.active ? (
          <Text testID={`seat-active-${seat.playerId}`} style={styles.turn}>
            ● to play
          </Text>
        ) : null}

        {(seat.labelKeys ?? []).map((key) => (
          <Text key={key} style={styles.tag}>
            {label(key)}
          </Text>
        ))}

        {(seat.facts ?? []).map((f, i) => (
          <Text key={`${f.labelKey}-${i}`} style={styles.fact}>
            {factText(f, players)}
          </Text>
        ))}
      </View>
    );
  });

  return (
    <Panel
      panelId={panelId}
      title="Players"
      minimized={minimized}
      onToggleMinimized={onToggleMinimized}
      testID="match-standings"
      summary={
        <View style={styles.summary} testID="match-standings-summary">
          {seats.map((seat) => {
            const standing = standings?.find((s) => s.playerId === seat.playerId);
            // The one number worth carrying onto the collapsed rail — a
            // running score where the game has one, otherwise whatever the
            // seat's own first fact says (a card count, a stack size). Either
            // way it's a value the open tile already shows, read off the
            // same fields rather than a second, cheaper copy of them.
            const status = standing
              ? String(shownScore(standing))
              : seat.facts?.[0]
                ? factText(seat.facts[0], players)
                : undefined;
            return (
              <View
                key={seat.playerId}
                testID={`seat-summary-${seat.playerId}`}
                style={[styles.summaryPill, seat.active && styles.summaryPillActive]}
              >
                <Text style={styles.summaryName} numberOfLines={1}>
                  {seat.active ? '● ' : ''}
                  {playerName(players, seat.playerId)}
                </Text>
                {status ? (
                  <Text style={styles.summaryStatus} numberOfLines={1}>
                    {status}
                  </Text>
                ) : null}
              </View>
            );
          })}
        </View>
      }
    >
      {metrics.narrow ? (
        <View style={styles.wrap} testID="seat-strip">
          {tiles}
        </View>
      ) : (
        <ScrollView
          horizontal
          showsHorizontalScrollIndicator={false}
          contentContainerStyle={styles.row}
          testID="seat-strip"
        >
          {tiles}
        </ScrollView>
      )}
    </Panel>
  );
}

function seatStyles(m: Metrics) {
  return StyleSheet.create({
    row: { gap: 8, paddingVertical: 4 },
    summary: { flexDirection: 'row', flexShrink: 1, minWidth: 0, gap: 6, overflow: 'hidden' },
    summaryPill: {
      flexDirection: 'row',
      alignItems: 'center',
      gap: 4,
      borderRadius: 6,
      borderWidth: 1,
      borderColor: colors.border,
      paddingHorizontal: 6,
      paddingVertical: 2,
      flexShrink: 0,
    },
    summaryPillActive: { borderColor: colors.accent },
    summaryName: { color: colors.text, fontSize: m.panel.bodyFont, fontWeight: '700' },
    summaryStatus: { color: colors.gold, fontSize: m.panel.bodyFont - 1, fontWeight: '700' },
    // alignItems: flex-start — a seat with more to show (standings, tags,
    // facts) would otherwise stretch every seat sharing its row to match it.
    wrap: { flexDirection: 'row', flexWrap: 'wrap', alignItems: 'flex-start', gap: 8, paddingVertical: 4 },
    seat: {
      minWidth: 116,
      backgroundColor: colors.surface,
      borderRadius: 10,
      borderWidth: 1,
      borderColor: colors.border,
      padding: 8,
    },
    // Two to a row rather than each claiming the full width — a phone still
    // gets every seat on screen without scrolling one of them off it.
    seatNarrow: { flexGrow: 1, flexBasis: '47%', minWidth: 0 },
    // The seat on turn is outlined rather than filled: a filled highlight on a
    // small tile competes with the cards, which are what a player is looking at.
    active: { borderColor: colors.accent, borderWidth: 2 },
    mine: { backgroundColor: '#22304a' },
    nameRow: { flexDirection: 'row', alignItems: 'center', gap: 6 },
    name: { color: colors.text, fontWeight: '700', fontSize: m.panel.bodyFont + 1, flexShrink: 1 },
    rank: {
      color: colors.onAccent,
      backgroundColor: colors.gold,
      fontSize: 10,
      fontWeight: '700',
      width: 15,
      height: 15,
      lineHeight: 15,
      textAlign: 'center',
      borderRadius: 8,
      overflow: 'hidden',
    },
    score: { color: colors.gold, fontSize: m.panel.bodyFont - 1, fontWeight: '700', marginTop: 2 },
    badge: {
      color: colors.onAccent,
      backgroundColor: colors.muted,
      fontSize: 9,
      fontWeight: '700',
      paddingHorizontal: 4,
      paddingVertical: 1,
      borderRadius: 4,
      overflow: 'hidden',
    },
    turn: { color: colors.accent, fontSize: m.panel.bodyFont - 1, fontWeight: '700', marginTop: 2 },
    tag: { color: colors.gold, fontSize: m.panel.bodyFont - 1, marginTop: 2 },
    fact: { color: colors.muted, fontSize: m.panel.bodyFont - 1, marginTop: 1 },
  });
}
