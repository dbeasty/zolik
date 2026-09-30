import { useMemo, useState } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import type { MatchPlayer, MoveLine } from '@/src/api/matchTypes';
import { useSkin } from '@/src/hooks/useSkin';
import { t } from '@/src/lib/i18n';
import { factText } from '@/src/lib/labels';
import type { Skin } from '@/src/skins/types';

/**
 * Who did what, in one line over the table.
 *
 * The board shows what changed, and the marks on it (`src/lib/changes.ts`)
 * show where, but neither says who. When one player adds a card to another
 * player's meld, the board looks the same as when the owner does it. The
 * server words each move and sends the last few; this shows the newest move
 * by somebody else, and a press opens the rest.
 *
 * The viewer's own moves are left out of the collapsed line, since they know
 * what they did. They stay in the open list, so the sequence reads in order.
 */
export function RecentMoves({
  moves,
  players,
  viewerId,
  inHeader,
}: {
  moves: MoveLine[];
  players: MatchPlayer[];
  viewerId: string;
  /** Drawn in the Table panel's header, beside its title, rather than on a line of its own. */
  inHeader?: boolean;
}) {
  const skin = useSkin();
  const styles = useMemo(() => recentStyles(skin), [skin]);
  const [open, setOpen] = useState(false);

  const latest = [...moves].reverse().find((m) => m.playerId !== viewerId);
  if (!latest) return null;

  return (
    <Pressable
      onPress={() => setOpen((was) => !was)}
      accessibilityRole="button"
      accessibilityState={{ expanded: open }}
      accessibilityLabel={t('moves.title')}
      style={[styles.strip, inHeader && styles.inHeader]}
      testID="recent-moves"
    >
      {open ? (
        <View style={styles.list} testID="recent-moves-list">
          <Text style={styles.title}>{t('moves.title')}</Text>
          {moves.map((m, i) => (
            <Text
              key={i}
              style={[styles.line, m.playerId === viewerId && styles.mine]}
              testID={`recent-move-${i}`}
            >
              {factText(m.fact, players)}
            </Text>
          ))}
        </View>
      ) : (
        <Text style={styles.latest} numberOfLines={1} testID="recent-moves-latest">
          {factText(latest.fact, players)} <Text style={styles.more}>›</Text>
        </Text>
      )}
    </Pressable>
  );
}

function recentStyles(s: Skin) {
  const colors = s.colors;
  return StyleSheet.create({
    strip: {
      borderWidth: 1,
      borderColor: colors.border,
      backgroundColor: colors.surface,
      borderRadius: 8,
      paddingHorizontal: 10,
      paddingVertical: 6,
      marginBottom: 8,
    },
    inHeader: { marginBottom: 0 },
    latest: { color: colors.text, fontSize: 16 },
    more: { color: colors.accent, fontWeight: '700' },
    list: { gap: 3 },
    title: {
      color: colors.muted,
      fontSize: 11,
      letterSpacing: 1,
      textTransform: 'uppercase',
      fontWeight: '600',
      marginBottom: 2,
    },
    line: { color: colors.text, fontSize: 15 },
    mine: { color: colors.muted },
  });
}
