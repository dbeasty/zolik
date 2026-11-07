// A static import, in a .native file: a dynamic import of a native module
// fails quietly on device (see the notes on expo-notifications), and the web
// bundle must not carry this module at all.
import { requestAgeRangeAsync } from 'expo-age-range';

import { ADULT_AGE, type AgeStatus, classifyAgeError, classifyAgeRange } from './ageRange';

/** Asks the OS. On iOS 26+ the first ask shows Apple's sharing sheet. */
export async function requestAgeStatus(): Promise<AgeStatus> {
  try {
    return classifyAgeRange(await requestAgeRangeAsync({ threshold1: ADULT_AGE }));
  } catch (e) {
    return classifyAgeError(e);
  }
}
