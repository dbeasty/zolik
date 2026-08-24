/**
 * The waiting-room connection has to be honest about *why* it isn't
 * showing a lobby: "first connect in flight" and "lost the connection and
 * retrying" are different situations, and collapsing both into one silent
 * spinner is exactly what made a misconfigured server address look
 * indistinguishable from a working-but-slow one during real testing.
 */
import React from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';

import { useLobbySocket, type LobbyConnectionStatus } from '@/src/hooks/useLobbySocket';
import type { WaitingPlayer } from '@/src/api/types';

type Handler = ((ev?: unknown) => void) | null;

class FakeWebSocket {
  static OPEN = 1;
  static instances: FakeWebSocket[] = [];

  readyState = 0;
  onopen: Handler = null;
  onclose: Handler = null;
  onerror: Handler = null;
  onmessage: ((ev: { data: string }) => void) | null = null;
  closedByClient = false;

  constructor(public url: string) {
    FakeWebSocket.instances.push(this);
  }

  close() {
    this.closedByClient = true;
  }

  send() {}

  fireOpen() {
    this.readyState = FakeWebSocket.OPEN;
    this.onopen?.();
  }

  fireMessage(data: unknown) {
    this.onmessage?.({ data: JSON.stringify(data) });
  }

  fireClose() {
    this.readyState = 3;
    this.onclose?.();
  }
}

type Probe = {
  players: WaitingPlayer[];
  status: LobbyConnectionStatus;
  attempts: number;
  retryNow: () => void;
  invited: { gameId: string; joinCode: string } | null;
};

// A module-level component reference, reused across updates, so toggling
// `enabled` via renderer.update triggers a normal prop change rather than an
// unmount-and-remount of a fresh anonymous component — the two look similar
// on the surface but exercise different code paths in the hook.
function ProbeComponent({ on, probe }: { on: boolean; probe: Probe }) {
  const { players, status, attempts, retryNow } = useLobbySocket(on, (gameId, joinCode) => {
    probe.invited = { gameId, joinCode };
  });
  probe.players = players;
  probe.status = status;
  probe.attempts = attempts;
  probe.retryNow = retryNow;
  return null;
}

// Every renderer created by renderProbe is tracked here and unmounted in
// afterEach (see the describe block below). Leaving a renderer's effects
// (and the WebSocket/timers they hold) live past the end of its own test
// is what caused the failure this comment is warning against: React's
// reconciler state is shared across the whole worker process, and an
// un-unmounted fiber tree from one test corrupted hook dispatch badly
// enough to break other test *files* sharing the same Jest worker — not
// just other tests in this file. useGameSocket.test.ts's own tests
// unmount explicitly for the same reason; this does it for every test
// automatically so a future test case can't reintroduce the same bug by
// forgetting to.
const activeRenderers: ReactTestRenderer[] = [];

function renderProbe(enabled = true): {
  probe: Probe;
  renderer: ReactTestRenderer;
  setEnabled: (on: boolean) => void;
} {
  const probe = {
    players: [],
    status: 'connecting',
    attempts: 0,
    retryNow: () => {},
    invited: null,
  } as Probe;
  let renderer!: ReactTestRenderer;
  act(() => {
    renderer = create(React.createElement(ProbeComponent, { on: enabled, probe }));
  });
  activeRenderers.push(renderer);
  const setEnabled = (on: boolean) => {
    act(() => renderer.update(React.createElement(ProbeComponent, { on, probe })));
  };
  return { probe, renderer, setEnabled };
}

