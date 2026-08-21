/**
 * Wording for the server's stable error-code vocabulary.
 *
 * Deliberately separate from src/lib/offers.ts: that module is pure lookup
 * over what the server decided and holds no strings at all, while this one
 * holds nothing but strings. The codes are owned by the engine
 * (rules.RulesErrorCode); only the phrasing lives here — which is exactly the
 * seam a locale bundle replaces in Phase 2 of docs/extensibility-plan.md,
 * with no change to any caller.
 */

const MESSAGES: Record<string, string> = {
  NOT_YOUR_TURN: "It's not your turn",
  WRONG_PHASE: 'Not available right now',
  GAME_SUSPENDED: 'The game is paused',
  GAME_NOT_ACTIVE: 'The game is not running',
  DISCARD_LOCKED: 'The discard pile is locked for now',
  DISCARD_PILE_EMPTY: 'The discard pile is empty',
  NO_CARDS_LEFT: 'No cards left to draw',
  ROUND_REQ_NOT_MET: 'Lay your own initial meld first',
  INCOMPLETE_INITIAL_MELD: 'Finish your initial meld first',
  DISCARD_CARD_NOT_MELDED: 'The card you picked up must go into your meld',
  JOKER_DISCARD_FORBIDDEN: "A joker can't be discarded",
  NOTHING_TO_UNDO: 'Nothing to undo',
  NO_JOKER_IN_MELD: 'No joker in this meld',
  JOKER_SWAP_MISMATCH: "That card doesn't take the joker's place",
  BREAKS_CLEAN_RUN: 'That run has to stay joker-free',
  WRONG_RUN_END: 'That card extends the other end of the run',
  INVALID_MELD: 'No card in your hand fits here',
  CARD_NOT_IN_HAND: 'That card is not in your hand',
};

/**
 * Player-facing text for an engine error code. An unknown code falls through
 * to `fallback` rather than rendering a raw SCREAMING_SNAKE token — a server
 * that adds a code must never leak it into the UI.
 */
export function reasonText(code: string | undefined, fallback = ''): string {
  if (!code) return fallback;
  return MESSAGES[code] ?? fallback;
}
