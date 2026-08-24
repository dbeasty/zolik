import React from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';

import { useWaitingLobbyStatus } from '@/src/hooks/useWaitingLobbyStatus';
import type { WaitingPlayer } from '@/src/api/types';

const mockGetWaitingLobby = jest.fn<Promise<WaitingPlayer[]>, []>();
jest.mock('@/src/api/client', () => ({
  apiClient: { getWaitingLobby: (...args: unknown[]) => mockGetWaitingLobby(...(args as [])) },
}));

const alice: WaitingPlayer = {
  playerId: 'p1',
  username: 'Alice',
  isGuest: true,
  joinedAt: '2026-01-01T00:00:00Z',
};

type Probe = { players: WaitingPlayer[]; loaded: boolean };

// See useLobbySocket.test.ts for why every renderer created here is tracked
// and unmounted in afterEach: a live effect (this hook's polling interval)
// left running past the end of its own test can corrupt React's reconciler
// state for other test files sharing the same Jest worker.
const activeRenderers: ReactTestRenderer[] = [];

function renderProbe(enabled = true): { probe: Probe; setEnabled: (on: boolean) => void } {
  const probe: Probe = { players: [], loaded: false };
  function ProbeComponent({ on }: { on: boolean }) {
    const { players, loaded } = useWaitingLobbyStatus(on);
    probe.players = players;
    probe.loaded = loaded;
    return null;
  }
  let renderer!: ReactTestRenderer;
  act(() => {
    renderer = create(React.createElement(ProbeComponent, { on: enabled }));
  });
  activeRenderers.push(renderer);
  const setEnabled = (on: boolean) => {
    act(() => renderer.update(React.createElement(ProbeComponent, { on })));
  };
  return { probe, setEnabled };
}

describe('useWaitingLobbyStatus', () => {
  beforeEach(() => {
    mockGetWaitingLobby.mockReset();
    mockGetWaitingLobby.mockResolvedValue([]);
    jest.useFakeTimers();
  });

  afterEach(() => {
    for (const renderer of activeRenderers.splice(0)) {
      act(() => renderer.unmount());
    }
    jest.useRealTimers();
  });

  it('fetches immediately on mount rather than waiting for the first poll tick', async () => {
    mockGetWaitingLobby.mockResolvedValue([alice]);
    const { probe } = renderProbe();

    await act(async () => {
      await Promise.resolve();
    });

    expect(mockGetWaitingLobby).toHaveBeenCalledTimes(1);
    expect(probe.players).toEqual([alice]);
    expect(probe.loaded).toBe(true);
  });

  it('polls again every 5 seconds', async () => {
    renderProbe();
    await act(async () => {
      await Promise.resolve();
    });
    expect(mockGetWaitingLobby).toHaveBeenCalledTimes(1);

    await act(async () => {
      jest.advanceTimersByTime(5000);
      await Promise.resolve();
    });
    expect(mockGetWaitingLobby).toHaveBeenCalledTimes(2);
  });

  it('does not fetch at all while disabled', async () => {
    renderProbe(false);
    await act(async () => {
      jest.advanceTimersByTime(20000);
      await Promise.resolve();
    });
    expect(mockGetWaitingLobby).not.toHaveBeenCalled();
  });

  it('keeps the last-known count on a failed poll rather than clearing it', async () => {
    mockGetWaitingLobby.mockResolvedValueOnce([alice]);
    const { probe } = renderProbe();
    await act(async () => {
      await Promise.resolve();
    });
    expect(probe.players).toEqual([alice]);

    mockGetWaitingLobby.mockRejectedValueOnce(new Error('offline'));
    await act(async () => {
      jest.advanceTimersByTime(5000);
      await Promise.resolve();
    });

    // A transient failure must not flash "0 players waiting" over a real,
    // still-probably-true count from moments ago.
    expect(probe.players).toEqual([alice]);
  });

  it('stops polling once disabled and clears the shown count', async () => {
    mockGetWaitingLobby.mockResolvedValue([alice]);
    const { probe, setEnabled } = renderProbe();
    await act(async () => {
      await Promise.resolve();
    });
    expect(probe.players).toEqual([alice]);

    setEnabled(false);
    expect(probe.players).toEqual([]);
    expect(probe.loaded).toBe(false);

    mockGetWaitingLobby.mockClear();
    await act(async () => {
      jest.advanceTimersByTime(20000);
      await Promise.resolve();
    });
    expect(mockGetWaitingLobby).not.toHaveBeenCalled();
  });
});
