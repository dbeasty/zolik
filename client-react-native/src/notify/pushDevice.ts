import { apiClient } from '@/src/api/client';
import type { PushDeviceRegistration } from '@/src/api/types';
import { storage } from '@/src/context/SessionContext';
import { getLocale } from '@/src/lib/i18n';

/**
 * The server's id for this device's push registration, remembered so that
 * signing out can take it back.
 *
 * Registering again is harmless — the server derives the id from the token
 * or endpoint and updates one row — so this is called on every launch, which
 * also keeps the locale the push text is written in current.
 */
const KEY = 'zolik_push_device';

export async function registerDevice(
  device: Omit<PushDeviceRegistration, 'locale'>,
): Promise<void> {
  if (!apiClient.accessToken) return;
  const { id } = await apiClient.registerPushDevice({ ...device, locale: getLocale() });
  try {
    await storage.setItem(KEY, id);
  } catch {
    // Only matters for unregistering later, and the server forgets dead
    // tokens on its own.
  }
}

/** Takes this device off the account it was registered to, if it was. */
export async function unregisterDevice(): Promise<void> {
  try {
    const id = await storage.getItem(KEY);
    if (!id) return;
    await storage.deleteItem(KEY);
    if (apiClient.accessToken) await apiClient.unregisterPushDevice(id);
  } catch {
    // A device the server still has will simply be re-keyed or pruned.
  }
}
