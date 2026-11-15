/**
 * The lobby's join and table screens stay mounted under Rules or Settings.
 * Their polls went on there, and when the host started the match each one
 * called router.replace — replacing the screen the player was actually on.
 * Rendered for real: the join screen, its poll, and its navigation.
 */
import React from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';

let mockFocused = true;
const mockReplace = jest.fn();
jest.mock('expo-router', () => ({
  useIsFocused: () => mockFocused,
  router: { replace: (...a: unknown[]) => mockReplace(...a) },
  useLocalSearchParams: () => ({ matchId: 'm1' }),
}));

const mockGetMatch = jest.fn();
jest.mock('@/src/context/SessionContext', () => ({
  useSession: () => ({ client: { getMatch: mockGetMatch }, session: { userId: 'u1' } }),
}));
jest.mock('@/src/age/AgeGate', () => ({
  useAgeGate: () => ({ blocks: () => false, refusesJoin: async () => false }),
  AdultsOnly: () => null,
}));
jest.mock('@/src/notify/TableEvents', () => ({ useAnnounceArrivals: () => undefined }));
jest.mock('@/src/components/InvitePanel', () => ({ InvitePanel: () => null }));
jest.mock('@/src/components/Screen', () => ({
  Screen: ({ children }: { children: React.ReactNode }) => children,
}));

import JoinMatchScreen from '@/app/lobby/join';

const lobby = { matchId: 'm1', moduleId: 'zoliky', status: 'lobby', players: [], joinCode: 'ABC' };

async function flush() {
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
  });
}

describe('lobby polling under another screen', () => {
  let r: ReactTestRenderer | null = null;

  beforeEach(() => {
    jest.useFakeTimers();
    mockFocused = true;
    mockReplace.mockClear();
    mockGetMatch.mockReset();
  });
  afterEach(() => {
    if (r) act(() => r!.unmount());
    r = null;
    jest.useRealTimers();
  });

  const render = async () => {
    await act(async () => {
      r = create(React.createElement(JoinMatchScreen));
    });
    await flush();
  };
  const rerender = async () => {
    await act(async () => r!.update(React.createElement(JoinMatchScreen)));
    await flush();
  };
  const tick = async (ms = 2000) => {
    await act(async () => {
      jest.advanceTimersByTime(ms);
    });
    await flush();
  };

  it('stops polling, and never navigates, while another screen is on top', async () => {
    mockGetMatch.mockResolvedValue(lobby);
    await render();
    const before = mockGetMatch.mock.calls.length;
    expect(before).toBeGreaterThan(0);

    mockFocused = false; // Rules pushed over the waiting room
    await rerender();
    mockGetMatch.mockResolvedValue({ ...lobby, status: 'active' }); // the host deals
    await tick();
    await tick();
    expect(mockGetMatch.mock.calls.length).toBe(before);
    expect(mockReplace).not.toHaveBeenCalled();

    mockFocused = true; // back to the waiting room
    await rerender();
    expect(mockReplace).toHaveBeenCalledWith('/match/m1');
  });

  it('does not navigate when an answer already in flight lands after the blur', async () => {
    let answer!: (m: unknown) => void;
    mockGetMatch.mockResolvedValueOnce(lobby);
    await render();
    mockGetMatch.mockImplementationOnce(() => new Promise((res) => (answer = res)));
    await tick(); // a poll goes out while focused
    mockFocused = false;
    await rerender();
    await act(async () => answer({ ...lobby, status: 'active' }));
    await flush();
    expect(mockReplace).not.toHaveBeenCalled();
  });

  it('clears a failed poll’s error once a poll succeeds', async () => {
    mockGetMatch.mockRejectedValueOnce(new Error('network down'));
    await render();
    const errorShown = () => r!.root.findAll((n) => n.props.testID === 'lobby-error').length > 0;
    expect(errorShown()).toBe(true);

    mockGetMatch.mockResolvedValue(lobby);
    await tick();
    expect(errorShown()).toBe(false);
  });
});
