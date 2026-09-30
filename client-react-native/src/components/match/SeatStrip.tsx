import { type ReactNode, useEffect, useMemo, useRef, useState } from 'react';
import { Animated, Easing, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';

import type { MatchPlayer, Seat, Standing } from '@/src/api/matchTypes';
import { Avatar } from '@/src/components/avatars/Avatar';
import { avatarFor } from '@/src/components/avatars/catalogue';
import { Panel, type Measurable } from '@/src/components/match/Panel';
import { seatElementId } from '@/src/lib/flights';
import { ms } from '@/src/lib/motion';
import { useMetrics } from '@/src/hooks/useMetrics';
import { useReducedMotion } from '@/src/hooks/useReducedMotion';
import { useSkin } from '@/src/hooks/useSkin';
import type { Metrics } from '@/src/lib/layout';
import { factText, isDealerLabel, label, playerName, shownScore } from '@/src/lib/labels';
import { partnerText, partnersOf } from '@/src/lib/sides';
import type { Skin } from '@/src/skins/types';
import { t } from '@/src/lib/i18n';

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
 *
 * On a wide one the tiles share the row rather than each taking the width its
 * longest line asks for, and past four seats each tile stacks its name under
 * its avatar to need less of it — a six-seat Samba table put two players off
 * the right edge of an 800-pixel window with the same missing hint. Where six
 * still do not fit, the row scrolls with its bar showing and an arrow at
 * whichever end has more seats beyond it (`SeatScroller`).
 *
 * Where the game has sides, each tile says whose side that seat is on. The
 * lobby has always shown the partnerships it was arranging and then the board
 * dropped the subject entirely, which left anyone who did not do the seating
 * to work their partner out from whose melds landed in their spread. Said
 * twice, deliberately: the line names the partners, and your own side wears a
 * colour — the words answer "who", the colour answers "which of these four"
 * without being read at all, which is what a glance at a board is.
 */

type Props = {
  seats: Seat[];
  players: MatchPlayer[];
  viewerId: string;
  standings?: Standing[];
  panelId?: string;
  minimized?: boolean;
  onToggleMinimized?: () => void;
  /**
   * Publishes where each seat tile is, under `seatElementId(playerId)` — a
   * seat is where a player's unseen cards visually live, so it is where a
   * card in flight leaves from or lands when their hand isn't on screen.
   */
  registerSpot?: (elementId: string, node: Measurable | null) => void;
  /**
   * Open the account behind a seat's score. Where it is given, the score on
   * the tile and on the collapsed rail is a control rather than a label — a
   * number that jumped by a thousand is the thing a player wants to ask about.
   */
  onOpenScore?: (playerId: string) => void;
};

export function SeatStrip({ seats, players, viewerId, standings, panelId, minimized, onToggleMinimized, registerSpot, onOpenScore }: Props) {
  const metrics = useMetrics();
  const skin = useSkin();
  // Asked for stillness, the seat on turn keeps its outline and its shadow
  // and simply does not rise — the fact is still on screen, in the two cues
  // that were never movement.
  const stillness = useReducedMotion();
  const styles = useMemo(() => seatStyles(metrics, skin), [metrics, skin]);
  const [stripWidth, setStripWidth] = useState(0);

  if (!seats.length) return null;

  // The viewer's own side, which is empty in a game without partnerships —
  // and has to be, because tinting a lone seat "ours" would mark every seat
  // at a Prší table as a team of one.
  const myPartners = partnersOf(seats, viewerId);
  const ourSide = new Set(myPartners.length ? [viewerId, ...myPartners] : []);
  const crowded = !metrics.narrow && seats.length > 4;
  // An equal share of the strip, set outright rather than left to flex: in a
  // horizontal scroller a tile is otherwise as wide as its longest line, and
  // "Team: Bartholomew" never wraps. Floored so a name is still a name; below
  // the floor the row scrolls instead (`SeatScroller`).
  const share =
    !metrics.narrow && stripWidth > 0
      ? Math.max(crowded ? SEAT_MIN_CROWDED : SEAT_MIN, Math.floor((stripWidth - SEAT_GAP * (seats.length - 1)) / seats.length))
      : undefined;

  const tiles = seats.map((seat) => {
    const isMe = seat.playerId === viewerId;
    // Partners of the viewer, not of each other: an opponent pair is still
    // worth naming on their own tiles, but it is not *my* team and must not
    // wear the colour that says so.
    const isPartner = ourSide.has(seat.playerId) && !isMe;
    const partners = partnerText(seats, players, seat.playerId, viewerId);
    const player = players.find((p) => p.id === seat.playerId);
    const standing = standings?.find((s) => s.playerId === seat.playerId);
    const name = playerName(players, seat.playerId);
    const avatar = skin.seats.avatars ? (
      <Avatar
        spec={avatarFor(seat.playerId, !!player?.isAI, player?.avatar)}
        size={metrics.seat.avatar}
        ringColor={seat.active ? skin.colors.gold : undefined}
      />
    ) : null;
    const rank = standing ? <Text style={styles.rank}>{standing.rank}</Text> : null;
    const bot = player?.isAI ? <Text style={styles.badge}>BOT</Text> : null;
    return (
      <View
        key={seat.playerId}
        ref={(n) => registerSpot?.(seatElementId(seat.playerId), n as unknown as Measurable | null)}
        testID={`seat-${seat.playerId}`}
        style={[
          styles.seat,
          metrics.narrow ? styles.seatNarrow : share ? { width: share, minWidth: 0 } : styles.seatShared,
          seat.active && styles.active,
          isPartner && styles.teammate,
          isMe && styles.mine,
          // Shadow only, and only on the measured node — the *lift* goes on
          // the wrapper inside, because a transform here would move the rect
          // this node is registered under and a card would fly to where the
          // seat used to be.
          seat.active && styles.activeShadow,
        ]}
      >
        <View style={seat.active && !stillness ? styles.lifted : undefined}>
        {crowded ? (
          // Past four seats: the name on a line of its own under the avatar,
          // so the tile is as wide as its longest line rather than both.
          <>
            <View style={styles.nameRow}>
              {avatar}
              {rank}
              {bot}
            </View>
            <Text style={[styles.name, styles.nameCrowded]} numberOfLines={1}>
              {name}
            </Text>
          </>
        ) : (
          <View style={styles.nameRow}>
            {avatar}
            {rank}
            <Text style={styles.name} numberOfLines={1}>
              {name}
            </Text>
            {bot}
          </View>
        )}

        {standing && onOpenScore ? (
          <Pressable
            onPress={() => onOpenScore(seat.playerId)}
            hitSlop={8}
            accessibilityRole="button"
            accessibilityHint={t('score.open')}
            testID={`standing-open-${seat.playerId}`}
          >
            <Text testID={`standing-${seat.playerId}`} style={[styles.score, styles.scoreLink]}>
              {shownScore(standing)} {label(standing.labelKey)}
            </Text>
          </Pressable>
        ) : standing ? (
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
              {t('match.toPlay')}
            </Text>
          </View>
        ) : null}

        {/* Who this seat plays with. On the viewer's partner it is the
            shorter, plainer sentence — "Your team" says the one thing that
            tile is there to say, where "Team: you" makes a reader translate
            it. Drawn at all only where the module gave the seat a side. */}
        {partners ? (
          <Text testID={`seat-team-${seat.playerId}`} style={[styles.team, isPartner && styles.teamMine]}>
            {isPartner ? t('match.yourTeam') : t('match.teammates', { names: partners })}
          </Text>
        ) : null}

        {/* The seat's marks. The dealer's is drawn as the button itself
            rather than spelled out — at a real table the button is an object
            that sits in front of somebody, and it is read by *where it is*
            rather than by reading a word off it. The word is still there, and
            still translated, for a screen reader and for anyone who has never
            seen one. */}
        {(seat.labelKeys ?? []).map((key) =>
          isDealerLabel(key) ? (
            <View key={key} style={styles.dealerRow} testID={`seat-dealer-${seat.playerId}`}>
              <DealerButton size={metrics.seat.avatarCompact} skin={skin} />
              <Text style={styles.tag}>{label(key)}</Text>
            </View>
          ) : (
            <Text key={key} style={styles.tag}>
              {label(key)}
            </Text>
          ),
        )}

        {(seat.facts ?? []).map((f, i) => (
          <Text key={`${f.labelKey}-${i}`} style={styles.fact}>
            {factText(f, players)}
          </Text>
        ))}
        </View>
      </View>
    );
  });

  return (
    <Panel
      panelId={panelId}
      title={t('match.players')}
      minimized={minimized}
      onToggleMinimized={onToggleMinimized}
      testID="match-standings"
      summary={
        <View style={styles.summary} testID="match-standings-summary">
          {seats.map((seat) => {
            const standing = standings?.find((s) => s.playerId === seat.playerId);
            const player = players.find((p) => p.id === seat.playerId);
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
                style={[
                  styles.summaryPill,
                  // The collapsed rail keeps the one cue that survives having
                  // no room for words: your side is tinted, theirs is not.
                  ourSide.has(seat.playerId) && styles.summaryPillOurs,
                  seat.active && styles.summaryPillActive,
                ]}
              >
                {/* The same face, small. Four names in a row is a list to be
                    read; four faces is a table to be glanced at. */}
                {skin.seats.avatars ? (
                  <Avatar
                    spec={avatarFor(seat.playerId, !!player?.isAI, player?.avatar)}
                    size={metrics.seat.avatarCompact}
                  />
                ) : null}
                {/* The one mark worth the width on a collapsed rail: whose
                    deal it is survives being folded away, where "folded" and
                    "all in" are already said by the numbers beside them. */}
                {(seat.labelKeys ?? []).some(isDealerLabel) ? (
                  <DealerButton size={Math.round(metrics.seat.avatarCompact * 0.8)} skin={skin} />
                ) : null}
                <Text style={styles.summaryName} numberOfLines={1}>
                  {seat.active ? '● ' : ''}
                  {playerName(players, seat.playerId)}
                </Text>
                {status && standing && onOpenScore ? (
                  // Only the number, not the pill: the rail itself is how the
                  // panel is opened out again, and that stays where it was.
                  <Pressable
                    onPress={() => onOpenScore(seat.playerId)}
                    hitSlop={8}
                    accessibilityRole="button"
                    accessibilityHint={t('score.open')}
                    testID={`seat-summary-score-${seat.playerId}`}
                  >
                    <Text style={[styles.summaryStatus, styles.scoreLink]} numberOfLines={1}>
                      {status}
                    </Text>
                  </Pressable>
                ) : status ? (
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
        <SeatScroller styles={styles} onWidth={setStripWidth}>
          {tiles}
        </SeatScroller>
      )}
    </Panel>
  );
}

/**
 * The wide strip's row. Scrolls only once the tiles, at their narrowest, no
 * longer fit — and then says so: the bar is shown, and an arrow sits at each
 * end that has seats past it, because a mouse wheel scrolls vertically and a
 * row that only moves sideways under a trackpad gesture is, for most people
 * at a desk, a row that does not move.
 */
function SeatScroller({
  styles,
  onWidth,
  children,
}: {
  styles: SeatStyles;
  onWidth: (width: number) => void;
  children: ReactNode;
}) {
  const scroller = useRef<ScrollView>(null);
  const [view, setView] = useState(0);
  const [content, setContent] = useState(0);
  const [x, setX] = useState(0);
  const before = x > 1;
  const after = x + view < content - 1;
  // Most of a viewport at a time, so the seat cut off at the edge is still in
  // sight after the step and a reader keeps their place.
  const step = (dir: 1 | -1) =>
    scroller.current?.scrollTo({ x: Math.max(0, Math.min(content - view, x + dir * view * 0.8)), animated: true });

  return (
    <View style={styles.scroller}>
      <ScrollView
        ref={scroller}
        horizontal
        showsHorizontalScrollIndicator={content > view + 1}
        contentContainerStyle={styles.row}
        onLayout={(e) => {
          setView(e.nativeEvent.layout.width);
          onWidth(e.nativeEvent.layout.width);
        }}
        onContentSizeChange={(w) => setContent(w)}
        onScroll={(e) => setX(e.nativeEvent.contentOffset.x)}
        scrollEventThrottle={32}
        testID="seat-strip"
      >
        {children}
      </ScrollView>
      {before ? (
        <Pressable
          testID="seat-strip-left"
          accessibilityRole="button"
          accessibilityLabel={t('seats.scrollLeft')}
          onPress={() => step(-1)}
          style={[styles.arrow, styles.arrowLeft]}
        >
          <Text style={styles.arrowText}>‹</Text>
        </Pressable>
      ) : null}
      {after ? (
        <Pressable
          testID="seat-strip-right"
          accessibilityRole="button"
          accessibilityLabel={t('seats.scrollRight')}
          onPress={() => step(1)}
          style={[styles.arrow, styles.arrowRight]}
        >
          <Text style={styles.arrowText}>›</Text>
        </Pressable>
      ) : null}
    </View>
  );
}

type SeatStyles = ReturnType<typeof seatStyles>;

const SEAT_GAP = 8;
const SEAT_MIN = 116;
const SEAT_MIN_CROWDED = 104;

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
          duration: ms(700),
          easing: Easing.inOut(Easing.quad),
          useNativeDriver: true,
        }),
        Animated.timing(progress, {
          toValue: 0,
          duration: ms(700),
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

/**
 * The dealer button: the little disc that sits in front of whoever deals, and
 * moves round the table a seat at a time.
 *
 * An object rather than a word, because that is what it is — the whole point
 * of a button at a real table is that it is read from across one without
 * being read. Sized from the metrics like every other size here (a skin may
 * repaint it and may not resize it), and drawn in the skin's own gold on the
 * card's own stock, so it reads as a chip lying on the felt.
 */
function DealerButton({ size, skin }: { size: number; skin: Skin }) {
  return (
    <View
      style={{
        width: size,
        height: size,
        borderRadius: size / 2,
        backgroundColor: skin.colors.cardBg,
        borderWidth: 1.5,
        borderColor: skin.colors.gold,
        alignItems: 'center',
        justifyContent: 'center',
      }}
    >
      <Text
        style={{
          color: skin.card.ink,
          fontWeight: '700',
          fontSize: Math.max(8, Math.round(size * 0.6)),
          lineHeight: Math.max(9, Math.round(size * 0.7)),
        }}
      >
        D
      </Text>
    </View>
  );
}

function seatStyles(m: Metrics, s: Skin) {
  const colors = s.colors;
  return StyleSheet.create({
    // flexGrow: the row is at least as wide as the strip, so `seatShared`
    // tiles spread across it rather than bunching at the left.
    row: { gap: SEAT_GAP, paddingVertical: 4, flexGrow: 1 },
    scroller: { position: 'relative' },
    arrow: {
      position: 'absolute',
      top: '50%',
      marginTop: -18,
      width: 36,
      height: 36,
      borderRadius: 18,
      alignItems: 'center',
      justifyContent: 'center',
      backgroundColor: colors.surface,
      borderWidth: 1,
      borderColor: colors.accent,
      shadowColor: '#000',
      shadowOffset: { width: 0, height: 2 },
      shadowOpacity: 0.4,
      shadowRadius: 6,
      elevation: 6,
    },
    arrowLeft: { left: 2 },
    arrowRight: { right: 2 },
    arrowText: { color: colors.text, fontSize: 24, lineHeight: 26, fontWeight: '700' },
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
    summaryPillOurs: { backgroundColor: s.seats.avatars ? 'rgba(240, 199, 94, 0.22)' : '#22304a' },
    summaryName: { color: colors.text, fontSize: m.panel.bodyFont, fontWeight: '700' },
    summaryStatus: { color: colors.gold, fontSize: m.panel.bodyFont - 1, fontWeight: '700' },
    // alignItems: flex-start — a seat with more to show (standings, tags,
    // facts) would otherwise stretch every seat sharing its row to match it.
    wrap: { flexDirection: 'row', flexWrap: 'wrap', alignItems: 'flex-start', gap: 8, paddingVertical: 4 },
    seat: {
      minWidth: SEAT_MIN,
      backgroundColor: colors.surface,
      borderRadius: 10,
      borderWidth: 1,
      borderColor: colors.border,
      padding: 8,
    },
    // Two to a row rather than each claiming the full width — a phone still
    // gets every seat on screen without scrolling one of them off it.
    seatNarrow: { flexGrow: 1, flexBasis: '47%', minWidth: 0 },
    // Equal shares of the row, none narrower than `seat.minWidth`; past that
    // the row scrolls instead of squeezing a name down to its first letter.
    seatShared: { flexGrow: 1, flexBasis: 0 },
    // The seat on turn is outlined rather than filled: a filled highlight on a
    // small tile competes with the cards, which are what a player is looking at.
    active: { borderColor: colors.accent, borderWidth: 2 },
    // The seat on turn sits a little proud of the felt. Three pixels and a
    // deeper shadow, which is as much as a tile can rise before the row it is
    // in starts to look ragged.
    activeShadow: {
      shadowColor: '#000',
      shadowOffset: { width: 0, height: 6 },
      shadowOpacity: 0.38,
      shadowRadius: 10,
      elevation: 8,
    },
    lifted: { transform: [{ translateY: -3 }] },
    mine: { backgroundColor: s.seats.avatars ? 'rgba(240, 199, 94, 0.22)' : '#22304a' },
    // Your partner, in the same colour as your own seat and weaker: the pair
    // reads as one block at a glance, and which of the two is you is still
    // never in doubt.
    teammate: { backgroundColor: s.seats.avatars ? 'rgba(240, 199, 94, 0.13)' : '#1e2a3d' },
    nameRow: { flexDirection: 'row', alignItems: 'center', gap: 6 },
    nameCrowded: { marginTop: 4 },
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
    // Says "this opens something" without taking any room: a dotted rule
    // under the figure, the convention for a term with an explanation.
    scoreLink: { textDecorationLine: 'underline', textDecorationStyle: 'dotted' },
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
    dealerRow: { flexDirection: 'row', alignItems: 'center', gap: 4, marginTop: 2 },
    // Louder than a fact, quieter than a tag: who you are playing with is not
    // a number and not a warning.
    team: { color: colors.text, fontSize: m.panel.bodyFont - 1, fontWeight: '600', marginTop: 2 },
    // Your partner's line is gold, which is what does the work the tint alone
    // could not: a background at a twentieth opacity is the right *weight* for
    // a secondary cue and, measured against the felt, very nearly invisible.
    // Same size, so this stays a colour change and not a layout one.
    teamMine: { color: colors.gold, fontWeight: '700' },
    fact: { color: colors.muted, fontSize: m.panel.bodyFont - 1, marginTop: 1 },
  });
}
