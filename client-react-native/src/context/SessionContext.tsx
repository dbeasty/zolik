import * as SecureStore from 'expo-secure-store';
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
import type { PlayerSession } from '@/src/api/types';

const SESSION_KEY = 'zolik_session';

type SessionContextValue = {
  session: PlayerSession | null;
  loading: boolean;
  client: typeof apiClient;
  setSession: (s: PlayerSession | null) => Promise<void>;
  guestLogin: (name: string) => Promise<void>;
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

export function SessionProvider({ children }: { children: React.ReactNode }) {
  const [session, setSessionState] = useState<PlayerSession | null>(null);
  const [loading, setLoading] = useState(true);

  // The server has already rejected these credentials, so clear them from state
  // and storage. Without this the rejected token is restored on the next boot
  // and every authenticated call keeps failing the same way.
  const expireSession = useCallback(async () => {
    setSessionState(null);
    await persistSession(null);
  }, []);

  const applySession = useCallback(async (s: PlayerSession | null) => {
    setSessionState(s);
    await persistSession(s);
    if (s) {
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
    }
  }, [expireSession]);

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
          setSessionState(s);
        }
      })
      .finally(() => setLoading(false));
  }, []);

  const setSession = useCallback(
    async (s: PlayerSession | null) => {
      await applySession(s);
    },
    [applySession],
  );

  const guestLogin = useCallback(
    async (name: string) => {
      const s = await apiClient.guestLogin(name);
      await applySession(s);
    },
    [applySession],
  );

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
      setSession,
      guestLogin,
      login,
      register,
      logout,
    }),
    [session, loading, setSession, guestLogin, login, register, logout],
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
