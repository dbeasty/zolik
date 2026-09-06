import { storage } from '@/src/context/SessionContext';

/**
 * What a host picked last time they set up a module: its variation, the
 * options chosen within it, and how many bots to seat. A preference about how
 * to play, not a fact about any game in progress, so it never leaves the
 * device and no module knows it exists. Keyed per module because a meld
 * minimum chosen for one game says nothing about another.
 */

const KEY = 'zolik_game_setup';

export type SavedGameSetup = {
  variation?: string;
  options?: Record<string, number>;
  bots?: number;
};

async function read(): Promise<Record<string, SavedGameSetup>> {
  try {
    const raw = await storage.getItem(KEY);
    if (!raw) return {};
    const parsed = JSON.parse(raw);
    return parsed && typeof parsed === 'object' ? (parsed as Record<string, SavedGameSetup>) : {};
  } catch {
    // Unreadable or from an older shape. A lost preference just means the
    // module's own defaults are used again, same as a first visit.
    return {};
  }
}

export async function loadGameSetup(moduleId: string): Promise<SavedGameSetup | undefined> {
  const all = await read();
  return all[moduleId];
}

export async function saveGameSetup(moduleId: string, setup: SavedGameSetup): Promise<void> {
  try {
    const all = await read();
    all[moduleId] = setup;
    await storage.setItem(KEY, JSON.stringify(all));
  } catch {
    // Storage can be full, or refused outright in a private window. Losing a
    // preference is a small thing; interrupting a game over it is not.
  }
}
