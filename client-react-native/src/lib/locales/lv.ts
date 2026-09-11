/**
 * Latvian. Rummy vocabulary: grupa for a set, secība for a run, kombinācija for a meld, kavas for the deck, džokers for a joker.
 */

export const lv: Record<string, string> = {
  // --- engine error codes (rules.RulesErrorCode) ---------------------------
  'err.NOT_YOUR_TURN': 'Nav tava kārta',
  'err.WRONG_PHASE': 'Pašlaik nav iespējams',
  'err.MUST_DRAW_FIRST': 'Pavelc kārti, pirms izliec',
  'err.GAME_SUSPENDED': 'Spēle ir apturēta',
  'err.GAME_NOT_ACTIVE': 'Spēle nenotiek',
  // --- runtime refusals, which are not the rules saying no -----------------
  //
  // Both were reachable long before they were worded, and a player who met
  // one was shown the code in capitals or, worse, nothing at all: an action
  // sent while the socket was down simply vanished, which reads as a legal
  // move being silently refused.
  'err.MATCH_NOT_ACTIVE': 'Galds ir apturēts — gaidām, kad spēlētājs atkal pievienosies',
  'err.NOT_CONNECTED': 'Nav savienojuma ar galdu — atkal pieslēdzamies, pēc tam mēģini vēlreiz',
  // Why the "start the next round" control is greyed out. Without wording the
  // shell printed the code itself, so a player agreeing to go on was answered
  // with ALREADY_READY in capitals.
  'err.ALREADY_READY': 'Tu esi gatavs',
  'err.NOT_BETWEEN_ROUNDS': 'Raunds vēl turpinās',
  'err.NOT_AT_THIS_TABLE': 'Tu neesi pie šī galda',
  // Bringing a table back that the sweeper set aside. Refusals a player
  // can actually provoke: a link to a table that is gone, one that was
  // already picked up in another tab, and a table with other people at it.
  'err.MATCH_MOVED_ON': 'Galds ir pavirzījies tālāk — pārlādē lapu',
  'err.MATCH_NOT_ABANDONED': 'Šis galds negaida atsākšanu',
  'err.MATCH_NOT_FOUND': 'Šī galda vairs nav',
  'err.TABLE_HAS_OTHER_PLAYERS': 'Atsākt var tikai galdu, pie kura visi pārējie ir boti',
  'err.DISCARD_LOCKED': 'Izmešanas kaudze pagaidām ir slēgta',
  'err.DISCARD_PILE_EMPTY': 'Izmešanas kaudze ir tukša',
  'err.NO_CARDS_LEFT': 'Vairs nav kārtu, ko vilkt',
  'err.ROUND_REQ_NOT_MET': 'Vispirms izliec savu pirmo kombināciju',
  'err.NEED_CLEAN_RUN': 'Lai tevi uzskatītu par izlikušos, uz galda vajadzīga secība bez džokera',
  'err.INCOMPLETE_INITIAL_MELD': 'Pabeidz izlikšanu vai atsauc to, pirms izmet kārti',
  'err.DISCARD_CARD_NOT_MELDED': 'Paņemtajai kārtij jānonāk tavā kombinācijā',
  'err.JOKER_DISCARD_FORBIDDEN': 'Džokeru nedrīkst izmest',
  'err.NOTHING_TO_UNDO': 'Nav ko atsaukt',
  'err.NO_JOKER_IN_MELD': 'Šajā kombinācijā nav džokera',
  'err.JOKER_SWAP_MISMATCH': 'Šī kārts neieņem džokera vietu',
  'err.RECLAIMED_JOKER_NOT_MELDED': 'No galda paņemtais džokers šajā gājienā jāizspēlē kombinācijā',
  'err.RUN_TOO_LONG': 'Šī secība jau ir pilnā garumā',
  'err.WRONG_RUN_END': 'Šī kārts pagarina secības otru galu',
  'err.INVALID_MELD': 'Neviena kārts tavā rokā šeit neder',
  'err.CARD_NOT_IN_HAND': 'Šīs kārts tavā rokā nav',
  'err.MELD_BELOW_MINIMUM': 'Tavām kombinācijām vēl trūkst punktu, lai izliktu',
  'err.MELD_NO_CONTRIBUTION': 'Šī kombinācija nevirza tavu prasību uz priekšu',
  'err.TOO_MANY_WILDS': 'Pārāk daudz džokeru šajā kombinācijā',
  'err.ADJACENT_WILDS': 'Divi džokeri nedrīkst atrasties blakus',
  'err.ACE_BRIDGE': 'Dūzis nevar savienot kungu un divnieku',

  // --- contract phrasing ---------------------------------------------------
  // Built from the structured contract the server sends (sets/runs counts),
  // never from a deal-number lookup table.
  // Whole phrases per count rather than a number plus a pluralised noun:
  // Czech inflects the noun by count (skupina / skupiny / skupin) in a way no
  // "add an s" helper survives, and the counts that actually occur are 1-3.
  // A count outside that falls back to contract.sets.n.
  'contract.sets.1': 'Viena grupa',
  'contract.sets.2': 'Divas grupas',
  'contract.sets.3': 'Trīs grupas',
  'contract.sets.n': 'Grupas: {n}',
  'contract.runs.1': 'Viena secība',
  'contract.runs.2': 'Divas secības',
  'contract.runs.3': 'Trīs secības',
  'contract.runs.n': 'Secības: {n}',
  'contract.any': 'Jebkura derīga kombinācija',
  'contract.cleanRunOnly': 'Jebkurš grupu un secību sajaukums — vismaz vienai secībai jābūt bez džokera',
  'contract.cleanRunSuffix': '{base} — vienai secībai jābūt bez džokera',

  // --- written rules ---------------------------------------------------------
  // Full-sentence keys a "see the rules" screen renders, one per module,
  // resolved server-side against the table's actual variation and options —
  // see internal/module/rules.go and each game's rules.go. Shared section
  // titles first, then one block per module in the order the four games were
  // added.
  'zolik.rules.section.goal': 'Mērķis',
  'zolik.rules.section.setup': 'Sagatavošana',
  'zolik.rules.section.turn': 'Tavs gājiens',
  'zolik.rules.section.melding': 'Izlikšana',
  'zolik.rules.section.end': 'Kā beidzas mačs',
  'zolik.rules.goal':
    'Esi pirmais, kas iztukšo roku, izliekot derīgas grupas un secības, savācot pēc iespējas mazāk soda punktu kārtīs, kuras vēl turi, kad kāds cits iziet.',
  'zolik.rules.deal': 'Katrs spēlētājs saņem {n} kārtis.',
  'zolik.rules.meldShapes':
    'Grupa ir {set}+ vienādas vērtības kārtis; secība ir {run}+ pēc kārtas ejošas vienas krāsas kārtis.',
  'zolik.rules.turn.draw': 'Savā gājienā pavelc vienu kārti — no kavām vai no izmešanas kaudzes.',
  'zolik.rules.pickup.topOnly': 'No izmešanas kaudzes drīkst ņemt tikai augšējo kārti.',
  'zolik.rules.pickup.anyFromPile':
    'No izmešanas kaudzes drīkst ņemt jebkuru kārti kopā ar visu, kas atrodas virs tās.',
  'zolik.rules.pickup.locked': 'No izmešanas kaudzes nedrīkst vilkt pirms {n}. raunda.',
  'zolik.rules.pickup.open': 'Izmešanas kaudze ir atvērta no pirmā raunda.',
  'zolik.rules.turn.discard': 'Pabeidz gājienu, izmetot vienu kārti.',
  'zolik.rules.jokers.restricted':
    'Džokeru nekad nedrīkst izmest, izņemot gadījumu, kad tā ir tieši tā kārts, kas iztukšo tavu roku.',
  'zolik.rules.lead.rotate':
    'Pirmais gājiens katrā dalījumā pārvietojas par vienu vietu, neatkarīgi no tā, kurš uzvarēja.',
  'zolik.rules.lead.winner': 'Kurš iziet, tas sāk nākamo dalījumu.',
  'zolik.rules.meldFloor.on':
    'Tavai pirmajai izlikšanai kopā jādod vismaz {n} dabiskie punkti, lai tu būtu izlicies.',
  'zolik.rules.meldFloor.off': 'Pirmajai izlikšanai nav minimālās punktu vērtības.',
  'zolik.rules.cleanRun.on':
    'Vismaz vienai no tavām secībām jābūt pilnīgi bez džokera, lai tevi uzskatītu par izlikušos.',
  'zolik.rules.cleanRun.off': 'Tavas secības drīkst brīvi izmantot džokerus — nevienai nav jābūt bez tiem.',
  'zolik.rules.contracts.rotating':
    'Mačs ilgst {n} dalījumus, un katrs dalījums prasa savu grupu un secību kombināciju.',
  'zolik.rules.contracts.static':
    'Katrs dalījums prasa vienu un to pašu kombināciju: {sets} grupas un {runs} secības.',
  'zolik.rules.end.afterDeals': 'Mačs beidzas pēc {n} dalījumiem.',
  'zolik.rules.end.atScore': 'Dala tālāk, līdz kāds sasniedz {n} punktus — tad ir beigas.',

  'prsi.rules.section.goal': 'Mērķis',
  'prsi.rules.section.setup': 'Sagatavošana',
  'prsi.rules.section.turn': 'Tavs gājiens',
  'prsi.rules.section.special': 'Īpašās kārtis',
  'prsi.rules.section.end': 'Kā beidzas mačs',
  'prsi.rules.goal': 'Esi pirmais, kas izspēlē visas rokas kārtis.',
  'prsi.rules.deck': 'Spēlē ar {value} kāršu kavām (no septītnieka uz augšu).',
  'prsi.rules.deal': 'Katrs spēlētājs sāk ar {n} kārtīm.',
  'prsi.rules.turn.match':
    'Izspēlē kārti, kas atbilst augšējās kārts krāsai vai vērtībai — vai velc, ja nevari.',
  'prsi.rules.turn.draw': 'Vilkšana beidz tavu gājienu bez izspēles.',
  'prsi.rules.sevens':
    'Izspēlē 7, un nākamais spēlētājs velk divas kārtis, ja vien viņš neatbild ar savu septītnieku.',
  'prsi.rules.aces': 'Izspēlē dūzi, un nākamā spēlētāja gājiens tiek izlaists.',
  'prsi.rules.queens': 'Izspēlē dāmu un nosauc krāsu, kas turpinās.',
  'prsi.rules.end': 'Mačs beidzas brīdī, kad kāda roka ir tukša.',

  'canasta.rules.section.goal': 'Mērķis',
  'canasta.rules.section.setup': 'Sagatavošana',
  'canasta.rules.section.melding': 'Izlikšana',
  'canasta.rules.section.end': 'Kā beidzas mačs',
  'canasta.rules.goal': 'Spēlē pāros; pirmā puse, kas sasniedz {n} punktus, uzvar mačā.',
  'canasta.rules.deck': 'Spēlē ar {value} kārtīm — {decks} kavas plus džokeri.',
  'canasta.rules.deal': 'Katrs spēlētājs saņem {n} kārtis.',
  'canasta.rules.drawCount': 'Gājiena sākumā tu paņem {n} kārtis.',
  'canasta.rules.redThrees':
    'Sarkano trijnieku rokā uzreiz parāda, un tas dod bonusu — ja vien tava puse nekad nepabeidz kanastu, tad tas skaitās pret tevi.',
  'canasta.rules.canasta': 'Kanasta ir kombinācija no {n} vai vairāk vienādas vērtības kārtīm.',
  'canasta.rules.sequences': 'Kombinācija var būt arī secība: trīs vai vairāk vienas masts kārtis pēc kārtas, nekad ar džokeru starp tām.',
  'canasta.rules.samba': 'Secība no septiņām kārtīm ir samba, un tā ir {n} punktu vērta.',
  'canasta.rules.blackThreesGoOut':
    'Melns trijnieks bloķē kaudzi un ir {n} punktu vērts. Trīs vai četrus no tiem drīkst izlikt tieši no rokas, nekad ar džokeru starp tiem, un tikai kā gājienu, ar kuru tava puse iziet.',
  'canasta.rules.blackThreesNeverMeld':
    'Melnu trijnieku nekad neizliek. Izmests tas bloķē kaudzi, bet palicis rokā dalīšanas beigās maksā {n} punktus.',
  'canasta.rules.pileAlwaysFrozen': 'Izmešanas kaudze ir iesaldēta visu dalījumu: to vari paņemt tikai tad, ja augšējai kārtij pievieno divas dabiskās kārtis no rokas.',
  'canasta.rules.pileOntoMeld': 'Ja tavai pusei jau ir nepabeigta kombinācija augšējās kārts vērtībā, vari paņemt visu izmešanas kaudzi un pievienot tai šo kārti — pāris rokā nav vajadzīgs.',
  'canasta.rules.pileNoMeldCapture': 'Kombinācija, kas jau ir uz galda, izmešanas kaudzi paņemt nevar: lai to paņemtu, augšējai kārtij jāpievieno divas kārtis no paša rokas.',
  'canasta.rules.meldFloorBands':
    'Tavai pirmajai izlikšanai jāsasniedz punktu minimums, kas aug līdz ar tavu rezultātu: {negative} zem nulles, {low} līdz 1500, {mid} līdz 3000, {high} virs tā.',
  'canasta.rules.meldFloorBandsFive': 'Tavai pirmajai kombinācijai jāsasniedz punktu minimums, kas aug līdz ar tavu rezultātu: {negative} zem nulles, {low} līdz 1500, {mid} līdz 3000, {high} līdz 7000 un {top} virs tā.',
  'canasta.rules.oneCanastaToGoOut': 'Viena pabeigta kanasta ir pietiekama, lai tava puse izietu.',
  'canasta.rules.twoCanastasToGoOut':
    'Tavai pusei vajadzīgas divas pabeigtas kanastas, pirms tā drīkst iziet.',
  'canasta.rules.end': 'Dala tālāk, līdz viena puse pārsniedz {n} punktus — tad mačs ir galā.',

  'holdem.rules.section.goal': 'Mērķis',
  'holdem.rules.section.setup': 'Sagatavošana',
  'holdem.rules.section.betting': 'Likmes',
  'holdem.rules.section.end': 'Kā beidzas mačs',
  'holdem.rules.goal':
    'Iegūsti žetonus ar labāko roku atklāšanā vai paliekot vienīgajam spēlētājam dalījumā.',
  'holdem.rules.stack': 'Katra vieta sāk ar {n} žetoniem.',
  'holdem.rules.blinds': 'Mazā aklā likme ir {sb}, lielā — {bb}; abas liek pirms kāršu izdalīšanas.',
  'holdem.rules.streets': 'Liek četrās kārtās — pirms flopa un pēc flopa, tērna un rivera.',
  'holdem.rules.showdown': 'Tie, kas vēl ir spēlē, atklāj kārtis; labākā piecu kāršu roka paņem banku.',
  'holdem.rules.noLimit': 'Bez limita — jebkura likme drīkst sasniegt visu tavu kaudzīti.',
  'holdem.rules.lastPlayerStanding': 'Spēlē, līdz viena vieta tur visus žetonus.',
  'holdem.rules.mostChipsWins': 'Kam spēles beigās ir visvairāk žetonu, tas uzvar mačā.',
  'holdem.rules.handLimit': 'Spēle apstājas pēc {n} dalījumiem.',

  // --- header --------------------------------------------------------------
  'header.deal': 'Dalījums {n}',
  'header.gameOf': 'Spēle {n} no {total}',
  'header.gameOfWithContract': 'Spēle {n} no {total}: {contract}',

  // --- meld preview --------------------------------------------------------
  'preview.validSet': 'Derīga grupa',
  'preview.validRun': 'Derīga secība',
  'preview.validMeld': 'Derīga kombinācija',
  'preview.notYet': 'Vēl nav kombinācija',
  'preview.points': '{shape} · {n} punkti',
  'preview.pointsWithLaid': '{shape} · {n} + {laid} jau izlikti = {total} punkti',
  'preview.meetsFloor': '{line} (sasniedz {n} ✓)',
  'preview.needsFloor': '{line} (vajag {n} ✗)',
  'preview.becauseOf': '{line} — {reason}',

  // --- discard -------------------------------------------------------------
  // A discard with a meld staged lays that meld first; this is what the
  // player is told when the server refuses it and the whole move is rolled
  // back. {reason} is the engine's own words for the refusal.
  'discard.meldRejected': '{reason} — nekas netika izmests, tavas kārtis joprojām ir sagatavotas.',

  // --- what the current selection would send -------------------------------
  // The client's own reason a control is not ready, next to the engine's own
  // `err.*` reasons rather than a second kind of message — a control greyed
  // out for "not your turn" and one greyed out for "you picked two, this
  // takes one" should read as the same kind of thing, not one of them looking
  // broken. See `fits` in `src/lib/drops.ts`.
  'sel.tooMany.1': 'Izvēlies tikai vienu kārti',
  'sel.tooMany.n': 'Izvēlies ne vairāk kā {n} kārtis',
  'sel.needMore': 'Izvēlies kārtis: {n}',
  'sel.notThese': 'Šīs kārtis šeit nevar nonākt',
  'sel.needsCompany': 'Šai kārtij vajadzīgas blakus esošās',

  // --- what a module says happened -----------------------------------------
  // The first module-sent keys in this bundle, and deliberately few. Anything
  // a module sends is still legible without an entry here — `humanise` turns
  // `holdem.seat.stack` into "Stack" — so a key earns a line only when that
  // fallback loses something. These do: what each of them means lives in its
  // *params* (who won, how much, whose cards), and a key on its own has
  // nowhere to put them. Without the entry the player read "Winner", full
  // stop, at the end of a match they had just won.
  'status.winner': 'Uzvarēja {winners}',
  'holdem.status.pot': '{winners} uzvarēja {amount} ar {hand}',
  'holdem.status.potUncontested': '{winners} uzvarēja {amount} — visi pārējie atmeta',
  'holdem.status.shown': '{playerId} parādīja {value}',
  'holdem.prompt.waitingFor': 'Gaidām {playerId}',

  // How a rummy scoreboard was ordered. Both earn a line for the same reason
  // as the keys above: the number lives in the params, and `humanise` would
  // render "Deals Won" with nothing after it.
  'zolik.standing.dealsWon': 'Uzvarēti dalījumi: {n}',
  'zolik.standing.inHand': 'Rokā: {n}',

  // --- rounds ---------------------------------------------------------------
  // What a round is called, and what one did. Each of these carries its number
  // in the params, so `humanise` alone would render a label with nothing after
  // it.
  // An instruction, not a heading. "Next round" named the thing rather than
  // asking for it, and sat above "Waiting for 1" — which reads as though the
  // player is the one waiting, when the table is waiting for them.
  'round.continue': 'Sākt nākamo raundu',

  // --- the flash that announces a round or a match ending ------------------
  //
  // One line and one number, held for about two seconds. Worded generically on
  // purpose: which *kind* of ending it was is the module's own fact, printed
  // underneath, and a client that phrased "went gin" itself would be a client
  // that knows a game.
  'flash.roundWon': '{winners} to paņēma',
  'flash.roundWonYou': 'Tu to paņēmi',
  'flash.roundDrawn': 'Neviens to nepaņēma',
  'flash.matchOver': 'Mačs beidzies',
  'flash.matchWon': '{winners} uzvarēja',
  'flash.matchWonYou': 'Tu uzvarēji',
  'flash.matchDrawn': 'Neviens neuzvarēja',
  'flash.nowOn': 'tagad {total}',

  'zolik.round.deal': 'Dalījums',
  'zolik.round.cleanRun': 'Vienai secībai jābūt bez džokera',
  'canasta.round.deal': 'Dalījums',
  'canasta.round.concealed': 'Izgāja slēpti',
  'canasta.round.exhausted': 'Kavas beidzās',
  'canasta.round.meldCards': 'Izliktās kārtis: {n}',
  'canasta.round.canastas': 'Kanastas: {n}',
  'canasta.round.redThrees': 'Sarkanie trijnieki: {n}',
  'canasta.round.goingOut': 'Iziešana: {n}',
  'canasta.round.inHand': 'Palika rokā: {n}',
  'holdem.round.hand': 'Roka',
  'holdem.round.pot': 'Banka {n}',
  'holdem.round.uncontested': 'Visi pārējie atmeta',
  'seat.ready': 'Gatavs',
  'zolik.seat.contractMet': 'Līgums izpildīts',
  'results.you': '(tu)',



  // --- refusals the other three games raise ---------------------------------
  // Worded here for the same reason Žolíky's are: reasonText falls back to the
  // code, and OfferBar hands it the code as that fallback, so an unworded
  // refusal reaches a player as PILE_FROZEN in capitals. serverKeys.test.ts
  // now fails the build rather than letting the next one through.
  'err.SET_TOO_LARGE': 'Grupā jau ir visas četras krāsas',
  'err.DISCARD_TAKEN_CARD_FORBIDDEN': 'Tu nevari izmest kārti, kuru tikko paņēmi — izspēlē to vai paturi',
  'err.CARD_DOES_NOT_FIT': 'Šī kārts neatbilst ne krāsai, ne vērtībai',
  'err.SUIT_REQUIRED': 'Nosauc krāsu, kas turpinās',
  'err.MUST_ANSWER_DRAW_OR_TAKE': 'Atbildi ar septītnieku vai paņem kārtis',
  'err.NOTHING_TO_DRAW': 'Vairs nav ko vilkt',
  'err.PILE_EMPTY': 'Kaudze ir tukša',
  'err.PILE_BLOCKED': 'Kaudze ir bloķēta — virspusē guļ melns trijnieks',
  'err.PILE_FROZEN': 'Kaudze ir iesaldēta — tev vajadzīgas divas dabiskas augšējās kārts vērtības kārtis',
  'err.MELD_CAPTURE_NOT_ALLOWED': 'Šajā spēlē kombinācija uz galda kaudzi paņemt nevar — vajadzīgas divas kārtis no rokas',
  'err.TOP_CARD_UNUSABLE': 'Tu nevari izmantot augšējo kārti',
  'err.MELD_CLOSED': 'Šī kombinācija ir pilna un slēgta',
  'err.MELD_TOO_SMALL': 'Kombinācijai vajag vairāk kāršu nekā tik',
  'err.MELD_TOO_LARGE': 'Šī kombinācija vairs nevar uzņemt kārtis',
  'err.MELD_MIXED_RANKS': 'Visām kombinācijas kārtīm jābūt vienādas vērtības',
  'err.SEQUENCE_NO_WILDS': 'Secībā nedrīkst būt džokeri',
  'err.SEQUENCE_NEEDS_ONE_SUIT': 'Visām secības kārtīm jābūt vienā mastā',
  'err.RUN_NOT_CONSECUTIVE': 'Secībai jāiet pēc kārtas, bez pārtraukumiem',
  'err.NOT_ENOUGH_NATURALS': 'Kombinācijai vajag vairāk dabisko kāršu nekā džokeru',
  'err.RANK_ALREADY_MELDED': 'Tavai pusei jau ir šādas vērtības kombinācija',
  'err.NOT_YOUR_MELD': 'Šī kombinācija pieder pretinieku pusei',
  'err.NO_SUCH_MELD': 'Šīs kombinācijas uz galda nav',
  'err.CANNOT_MELD_THREE': 'Trijniekus nekad neizliek',
  'err.BLACK_THREE_GO_OUT_ONLY': 'Melnus trijniekus izliek tikai kā gājienu, kas iztukšo tavu roku',
  'err.CANNOT_DISCARD_RED_THREE': 'Sarkano trijnieku nedrīkst izmest',
  'err.MUST_KEEP_A_CARD': 'Paturi vismaz vienu kārti — tā tu roku iztukšot nevari',
  'err.MUST_MELD_FIRST': 'Vispirms izliec savas puses pirmo kombināciju',
  'err.INITIAL_MELD_NOT_MET': 'Tavai pirmajai izlikšanai vēl trūkst punktu',
  'err.CANNOT_GO_OUT_YET': 'Tavai pusei vajadzīga pabeigta kanasta, pirms tā var iziet',
  'err.NOTHING_TO_CALL': 'Nav likmes, ko atbildēt',
  'err.CANNOT_CHECK': 'Tu nevari čekot — ir likme, uz kuru jāatbild',
  'err.CANNOT_RAISE': 'Šeit tu nevari paaugstināt',
  'err.RAISE_TOO_SMALL': 'Paaugstinājumam jābūt vismaz tikpat lielam kā iepriekšējam',
  'err.NOT_ENOUGH_CHIPS': 'Tev nav tik daudz žetonu',
  'err.AMOUNT_REQUIRED': 'Pasaki, cik',
  'err.AMOUNT_NOT_A_NUMBER': 'Šī summa nav skaitlis',
  'err.SEAT_NOT_IN_HAND': 'Tu nepiedalies šajā dalījumā',
  'err.WRONG_RANK': 'Šai kārtij šim nolūkam ir nepareiza vērtība',
  'err.MATCH_FULL': 'Galds ir pilns',
  'err.MATCH_ALREADY_STARTED': 'Mačs jau ir sācies',
  'err.TOO_FEW_PLAYERS': 'Spēlētāju vēl nepietiek',
  'err.WRONG_PLAYER_COUNT': 'Šo spēli nevar spēlēt ar tik daudz spēlētājiem',
  'err.NOT_THE_HOST': 'To var izdarīt tikai saimnieks',
  'err.BAD_SEATING': 'Šis vietu izkārtojums neatbilst tiem, kas ir pie galda',
  'err.NO_LONGER_WAITING': 'Galds vairs negaida',
  'err.WAITING_ROOM_UNAVAILABLE': 'Uzgaidāmā telpa nav pieejama',
  'err.SERVER_BUSY': 'Serveris pašlaik ir pilns — pamēģini pēc brīža',


  // --- rules the engine has always enforced and never stated ----------------
  // Every one of these was found by the guardrail rather than by review: the
  // validator could refuse a player for it, and no sentence anywhere said so.
  // See server/internal/zolikmod/ruleindex_test.go.
  'zolik.rules.section.layoff': 'Pievienošana kombinācijām',
  'zolik.rules.pickup.obligation':
    'Kamēr neesi izlicies, no izmešanas kaudzes paņemtā kārts jāizmanto tajā kombinācijā, ar kuru šajā gājienā izliecies.',
  'zolik.rules.pickup.noReturn':
    'No izmešanas kaudzes paņemtu kārti tajā pašā gājienā izmest atkal nedrīkst — izspēlē to vai paturi.',
  'zolik.rules.wilds.setLimit': 'Grupā nedrīkst būt vairāk džokeru nekā dabisko kāršu.',
  'zolik.rules.set.maxSize':
    'Grupā nedrīkst būt vairāk par {n} kārtīm — džokers aizstāj trūkstošo krāsu, tas nepapildina jau pilnu grupu.',
  'zolik.rules.run.maxLength':
    'Secībā nedrīkst būt vairāk par {n} kārtīm — dūzis apakšā, divpadsmit vērtības virs tā un dūzis augšā.',
  'zolik.rules.run.aceBridge':
    'Dūzis stāv virs kunga vai zem divnieka, nekad kā tilts starp abiem secības galiem.',
  'zolik.rules.contracts.contribution':
    'Kamēr neesi izlicies, katrai izliktajai kombinācijai jābūt tādai, kādu dalījuma līgums vēl prasa.',
  'zolik.rules.layoff.afterDown':
    'Svešām kombinācijām neko pievienot nedrīksti, kamēr neesi izlicis savu līgumu.',
  'zolik.rules.layoff.runEnds': 'Secībai pievienotai kārtij tā jāturpina vienā vai otrā galā.',
  'zolik.rules.jokers.swap':
    'Uz galda esošas kombinācijas džokeru var izpirkt tieši ar to kārti, kuru tas aizstāj.',
  'zolik.rules.jokers.reclaim.on':
    'No galda izpirkts džokers tajā pašā gājienā jāizspēlē kombinācijā — tas nedrīkst palikt rokā.',
  'zolik.rules.jokers.reclaim.off': 'No galda izpirkts džokers drīkst palikt rokā.',
  'zolik.rules.deck.reshuffle':
    'Kad kavas beidzas, izmešanas kaudzi sajauc un tā kļūst par jaunām kavām; ja abas ir tukšas, dalījums beidzas.',


  // --- what to do instead ---------------------------------------------------
  // The third layer of a refusal, after the reason and the rule. Sent by the
  // module because it is the only side that knows which card is owed and
  // which way out is on offer; see server/internal/zolikmod/remedy.go.
  'zolik.remedy.meldThePickup': 'Pievieno {card} savai izlikšanai vai atsauc paņemšanu.',
  'zolik.remedy.discardSomethingElse': 'Izmet citu kārti vai izspēlē {card} šajā gājienā.',
  'zolik.remedy.discardNotAJoker': 'Izmet kaut ko citu, ne džokeru.',
  'zolik.remedy.finishOrUndoLayDown': 'Pabeidz izlikšanu vai paņem to atpakaļ.',
  'zolik.remedy.needMorePoints': 'Tev vajag vēl {n} punktus, lai varētu izlikt.',
  'zolik.remedy.layACleanRun': 'Izliec secību bez džokera tajā.',
  'zolik.remedy.playReclaimedJoker': 'Izspēlē {card} kombinācijā vai atsauc tās paņemšanu.',
  'zolik.remedy.goDownFirst': 'Vispirms izliec savas kombinācijas.',
  'zolik.remedy.drawFirst': 'Vispirms pavelc kārti.',
  'zolik.remedy.drawFromStock': 'Velc no kavām — izmešanas kaudze atveras {n}. raundā.',
  'zolik.remedy.drawFromStockEmpty': 'Velc labāk no kavām.',


  // --- keys whose meaning lives in their params -----------------------------
  // The shape fallback renders these with the number missing ("Stack", "Pot"),
  // which is why they earn a line where most keys do not.
  'header.contract': 'Prasa {sets} grupas un {runs} secības',
  'header.contract.cleanRunOnly': 'Prasa secību bez džokera',
  'header.round': 'Raunds {n}',
  'header.deck': 'Kavas',
  'header.target': 'Mērķis',
  'header.suitInPlay': 'Krāsa spēlē',
  'seat.cards': 'Kārtis',
  'zolik.offer.meld': 'Izliec',
  'prompt.pickupMustBeMelded':
    '{value} nāca no izmešanas kaudzes — tai jānonāk kombinācijās, ar kurām šajā gājienā izliecies.',
  'prompt.jokerMustBePlayed': '{value} nāca no galda — tai jānonāk kombinācijā, pirms vari beigt gājienu.',
  'prompt.initialMeld': 'Tavas puses pirmajai kombinācijai jāsasniedz {n} punkti.',
  'prompt.canastasNeeded': 'Tavai pusei trūkst vēl {n} kanastu, lai varētu iziet.',
  'prompt.mustDrawOrAnswerSeven': 'Atbildi ar septītnieku vai pavelc {n} kārtis.',
  'prompt.chooseSuit': 'Izvēlies krāsu, kas turpinās',
  'prompt.skipPending': 'Tavs gājiens tiek izlaists',
  'status.lastDeal': 'Komanda {team} guva {value}',
  'status.teamScore': 'Komanda {team}: {value}',
  'canasta.offer.rank': 'Vērtība',
  'canasta.offer.sequence': 'Secība',
  'badge.naturalCanasta': 'Tīra kanasta',
  'badge.mixedCanasta': 'Netīra kanasta',
  'badge.samba': 'Samba',
  'badge.cleanRun': 'Tīra secība',
  'canasta.seat.teamScore': 'Komandas punkti',
  'canasta.seat.canastas': 'Kanastas',
  'holdem.header.pot': 'Banka',
  'holdem.header.street': 'Kārta',
  'holdem.header.hand': 'Roka',
  'holdem.header.handLimit': 'Roku kopā',
  'holdem.header.blinds': 'Aklās likmes',
  'holdem.cost.call': 'lai atbildētu',
  'holdem.cost.pot': 'bankā',
  'holdem.seat.stack': 'Kaudzīte',
  'holdem.seat.bet': 'Likme',
  'holdem.prompt.yourAction': 'Tava kārta rīkoties',
  'holdem.prompt.raiseTo': 'Paaugstināt līdz',
  'holdem.quick.halfPot': '½ Banka',
  'holdem.quick.pot': 'Banka',
  'holdem.quick.allIn': 'All-in',
  'zone.yourHand': 'Tava roka',
  'zone.opponentHand': 'Pretinieka roka',
  'zone.drawPile': 'Kavas',
  'zone.discardPile': 'Izmešanas kaudze',
  'zone.melds': 'Kombinācijas',
  'zone.teamMelds': 'Tavas puses kombinācijas',
  'zone.opponentMelds': 'Pretinieka puses kombinācijas',
  'zone.redThrees': 'Sarkanie trijnieki',
  'zone.board': 'Galds',
  'verb.drawFromDeck': 'Velc',
  'verb.takeFromDiscard': 'Ņem no kaudzes',


  // --- the why sheet's own furniture ---------------------------------------
  // The three layers a refusal is explained in. Labels, not sentences: what
  // goes under each is sent by the server or worded above.
  'why.reason': 'Kāpēc nē',
  'why.rule': 'Noteikums',
  'why.rules': 'Noteikumi',
  'why.remedy': 'Ko tu vari darīt',
  'why.readTheRules': 'Lasīt visus noteikumus →',
  'why.close': 'Aizvērt',
  'why.open': 'kāpēc',

  // --- marks on a particular card ------------------------------------------
  // A mark is a refusal that has not happened yet: the rule is enforced at the
  // discard, which is the last possible moment to hear about it, so the card
  // says so while there is still a turn left to act on it.
  'zolik.badge.owedToMeld':
    '{card} nāca no izmešanas kaudzes — tai jānonāk kombinācijās, ar kurām šajā gājienā izliecies.',
  'zolik.badge.jokerOwed': '{card} nāca no galda — tai jānonāk kombinācijā, pirms vari beigt gājienu.',

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
  'legal.terms': 'Noteikumi',
  // The document's own name, used as the heading above it. Kept apart from
  // `legal.notice.terms`, which is the same words inflected to sit inside a
  // sentence — a heading in the instrumental case reads as a mistake in every
  // Slavic language.
  'legal.terms.title': 'Lietošanas noteikumi',
  'legal.privacy.title': 'Privātuma paziņojums',
  'legal.privacy': 'Privātums',
  'legal.source': 'Pirmkods',
  'legal.updated': 'Versija {version}',
  'legal.draft': 'Projekts — vēl nav spēkā. Operatora nosaukums, valsts un kontaktadrese vēl jāaizpilda.',
  'legal.notice.before': 'Spēlējot tu piekrīti ',
  'legal.notice.terms': 'lietošanas noteikumiem',
  'legal.notice.between': '. Kas par tevi tiek glabāts, ir aprakstīts ',
  'legal.notice.privacy': 'privātuma paziņojumā',
  'legal.notice.after': '.',
  // --- gin rummy -------------------------------------------------------------
  'err.UPCARD_DECLINED': 'Tu jau atteicies no šīs kārts',
  'err.DEADWOOD_TOO_HIGH': 'Tavs deadwood ir pārāk liels, lai klauvētu',
  'err.CARD_DOES_NOT_EXTEND_MELD': 'Šī kārts šo kombināciju nepagarina',
  'ginrummy.rules.setup': 'Sagatavošana',
  'ginrummy.rules.turn': 'Tavs gājiens',
  'ginrummy.rules.melds': 'Kombinācijas',
  'ginrummy.rules.knocking': 'Klauvēšana',
  'ginrummy.rules.bigGin': 'Big gin',
  'ginrummy.rules.layoff': 'Pievienošana',
  'ginrummy.rules.deadHand': 'Beigts dalījums',
  'ginrummy.rules.scoring': 'Dalījuma punkti',
  'ginrummy.rules.match': 'Mača uzvarēšana',
  'ginrummy.rules.lineBonuses': 'Bonusi kopsavilkumā',
  'ginrummy.rules.deck': 'Spēlē ar {value} kāršu kavām.',
  'ginrummy.rules.deal': 'Katrs spēlētājs saņem {value} kārtis.',
  'ginrummy.rules.upcard': 'Vēl vienu kārti apgriež ar attēlu uz augšu, un tā sāk izmešanas kaudzi.',
  'ginrummy.rules.drawDiscard':
    'Savā gājienā pavelc vienu kārti — no kavām vai no izmešanas kaudzes — un tad vienu izmet.',
  'ginrummy.rules.setsAndRuns':
    'Kombinācija ir trīs vai četru vienas vērtības kāršu grupa vai trīs un vairāk vienas krāsas kāršu secība.',
  'ginrummy.rules.aceLow': 'Dūzis vienmēr ir zems — secības no dāmas līdz dūzim nav.',
  'ginrummy.rules.knockLimit': 'Tu drīksti klauvēt, tiklīdz tavs deadwood ir {n} vai mazāk.',
  'ginrummy.rules.oklahoma': 'Šī dalījuma klauvēšanas robežu nosaka apgrieztās kārts vērtība.',
  'ginrummy.rules.gin': 'Nulles deadwood ir džins — labākā iespējamā klauvēšana.',
  'ginrummy.rules.bigGinBonus':
    'Vienpadsmit kārtis, visas kombinācijās, bez jebkādas izmešanas, ir big gin un dod vēl {n} punktus.',
  'ginrummy.rules.layoffDescription':
    'Pēc klauvēšanas, kas nav džins, pretinieks drīkst savu deadwood pievienot tavām kombinācijām, pirms rokas tiek salīdzinātas.',
  'ginrummy.rules.deadHandDescription':
    'Ja kavās paliek pēdējās divas kārtis un neviens nav klauvējis, dalījums ir beigts — neviens nesaņem punktus, un tas pats dalītājs dala vēlreiz.',
  'ginrummy.rules.undercut':
    'Ja pretinieka deadwood nav lielāks par tavējo, viņš tevi nogriež: saņem starpību plus {n}.',
  'ginrummy.rules.ginBonus': 'Džins dod visu pretinieka roku plus {n}.',
  'ginrummy.rules.target': 'Kurš pēc dalījuma beigām pirmais pārsniedz {n} punktus, uzvar mačā.',
  'ginrummy.rules.shutout': 'Mača bonuss dubultojas līdz {n}, ja zaudētājs nav guvis nevienu punktu.',
  'ginrummy.rules.box': 'Katrs uzvarētais dalījums mača beigās ir {n} punktu vērts.',
  'ginrummy.rules.gameBonus': 'Uzvara mačā dod vēl {n} punktus.',
  'ginrummy.fact.deadwood': 'deadwood {value}',
  'ginrummy.fact.discardCard': 'Izmet {value}',
  'ginrummy.fact.meldCards': 'Pie {value}',
  'ginrummy.header.hand': 'Roka {n}',
  'ginrummy.round.kind': '{value}',
  'ginrummy.round.hand': 'Roka',
  'ginrummy.seat.score': '{value}',
  'ginrummy.seat.dealer': 'Dalītājs',
  'ginrummy.status.knocked': '{playerId} klauvēja ar deadwood {deadwood}',
  'ginrummy.status.gin': '{playerId} izdarīja džinu',
  'ginrummy.status.lastHand': 'Pēdējā roka: {winner} ({kind}, {delta} punkti)',
  'ginrummy.offer.drawStock': 'Velc no kavām',
  'ginrummy.offer.drawDiscard': 'Velc no izmešanas kaudzes',
  'ginrummy.offer.takeUpcard': 'Ņem apgriezto kārti',
  'ginrummy.offer.passUpcard': 'Garām',
  'ginrummy.offer.discard': 'Izmet',
  'ginrummy.offer.knock': 'Klauvē',
  'ginrummy.offer.gin': 'Džins!',
  'ginrummy.offer.bigGin': 'Big gin!',
  'ginrummy.offer.layOff': 'Pievieno',
  'ginrummy.offer.finishLayoff': 'Pievienošana pabeigta',
  'ginrummy.zone.knockerHand': 'Klauvētāja roka',
  'ginrummy.zone.melds': 'Kombinācijas',
  'ginrummy.prompt.upcardDecision': 'Ņem apgriezto kārti vai laid garām',
  'ginrummy.prompt.yourTurnDraw': 'Pavelc kārti',
  'ginrummy.prompt.yourTurnDiscard': 'Izmet — vai klauvē, ja vari',
  'ginrummy.prompt.layoff': 'Pievieno deadwood vai pabeidz',

  // --- rummy tiles -------------------------------------------------------------
  'err.TILE_NOT_IN_HAND': 'Šī kauliņa tavā rokā nav',
  'err.TILE_DOES_NOT_FIT': 'Tur tas neder',
  'err.NO_SUCH_SET': 'Šīs kombinācijas uz galda nav',
  'err.INITIAL_MELD_ONLY': 'Pirms pirmās izlikšanas tu drīksti pārkārtot tikai savas jaunās kombinācijas',
  'err.TABLE_NOT_VALID': 'Galds vēl nav derīgs',
  'err.TRAY_NOT_EMPTY': 'Tev vēl ir brīvi kauliņi, ko novietot',
  'err.NOTHING_PLAYED': 'Izspēlē vismaz vienu kauliņu, pirms beidz gājienu',
  'err.INITIAL_MELD_TOO_LOW': 'Tavai pirmajai izlikšanai jābūt vismaz 30 punktu vērtai',
  'err.NOT_A_RUN': 'Sadalīt var tikai secību',
  'err.BAD_SPLIT_POSITION': 'Tajā vietā šo secību sadalīt nevar',
  'err.NO_JOKER_IN_SET': 'Šajā kombinācijā džokera nav',
  'err.TILE_JOKER_SWAP_MISMATCH': 'Šis kauliņš nav tas, ko džokers aizstāj',
  'rummytiles.rules.setup': 'Sagatavošana',
  'rummytiles.rules.sets': 'Kombinācijas',
  'rummytiles.rules.initialMeld': 'Pirmā izlikšana',
  'rummytiles.rules.turn': 'Tavs gājiens',
  'rummytiles.rules.jokerTaking': 'Džokera paņemšana',
  'rummytiles.rules.ending': 'Raunda beigas',
  'rummytiles.rules.poolExhaustion': 'Ja krājums izsīkst',
  'rummytiles.rules.match': 'Mača uzvarēšana',
  'rummytiles.rules.tiles': 'Spēlē ar {value} kauliņiem.',
  'rummytiles.rules.dealCount': 'Katrs spēlētājs saņem {value} kauliņus.',
  'rummytiles.rules.group': 'Grupa ir trīs vai četri viena skaitļa kauliņi, katrs citā krāsā.',
  'rummytiles.rules.run': 'Secība ir trīs vai vairāk pēc kārtas ejoši skaitļi vienā krāsā.',
  'rummytiles.rules.noWrap': 'Pēc 13 atkal nesākas 1.',
  'rummytiles.rules.joker': 'Džokers aizstāj jebkuru kauliņu.',
  'rummytiles.rules.initialMeldDescription':
    'Kamēr vienā gājienā, tikai no savas rokas, neesi izlicis {n} vai vairāk punktus, tu nedrīksti aizskart neko, kas jau ir uz galda.',
  'rummytiles.rules.turnDescription':
    'Izspēlē vismaz vienu kauliņu no rokas, brīvi pārkārto galdu un pabeidz tā, lai katra kombinācija uz galda būtu derīga.',
  'rummytiles.rules.noDiscard':
    'Izmešanas nav — ja nevari pabeigt derīgu gājienu, tā vietā pavelc vienu kauliņu.',
  'rummytiles.rules.jokerTakingDescription':
    'Uz galda esošu džokeru vari paņemt, aizstājot to ar kauliņu, kuru tas aizstāj, no savas rokas — un tas jāizmanto kombinācijā, pirms tavs gājiens beidzas.',
  'rummytiles.rules.goingOut':
    'Raundu uzvar pirmais spēlētājs, kuram beidzas kauliņi. Visi pārējie saņem negatīvu palikušā vērtību; uzvarētājs saņem visu pārējo zaudējumu summu.',
  'rummytiles.rules.poolExhaustionLowestWins':
    'Ja krājums izsīkst un neviens nevar spēlēt, raunds beidzas un to uzvar zemākā rokas vērtība.',
  'rummytiles.rules.poolExhaustionNoWinner':
    'Ja krājums izsīkst un neviens nevar spēlēt, raunds beidzas bez uzvarētāja — katru roku vienkārši saskaita.',
  'rummytiles.rules.target': 'Kurš pēc raunda beigām pirmais pārsniedz {n} punktus, uzvar mačā.',
  'rummytiles.rules.roundLimit': 'Mačs beidzas pēc {n} raundiem — uzvar augstākais rezultāts.',
  'rummytiles.fact.setCards': '{value}',
  'rummytiles.header.pool': 'Krājums {n}',
  'rummytiles.header.round': 'Raunds {n}',
  'rummytiles.round.kind': '{value}',
  'rummytiles.round.round': 'Raunds',
  'rummytiles.seat.score': '{value}',
  'rummytiles.seat.notOpened': 'Nav atvēris',
  'rummytiles.status.lastRound': 'Pēdējais raunds: {winner} ({kind})',
  'rummytiles.badge.invalid': 'Vēl nav derīgs',
  'rummytiles.zone.pool': 'Krājums',
  'rummytiles.zone.table': 'Galds',
  'rummytiles.zone.tray': 'Statīvs',
  'rummytiles.offer.place': 'Novieto',
  'rummytiles.offer.addFromHand': 'Pievieno',
  'rummytiles.offer.addFromTray': 'Pievieno no statīva',
  'rummytiles.offer.take': 'Ņem',
  'rummytiles.offer.split': 'Sadali',
  'rummytiles.offer.swapJoker': 'Nomaini džokeru',
  'rummytiles.offer.resetTurn': 'Atiestatīt gājienu',
  'rummytiles.offer.commit': 'Gatavs',
  'rummytiles.offer.draw': 'Velc',
  'rummytiles.param.position': 'Sadali pie',

  // --- blackjack -----------------------------------------------------------
  //
  // The house rules are worded twice over — on and off — because a rule that
  // is *not* in force is still something a player has to be told. "No
  // surrender at this table" is information; silence is a guess.
  'err.BET_BELOW_MINIMUM': 'Tas ir zem galda minimuma',
  'err.ALREADY_BET': 'Tava likme jau ir izdarīta',
  'err.INSURANCE_CLOSED': 'Pašlaik apdrošināšanu ņemt nevar',
  'err.CANNOT_DOUBLE': 'Šo roku nevar dubultot',
  'err.CANNOT_SPLIT': 'Šo roku nevar sadalīt',
  'err.CANNOT_SURRENDER': 'No šīs rokas nevar atteikties',

  'blackjack.rules.section.table': 'Galds',
  'blackjack.rules.section.play': 'Rokas izspēle',
  'blackjack.rules.section.dealer': 'Dalītājs',
  'blackjack.rules.section.end': 'Kā beidzas mačs',
  'blackjack.rules.goal':
    'Pārspēj dalītāju, nepārsniedzot divdesmit vienu. Pārsniegšana zaudē uzreiz, lai ko dalītājs pēc tam darītu.',
  // Counts are their own phrase rather than a number glued to a noun: one
  // deck and six decks inflect differently, and Czech inflects them again.
  'blackjack.rules.decks': 'Kavu skaits kurpē: {n}.',
  'blackjack.rules.stack': 'Katra vieta apsēžas ar {n} žetoniem.',
  'blackjack.rules.minBet': 'Galda minimums ir {n} žetoni.',
  'blackjack.rules.faceUp':
    'Spēlētāju kārtis izdala ar attēlu uz augšu; dalītājs vienu kārti tur aizklātu, līdz visi ir izspēlējuši.',
  'blackjack.rules.hitStand': 'Velc tik kāršu, cik vēlies, vai paliec pie tā, kas tev ir.',
  'blackjack.rules.aces': 'Dūzis skaitās vienpadsmit, kamēr tas ietilpst, un viens, kad neietilpst.',
  'blackjack.rules.blackjack': 'Dūzis ar desmit vērtības kārti pirmajās divās kārtīs ir blekdžeks.',
  'blackjack.rules.pays3to2': 'Blekdžeks maksā 3:2.',
  'blackjack.rules.pays6to5': 'Blekdžeks maksā 6:5.',
  'blackjack.rules.paysEven': 'Blekdžeks maksā viens pret vienu.',
  'blackjack.rules.double':
    'Pirmajās divās kārtīs tu drīksti dubultot likmi un paņemt tieši vienu papildu kārti.',
  'blackjack.rules.doubleAfterSplit': 'Arī no sadalīšanas radušos roku drīkst dubultot.',
  'blackjack.rules.noDoubleAfterSplit': 'No sadalīšanas radušos roku dubultot nedrīkst.',
  'blackjack.rules.split':
    'Divas vienādas vērtības kārtis drīkst sadalīt atsevišķās rokās, katru ar savu likmi — līdz {n} reizēm, kopā {hands} rokām.',
  'blackjack.rules.noSplit': 'Pie šī galda pārus nesadala.',
  'blackjack.rules.splitAces':
    'Sadalītie dūži saņem pa vienai kārtij un tad paliek, un tā iegūts divdesmit viens nav blekdžeks.',
  'blackjack.rules.surrender':
    'Tu drīksti atteikties no pirmās rokas par pusi likmes, kad dalītājs ir pārbaudījis blekdžeku.',
  'blackjack.rules.noSurrender': 'Pie šī galda no rokām atteikties nevar.',
  'blackjack.rules.dealerDraws': 'Dalītājs velk līdz septiņpadsmit un tad paliek.',
  'blackjack.rules.hitsSoft17': 'Dalītājs velk arī pie septiņpadsmit, kas veidota ar dūzi.',
  'blackjack.rules.standsSoft17': 'Dalītājs paliek pie septiņpadsmit, kas veidota ar dūzi.',
  'blackjack.rules.dealerPeeks':
    'Rādot dūzi vai desmitnieku, dalītājs pārbauda blekdžeku, pirms kāds ir izspēlējis.',
  'blackjack.rules.insurance':
    'Pret dalītāja dūzi tu vari apdrošināties par pusi likmes; tas maksā 2:1, ja dalītājam ir blekdžeks.',
  'blackjack.rules.noInsurance': 'Pie šī galda apdrošināšanu nepiedāvā.',
  'blackjack.rules.rounds': 'Pie galda spēlē {n} raundus.',
  'blackjack.rules.mostChipsWins': 'Kam beigās ir visvairāk žetonu, tas uzvar mačā.',
  'blackjack.rules.bustedOut': 'Vieta, kas vairs nespēj segt {n} minimumu, atlikušo maču pavada malā.',

  'blackjack.zone.dealer': 'Dalītājs',
  'blackjack.zone.box': 'Roka',
  'blackjack.zone.yourBox': 'Tava roka',
  'blackjack.zone.shoe': 'Kurpe',

  'blackjack.header.round': 'Raunds {n} no {of}',
  'blackjack.header.minBet': 'Minimums',
  'blackjack.header.decks': 'Kavas',
  'blackjack.header.dealerTotal': 'Dalītājs rāda {n}',
  'blackjack.header.dealerSoftTotal': 'Dalītājs rāda mīksto {n}',

  'blackjack.seat.stack': 'Žetoni',
  'blackjack.seat.bet': 'Likme',
  'blackjack.seat.insurance': 'Apdrošināšana',
  'blackjack.seat.total': 'Kopā',
  'blackjack.seat.softTotal': 'Mīkstā summa',
  'blackjack.seat.out': 'Žetoni beigušies',

  'blackjack.prompt.placeBet': 'Izdari savu likmi',
  'blackjack.prompt.insurance': 'Apdrošināšana?',
  'blackjack.prompt.yourMove': 'Tavs gājiens',
  'blackjack.prompt.waitingFor': 'Gaidām {playerId}',
  'blackjack.prompt.betAmount': 'Likme',

  'blackjack.quick.doubleMin': '2× Minimums',
  'blackjack.quick.allIn': 'All-in',
  'blackjack.offer.bet': 'Likt',
  'blackjack.offer.hit': 'Kārti',
  'blackjack.offer.stand': 'Palieku',
  'blackjack.offer.double': 'Dubultot',
  'blackjack.offer.split': 'Sadalīt',
  'blackjack.offer.surrender': 'Atteikties',
  'blackjack.offer.insure': 'Apdrošināties',
  'blackjack.offer.declineInsurance': 'Bez apdrošināšanas',

  'blackjack.fact.tableMinimum': 'minimums',
  'blackjack.fact.insuranceCost': 'apdrošināšanai',
  'blackjack.fact.extraStake': 'likmei',
  'blackjack.fact.surrenderReturn': 'atpakaļ',

  'blackjack.status.dealerBlackjack': 'Dalītājam bija blekdžeks',
  'blackjack.status.dealerBust': 'Dalītājs pārsniedza ar {n}',
  'blackjack.status.dealerStands': 'Dalītājs paliek pie {n}',

  'blackjack.round.name': 'Raunds',
  'blackjack.round.dealerTotal': 'Dalītājs {n}',
  'blackjack.round.dealerBust': 'Dalītājs pārsniedza ({n})',
  'blackjack.round.dealerBlackjack': 'Dalītāja blekdžeks',
  'blackjack.round.outcome.blackjack': 'Blekdžeks',
  'blackjack.round.outcome.win': 'Uzvarēts',
  'blackjack.round.outcome.push': 'Neizšķirts',
  'blackjack.round.outcome.lose': 'Zaudēts',
  'blackjack.round.outcome.bust': 'Pārsniegts',
  'blackjack.round.outcome.surrender': 'Atteikts',

  'blackjack.badge.inPlay': 'Spēlē',
  'blackjack.badge.doubled': 'Dubultots',
  'blackjack.badge.split': 'Sadalīts',
  'blackjack.badge.blackjack': 'Blekdžeks',
  'blackjack.badge.bust': 'Pārsniegts',
  'blackjack.badge.won': 'Uzvarēts',
  'blackjack.badge.push': 'Neizšķirts',
  'blackjack.badge.lost': 'Zaudēts',
  'blackjack.badge.surrendered': 'Atteikts',

  'blackjack.unit.chips': 'žetoni',

  // --- the settings screen --------------------------------------------------
  //
  // Hardcoded English until now, which was survivable while the only other
  // language was unreachable and faintly absurd once the language picker
  // itself lived on this screen: a player who cannot read "Settings" is
  // exactly the one who came here to change it.
  'settings.title': 'Iestatījumi',
  'settings.signedInAs': 'Pierakstījies kā {username}',
  'settings.playingAsGuest': 'Spēlē kā {username} (viesis)',
  'settings.notSignedIn': 'Neesi pierakstījies — piesakies vai turpini kā viesis, lai spēlētu tiešsaistē.',
  'settings.subtitle': 'Kā izskaties tu un kā izskatās galds',
  'settings.face.heading': 'Tava seja pie galda',
  'settings.face.account': 'Glabājas tavā kontā, tāpēc seko tev uz citu ierīci.',
  'settings.face.device': 'Glabājas šajā ierīcē. Pieraksties, lai paņemtu to līdzi.',
  'settings.skin.heading': 'Galda izskats',
  'settings.language.heading': 'Valoda',
  'settings.language.status': 'Glabājas šajā ierīcē.',
  'settings.language.auto': 'Automātiski',
  'settings.language.auto.now': 'Seko tavai ierīcei — pašlaik {language}',
  'settings.legal.heading': 'Sīkais druks',
  'settings.legal.status': 'Kam tu spēlējot piekriti un kas par tevi tiek glabāts.',
  'settings.signIn': 'Pierakstīties',
  'settings.back': 'Atpakaļ',

  // A translated interface over an English notice is a worse lie than an
  // English interface, so the document says so itself rather than letting the
  // surrounding screen imply the text was written for this reader.
  'legal.untranslated':
    'Šis paziņojums vēl nav tulkots tavā valodā. Spēkā ir zemāk redzamais teksts angļu valodā.',

  // --- the navigation bar's screen titles ------------------------------------
  //
  // These sit in `app/_layout.tsx`, above every screen, and were the last
  // English left after the settings screen was translated — a German player
  // reading "Einstellungen" under a bar that said "Settings". A translated app
  // with an untranslated chrome bar looks like a bug, not like a limit.
  'nav.home': 'Zolik',
  'nav.emailSignIn': 'Pierakstīšanās ar e-pastu',
  'nav.signingIn': 'Notiek pierakstīšanās',
  'nav.usernameSignIn': 'Pierakstīšanās ar lietotājvārdu',
  'nav.legacyAccount': 'Vecs konts',
  'nav.guest': 'Viesis',
  'nav.account': 'Konts',
  'nav.games': 'Spēles',
  'nav.table': 'Tavs galds',
  'nav.join': 'Pievienoties galdam',
  // The screen a shared invite link lands on while it resolves the code.
  'nav.joining': 'Notiek pievienošanās',
  'nav.rules': 'Noteikumi',
  'nav.match': 'Mačs',
  'nav.scoreTable': 'Punktu tabula',
  'nav.stats': 'Statistika',
  'nav.more': 'Vairāk',
  'nav.about': 'Par',

  // --- the account menu behind the face in the corner ----------------------
  'menu.label': 'Konta izvēlne',
  'menu.signedIn': 'Pierakstījies',
  'menu.notSignedIn': 'Neesi pierakstījies',
  'menu.keepStats': 'lai saglabātu savu statistiku',
  'menu.signOut': 'Iziet',
  'more.scoreTable': 'Bezsaistes punktu tabula',
  'more.stats': 'Statistika un līderu saraksts',
  'more.needsAccount': 'piesakies, lai lietotu',
  'gate.title': 'Piesakies, lai to lietotu',
  'gate.body':
    'Punktu tabulas un statistika glabājas kopā ar tavu kontu, tāpēc tās seko tev uz citu ierīci. Viesim nav kur tās glabāt.',

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
  'error.generic': 'Tas neizdevās',
  'error.signIn': 'Pierakstīšanās neizdevās',
  'error.login': 'Pierakstīšanās neizdevās',
  'error.register': 'Reģistrācija neizdevās',
  'error.sendCode': 'Neizdevās nosūtīt kodu',
  'error.badCode': 'Tas kods nedarbojās',
  'error.rulesLoad': 'Neizdevās ielādēt noteikumus',
  'error.createFailed': 'Izveide neizdevās',
  'error.saveFailed': 'Saglabāšana neizdevās',
  'error.exportFailed': 'Eksportēšana neizdevās',

  // --- the route that does not exist ----------------------------------------
  'notFound.title': 'Ak vai!',
  'notFound.message': 'Šāda ekrāna nav.',
  'notFound.home': 'Doties uz sākuma ekrānu!',

  // --- signing in -----------------------------------------------------------
  'auth.login.subtitle': 'Saglabā savu statistiku visās ierīcēs',
  'auth.login.continueWithEmail': 'Turpināt ar e-pastu',
  'auth.login.usernameInstead': 'Pierakstīties labāk ar lietotājvārdu',
  'auth.email.title': 'Pierakstīšanās ar e-pastu',
  'auth.email.subtitle': 'Nosūtīsim tev vienreizēju kodu',
  'auth.email.address': 'E-pasta adrese',
  'auth.email.send': 'Nosūtīt kodu',
  'auth.email.codeTitle': 'Ievadi kodu',
  'auth.email.codePlaceholder': 'Sešciparu kods',
  'auth.email.differentAddress': 'Izmantot citu adresi',
  'auth.email.sentTo': 'Nosūtīts uz {email}',
  'auth.email.continue': 'Turpināt',
  'auth.guest.title': 'Spēle kā viesim',
  'auth.guest.subtitle': 'Konts nav vajadzīgs',
  'auth.guest.displayName': 'Rādāmais vārds',
  'auth.register.title': 'Izveidot kontu',
  'auth.register.username': 'Lietotājvārds',
  'auth.register.email': 'E-pasts (nav obligāts)',
  'auth.register.password': 'Parole',
  'auth.username.createAccount': 'Izveidot kontu ar lietotājvārdu un paroli',
  'auth.callback.signedIn': 'Pierakstījies.',

  // --- the account screen ---------------------------------------------------
  'account.signInPrompt': 'Pieraksties, lai pārvaldītu savu kontu.',
  'account.keepGames': 'Paturēt šīs spēles',
  'account.signedInWith': 'Pierakstījies ar',
  'account.addMethod': 'Pievienot pierakstīšanās veidu',
  'account.usernameAndPassword': 'Lietotājvārds un parole',
  'account.faceAndTable': 'Seja un galda izskats',
  'account.refresh': 'Atsvaidzināt',
  'account.remove': 'Noņemt',

  // --- the main menu --------------------------------------------------------
  'home.subtitle': 'Kontinentālais remijs · {server}',
  'home.playingAs': 'Tu spēlē kā {name}',
  'home.signInPrompt': 'Pieraksties vai turpini kā viesis, lai spēlētu tiešsaistē.',
  'home.statsAndLeaderboard': 'Statistika un rezultātu tabula',
  'home.play': 'Spēlēt',
  'home.offlineScoreTable': 'Punktu tabula bezsaistē',
  'home.signInToKeepStats': 'Pieraksties, lai saglabātu statistiku',
  'home.signOut': 'Izrakstīties',
  'home.continueAsGuest': 'Turpināt kā viesis',
  // Someone playing as a guest, marked in a list of names.
  'home.guestSuffix': '(viesis)',

  // --- the waiting room on the main menu ------------------------------------
  //
  // Counts are whole phrases per count, not a number glued to a noun — the
  // same reason `countLabel` exists. One and many are the cases the interface
  // actually produces, so they are the cases that get their own wording.
  'waiting.checking': 'Skatāmies, kas ir tuvumā…',
  'waiting.youAreWaiting': 'Tu gaidi spēli',
  'waiting.pickedUp': 'Jebkurš, kas atver galdu, var tevi paņemt — nevienam nav vajadzīgs kods no tevis.',
  'waiting.othersOne': 'Gaida vēl 1 spēlētājs',
  'waiting.othersMany': 'Gaida vēl {n} spēlētāji',
  'waiting.oneWaiting': '1 spēlētājs gaida spēli',
  'waiting.manyWaiting': '{n} spēlētāji gaida spēli',
  'waiting.adding': 'Pievienojam tevi gaidītāju sarakstam…',
  'waiting.slowHint':
    'Ja tas nebeidzas dažās sekundēs, pārbaudi, vai zemāk norādītā servera adrese ir sasniedzama no šīs ierīces.',
  'waiting.serverBusyDetail':
    'Mēģinājums {n}. Serveris pašlaik nepieņem jaunus savienojumus ar uzgaidāmo telpu.',
  'waiting.reconnecting': 'Savienojums zudis — savienojamies atkal…',
  'waiting.reconnectingDetail':
    'Mēģinājums {n}. Tā var notikt, ja mainījies tavas ierīces tīkls vai serveris pārstartējies.',
  'waiting.tryAgain': 'Mēģināt tagad vēlreiz',
  'waiting.makeAvailable': 'Padarīt mani pieejamu spēlei',
  'waiting.stop': 'Beigt gaidīt',
  'waiting.noneYet':
    'Pašlaik neviens negaida spēli. Pieraksties sarakstā, un tu būsi pirmais, ko kāds ieraudzīs.',
  'waiting.noOthersYet': 'Neviens cits vēl negaida. Saimnieki tevi tik un tā redz un var uzaicināt.',
  'waiting.server': 'Serveris',
  'waiting.none': 'Pašlaik neviens negaida. Kas galvenajā izvēlnē padara sevi pieejamu, parādās šeit.',

  // --- landing on a shared invite link --------------------------------------
  'join.missingCode': 'Šai saitei trūkst galda koda.',
  'join.staleLink': 'Palūdz uzaicinātājam svaigu saiti vai pievienojies ar kodu.',
  'join.enterCode': 'Ievadīt kodu',
  'join.backToMenu': 'Atpakaļ uz izvēlni',
  'join.takingSeat': 'Ieņemam vietu…',
  'join.takingSeatAt': 'Ieņemam vietu pie {game}…',

  // --- the lobby ------------------------------------------------------------
  'lobby.games.subtitle': 'Viss, ko šis serveris spēj piedāvāt',
  'lobby.games.bots': 'Boti',
  'lobby.games.playBot': 'Spēlēt pret botu',
  'lobby.games.playBots': 'Spēlēt pret {n} botiem',
  'lobby.games.openTable': 'Atvērt galdu',
  'lobby.games.players': 'Spēlētāji: {n}',
  'lobby.games.playerRange': 'Spēlētāji: {min}–{max}',
  'lobby.join.placeholder': 'Pievienošanās kods vai ielūguma saite',
  'lobby.join.needCode': 'Ievadi kodu, saiti vai mača ID',
  'lobby.games.signInFirst': 'Vispirms pieraksties',
  'lobby.join.action': 'Pievienoties',
  'lobby.join.waitingTitle': 'Gaidām saimnieku',
  // Two whole sentences rather than one with a swapped noun: "a game of {game}"
  // does not survive a language that inflects the game's name after "of".
  'lobby.join.joinedGame': 'Tu pievienojies spēlei {game} — gaidām sākumu',
  'lobby.join.joinedTable': 'Tu pievienojies galdam — gaidām sākumu',
  'lobby.table.addBot': 'Pievienot botu',
  'lobby.table.side': 'Puse {n}',
  'lobby.table.shuffleSeats': 'Sajaukt vietas',
  'lobby.table.moveSeatUp': 'Pārvietot {name} vienu vietu augšup',
  'lobby.table.moveSeatDown': 'Pārvietot {name} vienu vietu lejup',
  'lobby.table.start': 'Sākt',
  'lobby.table.waitingForHost': 'Gaidām, kad saimnieks sāks…',

  // --- inviting someone to a table ------------------------------------------
  'invite.heading': 'Uzaicināt spēlētājus',
  'invite.explain': 'Nosūti šo saiti. Kas to atvērs, nonāks pie šī galda — konts nav vajadzīgs.',
  'invite.noAddress': 'Šim serverim nav iestatīta kopīgojama adrese, tāpēc izmanto zemāk esošo kodu.',
  'invite.readOutCode': 'Vai nodiktē kodu:',
  'invite.copy': 'Kopēt saiti',
  'invite.share': 'Kopīgot saiti',
  'invite.copied': 'Nokopēts!',
  'invite.shared': 'Kopīgots',

  // --- the match screen -----------------------------------------------------
  'match.waitingForTable': 'Gaidām galdu…',
  'match.waitingForPlayer': 'Gaidām citu spēlētāju…',
  'match.nobodyWon': 'Neviens neuzvarēja.',
  'match.youWon': 'Tu uzvarēji.',
  'match.finished': 'Šis mačs ir beidzies.',
  'match.inProgress': 'Mačs norit — viss ir savienots un darbojas normāli.',
  'match.connecting': 'Savienojas…',
  'match.abandonedTitle': 'Galds nolikts malā',
  'match.abandoned': 'Neviens pie šī galda neatgriezās, tāpēc tas tika nolikts malā. Kārtis ir tieši tur, kur tu tās atstāji.',
  'match.resume': 'Turpini tur, kur beidzi',
  'match.resuming': 'Atjaunoju galdu…',
  'match.controls': 'Vadība',
  'match.over': 'Mača beigas',
  'match.settingUp': 'Gatavojam…',
  'match.playAgain': 'Spēlēt vēlreiz',
  'match.backToGames': 'Atpakaļ pie spēlēm',
  'match.table': 'Galds',
  'match.opponents': 'Pretinieki',
  // Marks which seat is the reader's own, in a list of seats.
  'match.youSuffix': '(tu)',
  // The winner line is composed from whole sentences rather than a name glued
  // to " won." — the verb agrees with the subject in most of these languages,
  // and a suffix cannot know that.
  'match.you': 'tu',
  'match.someoneWon': '{name} uzvarēja.',
  'match.wonBy': 'Uzvarēja {names}.',
  'match.pausedFor': 'Apturēts — gaidām, kad {name} atkal pieslēgsies.',
  'match.results': 'Rezultāti',
  'match.players': 'Spēlētāji',
  'match.toPlay': 'gājienā',

  // --- the offline score table ----------------------------------------------
  'scoring.namesHint': 'Vārdi, atdalīti ar komatiem (4–8 spēlētāji)',
  'scoring.newSession': 'Jauna sesija',
  // A worked example, not a sentence: the names are placeholders a translator
  // may localise, the shape `name:score,` is what the parser needs.
  'scoring.scoresPlaceholder': 'Anna:120,Bruno:80,…',
  'scoring.saveRound': 'Saglabāt raundu',
  'scoring.export': 'Eksportēt punktu lapu',
  'scoring.formatHint': 'Punktu formāts: Vārds:100,Vārds2:50',
  'scoring.nameCountError': 'Ievadi 2–8 spēlētāju vārdus, atdalītus ar komatiem',
  'scoring.session': 'Sesija: {id}',
  'scoring.players': 'Spēlētāji: {names}',
  'scoring.roundScores': '{n}. raunda punkti',
  'stats.loading': 'Ielādē…',
  // The figures could not be fetched. `{reason}` is whatever the server or the
  // network said, which is not ours to translate — the frame around it is.
  'stats.unavailable': '(nav pieejams: {reason})',

  // --- statistics -----------------------------------------------------------
  'stats.title': 'Statistika un rezultātu tabula',
  'stats.yours': 'Tava statistika',
  'stats.leaderboard': 'Rezultātu tabula',

  // --- a player's lifetime record, shown beside a finished match ------------
  'record.title': 'Tavs bilance',
  'record.guest':
    'Tu spēlē kā viesis, tāpēc bilance netiek uzturēta. Pieraksties, un spēles, ko šajā ierīcē jau esi izspēlējis — arī šī — paliks pie tava konta.',
  'record.signInToKeep': 'Pierakstīties un tās saglabāt',
  'record.failed': 'Tavu bilanci pašlaik neizdevās ielādēt. Mačs ir droši saglabāts.',
  'record.loading': 'Ielādē…',
  'record.played': 'Izspēlētas',
  'record.won': 'Uzvaras',
  'record.lost': 'Zaudējumi',
  'record.winRate': 'Uzvaru daļa',
  'record.streak': 'Sērija',
  'record.atThisGame': 'Šajā spēlē',
  // A streak is a count with a noun, so it is a whole phrase per count for
  // the same reason `countLabel` is: "1 win" / "2 wins" is an English rule,
  // and most of these languages do not share it.
  'record.streakNone': '—',
  'record.streakWinOne': '1 uzvara',
  'record.streakWinMany': 'Uzvaras: {n}',
  'record.streakLossOne': '1 zaudējums',
  'record.streakLossMany': 'Zaudējumi: {n}',

  // --- moving cards ---------------------------------------------------------
  'hand.dragHint': 'Velc kārti gar vēdekli, lai to pārkārtotu, vai uz galda, lai to izspēlētu',
  'hand.moveLeft': 'Pa kreisi',
  'hand.moveRight': 'Pa labi',
  'zone.collapseGroup': 'Sakļaut šo grupu',
  'zone.expandGroup': 'Rādīt visas šīs grupas kārtis',
  'zone.dropHere': 'Nomet šeit',
  'offer.pickCards': 'izvēlies kārtis vietai, kurai pieskāries',
  'offer.ambiguous': 'tas der vairāk nekā vienā vietā — izvēlies uz galda',

  // --- the build footer -----------------------------------------------------
  'build.app': 'lietotne',
  'build.server': 'serveris',
  'about.subtitle': 'Versija, ar kuru spēlējat, un sīkais druks.',
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
  'option.pauseBetweenRounds': 'Pauze starp raundiem',
  'choice.pauseBetweenRounds.1': 'Pauze',
  'choice.pauseBetweenRounds.0': 'Turpināt uzreiz',
  'option.openDiscardPile': 'Izmešanas kaudze',
  'choice.openDiscardPile.1': 'Var pārskatīt',
  'choice.openDiscardPile.0': 'Tikai augšējā kārts',
  'option.botSkill': 'Pretinieki',
  'choice.botSkill.0': 'Jaukti',
  'choice.botSkill.1': 'Viegli',
  'choice.botSkill.2': 'Vidēji',
  'choice.botSkill.3': 'Grūti',
  'option.initialMeldMinimum': 'Atvēršanas vērtība',
  'choice.initialMeldMinimum.0': 'Nav',
  'option.discardDrawMinRound': 'Ņemšana no izmešanas kaudzes',
  'choice.discardDrawMinRound.0': 'Atvērta',
  'choice.discardDrawMinRound.2': 'No 2. raunda',
  'choice.discardDrawMinRound.3': 'No 3. raunda',
  'option.requireCleanRun': 'Secība bez džokera',
  'choice.requireCleanRun.1': 'Obligāta',
  'choice.requireCleanRun.0': 'Nē',
  'option.jokerReclaimMustPlay': 'Izpirkts džokers',
  'choice.jokerReclaimMustPlay.1': 'Jāizspēlē tajā pašā gājienā',
  'choice.jokerReclaimMustPlay.0': 'Drīkst paturēt',
  'option.dealStarter': 'Kurš sāk',
  'choice.dealStarter.0': 'Pēc kārtas',
  'choice.dealStarter.1': 'Sāk uzvarētājs',
  'variation.prsi.classic': 'Klasisks',
  'option.handSize': 'Izdalītās kārtis',
  'variation.canasta.classic': 'Klasiska',
  'variation.canasta.modern_american': 'Modern American',
  'variation.canasta.samba': 'Samba',
  'option.targetScore': 'Mērķa rezultāts',
  'option.canastasToGoOut': 'Kanastas iziešanai',
  'variation.holdem.freezeout': 'Freezeout',
  'variation.holdem.timed': 'Noteikts dalījumu skaits',
  'option.startingStack': 'Sākuma žetoni',
  'option.bigBlind': 'Lielā aklā likme',
  'option.handLimit': 'Dalījumi',
  'choice.handLimit.0': 'Līdz paliek viena vieta',
  'variation.ginrummy.standard': 'Standarta',
  'option.knockLimit': 'Klauvēšanas robeža',
  'choice.knockLimit.0': 'Oklahoma (to nosaka apgrieztā kārts)',
  'option.bigGin': 'Big gin',
  'choice.bigGin.0': 'Izslēgts',
  'choice.bigGin.1': 'Ieslēgts (+25)',
  'option.lineBonuses': 'Bonusi kopsavilkumā',
  'choice.lineBonuses.1': 'Ieslēgti',
  'choice.lineBonuses.0': 'Izslēgti',
  'variation.rummytiles.standard': 'Standarta',
  'choice.targetScore.0': 'Nav',
  // Two games qualify a number the others mean plainly: Canasta's 500 is a
  // short game, Gin Rummy's and Rummy Tiles' 500 is just 500; Hold'em's
  // starting 200 is a short stack, Blackjack's 200 is just 200. Module-scoped
  // so the qualifier travels with the game that means it — a bare key would
  // hand the note to games it is false for. `scripts/check-server-labels.js`
  // is what finds these; both were shipping as English on a Czech screen.
  'choice.canasta.targetScore.500': '500 (īsa)',
  'choice.holdem.startingStack.200': '200 (īsa)',
  'option.roundLimit': 'Raundu ierobežojums',
  'choice.roundLimit.0': 'Nav',
  'option.poolExhaustion': 'Ja krājums izsīkst',
  'choice.poolExhaustion.1': 'Raundu uzvar zemākā roka',
  'choice.poolExhaustion.0': 'Raundu neuzvar neviens',
  'variation.blackjack.single': 'Viena kava',
  'option.minBet': 'Galda minimums',
  'option.rounds': 'Raundi',
  'option.decks': 'Kavas',
  'option.dealerHitsSoft17': 'Dalītājs pie mīkstas 17',
  'choice.dealerHitsSoft17.0': 'Paliek',
  'choice.dealerHitsSoft17.1': 'Velk',
  'option.blackjackPays': 'Blekdžeks maksā',
  'choice.blackjackPays.100': 'Viens pret vienu',
  'option.maxSplits': 'Sadalīšana',
  'choice.maxSplits.0': 'Bez sadalīšanas',
  'choice.maxSplits.1': 'Vienreiz (divas rokas)',
  'choice.maxSplits.3': 'Trīs reizes (četras rokas)',
  'option.doubleAfterSplit': 'Dubultošana pēc sadalīšanas',
  'choice.doubleAfterSplit.1': 'Atļauta',
  'choice.doubleAfterSplit.0': 'Nav atļauta',
  'option.surrender': 'Atteikšanās',
  'choice.surrender.0': 'Izslēgta',
  'choice.surrender.1': 'Vēlā atteikšanās',
  'option.insurance': 'Apdrošināšana',
  'choice.insurance.1': 'Tiek piedāvāta',
  'choice.insurance.0': 'Netiek piedāvāta',

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
  'verb.add': 'Pievieno',
  'verb.bet': 'Likt',
  'verb.call': 'Izlīdzinu',
  'verb.check': 'Čeko',
  'verb.commit': 'Gatavs',
  'verb.continue': 'Turpināt',
  'verb.decline_insurance': 'Bez apdrošināšanas',
  'verb.discard': 'Izmet',
  'verb.double': 'Dubultot',
  'verb.draw': 'Velc',
  'verb.finish_layoff': 'Pievienošana pabeigta',
  'verb.fold': 'Metu',
  'verb.hit': 'Kārti',
  'verb.insure': 'Apdrošināties',
  'verb.knock': 'Klauvē',
  'verb.lay_meld': 'Izliec',
  'verb.lay_off': 'Pievieno',
  'verb.pass': 'Garām',
  'verb.place': 'Novieto',
  'verb.play_card': 'Izspēlē',
  'verb.raise': 'Paaugstinu',
  'verb.reset_turn': 'Atiestatīt gājienu',
  'verb.split': 'Sadalīt',
  'verb.stand': 'Palieku',
  'verb.surrender': 'Atteikties',
  'verb.swap_joker': 'Nomaini džokeru',
  'verb.take': 'Ņem',
  'verb.take_pile': 'Ņem no kaudzes',
  // Declared by the server and listed in `serverKeys.json`, but never worded.
  'verb.takePileFromHand': 'Ņem kaudzi rokā',
  'verb.takePileOntoMeld': 'Ņem kaudzi uz kombināciju',
  'verb.takeTopForSequence': 'Paņem augšējo kārti secībā',
  'verb.undoDraw': 'Atsaukt vilkšanu',
  'verb.undoLayOff': 'Atsaukt pievienošanu',
  'verb.undoMeld': 'Atsaukt kombināciju',
  'verb.undoTakePile': 'Atsaukt ņemšanu no kaudzes',
  'verb.undoTurn': 'Atsaukt gājienu',

  // --- suits, spelled out ---------------------------------------------------
  //
  // Read aloud by a screen reader and shown where a pip would not fit. The
  // pips themselves are drawn, not written, so these are the only place the
  // suit is ever a word.
  'suit.C': 'Kreiss',
  'suit.D': 'Kāravs',
  'suit.H': 'Ercens',
  'suit.S': 'Pīķis',

  // --- counters the board prints beside a number ----------------------------
  'canasta.seat.notOpened': 'Nav atvēris',
  'canasta.unit.points': 'punkti',
  'ginrummy.unit.points': 'punkti',
  'holdem.seat.dealer': 'Dalītājs',
  'holdem.seat.folded': 'Pameta',
  'holdem.seat.allIn': 'All in',
  'holdem.seat.out': 'Ārā',
  'holdem.unit.chips': 'žetoni',
  'prsi.unit.cardsLeft': 'atlikušas kārtis',
  'rummytiles.prompt.initialMeld': 'Tavai pirmajai izlikšanai jābūt {n} punktu vērtai.',
  'rummytiles.unit.points': 'punkti',
  'zolik.unit.penalty': 'sods',
  'header.pileFrozen': 'Kaudze iesaldēta',
  // The module-less form of `ginrummy.prompt.yourTurnDraw`: Žolíky and Canasta
  // send the prompt without a module prefix. Neither the Go constants nor
  // `serverKeys.json` list it — the sweep is what found it.
  'prompt.yourTurnDraw': 'Pavelc kārti',
  'prompt.yourTurnMeld': 'Izlic, ja vari, tad izmet',
};
