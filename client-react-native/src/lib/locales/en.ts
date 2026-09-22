/**
 * The English bundle — the reference every other locale is checked against.
 *
 * Keys are namespaced by what they describe, so an unfamiliar one is at least
 * placeable. Adding a key here without adding it everywhere else fails
 * `i18n.test.ts`, which is the point: a half-translated locale is how
 * "mostly Czech with random English" ships.
 */

export const en: Record<string, string> = {
  // --- engine error codes (rules.RulesErrorCode) ---------------------------
  'err.NOT_YOUR_TURN': "It's not your turn",
  'err.WRONG_PHASE': 'Not at this point in the turn',
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
  // Bringing a table back that the sweeper set aside. Refusals a player
  // can actually provoke: a link to a table that is gone, one that was
  // already picked up in another tab, and a table with other people at it.
  'err.MATCH_MOVED_ON': 'The table moved on — reload to see where it is',
  'err.MATCH_NOT_ABANDONED': 'That table is not waiting to be brought back',
  'err.MATCH_NOT_FOUND': 'That table no longer exists',
  'err.MATCH_DELETED': 'The host deleted this table',
  'err.TABLE_HAS_PLAYERS_AWAY': 'Everyone has to be back at the table before this game can be picked up',
  'err.NOTHING_TO_REPLAY': 'There is no game here to replay yet',
  'err.REPLAY_UNAVAILABLE': 'Replays are not available on this server',
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
  'prsi.remedy.matchOrDraw': 'Play a {suit} card, or one that matches {card} — otherwise draw.',
  'prsi.remedy.answerSevenOrTake': 'Answer with a seven of your own, or take the {n} cards.',
  'prsi.remedy.playOrDraw': 'There is no skip waiting for you — play a {suit} card, or draw.',
  'prsi.remedy.nameASuit': 'Name the suit that follows your queen.',
  'prsi.remedy.nothingLeftToDraw': 'Nothing is left to draw — play a card if you can.',

  'canasta.rules.section.goal': 'Goal',
  'canasta.rules.section.setup': 'Setup',
  'canasta.rules.section.melding': 'Melding',
  'canasta.rules.section.end': 'How the match ends',
  'canasta.rules.section.turn': 'Your turn',
  'canasta.rules.goal':
    'Play in partnerships; the first side to reach {n} points wins the match.',
  'canasta.rules.deck': 'Played with {value} cards — {decks} decks plus jokers.',
  'canasta.rules.deal': 'Each player is dealt {n} cards.',
  'canasta.rules.drawCount': 'You draw {n} cards at the start of your turn.',
  'canasta.rules.redThrees':
    "A red three in your hand is shown immediately and scores as a bonus — unless your side never completes a canasta, when it counts against you instead.",
  'canasta.rules.canasta': 'A canasta is a meld of {n} or more cards of the same rank.',
  'canasta.rules.sequences': 'A meld can also be a sequence: three or more cards of the same suit in a row, never with a wild card among them.',
  'canasta.rules.samba': 'A sequence of seven cards is a samba, worth {n} points.',
  'canasta.rules.blackThreesGoOut':
    'A black three blocks the discard pile and is worth {n} points. Three or four of them may be melded straight from your hand, never with a wild card among them, and only as the move that takes your side out.',
  'canasta.rules.blackThreesNeverMeld':
    'A black three is never melded. It blocks the discard pile when discarded, and costs {n} points if it is still in your hand when the deal ends.',
  'canasta.rules.pileAlwaysFrozen': 'The discard pile is frozen all deal: to take it you must match its top card with two natural cards from your hand.',
  'canasta.rules.pileOntoMeld': "If your side already has an unfinished meld of the top card's rank, you can take the whole discard pile to add that card to it — no matching pair in your hand is needed.",
  'canasta.rules.pileNoMeldCapture': 'A meld already on the table cannot take the discard pile: to take it you must match its top card with two cards from your own hand.',
  'canasta.rules.meldFloorBands':
    'Your first meld must reach a point minimum that rises with your score: {negative} below zero, {low} up to {lowUpTo}, {mid} up to {midUpTo}, {high} beyond that.',
  'canasta.rules.meldFloorBandsFive': 'Your first meld must reach a point minimum that rises with your score: {negative} below zero, {low} up to {lowUpTo}, {mid} up to {midUpTo}, {high} up to {highUpTo}, {top} beyond that.',
  'canasta.rules.goingOutBonus':
    'Going out is worth {n} points on its own — {concealed} if your side does it in a single turn, with nothing melded before that.',
  'canasta.rules.goingOutBonusFlat': 'Going out is worth {n} points on its own.',
  'canasta.rules.turn':
    'A turn is one move into your hand — draw from the stock, or take the whole discard pile — then any melds you want to lay, then one card discarded.',
  'canasta.rules.turnDiscard': 'The discard is what ends a turn, so you always need a card to spare for it.',
  'canasta.rules.pileTopCard':
    'The discard pile can only be taken by a move that uses its top card straight away.',
  'canasta.rules.pileBlocked':
    'A black three on top blocks the pile — nobody may take it until the three is buried — and one left in your hand costs {n}.',
  'canasta.rules.pileFrozenByWild':
    "A wild card buried in the pile freezes it against everyone: taking it then costs two natural cards of the top card's rank, out of your own hand.",
  'canasta.rules.meldShape': 'A meld is {n} or more cards of the same rank.',
  'canasta.rules.wildLimit':
    'A meld may hold at most {wilds} wild cards, and never fewer than {naturals} natural ones.',
  'canasta.rules.wildRatio':
    'A meld needs {n} natural cards for every wild one, and never more than {wilds} wilds in all.',
  'canasta.rules.oneMeldPerRank':
    'Your side keeps one meld of each rank — more cards of that rank are laid off onto it.',
  'canasta.rules.meldsPerRankUnlimited': 'Your side may have several melds of the same rank.',
  'canasta.rules.canastaCloses': 'A canasta of {n} is complete and takes no more cards.',
  'canasta.rules.meldsAreShared':
    "Melds belong to the partnership: either partner may extend them, and neither side may touch the other's.",
  'canasta.rules.layOffAfterOpening':
    'Until your side has made its initial meld it may not lay off onto anything.',
  'canasta.rules.goOutKeepsACard':
    'You must always be able to finish your turn, so never meld away your whole hand unless it is the move that goes out.',
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
  'holdem.rules.checkOrCall': 'You may check only when nothing is owed; otherwise call, raise or fold.',
  'holdem.rules.minRaise': 'A raise has to be at least as big as the last one.',
  'holdem.rules.allIn':
    'You can never put in more than your stack, and going all in is always allowed — even for less than a full raise.',
  'holdem.rules.foldedOut': 'Once you fold you are out until the next hand is dealt.',
  'holdem.remedy.callOrFold': 'There is {n} to call — call, raise, or fold.',
  'holdem.remedy.checkOrRaise': 'Nothing is owed — check, or raise.',
  'holdem.remedy.callAllInOrFold': 'Your stack will not get above the bet — call {n} all in, or fold.',
  'holdem.remedy.raiseAtLeast': 'Raise to at least {n}.',
  'holdem.remedy.raiseAtMost': 'Raise to at most {n} — that is your whole stack.',
  'holdem.remedy.nameAnAmount': 'Say how much to raise to, between {min} and {max}.',
  'holdem.remedy.waitForNextHand': 'You are out of this hand — wait for the next deal.',

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
  'holdem.status.pot': '{winners} won {amount} with {hand} — {cards}',
  'holdem.status.potSplit': '{winners} won {amount} with {hand}',
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

  // --- the flash that announces a round or a match ending ------------------
  //
  // One line and one number, held for about two seconds. Worded generically on
  // purpose: which *kind* of ending it was is the module's own fact, printed
  // underneath, and a client that phrased "went gin" itself would be a client
  // that knows a game.
  'flash.roundWon': '{winners} took it',
  'flash.roundWonYou': 'You took it',
  'flash.roundDrawn': 'Nobody took it',
  'flash.matchOver': 'Match over',
  'flash.matchWon': '{winners} won',
  'flash.matchWonYou': 'You won',
  'flash.matchDrawn': 'Nobody won',
  'flash.nowOn': 'now {total}',

  'zolik.round.deal': 'Deal',
  'zolik.round.cleanRun': 'One run must be joker-free',
  'canasta.round.deal': 'Deal',
  'canasta.round.concealed': 'Went out concealed',
  'canasta.round.exhausted': 'The deck ran out',
  'canasta.round.meldCards': 'Cards laid {n}',
  'canasta.round.canastas': 'Canastas {n}',
  'canasta.round.redThrees': 'Red threes {n}',
  'canasta.round.goingOut': 'Going out {n}',
  'canasta.round.closed': '{player} closed with a {diff} point differential',
  'canasta.round.inHand': 'Caught in hand {n}',
  'holdem.round.hand': 'Hand',
  'holdem.round.pot': 'Pot {n}',
  'holdem.round.uncontested': 'Everyone else folded',
  'seat.ready': 'Ready',
  'zolik.seat.contractMet': 'Contract met',
  'results.you': '(you)',



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
  'err.NOTHING_TO_SKIP': 'There is no skip to take',
  'err.NOTHING_TO_DRAW': 'There is nothing left to draw',
  'err.PILE_EMPTY': 'The pile is empty',
  'err.PILE_BLOCKED': 'The pile is blocked — a black three is on top',
  'err.PILE_FROZEN': 'The pile is frozen — you need two natural cards of the top card\'s rank',
  'err.MELD_CAPTURE_NOT_ALLOWED': "A meld on the table can't take the pile in this game — you need two cards from your hand",
  'err.CAPTURE_NEEDS_TWO_CARDS': 'Taking the pile costs two cards from your hand',
  'err.TOP_CARD_UNUSABLE': "Your side can't use the top card",
  'err.MELD_CLOSED': 'That meld is complete and closed',
  'err.MELD_TOO_SMALL': 'A meld needs more cards than that',
  'err.MELD_TOO_LARGE': 'That meld can\'t take any more cards',
  'err.MELD_MIXED_RANKS': 'Every card in a meld must be the same rank',
  'err.SEQUENCE_NO_WILDS': "A sequence can't contain wild cards",
  'err.SEQUENCE_NEEDS_ONE_SUIT': 'Every card in a sequence must be the same suit',
  'err.RUN_NOT_CONSECUTIVE': 'A sequence must run in order, with no gaps',
  'err.NOT_ENOUGH_NATURALS': 'A meld needs more natural cards than wild ones',
  'err.RANK_ALREADY_MELDED': 'Your side already has a meld of that rank',
  'err.NOT_YOUR_MELD': 'That meld belongs to the other side',
  'err.NO_SUCH_MELD': 'That meld is not on the table',
  'err.CANNOT_MELD_THREE': 'Threes are never melded',
  'err.BLACK_THREE_GO_OUT_ONLY': 'Black threes go down only as the move that empties your hand',
  'err.CANNOT_DISCARD_RED_THREE': 'A red three can\'t be discarded',
  'err.MUST_KEEP_A_CARD': 'Keep at least one card — you can\'t empty your hand this way',
  'err.MUST_MELD_FIRST': 'Lay your side\'s first meld before doing that',
  'err.UNDO_MELDS_FIRST': 'Take back what you laid after the pile before taking the pile back',
  'err.INITIAL_MELD_NOT_MET': 'Your first meld is still short of the points needed',
  'err.CANNOT_GO_OUT_YET': 'Your side needs a completed canasta before it can go out',
  'err.NOTHING_TO_CALL': 'There is no bet to call',
  'err.CANNOT_CHECK': 'You can\'t check — there is a bet to answer',
  'err.CANNOT_RAISE': "You can't raise — your stack won't get above the bet",
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
  'err.BAD_SEATING': 'That seating does not match who is at the table',
  'err.NO_LONGER_WAITING': 'The table is no longer waiting',
  'err.WAITING_ROOM_UNAVAILABLE': 'The waiting room isn\'t available',
  'err.SERVER_BUSY': 'The server is full right now — try again in a moment',


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
  'canasta.offer.sequence': 'Sequence',
  'canasta.remedy.drawOrTakePile': 'Draw from the stock, or take the discard pile, before you meld.',
  'canasta.remedy.meldOrDiscard': 'You have already drawn — lay a meld, or discard to end your turn.',
  'canasta.remedy.drawFromStock': 'Draw from the stock instead.',
  'canasta.remedy.takePileInstead': 'The stock is empty — take the discard pile instead.',
  'canasta.remedy.pileBlocked': 'Draw from the stock — the black three on top keeps the pile shut.',
  'canasta.remedy.pileFrozen':
    'Draw from the stock, or take the pile with two natural cards from your hand that match {card}.',
  'canasta.remedy.topCardUnusable': 'Draw from the stock — your side has no use for the {card} on top.',
  'canasta.remedy.captureFromHand': 'Take the pile with two cards from your own hand that match {card}.',
  'canasta.remedy.needTwoMatching':
    'You need two cards from your hand that match {card} — otherwise draw from the stock.',
  'canasta.remedy.needMorePoints': "Your side's first meld is {n} points short of {floor}.",
  'canasta.remedy.openFirst': "Lay your side's first meld before you lay off.",
  'canasta.remedy.needCanastas': 'Your side needs {n} more canasta of {size} before it can go out.',
  'canasta.remedy.keepACard': 'Keep a card back to discard with.',
  'canasta.remedy.undoMeldsFirst':
    'Take back the melds you laid after the pile first — then the pile itself can go back.',
  'canasta.remedy.layOffInstead': 'Lay them off onto the meld your side already has.',
  'canasta.remedy.meldClosed': 'That meld is complete at {n} — start another, or lay off elsewhere.',
  'canasta.remedy.discardNotARedThree': 'Discard something other than a red three.',
  'canasta.remedy.blackThreesOnTheWayOut': 'Black threes go down only as the move that empties your hand.',
  'canasta.remedy.ownMeldsOnly': "Lay off onto one of your own side's melds.",
  'badge.naturalCanasta': 'Natural canasta',
  'badge.mixedCanasta': 'Mixed canasta',
  'badge.samba': 'Samba',
  'badge.cleanRun': 'Clean run',
  'canasta.seat.teamScore': 'Team score',
  'canasta.seat.canastas': 'Canastas',
  'holdem.header.pot': 'Pot',
  'holdem.header.street': 'Street',
  'holdem.header.hand': 'Hand',
  'holdem.header.handLimit': 'Hands in all',
  'holdem.header.blinds': 'Blinds',
  'holdem.cost.call': 'to call',
  'holdem.cost.pot': 'pot after call',
  'holdem.seat.stack': 'Stack',
  'holdem.seat.bet': 'Bet',
  'holdem.prompt.yourAction': 'Your action',
  'holdem.prompt.raiseTo': 'Raise to',
  'holdem.quick.halfPot': '½ Pot',
  'holdem.quick.pot': 'Pot',
  'holdem.quick.allIn': 'All-in',
  'zone.yourHand': 'Your hand',
  'zone.opponentHand': 'Their hand',
  'zone.drawPile': 'Stock',
  'zone.discardPile': 'Discard pile',
  'zone.melds': 'Melds',
  'zone.teamMelds': 'Your side\'s melds',
  'zone.opponentMelds': 'Their side\'s melds',
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

  // --- the legal notices ----------------------------------------------------
  // Only the furniture. The documents themselves are in `src/legal`, which is
  // a bundle of the same kind with a parity test of its own — prose that long
  // in a flat key map buries the keys this one exists for.
  //
  // The notice is five fragments rather than one sentence with two links glued
  // in, because Czech does not put the link where English does: "souhlasíš s
  // Podmínkami" inflects the noun the link is made of. Fragments let each
  // locale place and decline its own.
  // The short word for a link or a tab.
  'legal.terms': 'Terms',
  // The document's own name, used as the heading above it. Kept apart from
  // `legal.notice.terms`, which is the same words inflected to sit inside a
  // sentence — a heading in the instrumental case reads as a mistake in every
  // Slavic language.
  'legal.terms.title': 'Terms of Use',
  'legal.privacy.title': 'Privacy Notice',
  'legal.privacy': 'Privacy',
  'legal.source': 'Source',
  'legal.updated': 'Version {version}',
  'legal.draft': 'Draft — not yet in force. The operator’s name, country, and contact address are still to be filled in.',
  'legal.notice.before': 'By playing you agree to the ',
  'legal.notice.terms': 'Terms of Use',
  'legal.notice.between': '. What is stored about you is in the ',
  'legal.notice.privacy': 'Privacy Notice',
  'legal.notice.after': '.',
  // --- gin rummy -------------------------------------------------------------
  'err.UPCARD_DECLINED': 'You already passed on that card',
  'err.DEADWOOD_TOO_HIGH': 'Your deadwood is too high to knock',
  'err.CARD_DOES_NOT_EXTEND_MELD': "That card doesn't extend this meld",
  'ginrummy.rules.setup': 'Setup',
  'ginrummy.rules.turn': 'Your turn',
  'ginrummy.rules.melds': 'Melds',
  'ginrummy.rules.knocking': 'Knocking',
  'ginrummy.rules.bigGin': 'Big gin',
  'ginrummy.rules.layoff': 'The lay-off',
  'ginrummy.rules.deadHand': 'The dead hand',
  'ginrummy.rules.scoring': 'Scoring a hand',
  'ginrummy.rules.match': 'Winning the match',
  'ginrummy.rules.lineBonuses': 'Line bonuses',
  'ginrummy.rules.deck': 'Played with a {value}-card deck.',
  'ginrummy.rules.deal': 'Each player is dealt {value} cards.',
  'ginrummy.rules.upcard': 'One more card is turned face up to start the discard pile.',
  'ginrummy.rules.drawDiscard': 'On your turn, draw one card — from the stock or the discard pile — then discard one.',
  'ginrummy.rules.setsAndRuns': 'A meld is a set of three or four cards of one rank, or a run of three or more in one suit.',
  'ginrummy.rules.aceLow': 'The ace is always low — there is no run from Queen through Ace.',
  'ginrummy.rules.knockLimit': 'You may knock once your deadwood is {n} or less.',
  'ginrummy.rules.oklahoma': "This hand's knock limit is set by the value of the upcard.",
  'ginrummy.rules.gin': 'Zero deadwood is gin — the best possible knock.',
  'ginrummy.rules.bigGinBonus': 'Eleven cards melding with no discard at all is big gin, worth a further {n} points.',
  'ginrummy.rules.layoffDescription':
    'After a knock that is not gin, your opponent may lay their own deadwood onto your melds before the hands are compared.',
  'ginrummy.rules.deadHandDescription':
    'If the stock runs down to its last two cards and nobody has knocked, the hand is dead — nobody scores, and the same dealer deals again.',
  'ginrummy.rules.undercut': "If your opponent's deadwood is no higher than yours, they undercut you: they score the difference, plus {n}.",
  'ginrummy.rules.ginBonus': "Gin scores your opponent's whole hand, plus {n}.",
  'ginrummy.rules.target': 'First to pass {n} points after a hand ends wins the match.',
  'ginrummy.rules.shutout': "The game bonus doubles to {n} if the loser never scored a single point.",
  'ginrummy.rules.box': 'Each hand you won is worth {n} points at the end of the match.',
  'ginrummy.rules.gameBonus': 'Winning the match is worth a further {n} points.',
  'ginrummy.rules.upcardDance':
    'Before the first draw the non-dealer may take the upcard, then the dealer may; if both pass, the non-dealer must draw from the stock.',
  'ginrummy.rules.knockOnDiscard':
    'A knock replaces your discard, so it can only happen at the end of your turn.',
  'ginrummy.fact.deadwood': '{value} deadwood',
  'ginrummy.fact.discardCard': 'Discard {value}',
  'ginrummy.fact.meldCards': 'Onto {value}',
  'ginrummy.header.hand': 'Hand {n}',
  'ginrummy.round.kind': '{value}',
  'ginrummy.round.hand': 'Hand',
  'ginrummy.seat.score': '{value}',
  'ginrummy.seat.dealer': 'Dealer',
  'ginrummy.status.knocked': '{playerId} knocked with {deadwood} deadwood',
  'ginrummy.status.gin': '{playerId} went gin',
  'ginrummy.status.lastHand': 'Last hand: {winner} ({kind}, {delta} points)',
  'ginrummy.offer.drawStock': 'Draw from the stock',
  'ginrummy.offer.drawDiscard': 'Draw from the discard pile',
  'ginrummy.offer.takeUpcard': 'Take the upcard',
  'ginrummy.offer.passUpcard': 'Pass',
  'ginrummy.offer.discard': 'Discard',
  'ginrummy.offer.knock': 'Knock',
  'ginrummy.offer.gin': 'Gin!',
  'ginrummy.offer.bigGin': 'Big gin!',
  'ginrummy.offer.layOff': 'Lay off',
  'ginrummy.offer.finishLayoff': 'Done laying off',
  'ginrummy.remedy.takeOrPassUpcard': 'Take the upcard, or pass it.',
  'ginrummy.remedy.drawFirst': 'Draw a card first — from the stock or the discard pile.',
  'ginrummy.remedy.discardToEndTurn': 'Discard one card to end your turn.',
  'ginrummy.remedy.finishLayoff': 'Nothing more of yours fits — finish laying off.',
  'ginrummy.remedy.stockDrawForced': 'You both passed that card — draw from the stock.',
  'ginrummy.remedy.drawElsewhere': 'That pile is empty — draw from the other one.',
  'ginrummy.remedy.getDeadwoodDown': 'You can knock once your deadwood is {n} or less.',
  'ginrummy.zone.knockerHand': 'Knocked hand',
  'ginrummy.zone.melds': 'Melds',
  'ginrummy.prompt.upcardDecision': 'Take the upcard, or pass',
  'ginrummy.prompt.yourTurnDraw': 'Draw a card',
  'ginrummy.prompt.yourTurnDiscard': 'Discard — or knock, if you can',
  'ginrummy.prompt.layoff': 'Lay off deadwood, or finish',

  // --- rummy tiles -------------------------------------------------------------
  'err.TILE_NOT_IN_HAND': 'That tile is not in your hand',
  'err.TILE_DOES_NOT_FIT': "That tile doesn't fit in that set",
  'err.NO_SUCH_SET': 'That set is not on the table',
  'err.INITIAL_MELD_ONLY': 'You may only rearrange your own new sets before your first lay',
  'err.TABLE_NOT_VALID': "A set on the table isn't a valid group or run",
  'err.TRAY_NOT_EMPTY': 'You still have loose tiles to place',
  'err.NOTHING_PLAYED': 'Play at least one tile before finishing your turn',
  'err.INITIAL_MELD_TOO_LOW': 'Your first lay needs to be worth 30 points or more',
  'err.NOT_A_RUN': 'Only a run can be split',
  'err.BAD_SPLIT_POSITION': "That isn't a place this run can split",
  'err.NO_JOKER_IN_SET': 'There is no joker in that set',
  'err.TILE_JOKER_SWAP_MISMATCH': 'That tile is not what the joker stands for',
  'rummytiles.rules.setup': 'Setup',
  'rummytiles.rules.sets': 'Sets',
  'rummytiles.rules.initialMeld': 'The initial meld',
  'rummytiles.rules.turn': 'Your turn',
  'rummytiles.rules.jokerTaking': 'Taking a joker',
  'rummytiles.rules.ending': 'Ending a round',
  'rummytiles.rules.poolExhaustion': 'If the pool runs dry',
  'rummytiles.rules.match': 'Winning the match',
  'rummytiles.rules.tiles': 'Played with {value} tiles.',
  'rummytiles.rules.dealCount': 'Each player is dealt {value} tiles.',
  'rummytiles.rules.group': 'A group is three or four tiles of one number, each a different colour.',
  'rummytiles.rules.run': 'A run is three or more consecutive numbers in one colour.',
  'rummytiles.rules.noWrap': '13 does not run back around to 1.',
  'rummytiles.rules.joker': 'A joker stands for any tile.',
  'rummytiles.rules.initialMeldDescription':
    'Until you have laid {n} or more points in a single turn, from your own hand alone, you may not touch anything already on the table.',
  'rummytiles.rules.turnDescription':
    'Play at least one tile from your hand, rearranging the table freely, and end with every set on the table valid.',
  'rummytiles.rules.noDiscard': 'There is no discard — if you cannot complete a valid turn, you draw one tile instead.',
  'rummytiles.rules.jokerTakingDescription':
    'A joker on the table may be taken by replacing it with the tile it stands for, from your hand — and it must be used in a set before your turn ends.',
  'rummytiles.rules.goingOut':
    "The first player out of tiles wins the round. Everyone else scores the negated value of what is left in their hand; the winner scores the sum of what everyone else lost.",
  'rummytiles.rules.poolExhaustionLowestWins':
    'If the pool runs dry and nobody can play, the round ends and the lowest hand value wins it.',
  'rummytiles.rules.poolExhaustionNoWinner':
    'If the pool runs dry and nobody can play, the round ends with no winner — every hand is simply scored.',
  'rummytiles.rules.target': 'First to pass {n} points after a round ends wins the match.',
  'rummytiles.rules.roundLimit': 'The match ends after {n} rounds — highest score wins.',
  'rummytiles.remedy.emptyTheTray': 'Place the {n} tiles still in your tray, or reset the turn.',
  'rummytiles.remedy.playFromHandOrDraw':
    'Rearranging is not a turn — play at least one tile from your hand, or draw.',
  'rummytiles.remedy.fixOrReset':
    'Every set on the table has to be a valid group or run — fix them, or reset the turn.',
  'rummytiles.remedy.needMorePoints': 'Your first lay is {n} points short of {floor}.',
  'rummytiles.remedy.ownNewSetsOnly':
    'Until your first lay of {n} points you may only rearrange the sets you made this turn.',
  'rummytiles.remedy.startANewSet': 'Put it in a new set instead.',
  'rummytiles.remedy.splitLeavesThree': 'Split a run so that both halves keep at least {n} tiles.',
  'rummytiles.remedy.matchTheJoker': 'Swap the joker for the exact tile it stands for.',
  'rummytiles.fact.setCards': '{value}',
  'rummytiles.header.pool': 'Pool {n}',
  'rummytiles.header.round': 'Round {n}',
  'rummytiles.round.kind': '{value}',
  'rummytiles.round.round': 'Round',
  'rummytiles.seat.score': '{value}',
  'rummytiles.seat.notOpened': 'Not opened',
  'rummytiles.status.lastRound': 'Last round: {winner} ({kind})',
  'rummytiles.badge.invalid': 'Not yet valid',
  'rummytiles.zone.pool': 'Pool',
  'rummytiles.zone.table': 'Table',
  'rummytiles.zone.tray': 'Tray',
  'rummytiles.offer.place': 'Place',
  'rummytiles.offer.addFromHand': 'Add',
  'rummytiles.offer.addFromTray': 'Add from tray',
  'rummytiles.offer.take': 'Take',
  'rummytiles.offer.split': 'Split',
  'rummytiles.offer.swapJoker': 'Swap the joker',
  'rummytiles.offer.resetTurn': 'Reset turn',
  'rummytiles.offer.commit': 'Done',
  'rummytiles.offer.draw': 'Draw',
  'rummytiles.param.position': 'Split at',

  // --- blackjack -----------------------------------------------------------
  //
  // The house rules are worded twice over — on and off — because a rule that
  // is *not* in force is still something a player has to be told. "No
  // surrender at this table" is information; silence is a guess.
  'err.BET_BELOW_MINIMUM': 'That is under the table minimum',
  'err.ALREADY_BET': 'Your stake is already up',
  'err.INSURANCE_CLOSED': 'There is no insurance to take right now',
  'err.CANNOT_DOUBLE': "This hand can't be doubled",
  'err.CANNOT_SPLIT': "This hand can't be split",
  'err.CANNOT_SURRENDER': "This hand can't be surrendered",

  'blackjack.rules.section.table': 'The table',
  'blackjack.rules.section.play': 'Playing a hand',
  'blackjack.rules.section.dealer': 'The dealer',
  'blackjack.rules.section.end': 'How the match ends',
  'blackjack.rules.goal':
    'Beat the dealer without going over twenty-one. Going over loses at once, whatever the dealer does afterwards.',
  // Counts are their own phrase rather than a number glued to a noun: one
  // deck and six decks inflect differently, and Czech inflects them again.
  'blackjack.rules.decks': 'Decks in the shoe: {n}.',
  'blackjack.rules.stack': 'Every seat sits down with {n} chips.',
  'blackjack.rules.minBet': 'The table minimum is {n} chips.',
  'blackjack.rules.faceUp':
    'Player cards are dealt face up; the dealer keeps one card down until everyone has played.',
  'blackjack.rules.hitStand': 'Draw as many cards as you like, or stand on what you have.',
  'blackjack.rules.aces': 'An ace counts as eleven while that fits, and as one when it does not.',
  'blackjack.rules.blackjack':
    'An ace with a ten-value card, on the first two cards, is a blackjack.',
  'blackjack.rules.pays3to2': 'A blackjack pays 3:2.',
  'blackjack.rules.pays6to5': 'A blackjack pays 6:5.',
  'blackjack.rules.paysEven': 'A blackjack pays even money.',
  'blackjack.rules.double':
    'On your first two cards you may double your stake and take exactly one more card.',
  'blackjack.rules.doubleAfterSplit': 'A hand that came out of a split may be doubled too.',
  'blackjack.rules.noDoubleAfterSplit': 'A hand that came out of a split may not be doubled.',
  'blackjack.rules.split':
    'Two cards of the same value may be split into hands of their own, each with its own stake — up to {n} times, for {hands} hands in all.',
  'blackjack.rules.noSplit': 'Pairs are not split at this table.',
  'blackjack.rules.splitAces':
    'Split aces take one card each and then stand, and twenty-one made that way is not a blackjack.',
  'blackjack.rules.surrender':
    'You may give up your first hand for half its stake, once the dealer has checked for blackjack.',
  'blackjack.rules.noSurrender': 'Hands cannot be surrendered at this table.',
  'blackjack.rules.dealerDraws': 'The dealer draws to seventeen and then stands.',
  'blackjack.rules.hitsSoft17': 'The dealer draws to a seventeen made with an ace.',
  'blackjack.rules.standsSoft17': 'The dealer stands on a seventeen made with an ace.',
  'blackjack.rules.dealerPeeks':
    'Showing an ace or a ten, the dealer checks for blackjack before anybody plays.',
  'blackjack.rules.insurance':
    'Against a dealer ace you may insure for half your stake; it pays 2:1 if the dealer has blackjack.',
  'blackjack.rules.noInsurance': 'Insurance is not offered at this table.',
  'blackjack.rules.rounds': 'The table plays {n} rounds.',
  'blackjack.rules.mostChipsWins': 'Whoever holds the most chips at the end wins the match.',
  'blackjack.rules.bustedOut':
    'A seat that can no longer cover the minimum of {n} sits out the rest of the match.',
  'blackjack.rules.roundOrder':
    'A round goes in order: stakes up, cards dealt, insurance if the dealer shows an ace, then each seat plays its hand in turn.',
  'blackjack.rules.oneStakePerRound': 'One stake per round — once it is up it cannot be changed.',
  'blackjack.rules.stakeFromStack': 'You can only stake chips you actually hold.',
  'blackjack.remedy.putAStakeUp': 'Put a stake up first — {n} or more.',
  'blackjack.remedy.answerInsurance': 'Say yes or no to insurance first.',
  'blackjack.remedy.playThisHand': 'Play the hand in front of you — hit, or stand.',
  'blackjack.remedy.stakeIsUp': 'Your stake is already up — wait for the deal.',
  'blackjack.remedy.stakeAtLeast': 'Stake at least {n}.',
  'blackjack.remedy.stakeAtMost': 'Stake at most {n} — that is your whole stack.',
  'blackjack.remedy.sayHowMuch': 'Say how much to stake — {n} or more.',
  'blackjack.remedy.hitOrStand': 'Hit, or stand.',
  'blackjack.remedy.waitForNextRound': 'You are out of this round — wait for the next deal.',

  'blackjack.zone.dealer': 'Dealer',
  'blackjack.zone.box': 'Hand',
  'blackjack.zone.yourBox': 'Your hand',
  'blackjack.zone.shoe': 'Shoe',

  'blackjack.header.round': 'Round {n} of {of}',
  'blackjack.header.minBet': 'Minimum',
  'blackjack.header.decks': 'Decks',
  'blackjack.header.dealerTotal': 'Dealer shows {n}',
  'blackjack.header.dealerSoftTotal': 'Dealer shows soft {n}',

  'blackjack.seat.stack': 'Chips',
  'blackjack.seat.bet': 'Stake',
  'blackjack.seat.insurance': 'Insurance',
  'blackjack.seat.total': 'Total',
  'blackjack.seat.softTotal': 'Soft total',
  'blackjack.seat.out': 'Out of chips',

  'blackjack.prompt.placeBet': 'Place your stake',
  'blackjack.prompt.insurance': 'Insurance?',
  'blackjack.prompt.yourMove': 'Your move',
  'blackjack.prompt.waitingFor': 'Waiting for {playerId}',
  'blackjack.prompt.betAmount': 'Stake',

  'blackjack.quick.doubleMin': '2× Minimum',
  'blackjack.quick.allIn': 'All-in',
  'blackjack.offer.bet': 'Bet',
  'blackjack.offer.hit': 'Hit',
  'blackjack.offer.stand': 'Stand',
  'blackjack.offer.double': 'Double down',
  'blackjack.offer.split': 'Split',
  'blackjack.offer.surrender': 'Surrender',
  'blackjack.offer.insure': 'Take insurance',
  'blackjack.offer.declineInsurance': 'No insurance',

  'blackjack.fact.tableMinimum': 'minimum',
  'blackjack.fact.insuranceCost': 'to insure',
  'blackjack.fact.extraStake': 'to stake',
  'blackjack.fact.surrenderReturn': 'back',

  'blackjack.status.dealerBlackjack': 'The dealer had blackjack',
  'blackjack.status.dealerBust': 'The dealer went bust with {n}',
  'blackjack.status.dealerStands': 'The dealer stands on {n}',

  'blackjack.round.name': 'Round',
  'blackjack.round.dealerTotal': 'Dealer {n}',
  'blackjack.round.dealerBust': 'Dealer bust ({n})',
  'blackjack.round.dealerBlackjack': 'Dealer blackjack',
  'blackjack.round.outcome.blackjack': 'Blackjack',
  'blackjack.round.outcome.win': 'Won',
  'blackjack.round.outcome.push': 'Push',
  'blackjack.round.outcome.lose': 'Lost',
  'blackjack.round.outcome.bust': 'Bust',
  'blackjack.round.outcome.surrender': 'Surrendered',

  'blackjack.badge.inPlay': 'In play',
  'blackjack.badge.doubled': 'Doubled',
  'blackjack.badge.split': 'Split',
  'blackjack.badge.blackjack': 'Blackjack',
  'blackjack.badge.bust': 'Bust',
  'blackjack.badge.won': 'Won',
  'blackjack.badge.push': 'Push',
  'blackjack.badge.lost': 'Lost',
  'blackjack.badge.surrendered': 'Surrendered',

  'blackjack.unit.chips': 'chips',

  // --- the settings screen --------------------------------------------------
  //
  // Hardcoded English until now, which was survivable while the only other
  // language was unreachable and faintly absurd once the language picker
  // itself lived on this screen: a player who cannot read "Settings" is
  // exactly the one who came here to change it.
  'settings.title': 'Settings',
  'settings.signedInAs': 'Signed in as {username}',
  'settings.playingAsGuest': 'Playing as {username} (guest)',
  'settings.notSignedIn': 'Not signed in — sign in or continue as a guest to play online.',
  'settings.subtitle': 'How you look, and how the table does',
  'settings.face.heading': 'Your face at the table',
  'settings.face.account': 'Kept with your account, so it follows you to another device.',
  'settings.face.device': 'Kept on this device. Sign in to carry it with you.',
  'settings.skin.heading': 'Table look',
  'settings.language.heading': 'Language',
  'settings.language.status': 'Kept on this device.',
  'settings.language.auto': 'Automatic',
  'settings.language.auto.now': 'Follows your device — now {language}',
  'settings.legal.heading': 'The small print',
  'settings.legal.status': 'What you agreed to by playing, and what is stored about you.',
  'settings.signIn': 'Sign in',
  'settings.back': 'Back',

  // A translated interface over an English notice is a worse lie than an
  // English interface, so the document says so itself rather than letting the
  // surrounding screen imply the text was written for this reader.
  'legal.untranslated': 'This notice has not been translated into your language yet. The English text below is the version that applies.',

  // --- the navigation bar's screen titles ------------------------------------
  //
  // These sit in `app/_layout.tsx`, above every screen, and were the last
  // English left after the settings screen was translated — a German player
  // reading "Einstellungen" under a bar that said "Settings". A translated app
  // with an untranslated chrome bar looks like a bug, not like a limit.
  'nav.home': 'Jokerless',
  'nav.emailSignIn': 'Email sign-in',
  'nav.signingIn': 'Signing in',
  'nav.usernameSignIn': 'Sign in with username',
  'nav.legacyAccount': 'Legacy account',
  'nav.guest': 'Guest',
  'nav.account': 'Account',
  'nav.games': 'Games',
  'nav.table': 'Your table',
  'nav.join': 'Join a table',
  // The screen a shared invite link lands on while it resolves the code.
  'nav.joining': 'Joining',
  'nav.rules': 'Rules',
  'nav.match': 'Match',
  'nav.scoreTable': 'Score table',
  'nav.stats': 'Stats',
  'nav.more': 'More',
  'nav.myGames': 'My games',
  'nav.about': 'About',

  // --- the account menu behind the face in the corner ----------------------
  'menu.label': 'Account menu',
  'menu.signedIn': 'Signed in',
  'menu.notSignedIn': 'Not signed in',
  'menu.keepStats': 'to keep your stats',
  'menu.signOut': 'Sign out',
  'more.scoreTable': 'Offline score table',
  'more.stats': 'Stats & leaderboard',
  'more.needsAccount': 'sign in to use',
  'gate.title': 'Sign in to use this',
  'gate.body': 'Score tables and stats are kept with your account, so they follow you to another device. A guest has nowhere for them to be kept.',

  // --- screens that never reached a key at all -------------------------------
  //
  // Everything below was typed straight into JSX. The parity tests could not
  // see it — they prove every *key* is worded in twenty-four languages, not
  // that a sentence ever became a key — so the app read half in the player's
  // language and half in English. `scripts/find-untranslated.js` is the gate
  // that stops that recurring; these are what it found.

  // Errors shown when a thrown error carries no message of its own. The server
  // sends codes, not sentences (see `reasonText`); these cover the cases where
  // nothing came back at all — a socket that died, a fetch that never landed.
  'error.generic': 'That did not work',
  'error.signIn': 'Sign-in failed',
  'error.login': 'Login failed',
  'error.register': 'Registration failed',
  'error.sendCode': 'Could not send a code',
  'error.badCode': 'That code did not work',
  'error.rulesLoad': 'Could not load the rules',
  'error.createFailed': 'Create failed',
  'error.saveFailed': 'Save failed',
  'error.exportFailed': 'Export failed',

  // --- the route that does not exist ----------------------------------------
  'notFound.title': 'Oops!',
  'notFound.message': "This screen doesn't exist.",
  'notFound.home': 'Go to home screen!',

  // --- signing in -----------------------------------------------------------
  'auth.login.subtitle': 'Keep your statistics across devices',
  'auth.login.continueWithEmail': 'Continue with email',
  'auth.login.usernameInstead': 'Sign in with a username instead',
  'auth.email.title': 'Sign in with email',
  'auth.email.subtitle': "We'll email you a one-time code",
  'auth.email.address': 'Email address',
  'auth.email.send': 'Send code',
  'auth.email.codeTitle': 'Enter the code',
  'auth.email.codePlaceholder': '6-digit code',
  'auth.email.differentAddress': 'Use a different address',
  'auth.email.sentTo': 'Sent to {email}',
  'auth.email.continue': 'Continue',
  'auth.guest.title': 'Guest play',
  'auth.guest.subtitle': 'No account required',
  'auth.guest.displayName': 'Display name',
  'auth.register.title': 'Create account',
  'auth.register.username': 'Username',
  'auth.register.email': 'Email (optional)',
  'auth.register.password': 'Password',
  'auth.username.createAccount': 'Create a username/password account',
  'auth.callback.signedIn': 'Signed in.',

  // --- the account screen ---------------------------------------------------
  'account.signInPrompt': 'Sign in to manage your account.',
  'account.keepGames': 'Keep these games',
  'account.signedInWith': 'Signed in with',
  'account.addMethod': 'Add a sign-in method',
  'account.usernameAndPassword': 'Username and password',
  'account.faceAndTable': 'Face and table look',
  'account.refresh': 'Refresh',
  'account.remove': 'Remove',

  // --- the first-run intro screen (app/intro.tsx) ---------------------------
  //
  // Game names are deliberately not keyed here — see gameLabels.ts's own
  // comment on why "Žolíky", "Prší", "Canasta" and "Hold'em" stay as literals
  // in the component instead of a key per language.
  'intro.tagline': 'Classic card games, played online with real people',
  'intro.bulletFriends': 'Play with friends, or get seated with others online',
  'intro.bulletCrossDevice': 'One account, phone or browser — pick up where you left off',
  'intro.bulletGuest': 'No installs to try it — jump in as a guest',

  // --- the main menu --------------------------------------------------------
  'home.subtitle': 'Continental Rummy · {server}',
  'home.playingAs': 'Playing as {name}',
  'home.signInPrompt': 'Sign in or continue as guest to play online.',
  'home.statsAndLeaderboard': 'Stats & leaderboard',
  'home.play': 'Play',
  'home.offlineScoreTable': 'Offline score table',
  'home.signInToKeepStats': 'Sign in to keep your stats',
  'home.signOut': 'Sign out',
  'home.continueAsGuest': 'Continue as guest',
  // Someone playing as a guest, marked in a list of names.
  'home.guestSuffix': '(guest)',

  // --- the waiting room on the main menu ------------------------------------
  //
  // Counts are whole phrases per count, not a number glued to a noun — the
  // same reason `countLabel` exists. One and many are the cases the interface
  // actually produces, so they are the cases that get their own wording.
  'waiting.checking': "Checking who's around…",
  'waiting.youAreWaiting': "You're waiting to play",
  'waiting.pickedUp': "Anyone opening a table can pick you up — they don't need a code from you.",
  'waiting.othersOne': '1 other player is waiting too',
  'waiting.othersMany': '{n} other players are waiting too',
  'waiting.oneWaiting': '1 player is waiting to play',
  'waiting.manyWaiting': '{n} players are waiting to play',
  'waiting.adding': 'Adding you to the waiting list…',
  'waiting.slowHint':
    "If this doesn't finish in a few seconds, check the server address below is reachable from this device.",
  'waiting.serverBusyDetail':
    'Attempt {n}. The server is not taking new waiting-room connections right now.',
  'waiting.reconnecting': 'Connection lost — reconnecting…',
  'waiting.reconnectingDetail':
    "Attempt {n}. This can happen if your device's network changed, or the server restarted.",
  'waiting.tryAgain': 'Try again now',
  'waiting.makeAvailable': 'Make me available to play',
  'waiting.stop': 'Stop waiting',
  'waiting.noneYet':
    "Nobody is waiting to play right now. Put yourself on the list and you'll be the first anyone sees.",
  'waiting.noOthersYet': 'Nobody else is waiting yet. Hosts can still see you and invite you.',
  'waiting.server': 'Server',
  'waiting.none': 'No one is waiting right now. Anyone who makes themselves available on the main menu shows up here.',

  // --- landing on a shared invite link --------------------------------------
  'join.missingCode': 'That link is missing its table code.',
  'join.staleLink': 'Ask whoever invited you for a fresh link, or join with the code instead.',
  'join.enterCode': 'Enter a code',
  'join.backToMenu': 'Back to the menu',
  'join.takingSeat': 'Taking a seat…',
  'join.takingSeatAt': 'Taking a seat at {game}…',

  // --- the lobby ------------------------------------------------------------
  'lobby.games.subtitle': 'Everything this server can host',
  'lobby.games.bots': 'Bots',
  'lobby.games.setup': 'Settings',
  'lobby.games.playBot': 'Play against a bot',
  'lobby.games.playBots': 'Play against {n} bots',
  'lobby.games.openTable': 'Open a table',
  'lobby.games.players': '{n} players',
  'lobby.games.playerRange': '{min}–{max} players',
  'lobby.join.placeholder': 'Join code or invite link',
  'lobby.join.needCode': 'Enter a join code, a link, or a match ID',
  'lobby.games.signInFirst': 'Sign in first',
  'lobby.join.action': 'Join',
  'lobby.join.waitingTitle': 'Waiting for the host',
  // Two whole sentences rather than one with a swapped noun: "a game of {game}"
  // does not survive a language that inflects the game's name after "of".
  'lobby.join.joinedGame': 'Joined a game of {game} — waiting to start',
  'lobby.join.joinedTable': 'Joined the table — waiting to start',
  'lobby.table.addBot': 'Add a bot',
  'lobby.table.side': 'Side {n}',
  'lobby.table.shuffleSeats': 'Shuffle the seats',
  'lobby.table.moveSeatUp': 'Move {name} up a seat',
  'lobby.table.moveSeatDown': 'Move {name} down a seat',
  'lobby.table.start': 'Start',
  'lobby.table.waitingForHost': 'Waiting for the host to start…',

  // --- "my games": stored tables, resumed or deleted --------------------
  'mine.subtitle': 'Games you can pick back up, or clear away',
  'mine.tabUnfinished': 'In progress',
  'mine.tabFinished': 'Finished',
  'mine.emptyUnfinished': 'Nothing in progress right now',
  'mine.emptyFinished': 'No finished games yet',
  'mine.open': 'Open',
  'mine.resume': 'Resume',
  'mine.delete': 'Delete',
  'mine.deleteConfirmTitle': 'Delete this table?',
  'mine.deleteConfirmBody': 'This ends the game for everyone still seated. This cannot be undone.',
  'mine.deleteConfirm': 'Delete',
  'mine.deleteCancel': 'Cancel',
  'mine.viewAll': 'View all',
  'mine.status.lobby': 'Waiting to start',
  'mine.status.active': 'In progress',
  'mine.status.suspended': 'Paused',
  'mine.status.abandoned': 'Set aside',
  'mine.status.completed': 'Finished',

  // --- inviting someone to a table ------------------------------------------
  'invite.heading': 'Invite players',
  'invite.explain': 'Send this link. Whoever opens it lands at this table — no account needed.',
  'invite.noAddress': 'This server has no shareable address configured, so use the code below.',
  'invite.readOutCode': 'Or read out the code:',
  'invite.copy': 'Copy link',
  'invite.share': 'Share link',
  'invite.copied': 'Copied!',
  'invite.shared': 'Shared',

  // --- the match screen -----------------------------------------------------
  'match.waitingForTable': 'Waiting for the table…',
  'match.waitingForPlayer': 'Waiting for another player…',
  'match.nobodyWon': 'Nobody won.',
  'match.youWon': 'You won.',
  'match.finished': 'This match has finished.',
  'match.inProgress': 'Match in progress — everything is connected and moving normally.',
  'match.connecting': 'Connecting…',
  'match.abandonedTitle': 'Table set aside',
  'match.abandoned': 'Nobody came back to this table, so it was set aside. The cards are exactly where you left them.',
  'match.resume': 'Pick up where you left off',
  'match.resuming': 'Bringing the table back…',
  // Why a swept-up table is not offering the resume above. Everyone at it has
  // to be looking at it before one player may bring it back, so this names
  // the people it is still waiting for — something the reader can act on, by
  // sending them the link — rather than leaving a banner whose only button is
  // the way out.
  'match.abandonedWaitingFor': 'Waiting for {names} to come back to the table.',
  'match.controls': 'Controls',
  'match.over': 'Match over',
  'match.settingUp': 'Setting up…',
  'match.playAgain': 'Play again',
  'match.backToGames': 'Back to games',
  'match.table': 'Table',
  'match.opponents': 'Opponents',
  // Marks which seat is the reader's own, in a list of seats.
  'match.youSuffix': '(you)',
  // The winner line is composed from whole sentences rather than a name glued
  // to " won." — the verb agrees with the subject in most of these languages,
  // and a suffix cannot know that.
  'match.you': 'you',
  'match.yourTeam': 'Your team',
  'match.teammates': 'Team: {names}',
  'match.someoneWon': '{name} won.',
  'match.wonBy': 'Won by {names}.',
  'match.pausedFor': 'Paused — waiting for {name} to reconnect.',
  'match.results': 'Results',
  'match.players': 'Players',
  'match.toPlay': 'to play',

  // --- the offline score table ----------------------------------------------
  'scoring.namesHint': 'Comma-separated names (4–8 players)',
  'scoring.newSession': 'New session',
  // A worked example, not a sentence: the names are placeholders a translator
  // may localise, the shape `name:score,` is what the parser needs.
  'scoring.scoresPlaceholder': 'Alice:120,Bob:80,…',
  'scoring.saveRound': 'Save round',
  'scoring.export': 'Export scorecard',
  'scoring.formatHint': 'Scores format: Name:100,Name2:50',
  'scoring.nameCountError': 'Enter 2–8 comma-separated player names',
  'scoring.session': 'Session: {id}',
  'scoring.players': 'Players: {names}',
  'scoring.roundScores': 'Round {n} scores',
  'stats.loading': 'Loading…',
  // The figures could not be fetched. `{reason}` is whatever the server or the
  // network said, which is not ours to translate — the frame around it is.
  'stats.unavailable': '(unavailable: {reason})',

  // --- statistics -----------------------------------------------------------
  'stats.title': 'Stats & leaderboard',
  'stats.yours': 'Your stats',
  'stats.leaderboard': 'Leaderboard',

  // --- a player's lifetime record, shown beside a finished match ------------
  'record.title': 'Your record',
  'record.guest':
    'You are playing as a guest, so no record is being kept. Sign in and the games you have already played on this device — including this one — are kept with your account.',
  'record.signInToKeep': 'Sign in and keep these',
  'record.failed': 'Your record could not be loaded just now. The match is safely recorded.',
  'record.loading': 'Loading…',
  'record.played': 'Played',
  'record.won': 'Won',
  'record.lost': 'Lost',
  'record.winRate': 'Win rate',
  'record.streak': 'Streak',
  'record.atThisGame': 'At this game',
  // A streak is a count with a noun, so it is a whole phrase per count for
  // the same reason `countLabel` is: "1 win" / "2 wins" is an English rule,
  // and most of these languages do not share it.
  'record.streakNone': '—',
  'record.streakWinOne': '1 win',
  'record.streakWinMany': '{n} wins',
  'record.streakLossOne': '1 loss',
  'record.streakLossMany': '{n} losses',

  // --- moving cards ---------------------------------------------------------
  'hand.dragHint': 'Drag a card along the fan to rearrange it, or onto the board to play it',
  'hand.moveLeft': 'Move left',
  'hand.moveRight': 'Move right',
  'hand.openFan': 'Spread the cards out',
  'hand.closeFan': 'Close the cards up',
  'zone.collapseGroup': 'Collapse this group',
  'zone.expandGroup': 'Show all cards in this group',
  'zone.dropHere': 'Drop here',
  'offer.pickCards': 'pick cards for the place you tapped',
  'offer.ambiguous': 'more than one place this could go — pick on the board',

  // --- the build footer -----------------------------------------------------
  'build.app': 'app',
  'build.server': 'server',
  'about.subtitle': 'The build you are playing, and the small print.',
  // --- the words the server chose, worded here -------------------------------
  //
  // /modules describes each game, variation, option and choice with an English
  // label. That made the lobby the one screen that stayed English whatever the
  // picker said. `src/lib/gameLabels.ts` looks these up and falls back to the
  // label the server sent, so a game added after this build still reads.
  //
  // Absent on purpose: game names, place names, ratios and bare numbers.
  // "Texas Hold'em", "Vegas Strip", "3:2" and "500" read the same in every
  // language and fall through to the server's own label.
  'option.pauseBetweenRounds': 'Pause between rounds',
  'choice.pauseBetweenRounds.1': 'Pause',
  'choice.pauseBetweenRounds.0': 'Play straight on',
  'option.openDiscardPile': 'Discard pile',
  'choice.openDiscardPile.1': 'Open to look through',
  'choice.openDiscardPile.0': 'Top card only',
  'option.botSkill': 'Opponents',
  'choice.botSkill.0': 'Mixed',
  'choice.botSkill.1': 'Easy',
  'choice.botSkill.2': 'Medium',
  'choice.botSkill.3': 'Hard',
  'option.initialMeldMinimum': 'Meld value',
  'choice.initialMeldMinimum.0': 'Off',
  'option.discardDrawMinRound': 'Discard pickup',
  'choice.discardDrawMinRound.0': 'Open',
  'choice.discardDrawMinRound.2': 'Round 2',
  'choice.discardDrawMinRound.3': 'Round 3',
  'option.requireCleanRun': 'Clean run',
  'choice.requireCleanRun.1': 'Required',
  'choice.requireCleanRun.0': 'Off',
  'option.jokerReclaimMustPlay': 'Reclaimed joker',
  'choice.jokerReclaimMustPlay.1': 'Play same turn',
  'choice.jokerReclaimMustPlay.0': 'May be kept',
  'option.dealStarter': 'Deal starter',
  'choice.dealStarter.0': 'Rotate',
  'choice.dealStarter.1': 'Winner leads',
  'variation.prsi.classic': 'Classic',
  'option.handSize': 'Cards dealt',
  'variation.canasta.classic': 'Classic',
  'variation.canasta.modern_american': 'Modern American',
  'variation.canasta.samba': 'Samba',
  'option.targetScore': 'Target score',
  'option.canastasToGoOut': 'Canastas to go out',
  'variation.holdem.freezeout': 'Freezeout',
  'variation.holdem.timed': 'Fixed hands',
  'option.startingStack': 'Starting chips',
  'option.bigBlind': 'Big blind',
  'option.handLimit': 'Hands',
  'choice.handLimit.0': 'Until one seat is left',
  'variation.ginrummy.standard': 'Standard',
  'option.knockLimit': 'Knock limit',
  'choice.knockLimit.0': 'Oklahoma (the upcard sets it)',
  'option.bigGin': 'Big gin',
  'choice.bigGin.0': 'Off',
  'choice.bigGin.1': 'On (+25)',
  'option.lineBonuses': 'Line bonuses',
  'choice.lineBonuses.1': 'On',
  'choice.lineBonuses.0': 'Off',
  'variation.rummytiles.standard': 'Standard',
  'choice.targetScore.0': 'Off',
  // Two games qualify a number the others mean plainly: Canasta's 500 is a
  // short game, Gin Rummy's and Rummy Tiles' 500 is just 500; Hold'em's
  // starting 200 is a short stack, Blackjack's 200 is just 200. Module-scoped
  // so the qualifier travels with the game that means it — a bare key would
  // hand the note to games it is false for. `scripts/check-server-labels.js`
  // is what finds these; both were shipping as English on a Czech screen.
  'choice.canasta.targetScore.500': '500 (short)',
  'choice.holdem.startingStack.200': '200 (short)',
  'option.roundLimit': 'Round limit',
  'choice.roundLimit.0': 'Off',
  'option.poolExhaustion': 'Pool exhaustion',
  'choice.poolExhaustion.1': 'Lowest hand wins the round',
  'choice.poolExhaustion.0': 'Nobody wins the round',
  'variation.blackjack.single': 'Single deck',
  'option.minBet': 'Table minimum',
  'option.rounds': 'Rounds',
  'option.decks': 'Decks',
  'option.dealerHitsSoft17': 'Dealer on soft 17',
  'choice.dealerHitsSoft17.0': 'Stands',
  'choice.dealerHitsSoft17.1': 'Hits',
  'option.blackjackPays': 'Blackjack pays',
  'choice.blackjackPays.100': 'Even money',
  'option.maxSplits': 'Splitting',
  'choice.maxSplits.0': 'No splitting',
  'choice.maxSplits.1': 'Once (two hands)',
  'choice.maxSplits.3': 'Three times (four hands)',
  'option.doubleAfterSplit': 'Double after split',
  'choice.doubleAfterSplit.1': 'Allowed',
  'choice.doubleAfterSplit.0': 'Not allowed',
  'option.surrender': 'Surrender',
  'choice.surrender.0': 'Off',
  'choice.surrender.1': 'Late surrender',
  'option.insurance': 'Insurance',
  'choice.insurance.1': 'Offered',
  'choice.insurance.0': 'Not offered',

  // --- the verbs on the buttons ---------------------------------------------
  //
  // An offer without a `labelKey` of its own is labelled from its raw verb —
  // `label(offer.labelKey ?? `verb.${offer.verb}`)` in `OfferBar`. That key is
  // built on this side, so `cmd/dump-keys` never sees it and `serverKeys.json`
  // does not list it: the parity tests all passed while "Discard", "Lay meld",
  // "Undo draw" and "Undo lay off" sat in English on a Czech board, because
  // `humanise()` turned the key into English nobody had written and no search
  // for an English *string* could find.
  //
  // Worded here from the server's own `OfferVerb` constants rather than from
  // what one run happened to render, so a verb that only appears in a state
  // the sweep never reached is covered too.
  'verb.add': 'Add',
  'verb.bet': 'Bet',
  'verb.call': 'Call',
  'verb.check': 'Check',
  'verb.commit': 'Done',
  'verb.continue': 'Continue',
  'verb.decline_insurance': 'No insurance',
  'verb.discard': 'Discard',
  'verb.double': 'Double down',
  'verb.draw': 'Draw',
  'verb.finish_layoff': 'Done laying off',
  'verb.fold': 'Fold',
  'verb.hit': 'Hit',
  'verb.insure': 'Take insurance',
  'verb.knock': 'Knock',
  'verb.lay_meld': 'Meld',
  'verb.lay_off': 'Lay off',
  'verb.pass': 'Pass',
  'verb.place': 'Place',
  'verb.play_card': 'Play',
  'verb.raise': 'Raise',
  'verb.reset_turn': 'Reset turn',
  'verb.split': 'Split',
  'verb.stand': 'Stand',
  'verb.surrender': 'Surrender',
  'verb.swap_joker': 'Swap the joker',
  'verb.take': 'Take',
  'verb.take_pile': 'Take from pile',
  // Declared by the server and listed in `serverKeys.json`, but never worded.
  'verb.takePileFromHand': 'Take the pile into your hand',
  'verb.takePileOntoMeld': 'Take the pile onto a meld',
  'verb.takeTopForSequence': 'Take the top card onto a sequence',
  'verb.undoDraw': 'Undo draw',
  'verb.undoLayOff': 'Undo lay off',
  'verb.undoMeld': 'Undo meld',
  'verb.undoTakePile': 'Undo take pile',
  'verb.undoTurn': 'Undo turn',

  // --- suits, spelled out ---------------------------------------------------
  //
  // Read aloud by a screen reader and shown where a pip would not fit. The
  // pips themselves are drawn, not written, so these are the only place the
  // suit is ever a word.
  'suit.C': 'Clubs',
  'suit.D': 'Diamonds',
  'suit.H': 'Hearts',
  'suit.S': 'Spades',

  // --- counters the board prints beside a number ----------------------------
  'canasta.seat.notOpened': 'Not opened',
  'canasta.unit.points': 'points',
  'ginrummy.unit.points': 'points',
  'holdem.seat.dealer': 'Dealer',
  'holdem.seat.folded': 'Folded',
  'holdem.seat.allIn': 'All in',
  'holdem.seat.out': 'Out',
  'holdem.unit.chips': 'chips',
  'prsi.unit.cardsLeft': 'cards left',
  'rummytiles.prompt.initialMeld': 'Your first lay must be worth {n} points.',
  'rummytiles.unit.points': 'points',
  'zolik.unit.penalty': 'penalty',
  'header.pileFrozen': 'Pile frozen',
  // The module-less form of `ginrummy.prompt.yourTurnDraw`: Žolíky and Canasta
  // send the prompt without a module prefix. Neither the Go constants nor
  // `serverKeys.json` list it — the sweep is what found it.
  'prompt.yourTurnDraw': 'Draw a card',
  'prompt.yourTurnMeld': 'Meld if you can, then discard',

  // --- replay: stepping through a game that has stopped ---
  'nav.replay': 'Replay',
  'mine.replay': 'Replay',
  'replay.loading': 'Opening the replay…',
  'replay.failed': 'The replay could not be loaded',
  'replay.openHands': 'every hand face up',
  'replay.truncated': 'This game replays as far as move {at} — its rules have changed since it was played',
  'replay.position': '{at} of {of}',
  'replay.theDeal': 'The deal',
  'replay.round': 'Round',
  'replay.track.all': 'All',
  'replay.track.yours': 'Your moves',
  'replay.track.rounds': 'Rounds',
  'replay.track.marks': 'Marks',
  'replay.play': 'Play',
  'replay.pause': 'Pause',
  'replay.move': '{player} · {move}',

  // --- the showdown --------------------------------------------------------
  //
  // Hold'em stops after every hand with the cards on the table, and a player
  // may turn their own hand over before the next one is dealt. Most of these
  // earn a line for the usual reason — what they mean lives in their params —
  // and the rest because `humanise` would render "Shown Hand" or "Mucked" as
  // English that looks written rather than missing.
  'err.ALREADY_SHOWN': 'Your hand is already face up',
  'err.NOTHING_TO_SHOW': 'You have no hand to show',
  'holdem.offer.show': 'Show my hand',
  'holdem.offer.finish': 'Finish the match',
  'holdem.remedy.nothingToShow': 'You were not dealt into this hand — wait for the next one.',
  'holdem.rules.section.showdown': 'After the hand',
  'holdem.rules.stopEveryHand': 'Play stops after every hand, so the cards can be seen.',
  'holdem.rules.revealEveryone': 'At a showdown every hand that was called is turned face up.',
  'holdem.rules.revealWinners': 'At a showdown only the winning hand is turned face up.',
  'holdem.rules.showYourOwn': 'You may always turn your own hand face up, until you agree to go on.',
  'holdem.rules.showOnce': 'A hand can be shown only once.',
  'holdem.seat.won': 'Won',
  'holdem.seat.mucked': 'Mucked',
  'holdem.status.mucked': '{playerId} did not show',
  'holdem.status.shownVoluntary': '{playerId} showed their hand',
  'zone.shownHand': 'Shown',
  'option.showdownReveal': 'Show at showdown',
  'choice.showdownReveal.2': 'Everyone who was called',
  'choice.showdownReveal.1': 'The winner only',
  'home.playOffline': 'Play offline',
  'offline.title': 'Play offline',
  'offline.subtitle': 'No internet needed',
  'offline.body': 'This phone hosts the table itself, so you can play against bots anywhere — even in airplane mode.',
  'offline.nameLabel': 'Your name at the table',
  'offline.start': 'Start an offline table',
  'offline.starting': 'Starting…',
  'offline.active': 'You are at this phone\'s offline table. Games you start here stay on this phone.',
  'offline.chooseGame': 'Choose a game',
  'offline.backOnline': 'Back to online play',
  'offline.failed': 'The offline table did not start: {reason}',
  'offline.nearbyTitle': 'Tables nearby',
  'offline.nearbyLooking': 'Looking for tables on this network…',
  'offline.nearbyNothing': 'No tables found yet. Make sure you are on the same Wi-Fi or hotspot as the host, and that Zolik may use the local network.',
  'offline.joinThis': 'Join',
  'offline.byAddressTitle': 'Join by address',
  'offline.versionMismatch': 'This table runs a different version of Zolik. Update the app on both phones.',
  'offline.roomTitle': 'Players nearby',
  'offline.roomBody': 'Let other phones on this Wi-Fi or hotspot join your table. No internet needed.',
  'offline.roomOpenButton': 'Invite players nearby',
  'offline.roomOpen': 'Other phones on this network can now find your table. If they cannot see it, they can type the address or scan the code.',
  'offline.roomClose': 'Stop inviting',
  'offline.guestActive': 'You are at a table hosted by another phone ({host}).',
  'offline.joinFailed': 'Could not join that table: {reason}',
  'invite.offlineExplain': 'Players at this offline table join with this code, under “{menu}”.',
  'invite.offlineCode': 'Code:',
  'offline.bleTitle': 'Over Bluetooth',
  'offline.bleLook': 'Look for tables over Bluetooth',
  'offline.bleLooking': 'Looking for tables over Bluetooth…',
  'offline.bleOff': 'Turn Bluetooth on to find tables with no Wi-Fi.',
  'offline.bleDenied': 'Allow Zolik to use Bluetooth in Settings to play with no Wi-Fi.',
  'offline.bleInvite': 'Invite over Bluetooth',
  'offline.bleInviteBody': 'For phones that share no Wi-Fi with yours. No pairing needed.',
  'offline.bleOpen': 'Phones nearby can join over Bluetooth. Connected: {n}.',
  'offline.bleStop': 'Stop Bluetooth',
  'offline.bleCheck': 'Check code: {code}',
  'offline.guestBle': 'You are at a table hosted by another phone, over Bluetooth.',
  'offline.keyChanged': 'This is not the phone you played at before under that name. Ask the host, or join over Wi-Fi.',
  // --- game notifications: invites, the game circle, push (src/notify) ---
  'err.NOT_PLAYED_TOGETHER': 'You can only add people you have played with. Send them your friend link or ask by username.',
  'err.UNKNOWN_PLAYER': 'Nobody plays under that name.',
  'err.UNKNOWN_FRIEND_CODE': 'That friend link does not work any more.',
  'err.CANNOT_ADD_SELF': 'That is you. Add someone else.',
  'err.RATE_LIMITED': 'Too many at once. Try again in a few minutes.',
  'err.PUSH_UNAVAILABLE': 'Notifications are not set up on this server.',
  'notify.someone': 'Someone',
  'notify.join': 'Join',
  'notify.joining': 'Joining…',
  'notify.goToTable': 'Go to table',
  'notify.notNow': 'Not now',
  'notify.moreOptions': 'More options',
  'notify.muteHost': 'Stop invites from {host}',
  'notify.joinFailed': 'Could not join: {reason}',
  'notify.hostingOffline': 'You are hosting a table on this phone. Leave it first to join another.',
  'notify.on': 'On',
  'notify.off': 'Off',
  'notify.channelName': 'Game invites',
  'notify.banner.online': '{host} opened a table of {game}',
  'notify.banner.onlineNoGame': '{host} opened a table',
  'notify.banner.nearby': '{host} is hosting a table nearby',
  'notify.banner.seated': 'You have been seated at a table',
  'notify.banner.fromCircle': 'From your game circle',
  'notify.banner.known': 'You have played together before',
  'notify.banner.nearbyDetail': 'On a phone near you',
  'notify.banner.seatedDetail': 'A host picked you from the waiting room',
  'notify.banner.more': '+{n} more',
  'notify.pillOne': '1 table waiting',
  'notify.pillMany': '{n} tables waiting',
  'notify.waitingCard.title': 'Tables waiting for you',
  'notify.waitingCard.empty': 'No tables are waiting for you.',
  'notify.prompt.title': 'Get told when your circle starts a game',
  'notify.prompt.body': 'Only tables opened by people in your game circle. You can switch it off in Settings.',
  'notify.prompt.enable': 'Enable',
  'notify.prompt.later': 'Later',
  'notify.settings.heading': 'Notifications',
  'notify.settings.invites': 'Game invites',
  'notify.settings.invitesBody': 'Who may tell you when they open a table.',
  'notify.settings.invitesCircle': 'My circle',
  'notify.settings.invitesOff': 'Off',
  'notify.settings.nearby': 'Nearby tables',
  'notify.settings.nearbyBody': 'Show a banner when a phone near you opens a table. Works while the app is open.',
  'notify.settings.push': 'Notifications on this device',
  'notify.settings.pushOn': 'On. You are told about tables even when the app is closed.',
  'notify.settings.pushOff': 'Off. Invites only appear while the app is open.',
  'notify.settings.pushBlocked': 'Blocked. Allow notifications for this app in your phone\'s settings.',
  'notify.settings.pushBlockedWeb': 'Blocked. Allow notifications for this site in your browser\'s settings.',
  'notify.settings.pushInstall': 'On iPhone and iPad, add this site to your home screen first, then enable notifications there.',
  'notify.settings.pushUnavailable': 'This server does not send notifications.',
  'notify.settings.pushUnsupported': 'This device or browser cannot show notifications.',
  'notify.settings.signIn': 'Sign in or play as a guest to receive game invites.',
  'notify.announce.heading': 'Notify my circle',
  'notify.announce.body': 'The people in your game circle hear about this table and can join with one tap.',
  'notify.announce.toggle': 'Tell my circle when I open a table',
  'notify.announce.sending': 'Telling your circle…',
  'notify.announce.toldOne': 'Told 1 player',
  'notify.announce.toldMany': 'Told {n} players',
  'notify.announce.already': 'Your circle already knows about this table.',
  'notify.announce.nobody': 'Nobody to tell yet. Add people to your circle.',
  'notify.announce.circleLink': 'Game circle',
  'notify.addToCircle': 'Add {name} to your circle',
  'notify.addedToCircle': '{name} is in your circle',
  'notify.push.inviteTitle': '{host} opened a table',
  'notify.push.inviteBody': 'Come and play {game} with {host}. Tap to join.',
  'offline.tellNearby': 'Tell players nearby',
  'offline.tellNearbyBody': 'Phones in the room with the app open are offered this table.',
  'circle.title': 'Game circle',
  'circle.subtitle': 'The people you tell when you open a table, and the people who tell you.',
  'circle.signInRequired': 'Sign in or play as a guest to build a game circle.',
  'circle.requests.heading': 'Requests',
  'circle.requests.body': 'They would like to tell you when they open a table.',
  'circle.accept': 'Accept',
  'circle.decline': 'Decline',
  'circle.members.heading': 'Your circle',
  'circle.members.body': 'They hear about the tables you open for friends.',
  'circle.members.empty': 'Nobody yet. Add people below.',
  'circle.pending': 'Waiting for them to accept',
  'circle.since': 'Since {date}',
  'circle.remove': 'Remove',
  'circle.notifiers.heading': 'Who can invite you',
  'circle.notifiers.body': 'Mute anyone whose tables you would rather not hear about.',
  'circle.notifiers.empty': 'Nobody has added you yet.',
  'circle.mute': 'Mute',
  'circle.unmute': 'Unmute',
  'circle.muted': 'Muted',
  'circle.suggestions.heading': 'Played recently',
  'circle.suggestions.body': 'People you have shared a table with. Add them to tell them about your tables.',
  'circle.lastPlayed': 'Last played {date}',
  'circle.add': 'Add',
  'circle.added': '{name} is in your circle.',
  'circle.addSomeone.heading': 'Add someone',
  'circle.friendLink.body': 'Anyone who opens your friend link joins your circle, and you join theirs.',
  'circle.shareMessage': 'Join my game circle on Jokerless',
  'circle.username.body': 'Or ask by username. They have to accept.',
  'circle.username.placeholder': 'Username',
  'circle.username.submit': 'Send request',
  'circle.username.sent': 'Request sent to {name}.',
  'circle.settingsLink': 'Notification settings',
  'circle.addFriend.title': 'Add to game circle',
  'circle.addFriend.loading': 'Looking up the link…',
  'circle.addFriend.missing': 'This link has no friend code.',
  'circle.addFriend.body': '{name} invited you to their game circle. You will each hear when the other opens a table.',
  'circle.addFriend.confirm': 'Add {name}',
  'circle.addFriend.done': 'You and {name} are now in each other\'s circle.',
  'circle.addFriend.toCircle': 'Open game circle',
};
