import { ScrollView, StyleSheet, Text, View } from 'react-native';

import type { MatchPlayer, Seat, Standing } from '@/src/api/matchTypes';
import { factText, label, playerName } from '@/src/lib/labels';
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
 */

type Props = {
  seats: Seat[];
  players: MatchPlayer[];
  viewerId: string;
  standings?: Standing[];
};

export function SeatStrip({ seats, players, viewerId, standings }: Props) {
  if (!seats.length) return null;

  return (
    <View testID="match-standings">
      <ScrollView
        horizontal
        showsHorizontalScrollIndicator={false}
        contentContainerStyle={styles.row}
        testID="seat-strip"
      >
        {seats.map((seat) => {
          const isMe = seat.playerId === viewerId;
          const player = players.find((p) => p.id === seat.playerId);
          const standing = standings?.find((s) => s.playerId === seat.playerId);
          return (
            <View
              key={seat.playerId}
              testID={`seat-${seat.playerId}`}
              style={[styles.seat, seat.active && styles.active, isMe && styles.mine]}
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
                  {standing.score} {label(standing.labelKey)}
                </Text>
              ) : null}

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
                  {factText(f)}
                </Text>
              ))}
            </View>
          );
        })}
      </ScrollView>
    </View>
  );
}

const styles = StyleSheet.create({
  row: { gap: 8, paddingVertical: 4 },
  seat: {
    minWidth: 116,
    backgroundColor: colors.surface,
    borderRadius: 10,
    borderWidth: 1,
    borderColor: colors.border,
    padding: 8,
  },
  // The seat on turn is outlined rather than filled: a filled highlight on a
  // small tile competes with the cards, which are what a player is looking at.
  active: { borderColor: colors.accent, borderWidth: 2 },
  mine: { backgroundColor: '#22304a' },
  nameRow: { flexDirection: 'row', alignItems: 'center', gap: 6 },
  name: { color: colors.text, fontWeight: '700', fontSize: 13, flexShrink: 1 },
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
  score: { color: colors.gold, fontSize: 11, fontWeight: '700', marginTop: 2 },
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
  turn: { color: colors.accent, fontSize: 11, fontWeight: '700', marginTop: 2 },
  tag: { color: colors.gold, fontSize: 11, marginTop: 2 },
  fact: { color: colors.muted, fontSize: 11, marginTop: 1 },
});
