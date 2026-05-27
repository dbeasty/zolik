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
