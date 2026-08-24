/**
 * What is left of the client's own types.
 *
 * This file used to be 260 lines: `GameState` with twenty-four rummy-named
 * fields, `ResolvedRules`, `Contract`, `MeldPreview`, `MeldMeta`,
 * `RulesProfile`, `LobbyGame`, `WSAction` — one shape per idea in one game.
 * Every one of them described Žolíky, which is why a second game needed a
 * second client.
 *
 * The board, the offers and the scoreboard now live in `matchTypes.ts`, which
 * is a transcription of the server's game-agnostic protocol and contains no
 * game's vocabulary at all. What remains here is the session: who is signed
 * in, which is the one thing that is neither a game nor a board.
 */

/** A signed-in player, however they signed in. */
export type PlayerSession = {
  accessToken: string;
  refreshToken: string;
  userId: string;
  username: string;
  isGuest: boolean;
};

/** Any framed message off a socket, before it is narrowed by `type`. */
export type WSEnvelope = Record<string, unknown> & { type: string };
