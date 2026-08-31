import * as Linking from 'expo-linking';
import * as SecureStore from 'expo-secure-store';
import * as WebBrowser from 'expo-web-browser';
import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from 'react';
import { Platform } from 'react-native';

import { apiClient } from '@/src/api/client';
import { setLocale } from '@/src/lib/i18n';
import { authErrorMessage, parseAuthCallback } from '@/src/lib/auth';
import type {
  AccountProfile,
  AuthProvider,
  PlayerSession,
  SignInOutcome,
} from '@/src/api/types';

const SESSION_KEY = 'zolik_session';

/**
 * The device's guest identity, stored apart from the session and deliberately
 * kept across sign-out.
 *
 * It is not a login. It is the handle the server records a guest's matches
 * against, so keeping it is what lets somebody play for a week without an
 * account and still walk off with the record when they finally make one.
 * Clearing it on sign-out would silently orphan exactly the history this
 * feature exists to preserve.
 *
 * The refresh token is stored beside it because claiming that history later
 * requires proving possession of the guest *session*, not merely knowing the
 * id — the id travels in match records, so knowing it proves nothing.
 */
const GUEST_KEY = 'zolik_guest_identity';

type GuestIdentity = { guestId: string; refreshToken: string };

type SessionContextValue = {
  session: PlayerSession | null;
  loading: boolean;
  client: typeof apiClient;
  /** Sign-in methods this deployment offers; empty until fetched. */
  providers: AuthProvider[];
  /** The signed-in account, or null for guests and signed-out visitors. */
  account: AccountProfile | null;
  /** Matches on this device that an account could still absorb. */
  claimableMatches: number;
  setSession: (s: PlayerSession | null) => Promise<void>;
  guestLogin: (name: string) => Promise<void>;
  /** Mails a one-time sign-in code. */
  startEmailSignIn: (email: string) => Promise<void>;
  /** Redeems the code and signs in. */
  verifyEmailCode: (email: string, code: string) => Promise<SignInOutcome>;
  /** Runs a provider's browser sign-in end to end. */
  signInWithProvider: (providerId: string) => Promise<SignInOutcome | null>;
  /** Attaches another provider to the signed-in account. */
  linkProvider: (providerId: string) => Promise<void>;
  unlinkProvider: (providerId: string) => Promise<void>;
  /** Absorbs this device's guest history into the signed-in account. */
  claimGuestHistory: () => Promise<number>;
  refreshAccount: () => Promise<void>;
  login: (username: string, password: string) => Promise<void>;
  register: (username: string, password: string, email?: string) => Promise<void>;
  logout: () => Promise<void>;
};

const SessionContext = createContext<SessionContextValue | null>(null);

// expo-secure-store has no web implementation; fall back to localStorage
// there (it's not encrypted, but this only ever holds JWTs, same as any
// other browser-based session token).
export const storage = {
  async getItem(key: string): Promise<string | null> {
    if (Platform.OS === 'web') {
      return typeof localStorage !== 'undefined' ? localStorage.getItem(key) : null;
    }
    return SecureStore.getItemAsync(key);
  },
  async setItem(key: string, value: string): Promise<void> {
    if (Platform.OS === 'web') {
      if (typeof localStorage !== 'undefined') localStorage.setItem(key, value);
      return;
    }
    await SecureStore.setItemAsync(key, value);
  },
  async deleteItem(key: string): Promise<void> {
    if (Platform.OS === 'web') {
      if (typeof localStorage !== 'undefined') localStorage.removeItem(key);
      return;
    }
    await SecureStore.deleteItemAsync(key);
  },
};

async function persistSession(session: PlayerSession | null) {
  if (session) {
    await storage.setItem(SESSION_KEY, JSON.stringify(session));
  } else {
    await storage.deleteItem(SESSION_KEY);
  }
}

async function loadSession(): Promise<PlayerSession | null> {
  const raw = await storage.getItem(SESSION_KEY);
  if (!raw) return null;
  try {
    return JSON.parse(raw) as PlayerSession;
  } catch {
    return null;
  }
}

