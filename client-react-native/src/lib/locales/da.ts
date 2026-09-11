/**
 * Danish. Rummy vocabulary: gruppe for a set, række for a run, talon for the stock, kastebunke for the discard pile. Giv is a deal, runde a round.
 */

export const da: Record<string, string> = {
  // --- engine error codes (rules.RulesErrorCode) ---------------------------
  'err.NOT_YOUR_TURN': 'Det er ikke din tur',
  'err.WRONG_PHASE': 'Kan ikke lige nu',
  'err.MUST_DRAW_FIRST': 'Træk et kort, før du lægger ud',
  'err.GAME_SUSPENDED': 'Spillet er sat på pause',
  'err.GAME_NOT_ACTIVE': 'Spillet er ikke i gang',
  // --- runtime refusals, which are not the rules saying no -----------------
  //
  // Both were reachable long before they were worded, and a player who met
  // one was shown the code in capitals or, worse, nothing at all: an action
  // sent while the socket was down simply vanished, which reads as a legal
  // move being silently refused.
  'err.MATCH_NOT_ACTIVE': 'Bordet er på pause — vi venter på, at en spiller kommer tilbage',
  'err.NOT_CONNECTED': 'Ingen forbindelse til bordet — der forbindes igen, prøv bagefter',
  // Why the "start the next round" control is greyed out. Without wording the
  // shell printed the code itself, so a player agreeing to go on was answered
  // with ALREADY_READY in capitals.
  'err.ALREADY_READY': 'Du er klar',
  'err.NOT_BETWEEN_ROUNDS': 'Runden spilles stadig',
  'err.NOT_AT_THIS_TABLE': 'Du sidder ikke ved dette bord',
  // Bringing a table back that the sweeper set aside. Refusals a player
  // can actually provoke: a link to a table that is gone, one that was
  // already picked up in another tab, and a table with other people at it.
  'err.MATCH_MOVED_ON': 'Bordet er gået videre — genindlæs siden',
  'err.MATCH_NOT_ABANDONED': 'Dette bord venter ikke på at blive genoptaget',
  'err.MATCH_NOT_FOUND': 'Dette bord findes ikke længere',
  'err.TABLE_HAS_OTHER_PLAYERS': 'Kun et bord, hvor alle andre er bots, kan genoptages',
  'err.DISCARD_LOCKED': 'Kastebunken er låst indtil videre',
  'err.DISCARD_PILE_EMPTY': 'Kastebunken er tom',
  'err.NO_CARDS_LEFT': 'Der er ikke flere kort at trække',
  'err.ROUND_REQ_NOT_MET': 'Læg din egen åbning ud først',
  'err.NEED_CLEAN_RUN': 'Du skal have en jokerfri række på bordet for at tælle som lagt ud',
  'err.INCOMPLETE_INITIAL_MELD': 'Gør din udlægning færdig, eller fortryd den, før du smider ud',
  'err.DISCARD_CARD_NOT_MELDED': 'Kortet, du samlede op, skal indgå i din kombination',
  'err.JOKER_DISCARD_FORBIDDEN': 'En joker må ikke smides ud',
  'err.NOTHING_TO_UNDO': 'Der er intet at fortryde',
  'err.NO_JOKER_IN_MELD': 'Ingen joker i denne kombination',
  'err.JOKER_SWAP_MISMATCH': 'Det kort træder ikke i jokerens sted',
  'err.RECLAIMED_JOKER_NOT_MELDED': 'Jokeren, du tog fra bordet, skal spilles i en kombination denne tur',
  'err.RUN_TOO_LONG': 'Den række har allerede sin fulde længde',
  'err.WRONG_RUN_END': 'Det kort forlænger rækkens anden ende',
  'err.INVALID_MELD': 'Intet kort på din hånd passer her',
  'err.CARD_NOT_IN_HAND': 'Det kort er ikke på din hånd',
  'err.MELD_BELOW_MINIMUM': 'Dine kombinationer mangler stadig point, før du kan lægge ud',
  'err.MELD_NO_CONTRIBUTION': 'Den kombination bringer ikke dit krav videre',
  'err.TOO_MANY_WILDS': 'For mange jokere i den kombination',
  'err.ADJACENT_WILDS': 'To jokere må ikke ligge ved siden af hinanden',
  'err.ACE_BRIDGE': 'Et es kan ikke binde konge og toer sammen',

  // --- contract phrasing ---------------------------------------------------
  // Built from the structured contract the server sends (sets/runs counts),
  // never from a deal-number lookup table.
  // Whole phrases per count rather than a number plus a pluralised noun:
  // Czech inflects the noun by count (skupina / skupiny / skupin) in a way no
  // "add an s" helper survives, and the counts that actually occur are 1-3.
  // A count outside that falls back to contract.sets.n.
  'contract.sets.1': 'Én gruppe',
  'contract.sets.2': 'To grupper',
  'contract.sets.3': 'Tre grupper',
  'contract.sets.n': '{n} grupper',
  'contract.runs.1': 'Én række',
  'contract.runs.2': 'To rækker',
  'contract.runs.3': 'Tre rækker',
  'contract.runs.n': '{n} rækker',
  'contract.any': 'Enhver gyldig kombination',
  'contract.cleanRunOnly': 'Enhver blanding af grupper og rækker — mindst én række skal være jokerfri',
  'contract.cleanRunSuffix': '{base} — én række skal være jokerfri',

  // --- written rules ---------------------------------------------------------
  // Full-sentence keys a "see the rules" screen renders, one per module,
  // resolved server-side against the table's actual variation and options —
  // see internal/module/rules.go and each game's rules.go. Shared section
  // titles first, then one block per module in the order the four games were
  // added.
  'zolik.rules.section.goal': 'Mål',
  'zolik.rules.section.setup': 'Opstilling',
  'zolik.rules.section.turn': 'Din tur',
  'zolik.rules.section.melding': 'At lægge ud',
  'zolik.rules.section.end': 'Sådan slutter matchen',
  'zolik.rules.goal':
    'Vær den første til at tømme hånden ved at lægge gyldige grupper og rækker ud, og saml så få strafpoint som muligt i de kort, du stadig har, når en anden går ud.',
  'zolik.rules.deal': 'Hver spiller får {n} kort.',
  'zolik.rules.meldShapes':
    'En gruppe er {set}+ kort af samme værdi; en række er {run}+ kort i træk i samme farve.',
  'zolik.rules.turn.draw': 'Træk ét kort i din tur — fra talonen eller fra kastebunken.',
  'zolik.rules.pickup.topOnly': 'Kun det øverste kort i kastebunken må tages.',
  'zolik.rules.pickup.anyFromPile':
    'Ethvert kort i kastebunken må tages sammen med alt, der ligger oven på det.',
  'zolik.rules.pickup.locked': 'Der må ikke trækkes fra kastebunken før runde {n}.',
  'zolik.rules.pickup.open': 'Kastebunken er åben fra første runde.',
  'zolik.rules.turn.discard': 'Afslut din tur ved at smide ét kort ud.',
  'zolik.rules.jokers.restricted':
    'En joker må aldrig smides ud, undtagen som præcis det kort, der tømmer din hånd.',
  'zolik.rules.lead.rotate': 'Udspillet flytter én plads per giv, uanset hvem der vandt.',
  'zolik.rules.lead.winner': 'Den, der går ud, spiller ud i næste giv.',
  'zolik.rules.meldFloor.on': 'Din første udlægning skal give mindst {n} naturlige point, før du er lagt ud.',
  'zolik.rules.meldFloor.off': 'Der er ingen mindsteværdi i point på din første udlægning.',
  'zolik.rules.cleanRun.on': 'Mindst én af dine rækker skal være helt jokerfri, før du tæller som lagt ud.',
  'zolik.rules.cleanRun.off': 'Dine rækker må bruge jokere frit — ingen række behøver at være jokerfri.',
  'zolik.rules.contracts.rotating':
    'Matchen varer {n} giv, og hver giv kræver sin egen kombination af grupper og rækker.',
  'zolik.rules.contracts.static': 'Hver giv kræver den samme kombination: {sets} grupper og {runs} rækker.',
  'zolik.rules.end.afterDeals': 'Matchen slutter efter {n} giv.',
  'zolik.rules.end.atScore': 'Der gives videre, indtil nogen når {n} point — så er det slut.',

  'prsi.rules.section.goal': 'Mål',
  'prsi.rules.section.setup': 'Opstilling',
  'prsi.rules.section.turn': 'Din tur',
  'prsi.rules.section.special': 'Specialkort',
  'prsi.rules.section.end': 'Sådan slutter matchen',
  'prsi.rules.goal': 'Vær den første til at spille alle kort på hånden ud.',
  'prsi.rules.deck': 'Spilles med et spil på {value} kort (fra 7 og op).',
  'prsi.rules.deal': 'Hver spiller starter med {n} kort.',
  'prsi.rules.turn.match':
    'Spil et kort, der passer med det øverste korts farve eller værdi — eller træk, hvis du ikke kan.',
  'prsi.rules.turn.draw': 'At trække afslutter din tur uden udspil.',
  'prsi.rules.sevens':
    "Spil en 7'er, og den næste spiller trækker to kort, medmindre vedkommende svarer med sin egen 7'er.",
  'prsi.rules.aces': 'Spil et es, og den næste spillers tur springes over.',
  'prsi.rules.queens': 'Spil en dame og nævn den farve, der fortsætter.',
  'prsi.rules.end': 'Matchen slutter i det øjeblik, en hånd er tom.',

  'canasta.rules.section.goal': 'Mål',
  'canasta.rules.section.setup': 'Opstilling',
  'canasta.rules.section.melding': 'At lægge ud',
  'canasta.rules.section.end': 'Sådan slutter matchen',
  'canasta.rules.goal': 'Der spilles i makkerpar; den første side, der når {n} point, vinder matchen.',
  'canasta.rules.deck': 'Spilles med {value} kort — {decks} spil plus jokere.',
  'canasta.rules.deal': 'Hver spiller får {n} kort.',
  'canasta.rules.drawCount': 'Du trækker {n} kort i begyndelsen af din tur.',
  'canasta.rules.redThrees':
    'En rød treer på hånden vises straks og tæller som bonus — medmindre din side aldrig får en canasta færdig, og så tæller den imod dig.',
  'canasta.rules.canasta': 'En canasta er en kombination af {n} eller flere kort af samme værdi.',
  'canasta.rules.sequences': 'En kombination kan også være en sekvens: tre eller flere kort i samme farve i træk, aldrig med et vildt kort iblandt.',
  'canasta.rules.samba': 'En sekvens på syv kort er en samba og giver {n} point.',
  'canasta.rules.blackThreesGoOut':
    'En sort treer blokerer bunken og er {n} point værd. Tre eller fire af dem må lægges ud direkte fra hånden, aldrig med en joker iblandt, og kun som det træk, hvor din side går ud.',
  'canasta.rules.blackThreesNeverMeld':
    'En sort treer lægges aldrig ud. Kastes den, blokerer den bunken, og bliver den på hånden, når spillet slutter, koster den {n} point.',
  'canasta.rules.pileAlwaysFrozen': 'Kastebunken er frosset hele givet: du kan kun tage den ved at lægge to naturlige kort fra hånden til det øverste kort.',
  'canasta.rules.meldFloorBands':
    'Din første udlægning skal nå et pointminimum, der stiger med din stilling: {negative} under nul, {low} op til 1500, {mid} op til 3000, {high} derover.',
  'canasta.rules.meldFloorBandsFive': 'Din første kombination skal nå et pointminimum, der stiger med din score: {negative} under nul, {low} op til 1500, {mid} op til 3000, {high} op til 7000 og {top} derover.',
  'canasta.rules.oneCanastaToGoOut': 'Én færdig canasta er nok til, at din side kan gå ud.',
  'canasta.rules.twoCanastasToGoOut': 'Din side skal have to færdige canastaer, før den må gå ud.',
  'canasta.rules.end': 'Der gives videre, indtil en side passerer {n} point — så er matchen slut.',

  'holdem.rules.section.goal': 'Mål',
  'holdem.rules.section.setup': 'Opstilling',
  'holdem.rules.section.betting': 'Indsatser',
  'holdem.rules.section.end': 'Sådan slutter matchen',
  'holdem.rules.goal':
    'Vind jetoner ved at have den bedste hånd ved showdown, eller ved at være den eneste tilbage i givet.',
  'holdem.rules.stack': 'Hver plads starter med {n} jetoner.',
  'holdem.rules.blinds': 'Small blind er {sb} og big blind {bb}, lagt inden kortene deles ud.',
  'holdem.rules.streets': 'Der satses i fire runder — før floppen samt efter floppen, turn og river.',
  'holdem.rules.showdown':
    'De, der stadig er med, viser deres kort; den bedste hånd på fem kort tager puljen.',
  'holdem.rules.noLimit': 'No limit — enhver indsats må være på op til hele din stak.',
  'holdem.rules.lastPlayerStanding': 'Der spilles, indtil én plads har alle jetoner.',
  'holdem.rules.mostChipsWins': 'Den, der har flest jetoner, når spillet stopper, vinder matchen.',
  'holdem.rules.handLimit': 'Spillet stopper efter {n} hænder.',

  // --- header --------------------------------------------------------------
  'header.deal': 'Giv {n}',
  'header.gameOf': 'Spil {n} af {total}',
  'header.gameOfWithContract': 'Spil {n} af {total}: {contract}',

  // --- meld preview --------------------------------------------------------
  'preview.validSet': 'Gyldig gruppe',
  'preview.validRun': 'Gyldig række',
  'preview.validMeld': 'Gyldig kombination',
  'preview.notYet': 'Endnu ingen kombination',
  'preview.points': '{shape} · {n} point',
  'preview.pointsWithLaid': '{shape} · {n} + {laid} allerede lagt = {total} point',
  'preview.meetsFloor': '{line} (når {n} ✓)',
  'preview.needsFloor': '{line} (kræver {n} ✗)',
  'preview.becauseOf': '{line} — {reason}',

  // --- discard -------------------------------------------------------------
  // A discard with a meld staged lays that meld first; this is what the
  // player is told when the server refuses it and the whole move is rolled
  // back. {reason} is the engine's own words for the refusal.
  'discard.meldRejected': '{reason} — der blev intet smidt ud, dine kort ligger stadig klar.',

  // --- what the current selection would send -------------------------------
  // The client's own reason a control is not ready, next to the engine's own
  // `err.*` reasons rather than a second kind of message — a control greyed
  // out for "not your turn" and one greyed out for "you picked two, this
  // takes one" should read as the same kind of thing, not one of them looking
  // broken. See `fits` in `src/lib/drops.ts`.
  'sel.tooMany.1': 'Vælg kun ét kort',
  'sel.tooMany.n': 'Vælg højst {n} kort',
  'sel.needMore': 'Vælg {n} kort',
  'sel.notThese': 'De kort kan ikke ligge her',
  'sel.needsCompany': 'Det kort har brug for dem ved siden af',

  // --- what a module says happened -----------------------------------------
  // The first module-sent keys in this bundle, and deliberately few. Anything
  // a module sends is still legible without an entry here — `humanise` turns
  // `holdem.seat.stack` into "Stack" — so a key earns a line only when that
  // fallback loses something. These do: what each of them means lives in its
  // *params* (who won, how much, whose cards), and a key on its own has
  // nowhere to put them. Without the entry the player read "Winner", full
  // stop, at the end of a match they had just won.
  'status.winner': 'Vundet af {winners}',
  'holdem.status.pot': '{winners} vandt {amount} med {hand}',
  'holdem.status.potUncontested': '{winners} vandt {amount} — alle andre kastede sig',
  'holdem.status.shown': '{playerId} viste {value}',
  'holdem.prompt.waitingFor': 'Venter på {playerId}',

  // How a rummy scoreboard was ordered. Both earn a line for the same reason
  // as the keys above: the number lives in the params, and `humanise` would
  // render "Deals Won" with nothing after it.
  'zolik.standing.dealsWon': 'Vundne giv {n}',
  'zolik.standing.inHand': 'På hånden {n}',

  // --- rounds ---------------------------------------------------------------
  // What a round is called, and what one did. Each of these carries its number
  // in the params, so `humanise` alone would render a label with nothing after
  // it.
  // An instruction, not a heading. "Next round" named the thing rather than
  // asking for it, and sat above "Waiting for 1" — which reads as though the
  // player is the one waiting, when the table is waiting for them.
  'round.continue': 'Start næste runde',

  // --- the flash that announces a round or a match ending ------------------
  //
  // One line and one number, held for about two seconds. Worded generically on
  // purpose: which *kind* of ending it was is the module's own fact, printed
  // underneath, and a client that phrased "went gin" itself would be a client
  // that knows a game.
  'flash.roundWon': '{winners} tog den',
  'flash.roundWonYou': 'Du tog den',
  'flash.roundDrawn': 'Ingen tog den',
  'flash.matchOver': 'Matchen er slut',
  'flash.matchWon': '{winners} vandt',
  'flash.matchWonYou': 'Du vandt',
  'flash.matchDrawn': 'Ingen vandt',
  'flash.nowOn': 'nu {total}',

  'zolik.round.deal': 'Giv',
  'zolik.round.cleanRun': 'Én række skal være jokerfri',
  'canasta.round.deal': 'Giv',
  'canasta.round.concealed': 'Gik ud skjult',
  'canasta.round.exhausted': 'Kortene slap op',
  'canasta.round.meldCards': 'Lagte kort {n}',
  'canasta.round.canastas': 'Canastaer {n}',
  'canasta.round.redThrees': 'Røde treere {n}',
  'canasta.round.goingOut': 'At gå ud {n}',
  'canasta.round.inHand': 'Fanget på hånden {n}',
  'holdem.round.hand': 'Hånd',
  'holdem.round.pot': 'Pulje {n}',
  'holdem.round.uncontested': 'Alle andre kastede sig',
  'seat.ready': 'Klar',
  'zolik.seat.contractMet': 'Kontrakt opfyldt',
  'results.you': '(dig)',



  // --- refusals the other three games raise ---------------------------------
  // Worded here for the same reason Žolíky's are: reasonText falls back to the
  // code, and OfferBar hands it the code as that fallback, so an unworded
  // refusal reaches a player as PILE_FROZEN in capitals. serverKeys.test.ts
  // now fails the build rather than letting the next one through.
  'err.SET_TOO_LARGE': 'En gruppe har allerede alle fire farver',
  'err.DISCARD_TAKEN_CARD_FORBIDDEN': 'Du må ikke smide det kort ud, du lige tog — spil det eller behold det',
  'err.CARD_DOES_NOT_FIT': 'Det kort passer hverken i farve eller værdi',
  'err.SUIT_REQUIRED': 'Nævn den farve, der fortsætter',
  'err.MUST_ANSWER_DRAW_OR_TAKE': 'Svar med en syver, eller tag kortene',
  'err.NOTHING_TO_DRAW': 'Der er ikke mere at trække',
  'err.PILE_EMPTY': 'Bunken er tom',
  'err.PILE_BLOCKED': 'Bunken er blokeret — der ligger en sort treer øverst',
  'err.PILE_FROZEN': 'Bunken er frosset — du skal bruge to naturlige kort af det øverste korts værdi',
  'err.TOP_CARD_UNUSABLE': 'Du kan ikke bruge det øverste kort',
  'err.MELD_CLOSED': 'Den kombination er komplet og lukket',
  'err.MELD_TOO_SMALL': 'En kombination kræver flere kort end det',
  'err.MELD_TOO_LARGE': 'Den kombination kan ikke rumme flere kort',
  'err.MELD_MIXED_RANKS': 'Alle kort i en kombination skal have samme værdi',
  'err.SEQUENCE_NO_WILDS': 'En sekvens må ikke indeholde vilde kort',
  'err.SEQUENCE_NEEDS_ONE_SUIT': 'Alle kort i en sekvens skal være i samme farve',
  'err.RUN_NOT_CONSECUTIVE': 'En sekvens skal løbe i rækkefølge uden huller',
  'err.NOT_ENOUGH_NATURALS': 'En kombination kræver flere naturlige kort end vilde',
  'err.RANK_ALREADY_MELDED': 'Din side har allerede en kombination af den værdi',
  'err.NOT_YOUR_MELD': 'Den kombination tilhører modstandersiden',
  'err.NO_SUCH_MELD': 'Den kombination ligger ikke på bordet',
  'err.CANNOT_MELD_THREE': 'Treere lægges aldrig ud',
  'err.BLACK_THREE_GO_OUT_ONLY': 'Sorte treere lægges kun ud som det træk, der tømmer din hånd',
  'err.CANNOT_DISCARD_RED_THREE': 'En rød treer må ikke smides ud',
  'err.MUST_KEEP_A_CARD': 'Behold mindst ét kort — sådan kan du ikke tømme hånden',
  'err.MUST_MELD_FIRST': 'Læg din sides åbning ud først',
  'err.INITIAL_MELD_NOT_MET': 'Din første udlægning mangler stadig point',
  'err.CANNOT_GO_OUT_YET': 'Din side skal have en færdig canasta, før den kan gå ud',
  'err.NOTHING_TO_CALL': 'Der er ingen indsats at syne',
  'err.CANNOT_CHECK': 'Du kan ikke tjekke — der er en indsats at svare på',
  'err.CANNOT_RAISE': 'Her kan du ikke hæve',
  'err.RAISE_TOO_SMALL': 'En hævning skal være mindst lige så stor som den forrige',
  'err.NOT_ENOUGH_CHIPS': 'Så mange jetoner har du ikke',
  'err.AMOUNT_REQUIRED': 'Sig hvor meget',
  'err.AMOUNT_NOT_A_NUMBER': 'Det beløb er ikke et tal',
  'err.SEAT_NOT_IN_HAND': 'Du er ikke med i denne hånd',
  'err.WRONG_RANK': 'Det kort har den forkerte værdi til det her',
  'err.MATCH_FULL': 'Bordet er fuldt',
  'err.MATCH_ALREADY_STARTED': 'Matchen er allerede begyndt',
  'err.TOO_FEW_PLAYERS': 'Der er endnu ikke spillere nok',
  'err.WRONG_PLAYER_COUNT': 'Dette spil kan ikke spilles med så mange spillere',
  'err.NOT_THE_HOST': 'Det kan kun værten',
  'err.BAD_SEATING': 'Den pladsorden passer ikke til dem, der sidder ved bordet',
  'err.NO_LONGER_WAITING': 'Bordet venter ikke længere',
  'err.WAITING_ROOM_UNAVAILABLE': 'Venteværelset er ikke tilgængeligt',
  'err.SERVER_BUSY': 'Serveren er fuld lige nu — prøv igen om et øjeblik',


  // --- rules the engine has always enforced and never stated ----------------
  // Every one of these was found by the guardrail rather than by review: the
  // validator could refuse a player for it, and no sentence anywhere said so.
  // See server/internal/zolikmod/ruleindex_test.go.
  'zolik.rules.section.layoff': 'At lægge til kombinationer',
  'zolik.rules.pickup.obligation':
    'Før du er lagt ud, skal et kort taget fra kastebunken bruges i den kombination, du lægger ud med i denne tur.',
  'zolik.rules.pickup.noReturn':
    'Et kort, du tog fra kastebunken, må ikke smides ud igen i samme tur — spil det eller behold det.',
  'zolik.rules.wilds.setLimit': 'En gruppe må ikke indeholde flere jokere end naturlige kort.',
  'zolik.rules.set.maxSize':
    'En gruppe må ikke indeholde mere end {n} kort — en joker erstatter en manglende farve, den fylder ikke op i en komplet gruppe.',
  'zolik.rules.run.maxLength':
    'En række må ikke indeholde mere end {n} kort — esset nederst, de tolv værdier over det og esset øverst.',
  'zolik.rules.run.aceBridge':
    'Et es ligger over kongen eller under toeren, aldrig som bro mellem rækkens to ender.',
  'zolik.rules.contracts.contribution':
    'Indtil du er lagt ud, skal hver kombination, du lægger, være en, som givens kontrakt stadig kræver.',
  'zolik.rules.layoff.afterDown':
    'Du må ikke lægge til andres kombinationer, før du har lagt dit eget kontrakt ud.',
  'zolik.rules.layoff.runEnds':
    'Et kort, der lægges til en række, skal fortsætte den i den ene eller den anden ende.',
  'zolik.rules.jokers.swap':
    'En joker i en kombination på bordet må købes tilbage med præcis det kort, den står for.',
  'zolik.rules.jokers.reclaim.on':
    'En joker købt tilbage fra bordet skal spilles i en kombination i samme tur — den må ikke blive på hånden.',
  'zolik.rules.jokers.reclaim.off': 'En joker købt tilbage fra bordet må blive på hånden.',
  'zolik.rules.deck.reshuffle':
    'Når talonen slipper op, blandes kastebunken og bliver den nye talon; er begge tomme, slutter given.',


  // --- what to do instead ---------------------------------------------------
  // The third layer of a refusal, after the reason and the rule. Sent by the
  // module because it is the only side that knows which card is owed and
  // which way out is on offer; see server/internal/zolikmod/remedy.go.
  'zolik.remedy.meldThePickup': 'Læg {card} til din udlægning, eller fortryd opsamlingen.',
  'zolik.remedy.discardSomethingElse': 'Smid et andet kort ud, eller spil {card} i denne tur.',
  'zolik.remedy.discardNotAJoker': 'Smid noget andet end en joker ud.',
  'zolik.remedy.finishOrUndoLayDown': 'Gør din udlægning færdig, eller tag den tilbage.',
  'zolik.remedy.needMorePoints': 'Du mangler {n} point mere, før du kan lægge ud.',
  'zolik.remedy.layACleanRun': 'Læg en række uden joker i.',
  'zolik.remedy.playReclaimedJoker': 'Spil {card} i en kombination, eller fortryd at du tog den.',
  'zolik.remedy.goDownFirst': 'Læg dine egne kombinationer ud først.',
  'zolik.remedy.drawFirst': 'Træk et kort først.',
  'zolik.remedy.drawFromStock': 'Træk fra talonen — kastebunken åbner i runde {n}.',
  'zolik.remedy.drawFromStockEmpty': 'Træk fra talonen i stedet.',


  // --- keys whose meaning lives in their params -----------------------------
  // The shape fallback renders these with the number missing ("Stack", "Pot"),
  // which is why they earn a line where most keys do not.
  'header.contract': 'Kræver {sets} grupper og {runs} rækker',
  'header.contract.cleanRunOnly': 'Kræver en jokerfri række',
  'header.round': 'Runde {n}',
  'header.deck': 'Talon',
  'header.target': 'Mål',
  'header.suitInPlay': 'Farve i spil',
  'seat.cards': 'Kort',
  'zolik.offer.meld': 'Læg ud',
  'prompt.pickupMustBeMelded':
    '{value} kom fra kastebunken — det skal indgå i de kombinationer, du lægger ud med i denne tur.',
  'prompt.jokerMustBePlayed':
    '{value} kom fra bordet — det skal indgå i en kombination, før du kan afslutte din tur.',
  'prompt.initialMeld': 'Din sides åbning skal nå {n} point.',
  'prompt.canastasNeeded': 'Din side mangler {n} canastaer mere, før den kan gå ud.',
  'prompt.mustDrawOrAnswerSeven': 'Svar med en syver, eller træk {n} kort.',
  'prompt.chooseSuit': 'Vælg den farve, der fortsætter',
  'prompt.skipPending': 'Din tur springes over',
  'status.lastDeal': 'Hold {team} fik {value}',
  'status.teamScore': 'Hold {team}: {value}',
  'canasta.offer.rank': 'Værdi',
  'canasta.offer.sequence': 'Sekvens',
  'badge.naturalCanasta': 'Ren canasta',
  'badge.mixedCanasta': 'Uren canasta',
  'badge.samba': 'Samba',
  'badge.cleanRun': 'Ren sekvens',
  'canasta.seat.teamScore': 'Holdets point',
  'canasta.seat.canastas': 'Canastaer',
  'holdem.header.pot': 'Pulje',
  'holdem.header.street': 'Gade',
  'holdem.header.hand': 'Hånd',
  'holdem.header.handLimit': 'Hænder i alt',
  'holdem.header.blinds': 'Blinds',
  'holdem.cost.call': 'for at syne',
  'holdem.cost.pot': 'i puljen',
  'holdem.seat.stack': 'Stak',
  'holdem.seat.bet': 'Indsats',
  'holdem.prompt.yourAction': 'Din tur til at handle',
  'holdem.prompt.raiseTo': 'Hæv til',
  'holdem.quick.halfPot': '½ Pulje',
  'holdem.quick.pot': 'Pulje',
  'holdem.quick.allIn': 'All-in',
  'zone.yourHand': 'Din hånd',
  'zone.opponentHand': 'Modstanderens hånd',
  'zone.drawPile': 'Talon',
  'zone.discardPile': 'Kastebunke',
  'zone.melds': 'Kombinationer',
  'zone.teamMelds': 'Din sides kombinationer',
  'zone.opponentMelds': 'Modstanderens kombinationer',
  'zone.redThrees': 'Røde treere',
  'zone.board': 'Bord',
  'verb.drawFromDeck': 'Træk',
  'verb.takeFromDiscard': 'Tag fra bunken',


  // --- the why sheet's own furniture ---------------------------------------
  // The three layers a refusal is explained in. Labels, not sentences: what
  // goes under each is sent by the server or worded above.
  'why.reason': 'Hvorfor ikke',
  'why.rule': 'Reglen',
  'why.rules': 'Reglerne',
  'why.remedy': 'Hvad du kan gøre',
  'why.readTheRules': 'Læs hele reglerne →',
  'why.close': 'Luk',
  'why.open': 'hvorfor',

  // --- marks on a particular card ------------------------------------------
  // A mark is a refusal that has not happened yet: the rule is enforced at the
  // discard, which is the last possible moment to hear about it, so the card
  // says so while there is still a turn left to act on it.
  'zolik.badge.owedToMeld':
    '{card} kom fra kastebunken — det skal indgå i de kombinationer, du lægger ud med i denne tur.',
  'zolik.badge.jokerOwed':
    '{card} kom fra bordet — det skal indgå i en kombination, før du kan afslutte din tur.',

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
  'legal.terms': 'Vilkår',
  // The document's own name, used as the heading above it. Kept apart from
  // `legal.notice.terms`, which is the same words inflected to sit inside a
  // sentence — a heading in the instrumental case reads as a mistake in every
  // Slavic language.
  'legal.terms.title': 'Brugsvilkår',
  'legal.privacy.title': 'Privatlivspolitik',
  'legal.privacy': 'Privatliv',
  'legal.source': 'Kildekode',
  'legal.updated': 'Version {version}',
  'legal.draft':
    'Udkast — endnu ikke gældende. Operatørens navn, land og kontaktadresse mangler stadig at blive udfyldt.',
  'legal.notice.before': 'Ved at spille accepterer du ',
  'legal.notice.terms': 'brugsvilkårene',
  'legal.notice.between': '. Hvad der gemmes om dig, står i ',
  'legal.notice.privacy': 'privatlivspolitikken',
  'legal.notice.after': '.',
  // --- gin rummy -------------------------------------------------------------
  'err.UPCARD_DECLINED': 'Du har allerede sagt nej til det kort',
  'err.DEADWOOD_TOO_HIGH': 'Dit deadwood er for højt til at banke',
  'err.CARD_DOES_NOT_EXTEND_MELD': 'Det kort forlænger ikke denne kombination',
  'ginrummy.rules.setup': 'Opstilling',
  'ginrummy.rules.turn': 'Din tur',
  'ginrummy.rules.melds': 'Kombinationer',
  'ginrummy.rules.knocking': 'At banke',
  'ginrummy.rules.bigGin': 'Big gin',
  'ginrummy.rules.layoff': 'Pålægningen',
  'ginrummy.rules.deadHand': 'Den døde hånd',
  'ginrummy.rules.scoring': 'Point for en hånd',
  'ginrummy.rules.match': 'At vinde matchen',
  'ginrummy.rules.lineBonuses': 'Bonusser i opgørelsen',
  'ginrummy.rules.deck': 'Spilles med et spil på {value} kort.',
  'ginrummy.rules.deal': 'Hver spiller får {value} kort.',
  'ginrummy.rules.upcard': 'Endnu et kort vendes op for at starte kastebunken.',
  'ginrummy.rules.drawDiscard':
    'Træk ét kort i din tur — fra talonen eller kastebunken — og smid derefter ét ud.',
  'ginrummy.rules.setsAndRuns':
    'En kombination er en gruppe på tre eller fire kort af én værdi, eller en række på tre eller flere i én farve.',
  'ginrummy.rules.aceLow': 'Esset er altid lavt — der findes ingen række fra dame til es.',
  'ginrummy.rules.knockLimit': 'Du må banke, så snart dit deadwood er {n} eller derunder.',
  'ginrummy.rules.oklahoma': 'Bankegrænsen i denne hånd sættes af værdien på det opvendte kort.',
  'ginrummy.rules.gin': 'Nul deadwood er gin — den bedst mulige bankning.',
  'ginrummy.rules.bigGinBonus':
    'Elleve kort, der alle indgår i kombinationer, helt uden at smide ud, er big gin og giver yderligere {n} point.',
  'ginrummy.rules.layoffDescription':
    'Efter en bankning, der ikke er gin, må din modstander lægge sit eget deadwood på dine kombinationer, før hænderne sammenlignes.',
  'ginrummy.rules.deadHandDescription':
    'Hvis talonen falder til sine sidste to kort, uden at nogen har banket, er hånden død — ingen får point, og den samme giver giver igen.',
  'ginrummy.rules.undercut':
    'Er din modstanders deadwood ikke højere end dit, underskærer vedkommende dig: modstanderen får forskellen plus {n}.',
  'ginrummy.rules.ginBonus': 'Gin giver hele din modstanders hånd plus {n}.',
  'ginrummy.rules.target': 'Den første, der passerer {n} point efter en hånd, vinder matchen.',
  'ginrummy.rules.shutout': 'Matchbonussen fordobles til {n}, hvis taberen aldrig fik et eneste point.',
  'ginrummy.rules.box': 'Hver hånd, du vandt, er {n} point værd ved matchens slutning.',
  'ginrummy.rules.gameBonus': 'At vinde matchen giver yderligere {n} point.',
  'ginrummy.fact.deadwood': '{value} deadwood',
  'ginrummy.fact.discardCard': 'Smid {value} ud',
  'ginrummy.fact.meldCards': 'På {value}',
  'ginrummy.header.hand': 'Hånd {n}',
  'ginrummy.round.kind': '{value}',
  'ginrummy.round.hand': 'Hånd',
  'ginrummy.seat.score': '{value}',
  'ginrummy.seat.dealer': 'Giver',
  'ginrummy.status.knocked': '{playerId} bankede med {deadwood} i deadwood',
  'ginrummy.status.gin': '{playerId} gik gin',
  'ginrummy.status.lastHand': 'Sidste hånd: {winner} ({kind}, {delta} point)',
  'ginrummy.offer.drawStock': 'Træk fra talonen',
  'ginrummy.offer.drawDiscard': 'Træk fra kastebunken',
  'ginrummy.offer.takeUpcard': 'Tag det opvendte kort',
  'ginrummy.offer.passUpcard': 'Pas',
  'ginrummy.offer.discard': 'Smid ud',
  'ginrummy.offer.knock': 'Bank',
  'ginrummy.offer.gin': 'Gin!',
  'ginrummy.offer.bigGin': 'Big gin!',
  'ginrummy.offer.layOff': 'Læg på',
  'ginrummy.offer.finishLayoff': 'Færdig med at lægge på',
  'ginrummy.zone.knockerHand': 'Bankerens hånd',
  'ginrummy.zone.melds': 'Kombinationer',
  'ginrummy.prompt.upcardDecision': 'Tag det opvendte kort, eller pas',
  'ginrummy.prompt.yourTurnDraw': 'Træk et kort',
  'ginrummy.prompt.yourTurnDiscard': 'Smid ud — eller bank, hvis du kan',
  'ginrummy.prompt.layoff': 'Læg deadwood på, eller afslut',

  // --- rummy tiles -------------------------------------------------------------
  'err.TILE_NOT_IN_HAND': 'Den brik er ikke på din hånd',
  'err.TILE_DOES_NOT_FIT': 'Det passer ikke der',
  'err.NO_SUCH_SET': 'Den kombination ligger ikke på bordet',
  'err.INITIAL_MELD_ONLY': 'Før din første udlægning må du kun flytte rundt på dine egne nye kombinationer',
  'err.TABLE_NOT_VALID': 'Bordet er ikke gyldigt endnu',
  'err.TRAY_NOT_EMPTY': 'Du har stadig løse brikker at placere',
  'err.NOTHING_PLAYED': 'Læg mindst én brik, før du afslutter din tur',
  'err.INITIAL_MELD_TOO_LOW': 'Din første udlægning skal være mindst 30 point værd',
  'err.NOT_A_RUN': 'Kun en række kan deles',
  'err.BAD_SPLIT_POSITION': 'Der kan denne række ikke deles',
  'err.NO_JOKER_IN_SET': 'Der er ingen joker i den kombination',
  'err.TILE_JOKER_SWAP_MISMATCH': 'Den brik er ikke det, jokeren står for',
  'rummytiles.rules.setup': 'Opstilling',
  'rummytiles.rules.sets': 'Kombinationer',
  'rummytiles.rules.initialMeld': 'Den første udlægning',
  'rummytiles.rules.turn': 'Din tur',
  'rummytiles.rules.jokerTaking': 'At tage en joker',
  'rummytiles.rules.ending': 'At afslutte en runde',
  'rummytiles.rules.poolExhaustion': 'Hvis posen slipper op',
  'rummytiles.rules.match': 'At vinde matchen',
  'rummytiles.rules.tiles': 'Spilles med {value} brikker.',
  'rummytiles.rules.dealCount': 'Hver spiller får {value} brikker.',
  'rummytiles.rules.group': 'En gruppe er tre eller fire brikker med samme tal, hver i sin farve.',
  'rummytiles.rules.run': 'En række er tre eller flere tal i træk i samme farve.',
  'rummytiles.rules.noWrap': '13 fortsætter ikke rundt til 1.',
  'rummytiles.rules.joker': 'En joker står for en hvilken som helst brik.',
  'rummytiles.rules.initialMeldDescription':
    'Indtil du har lagt {n} point eller mere ud i én enkelt tur, alene fra din egen hånd, må du ikke røre noget af det, der allerede ligger på bordet.',
  'rummytiles.rules.turnDescription':
    'Læg mindst én brik fra din hånd, flyt frit rundt på bordet, og slut med at hver kombination på bordet er gyldig.',
  'rummytiles.rules.noDiscard':
    'Der smides ikke ud — kan du ikke gennemføre en gyldig tur, trækker du én brik i stedet.',
  'rummytiles.rules.jokerTakingDescription':
    'En joker på bordet må tages ved at erstatte den med den brik, den står for, fra din hånd — og den skal bruges i en kombination, før din tur slutter.',
  'rummytiles.rules.goingOut':
    'Den første spiller uden brikker vinder runden. Alle andre får den negerede værdi af det, de har tilbage; vinderen får summen af, hvad alle andre tabte.',
  'rummytiles.rules.poolExhaustionLowestWins':
    'Slipper posen op, og ingen kan spille, slutter runden, og den laveste håndværdi vinder den.',
  'rummytiles.rules.poolExhaustionNoWinner':
    'Slipper posen op, og ingen kan spille, slutter runden uden vinder — hver hånd tælles blot op.',
  'rummytiles.rules.target': 'Den første, der passerer {n} point efter en runde, vinder matchen.',
  'rummytiles.rules.roundLimit': 'Matchen slutter efter {n} runder — den højeste score vinder.',
  'rummytiles.fact.setCards': '{value}',
  'rummytiles.header.pool': 'Pose {n}',
  'rummytiles.header.round': 'Runde {n}',
  'rummytiles.round.kind': '{value}',
  'rummytiles.round.round': 'Runde',
  'rummytiles.seat.score': '{value}',
  'rummytiles.seat.notOpened': 'Ikke åbnet',
  'rummytiles.status.lastRound': 'Sidste runde: {winner} ({kind})',
  'rummytiles.badge.invalid': 'Ikke gyldig endnu',
  'rummytiles.zone.pool': 'Pose',
  'rummytiles.zone.table': 'Bord',
  'rummytiles.zone.tray': 'Stativ',
  'rummytiles.offer.place': 'Placer',
  'rummytiles.offer.addFromHand': 'Tilføj',
  'rummytiles.offer.addFromTray': 'Tilføj fra stativet',
  'rummytiles.offer.take': 'Tag',
  'rummytiles.offer.split': 'Del',
  'rummytiles.offer.swapJoker': 'Byt jokeren',
  'rummytiles.offer.resetTurn': 'Nulstil turen',
  'rummytiles.offer.commit': 'Færdig',
  'rummytiles.offer.draw': 'Træk',
  'rummytiles.param.position': 'Del ved',

  // --- blackjack -----------------------------------------------------------
  //
  // The house rules are worded twice over — on and off — because a rule that
  // is *not* in force is still something a player has to be told. "No
  // surrender at this table" is information; silence is a guess.
  'err.BET_BELOW_MINIMUM': 'Det er under bordets minimum',
  'err.ALREADY_BET': 'Din indsats ligger allerede',
  'err.INSURANCE_CLOSED': 'Der er ingen forsikring at tage lige nu',
  'err.CANNOT_DOUBLE': 'Denne hånd kan ikke fordobles',
  'err.CANNOT_SPLIT': 'Denne hånd kan ikke deles',
  'err.CANNOT_SURRENDER': 'Denne hånd kan ikke opgives',

  'blackjack.rules.section.table': 'Bordet',
  'blackjack.rules.section.play': 'At spille en hånd',
  'blackjack.rules.section.dealer': 'Dealeren',
  'blackjack.rules.section.end': 'Sådan slutter matchen',
  'blackjack.rules.goal':
    'Slå dealeren uden at komme over enogtyve. Kommer du over, taber du med det samme, uanset hvad dealeren gør bagefter.',
  // Counts are their own phrase rather than a number glued to a noun: one
  // deck and six decks inflect differently, and Czech inflects them again.
  'blackjack.rules.decks': 'Spil i skoen: {n}.',
  'blackjack.rules.stack': 'Hver plads sætter sig med {n} jetoner.',
  'blackjack.rules.minBet': 'Bordets minimum er {n} jetoner.',
  'blackjack.rules.faceUp':
    'Spillernes kort deles ud med billedsiden op; dealeren holder ét kort skjult, indtil alle har spillet.',
  'blackjack.rules.hitStand': 'Tag så mange kort, du vil, eller stå på det, du har.',
  'blackjack.rules.aces': 'Et es tæller som elleve, så længe det kan lade sig gøre, og ellers som ét.',
  'blackjack.rules.blackjack': 'Et es med et tikort, på de to første kort, er en blackjack.',
  'blackjack.rules.pays3to2': 'En blackjack betaler 3:2.',
  'blackjack.rules.pays6to5': 'En blackjack betaler 6:5.',
  'blackjack.rules.paysEven': 'En blackjack betaler én til én.',
  'blackjack.rules.double': 'På dine to første kort må du fordoble din indsats og tage præcis ét kort mere.',
  'blackjack.rules.doubleAfterSplit': 'En hånd, der er kommet ud af en deling, må også fordobles.',
  'blackjack.rules.noDoubleAfterSplit': 'En hånd, der er kommet ud af en deling, må ikke fordobles.',
  'blackjack.rules.split':
    'To kort af samme værdi må deles i hver sin hånd, hver med sin egen indsats — op til {n} gange, til {hands} hænder i alt.',
  'blackjack.rules.noSplit': 'Ved dette bord deles par ikke.',
  'blackjack.rules.splitAces':
    'Delte esser får ét kort hver og står så, og enogtyve lavet på den måde er ikke en blackjack.',
  'blackjack.rules.surrender':
    'Du må opgive din første hånd for halvdelen af indsatsen, når dealeren har tjekket for blackjack.',
  'blackjack.rules.noSurrender': 'Ved dette bord kan hænder ikke opgives.',
  'blackjack.rules.dealerDraws': 'Dealeren trækker til sytten og står så.',
  'blackjack.rules.hitsSoft17': 'Dealeren trækker på en sytten dannet med et es.',
  'blackjack.rules.standsSoft17': 'Dealeren står på en sytten dannet med et es.',
  'blackjack.rules.dealerPeeks':
    'Med et es eller en tier oppe tjekker dealeren for blackjack, før nogen spiller.',
  'blackjack.rules.insurance':
    'Mod et es hos dealeren må du forsikre for halvdelen af din indsats; det betaler 2:1, hvis dealeren har blackjack.',
  'blackjack.rules.noInsurance': 'Ved dette bord tilbydes der ikke forsikring.',
  'blackjack.rules.rounds': 'Bordet spiller {n} runder.',
  'blackjack.rules.mostChipsWins': 'Den, der har flest jetoner til sidst, vinder matchen.',
  'blackjack.rules.bustedOut':
    'En plads, der ikke længere kan dække minimum på {n}, sidder over resten af matchen.',

  'blackjack.zone.dealer': 'Dealer',
  'blackjack.zone.box': 'Hånd',
  'blackjack.zone.yourBox': 'Din hånd',
  'blackjack.zone.shoe': 'Sko',

  'blackjack.header.round': 'Runde {n} af {of}',
  'blackjack.header.minBet': 'Minimum',
  'blackjack.header.decks': 'Spil',
  'blackjack.header.dealerTotal': 'Dealeren viser {n}',
  'blackjack.header.dealerSoftTotal': 'Dealeren viser blød {n}',

  'blackjack.seat.stack': 'Jetoner',
  'blackjack.seat.bet': 'Indsats',
  'blackjack.seat.insurance': 'Forsikring',
  'blackjack.seat.total': 'I alt',
  'blackjack.seat.softTotal': 'Blød sum',
  'blackjack.seat.out': 'Løbet tør for jetoner',

  'blackjack.prompt.placeBet': 'Læg din indsats',
  'blackjack.prompt.insurance': 'Forsikring?',
  'blackjack.prompt.yourMove': 'Din tur',
  'blackjack.prompt.waitingFor': 'Venter på {playerId}',
  'blackjack.prompt.betAmount': 'Indsats',

  'blackjack.quick.doubleMin': '2× Minimum',
  'blackjack.quick.allIn': 'All-in',
  'blackjack.offer.bet': 'Sats',
  'blackjack.offer.hit': 'Tag kort',
  'blackjack.offer.stand': 'Stå',
  'blackjack.offer.double': 'Fordobl',
  'blackjack.offer.split': 'Del',
  'blackjack.offer.surrender': 'Opgiv',
  'blackjack.offer.insure': 'Tag forsikring',
  'blackjack.offer.declineInsurance': 'Ingen forsikring',

  'blackjack.fact.tableMinimum': 'minimum',
  'blackjack.fact.insuranceCost': 'for at forsikre',
  'blackjack.fact.extraStake': 'at satse',
  'blackjack.fact.surrenderReturn': 'tilbage',

  'blackjack.status.dealerBlackjack': 'Dealeren havde blackjack',
  'blackjack.status.dealerBust': 'Dealeren sprang med {n}',
  'blackjack.status.dealerStands': 'Dealeren står på {n}',

  'blackjack.round.name': 'Runde',
  'blackjack.round.dealerTotal': 'Dealer {n}',
  'blackjack.round.dealerBust': 'Dealeren sprang ({n})',
  'blackjack.round.dealerBlackjack': 'Dealerens blackjack',
  'blackjack.round.outcome.blackjack': 'Blackjack',
  'blackjack.round.outcome.win': 'Vundet',
  'blackjack.round.outcome.push': 'Uafgjort',
  'blackjack.round.outcome.lose': 'Tabt',
  'blackjack.round.outcome.bust': 'Sprang',
  'blackjack.round.outcome.surrender': 'Opgivet',

  'blackjack.badge.inPlay': 'I spil',
  'blackjack.badge.doubled': 'Fordoblet',
  'blackjack.badge.split': 'Delt',
  'blackjack.badge.blackjack': 'Blackjack',
  'blackjack.badge.bust': 'Sprang',
  'blackjack.badge.won': 'Vundet',
  'blackjack.badge.push': 'Uafgjort',
  'blackjack.badge.lost': 'Tabt',
  'blackjack.badge.surrendered': 'Opgivet',

  'blackjack.unit.chips': 'jetoner',

  // --- the settings screen --------------------------------------------------
  //
  // Hardcoded English until now, which was survivable while the only other
  // language was unreachable and faintly absurd once the language picker
  // itself lived on this screen: a player who cannot read "Settings" is
  // exactly the one who came here to change it.
  'settings.title': 'Indstillinger',
  'settings.signedInAs': 'Logget ind som {username}',
  'settings.playingAsGuest': 'Spiller som {username} (gæst)',
  'settings.notSignedIn': 'Ikke logget ind — log ind eller fortsæt som gæst for at spille online.',
  'settings.subtitle': 'Hvordan du ser ud, og hvordan bordet gør',
  'settings.face.heading': 'Dit ansigt ved bordet',
  'settings.face.account': 'Gemmes med din konto, så det følger med til en anden enhed.',
  'settings.face.device': 'Gemmes på denne enhed. Log ind for at tage det med dig.',
  'settings.skin.heading': 'Bordets udseende',
  'settings.language.heading': 'Sprog',
  'settings.language.status': 'Gemmes på denne enhed.',
  'settings.language.auto': 'Automatisk',
  'settings.language.auto.now': 'Følger din enhed — nu {language}',
  'settings.legal.heading': 'Det med småt',
  'settings.legal.status': 'Hvad du sagde ja til ved at spille, og hvad der gemmes om dig.',
  'settings.signIn': 'Log ind',
  'settings.back': 'Tilbage',

  // A translated interface over an English notice is a worse lie than an
  // English interface, so the document says so itself rather than letting the
  // surrounding screen imply the text was written for this reader.
  'legal.untranslated':
    'Denne meddelelse er endnu ikke oversat til dit sprog. Den engelske tekst nedenfor er den gældende version.',

  // --- the navigation bar's screen titles ------------------------------------
  //
  // These sit in `app/_layout.tsx`, above every screen, and were the last
  // English left after the settings screen was translated — a German player
  // reading "Einstellungen" under a bar that said "Settings". A translated app
  // with an untranslated chrome bar looks like a bug, not like a limit.
  'nav.home': 'Zolik',
  'nav.emailSignIn': 'Log ind med e-mail',
  'nav.signingIn': 'Logger ind',
  'nav.usernameSignIn': 'Log ind med brugernavn',
  'nav.legacyAccount': 'Gammel konto',
  'nav.guest': 'Gæst',
  'nav.account': 'Konto',
  'nav.games': 'Spil',
  'nav.table': 'Dit bord',
  'nav.join': 'Slut dig til et bord',
  // The screen a shared invite link lands on while it resolves the code.
  'nav.joining': 'Tilslutter',
  'nav.rules': 'Regler',
  'nav.match': 'Kamp',
  'nav.scoreTable': 'Pointtavle',
  'nav.stats': 'Statistik',
  'nav.more': 'Mere',
  'nav.about': 'Om',

  // --- the account menu behind the face in the corner ----------------------
  'menu.label': 'Kontomenu',
  'menu.signedIn': 'Logget ind',
  'menu.notSignedIn': 'Ikke logget ind',
  'menu.keepStats': 'for at beholde din statistik',
  'menu.signOut': 'Log ud',
  'more.scoreTable': 'Offline pointtabel',
  'more.stats': 'Statistik og rangliste',
  'more.needsAccount': 'log ind for at bruge',
  'gate.title': 'Log ind for at bruge dette',
  'gate.body':
    'Pointtabeller og statistik gemmes med din konto, så de følger dig til en anden enhed. En gæst har ingen steder at gemme dem.',

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
  'error.generic': 'Det virkede ikke',
  'error.signIn': 'Login mislykkedes',
  'error.login': 'Login mislykkedes',
  'error.register': 'Oprettelsen mislykkedes',
  'error.sendCode': 'Kunne ikke sende en kode',
  'error.badCode': 'Den kode virkede ikke',
  'error.rulesLoad': 'Kunne ikke hente reglerne',
  'error.createFailed': 'Kunne ikke oprette',
  'error.saveFailed': 'Kunne ikke gemme',
  'error.exportFailed': 'Kunne ikke eksportere',

  // --- the route that does not exist ----------------------------------------
  'notFound.title': 'Ups!',
  'notFound.message': 'Den her skærm findes ikke.',
  'notFound.home': 'Gå til startskærmen!',

  // --- signing in -----------------------------------------------------------
  'auth.login.subtitle': 'Behold din statistik på alle enheder',
  'auth.login.continueWithEmail': 'Fortsæt med e-mail',
  'auth.login.usernameInstead': 'Log ind med brugernavn i stedet',
  'auth.email.title': 'Log ind med e-mail',
  'auth.email.subtitle': 'Vi sender dig en engangskode',
  'auth.email.address': 'E-mailadresse',
  'auth.email.send': 'Send kode',
  'auth.email.codeTitle': 'Indtast koden',
  'auth.email.codePlaceholder': 'Sekscifret kode',
  'auth.email.differentAddress': 'Brug en anden adresse',
  'auth.email.sentTo': 'Sendt til {email}',
  'auth.email.continue': 'Fortsæt',
  'auth.guest.title': 'Spil som gæst',
  'auth.guest.subtitle': 'Ingen konto nødvendig',
  'auth.guest.displayName': 'Vist navn',
  'auth.register.title': 'Opret konto',
  'auth.register.username': 'Brugernavn',
  'auth.register.email': 'E-mail (valgfrit)',
  'auth.register.password': 'Adgangskode',
  'auth.username.createAccount': 'Opret en konto med brugernavn og adgangskode',
  'auth.callback.signedIn': 'Logget ind.',

  // --- the account screen ---------------------------------------------------
  'account.signInPrompt': 'Log ind for at administrere din konto.',
  'account.keepGames': 'Behold de her spil',
  'account.signedInWith': 'Logget ind med',
  'account.addMethod': 'Tilføj en loginmetode',
  'account.usernameAndPassword': 'Brugernavn og adgangskode',
  'account.faceAndTable': 'Ansigt og bordets udseende',
  'account.refresh': 'Opdater',
  'account.remove': 'Fjern',

  // --- the main menu --------------------------------------------------------
  'home.subtitle': 'Kontinental rommy · {server}',
  'home.playingAs': 'Du spiller som {name}',
  'home.signInPrompt': 'Log ind eller fortsæt som gæst for at spille online.',
  'home.statsAndLeaderboard': 'Statistik og rangliste',
  'home.play': 'Spil',
  'home.offlineScoreTable': 'Pointtavle offline',
  'home.signInToKeepStats': 'Log ind for at beholde din statistik',
  'home.signOut': 'Log ud',
  'home.continueAsGuest': 'Fortsæt som gæst',
  // Someone playing as a guest, marked in a list of names.
  'home.guestSuffix': '(gæst)',

  // --- the waiting room on the main menu ------------------------------------
  //
  // Counts are whole phrases per count, not a number glued to a noun — the
  // same reason `countLabel` exists. One and many are the cases the interface
  // actually produces, so they are the cases that get their own wording.
  'waiting.checking': 'Ser efter, hvem der er her…',
  'waiting.youAreWaiting': 'Du venter på at spille',
  'waiting.pickedUp': 'Enhver, der åbner et bord, kan samle dig op — ingen har brug for en kode fra dig.',
  'waiting.othersOne': '1 spiller mere venter også',
  'waiting.othersMany': '{n} spillere mere venter også',
  'waiting.oneWaiting': '1 spiller venter på at spille',
  'waiting.manyWaiting': '{n} spillere venter på at spille',
  'waiting.adding': 'Du bliver føjet til ventelisten…',
  'waiting.slowHint':
    'Hvis det ikke bliver færdigt på et par sekunder, så tjek, om serveradressen nedenfor kan nås fra denne enhed.',
  'waiting.serverBusyDetail':
    'Forsøg {n}. Serveren tager lige nu ikke imod nye forbindelser til venteværelset.',
  'waiting.reconnecting': 'Forbindelsen røg — forbinder igen…',
  'waiting.reconnectingDetail':
    'Forsøg {n}. Det kan ske, hvis enhedens netværk er skiftet, eller serveren blev genstartet.',
  'waiting.tryAgain': 'Prøv igen nu',
  'waiting.makeAvailable': 'Gør mig tilgængelig til at spille',
  'waiting.stop': 'Hold op med at vente',
  'waiting.noneYet':
    'Lige nu venter ingen på at spille. Skriv dig på listen, så er du den første, nogen ser.',
  'waiting.noOthersYet': 'Ingen andre venter endnu. Værter kan se dig alligevel og kan invitere dig.',
  'waiting.server': 'Server',
  'waiting.none': 'Lige nu venter ingen. Den, der gør sig tilgængelig i hovedmenuen, dukker op her.',

  // --- landing on a shared invite link --------------------------------------
  'join.missingCode': 'Det link mangler sin bordkode.',
  'join.staleLink': 'Bed den, der inviterede dig, om et nyt link, eller kom med via koden i stedet.',
  'join.enterCode': 'Indtast en kode',
  'join.backToMenu': 'Tilbage til menuen',
  'join.takingSeat': 'Tager plads…',
  'join.takingSeatAt': 'Tager plads ved {game}…',

  // --- the lobby ------------------------------------------------------------
  'lobby.games.subtitle': 'Alt hvad denne server kan byde på',
  'lobby.games.bots': 'Bots',
  'lobby.games.playBot': 'Spil mod en bot',
  'lobby.games.playBots': 'Spil mod {n} bots',
  'lobby.games.openTable': 'Åbn et bord',
  'lobby.games.players': '{n} spillere',
  'lobby.games.playerRange': '{min}–{max} spillere',
  'lobby.join.placeholder': 'Kode eller invitationslink',
  'lobby.join.needCode': 'Indtast en kode, et link eller et kamp-id',
  'lobby.games.signInFirst': 'Log ind først',
  'lobby.join.action': 'Deltag',
  'lobby.join.waitingTitle': 'Venter på værten',
  // Two whole sentences rather than one with a swapped noun: "a game of {game}"
  // does not survive a language that inflects the game's name after "of".
  'lobby.join.joinedGame': 'Du er med i et spil {game} — venter på start',
  'lobby.join.joinedTable': 'Du er med ved bordet — venter på start',
  'lobby.table.addBot': 'Tilføj en bot',
  'lobby.table.side': 'Side {n}',
  'lobby.table.shuffleSeats': 'Bland pladserne',
  'lobby.table.moveSeatUp': 'Flyt {name} en plads op',
  'lobby.table.moveSeatDown': 'Flyt {name} en plads ned',
  'lobby.table.start': 'Start',
  'lobby.table.waitingForHost': 'Venter på, at værten starter…',

  // --- inviting someone to a table ------------------------------------------
  'invite.heading': 'Inviter spillere',
  'invite.explain': 'Send det her link. Den, der åbner det, lander ved dette bord — ingen konto nødvendig.',
  'invite.noAddress': 'Denne server har ingen delbar adresse sat op, så brug koden nedenfor.',
  'invite.readOutCode': 'Eller læs koden op:',
  'invite.copy': 'Kopiér link',
  'invite.share': 'Del link',
  'invite.copied': 'Kopieret!',
  'invite.shared': 'Delt',

  // --- the match screen -----------------------------------------------------
  'match.waitingForTable': 'Venter på bordet…',
  'match.waitingForPlayer': 'Venter på en anden spiller…',
  'match.nobodyWon': 'Ingen vandt.',
  'match.youWon': 'Du vandt.',
  'match.finished': 'Denne kamp er slut.',
  'match.inProgress': 'Kampen er i gang — alt er forbundet og kører normalt.',
  'match.connecting': 'Forbinder…',
  'match.abandonedTitle': 'Bordet lagt til side',
  'match.abandoned': 'Ingen kom tilbage til dette bord, så det blev lagt til side. Kortene ligger præcis, hvor du forlod dem.',
  'match.resume': 'Fortsæt hvor du slap',
  'match.resuming': 'Henter bordet tilbage…',
  'match.controls': 'Betjening',
  'match.over': 'Kampen er slut',
  'match.settingUp': 'Gør klar…',
  'match.playAgain': 'Spil igen',
  'match.backToGames': 'Tilbage til spillene',
  'match.table': 'Bord',
  'match.opponents': 'Modstandere',
  // Marks which seat is the reader's own, in a list of seats.
  'match.youSuffix': '(dig)',
  // The winner line is composed from whole sentences rather than a name glued
  // to " won." — the verb agrees with the subject in most of these languages,
  // and a suffix cannot know that.
  'match.you': 'dig',
  'match.someoneWon': '{name} vandt.',
  'match.wonBy': 'Vundet af {names}.',
  'match.pausedFor': 'På pause — venter på, at {name} forbinder igen.',
  'match.results': 'Resultater',
  'match.players': 'Spillere',
  'match.toPlay': 'i tur',

  // --- the offline score table ----------------------------------------------
  'scoring.namesHint': 'Navne adskilt af kommaer (4–8 spillere)',
  'scoring.newSession': 'Ny session',
  // A worked example, not a sentence: the names are placeholders a translator
  // may localise, the shape `name:score,` is what the parser needs.
  'scoring.scoresPlaceholder': 'Anna:120,Bo:80,…',
  'scoring.saveRound': 'Gem runden',
  'scoring.export': 'Eksportér scorekortet',
  'scoring.formatHint': 'Pointformat: Navn:100,Navn2:50',
  'scoring.nameCountError': 'Indtast 2–8 spillernavne adskilt af kommaer',
  'scoring.session': 'Session: {id}',
  'scoring.players': 'Spillere: {names}',
  'scoring.roundScores': 'Point for runde {n}',
  'stats.loading': 'Indlæser…',
  // The figures could not be fetched. `{reason}` is whatever the server or the
  // network said, which is not ours to translate — the frame around it is.
  'stats.unavailable': '(utilgængelig: {reason})',

  // --- statistics -----------------------------------------------------------
  'stats.title': 'Statistik og rangliste',
  'stats.yours': 'Din statistik',
  'stats.leaderboard': 'Rangliste',

  // --- a player's lifetime record, shown beside a finished match ------------
  'record.title': 'Dit facit',
  'record.guest':
    'Du spiller som gæst, så der føres ikke noget facit. Log ind, og de spil, du allerede har spillet på denne enhed — også dette — knyttes til din konto.',
  'record.signInToKeep': 'Log ind og behold dem',
  'record.failed': 'Dit facit kunne ikke hentes lige nu. Kampen er trygt registreret.',
  'record.loading': 'Indlæser…',
  'record.played': 'Spillede',
  'record.won': 'Vundne',
  'record.lost': 'Tabte',
  'record.winRate': 'Vindprocent',
  'record.streak': 'Stime',
  'record.atThisGame': 'I dette spil',
  // A streak is a count with a noun, so it is a whole phrase per count for
  // the same reason `countLabel` is: "1 win" / "2 wins" is an English rule,
  // and most of these languages do not share it.
  'record.streakNone': '—',
  'record.streakWinOne': '1 sejr',
  'record.streakWinMany': '{n} sejre',
  'record.streakLossOne': '1 nederlag',
  'record.streakLossMany': '{n} nederlag',

  // --- moving cards ---------------------------------------------------------
  'hand.dragHint': 'Træk et kort langs viften for at flytte det, eller ud på bordet for at spille det',
  'hand.moveLeft': 'Til venstre',
  'hand.moveRight': 'Til højre',
  'zone.collapseGroup': 'Fold denne gruppe sammen',
  'zone.expandGroup': 'Vis alle kort i denne gruppe',
  'zone.dropHere': 'Slip her',
  'offer.pickCards': 'vælg kort til det sted, du trykkede på',
  'offer.ambiguous': 'det her kan komme flere steder hen — vælg på bordet',

  // --- the build footer -----------------------------------------------------
  'build.app': 'app',
  'build.server': 'server',
  'about.subtitle': 'Den version, du spiller, og det med småt.',
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
  'option.pauseBetweenRounds': 'Pause mellem runder',
  'choice.pauseBetweenRounds.1': 'Pause',
  'choice.pauseBetweenRounds.0': 'Spil videre med det samme',
  'option.botSkill': 'Modstandere',
  'choice.botSkill.0': 'Blandede',
  'choice.botSkill.1': 'Lette',
  'choice.botSkill.2': 'Mellem',
  'choice.botSkill.3': 'Svære',
  'option.initialMeldMinimum': 'Åbningsværdi',
  'choice.initialMeldMinimum.0': 'Ingen',
  'option.discardDrawMinRound': 'Tag fra kastebunken',
  'choice.discardDrawMinRound.0': 'Åben',
  'choice.discardDrawMinRound.2': 'Fra runde 2',
  'choice.discardDrawMinRound.3': 'Fra runde 3',
  'option.requireCleanRun': 'Jokerfri række',
  'choice.requireCleanRun.1': 'Påkrævet',
  'choice.requireCleanRun.0': 'Nej',
  'option.jokerReclaimMustPlay': 'Tilbagekøbt joker',
  'choice.jokerReclaimMustPlay.1': 'Spilles samme tur',
  'choice.jokerReclaimMustPlay.0': 'Må beholdes',
  'option.dealStarter': 'Udspil',
  'choice.dealStarter.0': 'På skift',
  'choice.dealStarter.1': 'Vinderen spiller ud',
  'variation.prsi.classic': 'Klassisk',
  'option.handSize': 'Uddelte kort',
  'variation.canasta.classic': 'Klassisk',
  'variation.canasta.modern_american': 'Modern American',
  'variation.canasta.samba': 'Samba',
  'option.targetScore': 'Målpoint',
  'option.canastasToGoOut': 'Canastaer for at gå ud',
  'variation.holdem.freezeout': 'Freezeout',
  'variation.holdem.timed': 'Fast antal hænder',
  'option.startingStack': 'Startjetoner',
  'option.bigBlind': 'Big blind',
  'option.handLimit': 'Hænder',
  'choice.handLimit.0': 'Indtil én plads er tilbage',
  'variation.ginrummy.standard': 'Standard',
  'option.knockLimit': 'Bankegrænse',
  'choice.knockLimit.0': 'Oklahoma (det opvendte kort sætter den)',
  'option.bigGin': 'Big gin',
  'choice.bigGin.0': 'Fra',
  'choice.bigGin.1': 'Til (+25)',
  'option.lineBonuses': 'Bonusser i opgørelsen',
  'choice.lineBonuses.1': 'Til',
  'choice.lineBonuses.0': 'Fra',
  'variation.rummytiles.standard': 'Standard',
  'choice.targetScore.0': 'Ingen',
  // Two games qualify a number the others mean plainly: Canasta's 500 is a
  // short game, Gin Rummy's and Rummy Tiles' 500 is just 500; Hold'em's
  // starting 200 is a short stack, Blackjack's 200 is just 200. Module-scoped
  // so the qualifier travels with the game that means it — a bare key would
  // hand the note to games it is false for. `scripts/check-server-labels.js`
  // is what finds these; both were shipping as English on a Czech screen.
  'choice.canasta.targetScore.500': '500 (kort)',
  'choice.holdem.startingStack.200': '200 (kort)',
  'option.roundLimit': 'Rundegrænse',
  'choice.roundLimit.0': 'Ingen',
  'option.poolExhaustion': 'Hvis posen slipper op',
  'choice.poolExhaustion.1': 'Laveste hånd vinder runden',
  'choice.poolExhaustion.0': 'Ingen vinder runden',
  'variation.blackjack.single': 'Ét spil',
  'option.minBet': 'Bordets minimum',
  'option.rounds': 'Runder',
  'option.decks': 'Spil',
  'option.dealerHitsSoft17': 'Dealer på blød 17',
  'choice.dealerHitsSoft17.0': 'Står',
  'choice.dealerHitsSoft17.1': 'Trækker',
  'option.blackjackPays': 'Blackjack betaler',
  'choice.blackjackPays.100': 'Én til én',
  'option.maxSplits': 'Deling',
  'choice.maxSplits.0': 'Ingen deling',
  'choice.maxSplits.1': 'Én gang (to hænder)',
  'choice.maxSplits.3': 'Tre gange (fire hænder)',
  'option.doubleAfterSplit': 'Fordobling efter deling',
  'choice.doubleAfterSplit.1': 'Tilladt',
  'choice.doubleAfterSplit.0': 'Ikke tilladt',
  'option.surrender': 'Opgivelse',
  'choice.surrender.0': 'Fra',
  'choice.surrender.1': 'Sen opgivelse',
  'option.insurance': 'Forsikring',
  'choice.insurance.1': 'Tilbydes',
  'choice.insurance.0': 'Tilbydes ikke',

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
  'verb.add': 'Tilføj',
  'verb.bet': 'Sats',
  'verb.call': 'Syn',
  'verb.check': 'Tjek',
  'verb.commit': 'Færdig',
  'verb.continue': 'Fortsæt',
  'verb.decline_insurance': 'Ingen forsikring',
  'verb.discard': 'Smid ud',
  'verb.double': 'Fordobl',
  'verb.draw': 'Træk',
  'verb.finish_layoff': 'Færdig med at lægge på',
  'verb.fold': 'Kast dig',
  'verb.hit': 'Tag kort',
  'verb.insure': 'Tag forsikring',
  'verb.knock': 'Bank',
  'verb.lay_meld': 'Læg ud',
  'verb.lay_off': 'Læg på',
  'verb.pass': 'Pas',
  'verb.place': 'Placer',
  'verb.play_card': 'Spil',
  'verb.raise': 'Hæv',
  'verb.reset_turn': 'Nulstil turen',
  'verb.split': 'Del',
  'verb.stand': 'Stå',
  'verb.surrender': 'Opgiv',
  'verb.swap_joker': 'Byt jokeren',
  'verb.take': 'Tag',
  'verb.take_pile': 'Tag fra bunken',
  // Declared by the server and listed in `serverKeys.json`, but never worded.
  'verb.takePileFromHand': 'Tag bunken på hånden',
  'verb.takePileOntoMeld': 'Tag bunken på en kombination',
  'verb.takeTopForSequence': 'Tag det øverste kort til en sekvens',
  'verb.undoDraw': 'Fortryd træk',
  'verb.undoLayOff': 'Fortryd pålægning',
  'verb.undoMeld': 'Fortryd kombination',
  'verb.undoTakePile': 'Fortryd træk fra bunken',
  'verb.undoTurn': 'Fortryd tur',

  // --- suits, spelled out ---------------------------------------------------
  //
  // Read aloud by a screen reader and shown where a pip would not fit. The
  // pips themselves are drawn, not written, so these are the only place the
  // suit is ever a word.
  'suit.C': 'Klør',
  'suit.D': 'Ruder',
  'suit.H': 'Hjerter',
  'suit.S': 'Spar',

  // --- counters the board prints beside a number ----------------------------
  'canasta.seat.notOpened': 'Ikke åbnet',
  'canasta.unit.points': 'point',
  'ginrummy.unit.points': 'point',
  'holdem.seat.dealer': 'Giver',
  'holdem.seat.folded': 'Passet',
  'holdem.seat.allIn': 'All in',
  'holdem.seat.out': 'Ude',
  'holdem.unit.chips': 'jetoner',
  'prsi.unit.cardsLeft': 'kort tilbage',
  'rummytiles.prompt.initialMeld': 'Din første udlægning skal være {n} point værd.',
  'rummytiles.unit.points': 'point',
  'zolik.unit.penalty': 'straf',
  'header.pileFrozen': 'Bunken frosset',
  // The module-less form of `ginrummy.prompt.yourTurnDraw`: Žolíky and Canasta
  // send the prompt without a module prefix. Neither the Go constants nor
  // `serverKeys.json` list it — the sweep is what found it.
  'prompt.yourTurnDraw': 'Træk et kort',
  'prompt.yourTurnMeld': 'Læg ned hvis du kan, og kast så',
};
