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

import { loadMinimized, saveMinimized } from '@/src/lib/panelStore';

beforeEach(() => mockBacking.clear());

describe('panelStore', () => {
  it('reports nothing minimized for a match never saved', async () => {
    expect(await loadMinimized('match-1')).toEqual([]);
  });

  it('round-trips what was saved', async () => {
    await saveMinimized('match-1', ['zone:melds', 'controls']);
    expect(await loadMinimized('match-1')).toEqual(['zone:melds', 'controls']);
  });

  it('keeps matches separate', async () => {
    await saveMinimized('match-1', ['zone:melds']);
    await saveMinimized('match-2', ['controls']);
    expect(await loadMinimized('match-1')).toEqual(['zone:melds']);
    expect(await loadMinimized('match-2')).toEqual(['controls']);
  });

  it('evicts the least recently touched match past the keep limit', async () => {
    for (let i = 0; i < 7; i++) {
      await saveMinimized(`match-${i}`, [`panel-${i}`]);
    }
    expect(await loadMinimized('match-0')).toEqual([]);
    expect(await loadMinimized('match-6')).toEqual(['panel-6']);
  });

  it('survives unreadable storage rather than throwing', async () => {
    mockBacking.set('zolik_panels', '{not json');
    expect(await loadMinimized('match-1')).toEqual([]);
  });
});
