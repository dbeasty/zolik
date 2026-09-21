import { router, Stack, useLocalSearchParams } from 'expo-router';
import { useCallback, useEffect, useMemo, useRef, useState, type RefObject } from 'react';
import {
  Animated,
  Pressable,
  ScrollView,
  Text,
  View,
  type NativeScrollEvent,
  type NativeSyntheticEvent,
} from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import type { ActionOffer, Zone } from '@/src/api/matchTypes';
import { POSITION_PARAM, offerGroupKey, submissionFor } from '@/src/api/matchTypes';
import { Attention } from '@/src/components/match/Attention';
import { BoardLayout, matchStyles } from '@/src/components/match/BoardLayout';
import { FlightLayer, type QueuedFlight } from '@/src/components/match/FlightLayer';
import { HandZone } from '@/src/components/match/HandZone';
import { LifetimeRecord } from '@/src/components/match/LifetimeRecord';
import { OfferBar, OfferGlance } from '@/src/components/match/OfferBar';
import { Panel } from '@/src/components/match/Panel';
import { ResultsFlash } from '@/src/components/match/ResultsFlash';
import { RoundResults } from '@/src/components/match/RoundResults';
import { TableSurface } from '@/src/components/match/TableSurface';
import { useSession } from '@/src/context/SessionContext';
import { useMetrics } from '@/src/hooks/useMetrics';
import { useDropRegistry, type Measurable } from '@/src/hooks/useDropRegistry';
import { useArrival } from '@/src/hooks/useArrival';
import { useHandOrder } from '@/src/hooks/useHandOrder';
import { useMatchSocket } from '@/src/hooks/useMatchSocket';
import { usePanelState } from '@/src/hooks/usePanelState';
import { useEndingScroll } from '@/src/hooks/useEndingScroll';
import { useOpeningScroll } from '@/src/hooks/useOpeningScroll';
import { useResultsFlash } from '@/src/hooks/useResultsFlash';
import {
  dropSpotsFor,
  groupElementId,
  positionAt,
  refusalAt,
  someOfferReady,
  sourceSpotsFor,
  takeableSpots,
  zoneElementId,
  type DropSpot,
} from '@/src/lib/drops';
import {
  EMPTY_FLIGHT_PLAN,
  OWN_MOVE_QUIET_MS,
  planFlights,
  type BoardLike,
  type FlightPlan,
} from '@/src/lib/flights';
import { useReducedMotion } from '@/src/hooks/useReducedMotion';
import { cardsForSelection, slotsForDrag, toggleSelection } from '@/src/lib/hand';
import { reasonText, t } from '@/src/lib/i18n';
import { ApiError } from '@/src/api/client';
import { savePendingDestination } from '@/src/lib/pendingDestination';
import { WhySheet, type Refusal } from '@/src/components/match/WhySheet';
import { useRuleIndex } from '@/src/hooks/useRuleIndex';
import { useSkinControls } from '@/src/hooks/useSkin';
import { factText, playerName } from '@/src/lib/labels';
import { dragLayer } from '@/src/theme';

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
  const { session, client, loading } = useSession();
  const [selected, setSelected] = useState<ReadonlySet<string>>(() => new Set());
  // Whether what is currently selected was picked by the *app* rather than by
  // the player — see the auto-select effect below and `toggleSlot`.
  const [selectionIsAuto, setSelectionIsAuto] = useState(false);
  const panels = usePanelState(matchId ? String(matchId) : undefined);

  const url = useMemo(() => {
    if (!matchId || !session?.accessToken) return null;
    return client.matchSocketUrl(String(matchId));
  }, [client, matchId, session?.accessToken]);

  // A link to a table almost never arrives at a signed-in app: it comes out of
  // a chat, on a device that may never have been used to play. Without this the
  // screen rendered "Connecting…" for ever and meant it — the socket URL needs
  // a token, `useMatchSocket` is handed null without one, and null is the one
  // input it neither opens nor fails on. So the screen sat there with no
  // socket, no error and no timeout, looking exactly like a server that was
  // not answering. `/join/[code]` has always handed sign-in a note saying where
  // to come back to; the table's own link, which is the one a player follows to
  // their own game, never learned to.
  //
  // Waits for `loading`, because the session is read from storage
  // asynchronously and acting before it settles would send a returning player
  // through guest sign-in they did not need.
  useEffect(() => {
    if (loading || session || !matchId) return;
    let live = true;
    void savePendingDestination(`/match/${encodeURIComponent(String(matchId))}`).then(() => {
      if (live) router.replace('/auth/guest');
    });
    return () => {
      live = false;
    };
  }, [loading, session, matchId]);

  const { state, error, connected, send, clearError } = useMatchSocket(url);
  // The table's own written rules, by id — what a refusal's `ruleIds` point
  // into. Fetched once per table and cached; empty until it lands, which
  // only means a sheet shows its reason and remedy with no rule behind it.
  const ruleIndex = useRuleIndex(state);
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
  // The refusal currently being explained, if any. One at a time: a sheet is
  // an answer to a question a player just asked, and the last one asked is
  // the one they meant.
  const [explaining, setExplaining] = useState<Refusal | null>(null);
  // Which of `hoveredDrop`'s ordered positions the drag is currently over — a
  // card carried over a run says up front which end it would extend, rather
  // than only after it is let go of.
  //
  // An index into the offer's own `positions` list, not the position's name,
  // so the shell that renders it (`ZoneView`) never has to know what "front"
  // or "end" means. `slot` is the same answer as a place rather than an
  // ordinal — where among the group's cards this one would land — which is
  // what lets the meld come apart to show the gap, the way the hand does.
  // The module supplies it (see `Placement.slots`); null when it did not, and
  // then the position is only shaded, never opened.
  const [hoveredPosition, setHoveredPosition] = useState<{
    index: number;
    count: number;
    slot: number | null;
  } | null>(null);
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
  // Bringing a swept-up table back, and whatever went wrong trying. Separate
  // from the pair above because they are separate offers on the same banner:
  // one continues this game, the other starts a new one like it.
  const [resuming, setResuming] = useState(false);
  const [resumeError, setResumeError] = useState('');
  // The end of a match arrives the same way the end of a round does, because it
  // is the same kind of thing happening: the table stopped, and here is why.
  //
  // Up here with the rest for the same reason they are: this is a hook, the
  // early return below is conditional, and a hook after it is not.
  const overArrival = useArrival(state?.status ?? '');

  // Which ending the table is sitting on, and whether it is news. Up here with
  // `overArrival` for exactly the reason given above it — and handed the whole
  // state, `null` included, because "no board has arrived yet" is a state this
  // has to be able to tell apart from "no round has ended".
  const flash = useResultsFlash(state);

  // The look the board is wearing, and the switcher's handle on the rest.
  // The style factory keys on the skin, so switching repaints everything at
  // once — and repaints only: no size a drag is measured against changes.
  const { skin, skins, setSkinId } = useSkinControls();
  const styles = useMemo(() => matchStyles(skin), [skin]);
  // Only for how wide the board is allowed to get — every other size on this
  // screen is decided by the component that draws it, from the same metrics.
  const metrics = useMetrics();
  const cycleSkin = () => {
    const at = skins.findIndex((s) => s.id === skin.id);
    setSkinId(skins[(at + 1) % skins.length]!.id);
  };

  // Cards seen travelling between zones. The server never says "a card
  // moved" — it sends the next board — so the journey is reconstructed by
  // comparing this board with the last one (see `src/lib/flights.ts`), and
  // flown on an overlay that touches nothing. Asked for stillness, nothing
  // is ever planned, and the board behaves exactly as it did before flights
  // existed — which is also what keeps the e2e suite deterministic.
  const stillness = useReducedMotion();
  const boardRef = useRef<BoardLike | null>(null);
  // When a card last left the hand by being physically carried somewhere —
  // that journey already happened under the player's finger, and replaying
  // it from the fan would be a second answer to the same question.
  const dragSentAt = useRef(0);
  const flightPlan = useMemo<FlightPlan>(() => {
    if (stillness || !state) return EMPTY_FLIGHT_PLAN;
    const next: BoardLike = { zones: state.view?.zones ?? [], seats: state.view?.seats ?? [] };
    const plan = planFlights(boardRef.current, next, viewerId);
    if (!plan.flights.length || Date.now() - dragSentAt.current > OWN_MOVE_QUIET_MS) return plan;
    const fromOwnFan = new Set(
      next.zones
        .filter((z) => z.kind === 'hand' && z.ownerId === viewerId)
        .map((z) => zoneElementId(z.id)),
    );
    const kept = plan.flights.filter((f) => !fromOwnFan.has(f.fromId));
    if (kept.length === plan.flights.length) return plan;
    const holds = new Map(plan.holds);
    for (const f of plan.flights) if (fromOwnFan.has(f.fromId)) holds.delete(f.toId);
    return { flights: kept, holds };
  }, [state, stillness, viewerId]);
  useEffect(() => {
    if (state) boardRef.current = { zones: state.view?.zones ?? [], seats: state.view?.seats ?? [] };
  }, [state]);

  // The flights currently in the air — appended when a plan lands, removed
  // as each one touches down. De-duplicated by the plan's own stable ids,
  // so replanning the same transition can never double a card.
  const [flightsInAir, setFlightsInAir] = useState<QueuedFlight[]>([]);
  useEffect(() => {
    if (!flightPlan.flights.length) return;
    setFlightsInAir((was) => {
      const have = new Set(was.map((f) => f.id));
      const fresh = flightPlan.flights.filter((f) => !have.has(f.id));
      const bornAt = Date.now();
      return fresh.length ? [...was, ...fresh.map((f) => ({ ...f, bornAt }))] : was;
    });
  }, [flightPlan]);
  const landFlight = useCallback((id: string) => {
    setFlightsInAir((was) => was.filter((f) => f.id !== id));
  }, []);

  // Where the player is when a round or the match ends. `useResultsFlash` above
  // says one is over; it does not say where the way on is, and the way on is
  // drawn at the top of a board whose reader is at the bottom of it. Up here
  // with the other hooks for the reason they all are, and handed the whole
  // state — `null` included — for the reason the flash is.
  //
  // `goTo` is the half the screen owns, because only the screen knows how the
  // board moves: someone who asked their system for less movement is taken
  // there rather than travelling there, exactly as with everything else here.
  const scrollRef = useRef<ScrollView>(null);
  const goTo = useCallback(
    (y: number) => {
      scrollRef.current?.scrollTo({ y, animated: !stillness });
    },
    [stillness],
  );
  const ending = useEndingScroll(state, goTo);
  // And where the player is when one begins, which is the top of a board whose
  // game is halfway down it. The other half of the same idea as `ending`, and
  // the two can never both want the scroller: a deal is on or the table has
  // stopped, never both. The scroller is handed over as something measurable
  // rather than as a ScrollView, because measuring is all this wants of it —
  // `measureInWindow` is on the instance both platforms hand back, and on
  // neither one is it on the published type.
  const opening = useOpeningScroll(
    state,
    goTo,
    scrollRef as unknown as RefObject<Measurable | null>,
  );
  // One scroller, two hooks that watch it. Each only wants to know where the
  // board is now, so neither minds the other having been told first.
  const onScroll = useCallback(
    (e: NativeSyntheticEvent<NativeScrollEvent>) => {
      ending.scrollProps.onScroll(e);
      opening.scrollProps.onScroll(e);
    },
    [ending.scrollProps, opening.scrollProps],
  );

  if (!state) {
    return (
      <View style={styles.root}>
        <TableSurface />
        <SafeAreaView style={styles.safe} edges={['top', 'left', 'right']}>
          {/* A refusal that arrives before any board has to be drawn here,
              because the one place this screen renders `error` is inside the
              controls panel — which does not exist until there is a state to
              build it from. So "that table no longer exists" was being set,
              and shown nowhere: the screen went on saying "Waiting for the
              table…" about a table the server had just said was gone. A
              player who followed an old link had no way to tell that from a
              server that had stopped answering, and no way out but the back
              button. */}
          {error ? (
            <>
              <Text testID="match-gone" style={styles.error}>
                {reasonText(error.code, error.message || error.code)}
              </Text>
              <Pressable
                testID="match-gone-leave"
                onPress={() => router.replace('/lobby/games')}
                style={styles.overButtonQuiet}
              >
                <Text style={styles.overButtonQuietText}>{t('match.backToGames')}</Text>
              </Pressable>
            </>
          ) : (
            <Text testID="match-connecting" style={styles.muted}>
              {connected ? t('match.waitingForTable') : t('match.connecting')}
            </Text>
          )}
        </SafeAreaView>
      </View>
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

  // Whether a card in hand carries a mark the module put there — Žolíky's
  // discard-pickup debt is the one live example (see `badgedCardViews` on the
  // server), but this reads the mark rather than deciding what it means, the
  // same way the badge itself is rendered with no idea what it says.
  const isMarked = (card: string) =>
    myHands.some((z) => (z.cards ?? []).some((c) => c.card === card && (c.badgeKeys?.length ?? 0) > 0));

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
        // A marked card is one the module itself flagged as owed to a meld
        // this turn — a pickup off the discard pile the player is very
        // likely still building around, not one they meant to abandon the
        // moment the next tap didn't finish a meld outright. Stay joined so
        // gathering a third card doesn't need the pickup re-selected by
        // hand; tapping the marked card itself still drops it, same as ever.
        const auto = heldSlots.find((s) => prev.has(s.id));
        if (auto && isMarked(auto.card)) return joined;
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
  // Every place the cards in flight land on — including the ones that would
  // refuse them, which now carry the reason instead of being dropped from the
  // list. `takeableSpots` is what lights up and what a release sends;
  // `refusalAt` is what a release anywhere else says.
  const spotsFor = (cards: string[]) => dropSpotsFor(state.legalActions, cards);
  const liveSpots = drag ? spotsFor(drag.cards) : [];
  const liveTakeable = takeableSpots(liveSpots);

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

  const pendingTakeable = takeableSpots(pendingSpots);

  // The piles you may take *from* right now — the deck, and the discard pile
  // in a game whose draw phase offers both. The mirror image of everything
  // above: those are places the cards in hand may go, this is a move with no
  // cards to it at all, and the only thing on screen that names it is the pile
  // itself. `sourceSpotsFor` decides which, from the offers and nothing else.
  //
  // Only while nothing is picked and nothing is in flight. A pile with cards
  // chosen means "put these here" — the discard pile is both, one phase apart
  // — and a press has to mean one thing. What is picked is what says which.
  const sourceSpots =
    !drag && selectedCards.length === 0 && !pendingGroupKey
      ? sourceSpotsFor(state.legalActions, zones, viewerId)
      : [];

  const activeDrops = new Set(
    [...liveTakeable, ...pendingTakeable, ...sourceSpots].map((s) => s.elementId),
  );
  // Where letting go would be refused, drawn as refusing rather than merely
  // left unlit — an unlit target and a forbidden one look identical, and the
  // difference is the whole question a player is asking mid-drag.
  const refusedDrops = new Set(
    liveSpots.filter((s) => s.refusal && !activeDrops.has(s.elementId)).map((s) => s.elementId),
  );
  const pressableDrops = new Set([...pendingTakeable, ...sourceSpots].map((s) => s.elementId));

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
    const spots = takeableSpots(spotsFor(current.cards));
    const over = drops.hit(x, y, spots.map((s) => s.elementId));
    setHoveredDrop((prev) => (prev === over ? prev : over));

    // Which of the target's ordered positions this hover currently means,
    // shown live so a player can see where a card will land before letting
    // go of it, rather than finding out only after. `null` when nothing is
    // hovered, or the target has no positions at all.
    //
    // A single position counts. It used to be dropped — with one answer there
    // was nothing to *choose* between, and a shaded band covering the whole
    // meld said no more than the meld's own highlight already did. That stops
    // being true once the group can come apart to show the gap: one legal
    // place is still a place, and "the 6 goes on the front of this run" is
    // exactly what a player wants to see before they let go.
    const spot = over ? spots.find((s) => s.elementId === over) : undefined;
    const rect = over ? drops.rectFor(over) : undefined;
    const positions = spot?.positions;
    const resolved = spot && rect ? positionAt(positions, y, rect) : undefined;
    const index = resolved && positions ? positions.indexOf(resolved) : -1;
    const next =
      positions?.length && index >= 0
        ? { index, count: positions.length, slot: spot?.slots?.[index] ?? null }
        : null;
    setHoveredPosition((prev) =>
      prev?.index === next?.index && prev?.count === next?.count && prev?.slot === next?.slot
        ? prev
        : next,
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
    const takeable = takeableSpots(spots);
    const over = drops.hit(x, y, spots.map((s) => s.elementId));
    const spot = takeable.find((s) => s.elementId === over);
    if (!spot) {
      // Let go somewhere the cards are not welcome. Before this the card
      // simply snapped home and nothing was said, which reads as an
      // unresponsive interface rather than a refused move — and the reason
      // was already in hand.
      const refusal = over ? refusalAt(spots, over) : undefined;
      if (refusal) {
        setExplaining(refusal);
        return true;
      }
      return false;
    }

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

    // The card was carried there by hand — its journey has been made, so the
    // planner must not fly it a second time (see `flightPlan`).
    dragSentAt.current = Date.now();
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
    // A target chosen by what is picked wins over a pile that would be taken
    // from, for the same reason `sourceSpots` is empty while anything is
    // picked: with cards in hand a pile is somewhere to put them.
    const spot =
      pendingSpots.find((s) => s.elementId === elementId) ??
      sourceSpots.find((s) => s.elementId === elementId);
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

  // Which cards the module marked, by card value. A mark is something true
  // about a particular card that a player should act on before it becomes a
  // refusal; only the module knows what, so this reads the marks rather than
  // deciding them.
  const badgesFor = (zone: Zone): ReadonlyMap<string, string[]> => {
    const out = new Map<string, string[]>();
    for (const c of zone.cards ?? []) {
      if (c.badgeKeys?.length) out.set(c.card, c.badgeKeys);
    }
    return out;
  };

  const dropProps = {
    registerDrop: (id: string, node: Measurable | null) => drops.register(id, node),
    activeDrops,
    sourceDrops: new Set(sourceSpots.map((s) => s.elementId)),
    refusedDrops,
    hoveredDrop,
    hoveredPosition,
    pressableDrops,
    onPressDrop: pressDrop,
    armableGroups,
    armedGroupId: armedMeldIdLive,
    onAimGroup,
    entranceDelays: flightPlan.holds,
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

  // This same table, carrying on from where it stopped.
  //
  // Nothing is rebuilt: the sweeper only wrote a status and an end time, so the
  // hands, the melds, the pile and the score are all still on the server and
  // the position comes back exactly as it was. There is deliberately no
  // navigation afterwards — this socket is still open and still in the table's
  // room, so the revived board arrives as an ordinary state message and the
  // banner goes away by itself.
  const resume = async () => {
    setResuming(true);
    setResumeError('');
    try {
      await client.resumeMatch(String(matchId));
    } catch (e) {
      // Rendered from the same locale bundle as every other refusal rather
      // than as whatever the exception stringifies to: "Everyone has to be
      // back at the table before this game can be picked up" is an answer,
      // where `ApiError: TABLE_HAS_PLAYERS_AWAY` is a stack trace shown to a
      // player. Reachable even with the button gated on the server's own
      // answer, because somebody can leave between the state message that
      // offered it and the press.
      const code = e instanceof ApiError ? e.code : undefined;
      setResumeError(reasonText(code, e instanceof Error ? e.message : String(e)));
    } finally {
      setResuming(false);
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
  const winnerNames = winners.map((id) => (id === viewerId ? t('match.you') : playerName(state.players, id)));
  const outcome =
    winners.length === 0
      ? t('match.nobodyWon')
      : winners.length === 1
        ? iWon
          ? t('match.youWon')
          : t('match.someoneWon', { name: winnerNames[0] })
        : t('match.wonBy', { names: winnerNames.join(', ') });

  // Offering the same table again only where this screen can actually set one
  // up: every other seat was a bot, so the same match is one create-and-start
  // away. A table with other people in it is a lobby's job, and pretending
  // otherwise would fail at the point of pressing.
  // A results panel is worth putting up at the end of a round as well as at the
  // end of a match, and there is nothing to put in one before the first round
  // has finished.
  const showResults =
    !!state.rounds?.rounds.length &&
    (state.status === 'completed' || state.status === 'abandoned' || !!state.rounds.paused);

  // The table is sitting between rounds. The module's own answer, never worked
  // out here from the controls that happen to be live.
  const paused = !!state.rounds?.paused;

  const againstBotsAlone =
    state.players.length > 1 && state.players.every((p) => p.isAI || p.id === viewerId);

  // The table was set aside by the sweeper rather than played to the end. A
  // different ending, and it needs different words and a different offer — it
  // is the one ending that can be undone.
  const wasAbandoned = state.status === 'abandoned';

  // Whether this table can be picked up, as the server answers it — never
  // worked out here.
  //
  // This screen used to decide for itself, using `againstBotsAlone` as a
  // stand-in for the old server rule. The two then diverged in the worst
  // direction: a game between two people who were both back and both looking
  // at the board was offered nothing at all, on a banner that told them the
  // cards were exactly where they had left them. Now the button appears when
  // the server would honour it, and when it would not, the line below says
  // who everyone is waiting for instead of leaving them to guess.
  const canResume = wasAbandoned && !!state.canResume;
  const awayNames = (state.awayPlayers ?? [])
    .map((id) => playerName(state.players, id))
    .filter(Boolean);

  // What the status dot means, in the same words the line it replaced used
  // to say. Red is the one case a player needs to notice — everything else
  // (active, completed) is green, since "simple red or green" was the ask,
  // not a status per state value.
  //
  // `abandoned` is red for the same reason `suspended` is, and adding it here
  // is half of a real bug: the dot fell through to green and the explainer
  // read "everything is connected and moving normally" on a table the sweeper
  // had resolved hours earlier, whose every control the engine was refusing.
  // That is the exact failure the comment above `winners` describes being
  // fixed once for finished matches, reappearing for swept-up ones.
  const statusOk = state.status !== 'suspended' && state.status !== 'abandoned';
  const statusExplainer =
    state.status === 'suspended'
      ? t('match.pausedFor', { name: playerName(state.players, state.suspendedPlayer ?? '') })
      : state.status === 'abandoned'
        ? t('match.abandoned')
        : state.status === 'completed'
          ? t('match.finished')
          : t('match.inProgress');

  // The controls, built once and rendered in one of two places.
  //
  // Ordinarily they sit directly under the hand, because every one of them acts
  // on the cards picked just above it. Between rounds none of that holds: the
  // module offers exactly one control — go on to the next round — the hand is
  // not actionable, and a single button left several screens below the
  // settlement it belongs to is how a paused table comes to read as a stuck
  // one. So while the table is paused they are rendered up with the results
  // instead, and the two positions share one piece of JSX so they can never
  // drift apart.
  const controlsPanel = (
    <Panel
      {...zonePanelProps('controls')}
      title={t('match.controls')}
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
        <Text
          testID="match-error"
          style={styles.error}
          onPress={() => {
            // A submission refused on arrival — a meld a person composed,
            // which had no greyed-out control of its own to have been
            // explained in advance — gets the same three layers as one
            // that did. The frame carries its own ruleIds; the remedy
            // comes from whichever offer is now on the table.
            setExplaining({ code: error.code, ruleIds: error.ruleIds });
            clearError();
          }}
        >
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
        onExplain={setExplaining}
        // Between rounds the module offers one thing: go on. Said here as
        // "the table is waiting on this bar" rather than as any offer's name,
        // so the bar rings whatever the one thing turns out to be.
        urgent={paused}
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
          {t('match.waitingForPlayer')}
        </Text>
      ) : null}
    </Panel>
  );

  // Your own cards, and everything you can do to them by touch: pick, carry,
  // rearrange. The one part of the board the two screens genuinely disagree
  // about, which is why BoardLayout takes it as a slot rather than drawing it
  // — a replay has the same cards and none of the verbs.
  // Raised onto the drag layer for as long as a card is in flight, so the card
  // being carried is drawn over the melds and the opponents below it rather
  // than sliced in half by the first panel edge it crosses. The hand keeps
  // hold of the card it is carrying (moving its node would lose the gesture),
  // so lifting the card means lifting the hand — see `dragLayer`.
  const handPanel = (
  <View style={[styles.mine, !!drag && dragLayer]} {...opening.anchor('hand')}>
    {myHands.map((z) => (
      <HandZone
        key={z.id}
        zone={z}
        slots={slotsFor(z.id)}
        selected={selected}
        onToggle={toggleSlot}
        onMove={(from, to) => {
          // Moving a card along the fan drops it from the selection.
          //
          // Tidying your hand and choosing what to play are different
          // intentions, and the tap that selected a card is also the
          // start of the drag that rearranges it — so a card shuffled
          // into place stayed lit, and the next card tapped joined a
          // selection the player had stopped thinking about. A card you
          // have just put somewhere is one you are organising, not one
          // you are about to spend.
          const moved = slotsFor(z.id)[from];
          if (moved) {
            setSelected((prev) => {
              if (!prev.has(moved.id)) return prev;
              const next = new Set(prev);
              next.delete(moved.id);
              return next;
            });
          }
          move(z.id, from, to);
        }}
        onAutoArrange={() => arrange(z.id)}
        onDragStart={(index) => beginDrag(z.id, index)}
        onDragMove={moveDrag}
        onDragEnd={endDrag}
        externalTarget={hoveredDrop}
        registerSpot={dropProps.registerDrop}
        entranceDelay={flightPlan.holds.get(zoneElementId(z.id)) ?? 0}
        badges={badgesFor(z)}
        onPressBadge={(card, badgeKeys) =>
          setExplaining({
            labelKey: badgeKeys[0],
            params: { card },
            // The rules and the way out behind the mark come from
            // whichever offer this card is about to be refused by —
            // asked for on long-press, rather than the module having
            // to say the same thing twice.
            ...refusalBehindBadge(state.legalActions),
          })
        }
        {...zonePanelProps(z.id)}
      />
    ))}
  </View>
  );

  return (
    <View style={styles.root}>
      {/* The felt. Behind everything, catches nothing. */}
      <TableSurface />
      {/* The status lives in the navigation bar, beside the screen's own name
          and hard against the left edge. It used to sit at the right end of
          the row below, where the longest module name pushed it under the
          scrollbar — and where it scrolled out of sight the moment a player
          looked at their hand. Up here it is pinned: the colour is the
          at-a-glance signal, the word beside it is the same fact spelled out,
          and a tap still gets the fuller explanation — now printed directly
          under the bar it was tapped on, so it lands in view from anywhere on
          the board. */}
      <Stack.Screen
        options={{
          headerTitleAlign: 'left',
          // The bar above the board dresses to match the board — without
          // this it keeps the app-wide chrome and the felt starts at a seam.
          headerStyle: { backgroundColor: skin.colors.bg },
          headerTintColor: skin.colors.text,
          headerTitle: () => (
            <Pressable
              testID="match-status-dot"
              onPress={() => setStatusExplainerOpen((v) => !v)}
              hitSlop={8}
              style={styles.headerTitleGroup}
            >
              <Text style={styles.headerTitleText}>{t('nav.match')}</Text>
              <View
                style={[styles.statusDot, statusOk ? styles.statusDotOk : styles.statusDotBad]}
              />
              <Text testID="match-status" style={styles.status}>
                {state.status}
              </Text>
            </Pressable>
          ),
        }}
      />
      <SafeAreaView style={styles.safe} edges={['top', 'left', 'right']}>
      {/* Drawn outside the board rather than at the top of it, so it answers
          the tap wherever the player happens to be scrolled to. */}
      {statusExplainerOpen ? (
        <Text testID="match-status-explainer" style={styles.statusExplainer}>
          {statusExplainer}
        </Text>
      ) : null}
      <ScrollView
        ref={scrollRef}
        // Wide enough for the table and no wider. On a large monitor the felt
        // runs to both edges (it is drawn behind everything) while the board
        // itself stops at a table's width — without this, a player's hand and
        // the draw pile end up at opposite ends of a metre of glass.
        contentContainerStyle={[styles.body, { maxWidth: metrics.maxWidth, width: '100%', alignSelf: 'center' }]}
        testID="match-screen"
        {...ending.scrollProps}
        // Last, and deliberately: the spread above carries an `onScroll` of
        // the ending's own, and this one replaces it and tells them both.
        onScroll={onScroll}
      >
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
              <Text style={styles.rulesLink}>{t('nav.rules')}</Text>
            </Pressable>
          </View>
          {/* Which look the board wears, cycled in place. A preference about
              pixels, not about the game — it lives on the device and no
              module knows it exists. */}
          <Pressable testID="skin-toggle" onPress={cycleSkin} hitSlop={8} style={styles.skinToggle}>
            <Text style={styles.skinToggleText}>◈ {skin.label}</Text>
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

        {/* The end of a match, said plainly and where the eye already is —
            above the board rather than under it, because the board below is
            the position that ended and a player arrives at this banner from
            the move they just made. */}
        {state.status === 'completed' || state.status === 'abandoned' ? (
          <Animated.View
            style={[styles.over, iWon && !wasAbandoned && styles.overWon, overArrival]}
            testID="match-over"
            {...ending.anchor('over')}
          >
            <Text testID="match-over-title" style={styles.overTitle}>
              {wasAbandoned ? t('match.abandonedTitle') : t('match.over')}
            </Text>
            {/* A swept-up table has no outcome to report — it did not end, it
                stopped — so this says what happened to it instead, and that
                nothing was lost. The winner of a deal played half an hour ago
                is not the news. */}
            <Text testID="match-over-outcome" style={styles.overOutcome}>
              {wasAbandoned ? t('match.abandoned') : outcome}
            </Text>
            {/* Why there is no resume above. Only on a swept-up table, and
                only when somebody is actually missing — a table nobody is
                waiting for has no one to name. */}
            {wasAbandoned && !canResume && awayNames.length ? (
              <Text testID="match-over-waiting" style={styles.overOutcome}>
                {t('match.abandonedWaitingFor', { names: awayNames.join(', ') })}
              </Text>
            ) : null}
            <View style={styles.overActions}>
              {/* Carrying on beats starting over, so it goes first and takes
                  the ring. Offered exactly where the server will honour it —
                  which on a table with other people at it means once they are
                  all back, since reviving it while somebody is away would
                  restart a game they had counted as over. */}
              {canResume ? (
                <Pressable
                  testID="match-over-resume"
                  accessibilityState={{ disabled: resuming }}
                  disabled={resuming}
                  onPress={resume}
                  style={[styles.overButton, resuming && styles.overButtonBusy]}
                >
                  <Attention active={!resuming} radius={8} />
                  <Text style={styles.overButtonText}>
                    {resuming ? t('match.resuming') : t('match.resume')}
                  </Text>
                </Pressable>
              ) : null}
              {againstBotsAlone ? (
                <Pressable
                  testID="match-over-again"
                  accessibilityState={{ disabled: startingAgain }}
                  disabled={startingAgain}
                  onPress={playAgain}
                  style={[styles.overButton, startingAgain && styles.overButtonBusy]}
                >
                  {/* The same ring the way on gets between rounds: a finished
                      match leaves one thing to do too. Not on a swept-up
                      table, where resuming is the offer being pointed at and a
                      second ring would point at nothing. */}
                  <Attention active={!startingAgain && !wasAbandoned} radius={8} />
                  <Text style={styles.overButtonText}>
                    {startingAgain ? t('match.settingUp') : t('match.playAgain')}
                  </Text>
                </Pressable>
              ) : null}
              <Pressable
                testID="match-over-leave"
                onPress={() => router.replace('/lobby/games')}
                style={styles.overButtonQuiet}
              >
                <Text style={styles.overButtonQuietText}>{t('match.backToGames')}</Text>
              </Pressable>
            </View>
            {againError || resumeError ? (
              <Text testID="match-over-error" style={styles.overError}>
                {againError || resumeError}
              </Text>
            ) : null}
          </Animated.View>
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
          <View style={styles.ending} {...ending.anchor('results')}>
            <RoundResults
              log={state.rounds!}
              players={state.players}
              standings={state.standings}
              viewerId={viewerId}
            />
            {state.status === 'completed' ? <LifetimeRecord moduleId={state.moduleId} /> : null}
          </View>
        ) : null}

        {/* Between rounds the one control the module still offers belongs
            here, with the settlement it acts on, rather than under a hand
            that cannot be played. See `controlsPanel`. */}
        {paused ? <View {...ending.anchor('wayOn')}>{controlsPanel}</View> : null}

        <BoardLayout
          state={state}
          viewerId={viewerId}
          styles={styles}
          zonePanelProps={zonePanelProps}
          dropProps={dropProps}
          hand={handPanel}
          controls={
            paused ? null : <View {...opening.anchor('controls')}>{controlsPanel}</View>
          }
          tableAnchor={opening.anchor('table')}
        />

      </ScrollView>

      {/* Why a move was refused: the reason, the rule behind it, and the move
          to make instead. Opened from a greyed-out control's reason line, a
          refused drop, or a submission the server turned down — one component
          for all three, because they are one question. */}
      <WhySheet
        refusal={explaining}
        ruleIndex={ruleIndex}
        offers={state.legalActions}
        players={state.players}
        onSend={send}
        onClose={() => setExplaining(null)}
        onOpenRules={(ruleId) => {
          setExplaining(null);
          router.push({
            pathname: '/rules',
            params: {
              moduleId: state.moduleId,
              variation: state.variation ?? '',
              options: JSON.stringify(state.options ?? {}),
              highlight: ruleId,
            },
          });
        }}
      />
      </SafeAreaView>

      {/* The air above the board: cards currently travelling between zones.
          Above everything, touchable by nothing — see FlightLayer. */}
      <FlightLayer
        flights={flightsInAir}
        rectFor={drops.rectFor}
        measure={drops.measure}
        onDone={landFlight}
      />

      {/* Two seconds saying a round or the match has ended, over everything
          and touchable by nothing — the board underneath takes a tap during
          those two seconds exactly as it would have. Last in the tree so it
          is on top; see `ResultsFlash` for why it is not a dialog. */}
      <ResultsFlash
        visible={flash.visible}
        kind={flash.kind}
        log={state.rounds}
        players={state.players}
        standings={state.standings}
        status={view.status}
        winners={winners}
        viewerId={viewerId}
      />
    </View>
  );
}

/**
 * The refusal a marked card is heading for, if a control is already greyed
 * out for one.
 *
 * A mark and a refusal are the same fact at two different moments — "you owe
 * this card to your lay-down" and "you can't discard, that card is owed" —
 * so the mark borrows the refusal's rules and remedy rather than the module
 * shipping a second copy of them. Nothing found means the mark stands on its
 * own wording, which is still a sentence.
 */
function refusalBehindBadge(offers: ActionOffer[]): Pick<Refusal, 'ruleIds' | 'remedy' | 'remedyOfferId'> {
  const refused = offers.find((o) => !o.enabled && (o.ruleIds?.length || o.remedy));
  if (!refused) return {};
  return { ruleIds: refused.ruleIds, remedy: refused.remedy, remedyOfferId: refused.remedyOfferId };
}
