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
  /** The face they are waiting under — the one they keep if picked up. */
  avatar?: string;
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
  prefs?: { language?: string; cardStyle?: string; avatar?: string };
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

// --- notifications: the game circle and invites ----------------------------
//
// Transcribed from docs/notifications-plan.md, which is the wire contract the
// server's internal/notify package implements. Everyone is addressed by a
// subject key, `user:<hex>` or `guest:<id>` — the same keys the match records
// carry — so a guest can be in a circle as fully as an account can.

/** Who may reach a player: everyone in their circle, or nobody. */
export type InvitePreference = 'circle' | 'off';

/** A player's own notification settings, from GET/PATCH /notify/me. */
export type NotifyProfile = {
  key: string;
  /** The code behind this player's friend link, `/add/<code>`. */
  friendCode: string;
  /** The whole link, when the server knows its public address. */
  friendUrl?: string;
  invites: InvitePreference;
  nearby: boolean;
};

/** One person on the circle screen, in whichever of its lists. */
export type CircleEntry = {
  key: string;
  name: string;
  avatar?: string;
  /** 'pending' is a username request the other side has not accepted yet. */
  status: 'active' | 'pending';
  /** ISO time the relationship began. */
  since: string;
  /** Notifiers only: this player has silenced them. */
  muted?: boolean;
};

/**
 * GET /notify/circle.
 *
 * `members` are the people this player tells about their tables; `notifiers`
 * are the people who tell this player about theirs; `requests` are notifiers
 * asking to become one, by username, and waiting for an answer.
 */
export type CircleLists = {
  members: CircleEntry[];
  requests: CircleEntry[];
  notifiers: CircleEntry[];
};

/** Somebody this player has shared a table with, not yet in their circle. */
export type CircleSuggestion = {
  key: string;
  name: string;
  avatar?: string;
  lastPlayedAt: string;
  matches: number;
};

/** The public face of a friend link, shown before anything is added. */
export type FriendPreview = { name: string; avatar?: string };

/** What the server says about push on this deployment. */
export type NotifyConfig = { vapidPublicKey: string | null; expo: boolean };

/** A device registration for OS push. */
export type PushDeviceRegistration = {
  kind: 'expo' | 'webpush';
  token?: string;
  subscription?: unknown;
  platform: string;
  locale: string;
};

/** A table somebody in this player's circle has opened, as the server sends it. */
export type TableInvite = {
  /** Equal to `matchId`: one invite per table, which is what de-duplicates the
   *  socket's copy against the push's. */
  id: string;
  matchId: string;
  joinCode: string;
  moduleId: string;
  /** The game's own name for itself, for when this build has no words for
   *  `moduleId` — game names are mostly left unkeyed (see gameLabels.ts). */
  moduleLabel?: string;
  variation?: string;
  host: { key: string; name: string; avatar?: string };
  sentAt: string;
};

/** Everything the personal socket, /ws/me, can say. */
export type MeWSMessage =
  | { type: 'table_invite'; invite: TableInvite }
  | { type: 'invite_revoked'; id: string }
  | { type: 'lobby_invited'; matchId: string; joinCode: string }
  | { type: 'circle_changed' };

/** What the cloud answers when a device enrols as a node of the database. */
export type NodeEnrolment = {
  nodeId: string;
  /** How this device authenticates its database sync from now on. */
  credential: string;
};

/** One offline seat a person claimed, as the cloud recorded it. */
export type ClaimedSeat = {
  guestId: string;
  nodeId: string;
  claimed: boolean;
  /** Why not, when it was not: the seat belongs to somebody else. */
  reason?: string;
};
