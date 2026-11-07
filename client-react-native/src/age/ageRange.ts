/**
 * What the device says about the player's age, reduced to the one question a
 * store build asks of it: may this player see the games played by staking
 * chips (`MatchModule.simulatedGambling`)?
 *
 * - `adult`: the OS reports 18 or over.
 * - `minor`: the OS reports under 18, or the player declined to share their
 *   age (Apple's `declinedSharing`). Declining is read as under 18 because
 *   Apple asks apps to treat it as no answer, and the cautious no-answer for
 *   a betting game is "not for you".
 * - `unknown`: there is nothing to go on — the web, iOS before 26, an
 *   Android device outside Google Play's age-signals regions, or an error.
 *   The games stay, with their 18+ notice: the store rates the whole app
 *   18+, so a device with no age signal is left to that rating.
 * - `pending`: still asking.
 */
export type AgeStatus = 'pending' | 'adult' | 'minor' | 'unknown';

/** The fields of expo-age-range's `AgeRangeResponse` that decide it. */
export type AgeRangeLike = {
  lowerBound?: number | null;
  upperBound?: number | null;
  userStatus?: string | null;
};

export const ADULT_AGE = 18;

export function classifyAgeRange(r: AgeRangeLike): AgeStatus {
  // Google Play: a verified adult may come back with no bounds at all.
  if (r.userStatus === 'VERIFIED') return 'adult';
  if (r.lowerBound != null && r.lowerBound >= ADULT_AGE) return 'adult';
  if (r.upperBound != null && r.upperBound < ADULT_AGE) return 'minor';
  if (r.lowerBound != null && r.lowerBound < ADULT_AGE) return 'minor';
  // A supervised Google account is a child's, bounds or not.
  if (r.userStatus?.startsWith('SUPERVISED')) return 'minor';
  return 'unknown';
}

/** expo-age-range rejects with this code when the player declines on iOS. */
export const DECLINED_CODE = 'ERR_AGE_RANGE_USER_DECLINED';

export function classifyAgeError(e: unknown): AgeStatus {
  const code = (e as { code?: unknown } | null)?.code;
  return code === DECLINED_CODE ? 'minor' : 'unknown';
}

/**
 * A build-time override, for the simulator and e2e: the iOS simulator cannot
 * answer an age-range request, and neither can a browser. Production builds
 * never set it.
 */
export function overrideStatus(value: string | undefined): AgeStatus | null {
  return value === 'minor' || value === 'adult' || value === 'unknown' ? value : null;
}
