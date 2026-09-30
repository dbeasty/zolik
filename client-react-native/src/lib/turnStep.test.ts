import type { ActionOffer } from '@/src/api/matchTypes';
import { turnStep } from '@/src/lib/turnStep';

const offer = (id: string, extra: Partial<ActionOffer> = {}): ActionOffer => ({ id, verb: id, enabled: true, ...extra });

describe('turnStep', () => {
  it('is nothing when nothing is on offer', () => {
    expect(turnStep([offer('a', { enabled: false })])).toBeNull();
  });

  it('lists the moves on offer once each, leaving out undos', () => {
    const step = turnStep([
      offer('a'),
      offer('b1', { labelKey: 'x' }),
      offer('b2', { labelKey: 'x' }),
      offer('u', { undo: true }),
    ]);
    expect(step?.moves.map((o) => o.id)).toEqual(['a', 'b1']);
    expect(step?.obligation).toBeUndefined();
  });

  it('puts an obligation first: a refusal whose way out is an undo', () => {
    const remedy = { labelKey: 'finish.it' };
    const step = turnStep([
      offer('a'),
      offer('end', { enabled: false, remedy, remedyOfferId: 'u' }),
      offer('u', { undo: true }),
    ]);
    expect(step?.obligation).toBe(remedy);
  });

  it('does not treat an ordinary remedy as an obligation', () => {
    const step = turnStep([
      offer('a'),
      offer('b', { enabled: false, remedy: { labelKey: 'do.a.first' }, remedyOfferId: 'a' }),
    ]);
    expect(step?.obligation).toBeUndefined();
  });
});
