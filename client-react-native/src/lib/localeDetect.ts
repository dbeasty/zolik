import { NativeModules, Platform } from 'react-native';

import { DEFAULT_LOCALE, isLocale, type Locale } from './i18n';

/**
 * What language the device is asking for, before the player has said.
 *
 * The point of guessing is that most people never open a language setting —
 * they open the app, and it is either in their language or it is not. Getting
 * this right is worth more than the picker is, because the picker only helps
 * the player who already went looking for it.
 *
 * Three deliberate limits:
 *
 *  1. **A guess is never remembered.** Detection runs on every launch and
 *     writes nothing. Only the player's own choice is persisted — see
 *     `localeStore`. A device that changes its language later should follow
 *     it, and a guess saved once would quietly outrank the device forever.
 *  2. **Region is dropped, script is dropped.** `pt-BR`, `de-AT` and
 *     `sr-Latn-RS` are matched on their language subtag alone. We ship one
 *     Portuguese, and a Brazilian reader is far better served by it than by
 *     English.
 *  3. **No cousin-language fallback.** Bosnian does not resolve to Croatian,
 *     Norwegian does not resolve to Danish, Catalan does not resolve to
 *     Spanish. They are close enough that it is tempting and close enough
 *     that being wrong about it is insulting. Unmatched means English, which
 *     is merely unhelpful.
 */

/**
 * ISO 639-2/639-3 spellings mapped onto the 639-1 codes we key bundles by.
 *
 * Browsers hand back two-letter tags, so this is for the platforms that do
 * not: some Android builds report `deu`, and a few locale identifiers still
 * carry the bibliographic spelling (`ger`, `fre`, `dut`) rather than the
 * terminological one.
 */
const ALIASES: Record<string, Locale> = {
  bul: 'bg',
  ces: 'cs',
  cze: 'cs',
  dan: 'da',
  deu: 'de',
  ger: 'de',
  ell: 'el',
  gre: 'el',
  eng: 'en',
  spa: 'es',
  est: 'et',
  fin: 'fi',
  fra: 'fr',
  fre: 'fr',
  gle: 'ga',
  hrv: 'hr',
  hun: 'hu',
  ita: 'it',
  lit: 'lt',
  lav: 'lv',
  mlt: 'mt',
  nld: 'nl',
  dut: 'nl',
  pol: 'pl',
  por: 'pt',
  ron: 'ro',
  rum: 'ro',
  slk: 'sk',
  slo: 'sk',
  slv: 'sl',
  swe: 'sv',
};

/**
 * One BCP 47 tag — or an Apple/Android locale identifier — to a locale we ship.
 *
 * Accepts `cs`, `cs-CZ`, `cs_CZ`, `ces`, `sr-Latn-RS`; returns undefined for
 * anything we have no bundle for.
 */
export function localeFromTag(tag: string | null | undefined): Locale | undefined {
  if (!tag) return undefined;
  // Apple reports `cs_CZ`, the web reports `cs-CZ`, and a stray `@calendar=…`
  // suffix turns up on some Android identifiers. The language subtag is the
  // only part any of them agree on.
  const primary = tag.trim().split(/[-_@.]/)[0].toLowerCase();
  if (!primary) return undefined;
  if (isLocale(primary)) return primary;
  return ALIASES[primary];
}

/**
 * The device's languages, most-preferred first.
 *
 * A list rather than one value because both platforms offer an ordered
 * preference and both mean it: a player whose phone reads
 * [Irish, English, French] wants Irish, and should get French before English
 * only if we had no Irish — which the caller decides, not this.
 *
 * Deliberately built on what the runtime already has rather than on
 * `expo-localization`, whose `getLocales()` would answer this in one call.
 * That package is a native module: adding it obliges everyone holding a dev
 * client to rebuild it, to detect something the platform already tells us.
 * The shape here is the shape `getLocales()` returns, mapped to tags, so the
 * day it earns its place the swap is this function's body and nothing else.
 *
 * Sources, in order of how much they know:
 *
 *  1. `navigator.languages` on web — the ordered list, and exactly what the
 *     browser sends as `Accept-Language`.
 *  2. The platform's own preference list on native, which is ordered on iOS.
 *  3. `Intl`, which every RN runtime since 0.70 has on both platforms. One
 *     locale rather than a list, but it is right about that one.
 */
export function devicePreferredTags(): string[] {
  const tags: string[] = [];

  if (Platform.OS === 'web') {
    if (typeof navigator !== 'undefined') {
      const list = navigator.languages;
      if (Array.isArray(list) && list.length) tags.push(...list);
      else if (navigator.language) tags.push(navigator.language);
    }
  } else {
    try {
      if (Platform.OS === 'ios') {
        const settings = NativeModules.SettingsManager?.settings;
        // `AppleLanguages` is the ordered preference list and the one to
        // trust; `AppleLocale` is a single *formatting* locale, and on a phone
        // set to Irish with a UK region it says en_GB. Both are tried, in that
        // order, because being wrong here means an Irish speaker gets English.
        const languages = settings?.AppleLanguages;
        if (Array.isArray(languages)) tags.push(...languages);
        if (settings?.AppleLocale) tags.push(settings.AppleLocale);
      } else {
        const identifier = NativeModules.I18nManager?.localeIdentifier;
        if (identifier) tags.push(identifier);
      }
    } catch {
      // A bare JS runtime, a test, or a core module that moved under the New
      // Architecture. Not knowing the language is a fine outcome and `Intl`
      // below still answers; throwing during launch is not.
    }
  }

  try {
    const resolved = Intl.DateTimeFormat().resolvedOptions().locale;
    if (resolved) tags.push(resolved);
  } catch {
    // No Intl. Nothing left to ask, and the caller defaults to English.
  }

  return tags;
}

/**
 * The best locale for this device, or English when it asks for nothing we have.
 *
 * `tags` is injectable so the test can state a preference list rather than
 * mock a native module.
 */
export function detectLocale(tags: string[] = devicePreferredTags()): Locale {
  for (const tag of tags) {
    const match = localeFromTag(tag);
    if (match) return match;
  }
  return DEFAULT_LOCALE;
}
