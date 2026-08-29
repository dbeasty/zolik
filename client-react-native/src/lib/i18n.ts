/**
 * Message keys and locale bundles.
 *
 * The server never sends a rendered sentence. It sends stable keys — engine
 * error codes (`NOT_YOUR_TURN`), option names, and structured facts like the
 * contract's sets/runs counts — and the wording lives here. That split is what
 * makes a Czech UI possible without touching the server, and it is the reason
 * Phase 1 shipped `whyNot` as a code rather than a message.
 *
 * Three rules this module keeps:
 *
 *  1. **A missing translation degrades, never blanks.** The lookup falls back
 *     locale → English → the caller's fallback → the key itself. A player
 *     seeing an untranslated English string is a bad day; a player seeing an
 *     empty control is a bug report.
 *  2. **Every key in one bundle exists in all of them.** Asserted by a test,
 *     not by review — a half-translated locale is how "mostly Czech with
 *     random English" ships.
 *  3. **No rule knowledge.** This file maps keys to words. It never decides
 *     what is legal, what a card is worth, or which profile is in play.
 */

export type Locale = 'en' | 'cs';

export const LOCALES: { id: Locale; label: string }[] = [
  { id: 'en', label: 'English' },
  { id: 'cs', label: 'Čeština' },
];

/**
 * Interpolation params. Values are substituted into `{name}` placeholders.
 */
export type Params = Record<string, string | number>;

