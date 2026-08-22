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
  /** @deprecated read the `draw:discard` offer's `whyNot`. */
  discardLocked?: boolean;
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

/**
 * The server's verdict on a candidate meld, in reply to a `preview_meld`
 * frame. Read-only: nothing is persisted or broadcast.
 *
 * `valid` answers "are these a meld?"; `playable` answers "may I lay them
 * now?" — a valid set is unplayable on someone else's turn, and saying which
 * is more useful than one greyed-out button. `whyNotPlayable` is always
 * populated when `playable` is false, so a caller reads one field.
 */
export type MeldPreview = {
  type: string;
  cards: string[];
  valid: boolean;
  meldType?: string;
  naturalValue: number;
  wildCount: number;
  whyNot?: string;
  playable: boolean;
  whyNotPlayable?: string;
  initialMeldMinimum: number;
  meetsMinimum: boolean;
};

/**
 * The game module's self-description, served by GET /module.
 *
 * A client renders its whole new-game form from this rather than carrying its
 * own copy of the option space (which used to live in the lobby screen as
 * `MELD_MINS = [0, 35, 50, 70]` and `DISCARD_LOCK_ROUNDS = [0, 1, 2, 3]`, next
 * to a hardcoded profile list). Adding a knob or a variation is now a
 * server-only change — see docs/extensibility-plan.md Phase 2.1.
 */
export type ModuleDescriptor = {
  id: string;
  label: string;
  minPlayers: number;
  maxPlayers: number;
  profiles: ProfileSpec[];
  options: OptionSpec[];
};

/** One shipped variation, with the ruleset it starts from already resolved. */
export type ProfileSpec = {
  id: string;
  label: string;
  rules: ResolvedRules;
  /** What this variation asks for to go down on its first deal. */
  contract: Contract;
};

export type OptionSpec = {
  /** Matches the JSON field the client sends back on create/settings. */
  name: string;
  type: 'enum_int' | string;
  label: string;
  help?: string;
  choices: OptionChoice[];
};

export type OptionChoice = {
  value: number;
  label: string;
};

/**
 * Lobby option values keyed by the descriptor's option names. Deliberately
 * open: the set of knobs is declared by the server, so enumerating them here
 * would reintroduce exactly the duplication Phase 2.1 removed.
 */
export type GameOptions = Record<string, number | undefined>;

/**
 * Response shape of GET /rules.
 *
 * @deprecated a strict subset of ModuleDescriptor (GET /module), which also
 * carries each variation's resolved ruleset and every label. The server
 * projects this from the descriptor, so the two cannot disagree.
 */
export type RulesInfo = {
  minPlayers: number;
  maxPlayers: number;
  initialMeldMinOptions: number[];
  discardDrawMinRoundOptions: number[];
  defaultInitialMeldMinimum: number;
  defaultDiscardDrawMinRound: number;
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
  /**
   * discard only: which of `card`'s server hand slots this is, so a
   * duplicate value (two decks in play) is disambiguated by position
   * rather than value alone.
   */
  cardIndex?: number;
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
