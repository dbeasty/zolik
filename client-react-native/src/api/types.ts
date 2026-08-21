export type GameState = {
  type: string;
  status: string;
  /** Which deal of the match this is (1-7); drives the initial-meld pattern. */
  game: number;
  /** Laps around the table within the current deal; gates discardDrawMinRound. */
  round: number;
  phase: string;
  currentTurn: string;
  myHand: string[];
  discardPile: string[];
  deckCount: number;
  reshuffleCount: number;
  cardCounts: Record<string, number>;
  melds: Record<string, string[][]>;
  meldMeta: Record<string, MeldMeta[]>;
  players: Player[];
  roundReqMet: Record<string, boolean>;
  totalScores: Record<string, number>;
  winnerId?: string;
  isDraw?: boolean;
  discardDrawnCardPendingMeld?: string;
  rulesProfile?: RulesProfile;

  /**
   * What this player may do right now, decided by the server's ruleset.
   * Read it through src/lib/offers.ts rather than re-deriving legality from
   * the raw fields above — see docs/extensibility-plan.md Phase 1.
   */
  legalActions?: ActionOffer[];
  /** The game's resolved ruleset. Read these instead of switching on rulesProfile. */
  rules?: ResolvedRules;
  /** What this player must lay down to go down, resolved for the current deal. */
  contract?: Contract;

  /** @deprecated derived from `legalActions`; read the offers instead. */
  initialMeldMinimum: number;
  /** @deprecated derived from `rules.discardDrawMinRound`. */
  discardDrawMinRound: number;
  /** @deprecated read the `undo:*` offers. */
  canUndoDiscardDraw?: boolean;
  /** @deprecated read the `undo:*` offers. */
  canUndoLayOff?: boolean;
  /** @deprecated read the `undo:*` offers. */
  canUndoLayMeld?: boolean;
  /** @deprecated read the `undo:*` offers. */
  canUndoTurn?: boolean;
};

/** One affordance the server is offering. Mirrors rules.ActionOffer. */
export type ActionOffer = {
  id: string;
  verb: 'draw' | 'lay_meld' | 'lay_off' | 'swap_joker' | 'discard' | 'undo';
  enabled: boolean;
  /** Engine error code explaining a disabled offer — a stable key, not a sentence. */
  whyNot?: string;
  source?: Selector;
  target?: Selector;
};

export type Selector = {
  zone: 'hand' | 'deck' | 'discard_pile' | 'meld' | 'table';
  ownerId?: string;
  meldId?: string;
  /** Individually-eligible cards in `zone`; always mirrors `placements` when both are set. */
  cards?: string[];
  placements?: Placement[];
  /** Bounds a shape offer (lay_meld), where enumerating every legal combination isn't feasible. */
  minCards?: number;
  maxCards?: number;
};

/**
 * One card an offer accepts, plus which end(s) of a run it may extend.
 * Empty/absent `positions` means "send no position" — either the meld has no
 * ends (a set) or the placement is unambiguous.
 */
export type Placement = {
  card: string;
  positions?: string[];
};

/** The game's resolved ruleset. Mirrors the server's RulesMsg. */
export type ResolvedRules = {
  profile: string;
  dealSize: number;
  minSetSize: number;
  minRunSize: number;
  initialMeldMinimum: number;
  discardDrawMinRound: number;
  discardPickupMode: 'top_only' | 'any_from_pile' | string;
  jokerDiscardRestricted: boolean;
  fixedDealCount: number;
  matchEndMode: 'after_deals' | 'at_score' | string;
  targetScore: number;
};

export type Contract = {
  sets: number;
  runs: number;
  requireCleanRun: boolean;
};

/** "continental" | "zolik_classic" | "custom" — see server rules.RulesConfig. */
export type RulesProfile = 'continental' | 'zolik_classic' | 'custom';

export type MeldMeta = {
  meldId: string;
  type: string;
};

export type Player = {
  id: string;
  name: string;
  isAI: boolean;
  aiDifficulty?: string;
};

export type LobbyGame = {
  id: string;
  status: string;
  game: number;
  round: number;
  phase: string;
  currentTurn: string;
  players: LobbyPlayer[];
  hostId?: string;
  rulesProfile?: RulesProfile;
  initialMeldMinimum?: number;
  discardDrawMinRound?: number;
  discardPileTop?: unknown;
};

export type LobbyPlayer = {
  id: string;
  name: string;
  isAI: boolean;
};

export type WSAction = {
  type: string;
  from?: string;
  cards?: string[];
  meldId?: string;
  card?: string;
  /** lay_off only: which end of a run the dropped card(s) must extend. */
  position?: 'front' | 'end';
};

export type PlayerSession = {
  accessToken: string;
  refreshToken: string;
  userId: string;
  username: string;
  isGuest: boolean;
};

export type WSEnvelope = Record<string, unknown> & { type: string };
