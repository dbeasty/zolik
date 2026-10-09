import * as Notifications from 'expo-notifications';

/**
 * This device's own notifications, for news the app hears while it is not in
 * front: somebody sat down at a table the player is at.
 *
 * Imported statically, here and only here, like push.native.ts: a dynamic
 * import of expo-notifications does not resolve in a native bundle, and a
 * failure there was swallowed - the permission was never asked for.
 */

/** Asks once whether the app may notify; a refusal is the OS's to remember. */
export async function askToNotify(): Promise<void> {
  try {
    const now = await Notifications.getPermissionsAsync();
    if (now.granted || !now.canAskAgain) return;
    await Notifications.requestPermissionsAsync();
  } catch {
    // No notifications on this build: the in-app banner still says it.
  }
}

/** Shows a notification now, if the player allowed them. */
export async function notifyNow(id: string, title: string, body: string, url: string): Promise<void> {
  try {
    const perm = await Notifications.getPermissionsAsync();
    if (!perm.granted) return;
    await Notifications.scheduleNotificationAsync({
      identifier: id,
      content: { title, body, data: { url } },
      trigger: null,
    });
  } catch {
    // As above.
  }
}
