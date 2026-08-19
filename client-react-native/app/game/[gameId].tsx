import { router, useLocalSearchParams } from 'expo-router';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { Pressable, ScrollView, Text, View } from 'react-native';

import { ActionBar } from '@/src/components/ActionBar';
import { CardView } from '@/src/components/CardView';
import { HandRow } from '@/src/components/HandRow';
import { MeldTable } from '@/src/components/MeldTable';
import { Screen } from '@/src/components/Screen';
import { useGameFlow } from '@/src/context/GameFlowContext';
import { useSession } from '@/src/context/SessionContext';
import { useGameSocket } from '@/src/hooks/useGameSocket';
import type { GameState, WSEnvelope } from '@/src/api/types';
import { autoOrganizeHand, roundRequirementLabel } from '@/src/lib/cards';
import { colors, shared } from '@/src/theme';

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

  function handleDoubleTapDiscard(index: number) {
    if (!state || !isMyTurn || state.offer) return;
    if (phase !== 'discard' && phase !== 'meld') return;
    const card = hand[index];
    if (!card) return;
    send({ type: 'discard', card });
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

  const actions: { label: string; onPress: () => void; disabled?: boolean }[] = [];

  if (state.offer && isMyTurn) {
    actions.push(
      { label: 'Accept offer', onPress: () => send({ type: 'accept_offer' }) },
      { label: 'Decline', onPress: () => send({ type: 'decline_offer' }) },
    );
  } else if (isMyTurn) {
    if (phase === 'draw') {
      actions.push({
        label: 'Draw deck',
        onPress: () => {
          send({ type: 'draw_card', from: 'deck' });
          clearSelect();
        },
      });
      const discardLocked =
        state.discardDrawMinRound > 1 && state.round < state.discardDrawMinRound;
      if (!discardLocked) {
        actions.push({
          label: 'Take discard',
          onPress: () => {
            send({ type: 'draw_card', from: 'discard' });
            clearSelect();
          },
        });
      }
    }
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
      if (cards.length === 1 && meldTargets.length > 0) {
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

        <MeldTable state={state} myUserId={userId} />

        <Text style={[shared.status, { marginTop: 8 }]}>
          Discard pile
          {state.discardDrawMinRound > 1 && state.round < state.discardDrawMinRound
            ? ` (pickup locked until round ${state.discardDrawMinRound})`
            : ''}
        </Text>
        <View style={{ flexDirection: 'row', alignItems: 'center' }}>
          {topDiscard ? <CardView card={topDiscard} /> : <Text style={shared.status}>Empty</Text>}
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
        <Text style={shared.status}>Drag a card to reorder your hand.</Text>
        <HandRow
          cards={hand}
          selected={selected}
          onToggle={toggleSelect}
          onReorder={(newOrder) => setLocalHand(newOrder)}
          onDoubleTap={handleDoubleTapDiscard}
        />
        <Text style={shared.status}>Double-tap a card to discard it.</Text>

        {actions.length > 0 ? <ActionBar actions={actions} /> : null}

        {!connected ? (
          <Pressable style={[shared.button, shared.buttonSecondary, { marginTop: 16 }]} onPress={reconnect}>
            <Text style={shared.buttonTextSecondary}>Reconnect</Text>
          </Pressable>
        ) : null}
      </ScrollView>
    </Screen>
  );
}
