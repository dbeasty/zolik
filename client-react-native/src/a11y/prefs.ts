import { useEffect, useState } from 'react';

import { storage } from '@/src/context/SessionContext';

/**
 * The accessibility choices a player makes about this device.
 *
 * Every default is today's behaviour: a player who never opens the section
 * sees the board exactly as it was. The two that follow the operating system
 * instead — motion and contrast — start at `auto`, because a person who has
 * asked their phone for stillness or for contrast has already answered the
 * question once and should not be asked again by every app.
 *
 * Stored as one JSON blob rather than a key per choice: they are read together
 * at startup, and a missing or unknown field falls back to its default
 * without disturbing the others — which is what lets a later build add a
 * choice without a migration.
 */
export type AnnounceLevel = 'all' | 'mine' | 'off';
export type CardSize = 'normal' | 'large' | 'xlarge';
export type AutoOnOff = 'auto' | 'on' | 'off';

export type A11yPrefs = {
  /** What the screen reader is told about the game: every move, only your turn and results, or nothing. */
  announce: AnnounceLevel;
  /** Hover and long-press explanations on controls and cards. */
  tooltips: boolean;
  /** Single-key shortcuts on the web board (WCAG 2.1.4 asks that they can be turned off). */
  shortcuts: boolean;
  /** Diamonds blue and clubs green, so no two suits share a colour. */
  fourColour: boolean;
  /** The high-contrast palette: `auto` follows the system's "increase contrast". */
  contrast: AutoOnOff;
  /** Card size, layered over the width-derived scale. */
  cardSize: CardSize;
  /** Stillness: `auto` follows the system's "reduce motion". */
  motion: AutoOnOff;
  /** Ask before a move that ends the hand — going out, a last discard. */
  confirmFinal: boolean;
};

export const DEFAULT_A11Y_PREFS: A11yPrefs = {
  announce: 'all',
  tooltips: true,
  shortcuts: true,
  fourColour: false,
  contrast: 'auto',
  cardSize: 'normal',
  motion: 'auto',
  confirmFinal: false,
};

const KEY = 'zolik_a11y_prefs';

const ENUMS: { [K in keyof A11yPrefs]?: readonly string[] } = {
  announce: ['all', 'mine', 'off'],
  contrast: ['auto', 'on', 'off'],
  cardSize: ['normal', 'large', 'xlarge'],
  motion: ['auto', 'on', 'off'],
};

/** Reads a stored blob field by field, keeping the default for anything malformed. */
export function parsePrefs(raw: string | null | undefined): A11yPrefs {
  const out: A11yPrefs = { ...DEFAULT_A11Y_PREFS };
  if (!raw) return out;
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return out;
  }
  if (!parsed || typeof parsed !== 'object') return out;
  const p = parsed as Record<string, unknown>;
  for (const key of Object.keys(DEFAULT_A11Y_PREFS) as (keyof A11yPrefs)[]) {
    const v = p[key];
    const allowed = ENUMS[key];
    if (allowed) {
      if (typeof v === 'string' && allowed.includes(v)) (out as Record<string, unknown>)[key] = v;
    } else if (typeof v === 'boolean') {
      (out as Record<string, unknown>)[key] = v;
    }
  }
  return out;
}

let current: A11yPrefs = { ...DEFAULT_A11Y_PREFS };
let loaded = false;
const listeners = new Set<(p: A11yPrefs) => void>();

/** The choices as last read or set — defaults until the stored ones arrive. */
export function getA11yPrefs(): A11yPrefs {
  return current;
}

export async function loadA11yPrefs(): Promise<A11yPrefs> {
  if (loaded) return current;
  try {
    current = parsePrefs(await storage.getItem(KEY));
  } catch {
    current = { ...DEFAULT_A11Y_PREFS };
  }
  loaded = true;
  listeners.forEach((fn) => fn(current));
  return current;
}

export async function setA11yPref<K extends keyof A11yPrefs>(key: K, value: A11yPrefs[K]): Promise<void> {
  current = { ...current, [key]: value };
  loaded = true;
  listeners.forEach((fn) => fn(current));
  try {
    await storage.setItem(KEY, JSON.stringify(current));
  } catch {
    // A choice that could not be saved still applies for this session.
  }
}

export function subscribeA11yPrefs(fn: (p: A11yPrefs) => void): () => void {
  listeners.add(fn);
  return () => {
    listeners.delete(fn);
  };
}

/** The current choices, re-rendering whoever reads them when one changes. */
export function useA11yPrefs(): A11yPrefs {
  const [prefs, setPrefs] = useState<A11yPrefs>(current);
  useEffect(() => {
    const off = subscribeA11yPrefs(setPrefs);
    void loadA11yPrefs().then(setPrefs);
    return off;
  }, []);
  return prefs;
}
