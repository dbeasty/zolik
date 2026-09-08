/**
 * Slovak. Mirrors the Czech bundle's vocabulary — skupina, postupka, žolík, kombinácia — because the two are read side by side by the same players, and gratuitous divergence between them reads as a mistake in one of the two.
 */

export const sk: Record<string, string> = {
  // --- engine error codes (rules.RulesErrorCode) ---------------------------
  'err.NOT_YOUR_TURN': 'Nie si na rade',
  'err.WRONG_PHASE': 'Teraz to nejde',
  'err.MUST_DRAW_FIRST': 'Najprv si potiahni kartu',
  'err.GAME_SUSPENDED': 'Hra je pozastavená',
  'err.GAME_NOT_ACTIVE': 'Hra nebeží',
  // --- runtime refusals, which are not the rules saying no -----------------
  //
  // Both were reachable long before they were worded, and a player who met
  // one was shown the code in capitals or, worse, nothing at all: an action
  // sent while the socket was down simply vanished, which reads as a legal
  // move being silently refused.
  'err.MATCH_NOT_ACTIVE': 'Stôl je pozastavený — čaká sa, kým sa hráč pripojí',
  'err.NOT_CONNECTED': 'Bez spojenia so stolom — pripájame znova, potom to skús ešte raz',
  // Why the "start the next round" control is greyed out. Without wording the
  // shell printed the code itself, so a player agreeing to go on was answered
  // with ALREADY_READY in capitals.
  'err.ALREADY_READY': 'Si pripravený',
  'err.NOT_BETWEEN_ROUNDS': 'Kolo ešte prebieha',
  'err.NOT_AT_THIS_TABLE': 'Nesedíš pri tomto stole',
  'err.DISCARD_LOCKED': 'Odhadzovací balíček je zatiaľ zamknutý',
  'err.DISCARD_PILE_EMPTY': 'Odhadzovací balíček je prázdny',
  'err.NO_CARDS_LEFT': 'Nie sú už karty na ťahanie',
  'err.ROUND_REQ_NOT_MET': 'Najprv vylož vlastnú prvú kombináciu',
  'err.NEED_CLEAN_RUN': 'Potrebuješ na stole postupku bez žolíka, aby si sa rátal za vyloženého',
  'err.INCOMPLETE_INITIAL_MELD': 'Dokonči vykladanie alebo ho vráť späť, než odhodíš kartu',
  'err.DISCARD_CARD_NOT_MELDED': 'Karta, ktorú si zobral, musí ísť do tvojej kombinácie',
  'err.JOKER_DISCARD_FORBIDDEN': 'Žolíka nemožno odhodiť',
  'err.NOTHING_TO_UNDO': 'Nie je čo vrátiť späť',
  'err.NO_JOKER_IN_MELD': 'V tejto kombinácii nie je žolík',
  'err.JOKER_SWAP_MISMATCH': 'Táto karta nezaujme miesto žolíka',
  'err.RECLAIMED_JOKER_NOT_MELDED': 'Žolík zobratý zo stola musí byť v tomto ťahu zahraný do kombinácie',
  'err.RUN_TOO_LONG': 'Táto postupka má už plnú dĺžku',
  'err.WRONG_RUN_END': 'Táto karta predlžuje druhý koniec postupky',
  'err.INVALID_MELD': 'Žiadna karta v tvojej ruke sem nepasuje',
  'err.CARD_NOT_IN_HAND': 'Túto kartu v ruke nemáš',
  'err.MELD_BELOW_MINIMUM': 'Tvojim kombináciám stále chýbajú body na vyloženie',
  'err.MELD_NO_CONTRIBUTION': 'Táto kombinácia neposúva tvoju požiadavku',
  'err.TOO_MANY_WILDS': 'Priveľa žolíkov v tejto kombinácii',
  'err.ADJACENT_WILDS': 'Dvaja žolíci nemôžu ležať vedľa seba',
  'err.ACE_BRIDGE': 'Eso nemôže spájať kráľa a dvojku',

  // --- contract phrasing ---------------------------------------------------
  // Built from the structured contract the server sends (sets/runs counts),
  // never from a deal-number lookup table.
  // Whole phrases per count rather than a number plus a pluralised noun:
  // Czech inflects the noun by count (skupina / skupiny / skupin) in a way no
  // "add an s" helper survives, and the counts that actually occur are 1-3.
  // A count outside that falls back to contract.sets.n.
  'contract.sets.1': 'Jedna skupina',
  'contract.sets.2': 'Dve skupiny',
  'contract.sets.3': 'Tri skupiny',
  'contract.sets.n': 'Skupiny: {n}',
  'contract.runs.1': 'Jedna postupka',
  'contract.runs.2': 'Dve postupky',
  'contract.runs.3': 'Tri postupky',
  'contract.runs.n': 'Postupky: {n}',
  'contract.any': 'Akákoľvek platná kombinácia',
  'contract.cleanRunOnly': 'Ľubovoľná zmes skupín a postupiek — aspoň jedna postupka musí byť bez žolíka',
  'contract.cleanRunSuffix': '{base} — jedna postupka musí byť bez žolíka',

  // --- written rules ---------------------------------------------------------
  // Full-sentence keys a "see the rules" screen renders, one per module,
  // resolved server-side against the table's actual variation and options —
  // see internal/module/rules.go and each game's rules.go. Shared section
  // titles first, then one block per module in the order the four games were
  // added.
  'zolik.rules.section.goal': 'Cieľ',
  'zolik.rules.section.setup': 'Príprava',
  'zolik.rules.section.turn': 'Tvoj ťah',
  'zolik.rules.section.melding': 'Vykladanie',
  'zolik.rules.section.end': 'Ako sa zápas končí',
  'zolik.rules.goal':
    'Buď prvý, kto si vyprázdni ruku vykladaním platných skupín a postupiek, a nazbieraj pritom čo najmenej trestných bodov v kartách, ktoré ti zostanú, keď niekto iný vyjde.',
  'zolik.rules.deal': 'Každý hráč dostane {n} kariet.',
  'zolik.rules.meldShapes':
    'Skupina je {set}+ kariet rovnakej hodnoty; postupka je {run}+ po sebe idúcich kariet tej istej farby.',
  'zolik.rules.turn.draw':
    'V svojom ťahu si potiahni jednu kartu — z balíčka alebo z odhadzovacieho balíčka.',
  'zolik.rules.pickup.topOnly': 'Z odhadzovacieho balíčka možno vziať iba vrchnú kartu.',
  'zolik.rules.pickup.anyFromPile':
    'Z odhadzovacieho balíčka možno vziať ktorúkoľvek kartu spolu so všetkým, čo leží nad ňou.',
  'zolik.rules.pickup.locked': 'Z odhadzovacieho balíčka sa nesmie ťahať pred kolom {n}.',
  'zolik.rules.pickup.open': 'Odhadzovací balíček je otvorený od prvého kola.',
  'zolik.rules.turn.discard': 'Ukonči svoj ťah odhodením jednej karty.',
  'zolik.rules.jokers.restricted':
    'Žolíka nikdy nemožno odhodiť — okrem prípadu, keď je presne tou kartou, ktorá ti vyprázdni ruku.',
  'zolik.rules.lead.rotate':
    'Prvý výnos sa posúva o jedno miesto každé rozdanie, bez ohľadu na to, kto vyhral.',
  'zolik.rules.lead.winner': 'Kto vyjde, vynáša v ďalšom rozdaní.',
  'zolik.rules.meldFloor.on':
    'Tvoje prvé vyloženie musí dať dokopy aspoň {n} prirodzených bodov, kým budeš vyložený.',
  'zolik.rules.meldFloor.off': 'Prvé vyloženie nemá minimálnu bodovú hodnotu.',
  'zolik.rules.cleanRun.on':
    'Aspoň jedna z tvojich postupiek musí byť úplne bez žolíka, kým sa budeš rátať za vyloženého.',
  'zolik.rules.cleanRun.off': 'Tvoje postupky môžu žolíkov využívať voľne — žiadna nemusí byť bez nich.',
  'zolik.rules.contracts.rotating':
    'Zápas trvá {n} rozdaní a každé rozdanie vyžaduje vlastnú kombináciu skupín a postupiek.',
  'zolik.rules.contracts.static':
    'Každé rozdanie vyžaduje tú istú kombináciu: {sets} skupín a {runs} postupiek.',
  'zolik.rules.end.afterDeals': 'Zápas sa končí po {n} rozdaniach.',
  'zolik.rules.end.atScore': 'Rozdáva sa ďalej, kým niekto nedosiahne {n} bodov — potom je koniec.',

  'prsi.rules.section.goal': 'Cieľ',
  'prsi.rules.section.setup': 'Príprava',
  'prsi.rules.section.turn': 'Tvoj ťah',
  'prsi.rules.section.special': 'Zvláštne karty',
  'prsi.rules.section.end': 'Ako sa zápas končí',
  'prsi.rules.goal': 'Buď prvý, kto zahrá všetky karty z ruky.',
  'prsi.rules.deck': 'Hrá sa s balíčkom {value} kariet (od sedmičky nahor).',
  'prsi.rules.deal': 'Každý hráč začína s {n} kartami.',
  'prsi.rules.turn.match':
    'Zahraj kartu, ktorá sedí farbou alebo hodnotou s vrchnou kartou — alebo si potiahni, ak nemôžeš.',
  'prsi.rules.turn.draw': 'Potiahnutie ukončí tvoj ťah bez zahrania.',
  'prsi.rules.sevens': 'Zahraj 7 a ďalší hráč si ťahá dve karty, ak neodpovie vlastnou sedmičkou.',
  'prsi.rules.aces': 'Zahraj eso a ďalší hráč ťah vynechá.',
  'prsi.rules.queens': 'Zahraj dámu a povedz farbu, ktorá pokračuje.',
  'prsi.rules.end': 'Zápas sa končí vo chvíli, keď je niečia ruka prázdna.',

  'canasta.rules.section.goal': 'Cieľ',
  'canasta.rules.section.setup': 'Príprava',
  'canasta.rules.section.melding': 'Vykladanie',
  'canasta.rules.section.end': 'Ako sa zápas končí',
  'canasta.rules.goal': 'Hrá sa vo dvojiciach; prvá strana, ktorá dosiahne {n} bodov, vyhráva zápas.',
  'canasta.rules.deck': 'Hrá sa s {value} kartami — dva balíčky plus žolíci.',
  'canasta.rules.deal': 'Každý hráč dostane {n} kariet.',
  'canasta.rules.redThrees':
    'Červená trojka v ruke sa hneď ukáže a počíta sa ako bonus — okrem prípadu, keď tvoja strana nikdy nedokončí canastu, vtedy sa počíta proti tebe.',
  'canasta.rules.canasta': 'Canasta je kombinácia {n} alebo viacerých kariet rovnakej hodnoty.',
  'canasta.rules.meldFloorBands':
    'Tvoje prvé vyloženie musí dosiahnuť bodové minimum, ktoré rastie so skóre: {negative} pod nulou, {low} do 1500, {mid} do 3000, {high} nad tým.',
  'canasta.rules.oneCanastaToGoOut': 'Jedna dokončená canasta stačí, aby tvoja strana mohla vyjsť.',
  'canasta.rules.twoCanastasToGoOut': 'Tvoja strana potrebuje dve dokončené canasty, kým môže vyjsť.',
  'canasta.rules.end': 'Rozdáva sa ďalej, kým jedna strana neprekročí {n} bodov — potom je zápas na konci.',

  'holdem.rules.section.goal': 'Cieľ',
  'holdem.rules.section.setup': 'Príprava',
  'holdem.rules.section.betting': 'Stávkovanie',
  'holdem.rules.section.end': 'Ako sa zápas končí',
  'holdem.rules.goal':
    'Vyhrávaj žetóny najlepšou kombináciou pri odkrytí kariet alebo tým, že zostaneš jediný v hre.',
  'holdem.rules.stack': 'Každé miesto začína s {n} žetónmi.',
  'holdem.rules.blinds': 'Malý blind je {sb} a veľký blind {bb}, vkladajú sa pred rozdaním kariet.',
  'holdem.rules.streets': 'Stávkuje sa v štyroch kolách — pred flopom a po flope, turne a riveri.',
  'holdem.rules.showdown': 'Kto je ešte v hre, odkryje karty; najlepšia päťkartová kombinácia berie bank.',
  'holdem.rules.noLimit': 'Bez limitu — každá stávka môže ísť až do výšky celého tvojho stacku.',
  'holdem.rules.lastPlayerStanding': 'Hrá sa, kým jedno miesto nedrží všetky žetóny.',
  'holdem.rules.mostChipsWins': 'Kto má pri ukončení hry najviac žetónov, vyhráva zápas.',
  'holdem.rules.handLimit': 'Hra sa končí po {n} rozdaniach.',

  // --- header --------------------------------------------------------------
  'header.deal': 'Rozdanie {n}',
  'header.gameOf': 'Hra {n} z {total}',
  'header.gameOfWithContract': 'Hra {n} z {total}: {contract}',

  // --- meld preview --------------------------------------------------------
  'preview.validSet': 'Platná skupina',
  'preview.validRun': 'Platná postupka',
  'preview.validMeld': 'Platná kombinácia',
  'preview.notYet': 'Zatiaľ to nie je kombinácia',
  'preview.points': '{shape} · {n} b.',
  'preview.pointsWithLaid': '{shape} · {n} + {laid} už vyložených = {total} b.',
  'preview.meetsFloor': '{line} (dosahuje {n} ✓)',
  'preview.needsFloor': '{line} (treba {n} ✗)',
  'preview.becauseOf': '{line} — {reason}',

  // --- discard -------------------------------------------------------------
  // A discard with a meld staged lays that meld first; this is what the
  // player is told when the server refuses it and the whole move is rolled
  // back. {reason} is the engine's own words for the refusal.
  'discard.meldRejected': '{reason} — nič sa neodhodilo, tvoje karty stále čakajú.',

  // --- what the current selection would send -------------------------------
  // The client's own reason a control is not ready, next to the engine's own
  // `err.*` reasons rather than a second kind of message — a control greyed
  // out for "not your turn" and one greyed out for "you picked two, this
  // takes one" should read as the same kind of thing, not one of them looking
  // broken. See `fits` in `src/lib/drops.ts`.
  'sel.tooMany.1': 'Vyber len jednu kartu',
  'sel.tooMany.n': 'Vyber najviac {n} kariet',
  'sel.needMore': 'Vyber karty: {n}',
  'sel.notThese': 'Tieto karty sem nemôžu',
  'sel.needsCompany': 'Táto karta potrebuje susedné',

  // --- what a module says happened -----------------------------------------
  // The first module-sent keys in this bundle, and deliberately few. Anything
  // a module sends is still legible without an entry here — `humanise` turns
  // `holdem.seat.stack` into "Stack" — so a key earns a line only when that
  // fallback loses something. These do: what each of them means lives in its
  // *params* (who won, how much, whose cards), and a key on its own has
  // nowhere to put them. Without the entry the player read "Winner", full
  // stop, at the end of a match they had just won.
  'status.winner': 'Vyhráva {winners}',
  'holdem.status.pot': '{winners} vyhráva {amount} s {hand}',
  'holdem.status.potUncontested': '{winners} vyhráva {amount} — všetci ostatní zložili',
  'holdem.status.shown': '{playerId} ukázal {value}',
  'holdem.prompt.waitingFor': 'Čaká sa na {playerId}',

  // How a rummy scoreboard was ordered. Both earn a line for the same reason
  // as the keys above: the number lives in the params, and `humanise` would
  // render "Deals Won" with nothing after it.
  'zolik.standing.dealsWon': 'Vyhraté rozdania: {n}',
  'zolik.standing.inHand': 'V ruke: {n}',

  // --- rounds ---------------------------------------------------------------
  // What a round is called, and what one did. Each of these carries its number
  // in the params, so `humanise` alone would render a label with nothing after
  // it.
  // An instruction, not a heading. "Next round" named the thing rather than
  // asking for it, and sat above "Waiting for 1" — which reads as though the
  // player is the one waiting, when the table is waiting for them.
  'round.continue': 'Začať ďalšie kolo',

  // --- the flash that announces a round or a match ending ------------------
  //
  // One line and one number, held for about two seconds. Worded generically on
  // purpose: which *kind* of ending it was is the module's own fact, printed
  // underneath, and a client that phrased "went gin" itself would be a client
  // that knows a game.
  'flash.roundWon': '{winners} to bral',
  'flash.roundWonYou': 'Bral si to ty',
  'flash.roundDrawn': 'Nebral to nikto',
  'flash.matchOver': 'Koniec zápasu',
  'flash.matchWon': '{winners} vyhráva',
  'flash.matchWonYou': 'Vyhrávaš',
  'flash.matchDrawn': 'Nevyhral nikto',
  'flash.nowOn': 'teraz {total}',

  'zolik.round.deal': 'Rozdanie',
  'zolik.round.cleanRun': 'Jedna postupka musí byť bez žolíka',
  'canasta.round.deal': 'Rozdanie',
  'canasta.round.concealed': 'Vyšiel zakrytý',
  'canasta.round.exhausted': 'Balíček sa minul',
  'canasta.round.meldCards': 'Vyložené karty: {n}',
  'canasta.round.canastas': 'Canasty: {n}',
  'canasta.round.redThrees': 'Červené trojky: {n}',
  'canasta.round.goingOut': 'Vyjdenie: {n}',
  'canasta.round.inHand': 'Zostalo v ruke: {n}',
  'holdem.round.hand': 'Rozdanie',
  'holdem.round.pot': 'Bank {n}',
  'holdem.round.uncontested': 'Všetci ostatní zložili',
  'seat.ready': 'Pripravený',



  // --- refusals the other three games raise ---------------------------------
  // Worded here for the same reason Žolíky's are: reasonText falls back to the
  // code, and OfferBar hands it the code as that fallback, so an unworded
  // refusal reaches a player as PILE_FROZEN in capitals. serverKeys.test.ts
  // now fails the build rather than letting the next one through.
  'err.SET_TOO_LARGE': 'Skupina už má všetky štyri farby',
  'err.DISCARD_TAKEN_CARD_FORBIDDEN':
    'Kartu, ktorú si práve vzal, nemôžeš odhodiť — zahraj ju alebo si ju nechaj',
  'err.CARD_DOES_NOT_FIT': 'Táto karta nesedí ani farbou, ani hodnotou',
  'err.SUIT_REQUIRED': 'Povedz farbu, ktorá pokračuje',
  'err.MUST_ANSWER_DRAW_OR_TAKE': 'Odpovedz sedmičkou, alebo si vezmi karty',
  'err.NOTHING_TO_DRAW': 'Nie je už čo ťahať',
  'err.PILE_EMPTY': 'Kôpka je prázdna',
  'err.PILE_BLOCKED': 'Kôpka je zablokovaná — navrchu leží čierna trojka',
  'err.PILE_FROZEN': 'Kôpka je zamrznutá — potrebuješ dve prirodzené karty v hodnote vrchnej karty',
  'err.TOP_CARD_UNUSABLE': 'Vrchnú kartu nemôžeš použiť',
  'err.MELD_CLOSED': 'Táto kombinácia je úplná a uzavretá',
  'err.MELD_TOO_SMALL': 'Kombinácia potrebuje viac kariet',
  'err.MELD_TOO_LARGE': 'Táto kombinácia už neprijme ďalšie karty',
  'err.MELD_MIXED_RANKS': 'Všetky karty v kombinácii musia mať rovnakú hodnotu',
  'err.NOT_ENOUGH_NATURALS': 'Kombinácia potrebuje viac prirodzených kariet než žolíkov',
  'err.RANK_ALREADY_MELDED': 'Tvoja strana už má kombináciu tejto hodnoty',
  'err.NOT_YOUR_MELD': 'Táto kombinácia patrí súperovej strane',
  'err.NO_SUCH_MELD': 'Táto kombinácia nie je na stole',
  'err.CANNOT_MELD_THREE': 'Trojky sa nikdy nevykladajú',
  'err.CANNOT_DISCARD_RED_THREE': 'Červenú trojku nemožno odhodiť',
  'err.MUST_KEEP_A_CARD': 'Nechaj si aspoň jednu kartu — takto si ruku nevyprázdniš',
  'err.MUST_MELD_FIRST': 'Najprv vylož prvú kombináciu svojej strany',
  'err.INITIAL_MELD_NOT_MET': 'Tvojmu prvému vyloženiu stále chýbajú body',
  'err.CANNOT_GO_OUT_YET': 'Tvoja strana potrebuje dokončenú canastu, kým môže vyjsť',
  'err.NOTHING_TO_CALL': 'Nie je žiadna stávka na dorovnanie',
  'err.CANNOT_CHECK': 'Nemôžeš checkovať — je tu stávka na odpoveď',
  'err.CANNOT_RAISE': 'Tu nemôžeš zvyšovať',
  'err.RAISE_TOO_SMALL': 'Zvýšenie musí byť aspoň také ako to predchádzajúce',
  'err.NOT_ENOUGH_CHIPS': 'Toľko žetónov nemáš',
  'err.AMOUNT_REQUIRED': 'Povedz koľko',
  'err.AMOUNT_NOT_A_NUMBER': 'Táto suma nie je číslo',
  'err.SEAT_NOT_IN_HAND': 'V tomto rozdaní nie si',
  'err.WRONG_RANK': 'Táto karta má na to nesprávnu hodnotu',
  'err.MATCH_FULL': 'Stôl je plný',
  'err.MATCH_ALREADY_STARTED': 'Zápas sa už začal',
  'err.TOO_FEW_PLAYERS': 'Zatiaľ nie je dosť hráčov',
  'err.WRONG_PLAYER_COUNT': 'Túto hru sa s toľkými hráčmi hrať nedá',
  'err.NOT_THE_HOST': 'To môže len hostiteľ',
  'err.NO_LONGER_WAITING': 'Stôl už nečaká',
  'err.WAITING_ROOM_UNAVAILABLE': 'Čakáreň nie je dostupná',
  'err.SERVER_BUSY': 'Server je práve plný — skús to o chvíľu znova',


  // --- rules the engine has always enforced and never stated ----------------
  // Every one of these was found by the guardrail rather than by review: the
  // validator could refuse a player for it, and no sentence anywhere said so.
  // See server/internal/zolikmod/ruleindex_test.go.
  'zolik.rules.section.layoff': 'Prikladanie ku kombináciám',
  'zolik.rules.pickup.obligation':
    'Kým nie si vyložený, karta vzatá z odhadzovacieho balíčka musí byť použitá v kombinácii, ktorou sa v tomto ťahu vykladáš.',
  'zolik.rules.pickup.noReturn':
    'Kartu vzatú z odhadzovacieho balíčka nemožno v tom istom ťahu znova odhodiť — zahraj ju alebo si ju nechaj.',
  'zolik.rules.wilds.setLimit': 'Skupina nesmie obsahovať viac žolíkov než prirodzených kariet.',
  'zolik.rules.set.maxSize':
    'Skupina nesmie mať viac ako {n} kariet — žolík nahrádza chýbajúcu farbu, nedopĺňa úplnú skupinu.',
  'zolik.rules.run.maxLength':
    'Postupka nesmie mať viac ako {n} kariet — eso dole, dvanásť hodnôt nad ním a eso hore.',
  'zolik.rules.run.aceBridge':
    'Eso stojí nad kráľom alebo pod dvojkou, nikdy ako most medzi oboma koncami postupky.',
  'zolik.rules.contracts.contribution':
    'Kým nie si vyložený, každá vyložená kombinácia musí byť tá, ktorú kontrakt rozdania ešte vyžaduje.',
  'zolik.rules.layoff.afterDown': 'K cudzím kombináciám nemôžeš prikladať, kým nevyložíš vlastný kontrakt.',
  'zolik.rules.layoff.runEnds': 'Karta priložená k postupke ju musí predĺžiť na jednom alebo druhom konci.',
  'zolik.rules.jokers.swap': 'Žolíka v kombinácii na stole možno vykúpiť presne tou kartou, ktorú zastupuje.',
  'zolik.rules.jokers.reclaim.on':
    'Žolík vykúpený zo stola musí byť v tom istom ťahu zahraný do kombinácie — nesmie zostať v ruke.',
  'zolik.rules.jokers.reclaim.off': 'Žolík vykúpený zo stola môže zostať v ruke.',
  'zolik.rules.deck.reshuffle':
    'Keď sa balíček minie, odhadzovací balíček sa zamieša a stane sa novým balíčkom; ak sú prázdne oba, rozdanie sa končí.',


  // --- what to do instead ---------------------------------------------------
  // The third layer of a refusal, after the reason and the rule. Sent by the
  // module because it is the only side that knows which card is owed and
  // which way out is on offer; see server/internal/zolikmod/remedy.go.
  'zolik.remedy.meldThePickup': 'Pridaj {card} k svojmu vykladaniu, alebo vráť zobratie späť.',
  'zolik.remedy.discardSomethingElse': 'Odhoď inú kartu, alebo zahraj {card} v tomto ťahu.',
  'zolik.remedy.discardNotAJoker': 'Odhoď niečo iné než žolíka.',
  'zolik.remedy.finishOrUndoLayDown': 'Dokonči vykladanie, alebo ho vezmi späť.',
  'zolik.remedy.needMorePoints': 'Chýba ti ešte {n} bodov, aby si sa mohol vyložiť.',
  'zolik.remedy.layACleanRun': 'Vylož postupku bez žolíka.',
  'zolik.remedy.playReclaimedJoker': 'Zahraj {card} do kombinácie, alebo vráť zobratie späť.',
  'zolik.remedy.goDownFirst': 'Najprv vylož vlastné kombinácie.',
  'zolik.remedy.drawFirst': 'Najprv si potiahni kartu.',
  'zolik.remedy.drawFromStock': 'Ťahaj z balíčka — odhadzovací balíček sa otvára v kole {n}.',
  'zolik.remedy.drawFromStockEmpty': 'Ťahaj radšej z balíčka.',


  // --- keys whose meaning lives in their params -----------------------------
  // The shape fallback renders these with the number missing ("Stack", "Pot"),
  // which is why they earn a line where most keys do not.
  'header.contract': 'Vyžaduje {sets} skupín a {runs} postupiek',
  'header.contract.cleanRunOnly': 'Vyžaduje postupku bez žolíka',
  'header.round': 'Kolo {n}',
  'header.deck': 'Balíček',
  'header.target': 'Cieľ',
  'header.suitInPlay': 'Farba v hre',
  'seat.cards': 'Karty',
  'zolik.offer.meld': 'Kombinácia',
  'prompt.pickupMustBeMelded':
    '{value} prišla z odhadzovacieho balíčka — musí ísť do kombinácií, ktorými sa v tomto ťahu vykladáš.',
  'prompt.jokerMustBePlayed': '{value} prišla zo stola — musí ísť do kombinácie, než budeš môcť ukončiť ťah.',
  'prompt.initialMeld': 'Prvé vyloženie tvojej strany musí dosiahnuť {n} bodov.',
  'prompt.canastasNeeded': 'Tvojej strane chýbajú ešte {n} canasty, aby mohla vyjsť.',
  'prompt.mustDrawOrAnswerSeven': 'Odpovedz sedmičkou, alebo si potiahni {n} kariet.',
  'prompt.chooseSuit': 'Vyber farbu, ktorá pokračuje',
  'prompt.skipPending': 'Tvoj ťah sa vynecháva',
  'status.lastDeal': 'Tím {team} získal {value}',
  'status.teamScore': 'Tím {team}: {value}',
  'canasta.offer.rank': 'Hodnota',
  'canasta.seat.teamScore': 'Skóre tímu',
  'canasta.seat.canastas': 'Canasty',
  'holdem.header.pot': 'Bank',
  'holdem.header.street': 'Ulica',
  'holdem.header.hand': 'Rozdanie',
  'holdem.header.handLimit': 'Rozdaní spolu',
  'holdem.header.blinds': 'Blindy',
  'holdem.cost.call': 'na dorovnanie',
  'holdem.cost.pot': 'v banku',
  'holdem.seat.stack': 'Stack',
  'holdem.seat.bet': 'Stávka',
  'holdem.prompt.yourAction': 'Si na rade',
  'holdem.prompt.raiseTo': 'Zvýšiť na',
  'zone.yourHand': 'Tvoja ruka',
  'zone.opponentHand': 'Súperova ruka',
  'zone.drawPile': 'Ťahací balíček',
  'zone.discardPile': 'Odhadzovací balíček',
  'zone.melds': 'Kombinácie',
  'zone.teamMelds': 'Kombinácie tvojej strany',
  'zone.redThrees': 'Červené trojky',
  'zone.board': 'Stôl',
  'verb.drawFromDeck': 'Potiahnuť',
  'verb.takeFromDiscard': 'Vziať z kôpky',


  // --- the why sheet's own furniture ---------------------------------------
  // The three layers a refusal is explained in. Labels, not sentences: what
  // goes under each is sent by the server or worded above.
  'why.reason': 'Prečo nie',
  'why.rule': 'Pravidlo',
  'why.rules': 'Pravidlá',
  'why.remedy': 'Čo môžeš urobiť',
  'why.readTheRules': 'Prečítať celé pravidlá →',
  'why.close': 'Zavrieť',
  'why.open': 'prečo',

  // --- marks on a particular card ------------------------------------------
  // A mark is a refusal that has not happened yet: the rule is enforced at the
  // discard, which is the last possible moment to hear about it, so the card
  // says so while there is still a turn left to act on it.
  'zolik.badge.owedToMeld':
    '{card} prišla z odhadzovacieho balíčka — musí ísť do kombinácií, ktorými sa v tomto ťahu vykladáš.',
  'zolik.badge.jokerOwed': '{card} prišla zo stola — musí ísť do kombinácie, než budeš môcť ukončiť ťah.',

  // --- the legal notices ----------------------------------------------------
  // Only the furniture. The documents themselves are in `src/legal`, which is
  // a bundle of the same kind with a parity test of its own — prose that long
  // in a flat key map buries the keys this one exists for.
  //
  // The notice is five fragments rather than one sentence with two links glued
  // in, because Czech does not put the link where English does: "souhlasíš s
  // Podmínkami" inflects the noun the link is made of. Fragments let each
  // locale place and decline its own.
  'legal.terms': 'Podmienky',
  'legal.privacy': 'Súkromie',
  'legal.source': 'Zdrojový kód',
  'legal.updated': 'Verzia {version}',
  'legal.draft':
    'Návrh — zatiaľ neplatí. Meno, krajina a kontaktná adresa prevádzkovateľa sa ešte majú doplniť.',
  'legal.notice.before': 'Hraním súhlasíš s ',
  'legal.notice.terms': 'podmienkami používania',
  'legal.notice.between': '. Čo sa o tebe ukladá, nájdeš v ',
  'legal.notice.privacy': 'oznámení o ochrane súkromia',
  'legal.notice.after': '.',
  // --- gin rummy -------------------------------------------------------------
  'err.UPCARD_DECLINED': 'Túto kartu si už odmietol',
  'err.DEADWOOD_TOO_HIGH': 'Tvoj deadwood je príliš vysoký na klepnutie',
  'err.CARD_DOES_NOT_EXTEND_MELD': 'Táto karta túto kombináciu nepredlžuje',
  'ginrummy.rules.setup': 'Príprava',
  'ginrummy.rules.turn': 'Tvoj ťah',
  'ginrummy.rules.melds': 'Kombinácie',
  'ginrummy.rules.knocking': 'Klepnutie',
  'ginrummy.rules.bigGin': 'Big gin',
  'ginrummy.rules.layoff': 'Prikladanie',
  'ginrummy.rules.deadHand': 'Mŕtve rozdanie',
  'ginrummy.rules.scoring': 'Bodovanie rozdania',
  'ginrummy.rules.match': 'Výhra v zápase',
  'ginrummy.rules.lineBonuses': 'Bonusy v zúčtovaní',
  'ginrummy.rules.deck': 'Hrá sa s balíčkom {value} kariet.',
  'ginrummy.rules.deal': 'Každý hráč dostane {value} kariet.',
  'ginrummy.rules.upcard': 'Ešte jedna karta sa otočí lícom nahor a začne odhadzovací balíček.',
  'ginrummy.rules.drawDiscard':
    'V svojom ťahu si potiahni jednu kartu — z balíčka alebo z odhadzovacieho balíčka — a potom jednu odhoď.',
  'ginrummy.rules.setsAndRuns':
    'Kombinácia je skupina troch alebo štyroch kariet jednej hodnoty, alebo postupka troch a viac kariet jednej farby.',
  'ginrummy.rules.aceLow': 'Eso je vždy nízke — postupka od dámy po eso neexistuje.',
  'ginrummy.rules.knockLimit': 'Klepnúť môžeš, len čo je tvoj deadwood {n} alebo menej.',
  'ginrummy.rules.oklahoma': 'Hranicu klepnutia v tomto rozdaní určuje hodnota otočenej karty.',
  'ginrummy.rules.gin': 'Nulový deadwood je gin — najlepšie možné klepnutie.',
  'ginrummy.rules.bigGinBonus':
    'Jedenásť kariet v kombináciách bez akéhokoľvek odhodenia je big gin a prináša ďalších {n} bodov.',
  'ginrummy.rules.layoffDescription':
    'Po klepnutí, ktoré nie je gin, môže súper priložiť vlastný deadwood k tvojim kombináciám, než sa ruky porovnajú.',
  'ginrummy.rules.deadHandDescription':
    'Ak balíček klesne na posledné dve karty a nikto neklepol, rozdanie je mŕtve — nikto neboduje a ten istý rozdávajúci rozdáva znova.',
  'ginrummy.rules.undercut':
    'Ak súperov deadwood nie je vyšší než tvoj, podreže ťa: zapíše si rozdiel plus {n}.',
  'ginrummy.rules.ginBonus': 'Gin zapíše celú súperovu ruku plus {n}.',
  'ginrummy.rules.target': 'Kto po skončení rozdania prvý prekročí {n} bodov, vyhráva zápas.',
  'ginrummy.rules.shutout': 'Bonus za zápas sa zdvojnásobí na {n}, ak porazený nezískal ani jeden bod.',
  'ginrummy.rules.box': 'Každé vyhraté rozdanie má na konci zápasu hodnotu {n} bodov.',
  'ginrummy.rules.gameBonus': 'Výhra v zápase prináša ďalších {n} bodov.',
  'ginrummy.fact.deadwood': 'deadwood {value}',
  'ginrummy.fact.discardCard': 'Odhodiť {value}',
  'ginrummy.fact.meldCards': 'Na {value}',
  'ginrummy.header.hand': 'Rozdanie {n}',
  'ginrummy.round.kind': '{value}',
  'ginrummy.round.hand': 'Rozdanie',
  'ginrummy.seat.score': '{value}',
  'ginrummy.seat.dealer': 'Rozdávajúci',
  'ginrummy.status.knocked': '{playerId} klepol s deadwoodom {deadwood}',
  'ginrummy.status.gin': '{playerId} dal gin',
  'ginrummy.status.lastHand': 'Posledné rozdanie: {winner} ({kind}, {delta} b.)',
  'ginrummy.offer.drawStock': 'Ťahať z balíčka',
  'ginrummy.offer.drawDiscard': 'Ťahať z odhadzovacieho balíčka',
  'ginrummy.offer.takeUpcard': 'Vziať otočenú kartu',
  'ginrummy.offer.passUpcard': 'Pas',
  'ginrummy.offer.discard': 'Odhodiť',
  'ginrummy.offer.knock': 'Klepnúť',
  'ginrummy.offer.gin': 'Gin!',
  'ginrummy.offer.bigGin': 'Big gin!',
  'ginrummy.offer.layOff': 'Priložiť',
  'ginrummy.offer.finishLayoff': 'Koniec prikladania',
  'ginrummy.zone.knockerHand': 'Ruka toho, kto klepol',
  'ginrummy.zone.melds': 'Kombinácie',
  'ginrummy.prompt.upcardDecision': 'Vezmi otočenú kartu, alebo pasuj',
  'ginrummy.prompt.yourTurnDraw': 'Potiahni kartu',
  'ginrummy.prompt.yourTurnDiscard': 'Odhoď — alebo klepni, ak môžeš',
  'ginrummy.prompt.layoff': 'Prilož deadwood, alebo ukonči',

  // --- rummy tiles -------------------------------------------------------------
  'err.TILE_NOT_IN_HAND': 'Tento kameň v ruke nemáš',
  'err.TILE_DOES_NOT_FIT': 'Tam to nepasuje',
  'err.NO_SUCH_SET': 'Táto kombinácia nie je na stole',
  'err.INITIAL_MELD_ONLY': 'Pred prvým vyložením môžeš prerovnávať iba vlastné nové kombinácie',
  'err.TABLE_NOT_VALID': 'Stôl zatiaľ nie je platný',
  'err.TRAY_NOT_EMPTY': 'Máš ešte voľné kamene na umiestnenie',
  'err.NOTHING_PLAYED': 'Zahraj aspoň jeden kameň, než ukončíš ťah',
  'err.INITIAL_MELD_TOO_LOW': 'Tvoje prvé vyloženie musí mať hodnotu aspoň 30 bodov',
  'err.NOT_A_RUN': 'Rozdeliť možno len postupku',
  'err.BAD_SPLIT_POSITION': 'Na tomto mieste sa táto postupka rozdeliť nedá',
  'err.NO_JOKER_IN_SET': 'V tejto kombinácii žiadny žolík nie je',
  'err.TILE_JOKER_SWAP_MISMATCH': 'Tento kameň nie je to, čo žolík zastupuje',
  'rummytiles.rules.setup': 'Príprava',
  'rummytiles.rules.sets': 'Kombinácie',
  'rummytiles.rules.initialMeld': 'Prvé vyloženie',
  'rummytiles.rules.turn': 'Tvoj ťah',
  'rummytiles.rules.jokerTaking': 'Zobratie žolíka',
  'rummytiles.rules.ending': 'Ukončenie kola',
  'rummytiles.rules.poolExhaustion': 'Ak sa banka vyčerpá',
  'rummytiles.rules.match': 'Výhra v zápase',
  'rummytiles.rules.tiles': 'Hrá sa s {value} kameňmi.',
  'rummytiles.rules.dealCount': 'Každý hráč dostane {value} kameňov.',
  'rummytiles.rules.group': 'Skupina sú tri alebo štyri kamene s rovnakým číslom, každý v inej farbe.',
  'rummytiles.rules.run': 'Postupka sú tri a viac po sebe idúcich čísel v jednej farbe.',
  'rummytiles.rules.noWrap': 'Po 13 sa nezačína znova od 1.',
  'rummytiles.rules.joker': 'Žolík zastupuje ktorýkoľvek kameň.',
  'rummytiles.rules.initialMeldDescription':
    'Kým nevyložíš {n} alebo viac bodov v jedinom ťahu, výlučne z vlastnej ruky, nesmieš siahnuť na nič, čo už na stole leží.',
  'rummytiles.rules.turnDescription':
    'Zahraj aspoň jeden kameň z ruky, stôl pritom preskladávaj voľne, a skonči tak, aby každá kombinácia na stole bola platná.',
  'rummytiles.rules.noDiscard':
    'Neodhadzuje sa — ak nedokážeš dokončiť platný ťah, potiahneš si namiesto toho jeden kameň.',
  'rummytiles.rules.jokerTakingDescription':
    'Žolíka na stole môžeš zobrať tak, že ho nahradíš kameňom, ktorý zastupuje, z vlastnej ruky — a musíš ho použiť v kombinácii ešte pred koncom ťahu.',
  'rummytiles.rules.goingOut':
    'Kolo vyhráva prvý hráč, ktorému dôjdu kamene. Všetci ostatní si zapíšu zápornú hodnotu toho, čo im zostalo; víťaz si zapíše súčet strát všetkých ostatných.',
  'rummytiles.rules.poolExhaustionLowestWins':
    'Ak sa banka vyčerpá a nikto nemôže hrať, kolo sa končí a vyhráva ho najnižšia hodnota ruky.',
  'rummytiles.rules.poolExhaustionNoWinner':
    'Ak sa banka vyčerpá a nikto nemôže hrať, kolo sa končí bez víťaza — každá ruka sa jednoducho spočíta.',
  'rummytiles.rules.target': 'Kto po skončení kola prvý prekročí {n} bodov, vyhráva zápas.',
  'rummytiles.rules.roundLimit': 'Zápas sa končí po {n} kolách — vyhráva najvyššie skóre.',
  'rummytiles.fact.setCards': '{value}',
  'rummytiles.header.pool': 'Banka {n}',
  'rummytiles.header.round': 'Kolo {n}',
  'rummytiles.round.kind': '{value}',
  'rummytiles.round.round': 'Kolo',
  'rummytiles.seat.score': '{value}',
  'rummytiles.seat.notOpened': 'Neotvorené',
  'rummytiles.status.lastRound': 'Posledné kolo: {winner} ({kind})',
  'rummytiles.badge.invalid': 'Zatiaľ neplatné',
  'rummytiles.zone.pool': 'Banka',
  'rummytiles.zone.table': 'Stôl',
  'rummytiles.zone.tray': 'Stojan',
  'rummytiles.offer.place': 'Položiť',
  'rummytiles.offer.addFromHand': 'Pridať',
  'rummytiles.offer.addFromTray': 'Pridať zo stojana',
  'rummytiles.offer.take': 'Vziať',
  'rummytiles.offer.split': 'Rozdeliť',
  'rummytiles.offer.swapJoker': 'Vymeniť žolíka',
  'rummytiles.offer.resetTurn': 'Vrátiť ťah',
  'rummytiles.offer.commit': 'Hotovo',
  'rummytiles.offer.draw': 'Potiahnuť',
  'rummytiles.param.position': 'Rozdeliť na',

  // --- blackjack -----------------------------------------------------------
  //
  // The house rules are worded twice over — on and off — because a rule that
  // is *not* in force is still something a player has to be told. "No
  // surrender at this table" is information; silence is a guess.
  'err.BET_BELOW_MINIMUM': 'To je pod minimom stola',
  'err.ALREADY_BET': 'Tvoja stávka už stojí',
  'err.INSURANCE_CLOSED': 'Práve teraz nie je poistenie k dispozícii',
  'err.CANNOT_DOUBLE': 'Toto rozdanie sa zdvojnásobiť nedá',
  'err.CANNOT_SPLIT': 'Toto rozdanie sa rozdeliť nedá',
  'err.CANNOT_SURRENDER': 'Toto rozdanie sa vzdať nedá',

  'blackjack.rules.section.table': 'Stôl',
  'blackjack.rules.section.play': 'Hranie rozdania',
  'blackjack.rules.section.dealer': 'Krupiér',
  'blackjack.rules.section.end': 'Ako sa zápas končí',
  'blackjack.rules.goal':
    'Poraz krupiéra bez toho, aby si prekročil dvadsaťjeden. Kto prekročí, prehráva okamžite, nech krupiér potom urobí čokoľvek.',
  // Counts are their own phrase rather than a number glued to a noun: one
  // deck and six decks inflect differently, and Czech inflects them again.
  'blackjack.rules.decks': 'Balíčky v botníku: {n}.',
  'blackjack.rules.stack': 'Každé miesto si sadá s {n} žetónmi.',
  'blackjack.rules.minBet': 'Minimum stola je {n} žetónov.',
  'blackjack.rules.faceUp':
    'Karty hráčov sa rozdávajú lícom nahor; krupiér drží jednu kartu zakrytú, kým všetci nedohrajú.',
  'blackjack.rules.hitStand': 'Ťahaj toľko kariet, koľko chceš, alebo zostaň s tým, čo máš.',
  'blackjack.rules.aces': 'Eso sa počíta ako jedenásť, kým sa to zmestí, a inak ako jedna.',
  'blackjack.rules.blackjack': 'Eso s kartou v hodnote desať na prvých dvoch kartách je blackjack.',
  'blackjack.rules.pays3to2': 'Blackjack platí 3:2.',
  'blackjack.rules.pays6to5': 'Blackjack platí 6:5.',
  'blackjack.rules.paysEven': 'Blackjack platí jedna k jednej.',
  'blackjack.rules.double':
    'Na prvých dvoch kartách môžeš zdvojnásobiť stávku a vziať si presne jednu ďalšiu kartu.',
  'blackjack.rules.doubleAfterSplit': 'Rozdanie vzniknuté rozdelením možno tiež zdvojnásobiť.',
  'blackjack.rules.noDoubleAfterSplit': 'Rozdanie vzniknuté rozdelením zdvojnásobiť nemožno.',
  'blackjack.rules.split':
    'Dve karty rovnakej hodnoty možno rozdeliť do samostatných rozdaní, každé s vlastnou stávkou — až {n}-krát, spolu na {hands} rozdaní.',
  'blackjack.rules.noSplit': 'Pri tomto stole sa páry nerozdeľujú.',
  'blackjack.rules.splitAces':
    'Rozdelené esá dostanú po jednej karte a potom zostávajú stáť, a dvadsaťjeden získané takto nie je blackjack.',
  'blackjack.rules.surrender':
    'Prvé rozdanie sa môžeš vzdať za polovicu stávky, keď už krupiér skontroloval blackjack.',
  'blackjack.rules.noSurrender': 'Pri tomto stole sa rozdaní vzdať nemožno.',
  'blackjack.rules.dealerDraws': 'Krupiér ťahá do sedemnástich a potom zostáva stáť.',
  'blackjack.rules.hitsSoft17': 'Krupiér ťahá aj pri sedemnástich vytvorených s esom.',
  'blackjack.rules.standsSoft17': 'Krupiér zostáva stáť pri sedemnástich vytvorených s esom.',
  'blackjack.rules.dealerPeeks':
    'S esom alebo desiatkou navrchu krupiér skontroluje blackjack, než ktokoľvek zahrá.',
  'blackjack.rules.insurance':
    'Proti krupiérovmu esu sa môžeš poistiť za polovicu stávky; platí 2:1, ak má krupiér blackjack.',
  'blackjack.rules.noInsurance': 'Pri tomto stole sa poistenie neponúka.',
  'blackjack.rules.rounds': 'Pri stole sa hrá {n} kôl.',
  'blackjack.rules.mostChipsWins': 'Kto má na konci najviac žetónov, vyhráva zápas.',
  'blackjack.rules.bustedOut': 'Miesto, ktoré už nedokáže pokryť minimum {n}, zvyšok zápasu sedí bokom.',

  'blackjack.zone.dealer': 'Krupiér',
  'blackjack.zone.box': 'Rozdanie',
  'blackjack.zone.yourBox': 'Tvoje rozdanie',
  'blackjack.zone.shoe': 'Botník',

  'blackjack.header.round': 'Kolo {n} z {of}',
  'blackjack.header.minBet': 'Minimum',
  'blackjack.header.decks': 'Balíčky',
  'blackjack.header.dealerTotal': 'Krupiér ukazuje {n}',
  'blackjack.header.dealerSoftTotal': 'Krupiér ukazuje mäkkých {n}',

  'blackjack.seat.stack': 'Žetóny',
  'blackjack.seat.bet': 'Stávka',
  'blackjack.seat.insurance': 'Poistenie',
  'blackjack.seat.total': 'Spolu',
  'blackjack.seat.softTotal': 'Mäkký súčet',
  'blackjack.seat.out': 'Bez žetónov',

  'blackjack.prompt.placeBet': 'Polož stávku',
  'blackjack.prompt.insurance': 'Poistenie?',
  'blackjack.prompt.yourMove': 'Si na rade',
  'blackjack.prompt.waitingFor': 'Čaká sa na {playerId}',
  'blackjack.prompt.betAmount': 'Stávka',

  'blackjack.offer.bet': 'Staviť',
  'blackjack.offer.hit': 'Kartu',
  'blackjack.offer.stand': 'Stojím',
  'blackjack.offer.double': 'Zdvojnásobiť',
  'blackjack.offer.split': 'Rozdeliť',
  'blackjack.offer.surrender': 'Vzdať sa',
  'blackjack.offer.insure': 'Poistiť sa',
  'blackjack.offer.declineInsurance': 'Bez poistenia',

  'blackjack.fact.tableMinimum': 'minimum',
  'blackjack.fact.insuranceCost': 'za poistenie',
  'blackjack.fact.extraStake': 'na stávku',
  'blackjack.fact.surrenderReturn': 'späť',

  'blackjack.status.dealerBlackjack': 'Krupiér mal blackjack',
  'blackjack.status.dealerBust': 'Krupiér preťahol s {n}',
  'blackjack.status.dealerStands': 'Krupiér zostáva na {n}',

  'blackjack.round.name': 'Kolo',
  'blackjack.round.dealerTotal': 'Krupiér {n}',
  'blackjack.round.dealerBust': 'Krupiér preťahol ({n})',
  'blackjack.round.dealerBlackjack': 'Krupiérov blackjack',
  'blackjack.round.outcome.blackjack': 'Blackjack',
  'blackjack.round.outcome.win': 'Výhra',
  'blackjack.round.outcome.push': 'Remíza',
  'blackjack.round.outcome.lose': 'Prehra',
  'blackjack.round.outcome.bust': 'Preťah',
  'blackjack.round.outcome.surrender': 'Vzdané',

  'blackjack.badge.inPlay': 'V hre',
  'blackjack.badge.doubled': 'Zdvojnásobené',
  'blackjack.badge.split': 'Rozdelené',
  'blackjack.badge.blackjack': 'Blackjack',
  'blackjack.badge.bust': 'Preťah',
  'blackjack.badge.won': 'Výhra',
  'blackjack.badge.push': 'Remíza',
  'blackjack.badge.lost': 'Prehra',
  'blackjack.badge.surrendered': 'Vzdané',

  'blackjack.unit.chips': 'žetónov',

  // --- the settings screen --------------------------------------------------
  //
  // Hardcoded English until now, which was survivable while the only other
  // language was unreachable and faintly absurd once the language picker
  // itself lived on this screen: a player who cannot read "Settings" is
  // exactly the one who came here to change it.
  'settings.title': 'Nastavenia',
  'settings.subtitle': 'Ako vyzeráš ty a ako stôl',
  'settings.face.heading': 'Tvoja tvár pri stole',
  'settings.face.account': 'Uložené k tvojmu účtu, takže ťa sprevádza aj na iné zariadenie.',
  'settings.face.device': 'Uložené v tomto zariadení. Prihlás sa, aby šlo s tebou.',
  'settings.skin.heading': 'Vzhľad stola',
  'settings.language.heading': 'Jazyk',
  'settings.language.status': 'Uložené v tomto zariadení.',
  'settings.language.auto': 'Automaticky',
  'settings.language.auto.now': 'Podľa zariadenia — teraz {language}',
  'settings.legal.heading': 'Drobným písmom',
  'settings.legal.status': 'S čím si hraním súhlasil a čo sa o tebe ukladá.',
  'settings.signIn': 'Prihlásiť sa',
  'settings.back': 'Späť',

  // A translated interface over an English notice is a worse lie than an
  // English interface, so the document says so itself rather than letting the
  // surrounding screen imply the text was written for this reader.
  'legal.untranslated':
    'Toto oznámenie zatiaľ nebolo preložené do tvojho jazyka. Platí anglické znenie nižšie.',

  // --- the navigation bar's screen titles ------------------------------------
  //
  // These sit in `app/_layout.tsx`, above every screen, and were the last
  // English left after the settings screen was translated — a German player
  // reading "Einstellungen" under a bar that said "Settings". A translated app
  // with an untranslated chrome bar looks like a bug, not like a limit.
  'nav.home': 'Zolik',
  'nav.emailSignIn': 'Prihlásenie e-mailom',
  'nav.signingIn': 'Prihlasovanie',
  'nav.usernameSignIn': 'Prihlásenie menom',
  'nav.legacyAccount': 'Starý účet',
  'nav.guest': 'Hosť',
  'nav.account': 'Účet',
  'nav.games': 'Hry',
  'nav.table': 'Tvoj stôl',
  'nav.join': 'Pripojiť sa k stolu',
  'nav.rules': 'Pravidlá',
  'nav.match': 'Zápas',
  'nav.scoreTable': 'Tabuľka skóre',
  'nav.stats': 'Štatistiky',
};
