import type { MatchModule } from '@/src/api/matchTypes';
import type { LeaderboardScope, StatsSubject, TallyView } from '@/src/api/types';
import { humanise } from '@/src/lib/labels';

/**
 * Turning a lifetime record into the words and numbers a screen shows.
 *
 * Kept apart from `app/stats.tsx` for the usual reason: this is where the
 * decisions live — which splits are worth a row, what an empty bucket reads
 * as, how a persona key becomes a name — and decisions are the part worth
 * testing. The screen below it only lays out what these functions return.
 *
 * One rule runs through all of it: **a bucket with no matches in it is not a
 * zero, it is an absence.** A player who has never sat at a 6-handed table has
 * not lost every 6-handed game, and printing "0%" at them says they have. So
 * empty buckets are dropped from the splits rather than rendered flat, and the
 * figures derived from them (win rate, average score) are null rather than 0.
 */

/** Whether a bucket describes anything that actually happened. */
export function played(tally: TallyView | undefined): tally is TallyView {
  return !!tally && tally.matches > 0;
}

/**
 * A win rate as a whole percentage, or null when nothing has been played.
 *
 * Null rather than 0 so a caller has to decide what an absence looks like;
 * see the note above about what "0%" claims.
 */
export function winPercent(tally: TallyView | undefined): number | null {
  if (!played(tally)) return null;
  return Math.round(tally.winRate * 100);
}

/** `winPercent` as text, with the em dash the screens use for "nothing yet". */
export function winPercentText(tally: TallyView | undefined): string {
  const pct = winPercent(tally);
  return pct === null ? '—' : `${pct}%`;
}

/**
 * The signed current streak, in words.
 *
 * The server's sign convention is the whole of the logic here: positive counts
 * consecutive wins, negative consecutive losses, and zero means the last match
 * was a draw (or there has not been one).
 */
export function streakText(streak: number): string {
  if (streak > 1) return `${streak} wins in a row`;
  if (streak === 1) return 'Won the last one';
  if (streak < -1) return `${Math.abs(streak)} losses in a row`;
  if (streak === -1) return 'Lost the last one';
  return 'No streak';
}

/**
 * An average finishing position, rounded to one decimal.
 *
 * Worth showing next to a win rate because it is the only placing figure that
 * survives a change of table size: winning a heads-up match and winning a
 * six-handed one are both "100%", and only the average rank tells them apart.
 */
export function avgRankText(tally: TallyView | undefined): string {
  if (!played(tally)) return '—';
  return tally.avgRank.toFixed(1);
}

/** A count with its noun, so callers stop writing `n === 1 ? …` inline. */
export function plural(n: number, one: string, many = `${one}s`): string {
  return `${n} ${n === 1 ? one : many}`;
}

/**
 * A game's display name, from the module list the lobby already fetches.
 *
 * The fallback matters more than it looks: `byModule` is keyed by whatever
 * module recorded the match, including one that has since been unregistered
 * and so is missing from `/modules` entirely. Those matches still happened and
 * still count, so the row is kept and the id is humanised rather than the
 * whole bucket being dropped for want of a label.
 */
export function moduleLabel(id: string, modules: MatchModule[]): string {
  return modules.find((m) => m.id === id)?.label ?? humanise(id);
}

/** `4` → `4 players`. The key is a count held as a string. */
export function playerCountLabel(key: string): string {
  const n = Number(key);
  return Number.isFinite(n) && n > 0 ? plural(n, 'player') : humanise(key);
}

const SKILL_ORDER = ['easy', 'medium', 'hard', 'expert'];

/**
 * A bot subject id as a person would say it.
 *
 * An id is a persona key — `hard:miroslav` — or, for a bot seated before
 * personas existed, a bare difficulty. Both shapes reach a client, from rows
 * written years apart, so both are handled here rather than at three call
 * sites: `hard:miroslav` → `Miroslav (hard)`, `hard` → `Hard`.
 */
export function aiLabel(id: string): string {
  const [skill, slug] = id.split(':');
  if (!slug) return humanise(skill || id);
  return `${humanise(slug)} (${skill})`;
}

/** Weakest first, unknown strengths last — the order a difficulty table reads
 *  in, rather than the order the store happened to return. */
export function skillRank(id: string): number {
  const i = SKILL_ORDER.indexOf(id.split(':')[0]);
  return i === -1 ? SKILL_ORDER.length : i;
}

/**
 * How a subject is named on screen.
 *
 * `name` is a snapshot from the last match that touched the row, so it can be
 * missing on rows written before it was recorded. A bot falls back to its own
 * id, which is meaningful; a person falls back to a placeholder, because a raw
 * account ObjectID is not a name and showing one is worse than admitting the
 * name is gone.
 */
export function subjectName(subject: StatsSubject | undefined): string {
  if (!subject) return 'Unknown player';
  if (subject.name) return subject.name;
  if (subject.kind === 'ai') return aiLabel(subject.id);
  return 'Unknown player';
}

/** One labelled bucket, ready to render. */
export type Split = { key: string; label: string; tally: TallyView };

/**
 * The non-empty buckets of a keyed split, in `order`.
 *
 * Empty buckets are dropped here rather than by every caller — see the note at
 * the top of the file about what a flat zero row claims.
 */
export function splits(
  buckets: Record<string, TallyView> | undefined,
  labelOf: (key: string) => string,
  order?: (a: string, b: string) => number,
): Split[] {
  const present = buckets ?? {};
  const keys = Object.keys(present).filter((k) => played(present[k]));
  keys.sort(order ?? ((a, b) => labelOf(a).localeCompare(labelOf(b))));
  return keys.map((key) => ({ key, label: labelOf(key), tally: present[key] }));
}

/** Difficulty buckets, weakest first. */
export function difficultySplits(buckets: Record<string, TallyView> | undefined): Split[] {
  return splits(buckets, aiLabel, (a, b) => skillRank(a) - skillRank(b) || a.localeCompare(b));
}

/** Table-size buckets, smallest table first. */
export function playerCountSplits(buckets: Record<string, TallyView> | undefined): Split[] {
  return splits(buckets, playerCountLabel, (a, b) => Number(a) - Number(b));
}

export const SCOPES: { id: LeaderboardScope; label: string; blurb: string }[] = [
  { id: 'overall', label: 'Overall', blurb: 'Every match, whoever was at the table.' },
  { id: 'vs_humans', label: 'vs humans', blurb: 'Matches with at least one other person in them.' },
  { id: 'vs_ai', label: 'vs bots', blurb: 'Matches with at least one bot in them.' },
];

/**
 * The two scopes overlap rather than partition — a table with a person and a
 * bot at it counts in both — and that surprises people looking at a board
 * whose numbers do not add up to the overall one. Said once, under the toggle.
 */
export function scopeBlurb(scope: LeaderboardScope): string {
  return SCOPES.find((s) => s.id === scope)?.blurb ?? '';
}
