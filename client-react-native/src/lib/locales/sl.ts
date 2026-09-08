/**
 * Slovenian. Remi vocabulary: skupina for a set, niz for a run, kombinacija for a meld, talon for the stock.
 */

export const sl: Record<string, string> = {
  // --- engine error codes (rules.RulesErrorCode) ---------------------------
  'err.NOT_YOUR_TURN': 'Nisi na vrsti',
  'err.WRONG_PHASE': 'Trenutno ni mogoče',
  'err.MUST_DRAW_FIRST': 'Vzemi karto, preden položiš',
  'err.GAME_SUSPENDED': 'Igra je zaustavljena',
  'err.GAME_NOT_ACTIVE': 'Igra ne teče',
  // --- runtime refusals, which are not the rules saying no -----------------
  //
  // Both were reachable long before they were worded, and a player who met
  // one was shown the code in capitals or, worse, nothing at all: an action
  // sent while the socket was down simply vanished, which reads as a legal
  // move being silently refused.
  'err.MATCH_NOT_ACTIVE': 'Miza je zaustavljena — čakamo, da se igralec znova poveže',
  'err.NOT_CONNECTED': 'Ni povezave z mizo — ponovno se povezujemo, nato poskusi znova',
  // Why the "start the next round" control is greyed out. Without wording the
  // shell printed the code itself, so a player agreeing to go on was answered
  // with ALREADY_READY in capitals.
  'err.ALREADY_READY': 'Pripravljen si',
  'err.NOT_BETWEEN_ROUNDS': 'Krog še poteka',
  'err.NOT_AT_THIS_TABLE': 'Nisi za to mizo',
  'err.DISCARD_LOCKED': 'Kup odvrženih je zaenkrat zaklenjen',
  'err.DISCARD_PILE_EMPTY': 'Kup odvrženih je prazen',
  'err.NO_CARDS_LEFT': 'Ni več kart za jemanje',
  'err.ROUND_REQ_NOT_MET': 'Najprej položi svojo prvo kombinacijo',
  'err.NEED_CLEAN_RUN': 'Na mizi potrebuješ niz brez jokerja, da se šteješ za položenega',
  'err.INCOMPLETE_INITIAL_MELD': 'Dokončaj polaganje ali ga razveljavi, preden odvržeš',
  'err.DISCARD_CARD_NOT_MELDED': 'Karta, ki si jo pobral, mora v tvojo kombinacijo',
  'err.JOKER_DISCARD_FORBIDDEN': 'Jokerja ni mogoče odvreči',
  'err.NOTHING_TO_UNDO': 'Ni ničesar za razveljaviti',
  'err.NO_JOKER_IN_MELD': 'V tej kombinaciji ni jokerja',
  'err.JOKER_SWAP_MISMATCH': 'Ta karta ne zasede jokerjevega mesta',
  'err.RECLAIMED_JOKER_NOT_MELDED': 'Joker, vzet z mize, mora biti v tej potezi odigran v kombinacijo',
  'err.RUN_TOO_LONG': 'Ta niz je že polne dolžine',
  'err.WRONG_RUN_END': 'Ta karta podaljša drugi konec niza',
  'err.INVALID_MELD': 'Nobena karta v tvoji roki sem ne sodi',
  'err.CARD_NOT_IN_HAND': 'Te karte ni v tvoji roki',
  'err.MELD_BELOW_MINIMUM': 'Tvojim kombinacijam še manjka točk za polaganje',
  'err.MELD_NO_CONTRIBUTION': 'Ta kombinacija ne premakne tvoje zahteve naprej',
  'err.TOO_MANY_WILDS': 'Preveč jokerjev v tej kombinaciji',
  'err.ADJACENT_WILDS': 'Dva jokerja ne smeta stati drug ob drugem',
  'err.ACE_BRIDGE': 'As ne more povezati kralja in dvojke',

  // --- contract phrasing ---------------------------------------------------
  // Built from the structured contract the server sends (sets/runs counts),
  // never from a deal-number lookup table.
  // Whole phrases per count rather than a number plus a pluralised noun:
  // Czech inflects the noun by count (skupina / skupiny / skupin) in a way no
  // "add an s" helper survives, and the counts that actually occur are 1-3.
  // A count outside that falls back to contract.sets.n.
  'contract.sets.1': 'Ena skupina',
  'contract.sets.2': 'Dve skupini',
  'contract.sets.3': 'Tri skupine',
  'contract.sets.n': 'Skupine: {n}',
  'contract.runs.1': 'En niz',
  'contract.runs.2': 'Dva niza',
  'contract.runs.3': 'Trije nizi',
  'contract.runs.n': 'Nizi: {n}',
  'contract.any': 'Katera koli veljavna kombinacija',
  'contract.cleanRunOnly': 'Poljubna mešanica skupin in nizov — vsaj en niz mora biti brez jokerja',
  'contract.cleanRunSuffix': '{base} — en niz mora biti brez jokerja',

  // --- written rules ---------------------------------------------------------
  // Full-sentence keys a "see the rules" screen renders, one per module,
  // resolved server-side against the table's actual variation and options —
  // see internal/module/rules.go and each game's rules.go. Shared section
  // titles first, then one block per module in the order the four games were
  // added.
  'zolik.rules.section.goal': 'Cilj',
  'zolik.rules.section.setup': 'Priprava',
  'zolik.rules.section.turn': 'Tvoja poteza',
  'zolik.rules.section.melding': 'Polaganje',
  'zolik.rules.section.end': 'Kako se tekma konča',
  'zolik.rules.goal':
    'Bodi prvi, ki izprazni roko s polaganjem veljavnih skupin in nizov, ob tem pa zberi čim manj kazenskih točk v kartah, ki jih še držiš, ko kdo drug izide.',
  'zolik.rules.deal': 'Vsak igralec dobi {n} kart.',
  'zolik.rules.meldShapes':
    'Skupina je {set}+ kart iste vrednosti; niz je {run}+ zaporednih kart iste barve.',
  'zolik.rules.turn.draw': 'V svoji potezi vzemi eno karto — iz talona ali s kupa odvrženih.',
  'zolik.rules.pickup.topOnly': 'S kupa odvrženih je mogoče vzeti le zgornjo karto.',
  'zolik.rules.pickup.anyFromPile':
    'S kupa odvrženih je mogoče vzeti katero koli karto skupaj z vsem, kar leži nad njo.',
  'zolik.rules.pickup.locked': 'S kupa odvrženih se pred krogom {n} ne sme jemati.',
  'zolik.rules.pickup.open': 'Kup odvrženih je odprt od prvega kroga.',
  'zolik.rules.turn.discard': 'Potezo končaj tako, da odvržeš eno karto.',
  'zolik.rules.jokers.restricted':
    'Jokerja nikoli ni dovoljeno odvreči, razen kadar je natanko tista karta, ki ti izprazni roko.',
  'zolik.rules.lead.rotate':
    'Prva poteza se ob vsakem deljenju premakne za eno mesto, ne glede na to, kdo je zmagal.',
  'zolik.rules.lead.winner': 'Kdor izide, začne naslednje deljenje.',
  'zolik.rules.meldFloor.on':
    'Tvoje prvo polaganje mora skupaj znašati vsaj {n} naravnih točk, da si položen.',
  'zolik.rules.meldFloor.off': 'Za prvo polaganje ni najmanjše točkovne vrednosti.',
  'zolik.rules.cleanRun.on':
    'Vsaj eden tvojih nizov mora biti povsem brez jokerja, da se šteješ za položenega.',
  'zolik.rules.cleanRun.off':
    'Tvoji nizi lahko jokerje uporabljajo prosto — nobenemu ni treba biti brez njih.',
  'zolik.rules.contracts.rotating':
    'Tekma traja {n} deljenj in vsako deljenje zahteva svojo kombinacijo skupin in nizov.',
  'zolik.rules.contracts.static': 'Vsako deljenje zahteva isto kombinacijo: {sets} skupin in {runs} nizov.',
  'zolik.rules.end.afterDeals': 'Tekma se konča po {n} deljenjih.',
  'zolik.rules.end.atScore': 'Deli se naprej, dokler kdo ne doseže {n} točk — takrat je konec.',

  'prsi.rules.section.goal': 'Cilj',
  'prsi.rules.section.setup': 'Priprava',
  'prsi.rules.section.turn': 'Tvoja poteza',
  'prsi.rules.section.special': 'Posebne karte',
  'prsi.rules.section.end': 'Kako se tekma konča',
  'prsi.rules.goal': 'Bodi prvi, ki odigra vse karte iz roke.',
  'prsi.rules.deck': 'Igra se s kompletom {value} kart (od sedmice navzgor).',
  'prsi.rules.deal': 'Vsak igralec začne z {n} kartami.',
  'prsi.rules.turn.match':
    'Odigraj karto, ki se ujema z barvo ali vrednostjo zgornje karte — ali vzemi karto, če ne moreš.',
  'prsi.rules.turn.draw': 'Jemanje konča tvojo potezo brez odigravanja.',
  'prsi.rules.sevens': 'Odigraj 7 in naslednji igralec vzame dve karti, razen če odgovori s svojo sedmico.',
  'prsi.rules.aces': 'Odigraj asa in poteza naslednjega igralca se preskoči.',
  'prsi.rules.queens': 'Odigraj damo in povej barvo, ki se nadaljuje.',
  'prsi.rules.end': 'Tekma se konča v trenutku, ko je nekomu roka prazna.',

  'canasta.rules.section.goal': 'Cilj',
  'canasta.rules.section.setup': 'Priprava',
  'canasta.rules.section.melding': 'Polaganje',
  'canasta.rules.section.end': 'Kako se tekma konča',
  'canasta.rules.goal': 'Igra se v parih; prva stran, ki doseže {n} točk, dobi tekmo.',
  'canasta.rules.deck': 'Igra se s {value} kartami — dva kompleta in jokerji.',
  'canasta.rules.deal': 'Vsak igralec dobi {n} kart.',
  'canasta.rules.redThrees':
    'Rdeča trojka v roki se takoj pokaže in šteje kot bonus — razen če tvoja stran nikoli ne dokonča canaste, tedaj šteje proti tebi.',
  'canasta.rules.canasta': 'Canasta je kombinacija {n} ali več kart iste vrednosti.',
  'canasta.rules.meldFloorBands':
    'Tvoje prvo polaganje mora doseči najmanjše število točk, ki raste z izidom: {negative} pod ničlo, {low} do 1500, {mid} do 3000, {high} nad tem.',
  'canasta.rules.oneCanastaToGoOut': 'Ena dokončana canasta zadošča, da tvoja stran izide.',
  'canasta.rules.twoCanastasToGoOut': 'Tvoja stran potrebuje dve dokončani canasti, preden lahko izide.',
  'canasta.rules.end': 'Deli se naprej, dokler ena stran ne preseže {n} točk — takrat je tekme konec.',

  'holdem.rules.section.goal': 'Cilj',
  'holdem.rules.section.setup': 'Priprava',
  'holdem.rules.section.betting': 'Stave',
  'holdem.rules.section.end': 'Kako se tekma konča',
  'holdem.rules.goal':
    'Osvajaj žetone z najboljšo roko ob razkritju kart ali tako, da ostaneš edini igralec v deljenju.',
  'holdem.rules.stack': 'Vsako mesto začne z {n} žetoni.',
  'holdem.rules.blinds': 'Mala stava je {sb}, velika pa {bb}, obe se vplačata pred deljenjem kart.',
  'holdem.rules.streets': 'Stavi se v štirih krogih — pred flopom ter po flopu, turnu in riverju.',
  'holdem.rules.showdown': 'Tisti, ki so še v igri, razkrijejo karte; najboljša peterica pobere pot.',
  'holdem.rules.noLimit': 'Brez omejitve — vsaka stava lahko sega do celotnega tvojega kupčka.',
  'holdem.rules.lastPlayerStanding': 'Igra se, dokler eno mesto ne drži vseh žetonov.',
  'holdem.rules.mostChipsWins': 'Kdor ima ob koncu igre največ žetonov, dobi tekmo.',
  'holdem.rules.handLimit': 'Igra se ustavi po {n} deljenjih.',

  // --- header --------------------------------------------------------------
  'header.deal': 'Deljenje {n}',
  'header.gameOf': 'Igra {n} od {total}',
  'header.gameOfWithContract': 'Igra {n} od {total}: {contract}',

  // --- meld preview --------------------------------------------------------
  'preview.validSet': 'Veljavna skupina',
  'preview.validRun': 'Veljaven niz',
  'preview.validMeld': 'Veljavna kombinacija',
  'preview.notYet': 'Še ni kombinacija',
  'preview.points': '{shape} · {n} točk',
  'preview.pointsWithLaid': '{shape} · {n} + {laid} že položenih = {total} točk',
  'preview.meetsFloor': '{line} (doseže {n} ✓)',
  'preview.needsFloor': '{line} (potrebuje {n} ✗)',
  'preview.becauseOf': '{line} — {reason}',

  // --- discard -------------------------------------------------------------
  // A discard with a meld staged lays that meld first; this is what the
  // player is told when the server refuses it and the whole move is rolled
  // back. {reason} is the engine's own words for the refusal.
  'discard.meldRejected': '{reason} — nič ni bilo odvrženo, tvoje karte še vedno čakajo.',

  // --- what the current selection would send -------------------------------
  // The client's own reason a control is not ready, next to the engine's own
  // `err.*` reasons rather than a second kind of message — a control greyed
  // out for "not your turn" and one greyed out for "you picked two, this
  // takes one" should read as the same kind of thing, not one of them looking
  // broken. See `fits` in `src/lib/drops.ts`.
  'sel.tooMany.1': 'Izberi samo eno karto',
  'sel.tooMany.n': 'Izberi največ {n} kart',
  'sel.needMore': 'Izberi karte: {n}',
  'sel.notThese': 'Te karte ne morejo sem',
  'sel.needsCompany': 'Ta karta potrebuje sosednji',

  // --- what a module says happened -----------------------------------------
  // The first module-sent keys in this bundle, and deliberately few. Anything
  // a module sends is still legible without an entry here — `humanise` turns
  // `holdem.seat.stack` into "Stack" — so a key earns a line only when that
  // fallback loses something. These do: what each of them means lives in its
  // *params* (who won, how much, whose cards), and a key on its own has
  // nowhere to put them. Without the entry the player read "Winner", full
  // stop, at the end of a match they had just won.
  'status.winner': 'Zmagal {winners}',
  'holdem.status.pot': '{winners} osvoji {amount} s {hand}',
  'holdem.status.potUncontested': '{winners} osvoji {amount} — vsi drugi so odstopili',
  'holdem.status.shown': '{playerId} je pokazal {value}',
  'holdem.prompt.waitingFor': 'Čakamo na {playerId}',

  // How a rummy scoreboard was ordered. Both earn a line for the same reason
  // as the keys above: the number lives in the params, and `humanise` would
  // render "Deals Won" with nothing after it.
  'zolik.standing.dealsWon': 'Dobljena deljenja: {n}',
  'zolik.standing.inHand': 'V roki: {n}',

  // --- rounds ---------------------------------------------------------------
  // What a round is called, and what one did. Each of these carries its number
  // in the params, so `humanise` alone would render a label with nothing after
  // it.
  // An instruction, not a heading. "Next round" named the thing rather than
  // asking for it, and sat above "Waiting for 1" — which reads as though the
  // player is the one waiting, when the table is waiting for them.
  'round.continue': 'Začni naslednji krog',

  // --- the flash that announces a round or a match ending ------------------
  //
  // One line and one number, held for about two seconds. Worded generically on
  // purpose: which *kind* of ending it was is the module's own fact, printed
  // underneath, and a client that phrased "went gin" itself would be a client
  // that knows a game.
  'flash.roundWon': '{winners} ga je vzel',
  'flash.roundWonYou': 'Ti si ga vzel',
  'flash.roundDrawn': 'Nihče ga ni vzel',
  'flash.matchOver': 'Tekme je konec',
  'flash.matchWon': '{winners} zmaga',
  'flash.matchWonYou': 'Zmagal si',
  'flash.matchDrawn': 'Nihče ni zmagal',
  'flash.nowOn': 'zdaj {total}',

  'zolik.round.deal': 'Deljenje',
  'zolik.round.cleanRun': 'En niz mora biti brez jokerja',
  'canasta.round.deal': 'Deljenje',
  'canasta.round.concealed': 'Izšel skrito',
  'canasta.round.exhausted': 'Karte so pošle',
  'canasta.round.meldCards': 'Položene karte: {n}',
  'canasta.round.canastas': 'Canaste: {n}',
  'canasta.round.redThrees': 'Rdeče trojke: {n}',
  'canasta.round.goingOut': 'Izhod: {n}',
  'canasta.round.inHand': 'Ostalo v roki: {n}',
  'holdem.round.hand': 'Deljenje',
  'holdem.round.pot': 'Pot {n}',
  'holdem.round.uncontested': 'Vsi drugi so odstopili',
  'seat.ready': 'Pripravljen',
  'results.you': '(ti)',



  // --- refusals the other three games raise ---------------------------------
  // Worded here for the same reason Žolíky's are: reasonText falls back to the
  // code, and OfferBar hands it the code as that fallback, so an unworded
  // refusal reaches a player as PILE_FROZEN in capitals. serverKeys.test.ts
  // now fails the build rather than letting the next one through.
  'err.SET_TOO_LARGE': 'Skupina ima že vse štiri barve',
  'err.DISCARD_TAKEN_CARD_FORBIDDEN':
    'Karte, ki si jo pravkar vzel, ne moreš odvreči — odigraj jo ali jo obdrži',
  'err.CARD_DOES_NOT_FIT': 'Ta karta se ne ujema ne po barvi ne po vrednosti',
  'err.SUIT_REQUIRED': 'Povej barvo, ki se nadaljuje',
  'err.MUST_ANSWER_DRAW_OR_TAKE': 'Odgovori s sedmico ali vzemi karte',
  'err.NOTHING_TO_DRAW': 'Ni več ničesar za vzeti',
  'err.PILE_EMPTY': 'Kup je prazen',
  'err.PILE_BLOCKED': 'Kup je blokiran — na vrhu leži črna trojka',
  'err.PILE_FROZEN': 'Kup je zamrznjen — potrebuješ dve naravni karti vrednosti zgornje karte',
  'err.TOP_CARD_UNUSABLE': 'Zgornje karte ne moreš uporabiti',
  'err.MELD_CLOSED': 'Ta kombinacija je popolna in zaprta',
  'err.MELD_TOO_SMALL': 'Kombinacija potrebuje več kart od tega',
  'err.MELD_TOO_LARGE': 'Ta kombinacija ne more sprejeti več kart',
  'err.MELD_MIXED_RANKS': 'Vse karte v kombinaciji morajo biti iste vrednosti',
  'err.NOT_ENOUGH_NATURALS': 'Kombinacija potrebuje več naravnih kart kot jokerjev',
  'err.RANK_ALREADY_MELDED': 'Tvoja stran že ima kombinacijo te vrednosti',
  'err.NOT_YOUR_MELD': 'Ta kombinacija pripada nasprotni strani',
  'err.NO_SUCH_MELD': 'Te kombinacije ni na mizi',
  'err.CANNOT_MELD_THREE': 'Trojk se nikoli ne polaga',
  'err.CANNOT_DISCARD_RED_THREE': 'Rdeče trojke ni mogoče odvreči',
  'err.MUST_KEEP_A_CARD': 'Obdrži vsaj eno karto — tako roke ne moreš izprazniti',
  'err.MUST_MELD_FIRST': 'Najprej položi prvo kombinacijo svoje strani',
  'err.INITIAL_MELD_NOT_MET': 'Tvojemu prvemu polaganju še manjka točk',
  'err.CANNOT_GO_OUT_YET': 'Tvoja stran potrebuje dokončano canasto, preden lahko izide',
  'err.NOTHING_TO_CALL': 'Ni stave za izenačitev',
  'err.CANNOT_CHECK': 'Ne moreš čekirati — na mizi je stava, na katero moraš odgovoriti',
  'err.CANNOT_RAISE': 'Tukaj ne moreš zvišati',
  'err.RAISE_TOO_SMALL': 'Zvišanje mora biti vsaj tolikšno kot prejšnje',
  'err.NOT_ENOUGH_CHIPS': 'Toliko žetonov nimaš',
  'err.AMOUNT_REQUIRED': 'Povej, koliko',
  'err.AMOUNT_NOT_A_NUMBER': 'Ta znesek ni število',
  'err.SEAT_NOT_IN_HAND': 'V tem deljenju nisi',
  'err.WRONG_RANK': 'Ta karta je za to napačne vrednosti',
  'err.MATCH_FULL': 'Miza je polna',
  'err.MATCH_ALREADY_STARTED': 'Tekma se je že začela',
  'err.TOO_FEW_PLAYERS': 'Igralcev še ni dovolj',
  'err.WRONG_PLAYER_COUNT': 'Te igre ni mogoče igrati s toliko igralci',
  'err.NOT_THE_HOST': 'To lahko stori samo gostitelj',
  'err.NO_LONGER_WAITING': 'Miza ne čaka več',
  'err.WAITING_ROOM_UNAVAILABLE': 'Čakalnica ni na voljo',
  'err.SERVER_BUSY': 'Strežnik je trenutno poln — poskusi znova čez trenutek',


  // --- rules the engine has always enforced and never stated ----------------
  // Every one of these was found by the guardrail rather than by review: the
  // validator could refuse a player for it, and no sentence anywhere said so.
  // See server/internal/zolikmod/ruleindex_test.go.
  'zolik.rules.section.layoff': 'Dodajanje h kombinacijam',
  'zolik.rules.pickup.obligation':
    'Dokler nisi položen, mora biti karta, vzeta s kupa odvrženih, uporabljena v kombinaciji, s katero v tej potezi položiš.',
  'zolik.rules.pickup.noReturn':
    'Karte, vzete s kupa odvrženih, v isti potezi ni dovoljeno znova odvreči — odigraj jo ali jo obdrži.',
  'zolik.rules.wilds.setLimit': 'Skupina ne sme vsebovati več jokerjev kot naravnih kart.',
  'zolik.rules.set.maxSize':
    'Skupina ne sme imeti več kot {n} kart — joker nadomesti manjkajočo barvo, ne dopolnjuje polne skupine.',
  'zolik.rules.run.maxLength':
    'Niz ne sme imeti več kot {n} kart — as spodaj, dvanajst vrednosti nad njim in as zgoraj.',
  'zolik.rules.run.aceBridge': 'As stoji nad kraljem ali pod dvojko, nikoli kot most med obema koncema niza.',
  'zolik.rules.contracts.contribution':
    'Dokler nisi položen, mora biti vsaka položena kombinacija taka, kot jo pogodba deljenja še zahteva.',
  'zolik.rules.layoff.afterDown': 'Tujim kombinacijam ne smeš dodajati, dokler ne položiš svoje pogodbe.',
  'zolik.rules.layoff.runEnds': 'Karta, dodana nizu, ga mora nadaljevati na enem ali drugem koncu.',
  'zolik.rules.jokers.swap':
    'Jokerja v kombinaciji na mizi je mogoče odkupiti natanko s karto, ki jo predstavlja.',
  'zolik.rules.jokers.reclaim.on':
    'Joker, odkupljen z mize, mora biti v isti potezi odigran v kombinacijo — ne sme ostati v roki.',
  'zolik.rules.jokers.reclaim.off': 'Joker, odkupljen z mize, lahko ostane v roki.',
  'zolik.rules.deck.reshuffle':
    'Ko talon poide, se kup odvrženih premeša in postane nov talon; če sta oba prazna, se deljenje konča.',


  // --- what to do instead ---------------------------------------------------
  // The third layer of a refusal, after the reason and the rule. Sent by the
  // module because it is the only side that knows which card is owed and
  // which way out is on offer; see server/internal/zolikmod/remedy.go.
  'zolik.remedy.meldThePickup': 'Dodaj {card} svojemu polaganju ali razveljavi pobiranje.',
  'zolik.remedy.discardSomethingElse': 'Odvrzi drugo karto ali odigraj {card} v tej potezi.',
  'zolik.remedy.discardNotAJoker': 'Odvrzi kaj drugega kot jokerja.',
  'zolik.remedy.finishOrUndoLayDown': 'Dokončaj polaganje ali ga vzemi nazaj.',
  'zolik.remedy.needMorePoints': 'Potrebuješ še {n} točk, da boš lahko položil.',
  'zolik.remedy.layACleanRun': 'Položi niz brez jokerja v njem.',
  'zolik.remedy.playReclaimedJoker': 'Odigraj {card} v kombinacijo ali razveljavi jemanje.',
  'zolik.remedy.goDownFirst': 'Najprej položi svoje kombinacije.',
  'zolik.remedy.drawFirst': 'Najprej vzemi karto.',
  'zolik.remedy.drawFromStock': 'Vzemi iz talona — kup odvrženih se odpre v krogu {n}.',
  'zolik.remedy.drawFromStockEmpty': 'Raje vzemi iz talona.',


  // --- keys whose meaning lives in their params -----------------------------
  // The shape fallback renders these with the number missing ("Stack", "Pot"),
  // which is why they earn a line where most keys do not.
  'header.contract': 'Zahteva {sets} skupin in {runs} nizov',
  'header.contract.cleanRunOnly': 'Zahteva niz brez jokerja',
  'header.round': 'Krog {n}',
  'header.deck': 'Talon',
  'header.target': 'Cilj',
  'header.suitInPlay': 'Barva v igri',
  'seat.cards': 'Karte',
  'zolik.offer.meld': 'Položi',
  'prompt.pickupMustBeMelded':
    '{value} je prišla s kupa odvrženih — mora v kombinacije, s katerimi v tej potezi položiš.',
  'prompt.jokerMustBePlayed': '{value} je prišla z mize — mora v kombinacijo, preden lahko končaš potezo.',
  'prompt.initialMeld': 'Prva kombinacija tvoje strani mora doseči {n} točk.',
  'prompt.canastasNeeded': 'Tvoji strani manjka še {n} canast, preden lahko izide.',
  'prompt.mustDrawOrAnswerSeven': 'Odgovori s sedmico ali vzemi {n} kart.',
  'prompt.chooseSuit': 'Izberi barvo, ki se nadaljuje',
  'prompt.skipPending': 'Tvoja poteza se preskoči',
  'status.lastDeal': 'Ekipa {team} je dosegla {value}',
  'status.teamScore': 'Ekipa {team}: {value}',
  'canasta.offer.rank': 'Vrednost',
  'canasta.seat.teamScore': 'Točke ekipe',
  'canasta.seat.canastas': 'Canaste',
  'holdem.header.pot': 'Pot',
  'holdem.header.street': 'Ulica',
  'holdem.header.hand': 'Deljenje',
  'holdem.header.handLimit': 'Deljenj skupaj',
  'holdem.header.blinds': 'Male in velike stave',
  'holdem.cost.call': 'za izenačitev',
  'holdem.cost.pot': 'v potu',
  'holdem.seat.stack': 'Kupček',
  'holdem.seat.bet': 'Stava',
  'holdem.prompt.yourAction': 'Na vrsti si',
  'holdem.prompt.raiseTo': 'Zvišaj na',
  'zone.yourHand': 'Tvoja roka',
  'zone.opponentHand': 'Nasprotnikova roka',
  'zone.drawPile': 'Talon',
  'zone.discardPile': 'Kup odvrženih',
  'zone.melds': 'Kombinacije',
  'zone.teamMelds': 'Kombinacije tvoje strani',
  'zone.redThrees': 'Rdeče trojke',
  'zone.board': 'Miza',
  'verb.drawFromDeck': 'Vzemi',
  'verb.takeFromDiscard': 'Vzemi s kupa',


  // --- the why sheet's own furniture ---------------------------------------
  // The three layers a refusal is explained in. Labels, not sentences: what
  // goes under each is sent by the server or worded above.
  'why.reason': 'Zakaj ne',
  'why.rule': 'Pravilo',
  'why.rules': 'Pravila',
  'why.remedy': 'Kaj lahko storiš',
  'why.readTheRules': 'Preberi vsa pravila →',
  'why.close': 'Zapri',
  'why.open': 'zakaj',

  // --- marks on a particular card ------------------------------------------
  // A mark is a refusal that has not happened yet: the rule is enforced at the
  // discard, which is the last possible moment to hear about it, so the card
  // says so while there is still a turn left to act on it.
  'zolik.badge.owedToMeld':
    '{card} je prišla s kupa odvrženih — mora v kombinacije, s katerimi v tej potezi položiš.',
  'zolik.badge.jokerOwed': '{card} je prišla z mize — mora v kombinacijo, preden lahko končaš potezo.',

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
  'legal.terms': 'Pogoji',
  // The document's own name, used as the heading above it. Kept apart from
  // `legal.notice.terms`, which is the same words inflected to sit inside a
  // sentence — a heading in the instrumental case reads as a mistake in every
  // Slavic language.
  'legal.terms.title': 'Pogoji uporabe',
  'legal.privacy.title': 'Obvestilo o zasebnosti',
  'legal.privacy': 'Zasebnost',
  'legal.source': 'Izvorna koda',
  'legal.updated': 'Različica {version}',
  'legal.draft': 'Osnutek — še ne velja. Ime, država in kontaktni naslov upravljavca so še neizpolnjeni.',
  'legal.notice.before': 'Z igranjem sprejemaš ',
  'legal.notice.terms': 'pogoje uporabe',
  'legal.notice.between': '. Kaj se o tebi hrani, je opisano v ',
  'legal.notice.privacy': 'obvestilu o zasebnosti',
  'legal.notice.after': '.',
  // --- gin rummy -------------------------------------------------------------
  'err.UPCARD_DECLINED': 'To karto si že izpustil',
  'err.DEADWOOD_TOO_HIGH': 'Tvoj deadwood je previsok za trkanje',
  'err.CARD_DOES_NOT_EXTEND_MELD': 'Ta karta te kombinacije ne podaljša',
  'ginrummy.rules.setup': 'Priprava',
  'ginrummy.rules.turn': 'Tvoja poteza',
  'ginrummy.rules.melds': 'Kombinacije',
  'ginrummy.rules.knocking': 'Trkanje',
  'ginrummy.rules.bigGin': 'Big gin',
  'ginrummy.rules.layoff': 'Prislanjanje',
  'ginrummy.rules.deadHand': 'Mrtvo deljenje',
  'ginrummy.rules.scoring': 'Točkovanje deljenja',
  'ginrummy.rules.match': 'Zmaga v tekmi',
  'ginrummy.rules.lineBonuses': 'Bonusi v obračunu',
  'ginrummy.rules.deck': 'Igra se s kompletom {value} kart.',
  'ginrummy.rules.deal': 'Vsak igralec dobi {value} kart.',
  'ginrummy.rules.upcard': 'Še ena karta se obrne z licem navzgor in začne kup odvrženih.',
  'ginrummy.rules.drawDiscard':
    'V svoji potezi vzemi eno karto — iz talona ali s kupa odvrženih — nato eno odvrzi.',
  'ginrummy.rules.setsAndRuns':
    'Kombinacija je skupina treh ali štirih kart ene vrednosti ali niz treh ali več kart iste barve.',
  'ginrummy.rules.aceLow': 'As je vedno nizek — niza od dame do asa ni.',
  'ginrummy.rules.knockLimit': 'Trkati smeš takoj, ko je tvoj deadwood {n} ali manj.',
  'ginrummy.rules.oklahoma': 'Mejo trkanja v tem deljenju določa vrednost obrnjene karte.',
  'ginrummy.rules.gin': 'Nič deadwooda je gin — najboljše možno trkanje.',
  'ginrummy.rules.bigGinBonus':
    'Enajst kart, vse v kombinacijah, brez slehernega odmeta, je big gin in prinese še {n} točk.',
  'ginrummy.rules.layoffDescription':
    'Po trkanju, ki ni gin, sme nasprotnik svoj deadwood prisloniti k tvojim kombinacijam, preden se roki primerjata.',
  'ginrummy.rules.deadHandDescription':
    'Če talon pade na zadnji dve karti in nihče ni potrkal, je deljenje mrtvo — nihče ne dobi točk in isti delivec deli znova.',
  'ginrummy.rules.undercut':
    'Če nasprotnikov deadwood ni višji od tvojega, te podreže: dobi razliko in še {n}.',
  'ginrummy.rules.ginBonus': 'Gin prinese celotno nasprotnikovo roko in še {n}.',
  'ginrummy.rules.target': 'Kdor po koncu deljenja prvi preseže {n} točk, dobi tekmo.',
  'ginrummy.rules.shutout': 'Bonus za tekmo se podvoji na {n}, če poraženec ni dosegel niti ene točke.',
  'ginrummy.rules.box': 'Vsako dobljeno deljenje je ob koncu tekme vredno {n} točk.',
  'ginrummy.rules.gameBonus': 'Zmaga v tekmi prinese še {n} točk.',
  'ginrummy.fact.deadwood': 'deadwood {value}',
  'ginrummy.fact.discardCard': 'Odvrzi {value}',
  'ginrummy.fact.meldCards': 'Na {value}',
  'ginrummy.header.hand': 'Deljenje {n}',
  'ginrummy.round.kind': '{value}',
  'ginrummy.round.hand': 'Deljenje',
  'ginrummy.seat.score': '{value}',
  'ginrummy.seat.dealer': 'Delivec',
  'ginrummy.status.knocked': '{playerId} je potrkal z deadwoodom {deadwood}',
  'ginrummy.status.gin': '{playerId} je naredil gin',
  'ginrummy.status.lastHand': 'Zadnje deljenje: {winner} ({kind}, {delta} točk)',
  'ginrummy.offer.drawStock': 'Vzemi iz talona',
  'ginrummy.offer.drawDiscard': 'Vzemi s kupa odvrženih',
  'ginrummy.offer.takeUpcard': 'Vzemi obrnjeno karto',
  'ginrummy.offer.passUpcard': 'Naprej',
  'ginrummy.offer.discard': 'Odvrzi',
  'ginrummy.offer.knock': 'Potrkaj',
  'ginrummy.offer.gin': 'Gin!',
  'ginrummy.offer.bigGin': 'Big gin!',
  'ginrummy.offer.layOff': 'Prisloni',
  'ginrummy.offer.finishLayoff': 'Konec prislanjanja',
  'ginrummy.zone.knockerHand': 'Roka tistega, ki je potrkal',
  'ginrummy.zone.melds': 'Kombinacije',
  'ginrummy.prompt.upcardDecision': 'Vzemi obrnjeno karto ali jo izpusti',
  'ginrummy.prompt.yourTurnDraw': 'Vzemi karto',
  'ginrummy.prompt.yourTurnDiscard': 'Odvrzi — ali potrkaj, če lahko',
  'ginrummy.prompt.layoff': 'Prisloni deadwood ali končaj',

  // --- rummy tiles -------------------------------------------------------------
  'err.TILE_NOT_IN_HAND': 'Te ploščice ni v tvoji roki',
  'err.TILE_DOES_NOT_FIT': 'To tja ne sodi',
  'err.NO_SUCH_SET': 'Te kombinacije ni na mizi',
  'err.INITIAL_MELD_ONLY': 'Pred prvim polaganjem lahko prerazporejaš samo svoje nove kombinacije',
  'err.TABLE_NOT_VALID': 'Miza še ni veljavna',
  'err.TRAY_NOT_EMPTY': 'Imaš še proste ploščice za postaviti',
  'err.NOTHING_PLAYED': 'Odigraj vsaj eno ploščico, preden končaš potezo',
  'err.INITIAL_MELD_TOO_LOW': 'Tvoje prvo polaganje mora biti vredno vsaj 30 točk',
  'err.NOT_A_RUN': 'Razdeliti je mogoče samo niz',
  'err.BAD_SPLIT_POSITION': 'Na tem mestu tega niza ni mogoče razdeliti',
  'err.NO_JOKER_IN_SET': 'V tej kombinaciji ni jokerja',
  'err.TILE_JOKER_SWAP_MISMATCH': 'Ta ploščica ni tisto, kar joker predstavlja',
  'rummytiles.rules.setup': 'Priprava',
  'rummytiles.rules.sets': 'Kombinacije',
  'rummytiles.rules.initialMeld': 'Prvo polaganje',
  'rummytiles.rules.turn': 'Tvoja poteza',
  'rummytiles.rules.jokerTaking': 'Jemanje jokerja',
  'rummytiles.rules.ending': 'Konec kroga',
  'rummytiles.rules.poolExhaustion': 'Če zaloga poide',
  'rummytiles.rules.match': 'Zmaga v tekmi',
  'rummytiles.rules.tiles': 'Igra se s {value} ploščicami.',
  'rummytiles.rules.dealCount': 'Vsak igralec dobi {value} ploščic.',
  'rummytiles.rules.group': 'Skupina so tri ali štiri ploščice iste številke, vsaka druge barve.',
  'rummytiles.rules.run': 'Niz so tri ali več zaporednih številk iste barve.',
  'rummytiles.rules.noWrap': 'Po 13 se ne začne znova pri 1.',
  'rummytiles.rules.joker': 'Joker predstavlja katero koli ploščico.',
  'rummytiles.rules.initialMeldDescription':
    'Dokler v eni sami potezi, izključno iz svoje roke, ne položiš {n} ali več točk, se ne smeš dotakniti ničesar, kar je že na mizi.',
  'rummytiles.rules.turnDescription':
    'Odigraj vsaj eno ploščico iz roke, mizo prosto prerazporejaj in končaj tako, da je vsaka kombinacija na mizi veljavna.',
  'rummytiles.rules.noDiscard':
    'Odmeta ni — če veljavne poteze ne moreš dokončati, namesto tega vzameš eno ploščico.',
  'rummytiles.rules.jokerTakingDescription':
    'Jokerja na mizi lahko vzameš tako, da ga zamenjaš s ploščico, ki jo predstavlja, iz svoje roke — in uporabiti ga moraš v kombinaciji, preden se ti poteza konča.',
  'rummytiles.rules.goingOut':
    'Krog dobi prvi igralec, ki mu zmanjka ploščic. Vsi drugi prejmejo negativno vrednost tistega, kar jim je ostalo; zmagovalec prejme vsoto izgub vseh drugih.',
  'rummytiles.rules.poolExhaustionLowestWins':
    'Če zaloga poide in nihče ne more igrati, se krog konča in dobi ga najnižja vrednost roke.',
  'rummytiles.rules.poolExhaustionNoWinner':
    'Če zaloga poide in nihče ne more igrati, se krog konča brez zmagovalca — vsaka roka se preprosto točkuje.',
  'rummytiles.rules.target': 'Kdor po koncu kroga prvi preseže {n} točk, dobi tekmo.',
  'rummytiles.rules.roundLimit': 'Tekma se konča po {n} krogih — zmaga najvišji izid.',
  'rummytiles.fact.setCards': '{value}',
  'rummytiles.header.pool': 'Zaloga {n}',
  'rummytiles.header.round': 'Krog {n}',
  'rummytiles.round.kind': '{value}',
  'rummytiles.round.round': 'Krog',
  'rummytiles.seat.score': '{value}',
  'rummytiles.seat.notOpened': 'Ni odprl',
  'rummytiles.status.lastRound': 'Zadnji krog: {winner} ({kind})',
  'rummytiles.badge.invalid': 'Še ni veljavno',
  'rummytiles.zone.pool': 'Zaloga',
  'rummytiles.zone.table': 'Miza',
  'rummytiles.zone.tray': 'Stojalo',
  'rummytiles.offer.place': 'Postavi',
  'rummytiles.offer.addFromHand': 'Dodaj',
  'rummytiles.offer.addFromTray': 'Dodaj s stojala',
  'rummytiles.offer.take': 'Vzemi',
  'rummytiles.offer.split': 'Razdeli',
  'rummytiles.offer.swapJoker': 'Zamenjaj jokerja',
  'rummytiles.offer.resetTurn': 'Ponastavi potezo',
  'rummytiles.offer.commit': 'Končano',
  'rummytiles.offer.draw': 'Vzemi',
  'rummytiles.param.position': 'Razdeli pri',

  // --- blackjack -----------------------------------------------------------
  //
  // The house rules are worded twice over — on and off — because a rule that
  // is *not* in force is still something a player has to be told. "No
  // surrender at this table" is information; silence is a guess.
  'err.BET_BELOW_MINIMUM': 'To je pod najnižjo stavo mize',
  'err.ALREADY_BET': 'Tvoja stava je že postavljena',
  'err.INSURANCE_CLOSED': 'Trenutno ni zavarovanja za vzeti',
  'err.CANNOT_DOUBLE': 'Te roke ni mogoče podvojiti',
  'err.CANNOT_SPLIT': 'Te roke ni mogoče razdeliti',
  'err.CANNOT_SURRENDER': 'Te roke ni mogoče predati',

  'blackjack.rules.section.table': 'Miza',
  'blackjack.rules.section.play': 'Igranje roke',
  'blackjack.rules.section.dealer': 'Delivec',
  'blackjack.rules.section.end': 'Kako se tekma konča',
  'blackjack.rules.goal':
    'Premagaj delivca, ne da bi presegel enaindvajset. Preseganje izgubi takoj, karkoli delivec stori pozneje.',
  // Counts are their own phrase rather than a number glued to a noun: one
  // deck and six decks inflect differently, and Czech inflects them again.
  'blackjack.rules.decks': 'Kompletov v čevlju: {n}.',
  'blackjack.rules.stack': 'Vsako mesto sede z {n} žetoni.',
  'blackjack.rules.minBet': 'Najnižja stava mize je {n} žetonov.',
  'blackjack.rules.faceUp':
    'Karte igralcev se delijo z licem navzgor; delivec eno karto drži zakrito, dokler vsi ne odigrajo.',
  'blackjack.rules.hitStand': 'Vzemi toliko kart, kolikor želiš, ali obstani pri tem, kar imaš.',
  'blackjack.rules.aces': 'As velja enajst, dokler to gre, sicer pa ena.',
  'blackjack.rules.blackjack': 'As s karto vrednosti deset, na prvih dveh kartah, je blackjack.',
  'blackjack.rules.pays3to2': 'Blackjack plača 3:2.',
  'blackjack.rules.pays6to5': 'Blackjack plača 6:5.',
  'blackjack.rules.paysEven': 'Blackjack plača ena proti ena.',
  'blackjack.rules.double': 'Na prvih dveh kartah lahko podvojiš stavo in vzameš natanko še eno karto.',
  'blackjack.rules.doubleAfterSplit': 'Tudi roko, nastalo iz delitve, je mogoče podvojiti.',
  'blackjack.rules.noDoubleAfterSplit': 'Roke, nastale iz delitve, ni mogoče podvojiti.',
  'blackjack.rules.split':
    'Dve karti iste vrednosti je mogoče razdeliti v ločeni roki, vsako s svojo stavo — do {n}-krat, skupaj za {hands} rok.',
  'blackjack.rules.noSplit': 'Za to mizo se pari ne delijo.',
  'blackjack.rules.splitAces':
    'Razdeljena asa dobita po eno karto in nato obstaneta, enaindvajset, dosežen tako, pa ni blackjack.',
  'blackjack.rules.surrender':
    'Prvo roko lahko predaš za polovico stave, potem ko delivec preveri blackjack.',
  'blackjack.rules.noSurrender': 'Za to mizo rok ni mogoče predati.',
  'blackjack.rules.dealerDraws': 'Delivec vleče do sedemnajst in nato obstane.',
  'blackjack.rules.hitsSoft17': 'Delivec vleče tudi pri sedemnajst, sestavljeni z asom.',
  'blackjack.rules.standsSoft17': 'Delivec obstane pri sedemnajst, sestavljeni z asom.',
  'blackjack.rules.dealerPeeks':
    'Kadar kaže asa ali desetko, delivec preveri blackjack, preden kdor koli odigra.',
  'blackjack.rules.insurance':
    'Proti delivčevemu asu se lahko zavaruješ za polovico stave; plača 2:1, če ima delivec blackjack.',
  'blackjack.rules.noInsurance': 'Za to mizo zavarovanje ni na voljo.',
  'blackjack.rules.rounds': 'Za mizo se igra {n} krogov.',
  'blackjack.rules.mostChipsWins': 'Kdor ima na koncu največ žetonov, dobi tekmo.',
  'blackjack.rules.bustedOut': 'Mesto, ki ne more več pokriti najnižje stave {n}, do konca tekme počiva.',

  'blackjack.zone.dealer': 'Delivec',
  'blackjack.zone.box': 'Roka',
  'blackjack.zone.yourBox': 'Tvoja roka',
  'blackjack.zone.shoe': 'Čevelj',

  'blackjack.header.round': 'Krog {n} od {of}',
  'blackjack.header.minBet': 'Najnižja stava',
  'blackjack.header.decks': 'Kompleti',
  'blackjack.header.dealerTotal': 'Delivec kaže {n}',
  'blackjack.header.dealerSoftTotal': 'Delivec kaže mehkih {n}',

  'blackjack.seat.stack': 'Žetoni',
  'blackjack.seat.bet': 'Stava',
  'blackjack.seat.insurance': 'Zavarovanje',
  'blackjack.seat.total': 'Skupaj',
  'blackjack.seat.softTotal': 'Mehka vsota',
  'blackjack.seat.out': 'Brez žetonov',

  'blackjack.prompt.placeBet': 'Postavi svojo stavo',
  'blackjack.prompt.insurance': 'Zavarovanje?',
  'blackjack.prompt.yourMove': 'Na vrsti si',
  'blackjack.prompt.waitingFor': 'Čakamo na {playerId}',
  'blackjack.prompt.betAmount': 'Stava',

  'blackjack.offer.bet': 'Stavi',
  'blackjack.offer.hit': 'Karta',
  'blackjack.offer.stand': 'Obstanem',
  'blackjack.offer.double': 'Podvoji',
  'blackjack.offer.split': 'Razdeli',
  'blackjack.offer.surrender': 'Predaj',
  'blackjack.offer.insure': 'Vzemi zavarovanje',
  'blackjack.offer.declineInsurance': 'Brez zavarovanja',

  'blackjack.fact.tableMinimum': 'najnižja stava',
  'blackjack.fact.insuranceCost': 'za zavarovanje',
  'blackjack.fact.extraStake': 'za stavo',
  'blackjack.fact.surrenderReturn': 'nazaj',

  'blackjack.status.dealerBlackjack': 'Delivec je imel blackjack',
  'blackjack.status.dealerBust': 'Delivec je počil pri {n}',
  'blackjack.status.dealerStands': 'Delivec obstane pri {n}',

  'blackjack.round.name': 'Krog',
  'blackjack.round.dealerTotal': 'Delivec {n}',
  'blackjack.round.dealerBust': 'Delivec počil ({n})',
  'blackjack.round.dealerBlackjack': 'Delivčev blackjack',
  'blackjack.round.outcome.blackjack': 'Blackjack',
  'blackjack.round.outcome.win': 'Dobljeno',
  'blackjack.round.outcome.push': 'Izenačeno',
  'blackjack.round.outcome.lose': 'Izgubljeno',
  'blackjack.round.outcome.bust': 'Počilo',
  'blackjack.round.outcome.surrender': 'Predano',

  'blackjack.badge.inPlay': 'V igri',
  'blackjack.badge.doubled': 'Podvojeno',
  'blackjack.badge.split': 'Razdeljeno',
  'blackjack.badge.blackjack': 'Blackjack',
  'blackjack.badge.bust': 'Počilo',
  'blackjack.badge.won': 'Dobljeno',
  'blackjack.badge.push': 'Izenačeno',
  'blackjack.badge.lost': 'Izgubljeno',
  'blackjack.badge.surrendered': 'Predano',

  'blackjack.unit.chips': 'žetonov',

  // --- the settings screen --------------------------------------------------
  //
  // Hardcoded English until now, which was survivable while the only other
  // language was unreachable and faintly absurd once the language picker
  // itself lived on this screen: a player who cannot read "Settings" is
  // exactly the one who came here to change it.
  'settings.title': 'Nastavitve',
  'settings.subtitle': 'Kako izgledaš ti in kako miza',
  'settings.face.heading': 'Tvoj obraz za mizo',
  'settings.face.account': 'Shranjeno pri tvojem računu, zato te spremlja na drugo napravo.',
  'settings.face.device': 'Shranjeno na tej napravi. Prijavi se, da ga vzameš s seboj.',
  'settings.skin.heading': 'Videz mize',
  'settings.language.heading': 'Jezik',
  'settings.language.status': 'Shranjeno na tej napravi.',
  'settings.language.auto': 'Samodejno',
  'settings.language.auto.now': 'Sledi tvoji napravi — zdaj {language}',
  'settings.legal.heading': 'Drobni tisk',
  'settings.legal.status': 'S čim si se z igranjem strinjal in kaj se o tebi hrani.',
  'settings.signIn': 'Prijava',
  'settings.back': 'Nazaj',

  // A translated interface over an English notice is a worse lie than an
  // English interface, so the document says so itself rather than letting the
  // surrounding screen imply the text was written for this reader.
  'legal.untranslated': 'To obvestilo še ni prevedeno v tvoj jezik. Velja angleško besedilo spodaj.',

  // --- the navigation bar's screen titles ------------------------------------
  //
  // These sit in `app/_layout.tsx`, above every screen, and were the last
  // English left after the settings screen was translated — a German player
  // reading "Einstellungen" under a bar that said "Settings". A translated app
  // with an untranslated chrome bar looks like a bug, not like a limit.
  'nav.home': 'Zolik',
  'nav.emailSignIn': 'Prijava z e-pošto',
  'nav.signingIn': 'Prijavljanje',
  'nav.usernameSignIn': 'Prijava z uporabniškim imenom',
  'nav.legacyAccount': 'Star račun',
  'nav.guest': 'Gost',
  'nav.account': 'Račun',
  'nav.games': 'Igre',
  'nav.table': 'Tvoja miza',
  'nav.join': 'Pridruži se mizi',
  // The screen a shared invite link lands on while it resolves the code.
  'nav.joining': 'Pridruževanje',
  'nav.rules': 'Pravila',
  'nav.match': 'Tekma',
  'nav.scoreTable': 'Tabela točk',
  'nav.stats': 'Statistika',

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
  'error.generic': 'To ni uspelo',
  'error.signIn': 'Prijava ni uspela',
  'error.login': 'Prijava ni uspela',
  'error.register': 'Registracija ni uspela',
  'error.sendCode': 'Kode ni bilo mogoče poslati',
  'error.badCode': 'Ta koda ni delovala',
  'error.rulesLoad': 'Pravil ni bilo mogoče naložiti',
  'error.createFailed': 'Ustvarjanje ni uspelo',
  'error.saveFailed': 'Shranjevanje ni uspelo',
  'error.exportFailed': 'Izvoz ni uspel',

  // --- the route that does not exist ----------------------------------------
  'notFound.title': 'Ups!',
  'notFound.message': 'Ta zaslon ne obstaja.',
  'notFound.home': 'Pojdi na začetni zaslon!',

  // --- signing in -----------------------------------------------------------
  'auth.login.subtitle': 'Obdrži svojo statistiko na vseh napravah',
  'auth.login.continueWithEmail': 'Nadaljuj z e-pošto',
  'auth.login.usernameInstead': 'Prijavi se raje z uporabniškim imenom',
  'auth.email.title': 'Prijava z e-pošto',
  'auth.email.subtitle': 'Poslali ti bomo enkratno kodo',
  'auth.email.address': 'E-poštni naslov',
  'auth.email.send': 'Pošlji kodo',
  'auth.email.codeTitle': 'Vnesi kodo',
  'auth.email.codePlaceholder': 'Šestmestna koda',
  'auth.email.differentAddress': 'Uporabi drug naslov',
  'auth.email.sentTo': 'Poslano na {email}',
  'auth.email.continue': 'Naprej',
  'auth.guest.title': 'Igra kot gost',
  'auth.guest.subtitle': 'Račun ni potreben',
  'auth.guest.displayName': 'Prikazano ime',
  'auth.register.title': 'Ustvari račun',
  'auth.register.username': 'Uporabniško ime',
  'auth.register.email': 'E-pošta (neobvezno)',
  'auth.register.password': 'Geslo',
  'auth.username.createAccount': 'Ustvari račun z uporabniškim imenom in geslom',
  'auth.callback.signedIn': 'Prijavljen.',

  // --- the account screen ---------------------------------------------------
  'account.signInPrompt': 'Prijavi se, da urediš svoj račun.',
  'account.keepGames': 'Obdrži te igre',
  'account.signedInWith': 'Prijavljen prek',
  'account.addMethod': 'Dodaj način prijave',
  'account.usernameAndPassword': 'Uporabniško ime in geslo',
  'account.faceAndTable': 'Obraz in videz mize',
  'account.refresh': 'Osveži',
  'account.remove': 'Odstrani',

  // --- the main menu --------------------------------------------------------
  'home.subtitle': 'Continental Rummy · {server}',
  'home.playingAs': 'Igraš kot {name}',
  'home.signInPrompt': 'Prijavi se ali nadaljuj kot gost, da igraš na spletu.',
  'home.statsAndLeaderboard': 'Statistika in lestvica',
  'home.play': 'Igraj',
  'home.offlineScoreTable': 'Tabela točk brez povezave',
  'home.signInToKeepStats': 'Prijavi se, da obdržiš statistiko',
  'home.signOut': 'Odjava',
  'home.continueAsGuest': 'Nadaljuj kot gost',
  // Someone playing as a guest, marked in a list of names.
  'home.guestSuffix': '(gost)',

  // --- the waiting room on the main menu ------------------------------------
  //
  // Counts are whole phrases per count, not a number glued to a noun — the
  // same reason `countLabel` exists. One and many are the cases the interface
  // actually produces, so they are the cases that get their own wording.
  'waiting.checking': 'Gledamo, kdo je v bližini…',
  'waiting.youAreWaiting': 'Čakaš na igro',
  'waiting.pickedUp': 'Kdor koli odpre mizo, te lahko pobere — nihče ne potrebuje kode od tebe.',
  'waiting.othersOne': 'Čaka še 1 igralec',
  'waiting.othersMany': 'Čaka še {n} igralcev',
  'waiting.oneWaiting': '1 igralec čaka na igro',
  'waiting.manyWaiting': '{n} igralcev čaka na igro',
  'waiting.adding': 'Dodajamo te na čakalni seznam…',
  'waiting.slowHint':
    'Če se to v nekaj sekundah ne konča, preveri, ali je spodnji naslov strežnika dosegljiv s te naprave.',
  'waiting.serverBusyDetail': 'Poskus {n}. Strežnik trenutno ne sprejema novih povezav v čakalnico.',
  'waiting.reconnecting': 'Povezava izgubljena — ponovno se povezujemo…',
  'waiting.reconnectingDetail':
    'Poskus {n}. To se lahko zgodi, če se je omrežje tvoje naprave spremenilo ali se je strežnik znova zagnal.',
  'waiting.tryAgain': 'Poskusi zdaj znova',
  'waiting.makeAvailable': 'Naredi me na voljo za igro',
  'waiting.stop': 'Nehaj čakati',
  'waiting.noneYet': 'Trenutno nihče ne čaka na igro. Vpiši se na seznam in boš prvi, ki ga kdor koli vidi.',
  'waiting.noOthersYet': 'Nihče drug še ne čaka. Gostitelji te vseeno vidijo in te lahko povabijo.',
  'waiting.server': 'Strežnik',
  'waiting.none': 'Trenutno nihče ne čaka. Kdor se v glavnem meniju naredi na voljo, se pojavi tukaj.',

  // --- landing on a shared invite link --------------------------------------
  'join.staleLink': 'Prosi tistega, ki te je povabil, za svežo povezavo, ali se pridruži s kodo.',
  'join.enterCode': 'Vnesi kodo',
  'join.backToMenu': 'Nazaj v meni',
  'join.takingSeat': 'Zasedamo mesto…',
  'join.takingSeatAt': 'Zasedamo mesto pri {game}…',

  // --- the lobby ------------------------------------------------------------
  'lobby.games.subtitle': 'Vse, kar ta strežnik zmore ponuditi',
  'lobby.games.bots': 'Boti',
  'lobby.games.playBot': 'Igraj proti botu',
  'lobby.games.playBots': 'Igraj proti {n} botom',
  'lobby.games.openTable': 'Odpri mizo',
  'lobby.games.players': 'Igralcev: {n}',
  'lobby.games.playerRange': 'Igralcev: {min}–{max}',
  'lobby.join.placeholder': 'Koda za pridružitev ali povezava s povabilom',
  'lobby.join.action': 'Pridruži se',
  'lobby.join.waitingTitle': 'Čakamo gostitelja',
  // Two whole sentences rather than one with a swapped noun: "a game of {game}"
  // does not survive a language that inflects the game's name after "of".
  'lobby.join.joinedGame': 'Pridružil si se igri {game} — čakamo na začetek',
  'lobby.join.joinedTable': 'Pridružil si se mizi — čakamo na začetek',
  'lobby.table.addBot': 'Dodaj bota',
  'lobby.table.start': 'Začni',
  'lobby.table.waitingForHost': 'Čakamo, da gostitelj začne…',

  // --- inviting someone to a table ------------------------------------------
  'invite.heading': 'Povabi igralce',
  'invite.explain': 'Pošlji to povezavo. Kdor jo odpre, pristane pri tej mizi — račun ni potreben.',
  'invite.noAddress': 'Za ta strežnik ni nastavljen naslov za deljenje, zato uporabi spodnjo kodo.',
  'invite.readOutCode': 'Ali pa narekuj kodo:',
  'invite.copy': 'Kopiraj povezavo',
  'invite.share': 'Deli povezavo',
  'invite.copied': 'Kopirano!',
  'invite.shared': 'Deljeno',

  // --- the match screen -----------------------------------------------------
  'match.waitingForTable': 'Čakamo mizo…',
  'match.waitingForPlayer': 'Čakamo drugega igralca…',
  'match.nobodyWon': 'Nihče ni zmagal.',
  'match.youWon': 'Zmagal si.',
  'match.finished': 'Ta tekma se je končala.',
  'match.inProgress': 'Tekma poteka — vse je povezano in teče normalno.',
  'match.controls': 'Upravljanje',
  'match.over': 'Konec tekme',
  'match.settingUp': 'Pripravljamo…',
  'match.playAgain': 'Igraj znova',
  'match.backToGames': 'Nazaj k igram',
  'match.table': 'Miza',
  'match.opponents': 'Nasprotniki',
  // Marks which seat is the reader's own, in a list of seats.
  'match.youSuffix': '(ti)',
  // The winner line is composed from whole sentences rather than a name glued
  // to " won." — the verb agrees with the subject in most of these languages,
  // and a suffix cannot know that.
  'match.you': 'ti',
  'match.someoneWon': '{name} zmaga.',
  'match.wonBy': 'Zmaga {names}.',
  'match.pausedFor': 'Zaustavljeno — čakamo, da se {name} znova poveže.',
  'match.results': 'Rezultati',
  'match.players': 'Igralci',
  'match.toPlay': 'na potezi',

  // --- the offline score table ----------------------------------------------
  'scoring.namesHint': 'Imena, ločena z vejicami (4–8 igralcev)',
  'scoring.newSession': 'Nova seja',
  // A worked example, not a sentence: the names are placeholders a translator
  // may localise, the shape `name:score,` is what the parser needs.
  'scoring.scoresPlaceholder': 'Ana:120,Boris:80,…',
  'scoring.saveRound': 'Shrani krog',
  'scoring.export': 'Izvozi zapisnik',
  'scoring.formatHint': 'Oblika točk: Ime:100,Ime2:50',
  'scoring.session': 'Seja: {id}',
  'scoring.players': 'Igralci: {names}',
  'scoring.roundScores': 'Točke kroga {n}',
  'stats.loading': 'Nalaganje…',

  // --- statistics -----------------------------------------------------------
  'stats.title': 'Statistika in lestvica',
  'stats.yours': 'Tvoja statistika',
  'stats.leaderboard': 'Lestvica',

  // --- a player's lifetime record, shown beside a finished match ------------
  'record.title': 'Tvoj izkupiček',
  'record.guest':
    'Igraš kot gost, zato se izkupiček ne vodi. Prijavi se in igre, ki si jih na tej napravi že odigral — vključno s to — bodo ostale pri tvojem računu.',
  'record.signInToKeep': 'Prijavi se in jih obdrži',
  'record.failed': 'Tvojega izkupička zdaj ni bilo mogoče naložiti. Tekma je varno zabeležena.',
  'record.loading': 'Nalaganje…',
  'record.played': 'Odigrano',
  'record.won': 'Zmage',
  'record.lost': 'Porazi',
  'record.winRate': 'Delež zmag',
  'record.streak': 'Niz',
  'record.atThisGame': 'Pri tej igri',
  // A streak is a count with a noun, so it is a whole phrase per count for
  // the same reason `countLabel` is: "1 win" / "2 wins" is an English rule,
  // and most of these languages do not share it.
  'record.streakNone': '—',
  'record.streakWinOne': '1 zmaga',
  'record.streakWinMany': '{n} zmag',
  'record.streakLossOne': '1 poraz',
  'record.streakLossMany': '{n} porazov',

  // --- moving cards ---------------------------------------------------------
  'hand.dragHint': 'Povleci karto vzdolž pahljače, da jo prerazporediš, ali na mizo, da jo odigraš',
  'hand.moveLeft': 'Levo',
  'hand.moveRight': 'Desno',
  'zone.collapseGroup': 'Strni to skupino',
  'zone.expandGroup': 'Pokaži vse karte te skupine',
  'zone.dropHere': 'Spusti sem',
  'offer.pickCards': 'izberi karte za mesto, ki si se ga dotaknil',
  'offer.ambiguous': 'to lahko gre na več mest — izberi na mizi',

  // --- the build footer -----------------------------------------------------
  'build.app': 'aplikacija',
  'build.server': 'strežnik',
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
  'option.pauseBetweenRounds': 'Premor med krogi',
  'choice.pauseBetweenRounds.1': 'Premor',
  'choice.pauseBetweenRounds.0': 'Nadaljuj takoj',
  'option.botSkill': 'Nasprotniki',
  'choice.botSkill.0': 'Mešani',
  'choice.botSkill.1': 'Lahki',
  'choice.botSkill.2': 'Srednji',
  'choice.botSkill.3': 'Težki',
  'option.initialMeldMinimum': 'Vrednost odprtja',
  'choice.initialMeldMinimum.0': 'Brez',
  'option.discardDrawMinRound': 'Jemanje s kupa odvrženih',
  'choice.discardDrawMinRound.0': 'Odprto',
  'choice.discardDrawMinRound.2': 'Od kroga 2',
  'choice.discardDrawMinRound.3': 'Od kroga 3',
  'option.requireCleanRun': 'Niz brez jokerja',
  'choice.requireCleanRun.1': 'Obvezen',
  'choice.requireCleanRun.0': 'Ne',
  'option.jokerReclaimMustPlay': 'Odkupljeni joker',
  'choice.jokerReclaimMustPlay.1': 'Odigrati v isti potezi',
  'choice.jokerReclaimMustPlay.0': 'Lahko ostane v roki',
  'option.dealStarter': 'Kdo začne',
  'choice.dealStarter.0': 'Izmenično',
  'choice.dealStarter.1': 'Začne zmagovalec',
  'variation.prsi.classic': 'Klasično',
  'option.handSize': 'Razdeljene karte',
  'variation.canasta.classic': 'Klasična',
  'variation.canasta.modern_american': 'Modern American',
  'option.targetScore': 'Ciljne točke',
  'option.canastasToGoOut': 'Canaste za izhod',
  'variation.holdem.freezeout': 'Freezeout',
  'variation.holdem.timed': 'Stalno število deljenj',
  'option.startingStack': 'Začetni žetoni',
  'option.bigBlind': 'Velika stava',
  'option.handLimit': 'Deljenja',
  'choice.handLimit.0': 'Dokler ne ostane eno mesto',
  'variation.ginrummy.standard': 'Standardni',
  'option.knockLimit': 'Meja trkanja',
  'choice.knockLimit.0': 'Oklahoma (določi jo obrnjena karta)',
  'option.bigGin': 'Big gin',
  'choice.bigGin.0': 'Izklopljeno',
  'choice.bigGin.1': 'Vklopljeno (+25)',
  'option.lineBonuses': 'Bonusi v obračunu',
  'choice.lineBonuses.1': 'Vklopljeni',
  'choice.lineBonuses.0': 'Izklopljeni',
  'variation.rummytiles.standard': 'Standardno',
  'choice.targetScore.0': 'Brez',
  'option.roundLimit': 'Omejitev krogov',
  'choice.roundLimit.0': 'Brez',
  'option.poolExhaustion': 'Če zaloga poide',
  'choice.poolExhaustion.1': 'Krog dobi najnižja roka',
  'choice.poolExhaustion.0': 'Kroga ne dobi nihče',
  'variation.blackjack.single': 'En komplet',
  'option.minBet': 'Najnižja stava mize',
  'option.rounds': 'Krogi',
  'option.decks': 'Kompleti',
  'option.dealerHitsSoft17': 'Delivec pri mehki 17',
  'choice.dealerHitsSoft17.0': 'Obstane',
  'choice.dealerHitsSoft17.1': 'Vleče',
  'option.blackjackPays': 'Blackjack plača',
  'choice.blackjackPays.100': 'Ena proti ena',
  'option.maxSplits': 'Deljenje',
  'choice.maxSplits.0': 'Brez deljenja',
  'choice.maxSplits.1': 'Enkrat (dve roki)',
  'choice.maxSplits.3': 'Trikrat (štiri roke)',
  'option.doubleAfterSplit': 'Podvojitev po deljenju',
  'choice.doubleAfterSplit.1': 'Dovoljeno',
  'choice.doubleAfterSplit.0': 'Ni dovoljeno',
  'option.surrender': 'Predaja',
  'choice.surrender.0': 'Izklopljena',
  'choice.surrender.1': 'Pozna predaja',
  'option.insurance': 'Zavarovanje',
  'choice.insurance.1': 'Na voljo',
  'choice.insurance.0': 'Ni na voljo',
};
