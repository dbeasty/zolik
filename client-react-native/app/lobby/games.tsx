import { Redirect, router, useLocalSearchParams } from 'expo-router';
import { useEffect, useState } from 'react';
import { ActivityIndicator, Pressable, StyleSheet, Text, View } from 'react-native';

import type { MatchModule } from '@/src/api/matchTypes';
import { Screen } from '@/src/components/Screen';
import { WaitingCard } from '@/src/components/WaitingCard';
import { useSession } from '@/src/context/SessionContext';
import { useLocale } from '@/src/hooks/useLocale';
import { formatApiError } from '@/src/lib/apiError';
import { moduleLabel } from '@/src/lib/gameLabels';
import { t } from '@/src/lib/i18n';
import { colors, shared } from '@/src/theme';

/**
 * One game's own page: who is waiting to play it, and the two ways to start
 * one — a table for people, or a game against bots.
 *
 * The settings are a screen further in (`/lobby/setup`), reached from either
 * button, so this page stays about choosing *how* to play rather than being a
 * wall of options in front of the people waiting.
 *
 * Without a `moduleId` there is nothing to show: the list of games is the main
 * menu now, and an old link to this screen goes there.
 */
export default function GamePage() {
  const { moduleId } = useLocalSearchParams<{ moduleId?: string }>();
  // Nothing else here re-renders once the saved language has loaded.
  useLocale();
  if (!moduleId) return <Redirect href="/" />;
  return <GamePageFor moduleId={String(moduleId)} />;
}

function GamePageFor({ moduleId }: { moduleId: string }) {
  const { client, session, offline } = useSession();
  const [mod, setMod] = useState<MatchModule | null>(null);
  const [error, setError] = useState('');

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const found = (await client.modules()).find((m) => m.id === moduleId);
        if (cancelled) return;
        if (!found) {
          // A stale or mistyped link: the main menu is the place to pick from.
          router.replace('/');
          return;
        }
        setMod(found);
      } catch (e) {
        if (!cancelled) setError(formatApiError(e));
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [client, moduleId]);

  if (error) {
    return (
      <Screen title={t('nav.games')}>
        <Text testID="games-error" style={shared.error}>
          {error}
        </Text>
      </Screen>
    );
  }
  if (!mod) {
    return (
      <Screen title={t('nav.games')}>
        <ActivityIndicator color={colors.accent} />
      </Screen>
    );
  }

  const setup = (mode: 'table' | 'bots') =>
    router.push(`/lobby/setup?moduleId=${encodeURIComponent(mod.id)}&mode=${mode}`);

  return (
    <Screen scroll>
      <View style={styles.header} testID={`module-${mod.id}`}>
        <View style={{ flexShrink: 1 }}>
          <Text style={shared.title}>{moduleLabel(mod)}</Text>
          <Text style={styles.meta}>
            {mod.minPlayers === mod.maxPlayers
              ? t('lobby.games.players', { n: mod.minPlayers })
              : t('lobby.games.playerRange', { min: mod.minPlayers, max: mod.maxPlayers })}
          </Text>
        </View>
        <Pressable
          testID={`rules-${mod.id}`}
          accessibilityRole="link"
          onPress={() => router.push(`/rules?moduleId=${encodeURIComponent(mod.id)}`)}
          style={({ pressed }) => [styles.rulesLink, pressed && styles.pressed]}
        >
          <Text style={styles.rulesLinkText}>{t('nav.rules')} ›</Text>
        </Pressable>
      </View>

      {/* The waiting room is the online server's, and nobody online can pick
          up a player at a table on this phone. */}
      {session && !offline ? <WaitingCard moduleId={mod.id} /> : null}

      <Text style={styles.heading}>{t('game.startYourOwn')}</Text>
      <Pressable
        testID={`play-friends-${mod.id}`}
        accessibilityRole="button"
        style={shared.button}
        onPress={() => setup('table')}
      >
        <Text style={shared.buttonText}>{t('lobby.games.openTable')}</Text>
      </Pressable>
      <Pressable
        testID={`play-bots-${mod.id}`}
        accessibilityRole="button"
        style={[shared.button, shared.buttonSecondary]}
        onPress={() => setup('bots')}
      >
        <Text style={[shared.buttonText, shared.buttonTextSecondary]}>{t('game.playBots')}</Text>
      </Pressable>
    </Screen>
  );
}

const styles = StyleSheet.create({
  header: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'flex-start',
    gap: 12,
  },
  meta: { color: colors.muted, fontSize: 13, marginTop: -4 },
  rulesLink: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 6,
    paddingHorizontal: 10,
    paddingVertical: 5,
    marginTop: 4,
  },
  rulesLinkText: { color: colors.accentButton, fontSize: 12, fontWeight: '700' },
  pressed: { borderColor: colors.accent },
  heading: { color: colors.muted, fontSize: 13, fontWeight: '600', marginTop: 20, marginBottom: 8 },
});
