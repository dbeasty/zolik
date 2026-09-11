/**
 * Finnish. Rummy vocabulary: ryhmä for a set, suora for a run, yhdistelmä for a meld, nostopakka and poistopino for the two piles.
 */

export const fi: Record<string, string> = {
  // --- engine error codes (rules.RulesErrorCode) ---------------------------
  'err.NOT_YOUR_TURN': 'Ei ole sinun vuorosi',
  'err.WRONG_PHASE': 'Ei onnistu juuri nyt',
  'err.MUST_DRAW_FIRST': 'Nosta kortti ennen kuin lasket',
  'err.GAME_SUSPENDED': 'Peli on tauolla',
  'err.GAME_NOT_ACTIVE': 'Peli ei ole käynnissä',
  // --- runtime refusals, which are not the rules saying no -----------------
  //
  // Both were reachable long before they were worded, and a player who met
  // one was shown the code in capitals or, worse, nothing at all: an action
  // sent while the socket was down simply vanished, which reads as a legal
  // move being silently refused.
  'err.MATCH_NOT_ACTIVE': 'Pöytä on tauolla — odotetaan pelaajan palaavan',
  'err.NOT_CONNECTED': 'Ei yhteyttä pöytään — yhdistetään uudelleen, yritä sitten uudestaan',
  // Why the "start the next round" control is greyed out. Without wording the
  // shell printed the code itself, so a player agreeing to go on was answered
  // with ALREADY_READY in capitals.
  'err.ALREADY_READY': 'Olet valmis',
  'err.NOT_BETWEEN_ROUNDS': 'Kierros on vielä kesken',
  'err.NOT_AT_THIS_TABLE': 'Et ole tässä pöydässä',
  // Bringing a table back that the sweeper set aside. Refusals a player
  // can actually provoke: a link to a table that is gone, one that was
  // already picked up in another tab, and a table with other people at it.
  'err.MATCH_MOVED_ON': 'Pöytä on edennyt — lataa sivu uudelleen',
  'err.MATCH_NOT_ABANDONED': 'Tämä pöytä ei odota jatkamista',
  'err.MATCH_NOT_FOUND': 'Tätä pöytää ei ole enää',
  'err.TABLE_HAS_OTHER_PLAYERS': 'Vain pöydän, jossa kaikki muut ovat botteja, voi palauttaa',
  'err.DISCARD_LOCKED': 'Poistopino on toistaiseksi lukittu',
  'err.DISCARD_PILE_EMPTY': 'Poistopino on tyhjä',
  'err.NO_CARDS_LEFT': 'Nostettavia kortteja ei ole jäljellä',
  'err.ROUND_REQ_NOT_MET': 'Laske ensin oma avauksesi',
  'err.NEED_CLEAN_RUN': 'Tarvitset pöytään jokerittoman suoran, jotta sinut lasketaan alas laskeneeksi',
  'err.INCOMPLETE_INITIAL_MELD': 'Viimeistele laskusi tai peru se ennen kuin poistat kortin',
  'err.DISCARD_CARD_NOT_MELDED': 'Nostamasi kortin on mentävä yhdistelmääsi',
  'err.JOKER_DISCARD_FORBIDDEN': 'Jokeria ei voi poistaa',
  'err.NOTHING_TO_UNDO': 'Ei ole mitään peruttavaa',
  'err.NO_JOKER_IN_MELD': 'Tässä yhdistelmässä ei ole jokeria',
  'err.JOKER_SWAP_MISMATCH': 'Tuo kortti ei ota jokerin paikkaa',
  'err.RECLAIMED_JOKER_NOT_MELDED': 'Pöydästä otettu jokeri on pelattava yhdistelmään tällä vuorolla',
  'err.RUN_TOO_LONG': 'Tuo suora on jo täysimittainen',
  'err.WRONG_RUN_END': 'Tuo kortti jatkaa suoran toista päätä',
  'err.INVALID_MELD': 'Yksikään kädessäsi oleva kortti ei sovi tähän',
  'err.CARD_NOT_IN_HAND': 'Tuota korttia ei ole kädessäsi',
  'err.MELD_BELOW_MINIMUM': 'Yhdistelmiltäsi puuttuu vielä pisteitä laskemiseen',
  'err.MELD_NO_CONTRIBUTION': 'Tuo yhdistelmä ei edistä vaatimustasi',
  'err.TOO_MANY_WILDS': 'Liikaa jokereita tuossa yhdistelmässä',
  'err.ADJACENT_WILDS': 'Kaksi jokeria ei voi olla vierekkäin',
  'err.ACE_BRIDGE': 'Ässä ei voi yhdistää kuningasta ja kakkosta',

  // --- contract phrasing ---------------------------------------------------
  // Built from the structured contract the server sends (sets/runs counts),
  // never from a deal-number lookup table.
  // Whole phrases per count rather than a number plus a pluralised noun:
  // Czech inflects the noun by count (skupina / skupiny / skupin) in a way no
  // "add an s" helper survives, and the counts that actually occur are 1-3.
  // A count outside that falls back to contract.sets.n.
  'contract.sets.1': 'Yksi ryhmä',
  'contract.sets.2': 'Kaksi ryhmää',
  'contract.sets.3': 'Kolme ryhmää',
  'contract.sets.n': '{n} ryhmää',
  'contract.runs.1': 'Yksi suora',
  'contract.runs.2': 'Kaksi suoraa',
  'contract.runs.3': 'Kolme suoraa',
  'contract.runs.n': '{n} suoraa',
  'contract.any': 'Mikä tahansa kelvollinen yhdistelmä',
  'contract.cleanRunOnly':
    'Mikä tahansa ryhmien ja suorien sekoitus — vähintään yhden suoran on oltava jokeriton',
  'contract.cleanRunSuffix': '{base} — yhden suoran on oltava jokeriton',

  // --- written rules ---------------------------------------------------------
  // Full-sentence keys a "see the rules" screen renders, one per module,
  // resolved server-side against the table's actual variation and options —
  // see internal/module/rules.go and each game's rules.go. Shared section
  // titles first, then one block per module in the order the four games were
  // added.
  'zolik.rules.section.goal': 'Tavoite',
  'zolik.rules.section.setup': 'Valmistelu',
  'zolik.rules.section.turn': 'Sinun vuorosi',
  'zolik.rules.section.melding': 'Laskeminen',
  'zolik.rules.section.end': 'Miten ottelu päättyy',
  'zolik.rules.goal':
    'Tyhjennä kätesi ensimmäisenä laskemalla kelvollisia ryhmiä ja suoria, ja kerää mahdollisimman vähän miinuspisteitä niihin kortteihin, jotka sinulla on vielä kädessä, kun joku muu pääsee ulos.',
  'zolik.rules.deal': 'Jokainen pelaaja saa {n} korttia.',
  'zolik.rules.meldShapes':
    'Ryhmä on {set}+ samanarvoista korttia; suora on {run}+ peräkkäistä samanmaista korttia.',
  'zolik.rules.turn.draw': 'Nosta vuorollasi yksi kortti — nostopakasta tai poistopinosta.',
  'zolik.rules.pickup.topOnly': 'Poistopinosta saa ottaa vain päällimmäisen kortin.',
  'zolik.rules.pickup.anyFromPile':
    'Poistopinosta saa ottaa minkä tahansa kortin ja kaiken sen päällä olevan.',
  'zolik.rules.pickup.locked': 'Poistopinosta ei saa nostaa ennen kierrosta {n}.',
  'zolik.rules.pickup.open': 'Poistopino on auki ensimmäisestä kierroksesta alkaen.',
  'zolik.rules.turn.discard': 'Päätä vuorosi poistamalla yksi kortti.',
  'zolik.rules.jokers.restricted':
    'Jokeria ei saa koskaan poistaa, paitsi jos se on täsmälleen se kortti, joka tyhjentää kätesi.',
  'zolik.rules.lead.rotate': 'Aloitusvuoro siirtyy yhden paikan joka jaossa riippumatta siitä, kuka voitti.',
  'zolik.rules.lead.winner': 'Ulos päässyt aloittaa seuraavan jaon.',
  'zolik.rules.meldFloor.on':
    'Ensimmäisen laskusi on oltava vähintään {n} luonnollista pistettä, ennen kuin olet laskenut alas.',
  'zolik.rules.meldFloor.off': 'Ensimmäiselle laskulle ei ole pisteminimiä.',
  'zolik.rules.cleanRun.on':
    'Vähintään yhden suorasi on oltava täysin jokeriton, ennen kuin sinut lasketaan alas laskeneeksi.',
  'zolik.rules.cleanRun.off':
    'Suorasi saavat käyttää jokereita vapaasti — yhdenkään ei tarvitse olla jokeriton.',
  'zolik.rules.contracts.rotating':
    'Ottelu kestää {n} jakoa, ja jokainen jako vaatii oman ryhmien ja suorien yhdistelmänsä.',
  'zolik.rules.contracts.static': 'Jokainen jako vaatii saman yhdistelmän: {sets} ryhmää ja {runs} suoraa.',
  'zolik.rules.end.afterDeals': 'Ottelu päättyy {n} jaon jälkeen.',
  'zolik.rules.end.atScore': 'Jakoa jatketaan, kunnes joku saavuttaa {n} pistettä — sitten se on ohi.',

  'prsi.rules.section.goal': 'Tavoite',
  'prsi.rules.section.setup': 'Valmistelu',
  'prsi.rules.section.turn': 'Sinun vuorosi',
  'prsi.rules.section.special': 'Erikoiskortit',
  'prsi.rules.section.end': 'Miten ottelu päättyy',
  'prsi.rules.goal': 'Pelaa ensimmäisenä kaikki korttisi kädestä.',
  'prsi.rules.deck': 'Pelataan {value} kortin pakalla (seiskasta ylöspäin).',
  'prsi.rules.deal': 'Jokainen pelaaja aloittaa {n} kortilla.',
  'prsi.rules.turn.match':
    'Pelaa kortti, joka vastaa päällimmäisen kortin maata tai arvoa — tai nosta, jos et voi.',
  'prsi.rules.turn.draw': 'Nostaminen päättää vuorosi ilman pelaamista.',
  'prsi.rules.sevens':
    'Pelaa 7, niin seuraava pelaaja nostaa kaksi korttia, ellei hän vastaa omalla seiskallaan.',
  'prsi.rules.aces': 'Pelaa ässä, niin seuraavan pelaajan vuoro ohitetaan.',
  'prsi.rules.queens': 'Pelaa rouva ja nimeä maa, joka jatkuu.',
  'prsi.rules.end': 'Ottelu päättyy sillä hetkellä, kun jonkun käsi on tyhjä.',

  'canasta.rules.section.goal': 'Tavoite',
  'canasta.rules.section.setup': 'Valmistelu',
  'canasta.rules.section.melding': 'Laskeminen',
  'canasta.rules.section.end': 'Miten ottelu päättyy',
  'canasta.rules.goal': 'Pelataan pareittain; ensimmäisenä {n} pisteeseen yltävä puoli voittaa ottelun.',
  'canasta.rules.deck': 'Pelataan {value} kortilla — {decks} pakkaa ja jokerit.',
  'canasta.rules.deal': 'Jokainen pelaaja saa {n} korttia.',
  'canasta.rules.drawCount': 'Nostat {n} korttia vuorosi alussa.',
  'canasta.rules.redThrees':
    'Punainen kolmonen kädessäsi näytetään heti ja se antaa bonuksen — paitsi jos puolesi ei koskaan saa canastaa valmiiksi, jolloin se lasketaan sinua vastaan.',
  'canasta.rules.canasta': 'Canasta on yhdistelmä, jossa on {n} tai useampi samanarvoinen kortti.',
  'canasta.rules.sequences': 'Yhdistelmä voi olla myös jono: kolme tai useampi saman maan kortti peräkkäin, ei koskaan jokeria mukana.',
  'canasta.rules.samba': 'Seitsemän kortin jono on samba ja se on {n} pisteen arvoinen.',
  'canasta.rules.blackThreesGoOut':
    'Musta kolmonen tukkii pinon ja on {n} pisteen arvoinen. Kolme tai neljä niistä saa laskea suoraan kädestä, ei koskaan jokerin kanssa, ja vain siirtona, jolla puolesi pääsee ulos.',
  'canasta.rules.blackThreesNeverMeld':
    'Mustaa kolmosta ei lasketa koskaan. Poistettuna se tukkii pinon, ja jaon lopussa käteen jäänyt maksaa {n} pistettä.',
  'canasta.rules.pileAlwaysFrozen': 'Poistopino on jäädytetty koko jaon ajan: saat sen vain sovittamalla päällimmäisen kortin kahteen luonnolliseen korttiin kädestäsi.',
  'canasta.rules.pileOntoMeld': 'Jos puolellasi on jo pöydässä keskeneräinen yhdistelmä päällimmäisen kortin arvoa, saat ottaa koko poistopinon ja lisätä sen kortin siihen — paria kädessä ei tarvita.',
  'canasta.rules.pileNoMeldCapture': 'Pöydässä jo oleva yhdistelmä ei voi ottaa poistopinoa: sen ottamiseen tarvitset päällimmäisen kortin pariksi kaksi korttia omasta kädestäsi.',
  'canasta.rules.meldFloorBands':
    'Ensimmäisen laskusi on yllettävä pisterajaan, joka nousee pistetilanteesi mukana: {negative} alle nollan, {low} 1500:aan asti, {mid} 3000:een asti, {high} sen yli.',
  'canasta.rules.meldFloorBandsFive': 'Ensimmäisen yhdistelmäsi on yllettävä pisterajaan, joka nousee pistetilanteesi mukana: {negative} alle nollan, {low} 1500 asti, {mid} 3000 asti, {high} 7000 asti ja {top} sen yli.',
  'canasta.rules.oneCanastaToGoOut': 'Yksi valmis canasta riittää, jotta puolesi pääsee ulos.',
  'canasta.rules.twoCanastasToGoOut': 'Puolesi tarvitsee kaksi valmista canastaa ennen kuin se pääsee ulos.',
  'canasta.rules.end': 'Jakoa jatketaan, kunnes toinen puoli ylittää {n} pistettä — sitten ottelu on ohi.',

  'holdem.rules.section.goal': 'Tavoite',
  'holdem.rules.section.setup': 'Valmistelu',
  'holdem.rules.section.betting': 'Panostus',
  'holdem.rules.section.end': 'Miten ottelu päättyy',
  'holdem.rules.goal':
    'Voita pelimerkkejä parhaalla kädellä lopunäytössä tai jäämällä ainoaksi pelaajaksi jakoon.',
  'holdem.rules.stack': 'Jokainen paikka aloittaa {n} pelimerkillä.',
  'holdem.rules.blinds': 'Pieni blindi on {sb} ja iso blindi {bb}, ja ne asetetaan ennen korttien jakoa.',
  'holdem.rules.streets':
    'Panostetaan neljällä kierroksella — ennen floppia sekä flopin, turnin ja riverin jälkeen.',
  'holdem.rules.showdown': 'Vielä mukana olevat näyttävät korttinsa; paras viiden kortin käsi vie potin.',
  'holdem.rules.noLimit': 'No limit — mikä tahansa panos saa olla aina koko pinosi verran.',
  'holdem.rules.lastPlayerStanding': 'Pelataan, kunnes yksi paikka omistaa kaikki pelimerkit.',
  'holdem.rules.mostChipsWins': 'Se, jolla on eniten pelimerkkejä pelin päättyessä, voittaa ottelun.',
  'holdem.rules.handLimit': 'Peli päättyy {n} jaon jälkeen.',

  // --- header --------------------------------------------------------------
  'header.deal': 'Jako {n}',
  'header.gameOf': 'Peli {n}/{total}',
  'header.gameOfWithContract': 'Peli {n}/{total}: {contract}',

  // --- meld preview --------------------------------------------------------
  'preview.validSet': 'Kelvollinen ryhmä',
  'preview.validRun': 'Kelvollinen suora',
  'preview.validMeld': 'Kelvollinen yhdistelmä',
  'preview.notYet': 'Ei vielä yhdistelmä',
  'preview.points': '{shape} · {n} pistettä',
  'preview.pointsWithLaid': '{shape} · {n} + {laid} jo laskettu = {total} pistettä',
  'preview.meetsFloor': '{line} (yltää {n} ✓)',
  'preview.needsFloor': '{line} (vaatii {n} ✗)',
  'preview.becauseOf': '{line} — {reason}',

  // --- discard -------------------------------------------------------------
  // A discard with a meld staged lays that meld first; this is what the
  // player is told when the server refuses it and the whole move is rolled
  // back. {reason} is the engine's own words for the refusal.
  'discard.meldRejected': '{reason} — mitään ei poistettu, korttisi ovat yhä valmiina.',

  // --- what the current selection would send -------------------------------
  // The client's own reason a control is not ready, next to the engine's own
  // `err.*` reasons rather than a second kind of message — a control greyed
  // out for "not your turn" and one greyed out for "you picked two, this
  // takes one" should read as the same kind of thing, not one of them looking
  // broken. See `fits` in `src/lib/drops.ts`.
  'sel.tooMany.1': 'Valitse vain yksi kortti',
  'sel.tooMany.n': 'Valitse enintään {n} korttia',
  'sel.needMore': 'Valitse {n} korttia',
  'sel.notThese': 'Nuo kortit eivät voi mennä tähän',
  'sel.needsCompany': 'Tuo kortti tarvitsee viereisensä',

  // --- what a module says happened -----------------------------------------
  // The first module-sent keys in this bundle, and deliberately few. Anything
  // a module sends is still legible without an entry here — `humanise` turns
  // `holdem.seat.stack` into "Stack" — so a key earns a line only when that
  // fallback loses something. These do: what each of them means lives in its
  // *params* (who won, how much, whose cards), and a key on its own has
  // nowhere to put them. Without the entry the player read "Winner", full
  // stop, at the end of a match they had just won.
  'status.winner': 'Voitti {winners}',
  'holdem.status.pot': '{winners} voitti {amount} kädellä {hand}',
  'holdem.status.potUncontested': '{winners} voitti {amount} — kaikki muut luovuttivat',
  'holdem.status.shown': '{playerId} näytti {value}',
  'holdem.prompt.waitingFor': 'Odotetaan pelaajaa {playerId}',

  // How a rummy scoreboard was ordered. Both earn a line for the same reason
  // as the keys above: the number lives in the params, and `humanise` would
  // render "Deals Won" with nothing after it.
  'zolik.standing.dealsWon': 'Voitetut jaot {n}',
  'zolik.standing.inHand': 'Kädessä {n}',

  // --- rounds ---------------------------------------------------------------
  // What a round is called, and what one did. Each of these carries its number
  // in the params, so `humanise` alone would render a label with nothing after
  // it.
  // An instruction, not a heading. "Next round" named the thing rather than
  // asking for it, and sat above "Waiting for 1" — which reads as though the
  // player is the one waiting, when the table is waiting for them.
  'round.continue': 'Aloita seuraava kierros',

  // --- the flash that announces a round or a match ending ------------------
  //
  // One line and one number, held for about two seconds. Worded generically on
  // purpose: which *kind* of ending it was is the module's own fact, printed
  // underneath, and a client that phrased "went gin" itself would be a client
  // that knows a game.
  'flash.roundWon': '{winners} vei sen',
  'flash.roundWonYou': 'Sinä veit sen',
  'flash.roundDrawn': 'Kukaan ei vienyt sitä',
  'flash.matchOver': 'Ottelu päättyi',
  'flash.matchWon': '{winners} voitti',
  'flash.matchWonYou': 'Voitit',
  'flash.matchDrawn': 'Kukaan ei voittanut',
  'flash.nowOn': 'nyt {total}',

  'zolik.round.deal': 'Jako',
  'zolik.round.cleanRun': 'Yhden suoran on oltava jokeriton',
  'canasta.round.deal': 'Jako',
  'canasta.round.concealed': 'Pääsi ulos piilossa',
  'canasta.round.exhausted': 'Pakka loppui',
  'canasta.round.meldCards': 'Lasketut kortit {n}',
  'canasta.round.canastas': 'Canastat {n}',
  'canasta.round.redThrees': 'Punaiset kolmoset {n}',
  'canasta.round.goingOut': 'Ulospääsy {n}',
  'canasta.round.inHand': 'Jäi käteen {n}',
  'holdem.round.hand': 'Käsi',
  'holdem.round.pot': 'Potti {n}',
  'holdem.round.uncontested': 'Kaikki muut luovuttivat',
  'seat.ready': 'Valmis',
  'zolik.seat.contractMet': 'Sopimus täytetty',
  'results.you': '(sinä)',



  // --- refusals the other three games raise ---------------------------------
  // Worded here for the same reason Žolíky's are: reasonText falls back to the
  // code, and OfferBar hands it the code as that fallback, so an unworded
  // refusal reaches a player as PILE_FROZEN in capitals. serverKeys.test.ts
  // now fails the build rather than letting the next one through.
  'err.SET_TOO_LARGE': 'Ryhmässä on jo kaikki neljä maata',
  'err.DISCARD_TAKEN_CARD_FORBIDDEN': 'Et voi poistaa juuri ottamaasi korttia — pelaa se tai pidä se',
  'err.CARD_DOES_NOT_FIT': 'Tuo kortti ei vastaa maata eikä arvoa',
  'err.SUIT_REQUIRED': 'Nimeä maa, joka jatkuu',
  'err.MUST_ANSWER_DRAW_OR_TAKE': 'Vastaa seiskalla tai ota kortit',
  'err.NOTHING_TO_DRAW': 'Nostettavaa ei ole enää jäljellä',
  'err.PILE_EMPTY': 'Pino on tyhjä',
  'err.PILE_BLOCKED': 'Pino on tukossa — päällimmäisenä on musta kolmonen',
  'err.PILE_FROZEN': 'Pino on jäädytetty — tarvitset kaksi luonnollista korttia päällimmäisen kortin arvosta',
  'err.MELD_CAPTURE_NOT_ALLOWED': 'Tässä pelissä pöydän yhdistelmä ei voi ottaa pinoa — tarvitset kaksi korttia kädestä',
  'err.TOP_CARD_UNUSABLE': 'Et voi käyttää päällimmäistä korttia',
  'err.MELD_CLOSED': 'Tuo yhdistelmä on täysi ja suljettu',
  'err.MELD_TOO_SMALL': 'Yhdistelmä vaatii enemmän kortteja',
  'err.MELD_TOO_LARGE': 'Tuohon yhdistelmään ei mahdu enää kortteja',
  'err.MELD_MIXED_RANKS': 'Yhdistelmän jokaisen kortin on oltava samanarvoinen',
  'err.SEQUENCE_NO_WILDS': 'Jonossa ei saa olla jokereita',
  'err.SEQUENCE_NEEDS_ONE_SUIT': 'Jonon kaikkien korttien on oltava samaa maata',
  'err.RUN_NOT_CONSECUTIVE': 'Jonon on kuljettava järjestyksessä ilman aukkoja',
  'err.NOT_ENOUGH_NATURALS': 'Yhdistelmä vaatii enemmän luonnollisia kortteja kuin jokereita',
  'err.RANK_ALREADY_MELDED': 'Puolellasi on jo tämän arvoinen yhdistelmä',
  'err.NOT_YOUR_MELD': 'Tuo yhdistelmä kuuluu vastapuolelle',
  'err.NO_SUCH_MELD': 'Tuota yhdistelmää ei ole pöydässä',
  'err.CANNOT_MELD_THREE': 'Kolmosia ei koskaan lasketa',
  'err.BLACK_THREE_GO_OUT_ONLY': 'Mustat kolmoset lasketaan vain siirtona, joka tyhjentää kätesi',
  'err.CANNOT_DISCARD_RED_THREE': 'Punaista kolmosta ei voi poistaa',
  'err.MUST_KEEP_A_CARD': 'Pidä vähintään yksi kortti — näin et voi tyhjentää kättäsi',
  'err.MUST_MELD_FIRST': 'Laske ensin puolesi avaus',
  'err.INITIAL_MELD_NOT_MET': 'Ensimmäiseltä laskultasi puuttuu vielä pisteitä',
  'err.CANNOT_GO_OUT_YET': 'Puolesi tarvitsee valmiin canastan ennen kuin se pääsee ulos',
  'err.NOTHING_TO_CALL': 'Ei ole panosta maksettavaksi',
  'err.CANNOT_CHECK': 'Et voi tsekata — vastattavana on panos',
  'err.CANNOT_RAISE': 'Tässä et voi korottaa',
  'err.RAISE_TOO_SMALL': 'Korotuksen on oltava vähintään edellisen suuruinen',
  'err.NOT_ENOUGH_CHIPS': 'Sinulla ei ole niin monta pelimerkkiä',
  'err.AMOUNT_REQUIRED': 'Kerro paljonko',
  'err.AMOUNT_NOT_A_NUMBER': 'Tuo summa ei ole luku',
  'err.SEAT_NOT_IN_HAND': 'Et ole mukana tässä jaossa',
  'err.WRONG_RANK': 'Tuo kortti on tähän väärän arvoinen',
  'err.MATCH_FULL': 'Pöytä on täynnä',
  'err.MATCH_ALREADY_STARTED': 'Ottelu on jo alkanut',
  'err.TOO_FEW_PLAYERS': 'Pelaajia ei ole vielä tarpeeksi',
  'err.WRONG_PLAYER_COUNT': 'Tätä peliä ei voi pelata noin monella pelaajalla',
  'err.NOT_THE_HOST': 'Sen voi tehdä vain isäntä',
  'err.BAD_SEATING': 'Tuo paikkajärjestys ei vastaa pöydässä olijoita',
  'err.NO_LONGER_WAITING': 'Pöytä ei enää odota',
  'err.WAITING_ROOM_UNAVAILABLE': 'Odotushuone ei ole käytettävissä',
  'err.SERVER_BUSY': 'Palvelin on juuri nyt täynnä — yritä hetken kuluttua uudelleen',


  // --- rules the engine has always enforced and never stated ----------------
  // Every one of these was found by the guardrail rather than by review: the
  // validator could refuse a player for it, and no sentence anywhere said so.
  // See server/internal/zolikmod/ruleindex_test.go.
  'zolik.rules.section.layoff': 'Yhdistelmiin lisääminen',
  'zolik.rules.pickup.obligation':
    'Ennen kuin olet laskenut alas, poistopinosta otettu kortti on käytettävä siihen yhdistelmään, jolla lasket alas tällä vuorolla.',
  'zolik.rules.pickup.noReturn':
    'Poistopinosta ottamaasi korttia ei saa poistaa uudelleen samalla vuorolla — pelaa se tai pidä se.',
  'zolik.rules.wilds.setLimit': 'Ryhmässä ei saa olla enempää jokereita kuin luonnollisia kortteja.',
  'zolik.rules.set.maxSize':
    'Ryhmässä saa olla enintään {n} korttia — jokeri korvaa puuttuvan maan, se ei täydennä täyttä ryhmää.',
  'zolik.rules.run.maxLength':
    'Suorassa saa olla enintään {n} korttia — ässä alimpana, kaksitoista arvoa sen yläpuolella ja ässä ylimpänä.',
  'zolik.rules.run.aceBridge':
    'Ässä on kuninkaan yläpuolella tai kakkosen alapuolella, ei koskaan siltana suoran kahden pään välillä.',
  'zolik.rules.contracts.contribution':
    'Kunnes olet laskenut alas, jokaisen laskemasi yhdistelmän on oltava sellainen, jota jaon sopimus vielä vaatii.',
  'zolik.rules.layoff.afterDown':
    'Et voi lisätä muiden yhdistelmiin ennen kuin olet laskenut oman sopimuksesi.',
  'zolik.rules.layoff.runEnds': 'Suoraan lisätyn kortin on jatkettava sitä jommastakummasta päästä.',
  'zolik.rules.jokers.swap':
    'Pöydässä olevan yhdistelmän jokerin saa lunastaa täsmälleen sillä kortilla, jota se edustaa.',
  'zolik.rules.jokers.reclaim.on':
    'Pöydästä lunastettu jokeri on pelattava yhdistelmään samalla vuorolla — sitä ei saa jättää käteen.',
  'zolik.rules.jokers.reclaim.off': 'Pöydästä lunastetun jokerin saa jättää käteen.',
  'zolik.rules.deck.reshuffle':
    'Kun nostopakka loppuu, poistopino sekoitetaan ja siitä tulee uusi nostopakka; jos molemmat ovat tyhjiä, jako päättyy.',


  // --- what to do instead ---------------------------------------------------
  // The third layer of a refusal, after the reason and the rule. Sent by the
  // module because it is the only side that knows which card is owed and
  // which way out is on offer; see server/internal/zolikmod/remedy.go.
  'zolik.remedy.meldThePickup': 'Lisää {card} laskuusi tai peru nosto.',
  'zolik.remedy.discardSomethingElse': 'Poista toinen kortti tai pelaa {card} tällä vuorolla.',
  'zolik.remedy.discardNotAJoker': 'Poista jokin muu kuin jokeri.',
  'zolik.remedy.finishOrUndoLayDown': 'Viimeistele laskusi tai ota se takaisin.',
  'zolik.remedy.needMorePoints': 'Tarvitset vielä {n} pistettä, ennen kuin voit laskea alas.',
  'zolik.remedy.layACleanRun': 'Laske suora, jossa ei ole jokeria.',
  'zolik.remedy.playReclaimedJoker': 'Pelaa {card} yhdistelmään tai peru sen ottaminen.',
  'zolik.remedy.goDownFirst': 'Laske ensin omat yhdistelmäsi.',
  'zolik.remedy.drawFirst': 'Nosta ensin kortti.',
  'zolik.remedy.drawFromStock': 'Nosta nostopakasta — poistopino aukeaa kierroksella {n}.',
  'zolik.remedy.drawFromStockEmpty': 'Nosta sen sijaan nostopakasta.',


  // --- keys whose meaning lives in their params -----------------------------
  // The shape fallback renders these with the number missing ("Stack", "Pot"),
  // which is why they earn a line where most keys do not.
  'header.contract': 'Vaatii {sets} ryhmää ja {runs} suoraa',
  'header.contract.cleanRunOnly': 'Vaatii jokerittoman suoran',
  'header.round': 'Kierros {n}',
  'header.deck': 'Nostopakka',
  'header.target': 'Tavoite',
  'header.suitInPlay': 'Maa pelissä',
  'seat.cards': 'Kortit',
  'zolik.offer.meld': 'Laske',
  'prompt.pickupMustBeMelded':
    '{value} tuli poistopinosta — sen on mentävä niihin yhdistelmiin, joilla lasket alas tällä vuorolla.',
  'prompt.jokerMustBePlayed':
    '{value} tuli pöydästä — sen on mentävä yhdistelmään ennen kuin voit päättää vuorosi.',
  'prompt.initialMeld': 'Puolesi avauksen on yllettävä {n} pisteeseen.',
  'prompt.canastasNeeded': 'Puolesi tarvitsee vielä {n} canastaa ennen kuin se pääsee ulos.',
  'prompt.mustDrawOrAnswerSeven': 'Vastaa seiskalla tai nosta {n} korttia.',
  'prompt.chooseSuit': 'Valitse maa, joka jatkuu',
  'prompt.skipPending': 'Vuorosi ohitetaan',
  'status.lastDeal': 'Joukkue {team} sai {value}',
  'status.teamScore': 'Joukkue {team}: {value}',
  'canasta.offer.rank': 'Arvo',
  'canasta.offer.sequence': 'Jono',
  'badge.naturalCanasta': 'Puhdas canasta',
  'badge.mixedCanasta': 'Epäpuhdas canasta',
  'badge.samba': 'Samba',
  'badge.cleanRun': 'Puhdas jono',
  'canasta.seat.teamScore': 'Joukkueen pisteet',
  'canasta.seat.canastas': 'Canastat',
  'holdem.header.pot': 'Potti',
  'holdem.header.street': 'Katu',
  'holdem.header.hand': 'Käsi',
  'holdem.header.handLimit': 'Käsiä yhteensä',
  'holdem.header.blinds': 'Blindit',
  'holdem.cost.call': 'maksuun',
  'holdem.cost.pot': 'potissa',
  'holdem.seat.stack': 'Pino',
  'holdem.seat.bet': 'Panos',
  'holdem.prompt.yourAction': 'Sinun vuorosi',
  'holdem.prompt.raiseTo': 'Korota määrään',
  'holdem.quick.halfPot': '½ Potti',
  'holdem.quick.pot': 'Potti',
  'holdem.quick.allIn': 'All-in',
  'zone.yourHand': 'Kätesi',
  'zone.opponentHand': 'Vastustajan käsi',
  'zone.drawPile': 'Nostopakka',
  'zone.discardPile': 'Poistopino',
  'zone.melds': 'Yhdistelmät',
  'zone.teamMelds': 'Puolesi yhdistelmät',
  'zone.opponentMelds': 'Vastustajan yhdistelmät',
  'zone.redThrees': 'Punaiset kolmoset',
  'zone.board': 'Pöytä',
  'verb.drawFromDeck': 'Nosta',
  'verb.takeFromDiscard': 'Ota pinosta',


  // --- the why sheet's own furniture ---------------------------------------
  // The three layers a refusal is explained in. Labels, not sentences: what
  // goes under each is sent by the server or worded above.
  'why.reason': 'Miksi ei',
  'why.rule': 'Sääntö',
  'why.rules': 'Säännöt',
  'why.remedy': 'Mitä voit tehdä',
  'why.readTheRules': 'Lue koko säännöt →',
  'why.close': 'Sulje',
  'why.open': 'miksi',

  // --- marks on a particular card ------------------------------------------
  // A mark is a refusal that has not happened yet: the rule is enforced at the
  // discard, which is the last possible moment to hear about it, so the card
  // says so while there is still a turn left to act on it.
  'zolik.badge.owedToMeld':
    '{card} tuli poistopinosta — sen on mentävä niihin yhdistelmiin, joilla lasket alas tällä vuorolla.',
  'zolik.badge.jokerOwed':
    '{card} tuli pöydästä — sen on mentävä yhdistelmään ennen kuin voit päättää vuorosi.',

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
  'legal.terms': 'Ehdot',
  // The document's own name, used as the heading above it. Kept apart from
  // `legal.notice.terms`, which is the same words inflected to sit inside a
  // sentence — a heading in the instrumental case reads as a mistake in every
  // Slavic language.
  'legal.terms.title': 'Käyttöehdot',
  'legal.privacy.title': 'Tietosuojailmoitus',
  'legal.privacy': 'Tietosuoja',
  'legal.source': 'Lähdekoodi',
  'legal.updated': 'Versio {version}',
  'legal.draft': 'Luonnos — ei vielä voimassa. Ylläpitäjän nimi, maa ja yhteysosoite ovat vielä täyttämättä.',
  'legal.notice.before': 'Pelaamalla hyväksyt ',
  'legal.notice.terms': 'käyttöehdot',
  'legal.notice.between': '. Se, mitä sinusta tallennetaan, kerrotaan ',
  'legal.notice.privacy': 'tietosuojailmoituksessa',
  'legal.notice.after': '.',
  // --- gin rummy -------------------------------------------------------------
  'err.UPCARD_DECLINED': 'Olet jo ohittanut tuon kortin',
  'err.DEADWOOD_TOO_HIGH': 'Deadwoodisi on liian korkea koputukseen',
  'err.CARD_DOES_NOT_EXTEND_MELD': 'Tuo kortti ei jatka tätä yhdistelmää',
  'ginrummy.rules.setup': 'Valmistelu',
  'ginrummy.rules.turn': 'Sinun vuorosi',
  'ginrummy.rules.melds': 'Yhdistelmät',
  'ginrummy.rules.knocking': 'Koputus',
  'ginrummy.rules.bigGin': 'Big gin',
  'ginrummy.rules.layoff': 'Liittäminen',
  'ginrummy.rules.deadHand': 'Kuollut jako',
  'ginrummy.rules.scoring': 'Jaon pisteytys',
  'ginrummy.rules.match': 'Ottelun voittaminen',
  'ginrummy.rules.lineBonuses': 'Loppulaskennan bonukset',
  'ginrummy.rules.deck': 'Pelataan {value} kortin pakalla.',
  'ginrummy.rules.deal': 'Jokainen pelaaja saa {value} korttia.',
  'ginrummy.rules.upcard': 'Vielä yksi kortti käännetään kuvapuoli ylöspäin aloittamaan poistopino.',
  'ginrummy.rules.drawDiscard':
    'Nosta vuorollasi yksi kortti — nostopakasta tai poistopinosta — ja poista sitten yksi.',
  'ginrummy.rules.setsAndRuns':
    'Yhdistelmä on kolmen tai neljän samanarvoisen kortin ryhmä tai kolmen tai useamman samanmaisen kortin suora.',
  'ginrummy.rules.aceLow': 'Ässä on aina matala — rouvasta ässään ei ole suoraa.',
  'ginrummy.rules.knockLimit': 'Voit koputtaa heti kun deadwoodisi on {n} tai vähemmän.',
  'ginrummy.rules.oklahoma': 'Tämän jaon koputusrajan määrää avatun kortin arvo.',
  'ginrummy.rules.gin': 'Nolla deadwoodia on gin — paras mahdollinen koputus.',
  'ginrummy.rules.bigGinBonus':
    'Yksitoista korttia kaikki yhdistelmissä, ilman minkäänlaista poistoa, on big gin ja tuo vielä {n} pistettä.',
  'ginrummy.rules.layoffDescription':
    'Koputuksen jälkeen, joka ei ole gin, vastustajasi saa liittää oman deadwoodinsa sinun yhdistelmiisi ennen kuin kädet vertaillaan.',
  'ginrummy.rules.deadHandDescription':
    'Jos nostopakassa on enää kaksi korttia eikä kukaan ole koputtanut, jako on kuollut — kukaan ei saa pisteitä ja sama jakaja jakaa uudelleen.',
  'ginrummy.rules.undercut':
    'Jos vastustajasi deadwood ei ole sinun deadwoodiasi korkeampi, hän alittaa sinut: hän saa erotuksen ja lisäksi {n}.',
  'ginrummy.rules.ginBonus': 'Gin tuo vastustajasi koko käden ja lisäksi {n}.',
  'ginrummy.rules.target': 'Ensimmäisenä {n} pisteen yli jaon päättyessä yltävä voittaa ottelun.',
  'ginrummy.rules.shutout':
    'Ottelubonus kaksinkertaistuu {n} pisteeseen, jos häviäjä ei saanut yhtäkään pistettä.',
  'ginrummy.rules.box': 'Jokainen voittamasi jako on ottelun lopussa {n} pisteen arvoinen.',
  'ginrummy.rules.gameBonus': 'Ottelun voittaminen tuo vielä {n} pistettä.',
  'ginrummy.fact.deadwood': '{value} deadwoodia',
  'ginrummy.fact.discardCard': 'Poista {value}',
  'ginrummy.fact.meldCards': 'Kohteeseen {value}',
  'ginrummy.header.hand': 'Käsi {n}',
  'ginrummy.round.kind': '{value}',
  'ginrummy.round.hand': 'Käsi',
  'ginrummy.seat.score': '{value}',
  'ginrummy.seat.dealer': 'Jakaja',
  'ginrummy.status.knocked': '{playerId} koputti {deadwood} deadwoodilla',
  'ginrummy.status.gin': '{playerId} teki ginin',
  'ginrummy.status.lastHand': 'Viimeisin käsi: {winner} ({kind}, {delta} pistettä)',
  'ginrummy.offer.drawStock': 'Nosta nostopakasta',
  'ginrummy.offer.drawDiscard': 'Nosta poistopinosta',
  'ginrummy.offer.takeUpcard': 'Ota avattu kortti',
  'ginrummy.offer.passUpcard': 'Passaa',
  'ginrummy.offer.discard': 'Poista',
  'ginrummy.offer.knock': 'Koputa',
  'ginrummy.offer.gin': 'Gin!',
  'ginrummy.offer.bigGin': 'Big gin!',
  'ginrummy.offer.layOff': 'Liitä',
  'ginrummy.offer.finishLayoff': 'Liittäminen valmis',
  'ginrummy.zone.knockerHand': 'Koputtajan käsi',
  'ginrummy.zone.melds': 'Yhdistelmät',
  'ginrummy.prompt.upcardDecision': 'Ota avattu kortti tai passaa',
  'ginrummy.prompt.yourTurnDraw': 'Nosta kortti',
  'ginrummy.prompt.yourTurnDiscard': 'Poista — tai koputa, jos voit',
  'ginrummy.prompt.layoff': 'Liitä deadwoodia tai lopeta',

  // --- rummy tiles -------------------------------------------------------------
  'err.TILE_NOT_IN_HAND': 'Tuo laatta ei ole kädessäsi',
  'err.TILE_DOES_NOT_FIT': 'Tuo ei sovi siihen',
  'err.NO_SUCH_SET': 'Tuota yhdistelmää ei ole pöydässä',
  'err.INITIAL_MELD_ONLY': 'Ennen ensimmäistä laskuasi voit järjestellä vain omia uusia yhdistelmiäsi',
  'err.TABLE_NOT_VALID': 'Pöytä ei ole vielä kelvollinen',
  'err.TRAY_NOT_EMPTY': 'Sinulla on vielä irrallisia laattoja sijoitettavana',
  'err.NOTHING_PLAYED': 'Pelaa vähintään yksi laatta ennen kuin päätät vuorosi',
  'err.INITIAL_MELD_TOO_LOW': 'Ensimmäisen laskusi on oltava vähintään 30 pisteen arvoinen',
  'err.NOT_A_RUN': 'Vain suoran voi jakaa',
  'err.BAD_SPLIT_POSITION': 'Tuosta kohdasta tätä suoraa ei voi jakaa',
  'err.NO_JOKER_IN_SET': 'Tuossa yhdistelmässä ei ole jokeria',
  'err.TILE_JOKER_SWAP_MISMATCH': 'Tuo laatta ei ole se, mitä jokeri edustaa',
  'rummytiles.rules.setup': 'Valmistelu',
  'rummytiles.rules.sets': 'Yhdistelmät',
  'rummytiles.rules.initialMeld': 'Avausasetelma',
  'rummytiles.rules.turn': 'Sinun vuorosi',
  'rummytiles.rules.jokerTaking': 'Jokerin ottaminen',
  'rummytiles.rules.ending': 'Kierroksen päättäminen',
  'rummytiles.rules.poolExhaustion': 'Jos pussi tyhjenee',
  'rummytiles.rules.match': 'Ottelun voittaminen',
  'rummytiles.rules.tiles': 'Pelataan {value} laatalla.',
  'rummytiles.rules.dealCount': 'Jokainen pelaaja saa {value} laattaa.',
  'rummytiles.rules.group': 'Ryhmä on kolme tai neljä saman numeron laattaa, kukin eri värissä.',
  'rummytiles.rules.run': 'Suora on kolme tai useampi peräkkäinen numero samassa värissä.',
  'rummytiles.rules.noWrap': '13 ei jatku takaisin ykköseen.',
  'rummytiles.rules.joker': 'Jokeri edustaa mitä tahansa laattaa.',
  'rummytiles.rules.initialMeldDescription':
    'Ennen kuin olet laskenut {n} pistettä tai enemmän yhdellä vuorolla, pelkästään omasta kädestäsi, et saa koskea mihinkään, mikä on jo pöydässä.',
  'rummytiles.rules.turnDescription':
    'Pelaa vähintään yksi laatta kädestäsi, järjestele pöytää vapaasti, ja päätä vuoro niin että jokainen pöydän yhdistelmä on kelvollinen.',
  'rummytiles.rules.noDiscard':
    'Poistoa ei ole — jos et pysty tekemään kelvollista vuoroa, nostat sen sijaan yhden laatan.',
  'rummytiles.rules.jokerTakingDescription':
    'Pöydässä olevan jokerin saa ottaa korvaamalla sen kädestäsi sillä laatalla, jota se edustaa — ja se on käytettävä yhdistelmässä ennen vuorosi loppua.',
  'rummytiles.rules.goingOut':
    'Ensimmäisenä laatoistaan pääsevä pelaaja voittaa kierroksen. Kaikki muut saavat käteen jääneiden laattojen arvon miinusmerkkisenä; voittaja saa kaikkien muiden menetysten summan.',
  'rummytiles.rules.poolExhaustionLowestWins':
    'Jos pussi tyhjenee eikä kukaan voi pelata, kierros päättyy ja sen voittaa matalin käsi.',
  'rummytiles.rules.poolExhaustionNoWinner':
    'Jos pussi tyhjenee eikä kukaan voi pelata, kierros päättyy ilman voittajaa — jokainen käsi vain lasketaan.',
  'rummytiles.rules.target': 'Ensimmäisenä {n} pisteen yli kierroksen päättyessä yltävä voittaa ottelun.',
  'rummytiles.rules.roundLimit': 'Ottelu päättyy {n} kierroksen jälkeen — korkein pistemäärä voittaa.',
  'rummytiles.fact.setCards': '{value}',
  'rummytiles.header.pool': 'Pussi {n}',
  'rummytiles.header.round': 'Kierros {n}',
  'rummytiles.round.kind': '{value}',
  'rummytiles.round.round': 'Kierros',
  'rummytiles.seat.score': '{value}',
  'rummytiles.seat.notOpened': 'Ei avattu',
  'rummytiles.status.lastRound': 'Viimeisin kierros: {winner} ({kind})',
  'rummytiles.badge.invalid': 'Ei vielä kelvollinen',
  'rummytiles.zone.pool': 'Pussi',
  'rummytiles.zone.table': 'Pöytä',
  'rummytiles.zone.tray': 'Teline',
  'rummytiles.offer.place': 'Aseta',
  'rummytiles.offer.addFromHand': 'Lisää',
  'rummytiles.offer.addFromTray': 'Lisää telineestä',
  'rummytiles.offer.take': 'Ota',
  'rummytiles.offer.split': 'Jaa',
  'rummytiles.offer.swapJoker': 'Vaihda jokeri',
  'rummytiles.offer.resetTurn': 'Nollaa vuoro',
  'rummytiles.offer.commit': 'Valmis',
  'rummytiles.offer.draw': 'Nosta',
  'rummytiles.param.position': 'Jaa kohdasta',

  // --- blackjack -----------------------------------------------------------
  //
  // The house rules are worded twice over — on and off — because a rule that
  // is *not* in force is still something a player has to be told. "No
  // surrender at this table" is information; silence is a guess.
  'err.BET_BELOW_MINIMUM': 'Tuo on alle pöydän minimin',
  'err.ALREADY_BET': 'Panoksesi on jo pöydässä',
  'err.INSURANCE_CLOSED': 'Juuri nyt ei ole vakuutusta otettavissa',
  'err.CANNOT_DOUBLE': 'Tätä kättä ei voi tuplata',
  'err.CANNOT_SPLIT': 'Tätä kättä ei voi jakaa',
  'err.CANNOT_SURRENDER': 'Tästä kädestä ei voi luopua',

  'blackjack.rules.section.table': 'Pöytä',
  'blackjack.rules.section.play': 'Käden pelaaminen',
  'blackjack.rules.section.dealer': 'Jakaja',
  'blackjack.rules.section.end': 'Miten ottelu päättyy',
  'blackjack.rules.goal':
    'Voita jakaja menemättä yli kahdenkymmenenyhden. Yli meneminen häviää heti, tekipä jakaja jälkeenpäin mitä tahansa.',
  // Counts are their own phrase rather than a number glued to a noun: one
  // deck and six decks inflect differently, and Czech inflects them again.
  'blackjack.rules.decks': 'Pakkoja kengässä: {n}.',
  'blackjack.rules.stack': 'Jokainen paikka istuutuu {n} pelimerkillä.',
  'blackjack.rules.minBet': 'Pöydän minimi on {n} pelimerkkiä.',
  'blackjack.rules.faceUp':
    'Pelaajien kortit jaetaan kuvapuoli ylöspäin; jakaja pitää yhden kortin piilossa, kunnes kaikki ovat pelanneet.',
  'blackjack.rules.hitStand': 'Ota niin monta korttia kuin haluat, tai jää siihen mitä sinulla on.',
  'blackjack.rules.aces': 'Ässä lasketaan yhdeksitoista niin kauan kuin se mahtuu, ja muuten ykköseksi.',
  'blackjack.rules.blackjack': 'Ässä ja kympin arvoinen kortti kahdella ensimmäisellä kortilla on blackjack.',
  'blackjack.rules.pays3to2': 'Blackjack maksaa 3:2.',
  'blackjack.rules.pays6to5': 'Blackjack maksaa 6:5.',
  'blackjack.rules.paysEven': 'Blackjack maksaa yksi yhteen.',
  'blackjack.rules.double':
    'Kahdella ensimmäisellä kortillasi saat tuplata panoksesi ja ottaa täsmälleen yhden lisäkortin.',
  'blackjack.rules.doubleAfterSplit': 'Myös jaosta syntyneen käden saa tuplata.',
  'blackjack.rules.noDoubleAfterSplit': 'Jaosta syntynyttä kättä ei saa tuplata.',
  'blackjack.rules.split':
    'Kaksi samanarvoista korttia saa jakaa omiksi käsikseen, kukin omalla panoksellaan — enintään {n} kertaa, yhteensä {hands} kädeksi.',
  'blackjack.rules.noSplit': 'Tässä pöydässä pareja ei jaeta.',
  'blackjack.rules.splitAces':
    'Jaetut ässät saavat yhden kortin kumpikin ja jäävät sitten, eikä näin syntynyt kaksikymmentäyksi ole blackjack.',
  'blackjack.rules.surrender':
    'Voit luopua ensimmäisestä kädestäsi puolella panoksesta, kun jakaja on tarkistanut blackjackin.',
  'blackjack.rules.noSurrender': 'Tässä pöydässä käsistä ei voi luopua.',
  'blackjack.rules.dealerDraws': 'Jakaja ottaa seitsemääntoista asti ja jää sitten.',
  'blackjack.rules.hitsSoft17': 'Jakaja ottaa kortin ässällä muodostetulla seitsemällätoista.',
  'blackjack.rules.standsSoft17': 'Jakaja jää ässällä muodostettuun seitsemääntoista.',
  'blackjack.rules.dealerPeeks':
    'Ässä tai kymppi näkyvissä jakaja tarkistaa blackjackin ennen kuin kukaan pelaa.',
  'blackjack.rules.insurance':
    'Jakajan ässää vastaan voit vakuuttaa puolella panoksestasi; se maksaa 2:1, jos jakajalla on blackjack.',
  'blackjack.rules.noInsurance': 'Tässä pöydässä ei tarjota vakuutusta.',
  'blackjack.rules.rounds': 'Pöydässä pelataan {n} kierrosta.',
  'blackjack.rules.mostChipsWins': 'Eniten pelimerkkejä lopussa omistava voittaa ottelun.',
  'blackjack.rules.bustedOut':
    'Paikka, joka ei enää pysty kattamaan {n} minimiä, on sivussa loppuottelun ajan.',

  'blackjack.zone.dealer': 'Jakaja',
  'blackjack.zone.box': 'Käsi',
  'blackjack.zone.yourBox': 'Kätesi',
  'blackjack.zone.shoe': 'Kenkä',

  'blackjack.header.round': 'Kierros {n}/{of}',
  'blackjack.header.minBet': 'Minimi',
  'blackjack.header.decks': 'Pakat',
  'blackjack.header.dealerTotal': 'Jakaja näyttää {n}',
  'blackjack.header.dealerSoftTotal': 'Jakaja näyttää pehmeän {n}',

  'blackjack.seat.stack': 'Pelimerkit',
  'blackjack.seat.bet': 'Panos',
  'blackjack.seat.insurance': 'Vakuutus',
  'blackjack.seat.total': 'Yhteensä',
  'blackjack.seat.softTotal': 'Pehmeä summa',
  'blackjack.seat.out': 'Pelimerkit loppu',

  'blackjack.prompt.placeBet': 'Aseta panoksesi',
  'blackjack.prompt.insurance': 'Vakuutus?',
  'blackjack.prompt.yourMove': 'Sinun vuorosi',
  'blackjack.prompt.waitingFor': 'Odotetaan pelaajaa {playerId}',
  'blackjack.prompt.betAmount': 'Panos',

  'blackjack.quick.doubleMin': '2× Minimi',
  'blackjack.quick.allIn': 'All-in',
  'blackjack.offer.bet': 'Panosta',
  'blackjack.offer.hit': 'Kortti',
  'blackjack.offer.stand': 'Jään',
  'blackjack.offer.double': 'Tuplaa',
  'blackjack.offer.split': 'Jaa',
  'blackjack.offer.surrender': 'Luovu',
  'blackjack.offer.insure': 'Ota vakuutus',
  'blackjack.offer.declineInsurance': 'Ei vakuutusta',

  'blackjack.fact.tableMinimum': 'minimi',
  'blackjack.fact.insuranceCost': 'vakuutukseen',
  'blackjack.fact.extraStake': 'panokseen',
  'blackjack.fact.surrenderReturn': 'takaisin',

  'blackjack.status.dealerBlackjack': 'Jakajalla oli blackjack',
  'blackjack.status.dealerBust': 'Jakaja meni yli lukemalla {n}',
  'blackjack.status.dealerStands': 'Jakaja jää lukemaan {n}',

  'blackjack.round.name': 'Kierros',
  'blackjack.round.dealerTotal': 'Jakaja {n}',
  'blackjack.round.dealerBust': 'Jakaja yli ({n})',
  'blackjack.round.dealerBlackjack': 'Jakajan blackjack',
  'blackjack.round.outcome.blackjack': 'Blackjack',
  'blackjack.round.outcome.win': 'Voitto',
  'blackjack.round.outcome.push': 'Tasan',
  'blackjack.round.outcome.lose': 'Häviö',
  'blackjack.round.outcome.bust': 'Yli',
  'blackjack.round.outcome.surrender': 'Luovuttu',

  'blackjack.badge.inPlay': 'Pelissä',
  'blackjack.badge.doubled': 'Tuplattu',
  'blackjack.badge.split': 'Jaettu',
  'blackjack.badge.blackjack': 'Blackjack',
  'blackjack.badge.bust': 'Yli',
  'blackjack.badge.won': 'Voitto',
  'blackjack.badge.push': 'Tasan',
  'blackjack.badge.lost': 'Häviö',
  'blackjack.badge.surrendered': 'Luovuttu',

  'blackjack.unit.chips': 'pelimerkkiä',

  // --- the settings screen --------------------------------------------------
  //
  // Hardcoded English until now, which was survivable while the only other
  // language was unreachable and faintly absurd once the language picker
  // itself lived on this screen: a player who cannot read "Settings" is
  // exactly the one who came here to change it.
  'settings.title': 'Asetukset',
  'settings.signedInAs': 'Kirjautunut sisään nimellä {username}',
  'settings.playingAsGuest': 'Pelaat vieraana {username} (vieras)',
  'settings.notSignedIn':
    'Et ole kirjautunut sisään — kirjaudu sisään tai jatka vieraana pelataksesi verkossa.',
  'settings.subtitle': 'Miltä sinä näytät ja miltä pöytä näyttää',
  'settings.face.heading': 'Kasvosi pöydässä',
  'settings.face.account': 'Tallennetaan tiliisi, joten se seuraa sinua toiselle laitteelle.',
  'settings.face.device': 'Tallennetaan tälle laitteelle. Kirjaudu sisään, niin saat sen mukaasi.',
  'settings.skin.heading': 'Pöydän ulkoasu',
  'settings.language.heading': 'Kieli',
  'settings.language.status': 'Tallennetaan tälle laitteelle.',
  'settings.language.auto': 'Automaattinen',
  'settings.language.auto.now': 'Seuraa laitettasi — nyt {language}',
  'settings.legal.heading': 'Pienellä painettu',
  'settings.legal.status': 'Mihin suostuit pelaamalla ja mitä sinusta tallennetaan.',
  'settings.signIn': 'Kirjaudu sisään',
  'settings.back': 'Takaisin',

  // A translated interface over an English notice is a worse lie than an
  // English interface, so the document says so itself rather than letting the
  // surrounding screen imply the text was written for this reader.
  'legal.untranslated':
    'Tätä ilmoitusta ei ole vielä käännetty kielellesi. Alla oleva englanninkielinen teksti on se versio, joka pätee.',

  // --- the navigation bar's screen titles ------------------------------------
  //
  // These sit in `app/_layout.tsx`, above every screen, and were the last
  // English left after the settings screen was translated — a German player
  // reading "Einstellungen" under a bar that said "Settings". A translated app
  // with an untranslated chrome bar looks like a bug, not like a limit.
  'nav.home': 'Zolik',
  'nav.emailSignIn': 'Kirjautuminen sähköpostilla',
  'nav.signingIn': 'Kirjaudutaan',
  'nav.usernameSignIn': 'Kirjautuminen käyttäjänimellä',
  'nav.legacyAccount': 'Vanha tili',
  'nav.guest': 'Vieras',
  'nav.account': 'Tili',
  'nav.games': 'Pelit',
  'nav.table': 'Sinun pöytäsi',
  'nav.join': 'Liity pöytään',
  // The screen a shared invite link lands on while it resolves the code.
  'nav.joining': 'Liitytään',
  'nav.rules': 'Säännöt',
  'nav.match': 'Ottelu',
  'nav.scoreTable': 'Pistetaulukko',
  'nav.stats': 'Tilastot',
  'nav.more': 'Lisää',
  'nav.about': 'Tietoja',

  // --- the account menu behind the face in the corner ----------------------
  'menu.label': 'Tilivalikko',
  'menu.signedIn': 'Kirjautunut sisään',
  'menu.notSignedIn': 'Ei kirjautunut',
  'menu.keepStats': 'jotta tilastosi säilyvät',
  'menu.signOut': 'Kirjaudu ulos',
  'more.scoreTable': 'Offline-pistetaulukko',
  'more.stats': 'Tilastot ja tulostaulu',
  'more.needsAccount': 'kirjaudu käyttääksesi',
  'gate.title': 'Kirjaudu sisään käyttääksesi tätä',
  'gate.body':
    'Pistetaulukot ja tilastot tallennetaan tilillesi, joten ne seuraavat sinua toiselle laitteelle. Vieraalla ei ole niille tallennuspaikkaa.',

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
  'error.generic': 'Se ei onnistunut',
  'error.signIn': 'Kirjautuminen epäonnistui',
  'error.login': 'Kirjautuminen epäonnistui',
  'error.register': 'Rekisteröityminen epäonnistui',
  'error.sendCode': 'Koodia ei voitu lähettää',
  'error.badCode': 'Tuo koodi ei toiminut',
  'error.rulesLoad': 'Sääntöjä ei voitu ladata',
  'error.createFailed': 'Luonti epäonnistui',
  'error.saveFailed': 'Tallennus epäonnistui',
  'error.exportFailed': 'Vienti epäonnistui',

  // --- the route that does not exist ----------------------------------------
  'notFound.title': 'Hupsis!',
  'notFound.message': 'Tätä näkymää ei ole.',
  'notFound.home': 'Siirry aloitusnäkymään!',

  // --- signing in -----------------------------------------------------------
  'auth.login.subtitle': 'Säilytä tilastosi kaikilla laitteilla',
  'auth.login.continueWithEmail': 'Jatka sähköpostilla',
  'auth.login.usernameInstead': 'Kirjaudu sen sijaan käyttäjänimellä',
  'auth.email.title': 'Kirjautuminen sähköpostilla',
  'auth.email.subtitle': 'Lähetämme sinulle kertakäyttöisen koodin',
  'auth.email.address': 'Sähköpostiosoite',
  'auth.email.send': 'Lähetä koodi',
  'auth.email.codeTitle': 'Syötä koodi',
  'auth.email.codePlaceholder': 'Kuusinumeroinen koodi',
  'auth.email.differentAddress': 'Käytä toista osoitetta',
  'auth.email.sentTo': 'Lähetetty osoitteeseen {email}',
  'auth.email.continue': 'Jatka',
  'auth.guest.title': 'Pelaa vieraana',
  'auth.guest.subtitle': 'Tiliä ei tarvita',
  'auth.guest.displayName': 'Näyttönimi',
  'auth.register.title': 'Luo tili',
  'auth.register.username': 'Käyttäjänimi',
  'auth.register.email': 'Sähköposti (valinnainen)',
  'auth.register.password': 'Salasana',
  'auth.username.createAccount': 'Luo tili käyttäjänimellä ja salasanalla',
  'auth.callback.signedIn': 'Kirjauduttu sisään.',

  // --- the account screen ---------------------------------------------------
  'account.signInPrompt': 'Kirjaudu sisään hallitaksesi tiliäsi.',
  'account.keepGames': 'Säilytä nämä pelit',
  'account.signedInWith': 'Kirjauduttu tunnuksella',
  'account.addMethod': 'Lisää kirjautumistapa',
  'account.usernameAndPassword': 'Käyttäjänimi ja salasana',
  'account.faceAndTable': 'Kasvot ja pöydän ulkoasu',
  'account.refresh': 'Päivitä',
  'account.remove': 'Poista',

  // --- the main menu --------------------------------------------------------
  'home.subtitle': 'Continental-rommi · {server}',
  'home.playingAs': 'Pelaat nimellä {name}',
  'home.signInPrompt': 'Kirjaudu sisään tai jatka vieraana pelataksesi verkossa.',
  'home.statsAndLeaderboard': 'Tilastot ja tulostaulu',
  'home.play': 'Pelaa',
  'home.offlineScoreTable': 'Pistetaulukko ilman verkkoa',
  'home.signInToKeepStats': 'Kirjaudu sisään säilyttääksesi tilastot',
  'home.signOut': 'Kirjaudu ulos',
  'home.continueAsGuest': 'Jatka vieraana',
  // Someone playing as a guest, marked in a list of names.
  'home.guestSuffix': '(vieras)',

  // --- the waiting room on the main menu ------------------------------------
  //
  // Counts are whole phrases per count, not a number glued to a noun — the
  // same reason `countLabel` exists. One and many are the cases the interface
  // actually produces, so they are the cases that get their own wording.
  'waiting.checking': 'Katsotaan, ketä on paikalla…',
  'waiting.youAreWaiting': 'Odotat pääseväsi pelaamaan',
  'waiting.pickedUp':
    'Kuka tahansa pöydän avaava voi poimia sinut mukaan — kukaan ei tarvitse sinulta koodia.',
  'waiting.othersOne': '1 muu pelaaja odottaa myös',
  'waiting.othersMany': '{n} muuta pelaajaa odottaa myös',
  'waiting.oneWaiting': '1 pelaaja odottaa pääsevänsä pelaamaan',
  'waiting.manyWaiting': '{n} pelaajaa odottaa pääsevänsä pelaamaan',
  'waiting.adding': 'Sinua lisätään jonoon…',
  'waiting.slowHint':
    'Jos tämä ei valmistu muutamassa sekunnissa, tarkista, että alla oleva palvelinosoite on tavoitettavissa tältä laitteelta.',
  'waiting.serverBusyDetail':
    'Yritys {n}. Palvelin ei juuri nyt ota vastaan uusia yhteyksiä odotushuoneeseen.',
  'waiting.reconnecting': 'Yhteys katkesi — yhdistetään uudelleen…',
  'waiting.reconnectingDetail':
    'Yritys {n}. Näin voi käydä, jos laitteesi verkko vaihtui tai palvelin käynnistyi uudelleen.',
  'waiting.tryAgain': 'Yritä nyt uudelleen',
  'waiting.makeAvailable': 'Ilmoittaudu pelivalmiiksi',
  'waiting.stop': 'Lopeta odottaminen',
  'waiting.noneYet':
    'Juuri nyt kukaan ei odota pelaamista. Laita itsesi listalle, niin olet ensimmäinen, jonka kuka tahansa näkee.',
  'waiting.noOthersYet': 'Kukaan muu ei odota vielä. Isännät näkevät sinut silti ja voivat kutsua sinut.',
  'waiting.server': 'Palvelin',
  'waiting.none': 'Juuri nyt kukaan ei odota. Se, joka ilmoittautuu päävalikossa, ilmestyy tähän.',

  // --- landing on a shared invite link --------------------------------------
  'join.missingCode': 'Tästä linkistä puuttuu pöydän koodi.',
  'join.staleLink': 'Pyydä kutsujaltasi tuore linkki tai liity sen sijaan koodilla.',
  'join.enterCode': 'Syötä koodi',
  'join.backToMenu': 'Takaisin valikkoon',
  'join.takingSeat': 'Otetaan paikka…',
  'join.takingSeatAt': 'Otetaan paikka pelistä {game}…',

  // --- the lobby ------------------------------------------------------------
  'lobby.games.subtitle': 'Kaikki, mitä tämä palvelin osaa tarjota',
  'lobby.games.bots': 'Botit',
  'lobby.games.playBot': 'Pelaa bottia vastaan',
  'lobby.games.playBots': 'Pelaa {n} bottia vastaan',
  'lobby.games.openTable': 'Avaa pöytä',
  'lobby.games.players': '{n} pelaajaa',
  'lobby.games.playerRange': '{min}–{max} pelaajaa',
  'lobby.join.placeholder': 'Liittymiskoodi tai kutsulinkki',
  'lobby.join.needCode': 'Anna liittymiskoodi, linkki tai ottelun tunnus',
  'lobby.games.signInFirst': 'Kirjaudu ensin sisään',
  'lobby.join.action': 'Liity',
  'lobby.join.waitingTitle': 'Odotetaan isäntää',
  // Two whole sentences rather than one with a swapped noun: "a game of {game}"
  // does not survive a language that inflects the game's name after "of".
  'lobby.join.joinedGame': 'Liityit peliin {game} — odotetaan aloitusta',
  'lobby.join.joinedTable': 'Liityit pöytään — odotetaan aloitusta',
  'lobby.table.addBot': 'Lisää botti',
  'lobby.table.side': 'Puoli {n}',
  'lobby.table.shuffleSeats': 'Sekoita paikat',
  'lobby.table.moveSeatUp': 'Siirrä {name} paikkaa ylemmäs',
  'lobby.table.moveSeatDown': 'Siirrä {name} paikkaa alemmas',
  'lobby.table.start': 'Aloita',
  'lobby.table.waitingForHost': 'Odotetaan, että isäntä aloittaa…',

  // --- inviting someone to a table ------------------------------------------
  'invite.heading': 'Kutsu pelaajia',
  'invite.explain': 'Lähetä tämä linkki. Se, joka avaa sen, päätyy tähän pöytään — tiliä ei tarvita.',
  'invite.noAddress':
    'Tälle palvelimelle ei ole asetettu jaettavaa osoitetta, joten käytä alla olevaa koodia.',
  'invite.readOutCode': 'Tai sanele koodi:',
  'invite.copy': 'Kopioi linkki',
  'invite.share': 'Jaa linkki',
  'invite.copied': 'Kopioitu!',
  'invite.shared': 'Jaettu',

  // --- the match screen -----------------------------------------------------
  'match.waitingForTable': 'Odotetaan pöytää…',
  'match.waitingForPlayer': 'Odotetaan toista pelaajaa…',
  'match.nobodyWon': 'Kukaan ei voittanut.',
  'match.youWon': 'Voitit.',
  'match.finished': 'Tämä ottelu on päättynyt.',
  'match.inProgress': 'Ottelu käynnissä — kaikki on yhteydessä ja etenee normaalisti.',
  'match.connecting': 'Yhdistetään…',
  'match.abandonedTitle': 'Pöytä siirretty sivuun',
  'match.abandoned': 'Kukaan ei palannut tähän pöytään, joten se siirrettiin sivuun. Kortit ovat täsmälleen siinä, mihin jätit ne.',
  'match.resume': 'Jatka siitä, mihin jäit',
  'match.resuming': 'Palautetaan pöytää…',
  'match.controls': 'Ohjaimet',
  'match.over': 'Ottelu päättyi',
  'match.settingUp': 'Valmistellaan…',
  'match.playAgain': 'Pelaa uudelleen',
  'match.backToGames': 'Takaisin peleihin',
  'match.table': 'Pöytä',
  'match.opponents': 'Vastustajat',
  // Marks which seat is the reader's own, in a list of seats.
  'match.youSuffix': '(sinä)',
  // The winner line is composed from whole sentences rather than a name glued
  // to " won." — the verb agrees with the subject in most of these languages,
  // and a suffix cannot know that.
  'match.you': 'sinä',
  'match.someoneWon': '{name} voitti.',
  'match.wonBy': 'Voittaja: {names}.',
  'match.pausedFor': 'Tauolla — odotetaan, että {name} yhdistää uudelleen.',
  'match.results': 'Tulokset',
  'match.players': 'Pelaajat',
  'match.toPlay': 'vuorossa',

  // --- the offline score table ----------------------------------------------
  'scoring.namesHint': 'Nimet pilkuilla eroteltuina (4–8 pelaajaa)',
  'scoring.newSession': 'Uusi istunto',
  // A worked example, not a sentence: the names are placeholders a translator
  // may localise, the shape `name:score,` is what the parser needs.
  'scoring.scoresPlaceholder': 'Anna:120,Bertta:80,…',
  'scoring.saveRound': 'Tallenna kierros',
  'scoring.export': 'Vie pistelappu',
  'scoring.formatHint': 'Pisteiden muoto: Nimi:100,Nimi2:50',
  'scoring.nameCountError': 'Anna 2–8 pelaajan nimeä pilkuilla eroteltuina',
  'scoring.session': 'Istunto: {id}',
  'scoring.players': 'Pelaajat: {names}',
  'scoring.roundScores': 'Kierroksen {n} pisteet',
  'stats.loading': 'Ladataan…',
  // The figures could not be fetched. `{reason}` is whatever the server or the
  // network said, which is not ours to translate — the frame around it is.
  'stats.unavailable': '(ei saatavilla: {reason})',

  // --- statistics -----------------------------------------------------------
  'stats.title': 'Tilastot ja tulostaulu',
  'stats.yours': 'Sinun tilastosi',
  'stats.leaderboard': 'Tulostaulu',

  // --- a player's lifetime record, shown beside a finished match ------------
  'record.title': 'Sinun saldosi',
  'record.guest':
    'Pelaat vieraana, joten saldoa ei pidetä. Kirjaudu sisään, niin tällä laitteella jo pelaamasi pelit — tämä mukaan lukien — liitetään tiliisi.',
  'record.signInToKeep': 'Kirjaudu sisään ja säilytä ne',
  'record.failed': 'Saldoasi ei juuri nyt saatu ladattua. Ottelu on tallessa.',
  'record.loading': 'Ladataan…',
  'record.played': 'Pelatut',
  'record.won': 'Voitot',
  'record.lost': 'Häviöt',
  'record.winRate': 'Voittoprosentti',
  'record.streak': 'Putki',
  'record.atThisGame': 'Tässä pelissä',
  // A streak is a count with a noun, so it is a whole phrase per count for
  // the same reason `countLabel` is: "1 win" / "2 wins" is an English rule,
  // and most of these languages do not share it.
  'record.streakNone': '—',
  'record.streakWinOne': '1 voitto',
  'record.streakWinMany': '{n} voittoa',
  'record.streakLossOne': '1 häviö',
  'record.streakLossMany': '{n} häviötä',

  // --- moving cards ---------------------------------------------------------
  'hand.dragHint': 'Raahaa korttia viuhkaa pitkin järjestääksesi sen uudelleen, tai pöydälle pelataksesi sen',
  'hand.moveLeft': 'Vasemmalle',
  'hand.moveRight': 'Oikealle',
  'zone.collapseGroup': 'Tiivistä tämä ryhmä',
  'zone.expandGroup': 'Näytä kaikki tämän ryhmän kortit',
  'zone.dropHere': 'Pudota tähän',
  'offer.pickCards': 'valitse kortit koskettamallesi paikalle',
  'offer.ambiguous': 'tämä sopii useampaan paikkaan — valitse pöydältä',

  // --- the build footer -----------------------------------------------------
  'build.app': 'sovellus',
  'build.server': 'palvelin',
  'about.subtitle': 'Versio, jota pelaat, ja pienellä painettu teksti.',
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
  'option.pauseBetweenRounds': 'Tauko kierrosten välissä',
  'choice.pauseBetweenRounds.1': 'Tauko',
  'choice.pauseBetweenRounds.0': 'Jatka suoraan',
  'option.botSkill': 'Vastustajat',
  'choice.botSkill.0': 'Sekalaiset',
  'choice.botSkill.1': 'Helppo',
  'choice.botSkill.2': 'Keskitaso',
  'choice.botSkill.3': 'Vaikea',
  'option.initialMeldMinimum': 'Avausarvo',
  'choice.initialMeldMinimum.0': 'Ei mitään',
  'option.discardDrawMinRound': 'Nosto poistopinosta',
  'choice.discardDrawMinRound.0': 'Auki',
  'choice.discardDrawMinRound.2': 'Kierroksesta 2',
  'choice.discardDrawMinRound.3': 'Kierroksesta 3',
  'option.requireCleanRun': 'Jokeriton suora',
  'choice.requireCleanRun.1': 'Vaaditaan',
  'choice.requireCleanRun.0': 'Ei',
  'option.jokerReclaimMustPlay': 'Lunastettu jokeri',
  'choice.jokerReclaimMustPlay.1': 'Pelattava samalla vuorolla',
  'choice.jokerReclaimMustPlay.0': 'Saa jäädä käteen',
  'option.dealStarter': 'Aloittaja',
  'choice.dealStarter.0': 'Vuorotellen',
  'choice.dealStarter.1': 'Voittaja aloittaa',
  'variation.prsi.classic': 'Klassinen',
  'option.handSize': 'Jaetut kortit',
  'variation.canasta.classic': 'Klassinen',
  'variation.canasta.modern_american': 'Modern American',
  'variation.canasta.samba': 'Samba',
  'option.targetScore': 'Tavoitepisteet',
  'option.canastasToGoOut': 'Canastat ulospääsyyn',
  'variation.holdem.freezeout': 'Freezeout',
  'variation.holdem.timed': 'Kiinteä määrä jakoja',
  'option.startingStack': 'Aloituspelimerkit',
  'option.bigBlind': 'Iso blindi',
  'option.handLimit': 'Jaot',
  'choice.handLimit.0': 'Kunnes yksi paikka on jäljellä',
  'variation.ginrummy.standard': 'Vakio',
  'option.knockLimit': 'Koputusraja',
  'choice.knockLimit.0': 'Oklahoma (avattu kortti määrää sen)',
  'option.bigGin': 'Big gin',
  'choice.bigGin.0': 'Pois',
  'choice.bigGin.1': 'Päällä (+25)',
  'option.lineBonuses': 'Loppulaskennan bonukset',
  'choice.lineBonuses.1': 'Päällä',
  'choice.lineBonuses.0': 'Pois',
  'variation.rummytiles.standard': 'Vakio',
  'choice.targetScore.0': 'Ei mitään',
  // Two games qualify a number the others mean plainly: Canasta's 500 is a
  // short game, Gin Rummy's and Rummy Tiles' 500 is just 500; Hold'em's
  // starting 200 is a short stack, Blackjack's 200 is just 200. Module-scoped
  // so the qualifier travels with the game that means it — a bare key would
  // hand the note to games it is false for. `scripts/check-server-labels.js`
  // is what finds these; both were shipping as English on a Czech screen.
  'choice.canasta.targetScore.500': '500 (lyhyt)',
  'choice.holdem.startingStack.200': '200 (lyhyt)',
  'option.roundLimit': 'Kierrosraja',
  'choice.roundLimit.0': 'Ei mitään',
  'option.poolExhaustion': 'Jos pussi tyhjenee',
  'choice.poolExhaustion.1': 'Matalin käsi voittaa kierroksen',
  'choice.poolExhaustion.0': 'Kukaan ei voita kierrosta',
  'variation.blackjack.single': 'Yksi pakka',
  'option.minBet': 'Pöydän minimi',
  'option.rounds': 'Kierrokset',
  'option.decks': 'Pakat',
  'option.dealerHitsSoft17': 'Jakaja pehmeällä 17:llä',
  'choice.dealerHitsSoft17.0': 'Jää',
  'choice.dealerHitsSoft17.1': 'Ottaa',
  'option.blackjackPays': 'Blackjack maksaa',
  'choice.blackjackPays.100': 'Yksi yhteen',
  'option.maxSplits': 'Jakaminen',
  'choice.maxSplits.0': 'Ei jakamista',
  'choice.maxSplits.1': 'Kerran (kaksi kättä)',
  'choice.maxSplits.3': 'Kolmesti (neljä kättä)',
  'option.doubleAfterSplit': 'Tuplaus jaon jälkeen',
  'choice.doubleAfterSplit.1': 'Sallittu',
  'choice.doubleAfterSplit.0': 'Ei sallittu',
  'option.surrender': 'Luovutus',
  'choice.surrender.0': 'Pois',
  'choice.surrender.1': 'Myöhäinen luovutus',
  'option.insurance': 'Vakuutus',
  'choice.insurance.1': 'Tarjotaan',
  'choice.insurance.0': 'Ei tarjota',

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
  'verb.add': 'Lisää',
  'verb.bet': 'Panosta',
  'verb.call': 'Maksan',
  'verb.check': 'Passaan',
  'verb.commit': 'Valmis',
  'verb.continue': 'Jatka',
  'verb.decline_insurance': 'Ei vakuutusta',
  'verb.discard': 'Poista',
  'verb.double': 'Tuplaa',
  'verb.draw': 'Nosta',
  'verb.finish_layoff': 'Liittäminen valmis',
  'verb.fold': 'Luovutan',
  'verb.hit': 'Kortti',
  'verb.insure': 'Ota vakuutus',
  'verb.knock': 'Koputa',
  'verb.lay_meld': 'Laske',
  'verb.lay_off': 'Liitä',
  'verb.pass': 'Passaa',
  'verb.place': 'Aseta',
  'verb.play_card': 'Pelaa',
  'verb.raise': 'Korotan',
  'verb.reset_turn': 'Nollaa vuoro',
  'verb.split': 'Jaa',
  'verb.stand': 'Jään',
  'verb.surrender': 'Luovu',
  'verb.swap_joker': 'Vaihda jokeri',
  'verb.take': 'Ota',
  'verb.take_pile': 'Ota pinosta',
  // Declared by the server and listed in `serverKeys.json`, but never worded.
  'verb.takePileFromHand': 'Ota pino käteen',
  'verb.takePileOntoMeld': 'Ota pino yhdistelmään',
  'verb.takeTopForSequence': 'Ota päällimmäinen kortti jonoon',
  'verb.undoDraw': 'Kumoa nosto',
  'verb.undoLayOff': 'Kumoa liittäminen',
  'verb.undoMeld': 'Kumoa yhdistelmä',
  'verb.undoTakePile': 'Kumoa pinon otto',
  'verb.undoTurn': 'Kumoa vuoro',

  // --- suits, spelled out ---------------------------------------------------
  //
  // Read aloud by a screen reader and shown where a pip would not fit. The
  // pips themselves are drawn, not written, so these are the only place the
  // suit is ever a word.
  'suit.C': 'Risti',
  'suit.D': 'Ruutu',
  'suit.H': 'Hertta',
  'suit.S': 'Pata',

  // --- counters the board prints beside a number ----------------------------
  'canasta.seat.notOpened': 'Ei avattu',
  'canasta.unit.points': 'pistettä',
  'ginrummy.unit.points': 'pistettä',
  'holdem.seat.dealer': 'Jakaja',
  'holdem.seat.folded': 'Luovutti',
  'holdem.seat.allIn': 'All in',
  'holdem.seat.out': 'Ulkona',
  'holdem.unit.chips': 'pelimerkkiä',
  'prsi.unit.cardsLeft': 'korttia jäljellä',
  'rummytiles.prompt.initialMeld': 'Ensimmäisen laskusi on oltava {n} pisteen arvoinen.',
  'rummytiles.unit.points': 'pistettä',
  'zolik.unit.penalty': 'rangaistus',
  'header.pileFrozen': 'Pino jäädytetty',
  // The module-less form of `ginrummy.prompt.yourTurnDraw`: Žolíky and Canasta
  // send the prompt without a module prefix. Neither the Go constants nor
  // `serverKeys.json` list it — the sweep is what found it.
  'prompt.yourTurnDraw': 'Nosta kortti',
  'prompt.yourTurnMeld': 'Yhdistä jos voit, sitten poista',
};
