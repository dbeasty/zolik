import { Redirect, router, useLocalSearchParams } from 'expo-router';

import { useOpenGames } from '@/src/desktop/openGames';
import { useEffect, useState } from 'react';
import { ActivityIndicator, Pressable, StyleSheet, Text, View } from 'react-native';

import { nearbyAvailable } from '@/modules/zolik-nearby';
import type { MatchModule } from '@/src/api/matchTypes';
import { FormError } from '@/src/a11y/Field';
import { heading } from '@/src/a11y/props';
import { SettleIn } from '@/src/components/match/SettleIn';
import { OpenTablesCard } from '@/src/components/OpenTablesCard';
import { Screen } from '@/src/components/Screen';
import { TableCodeJoin } from '@/src/components/TableCodeJoin';
import { TableRow } from '@/src/components/TableRow';
import { WaitingCard } from '@/src/components/WaitingCard';
import { useSession } from '@/src/context/SessionContext';
import { LearnButton } from '@/src/guide/LearnButton';
import { useLocale } from '@/src/hooks/useLocale';
import { useMyTables } from '@/src/hooks/useMyTables';
import { useWide } from '@/src/hooks/useWide';
import { formatApiError } from '@/src/lib/apiError';
import { dealWithBots, resolveSetup } from '@/src/lib/dealSetup';
import { moduleLabel } from '@/src/lib/gameLabels';
import { loadGameSetup } from '@/src/lib/gameSetupStore';
import { t } from '@/src/lib/i18n';
import { routeForMatch } from '@/src/lib/matchRoute';
import { gameRowStatus } from '@/src/lib/picker';
import { colors, shared } from '@/src/theme';

