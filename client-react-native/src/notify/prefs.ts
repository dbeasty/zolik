import { useEffect, useState } from 'react';

import { storage } from '@/src/context/SessionContext';

/**
 * The notification choices that belong to this device rather than to the
 * player's account.
 *
 * "Show tables nearby" is about this phone's radios, and "tell my circle when
 * I open a table" about how this person hosts from here, so both live in
 * device storage the way the language override does (`localeStore.ts`). They
 * are also mirrored to the server where it has a field for them, but the
 * device's copy is the one the watcher obeys: it has to be right with no
 * network at all, which is when nearby play happens.
 *
 * Held in memory as well, with listeners, because a switch flipped in
 * settings has to stop a watcher mounted at the root of the app, and neither
 * one renders the other.
 */

/**
 * - `nearby`: show a banner for tables other phones in the room are hosting.
 * - `announce`: tell my circle when I open a table online.
 * - `tellNearby`: let the room in (Wi-Fi) as soon as I host an offline table.
 */
export type DeviceFlag = 'nearby' | 'announce' | 'tellNearby';

const KEYS: Record<DeviceFlag, string> = {
  nearby: 'zolik_notify_nearby',
  announce: 'zolik_notify_announce',
  tellNearby: 'zolik_notify_tell_nearby',
};

/** All on until the player says otherwise: the point of the feature is that
 *  nobody has to go and find it. */
const DEFAULTS: Record<DeviceFlag, boolean> = { nearby: true, announce: true, tellNearby: true };

const cache: Partial<Record<DeviceFlag, boolean>> = {};
const listeners = new Set<() => void>();

export async function loadFlag(flag: DeviceFlag): Promise<boolean> {
  if (cache[flag] !== undefined) return cache[flag]!;
  let value = DEFAULTS[flag];
  try {
    const raw = await storage.getItem(KEYS[flag]);
    if (raw === '0') value = false;
    else if (raw === '1') value = true;
  } catch {
    // Unreadable storage means the default, not off.
  }
  cache[flag] = value;
  return value;
}

export async function saveFlag(flag: DeviceFlag, value: boolean): Promise<void> {
  cache[flag] = value;
  for (const l of listeners) l();
  try {
    await storage.setItem(KEYS[flag], value ? '1' : '0');
  } catch {
    // It holds for this run; losing it on restart is a small thing.
  }
}

/** The flag, live. `null` until storage has been read. */
export function useDeviceFlag(flag: DeviceFlag): boolean | null {
  const [value, setValue] = useState<boolean | null>(cache[flag] ?? null);
  useEffect(() => {
    let live = true;
    const read = () => void loadFlag(flag).then((v) => live && setValue(v));
    read();
    listeners.add(read);
    return () => {
      live = false;
      listeners.delete(read);
    };
  }, [flag]);
  return value;
}

// --- the pre-prompt's "Later" ----------------------------------------------

const LATER_KEY = 'zolik_push_later';

/** How long "Later" keeps the card away. Long enough not to nag, short enough
 *  that someone who meant "not during this game" is asked again. */
export const PUSH_LATER_MS = 14 * 24 * 60 * 60 * 1000;

export async function pushPromptSnoozed(now = Date.now()): Promise<boolean> {
  try {
    const at = Number(await storage.getItem(LATER_KEY));
    return Number.isFinite(at) && at > 0 && now - at < PUSH_LATER_MS;
  } catch {
    return false;
  }
}

export async function snoozePushPrompt(now = Date.now()): Promise<void> {
  try {
    await storage.setItem(LATER_KEY, String(now));
  } catch {
    // Asked again sooner than promised; nothing worse.
  }
}
