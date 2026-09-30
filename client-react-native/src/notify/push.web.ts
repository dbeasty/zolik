import { apiClient } from '@/src/api/client';
import { logger } from '@/src/lib/logger';
import { registerDevice } from '@/src/notify/pushDevice';
import { messageFromPush } from '@/src/notify/pushPayload';
import type { PushApi, PushStatus } from '@/src/notify/pushTypes';
import { pathFromPushUrl } from '@/src/notify/routing';

/**
 * Web Push, through a service worker at `/sw.js` (`public/sw.js`, which the
 * static export copies to the site root — the root is the only place a worker
 * can control every page from).
 *
 * The worker decides who shows a push. With a tab of this site in front it
 * posts the push to that tab and the in-app banner shows it, because an OS
 * notification about the page you are looking at is noise; otherwise it shows
 * a notification tagged `invite:<matchId>`, which a later revoke closes.
 */

const SW_PATH = '/sw.js';

type WorkerNote =
  | { source: 'zolik-sw'; kind: 'push'; payload: unknown }
  | { source: 'zolik-sw'; kind: 'open'; url: string; payload?: unknown };

function hasWindow(): boolean {
  return typeof window !== 'undefined' && typeof navigator !== 'undefined';
}

function supported(): boolean {
  return (
    hasWindow() &&
    'serviceWorker' in navigator &&
    'PushManager' in window &&
    typeof Notification !== 'undefined'
  );
}

/**
 * iOS Safari keeps web push for sites added to the home screen, and hides
 * `PushManager` from a plain tab. Worth telling apart from "unsupported",
 * because this one the player can fix.
 */
function iosNeedsInstall(): boolean {
  if (!hasWindow()) return false;
  const ios = /iPad|iPhone|iPod/.test(navigator.userAgent);
  const standalone =
    window.matchMedia?.('(display-mode: standalone)').matches ||
    (navigator as Navigator & { standalone?: boolean }).standalone === true;
  return ios && !standalone;
}

function permission(): PushStatus {
  const p = Notification.permission;
  return p === 'granted' ? 'granted' : p === 'denied' ? 'denied' : 'default';
}

/** VAPID keys travel base64url-encoded; `subscribe` wants the raw bytes. */
function keyBytes(base64url: string): Uint8Array<ArrayBuffer> {
  const padded = base64url.replace(/-/g, '+').replace(/_/g, '/') + '='.repeat((4 - (base64url.length % 4)) % 4);
  const raw = atob(padded);
  const out = new Uint8Array(new ArrayBuffer(raw.length));
  for (let i = 0; i < raw.length; i++) out[i] = raw.charCodeAt(i);
  return out;
}

let registration: Promise<ServiceWorkerRegistration> | null = null;

function worker(): Promise<ServiceWorkerRegistration> {
  registration ??= navigator.serviceWorker.register(SW_PATH, { scope: '/' });
  return registration;
}

async function subscribeAndRegister(): Promise<PushStatus> {
  const config = await apiClient.getNotifyConfig();
  if (!config.vapidPublicKey) return 'unavailable';
  const reg = await worker();
  let sub = await reg.pushManager.getSubscription();
  if (!sub) {
    sub = await reg.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: keyBytes(config.vapidPublicKey),
    });
  }
  await registerDevice({ kind: 'webpush', subscription: sub.toJSON(), platform: 'web' });
  return 'granted';
}

export const pushStatus: PushApi['pushStatus'] = async () => {
  if (!supported()) return iosNeedsInstall() ? 'install' : 'unsupported';
  return permission();
};

export const enablePush: PushApi['enablePush'] = async () => {
  if (!supported()) return iosNeedsInstall() ? 'install' : 'unsupported';
  try {
    const answer = await Notification.requestPermission();
    if (answer !== 'granted') return answer === 'denied' ? 'denied' : 'default';
    return await subscribeAndRegister();
  } catch (e) {
    logger.warn('push', 'enable failed', { error: e instanceof Error ? e.message : String(e) });
    return 'unavailable';
  }
};

export const syncPushRegistration: PushApi['syncPushRegistration'] = async () => {
  if (!supported() || permission() !== 'granted') return;
  try {
    await subscribeAndRegister();
  } catch (e) {
    logger.warn('push', 'register failed', { error: e instanceof Error ? e.message : String(e) });
  }
};

export const startPushListeners: PushApi['startPushListeners'] = (handlers) => {
  if (!supported()) return () => {};
  const onMessage = (event: MessageEvent) => {
    const note = event.data as WorkerNote | undefined;
    if (!note || note.source !== 'zolik-sw') return;
    const msg = messageFromPush(note.payload);
    if (msg) handlers.onMessage(msg);
    if (note.kind === 'open') {
      const path = pathFromPushUrl(note.url);
      if (path) handlers.onOpen(path);
    }
  };
  navigator.serviceWorker.addEventListener('message', onMessage);
  // Registered as soon as anything listens, so an already-granted browser's
  // worker is in place before the first push rather than after a visit to
  // settings. Registration asks the player nothing.
  if (permission() === 'granted') void worker().catch(() => {});
  return () => navigator.serviceWorker.removeEventListener('message', onMessage);
};
