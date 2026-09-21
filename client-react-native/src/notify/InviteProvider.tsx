import { router, usePathname } from 'expo-router';
import React, { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react';
import { Platform } from 'react-native';

import { nearbyAvailable } from '@/modules/zolik-nearby';
import { apiClient } from '@/src/api/client';
import type { MeWSMessage } from '@/src/api/types';
import { loadGuestId, loadOfflineName, NearbyVersionError, useSession } from '@/src/context/SessionContext';
import { formatApiError } from '@/src/lib/apiError';
import { BleHostKeyChanged } from '@/src/net/ble/transport';
import { guestNameFor } from '@/src/lib/guestName';
import { t } from '@/src/lib/i18n';
import { startPushListeners, syncPushRegistration } from '@/src/notify/push';
import { useDeviceFlag } from '@/src/notify/prefs';
import {
  EMPTY_QUEUE,
  bannerInvite,
  dismiss as dismissInQueue,
  dismissHost,
  expire,
  inviteFromWire,
  lostOnWifi,
  receive,
  remove,
  revoke,
  shelve as shelveInQueue,
  type QueueState,
} from '@/src/notify/queue';
import { followInvite } from '@/src/notify/routing';
import { subjectKeyForSession } from '@/src/notify/subjectKey';
import type { Invite } from '@/src/notify/types';
import { useAppActive } from '@/src/notify/useAppActive';
import { useNearbyWatcher } from '@/src/notify/useNearbyWatcher';
import { useUserSocket } from '@/src/notify/useUserSocket';
import { showUnreadInTitle } from '@/src/notify/webTitle';

type InviteContextValue = {
  /** Every table waiting for this player, newest first. */
  invites: Invite[];
  /** The one the banner is showing, if any. */
  banner: Invite | null;
  /** The player is at a table: the banner shrinks to a pill so it never
   *  covers the game. */
  inMatch: boolean;
  join: (id: string) => Promise<void>;
  /** "Not now". */
  dismiss: (id: string) => void;
  /** Silences the invite's host for good, and drops what they sent. */
  muteHost: (id: string) => Promise<void>;
  /** The banner has had its turn; the invite stays on the list. */
  shelve: (id: string) => void;
  /** The waiting room seated this player (`lobby_invited`). */
  receiveSeated: (matchId: string, joinCode: string) => void;
  joiningId: string;
  error: string;
  clearError: () => void;
  /** Circle requests waiting for an answer — the account menu's badge. */
  circleRequests: number;
  /** Bumped whenever the circle changed on the server, so screens showing it
   *  know to read it again. */
  circleVersion: number;
  refreshCircle: () => void;
};

const InviteContext = createContext<InviteContextValue | null>(null);

/** How often quiet invites are swept. */
const SWEEP_MS = 15_000;

/**
 * Every "come and play" the app can receive, in one queue.
 *
 * Four sources feed it: the personal socket (a circle member's table, a
 * revoke, the waiting room seating this player), OS pushes that arrive while
 * the app is in front, and — on a phone — tables in the room found over
 * Wi-Fi and Bluetooth. Each speaks its own dialect; by the time anything
 * reaches the queue it is an `Invite`, and the banner, the pill and the home
 * screen's list read nothing else. The rules for which copy wins and how long
 * "Not now" lasts are in `queue.ts`, as plain functions, so they can be
 * tested without this component.
 *
 * Mounted inside SessionProvider, above the Stack, so it outlives every
 * screen: an invite arriving mid-navigation is not lost with the screen that
 * happened to be open.
 */
export function InviteProvider({ children }: { children: React.ReactNode }) {
  const { onlineSession, offline, joinNearby, joinBluetooth, leaveOffline } = useSession();
  const pathname = usePathname() ?? '';
  const inMatch = pathname.startsWith('/match/');
  const appActive = useAppActive();
  const nearbyFlag = useDeviceFlag('nearby');

  const [queue, setQueue] = useState<QueueState>(EMPTY_QUEUE);
  const [joiningId, setJoiningId] = useState('');
  const [error, setError] = useState('');
  const [circleRequests, setCircleRequests] = useState(0);
  const [circleVersion, setCircleVersion] = useState(0);

  // Who the online side is for. The personal socket follows it, and a
  // change of player empties the queue: one player's invitations are not
  // the next one's.
  const identity = onlineSession ? subjectKeyForSession(onlineSession) : null;
  useEffect(() => {
    setQueue(EMPTY_QUEUE);
  }, [identity]);

  const inMatchRef = useRef(inMatch);
  inMatchRef.current = inMatch;
  const pathRef = useRef(pathname);
  pathRef.current = pathname;

  const refreshCircle = useCallback(() => {
    setCircleVersion((v) => v + 1);
  }, []);

  // The badge. Read on sign-in and whenever the server says the circle moved.
  useEffect(() => {
    if (!identity) {
      setCircleRequests(0);
      return;
    }
    let live = true;
    apiClient
      .getCircle()
      .then((c) => live && setCircleRequests(c.requests.length))
      .catch(() => {
        /* an older server, or offline — no badge is the honest answer */
      });
    return () => {
      live = false;
    };
  }, [identity, circleVersion]);

  // The waiting room's invite arrives twice — on the waiting-room socket and
  // on the personal one — and following it twice would replace the screen
  // it just opened.
  const lastSeated = useRef({ matchId: '', at: 0 });

  const receiveSeated = useCallback((matchId: string, joinCode: string) => {
    if (!matchId) return;
    const now0 = Date.now();
    if (lastSeated.current.matchId === matchId && now0 - lastSeated.current.at < 30_000) return;
    lastSeated.current = { matchId, at: now0 };
    // Already seated, so going there is right — unless the player is in the
    // middle of another game, which a navigation would yank them out of.
    // Then it waits in the queue like any other invite.
    if (!inMatchRef.current) {
      router.replace(`/lobby/join?matchId=${encodeURIComponent(matchId)}`);
      return;
    }
    const now = Date.now();
    setQueue((q) =>
      receive(
        q,
        {
          id: matchId,
          source: 'waiting-room',
          host: { name: '' },
          target: { kind: 'online', matchId, joinCode, seated: true },
          receivedAt: now,
          seenAt: now,
        },
        now,
      ),
    );
  }, []);

  const onMessage = useCallback(
    (msg: MeWSMessage) => {
      const now = Date.now();
      switch (msg.type) {
        case 'table_invite':
          // Not about the table on screen: the player is already there.
          if (msg.invite?.matchId && pathRef.current.includes(msg.invite.matchId)) return;
          if (msg.invite?.matchId) setQueue((q) => receive(q, inviteFromWire(msg.invite, now), now));
          return;
        case 'invite_revoked':
          setQueue((q) => revoke(q, msg.id, now));
          return;
        case 'lobby_invited':
          receiveSeated(msg.matchId, msg.joinCode);
          return;
        case 'circle_changed':
          refreshCircle();
          return;
      }
    },
    [receiveSeated, refreshCircle],
  );

  // Closed while a phone's app is in the background, where the OS push is
  // the channel. A browser tab is different: a hidden tab is still the app,
  // and in a browser that shows no notifications its title count is the only
  // way a player who switched tabs learns a table arrived.
  const socketLive = Platform.OS === 'web' || appActive;
  useUserSocket(identity && socketLive ? identity : null, onMessage);

  // OS pushes. Registered again on every sign-in — never asking, only
  // refreshing a grant the player already gave — so the server always holds
  // this device against whoever is signed in on it now.
  useEffect(() => {
    if (identity) void syncPushRegistration();
  }, [identity]);
  useEffect(
    () =>
      startPushListeners({
        onMessage,
        // A tap that launched the app is handled during the root layout's
        // first effects; the next tick lets the navigator finish mounting.
        onOpen: (path) => setTimeout(() => router.push(path as Parameters<typeof router.push>[0]), 0),
      }),
    [onMessage],
  );

  const watcher = useNearbyWatcher(
    nearbyAvailable && appActive && nearbyFlag === true,
    { offlineInstanceId: offline?.instanceId ?? null },
    {
      found: (invite) => setQueue((q) => receive(q, invite, Date.now())),
      lostOnWifi: (id) => setQueue((q) => lostOnWifi(q, id)),
    },
  );

  useEffect(() => {
    const timer = setInterval(() => setQueue((q) => expire(q, Date.now())), SWEEP_MS);
    return () => clearInterval(timer);
  }, []);

  useEffect(() => {
    if (Platform.OS === 'web') return showUnreadInTitle(queue.invites.length);
    return undefined;
  }, [queue.invites.length]);

  const dismiss = useCallback((id: string) => setQueue((q) => dismissInQueue(q, id, Date.now())), []);
  const shelve = useCallback((id: string) => setQueue((q) => shelveInQueue(q, id)), []);

  const queueRef = useRef(queue);
  queueRef.current = queue;

  const muteHost = useCallback(async (id: string) => {
    const invite = queueRef.current.invites.find((i) => i.id === id);
    const key = invite?.host.key;
    if (!key) {
      // A nearby host has no account behind the banner to mute; "Not now"
      // is the most that can be done, and it is done.
      setQueue((q) => dismissInQueue(q, id, Date.now()));
      return;
    }
    setQueue((q) => dismissHost(q, key, Date.now()));
    try {
      await apiClient.muteNotifier(key, true);
    } catch (e) {
      setError(formatApiError(e));
    }
  }, []);

  const join = useCallback(
    async (id: string) => {
      const invite = queueRef.current.invites.find((i) => i.id === id);
      if (!invite) return;
      setError('');
      // A phone hosting its own table cannot walk away from it for another:
      // the guests at it would lose their game. A guest at one can.
      if (offline?.role === 'host') {
        setError(t('notify.hostingOffline'));
        return;
      }
      setJoiningId(id);
      watcher.pauseBle();
      try {
        if (offline) await leaveOffline();
        await followInvite(invite, {
          joinNearby,
          joinBluetooth,
          nearbyName: async () =>
            (await loadOfflineName()) || onlineSession?.username || guestNameFor(await loadGuestId()),
          navigate: (href) => router.push(href as Parameters<typeof router.push>[0]),
        });
        setQueue((q) => remove(q, id));
      } catch (e) {
        // The same words the Offline screen uses for the same two failures.
        if (e instanceof NearbyVersionError) setError(t('offline.versionMismatch'));
        else if (e instanceof BleHostKeyChanged) setError(t('offline.keyChanged'));
        else setError(t('notify.joinFailed', { reason: formatApiError(e) }));
      } finally {
        setJoiningId('');
        watcher.resumeBle();
      }
    },
    [offline, leaveOffline, joinNearby, joinBluetooth, onlineSession?.username, watcher],
  );

  const value = useMemo<InviteContextValue>(
    () => ({
      invites: queue.invites,
      banner: bannerInvite(queue),
      inMatch,
      join,
      dismiss,
      muteHost,
      shelve,
      receiveSeated,
      joiningId,
      error,
      clearError: () => setError(''),
      circleRequests,
      circleVersion,
      refreshCircle,
    }),
    [queue, inMatch, join, dismiss, muteHost, shelve, receiveSeated, joiningId, error, circleRequests, circleVersion, refreshCircle],
  );

  return <InviteContext.Provider value={value}>{children}</InviteContext.Provider>;
}

export function useInvites(): InviteContextValue {
  const ctx = useContext(InviteContext);
  if (!ctx) throw new Error('useInvites must be used within InviteProvider');
  return ctx;
}
