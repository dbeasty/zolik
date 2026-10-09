import { useEffect, useState } from 'react';
import { AccessibilityInfo, Platform } from 'react-native';
import * as SecureStore from 'expo-secure-store';

/**
 * Where a phone's last answer is kept for the next start — see
 * `startupContrast.ts`, which needs it before any answer can arrive.
 */
export const SYSTEM_CONTRAST_KEY = 'zolik_system_contrast';

function remember(on: boolean) {
  SecureStore.setItemAsync(SYSTEM_CONTRAST_KEY, on ? '1' : '0').catch(() => {});
}

/**
 * Whether the operating system has been asked for more contrast.
 *
 * Each platform says it differently, and React Native 0.85 exposes the two
 * native ones as separate, platform-only calls:
 *
 * - **Web**: `prefers-contrast: more` (macOS and iOS "Increase contrast",
 *   or a browser setting), or `forced-colors: active` (Windows contrast
 *   themes, where the browser repaints everything in the system's colours
 *   anyway — the high-contrast board is the one whose palette survives that
 *   best, having no gradients or implied edges to lose).
 * - **iOS**: "Increase Contrast", which UIKit calls *darker system colors*
 *   — `isDarkerSystemColorsEnabled`, with `darkerSystemColorsChanged`.
 * - **Android**: "High contrast text" — `isHighTextContrastEnabled`, with
 *   `highTextContrastChanged`.
 *
 * Read synchronously on the web (a media query answers at once, so the
 * first frame is already the right board) and asynchronously on a phone.
 */
export function systemContrastNow(): boolean {
  if (Platform.OS === 'web' && typeof window !== 'undefined' && window.matchMedia) {
    return (
      window.matchMedia('(prefers-contrast: more)').matches ||
      window.matchMedia('(forced-colors: active)').matches
    );
  }
  return false;
}

export function useSystemContrast(): boolean {
  const [more, setMore] = useState(systemContrastNow);

  useEffect(() => {
    if (Platform.OS === 'web') {
      if (typeof window === 'undefined' || !window.matchMedia) return;
      const queries = [
        window.matchMedia('(prefers-contrast: more)'),
        window.matchMedia('(forced-colors: active)'),
      ];
      const onChange = () => setMore(queries.some((q) => q.matches));
      onChange();
      queries.forEach((q) => q.addEventListener?.('change', onChange));
      return () => queries.forEach((q) => q.removeEventListener?.('change', onChange));
    }

    let live = true;
    const ios = Platform.OS === 'ios';
    const ask = ios
      ? AccessibilityInfo.isDarkerSystemColorsEnabled
      : AccessibilityInfo.isHighTextContrastEnabled;
    // Called through `AccessibilityInfo` rather than detached, and guarded:
    // each exists on one platform only.
    Promise.resolve(ask ? ask.call(AccessibilityInfo) : false)
      .then((on) => {
        remember(!!on);
        if (live) setMore(!!on);
      })
      .catch(() => {});
    const sub = AccessibilityInfo.addEventListener(
      ios ? 'darkerSystemColorsChanged' : 'highTextContrastChanged',
      (on: boolean) => {
        remember(!!on);
        setMore(!!on);
      },
    );
    return () => {
      live = false;
      sub?.remove?.();
    };
  }, []);

  return more;
}
