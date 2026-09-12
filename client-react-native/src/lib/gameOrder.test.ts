import type { MatchModule } from '@/src/api/matchTypes';
import { DEFAULT_POPULARITY_ORDER, orderModules } from '@/src/lib/gameOrder';

function mod(id: string): MatchModule {
  return { id, label: id, minPlayers: 2, maxPlayers: 4 };
}

describe('orderModules', () => {
  it('falls back to the popularity default with no play history', () => {
    const shuffled = ['prsi', 'holdem', 'canasta', 'ginrummy'].map(mod);
    const ordered = orderModules(shuffled).map((m) => m.id);
    expect(ordered).toEqual(['holdem', 'ginrummy', 'canasta', 'prsi']);
  });

  it('keeps the server order for modules the popularity list has never heard of', () => {
    const modules = [mod('zolik'), mod('newgame'), mod('canasta')];
    const ordered = orderModules(modules).map((m) => m.id);
    // zolik and canasta are both ranked (zolik ahead of canasta); newgame is
    // unranked and falls in behind every ranked module, in server order.
    expect(ordered).toEqual(['zolik', 'canasta', 'newgame']);
  });

  it('puts the most-played module first regardless of its popularity rank', () => {
    const modules = ['holdem', 'prsi', 'canasta'].map(mod);
    const ordered = orderModules(modules, { prsi: 40, holdem: 3 }).map((m) => m.id);
    expect(ordered).toEqual(['prsi', 'holdem', 'canasta']);
  });

  it('breaks a tie in play count with the popularity default', () => {
    const modules = ['prsi', 'holdem', 'blackjack'].map(mod);
    // holdem and blackjack are tied at 5 plays each; holdem outranks
    // blackjack in DEFAULT_POPULARITY_ORDER, so it still comes first.
    const ordered = orderModules(modules, { prsi: 1, holdem: 5, blackjack: 5 }).map((m) => m.id);
    expect(ordered).toEqual(['holdem', 'blackjack', 'prsi']);
  });

  it('treats a module missing from playCounts as zero plays, not as dropped', () => {
    const modules = ['zolik', 'rummytiles'].map(mod);
    const ordered = orderModules(modules, { zolik: 2 }).map((m) => m.id);
    expect(ordered).toEqual(['zolik', 'rummytiles']);
    expect(ordered).toHaveLength(2);
  });

  it('every module the server currently registers has a popularity rank', () => {
    // Not a hard requirement of orderModules — an unranked module still
    // renders, just at the back — but letting one go unranked by accident
    // would quietly demote it below every game a player has never touched.
    const registered = ['zolik', 'prsi', 'canasta', 'holdem', 'ginrummy', 'rummytiles', 'blackjack'];
    for (const id of registered) {
      expect(DEFAULT_POPULARITY_ORDER).toContain(id);
    }
  });
});
