import { ZOLIK_BASE_URL } from '@/src/config';
import type { MatchModule, MatchState, ModuleRules } from '@/src/api/matchTypes';
import type {
  AccountProfile,
  AuthProvider,
  CapacitySnapshot,
  LifetimeStats,
  LinkedIdentity,
  PlayerSession,
  SignInOutcome,
  WaitingPlayer,
} from '@/src/api/types';

export class ApiError extends Error {
  constructor(
    message: string,
    public status?: number,
    public code?: string,
    public retryAfterMs?: number,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

type TokenHolder = {
  accessToken: string;
  refreshToken: string;
  onTokensUpdated?: (access: string, refresh: string) => void;
};

/** The shape every sign-in endpoint answers with, whichever door was used. */
type SignInResponse = {
  accessToken: string;
  refreshToken: string;
  userId: string;
  username: string;
  created?: boolean;
  claimedMatches?: number;
};

export class ZolikClient {
  baseUrl: string;
  accessToken = '';
  refreshToken = '';
  userId = '';
  /**
   * The face to take to any seat this client sits down in.
   *
   * Held here rather than passed to each call for the same reason the token
   * is: three doors lead to a seat — creating a table, joining one, and
   * waiting to be picked up out of the pool — and a face that only some of
   * them carried would be a face that sometimes changed on the way in.
   *
   * Empty is an ordinary state, meaning "never chose one"; the server stores
   * nothing and every client derives the same face from the player id.
   */
  avatarId = '';
  private onTokensUpdated?: (access: string, refresh: string) => void;
  private onSessionExpired?: () => void;

  constructor(baseUrl: string = ZOLIK_BASE_URL) {
    this.baseUrl = baseUrl.replace(/\/$/, '');
  }

  bindSession(
    session: PlayerSession,
    onUpdate?: (access: string, refresh: string) => void,
    onExpired?: () => void,
  ) {
    this.accessToken = session.accessToken;
    this.refreshToken = session.refreshToken;
    this.userId = session.userId;
    this.onTokensUpdated = onUpdate;
    this.onSessionExpired = onExpired;
  }

  /** The waiting room's socket: connecting to it *is* "I'm waiting to be
   *  picked up" — the only thing negotiated on open beyond the token is the
   *  face to be seen waiting under, so a host invites the person they saw. */
  lobbyWsUrl(): string {
    const u = new URL(this.baseUrl);
    const scheme = u.protocol === 'https:' ? 'wss' : 'ws';
    const token = encodeURIComponent(this.accessToken);
    const face = this.avatarId ? `&avatar=${encodeURIComponent(this.avatarId)}` : '';
    return `${scheme}://${u.host}/ws/lobby?token=${token}${face}`;
  }

  /** A snapshot of who's currently waiting, for a host browsing whom to
   *  invite. Polled rather than streamed — the host's one socket is
   *  usually already spent on their own match's room. */
  async getWaitingLobby(): Promise<WaitingPlayer[]> {
    const data = await this.get<{ players: WaitingPlayer[] }>('/lobby/waiting', true);
    return data.players ?? [];
  }

  /** Seats a specific waiting player directly into this lobby, no join code
   *  needed. Host-only; the server re-checks the target is still actually
   *  waiting before seating them. */
  async invitePlayer(idOrJoin: string, playerId: string): Promise<{ alreadyJoined?: boolean }> {
    return this.post(`/matches/${encodeURIComponent(idOrJoin)}/invite`, { playerId }, true);
  }

  /**
   * Starts a guest session, reusing this device's guest identity when it has
   * one.
   *
   * Passing the existing id back is what keeps a guest's play attributable to
   * one device across sessions, and therefore what makes it claimable when
   * they eventually sign in. Without it every launch would look like a new
   * person and the history would be unreachable.
   */
  async guestLogin(name: string, guestId?: string): Promise<PlayerSession> {
    const data = await this.post<{
      accessToken: string;
      refreshToken: string;
      guestName: string;
      guestId: string;
      userId: string;
      claimableMatches?: number;
    }>('/auth/guest', { guestName: name || 'Player', guestId: guestId || undefined }, false);
    this.accessToken = data.accessToken;
    this.refreshToken = data.refreshToken;
    this.userId = data.userId || data.guestId;
    return {
      accessToken: data.accessToken,
      refreshToken: data.refreshToken,
      userId: this.userId,
      username: data.guestName || name || 'Guest',
      isGuest: true,
      guestId: data.guestId,
      claimableMatches: data.claimableMatches ?? 0,
    };
  }

  /** The sign-in methods this deployment offers. */
  async getAuthProviders(): Promise<AuthProvider[]> {
    const data = await this.get<{ providers: AuthProvider[] }>('/auth/providers', false);
    return data.providers ?? [];
  }

  /** Mails a one-time code. Says nothing about whether the address has an
   *  account here — that would make the endpoint a membership oracle. */
  async startEmailSignIn(email: string): Promise<void> {
    await this.post('/auth/email/start', { email }, false);
  }

  /**
   * Redeems a mailed code.
   *
   * Sent with the current session's Authorization header when there is one, so
   * a guest signing in has their play history claimed as part of the same
   * call rather than needing a second, racier round trip.
   */
  async verifyEmailCode(email: string, code: string): Promise<SignInOutcome> {
    return this.toOutcome(
      await this.post<SignInResponse>('/auth/email/verify', { email, code }, true),
    );
  }

  /**
   * Asks the server to begin a browser sign-in and returns the URL to open.
   *
   * A POST rather than opening a URL directly, so the current session travels
   * in a header instead of a query string — that header is what tells the
   * server whether this is a guest upgrade, a link, or a plain sign-in.
   */
  async startOAuth(
    provider: string,
    returnTo: string,
    link = false,
  ): Promise<{ authorizationUrl: string; returnTo: string }> {
    return this.post(
      `/auth/oauth/${encodeURIComponent(provider)}/start`,
      { returnTo, link },
      true,
    );
  }

  /** Swaps the one-time code from the callback for real tokens. */
  async exchangeOAuthCode(code: string): Promise<SignInOutcome> {
    return this.toOutcome(await this.post<SignInResponse>('/auth/oauth/exchange', { code }, false));
  }

  /** Signs in with an ID token from a native SDK (Google Sign-In, Sign in
   *  with Apple, MSAL) — no browser involved. */
  async signInWithIdToken(
    provider: string,
    idToken: string,
    opts: { nonce?: string; link?: boolean } = {},
  ): Promise<SignInOutcome> {
    return this.toOutcome(
      await this.post<SignInResponse>(
        `/auth/oauth/${encodeURIComponent(provider)}/token`,
        { idToken, nonce: opts.nonce, link: opts.link },
        true,
      ),
    );
  }

  /**
   * Moves this device's guest play onto the signed-in account.
   *
   * Takes the guest *refresh token* rather than the guest id, because the id
   * travels in game state and match records — possession of the session is
   * what actually distinguishes the owner of that history.
   */
  async claimGuestHistory(guestRefreshToken: string): Promise<number> {
    const data = await this.post<{ claimedMatches: number }>(
      '/auth/claim-guest',
      { guestRefreshToken },
      true,
    );
    return data.claimedMatches ?? 0;
  }

  /** What this guest session stands to keep by signing in. */
  async getGuestSummary(): Promise<{ guestId: string; claimableMatches: number }> {
    return this.get('/auth/guest-summary', true);
  }

  async getIdentities(): Promise<LinkedIdentity[]> {
    const data = await this.get<{ identities: LinkedIdentity[] }>('/auth/identities', true);
    return data.identities ?? [];
  }

  async unlinkIdentity(provider: string): Promise<void> {
    await this.request('DELETE', `/auth/identities/${encodeURIComponent(provider)}`, undefined, true);
  }

  async register(username: string, password: string, email?: string): Promise<PlayerSession> {
    const data = await this.post<{ accessToken: string; refreshToken: string }>(
      '/auth/register',
      { username, password, email: email || undefined },
      false,
    );
    this.accessToken = data.accessToken;
    this.refreshToken = data.refreshToken;
    await this.loadUserId();
    return this.toSession(username, false);
  }

  async login(username: string, password: string): Promise<PlayerSession> {
    const data = await this.post<{ accessToken: string; refreshToken: string }>(
      '/auth/login',
      { username, password },
      false,
    );
    this.accessToken = data.accessToken;
    this.refreshToken = data.refreshToken;
    await this.loadUserId();
    return this.toSession(username, false);
  }

  async logout(): Promise<void> {
    if (this.refreshToken) {
      try {
        await this.post('/auth/logout', { refreshToken: this.refreshToken }, false);
      } catch {
        /* ignore */
      }
    }
    this.accessToken = '';
    this.refreshToken = '';
    this.userId = '';
  }

  async refreshTokens(): Promise<void> {
    if (!this.refreshToken) {
      throw new ApiError('no refresh token');
    }
    const data = await this.post<{ accessToken: string; refreshToken: string }>(
      '/auth/refresh',
      { refreshToken: this.refreshToken },
      false,
    );
    this.accessToken = data.accessToken;
    this.refreshToken = data.refreshToken;
    this.onTokensUpdated?.(data.accessToken, data.refreshToken);
  }

  // --- matches -------------------------------------------------------------
  //
  // Six methods, none of which names a game. What they replaced —
  // createGame/updateGameSettings/joinGame/addAI/startGame/getLobby/getRules/
  // getModuleDescriptor/getScoreboard, plus a WebSocket URL builder and a
  // typed rummy action sender — was the same six ideas with rummy baked into
  // each one.

  /** Every game this server hosts, and what each one lets a lobby set. */
  async modules(): Promise<MatchModule[]> {
    const body = await this.get<{ modules: MatchModule[] }>('/modules', false);
    return body.modules ?? [];
  }

  /**
   * Which build the server is running, for the footer next to our own — so a
   * bug report says which server the reporter was actually talking to.
   * Unauthenticated, and deliberately outside the 401-refresh retry path
   * (auth=false): this is a probe fired before login, not an API call that
   * should ever trigger a token refresh.
   */
  async getVersion(): Promise<{ version: string; commit: string }> {
    return this.get('/version', false);
  }

  /** Whether the server is accepting new connections. Probed after a refused
   *  WebSocket handshake, since React Native cannot read the HTTP status. */
  async getCapacity(): Promise<CapacitySnapshot> {
    return this.get('/healthz/capacity', false);
  }

  /**
   * One module's written rules, resolved against a variation and option
   * overrides — the same choices a lobby's picker holds, so the sentences
   * describe the table being configured rather than the module's defaults.
   */
  async moduleRules(
    moduleId: string,
    variation?: string,
    options?: Record<string, number>,
  ): Promise<ModuleRules> {
    const q = new URLSearchParams();
    if (variation) q.set('variation', variation);
    for (const [name, value] of Object.entries(options ?? {})) {
      q.set(`opt.${name}`, String(value));
    }
    const qs = q.toString();
    return this.get<ModuleRules>(
      `/modules/${encodeURIComponent(moduleId)}/rules${qs ? `?${qs}` : ''}`,
      false,
    );
  }

  async createMatch(
    moduleId: string,
    variation?: string,
    options: Record<string, number> = {},
  ): Promise<{ matchId: string; joinCode: string }> {
    return this.post('/matches', { moduleId, variation, options, avatar: this.avatarId }, true);
  }

  /** Joins by match id or by the short code a host reads out. */
  async joinMatch(idOrCode: string): Promise<string> {
    const data = await this.post<{ matchId: string }>(
      `/matches/${encodeURIComponent(idOrCode)}/join`,
      { avatar: this.avatarId },
      true,
    );
    return data.matchId;
  }

  /**
   * Seat a bot.
   *
   * `skill` overrides the table's own Opponents setting for this one seat,
   * which is how a deliberately mixed table gets built. Omitting it — which is
   * what the games screen does — lets the server answer from the match's
   * botSkill option, including drawing a strength per seat under Mixed.
   */
  async addBot(
    idOrCode: string,
    skill?: string,
  ): Promise<{ playerId: string; name?: string; skill?: string; aiPersona?: string }> {
    return this.post(
      `/matches/${encodeURIComponent(idOrCode)}/add-bot`,
      skill ? { skill } : null,
      true,
    );
  }

  async startMatch(idOrCode: string): Promise<void> {
    await this.post(`/matches/${encodeURIComponent(idOrCode)}/start`, null, true);
  }

  /** A viewer's state over plain HTTP; the socket is the live path. */
  async getMatch(idOrCode: string, as?: string): Promise<MatchState> {
    const q = as ? `?as=${encodeURIComponent(as)}` : '';
    return this.get(`/matches/${encodeURIComponent(idOrCode)}${q}`, false);
  }

  /** The socket that carries actions in and per-viewer state out. */
  matchSocketUrl(matchId: string): string {
    const u = new URL(this.baseUrl);
    const scheme = u.protocol === 'https:' ? 'wss' : 'ws';
    return `${scheme}://${u.host}/ws/matches/${encodeURIComponent(matchId)}?token=${encodeURIComponent(
      this.accessToken,
    )}`;
  }

  async getMe(): Promise<AccountProfile> {
    return this.get('/users/me', true);
  }

  /**
   * Renames the account or updates its preferences.
   *
   * Preferences are sent whole rather than as a patch of one key, because the
   * server sets the object whole — a half-populated one would quietly clear
   * the rest. Callers spread the current preferences and change the one they
   * mean.
   */
  async updateMe(patch: {
    username?: string;
    preferences?: { language?: string; cardStyle?: string; avatar?: string };
  }): Promise<void> {
    await this.request('PATCH', '/users/me', patch, true);
  }

  /**
   * A registered player's lifetime record.
   *
   * Rejected for guests, by design: a guest identity is per-device and keyed on
   * a claimable display name, so a lifetime record for one would merge
   * strangers' histories. Guests claim theirs by registering — see
   * `claimGuestHistory`.
   */
  async getStats(): Promise<LifetimeStats> {
    return this.get('/users/me/stats', true) as Promise<LifetimeStats>;
  }

  async getHistory(): Promise<unknown> {
    return this.get('/users/me/history', true);
  }

  /** Paged match history, newest first. Pass the previous page's
   *  `nextBefore` as `before` to fetch the next one. */
  async getMatches(opts: { before?: string; limit?: number } = {}): Promise<unknown> {
    return this.get(`/users/me/matches${queryString(opts)}`, true);
  }

  /** This player's record against each opponent they have faced, bots
   *  included — a bot keeps a lifetime record just as a person does. */
  async getHeadToHead(): Promise<unknown> {
    return this.get('/users/me/head-to-head', true);
  }

  /** `scope` picks which record ranks: 'overall' | 'vs_humans' | 'vs_ai'.
   *  `kind` defaults to human players; pass 'ai' for the bot standings. */
  async getLeaderboard(
    opts: { scope?: string; kind?: string; minMatches?: number; limit?: number } = {},
  ): Promise<unknown> {
    return this.get(`/leaderboard${queryString(opts)}`, false);
  }



  async createScoringSession(players: string[]): Promise<string> {
    const data = await this.post<{ id: string }>(
      '/scoring-sessions',
      { players },
      false,
    );
    return data.id;
  }

  async getScoringSession(id: string): Promise<Record<string, unknown>> {
    return this.get(`/scoring-sessions/${encodeURIComponent(id)}`, false);
  }

  async patchScoringSession(
    id: string,
    round: number,
    scores: Record<string, number>,
  ): Promise<void> {
    await this.patch(`/scoring-sessions/${encodeURIComponent(id)}`, { round, scores });
  }

  async exportScoringSession(id: string): Promise<string> {
    const res = await fetch(
      `${this.baseUrl}/scoring-sessions/${encodeURIComponent(id)}/export`,
    );
    const text = await res.text();
    if (!res.ok) {
      throw new ApiError(text || `HTTP ${res.status}`, res.status);
    }
    return text;
  }

  /** Adopts a completed sign-in as the current session. One place, so every
   *  sign-in path leaves the client in the same state. */
  private toOutcome(data: SignInResponse): SignInOutcome {
    this.accessToken = data.accessToken;
    this.refreshToken = data.refreshToken;
    this.userId = data.userId;
    return {
      session: {
        accessToken: data.accessToken,
        refreshToken: data.refreshToken,
        userId: data.userId,
        username: data.username,
        isGuest: false,
      },
      claimedMatches: data.claimedMatches ?? 0,
      created: data.created ?? false,
    };
  }

  private toSession(username: string, isGuest: boolean): PlayerSession {
    return {
      accessToken: this.accessToken,
      refreshToken: this.refreshToken,
      userId: this.userId,
      username,
      isGuest,
    };
  }

  private async loadUserId(): Promise<void> {
    try {
      const me = await this.getMe();
      if (me.id) {
        this.userId = me.id;
        return;
      }
    } catch {
      /* guest or offline */
    }
    this.userId = this.accessToken;
  }

  private async get<T>(path: string, auth: boolean): Promise<T> {
    return this.request<T>('GET', path, undefined, auth);
  }

  private async post<T>(path: string, body: unknown, auth: boolean): Promise<T> {
    return this.request<T>('POST', path, body, auth);
  }

  private async patch(path: string, body: unknown): Promise<void> {
    await this.request('PATCH', path, body, false);
  }

  private async request<T>(
    method: string,
    path: string,
    body: unknown,
    auth: boolean,
    retried = false,
  ): Promise<T> {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    };
    if (auth && this.accessToken) {
      headers.Authorization = `Bearer ${this.accessToken}`;
    }
    const res = await fetch(`${this.baseUrl}${path}`, {
      method,
      headers,
      body: body != null ? JSON.stringify(body) : undefined,
    });
    if (res.status === 401 && auth && this.refreshToken && !retried) {
      try {
        await this.refreshTokens();
      } catch {
        // The stored refresh token is gone for good: expired and reaped by the
        // sessions TTL index, rotated away, or issued by a database we are no
        // longer talking to. Letting it sit in storage wedges the app forever,
        // since every reload restores it and replays this same failure. Drop it
        // and surface a session-expired error the UI can route back to sign-in.
        this.accessToken = '';
        this.refreshToken = '';
        this.userId = '';
        this.onSessionExpired?.();
        throw new ApiError('session expired, please sign in again', 401);
      }
      return this.request<T>(method, path, body, auth, true);
    }
    const text = await res.text();
    if (!res.ok) {
      throw apiErrorFromResponse(text, res.status, res.headers);
    }
    if (!text) {
      return undefined as T;
    }
    return JSON.parse(text) as T;
  }
}

/** Builds a `?a=1&b=2` suffix, dropping empty values and returning '' when
 *  nothing is set. Hand-rolled rather than using URLSearchParams, whose React
 *  Native polyfill implements only part of the interface. */
function queryString(params: Record<string, string | number | undefined>): string {
  const parts: string[] = [];
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === '') continue;
    parts.push(`${encodeURIComponent(key)}=${encodeURIComponent(String(value))}`);
  }
  return parts.length ? `?${parts.join('&')}` : '';
}

export const apiClient = new ZolikClient();

function parseRetryAfterMs(header: string | null): number | undefined {
  if (!header) return undefined;
  const trimmed = header.trim();
  const seconds = parseInt(trimmed, 10);
  if (!Number.isNaN(seconds)) return seconds * 1000;
  const when = Date.parse(trimmed);
  if (!Number.isNaN(when)) return Math.max(0, when - Date.now());
  return undefined;
}

export function apiErrorFromResponse(
  text: string,
  status: number,
  headers: { get(name: string): string | null },
): ApiError {
  let message = text.trim() || `HTTP ${status}`;
  let code: string | undefined;
  try {
    const body = JSON.parse(text) as { code?: string; message?: string };
    if (body.code) code = body.code;
    if (body.message) message = body.message;
    else if (body.code) message = body.code;
  } catch {
    /* keep raw text */
  }
  return new ApiError(message, status, code, parseRetryAfterMs(headers.get('Retry-After')));
}
