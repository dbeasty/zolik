import type { MoveLine } from '@/src/api/matchTypes';
import { reactionsFor, reactionsForEnding } from '@/src/components/avatars/reactions';

/** Faces move on exactly the events the board sounds for — no more, no fewer. */

const line = (playerId: string, labelKey: string): MoveLine => ({ playerId, fact: { labelKey } });

describe('reactionsFor', () => {
  it('nods the player whose card made the sound', () => {
    expect(reactionsFor([line('a', 'prsi.move.played')], 'card')).toEqual({ a: 'nod' });
  });

  it('has the player who took the pile grab it', () => {
    expect(reactionsFor([line('b', 'zoliky.move.tookDiscard')], 'take')).toEqual({ b: 'grab' });
  });

  it('moves only the newest actor of a burst, as one burst makes one sound', () => {
    expect(reactionsFor([line('a', 'x.played'), line('b', 'x.played')], 'card')).toEqual({ b: 'nod' });
  });

  it('stays still when the board made no sound', () => {
    // A deal or a fresh round window: lots of new lines, no card sound.
    expect(reactionsFor([line('a', 'x.played'), line('b', 'x.played')], null)).toEqual({});
  });

  it('turns a called-out play into a call, even without a card sound', () => {
    expect(reactionsFor([line('c', 'lastcard.move.called')], null)).toEqual({ c: 'call' });
  });

  it('lets the call win over the card sound for the same player', () => {
    expect(reactionsFor([line('a', 'lastcard.move.playedSkip')], 'card')).toEqual({ a: 'call' });
  });

  it('calls only the newest called-out play, the one the announcer speaks', () => {
    const out = reactionsFor([line('a', 'lastcard.move.playedSkip'), line('b', 'lastcard.move.playedReverse')], 'card');
    expect(out).toEqual({ b: 'call' });
  });
});

describe('reactionsForEnding', () => {
  it('cheers the winners and slumps the rest', () => {
    expect(reactionsForEnding(['a', 'b', 'c'], ['b'])).toEqual({ a: 'slump', b: 'cheer', c: 'slump' });
  });

  it('cheers everyone at a match nobody won', () => {
    expect(reactionsForEnding(['a', 'b'], [])).toEqual({ a: 'cheer', b: 'cheer' });
  });
});
