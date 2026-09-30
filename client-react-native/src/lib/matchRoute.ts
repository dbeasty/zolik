import type { Href } from 'expo-router';

/**
 * Where a seated player belongs, given a match's own status and whether they
 * host it.
 *
 * The three-way decision `app/join/[code].tsx` makes right after joining, and
 * the one a "my games" row makes when it is tapped, are the same decision:
 * a table that has been dealt goes straight to the board, a lobby sends its
 * host to the table screen and everyone else to the waiting screen. Pulled
 * out here so the two call sites cannot quietly drift onto two different
 * ideas of where a lobby host goes.
 *
 * Cast `as Href` rather than typed at the return: expo-router's typed routes
 * validate an href against its known patterns via a template-literal type,
 * which only sees through an actual template literal at the call site — a
 * value built in one function and returned infers as plain `string` and
 * fails to typecheck no matter how it was built. See the identical cast in
 * `app/index.tsx`'s `useFollowPendingDestination`.
 */
export function routeForMatch(status: string, isHost: boolean, matchId: string): Href {
  if (status !== 'lobby') {
    return `/match/${encodeURIComponent(matchId)}` as Href;
  }
  return (
    isHost
      ? `/lobby/table?matchId=${encodeURIComponent(matchId)}`
      : `/lobby/join?matchId=${encodeURIComponent(matchId)}`
  ) as Href;
}

/**
 * Where a stopped game is stepped through.
 *
 * Deliberately not folded into `routeForMatch`: resuming a table and
 * replaying one are different intentions, and a row offers both. The same
 * `as Href` cast applies, for the reason given above.
 */
export function routeForReplay(matchId: string): Href {
  return `/replay/${encodeURIComponent(matchId)}` as Href;
}
