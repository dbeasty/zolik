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
  initialMeldMinimum: number;
  discardDrawMinRound: number;
  discardDrawnCardPendingMeld?: string;
  canUndoDiscardDraw?: boolean;
  canUndoLayOff?: boolean;
  canUndoLayMeld?: boolean;
  rulesProfile?: RulesProfile;
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
