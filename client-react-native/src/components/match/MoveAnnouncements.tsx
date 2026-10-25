import { useMemo, useRef, useState, useSyncExternalStore } from 'react';
import { useIsFocused } from 'expo-router';
import { Platform, Pressable, ScrollView, StyleSheet, Text, View, useWindowDimensions } from 'react-native';

import { announce } from '@/src/a11y/announce';
import { Tip } from '@/src/a11y/Tip';
import type { Fact, MatchPlayer, MoveLine } from '@/src/api/matchTypes';
import { LastCardInk } from '@/src/components/cards/LastCardInk';
import { HintsSheet } from '@/src/components/match/HintsSheet';
import { useSkin } from '@/src/hooks/useSkin';
import { t } from '@/src/lib/i18n';
import { factText, spokenFactText } from '@/src/lib/labels';
import type { Skin } from '@/src/skins/types';

/**
 * What happened since your last move, between the board and your hand.
 *
 * The board shows what changed, and the marks on it (`src/lib/changes.ts`)
 * show where, but neither says who, and in a game where moves chain — a card
 * that turns play round, one that takes a turn away, one that makes somebody
 * pick up, all between two of yours — the board shows only where the chain
 * ended. Folded, the box shows what happened since the viewer's own last
 * move, newest only; open, it lists every recent move the server sent,
 * oldest first, the newest marked, in a region that scrolls so the older
 * ones are reachable rather than clipped.
 *
 * The board's turn prompts — what the table is waiting on this viewer for —
 * close the box, below the scrolling moves so they are never pushed out.
 * Standing requirements (`hint`) are not here: they are a sheet of their
 * own, opened from the box's corner, listed most relevant first.
 *
 * It starts folded to the newest line, which is most of what a player needs
 * in the least of the table's room; the arrow opens the list, and the
 * choice holds for every table this session. The prompts show either way.
 *
 * The lines are the module's own words; this knows nothing of any game.
 */
