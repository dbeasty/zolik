import { NEARBY_LANDING, followInvite, pathFromPushUrl, planJoin } from '@/src/notify/routing';
import { subjectKeyForSeat, subjectKeyForSession } from '@/src/notify/subjectKey';
import type { Invite } from '@/src/notify/types';

const base = { host: { name: 'Anna' }, receivedAt: 0, seenAt: 0 };

const online: Invite = {
  ...base,
  id: 'm1',
  source: 'online',
  target: { kind: 'online', matchId: 'm1', joinCode: 'ABC123' },
};

describe('join routing', () => {
  it('takes a circle invite through the join link, which seats the player', () => {
    expect(planJoin(online)).toEqual({ kind: 'navigate', href: '/join/ABC123' });
  });

  it('walks an already-seated waiting-room invite straight to the table', () => {
    const seated: Invite = {
      ...online,
      source: 'waiting-room',
      target: { kind: 'online', matchId: 'm 1', joinCode: 'ABC123', seated: true },
    };
    expect(planJoin(seated)).toEqual({ kind: 'navigate', href: '/lobby/join?matchId=m%201' });
  });

  it('falls back to the match id when an invite carries no code', () => {
    expect(planJoin({ ...online, target: { kind: 'online', matchId: 'm1', joinCode: '' } })).toEqual({
      kind: 'navigate',
      href: '/join/m1',
    });
  });

  it('prefers Wi-Fi to Bluetooth for a table in the room', () => {
    const both: Invite = {
      ...base,
      id: 'nearby:anna',
      source: 'wifi',
      target: { kind: 'nearby', address: '10.0.0.2:47800', peripheralId: 'p1' },
    };
    expect(planJoin(both)).toEqual({ kind: 'wifi', address: '10.0.0.2:47800' });
    expect(planJoin({ ...both, target: { kind: 'nearby', peripheralId: 'p1' } })).toEqual({
      kind: 'ble',
      peripheralId: 'p1',
    });
  });

  it('joins a nearby table under the offline name, then opens the offline screen', async () => {
    const calls: string[] = [];
    await followInvite(
      { ...base, id: 'nearby:anna', source: 'ble', target: { kind: 'nearby', peripheralId: 'p1' } },
      {
        joinNearby: async () => {
          calls.push('wifi');
        },
        joinBluetooth: async (id, name) => {
          calls.push(`ble:${id}:${name}`);
        },
        nearbyName: async () => 'Bob',
        navigate: (href) => calls.push(`go:${href}`),
      },
    );
    expect(calls).toEqual(['ble:p1:Bob', `go:${NEARBY_LANDING}`]);
  });

  it('does not navigate when a nearby join fails', async () => {
    const navigate = jest.fn();
    await expect(
      followInvite(
        { ...base, id: 'nearby:anna', source: 'wifi', target: { kind: 'nearby', address: '10.0.0.2:1' } },
        {
          joinNearby: async () => {
            throw new Error('gone');
          },
          joinBluetooth: async () => {},
          nearbyName: async () => 'Bob',
          navigate,
        },
      ),
    ).rejects.toThrow('gone');
    expect(navigate).not.toHaveBeenCalled();
  });

  it('follows only in-app paths from a push url', () => {
    expect(pathFromPushUrl('https://jokerless.com/join/ABC123')).toBe('/join/ABC123');
    expect(pathFromPushUrl('/join/ABC123?x=1')).toBe('/join/ABC123?x=1');
    expect(pathFromPushUrl('//evil.example/x')).toBe('');
    expect(pathFromPushUrl('javascript:alert(1)')).toBe('');
    expect(pathFromPushUrl(undefined)).toBe('');
  });
});

describe('subject keys', () => {
  it('tells a guest seat from an account seat by the id the server issues', () => {
    expect(subjectKeyForSeat('0123456789abcdef0123456789abcdef')).toBe('guest:0123456789abcdef0123456789abcdef');
    expect(subjectKeyForSeat('64b7f0c2a1b2c3d4e5f60718')).toBe('user:64b7f0c2a1b2c3d4e5f60718');
  });

  it('keys a session by whether it is a guest', () => {
    expect(subjectKeyForSession({ userId: 'abc', isGuest: true })).toBe('guest:abc');
    expect(subjectKeyForSession({ userId: 'abc', isGuest: false })).toBe('user:abc');
  });
});
