import type { ActionOffer } from '@/src/api/matchTypes';
import { isMeldLift } from '@/src/lib/meldLift';

describe('isMeldLift', () => {
  const base: ActionOffer = { id: 'x', verb: 'x', enabled: true };

  it('is a pick off a meld', () => {
    expect(isMeldLift({ ...base, source: { zone: 'from_meld', meldId: 't0-8', minCards: 1 } })).toBe(true);
  });

  it('is never an undo, though Canasta points its undos at a meld', () => {
    const undoMeld: ActionOffer = {
      ...base,
      id: 'undo_lay_meld',
      verb: 'undo_lay_meld',
      undo: true,
      source: { zone: 'from_meld', meldId: 't0-8' },
      target: { zone: 'from_hand', ownerId: 'p0' },
    };
    expect(isMeldLift(undoMeld)).toBe(false);
  });

  it('is not a lift the module already named', () => {
    expect(isMeldLift({ ...base, source: { zone: 'from_meld', submit: ['AH'] } })).toBe(false);
  });
});
