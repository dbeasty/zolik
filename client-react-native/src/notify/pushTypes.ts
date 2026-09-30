import type { MeWSMessage } from '@/src/api/types';

/**
 * Where OS notifications stand on this device.
 *
 * - `unsupported`: this platform or browser has no push at all.
 * - `install`: iOS Safari, which only offers web push to a site added to the
 *   home screen.
 * - `unavailable`: the device could, but this server has no push configured
 *   (no VAPID key, no Expo project) — nothing the player can fix.
 * - `default`: never asked. The only state the pre-prompt card appears in.
 * - `granted` / `denied`: the player's answer to the OS.
 */
export type PushStatus = 'unsupported' | 'install' | 'unavailable' | 'default' | 'granted' | 'denied';

export type PushHandlers = {
  /** A push that arrived while the app was open and in front: shown as the
   *  in-app banner, exactly as the socket's copy would be. */
  onMessage: (msg: MeWSMessage) => void;
  /** A notification the player tapped: the route to open. */
  onOpen: (path: string) => void;
};

/**
 * The one surface both platform implementations offer, so nothing outside
 * `src/notify/push*.ts` knows whether it is talking to Expo or to a service
 * worker.
 */
export type PushApi = {
  pushStatus: () => Promise<PushStatus>;
  /** Asks the OS (call it from a tap) and registers this device. */
  enablePush: () => Promise<PushStatus>;
  /** Registers again if permission is already granted, and does nothing
   *  otherwise. Safe on every launch and every sign-in; never prompts. */
  syncPushRegistration: () => Promise<void>;
  startPushListeners: (handlers: PushHandlers) => () => void;
};
