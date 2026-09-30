import { useEffect, useState } from 'react';

import { useSession } from '@/src/context/SessionContext';

export type ServerBuild = { version: string; commit: string };

/**
 * The build the server answered with, fetched once on mount.
 *
 * Not polled: unlike waiting-room status this cannot change without a full
 * reload of both halves. A failed fetch (unreachable server, cold start)
 * resolves to `null` rather than an error — every caller shows this beside
 * the app's own build in quiet type, and a server that is briefly down must
 * not turn that into something a player has to dismiss.
 *
 * Shared by the main menu's footer and the About screen, which ask the same
 * question and used to answer it with two copies of this effect.
 *
 * It asks whichever server the session is talking to. At an offline table
 * that is the phone's own copy, and its build is the one worth reporting.
 */
export function useServerBuild(): ServerBuild | null {
  const { client } = useSession();
  const [server, setServer] = useState<ServerBuild | null>(null);

  useEffect(() => {
    let cancelled = false;
    setServer(null);
    client
      .getVersion()
      .then((build) => {
        if (!cancelled) setServer(build);
      })
      .catch(() => {
        // Best-effort only — see the comment above.
      });
    return () => {
      cancelled = true;
    };
  }, [client]);

  return server;
}