// The English bundle is the reference: every other locale is checked against
// its key set. Keys are namespaced by what they describe, so an unfamiliar one
// is at least placeable.
const en: Record<string, string> = {
  // --- engine error codes (rules.RulesErrorCode) ---------------------------
  'err.NOT_YOUR_TURN': "It's not your turn",
  'err.WRONG_PHASE': 'Not available right now',
  'err.MUST_DRAW_FIRST': 'Draw a card before melding',
  'err.GAME_SUSPENDED': 'The game is paused',
  'err.GAME_NOT_ACTIVE': 'The game is not running',
  // --- runtime refusals, which are not the rules saying no -----------------
  //
  // Both were reachable long before they were worded, and a player who met
  // one was shown the code in capitals or, worse, nothing at all: an action
  // sent while the socket was down simply vanished, which reads as a legal
  // move being silently refused.
  'err.MATCH_NOT_ACTIVE': 'The table is paused — waiting for a player to reconnect',
  'err.NOT_CONNECTED': 'Not connected to the table — reconnecting, then try again',
  // Why the "start the next round" control is greyed out. Without wording the
  // shell printed the code itself, so a player agreeing to go on was answered
  // with ALREADY_READY in capitals.
  'err.ALREADY_READY': 'You are ready',
  'err.NOT_BETWEEN_ROUNDS': 'The round is still being played',
  'err.NOT_AT_THIS_TABLE': 'You are not at this table',
  'err.DISCARD_LOCKED': 'The discard pile is locked for now',
  'err.DISCARD_PILE_EMPTY': 'The discard pile is empty',
  'err.NO_CARDS_LEFT': 'No cards left to draw',
  'err.ROUND_REQ_NOT_MET': 'Lay your own initial meld first',
  'err.NEED_CLEAN_RUN': 'You need a joker-free run on the table before you count as down',
  'err.INCOMPLETE_INITIAL_MELD': 'Finish your lay-down, or undo it, before you discard',
  'err.DISCARD_CARD_NOT_MELDED': 'The card you picked up must go into your meld',
  'err.JOKER_DISCARD_FORBIDDEN': "A joker can't be discarded",
  'err.NOTHING_TO_UNDO': 'Nothing to undo',
  'err.NO_JOKER_IN_MELD': 'No joker in this meld',
  'err.JOKER_SWAP_MISMATCH': "That card doesn't take the joker's place",
  'err.RECLAIMED_JOKER_NOT_MELDED': 'The joker you took off the table must be played into a meld this turn',
  'err.RUN_TOO_LONG': 'That run is already at its full length',
  'err.WRONG_RUN_END': 'That card extends the other end of the run',
  'err.INVALID_MELD': 'No card in your hand fits here',
  'err.CARD_NOT_IN_HAND': 'That card is not in your hand',
  'err.MELD_BELOW_MINIMUM': 'Your melds are still short of the points needed to go down',
  'err.MELD_NO_CONTRIBUTION': "That meld doesn't advance your requirement",
  'err.TOO_MANY_WILDS': 'Too many wild cards in that meld',
  'err.ADJACENT_WILDS': 'Two wild cards cannot sit next to each other',
  'err.ACE_BRIDGE': 'An ace cannot bridge king and two',

  // --- contract phrasing ---------------------------------------------------
  // Built from the structured contract the server sends (sets/runs counts),
  // never from a deal-number lookup table.
  // Whole phrases per count rather than a number plus a pluralised noun:
  // Czech inflects the noun by count (skupina / skupiny / skupin) in a way no
  // "add an s" helper survives, and the counts that actually occur are 1-3.
  // A count outside that falls back to contract.sets.n.
  'contract.sets.1': 'One set',
  'contract.sets.2': 'Two sets',
  'contract.sets.3': 'Three sets',
  'contract.sets.n': '{n} sets',
  'contract.runs.1': 'One run',
  'contract.runs.2': 'Two runs',
  'contract.runs.3': 'Three runs',
  'contract.runs.n': '{n} runs',
  'contract.any': 'Any valid meld',
  'contract.cleanRunOnly': 'Any mix of sets and runs — at least one run must be joker-free',
  'contract.cleanRunSuffix': '{base} — one run must be joker-free',

  // --- written rules ---------------------------------------------------------
  // Full-sentence keys a "see the rules" screen renders, one per module,
  // resolved server-side against the table's actual variation and options —
  // see internal/module/rules.go and each game's rules.go. Shared section
  // titles first, then one block per module in the order the four games were
  // added.
  'zolik.rules.section.goal': 'Goal',
  'zolik.rules.section.setup': 'Setup',
  'zolik.rules.section.turn': 'Your turn',
  'zolik.rules.section.melding': 'Laying down',
  'zolik.rules.section.end': 'How the match ends',
  'zolik.rules.goal':
    "Be the first to empty your hand by laying down valid sets and runs, while scoring as few penalty points as possible in the cards you're still holding when someone else goes out.",
  'zolik.rules.deal': 'Each player is dealt {n} cards.',
  'zolik.rules.meldShapes':
    'A set is {set}+ cards of the same rank; a run is {run}+ consecutive cards of the same suit.',
  'zolik.rules.turn.draw': 'On your turn, draw one card — from the stock, or from the discard pile.',
  'zolik.rules.pickup.topOnly': 'Only the top card of the discard pile may be taken.',
  'zolik.rules.pickup.anyFromPile':
    'Any card in the discard pile may be taken, along with everything stacked above it.',
  'zolik.rules.pickup.locked': "The discard pile can't be drawn from until round {n}.",
  'zolik.rules.pickup.open': 'The discard pile is open from the first round.',
  'zolik.rules.turn.discard': 'End your turn by discarding one card.',
  'zolik.rules.jokers.restricted':
    'A joker can never be discarded, except as the exact card that empties your hand.',
  'zolik.rules.lead.rotate': 'The lead rotates around the table one seat per deal, regardless of who won.',
  'zolik.rules.lead.winner': 'Whoever goes out leads the next deal.',
  'zolik.rules.meldFloor.on':
    "Your first meld (or melds) must total at least {n} natural points before you're down.",
  'zolik.rules.meldFloor.off': "There's no minimum point value on your first meld.",
  'zolik.rules.cleanRun.on':
    'At least one of your runs must be completely joker-free before you count as down.',
  'zolik.rules.cleanRun.off': "Your runs may use jokers freely — no run has to be joker-free.",
  'zolik.rules.contracts.rotating':
    'The match is {n} deals long, and each deal requires its own combination of sets and runs.',
  'zolik.rules.contracts.static': 'Every deal requires the same combination: {sets} sets and {runs} runs.',
  'zolik.rules.end.afterDeals': 'The match ends after {n} deals.',
  'zolik.rules.end.atScore': "The match keeps redealing until someone reaches {n} points — then it's over.",

  'prsi.rules.section.goal': 'Goal',
  'prsi.rules.section.setup': 'Setup',
  'prsi.rules.section.turn': 'Your turn',
  'prsi.rules.section.special': 'Special cards',
  'prsi.rules.section.end': 'How the match ends',
  'prsi.rules.goal': 'Be the first to play every card in your hand.',
  'prsi.rules.deck': 'Played with a {value}-card deck (7 and up).',
  'prsi.rules.deal': 'Each player starts with {n} cards.',
  'prsi.rules.turn.match':
    "Play a card that matches the top card's suit or rank — or draw if you can't.",
  'prsi.rules.turn.draw': 'Drawing ends your turn without a play.',
  'prsi.rules.sevens':
    'Play a 7 and the next player draws two cards, unless they can answer with a 7 of their own.',
  'prsi.rules.aces': "Play an ace and the next player's turn is skipped.",
  'prsi.rules.queens': 'Play a queen and name the suit that continues.',
  'prsi.rules.end': "The match ends the moment someone's hand is empty.",

  'canasta.rules.section.goal': 'Goal',
  'canasta.rules.section.setup': 'Setup',
  'canasta.rules.section.melding': 'Melding',
  'canasta.rules.section.end': 'How the match ends',
  'canasta.rules.goal':
    'Play in partnerships; the first side to reach {n} points wins the match.',
  'canasta.rules.deck': 'Played with {value} cards — two decks plus jokers.',
  'canasta.rules.deal': 'Each player is dealt {n} cards.',
  'canasta.rules.redThrees':
    "A red three in your hand is shown immediately and scores as a bonus — unless your side never completes a canasta, when it counts against you instead.",
  'canasta.rules.canasta': 'A canasta is a meld of {n} or more cards of the same rank.',
  'canasta.rules.meldFloorBands':
    'Your first meld must reach a point minimum that rises with your score: {negative} below zero, {low} up to 1500, {mid} up to 3000, {high} beyond that.',
  'canasta.rules.oneCanastaToGoOut': 'One completed canasta is enough for your side to go out.',
  'canasta.rules.twoCanastasToGoOut':
    'Your side needs two completed canastas before it may go out.',
  'canasta.rules.end':
    'The deal keeps being redealt until one side passes {n} points — then the match is over.',

  'holdem.rules.section.goal': 'Goal',
  'holdem.rules.section.setup': 'Setup',
  'holdem.rules.section.betting': 'Betting',
  'holdem.rules.section.end': 'How the match ends',
  'holdem.rules.goal':
    'Win chips by having the best hand at showdown, or by being the only player left in the hand.',
  'holdem.rules.stack': 'Every seat starts with {n} chips.',
  'holdem.rules.blinds':
    'The small blind is {sb} and the big blind is {bb}, posted before the cards are dealt.',
  'holdem.rules.streets':
    'Betting happens in four rounds — before the flop, and after the flop, turn and river are dealt.',
  'holdem.rules.showdown':
    'Players still in the hand reveal their cards; the best five-card hand wins the pot.',
  'holdem.rules.noLimit':
    'No-limit betting — any bet may be for any amount up to your whole stack.',
  'holdem.rules.lastPlayerStanding': 'The match plays until one seat holds every chip.',
  'holdem.rules.mostChipsWins': 'Whoever holds the most chips when play stops wins the match.',
  'holdem.rules.handLimit': 'Play stops after {n} hands.',

  // --- header --------------------------------------------------------------
  'header.deal': 'Deal {n}',
  'header.gameOf': 'Game {n} of {total}',
  'header.gameOfWithContract': 'Game {n} of {total}: {contract}',

  // --- meld preview --------------------------------------------------------
  'preview.validSet': 'Valid set',
  'preview.validRun': 'Valid run',
  'preview.validMeld': 'Valid meld',
  'preview.notYet': 'Not a meld yet',
  'preview.points': '{shape} · {n} points',
  'preview.pointsWithLaid': '{shape} · {n} + {laid} already laid = {total} points',
  'preview.meetsFloor': '{line} (meets {n} ✓)',
  'preview.needsFloor': '{line} (needs {n} ✗)',
  'preview.becauseOf': '{line} — {reason}',

  // --- discard -------------------------------------------------------------
  // A discard with a meld staged lays that meld first; this is what the
  // player is told when the server refuses it and the whole move is rolled
  // back. {reason} is the engine's own words for the refusal.
  'discard.meldRejected': '{reason} — nothing was discarded, your cards are still staged.',

  // --- what the current selection would send -------------------------------
  // The client's own reason a control is not ready, next to the engine's own
  // `err.*` reasons rather than a second kind of message — a control greyed
  // out for "not your turn" and one greyed out for "you picked two, this
  // takes one" should read as the same kind of thing, not one of them looking
  // broken. See `fits` in `src/lib/drops.ts`.
  'sel.tooMany.1': 'Select just one card',
  'sel.tooMany.n': 'Select at most {n} cards',
  'sel.needMore': 'Select {n} card(s)',
  'sel.notThese': "Those cards can't go here",
  'sel.needsCompany': 'That card needs the ones next to it',

  // --- what a module says happened -----------------------------------------
  // The first module-sent keys in this bundle, and deliberately few. Anything
  // a module sends is still legible without an entry here — `humanise` turns
  // `holdem.seat.stack` into "Stack" — so a key earns a line only when that
  // fallback loses something. These do: what each of them means lives in its
  // *params* (who won, how much, whose cards), and a key on its own has
  // nowhere to put them. Without the entry the player read "Winner", full
  // stop, at the end of a match they had just won.
  'status.winner': 'Won by {winners}',
  'holdem.status.pot': '{winners} won {amount} with {hand}',
  'holdem.status.potUncontested': '{winners} won {amount} — everyone else folded',
  'holdem.status.shown': '{playerId} showed {value}',
  'holdem.prompt.waitingFor': 'Waiting for {playerId}',

  // How a rummy scoreboard was ordered. Both earn a line for the same reason
  // as the keys above: the number lives in the params, and `humanise` would
  // render "Deals Won" with nothing after it.
  'zolik.standing.dealsWon': 'Deals won {n}',
  'zolik.standing.inHand': 'In hand {n}',

  // --- rounds ---------------------------------------------------------------
  // What a round is called, and what one did. Each of these carries its number
  // in the params, so `humanise` alone would render a label with nothing after
  // it.
  // An instruction, not a heading. "Next round" named the thing rather than
  // asking for it, and sat above "Waiting for 1" — which reads as though the
  // player is the one waiting, when the table is waiting for them.
  'round.continue': 'Start the next round',

  'zolik.round.deal': 'Deal',
  'zolik.round.cleanRun': 'One run must be joker-free',
  'canasta.round.deal': 'Deal',
  'canasta.round.concealed': 'Went out concealed',
  'canasta.round.exhausted': 'The deck ran out',
  'canasta.round.meldCards': 'Cards laid {n}',
  'canasta.round.canastas': 'Canastas {n}',
  'canasta.round.redThrees': 'Red threes {n}',
  'canasta.round.goingOut': 'Going out {n}',
  'canasta.round.inHand': 'Caught in hand {n}',
  'holdem.round.hand': 'Hand',
  'holdem.round.pot': 'Pot {n}',
  'holdem.round.uncontested': 'Everyone else folded',
  'seat.ready': 'Ready',



  // --- refusals the other three games raise ---------------------------------
  // Worded here for the same reason Žolíky's are: reasonText falls back to the
  // code, and OfferBar hands it the code as that fallback, so an unworded
  // refusal reaches a player as PILE_FROZEN in capitals. serverKeys.test.ts
  // now fails the build rather than letting the next one through.
  'err.SET_TOO_LARGE': 'A set already has all four suits',
  'err.DISCARD_TAKEN_CARD_FORBIDDEN': 'You can\'t discard the card you just took — play it or keep it',
  'err.CARD_DOES_NOT_FIT': 'That card doesn\'t match the suit or the rank',
  'err.SUIT_REQUIRED': 'Name the suit that continues',
  'err.MUST_ANSWER_DRAW_OR_TAKE': 'Answer with a seven, or take the cards',
  'err.NOTHING_TO_DRAW': 'There is nothing left to draw',
  'err.PILE_EMPTY': 'The pile is empty',
  'err.PILE_BLOCKED': 'The pile is blocked — a black three is on top',
  'err.PILE_FROZEN': 'The pile is frozen — you need two natural cards of the top card\'s rank',
  'err.TOP_CARD_UNUSABLE': 'You can\'t use the top card',
  'err.MELD_CLOSED': 'That meld is complete and closed',
  'err.MELD_TOO_SMALL': 'A meld needs more cards than that',
  'err.MELD_TOO_LARGE': 'That meld can\'t take any more cards',
  'err.MELD_MIXED_RANKS': 'Every card in a meld must be the same rank',
  'err.NOT_ENOUGH_NATURALS': 'A meld needs more natural cards than wild ones',
  'err.RANK_ALREADY_MELDED': 'Your side already has a meld of that rank',
  'err.NOT_YOUR_MELD': 'That meld belongs to the other side',
  'err.NO_SUCH_MELD': 'That meld is not on the table',
  'err.CANNOT_MELD_THREE': 'Threes are never melded',
  'err.CANNOT_DISCARD_RED_THREE': 'A red three can\'t be discarded',
  'err.MUST_KEEP_A_CARD': 'Keep at least one card — you can\'t empty your hand this way',
  'err.MUST_MELD_FIRST': 'Lay your side\'s first meld before doing that',
  'err.INITIAL_MELD_NOT_MET': 'Your first meld is still short of the points needed',
  'err.CANNOT_GO_OUT_YET': 'Your side needs a completed canasta before it can go out',
  'err.NOTHING_TO_CALL': 'There is no bet to call',
  'err.CANNOT_CHECK': 'You can\'t check — there is a bet to answer',
  'err.CANNOT_RAISE': 'You can\'t raise here',
  'err.RAISE_TOO_SMALL': 'A raise has to be at least the last one',
  'err.NOT_ENOUGH_CHIPS': 'You don\'t have that many chips',
  'err.AMOUNT_REQUIRED': 'Say how much',
  'err.AMOUNT_NOT_A_NUMBER': 'That amount isn\'t a number',
  'err.SEAT_NOT_IN_HAND': 'You are not in this hand',
  'err.WRONG_RANK': 'That card is the wrong rank for this',
  'err.MATCH_FULL': 'The table is full',
  'err.MATCH_ALREADY_STARTED': 'The match has already started',
  'err.TOO_FEW_PLAYERS': 'Not enough players yet',
  'err.WRONG_PLAYER_COUNT': 'This game can\'t be played with that many players',
  'err.NOT_THE_HOST': 'Only the host can do that',
  'err.NO_LONGER_WAITING': 'The table is no longer waiting',
  'err.WAITING_ROOM_UNAVAILABLE': 'The waiting room isn\'t available',


  // --- rules the engine has always enforced and never stated ----------------
  // Every one of these was found by the guardrail rather than by review: the
  // validator could refuse a player for it, and no sentence anywhere said so.
  // See server/internal/zolikmod/ruleindex_test.go.
  'zolik.rules.section.layoff': 'Adding to melds',
  'zolik.rules.pickup.obligation': 'Before you\'re down, a card taken from the discard pile must be used in the meld that takes you down this turn.',
  'zolik.rules.pickup.noReturn': 'A card you took from the discard pile can\'t be discarded again on the same turn — play it or keep it.',
  'zolik.rules.wilds.setLimit': 'A set can\'t hold more jokers than natural cards.',
  'zolik.rules.set.maxSize': 'A set can\'t hold more than {n} cards — a joker fills a missing suit, it doesn\'t pad a full one.',
  'zolik.rules.run.maxLength': 'A run can\'t hold more than {n} cards — the ace at the bottom, the twelve ranks above it, and the ace at the top.',
  'zolik.rules.run.aceBridge': 'An ace sits above the king or below the two, never bridging the two ends of a run.',
  'zolik.rules.contracts.contribution': 'Until you\'re down, every meld you lay must be one the deal\'s contract still needs.',
  'zolik.rules.layoff.afterDown': 'You can\'t add to anyone\'s melds until you\'ve laid your own contract.',
  'zolik.rules.layoff.runEnds': 'A card added to a run must continue it at one end or the other.',
  'zolik.rules.jokers.swap': 'A joker in a meld on the table may be bought back with the exact card it stands for.',
  'zolik.rules.jokers.reclaim.on':
    'A joker bought back off the table must be played into a meld the same turn — it can\'t be kept in hand.',
  'zolik.rules.jokers.reclaim.off': 'A joker bought back off the table may be kept in hand.',
  'zolik.rules.deck.reshuffle': 'When the stock runs out the discard pile is shuffled and becomes the new stock; if both are empty, the deal ends.',


  // --- what to do instead ---------------------------------------------------
  // The third layer of a refusal, after the reason and the rule. Sent by the
  // module because it is the only side that knows which card is owed and
  // which way out is on offer; see server/internal/zolikmod/remedy.go.
  'zolik.remedy.meldThePickup': 'Add {card} to your lay-down, or undo the pickup.',
  'zolik.remedy.discardSomethingElse': 'Discard a different card, or play {card} this turn.',
  'zolik.remedy.discardNotAJoker': 'Discard something other than a joker.',
  'zolik.remedy.finishOrUndoLayDown': 'Finish your lay-down, or take it back.',
  'zolik.remedy.needMorePoints': 'You need {n} more points before you can go down.',
  'zolik.remedy.layACleanRun': 'Lay a run with no joker in it.',
  'zolik.remedy.playReclaimedJoker': 'Play {card} into a meld, or undo taking it.',
  'zolik.remedy.goDownFirst': 'Lay your own melds first.',
  'zolik.remedy.drawFirst': 'Draw a card first.',
  'zolik.remedy.drawFromStock': 'Draw from the stock — the discard pile opens in round {n}.',
  'zolik.remedy.drawFromStockEmpty': 'Draw from the stock instead.',


  // --- keys whose meaning lives in their params -----------------------------
  // The shape fallback renders these with the number missing ("Stack", "Pot"),
  // which is why they earn a line where most keys do not.
  'header.contract': 'Needs {sets} sets and {runs} runs',
  'header.contract.cleanRunOnly': 'Needs a joker-free run',
  'header.round': 'Round {n}',
  'header.deck': 'Deck',
  'header.target': 'Target',
  'header.suitInPlay': 'Suit in play',
  'seat.cards': 'Cards',
  'zolik.offer.meld': 'Meld',
  'prompt.pickupMustBeMelded': '{value} came off the discard pile — it has to go into the melds you go down with this turn.',
  'prompt.jokerMustBePlayed': '{value} came off the table — it has to go into a meld before you can end your turn.',
  'prompt.initialMeld': 'Your side\'s first meld must reach {n} points.',
  'prompt.canastasNeeded': 'Your side needs {n} more canastas before it can go out.',
  'prompt.mustDrawOrAnswerSeven': 'Answer with a seven, or draw {n} cards.',
  'prompt.chooseSuit': 'Choose the suit that continues',
  'prompt.skipPending': 'Your turn is skipped',
  'status.lastDeal': 'Team {team} scored {value}',
  'status.teamScore': 'Team {team}: {value}',
  'canasta.offer.rank': 'Rank',
  'canasta.seat.teamScore': 'Team score',
  'canasta.seat.canastas': 'Canastas',
  'holdem.header.pot': 'Pot',
  'holdem.header.street': 'Street',
  'holdem.header.hand': 'Hand',
  'holdem.header.handLimit': 'Hands in all',
  'holdem.header.blinds': 'Blinds',
  'holdem.cost.call': 'to call',
  'holdem.cost.pot': 'in the pot',
  'holdem.seat.stack': 'Stack',
  'holdem.seat.bet': 'Bet',
  'holdem.prompt.yourAction': 'Your action',
  'holdem.prompt.raiseTo': 'Raise to',
  'zone.yourHand': 'Your hand',
  'zone.opponentHand': 'Their hand',
  'zone.drawPile': 'Stock',
  'zone.discardPile': 'Discard pile',
  'zone.melds': 'Melds',
  'zone.teamMelds': 'Your side\'s melds',
  'zone.redThrees': 'Red threes',
  'zone.board': 'Board',
  'verb.drawFromDeck': 'Draw',
  'verb.takeFromDiscard': 'Take from pile',


  // --- the why sheet's own furniture ---------------------------------------
  // The three layers a refusal is explained in. Labels, not sentences: what
  // goes under each is sent by the server or worded above.
  'why.reason': 'Why not',
  'why.rule': 'The rule',
  'why.rules': 'The rules',
  'why.remedy': 'What you can do',
  'why.readTheRules': 'Read the full rules →',
  'why.close': 'Close',
  'why.open': 'why',

  // --- marks on a particular card ------------------------------------------
  // A mark is a refusal that has not happened yet: the rule is enforced at the
  // discard, which is the last possible moment to hear about it, so the card
  // says so while there is still a turn left to act on it.
  'zolik.badge.owedToMeld': '{card} came off the discard pile — it has to go into the melds you go down with this turn.',
  'zolik.badge.jokerOwed': '{card} came off the table — it has to go into a meld before you can end your turn.',
};

