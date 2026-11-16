import { memo, useEffect, useMemo, useRef, useState } from 'react';
import {
  Animated,
  Platform,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  View,
  useWindowDimensions,
} from 'react-native';

import { useArrival } from '@/src/hooks/useArrival';
import { useMetrics } from '@/src/hooks/useMetrics';
import { factText, label, playerName, shownScore } from '@/src/lib/labels';
import type { Metrics } from '@/src/lib/layout';
import type { Fact, MatchPlayer, RoundLog, RoundScore, Seat, Standing } from '@/src/api/matchTypes';
import { useSkin } from '@/src/hooks/useSkin';
import type { Skin } from '@/src/skins/types';

import { Panel } from './Panel';
import { sidesOf } from './ScoreSheet';
import { t } from '@/src/lib/i18n';

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
  /** Who sits with whom: a partnership is one column, not one per member. */
  seats?: Seat[];
  standings?: Standing[];
  viewerId: string;
  /** Open the account behind one cell: that player, that round. */
  onOpenScore?: (playerId: string, round: number) => void;
  /**
   * Folded away while a round is being played, like any other panel. Absent
   * between rounds and at the end, when the results are the news: the panel
   * then opens for each new round and can still be folded by hand, so a long
   * match never stands between the player and the way on.
   */
  minimized?: boolean;
  onToggleMinimized?: () => void;
};

