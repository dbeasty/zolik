import { ZOLIK_BASE_URL } from '@/src/config';
import { HttpTransport, type SocketLike, type Transport } from '@/src/net/transport';
import type { MatchModule, MatchState, ModuleRules, Replay, StoredTable } from '@/src/api/matchTypes';
import type {
  AccountProfile,
  AuthProvider,
  CapacitySnapshot,
  CircleEntry,
  CircleLists,
  CircleSuggestion,
  FriendPreview,
  InvitePreference,
  LifetimeStats,
  LinkedIdentity,
  NotifyConfig,
  NotifyProfile,
  PlayerSession,
  PushDeviceRegistration,
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
  /** The refresh every 401 is waiting on, while one is out. */
  private refreshInFlight?: Promise<void>;

  /** How requests and sockets reach the server. See src/net/transport.ts. */
  readonly transport: Transport;

  constructor(baseUrl: string = ZOLIK_BASE_URL, transport?: Transport) {
    this.baseUrl = baseUrl.replace(/\/$/, '');
    this.transport = transport ?? new HttpTransport(this.baseUrl);
  }

  /** Opens a socket at an address this client's `*SocketUrl` produced. */
  openSocket(url: string): SocketLike {
    return this.transport.openSocket(url);
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
    const token = encodeURIComponent(this.accessToken);
    const face = this.avatarId ? `&avatar=${encodeURIComponent(this.avatarId)}` : '';
    return this.transport.socketUrl(`/ws/lobby?token=${token}${face}`);
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
      // A name is sent only when there is one. An empty field means "you
      // pick", and the server picks from the device's guest id — see
      // src/lib/guestName.ts for why neither side answers "Player".
    }>('/auth/guest', { guestName: name || undefined, guestId: guestId || undefined }, false);
    this.accessToken = data.accessToken;
    this.refreshToken = data.refreshToken;
    this.userId = data.userId || data.guestId;
    return {
      accessToken: data.accessToken,
      refreshToken: data.refreshToken,
      userId: this.userId,
      // The server's answer is the one that counts: when no name was sent it
      // is the name it invented, and that is what the table will show.
      username: data.guestName || name,
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

  /**
   * Trades the refresh token for a new pair. Every caller shares one request:
   * the server retires a refresh token the moment it is used, so two requests
   * that came back 401 together and each refreshed on its own would spend the
   * same token twice — the first succeeds, the second is refused, and that
   * refusal used to sign the player out mid-game.
   */
  refreshTokens(): Promise<void> {
    if (!this.refreshInFlight) {
      this.refreshInFlight = this.exchangeRefreshToken().finally(() => {
        this.refreshInFlight = undefined;
      });
    }
    return this.refreshInFlight;
  }

  private async exchangeRefreshToken(): Promise<void> {
    const spent = this.refreshToken;
    if (!spent) {
      throw new ApiError('no refresh token', 401);
    }
    const data = await this.post<{ accessToken: string; refreshToken: string }>(
      '/auth/refresh',
      { refreshToken: spent },
      false,
    );
    // Signed out or signed in as someone else while this was in the air: the
    // answer belongs to a session that is no longer bound.
    if (this.refreshToken !== spent) return;
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
  ): Promise<{ matchId: string; joinCode: string; inviteUrl?: string }> {
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

  /**
   * Put the table in a given seat order, before it is dealt. Host only.
   *
   * This is how partnerships are chosen. In a game with sides the turn
   * alternates between them, so a side is a position in the seating — which
   * means there is nothing to send but the order, and no separate idea of a
   * "team" on the wire at all.
   */
  async seatTable(
    idOrCode: string,
    order: string[],
  ): Promise<{ order: string[]; sides?: string[][] }> {
    return this.post(`/matches/${encodeURIComponent(idOrCode)}/seats`, { order }, true);
  }

  /**
   * Bring back a table the server swept up after nobody came back to it.
   *
   * Only ever offered for a table whose other seats are all bots — the server
   * enforces that rather than trusting the screen, and answers
   * TABLE_HAS_OTHER_PLAYERS if the screen asks anyway.
   */
  async resumeMatch(idOrCode: string): Promise<void> {
    await this.post(`/matches/${encodeURIComponent(idOrCode)}/resume`, null, true);
  }

  /** A viewer's state over plain HTTP; the socket is the live path. */
  async getMatch(idOrCode: string, as?: string): Promise<MatchState> {
    const q = as ? `?as=${encodeURIComponent(as)}` : '';
    return this.get(`/matches/${encodeURIComponent(idOrCode)}${q}`, false);
  }

  /**
   * Every stored game this player is seated at — unfinished by default, or
   * the finished tab. Works for a guest exactly as it does for an account:
   * the server keys the list on the same subject either carries.
   */
  async listMyTables(scope: 'unfinished' | 'finished' = 'unfinished'): Promise<StoredTable[]> {
    const q = scope === 'finished' ? '?status=finished' : '';
    const data = await this.get<{ tables: StoredTable[] }>(`/users/me/tables${q}`, true);
    return data.tables;
  }

  /**
   * A stopped game, played back frame by frame.
   *
   * Seated players only, and always from the caller's own seat — there is no
   * way to ask for somebody else's view of it. A finished game comes back
   * with every hand face up; anything still resumable does not.
   *
   * Paged: `from` and `limit` window the frames, and `total` says how long
   * the match actually is, so a scrub bar is drawable from the first page.
   */
  async getReplay(idOrCode: string, from = 0, limit = 100): Promise<Replay> {
    const q = `?from=${from}&limit=${limit}`;
    return this.get<Replay>(`/matches/${encodeURIComponent(idOrCode)}/replay${q}`, true);
  }

  /**
   * Ends a table outright — host only, in any status, for every seat. See
   * `server/internal/match/presence.go`'s `DeleteAsHost` for the rule this
   * enforces; the client offers the button only where `canDelete` said so,
   * but the server is the one that actually holds the line.
   */
  async deleteMatch(idOrCode: string): Promise<void> {
    await this.del(`/matches/${encodeURIComponent(idOrCode)}`, true);
  }

  /** The socket that carries actions in and per-viewer state out. */
  matchSocketUrl(matchId: string): string {
    return this.transport.socketUrl(
      `/ws/matches/${encodeURIComponent(matchId)}?token=${encodeURIComponent(this.accessToken)}`,
    );
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



  // --- notifications: the game circle and invites ------------------------
  //
  // One method per route in docs/notifications-plan.md. Every one of them
  // speaks to the online server, never to a table on a phone in the room:
  // callers use `apiClient`, not the session's client, for exactly that
  // reason.

  /** The personal socket that carries invites to whichever screen is open. */
  meWsUrl(): string {
    return this.transport.socketUrl(`/ws/me?token=${encodeURIComponent(this.accessToken)}`);
  }

  async getNotifyProfile(): Promise<NotifyProfile> {
    return this.get('/notify/me', true);
  }

  async updateNotifyProfile(patch: { invites?: InvitePreference; nearby?: boolean }): Promise<NotifyProfile> {
    return this.request('PATCH', '/notify/me', patch, true);
  }

  async getCircle(): Promise<CircleLists> {
    const data = await this.get<Partial<CircleLists>>('/notify/circle', true);
    return {
      members: data.members ?? [],
      requests: data.requests ?? [],
      notifiers: data.notifiers ?? [],
    };
  }

  async getCircleSuggestions(): Promise<CircleSuggestion[]> {
    const data = await this.get<{ players?: CircleSuggestion[] }>('/notify/circle/suggestions', true);
    return data.players ?? [];
  }

  /**
   * Adds somebody to this player's circle: by key for a past opponent, which
   * takes effect at once, or by username, which sends a request the other
   * side has to accept.
   */
  async addToCircle(who: { key: string } | { username: string }): Promise<CircleEntry> {
    const data = await this.post<{ entry: CircleEntry }>('/notify/circle', who, true);
    return data.entry;
  }

  async removeFromCircle(key: string): Promise<void> {
    await this.del(`/notify/circle/${encodeURIComponent(key)}`, true);
  }

  async muteNotifier(key: string, muted: boolean): Promise<void> {
    await this.post(`/notify/circle/${encodeURIComponent(key)}/mute`, { muted }, true);
  }

  async acceptCircleRequest(key: string): Promise<CircleEntry> {
    const data = await this.post<{ entry: CircleEntry }>(
      `/notify/circle/requests/${encodeURIComponent(key)}/accept`,
      null,
      true,
    );
    return data.entry;
  }

  async declineCircleRequest(key: string): Promise<void> {
    await this.post(`/notify/circle/requests/${encodeURIComponent(key)}/decline`, null, true);
  }

  /** Whose friend link this is. Public, so a signed-out visitor sees it too. */
  async previewFriendLink(code: string): Promise<FriendPreview> {
    return this.get(`/notify/friend/${encodeURIComponent(code)}`, false);
  }

  async acceptFriendLink(code: string): Promise<CircleEntry> {
    const data = await this.post<{ entry: CircleEntry }>(
      `/notify/friend/${encodeURIComponent(code)}`,
      null,
      true,
    );
    return data.entry;
  }

  /** Tells the host's circle about a table. Repeat calls reach only the
   *  people not told yet, so calling it again is safe. */
  async announceTable(matchId: string, keys?: string[]): Promise<{ notified: number; already?: boolean }> {
    return this.post('/notify/announce', keys ? { matchId, keys } : { matchId }, true);
  }

  async getNotifyConfig(): Promise<NotifyConfig> {
    return this.get('/notify/config', false);
  }

  async registerPushDevice(device: PushDeviceRegistration): Promise<{ id: string }> {
    return this.post('/notify/devices', device, true);
  }

  async unregisterPushDevice(id: string): Promise<void> {
    await this.del(`/notify/devices/${encodeURIComponent(id)}`, true);
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
    const res = await this.transport.fetch(
      `/scoring-sessions/${encodeURIComponent(id)}/export`,
      { method: 'GET' },
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

  private async del(path: string, auth: boolean): Promise<void> {
    await this.request('DELETE', path, undefined, auth);
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
    const sentWith = this.accessToken;
    if (auth && sentWith) {
      headers.Authorization = `Bearer ${sentWith}`;
    }
    const res = await this.transport.fetch(path, {
      method,
      headers,
      body: body != null ? JSON.stringify(body) : undefined,
    });
    if (res.status === 401 && auth && this.refreshToken && !retried) {
      // Someone else refreshed while this request was out: its token is
      // already stale, and the new one has not been tried yet.
      if (this.accessToken && this.accessToken !== sentWith) {
        return this.request<T>(method, path, body, auth, true);
      }
      try {
        await this.refreshTokens();
      } catch (e) {
        // Only the server refusing the refresh token means the session is
        // over. A dropped connection, a restart's 502 or a 503 says nothing
        // about the credentials, and signing the player out for one of those
        // throws away a perfectly good session in the middle of a game.
        if (!(e instanceof ApiError) || e.status !== 401) throw e;
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


export { socketBase } from '@/src/net/transport';