describe('useLobbySocket', () => {
  const realWebSocket = (global as { WebSocket?: unknown }).WebSocket;

  beforeEach(() => {
    FakeWebSocket.instances = [];
    (global as { WebSocket?: unknown }).WebSocket = FakeWebSocket;
    jest.useFakeTimers();
  });

  // Registered before the timer-restoring afterEach below, so it runs first
  // (Jest runs same-scope afterEach hooks in registration order) — unmount
  // needs fake timers and the fake WebSocket still in place to run its
  // cleanup effects the same way the test itself ran.
  afterEach(() => {
    for (const renderer of activeRenderers.splice(0)) {
      act(() => renderer.unmount());
    }
  });

  afterEach(() => {
    jest.useRealTimers();
    (global as { WebSocket?: unknown }).WebSocket = realWebSocket;
  });

  it('starts in "connecting" and moves to "open" once the socket connects', () => {
    const { probe } = renderProbe();
    expect(probe.status).toBe('connecting');

    act(() => FakeWebSocket.instances[0].fireOpen());
    expect(probe.status).toBe('open');
    expect(probe.attempts).toBe(0);
  });

  it('applies an incoming lobby_waiting broadcast', () => {
    const { probe } = renderProbe();
    const ws = FakeWebSocket.instances[0];
    act(() => ws.fireOpen());

    act(() =>
      ws.fireMessage({
        type: 'lobby_waiting',
        players: [{ playerId: 'p1', username: 'Alice', isGuest: true, joinedAt: '2026-01-01T00:00:00Z' }],
      }),
    );
    expect(probe.players).toHaveLength(1);
    expect(probe.players[0].username).toBe('Alice');
  });

  it('calls onInvited for a personal lobby_invited push, without touching the player list', () => {
    const { probe } = renderProbe();
    const ws = FakeWebSocket.instances[0];
    act(() => ws.fireOpen());
    act(() =>
      ws.fireMessage({
        type: 'lobby_waiting',
        players: [{ playerId: 'p1', username: 'Alice', isGuest: false, joinedAt: '2026-01-01T00:00:00Z' }],
      }),
    );

    act(() => ws.fireMessage({ type: 'lobby_invited', gameId: 'game-9', joinCode: 'ABC123' }));

    expect(probe.invited).toEqual({ gameId: 'game-9', joinCode: 'ABC123' });
    expect(probe.players).toHaveLength(1); // unrelated state, must not be clobbered
  });

  it('a malformed message is ignored rather than crashing the connection', () => {
    const { probe } = renderProbe();
    const ws = FakeWebSocket.instances[0];
    act(() => ws.fireOpen());

    expect(() => act(() => ws.onmessage?.({ data: 'not json' }))).not.toThrow();
    expect(probe.status).toBe('open');
  });

  it('moves to "reconnecting" and counts attempts when the connection drops', () => {
    const { probe } = renderProbe();
    act(() => FakeWebSocket.instances[0].fireOpen());
    expect(probe.status).toBe('open');

    act(() => FakeWebSocket.instances[0].fireClose());
    expect(probe.status).toBe('reconnecting');
    expect(probe.attempts).toBe(1);
  });

  it('keeps showing the last-known player list while reconnecting rather than blanking it', () => {
    const { probe } = renderProbe();
    const ws = FakeWebSocket.instances[0];
    act(() => ws.fireOpen());
    act(() =>
      ws.fireMessage({
        type: 'lobby_waiting',
        players: [{ playerId: 'p1', username: 'Alice', isGuest: false, joinedAt: '2026-01-01T00:00:00Z' }],
      }),
    );

    act(() => ws.fireClose());
    expect(probe.status).toBe('reconnecting');
    expect(probe.players).toHaveLength(1); // still there — not cleared on disconnect
  });

  it('retries with a growing, capped backoff rather than hammering the server', () => {
    renderProbe();
    act(() => FakeWebSocket.instances[0].fireClose()); // never opened: first attempt fails

    // Nothing new yet — the first retry is scheduled, not immediate.
    expect(FakeWebSocket.instances).toHaveLength(1);

    act(() => jest.advanceTimersByTime(1500));
    expect(FakeWebSocket.instances).toHaveLength(2);

    act(() => FakeWebSocket.instances[1].fireClose());
    // Second attempt waits longer (exponential) rather than the same interval.
    act(() => jest.advanceTimersByTime(1500));
    expect(FakeWebSocket.instances).toHaveLength(2); // not yet — backoff grew past 1500ms
    act(() => jest.advanceTimersByTime(2000));
    expect(FakeWebSocket.instances).toHaveLength(3);
  });

  it('resets the attempt count back to zero once a retry succeeds', () => {
    const { probe } = renderProbe();
    act(() => FakeWebSocket.instances[0].fireClose());
    act(() => jest.advanceTimersByTime(1500));
    expect(probe.attempts).toBe(1);

    act(() => FakeWebSocket.instances[1].fireOpen());
    expect(probe.status).toBe('open');
    expect(probe.attempts).toBe(0);
  });

  it('retryNow connects immediately, skipping the scheduled backoff wait', () => {
    const { probe } = renderProbe();
    act(() => FakeWebSocket.instances[0].fireClose());
    expect(FakeWebSocket.instances).toHaveLength(1);

    act(() => probe.retryNow());
    expect(FakeWebSocket.instances).toHaveLength(2);
    expect(probe.status).toBe('connecting');
    expect(probe.attempts).toBe(0);
  });

  it('stops retrying and closes the socket once disabled', () => {
    const { setEnabled } = renderProbe();
    act(() => FakeWebSocket.instances[0].fireOpen());

    setEnabled(false);

    expect(FakeWebSocket.instances[0].closedByClient).toBe(true);

    // A close event arriving after teardown must not resurrect a reconnect
    // loop for a screen the person has already left.
    act(() => FakeWebSocket.instances[0].fireClose());
    act(() => jest.advanceTimersByTime(20000));
    expect(FakeWebSocket.instances).toHaveLength(1);
  });
});
