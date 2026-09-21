/**
 * Subject keys — `user:<hex>` and `guest:<id>` — as the circle addresses
 * people.
 *
 * A seat on the wire carries only its player id, and that id *is* the login
 * subject: the server seats a player under the token's user id, which for a
 * guest is the device's guest id. The two are told apart by shape, which the
 * server fixes: a guest id is 32 lowercase hex characters
 * (auth.sanitizeGuestID), an account id a 24-character ObjectID.
 */
const GUEST_ID = /^[0-9a-f]{32}$/;

export function subjectKeyForSeat(playerId: string): string {
  return GUEST_ID.test(playerId) ? `guest:${playerId}` : `user:${playerId}`;
}

/** This player's own key, from their session. */
export function subjectKeyForSession(session: { userId: string; isGuest: boolean }): string {
  return `${session.isGuest ? 'guest' : 'user'}:${session.userId}`;
}
