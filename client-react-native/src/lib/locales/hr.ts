/**
 * Croatian. Remi vocabulary: grupa for a set, niz for a run, kombinacija for a meld, špil for the stock, džoker for a joker.
 */

export const hr: Record<string, string> = {
  // --- engine error codes (rules.RulesErrorCode) ---------------------------
  'err.NOT_YOUR_TURN': 'Nisi ti na redu',
  'err.WRONG_PHASE': 'Trenutno nije moguće',
  'err.MUST_DRAW_FIRST': 'Vuci kartu prije nego što spustiš',
  'err.GAME_SUSPENDED': 'Igra je pauzirana',
  'err.GAME_NOT_ACTIVE': 'Igra nije u tijeku',
  // --- runtime refusals, which are not the rules saying no -----------------
  //
  // Both were reachable long before they were worded, and a player who met
  // one was shown the code in capitals or, worse, nothing at all: an action
  // sent while the socket was down simply vanished, which reads as a legal
  // move being silently refused.
  'err.MATCH_NOT_ACTIVE': 'Stol je pauziran — čeka se da se igrač ponovno spoji',
  'err.NOT_CONNECTED': 'Nema veze sa stolom — ponovno se spajamo, zatim pokušaj opet',
  // Why the "start the next round" control is greyed out. Without wording the
  // shell printed the code itself, so a player agreeing to go on was answered
  // with ALREADY_READY in capitals.
  'err.ALREADY_READY': 'Spreman si',
  'err.NOT_BETWEEN_ROUNDS': 'Runda još traje',
  'err.NOT_AT_THIS_TABLE': 'Nisi za ovim stolom',
  'err.DISCARD_LOCKED': 'Hrpa odbačenih je zasad zaključana',
  'err.DISCARD_PILE_EMPTY': 'Hrpa odbačenih je prazna',
  'err.NO_CARDS_LEFT': 'Nema više karata za vučenje',
  'err.ROUND_REQ_NOT_MET': 'Prvo spusti vlastito otvaranje',
  'err.NEED_CLEAN_RUN': 'Treba ti niz bez džokera na stolu da bi se računao kao spušten',
  'err.INCOMPLETE_INITIAL_MELD': 'Dovrši spuštanje ili ga poništi prije nego što odbaciš',
  'err.DISCARD_CARD_NOT_MELDED': 'Karta koju si uzeo mora ući u tvoju kombinaciju',
  'err.JOKER_DISCARD_FORBIDDEN': 'Džoker se ne može odbaciti',
  'err.NOTHING_TO_UNDO': 'Nema se što poništiti',
  'err.NO_JOKER_IN_MELD': 'U ovoj kombinaciji nema džokera',
  'err.JOKER_SWAP_MISMATCH': 'Ta karta ne zauzima mjesto džokera',
  'err.RECLAIMED_JOKER_NOT_MELDED': 'Džoker uzet sa stola mora se odigrati u kombinaciju u ovom potezu',
  'err.RUN_TOO_LONG': 'Taj niz već ima punu duljinu',
  'err.WRONG_RUN_END': 'Ta karta produžuje drugi kraj niza',
  'err.INVALID_MELD': 'Nijedna karta u tvojoj ruci ovdje ne odgovara',
  'err.CARD_NOT_IN_HAND': 'Ta karta nije u tvojoj ruci',
  'err.MELD_BELOW_MINIMUM': 'Tvojim kombinacijama još nedostaju bodovi za spuštanje',
  'err.MELD_NO_CONTRIBUTION': 'Ta kombinacija ne pomiče tvoj zahtjev naprijed',
  'err.TOO_MANY_WILDS': 'Previše džokera u toj kombinaciji',
  'err.ADJACENT_WILDS': 'Dva džokera ne smiju stajati jedan uz drugi',
  'err.ACE_BRIDGE': 'As ne može povezati kralja i dvojku',

  // --- contract phrasing ---------------------------------------------------
  // Built from the structured contract the server sends (sets/runs counts),
  // never from a deal-number lookup table.
  // Whole phrases per count rather than a number plus a pluralised noun:
  // Czech inflects the noun by count (skupina / skupiny / skupin) in a way no
  // "add an s" helper survives, and the counts that actually occur are 1-3.
  // A count outside that falls back to contract.sets.n.
  'contract.sets.1': 'Jedna grupa',
  'contract.sets.2': 'Dvije grupe',
  'contract.sets.3': 'Tri grupe',
  'contract.sets.n': 'Grupe: {n}',
  'contract.runs.1': 'Jedan niz',
  'contract.runs.2': 'Dva niza',
  'contract.runs.3': 'Tri niza',
  'contract.runs.n': 'Nizovi: {n}',
  'contract.any': 'Bilo koja valjana kombinacija',
  'contract.cleanRunOnly': 'Bilo koja mješavina grupa i nizova — barem jedan niz mora biti bez džokera',
  'contract.cleanRunSuffix': '{base} — jedan niz mora biti bez džokera',

  // --- written rules ---------------------------------------------------------
  // Full-sentence keys a "see the rules" screen renders, one per module,
  // resolved server-side against the table's actual variation and options —
  // see internal/module/rules.go and each game's rules.go. Shared section
  // titles first, then one block per module in the order the four games were
  // added.
  'zolik.rules.section.goal': 'Cilj',
  'zolik.rules.section.setup': 'Priprema',
  'zolik.rules.section.turn': 'Tvoj potez',
  'zolik.rules.section.melding': 'Spuštanje',
  'zolik.rules.section.end': 'Kako meč završava',
  'zolik.rules.goal':
    'Budi prvi koji isprazni ruku spuštajući valjane grupe i nizove, skupljajući pritom što manje kaznenih bodova u kartama koje još držiš kad netko drugi izađe.',
  'zolik.rules.deal': 'Svaki igrač dobiva {n} karata.',
  'zolik.rules.meldShapes':
    'Grupa je {set}+ karata iste vrijednosti; niz je {run}+ uzastopnih karata iste boje.',
  'zolik.rules.turn.draw': 'U svom potezu vuci jednu kartu — iz špila ili s hrpe odbačenih.',
  'zolik.rules.pickup.topOnly': 'S hrpe odbačenih smije se uzeti samo gornja karta.',
  'zolik.rules.pickup.anyFromPile':
    'S hrpe odbačenih smije se uzeti bilo koja karta, zajedno sa svime što je iznad nje.',
  'zolik.rules.pickup.locked': 'S hrpe odbačenih ne smije se vući prije runde {n}.',
  'zolik.rules.pickup.open': 'Hrpa odbačenih otvorena je od prve runde.',
  'zolik.rules.turn.discard': 'Završi svoj potez odbacivanjem jedne karte.',
  'zolik.rules.jokers.restricted':
    'Džoker se nikada ne smije odbaciti, osim kao točno ona karta koja ti prazni ruku.',
  'zolik.rules.lead.rotate':
    'Prvi potez pomiče se za jedno mjesto pri svakom dijeljenju, bez obzira na to tko je pobijedio.',
  'zolik.rules.lead.winner': 'Tko izađe, otvara sljedeće dijeljenje.',
  'zolik.rules.meldFloor.on':
    'Tvoje prvo spuštanje mora dati najmanje {n} prirodnih bodova da bi bio spušten.',
  'zolik.rules.meldFloor.off': 'Za prvo spuštanje nema najmanje bodovne vrijednosti.',
  'zolik.rules.cleanRun.on':
    'Barem jedan tvoj niz mora biti potpuno bez džokera da bi se računao kao spušten.',
  'zolik.rules.cleanRun.off':
    'Tvoji nizovi smiju slobodno koristiti džokere — nijedan ne mora biti bez njih.',
  'zolik.rules.contracts.rotating':
    'Meč traje {n} dijeljenja, a svako dijeljenje traži vlastitu kombinaciju grupa i nizova.',
  'zolik.rules.contracts.static': 'Svako dijeljenje traži istu kombinaciju: {sets} grupa i {runs} nizova.',
  'zolik.rules.end.afterDeals': 'Meč završava nakon {n} dijeljenja.',
  'zolik.rules.end.atScore': 'Dijeli se dalje dok netko ne dosegne {n} bodova — tada je gotovo.',

  'prsi.rules.section.goal': 'Cilj',
  'prsi.rules.section.setup': 'Priprema',
  'prsi.rules.section.turn': 'Tvoj potez',
  'prsi.rules.section.special': 'Posebne karte',
  'prsi.rules.section.end': 'Kako meč završava',
  'prsi.rules.goal': 'Budi prvi koji odigra svaku kartu iz ruke.',
  'prsi.rules.deck': 'Igra se špilom od {value} karata (od sedmice naviše).',
  'prsi.rules.deal': 'Svaki igrač počinje s {n} karata.',
  'prsi.rules.turn.match':
    'Odigraj kartu koja odgovara boji ili vrijednosti gornje karte — ili vuci ako ne možeš.',
  'prsi.rules.turn.draw': 'Vučenje završava tvoj potez bez odigravanja.',
  'prsi.rules.sevens': 'Odigraj 7 i sljedeći igrač vuče dvije karte, osim ako odgovori vlastitom sedmicom.',
  'prsi.rules.aces': 'Odigraj asa i potez sljedećeg igrača se preskače.',
  'prsi.rules.queens': 'Odigraj damu i reci boju koja se nastavlja.',
  'prsi.rules.end': 'Meč završava u trenutku kad je nečija ruka prazna.',

  'canasta.rules.section.goal': 'Cilj',
  'canasta.rules.section.setup': 'Priprema',
  'canasta.rules.section.melding': 'Spuštanje',
  'canasta.rules.section.end': 'Kako meč završava',
  'canasta.rules.goal': 'Igra se u parovima; prva strana koja dosegne {n} bodova pobjeđuje u meču.',
  'canasta.rules.deck': 'Igra se s {value} karata — dva špila plus džokeri.',
  'canasta.rules.deal': 'Svaki igrač dobiva {n} karata.',
  'canasta.rules.redThrees':
    'Crvena trojka u ruci odmah se pokazuje i broji se kao bonus — osim ako tvoja strana nikad ne dovrši canastu, tada se broji protiv tebe.',
  'canasta.rules.canasta': 'Canasta je kombinacija od {n} ili više karata iste vrijednosti.',
  'canasta.rules.meldFloorBands':
    'Tvoje prvo spuštanje mora doseći bodovni minimum koji raste s tvojim rezultatom: {negative} ispod nule, {low} do 1500, {mid} do 3000, {high} iznad toga.',
  'canasta.rules.oneCanastaToGoOut': 'Jedna dovršena canasta dovoljna je da tvoja strana izađe.',
  'canasta.rules.twoCanastasToGoOut':
    'Tvojoj strani trebaju dvije dovršene canaste prije nego što smije izaći.',
  'canasta.rules.end': 'Dijeli se dalje dok jedna strana ne prijeđe {n} bodova — tada je meč gotov.',

  'holdem.rules.section.goal': 'Cilj',
  'holdem.rules.section.setup': 'Priprema',
  'holdem.rules.section.betting': 'Ulaganje',
  'holdem.rules.section.end': 'Kako meč završava',
  'holdem.rules.goal':
    'Osvajaj žetone najboljom rukom pri otkrivanju karata ili tako da ostaneš jedini igrač u dijeljenju.',
  'holdem.rules.stack': 'Svako mjesto počinje s {n} žetona.',
  'holdem.rules.blinds': 'Mali blind je {sb}, a veliki {bb}, ulažu se prije dijeljenja karata.',
  'holdem.rules.streets': 'Ulaže se u četiri kruga — prije flopa te nakon flopa, turna i rivera.',
  'holdem.rules.showdown': 'Oni koji su još u igri otkrivaju karte; najbolja ruka od pet karata uzima pot.',
  'holdem.rules.noLimit': 'Bez limita — svaki ulog može ići do cijelog tvog stacka.',
  'holdem.rules.lastPlayerStanding': 'Igra se dok jedno mjesto ne drži sve žetone.',
  'holdem.rules.mostChipsWins': 'Tko na kraju igre ima najviše žetona, pobjeđuje u meču.',
  'holdem.rules.handLimit': 'Igra se zaustavlja nakon {n} dijeljenja.',

  // --- header --------------------------------------------------------------
  'header.deal': 'Dijeljenje {n}',
  'header.gameOf': 'Igra {n} od {total}',
  'header.gameOfWithContract': 'Igra {n} od {total}: {contract}',

  // --- meld preview --------------------------------------------------------
  'preview.validSet': 'Valjana grupa',
  'preview.validRun': 'Valjan niz',
  'preview.validMeld': 'Valjana kombinacija',
  'preview.notYet': 'Još nije kombinacija',
  'preview.points': '{shape} · {n} bodova',
  'preview.pointsWithLaid': '{shape} · {n} + {laid} već spušteno = {total} bodova',
  'preview.meetsFloor': '{line} (doseže {n} ✓)',
  'preview.needsFloor': '{line} (treba {n} ✗)',
  'preview.becauseOf': '{line} — {reason}',

  // --- discard -------------------------------------------------------------
  // A discard with a meld staged lays that meld first; this is what the
  // player is told when the server refuses it and the whole move is rolled
  // back. {reason} is the engine's own words for the refusal.
  'discard.meldRejected': '{reason} — ništa nije odbačeno, tvoje karte i dalje čekaju.',

  // --- what the current selection would send -------------------------------
  // The client's own reason a control is not ready, next to the engine's own
  // `err.*` reasons rather than a second kind of message — a control greyed
  // out for "not your turn" and one greyed out for "you picked two, this
  // takes one" should read as the same kind of thing, not one of them looking
  // broken. See `fits` in `src/lib/drops.ts`.
  'sel.tooMany.1': 'Odaberi samo jednu kartu',
  'sel.tooMany.n': 'Odaberi najviše {n} karata',
  'sel.needMore': 'Odaberi karte: {n}',
  'sel.notThese': 'Te karte ne mogu ovdje',
  'sel.needsCompany': 'Toj karti trebaju one pored nje',

  // --- what a module says happened -----------------------------------------
  // The first module-sent keys in this bundle, and deliberately few. Anything
  // a module sends is still legible without an entry here — `humanise` turns
  // `holdem.seat.stack` into "Stack" — so a key earns a line only when that
  // fallback loses something. These do: what each of them means lives in its
  // *params* (who won, how much, whose cards), and a key on its own has
  // nowhere to put them. Without the entry the player read "Winner", full
  // stop, at the end of a match they had just won.
  'status.winner': 'Pobijedio {winners}',
  'holdem.status.pot': '{winners} osvaja {amount} s {hand}',
  'holdem.status.potUncontested': '{winners} osvaja {amount} — svi ostali su odustali',
  'holdem.status.shown': '{playerId} je pokazao {value}',
  'holdem.prompt.waitingFor': 'Čeka se {playerId}',

  // How a rummy scoreboard was ordered. Both earn a line for the same reason
  // as the keys above: the number lives in the params, and `humanise` would
  // render "Deals Won" with nothing after it.
  'zolik.standing.dealsWon': 'Dobivena dijeljenja: {n}',
  'zolik.standing.inHand': 'U ruci: {n}',

  // --- rounds ---------------------------------------------------------------
  // What a round is called, and what one did. Each of these carries its number
  // in the params, so `humanise` alone would render a label with nothing after
  // it.
  // An instruction, not a heading. "Next round" named the thing rather than
  // asking for it, and sat above "Waiting for 1" — which reads as though the
  // player is the one waiting, when the table is waiting for them.
  'round.continue': 'Pokreni sljedeću rundu',

  // --- the flash that announces a round or a match ending ------------------
  //
  // One line and one number, held for about two seconds. Worded generically on
  // purpose: which *kind* of ending it was is the module's own fact, printed
  // underneath, and a client that phrased "went gin" itself would be a client
  // that knows a game.
  'flash.roundWon': '{winners} ju je uzeo',
  'flash.roundWonYou': 'Ti si je uzeo',
  'flash.roundDrawn': 'Nitko je nije uzeo',
  'flash.matchOver': 'Meč je gotov',
  'flash.matchWon': '{winners} pobjeđuje',
  'flash.matchWonYou': 'Pobjeđuješ',
  'flash.matchDrawn': 'Nitko nije pobijedio',
  'flash.nowOn': 'sada {total}',

  'zolik.round.deal': 'Dijeljenje',
  'zolik.round.cleanRun': 'Jedan niz mora biti bez džokera',
  'canasta.round.deal': 'Dijeljenje',
  'canasta.round.concealed': 'Izašao skriveno',
  'canasta.round.exhausted': 'Špil je potrošen',
  'canasta.round.meldCards': 'Spuštene karte: {n}',
  'canasta.round.canastas': 'Canaste: {n}',
  'canasta.round.redThrees': 'Crvene trojke: {n}',
  'canasta.round.goingOut': 'Izlazak: {n}',
  'canasta.round.inHand': 'Ostalo u ruci: {n}',
  'holdem.round.hand': 'Dijeljenje',
  'holdem.round.pot': 'Pot {n}',
  'holdem.round.uncontested': 'Svi ostali su odustali',
  'seat.ready': 'Spreman',
  'results.you': '(ti)',



  // --- refusals the other three games raise ---------------------------------
  // Worded here for the same reason Žolíky's are: reasonText falls back to the
  // code, and OfferBar hands it the code as that fallback, so an unworded
  // refusal reaches a player as PILE_FROZEN in capitals. serverKeys.test.ts
  // now fails the build rather than letting the next one through.
  'err.SET_TOO_LARGE': 'Grupa već ima sve četiri boje',
  'err.DISCARD_TAKEN_CARD_FORBIDDEN':
    'Ne možeš odbaciti kartu koju si upravo uzeo — odigraj je ili je zadrži',
  'err.CARD_DOES_NOT_FIT': 'Ta karta ne odgovara ni po boji ni po vrijednosti',
  'err.SUIT_REQUIRED': 'Reci boju koja se nastavlja',
  'err.MUST_ANSWER_DRAW_OR_TAKE': 'Odgovori sedmicom ili uzmi karte',
  'err.NOTHING_TO_DRAW': 'Nije ostalo ništa za vučenje',
  'err.PILE_EMPTY': 'Hrpa je prazna',
  'err.PILE_BLOCKED': 'Hrpa je blokirana — na vrhu je crna trojka',
  'err.PILE_FROZEN': 'Hrpa je zamrznuta — trebaju ti dvije prirodne karte vrijednosti gornje karte',
  'err.TOP_CARD_UNUSABLE': 'Ne možeš upotrijebiti gornju kartu',
  'err.MELD_CLOSED': 'Ta kombinacija je potpuna i zatvorena',
  'err.MELD_TOO_SMALL': 'Kombinaciji treba više karata od toga',
  'err.MELD_TOO_LARGE': 'Ta kombinacija ne može primiti više karata',
  'err.MELD_MIXED_RANKS': 'Sve karte u kombinaciji moraju biti iste vrijednosti',
  'err.NOT_ENOUGH_NATURALS': 'Kombinaciji treba više prirodnih karata nego džokera',
  'err.RANK_ALREADY_MELDED': 'Tvoja strana već ima kombinaciju te vrijednosti',
  'err.NOT_YOUR_MELD': 'Ta kombinacija pripada protivničkoj strani',
  'err.NO_SUCH_MELD': 'Ta kombinacija nije na stolu',
  'err.CANNOT_MELD_THREE': 'Trojke se nikad ne spuštaju',
  'err.CANNOT_DISCARD_RED_THREE': 'Crvena trojka se ne može odbaciti',
  'err.MUST_KEEP_A_CARD': 'Zadrži barem jednu kartu — tako ne možeš isprazniti ruku',
  'err.MUST_MELD_FIRST': 'Prvo spusti otvaranje svoje strane',
  'err.INITIAL_MELD_NOT_MET': 'Tvojem prvom spuštanju još nedostaju bodovi',
  'err.CANNOT_GO_OUT_YET': 'Tvojoj strani treba dovršena canasta prije nego što može izaći',
  'err.NOTHING_TO_CALL': 'Nema uloga za praćenje',
  'err.CANNOT_CHECK': 'Ne možeš čekirati — postoji ulog na koji treba odgovoriti',
  'err.CANNOT_RAISE': 'Ovdje ne možeš podizati',
  'err.RAISE_TOO_SMALL': 'Podizanje mora biti barem koliko i prethodno',
  'err.NOT_ENOUGH_CHIPS': 'Nemaš toliko žetona',
  'err.AMOUNT_REQUIRED': 'Reci koliko',
  'err.AMOUNT_NOT_A_NUMBER': 'Taj iznos nije broj',
  'err.SEAT_NOT_IN_HAND': 'Nisi u ovom dijeljenju',
  'err.WRONG_RANK': 'Ta karta je za ovo krive vrijednosti',
  'err.MATCH_FULL': 'Stol je pun',
  'err.MATCH_ALREADY_STARTED': 'Meč je već počeo',
  'err.TOO_FEW_PLAYERS': 'Još nema dovoljno igrača',
  'err.WRONG_PLAYER_COUNT': 'Ova se igra ne može igrati s toliko igrača',
  'err.NOT_THE_HOST': 'To može samo domaćin',
  'err.NO_LONGER_WAITING': 'Stol više ne čeka',
  'err.WAITING_ROOM_UNAVAILABLE': 'Čekaonica nije dostupna',
  'err.SERVER_BUSY': 'Poslužitelj je trenutno pun — pokušaj ponovno za koji trenutak',


  // --- rules the engine has always enforced and never stated ----------------
  // Every one of these was found by the guardrail rather than by review: the
  // validator could refuse a player for it, and no sentence anywhere said so.
  // See server/internal/zolikmod/ruleindex_test.go.
  'zolik.rules.section.layoff': 'Dodavanje kombinacijama',
  'zolik.rules.pickup.obligation':
    'Dok nisi spušten, karta uzeta s hrpe odbačenih mora se upotrijebiti u kombinaciji kojom se u ovom potezu spuštaš.',
  'zolik.rules.pickup.noReturn':
    'Karta uzeta s hrpe odbačenih ne smije se ponovno odbaciti u istom potezu — odigraj je ili je zadrži.',
  'zolik.rules.wilds.setLimit': 'Grupa ne smije sadržavati više džokera nego prirodnih karata.',
  'zolik.rules.set.maxSize':
    'Grupa ne smije imati više od {n} karata — džoker nadomješta boju koja nedostaje, ne dopunjuje potpunu grupu.',
  'zolik.rules.run.maxLength':
    'Niz ne smije imati više od {n} karata — as dolje, dvanaest vrijednosti iznad njega i as gore.',
  'zolik.rules.run.aceBridge':
    'As stoji iznad kralja ili ispod dvojke, nikad kao most između dva kraja niza.',
  'zolik.rules.contracts.contribution':
    'Dok nisi spušten, svaka kombinacija koju spustiš mora biti ona koju ugovor dijeljenja još traži.',
  'zolik.rules.layoff.afterDown': 'Tuđim kombinacijama ne smiješ dodavati dok ne spustiš vlastiti ugovor.',
  'zolik.rules.layoff.runEnds': 'Karta dodana nizu mora ga nastaviti na jednom ili drugom kraju.',
  'zolik.rules.jokers.swap':
    'Džoker u kombinaciji na stolu može se otkupiti točno onom kartom koju predstavlja.',
  'zolik.rules.jokers.reclaim.on':
    'Džoker otkupljen sa stola mora se odigrati u kombinaciju u istom potezu — ne smije ostati u ruci.',
  'zolik.rules.jokers.reclaim.off': 'Džoker otkupljen sa stola smije ostati u ruci.',
  'zolik.rules.deck.reshuffle':
    'Kad se špil potroši, hrpa odbačenih se izmiješa i postaje novi špil; ako su oba prazna, dijeljenje završava.',


  // --- what to do instead ---------------------------------------------------
  // The third layer of a refusal, after the reason and the rule. Sent by the
  // module because it is the only side that knows which card is owed and
  // which way out is on offer; see server/internal/zolikmod/remedy.go.
  'zolik.remedy.meldThePickup': 'Dodaj {card} svom spuštanju ili poništi uzimanje.',
  'zolik.remedy.discardSomethingElse': 'Odbaci drugu kartu ili odigraj {card} u ovom potezu.',
  'zolik.remedy.discardNotAJoker': 'Odbaci nešto drugo, a ne džokera.',
  'zolik.remedy.finishOrUndoLayDown': 'Dovrši spuštanje ili ga povuci natrag.',
  'zolik.remedy.needMorePoints': 'Treba ti još {n} bodova da bi se mogao spustiti.',
  'zolik.remedy.layACleanRun': 'Spusti niz bez džokera u njemu.',
  'zolik.remedy.playReclaimedJoker': 'Odigraj {card} u kombinaciju ili poništi uzimanje.',
  'zolik.remedy.goDownFirst': 'Prvo spusti vlastite kombinacije.',
  'zolik.remedy.drawFirst': 'Prvo vuci kartu.',
  'zolik.remedy.drawFromStock': 'Vuci iz špila — hrpa odbačenih otvara se u rundi {n}.',
  'zolik.remedy.drawFromStockEmpty': 'Vuci radije iz špila.',


  // --- keys whose meaning lives in their params -----------------------------
  // The shape fallback renders these with the number missing ("Stack", "Pot"),
  // which is why they earn a line where most keys do not.
  'header.contract': 'Traži {sets} grupa i {runs} nizova',
  'header.contract.cleanRunOnly': 'Traži niz bez džokera',
  'header.round': 'Runda {n}',
  'header.deck': 'Špil',
  'header.target': 'Cilj',
  'header.suitInPlay': 'Boja u igri',
  'seat.cards': 'Karte',
  'zolik.offer.meld': 'Spusti',
  'prompt.pickupMustBeMelded':
    '{value} je došla s hrpe odbačenih — mora ući u kombinacije kojima se u ovom potezu spuštaš.',
  'prompt.jokerMustBePlayed':
    '{value} je došla sa stola — mora ući u kombinaciju prije nego što možeš završiti potez.',
  'prompt.initialMeld': 'Otvaranje tvoje strane mora doseći {n} bodova.',
  'prompt.canastasNeeded': 'Tvojoj strani treba još {n} canasta prije nego što može izaći.',
  'prompt.mustDrawOrAnswerSeven': 'Odgovori sedmicom ili vuci {n} karata.',
  'prompt.chooseSuit': 'Odaberi boju koja se nastavlja',
  'prompt.skipPending': 'Tvoj potez se preskače',
  'status.lastDeal': 'Ekipa {team} osvojila je {value}',
  'status.teamScore': 'Ekipa {team}: {value}',
  'canasta.offer.rank': 'Vrijednost',
  'canasta.seat.teamScore': 'Rezultat ekipe',
  'canasta.seat.canastas': 'Canaste',
  'holdem.header.pot': 'Pot',
  'holdem.header.street': 'Ulica',
  'holdem.header.hand': 'Dijeljenje',
  'holdem.header.handLimit': 'Dijeljenja ukupno',
  'holdem.header.blinds': 'Blindovi',
  'holdem.cost.call': 'za praćenje',
  'holdem.cost.pot': 'u potu',
  'holdem.seat.stack': 'Stack',
  'holdem.seat.bet': 'Ulog',
  'holdem.prompt.yourAction': 'Ti si na redu',
  'holdem.prompt.raiseTo': 'Podigni na',
  'zone.yourHand': 'Tvoja ruka',
  'zone.opponentHand': 'Protivnikova ruka',
  'zone.drawPile': 'Špil',
  'zone.discardPile': 'Hrpa odbačenih',
  'zone.melds': 'Kombinacije',
  'zone.teamMelds': 'Kombinacije tvoje strane',
  'zone.redThrees': 'Crvene trojke',
  'zone.board': 'Stol',
  'verb.drawFromDeck': 'Vuci',
  'verb.takeFromDiscard': 'Uzmi s hrpe',


  // --- the why sheet's own furniture ---------------------------------------
  // The three layers a refusal is explained in. Labels, not sentences: what
  // goes under each is sent by the server or worded above.
  'why.reason': 'Zašto ne',
  'why.rule': 'Pravilo',
  'why.rules': 'Pravila',
  'why.remedy': 'Što možeš učiniti',
  'why.readTheRules': 'Pročitaj cijela pravila →',
  'why.close': 'Zatvori',
  'why.open': 'zašto',

  // --- marks on a particular card ------------------------------------------
  // A mark is a refusal that has not happened yet: the rule is enforced at the
  // discard, which is the last possible moment to hear about it, so the card
  // says so while there is still a turn left to act on it.
  'zolik.badge.owedToMeld':
    '{card} je došla s hrpe odbačenih — mora ući u kombinacije kojima se u ovom potezu spuštaš.',
  'zolik.badge.jokerOwed':
    '{card} je došla sa stola — mora ući u kombinaciju prije nego što možeš završiti potez.',

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
  'legal.terms': 'Uvjeti',
  // The document's own name, used as the heading above it. Kept apart from
  // `legal.notice.terms`, which is the same words inflected to sit inside a
  // sentence — a heading in the instrumental case reads as a mistake in every
  // Slavic language.
  'legal.terms.title': 'Uvjeti korištenja',
  'legal.privacy.title': 'Obavijest o privatnosti',
  'legal.privacy': 'Privatnost',
  'legal.source': 'Izvorni kod',
  'legal.updated': 'Verzija {version}',
  'legal.draft':
    'Nacrt — još nije na snazi. Ime, država i kontaktna adresa operatera tek trebaju biti popunjeni.',
  'legal.notice.before': 'Igranjem prihvaćaš ',
  'legal.notice.terms': 'uvjete korištenja',
  'legal.notice.between': '. Što se o tebi pohranjuje, opisano je u ',
  'legal.notice.privacy': 'obavijesti o privatnosti',
  'legal.notice.after': '.',
  // --- gin rummy -------------------------------------------------------------
  'err.UPCARD_DECLINED': 'Tu si kartu već propustio',
  'err.DEADWOOD_TOO_HIGH': 'Tvoj deadwood je previsok za kucanje',
  'err.CARD_DOES_NOT_EXTEND_MELD': 'Ta karta ne produžuje ovu kombinaciju',
  'ginrummy.rules.setup': 'Priprema',
  'ginrummy.rules.turn': 'Tvoj potez',
  'ginrummy.rules.melds': 'Kombinacije',
  'ginrummy.rules.knocking': 'Kucanje',
  'ginrummy.rules.bigGin': 'Big gin',
  'ginrummy.rules.layoff': 'Prislanjanje',
  'ginrummy.rules.deadHand': 'Mrtvo dijeljenje',
  'ginrummy.rules.scoring': 'Bodovanje dijeljenja',
  'ginrummy.rules.match': 'Pobjeda u meču',
  'ginrummy.rules.lineBonuses': 'Bonusi u obračunu',
  'ginrummy.rules.deck': 'Igra se špilom od {value} karata.',
  'ginrummy.rules.deal': 'Svaki igrač dobiva {value} karata.',
  'ginrummy.rules.upcard': 'Još se jedna karta okreće licem gore i započinje hrpu odbačenih.',
  'ginrummy.rules.drawDiscard':
    'U svom potezu vuci jednu kartu — iz špila ili s hrpe odbačenih — pa zatim odbaci jednu.',
  'ginrummy.rules.setsAndRuns':
    'Kombinacija je grupa od tri ili četiri karte jedne vrijednosti, ili niz od tri ili više karata iste boje.',
  'ginrummy.rules.aceLow': 'As je uvijek nizak — nema niza od dame do asa.',
  'ginrummy.rules.knockLimit': 'Smiješ kucnuti čim ti deadwood padne na {n} ili manje.',
  'ginrummy.rules.oklahoma': 'Granicu kucanja u ovom dijeljenju određuje vrijednost okrenute karte.',
  'ginrummy.rules.gin': 'Nulti deadwood je gin — najbolje moguće kucanje.',
  'ginrummy.rules.bigGinBonus':
    'Jedanaest karata svih u kombinacijama, bez ijednog odbacivanja, je big gin i vrijedi dodatnih {n} bodova.',
  'ginrummy.rules.layoffDescription':
    'Nakon kucanja koje nije gin, protivnik smije prisloniti vlastiti deadwood uz tvoje kombinacije prije nego što se ruke usporede.',
  'ginrummy.rules.deadHandDescription':
    'Ako špil padne na posljednje dvije karte, a nitko nije kucnuo, dijeljenje je mrtvo — nitko ne bodovi, a isti djelitelj dijeli ponovno.',
  'ginrummy.rules.undercut':
    'Ako protivnikov deadwood nije viši od tvojeg, podrezuje te: upisuje razliku plus {n}.',
  'ginrummy.rules.ginBonus': 'Gin donosi cijelu protivnikovu ruku plus {n}.',
  'ginrummy.rules.target': 'Tko prvi prijeđe {n} bodova nakon završetka dijeljenja, pobjeđuje u meču.',
  'ginrummy.rules.shutout': 'Bonus za meč udvostručuje se na {n} ako poraženi nije osvojio nijedan bod.',
  'ginrummy.rules.box': 'Svako dobiveno dijeljenje vrijedi {n} bodova na kraju meča.',
  'ginrummy.rules.gameBonus': 'Pobjeda u meču vrijedi dodatnih {n} bodova.',
  'ginrummy.fact.deadwood': 'deadwood {value}',
  'ginrummy.fact.discardCard': 'Odbaci {value}',
  'ginrummy.fact.meldCards': 'Na {value}',
  'ginrummy.header.hand': 'Dijeljenje {n}',
  'ginrummy.round.kind': '{value}',
  'ginrummy.round.hand': 'Dijeljenje',
  'ginrummy.seat.score': '{value}',
  'ginrummy.seat.dealer': 'Djelitelj',
  'ginrummy.status.knocked': '{playerId} je kucnuo s deadwoodom {deadwood}',
  'ginrummy.status.gin': '{playerId} je napravio gin',
  'ginrummy.status.lastHand': 'Posljednje dijeljenje: {winner} ({kind}, {delta} bodova)',
  'ginrummy.offer.drawStock': 'Vuci iz špila',
  'ginrummy.offer.drawDiscard': 'Vuci s hrpe odbačenih',
  'ginrummy.offer.takeUpcard': 'Uzmi okrenutu kartu',
  'ginrummy.offer.passUpcard': 'Dalje',
  'ginrummy.offer.discard': 'Odbaci',
  'ginrummy.offer.knock': 'Kucni',
  'ginrummy.offer.gin': 'Gin!',
  'ginrummy.offer.bigGin': 'Big gin!',
  'ginrummy.offer.layOff': 'Prisloni',
  'ginrummy.offer.finishLayoff': 'Gotovo s prislanjanjem',
  'ginrummy.zone.knockerHand': 'Ruka onoga tko je kucnuo',
  'ginrummy.zone.melds': 'Kombinacije',
  'ginrummy.prompt.upcardDecision': 'Uzmi okrenutu kartu ili propusti',
  'ginrummy.prompt.yourTurnDraw': 'Vuci kartu',
  'ginrummy.prompt.yourTurnDiscard': 'Odbaci — ili kucni, ako možeš',
  'ginrummy.prompt.layoff': 'Prisloni deadwood ili završi',

  // --- rummy tiles -------------------------------------------------------------
  'err.TILE_NOT_IN_HAND': 'Ta pločica nije u tvojoj ruci',
  'err.TILE_DOES_NOT_FIT': 'To tamo ne pristaje',
  'err.NO_SUCH_SET': 'Ta kombinacija nije na stolu',
  'err.INITIAL_MELD_ONLY': 'Prije prvog spuštanja smiješ presložiti samo vlastite nove kombinacije',
  'err.TABLE_NOT_VALID': 'Stol još nije valjan',
  'err.TRAY_NOT_EMPTY': 'Još imaš slobodnih pločica za postaviti',
  'err.NOTHING_PLAYED': 'Odigraj barem jednu pločicu prije nego što završiš potez',
  'err.INITIAL_MELD_TOO_LOW': 'Tvoje prvo spuštanje mora vrijediti 30 bodova ili više',
  'err.NOT_A_RUN': 'Samo se niz može razdvojiti',
  'err.BAD_SPLIT_POSITION': 'Na tom mjestu se ovaj niz ne može razdvojiti',
  'err.NO_JOKER_IN_SET': 'U toj kombinaciji nema džokera',
  'err.TILE_JOKER_SWAP_MISMATCH': 'Ta pločica nije ono što džoker predstavlja',
  'rummytiles.rules.setup': 'Priprema',
  'rummytiles.rules.sets': 'Kombinacije',
  'rummytiles.rules.initialMeld': 'Prvo spuštanje',
  'rummytiles.rules.turn': 'Tvoj potez',
  'rummytiles.rules.jokerTaking': 'Uzimanje džokera',
  'rummytiles.rules.ending': 'Završetak runde',
  'rummytiles.rules.poolExhaustion': 'Ako se zaliha potroši',
  'rummytiles.rules.match': 'Pobjeda u meču',
  'rummytiles.rules.tiles': 'Igra se s {value} pločica.',
  'rummytiles.rules.dealCount': 'Svaki igrač dobiva {value} pločica.',
  'rummytiles.rules.group': 'Grupa su tri ili četiri pločice istog broja, svaka druge boje.',
  'rummytiles.rules.run': 'Niz su tri ili više uzastopnih brojeva iste boje.',
  'rummytiles.rules.noWrap': 'Nakon 13 se ne kreće opet od 1.',
  'rummytiles.rules.joker': 'Džoker predstavlja bilo koju pločicu.',
  'rummytiles.rules.initialMeldDescription':
    'Dok ne spustiš {n} ili više bodova u jednom potezu, isključivo iz vlastite ruke, ne smiješ dirati ništa što je već na stolu.',
  'rummytiles.rules.turnDescription':
    'Odigraj barem jednu pločicu iz ruke, slobodno preslaguj stol, i završi tako da svaka kombinacija na stolu bude valjana.',
  'rummytiles.rules.noDiscard':
    'Nema odbacivanja — ako ne možeš dovršiti valjan potez, umjesto toga vučeš jednu pločicu.',
  'rummytiles.rules.jokerTakingDescription':
    'Džoker na stolu smiješ uzeti tako da ga zamijeniš pločicom koju predstavlja, iz svoje ruke — i moraš ga upotrijebiti u kombinaciji prije kraja poteza.',
  'rummytiles.rules.goingOut':
    'Rundu dobiva prvi igrač koji ostane bez pločica. Svi ostali upisuju negativnu vrijednost onoga što im je ostalo; pobjednik upisuje zbroj svih tuđih gubitaka.',
  'rummytiles.rules.poolExhaustionLowestWins':
    'Ako se zaliha potroši i nitko ne može igrati, runda završava i dobiva je najniža vrijednost ruke.',
  'rummytiles.rules.poolExhaustionNoWinner':
    'Ako se zaliha potroši i nitko ne može igrati, runda završava bez pobjednika — svaka se ruka jednostavno boduje.',
  'rummytiles.rules.target': 'Tko prvi prijeđe {n} bodova nakon završetka runde, pobjeđuje u meču.',
  'rummytiles.rules.roundLimit': 'Meč završava nakon {n} rundi — pobjeđuje najviši rezultat.',
  'rummytiles.fact.setCards': '{value}',
  'rummytiles.header.pool': 'Zaliha {n}',
  'rummytiles.header.round': 'Runda {n}',
  'rummytiles.round.kind': '{value}',
  'rummytiles.round.round': 'Runda',
  'rummytiles.seat.score': '{value}',
  'rummytiles.seat.notOpened': 'Nije otvorio',
  'rummytiles.status.lastRound': 'Posljednja runda: {winner} ({kind})',
  'rummytiles.badge.invalid': 'Još nije valjano',
  'rummytiles.zone.pool': 'Zaliha',
  'rummytiles.zone.table': 'Stol',
  'rummytiles.zone.tray': 'Stalak',
  'rummytiles.offer.place': 'Postavi',
  'rummytiles.offer.addFromHand': 'Dodaj',
  'rummytiles.offer.addFromTray': 'Dodaj sa stalka',
  'rummytiles.offer.take': 'Uzmi',
  'rummytiles.offer.split': 'Razdvoji',
  'rummytiles.offer.swapJoker': 'Zamijeni džokera',
  'rummytiles.offer.resetTurn': 'Poništi potez',
  'rummytiles.offer.commit': 'Gotovo',
  'rummytiles.offer.draw': 'Vuci',
  'rummytiles.param.position': 'Razdvoji na',

  // --- blackjack -----------------------------------------------------------
  //
  // The house rules are worded twice over — on and off — because a rule that
  // is *not* in force is still something a player has to be told. "No
  // surrender at this table" is information; silence is a guess.
  'err.BET_BELOW_MINIMUM': 'To je ispod minimuma stola',
  'err.ALREADY_BET': 'Tvoj ulog je već postavljen',
  'err.INSURANCE_CLOSED': 'Trenutno nema osiguranja za uzeti',
  'err.CANNOT_DOUBLE': 'Ova se ruka ne može udvostručiti',
  'err.CANNOT_SPLIT': 'Ova se ruka ne može razdvojiti',
  'err.CANNOT_SURRENDER': 'Ova se ruka ne može predati',

  'blackjack.rules.section.table': 'Stol',
  'blackjack.rules.section.play': 'Igranje ruke',
  'blackjack.rules.section.dealer': 'Djelitelj',
  'blackjack.rules.section.end': 'Kako meč završava',
  'blackjack.rules.goal':
    'Pobijedi djelitelja bez prelaska dvadeset i jedan. Prijeđeš li, odmah gubiš, što god djelitelj poslije napravio.',
  // Counts are their own phrase rather than a number glued to a noun: one
  // deck and six decks inflect differently, and Czech inflects them again.
  'blackjack.rules.decks': 'Špilova u cipeli: {n}.',
  'blackjack.rules.stack': 'Svako mjesto sjeda s {n} žetona.',
  'blackjack.rules.minBet': 'Minimum stola je {n} žetona.',
  'blackjack.rules.faceUp':
    'Karte igrača dijele se licem gore; djelitelj drži jednu kartu pokrivenu dok svi ne odigraju.',
  'blackjack.rules.hitStand': 'Vuci koliko god karata želiš ili stani na onome što imaš.',
  'blackjack.rules.aces': 'As vrijedi jedanaest dok to stane, a inače jedan.',
  'blackjack.rules.blackjack': 'As s kartom vrijednosti deset, na prve dvije karte, je blackjack.',
  'blackjack.rules.pays3to2': 'Blackjack plaća 3:2.',
  'blackjack.rules.pays6to5': 'Blackjack plaća 6:5.',
  'blackjack.rules.paysEven': 'Blackjack plaća jedan naprema jedan.',
  'blackjack.rules.double': 'Na prve dvije karte smiješ udvostručiti ulog i uzeti točno još jednu kartu.',
  'blackjack.rules.doubleAfterSplit': 'I ruka nastala razdvajanjem smije se udvostručiti.',
  'blackjack.rules.noDoubleAfterSplit': 'Ruka nastala razdvajanjem ne smije se udvostručiti.',
  'blackjack.rules.split':
    'Dvije karte iste vrijednosti smiju se razdvojiti u zasebne ruke, svaka s vlastitim ulogom — do {n} puta, za ukupno {hands} ruku.',
  'blackjack.rules.noSplit': 'Za ovim stolom parovi se ne razdvajaju.',
  'blackjack.rules.splitAces':
    'Razdvojeni asovi dobivaju po jednu kartu i zatim staju, a dvadeset i jedan ostvaren tako nije blackjack.',
  'blackjack.rules.surrender':
    'Prvu ruku smiješ predati za polovicu uloga, nakon što djelitelj provjeri blackjack.',
  'blackjack.rules.noSurrender': 'Za ovim stolom ruke se ne mogu predati.',
  'blackjack.rules.dealerDraws': 'Djelitelj vuče do sedamnaest, a zatim staje.',
  'blackjack.rules.hitsSoft17': 'Djelitelj vuče i na sedamnaest sastavljenoj s asom.',
  'blackjack.rules.standsSoft17': 'Djelitelj staje na sedamnaest sastavljenoj s asom.',
  'blackjack.rules.dealerPeeks':
    'Pokazuje li asa ili desetku, djelitelj provjerava blackjack prije nego što itko odigra.',
  'blackjack.rules.insurance':
    'Protiv djeliteljeva asa smiješ se osigurati za polovicu uloga; plaća 2:1 ako djelitelj ima blackjack.',
  'blackjack.rules.noInsurance': 'Za ovim stolom osiguranje se ne nudi.',
  'blackjack.rules.rounds': 'Za stolom se igra {n} rundi.',
  'blackjack.rules.mostChipsWins': 'Tko na kraju ima najviše žetona, pobjeđuje u meču.',
  'blackjack.rules.bustedOut':
    'Mjesto koje više ne može pokriti minimum od {n} sjedi po strani do kraja meča.',

  'blackjack.zone.dealer': 'Djelitelj',
  'blackjack.zone.box': 'Ruka',
  'blackjack.zone.yourBox': 'Tvoja ruka',
  'blackjack.zone.shoe': 'Cipela',

  'blackjack.header.round': 'Runda {n} od {of}',
  'blackjack.header.minBet': 'Minimum',
  'blackjack.header.decks': 'Špilovi',
  'blackjack.header.dealerTotal': 'Djelitelj pokazuje {n}',
  'blackjack.header.dealerSoftTotal': 'Djelitelj pokazuje meku {n}',

  'blackjack.seat.stack': 'Žetoni',
  'blackjack.seat.bet': 'Ulog',
  'blackjack.seat.insurance': 'Osiguranje',
  'blackjack.seat.total': 'Ukupno',
  'blackjack.seat.softTotal': 'Meki zbroj',
  'blackjack.seat.out': 'Bez žetona',

  'blackjack.prompt.placeBet': 'Postavi svoj ulog',
  'blackjack.prompt.insurance': 'Osiguranje?',
  'blackjack.prompt.yourMove': 'Ti si na redu',
  'blackjack.prompt.waitingFor': 'Čeka se {playerId}',
  'blackjack.prompt.betAmount': 'Ulog',

  'blackjack.offer.bet': 'Uloži',
  'blackjack.offer.hit': 'Karta',
  'blackjack.offer.stand': 'Stajem',
  'blackjack.offer.double': 'Udvostruči',
  'blackjack.offer.split': 'Razdvoji',
  'blackjack.offer.surrender': 'Predaj',
  'blackjack.offer.insure': 'Uzmi osiguranje',
  'blackjack.offer.declineInsurance': 'Bez osiguranja',

  'blackjack.fact.tableMinimum': 'minimum',
  'blackjack.fact.insuranceCost': 'za osiguranje',
  'blackjack.fact.extraStake': 'za ulog',
  'blackjack.fact.surrenderReturn': 'natrag',

  'blackjack.status.dealerBlackjack': 'Djelitelj je imao blackjack',
  'blackjack.status.dealerBust': 'Djelitelj je pukao s {n}',
  'blackjack.status.dealerStands': 'Djelitelj staje na {n}',

  'blackjack.round.name': 'Runda',
  'blackjack.round.dealerTotal': 'Djelitelj {n}',
  'blackjack.round.dealerBust': 'Djelitelj pukao ({n})',
  'blackjack.round.dealerBlackjack': 'Djeliteljev blackjack',
  'blackjack.round.outcome.blackjack': 'Blackjack',
  'blackjack.round.outcome.win': 'Dobiveno',
  'blackjack.round.outcome.push': 'Neriješeno',
  'blackjack.round.outcome.lose': 'Izgubljeno',
  'blackjack.round.outcome.bust': 'Puklo',
  'blackjack.round.outcome.surrender': 'Predano',

  'blackjack.badge.inPlay': 'U igri',
  'blackjack.badge.doubled': 'Udvostručeno',
  'blackjack.badge.split': 'Razdvojeno',
  'blackjack.badge.blackjack': 'Blackjack',
  'blackjack.badge.bust': 'Puklo',
  'blackjack.badge.won': 'Dobiveno',
  'blackjack.badge.push': 'Neriješeno',
  'blackjack.badge.lost': 'Izgubljeno',
  'blackjack.badge.surrendered': 'Predano',

  'blackjack.unit.chips': 'žetona',

  // --- the settings screen --------------------------------------------------
  //
  // Hardcoded English until now, which was survivable while the only other
  // language was unreachable and faintly absurd once the language picker
  // itself lived on this screen: a player who cannot read "Settings" is
  // exactly the one who came here to change it.
  'settings.title': 'Postavke',
  'settings.signedInAs': 'Prijavljen kao {username}',
  'settings.playingAsGuest': 'Igraš kao {username} (gost)',
  'settings.notSignedIn': 'Nisi prijavljen — prijavi se ili nastavi kao gost za igru online.',
  'settings.subtitle': 'Kako izgledaš ti i kako stol',
  'settings.face.heading': 'Tvoje lice za stolom',
  'settings.face.account': 'Čuva se uz tvoj račun, pa te prati i na drugi uređaj.',
  'settings.face.device': 'Čuva se na ovom uređaju. Prijavi se da ga poneseš sa sobom.',
  'settings.skin.heading': 'Izgled stola',
  'settings.language.heading': 'Jezik',
  'settings.language.status': 'Čuva se na ovom uređaju.',
  'settings.language.auto': 'Automatski',
  'settings.language.auto.now': 'Prati tvoj uređaj — sada {language}',
  'settings.legal.heading': 'Sitna slova',
  'settings.legal.status': 'Na što si igranjem pristao i što se o tebi pohranjuje.',
  'settings.signIn': 'Prijava',
  'settings.back': 'Natrag',

  // A translated interface over an English notice is a worse lie than an
  // English interface, so the document says so itself rather than letting the
  // surrounding screen imply the text was written for this reader.
  'legal.untranslated': 'Ova obavijest još nije prevedena na tvoj jezik. Vrijedi engleski tekst ispod.',

  // --- the navigation bar's screen titles ------------------------------------
  //
  // These sit in `app/_layout.tsx`, above every screen, and were the last
  // English left after the settings screen was translated — a German player
  // reading "Einstellungen" under a bar that said "Settings". A translated app
  // with an untranslated chrome bar looks like a bug, not like a limit.
  'nav.home': 'Zolik',
  'nav.emailSignIn': 'Prijava e-poštom',
  'nav.signingIn': 'Prijava u tijeku',
  'nav.usernameSignIn': 'Prijava korisničkim imenom',
  'nav.legacyAccount': 'Stari račun',
  'nav.guest': 'Gost',
  'nav.account': 'Račun',
  'nav.games': 'Igre',
  'nav.table': 'Tvoj stol',
  'nav.join': 'Pridruži se stolu',
  // The screen a shared invite link lands on while it resolves the code.
  'nav.joining': 'Pridruživanje',
  'nav.rules': 'Pravila',
  'nav.match': 'Meč',
  'nav.scoreTable': 'Tablica bodova',
  'nav.stats': 'Statistika',
  'nav.more': 'Više',

  // --- the account menu behind the face in the corner ----------------------
  'menu.label': 'Izbornik računa',
  'menu.signedIn': 'Prijavljen',
  'menu.notSignedIn': 'Nisi prijavljen',
  'menu.keepStats': 'da sačuvaš svoju statistiku',
  'menu.signOut': 'Odjava',
  'more.scoreTable': 'Offline tablica rezultata',
  'more.stats': 'Statistika i ljestvica',
  'more.needsAccount': 'prijavi se za korištenje',
  'gate.title': 'Prijavi se da ovo koristiš',
  'gate.body':
    'Tablice rezultata i statistika čuvaju se uz tvoj račun, pa te prate na drugi uređaj. Gost ih nema gdje čuvati.',

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
  'error.generic': 'To nije uspjelo',
  'error.signIn': 'Prijava nije uspjela',
  'error.login': 'Prijava nije uspjela',
  'error.register': 'Registracija nije uspjela',
  'error.sendCode': 'Kôd nije bilo moguće poslati',
  'error.badCode': 'Taj kôd nije radio',
  'error.rulesLoad': 'Pravila nije bilo moguće učitati',
  'error.createFailed': 'Stvaranje nije uspjelo',
  'error.saveFailed': 'Spremanje nije uspjelo',
  'error.exportFailed': 'Izvoz nije uspio',

  // --- the route that does not exist ----------------------------------------
  'notFound.title': 'Ups!',
  'notFound.message': 'Ovaj zaslon ne postoji.',
  'notFound.home': 'Idi na početni zaslon!',

  // --- signing in -----------------------------------------------------------
  'auth.login.subtitle': 'Zadrži svoju statistiku na svim uređajima',
  'auth.login.continueWithEmail': 'Nastavi e-poštom',
  'auth.login.usernameInstead': 'Prijavi se radije korisničkim imenom',
  'auth.email.title': 'Prijava e-poštom',
  'auth.email.subtitle': 'Poslat ćemo ti jednokratni kôd',
  'auth.email.address': 'Adresa e-pošte',
  'auth.email.send': 'Pošalji kôd',
  'auth.email.codeTitle': 'Upiši kôd',
  'auth.email.codePlaceholder': 'Šesteroznamenkasti kôd',
  'auth.email.differentAddress': 'Upotrijebi drugu adresu',
  'auth.email.sentTo': 'Poslano na {email}',
  'auth.email.continue': 'Nastavi',
  'auth.guest.title': 'Igra kao gost',
  'auth.guest.subtitle': 'Račun nije potreban',
  'auth.guest.displayName': 'Prikazano ime',
  'auth.register.title': 'Stvori račun',
  'auth.register.username': 'Korisničko ime',
  'auth.register.email': 'E-pošta (neobavezno)',
  'auth.register.password': 'Lozinka',
  'auth.username.createAccount': 'Stvori račun s korisničkim imenom i lozinkom',
  'auth.callback.signedIn': 'Prijavljeno.',

  // --- the account screen ---------------------------------------------------
  'account.signInPrompt': 'Prijavi se kako bi upravljao računom.',
  'account.keepGames': 'Zadrži ove igre',
  'account.signedInWith': 'Prijavljen preko',
  'account.addMethod': 'Dodaj način prijave',
  'account.usernameAndPassword': 'Korisničko ime i lozinka',
  'account.faceAndTable': 'Lice i izgled stola',
  'account.refresh': 'Osvježi',
  'account.remove': 'Ukloni',

  // --- the main menu --------------------------------------------------------
  'home.subtitle': 'Kontinentalni remi · {server}',
  'home.playingAs': 'Igraš kao {name}',
  'home.signInPrompt': 'Prijavi se ili nastavi kao gost da bi igrao na mreži.',
  'home.statsAndLeaderboard': 'Statistika i ljestvica',
  'home.play': 'Igraj',
  'home.offlineScoreTable': 'Tablica bodova izvan mreže',
  'home.signInToKeepStats': 'Prijavi se da zadržiš statistiku',
  'home.signOut': 'Odjava',
  'home.continueAsGuest': 'Nastavi kao gost',
  // Someone playing as a guest, marked in a list of names.
  'home.guestSuffix': '(gost)',

  // --- the waiting room on the main menu ------------------------------------
  //
  // Counts are whole phrases per count, not a number glued to a noun — the
  // same reason `countLabel` exists. One and many are the cases the interface
  // actually produces, so they are the cases that get their own wording.
  'waiting.checking': 'Gledamo tko je u blizini…',
  'waiting.youAreWaiting': 'Čekaš da igraš',
  'waiting.pickedUp': 'Tko god otvori stol može te pokupiti — nikome ne treba kôd od tebe.',
  'waiting.othersOne': 'Čeka i još 1 igrač',
  'waiting.othersMany': 'Čeka i još {n} igrača',
  'waiting.oneWaiting': '1 igrač čeka da igra',
  'waiting.manyWaiting': '{n} igrača čeka da igra',
  'waiting.adding': 'Dodajemo te na listu čekanja…',
  'waiting.slowHint':
    'Ako ovo ne završi u par sekundi, provjeri je li adresa poslužitelja ispod dostupna s ovog uređaja.',
  'waiting.serverBusyDetail': 'Pokušaj {n}. Poslužitelj trenutno ne prima nove veze prema čekaonici.',
  'waiting.reconnecting': 'Veza izgubljena — ponovno se spajamo…',
  'waiting.reconnectingDetail':
    'Pokušaj {n}. To se može dogoditi ako se mreža tvog uređaja promijenila ili se poslužitelj ponovno pokrenuo.',
  'waiting.tryAgain': 'Pokušaj odmah ponovno',
  'waiting.makeAvailable': 'Učini me dostupnim za igru',
  'waiting.stop': 'Prestani čekati',
  'waiting.noneYet': 'Trenutno nitko ne čeka igru. Upiši se na listu i bit ćeš prvi koga itko vidi.',
  'waiting.noOthersYet': 'Nitko drugi još ne čeka. Domaćini te ipak vide i mogu te pozvati.',
  'waiting.server': 'Poslužitelj',
  'waiting.none': 'Trenutno nitko ne čeka. Tko se učini dostupnim u glavnom izborniku, pojavljuje se ovdje.',

  // --- landing on a shared invite link --------------------------------------
  'join.missingCode': 'Toj poveznici nedostaje kôd stola.',
  'join.staleLink': 'Zatraži svježu poveznicu od onoga tko te pozvao, ili se pridruži kodom.',
  'join.enterCode': 'Upiši kôd',
  'join.backToMenu': 'Natrag na izbornik',
  'join.takingSeat': 'Zauzimamo mjesto…',
  'join.takingSeatAt': 'Zauzimamo mjesto za {game}…',

  // --- the lobby ------------------------------------------------------------
  'lobby.games.subtitle': 'Sve što ovaj poslužitelj može ponuditi',
  'lobby.games.bots': 'Botovi',
  'lobby.games.playBot': 'Igraj protiv bota',
  'lobby.games.playBots': 'Igraj protiv {n} bota',
  'lobby.games.openTable': 'Otvori stol',
  'lobby.games.players': 'Igrača: {n}',
  'lobby.games.playerRange': 'Igrača: {min}–{max}',
  'lobby.join.placeholder': 'Kôd za pridruživanje ili poveznica s pozivom',
  'lobby.join.needCode': 'Upiši kôd, poveznicu ili ID meča',
  'lobby.games.signInFirst': 'Prvo se prijavi',
  'lobby.join.action': 'Pridruži se',
  'lobby.join.waitingTitle': 'Čekamo domaćina',
  // Two whole sentences rather than one with a swapped noun: "a game of {game}"
  // does not survive a language that inflects the game's name after "of".
  'lobby.join.joinedGame': 'Pridružio si se igri {game} — čekamo početak',
  'lobby.join.joinedTable': 'Pridružio si se stolu — čekamo početak',
  'lobby.table.addBot': 'Dodaj bota',
  'lobby.table.start': 'Počni',
  'lobby.table.waitingForHost': 'Čekamo da domaćin počne…',

  // --- inviting someone to a table ------------------------------------------
  'invite.heading': 'Pozovi igrače',
  'invite.explain': 'Pošalji ovu poveznicu. Tko je otvori, dolazi za ovaj stol — račun nije potreban.',
  'invite.noAddress': 'Za ovaj poslužitelj nije postavljena adresa za dijeljenje, pa upotrijebi kôd ispod.',
  'invite.readOutCode': 'Ili izdiktiraj kôd:',
  'invite.copy': 'Kopiraj poveznicu',
  'invite.share': 'Podijeli poveznicu',
  'invite.copied': 'Kopirano!',
  'invite.shared': 'Podijeljeno',

  // --- the match screen -----------------------------------------------------
  'match.waitingForTable': 'Čekamo stol…',
  'match.waitingForPlayer': 'Čekamo drugog igrača…',
  'match.nobodyWon': 'Nitko nije pobijedio.',
  'match.youWon': 'Pobijedio si.',
  'match.finished': 'Ovaj meč je završio.',
  'match.inProgress': 'Meč je u tijeku — sve je povezano i teče normalno.',
  'match.controls': 'Upravljanje',
  'match.over': 'Kraj meča',
  'match.settingUp': 'Pripremamo…',
  'match.playAgain': 'Igraj ponovno',
  'match.backToGames': 'Natrag na igre',
  'match.table': 'Stol',
  'match.opponents': 'Protivnici',
  // Marks which seat is the reader's own, in a list of seats.
  'match.youSuffix': '(ti)',
  // The winner line is composed from whole sentences rather than a name glued
  // to " won." — the verb agrees with the subject in most of these languages,
  // and a suffix cannot know that.
  'match.you': 'ti',
  'match.someoneWon': '{name} pobjeđuje.',
  'match.wonBy': 'Pobjeđuje {names}.',
  'match.pausedFor': 'Pauzirano — čekamo da se {name} ponovno spoji.',
  'match.results': 'Rezultati',
  'match.players': 'Igrači',
  'match.toPlay': 'na potezu',

  // --- the offline score table ----------------------------------------------
  'scoring.namesHint': 'Imena odvojena zarezima (4–8 igrača)',
  'scoring.newSession': 'Nova sesija',
  // A worked example, not a sentence: the names are placeholders a translator
  // may localise, the shape `name:score,` is what the parser needs.
  'scoring.scoresPlaceholder': 'Ana:120,Boris:80,…',
  'scoring.saveRound': 'Spremi rundu',
  'scoring.export': 'Izvezi bodovnu listu',
  'scoring.formatHint': 'Format bodova: Ime:100,Ime2:50',
  'scoring.nameCountError': 'Upiši 2–8 imena igrača odvojenih zarezima',
  'scoring.session': 'Sesija: {id}',
  'scoring.players': 'Igrači: {names}',
  'scoring.roundScores': 'Bodovi runde {n}',
  'stats.loading': 'Učitavanje…',
  // The figures could not be fetched. `{reason}` is whatever the server or the
  // network said, which is not ours to translate — the frame around it is.
  'stats.unavailable': '(nedostupno: {reason})',

  // --- statistics -----------------------------------------------------------
  'stats.title': 'Statistika i ljestvica',
  'stats.yours': 'Tvoja statistika',
  'stats.leaderboard': 'Ljestvica',

  // --- a player's lifetime record, shown beside a finished match ------------
  'record.title': 'Tvoj učinak',
  'record.guest':
    'Igraš kao gost, pa se učinak ne vodi. Prijavi se i igre koje si na ovom uređaju već odigrao — uključujući ovu — ostat će uz tvoj račun.',
  'record.signInToKeep': 'Prijavi se i zadrži ih',
  'record.failed': 'Tvoj učinak trenutno nije bilo moguće učitati. Meč je sigurno zabilježen.',
  'record.loading': 'Učitavanje…',
  'record.played': 'Odigrano',
  'record.won': 'Pobjede',
  'record.lost': 'Porazi',
  'record.winRate': 'Postotak pobjeda',
  'record.streak': 'Niz',
  'record.atThisGame': 'U ovoj igri',
  // A streak is a count with a noun, so it is a whole phrase per count for
  // the same reason `countLabel` is: "1 win" / "2 wins" is an English rule,
  // and most of these languages do not share it.
  'record.streakNone': '—',
  'record.streakWinOne': '1 pobjeda',
  'record.streakWinMany': '{n} pobjeda',
  'record.streakLossOne': '1 poraz',
  'record.streakLossMany': '{n} poraza',

  // --- moving cards ---------------------------------------------------------
  'hand.dragHint': 'Povuci kartu duž lepeze da je premjestiš, ili na stol da je odigraš',
  'hand.moveLeft': 'Ulijevo',
  'hand.moveRight': 'Udesno',
  'zone.collapseGroup': 'Sažmi ovu grupu',
  'zone.expandGroup': 'Prikaži sve karte ove grupe',
  'zone.dropHere': 'Ispusti ovdje',
  'offer.pickCards': 'odaberi karte za mjesto koje si dodirnuo',
  'offer.ambiguous': 'ovo može ići na više mjesta — odaberi na stolu',

  // --- the build footer -----------------------------------------------------
  'build.app': 'aplikacija',
  'build.server': 'poslužitelj',
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
  'option.pauseBetweenRounds': 'Pauza između rundi',
  'choice.pauseBetweenRounds.1': 'Pauza',
  'choice.pauseBetweenRounds.0': 'Nastavi odmah',
  'option.botSkill': 'Protivnici',
  'choice.botSkill.0': 'Miješani',
  'choice.botSkill.1': 'Lagani',
  'choice.botSkill.2': 'Srednji',
  'choice.botSkill.3': 'Teški',
  'option.initialMeldMinimum': 'Vrijednost otvaranja',
  'choice.initialMeldMinimum.0': 'Bez',
  'option.discardDrawMinRound': 'Uzimanje s hrpe odbačenih',
  'choice.discardDrawMinRound.0': 'Otvoreno',
  'choice.discardDrawMinRound.2': 'Od runde 2',
  'choice.discardDrawMinRound.3': 'Od runde 3',
  'option.requireCleanRun': 'Niz bez džokera',
  'choice.requireCleanRun.1': 'Obavezan',
  'choice.requireCleanRun.0': 'Ne',
  'option.jokerReclaimMustPlay': 'Otkupljeni džoker',
  'choice.jokerReclaimMustPlay.1': 'Odigrati u istom potezu',
  'choice.jokerReclaimMustPlay.0': 'Smije se zadržati',
  'option.dealStarter': 'Tko počinje',
  'choice.dealStarter.0': 'Redom',
  'choice.dealStarter.1': 'Počinje pobjednik',
  'variation.prsi.classic': 'Klasično',
  'option.handSize': 'Podijeljene karte',
  'variation.canasta.classic': 'Klasična',
  'variation.canasta.modern_american': 'Modern American',
  'option.targetScore': 'Ciljni rezultat',
  'option.canastasToGoOut': 'Canaste za izlazak',
  'variation.holdem.freezeout': 'Freezeout',
  'variation.holdem.timed': 'Stalan broj dijeljenja',
  'option.startingStack': 'Početni žetoni',
  'option.bigBlind': 'Veliki blind',
  'option.handLimit': 'Dijeljenja',
  'choice.handLimit.0': 'Dok ne ostane jedno mjesto',
  'variation.ginrummy.standard': 'Standardni',
  'option.knockLimit': 'Granica kucanja',
  'choice.knockLimit.0': 'Oklahoma (određuje je okrenuta karta)',
  'option.bigGin': 'Big gin',
  'choice.bigGin.0': 'Isključeno',
  'choice.bigGin.1': 'Uključeno (+25)',
  'option.lineBonuses': 'Bonusi u obračunu',
  'choice.lineBonuses.1': 'Uključeni',
  'choice.lineBonuses.0': 'Isključeni',
  'variation.rummytiles.standard': 'Standardno',
  'choice.targetScore.0': 'Bez',
  // Two games qualify a number the others mean plainly: Canasta's 500 is a
  // short game, Gin Rummy's and Rummy Tiles' 500 is just 500; Hold'em's
  // starting 200 is a short stack, Blackjack's 200 is just 200. Module-scoped
  // so the qualifier travels with the game that means it — a bare key would
  // hand the note to games it is false for. `scripts/check-server-labels.js`
  // is what finds these; both were shipping as English on a Czech screen.
  'choice.canasta.targetScore.500': '500 (kratka)',
  'choice.holdem.startingStack.200': '200 (kratka)',
  'option.roundLimit': 'Ograničenje rundi',
  'choice.roundLimit.0': 'Bez',
  'option.poolExhaustion': 'Ako se zaliha potroši',
  'choice.poolExhaustion.1': 'Rundu dobiva najniža ruka',
  'choice.poolExhaustion.0': 'Rundu ne dobiva nitko',
  'variation.blackjack.single': 'Jedan špil',
  'option.minBet': 'Minimum stola',
  'option.rounds': 'Runde',
  'option.decks': 'Špilovi',
  'option.dealerHitsSoft17': 'Djelitelj na mekoj 17',
  'choice.dealerHitsSoft17.0': 'Staje',
  'choice.dealerHitsSoft17.1': 'Vuče',
  'option.blackjackPays': 'Blackjack plaća',
  'choice.blackjackPays.100': 'Jedan naprema jedan',
  'option.maxSplits': 'Razdvajanje',
  'choice.maxSplits.0': 'Bez razdvajanja',
  'choice.maxSplits.1': 'Jednom (dvije ruke)',
  'choice.maxSplits.3': 'Tri puta (četiri ruke)',
  'option.doubleAfterSplit': 'Udvostručenje nakon razdvajanja',
  'choice.doubleAfterSplit.1': 'Dopušteno',
  'choice.doubleAfterSplit.0': 'Nije dopušteno',
  'option.surrender': 'Predaja',
  'choice.surrender.0': 'Isključena',
  'choice.surrender.1': 'Kasna predaja',
  'option.insurance': 'Osiguranje',
  'choice.insurance.1': 'Nudi se',
  'choice.insurance.0': 'Ne nudi se',

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
  'verb.add': 'Dodaj',
  'verb.bet': 'Uloži',
  'verb.call': 'Prati',
  'verb.check': 'Čekiram',
  'verb.commit': 'Gotovo',
  'verb.continue': 'Nastavi',
  'verb.decline_insurance': 'Bez osiguranja',
  'verb.discard': 'Odbaci',
  'verb.double': 'Udvostruči',
  'verb.draw': 'Vuci',
  'verb.finish_layoff': 'Gotovo s prislanjanjem',
  'verb.fold': 'Odustajem',
  'verb.hit': 'Karta',
  'verb.insure': 'Uzmi osiguranje',
  'verb.knock': 'Kucni',
  'verb.lay_meld': 'Spusti',
  'verb.lay_off': 'Prisloni',
  'verb.pass': 'Dalje',
  'verb.place': 'Postavi',
  'verb.play_card': 'Odigraj',
  'verb.raise': 'Podiži',
  'verb.reset_turn': 'Poništi potez',
  'verb.split': 'Razdvoji',
  'verb.stand': 'Stajem',
  'verb.surrender': 'Predaj',
  'verb.swap_joker': 'Zamijeni džokera',
  'verb.take': 'Uzmi',
  'verb.take_pile': 'Uzmi s hrpe',
  // Declared by the server and listed in `serverKeys.json`, but never worded.
  'verb.takePileFromHand': 'Uzmi hrpu u ruku',
  'verb.takePileOntoMeld': 'Uzmi hrpu na kombinaciju',
  'verb.undoDraw': 'Poništi vučenje',
  'verb.undoLayOff': 'Poništi prislanjanje',
  'verb.undoMeld': 'Poništi kombinaciju',
  'verb.undoTurn': 'Poništi potez',

  // --- suits, spelled out ---------------------------------------------------
  //
  // Read aloud by a screen reader and shown where a pip would not fit. The
  // pips themselves are drawn, not written, so these are the only place the
  // suit is ever a word.
  'suit.C': 'Tref',
  'suit.D': 'Karo',
  'suit.H': 'Herc',
  'suit.S': 'Pik',

  // --- counters the board prints beside a number ----------------------------
  'canasta.seat.notOpened': 'Nije otvorio',
  'canasta.unit.points': 'bodova',
  'ginrummy.unit.points': 'bodova',
  'holdem.seat.dealer': 'Djelitelj',
  'holdem.unit.chips': 'žetona',
  'prsi.unit.cardsLeft': 'preostalo karata',
  'rummytiles.prompt.initialMeld': 'Tvoje prvo spuštanje mora vrijediti {n} bodova.',
  'rummytiles.unit.points': 'bodova',
  'zolik.unit.penalty': 'kazna',
  'header.pileFrozen': 'Hrpa zamrznuta',
  // The module-less form of `ginrummy.prompt.yourTurnDraw`: Žolíky and Canasta
  // send the prompt without a module prefix. Neither the Go constants nor
  // `serverKeys.json` list it — the sweep is what found it.
  'prompt.yourTurnDraw': 'Vuci kartu',
};
