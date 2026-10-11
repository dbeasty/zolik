import { storage } from '@/src/context/SessionContext';

import { START, type Progress } from './progress';

/**
 * What this device remembers about guided lessons.
 *
 * Per device, like `introStore`: the guide is for the person at this screen,
 * and a guest who never signs in should still be able to put it down and pick
 * it up. Every read and write is non-fatal — storage can be refused outright
 * in a private window, and the worst that costs is starting a lesson over.
 */

const progressKey = (matchId: string) => `zolik_guide_progress_${matchId}`;
const tableKey = (moduleId: string, lessonId: string) => `zolik_guide_table_${moduleId}_${lessonId}`;
const DONE_KEY = 'zolik_guide_done';

export async function loadProgress(matchId: string): Promise<Progress> {
  try {
    const raw = await storage.getItem(progressKey(matchId));
    if (!raw) return START;
    const p = JSON.parse(raw) as Partial<Progress>;
    if (typeof p.idx === 'number' && typeof p.done === 'number' && p.idx >= 0 && p.done >= 0) {
      return { idx: p.idx, done: p.done };
    }
  } catch {
    // Fall through: start from the top.
  }
  return START;
}

export async function saveProgress(matchId: string, p: Progress): Promise<void> {
  try {
    await storage.setItem(progressKey(matchId), JSON.stringify(p));
  } catch {
    // Nothing worth telling anyone.
  }
}

/** The tutorial table last dealt for this lesson, so "Continue" finds it again. */
export async function rememberTable(moduleId: string, lessonId: string, matchId: string): Promise<void> {
  try {
    await storage.setItem(tableKey(moduleId, lessonId), matchId);
  } catch {
    // As above.
  }
}

export async function recallTable(moduleId: string, lessonId: string): Promise<string | null> {
  try {
    return await storage.getItem(tableKey(moduleId, lessonId));
  } catch {
    return null;
  }
}

export async function forgetTable(moduleId: string, lessonId: string): Promise<void> {
  try {
    await storage.deleteItem(tableKey(moduleId, lessonId));
  } catch {
    // As above.
  }
}

/** Lessons finished on this device, as `module:lesson`. */
export async function loadDone(): Promise<Set<string>> {
  try {
    const raw = await storage.getItem(DONE_KEY);
    if (raw) return new Set(JSON.parse(raw) as string[]);
  } catch {
    // Treated as none done.
  }
  return new Set();
}

export async function markDone(moduleId: string, lessonId: string): Promise<void> {
  try {
    const done = await loadDone();
    done.add(`${moduleId}:${lessonId}`);
    await storage.setItem(DONE_KEY, JSON.stringify([...done]));
  } catch {
    // As above.
  }
}
