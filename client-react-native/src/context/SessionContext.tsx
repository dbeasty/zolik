import * as SecureStore from 'expo-secure-store';
import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from 'react';

import { apiClient } from '@/src/api/client';
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

async function persistSession(session: PlayerSession | null) {
  if (session) {
    await SecureStore.setItemAsync(SESSION_KEY, JSON.stringify(session));
  } else {
    await SecureStore.deleteItemAsync(SESSION_KEY);
  }
}

async function loadSession(): Promise<PlayerSession | null> {
  const raw = await SecureStore.getItemAsync(SESSION_KEY);
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

  const applySession = useCallback(async (s: PlayerSession | null) => {
    setSessionState(s);
    await persistSession(s);
    if (s) {
      apiClient.bindSession(s, async (access, refresh) => {
        const updated = { ...s, accessToken: access, refreshToken: refresh };
        setSessionState(updated);
        await persistSession(updated);
      });
    }
  }, []);

  useEffect(() => {
    loadSession()
      .then((s) => {
        if (s) {
          apiClient.bindSession(s, async (access, refresh) => {
            const updated = { ...s, accessToken: access, refreshToken: refresh };
            setSessionState(updated);
            await persistSession(updated);
          });
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
