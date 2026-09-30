import type { PushApi } from '@/src/notify/pushTypes';

/**
 * The type-checker's view of push, and the answer on any platform without
 * one of its own.
 *
 * Metro picks `push.native.ts` on iOS and Android and `push.web.ts` in the
 * browser, so this file only ever runs where neither applies. Both of those
 * are checked against `PushApi` with `satisfies`, which is what keeps the
 * three in step.
 */
export const pushStatus: PushApi['pushStatus'] = async () => 'unsupported';
export const enablePush: PushApi['enablePush'] = async () => 'unsupported';
export const syncPushRegistration: PushApi['syncPushRegistration'] = async () => {};
export const startPushListeners: PushApi['startPushListeners'] = () => () => {};
