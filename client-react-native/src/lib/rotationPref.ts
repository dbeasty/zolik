import { useEffect, useState } from 'react';
import { Platform } from 'react-native';
import * as ScreenOrientation from 'expo-screen-orientation';

import { storage } from '@/src/context/SessionContext';

/**
 * Which way the phone app's screen may turn: with the phone, or held in
 * portrait or landscape whichever way the phone is turned.
 *
 * The native project allows every orientation (app.json's `"orientation":
 * "default"`); this narrows it at runtime, so the choice takes effect the
 * moment it is made, with no rebuild.
 *
 * A device preference, never sent anywhere. It means nothing in a browser or
 * the desktop apps, whose window is whatever shape its owner drags it to, so
 * the setting is only offered on a phone or tablet. Storage failure degrades
 * to following the phone.
 */
export type Rotation = 'auto' | 'portrait' | 'landscape';

export const ROTATIONS: readonly Rotation[] = ['auto', 'portrait', 'landscape'];

/** Only iOS and Android can turn; everything else has a window instead. */
export const ROTATION_SUPPORTED = Platform.OS === 'ios' || Platform.OS === 'android';

const KEY = 'zolik_rotation';
const listeners = new Set<(r: Rotation) => void>();
let cached: Rotation | null = null;

export function parseRotation(raw: string | null | undefined): Rotation {
  return raw === 'portrait' || raw === 'landscape' ? raw : 'auto';
}

async function load(): Promise<Rotation> {
  if (cached) return cached;
  try {
    cached = parseRotation(await storage.getItem(KEY));
  } catch {
    cached = 'auto';
  }
  return cached;
}

const LOCKS: Record<Rotation, ScreenOrientation.OrientationLock> = {
  auto: ScreenOrientation.OrientationLock.DEFAULT,
  portrait: ScreenOrientation.OrientationLock.PORTRAIT_UP,
  landscape: ScreenOrientation.OrientationLock.LANDSCAPE,
};

async function apply(r: Rotation): Promise<void> {
  if (!ROTATION_SUPPORTED) return;
  try {
    await ScreenOrientation.lockAsync(LOCKS[r]);
  } catch {
    // A device that refuses a lock keeps turning with the phone.
  }
}

export async function setRotation(r: Rotation): Promise<void> {
  cached = r;
  listeners.forEach((fn) => fn(r));
  void apply(r);
  try {
    await storage.setItem(KEY, r);
  } catch {
    // Kept for this session; it simply will not survive a restart.
  }
}

/** This device's choice. */
export function useRotation(): Rotation {
  const [r, setR] = useState<Rotation>(cached ?? 'auto');
  useEffect(() => {
    let live = true;
    void load().then((v) => live && setR(v));
    listeners.add(setR);
    return () => {
      live = false;
      listeners.delete(setR);
    };
  }, []);
  return r;
}

/** Puts the saved choice into effect at startup; mounted once, in the root layout. */
export function useApplyRotation(): void {
  useEffect(() => {
    if (ROTATION_SUPPORTED) void load().then(apply);
  }, []);
}
