import React, { createContext, useContext, useMemo, useState } from 'react';

import type { GameState, WSEnvelope } from '@/src/api/types';

type GameFlowContextValue = {
  roundEnd: { data: WSEnvelope; state: GameState; gameId: string } | null;
  setRoundEnd: (v: GameFlowContextValue['roundEnd']) => void;
  gameEnd: { data: WSEnvelope; state: GameState } | null;
  setGameEnd: (v: GameFlowContextValue['gameEnd']) => void;
};

const GameFlowContext = createContext<GameFlowContextValue | null>(null);

export function GameFlowProvider({ children }: { children: React.ReactNode }) {
  const [roundEnd, setRoundEnd] = useState<GameFlowContextValue['roundEnd']>(null);
  const [gameEnd, setGameEnd] = useState<GameFlowContextValue['gameEnd']>(null);
  const value = useMemo(
    () => ({ roundEnd, setRoundEnd, gameEnd, setGameEnd }),
    [roundEnd, gameEnd],
  );
  return (
    <GameFlowContext.Provider value={value}>{children}</GameFlowContext.Provider>
  );
}

export function useGameFlow() {
  const ctx = useContext(GameFlowContext);
  if (!ctx) {
    throw new Error('useGameFlow must be used within GameFlowProvider');
  }
  return ctx;
}
