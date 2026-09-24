import * as Linking from 'expo-linking';
import * as SecureStore from 'expo-secure-store';
import * as WebBrowser from 'expo-web-browser';
import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import { Platform } from 'react-native';

import { apiClient, ZolikClient } from '@/src/api/client';
import * as nearby from '@/modules/zolik-nearby';
import { connectToTable } from '@/src/net/ble/link';
import { BleTransport } from '@/src/net/ble/transport';
import { authErrorMessage, parseAuthCallback } from '@/src/lib/auth';
import { nearbyBaseUrl } from '@/src/lib/nearbyAddress';
import { ZOLIK_BASE_URL } from '@/src/config';
import { startNodeFor, stopNodeFor } from '@/src/net/nodeSession';
import { useReplicaSync } from '@/src/net/useReplicaSync';
import type {
  AccountProfile,
  AuthProvider,
  PlayerSession,
  SignInOutcome,
} from '@/src/api/types';

// The node credential is kept where the session is: it names this device as
// this person's, and is exactly as sensitive as the session itself.
const nodeCredentialStore = {
  getItem: (key: string) => storage.getItem(key),
  setItem: (key: string, value: string) => storage.setItem(key, value),
  deleteItem: (key: string) => storage.deleteItem(key),
};

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

/**
 * A table this phone hosts itself, with no internet: the embedded server
 * from `modules/zolik-nearby`, reached over loopback.
 *
 * While one is open it *is* the session as far as every screen can tell:
 * `client` and `session` point at it, so the game picker, the table and the
 * match screen work unchanged. The online session is left untouched
 * underneath and comes back as soon as the player leaves.
 */
export type OfflineTable = {
  instanceId: string;
  baseUrl: string;
  /** Whether this phone runs the table or sits at somebody else's. */
  role: 'host' | 'guest';
  /** How a guest reaches the host. A host is always 'self'. */
  via: 'self' | 'wifi' | 'bluetooth';
  /**
   * Over Bluetooth, the four characters both phones can show to check that
   * nobody sits between them. Empty otherwise.
   */
  checkCode: string;
};

/** Thrown when a nearby table runs a different app version than this one. */
export class NearbyVersionError extends Error {
  constructor(readonly theirs: number) {
    super('NEARBY_VERSION_MISMATCH');
  }
}


type OfflineState = OfflineTable & {
  client: ZolikClient;
  session: PlayerSession;
  /** The Bluetooth tunnel, for a guest at a table over Bluetooth. */
  ble?: BleTransport;
};

/** The Bluetooth key a table answered with, pinned per instance id. */
const hostKeyKey = (instanceId: string) => `zolik_hostkey_${instanceId}`;
/** The peripheral a table was last reached at: a first guess, nothing more. */
const hostRadioKey = (instanceId: string) => `zolik_hostradio_${instanceId}`;

/**
 * Whether this phone has sat at a table with that instance id before — its
 * Bluetooth key was pinned then. A nearby banner says so, because "a table
 * nearby" from somebody you have played with is a different invitation from
 * one from a stranger.
 */
export async function isKnownHost(instanceId: string): Promise<boolean> {
  if (!instanceId) return false;
  try {
    return !!(await storage.getItem(hostKeyKey(instanceId)));
  } catch {
    return false;
  }
}

async function pinIfNew(instanceId: string, base64Key: string | undefined) {
  if (!base64Key) return;
  if (!(await storage.getItem(hostKeyKey(instanceId)))) {
    await storage.setItem(hostKeyKey(instanceId), base64Key);
  }
}

/** Per host, because each install's host signs its own tokens. */
const offlineKey = (instanceId: string) => `zolik_offline_${instanceId}`;

/** The name last used at an offline table, offered again next time. */
const OFFLINE_NAME_KEY = 'zolik_offline_name';

function bytesToBase64(b: Uint8Array): string {
  return btoa(String.fromCharCode(...b));
}

function base64ToBytes(s: string): Uint8Array {
  return Uint8Array.from(atob(s), (c) => c.charCodeAt(0));
}

export async function loadOfflineName(): Promise<string | null> {
  return storage.getItem(OFFLINE_NAME_KEY);
}

