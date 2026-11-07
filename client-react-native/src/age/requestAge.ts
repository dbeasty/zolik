import type { AgeStatus } from './ageRange';

/**
 * The web has nothing to ask: no browser tells a site its visitor's age. The
 * games stay, each with its 18+ notice.
 */
export async function requestAgeStatus(): Promise<AgeStatus> {
  return 'unknown';
}