// Czech. Present to prove the seam is real rather than theoretical: if a
// second locale can be added without touching the server or any rule logic,
// the message-key split did its job.
const cs: Record<string, string> = {
  'err.NOT_YOUR_TURN': 'Nejsi na řadě',
  'err.WRONG_PHASE': 'Teď to nejde',
  'err.MUST_DRAW_FIRST': 'Nejdřív si lízni kartu',
  'err.GAME_SUSPENDED': 'Hra je pozastavena',
  'err.GAME_NOT_ACTIVE': 'Hra neběží',
  'err.MATCH_NOT_ACTIVE': 'Stůl je pozastavený — čeká se na návrat hráče',
  'err.NOT_CONNECTED': 'Nejsi připojen ke stolu — připojuji znovu, pak to zkus',
  // "Potvrzeno" rather than "jsi připraven": a Czech participle agrees with the
  // gender of whoever it describes, and a seat carries none.
  'err.ALREADY_READY': 'Potvrzeno',
  'err.NOT_BETWEEN_ROUNDS': 'Kolo ještě běží',
  'err.NOT_AT_THIS_TABLE': 'Nejsi u tohoto stolu',
  'err.DISCARD_LOCKED': 'Odhazovací balíček je zatím zamčený',
  'err.DISCARD_PILE_EMPTY': 'Odhazovací balíček je prázdný',
  'err.NO_CARDS_LEFT': 'Už nezbývají žádné karty',
  'err.ROUND_REQ_NOT_MET': 'Nejdřív vylož vlastní první kombinaci',
  'err.NEED_CLEAN_RUN': 'Než budeš dole, musíš mít na stole čistou postupku bez žolíka',
  'err.INCOMPLETE_INITIAL_MELD': 'Dokonči výklad, nebo ho vrať zpět, než odhodíš',
  'err.DISCARD_CARD_NOT_MELDED': 'Vzatá karta musí jít do tvé kombinace',
  'err.JOKER_DISCARD_FORBIDDEN': 'Žolíka nelze odhodit',
  'err.NOTHING_TO_UNDO': 'Není co vrátit',
  'err.NO_JOKER_IN_MELD': 'V této kombinaci není žolík',
  'err.JOKER_SWAP_MISMATCH': 'Tato karta nenahradí žolíka',
  'err.RECLAIMED_JOKER_NOT_MELDED': 'Žolíka vzatého ze stolu musíš v tomto tahu zahrát do kombinace',
  'err.RUN_TOO_LONG': 'Postupka už je na plné délce',
  'err.WRONG_RUN_END': 'Tato karta patří na druhý konec postupky',
  'err.INVALID_MELD': 'Žádná karta v ruce sem nepasuje',
  'err.CARD_NOT_IN_HAND': 'Tuto kartu v ruce nemáš',
  'err.MELD_BELOW_MINIMUM': 'Tvé kombinace zatím nemají dost bodů na vyložení',
  'err.MELD_NO_CONTRIBUTION': 'Tato kombinace ti nepomůže splnit požadavek',
  'err.TOO_MANY_WILDS': 'Příliš mnoho žolíků v kombinaci',
  'err.ADJACENT_WILDS': 'Dva žolíci nemohou být vedle sebe',
  'err.ACE_BRIDGE': 'Eso nemůže spojit krále a dvojku',

  'contract.sets.1': 'Jedna skupina',
  'contract.sets.2': 'Dvě skupiny',
  'contract.sets.3': 'Tři skupiny',
  'contract.sets.n': '{n} skupin',
  'contract.runs.1': 'Jedna postupka',
  'contract.runs.2': 'Dvě postupky',
  'contract.runs.3': 'Tři postupky',
  'contract.runs.n': '{n} postupek',
  'contract.any': 'Jakákoli platná kombinace',
  'contract.cleanRunOnly': 'Libovolná směs skupin a postupek — alespoň jedna postupka bez žolíka',
  'contract.cleanRunSuffix': '{base} — jedna postupka bez žolíka',

  'zolik.rules.section.goal': 'Cíl',
  'zolik.rules.section.setup': 'Příprava',
  'zolik.rules.section.turn': 'Tvůj tah',
  'zolik.rules.section.melding': 'Vykládání',
  'zolik.rules.section.end': 'Konec zápasu',
  'zolik.rules.goal':
    'Jako první se zbav všech karet vykládáním platných skupin a postupek — a snaž se přitom mít v ruce co nejméně trestných bodů, až někdo jiný skončí kolo.',
  'zolik.rules.deal': 'Každý hráč dostane {n} karet.',
  'zolik.rules.meldShapes':
    'Skupina je {set}+ karet stejné hodnoty; postupka je {run}+ karet stejné barvy jdoucích po sobě.',
  'zolik.rules.turn.draw': 'Na svém tahu si lízni jednu kartu — z balíčku, nebo z odhazovacího balíčku.',
  'zolik.rules.pickup.topOnly': 'Z odhazovacího balíčku lze vzít jen vrchní kartu.',
  'zolik.rules.pickup.anyFromPile':
    'Z odhazovacího balíčku lze vzít libovolnou kartu i se vším, co je na ní navrch.',
  'zolik.rules.pickup.locked': 'Z odhazovacího balíčku nelze brát karty až do kola {n}.',
  'zolik.rules.pickup.open': 'Odhazovací balíček je otevřený už od prvního kola.',
  'zolik.rules.turn.discard': 'Tah ukonči odhozením jedné karty.',
  'zolik.rules.jokers.restricted':
    'Žolíka nelze odhodit, kromě situace, kdy jím právě zbavíš ruku poslední karty.',
  'zolik.rules.lead.rotate': 'Kdo je na řadě jako první, se každé kolo posouvá o jedno místo dál — bez ohledu na to, kdo vyhrál.',
  'zolik.rules.lead.winner': 'Další kolo začíná ten, kdo se právě zbavil karet.',
  'zolik.rules.meldFloor.on':
    'Tvoje první kombinace (nebo kombinace) musí dohromady mít aspoň {n} přirozených bodů, než jsi dole.',
  'zolik.rules.meldFloor.off': 'Na první kombinaci není žádná minimální bodová hranice.',
  'zolik.rules.cleanRun.on':
    'Než se počítáš jako dole, musíš mít na stole aspoň jednu postupku úplně bez žolíka.',
  'zolik.rules.cleanRun.off': 'Postupky můžou žolíky používat volně — žádná nemusí být bez žolíka.',
  'zolik.rules.contracts.rotating':
    'Zápas má {n} rozdání a každé vyžaduje jinou kombinaci skupin a postupek.',
  'zolik.rules.contracts.static': 'Každé rozdání vyžaduje stejnou kombinaci: {sets} skupin a {runs} postupek.',
  'zolik.rules.end.afterDeals': 'Zápas končí po {n} rozdáních.',
  'zolik.rules.end.atScore': 'Rozdává se dál, dokud někdo nedosáhne {n} bodů — pak zápas končí.',

  'prsi.rules.section.goal': 'Cíl',
  'prsi.rules.section.setup': 'Příprava',
  'prsi.rules.section.turn': 'Tvůj tah',
  'prsi.rules.section.special': 'Speciální karty',
  'prsi.rules.section.end': 'Konec hry',
  'prsi.rules.goal': 'Jako první se zbav všech karet z ruky.',
  'prsi.rules.deck': 'Hraje se s balíčkem {value} karet (od sedmy výš).',
  'prsi.rules.deal': 'Každý hráč začíná s {n} kartami.',
  'prsi.rules.turn.match':
    'Zahraj kartu, která odpovídá barvou nebo hodnotou vrchní kartě — pokud nemůžeš, lízni si.',
  'prsi.rules.turn.draw': 'Lízání ukončí tah bez zahrání karty.',
  'prsi.rules.sevens':
    'Zahraješ-li sedmu, další hráč líže dvě karty, pokud nemá vlastní sedmu k odpovědi.',
  'prsi.rules.aces': 'Zahraješ-li eso, další hráč je vynechán.',
  'prsi.rules.queens': 'Zahraješ-li dámu, urči barvu, kterou se hraje dál.',
  'prsi.rules.end': 'Hra končí ve chvíli, kdy má někdo prázdnou ruku.',

  'canasta.rules.section.goal': 'Cíl',
  'canasta.rules.section.setup': 'Příprava',
  'canasta.rules.section.melding': 'Vykládání',
  'canasta.rules.section.end': 'Konec zápasu',
  'canasta.rules.goal':
    'Hraje se ve dvojicích; zápas vyhrává strana, která první dosáhne {n} bodů.',
  'canasta.rules.deck': 'Hraje se s {value} kartami — dva balíčky plus žolíci.',
  'canasta.rules.deal': 'Každý hráč dostane {n} karet.',
  'canasta.rules.redThrees':
    'Červenou trojku v ruce hned ukážeš a započítá se jako bonus — pokud ale tvá strana nedokončí ani jednu kanastu, počítá se naopak proti vám.',
  'canasta.rules.canasta': 'Kanasta je kombinace {n} a více karet stejné hodnoty.',
  'canasta.rules.meldFloorBands':
    'Minimální hodnota první kombinace roste s vaším skóre: {negative} pod nulou, {low} do 1500, {mid} do 3000, {high} nad tím.',
  'canasta.rules.oneCanastaToGoOut': 'Vaší straně stačí k ukončení hry jedna dokončená kanasta.',
  'canasta.rules.twoCanastasToGoOut':
    'Vaše strana potřebuje k ukončení hry dvě dokončené kanasty.',
  'canasta.rules.end':
    'Rozdává se dál, dokud jedna strana nepřekročí {n} bodů — pak zápas končí.',

  'holdem.rules.section.goal': 'Cíl',
  'holdem.rules.section.setup': 'Příprava',
  'holdem.rules.section.betting': 'Sázení',
  'holdem.rules.section.end': 'Konec zápasu',
  'holdem.rules.goal':
    'Vyhrávej žetony nejlepší kombinací u vyhodnocení, nebo tím, že v kole zůstaneš jako jediný.',
  'holdem.rules.stack': 'Každé místo začíná s {n} žetony.',
  'holdem.rules.blinds':
    'Malý blind je {sb} a velký blind {bb}, vkládají se ještě před rozdáním karet.',
  'holdem.rules.streets':
    'Sázelo se ve čtyřech kolech — před flopem a po rozdání flopu, turnu a riveru.',
  'holdem.rules.showdown':
    'Hráči, kteří zůstali ve hře, odkryjí karty; bank bere nejlepší kombinace z pěti karet.',
  'holdem.rules.noLimit':
    'Sázení bez limitu — vsadit lze libovolnou částku až do výše celého stacku.',
  'holdem.rules.lastPlayerStanding': 'Hraje se, dokud jedno místo nezíská všechny žetony.',
  'holdem.rules.mostChipsWins': 'Zápas vyhrává ten, kdo má na konci nejvíc žetonů.',
  'holdem.rules.handLimit': 'Hraje se {n} rozdání.',

  'header.deal': 'Rozdání {n}',
  'header.gameOf': 'Hra {n} ze {total}',
  'header.gameOfWithContract': 'Hra {n} ze {total}: {contract}',

  'preview.validSet': 'Platná skupina',
  'preview.validRun': 'Platná postupka',
  'preview.validMeld': 'Platná kombinace',
  'preview.notYet': 'Zatím není kombinace',
  'preview.points': '{shape} · {n} bodů',
  'preview.pointsWithLaid': '{shape} · {n} + {laid} již vyloženo = {total} bodů',
  'preview.meetsFloor': '{line} (splňuje {n} ✓)',
  'preview.needsFloor': '{line} (potřebuje {n} ✗)',
  'preview.becauseOf': '{line} — {reason}',

  'discard.meldRejected': '{reason} — nic se neodhodilo, karty máš pořád připravené.',

  'sel.tooMany.1': 'Vyber jen jednu kartu',
  'sel.tooMany.n': 'Vyber nejvýš {n} karet',
  'sel.needMore': 'Vyber {n} kartu/karty',
  'sel.notThese': 'Tyhle karty sem nepatří',
  'sel.needsCompany': 'Tahle karta potřebuje ty vedle sebe',

  // Wording that names no verb, because a Czech verb agrees with the gender of
  // whoever did it and a player id carries no gender. "Vítěz: Anna" is right
  // where "Vyhrál Anna" is wrong.
  'status.winner': 'Vítěz: {winners}',
  'holdem.status.pot': 'Bank {amount} bere {winners} — {hand}',
  'holdem.status.potUncontested': 'Bank {amount} bere {winners} — ostatní složili',
  'holdem.status.shown': 'Karty hráče {playerId}: {value}',
  'holdem.prompt.waitingFor': 'Čeká se na hráče {playerId}',
  'zolik.standing.dealsWon': 'Vyhraných kol {n}',
  'zolik.standing.inHand': 'V ruce {n}',

  // Wording that names no verb where a player is the subject, for the same
  // reason status.winner does not: a Czech verb agrees with the gender of
  // whoever did it, and a player id carries none.
  // Imperative, and with no count: Czech inflects "hráč" by it — jeden hráč,
  // dva hráči, pět hráčů — so a number here would need three phrasings to buy
  // something the seat markers already show.
  'round.continue': 'Začít další kolo',

  'zolik.round.deal': 'Rozdání',
  'zolik.round.cleanRun': 'Jedna postupka bez žolíka',
  'canasta.round.deal': 'Rozdání',
  'canasta.round.concealed': 'Vyložení naráz',
  'canasta.round.exhausted': 'Došel balíček',
  'canasta.round.meldCards': 'Vyložené karty {n}',
  'canasta.round.canastas': 'Kanasty {n}',
  'canasta.round.redThrees': 'Červené trojky {n}',
  'canasta.round.goingOut': 'Za ukončení {n}',
  'canasta.round.inHand': 'Zbylo v ruce {n}',
  'holdem.round.hand': 'Hra',
  'holdem.round.pot': 'Bank {n}',
  'holdem.round.uncontested': 'Ostatní složili',
  'seat.ready': 'Připraven',



  // --- refusals the other three games raise ---------------------------------
  // Worded here for the same reason Žolíky's are: reasonText falls back to the
  // code, and OfferBar hands it the code as that fallback, so an unworded
  // refusal reaches a player as PILE_FROZEN in capitals. serverKeys.test.ts
  // now fails the build rather than letting the next one through.
  'err.SET_TOO_LARGE': 'Skupina už má všechny čtyři barvy',
  'err.DISCARD_TAKEN_CARD_FORBIDDEN': 'Kartu, kterou sis právě vzal, nemůžeš odhodit — zahraj ji, nebo si ji nech',
  'err.CARD_DOES_NOT_FIT': 'Tato karta nesedí barvou ani hodnotou',
  'err.SUIT_REQUIRED': 'Řekni, jaká barva se hraje dál',
  'err.MUST_ANSWER_DRAW_OR_TAKE': 'Odpověz sedmičkou, nebo si karty vezmi',
  'err.NOTHING_TO_DRAW': 'Není co líznout',
  'err.PILE_EMPTY': 'Balíček je prázdný',
  'err.PILE_BLOCKED': 'Balíček je zablokovaný — nahoře leží černá trojka',
  'err.PILE_FROZEN': 'Balíček je zmrazený — potřebuješ dvě přirozené karty hodnoty vrchní karty',
  'err.TOP_CARD_UNUSABLE': 'Vrchní kartu použít nemůžeš',
  'err.MELD_CLOSED': 'Tato kombinace je hotová a uzavřená',
  'err.MELD_TOO_SMALL': 'Kombinace potřebuje víc karet',
  'err.MELD_TOO_LARGE': 'Do této kombinace už další karty nejdou',
  'err.MELD_MIXED_RANKS': 'Všechny karty v kombinaci musí mít stejnou hodnotu',
  'err.NOT_ENOUGH_NATURALS': 'Kombinace potřebuje víc přirozených karet než žolíků',
  'err.RANK_ALREADY_MELDED': 'Tvoje strana už kombinaci této hodnoty má',
  'err.NOT_YOUR_MELD': 'Tato kombinace patří druhé straně',
  'err.NO_SUCH_MELD': 'Taková kombinace na stole není',
  'err.CANNOT_MELD_THREE': 'Trojky se nevykládají',
  'err.CANNOT_DISCARD_RED_THREE': 'Červená trojka se nedá odhodit',
  'err.MUST_KEEP_A_CARD': 'Nech si aspoň jednu kartu — takhle ruku vyprázdnit nemůžeš',
  'err.MUST_MELD_FIRST': 'Než to uděláš, vylož první kombinaci své strany',
  'err.INITIAL_MELD_NOT_MET': 'První kombinaci ještě chybí body',
  'err.CANNOT_GO_OUT_YET': 'Tvoje strana potřebuje hotovou canastu, než může vyjít',
  'err.NOTHING_TO_CALL': 'Není co dorovnat',
  'err.CANNOT_CHECK': 'Nemůžeš čekat — je tu sázka k dorovnání',
  'err.CANNOT_RAISE': 'Tady zvýšit nemůžeš',
  'err.RAISE_TOO_SMALL': 'Zvýšení musí být aspoň o poslední sázku',
  'err.NOT_ENOUGH_CHIPS': 'Tolik žetonů nemáš',
  'err.AMOUNT_REQUIRED': 'Zadej kolik',
  'err.AMOUNT_NOT_A_NUMBER': 'Tato částka není číslo',
  'err.SEAT_NOT_IN_HAND': 'V tomto rozdání nehraješ',
  'err.WRONG_RANK': 'Tato karta má pro tohle špatnou hodnotu',
  'err.MATCH_FULL': 'Stůl je plný',
  'err.MATCH_ALREADY_STARTED': 'Zápas už začal',
  'err.TOO_FEW_PLAYERS': 'Zatím je málo hráčů',
  'err.WRONG_PLAYER_COUNT': 'Tuto hru nelze hrát s tímto počtem hráčů',
  'err.NOT_THE_HOST': 'To může udělat jen zakladatel stolu',
  'err.NO_LONGER_WAITING': 'Stůl už nečeká',
  'err.WAITING_ROOM_UNAVAILABLE': 'Čekárna není dostupná',


  // --- rules the engine has always enforced and never stated ----------------
  // Every one of these was found by the guardrail rather than by review: the
  // validator could refuse a player for it, and no sentence anywhere said so.
  // See server/internal/zolikmod/ruleindex_test.go.
  'zolik.rules.section.layoff': 'Přikládání ke kombinacím',
  'zolik.rules.pickup.obligation': 'Dokud nejsi dole, musí karta vzatá z odhazovacího balíčku být použita v kombinaci, se kterou v tomto tahu jdeš dolů.',
  'zolik.rules.pickup.noReturn': 'Kartu vzatou z odhazovacího balíčku nemůžeš ve stejném tahu odhodit — zahraj ji, nebo si ji nech.',
  'zolik.rules.wilds.setLimit': 'Skupina nesmí mít víc žolíků než přirozených karet.',
  'zolik.rules.set.maxSize': 'Skupina nesmí mít víc než {n} karty — žolík doplní chybějící barvu, nenafukuje plnou.',
  'zolik.rules.run.maxLength': 'Postupka je nejvýš {n} karet dlouhá — eso dole, dvanáct hodnot nad ním a eso nahoře.',
  'zolik.rules.run.aceBridge': 'Eso leží nad králem nebo pod dvojkou, nikdy nespojuje oba konce postupky.',
  'zolik.rules.contracts.contribution': 'Dokud nejsi dole, musí každá vyložená kombinace být taková, jakou zadání rozdání ještě potřebuje.',
  'zolik.rules.layoff.afterDown': 'Ke kombinacím nemůžeš přikládat, dokud nevyložíš vlastní zadání.',
  'zolik.rules.layoff.runEnds': 'Karta přiložená k postupce ji musí prodloužit na jednom nebo druhém konci.',
  'zolik.rules.jokers.swap': 'Žolíka v kombinaci na stole můžeš vyměnit za přesně tu kartu, kterou zastupuje.',
  'zolik.rules.jokers.reclaim.on':
    'Žolíka vzatého ze stolu musíš ve stejném tahu zahrát do kombinace — nesmí ti zůstat v ruce.',
  'zolik.rules.jokers.reclaim.off': 'Žolíka vzatého ze stolu si můžeš nechat v ruce.',
  'zolik.rules.deck.reshuffle': 'Když dojde lízací balíček, odhazovací balíček se zamíchá a stane se novým lízacím; pokud jsou prázdné oba, rozdání končí.',


  // --- what to do instead ---------------------------------------------------
  // The third layer of a refusal, after the reason and the rule. Sent by the
  // module because it is the only side that knows which card is owed and
  // which way out is on offer; see server/internal/zolikmod/remedy.go.
  'zolik.remedy.meldThePickup': 'Přidej {card} do výkladu, nebo vrať vzetí zpět.',
  'zolik.remedy.discardSomethingElse': 'Odhoď jinou kartu, nebo {card} v tomto tahu zahraj.',
  'zolik.remedy.discardNotAJoker': 'Odhoď něco jiného než žolíka.',
  'zolik.remedy.finishOrUndoLayDown': 'Dokonči výklad, nebo ho vrať zpět.',
  'zolik.remedy.needMorePoints': 'Než půjdeš dolů, chybí ti ještě {n} bodů.',
  'zolik.remedy.layACleanRun': 'Vylož postupku bez žolíka.',
  'zolik.remedy.playReclaimedJoker': 'Zahraj {card} do kombinace, nebo vrať vzetí zpět.',
  'zolik.remedy.goDownFirst': 'Nejdřív vylož vlastní kombinace.',
  'zolik.remedy.drawFirst': 'Nejdřív si lízni kartu.',
  'zolik.remedy.drawFromStock': 'Lízni si z balíčku — odhazovací balíček se otevře v kole {n}.',
  'zolik.remedy.drawFromStockEmpty': 'Lízni si místo toho z balíčku.',


  // --- keys whose meaning lives in their params -----------------------------
  // The shape fallback renders these with the number missing ("Stack", "Pot"),
  // which is why they earn a line where most keys do not.
  'header.contract': 'Potřebuje {sets} skupiny a {runs} postupky',
  'header.contract.cleanRunOnly': 'Potřebuje postupku bez žolíka',
  'header.round': 'Kolo {n}',
  'header.deck': 'Balíček',
  'header.target': 'Cíl',
  'header.suitInPlay': 'Hraje se',
  'seat.cards': 'Karet',
  'zolik.offer.meld': 'Kombinace',
  'prompt.pickupMustBeMelded': '{value} je z odhazovacího balíčku — musí jít do kombinací, se kterými v tomto tahu jdeš dolů.',
  'prompt.jokerMustBePlayed': '{value} je ze stolu — než ukončíš tah, musí jít do kombinace.',
  'prompt.initialMeld': 'První kombinace tvé strany musí mít {n} bodů.',
  'prompt.canastasNeeded': 'Tvé straně chybí ještě {n} canasty, než může vyjít.',
  'prompt.mustDrawOrAnswerSeven': 'Odpověz sedmičkou, nebo si lízni {n} karty.',
  'prompt.chooseSuit': 'Vyber barvu, která se hraje dál',
  'prompt.skipPending': 'Tvůj tah se přeskakuje',
  'status.lastDeal': 'Tým {team} získal {value}',
  'status.teamScore': 'Tým {team}: {value}',
  'canasta.offer.rank': 'Hodnota',
  'canasta.seat.teamScore': 'Skóre týmu',
  'canasta.seat.canastas': 'Canasty',
  'holdem.header.pot': 'Bank',
  'holdem.header.street': 'Fáze',
  'holdem.header.hand': 'Rozdání',
  'holdem.header.handLimit': 'Rozdání celkem',
  'holdem.header.blinds': 'Blindy',
  'holdem.cost.call': 'k dorovnání',
  'holdem.cost.pot': 'v banku',
  'holdem.seat.stack': 'Žetony',
  'holdem.seat.bet': 'Sázka',
  'holdem.prompt.yourAction': 'Jsi na tahu',
  'holdem.prompt.raiseTo': 'Zvýšit na',
  'zone.yourHand': 'Tvoje karty',
  'zone.opponentHand': 'Karty soupeře',
  'zone.drawPile': 'Lízací balíček',
  'zone.discardPile': 'Odhazovací balíček',
  'zone.melds': 'Kombinace',
  'zone.teamMelds': 'Kombinace tvé strany',
  'zone.redThrees': 'Červené trojky',
  'zone.board': 'Stůl',
  'verb.drawFromDeck': 'Líznout',
  'verb.takeFromDiscard': 'Vzít z balíčku',


  // --- the why sheet's own furniture ---------------------------------------
  'why.reason': 'Proč ne',
  'why.rule': 'Pravidlo',
  'why.rules': 'Pravidla',
  'why.remedy': 'Co s tím můžeš udělat',
  'why.readTheRules': 'Zobrazit celá pravidla →',
  'why.close': 'Zavřít',
  'why.open': 'proč',

  // --- marks on a particular card ------------------------------------------
  'zolik.badge.owedToMeld': '{card} je z odhazovacího balíčku — musí jít do kombinací, se kterými v tomto tahu jdeš dolů.',
  'zolik.badge.jokerOwed': '{card} je ze stolu — než ukončíš tah, musí jít do kombinace.',
};

