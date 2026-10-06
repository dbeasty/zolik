import { Redirect, router, useLocalSearchParams } from 'expo-router';
import { useEffect, useState } from 'react';
import { ActivityIndicator, Pressable, StyleSheet, Text, View } from 'react-native';

import type { MatchModule } from '@/src/api/matchTypes';
import { SettleIn } from '@/src/components/match/SettleIn';
import { Screen } from '@/src/components/Screen';
import { TableRow } from '@/src/components/TableRow';
import { WaitingCard } from '@/src/components/WaitingCard';
import { useSession } from '@/src/context/SessionContext';
import { useLocale } from '@/src/hooks/useLocale';
import { useMyTables } from '@/src/hooks/useMyTables';
import { useWide } from '@/src/hooks/useWide';
import { formatApiError } from '@/src/lib/apiError';
import { moduleLabel } from '@/src/lib/gameLabels';
import { t } from '@/src/lib/i18n';
import { routeForMatch } from '@/src/lib/matchRoute';
import { gameRowStatus } from '@/src/lib/picker';
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
  const wide = useWide();
  // This game's tables only: the in-progress ones feed Resume (and, wide, the
  // list beside it), the finished ones the recent results on a wide window.
  const going = useMyTables('unfinished', !!session && !offline, { turns: true });
  const finished = useMyTables('finished', wide && !!session && !offline);

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

  const mine = (going ?? []).filter((r) => r.moduleId === mod.id);
  const recent = (finished ?? []).filter((r) => r.moduleId === mod.id).slice(0, 4);
  const { resume } = gameRowStatus(mod.id, mine, [], session?.userId);

  const actions = (
    <>
      <Text style={styles.heading}>{t('game.startYourOwn')}</Text>
      <View style={styles.buttons}>
        <SettleIn kind="deal" delay={0} style={styles.cell}>
          <Pressable
            testID={`play-friends-${mod.id}`}
            accessibilityRole="button"
            style={[shared.button, styles.cellButton]}
            onPress={() => setup('table')}
          >
            <Text style={shared.buttonText}>{t('lobby.games.openTable')}</Text>
          </Pressable>
        </SettleIn>
        <SettleIn kind="deal" delay={70} style={styles.cell}>
          <Pressable
            testID={`play-bots-${mod.id}`}
            accessibilityRole="button"
            style={[shared.button, shared.buttonSecondary, styles.cellButton]}
            onPress={() => setup('bots')}
          >
            <Text style={[shared.buttonText, shared.buttonTextSecondary, styles.cellText]}>
              {t('game.playBots')}
            </Text>
          </Pressable>
        </SettleIn>
      </View>
      {session ? (
        <View style={[styles.buttons, { marginTop: 8 }]}>
          {/* Every finished game of this one, not just the latest: the menu
              tile resumes a single table, so the rest are reached from here. */}
          <SettleIn kind="deal" delay={140} style={styles.cell}>
            <Pressable
              testID={`previous-games-${mod.id}`}
              accessibilityRole="button"
              style={[shared.button, shared.buttonSecondary, styles.cellButton]}
              onPress={() =>
                router.push(`/lobby/mine?moduleId=${encodeURIComponent(mod.id)}&scope=finished`)
              }
            >
              <Text style={[shared.buttonText, shared.buttonTextSecondary, styles.cellText]}>
                {t('game.previousGames')}
              </Text>
            </Pressable>
          </SettleIn>
          {/* Only with a table to go back to. Without one the cell is left
              empty rather than filled with something else, so the grid keeps
              its shape. */}
          <SettleIn kind="deal" delay={210} style={styles.cell}>
            {resume ? (
              <Pressable
                testID={`game-resume-${mod.id}`}
                accessibilityRole="button"
                style={[shared.button, shared.buttonSecondary, styles.cellButton]}
                onPress={() =>
                  router.push(routeForMatch(resume.status, resume.isHost, resume.matchId))
                }
              >
                <Text style={[shared.buttonText, shared.buttonTextSecondary, styles.cellText]}>
                  {t('picker.resume')}
                </Text>
              </Pressable>
            ) : null}
          </SettleIn>
        </View>
      ) : null}
    </>
  );

  const header = (
    <SettleIn kind="deal">
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
    </SettleIn>
  );

  // The waiting room is the online server's, and nobody online can pick
  // up a player at a table on this phone.
  const waiting = session && !offline ? <WaitingCard moduleId={mod.id} /> : null;

  if (!wide) {
    return (
      <Screen scroll>
        {header}
        {waiting}
        {actions}
      </Screen>
    );
  }

  return (
    <Screen scroll wide>
      {header}
      <View style={styles.wideCols}>
        <View style={styles.wideLeft}>
          {actions}
          <View style={{ marginTop: 16 }}>{waiting}</View>
        </View>
        <View style={styles.wideRight} testID={`game-side-${mod.id}`}>
          {mine.length > 0 ? (
            <SettleIn kind="deal" delay={140}>
              <Text style={[styles.heading, { marginTop: 0 }]}>{t('mine.tabUnfinished')}</Text>
              <View style={styles.panelCard}>
                {mine.map((row) => (
                  <TableRow key={row.matchId} row={row} selfId={session?.userId} />
                ))}
              </View>
            </SettleIn>
          ) : null}
          {recent.length > 0 ? (
            <SettleIn kind="deal" delay={210}>
              <View style={styles.recentHead}>
                <Text style={[styles.heading, { marginTop: 16, marginBottom: 0 }]}>
                  {t('mine.tabFinished')}
                </Text>
                <Pressable
                  onPress={() =>
                    router.push(`/lobby/mine?moduleId=${encodeURIComponent(mod.id)}&scope=finished`)
                  }
                >
                  <Text style={styles.rulesLinkText}>{t('game.previousGames')} ›</Text>
                </Pressable>
              </View>
              <View style={styles.panelCard}>
                {recent.map((row) => (
                  <TableRow key={row.matchId} row={row} selfId={session?.userId} />
                ))}
              </View>
            </SettleIn>
          ) : null}
        </View>
      </View>
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
  buttons: { flexDirection: 'row', gap: 8 },
  cell: { flex: 1 },
  cellButton: { marginBottom: 0, paddingHorizontal: 8, minHeight: 50, justifyContent: 'center' },
  cellText: { fontSize: 15, textAlign: 'center' },
  wideCols: { flexDirection: 'row', gap: 24, alignItems: 'flex-start' },
  wideLeft: { flex: 5, minWidth: 0 },
  wideRight: { flex: 7, minWidth: 0 },
  recentHead: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'baseline', marginBottom: 8 },
  panelCard: {
    backgroundColor: colors.surface,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 10,
    paddingHorizontal: 12,
  },
});
