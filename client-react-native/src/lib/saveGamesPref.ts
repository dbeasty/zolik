import { useEffect, useState } from 'react';

import { storage } from '@/src/context/SessionContext';

/**
 * What this phone does with a game it hosted once it ends: ask its owner
 * (the default), save it to their account without asking, or never offer.
 *
 * Only the owner's own seat is decided by this. Every other player at the
 * table is always asked, on their own screen, and a seat nobody said yes for
 * reaches the cloud as an anonymous "Player N" - see match.LocalSaves.
 */
export type SaveGamesMode = 'ask' | 'always' | 'never';

const KEY = 'zolik_save_games';
const listeners = new Set<(m: SaveGamesMode) => void>();

export async function loadSaveGamesMode(): Promise<SaveGamesMode> {
  try {
    const v = await storage.getItem(KEY);
    return v === 'always' || v === 'never' ? v : 'ask';
  } catch {
    return 'ask';
  }
}

export async function setSaveGamesMode(mode: SaveGamesMode): Promise<void> {
  await storage.setItem(KEY, mode);
  listeners.forEach((fn) => fn(mode));
}

/** The current mode, or null until it has been read. */
export function useSaveGamesMode(): SaveGamesMode | null {
  const [mode, setMode] = useState<SaveGamesMode | null>(null);
  useEffect(() => {
    let live = true;
    void loadSaveGamesMode().then((m) => live && setMode(m));
    listeners.add(setMode);
    return () => {
      live = false;
      listeners.delete(setMode);
    };
  }, []);
  return mode;
}
