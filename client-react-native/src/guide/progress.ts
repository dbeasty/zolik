import type { GuideLesson, GuideStep } from './types';

/**
 * Where a player is in a lesson, and what each thing they do does to it.
 *
 * All of it pure, because the rules for "which move counts" and "which move is
 * held back" are the part of the guide that can be wrong in ways nobody sees
 * until a lesson is stuck. The screen only feeds it events.
 */

export type Progress = {
  /** Index of the current step; equal to the step count once the lesson is over. */
  idx: number;
  /** Moves made towards the current act step. */
  done: number;
};

export const START: Progress = { idx: 0, done: 0 };

export function currentStep(lesson: GuideLesson, p: Progress): GuideStep | null {
  return lesson.steps[p.idx] ?? null;
}

export function isComplete(lesson: GuideLesson, p: Progress): boolean {
  return p.idx >= lesson.steps.length;
}

/** The player pressed Next on a `say` step, or Back on any. */
export function next(lesson: GuideLesson, p: Progress): Progress {
  const step = currentStep(lesson, p);
  if (!step || step.kind !== 'say') return p;
  return { idx: p.idx + 1, done: 0 };
}

export function back(lesson: GuideLesson, p: Progress): Progress {
  if (p.idx <= 0) return p;
  // Back to the nearest earlier step worth re-reading: an act step already
  // done cannot be redone, so Back lands on the `say` before it.
  let i = p.idx - 1;
  while (i > 0 && lesson.steps[i].kind === 'act') i--;
  return { idx: i, done: 0 };
}

/** An accepted move of the player's own. */
export function moved(lesson: GuideLesson, p: Progress, verb: string): Progress {
  const step = currentStep(lesson, p);
  if (!step || step.kind !== 'act' || !(step.verbs ?? []).includes(verb)) return p;
  const done = p.done + 1;
  if (done >= (step.count && step.count > 0 ? step.count : 1)) return { idx: p.idx + 1, done: 0 };
  return { idx: p.idx, done };
}

/** Moves that are never held back, however strict the step. */
const ALWAYS = new Set(['leave']);

export type Gate = { allow: true } | { allow: false; key: string };

/**
 * Whether a move the player is about to make may go through.
 *
 * A `say` step holds every move: the card is asking to be read, and a move
 * made over it would be counted against a step the player has not seen. A
 * strict `act` step holds everything but its own verbs. A loose one holds
 * nothing.
 */
export function gate(lesson: GuideLesson, p: Progress, verb: string): Gate {
  if (ALWAYS.has(verb)) return { allow: true };
  const step = currentStep(lesson, p);
  if (!step) return { allow: true };
  if (step.kind === 'say') return { allow: false, key: 'guide.common.readFirst' };
  if (step.strict && !(step.verbs ?? []).includes(verb)) {
    return { allow: false, key: step.wrongKey ?? 'guide.common.notYet' };
  }
  return { allow: true };
}
