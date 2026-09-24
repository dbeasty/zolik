import * as nearby from '../../modules/zolik-nearby';
import { credentialKey, startNodeFor, stopNodeFor } from './nodeSession';

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
