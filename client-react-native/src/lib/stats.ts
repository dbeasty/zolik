import type { MatchModule } from '@/src/api/matchTypes';
import type { LeaderboardScope, StatsSubject, TallyView } from '@/src/api/types';
import { t } from '@/src/lib/i18n';
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
  if (streak > 1) return t('stats.streak.winsMany', { n: streak });
  if (streak === 1) return t('stats.streak.wonLast');
  if (streak < -1) return t('stats.streak.lossesMany', { n: Math.abs(streak) });
  if (streak === -1) return t('stats.streak.lostLast');
  return t('stats.streak.none');
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

/** A count with its noun, so callers stop writing `n === 1 ? …` inline.
 *  English only — a translated count is a whole phrase per count (see
 *  `playerCountLabel`), since most of the other languages do not pluralise
 *  by appending a letter. */
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
  // Worded the way the lobby words it (see gameLabels.ts): the player's
  // language first, then the server's own label, then the humanised id.
  const fallback = modules.find((m) => m.id === id)?.label ?? humanise(id);
  return t(`module.${id}`, undefined, fallback);
}

/** `4` → `4 players`. The key is a count held as a string. */
export function playerCountLabel(key: string): string {
  const n = Number(key);
  if (!Number.isFinite(n) || n <= 0) return humanise(key);
  return n === 1 ? t('stats.tableSizeOne') : t('stats.tableSizeMany', { n });
}

const SKILL_ORDER = ['easy', 'medium', 'hard', 'ai', 'expert'];

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
  if (!slug) {
    const bare = skillWord(skill);
    return bare ? bare.charAt(0).toUpperCase() + bare.slice(1) : humanise(skill || id);
  }
  return t('stats.botPersona', { name: humanise(slug), skill: skillWord(skill) ?? skill });
}

/** A difficulty in the player's language, or null for one this build has no
 *  word for — which then shows as the server spelled it. */
function skillWord(skill: string): string | null {
  return SKILL_ORDER.includes(skill) ? t(`stats.skill.${skill}`) : null;
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
  if (!subject) return t('stats.unknownPlayer');
  if (subject.name) return subject.name;
  if (subject.kind === 'ai') return aiLabel(subject.id);
  return t('stats.unknownPlayer');
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

/** The leaderboard's scopes, in toggle order. Words are looked up at render
 *  time rather than held here, so a change of language repaints them. */
export const SCOPES: { id: LeaderboardScope; labelKey: string; blurbKey: string }[] = [
  { id: 'overall', labelKey: 'stats.scope.overall', blurbKey: 'stats.scopeNote.overall' },
  { id: 'vs_humans', labelKey: 'stats.scope.vsHumans', blurbKey: 'stats.scopeNote.vsHumans' },
  { id: 'vs_ai', labelKey: 'stats.scope.vsBots', blurbKey: 'stats.scopeNote.vsBots' },
];

/**
 * The two scopes overlap rather than partition — a table with a person and a
 * bot at it counts in both — and that surprises people looking at a board
 * whose numbers do not add up to the overall one. Said once, under the toggle.
 */
export function scopeBlurb(scope: LeaderboardScope): string {
  const found = SCOPES.find((s) => s.id === scope);
  return found ? t(found.blurbKey) : '';
}
