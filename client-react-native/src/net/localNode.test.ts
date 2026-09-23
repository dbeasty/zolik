import * as nearby from '../../modules/zolik-nearby';
import * as localNode from './localNode';

jest.mock('../../modules/zolik-nearby', () => ({
  hostStatus: jest.fn(),
  replicaReady: jest.fn(),
  syncNow: jest.fn(),
  followMatch: jest.fn(),
}));

const hostStatus = nearby.hostStatus as jest.Mock;
const replicaReady = nearby.replicaReady as jest.Mock;
const syncNow = nearby.syncNow as jest.Mock;

const running = { port: 1234, baseUrl: 'http://127.0.0.1:1234', instanceId: 'i', lanPort: 0 };

function answer(status: number, body?: unknown) {
  return Promise.resolve({
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  } as Response);
}

describe('the copy of a person’s data on their own device', () => {
  beforeEach(() => {
    jest.resetAllMocks();
    global.fetch = jest.fn() as unknown as typeof fetch;
  });

  it('reads the history from the device rather than the network', async () => {
    hostStatus.mockReturnValue(running);
    (global.fetch as jest.Mock).mockReturnValue(
      answer(200, [{ matchId: 'm1', moduleId: 'prsi', finished: true }]),
    );

    const rows = await localNode.matches();

    expect(rows).toHaveLength(1);
    expect(rows[0].moduleId).toBe('prsi');
    expect(global.fetch).toHaveBeenCalledWith('http://127.0.0.1:1234/me/matches');
  });

  it('treats “nothing synced yet” as an answer rather than a failure', async () => {
    // The difference matters on a screen: "we have not synced" and "you have
    // never played" look the same if this throws or returns an empty list
    // without saying which it is.
    hostStatus.mockReturnValue(running);
    (global.fetch as jest.Mock).mockReturnValue(answer(503));

    await expect(localNode.matches()).resolves.toEqual([]);
    await expect(localNode.prefs()).resolves.toBeNull();
    expect(await localNode.status()).toEqual({ ready: false });
  });

  it('is unavailable with no host, and says so instead of guessing', async () => {
    hostStatus.mockReturnValue(null);
    expect(localNode.localNodeAvailable()).toBe(false);
    await expect(localNode.matches()).resolves.toEqual([]);
    await expect(localNode.putPrefs({ language: 'cs' })).rejects.toThrow(/no host/);
  });

  it('is only available once the account’s data has actually arrived', () => {
    hostStatus.mockReturnValue(running);
    replicaReady.mockReturnValue(false);
    expect(localNode.localNodeAvailable()).toBe(false);
    replicaReady.mockReturnValue(true);
    expect(localNode.localNodeAvailable()).toBe(true);
  });

  it('writes a person’s own settings to the device', async () => {
    hostStatus.mockReturnValue(running);
    (global.fetch as jest.Mock).mockReturnValue(answer(204));

    await localNode.putPrefs({ language: 'en' });

    expect(global.fetch).toHaveBeenCalledWith(
      'http://127.0.0.1:1234/me/prefs',
      expect.objectContaining({ method: 'PUT', body: JSON.stringify({ language: 'en' }) }),
    );
  });

  it('never fails a screen because a sync could not happen', async () => {
    // A sync that did not work is something to try again at the next
    // foreground, not an error to put in front of somebody.
    syncNow.mockRejectedValue(new Error('offline'));
    await expect(localNode.sync()).resolves.toBeUndefined();
  });
});