type SessionContextValue = {
  session: PlayerSession | null;
  /**
   * The online session, even while an offline table stands in for it as
   * `session`. The circle, invites and push all live on the online server,
   * and a player sitting at a table in the room is still somebody their
   * circle can reach.
   */
  onlineSession: PlayerSession | null;
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
  /** The offline table this phone is hosting, or null when playing online. */
  offline: OfflineTable | null;
  /** Starts this phone's own server and seats the player at it. */
  playOffline: (name: string) => Promise<void>;
  /** Seats the player at a table another phone in the room is hosting. */
  joinNearby: (address: string, name: string) => Promise<void>;
  /** The same, over Bluetooth, to a table a scan found. */
  joinBluetooth: (peripheralId: string, name: string) => Promise<void>;
  /** Back to the online server. Tables stay on the phone for next time. */
  leaveOffline: () => Promise<void>;
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
  const [offline, setOffline] = useState<OfflineState | null>(null);

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
        async (access, refresh, offlinePass) => {
          // The pass is renewed with the tokens, and keeping the old one
          // would mean a device that had been signed in for a month could no
          // longer seat its owner at a table with no internet.
          const updated = {
            ...s,
            accessToken: access,
            refreshToken: refresh,
            offlinePass: offlinePass ?? s.offlinePass,
          };
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

  // Whether this device is holding the signed-in account's data, which is
  // what lets a screen read from here instead of the network.
  const [localNodeReady, setLocalNodeReady] = useState(false);
  // Who this device is currently a node for, so a second sign-in by the same
  // person is not a second enrolment and a different person's is a handover.
  const previousAccount = useRef('');

  // Starting and stopping this device as a node of the database. It is
  // best-effort in both directions: a phone that cannot enrol, or an app
  // build with no embedded server in it, goes on reading everything from the
  // cloud exactly as it always has.
  const followAccountOnThisDevice = useCallback(async (s: PlayerSession | null) => {
    try {
      if (!s || s.isGuest) {
        await stopNodeFor(previousAccount.current, nodeCredentialStore);
        previousAccount.current = '';
        setLocalNodeReady(false);
        return;
      }
      if (previousAccount.current === s.userId) return;
      if (previousAccount.current) await stopNodeFor(previousAccount.current, nodeCredentialStore);
      previousAccount.current = s.userId;
      const started = await startNodeFor(
        s.userId,
        nodeCredentialStore,
        (pubkey, kind) => apiClient.enrollNode(pubkey, kind),
        ZOLIK_BASE_URL,
      );
      setLocalNodeReady(started);
    } catch (err) {
      // Offline at sign-in, or a build with no host. Neither is worth telling
      // somebody about: the device holds nothing yet, and the next sign-in
      // tries again. It is logged, because a device that quietly never became
      // a node looks exactly like one that did until somebody goes looking
      // for their history.
      console.warn('this device did not start holding its account:', err);
      setLocalNodeReady(false);
    }
  }, []);

  const applySession = useCallback(
    async (s: PlayerSession | null) => {
      setSessionState(s);
      await persistSession(s);
      if (s) bind(s);
      if (!s || s.isGuest) setAccount(null);
      // A signed-in account's own data belongs on their device: their
      // settings, their circle, their scorepads and the matches they have
      // played, all readable with no connection. Enrolling the device is
      // what makes that possible, and it happens once, here, rather than
      // being something a person is asked about.
      void followAccountOnThisDevice(s);
    },
    [bind, followAccountOnThisDevice],
  );

  useReplicaSync(localNodeReady);

  useEffect(() => {
    loadSession()
      .then((s) => {
        if (s) {
          bind(s);
          setSessionState(s);
          setClaimableMatches(s.claimableMatches ?? 0);
          // Also on a cold start, not only when somebody signs in: the
          // session is usually one that was already stored, and a device
          // that only became a node at sign-in would stop being one the
          // first time the app was closed and opened again.
          void followAccountOnThisDevice(s);
        }
      })
      .finally(() => setLoading(false));
  }, [bind, followAccountOnThisDevice]);

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
    // Taken back while the token still names whose device this is, so the
    // next person on this phone is not woken for the last one's tables.
    // Loaded lazily: pushDevice reads this module's storage.
    await (await import('@/src/notify/pushDevice')).unregisterDevice();
    await apiClient.logout();
    await applySession(null);
  }, [applySession]);

  /** Signs in at a table's server and makes it the session. */
  const sitAt = useCallback(
    async (
      client: ZolikClient,
      instanceId: string,
      table: Pick<OfflineTable, 'role' | 'via' | 'checkCode'>,
      name: string,
      ble?: BleTransport,
      offlinePass?: string,
    ) => {
      const key = offlineKey(instanceId);
      // The guest id is kept per host, so this player gets the same seat back
      // at a table they left, even across app restarts. Signing in afresh each
      // time costs nothing on the local network, and it never leaves a stale
      // token that the next call finds out about.
      let known: string | undefined;
      try {
        known = (JSON.parse((await storage.getItem(key)) ?? '{}') as { guestId?: string }).guestId;
      } catch {
        known = undefined;
      }
      // A signed-in person sits at their own table as themselves: the pass
      // the cloud gave them says who they are, and the host checks it against
      // the keys it cached while it last had a connection. Whatever they play
      // is then theirs when it reaches the cloud, rather than a stranger's
      // that has to be claimed afterwards.
      //
      // A host that has never been online, or a pass that has expired,
      // refuses it, and this falls back to sitting as a guest - which is what
      // every offline table did before any of this existed, and still works.
      let s: PlayerSession | null = null;
      if (offlinePass) {
        try {
          s = await client.offlinePassLogin(offlinePass);
        } catch {
          s = null;
        }
      }
      if (!s) s = await client.guestLogin(name, known);
      if (s.guestId) await storage.setItem(key, JSON.stringify({ guestId: s.guestId }));
      await storage.setItem(OFFLINE_NAME_KEY, s.username);
      client.bindSession(
        s,
        (access, refresh) =>
          setOffline((prev) =>
            prev && prev.client === client
              ? { ...prev, session: { ...prev.session, accessToken: access, refreshToken: refresh } }
              : prev,
          ),
        () => setOffline((prev) => (prev && prev.client === client ? null : prev)),
      );
      setOffline({ instanceId, baseUrl: client.baseUrl, ...table, client, session: s, ble });
    },
    [],
  );

  const playOffline = useCallback(
    async (name: string) => {
      const host = await nearby.startHost();
      await sitAt(
        new ZolikClient(host.baseUrl),
        host.instanceId,
        { role: 'host', via: 'self', checkCode: '' },
        name,
        undefined,
        session?.isGuest ? undefined : session?.offlinePass,
      );
    },
    [sitAt, session],
  );

  const joinNearby = useCallback(
    async (address: string, name: string) => {
      const baseUrl = nearbyBaseUrl(address);
      // Asked of the host itself rather than trusted from the advertisement,
      // because a typed or scanned address has no advertisement behind it.
      const res = await fetch(`${baseUrl}/nearby/info`);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const info = (await res.json()) as { protocol?: number; instanceId?: string; publicKey?: string };
      if (info.protocol !== nearby.PROTOCOL_VERSION) {
        throw new NearbyVersionError(info.protocol ?? 0);
      }
      if (!info.instanceId) throw new Error('not a Zolik table');
      // Met over Wi-Fi first, the table's Bluetooth key is pinned now, so a
      // later Bluetooth join to it is checked rather than trusted.
      await pinIfNew(info.instanceId, info.publicKey);
      await sitAt(
        new ZolikClient(baseUrl),
        info.instanceId,
        { role: 'guest', via: 'wifi', checkCode: '' },
        name,
      );
    },
    [sitAt],
  );

  const joinBluetooth = useCallback(
    async (peripheralId: string, name: string) => {
      // The first link is opened here, to read which table this is before
      // anything is spent on it. Later links (after a drop) find the table
      // again by its instance id.
      let first: nearby.BleConnection | null = await nearby.bleConnect(peripheralId);
      const info = first.info;
      if (info.v !== nearby.PROTOCOL_VERSION) {
        first.close();
        throw new NearbyVersionError(info.v ?? 0);
      }
      await storage.setItem(hostRadioKey(info.id), peripheralId);
      const transport = new BleTransport({
        instanceId: info.id,
        connect: async () => {
          if (first) {
            const link = first;
            first = null;
            return link;
          }
          const last = await storage.getItem(hostRadioKey(info.id));
          return connectToTable(info.id, last, (id) => void storage.setItem(hostRadioKey(info.id), id));
        },
        pinnedKey: async () => {
          const k = await storage.getItem(hostKeyKey(info.id));
          return k ? base64ToBytes(k) : null;
        },
        pinKey: (k) => storage.setItem(hostKeyKey(info.id), bytesToBase64(k)),
        random32: () => nearby.randomBytes(32),
        // After a reconnect the code changes, on the host's screen too.
        onCheckCode: (code) =>
          setOffline((prev) => (prev && prev.ble === transport ? { ...prev, checkCode: code } : prev)),
      });
      try {
        await transport.ready();
      } catch (e) {
        transport.close();
        throw e;
      }
      const client = new ZolikClient(`ble://${info.id}`, transport);
      await sitAt(
        client,
        info.id,
        { role: 'guest', via: 'bluetooth', checkCode: transport.checkCode },
        name,
        transport,
      );
    },
    [sitAt],
  );

  const leaveOffline = useCallback(async () => {
    const was = offline;
    setOffline(null);
    // A host has a server and a radio running here. A guest leaving just
    // stops talking to somebody else's phone.
    was?.ble?.close();
    if (was?.role === 'host') {
      await nearby.closeRoom();
      await nearby.bleHostStop();
      await nearby.stopHost();
    }
  }, [offline]);

  const offlineTable = useMemo(
    () =>
      offline
        ? {
            instanceId: offline.instanceId,
            baseUrl: offline.baseUrl,
            role: offline.role,
            via: offline.via,
            checkCode: offline.checkCode,
          }
        : null,
    [offline],
  );

  const value = useMemo(
    () => ({
      session: offline ? offline.session : session,
      onlineSession: session,
      loading,
      client: offline ? offline.client : apiClient,
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
      offline: offlineTable,
      playOffline,
      joinNearby,
      joinBluetooth,
      leaveOffline,
    }),
    [
      offline,
      offlineTable,
      playOffline,
      joinNearby,
      joinBluetooth,
      leaveOffline,
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
