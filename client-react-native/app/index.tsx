import { router, useFocusEffect, useIsFocused, type Href } from 'expo-router';
import { useCallback, useEffect, useRef, useState } from 'react';
import { ActivityIndicator, Pressable, StyleSheet, Text, TextInput, View } from 'react-native';
import type { StyleProp, ViewStyle } from 'react-native';

import type { MatchModule, StoredTable } from '@/src/api/matchTypes';
import { BuildFooter } from '@/src/components/BuildFooter';
import { useOrderedModules } from '@/src/components/GameButtons';
import { StartHero } from '@/src/components/fun/StartHero';
import { SettleIn } from '@/src/components/match/SettleIn';
import { Screen } from '@/src/components/Screen';
import { nearbyAvailable } from '@/modules/zolik-nearby';
import { useAvailability } from '@/src/context/AvailabilityContext';
import { useSession } from '@/src/context/SessionContext';
import { useLocale } from '@/src/hooks/useLocale';
import { useWaitingLobbyStatus } from '@/src/hooks/useWaitingLobbyStatus';
import { moduleLabel, moduleName } from '@/src/lib/gameLabels';
import { t } from '@/src/lib/i18n';
import { codeFromInviteInput } from '@/src/lib/inviteLink';
import { hasSeenIntro } from '@/src/lib/introStore';
import { routeForMatch } from '@/src/lib/matchRoute';
import { consumePendingDestination } from '@/src/lib/pendingDestination';
import { gameRowStatus } from '@/src/lib/picker';
import { useInvites } from '@/src/notify/InviteProvider';
import { InviteRow } from '@/src/notify/InviteRow';
import { colors, shared } from '@/src/theme';

function MenuButton({
  label,
  onPress,
  secondary,
  style,
}: {
  label: string;
  onPress: () => void;
  secondary?: boolean;
  /** Only for spacing — the button's own look is not negotiable, which is
   *  the point: every button on this screen is this one. */
  style?: StyleProp<ViewStyle>;
}) {
  return (
    <Pressable
      style={[shared.button, secondary && shared.buttonSecondary, style]}
      onPress={onPress}
    >
      <Text style={[shared.buttonText, secondary && shared.buttonTextSecondary]}>
        {label}
      </Text>
    </Pressable>
  );
}

/**
 * The main menu is the list of games.
 *
 * It used to be a status page — who you were playing as, your recent tables,
 * the waiting room — with "Play" leading to the list of games one screen
 * further in. Everything on it was about something that belonged somewhere
 * else, so each piece went where it belongs: your games to the account menu,
 * the waiting room to each game's own page. What is left is the question a
 * player opens the app to answer, "what shall I play?", and the two things
 * worth knowing while answering it, which ride on each game's row: a table of
 * it waiting for you, and people waiting to play it.
 */
export default function MainMenu() {
  const { session, loading, offline } = useSession();
  const introChecked = useIntroGate();
  useFollowPendingDestination(!!session && !loading);
  // Nothing else here re-renders once the saved language has loaded.
  useLocale();

  if (loading || !introChecked) {
    return (
      <Screen>
        <ActivityIndicator color="#3d8bfd" />
      </Screen>
    );
  }

  return (
    <Screen title="Jokerless" subtitle={offline ? t('offline.subtitle') : undefined} scroll>
      <StartHero />
      <WaitingInvitesCard />
      <GameRows />

      {session ? <TableCodeJoin /> : null}

      <View style={{ marginTop: 16 }}>
        {/* Only where the app carries its own server: iOS and Android, not
            the web build or Expo Go. Offered to everyone, signed in or not,
            because the point is that it needs nothing from the internet. */}
        {nearbyAvailable ? (
          <MenuButton
            label={offline ? t('offline.title') : t('home.playOffline')}
            secondary
            onPress={() => router.push('/offline')}
          />
        ) : null}
        {/* Settings, sign-out, your games and the second-tier screens are
            behind the face in the top corner, which is where a player looks
            for themselves. See `AccountMenu`. Somebody with no session yet
            gets the two ways to get one. */}
        {!session ? (
          <>
            <Text style={[shared.status, { marginTop: 0, marginBottom: 10 }]}>
              {t('home.signInPrompt')}
            </Text>
            <MenuButton label={t('settings.signIn')} onPress={() => router.push('/auth/login')} />
            <MenuButton
              label={t('home.continueAsGuest')}
              secondary
              onPress={() => router.push('/auth/guest')}
            />
          </>
        ) : null}
      </View>

      <BuildFooter onPressVersions={() => router.push('/about')} />
    </Screen>
  );
}

