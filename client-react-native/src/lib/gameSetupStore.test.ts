const mockBacking = new Map<string, string>();

jest.mock('@/src/context/SessionContext', () => ({
  storage: {
    async getItem(key: string) {
      return mockBacking.has(key) ? mockBacking.get(key)! : null;
    },
    async setItem(key: string, value: string) {
      mockBacking.set(key, value);
    },
  },
}));

import { loadGameSetup, saveGameSetup } from '@/src/lib/gameSetupStore';

beforeEach(() => mockBacking.clear());

describe('gameSetupStore', () => {
  it('reports nothing saved for a module never set up', async () => {
    expect(await loadGameSetup('gin-rummy')).toBeUndefined();
  });

  it('round-trips what was saved', async () => {
    await saveGameSetup('gin-rummy', {
      variation: 'standard',
      options: { initialMeldMinimum: 50, discardDrawMinRound: 2 },
      bots: 3,
    });
    expect(await loadGameSetup('gin-rummy')).toEqual({
      variation: 'standard',
      options: { initialMeldMinimum: 50, discardDrawMinRound: 2 },
      bots: 3,
    });
  });

  it('keeps modules separate', async () => {
    await saveGameSetup('gin-rummy', { bots: 2 });
    await saveGameSetup('rummy-tiles', { bots: 4 });
    expect(await loadGameSetup('gin-rummy')).toEqual({ bots: 2 });
    expect(await loadGameSetup('rummy-tiles')).toEqual({ bots: 4 });
  });

  it('overwrites a module previous setup on re-save', async () => {
    await saveGameSetup('gin-rummy', { bots: 2 });
    await saveGameSetup('gin-rummy', { bots: 5 });
    expect(await loadGameSetup('gin-rummy')).toEqual({ bots: 5 });
  });

  it('survives unreadable storage rather than throwing', async () => {
    mockBacking.set('zolik_game_setup', '{not json');
    expect(await loadGameSetup('gin-rummy')).toBeUndefined();
  });
});
