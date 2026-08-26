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

/** Build identity, attached to a feedback report so it says which app it came from. */
export const APP_VERSION =
  (Constants.expoConfig?.version as string | undefined) ?? 'unknown';

/**
 * Platform label for a feedback report — "ios 17.4", "android 34", "web" — so
 * "only on Android" can be told apart from "everywhere" without going back to
 * the reporter to ask.
 *
 * Web is deliberately unversioned: react-native-web reports a placeholder
 * `Platform.Version` ("0.0.0"), and a version that is always the same number
 * is worse than no version — it reads as real data.
 */
export const APP_PLATFORM =
  Platform.OS === 'web' ? 'web' : `${Platform.OS} ${Platform.Version ?? ''}`.trim();
