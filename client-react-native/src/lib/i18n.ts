/**
 * Message keys and locale bundles.
 *
 * The server never sends a rendered sentence. It sends stable keys — engine
 * error codes (`NOT_YOUR_TURN`), option names, and structured facts like the
 * contract's sets/runs counts — and the wording lives here. That split is what
 * makes a Czech UI possible without touching the server, and it is the reason
 * Phase 1 shipped `whyNot` as a code rather than a message.
 *
 * Three rules this module keeps:
 *
 *  1. **A missing translation degrades, never blanks.** The lookup falls back
 *     locale → English → the caller's fallback → the key itself. A player
 *     seeing an untranslated English string is a bad day; a player seeing an
 *     empty control is a bug report.
 *  2. **Every key in one bundle exists in all of them.** Asserted by a test,
 *     not by review — a half-translated locale is how "mostly Czech with
 *     random English" ships.
 *  3. **No rule knowledge.** This file maps keys to words. It never decides
 *     what is legal, what a card is worth, or which profile is in play.
 *
 * The bundles themselves live one file per language in `./locales`. They were
 * inline here while there were two of them; at twenty-four that made the
 * lookup logic unfindable between the words.
 */

import { bg } from './locales/bg';
import { cs } from './locales/cs';
import { da } from './locales/da';
import { de } from './locales/de';
import { el } from './locales/el';
import { en } from './locales/en';
import { es } from './locales/es';
import { et } from './locales/et';
import { fi } from './locales/fi';
import { fr } from './locales/fr';
import { ga } from './locales/ga';
import { hr } from './locales/hr';
import { hu } from './locales/hu';
import { it } from './locales/it';
import { lt } from './locales/lt';
import { lv } from './locales/lv';
import { mt } from './locales/mt';
import { nl } from './locales/nl';
import { pl } from './locales/pl';
import { pt } from './locales/pt';
import { ro } from './locales/ro';
import { sk } from './locales/sk';
import { sl } from './locales/sl';
import { sv } from './locales/sv';

/**
 * The twenty-four official languages of the European Union.
 *
 * The list is the EU's, not a ranking of our players: picking "the big five"
 * and calling it European support is how Maltese and Irish players learn the
 * product was not built with them in mind. A language nobody selects costs a
 * file; a language somebody needed and did not find costs the player.
 */
export type Locale =
  | 'bg'
  | 'cs'
  | 'da'
  | 'de'
  | 'el'
  | 'en'
  | 'es'
  | 'et'
  | 'fi'
  | 'fr'
  | 'ga'
  | 'hr'
  | 'hu'
  | 'it'
  | 'lt'
  | 'lv'
  | 'mt'
  | 'nl'
  | 'pl'
  | 'pt'
  | 'ro'
  | 'sk'
  | 'sl'
  | 'sv';

/**
 * Every locale, labelled in itself.
 *
 * "Deutsch", not "German": someone hunting for their language in a list is
 * scanning for the word they would use, and by definition cannot read the
 * label if it is written in a language they do not have. `english` rides
 * alongside for the benefit of anyone reading this file rather than the
 * picker.
 *
 * Ordered by code rather than by speaker count, which keeps the list stable
 * and keeps us out of the business of ranking languages.
 */
export const LOCALES: { id: Locale; label: string; english: string }[] = [
  { id: 'bg', label: 'Български', english: 'Bulgarian' },
  { id: 'cs', label: 'Čeština', english: 'Czech' },
  { id: 'da', label: 'Dansk', english: 'Danish' },
  { id: 'de', label: 'Deutsch', english: 'German' },
  { id: 'el', label: 'Ελληνικά', english: 'Greek' },
  { id: 'en', label: 'English', english: 'English' },
  { id: 'es', label: 'Español', english: 'Spanish' },
  { id: 'et', label: 'Eesti', english: 'Estonian' },
  { id: 'fi', label: 'Suomi', english: 'Finnish' },
  { id: 'fr', label: 'Français', english: 'French' },
  { id: 'ga', label: 'Gaeilge', english: 'Irish' },
  { id: 'hr', label: 'Hrvatski', english: 'Croatian' },
  { id: 'hu', label: 'Magyar', english: 'Hungarian' },
  { id: 'it', label: 'Italiano', english: 'Italian' },
  { id: 'lt', label: 'Lietuvių', english: 'Lithuanian' },
  { id: 'lv', label: 'Latviešu', english: 'Latvian' },
  { id: 'mt', label: 'Malti', english: 'Maltese' },
  { id: 'nl', label: 'Nederlands', english: 'Dutch' },
  { id: 'pl', label: 'Polski', english: 'Polish' },
  { id: 'pt', label: 'Português', english: 'Portuguese' },
  { id: 'ro', label: 'Română', english: 'Romanian' },
  { id: 'sk', label: 'Slovenčina', english: 'Slovak' },
  { id: 'sl', label: 'Slovenščina', english: 'Slovenian' },
  { id: 'sv', label: 'Svenska', english: 'Swedish' },
];

