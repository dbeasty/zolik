/**
 * Node mode and a table this phone hosts run on one embedded server. Through
 * the real SessionProvider: leaving the table must not end node mode, and
 * signing out must not end the table.
 */
import React from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';

const mockStore: Record<string, string> = {};
jest.mock('expo-secure-store', () => ({
  getItemAsync: async (k: string) => mockStore[k] ?? null,
  setItemAsync: async (k: string, v: string) => {
    mockStore[k] = v;
  },
  deleteItemAsync: async (k: string) => {
    delete mockStore[k];
  },
}));
jest.mock('expo-web-browser', () => ({ openAuthSessionAsync: jest.fn() }));
jest.mock('expo-linking', () => ({ createURL: (p: string) => p }));

jest.mock('@/modules/zolik-nearby', () => ({
  nearbyAvailable: true,
  coreHoldsGuests: false,
  PROTOCOL_VERSION: 1,
  startHost: jest.fn(async () => ({ baseUrl: 'http://127.0.0.1:1', instanceId: 'inst' })),
  startNode: jest.fn(async () => ({})),
  stopHost: jest.fn(async () => undefined),
  nodeIdentity: jest.fn(() => ({ nodeId: 'n', publicKey: 'pub' })),
  closeRelay: jest.fn(async () => undefined),
  closeRoom: jest.fn(async () => undefined),
  bleHostStop: jest.fn(async () => undefined),
  setJoinedTexts: jest.fn(),
  hostResumed: jest.fn(),
}));
const mockNearby = jest.requireMock('@/modules/zolik-nearby') as Record<string, jest.Mock>;

const mockApi = {
  bindSession: jest.fn(),
  getAuthProviders: jest.fn(async () => []),
  getMe: jest.fn(async () => ({})),
  enrollNode: jest.fn(),
};
jest.mock('@/src/api/client', () => ({
  get apiClient() {
    return mockApi;
  },
  ZolikClient: class {
    baseUrl: string;
    constructor(baseUrl: string) {
      this.baseUrl = baseUrl;
    }
    async guestLogin() {
      return { accessToken: 'a', refreshToken: 'r', userId: 'g1', username: 'G', isGuest: true, guestId: 'g1' };
    }
    bindSession() {}
  },
}));
// Skips the signed-in seat-receipt step, a dynamic import jest cannot load.
jest.mock('@/src/net/servingTable', () => ({ servingTable: async () => ({ instanceId: 'served' }) }));
jest.mock('@/src/net/useReplicaSync', () => ({ useReplicaSync: () => undefined }));
jest.mock('@/src/notify/localNotify', () => ({ askToNotify: async () => undefined }));

import { SessionProvider, useSession } from '@/src/context/SessionContext';
import { credentialKey } from '@/src/net/nodeSession';

type Ctx = ReturnType<typeof useSession>;
const user = { accessToken: 'a', refreshToken: 'r', userId: 'u1', username: 'Ann', isGuest: false };

async function settle() {
  await act(async () => {
    await new Promise((res) => setTimeout(res, 0));
  });
}

async function mountSignedIn(): Promise<{ ctx: () => Ctx; r: ReactTestRenderer }> {
  mockStore.zolik_session = JSON.stringify(user);
  mockStore[credentialKey('u1')] = 'cred';
  let latest: Ctx | null = null;
  function Probe() {
    latest = useSession();
    return null;
  }
  let r!: ReactTestRenderer;
  await act(async () => {
    r = create(React.createElement(SessionProvider, null, React.createElement(Probe)));
  });
  await settle();
  return { ctx: () => latest!, r };
}

describe('one embedded server under the session', () => {
  beforeEach(() => {
    for (const k of Object.keys(mockStore)) delete mockStore[k];
    jest.clearAllMocks();
  });

  it('leaving a hosted table keeps node mode running', async () => {
    const { ctx, r } = await mountSignedIn();
    expect(mockNearby.startNode).toHaveBeenCalledWith('cred', 'u1', expect.any(String));

    await act(async () => ctx().playOffline('Ann'));
    expect(ctx().offline?.role).toBe('host');
    await act(async () => ctx().leaveOffline());

    expect(mockNearby.closeRoom).toHaveBeenCalled();
    expect(mockNearby.stopHost).not.toHaveBeenCalled();
    act(() => r.unmount());
  });

  it('signing out keeps a hosted table running until it ends', async () => {
    const { ctx, r } = await mountSignedIn();
    await act(async () => ctx().playOffline('Ann'));

    await act(async () => ctx().setSession(null));
    await settle();
    expect(mockNearby.stopHost).not.toHaveBeenCalled();
    expect(ctx().offline?.role).toBe('host');
    expect(mockStore[credentialKey('u1')]).toBeUndefined();

    await act(async () => ctx().leaveOffline());
    expect(mockNearby.stopHost).toHaveBeenCalledTimes(1);
    act(() => r.unmount());
  });
});
