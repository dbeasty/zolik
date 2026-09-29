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

  it("keeps the marks through the viewer's own turn, and never marks their own moves", () => {
    const marked = nextMarks(NO_MARKS, board('eva', {}), board('me', { m1: ['7H', '8H', '9H'] }), 'me');
    const after = nextMarks(marked, board('me', { m1: ['7H', '8H', '9H'] }), board('me', { m1: ['7H', '8H', '9H', 'TH'] }), 'me');
    expect(after.get('m1')?.added).toEqual(['7H', '8H', '9H']);
  });

  it("clears once the viewer's turn is over", () => {
    const marked = nextMarks(NO_MARKS, board('eva', {}), board('me', { m1: ['7H', '8H', '9H'] }), 'me');
    expect(nextMarks(marked, board('me', { m1: ['7H', '8H', '9H'] }), board('eva', { m1: ['7H', '8H', '9H'] }), 'me').size).toBe(0);
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
