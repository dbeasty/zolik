import { useNavigationContainerRef } from 'expo-router';
import { useEffect } from 'react';
import { Platform } from 'react-native';

import { t } from '@/src/lib/i18n';

/**
 * The browser tab's title, per screen (WCAG 2.4.2).
 *
 * Every tab used to say "Jokerless" — or nothing, in the dev server, which
 * does not render `+html.tsx` — so a player with six tabs open, or a screen
 * reader announcing the page they had just arrived on, could not tell the
 * rules from the lobby. The words already exist: each route in
 * `app/_layout.tsx` has a navigation-bar title. This follows whichever one is
 * on screen, so the tab and the bar cannot disagree, a new route gets a title
 * for free, and a screen that changes its own bar title changes the tab too.
 *
 * Derived from the router rather than set by each screen because expo-router
 * switches React Navigation's own document title off (it would show raw route
 * names before the options arrive) — and once one place owns the title, no
 * screen has to remember to.
 */

/** "Rules · Jokerless"; the app's own name alone for the home screen or a screen with no title. */
export function pageTitle(screen?: string | null): string {
  const app = t('nav.home');
  const name = (screen ?? '').trim();
  if (!name || name === app) return app;
  return t('a11y.page.title', { screen: name });
}

type Options = { title?: unknown; headerTitle?: unknown };

/** The focused screen's title as a plain string, if it has one. */
export function screenTitle(options: Options | undefined): string | undefined {
  if (!options) return undefined;
  if (typeof options.title === 'string') return options.title;
  if (typeof options.headerTitle === 'string') return options.headerTitle;
  return undefined;
}

/**
 * Keeps `document.title` on the focused screen's title. Web only; mounted once
 * at the root (`A11yRoot`), re-run when the language changes so the title is
 * re-worded with the bar.
 */
export function useRouteDocumentTitle(locale: string) {
  const nav = useNavigationContainerRef();

  useEffect(() => {
    if (Platform.OS !== 'web' || typeof document === 'undefined' || !nav) return;
    const apply = (options?: Options) => {
      const current = options ?? (nav.isReady() ? (nav.getCurrentOptions() as Options | undefined) : undefined);
      document.title = pageTitle(screenTitle(current));
    };
    apply();
    // `options` fires when the focused screen changes and when a screen sets
    // new options; `state` covers the first render, before any options event.
    const offOptions = nav.addListener('options', (e) => apply(e.data.options as Options));
    const offState = nav.addListener('state', () => apply());
    // The bar's titles are re-worded a frame after the language changes, and
    // nothing re-emits `options` for a screen that stayed put.
    const timer = setTimeout(() => apply(), 50);
    return () => {
      offOptions();
      offState();
      clearTimeout(timer);
    };
  }, [nav, locale]);
}
