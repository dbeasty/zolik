import type { TableInvite } from '@/src/api/types';
import type { Invite } from '@/src/notify/types';

/**
 * The invite queue, as plain functions over plain data.
 *
 * Kept out of the provider so the rules that decide what a player is shown —
 * which copy of an invite wins, how long "Not now" lasts, when a nearby table
 * counts as gone — can be tested without mounting anything. The provider
 * holds one of these in state and calls nothing but the functions below.
 */

export type QueueState = {
  /** Newest first. */
  invites: Invite[];
  /** Invite id → the time (ms) until which it is not to be shown again. */
  suppressed: Record<string, number>;
};

export const EMPTY_QUEUE: QueueState = { invites: [], suppressed: {} };

/** "Not now" on a table in the room: long enough that the banner does not
 *  come straight back while the host is still dealing, short enough that
 *  the same host's next table is announced. */
export const NEARBY_DISMISS_MS = 10 * 60 * 1000;

/** "Not now" on an online table. The server tells each player about a table
 *  once, so this only has to outlast a late push copy of the same invite. */
export const ONLINE_DISMISS_MS = 12 * 60 * 60 * 1000;

/**
 * A Bluetooth table not seen for this long has gone. Bluetooth never says
 * goodbye — a scan only reports what it sees — and the watcher scans in
 * bursts every half minute, so three missed bursts is the signal.
 */
export const BLE_STALE_MS = 95 * 1000;

/** An online invite nobody revoked is not believed for ever: a server that
 *  restarted mid-table may never send the revoke. */
export const ONLINE_STALE_MS = 2 * 60 * 60 * 1000;

/** The id a nearby table is known by: its host's name, in any case. */
export function nearbyInviteId(hostName: string): string {
  return `nearby:${hostName.trim().toLowerCase()}`;
}

/** An online invite, from the socket's or a push's copy of it. */
export function inviteFromWire(w: TableInvite, now: number): Invite {
  return {
    id: w.id || w.matchId,
    source: 'online',
    host: { name: w.host?.name ?? '', avatar: w.host?.avatar, key: w.host?.key },
    moduleId: w.moduleId,
    moduleLabel: w.moduleLabel,
    variation: w.variation,
    target: { kind: 'online', matchId: w.matchId, joinCode: w.joinCode },
    receivedAt: now,
    seenAt: now,
  };
}

function withoutStaleSuppressions(s: Record<string, number>, now: number): Record<string, number> {
  const next: Record<string, number> = {};
  for (const [id, until] of Object.entries(s)) if (until > now) next[id] = until;
  return next;
}

/**
 * Adds an invite, or refreshes the one already queued under its id.
 *
 * A refresh keeps the original arrival time and whether the banner already
 * had its turn, so a table seen again on the next Bluetooth burst does not
 * pop the banner back up every thirty seconds. Wi-Fi outranks Bluetooth for
 * the same host: it joins faster and does not tie up the radio.
 */
export function receive(state: QueueState, invite: Invite, now: number): QueueState {
  const suppressed = withoutStaleSuppressions(state.suppressed, now);
  if (suppressed[invite.id]) return { ...state, suppressed };

  const at = state.invites.findIndex((i) => i.id === invite.id);
  if (at === -1) {
    const invites = [invite, ...state.invites].sort((a, b) => b.receivedAt - a.receivedAt);
    return { invites, suppressed };
  }

  const old = state.invites[at]!;
  // A scan reports the same table many times a second. Nothing about it has
  // changed, and a fresh state object for each report would re-render the
  // whole app at that rate.
  if (invite.target.kind === 'nearby' && old.source === invite.source && now - old.seenAt < 5_000) {
    return state;
  }
  let merged: Invite = { ...old, seenAt: now, host: { ...old.host, ...definedOnly(invite.host) } };
  merged.host.known = !!(old.host.known || invite.host.known);
  if (old.source === 'ble' && invite.source === 'wifi') {
    merged = { ...merged, source: 'wifi', target: invite.target };
  } else if (invite.source === 'waiting-room') {
    // The waiting room seated them: whatever the circle said about this
    // table, Join now only has to walk them to it.
    merged = { ...merged, source: 'waiting-room', target: invite.target };
  } else if (old.source === invite.source) {
    merged = { ...merged, target: invite.target };
  }
  if (invite.moduleId) merged.moduleId = invite.moduleId;
  if (invite.variation) merged.variation = invite.variation;

  const invites = state.invites.slice();
  invites[at] = merged;
  return { invites, suppressed };
}

/** The fields a copy actually carries: a blank name in one copy (the
 *  waiting room's, which knows none) must not erase the name in another. */
function definedOnly<T extends object>(o: T): Partial<T> {
  const out: Partial<T> = {};
  for (const [k, v] of Object.entries(o)) if (v !== undefined && v !== '') (out as Record<string, unknown>)[k] = v;
  return out;
}

/** The table started, filled or was deleted. It is not coming back, and a
 *  push copy that arrives late must not resurrect it. */
export function revoke(state: QueueState, id: string, now: number): QueueState {
  return {
    invites: state.invites.filter((i) => i.id !== id),
    suppressed: { ...withoutStaleSuppressions(state.suppressed, now), [id]: now + ONLINE_DISMISS_MS },
  };
}

/** "Not now": gone from every list, and kept away for a while. */
export function dismiss(state: QueueState, id: string, now: number): QueueState {
  const invite = state.invites.find((i) => i.id === id);
  const span = invite && invite.target.kind === 'nearby' ? NEARBY_DISMISS_MS : ONLINE_DISMISS_MS;
  return {
    invites: state.invites.filter((i) => i.id !== id),
    suppressed: { ...withoutStaleSuppressions(state.suppressed, now), [id]: now + span },
  };
}

/** Everything queued from one host, after the player muted them. */
export function dismissHost(state: QueueState, hostKey: string, now: number): QueueState {
  let next = state;
  for (const i of state.invites) if (i.host.key === hostKey) next = dismiss(next, i.id, now);
  return next;
}

/** Joined: the invite has done its job. Nothing is suppressed, so a later
 *  table from the same host still gets through. */
export function remove(state: QueueState, id: string): QueueState {
  return { ...state, invites: state.invites.filter((i) => i.id !== id) };
}

/** The banner has had its turn. */
export function shelve(state: QueueState, id: string): QueueState {
  if (!state.invites.some((i) => i.id === id && !i.shelved)) return state;
  return { ...state, invites: state.invites.map((i) => (i.id === id ? { ...i, shelved: true } : i)) };
}

/**
 * A Wi-Fi host stopped advertising. Only drops an invite that Wi-Fi is the
 * reason for: the same table still in Bluetooth range stays.
 */
export function lostOnWifi(state: QueueState, id: string): QueueState {
  if (!state.invites.some((i) => i.id === id && i.source === 'wifi')) return state;
  return { ...state, invites: state.invites.filter((i) => !(i.id === id && i.source === 'wifi')) };
}

/** Drops what has gone quiet. Returns the same object when nothing changed,
 *  so a timer calling it does not re-render the tree for nothing. */
export function expire(state: QueueState, now: number): QueueState {
  const invites = state.invites.filter((i) => {
    if (i.source === 'ble') return now - i.seenAt < BLE_STALE_MS;
    if (i.target.kind === 'online') return now - i.receivedAt < ONLINE_STALE_MS;
    return true;
  });
  return invites.length === state.invites.length ? state : { ...state, invites };
}

/** What the banner shows: the newest invite that has not had its turn. */
export function bannerInvite(state: QueueState): Invite | null {
  return state.invites.find((i) => !i.shelved) ?? null;
}
