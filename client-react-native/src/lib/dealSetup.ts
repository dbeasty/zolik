import type { ZolikClient } from '@/src/api/client';
import type { MatchModule } from '@/src/api/matchTypes';
import type { SavedGameSetup } from '@/src/lib/gameSetupStore';

/**
 * How a game is set up before it is dealt — shared by the setup screen, which
 * shows it, and a game page's Play now, which deals it without asking.
 */
export type DealSetup = {
  variation: string | undefined;
  options: Record<string, number>;
  bots: number | undefined;
};

/**
 * What a module is dealt with when the player says nothing: the setup they
 * saved last time, as far as it still fits the module.
 *
 * Only a saved pick that still matches the module's current descriptor is
 * trusted: a variation or option the server has since renamed or dropped
 * falls back to the module's own default rather than resurrecting a stale
 * choice.
 */
export function resolveSetup(mod: MatchModule, saved: SavedGameSetup | undefined): DealSetup {
  const savedVariation =
    saved?.variation && mod.variations?.some((x) => x.id === saved.variation) ? saved.variation : undefined;
  const variation = savedVariation ?? mod.variations?.[0]?.id;
  const spec = mod.variations?.find((x) => x.id === variation);
  const options: Record<string, number> = { ...(spec?.defaults ?? {}) };
  for (const opt of mod.options ?? []) {
    // A saved value the option no longer offers is as stale as a dropped
    // option: "AI" saved against a game that has since stopped offering it
    // would otherwise be sent back and refused.
    const v = saved?.options?.[opt.name];
    if (v !== undefined && (opt.choices ?? []).some((c) => c.value === v)) options[opt.name] = v;
  }
  return { variation, options, bots: saved?.bots };
}

/**
 * The seat range a table of this variation is dealt with: the variation's own
 * bounds where it names them, the module's otherwise — the client twin of the
 * server's `SeatRange`, which is what refuses a deal outside it.
 */
export function seatRange(mod: MatchModule, variation?: string): { min: number; max: number } {
  const spec = variation ? mod.variations?.find((v) => v.id === variation) : undefined;
  return {
    min: spec?.minPlayers || mod.minPlayers,
    max: spec?.maxPlayers || mod.maxPlayers,
  };
}

/**
 * The bot counts a table can be opened with: enough to reach its minimum
 * table at the low end, one short of its maximum at the high end — because
 * one of the seats is the host's. A one-seat game has exactly one answer:
 * none.
 *
 * Read from the variation, not the module: Partnership Pinochle is dealt to
 * four on a module that seats two to four, and Classic Canasta stops at four
 * on one that seats six. The module's range would offer counts the deal then
 * refuses.
 */
export function botChoices(mod: MatchModule, variation?: string): number[] {
  const { min, max } = seatRange(mod, variation);
  if (max <= 1) return [0];
  const out: number[] = [];
  for (let n = Math.max(1, min - 1); n <= Math.max(1, max - 1); n++) {
    out.push(n);
  }
  return out;
}

/**
 * How many bots to seat: what the host picked, clamped to the range above.
 * A clamp rather than a plain read, so a count left over from a game (or a
 * variation) whose range has since moved can never post a seat the server is
 * going to refuse.
 */
export function botCount(mod: MatchModule, picked?: number, variation?: string): number {
  const choices = botChoices(mod, variation);
  const lo = choices[0];
  const hi = choices[choices.length - 1];
  return Math.min(hi, Math.max(lo, picked ?? lo));
}

/** Opens a table, fills every other seat with bots, and deals. */
export async function dealWithBots(client: ZolikClient, mod: MatchModule, setup: DealSetup): Promise<string> {
  const { matchId } = await client.createMatch(mod.id, setup.variation, setup.options);
  // As many bots as asked for, clamped to the seats this module has. The
  // screen does not know what a legal table is for any game — the
  // descriptor does, and the clamp is the only thing reading it.
  const seats = botCount(mod, setup.bots, setup.variation);
  for (let i = 0; i < seats; i++) {
    await client.addBot(matchId);
  }
  await client.startMatch(matchId);
  return matchId;
}