/** The locale used when nothing else is known, and the fallback for every lookup. */
export const DEFAULT_LOCALE: Locale = 'en';

/**
 * Interpolation params. Values are substituted into `{name}` placeholders.
 */
export type Params = Record<string, string | number>;

export const BUNDLES: Record<Locale, Record<string, string>> = {
  bg,
  cs,
  da,
  de,
  el,
  en,
  es,
  et,
  fi,
  fr,
  ga,
  hr,
  hu,
  it,
  lt,
  lv,
  mt,
  nl,
  pl,
  pt,
  ro,
  sk,
  sl,
  sv,
};

/** Narrows an arbitrary string to a locale we actually ship. */
export function isLocale(value: unknown): value is Locale {
  return typeof value === 'string' && Object.prototype.hasOwnProperty.call(BUNDLES, value);
}

let currentLocale: Locale = DEFAULT_LOCALE;

/**
 * Who wants telling when the language changes.
 *
 * `t` reads a module-level variable, which was fine while the locale was set
 * once at launch and never again. A picker makes it change while the tree is
 * mounted, and a component that already rendered "Draw" has no reason to
 * render again — the words it called `t` for are not props or state. This is
 * the missing dependency, exposed so `useLocale` can subscribe to it with
 * `useSyncExternalStore`.
 */
const listeners = new Set<() => void>();

export function subscribeToLocale(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export function setLocale(locale: Locale) {
  const next = isLocale(locale) ? locale : DEFAULT_LOCALE;
  if (next === currentLocale) return;
  currentLocale = next;
  for (const listener of listeners) listener();
}

export function getLocale(): Locale {
  return currentLocale;
}

/**
 * Looks up a key and substitutes `{name}` placeholders.
 *
 * Falls back locale → English → `fallback` → the key itself, so an
 * untranslated string degrades to a readable one rather than to nothing.
 */
export function t(key: string, params?: Params, fallback?: string): string {
  return interpolate(messageTemplate(key) ?? fallback ?? key, params);
}

/**
 * The wording a key would use, or undefined for a key no bundle knows.
 *
 * Exported so a caller can tell "this key has words of its own" from "this key
 * is about to be rendered by its own shape". The difference matters to
 * whoever is composing a line out of more than the key alone — see `factText`,
 * where a phrase that already places its own values must not have another one
 * appended after it.
 */
export function messageTemplate(key: string): string | undefined {
  return BUNDLES[currentLocale]?.[key] ?? BUNDLES.en[key];
}

function interpolate(template: string, params?: Params): string {
  if (!params) return template;
  return template.replace(/\{(\w+)\}/g, (whole, name: string) =>
    name in params ? String(params[name]) : whole,
  );
}

/**
 * Wording for an engine error code. The codes are the server's stable
 * vocabulary; only the phrasing is ours. An unrecognised code — a newer
 * server than this build — falls back rather than printing SCREAMING_SNAKE at
 * the player.
 */
export function reasonText(code: string | undefined, fallback = ''): string {
  if (!code) return fallback;
  return t(`err.${code}`, undefined, fallback);
}

/**
 * "One set", "Two runs" — the whole phrase, looked up by count.
 *
 * Not a number glued to a pluralised noun: Czech inflects the noun by count
 * in a way no "add an s" helper survives, so the phrase is what the bundle
 * owns. Counts beyond the enumerated ones fall back to a generic form.
 */
export function countLabel(n: number, noun: 'sets' | 'runs'): string {
  const exact = `contract.${noun}.${n}`;
  if (BUNDLES.en[exact]) return t(exact);
  return t(`contract.${noun}.n`, { n });
}
