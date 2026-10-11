import { useEffect, useState } from 'react';

import { storage } from '@/src/context/SessionContext';

/**
 * The sound choices a player makes about this device.
 *
 * `enabled` is the master switch — the one the match screen's quick button
 * flips. `voice` is a choice within it: the spoken announcer is the part of
 * the sound a table may least want, so it starts off, and muting everything
 * silences it too.
 *
 * Stored as one JSON blob, like the accessibility choices, so a build that
 * adds a field needs no migration: anything missing or malformed keeps its
 * default.
 */
export type SoundPrefs = {
  /** Master switch: card, pile, turn, round and match sounds. */
  enabled: boolean;
  /** A spoken announcer for the plays worth calling out — "Take two", "Last card". */
  voice: boolean;
};

export const DEFAULT_SOUND_PREFS: SoundPrefs = { enabled: true, voice: false };

const KEY = 'zolik_sound_prefs';

export function parseSoundPrefs(raw: string | null | undefined): SoundPrefs {
  const out = { ...DEFAULT_SOUND_PREFS };
  if (!raw) return out;
  try {
    const p: unknown = JSON.parse(raw);
    if (p && typeof p === 'object') {
      const r = p as Record<string, unknown>;
      if (typeof r.enabled === 'boolean') out.enabled = r.enabled;
      if (typeof r.voice === 'boolean') out.voice = r.voice;
    }
  } catch {
    // Keep the defaults.
  }
  return out;
}

let current: SoundPrefs = { ...DEFAULT_SOUND_PREFS };
let loaded = false;
const listeners = new Set<(p: SoundPrefs) => void>();

/** The choices as last read or set — defaults until the stored ones arrive. */
export function getSoundPrefs(): SoundPrefs {
  return current;
}

export async function loadSoundPrefs(): Promise<SoundPrefs> {
  if (loaded) return current;
  try {
    current = parseSoundPrefs(await storage.getItem(KEY));
  } catch {
    current = { ...DEFAULT_SOUND_PREFS };
  }
  loaded = true;
  listeners.forEach((fn) => fn(current));
  return current;
}

export async function setSoundPref<K extends keyof SoundPrefs>(key: K, value: SoundPrefs[K]): Promise<void> {
  current = { ...current, [key]: value };
  loaded = true;
  listeners.forEach((fn) => fn(current));
  try {
    await storage.setItem(KEY, JSON.stringify(current));
  } catch {
    // A choice that could not be saved still applies for this session.
  }
}

/** The current choices, re-rendering whoever reads them when one changes. */
export function useSoundPrefs(): SoundPrefs {
  const [prefs, setPrefs] = useState<SoundPrefs>(current);
  useEffect(() => {
    listeners.add(setPrefs);
    void loadSoundPrefs().then(setPrefs);
    return () => {
      listeners.delete(setPrefs);
    };
  }, []);
  return prefs;
}
