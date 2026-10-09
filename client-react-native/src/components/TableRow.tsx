import { router } from 'expo-router';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import type { StoredTable } from '@/src/api/matchTypes';
import { useOpenGames } from '@/src/desktop/openGames';
import { t } from '@/src/lib/i18n';
import { routeForMatch, routeForReplay } from '@/src/lib/matchRoute';
import { colors } from '@/src/theme';

/**
 * One of the player's tables on a wide window's side panel: the game (when
 * the panel spans several), who it is with, how it stands, and the way back
 * in — Resume for a table still going, Replay for one that is over.
 *
 * Reuses the account menu's wording (`mine.status.*`) so a table is called the
 * same thing wherever it is listed.
 */
export function TableRow({
  row,
  gameLabel,
  selfId,
}: {
  row: StoredTable;
  /** The game's name, when the list spans several games and each row must say which. */
  gameLabel?: string;
  selfId?: string;
}) {
  const over = row.status === 'completed';
  const openGames = useOpenGames();
  const playing = !over && openGames.has(row.matchId);
  const others = row.players.filter((p) => p.id !== selfId);
  const people = others.filter((p) => !p.isAI).map((p) => p.name);
  const withWho = people.join(', ');

  return (
    <View style={styles.row} testID={`table-row-${row.matchId}`}>
      <View style={{ flex: 1, minWidth: 0 }}>
        <Text style={styles.name} numberOfLines={2}>
          {gameLabel ?? (withWho || t('lobby.games.bots'))}
        </Text>
        {gameLabel ? (
          <Text style={styles.meta} numberOfLines={2}>
            {withWho || t('lobby.games.bots')}
          </Text>
        ) : null}
      </View>
      {row.yourTurn ? (
        <Text style={[styles.badge, styles.badgeTurn]}>{t('picker.yourTurn')}</Text>
      ) : (
        <Text style={[styles.badge, styles.badgeQuiet]}>
          {playing ? t('desktop.game.playing') : t(`mine.status.${row.status}`, undefined, row.status)}
        </Text>
      )}
      <Pressable
        testID={`table-row-go-${row.matchId}`}
        accessibilityRole="button"
        // "Resume" five times in a row is five identical buttons to a screen
        // reader listing them; this says which table each one opens.
        aria-label={t('a11y.actionFor', {
          action: over ? t('mine.replay') : playing ? t('desktop.game.show') : t('picker.resume'),
          what: [gameLabel, withWho || t('lobby.games.bots')].filter(Boolean).join(', '),
        })}
        onPress={() =>
          router.push(
            over && row.canReplay
              ? routeForReplay(row.matchId)
              : routeForMatch(row.status, row.isHost, row.matchId),
          )
        }
        style={({ pressed }) => [styles.go, pressed && { borderColor: colors.accent }]}
      >
        <Text style={styles.goText}>{over ? t('mine.replay') : playing ? t('desktop.game.show') : t('picker.resume')}</Text>
      </Pressable>
    </View>
  );
}

const styles = StyleSheet.create({
  row: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
    paddingVertical: 9,
    borderBottomWidth: StyleSheet.hairlineWidth,
    borderBottomColor: colors.border,
  },
  name: { color: colors.text, fontSize: 14, fontWeight: '600' },
  meta: { color: colors.muted, fontSize: 12, marginTop: 2 },
  badge: {
    fontSize: 11,
    fontWeight: '600',
    borderRadius: 6,
    paddingHorizontal: 7,
    paddingVertical: 2,
    overflow: 'hidden',
  },
  badgeTurn: { color: colors.onAccent, backgroundColor: colors.gold },
  badgeQuiet: { color: colors.muted, borderWidth: 1, borderColor: colors.border },
  go: {
    borderWidth: 1,
    borderColor: colors.accentButton,
    borderRadius: 7,
    paddingHorizontal: 10,
    paddingVertical: 4,
  },
  goText: { color: colors.accentButton, fontWeight: '700', fontSize: 12 },
});
