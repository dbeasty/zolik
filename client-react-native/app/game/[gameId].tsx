import { router, useLocalSearchParams } from 'expo-router';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';
import Animated, { useAnimatedStyle, useSharedValue } from 'react-native-reanimated';

import { ActionBar } from '@/src/components/ActionBar';
import { CardView } from '@/src/components/CardView';
import { HandRow, useDragPreview, type DropZone } from '@/src/components/HandRow';
import { MeldTable } from '@/src/components/MeldTable';
import { Screen } from '@/src/components/Screen';
import { useGameFlow } from '@/src/context/GameFlowContext';
import { useSession } from '@/src/context/SessionContext';
import { useGameSocket } from '@/src/hooks/useGameSocket';
import type { GameState, WSEnvelope } from '@/src/api/types';
import { autoOrganizeHand, roundRequirementLabel } from '@/src/lib/cards';
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
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [localHand, setLocalHand] = useState<string[] | null>(null);
  const discardZoneRef = useRef<View>(null);
  const meldViewRefs = useRef<Map<string, View>>(new Map());
  const overlayRootRef = useRef<View>(null);
  const overlayOriginX = useSharedValue(0);
  const overlayOriginY = useSharedValue(0);
  const dragPreview = useDragPreview();
  const [draggedCard, setDraggedCard] = useState<string | null>(null);
  const [discardFlash, setDiscardFlash] = useState(false);
  const prevTopDiscardRef = useRef<string | undefined>(undefined);

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
    setSelected(new Set());
  }, [state?.myHand, state?.phase, state?.round]);

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

  function toggleSelect(index: number) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(index)) next.delete(index);
      else next.add(index);
      return next;
    });
  }

  function clearSelect() {
    setSelected(new Set());
  }

  // Phase/round-requirement legality is left to the server (which replies
  // with a readable error surfaced via `status`) rather than silently
  // no-oping here — a card dropped on the discard pile or a meld should
  // always visibly do *something*, even if that's "not allowed right now".
  function discardCardAt(index: number) {
    if (!state || !isMyTurn || state.offer) return;
    const card = hand[index];
    if (!card) return;
    send({ type: 'discard', card });
    clearSelect();
  }

  const canLayOff = !!state && isMyTurn && !state.offer;

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
      </Screen>
    );
  }

  const topDiscard = state.discardPile[state.discardPile.length - 1];
  const header = `Round ${state.round}: ${roundRequirementLabel(state.round)} · Deck ${state.deckCount}`;
  const turnLabel = isMyTurn
    ? 'Your turn'
    : (() => {
        const p = state.players.find((x) => x.id === state.currentTurn);
        return p ? `${p.name}'s turn` : 'Waiting…';
      })();

  const discardLocked = state.discardDrawMinRound > 1 && state.round < state.discardDrawMinRound;
  const canDrawDeck = isMyTurn && !state.offer && phase === 'draw';
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

  const actions: { label: string; onPress: () => void; disabled?: boolean }[] = [];

  if (state.offer && isMyTurn) {
    actions.push(
      { label: 'Accept offer', onPress: () => send({ type: 'accept_offer' }) },
      { label: 'Decline', onPress: () => send({ type: 'decline_offer' }) },
    );
  } else {
    // Always visible (grayed out rather than hidden) so it's obvious *why*
    // you can't draw right now — wrong phase, not your turn, or the
    // discard pile is locked — instead of the action just disappearing.
    actions.push({ label: 'Draw deck', onPress: drawFromDeck, disabled: !canDrawDeck });
    actions.push({ label: 'Take discard', onPress: takeDiscard, disabled: !canTakeDiscard });
  }
  if (isMyTurn && !state.offer) {
    if (phase === 'meld') {
      const cards = selectedCards(hand, selected);
      if (cards.length >= 1) {
        actions.push({
          label: `Lay meld (${cards.length})`,
          onPress: () => {
            send({ type: 'lay_meld', cards });
            clearSelect();
          },
        });
      }
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
      const cards = selectedCards(hand, selected);
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
          <Text style={shared.title}>{header}</Text>
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
          Drag a card to reorder, onto the discard pile to discard, or onto a table meld to lay
          off. Select a bunch and tap Lay meld to start a new run or set.
        </Text>
        <HandRow
          cards={hand}
          selected={selected}
          onToggle={toggleSelect}
          onReorder={(newOrder) => setLocalHand(newOrder)}
          onDoubleTap={discardCardAt}
          measureDropZone={measureDropZone}
          onDropOnZone={discardCardAt}
          measureMeldZones={measureMeldZones}
          onDropOnMeld={layOffCardAt}
          onDragCardChange={setDraggedCard}
          dragPreview={dragPreview}
          tapToDiscard={isMyTurn && !state.offer && phase === 'discard'}
        />

        {actions.length > 0 ? <ActionBar actions={actions} /> : null}

        {!connected ? (
          <Pressable style={[shared.button, shared.buttonSecondary, { marginTop: 16 }]} onPress={reconnect}>
            <Text style={shared.buttonTextSecondary}>Reconnect</Text>
          </Pressable>
        ) : null}
        </ScrollView>
      </Screen>
      <Animated.View style={dragOverlayStyle} pointerEvents="none">
        {draggedCard ? <CardView card={draggedCard} /> : null}
      </Animated.View>
    </View>
  );
}