export function MoveAnnouncements({
  moves,
  prompts,
  standing = [],
  players,
  viewerId,
}: {
  moves: MoveLine[];
  prompts: Fact[];
  /** What is true of the play as a whole — the colour or suit in play — kept in the box's right corner. */
  standing?: Fact[];
  players: MatchPlayer[];
  viewerId: string;
}) {
  const skin = useSkin();
  const open = useSyncExternalStore(subscribeMovesOpen, movesOpen, movesOpen);
  const setOpen = setMovesOpen;
  // Four lines' room where the window has it; two on a short one, so the
  // pile, the hand and its controls still fit together. Fixed by the window,
  // never by what is in the box — see below.
  const { height } = useWindowDimensions();
  const short = height < SHORT_WINDOW;
  const linesHeight = !open ? undefined : short ? LINE * 3 : LINE * 5;
  const [hintsOpen, setHintsOpen] = useState(false);
  // Screens stay mounted under the one that replaced them, and a Modal does
  // not hide with its screen: the sheet was left over every other screen.
  const focused = useIsFocused();
  const hints = prompts.filter((f) => f.hint);
  const turnPrompts = prompts.filter((f) => !f.hint);
  const scroller = useRef<ScrollView>(null);
  // The corner's real width, measured: its labels are translated and the
  // standing facts vary, so a guessed width let the first line run under it.
  const [cornerWidth, setCornerWidth] = useState(0);
  const styles = useMemo(() => announceStyles(skin, linesHeight, short, cornerWidth), [skin, linesHeight, short, cornerWidth]);

  // Everything after the viewer's own last move. When their move was the
  // last one, that move alone, dimmed — so the box says what the table is
  // looking at instead of going blank the moment they act.
  let lastMine = -1;
  moves.forEach((m, i) => {
    if (m.playerId === viewerId) lastMine = i;
  });
  const since = moves.slice(lastMine + 1).slice(-1);
  const shown = open ? moves : since.length > 0 ? since : moves.slice(-1);
  const settled = !open && since.length === 0;

  // A fixed height, always there. The box sits above the hand, so a box that
  // grew with each move would move the hand under a finger mid-drag; this
  // one keeps its size, the newest line at the bottom and the oldest falling
  // off the top. The prompt — what the table is waiting on this player for —
  // goes last, where nothing pushes it out of sight.
  // The newest line, said again on request — the screen reader heard it once,
  // as it happened, possibly over something else.
  const last = moves[moves.length - 1];
  const repeat = () => announce(last ? spokenFactText(last.fact, players) : t('a11y.board.moves.none'), 'assertive');

  // Not a live region of its own any more: every move is already announced,
  // once and in words, by the match screen through `announceGame` — which
  // also honours the player's choice of how much to hear. A second, polite
  // copy here read each move twice and ignored that choice.
  //
  // It is the board's "recent moves" region instead, a list a screen reader
  // can walk back through, focusable from the keyboard (M) so that walking
  // can start from a key.
  const moveRows = shown.map((m, i) => {
          const newest = !settled && i === shown.length - 1;
          return (
            <View
              key={i}
              style={[styles.row, newest && styles.newestRow, open && i === 0 && styles.firstOpenRow]}
              // Each line said with its cards in words; the visible line keeps
              // the glyphs a sighted reader takes in faster.
              {...((Platform.OS === 'web'
                ? { role: 'listitem', 'aria-label': spokenFactText(m.fact, players) }
                : { accessible: true, accessibilityLabel: spokenFactText(m.fact, players) }) as object)}
            >
              <Text
                testID={`announce-${i}`}
                // Folded, a line may take a second row rather than end in an
                // ellipsis: the corner buttons leave it about two thirds of the
                // width, and a name plus what it did is longer than that.
                numberOfLines={open ? undefined : 2}
                style={[styles.line, newest && styles.newest, settled && styles.settled, !open && styles.foldedLine]}
              >
                <LastCardInk text={factText(m.fact, players)} />
              </Text>
            </View>
          );
        });

  // Nothing to say yet — a fresh deal, before anyone has moved, with no
  // prompt, requirement or standing fact — is no box at all. It was a bordered
  // rectangle holding only its two corner buttons.
  if (moves.length === 0 && turnPrompts.length === 0 && hints.length === 0 && standing.length === 0) {
    return null;
  }

  return (
    <View
      style={styles.box}
      testID="move-announcements"
      {...((Platform.OS === 'web'
        ? { role: 'region', 'aria-label': t('a11y.board.region.moves'), tabIndex: -1 }
        : {
            accessibilityActions: [{ name: 'repeat', label: t('a11y.board.moves.repeat') }],
            onAccessibilityAction: (e: { nativeEvent: { actionName: string } }) => {
              if (e.nativeEvent.actionName === 'repeat') repeat();
            },
          }) as object)}
    >
      {/* Repeat-last, over the box's corner rather than in its flow: the box
          keeps one fixed height whatever is in it (see below). */}
      <View
        style={styles.repeatAt}
        onLayout={(e) => {
          const w = Math.ceil(e.nativeEvent.layout.width);
          setCornerWidth((prev) => (prev === w ? prev : w));
        }}
      >
        {hints.length > 0 ? (
          <Pressable
            testID="hints-open"
            accessibilityRole="button"
            accessibilityLabel={t('hints.title')}
            onPress={() => setHintsOpen(true)}
            hitSlop={8}
            style={styles.hintsButton}
          >
            <Text style={styles.hintsText}>
              {t('hints.title')} · {hints.length}
            </Text>
          </Pressable>
        ) : null}
        <Tip text={t('a11y.board.tip.repeat')}>
          <Pressable
            testID="moves-repeat"
            accessibilityRole="button"
            accessibilityLabel={t('a11y.board.moves.repeat')}
            onPress={repeat}
            hitSlop={8}
            style={styles.repeat}
          >
            <Text style={styles.repeatText}>↻</Text>
          </Pressable>
        </Tip>
        <Pressable
          testID="moves-toggle"
          accessibilityRole="button"
          accessibilityLabel={t('moves.title')}
          accessibilityState={{ expanded: open }}
          // react-native-web does not carry accessibilityState over to the
          // page, so a screen reader in a browser heard no open or closed.
          {...((Platform.OS === 'web' ? { 'aria-expanded': open } : {}) as object)}
          onPress={() => setOpen(!open)}
          hitSlop={8}
          style={styles.repeat}
        >
          <Text style={styles.toggleText}>{open ? '▴' : '▾'}</Text>
        </Pressable>
      </View>
      {/* On a short window the lines speak for themselves; the room goes to
          the hand. Folded, the one line does. */}
      {short || !open ? null : (
        <Text style={styles.title} {...((Platform.OS === 'web' ? {} : { accessibilityRole: 'header' }) as object)}>
          {t('moves.title')}
        </Text>
      )}
      <View style={styles.lines}>
        {/* The moves are a list of their own — the prompts after them are
            not moves, and a list may hold nothing but its items. Spaced the
            way the lines are, so the box looks exactly as it did. */}
        {open ? (
          <ScrollView
            ref={scroller}
            testID="moves-scroll"
            style={styles.scroll}
            contentContainerStyle={styles.list}
            onContentSizeChange={() => scroller.current?.scrollToEnd({ animated: false })}
            {...((Platform.OS === 'web' ? { role: 'list' } : {}) as object)}
          >
            {moveRows}
          </ScrollView>
        ) : (
        <View style={styles.list} {...((Platform.OS === 'web' ? { role: 'list' } : {}) as object)}>
        {moveRows}
        </View>
        )}
        {turnPrompts.map((f, i) => (
          <Text
            key={`prompt-${i}`}
            testID={`prompt-${i}`}
            // With no move above it, the first prompt is the box's first line
            // and shares the corner with the buttons: a long one ran under them.
            style={[styles.prompt, i === 0 && shown.length === 0 && styles.firstOpenRow]}
          >
            <LastCardInk text={factText(f, players)} />
          </Text>
        ))}
      </View>
      {/* Under the lines, not in the corner: set beside the buttons it took
          a third of the row, and the line above it — a player's name and what
          they did — was cut short to make room for it. */}
      {standing.length > 0 ? (
        <View style={styles.standingRow}>
          {standing.map((f, i) => (
            <Text key={`standing-${i}`} testID={`standing-${i}`} style={styles.standing}>
              <LastCardInk text={factText(f, players)} />
            </Text>
          ))}
        </View>
      ) : null}
      {hintsOpen && focused ? <HintsSheet hints={hints} players={players} onClose={() => setHintsOpen(false)} /> : null}
    </View>
  );
}

