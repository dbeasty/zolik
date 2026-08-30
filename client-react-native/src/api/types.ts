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
/** One human player currently waiting in the lobby to be picked up into a
 *  match — see GET /lobby/waiting and WS /ws/lobby. */
export type WaitingPlayer = {
  playerId: string;
  username: string;
  isGuest: boolean;
  joinedAt: string;
};

/** A push on the /ws/lobby socket. 'lobby_waiting' is the current pool,
 *  broadcast whenever it changes; 'lobby_invited' is personal — it means a
 *  host just seated this player directly into their match. */
export type LobbyWSMessage =
  | { type: 'lobby_waiting'; players: WaitingPlayer[] }
  | { type: 'lobby_invited'; matchId: string; joinCode: string };

/** Admission snapshot from GET /healthz/capacity. */
export type CapacitySnapshot = {
  live: number;
  maxConnections?: number;
  memoryUsedBytes?: number;
  memoryLimitBytes?: number;
  memoryFraction?: number;
  cpuStallFraction?: number;
  accepting: boolean;
  waitingRoomOpen: boolean;
  startingMatches: boolean;
};

/** A signed-in player, however they signed in. */
export type PlayerSession = {
  accessToken: string;
  refreshToken: string;
  userId: string;
  username: string;
  isGuest: boolean;
  /**
   * The device's durable guest identity, present on guest sessions.
   *
   * Stored separately from the session and deliberately *not* cleared on sign
   * out: it is what the server attributes guest play to, so keeping it is what
   * makes "play now, sign in later, keep your statistics" work. It is not a
   * credential and grants no access to any account.
   */
  guestId?: string;
  /** Matches recorded against this device's guest id that an account could
   *  still absorb. Drives the "sign in to keep your N games" prompt. */
  claimableMatches?: number;
};

/** One way of signing in, as advertised by the server.
 *
 *  The list is fetched rather than hardcoded so enabling Apple or Microsoft
 *  server-side lights up the button without shipping a new app build. */
export type AuthProvider = {
  id: string;
  displayName: string;
  /** 'guest' needs no input, 'email' collects an address, 'oauth' opens the
   *  provider in a browser. */
  kind: 'guest' | 'email' | 'oauth';
};

/** A sign-in method attached to the signed-in account. */
export type LinkedIdentity = {
  provider: string;
  email?: string;
  displayName?: string;
  linkedAt: string;
  lastLoginAt?: string;
};

/** The signed-in account, as /users/me reports it. */
export type AccountProfile = {
  id: string;
  username: string;
  email?: string;
  emailVerified: boolean;
  avatarUrl?: string;
  createdAt: string;
  identities: LinkedIdentity[];
  hasPassword: boolean;
  prefs?: { language?: string; cardStyle?: string };
};

/** What a completed sign-in returns, whichever door it came through. */
export type SignInOutcome = {
  session: PlayerSession;
  /** How many guest matches were absorbed into the account. */
  claimedMatches: number;
  /** True when the sign-in created a brand-new account. */
  created: boolean;
};

/** Any framed message off a socket, before it is narrowed by `type`. */
export type WSEnvelope = Record<string, unknown> & { type: string };

/**
 * One bucket of a lifetime record, with the figures derived from it.
 *
 * `bestScore` and `worstScore` are null rather than a sentinel until a match
 * has been played, so a client never renders a placeholder as a real score.
 */
export type TallyView = {
  matches: number;
  wins: number;
  losses: number;
  draws: number;
  scoreSum: number;
  rankSum: number;
  winRate: number;
  avgScore: number;
  avgRank: number;
  bestScore: number | null;
  worstScore: number | null;
};

/**
 * A registered player's lifetime record, as `/users/me/stats` returns it.
 *
 * `vsHumans` and `vsAI` overlap rather than partition: a mixed table counts in
 * both, because the interesting question is whether a person was involved, not
 * whether the table was pure.
 */
export type LifetimeStats = {
  subject?: { kind: string; id: string; name: string };
  overall: TallyView;
  vsHumans: TallyView;
  vsAI: TallyView;
  /**
   * Keyed by game. A Canasta total and a poker stack are not comparable, so an
   * average across both would be noise — this split is what keeps each game's
   * figures meaningful.
   */
  byModule?: Record<string, TallyView>;
  byAIDifficulty?: Record<string, TallyView>;
  byPlayerCount?: Record<string, TallyView>;
  /** Signed: positive for consecutive wins, negative for losses, zero after a draw. */
  currentStreak: number;
  longestWinStreak: number;
  longestLossStreak: number;
};