export const BUNDLES: Record<Locale, Record<string, string>> = { en, cs };

let currentLocale: Locale = 'en';

export function setLocale(locale: Locale) {
  currentLocale = BUNDLES[locale] ? locale : 'en';
}

export function getLocale(): Locale {
  return currentLocale;
}

/**
 * Looks up a key and substitutes `{name}` placeholders.
 *
 * Falls back locale → English → `fallback` → the key itself, so an
 * untranslated string degrades to a readable one rather than to nothing.
 */
export function t(key: string, params?: Params, fallback?: string): string {
  return interpolate(messageTemplate(key) ?? fallback ?? key, params);
}

/**
 * The wording a key would use, or undefined for a key no bundle knows.
 *
 * Exported so a caller can tell "this key has words of its own" from "this key
 * is about to be rendered by its own shape". The difference matters to
 * whoever is composing a line out of more than the key alone — see `factText`,
 * where a phrase that already places its own values must not have another one
 * appended after it.
 */
export function messageTemplate(key: string): string | undefined {
  return BUNDLES[currentLocale]?.[key] ?? BUNDLES.en[key];
}

function interpolate(template: string, params?: Params): string {
  if (!params) return template;
  return template.replace(/\{(\w+)\}/g, (whole, name: string) =>
    name in params ? String(params[name]) : whole,
  );
}

/**
 * Wording for an engine error code. The codes are the server's stable
 * vocabulary; only the phrasing is ours. An unrecognised code — a newer
 * server than this build — falls back rather than printing SCREAMING_SNAKE at
 * the player.
 */
export function reasonText(code: string | undefined, fallback = ''): string {
  if (!code) return fallback;
  return t(`err.${code}`, undefined, fallback);
}

/**
 * "One set", "Two runs" — the whole phrase, looked up by count.
 *
 * Not a number glued to a pluralised noun: Czech inflects the noun by count
 * in a way no "add an s" helper survives, so the phrase is what the bundle
 * owns. Counts beyond the enumerated ones fall back to a generic form.
 */
export function countLabel(n: number, noun: 'sets' | 'runs'): string {
  const exact = `contract.${noun}.${n}`;
  if (BUNDLES.en[exact]) return t(exact);
  return t(`contract.${noun}.n`, { n });
}
