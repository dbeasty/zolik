import { router, useLocalSearchParams } from 'expo-router';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Modal, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';
import Animated, { useAnimatedStyle, useSharedValue } from 'react-native-reanimated';

import { ActionBar } from '@/src/components/ActionBar';
import { CardView } from '@/src/components/CardView';
import { HandRow, useDragPreview, type DropZone } from '@/src/components/HandRow';
import { MeldStagingArea, type StagingGroup } from '@/src/components/MeldStagingArea';
import { MeldTable } from '@/src/components/MeldTable';
import { Screen } from '@/src/components/Screen';
import { useGameFlow } from '@/src/context/GameFlowContext';
import { useSession } from '@/src/context/SessionContext';
import { useGameSocket } from '@/src/hooks/useGameSocket';
import type { GameState, WSEnvelope } from '@/src/api/types';
import {
  autoOrganizeHand,
  dealHeaderLabel,
  moveCardToIndex,
  profileDisplayName,
  rulesSummaryLines,
} from '@/src/lib/cards';
import { colors, shared } from '@/src/theme';

const pileStyles = StyleSheet.create({
  deckBack: {
    width: 52,
    height: 72,
    backgroundColor: colors.accentDim,
    borderRadius: 6,
    borderWidth: 2,
    borderColor: colors.border,
    marginRight: 12,
    alignItems: 'center',
    justifyContent: 'center',
  },
  deckBackText: {
    color: '#fff',
    fontSize: 16,
    fontWeight: '700',
  },
  pileDisabled: {
    opacity: 0.4,
  },
  pressed: {
    opacity: 0.85,
  },
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

// Keeps the player's custom card order stable across draws/discards: cards
// still in hand keep their relative position, cards no longer in hand
// (discarded/melded) drop out, and newly received cards are appended.
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
  return [...kept, ...remaining];
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
  const [selectedForMeld, setSelectedForMeld] = useState<Set<number>>(new Set());
  const [localHand, setLocalHand] = useState<string[] | null>(null);
  const discardZoneRef = useRef<View>(null);
  const stagingZoneRef = useRef<View>(null);
  const meldViewRefs = useRef<Map<string, View>>(new Map());
  const overlayRootRef = useRef<View>(null);
  const overlayOriginX = useSharedValue(0);
  const overlayOriginY = useSharedValue(0);
  const dragPreview = useDragPreview();
  const [draggedCard, setDraggedCard] = useState<string | null>(null);
  const [discardFlash, setDiscardFlash] = useState(false);
  const prevTopDiscardRef = useRef<string | undefined>(undefined);
  const [justDrawnCard, setJustDrawnCard] = useState<string | null>(null);
  const [rulesModalOpen, setRulesModalOpen] = useState(false);
  const prevHandRef = useRef<{ game: number | undefined; hand: string[] }>({
    game: undefined,
    hand: [],
  });

  // Measured live at drag-end time (see HandRow) rather than cached from
  // scroll/layout events, so a drop always sees the current on-screen rect
  // even if the layout shifted since the last such event fired.
  function measureDropZone(cb: (zone: DropZone | null) => void) {
    if (!discardZoneRef.current) {
      cb(null);
      return;
    }
    discardZoneRef.current.measureInWindow((x, y, width, height) => cb({ x, y, width, height }));
  }

  function measureStagingZone(cb: (zone: DropZone | null) => void) {
    if (!canBuildMeld || !stagingZoneRef.current) {
      cb(null);
      return;
    }
    stagingZoneRef.current.measureInWindow((x, y, width, height) => cb({ x, y, width, height }));
  }

  function measureMeldZones(cb: (zones: { meldId: string; zone: DropZone }[]) => void) {
    const entries = canLayOff ? Array.from(meldViewRefs.current.entries()) : [];
    if (entries.length === 0) {
      cb([]);
      return;
    }
    const results: { meldId: string; zone: DropZone }[] = [];
    let remaining = entries.length;
    entries.forEach(([meldId, el]) => {
      el.measureInWindow((x, y, width, height) => {
        results.push({ meldId, zone: { x, y, width, height } });
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
      router.push('/game-end');
    },
    [setGameEnd],
  );

  const { state, status, connected, send, reconnect } = useGameSocket({
    gameId: id,
    onRoundEnd,
    onGameEnd,
  });

  const hand = localHand ?? state?.myHand ?? [];
  const userId = session?.userId ?? '';

  useEffect(() => {
    setLocalHand((prev) => reconcileHandOrder(prev, state?.myHand ?? []));
    setGroups([[]]);
    setSelectedForMeld(new Set());
  }, [state?.myHand, state?.phase, state?.game]);

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
    const top = state?.discardPile[state.discardPile.length - 1];
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

  const meldTargets = useMemo(() => {
    if (!state?.meldMeta) return [];
    const out: { meldId: string; label: string; owner: string }[] = [];
    let i = 0;
    const letters = 'abcdefghijklmnopqrstuvwxyz';
    for (const [owner, metas] of Object.entries(state.meldMeta)) {
      for (const meta of metas) {
        out.push({
          meldId: meta.meldId,
          label: letters[i] ?? String(i),
          owner,
        });
        i++;
      }
    }
    return out;
  }, [state?.meldMeta]);

  // Moves a card straight into the group currently being built — used by
  // drag-to-stage (dropping onto the staging area) and by "+ Add to meld"
  // for each selected card. Skips the select step entirely.
  function stageCard(index: number) {
    // Melding is only a thing between your draw and your discard — before
    // you've drawn there's nothing to meld with yet.
    if (!canBuildMeld) return;
    const card = hand[index];
    if (!card) return;
    setGroups((prev) => {
      const next = prev.map((g) => [...g]);
      next[next.length - 1].push(card);
      return next;
    });
    setSelectedForMeld((prev) => {
      if (!prev.has(index)) return prev;
      const next = new Set(prev);
      next.delete(index);
      return next;
    });
  }

  // Pulls a card back out of whichever staged group it's in (tapping a
  // staged card in the meld area) — the reverse of stageCard.
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

  // Tapping a hand card only selects it (gold ring) — it stays in the hand
  // row until "+ Add to meld" commits the selection. Without this guard a
  // tap would still visibly select the card even though no staging area
  // exists to act on it, which reads as melding being available when it
  // isn't.
  function toggleHandSelect(index: number) {
    if (!canBuildMeld) return;
    setSelectedForMeld((prev) => {
      const next = new Set(prev);
      if (next.has(index)) next.delete(index);
      else next.add(index);
      return next;
    });
  }

  // Commits every selected card into the group currently being built, in
  // hand order, in one go. If the last group already has cards in it, this
  // selection is a separate meld the player is building next — starting a
  // fresh group automatically (rather than merging into the last one)
  // avoids silently mixing two unrelated runs/sets together when the
  // player selects cards for a second meld without first tapping "+ Add
  // another run or set".
  function addSelectedToMeld() {
    if (selectedForMeld.size === 0) return;
    const indices = Array.from(selectedForMeld).sort((a, b) => a - b);
    const cards = indices.map((i) => hand[i]).filter((c): c is string => !!c);
    if (cards.length === 0) return;
    setGroups((prev) => {
      const next = prev.map((g) => [...g]);
      if (next[next.length - 1].length > 0) next.push([...cards]);
      else next[next.length - 1].push(...cards);
      return next;
    });
    setSelectedForMeld(new Set());
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
    send({ type: 'discard', card });
    clearSelect();
  }

  const canLayOff = !!state && isMyTurn;
  const canBuildMeld = !!state && isMyTurn && phase === 'meld';

  function layOffCardAt(index: number, meldId: string) {
    if (!canLayOff) return;
    const card = hand[index];
    if (!card) return;
    send({ type: 'lay_off', meldId, card });
    clearSelect();
  }

  if (!state) {
    return (
      <Screen title="Game">
        <Text style={shared.status}>{status || 'Loading game…'}</Text>
        <Pressable style={shared.button} onPress={reconnect}>
          <Text style={shared.buttonText}>Reconnect</Text>
        </Pressable>
        <Pressable
          style={[shared.button, shared.buttonSecondary, { marginTop: 8 }]}
          onPress={() => router.replace('/lobby/create')}
        >
          <Text style={shared.buttonTextSecondary}>Start a new game</Text>
        </Pressable>
      </Screen>
    );
  }

  const topDiscard = state.discardPile[state.discardPile.length - 1];
  const header = `${dealHeaderLabel(state.rulesProfile, state.game)} · Round ${state.round} · Deck ${state.deckCount}`;
  const turnLabel = isMyTurn
    ? 'Your turn'
    : (() => {
        const p = state.players.find((x) => x.id === state.currentTurn);
        return p ? `${p.name}'s turn` : 'Waiting…';
      })();

  const discardLocked = state.discardDrawMinRound > 1 && state.round < state.discardDrawMinRound;
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

  const actions: { label: string; onPress: () => void; disabled?: boolean }[] = [];

  // Always visible (grayed out rather than hidden) so it's obvious *why*
  // you can't draw right now — wrong phase, not your turn, or the
  // discard pile is locked — instead of the action just disappearing.
  actions.push({ label: 'Draw deck', onPress: drawFromDeck, disabled: !canDrawDeck });
  actions.push({ label: 'Take discard', onPress: takeDiscard, disabled: !canTakeDiscard });
  if (isMyTurn) {
    if (phase === 'meld') {
      const cards = selectedCards(hand, selectedForMeld);
      if (cards.length === 1 && meldTargets.length > 0 && state.roundReqMet[userId]) {
        meldTargets.slice(0, 6).forEach((m) => {
          actions.push({
            label: `Lay off on ${m.label}`,
            onPress: () => {
              send({ type: 'lay_off', meldId: m.meldId, card: cards[0] });
              clearSelect();
            },
          });
        });
      }
    }
    if (phase === 'discard') {
      const cards = selectedCards(hand, allStaged);
      if (cards.length === 1) {
        actions.push({
          label: 'Discard',
          onPress: () => {
            send({ type: 'discard', card: cards[0] });
            clearSelect();
          },
        });
      }
    }
  }

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
        <ScrollView>
          <View style={{ flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between' }}>
            <View style={{ flex: 1 }}>
              <Text style={{ color: colors.accent, fontSize: 12, fontWeight: '700' }}>
                {profileDisplayName(state.rulesProfile)}
              </Text>
              <Text style={shared.title}>{header}</Text>
            </View>
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
          {!connected ? ' · offline' : ''}
        </Text>
        {status ? <Text style={shared.error}>{status}</Text> : null}

        <Text style={[shared.status, { marginTop: 8 }]}>Opponents</Text>
        {state.players
          .filter((p) => p.id !== userId)
          .map((p) => (
            <Text key={p.id} style={{ color: colors.text }}>
              {p.name}: {state.cardCounts[p.id] ?? 0} cards
              {state.roundReqMet[p.id] ? ' ✓' : ''}
            </Text>
          ))}

        <MeldTable state={state} myUserId={userId} onMeldRef={registerMeldRef} />

        {canBuildMeld ? (
          <MeldStagingArea
            ref={stagingZoneRef}
            groups={stagingGroups}
            onRemove={unstageCard}
            onReorderGroup={reorderGroup}
            onCancelGroup={cancelGroup}
            onAddGroup={addGroup}
            canAddGroup={canAddGroup}
            onLayAll={layAllGroups}
            canLayAll={nonEmptyGroups.length > 0}
            layCount={nonEmptyGroups.length}
          />
        ) : null}

        {isMyTurn && state.discardDrawnCardPendingMeld ? (
          <Text style={[shared.error, { marginTop: 8 }]}>
            You picked up {state.discardDrawnCardPendingMeld} from the discard pile — it must go
            into your initial meld before you can discard.
          </Text>
        ) : null}

        <View ref={discardZoneRef} style={{ paddingVertical: 4 }}>
          <Text style={[shared.status, { marginTop: 8 }]}>
            Deck ({state.deckCount}) · Discard pile
            {discardLocked ? ` (pickup locked until round ${state.discardDrawMinRound})` : ''}
          </Text>
          <View style={{ flexDirection: 'row', alignItems: 'center' }}>
            <Pressable
              onPress={drawFromDeck}
              disabled={!canDrawDeck}
              style={({ pressed }) => [
                pileStyles.deckBack,
                !canDrawDeck && pileStyles.pileDisabled,
                pressed && canDrawDeck && pileStyles.pressed,
              ]}
            >
              <Text style={pileStyles.deckBackText}>{state.deckCount}</Text>
            </Pressable>
            {topDiscard ? (
              <View style={[pileStyles.discardWrap, discardFlash && pileStyles.discardWrapFlash]}>
                {discardFlash ? <Text style={pileStyles.discardFlashLabel}>✓ Added</Text> : null}
                <CardView card={topDiscard} onPress={canTakeDiscard ? takeDiscard : undefined} />
              </View>
            ) : (
              <Text style={shared.status}>Empty</Text>
            )}
          </View>
        </View>

        <View
          style={{
            flexDirection: 'row',
            justifyContent: 'space-between',
            alignItems: 'center',
            marginTop: 12,
          }}
        >
          <Text style={shared.status}>Your hand ({hand.length})</Text>
          <Pressable onPress={() => setLocalHand(autoOrganizeHand(hand))}>
            <Text style={{ color: colors.accent }}>Auto-organize</Text>
          </Pressable>
        </View>
        <Text style={shared.status}>
          Drag a card to reorder, onto the discard pile to discard, onto a table meld to lay off,
          or into the meld area to stage it directly. Or tap cards to select them, then tap "+ Add
          to meld" below. Drag a staged card within its run or set to reorder it. Lay as many
          melds as you like before discarding to end your turn.
        </Text>
        {canBuildMeld ? (
          <Pressable
            style={[shared.button, selectedForMeld.size === 0 && pileStyles.pileDisabled]}
            onPress={addSelectedToMeld}
            disabled={selectedForMeld.size === 0}
          >
            <Text style={shared.buttonText}>
              + Add to meld{selectedForMeld.size > 0 ? ` (${selectedForMeld.size})` : ''}
            </Text>
          </Pressable>
        ) : null}
        <HandRow
          cards={visibleHand}
          selected={visibleSelected}
          onToggle={(vi) => toggleHandSelect(visibleToFullIndex[vi])}
          onReorder={(newVisibleOrder) => {
            const stagedCards = hand.filter((_, i) => allStaged.has(i));
            setLocalHand([...newVisibleOrder, ...stagedCards]);
          }}
          onDoubleTap={(vi) => discardCardAt(visibleToFullIndex[vi])}
          measureDropZone={measureDropZone}
          onDropOnZone={(vi) => discardCardAt(visibleToFullIndex[vi])}
          measureMeldZones={measureMeldZones}
          onDropOnMeld={(vi, meldId) => layOffCardAt(visibleToFullIndex[vi], meldId)}
          measureStagingZone={measureStagingZone}
          onDropOnStaging={(vi) => stageCard(visibleToFullIndex[vi])}
          onDragCardChange={setDraggedCard}
          justDrawnCard={justDrawnCard}
          dragPreview={dragPreview}
          tapToDiscard={isMyTurn && phase === 'discard'}
        />

        {actions.length > 0 ? <ActionBar actions={actions} /> : null}

        {!connected ? (
          <View style={{ flexDirection: 'row', marginTop: 16, gap: 8 }}>
            <Pressable style={[shared.button, shared.buttonSecondary, { flex: 1 }]} onPress={reconnect}>
              <Text style={shared.buttonTextSecondary}>Reconnect</Text>
            </Pressable>
            <Pressable
              style={[shared.button, shared.buttonSecondary, { flex: 1 }]}
              onPress={() => router.replace('/lobby/create')}
            >
              <Text style={shared.buttonTextSecondary}>Start a new game</Text>
            </Pressable>
          </View>
        ) : null}
        </ScrollView>
      </Screen>
      <Animated.View style={dragOverlayStyle} pointerEvents="none">
        {draggedCard ? <CardView card={draggedCard} /> : null}
      </Animated.View>
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
