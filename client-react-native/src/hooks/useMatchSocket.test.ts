/**
 * The match socket, and the one thing it must never do: leave a reconnect
 * chain running behind the live one.
 *
 * This is written from a real table that jammed. The url changed once — a
 * token refresh is enough, it is part of the query string — and from then on
 * two chains reconnected forever, roughly one socket per second for a quarter
 * of an hour. The server closes whichever connection a new one displaces, so
 * each chain kept killing the other's socket, and each of those deaths paused
 * the table. What the player saw was none of that: the board stayed on screen
 * and the discard button did nothing at all, because `send` fired into a
 * socket that had just been closed and returned without a word.
 *
 * So the tests below are about the invariant rather than the symptom: after
 * the effect re-runs, exactly one socket may ever be opened per close, and a
 * superseded chain must be finished for good.
 */
import React from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';

import { useMatchSocket, type MatchSocketState } from '@/src/hooks/useMatchSocket';

const mockGetCapacity = jest.fn(async () => ({
  accepting: true,
  waitingRoomOpen: true,
  startingMatches: true,
  live: 0,
}));

jest.mock('@/src/api/client', () => ({
  apiClient: {
    get getCapacity() {
      return mockGetCapacity;
    },
  },
}));

async function flushClose() {
  await act(async () => {
    await Promise.resolve();
  });
}

type Handler = ((ev?: unknown) => void) | null;

class FakeWebSocket {
  static OPEN = 1;
  static instances: FakeWebSocket[] = [];

  readyState = 0;
  onopen: Handler = null;
  onclose: Handler = null;
  onerror: Handler = null;
  onmessage: ((ev: { data: string }) => void) | null = null;
  sent: string[] = [];

  constructor(public url: string) {
    FakeWebSocket.instances.push(this);
  }

  close() {
    // A browser does not deliver `onclose` synchronously from `close()`, and
    // the bug this file exists for lives in exactly that gap: the next run of
    // the effect gets to run first. Tests fire it explicitly.
    this.readyState = 2;
  }

  send(data: string) {
    this.sent.push(data);
  }

  fireOpen() {
    this.readyState = FakeWebSocket.OPEN;
    this.onopen?.();
  }

  fireClose() {
    this.readyState = 3;
    this.onclose?.();
  }
}

type Probe = { hook: MatchSocketState | null };

// One component reference, reused across updates, so changing `url` is a prop
// change rather than an unmount and a remount of a fresh component — a
// remount gets fresh refs and would hide the very leak under test.
function ProbeComponent({ url, probe }: { url: string | null; probe: Probe }) {
  probe.hook = useMatchSocket(url);
  return null;
}

const activeRenderers: ReactTestRenderer[] = [];

function renderProbe(url: string | null) {
  const probe: Probe = { hook: null };
  let renderer!: ReactTestRenderer;
  act(() => {
    renderer = create(React.createElement(ProbeComponent, { url, probe }));
  });
  activeRenderers.push(renderer);
  const setUrl = (next: string | null) => {
    act(() => renderer.update(React.createElement(ProbeComponent, { url: next, probe })));
  };
  return { probe, setUrl };
}

describe('useMatchSocket', () => {
  const realWebSocket = (global as { WebSocket?: unknown }).WebSocket;
  let randomSpy: jest.SpyInstance;

  beforeEach(() => {
    FakeWebSocket.instances = [];
    (global as { WebSocket?: unknown }).WebSocket = FakeWebSocket;
    jest.useFakeTimers();
    randomSpy = jest.spyOn(Math, 'random').mockReturnValue(0);
    mockGetCapacity.mockClear();
    mockGetCapacity.mockResolvedValue({
      accepting: true,
      waitingRoomOpen: true,
      startingMatches: true,
      live: 0,
    });
  });

  afterEach(() => {
    for (const renderer of activeRenderers.splice(0)) {
      act(() => renderer.unmount());
    }
  });

  afterEach(() => {
    randomSpy.mockRestore();
    jest.useRealTimers();
    (global as { WebSocket?: unknown }).WebSocket = realWebSocket;
  });

  it('opens one socket and reconnects after it drops', async () => {
    renderProbe('ws://table/1?token=a');
    expect(FakeWebSocket.instances).toHaveLength(1);

    act(() => FakeWebSocket.instances[0].fireOpen());
    act(() => FakeWebSocket.instances[0].fireClose());
    await flushClose();
    act(() => {
      jest.advanceTimersByTime(1000);
    });

    expect(FakeWebSocket.instances).toHaveLength(2);
    expect(FakeWebSocket.instances[1].url).toBe('ws://table/1?token=a');
  });

  it('does not leave the old chain reconnecting after the url changes', () => {
    const { setUrl } = renderProbe('ws://table/1?token=a');
    const first = FakeWebSocket.instances[0];
    act(() => first.fireOpen());

    // The token is refreshed: same table, new url. Cleanup closes the first
    // socket and the new run opens the second.
    setUrl('ws://table/1?token=b');
    expect(FakeWebSocket.instances).toHaveLength(2);
    expect(FakeWebSocket.instances[1].url).toBe('ws://table/1?token=b');

    // And only *now* does the browser deliver the first socket's close, after
    // the new run has already started. This is the moment the old code
    // rearmed a chain nothing could stop.
    act(() => first.fireClose());
    act(() => {
      jest.advanceTimersByTime(60_000);
    });

    expect(FakeWebSocket.instances).toHaveLength(2);
  });

  it('keeps sending on the live socket after the url changes', () => {
    const { probe, setUrl } = renderProbe('ws://table/1?token=a');
    const first = FakeWebSocket.instances[0];
    act(() => first.fireOpen());

    setUrl('ws://table/1?token=b');
    const second = FakeWebSocket.instances[1];
    act(() => second.fireOpen());
    // The displaced socket's close lands late, and must not clear the ref the
    // live socket was just put in.
    act(() => first.fireClose());

    act(() => probe.hook!.send({ verb: 'discard', cards: ['8H'] }));

    expect(second.sent).toHaveLength(1);
    expect(JSON.parse(second.sent[0])).toEqual({ verb: 'discard', cards: ['8H'] });
    expect(probe.hook!.error).toBeNull();
  });

  it('reports an action it could not send instead of dropping it silently', async () => {
    const { probe } = renderProbe('ws://table/1?token=a');
    const first = FakeWebSocket.instances[0];
    act(() => first.fireOpen());
    act(() => first.fireClose());
    await flushClose();

    act(() => probe.hook!.send({ verb: 'discard', cards: ['8H'] }));

    expect(first.sent).toHaveLength(0);
    expect(probe.hook!.error).toEqual({ code: 'NOT_CONNECTED' });
  });

  it('surfaces SERVER_BUSY and backs off long when capacity says the server is full', async () => {
    mockGetCapacity.mockResolvedValue({
      accepting: false,
      waitingRoomOpen: false,
      startingMatches: false,
      live: 100,
    });
    const { probe } = renderProbe('ws://table/1?token=a');
    act(() => FakeWebSocket.instances[0].fireOpen());
    act(() => FakeWebSocket.instances[0].fireClose());
    await flushClose();

    expect(probe.hook!.error).toEqual({ code: 'SERVER_BUSY' });

    act(() => jest.advanceTimersByTime(10_000));
    expect(FakeWebSocket.instances).toHaveLength(1);

    act(() => jest.advanceTimersByTime(25_000));
    expect(FakeWebSocket.instances.length).toBeGreaterThan(1);
  });
});
