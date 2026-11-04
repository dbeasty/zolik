import { useCallback, useEffect, useState } from 'react';

import { storage } from '@/src/context/SessionContext';

/**
 * Whether this device keeps the hand and its controls docked at the top of the
 * match screen, per game.
 *
 * Per game because it earns its place where the melds pile up — a Canasta side
 * with a dozen of them, where looking them over scrolls the hand off the top —
 * and costs room for nothing in a game played from one pile.
 *
 * A device preference, never sent anywhere: it changes where this viewer's own
 * hand is drawn, nothing anybody else sees. Off by default, because the hand
 * has always been drawn in the page, and a preference nobody set must not
 * move it. Storage failure degrades to off, as every other store here does.
 */

const KEY = 'zolik_floating_hand';
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

export async function setFloatingHand(moduleId: string, on: boolean): Promise<void> {
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
export function useFloatingHand(moduleId: string | undefined): [boolean, (on: boolean) => void] {
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
  const set = useCallback(
    (on: boolean) => {
      if (moduleId) void setFloatingHand(moduleId, on);
    },
    [moduleId],
  );
  return [!!moduleId && !!all[moduleId], set];
}
