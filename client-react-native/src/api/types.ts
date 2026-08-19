export type GameState = {
  type: string;
  status: string;
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
  deckDrawMinRound: number;
  offer?: Offer;
};

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

export type Offer = {
  card: string;
};

export type LobbyGame = {
  id: string;
  status: string;
  round: number;
  phase: string;
  currentTurn: string;
  players: LobbyPlayer[];
  hostId?: string;
  initialMeldMinimum?: number;
  discardDrawMinRound?: number;
  deckDrawMinRound?: number;
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
};

export type PlayerSession = {
  accessToken: string;
  refreshToken: string;
  userId: string;
  username: string;
  isGuest: boolean;
};

export type WSEnvelope = Record<string, unknown> & { type: string };
