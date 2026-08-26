import { router, useLocalSearchParams } from 'expo-router';
import { useEffect, useMemo, useRef, useState } from 'react';
import { Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';

import type { Zone } from '@/src/api/matchTypes';
import { POSITION_PARAM, offerGroupKey, submissionFor } from '@/src/api/matchTypes';
import { HandZone } from '@/src/components/match/HandZone';
import { LifetimeRecord } from '@/src/components/match/LifetimeRecord';
import { OfferBar, OfferGlance } from '@/src/components/match/OfferBar';
import { Panel } from '@/src/components/match/Panel';
import { RoundResults } from '@/src/components/match/RoundResults';
import { SeatStrip } from '@/src/components/match/SeatStrip';
import { ZoneView } from '@/src/components/match/ZoneView';
import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { useDropRegistry, type Measurable } from '@/src/hooks/useDropRegistry';
import { useHandOrder } from '@/src/hooks/useHandOrder';
import { useMatchSocket } from '@/src/hooks/useMatchSocket';
import { usePanelState } from '@/src/hooks/usePanelState';
import { drawableZones } from '@/src/lib/board';
import { dropSpotsFor, groupElementId, positionAt, someOfferReady, type DropSpot } from '@/src/lib/drops';
import { cardsForSelection, slotsForDrag, toggleSelection } from '@/src/lib/hand';
import { reasonText } from '@/src/lib/i18n';
import { factText, label, playerName } from '@/src/lib/labels';
import { colors, dragLayer } from '@/src/theme';

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
 *
 * Every region below is drawn inside a `Panel`, which is what gives a player
 * a minimize control on each one for free, and what makes a phone-width
 * layout — controls that wrap onto more than one line, a spread named by
 * whose it is instead of a bare "Melds", nothing drawn for a hand nobody can
 * see — one change to the pieces this file already assembles rather than a
 * second screen.
 */
export default function MatchScreen() {
  const { matchId } = useLocalSearchParams<{ matchId: string }>();
  const { session, client } = useSession();
  const [selected, setSelected] = useState<ReadonlySet<string>>(() => new Set());
  // Whether what is currently selected was picked by the *app* rather than by
  // the player — see the auto-select effect below and `toggleSlot`.
  const [selectionIsAuto, setSelectionIsAuto] = useState(false);
  const panels = usePanelState(matchId ? String(matchId) : undefined);

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

  // A hand is the only zone anyone may rearrange, and `hand` is a kind every
  // module already declares — so this reaches all four games without naming
  // one. Computed before the connecting-early-return below, because it feeds
  // a hook and hooks may not be conditional.
  const myHands = mine.filter((z) => z.kind === 'hand');
  // Keyed by match, so an arrangement is remembered across a reload but never
  // carried into a different deal, where it would mean nothing.
  const { slotsFor, move, arrange, autoSelectIds } = useHandOrder(myHands, matchId ? String(matchId) : undefined);

  const heldSlots = myHands.flatMap((z) => slotsFor(z.id));

  // A card that just arrived in hand, with nothing else about it changing, is
  // the one thing in the fan a player didn't have a moment ago — see
  // `justArrived` in `src/lib/hand.ts` for exactly what that does and doesn't
  // cover. Landing it selected means whatever's about to leave the hand is
  // already picked, rather than making a player go find it among a dozen
  // others first.
  //
  // Flagged as the app's pick rather than the player's, which is what lets
  // the next card they touch replace it instead of joining it — see
  // `toggleSlot`.
  useEffect(() => {
    if (autoSelectIds.length) {
      setSelected(new Set(autoSelectIds));
      setSelectionIsAuto(true);
    }
  }, [autoSelectIds]);

  // Dragging a card somewhere. `drag` renders the highlights; `dragRef` is the
  // same thing readable synchronously, because the first pointer move can
  // arrive before the state that started the drag has been committed, and a
  // drag whose first frames land nowhere reads as an unresponsive one.
  const drops = useDropRegistry();
  const dragRef = useRef<{ slotIds: string[]; cards: string[] } | null>(null);
  const [drag, setDrag] = useState<{ cards: string[] } | null>(null);
  const [hoveredDrop, setHoveredDrop] = useState<string | null>(null);
  // Which of `hoveredDrop`'s ordered positions the drag is currently over,
  // for a target with a choice of more than one — a card carried over a run
  // says up front which end it would extend, rather than only after it is
  // let go of. An index into the offer's own `positions` list, not the
  // position's name, so the shell that renders it (`ZoneView`) never has to
  // know what "front" or "end" means — only which of N ordered slices this
  // is, the same "first is drawn first" contract `positions` already keeps.
  const [hoveredPosition, setHoveredPosition] = useState<{ index: number; count: number } | null>(null);
  // A folded control in the offer bar was pressed but the current selection
  // does not say which of its targets was meant. Rather than guess, the
  // targets it could still mean light up the same way a drag lights them up,
  // and a press on one finishes the job a drag would have — see
  // `onAmbiguous`/`pressDrop` below.
  const [pendingGroupKey, setPendingGroupKey] = useState<string | null>(null);
  // A meld tapped before any card was picked — `target.meldId`, the same
  // vocabulary an offer already uses, not an element id. This is the other
  // order a move can be made in: `pendingGroupKey` above narrows the board
  // once a *control* said which kind of move was meant; this narrows it once
  // the *target* was pointed at first, before any control or any card. See
  // `onAimGroup` below for how it is set and cleared.
  const [armedMeldId, setArmedMeldId] = useState<string | null>(null);
  // Whether a tap on the status dot has opened its explanation — declared
  // before the `!state` early return below so hook order stays fixed
  // whether or not the socket has delivered a state yet.
  const [statusExplainerOpen, setStatusExplainerOpen] = useState(false);
  // Setting up the same table again, and whatever went wrong trying — declared
  // up here for the same reason: hooks may not be conditional.
  const [startingAgain, setStartingAgain] = useState(false);
  const [againError, setAgainError] = useState('');

  if (!state) {
    return (
      <Screen>
        <Text testID="match-connecting" style={styles.muted}>
          {connected ? 'Waiting for the table…' : 'Connecting…'}
        </Text>
      </Screen>
    );
  }

  // Nothing selected, and nothing provisionally selected either — a fresh
  // start for whatever the player does next. Used everywhere a selection is
  // spent, so the auto-pick flag can never outlive the cards it described.
  // Spending a selection also disarms whatever target was pointed at: a move
  // just went to it, so aiming has done its job.
  const clearSelection = () => {
    setSelected(new Set());
    setSelectionIsAuto(false);
    setArmedMeldId(null);
  };

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
      // `provisional` is what makes a tap on some *other* card replace the
      // app's own pick rather than join it — see `toggleSelection` — but
      // only when joining goes nowhere. A multi-card lay-off is exactly a
      // second tap meant to join the drawn card, not replace it, so try
      // joining first and fall back to replacing only when no offer would
      // take the two together.
      if (selectionIsAuto && !prev.has(slotId)) {
        const joined = toggleSelection(heldSlots, prev, slotId, { provisional: false });
        if (someOfferReady(state.legalActions, cardsForSelection(heldSlots, joined))) return joined;
      }
      return toggleSelection(heldSlots, prev, slotId, { provisional: selectionIsAuto });
    });
    // Whatever that did, the selection is now the player's own, so further
    // taps accumulate normally — that is how several cards are gathered into
    // one meld.
    setSelectionIsAuto(false);
  };

  const selectedCards = cardsForSelection(heldSlots, selected);

  const canAct = state.legalActions.some((o) => o.enabled);

  // Everywhere the cards in flight could be let go of. Derived from the offer
  // list on every drag, which is why a game added tomorrow gets drag and drop
  // without this screen being edited: an offer that says which cards it takes
  // and where it lands *is* a drop target.
  const spotsFor = (cards: string[]) => dropSpotsFor(state.legalActions, cards);
  const liveSpots = drag ? spotsFor(drag.cards) : [];

  // A card in hand is a card looking for somewhere to go. Every enabled offer
  // that would take the current selection is a live target — the same set a
  // drag would light up, offered to a tap as well: dragging a card across a
  // scrolling board to reach a meld below the fold is a poor way to play on a
  // phone, and it is the whole reason an opponent's meld could look
  // unreachable even though the offer to lay off onto it was right there.
  //
  // `pendingGroupKey`, when set, narrows this to one folded control's own
  // targets — pressing it said which *kind* of move was meant, and only
  // which target is still open (see `onAmbiguous` below). Unset — a card was
  // simply tapped, no control pressed — every enabled offer is a candidate,
  // recomputed on every render rather than captured at selection time, so a
  // target that stopped being legal mid-choice simply stops lighting up
  // instead of a stale one staying pressable.
  //
  // `armedMeldId` narrows the same way from the other order: a meld tapped
  // *before* any card was picked, rather than a control pressed after. A
  // target that stopped offering anything simply stops narrowing — computed
  // fresh each render against the live offer list rather than trusted from
  // whenever it was tapped, so a meld played away by a bot while armed does
  // not leave the screen pointed at nothing.
  const meldsWithOffers = new Set(
    state.legalActions
      .filter((o) => o.enabled && o.target?.meldId && (o.source?.minCards ?? 0) > 0)
      .map((o) => o.target!.meldId!),
  );
  const armedMeldIdLive = armedMeldId && meldsWithOffers.has(armedMeldId) ? armedMeldId : null;
  const pendingCandidates = state.legalActions.filter(
    (o) =>
      o.enabled &&
      (!pendingGroupKey || (o.target?.meldId && offerGroupKey(o) === pendingGroupKey)) &&
      (!armedMeldIdLive || o.target?.meldId === armedMeldIdLive),
  );
  // `dropSpotsFor` answers "where may *these cards* be let go", which is
  // exactly wrong when nothing is selected — every one of a folded control's
  // enabled, already-one-tap-ready offers is still a live target then, cards
  // or no cards, the same as a lone offer of the same shape would be. With
  // nothing selected and no folded control pressed, nothing is pending.
  const pendingSpots: DropSpot[] =
    selectedCards.length > 0
      ? dropSpotsFor(pendingCandidates, selectedCards)
      : pendingGroupKey
        ? pendingCandidates.flatMap((o) =>
            o.target?.meldId ? [{ offerId: o.id, elementId: groupElementId(o.target.meldId), ready: true }] : [],
          )
        : [];

  const activeDrops = new Set([...liveSpots, ...pendingSpots].map((s) => s.elementId));
  const pressableDrops = new Set(pendingSpots.map((s) => s.elementId));

  // Which melds could be *aimed at* right now — pointed at before any card is
  // picked, the other order from the usual "select cards, then a target
  // lights up". Only offered while nothing is selected: once cards are
  // chosen, a fitting target is already live and pressable above, and aiming
  // has nothing left to add. Tapping one arms it (see `onAimGroup`); tapping
  // the armed one again clears it.
  const armableGroups = selectedCards.length === 0 ? meldsWithOffers : new Set<string>();
  const onAimGroup = (meldId: string) => {
    if (!meldsWithOffers.has(meldId)) return;
    setArmedMeldId((prev) => (prev === meldId ? null : meldId));
  };

  // What's worth putting on screen at all — a hidden zone with a count and no
  // cards says nothing the seat strip hasn't already said, unless it's the
  // viewer's own, or a target the card in flight could land on right now.
  const visible = drawableZones(zones, viewerId, activeDrops);

  // Every spread, whoever's it is, in one row — a rummy meld, a canasta
  // partnership's melds, a poker board. Kept apart from the piles and stacks
  // below only by `kind`, never by whose it is or which game sent it.
  const spreadZones = visible.filter((z) => z.kind === 'spread');
  const mySpreads = spreadZones.filter((z) => z.ownerId === viewerId);
  const otherSpreads = spreadZones.filter((z) => z.ownerId !== viewerId);
  const orderedSpreads = [...mySpreads, ...otherSpreads];

  const tableZones = visible.filter((z) => !z.ownerId && z.kind !== 'spread');
  // Whatever is left: an opponent zone that isn't a spread — a hand revealed
  // at a showdown, say — or a kind this shell has never seen. The fallback
  // that keeps a game this screen wasn't written against from losing content
  // silently.
  const otherZones = visible.filter((z) => z.ownerId && z.ownerId !== viewerId && z.kind !== 'spread');

  const beginDrag = (zoneId: string, index: number) => {
    const slots = slotsFor(zoneId);
    // Picking a card up is as much "I mean this one" as tapping it, so the
    // same rule applies as `toggleSlot`: a provisional selection the player
    // didn't make joins the card just picked up when some offer would take
    // the two together — a multi-card lay-off dragged straight off the
    // drawn card — and is otherwise dropped, the same as before, rather
    // than surviving the drag and getting merged back in by the staging
    // branch of `endDrag`, putting a card they never chose into the next
    // thing they try to play.
    const picked = slots[index];
    let dragSelection = selected;
    if (picked && selectionIsAuto && !selected.has(picked.id)) {
      const joined = new Set([...selected, picked.id]);
      dragSelection = someOfferReady(state.legalActions, cardsForSelection(heldSlots, joined))
        ? joined
        : new Set<string>();
      setSelected(dragSelection);
    }
    setSelectionIsAuto(false);

    const carried = slotsForDrag(slots, dragSelection, index);
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
    const spots = spotsFor(current.cards);
    const over = drops.hit(x, y, spots.map((s) => s.elementId));
    setHoveredDrop((prev) => (prev === over ? prev : over));

    // Which of the target's ordered positions this hover currently means,
    // shown live so a player can see where a card will land before letting
    // go of it, rather than finding out only after. `null` for a target with
    // no choice to preview (one legal position, or none), or nothing hovered.
    const spot = over ? spots.find((s) => s.elementId === over) : undefined;
    const rect = over ? drops.rectFor(over) : undefined;
    const positions = spot?.positions;
    const resolved = spot && rect ? positionAt(positions, y, rect) : undefined;
    const index = resolved && positions ? positions.indexOf(resolved) : -1;
    const next = positions && positions.length > 1 && index >= 0 ? { index, count: positions.length } : null;
    setHoveredPosition((prev) =>
      prev?.index === next?.index && prev?.count === next?.count ? prev : next,
    );
  };

  const endDrag = (x: number, y: number): boolean => {
    const current = dragRef.current;
    dragRef.current = null;
    setDrag(null);
    setHoveredDrop(null);
    setHoveredPosition(null);
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
    // one thing only the gesture knows — vertically, because a group is
    // drawn as a stack overlapping top to bottom (see `positionAt`), so this
    // is the same axis `moveDrag` was already previewing above.
    const position = positionAt(spot.positions, y, drops.rectFor(over!) ?? { y: 0, height: 0 });
    if (position) action.params = { ...(action.params ?? {}), [POSITION_PARAM]: position };

    send(action);
    clearSelection();
    return true;
  };

  // The press equivalent of `endDrag`, for a target lit up by `pendingSpots`
  // rather than by a card in flight. `pageY` stands in for the gesture's own
  // y — the one thing a tap still supplies that a plain press otherwise
  // would not — so a target with a choice of two positions reads a press on
  // its top half the same way it would read a drop there.
  const pressDrop = (elementId: string, pageY: number) => {
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
    const position = rect ? positionAt(spot.positions, pageY, rect) : spot.positions?.[0];
    if (position) action.params = { ...(action.params ?? {}), [POSITION_PARAM]: position };

    send(action);
    clearSelection();
    setPendingGroupKey(null);
  };

  const dropProps = {
    registerDrop: (id: string, node: Measurable | null) => drops.register(id, node),
    activeDrops,
    hoveredDrop,
    hoveredPosition,
    pressableDrops,
    onPressDrop: pressDrop,
    armableGroups,
    armedGroupId: armedMeldIdLive,
    onAimGroup,
  };

  // The same table again: same game, same variation, the same numbers the
  // lobby chose, and a bot for every bot that was in this one. The options
  // come back from the server on the state message, so "again" means the table
  // that was actually played rather than whatever the defaults happen to be.
  const playAgain = async () => {
    setStartingAgain(true);
    setAgainError('');
    try {
      const { matchId: next } = await client.createMatch(
        state.moduleId,
        state.variation,
        state.options ?? {},
      );
      for (const p of state.players) {
        if (p.isAI) await client.addBot(next);
      }
      await client.startMatch(next);
      router.replace(`/match/${next}`);
    } catch (e) {
      setAgainError(String(e));
      setStartingAgain(false);
    }
  };

  // A stable id for remembering whether a zone's own panel is put away —
  // shared by every place this screen draws one.
  const panelIdFor = (zoneId: string) => `zone:${zoneId}`;
  const zonePanelProps = (zoneId: string) => ({
    panelId: panelIdFor(zoneId),
    minimized: panels.isMinimized(panelIdFor(zoneId)),
    onToggleMinimized: () => panels.toggle(panelIdFor(zoneId)),
  });

  // How the match ended, in the one vocabulary every game shares: who the
  // server says won.
  //
  // This exists because a finished match used to look exactly like a stuck
  // one. The board stayed on the last position, every control greyed out with
  // the engine's "the game is not running" beside it, and the only thing that
  // changed was a twelve-pixel word next to a dot that stayed green — so a
  // player whose own last move ended the match reported it as a hang, which is
  // the correct reading of a screen that says nothing.
  //
  // Read off the match envelope rather than the module's own status facts,
  // because that is the field every module fills: two of the four send no
  // end-of-match fact at all, and this has to be right for the next one too.
  // Naming yourself "you" is the only judgement in it, and it is about who is
  // reading rather than about what was played.
  const winners = state.winners ?? (state.winnerId ? [state.winnerId] : []);
  const iWon = winners.includes(viewerId);
  const winnerNames = winners.map((id) => (id === viewerId ? 'you' : playerName(state.players, id)));
  const outcome =
    winners.length === 0
      ? 'Nobody won.'
      : winners.length === 1
        ? iWon
          ? 'You won.'
          : `${winnerNames[0]} won.`
        : `Won by ${winnerNames.join(', ')}.`;

  // Offering the same table again only where this screen can actually set one
  // up: every other seat was a bot, so the same match is one create-and-start
  // away. A table with other people in it is a lobby's job, and pretending
  // otherwise would fail at the point of pressing.
  // A results panel is worth putting up at the end of a round as well as at the
  // end of a match, and there is nothing to put in one before the first round
  // has finished.
  const showResults =
    !!state.rounds?.rounds.length && (state.status === 'completed' || !!state.rounds.paused);

  const againstBotsAlone =
    state.players.length > 1 && state.players.every((p) => p.isAI || p.id === viewerId);

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
          <View style={styles.moduleGroup}>
            <Text testID="match-module" style={styles.module}>
              {state.moduleId}
              {state.variation ? ` · ${state.variation}` : ''}
            </Text>
            <Pressable
              testID="match-rules"
              onPress={() =>
                // One continuous template literal — see the matching comment
                // in app/lobby/games.tsx for why a `+` chain fails to typecheck
                // against expo-router's typed routes.
                router.push(
                  `/rules?moduleId=${encodeURIComponent(state.moduleId)}&variation=${encodeURIComponent(state.variation ?? '')}&options=${encodeURIComponent(JSON.stringify(state.options ?? {}))}`,
                )
              }
              hitSlop={8}
            >
              <Text style={styles.rulesLink}>Rules</Text>
            </Pressable>
          </View>
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
                {factText(f, state.players)}
              </Text>
            ))}
          </View>
        ) : null}
        {statusExplainerOpen ? (
          <Text testID="match-status-explainer" style={styles.statusExplainer}>
            {statusExplainer}
          </Text>
        ) : null}

        {/* The end of a match, said plainly and where the eye already is —
            above the board rather than under it, because the board below is
            the position that ended and a player arrives at this banner from
            the move they just made. */}
        {state.status === 'completed' ? (
          <View style={[styles.over, iWon && styles.overWon]} testID="match-over">
            <Text testID="match-over-title" style={styles.overTitle}>
              Match over
            </Text>
            <Text testID="match-over-outcome" style={styles.overOutcome}>
              {outcome}
            </Text>
            <View style={styles.overActions}>
              {againstBotsAlone ? (
                <Pressable
                  testID="match-over-again"
                  accessibilityState={{ disabled: startingAgain }}
                  disabled={startingAgain}
                  onPress={playAgain}
                  style={[styles.overButton, startingAgain && styles.overButtonBusy]}
                >
                  <Text style={styles.overButtonText}>
                    {startingAgain ? 'Setting up…' : 'Play again'}
                  </Text>
                </Pressable>
              ) : null}
              <Pressable
                testID="match-over-leave"
                onPress={() => router.replace('/lobby/games')}
                style={styles.overButtonQuiet}
              >
                <Text style={styles.overButtonQuietText}>Back to games</Text>
              </Pressable>
            </View>
            {againError ? (
              <Text testID="match-over-error" style={styles.overError}>
                {againError}
              </Text>
            ) : null}
          </View>
        ) : null}

        {/* What the match actually did, and what it did to the player's own
            record — under the banner that says it is over, and above the board,
            which is only the position it happened to stop in.

            Shown when the module says the table is between rounds too, not only
            at the end: a round's settlement is wiped off the table by the next
            one, so the moment it is readable is the only moment it exists. That
            the table is paused is the module's own answer on the round log, not
            something worked out here from the controls that happen to be live —
            and the control to go on is an ordinary offer, so the action bar
            below renders it like any other. */}
        {showResults ? (
          <>
            <RoundResults
              log={state.rounds!}
              players={state.players}
              standings={state.standings}
              viewerId={viewerId}
            />
            {state.status === 'completed' ? <LifetimeRecord moduleId={state.moduleId} /> : null}
          </>
        ) : null}

        <SeatStrip
          seats={view.seats ?? []}
          players={state.players}
          viewerId={viewerId}
          standings={state.standings}
          {...zonePanelProps('seats')}
        />

        {(view.prompts ?? []).map((f, i) => (
          <Text key={`prompt-${i}`} testID={`prompt-${i}`} style={styles.prompt}>
            {factText(f, state.players)}
          </Text>
        ))}

        {/* The piles and stacks everyone draws from and discards to. A
            full-width row of its own now that the controls have moved down
            under the hand — the zones inside it are a couple of cards wide
            and sit side by side in there, so it costs far less height than
            a full-width row suggests. */}
        <Section
          title="Table"
          zones={tableZones}
          compact
          {...zonePanelProps('section:table')}
          panelPropsFor={zonePanelProps}
          {...dropProps}
        />

        {/* Your hand, then the controls that act on it, as one pair — it's
            what your thumb is on every turn, and choosing a card and
            spending it should not be at two ends of the screen. Everyone's
            melds (yours and the opponents') go below the pair rather than
            above it, so reaching your cards never means scrolling past a
            wall of board state first. */}
        {/* Raised onto the drag layer for as long as a card is in flight, so
            the card being carried is drawn over the melds and the opponents
            below it rather than sliced in half by the first panel edge it
            crosses. The hand keeps hold of the card it is carrying (moving
            its node would lose the gesture), so lifting the card means
            lifting the hand — see `dragLayer`. */}
        <View style={[styles.mine, !!drag && dragLayer]}>
          {myHands.map((z) => (
            <HandZone
              key={z.id}
              zone={z}
              slots={slotsFor(z.id)}
              selected={selected}
              onToggle={toggleSlot}
              onMove={(from, to) => move(z.id, from, to)}
              onAutoArrange={() => arrange(z.id)}
              onDragStart={(index) => beginDrag(z.id, index)}
              onDragMove={moveDrag}
              onDragEnd={endDrag}
              externalTarget={hoveredDrop}
              {...zonePanelProps(z.id)}
            />
          ))}
        </View>

        {/* Directly under your hand, at every screen width. Every control
            here acts on the cards picked just above it, and a bar up beside
            the piles meant looking in one place to choose and another to
            act — with the whole hand in between, which on a phone is most
            of the screen. Under the hand rather than over it because that
            is the edge a thumb is already resting on.

            The cost is that the piles no longer have a neighbour to share
            their band with on a wide screen. Worth paying: that band was
            shared at the price of putting every button a full hand away
            from the cards it spends. */}
        <Panel
          {...zonePanelProps('controls')}
          title="Controls"
          testID="controls-panel"
          summary={
            <OfferGlance
              offers={state.legalActions}
              selectedCards={selectedCards}
              armedGroupId={armedMeldIdLive}
              onSend={send}
              onConsumeSelection={clearSelection}
              onAmbiguous={(groupKey) => {
                setPendingGroupKey(groupKey);
                drops.measure();
              }}
              testID="controls-summary"
            />
          }
        >
          {/* The engine's own sentence stands in for a code this build has
              no translation for — it is at least a sentence, where the bare
              code reads as a crash. A code we do know still wins, so a
              translated message never regresses to English. */}
          {error ? (
            <Text testID="match-error" style={styles.error} onPress={clearError}>
              {reasonText(error.code, error.message || error.code)}
            </Text>
          ) : null}
          {/* Disabled offers stay on screen with their reason. An offer
              that vanished when it became illegal would be
              indistinguishable from a bug, which is why the server sends
              the whole set every time. */}
          <OfferBar
            offers={state.legalActions}
            selectedCards={selectedCards}
            armedGroupId={armedMeldIdLive}
            onSend={send}
            onConsumeSelection={clearSelection}
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
        </Panel>

        {/* Every spread on the board, whoever's it is, sharing a wrapping
            row instead of each claiming a full-width line — named by its
            owner where the server sent one, so two or more players' melds
            read as whose they are at a glance rather than an anonymous
            stack of "Melds". */}
        {orderedSpreads.length > 0 ? (
          <View style={styles.spreads} testID="section-spreads">
            {orderedSpreads.map((z) => (
              <ZoneView
                key={z.id}
                zone={z}
                title={z.ownerId ? playerName(state.players, z.ownerId) + (z.ownerId === viewerId ? ' (you)' : '') : undefined}
                {...zonePanelProps(z.id)}
                {...dropProps}
              />
            ))}
          </View>
        ) : null}

        <Section
          title="Opponents"
          zones={otherZones}
          compact
          {...zonePanelProps('section:opponents')}
          panelPropsFor={zonePanelProps}
          {...dropProps}
        />

        {(view.status ?? []).map((f, i) => (
          <Text key={`status-${i}`} testID={`status-${i}`} style={styles.muted}>
            {factText(f, state.players)}
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
  panelId,
  minimized,
  onToggleMinimized,
  panelPropsFor,
  ...drops
}: {
  title: string;
  zones: Zone[];
  compact?: boolean;
  panelId: string;
  minimized: boolean;
  onToggleMinimized: () => void;
  panelPropsFor: (zoneId: string) => { panelId: string; minimized: boolean; onToggleMinimized: () => void };
  registerDrop?: (id: string, node: Measurable | null) => void;
  activeDrops?: ReadonlySet<string>;
  hoveredDrop?: string | null;
  hoveredPosition?: { index: number; count: number } | null;
  pressableDrops?: ReadonlySet<string>;
  onPressDrop?: (elementId: string, pageY: number) => void;
  armableGroups?: ReadonlySet<string>;
  armedGroupId?: string | null;
  onAimGroup?: (groupId: string) => void;
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
    <Panel
      panelId={panelId}
      title={title}
      minimized={minimized}
      onToggleMinimized={onToggleMinimized}
      testID={`section-${title.toLowerCase()}`}
      style={styles.section}
      summary={
        <View style={styles.sectionSummary}>
          {zones.map((z, i) => (
            <Text key={z.id} style={styles.sectionSummaryText} numberOfLines={1}>
              {label(z.labelKey) || z.id} {z.count}
              {i < zones.length - 1 ? ' · ' : ''}
            </Text>
          ))}
        </View>
      }
    >
      {beside.length > 0 ? (
        <View style={styles.beside} testID={`section-beside-${title.toLowerCase()}`}>
          {beside.map((z) => (
            <ZoneView key={z.id} zone={z} compact={compact} inline nested {...panelPropsFor(z.id)} {...drops} />
          ))}
        </View>
      ) : null}
      {stacked.map((z) => (
        <ZoneView key={z.id} zone={z} compact={compact} nested {...panelPropsFor(z.id)} {...drops} />
      ))}
    </Panel>
  );
}

const styles = StyleSheet.create({
  body: { paddingBottom: 40, gap: 4 },
  headerRow: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center' },
  moduleGroup: { flexDirection: 'row', alignItems: 'center', gap: 8 },
  module: { color: colors.text, fontWeight: '700', fontSize: 16 },
  rulesLink: { color: colors.accent, fontSize: 12, fontWeight: '700' },
  statusGroup: { flexDirection: 'row', alignItems: 'center', gap: 6 },
  status: { color: colors.muted, fontSize: 12 },
  facts: { flexDirection: 'row', flexWrap: 'wrap', gap: 10, marginTop: 2 },
  fact: { color: colors.muted, fontSize: 12 },
  statusDot: { width: 10, height: 10, borderRadius: 5, marginTop: 1 },
  statusDotOk: { backgroundColor: colors.success },
  statusDotBad: { backgroundColor: colors.danger },
  statusExplainer: { color: colors.muted, fontSize: 12, marginTop: 4 },
  prompt: { color: colors.gold, fontSize: 13, marginTop: 6 },
  section: { marginTop: 10 },
  sectionSummary: { flexDirection: 'row', flexShrink: 1, minWidth: 0 },
  sectionSummaryText: { color: colors.muted, fontSize: 12 },
  beside: { flexDirection: 'row', flexWrap: 'wrap', alignItems: 'flex-start', gap: 8, marginBottom: 8 },
  mine: { marginTop: 10 },
  // Two or more to a row rather than each claiming a full-width line — see
  // the comment above where this is used. alignItems: flex-start keeps each
  // panel sized to its own content instead of the row's default stretch,
  // which would size every panel in a row to match its tallest neighbour —
  // and make a minimized panel look exactly as tall as an open one beside it.
  spreads: { flexDirection: 'row', flexWrap: 'wrap', alignItems: 'flex-start', gap: 8, marginTop: 10 },
  error: { color: colors.danger, fontSize: 13, marginVertical: 6 },
  muted: { color: colors.muted, fontSize: 12, marginTop: 6 },

  // The end of a match, built like the rule-violation banner in `shared`: a
  // tinted box with a border of its own, because the thing it has to beat is
  // being mistaken for nothing having happened. Green only when the reader
  // won — a coloured congratulation on a loss is worse than a plain box.
  over: {
    backgroundColor: 'rgba(61, 139, 253, 0.10)',
    borderWidth: 1,
    borderColor: colors.accent,
    borderRadius: 8,
    padding: 12,
    marginTop: 8,
    gap: 6,
  },
  overWon: {
    backgroundColor: 'rgba(74, 222, 128, 0.12)',
    borderColor: colors.success,
  },
  overTitle: { color: colors.text, fontSize: 15, fontWeight: '700' },
  overOutcome: { color: colors.text, fontSize: 14 },
  overActions: { flexDirection: 'row', flexWrap: 'wrap', gap: 8, marginTop: 4 },
  overButton: {
    backgroundColor: colors.accentButton,
    paddingVertical: 10,
    paddingHorizontal: 16,
    borderRadius: 8,
  },
  overButtonBusy: { opacity: 0.4 },
  overButtonText: { color: colors.onAccent, fontSize: 14, fontWeight: '600' },
  overButtonQuiet: {
    backgroundColor: colors.surface,
    borderWidth: 1,
    borderColor: colors.border,
    paddingVertical: 10,
    paddingHorizontal: 16,
    borderRadius: 8,
  },
  overButtonQuietText: { color: colors.text, fontSize: 14, fontWeight: '600' },
  overError: { color: colors.danger, fontSize: 12 },
});
