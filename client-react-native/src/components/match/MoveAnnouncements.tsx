import { useMemo, useState } from 'react';
import { Platform, Pressable, StyleSheet, Text, View, useWindowDimensions } from 'react-native';

import { announce } from '@/src/a11y/announce';
import { Tip } from '@/src/a11y/Tip';
import type { Fact, MatchPlayer, MoveLine } from '@/src/api/matchTypes';
import { LastCardInk } from '@/src/components/cards/LastCardInk';
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
 * ended. So every move since the viewer's own last one is listed, oldest
 * first, the newest marked, set where the eye goes on its way to the hand.
 * It used to be one line over the table, which is where nobody looked.
 *
 * The board's prompts — what the table is waiting on this viewer for — lead
 * the box, since they are the reason to read it.
 *
 * It starts folded to the newest line, which is most of what a player needs
 * in the least of the table's room; the arrow opens the whole list, and the
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
  const [open, setOpenState] = useState(openPreference);
  const setOpen = (v: boolean) => {
    openPreference = v;
    setOpenState(v);
  };
  // Four lines' room where the window has it; two on a short one, so the
  // pile, the hand and its controls still fit together. Fixed by the window,
  // never by what is in the box — see below.
  const { height } = useWindowDimensions();
  const short = height < SHORT_WINDOW;
  const linesHeight = !open ? undefined : short ? LINE * 2 : LINE * 4;
  const styles = useMemo(() => announceStyles(skin, linesHeight, short, standing.length > 0), [skin, linesHeight, short, standing.length]);

  // Everything after the viewer's own last move. When their move was the
  // last one, that move alone, dimmed — so the box says what the table is
  // looking at instead of going blank the moment they act.
  let lastMine = -1;
  moves.forEach((m, i) => {
    if (m.playerId === viewerId) lastMine = i;
  });
  const since = moves.slice(lastMine + 1).slice(open ? -MAX_LINES : -1);
  const shown = since.length > 0 ? since : moves.slice(-1);
  const settled = since.length === 0;

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
      <View style={styles.repeatAt}>
        {standing.map((f, i) => (
          <Text key={`standing-${i}`} testID={`standing-${i}`} style={styles.standing} numberOfLines={1}>
            <LastCardInk text={factText(f, players)} />
          </Text>
        ))}
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
        <View style={styles.list} {...((Platform.OS === 'web' ? { role: 'list' } : {}) as object)}>
        {shown.map((m, i) => {
          const newest = !settled && i === shown.length - 1;
          return (
            <View
              key={i}
              style={[styles.row, newest && styles.newestRow]}
              // Each line said with its cards in words; the visible line keeps
              // the glyphs a sighted reader takes in faster.
              {...((Platform.OS === 'web'
                ? { role: 'listitem', 'aria-label': spokenFactText(m.fact, players) }
                : { accessible: true, accessibilityLabel: spokenFactText(m.fact, players) }) as object)}
            >
              <Text
                testID={`announce-${i}`}
                numberOfLines={open ? undefined : 1}
                style={[styles.line, newest && styles.newest, settled && styles.settled, !open && styles.foldedLine]}
              >
                <LastCardInk text={factText(m.fact, players)} />
              </Text>
            </View>
          );
        })}
        </View>
        {prompts.map((f, i) => (
          <Text key={`prompt-${i}`} testID={`prompt-${i}`} style={styles.prompt}>
            <LastCardInk text={factText(f, players)} />
          </Text>
        ))}
      </View>
    </View>
  );
}

/** Whether the list is open, for every table this session: it starts folded. */
let openPreference = false;

/** About a round at a full table; older moves are in the strip's own history. */
const MAX_LINES = 8;

/** One line's room in the box. */
const LINE = 24;

/** Room a standing fact takes in the corner, so a folded line stops short of it. */
const STANDING_ROOM = 150;

/** Below this window height the box keeps two lines' room rather than four. */
const SHORT_WINDOW = 760;

function announceStyles(s: Skin, linesHeight: number | undefined, short: boolean, standing: boolean) {
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
    row: { borderLeftWidth: 3, borderLeftColor: 'transparent', paddingLeft: 8 },
    // In the box's top-right corner, out of the flow — see the render.
    repeatAt: { position: 'absolute', top: short ? 2 : 6, right: 8, zIndex: 1, flexDirection: 'row', alignItems: 'center', gap: 6 },
    standing: { color: colors.text, fontSize: 14, fontWeight: '600', marginRight: 6 },
    repeat: { minWidth: 24, alignItems: 'center' },
    repeatText: { color: colors.muted, fontSize: 14, fontWeight: '700' },
    toggleText: { color: colors.muted, fontSize: 16, fontWeight: '700' },
    // Clear of the two buttons in the corner.
    foldedLine: { paddingRight: standing ? 64 + STANDING_ROOM : 64 },
    newestRow: { borderLeftColor: colors.accent },
    line: { color: colors.text, fontSize: 15 },
    newest: { fontSize: 17, fontWeight: '700' },
    settled: { color: colors.muted },
  });
}
