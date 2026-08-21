import { router, useLocalSearchParams } from 'expo-router';
import { useCallback, useEffect, useRef, useState } from 'react';
import { Dimensions, Modal, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';
import Animated, { useAnimatedStyle, useSharedValue } from 'react-native-reanimated';

import { ActionBar } from '@/src/components/ActionBar';
import { CardView } from '@/src/components/CardView';
import { DeckDragOverlay } from '@/src/components/DeckDragOverlay';
import { DeckPile } from '@/src/components/DeckPile';
import {
  HandRow,
  closestZone,
  pointInZone,
  useDragPreview,
  type DropZone,
  type MeldHoverTarget,
  type MeldZone,
} from '@/src/components/HandRow';
import { MeldStagingArea, type StagingGroup } from '@/src/components/MeldStagingArea';
import { MeldTable } from '@/src/components/MeldTable';
import { Screen } from '@/src/components/Screen';
import { useGameFlow } from '@/src/context/GameFlowContext';
import { storage } from '@/src/context/SessionContext';
import { useSession } from '@/src/context/SessionContext';
import { useDropPulseStyle } from '@/src/hooks/useDropPulse';
import { useGameSocket } from '@/src/hooks/useGameSocket';
import type { GameState, WSEnvelope } from '@/src/api/types';
import {
  autoOrganizeHand,
  dealHeaderLabel,
  isViableMeld,
  moveCardToIndex,
  profileDisplayName,
  rulesSummaryLines,
} from '@/src/lib/cards';
import { colors, shared } from '@/src/theme';

const pileStyles = StyleSheet.create({
  discardWrap: {
    position: 'relative',
    borderRadius: 8,
    padding: 3,
    borderWidth: 2,
    borderColor: 'transparent',
  },
  discardWrapFlash: {
    borderColor: colors.success,
    backgroundColor: 'rgba(74, 222, 128, 0.15)',
  },
  discardFlashLabel: {
    position: 'absolute',
    bottom: -18,
    left: 0,
    right: 0,
    textAlign: 'center',
    color: colors.success,
    fontSize: 11,
    fontWeight: '700',
  },
  // Permanent bordered box around the table melds + staging area, always
  // rendered at a fixed spot near the top — mirrors the deck/discard boxes
  // below so nothing in the "board" portion of the screen appears,
  // disappears, or resizes the surrounding layout as the game state
  // changes.
  meldBox: {
    borderWidth: 2,
    borderColor: colors.border,
    borderRadius: 10,
    padding: 8,
    marginTop: 6,
  },
  meldBoxLabel: {
    color: colors.muted,
    fontSize: 11,
    fontWeight: '600',
    marginBottom: 4,
  },
  meldBoxEmpty: {
    color: colors.muted,
    fontSize: 12,
    textAlign: 'center',
    paddingVertical: 6,
  },
  // Deck and discard pile now sit in their own separate bordered rectangles
  // side by side, rather than sharing one loose row.
  pilesRow: {
    flexDirection: 'row',
    gap: 10,
    marginTop: 8,
  },
  pileBox: {
    borderWidth: 2,
    borderColor: colors.border,
    borderRadius: 10,
    padding: 10,
    paddingBottom: 22,
    alignItems: 'center',
  },
  pileBoxLabel: {
    color: colors.muted,
    fontSize: 11,
    fontWeight: '600',
    marginBottom: 6,
    textAlign: 'center',
  },
  handInfoBar: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginTop: 12,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
    paddingVertical: 6,
    paddingHorizontal: 10,
  },
  handInfoToggle: {
    color: colors.accent,
    fontSize: 12,
    fontWeight: '600',
  },
  // Same box shape/height whether or not there's an error, so filling in
  // or clearing the status text never changes the row's footprint — only
  // the border/background swap for the error state.
  statusRow: {
    flexDirection: 'row',
    alignItems: 'flex-start',
    gap: 6,
    borderWidth: 1,
    borderColor: 'transparent',
    backgroundColor: 'transparent',
    borderRadius: 8,
    paddingVertical: 8,
    paddingHorizontal: 10,
    marginTop: 8,
  },
  statusRowError: {
    backgroundColor: 'rgba(248, 113, 113, 0.12)',
    borderColor: colors.danger,
  },
  statusRowText: {
    color: colors.muted,
    fontSize: 13,
    lineHeight: 18,
    flex: 1,
  },
});

function selectedCards(hand: string[], selected: Set<number>): string[] {
  return hand.filter((_, i) => selected.has(i));
}

// Groups are tracked by card *value*, not hand index — a lay_meld on one
// group changes hand indices out from under any other still-staged group,
// so indices can't be stored directly. This resolves each group's card
// values back to concrete (unclaimed) hand indices fresh every render, one
// physical card per requested value, first group first — self-healing if
// the hand changes underneath (a resolved card that's no longer in hand
// just silently drops out of its group).
function resolveGroupIndices(hand: string[], groups: string[][]): number[][] {
  const claimed = new Array(hand.length).fill(false);
  return groups.map((group) => {
    const indices: number[] = [];
    for (const card of group) {
      const idx = hand.findIndex((c, i) => c === card && !claimed[i]);
      if (idx !== -1) {
        claimed[idx] = true;
        indices.push(idx);
      }
    }
    return indices;
  });
}

// Keeps the player's custom card order stable across draws/discards/
// reconnects: cards still in hand keep their relative position, cards no
// longer in hand (discarded/melded) drop out, and newly received cards are
// placed at the front (left) of the hand.
function reconcileHandOrder(customOrder: string[] | null, serverHand: string[]): string[] | null {
  if (!customOrder) return null;
  const remaining = [...serverHand];
  const kept: string[] = [];
  for (const c of customOrder) {
    const idx = remaining.indexOf(c);
    if (idx >= 0) {
      kept.push(c);
      remaining.splice(idx, 1);
    }
  }
  return [...remaining, ...kept];
}

// Maps a displayed hand order back onto the server's own `myHand` array
// (which two decks in play can hold duplicate values in — e.g. two "9S"s),
// so an action naming a specific displayed card can also say *which* of two
// same-value server slots it means. Mirrors reconcileHandOrder's own
// first-unclaimed-match greedy algorithm so the mapping stays consistent
// with whatever's actually on screen; `displayOrder` is normally `hand`
// itself (already reconciled), which this happens to be idempotent over.
function mapDisplayToServerIndices(displayOrder: string[], serverHand: string[]): number[] {
  const used = new Array(serverHand.length).fill(false);
  return displayOrder.map((c) => {
    const idx = serverHand.findIndex((v, i) => v === c && !used[i]);
    if (idx === -1) return -1;
    used[idx] = true;
    return idx;
  });
}

function handOrderStorageKey(gameId: string, userId: string): string {
  return `zolik_hand_order_${gameId}_${userId}`;
}