/**
 * One game's own page: tables open to anybody and people waiting to play it,
 * and every way into a game of it — Play now against bots, a table for people,
 * a table with no internet (where the app carries its own server), and
 * somebody else's table by its code or invite link. All of them live here
 * rather than on the main menu, so the menu stays a list of games.
 *
 * Play now deals at once, with the settings this player used last time; the
 * settings themselves are a screen further in (`/lobby/setup`), behind the
 * link beside it and behind Open a table, so this page stays about choosing
 * *how* to play rather than being a wall of options.
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
  const openGames = useOpenGames();
  const { client, session, offline } = useSession();
  const [mod, setMod] = useState<MatchModule | null>(null);
  const [error, setError] = useState('');
  const [dealing, setDealing] = useState(false);
  const [dealError, setDealError] = useState('');
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
        <ActivityIndicator aria-label={t('a11y.loading')} color={colors.accent} />
      </Screen>
    );
  }

  const setup = (mode: 'table' | 'bots') =>
    router.push(`/lobby/setup?moduleId=${encodeURIComponent(mod.id)}&mode=${mode}`);

  // Straight to the cards: last time's settings, or the game's own defaults.
  const playNow = async () => {
    if (dealing) return;
    setDealing(true);
    setDealError('');
    try {
      const matchId = await dealWithBots(client, mod, resolveSetup(mod, await loadGameSetup(mod.id)));
      router.push(`/match/${matchId}`);
    } catch (e) {
      setDealError(formatApiError(e));
    } finally {
      setDealing(false);
    }
  };

  const mine = (going ?? []).filter((r) => r.moduleId === mod.id);
  const recent = (finished ?? []).filter((r) => r.moduleId === mod.id).slice(0, 4);
  const { resume } = gameRowStatus(mod.id, mine, [], session?.userId);

  const actions = (
    <>
      <View style={styles.headingRow}>
        <Text style={[styles.heading, { marginTop: 0, marginBottom: 0 }]} {...heading(3)}>
          {t('game.startYourOwn')}
        </Text>
        {/* What Play now deals with, to change before dealing. */}
        {mod.maxPlayers > 1 ? (
          <Pressable
            testID={`play-settings-${mod.id}`}
            role="link"
            aria-label={t('a11y.actionFor', { action: t('settings.title'), what: t('game.playNow') })}
            onPress={() => setup('bots')}
          >
            <Text style={styles.rulesLinkText}>{t('settings.title')} ›</Text>
          </Pressable>
        ) : null}
      </View>
      {mod.maxPlayers === 1 ? (
        // A one-seat game has no table to open and no bots to play: one
        // button, which deals.
        <View style={styles.buttons}>
          <SettleIn kind="deal" delay={0} style={styles.cell}>
            <Pressable
              testID={`play-solo-${mod.id}`}
              accessibilityRole="button"
              style={[shared.button, styles.cellButton]}
              onPress={() => setup('bots')}
            >
              <Text style={shared.buttonText}>{t('setup.dealMeIn')}</Text>
            </Pressable>
          </SettleIn>
        </View>
      ) : (
        <View style={styles.buttons}>
          {/* The quickest way into a game is the highlighted one: bots fill
              the other seats, so it waits for nobody. A screen reader hears
              who sits there; the label alone does not say. */}
          <SettleIn kind="deal" delay={0} style={styles.cell}>
            <Pressable
              testID={`play-bots-${mod.id}`}
              accessibilityRole="button"
              aria-label={`${t('game.playNow')} — ${t('game.playBots')}`}
              aria-busy={dealing}
              disabled={dealing}
              style={[shared.button, styles.cellButton, dealing && { opacity: 0.6 }]}
              onPress={() => void playNow()}
            >
              {dealing ? (
                <ActivityIndicator aria-label={t('a11y.loading')} color={colors.onAccent} />
              ) : (
                <Text style={[shared.buttonText, styles.cellText]}>{t('game.playNow')}</Text>
              )}
            </Pressable>
          </SettleIn>
          <SettleIn kind="deal" delay={70} style={styles.cell}>
            <Pressable
              testID={`play-friends-${mod.id}`}
              accessibilityRole="button"
              style={[shared.button, shared.buttonSecondary, styles.cellButton]}
              onPress={() => setup('table')}
            >
              <Text style={[shared.buttonText, shared.buttonTextSecondary, styles.cellText]}>
                {t('lobby.games.openTable')}
              </Text>
            </Pressable>
          </SettleIn>
        </View>
      )}
      {dealError ? <FormError testID={`play-now-error-${mod.id}`} message={dealError} /> : null}
      {/* Only where the app carries its own server: iOS, Android and the Mac
          app, not the web build or Expo Go. The offline screen comes back
          here once seated, so the next tap opens a table of this game. */}
      {nearbyAvailable ? (
        <SettleIn kind="deal" delay={105} style={{ marginTop: 8 }}>
          <Pressable
            testID={`play-offline-${mod.id}`}
            accessibilityRole="button"
            style={[shared.button, shared.buttonSecondary, styles.cellButton]}
            onPress={() => router.push(`/offline?moduleId=${encodeURIComponent(mod.id)}`)}
          >
            <Text style={[shared.buttonText, shared.buttonTextSecondary, styles.cellText]}>
              {offline ? t('offline.title') : t('home.playOffline')}
            </Text>
          </Pressable>
        </SettleIn>
      ) : null}
      {/* A guided walk-through of this game, at its default settings; the
          setup screen offers the same for whatever is selected there. Shows
          nothing for a game that has no guide yet. */}
      <LearnButton moduleId={mod.id} />
      {session ? (
        <View style={[styles.buttons, { marginTop: 8 }]}>
          {/* Every stored game of this one, not just the latest: the menu
              tile resumes a single table, so the rest are reached from here.
              It opens on the ones still in progress, the tables a player
              most often comes back for; Finished is one tap away. */}
          <SettleIn kind="deal" delay={140} style={styles.cell}>
            <Pressable
              testID={`previous-games-${mod.id}`}
              accessibilityRole="button"
              style={[shared.button, shared.buttonSecondary, styles.cellButton]}
              onPress={() => router.push(`/lobby/mine?moduleId=${encodeURIComponent(mod.id)}`)}
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
                  {openGames.has(resume.matchId) ? t('desktop.game.show') : t('picker.resume')}
                </Text>
              </Pressable>
            ) : null}
          </SettleIn>
        </View>
      ) : null}
      {/* Somebody else's table, by its code or the link they sent. A
          one-seat game has nobody else's table to sit at. */}
      {session && mod.maxPlayers > 1 ? <TableCodeJoin /> : null}
    </>
  );

  const header = (
    <SettleIn kind="deal">
      <View style={styles.header} testID={`module-${mod.id}`}>
        <View style={{ flexShrink: 1 }}>
          <Text style={shared.title} {...heading(2)}>
            {moduleLabel(mod)}
          </Text>
          <Text style={styles.meta}>
            {mod.minPlayers === mod.maxPlayers
              ? mod.minPlayers === 1
                ? t('lobby.games.onePlayer')
                : t('lobby.games.players', { n: mod.minPlayers })
              : t('lobby.games.playerRange', { min: mod.minPlayers, max: mod.maxPlayers })}
          </Text>
        </View>
        <Pressable
          testID={`rules-${mod.id}`}
          accessibilityRole="link"
          // "Rules" of which game, for a screen reader's list of links.
          aria-label={t('a11y.actionFor', { action: t('nav.rules'), what: moduleLabel(mod) })}
          onPress={() => router.push(`/rules?moduleId=${encodeURIComponent(mod.id)}`)}
          style={({ pressed }) => [styles.rulesLink, pressed && styles.pressed]}
        >
          <Text style={styles.rulesLinkText}>{t('nav.rules')} ›</Text>
        </Pressable>
      </View>
    </SettleIn>
  );

  // Tables open to anybody first: sitting down at one is quicker than waiting
  // to be picked up. Both are the online server's, and nobody online can
  // reach a table on this phone.
  const waiting =
    session && !offline ? (
      <>
        {mod.maxPlayers > 1 ? <OpenTablesCard moduleId={mod.id} /> : null}
        <WaitingCard moduleId={mod.id} />
      </>
    ) : null;

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
              <Text style={[styles.heading, { marginTop: 0 }]} {...heading(3)}>
                {t('mine.tabUnfinished')}
              </Text>
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
                <Text style={[styles.heading, { marginTop: 16, marginBottom: 0 }]} {...heading(3)}>
                  {t('mine.tabFinished')}
                </Text>
                <Pressable
                  role="link"
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
  headingRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'baseline',
    marginTop: 20,
    marginBottom: 8,
  },
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
