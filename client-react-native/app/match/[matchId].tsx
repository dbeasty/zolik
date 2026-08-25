import { useLocalSearchParams } from 'expo-router';
import { useMemo, useRef, useState } from 'react';
import { Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';

import type { Zone } from '@/src/api/matchTypes';
import { POSITION_PARAM, offerGroupKey, submissionFor } from '@/src/api/matchTypes';
import { HandZone } from '@/src/components/match/HandZone';
import { OfferBar } from '@/src/components/match/OfferBar';
import { SeatStrip } from '@/src/components/match/SeatStrip';
import { ZoneView } from '@/src/components/match/ZoneView';
import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { useDropRegistry, type Measurable } from '@/src/hooks/useDropRegistry';
import { useHandOrder } from '@/src/hooks/useHandOrder';
import { useMatchSocket } from '@/src/hooks/useMatchSocket';
import { dropSpotsFor, groupElementId, positionAt, type DropSpot } from '@/src/lib/drops';
import { cardsForSelection, pruneSelection, slotsForDrag } from '@/src/lib/hand';
import { reasonText } from '@/src/lib/i18n';
import { factText, playerName } from '@/src/lib/labels';
import { colors } from '@/src/theme';

/**
 * One screen, every game.
 *
 * This is `architecture.md` §7.7's last untested claim: a shell that renders
 * zones and offer buttons should play any module with no new screen. It plays
 * Žolíky, Prší, Canasta and Texas Hold'em, and it will play the next one
 * without being edited.
 *
 * **The acceptance test is that this file contains no game's vocabulary.** Not
 * "no rummy logic" — no *mention* of a meld, a suit, a rank, a canasta, a
 * blind, a pot or a trick, anywhere. Everything on screen is something the
 * server said: zones to lay out, seats to draw, offers to press, and message
 * keys to look up. `e2e/tests/generic-shell.spec.ts` plays three different
 * games through it to prove the claim rather than assert it.
 *
 * It replaced a 1,756-line screen that played exactly one game. The difference
 * was not effort; it was that the rules had moved to the server, and a screen
 * that derives nothing needs far less of itself.
 */
export default function MatchScreen() {
  const { matchId } = useLocalSearchParams<{ matchId: string }>();
  const { session, client } = useSession();
  const [selected, setSelected] = useState<ReadonlySet<string>>(() => new Set());

  const url = useMemo(() => {
    if (!matchId || !session?.accessToken) return null;
    return client.matchSocketUrl(String(matchId));
  }, [client, matchId, session?.accessToken]);

  const { state, error, connected, send, clearError } = useMatchSocket(url);
  const viewerId = session?.userId ?? '';

  const view = state?.view ?? { zones: [] };
  const zones = view.zones ?? [];

  // The one piece of layout judgement in the file, and it is about *ownership*
  // rather than about any game: your own cards go at the bottom where your
  // thumb is, everyone else's go up top. Which zone is yours is a field the
  // server sets.
  const mine = zones.filter((z) => z.ownerId === viewerId);
  const shared = zones.filter((z) => !z.ownerId);
  const others = zones.filter((z) => z.ownerId && z.ownerId !== viewerId);

  // A hand is the only zone anyone may rearrange, and `hand` is a kind every
  // module already declares — so this reaches all four games without naming
  // one. Computed before the connecting-early-return below, because it feeds
  // a hook and hooks may not be conditional.
  const myHands = mine.filter((z) => z.kind === 'hand');
  // Keyed by match, so an arrangement is remembered across a reload but never
  // carried into a different deal, where it would mean nothing.
  const { slotsFor, move } = useHandOrder(myHands, matchId ? String(matchId) : undefined);

  const heldSlots = myHands.flatMap((z) => slotsFor(z.id));

  // Dragging a card somewhere. `drag` renders the highlights; `dragRef` is the
  // same thing readable synchronously, because the first pointer move can
  // arrive before the state that started the drag has been committed, and a
  // drag whose first frames land nowhere reads as an unresponsive one.
  const drops = useDropRegistry();
  const dragRef = useRef<{ slotIds: string[]; cards: string[] } | null>(null);
  const [drag, setDrag] = useState<{ cards: string[] } | null>(null);
  const [hoveredDrop, setHoveredDrop] = useState<string | null>(null);
  // A folded control in the offer bar was pressed but the current selection
  // does not say which of its targets was meant. Rather than guess, the
  // targets it could still mean light up the same way a drag lights them up,
  // and a press on one finishes the job a drag would have — see
  // `onAmbiguous`/`pressDrop` below.
  const [pendingGroupKey, setPendingGroupKey] = useState<string | null>(null);
  // Whether a tap on the status dot has opened its explanation — declared
  // before the `!state` early return below so hook order stays fixed
  // whether or not the socket has delivered a state yet.
  const [statusExplainerOpen, setStatusExplainerOpen] = useState(false);

  if (!state) {
    return (
      <Screen>
        <Text testID="match-connecting" style={styles.muted}>
          {connected ? 'Waiting for the table…' : 'Connecting…'}
        </Text>
      </Screen>
    );
  }

  // Selection is by *slot*, not by card string. With two decks in play a hand
  // can hold two identical strings, and selecting by string could neither
  // light up the copy that was tapped nor put both of them in one meld.
  const toggleSlot = (slotId: string) => {
    // Whatever a folded control's press was waiting on was computed for the
    // selection as it stood a moment ago; changing it here means starting
    // that choice over rather than resolving it against a selection that has
    // since moved on.
    setPendingGroupKey(null);
    setSelected((prev) => {
      const next = pruneSelection(heldSlots, prev);
      if (next.has(slotId)) next.delete(slotId);
      else next.add(slotId);
      return next;
    });
  };

  const selectedCards = cardsForSelection(heldSlots, selected);

  const canAct = state.legalActions.some((o) => o.enabled);

  // Everywhere the cards in flight could be let go of. Derived from the offer
  // list on every drag, which is why a game added tomorrow gets drag and drop
  // without this screen being edited: an offer that says which cards it takes
  // and where it lands *is* a drop target.
  const spotsFor = (cards: string[]) => dropSpotsFor(state.legalActions, cards);
  const liveSpots = drag ? spotsFor(drag.cards) : [];

  // The same lookup, aimed at a folded control's own offers instead of a card
  // in flight — recomputed from the live offer list on every render rather
  // than captured at the moment of the press, so a target that stopped being
  // legal mid-choice simply stops lighting up instead of a stale one staying
  // pressable.
  const pendingOffers = pendingGroupKey
    ? state.legalActions.filter((o) => o.enabled && o.target?.meldId && offerGroupKey(o) === pendingGroupKey)
    : [];
  // `dropSpotsFor` answers "where may *these cards* be let go", which is
  // exactly wrong when nothing is selected — every one of a folded control's
  // enabled, already-one-tap-ready offers is still a live target then, cards
  // or no cards, the same as a lone offer of the same shape would be.
  const pendingSpots: DropSpot[] =
    selectedCards.length > 0
      ? dropSpotsFor(pendingOffers, selectedCards)
      : pendingOffers.flatMap((o) =>
          o.target?.meldId ? [{ offerId: o.id, elementId: groupElementId(o.target.meldId), ready: true }] : [],
        );

  const activeDrops = new Set([...liveSpots, ...pendingSpots].map((s) => s.elementId));
  const pressableDrops = new Set(pendingSpots.map((s) => s.elementId));

  const beginDrag = (zoneId: string, index: number) => {
    const carried = slotsForDrag(slotsFor(zoneId), selected, index);
    if (!carried.length) return;

    const cards = carried.map((s) => s.card);
    dragRef.current = { slotIds: carried.map((s) => s.id), cards };
    setDrag({ cards });
    // The board is inside a scroll view, so where a meld was during the last
    // drag is not where it is now.
    drops.measure();
  };

  const moveDrag = (x: number, y: number) => {
    const current = dragRef.current;
    if (!current) return;
    const over = drops.hit(x, y, spotsFor(current.cards).map((s) => s.elementId));
    setHoveredDrop((prev) => (prev === over ? prev : over));
  };

  const endDrag = (x: number, y: number): boolean => {
    const current = dragRef.current;
    dragRef.current = null;
    setDrag(null);
    setHoveredDrop(null);
    if (!current) return false;

    const spots = spotsFor(current.cards);
    const over = drops.hit(x, y, spots.map((s) => s.elementId));
    const spot = spots.find((s) => s.elementId === over);
    if (!spot) return false;

    const offer = state.legalActions.find((o) => o.id === spot.offerId);
    if (!offer) return false;

    // Dropped somewhere that wants more cards than are in flight — a rummy
    // meld needs three. Keep them rather than sending a fragment the server
    // would refuse: they stay selected, so dropping the next one adds to them
    // and the offer's own button lights up when it has enough.
    if (!spot.ready) {
      setSelected((prev) => new Set([...prev, ...current.slotIds]));
      return true;
    }

    const action = submissionFor(offer, { cards: current.cards });
    if (!action) return false;

    // Which end of a run this landed on, when the offer said there was a
    // choice. Decided by where in the target the pointer was, which is the
    // one thing only the gesture knows.
    const position = positionAt(spot.positions, x, drops.rectFor(over!) ?? { x: 0, width: 0 });
    if (position) action.params = { ...(action.params ?? {}), [POSITION_PARAM]: position };

    send(action);
    setSelected(new Set());
    return true;
  };

  // The press equivalent of `endDrag`, for a target lit up by `pendingSpots`
  // rather than by a card in flight. `pageX` stands in for the gesture's own
  // x — the one thing a tap still supplies that a plain press otherwise
  // would not — so a target with a choice of two positions reads a press on
  // its left half the same way it would read a drop there.
  const pressDrop = (elementId: string, pageX: number) => {
    const spot = pendingSpots.find((s) => s.elementId === elementId);
    if (!spot) return;
    const offer = state.legalActions.find((o) => o.id === spot.offerId);
    if (!offer) return;

    // An empty selection has to travel as `undefined`, not `[]` — an offer
    // built with no cards named falls back to its own one-tap default, the
    // same way a bare press on a lone offer of the same shape already does;
    // an empty array is a *chosen* zero, and settles on nothing at all.
    const action = submissionFor(offer, { cards: selectedCards.length ? selectedCards : undefined });
    if (!action) return;

    const rect = drops.rectFor(elementId);
    const position = rect ? positionAt(spot.positions, pageX, rect) : spot.positions?.[0];
    if (position) action.params = { ...(action.params ?? {}), [POSITION_PARAM]: position };

    send(action);
    setSelected(new Set());
    setPendingGroupKey(null);
  };

  const dropProps = {
    registerDrop: (id: string, node: Measurable | null) => drops.register(id, node),
    activeDrops,
    hoveredDrop,
    pressableDrops,
    onPressDrop: pressDrop,
  };

  // What the status dot means, in the same words the line it replaced used
  // to say. Red is the one case a player needs to notice — everything else
  // (active, completed) is green, since "simple red or green" was the ask,
  // not a status per state value.
  const statusOk = state.status !== 'suspended';
  const statusExplainer =
    state.status === 'suspended'
      ? `Paused — waiting for ${playerName(state.players, state.suspendedPlayer ?? '')} to reconnect.`
      : state.status === 'completed'
        ? 'This match has finished.'
        : 'Match in progress — everything is connected and moving normally.';

  return (
    <Screen>
      <ScrollView contentContainerStyle={styles.body} testID="match-screen">
        <View style={styles.headerRow}>
          <Text testID="match-module" style={styles.module}>
            {state.moduleId}
            {state.variation ? ` · ${state.variation}` : ''}
          </Text>
          {/* Text and dot as one unit — the colour is the at-a-glance signal,
              the word next to it is the same fact spelled out, and a tap gets
              the fuller explanation the old standalone status line gave, on
              demand instead of by surprise. Always mounted here rather than
              inside the (optional) header-facts row below, so it never
              disappears when a module sends no header facts. */}
          <Pressable
            testID="match-status-dot"
            onPress={() => setStatusExplainerOpen((v) => !v)}
            hitSlop={8}
            style={styles.statusGroup}
          >
            <Text testID="match-status" style={styles.status}>
              {state.status}
            </Text>
            <View style={[styles.statusDot, statusOk ? styles.statusDotOk : styles.statusDotBad]} />
          </Pressable>
        </View>

        {(view.header ?? []).length > 0 ? (
          <View style={styles.facts} testID="match-header">
            {(view.header ?? []).map((f, i) => (
              <Text key={`${f.labelKey}-${i}`} style={styles.fact}>
                {factText(f)}
              </Text>
            ))}
          </View>
        ) : null}
        {statusExplainerOpen ? (
          <Text testID="match-status-explainer" style={styles.statusExplainer}>
            {statusExplainer}
          </Text>
        ) : null}

        <SeatStrip
          seats={view.seats ?? []}
          players={state.players}
          viewerId={viewerId}
          standings={state.standings}
        />

        {(view.prompts ?? []).map((f, i) => (
          <Text key={`prompt-${i}`} testID={`prompt-${i}`} style={styles.prompt}>
            {factText(f)}
          </Text>
        ))}

        {/* Buttons sit beside the piles rather than below them, sharing that
            band of the screen instead of costing it a full-width row of
            their own — OfferBar already scrolls horizontally, so a narrow
            column here just means an earlier scroll, never a clipped or
            wrapped button. */}
        <View style={styles.tableRow}>
          <Section title="Table" zones={shared} {...dropProps} />
          <View style={styles.offerBarWrap}>
            {error ? (
              <Text testID="match-error" style={styles.error} onPress={clearError}>
                {reasonText(error.code, error.code)}
              </Text>
            ) : null}
            {/* Disabled offers stay on screen with their reason. An offer
                that vanished when it became illegal would be
                indistinguishable from a bug, which is why the server sends
                the whole set every time. */}
            <OfferBar
              offers={state.legalActions}
              selectedCards={selectedCards}
              onSend={send}
              onConsumeSelection={() => setSelected(new Set())}
              onAmbiguous={(groupKey) => {
                setPendingGroupKey(groupKey);
                // The board is inside a scroll view, so a target's position
                // as of the last drag is not necessarily where it is now
                // either.
                drops.measure();
              }}
            />
            {!canAct && state.status === 'active' ? (
              <Text testID="match-waiting" style={styles.muted}>
                Waiting for another player…
              </Text>
            ) : null}
          </View>
        </View>

        {/* Your hand first — it's what your thumb is on every turn — with
            everyone's melds (yours and the opponents') below it rather than
            above, so reaching your cards never means scrolling past a wall
            of board state first. */}
        <View style={styles.mine}>
          {myHands.map((z) => (
            <HandZone
              key={z.id}
              zone={z}
              slots={slotsFor(z.id)}
              selected={selected}
              onToggle={toggleSlot}
              onMove={(from, to) => move(z.id, from, to)}
              onDragStart={(index) => beginDrag(z.id, index)}
              onDragMove={moveDrag}
              onDragEnd={endDrag}
              externalTarget={hoveredDrop}
            />
          ))}
        </View>

        <View style={styles.mine}>
          {mine
            .filter((z) => z.kind !== 'hand')
            .map((z) => (
              <ZoneView key={z.id} zone={z} {...dropProps} />
            ))}
        </View>

        <Section title="Opponents" zones={others} compact {...dropProps} />

        {(view.status ?? []).map((f, i) => (
          <Text key={`status-${i}`} testID={`status-${i}`} style={styles.muted}>
            {factText(f)}
          </Text>
        ))}
      </ScrollView>
    </Screen>
  );
}

function Section({
  title,
  zones,
  compact,
  ...drops
}: {
  title: string;
  zones: Zone[];
  compact?: boolean;
  registerDrop?: (id: string, node: Measurable | null) => void;
  activeDrops?: ReadonlySet<string>;
  hoveredDrop?: string | null;
  pressableDrops?: ReadonlySet<string>;
  onPressDrop?: (elementId: string, pageX: number) => void;
}) {
  if (!zones.length) return null;

  // Side by side if a zone is small, on its own line if it is wide — decided
  // by kind, which is the one thing the shell is allowed to know. A stack and
  // a pile are a couple of cards across and look absurd each occupying a full
  // row; a hand or a spread of melds needs the width. No game is named, so a
  // game added tomorrow is laid out by the same rule.
  const beside = zones.filter((z) => z.kind === 'stack' || z.kind === 'pile');
  const stacked = zones.filter((z) => z.kind !== 'stack' && z.kind !== 'pile');

  return (
    <View style={styles.section}>
      <Text style={styles.sectionTitle}>{title}</Text>
      {beside.length > 0 ? (
        <View style={styles.beside} testID={`section-beside-${title.toLowerCase()}`}>
          {beside.map((z) => (
            <ZoneView key={z.id} zone={z} compact={compact} inline {...drops} />
          ))}
        </View>
      ) : null}
      {stacked.map((z) => (
        <ZoneView key={z.id} zone={z} compact={compact} {...drops} />
      ))}
    </View>
  );
}

const styles = StyleSheet.create({
  body: { paddingBottom: 40, gap: 4 },
  headerRow: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center' },
  module: { color: colors.text, fontWeight: '700', fontSize: 16 },
  statusGroup: { flexDirection: 'row', alignItems: 'center', gap: 6 },
  status: { color: colors.muted, fontSize: 12 },
  facts: { flexDirection: 'row', flexWrap: 'wrap', gap: 10, marginTop: 2 },
  tableRow: { flexDirection: 'row', alignItems: 'flex-start', gap: 8 },
  // minWidth: 0 lets OfferBar's horizontal ScrollView actually shrink to
  // whatever room is left beside the piles instead of overflowing its row —
  // a flex child's default min-width is its content width, which for a
  // scrollable row of buttons is unbounded.
  offerBarWrap: { flex: 1, minWidth: 0, marginTop: 18 },
  fact: { color: colors.muted, fontSize: 12 },
  statusDot: { width: 10, height: 10, borderRadius: 5, marginTop: 1 },
  statusDotOk: { backgroundColor: colors.success },
  statusDotBad: { backgroundColor: colors.danger },
  statusExplainer: { color: colors.muted, fontSize: 12, marginTop: 4 },
  prompt: { color: colors.gold, fontSize: 13, marginTop: 6 },
  section: { marginTop: 10 },
  beside: { flexDirection: 'row', flexWrap: 'wrap', alignItems: 'flex-start', gap: 8, marginBottom: 8 },
  sectionTitle: { color: colors.muted, fontSize: 11, fontWeight: '700', marginBottom: 4 },
  mine: { marginTop: 10 },
  error: { color: colors.danger, fontSize: 13, marginVertical: 6 },
  muted: { color: colors.muted, fontSize: 12, marginTop: 6 },
});
