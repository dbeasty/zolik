/**
 * Swedish. Rummy vocabulary: grupp for a set, svit for a run, talong for the stock, kasthög for the discard pile. Giv is a deal, rond a round — Swedish keeps the two apart where English does.
 */

export const sv: Record<string, string> = {
  // --- engine error codes (rules.RulesErrorCode) ---------------------------
  'err.NOT_YOUR_TURN': 'Det är inte din tur',
  'err.WRONG_PHASE': 'Inte möjligt just nu',
  'err.MUST_DRAW_FIRST': 'Dra ett kort innan du lägger ut',
  'err.GAME_SUSPENDED': 'Spelet är pausat',
  'err.GAME_NOT_ACTIVE': 'Spelet pågår inte',
  // --- runtime refusals, which are not the rules saying no -----------------
  //
  // Both were reachable long before they were worded, and a player who met
  // one was shown the code in capitals or, worse, nothing at all: an action
  // sent while the socket was down simply vanished, which reads as a legal
  // move being silently refused.
  'err.MATCH_NOT_ACTIVE': 'Bordet är pausat — vi väntar på att en spelare ska återansluta',
  'err.NOT_CONNECTED': 'Ingen anslutning till bordet — återansluter, försök sedan igen',
  // Why the "start the next round" control is greyed out. Without wording the
  // shell printed the code itself, so a player agreeing to go on was answered
  // with ALREADY_READY in capitals.
  'err.ALREADY_READY': 'Du är redo',
  'err.NOT_BETWEEN_ROUNDS': 'Ronden pågår fortfarande',
  'err.NOT_AT_THIS_TABLE': 'Du sitter inte vid det här bordet',
  'err.DISCARD_LOCKED': 'Kasthögen är låst tills vidare',
  'err.DISCARD_PILE_EMPTY': 'Kasthögen är tom',
  'err.NO_CARDS_LEFT': 'Inga kort kvar att dra',
  'err.ROUND_REQ_NOT_MET': 'Lägg ut din egen öppning först',
  'err.NEED_CLEAN_RUN': 'Du behöver en jokerfri svit på bordet för att räknas som utlagd',
  'err.INCOMPLETE_INITIAL_MELD': 'Avsluta din utläggning, eller ångra den, innan du kastar',
  'err.DISCARD_CARD_NOT_MELDED': 'Kortet du tog upp måste ingå i din kombination',
  'err.JOKER_DISCARD_FORBIDDEN': 'En joker får inte kastas',
  'err.NOTHING_TO_UNDO': 'Det finns inget att ångra',
  'err.NO_JOKER_IN_MELD': 'Ingen joker i den här kombinationen',
  'err.JOKER_SWAP_MISMATCH': 'Det kortet tar inte jokerns plats',
  'err.RECLAIMED_JOKER_NOT_MELDED': 'Jokern du tog från bordet måste spelas i en kombination den här turen',
  'err.RUN_TOO_LONG': 'Den sviten har redan sin fulla längd',
  'err.WRONG_RUN_END': 'Det kortet förlänger sviten i andra änden',
  'err.INVALID_MELD': 'Inget kort på din hand passar här',
  'err.CARD_NOT_IN_HAND': 'Det kortet finns inte på din hand',
  'err.MELD_BELOW_MINIMUM': 'Dina kombinationer saknar fortfarande poäng för att du ska få lägga ut',
  'err.MELD_NO_CONTRIBUTION': 'Den kombinationen för inte ditt krav framåt',
  'err.TOO_MANY_WILDS': 'För många jokrar i den kombinationen',
  'err.ADJACENT_WILDS': 'Två jokrar får inte ligga bredvid varandra',
  'err.ACE_BRIDGE': 'Ett ess kan inte binda ihop kung och tvåa',

  // --- contract phrasing ---------------------------------------------------
  // Built from the structured contract the server sends (sets/runs counts),
  // never from a deal-number lookup table.
  // Whole phrases per count rather than a number plus a pluralised noun:
  // Czech inflects the noun by count (skupina / skupiny / skupin) in a way no
  // "add an s" helper survives, and the counts that actually occur are 1-3.
  // A count outside that falls back to contract.sets.n.
  'contract.sets.1': 'En grupp',
  'contract.sets.2': 'Två grupper',
  'contract.sets.3': 'Tre grupper',
  'contract.sets.n': '{n} grupper',
  'contract.runs.1': 'En svit',
  'contract.runs.2': 'Två sviter',
  'contract.runs.3': 'Tre sviter',
  'contract.runs.n': '{n} sviter',
  'contract.any': 'Vilken giltig kombination som helst',
  'contract.cleanRunOnly': 'Valfri blandning av grupper och sviter — minst en svit måste vara jokerfri',
  'contract.cleanRunSuffix': '{base} — en svit måste vara jokerfri',

  // --- written rules ---------------------------------------------------------
  // Full-sentence keys a "see the rules" screen renders, one per module,
  // resolved server-side against the table's actual variation and options —
  // see internal/module/rules.go and each game's rules.go. Shared section
  // titles first, then one block per module in the order the four games were
  // added.
  'zolik.rules.section.goal': 'Mål',
  'zolik.rules.section.setup': 'Uppställning',
  'zolik.rules.section.turn': 'Din tur',
  'zolik.rules.section.melding': 'Lägga ut',
  'zolik.rules.section.end': 'Så slutar matchen',
  'zolik.rules.goal':
    'Var först med att tömma handen genom att lägga ut giltiga grupper och sviter, och samla så få straffpoäng som möjligt i de kort du fortfarande håller när någon annan går ut.',
  'zolik.rules.deal': 'Varje spelare får {n} kort.',
  'zolik.rules.meldShapes':
    'En grupp är {set}+ kort av samma valör; en svit är {run}+ kort i följd i samma färg.',
  'zolik.rules.turn.draw': 'Dra ett kort på din tur — från talongen eller från kasthögen.',
  'zolik.rules.pickup.topOnly': 'Bara det översta kortet i kasthögen får tas.',
  'zolik.rules.pickup.anyFromPile':
    'Vilket kort som helst i kasthögen får tas, tillsammans med allt som ligger ovanpå det.',
  'zolik.rules.pickup.locked': 'Från kasthögen får man inte dra förrän rond {n}.',
  'zolik.rules.pickup.open': 'Kasthögen är öppen från första ronden.',
  'zolik.rules.turn.discard': 'Avsluta din tur genom att kasta ett kort.',
  'zolik.rules.jokers.restricted': 'En joker får aldrig kastas, utom som exakt det kort som tömmer din hand.',
  'zolik.rules.lead.rotate': 'Förhanden flyttas ett säte per giv, oavsett vem som vann.',
  'zolik.rules.lead.winner': 'Den som går ut spelar ut i nästa giv.',
  'zolik.rules.meldFloor.on': 'Din första utläggning måste ge minst {n} naturliga poäng innan du är utlagd.',
  'zolik.rules.meldFloor.off': 'Det finns inget lägsta poängvärde för din första utläggning.',
  'zolik.rules.cleanRun.on': 'Minst en av dina sviter måste vara helt jokerfri innan du räknas som utlagd.',
  'zolik.rules.cleanRun.off': 'Dina sviter får använda jokrar fritt — ingen svit behöver vara jokerfri.',
  'zolik.rules.contracts.rotating':
    'Matchen är {n} givar lång, och varje giv kräver sin egen kombination av grupper och sviter.',
  'zolik.rules.contracts.static': 'Varje giv kräver samma kombination: {sets} grupper och {runs} sviter.',
  'zolik.rules.end.afterDeals': 'Matchen slutar efter {n} givar.',
  'zolik.rules.end.atScore': 'Man fortsätter ge tills någon når {n} poäng — då är det över.',

  'prsi.rules.section.goal': 'Mål',
  'prsi.rules.section.setup': 'Uppställning',
  'prsi.rules.section.turn': 'Din tur',
  'prsi.rules.section.special': 'Specialkort',
  'prsi.rules.section.end': 'Så slutar matchen',
  'prsi.rules.goal': 'Var först med att spela ut alla kort på handen.',
  'prsi.rules.deck': 'Spelas med en kortlek på {value} kort (från 7 och uppåt).',
  'prsi.rules.deal': 'Varje spelare börjar med {n} kort.',
  'prsi.rules.turn.match':
    'Spela ett kort som matchar det översta kortets färg eller valör — eller dra, om du inte kan.',
  'prsi.rules.turn.draw': 'Att dra avslutar din tur utan utspel.',
  'prsi.rules.sevens': 'Spela en 7:a så drar nästa spelare två kort, om denne inte svarar med en egen 7:a.',
  'prsi.rules.aces': 'Spela ett ess så hoppas nästa spelares tur över.',
  'prsi.rules.queens': 'Spela en dam och säg vilken färg som gäller vidare.',
  'prsi.rules.end': 'Matchen slutar i samma stund som någons hand är tom.',

  'canasta.rules.section.goal': 'Mål',
  'canasta.rules.section.setup': 'Uppställning',
  'canasta.rules.section.melding': 'Lägga ut',
  'canasta.rules.section.end': 'Så slutar matchen',
  'canasta.rules.goal': 'Spelas i par; den sida som först når {n} poäng vinner matchen.',
  'canasta.rules.deck': 'Spelas med {value} kort — två lekar plus jokrar.',
  'canasta.rules.deal': 'Varje spelare får {n} kort.',
  'canasta.rules.redThrees':
    'En röd trea på handen visas genast och ger bonus — utom om din sida aldrig får ihop en canasta, då räknas den mot dig i stället.',
  'canasta.rules.canasta': 'En canasta är en kombination av {n} eller fler kort av samma valör.',
  'canasta.rules.meldFloorBands':
    'Din första utläggning måste nå ett poängminimum som stiger med din ställning: {negative} under noll, {low} upp till 1500, {mid} upp till 3000, {high} däröver.',
  'canasta.rules.oneCanastaToGoOut': 'En färdig canasta räcker för att din sida ska få gå ut.',
  'canasta.rules.twoCanastasToGoOut': 'Din sida behöver två färdiga canastor innan den får gå ut.',
  'canasta.rules.end': 'Man fortsätter ge tills en sida passerar {n} poäng — då är matchen över.',

  'holdem.rules.section.goal': 'Mål',
  'holdem.rules.section.setup': 'Uppställning',
  'holdem.rules.section.betting': 'Satsningar',
  'holdem.rules.section.end': 'Så slutar matchen',
  'holdem.rules.goal':
    'Vinn marker genom att ha bästa handen vid showdown, eller genom att bli ensam kvar i given.',
  'holdem.rules.stack': 'Varje plats börjar med {n} marker.',
  'holdem.rules.blinds': 'Lilla mörken är {sb} och stora mörken {bb}, lagda innan korten delas ut.',
  'holdem.rules.streets': 'Man satsar i fyra omgångar — före floppen samt efter floppen, turn och river.',
  'holdem.rules.showdown': 'De som är kvar visar sina kort; bästa femkortshanden tar potten.',
  'holdem.rules.noLimit': 'No limit — varje satsning får gå ända upp till hela din stack.',
  'holdem.rules.lastPlayerStanding': 'Man spelar tills en plats håller alla marker.',
  'holdem.rules.mostChipsWins': 'Den som har flest marker när spelet avbryts vinner matchen.',
  'holdem.rules.handLimit': 'Spelet avbryts efter {n} givar.',

  // --- header --------------------------------------------------------------
  'header.deal': 'Giv {n}',
  'header.gameOf': 'Spel {n} av {total}',
  'header.gameOfWithContract': 'Spel {n} av {total}: {contract}',

  // --- meld preview --------------------------------------------------------
  'preview.validSet': 'Giltig grupp',
  'preview.validRun': 'Giltig svit',
  'preview.validMeld': 'Giltig kombination',
  'preview.notYet': 'Ännu ingen kombination',
  'preview.points': '{shape} · {n} poäng',
  'preview.pointsWithLaid': '{shape} · {n} + {laid} redan utlagda = {total} poäng',
  'preview.meetsFloor': '{line} (når {n} ✓)',
  'preview.needsFloor': '{line} (kräver {n} ✗)',
  'preview.becauseOf': '{line} — {reason}',

  // --- discard -------------------------------------------------------------
  // A discard with a meld staged lays that meld first; this is what the
  // player is told when the server refuses it and the whole move is rolled
  // back. {reason} is the engine's own words for the refusal.
  'discard.meldRejected': '{reason} — inget kastades, dina kort ligger kvar redo.',

  // --- what the current selection would send -------------------------------
  // The client's own reason a control is not ready, next to the engine's own
  // `err.*` reasons rather than a second kind of message — a control greyed
  // out for "not your turn" and one greyed out for "you picked two, this
  // takes one" should read as the same kind of thing, not one of them looking
  // broken. See `fits` in `src/lib/drops.ts`.
  'sel.tooMany.1': 'Välj bara ett kort',
  'sel.tooMany.n': 'Välj högst {n} kort',
  'sel.needMore': 'Välj {n} kort',
  'sel.notThese': 'De korten kan inte ligga här',
  'sel.needsCompany': 'Det kortet behöver korten bredvid',

  // --- what a module says happened -----------------------------------------
  // The first module-sent keys in this bundle, and deliberately few. Anything
  // a module sends is still legible without an entry here — `humanise` turns
  // `holdem.seat.stack` into "Stack" — so a key earns a line only when that
  // fallback loses something. These do: what each of them means lives in its
  // *params* (who won, how much, whose cards), and a key on its own has
  // nowhere to put them. Without the entry the player read "Winner", full
  // stop, at the end of a match they had just won.
  'status.winner': 'Vunnen av {winners}',
  'holdem.status.pot': '{winners} vann {amount} med {hand}',
  'holdem.status.potUncontested': '{winners} vann {amount} — alla andra lade sig',
  'holdem.status.shown': '{playerId} visade {value}',
  'holdem.prompt.waitingFor': 'Väntar på {playerId}',

  // How a rummy scoreboard was ordered. Both earn a line for the same reason
  // as the keys above: the number lives in the params, and `humanise` would
  // render "Deals Won" with nothing after it.
  'zolik.standing.dealsWon': 'Vunna givar {n}',
  'zolik.standing.inHand': 'På handen {n}',

  // --- rounds ---------------------------------------------------------------
  // What a round is called, and what one did. Each of these carries its number
  // in the params, so `humanise` alone would render a label with nothing after
  // it.
  // An instruction, not a heading. "Next round" named the thing rather than
  // asking for it, and sat above "Waiting for 1" — which reads as though the
  // player is the one waiting, when the table is waiting for them.
  'round.continue': 'Starta nästa rond',

  // --- the flash that announces a round or a match ending ------------------
  //
  // One line and one number, held for about two seconds. Worded generically on
  // purpose: which *kind* of ending it was is the module's own fact, printed
  // underneath, and a client that phrased "went gin" itself would be a client
  // that knows a game.
  'flash.roundWon': '{winners} tog den',
  'flash.roundWonYou': 'Du tog den',
  'flash.roundDrawn': 'Ingen tog den',
  'flash.matchOver': 'Matchen är slut',
  'flash.matchWon': '{winners} vann',
  'flash.matchWonYou': 'Du vann',
  'flash.matchDrawn': 'Ingen vann',
  'flash.nowOn': 'nu {total}',

  'zolik.round.deal': 'Giv',
  'zolik.round.cleanRun': 'En svit måste vara jokerfri',
  'canasta.round.deal': 'Giv',
  'canasta.round.concealed': 'Gick ut dolt',
  'canasta.round.exhausted': 'Leken tog slut',
  'canasta.round.meldCards': 'Utlagda kort {n}',
  'canasta.round.canastas': 'Canastor {n}',
  'canasta.round.redThrees': 'Röda treor {n}',
  'canasta.round.goingOut': 'Gå ut {n}',
  'canasta.round.inHand': 'Kvar på handen {n}',
  'holdem.round.hand': 'Giv',
  'holdem.round.pot': 'Pott {n}',
  'holdem.round.uncontested': 'Alla andra lade sig',
  'seat.ready': 'Redo',



  // --- refusals the other three games raise ---------------------------------
  // Worded here for the same reason Žolíky's are: reasonText falls back to the
  // code, and OfferBar hands it the code as that fallback, so an unworded
  // refusal reaches a player as PILE_FROZEN in capitals. serverKeys.test.ts
  // now fails the build rather than letting the next one through.
  'err.SET_TOO_LARGE': 'En grupp har redan alla fyra färgerna',
  'err.DISCARD_TAKEN_CARD_FORBIDDEN': 'Du får inte kasta kortet du just tog — spela det eller behåll det',
  'err.CARD_DOES_NOT_FIT': 'Det kortet matchar varken färg eller valör',
  'err.SUIT_REQUIRED': 'Säg vilken färg som gäller vidare',
  'err.MUST_ANSWER_DRAW_OR_TAKE': 'Svara med en sjua, eller ta korten',
  'err.NOTHING_TO_DRAW': 'Det finns inget kvar att dra',
  'err.PILE_EMPTY': 'Högen är tom',
  'err.PILE_BLOCKED': 'Högen är blockerad — en svart trea ligger överst',
  'err.PILE_FROZEN': 'Högen är frusen — du behöver två naturliga kort av det översta kortets valör',
  'err.TOP_CARD_UNUSABLE': 'Du kan inte använda det översta kortet',
  'err.MELD_CLOSED': 'Den kombinationen är fullständig och stängd',
  'err.MELD_TOO_SMALL': 'En kombination behöver fler kort än så',
  'err.MELD_TOO_LARGE': 'Den kombinationen rymmer inga fler kort',
  'err.MELD_MIXED_RANKS': 'Alla kort i en kombination måste ha samma valör',
  'err.NOT_ENOUGH_NATURALS': 'En kombination behöver fler naturliga kort än vilda',
  'err.RANK_ALREADY_MELDED': 'Din sida har redan en kombination av den valören',
  'err.NOT_YOUR_MELD': 'Den kombinationen tillhör motståndarsidan',
  'err.NO_SUCH_MELD': 'Den kombinationen ligger inte på bordet',
  'err.CANNOT_MELD_THREE': 'Treor läggs aldrig ut',
  'err.CANNOT_DISCARD_RED_THREE': 'En röd trea får inte kastas',
  'err.MUST_KEEP_A_CARD': 'Behåll minst ett kort — så kan du inte tömma handen',
  'err.MUST_MELD_FIRST': 'Lägg ut din sidas öppning först',
  'err.INITIAL_MELD_NOT_MET': 'Din första utläggning saknar fortfarande poäng',
  'err.CANNOT_GO_OUT_YET': 'Din sida behöver en färdig canasta innan den kan gå ut',
  'err.NOTHING_TO_CALL': 'Det finns ingen satsning att syna',
  'err.CANNOT_CHECK': 'Du kan inte checka — det finns en satsning att svara på',
  'err.CANNOT_RAISE': 'Här kan du inte höja',
  'err.RAISE_TOO_SMALL': 'En höjning måste vara minst lika stor som den förra',
  'err.NOT_ENOUGH_CHIPS': 'Så många marker har du inte',
  'err.AMOUNT_REQUIRED': 'Säg hur mycket',
  'err.AMOUNT_NOT_A_NUMBER': 'Det beloppet är inget tal',
  'err.SEAT_NOT_IN_HAND': 'Du är inte med i den här given',
  'err.WRONG_RANK': 'Det kortet har fel valör för det här',
  'err.MATCH_FULL': 'Bordet är fullt',
  'err.MATCH_ALREADY_STARTED': 'Matchen har redan börjat',
  'err.TOO_FEW_PLAYERS': 'Ännu inte tillräckligt många spelare',
  'err.WRONG_PLAYER_COUNT': 'Det här spelet går inte att spela med så många spelare',
  'err.NOT_THE_HOST': 'Bara värden kan göra det',
  'err.NO_LONGER_WAITING': 'Bordet väntar inte längre',
  'err.WAITING_ROOM_UNAVAILABLE': 'Väntrummet är inte tillgängligt',
  'err.SERVER_BUSY': 'Servern är full just nu — försök igen om en stund',


  // --- rules the engine has always enforced and never stated ----------------
  // Every one of these was found by the guardrail rather than by review: the
  // validator could refuse a player for it, and no sentence anywhere said so.
  // See server/internal/zolikmod/ruleindex_test.go.
  'zolik.rules.section.layoff': 'Lägga till i kombinationer',
  'zolik.rules.pickup.obligation':
    'Innan du är utlagd måste ett kort som tagits från kasthögen användas i den kombination du lägger ut med den här turen.',
  'zolik.rules.pickup.noReturn':
    'Ett kort du tagit från kasthögen får inte kastas igen samma tur — spela det eller behåll det.',
  'zolik.rules.wilds.setLimit': 'En grupp får inte innehålla fler jokrar än naturliga kort.',
  'zolik.rules.set.maxSize':
    'En grupp får inte innehålla fler än {n} kort — en joker ersätter en färg som saknas, den fyller inte på en fullständig grupp.',
  'zolik.rules.run.maxLength':
    'En svit får inte innehålla fler än {n} kort — esset längst ner, de tolv valörerna ovanför och esset längst upp.',
  'zolik.rules.run.aceBridge':
    'Ett ess ligger ovanför kungen eller under tvåan, aldrig som brygga mellan svitens båda ändar.',
  'zolik.rules.contracts.contribution':
    'Tills du är utlagd måste varje kombination du lägger vara en som givens kontrakt fortfarande kräver.',
  'zolik.rules.layoff.afterDown':
    'Du får inte lägga till i någon annans kombinationer förrän du lagt ut ditt eget kontrakt.',
  'zolik.rules.layoff.runEnds':
    'Ett kort som läggs till i en svit måste fortsätta den i den ena eller andra änden.',
  'zolik.rules.jokers.swap':
    'En joker i en kombination på bordet får köpas tillbaka med exakt det kort den står för.',
  'zolik.rules.jokers.reclaim.on':
    'En joker som köpts tillbaka från bordet måste spelas i en kombination samma tur — den får inte behållas på handen.',
  'zolik.rules.jokers.reclaim.off': 'En joker som köpts tillbaka från bordet får behållas på handen.',
  'zolik.rules.deck.reshuffle':
    'När talongen tar slut blandas kasthögen och blir den nya talongen; är båda tomma tar given slut.',


  // --- what to do instead ---------------------------------------------------
  // The third layer of a refusal, after the reason and the rule. Sent by the
  // module because it is the only side that knows which card is owed and
  // which way out is on offer; see server/internal/zolikmod/remedy.go.
  'zolik.remedy.meldThePickup': 'Lägg till {card} i din utläggning, eller ångra upptagningen.',
  'zolik.remedy.discardSomethingElse': 'Kasta ett annat kort, eller spela {card} den här turen.',
  'zolik.remedy.discardNotAJoker': 'Kasta något annat än en joker.',
  'zolik.remedy.finishOrUndoLayDown': 'Avsluta din utläggning, eller ta tillbaka den.',
  'zolik.remedy.needMorePoints': 'Du behöver {n} poäng till innan du får lägga ut.',
  'zolik.remedy.layACleanRun': 'Lägg ut en svit utan joker i.',
  'zolik.remedy.playReclaimedJoker': 'Spela {card} i en kombination, eller ångra att du tog den.',
  'zolik.remedy.goDownFirst': 'Lägg ut dina egna kombinationer först.',
  'zolik.remedy.drawFirst': 'Dra ett kort först.',
  'zolik.remedy.drawFromStock': 'Dra från talongen — kasthögen öppnar i rond {n}.',
  'zolik.remedy.drawFromStockEmpty': 'Dra från talongen i stället.',


  // --- keys whose meaning lives in their params -----------------------------
  // The shape fallback renders these with the number missing ("Stack", "Pot"),
  // which is why they earn a line where most keys do not.
  'header.contract': 'Kräver {sets} grupper och {runs} sviter',
  'header.contract.cleanRunOnly': 'Kräver en jokerfri svit',
  'header.round': 'Rond {n}',
  'header.deck': 'Talong',
  'header.target': 'Mål',
  'header.suitInPlay': 'Färg i spel',
  'seat.cards': 'Kort',
  'zolik.offer.meld': 'Lägg ut',
  'prompt.pickupMustBeMelded':
    '{value} kom från kasthögen — det måste ingå i de kombinationer du lägger ut med den här turen.',
  'prompt.jokerMustBePlayed':
    '{value} kom från bordet — det måste ingå i en kombination innan du kan avsluta din tur.',
  'prompt.initialMeld': 'Din sidas öppning måste nå {n} poäng.',
  'prompt.canastasNeeded': 'Din sida behöver {n} canastor till innan den kan gå ut.',
  'prompt.mustDrawOrAnswerSeven': 'Svara med en sjua, eller dra {n} kort.',
  'prompt.chooseSuit': 'Välj färgen som gäller vidare',
  'prompt.skipPending': 'Din tur hoppas över',
  'status.lastDeal': 'Lag {team} fick {value}',
  'status.teamScore': 'Lag {team}: {value}',
  'canasta.offer.rank': 'Valör',
  'canasta.seat.teamScore': 'Lagets poäng',
  'canasta.seat.canastas': 'Canastor',
  'holdem.header.pot': 'Pott',
  'holdem.header.street': 'Gata',
  'holdem.header.hand': 'Giv',
  'holdem.header.handLimit': 'Givar totalt',
  'holdem.header.blinds': 'Mörkar',
  'holdem.cost.call': 'för att syna',
  'holdem.cost.pot': 'i potten',
  'holdem.seat.stack': 'Stack',
  'holdem.seat.bet': 'Satsning',
  'holdem.prompt.yourAction': 'Din tur att agera',
  'holdem.prompt.raiseTo': 'Höj till',
  'zone.yourHand': 'Din hand',
  'zone.opponentHand': 'Motståndarens hand',
  'zone.drawPile': 'Talong',
  'zone.discardPile': 'Kasthög',
  'zone.melds': 'Kombinationer',
  'zone.teamMelds': 'Din sidas kombinationer',
  'zone.redThrees': 'Röda treor',
  'zone.board': 'Bord',
  'verb.drawFromDeck': 'Dra',
  'verb.takeFromDiscard': 'Ta från högen',


  // --- the why sheet's own furniture ---------------------------------------
  // The three layers a refusal is explained in. Labels, not sentences: what
  // goes under each is sent by the server or worded above.
  'why.reason': 'Varför inte',
  'why.rule': 'Regeln',
  'why.rules': 'Reglerna',
  'why.remedy': 'Vad du kan göra',
  'why.readTheRules': 'Läs hela reglerna →',
  'why.close': 'Stäng',
  'why.open': 'varför',

  // --- marks on a particular card ------------------------------------------
  // A mark is a refusal that has not happened yet: the rule is enforced at the
  // discard, which is the last possible moment to hear about it, so the card
  // says so while there is still a turn left to act on it.
  'zolik.badge.owedToMeld':
    '{card} kom från kasthögen — det måste ingå i de kombinationer du lägger ut med den här turen.',
  'zolik.badge.jokerOwed':
    '{card} kom från bordet — det måste ingå i en kombination innan du kan avsluta din tur.',

  // --- the legal notices ----------------------------------------------------
  // Only the furniture. The documents themselves are in `src/legal`, which is
  // a bundle of the same kind with a parity test of its own — prose that long
  // in a flat key map buries the keys this one exists for.
  //
  // The notice is five fragments rather than one sentence with two links glued
  // in, because Czech does not put the link where English does: "souhlasíš s
  // Podmínkami" inflects the noun the link is made of. Fragments let each
  // locale place and decline its own.
  'legal.terms': 'Villkor',
  'legal.privacy': 'Integritet',
  'legal.source': 'Källkod',
  'legal.updated': 'Version {version}',
  'legal.draft': 'Utkast — ännu inte i kraft. Operatörens namn, land och kontaktadress återstår att fylla i.',
  'legal.notice.before': 'Genom att spela godkänner du ',
  'legal.notice.terms': 'användarvillkoren',
  'legal.notice.between': '. Vad som sparas om dig står i ',
  'legal.notice.privacy': 'integritetspolicyn',
  'legal.notice.after': '.',
  // --- gin rummy -------------------------------------------------------------
  'err.UPCARD_DECLINED': 'Du har redan tackat nej till det kortet',
  'err.DEADWOOD_TOO_HIGH': 'Ditt deadwood är för högt för att knacka',
  'err.CARD_DOES_NOT_EXTEND_MELD': 'Det kortet förlänger inte den här kombinationen',
  'ginrummy.rules.setup': 'Uppställning',
  'ginrummy.rules.turn': 'Din tur',
  'ginrummy.rules.melds': 'Kombinationer',
  'ginrummy.rules.knocking': 'Knacka',
  'ginrummy.rules.bigGin': 'Big gin',
  'ginrummy.rules.layoff': 'Påläggningen',
  'ginrummy.rules.deadHand': 'Den döda given',
  'ginrummy.rules.scoring': 'Poäng för en giv',
  'ginrummy.rules.match': 'Vinna matchen',
  'ginrummy.rules.lineBonuses': 'Bonusar i uppräkningen',
  'ginrummy.rules.deck': 'Spelas med en kortlek på {value} kort.',
  'ginrummy.rules.deal': 'Varje spelare får {value} kort.',
  'ginrummy.rules.upcard': 'Ytterligare ett kort vänds upp för att starta kasthögen.',
  'ginrummy.rules.drawDiscard':
    'Dra ett kort på din tur — från talongen eller kasthögen — och kasta sedan ett.',
  'ginrummy.rules.setsAndRuns':
    'En kombination är en grupp om tre eller fyra kort av en valör, eller en svit om tre eller fler i en färg.',
  'ginrummy.rules.aceLow': 'Esset är alltid lågt — det finns ingen svit från dam till ess.',
  'ginrummy.rules.knockLimit': 'Du får knacka så snart ditt deadwood är {n} eller lägre.',
  'ginrummy.rules.oklahoma': 'Knackgränsen i den här given sätts av det uppvända kortets värde.',
  'ginrummy.rules.gin': 'Noll deadwood är gin — bästa möjliga knackning.',
  'ginrummy.rules.bigGinBonus':
    'Elva kort som alla ingår i kombinationer, helt utan kast, är big gin och ger ytterligare {n} poäng.',
  'ginrummy.rules.layoffDescription':
    'Efter en knackning som inte är gin får din motståndare lägga sitt eget deadwood på dina kombinationer innan händerna jämförs.',
  'ginrummy.rules.deadHandDescription':
    'Om talongen sjunker till sina sista två kort utan att någon knackat är given död — ingen får poäng, och samma givare ger om.',
  'ginrummy.rules.undercut':
    'Är motståndarens deadwood inte högre än ditt, underskär denne dig: motståndaren får mellanskillnaden plus {n}.',
  'ginrummy.rules.ginBonus': 'Gin ger motståndarens hela hand plus {n}.',
  'ginrummy.rules.target': 'Den som först passerar {n} poäng när en giv är slut vinner matchen.',
  'ginrummy.rules.shutout': 'Matchbonusen fördubblas till {n} om förloraren aldrig fick en enda poäng.',
  'ginrummy.rules.box': 'Varje giv du vann är värd {n} poäng vid matchens slut.',
  'ginrummy.rules.gameBonus': 'Att vinna matchen ger ytterligare {n} poäng.',
  'ginrummy.fact.deadwood': '{value} deadwood',
  'ginrummy.fact.discardCard': 'Kasta {value}',
  'ginrummy.fact.meldCards': 'På {value}',
  'ginrummy.header.hand': 'Giv {n}',
  'ginrummy.round.kind': '{value}',
  'ginrummy.round.hand': 'Giv',
  'ginrummy.seat.score': '{value}',
  'ginrummy.seat.dealer': 'Givare',
  'ginrummy.status.knocked': '{playerId} knackade med {deadwood} i deadwood',
  'ginrummy.status.gin': '{playerId} gick gin',
  'ginrummy.status.lastHand': 'Senaste given: {winner} ({kind}, {delta} poäng)',
  'ginrummy.offer.drawStock': 'Dra från talongen',
  'ginrummy.offer.drawDiscard': 'Dra från kasthögen',
  'ginrummy.offer.takeUpcard': 'Ta det uppvända kortet',
  'ginrummy.offer.passUpcard': 'Passa',
  'ginrummy.offer.discard': 'Kasta',
  'ginrummy.offer.knock': 'Knacka',
  'ginrummy.offer.gin': 'Gin!',
  'ginrummy.offer.bigGin': 'Big gin!',
  'ginrummy.offer.layOff': 'Lägg på',
  'ginrummy.offer.finishLayoff': 'Klar med påläggning',
  'ginrummy.zone.knockerHand': 'Knackarens hand',
  'ginrummy.zone.melds': 'Kombinationer',
  'ginrummy.prompt.upcardDecision': 'Ta det uppvända kortet, eller passa',
  'ginrummy.prompt.yourTurnDraw': 'Dra ett kort',
  'ginrummy.prompt.yourTurnDiscard': 'Kasta — eller knacka, om du kan',
  'ginrummy.prompt.layoff': 'Lägg på deadwood, eller avsluta',

  // --- rummy tiles -------------------------------------------------------------
  'err.TILE_NOT_IN_HAND': 'Den brickan finns inte på din hand',
  'err.TILE_DOES_NOT_FIT': 'Det passar inte där',
  'err.NO_SUCH_SET': 'Den kombinationen ligger inte på bordet',
  'err.INITIAL_MELD_ONLY': 'Före din första utläggning får du bara flytta om dina egna nya kombinationer',
  'err.TABLE_NOT_VALID': 'Bordet är inte giltigt ännu',
  'err.TRAY_NOT_EMPTY': 'Du har fortfarande lösa brickor att placera',
  'err.NOTHING_PLAYED': 'Lägg minst en bricka innan du avslutar din tur',
  'err.INITIAL_MELD_TOO_LOW': 'Din första utläggning måste vara värd minst 30 poäng',
  'err.NOT_A_RUN': 'Bara en svit kan delas',
  'err.BAD_SPLIT_POSITION': 'Där kan den här sviten inte delas',
  'err.NO_JOKER_IN_SET': 'Det finns ingen joker i den kombinationen',
  'err.TILE_JOKER_SWAP_MISMATCH': 'Den brickan är inte vad jokern står för',
  'rummytiles.rules.setup': 'Uppställning',
  'rummytiles.rules.sets': 'Kombinationer',
  'rummytiles.rules.initialMeld': 'Första utläggningen',
  'rummytiles.rules.turn': 'Din tur',
  'rummytiles.rules.jokerTaking': 'Ta en joker',
  'rummytiles.rules.ending': 'Avsluta en rond',
  'rummytiles.rules.poolExhaustion': 'Om påsen tar slut',
  'rummytiles.rules.match': 'Vinna matchen',
  'rummytiles.rules.tiles': 'Spelas med {value} brickor.',
  'rummytiles.rules.dealCount': 'Varje spelare får {value} brickor.',
  'rummytiles.rules.group': 'En grupp är tre eller fyra brickor med samma siffra, var och en i olika färg.',
  'rummytiles.rules.run': 'En svit är tre eller fler siffror i följd i samma färg.',
  'rummytiles.rules.noWrap': '13 fortsätter inte runt till 1.',
  'rummytiles.rules.joker': 'En joker står för vilken bricka som helst.',
  'rummytiles.rules.initialMeldDescription':
    'Tills du lagt ut {n} poäng eller mer under en enda tur, enbart från din egen hand, får du inte röra något som redan ligger på bordet.',
  'rummytiles.rules.turnDescription':
    'Lägg minst en bricka från din hand, flytta om bordet fritt, och avsluta med varje kombination på bordet giltig.',
  'rummytiles.rules.noDiscard':
    'Det finns inget kast — kan du inte fullborda en giltig tur drar du en bricka i stället.',
  'rummytiles.rules.jokerTakingDescription':
    'En joker på bordet får tas genom att du ersätter den med brickan den står för, från din hand — och den måste användas i en kombination innan din tur är slut.',
  'rummytiles.rules.goingOut':
    'Den spelare som först blir av med sina brickor vinner ronden. Alla andra får det negerade värdet av det de har kvar; vinnaren får summan av vad alla andra förlorade.',
  'rummytiles.rules.poolExhaustionLowestWins':
    'Tar påsen slut och ingen kan spela avslutas ronden, och den lägsta handen vinner den.',
  'rummytiles.rules.poolExhaustionNoWinner':
    'Tar påsen slut och ingen kan spela avslutas ronden utan vinnare — varje hand räknas helt enkelt.',
  'rummytiles.rules.target': 'Den som först passerar {n} poäng när en rond är slut vinner matchen.',
  'rummytiles.rules.roundLimit': 'Matchen slutar efter {n} ronder — högsta poäng vinner.',
  'rummytiles.fact.setCards': '{value}',
  'rummytiles.header.pool': 'Påse {n}',
  'rummytiles.header.round': 'Rond {n}',
  'rummytiles.round.kind': '{value}',
  'rummytiles.round.round': 'Rond',
  'rummytiles.seat.score': '{value}',
  'rummytiles.seat.notOpened': 'Inte öppnad',
  'rummytiles.status.lastRound': 'Senaste ronden: {winner} ({kind})',
  'rummytiles.badge.invalid': 'Inte giltig ännu',
  'rummytiles.zone.pool': 'Påse',
  'rummytiles.zone.table': 'Bord',
  'rummytiles.zone.tray': 'Ställ',
  'rummytiles.offer.place': 'Placera',
  'rummytiles.offer.addFromHand': 'Lägg till',
  'rummytiles.offer.addFromTray': 'Lägg till från stället',
  'rummytiles.offer.take': 'Ta',
  'rummytiles.offer.split': 'Dela',
  'rummytiles.offer.swapJoker': 'Byt jokern',
  'rummytiles.offer.resetTurn': 'Återställ turen',
  'rummytiles.offer.commit': 'Klar',
  'rummytiles.offer.draw': 'Dra',
  'rummytiles.param.position': 'Dela vid',

  // --- blackjack -----------------------------------------------------------
  //
  // The house rules are worded twice over — on and off — because a rule that
  // is *not* in force is still something a player has to be told. "No
  // surrender at this table" is information; silence is a guess.
  'err.BET_BELOW_MINIMUM': 'Det är under bordets minimum',
  'err.ALREADY_BET': 'Din insats ligger redan',
  'err.INSURANCE_CLOSED': 'Det finns ingen försäkring att ta just nu',
  'err.CANNOT_DOUBLE': 'Den här given kan inte dubblas',
  'err.CANNOT_SPLIT': 'Den här given kan inte delas',
  'err.CANNOT_SURRENDER': 'Den här given kan inte ges upp',

  'blackjack.rules.section.table': 'Bordet',
  'blackjack.rules.section.play': 'Att spela en giv',
  'blackjack.rules.section.dealer': 'Givaren',
  'blackjack.rules.section.end': 'Så slutar matchen',
  'blackjack.rules.goal':
    'Slå givaren utan att gå över tjugoett. Går du över förlorar du direkt, vad givaren än gör sedan.',
  // Counts are their own phrase rather than a number glued to a noun: one
  // deck and six decks inflect differently, and Czech inflects them again.
  'blackjack.rules.decks': 'Lekar i skon: {n}.',
  'blackjack.rules.stack': 'Varje plats sätter sig med {n} marker.',
  'blackjack.rules.minBet': 'Bordets minimum är {n} marker.',
  'blackjack.rules.faceUp':
    'Spelarnas kort delas ut öppet; givaren håller ett kort dolt tills alla har spelat.',
  'blackjack.rules.hitStand': 'Ta så många kort du vill, eller stanna på det du har.',
  'blackjack.rules.aces': 'Ett ess räknas som elva så länge det får plats, och som ett när det inte gör det.',
  'blackjack.rules.blackjack': 'Ett ess med ett tiokort, på de två första korten, är en blackjack.',
  'blackjack.rules.pays3to2': 'En blackjack betalar 3:2.',
  'blackjack.rules.pays6to5': 'En blackjack betalar 6:5.',
  'blackjack.rules.paysEven': 'En blackjack betalar lika mycket tillbaka.',
  'blackjack.rules.double': 'På dina två första kort får du dubbla insatsen och ta exakt ett kort till.',
  'blackjack.rules.doubleAfterSplit': 'En giv som kommit ur en delning får också dubblas.',
  'blackjack.rules.noDoubleAfterSplit': 'En giv som kommit ur en delning får inte dubblas.',
  'blackjack.rules.split':
    'Två kort av samma värde får delas till egna givar, var och en med egen insats — upp till {n} gånger, till {hands} givar totalt.',
  'blackjack.rules.noSplit': 'Vid det här bordet delas inte par.',
  'blackjack.rules.splitAces':
    'Delade ess får ett kort var och stannar sedan, och tjugoett som blir till så är ingen blackjack.',
  'blackjack.rules.surrender':
    'Du får ge upp din första giv för halva insatsen, när givaren väl kollat efter blackjack.',
  'blackjack.rules.noSurrender': 'Vid det här bordet går det inte att ge upp givar.',
  'blackjack.rules.dealerDraws': 'Givaren drar till sjutton och stannar sedan.',
  'blackjack.rules.hitsSoft17': 'Givaren drar på en sjutton som bildats med ett ess.',
  'blackjack.rules.standsSoft17': 'Givaren stannar på en sjutton som bildats med ett ess.',
  'blackjack.rules.dealerPeeks':
    'Med ett ess eller en tia uppe kollar givaren efter blackjack innan någon spelar.',
  'blackjack.rules.insurance':
    'Mot ett ess hos givaren får du försäkra dig för halva insatsen; det betalar 2:1 om givaren har blackjack.',
  'blackjack.rules.noInsurance': 'Vid det här bordet erbjuds ingen försäkring.',
  'blackjack.rules.rounds': 'Bordet spelar {n} ronder.',
  'blackjack.rules.mostChipsWins': 'Den som har flest marker till slut vinner matchen.',
  'blackjack.rules.bustedOut':
    'En plats som inte längre klarar minimum på {n} sitter över resten av matchen.',

  'blackjack.zone.dealer': 'Givare',
  'blackjack.zone.box': 'Giv',
  'blackjack.zone.yourBox': 'Din giv',
  'blackjack.zone.shoe': 'Sko',

  'blackjack.header.round': 'Rond {n} av {of}',
  'blackjack.header.minBet': 'Minimum',
  'blackjack.header.decks': 'Lekar',
  'blackjack.header.dealerTotal': 'Givaren visar {n}',
  'blackjack.header.dealerSoftTotal': 'Givaren visar mjuk {n}',

  'blackjack.seat.stack': 'Marker',
  'blackjack.seat.bet': 'Insats',
  'blackjack.seat.insurance': 'Försäkring',
  'blackjack.seat.total': 'Totalt',
  'blackjack.seat.softTotal': 'Mjuk summa',
  'blackjack.seat.out': 'Slut på marker',

  'blackjack.prompt.placeBet': 'Lägg din insats',
  'blackjack.prompt.insurance': 'Försäkring?',
  'blackjack.prompt.yourMove': 'Din tur',
  'blackjack.prompt.waitingFor': 'Väntar på {playerId}',
  'blackjack.prompt.betAmount': 'Insats',

  'blackjack.offer.bet': 'Satsa',
  'blackjack.offer.hit': 'Ta kort',
  'blackjack.offer.stand': 'Stanna',
  'blackjack.offer.double': 'Dubbla',
  'blackjack.offer.split': 'Dela',
  'blackjack.offer.surrender': 'Ge upp',
  'blackjack.offer.insure': 'Ta försäkring',
  'blackjack.offer.declineInsurance': 'Ingen försäkring',

  'blackjack.fact.tableMinimum': 'minimum',
  'blackjack.fact.insuranceCost': 'att försäkra',
  'blackjack.fact.extraStake': 'att satsa',
  'blackjack.fact.surrenderReturn': 'tillbaka',

  'blackjack.status.dealerBlackjack': 'Givaren hade blackjack',
  'blackjack.status.dealerBust': 'Givaren blev tjock på {n}',
  'blackjack.status.dealerStands': 'Givaren stannar på {n}',

  'blackjack.round.name': 'Rond',
  'blackjack.round.dealerTotal': 'Givare {n}',
  'blackjack.round.dealerBust': 'Givaren tjock ({n})',
  'blackjack.round.dealerBlackjack': 'Givarens blackjack',
  'blackjack.round.outcome.blackjack': 'Blackjack',
  'blackjack.round.outcome.win': 'Vunnen',
  'blackjack.round.outcome.push': 'Lika',
  'blackjack.round.outcome.lose': 'Förlorad',
  'blackjack.round.outcome.bust': 'Tjock',
  'blackjack.round.outcome.surrender': 'Uppgiven',

  'blackjack.badge.inPlay': 'I spel',
  'blackjack.badge.doubled': 'Dubblad',
  'blackjack.badge.split': 'Delad',
  'blackjack.badge.blackjack': 'Blackjack',
  'blackjack.badge.bust': 'Tjock',
  'blackjack.badge.won': 'Vunnen',
  'blackjack.badge.push': 'Lika',
  'blackjack.badge.lost': 'Förlorad',
  'blackjack.badge.surrendered': 'Uppgiven',

  'blackjack.unit.chips': 'marker',

  // --- the settings screen --------------------------------------------------
  //
  // Hardcoded English until now, which was survivable while the only other
  // language was unreachable and faintly absurd once the language picker
  // itself lived on this screen: a player who cannot read "Settings" is
  // exactly the one who came here to change it.
  'settings.title': 'Inställningar',
  'settings.subtitle': 'Hur du ser ut, och hur bordet gör det',
  'settings.face.heading': 'Ditt ansikte vid bordet',
  'settings.face.account': 'Sparas med ditt konto, så det följer med till en annan enhet.',
  'settings.face.device': 'Sparas på den här enheten. Logga in för att ta det med dig.',
  'settings.skin.heading': 'Bordets utseende',
  'settings.language.heading': 'Språk',
  'settings.language.status': 'Sparas på den här enheten.',
  'settings.language.auto': 'Automatiskt',
  'settings.language.auto.now': 'Följer din enhet — just nu {language}',
  'settings.legal.heading': 'Det finstilta',
  'settings.legal.status': 'Vad du godkände genom att spela, och vad som sparas om dig.',
  'settings.signIn': 'Logga in',
  'settings.back': 'Tillbaka',

  // A translated interface over an English notice is a worse lie than an
  // English interface, so the document says so itself rather than letting the
  // surrounding screen imply the text was written for this reader.
  'legal.untranslated':
    'Det här meddelandet är ännu inte översatt till ditt språk. Den engelska texten nedan är den version som gäller.',

  // --- the navigation bar's screen titles ------------------------------------
  //
  // These sit in `app/_layout.tsx`, above every screen, and were the last
  // English left after the settings screen was translated — a German player
  // reading "Einstellungen" under a bar that said "Settings". A translated app
  // with an untranslated chrome bar looks like a bug, not like a limit.
  'nav.home': 'Zolik',
  'nav.emailSignIn': 'Inloggning med e-post',
  'nav.signingIn': 'Loggar in',
  'nav.usernameSignIn': 'Inloggning med användarnamn',
  'nav.legacyAccount': 'Gammalt konto',
  'nav.guest': 'Gäst',
  'nav.account': 'Konto',
  'nav.games': 'Spel',
  'nav.table': 'Ditt bord',
  'nav.join': 'Gå med vid ett bord',
  'nav.rules': 'Regler',
  'nav.match': 'Match',
  'nav.scoreTable': 'Poängtabell',
  'nav.stats': 'Statistik',
};
