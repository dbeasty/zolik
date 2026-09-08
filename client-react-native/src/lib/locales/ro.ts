/**
 * Romanian. Remi vocabulary: grup for a set, scară for a run, combinație for a meld, talon for the stock.
 */

export const ro: Record<string, string> = {
  // --- engine error codes (rules.RulesErrorCode) ---------------------------
  'err.NOT_YOUR_TURN': 'Nu e rândul tău',
  'err.WRONG_PHASE': 'Nu se poate acum',
  'err.MUST_DRAW_FIRST': 'Trage o carte înainte să cobori',
  'err.GAME_SUSPENDED': 'Jocul este în pauză',
  'err.GAME_NOT_ACTIVE': 'Jocul nu se desfășoară',
  // --- runtime refusals, which are not the rules saying no -----------------
  //
  // Both were reachable long before they were worded, and a player who met
  // one was shown the code in capitals or, worse, nothing at all: an action
  // sent while the socket was down simply vanished, which reads as a legal
  // move being silently refused.
  'err.MATCH_NOT_ACTIVE': 'Masa este în pauză — se așteaptă reconectarea unui jucător',
  'err.NOT_CONNECTED': 'Fără conexiune la masă — se reconectează, apoi încearcă din nou',
  // Why the "start the next round" control is greyed out. Without wording the
  // shell printed the code itself, so a player agreeing to go on was answered
  // with ALREADY_READY in capitals.
  'err.ALREADY_READY': 'Ești pregătit',
  'err.NOT_BETWEEN_ROUNDS': 'Runda încă se joacă',
  'err.NOT_AT_THIS_TABLE': 'Nu ești la această masă',
  'err.DISCARD_LOCKED': 'Teancul de aruncate este blocat deocamdată',
  'err.DISCARD_PILE_EMPTY': 'Teancul de aruncate este gol',
  'err.NO_CARDS_LEFT': 'Nu mai sunt cărți de tras',
  'err.ROUND_REQ_NOT_MET': 'Coboară mai întâi deschiderea ta',
  'err.NEED_CLEAN_RUN': 'Ai nevoie de o scară fără joker pe masă ca să fii considerat coborât',
  'err.INCOMPLETE_INITIAL_MELD': 'Termină coborârea, sau anuleaz-o, înainte să arunci',
  'err.DISCARD_CARD_NOT_MELDED': 'Cartea pe care ai luat-o trebuie să intre în combinația ta',
  'err.JOKER_DISCARD_FORBIDDEN': 'Un joker nu poate fi aruncat',
  'err.NOTHING_TO_UNDO': 'Nu e nimic de anulat',
  'err.NO_JOKER_IN_MELD': 'Nu există joker în această combinație',
  'err.JOKER_SWAP_MISMATCH': 'Acea carte nu ia locul jokerului',
  'err.RECLAIMED_JOKER_NOT_MELDED': 'Jokerul luat de pe masă trebuie jucat într-o combinație în această tură',
  'err.RUN_TOO_LONG': 'Acea scară are deja lungimea maximă',
  'err.WRONG_RUN_END': 'Acea carte prelungește celălalt capăt al scării',
  'err.INVALID_MELD': 'Nicio carte din mâna ta nu se potrivește aici',
  'err.CARD_NOT_IN_HAND': 'Acea carte nu este în mâna ta',
  'err.MELD_BELOW_MINIMUM': 'Combinațiilor tale le lipsesc încă puncte ca să cobori',
  'err.MELD_NO_CONTRIBUTION': 'Acea combinație nu îți avansează cerința',
  'err.TOO_MANY_WILDS': 'Prea mulți jokeri în acea combinație',
  'err.ADJACENT_WILDS': 'Doi jokeri nu pot sta unul lângă altul',
  'err.ACE_BRIDGE': 'Un as nu poate lega popa de doi',

  // --- contract phrasing ---------------------------------------------------
  // Built from the structured contract the server sends (sets/runs counts),
  // never from a deal-number lookup table.
  // Whole phrases per count rather than a number plus a pluralised noun:
  // Czech inflects the noun by count (skupina / skupiny / skupin) in a way no
  // "add an s" helper survives, and the counts that actually occur are 1-3.
  // A count outside that falls back to contract.sets.n.
  'contract.sets.1': 'Un grup',
  'contract.sets.2': 'Două grupuri',
  'contract.sets.3': 'Trei grupuri',
  'contract.sets.n': '{n} grupuri',
  'contract.runs.1': 'O scară',
  'contract.runs.2': 'Două scări',
  'contract.runs.3': 'Trei scări',
  'contract.runs.n': '{n} scări',
  'contract.any': 'Orice combinație validă',
  'contract.cleanRunOnly': 'Orice amestec de grupuri și scări — cel puțin o scară trebuie să fie fără joker',
  'contract.cleanRunSuffix': '{base} — o scară trebuie să fie fără joker',

  // --- written rules ---------------------------------------------------------
  // Full-sentence keys a "see the rules" screen renders, one per module,
  // resolved server-side against the table's actual variation and options —
  // see internal/module/rules.go and each game's rules.go. Shared section
  // titles first, then one block per module in the order the four games were
  // added.
  'zolik.rules.section.goal': 'Scop',
  'zolik.rules.section.setup': 'Pregătire',
  'zolik.rules.section.turn': 'Rândul tău',
  'zolik.rules.section.melding': 'Coborârea',
  'zolik.rules.section.end': 'Cum se încheie meciul',
  'zolik.rules.goal':
    'Fii primul care își golește mâna coborând grupuri și scări valide, strângând cât mai puține puncte de penalizare în cărțile pe care le mai ții când altcineva iese.',
  'zolik.rules.deal': 'Fiecare jucător primește {n} cărți.',
  'zolik.rules.meldShapes':
    'Un grup are {set}+ cărți de aceeași valoare; o scară are {run}+ cărți consecutive de aceeași culoare.',
  'zolik.rules.turn.draw': 'La rândul tău, trage o carte — din talon sau din teancul de aruncate.',
  'zolik.rules.pickup.topOnly': 'Doar cartea de deasupra teancului de aruncate poate fi luată.',
  'zolik.rules.pickup.anyFromPile':
    'Orice carte din teancul de aruncate poate fi luată, împreună cu tot ce se află deasupra ei.',
  'zolik.rules.pickup.locked': 'Din teancul de aruncate nu se poate trage înainte de runda {n}.',
  'zolik.rules.pickup.open': 'Teancul de aruncate este deschis din prima rundă.',
  'zolik.rules.turn.discard': 'Încheie-ți rândul aruncând o carte.',
  'zolik.rules.jokers.restricted':
    'Un joker nu poate fi aruncat niciodată, decât dacă este exact cartea care îți golește mâna.',
  'zolik.rules.lead.rotate':
    'Prima mutare se deplasează cu un loc la fiecare împărțire, indiferent cine a câștigat.',
  'zolik.rules.lead.winner': 'Cine iese începe împărțirea următoare.',
  'zolik.rules.meldFloor.on':
    'Prima ta coborâre trebuie să totalizeze cel puțin {n} puncte naturale ca să fii coborât.',
  'zolik.rules.meldFloor.off': 'Nu există o valoare minimă de puncte la prima coborâre.',
  'zolik.rules.cleanRun.on':
    'Cel puțin una dintre scările tale trebuie să fie complet fără joker ca să fii considerat coborât.',
  'zolik.rules.cleanRun.off': 'Scările tale pot folosi jokeri liber — niciuna nu trebuie să fie fără ei.',
  'zolik.rules.contracts.rotating':
    'Meciul are {n} împărțiri, iar fiecare împărțire cere propria combinație de grupuri și scări.',
  'zolik.rules.contracts.static':
    'Fiecare împărțire cere aceeași combinație: {sets} grupuri și {runs} scări.',
  'zolik.rules.end.afterDeals': 'Meciul se încheie după {n} împărțiri.',
  'zolik.rules.end.atScore':
    'Se împarte mai departe până când cineva ajunge la {n} puncte — atunci s-a terminat.',

  'prsi.rules.section.goal': 'Scop',
  'prsi.rules.section.setup': 'Pregătire',
  'prsi.rules.section.turn': 'Rândul tău',
  'prsi.rules.section.special': 'Cărți speciale',
  'prsi.rules.section.end': 'Cum se încheie meciul',
  'prsi.rules.goal': 'Fii primul care joacă toate cărțile din mână.',
  'prsi.rules.deck': 'Se joacă cu un pachet de {value} cărți (de la 7 în sus).',
  'prsi.rules.deal': 'Fiecare jucător începe cu {n} cărți.',
  'prsi.rules.turn.match':
    'Joacă o carte care se potrivește la culoare sau la valoare cu cea de deasupra — sau trage, dacă nu poți.',
  'prsi.rules.turn.draw': 'Tragerea îți încheie rândul fără să joci.',
  'prsi.rules.sevens': 'Joacă un 7 și următorul jucător trage două cărți, dacă nu răspunde cu un 7 al lui.',
  'prsi.rules.aces': 'Joacă un as și rândul următorului jucător este sărit.',
  'prsi.rules.queens': 'Joacă o damă și spune culoarea care continuă.',
  'prsi.rules.end': 'Meciul se încheie în clipa în care mâna cuiva este goală.',

  'canasta.rules.section.goal': 'Scop',
  'canasta.rules.section.setup': 'Pregătire',
  'canasta.rules.section.melding': 'Coborârea',
  'canasta.rules.section.end': 'Cum se încheie meciul',
  'canasta.rules.goal': 'Se joacă în echipe; prima tabără care ajunge la {n} puncte câștigă meciul.',
  'canasta.rules.deck': 'Se joacă cu {value} cărți — două pachete plus jokeri.',
  'canasta.rules.deal': 'Fiecare jucător primește {n} cărți.',
  'canasta.rules.redThrees':
    'Un trei roșu din mâna ta se arată imediat și aduce bonus — dacă tabăra ta nu termină nicio canastă, se socotește însă împotriva ta.',
  'canasta.rules.canasta': 'O canastă este o combinație de {n} sau mai multe cărți de aceeași valoare.',
  'canasta.rules.meldFloorBands':
    'Prima ta coborâre trebuie să atingă un minim de puncte care crește odată cu scorul tău: {negative} sub zero, {low} până la 1500, {mid} până la 3000, {high} peste.',
  'canasta.rules.oneCanastaToGoOut': 'O canastă terminată e de ajuns ca tabăra ta să iasă.',
  'canasta.rules.twoCanastasToGoOut': 'Tabăra ta are nevoie de două canaste terminate înainte să poată ieși.',
  'canasta.rules.end':
    'Se împarte mai departe până când o tabără depășește {n} puncte — atunci meciul s-a încheiat.',

  'holdem.rules.section.goal': 'Scop',
  'holdem.rules.section.setup': 'Pregătire',
  'holdem.rules.section.betting': 'Pariurile',
  'holdem.rules.section.end': 'Cum se încheie meciul',
  'holdem.rules.goal':
    'Câștigă jetoane având cea mai bună mână la arătare, sau rămânând singurul jucător în mână.',
  'holdem.rules.stack': 'Fiecare loc începe cu {n} jetoane.',
  'holdem.rules.blinds': 'Blindul mic este {sb}, iar cel mare {bb}, puse înainte de împărțirea cărților.',
  'holdem.rules.streets': 'Se pariază în patru runde — înainte de flop și după flop, turn și river.',
  'holdem.rules.showdown': 'Cei rămași în mână își arată cărțile; cea mai bună mână de cinci cărți ia potul.',
  'holdem.rules.noLimit': 'Fără limită — orice pariu poate merge până la tot stacul tău.',
  'holdem.rules.lastPlayerStanding': 'Se joacă până când un loc deține toate jetoanele.',
  'holdem.rules.mostChipsWins': 'Cine are cele mai multe jetoane când jocul se oprește câștigă meciul.',
  'holdem.rules.handLimit': 'Jocul se oprește după {n} mâini.',

  // --- header --------------------------------------------------------------
  'header.deal': 'Mâna {n}',
  'header.gameOf': 'Jocul {n} din {total}',
  'header.gameOfWithContract': 'Jocul {n} din {total}: {contract}',

  // --- meld preview --------------------------------------------------------
  'preview.validSet': 'Grup valid',
  'preview.validRun': 'Scară validă',
  'preview.validMeld': 'Combinație validă',
  'preview.notYet': 'Încă nu e o combinație',
  'preview.points': '{shape} · {n} puncte',
  'preview.pointsWithLaid': '{shape} · {n} + {laid} deja coborâte = {total} puncte',
  'preview.meetsFloor': '{line} (atinge {n} ✓)',
  'preview.needsFloor': '{line} (necesită {n} ✗)',
  'preview.becauseOf': '{line} — {reason}',

  // --- discard -------------------------------------------------------------
  // A discard with a meld staged lays that meld first; this is what the
  // player is told when the server refuses it and the whole move is rolled
  // back. {reason} is the engine's own words for the refusal.
  'discard.meldRejected': '{reason} — nu s-a aruncat nimic, cărțile tale sunt încă pregătite.',

  // --- what the current selection would send -------------------------------
  // The client's own reason a control is not ready, next to the engine's own
  // `err.*` reasons rather than a second kind of message — a control greyed
  // out for "not your turn" and one greyed out for "you picked two, this
  // takes one" should read as the same kind of thing, not one of them looking
  // broken. See `fits` in `src/lib/drops.ts`.
  'sel.tooMany.1': 'Alege doar o carte',
  'sel.tooMany.n': 'Alege cel mult {n} cărți',
  'sel.needMore': 'Alege {n} cărți',
  'sel.notThese': 'Acele cărți nu pot ajunge aici',
  'sel.needsCompany': 'Acea carte are nevoie de cele de lângă ea',

  // --- what a module says happened -----------------------------------------
  // The first module-sent keys in this bundle, and deliberately few. Anything
  // a module sends is still legible without an entry here — `humanise` turns
  // `holdem.seat.stack` into "Stack" — so a key earns a line only when that
  // fallback loses something. These do: what each of them means lives in its
  // *params* (who won, how much, whose cards), and a key on its own has
  // nowhere to put them. Without the entry the player read "Winner", full
  // stop, at the end of a match they had just won.
  'status.winner': 'Câștigat de {winners}',
  'holdem.status.pot': '{winners} a câștigat {amount} cu {hand}',
  'holdem.status.potUncontested': '{winners} a câștigat {amount} — toți ceilalți s-au retras',
  'holdem.status.shown': '{playerId} a arătat {value}',
  'holdem.prompt.waitingFor': 'Se așteaptă {playerId}',

  // How a rummy scoreboard was ordered. Both earn a line for the same reason
  // as the keys above: the number lives in the params, and `humanise` would
  // render "Deals Won" with nothing after it.
  'zolik.standing.dealsWon': 'Împărțiri câștigate: {n}',
  'zolik.standing.inHand': 'În mână: {n}',

  // --- rounds ---------------------------------------------------------------
  // What a round is called, and what one did. Each of these carries its number
  // in the params, so `humanise` alone would render a label with nothing after
  // it.
  // An instruction, not a heading. "Next round" named the thing rather than
  // asking for it, and sat above "Waiting for 1" — which reads as though the
  // player is the one waiting, when the table is waiting for them.
  'round.continue': 'Începe runda următoare',

  // --- the flash that announces a round or a match ending ------------------
  //
  // One line and one number, held for about two seconds. Worded generically on
  // purpose: which *kind* of ending it was is the module's own fact, printed
  // underneath, and a client that phrased "went gin" itself would be a client
  // that knows a game.
  'flash.roundWon': '{winners} a luat-o',
  'flash.roundWonYou': 'Ai luat-o',
  'flash.roundDrawn': 'Nu a luat-o nimeni',
  'flash.matchOver': 'Meciul s-a încheiat',
  'flash.matchWon': '{winners} a câștigat',
  'flash.matchWonYou': 'Ai câștigat',
  'flash.matchDrawn': 'Nu a câștigat nimeni',
  'flash.nowOn': 'acum {total}',

  'zolik.round.deal': 'Mână',
  'zolik.round.cleanRun': 'O scară trebuie să fie fără joker',
  'canasta.round.deal': 'Mână',
  'canasta.round.concealed': 'A ieșit ascuns',
  'canasta.round.exhausted': 'Pachetul s-a terminat',
  'canasta.round.meldCards': 'Cărți coborâte: {n}',
  'canasta.round.canastas': 'Canaste: {n}',
  'canasta.round.redThrees': 'Treiuri roșii: {n}',
  'canasta.round.goingOut': 'Ieșire: {n}',
  'canasta.round.inHand': 'Rămase în mână: {n}',
  'holdem.round.hand': 'Mână',
  'holdem.round.pot': 'Pot {n}',
  'holdem.round.uncontested': 'Toți ceilalți s-au retras',
  'seat.ready': 'Pregătit',
  'results.you': '(tu)',



  // --- refusals the other three games raise ---------------------------------
  // Worded here for the same reason Žolíky's are: reasonText falls back to the
  // code, and OfferBar hands it the code as that fallback, so an unworded
  // refusal reaches a player as PILE_FROZEN in capitals. serverKeys.test.ts
  // now fails the build rather than letting the next one through.
  'err.SET_TOO_LARGE': 'Un grup are deja toate cele patru culori',
  'err.DISCARD_TAKEN_CARD_FORBIDDEN': 'Nu poți arunca cartea pe care tocmai ai luat-o — joac-o sau ține-o',
  'err.CARD_DOES_NOT_FIT': 'Acea carte nu se potrivește nici la culoare, nici la valoare',
  'err.SUIT_REQUIRED': 'Spune culoarea care continuă',
  'err.MUST_ANSWER_DRAW_OR_TAKE': 'Răspunde cu un șapte, sau ia cărțile',
  'err.NOTHING_TO_DRAW': 'Nu a mai rămas nimic de tras',
  'err.PILE_EMPTY': 'Teancul este gol',
  'err.PILE_BLOCKED': 'Teancul este blocat — deasupra stă un trei negru',
  'err.PILE_FROZEN':
    'Teancul este înghețat — ai nevoie de două cărți naturale de valoarea cărții de deasupra',
  'err.TOP_CARD_UNUSABLE': 'Nu poți folosi cartea de deasupra',
  'err.MELD_CLOSED': 'Acea combinație este completă și închisă',
  'err.MELD_TOO_SMALL': 'O combinație are nevoie de mai multe cărți decât atât',
  'err.MELD_TOO_LARGE': 'Acea combinație nu mai poate primi cărți',
  'err.MELD_MIXED_RANKS': 'Toate cărțile dintr-o combinație trebuie să aibă aceeași valoare',
  'err.NOT_ENOUGH_NATURALS': 'O combinație are nevoie de mai multe cărți naturale decât jokeri',
  'err.RANK_ALREADY_MELDED': 'Tabăra ta are deja o combinație de această valoare',
  'err.NOT_YOUR_MELD': 'Acea combinație aparține taberei adverse',
  'err.NO_SUCH_MELD': 'Acea combinație nu este pe masă',
  'err.CANNOT_MELD_THREE': 'Treiurile nu se coboară niciodată',
  'err.CANNOT_DISCARD_RED_THREE': 'Un trei roșu nu poate fi aruncat',
  'err.MUST_KEEP_A_CARD': 'Păstrează cel puțin o carte — așa nu îți poți goli mâna',
  'err.MUST_MELD_FIRST': 'Coboară mai întâi deschiderea taberei tale',
  'err.INITIAL_MELD_NOT_MET': 'Primei tale coborâri îi lipsesc încă puncte',
  'err.CANNOT_GO_OUT_YET': 'Tabăra ta are nevoie de o canastă terminată înainte să poată ieși',
  'err.NOTHING_TO_CALL': 'Nu există niciun pariu de plătit',
  'err.CANNOT_CHECK': 'Nu poți verifica — există un pariu la care trebuie să răspunzi',
  'err.CANNOT_RAISE': 'Aici nu poți mări',
  'err.RAISE_TOO_SMALL': 'O mărire trebuie să fie cel puțin cât cea anterioară',
  'err.NOT_ENOUGH_CHIPS': 'Nu ai atâtea jetoane',
  'err.AMOUNT_REQUIRED': 'Spune cât',
  'err.AMOUNT_NOT_A_NUMBER': 'Acea sumă nu este un număr',
  'err.SEAT_NOT_IN_HAND': 'Nu ești în această mână',
  'err.WRONG_RANK': 'Acea carte are valoarea greșită pentru asta',
  'err.MATCH_FULL': 'Masa este plină',
  'err.MATCH_ALREADY_STARTED': 'Meciul a început deja',
  'err.TOO_FEW_PLAYERS': 'Încă nu sunt destui jucători',
  'err.WRONG_PLAYER_COUNT': 'Acest joc nu se poate juca cu atâția jucători',
  'err.NOT_THE_HOST': 'Doar gazda poate face asta',
  'err.NO_LONGER_WAITING': 'Masa nu mai așteaptă',
  'err.WAITING_ROOM_UNAVAILABLE': 'Sala de așteptare nu este disponibilă',
  'err.SERVER_BUSY': 'Serverul este plin în acest moment — încearcă din nou peste un moment',


  // --- rules the engine has always enforced and never stated ----------------
  // Every one of these was found by the guardrail rather than by review: the
  // validator could refuse a player for it, and no sentence anywhere said so.
  // See server/internal/zolikmod/ruleindex_test.go.
  'zolik.rules.section.layoff': 'Adăugarea la combinații',
  'zolik.rules.pickup.obligation':
    'Până cobori, o carte luată din teancul de aruncate trebuie folosită în combinația cu care cobori în această tură.',
  'zolik.rules.pickup.noReturn':
    'O carte luată din teancul de aruncate nu poate fi aruncată din nou în aceeași tură — joac-o sau ține-o.',
  'zolik.rules.wilds.setLimit': 'Un grup nu poate avea mai mulți jokeri decât cărți naturale.',
  'zolik.rules.set.maxSize':
    'Un grup nu poate avea mai mult de {n} cărți — jokerul înlocuiește o culoare care lipsește, nu completează un grup deja plin.',
  'zolik.rules.run.maxLength':
    'O scară nu poate avea mai mult de {n} cărți — asul jos, cele douăsprezece valori deasupra lui și asul sus.',
  'zolik.rules.run.aceBridge':
    'Asul stă deasupra popei sau sub doi, niciodată ca punte între cele două capete ale unei scări.',
  'zolik.rules.contracts.contribution':
    'Până cobori, fiecare combinație pe care o cobori trebuie să fie una pe care contractul împărțirii o mai cere.',
  'zolik.rules.layoff.afterDown':
    'Nu poți adăuga la combinațiile altora până nu ți-ai coborât propriul contract.',
  'zolik.rules.layoff.runEnds':
    'O carte adăugată la o scară trebuie să o continue la un capăt sau la celălalt.',
  'zolik.rules.jokers.swap':
    'Un joker dintr-o combinație de pe masă poate fi răscumpărat cu exact cartea pe care o reprezintă.',
  'zolik.rules.jokers.reclaim.on':
    'Un joker răscumpărat de pe masă trebuie jucat într-o combinație în aceeași tură — nu poate rămâne în mână.',
  'zolik.rules.jokers.reclaim.off': 'Un joker răscumpărat de pe masă poate rămâne în mână.',
  'zolik.rules.deck.reshuffle':
    'Când talonul se termină, teancul de aruncate este amestecat și devine noul talon; dacă amândouă sunt goale, împărțirea se încheie.',


  // --- what to do instead ---------------------------------------------------
  // The third layer of a refusal, after the reason and the rule. Sent by the
  // module because it is the only side that knows which card is owed and
  // which way out is on offer; see server/internal/zolikmod/remedy.go.
  'zolik.remedy.meldThePickup': 'Adaugă {card} la coborârea ta, sau anulează luarea.',
  'zolik.remedy.discardSomethingElse': 'Aruncă altă carte, sau joacă {card} în această tură.',
  'zolik.remedy.discardNotAJoker': 'Aruncă altceva decât un joker.',
  'zolik.remedy.finishOrUndoLayDown': 'Termină coborârea, sau ia-o înapoi.',
  'zolik.remedy.needMorePoints': 'Îți mai trebuie {n} puncte ca să poți coborî.',
  'zolik.remedy.layACleanRun': 'Coboară o scară fără niciun joker în ea.',
  'zolik.remedy.playReclaimedJoker': 'Joacă {card} într-o combinație, sau anulează luarea.',
  'zolik.remedy.goDownFirst': 'Coboară mai întâi propriile combinații.',
  'zolik.remedy.drawFirst': 'Trage mai întâi o carte.',
  'zolik.remedy.drawFromStock': 'Trage din talon — teancul de aruncate se deschide în runda {n}.',
  'zolik.remedy.drawFromStockEmpty': 'Trage din talon în schimb.',


  // --- keys whose meaning lives in their params -----------------------------
  // The shape fallback renders these with the number missing ("Stack", "Pot"),
  // which is why they earn a line where most keys do not.
  'header.contract': 'Necesită {sets} grupuri și {runs} scări',
  'header.contract.cleanRunOnly': 'Necesită o scară fără joker',
  'header.round': 'Runda {n}',
  'header.deck': 'Talon',
  'header.target': 'Țintă',
  'header.suitInPlay': 'Culoarea în joc',
  'seat.cards': 'Cărți',
  'zolik.offer.meld': 'Coboară',
  'prompt.pickupMustBeMelded':
    '{value} a venit din teancul de aruncate — trebuie să intre în combinațiile cu care cobori în această tură.',
  'prompt.jokerMustBePlayed':
    '{value} a venit de pe masă — trebuie să intre într-o combinație înainte să îți poți încheia tura.',
  'prompt.initialMeld': 'Deschiderea taberei tale trebuie să ajungă la {n} puncte.',
  'prompt.canastasNeeded': 'Taberei tale îi mai trebuie {n} canaste ca să poată ieși.',
  'prompt.mustDrawOrAnswerSeven': 'Răspunde cu un șapte, sau trage {n} cărți.',
  'prompt.chooseSuit': 'Alege culoarea care continuă',
  'prompt.skipPending': 'Rândul tău este sărit',
  'status.lastDeal': 'Echipa {team} a făcut {value}',
  'status.teamScore': 'Echipa {team}: {value}',
  'canasta.offer.rank': 'Valoare',
  'canasta.seat.teamScore': 'Scorul echipei',
  'canasta.seat.canastas': 'Canaste',
  'holdem.header.pot': 'Pot',
  'holdem.header.street': 'Stradă',
  'holdem.header.hand': 'Mână',
  'holdem.header.handLimit': 'Mâini în total',
  'holdem.header.blinds': 'Blinduri',
  'holdem.cost.call': 'pentru plată',
  'holdem.cost.pot': 'în pot',
  'holdem.seat.stack': 'Stac',
  'holdem.seat.bet': 'Pariu',
  'holdem.prompt.yourAction': 'E rândul tău',
  'holdem.prompt.raiseTo': 'Mărește la',
  'zone.yourHand': 'Mâna ta',
  'zone.opponentHand': 'Mâna adversarului',
  'zone.drawPile': 'Talon',
  'zone.discardPile': 'Teancul de aruncate',
  'zone.melds': 'Combinații',
  'zone.teamMelds': 'Combinațiile taberei tale',
  'zone.redThrees': 'Treiuri roșii',
  'zone.board': 'Masă',
  'verb.drawFromDeck': 'Trage',
  'verb.takeFromDiscard': 'Ia din teanc',


  // --- the why sheet's own furniture ---------------------------------------
  // The three layers a refusal is explained in. Labels, not sentences: what
  // goes under each is sent by the server or worded above.
  'why.reason': 'De ce nu',
  'why.rule': 'Regula',
  'why.rules': 'Regulile',
  'why.remedy': 'Ce poți face',
  'why.readTheRules': 'Citește regulile complete →',
  'why.close': 'Închide',
  'why.open': 'de ce',

  // --- marks on a particular card ------------------------------------------
  // A mark is a refusal that has not happened yet: the rule is enforced at the
  // discard, which is the last possible moment to hear about it, so the card
  // says so while there is still a turn left to act on it.
  'zolik.badge.owedToMeld':
    '{card} a venit din teancul de aruncate — trebuie să intre în combinațiile cu care cobori în această tură.',
  'zolik.badge.jokerOwed':
    '{card} a venit de pe masă — trebuie să intre într-o combinație înainte să îți poți încheia tura.',

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
  'legal.terms': 'Termeni',
  // The document's own name, used as the heading above it. Kept apart from
  // `legal.notice.terms`, which is the same words inflected to sit inside a
  // sentence — a heading in the instrumental case reads as a mistake in every
  // Slavic language.
  'legal.terms.title': 'Termeni de utilizare',
  'legal.privacy.title': 'Notă de confidențialitate',
  'legal.privacy': 'Confidențialitate',
  'legal.source': 'Cod sursă',
  'legal.updated': 'Versiunea {version}',
  'legal.draft':
    'Proiect — încă nu este în vigoare. Numele, țara și adresa de contact ale operatorului rămân de completat.',
  'legal.notice.before': 'Jucând, accepți ',
  'legal.notice.terms': 'termenii de utilizare',
  'legal.notice.between': '. Ce se păstrează despre tine este descris în ',
  'legal.notice.privacy': 'nota de confidențialitate',
  'legal.notice.after': '.',
  // --- gin rummy -------------------------------------------------------------
  'err.UPCARD_DECLINED': 'Ai refuzat deja acea carte',
  'err.DEADWOOD_TOO_HIGH': 'Deadwood-ul tău este prea mare ca să bați',
  'err.CARD_DOES_NOT_EXTEND_MELD': 'Acea carte nu prelungește această combinație',
  'ginrummy.rules.setup': 'Pregătire',
  'ginrummy.rules.turn': 'Rândul tău',
  'ginrummy.rules.melds': 'Combinații',
  'ginrummy.rules.knocking': 'Bătaia',
  'ginrummy.rules.bigGin': 'Big gin',
  'ginrummy.rules.layoff': 'Alipirea',
  'ginrummy.rules.deadHand': 'Mâna moartă',
  'ginrummy.rules.scoring': 'Punctarea unei mâini',
  'ginrummy.rules.match': 'Câștigarea meciului',
  'ginrummy.rules.lineBonuses': 'Bonusuri la socoteală',
  'ginrummy.rules.deck': 'Se joacă cu un pachet de {value} cărți.',
  'ginrummy.rules.deal': 'Fiecare jucător primește {value} cărți.',
  'ginrummy.rules.upcard': 'Încă o carte se întoarce cu fața în sus ca să înceapă teancul de aruncate.',
  'ginrummy.rules.drawDiscard':
    'La rândul tău, trage o carte — din talon sau din teancul de aruncate — apoi aruncă una.',
  'ginrummy.rules.setsAndRuns':
    'O combinație este un grup de trei sau patru cărți de o valoare, sau o scară de trei sau mai multe de aceeași culoare.',
  'ginrummy.rules.aceLow': 'Asul este întotdeauna mic — nu există scară de la damă la as.',
  'ginrummy.rules.knockLimit': 'Poți bate de îndată ce deadwood-ul tău este {n} sau mai mic.',
  'ginrummy.rules.oklahoma': 'Limita de bătaie a acestei mâini este dată de valoarea cărții întoarse.',
  'ginrummy.rules.gin': 'Deadwood zero înseamnă gin — cea mai bună bătaie posibilă.',
  'ginrummy.rules.bigGinBonus':
    'Unsprezece cărți toate în combinații, fără nicio aruncare, înseamnă big gin și aduce încă {n} puncte.',
  'ginrummy.rules.layoffDescription':
    'După o bătaie care nu este gin, adversarul tău își poate alipi propriul deadwood la combinațiile tale înainte ca mâinile să fie comparate.',
  'ginrummy.rules.deadHandDescription':
    'Dacă talonul coboară la ultimele două cărți și nimeni nu a bătut, mâna este moartă — nimeni nu punctează, iar același împărțitor împarte din nou.',
  'ginrummy.rules.undercut':
    'Dacă deadwood-ul adversarului nu e mai mare decât al tău, te taie: primește diferența, plus {n}.',
  'ginrummy.rules.ginBonus': 'Ginul aduce toată mâna adversarului tău, plus {n}.',
  'ginrummy.rules.target': 'Primul care depășește {n} puncte la sfârșitul unei mâini câștigă meciul.',
  'ginrummy.rules.shutout': 'Bonusul de meci se dublează la {n} dacă cel învins nu a marcat niciun punct.',
  'ginrummy.rules.box': 'Fiecare mână câștigată valorează {n} puncte la sfârșitul meciului.',
  'ginrummy.rules.gameBonus': 'Câștigarea meciului aduce încă {n} puncte.',
  'ginrummy.fact.deadwood': '{value} deadwood',
  'ginrummy.fact.discardCard': 'Aruncă {value}',
  'ginrummy.fact.meldCards': 'La {value}',
  'ginrummy.header.hand': 'Mâna {n}',
  'ginrummy.round.kind': '{value}',
  'ginrummy.round.hand': 'Mână',
  'ginrummy.seat.score': '{value}',
  'ginrummy.seat.dealer': 'Împărțitor',
  'ginrummy.status.knocked': '{playerId} a bătut cu {deadwood} deadwood',
  'ginrummy.status.gin': '{playerId} a făcut gin',
  'ginrummy.status.lastHand': 'Ultima mână: {winner} ({kind}, {delta} puncte)',
  'ginrummy.offer.drawStock': 'Trage din talon',
  'ginrummy.offer.drawDiscard': 'Trage din teancul de aruncate',
  'ginrummy.offer.takeUpcard': 'Ia cartea întoarsă',
  'ginrummy.offer.passUpcard': 'Pas',
  'ginrummy.offer.discard': 'Aruncă',
  'ginrummy.offer.knock': 'Bate',
  'ginrummy.offer.gin': 'Gin!',
  'ginrummy.offer.bigGin': 'Big gin!',
  'ginrummy.offer.layOff': 'Alipește',
  'ginrummy.offer.finishLayoff': 'Gata cu alipirea',
  'ginrummy.zone.knockerHand': 'Mâna celui care a bătut',
  'ginrummy.zone.melds': 'Combinații',
  'ginrummy.prompt.upcardDecision': 'Ia cartea întoarsă, sau pasează',
  'ginrummy.prompt.yourTurnDraw': 'Trage o carte',
  'ginrummy.prompt.yourTurnDiscard': 'Aruncă — sau bate, dacă poți',
  'ginrummy.prompt.layoff': 'Alipește deadwood, sau termină',

  // --- rummy tiles -------------------------------------------------------------
  'err.TILE_NOT_IN_HAND': 'Acea piesă nu este în mâna ta',
  'err.TILE_DOES_NOT_FIT': 'Aia nu încape acolo',
  'err.NO_SUCH_SET': 'Acea combinație nu este pe masă',
  'err.INITIAL_MELD_ONLY': 'Înainte de prima ta coborâre poți rearanja doar propriile combinații noi',
  'err.TABLE_NOT_VALID': 'Masa nu este încă validă',
  'err.TRAY_NOT_EMPTY': 'Mai ai piese nepuse',
  'err.NOTHING_PLAYED': 'Joacă cel puțin o piesă înainte să îți încheii tura',
  'err.INITIAL_MELD_TOO_LOW': 'Prima ta coborâre trebuie să valoreze 30 de puncte sau mai mult',
  'err.NOT_A_RUN': 'Doar o scară poate fi tăiată',
  'err.BAD_SPLIT_POSITION': 'Acolo această scară nu poate fi tăiată',
  'err.NO_JOKER_IN_SET': 'Nu există niciun joker în acea combinație',
  'err.TILE_JOKER_SWAP_MISMATCH': 'Acea piesă nu este ceea ce reprezintă jokerul',
  'rummytiles.rules.setup': 'Pregătire',
  'rummytiles.rules.sets': 'Combinații',
  'rummytiles.rules.initialMeld': 'Prima coborâre',
  'rummytiles.rules.turn': 'Rândul tău',
  'rummytiles.rules.jokerTaking': 'Luarea unui joker',
  'rummytiles.rules.ending': 'Încheierea unei runde',
  'rummytiles.rules.poolExhaustion': 'Dacă rezerva se termină',
  'rummytiles.rules.match': 'Câștigarea meciului',
  'rummytiles.rules.tiles': 'Se joacă cu {value} piese.',
  'rummytiles.rules.dealCount': 'Fiecare jucător primește {value} piese.',
  'rummytiles.rules.group': 'Un grup are trei sau patru piese cu același număr, fiecare de altă culoare.',
  'rummytiles.rules.run': 'O scară are trei sau mai multe numere consecutive de aceeași culoare.',
  'rummytiles.rules.noWrap': '13 nu se leagă înapoi la 1.',
  'rummytiles.rules.joker': 'Un joker reprezintă orice piesă.',
  'rummytiles.rules.initialMeldDescription':
    'Până când nu ai coborât {n} sau mai multe puncte într-o singură tură, doar din mâna ta, nu ai voie să atingi nimic din ce este deja pe masă.',
  'rummytiles.rules.turnDescription':
    'Joacă cel puțin o piesă din mâna ta, rearanjând masa liber, și încheie cu fiecare combinație de pe masă validă.',
  'rummytiles.rules.noDiscard':
    'Nu există aruncare — dacă nu poți încheia o tură validă, tragi în schimb o piesă.',
  'rummytiles.rules.jokerTakingDescription':
    'Un joker de pe masă poate fi luat înlocuindu-l cu piesa pe care o reprezintă, din mâna ta — și trebuie folosit într-o combinație înainte să ți se încheie tura.',
  'rummytiles.rules.goingOut':
    'Primul jucător rămas fără piese câștigă runda. Toți ceilalți primesc valoarea negativă a ceea ce le-a rămas; câștigătorul primește suma a ceea ce au pierdut toți ceilalți.',
  'rummytiles.rules.poolExhaustionLowestWins':
    'Dacă rezerva se termină și nimeni nu mai poate juca, runda se încheie și o câștigă mâna cu valoarea cea mai mică.',
  'rummytiles.rules.poolExhaustionNoWinner':
    'Dacă rezerva se termină și nimeni nu mai poate juca, runda se încheie fără câștigător — fiecare mână este pur și simplu punctată.',
  'rummytiles.rules.target': 'Primul care depășește {n} puncte la sfârșitul unei runde câștigă meciul.',
  'rummytiles.rules.roundLimit': 'Meciul se încheie după {n} runde — câștigă scorul cel mai mare.',
  'rummytiles.fact.setCards': '{value}',
  'rummytiles.header.pool': 'Rezervă {n}',
  'rummytiles.header.round': 'Runda {n}',
  'rummytiles.round.kind': '{value}',
  'rummytiles.round.round': 'Rundă',
  'rummytiles.seat.score': '{value}',
  'rummytiles.seat.notOpened': 'Nedeschis',
  'rummytiles.status.lastRound': 'Ultima rundă: {winner} ({kind})',
  'rummytiles.badge.invalid': 'Încă nevalid',
  'rummytiles.zone.pool': 'Rezervă',
  'rummytiles.zone.table': 'Masă',
  'rummytiles.zone.tray': 'Suport',
  'rummytiles.offer.place': 'Așază',
  'rummytiles.offer.addFromHand': 'Adaugă',
  'rummytiles.offer.addFromTray': 'Adaugă de pe suport',
  'rummytiles.offer.take': 'Ia',
  'rummytiles.offer.split': 'Taie',
  'rummytiles.offer.swapJoker': 'Schimbă jokerul',
  'rummytiles.offer.resetTurn': 'Resetează tura',
  'rummytiles.offer.commit': 'Gata',
  'rummytiles.offer.draw': 'Trage',
  'rummytiles.param.position': 'Taie la',

  // --- blackjack -----------------------------------------------------------
  //
  // The house rules are worded twice over — on and off — because a rule that
  // is *not* in force is still something a player has to be told. "No
  // surrender at this table" is information; silence is a guess.
  'err.BET_BELOW_MINIMUM': 'Este sub minimul mesei',
  'err.ALREADY_BET': 'Miza ta este deja pusă',
  'err.INSURANCE_CLOSED': 'Nu există asigurare de luat în acest moment',
  'err.CANNOT_DOUBLE': 'Această mână nu poate fi dublată',
  'err.CANNOT_SPLIT': 'Această mână nu poate fi despărțită',
  'err.CANNOT_SURRENDER': 'Această mână nu poate fi predată',

  'blackjack.rules.section.table': 'Masa',
  'blackjack.rules.section.play': 'Jucarea unei mâini',
  'blackjack.rules.section.dealer': 'Crupierul',
  'blackjack.rules.section.end': 'Cum se încheie meciul',
  'blackjack.rules.goal':
    'Învinge crupierul fără să treci de douăzeci și unu. Depășirea pierde imediat, orice ar face crupierul după aceea.',
  // Counts are their own phrase rather than a number glued to a noun: one
  // deck and six decks inflect differently, and Czech inflects them again.
  'blackjack.rules.decks': 'Pachete în sabot: {n}.',
  'blackjack.rules.stack': 'Fiecare loc se așază cu {n} jetoane.',
  'blackjack.rules.minBet': 'Minimul mesei este {n} jetoane.',
  'blackjack.rules.faceUp':
    'Cărțile jucătorilor se împart cu fața în sus; crupierul ține o carte acoperită până joacă toată lumea.',
  'blackjack.rules.hitStand': 'Trage câte cărți vrei, sau rămâi la ce ai.',
  'blackjack.rules.aces': 'Un as valorează unsprezece cât timp încape, și unu când nu încape.',
  'blackjack.rules.blackjack': 'Un as cu o carte de valoare zece, pe primele două cărți, este blackjack.',
  'blackjack.rules.pays3to2': 'Blackjack-ul plătește 3:2.',
  'blackjack.rules.pays6to5': 'Blackjack-ul plătește 6:5.',
  'blackjack.rules.paysEven': 'Blackjack-ul plătește unu la unu.',
  'blackjack.rules.double': 'Pe primele două cărți poți dubla miza și primești exact încă o carte.',
  'blackjack.rules.doubleAfterSplit': 'O mână ieșită dintr-o despărțire poate fi și ea dublată.',
  'blackjack.rules.noDoubleAfterSplit': 'O mână ieșită dintr-o despărțire nu poate fi dublată.',
  'blackjack.rules.split':
    'Două cărți de aceeași valoare pot fi despărțite în mâini separate, fiecare cu miza ei — de până la {n} ori, pentru {hands} mâini în total.',
  'blackjack.rules.noSplit': 'La această masă perechile nu se despart.',
  'blackjack.rules.splitAces':
    'Așii despărțiți primesc câte o carte și apoi se opresc, iar douăzeci și unu obținut astfel nu este blackjack.',
  'blackjack.rules.surrender':
    'Poți preda prima mână pentru jumătate din miză, după ce crupierul a verificat blackjack-ul.',
  'blackjack.rules.noSurrender': 'La această masă mâinile nu pot fi predate.',
  'blackjack.rules.dealerDraws': 'Crupierul trage până la șaptesprezece și apoi se oprește.',
  'blackjack.rules.hitsSoft17': 'Crupierul trage la un șaptesprezece făcut cu un as.',
  'blackjack.rules.standsSoft17': 'Crupierul se oprește la un șaptesprezece făcut cu un as.',
  'blackjack.rules.dealerPeeks':
    'Arătând un as sau un zece, crupierul verifică blackjack-ul înainte să joace cineva.',
  'blackjack.rules.insurance':
    'Împotriva unui as al crupierului te poți asigura pentru jumătate din miză; plătește 2:1 dacă crupierul are blackjack.',
  'blackjack.rules.noInsurance': 'La această masă nu se oferă asigurare.',
  'blackjack.rules.rounds': 'La masă se joacă {n} runde.',
  'blackjack.rules.mostChipsWins': 'Cine are cele mai multe jetoane la final câștigă meciul.',
  'blackjack.rules.bustedOut':
    'Un loc care nu mai poate acoperi minimul de {n} stă deoparte pentru restul meciului.',

  'blackjack.zone.dealer': 'Crupier',
  'blackjack.zone.box': 'Mână',
  'blackjack.zone.yourBox': 'Mâna ta',
  'blackjack.zone.shoe': 'Sabot',

  'blackjack.header.round': 'Runda {n} din {of}',
  'blackjack.header.minBet': 'Minim',
  'blackjack.header.decks': 'Pachete',
  'blackjack.header.dealerTotal': 'Crupierul arată {n}',
  'blackjack.header.dealerSoftTotal': 'Crupierul arată {n} moale',

  'blackjack.seat.stack': 'Jetoane',
  'blackjack.seat.bet': 'Miză',
  'blackjack.seat.insurance': 'Asigurare',
  'blackjack.seat.total': 'Total',
  'blackjack.seat.softTotal': 'Total moale',
  'blackjack.seat.out': 'Fără jetoane',

  'blackjack.prompt.placeBet': 'Pune-ți miza',
  'blackjack.prompt.insurance': 'Asigurare?',
  'blackjack.prompt.yourMove': 'E rândul tău',
  'blackjack.prompt.waitingFor': 'Se așteaptă {playerId}',
  'blackjack.prompt.betAmount': 'Miză',

  'blackjack.offer.bet': 'Pariază',
  'blackjack.offer.hit': 'Carte',
  'blackjack.offer.stand': 'Rămân',
  'blackjack.offer.double': 'Dublează',
  'blackjack.offer.split': 'Desparte',
  'blackjack.offer.surrender': 'Predă',
  'blackjack.offer.insure': 'Ia asigurare',
  'blackjack.offer.declineInsurance': 'Fără asigurare',

  'blackjack.fact.tableMinimum': 'minim',
  'blackjack.fact.insuranceCost': 'pentru asigurare',
  'blackjack.fact.extraStake': 'de mizat',
  'blackjack.fact.surrenderReturn': 'înapoi',

  'blackjack.status.dealerBlackjack': 'Crupierul avea blackjack',
  'blackjack.status.dealerBust': 'Crupierul a sărit cu {n}',
  'blackjack.status.dealerStands': 'Crupierul se oprește la {n}',

  'blackjack.round.name': 'Rundă',
  'blackjack.round.dealerTotal': 'Crupier {n}',
  'blackjack.round.dealerBust': 'Crupierul a sărit ({n})',
  'blackjack.round.dealerBlackjack': 'Blackjack-ul crupierului',
  'blackjack.round.outcome.blackjack': 'Blackjack',
  'blackjack.round.outcome.win': 'Câștigată',
  'blackjack.round.outcome.push': 'Egalitate',
  'blackjack.round.outcome.lose': 'Pierdută',
  'blackjack.round.outcome.bust': 'Sărit',
  'blackjack.round.outcome.surrender': 'Predată',

  'blackjack.badge.inPlay': 'În joc',
  'blackjack.badge.doubled': 'Dublată',
  'blackjack.badge.split': 'Despărțită',
  'blackjack.badge.blackjack': 'Blackjack',
  'blackjack.badge.bust': 'Sărit',
  'blackjack.badge.won': 'Câștigată',
  'blackjack.badge.push': 'Egalitate',
  'blackjack.badge.lost': 'Pierdută',
  'blackjack.badge.surrendered': 'Predată',

  'blackjack.unit.chips': 'jetoane',

  // --- the settings screen --------------------------------------------------
  //
  // Hardcoded English until now, which was survivable while the only other
  // language was unreachable and faintly absurd once the language picker
  // itself lived on this screen: a player who cannot read "Settings" is
  // exactly the one who came here to change it.
  'settings.title': 'Setări',
  'settings.signedInAs': 'Autentificat ca {username}',
  'settings.playingAsGuest': 'Joci ca {username} (invitat)',
  'settings.notSignedIn':
    'Nu ești autentificat — autentifică-te sau continuă ca invitat pentru a juca online.',
  'settings.subtitle': 'Cum arăți tu și cum arată masa',
  'settings.face.heading': 'Chipul tău la masă',
  'settings.face.account': 'Păstrat în contul tău, așa că te însoțește pe alt dispozitiv.',
  'settings.face.device': 'Păstrat pe acest dispozitiv. Autentifică-te ca să-l iei cu tine.',
  'settings.skin.heading': 'Aspectul mesei',
  'settings.language.heading': 'Limbă',
  'settings.language.status': 'Păstrată pe acest dispozitiv.',
  'settings.language.auto': 'Automat',
  'settings.language.auto.now': 'Urmează dispozitivul tău — acum {language}',
  'settings.legal.heading': 'Scrisul mărunt',
  'settings.legal.status': 'Cu ce ai fost de acord jucând, și ce se păstrează despre tine.',
  'settings.signIn': 'Autentificare',
  'settings.back': 'Înapoi',

  // A translated interface over an English notice is a worse lie than an
  // English interface, so the document says so itself rather than letting the
  // surrounding screen imply the text was written for this reader.
  'legal.untranslated':
    'Această notificare nu a fost încă tradusă în limba ta. Textul în engleză de mai jos este versiunea care se aplică.',

  // --- the navigation bar's screen titles ------------------------------------
  //
  // These sit in `app/_layout.tsx`, above every screen, and were the last
  // English left after the settings screen was translated — a German player
  // reading "Einstellungen" under a bar that said "Settings". A translated app
  // with an untranslated chrome bar looks like a bug, not like a limit.
  'nav.home': 'Zolik',
  'nav.emailSignIn': 'Autentificare cu e-mail',
  'nav.signingIn': 'Se autentifică',
  'nav.usernameSignIn': 'Autentificare cu nume de utilizator',
  'nav.legacyAccount': 'Cont vechi',
  'nav.guest': 'Invitat',
  'nav.account': 'Cont',
  'nav.games': 'Jocuri',
  'nav.table': 'Masa ta',
  'nav.join': 'Alătură-te unei mese',
  // The screen a shared invite link lands on while it resolves the code.
  'nav.joining': 'Se alătură',
  'nav.rules': 'Reguli',
  'nav.match': 'Meci',
  'nav.scoreTable': 'Tabel de scor',
  'nav.stats': 'Statistici',
  'nav.more': 'Mai multe',

  // --- the account menu behind the face in the corner ----------------------
  'menu.label': 'Meniul contului',
  'menu.signedIn': 'Autentificat',
  'menu.notSignedIn': 'Neautentificat',
  'menu.keepStats': 'ca să-ți păstrezi statisticile',
  'menu.signOut': 'Deconectare',
  'more.scoreTable': 'Tabel de scor offline',
  'more.stats': 'Statistici și clasament',
  'more.needsAccount': 'autentifică-te ca să folosești',
  'gate.title': 'Autentifică-te ca să folosești asta',
  'gate.body':
    'Tabelele de scor și statisticile sunt păstrate cu contul tău, așa că te însoțesc pe alt dispozitiv. Un invitat nu are unde să le păstreze.',

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
  'error.generic': 'Nu a mers',
  'error.signIn': 'Autentificarea a eșuat',
  'error.login': 'Autentificarea a eșuat',
  'error.register': 'Înregistrarea a eșuat',
  'error.sendCode': 'Nu s-a putut trimite un cod',
  'error.badCode': 'Codul acela nu a funcționat',
  'error.rulesLoad': 'Regulile nu au putut fi încărcate',
  'error.createFailed': 'Crearea a eșuat',
  'error.saveFailed': 'Salvarea a eșuat',
  'error.exportFailed': 'Exportul a eșuat',

  // --- the route that does not exist ----------------------------------------
  'notFound.title': 'Hopa!',
  'notFound.message': 'Ecranul acesta nu există.',
  'notFound.home': 'Mergi la ecranul principal!',

  // --- signing in -----------------------------------------------------------
  'auth.login.subtitle': 'Păstrează-ți statisticile pe toate dispozitivele',
  'auth.login.continueWithEmail': 'Continuă cu e-mailul',
  'auth.login.usernameInstead': 'Autentifică-te mai degrabă cu un nume de utilizator',
  'auth.email.title': 'Autentificare cu e-mail',
  'auth.email.subtitle': 'Îți trimitem un cod de unică folosință',
  'auth.email.address': 'Adresă de e-mail',
  'auth.email.send': 'Trimite codul',
  'auth.email.codeTitle': 'Introdu codul',
  'auth.email.codePlaceholder': 'Cod din 6 cifre',
  'auth.email.differentAddress': 'Folosește altă adresă',
  'auth.email.sentTo': 'Trimis la {email}',
  'auth.email.continue': 'Continuă',
  'auth.guest.title': 'Joc ca invitat',
  'auth.guest.subtitle': 'Nu e nevoie de cont',
  'auth.guest.displayName': 'Nume afișat',
  'auth.register.title': 'Creează cont',
  'auth.register.username': 'Nume de utilizator',
  'auth.register.email': 'E-mail (opțional)',
  'auth.register.password': 'Parolă',
  'auth.username.createAccount': 'Creează un cont cu nume de utilizator și parolă',
  'auth.callback.signedIn': 'Autentificat.',

  // --- the account screen ---------------------------------------------------
  'account.signInPrompt': 'Autentifică-te ca să îți administrezi contul.',
  'account.keepGames': 'Păstrează aceste jocuri',
  'account.signedInWith': 'Autentificat prin',
  'account.addMethod': 'Adaugă o metodă de autentificare',
  'account.usernameAndPassword': 'Nume de utilizator și parolă',
  'account.faceAndTable': 'Chip și aspectul mesei',
  'account.refresh': 'Reîmprospătează',
  'account.remove': 'Elimină',

  // --- the main menu --------------------------------------------------------
  'home.subtitle': 'Remi continental · {server}',
  'home.playingAs': 'Joci ca {name}',
  'home.signInPrompt': 'Autentifică-te sau continuă ca invitat ca să joci online.',
  'home.statsAndLeaderboard': 'Statistici și clasament',
  'home.play': 'Joacă',
  'home.offlineScoreTable': 'Tabel de scor offline',
  'home.signInToKeepStats': 'Autentifică-te ca să îți păstrezi statisticile',
  'home.signOut': 'Deconectare',
  'home.continueAsGuest': 'Continuă ca invitat',
  // Someone playing as a guest, marked in a list of names.
  'home.guestSuffix': '(invitat)',

  // --- the waiting room on the main menu ------------------------------------
  //
  // Counts are whole phrases per count, not a number glued to a noun — the
  // same reason `countLabel` exists. One and many are the cases the interface
  // actually produces, so they are the cases that get their own wording.
  'waiting.checking': 'Vedem cine e pe aici…',
  'waiting.youAreWaiting': 'Aștepți să joci',
  'waiting.pickedUp': 'Oricine deschide o masă te poate lua — nimeni nu are nevoie de vreun cod de la tine.',
  'waiting.othersOne': 'Mai așteaptă încă 1 jucător',
  'waiting.othersMany': 'Mai așteaptă încă {n} jucători',
  'waiting.oneWaiting': '1 jucător așteaptă să joace',
  'waiting.manyWaiting': '{n} jucători așteaptă să joace',
  'waiting.adding': 'Te adăugăm pe lista de așteptare…',
  'waiting.slowHint':
    'Dacă asta nu se termină în câteva secunde, verifică dacă adresa serverului de mai jos e accesibilă de pe acest dispozitiv.',
  'waiting.serverBusyDetail':
    'Încercarea {n}. Serverul nu acceptă momentan conexiuni noi la sala de așteptare.',
  'waiting.reconnecting': 'Conexiune pierdută — reconectare…',
  'waiting.reconnectingDetail':
    'Încercarea {n}. Se poate întâmpla dacă rețeaua dispozitivului tău s-a schimbat sau dacă serverul a repornit.',
  'waiting.tryAgain': 'Încearcă din nou acum',
  'waiting.makeAvailable': 'Fă-mă disponibil pentru joc',
  'waiting.stop': 'Nu mai aștepta',
  'waiting.noneYet':
    'Chiar acum nu așteaptă nimeni să joace. Înscrie-te pe listă și vei fi primul pe care îl vede oricine.',
  'waiting.noOthersYet': 'Nimeni altcineva nu așteaptă încă. Gazdele te văd oricum și te pot invita.',
  'waiting.server': 'Server',
  'waiting.none': 'Chiar acum nu așteaptă nimeni. Cine se face disponibil din meniul principal apare aici.',

  // --- landing on a shared invite link --------------------------------------
  'join.missingCode': 'Acelui link îi lipsește codul mesei.',
  'join.staleLink': 'Cere un link nou de la cine te-a invitat, sau alătură-te cu codul.',
  'join.enterCode': 'Introdu un cod',
  'join.backToMenu': 'Înapoi la meniu',
  'join.takingSeat': 'Îți ocupăm un loc…',
  'join.takingSeatAt': 'Îți ocupăm un loc la {game}…',

  // --- the lobby ------------------------------------------------------------
  'lobby.games.subtitle': 'Tot ce poate găzdui acest server',
  'lobby.games.bots': 'Boți',
  'lobby.games.playBot': 'Joacă împotriva unui bot',
  'lobby.games.playBots': 'Joacă împotriva a {n} boți',
  'lobby.games.openTable': 'Deschide o masă',
  'lobby.games.players': '{n} jucători',
  'lobby.games.playerRange': '{min}–{max} jucători',
  'lobby.join.placeholder': 'Cod de alăturare sau link de invitație',
  'lobby.join.needCode': 'Introdu un cod, un link sau un ID de meci',
  'lobby.games.signInFirst': 'Autentifică-te mai întâi',
  'lobby.join.action': 'Alătură-te',
  'lobby.join.waitingTitle': 'Așteptăm gazda',
  // Two whole sentences rather than one with a swapped noun: "a game of {game}"
  // does not survive a language that inflects the game's name after "of".
  'lobby.join.joinedGame': 'Te-ai alăturat unui joc de {game} — așteptăm startul',
  'lobby.join.joinedTable': 'Te-ai alăturat mesei — așteptăm startul',
  'lobby.table.addBot': 'Adaugă un bot',
  'lobby.table.start': 'Începe',
  'lobby.table.waitingForHost': 'Așteptăm ca gazda să înceapă…',

  // --- inviting someone to a table ------------------------------------------
  'invite.heading': 'Invită jucători',
  'invite.explain': 'Trimite acest link. Cine îl deschide ajunge la această masă — fără cont.',
  'invite.noAddress':
    'Acest server nu are configurată o adresă care se poate distribui, așa că folosește codul de mai jos.',
  'invite.readOutCode': 'Sau dictează codul:',
  'invite.copy': 'Copiază linkul',
  'invite.share': 'Distribuie linkul',
  'invite.copied': 'Copiat!',
  'invite.shared': 'Distribuit',

  // --- the match screen -----------------------------------------------------
  'match.waitingForTable': 'Așteptăm masa…',
  'match.waitingForPlayer': 'Așteptăm alt jucător…',
  'match.nobodyWon': 'Nu a câștigat nimeni.',
  'match.youWon': 'Ai câștigat.',
  'match.finished': 'Acest meci s-a încheiat.',
  'match.inProgress': 'Meci în desfășurare — totul e conectat și merge normal.',
  'match.controls': 'Comenzi',
  'match.over': 'Meci încheiat',
  'match.settingUp': 'Pregătim…',
  'match.playAgain': 'Joacă din nou',
  'match.backToGames': 'Înapoi la jocuri',
  'match.table': 'Masă',
  'match.opponents': 'Adversari',
  // Marks which seat is the reader's own, in a list of seats.
  'match.youSuffix': '(tu)',
  // The winner line is composed from whole sentences rather than a name glued
  // to " won." — the verb agrees with the subject in most of these languages,
  // and a suffix cannot know that.
  'match.you': 'tu',
  'match.someoneWon': '{name} a câștigat.',
  'match.wonBy': 'Câștigat de {names}.',
  'match.pausedFor': 'În pauză — așteptăm ca {name} să se reconecteze.',
  'match.results': 'Rezultate',
  'match.players': 'Jucători',
  'match.toPlay': 'la rând',

  // --- the offline score table ----------------------------------------------
  'scoring.namesHint': 'Nume separate prin virgulă (4–8 jucători)',
  'scoring.newSession': 'Sesiune nouă',
  // A worked example, not a sentence: the names are placeholders a translator
  // may localise, the shape `name:score,` is what the parser needs.
  'scoring.scoresPlaceholder': 'Ana:120,Bogdan:80,…',
  'scoring.saveRound': 'Salvează runda',
  'scoring.export': 'Exportă fișa de scor',
  'scoring.formatHint': 'Formatul punctelor: Nume:100,Nume2:50',
  'scoring.nameCountError': 'Introdu 2–8 nume de jucători separate prin virgulă',
  'scoring.session': 'Sesiune: {id}',
  'scoring.players': 'Jucători: {names}',
  'scoring.roundScores': 'Punctele rundei {n}',
  'stats.loading': 'Se încarcă…',
  // The figures could not be fetched. `{reason}` is whatever the server or the
  // network said, which is not ours to translate — the frame around it is.
  'stats.unavailable': '(indisponibil: {reason})',

  // --- statistics -----------------------------------------------------------
  'stats.title': 'Statistici și clasament',
  'stats.yours': 'Statisticile tale',
  'stats.leaderboard': 'Clasament',

  // --- a player's lifetime record, shown beside a finished match ------------
  'record.title': 'Bilanțul tău',
  'record.guest':
    'Joci ca invitat, așa că nu se ține niciun bilanț. Autentifică-te și jocurile pe care le-ai jucat deja pe acest dispozitiv — inclusiv acesta — rămân legate de contul tău.',
  'record.signInToKeep': 'Autentifică-te și păstrează-le',
  'record.failed': 'Bilanțul tău nu a putut fi încărcat acum. Meciul e înregistrat în siguranță.',
  'record.loading': 'Se încarcă…',
  'record.played': 'Jucate',
  'record.won': 'Câștigate',
  'record.lost': 'Pierdute',
  'record.winRate': 'Rata de victorii',
  'record.streak': 'Serie',
  'record.atThisGame': 'La acest joc',
  // A streak is a count with a noun, so it is a whole phrase per count for
  // the same reason `countLabel` is: "1 win" / "2 wins" is an English rule,
  // and most of these languages do not share it.
  'record.streakNone': '—',
  'record.streakWinOne': '1 victorie',
  'record.streakWinMany': '{n} victorii',
  'record.streakLossOne': '1 înfrângere',
  'record.streakLossMany': '{n} înfrângeri',

  // --- moving cards ---------------------------------------------------------
  'hand.dragHint': 'Trage o carte de-a lungul evantaiului ca să o rearanjezi, sau pe masă ca să o joci',
  'hand.moveLeft': 'La stânga',
  'hand.moveRight': 'La dreapta',
  'zone.collapseGroup': 'Restrânge acest grup',
  'zone.expandGroup': 'Arată toate cărțile din acest grup',
  'zone.dropHere': 'Lasă aici',
  'offer.pickCards': 'alege cărți pentru locul pe care l-ai atins',
  'offer.ambiguous': 'asta poate merge în mai multe locuri — alege pe masă',

  // --- the build footer -----------------------------------------------------
  'build.app': 'aplicație',
  'build.server': 'server',
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
  'option.pauseBetweenRounds': 'Pauză între runde',
  'choice.pauseBetweenRounds.1': 'Pauză',
  'choice.pauseBetweenRounds.0': 'Continuă direct',
  'option.botSkill': 'Adversari',
  'choice.botSkill.0': 'Amestecați',
  'choice.botSkill.1': 'Ușori',
  'choice.botSkill.2': 'Medii',
  'choice.botSkill.3': 'Grei',
  'option.initialMeldMinimum': 'Valoarea de deschidere',
  'choice.initialMeldMinimum.0': 'Fără',
  'option.discardDrawMinRound': 'Luare din teancul de aruncate',
  'choice.discardDrawMinRound.0': 'Deschis',
  'choice.discardDrawMinRound.2': 'Din runda 2',
  'choice.discardDrawMinRound.3': 'Din runda 3',
  'option.requireCleanRun': 'Scară fără joker',
  'choice.requireCleanRun.1': 'Obligatorie',
  'choice.requireCleanRun.0': 'Nu',
  'option.jokerReclaimMustPlay': 'Joker răscumpărat',
  'choice.jokerReclaimMustPlay.1': 'De jucat în aceeași tură',
  'choice.jokerReclaimMustPlay.0': 'Poate fi păstrat',
  'option.dealStarter': 'Cine începe',
  'choice.dealStarter.0': 'Pe rând',
  'choice.dealStarter.1': 'Începe câștigătorul',
  'variation.prsi.classic': 'Clasic',
  'option.handSize': 'Cărți împărțite',
  'variation.canasta.classic': 'Clasică',
  'variation.canasta.modern_american': 'Modern American',
  'option.targetScore': 'Scor țintă',
  'option.canastasToGoOut': 'Canaste pentru ieșire',
  'variation.holdem.freezeout': 'Freezeout',
  'variation.holdem.timed': 'Număr fix de mâini',
  'option.startingStack': 'Jetoane de start',
  'option.bigBlind': 'Blind mare',
  'option.handLimit': 'Mâini',
  'choice.handLimit.0': 'Până rămâne un singur loc',
  'variation.ginrummy.standard': 'Standard',
  'option.knockLimit': 'Limita de bătaie',
  'choice.knockLimit.0': 'Oklahoma (o stabilește cartea întoarsă)',
  'option.bigGin': 'Big gin',
  'choice.bigGin.0': 'Oprit',
  'choice.bigGin.1': 'Pornit (+25)',
  'option.lineBonuses': 'Bonusuri la socoteală',
  'choice.lineBonuses.1': 'Pornit',
  'choice.lineBonuses.0': 'Oprit',
  'variation.rummytiles.standard': 'Standard',
  'choice.targetScore.0': 'Fără',
  // Two games qualify a number the others mean plainly: Canasta's 500 is a
  // short game, Gin Rummy's and Rummy Tiles' 500 is just 500; Hold'em's
  // starting 200 is a short stack, Blackjack's 200 is just 200. Module-scoped
  // so the qualifier travels with the game that means it — a bare key would
  // hand the note to games it is false for. `scripts/check-server-labels.js`
  // is what finds these; both were shipping as English on a Czech screen.
  'choice.canasta.targetScore.500': '500 (scurtă)',
  'choice.holdem.startingStack.200': '200 (scurtă)',
  'option.roundLimit': 'Limită de runde',
  'choice.roundLimit.0': 'Fără',
  'option.poolExhaustion': 'Dacă rezerva se termină',
  'choice.poolExhaustion.1': 'Câștigă runda mâna cea mai mică',
  'choice.poolExhaustion.0': 'Nu câștigă nimeni runda',
  'variation.blackjack.single': 'Un singur pachet',
  'option.minBet': 'Minimul mesei',
  'option.rounds': 'Runde',
  'option.decks': 'Pachete',
  'option.dealerHitsSoft17': 'Crupierul la 17 moale',
  'choice.dealerHitsSoft17.0': 'Se oprește',
  'choice.dealerHitsSoft17.1': 'Trage',
  'option.blackjackPays': 'Blackjack-ul plătește',
  'choice.blackjackPays.100': 'Unu la unu',
  'option.maxSplits': 'Despărțire',
  'choice.maxSplits.0': 'Fără despărțire',
  'choice.maxSplits.1': 'O dată (două mâini)',
  'choice.maxSplits.3': 'De trei ori (patru mâini)',
  'option.doubleAfterSplit': 'Dublare după despărțire',
  'choice.doubleAfterSplit.1': 'Permisă',
  'choice.doubleAfterSplit.0': 'Nepermisă',
  'option.surrender': 'Predare',
  'choice.surrender.0': 'Oprit',
  'choice.surrender.1': 'Predare târzie',
  'option.insurance': 'Asigurare',
  'choice.insurance.1': 'Se oferă',
  'choice.insurance.0': 'Nu se oferă',

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
  'verb.add': 'Adaugă',
  'verb.bet': 'Pariază',
  'verb.call': 'Plătesc',
  'verb.check': 'Verific',
  'verb.commit': 'Gata',
  'verb.continue': 'Continuă',
  'verb.decline_insurance': 'Fără asigurare',
  'verb.discard': 'Aruncă',
  'verb.double': 'Dublează',
  'verb.draw': 'Trage',
  'verb.finish_layoff': 'Gata cu alipirea',
  'verb.fold': 'Mă retrag',
  'verb.hit': 'Carte',
  'verb.insure': 'Ia asigurare',
  'verb.knock': 'Bate',
  'verb.lay_meld': 'Coboară',
  'verb.lay_off': 'Alipește',
  'verb.pass': 'Pas',
  'verb.place': 'Așază',
  'verb.play_card': 'Joacă',
  'verb.raise': 'Măresc',
  'verb.reset_turn': 'Resetează tura',
  'verb.split': 'Desparte',
  'verb.stand': 'Rămân',
  'verb.surrender': 'Predă',
  'verb.swap_joker': 'Schimbă jokerul',
  'verb.take': 'Ia',
  'verb.take_pile': 'Ia din teanc',
  // Declared by the server and listed in `serverKeys.json`, but never worded.
  'verb.takePileFromHand': 'Ia teancul în mână',
  'verb.takePileOntoMeld': 'Ia teancul pe o combinație',
  'verb.undoDraw': 'Anulează tragerea',
  'verb.undoLayOff': 'Anulează alipirea',
  'verb.undoMeld': 'Anulează combinația',
  'verb.undoTurn': 'Anulează tura',

  // --- suits, spelled out ---------------------------------------------------
  //
  // Read aloud by a screen reader and shown where a pip would not fit. The
  // pips themselves are drawn, not written, so these are the only place the
  // suit is ever a word.
  'suit.C': 'Trefle',
  'suit.D': 'Caro',
  'suit.H': 'Cupă',
  'suit.S': 'Pică',

  // --- counters the board prints beside a number ----------------------------
  'canasta.seat.notOpened': 'Nedeschis',
  'canasta.unit.points': 'puncte',
  'ginrummy.unit.points': 'puncte',
  'holdem.seat.dealer': 'Împărțitor',
  'holdem.unit.chips': 'jetoane',
  'prsi.unit.cardsLeft': 'cărți rămase',
  'rummytiles.prompt.initialMeld': 'Prima ta coborâre trebuie să valoreze {n} puncte.',
  'rummytiles.unit.points': 'puncte',
  'zolik.unit.penalty': 'penalizare',
  'header.pileFrozen': 'Teanc înghețat',
  // The module-less form of `ginrummy.prompt.yourTurnDraw`: Žolíky and Canasta
  // send the prompt without a module prefix. Neither the Go constants nor
  // `serverKeys.json` list it — the sweep is what found it.
  'prompt.yourTurnDraw': 'Trage o carte',
};
