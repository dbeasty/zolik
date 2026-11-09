import React from 'react';
import { act, create } from 'react-test-renderer';

import { useOrderedModules } from '@/src/components/GameButtons';

const mockModules = jest.fn();
let mockClient = { modules: mockModules, getStats: jest.fn() };

jest.mock('@/src/context/SessionContext', () => ({
  useSession: () => ({ client: mockClient, session: null }),
  storage: { getItem: jest.fn(), setItem: jest.fn() },
}));
jest.mock('@/src/age/AgeGate', () => ({ useAgeGate: () => ({ hidesFromLists: () => false }) }));

/**
 * A phone with no connection has to get the game list back without a restart,
 * and starting offline play swaps the client under the same screen: both left
 * the home screen on the old error, because nothing retried and a good answer
 * never cleared it.
 */
describe('useOrderedModules', () => {
  const game = { id: 'holdem', label: 'Poker', minPlayers: 2, maxPlayers: 9 };
  let seen: ReturnType<typeof useOrderedModules>;
  function Probe() {
    seen = useOrderedModules();
    return null;
  }
  async function settle(ms = 0) {
    await act(async () => {
      jest.advanceTimersByTime(ms);
      await Promise.resolve();
      await Promise.resolve();
      await Promise.resolve();
    });
  }

  beforeEach(() => {
    jest.useFakeTimers();
    mockModules.mockReset();
    mockClient = { modules: mockModules, getStats: jest.fn() };
  });
  afterEach(() => jest.useRealTimers());

  it('asks again while it cannot reach the server, and clears the error when it can', async () => {
    mockModules.mockRejectedValueOnce(new Error('fetch failed'));
    mockModules.mockResolvedValue([game]);
    act(() => {
      create(React.createElement(Probe));
    });
    await settle();
    expect(seen.error).toBeTruthy();
    expect(seen.modules).toBeNull();
    await settle(5000);
    await settle();
    expect(seen.error).toBe('');
    expect(seen.modules?.map((m) => m.id)).toEqual(['holdem']);
  });

  it('does not keep asking once it has the list', async () => {
    mockModules.mockResolvedValue([game]);
    act(() => {
      create(React.createElement(Probe));
    });
    await settle();
    await settle(60000);
    expect(mockModules).toHaveBeenCalledTimes(1);
  });
});
