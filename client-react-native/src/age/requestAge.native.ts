// A static import, in a .native file: a dynamic import of a native module
// fails quietly on device (see the notes on expo-notifications), and the web
// bundle must not carry this module at all.
//
// The native module is looked up rather than imported through expo-age-range,
// whose entry calls `requireNativeModule` as it loads and so throws at import
// in a binary built before the module was added, taking the whole app with it.
// Its JS `requestAgeRangeAsync` only forwards to this same native function; the
// package stays a dependency because that is what links the native side.
import { requireOptionalNativeModule } from 'expo';
import type { AgeRangeRequest, AgeRangeResponse } from 'expo-age-range';

import { ADULT_AGE, type AgeStatus, classifyAgeError, classifyAgeRange } from './ageRange';

type AgeRangeNative = {
  requestAgeRangeAsync(options: AgeRangeRequest): Promise<AgeRangeResponse>;
};

/** Asks the OS. On iOS 26+ the first ask shows Apple's sharing sheet. */
export async function requestAgeStatus(): Promise<AgeStatus> {
  const native = requireOptionalNativeModule<AgeRangeNative>('ExpoAgeRange');
  if (!native) return 'unknown';
  try {
    return classifyAgeRange(await native.requestAgeRangeAsync({ threshold1: ADULT_AGE }));
  } catch (e) {
    return classifyAgeError(e);
  }
}
