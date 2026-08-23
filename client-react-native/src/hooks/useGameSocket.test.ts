/**
 * A superseded socket must not speak for the live one.
 *
 * Any connect() that replaces an in-flight socket (a manual reconnect, or a
 * second game screen left mounted by a routing slip) leaves an older
 * WebSocket behind whose close event lands *after* the replacement has
 * already opened. Treating that as a disconnect pinned the game header on
 * "Your turn · draw · reconnecting…" over a perfectly healthy connection,
 * because nothing short of a fresh onopen ever clears it again.
 */
import React from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';

import { useGameSocket } from '@/src/hooks/useGameSocket';

type Handler = ((ev?: unknown) => void) | null;

class FakeWebSocket {
  static OPEN = 1;
  static instances: FakeWebSocket[] = [];

  readyState = 0;
  onopen: Handler = null;
  onclose: Handler = null;
  onerror: Handler = null;
  onmessage: Handler = null;
  closedByClient = false;

  constructor(public url: string) {
    FakeWebSocket.instances.push(this);
  }

  close() {
    this.closedByClient = true;
  }

  send() {}

  /** The server accepted this socket. */
  fireOpen() {
    this.readyState = FakeWebSocket.OPEN;
    this.onopen?.();
  }

  /** This socket's close event is delivered (always asynchronously in real life). */
  fireClose() {
    this.readyState = 3;
    this.onclose?.();
  }
}

type Probe = { connected: boolean; reconnect: () => void };

function renderProbe(gameId: string): { probe: Probe; renderer: ReactTestRenderer } {
  const probe = { connected: false, reconnect: () => {} } as Probe;
  function ProbeComponent() {
    const { connected, reconnect } = useGameSocket({ gameId });
    probe.connected = connected;
    probe.reconnect = reconnect;
    return null;
  }
  let renderer!: ReactTestRenderer;
  act(() => {
    renderer = create(React.createElement(ProbeComponent));
  });
  return { probe, renderer };
}

describe('useGameSocket connection state', () => {
  const realWebSocket = (global as { WebSocket?: unknown }).WebSocket;

  beforeEach(() => {
    FakeWebSocket.instances = [];
    (global as { WebSocket?: unknown }).WebSocket = FakeWebSocket;
    jest.useFakeTimers();
  });

  afterEach(() => {
    jest.useRealTimers();
    (global as { WebSocket?: unknown }).WebSocket = realWebSocket;
  });

  it('ignores a superseded socket closing after its replacement is open', () => {
    const { probe, renderer } = renderProbe('game-1');

    const first = FakeWebSocket.instances[0];
    act(() => first.fireOpen());
    expect(probe.connected).toBe(true);

    act(() => probe.reconnect());
    const second = FakeWebSocket.instances[1];
    expect(second).toBeDefined();
    expect(first.closedByClient).toBe(true);

    act(() => second.fireOpen());
    expect(probe.connected).toBe(true);

    // The superseded socket's close event finally arrives. It says nothing
    // about the connection the player actually has.
    act(() => first.fireClose());
    expect(probe.connected).toBe(true);

    act(() => renderer.unmount());
  });

  it('still reports a genuine disconnect of the live socket', () => {
    const { probe, renderer } = renderProbe('game-2');

    const first = FakeWebSocket.instances[0];
    act(() => first.fireOpen());
    expect(probe.connected).toBe(true);

    act(() => first.fireClose());
    expect(probe.connected).toBe(false);

    act(() => renderer.unmount());
  });

  it('ignores an error raised by a superseded socket', () => {
    const { probe, renderer } = renderProbe('game-3');

    const first = FakeWebSocket.instances[0];
    act(() => first.fireOpen());

    act(() => probe.reconnect());
    const second = FakeWebSocket.instances[1];
    act(() => second.fireOpen());
    expect(probe.connected).toBe(true);

    act(() => first.onerror?.());
    expect(probe.connected).toBe(true);

    act(() => renderer.unmount());
  });
});
