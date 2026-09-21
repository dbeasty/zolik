import Constants from 'expo-constants';
import { Platform } from 'react-native';

const envUrl = process.env.EXPO_PUBLIC_ZOLIK_BASE_URL;

/** Default API base for local dev (override with EXPO_PUBLIC_ZOLIK_BASE_URL). */
function defaultBaseUrl(): string {
  if (Platform.OS === 'android') {
    return 'http://10.0.2.2:8090';
  }
  return 'http://127.0.0.1:8090';
}

/**
 * The origin this page was served from, when the build says the API lives
 * there too.
 *
 * The production image compiles the web export into the server binary, so the
 * API is by construction whoever served the page — and one deployment answers
 * on several domains (jokerless.com, jokerless.org, play.limidus.com). A base
 * URL baked in at build time names only one of them, and a page loaded from
 * any other would call it cross-origin, which the server does not allow. The
 * Dockerfile sets EXPO_PUBLIC_ZOLIK_API_SAME_ORIGIN=1; a plain Expo dev
 * server, where the client and API are on different ports, does not.
 *
 * Empty during the static prerender (no `window`) and on native, where the
 * baked-in URL below is the answer.
 */
function sameOriginBaseUrl(): string {
  if (process.env.EXPO_PUBLIC_ZOLIK_API_SAME_ORIGIN !== '1') return '';
  if (Platform.OS !== 'web') return '';
  if (typeof window === 'undefined' || !window.location?.origin) return '';
  return window.location.origin;
}

export const ZOLIK_BASE_URL = (sameOriginBaseUrl() || envUrl || defaultBaseUrl()).replace(
  /\/$/,
  '',
);

export const APP_NAME =
  (Constants.expoConfig?.name as string | undefined) ?? 'Žolíky';

/**
 * The build this bundle was made from — set by scripts/version.sh via the
 * npm scripts and dev-stack.sh (see package.json). EXPO_PUBLIC_* rather than
 * an app.config.js `extra` value: on web that value is inlined once and
 * cached per-file by metro, so it would go stale until a manual cache clear,
 * and jest-expo mocks the manifest to `{}` under test regardless of platform,
 * so it can never be asserted on there either. EXPO_PUBLIC_* is re-read from
 * the dev server's live environment on every bundle and resolves fine under
 * jest, with neither problem.
 *
 * The fallbacks are deliberately not a plausible version: "0.0.0-dev" in the
 * footer means "Expo was started without the version script", not "you're on
 * version zero".
 */
export const CLIENT_VERSION = process.env.EXPO_PUBLIC_ZOLIK_VERSION || '0.0.0-dev';
export const CLIENT_COMMIT = process.env.EXPO_PUBLIC_ZOLIK_COMMIT || 'unknown';

/**
 * Who the legal notices name as the operator, set at build time by
 * `scripts/deploy.sh`.
 *
 * Build-time rather than a checked-in constant because it is deployment
 * configuration, not code: the same source deployed by someone else names
 * someone else, and a fork that ships with our company name in its privacy
 * notice would be making a claim about us. It rides the same EXPO_PUBLIC_*
 * channel as the version above, for the same reasons.
 *
 * Empty rather than a default here — `src/legal` decides what an unnamed
 * operator means, and what it means is a draft banner rather than a document
 * confidently naming nobody.
 */
export const OPERATOR_NAME = process.env.EXPO_PUBLIC_ZOLIK_OPERATOR || '';
export const OPERATOR_COUNTRY = process.env.EXPO_PUBLIC_ZOLIK_OPERATOR_COUNTRY || '';
export const OPERATOR_CONTACT = process.env.EXPO_PUBLIC_ZOLIK_OPERATOR_CONTACT || '';

/**
 * Where the source of the running build can be had, for the AGPL's benefit.
 *
 * Section 13 is the clause that makes zolik's licence different from a plain
 * GPL one: a network user who never receives a binary must still be offered
 * the Corresponding Source. So the offer has to live in the app, not only in
 * the repository — a LICENSE file rsynced to a server nobody logs into offers
 * nothing to the person playing at jokerless.com.
 *
 * Overridable at build time, and that is the point rather than a convenience:
 * whoever deploys a modified zolik owes their readers *their* source, not
 * ours. The default is right for a deployment of this source unmodified, which
 * is what ours is; `scripts/deploy.sh` passes ZOLIK_SOURCE_URL for anything
 * else. Unlike the operator fields above there is no draft state — a wrong
 * source link is a broken promise, an unset one would be no promise at all,
 * and the canonical repository is a true answer for every build that has not
 * changed the code.
 */
export const SOURCE_URL = (
  process.env.EXPO_PUBLIC_ZOLIK_SOURCE_URL || 'https://github.com/dbeasty/zolik'
).replace(/\/$/, '');
