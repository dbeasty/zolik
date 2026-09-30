import type { Zone } from '@/src/api/matchTypes';
import { diffGroups, marksIn, nextMarks, NO_MARKS } from '@/src/lib/changes';

const spread = (ownerId: string, groups: Record<string, string[]>): Zone =>
  ({
    id: `spread-${ownerId}`,
    kind: 'spread',
    ownerId,
    count: 0,
    groups: Object.entries(groups).map(([id, cards]) => ({ id, kind: 'group', cards })),
  }) as unknown as Zone;

const board = (active: string, groups: Record<string, string[]>) => ({
  zones: [spread('eva', groups)],
  seats: [
    { playerId: 'me', active: active === 'me' },
    { playerId: 'eva', active: active === 'eva' },
  ],
});

describe('diffGroups', () => {
  it('sees a group laid down', () => {
    const d = diffGroups(board('eva', {}), board('eva', { m1: ['7H', '8H', '9H'] }));
    expect(d.get('m1')).toEqual({ fresh: true, added: ['7H', '8H', '9H'], reshaped: false });
  });

  it('sees one card added, even a second copy of a card already there', () => {
    const d = diffGroups(board('eva', { m1: ['7H', '7H'] }), board('eva', { m1: ['7H', '7H', '7H'] }));
    expect(d.get('m1')).toEqual({ fresh: false, added: ['7H'], reshaped: false });
  });

  it('sees a card swapped out', () => {
    const d = diffGroups(board('eva', { m1: ['7H', 'X1', '9H'] }), board('eva', { m1: ['7H', '8H', '9H'] }));
    expect(d.get('m1')).toEqual({ fresh: false, added: ['8H'], reshaped: true });
  });

  it('says nothing about a group that did not change', () => {
    expect(diffGroups(board('eva', { m1: ['7H'] }), board('me', { m1: ['7H'] })).size).toBe(0);
  });
});

describe('nextMarks', () => {
  it('marks what somebody else did, and keeps adding to it', () => {
    let marks = nextMarks(NO_MARKS, board('eva', {}), board('eva', { m1: ['7H', '8H', '9H'] }), 'me');
    marks = nextMarks(marks, board('eva', { m1: ['7H', '8H', '9H'] }), board('eva', { m1: ['7H', '8H', '9H', 'TH'] }), 'me');
    expect(marks.get('m1')).toEqual({ fresh: true, added: ['7H', '8H', '9H', 'TH'], reshaped: false });
  });

  it("keeps the marks when the viewer's turn comes round, until they move", () => {
    const marked = nextMarks(NO_MARKS, board('eva', {}), board('me', { m1: ['7H', '8H', '9H'] }), 'me');
    expect(marked.get('m1')?.added).toEqual(['7H', '8H', '9H']);
    // A board on the viewer's turn with nothing moved is not their move.
    const same = nextMarks(marked, board('me', { m1: ['7H', '8H', '9H'] }), board('me', { m1: ['7H', '8H', '9H'] }), 'me');
    expect(same).toBe(marked);
  });

  it("clears on the viewer's first move, and never marks their own moves", () => {
    const marked = nextMarks(NO_MARKS, board('eva', {}), board('me', { m1: ['7H', '8H', '9H'] }), 'me');
    const after = nextMarks(marked, board('me', { m1: ['7H', '8H', '9H'] }), board('me', { m1: ['7H', '8H', '9H', 'TH'] }), 'me');
    expect(after.size).toBe(0);
    const later = nextMarks(after, board('me', { m1: ['7H', '8H', '9H', 'TH'] }), board('eva', { m1: ['7H', '8H', '9H', 'TH'], m2: ['2S', '2C', '2D'] }), 'me');
    expect(later.size).toBe(0);
  });

  it("marks a partner's changes as well as an opponent's", () => {
    const table = (active: string, groups: Record<string, string[]>) => ({
      zones: [spread('pat', groups)],
      seats: [
        { playerId: 'me', side: 'a', active: active === 'me' },
        { playerId: 'eva', side: 'b', active: active === 'eva' },
        { playerId: 'pat', side: 'a', active: active === 'pat' },
      ],
    });
    const byPartner = nextMarks(NO_MARKS, table('pat', {}), table('eva', { m1: ['7H', '8H', '9H'] }), 'me');
    expect([...byPartner.keys()]).toEqual(['m1']);
    const byOpponent = nextMarks(byPartner, table('eva', { m1: ['7H', '8H', '9H'] }), table('pat', { m1: ['7H', '8H', '9H'], m2: ['KS', 'KC', 'KD'] }), 'me');
    expect([...byOpponent.keys()]).toEqual(['m1', 'm2']);
  });

  it('forgets a group that left the board', () => {
    const marked = nextMarks(NO_MARKS, board('eva', {}), board('eva', { m1: ['7H'] }), 'me');
    expect(nextMarks(marked, board('eva', { m1: ['7H'] }), board('eva', {}), 'me').size).toBe(0);
  });

  it('marks nothing on the first board it sees', () => {
    expect(nextMarks(NO_MARKS, null, board('eva', { m1: ['7H'] }), 'me').size).toBe(0);
  });

  it('counts the marked groups in a zone', () => {
    const b = board('eva', { m1: ['7H'], m2: ['8S'] });
    const marks = nextMarks(NO_MARKS, board('eva', { m2: ['8S'] }), b, 'me');
    expect(marksIn(b.zones[0], marks)).toBe(1);
  });
});
