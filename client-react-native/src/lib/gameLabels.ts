import { t } from '@/src/lib/i18n';

/**
 * The words the *server* chose, rendered in the player's language.
 *
 * `/modules` is the one place the "server ships keys, never sentences" rule
 * does not hold: a module describes itself with a `label` per game, per
 * variation, per option and per choice, and those labels are English. So the
 * lobby stayed English no matter what the picker said — the single largest
 * mixed-language screen in the app, and the one every player passes through
 * on the way to a game.
 *
 * Fixing it server-side would mean a server change and a migration of every
 * module's metadata. This does it client-side instead, the same way
 * `src/lib/labels.ts` already handles a message key an old build has never
 * seen: derive a key from the ids, look it up, and **fall back to the label
 * the server sent**.
 *
 * That fallback is what makes this safe to ship against a server that is
 * ahead of the app. A game added tomorrow, an option added next week, a
 * choice nobody has translated — each still reads as the English the server
 * intended rather than as a raw key, and gets its words whenever somebody
 * writes them. Nothing here has to be kept in step with the server to avoid
 * breaking; it only has to be kept in step to be complete.
 *
 * Keys deliberately absent: game names, place names, ratios and bare numbers.
 * "Texas Hold'em", "Vegas Strip", "3:2" and "500" are the same in every
 * language, and a key for each would be a row of duplicated strings in
 * twenty-four files for no reader's benefit. They fall through to the
 * server's label, which is already right.
 */

/** A game's name, as shown in the lobby list. */
export function moduleLabel(mod: { id: string; label: string }): string {
  return t(`module.${mod.id}`, undefined, mod.label);
}

/** One of a game's rule sets — "Classic", "Oklahoma", "Single deck". */
export function variationLabel(moduleId: string, v: { id: string; label: string }): string {
  return t(`variation.${moduleId}.${v.id}`, undefined, v.label);
}

/**
 * A table setting's name.
 *
 * Keyed by the option's `name` alone rather than by module: `handSize` means
 * "cards dealt" in every game that has it, and duplicating it per module
 * would be twenty-four files of the same word — and an invitation for the
 * same setting to end up worded two different ways.
 */
export function optionLabel(opt: { name: string; label: string }): string {
  return t(`option.${opt.name}`, undefined, opt.label);
}

/** One value a setting can take — "Easy", "Off", "Late surrender". */
export function choiceLabel(
  optionName: string,
  choice: { value: number | string; label: string },
): string {
  return t(`choice.${optionName}.${choice.value}`, undefined, choice.label);
}