/**
 * One row per game, each carrying what the player needs to know about it
 * before choosing: a table of it waiting for them (and a way straight back
 * to it), and how many people are waiting to play it.
 *
 * The row opens the game's own page; Resume skips that and goes back to the
 * table. Without a session both lead to the guest screen, which comes back
 * here afterwards.
 */
function GameRows() {
  const { client, session, offline } = useSession();
  const { modules, error } = useOrderedModules();
  // null until asked: an empty list and "not yet known" must not look the
  // same, or every row would flash badge-less on each visit.
  const [tables, setTables] = useState<StoredTable[] | null>(null);
  // Polled only while the menu is on screen: it stays mounted under every
  // screen it leads to, a match included.
  const focused = useIsFocused();
  const { players: waiting } = useWaitingLobbyStatus(!!session && !offline && focused);

  // Refetched whenever the menu comes back into view: the usual way back here
  // is from a table, which has just changed whose turn it is.
  useFocusEffect(
    useCallback(() => {
      if (!session) {
        setTables([]);
        return undefined;
      }
      let cancelled = false;
      client
        .listMyTables('unfinished', { turns: true })
        .then((rows) => {
          if (!cancelled) setTables(rows);
        })
        .catch(() => {
          // A quiet failure costs nothing real: the rows still open each
          // game, and the full list is in the account menu.
          if (!cancelled) setTables([]);
        });
      return () => {
        cancelled = true;
      };
    }, [client, session]),
  );

  if (error) {
    return (
      <Text testID="games-error" style={shared.error}>
        {error}
      </Text>
    );
  }
  if (!modules) return <ActivityIndicator color={colors.accent} />;

  return (
    <View testID="games-list">
      {session && !offline ? <AvailabilityStrip modules={modules} /> : null}
      <Text style={styles.heading}>{t('picker.title')}</Text>
      {modules.map((mod, i) => (
        // Dealt onto the page one after another, like cards to a table.
        <SettleIn key={mod.id} kind="deal" delay={Math.min(i, 8) * 70}>
          <GameRow
            mod={mod}
            signedIn={!!session}
            status={gameRowStatus(mod.id, tables ?? [], waiting, session?.userId)}
          />
        </SettleIn>
      ))}
    </View>
  );
}

function GameRow({
  mod,
  signedIn,
  status,
}: {
  mod: MatchModule;
  signedIn: boolean;
  status: ReturnType<typeof gameRowStatus>;
}) {
  const open = () => {
    if (!signedIn) {
      router.push('/auth/guest');
      return;
    }
    router.push(`/lobby/games?moduleId=${encodeURIComponent(mod.id)}`);
  };
  const { resume } = status;

  return (
    <Pressable
      testID={`game-${mod.id}`}
      accessibilityRole="button"
      onPress={open}
      style={({ pressed }) => [styles.row, pressed && styles.rowPressed]}
    >
      <View style={styles.rowTop}>
        <Text style={styles.name}>{moduleLabel(mod)}</Text>
        <Text style={styles.chevron} aria-hidden>
          ›
        </Text>
      </View>
      {resume || status.waiting > 0 ? (
        <View style={styles.badges}>
          {resume ? (
            <Text
              testID={`picker-${mod.id}-table`}
              style={[styles.badge, status.yourTurn ? styles.badgeTurn : styles.badgeQuiet]}
            >
              {status.yourTurn
                ? t('picker.yourTurn')
                : status.tables > 1
                  ? t('picker.tablesMany', { n: status.tables })
                  : t(`mine.status.${resume.status}`, undefined, resume.status)}
            </Text>
          ) : null}
          {status.waiting > 0 ? (
            <Text testID={`picker-${mod.id}-waiting`} style={[styles.badge, styles.badgeWaiting]}>
              {t('picker.waiting', { n: status.waiting })}
            </Text>
          ) : null}
          <View style={{ flex: 1 }} />
          {resume ? (
            <Pressable
              testID={`picker-${mod.id}-resume`}
              accessibilityRole="button"
              onPress={() => router.push(routeForMatch(resume.status, resume.isHost, resume.matchId))}
              style={({ pressed }) => [styles.resume, pressed && styles.rowPressed]}
            >
              <Text style={styles.resumeText}>{t('picker.resume')}</Text>
            </Pressable>
          ) : null}
        </View>
      ) : null}
    </Pressable>
  );
}

