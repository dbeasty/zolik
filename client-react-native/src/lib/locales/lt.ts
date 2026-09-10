/**
 * Lithuanian. Rummy vocabulary: grupė for a set, seka for a run, derinys for a meld, kaladė for the stock, džokeris for a joker.
 */

export const lt: Record<string, string> = {
  // --- engine error codes (rules.RulesErrorCode) ---------------------------
  'err.NOT_YOUR_TURN': 'Ne tavo eilė',
  'err.WRONG_PHASE': 'Šiuo metu negalima',
  'err.MUST_DRAW_FIRST': 'Prieš išdėdamas paimk kortą',
  'err.GAME_SUSPENDED': 'Žaidimas pristabdytas',
  'err.GAME_NOT_ACTIVE': 'Žaidimas nevyksta',
  // --- runtime refusals, which are not the rules saying no -----------------
  //
  // Both were reachable long before they were worded, and a player who met
  // one was shown the code in capitals or, worse, nothing at all: an action
  // sent while the socket was down simply vanished, which reads as a legal
  // move being silently refused.
  'err.MATCH_NOT_ACTIVE': 'Stalas pristabdytas — laukiama, kol žaidėjas vėl prisijungs',
  'err.NOT_CONNECTED': 'Nėra ryšio su stalu — jungiamės iš naujo, paskui bandyk dar kartą',
  // Why the "start the next round" control is greyed out. Without wording the
  // shell printed the code itself, so a player agreeing to go on was answered
  // with ALREADY_READY in capitals.
  'err.ALREADY_READY': 'Esi pasiruošęs',
  'err.NOT_BETWEEN_ROUNDS': 'Raundas dar vyksta',
  'err.NOT_AT_THIS_TABLE': 'Tu nesėdi prie šio stalo',
  // Bringing a table back that the sweeper set aside. Refusals a player
  // can actually provoke: a link to a table that is gone, one that was
  // already picked up in another tab, and a table with other people at it.
  'err.MATCH_MOVED_ON': 'Stalas pajudėjo toliau — perkrauk puslapį',
  'err.MATCH_NOT_ABANDONED': 'Šis stalas nelaukia atnaujinimo',
  'err.MATCH_NOT_FOUND': 'Šio stalo nebėra',
  'err.TABLE_HAS_OTHER_PLAYERS': 'Atnaujinti galima tik stalą, prie kurio visi kiti yra botai',
  'err.DISCARD_LOCKED': 'Atmetimo krūvelė kol kas užrakinta',
  'err.DISCARD_PILE_EMPTY': 'Atmetimo krūvelė tuščia',
  'err.NO_CARDS_LEFT': 'Nebėra kortų traukti',
  'err.ROUND_REQ_NOT_MET': 'Pirma išdėk savo pradinį derinį',
  'err.NEED_CLEAN_RUN': 'Kad būtum laikomas išdėjusiu, ant stalo tau reikia sekos be džokerio',
  'err.INCOMPLETE_INITIAL_MELD': 'Užbaik išdėjimą arba jį atšauk, prieš atmesdamas kortą',
  'err.DISCARD_CARD_NOT_MELDED': 'Paimta korta turi patekti į tavo derinį',
  'err.JOKER_DISCARD_FORBIDDEN': 'Džokerio atmesti negalima',
  'err.NOTHING_TO_UNDO': 'Nėra ko atšaukti',
  'err.NO_JOKER_IN_MELD': 'Šiame derinyje džokerio nėra',
  'err.JOKER_SWAP_MISMATCH': 'Ši korta neužima džokerio vietos',
  'err.RECLAIMED_JOKER_NOT_MELDED': 'Nuo stalo paimtas džokeris šį ėjimą turi būti sužaistas į derinį',
  'err.RUN_TOO_LONG': 'Ši seka jau yra viso ilgio',
  'err.WRONG_RUN_END': 'Ši korta pratęsia kitą sekos galą',
  'err.INVALID_MELD': 'Nė viena tavo rankos korta čia netinka',
  'err.CARD_NOT_IN_HAND': 'Šios kortos tavo rankoje nėra',
  'err.MELD_BELOW_MINIMUM': 'Tavo deriniams dar trūksta taškų išdėti',
  'err.MELD_NO_CONTRIBUTION': 'Šis derinys nepajudina tavo reikalavimo',
  'err.TOO_MANY_WILDS': 'Per daug džokerių šiame derinyje',
  'err.ADJACENT_WILDS': 'Du džokeriai negali gulėti greta',
  'err.ACE_BRIDGE': 'Tūzas negali sujungti karaliaus ir dvejeto',

  // --- contract phrasing ---------------------------------------------------
  // Built from the structured contract the server sends (sets/runs counts),
  // never from a deal-number lookup table.
  // Whole phrases per count rather than a number plus a pluralised noun:
  // Czech inflects the noun by count (skupina / skupiny / skupin) in a way no
  // "add an s" helper survives, and the counts that actually occur are 1-3.
  // A count outside that falls back to contract.sets.n.
  'contract.sets.1': 'Viena grupė',
  'contract.sets.2': 'Dvi grupės',
  'contract.sets.3': 'Trys grupės',
  'contract.sets.n': 'Grupės: {n}',
  'contract.runs.1': 'Viena seka',
  'contract.runs.2': 'Dvi sekos',
  'contract.runs.3': 'Trys sekos',
  'contract.runs.n': 'Sekos: {n}',
  'contract.any': 'Bet koks galiojantis derinys',
  'contract.cleanRunOnly': 'Bet koks grupių ir sekų derinys — bent viena seka turi būti be džokerio',
  'contract.cleanRunSuffix': '{base} — viena seka turi būti be džokerio',

  // --- written rules ---------------------------------------------------------
  // Full-sentence keys a "see the rules" screen renders, one per module,
  // resolved server-side against the table's actual variation and options —
  // see internal/module/rules.go and each game's rules.go. Shared section
  // titles first, then one block per module in the order the four games were
  // added.
  'zolik.rules.section.goal': 'Tikslas',
  'zolik.rules.section.setup': 'Pasiruošimas',
  'zolik.rules.section.turn': 'Tavo ėjimas',
  'zolik.rules.section.melding': 'Išdėjimas',
  'zolik.rules.section.end': 'Kaip baigiasi rungtynės',
  'zolik.rules.goal':
    'Būk pirmas, kuris ištuština ranką išdėdamas galiojančias grupes ir sekas, surinkdamas kuo mažiau baudos taškų kortose, kurias vis dar laikai, kai kas nors kitas išeina.',
  'zolik.rules.deal': 'Kiekvienas žaidėjas gauna {n} kortų.',
  'zolik.rules.meldShapes':
    'Grupė — tai {set}+ tos pačios vertės kortos; seka — tai {run}+ iš eilės einančios tos pačios rūšies kortos.',
  'zolik.rules.turn.draw': 'Savo ėjimo metu paimk vieną kortą — iš kaladės arba iš atmetimo krūvelės.',
  'zolik.rules.pickup.topOnly': 'Iš atmetimo krūvelės galima imti tik viršutinę kortą.',
  'zolik.rules.pickup.anyFromPile':
    'Iš atmetimo krūvelės galima imti bet kurią kortą kartu su viskuo, kas guli virš jos.',
  'zolik.rules.pickup.locked': 'Iš atmetimo krūvelės negalima traukti iki {n} raundo.',
  'zolik.rules.pickup.open': 'Atmetimo krūvelė atvira nuo pirmo raundo.',
  'zolik.rules.turn.discard': 'Užbaik ėjimą atmesdamas vieną kortą.',
  'zolik.rules.jokers.restricted':
    'Džokerio niekada negalima atmesti, nebent jis yra būtent ta korta, kuri ištuština tavo ranką.',
  'zolik.rules.lead.rotate':
    'Pirmas ėjimas kiekvieną dalijimą pasislenka per vieną vietą, nesvarbu, kas laimėjo.',
  'zolik.rules.lead.winner': 'Kas išeina, pradeda kitą dalijimą.',
  'zolik.rules.meldFloor.on': 'Tavo pirmas išdėjimas turi duoti bent {n} natūralių taškų, kad būtum išdėjęs.',
  'zolik.rules.meldFloor.off': 'Pirmam išdėjimui minimalios taškų vertės nėra.',
  'zolik.rules.cleanRun.on':
    'Bent viena tavo seka turi būti visiškai be džokerio, kad būtum laikomas išdėjusiu.',
  'zolik.rules.cleanRun.off': 'Tavo sekos gali laisvai naudoti džokerius — nė viena neprivalo būti be jų.',
  'zolik.rules.contracts.rotating':
    'Rungtynės trunka {n} dalijimų, ir kiekvienas dalijimas reikalauja savo grupių bei sekų derinio.',
  'zolik.rules.contracts.static':
    'Kiekvienas dalijimas reikalauja to paties derinio: {sets} grupių ir {runs} sekų.',
  'zolik.rules.end.afterDeals': 'Rungtynės baigiasi po {n} dalijimų.',
  'zolik.rules.end.atScore': 'Dalijama toliau, kol kas nors pasiekia {n} taškų — tada baigta.',

  'prsi.rules.section.goal': 'Tikslas',
  'prsi.rules.section.setup': 'Pasiruošimas',
  'prsi.rules.section.turn': 'Tavo ėjimas',
  'prsi.rules.section.special': 'Ypatingos kortos',
  'prsi.rules.section.end': 'Kaip baigiasi rungtynės',
  'prsi.rules.goal': 'Būk pirmas, kuris sužais visas rankos kortas.',
  'prsi.rules.deck': 'Žaidžiama {value} kortų kalade (nuo septyneto ir aukščiau).',
  'prsi.rules.deal': 'Kiekvienas žaidėjas pradeda su {n} kortomis.',
  'prsi.rules.turn.match':
    'Sužaisk kortą, atitinkančią viršutinės kortos rūšį arba vertę — arba traukk, jei negali.',
  'prsi.rules.turn.draw': 'Traukimas užbaigia tavo ėjimą nesužaidus.',
  'prsi.rules.sevens': 'Sužaisk 7 ir kitas žaidėjas traukia dvi kortas, nebent atsakys savo septynetu.',
  'prsi.rules.aces': 'Sužaisk tūzą ir kito žaidėjo ėjimas praleidžiamas.',
  'prsi.rules.queens': 'Sužaisk damą ir pasakyk rūšį, kuri tęsiasi.',
  'prsi.rules.end': 'Rungtynės baigiasi tą akimirką, kai kieno nors ranka tuščia.',

  'canasta.rules.section.goal': 'Tikslas',
  'canasta.rules.section.setup': 'Pasiruošimas',
  'canasta.rules.section.melding': 'Išdėjimas',
  'canasta.rules.section.end': 'Kaip baigiasi rungtynės',
  'canasta.rules.goal': 'Žaidžiama poromis; pirmoji pusė, pasiekusi {n} taškų, laimi rungtynes.',
  'canasta.rules.deck': 'Žaidžiama {value} kortomis — dvi kaladės ir džokeriai.',
  'canasta.rules.deal': 'Kiekvienas žaidėjas gauna {n} kortų.',
  'canasta.rules.drawCount': 'Ėjimo pradžioje imi {n} kortas.',
  'canasta.rules.redThrees':
    'Raudonas trejetas tavo rankoje parodomas iškart ir duoda premiją — nebent tavo pusė niekada nesudaro kanastos, tada jis skaičiuojamas prieš tave.',
  'canasta.rules.canasta': 'Kanasta — tai {n} ar daugiau tos pačios vertės kortų derinys.',
  'canasta.rules.sequences': 'Derinys gali būti ir seka: trys ar daugiau tos pačios rūšies kortų iš eilės, niekada su džokeriu tarp jų.',
  'canasta.rules.samba': 'Septynių kortų seka yra samba, verta {n} taškų.',
  'canasta.rules.pileAlwaysFrozen': 'Atmetimo krūvelė užšaldyta visą dalijimą: paimti ją gali tik pridėjęs prie viršutinės kortos dvi natūralias kortas iš rankos.',
  'canasta.rules.meldFloorBands':
    'Tavo pirmas išdėjimas turi pasiekti taškų minimumą, kuris auga kartu su tavo rezultatu: {negative} žemiau nulio, {low} iki 1500, {mid} iki 3000, {high} virš to.',
  'canasta.rules.meldFloorBandsFive': 'Tavo pirmasis derinys turi pasiekti taškų minimumą, kuris auga kartu su rezultatu: {negative} žemiau nulio, {low} iki 1500, {mid} iki 3000, {high} iki 7000 ir {top} virš to.',
  'canasta.rules.oneCanastaToGoOut': 'Vienos užbaigtos kanastos pakanka, kad tavo pusė išeitų.',
  'canasta.rules.twoCanastasToGoOut': 'Tavo pusei reikia dviejų užbaigtų kanastų, kad galėtų išeiti.',
  'canasta.rules.end': 'Dalijama toliau, kol viena pusė peržengia {n} taškų — tada rungtynės baigtos.',

  'holdem.rules.section.goal': 'Tikslas',
  'holdem.rules.section.setup': 'Pasiruošimas',
  'holdem.rules.section.betting': 'Statymai',
  'holdem.rules.section.end': 'Kaip baigiasi rungtynės',
  'holdem.rules.goal':
    'Laimėk žetonų turėdamas geriausią ranką atvertimo metu arba likdamas vienintelis žaidėjas dalijime.',
  'holdem.rules.stack': 'Kiekviena vieta pradeda su {n} žetonų.',
  'holdem.rules.blinds':
    'Mažasis aklasis statymas yra {sb}, didysis — {bb}; abu padedami prieš išdalijant kortas.',
  'holdem.rules.streets': 'Statoma keturiais ratais — prieš flopą bei po flopo, terno ir riverio.',
  'holdem.rules.showdown': 'Likusieji dalijime atverčia kortas; geriausia penkių kortų ranka pasiima banką.',
  'holdem.rules.noLimit': 'Be limito — bet kuris statymas gali siekti visą tavo krūvelę.',
  'holdem.rules.lastPlayerStanding': 'Žaidžiama, kol viena vieta turi visus žetonus.',
  'holdem.rules.mostChipsWins': 'Kas turi daugiausia žetonų, kai žaidimas sustoja, laimi rungtynes.',
  'holdem.rules.handLimit': 'Žaidimas sustoja po {n} dalijimų.',

  // --- header --------------------------------------------------------------
  'header.deal': 'Dalijimas {n}',
  'header.gameOf': 'Žaidimas {n} iš {total}',
  'header.gameOfWithContract': 'Žaidimas {n} iš {total}: {contract}',

  // --- meld preview --------------------------------------------------------
  'preview.validSet': 'Galiojanti grupė',
  'preview.validRun': 'Galiojanti seka',
  'preview.validMeld': 'Galiojantis derinys',
  'preview.notYet': 'Dar ne derinys',
  'preview.points': '{shape} · {n} taškų',
  'preview.pointsWithLaid': '{shape} · {n} + {laid} jau išdėta = {total} taškų',
  'preview.meetsFloor': '{line} (pasiekia {n} ✓)',
  'preview.needsFloor': '{line} (reikia {n} ✗)',
  'preview.becauseOf': '{line} — {reason}',

  // --- discard -------------------------------------------------------------
  // A discard with a meld staged lays that meld first; this is what the
  // player is told when the server refuses it and the whole move is rolled
  // back. {reason} is the engine's own words for the refusal.
  'discard.meldRejected': '{reason} — nieko nebuvo atmesta, tavo kortos vis dar paruoštos.',

  // --- what the current selection would send -------------------------------
  // The client's own reason a control is not ready, next to the engine's own
  // `err.*` reasons rather than a second kind of message — a control greyed
  // out for "not your turn" and one greyed out for "you picked two, this
  // takes one" should read as the same kind of thing, not one of them looking
  // broken. See `fits` in `src/lib/drops.ts`.
  'sel.tooMany.1': 'Pasirink tik vieną kortą',
  'sel.tooMany.n': 'Pasirink daugiausia {n} kortų',
  'sel.needMore': 'Pasirink kortų: {n}',
  'sel.notThese': 'Šios kortos čia negali patekti',
  'sel.needsCompany': 'Šiai kortai reikia gretimų',

  // --- what a module says happened -----------------------------------------
  // The first module-sent keys in this bundle, and deliberately few. Anything
  // a module sends is still legible without an entry here — `humanise` turns
  // `holdem.seat.stack` into "Stack" — so a key earns a line only when that
  // fallback loses something. These do: what each of them means lives in its
  // *params* (who won, how much, whose cards), and a key on its own has
  // nowhere to put them. Without the entry the player read "Winner", full
  // stop, at the end of a match they had just won.
  'status.winner': 'Laimėjo {winners}',
  'holdem.status.pot': '{winners} laimėjo {amount} su {hand}',
  'holdem.status.potUncontested': '{winners} laimėjo {amount} — visi kiti pasitraukė',
  'holdem.status.shown': '{playerId} parodė {value}',
  'holdem.prompt.waitingFor': 'Laukiama {playerId}',

  // How a rummy scoreboard was ordered. Both earn a line for the same reason
  // as the keys above: the number lives in the params, and `humanise` would
  // render "Deals Won" with nothing after it.
  'zolik.standing.dealsWon': 'Laimėta dalijimų: {n}',
  'zolik.standing.inHand': 'Rankoje: {n}',

  // --- rounds ---------------------------------------------------------------
  // What a round is called, and what one did. Each of these carries its number
  // in the params, so `humanise` alone would render a label with nothing after
  // it.
  // An instruction, not a heading. "Next round" named the thing rather than
  // asking for it, and sat above "Waiting for 1" — which reads as though the
  // player is the one waiting, when the table is waiting for them.
  'round.continue': 'Pradėti kitą raundą',

  // --- the flash that announces a round or a match ending ------------------
  //
  // One line and one number, held for about two seconds. Worded generically on
  // purpose: which *kind* of ending it was is the module's own fact, printed
  // underneath, and a client that phrased "went gin" itself would be a client
  // that knows a game.
  'flash.roundWon': '{winners} jį paėmė',
  'flash.roundWonYou': 'Tu jį paėmei',
  'flash.roundDrawn': 'Niekas jo nepaėmė',
  'flash.matchOver': 'Rungtynės baigtos',
  'flash.matchWon': '{winners} laimėjo',
  'flash.matchWonYou': 'Tu laimėjai',
  'flash.matchDrawn': 'Niekas nelaimėjo',
  'flash.nowOn': 'dabar {total}',

  'zolik.round.deal': 'Dalijimas',
  'zolik.round.cleanRun': 'Viena seka turi būti be džokerio',
  'canasta.round.deal': 'Dalijimas',
  'canasta.round.concealed': 'Išėjo slaptai',
  'canasta.round.exhausted': 'Kaladė baigėsi',
  'canasta.round.meldCards': 'Išdėtos kortos: {n}',
  'canasta.round.canastas': 'Kanastos: {n}',
  'canasta.round.redThrees': 'Raudoni trejetai: {n}',
  'canasta.round.goingOut': 'Išėjimas: {n}',
  'canasta.round.inHand': 'Liko rankoje: {n}',
  'holdem.round.hand': 'Ranka',
  'holdem.round.pot': 'Bankas {n}',
  'holdem.round.uncontested': 'Visi kiti pasitraukė',
  'seat.ready': 'Pasiruošęs',
  'zolik.seat.contractMet': 'Sutartis įvykdyta',
  'results.you': '(tu)',



  // --- refusals the other three games raise ---------------------------------
  // Worded here for the same reason Žolíky's are: reasonText falls back to the
  // code, and OfferBar hands it the code as that fallback, so an unworded
  // refusal reaches a player as PILE_FROZEN in capitals. serverKeys.test.ts
  // now fails the build rather than letting the next one through.
  'err.SET_TOO_LARGE': 'Grupė jau turi visas keturias rūšis',
  'err.DISCARD_TAKEN_CARD_FORBIDDEN': 'Negali atmesti kortos, kurią ką tik paėmei — sužaisk ją arba pasilik',
  'err.CARD_DOES_NOT_FIT': 'Ši korta nesutampa nei rūšimi, nei verte',
  'err.SUIT_REQUIRED': 'Pasakyk rūšį, kuri tęsiasi',
  'err.MUST_ANSWER_DRAW_OR_TAKE': 'Atsakyk septynetu arba pasiimk kortas',
  'err.NOTHING_TO_DRAW': 'Nebeliko ko traukti',
  'err.PILE_EMPTY': 'Krūvelė tuščia',
  'err.PILE_BLOCKED': 'Krūvelė užblokuota — viršuje guli juodas trejetas',
  'err.PILE_FROZEN': 'Krūvelė užšaldyta — tau reikia dviejų natūralių viršutinės kortos vertės kortų',
  'err.TOP_CARD_UNUSABLE': 'Viršutinės kortos panaudoti negali',
  'err.MELD_CLOSED': 'Šis derinys pilnas ir uždarytas',
  'err.MELD_TOO_SMALL': 'Deriniui reikia daugiau kortų nei tiek',
  'err.MELD_TOO_LARGE': 'Šis derinys daugiau kortų nebepriima',
  'err.MELD_MIXED_RANKS': 'Visos derinio kortos turi būti tos pačios vertės',
  'err.SEQUENCE_NO_WILDS': 'Sekoje negali būti džokerių',
  'err.SEQUENCE_NEEDS_ONE_SUIT': 'Visos sekos kortos turi būti tos pačios rūšies',
  'err.RUN_NOT_CONSECUTIVE': 'Seka turi eiti iš eilės, be tarpų',
  'err.NOT_ENOUGH_NATURALS': 'Deriniui reikia daugiau natūralių kortų nei džokerių',
  'err.RANK_ALREADY_MELDED': 'Tavo pusė jau turi tokios vertės derinį',
  'err.NOT_YOUR_MELD': 'Šis derinys priklauso priešingai pusei',
  'err.NO_SUCH_MELD': 'Šio derinio ant stalo nėra',
  'err.CANNOT_MELD_THREE': 'Trejetai niekada neišdedami',
  'err.CANNOT_DISCARD_RED_THREE': 'Raudono trejeto atmesti negalima',
  'err.MUST_KEEP_A_CARD': 'Pasilik bent vieną kortą — taip rankos neištuštinsi',
  'err.MUST_MELD_FIRST': 'Pirma išdėk savo pusės pradinį derinį',
  'err.INITIAL_MELD_NOT_MET': 'Tavo pirmam išdėjimui vis dar trūksta taškų',
  'err.CANNOT_GO_OUT_YET': 'Tavo pusei reikia užbaigtos kanastos, kad galėtų išeiti',
  'err.NOTHING_TO_CALL': 'Nėra statymo, kurį reikėtų atsakyti',
  'err.CANNOT_CHECK': 'Negali praleisti — yra statymas, į kurį reikia atsakyti',
  'err.CANNOT_RAISE': 'Čia kelti negali',
  'err.RAISE_TOO_SMALL': 'Kėlimas turi būti bent toks kaip ankstesnis',
  'err.NOT_ENOUGH_CHIPS': 'Tiek žetonų neturi',
  'err.AMOUNT_REQUIRED': 'Pasakyk kiek',
  'err.AMOUNT_NOT_A_NUMBER': 'Ši suma nėra skaičius',
  'err.SEAT_NOT_IN_HAND': 'Tu nedalyvauji šiame dalijime',
  'err.WRONG_RANK': 'Ši korta tam yra netinkamos vertės',
  'err.MATCH_FULL': 'Stalas pilnas',
  'err.MATCH_ALREADY_STARTED': 'Rungtynės jau prasidėjo',
  'err.TOO_FEW_PLAYERS': 'Žaidėjų dar nepakanka',
  'err.WRONG_PLAYER_COUNT': 'Šio žaidimo negalima žaisti su tiek žaidėjų',
  'err.NOT_THE_HOST': 'Tai gali padaryti tik šeimininkas',
  'err.NO_LONGER_WAITING': 'Stalas nebelaukia',
  'err.WAITING_ROOM_UNAVAILABLE': 'Laukiamasis nepasiekiamas',
  'err.SERVER_BUSY': 'Serveris šiuo metu pilnas — pabandyk po akimirkos',


  // --- rules the engine has always enforced and never stated ----------------
  // Every one of these was found by the guardrail rather than by review: the
  // validator could refuse a player for it, and no sentence anywhere said so.
  // See server/internal/zolikmod/ruleindex_test.go.
  'zolik.rules.section.layoff': 'Pridėjimas prie derinių',
  'zolik.rules.pickup.obligation':
    'Kol nesi išdėjęs, iš atmetimo krūvelės paimta korta turi būti panaudota tame derinyje, su kuriuo šį ėjimą išsidėstai.',
  'zolik.rules.pickup.noReturn':
    'Iš atmetimo krūvelės paimtos kortos to paties ėjimo metu atmesti negalima — sužaisk ją arba pasilik.',
  'zolik.rules.wilds.setLimit': 'Grupėje negali būti daugiau džokerių nei natūralių kortų.',
  'zolik.rules.set.maxSize':
    'Grupėje negali būti daugiau kaip {n} kortos — džokeris pakeičia trūkstamą rūšį, jis nepapildo jau pilnos grupės.',
  'zolik.rules.run.maxLength':
    'Sekoje negali būti daugiau kaip {n} kortų — tūzas apačioje, dvylika verčių virš jo ir tūzas viršuje.',
  'zolik.rules.run.aceBridge':
    'Tūzas stovi virš karaliaus arba po dvejeto, niekada kaip tiltas tarp abiejų sekos galų.',
  'zolik.rules.contracts.contribution':
    'Kol nesi išdėjęs, kiekvienas išdedamas derinys turi būti toks, kokio dalijimo sutartis dar reikalauja.',
  'zolik.rules.layoff.afterDown': 'Prie svetimų derinių pridėti negali, kol neišdėjai savo sutarties.',
  'zolik.rules.layoff.runEnds': 'Prie sekos pridėta korta turi ją tęsti viename arba kitame gale.',
  'zolik.rules.jokers.swap':
    'Ant stalo esančio derinio džokerį galima išpirkti būtent ta korta, kurią jis atstoja.',
  'zolik.rules.jokers.reclaim.on':
    'Nuo stalo išpirktas džokeris turi būti to paties ėjimo metu sužaistas į derinį — jis negali likti rankoje.',
  'zolik.rules.jokers.reclaim.off': 'Nuo stalo išpirktas džokeris gali likti rankoje.',
  'zolik.rules.deck.reshuffle':
    'Kai kaladė baigiasi, atmetimo krūvelė sumaišoma ir tampa nauja kalade; jei abi tuščios, dalijimas baigiasi.',


  // --- what to do instead ---------------------------------------------------
  // The third layer of a refusal, after the reason and the rule. Sent by the
  // module because it is the only side that knows which card is owed and
  // which way out is on offer; see server/internal/zolikmod/remedy.go.
  'zolik.remedy.meldThePickup': 'Pridėk {card} prie savo išdėjimo arba atšauk paėmimą.',
  'zolik.remedy.discardSomethingElse': 'Atmesk kitą kortą arba sužaisk {card} šį ėjimą.',
  'zolik.remedy.discardNotAJoker': 'Atmesk ką nors kita, ne džokerį.',
  'zolik.remedy.finishOrUndoLayDown': 'Užbaik išdėjimą arba jį atsiimk.',
  'zolik.remedy.needMorePoints': 'Tau reikia dar {n} taškų, kad galėtum išdėti.',
  'zolik.remedy.layACleanRun': 'Išdėk seką be džokerio joje.',
  'zolik.remedy.playReclaimedJoker': 'Sužaisk {card} į derinį arba atšauk paėmimą.',
  'zolik.remedy.goDownFirst': 'Pirma išdėk savo derinius.',
  'zolik.remedy.drawFirst': 'Pirma paimk kortą.',
  'zolik.remedy.drawFromStock': 'Traukk iš kaladės — atmetimo krūvelė atsidaro {n} raunde.',
  'zolik.remedy.drawFromStockEmpty': 'Verčiau traukk iš kaladės.',


  // --- keys whose meaning lives in their params -----------------------------
  // The shape fallback renders these with the number missing ("Stack", "Pot"),
  // which is why they earn a line where most keys do not.
  'header.contract': 'Reikia {sets} grupių ir {runs} sekų',
  'header.contract.cleanRunOnly': 'Reikia sekos be džokerio',
  'header.round': 'Raundas {n}',
  'header.deck': 'Kaladė',
  'header.target': 'Tikslas',
  'header.suitInPlay': 'Žaidžiama rūšis',
  'seat.cards': 'Kortos',
  'zolik.offer.meld': 'Išdėk',
  'prompt.pickupMustBeMelded':
    '{value} atkeliavo iš atmetimo krūvelės — ji turi patekti į derinius, su kuriais šį ėjimą išsidėstai.',
  'prompt.jokerMustBePlayed':
    '{value} atkeliavo nuo stalo — ji turi patekti į derinį, kad galėtum baigti ėjimą.',
  'prompt.initialMeld': 'Tavo pusės pradinis derinys turi pasiekti {n} taškų.',
  'prompt.canastasNeeded': 'Tavo pusei trūksta dar {n} kanastų, kad galėtų išeiti.',
  'prompt.mustDrawOrAnswerSeven': 'Atsakyk septynetu arba paimk {n} kortų.',
  'prompt.chooseSuit': 'Pasirink rūšį, kuri tęsiasi',
  'prompt.skipPending': 'Tavo ėjimas praleidžiamas',
  'status.lastDeal': 'Komanda {team} surinko {value}',
  'status.teamScore': 'Komanda {team}: {value}',
  'canasta.offer.rank': 'Vertė',
  'canasta.offer.sequence': 'Seka',
  'badge.naturalCanasta': 'Švari kanasta',
  'badge.mixedCanasta': 'Nešvari kanasta',
  'badge.samba': 'Samba',
  'badge.cleanRun': 'Švari seka',
  'canasta.seat.teamScore': 'Komandos taškai',
  'canasta.seat.canastas': 'Kanastos',
  'holdem.header.pot': 'Bankas',
  'holdem.header.street': 'Ratas',
  'holdem.header.hand': 'Ranka',
  'holdem.header.handLimit': 'Rankų iš viso',
  'holdem.header.blinds': 'Aklieji statymai',
  'holdem.cost.call': 'atsakyti',
  'holdem.cost.pot': 'banke',
  'holdem.seat.stack': 'Krūvelė',
  'holdem.seat.bet': 'Statymas',
  'holdem.prompt.yourAction': 'Tavo eilė veikti',
  'holdem.prompt.raiseTo': 'Kelti iki',
  'holdem.quick.halfPot': '½ Bankas',
  'holdem.quick.pot': 'Bankas',
  'holdem.quick.allIn': 'All-in',
  'zone.yourHand': 'Tavo ranka',
  'zone.opponentHand': 'Priešininko ranka',
  'zone.drawPile': 'Kaladė',
  'zone.discardPile': 'Atmetimo krūvelė',
  'zone.melds': 'Deriniai',
  'zone.teamMelds': 'Tavo pusės deriniai',
  'zone.opponentMelds': 'Priešininko pusės deriniai',
  'zone.redThrees': 'Raudoni trejetai',
  'zone.board': 'Stalas',
  'verb.drawFromDeck': 'Traukti',
  'verb.takeFromDiscard': 'Imti iš krūvelės',


  // --- the why sheet's own furniture ---------------------------------------
  // The three layers a refusal is explained in. Labels, not sentences: what
  // goes under each is sent by the server or worded above.
  'why.reason': 'Kodėl ne',
  'why.rule': 'Taisyklė',
  'why.rules': 'Taisyklės',
  'why.remedy': 'Ką gali padaryti',
  'why.readTheRules': 'Skaityti visas taisykles →',
  'why.close': 'Uždaryti',
  'why.open': 'kodėl',

  // --- marks on a particular card ------------------------------------------
  // A mark is a refusal that has not happened yet: the rule is enforced at the
  // discard, which is the last possible moment to hear about it, so the card
  // says so while there is still a turn left to act on it.
  'zolik.badge.owedToMeld':
    '{card} atkeliavo iš atmetimo krūvelės — ji turi patekti į derinius, su kuriais šį ėjimą išsidėstai.',
  'zolik.badge.jokerOwed': '{card} atkeliavo nuo stalo — ji turi patekti į derinį, kad galėtum baigti ėjimą.',

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
  'legal.terms': 'Sąlygos',
  // The document's own name, used as the heading above it. Kept apart from
  // `legal.notice.terms`, which is the same words inflected to sit inside a
  // sentence — a heading in the instrumental case reads as a mistake in every
  // Slavic language.
  'legal.terms.title': 'Naudojimo sąlygos',
  'legal.privacy.title': 'Privatumo pranešimas',
  'legal.privacy': 'Privatumas',
  'legal.source': 'Pirminis kodas',
  'legal.updated': 'Versija {version}',
  'legal.draft':
    'Projektas — dar negalioja. Operatoriaus pavadinimas, šalis ir kontaktinis adresas dar neužpildyti.',
  'legal.notice.before': 'Žaisdamas sutinki su ',
  'legal.notice.terms': 'naudojimo sąlygomis',
  'legal.notice.between': '. Kas apie tave saugoma, aprašyta ',
  'legal.notice.privacy': 'privatumo pranešime',
  'legal.notice.after': '.',
  // --- gin rummy -------------------------------------------------------------
  'err.UPCARD_DECLINED': 'Tu jau praleidai tą kortą',
  'err.DEADWOOD_TOO_HIGH': 'Tavo deadwood per didelis, kad galėtum belsti',
  'err.CARD_DOES_NOT_EXTEND_MELD': 'Ši korta šio derinio nepratęsia',
  'ginrummy.rules.setup': 'Pasiruošimas',
  'ginrummy.rules.turn': 'Tavo ėjimas',
  'ginrummy.rules.melds': 'Deriniai',
  'ginrummy.rules.knocking': 'Beldimas',
  'ginrummy.rules.bigGin': 'Big gin',
  'ginrummy.rules.layoff': 'Pridėjimas',
  'ginrummy.rules.deadHand': 'Negyvas dalijimas',
  'ginrummy.rules.scoring': 'Dalijimo taškai',
  'ginrummy.rules.match': 'Rungtynių laimėjimas',
  'ginrummy.rules.lineBonuses': 'Premijos suvestinėje',
  'ginrummy.rules.deck': 'Žaidžiama {value} kortų kalade.',
  'ginrummy.rules.deal': 'Kiekvienas žaidėjas gauna {value} kortų.',
  'ginrummy.rules.upcard': 'Dar viena korta atverčiama ir pradeda atmetimo krūvelę.',
  'ginrummy.rules.drawDiscard':
    'Savo ėjimo metu paimk vieną kortą — iš kaladės arba iš atmetimo krūvelės — ir tada vieną atmesk.',
  'ginrummy.rules.setsAndRuns':
    'Derinys — tai trijų ar keturių vienos vertės kortų grupė arba trijų ar daugiau vienos rūšies kortų seka.',
  'ginrummy.rules.aceLow': 'Tūzas visada žemas — sekos nuo damos iki tūzo nėra.',
  'ginrummy.rules.knockLimit': 'Belsti gali, kai tik tavo deadwood yra {n} arba mažiau.',
  'ginrummy.rules.oklahoma': 'Šio dalijimo beldimo ribą nustato atverstos kortos vertė.',
  'ginrummy.rules.gin': 'Nulinis deadwood — tai ginas, geriausias įmanomas beldimas.',
  'ginrummy.rules.bigGinBonus':
    'Vienuolika kortų, visos deriniuose, visai be atmetimo, yra big gin ir duoda dar {n} taškų.',
  'ginrummy.rules.layoffDescription':
    'Po beldimo, kuris nėra ginas, priešininkas gali pridėti savo deadwood prie tavo derinių, prieš lyginant rankas.',
  'ginrummy.rules.deadHandDescription':
    'Jei kalade lieka paskutinės dvi kortos ir niekas nepabeldė, dalijimas negyvas — niekas nerenka taškų ir tas pats dalytojas dalija iš naujo.',
  'ginrummy.rules.undercut':
    'Jei priešininko deadwood ne didesnis už tavąjį, jis tave pakerta: gauna skirtumą plius {n}.',
  'ginrummy.rules.ginBonus': 'Ginas duoda visą priešininko ranką plius {n}.',
  'ginrummy.rules.target': 'Kas dalijimui pasibaigus pirmas peržengia {n} taškų, laimi rungtynes.',
  'ginrummy.rules.shutout':
    'Rungtynių premija padvigubėja iki {n}, jei pralaimėjęs nesurinko nė vieno taško.',
  'ginrummy.rules.box': 'Kiekvienas laimėtas dalijimas rungtynių pabaigoje vertas {n} taškų.',
  'ginrummy.rules.gameBonus': 'Rungtynių laimėjimas duoda dar {n} taškų.',
  'ginrummy.fact.deadwood': 'deadwood {value}',
  'ginrummy.fact.discardCard': 'Atmesk {value}',
  'ginrummy.fact.meldCards': 'Prie {value}',
  'ginrummy.header.hand': 'Ranka {n}',
  'ginrummy.round.kind': '{value}',
  'ginrummy.round.hand': 'Ranka',
  'ginrummy.seat.score': '{value}',
  'ginrummy.seat.dealer': 'Dalytojas',
  'ginrummy.status.knocked': '{playerId} pabeldė su {deadwood} deadwood',
  'ginrummy.status.gin': '{playerId} padarė giną',
  'ginrummy.status.lastHand': 'Paskutinė ranka: {winner} ({kind}, {delta} taškų)',
  'ginrummy.offer.drawStock': 'Traukti iš kaladės',
  'ginrummy.offer.drawDiscard': 'Traukti iš atmetimo krūvelės',
  'ginrummy.offer.takeUpcard': 'Imti atverstą kortą',
  'ginrummy.offer.passUpcard': 'Praleisti',
  'ginrummy.offer.discard': 'Atmesti',
  'ginrummy.offer.knock': 'Belsti',
  'ginrummy.offer.gin': 'Ginas!',
  'ginrummy.offer.bigGin': 'Big gin!',
  'ginrummy.offer.layOff': 'Pridėti',
  'ginrummy.offer.finishLayoff': 'Pridėjimas baigtas',
  'ginrummy.zone.knockerHand': 'Pabeldusiojo ranka',
  'ginrummy.zone.melds': 'Deriniai',
  'ginrummy.prompt.upcardDecision': 'Imk atverstą kortą arba praleisk',
  'ginrummy.prompt.yourTurnDraw': 'Paimk kortą',
  'ginrummy.prompt.yourTurnDiscard': 'Atmesk — arba belsk, jei gali',
  'ginrummy.prompt.layoff': 'Pridėk deadwood arba baik',

  // --- rummy tiles -------------------------------------------------------------
  'err.TILE_NOT_IN_HAND': 'Šios plytelės tavo rankoje nėra',
  'err.TILE_DOES_NOT_FIT': 'Ten tai netinka',
  'err.NO_SUCH_SET': 'Šio derinio ant stalo nėra',
  'err.INITIAL_MELD_ONLY': 'Prieš pirmą išdėjimą gali pertvarkyti tik savo naujus derinius',
  'err.TABLE_NOT_VALID': 'Stalas dar negalioja',
  'err.TRAY_NOT_EMPTY': 'Tau dar liko laisvų plytelių padėti',
  'err.NOTHING_PLAYED': 'Sužaisk bent vieną plytelę, prieš baigdamas ėjimą',
  'err.INITIAL_MELD_TOO_LOW': 'Tavo pirmas išdėjimas turi būti vertas bent 30 taškų',
  'err.NOT_A_RUN': 'Perskirti galima tik seką',
  'err.BAD_SPLIT_POSITION': 'Toje vietoje šios sekos perskirti negalima',
  'err.NO_JOKER_IN_SET': 'Tame derinyje džokerio nėra',
  'err.TILE_JOKER_SWAP_MISMATCH': 'Ši plytelė nėra tai, ką atstoja džokeris',
  'rummytiles.rules.setup': 'Pasiruošimas',
  'rummytiles.rules.sets': 'Deriniai',
  'rummytiles.rules.initialMeld': 'Pirmas išdėjimas',
  'rummytiles.rules.turn': 'Tavo ėjimas',
  'rummytiles.rules.jokerTaking': 'Džokerio paėmimas',
  'rummytiles.rules.ending': 'Raundo pabaiga',
  'rummytiles.rules.poolExhaustion': 'Jei atsargos baigiasi',
  'rummytiles.rules.match': 'Rungtynių laimėjimas',
  'rummytiles.rules.tiles': 'Žaidžiama {value} plytelėmis.',
  'rummytiles.rules.dealCount': 'Kiekvienas žaidėjas gauna {value} plytelių.',
  'rummytiles.rules.group':
    'Grupė — tai trys ar keturios to paties skaičiaus plytelės, kiekviena kitos spalvos.',
  'rummytiles.rules.run': 'Seka — tai trys ar daugiau iš eilės einančių skaičių vienos spalvos.',
  'rummytiles.rules.noWrap': 'Po 13 vėl neprasideda 1.',
  'rummytiles.rules.joker': 'Džokeris atstoja bet kurią plytelę.',
  'rummytiles.rules.initialMeldDescription':
    'Kol vienu ėjimu, vien iš savo rankos, neišdėjai {n} ar daugiau taškų, negali liesti nieko, kas jau yra ant stalo.',
  'rummytiles.rules.turnDescription':
    'Sužaisk bent vieną plytelę iš rankos, laisvai pertvarkyk stalą ir baik taip, kad kiekvienas derinys ant stalo galiotų.',
  'rummytiles.rules.noDiscard':
    'Atmetimo nėra — jei negali užbaigti galiojančio ėjimo, vietoj to traukk vieną plytelę.',
  'rummytiles.rules.jokerTakingDescription':
    'Ant stalo esantį džokerį gali paimti pakeisdamas jį plytele, kurią jis atstoja, iš savo rankos — ir turi jį panaudoti derinyje, kol tavo ėjimas nesibaigė.',
  'rummytiles.rules.goingOut':
    'Raundą laimi pirmas žaidėjas, kuriam baigiasi plytelės. Visi kiti gauna neigiamą to, kas jiems liko, vertę; laimėtojas gauna visų kitų praradimų sumą.',
  'rummytiles.rules.poolExhaustionLowestWins':
    'Jei atsargos baigiasi ir niekas negali žaisti, raundas baigiasi ir jį laimi mažiausia rankos vertė.',
  'rummytiles.rules.poolExhaustionNoWinner':
    'Jei atsargos baigiasi ir niekas negali žaisti, raundas baigiasi be laimėtojo — kiekviena ranka tiesiog suskaičiuojama.',
  'rummytiles.rules.target': 'Kas raundui pasibaigus pirmas peržengia {n} taškų, laimi rungtynes.',
  'rummytiles.rules.roundLimit': 'Rungtynės baigiasi po {n} raundų — laimi aukščiausias rezultatas.',
  'rummytiles.fact.setCards': '{value}',
  'rummytiles.header.pool': 'Atsargos {n}',
  'rummytiles.header.round': 'Raundas {n}',
  'rummytiles.round.kind': '{value}',
  'rummytiles.round.round': 'Raundas',
  'rummytiles.seat.score': '{value}',
  'rummytiles.seat.notOpened': 'Neatidarė',
  'rummytiles.status.lastRound': 'Paskutinis raundas: {winner} ({kind})',
  'rummytiles.badge.invalid': 'Dar negalioja',
  'rummytiles.zone.pool': 'Atsargos',
  'rummytiles.zone.table': 'Stalas',
  'rummytiles.zone.tray': 'Stovas',
  'rummytiles.offer.place': 'Padėti',
  'rummytiles.offer.addFromHand': 'Pridėti',
  'rummytiles.offer.addFromTray': 'Pridėti nuo stovo',
  'rummytiles.offer.take': 'Imti',
  'rummytiles.offer.split': 'Perskirti',
  'rummytiles.offer.swapJoker': 'Pakeisti džokerį',
  'rummytiles.offer.resetTurn': 'Atstatyti ėjimą',
  'rummytiles.offer.commit': 'Baigta',
  'rummytiles.offer.draw': 'Traukti',
  'rummytiles.param.position': 'Perskirti ties',

  // --- blackjack -----------------------------------------------------------
  //
  // The house rules are worded twice over — on and off — because a rule that
  // is *not* in force is still something a player has to be told. "No
  // surrender at this table" is information; silence is a guess.
  'err.BET_BELOW_MINIMUM': 'Tai mažiau nei stalo minimumas',
  'err.ALREADY_BET': 'Tavo statymas jau padėtas',
  'err.INSURANCE_CLOSED': 'Šiuo metu draudimo imti negalima',
  'err.CANNOT_DOUBLE': 'Šios rankos padvigubinti negalima',
  'err.CANNOT_SPLIT': 'Šios rankos perskirti negalima',
  'err.CANNOT_SURRENDER': 'Šios rankos atiduoti negalima',

  'blackjack.rules.section.table': 'Stalas',
  'blackjack.rules.section.play': 'Rankos žaidimas',
  'blackjack.rules.section.dealer': 'Dalytojas',
  'blackjack.rules.section.end': 'Kaip baigiasi rungtynės',
  'blackjack.rules.goal':
    'Įveik dalytoją neperžengdamas dvidešimt vieno. Peržengęs pralaimi iškart, kad ir ką dalytojas darytų po to.',
  // Counts are their own phrase rather than a number glued to a noun: one
  // deck and six decks inflect differently, and Czech inflects them again.
  'blackjack.rules.decks': 'Kaladžių bate: {n}.',
  'blackjack.rules.stack': 'Kiekviena vieta sėdasi su {n} žetonų.',
  'blackjack.rules.minBet': 'Stalo minimumas — {n} žetonų.',
  'blackjack.rules.faceUp':
    'Žaidėjų kortos dalijamos atverstos; dalytojas vieną kortą laiko uždengtą, kol visi sužais.',
  'blackjack.rules.hitStand': 'Traukk tiek kortų, kiek nori, arba likk prie to, ką turi.',
  'blackjack.rules.aces': 'Tūzas skaičiuojamas kaip vienuolika, kol tai telpa, ir kaip vienas, kai netelpa.',
  'blackjack.rules.blackjack': 'Tūzas su dešimties vertės korta pirmose dviejose kortose yra blackjack.',
  'blackjack.rules.pays3to2': 'Blackjack moka 3:2.',
  'blackjack.rules.pays6to5': 'Blackjack moka 6:5.',
  'blackjack.rules.paysEven': 'Blackjack moka vienas prieš vieną.',
  'blackjack.rules.double':
    'Pirmose dviejose kortose gali padvigubinti statymą ir paimti lygiai vieną papildomą kortą.',
  'blackjack.rules.doubleAfterSplit': 'Ir iš perskyrimo atsiradusią ranką galima padvigubinti.',
  'blackjack.rules.noDoubleAfterSplit': 'Iš perskyrimo atsiradusios rankos padvigubinti negalima.',
  'blackjack.rules.split':
    'Dvi vienodos vertės kortos gali būti perskirtos į atskiras rankas, kiekviena su savo statymu — iki {n} kartų, iš viso {hands} rankoms.',
  'blackjack.rules.noSplit': 'Prie šio stalo poros neperskiriamos.',
  'blackjack.rules.splitAces':
    'Perskirti tūzai gauna po vieną kortą ir tada lieka, o taip pasiektas dvidešimt vienas nėra blackjack.',
  'blackjack.rules.surrender':
    'Pirmos rankos gali atsisakyti už pusę statymo, kai dalytojas patikrina blackjack.',
  'blackjack.rules.noSurrender': 'Prie šio stalo rankų atiduoti negalima.',
  'blackjack.rules.dealerDraws': 'Dalytojas traukia iki septyniolikos ir tada lieka.',
  'blackjack.rules.hitsSoft17': 'Dalytojas traukia ir prie septyniolikos, sudarytos su tūzu.',
  'blackjack.rules.standsSoft17': 'Dalytojas lieka prie septyniolikos, sudarytos su tūzu.',
  'blackjack.rules.dealerPeeks':
    'Rodydamas tūzą ar dešimtakę, dalytojas patikrina blackjack, prieš kam nors sužaidžiant.',
  'blackjack.rules.insurance':
    'Prieš dalytojo tūzą gali apsidrausti už pusę statymo; tai moka 2:1, jei dalytojas turi blackjack.',
  'blackjack.rules.noInsurance': 'Prie šio stalo draudimas nesiūlomas.',
  'blackjack.rules.rounds': 'Prie stalo žaidžiama {n} raundų.',
  'blackjack.rules.mostChipsWins': 'Kas pabaigoje turi daugiausia žetonų, laimi rungtynes.',
  'blackjack.rules.bustedOut':
    'Vieta, kuri nebepajėgia padengti {n} minimumo, likusias rungtynes praleidžia.',

  'blackjack.zone.dealer': 'Dalytojas',
  'blackjack.zone.box': 'Ranka',
  'blackjack.zone.yourBox': 'Tavo ranka',
  'blackjack.zone.shoe': 'Batas',

  'blackjack.header.round': 'Raundas {n} iš {of}',
  'blackjack.header.minBet': 'Minimumas',
  'blackjack.header.decks': 'Kaladės',
  'blackjack.header.dealerTotal': 'Dalytojas rodo {n}',
  'blackjack.header.dealerSoftTotal': 'Dalytojas rodo minkštą {n}',

  'blackjack.seat.stack': 'Žetonai',
  'blackjack.seat.bet': 'Statymas',
  'blackjack.seat.insurance': 'Draudimas',
  'blackjack.seat.total': 'Iš viso',
  'blackjack.seat.softTotal': 'Minkšta suma',
  'blackjack.seat.out': 'Žetonai baigėsi',

  'blackjack.prompt.placeBet': 'Padėk savo statymą',
  'blackjack.prompt.insurance': 'Draudimas?',
  'blackjack.prompt.yourMove': 'Tavo ėjimas',
  'blackjack.prompt.waitingFor': 'Laukiama {playerId}',
  'blackjack.prompt.betAmount': 'Statymas',

  'blackjack.quick.doubleMin': '2× Minimumas',
  'blackjack.quick.allIn': 'All-in',
  'blackjack.offer.bet': 'Statyti',
  'blackjack.offer.hit': 'Korta',
  'blackjack.offer.stand': 'Lieku',
  'blackjack.offer.double': 'Padvigubinti',
  'blackjack.offer.split': 'Perskirti',
  'blackjack.offer.surrender': 'Atiduoti',
  'blackjack.offer.insure': 'Apsidrausti',
  'blackjack.offer.declineInsurance': 'Be draudimo',

  'blackjack.fact.tableMinimum': 'minimumas',
  'blackjack.fact.insuranceCost': 'draudimui',
  'blackjack.fact.extraStake': 'statyti',
  'blackjack.fact.surrenderReturn': 'atgal',

  'blackjack.status.dealerBlackjack': 'Dalytojas turėjo blackjack',
  'blackjack.status.dealerBust': 'Dalytojas persistatė su {n}',
  'blackjack.status.dealerStands': 'Dalytojas lieka prie {n}',

  'blackjack.round.name': 'Raundas',
  'blackjack.round.dealerTotal': 'Dalytojas {n}',
  'blackjack.round.dealerBust': 'Dalytojas persistatė ({n})',
  'blackjack.round.dealerBlackjack': 'Dalytojo blackjack',
  'blackjack.round.outcome.blackjack': 'Blackjack',
  'blackjack.round.outcome.win': 'Laimėta',
  'blackjack.round.outcome.push': 'Lygiosios',
  'blackjack.round.outcome.lose': 'Pralaimėta',
  'blackjack.round.outcome.bust': 'Persistatė',
  'blackjack.round.outcome.surrender': 'Atiduota',

  'blackjack.badge.inPlay': 'Žaidime',
  'blackjack.badge.doubled': 'Padvigubinta',
  'blackjack.badge.split': 'Perskirta',
  'blackjack.badge.blackjack': 'Blackjack',
  'blackjack.badge.bust': 'Persistatė',
  'blackjack.badge.won': 'Laimėta',
  'blackjack.badge.push': 'Lygiosios',
  'blackjack.badge.lost': 'Pralaimėta',
  'blackjack.badge.surrendered': 'Atiduota',

  'blackjack.unit.chips': 'žetonų',

  // --- the settings screen --------------------------------------------------
  //
  // Hardcoded English until now, which was survivable while the only other
  // language was unreachable and faintly absurd once the language picker
  // itself lived on this screen: a player who cannot read "Settings" is
  // exactly the one who came here to change it.
  'settings.title': 'Nustatymai',
  'settings.signedInAs': 'Prisijungęs kaip {username}',
  'settings.playingAsGuest': 'Žaidi kaip {username} (svečias)',
  'settings.notSignedIn': 'Neprisijungęs — prisijunk arba tęsk kaip svečias, kad žaistum internetu.',
  'settings.subtitle': 'Kaip atrodai tu ir kaip atrodo stalas',
  'settings.face.heading': 'Tavo veidas prie stalo',
  'settings.face.account': 'Saugoma paskyroje, todėl keliauja su tavimi į kitą įrenginį.',
  'settings.face.device': 'Saugoma šiame įrenginyje. Prisijunk, kad pasiimtum su savimi.',
  'settings.skin.heading': 'Stalo išvaizda',
  'settings.language.heading': 'Kalba',
  'settings.language.status': 'Saugoma šiame įrenginyje.',
  'settings.language.auto': 'Automatiškai',
  'settings.language.auto.now': 'Seka tavo įrenginį — dabar {language}',
  'settings.legal.heading': 'Smulkusis šriftas',
  'settings.legal.status': 'Su kuo sutikai žaisdamas ir kas apie tave saugoma.',
  'settings.signIn': 'Prisijungti',
  'settings.back': 'Atgal',

  // A translated interface over an English notice is a worse lie than an
  // English interface, so the document says so itself rather than letting the
  // surrounding screen imply the text was written for this reader.
  'legal.untranslated':
    'Šis pranešimas dar neišverstas į tavo kalbą. Galioja žemiau pateiktas tekstas anglų kalba.',

  // --- the navigation bar's screen titles ------------------------------------
  //
  // These sit in `app/_layout.tsx`, above every screen, and were the last
  // English left after the settings screen was translated — a German player
  // reading "Einstellungen" under a bar that said "Settings". A translated app
  // with an untranslated chrome bar looks like a bug, not like a limit.
  'nav.home': 'Zolik',
  'nav.emailSignIn': 'Prisijungimas el. paštu',
  'nav.signingIn': 'Jungiamasi',
  'nav.usernameSignIn': 'Prisijungimas naudotojo vardu',
  'nav.legacyAccount': 'Sena paskyra',
  'nav.guest': 'Svečias',
  'nav.account': 'Paskyra',
  'nav.games': 'Žaidimai',
  'nav.table': 'Tavo stalas',
  'nav.join': 'Prisijungti prie stalo',
  // The screen a shared invite link lands on while it resolves the code.
  'nav.joining': 'Jungiamasi prie stalo',
  'nav.rules': 'Taisyklės',
  'nav.match': 'Rungtynės',
  'nav.scoreTable': 'Taškų lentelė',
  'nav.stats': 'Statistika',
  'nav.more': 'Daugiau',

  // --- the account menu behind the face in the corner ----------------------
  'menu.label': 'Paskyros meniu',
  'menu.signedIn': 'Prisijungęs',
  'menu.notSignedIn': 'Neprisijungęs',
  'menu.keepStats': 'kad išsaugotum statistiką',
  'menu.signOut': 'Atsijungti',
  'more.scoreTable': 'Nešėtinė taškų lentelė',
  'more.stats': 'Statistika ir lyderių lentelė',
  'more.needsAccount': 'prisijunk, kad naudotum',
  'gate.title': 'Prisijunk, kad tai naudotum',
  'gate.body':
    'Taškų lentelės ir statistika saugomos su tavo paskyra, tad keliauja su tavimi į kitą įrenginį. Svečias neturi kur jų laikyti.',

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
  'error.generic': 'Tai nepavyko',
  'error.signIn': 'Prisijungti nepavyko',
  'error.login': 'Prisijungti nepavyko',
  'error.register': 'Registruotis nepavyko',
  'error.sendCode': 'Nepavyko išsiųsti kodo',
  'error.badCode': 'Tas kodas neveikė',
  'error.rulesLoad': 'Nepavyko įkelti taisyklių',
  'error.createFailed': 'Sukurti nepavyko',
  'error.saveFailed': 'Įrašyti nepavyko',
  'error.exportFailed': 'Eksportuoti nepavyko',

  // --- the route that does not exist ----------------------------------------
  'notFound.title': 'Ups!',
  'notFound.message': 'Tokio ekrano nėra.',
  'notFound.home': 'Eiti į pradinį ekraną!',

  // --- signing in -----------------------------------------------------------
  'auth.login.subtitle': 'Išsaugok statistiką visuose įrenginiuose',
  'auth.login.continueWithEmail': 'Tęsti su el. paštu',
  'auth.login.usernameInstead': 'Prisijungti vietoj to naudotojo vardu',
  'auth.email.title': 'Prisijungimas el. paštu',
  'auth.email.subtitle': 'Atsiųsime tau vienkartinį kodą',
  'auth.email.address': 'El. pašto adresas',
  'auth.email.send': 'Siųsti kodą',
  'auth.email.codeTitle': 'Įvesk kodą',
  'auth.email.codePlaceholder': 'Šešių skaitmenų kodas',
  'auth.email.differentAddress': 'Naudoti kitą adresą',
  'auth.email.sentTo': 'Išsiųsta adresu {email}',
  'auth.email.continue': 'Tęsti',
  'auth.guest.title': 'Žaidimas kaip svečias',
  'auth.guest.subtitle': 'Paskyros nereikia',
  'auth.guest.displayName': 'Rodomas vardas',
  'auth.register.title': 'Sukurti paskyrą',
  'auth.register.username': 'Naudotojo vardas',
  'auth.register.email': 'El. paštas (nebūtina)',
  'auth.register.password': 'Slaptažodis',
  'auth.username.createAccount': 'Sukurti paskyrą su naudotojo vardu ir slaptažodžiu',
  'auth.callback.signedIn': 'Prisijungta.',

  // --- the account screen ---------------------------------------------------
  'account.signInPrompt': 'Prisijunk, kad galėtum tvarkyti paskyrą.',
  'account.keepGames': 'Išsaugoti šiuos žaidimus',
  'account.signedInWith': 'Prisijungta per',
  'account.addMethod': 'Pridėti prisijungimo būdą',
  'account.usernameAndPassword': 'Naudotojo vardas ir slaptažodis',
  'account.faceAndTable': 'Veidas ir stalo išvaizda',
  'account.refresh': 'Atnaujinti',
  'account.remove': 'Pašalinti',

  // --- the main menu --------------------------------------------------------
  'home.subtitle': 'Kontinentinis remis · {server}',
  'home.playingAs': 'Žaidi kaip {name}',
  'home.signInPrompt': 'Prisijunk arba tęsk kaip svečias, kad žaistum internete.',
  'home.statsAndLeaderboard': 'Statistika ir lentelė',
  'home.play': 'Žaisti',
  'home.offlineScoreTable': 'Taškų lentelė neprisijungus',
  'home.signInToKeepStats': 'Prisijunk, kad išsaugotum statistiką',
  'home.signOut': 'Atsijungti',
  'home.continueAsGuest': 'Tęsti kaip svečias',
  // Someone playing as a guest, marked in a list of names.
  'home.guestSuffix': '(svečias)',

  // --- the waiting room on the main menu ------------------------------------
  //
  // Counts are whole phrases per count, not a number glued to a noun — the
  // same reason `countLabel` exists. One and many are the cases the interface
  // actually produces, so they are the cases that get their own wording.
  'waiting.checking': 'Žiūrime, kas netoliese…',
  'waiting.youAreWaiting': 'Lauki, kol galėsi žaisti',
  'waiting.pickedUp': 'Bet kas, kas atveria stalą, gali tave paimti — niekam nereikia iš tavęs kodo.',
  'waiting.othersOne': 'Laukia dar 1 žaidėjas',
  'waiting.othersMany': 'Laukia dar {n} žaidėjų',
  'waiting.oneWaiting': '1 žaidėjas laukia, kol galės žaisti',
  'waiting.manyWaiting': '{n} žaidėjų laukia, kol galės žaisti',
  'waiting.adding': 'Įtraukiame tave į laukiančiųjų sąrašą…',
  'waiting.slowHint':
    'Jei tai nesibaigs per kelias sekundes, patikrink, ar žemiau nurodytas serverio adresas pasiekiamas iš šio įrenginio.',
  'waiting.serverBusyDetail': 'Bandymas {n}. Serveris šiuo metu nepriima naujų prisijungimų prie laukiamojo.',
  'waiting.reconnecting': 'Ryšys nutrūko — jungiamės iš naujo…',
  'waiting.reconnectingDetail':
    'Bandymas {n}. Taip gali nutikti, jei pasikeitė tavo įrenginio tinklas arba serveris buvo paleistas iš naujo.',
  'waiting.tryAgain': 'Bandyti dabar iš naujo',
  'waiting.makeAvailable': 'Pažymėti mane kaip pasiruošusį žaisti',
  'waiting.stop': 'Nustoti laukti',
  'waiting.noneYet':
    'Šiuo metu niekas nelaukia žaidimo. Įsirašyk į sąrašą ir būsi pirmas, kurį bet kas pamatys.',
  'waiting.noOthersYet': 'Daugiau niekas dar nelaukia. Šeimininkai tave vis tiek mato ir gali pakviesti.',
  'waiting.server': 'Serveris',
  'waiting.none':
    'Šiuo metu niekas nelaukia. Kas pagrindiniame meniu pažymi save kaip pasiruošusį, pasirodo čia.',

  // --- landing on a shared invite link --------------------------------------
  'join.missingCode': 'Šiai nuorodai trūksta stalo kodo.',
  'join.staleLink': 'Paprašyk pakvietusiojo naujos nuorodos arba prisijunk su kodu.',
  'join.enterCode': 'Įvesti kodą',
  'join.backToMenu': 'Atgal į meniu',
  'join.takingSeat': 'Užimame vietą…',
  'join.takingSeatAt': 'Užimame vietą prie {game}…',

  // --- the lobby ------------------------------------------------------------
  'lobby.games.subtitle': 'Viskas, ką šis serveris gali pasiūlyti',
  'lobby.games.bots': 'Botai',
  'lobby.games.playBot': 'Žaisti prieš botą',
  'lobby.games.playBots': 'Žaisti prieš {n} botus',
  'lobby.games.openTable': 'Atverti stalą',
  'lobby.games.players': 'Žaidėjų: {n}',
  'lobby.games.playerRange': 'Žaidėjų: {min}–{max}',
  'lobby.join.placeholder': 'Prisijungimo kodas arba kvietimo nuoroda',
  'lobby.join.needCode': 'Įvesk kodą, nuorodą arba rungtynių ID',
  'lobby.games.signInFirst': 'Pirma prisijunk',
  'lobby.join.action': 'Prisijungti',
  'lobby.join.waitingTitle': 'Laukiame šeimininko',
  // Two whole sentences rather than one with a swapped noun: "a game of {game}"
  // does not survive a language that inflects the game's name after "of".
  'lobby.join.joinedGame': 'Prisijungei prie žaidimo {game} — laukiame starto',
  'lobby.join.joinedTable': 'Prisijungei prie stalo — laukiame starto',
  'lobby.table.addBot': 'Pridėti botą',
  'lobby.table.start': 'Pradėti',
  'lobby.table.waitingForHost': 'Laukiame, kol šeimininkas pradės…',

  // --- inviting someone to a table ------------------------------------------
  'invite.heading': 'Pakviesti žaidėjų',
  'invite.explain': 'Nusiųsk šią nuorodą. Kas ją atvers, pateks prie šio stalo — paskyros nereikia.',
  'invite.noAddress': 'Šiam serveriui nenustatytas bendrinamas adresas, tad naudok kodą žemiau.',
  'invite.readOutCode': 'Arba padiktuok kodą:',
  'invite.copy': 'Kopijuoti nuorodą',
  'invite.share': 'Dalytis nuoroda',
  'invite.copied': 'Nukopijuota!',
  'invite.shared': 'Pasidalyta',

  // --- the match screen -----------------------------------------------------
  'match.waitingForTable': 'Laukiame stalo…',
  'match.waitingForPlayer': 'Laukiame kito žaidėjo…',
  'match.nobodyWon': 'Niekas nelaimėjo.',
  'match.youWon': 'Tu laimėjai.',
  'match.finished': 'Šios rungtynės baigėsi.',
  'match.inProgress': 'Rungtynės vyksta — viskas prijungta ir juda įprastai.',
  'match.connecting': 'Jungiamasi…',
  'match.abandonedTitle': 'Stalas padėtas į šalį',
  'match.abandoned': 'Prie šio stalo niekas negrįžo, todėl jis buvo padėtas į šalį. Kortos yra būtent ten, kur jas palikai.',
  'match.resume': 'Tęsk ten, kur baigei',
  'match.resuming': 'Atkuriamas stalas…',
  'match.controls': 'Valdikliai',
  'match.over': 'Rungtynių pabaiga',
  'match.settingUp': 'Ruošiame…',
  'match.playAgain': 'Žaisti dar kartą',
  'match.backToGames': 'Atgal į žaidimus',
  'match.table': 'Stalas',
  'match.opponents': 'Priešininkai',
  // Marks which seat is the reader's own, in a list of seats.
  'match.youSuffix': '(tu)',
  // The winner line is composed from whole sentences rather than a name glued
  // to " won." — the verb agrees with the subject in most of these languages,
  // and a suffix cannot know that.
  'match.you': 'tu',
  'match.someoneWon': '{name} laimėjo.',
  'match.wonBy': 'Laimėjo {names}.',
  'match.pausedFor': 'Sustabdyta — laukiame, kol {name} vėl prisijungs.',
  'match.results': 'Rezultatai',
  'match.players': 'Žaidėjai',
  'match.toPlay': 'eina',

  // --- the offline score table ----------------------------------------------
  'scoring.namesHint': 'Vardai, atskirti kableliais (4–8 žaidėjai)',
  'scoring.newSession': 'Nauja sesija',
  // A worked example, not a sentence: the names are placeholders a translator
  // may localise, the shape `name:score,` is what the parser needs.
  'scoring.scoresPlaceholder': 'Aistė:120,Benas:80,…',
  'scoring.saveRound': 'Įrašyti raundą',
  'scoring.export': 'Eksportuoti taškų lapą',
  'scoring.formatHint': 'Taškų formatas: Vardas:100,Vardas2:50',
  'scoring.nameCountError': 'Įvesk 2–8 žaidėjų vardus, atskirtus kableliais',
  'scoring.session': 'Sesija: {id}',
  'scoring.players': 'Žaidėjai: {names}',
  'scoring.roundScores': '{n} raundo taškai',
  'stats.loading': 'Įkeliama…',
  // The figures could not be fetched. `{reason}` is whatever the server or the
  // network said, which is not ours to translate — the frame around it is.
  'stats.unavailable': '(nepasiekiama: {reason})',

  // --- statistics -----------------------------------------------------------
  'stats.title': 'Statistika ir lentelė',
  'stats.yours': 'Tavo statistika',
  'stats.leaderboard': 'Lentelė',

  // --- a player's lifetime record, shown beside a finished match ------------
  'record.title': 'Tavo rezultatai',
  'record.guest':
    'Žaidi kaip svečias, tad rezultatai nekaupiami. Prisijunk ir žaidimai, kuriuos šiame įrenginyje jau sužaidei — įskaitant šį — liks prie tavo paskyros.',
  'record.signInToKeep': 'Prisijungti ir juos išsaugoti',
  'record.failed': 'Tavo rezultatų dabar nepavyko įkelti. Rungtynės saugiai užfiksuotos.',
  'record.loading': 'Įkeliama…',
  'record.played': 'Sužaista',
  'record.won': 'Laimėta',
  'record.lost': 'Pralaimėta',
  'record.winRate': 'Pergalių dalis',
  'record.streak': 'Serija',
  'record.atThisGame': 'Šiame žaidime',
  // A streak is a count with a noun, so it is a whole phrase per count for
  // the same reason `countLabel` is: "1 win" / "2 wins" is an English rule,
  // and most of these languages do not share it.
  'record.streakNone': '—',
  'record.streakWinOne': '1 pergalė',
  'record.streakWinMany': 'Pergalės: {n}',
  'record.streakLossOne': '1 pralaimėjimas',
  'record.streakLossMany': 'Pralaimėjimai: {n}',

  // --- moving cards ---------------------------------------------------------
  'hand.dragHint': 'Tempk kortą palei vėduoklę, kad ją perstumtum, arba ant stalo, kad ją sužaistum',
  'hand.moveLeft': 'Kairėn',
  'hand.moveRight': 'Dešinėn',
  'zone.collapseGroup': 'Sutraukti šią grupę',
  'zone.expandGroup': 'Rodyti visas šios grupės kortas',
  'zone.dropHere': 'Padėk čia',
  'offer.pickCards': 'pasirink kortas vietai, kurią palietei',
  'offer.ambiguous': 'tai tinka daugiau nei vienoje vietoje — pasirink ant stalo',

  // --- the build footer -----------------------------------------------------
  'build.app': 'programa',
  'build.server': 'serveris',
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
  'option.pauseBetweenRounds': 'Pertrauka tarp raundų',
  'choice.pauseBetweenRounds.1': 'Pertrauka',
  'choice.pauseBetweenRounds.0': 'Tęsti iš karto',
  'option.botSkill': 'Priešininkai',
  'choice.botSkill.0': 'Mišrūs',
  'choice.botSkill.1': 'Lengvi',
  'choice.botSkill.2': 'Vidutiniai',
  'choice.botSkill.3': 'Sunkūs',
  'option.initialMeldMinimum': 'Atvėrimo vertė',
  'choice.initialMeldMinimum.0': 'Nėra',
  'option.discardDrawMinRound': 'Ėmimas iš atmetimo krūvelės',
  'choice.discardDrawMinRound.0': 'Atvira',
  'choice.discardDrawMinRound.2': 'Nuo 2 raundo',
  'choice.discardDrawMinRound.3': 'Nuo 3 raundo',
  'option.requireCleanRun': 'Seka be džokerio',
  'choice.requireCleanRun.1': 'Privaloma',
  'choice.requireCleanRun.0': 'Ne',
  'option.jokerReclaimMustPlay': 'Išpirktas džokeris',
  'choice.jokerReclaimMustPlay.1': 'Sužaisti tą patį ėjimą',
  'choice.jokerReclaimMustPlay.0': 'Galima pasilikti',
  'option.dealStarter': 'Kas pradeda',
  'choice.dealStarter.0': 'Paeiliui',
  'choice.dealStarter.1': 'Pradeda laimėtojas',
  'variation.prsi.classic': 'Klasikinis',
  'option.handSize': 'Išdalytos kortos',
  'variation.canasta.classic': 'Klasikinė',
  'variation.canasta.modern_american': 'Modern American',
  'variation.canasta.samba': 'Samba',
  'option.targetScore': 'Tikslinis rezultatas',
  'option.canastasToGoOut': 'Kanastos išėjimui',
  'variation.holdem.freezeout': 'Freezeout',
  'variation.holdem.timed': 'Fiksuotas dalijimų skaičius',
  'option.startingStack': 'Pradiniai žetonai',
  'option.bigBlind': 'Didysis aklasis',
  'option.handLimit': 'Dalijimai',
  'choice.handLimit.0': 'Kol liks viena vieta',
  'variation.ginrummy.standard': 'Standartinis',
  'option.knockLimit': 'Beldimo riba',
  'choice.knockLimit.0': 'Oklahoma (ją nustato atversta korta)',
  'option.bigGin': 'Big gin',
  'choice.bigGin.0': 'Išjungta',
  'choice.bigGin.1': 'Įjungta (+25)',
  'option.lineBonuses': 'Premijos suvestinėje',
  'choice.lineBonuses.1': 'Įjungtos',
  'choice.lineBonuses.0': 'Išjungtos',
  'variation.rummytiles.standard': 'Standartinis',
  'choice.targetScore.0': 'Nėra',
  // Two games qualify a number the others mean plainly: Canasta's 500 is a
  // short game, Gin Rummy's and Rummy Tiles' 500 is just 500; Hold'em's
  // starting 200 is a short stack, Blackjack's 200 is just 200. Module-scoped
  // so the qualifier travels with the game that means it — a bare key would
  // hand the note to games it is false for. `scripts/check-server-labels.js`
  // is what finds these; both were shipping as English on a Czech screen.
  'choice.canasta.targetScore.500': '500 (trumpa)',
  'choice.holdem.startingStack.200': '200 (trumpa)',
  'option.roundLimit': 'Raundų riba',
  'choice.roundLimit.0': 'Nėra',
  'option.poolExhaustion': 'Jei atsargos baigiasi',
  'choice.poolExhaustion.1': 'Raundą laimi žemiausia ranka',
  'choice.poolExhaustion.0': 'Raundo nelaimi niekas',
  'variation.blackjack.single': 'Viena kaladė',
  'option.minBet': 'Stalo minimumas',
  'option.rounds': 'Raundai',
  'option.decks': 'Kaladės',
  'option.dealerHitsSoft17': 'Dalytojas prie minkštos 17',
  'choice.dealerHitsSoft17.0': 'Lieka',
  'choice.dealerHitsSoft17.1': 'Traukia',
  'option.blackjackPays': 'Blackjack moka',
  'choice.blackjackPays.100': 'Vienas prieš vieną',
  'option.maxSplits': 'Perskyrimas',
  'choice.maxSplits.0': 'Be perskyrimo',
  'choice.maxSplits.1': 'Kartą (dvi rankos)',
  'choice.maxSplits.3': 'Tris kartus (keturios rankos)',
  'option.doubleAfterSplit': 'Padvigubinimas po perskyrimo',
  'choice.doubleAfterSplit.1': 'Leidžiama',
  'choice.doubleAfterSplit.0': 'Neleidžiama',
  'option.surrender': 'Atidavimas',
  'choice.surrender.0': 'Išjungta',
  'choice.surrender.1': 'Vėlyvas atidavimas',
  'option.insurance': 'Draudimas',
  'choice.insurance.1': 'Siūlomas',
  'choice.insurance.0': 'Nesiūlomas',

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
  'verb.add': 'Pridėti',
  'verb.bet': 'Statyti',
  'verb.call': 'Atsakau',
  'verb.check': 'Tikrinu',
  'verb.commit': 'Baigta',
  'verb.continue': 'Tęsti',
  'verb.decline_insurance': 'Be draudimo',
  'verb.discard': 'Atmesti',
  'verb.double': 'Padvigubinti',
  'verb.draw': 'Traukti',
  'verb.finish_layoff': 'Pridėjimas baigtas',
  'verb.fold': 'Pasitraukiu',
  'verb.hit': 'Korta',
  'verb.insure': 'Apsidrausti',
  'verb.knock': 'Belsti',
  'verb.lay_meld': 'Išdėk',
  'verb.lay_off': 'Pridėti',
  'verb.pass': 'Praleisti',
  'verb.place': 'Padėti',
  'verb.play_card': 'Sužaisk',
  'verb.raise': 'Keliu',
  'verb.reset_turn': 'Atstatyti ėjimą',
  'verb.split': 'Perskirti',
  'verb.stand': 'Lieku',
  'verb.surrender': 'Atiduoti',
  'verb.swap_joker': 'Pakeisti džokerį',
  'verb.take': 'Imti',
  'verb.take_pile': 'Imti iš krūvelės',
  // Declared by the server and listed in `serverKeys.json`, but never worded.
  'verb.takePileFromHand': 'Imk krūvelę į ranką',
  'verb.takePileOntoMeld': 'Imk krūvelę į derinį',
  'verb.takeTopForSequence': 'Imk viršutinę kortą į seką',
  'verb.undoDraw': 'Atšaukti traukimą',
  'verb.undoLayOff': 'Atšaukti pridėjimą',
  'verb.undoMeld': 'Atšaukti derinį',
  'verb.undoTakePile': 'Atšaukti ėmimą iš krūvelės',
  'verb.undoTurn': 'Atšaukti ėjimą',

  // --- suits, spelled out ---------------------------------------------------
  //
  // Read aloud by a screen reader and shown where a pip would not fit. The
  // pips themselves are drawn, not written, so these are the only place the
  // suit is ever a word.
  'suit.C': 'Kryžiai',
  'suit.D': 'Būgnai',
  'suit.H': 'Širdys',
  'suit.S': 'Vynai',

  // --- counters the board prints beside a number ----------------------------
  'canasta.seat.notOpened': 'Neatidarė',
  'canasta.unit.points': 'taškų',
  'ginrummy.unit.points': 'taškų',
  'holdem.seat.dealer': 'Dalytojas',
  'holdem.seat.folded': 'Pasitraukė',
  'holdem.seat.allIn': 'All in',
  'holdem.seat.out': 'Iškritęs',
  'holdem.unit.chips': 'žetonų',
  'prsi.unit.cardsLeft': 'liko kortų',
  'rummytiles.prompt.initialMeld': 'Tavo pirmas išdėjimas turi būti vertas {n} taškų.',
  'rummytiles.unit.points': 'taškų',
  'zolik.unit.penalty': 'bauda',
  'header.pileFrozen': 'Krūvelė užšaldyta',
  // The module-less form of `ginrummy.prompt.yourTurnDraw`: Žolíky and Canasta
  // send the prompt without a module prefix. Neither the Go constants nor
  // `serverKeys.json` list it — the sweep is what found it.
  'prompt.yourTurnDraw': 'Paimk kortą',
  'prompt.yourTurnMeld': 'Išdėk, jei gali, tada išmesk',
};
