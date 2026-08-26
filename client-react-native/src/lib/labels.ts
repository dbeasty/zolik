import type { Fact } from '@/src/api/matchTypes';
import { messageTemplate, t } from '@/src/lib/i18n';

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

/** Anyone the server may have named by id rather than by name. */
export type Named = { id: string; name: string };

/**
 * A string shaped like a message key — `a.b`, `holdem.hand.twoPair`.
 *
 * Nothing else a module puts in a fact looks like this: card codes, streets,
 * scores and names have no dots in them. So it is safe to treat a value of
 * this shape as another key to look up, which is what lets a module name one
 * of its own concepts inside a fact instead of shipping a word.
 */
const KEY_SHAPED = /^[a-z][A-Za-z0-9]*(\.[A-Za-z0-9]+)+$/;

/** Whether wording places values of its own, rather than just naming a thing. */
const PLACES_ITS_OWN = /\{\w+\}/;

/**
 * One token the server sent, as a reader should see it.
 *
 * Two kinds of token reach a client opaque: a player id, which means nothing
 * without the match's own player list, and another message key. Both are the
 * same idea — the server names things in its own stable vocabulary and the
 * client turns that into words — so both are resolved in one place, and a
 * list of either becomes a list of names.
 */
function tokenText(value: unknown, players: Named[]): string {
  if (Array.isArray(value)) return value.map((v) => tokenText(v, players)).join(', ');
  if (typeof value !== 'string') return String(value);
  const player = players.find((p) => p.id === value);
  if (player) return player.name;
  return KEY_SHAPED.test(value) ? label(value) : value;
}

function resolveParams(
  params: Record<string, unknown> | undefined,
  players: Named[],
): Record<string, string> | undefined {
  if (!params) return undefined;
  const out: Record<string, string> = {};
  for (const [k, v] of Object.entries(params)) out[k] = tokenText(v, players);
  return out;
}

/**
 * A fact as one line of text.
 *
 * A fact has three parts and this composes them in the one order that reads:
 * its key names the thing, its params say which and how much, and its value is
 * the number or token it carries. Pass the match's players and any id among
 * those params becomes a name — without them a fact whose whole meaning is
 * *whose* renders as a line with the subject missing ("Winner", "Waiting
 * for"), which is what it did before this took the list.
 *
 * Where the bundle has wording that places values itself, that wording is the
 * whole line: it has already said everything the fact carries, including the
 * value under `{value}`, and appending the value after it would say it twice.
 * A key with no wording of its own — most of them, rendered by `humanise` —
 * takes the value after it, which is what reads correctly for every plain fact
 * any of the four modules sends ("Deck 43", "Stack 980").
 */
export function factText(f: Fact, players: Named[] = []): string {
  const params = resolveParams(f.params, players);
  const value =
    f.value === undefined || f.value === '' ? undefined : tokenText(f.value, players);
  const name = label(f.labelKey, value === undefined ? params : { ...params, value });
  const wording = f.labelKey ? messageTemplate(f.labelKey) : undefined;
  if (wording && PLACES_ITS_OWN.test(wording)) return name;
  return value === undefined ? name : `${name} ${value}`;
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

/**
 * The number to print for a standing.
 *
 * One definition, because there are two fields and only one of them is meant
 * for a person. `score` is oriented so higher is always better — that is what
 * lets the server rank four different games and keep one lifetime average per
 * player without knowing which way any of them counts. Rummy counts downwards,
 * so its score arrives negated, and a scoreboard that printed it read
 * "-29 Penalty" at a player who had 29 penalty points.
 */
export function shownScore(s: { score: number; shown?: number }): number {
  return s.shown ?? s.score;
}