export const RoundResults = memo(function RoundResults({
  log,
  players,
  seats,
  standings,
  viewerId,
  onOpenScore,
  minimized,
  onToggleMinimized,
}: Props) {
  const metrics = useMetrics();
  const skin = useSkin();
  const styles = useMemo(() => roundStyles(metrics, skin), [metrics, skin]);

  // A column per side: the viewer's own first, the rest best first. Partners
  // share a score, so a partnership printed a column per member repeated
  // every number once per partner and never said the two columns were the
  // same team. Each column is keyed by one member — the viewer where the
  // viewer is on it — and every member's row reads the same.
  //
  // The viewer's column leads because it is the one they read every time;
  // ranked in among the others it moved sideways each time the standings
  // changed, and had to be found again.
  const columns = useMemo(
    () =>
      viewerFirst(sidesOf(seats ?? [], standings, players), viewerId).map((members) => ({
        id: members.includes(viewerId) ? viewerId : members[0],
        members,
      })),
    [seats, standings, players, viewerId],
  );
  const headOf = (members: string[]) =>
    members.length > 1
      ? t('match.teammates', { names: members.map((m) => playerName(players, m)).join(' & ') })
      : playerName(players, members[0]);

  // A fact that is true of every round is not a fact about a round. Some games
  // rotate what a round demands and some fix it once for the match, and a table
  // that repeated the fixed one down eleven rows would be saying nothing eleven
  // times. Which of the two a given game is doing is not knowable here, so this
  // does not ask: it hoists whatever turns out to be identical throughout, and
  // leaves whatever varies where it varies.
  const { shared, perRound } = useMemo(() => splitSharedFacts(log), [log]);

  // Inside a horizontal ScrollView the table has no width to fill, so it took
  // its content's natural width: every heading on one line, a two-name team
  // heading as wide as both names, and the panel scrolling sideways for a
  // table that would have fitted had its headings wrapped. The table is given
  // the room it is shown in, so the columns share it and their headings wrap,
  // and only a table whose columns cannot fit at their minimum width scrolls.
  //
  // Where even that would not fit, the table closes up before it scrolls:
  // narrower columns, a narrower round column and less padding. That is
  // decided by the room the panel has, not by the window, because a wide
  // window with the board in two columns gives the results a column as
  // narrow as a phone's, and a table sized for the window scrolled there.
  const [room, setRoom] = useState(0);
  const roomy = styles.labelCell.width + columns.length * styles.cell.minWidth;
  const compact = room > 0 && roomy > room;
  const floor = compact ? styles.labelCompact.width + columns.length * styles.cellCompact.minWidth : roomy;
  const overflows = room > 0 && floor > room;
  const cs = [styles.cell, compact && styles.cellCompact];
  const ls = [styles.labelCell, compact && styles.labelCompact];

  // Re-played per round, so each result announces itself rather than only the
  // first one of the match.
  const arrival = useArrival(log.rounds.length);

  // A match of forty hands is forty rows, and between hands the control to
  // deal the next one sits under the last of them: the player scrolled past
  // the whole match every hand to reach it. The table shows the latest few
  // rounds — as many as the window has height for — and the rest on request,
  // inside a box of its own height that scrolls, so even the full table never
  // pushes the way on off the screen.
  const { height } = useWindowDimensions();
  const latest = latestRows(height);
  const [all, setAll] = useState(false);
  const hidden = Math.max(0, log.rounds.length - latest);
  const capped = hidden > 0;
  const rows = capped && !all ? log.rounds.slice(-latest) : log.rounds;
  const fullHeight = Math.max(200, Math.round(height * 0.45));
  const rowsRef = useRef<ScrollView>(null);
  // Opened onto the newest rows, which are the news; the old ones are above.
  useEffect(() => {
    if (all) rowsRef.current?.scrollToEnd({ animated: false });
  }, [all, log.rounds.length]);

  // Held open while the results are the news, but never forced: a player who
  // has read them folds them away, and the next round opens them again.
  const [folded, setFolded] = useState(false);
  useEffect(() => setFolded(false), [log.rounds.length]);

  if (!log.rounds.length) return null;

  const byId = new Map(standings?.map((s) => [s.playerId, s]));
  const unit = standings?.[0]?.labelKey;
  const foldable = !!onToggleMinimized;
  const isFolded = foldable ? minimized : folded;
  const toggleFolded = foldable ? onToggleMinimized : () => setFolded((was) => !was);

  // What a folded panel still says: the viewer's own standing, since that is
  // the number a player keeps glancing up for mid-round.
  const mine = byId.get(viewerId);
  const lastMine = log.rounds[log.rounds.length - 1]?.scores.find((x) => x.playerId === viewerId);
  const summary = mine || lastMine ? (
    <Text style={styles.summary} testID="round-results-summary">
      {label('results.you')}{' '}
      <Text style={styles.summaryScore}>{mine ? shownScore(mine) : runningTotal(lastMine!)}</Text>
      {unit ? ` ${label(unit)}` : ''}
    </Text>
  ) : undefined;

  return (
    <Animated.View style={arrival}>
      <Panel
        title={t('match.results')}
        subtitle={shared.length ? shared.map((f) => factText(f, players)).join(' · ') : undefined}
        panelId="results"
        minimized={isFolded}
        onToggleMinimized={toggleFolded}
        summary={summary}
        accessory={
          capped && !isFolded ? (
            <Pressable
              testID="round-results-toggle"
              accessibilityRole="button"
              accessibilityState={{ expanded: all }}
              {...((Platform.OS === 'web' ? { 'aria-expanded': all } : {}) as object)}
              accessibilityLabel={
                all ? t('match.resultsShowLatest') : t('match.resultsShowAll', { n: log.rounds.length })
              }
              onPress={() => setAll((was) => !was)}
              hitSlop={8}
              style={styles.toggle}
            >
              <Text style={styles.toggleText}>
                {all ? `${t('match.resultsLatest')} ⌃` : `${t('match.resultsAll', { n: log.rounds.length })} ⌄`}
              </Text>
            </Pressable>
          ) : undefined
        }
        testID="round-results"
        // Tinted only between rounds. At the end of a match the banner above
        // is already doing the announcing, and two tinted boxes stacked on one
        // another read as noise rather than emphasis.
        style={log.paused ? styles.paused : undefined}
      >
        <ScrollView
          horizontal
          showsHorizontalScrollIndicator
          style={styles.scroller}
          contentContainerStyle={[styles.scrollerContent, overflows && styles.scrollerOverflow]}
          onLayout={(e) => setRoom(e.nativeEvent.layout.width)}
        >
          <View style={[styles.table, room > 0 && { width: Math.max(room, floor) }]}>
            <View style={[styles.row, styles.headRow]}>
              <Text style={[cs, ls, styles.head]}>
                {label(log.labelKey)}
              </Text>
              {columns.map(({ id, members }) => (
                <View key={id} style={[cs, id === viewerId && styles.mineCol]} testID={`round-head-${id}`}>
                  <Text
                    numberOfLines={members.length > 1 ? undefined : compact ? 1 : 2}
                    style={[styles.name, styles.head, id === viewerId && styles.mine]}
                  >
                    {headOf(members)}
                  </Text>
                  {/* Set under the name rather than after it: a column is
                      narrow, and a marker appended inline is the half that
                      gets truncated away on the one column that needs it. */}
                  {id === viewerId ? (
                    <Text style={[styles.name, styles.youTag]}>{label('results.you')}</Text>
                  ) : null}
                </View>
              ))}
            </View>

            <ScrollView
              ref={rowsRef}
              nestedScrollEnabled
              showsVerticalScrollIndicator
              persistentScrollbar
              scrollEnabled={all}
              style={all ? { maxHeight: fullHeight } : undefined}
              testID="round-results-rows"
            >
            {rows.map((r) => {
              const scores = new Map(r.scores.map((s) => [s.playerId, s]));
              const took = new Set(r.winners ?? []);
              // The row that just landed, picked out so a reader arriving at a
              // table of eleven can see which line is the news.
              const newest = r === log.rounds[log.rounds.length - 1];
              return (
                <View
                  key={r.number}
                  style={[styles.row, newest && styles.newest]}
                  testID={`round-${r.number}`}
                >
                  <View style={[cs, ls]}>
                    <Text style={styles.roundNumber}>{r.number}</Text>
                    {r.headline ? (
                      <Text style={styles.roundFact} testID={`round-${r.number}-headline`}>
                        {factText(r.headline, players)}
                      </Text>
                    ) : null}
                    {(perRound.get(r.number) ?? []).map((f, i) => (
                      <Text key={`${f.labelKey}-${i}`} style={styles.roundFact}>
                        {factText(f, players)}
                      </Text>
                    ))}
                  </View>
                  {columns.map(({ id, members }) => {
                    const s = scores.get(id) ?? members.map((m) => scores.get(m)).find(Boolean);
                    const isMine = id === viewerId;
                    const moved = s ? (s.shown ?? s.delta) : 0;
                    const content = (
                      <>
                        <Text
                          testID={`round-${r.number}-${id}`}
                          style={[
                            styles.delta,
                            // The viewer's own column reads as theirs at a
                            // glance: bold, and coloured by which way it went.
                            isMine && styles.mineDelta,
                            isMine && moved > 0 && styles.up,
                            isMine && moved < 0 && styles.down,
                            members.some((m) => took.has(m)) && styles.tookIt,
                          ]}
                        >
                          {s ? formatDelta(s) : '—'}
                        </Text>
                        {s ? <Text style={[styles.total, isMine && styles.mineTotal]}>{runningTotal(s)}</Text> : null}
                      </>
                    );
                    const cellStyle = [cs, isMine && styles.mineCol];
                    return s && onOpenScore ? (
                      <Pressable
                        key={id}
                        style={cellStyle}
                        onPress={() => onOpenScore(id, r.number)}
                        accessibilityRole="button"
                        accessibilityHint={t('score.open')}
                        testID={`round-${r.number}-${id}-open`}
                      >
                        {content}
                      </Pressable>
                    ) : (
                      <View key={id} style={cellStyle}>
                        {content}
                      </View>
                    );
                  })}
                </View>
              );
            })}
            </ScrollView>

            {standings?.length ? (
              <View style={[styles.row, styles.totalRow]}>
                <Text style={[cs, ls, styles.head]}>
                  {unit ? label(unit) : 'Total'}
                </Text>
                {columns.map(({ id }) => {
                  const s = byId.get(id);
                  return (
                    <View key={id} style={[cs, id === viewerId && styles.mineCol]}>
                      <Text testID={`round-total-${id}`} style={[styles.grand, id === viewerId && styles.mineGrand]}>
                        {s ? shownScore(s) : '—'}
                      </Text>
                    </View>
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

/** The viewer's side moved to the front; every other side keeps its place. */
export function viewerFirst(sides: string[][], viewerId: string): string[][] {
  const i = sides.findIndex((members) => members.includes(viewerId));
  return i <= 0 ? sides : [sides[i], ...sides.slice(0, i), ...sides.slice(i + 1)];
}

/**
 * How many of the latest rounds the table shows before the rest are asked
 * for: three on a phone, more where the window is taller.
 */
export function latestRows(windowHeight: number): number {
  if (windowHeight >= 1200) return 8;
  if (windowHeight >= 900) return 5;
  return 3;
}

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

function roundStyles(m: Metrics, s: Skin) {
  const colors = s.colors;
  return StyleSheet.create({
    scroller: { marginTop: 4 },
    // The table fills the panel when the panel is wider than its columns, and
    // scrolls sideways when it is narrower. Without this it was as wide as its
    // columns' fixed widths and left a third of a desktop panel empty.
    scrollerContent: { flexGrow: 1 },
    // Room under the last row for the scrollbar, which overlays content on
    // macOS and iOS and otherwise sits on the totals it scrolls.
    scrollerOverflow: { paddingBottom: 14 },
    table: { flexGrow: 1 },
    // The same tint the end-of-match banner uses, for the same reason: the
    // thing it has to beat is being mistaken for nothing having happened.
    paused: { backgroundColor: 'rgba(61, 139, 253, 0.10)', borderColor: colors.accent },
    newest: { backgroundColor: 'rgba(61, 139, 253, 0.07)', borderRadius: 4 },
    row: { flexDirection: 'row', alignItems: 'flex-start' },
    headRow: { borderBottomWidth: 1, borderBottomColor: colors.border, paddingBottom: 4 },
    totalRow: { borderTopWidth: 1, borderTopColor: colors.border, marginTop: 4, paddingTop: 6 },
    // Equal shares, not content-sized. A column that grows to its widest
    // content grows by a different amount in the head row (a name) than in the
    // rows below it (a number), and every column after the widest name is then
    // a few pixels out of line with its own heading — which is the one thing a
    // table has to get right. A zero basis makes every column's width depend
    // only on the room, never on what is in it, and the minimum keeps the old
    // width as the floor. Content too wide for the width truncates instead.
    cell: {
      flexGrow: 1,
      flexBasis: 0,
      minWidth: m.narrow ? 72 : 92,
      paddingVertical: 4,
      paddingHorizontal: 6,
      alignItems: 'flex-end',
    },
    // The left-hand column is wider: it carries the round's own number and
    // whatever the module said was true of it.
    labelCell: {
      flexGrow: 0,
      flexBasis: 'auto',
      minWidth: 0,
      width: m.narrow ? 92 : 132,
      alignItems: 'flex-start',
    },
    // The same table closed up, for a panel narrower than the one above
    // fits: columns at the width of a score rather than of a name. A name
    // there is cut short on one line ("Crafty Kl…"), because wrapped into a
    // column that narrow it broke mid-word ("Craft / y"). The label column
    // keeps room for one word of its heading ("penalty / points").
    cellCompact: { minWidth: 50, paddingVertical: 2, paddingHorizontal: 3 },
    labelCompact: { width: 76 },
    // Numbers read down the column, so they end on the same edge; names sit
    // over the numbers they belong to and end on it too.
    name: { alignSelf: 'stretch', textAlign: 'right' },
    head: { color: colors.muted, fontSize: m.panel.bodyFont, fontWeight: '600' },
    mine: { color: colors.text },
    youTag: { color: colors.accent, fontSize: Math.max(10, m.panel.bodyFont - 3), fontWeight: '600' },
    roundNumber: { color: colors.text, fontSize: m.panel.bodyFont, fontWeight: '600' },
    roundFact: { color: colors.muted, fontSize: Math.max(10, m.panel.bodyFont - 3) },
    delta: { color: colors.text, fontSize: m.panel.bodyFont },
    // Whoever took the round, marked on the cell rather than in a legend.
    tookIt: { color: colors.gold, fontWeight: '700' },
    total: { color: colors.muted, fontSize: Math.max(10, m.panel.bodyFont - 3) },
    grand: { color: colors.text, fontSize: m.panel.bodyFont, fontWeight: '700' },
    // The viewer's column: one tinted band from name to total, so "which of
    // these numbers are mine" is answered before anything is read.
    mineCol: { backgroundColor: 'rgba(212, 175, 55, 0.14)' },
    mineDelta: { fontWeight: '700' },
    mineTotal: { color: colors.text, fontWeight: '600' },
    mineGrand: { color: colors.gold },
    up: { color: colors.success },
    down: { color: colors.danger },
    summary: { color: colors.muted, fontSize: m.panel.bodyFont },
    // The same bordered badge a pile's show-all wears, so the two read as one
    // kind of control and neither as a second minimize chevron.
    toggle: {
      paddingHorizontal: 6,
      paddingVertical: 2,
      borderRadius: 6,
      borderWidth: 1,
      borderColor: colors.border,
    },
    toggleText: { color: colors.accentButton, fontSize: m.panel.bodyFont, fontWeight: '700' },
    summaryScore: { color: colors.gold, fontWeight: '700' },
  });
}
