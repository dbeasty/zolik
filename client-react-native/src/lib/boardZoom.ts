import { useEffect, useState } from 'react';

import { storage } from '@/src/context/SessionContext';
import { metricsFor, type CardSize, type ScreenShape } from '@/src/lib/layout';

/**
 * How far the player has zoomed the board — the − / + beside Layout.
 *
 * A multiple of the scale the window and the card size already give, applied
 * in `metricsFor` like the card size and for the same reason: everything the
 * scale feeds (slot pitch, fan peek, meld cards, panel chrome, drop gaps)
 * moves with it, so a zoomed board is the same board measured the same way.
 * A CSS transform would draw it bigger but leave every hit test and every
 * drop gap where the unzoomed board had them.
 *
 * Only the board: the match screen wraps its own scroll view in
 * `BoardMetricsProvider`, and the rest of the app keeps its size.
 *
 * Once for the device rather than per game. Zoom answers "how big is this
 * screen, how good are my eyes", which is the same answer at every table.
 */

/** The steps − and + walk through; 1 is the board as it always was. */
export const ZOOM_STEPS = [0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1, 1.1, 1.25, 1.4, 1.6] as const;

const KEY = 'zolik_board_zoom';

/** The nearest step to a stored value, so a step list that changes never strands a player off it. */
export function nearestStep(z: unknown): number {
  if (typeof z !== 'number' || !Number.isFinite(z)) return 1;
  let best: number = ZOOM_STEPS[0];
  for (const s of ZOOM_STEPS) if (Math.abs(s - z) < Math.abs(best - z)) best = s;
  return best;
}

/** One step in, or out; stays put at either end. */
export function stepZoom(z: number, dir: 1 | -1): number {
  const i = ZOOM_STEPS.indexOf(nearestStep(z) as (typeof ZOOM_STEPS)[number]);
  return ZOOM_STEPS[Math.max(0, Math.min(ZOOM_STEPS.length - 1, i + dir))];
}

/**
 * Narrower than this, a card's rank and suit no longer fit on its face: the
 * fonts stop shrinking at 9px long before the card does.
 */
export const MIN_ZOOMED_CARD_WIDTH = 20;

/**
 * The smallest step this window can draw. A wide screen goes all the way to
 * the first step; a phone, whose cards start small, stops sooner.
 */
export function smallestZoom(width: number, cardSize: CardSize = 'normal', screen: ScreenShape = {}): number {
  for (const s of ZOOM_STEPS) if (metricsFor(width, cardSize, s, screen).card.width >= MIN_ZOOMED_CARD_WIDTH) return s;
  return 1;
}

let current = 1;
let loaded = false;
const listeners = new Set<(z: number) => void>();

function publish(z: number) {
  current = z;
  loaded = true;
  listeners.forEach((fn) => fn(current));
}

async function load(): Promise<number> {
  if (loaded) return current;
  try {
    const raw = await storage.getItem(KEY);
    publish(raw == null ? 1 : nearestStep(Number(raw)));
  } catch {
    publish(1);
  }
  return current;
}

export async function setBoardZoom(z: number): Promise<void> {
  publish(nearestStep(z));
  try {
    await storage.setItem(KEY, String(current));
  } catch {
    // A zoom that could not be saved still applies until the app closes.
  }
}

/**
 * One step from the zoom as it is now, not as the last render saw it — two
 * quick presses of + are two steps, though the button has not redrawn between.
 * From no lower than `floor`, which is where the board is really drawn when
 * a zoom saved on a wider window is too small for this one.
 */
export function stepBoardZoom(dir: 1 | -1, floor: number = ZOOM_STEPS[0]): Promise<void> {
  return setBoardZoom(Math.max(floor, stepZoom(Math.max(current, floor), dir)));
}

// Another tab zoomed: follow it, as the accessibility choices do.
if (typeof window !== 'undefined' && typeof window.addEventListener === 'function') {
  window.addEventListener('storage', (e: StorageEvent) => {
    if (e.key !== null && e.key !== KEY) return;
    publish(e.key === null || e.newValue == null ? 1 : nearestStep(Number(e.newValue)));
  });
}

/** The board's zoom, re-rendering whoever reads it when it changes. */
export function useBoardZoom(): number {
  const [z, setZ] = useState(current);
  useEffect(() => {
    listeners.add(setZ);
    void load().then(setZ);
    return () => {
      listeners.delete(setZ);
    };
  }, []);
  return z;
}
