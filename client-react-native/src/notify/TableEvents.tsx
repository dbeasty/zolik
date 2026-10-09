import { router } from 'expo-router';
import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react';
import { AppState, Platform, Pressable, StyleSheet, Text, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import type { MatchPlayer } from '@/src/api/matchTypes';
import { Avatar } from '@/src/components/avatars/Avatar';
import { avatarFor } from '@/src/components/avatars/catalogue';
import { useLocale } from '@/src/hooks/useLocale';
import { moduleName } from '@/src/lib/gameLabels';
import { t } from '@/src/lib/i18n';
import { colors } from '@/src/theme';
import { notifyNow } from '@/src/notify/localNotify';
import { flashTitle } from '@/src/notify/webTitle';

/**
 * "Bea joined the table", wherever the player is looking.
 *
 * Everybody already sitting at a table hears when somebody else sits down:
 * the host waiting for their friends most of all. The news comes from two
 * places, and the same arrival from both is shown once:
 *
 *  - **Online**, the cloud says so on the player's own socket (and as a push
 *    when the app is not open), wherever they are in the app.
 *  - **At a table on a phone**, which has no such socket, the screens that
 *    show the table's players notice a new one (`useAnnounceArrivals`).
 *
 * In front it is a banner over the top of the screen, gone after a few
 * seconds and touchable through. Behind - a hidden browser tab - it is a
 * system notification where the browser allows one, and a blinking tab title
 * where it does not. A phone app in the background is told by its OS: by a
 * push online, and by the phone's own embedded server at a table it hosts.
 */

export type Arrival = {
  matchId: string;
  playerId: string;
  name: string;
  avatar?: string;
  moduleId?: string;
  joinCode?: string;
};

/** How long the banner stays. */
export const ARRIVAL_MS = 5_000;

type Ctx = { announce: (a: Arrival) => void };

const TableEventsContext = createContext<Ctx>({ announce: () => {} });

export function useTableEvents(): Ctx {
  return useContext(TableEventsContext);
}

export function TableEventsProvider({ children }: { children: React.ReactNode }) {
  const [shown, setShown] = useState<Arrival | null>(null);
  const seen = useRef(new Set<string>());

  const announce = useCallback((a: Arrival) => {
    if (!a.matchId || !a.playerId) return;
    const key = `${a.matchId}:${a.playerId}`;
    if (seen.current.has(key)) return;
    seen.current.add(key);
    setShown(a);
    void notifyBehind(a);
  }, []);

  useEffect(() => {
    if (!shown) return undefined;
    const timer = setTimeout(() => setShown(null), ARRIVAL_MS);
    return () => clearTimeout(timer);
  }, [shown]);

  const value = useMemo(() => ({ announce }), [announce]);
  return (
    <TableEventsContext.Provider value={value}>
      {children}
      {shown ? <ArrivalBanner arrival={shown} onClose={() => setShown(null)} /> : null}
    </TableEventsContext.Provider>
  );
}

function arrivalText(a: Arrival) {
  const game = a.moduleId ? moduleName(a.moduleId) : '';
  return {
    title: t('notify.push.joinedTitle', { name: a.name, game }),
    body: game ? t('notify.push.joinedBody', { name: a.name, game }) : '',
  };
}

function arrivalPath(a: Arrival): string {
  return a.joinCode ? `/join/${encodeURIComponent(a.joinCode)}` : `/match/${encodeURIComponent(a.matchId)}`;
}

/**
 * The system notification, for a player who is not looking: a hidden browser
 * tab, or (where its JS is still running) an app in the background.
 */
async function notifyBehind(a: Arrival): Promise<void> {
  const { title, body } = arrivalText(a);
  const tag = `joined:${a.matchId}:${a.playerId}`;
  if (Platform.OS === 'web') {
    if (typeof document === 'undefined' || !document.hidden) return;
    flashTitle(title);
    // Only where the browser already allows it: a page served by a phone
    // over plain http never can, and asking here would be a prompt out of
    // nowhere. The tag matches the cloud's push, so the two never stack.
    try {
      if (typeof Notification === 'undefined' || Notification.permission !== 'granted') return;
      if (!window.isSecureContext) return;
      const reg = await navigator.serviceWorker?.getRegistration?.();
      const options = { body, tag, icon: '/icon-192.png', data: { url: arrivalPath(a) } };
      if (reg) await reg.showNotification(title, options);
      else new Notification(title, options);
    } catch {
      // A notification that could not be shown is the title's job instead.
    }
    return;
  }
  if (AppState.currentState === 'active') return;
  await notifyNow(tag, title, body, arrivalPath(a));
}

function ArrivalBanner({ arrival, onClose }: { arrival: Arrival; onClose: () => void }) {
  useLocale();
  const insets = useSafeAreaInsets();
  const { title, body } = arrivalText(arrival);
  return (
    <View pointerEvents="box-none" style={[styles.wrap, { top: insets.top + 8 }]}>
      <Pressable
        testID="arrival-banner"
        accessibilityRole="alert"
        style={styles.card}
        onPress={() => {
          onClose();
          router.push(arrivalPath(arrival) as Parameters<typeof router.push>[0]);
        }}
      >
        <Avatar spec={avatarFor(arrival.playerId, false, arrival.avatar)} size={32} />
        <View style={{ flexShrink: 1 }}>
          <Text style={styles.title}>{title}</Text>
          {body ? <Text style={styles.body}>{body}</Text> : null}
        </View>
      </Pressable>
    </View>
  );
}

/**
 * Announces the players who appear at a table this screen is showing.
 *
 * The first list it is given is what was already there, so opening a table
 * announces nobody; after that, every new seat but the viewer's own is news.
 * For a table on a phone this is the only source; online it agrees with the
 * cloud's message and is shown once.
 */
export function useAnnounceArrivals(
  matchId: string | undefined,
  players: Pick<MatchPlayer, 'id' | 'name' | 'isAI' | 'avatar'>[] | undefined,
  viewerId: string | undefined,
  extra: { moduleId?: string; joinCode?: string } = {},
) {
  const { announce } = useTableEvents();
  const known = useRef<{ matchId: string; ids: Set<string> } | null>(null);
  const { moduleId, joinCode } = extra;
  useEffect(() => {
    if (!matchId || !players) return;
    if (!known.current || known.current.matchId !== matchId) {
      known.current = { matchId, ids: new Set(players.map((p) => p.id)) };
      return;
    }
    for (const p of players) {
      if (known.current.ids.has(p.id)) continue;
      known.current.ids.add(p.id);
      if (p.isAI || p.id === viewerId) continue;
      announce({ matchId, playerId: p.id, name: p.name, avatar: p.avatar, moduleId, joinCode });
    }
  }, [announce, matchId, players, viewerId, moduleId, joinCode]);
}

const styles = StyleSheet.create({
  wrap: { position: 'absolute', left: 12, right: 12, alignItems: 'center', zIndex: 1000 },
  card: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 10,
    maxWidth: 520,
    width: '100%',
    paddingVertical: 10,
    paddingHorizontal: 14,
    borderRadius: 12,
    borderWidth: 1,
    borderColor: colors.accent,
    backgroundColor: colors.surface,
  },
  title: { color: colors.text, fontWeight: '700' },
  body: { color: colors.muted, marginTop: 2 },
});
