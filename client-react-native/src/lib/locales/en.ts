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
  'canasta.round.inHand': 'Caught in hand {n}',
  'holdem.round.hand': 'Hand',
  'holdem.round.pot': 'Pot {n}',
  'holdem.round.uncontested': 'Everyone else folded',
  'seat.ready': 'Ready',
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

  // --- the legal notices ----------------------------------------------------
  // Only the furniture. The documents themselves are in `src/legal`, which is
  // a bundle of the same kind with a parity test of its own — prose that long
  // in a flat key map buries the keys this one exists for.
  //
  // The notice is five fragments rather than one sentence with two links glued
  // in, because Czech does not put the link where English does: "souhlasíš s
  // Podmínkami" inflects the noun the link is made of. Fragments let each
  // locale place and decline its own.
  'legal.terms': 'Terms',
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
  'ginrummy.zone.knockerHand': 'Knocked hand',
  'ginrummy.zone.melds': 'Melds',
  'ginrummy.prompt.upcardDecision': 'Take the upcard, or pass',
  'ginrummy.prompt.yourTurnDraw': 'Draw a card',
  'ginrummy.prompt.yourTurnDiscard': 'Discard — or knock, if you can',
  'ginrummy.prompt.layoff': 'Lay off deadwood, or finish',

  // --- rummy tiles -------------------------------------------------------------
  'err.TILE_NOT_IN_HAND': 'That tile is not in your hand',
  'err.TILE_DOES_NOT_FIT': "That doesn't fit there",
  'err.NO_SUCH_SET': 'That set is not on the table',
  'err.INITIAL_MELD_ONLY': 'You may only rearrange your own new sets before your first lay',
  'err.TABLE_NOT_VALID': "The table isn't valid yet",
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
  'settings.subtitle': 'How you look, and how the table does',
  'settings.signedInAs': 'Signed in as {username}',
  'settings.playingAsGuest': 'Playing as {username} (guest)',
  'settings.notSignedIn': 'Not signed in — sign in or continue as a guest to play online.',
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
  'nav.home': 'Zolik',
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
};
