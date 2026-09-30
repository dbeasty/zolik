import { useCallback, useEffect, useState, useSyncExternalStore } from 'react';

import {
  getLocale,
  setLocale,
  setMissingKeyMarker,
  subscribeToLocale,
  type Locale,
} from '@/src/lib/i18n';
import { detectLocale } from '@/src/lib/localeDetect';
import { loadLocaleOverride, saveLocaleOverride } from '@/src/lib/localeStore';

/**
 * The language the interface is in, and how to change it.
 *
 * Two things are deliberately separate here:
 *
 *  - **What is on screen** (`locale`) is always a real language.
 *  - **What the player chose** (`override`) is a language *or null*, where
 *    null means "follow the device". The picker has to show which of those
 *    two it is, because "Deutsch" and "Automatic, which is currently Deutsch"
 *    behave differently the day the player changes their phone's language.
 *
 * `locale` comes from `useSyncExternalStore` rather than from React state
 * because `t()` reads a module global that non-component code also writes.
 * Subscribing to that global is what makes a language change repaint a screen
 * whose words are not props.
 */

type LocaleState = {
  /** The language actually being rendered. Never null. */
  locale: Locale;
  /** The player's explicit choice, or null when following the device. */
  override: Locale | null;
  /** What the device asks for, shown against the "Automatic" row. */
  detected: Locale;
  /** Whether the saved choice has been read yet. */
  ready: boolean;
  /** Sets the choice; `null` hands control back to the device. */
  chooseLocale: (locale: Locale | null) => void;
};

/**
 * Reads the saved choice and applies it, once, at launch.
 *
 * Runs before the first paint completes rather than after, so the opening
 * screen is already in the player's language instead of flashing English —
 * the same reason `SessionContext` did this by hand before there was a
 * picker to make it worth a module.
 */
export function useLocaleBootstrap(): boolean {
  const [ready, setReady] = useState(false);

  useEffect(() => {
    // Armed only by an explicit local flag, and read once at launch. This is
    // how `_TX_<key>_` gets switched on for a diagnostic pass — see
    // `setMissingKeyMarker` — without the marker ever being reachable from
    // anything a player can press.
    try {
      if (typeof localStorage !== 'undefined') {
        setMissingKeyMarker(localStorage.getItem('zolik_i18n_debug') === '1');
      }
    } catch {
      /* no storage here; the marker simply stays off, which is the default */
    }
  }, []);

  useEffect(() => {
    let live = true;
    // The device's answer first, so a launch with no saved choice is already
    // in the right language while the (async, possibly slow on native) read
    // of the override is still in flight.
    setLocale(detectLocale());
    loadLocaleOverride()
      .then((saved) => {
        if (!live) return;
        if (saved) setLocale(saved);
        setReady(true);
      })
      .catch(() => live && setReady(true));
    return () => {
      live = false;
    };
  }, []);

  return ready;
}

/** The active locale, re-rendering the caller whenever it changes. */
export function useLocale(): Locale {
  return useSyncExternalStore(subscribeToLocale, getLocale, getLocale);
}

/** The picker's view: the choice, what automatic currently resolves to, and the setter. */
export function useLocaleControls(): LocaleState {
  const locale = useLocale();
  const [override, setOverride] = useState<Locale | null>(null);
  const [ready, setReady] = useState(false);
  const [detected] = useState(() => detectLocale());

  useEffect(() => {
    let live = true;
    loadLocaleOverride().then((saved) => {
      if (!live) return;
      setOverride(saved);
      setReady(true);
    });
    return () => {
      live = false;
    };
  }, []);

  const chooseLocale = useCallback(
    (next: Locale | null) => {
      setOverride(next);
      setLocale(next ?? detectLocale());
      void saveLocaleOverride(next);
    },
    [],
  );

  return { locale, override, detected, ready, chooseLocale };
}
