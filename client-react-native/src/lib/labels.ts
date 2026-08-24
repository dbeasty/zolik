import type { Fact } from '@/src/api/matchTypes';
import { t } from '@/src/lib/i18n';

/**
 * Rendering the server's message keys, including ones this build has never
 * seen.
 *
 * The server ships keys, never sentences — that is what makes a Czech UI a
 * client-only change. But a *generic* shell meets a new problem the Žolíky
 * screen never had: a module added after this build shipped will send keys no
 * bundle contains, and `t()`'s last fallback is the raw key. Printing
 * `holdem.seat.stack` at a player is only marginally better than printing
 * nothing.
 *
 * So there is one more fallback step: turn an untranslated key into readable
 * English by its own structure. It is not a translation and does not pretend to
 * be — it is what keeps a new game legible on an old client until somebody
 * writes the words. Anything actually in the bundle wins, always.
 */

/** `holdem.seat.stack` → `Stack`; `zone.drawPile` → `Draw pile`. */
export function humanise(key: string): string {
  const last = key.split('.').pop() ?? key;
  const spaced = last
    .replace(/([a-z0-9])([A-Z])/g, '$1 $2')
    .replace(/[_-]+/g, ' ')
    .trim();
  if (!spaced) return key;
  return spaced.charAt(0).toUpperCase() + spaced.slice(1).toLowerCase();
}

/** A message key rendered for display, falling back to its own shape. */
export function label(key: string | undefined, params?: Record<string, unknown>): string {
  if (!key) return '';
  return t(key, params as Record<string, string | number> | undefined, humanise(key));
}

/**
 * A fact as one line of text.
 *
 * The value goes after the label, which reads correctly for every fact any of
 * the four modules currently sends ("Pot 120", "Deck 43", "Stack 980") and is
 * the shape a label key is written for.
 */
export function factText(f: Fact): string {
  const name = label(f.labelKey, f.params);
  if (f.value === undefined || f.value === '') return name;
  return `${name} ${f.value}`;
}

/**
 * A player's display name, from the match's own player list.
 *
 * Falls back to the id, which is ugly but unambiguous — better than an empty
 * seat where a name should be.
 */
export function playerName(players: { id: string; name: string }[], id: string): string {
  return players.find((p) => p.id === id)?.name || id;
}
