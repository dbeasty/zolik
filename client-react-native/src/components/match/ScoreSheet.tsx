import { useMemo, useState } from 'react';
import { Modal, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';

import type { Fact, MatchPlayer, RoundLog, RoundResult, RoundScore, ScoreLine, Seat, Standing } from '@/src/api/matchTypes';
import { useMetrics } from '@/src/hooks/useMetrics';
import { useSkin } from '@/src/hooks/useSkin';
import { t } from '@/src/lib/i18n';
import { factText, label, playerName, shownScore } from '@/src/lib/labels';
import type { Metrics } from '@/src/lib/layout';
import { partnersOf } from '@/src/lib/sides';
import type { Skin } from '@/src/skins/types';

/**
 * Where a score came from: every round, as an account that adds up.
 *
 * A Canasta score jumps by a thousand at the end of a deal and the table said
 * nothing about why — the melds that earned it are swept off by the next deal.
 * The server kept the account the whole time; this is where it is read.
 *
 * Like `RoundResults`, it knows no game. Each round's `lines` are key, params
 * and points, pre-computed and pre-balanced on the server (`CheckLines` fails
 * a module whose account does not sum to its delta), so this component prints
 * and indents and never adds anything up. A round without lines — an older
 * match, or a game that has not written its account — shows its facts instead.
 *
 * Its own surface rather than a section of the board: opened from a score,
 * over the table, and closed back to it. Full height on a phone, where there is
 * no room beside the board; a side panel on a wide screen, where the board it
 * explains stays in view.
 */
type Props = {
  /** Whose score was tapped. Null is closed. */
  subjectId: string | null;
  log?: RoundLog;
  seats: Seat[];
  players: MatchPlayer[];
  standings?: Standing[];
  viewerId: string;
  /** A round to open at, when the sheet was opened from that round's cell. */
  focusRound?: number;
  onClose: () => void;
};

export function ScoreSheet(props: Props) {
  if (!props.subjectId) return null;
  // Keyed on what was tapped, so a second tap on another score starts from
  // that side and that round rather than wherever the last one was left.
  return <ScoreSheetBody key={`${props.subjectId}:${props.focusRound ?? ''}`} {...props} subjectId={props.subjectId} />;
}

function ScoreSheetBody({
  subjectId,
  log,
  seats,
  players,
  standings,
  viewerId,
  focusRound,
  onClose,
}: Props & { subjectId: string }) {
  const metrics = useMetrics();
  const skin = useSkin();
  const styles = useMemo(() => sheetStyles(metrics, skin), [metrics, skin]);

  const sides = useMemo(() => sidesOf(seats, standings, players), [seats, standings, players]);
  const [sideIndex, setSideIndex] = useState(() =>
    Math.max(0, sides.findIndex((s) => s.includes(subjectId))),
  );
  const side = sides[sideIndex] ?? [subjectId];
  // A partnership's row is every member's row, so any member reads it.
  const who = side[0];

  const rounds = useMemo(() => [...(log?.rounds ?? [])].reverse(), [log]);
  const latest = rounds[0]?.number;
  const [open, setOpen] = useState<ReadonlySet<number>>(
    () => new Set(focusRound !== undefined ? [focusRound] : latest !== undefined ? [latest] : []),
  );
  const toggle = (n: number) =>
    setOpen((prev) => {
      const next = new Set(prev);
      if (next.has(n)) next.delete(n);
      else next.add(n);
      return next;
    });

  const standing = standings?.find((s) => s.playerId === who);
  const sideName = (ids: string[]) =>
    ids
      .map((id) => playerName(players, id) + (id === viewerId ? ` ${t('match.youSuffix')}` : ''))
      .join(' & ');

  return (
    <Modal transparent animationType="fade" visible onRequestClose={onClose}>
      <Pressable
        style={[styles.backdrop, !metrics.narrow && styles.backdropWide]}
        onPress={onClose}
        testID="score-sheet-backdrop"
      >
        {/* Stops a press inside the sheet from closing it. */}
        <Pressable style={[styles.sheet, !metrics.narrow && styles.sheetWide]} onPress={() => {}} testID="score-sheet">
          <View style={styles.header}>
            <Text style={styles.key}>{t('score.title')}</Text>
            <View style={styles.headerRow}>
              <Text style={styles.sideName} numberOfLines={2} testID="score-sheet-side">
                {sideName(side)}
              </Text>
              {standing ? (
                <Text style={styles.headerTotal} testID="score-sheet-total">
                  {shownScore(standing)} {label(standing.labelKey)}
                </Text>
              ) : null}
            </View>
            {sides.length > 1 ? (
              <View style={styles.tabs}>
                {sides.map((s, i) => (
                  <Pressable
                    key={s.join(',')}
                    onPress={() => setSideIndex(i)}
                    style={[styles.tab, i === sideIndex && styles.tabOn]}
                    accessibilityRole="tab"
                    accessibilityState={{ selected: i === sideIndex }}
                    testID={`score-sheet-tab-${s[0]}`}
                  >
                    <Text style={[styles.tabText, i === sideIndex && styles.tabTextOn]} numberOfLines={1}>
                      {sideName(s)}
                    </Text>
                  </Pressable>
                ))}
              </View>
            ) : null}
          </View>

          <ScrollView contentContainerStyle={styles.body}>
            {rounds.length === 0 ? (
              <Text style={styles.empty} testID="score-sheet-empty">
                {t('score.empty')}
              </Text>
            ) : null}
            {rounds.map((r) => {
              const rs = r.scores.find((s) => s.playerId === who);
              if (!rs) return null;
              return (
                <RoundCard
                  key={r.number}
                  round={r}
                  score={rs}
                  roundLabel={label(log?.labelKey)}
                  players={players}
                  open={open.has(r.number)}
                  onToggle={() => toggle(r.number)}
                  styles={styles}
                />
              );
            })}
          </ScrollView>

          <Pressable style={styles.close} onPress={onClose} testID="score-sheet-close">
            <Text style={styles.closeText}>{t('score.close')}</Text>
          </Pressable>
        </Pressable>
      </Pressable>
    </Modal>
  );
}

function RoundCard({
  round,
  score,
  roundLabel,
  players,
  open,
  onToggle,
  styles,
}: {
  round: RoundResult;
  score: RoundScore;
  roundLabel: string;
  players: MatchPlayer[];
  open: boolean;
  onToggle: () => void;
  styles: SheetStyles;
}) {
  const delta = score.shown ?? score.delta;
  const facts: Fact[] = [...(round.headline ? [round.headline] : []), ...(round.facts ?? [])];
  return (
    <View style={styles.card} testID={`score-round-${round.number}`}>
      <Pressable
        onPress={onToggle}
        style={styles.cardHead}
        accessibilityRole="button"
        accessibilityState={{ expanded: open }}
        testID={`score-round-${round.number}-toggle`}
      >
        <Text style={styles.cardTitle}>
          {open ? '▾' : '▸'} {roundLabel} {round.number}
        </Text>
        <Text style={styles.cardDelta} testID={`score-round-${round.number}-delta`}>
          {signed(delta)}
          <Text style={styles.cardRunning}> → {score.shownTotal ?? score.total}</Text>
        </Text>
      </Pressable>
      {facts.map((f, i) => (
        <Text key={`${f.labelKey}-${i}`} style={styles.roundFact}>
          {factText(f, players)}
        </Text>
      ))}
      {open ? (
        <View style={styles.lines} testID={`score-round-${round.number}-lines`}>
          {score.lines?.length ? (
            <>
              {score.lines.map((l, i) => (
                <LineRow key={`${l.labelKey}-${i}`} line={l} depth={0} players={players} styles={styles} />
              ))}
              <View style={[styles.lineRow, styles.totalRow]}>
                <Text style={[styles.lineLabel, styles.totalLabel]}>{t('score.total')}</Text>
                <Text style={[styles.linePoints, styles.totalLabel]}>{signed(delta)}</Text>
              </View>
            </>
          ) : (
            (score.facts ?? []).map((f, i) => (
              <Text key={`${f.labelKey}-${i}`} style={styles.lineLabel}>
                {factText(f, players)}
              </Text>
            ))
          )}
        </View>
      ) : null}
    </View>
  );
}

function LineRow({
  line,
  depth,
  players,
  styles,
}: {
  line: ScoreLine;
  depth: number;
  players: MatchPlayer[];
  styles: SheetStyles;
}) {
  // A line with nothing to add explains rather than scores: it gets no figure,
  // because a "0" beside "two more cards to a canasta" reads as a penalty.
  const scores = line.points !== 0 || !!line.sub?.length;
  return (
    <>
      <View style={[styles.lineRow, { paddingLeft: depth * 14 }]} testID="score-line">
        <Text style={[styles.lineLabel, depth > 0 && styles.subLabel]}>
          {factText({ labelKey: line.labelKey, params: line.params } as Fact, players)}
        </Text>
        {scores ? (
          <Text
            style={[styles.linePoints, depth > 0 && styles.subLabel, line.points < 0 && styles.negative]}
            testID={depth === 0 ? 'score-line-points' : undefined}
          >
            {signed(line.points)}
          </Text>
        ) : null}
      </View>
      {(line.sub ?? []).map((s, i) => (
        <LineRow key={`${s.labelKey}-${i}`} line={s} depth={depth + 1} players={players} styles={styles} />
      ))}
    </>
  );
}

/**
 * The table's sides, best first: a partnership is one side however many seats
 * it has, and a game without partnerships has a side per seat.
 */
export function sidesOf(seats: Seat[], standings: Standing[] | undefined, players: MatchPlayer[]): string[][] {
  const order = standings?.length
    ? standings.map((s) => s.playerId)
    : seats.length
      ? seats.map((s) => s.playerId)
      : players.map((p) => p.id);
  const seen = new Set<string>();
  const out: string[][] = [];
  for (const id of order) {
    if (seen.has(id)) continue;
    const side = [id, ...partnersOf(seats, id)];
    side.forEach((m) => seen.add(m));
    out.push(side);
  }
  return out;
}

function signed(n: number): string {
  return n > 0 ? `+${n}` : String(n);
}

type SheetStyles = ReturnType<typeof sheetStyles>;

function sheetStyles(metrics: Metrics, s: Skin) {
  const colors = s.colors;
  // The panels' own type scale, so the sheet reads at the size of the board
  // it sits beside rather than a size of its own.
  const body = metrics.panel.bodyFont + 1;
  const small = Math.max(10, metrics.panel.bodyFont - 1);
  const heading = metrics.panel.bodyFont + 4;
  return StyleSheet.create({
    backdrop: {
      flex: 1,
      backgroundColor: 'rgba(0,0,0,0.6)',
      justifyContent: 'flex-end',
    },
    // Beside the board rather than over it, so the table the numbers are
    // about stays in view.
    backdropWide: { flexDirection: 'row', justifyContent: 'flex-end' },
    sheet: {
      backgroundColor: colors.surface,
      borderTopWidth: 1,
      borderLeftWidth: 1,
      borderRightWidth: 1,
      borderColor: colors.border,
      borderTopLeftRadius: 14,
      borderTopRightRadius: 14,
      paddingHorizontal: 16,
      paddingTop: 16,
      paddingBottom: 20,
      height: '92%',
      gap: 12,
    },
    sheetWide: {
      height: '100%',
      width: 440,
      maxWidth: '100%',
      borderTopRightRadius: 0,
      borderRightWidth: 0,
      borderBottomLeftRadius: 14,
    },
    header: { gap: 6 },
    headerRow: { flexDirection: 'row', alignItems: 'baseline', justifyContent: 'space-between', gap: 12 },
    key: {
      color: colors.muted,
      fontSize: Math.max(10, metrics.panel.bodyFont - 2),
      letterSpacing: 1,
      textTransform: 'uppercase',
      fontWeight: '600',
    },
    sideName: { flexShrink: 1, color: colors.text, fontSize: heading, fontWeight: '700' },
    headerTotal: { color: colors.gold, fontSize: heading, fontWeight: '700' },
    tabs: { flexDirection: 'row', flexWrap: 'wrap', gap: 6 },
    tab: {
      borderWidth: 1,
      borderColor: colors.border,
      borderRadius: 14,
      paddingHorizontal: 10,
      paddingVertical: 4,
      maxWidth: '100%',
    },
    tabOn: { borderColor: colors.accent, backgroundColor: 'rgba(61, 139, 253, 0.12)' },
    tabText: { color: colors.muted, fontSize: small },
    tabTextOn: { color: colors.text, fontWeight: '600' },
    body: { gap: 10, paddingBottom: 8 },
    empty: { color: colors.muted, fontSize: body },
    card: {
      borderWidth: 1,
      borderColor: colors.border,
      borderRadius: 10,
      paddingHorizontal: 12,
      paddingVertical: 10,
      gap: 4,
    },
    cardHead: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'baseline', gap: 8 },
    cardTitle: { color: colors.text, fontSize: body, fontWeight: '700' },
    cardDelta: { color: colors.text, fontSize: body, fontWeight: '700' },
    cardRunning: { color: colors.muted, fontWeight: '400' },
    roundFact: { color: colors.muted, fontSize: small },
    lines: { marginTop: 6, gap: 3 },
    lineRow: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'baseline', gap: 12 },
    lineLabel: { flexShrink: 1, color: colors.text, fontSize: body, lineHeight: Math.round(body * 1.4) },
    subLabel: { color: colors.muted, fontSize: small, lineHeight: Math.round(small * 1.4) },
    linePoints: { color: colors.text, fontSize: body, fontVariant: ['tabular-nums'] },
    negative: { color: colors.danger },
    totalRow: { borderTopWidth: 1, borderTopColor: colors.border, marginTop: 4, paddingTop: 6 },
    totalLabel: { fontWeight: '700' },
    close: {
      borderWidth: 1,
      borderColor: colors.border,
      borderRadius: 8,
      paddingVertical: 11,
      alignItems: 'center',
    },
    closeText: { color: colors.muted, fontWeight: '600', fontSize: body },
  });
}
