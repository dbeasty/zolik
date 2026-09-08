import { storage } from '@/src/context/SessionContext';

import { isLocale, type Locale } from './i18n';

/**
 * The player's language choice, remembered on the device.
 *
 * `null` is not "no language" — it is *automatic*, and it is the default.
 * The distinction is the whole point of this module: a device set to follow
 * its own language must keep following it when the player later changes the
 * phone's language, and storing the detected result would silently pin them
 * to whatever they happened to have on first launch.
 *
 * So only an explicit choice is written, and choosing "Automatic" in the
 * picker deletes rather than writes. The key is the one the app has always
 * read (`zolik_locale`), so a value put there by an earlier build still
 * counts — though until now nothing ever wrote one.
 */

const KEY = 'zolik_locale';

/** The saved override, or null for automatic. */
export async function loadLocaleOverride(): Promise<Locale | null> {
  try {
    const saved = await storage.getItem(KEY);
    return isLocale(saved) ? saved : null;
  } catch {
    return null;
  }
}

/** Saves an explicit choice, or clears it back to automatic with `null`. */
export async function saveLocaleOverride(locale: Locale | null): Promise<void> {
  try {
    if (locale === null) await storage.deleteItem(KEY);
    else await storage.setItem(KEY, locale);
  } catch {
    // Losing a preference is a small thing; interrupting a game over it is not.
  }
}
