import { useEffect, useMemo, useRef } from 'react';
import { Animated, Easing, ScrollView, StyleSheet, Text, View } from 'react-native';

import type { MatchPlayer, Seat, Standing } from '@/src/api/matchTypes';
import { Panel } from '@/src/components/match/Panel';
import { useMetrics } from '@/src/hooks/useMetrics';
import { useReducedMotion } from '@/src/hooks/useReducedMotion';
import { useSkin } from '@/src/hooks/useSkin';
import type { Metrics } from '@/src/lib/layout';
import { factText, label, playerName, shownScore } from '@/src/lib/labels';
import type { Skin } from '@/src/skins/types';

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
  const skin = useSkin();
  const styles = useMemo(() => seatStyles(metrics, skin), [metrics, skin]);

  if (!seats.length) return null;

  const tiles = seats.map((seat) => {
    const isMe = seat.playerId === viewerId;
    const player = players.find((p) => p.id === seat.playerId);
    const standing = standings?.find((s) => s.playerId === seat.playerId);
    const name = playerName(players, seat.playerId);
    return (
      <View
        key={seat.playerId}
        testID={`seat-${seat.playerId}`}
        style={[styles.seat, metrics.narrow && styles.seatNarrow, seat.active && styles.active, isMe && styles.mine]}
      >
        <View style={styles.nameRow}>
          {skin.seats.avatars ? (
            <SeatAvatar name={name} active={seat.active} styles={styles} />
          ) : null}
          {standing ? <Text style={styles.rank}>{standing.rank}</Text> : null}
          <Text style={styles.name} numberOfLines={1}>
            {name}
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
          <View style={styles.turnRow}>
            <TurnPulse color={skin.colors.accent} />
            <Text testID={`seat-active-${seat.playerId}`} style={styles.turn}>
              to play
            </Text>
          </View>
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

/**
 * An initial-circle standing in for a portrait nobody uploaded. The colour is
 * picked from the name, so a player keeps their colour across matches without
 * anything being stored — and two players at one table rarely collide.
 */
function SeatAvatar({ name, active, styles }: { name: string; active?: boolean; styles: SeatStyles }) {
  const initials =
    name
      .split(/\s+/)
      .filter(Boolean)
      .slice(0, 2)
      .map((w) => w[0]!.toUpperCase())
      .join('') || '?';
  let hash = 0;
  for (let i = 0; i < name.length; i++) hash = (hash * 31 + name.charCodeAt(i)) >>> 0;
  const fill = AVATAR_FILLS[hash % AVATAR_FILLS.length];
  return (
    <View style={[styles.avatar, { backgroundColor: fill }, active && styles.avatarActive]}>
      <Text style={styles.avatarText}>{initials}</Text>
    </View>
  );
}

/** Muted, felt-friendly fills that keep white initials readable on all of them. */
const AVATAR_FILLS = ['#7c5cbf', '#2f7fb8', '#b8722f', '#3f8f6a', '#b84f6e', '#5a7d3f'];

/**
 * The dot beside "to play", breathing. The one looping animation on the
 * board, because whose turn it is is the one fact that stays true and worth
 * noticing for as long as it is on screen. Still under reduce-motion.
 */
function TurnPulse({ color }: { color: string }) {
  const progress = useRef(new Animated.Value(0)).current;
  const stillness = useReducedMotion();

  useEffect(() => {
    if (stillness) {
      progress.setValue(1);
      return;
    }
    const loop = Animated.loop(
      Animated.sequence([
        Animated.timing(progress, {
          toValue: 1,
          duration: 700,
          easing: Easing.inOut(Easing.quad),
          useNativeDriver: true,
        }),
        Animated.timing(progress, {
          toValue: 0,
          duration: 700,
          easing: Easing.inOut(Easing.quad),
          useNativeDriver: true,
        }),
      ]),
    );
    loop.start();
    return () => loop.stop();
  }, [stillness, progress]);

  return (
    <Animated.View
      style={{
        width: 8,
        height: 8,
        borderRadius: 4,
        backgroundColor: color,
        opacity: progress.interpolate({ inputRange: [0, 1], outputRange: [0.45, 1] }),
        transform: [{ scale: progress.interpolate({ inputRange: [0, 1], outputRange: [0.85, 1.2] }) }],
      }}
    />
  );
}

function seatStyles(m: Metrics, s: Skin) {
  const colors = s.colors;
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
    mine: { backgroundColor: s.seats.avatars ? 'rgba(240, 199, 94, 0.10)' : '#22304a' },
    nameRow: { flexDirection: 'row', alignItems: 'center', gap: 6 },
    avatar: {
      width: 26,
      height: 26,
      borderRadius: 13,
      alignItems: 'center',
      justifyContent: 'center',
      borderWidth: 2,
      borderColor: 'transparent',
    },
    avatarActive: { borderColor: colors.gold },
    avatarText: { color: '#ffffff', fontWeight: '800', fontSize: 11 },
    turnRow: { flexDirection: 'row', alignItems: 'center', gap: 5, marginTop: 3 },
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
    turn: { color: colors.accent, fontSize: m.panel.bodyFont - 1, fontWeight: '700' },
    tag: { color: colors.gold, fontSize: m.panel.bodyFont - 1, marginTop: 2 },
    fact: { color: colors.muted, fontSize: m.panel.bodyFont - 1, marginTop: 1 },
  });
}

type SeatStyles = ReturnType<typeof seatStyles>;
