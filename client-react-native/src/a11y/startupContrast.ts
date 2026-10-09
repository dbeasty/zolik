import { Platform } from 'react-native';
import * as SecureStore from 'expo-secure-store';

import { SYSTEM_CONTRAST_KEY, systemContrastNow } from '@/src/a11y/systemContrast';

/**
 * Whether high contrast was asked for *as the app started* — for the screens
 * outside a match, whose palette (`src/theme.ts`) is fixed when the module
 * loads and baked into style sheets all over the app.
 *
 * The board reads the live answer through `useSkin` and changes the moment
 * the setting does. These screens cannot without every one of them moving
 * its styles into a hook, which is a large, risky change for a palette that
 * already passes AA (`contrast.test.ts`). So they read the answer once, here,
 * synchronously, before any style sheet exists, and follow a change the next
 * time the app opens — Settings says so when it applies.
 *
 * Deliberately standalone (no import of `prefs.ts`, which reaches the session
 * context): `theme.ts` is imported by nearly everything, and a cycle through
 * it would hand some module an undefined palette.
 */

const PREFS_KEY = 'zolik_a11y_prefs';

function readSync(key: string): string | null {
  try {
    if (Platform.OS === 'web') {
      return typeof localStorage !== 'undefined' ? localStorage.getItem(key) : null;
    }
    return SecureStore.getItem(key);
  } catch {
    return null;
  }
}

/** The stored `contrast` choice, or `auto` when there is none or it is unreadable. */
function storedChoice(): 'auto' | 'on' | 'off' {
  try {
    const raw = readSync(PREFS_KEY);
    const v = raw ? (JSON.parse(raw) as { contrast?: unknown }).contrast : undefined;
    return v === 'on' || v === 'off' ? v : 'auto';
  } catch {
    return 'auto';
  }
}

function computeStartupContrast(): boolean {
  const choice = storedChoice();
  if (choice !== 'auto') return choice === 'on';
  if (Platform.OS === 'web') return systemContrastNow();
  // A phone answers asynchronously, too late for a style sheet: use what it
  // said last time (`useSystemContrast` keeps it).
  return readSync(SYSTEM_CONTRAST_KEY) === '1';
}

/** Computed once, at first import — the point is that it does not change. */
export const STARTUP_HIGH_CONTRAST = computeStartupContrast();
