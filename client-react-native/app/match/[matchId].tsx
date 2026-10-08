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

import type { ActionOffer, MatchAction, Zone } from '@/src/api/matchTypes';
import { POSITION_PARAM, offerGroupKey, submissionFor } from '@/src/api/matchTypes';
import { Attention } from '@/src/components/match/Attention';
import { BoardLayout, matchStyles } from '@/src/components/match/BoardLayout';
import { DeckProvider } from '@/src/lib/deck';
import { FlightLayer, type QueuedFlight } from '@/src/components/match/FlightLayer';
import { HandZone } from '@/src/components/match/HandZone';
import { LifetimeRecord } from '@/src/components/match/LifetimeRecord';
import { OfferBar, OfferGlance, type OfferParams } from '@/src/components/match/OfferBar';
import { Panel } from '@/src/components/match/Panel';
import { ResultsFlash } from '@/src/components/match/ResultsFlash';
import { RoundResults } from '@/src/components/match/RoundResults';
import { ScoreSheet } from '@/src/components/match/ScoreSheet';
import { InviteBackSheet } from '@/src/components/match/InviteBackSheet';
import { TableCode } from '@/src/components/match/TableCode';
import { TableSurface } from '@/src/components/match/TableSurface';
import { useSession } from '@/src/context/SessionContext';
import { useSeatSession } from '@/src/hooks/useSeatSession';
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
  liftKey,
  positionAt,
  readyWith,
  refusalAt,
  someOfferDroppable,
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
import { cardsForSelection, slotsForCards, slotsForDrag, toggleSelection } from '@/src/lib/hand';
import { nextMarks, NO_MARKS, type ChangeMarks } from '@/src/lib/changes';
import { reasonText, t } from '@/src/lib/i18n';
import { ApiError } from '@/src/api/client';
import { savePendingDestination } from '@/src/lib/pendingDestination';
import { dealUrlFor, shareInviteLink } from '@/src/lib/inviteLink';
import { moduleName } from '@/src/lib/gameLabels';
import { routeForMatch } from '@/src/lib/matchRoute';
import { WhySheet, type Refusal } from '@/src/components/match/WhySheet';
import { useRuleIndex } from '@/src/hooks/useRuleIndex';
import { useSkinControls } from '@/src/hooks/useSkin';
import { factText, label, playerName } from '@/src/lib/labels';
import { turnStep } from '@/src/lib/turnStep';
import { dragLayer } from '@/src/theme';
import { AddToCircle } from '@/src/notify/AddToCircle';
import { SameDeal } from '@/src/components/match/SameDeal';

