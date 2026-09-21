import Constants from 'expo-constants';
import * as Notifications from 'expo-notifications';
import { Platform } from 'react-native';

import { apiClient } from '@/src/api/client';
import { t } from '@/src/lib/i18n';
import { logger } from '@/src/lib/logger';
import { registerDevice } from '@/src/notify/pushDevice';
import { messageFromPush } from '@/src/notify/pushPayload';
import type { PushApi, PushStatus } from '@/src/notify/pushTypes';
import { pathFromPushUrl } from '@/src/notify/routing';

/**
 * OS push on iOS and Android, through Expo's push service.
 *
 * The server sends a push to every registered device whether or not an app is
 * open, because tracking who is looking at what is exactly the kind of state
 * that goes stale. So the app's half of the bargain is here: a push arriving
 * while the app is in front is *not* shown by the OS — the handler below
 * swallows it and hands it to the invite banner instead, which the socket has
 * usually already filled with the same invite (same id, so it is one).
 */

/** The Android channel the server names in every push (`channelId`). */
const CHANNEL = 'invites';

/** The EAS project the Expo push token is minted for. Without one there is
 *  no Expo push to register with — a dev build with no EAS project, say. */
function projectId(): string | undefined {
  const fromExtra = (Constants.expoConfig?.extra as { eas?: { projectId?: string } } | undefined)?.eas
    ?.projectId;
  return fromExtra ?? Constants.easConfig?.projectId ?? undefined;
}

function statusOf(p: Notifications.NotificationPermissionsStatus): PushStatus {
  // iOS reports provisional and ephemeral grants apart from "authorized";
  // both deliver, so both count.
  const ios = p.ios?.status;
  if (
    ios === Notifications.IosAuthorizationStatus.PROVISIONAL ||
    ios === Notifications.IosAuthorizationStatus.EPHEMERAL
  ) {
    return 'granted';
  }
  if (p.granted) return 'granted';
  return p.status === 'denied' ? 'denied' : 'default';
}

async function ensureChannel(): Promise<void> {
  if (Platform.OS !== 'android') return;
  await Notifications.setNotificationChannelAsync(CHANNEL, {
    name: t('notify.channelName'),
    importance: Notifications.AndroidImportance.HIGH,
  });
}

async function register(): Promise<PushStatus> {
  const id = projectId();
  if (!id) return 'unavailable';
  let config;
  try {
    config = await apiClient.getNotifyConfig();
  } catch {
    // Not knowing is not the same as "not configured": try anyway, and let
    // the registration itself be the answer.
    config = { expo: true, vapidPublicKey: null };
  }
  if (!config.expo) return 'unavailable';
  await ensureChannel();
  const token = await Notifications.getExpoPushTokenAsync({ projectId: id });
  await registerDevice({ kind: 'expo', token: token.data, platform: Platform.OS });
  return 'granted';
}

export const pushStatus: PushApi['pushStatus'] = async () => {
  try {
    const status = statusOf(await Notifications.getPermissionsAsync());
    if (status === 'granted' && !projectId()) return 'unavailable';
    return status;
  } catch {
    return 'unsupported';
  }
};

export const enablePush: PushApi['enablePush'] = async () => {
  try {
    // The channel has to exist before Android 13 will show its prompt.
    await ensureChannel();
    const status = statusOf(await Notifications.requestPermissionsAsync());
    if (status !== 'granted') return status;
    return await register();
  } catch (e) {
    logger.warn('push', 'enable failed', { error: e instanceof Error ? e.message : String(e) });
    return 'unavailable';
  }
};

export const syncPushRegistration: PushApi['syncPushRegistration'] = async () => {
  try {
    if (statusOf(await Notifications.getPermissionsAsync()) !== 'granted') return;
    await register();
  } catch (e) {
    logger.warn('push', 'register failed', { error: e instanceof Error ? e.message : String(e) });
  }
};

/** Takes down any notification in the tray about that table. */
async function dismissShown(inviteId: string): Promise<void> {
  try {
    for (const n of await Notifications.getPresentedNotificationsAsync()) {
      const data = n.request.content.data as { invite?: { id?: string; matchId?: string } } | undefined;
      if (data?.invite?.id === inviteId || data?.invite?.matchId === inviteId) {
        await Notifications.dismissNotificationAsync(n.request.identifier);
      }
    }
  } catch {
    // A notification left in the tray fails safe: its Join lands on the
    // join screen's "that table is gone".
  }
}

/** Responses already acted on, so the cold-start one is not followed twice. */
const handled = new Set<string>();

export const startPushListeners: PushApi['startPushListeners'] = (handlers) => {
  Notifications.setNotificationHandler({
    handleNotification: async (notification) => {
      const msg = messageFromPush(notification.request.content.data);
      if (msg) {
        // In front: the banner shows it, the OS does not.
        handlers.onMessage(msg);
        return { shouldShowBanner: false, shouldShowList: false, shouldPlaySound: false, shouldSetBadge: false };
      }
      return { shouldShowBanner: true, shouldShowList: true, shouldPlaySound: false, shouldSetBadge: false };
    },
  });

  const follow = (response: Notifications.NotificationResponse | null) => {
    if (!response) return;
    const key = response.notification.request.identifier;
    if (handled.has(key)) return;
    handled.add(key);
    const data = response.notification.request.content.data as Record<string, unknown> | undefined;
    const msg = messageFromPush(data);
    // Seen through the banner's eyes as well as opened, so the invite is in
    // the queue — and the home screen's list — whatever the route does.
    if (msg) handlers.onMessage(msg);
    const path = pathFromPushUrl(data?.url);
    if (path) handlers.onOpen(path);
  };

  // A tap that launched the app arrived before anything was listening.
  follow(Notifications.getLastNotificationResponse());
  const sub = Notifications.addNotificationResponseReceivedListener(follow);

  // A revoke is data-only: nothing for the OS to show, and nothing reaches
  // the handler above on every platform. Heard here instead, it takes the
  // table off the banner and takes down the notification that announced it,
  // which would otherwise sit in the tray offering a table that is gone.
  const received = Notifications.addNotificationReceivedListener((n) => {
    const msg = messageFromPush(n.request.content.data);
    if (msg?.type !== 'invite_revoked') return;
    handlers.onMessage(msg);
    void dismissShown(msg.id);
  });

  return () => {
    sub.remove();
    received.remove();
    Notifications.setNotificationHandler(null);
  };
};
