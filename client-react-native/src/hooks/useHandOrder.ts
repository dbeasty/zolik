import { useCallback, useRef, useState } from 'react';

import type { Zone } from '@/src/api/matchTypes';
import { arrangeSlots, moveSlot, type Slot } from '@/src/lib/hand';

/**
 * Keeps each hand in the order its owner put it in.
 *
 * The problem this exists for is that the server re-pushes the entire board
 * after every move by anyone at the table, and it sends a hand in whatever
 * order the module keeps it — which is not the order the player arranged it
 * in. Rendering straight from that push would shuffle someone's hand under
 * their hands every time an opponent discarded.
 *
 * So arrangement is held here, on the client, and reconciled against each
 * push. That is the right side of the wire for it: how a player likes their
 * cards laid out is not a fact about the game, changes nothing about which of
 * them are playable, and a submission still travels as card strings. No module
 * knows this feature exists, which is why every module gets it.
 *
 * The order survives an opponent's move and a reconnect within the session. It
 * does not survive a full page reload, which would need somewhere durable to
 * put it.
 */
export function useHandOrder(zones: Zone[]) {
  const [order, setOrder] = useState<Record<string, Slot[]>>({});

  // Distinct ids for the lifetime of the screen. A counter rather than the
  // card string, because the whole point is telling two identical strings
  // apart.
  const minted = useRef(0);

  // What the server last sent, flattened. Comparing this rather than object
  // identity is what makes a *local* reorder survive: moving a card changes
  // `order` but not the hand the server sent, so it does not look like a push
  // and does not get reconciled away.
  const signature = zones
    .map((z) => `${z.id}:${(z.cards ?? []).map((c) => c.card).join(',')}`)
    .join('|');
  const [lastSignature, setLastSignature] = useState<string | null>(null);

  if (signature !== lastSignature) {
    // React's documented way to adjust state when props change: set it during
    // render, and it re-renders before committing anything to the screen. The
    // alternative — reconciling in an effect — would show one frame of the
    // server's order before correcting itself, which reads as a flicker in
    // the player's hand after every opponent move.
    setLastSignature(signature);
    setOrder((prev) => {
      const next: Record<string, Slot[]> = {};
      for (const zone of zones) {
        next[zone.id] = arrangeSlots(
          prev[zone.id] ?? [],
          (zone.cards ?? []).map((c) => c.card),
          (card) => `${zone.id}#${minted.current++}:${card}`,
        );
      }
      return next;
    });
  }

  const slotsFor = useCallback(
    (zoneId: string): Slot[] => order[zoneId] ?? [],
    [order],
  );

  const move = useCallback((zoneId: string, from: number, to: number) => {
    setOrder((prev) => ({ ...prev, [zoneId]: moveSlot(prev[zoneId] ?? [], from, to) }));
  }, []);

  return { slotsFor, move };
}
