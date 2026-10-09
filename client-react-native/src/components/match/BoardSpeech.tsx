import { createContext, useContext, type ReactNode } from 'react';

import type { MatchPlayer, Zone } from '@/src/api/matchTypes';

/**
 * What a piece of the board needs to know to say itself — who the players
 * are, who is looking, and what else is on the table — without every zone,
 * seat and pile being handed three more props by every parent on the way
 * down. "Anna's run" needs Anna's name; "fits the discard pile" needs to
 * know what the discard pile is called.
 *
 * Read-only and per board: `BoardLayout` provides it, for the live table and
 * for a replay alike. A component drawn outside one (a test, a lobby
 * preview) gets an empty board and says less, never something wrong.
 */
export type BoardSpeech = { players: MatchPlayer[]; viewerId: string; zones: Zone[] };

const EMPTY: BoardSpeech = { players: [], viewerId: '', zones: [] };

const Ctx = createContext<BoardSpeech>(EMPTY);

export function BoardSpeechProvider({ value, children }: { value: BoardSpeech; children: ReactNode }) {
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useBoardSpeech(): BoardSpeech {
  return useContext(Ctx);
}
