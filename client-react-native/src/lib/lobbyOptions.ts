/**
 * Pure helpers for a lobby form driven by the module descriptor.
 *
 * Same invariant as src/lib/offers.ts: **no rule knowledge**. Nothing here
 * names a profile, a meld minimum, or a lock round. Every value comes from the
 * descriptor the server served, which is what makes adding a knob or a
 * variation a server-only change (docs/extensibility-plan.md Phase 2.1).
 */

import type { OptionSpec, ProfileSpec } from '@/src/api/types';

/** Option values keyed by the descriptor's own option names. */
export type OptionValues = Record<string, number>;

/**
 * The starting values for a profile: each option read off that profile's
 * resolved ruleset, by the same name the option declares.
 *
 * That name correspondence is not a convention this file hopes for — it is
 * asserted server-side by TestDescriptor_ProfileDefaultsAreThemselvesLegalOptions,
 * which fails if a profile ships a default the option schema does not allow.
 */
export function defaultsFor(
  profile: ProfileSpec | undefined,
  options: OptionSpec[],
): OptionValues {
  const out: OptionValues = {};
  if (!profile) return out;
  const rules = profile.rules as unknown as Record<string, unknown>;
  for (const o of options) {
    const v = rules[o.name];
    if (typeof v === 'number') out[o.name] = v;
  }
  return out;
}

/**
 * How to label a value. Falls back to the raw number rather than rendering
 * nothing, so a server that adds a choice this build has no label for still
 * shows something truthful.
 */
export function labelFor(option: OptionSpec, value: number | undefined): string {
  if (value == null) return '—';
  return option.choices.find((c) => c.value === value)?.label ?? String(value);
}

/**
 * The next value when the chip is tapped, wrapping around. A value that is not
 * in the list (a stale setting, or one the server has since retired) starts
 * from the beginning rather than sticking.
 */
export function nextChoice(option: OptionSpec, current: number | undefined): number {
  if (option.choices.length === 0) return current ?? 0;
  const idx = option.choices.findIndex((c) => c.value === current);
  return option.choices[(idx + 1) % option.choices.length].value;
}

/**
 * Restores a previously chosen value only if the server still declares it.
 * A variation or a setting that has been retired must not come back from this
 * device's storage — the descriptor is the authority, not the cache.
 */
export function restoreChoice(option: OptionSpec, stored: string | null): number | undefined {
  if (stored == null) return undefined;
  const parsed = Number(stored);
  if (!Number.isFinite(parsed)) return undefined;
  return option.choices.some((c) => c.value === parsed) ? parsed : undefined;
}

/**
 * Storage key for one option's last value, scoped to the variation it was
 * chosen under.
 *
 * Derived from the option name, never enumerated — and per profile, because
 * each variation has its own sensible default (Continental starts at a
 * 35-point floor with pickup locked to round 3; Žolík Classic at neither), so
 * a value picked under one must not resurface in a game created under
 * another.
 */
export const lastOptionKey = (name: string, profileId: string) =>
  `zolik_last_opt_${name}:${profileId}`;
