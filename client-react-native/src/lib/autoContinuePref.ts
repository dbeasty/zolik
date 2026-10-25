import { useCallback, useEffect, useState } from 'react';

import { storage } from '@/src/context/SessionContext';

/**
 * Whether this device presses "start the next round" by itself, per game.
 *
 * Per game rather than once for everything: poker is played hand after hand
 * with nothing to decide in between, while a Canasta deal ends in a score sheet
 * worth reading. A player who wants the one dealt straight on usually does not
 * want the other.
 *
 * A device preference, not a table option. It decides only this viewer's own
 * "go on" — every other seat still agrees in its own time — so it is nobody
 * else's business and needs no server. Off by default: the table has always
 * waited for the button, and a preference nobody set must not change that.
 * Storage failure degrades to off, same as every other store here.
 */

const KEY = 'zolik_auto_continue';
const listeners = new Set<(m: Record<string, boolean>) => void>();
let cached: Record<string, boolean> | null = null;

async function load(): Promise<Record<string, boolean>> {
  if (cached) return cached;
  try {
    const raw = await storage.getItem(KEY);
    const parsed = raw ? JSON.parse(raw) : {};
    cached = parsed && typeof parsed === 'object' ? parsed : {};
  } catch {
    cached = {};
  }
  return cached!;
}

export async function setAutoContinue(moduleId: string, on: boolean): Promise<void> {
  const next = { ...(await load()), [moduleId]: on };
  cached = next;
  listeners.forEach((fn) => fn(next));
  try {
    await storage.setItem(KEY, JSON.stringify(next));
  } catch {
    // Kept for this session; it simply will not survive a reload.
  }
}

/** This device's choice for one game, and a setter for it. */
export function useAutoContinue(moduleId: string | undefined): [boolean, (on: boolean) => void] {
  const [all, setAll] = useState<Record<string, boolean>>(cached ?? {});
  useEffect(() => {
    let live = true;
    void load().then((m) => live && setAll(m));
    listeners.add(setAll);
    return () => {
      live = false;
      listeners.delete(setAll);
    };
  }, []);
  const set = useCallback((on: boolean) => {
    if (moduleId) void setAutoContinue(moduleId, on);
  }, [moduleId]);
  return [!!moduleId && !!all[moduleId], set];
}
