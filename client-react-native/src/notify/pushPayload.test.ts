import { messageFromPush } from '@/src/notify/pushPayload';

const invite = {
  id: 'm1',
  matchId: 'm1',
  joinCode: 'ABC123',
  moduleId: 'canasta',
  host: { key: 'user:anna', name: 'Anna' },
  sentAt: '2026-09-21T10:00:00Z',
};

describe('push payloads', () => {
  it('reads a web push invite as the socket message it duplicates', () => {
    expect(
      messageFromPush({ type: 'table_invite', invite, title: 't', body: 'b', url: '/join/ABC123', tag: 'invite:m1' }),
    ).toEqual({ type: 'table_invite', invite });
  });

  it('fills the invite id from the match id when a payload leaves it out', () => {
    const msg = messageFromPush({ type: 'table_invite', invite: { ...invite, id: '' } });
    expect(msg).toMatchObject({ type: 'table_invite', invite: { id: 'm1' } });
  });

  it('reads a revoke by id, or by its tag', () => {
    expect(messageFromPush({ type: 'invite_revoked', id: 'm1', silent: true })).toEqual({
      type: 'invite_revoked',
      id: 'm1',
    });
    expect(messageFromPush({ type: 'invite_revoked', tag: 'invite:m2' })).toEqual({
      type: 'invite_revoked',
      id: 'm2',
    });
  });

  it('drops anything it does not recognise', () => {
    expect(messageFromPush(null)).toBeNull();
    expect(messageFromPush({ type: 'table_invite' })).toBeNull();
    expect(messageFromPush({ type: 'something_new' })).toBeNull();
  });
});
