import * as nearby from '../../modules/zolik-nearby';
import {
  credentialKey,
  hostingTable,
  hostTable,
  releaseTable,
  runningNode,
  startNodeFor,
  stopNodeFor,
} from './nodeSession';

jest.mock('../../modules/zolik-nearby', () => ({
  nearbyAvailable: true,
  startHost: jest.fn(),
  startNode: jest.fn(),
  stopHost: jest.fn(),
  nodeIdentity: jest.fn(),
}));

const startHost = nearby.startHost as jest.Mock;
const startNode = nearby.startNode as jest.Mock;
const stopHost = nearby.stopHost as jest.Mock;
const nodeIdentity = nearby.nodeIdentity as jest.Mock;

function memoryStorage(initial: Record<string, string> = {}) {
  const items = { ...initial };
  return {
    items,
    getItem: async (k: string) => items[k] ?? null,
    setItem: async (k: string, v: string) => {
      items[k] = v;
    },
    deleteItem: async (k: string) => {
      delete items[k];
    },
  };
}

const user = '65f0c0ffee';

describe('making a device hold an account’s data', () => {
  beforeEach(() => jest.clearAllMocks());

  it('enrols once and then starts in node mode', async () => {
    // The key only exists after the host has run once, so the first time is
    // start, enrol, start again - and every later time is just start.
    nodeIdentity.mockReturnValue({ nodeId: 'n1', publicKey: 'pub' });
    const storage = memoryStorage();
    const enrol = jest.fn().mockResolvedValue({ nodeId: 'n1', credential: 'cred' });

    expect(await startNodeFor(user, storage, enrol, 'http://127.0.0.1:8099')).toBe(true);

    expect(startHost).toHaveBeenCalledTimes(1);
    expect(enrol).toHaveBeenCalledWith('pub', 'phone');
    expect(startNode).toHaveBeenCalledWith('cred', user, 'http://127.0.0.1:8099');
    expect(storage.items[credentialKey(user)]).toBe('cred');

    jest.clearAllMocks();
    expect(await startNodeFor(user, storage, enrol, 'http://127.0.0.1:8099')).toBe(true);
    expect(enrol).not.toHaveBeenCalled();
    expect(startHost).not.toHaveBeenCalled();
    expect(startNode).toHaveBeenCalledWith('cred', user, 'http://127.0.0.1:8099');
  });

  it('does nothing where there is no embedded server', async () => {
    // The web build. Not a failure: the app reads everything from the cloud
    // there, as it always has.
    jest.replaceProperty(nearby as { nearbyAvailable: boolean }, 'nearbyAvailable', false);
    const enrol = jest.fn();
    expect(await startNodeFor(user, memoryStorage(), enrol, 'http://127.0.0.1:8099')).toBe(false);
    expect(enrol).not.toHaveBeenCalled();
    jest.replaceProperty(nearby as { nearbyAvailable: boolean }, 'nearbyAvailable', true);
  });

  it('forgets the credential when somebody signs out', async () => {
    // It names this device as that person's, and a shared phone should not
    // stay enrolled as theirs after they leave.
    const storage = memoryStorage({ [credentialKey(user)]: 'cred' });
    await stopNodeFor(user, storage);
    expect(stopHost).toHaveBeenCalled();
    expect(storage.items[credentialKey(user)]).toBeUndefined();
  });
});

describe('one embedded server, a node and a hosted table', () => {
  // Both use the phone's single server. Neither may stop it while the other
  // still needs it.
  const cloud = 'http://127.0.0.1:8099';
  const enrolled = () => memoryStorage({ [credentialKey(user)]: 'cred' });

  beforeEach(async () => {
    jest.clearAllMocks();
    startHost.mockResolvedValue({ baseUrl: 'http://127.0.0.1:1', instanceId: 'i1' });
    // Leave no state behind from the test before.
    await releaseTable();
    await stopNodeFor('', memoryStorage());
    jest.clearAllMocks();
  });

  it('leaving a hosted table keeps node mode running', async () => {
    await startNodeFor(user, enrolled(), jest.fn(), cloud);
    await hostTable();
    await releaseTable();
    expect(stopHost).not.toHaveBeenCalled();
    expect(runningNode()).toBe(true);
    expect(hostingTable()).toBe(false);
  });

  it('signing out keeps a hosted table running, and the table end stops the server', async () => {
    const storage = enrolled();
    await startNodeFor(user, storage, jest.fn(), cloud);
    await hostTable();
    await stopNodeFor(user, storage);
    expect(stopHost).not.toHaveBeenCalled();
    // The credential still goes at once: the device is no longer theirs.
    expect(storage.items[credentialKey(user)]).toBeUndefined();

    await releaseTable();
    expect(stopHost).toHaveBeenCalledTimes(1);
  });

  it('a sign-in while hosting waits for the table, rather than restarting the server under it', async () => {
    await hostTable();
    const enrol = jest.fn();
    expect(await startNodeFor(user, enrolled(), enrol, cloud)).toBe(false);
    expect(startNode).not.toHaveBeenCalled();
    expect(stopHost).not.toHaveBeenCalled();

    await releaseTable();
    expect(await startNodeFor(user, enrolled(), enrol, cloud)).toBe(true);
    expect(startNode).toHaveBeenCalledWith('cred', user, cloud);
  });

  it('leaving a host seat this window did not open still stops the server', async () => {
    // A Mac game window sits at the app's own table (sitFromApp) without
    // having called hostTable; leaving it stopped the server before.
    await releaseTable();
    expect(stopHost).toHaveBeenCalledTimes(1);
  });

  it('a hosted table with no node stops the server when it ends', async () => {
    await hostTable();
    await releaseTable();
    expect(stopHost).toHaveBeenCalledTimes(1);
  });
});
