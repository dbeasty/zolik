import type { TableInvite } from '@/src/api/types';
import {
  BLE_STALE_MS,
  EMPTY_QUEUE,
  NEARBY_DISMISS_MS,
  ONLINE_STALE_MS,
  bannerInvite,
  dismiss,
  dismissHost,
  expire,
  inviteFromWire,
  lostOnWifi,
  nearbyInviteId,
  receive,
  remove,
  revoke,
  shelve,
} from '@/src/notify/queue';
import type { Invite } from '@/src/notify/types';

const wire = (matchId: string, host = 'Anna', key = 'user:anna'): TableInvite => ({
  id: matchId,
  matchId,
  joinCode: `J${matchId}`,
  moduleId: 'canasta',
  host: { key, name: host },
  sentAt: '2026-09-21T10:00:00Z',
});

const nearby = (name: string, source: 'wifi' | 'ble', now: number): Invite => ({
  id: nearbyInviteId(name),
  source,
  host: { name },
  target:
    source === 'wifi'
      ? { kind: 'nearby', instanceId: 'inst-1', address: '192.168.1.5:47800' }
      : { kind: 'nearby', peripheralId: 'periph-1' },
  receivedAt: now,
  seenAt: now,
});

describe('invite queue', () => {
  it('keeps one invite per table when the socket and a push both deliver it', () => {
    let q = receive(EMPTY_QUEUE, inviteFromWire(wire('m1'), 1000), 1000);
    q = receive(q, inviteFromWire(wire('m1'), 2000), 2000);
    expect(q.invites).toHaveLength(1);
    // The first arrival's time stands, so the banner does not restart.
    expect(q.invites[0]!.receivedAt).toBe(1000);
  });

  it('orders newest first and shows the newest on the banner', () => {
    let q = receive(EMPTY_QUEUE, inviteFromWire(wire('m1'), 1000), 1000);
    q = receive(q, inviteFromWire(wire('m2', 'Bob', 'user:bob'), 2000), 2000);
    expect(q.invites.map((i) => i.id)).toEqual(['m2', 'm1']);
    expect(bannerInvite(q)?.id).toBe('m2');
  });

  it('merges a table seen over Wi-Fi and Bluetooth by host name, preferring Wi-Fi', () => {
    let q = receive(EMPTY_QUEUE, nearby('Anna', 'ble', 1000), 1000);
    q = receive(q, nearby('anna ', 'wifi', 2000), 2000);
    expect(q.invites).toHaveLength(1);
    expect(q.invites[0]!.source).toBe('wifi');
    expect(q.invites[0]!.target).toMatchObject({ address: '192.168.1.5:47800' });
    // A later Bluetooth sighting refreshes it without taking Wi-Fi's place.
    q = receive(q, nearby('Anna', 'ble', 3000), 3000);
    expect(q.invites[0]!.source).toBe('wifi');
    expect(q.invites[0]!.seenAt).toBe(3000);
  });

  it('does not bring a shelved invite back to the banner on a repeat sighting', () => {
    let q = receive(EMPTY_QUEUE, nearby('Anna', 'ble', 1000), 1000);
    q = shelve(q, nearbyInviteId('Anna'));
    q = receive(q, nearby('Anna', 'ble', 31_000), 31_000);
    expect(q.invites).toHaveLength(1);
    expect(bannerInvite(q)).toBeNull();
  });

  it('drops a revoked table and ignores a push copy that arrives after the revoke', () => {
    let q = receive(EMPTY_QUEUE, inviteFromWire(wire('m1'), 1000), 1000);
    q = revoke(q, 'm1', 2000);
    expect(q.invites).toHaveLength(0);
    q = receive(q, inviteFromWire(wire('m1'), 3000), 3000);
    expect(q.invites).toHaveLength(0);
  });

  it('keeps a dismissed nearby table away for ten minutes, then lets it back', () => {
    let q = receive(EMPTY_QUEUE, nearby('Anna', 'wifi', 0), 0);
    q = dismiss(q, nearbyInviteId('Anna'), 0);
    q = receive(q, nearby('Anna', 'wifi', NEARBY_DISMISS_MS - 1), NEARBY_DISMISS_MS - 1);
    expect(q.invites).toHaveLength(0);
    q = receive(q, nearby('Anna', 'wifi', NEARBY_DISMISS_MS + 1), NEARBY_DISMISS_MS + 1);
    expect(q.invites).toHaveLength(1);
  });

  it('removes every invite from a muted host and nobody else', () => {
    let q = receive(EMPTY_QUEUE, inviteFromWire(wire('m1'), 1), 1);
    q = receive(q, inviteFromWire(wire('m2'), 2), 2);
    q = receive(q, inviteFromWire(wire('m3', 'Bob', 'user:bob'), 3), 3);
    q = dismissHost(q, 'user:anna', 4);
    expect(q.invites.map((i) => i.id)).toEqual(['m3']);
  });

  it('forgets a joined invite without suppressing the next one', () => {
    let q = receive(EMPTY_QUEUE, inviteFromWire(wire('m1'), 1), 1);
    q = remove(q, 'm1');
    q = receive(q, inviteFromWire(wire('m1'), 2), 2);
    expect(q.invites).toHaveLength(1);
  });

  it('turns an online invite into a seated one when the waiting room seats the player', () => {
    let q = receive(EMPTY_QUEUE, inviteFromWire(wire('m1'), 1), 1);
    q = receive(
      q,
      {
        id: 'm1',
        source: 'waiting-room',
        host: { name: '' },
        target: { kind: 'online', matchId: 'm1', joinCode: 'Jm1', seated: true },
        receivedAt: 2,
        seenAt: 2,
      },
      2,
    );
    expect(q.invites[0]!.target).toMatchObject({ seated: true });
    // The host's name from the circle's copy survives the empty one.
    expect(q.invites[0]!.host.name).toBe('Anna');
  });

  it('drops a Wi-Fi table when it stops advertising, but not one still in Bluetooth range', () => {
    let q = receive(EMPTY_QUEUE, nearby('Anna', 'wifi', 1), 1);
    q = lostOnWifi(q, nearbyInviteId('Anna'));
    expect(q.invites).toHaveLength(0);
    q = receive(EMPTY_QUEUE, nearby('Bob', 'ble', 1), 1);
    q = lostOnWifi(q, nearbyInviteId('Bob'));
    expect(q.invites).toHaveLength(1);
  });

  it('expires Bluetooth tables that went quiet and stale online invites', () => {
    let q = receive(EMPTY_QUEUE, nearby('Anna', 'ble', 0), 0);
    q = receive(q, inviteFromWire(wire('m1'), 0), 0);
    q = receive(q, nearby('Cara', 'wifi', 0), 0);
    expect(expire(q, BLE_STALE_MS - 1)).toBe(q);
    const later = expire(q, BLE_STALE_MS + 1);
    expect(later.invites.map((i) => i.id).sort()).toEqual(['m1', nearbyInviteId('Cara')].sort());
    expect(expire(later, ONLINE_STALE_MS + 1).invites.map((i) => i.id)).toEqual([nearbyInviteId('Cara')]);
  });
});