/**
 * The one reminder the waiting room leaves on the menu: that you are in it.
 * Being available outlives the game page it was switched on from, so without
 * this a player could be picked up by a host they had long forgotten asking.
 */
function AvailabilityStrip({ modules }: { modules: MatchModule[] }) {
  const { availableFor, setAvailableFor } = useAvailability();
  if (!availableFor) return null;
  const mod = modules.find((m) => m.id === availableFor);
  return (
    <View style={[shared.card, styles.strip]} testID="availability-strip">
      <Pressable
        style={{ flex: 1 }}
        onPress={() => router.push(`/lobby/games?moduleId=${encodeURIComponent(availableFor)}`)}
      >
        <Text style={{ color: colors.success, fontWeight: '600' }}>
          {t('picker.waitingFor', { game: mod ? moduleLabel(mod) : moduleName(availableFor) })}
        </Text>
      </Pressable>
      <Pressable testID="availability-strip-stop" onPress={() => setAvailableFor(null)}>
        <Text style={styles.stripStop}>{t('waiting.stop')}</Text>
      </Pressable>
    </View>
  );
}

/**
 * A table somebody else opened, by its code — or by the whole link, pasted,
 * because pasting the thing you were sent is the obvious move. `/join/<code>`
 * does the rest, exactly as it does for a link that was tapped.
 */
function TableCodeJoin() {
  const [code, setCode] = useState('');
  const [error, setError] = useState('');
  const join = () => {
    const trimmed = codeFromInviteInput(code);
    if (!trimmed) {
      setError(t('lobby.join.needCode'));
      return;
    }
    setError('');
    setCode('');
    router.push(`/join/${encodeURIComponent(trimmed)}` as Href);
  };
  return (
    <View style={{ marginTop: 16 }}>
      <View style={styles.codeRow}>
        <TextInput
          testID="table-code-input"
          value={code}
          onChangeText={(v) => {
            setCode(v);
            if (error) setError('');
          }}
          onSubmitEditing={join}
          placeholder={t('lobby.join.placeholder')}
          placeholderTextColor={colors.muted}
          autoCapitalize="characters"
          autoCorrect={false}
          style={[shared.input, styles.codeInput]}
        />
        <Pressable
          testID="table-code-join"
          accessibilityRole="button"
          onPress={join}
          style={[shared.button, shared.buttonSecondary, styles.codeButton]}
        >
          <Text style={[shared.buttonText, shared.buttonTextSecondary]}>{t('picker.join')}</Text>
        </Pressable>
      </View>
      {error ? <Text style={shared.error}>{error}</Text> : null}
    </View>
  );
}

/**
 * Whether this device has cleared the first-run intro, or never needed to
 * see it fetched at all.
 *
 * Stays `false` — never `true` — for a device that has not seen it: the
 * redirect to `/intro` it fires unmounts this screen, so there is no second
 * state to hold. Guarded by a ref rather than an effect dependency, same
 * trick as `useFollowPendingDestination` below, so a re-render mid-check
 * cannot ask twice and race the eventual `router.replace`.
 */
function useIntroGate(): boolean {
  const [checked, setChecked] = useState(false);
  const gated = useRef(false);

  useEffect(() => {
    if (gated.current) return;
    gated.current = true;
    let live = true;
    hasSeenIntro().then((seen) => {
      if (!live) return;
      if (!seen) {
        router.replace('/intro');
        return;
      }
      setChecked(true);
    });
    return () => {
      live = false;
    };
  }, []);

  return checked;
}

