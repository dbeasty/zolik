/**
 * Hungarian. Römi vocabulary: csoport for a set, sor for a run, kombináció for a meld, húzópakli and dobópakli for the two piles.
 */

export const hu: Record<string, string> = {
  // --- engine error codes (rules.RulesErrorCode) ---------------------------
  'err.NOT_YOUR_TURN': 'Nem te következel',
  'err.WRONG_PHASE': 'Most nem lehet',
  'err.MUST_DRAW_FIRST': 'Húzz egy lapot, mielőtt leraksz',
  'err.GAME_SUSPENDED': 'A játék szünetel',
  'err.GAME_NOT_ACTIVE': 'A játék nem fut',
  // --- runtime refusals, which are not the rules saying no -----------------
  //
  // Both were reachable long before they were worded, and a player who met
  // one was shown the code in capitals or, worse, nothing at all: an action
  // sent while the socket was down simply vanished, which reads as a legal
  // move being silently refused.
  'err.MATCH_NOT_ACTIVE': 'Az asztal szünetel — várunk, hogy egy játékos visszatérjen',
  'err.NOT_CONNECTED': 'Nincs kapcsolat az asztallal — újracsatlakozás, utána próbáld újra',
  // Why the "start the next round" control is greyed out. Without wording the
  // shell printed the code itself, so a player agreeing to go on was answered
  // with ALREADY_READY in capitals.
  'err.ALREADY_READY': 'Készen állsz',
  'err.NOT_BETWEEN_ROUNDS': 'A kör még tart',
  'err.NOT_AT_THIS_TABLE': 'Nem ülsz ennél az asztalnál',
  'err.DISCARD_LOCKED': 'A dobópakli egyelőre zárva van',
  'err.DISCARD_PILE_EMPTY': 'A dobópakli üres',
  'err.NO_CARDS_LEFT': 'Nincs több húzható lap',
  'err.ROUND_REQ_NOT_MET': 'Előbb rakd le a saját nyitásodat',
  'err.NEED_CLEAN_RUN': 'Joker nélküli sorra van szükséged az asztalon, hogy leraktnak számíts',
  'err.INCOMPLETE_INITIAL_MELD': 'Fejezd be a lerakást, vagy vond vissza, mielőtt dobsz',
  'err.DISCARD_CARD_NOT_MELDED': 'A felvett lapnak a kombinációdba kell kerülnie',
  'err.JOKER_DISCARD_FORBIDDEN': 'Jokert nem lehet eldobni',
  'err.NOTHING_TO_UNDO': 'Nincs mit visszavonni',
  'err.NO_JOKER_IN_MELD': 'Ebben a kombinációban nincs joker',
  'err.JOKER_SWAP_MISMATCH': 'Ez a lap nem veszi át a joker helyét',
  'err.RECLAIMED_JOKER_NOT_MELDED': 'Az asztalról levett jokert ebben a körben kombinációba kell játszani',
  'err.RUN_TOO_LONG': 'Ez a sor már teljes hosszúságú',
  'err.WRONG_RUN_END': 'Ez a lap a sor másik végét hosszabbítja',
  'err.INVALID_MELD': 'A kezedben egyetlen lap sem illik ide',
  'err.CARD_NOT_IN_HAND': 'Ez a lap nincs a kezedben',
  'err.MELD_BELOW_MINIMUM': 'A kombinációidból még hiányoznak pontok a lerakáshoz',
  'err.MELD_NO_CONTRIBUTION': 'Ez a kombináció nem viszi előre a követelményedet',
  'err.TOO_MANY_WILDS': 'Túl sok joker van ebben a kombinációban',
  'err.ADJACENT_WILDS': 'Két joker nem állhat egymás mellett',
  'err.ACE_BRIDGE': 'Az ász nem kötheti össze a királyt és a kettest',

  // --- contract phrasing ---------------------------------------------------
  // Built from the structured contract the server sends (sets/runs counts),
  // never from a deal-number lookup table.
  // Whole phrases per count rather than a number plus a pluralised noun:
  // Czech inflects the noun by count (skupina / skupiny / skupin) in a way no
  // "add an s" helper survives, and the counts that actually occur are 1-3.
  // A count outside that falls back to contract.sets.n.
  'contract.sets.1': 'Egy csoport',
  'contract.sets.2': 'Két csoport',
  'contract.sets.3': 'Három csoport',
  'contract.sets.n': '{n} csoport',
  'contract.runs.1': 'Egy sor',
  'contract.runs.2': 'Két sor',
  'contract.runs.3': 'Három sor',
  'contract.runs.n': '{n} sor',
  'contract.any': 'Bármilyen érvényes kombináció',
  'contract.cleanRunOnly':
    'Csoportok és sorok tetszőleges keveréke — legalább egy sornak joker nélkülinek kell lennie',
  'contract.cleanRunSuffix': '{base} — egy sornak joker nélkülinek kell lennie',

  // --- written rules ---------------------------------------------------------
  // Full-sentence keys a "see the rules" screen renders, one per module,
  // resolved server-side against the table's actual variation and options —
  // see internal/module/rules.go and each game's rules.go. Shared section
  // titles first, then one block per module in the order the four games were
  // added.
  'zolik.rules.section.goal': 'Cél',
  'zolik.rules.section.setup': 'Előkészítés',
  'zolik.rules.section.turn': 'A te köröd',
  'zolik.rules.section.melding': 'Lerakás',
  'zolik.rules.section.end': 'Hogyan ér véget a mérkőzés',
  'zolik.rules.goal':
    'Ürítsd ki elsőként a kezed érvényes csoportok és sorok lerakásával, és gyűjts közben minél kevesebb büntetőpontot azokban a lapokban, amelyeket még tartasz, amikor valaki más kiszáll.',
  'zolik.rules.deal': 'Minden játékos {n} lapot kap.',
  'zolik.rules.meldShapes':
    'A csoport {set}+ azonos értékű lap; a sor {run}+ egymást követő, azonos színű lap.',
  'zolik.rules.turn.draw': 'A körödben húzz egy lapot — a húzópakliból vagy a dobópakliból.',
  'zolik.rules.pickup.topOnly': 'A dobópakliból csak a felső lap vehető el.',
  'zolik.rules.pickup.anyFromPile':
    'A dobópakliból bármelyik lap elvehető, mindennel együtt, ami fölötte van.',
  'zolik.rules.pickup.locked': 'A dobópakliból a(z) {n}. kör előtt nem lehet húzni.',
  'zolik.rules.pickup.open': 'A dobópakli az első körtől nyitva van.',
  'zolik.rules.turn.discard': 'Zárd le a köröd egy lap eldobásával.',
  'zolik.rules.jokers.restricted':
    'Jokert soha nem lehet eldobni, kivéve ha pontosan az a lap, amely kiüríti a kezed.',
  'zolik.rules.lead.rotate':
    'A kezdés minden leosztásban egy hellyel arrébb kerül, függetlenül attól, ki nyert.',
  'zolik.rules.lead.winner': 'Aki kiszáll, az kezdi a következő leosztást.',
  'zolik.rules.meldFloor.on':
    'Az első lerakásodnak legalább {n} természetes pontot kell adnia ahhoz, hogy leraktnak számíts.',
  'zolik.rules.meldFloor.off': 'Az első lerakásodra nincs minimális pontérték.',
  'zolik.rules.cleanRun.on':
    'Legalább az egyik sorodnak teljesen joker nélkülinek kell lennie ahhoz, hogy leraktnak számíts.',
  'zolik.rules.cleanRun.off':
    'A soraid szabadon használhatnak jokert — egyiknek sem kell joker nélkülinek lennie.',
  'zolik.rules.contracts.rotating':
    'A mérkőzés {n} leosztásból áll, és minden leosztás a maga csoport- és sorkombinációját kéri.',
  'zolik.rules.contracts.static':
    'Minden leosztás ugyanazt a kombinációt kéri: {sets} csoport és {runs} sor.',
  'zolik.rules.end.afterDeals': 'A mérkőzés {n} leosztás után ér véget.',
  'zolik.rules.end.atScore': 'Addig osztanak újra, amíg valaki el nem éri a(z) {n} pontot — akkor vége.',

  'prsi.rules.section.goal': 'Cél',
  'prsi.rules.section.setup': 'Előkészítés',
  'prsi.rules.section.turn': 'A te köröd',
  'prsi.rules.section.special': 'Különleges lapok',
  'prsi.rules.section.end': 'Hogyan ér véget a mérkőzés',
  'prsi.rules.goal': 'Játszd ki elsőként a kezedben lévő összes lapot.',
  'prsi.rules.deck': '{value} lapos paklival játsszák (héttől felfelé).',
  'prsi.rules.deal': 'Minden játékos {n} lappal kezd.',
  'prsi.rules.turn.match':
    'Játssz ki egy lapot, amely illik a felső lap színéhez vagy értékéhez — vagy húzz, ha nem tudsz.',
  'prsi.rules.turn.draw': 'A húzás kijátszás nélkül zárja le a köröd.',
  'prsi.rules.sevens':
    'Ha 7-est játszol ki, a következő játékos két lapot húz, hacsak nem válaszol saját 7-essel.',
  'prsi.rules.aces': 'Ha ászt játszol ki, a következő játékos köre kimarad.',
  'prsi.rules.queens': 'Játssz ki egy dámát, és mondd meg, melyik szín folytatódik.',
  'prsi.rules.end': 'A mérkőzés abban a pillanatban véget ér, amikor valakinek üres a keze.',

  'canasta.rules.section.goal': 'Cél',
  'canasta.rules.section.setup': 'Előkészítés',
  'canasta.rules.section.melding': 'Lerakás',
  'canasta.rules.section.end': 'Hogyan ér véget a mérkőzés',
  'canasta.rules.goal':
    'Párokban játsszák; az az oldal nyeri a mérkőzést, amelyik elsőként éri el a(z) {n} pontot.',
  'canasta.rules.deck': '{value} lappal játsszák — két pakli plusz jokerek.',
  'canasta.rules.deal': 'Minden játékos {n} lapot kap.',
  'canasta.rules.redThrees':
    'A kezedben lévő piros hármast azonnal fel kell mutatni, és bónuszként számít — kivéve ha az oldalad soha nem fejez be canastát, mert akkor ellened számít.',
  'canasta.rules.canasta': 'A canasta {n} vagy több azonos értékű lapból álló kombináció.',
  'canasta.rules.meldFloorBands':
    'Az első lerakásodnak el kell érnie egy pontminimumot, amely az állásoddal együtt emelkedik: {negative} nulla alatt, {low} 1500-ig, {mid} 3000-ig, {high} azon felül.',
  'canasta.rules.oneCanastaToGoOut': 'Egy befejezett canasta elég ahhoz, hogy az oldalad kiszálljon.',
  'canasta.rules.twoCanastasToGoOut':
    'Az oldaladnak két befejezett canastára van szüksége, mielőtt kiszállhatna.',
  'canasta.rules.end':
    'Addig osztanak újra, amíg az egyik oldal át nem lépi a(z) {n} pontot — akkor a mérkőzésnek vége.',

  'holdem.rules.section.goal': 'Cél',
  'holdem.rules.section.setup': 'Előkészítés',
  'holdem.rules.section.betting': 'Tétek',
  'holdem.rules.section.end': 'Hogyan ér véget a mérkőzés',
  'holdem.rules.goal':
    'Nyerj zsetont a legjobb lappal a lapfelfedésnél, vagy azzal, hogy egyedül maradsz a leosztásban.',
  'holdem.rules.stack': 'Minden hely {n} zsetonnal kezd.',
  'holdem.rules.blinds': 'A kis vak {sb}, a nagy vak {bb}, mindkettőt az osztás előtt teszik be.',
  'holdem.rules.streets': 'Négy körben licitálnak — a flop előtt, majd a flop, a turn és a river után.',
  'holdem.rules.showdown':
    'Akik még játékban vannak, felfedik a lapjaikat; a legjobb ötlapos kéz viszi a potot.',
  'holdem.rules.noLimit': 'Nincs limit — bármelyik tét mehet a teljes zsetonhalmodig.',
  'holdem.rules.lastPlayerStanding': 'Addig játszanak, amíg egy hely nem birtokolja az összes zsetont.',
  'holdem.rules.mostChipsWins': 'Aki a játék végén a legtöbb zsetont birtokolja, megnyeri a mérkőzést.',
  'holdem.rules.handLimit': 'A játék {n} leosztás után áll le.',

  // --- header --------------------------------------------------------------
  'header.deal': '{n}. leosztás',
  'header.gameOf': '{n}. játszma / {total}',
  'header.gameOfWithContract': '{n}. játszma / {total}: {contract}',

  // --- meld preview --------------------------------------------------------
  'preview.validSet': 'Érvényes csoport',
  'preview.validRun': 'Érvényes sor',
  'preview.validMeld': 'Érvényes kombináció',
  'preview.notYet': 'Ez még nem kombináció',
  'preview.points': '{shape} · {n} pont',
  'preview.pointsWithLaid': '{shape} · {n} + {laid} már lerakva = {total} pont',
  'preview.meetsFloor': '{line} (eléri: {n} ✓)',
  'preview.needsFloor': '{line} (kell: {n} ✗)',
  'preview.becauseOf': '{line} — {reason}',

  // --- discard -------------------------------------------------------------
  // A discard with a meld staged lays that meld first; this is what the
  // player is told when the server refuses it and the whole move is rolled
  // back. {reason} is the engine's own words for the refusal.
  'discard.meldRejected': '{reason} — semmit nem dobtunk el, a lapjaid továbbra is készen állnak.',

  // --- what the current selection would send -------------------------------
  // The client's own reason a control is not ready, next to the engine's own
  // `err.*` reasons rather than a second kind of message — a control greyed
  // out for "not your turn" and one greyed out for "you picked two, this
  // takes one" should read as the same kind of thing, not one of them looking
  // broken. See `fits` in `src/lib/drops.ts`.
  'sel.tooMany.1': 'Csak egy lapot válassz',
  'sel.tooMany.n': 'Legfeljebb {n} lapot válassz',
  'sel.needMore': 'Válassz {n} lapot',
  'sel.notThese': 'Ezek a lapok nem kerülhetnek ide',
  'sel.needsCompany': 'Ennek a lapnak a mellette lévőkre is szüksége van',

  // --- what a module says happened -----------------------------------------
  // The first module-sent keys in this bundle, and deliberately few. Anything
  // a module sends is still legible without an entry here — `humanise` turns
  // `holdem.seat.stack` into "Stack" — so a key earns a line only when that
  // fallback loses something. These do: what each of them means lives in its
  // *params* (who won, how much, whose cards), and a key on its own has
  // nowhere to put them. Without the entry the player read "Winner", full
  // stop, at the end of a match they had just won.
  'status.winner': 'Nyerte: {winners}',
  'holdem.status.pot': '{winners} nyert {amount} zsetont ezzel: {hand}',
  'holdem.status.potUncontested': '{winners} nyert {amount} zsetont — mindenki más bedobta',
  'holdem.status.shown': '{playerId} megmutatta: {value}',
  'holdem.prompt.waitingFor': 'Várakozás rá: {playerId}',

  // How a rummy scoreboard was ordered. Both earn a line for the same reason
  // as the keys above: the number lives in the params, and `humanise` would
  // render "Deals Won" with nothing after it.
  'zolik.standing.dealsWon': 'Nyert leosztások: {n}',
  'zolik.standing.inHand': 'Kézben: {n}',

  // --- rounds ---------------------------------------------------------------
  // What a round is called, and what one did. Each of these carries its number
  // in the params, so `humanise` alone would render a label with nothing after
  // it.
  // An instruction, not a heading. "Next round" named the thing rather than
  // asking for it, and sat above "Waiting for 1" — which reads as though the
  // player is the one waiting, when the table is waiting for them.
  'round.continue': 'Következő kör indítása',

  // --- the flash that announces a round or a match ending ------------------
  //
  // One line and one number, held for about two seconds. Worded generically on
  // purpose: which *kind* of ending it was is the module's own fact, printed
  // underneath, and a client that phrased "went gin" itself would be a client
  // that knows a game.
  'flash.roundWon': '{winners} vitte el',
  'flash.roundWonYou': 'Te vitted el',
  'flash.roundDrawn': 'Senki nem vitte el',
  'flash.matchOver': 'A mérkőzés véget ért',
  'flash.matchWon': '{winners} nyert',
  'flash.matchWonYou': 'Nyertél',
  'flash.matchDrawn': 'Senki nem nyert',
  'flash.nowOn': 'most {total}',

  'zolik.round.deal': 'Leosztás',
  'zolik.round.cleanRun': 'Egy sornak joker nélkülinek kell lennie',
  'canasta.round.deal': 'Leosztás',
  'canasta.round.concealed': 'Rejtve szállt ki',
  'canasta.round.exhausted': 'A pakli elfogyott',
  'canasta.round.meldCards': 'Lerakott lapok: {n}',
  'canasta.round.canastas': 'Canasták: {n}',
  'canasta.round.redThrees': 'Piros hármasok: {n}',
  'canasta.round.goingOut': 'Kiszállás: {n}',
  'canasta.round.inHand': 'Kézben maradt: {n}',
  'holdem.round.hand': 'Leosztás',
  'holdem.round.pot': 'Pot: {n}',
  'holdem.round.uncontested': 'Mindenki más bedobta',
  'seat.ready': 'Kész',
  'results.you': '(te)',



  // --- refusals the other three games raise ---------------------------------
  // Worded here for the same reason Žolíky's are: reasonText falls back to the
  // code, and OfferBar hands it the code as that fallback, so an unworded
  // refusal reaches a player as PILE_FROZEN in capitals. serverKeys.test.ts
  // now fails the build rather than letting the next one through.
  'err.SET_TOO_LARGE': 'A csoportban már mind a négy szín szerepel',
  'err.DISCARD_TAKEN_CARD_FORBIDDEN': 'Nem dobhatod el az imént felvett lapot — játszd ki vagy tartsd meg',
  'err.CARD_DOES_NOT_FIT': 'Ez a lap sem színben, sem értékben nem illik',
  'err.SUIT_REQUIRED': 'Mondd meg, melyik szín folytatódik',
  'err.MUST_ANSWER_DRAW_OR_TAKE': 'Válaszolj hetessel, vagy vedd fel a lapokat',
  'err.NOTHING_TO_DRAW': 'Nem maradt mit húzni',
  'err.PILE_EMPTY': 'A pakli üres',
  'err.PILE_BLOCKED': 'A pakli le van zárva — fekete hármas van a tetején',
  'err.PILE_FROZEN': 'A pakli be van fagyasztva — két természetes lap kell a felső lap értékéből',
  'err.TOP_CARD_UNUSABLE': 'A felső lapot nem tudod használni',
  'err.MELD_CLOSED': 'Ez a kombináció teljes és lezárt',
  'err.MELD_TOO_SMALL': 'Egy kombinációhoz ennél több lap kell',
  'err.MELD_TOO_LARGE': 'Ez a kombináció már nem fogad be több lapot',
  'err.MELD_MIXED_RANKS': 'Egy kombináció minden lapjának azonos értékűnek kell lennie',
  'err.NOT_ENOUGH_NATURALS': 'Egy kombinációban több természetes lapnak kell lennie, mint jokernek',
  'err.RANK_ALREADY_MELDED': 'Az oldalodnak már van ilyen értékű kombinációja',
  'err.NOT_YOUR_MELD': 'Ez a kombináció az ellenfél oldaláé',
  'err.NO_SUCH_MELD': 'Ez a kombináció nincs az asztalon',
  'err.CANNOT_MELD_THREE': 'A hármasokat soha nem rakják le',
  'err.CANNOT_DISCARD_RED_THREE': 'Piros hármast nem lehet eldobni',
  'err.MUST_KEEP_A_CARD': 'Tarts meg legalább egy lapot — így nem ürítheted ki a kezed',
  'err.MUST_MELD_FIRST': 'Előbb rakd le az oldalad nyitását',
  'err.INITIAL_MELD_NOT_MET': 'Az első lerakásodból még hiányoznak pontok',
  'err.CANNOT_GO_OUT_YET': 'Az oldaladnak befejezett canastára van szüksége, mielőtt kiszállhatna',
  'err.NOTHING_TO_CALL': 'Nincs megtartható tét',
  'err.CANNOT_CHECK': 'Nem passzolhatsz — van tét, amire válaszolni kell',
  'err.CANNOT_RAISE': 'Itt nem emelhetsz',
  'err.RAISE_TOO_SMALL': 'Az emelésnek legalább akkorának kell lennie, mint az előző',
  'err.NOT_ENOUGH_CHIPS': 'Nincs ennyi zsetonod',
  'err.AMOUNT_REQUIRED': 'Mondd meg, mennyit',
  'err.AMOUNT_NOT_A_NUMBER': 'Ez az összeg nem szám',
  'err.SEAT_NOT_IN_HAND': 'Nem vagy benne ebben a leosztásban',
  'err.WRONG_RANK': 'Ez a lap ehhez rossz értékű',
  'err.MATCH_FULL': 'Az asztal megtelt',
  'err.MATCH_ALREADY_STARTED': 'A mérkőzés már elkezdődött',
  'err.TOO_FEW_PLAYERS': 'Még nincs elég játékos',
  'err.WRONG_PLAYER_COUNT': 'Ezt a játékot ennyi játékossal nem lehet játszani',
  'err.NOT_THE_HOST': 'Ezt csak a házigazda teheti meg',
  'err.NO_LONGER_WAITING': 'Az asztal már nem vár',
  'err.WAITING_ROOM_UNAVAILABLE': 'A váróterem nem érhető el',
  'err.SERVER_BUSY': 'A kiszolgáló most tele van — próbáld újra egy pillanat múlva',


  // --- rules the engine has always enforced and never stated ----------------
  // Every one of these was found by the guardrail rather than by review: the
  // validator could refuse a player for it, and no sentence anywhere said so.
  // See server/internal/zolikmod/ruleindex_test.go.
  'zolik.rules.section.layoff': 'Hozzárakás kombinációkhoz',
  'zolik.rules.pickup.obligation':
    'Amíg nem vagy lerakva, a dobópakliból elvett lapot abban a kombinációban kell felhasználni, amellyel ebben a körben lerakod magad.',
  'zolik.rules.pickup.noReturn':
    'A dobópakliból elvett lapot ugyanabban a körben nem lehet újra eldobni — játszd ki vagy tartsd meg.',
  'zolik.rules.wilds.setLimit': 'Egy csoportban nem lehet több joker, mint természetes lap.',
  'zolik.rules.set.maxSize':
    'Egy csoport nem tartalmazhat {n} lapnál többet — a joker egy hiányzó színt pótol, nem egészít ki egy teljes csoportot.',
  'zolik.rules.run.maxLength':
    'Egy sor nem tartalmazhat {n} lapnál többet — az ász alul, a tizenkét érték fölötte, és az ász felül.',
  'zolik.rules.run.aceBridge':
    'Az ász a király fölött vagy a kettes alatt áll, soha nem hídként a sor két vége között.',
  'zolik.rules.contracts.contribution':
    'Amíg nem vagy lerakva, minden lerakott kombinációnak olyannak kell lennie, amilyet a leosztás szerződése még kér.',
  'zolik.rules.layoff.afterDown':
    'Mások kombinációihoz nem rakhatsz hozzá, amíg le nem raktad a saját szerződésedet.',
  'zolik.rules.layoff.runEnds':
    'A sorhoz hozzárakott lapnak azt az egyik vagy a másik végén kell folytatnia.',
  'zolik.rules.jokers.swap':
    'Az asztalon lévő kombináció jokerét pontosan azzal a lappal lehet kiváltani, amelyet helyettesít.',
  'zolik.rules.jokers.reclaim.on':
    'Az asztalról kiváltott jokert ugyanabban a körben kombinációba kell játszani — nem maradhat a kézben.',
  'zolik.rules.jokers.reclaim.off': 'Az asztalról kiváltott joker a kézben maradhat.',
  'zolik.rules.deck.reshuffle':
    'Ha a húzópakli elfogy, a dobópaklit megkeverik, és az lesz az új húzópakli; ha mindkettő üres, a leosztás véget ér.',


  // --- what to do instead ---------------------------------------------------
  // The third layer of a refusal, after the reason and the rule. Sent by the
  // module because it is the only side that knows which card is owed and
  // which way out is on offer; see server/internal/zolikmod/remedy.go.
  'zolik.remedy.meldThePickup': 'Tedd hozzá a(z) {card} lapot a lerakásodhoz, vagy vond vissza a felvételt.',
  'zolik.remedy.discardSomethingElse':
    'Dobj el másik lapot, vagy játszd ki a(z) {card} lapot ebben a körben.',
  'zolik.remedy.discardNotAJoker': 'Dobj el valami mást, ne jokert.',
  'zolik.remedy.finishOrUndoLayDown': 'Fejezd be a lerakást, vagy vedd vissza.',
  'zolik.remedy.needMorePoints': 'Még {n} pontra van szükséged, hogy lerakhass.',
  'zolik.remedy.layACleanRun': 'Rakj le egy sort joker nélkül.',
  'zolik.remedy.playReclaimedJoker': 'Játszd a(z) {card} lapot kombinációba, vagy vond vissza az elvételt.',
  'zolik.remedy.goDownFirst': 'Előbb rakd le a saját kombinációidat.',
  'zolik.remedy.drawFirst': 'Előbb húzz egy lapot.',
  'zolik.remedy.drawFromStock': 'Húzz a húzópakliból — a dobópakli a(z) {n}. körben nyílik meg.',
  'zolik.remedy.drawFromStockEmpty': 'Húzz inkább a húzópakliból.',


  // --- keys whose meaning lives in their params -----------------------------
  // The shape fallback renders these with the number missing ("Stack", "Pot"),
  // which is why they earn a line where most keys do not.
  'header.contract': 'Kell: {sets} csoport és {runs} sor',
  'header.contract.cleanRunOnly': 'Joker nélküli sor kell',
  'header.round': '{n}. kör',
  'header.deck': 'Húzópakli',
  'header.target': 'Cél',
  'header.suitInPlay': 'Játékban lévő szín',
  'seat.cards': 'Lapok',
  'zolik.offer.meld': 'Lerak',
  'prompt.pickupMustBeMelded':
    'A(z) {value} a dobópakliból jött — abba a kombinációba kell kerülnie, amellyel ebben a körben lerakod magad.',
  'prompt.jokerMustBePlayed':
    'A(z) {value} az asztalról jött — kombinációba kell kerülnie, mielőtt lezárhatnád a köröd.',
  'prompt.initialMeld': 'Az oldalad nyitásának el kell érnie a(z) {n} pontot.',
  'prompt.canastasNeeded': 'Az oldaladnak még {n} canastára van szüksége, hogy kiszállhasson.',
  'prompt.mustDrawOrAnswerSeven': 'Válaszolj hetessel, vagy húzz {n} lapot.',
  'prompt.chooseSuit': 'Válaszd ki a folytatódó színt',
  'prompt.skipPending': 'A köröd kimarad',
  'status.lastDeal': 'A(z) {team} csapat {value} pontot szerzett',
  'status.teamScore': '{team} csapat: {value}',
  'canasta.offer.rank': 'Érték',
  'canasta.seat.teamScore': 'Csapatpontszám',
  'canasta.seat.canastas': 'Canasták',
  'holdem.header.pot': 'Pot',
  'holdem.header.street': 'Kör',
  'holdem.header.hand': 'Leosztás',
  'holdem.header.handLimit': 'Leosztás összesen',
  'holdem.header.blinds': 'Vakok',
  'holdem.cost.call': 'a megtartáshoz',
  'holdem.cost.pot': 'a potban',
  'holdem.seat.stack': 'Zsetonhalom',
  'holdem.seat.bet': 'Tét',
  'holdem.prompt.yourAction': 'Te következel',
  'holdem.prompt.raiseTo': 'Emelés eddig',
  'zone.yourHand': 'A kezed',
  'zone.opponentHand': 'Az ellenfél keze',
  'zone.drawPile': 'Húzópakli',
  'zone.discardPile': 'Dobópakli',
  'zone.melds': 'Kombinációk',
  'zone.teamMelds': 'Az oldalad kombinációi',
  'zone.redThrees': 'Piros hármasok',
  'zone.board': 'Asztal',
  'verb.drawFromDeck': 'Húzás',
  'verb.takeFromDiscard': 'Elvétel a pakliból',


  // --- the why sheet's own furniture ---------------------------------------
  // The three layers a refusal is explained in. Labels, not sentences: what
  // goes under each is sent by the server or worded above.
  'why.reason': 'Miért nem',
  'why.rule': 'A szabály',
  'why.rules': 'A szabályok',
  'why.remedy': 'Mit tehetsz',
  'why.readTheRules': 'Teljes szabályok elolvasása →',
  'why.close': 'Bezárás',
  'why.open': 'miért',

  // --- marks on a particular card ------------------------------------------
  // A mark is a refusal that has not happened yet: the rule is enforced at the
  // discard, which is the last possible moment to hear about it, so the card
  // says so while there is still a turn left to act on it.
  'zolik.badge.owedToMeld':
    'A(z) {card} a dobópakliból jött — abba a kombinációba kell kerülnie, amellyel ebben a körben lerakod magad.',
  'zolik.badge.jokerOwed':
    'A(z) {card} az asztalról jött — kombinációba kell kerülnie, mielőtt lezárhatnád a köröd.',

  // --- the legal notices ----------------------------------------------------
  // Only the furniture. The documents themselves are in `src/legal`, which is
  // a bundle of the same kind with a parity test of its own — prose that long
  // in a flat key map buries the keys this one exists for.
  //
  // The notice is five fragments rather than one sentence with two links glued
  // in, because Czech does not put the link where English does: "souhlasíš s
  // Podmínkami" inflects the noun the link is made of. Fragments let each
  // locale place and decline its own.
  'legal.terms': 'Feltételek',
  'legal.privacy': 'Adatvédelem',
  'legal.source': 'Forráskód',
  'legal.updated': '{version}. verzió',
  'legal.draft':
    'Tervezet — még nincs hatályban. Az üzemeltető nevét, országát és kapcsolattartási címét még ki kell tölteni.',
  'legal.notice.before': 'A játékkal elfogadod a ',
  'legal.notice.terms': 'felhasználási feltételeket',
  'legal.notice.between': '. Hogy mit tárolunk rólad, azt az ',
  'legal.notice.privacy': 'adatvédelmi tájékoztató',
  'legal.notice.after': ' írja le.',
  // --- gin rummy -------------------------------------------------------------
  'err.UPCARD_DECLINED': 'Ezt a lapot már egyszer passzoltad',
  'err.DEADWOOD_TOO_HIGH': 'A deadwoodod túl magas a kopogáshoz',
  'err.CARD_DOES_NOT_EXTEND_MELD': 'Ez a lap nem hosszabbítja meg ezt a kombinációt',
  'ginrummy.rules.setup': 'Előkészítés',
  'ginrummy.rules.turn': 'A te köröd',
  'ginrummy.rules.melds': 'Kombinációk',
  'ginrummy.rules.knocking': 'Kopogás',
  'ginrummy.rules.bigGin': 'Big gin',
  'ginrummy.rules.layoff': 'A hozzárakás',
  'ginrummy.rules.deadHand': 'A holt leosztás',
  'ginrummy.rules.scoring': 'Egy leosztás pontozása',
  'ginrummy.rules.match': 'A mérkőzés megnyerése',
  'ginrummy.rules.lineBonuses': 'Bónuszok az elszámolásban',
  'ginrummy.rules.deck': '{value} lapos paklival játsszák.',
  'ginrummy.rules.deal': 'Minden játékos {value} lapot kap.',
  'ginrummy.rules.upcard': 'Még egy lapot felfordítanak, ezzel indul a dobópakli.',
  'ginrummy.rules.drawDiscard':
    'A körödben húzz egy lapot — a húzópakliból vagy a dobópakliból —, majd dobj el egyet.',
  'ginrummy.rules.setsAndRuns':
    'A kombináció három vagy négy azonos értékű lapból álló csoport, vagy három vagy több azonos színű lapból álló sor.',
  'ginrummy.rules.aceLow': 'Az ász mindig alacsony — dámától ászig nincs sor.',
  'ginrummy.rules.knockLimit': 'Kopoghatsz, amint a deadwoodod {n} vagy kevesebb.',
  'ginrummy.rules.oklahoma': 'Ebben a leosztásban a kopogási határt a felfordított lap értéke szabja meg.',
  'ginrummy.rules.gin': 'A nulla deadwood a gin — a lehető legjobb kopogás.',
  'ginrummy.rules.bigGinBonus':
    'Tizenegy lap mind kombinációban, dobás nélkül: ez a big gin, és további {n} pontot ér.',
  'ginrummy.rules.layoffDescription':
    'Olyan kopogás után, amely nem gin, az ellenfeled a saját deadwoodját hozzárakhatja a te kombinációidhoz, mielőtt a kezeket összehasonlítják.',
  'ginrummy.rules.deadHandDescription':
    'Ha a húzópakli az utolsó két lapjára fogy, és senki nem kopogott, a leosztás holt — senki nem kap pontot, és ugyanaz az osztó oszt újra.',
  'ginrummy.rules.undercut':
    'Ha az ellenfeled deadwoodja nem magasabb a tiédnél, alávág: ő kapja a különbséget, plusz {n}.',
  'ginrummy.rules.ginBonus': 'A gin az ellenfeled teljes kezét hozza, plusz {n}.',
  'ginrummy.rules.target': 'Aki egy leosztás végén elsőként lépi át a(z) {n} pontot, megnyeri a mérkőzést.',
  'ginrummy.rules.shutout':
    'A mérkőzésbónusz {n} pontra duplázódik, ha a vesztes egyetlen pontot sem szerzett.',
  'ginrummy.rules.box': 'Minden megnyert leosztás {n} pontot ér a mérkőzés végén.',
  'ginrummy.rules.gameBonus': 'A mérkőzés megnyerése további {n} pontot ér.',
  'ginrummy.fact.deadwood': '{value} deadwood',
  'ginrummy.fact.discardCard': 'Dobd el: {value}',
  'ginrummy.fact.meldCards': 'Ehhez: {value}',
  'ginrummy.header.hand': '{n}. leosztás',
  'ginrummy.round.kind': '{value}',
  'ginrummy.round.hand': 'Leosztás',
  'ginrummy.seat.score': '{value}',
  'ginrummy.seat.dealer': 'Osztó',
  'ginrummy.status.knocked': '{playerId} kopogott {deadwood} deadwooddal',
  'ginrummy.status.gin': '{playerId} gint csinált',
  'ginrummy.status.lastHand': 'Utolsó leosztás: {winner} ({kind}, {delta} pont)',
  'ginrummy.offer.drawStock': 'Húzás a húzópakliból',
  'ginrummy.offer.drawDiscard': 'Húzás a dobópakliból',
  'ginrummy.offer.takeUpcard': 'Felfordított lap elvétele',
  'ginrummy.offer.passUpcard': 'Passz',
  'ginrummy.offer.discard': 'Eldobás',
  'ginrummy.offer.knock': 'Kopogás',
  'ginrummy.offer.gin': 'Gin!',
  'ginrummy.offer.bigGin': 'Big gin!',
  'ginrummy.offer.layOff': 'Hozzárakás',
  'ginrummy.offer.finishLayoff': 'Hozzárakás kész',
  'ginrummy.zone.knockerHand': 'A kopogó keze',
  'ginrummy.zone.melds': 'Kombinációk',
  'ginrummy.prompt.upcardDecision': 'Vedd el a felfordított lapot, vagy passzolj',
  'ginrummy.prompt.yourTurnDraw': 'Húzz egy lapot',
  'ginrummy.prompt.yourTurnDiscard': 'Dobj el egy lapot — vagy kopogj, ha tudsz',
  'ginrummy.prompt.layoff': 'Rakd hozzá a deadwoodot, vagy fejezd be',

  // --- rummy tiles -------------------------------------------------------------
  'err.TILE_NOT_IN_HAND': 'Ez a lapka nincs a kezedben',
  'err.TILE_DOES_NOT_FIT': 'Ez oda nem illik',
  'err.NO_SUCH_SET': 'Ez a kombináció nincs az asztalon',
  'err.INITIAL_MELD_ONLY': 'Az első lerakásod előtt csak a saját új kombinációidat rendezheted át',
  'err.TABLE_NOT_VALID': 'Az asztal még nem érvényes',
  'err.TRAY_NOT_EMPTY': 'Még vannak elhelyezetlen lapkáid',
  'err.NOTHING_PLAYED': 'Játssz ki legalább egy lapkát, mielőtt lezárnád a köröd',
  'err.INITIAL_MELD_TOO_LOW': 'Az első lerakásodnak legalább 30 pontot kell érnie',
  'err.NOT_A_RUN': 'Csak sort lehet kettévágni',
  'err.BAD_SPLIT_POSITION': 'Ez a sor ott nem vágható ketté',
  'err.NO_JOKER_IN_SET': 'Ebben a kombinációban nincs joker',
  'err.TILE_JOKER_SWAP_MISMATCH': 'Ez a lapka nem az, amit a joker helyettesít',
  'rummytiles.rules.setup': 'Előkészítés',
  'rummytiles.rules.sets': 'Kombinációk',
  'rummytiles.rules.initialMeld': 'Az első lerakás',
  'rummytiles.rules.turn': 'A te köröd',
  'rummytiles.rules.jokerTaking': 'Joker elvétele',
  'rummytiles.rules.ending': 'Egy kör lezárása',
  'rummytiles.rules.poolExhaustion': 'Ha a készlet kifogy',
  'rummytiles.rules.match': 'A mérkőzés megnyerése',
  'rummytiles.rules.tiles': '{value} lapkával játsszák.',
  'rummytiles.rules.dealCount': 'Minden játékos {value} lapkát kap.',
  'rummytiles.rules.group': 'A csoport három vagy négy azonos számú lapka, mindegyik más színben.',
  'rummytiles.rules.run': 'A sor három vagy több egymást követő szám azonos színben.',
  'rummytiles.rules.noWrap': 'A 13 után nem kezdődik újra az 1.',
  'rummytiles.rules.joker': 'A joker bármelyik lapkát helyettesíti.',
  'rummytiles.rules.initialMeldDescription':
    'Amíg egyetlen körben, kizárólag a saját kezedből, nem raktál le {n} vagy több pontot, semmihez nem nyúlhatsz, ami már az asztalon van.',
  'rummytiles.rules.turnDescription':
    'Játssz ki legalább egy lapkát a kezedből, rendezd át az asztalt szabadon, és úgy fejezd be, hogy az asztalon minden kombináció érvényes legyen.',
  'rummytiles.rules.noDiscard':
    'Nincs eldobás — ha nem tudsz érvényes kört befejezni, helyette húzol egy lapkát.',
  'rummytiles.rules.jokerTakingDescription':
    'Az asztalon lévő jokert úgy veheted el, hogy a kezedből kicseréled arra a lapkára, amelyet helyettesít — és a köröd vége előtt kombinációban kell felhasználnod.',
  'rummytiles.rules.goingOut':
    'A kört az a játékos nyeri, akinek elsőként fogynak el a lapkái. Mindenki más a nála maradt lapkák értékét negatívan kapja meg; a győztes az összes többi veszteségének összegét kapja.',
  'rummytiles.rules.poolExhaustionLowestWins':
    'Ha a készlet kifogy, és senki nem tud játszani, a kör véget ér, és a legalacsonyabb kézérték nyeri.',
  'rummytiles.rules.poolExhaustionNoWinner':
    'Ha a készlet kifogy, és senki nem tud játszani, a kör győztes nélkül ér véget — minden kezet egyszerűen kiértékelnek.',
  'rummytiles.rules.target': 'Aki egy kör végén elsőként lépi át a(z) {n} pontot, megnyeri a mérkőzést.',
  'rummytiles.rules.roundLimit': 'A mérkőzés {n} kör után ér véget — a legmagasabb pontszám nyer.',
  'rummytiles.fact.setCards': '{value}',
  'rummytiles.header.pool': 'Készlet: {n}',
  'rummytiles.header.round': '{n}. kör',
  'rummytiles.round.kind': '{value}',
  'rummytiles.round.round': 'Kör',
  'rummytiles.seat.score': '{value}',
  'rummytiles.seat.notOpened': 'Nem nyitott',
  'rummytiles.status.lastRound': 'Utolsó kör: {winner} ({kind})',
  'rummytiles.badge.invalid': 'Még nem érvényes',
  'rummytiles.zone.pool': 'Készlet',
  'rummytiles.zone.table': 'Asztal',
  'rummytiles.zone.tray': 'Tartó',
  'rummytiles.offer.place': 'Elhelyezés',
  'rummytiles.offer.addFromHand': 'Hozzáadás',
  'rummytiles.offer.addFromTray': 'Hozzáadás a tartóból',
  'rummytiles.offer.take': 'Elvétel',
  'rummytiles.offer.split': 'Kettévágás',
  'rummytiles.offer.swapJoker': 'Joker cseréje',
  'rummytiles.offer.resetTurn': 'Kör visszaállítása',
  'rummytiles.offer.commit': 'Kész',
  'rummytiles.offer.draw': 'Húzás',
  'rummytiles.param.position': 'Vágás itt',

  // --- blackjack -----------------------------------------------------------
  //
  // The house rules are worded twice over — on and off — because a rule that
  // is *not* in force is still something a player has to be told. "No
  // surrender at this table" is information; silence is a guess.
  'err.BET_BELOW_MINIMUM': 'Ez az asztal minimuma alatt van',
  'err.ALREADY_BET': 'A tétedet már megtetted',
  'err.INSURANCE_CLOSED': 'Most nincs felvehető biztosítás',
  'err.CANNOT_DOUBLE': 'Ezt a kezet nem lehet duplázni',
  'err.CANNOT_SPLIT': 'Ezt a kezet nem lehet szétosztani',
  'err.CANNOT_SURRENDER': 'Ezt a kezet nem lehet feladni',

  'blackjack.rules.section.table': 'Az asztal',
  'blackjack.rules.section.play': 'Egy kéz lejátszása',
  'blackjack.rules.section.dealer': 'Az osztó',
  'blackjack.rules.section.end': 'Hogyan ér véget a mérkőzés',
  'blackjack.rules.goal':
    'Győzd le az osztót anélkül, hogy huszonegy fölé mennél. A túllépés azonnal veszít, bármit tegyen is utána az osztó.',
  // Counts are their own phrase rather than a number glued to a noun: one
  // deck and six decks inflect differently, and Czech inflects them again.
  'blackjack.rules.decks': 'Paklik a cipőben: {n}.',
  'blackjack.rules.stack': 'Minden hely {n} zsetonnal ül le.',
  'blackjack.rules.minBet': 'Az asztal minimuma {n} zseton.',
  'blackjack.rules.faceUp':
    'A játékosok lapjait nyíltan osztják; az osztó egy lapot lefordítva tart, amíg mindenki le nem játszott.',
  'blackjack.rules.hitStand': 'Húzz annyi lapot, amennyit akarsz, vagy maradj annál, amid van.',
  'blackjack.rules.aces': 'Az ász tizenegyet ér, amíg belefér, egyébként egyet.',
  'blackjack.rules.blackjack': 'Ász egy tízértékű lappal, az első két lapon: ez a blackjack.',
  'blackjack.rules.pays3to2': 'A blackjack 3:2-t fizet.',
  'blackjack.rules.pays6to5': 'A blackjack 6:5-öt fizet.',
  'blackjack.rules.paysEven': 'A blackjack egy az egyben fizet.',
  'blackjack.rules.double':
    'Az első két lapodon megduplázhatod a tétedet, és pontosan egy további lapot kapsz.',
  'blackjack.rules.doubleAfterSplit': 'A szétosztásból származó kezet is meg lehet duplázni.',
  'blackjack.rules.noDoubleAfterSplit': 'A szétosztásból származó kezet nem lehet megduplázni.',
  'blackjack.rules.split':
    'Két azonos értékű lapot külön kezekre lehet osztani, mindegyiket saját téttel — legfeljebb {n} alkalommal, összesen {hands} kézre.',
  'blackjack.rules.noSplit': 'Ennél az asztalnál a párokat nem osztják szét.',
  'blackjack.rules.splitAces':
    'A szétosztott ászok egy-egy lapot kapnak, majd megállnak, és az így elért huszonegy nem blackjack.',
  'blackjack.rules.surrender':
    'Az első kezedet a tét feléért feladhatod, miután az osztó ellenőrizte a blackjacket.',
  'blackjack.rules.noSurrender': 'Ennél az asztalnál a kezeket nem lehet feladni.',
  'blackjack.rules.dealerDraws': 'Az osztó tizenhétig húz, aztán megáll.',
  'blackjack.rules.hitsSoft17': 'Az osztó ásszal alkotott tizenhétnél is húz.',
  'blackjack.rules.standsSoft17': 'Az osztó megáll az ásszal alkotott tizenhétnél.',
  'blackjack.rules.dealerPeeks':
    'Ha ászt vagy tízest mutat, az osztó ellenőrzi a blackjacket, mielőtt bárki játszana.',
  'blackjack.rules.insurance':
    'Az osztó ásza ellen a téted feléért biztosíthatsz; 2:1-et fizet, ha az osztónak blackjackje van.',
  'blackjack.rules.noInsurance': 'Ennél az asztalnál nincs biztosítás.',
  'blackjack.rules.rounds': 'Az asztalnál {n} kört játszanak.',
  'blackjack.rules.mostChipsWins': 'Aki a végén a legtöbb zsetont birtokolja, megnyeri a mérkőzést.',
  'blackjack.rules.bustedOut':
    'Az a hely, amely már nem tudja fedezni a(z) {n} minimumot, a mérkőzés hátralévő részében kimarad.',

  'blackjack.zone.dealer': 'Osztó',
  'blackjack.zone.box': 'Kéz',
  'blackjack.zone.yourBox': 'A kezed',
  'blackjack.zone.shoe': 'Cipő',

  'blackjack.header.round': '{n}. kör / {of}',
  'blackjack.header.minBet': 'Minimum',
  'blackjack.header.decks': 'Paklik',
  'blackjack.header.dealerTotal': 'Az osztó mutat: {n}',
  'blackjack.header.dealerSoftTotal': 'Az osztó lágy {n}-et mutat',

  'blackjack.seat.stack': 'Zsetonok',
  'blackjack.seat.bet': 'Tét',
  'blackjack.seat.insurance': 'Biztosítás',
  'blackjack.seat.total': 'Összesen',
  'blackjack.seat.softTotal': 'Lágy összeg',
  'blackjack.seat.out': 'Elfogytak a zsetonok',

  'blackjack.prompt.placeBet': 'Tedd meg a tétedet',
  'blackjack.prompt.insurance': 'Biztosítás?',
  'blackjack.prompt.yourMove': 'Te következel',
  'blackjack.prompt.waitingFor': 'Várakozás rá: {playerId}',
  'blackjack.prompt.betAmount': 'Tét',

  'blackjack.offer.bet': 'Tét',
  'blackjack.offer.hit': 'Lapot',
  'blackjack.offer.stand': 'Megállok',
  'blackjack.offer.double': 'Duplázás',
  'blackjack.offer.split': 'Szétosztás',
  'blackjack.offer.surrender': 'Feladás',
  'blackjack.offer.insure': 'Biztosítás kérése',
  'blackjack.offer.declineInsurance': 'Nem kérek biztosítást',

  'blackjack.fact.tableMinimum': 'minimum',
  'blackjack.fact.insuranceCost': 'a biztosításhoz',
  'blackjack.fact.extraStake': 'tétnek',
  'blackjack.fact.surrenderReturn': 'vissza',

  'blackjack.status.dealerBlackjack': 'Az osztónak blackjackje volt',
  'blackjack.status.dealerBust': 'Az osztó besokallt {n}-nél',
  'blackjack.status.dealerStands': 'Az osztó megáll {n}-nél',

  'blackjack.round.name': 'Kör',
  'blackjack.round.dealerTotal': 'Osztó: {n}',
  'blackjack.round.dealerBust': 'Az osztó besokallt ({n})',
  'blackjack.round.dealerBlackjack': 'Az osztó blackjackje',
  'blackjack.round.outcome.blackjack': 'Blackjack',
  'blackjack.round.outcome.win': 'Nyert',
  'blackjack.round.outcome.push': 'Döntetlen',
  'blackjack.round.outcome.lose': 'Vesztett',
  'blackjack.round.outcome.bust': 'Besokallt',
  'blackjack.round.outcome.surrender': 'Feladva',

  'blackjack.badge.inPlay': 'Játékban',
  'blackjack.badge.doubled': 'Duplázva',
  'blackjack.badge.split': 'Szétosztva',
  'blackjack.badge.blackjack': 'Blackjack',
  'blackjack.badge.bust': 'Besokallt',
  'blackjack.badge.won': 'Nyert',
  'blackjack.badge.push': 'Döntetlen',
  'blackjack.badge.lost': 'Vesztett',
  'blackjack.badge.surrendered': 'Feladva',

  'blackjack.unit.chips': 'zseton',

  // --- the settings screen --------------------------------------------------
  //
  // Hardcoded English until now, which was survivable while the only other
  // language was unreachable and faintly absurd once the language picker
  // itself lived on this screen: a player who cannot read "Settings" is
  // exactly the one who came here to change it.
  'settings.title': 'Beállítások',
  'settings.subtitle': 'Hogy nézel ki te, és hogy néz ki az asztal',
  'settings.face.heading': 'Az arcod az asztalnál',
  'settings.face.account': 'A fiókodhoz mentve, így elkísér egy másik eszközre is.',
  'settings.face.device': 'Ezen az eszközön tárolva. Jelentkezz be, hogy magaddal vidd.',
  'settings.skin.heading': 'Az asztal kinézete',
  'settings.language.heading': 'Nyelv',
  'settings.language.status': 'Ezen az eszközön tárolva.',
  'settings.language.auto': 'Automatikus',
  'settings.language.auto.now': 'Az eszközödet követi — most {language}',
  'settings.legal.heading': 'Az apró betűs rész',
  'settings.legal.status': 'Mihez járultál hozzá a játékkal, és mit tárolunk rólad.',
  'settings.signIn': 'Bejelentkezés',
  'settings.back': 'Vissza',

  // A translated interface over an English notice is a worse lie than an
  // English interface, so the document says so itself rather than letting the
  // surrounding screen imply the text was written for this reader.
  'legal.untranslated':
    'Ezt a tájékoztatót még nem fordítottuk le a nyelvedre. Az alábbi angol szöveg az érvényes változat.',

  // --- the navigation bar's screen titles ------------------------------------
  //
  // These sit in `app/_layout.tsx`, above every screen, and were the last
  // English left after the settings screen was translated — a German player
  // reading "Einstellungen" under a bar that said "Settings". A translated app
  // with an untranslated chrome bar looks like a bug, not like a limit.
  'nav.home': 'Zolik',
  'nav.emailSignIn': 'Belépés e-maillel',
  'nav.signingIn': 'Belépés folyamatban',
  'nav.usernameSignIn': 'Belépés felhasználónévvel',
  'nav.legacyAccount': 'Régi fiók',
  'nav.guest': 'Vendég',
  'nav.account': 'Fiók',
  'nav.games': 'Játékok',
  'nav.table': 'A te asztalod',
  'nav.join': 'Csatlakozás asztalhoz',
  // The screen a shared invite link lands on while it resolves the code.
  'nav.joining': 'Csatlakozás',
  'nav.rules': 'Szabályok',
  'nav.match': 'Mérkőzés',
  'nav.scoreTable': 'Ponttábla',
  'nav.stats': 'Statisztika',
};
