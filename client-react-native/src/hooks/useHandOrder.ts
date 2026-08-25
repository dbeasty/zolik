import { useCallback, useEffect, useRef, useState } from 'react';

import type { Zone } from '@/src/api/matchTypes';
import { applySavedOrder, arrangeSlots, moveSlot, type Slot } from '@/src/lib/hand';
import { loadHandOrder, saveHandOrder } from '@/src/lib/handOrderStore';

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
 * It is also written to the device, so it survives a reload and coming back to
 * a match later — arranging a hand is only worth the trouble if it stays done.
 */
export function useHandOrder(zones: Zone[], matchId?: string) {
  const [order, setOrder] = useState<Record<string, Slot[]>>({});

  // Distinct ids for the lifetime of the screen. A counter rather than the
  // card string, because the whole point is telling two identical strings
  // apart.
  const minted = useRef(0);

  // What was stored for this match, and which zones have had it put back.
  //
  // Both are needed because the two things arrive in either order: the stored
  // arrangement is read from the device while the first hand is still on its
  // way over the socket. Applying it the moment it is read restores an order
  // onto a hand of no cards, and the first push then fills that empty hand in
  // the server's order — the arrangement lost precisely because it was
  // restored too eagerly. So it is kept until there are cards to put in it.
  const saved = useRef<Record<string, string[]> | null>(null);
  const restoredZones = useRef(new Set<string>());

  // What the server last sent, flattened. Comparing this rather than object
  // identity is what makes a *local* reorder survive: moving a card changes
  // `order` but not the hand the server sent, so it does not look like a push
  // and does not get reconciled away.
  const signature = zones
    .map((z) => `${z.id}:${(z.cards ?? []).map((c) => c.card).join(',')}`)
    .join('|');
  const [lastSignature, setLastSignature] = useState<string | null>(null);

  /** Reconciles one zone against a push, restoring a stored order if one is owed. */
  const reconcile = useCallback((zone: Zone, previous: Slot[]): Slot[] => {
    const slots = arrangeSlots(
      previous,
      (zone.cards ?? []).map((c) => c.card),
      (card) => `${zone.id}#${minted.current++}:${card}`,
    );
    const stored = saved.current?.[zone.id];
    if (!stored || restoredZones.current.has(zone.id) || slots.length === 0) return slots;
    // Once only: after this the player's live arrangement is the truth, and
    // reapplying what was on disk would undo every move they have made since.
    restoredZones.current.add(zone.id);
    return applySavedOrder(slots, stored);
  }, []);

  if (signature !== lastSignature) {
    // React's documented way to adjust state when props change: set it during
    // render, and it re-renders before committing anything to the screen. The
    // alternative — reconciling in an effect — would show one frame of the
    // server's order before correcting itself, which reads as a flicker in
    // the player's hand after every opponent move.
    setLastSignature(signature);
    setOrder((prev) => {
      const next: Record<string, Slot[]> = {};
      for (const zone of zones) next[zone.id] = reconcile(zone, prev[zone.id] ?? []);
      return next;
    });
  }

  // Nothing is written until what was already stored has been read, or the
  // first push would overwrite the arrangement being restored with the order
  // it is meant to replace.
  const [restored, setRestored] = useState(false);

  useEffect(() => {
    saved.current = null;
    restoredZones.current = new Set();
    setRestored(false);
    if (!matchId) {
      setRestored(true);
      return;
    }
    let live = true;
    loadHandOrder(matchId)
      .then((stored) => {
        if (!live) return;
        saved.current = stored;
        // The hand may already be here, in which case put it in order now;
        // if it is not, `reconcile` will do it when the cards arrive.
        if (stored) {
          setOrder((prev) => {
            const next = { ...prev };
            for (const [zoneId, cards] of Object.entries(stored)) {
              const slots = prev[zoneId];
              if (!slots?.length || restoredZones.current.has(zoneId)) continue;
              restoredZones.current.add(zoneId);
              next[zoneId] = applySavedOrder(slots, cards);
            }
            return next;
          });
        }
        setRestored(true);
      })
      .catch(() => {
        if (live) setRestored(true);
      });
    return () => {
      live = false;
    };
  }, [matchId]);

  // Only when it has actually changed. Reconciling a push rebuilds this object
  // every time anyone at the table moves, and almost none of those are a
  // rearrangement — writing on each would be a write a second for nothing.
  const written = useRef<string | null>(null);

  useEffect(() => {
    if (!restored || !matchId) return;
    const zonesOut: Record<string, string[]> = {};
    for (const [zoneId, slots] of Object.entries(order)) {
      if (slots.length) zonesOut[zoneId] = slots.map((s) => s.card);
    }
    if (!Object.keys(zonesOut).length) return;
    const payload = JSON.stringify(zonesOut);
    if (payload === written.current) return;
    written.current = payload;
    void saveHandOrder(matchId, zonesOut);
  }, [restored, matchId, order]);

  const slotsFor = useCallback((zoneId: string): Slot[] => order[zoneId] ?? [], [order]);

  const move = useCallback((zoneId: string, from: number, to: number) => {
    setOrder((prev) => ({ ...prev, [zoneId]: moveSlot(prev[zoneId] ?? [], from, to) }));
  }, []);

  return { slotsFor, move };
}
