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

/** The part of `r` inside `clip`, or null when they do not overlap. */
export function intersect(r: Rect, clip: Rect): Rect | null {
  const x = Math.max(r.x, clip.x);
  const y = Math.max(r.y, clip.y);
  const right = Math.min(r.x + r.width, clip.x + clip.width);
  const bottom = Math.min(r.y + r.height, clip.y + clip.height);
  if (right <= x || bottom <= y) return null;
  return { x, y, width: right - x, height: bottom - y };
}

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
  // Regions drawn outside the scrolling board — the hand while it is docked at
  // the top of the screen — which the board's own edges must not clip.
  const unclipped = useRef(new Set<string>());
  // The scrolling board's own box in the window. A meld scrolled out of it is
  // still somewhere in window coordinates — above the board's top edge, under
  // whatever is drawn there, which with the hand docked is the hand itself —
  // and must not take a card let go over that.
  const viewportNode = useRef<Measurable | null>(null);
  const viewportRect = useRef<Rect | null>(null);

  const register = useCallback((id: string, node: Measurable | null) => {
    unclipped.current.delete(id);
    if (node) nodes.current.set(id, node);
    else nodes.current.delete(id);
  }, []);

  /** `register`, for a region that is not inside the scrolling board. */
  const registerUnclipped = useCallback((id: string, node: Measurable | null) => {
    if (node) {
      nodes.current.set(id, node);
      unclipped.current.add(id);
    } else {
      nodes.current.delete(id);
      unclipped.current.delete(id);
    }
  }, []);

  /** The scrolling board, whose visible box clips every region inside it. */
  const setViewport = useCallback((node: Measurable | null) => {
    viewportNode.current = node;
    if (!node) viewportRect.current = null;
  }, []);

  const measurePass = useCallback(() => {
    nodes.current.forEach((node, id) => {
      node.measureInWindow((x, y, width, height) => {
        rects.current.set(id, { x, y, width, height });
      });
    });
    viewportNode.current?.measureInWindow((x, y, width, height) => {
      viewportRect.current = width > 0 && height > 0 ? { x, y, width, height } : null;
    });
  }, []);

  const measure = useCallback(() => {
    measurePass();
    // A panel that just became a live drop target opens regardless of
    // whether it was minimized (see `Panel`'s `forceOpen`), but that state
    // change and this measure both fire from the same gesture callback,
    // before React has committed and laid the panel back out — so the first
    // pass can catch it still collapsed. One more pass, deferred to the next
    // frame, catches the rect the first pass missed. Idempotent and cheap
    // enough to run unconditionally rather than trying to guess whether
    // anything actually opened.
    const raf =
      typeof requestAnimationFrame === 'function'
        ? requestAnimationFrame
        : (cb: () => void) => setTimeout(cb, 0);
    raf(measurePass);
  }, [measurePass]);

  const rectFor = useCallback((id: string) => rects.current.get(id), []);

  /** The scrolling board's visible box in the window, as of the last measure. */
  const viewport = useCallback(() => viewportRect.current, []);

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
    const clip = viewportRect.current;
    for (const id of among) {
      const whole = rects.current.get(id);
      if (!whole) continue;
      // Only the part of a region that can be seen can be aimed at — and the
      // slop around it stops at the board's edge too, so a card let go just
      // above the board is not taken by a meld hidden just inside it.
      const clipped = clip && !unclipped.current.has(id);
      const r = clipped ? intersect(whole, clip) : whole;
      if (!r) continue;
      if (clipped && (x < clip.x || x > clip.x + clip.width || y < clip.y || y > clip.y + clip.height)) continue;
      if (x < r.x - HIT_SLOP || x > r.x + r.width + HIT_SLOP) continue;
      if (y < r.y - HIT_SLOP || y > r.y + r.height + HIT_SLOP) continue;
      // The whole region's size, not its visible part's: "smallest wins" is
      // what makes a meld reachable inside the section around it, and a
      // section half scrolled out of view must not come out smaller than the
      // meld it holds.
      const area = whole.width * whole.height;
      if (area < bestArea) {
        bestArea = area;
        best = id;
      }
    }
    return best;
  }, []);

  return { register, registerUnclipped, setViewport, measure, rectFor, viewport, hit };
}
