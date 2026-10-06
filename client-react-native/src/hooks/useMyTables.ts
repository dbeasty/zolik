import { useFocusEffect } from 'expo-router';
import { useCallback, useState } from 'react';

import type { StoredTable } from '@/src/api/matchTypes';
import { useSession } from '@/src/context/SessionContext';

/**
 * The player's own tables, unfinished or finished, refetched whenever the
 * screen comes back into view — the usual way back to a menu is from a table
 * that has just changed whose turn it is.
 *
 * `null` until asked, so "not yet known" never looks like "none": a panel
 * built on this shows nothing rather than flashing an empty state on every
 * visit. A failed fetch settles on `[]`; the full list is one tap away in the
 * account menu, so a quiet failure costs nothing real.
 *
 * `enabled` false skips the request altogether, for panels only a wide window
 * draws.
 */
export function useMyTables(
  scope: 'unfinished' | 'finished',
  enabled = true,
  opts: { turns?: boolean } = {},
): StoredTable[] | null {
  const { client, session } = useSession();
  const [tables, setTables] = useState<StoredTable[] | null>(null);
  const turns = !!opts.turns;

  useFocusEffect(
    useCallback(() => {
      if (!enabled) return undefined;
      if (!session) {
        setTables([]);
        return undefined;
      }
      let cancelled = false;
      client
        .listMyTables(scope, { turns })
        .then((rows) => {
          if (!cancelled) setTables(rows);
        })
        .catch(() => {
          if (!cancelled) setTables([]);
        });
      return () => {
        cancelled = true;
      };
    }, [client, session, scope, turns, enabled]),
  );

  return tables;
}
