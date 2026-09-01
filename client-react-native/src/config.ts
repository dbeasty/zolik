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

export const ZOLIK_BASE_URL = (envUrl || defaultBaseUrl()).replace(/\/$/, '');

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
