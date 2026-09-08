import { messageTemplate, t } from '@/src/lib/i18n';

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
 * Tried module-first, then bare. `handSize` means "cards dealt" in every game
 * that has it, so the bare key carries it once instead of in seven copies —
 * but a module that means something different by the same option name can say
 * so without the others inheriting it.
 */
export function optionLabel(moduleId: string, opt: { name: string; label: string }): string {
  return scoped(`option.${moduleId}.${opt.name}`, `option.${opt.name}`, opt.label);
}

/**
 * One value a setting can take — "Easy", "Off", "Late surrender".
 *
 * Module-first for a reason found the hard way: Canasta's target score of 500
 * is labelled "500 (short)" while Gin Rummy's and Rummy Tiles' 500 is just
 * "500". A bare `choice.targetScore.500` gave all three whichever wording was
 * keyed last, so one of them was wrong whichever way it went.
 */
export function choiceLabel(
  moduleId: string,
  optionName: string,
  choice: { value: number | string; label: string },
): string {
  return scoped(
    `choice.${moduleId}.${optionName}.${choice.value}`,
    `choice.${optionName}.${choice.value}`,
    choice.label,
  );
}

/**
 * The specific key if it has words, else the shared one, else the server's own
 * label — so a module opts out of the shared wording only by having its own.
 *
 * The probe is `messageTemplate`, not a `t` call with a sentinel: under the
 * missing-key marker `t` answers `_TX_<key>_` for everything unworded, and a
 * sentinel comparison would then read every *deliberately* absent
 * module-scoped key as present — turning the diagnostic into a source of
 * leaks that are not real.
 */
function scoped(specific: string, shared: string, serverLabel: string): string {
  if (messageTemplate(specific) !== undefined) return t(specific);
  return t(shared, undefined, serverLabel);
}