async function loadGuestIdentity(): Promise<GuestIdentity | null> {
  const raw = await storage.getItem(GUEST_KEY);
  if (!raw) return null;
  try {
    const parsed = JSON.parse(raw) as GuestIdentity;
    return parsed?.guestId ? parsed : null;
  } catch {
    return null;
  }
}

/**
 * This device's guest id, for anything that needs a stable identity before
 * there is a session to read one off.
 *
 * The face a first-time guest is shown is derived from it, which is what makes
 * somebody who has played before and come back find the same face waiting
 * rather than a fresh one each visit.
 */
export async function loadGuestId(): Promise<string | null> {
  return (await loadGuestIdentity())?.guestId ?? null;
}

export function SessionProvider({ children }: { children: React.ReactNode }) {
  const [session, setSessionState] = useState<PlayerSession | null>(null);
  const [loading, setLoading] = useState(true);
  const [providers, setProviders] = useState<AuthProvider[]>([]);
  const [account, setAccount] = useState<AccountProfile | null>(null);
  const [claimableMatches, setClaimableMatches] = useState(0);

  // The server has already rejected these credentials, so clear them from state
  // and storage. Without this the rejected token is restored on the next boot
  // and every authenticated call keeps failing the same way.
  const expireSession = useCallback(async () => {
    setSessionState(null);
    setAccount(null);
    await persistSession(null);
  }, []);

  const bind = useCallback(
    (s: PlayerSession) => {
      apiClient.bindSession(
        s,
        async (access, refresh) => {
          const updated = { ...s, accessToken: access, refreshToken: refresh };
          setSessionState(updated);
          await persistSession(updated);
        },
        () => {
          void expireSession();
        },
      );
    },
    [expireSession],
  );

  const applySession = useCallback(
    async (s: PlayerSession | null) => {
      setSessionState(s);
      await persistSession(s);
      if (s) bind(s);
      if (!s || s.isGuest) setAccount(null);
    },
    [bind],
  );

  // Restore the chosen locale before anything renders, so the first paint is
  // already in the player's language rather than flashing English first.
  useEffect(() => {
    storage.getItem('zolik_locale').then((saved) => {
      if (saved === 'en' || saved === 'cs') setLocale(saved);
    });
  }, []);

  useEffect(() => {
    loadSession()
      .then((s) => {
        if (s) {
          bind(s);
          setSessionState(s);
          setClaimableMatches(s.claimableMatches ?? 0);
        }
      })
      .finally(() => setLoading(false));
  }, [bind]);

  // The sign-in screen is built from what the server actually offers, so a
  // provider enabled server-side appears without an app update. A failure here
  // is not fatal: guest and email are always available.
  useEffect(() => {
    apiClient
      .getAuthProviders()
      .then(setProviders)
      .catch(() => setProviders([]));
  }, []);

  const refreshAccount = useCallback(async () => {
    if (!session || session.isGuest) {
      setAccount(null);
      return;
    }
    try {
      setAccount(await apiClient.getMe());
    } catch {
      /* offline, or the session just expired — the 401 path handles it */
    }
  }, [session]);

  useEffect(() => {
    void refreshAccount();
  }, [refreshAccount]);

  const setSession = useCallback(
    async (s: PlayerSession | null) => {
      await applySession(s);
    },
    [applySession],
  );

  const guestLogin = useCallback(
    async (name: string) => {
      const existing = await loadGuestIdentity();
      const s = await apiClient.guestLogin(name, existing?.guestId);
      if (s.guestId) {
        await storage.setItem(
          GUEST_KEY,
          JSON.stringify({ guestId: s.guestId, refreshToken: s.refreshToken }),
        );
      }
      setClaimableMatches(s.claimableMatches ?? 0);
      await applySession(s);
    },
    [applySession],
  );

  /** Shared tail of every sign-in: adopt the session and forget the guest
   *  identity whose history the server has just moved across. */
  const adopt = useCallback(
    async (outcome: SignInOutcome) => {
      await applySession(outcome.session);
      if (outcome.claimedMatches > 0) {
        await storage.deleteItem(GUEST_KEY);
      }
      setClaimableMatches(0);
      return outcome;
    },
    [applySession],
  );

  const startEmailSignIn = useCallback(async (email: string) => {
    await apiClient.startEmailSignIn(email);
  }, []);

  const verifyEmailCode = useCallback(
    async (email: string, code: string) => adopt(await apiClient.verifyEmailCode(email, code)),
    [adopt],
  );

  /**
   * Runs a provider's browser flow.
   *
   * The current session (guest or signed-in) is already bound to the API
   * client, so `startOAuth` carries it in a header — which is what tells the
   * server to claim this device's guest history, or to link rather than sign
   * in. Nothing sensitive goes in a URL.
   *
   * Returns null when the person simply closed the browser, which is not an
   * error and should not be reported as one.
   */
  const runOAuth = useCallback(
    async (providerId: string, link: boolean): Promise<SignInOutcome | null> => {
      const returnTo = Linking.createURL('/auth/callback');
      const { authorizationUrl } = await apiClient.startOAuth(providerId, returnTo, link);

      const result = await WebBrowser.openAuthSessionAsync(authorizationUrl, returnTo);
      if (result.type !== 'success') return null;

      const callback = parseAuthCallback(result.url);
      if (callback.status === 'cancelled') return null;
      if (callback.status === 'error') throw new Error(authErrorMessage(callback.reason));

      return apiClient.exchangeOAuthCode(callback.code);
    },
    [],
  );

  const signInWithProvider = useCallback(
    async (providerId: string) => {
      const outcome = await runOAuth(providerId, false);
      return outcome ? adopt(outcome) : null;
    },
    [runOAuth, adopt],
  );

  const linkProvider = useCallback(
    async (providerId: string) => {
      // A link flow mints no session — the person is already signed in on this
      // device — so only the account's linked-method list needs refreshing.
      await runOAuth(providerId, true);
      await refreshAccount();
    },
    [runOAuth, refreshAccount],
  );

  const unlinkProvider = useCallback(
    async (providerId: string) => {
      await apiClient.unlinkIdentity(providerId);
      await refreshAccount();
    },
    [refreshAccount],
  );

  const claimGuestHistory = useCallback(async () => {
    const guest = await loadGuestIdentity();
    if (!guest?.refreshToken) return 0;
    const claimed = await apiClient.claimGuestHistory(guest.refreshToken);
    // The guest session is retired server-side; keeping it here would leave a
    // token that no longer works and an id nothing will ever be recorded against.
    await storage.deleteItem(GUEST_KEY);
    setClaimableMatches(0);
    return claimed;
  }, []);

  const login = useCallback(
    async (username: string, password: string) => {
      const s = await apiClient.login(username, password);
      await applySession(s);
    },
    [applySession],
  );

  const register = useCallback(
    async (username: string, password: string, email?: string) => {
      const s = await apiClient.register(username, password, email);
      await applySession(s);
    },
    [applySession],
  );

  const logout = useCallback(async () => {
    await apiClient.logout();
    await applySession(null);
  }, [applySession]);

  const value = useMemo(
    () => ({
      session,
      loading,
      client: apiClient,
      providers,
      account,
      claimableMatches,
      setSession,
      guestLogin,
      startEmailSignIn,
      verifyEmailCode,
      signInWithProvider,
      linkProvider,
      unlinkProvider,
      claimGuestHistory,
      refreshAccount,
      login,
      register,
      logout,
    }),
    [
      session,
      loading,
      providers,
      account,
      claimableMatches,
      setSession,
      guestLogin,
      startEmailSignIn,
      verifyEmailCode,
      signInWithProvider,
      linkProvider,
      unlinkProvider,
      claimGuestHistory,
      refreshAccount,
      login,
      register,
      logout,
    ],
  );

  return (
    <SessionContext.Provider value={value}>{children}</SessionContext.Provider>
  );
}

export function useSession() {
  const ctx = useContext(SessionContext);
  if (!ctx) {
    throw new Error('useSession must be used within SessionProvider');
  }
  return ctx;
}