/** Whether the list is open, for every table this session: it starts folded. */
let openPreference = false;
const openListeners = new Set<() => void>();

function subscribeMovesOpen(listener: () => void) {
  openListeners.add(listener);
  return () => {
    openListeners.delete(listener);
  };
}

/** Whether the list of recent moves is open, rather than just the last one. */
export function movesOpen(): boolean {
  return openPreference;
}

/** Whether the list of recent moves is open, kept current. */
export function useMovesOpen(): boolean {
  return useSyncExternalStore(subscribeMovesOpen, movesOpen, movesOpen);
}

/**
 * Opens or closes the list of recent moves, wherever it is drawn. Exported for
 * the Mac app's View › Show Log, which reaches it from outside the screen.
 */
export function setMovesOpen(v: boolean) {
  openPreference = v;
  for (const l of openListeners) l();
}

/** One line's room in the box. */
const LINE = 24;

/** Below this window height the box keeps two lines' room rather than four. */
const SHORT_WINDOW = 760;

function announceStyles(s: Skin, linesHeight: number | undefined, short: boolean, cornerWidth: number) {
  const colors = s.colors;
  return StyleSheet.create({
    box: {
      borderWidth: 1,
      borderColor: colors.accent,
      backgroundColor: colors.surface,
      borderRadius: 10,
      paddingHorizontal: 12,
      paddingVertical: short ? 4 : 8,
      marginTop: short ? 6 : 10,
      gap: 3,
    },
    prompt: { color: colors.gold, fontSize: 16, fontWeight: '700', marginTop: 2 },
    // Room for about four lines; see the render for why it never grows.
    // Folded, it is one line and whatever the prompts take.
    lines: { height: linesHeight, minHeight: LINE, overflow: 'hidden', justifyContent: 'flex-end', gap: 3 },
    title: {
      color: colors.muted,
      fontSize: 11,
      letterSpacing: 1,
      textTransform: 'uppercase',
      fontWeight: '600',
    },
    list: { gap: 3 },
    scroll: { flexShrink: 1 },
    hintsButton: { minHeight: 24, justifyContent: 'center', paddingHorizontal: 6, borderRadius: 6, borderWidth: 1, borderColor: colors.gold },
    hintsText: { color: colors.gold, fontSize: 12, fontWeight: '700' },
    row: { borderLeftWidth: 3, borderLeftColor: 'transparent', paddingLeft: 8 },
    // In the box's top-right corner, out of the flow — see the render.
    repeatAt: { position: 'absolute', top: short ? 2 : 6, right: 8, zIndex: 1, flexDirection: 'row', alignItems: 'center', gap: 6 },
    standingRow: { flexDirection: 'row', flexWrap: 'wrap', columnGap: 12, paddingLeft: 11 },
    standing: { color: colors.text, fontSize: 14, fontWeight: '600' },
    repeat: { minWidth: 24, minHeight: 24, alignItems: 'center', justifyContent: 'center' },
    repeatText: { color: colors.muted, fontSize: 14, fontWeight: '700' },
    toggleText: { color: colors.muted, fontSize: 16, fontWeight: '700' },
    // Clear of the two buttons in the corner.
    foldedLine: { paddingRight: cornerWidth + 8 },
    // Open, only the first row shares the corner with the buttons.
    firstOpenRow: { paddingRight: cornerWidth + 8 },
    newestRow: { borderLeftColor: colors.accent },
    line: { color: colors.text, fontSize: 15 },
    newest: { fontSize: 17, fontWeight: '700' },
    settled: { color: colors.muted },
  });
}