/** How long a player may hold the move before the likeliest control is ringed. */
const IDLE_NUDGE_MS = 20_000;

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
  const own = useSession();
  // A seat taken through a seat link plays this table — and only this one —
  // as that seat, on a device that may be somebody else entirely, or nobody.
  const seatLink = useSeatSession(matchId ? String(matchId) : undefined, own.session);
  const session = seatLink.seat ?? own.session;
  const client = seatLink.client ?? own.client;
  const loading = own.loading || !seatLink.loaded;
  const { offline } = own;
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

  const { state, error, connected, send, clearError } = useMatchSocket(url, client);
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
  const dragRef = useRef<{ slotIds: string[]; cards: string[]; fromTable?: boolean } | null>(null);
  const [drag, setDrag] = useState<{ cards: string[]; fromTable?: boolean } | null>(null);
  const [hoveredDrop, setHoveredDrop] = useState<string | null>(null);
  // The refusal currently being explained, if any. One at a time: a sheet is
  // an answer to a question a player just asked, and the last one asked is
  // the one they meant.
  const [explaining, setExplaining] = useState<Refusal | null>(null);
  // Whose score is being explained, and from which round's cell if any.
  const [scoreOf, setScoreOf] = useState<{ playerId: string; round?: number } | null>(null);
  // Amounts dialled into the controls and not yet sent. Held here, not in the
  // bar, because the bar unmounts when its panel collapses and the collapsed
  // rail's pills send the same offers — both read this one store.
  const [offerParams, setOfferParams] = useState<OfferParams>({});
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
  // A run picked up off the table by a press — solitaire's cards are on the
  // table, not in a hand — and waiting for the player to say where it goes.
  // The cards are the offer's own `source.submit`, never worked out here.
  const [tablePick, setTablePick] = useState<string[] | null>(null);
  // Whether a tap on the status dot has opened its explanation — declared
  // before the `!state` early return below so hook order stays fixed
  // whether or not the socket has delivered a state yet.
  const [statusExplainerOpen, setStatusExplainerOpen] = useState(false);
  const [inviteBackOpen, setInviteBackOpen] = useState(false);
  // Setting up the same table again, and whatever went wrong trying — declared
  // up here for the same reason: hooks may not be conditional.
  const [startingAgain, setStartingAgain] = useState(false);
  // A finished game's deal, sent on: the link, and whether it got as far as a
  // share sheet or the clipboard. When it did not, the link is shown to copy.
  const [sentDeal, setSentDeal] = useState<{ url: string; handedOver: boolean } | null>(null);
  const [sendingDeal, setSendingDeal] = useState(false);
  const [againError, setAgainError] = useState('');
  // "No thanks" to somebody else's rematch: the banner goes back to its own
  // offers. Held here rather than on the server's answer, because the
  // reservation it gives up belongs to the other table, not to this state.
  const [declinedRematch, setDeclinedRematch] = useState(false);
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

  // Groups somebody else changed since this player last acted, compared board
  // to board the same way flights are (see `src/lib/changes.ts`). Kept in its
  // own ref rather than sharing `boardRef`, whose update order the flights
  // depend on.
  const [changeMarks, setChangeMarks] = useState<ChangeMarks>(NO_MARKS);
  const marksBoardRef = useRef<BoardLike | null>(null);
  useEffect(() => {
    if (!state) return;
    const next: BoardLike = { zones: state.view?.zones ?? [], seats: state.view?.seats ?? [] };
    const prev = marksBoardRef.current;
    marksBoardRef.current = next;
    setChangeMarks((was) => nextMarks(was, prev, next, viewerId));
  }, [state, viewerId]);

  // A player who has had the move for a while without making it gets the
  // first control on offer ringed: a suggestion of where to start, for the
  // moment someone is stuck rather than thinking.
  const [idle, setIdle] = useState(false);
  // The move this seat's own bot would make, when the player asked for one —
  // and why not, when the server said no. Both belong to the board they were
  // asked about, so the next board clears them.
  const [hint, setHint] = useState<MatchAction | null>(null);
  const [hintRefusal, setHintRefusal] = useState<string | null>(null);
  useEffect(() => {
    setHint(null);
    setHintRefusal(null);
    // A pick belongs to the board it was made on.
    setTablePick(null);
  }, [state]);
  useEffect(() => {
    setIdle(false);
    if (!state?.legalActions?.some((o) => o.enabled)) return;
    const timer = setTimeout(() => setIdle(true), IDLE_NUDGE_MS);
    return () => clearTimeout(timer);
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
                onPress={() => router.dismissTo('/')}
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

  // Cards an offer lifts straight off the table: the head of the run it names
  // as `source.submit`, in a group or a pile that is nobody's hand. Read off
  // the offers and nothing else — which card heads a movable run is a rule,
  // and the module has already answered it by offering the move.
  const handIds = new Set(myHands.map((z) => z.id));
  const liftOffers = state.legalActions.filter(
    (o) =>
      o.enabled &&
      !!o.source?.submit?.length &&
      (!!o.source.meldId || (!!o.source.zoneId && !handIds.has(o.source.zoneId))),
  );
  const liftable = new Set(
    liftOffers.map((o) => liftKey(o.source!.meldId ?? o.source!.zoneId!, o.source!.submit![0]!)),
  );
  const liftsOf = (card: string) => liftOffers.filter((o) => o.source!.submit![0] === card);

  const canAct = state.legalActions.some((o) => o.enabled);
  const step = turnStep(state.legalActions);
  // Whose turn it is when it is somebody else's: the first thing a player
  // asking "why can't I?" needs, so a not-your-turn refusal names them.
  const turnHolder = state.view?.seats?.find((s) => s.active && s.playerId !== viewerId);
  const turnHolderName = turnHolder ? playerName(state.players, turnHolder.playerId) : undefined;
  const hintsAllowed = (state.options?.hints ?? 1) !== 0;
  // Asks the server what this seat's bot would do, then sets the board up for
  // it without doing it: the cards picked, the target aimed at, and the
  // control ringed. The player still presses it — or doesn't.
  const askHint = async () => {
    setHintRefusal(null);
    try {
      const { action } = await client.hint(String(matchId));
      setHint(action);
      const offer = state.legalActions.find((o) => o.id === action.offerId);
      if (offer && liftOffers.includes(offer) && action.cards?.length) {
        setTablePick(action.cards);
      } else if (action.cards?.length) {
        const slots = slotsForCards(heldSlots, action.cards);
        if (slots.size) {
          setSelected(slots);
          setSelectionIsAuto(false);
        }
      }
      const aim = offer?.target?.meldId;
      if (aim) setArmedMeldId(aim);
    } catch (e) {
      setHint(null);
      setHintRefusal(e instanceof ApiError && e.code ? e.code : 'ERROR');
    }
  };
  const hintOffer = hint ? state.legalActions.find((o) => o.id === hint.offerId) : undefined;
  const explain = (r: Refusal) =>
    setExplaining(
      r.code === 'NOT_YOUR_TURN' && !r.labelKey && turnHolderName
        ? { ...r, labelKey: 'why.notYourTurnWho', params: { name: turnHolderName } }
        : r,
    );

  // Everywhere the cards in flight could be let go of. Derived from the offer
  // list on every drag, which is why a game added tomorrow gets drag and drop
  // without this screen being edited: an offer that says which cards it takes
  // and where it lands *is* a drop target.
  // Every place the cards in flight land on — including the ones that would
  // refuse them, which now carry the reason instead of being dropped from the
  // list. `takeableSpots` is what lights up and what a release sends;
  // `refusalAt` is what a release anywhere else says.
  // A run lifted off the table goes only where an offer for that very run
  // says: every other offer names other cards, and would only light up as
  // refusing.
  const spotsFor = (cards: string[], fromTable?: boolean) =>
    dropSpotsFor(fromTable ? liftsOf(cards[0] ?? '') : state.legalActions, cards);
  const liveSpots = drag ? spotsFor(drag.cards, drag.fromTable) : [];
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
  const pendingSpots: DropSpot[] = tablePick
    ? dropSpotsFor(liftsOf(tablePick[0] ?? ''), tablePick)
    : selectedCards.length > 0
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
    !drag && selectedCards.length === 0 && !pendingGroupKey && !tablePick
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


  // A card on the table pressed: with one place to go it goes there, the way
  // a solitaire player expects a tap to play; with several, they light up and
  // the next press says which. A target aimed at first decides it outright.
  const liftCard = (card: string) => {
    const offers = liftsOf(card);
    if (!offers.length) return;
    const aimed = armedMeldIdLive ? offers.filter((o) => o.target?.meldId === armedMeldIdLive) : [];
    const only = aimed.length === 1 ? aimed[0] : offers.length === 1 ? offers[0] : undefined;
    if (only) {
      const action = submissionFor(only, { cards: only.source!.submit });
      if (!action) return;
      send(action);
      clearSelection();
      setTablePick(null);
      return;
    }
    clearSelection();
    setPendingGroupKey(null);
    setTablePick((was) => (was?.[0] === card ? null : offers[0]!.source!.submit!));
  };

  // The same run carried by hand rather than picked by a press.
  const beginTableDrag = (card: string) => {
    const offer = liftsOf(card)[0];
    if (!offer) return;
    const cards = offer.source!.submit!;
    setTablePick(null);
    dragRef.current = { slotIds: [], cards, fromTable: true };
    setDrag({ cards, fromTable: true });
    drops.measure();
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
    const spots = takeableSpots(spotsFor(current.cards, current.fromTable));
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

    const spots = spotsFor(current.cards, current.fromTable);
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

    // A target lit for a selection that is not a whole submission yet. A drag
    // gathers into one of these — `endDrag` keeps the cards and waits for the
    // next — but a press has nothing left to gather: the cards are already
    // picked, so the same branch here would do literally nothing, which is
    // what it did. The board would light up, invite the tap, and swallow it,
    // with `submissionFor` refusing the short submission two lines later and
    // the press dying on `if (!action) return`. Say what the control beside it
    // says instead, in the same words.
    if (!spot.ready) {
      const fit = readyWith(offer, selectedCards);
      if (!fit.ok) setExplaining({ labelKey: fit.labelKey, params: fit.params });
      return;
    }

    // An empty selection has to travel as `undefined`, not `[]` — an offer
    // built with no cards named falls back to its own one-tap default, the
    // same way a bare press on a lone offer of the same shape already does;
    // an empty array is a *chosen* zero, and settles on nothing at all.
    const picked = tablePick ?? selectedCards;
    const action = submissionFor(offer, { cards: picked.length ? picked : undefined });
    if (!action) return;

    const rect = drops.rectFor(elementId);
    const position = rect ? positionAt(spot.positions, pageY, rect) : spot.positions?.[0];
    if (position) action.params = { ...(action.params ?? {}), [POSITION_PARAM]: position };

    send(action);
    clearSelection();
    setPendingGroupKey(null);
    setTablePick(null);
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
    changedGroups: changeMarks,
    liftable,
    picked: tablePick?.[0] ?? null,
    onLift: liftCard,
    onLiftDragStart: beginTableDrag,
    onDragMove: moveDrag,
    onDragEnd: endDrag,
  };

  // The same table again, with the same people: same game, variation and
  // options, the same bots down to their names, and a seat held for everybody
  // else who played. The server decides all of it — including that a second
  // press, from anyone at this table, sits down at the first one's rematch
  // rather than opening another. It answers with where to go: a lobby when
  // there are people to wait for, a dealt table when there are none.
  const playAgain = async () => {
    setStartingAgain(true);
    setAgainError('');
    try {
      const next = await client.rematch(String(matchId));
      router.replace(routeForMatch(next.status, next.hostId === viewerId, next.matchId));
    } catch (e) {
      const code = e instanceof ApiError ? e.code : undefined;
      setAgainError(reasonText(code, e instanceof Error ? e.message : String(e)));
      setStartingAgain(false);
    }
  };

  // The same cards again, at a new table — offered where the server says a
  // game may be (`canDealAgain`), which is a game played alone.
  const dealAgain = async () => {
    setStartingAgain(true);
    setAgainError('');
    try {
      const next = await client.dealAgain(String(matchId));
      router.replace(routeForMatch(next.status, next.hostId === viewerId, next.matchId));
    } catch (e) {
      const code = e instanceof ApiError ? e.code : undefined;
      setAgainError(reasonText(code, e instanceof Error ? e.message : String(e)));
      setStartingAgain(false);
    }
  };

  // The same cards for somebody else. The server mints the link and seals the
  // game it came from inside it, so nothing here — or in the link — says
  // which game that was.
  const sendDeal = async () => {
    setSendingDeal(true);
    setAgainError('');
    try {
      const { token } = await client.dealLink(String(matchId));
      const url = dealUrlFor(token);
      const handedOver = url
        ? await shareInviteLink(url, t('match.sendDealMessage', { game: moduleName(state.moduleId) }))
        : false;
      setSentDeal({ url, handedOver });
    } catch (e) {
      const code = e instanceof ApiError ? e.code : undefined;
      setAgainError(reasonText(code, e instanceof Error ? e.message : String(e)));
    } finally {
      setSendingDeal(false);
    }
  };

  // Giving the held seat back, so the host is not left waiting. Best-effort:
  // if it does not reach the server the seat is let go anyway once the host
  // deals, and there is nothing this player could do about it from here.
  const declineRematch = () => {
    setDeclinedRematch(true);
    if (state.rematch) client.declineRematch(state.rematch.matchId).catch(() => {});
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
  // A game played alone that nobody won was not lost to anybody: it was not
  // solved. Told apart by the table having one seat — a fact about the table,
  // not about which game is on it.
  const outcome =
    winners.length === 0
      ? state.players.length === 1
        ? t('match.notSolved')
        : t('match.nobodyWon')
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

  // Only somebody who played here can ask to play it again; a spectator gets
  // the way out and nothing else.
  const seatedHere = state.players.some((p) => p.id === viewerId && !p.isAI);
  // Somebody else from this table has asked for a rematch and is holding this
  // viewer a seat at it — the one offer worth more than starting another.
  const rematchOffer =
    seatedHere && state.rematch && state.rematch.hostId !== viewerId && !declinedRematch
      ? state.rematch
      : undefined;

  // The table was set aside by the sweeper rather than played to the end. A
  // different ending, and it needs different words and a different offer — it
  // is the one ending that can be undone.
  const wasAbandoned = state.status === 'abandoned';

  // Whether this table can be picked up, as the server answers it — never
  // worked out here.
  //
  // This screen used to decide for itself, using "every other seat is a bot"
  // as a stand-in for the old server rule. The two then diverged in the worst
  // direction: a game between two people who were both back and both looking
  // at the board was offered nothing at all, on a banner that told them the
  // cards were exactly where they had left them. Now the button appears when
  // the server would honour it, and when it would not, the line below says
  // who everyone is waiting for instead of leaving them to guess.
  const canResume = wasAbandoned && !!state.canResume;
  const awayNames = (state.awayPlayers ?? [])
    .map((id) => playerName(state.players, id))
    .filter(Boolean);

  // The join code, kept in view for as long as the table can still be played,
  // because it is how a seated player who left gets back: the join call lets
  // an existing seat in whatever the table is doing. Only for a player seated
  // here, and only where somebody else could need it — a table against bots
  // has nobody to send it to.
  const tableCode =
    seatedHere &&
    state.status !== 'completed' &&
    state.players.some((p) => !p.isAI && p.id !== viewerId)
      ? (state.joinCode ?? '')
      : '';

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
          params={offerParams}
          onParamsChange={setOfferParams}
          onAmbiguous={(groupKey) => {
            setPendingGroupKey(groupKey);
            drops.measure();
          }}
          onExplain={explain}
          testID="controls-summary"
        />
      }
    >
      {/* What to do now, in one line, from the same offers the controls
          below are drawn from — see `src/lib/turnStep.ts`. */}
      {step && !paused ? (
        <View style={styles.stepRow}>
          <Text testID="turn-step" style={step.obligation ? styles.stepObligation : styles.step}>
            {step.obligation
              ? factText(step.obligation, state.players)
              : t('step.yourTurn', {
                  moves: step.moves.map((o) => label(o.labelKey ?? `verb.${o.verb}`) || o.verb).join(' · '),
                })}
          </Text>
          {hintsAllowed ? (
            <Pressable testID="hint-button" accessibilityRole="button" onPress={askHint} style={styles.hintButton}>
              <Text style={styles.hintButtonText}>{t('hint.button')}</Text>
            </Pressable>
          ) : null}
        </View>
      ) : null}
      {hint && hintOffer ? (
        <Text testID="hint-line" style={styles.hintLine}>
          {factText(
            {
              labelKey: hint.cards?.length ? 'hint.line' : 'hint.lineNoCards',
              params: { move: hintOffer.labelKey ?? `verb.${hintOffer.verb}`, cards: hint.cards ?? [] },
            },
            state.players,
          )}
        </Text>
      ) : null}
      {hintRefusal ? (
        <Text testID="hint-refused" style={styles.muted}>
          {reasonText(hintRefusal, hintRefusal)}
        </Text>
      ) : null}
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
        params={offerParams}
        onParamsChange={setOfferParams}
        onExplain={explain}
        // Between rounds the module offers one thing: go on. Said here as
        // "the table is waiting on this bar" rather than as any offer's name,
        // so the bar rings whatever the one thing turns out to be.
        urgent={paused}
        nudge={idle}
        hintOfferId={hint?.offerId}
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
          {turnHolderName ? t('match.waitingForName', { name: turnHolderName }) : t('match.waitingForPlayer')}
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
  // Whether the hint may say a card goes "onto the board" — true only while
  // some offer would take one dropped there. Read from the offers, never the
  // game: a game whose cards only ever leave through a button gets a hint
  // about rearranging and nothing more.
  const canDropOnBoard = someOfferDroppable(state.legalActions);
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
        canDropOnBoard={canDropOnBoard}
        badges={badgesFor(z)}
        onPressBadge={(card, badgeKeys) =>
          setExplaining({
            labelKey: badgeKeys[0],
            params: { card },
            // The rules and the way out behind the mark come from
            // whichever offer this card is about to be refused by —
            // asked for on long-press, rather than the module having
            // to say the same thing twice.
            ...refusalBehindBadge(state.legalActions, card),
          })
        }
        {...zonePanelProps(z.id)}
      />
    ))}
  </View>
  );

  const screen = (
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
          // The join code rides in the bar too, for the same reason the
          // status does: the board scrolls to the hand on arrival, and a code
          // at the top of the board would be gone before anyone looked for it.
          // A sibling of the status rather than inside its Pressable, so a
          // press on the code shares the link instead of toggling the
          // explainer.
          headerTitle: () => (
            <View style={styles.headerTitleGroup}>
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
              {tableCode ? (
                <TableCode
                  joinCode={tableCode}
                  onPress={() => setInviteBackOpen(true)}
                  chipStyle={styles.tableCode}
                  textStyle={styles.tableCodeText}
                />
              ) : null}
            </View>
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
                // in app/lobby/setup.tsx for why a `+` chain fails to typecheck
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
            {/* And how to get them here, on whatever device they have now:
                the same sheet the code in the bar opens. */}
            {wasAbandoned && !canResume && awayNames.length && tableCode ? (
              <>
                <Text testID="match-over-send-code" style={styles.overOutcome}>
                  {t('match.abandonedSendCode')}
                </Text>
                <Pressable
                  testID="match-over-invite-back"
                  onPress={() => setInviteBackOpen(true)}
                  style={styles.overButton}
                >
                  <Text style={styles.overButtonText}>{t('invite.backTitle')}</Text>
                </Pressable>
              </>
            ) : null}
            {/* A deal played again says so: a score made on cards already
                seen is not a first attempt. */}
            {state.repeatDeal && !wasAbandoned ? (
              <Text testID="match-over-repeat" style={styles.overOutcome}>
                {t('match.repeatDeal')}
              </Text>
            ) : null}
            {rematchOffer ? (
              <Text testID="match-over-rematch-offer" style={styles.overOutcome}>
                {t('match.rematchOffer', { name: playerName(state.players, rematchOffer.hostId) })}
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
              {seatedHere ? (
                <Pressable
                  testID={rematchOffer ? 'match-over-join-rematch' : 'match-over-again'}
                  accessibilityState={{ disabled: startingAgain }}
                  disabled={startingAgain}
                  onPress={playAgain}
                  style={[styles.overButton, startingAgain && styles.overButtonBusy]}
                >
                  {/* The same ring the way on gets between rounds: a finished
                      match leaves one thing to do too. Not on a swept-up
                      table, where resuming is the offer being pointed at and a
                      second ring would point at nothing — unless somebody is
                      holding this player a seat, which is the news. */}
                  <Attention active={!startingAgain && (!wasAbandoned || !!rematchOffer)} radius={8} />
                  <Text style={styles.overButtonText}>
                    {startingAgain
                      ? t('match.settingUp')
                      : rematchOffer
                        ? t('match.joinRematch')
                        : t('match.playAgain')}
                  </Text>
                </Pressable>
              ) : null}
              {seatedHere && state.canDealAgain ? (
                <Pressable
                  testID="match-over-deal-again"
                  accessibilityState={{ disabled: startingAgain }}
                  disabled={startingAgain}
                  onPress={dealAgain}
                  style={[styles.overButton, startingAgain && styles.overButtonBusy]}
                >
                  <Text style={styles.overButtonText}>{t('match.dealAgain')}</Text>
                </Pressable>
              ) : null}
              {seatedHere && state.canDealAgain ? (
                <Pressable
                  testID="match-over-send-deal"
                  accessibilityState={{ disabled: sendingDeal }}
                  disabled={sendingDeal}
                  onPress={sendDeal}
                  style={[styles.overButton, sendingDeal && styles.overButtonBusy]}
                >
                  <Text style={styles.overButtonText}>{t('match.sendDeal')}</Text>
                </Pressable>
              ) : null}
              {rematchOffer ? (
                <Pressable
                  testID="match-over-decline-rematch"
                  onPress={declineRematch}
                  style={styles.overButtonQuiet}
                >
                  <Text style={styles.overButtonQuietText}>{t('match.declineRematch')}</Text>
                </Pressable>
              ) : null}
              <Pressable
                testID="match-over-leave"
                onPress={() => router.dismissTo('/')}
                style={styles.overButtonQuiet}
              >
                <Text style={styles.overButtonQuietText}>{t('match.backToGames')}</Text>
              </Pressable>
            </View>
            {/* What became of the link — handed to a share sheet or copied,
                or neither — and the link itself, either way: "copied" is
                invisible, and people like to see what they are about to paste. */}
            {sentDeal ? (
              <View testID="match-over-deal-link">
                <Text style={styles.overOutcome}>
                  {sentDeal.handedOver ? t('match.sendDealDone') : t('match.sendDealCopy')}
                </Text>
                {sentDeal.url ? (
                  <Text selectable testID="match-over-deal-url" style={styles.overOutcome}>
                    {sentDeal.url}
                  </Text>
                ) : null}
              </View>
            ) : null}
            {againError || resumeError ? (
              <Text testID="match-over-error" style={styles.overError}>
                {againError || resumeError}
              </Text>
            ) : null}
            {/* Somebody worth playing again: offered their place in the
                circle while the game is still warm. Online tables only —
                the players at a table on a phone in the room are that
                phone's guests, not accounts the online circle knows. */}
            {/* Everybody else who played these cards, once this player has. */}
            {state.status === 'completed' && state.canDealAgain && seatedHere && !offline ? (
              <SameDeal client={client} matchId={String(matchId)} palette={skin.colors} />
            ) : null}
            {state.status === 'completed' && !offline ? (
              <AddToCircle players={state.players} viewerId={viewerId} palette={skin.colors} />
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
              onOpenScore={(playerId, round) => setScoreOf({ playerId, round })}
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
          // Only where the game keeps rounds: a score with no account behind
          // it — Prší's, which is a card count — has nothing to open.
          onOpenScore={state.rounds ? (playerId) => setScoreOf({ playerId }) : undefined}
        />

      </ScrollView>

      {/* Why a move was refused: the reason, the rule behind it, and the move
          to make instead. Opened from a greyed-out control's reason line, a
          refused drop, or a submission the server turned down — one component
          for all three, because they are one question. */}
      <ScoreSheet
        subjectId={scoreOf?.playerId ?? null}
        focusRound={scoreOf?.round}
        log={state.rounds}
        seats={view.seats ?? []}
        players={state.players}
        standings={state.standings}
        viewerId={viewerId}
        onClose={() => setScoreOf(null)}
      />

      {tableCode ? (
        <InviteBackSheet
          open={inviteBackOpen}
          onClose={() => setInviteBackOpen(false)}
          client={client}
          matchId={state.matchId}
          joinCode={tableCode}
          inviteUrl={state.inviteUrl}
          players={state.players.filter((p) => !p.isAI && p.id !== viewerId)}
        />
      ) : null}

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
  // Which pack the cards are drawn from — see src/lib/deck.ts.
  return <DeckProvider deck={state.deck}>{screen}</DeckProvider>;
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
function refusalBehindBadge(
  offers: ActionOffer[],
  card: string,
): Pick<Refusal, 'ruleIds' | 'remedy' | 'remedyOfferId'> {
  // A card an enabled offer turns down by name — the one just taken off the
  // pile — is its own refusal, and a closer answer than any greyed-out offer.
  for (const o of offers) {
    const own = o.enabled ? o.source?.refused?.find((r) => r.card === card) : undefined;
    if (own && (own.ruleIds?.length || own.remedy)) return { ruleIds: own.ruleIds, remedy: own.remedy };
  }
  const refused = offers.find((o) => !o.enabled && (o.ruleIds?.length || o.remedy));
  if (!refused) return {};
  return { ruleIds: refused.ruleIds, remedy: refused.remedy, remedyOfferId: refused.remedyOfferId };
}
