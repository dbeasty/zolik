import { useCallback, useRef } from 'react';

import type { Rect } from '@/src/lib/hand';

export type Measurable = {
  measureInWindow: (cb: (x: number, y: number, width: number, height: number) => void) => void;
};

/**
 * A little further than the eye: a drop that visually grazes the edge of a
 * meld was meant for that meld, and refusing it because the pointer was three
 * pixels outside reads as the drag having failed. The same 24px the screen
 * this replaces used, kept because it was arrived at by using the thing.
 */
const HIT_SLOP = 24;

/**
 * Keeps track of which parts of the board can be dropped on, and where they
 * are.
 *
 * Anything drawn on the board may register itself — zones and the meld groups
 * inside them — under the same id the drop logic uses to name it. Nothing here
 * knows why a region is interesting, or which of them a given drag may land
 * on; it answers "what is under this pointer, of these candidates?" and the
 * offers decide the rest.
 *
 * Positions are read in window coordinates, the space a gesture reports its
 * pointer in, and re-read at the start of every drag rather than cached — the
 * board is inside a scroll view and a meld that has scrolled since the last
 * drag is not where it was.
 */
export function useDropRegistry() {
  const nodes = useRef(new Map<string, Measurable>());
  const rects = useRef(new Map<string, Rect>());

  const register = useCallback((id: string, node: Measurable | null) => {
    if (node) nodes.current.set(id, node);
    else nodes.current.delete(id);
  }, []);

  const measure = useCallback(() => {
    nodes.current.forEach((node, id) => {
      node.measureInWindow((x, y, width, height) => {
        rects.current.set(id, { x, y, width, height });
      });
    });
  }, []);

  const rectFor = useCallback((id: string) => rects.current.get(id), []);

  /**
   * Which candidate region a pointer is over.
   *
   * Smallest wins, which is what makes a meld inside a spread reachable: the
   * pointer is inside both, and the one the player is aiming at is the one
   * they can see the edges of.
   */
  const hit = useCallback((x: number, y: number, among: readonly string[]): string | null => {
    let best: string | null = null;
    let bestArea = Infinity;
    for (const id of among) {
      const r = rects.current.get(id);
      if (!r) continue;
      if (x < r.x - HIT_SLOP || x > r.x + r.width + HIT_SLOP) continue;
      if (y < r.y - HIT_SLOP || y > r.y + r.height + HIT_SLOP) continue;
      const area = r.width * r.height;
      if (area < bestArea) {
        bestArea = area;
        best = id;
      }
    }
    return best;
  }, []);

  return { register, measure, rectFor, hit };
}
