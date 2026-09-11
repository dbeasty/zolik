/**
 * Estonian. Rummy vocabulary: grupp for a set, jada for a run, kombinatsioon for a meld, tõmbepakk and viskepakk for the two piles.
 */

export const et: Record<string, string> = {
  // --- engine error codes (rules.RulesErrorCode) ---------------------------
  'err.NOT_YOUR_TURN': 'Praegu ei ole sinu kord',
  'err.WRONG_PHASE': 'Praegu ei saa',
  'err.MUST_DRAW_FIRST': 'Tõmba kaart, enne kui välja paned',
  'err.GAME_SUSPENDED': 'Mäng on pausil',
  'err.GAME_NOT_ACTIVE': 'Mäng ei käi',
  // --- runtime refusals, which are not the rules saying no -----------------
  //
  // Both were reachable long before they were worded, and a player who met
  // one was shown the code in capitals or, worse, nothing at all: an action
  // sent while the socket was down simply vanished, which reads as a legal
  // move being silently refused.
  'err.MATCH_NOT_ACTIVE': 'Laud on pausil — ootame mängija taasühendumist',
  'err.NOT_CONNECTED': 'Lauaga puudub ühendus — ühendame uuesti, siis proovi jälle',
  // Why the "start the next round" control is greyed out. Without wording the
  // shell printed the code itself, so a player agreeing to go on was answered
  // with ALREADY_READY in capitals.
  'err.ALREADY_READY': 'Oled valmis',
  'err.NOT_BETWEEN_ROUNDS': 'Voor käib veel',
  'err.NOT_AT_THIS_TABLE': 'Sa ei ole selle laua taga',
  // Bringing a table back that the sweeper set aside. Refusals a player
  // can actually provoke: a link to a table that is gone, one that was
  // already picked up in another tab, and a table with other people at it.
  'err.MATCH_MOVED_ON': 'Laud on edasi liikunud — laadi leht uuesti',
  'err.MATCH_NOT_ABANDONED': 'See laud ei oota jätkamist',
  'err.MATCH_NOT_FOUND': 'Seda lauda enam ei ole',
  'err.TABLE_HAS_OTHER_PLAYERS': 'Taastada saab ainult laua, kus kõik teised on robotid',
  'err.DISCARD_LOCKED': 'Viskepakk on esialgu lukus',
  'err.DISCARD_PILE_EMPTY': 'Viskepakk on tühi',
  'err.NO_CARDS_LEFT': 'Tõmmata pole enam midagi',
  'err.ROUND_REQ_NOT_MET': 'Pane esmalt välja oma avang',
  'err.NEED_CLEAN_RUN': 'Selleks et sind loetaks väljas olevaks, on sul lauale vaja jokkerita jada',
  'err.INCOMPLETE_INITIAL_MELD': 'Lõpeta väljapanek või võta see tagasi, enne kui viskad',
  'err.DISCARD_CARD_NOT_MELDED': 'Võetud kaart peab minema sinu kombinatsiooni',
  'err.JOKER_DISCARD_FORBIDDEN': 'Jokkerit ei tohi ära visata',
  'err.NOTHING_TO_UNDO': 'Midagi pole tagasi võtta',
  'err.NO_JOKER_IN_MELD': 'Selles kombinatsioonis pole jokkerit',
  'err.JOKER_SWAP_MISMATCH': 'See kaart ei asu jokkeri kohale',
  'err.RECLAIMED_JOKER_NOT_MELDED': 'Laualt võetud jokker tuleb selles käigus kombinatsiooni mängida',
  'err.RUN_TOO_LONG': 'See jada on juba täispikk',
  'err.WRONG_RUN_END': 'See kaart pikendab jada teist otsa',
  'err.INVALID_MELD': 'Ükski kaart sinu käes siia ei sobi',
  'err.CARD_NOT_IN_HAND': 'Seda kaarti pole sinu käes',
  'err.MELD_BELOW_MINIMUM': 'Sinu kombinatsioonidel jääb väljapanekuks veel punkte puudu',
  'err.MELD_NO_CONTRIBUTION': 'See kombinatsioon ei vii sinu nõuet edasi',
  'err.TOO_MANY_WILDS': 'Selles kombinatsioonis on liiga palju jokkereid',
  'err.ADJACENT_WILDS': 'Kaks jokkerit ei tohi olla kõrvuti',
  'err.ACE_BRIDGE': 'Äss ei saa siduda kuningat ja kahte',

  // --- contract phrasing ---------------------------------------------------
  // Built from the structured contract the server sends (sets/runs counts),
  // never from a deal-number lookup table.
  // Whole phrases per count rather than a number plus a pluralised noun:
  // Czech inflects the noun by count (skupina / skupiny / skupin) in a way no
  // "add an s" helper survives, and the counts that actually occur are 1-3.
  // A count outside that falls back to contract.sets.n.
  'contract.sets.1': 'Üks grupp',
  'contract.sets.2': 'Kaks gruppi',
  'contract.sets.3': 'Kolm gruppi',
  'contract.sets.n': '{n} gruppi',
  'contract.runs.1': 'Üks jada',
  'contract.runs.2': 'Kaks jada',
  'contract.runs.3': 'Kolm jada',
  'contract.runs.n': '{n} jada',
  'contract.any': 'Ükskõik milline kehtiv kombinatsioon',
  'contract.cleanRunOnly': 'Suvaline gruppide ja jadade segu — vähemalt üks jada peab olema jokkerita',
  'contract.cleanRunSuffix': '{base} — üks jada peab olema jokkerita',

  // --- written rules ---------------------------------------------------------
  // Full-sentence keys a "see the rules" screen renders, one per module,
  // resolved server-side against the table's actual variation and options —
  // see internal/module/rules.go and each game's rules.go. Shared section
  // titles first, then one block per module in the order the four games were
  // added.
  'zolik.rules.section.goal': 'Eesmärk',
  'zolik.rules.section.setup': 'Ettevalmistus',
  'zolik.rules.section.turn': 'Sinu käik',
  'zolik.rules.section.melding': 'Väljapanek',
  'zolik.rules.section.end': 'Kuidas matš lõpeb',
  'zolik.rules.goal':
    'Ole esimene, kes tühjendab oma käe, pannes välja kehtivaid gruppe ja jadu, ning kogu seejuures võimalikult vähe trahvipunkte kaartidest, mis sul veel käes on, kui keegi teine välja läheb.',
  'zolik.rules.deal': 'Iga mängija saab {n} kaarti.',
  'zolik.rules.meldShapes':
    'Grupp on {set}+ sama väärtusega kaarti; jada on {run}+ järjestikust sama masti kaarti.',
  'zolik.rules.turn.draw': 'Tõmba oma käigul üks kaart — tõmbepakist või viskepakist.',
  'zolik.rules.pickup.topOnly': 'Viskepakist tohib võtta ainult pealmise kaardi.',
  'zolik.rules.pickup.anyFromPile':
    'Viskepakist tohib võtta ükskõik millise kaardi koos kõige sellega, mis on selle peal.',
  'zolik.rules.pickup.locked': 'Viskepakist ei tohi tõmmata enne {n}. vooru.',
  'zolik.rules.pickup.open': 'Viskepakk on avatud esimesest voorust alates.',
  'zolik.rules.turn.discard': 'Lõpeta oma käik ühe kaardi äraviskamisega.',
  'zolik.rules.jokers.restricted':
    'Jokkerit ei tohi kunagi ära visata, välja arvatud siis, kui see on täpselt see kaart, mis sinu käe tühjendab.',
  'zolik.rules.lead.rotate':
    'Käigu algus nihkub igal jagamisel ühe koha võrra edasi, sõltumata sellest, kes võitis.',
  'zolik.rules.lead.winner': 'Kes välja läheb, alustab järgmist jagamist.',
  'zolik.rules.meldFloor.on':
    'Sinu esimene väljapanek peab kokku andma vähemalt {n} loomulikku punkti, enne kui oled väljas.',
  'zolik.rules.meldFloor.off': 'Esimesel väljapanekul pole punktide alammäära.',
  'zolik.rules.cleanRun.on':
    'Vähemalt üks sinu jadadest peab olema täiesti jokkerita, enne kui sind loetakse väljas olevaks.',
  'zolik.rules.cleanRun.off': 'Sinu jadad võivad jokkereid vabalt kasutada — ükski ei pea neist puhas olema.',
  'zolik.rules.contracts.rotating':
    'Matš kestab {n} jagamist ja iga jagamine nõuab oma gruppide ja jadade kombinatsiooni.',
  'zolik.rules.contracts.static': 'Iga jagamine nõuab sama kombinatsiooni: {sets} gruppi ja {runs} jada.',
  'zolik.rules.end.afterDeals': 'Matš lõpeb {n} jagamise järel.',
  'zolik.rules.end.atScore': 'Jagatakse edasi, kuni keegi jõuab {n} punktini — siis on läbi.',

  'prsi.rules.section.goal': 'Eesmärk',
  'prsi.rules.section.setup': 'Ettevalmistus',
  'prsi.rules.section.turn': 'Sinu käik',
  'prsi.rules.section.special': 'Erikaardid',
  'prsi.rules.section.end': 'Kuidas matš lõpeb',
  'prsi.rules.goal': 'Ole esimene, kes mängib välja kõik oma käes olevad kaardid.',
  'prsi.rules.deck': 'Mängitakse {value} kaardiga pakiga (seitsmest ülespoole).',
  'prsi.rules.deal': 'Iga mängija alustab {n} kaardiga.',
  'prsi.rules.turn.match':
    'Mängi kaart, mis sobib pealmise kaardi masti või väärtusega — või tõmba, kui ei saa.',
  'prsi.rules.turn.draw': 'Tõmbamine lõpetab su käigu ilma mängimata.',
  'prsi.rules.sevens': 'Mängi 7 ja järgmine mängija tõmbab kaks kaarti, kui ta ei vasta oma seitsmega.',
  'prsi.rules.aces': 'Mängi äss ja järgmise mängija käik jäetakse vahele.',
  'prsi.rules.queens': 'Mängi emand ja nimeta mast, mis jätkub.',
  'prsi.rules.end': 'Matš lõpeb hetkel, mil kellegi käsi on tühi.',

  'canasta.rules.section.goal': 'Eesmärk',
  'canasta.rules.section.setup': 'Ettevalmistus',
  'canasta.rules.section.melding': 'Väljapanek',
  'canasta.rules.section.end': 'Kuidas matš lõpeb',
  'canasta.rules.goal': 'Mängitakse paarides; esimene pool, kes jõuab {n} punktini, võidab matši.',
  'canasta.rules.deck': 'Mängitakse {value} kaardiga — {decks} pakki pluss jokkerid.',
  'canasta.rules.deal': 'Iga mängija saab {n} kaarti.',
  'canasta.rules.drawCount': 'Käigu alguses tõmbad {n} kaarti.',
  'canasta.rules.redThrees':
    'Punane kolm sinu käes näidatakse kohe ette ja see annab boonuse — kui sinu pool aga kunagi canastat valmis ei saa, läheb see sinu kahjuks.',
  'canasta.rules.canasta': 'Canasta on {n} või enama sama väärtusega kaardi kombinatsioon.',
  'canasta.rules.sequences': 'Kombinatsioon võib olla ka jada: kolm või rohkem sama masti kaarti järjest, mitte kunagi jokkeriga vahel.',
  'canasta.rules.samba': 'Seitsmest kaardist jada on samba ja annab {n} punkti.',
  'canasta.rules.pileAlwaysFrozen': 'Viskepakk on kogu jagamise vältel külmutatud: saad selle võtta ainult sobitades ülemise kaardi kahe loomuliku kaardiga oma käest.',
  'canasta.rules.meldFloorBands':
    'Sinu esimene väljapanek peab ulatuma punktide alammäärani, mis kasvab koos su seisuga: {negative} alla nulli, {low} kuni 1500, {mid} kuni 3000, {high} sellest üle.',
  'canasta.rules.meldFloorBandsFive': 'Su esimene kombinatsioon peab ulatuma punktimiinimumini, mis kasvab koos su skooriga: {negative} alla nulli, {low} kuni 1500, {mid} kuni 3000, {high} kuni 7000 ja {top} üle selle.',
  'canasta.rules.oneCanastaToGoOut': 'Üks valmis canasta piisab, et sinu pool välja läheks.',
  'canasta.rules.twoCanastasToGoOut': 'Sinu pool vajab kaht valmis canastat, enne kui tohib välja minna.',
  'canasta.rules.end': 'Jagatakse edasi, kuni üks pool ületab {n} punkti — siis on matš läbi.',

  'holdem.rules.section.goal': 'Eesmärk',
  'holdem.rules.section.setup': 'Ettevalmistus',
  'holdem.rules.section.betting': 'Panustamine',
  'holdem.rules.section.end': 'Kuidas matš lõpeb',
  'holdem.rules.goal': 'Võida žetoone parima käega avamisel või jäädes ainsaks mängijaks jagamises.',
  'holdem.rules.stack': 'Iga koht alustab {n} žetooniga.',
  'holdem.rules.blinds': 'Väike pimepanus on {sb} ja suur {bb}, mõlemad tehakse enne kaartide jagamist.',
  'holdem.rules.streets': 'Panustatakse neljas ringis — enne floppi ning pärast floppi, turni ja riverit.',
  'holdem.rules.showdown':
    'Need, kes on veel mängus, näitavad oma kaarte; parim viiekaardiline käsi võtab panga.',
  'holdem.rules.noLimit': 'Ilma piirmäärata — iga panus võib ulatuda kogu sinu virnani.',
  'holdem.rules.lastPlayerStanding': 'Mängitakse, kuni üks koht hoiab kõiki žetoone.',
  'holdem.rules.mostChipsWins': 'Kellel on mängu lõppedes kõige rohkem žetoone, võidab matši.',
  'holdem.rules.handLimit': 'Mäng peatub {n} jagamise järel.',

  // --- header --------------------------------------------------------------
  'header.deal': 'Jagamine {n}',
  'header.gameOf': 'Mäng {n} / {total}',
  'header.gameOfWithContract': 'Mäng {n} / {total}: {contract}',

  // --- meld preview --------------------------------------------------------
  'preview.validSet': 'Kehtiv grupp',
  'preview.validRun': 'Kehtiv jada',
  'preview.validMeld': 'Kehtiv kombinatsioon',
  'preview.notYet': 'Veel mitte kombinatsioon',
  'preview.points': '{shape} · {n} punkti',
  'preview.pointsWithLaid': '{shape} · {n} + {laid} juba väljas = {total} punkti',
  'preview.meetsFloor': '{line} (ulatub {n} ✓)',
  'preview.needsFloor': '{line} (vaja {n} ✗)',
  'preview.becauseOf': '{line} — {reason}',

  // --- discard -------------------------------------------------------------
  // A discard with a meld staged lays that meld first; this is what the
  // player is told when the server refuses it and the whole move is rolled
  // back. {reason} is the engine's own words for the refusal.
  'discard.meldRejected': '{reason} — midagi ei visatud ära, sinu kaardid on endiselt valmis.',

  // --- what the current selection would send -------------------------------
  // The client's own reason a control is not ready, next to the engine's own
  // `err.*` reasons rather than a second kind of message — a control greyed
  // out for "not your turn" and one greyed out for "you picked two, this
  // takes one" should read as the same kind of thing, not one of them looking
  // broken. See `fits` in `src/lib/drops.ts`.
  'sel.tooMany.1': 'Vali ainult üks kaart',
  'sel.tooMany.n': 'Vali kõige rohkem {n} kaarti',
  'sel.needMore': 'Vali {n} kaarti',
  'sel.notThese': 'Need kaardid ei saa siia minna',
  'sel.needsCompany': 'See kaart vajab kõrvalolevaid',

  // --- what a module says happened -----------------------------------------
  // The first module-sent keys in this bundle, and deliberately few. Anything
  // a module sends is still legible without an entry here — `humanise` turns
  // `holdem.seat.stack` into "Stack" — so a key earns a line only when that
  // fallback loses something. These do: what each of them means lives in its
  // *params* (who won, how much, whose cards), and a key on its own has
  // nowhere to put them. Without the entry the player read "Winner", full
  // stop, at the end of a match they had just won.
  'status.winner': 'Võitis {winners}',
  'holdem.status.pot': '{winners} võitis {amount} käega {hand}',
  'holdem.status.potUncontested': '{winners} võitis {amount} — kõik teised loobusid',
  'holdem.status.shown': '{playerId} näitas {value}',
  'holdem.prompt.waitingFor': 'Ootame mängijat {playerId}',

  // How a rummy scoreboard was ordered. Both earn a line for the same reason
  // as the keys above: the number lives in the params, and `humanise` would
  // render "Deals Won" with nothing after it.
  'zolik.standing.dealsWon': 'Võidetud jagamisi: {n}',
  'zolik.standing.inHand': 'Käes: {n}',

  // --- rounds ---------------------------------------------------------------
  // What a round is called, and what one did. Each of these carries its number
  // in the params, so `humanise` alone would render a label with nothing after
  // it.
  // An instruction, not a heading. "Next round" named the thing rather than
  // asking for it, and sat above "Waiting for 1" — which reads as though the
  // player is the one waiting, when the table is waiting for them.
  'round.continue': 'Alusta järgmist vooru',

  // --- the flash that announces a round or a match ending ------------------
  //
  // One line and one number, held for about two seconds. Worded generically on
  // purpose: which *kind* of ending it was is the module's own fact, printed
  // underneath, and a client that phrased "went gin" itself would be a client
  // that knows a game.
  'flash.roundWon': '{winners} võttis selle',
  'flash.roundWonYou': 'Sina võtsid selle',
  'flash.roundDrawn': 'Keegi ei võtnud seda',
  'flash.matchOver': 'Matš on läbi',
  'flash.matchWon': '{winners} võitis',
  'flash.matchWonYou': 'Sina võitsid',
  'flash.matchDrawn': 'Keegi ei võitnud',
  'flash.nowOn': 'nüüd {total}',

  'zolik.round.deal': 'Jagamine',
  'zolik.round.cleanRun': 'Üks jada peab olema jokkerita',
  'canasta.round.deal': 'Jagamine',
  'canasta.round.concealed': 'Läks välja varjatult',
  'canasta.round.exhausted': 'Pakk sai otsa',
  'canasta.round.meldCards': 'Väljapandud kaarte: {n}',
  'canasta.round.canastas': 'Canastasid: {n}',
  'canasta.round.redThrees': 'Punaseid kolmi: {n}',
  'canasta.round.goingOut': 'Väljaminek: {n}',
  'canasta.round.inHand': 'Kätte jäänud: {n}',
  'holdem.round.hand': 'Käsi',
  'holdem.round.pot': 'Pank {n}',
  'holdem.round.uncontested': 'Kõik teised loobusid',
  'seat.ready': 'Valmis',
  'zolik.seat.contractMet': 'Leping täidetud',
  'results.you': '(sina)',



  // --- refusals the other three games raise ---------------------------------
  // Worded here for the same reason Žolíky's are: reasonText falls back to the
  // code, and OfferBar hands it the code as that fallback, so an unworded
  // refusal reaches a player as PILE_FROZEN in capitals. serverKeys.test.ts
  // now fails the build rather than letting the next one through.
  'err.SET_TOO_LARGE': 'Grupil on juba kõik neli masti',
  'err.DISCARD_TAKEN_CARD_FORBIDDEN':
    'Sa ei saa ära visata kaarti, mille just võtsid — mängi see välja või hoia alles',
  'err.CARD_DOES_NOT_FIT': 'See kaart ei sobi ei masti ega väärtuse poolest',
  'err.SUIT_REQUIRED': 'Nimeta mast, mis jätkub',
  'err.MUST_ANSWER_DRAW_OR_TAKE': 'Vasta seitsmega või võta kaardid',
  'err.NOTHING_TO_DRAW': 'Tõmmata pole enam midagi',
  'err.PILE_EMPTY': 'Hunnik on tühi',
  'err.PILE_BLOCKED': 'Hunnik on blokeeritud — peal on must kolm',
  'err.PILE_FROZEN': 'Hunnik on külmutatud — vaja on kaht loomulikku pealmise kaardi väärtusega kaarti',
  'err.TOP_CARD_UNUSABLE': 'Sa ei saa pealmist kaarti kasutada',
  'err.MELD_CLOSED': 'See kombinatsioon on täielik ja suletud',
  'err.MELD_TOO_SMALL': 'Kombinatsioon vajab rohkem kaarte kui see',
  'err.MELD_TOO_LARGE': 'See kombinatsioon ei mahuta rohkem kaarte',
  'err.MELD_MIXED_RANKS': 'Kõik kombinatsiooni kaardid peavad olema sama väärtusega',
  'err.SEQUENCE_NO_WILDS': 'Jada ei tohi sisaldada jokkereid',
  'err.SEQUENCE_NEEDS_ONE_SUIT': 'Kõik jada kaardid peavad olema sama masti',
  'err.RUN_NOT_CONSECUTIVE': 'Jada peab kulgema järjest, ilma aukudeta',
  'err.NOT_ENOUGH_NATURALS': 'Kombinatsioon vajab rohkem loomulikke kaarte kui jokkereid',
  'err.RANK_ALREADY_MELDED': 'Sinu poolel on juba selle väärtusega kombinatsioon',
  'err.NOT_YOUR_MELD': 'See kombinatsioon kuulub vastaspoolele',
  'err.NO_SUCH_MELD': 'Seda kombinatsiooni pole laual',
  'err.CANNOT_MELD_THREE': 'Kolmi ei panda kunagi välja',
  'err.CANNOT_DISCARD_RED_THREE': 'Punast kolme ei tohi ära visata',
  'err.MUST_KEEP_A_CARD': 'Jäta alles vähemalt üks kaart — nii ei saa sa oma kätt tühjendada',
  'err.MUST_MELD_FIRST': 'Pane esmalt välja oma poole avang',
  'err.INITIAL_MELD_NOT_MET': 'Sinu esimesel väljapanekul jääb veel punkte puudu',
  'err.CANNOT_GO_OUT_YET': 'Sinu pool vajab valmis canastat, enne kui saab välja minna',
  'err.NOTHING_TO_CALL': 'Pole ühtegi panust, mida maksta',
  'err.CANNOT_CHECK': 'Sa ei saa passida — laual on panus, millele vastata',
  'err.CANNOT_RAISE': 'Siin sa tõsta ei saa',
  'err.RAISE_TOO_SMALL': 'Tõstmine peab olema vähemalt eelmise suurune',
  'err.NOT_ENOUGH_CHIPS': 'Sul pole nii palju žetoone',
  'err.AMOUNT_REQUIRED': 'Ütle, kui palju',
  'err.AMOUNT_NOT_A_NUMBER': 'See summa ei ole arv',
  'err.SEAT_NOT_IN_HAND': 'Sa ei ole selles jagamises',
  'err.WRONG_RANK': 'See kaart on selleks vale väärtusega',
  'err.MATCH_FULL': 'Laud on täis',
  'err.MATCH_ALREADY_STARTED': 'Matš on juba alanud',
  'err.TOO_FEW_PLAYERS': 'Mängijaid pole veel piisavalt',
  'err.WRONG_PLAYER_COUNT': 'Seda mängu ei saa nii paljude mängijatega mängida',
  'err.NOT_THE_HOST': 'Seda saab teha ainult võõrustaja',
  'err.BAD_SEATING': 'See istekohtade järjekord ei vasta lauas olijatele',
  'err.NO_LONGER_WAITING': 'Laud enam ei oota',
  'err.WAITING_ROOM_UNAVAILABLE': 'Ooteruum ei ole saadaval',
  'err.SERVER_BUSY': 'Server on praegu täis — proovi hetke pärast uuesti',


  // --- rules the engine has always enforced and never stated ----------------
  // Every one of these was found by the guardrail rather than by review: the
  // validator could refuse a player for it, and no sentence anywhere said so.
  // See server/internal/zolikmod/ruleindex_test.go.
  'zolik.rules.section.layoff': 'Kombinatsioonidele lisamine',
  'zolik.rules.pickup.obligation':
    'Enne kui oled väljas, tuleb viskepakist võetud kaart kasutada selles kombinatsioonis, millega sa sel käigul välja lähed.',
  'zolik.rules.pickup.noReturn':
    'Viskepakist võetud kaarti ei tohi samal käigul uuesti ära visata — mängi see välja või hoia alles.',
  'zolik.rules.wilds.setLimit': 'Grupis ei tohi olla rohkem jokkereid kui loomulikke kaarte.',
  'zolik.rules.set.maxSize':
    'Grupis ei tohi olla rohkem kui {n} kaarti — jokker asendab puuduvat masti, ta ei täienda juba täielikku gruppi.',
  'zolik.rules.run.maxLength':
    'Jadas ei tohi olla rohkem kui {n} kaarti — äss all, kaksteist väärtust selle peal ja äss üleval.',
  'zolik.rules.run.aceBridge':
    'Äss seisab kuninga kohal või kahe all, mitte kunagi sillana jada kahe otsa vahel.',
  'zolik.rules.contracts.contribution':
    'Kuni sa pole väljas, peab iga väljapandav kombinatsioon olema selline, mida jagamise leping veel nõuab.',
  'zolik.rules.layoff.afterDown':
    'Sa ei tohi teiste kombinatsioonidele midagi lisada, enne kui oled välja pannud oma lepingu.',
  'zolik.rules.layoff.runEnds': 'Jadale lisatud kaart peab seda ühest või teisest otsast jätkama.',
  'zolik.rules.jokers.swap':
    'Laual oleva kombinatsiooni jokkeri saab välja osta täpselt selle kaardiga, mida ta esindab.',
  'zolik.rules.jokers.reclaim.on':
    'Laualt välja ostetud jokker tuleb samal käigul kombinatsiooni mängida — see ei tohi kätte jääda.',
  'zolik.rules.jokers.reclaim.off': 'Laualt välja ostetud jokker võib kätte jääda.',
  'zolik.rules.deck.reshuffle':
    'Kui tõmbepakk saab otsa, segatakse viskepakk ja sellest saab uus tõmbepakk; kui mõlemad on tühjad, jagamine lõpeb.',


  // --- what to do instead ---------------------------------------------------
  // The third layer of a refusal, after the reason and the rule. Sent by the
  // module because it is the only side that knows which card is owed and
  // which way out is on offer; see server/internal/zolikmod/remedy.go.
  'zolik.remedy.meldThePickup': 'Lisa {card} oma väljapanekusse või võta võtmine tagasi.',
  'zolik.remedy.discardSomethingElse': 'Viska ära mõni teine kaart või mängi {card} sel käigul välja.',
  'zolik.remedy.discardNotAJoker': 'Viska ära midagi muud kui jokker.',
  'zolik.remedy.finishOrUndoLayDown': 'Lõpeta väljapanek või võta see tagasi.',
  'zolik.remedy.needMorePoints': 'Sul on vaja veel {n} punkti, et saaksid välja panna.',
  'zolik.remedy.layACleanRun': 'Pane välja jada, milles pole jokkerit.',
  'zolik.remedy.playReclaimedJoker': 'Mängi {card} kombinatsiooni või võta selle võtmine tagasi.',
  'zolik.remedy.goDownFirst': 'Pane esmalt välja oma kombinatsioonid.',
  'zolik.remedy.drawFirst': 'Tõmba esmalt kaart.',
  'zolik.remedy.drawFromStock': 'Tõmba tõmbepakist — viskepakk avaneb {n}. voorus.',
  'zolik.remedy.drawFromStockEmpty': 'Tõmba selle asemel tõmbepakist.',


  // --- keys whose meaning lives in their params -----------------------------
  // The shape fallback renders these with the number missing ("Stack", "Pot"),
  // which is why they earn a line where most keys do not.
  'header.contract': 'Nõuab {sets} gruppi ja {runs} jada',
  'header.contract.cleanRunOnly': 'Nõuab jokkerita jada',
  'header.round': 'Voor {n}',
  'header.deck': 'Tõmbepakk',
  'header.target': 'Siht',
  'header.suitInPlay': 'Mängus olev mast',
  'seat.cards': 'Kaardid',
  'zolik.offer.meld': 'Pane välja',
  'prompt.pickupMustBeMelded':
    '{value} tuli viskepakist — see peab minema kombinatsioonidesse, millega sa sel käigul välja lähed.',
  'prompt.jokerMustBePlayed':
    '{value} tuli laualt — see peab minema kombinatsiooni, enne kui saad oma käigu lõpetada.',
  'prompt.initialMeld': 'Sinu poole avang peab ulatuma {n} punktini.',
  'prompt.canastasNeeded': 'Sinu poolel on puudu veel {n} canastat, enne kui ta saab välja minna.',
  'prompt.mustDrawOrAnswerSeven': 'Vasta seitsmega või tõmba {n} kaarti.',
  'prompt.chooseSuit': 'Vali mast, mis jätkub',
  'prompt.skipPending': 'Sinu käik jäetakse vahele',
  'status.lastDeal': 'Meeskond {team} sai {value}',
  'status.teamScore': 'Meeskond {team}: {value}',
  'canasta.offer.rank': 'Väärtus',
  'canasta.offer.sequence': 'Jada',
  'badge.naturalCanasta': 'Puhas kanasta',
  'badge.mixedCanasta': 'Segakanasta',
  'badge.samba': 'Samba',
  'badge.cleanRun': 'Puhas jada',
  'canasta.seat.teamScore': 'Meeskonna punktid',
  'canasta.seat.canastas': 'Canastad',
  'holdem.header.pot': 'Pank',
  'holdem.header.street': 'Ring',
  'holdem.header.hand': 'Käsi',
  'holdem.header.handLimit': 'Käsi kokku',
  'holdem.header.blinds': 'Pimepanused',
  'holdem.cost.call': 'maksmiseks',
  'holdem.cost.pot': 'pangas',
  'holdem.seat.stack': 'Virn',
  'holdem.seat.bet': 'Panus',
  'holdem.prompt.yourAction': 'Sinu kord tegutseda',
  'holdem.prompt.raiseTo': 'Tõsta kuni',
  'holdem.quick.halfPot': '½ Pank',
  'holdem.quick.pot': 'Pank',
  'holdem.quick.allIn': 'All-in',
  'zone.yourHand': 'Sinu käsi',
  'zone.opponentHand': 'Vastase käsi',
  'zone.drawPile': 'Tõmbepakk',
  'zone.discardPile': 'Viskepakk',
  'zone.melds': 'Kombinatsioonid',
  'zone.teamMelds': 'Sinu poole kombinatsioonid',
  'zone.opponentMelds': 'Vastase poole kombinatsioonid',
  'zone.redThrees': 'Punased kolmed',
  'zone.board': 'Laud',
  'verb.drawFromDeck': 'Tõmba',
  'verb.takeFromDiscard': 'Võta hunnikust',


  // --- the why sheet's own furniture ---------------------------------------
  // The three layers a refusal is explained in. Labels, not sentences: what
  // goes under each is sent by the server or worded above.
  'why.reason': 'Miks mitte',
  'why.rule': 'Reegel',
  'why.rules': 'Reeglid',
  'why.remedy': 'Mida sa teha saad',
  'why.readTheRules': 'Loe kõiki reegleid →',
  'why.close': 'Sulge',
  'why.open': 'miks',

  // --- marks on a particular card ------------------------------------------
  // A mark is a refusal that has not happened yet: the rule is enforced at the
  // discard, which is the last possible moment to hear about it, so the card
  // says so while there is still a turn left to act on it.
  'zolik.badge.owedToMeld':
    '{card} tuli viskepakist — see peab minema kombinatsioonidesse, millega sa sel käigul välja lähed.',
  'zolik.badge.jokerOwed':
    '{card} tuli laualt — see peab minema kombinatsiooni, enne kui saad oma käigu lõpetada.',

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
  'legal.terms': 'Tingimused',
  // The document's own name, used as the heading above it. Kept apart from
  // `legal.notice.terms`, which is the same words inflected to sit inside a
  // sentence — a heading in the instrumental case reads as a mistake in every
  // Slavic language.
  'legal.terms.title': 'Kasutustingimused',
  'legal.privacy.title': 'Privaatsusteade',
  'legal.privacy': 'Privaatsus',
  'legal.source': 'Lähtekood',
  'legal.updated': 'Versioon {version}',
  'legal.draft': 'Kavand — ei ole veel jõus. Käitaja nimi, riik ja kontaktaadress on veel täitmata.',
  'legal.notice.before': 'Mängides nõustud ',
  'legal.notice.terms': 'kasutustingimustega',
  'legal.notice.between': '. Mida sinu kohta säilitatakse, on kirjas ',
  'legal.notice.privacy': 'privaatsusteates',
  'legal.notice.after': '.',
  // --- gin rummy -------------------------------------------------------------
  'err.UPCARD_DECLINED': 'Sa oled sellest kaardist juba loobunud',
  'err.DEADWOOD_TOO_HIGH': 'Sinu deadwood on koputamiseks liiga kõrge',
  'err.CARD_DOES_NOT_EXTEND_MELD': 'See kaart ei pikenda seda kombinatsiooni',
  'ginrummy.rules.setup': 'Ettevalmistus',
  'ginrummy.rules.turn': 'Sinu käik',
  'ginrummy.rules.melds': 'Kombinatsioonid',
  'ginrummy.rules.knocking': 'Koputamine',
  'ginrummy.rules.bigGin': 'Big gin',
  'ginrummy.rules.layoff': 'Külgepanek',
  'ginrummy.rules.deadHand': 'Surnud jagamine',
  'ginrummy.rules.scoring': 'Jagamise punktid',
  'ginrummy.rules.match': 'Matši võitmine',
  'ginrummy.rules.lineBonuses': 'Boonused arvestuses',
  'ginrummy.rules.deck': 'Mängitakse {value} kaardiga pakiga.',
  'ginrummy.rules.deal': 'Iga mängija saab {value} kaarti.',
  'ginrummy.rules.upcard': 'Veel üks kaart keeratakse kuvapoolega üles, alustades viskepakki.',
  'ginrummy.rules.drawDiscard':
    'Tõmba oma käigul üks kaart — tõmbepakist või viskepakist — ja viska seejärel üks ära.',
  'ginrummy.rules.setsAndRuns':
    'Kombinatsioon on kolme või nelja sama väärtusega kaardi grupp või kolme või enama sama masti kaardi jada.',
  'ginrummy.rules.aceLow': 'Äss on alati madal — emandast ässani jada ei ole.',
  'ginrummy.rules.knockLimit': 'Sa võid koputada niipea, kui sinu deadwood on {n} või vähem.',
  'ginrummy.rules.oklahoma': 'Selle jagamise koputamispiiri määrab lahtise kaardi väärtus.',
  'ginrummy.rules.gin': 'Null deadwoodi on gin — parim võimalik koputus.',
  'ginrummy.rules.bigGinBonus':
    'Üksteist kaarti kõik kombinatsioonides, ilma ühegi äravisketa, on big gin ja annab veel {n} punkti.',
  'ginrummy.rules.layoffDescription':
    'Pärast koputust, mis pole gin, tohib vastane oma deadwoodi sinu kombinatsioonide külge panna, enne kui käsi võrreldakse.',
  'ginrummy.rules.deadHandDescription':
    'Kui tõmbepakki jääb viimased kaks kaarti ja keegi pole koputanud, on jagamine surnud — keegi ei saa punkte ja sama jagaja jagab uuesti.',
  'ginrummy.rules.undercut':
    'Kui vastase deadwood ei ole sinu omast kõrgem, lõikab ta sind alt: ta saab vahe pluss {n}.',
  'ginrummy.rules.ginBonus': 'Gin annab vastase kogu käe pluss {n}.',
  'ginrummy.rules.target': 'Kes jagamise lõppedes esimesena ületab {n} punkti, võidab matši.',
  'ginrummy.rules.shutout': 'Matšiboonus kahekordistub {n} punktini, kui kaotaja ei saanud ühtegi punkti.',
  'ginrummy.rules.box': 'Iga võidetud jagamine on matši lõpus väärt {n} punkti.',
  'ginrummy.rules.gameBonus': 'Matši võitmine annab veel {n} punkti.',
  'ginrummy.fact.deadwood': '{value} deadwoodi',
  'ginrummy.fact.discardCard': 'Viska ära {value}',
  'ginrummy.fact.meldCards': 'Kombinatsiooni {value}',
  'ginrummy.header.hand': 'Käsi {n}',
  'ginrummy.round.kind': '{value}',
  'ginrummy.round.hand': 'Käsi',
  'ginrummy.seat.score': '{value}',
  'ginrummy.seat.dealer': 'Jagaja',
  'ginrummy.status.knocked': '{playerId} koputas {deadwood} deadwoodiga',
  'ginrummy.status.gin': '{playerId} tegi gini',
  'ginrummy.status.lastHand': 'Viimane käsi: {winner} ({kind}, {delta} punkti)',
  'ginrummy.offer.drawStock': 'Tõmba tõmbepakist',
  'ginrummy.offer.drawDiscard': 'Tõmba viskepakist',
  'ginrummy.offer.takeUpcard': 'Võta lahtine kaart',
  'ginrummy.offer.passUpcard': 'Passi',
  'ginrummy.offer.discard': 'Viska ära',
  'ginrummy.offer.knock': 'Koputa',
  'ginrummy.offer.gin': 'Gin!',
  'ginrummy.offer.bigGin': 'Big gin!',
  'ginrummy.offer.layOff': 'Pane külge',
  'ginrummy.offer.finishLayoff': 'Külgepanek tehtud',
  'ginrummy.zone.knockerHand': 'Koputaja käsi',
  'ginrummy.zone.melds': 'Kombinatsioonid',
  'ginrummy.prompt.upcardDecision': 'Võta lahtine kaart või passi',
  'ginrummy.prompt.yourTurnDraw': 'Tõmba kaart',
  'ginrummy.prompt.yourTurnDiscard': 'Viska ära — või koputa, kui saad',
  'ginrummy.prompt.layoff': 'Pane deadwood külge või lõpeta',

  // --- rummy tiles -------------------------------------------------------------
  'err.TILE_NOT_IN_HAND': 'Seda kivi pole sinu käes',
  'err.TILE_DOES_NOT_FIT': 'See sinna ei sobi',
  'err.NO_SUCH_SET': 'Seda kombinatsiooni pole laual',
  'err.INITIAL_MELD_ONLY': 'Enne esimest väljapanekut tohid ümber tõsta ainult oma uusi kombinatsioone',
  'err.TABLE_NOT_VALID': 'Laud ei ole veel kehtiv',
  'err.TRAY_NOT_EMPTY': 'Sul on veel vabu kive paigutada',
  'err.NOTHING_PLAYED': 'Mängi välja vähemalt üks kivi, enne kui oma käigu lõpetad',
  'err.INITIAL_MELD_TOO_LOW': 'Sinu esimene väljapanek peab olema väärt vähemalt 30 punkti',
  'err.NOT_A_RUN': 'Jagada saab ainult jada',
  'err.BAD_SPLIT_POSITION': 'Sellest kohast seda jada jagada ei saa',
  'err.NO_JOKER_IN_SET': 'Selles kombinatsioonis pole jokkerit',
  'err.TILE_JOKER_SWAP_MISMATCH': 'See kivi ei ole see, mida jokker esindab',
  'rummytiles.rules.setup': 'Ettevalmistus',
  'rummytiles.rules.sets': 'Kombinatsioonid',
  'rummytiles.rules.initialMeld': 'Esimene väljapanek',
  'rummytiles.rules.turn': 'Sinu käik',
  'rummytiles.rules.jokerTaking': 'Jokkeri võtmine',
  'rummytiles.rules.ending': 'Vooru lõpetamine',
  'rummytiles.rules.poolExhaustion': 'Kui varu otsa saab',
  'rummytiles.rules.match': 'Matši võitmine',
  'rummytiles.rules.tiles': 'Mängitakse {value} kiviga.',
  'rummytiles.rules.dealCount': 'Iga mängija saab {value} kivi.',
  'rummytiles.rules.group': 'Grupp on kolm või neli sama numbriga kivi, igaüks eri värvi.',
  'rummytiles.rules.run': 'Jada on kolm või enam järjestikust numbrit ühes värvis.',
  'rummytiles.rules.noWrap': '13 järel ei alga uuesti 1.',
  'rummytiles.rules.joker': 'Jokker esindab ükskõik millist kivi.',
  'rummytiles.rules.initialMeldDescription':
    'Kuni sa pole ühe käiguga, ainuüksi oma käest, välja pannud {n} või enam punkti, ei tohi sa puutuda midagi, mis on juba laual.',
  'rummytiles.rules.turnDescription':
    'Mängi välja vähemalt üks kivi oma käest, tõsta lauda vabalt ümber ja lõpeta nii, et iga kombinatsioon laual on kehtiv.',
  'rummytiles.rules.noDiscard':
    'Äraviskamist ei ole — kui sa ei suuda kehtivat käiku lõpetada, tõmbad selle asemel ühe kivi.',
  'rummytiles.rules.jokerTakingDescription':
    'Laual oleva jokkeri saad võtta, asendades selle oma käest kiviga, mida ta esindab — ja sul tuleb seda enne käigu lõppu kombinatsioonis kasutada.',
  'rummytiles.rules.goingOut':
    'Vooru võidab esimene mängija, kellel kivid otsa saavad. Kõik teised saavad neile jäänu väärtuse miinusmärgiga; võitja saab kõigi teiste kaotuste summa.',
  'rummytiles.rules.poolExhaustionLowestWins':
    'Kui varu saab otsa ja keegi ei saa mängida, voor lõpeb ja selle võidab madalaim käeväärtus.',
  'rummytiles.rules.poolExhaustionNoWinner':
    'Kui varu saab otsa ja keegi ei saa mängida, lõpeb voor võitjata — iga käsi lihtsalt arvestatakse.',
  'rummytiles.rules.target': 'Kes vooru lõppedes esimesena ületab {n} punkti, võidab matši.',
  'rummytiles.rules.roundLimit': 'Matš lõpeb {n} vooru järel — võidab kõrgeim tulemus.',
  'rummytiles.fact.setCards': '{value}',
  'rummytiles.header.pool': 'Varu {n}',
  'rummytiles.header.round': 'Voor {n}',
  'rummytiles.round.kind': '{value}',
  'rummytiles.round.round': 'Voor',
  'rummytiles.seat.score': '{value}',
  'rummytiles.seat.notOpened': 'Ei ole avanud',
  'rummytiles.status.lastRound': 'Viimane voor: {winner} ({kind})',
  'rummytiles.badge.invalid': 'Veel mitte kehtiv',
  'rummytiles.zone.pool': 'Varu',
  'rummytiles.zone.table': 'Laud',
  'rummytiles.zone.tray': 'Alus',
  'rummytiles.offer.place': 'Aseta',
  'rummytiles.offer.addFromHand': 'Lisa',
  'rummytiles.offer.addFromTray': 'Lisa aluselt',
  'rummytiles.offer.take': 'Võta',
  'rummytiles.offer.split': 'Jaga',
  'rummytiles.offer.swapJoker': 'Vaheta jokker',
  'rummytiles.offer.resetTurn': 'Lähtesta käik',
  'rummytiles.offer.commit': 'Valmis',
  'rummytiles.offer.draw': 'Tõmba',
  'rummytiles.param.position': 'Jaga kohalt',

  // --- blackjack -----------------------------------------------------------
  //
  // The house rules are worded twice over — on and off — because a rule that
  // is *not* in force is still something a player has to be told. "No
  // surrender at this table" is information; silence is a guess.
  'err.BET_BELOW_MINIMUM': 'See jääb alla laua alammäära',
  'err.ALREADY_BET': 'Sinu panus on juba tehtud',
  'err.INSURANCE_CLOSED': 'Praegu pole kindlustust võtta',
  'err.CANNOT_DOUBLE': 'Seda kätt ei saa kahekordistada',
  'err.CANNOT_SPLIT': 'Seda kätt ei saa jagada',
  'err.CANNOT_SURRENDER': 'Sellest käest ei saa loobuda',

  'blackjack.rules.section.table': 'Laud',
  'blackjack.rules.section.play': 'Käe mängimine',
  'blackjack.rules.section.dealer': 'Jagaja',
  'blackjack.rules.section.end': 'Kuidas matš lõpeb',
  'blackjack.rules.goal':
    'Löö jagaja, minemata üle kahekümne ühe. Üleminek kaotab kohe, ükskõik mida jagaja pärast teeb.',
  // Counts are their own phrase rather than a number glued to a noun: one
  // deck and six decks inflect differently, and Czech inflects them again.
  'blackjack.rules.decks': 'Pakke kingas: {n}.',
  'blackjack.rules.stack': 'Iga koht istub {n} žetooniga.',
  'blackjack.rules.minBet': 'Laua alammäär on {n} žetooni.',
  'blackjack.rules.faceUp':
    'Mängijate kaardid jagatakse kuvapoolega üles; jagaja hoiab ühte kaarti kinni, kuni kõik on mänginud.',
  'blackjack.rules.hitStand': 'Võta nii palju kaarte kui soovid või jää sellele, mis sul on.',
  'blackjack.rules.aces': 'Äss loeb üksteist seni, kuni see mahub, ja muidu üks.',
  'blackjack.rules.blackjack': 'Äss koos kümne väärtusega kaardiga kahel esimesel kaardil on blackjack.',
  'blackjack.rules.pays3to2': 'Blackjack maksab 3:2.',
  'blackjack.rules.pays6to5': 'Blackjack maksab 6:5.',
  'blackjack.rules.paysEven': 'Blackjack maksab üks ühele.',
  'blackjack.rules.double':
    'Kahel esimesel kaardil võid oma panuse kahekordistada ja võtta täpselt ühe lisakaardi.',
  'blackjack.rules.doubleAfterSplit': 'Ka jagamisest tekkinud kätt tohib kahekordistada.',
  'blackjack.rules.noDoubleAfterSplit': 'Jagamisest tekkinud kätt ei tohi kahekordistada.',
  'blackjack.rules.split':
    'Kaks sama väärtusega kaarti tohib jagada omaette kätteks, igaüks oma panusega — kuni {n} korda, kokku {hands} käeks.',
  'blackjack.rules.noSplit': 'Selle laua taga paare ei jagata.',
  'blackjack.rules.splitAces':
    'Jagatud ässad saavad kumbki ühe kaardi ja jäävad siis seisma, ning nii saadud kakskümmend üks ei ole blackjack.',
  'blackjack.rules.surrender':
    'Sa võid oma esimesest käest loobuda poole panuse eest, kui jagaja on blackjacki kontrollinud.',
  'blackjack.rules.noSurrender': 'Selle laua taga kätest loobuda ei saa.',
  'blackjack.rules.dealerDraws': 'Jagaja tõmbab seitsmeteistkümneni ja jääb siis seisma.',
  'blackjack.rules.hitsSoft17': 'Jagaja tõmbab ka ässaga tehtud seitsmeteistkümne peal.',
  'blackjack.rules.standsSoft17': 'Jagaja jääb ässaga tehtud seitsmeteistkümne peale seisma.',
  'blackjack.rules.dealerPeeks':
    'Ässa või kümne näidates kontrollib jagaja blackjacki, enne kui keegi mängib.',
  'blackjack.rules.insurance':
    'Jagaja ässa vastu võid end kindlustada poole panuse eest; see maksab 2:1, kui jagajal on blackjack.',
  'blackjack.rules.noInsurance': 'Selle laua taga kindlustust ei pakuta.',
  'blackjack.rules.rounds': 'Laua taga mängitakse {n} vooru.',
  'blackjack.rules.mostChipsWins': 'Kellel on lõpuks kõige rohkem žetoone, võidab matši.',
  'blackjack.rules.bustedOut':
    'Koht, mis enam {n} alammäära katta ei suuda, jääb ülejäänud matši ajaks kõrvale.',

  'blackjack.zone.dealer': 'Jagaja',
  'blackjack.zone.box': 'Käsi',
  'blackjack.zone.yourBox': 'Sinu käsi',
  'blackjack.zone.shoe': 'King',

  'blackjack.header.round': 'Voor {n} / {of}',
  'blackjack.header.minBet': 'Alammäär',
  'blackjack.header.decks': 'Pakid',
  'blackjack.header.dealerTotal': 'Jagaja näitab {n}',
  'blackjack.header.dealerSoftTotal': 'Jagaja näitab pehmet {n}',

  'blackjack.seat.stack': 'Žetoonid',
  'blackjack.seat.bet': 'Panus',
  'blackjack.seat.insurance': 'Kindlustus',
  'blackjack.seat.total': 'Kokku',
  'blackjack.seat.softTotal': 'Pehme summa',
  'blackjack.seat.out': 'Žetoonid otsas',

  'blackjack.prompt.placeBet': 'Tee oma panus',
  'blackjack.prompt.insurance': 'Kindlustus?',
  'blackjack.prompt.yourMove': 'Sinu kord',
  'blackjack.prompt.waitingFor': 'Ootame mängijat {playerId}',
  'blackjack.prompt.betAmount': 'Panus',

  'blackjack.quick.doubleMin': '2× Alammäär',
  'blackjack.quick.allIn': 'All-in',
  'blackjack.offer.bet': 'Panusta',
  'blackjack.offer.hit': 'Kaart',
  'blackjack.offer.stand': 'Jään',
  'blackjack.offer.double': 'Kahekordista',
  'blackjack.offer.split': 'Jaga',
  'blackjack.offer.surrender': 'Loobu',
  'blackjack.offer.insure': 'Võta kindlustus',
  'blackjack.offer.declineInsurance': 'Kindlustuseta',

  'blackjack.fact.tableMinimum': 'alammäär',
  'blackjack.fact.insuranceCost': 'kindlustuseks',
  'blackjack.fact.extraStake': 'panuseks',
  'blackjack.fact.surrenderReturn': 'tagasi',

  'blackjack.status.dealerBlackjack': 'Jagajal oli blackjack',
  'blackjack.status.dealerBust': 'Jagaja läks üle {n} juures',
  'blackjack.status.dealerStands': 'Jagaja jääb {n} juurde',

  'blackjack.round.name': 'Voor',
  'blackjack.round.dealerTotal': 'Jagaja {n}',
  'blackjack.round.dealerBust': 'Jagaja üle ({n})',
  'blackjack.round.dealerBlackjack': 'Jagaja blackjack',
  'blackjack.round.outcome.blackjack': 'Blackjack',
  'blackjack.round.outcome.win': 'Võidetud',
  'blackjack.round.outcome.push': 'Viik',
  'blackjack.round.outcome.lose': 'Kaotatud',
  'blackjack.round.outcome.bust': 'Üle',
  'blackjack.round.outcome.surrender': 'Loobutud',

  'blackjack.badge.inPlay': 'Mängus',
  'blackjack.badge.doubled': 'Kahekordistatud',
  'blackjack.badge.split': 'Jagatud',
  'blackjack.badge.blackjack': 'Blackjack',
  'blackjack.badge.bust': 'Üle',
  'blackjack.badge.won': 'Võidetud',
  'blackjack.badge.push': 'Viik',
  'blackjack.badge.lost': 'Kaotatud',
  'blackjack.badge.surrendered': 'Loobutud',

  'blackjack.unit.chips': 'žetooni',

  // --- the settings screen --------------------------------------------------
  //
  // Hardcoded English until now, which was survivable while the only other
  // language was unreachable and faintly absurd once the language picker
  // itself lived on this screen: a player who cannot read "Settings" is
  // exactly the one who came here to change it.
  'settings.title': 'Seaded',
  'settings.signedInAs': 'Sisse logitud kui {username}',
  'settings.playingAsGuest': 'Mängid kui {username} (külaline)',
  'settings.notSignedIn': 'Ei ole sisse logitud — logi sisse või jätka külalisena, et mängida veebis.',
  'settings.subtitle': 'Milline näed välja sina ja milline laud',
  'settings.face.heading': 'Sinu nägu laua taga',
  'settings.face.account': 'Salvestatud sinu kontole, nii et see tuleb teise seadmesse kaasa.',
  'settings.face.device': 'Salvestatud sellesse seadmesse. Logi sisse, et see endaga kaasa võtta.',
  'settings.skin.heading': 'Laua välimus',
  'settings.language.heading': 'Keel',
  'settings.language.status': 'Salvestatud sellesse seadmesse.',
  'settings.language.auto': 'Automaatne',
  'settings.language.auto.now': 'Järgib sinu seadet — praegu {language}',
  'settings.legal.heading': 'Väikses kirjas',
  'settings.legal.status': 'Millega sa mängides nõustusid ja mida sinu kohta säilitatakse.',
  'settings.signIn': 'Logi sisse',
  'settings.back': 'Tagasi',

  // A translated interface over an English notice is a worse lie than an
  // English interface, so the document says so itself rather than letting the
  // surrounding screen imply the text was written for this reader.
  'legal.untranslated':
    'Seda teadet ei ole veel sinu keelde tõlgitud. Kehtiv on allpool olev ingliskeelne tekst.',

  // --- the navigation bar's screen titles ------------------------------------
  //
  // These sit in `app/_layout.tsx`, above every screen, and were the last
  // English left after the settings screen was translated — a German player
  // reading "Einstellungen" under a bar that said "Settings". A translated app
  // with an untranslated chrome bar looks like a bug, not like a limit.
  'nav.home': 'Zolik',
  'nav.emailSignIn': 'Sisselogimine e-postiga',
  'nav.signingIn': 'Sisselogimine',
  'nav.usernameSignIn': 'Sisselogimine kasutajanimega',
  'nav.legacyAccount': 'Vana konto',
  'nav.guest': 'Külaline',
  'nav.account': 'Konto',
  'nav.games': 'Mängud',
  'nav.table': 'Sinu laud',
  'nav.join': 'Liitu lauaga',
  // The screen a shared invite link lands on while it resolves the code.
  'nav.joining': 'Liitumine',
  'nav.rules': 'Reeglid',
  'nav.match': 'Matš',
  'nav.scoreTable': 'Punktitabel',
  'nav.stats': 'Statistika',
  'nav.more': 'Rohkem',
  'nav.about': 'Teave',

  // --- the account menu behind the face in the corner ----------------------
  'menu.label': 'Konto menüü',
  'menu.signedIn': 'Sisse logitud',
  'menu.notSignedIn': 'Pole sisse logitud',
  'menu.keepStats': 'et statistika alles jääks',
  'menu.signOut': 'Logi välja',
  'more.scoreTable': 'Vahetu punktitabel',
  'more.stats': 'Statistika ja edetabel',
  'more.needsAccount': 'logi sisse',
  'gate.title': 'Logi sisse, et seda kasutada',
  'gate.body':
    'Punktitabelid ja statistika salvestatakse sinu kontoga, nii et need järgnevad sulle teise seadmesse. Külalisel pole neid kuhugi salvestada.',

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
  'error.generic': 'See ei õnnestunud',
  'error.signIn': 'Sisselogimine ebaõnnestus',
  'error.login': 'Sisselogimine ebaõnnestus',
  'error.register': 'Registreerimine ebaõnnestus',
  'error.sendCode': 'Koodi ei õnnestunud saata',
  'error.badCode': 'See kood ei töötanud',
  'error.rulesLoad': 'Reegleid ei õnnestunud laadida',
  'error.createFailed': 'Loomine ebaõnnestus',
  'error.saveFailed': 'Salvestamine ebaõnnestus',
  'error.exportFailed': 'Eksport ebaõnnestus',

  // --- the route that does not exist ----------------------------------------
  'notFound.title': 'Ups!',
  'notFound.message': 'Sellist ekraani ei ole.',
  'notFound.home': 'Mine avaekraanile!',

  // --- signing in -----------------------------------------------------------
  'auth.login.subtitle': 'Hoia oma statistikat kõigis seadmetes',
  'auth.login.continueWithEmail': 'Jätka e-postiga',
  'auth.login.usernameInstead': 'Logi selle asemel sisse kasutajanimega',
  'auth.email.title': 'Sisselogimine e-postiga',
  'auth.email.subtitle': 'Saadame sulle ühekordse koodi',
  'auth.email.address': 'E-posti aadress',
  'auth.email.send': 'Saada kood',
  'auth.email.codeTitle': 'Sisesta kood',
  'auth.email.codePlaceholder': 'Kuuekohaline kood',
  'auth.email.differentAddress': 'Kasuta teist aadressi',
  'auth.email.sentTo': 'Saadetud aadressile {email}',
  'auth.email.continue': 'Jätka',
  'auth.guest.title': 'Mäng külalisena',
  'auth.guest.subtitle': 'Kontot pole vaja',
  'auth.guest.displayName': 'Kuvatav nimi',
  'auth.register.title': 'Loo konto',
  'auth.register.username': 'Kasutajanimi',
  'auth.register.email': 'E-post (valikuline)',
  'auth.register.password': 'Parool',
  'auth.username.createAccount': 'Loo konto kasutajanime ja parooliga',
  'auth.callback.signedIn': 'Sisse logitud.',

  // --- the account screen ---------------------------------------------------
  'account.signInPrompt': 'Logi sisse, et oma kontot hallata.',
  'account.keepGames': 'Säilita need mängud',
  'account.signedInWith': 'Sisse logitud kaudu',
  'account.addMethod': 'Lisa sisselogimisviis',
  'account.usernameAndPassword': 'Kasutajanimi ja parool',
  'account.faceAndTable': 'Nägu ja laua välimus',
  'account.refresh': 'Värskenda',
  'account.remove': 'Eemalda',

  // --- the main menu --------------------------------------------------------
  'home.subtitle': 'Kontinentaalne rummi · {server}',
  'home.playingAs': 'Mängid nimega {name}',
  'home.signInPrompt': 'Logi sisse või jätka külalisena, et võrgus mängida.',
  'home.statsAndLeaderboard': 'Statistika ja edetabel',
  'home.play': 'Mängi',
  'home.offlineScoreTable': 'Punktitabel võrguühenduseta',
  'home.signInToKeepStats': 'Logi sisse, et statistika alles jääks',
  'home.signOut': 'Logi välja',
  'home.continueAsGuest': 'Jätka külalisena',
  // Someone playing as a guest, marked in a list of names.
  'home.guestSuffix': '(külaline)',

  // --- the waiting room on the main menu ------------------------------------
  //
  // Counts are whole phrases per count, not a number glued to a noun — the
  // same reason `countLabel` exists. One and many are the cases the interface
  // actually produces, so they are the cases that get their own wording.
  'waiting.checking': 'Vaatame, kes on ligi…',
  'waiting.youAreWaiting': 'Sa ootad mängu',
  'waiting.pickedUp': 'Igaüks, kes laua avab, võib sind kaasa võtta — kellelgi pole sinult koodi vaja.',
  'waiting.othersOne': 'Ootab veel 1 mängija',
  'waiting.othersMany': 'Ootab veel {n} mängijat',
  'waiting.oneWaiting': '1 mängija ootab mängu',
  'waiting.manyWaiting': '{n} mängijat ootab mängu',
  'waiting.adding': 'Lisame su ootejärjekorda…',
  'waiting.slowHint':
    'Kui see ei lõpe paari sekundiga, kontrolli, kas allolev serveri aadress on sellest seadmest kättesaadav.',
  'waiting.serverBusyDetail': 'Katse {n}. Server ei võta praegu ooteruumi uusi ühendusi vastu.',
  'waiting.reconnecting': 'Ühendus katkes — ühendame uuesti…',
  'waiting.reconnectingDetail':
    'Katse {n}. Nii võib juhtuda, kui su seadme võrk muutus või server taaskäivitus.',
  'waiting.tryAgain': 'Proovi kohe uuesti',
  'waiting.makeAvailable': 'Tee mind mängimiseks kättesaadavaks',
  'waiting.stop': 'Lõpeta ootamine',
  'waiting.noneYet': 'Praegu ei oota keegi mängu. Pane end nimekirja, siis oled esimene, keda keegi näeb.',
  'waiting.noOthersYet':
    'Keegi teine veel ei oota. Võõrustajad näevad sind sellegipoolest ja võivad sind kutsuda.',
  'waiting.server': 'Server',
  'waiting.none': 'Praegu ei oota keegi. Kes end peamenüüs kättesaadavaks teeb, ilmub siia.',

  // --- landing on a shared invite link --------------------------------------
  'join.missingCode': 'Sellel lingil puudub laua kood.',
  'join.staleLink': 'Küsi kutsujalt värsket linki või liitu selle asemel koodiga.',
  'join.enterCode': 'Sisesta kood',
  'join.backToMenu': 'Tagasi menüüsse',
  'join.takingSeat': 'Võtame koha…',
  'join.takingSeatAt': 'Võtame koha mängus {game}…',

  // --- the lobby ------------------------------------------------------------
  'lobby.games.subtitle': 'Kõik, mida see server pakkuda oskab',
  'lobby.games.bots': 'Robotid',
  'lobby.games.playBot': 'Mängi roboti vastu',
  'lobby.games.playBots': 'Mängi {n} roboti vastu',
  'lobby.games.openTable': 'Ava laud',
  'lobby.games.players': '{n} mängijat',
  'lobby.games.playerRange': '{min}–{max} mängijat',
  'lobby.join.placeholder': 'Liitumiskood või kutselink',
  'lobby.join.needCode': 'Sisesta liitumiskood, link või matši ID',
  'lobby.games.signInFirst': 'Logi kõigepealt sisse',
  'lobby.join.action': 'Liitu',
  'lobby.join.waitingTitle': 'Ootame võõrustajat',
  // Two whole sentences rather than one with a swapped noun: "a game of {game}"
  // does not survive a language that inflects the game's name after "of".
  'lobby.join.joinedGame': 'Liitusid mänguga {game} — ootame algust',
  'lobby.join.joinedTable': 'Liitusid lauaga — ootame algust',
  'lobby.table.addBot': 'Lisa robot',
  'lobby.table.side': 'Pool {n}',
  'lobby.table.shuffleSeats': 'Sega kohad',
  'lobby.table.moveSeatUp': 'Liiguta {name} koha võrra üles',
  'lobby.table.moveSeatDown': 'Liiguta {name} koha võrra alla',
  'lobby.table.start': 'Alusta',
  'lobby.table.waitingForHost': 'Ootame, kuni võõrustaja alustab…',

  // --- inviting someone to a table ------------------------------------------
  'invite.heading': 'Kutsu mängijaid',
  'invite.explain': 'Saada see link. Kes selle avab, jõuab selle laua taha — kontot pole vaja.',
  'invite.noAddress': 'Sellel serveril pole jagatavat aadressi seadistatud, kasuta seega allolevat koodi.',
  'invite.readOutCode': 'Või ütle kood ette:',
  'invite.copy': 'Kopeeri link',
  'invite.share': 'Jaga linki',
  'invite.copied': 'Kopeeritud!',
  'invite.shared': 'Jagatud',

  // --- the match screen -----------------------------------------------------
  'match.waitingForTable': 'Ootame lauda…',
  'match.waitingForPlayer': 'Ootame teist mängijat…',
  'match.nobodyWon': 'Keegi ei võitnud.',
  'match.youWon': 'Sa võitsid.',
  'match.finished': 'See matš on lõppenud.',
  'match.inProgress': 'Matš käib — kõik on ühendatud ja liigub tavapäraselt.',
  'match.connecting': 'Ühendan…',
  'match.abandonedTitle': 'Laud kõrvale pandud',
  'match.abandoned': 'Keegi ei tulnud selle laua juurde tagasi, nii et see pandi kõrvale. Kaardid on täpselt seal, kuhu sa need jätsid.',
  'match.resume': 'Jätka sealt, kus pooleli jäid',
  'match.resuming': 'Taastan lauda…',
  'match.controls': 'Juhtnupud',
  'match.over': 'Matš läbi',
  'match.settingUp': 'Valmistame ette…',
  'match.playAgain': 'Mängi uuesti',
  'match.backToGames': 'Tagasi mängude juurde',
  'match.table': 'Laud',
  'match.opponents': 'Vastased',
  // Marks which seat is the reader's own, in a list of seats.
  'match.youSuffix': '(sina)',
  // The winner line is composed from whole sentences rather than a name glued
  // to " won." — the verb agrees with the subject in most of these languages,
  // and a suffix cannot know that.
  'match.you': 'sina',
  'match.someoneWon': '{name} võitis.',
  'match.wonBy': 'Võitis {names}.',
  'match.pausedFor': 'Peatatud — ootame, kuni {name} uuesti ühendub.',
  'match.results': 'Tulemused',
  'match.players': 'Mängijad',
  'match.toPlay': 'käigul',

  // --- the offline score table ----------------------------------------------
  'scoring.namesHint': 'Nimed komadega eraldatult (4–8 mängijat)',
  'scoring.newSession': 'Uus seanss',
  // A worked example, not a sentence: the names are placeholders a translator
  // may localise, the shape `name:score,` is what the parser needs.
  'scoring.scoresPlaceholder': 'Anne:120,Bruno:80,…',
  'scoring.saveRound': 'Salvesta voor',
  'scoring.export': 'Ekspordi punktileht',
  'scoring.formatHint': 'Punktide vorming: Nimi:100,Nimi2:50',
  'scoring.nameCountError': 'Sisesta 2–8 mängija nime komadega eraldatult',
  'scoring.session': 'Seanss: {id}',
  'scoring.players': 'Mängijad: {names}',
  'scoring.roundScores': '{n}. vooru punktid',
  'stats.loading': 'Laadime…',
  // The figures could not be fetched. `{reason}` is whatever the server or the
  // network said, which is not ours to translate — the frame around it is.
  'stats.unavailable': '(pole saadaval: {reason})',

  // --- statistics -----------------------------------------------------------
  'stats.title': 'Statistika ja edetabel',
  'stats.yours': 'Sinu statistika',
  'stats.leaderboard': 'Edetabel',

  // --- a player's lifetime record, shown beside a finished match ------------
  'record.title': 'Sinu saldo',
  'record.guest':
    'Sa mängid külalisena, seega saldot ei peeta. Logi sisse ja mängud, mille oled selles seadmes juba mänginud — ka see siin — jäävad sinu konto külge.',
  'record.signInToKeep': 'Logi sisse ja säilita need',
  'record.failed': 'Sinu saldot ei õnnestunud praegu laadida. Matš on turvaliselt talletatud.',
  'record.loading': 'Laadime…',
  'record.played': 'Mängitud',
  'record.won': 'Võidud',
  'record.lost': 'Kaotused',
  'record.winRate': 'Võiduprotsent',
  'record.streak': 'Seeria',
  'record.atThisGame': 'Selles mängus',
  // A streak is a count with a noun, so it is a whole phrase per count for
  // the same reason `countLabel` is: "1 win" / "2 wins" is an English rule,
  // and most of these languages do not share it.
  'record.streakNone': '—',
  'record.streakWinOne': '1 võit',
  'record.streakWinMany': '{n} võitu',
  'record.streakLossOne': '1 kaotus',
  'record.streakLossMany': '{n} kaotust',

  // --- moving cards ---------------------------------------------------------
  'hand.dragHint': 'Lohista kaarti mööda lehvikut, et see ümber paigutada, või lauale, et see välja mängida',
  'hand.moveLeft': 'Vasakule',
  'hand.moveRight': 'Paremale',
  'zone.collapseGroup': 'Ahenda see rühm',
  'zone.expandGroup': 'Näita selle rühma kõiki kaarte',
  'zone.dropHere': 'Kukuta siia',
  'offer.pickCards': 'vali kaardid kohale, mida puudutasid',
  'offer.ambiguous': 'see sobib mitmesse kohta — vali laual',

  // --- the build footer -----------------------------------------------------
  'build.app': 'rakendus',
  'build.server': 'server',
  'about.subtitle': 'Versioon, mida sa mängid, ja väike kiri.',
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
  'option.pauseBetweenRounds': 'Paus voorude vahel',
  'choice.pauseBetweenRounds.1': 'Paus',
  'choice.pauseBetweenRounds.0': 'Mängi kohe edasi',
  'option.botSkill': 'Vastased',
  'choice.botSkill.0': 'Segatud',
  'choice.botSkill.1': 'Kerge',
  'choice.botSkill.2': 'Keskmine',
  'choice.botSkill.3': 'Raske',
  'option.initialMeldMinimum': 'Avamisväärtus',
  'choice.initialMeldMinimum.0': 'Puudub',
  'option.discardDrawMinRound': 'Võtmine viskepakist',
  'choice.discardDrawMinRound.0': 'Avatud',
  'choice.discardDrawMinRound.2': 'Alates voorust 2',
  'choice.discardDrawMinRound.3': 'Alates voorust 3',
  'option.requireCleanRun': 'Jokkerita jada',
  'choice.requireCleanRun.1': 'Nõutav',
  'choice.requireCleanRun.0': 'Ei',
  'option.jokerReclaimMustPlay': 'Välja ostetud jokker',
  'choice.jokerReclaimMustPlay.1': 'Mängida samal käigul',
  'choice.jokerReclaimMustPlay.0': 'Võib kätte jääda',
  'option.dealStarter': 'Kes alustab',
  'choice.dealStarter.0': 'Kordamööda',
  'choice.dealStarter.1': 'Alustab võitja',
  'variation.prsi.classic': 'Klassikaline',
  'option.handSize': 'Jagatud kaardid',
  'variation.canasta.classic': 'Klassikaline',
  'variation.canasta.modern_american': 'Modern American',
  'variation.canasta.samba': 'Samba',
  'option.targetScore': 'Sihtpunktid',
  'option.canastasToGoOut': 'Canastasid väljaminekuks',
  'variation.holdem.freezeout': 'Freezeout',
  'variation.holdem.timed': 'Kindel arv jagamisi',
  'option.startingStack': 'Algžetoonid',
  'option.bigBlind': 'Suur pimepanus',
  'option.handLimit': 'Jagamised',
  'choice.handLimit.0': 'Kuni jääb üks koht',
  'variation.ginrummy.standard': 'Standardne',
  'option.knockLimit': 'Koputamispiir',
  'choice.knockLimit.0': 'Oklahoma (selle määrab lahtine kaart)',
  'option.bigGin': 'Big gin',
  'choice.bigGin.0': 'Väljas',
  'choice.bigGin.1': 'Sees (+25)',
  'option.lineBonuses': 'Boonused arvestuses',
  'choice.lineBonuses.1': 'Sees',
  'choice.lineBonuses.0': 'Väljas',
  'variation.rummytiles.standard': 'Standardne',
  'choice.targetScore.0': 'Puudub',
  // Two games qualify a number the others mean plainly: Canasta's 500 is a
  // short game, Gin Rummy's and Rummy Tiles' 500 is just 500; Hold'em's
  // starting 200 is a short stack, Blackjack's 200 is just 200. Module-scoped
  // so the qualifier travels with the game that means it — a bare key would
  // hand the note to games it is false for. `scripts/check-server-labels.js`
  // is what finds these; both were shipping as English on a Czech screen.
  'choice.canasta.targetScore.500': '500 (lühike)',
  'choice.holdem.startingStack.200': '200 (lühike)',
  'option.roundLimit': 'Voorude piir',
  'choice.roundLimit.0': 'Puudub',
  'option.poolExhaustion': 'Kui varu saab otsa',
  'choice.poolExhaustion.1': 'Vooru võidab madalaim käsi',
  'choice.poolExhaustion.0': 'Vooru ei võida keegi',
  'variation.blackjack.single': 'Üks pakk',
  'option.minBet': 'Laua alammäär',
  'option.rounds': 'Voorud',
  'option.decks': 'Pakid',
  'option.dealerHitsSoft17': 'Jagaja pehme 17 puhul',
  'choice.dealerHitsSoft17.0': 'Jääb',
  'choice.dealerHitsSoft17.1': 'Võtab',
  'option.blackjackPays': 'Blackjack maksab',
  'choice.blackjackPays.100': 'Üks ühele',
  'option.maxSplits': 'Jagamine',
  'choice.maxSplits.0': 'Ei jagata',
  'choice.maxSplits.1': 'Üks kord (kaks kätt)',
  'choice.maxSplits.3': 'Kolm korda (neli kätt)',
  'option.doubleAfterSplit': 'Kahekordistamine pärast jagamist',
  'choice.doubleAfterSplit.1': 'Lubatud',
  'choice.doubleAfterSplit.0': 'Pole lubatud',
  'option.surrender': 'Loobumine',
  'choice.surrender.0': 'Väljas',
  'choice.surrender.1': 'Hiline loobumine',
  'option.insurance': 'Kindlustus',
  'choice.insurance.1': 'Pakutakse',
  'choice.insurance.0': 'Ei pakuta',

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
  'verb.add': 'Lisa',
  'verb.bet': 'Panusta',
  'verb.call': 'Maksan',
  'verb.check': 'Passin',
  'verb.commit': 'Valmis',
  'verb.continue': 'Jätka',
  'verb.decline_insurance': 'Kindlustuseta',
  'verb.discard': 'Viska ära',
  'verb.double': 'Kahekordista',
  'verb.draw': 'Tõmba',
  'verb.finish_layoff': 'Külgepanek tehtud',
  'verb.fold': 'Loobun',
  'verb.hit': 'Kaart',
  'verb.insure': 'Võta kindlustus',
  'verb.knock': 'Koputa',
  'verb.lay_meld': 'Pane välja',
  'verb.lay_off': 'Pane külge',
  'verb.pass': 'Passi',
  'verb.place': 'Aseta',
  'verb.play_card': 'Mängi',
  'verb.raise': 'Tõstan',
  'verb.reset_turn': 'Lähtesta käik',
  'verb.split': 'Jaga',
  'verb.stand': 'Jään',
  'verb.surrender': 'Loobu',
  'verb.swap_joker': 'Vaheta jokker',
  'verb.take': 'Võta',
  'verb.take_pile': 'Võta hunnikust',
  // Declared by the server and listed in `serverKeys.json`, but never worded.
  'verb.takePileFromHand': 'Võta hunnik kätte',
  'verb.takePileOntoMeld': 'Võta hunnik kombinatsiooni',
  'verb.takeTopForSequence': 'Võta ülemine kaart jadasse',
  'verb.undoDraw': 'Võta tõmme tagasi',
  'verb.undoLayOff': 'Võta külgepanek tagasi',
  'verb.undoMeld': 'Võta kombinatsioon tagasi',
  'verb.undoTakePile': 'Võta hunnikuvõtt tagasi',
  'verb.undoTurn': 'Võta käik tagasi',

  // --- suits, spelled out ---------------------------------------------------
  //
  // Read aloud by a screen reader and shown where a pip would not fit. The
  // pips themselves are drawn, not written, so these are the only place the
  // suit is ever a word.
  'suit.C': 'Risti',
  'suit.D': 'Ruutu',
  'suit.H': 'Ärtu',
  'suit.S': 'Poti',

  // --- counters the board prints beside a number ----------------------------
  'canasta.seat.notOpened': 'Ei ole avanud',
  'canasta.unit.points': 'punkti',
  'ginrummy.unit.points': 'punkti',
  'holdem.seat.dealer': 'Jagaja',
  'holdem.seat.folded': 'Loobus',
  'holdem.seat.allIn': 'All in',
  'holdem.seat.out': 'Väljas',
  'holdem.unit.chips': 'žetooni',
  'prsi.unit.cardsLeft': 'kaarti jäänud',
  'rummytiles.prompt.initialMeld': 'Sinu esimene väljapanek peab olema väärt {n} punkti.',
  'rummytiles.unit.points': 'punkti',
  'zolik.unit.penalty': 'trahv',
  'header.pileFrozen': 'Hunnik külmutatud',
  // The module-less form of `ginrummy.prompt.yourTurnDraw`: Žolíky and Canasta
  // send the prompt without a module prefix. Neither the Go constants nor
  // `serverKeys.json` list it — the sweep is what found it.
  'prompt.yourTurnDraw': 'Tõmba kaart',
  'prompt.yourTurnMeld': 'Laota välja, kui saad, siis viska ära',
};
