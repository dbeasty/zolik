/**
 * The device's guest identity, through the real SessionProvider: what a bad
 * guest link and a web browser sign-in leave stored on this device.
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
const mockReplace = jest.fn();
let mockParams: { code?: string; error?: string } = { code: 'code-1' };
jest.mock('expo-router', () => ({
  router: { replace: (...a: unknown[]) => mockReplace(...a) },
  useGlobalSearchParams: () => mockParams,
}));
jest.mock('@/src/components/Screen', () => ({
  Screen: ({ children }: { children: React.ReactNode }) => children,
}));
jest.mock('expo-linking', () => ({ createURL: (p: string) => p }));

const mockApi = {
  bindSession: jest.fn(),
  guestLogin: jest.fn(),
  exchangeOAuthCode: jest.fn(),
  getAuthProviders: jest.fn(async () => []),
  getMe: jest.fn(async () => ({})),
  claimOfflineSeats: jest.fn(async () => 0),
};
jest.mock('@/src/api/client', () => ({
  // A getter: the factory runs before this file's consts are initialised.
  get apiClient() {
    return mockApi;
  },
  ZolikClient: class {},
}));
// A signed-in session makes the provider reach for seat receipts through a
// dynamic import, which jest cannot load; a page "served by a table" skips
// that step and changes nothing else this file looks at.
let mockServedBy: { instanceId: string } | null = null;
jest.mock('@/src/net/servingTable', () => ({ servingTable: async () => mockServedBy }));
jest.mock('@/src/net/useReplicaSync', () => ({ useReplicaSync: () => undefined }));
jest.mock('@/src/notify/localNotify', () => ({ askToNotify: async () => undefined }));

import AuthCallbackScreen from '@/app/auth/callback';
import { SessionProvider, useSession } from '@/src/context/SessionContext';
import { setLocale } from '@/src/lib/i18n';

const GUEST = 'zolik_guest_identity';
type Ctx = ReturnType<typeof useSession>;

function guest(id: string, key: string) {
  return {
    accessToken: `a-${id}`,
    refreshToken: `r-${id}`,
    userId: id,
    username: id,
    isGuest: true,
    guestId: id,
    guestKey: key,
    claimableMatches: 0,
  };
}

async function mount(...extra: React.ReactElement[]): Promise<{ ctx: () => Ctx; r: ReactTestRenderer }> {
  let latest: Ctx | null = null;
  function Probe() {
    latest = useSession();
    return null;
  }
  let r!: ReactTestRenderer;
  await act(async () => {
    r = create(React.createElement(SessionProvider, null, React.createElement(Probe), ...extra));
  });
  return { ctx: () => latest!, r };
}

describe('the device guest identity', () => {
  beforeEach(() => {
    for (const k of Object.keys(mockStore)) delete mockStore[k];
    jest.clearAllMocks();
    mockServedBy = null;
    mockParams = { code: 'code-1' };
    setLocale('en');
  });

  it('a bad guest link leaves this device’s own guest key in place', async () => {
    mockStore[GUEST] = JSON.stringify({ guestId: 'g-own', guestKey: 'g-own.sig' });
    // The server does not recognise the link's key and mints a stranger;
    // asked with the device's own key, it answers with the device's guest.
    mockApi.guestLogin.mockImplementation(async (_n: string, proof: { guestKey?: string }) =>
      proof.guestKey === 'g-own.sig' ? guest('g-own', 'g-own.sig') : guest('g-fresh', 'g-fresh.sig'),
    );
    const { ctx, r } = await mount();

    let ok = true;
    await act(async () => {
      ok = await ctx().resumeGuestFromLink('g-typo.bad');
    });

    expect(ok).toBe(false);
    expect(JSON.parse(mockStore[GUEST]).guestKey).toBe('g-own.sig');
    expect(ctx().guestKey).toBe('g-own.sig');
    expect(ctx().session?.userId).toBe('g-own');
    act(() => r.unmount());
  });

  it('a good guest link becomes this device’s guest', async () => {
    mockStore[GUEST] = JSON.stringify({ guestId: 'g-own', guestKey: 'g-own.sig' });
    mockApi.guestLogin.mockResolvedValue(guest('g-link', 'g-link.sig'));
    const { ctx, r } = await mount();

    let ok = false;
    await act(async () => {
      ok = await ctx().resumeGuestFromLink('g-link.sig');
    });
    expect(ok).toBe(true);
    expect(JSON.parse(mockStore[GUEST]).guestKey).toBe('g-link.sig');
    expect(ctx().session?.userId).toBe('g-link');
    act(() => r.unmount());
  });

  it('a bad link on a device with no guest keeps the fresh one, signed in once', async () => {
    mockApi.guestLogin.mockResolvedValue(guest('g-fresh', 'g-fresh.sig'));
    const { ctx, r } = await mount();
    await act(async () => {
      await ctx().resumeGuestFromLink('g-typo.bad');
    });
    expect(mockApi.guestLogin).toHaveBeenCalledTimes(1);
    expect(JSON.parse(mockStore[GUEST]).guestKey).toBe('g-fresh.sig');
    expect(ctx().session?.userId).toBe('g-fresh');
    act(() => r.unmount());
  });

  it('a web browser sign-in that claimed the guest’s games forgets the guest and the prompt', async () => {
    mockServedBy = { instanceId: 'i' };
    mockStore[GUEST] = JSON.stringify({ guestId: 'g-own', guestKey: 'g-own.sig' });
    mockApi.exchangeOAuthCode.mockResolvedValue({
      session: { accessToken: 'a', refreshToken: 'r', userId: 'u1', username: 'Ann', isGuest: false },
      claimedMatches: 3,
      created: false,
    });
    // The web build's landing page after the provider's redirect.
    const { ctx, r } = await mount(React.createElement(AuthCallbackScreen));
    await act(async () => {
      await new Promise((res) => setTimeout(res, 0));
    });
    expect(mockApi.exchangeOAuthCode).toHaveBeenCalledWith('code-1');
    expect(mockReplace).toHaveBeenCalledWith('/');
    expect(mockStore[GUEST]).toBeUndefined();
    expect(ctx().guestKey).toBeNull();
    expect(ctx().claimableMatches).toBe(0);
    expect(ctx().session?.userId).toBe('u1');
    act(() => r.unmount());
  });

  it('words a failed sign-in in the language that finishes loading after the screen mounts', async () => {
    jest.useFakeTimers();
    mockParams = { error: 'expired' };
    const { r } = await mount(React.createElement(AuthCallbackScreen));
    const shown = () =>
      r.root
        .findAll((n) => typeof n.props.children === 'string' && n.props['aria-live'] === 'polite')
        .map((n) => n.props.children as string);
    expect(shown()).toContain('That sign-in took too long. Please try again.');
    await act(async () => setLocale('cs'));
    expect(shown()).toContain('Přihlášení trvalo příliš dlouho. Zkuste to znovu.');
    act(() => r.unmount());
    jest.useRealTimers();
    setLocale('en');
  });
});
