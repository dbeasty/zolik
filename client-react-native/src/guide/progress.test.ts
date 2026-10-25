import { START, back, currentStep, gate, isComplete, moved, next } from './progress';
import type { GuideLesson } from './types';

const lesson: GuideLesson = {
  id: 'basics',
  titleKey: 't',
  seed: 1,
  bots: 1,
  steps: [
    { id: 'a', kind: 'say', textKey: 'a' },
    { id: 'b', kind: 'act', textKey: 'b', verbs: ['play_card', 'draw'] },
    { id: 'c', kind: 'act', textKey: 'c', verbs: ['play_card'], count: 2, strict: true, wrongKey: 'wrong' },
    { id: 'd', kind: 'say', textKey: 'd' },
  ],
};

describe('guide progress', () => {
  it('walks a lesson from the first card to the last', () => {
    let p = START;
    expect(currentStep(lesson, p)?.id).toBe('a');
    p = next(lesson, p);
    expect(currentStep(lesson, p)?.id).toBe('b');
    p = moved(lesson, p, 'draw');
    expect(currentStep(lesson, p)?.id).toBe('c');
    p = moved(lesson, p, 'play_card');
    expect(p).toEqual({ idx: 2, done: 1 });
    p = moved(lesson, p, 'play_card');
    expect(currentStep(lesson, p)?.id).toBe('d');
    p = next(lesson, p);
    expect(isComplete(lesson, p)).toBe(true);
  });

  it('ignores a move that is not the one the step waits for', () => {
    const p = { idx: 2, done: 0 };
    expect(moved(lesson, p, 'draw')).toBe(p);
  });

  it('does not let Next skip a step that waits for a move', () => {
    const p = { idx: 1, done: 0 };
    expect(next(lesson, p)).toBe(p);
  });

  it('holds every move while a card is waiting to be read', () => {
    expect(gate(lesson, START, 'draw')).toEqual({ allow: false, key: 'guide.common.readFirst' });
  });

  it('holds other moves on a strict step, with that step’s own words', () => {
    expect(gate(lesson, { idx: 2, done: 0 }, 'draw')).toEqual({ allow: false, key: 'wrong' });
    expect(gate(lesson, { idx: 2, done: 0 }, 'play_card')).toEqual({ allow: true });
  });

  it('lets a loose step through', () => {
    expect(gate(lesson, { idx: 1, done: 0 }, 'pass')).toEqual({ allow: true });
  });

  it('never holds back leaving the table', () => {
    expect(gate(lesson, START, 'leave')).toEqual({ allow: true });
    expect(gate(lesson, { idx: 2, done: 0 }, 'leave')).toEqual({ allow: true });
  });

  it('goes back to the card before, not into a move already made', () => {
    expect(currentStep(lesson, back(lesson, { idx: 3, done: 0 }))?.id).toBe('a');
    expect(back(lesson, START)).toBe(START);
  });
});
