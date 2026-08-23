import React, { createContext, useContext, useEffect, useState } from 'react';

import { apiClient } from '@/src/api/client';
import type { RulesInfo } from '@/src/api/types';

// Mirrors the server defaults (server/internal/rules/config.go,
// profiles.go) so the UI has something sane to render for the one frame
// before GET /rules resolves, and stays usable if that request ever fails.
const FALLBACK_RULES_INFO: RulesInfo = {
  minPlayers: 2,
  maxPlayers: 8,
  initialMeldMinOptions: [35, 50, 70],
  discardDrawMinRoundOptions: [1, 2, 3],
  defaultInitialMeldMinimum: 35,
  defaultDiscardDrawMinRound: 3,
};

const RulesConfigContext = createContext<RulesInfo>(FALLBACK_RULES_INFO);

// Cached at module scope (not just per-provider-instance state) since this
// never changes at runtime and every screen mounting the provider fresh
// shouldn't have to re-fetch it.
let cached: RulesInfo | null = null;

export function RulesConfigProvider({ children }: { children: React.ReactNode }) {
  const [info, setInfo] = useState<RulesInfo>(cached ?? FALLBACK_RULES_INFO);

  useEffect(() => {
    if (cached) return;
    let cancelled = false;
    apiClient
      .getRules()
      .then((fetched) => {
        if (cancelled) return;
        cached = fetched;
        setInfo(fetched);
      })
      .catch(() => {
        // Keep the fallback — GET /rules is unauthenticated and best-effort
        // here; nothing downstream treats this data as load-bearing.
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return <RulesConfigContext.Provider value={info}>{children}</RulesConfigContext.Provider>;
}

export function useRulesConfig(): RulesInfo {
  return useContext(RulesConfigContext);
}
