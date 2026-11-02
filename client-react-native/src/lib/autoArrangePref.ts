import { useCallback, useEffect, useState } from 'react';

import { storage } from '@/src/context/SessionContext';

/**
 * Whether this device keeps every hand auto-arranged.
 *
 * A standing choice rather than a one-off tidy: while it is on, the hand is put
 * back in order after every push, so a drawn card lands among its meld
 * material instead of at the end. Once for every game rather than per game —
 * the arrangement itself already knows each deck (see `arrangeAuto`).
 *
 * A device preference, never sent anywhere, for the same reason the
 * arrangement it drives never is. Off by default: hands have always been left
 * in the order they arrived, and a preference nobody set must not change that.
 * Storage failure degrades to off.
 */

const KEY = 'zolik_auto_arrange';
const listeners = new Set<(on: boolean) => void>();
let cached: boolean | null = null;

async function load(): Promise<boolean> {
  if (cached !== null) return cached;
  try {
    cached = (await storage.getItem(KEY)) === '1';
  } catch {
    cached = false;
  }
  return cached;
}

export async function setAutoArrange(on: boolean): Promise<void> {
  cached = on;
  listeners.forEach((fn) => fn(on));
  try {
    await storage.setItem(KEY, on ? '1' : '0');
  } catch {
    // Kept for this session; it simply will not survive a reload.
  }
}

/** This device's choice, and a setter for it. */
export function useAutoArrange(): [boolean, (on: boolean) => void] {
  const [on, setOn] = useState(cached ?? false);
  useEffect(() => {
    let live = true;
    void load().then((v) => live && setOn(v));
    listeners.add(setOn);
    return () => {
      live = false;
      listeners.delete(setOn);
    };
  }, []);
  const set = useCallback((next: boolean) => void setAutoArrange(next), []);
  return [on, set];
}
