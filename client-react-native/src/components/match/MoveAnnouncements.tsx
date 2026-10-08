import { useMemo } from 'react';
import { StyleSheet, Text, View, useWindowDimensions } from 'react-native';

import type { Fact, MatchPlayer, MoveLine } from '@/src/api/matchTypes';
import { useSkin } from '@/src/hooks/useSkin';
import { t } from '@/src/lib/i18n';
import { factText } from '@/src/lib/labels';
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
 * The lines are the module's own words; this knows nothing of any game.
 */
export function MoveAnnouncements({
  moves,
  prompts,
  players,
  viewerId,
}: {
  moves: MoveLine[];
  prompts: Fact[];
  players: MatchPlayer[];
  viewerId: string;
}) {
  const skin = useSkin();
  // Four lines' room where the window has it; two on a short one, so the
  // pile, the hand and its controls still fit together. Fixed by the window,
  // never by what is in the box — see below.
  const { height } = useWindowDimensions();
  const short = height < SHORT_WINDOW;
  const linesHeight = short ? LINE * 2 : LINE * 4;
  const styles = useMemo(() => announceStyles(skin, linesHeight, short), [skin, linesHeight, short]);

  // Everything after the viewer's own last move. When their move was the
  // last one, that move alone, dimmed — so the box says what the table is
  // looking at instead of going blank the moment they act.
  let lastMine = -1;
  moves.forEach((m, i) => {
    if (m.playerId === viewerId) lastMine = i;
  });
  const since = moves.slice(lastMine + 1).slice(-MAX_LINES);
  const shown = since.length > 0 ? since : moves.slice(-1);
  const settled = since.length === 0;

  // A fixed height, always there. The box sits above the hand, so a box that
  // grew with each move would move the hand under a finger mid-drag; this
  // one keeps its size, the newest line at the bottom and the oldest falling
  // off the top. The prompt — what the table is waiting on this player for —
  // goes last, where nothing pushes it out of sight.
  return (
    <View style={styles.box} testID="move-announcements" accessibilityLiveRegion="polite">
      {/* On a short window the lines speak for themselves; the room goes to the hand. */}
      {short ? null : <Text style={styles.title}>{t('moves.title')}</Text>}
      <View style={styles.lines}>
        {shown.map((m, i) => {
          const newest = !settled && i === shown.length - 1;
          return (
            <View key={i} style={[styles.row, newest && styles.newestRow]}>
              <Text testID={`announce-${i}`} style={[styles.line, newest && styles.newest, settled && styles.settled]}>
                {factText(m.fact, players)}
              </Text>
            </View>
          );
        })}
        {prompts.map((f, i) => (
          <Text key={`prompt-${i}`} testID={`prompt-${i}`} style={styles.prompt}>
            {factText(f, players)}
          </Text>
        ))}
      </View>
    </View>
  );
}

/** About a round at a full table; older moves are in the strip's own history. */
const MAX_LINES = 8;

/** One line's room in the box. */
const LINE = 24;

/** Below this window height the box keeps two lines' room rather than four. */
const SHORT_WINDOW = 760;

function announceStyles(s: Skin, linesHeight: number, short: boolean) {
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
    lines: { height: linesHeight, overflow: 'hidden', justifyContent: 'flex-end', gap: 3 },
    title: {
      color: colors.muted,
      fontSize: 11,
      letterSpacing: 1,
      textTransform: 'uppercase',
      fontWeight: '600',
    },
    row: { borderLeftWidth: 3, borderLeftColor: 'transparent', paddingLeft: 8 },
    newestRow: { borderLeftColor: colors.accent },
    line: { color: colors.text, fontSize: 15 },
    newest: { fontSize: 17, fontWeight: '700' },
    settled: { color: colors.muted },
  });
}
