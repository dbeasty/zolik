import { useSyncExternalStore } from 'react';

import { storage } from '@/src/context/SessionContext';

/**
 * The tour of the app itself: what each screen is for, walked over the real
 * screens rather than a picture of them.
 *
 * It needs no table and no server — a step names a screen and says what that
 * screen is for, and the banner at the top of that screen says it. Moving on
 * takes the player to the next step's screen. Leaving is one press, on every
 * step, and forgets nothing the player did in between.
 */

/** The game the tour uses as its example: any game would do, this one is short. */
export const TOUR_GAME = 'prsi';

export type TourStep = {
  id: string;
  /** The screen this step is told on (an expo-router pathname). */
  path: string;
  /** Where Next takes the player to *reach* this step. */
  href: string;
  textKey: string;
};

export const TOUR_STEPS: TourStep[] = [
  { id: 'home', path: '/', href: '/', textKey: 'tour.home' },
  { id: 'game', path: '/lobby/games', href: `/lobby/games?moduleId=${TOUR_GAME}`, textKey: 'tour.game' },
  { id: 'setup', path: '/lobby/setup', href: `/lobby/setup?moduleId=${TOUR_GAME}&mode=bots`, textKey: 'tour.setup' },
  { id: 'learn', path: '/lobby/games', href: `/lobby/games?moduleId=${TOUR_GAME}`, textKey: 'tour.learn' },
  { id: 'more', path: '/more', href: '/more', textKey: 'tour.more' },
  { id: 'settings', path: '/settings', href: '/settings', textKey: 'tour.settings' },
];

export type TourState = { active: boolean; idx: number };

const KEY = 'zolik_tour';
let state: TourState = { active: false, idx: 0 };
let loaded = false;
const listeners = new Set<() => void>();

function set(next: TourState): void {
  state = next;
  listeners.forEach((l) => l());
  void storage.setItem(KEY, JSON.stringify(next)).catch(() => undefined);
}

async function load(): Promise<void> {
  if (loaded) return;
  loaded = true;
  try {
    const raw = await storage.getItem(KEY);
    if (!raw) return;
    const p = JSON.parse(raw) as Partial<TourState>;
    if (p.active && typeof p.idx === 'number' && p.idx >= 0 && p.idx < TOUR_STEPS.length) {
      state = { active: true, idx: p.idx };
      listeners.forEach((l) => l());
    }
  } catch {
    // A tour that cannot be remembered simply is not resumed.
  }
}

export function startTour(): TourStep {
  set({ active: true, idx: 0 });
  return TOUR_STEPS[0];
}

export function endTour(): void {
  set({ active: false, idx: 0 });
}

/** Moves to the neighbouring step and returns it, or null when the tour is over. */
export function stepTour(by: 1 | -1): TourStep | null {
  const idx = state.idx + by;
  if (idx < 0) return TOUR_STEPS[0];
  if (idx >= TOUR_STEPS.length) {
    endTour();
    return null;
  }
  set({ active: true, idx });
  return TOUR_STEPS[idx];
}

export function useTour(): TourState {
  void load();
  return useSyncExternalStore(
    (cb) => {
      listeners.add(cb);
      return () => listeners.delete(cb);
    },
    () => state,
    () => state,
  );
}

/** The step to show on this screen, or null: the tour is off, or is told elsewhere. */
export function stepFor(s: TourState, pathname: string): TourStep | null {
  if (!s.active) return null;
  const step = TOUR_STEPS[s.idx];
  return step && step.path === pathname ? step : null;
}
