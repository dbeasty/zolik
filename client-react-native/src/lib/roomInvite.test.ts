import { roomInviteUrl } from './roomInvite';

jest.mock('@/modules/zolik-nearby', () => ({ hostStatus: () => null, localAddresses: () => [] }));

describe('room invite links', () => {
  const here = () => ({ address: '192.168.1.23', port: 47800 });

  it('names the hosting phone by its address on the room network', () => {
    expect(roomInviteUrl({ role: 'host', via: 'self', baseUrl: 'http://127.0.0.1:5555' }, 'ABC123', false, here)).toBe(
      'http://192.168.1.23:47800/join/ABC123',
    );
  });

  it('offers nothing on a host whose room is closed', () => {
    expect(roomInviteUrl({ role: 'host', via: 'self', baseUrl: 'http://127.0.0.1:5555' }, 'ABC123', false, () => null)).toBe('');
  });

  it('passes on the address a Wi-Fi guest reached the table by', () => {
    expect(roomInviteUrl({ role: 'guest', via: 'wifi', baseUrl: 'http://192.168.1.23:47800/' }, 'ABC123', false, here)).toBe(
      'http://192.168.1.23:47800/join/ABC123',
    );
  });

  it('has nothing to offer over Bluetooth', () => {
    expect(roomInviteUrl({ role: 'guest', via: 'bluetooth', baseUrl: 'ble://x' }, 'ABC123', false, here)).toBe('');
  });

  it('needs a join code', () => {
    expect(roomInviteUrl({ role: 'host', via: 'self', baseUrl: '' }, '', false, here)).toBe('');
  });
});