/**
 * Resume an invite that was interrupted by signing in.
 *
 * Every account sign-in path — email code, username, OAuth callback, the
 * legacy registration — finishes with `router.replace('/')`, so the menu is
 * where they all come back to and therefore the only place one hook covers
 * them all. (Guest sign-in consumes its own first; see app/auth/guest.tsx.)
 *
 * Consumed, never merely read: the note is deleted before the navigation it
 * causes, so an invite followed once cannot ambush somebody on their next
 * visit to the menu. Nothing happens without a session, which is what keeps
 * this from firing during the moment before storage has been read.
 */
function useFollowPendingDestination(ready: boolean) {
  // Once per mount. The effect's dependencies settle more than once while the
  // session loads, and consuming twice would race the storage delete.
  const followed = useRef(false);

  useEffect(() => {
    if (!ready || followed.current) return;
    followed.current = true;
    let live = true;
    consumePendingDestination().then((path) => {
      // Cast because expo-router types routes as literals and this one is
      // only known at run time. Safe by construction rather than by assertion:
      // `pendingDestination` stores nothing that is not a path rooted at `/`,
      // and re-checks on the way out — a route that no longer exists lands on
      // the app's own not-found screen, which is the same thing a stale link
      // pasted into the address bar does.
      if (live && path) router.replace(path as Href);
    });
    return () => {
      live = false;
    };
  }, [ready]);
}

/**
 * "Tables waiting for you": every invite still queued, for the player who
 * let the banner go by — or who arrives here from a notification.
 *
 * Absent when there are none, for the same reason `MyTablesCard` is: an
 * empty card on every visit would be clutter on the one screen designed to
 * have almost nothing on it.
 */
function WaitingInvitesCard() {
  const { invites, join, dismiss, joiningId } = useInvites();
  if (invites.length === 0) return null;
  return (
    <View style={[shared.card, { marginTop: 12 }]} testID="waiting-invites-card">
      <Text style={{ color: colors.text, fontWeight: '600', marginBottom: 4 }}>
        {t('notify.waitingCard.title')}
      </Text>
      {invites.map((invite) => (
        <InviteRow
          key={invite.id}
          invite={invite}
          palette={colors}
          busy={joiningId === invite.id}
          onJoin={() => void join(invite.id)}
          onDismiss={() => dismiss(invite.id)}
        />
      ))}
    </View>
  );
}

const styles = StyleSheet.create({
  heading: { color: colors.muted, fontSize: 13, fontWeight: '600', marginTop: 12, marginBottom: 8 },
  row: {
    backgroundColor: colors.surface,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 10,
    paddingVertical: 12,
    paddingHorizontal: 14,
    marginBottom: 8,
  },
  rowPressed: { borderColor: colors.accent },
  rowTop: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between' },
  name: { color: colors.text, fontSize: 16, fontWeight: '600' },
  chevron: { color: colors.muted, fontSize: 18 },
  badges: { flexDirection: 'row', alignItems: 'center', flexWrap: 'wrap', gap: 6, marginTop: 8 },
  badge: {
    fontSize: 12,
    fontWeight: '600',
    borderRadius: 6,
    paddingHorizontal: 8,
    paddingVertical: 3,
    overflow: 'hidden',
  },
  badgeTurn: { color: colors.onAccent, backgroundColor: colors.gold },
  badgeQuiet: { color: colors.muted, borderWidth: 1, borderColor: colors.border },
  badgeWaiting: { color: colors.onAccent, backgroundColor: colors.success },
  resume: {
    borderWidth: 1,
    borderColor: colors.accentButton,
    borderRadius: 8,
    paddingHorizontal: 12,
    paddingVertical: 5,
  },
  resumeText: { color: colors.accentButton, fontWeight: '700', fontSize: 13 },
  strip: { flexDirection: 'row', alignItems: 'center', gap: 12, marginTop: 4, marginBottom: 4 },
  stripStop: { color: colors.muted, fontSize: 13, textDecorationLine: 'underline' },
  codeRow: { flexDirection: 'row', gap: 8, alignItems: 'flex-start' },
  codeInput: { flex: 1, marginBottom: 0 },
  codeButton: { marginBottom: 0, paddingVertical: 12 },
});
