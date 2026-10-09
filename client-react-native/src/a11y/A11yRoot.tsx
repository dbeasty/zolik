import { useEffect } from 'react';
import { Platform } from 'react-native';

import { useRouteDocumentTitle } from '@/src/a11y/documentTitle';
import { LiveRegion } from '@/src/a11y/LiveRegion';
import { loadA11yPrefs } from '@/src/a11y/prefs';
import { TipHost } from '@/src/a11y/Tip';
import { useLocale } from '@/src/hooks/useLocale';

/**
 * Everything accessibility needs mounted once, at the root, over every screen:
 * the live region `announce()` writes to on the web, the tooltip bubble, the
 * document's language and title, and the focus ring.
 */
export function A11yRoot() {
  const locale = useLocale();
  // The tab's title follows the screen's — see `documentTitle.ts`.
  useRouteDocumentTitle(locale);

  useEffect(() => {
    void loadA11yPrefs();
  }, []);

  // A screen reader picks its voice from `<html lang>`. The static export
  // writes "en" into every page; a Czech player's Czech words read in an
  // English voice are barely words at all.
  useEffect(() => {
    if (Platform.OS === 'web' && typeof document !== 'undefined') {
      document.documentElement.lang = locale;
    }
  }, [locale]);

  useEffect(() => {
    if (Platform.OS !== 'web' || typeof document === 'undefined') return;
    if (document.getElementById('zolik-a11y-css')) return;
    const style = document.createElement('style');
    style.id = 'zolik-a11y-css';
    style.textContent = FOCUS_CSS;
    document.head.appendChild(style);
  }, []);

  return (
    <>
      <LiveRegion />
      <TipHost />
    </>
  );
}

/**
 * The keyboard focus ring, the same everywhere: white inside, black outside,
 * so it holds at least 3:1 against the dark chrome, the green felt and a white
 * card face alike (WCAG 2.4.7, 1.4.11). `:focus-visible` only — a mouse click
 * does not draw it. React Native Web resets `outline` on its pressables, hence
 * the `!important`.
 */
const FOCUS_CSS = `
:focus-visible {
  outline: 2px solid #ffffff !important;
  outline-offset: 2px !important;
  box-shadow: 0 0 0 4px #000000 !important;
}
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after { scroll-behavior: auto !important; }
}
`;
