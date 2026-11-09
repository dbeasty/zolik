import { useEffect, useMemo, useState } from 'react';
import { ActivityIndicator, Pressable, StyleSheet, Text, View } from 'react-native';

import type { MatchModule } from '@/src/api/matchTypes';
import { useAgeGate } from '@/src/age/AgeGate';
import { useSession } from '@/src/context/SessionContext';
import { useLocale } from '@/src/hooks/useLocale';
import { formatApiError } from '@/src/lib/apiError';
import { moduleLabel } from '@/src/lib/gameLabels';
import { orderModules } from '@/src/lib/gameOrder';
import { t } from '@/src/lib/i18n';
import { colors } from '@/src/theme';

/**
 * Every game this server hosts, one button each — the way into a game's own
 * setup. Shown by the first-run intro; the main menu lists the same games, in
 * the same order, through `useOrderedModules`, so the list a new
 * visitor sees and the one a returning player picks from are the same list.
 *
 * Rendered from `/modules`, which needs no session: the intro shows it to a
 * visitor who has not signed in yet. Ordered by the player's own history when
 * there is one to read, the general popularity ranking otherwise.
 */
export function GameButtons({ onPick }: { onPick: (mod: MatchModule) => void }) {
  useLocale();
  const { modules, error } = useOrderedModules();

  if (error) {
    return (
      <Text testID="games-error" style={styles.error}>
        {error}
      </Text>
    );
  }
  if (!modules) return <ActivityIndicator aria-label={t('a11y.loading')} color={colors.accent} />;

  return (
    <View style={styles.grid} testID="game-buttons">
      {modules.map((mod) => (
        <Pressable
          key={mod.id}
          testID={`game-${mod.id}`}
          accessibilityRole="button"
          onPress={() => onPick(mod)}
          style={({ pressed }) => [styles.button, pressed && styles.pressed]}
        >
          <Text style={styles.name}>{moduleLabel(mod)}</Text>
          <Text style={styles.meta}>
            {mod.minPlayers === mod.maxPlayers
              ? mod.minPlayers === 1
                ? t('lobby.games.onePlayer')
                : t('lobby.games.players', { n: mod.minPlayers })
              : t('lobby.games.playerRange', { min: mod.minPlayers, max: mod.maxPlayers })}
          </Text>
        </Pressable>
      ))}
    </View>
  );
}

/** How long to wait before asking again for a list that could not be fetched. */
const MODULES_RETRY_MS = 5000;

/**
 * Every game this server hosts, in the order a picker should list them: the
 * player's own history when there is one to read, the general popularity
 * ranking otherwise. Shared by `GameButtons` and the main menu's game list.
 */
export function useOrderedModules(): { modules: MatchModule[] | null; error: string } {
  const { client, session } = useSession();
  const [modules, setModules] = useState<MatchModule[] | null>(null);
  const [error, setError] = useState('');
  // Which try this is. A phone with no connection gets one back without being
  // told, and the list has to notice, or it stays on the error until a restart.
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    if (!error) return;
    const retry = setTimeout(() => setAttempt((n) => n + 1), MODULES_RETRY_MS);
    return () => clearTimeout(retry);
  }, [error, attempt]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const fetched = await client.modules();
        let playCounts: Record<string, number> | undefined;
        // A guest has no lifetime record: the server refuses them one by design.
        if (session?.accessToken && !session.isGuest) {
          try {
            const stats = await client.getStats();
            playCounts = {};
            for (const [id, tally] of Object.entries(stats.byModule ?? {})) {
              playCounts[id] = tally.matches;
            }
          } catch {
            // A personalization nicety; the default order still stands.
          }
        }
        if (!cancelled) {
          setModules(orderModules(fetched, playCounts));
          // Including an error from another client: starting offline play swaps
          // the client, and the list left the old failure on screen.
          setError('');
        }
      } catch (e) {
        if (!cancelled) setError(formatApiError(e));
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [client, session?.accessToken, attempt]);

  // Under 18, the chip games are not on the list at all.
  const { hidesFromLists } = useAgeGate();
  const shown = useMemo(
    () => (modules ? modules.filter((m) => !hidesFromLists(m)) : null),
    [modules, hidesFromLists],
  );
  return { modules: shown, error };
}

const styles = StyleSheet.create({
  grid: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: 8,
  },
  button: {
    flexGrow: 1,
    flexBasis: '45%',
    backgroundColor: colors.surface,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 10,
    paddingVertical: 12,
    paddingHorizontal: 10,
    alignItems: 'center',
  },
  pressed: {
    borderColor: colors.accent,
  },
  name: {
    fontSize: 15,
    fontWeight: '600',
    color: colors.text,
  },
  meta: {
    fontSize: 12,
    color: colors.muted,
    marginTop: 2,
  },
  error: {
    color: colors.danger,
  },
});