export default function GameScreen() {
  const { gameId } = useLocalSearchParams<{ gameId: string }>();
  const id = String(gameId ?? '');
  const { session } = useSession();
  const { setRoundEnd, setGameEnd } = useGameFlow();
  // One array of staged card values per pending meld — see
  // resolveGroupIndices. Always at least one (possibly empty) group so
  // there's always somewhere for a tap/drag to land.
  const [groups, setGroups] = useState<string[][]>([[]]);
  // Hand indices tapped to select for melding but not yet staged — a card
  // in here stays visible in the hand row (with the gold ring) until "+ Add
  // to meld" moves it into `groups`. Dragging a card straight onto the
  // staging area skips this and stages it immediately (see stageCard).
  // A plain tap on a hand card toggles this — the gold-ring selection used
  // to pick one or more cards for the per-meld "Lay off here" / "Swap joker
  // here" buttons in MeldTable (a card in here still shows in the hand row;
  // dragging one of the selected cards onto a meld lays off the whole
  // selection together, see onDropOnMeld below). Every other meld action
  // (staging a new meld, laying off a single unselected card, discarding)
  // happens by dragging the card straight to its target instead — see
  // stageCard/layOffCardAt/discardCardAt.
  const [selectedForMeld, setSelectedForMeld] = useState<Set<number>>(new Set());
  const [localHand, setLocalHand] = useState<string[] | null>(null);
  // Custom hand order (auto-organize or manual drag) loaded from disk, so it
  // survives a reconnect that remounts this screen (app restart/reload) —
  // not just the in-memory reconciliation the effect below already does for
  // ordinary draws/discards within a single mount. `undefined` means "not
  // loaded yet"; `null` means "loaded, nothing was saved".
  const persistedHandOrderRef = useRef<string[] | null | undefined>(undefined);
  const discardZoneRef = useRef<View>(null);
  const stagingZoneRef = useRef<View>(null);
  const meldViewRefs = useRef<Map<string, View>>(new Map());
  // Keyed by group index rather than meld ID — a staging group has no
  // stable ID of its own, just its position in `groups` (see
  // measureGroupRowZones below).
  const groupRowRefs = useRef<Map<number, View>>(new Map());
  const overlayRootRef = useRef<View>(null);
  const overlayOriginX = useSharedValue(0);
  const overlayOriginY = useSharedValue(0);
  const dragPreview = useDragPreview();
  // Deck-drag gets its own DragPreview/flip rather than reusing `dragPreview`
  // above — that one is keyed to `draggedCard` (a hand card) for the card
  // overlay below; conflating the two would mean a deck drag's `active`
  // toggling also affects hand-drag bookkeeping it has nothing to do with.
  const deckDragPreview = useDragPreview();
  const deckFlip = useSharedValue(0);
  const handRowZoneRef = useRef<View>(null);
  const [draggedCard, setDraggedCard] = useState<string | null>(null);
  // Which meld (and which end of it) a card being dragged is currently
  // over — live drag feedback, distinct from the drop-time check in
  // onDropOnMeld below. Cleared whenever the drag ends (see
  // onDragCardChange).
  const [hoverTarget, setHoverTarget] = useState<MeldHoverTarget>(null);
  // Same idea as hoverTarget, but for the staging area: which group and
  // which position within it a card being dragged is currently over — see
  // handleDragHover and MeldStagingArea's insertHover prop. Cleared
  // whenever the drag ends (see onDragCardChange).
  const [stagingInsertHover, setStagingInsertHover] = useState<{
    groupIndex: number;
    pos: number;
  } | null>(null);
  // Meld that just received a successful lay-off — flashed green briefly so
  // the drop reads as "landed" rather than the card just silently
  // disappearing once the server confirms it. Set when we send lay_off,
  // cleared automatically after a short delay (see the effect below).
  const [flashMeldId, setFlashMeldId] = useState<string | null>(null);
  const flashMeldTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  function flashMeld(meldId: string) {
    setFlashMeldId(meldId);
    if (flashMeldTimeoutRef.current) clearTimeout(flashMeldTimeoutRef.current);
    flashMeldTimeoutRef.current = setTimeout(() => setFlashMeldId(null), 600);
  }
  const lastHoverCheckAtRef = useRef(0);
  // Throttled rather than run on every pan frame — a live highlight only
  // needs to feel immediate, not literally hit 60fps, and each check costs
  // one measureInWindow round trip per table meld.
  const HOVER_CHECK_INTERVAL_MS = 60;
  // Outer page ScrollView — melds/staging area sit near the top, the hand
  // row near the bottom, and with several melds down (or a tall keyboard/
  // small screen) both can't be on screen at once. Dragging a card toward
  // either edge auto-scrolls the page so the far end is reachable without
  // letting go mid-drag. scrollOffsetYRef mirrors the live scroll position
  // (via onScroll below) since scrollTo takes an absolute offset, not a
  // relative delta.
  const outerScrollRef = useRef<ScrollView>(null);
  const scrollOffsetYRef = useRef(0);
  const AUTO_SCROLL_EDGE = 90;
  const AUTO_SCROLL_MAX_STEP = 16;
  function handleAutoScroll(absoluteY: number) {
    const screenHeight = Dimensions.get('window').height;
    let delta = 0;
    if (absoluteY < AUTO_SCROLL_EDGE) {
      delta = -AUTO_SCROLL_MAX_STEP * (1 - absoluteY / AUTO_SCROLL_EDGE);
    } else if (absoluteY > screenHeight - AUTO_SCROLL_EDGE) {
      delta = AUTO_SCROLL_MAX_STEP * (1 - (screenHeight - absoluteY) / AUTO_SCROLL_EDGE);
    }
    if (delta === 0) return;
    const nextY = Math.max(0, scrollOffsetYRef.current + delta);
    outerScrollRef.current?.scrollTo({ y: nextY, animated: false });
  }
  function handleDragHover(absoluteX: number, absoluteY: number) {
    const now = Date.now();
    if (now - lastHoverCheckAtRef.current < HOVER_CHECK_INTERVAL_MS) return;
    lastHoverCheckAtRef.current = now;
    handleAutoScroll(absoluteY);
    measureMeldZones((zones) => {
      const hit = closestZone(absoluteX, absoluteY, zones);
      if (hit) {
        setStagingInsertHover(null);
        if (hit.type !== 'run') {
          setHoverTarget({ meldId: hit.meldId, position: 'front' });
          return;
        }
        const position = absoluteX < hit.zone.x + hit.zone.width / 2 ? 'front' : 'end';
        setHoverTarget({ meldId: hit.meldId, position });
        return;
      }
      setHoverTarget(null);
      // Not over any table meld — check the staging area next, so the
      // player sees exactly where a card would land within the group
      // they're dragging over (not just "somewhere in this box").
      measureGroupRowZones((zones2) => {
        const ghit = zones2.find(({ zone }) => pointInZone(absoluteX, absoluteY, zone));
        if (!ghit) {
          setStagingInsertHover(null);
          return;
        }
        setStagingInsertHover({
          groupIndex: ghit.groupIndex,
          pos: computeInsertIndex(absoluteX, ghit.zone, ghit.count),
        });
      });
    });
  }
  const [discardFlash, setDiscardFlash] = useState(false);
  const prevTopDiscardRef = useRef<string | undefined>(undefined);
  // Tracks the (game, phase, sorted-hand) tuple the staging groups were
  // last reset for — see the effect below.
  const resetKeyRef = useRef<string>('');
  const [justDrawnCard, setJustDrawnCard] = useState<string | null>(null);
  const [rulesModalOpen, setRulesModalOpen] = useState(false);
  // Collapsed by default so the "Your hand (N)" label + how-to-play text —
  // whose content/length changes with phase — never pushes the hand row
  // around during ordinary play. Only an explicit tap expands it, which is
  // expected motion rather than a surprise one.
  const [handInfoOpen, setHandInfoOpen] = useState(false);
  const prevHandRef = useRef<{ game: number | undefined; hand: string[] }>({
    game: undefined,
    hand: [],
  });

  // Measured live at drag-end time (see HandRow) rather than cached from
  // scroll/layout events, so a drop always sees the current on-screen rect
  // even if the layout shifted since the last such event fired.
  // Grows a measured rect by `margin` on every side before it's used for hit
  // testing — a drop that lands just outside the visible box (a finger
  // slightly overshooting a small target, e.g. the minimized staging strip)
  // still counts, rather than requiring pixel-perfect aim.
  function inflateZone(zone: DropZone, margin: number): DropZone {
    return inflateZoneXY(zone, margin, margin);
  }
  function inflateZoneXY(zone: DropZone, marginX: number, marginY: number): DropZone {
    return {
      x: zone.x - marginX,
      y: zone.y - marginY,
      width: zone.width + marginX * 2,
      height: zone.height + marginY * 2,
    };
  }
  const DROP_ZONE_HIT_SLOP = 24;
  // Table meld rows now sit close together (tightened up to keep the melds
  // area compact — see meldRow spacing in MeldTable), so a generous vertical
  // slop here would make neighboring melds' hit zones swallow each other.
  // Horizontal slop stays generous (rows are as wide as the screen, so
  // there's no neighbor to collide with sideways); vertical slop is small
  // enough to stay forgiving without bleeding far into the next row —
  // closestZone (see HandRow) still resolves any remaining overlap by
  // picking whichever meld's center the drop actually landed nearest.
  const MELD_HIT_SLOP_X = 28;
  const MELD_HIT_SLOP_Y = 10;

  function measureDropZone(cb: (zone: DropZone | null) => void) {
    if (!discardZoneRef.current) {
      cb(null);
      return;
    }
    discardZoneRef.current.measureInWindow((x, y, width, height) =>
      cb(inflateZone({ x, y, width, height }, DROP_ZONE_HIT_SLOP)),
    );
  }

  // Used by the deck pile's drag-to-draw gesture: the drop target and the
  // flip animation's vertical range both need the hand row's live rect, not
  // one measured back when it was last laid out.
  function measureHandZone(cb: (zone: DropZone | null) => void) {
    if (!handRowZoneRef.current) {
      cb(null);
      return;
    }
    handRowZoneRef.current.measureInWindow((x, y, width, height) => cb({ x, y, width, height }));
  }

  function measureStagingZone(cb: (zone: DropZone | null) => void) {
    if (!canBuildMeld || !stagingZoneRef.current) {
      cb(null);
      return;
    }
    stagingZoneRef.current.measureInWindow((x, y, width, height) =>
      cb(inflateZone({ x, y, width, height }, DROP_ZONE_HIT_SLOP)),
    );
  }

  // Per-group card-row rects within the staging area — lets a dropped card
  // resolve to a specific group and position within it, rather than always
  // landing at the end of whichever group happens to be last (see
  // stageCardsAt). Mirrors measureMeldZones' shape/approach.
  function measureGroupRowZones(
    cb: (zones: { groupIndex: number; zone: DropZone; count: number }[]) => void,
  ) {
    const entries = canBuildMeld ? Array.from(groupRowRefs.current.entries()) : [];
    if (entries.length === 0) {
      cb([]);
      return;
    }
    const results: { groupIndex: number; zone: DropZone; count: number }[] = [];
    let remaining = entries.length;
    entries.forEach(([groupIndex, el]) => {
      el.measureInWindow((x, y, width, height) => {
        results.push({
          groupIndex,
          zone: { x, y, width, height },
          count: groups[groupIndex]?.length ?? 0,
        });
        remaining -= 1;
        if (remaining === 0) cb(results);
      });
    });
  }

  function registerGroupRowRef(groupIndex: number, el: View | null) {
    if (el) groupRowRefs.current.set(groupIndex, el);
    else groupRowRefs.current.delete(groupIndex);
  }

  // A group's row lays its cards out left-to-right, so the fraction of the
  // row's width a drop point falls at maps directly to "insert before the
  // card at this index" — 0 cards clamps to index 0, count cards clamps to
  // "append at the end". Approximate (the row is center-justified, so with
  // few cards there's blank space at both edges included in the fraction)
  // rather than measuring each individual card's rect, which is a good
  // enough trade for how much simpler it keeps the drop handling.
  function computeInsertIndex(x: number, zone: DropZone, count: number): number {
    if (count === 0) return 0;
    const fraction = (x - zone.x) / zone.width;
    return Math.max(0, Math.min(count, Math.round(fraction * count)));
  }

  // Looks up a meld's type (run/set) across every player's meldMeta —
  // needed so a dragged card's drop zone can tell whether front/end
  // splitting even applies (only runs have ends).
  function meldTypeById(meldId: string): 'run' | 'set' | undefined {
    if (!state) return undefined;
    for (const p of state.players) {
      const meta = (state.meldMeta[p.id] ?? []).find((m) => m.meldId === meldId);
      if (meta) return meta.type as 'run' | 'set';
    }
    return undefined;
  }

  function measureMeldZones(cb: (zones: MeldZone[]) => void) {
    const entries = canLayOff ? Array.from(meldViewRefs.current.entries()) : [];
    if (entries.length === 0) {
      cb([]);
      return;
    }
    const results: MeldZone[] = [];
    let remaining = entries.length;
    entries.forEach(([meldId, el]) => {
      el.measureInWindow((x, y, width, height) => {
        results.push({
          meldId,
          zone: inflateZoneXY({ x, y, width, height }, MELD_HIT_SLOP_X, MELD_HIT_SLOP_Y),
          type: meldTypeById(meldId),
        });
        remaining -= 1;
        if (remaining === 0) cb(results);
      });
    });
  }

  function registerMeldRef(meldId: string, el: View | null) {
    if (el) meldViewRefs.current.set(meldId, el);
    else meldViewRefs.current.delete(meldId);
  }

  // The overlay is positioned with the gesture's window-relative
  // absoluteX/absoluteY, but position:absolute is relative to this
  // component's own box — which sits below the Stack navigator's header,
  // not the true window origin. Measuring that offset (rather than
  // assuming it's zero) keeps the overlay glued to the cursor regardless
  // of header height, safe-area insets, or Screen's own padding.
  function measureOverlayOrigin() {
    overlayRootRef.current?.measureInWindow((x, y) => {
      overlayOriginX.value = x;
      overlayOriginY.value = y;
    });
  }

  // Floats above the whole screen (outside the scrolling hand row, which
  // would otherwise clip it) and tracks the finger during a drag so the
  // dragged card visibly sits on top of the discard pile instead of being
  // hidden behind it.
  const dragOverlayStyle = useAnimatedStyle(() => ({
    position: 'absolute',
    left: dragPreview.x.value - dragPreview.offsetX.value - overlayOriginX.value,
    top: dragPreview.y.value - dragPreview.offsetY.value - overlayOriginY.value,
    opacity: dragPreview.active.value ? 1 : 0,
    zIndex: 1000,
    elevation: 1000,
  }));

  const onRoundEnd = useCallback(
    (data: WSEnvelope, state: GameState) => {
      setRoundEnd({ data, state, gameId: id });
      router.push('/round-end');
    },
    [id, setRoundEnd],
  );

  const onGameEnd = useCallback(
    (data: WSEnvelope, state: GameState) => {
      setGameEnd({ data, state });
      if (id && session?.userId) {
        storage.deleteItem(handOrderStorageKey(id, session.userId)).catch(() => {});
      }
      router.push('/game-end');
    },
    [setGameEnd, id, session?.userId],
  );

  const { state, status, statusIsError, connected, send } = useGameSocket({
    gameId: id,
    onRoundEnd,
    onGameEnd,
  });

  const hand = localHand ?? state?.myHand ?? [];
  // hand[i]'s slot in state.myHand (the server's own hand array) — see
  // mapDisplayToServerIndices. Needed so discarding a duplicate-value card
  // (two decks in play) tells the server which physical instance was
  // dropped rather than just its value, which the server can't otherwise
  // tell apart from another copy sitting elsewhere in the hand.
  const handServerIndices = mapDisplayToServerIndices(hand, state?.myHand ?? []);
  const userId = session?.userId ?? '';

  // Loads any previously saved custom hand order for this game+player as
  // soon as we know who's asking, so a remounted screen (app relaunch, or
  // navigating away and back) picks the saved arrangement back up instead of
  // falling back to raw server/deal order. Runs once per game+user.
  useEffect(() => {
    if (!id || !userId) return;
    let cancelled = false;
    storage.getItem(handOrderStorageKey(id, userId)).then((raw) => {
      if (cancelled) return;
      let parsed: string[] | null = null;
      if (raw) {
        try {
          const arr = JSON.parse(raw);
          if (Array.isArray(arr)) parsed = arr;
        } catch {
          parsed = null;
        }
      }
      persistedHandOrderRef.current = parsed;
      if (parsed) {
        setLocalHand((prev) => prev ?? reconcileHandOrder(parsed, state?.myHand ?? parsed));
      }
    });
    return () => {
      cancelled = true;
    };
  }, [id, userId]);

  // The server sends a fresh `myHand` array on every broadcast — including
  // ones that have nothing to do with you (an opponent/AI's move) — so a
  // new array *reference* doesn't mean your hand actually changed. Resetting
  // in-progress meld staging on every such broadcast made the staging area
  // appear to "refresh" out from under you mid-build. Only reset when the
  // hand's contents (or phase/round) actually changed.
  useEffect(() => {
    const newHand = state?.myHand ?? [];
    setLocalHand((prev) => {
      const base = prev ?? persistedHandOrderRef.current ?? null;
      return reconcileHandOrder(base, newHand);
    });
    const key = `${state?.game ?? ''}|${state?.phase ?? ''}|${[...newHand].sort().join(',')}`;
    if (key !== resetKeyRef.current) {
      resetKeyRef.current = key;
      setGroups([[]]);
      setSelectedForMeld(new Set());
    }
  }, [state?.myHand, state?.phase, state?.game]);

  // Persists the current custom hand order (auto-organize or manual drag) to
  // disk so it survives a reconnect that remounts this screen — e.g. an app
  // restart, not just a WebSocket drop within the same mount.
  useEffect(() => {
    if (!id || !userId || !localHand) return;
    storage.setItem(handOrderStorageKey(id, userId), JSON.stringify(localHand)).catch(() => {});
  }, [id, userId, localHand]);

  // Tracks the card(s) just drawn from the deck/discard pile so they can be
  // highlighted in the hand — a multiset diff against the previous server
  // hand (not localHand, which reordering shouldn't affect) rather than
  // "last element", since draws always land at the tail but melds/discards
  // shift what's there. Sticky across the rest of the turn: a meld/discard
  // (pure removal) never grows any count, so the highlight naturally
  // survives everything except the next draw or the turn ending.
  useEffect(() => {
    const newHand = state?.myHand ?? [];
    const prev = prevHandRef.current;
    if (prev.game === state?.game) {
      const prevCounts = new Map<string, number>();
      for (const c of prev.hand) prevCounts.set(c, (prevCounts.get(c) ?? 0) + 1);
      const newCounts = new Map<string, number>();
      for (const c of newHand) newCounts.set(c, (newCounts.get(c) ?? 0) + 1);
      for (const [c, n] of newCounts) {
        if (n > (prevCounts.get(c) ?? 0)) {
          setJustDrawnCard(c);
          break;
        }
      }
    } else {
      setJustDrawnCard(null);
    }
    prevHandRef.current = { game: state?.game, hand: newHand };
  }, [state?.myHand, state?.game]);

  // Flashes the discard pile whenever a new card lands on top, so a
  // drag-drop (or button/tap discard) gets a visible "yes, that landed"
  // confirmation instead of the pile just silently changing.
  useEffect(() => {
    const discardPile = state?.discardPile ?? [];
    const top = discardPile[discardPile.length - 1];
    if (prevTopDiscardRef.current !== undefined && top && top !== prevTopDiscardRef.current) {
      setDiscardFlash(true);
      const t = setTimeout(() => setDiscardFlash(false), 800);
      prevTopDiscardRef.current = top;
      return () => clearTimeout(t);
    }
    prevTopDiscardRef.current = top;
  }, [state?.discardPile]);
  const isMyTurn = state?.currentTurn === userId;
  const phase = state?.phase ?? '';

  // Once the turn passes, the highlight is stale until the next draw.
  useEffect(() => {
    if (!isMyTurn) setJustDrawnCard(null);
  }, [isMyTurn]);

  // Moves one or more cards straight into the group currently being built,
  // inserted at insertPos rather than always appended at the end — reached
  // by dragging a hand card onto the staging area (see onDropOnStaging
  // below), which resolves the drop point to this exact (groupIndex,
  // insertPos) via measureGroupRowZones/computeInsertIndex. `indices` holds
  // more than one hand index when the dragged card was part of a
  // multi-card selection (see onDropOnStaging), so a whole selection can be
  // staged together in one drag instead of one card at a time.
  function stageCardsAt(indices: number[], groupIndex: number, insertPos: number) {
    // Melding is only a thing between your draw and your discard — before
    // you've drawn there's nothing to meld with yet.
    if (!canBuildMeld) return;
    const cards = indices.map((i) => hand[i]).filter((c): c is string => !!c);
    if (cards.length === 0) return;
    setGroups((prev) => {
      const next = prev.map((g) => [...g]);
      const gi = Math.max(0, Math.min(next.length - 1, groupIndex));
      const pos = Math.max(0, Math.min(next[gi].length, insertPos));
      next[gi].splice(pos, 0, ...cards);
      return next;
    });
    setSelectedForMeld((prev) => {
      if (indices.every((i) => !prev.has(i))) return prev;
      const next = new Set(prev);
      indices.forEach((i) => next.delete(i));
      return next;
    });
  }

  // Pulls a card back out of whichever staged group it's in (tapping a
  // staged card in the meld area) — the reverse of stageCardsAt.
  function unstageCard(index: number) {
    const card = hand[index];
    if (!card) return;
    const resolved = resolveGroupIndices(hand, groups);
    const groupIdx = resolved.findIndex((idxs) => idxs.includes(index));
    if (groupIdx === -1) return;
    setGroups((prev) => {
      const next = prev.map((g) => [...g]);
      const pos = next[groupIdx].indexOf(card);
      if (pos !== -1) next[groupIdx].splice(pos, 1);
      return next;
    });
  }

  // A plain tap on a hand card — selects it with the gold ring for the
  // *lay-off* / *swap joker* flow: pick one or more cards this way, then
  // either tap a table meld's "Lay off here" / "Swap joker here" button, or
  // drag one of the selected cards straight onto the meld (see
  // layOffOnto/swapJokerOnto/onDropOnMeld below).
  function toggleHandSelect(index: number) {
    if (!canBuildMeld) return;
    setSelectedForMeld((prev) => {
      const next = new Set(prev);
      if (next.has(index)) next.delete(index);
      else next.add(index);
      return next;
    });
  }

  function reorderGroup(groupIndex: number, from: number, to: number) {
    setGroups((prev) => {
      if (!prev[groupIndex]) return prev;
      const next = prev.map((g) => [...g]);
      next[groupIndex] = moveCardToIndex(next[groupIndex], from, to);
      return next;
    });
  }

  function clearSelect() {
    setGroups([[]]);
    setSelectedForMeld(new Set());
  }

  // Phase/round-requirement legality is left to the server (which replies
  // with a readable error surfaced via `status`) rather than silently
  // no-oping here — a card dropped on the discard pile or a meld should
  // always visibly do *something*, even if that's "not allowed right now".
  function discardCardAt(index: number) {
    if (!state || !isMyTurn) return;
    if (phase !== 'discard' && phase !== 'meld') return;
    const card = hand[index];
    if (!card) return;
    const cardIndex = handServerIndices[index];
    send({ type: 'discard', card, cardIndex: cardIndex >= 0 ? cardIndex : undefined });
    clearSelect();
  }

  // Lay-off is a post-"down" action: you can only extend melds (yours or
  // anyone else's) once you've met your own round requirement this deal.
  const canLayOff = !!state && isMyTurn && phase === 'meld' && !!state.roundReqMet[userId];
  const canBuildMeld = !!state && isMyTurn && phase === 'meld';
  // Mirrors discardCardAt's own guard — used to decide whether the discard
  // pile pulses as a valid drop target while a card is being dragged.
  const canDiscardNow = !!state && isMyTurn && (phase === 'discard' || phase === 'meld');
  // Hook, so it has to run unconditionally on every render (including the
  // "loading" render before `state` exists below) — same reason the drag
  // overlay's shared values are declared up top instead of inside the JSX.
  const discardPulseStyle = useDropPulseStyle(draggedCard !== null && canDiscardNow);

  // Minimum run length mirrors the server's per-profile rule (see
  // rulesSummaryLines) — Žolík Classic allows 3-card runs, everything else
  // needs 4.
  const minRunLength = state?.rulesProfile === 'zolik_classic' ? 3 : 4;

  // The moment the group currently being built already reads as a complete
  // set or run, open the next box automatically — so a hand with two melds
  // to lay doesn't need a manual "+ Add another run or set" tap in between.
  // Runs once per group: right after this appends the empty tail, the tail
  // itself is what's now last and it's empty, so the check below no-ops
  // (returns the same array reference, so React doesn't even re-render).
  useEffect(() => {
    if (!canBuildMeld) return;
    setGroups((prev) => {
      const tail = prev[prev.length - 1];
      if (!tail || tail.length === 0 || !isViableMeld(tail, minRunLength)) return prev;
      return [...prev, []];
    });
  }, [groups, canBuildMeld, minRunLength]);

  function layOffCardAt(index: number, meldId: string, position: 'front' | 'end') {
    if (!canLayOff) return;
    const card = hand[index];
    if (!card) return;
    // Dropping a card onto a meld always means "lay this off/extend the
    // meld" — even when the meld holds a joker, e.g. dropping a J onto a
    // Q-JOKER-A run to extend it to J-Q-JOKER-A. Swapping a card into the
    // joker's own slot is a distinct, less common intent with its own
    // explicit "Swap joker here" button (see swapJokerOnto) rather than
    // being guessed from the drop target.
    //
    // Which half of the meld the card was dropped on tells the server
    // which end of a run it has to extend — a set has no ends, so the
    // server just ignores this for sets.
    send({ type: 'lay_off', meldId, card, position });
    flashMeld(meldId);
    clearSelect();
  }

  if (!state) {
    return (
      <Screen title="Game">
        <Text style={shared.status}>{status || 'Loading game…'}</Text>
        {/* The socket already auto-reconnects with backoff (see
            useGameSocket) — a manual "Reconnect" button here would just
            race that retry, so the only action worth offering is bailing
            out to start over. */}
        <Pressable style={shared.button} onPress={() => router.replace('/lobby/create')}>
          <Text style={shared.buttonText}>Start a new game</Text>
        </Pressable>
      </Screen>
    );
  }

  // Defends against a null discardPile from the server (a nil Go slice
  // serializes to JSON `null`, not `[]`) rather than assuming the array is
  // always present — see the DiscardPile comment in
  // rules.snapshotTurnMeld for the bug this guards against.
  const discardPileSafe = state.discardPile ?? [];
  const topDiscard = discardPileSafe[discardPileSafe.length - 1];
  const anyMelds = state.players.some((p) => (state.melds[p.id] ?? []).length > 0);
  const header = `${dealHeaderLabel(state.rulesProfile, state.game)} · Round ${state.round} · Deck ${state.deckCount}`;
  const turnLabel = isMyTurn
    ? 'Your turn'
    : (() => {
        const p = state.players.find((x) => x.id === state.currentTurn);
        return p ? `${p.name}'s turn` : 'Waiting…';
      })();

  const discardLocked = state.discardLocked;
  const canDrawDeck = isMyTurn && phase === 'draw';
  const canTakeDiscard = canDrawDeck && !discardLocked && !!topDiscard;

  function drawFromDeck() {
    if (!canDrawDeck) return;
    send({ type: 'draw_card', from: 'deck' });
    clearSelect();
  }

  function takeDiscard() {
    if (!canTakeDiscard) return;
    send({ type: 'draw_card', from: 'discard' });
    clearSelect();
  }

  const resolvedGroups = resolveGroupIndices(hand, groups);
  const stagingGroups: StagingGroup[] = resolvedGroups.map((indices) => ({
    entries: indices.map((index) => ({ index, card: hand[index] })),
  }));
  const allStaged = new Set(resolvedGroups.flat());

  // A card moved into the meld area comes out of the hand entirely — only
  // Cancel (see cancelGroup) puts it back — so the hand row only ever shows
  // what's still actually in your hand to play with. HandRow only knows
  // about the cards it's given, so its indices are positions within this
  // filtered array; visibleToFullIndex translates them back to `hand`
  // positions for every callback below.
  const visibleToFullIndex: number[] = [];
  const visibleHand: string[] = [];
  hand.forEach((card, i) => {
    if (!allStaged.has(i)) {
      visibleToFullIndex.push(i);
      visibleHand.push(card);
    }
  });
  const visibleSelected = new Set<number>();
  visibleToFullIndex.forEach((fullIndex, vi) => {
    if (selectedForMeld.has(fullIndex)) visibleSelected.add(vi);
  });

  const nonEmptyGroups = groups.filter((g) => g.length > 0);

  // Lays every group that currently has cards in it — one lay_meld per
  // group, sent back to back over the same connection so the server
  // processes them in order. A hand that only ever builds one group (the
  // common case) just lays that one; a run+set built side by side goes out
  // together in the same tap instead of needing two separate presses.
  function layAllGroups() {
    if (nonEmptyGroups.length === 0) return;
    for (const cards of nonEmptyGroups) {
      send({ type: 'lay_meld', cards });
    }
    setGroups([[]]);
  }

  function cancelGroup(groupIndex: number) {
    setGroups((prev) => {
      const next = prev.filter((_, i) => i !== groupIndex);
      return next.length ? next : [[]];
    });
  }

  // Only offer a new box once the current last one actually has cards in
  // it — otherwise "+ Add another meld" would just pile up empty boxes.
  const canAddGroup = groups.length > 0 && groups[groups.length - 1].length > 0;

  function addGroup() {
    if (!canAddGroup) return;
    setGroups((prev) => [...prev, []]);
  }

  const actions: { label: string; onPress: () => void; disabled?: boolean; active?: boolean }[] = [];

  // Always visible (grayed out rather than hidden) so it's obvious *why*
  // you can't draw right now — wrong phase, not your turn, or the
  // discard pile is locked — instead of the action just disappearing.
  actions.push({ label: 'Draw deck', onPress: drawFromDeck, disabled: !canDrawDeck });
  actions.push({ label: 'Take discard', onPress: takeDiscard, disabled: !canTakeDiscard });
  // Cards currently selected for a meld action — hoisted out of the
  // per-phase block below because MeldTable also needs it, to decide which
  // per-meld "Lay off here" / "Swap joker here" buttons to show.
  const meldSelectedCards = phase === 'meld' ? selectedCards(hand, selectedForMeld) : [];

  function layOffOnto(meldId: string, position?: 'front' | 'end') {
    if (meldSelectedCards.length === 0) return;
    send({ type: 'lay_off', meldId, cards: meldSelectedCards, position });
    flashMeld(meldId);
    clearSelect();
  }

  function undoLastLayOff() {
    if (!state?.canUndoLayOff) return;
    send({ type: 'undo_lay_off' });
  }

  function undoLastMeld() {
    if (!state?.canUndoLayMeld) return;
    send({ type: 'undo_lay_meld' });
  }

  function undoTakeDiscard() {
    if (!state?.canUndoDiscardDraw) return;
    send({ type: 'undo_draw_discard' });
  }

  // Unlike the single-step undos below (which only reach back one action),
  // this reverts every meld/lay-off/joker-swap made since the draw that
  // started this turn's meld phase — the always-available fallback for
  // "get back to a known state before I discard," no matter how many
  // actions have piled up since.
  function undoTurn() {
    if (!state?.canUndoTurn) return;
    send({ type: 'undo_turn' });
    clearSelect();
  }

  function swapJokerOnto(meldId: string) {
    if (meldSelectedCards.length !== 1) return;
    send({ type: 'swap_joker', meldId, card: meldSelectedCards[0] });
    clearSelect();
  }

  // Always pushed (grayed out rather than omitted) so the action bar's
  // button count — and everything laid out below it — never changes as
  // these become/stop being available mid-turn; see the disabled-not-
  // unmounted pattern used throughout this screen.
  //
  // Undo is only offered for taking the discard pile card — the game's
  // actual rule is that you can back out of that pickup and draw from the
  // deck instead, before anything else this turn has happened. Lay-off,
  // meld, and whole-turn undo remain implemented server-side but aren't
  // surfaced in this UI since they're not legal undos in this game.
  actions.push({
    label: 'Undo take discard',
    onPress: undoTakeDiscard,
    disabled: !state?.canUndoDiscardDraw,
  });

  const discardSelectedCards = isMyTurn && phase === 'discard' ? selectedCards(hand, allStaged) : [];
  const discardSelectedIndex = isMyTurn && phase === 'discard' ? [...allStaged][0] : undefined;
  actions.push({
    label: 'Discard',
    onPress: () => {
      if (discardSelectedCards.length !== 1 || discardSelectedIndex === undefined) return;
      const cardIndex = handServerIndices[discardSelectedIndex];
      send({
        type: 'discard',
        card: discardSelectedCards[0],
        cardIndex: cardIndex >= 0 ? cardIndex : undefined,
      });
      clearSelect();
    },
    disabled: discardSelectedCards.length !== 1,
  });

  return (
    // The drag overlay's position:absolute left/top comes from the
    // gesture's window-relative absoluteX/absoluteY, but this View sits
    // below the Stack navigator's header (and Screen's own padding), not
    // at the window origin. measureOverlayOrigin (called on layout, since
    // the header's presence/height isn't known upfront) tells the overlay
    // style how far this box is offset from the window so it can subtract
    // that out — otherwise the dragged card renders header-height too low
    // and no longer tracks the cursor.
    <View ref={overlayRootRef} style={{ flex: 1 }} onLayout={measureOverlayOrigin}>
      <Screen>
        <ScrollView
          testID="game-scroll-view"
          ref={outerScrollRef}
          onScroll={(e) => {
            scrollOffsetYRef.current = e.nativeEvent.contentOffset.y;
          }}
          scrollEventThrottle={32}
        >
          <View style={{ flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between' }}>
            <View style={{ flex: 1 }}>
              <Text style={{ color: colors.accent, fontSize: 12, fontWeight: '700' }}>
                {profileDisplayName(state.rulesProfile)}
              </Text>
              <Text style={shared.title}>{header}</Text>
            </View>
            {/* Always available at the top, not just once you've gone
                offline — the socket already auto-reconnects on its own, so
                the one thing worth a dedicated escape hatch is bailing out
                to a fresh game, not retrying the connection by hand. */}
            <Pressable
              onPress={() => router.replace('/lobby/create')}
              style={{
                borderWidth: 1,
                borderColor: colors.border,
                borderRadius: 8,
                paddingVertical: 6,
                paddingHorizontal: 10,
                marginRight: 8,
              }}
            >
              <Text style={{ color: colors.text, fontSize: 12, fontWeight: '600' }}>New game</Text>
            </Pressable>
            <Pressable
              onPress={() => setRulesModalOpen(true)}
              style={{
                borderWidth: 1,
                borderColor: colors.border,
                borderRadius: 8,
                paddingVertical: 6,
                paddingHorizontal: 10,
              }}
            >
              <Text style={{ color: colors.text, fontSize: 12, fontWeight: '600' }}>Rules</Text>
            </Pressable>
          </View>
        <Text style={[shared.status, { color: isMyTurn ? colors.success : colors.muted }]}>
          {turnLabel} · {phase}
          {!connected ? ' · reconnecting…' : ''}
        </Text>
        {/* Always mounted at a fixed height (whether or not there's
            anything to say) so a status appearing/clearing (Connected,
            Deck recycled, a rule error, ...) never grows/shrinks the
            layout and shoves everything below it up and down. */}
        <View style={[pileStyles.statusRow, statusIsError && pileStyles.statusRowError]}>
          {statusIsError ? <Text style={shared.errorBannerIcon}>⚠</Text> : null}
          <Text style={statusIsError ? shared.errorBannerText : pileStyles.statusRowText}>
            {status || ' '}
          </Text>
        </View>

        <Text style={[shared.status, { marginTop: 8 }]}>Opponents</Text>
        {state.players
          .filter((p) => p.id !== userId)
          .map((p) => (
            <Text key={p.id} style={{ color: colors.text }}>
              {p.name}: {state.cardCounts[p.id] ?? 0} cards
              {state.roundReqMet[p.id] ? ' ✓' : ''}
            </Text>
          ))}

        <View style={pileStyles.meldBox}>
          <Text style={pileStyles.meldBoxLabel}>MELDS</Text>
          {anyMelds ? (
            <MeldTable
              state={state}
              myUserId={userId}
              onMeldRef={registerMeldRef}
              hoverTarget={hoverTarget}
              dragActive={canLayOff && draggedCard !== null}
              flashMeldId={flashMeldId}
              selectedCards={meldSelectedCards}
              canLayOff={canLayOff}
              onLayOff={layOffOnto}
              onSwapJoker={swapJokerOnto}
            />
          ) : (
            <Text style={pileStyles.meldBoxEmpty}>No melds on the table yet</Text>
          )}

          {/* Always mounted (not just during the meld phase) so the boxes
              below it — the pending-meld warning, action bar, deck/discard
              piles — never jump position as the phase changes; its own
              buttons disable instead of unmounting for the same reason
              (see the comment in MeldStagingArea itself). */}
          <MeldStagingArea
            ref={stagingZoneRef}
            groups={stagingGroups}
            dragActive={canBuildMeld && draggedCard !== null}
            onRemove={unstageCard}
            onReorderGroup={reorderGroup}
            onCancelGroup={cancelGroup}
            onAddGroup={addGroup}
            canAddGroup={canBuildMeld && canAddGroup}
            onLayAll={layAllGroups}
            canLayAll={canBuildMeld && nonEmptyGroups.length > 0}
            layCount={nonEmptyGroups.length}
            onGroupRowRef={registerGroupRowRef}
            insertHover={stagingInsertHover}
          />
        </View>

        {/* Always mounted at a fixed height (same treatment as statusRow
            above) so the action bar and piles below never shift position
            as this warning comes and goes mid-turn. */}
        <View
          style={[
            pileStyles.statusRow,
            isMyTurn && state.discardDrawnCardPendingMeld ? pileStyles.statusRowError : undefined,
          ]}
        >
          <Text style={isMyTurn && state.discardDrawnCardPendingMeld ? shared.errorBannerText : pileStyles.statusRowText}>
            {isMyTurn && state.discardDrawnCardPendingMeld
              ? `You picked up ${state.discardDrawnCardPendingMeld} from the discard pile — it must go into your initial meld before you can discard.`
              : ' '}
          </Text>
        </View>

        {/* Constant, always-visible action bar — what to do right now (draw,
            take discard, discard, undo) — placed directly above the deck/
            discard piles it mostly acts on. */}
        <ActionBar actions={actions} />

        <View style={pileStyles.pilesRow}>
          <View style={pileStyles.pileBox}>
            <Text style={pileStyles.pileBoxLabel}>Draw pile</Text>
            <DeckPile
              count={state.deckCount}
              canDraw={canDrawDeck}
              onDraw={drawFromDeck}
              measureHandZone={measureHandZone}
              dragPreview={deckDragPreview}
              flip={deckFlip}
            />
          </View>

          <Animated.View
            testID="discard-zone"
            ref={discardZoneRef}
            style={[
              pileStyles.pileBox,
              { flex: 1 },
              draggedCard !== null && canDiscardNow ? discardPulseStyle : undefined,
            ]}
          >
            <Text style={pileStyles.pileBoxLabel}>
              Discard pile
              {discardLocked ? ` (locked until round ${state.discardDrawMinRound})` : ''}
            </Text>
            {topDiscard ? (
              <View style={[pileStyles.discardWrap, discardFlash && pileStyles.discardWrapFlash]}>
                {discardFlash ? <Text style={pileStyles.discardFlashLabel}>✓ Added</Text> : null}
                <CardView
                  card={topDiscard}
                  onPress={canTakeDiscard ? takeDiscard : undefined}
                  testID="discard-top-card"
                />
              </View>
            ) : (
              <Text style={shared.status}>Empty</Text>
            )}
          </Animated.View>
        </View>

        <Pressable style={pileStyles.handInfoBar} onPress={() => setHandInfoOpen((v) => !v)}>
          <Text style={shared.status}>Your hand ({hand.length})</Text>
          <View style={{ flexDirection: 'row', alignItems: 'center', gap: 12 }}>
            <Pressable onPress={() => setLocalHand(autoOrganizeHand(hand))} hitSlop={8}>
              <Text style={{ color: colors.accent }}>Auto-organize</Text>
            </Pressable>
            <Text style={pileStyles.handInfoToggle}>{handInfoOpen ? '▾ Hide help' : '▸ How to play'}</Text>
          </View>
        </Pressable>
        {handInfoOpen ? (
          <Text style={shared.status}>
            {canBuildMeld
              ? 'Drag a card anywhere: onto the discard pile to discard it, onto the box above to ' +
                'start a new meld, or onto a table meld to lay it off (glowing outlines show where ' +
                "it can land). Tap one or more cards to select them (gold ring) first if you'd " +
                'rather use the "Lay off here" / "Swap joker here" buttons, or to drag several off ' +
                'together. Drag a staged card within its run or set to reorder it.'
              : 'Drag a card onto the discard pile to discard it, or flick it upward. Drag it onto ' +
                'a table meld to lay it off.'}
          </Text>
        ) : null}
        <View ref={handRowZoneRef}>
          <HandRow
            cards={visibleHand}
            selected={visibleSelected}
            onTapCard={(vi) => toggleHandSelect(visibleToFullIndex[vi])}
            onReorder={(newVisibleOrder) => {
              const stagedCards = hand.filter((_, i) => allStaged.has(i));
              setLocalHand([...newVisibleOrder, ...stagedCards]);
            }}
            measureDropZone={measureDropZone}
            onDropOnZone={(vi) => discardCardAt(visibleToFullIndex[vi])}
            measureMeldZones={measureMeldZones}
            onDropOnMeld={(vi, meldId, position) => {
              const fullIndex = visibleToFullIndex[vi];
              // Dragging a card that's part of a multi-card selection lays off
              // the whole selection together (matching the "Lay off N here"
              // button) — not just the one card that was physically dragged.
              // Dragging an unselected card (the common case: no selection at
              // all) still lays off just that card.
              if (selectedForMeld.has(fullIndex) && meldSelectedCards.length > 1) {
                layOffOnto(meldId, position);
              } else {
                layOffCardAt(fullIndex, meldId, position);
              }
            }}
            measureStagingZone={measureStagingZone}
            onDropOnStaging={(vi, absoluteX, absoluteY) => {
              const fullIndex = visibleToFullIndex[vi];
              // Same "drag one of several selected cards to move the whole
              // selection together" rule as onDropOnMeld above.
              const indices =
                selectedForMeld.has(fullIndex) && meldSelectedCards.length > 1
                  ? Array.from(selectedForMeld)
                  : [fullIndex];
              measureGroupRowZones((zones) => {
                if (zones.length === 0) {
                  // The staging area is still in its collapsed "minimized"
                  // state (nothing staged yet at all) — MeldStagingArea
                  // doesn't render any group row to measure until there's at
                  // least one staged card, so there's nothing to hit-test
                  // against yet. The only group that can possibly exist here
                  // is group 0, empty.
                  stageCardsAt(indices, 0, 0);
                  return;
                }
                const hit = zones.find(({ zone }) => pointInZone(absoluteX, absoluteY, zone));
                // Landed inside the staging box (HandRow already checked
                // that via measureStagingZone) but not over any specific
                // group's row — over the Cancel/Add/Lay meld buttons, say.
                // Falls back to the last group, appended at the end: the
                // same behavior this always had before drop position was
                // tracked at all.
                const lastGroup = zones.reduce((a, b) => (b.groupIndex > a.groupIndex ? b : a));
                const target = hit ?? lastGroup;
                const insertPos = hit ? computeInsertIndex(absoluteX, target.zone, target.count) : target.count;
                stageCardsAt(indices, target.groupIndex, insertPos);
              });
            }}
            onDragCardChange={(card) => {
              setDraggedCard(card);
              if (!card) {
                setHoverTarget(null);
                setStagingInsertHover(null);
              }
            }}
            onDragHover={handleDragHover}
            justDrawnCard={justDrawnCard}
            dragPreview={dragPreview}
          />
        </View>

        </ScrollView>
      </Screen>
      <Animated.View style={dragOverlayStyle} pointerEvents="none">
        {draggedCard ? <CardView card={draggedCard} dragging testID="drag-overlay-card" /> : null}
      </Animated.View>
      <DeckDragOverlay
        dragPreview={deckDragPreview}
        originX={overlayOriginX}
        originY={overlayOriginY}
        flip={deckFlip}
      />
      <Modal
        visible={rulesModalOpen}
        transparent
        animationType="fade"
        onRequestClose={() => setRulesModalOpen(false)}
      >
        <Pressable
          style={{ flex: 1, backgroundColor: 'rgba(0,0,0,0.6)', justifyContent: 'center', padding: 24 }}
          onPress={() => setRulesModalOpen(false)}
        >
          <Pressable
            style={{
              backgroundColor: colors.surface,
              borderRadius: 12,
              borderWidth: 1,
              borderColor: colors.border,
              padding: 16,
            }}
            onPress={(e) => e.stopPropagation()}
          >
            <Text style={{ color: colors.text, fontWeight: '700', fontSize: 16, marginBottom: 12 }}>
              {profileDisplayName(state.rulesProfile)} rules
            </Text>
            {rulesSummaryLines(
              state.rulesProfile,
              state.game,
              state.initialMeldMinimum,
              state.discardDrawMinRound,
            ).map((line) => (
              <View key={line.label} style={{ marginBottom: 8 }}>
                <Text style={{ color: colors.muted, fontSize: 11 }}>{line.label}</Text>
                <Text style={{ color: colors.text, fontSize: 13 }}>{line.value}</Text>
              </View>
            ))}
            <Pressable
              style={[shared.button, { marginTop: 8 }]}
              onPress={() => setRulesModalOpen(false)}
            >
              <Text style={shared.buttonText}>Close</Text>
            </Pressable>
          </Pressable>
        </Pressable>
      </Modal>
    </View>
  );
}
