import type { Invite } from '@/src/notify/types';

/**
 * Where tapping Join takes a player, decided apart from doing it.
 *
 * The decision is the part worth testing, and it has one subtlety: an online
 * invite from the circle has *not* seated anybody, so it goes through
 * `/join/<code>`, which takes the seat, sends the host to their own table and
 * words a stale invite (full, already dealt, gone) on its existing error
 * screen. Only the waiting room seats a player before telling them, and only
 * that invite goes straight to `/lobby/join?matchId=`, which watches a seat
 * already held and never takes one.
 */
export type JoinPlan =
  | { kind: 'navigate'; href: string }
  | { kind: 'wifi'; address: string }
  | { kind: 'ble'; peripheralId: string }
  | { kind: 'none' };

export function planJoin(invite: Invite): JoinPlan {
  const target = invite.target;
  if (target.kind === 'online') {
    if (target.seated) {
      return { kind: 'navigate', href: `/lobby/join?matchId=${encodeURIComponent(target.matchId)}` };
    }
    // The join route takes a match id as readily as a code, so an invite that
    // somehow arrived without a code still leads somewhere.
    const code = target.joinCode || target.matchId;
    if (!code) return { kind: 'none' };
    return { kind: 'navigate', href: `/join/${encodeURIComponent(code)}` };
  }
  // Wi-Fi first: it joins faster, and it leaves the radio alone.
  if (target.address) return { kind: 'wifi', address: target.address };
  if (target.peripheralId) return { kind: 'ble', peripheralId: target.peripheralId };
  return { kind: 'none' };
}

/** Where a seated nearby guest lands: the screen that knows about the table
 *  in the room and offers its games. */
export const NEARBY_LANDING = '/offline';

export type JoinDeps = {
  joinNearby: (address: string, name: string) => Promise<void>;
  joinBluetooth: (peripheralId: string, name: string) => Promise<void>;
  /** The name to sit down under at a table in the room. */
  nearbyName: () => Promise<string>;
  navigate: (href: string) => void;
};

/**
 * Carries out a plan. Throws what the join threw, so the caller can put the
 * reason in front of the player — a nearby join has no error screen of its
 * own to land on.
 */
export async function followInvite(invite: Invite, deps: JoinDeps): Promise<void> {
  const plan = planJoin(invite);
  switch (plan.kind) {
    case 'navigate':
      deps.navigate(plan.href);
      return;
    case 'wifi':
      await deps.joinNearby(plan.address, await deps.nearbyName());
      deps.navigate(NEARBY_LANDING);
      return;
    case 'ble':
      await deps.joinBluetooth(plan.peripheralId, await deps.nearbyName());
      deps.navigate(NEARBY_LANDING);
      return;
    case 'none':
      return;
  }
}

/**
 * The in-app route behind a push's `url`, or '' when it names none.
 *
 * A push carries an absolute link, because a browser opening it from a
 * notification needs one. Tapped inside the app only its path matters, and
 * only a path rooted at `/` is followed: the payload came over the network,
 * and "navigate wherever this string says" is not worth trusting it with.
 */
export function pathFromPushUrl(url: unknown): string {
  if (typeof url !== 'string' || !url) return '';
  const stripped = url.replace(/^[a-z][a-z0-9+.-]*:\/\/[^/?#]*/i, '');
  if (!stripped.startsWith('/') || stripped.startsWith('//')) return '';
  return stripped;
}
