/** Exponential backoff with jitter to avoid synchronized reconnect storms. */
export function jitteredBackoff(attempt: number, baseMs = 1000, maxMs = 10_000): number {
  const capped = Math.min(baseMs * 2 ** attempt, maxMs);
  return capped + Math.floor(Math.random() * capped * 0.25);
}

/** Longer backoff when the server reports it is full. */
export function busyBackoff(attempt: number): number {
  return jitteredBackoff(attempt, 30_000, 120_000);
}
