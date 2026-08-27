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
