import { useCallback, useEffect, useMemo, useRef, useState } from 'react';

import type { ZolikClient } from '@/src/api/client';
import type { MatchAction, MatchState } from '@/src/api/matchTypes';

import { forgetTable, loadProgress, markDone, saveProgress } from './guideStore';
import {
  START,
  back as backFrom,
  currentStep,
  gate as gateFor,
  isComplete,
  moved,
  next as nextFrom,
  type Progress,
} from './progress';
import type { GuideAnchor, GuideLesson, GuideStep } from './types';

/**
 * The guide at one table.
 *
 * The table is an ordinary match; this hook is everything that makes it a
 * lesson. It fetches the lesson the server dealt the table for, remembers how
 * far the player is, holds back the moves a step does not want, and counts the
 * ones it does — counting a move only once the table has answered it, so a
 * move the engine refused never advances a lesson.
 *
 * Returns null at an ordinary table, which is every table but a lesson's.
 */
export type Guide = {
  lesson: GuideLesson;
  /** The table's module, variation and options (JSON), for links that must name the same game. */
  moduleId: string;
  variation: string;
  options: string;
  step: GuideStep | null;
  index: number;
  total: number;
  complete: boolean;
  /** Whether the table is waiting on this player for the current step. */
  myTurn: boolean;
  /** Why the last move was held back, as a locale key; cleared by the next thing that happens. */
  held: string | null;
  anchored: (anchor: GuideAnchor) => boolean;
  /** The current step is about the piles or the table in the middle: ring it as one. */
  tableRing: boolean;
  next: () => void;
  back: () => void;
  /** Whether a move about to be made may go. False when the guide holds it back, and says why. */
  allow: (action: MatchAction) => boolean;
  /** A move has gone to the table; it counts once the table answers it. */
  sent: (action: MatchAction) => void;
  /** Forget this table's saved place and, optionally, the table itself. */
  finish: () => Promise<void>;
};

/** How long a sent move waits for the table's answer before it stops counting. */
const PENDING_MS = 6000;

export function useGuide(
  client: ZolikClient,
  state: MatchState | null,
  error: { code: string } | null,
): Guide | null {
  const lessonId = state?.tutorial?.lessonId;
  const matchId = state?.matchId;
  const moduleId = state?.moduleId;
  const variation = state?.variation;
  const optionsKey = JSON.stringify(state?.options ?? {});

  const [lesson, setLesson] = useState<GuideLesson | null>(null);
  const [progress, setProgress] = useState<Progress | null>(null);
  const [held, setHeld] = useState<string | null>(null);
  const pending = useRef<{ verb: string; at: number } | null>(null);

  useEffect(() => {
    if (!lessonId || !moduleId) return;
    let live = true;
    void client
      .moduleLessons(moduleId, variation, JSON.parse(optionsKey) as Record<string, number>)
      .then((all) => {
        if (live) setLesson(all.find((l) => l.id === lessonId) ?? null);
      })
      .catch(() => {
        // No lesson to show is no guide, not a broken table.
        if (live) setLesson(null);
      });
    return () => {
      live = false;
    };
  }, [client, lessonId, moduleId, variation, optionsKey]);

  useEffect(() => {
    if (!lessonId || !matchId) return;
    let live = true;
    void loadProgress(matchId).then((p) => {
      if (live) setProgress(p);
    });
    return () => {
      live = false;
    };
  }, [lessonId, matchId]);

  const advance = useCallback(
    (to: (p: Progress) => Progress) => {
      setProgress((p) => {
        if (!p || !lesson) return p;
        const n = to(p);
        if (n !== p && matchId) void saveProgress(matchId, n);
        return n;
      });
    },
    [lesson, matchId],
  );

  // A sent move counts when the table next speaks, unless what it says is no.
  useEffect(() => {
    if (!error) return;
    pending.current = null;
  }, [error]);
  useEffect(() => {
    const sent = pending.current;
    if (!sent) return;
    pending.current = null;
    if (Date.now() - sent.at > PENDING_MS) return;
    setHeld(null);
    advance((p) => (lesson ? moved(lesson, p, sent.verb) : p));
    // Only the table's answer matters; `advance` and `lesson` are read fresh.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [state]);

  const complete = !!lesson && !!progress && isComplete(lesson, progress);
  useEffect(() => {
    if (complete && lesson && moduleId) void markDone(moduleId, lesson.id);
  }, [complete, lesson, moduleId]);

  const allow = useCallback(
    (action: MatchAction) => {
      if (!lesson || !progress) return true;
      const g = gateFor(lesson, progress, action.verb);
      if (!g.allow) {
        setHeld(g.key);
        return false;
      }
      setHeld(null);
      return true;
    },
    [lesson, progress],
  );
  const sent = useCallback((action: MatchAction) => {
    pending.current = { verb: action.verb, at: Date.now() };
  }, []);

  const next = useCallback(() => {
    setHeld(null);
    advance((p) => (lesson ? nextFrom(lesson, p) : p));
  }, [advance, lesson]);
  const back = useCallback(() => {
    setHeld(null);
    advance((p) => (lesson ? backFrom(lesson, p) : p));
  }, [advance, lesson]);

  const finish = useCallback(async () => {
    if (moduleId && lesson) await forgetTable(moduleId, lesson.id);
  }, [moduleId, lesson]);

  const step = lesson && progress ? currentStep(lesson, progress) : null;
  const myTurn = !!state?.legalActions?.some((o) => o.enabled && !o.manual);

  return useMemo(() => {
    if (!lessonId || !lesson || !progress) return null;
    const p = progress ?? START;
    const anchors = new Set<GuideAnchor>(step?.anchors ?? []);
    return {
      lesson,
      moduleId: moduleId ?? '',
      variation: variation ?? '',
      options: optionsKey,
      step,
      index: Math.min(p.idx, lesson.steps.length),
      total: lesson.steps.length,
      complete,
      myTurn,
      held,
      // A step that waits for a move only rings its anchors on the player's
      // own turn: ringing the hand while a bot is thinking points at nothing.
      anchored: (a: GuideAnchor) => anchors.has(a) && (step?.kind !== 'act' || myTurn),
      tableRing:
        (anchors.has('table') || anchors.has('discard') || anchors.has('drawPile')) &&
        (step?.kind !== 'act' || myTurn),
      next,
      back,
      allow,
      sent,
      finish,
    };
  }, [lessonId, lesson, progress, step, complete, myTurn, held, next, back, allow, sent, finish, moduleId, variation, optionsKey]);
}
