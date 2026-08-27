import { useEffect, useState } from 'react';

import type { MatchState, RuleItem } from '@/src/api/matchTypes';
import type { ModuleRules } from '@/src/api/matchTypes';
import type { ZolikClient } from '@/src/api/client';
import { useSession } from '@/src/context/SessionContext';
import { logger } from '@/src/lib/logger';

/**
 * The table's own written rules, by id.
 *
 * This is what makes "why can't I do that?" answerable without any rule
 * knowledge on this side. A refusal arrives carrying `ruleIds` — pointers,
 * not sentences — and this resolves them against the rules the server already
 * publishes for the exact variation and options this match is being played
 * under. The sentence a player reads under a greyed-out control is therefore
 * the same object the rules screen prints, in the same locale, with the same
 * numbers in it, and there is no second copy of anything to drift.
 *
 * Fetched once per table and cached at module scope, because the rules cannot
 * change while a match is being played: a reconnect, a re-render or a second
 * screen opening the same match all reuse the one response. Keyed by what
 * actually determines the text — the module, the variation and the options —
 * so two tables of the same game with different house rules do not share an
 * entry.
 *
 * A failure degrades rather than blocking: an empty index means a refusal
 * shows its reason and its remedy with no rule behind it, which is exactly
 * what it did before this existed. It never leaves a control unexplained.
 */

const cache = new Map<string, Map<string, RuleItem>>();
const inFlight = new Map<string, Promise<Map<string, RuleItem>>>();

/** What determines the wording: the game, its variation, and its options. */
function tableKey(state: Pick<MatchState, 'moduleId' | 'variation' | 'options'>): string {
  const opts = Object.entries(state.options ?? {})
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([k, v]) => `${k}=${v}`)
    .join(',');
  return `${state.moduleId}|${state.variation ?? ''}|${opts}`;
}

async function load(
  client: ZolikClient,
  state: Pick<MatchState, 'moduleId' | 'variation' | 'options'>,
): Promise<Map<string, RuleItem>> {
  const key = tableKey(state);
  const cached = cache.get(key);
  if (cached) return cached;

  const running = inFlight.get(key);
  if (running) return running;

  const promise = client
    .moduleRules(state.moduleId, state.variation, state.options)
    .then((rules: ModuleRules) => {
      const index = new Map<string, RuleItem>();
      for (const section of rules.sections ?? []) {
        for (const item of section.items ?? []) {
          if (item.id) index.set(item.id, item);
        }
      }
      cache.set(key, index);
      return index;
    })
    .catch((e: unknown) => {
      // Not rethrown: a table whose rules could not be fetched is still a
      // playable table, and a refusal there is still explained by its reason
      // and its remedy.
      logger.warn('rules', 'rule index unavailable', {
        error: e instanceof Error ? e.message : String(e),
      });
      return new Map<string, RuleItem>();
    })
    .finally(() => inFlight.delete(key));

  inFlight.set(key, promise);
  return promise;
}

/** The rule index for a match, empty until it arrives. */
export function useRuleIndex(
  state: Pick<MatchState, 'moduleId' | 'variation' | 'options'> | null,
): Map<string, RuleItem> {
  const { client } = useSession();
  const [index, setIndex] = useState<Map<string, RuleItem>>(() => new Map());
  const key = state ? tableKey(state) : '';

  useEffect(() => {
    if (!state?.moduleId) return;
    let cancelled = false;
    load(client, state).then((next) => {
      if (!cancelled) setIndex(next);
    });
    return () => {
      cancelled = true;
    };
    // `key` is the whole identity of the request; `state` is a fresh object on
    // every socket frame and would refetch on each one.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [client, key]);

  return index;
}

/** Forget every cached index. For tests, and for a locale change. */
export function clearRuleIndexCache() {
  cache.clear();
  inFlight.clear();
}
