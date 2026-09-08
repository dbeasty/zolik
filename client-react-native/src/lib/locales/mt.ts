/**
 * Maltese. Rummy vocabulary: grupp for a set, sekwenza for a run, kombinazzjoni for a meld, mazz for the stock, munzell tal-iskart for the discard pile.
 */

export const mt: Record<string, string> = {
  // --- engine error codes (rules.RulesErrorCode) ---------------------------
  'err.NOT_YOUR_TURN': "M'huwiex imissek",
  'err.WRONG_PHASE': 'Bħalissa mhux possibbli',
  'err.MUST_DRAW_FIRST': 'Iġbed karta qabel ma tniżżel',
  'err.GAME_SUSPENDED': 'Il-logħba hija wieqfa',
  'err.GAME_NOT_ACTIVE': 'Il-logħba mhijiex għaddejja',
  // --- runtime refusals, which are not the rules saying no -----------------
  //
  // Both were reachable long before they were worded, and a player who met
  // one was shown the code in capitals or, worse, nothing at all: an action
  // sent while the socket was down simply vanished, which reads as a legal
  // move being silently refused.
  'err.MATCH_NOT_ACTIVE': "Il-mejda hija wieqfa — qed nistennew li plejer jerġa' jaqbad",
  'err.NOT_CONNECTED': "M'hemmx konnessjoni mal-mejda — qed nerġgħu naqbdu, imbagħad erġa' pprova",
  // Why the "start the next round" control is greyed out. Without wording the
  // shell printed the code itself, so a player agreeing to go on was answered
  // with ALREADY_READY in capitals.
  'err.ALREADY_READY': 'Int lest',
  'err.NOT_BETWEEN_ROUNDS': 'Ir-rawnd għadu għaddej',
  'err.NOT_AT_THIS_TABLE': "M'intix f'din il-mejda",
  'err.DISCARD_LOCKED': 'Il-munzell tal-iskart huwa msakkar għalissa',
  'err.DISCARD_PILE_EMPTY': 'Il-munzell tal-iskart huwa vojt',
  'err.NO_CARDS_LEFT': "M'hemmx aktar karti x'tiġbed",
  'err.ROUND_REQ_NOT_MET': 'L-ewwel niżżel il-ftuħ tiegħek',
  'err.NEED_CLEAN_RUN': 'Għandek bżonn sekwenza mingħajr joker fuq il-mejda biex titqies li niżżilt',
  'err.INCOMPLETE_INITIAL_MELD': 'Temm it-tniżżil tiegħek, jew ħassru, qabel ma tarmi',
  'err.DISCARD_CARD_NOT_MELDED': 'Il-karta li ġbidt trid tidħol fil-kombinazzjoni tiegħek',
  'err.JOKER_DISCARD_FORBIDDEN': 'Joker ma jistax jintrema',
  'err.NOTHING_TO_UNDO': "M'hemm xejn x'iġġib lura",
  'err.NO_JOKER_IN_MELD': "M'hemm ebda joker f'din il-kombinazzjoni",
  'err.JOKER_SWAP_MISMATCH': 'Dik il-karta ma tiħux post il-joker',
  'err.RECLAIMED_JOKER_NOT_MELDED':
    "Il-joker li ħadt minn fuq il-mejda għandu jintlagħab f'kombinazzjoni f'din id-dawra",
  'err.RUN_TOO_LONG': 'Dik is-sekwenza diġà għandha t-tul sħiħ tagħha',
  'err.WRONG_RUN_END': 'Dik il-karta ttawwal it-tarf l-ieħor tas-sekwenza',
  'err.INVALID_MELD': "L-ebda karta f'idek ma tmur hawn",
  'err.CARD_NOT_IN_HAND': "Dik il-karta mhijiex f'idek",
  'err.MELD_BELOW_MINIMUM': 'Il-kombinazzjonijiet tiegħek għadhom nieqsa mill-punti biex tniżżel',
  'err.MELD_NO_CONTRIBUTION': "Dik il-kombinazzjoni ma ġġibx 'il quddiem ir-rekwiżit tiegħek",
  'err.TOO_MANY_WILDS': "Wisq jokers f'dik il-kombinazzjoni",
  'err.ADJACENT_WILDS': 'Żewġ jokers ma jistgħux joqogħdu maġenb xulxin',
  'err.ACE_BRIDGE': 'Ass ma jistax jgħaqqad ir-Re mat-Tnejn',

  // --- contract phrasing ---------------------------------------------------
  // Built from the structured contract the server sends (sets/runs counts),
  // never from a deal-number lookup table.
  // Whole phrases per count rather than a number plus a pluralised noun:
  // Czech inflects the noun by count (skupina / skupiny / skupin) in a way no
  // "add an s" helper survives, and the counts that actually occur are 1-3.
  // A count outside that falls back to contract.sets.n.
  'contract.sets.1': 'Grupp wieħed',
  'contract.sets.2': 'Żewġ gruppi',
  'contract.sets.3': 'Tliet gruppi',
  'contract.sets.n': 'Gruppi: {n}',
  'contract.runs.1': 'Sekwenza waħda',
  'contract.runs.2': 'Żewġ sekwenzi',
  'contract.runs.3': 'Tliet sekwenzi',
  'contract.runs.n': 'Sekwenzi: {n}',
  'contract.any': 'Kwalunkwe kombinazzjoni valida',
  'contract.cleanRunOnly':
    "Kwalunkwe taħlita ta' gruppi u sekwenzi — mill-inqas sekwenza waħda trid tkun mingħajr joker",
  'contract.cleanRunSuffix': '{base} — sekwenza waħda trid tkun mingħajr joker',

  // --- written rules ---------------------------------------------------------
  // Full-sentence keys a "see the rules" screen renders, one per module,
  // resolved server-side against the table's actual variation and options —
  // see internal/module/rules.go and each game's rules.go. Shared section
  // titles first, then one block per module in the order the four games were
  // added.
  'zolik.rules.section.goal': 'Għan',
  'zolik.rules.section.setup': 'Tħejjija',
  'zolik.rules.section.turn': 'Id-dawra tiegħek',
  'zolik.rules.section.melding': 'It-tniżżil',
  'zolik.rules.section.end': 'Kif tispiċċa l-partita',
  'zolik.rules.goal':
    "Kun l-ewwel wieħed li jbattal idu billi tniżżel gruppi u sekwenzi validi, filwaqt li tiġbor l-inqas punti ta' penali possibbli fil-karti li jkun għad għandek f'idek meta ħaddieħor joħroġ.",
  'zolik.rules.deal': 'Kull plejer jieħu {n} karti.',
  'zolik.rules.meldShapes':
    'Grupp huwa {set}+ karti tal-istess valur; sekwenza hija {run}+ karti wara xulxin tal-istess kulur.',
  'zolik.rules.turn.draw': 'Fid-dawra tiegħek iġbed karta waħda — mill-mazz jew mill-munzell tal-iskart.',
  'zolik.rules.pickup.topOnly': "Mill-munzell tal-iskart tista' tittieħed biss il-karta ta' fuq.",
  'zolik.rules.pickup.anyFromPile':
    "Mill-munzell tal-iskart tista' tittieħed kwalunkwe karta, flimkien ma' dak kollu li jkun fuqha.",
  'zolik.rules.pickup.locked': 'Mill-munzell tal-iskart ma tistax tiġbed qabel ir-rawnd {n}.',
  'zolik.rules.pickup.open': 'Il-munzell tal-iskart huwa miftuħ mill-ewwel rawnd.',
  'zolik.rules.turn.discard': 'Temm id-dawra tiegħek billi tarmi karta waħda.',
  'zolik.rules.jokers.restricted':
    "Joker qatt ma jista' jintrema, ħlief meta jkun eżattament il-karta li tbattal idek.",
  'zolik.rules.lead.rotate': "Il-bidu jiċċaqlaq post wieħed f'kull tqassim, ikun min ikun rebaħ.",
  'zolik.rules.lead.winner': 'Min joħroġ jibda t-tqassim li jmiss.',
  'zolik.rules.meldFloor.on':
    'L-ewwel tniżżil tiegħek irid jilħaq mill-inqas {n} punti naturali qabel ma tkun niżżilt.',
  'zolik.rules.meldFloor.off': "M'hemm ebda valur minimu ta' punti fl-ewwel tniżżil tiegħek.",
  'zolik.rules.cleanRun.on':
    'Mill-inqas waħda mis-sekwenzi tiegħek trid tkun kompletament mingħajr joker qabel ma titqies li niżżilt.',
  'zolik.rules.cleanRun.off':
    "Is-sekwenzi tiegħek jistgħu jużaw jokers b'mod liberu — ebda waħda ma trid tkun mingħajrhom.",
  'zolik.rules.contracts.rotating':
    "Il-partita ddum {n} tqassimiet, u kull tqassim jitlob it-taħlita tiegħu ta' gruppi u sekwenzi.",
  'zolik.rules.contracts.static': 'Kull tqassim jitlob l-istess taħlita: {sets} gruppi u {runs} sekwenzi.',
  'zolik.rules.end.afterDeals': 'Il-partita tispiċċa wara {n} tqassimiet.',
  'zolik.rules.end.atScore': "Jibqa' jitqassam sakemm xi ħadd jilħaq {n} punti — imbagħad tispiċċa.",

  'prsi.rules.section.goal': 'Għan',
  'prsi.rules.section.setup': 'Tħejjija',
  'prsi.rules.section.turn': 'Id-dawra tiegħek',
  'prsi.rules.section.special': 'Karti speċjali',
  'prsi.rules.section.end': 'Kif tispiċċa l-partita',
  'prsi.rules.goal': 'Kun l-ewwel wieħed li jilgħab kull karta minn idu.',
  'prsi.rules.deck': "Jintlagħab b'mazz ta' {value} karti (mis-7 'il fuq).",
  'prsi.rules.deal': "Kull plejer jibda b'{n} karti.",
  'prsi.rules.turn.match':
    "Ilgħab karta li taqbel mal-kulur jew mal-valur tal-karta ta' fuq — jew iġbed, jekk ma tistax.",
  'prsi.rules.turn.draw': 'Il-ġbid itemm id-dawra tiegħek mingħajr ma tilgħab.',
  'prsi.rules.sevens': "Ilgħab 7 u l-plejer li jmiss jiġbed żewġ karti, ħlief jekk iwieġeb b'7 tiegħu.",
  'prsi.rules.aces': 'Ilgħab ass u d-dawra tal-plejer li jmiss taqbeż.',
  'prsi.rules.queens': 'Ilgħab reġina u semmi l-kulur li jkompli.',
  'prsi.rules.end': 'Il-partita tispiċċa fil-mument li id xi ħadd tkun vojta.',

  'canasta.rules.section.goal': 'Għan',
  'canasta.rules.section.setup': 'Tħejjija',
  'canasta.rules.section.melding': 'It-tniżżil',
  'canasta.rules.section.end': 'Kif tispiċċa l-partita',
  'canasta.rules.goal': "Jintlagħab f'pari; l-ewwel naħa li tilħaq {n} punti tirbaħ il-partita.",
  'canasta.rules.deck': "Jintlagħab b'{value} karti — żewġ mazzi flimkien mal-jokers.",
  'canasta.rules.deal': 'Kull plejer jieħu {n} karti.',
  'canasta.rules.redThrees':
    "Tlieta ħamra f'idek tintwera mill-ewwel u tgħodd bħala bonus — ħlief jekk in-naħa tiegħek qatt ma tlesti canasta, u mbagħad tgħodd kontrik.",
  'canasta.rules.canasta': "Canasta hija kombinazzjoni ta' {n} karti jew aktar tal-istess valur.",
  'canasta.rules.meldFloorBands':
    "L-ewwel tniżżil tiegħek irid jilħaq minimu ta' punti li jitla' mal-iskor tiegħek: {negative} taħt iż-żero, {low} sa 1500, {mid} sa 3000, {high} 'il fuq minn hekk.",
  'canasta.rules.oneCanastaToGoOut': 'Canasta waħda mlestija hija biżżejjed biex in-naħa tiegħek toħroġ.',
  'canasta.rules.twoCanastasToGoOut':
    "In-naħa tiegħek għandha bżonn żewġ canastas mlestija qabel ma tkun tista' toħroġ.",
  'canasta.rules.end':
    "Jibqa' jitqassam sakemm naħa waħda taqbeż {n} punti — imbagħad il-partita tkun spiċċat.",

  'holdem.rules.section.goal': 'Għan',
  'holdem.rules.section.setup': 'Tħejjija',
  'holdem.rules.section.betting': 'L-imħatri',
  'holdem.rules.section.end': 'Kif tispiċċa l-partita',
  'holdem.rules.goal':
    "Irbaħ ċipsijiet billi jkollok l-aħjar id fil-wiri, jew billi tibqa' l-uniku plejer fl-id.",
  'holdem.rules.stack': "Kull post jibda b'{n} ċipsijiet.",
  'holdem.rules.blinds': 'Il-blind iż-żgħir huwa {sb} u l-kbir {bb}, jitqiegħdu qabel ma jitqassmu l-karti.',
  'holdem.rules.streets': "Isir imħatri f'erba' rawnds — qabel il-flop, u wara l-flop, it-turn u r-river.",
  'holdem.rules.showdown':
    "Dawk li għadhom fl-id jikxfu l-karti tagħhom; l-aħjar id ta' ħames karti tieħu l-pot.",
  'holdem.rules.noLimit': "Bla limitu — kwalunkwe mħatra tista' tkun sal-munzell kollu tiegħek.",
  'holdem.rules.lastPlayerStanding': 'Jintlagħab sakemm post wieħed ikollu ċ-ċipsijiet kollha.',
  'holdem.rules.mostChipsWins': 'Min ikollu l-aktar ċipsijiet meta tieqaf il-logħba jirbaħ il-partita.',
  'holdem.rules.handLimit': 'Il-logħba tieqaf wara {n} idejn.',

  // --- header --------------------------------------------------------------
  'header.deal': 'Tqassim {n}',
  'header.gameOf': 'Logħba {n} minn {total}',
  'header.gameOfWithContract': 'Logħba {n} minn {total}: {contract}',

  // --- meld preview --------------------------------------------------------
  'preview.validSet': 'Grupp validu',
  'preview.validRun': 'Sekwenza valida',
  'preview.validMeld': 'Kombinazzjoni valida',
  'preview.notYet': 'Għadha mhix kombinazzjoni',
  'preview.points': '{shape} · {n} punti',
  'preview.pointsWithLaid': '{shape} · {n} + {laid} diġà mniżżla = {total} punti',
  'preview.meetsFloor': '{line} (jilħaq {n} ✓)',
  'preview.needsFloor': '{line} (jeħtieġ {n} ✗)',
  'preview.becauseOf': '{line} — {reason}',

  // --- discard -------------------------------------------------------------
  // A discard with a meld staged lays that meld first; this is what the
  // player is told when the server refuses it and the whole move is rolled
  // back. {reason} is the engine's own words for the refusal.
  'discard.meldRejected': '{reason} — ma ntrema xejn, il-karti tiegħek għadhom lesti.',

  // --- what the current selection would send -------------------------------
  // The client's own reason a control is not ready, next to the engine's own
  // `err.*` reasons rather than a second kind of message — a control greyed
  // out for "not your turn" and one greyed out for "you picked two, this
  // takes one" should read as the same kind of thing, not one of them looking
  // broken. See `fits` in `src/lib/drops.ts`.
  'sel.tooMany.1': 'Agħżel karta waħda biss',
  'sel.tooMany.n': 'Agħżel l-aktar {n} karti',
  'sel.needMore': 'Agħżel {n} karti',
  'sel.notThese': 'Dawk il-karti ma jistgħux jiġu hawn',
  'sel.needsCompany': "Dik il-karta għandha bżonn dawk ta' maġenbha",

  // --- what a module says happened -----------------------------------------
  // The first module-sent keys in this bundle, and deliberately few. Anything
  // a module sends is still legible without an entry here — `humanise` turns
  // `holdem.seat.stack` into "Stack" — so a key earns a line only when that
  // fallback loses something. These do: what each of them means lives in its
  // *params* (who won, how much, whose cards), and a key on its own has
  // nowhere to put them. Without the entry the player read "Winner", full
  // stop, at the end of a match they had just won.
  'status.winner': 'Rebħet minn {winners}',
  'holdem.status.pot': "{winners} rebaħ {amount} b'{hand}",
  'holdem.status.potUncontested': '{winners} rebaħ {amount} — kulħadd ieħor warrab',
  'holdem.status.shown': '{playerId} wera {value}',
  'holdem.prompt.waitingFor': 'Nistennew lil {playerId}',

  // How a rummy scoreboard was ordered. Both earn a line for the same reason
  // as the keys above: the number lives in the params, and `humanise` would
  // render "Deals Won" with nothing after it.
  'zolik.standing.dealsWon': 'Tqassimiet mirbuħa: {n}',
  'zolik.standing.inHand': "F'idek: {n}",

  // --- rounds ---------------------------------------------------------------
  // What a round is called, and what one did. Each of these carries its number
  // in the params, so `humanise` alone would render a label with nothing after
  // it.
  // An instruction, not a heading. "Next round" named the thing rather than
  // asking for it, and sat above "Waiting for 1" — which reads as though the
  // player is the one waiting, when the table is waiting for them.
  'round.continue': 'Ibda r-rawnd li jmiss',

  // --- the flash that announces a round or a match ending ------------------
  //
  // One line and one number, held for about two seconds. Worded generically on
  // purpose: which *kind* of ending it was is the module's own fact, printed
  // underneath, and a client that phrased "went gin" itself would be a client
  // that knows a game.
  'flash.roundWon': '{winners} ħadha',
  'flash.roundWonYou': 'Int ħadtha',
  'flash.roundDrawn': 'Ħadd ma ħadha',
  'flash.matchOver': 'Il-partita spiċċat',
  'flash.matchWon': '{winners} rebaħ',
  'flash.matchWonYou': 'Int rbaħt',
  'flash.matchDrawn': 'Ħadd ma rebaħ',
  'flash.nowOn': 'issa {total}',

  'zolik.round.deal': 'Tqassim',
  'zolik.round.cleanRun': 'Sekwenza waħda trid tkun mingħajr joker',
  'canasta.round.deal': 'Tqassim',
  'canasta.round.concealed': 'Ħareġ mistur',
  'canasta.round.exhausted': 'Il-mazz spiċċa',
  'canasta.round.meldCards': 'Karti mniżżla: {n}',
  'canasta.round.canastas': 'Canastas: {n}',
  'canasta.round.redThrees': 'Tlietiet ħomor: {n}',
  'canasta.round.goingOut': 'Ħruġ: {n}',
  'canasta.round.inHand': "Baqgħu f'idek: {n}",
  'holdem.round.hand': 'Id',
  'holdem.round.pot': 'Pot {n}',
  'holdem.round.uncontested': 'Kulħadd ieħor warrab',
  'seat.ready': 'Lest',



  // --- refusals the other three games raise ---------------------------------
  // Worded here for the same reason Žolíky's are: reasonText falls back to the
  // code, and OfferBar hands it the code as that fallback, so an unworded
  // refusal reaches a player as PILE_FROZEN in capitals. serverKeys.test.ts
  // now fails the build rather than letting the next one through.
  'err.SET_TOO_LARGE': "Grupp diġà għandu l-erba' kuluri kollha",
  'err.DISCARD_TAKEN_CARD_FORBIDDEN': 'Ma tistax tarmi l-karta li għadek kif ħadt — ilgħabha jew żommha',
  'err.CARD_DOES_NOT_FIT': 'Dik il-karta la taqbel fil-kulur u lanqas fil-valur',
  'err.SUIT_REQUIRED': 'Semmi l-kulur li jkompli',
  'err.MUST_ANSWER_DRAW_OR_TAKE': "Wieġeb b'sebgħa, jew ħu l-karti",
  'err.NOTHING_TO_DRAW': "Ma baqa' xejn x'tiġbed",
  'err.PILE_EMPTY': 'Il-munzell huwa vojt',
  'err.PILE_BLOCKED': 'Il-munzell huwa mblukkat — hemm tlieta sewda fuq',
  'err.PILE_FROZEN':
    "Il-munzell huwa ffriżat — għandek bżonn żewġ karti naturali tal-valur tal-karta ta' fuq",
  'err.TOP_CARD_UNUSABLE': "Ma tistax tuża l-karta ta' fuq",
  'err.MELD_CLOSED': 'Dik il-kombinazzjoni hija sħiħa u magħluqa',
  'err.MELD_TOO_SMALL': 'Kombinazzjoni għandha bżonn aktar karti minn hekk',
  'err.MELD_TOO_LARGE': 'Dik il-kombinazzjoni ma tistax tieħu aktar karti',
  'err.MELD_MIXED_RANKS': "Kull karta f'kombinazzjoni trid tkun tal-istess valur",
  'err.NOT_ENOUGH_NATURALS': 'Kombinazzjoni għandha bżonn aktar karti naturali milli jokers',
  'err.RANK_ALREADY_MELDED': "In-naħa tiegħek diġà għandha kombinazzjoni ta' dak il-valur",
  'err.NOT_YOUR_MELD': 'Dik il-kombinazzjoni hija tan-naħa l-oħra',
  'err.NO_SUCH_MELD': 'Dik il-kombinazzjoni mhijiex fuq il-mejda',
  'err.CANNOT_MELD_THREE': 'It-tlietiet qatt ma jitniżżlu',
  'err.CANNOT_DISCARD_RED_THREE': 'Tlieta ħamra ma tistax tintrema',
  'err.MUST_KEEP_A_CARD': 'Żomm mill-inqas karta waħda — hekk ma tistax tbattal idek',
  'err.MUST_MELD_FIRST': 'L-ewwel niżżel il-ftuħ tan-naħa tiegħek',
  'err.INITIAL_MELD_NOT_MET': 'L-ewwel tniżżil tiegħek għadu nieqes mill-punti',
  'err.CANNOT_GO_OUT_YET': "In-naħa tiegħek għandha bżonn canasta mlestija qabel ma tkun tista' toħroġ",
  'err.NOTHING_TO_CALL': "M'hemm ebda mħatra x'issejjaħ",
  'err.CANNOT_CHECK': "Ma tistax tiċċekkja — hemm imħatra x'twieġeb",
  'err.CANNOT_RAISE': 'Hawn ma tistax togħla',
  'err.RAISE_TOO_SMALL': "Żieda trid tkun mill-inqas daqs dik ta' qabel",
  'err.NOT_ENOUGH_CHIPS': "M'għandekx daqshekk ċipsijiet",
  'err.AMOUNT_REQUIRED': 'Għid kemm',
  'err.AMOUNT_NOT_A_NUMBER': 'Dak l-ammont mhuwiex numru',
  'err.SEAT_NOT_IN_HAND': "M'intix f'din l-id",
  'err.WRONG_RANK': 'Dik il-karta għandha l-valur ħażin għal dan',
  'err.MATCH_FULL': 'Il-mejda hija mimlija',
  'err.MATCH_ALREADY_STARTED': 'Il-partita diġà bdiet',
  'err.TOO_FEW_PLAYERS': "Għadhom m'hemmx biżżejjed plejers",
  'err.WRONG_PLAYER_COUNT': "Din il-logħba ma tistax tintlagħab b'daqshekk plejers",
  'err.NOT_THE_HOST': "Dak jista' jagħmlu biss il-ħost",
  'err.NO_LONGER_WAITING': "Il-mejda m'għadhiex tistenna",
  'err.WAITING_ROOM_UNAVAILABLE': 'Il-kamra tal-istennija mhijiex disponibbli',
  'err.SERVER_BUSY': "Is-server huwa mimli bħalissa — erġa' pprova fi ftit",


  // --- rules the engine has always enforced and never stated ----------------
  // Every one of these was found by the guardrail rather than by review: the
  // validator could refuse a player for it, and no sentence anywhere said so.
  // See server/internal/zolikmod/ruleindex_test.go.
  'zolik.rules.section.layoff': "Żieda ma' kombinazzjonijiet",
  'zolik.rules.pickup.obligation':
    "Qabel ma tkun niżżilt, karta meħuda mill-munzell tal-iskart trid tintuża fil-kombinazzjoni li biha tniżżel f'din id-dawra.",
  'zolik.rules.pickup.noReturn':
    "Karta li ħadt mill-munzell tal-iskart ma tistax terġa' tintrema fl-istess dawra — ilgħabha jew żommha.",
  'zolik.rules.wilds.setLimit': 'Grupp ma jistax ikollu aktar jokers milli karti naturali.',
  'zolik.rules.set.maxSize':
    "Grupp ma jistax ikollu aktar minn {n} karti — joker jimla kulur nieqes, ma jżidx ma' wieħed sħiħ.",
  'zolik.rules.run.maxLength':
    'Sekwenza ma tistax ikollha aktar minn {n} karti — l-ass taħt, it-tnax-il valur fuqu, u l-ass fil-quċċata.',
  'zolik.rules.run.aceBridge':
    "L-ass joqgħod fuq ir-Re jew taħt it-Tnejn, qatt bħala pont bejn iż-żewġ truf ta' sekwenza.",
  'zolik.rules.contracts.contribution':
    'Sakemm ma tkunx niżżilt, kull kombinazzjoni li tniżżel trid tkun waħda li l-kuntratt tat-tqassim għadu jitlob.',
  'zolik.rules.layoff.afterDown':
    "Ma tistax iżżid mal-kombinazzjonijiet ta' ħaddieħor sakemm ma tniżżilx il-kuntratt tiegħek stess.",
  'zolik.rules.layoff.runEnds': "Karta miżjuda ma' sekwenza trid tkompliha f'tarf wieħed jew fl-ieħor.",
  'zolik.rules.jokers.swap':
    "Joker f'kombinazzjoni fuq il-mejda jista' jinxtara lura bil-karta eżatta li jirrappreżenta.",
  'zolik.rules.jokers.reclaim.on':
    "Joker mixtri lura minn fuq il-mejda għandu jintlagħab f'kombinazzjoni fl-istess dawra — ma jistax jinżamm f'idek.",
  'zolik.rules.jokers.reclaim.off': "Joker mixtri lura minn fuq il-mejda jista' jinżamm f'idek.",
  'zolik.rules.deck.reshuffle':
    'Meta l-mazz jispiċċa, il-munzell tal-iskart jitħawwad u jsir il-mazz il-ġdid; jekk it-tnejn ikunu vojta, it-tqassim jispiċċa.',


  // --- what to do instead ---------------------------------------------------
  // The third layer of a refusal, after the reason and the rule. Sent by the
  // module because it is the only side that knows which card is owed and
  // which way out is on offer; see server/internal/zolikmod/remedy.go.
  'zolik.remedy.meldThePickup': 'Żid {card} mat-tniżżil tiegħek, jew ħassar il-ġbid.',
  'zolik.remedy.discardSomethingElse': "Armi karta oħra, jew ilgħab {card} f'din id-dawra.",
  'zolik.remedy.discardNotAJoker': 'Armi xi ħaġa oħra li mhijiex joker.',
  'zolik.remedy.finishOrUndoLayDown': 'Temm it-tniżżil tiegħek, jew ħudu lura.',
  'zolik.remedy.needMorePoints': "Għandek bżonn {n} punti oħra qabel ma tkun tista' tniżżel.",
  'zolik.remedy.layACleanRun': 'Niżżel sekwenza mingħajr joker fiha.',
  'zolik.remedy.playReclaimedJoker': "Ilgħab {card} f'kombinazzjoni, jew ħassar it-teħid tiegħu.",
  'zolik.remedy.goDownFirst': 'L-ewwel niżżel il-kombinazzjonijiet tiegħek stess.',
  'zolik.remedy.drawFirst': 'L-ewwel iġbed karta.',
  'zolik.remedy.drawFromStock': 'Iġbed mill-mazz — il-munzell tal-iskart jinfetaħ fir-rawnd {n}.',
  'zolik.remedy.drawFromStockEmpty': 'Iġbed mill-mazz minflok.',


  // --- keys whose meaning lives in their params -----------------------------
  // The shape fallback renders these with the number missing ("Stack", "Pot"),
  // which is why they earn a line where most keys do not.
  'header.contract': 'Jeħtieġ {sets} gruppi u {runs} sekwenzi',
  'header.contract.cleanRunOnly': 'Jeħtieġ sekwenza mingħajr joker',
  'header.round': 'Rawnd {n}',
  'header.deck': 'Mazz',
  'header.target': 'Mira',
  'header.suitInPlay': 'Kulur fil-logħob',
  'seat.cards': 'Karti',
  'zolik.offer.meld': 'Niżżel',
  'prompt.pickupMustBeMelded':
    "{value} ġie mill-munzell tal-iskart — irid jidħol fil-kombinazzjonijiet li bihom tniżżel f'din id-dawra.",
  'prompt.jokerMustBePlayed':
    "{value} ġie minn fuq il-mejda — irid jidħol f'kombinazzjoni qabel ma tkun tista' ttemm id-dawra tiegħek.",
  'prompt.initialMeld': 'Il-ftuħ tan-naħa tiegħek irid jilħaq {n} punti.',
  'prompt.canastasNeeded':
    "In-naħa tiegħek għad għandha bżonn {n} canastas oħra qabel ma tkun tista' toħroġ.",
  'prompt.mustDrawOrAnswerSeven': "Wieġeb b'sebgħa, jew iġbed {n} karti.",
  'prompt.chooseSuit': 'Agħżel il-kulur li jkompli',
  'prompt.skipPending': 'Id-dawra tiegħek tinqabeż',
  'status.lastDeal': 'It-tim {team} ġab {value}',
  'status.teamScore': 'Tim {team}: {value}',
  'canasta.offer.rank': 'Valur',
  'canasta.seat.teamScore': 'Skor tat-tim',
  'canasta.seat.canastas': 'Canastas',
  'holdem.header.pot': 'Pot',
  'holdem.header.street': 'Rawnd',
  'holdem.header.hand': 'Id',
  'holdem.header.handLimit': "Idejn b'kollox",
  'holdem.header.blinds': 'Blinds',
  'holdem.cost.call': 'biex issejjaħ',
  'holdem.cost.pot': 'fil-pot',
  'holdem.seat.stack': 'Munzell',
  'holdem.seat.bet': 'Imħatra',
  'holdem.prompt.yourAction': 'Imissek taġixxi',
  'holdem.prompt.raiseTo': 'Għolli sa',
  'zone.yourHand': 'Idek',
  'zone.opponentHand': 'Idu',
  'zone.drawPile': 'Mazz',
  'zone.discardPile': 'Munzell tal-iskart',
  'zone.melds': 'Kombinazzjonijiet',
  'zone.teamMelds': 'Il-kombinazzjonijiet tan-naħa tiegħek',
  'zone.redThrees': 'Tlietiet ħomor',
  'zone.board': 'Mejda',
  'verb.drawFromDeck': 'Iġbed',
  'verb.takeFromDiscard': 'Ħu mill-munzell',


  // --- the why sheet's own furniture ---------------------------------------
  // The three layers a refusal is explained in. Labels, not sentences: what
  // goes under each is sent by the server or worded above.
  'why.reason': 'Għaliex le',
  'why.rule': 'Ir-regola',
  'why.rules': 'Ir-regoli',
  'why.remedy': "X'tista' tagħmel",
  'why.readTheRules': 'Aqra r-regoli sħaħ →',
  'why.close': 'Agħlaq',
  'why.open': 'għaliex',

  // --- marks on a particular card ------------------------------------------
  // A mark is a refusal that has not happened yet: the rule is enforced at the
  // discard, which is the last possible moment to hear about it, so the card
  // says so while there is still a turn left to act on it.
  'zolik.badge.owedToMeld':
    "{card} ġie mill-munzell tal-iskart — irid jidħol fil-kombinazzjonijiet li bihom tniżżel f'din id-dawra.",
  'zolik.badge.jokerOwed':
    "{card} ġie minn fuq il-mejda — irid jidħol f'kombinazzjoni qabel ma tkun tista' ttemm id-dawra tiegħek.",

  // --- the legal notices ----------------------------------------------------
  // Only the furniture. The documents themselves are in `src/legal`, which is
  // a bundle of the same kind with a parity test of its own — prose that long
  // in a flat key map buries the keys this one exists for.
  //
  // The notice is five fragments rather than one sentence with two links glued
  // in, because Czech does not put the link where English does: "souhlasíš s
  // Podmínkami" inflects the noun the link is made of. Fragments let each
  // locale place and decline its own.
  'legal.terms': 'Termini',
  'legal.privacy': 'Privatezza',
  'legal.source': 'Kodiċi sors',
  'legal.updated': 'Verżjoni {version}',
  'legal.draft':
    "Abbozz — għadu mhux fis-seħħ. L-isem, il-pajjiż u l-indirizz ta' kuntatt tal-operatur għadhom iridu jimtlew.",
  'legal.notice.before': 'Billi tilgħab taqbel mat-',
  'legal.notice.terms': 'termini tal-użu',
  'legal.notice.between': '. Dak li jinżamm dwarek jinsab fl-',
  'legal.notice.privacy': 'avviż tal-privatezza',
  'legal.notice.after': '.',
  // --- gin rummy -------------------------------------------------------------
  'err.UPCARD_DECLINED': 'Diġà għaddejt fuq dik il-karta',
  'err.DEADWOOD_TOO_HIGH': 'Id-deadwood tiegħek huwa għoli wisq biex tħabbat',
  'err.CARD_DOES_NOT_EXTEND_MELD': 'Dik il-karta ma ttawwalx din il-kombinazzjoni',
  'ginrummy.rules.setup': 'Tħejjija',
  'ginrummy.rules.turn': 'Id-dawra tiegħek',
  'ginrummy.rules.melds': 'Kombinazzjonijiet',
  'ginrummy.rules.knocking': 'It-tħabbit',
  'ginrummy.rules.bigGin': 'Big gin',
  'ginrummy.rules.layoff': 'Iż-żieda',
  'ginrummy.rules.deadHand': 'L-id mejta',
  'ginrummy.rules.scoring': "L-iskor ta' id",
  'ginrummy.rules.match': 'Kif tirbaħ il-partita',
  'ginrummy.rules.lineBonuses': 'Bonusijiet fil-kont',
  'ginrummy.rules.deck': "Jintlagħab b'mazz ta' {value} karti.",
  'ginrummy.rules.deal': 'Kull plejer jieħu {value} karti.',
  'ginrummy.rules.upcard': "Karta oħra tinqaleb 'il fuq biex tibda l-munzell tal-iskart.",
  'ginrummy.rules.drawDiscard':
    'Fid-dawra tiegħek iġbed karta waħda — mill-mazz jew mill-munzell tal-iskart — u mbagħad armi waħda.',
  'ginrummy.rules.setsAndRuns':
    "Kombinazzjoni hija grupp ta' tliet jew erba' karti ta' valur wieħed, jew sekwenza ta' tlieta jew aktar tal-istess kulur.",
  'ginrummy.rules.aceLow': "L-ass dejjem baxx — m'hemmx sekwenza minn Reġina sa Ass.",
  'ginrummy.rules.knockLimit': "Tista' tħabbat malli d-deadwood tiegħek ikun {n} jew inqas.",
  'ginrummy.rules.oklahoma':
    "Il-limitu tat-tħabbit ta' din l-id jiġi stabbilit mill-valur tal-karta mikxufa.",
  'ginrummy.rules.gin': 'Deadwood żero huwa gin — l-aħjar tħabbita possibbli.',
  'ginrummy.rules.bigGinBonus':
    "Ħdax-il karta kollha f'kombinazzjonijiet, mingħajr ebda skart, huwa big gin, u jiswa {n} punti oħra.",
  'ginrummy.rules.layoffDescription':
    "Wara tħabbita li mhijiex gin, l-avversarju tiegħek jista' jżid id-deadwood tiegħu mal-kombinazzjonijiet tiegħek qabel ma jitqabblu l-idejn.",
  'ginrummy.rules.deadHandDescription':
    "Jekk il-mazz jinżel sal-aħħar żewġ karti u ħadd ma jkun ħabbat, l-id tkun mejta — ħadd ma jieħu punti, u l-istess dealer jerġa' jqassam.",
  'ginrummy.rules.undercut':
    "Jekk id-deadwood tal-avversarju ma jkunx ogħla minn tiegħek, jaqtagħlek taħt: jieħu d-differenza, flimkien ma' {n}.",
  'ginrummy.rules.ginBonus': "Gin iġib l-id sħiħa tal-avversarju tiegħek, flimkien ma' {n}.",
  'ginrummy.rules.target': 'L-ewwel wieħed li jaqbeż {n} punti wara li tispiċċa id jirbaħ il-partita.',
  'ginrummy.rules.shutout':
    'Il-bonus tal-partita jirdoppja għal {n} jekk it-telliefa qatt ma jkun ġab punt wieħed.',
  'ginrummy.rules.box': 'Kull id li rbaħt tiswa {n} punti fi tmiem il-partita.',
  'ginrummy.rules.gameBonus': 'Ir-rebħ tal-partita jiswa {n} punti oħra.',
  'ginrummy.fact.deadwood': 'deadwood {value}',
  'ginrummy.fact.discardCard': 'Armi {value}',
  'ginrummy.fact.meldCards': 'Fuq {value}',
  'ginrummy.header.hand': 'Id {n}',
  'ginrummy.round.kind': '{value}',
  'ginrummy.round.hand': 'Id',
  'ginrummy.seat.score': '{value}',
  'ginrummy.seat.dealer': 'Dealer',
  'ginrummy.status.knocked': "{playerId} ħabbat b'deadwood ta' {deadwood}",
  'ginrummy.status.gin': '{playerId} għamel gin',
  'ginrummy.status.lastHand': 'L-aħħar id: {winner} ({kind}, {delta} punti)',
  'ginrummy.offer.drawStock': 'Iġbed mill-mazz',
  'ginrummy.offer.drawDiscard': 'Iġbed mill-munzell tal-iskart',
  'ginrummy.offer.takeUpcard': 'Ħu l-karta mikxufa',
  'ginrummy.offer.passUpcard': 'Għaddi',
  'ginrummy.offer.discard': 'Armi',
  'ginrummy.offer.knock': 'Ħabbat',
  'ginrummy.offer.gin': 'Gin!',
  'ginrummy.offer.bigGin': 'Big gin!',
  'ginrummy.offer.layOff': 'Żid',
  'ginrummy.offer.finishLayoff': 'Iż-żieda lesta',
  'ginrummy.zone.knockerHand': "L-id ta' min ħabbat",
  'ginrummy.zone.melds': 'Kombinazzjonijiet',
  'ginrummy.prompt.upcardDecision': 'Ħu l-karta mikxufa, jew għaddi',
  'ginrummy.prompt.yourTurnDraw': 'Iġbed karta',
  'ginrummy.prompt.yourTurnDiscard': "Armi — jew ħabbat, jekk tista'",
  'ginrummy.prompt.layoff': 'Żid id-deadwood, jew temm',

  // --- rummy tiles -------------------------------------------------------------
  'err.TILE_NOT_IN_HAND': "Dik il-plakka mhijiex f'idek",
  'err.TILE_DOES_NOT_FIT': 'Dak ma joqgħodx hemm',
  'err.NO_SUCH_SET': 'Dik il-kombinazzjoni mhijiex fuq il-mejda',
  'err.INITIAL_MELD_ONLY':
    "Qabel l-ewwel tniżżil tiegħek tista' torganizza biss il-kombinazzjonijiet ġodda tiegħek stess",
  'err.TABLE_NOT_VALID': 'Il-mejda għadha mhijiex valida',
  'err.TRAY_NOT_EMPTY': "Għad għandek plakek maħlula x'tpoġġi",
  'err.NOTHING_PLAYED': 'Ilgħab mill-inqas plakka waħda qabel ma ttemm id-dawra tiegħek',
  'err.INITIAL_MELD_TOO_LOW': 'L-ewwel tniżżil tiegħek irid jiswa 30 punt jew aktar',
  'err.NOT_A_RUN': "Sekwenza biss tista' tinqasam",
  'err.BAD_SPLIT_POSITION': 'Hemmhekk din is-sekwenza ma tistax tinqasam',
  'err.NO_JOKER_IN_SET': "M'hemm ebda joker f'dik il-kombinazzjoni",
  'err.TILE_JOKER_SWAP_MISMATCH': 'Dik il-plakka mhijiex dak li jirrappreżenta l-joker',
  'rummytiles.rules.setup': 'Tħejjija',
  'rummytiles.rules.sets': 'Kombinazzjonijiet',
  'rummytiles.rules.initialMeld': 'L-ewwel tniżżil',
  'rummytiles.rules.turn': 'Id-dawra tiegħek',
  'rummytiles.rules.jokerTaking': 'Kif tieħu joker',
  'rummytiles.rules.ending': 'Kif ittemm rawnd',
  'rummytiles.rules.poolExhaustion': 'Jekk il-borża tispiċċa',
  'rummytiles.rules.match': 'Kif tirbaħ il-partita',
  'rummytiles.rules.tiles': "Jintlagħab b'{value} plakek.",
  'rummytiles.rules.dealCount': 'Kull plejer jieħu {value} plakek.',
  'rummytiles.rules.group':
    "Grupp huwa tlieta jew erba' plakek tal-istess numru, kull waħda ta' kulur differenti.",
  'rummytiles.rules.run': 'Sekwenza hija tliet numri jew aktar wara xulxin tal-istess kulur.',
  'rummytiles.rules.noWrap': 'It-13 ma jerġax jibda mill-1.',
  'rummytiles.rules.joker': 'Joker jirrappreżenta kwalunkwe plakka.',
  'rummytiles.rules.initialMeldDescription':
    "Sakemm ma tniżżilx {n} punti jew aktar f'dawra waħda, minn idek biss, ma tistax tmiss xejn minn dak li diġà jinsab fuq il-mejda.",
  'rummytiles.rules.turnDescription':
    "Ilgħab mill-inqas plakka waħda minn idek, organizza l-mejda kif trid, u temm b'kull kombinazzjoni fuq il-mejda valida.",
  'rummytiles.rules.noDiscard':
    "M'hemmx skart — jekk ma tistax tlesti dawra valida, minflok tiġbed plakka waħda.",
  'rummytiles.rules.jokerTakingDescription':
    "Joker fuq il-mejda tista' tieħdu billi tbiddlu mal-plakka li jirrappreżenta, minn idek — u trid tużah f'kombinazzjoni qabel ma tispiċċa d-dawra tiegħek.",
  'rummytiles.rules.goingOut':
    "L-ewwel plejer li jibqa' bla plakek jirbaħ ir-rawnd. Kulħadd ieħor jieħu l-valur negattiv ta' dak li jibqagħlu; ir-rebbieħ jieħu s-somma ta' dak li tilfu l-oħrajn kollha.",
  'rummytiles.rules.poolExhaustionLowestWins':
    "Jekk il-borża tispiċċa u ħadd ma jkun jista' jilgħab, ir-rawnd jintemm u jirbħu l-inqas valur f'idejh.",
  'rummytiles.rules.poolExhaustionNoWinner':
    "Jekk il-borża tispiċċa u ħadd ma jkun jista' jilgħab, ir-rawnd jintemm mingħajr rebbieħ — kull id sempliċement tingħadd.",
  'rummytiles.rules.target': 'L-ewwel wieħed li jaqbeż {n} punti wara li jispiċċa rawnd jirbaħ il-partita.',
  'rummytiles.rules.roundLimit': 'Il-partita tispiċċa wara {n} rawnds — jirbaħ l-ogħla skor.',
  'rummytiles.fact.setCards': '{value}',
  'rummytiles.header.pool': 'Borża {n}',
  'rummytiles.header.round': 'Rawnd {n}',
  'rummytiles.round.kind': '{value}',
  'rummytiles.round.round': 'Rawnd',
  'rummytiles.seat.score': '{value}',
  'rummytiles.seat.notOpened': 'Ma fetaħx',
  'rummytiles.status.lastRound': 'L-aħħar rawnd: {winner} ({kind})',
  'rummytiles.badge.invalid': 'Għadha mhix valida',
  'rummytiles.zone.pool': 'Borża',
  'rummytiles.zone.table': 'Mejda',
  'rummytiles.zone.tray': 'Xkaffa',
  'rummytiles.offer.place': 'Poġġi',
  'rummytiles.offer.addFromHand': 'Żid',
  'rummytiles.offer.addFromTray': 'Żid mix-xkaffa',
  'rummytiles.offer.take': 'Ħu',
  'rummytiles.offer.split': 'Aqsam',
  'rummytiles.offer.swapJoker': 'Ibdel il-joker',
  'rummytiles.offer.resetTurn': "Erġa' ibda d-dawra",
  'rummytiles.offer.commit': 'Lest',
  'rummytiles.offer.draw': 'Iġbed',
  'rummytiles.param.position': "Aqsam f'",

  // --- blackjack -----------------------------------------------------------
  //
  // The house rules are worded twice over — on and off — because a rule that
  // is *not* in force is still something a player has to be told. "No
  // surrender at this table" is information; silence is a guess.
  'err.BET_BELOW_MINIMUM': 'Dak huwa taħt il-minimu tal-mejda',
  'err.ALREADY_BET': 'L-imħatra tiegħek diġà tqiegħdet',
  'err.INSURANCE_CLOSED': "Bħalissa m'hemm ebda assigurazzjoni x'tieħu",
  'err.CANNOT_DOUBLE': 'Din l-id ma tistax tirdoppja',
  'err.CANNOT_SPLIT': 'Din l-id ma tistax tinqasam',
  'err.CANNOT_SURRENDER': 'Din l-id ma tistax tiġi ċeduta',

  'blackjack.rules.section.table': 'Il-mejda',
  'blackjack.rules.section.play': 'Kif tilgħab id',
  'blackjack.rules.section.dealer': 'Id-dealer',
  'blackjack.rules.section.end': 'Kif tispiċċa l-partita',
  'blackjack.rules.goal':
    "Egħleb lid-dealer mingħajr ma taqbeż wieħed u għoxrin. Min jaqbeż jitlef mill-ewwel, ikun x'ikun li jagħmel id-dealer wara.",
  // Counts are their own phrase rather than a number glued to a noun: one
  // deck and six decks inflect differently, and Czech inflects them again.
  'blackjack.rules.decks': 'Mazzi fiż-żarbun: {n}.',
  'blackjack.rules.stack': "Kull post joqgħod b'{n} ċipsijiet.",
  'blackjack.rules.minBet': 'Il-minimu tal-mejda huwa {n} ċipsijiet.',
  'blackjack.rules.faceUp':
    'Il-karti tal-plejers jitqassmu mikxufa; id-dealer iżomm karta waħda mgħottija sakemm kulħadd ikun lagħab.',
  'blackjack.rules.hitStand': 'Iġbed kemm karti trid, jew ieqaf fuq dak li għandek.',
  'blackjack.rules.aces': 'Ass jgħodd ħdax sakemm dak joqgħod, u wieħed meta ma joqgħodx.',
  'blackjack.rules.blackjack': "Ass ma' karta ta' valur għaxra, fl-ewwel żewġ karti, huwa blackjack.",
  'blackjack.rules.pays3to2': 'Blackjack iħallas 3:2.',
  'blackjack.rules.pays6to5': 'Blackjack iħallas 6:5.',
  'blackjack.rules.paysEven': "Blackjack iħallas wieħed ma' wieħed.",
  'blackjack.rules.double':
    "Fuq l-ewwel żewġ karti tiegħek tista' tirdoppja l-imħatra u tieħu eżattament karta waħda oħra.",
  'blackjack.rules.doubleAfterSplit': "Id li ħarġet minn qasma tista' tirdoppja wkoll.",
  'blackjack.rules.noDoubleAfterSplit': 'Id li ħarġet minn qasma ma tistax tirdoppja.',
  'blackjack.rules.split':
    "Żewġ karti tal-istess valur jistgħu jinqasmu f'idejn għalihom, kull waħda bl-imħatra tagħha — sa {n} darbiet, għal {hands} idejn b'kollox.",
  'blackjack.rules.noSplit': "F'din il-mejda l-pari ma jinqasmux.",
  'blackjack.rules.splitAces':
    'Assijiet maqsuma jieħdu karta waħda kull wieħed u mbagħad jieqfu, u wieħed u għoxrin li jsir hekk mhuwiex blackjack.',
  'blackjack.rules.surrender':
    "Tista' ċċedi l-ewwel id tiegħek għal nofs l-imħatra, ladarba d-dealer ikun iċċekkja għal blackjack.",
  'blackjack.rules.noSurrender': "F'din il-mejda l-idejn ma jistgħux jiġu ċeduti.",
  'blackjack.rules.dealerDraws': 'Id-dealer jiġbed sa sbatax u mbagħad jieqaf.',
  'blackjack.rules.hitsSoft17': "Id-dealer jiġbed fuq sbatax magħmula b'ass.",
  'blackjack.rules.standsSoft17': "Id-dealer jieqaf fuq sbatax magħmula b'ass.",
  'blackjack.rules.dealerPeeks':
    'Meta juri ass jew għaxra, id-dealer jiċċekkja għal blackjack qabel ma jilgħab xi ħadd.',
  'blackjack.rules.insurance':
    "Kontra ass tad-dealer tista' tassigura għal nofs l-imħatra tiegħek; iħallas 2:1 jekk id-dealer ikollu blackjack.",
  'blackjack.rules.noInsurance': "F'din il-mejda ma tiġix offruta assigurazzjoni.",
  'blackjack.rules.rounds': 'Fil-mejda jintlagħbu {n} rawnds.',
  'blackjack.rules.mostChipsWins': 'Min ikollu l-aktar ċipsijiet fl-aħħar jirbaħ il-partita.',
  'blackjack.rules.bustedOut':
    "Post li ma jkunx jista' jkopri aktar il-minimu ta' {n} joqgħod barra għall-bqija tal-partita.",

  'blackjack.zone.dealer': 'Dealer',
  'blackjack.zone.box': 'Id',
  'blackjack.zone.yourBox': 'Idek',
  'blackjack.zone.shoe': 'Żarbun',

  'blackjack.header.round': 'Rawnd {n} minn {of}',
  'blackjack.header.minBet': 'Minimu',
  'blackjack.header.decks': 'Mazzi',
  'blackjack.header.dealerTotal': 'Id-dealer juri {n}',
  'blackjack.header.dealerSoftTotal': 'Id-dealer juri {n} artab',

  'blackjack.seat.stack': 'Ċipsijiet',
  'blackjack.seat.bet': 'Imħatra',
  'blackjack.seat.insurance': 'Assigurazzjoni',
  'blackjack.seat.total': 'Total',
  'blackjack.seat.softTotal': 'Total artab',
  'blackjack.seat.out': 'Bla ċipsijiet',

  'blackjack.prompt.placeBet': 'Poġġi l-imħatra tiegħek',
  'blackjack.prompt.insurance': 'Assigurazzjoni?',
  'blackjack.prompt.yourMove': 'Imissek',
  'blackjack.prompt.waitingFor': 'Nistennew lil {playerId}',
  'blackjack.prompt.betAmount': 'Imħatra',

  'blackjack.offer.bet': 'Poġġi',
  'blackjack.offer.hit': 'Karta',
  'blackjack.offer.stand': 'Nieqaf',
  'blackjack.offer.double': 'Irdoppja',
  'blackjack.offer.split': 'Aqsam',
  'blackjack.offer.surrender': 'Ċedi',
  'blackjack.offer.insure': 'Ħu assigurazzjoni',
  'blackjack.offer.declineInsurance': 'Bla assigurazzjoni',

  'blackjack.fact.tableMinimum': 'minimu',
  'blackjack.fact.insuranceCost': 'għall-assigurazzjoni',
  'blackjack.fact.extraStake': 'biex tħatri',
  'blackjack.fact.surrenderReturn': 'lura',

  'blackjack.status.dealerBlackjack': 'Id-dealer kellu blackjack',
  'blackjack.status.dealerBust': "Id-dealer qabeż b'{n}",
  'blackjack.status.dealerStands': 'Id-dealer jieqaf fuq {n}',

  'blackjack.round.name': 'Rawnd',
  'blackjack.round.dealerTotal': 'Dealer {n}',
  'blackjack.round.dealerBust': 'Id-dealer qabeż ({n})',
  'blackjack.round.dealerBlackjack': 'Blackjack tad-dealer',
  'blackjack.round.outcome.blackjack': 'Blackjack',
  'blackjack.round.outcome.win': 'Rebħet',
  'blackjack.round.outcome.push': 'Indaqs',
  'blackjack.round.outcome.lose': 'Tilfet',
  'blackjack.round.outcome.bust': 'Qabeż',
  'blackjack.round.outcome.surrender': 'Ċeduta',

  'blackjack.badge.inPlay': 'Fil-logħob',
  'blackjack.badge.doubled': 'Irdoppjata',
  'blackjack.badge.split': 'Maqsuma',
  'blackjack.badge.blackjack': 'Blackjack',
  'blackjack.badge.bust': 'Qabeż',
  'blackjack.badge.won': 'Rebħet',
  'blackjack.badge.push': 'Indaqs',
  'blackjack.badge.lost': 'Tilfet',
  'blackjack.badge.surrendered': 'Ċeduta',

  'blackjack.unit.chips': 'ċipsijiet',

  // --- the settings screen --------------------------------------------------
  //
  // Hardcoded English until now, which was survivable while the only other
  // language was unreachable and faintly absurd once the language picker
  // itself lived on this screen: a player who cannot read "Settings" is
  // exactly the one who came here to change it.
  'settings.title': 'Issettjar',
  'settings.subtitle': 'Kif tidher int, u kif tidher il-mejda',
  'settings.face.heading': 'Wiċċek fuq il-mejda',
  'settings.face.account': 'Jinżamm mal-kont tiegħek, u għalhekk jiġi miegħek fuq apparat ieħor.',
  'settings.face.device': 'Jinżamm fuq dan l-apparat. Idħol biex teħdu miegħek.',
  'settings.skin.heading': 'Id-dehra tal-mejda',
  'settings.language.heading': 'Lingwa',
  'settings.language.status': 'Tinżamm fuq dan l-apparat.',
  'settings.language.auto': 'Awtomatika',
  'settings.language.auto.now': 'Issegwi l-apparat tiegħek — issa {language}',
  'settings.legal.heading': 'Il-kitba ż-żgħira',
  'settings.legal.status': "Ma' xiex qbilt billi lgħabt, u x'jinżamm dwarek.",
  'settings.signIn': 'Idħol',
  'settings.back': 'Lura',

  // A translated interface over an English notice is a worse lie than an
  // English interface, so the document says so itself rather than letting the
  // surrounding screen imply the text was written for this reader.
  'legal.untranslated':
    'Dan l-avviż għadu ma ġiex tradott għal-lingwa tiegħek. It-test bl-Ingliż hawn taħt huwa l-verżjoni li tapplika.',

  // --- the navigation bar's screen titles ------------------------------------
  //
  // These sit in `app/_layout.tsx`, above every screen, and were the last
  // English left after the settings screen was translated — a German player
  // reading "Einstellungen" under a bar that said "Settings". A translated app
  // with an untranslated chrome bar looks like a bug, not like a limit.
  'nav.home': 'Zolik',
  'nav.emailSignIn': 'Dħul bl-email',
  'nav.signingIn': 'Qed tidħol',
  'nav.usernameSignIn': "Dħul b'isem tal-utent",
  'nav.legacyAccount': 'Kont antik',
  'nav.guest': 'Mistieden',
  'nav.account': 'Kont',
  'nav.games': 'Logħob',
  'nav.table': 'Il-mejda tiegħek',
  'nav.join': "Ingħaqad ma' mejda",
  // The screen a shared invite link lands on while it resolves the code.
  'nav.joining': 'Qed tingħaqad',
  'nav.rules': 'Regoli',
  'nav.match': 'Partita',
  'nav.scoreTable': 'Tabella tal-iskor',
  'nav.stats': 'Statistika',
};
