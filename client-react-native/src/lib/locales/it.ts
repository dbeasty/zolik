/**
 * Italian. Ramino vocabulary: gruppo for a set, scala for a run, combinazione for a meld, tallone for the stock, jolly for a joker.
 */

export const it: Record<string, string> = {
  // --- engine error codes (rules.RulesErrorCode) ---------------------------
  'err.NOT_YOUR_TURN': 'Non è il tuo turno',
  'err.WRONG_PHASE': 'Al momento non è possibile',
  'err.MUST_DRAW_FIRST': 'Pesca una carta prima di calare',
  'err.GAME_SUSPENDED': 'La partita è in pausa',
  'err.GAME_NOT_ACTIVE': 'La partita non è in corso',
  // --- runtime refusals, which are not the rules saying no -----------------
  //
  // Both were reachable long before they were worded, and a player who met
  // one was shown the code in capitals or, worse, nothing at all: an action
  // sent while the socket was down simply vanished, which reads as a legal
  // move being silently refused.
  'err.MATCH_NOT_ACTIVE': 'Il tavolo è in pausa — si aspetta che un giocatore si ricolleghi',
  'err.NOT_CONNECTED': 'Non sei collegato al tavolo — riconnessione in corso, poi riprova',
  // Why the "start the next round" control is greyed out. Without wording the
  // shell printed the code itself, so a player agreeing to go on was answered
  // with ALREADY_READY in capitals.
  'err.ALREADY_READY': 'Sei già pronto',
  'err.NOT_BETWEEN_ROUNDS': 'Il round è ancora in gioco',
  'err.NOT_AT_THIS_TABLE': 'Non sei a questo tavolo',
  // Bringing a table back that the sweeper set aside. Refusals a player
  // can actually provoke: a link to a table that is gone, one that was
  // already picked up in another tab, and a table with other people at it.
  'err.MATCH_MOVED_ON': 'Il tavolo è andato avanti — ricarica la pagina',
  'err.MATCH_NOT_ABANDONED': 'Questo tavolo non è in attesa di essere ripreso',
  'err.MATCH_NOT_FOUND': 'Questo tavolo non esiste più',
  'err.TABLE_HAS_OTHER_PLAYERS': 'Si può riprendere solo un tavolo in cui tutti gli altri sono bot',
  'err.DISCARD_LOCKED': 'La pila degli scarti è bloccata per ora',
  'err.DISCARD_PILE_EMPTY': 'La pila degli scarti è vuota',
  'err.NO_CARDS_LEFT': 'Non ci sono più carte da pescare',
  'err.ROUND_REQ_NOT_MET': 'Cala prima la tua apertura',
  'err.NEED_CLEAN_RUN': 'Ti serve una scala senza jolly sul tavolo per risultare calato',
  'err.INCOMPLETE_INITIAL_MELD': 'Completa la calata, o annullala, prima di scartare',
  'err.DISCARD_CARD_NOT_MELDED': 'La carta raccolta deve finire nella tua combinazione',
  'err.JOKER_DISCARD_FORBIDDEN': 'Un jolly non si può scartare',
  'err.NOTHING_TO_UNDO': "Non c'è nulla da annullare",
  'err.NO_JOKER_IN_MELD': 'Nessun jolly in questa combinazione',
  'err.JOKER_SWAP_MISMATCH': 'Quella carta non prende il posto del jolly',
  'err.RECLAIMED_JOKER_NOT_MELDED':
    'Il jolly ripreso dal tavolo va giocato in una combinazione in questo turno',
  'err.RUN_TOO_LONG': 'Quella scala è già alla lunghezza massima',
  'err.WRONG_RUN_END': "Quella carta prolunga l'altro capo della scala",
  'err.INVALID_MELD': 'Nessuna carta della tua mano va bene qui',
  'err.CARD_NOT_IN_HAND': 'Quella carta non è nella tua mano',
  'err.MELD_BELOW_MINIMUM': 'Alle tue combinazioni mancano ancora punti per calare',
  'err.MELD_NO_CONTRIBUTION': 'Quella combinazione non fa avanzare il tuo requisito',
  'err.TOO_MANY_WILDS': 'Troppi jolly in quella combinazione',
  'err.ADJACENT_WILDS': "Due jolly non possono stare uno accanto all'altro",
  'err.ACE_BRIDGE': 'Un asso non può fare da ponte tra re e due',

  // --- contract phrasing ---------------------------------------------------
  // Built from the structured contract the server sends (sets/runs counts),
  // never from a deal-number lookup table.
  // Whole phrases per count rather than a number plus a pluralised noun:
  // Czech inflects the noun by count (skupina / skupiny / skupin) in a way no
  // "add an s" helper survives, and the counts that actually occur are 1-3.
  // A count outside that falls back to contract.sets.n.
  'contract.sets.1': 'Un gruppo',
  'contract.sets.2': 'Due gruppi',
  'contract.sets.3': 'Tre gruppi',
  'contract.sets.n': '{n} gruppi',
  'contract.runs.1': 'Una scala',
  'contract.runs.2': 'Due scale',
  'contract.runs.3': 'Tre scale',
  'contract.runs.n': '{n} scale',
  'contract.any': 'Qualsiasi combinazione valida',
  'contract.cleanRunOnly': 'Qualsiasi mix di gruppi e scale — almeno una scala deve essere senza jolly',
  'contract.cleanRunSuffix': '{base} — una scala deve essere senza jolly',

  // --- written rules ---------------------------------------------------------
  // Full-sentence keys a "see the rules" screen renders, one per module,
  // resolved server-side against the table's actual variation and options —
  // see internal/module/rules.go and each game's rules.go. Shared section
  // titles first, then one block per module in the order the four games were
  // added.
  'zolik.rules.section.goal': 'Obiettivo',
  'zolik.rules.section.setup': 'Preparazione',
  'zolik.rules.section.turn': 'Il tuo turno',
  'zolik.rules.section.melding': 'Calare',
  'zolik.rules.section.end': 'Come finisce la partita',
  'zolik.rules.goal':
    'Sii il primo a svuotare la mano calando gruppi e scale validi, accumulando meno punti di penalità possibile nelle carte che ti restano quando qualcun altro chiude.',
  'zolik.rules.deal': 'A ogni giocatore vengono date {n} carte.',
  'zolik.rules.meldShapes':
    'Un gruppo è formato da {set}+ carte dello stesso valore; una scala da {run}+ carte consecutive dello stesso seme.',
  'zolik.rules.turn.draw': 'Nel tuo turno pesca una carta — dal tallone o dalla pila degli scarti.',
  'zolik.rules.pickup.topOnly': 'Si può prendere solo la carta in cima alla pila degli scarti.',
  'zolik.rules.pickup.anyFromPile':
    'Si può prendere qualsiasi carta della pila degli scarti, insieme a tutto ciò che le sta sopra.',
  'zolik.rules.pickup.locked': 'Dalla pila degli scarti non si può pescare prima del round {n}.',
  'zolik.rules.pickup.open': 'La pila degli scarti è aperta fin dal primo round.',
  'zolik.rules.turn.discard': 'Chiudi il tuo turno scartando una carta.',
  'zolik.rules.jokers.restricted':
    'Un jolly non si scarta mai, tranne quando è esattamente la carta che ti svuota la mano.',
  'zolik.rules.lead.rotate': 'La mano passa di un posto a ogni smazzata, indipendentemente da chi ha vinto.',
  'zolik.rules.lead.winner': 'Chi chiude apre la smazzata successiva.',
  'zolik.rules.meldFloor.on':
    'La tua prima calata deve totalizzare almeno {n} punti naturali perché tu risulti calato.',
  'zolik.rules.meldFloor.off': "Non c'è un minimo di punti sulla tua prima calata.",
  'zolik.rules.cleanRun.on':
    'Almeno una delle tue scale deve essere del tutto priva di jolly perché tu risulti calato.',
  'zolik.rules.cleanRun.off': 'Le tue scale possono usare i jolly liberamente — nessuna deve esserne priva.',
  'zolik.rules.contracts.rotating':
    'La partita dura {n} smazzate, e ogni smazzata richiede la propria combinazione di gruppi e scale.',
  'zolik.rules.contracts.static':
    'Ogni smazzata richiede la stessa combinazione: {sets} gruppi e {runs} scale.',
  'zolik.rules.end.afterDeals': 'La partita finisce dopo {n} smazzate.',
  'zolik.rules.end.atScore': 'Si continua a smazzare finché qualcuno raggiunge {n} punti — poi è finita.',

  'prsi.rules.section.goal': 'Obiettivo',
  'prsi.rules.section.setup': 'Preparazione',
  'prsi.rules.section.turn': 'Il tuo turno',
  'prsi.rules.section.special': 'Carte speciali',
  'prsi.rules.section.end': 'Come finisce la partita',
  'prsi.rules.goal': 'Sii il primo a giocare tutte le carte della tua mano.',
  'prsi.rules.deck': 'Si gioca con un mazzo da {value} carte (dal 7 in su).',
  'prsi.rules.deal': 'Ogni giocatore parte con {n} carte.',
  'prsi.rules.turn.match':
    'Gioca una carta che corrisponda al seme o al valore della carta in cima — oppure pesca se non puoi.',
  'prsi.rules.turn.draw': 'Pescare chiude il tuo turno senza giocare.',
  'prsi.rules.sevens':
    'Gioca un 7 e il giocatore successivo pesca due carte, a meno che non risponda con un 7 suo.',
  'prsi.rules.aces': 'Gioca un asso e il turno del giocatore successivo salta.',
  'prsi.rules.queens': 'Gioca una donna e indica il seme che prosegue.',
  'prsi.rules.end': 'La partita finisce nel momento in cui una mano resta vuota.',

  'canasta.rules.section.goal': 'Obiettivo',
  'canasta.rules.section.setup': 'Preparazione',
  'canasta.rules.section.melding': 'Calare',
  'canasta.rules.section.end': 'Come finisce la partita',
  'canasta.rules.goal': 'Si gioca a coppie; la prima coppia a raggiungere {n} punti vince la partita.',
  'canasta.rules.deck': 'Si gioca con {value} carte — {decks} mazzi più i jolly.',
  'canasta.rules.deal': 'A ogni giocatore vengono date {n} carte.',
  'canasta.rules.drawCount': "All'inizio del turno peschi {n} carte.",
  'canasta.rules.redThrees':
    'Un tre rosso in mano si mostra subito e vale come bonus — a meno che la tua coppia non completi mai una canasta, nel qual caso conta contro di te.',
  'canasta.rules.canasta': 'Una canasta è una combinazione di {n} o più carte dello stesso valore.',
  'canasta.rules.sequences': 'Una combinazione può anche essere una scala: tre o più carte dello stesso seme in fila, mai con un jolly tra loro.',
  'canasta.rules.samba': 'Una scala di sette carte è una samba e vale {n} punti.',
  'canasta.rules.pileAlwaysFrozen': 'La pila degli scarti resta congelata per tutta la mano: per prenderla devi abbinare la carta in cima a due carte naturali della tua mano.',
  'canasta.rules.meldFloorBands':
    'La tua prima calata deve raggiungere un minimo di punti che sale con il tuo punteggio: {negative} sotto zero, {low} fino a 1500, {mid} fino a 3000, {high} oltre.',
  'canasta.rules.meldFloorBandsFive': 'La tua prima combinazione deve raggiungere un minimo di punti che cresce con il punteggio: {negative} sotto zero, {low} fino a 1500, {mid} fino a 3000, {high} fino a 7000, {top} oltre.',
  'canasta.rules.oneCanastaToGoOut': 'Una canasta completa basta alla tua coppia per chiudere.',
  'canasta.rules.twoCanastasToGoOut': 'Alla tua coppia servono due canaste complete prima di poter chiudere.',
  'canasta.rules.end': 'Si continua a smazzare finché una coppia supera {n} punti — poi la partita è finita.',

  'holdem.rules.section.goal': 'Obiettivo',
  'holdem.rules.section.setup': 'Preparazione',
  'holdem.rules.section.betting': 'Le puntate',
  'holdem.rules.section.end': 'Come finisce la partita',
  'holdem.rules.goal':
    "Vinci fiches avendo il punto migliore allo showdown, oppure restando l'unico giocatore nel piatto.",
  'holdem.rules.stack': 'Ogni posto parte con {n} fiches.',
  'holdem.rules.blinds': 'Il piccolo buio è {sb} e il grande buio {bb}, versati prima della distribuzione.',
  'holdem.rules.streets': 'Si punta in quattro giri — prima del flop e dopo flop, turn e river.',
  'holdem.rules.showdown':
    'Chi è ancora in gioco scopre le carte; il miglior punto di cinque carte vince il piatto.',
  'holdem.rules.noLimit': 'No-limit — ogni puntata può arrivare fino a tutto il tuo stack.',
  'holdem.rules.lastPlayerStanding': 'Si gioca finché un posto non detiene tutte le fiches.',
  'holdem.rules.mostChipsWins': 'Chi ha più fiches quando il gioco si ferma vince la partita.',
  'holdem.rules.handLimit': 'Il gioco si ferma dopo {n} mani.',

  // --- header --------------------------------------------------------------
  'header.deal': 'Smazzata {n}',
  'header.gameOf': 'Partita {n} di {total}',
  'header.gameOfWithContract': 'Partita {n} di {total}: {contract}',

  // --- meld preview --------------------------------------------------------
  'preview.validSet': 'Gruppo valido',
  'preview.validRun': 'Scala valida',
  'preview.validMeld': 'Combinazione valida',
  'preview.notYet': 'Non è ancora una combinazione',
  'preview.points': '{shape} · {n} punti',
  'preview.pointsWithLaid': '{shape} · {n} + {laid} già calati = {total} punti',
  'preview.meetsFloor': '{line} (raggiunge {n} ✓)',
  'preview.needsFloor': '{line} (richiede {n} ✗)',
  'preview.becauseOf': '{line} — {reason}',

  // --- discard -------------------------------------------------------------
  // A discard with a meld staged lays that meld first; this is what the
  // player is told when the server refuses it and the whole move is rolled
  // back. {reason} is the engine's own words for the refusal.
  'discard.meldRejected': '{reason} — non è stato scartato nulla, le tue carte sono ancora pronte.',

  // --- what the current selection would send -------------------------------
  // The client's own reason a control is not ready, next to the engine's own
  // `err.*` reasons rather than a second kind of message — a control greyed
  // out for "not your turn" and one greyed out for "you picked two, this
  // takes one" should read as the same kind of thing, not one of them looking
  // broken. See `fits` in `src/lib/drops.ts`.
  'sel.tooMany.1': 'Seleziona una sola carta',
  'sel.tooMany.n': 'Seleziona al massimo {n} carte',
  'sel.needMore': 'Seleziona {n} carta/e',
  'sel.notThese': 'Quelle carte non possono andare qui',
  'sel.needsCompany': 'Quella carta ha bisogno di quelle accanto',

  // --- what a module says happened -----------------------------------------
  // The first module-sent keys in this bundle, and deliberately few. Anything
  // a module sends is still legible without an entry here — `humanise` turns
  // `holdem.seat.stack` into "Stack" — so a key earns a line only when that
  // fallback loses something. These do: what each of them means lives in its
  // *params* (who won, how much, whose cards), and a key on its own has
  // nowhere to put them. Without the entry the player read "Winner", full
  // stop, at the end of a match they had just won.
  'status.winner': 'Vinta da {winners}',
  'holdem.status.pot': '{winners} vince {amount} con {hand}',
  'holdem.status.potUncontested': '{winners} vince {amount} — tutti gli altri hanno lasciato',
  'holdem.status.shown': '{playerId} ha mostrato {value}',
  'holdem.prompt.waitingFor': 'In attesa di {playerId}',

  // How a rummy scoreboard was ordered. Both earn a line for the same reason
  // as the keys above: the number lives in the params, and `humanise` would
  // render "Deals Won" with nothing after it.
  'zolik.standing.dealsWon': 'Smazzate vinte {n}',
  'zolik.standing.inHand': 'In mano {n}',

  // --- rounds ---------------------------------------------------------------
  // What a round is called, and what one did. Each of these carries its number
  // in the params, so `humanise` alone would render a label with nothing after
  // it.
  // An instruction, not a heading. "Next round" named the thing rather than
  // asking for it, and sat above "Waiting for 1" — which reads as though the
  // player is the one waiting, when the table is waiting for them.
  'round.continue': 'Inizia il round successivo',

  // --- the flash that announces a round or a match ending ------------------
  //
  // One line and one number, held for about two seconds. Worded generically on
  // purpose: which *kind* of ending it was is the module's own fact, printed
  // underneath, and a client that phrased "went gin" itself would be a client
  // that knows a game.
  'flash.roundWon': "{winners} se l'è presa",
  'flash.roundWonYou': 'Te la sei presa',
  'flash.roundDrawn': "Non se l'è presa nessuno",
  'flash.matchOver': 'Partita finita',
  'flash.matchWon': '{winners} vince',
  'flash.matchWonYou': 'Hai vinto',
  'flash.matchDrawn': 'Non ha vinto nessuno',
  'flash.nowOn': 'ora {total}',

  'zolik.round.deal': 'Smazzata',
  'zolik.round.cleanRun': 'Una scala deve essere senza jolly',
  'canasta.round.deal': 'Smazzata',
  'canasta.round.concealed': 'Chiusura in mano coperta',
  'canasta.round.exhausted': 'Il mazzo è finito',
  'canasta.round.meldCards': 'Carte calate {n}',
  'canasta.round.canastas': 'Canaste {n}',
  'canasta.round.redThrees': 'Tre rossi {n}',
  'canasta.round.goingOut': 'Chiusura {n}',
  'canasta.round.inHand': 'Rimaste in mano {n}',
  'holdem.round.hand': 'Mano',
  'holdem.round.pot': 'Piatto {n}',
  'holdem.round.uncontested': 'Tutti gli altri hanno lasciato',
  'seat.ready': 'Pronto',
  'zolik.seat.contractMet': 'Contratto soddisfatto',
  'results.you': '(tu)',



  // --- refusals the other three games raise ---------------------------------
  // Worded here for the same reason Žolíky's are: reasonText falls back to the
  // code, and OfferBar hands it the code as that fallback, so an unworded
  // refusal reaches a player as PILE_FROZEN in capitals. serverKeys.test.ts
  // now fails the build rather than letting the next one through.
  'err.SET_TOO_LARGE': 'Un gruppo ha già tutti e quattro i semi',
  'err.DISCARD_TAKEN_CARD_FORBIDDEN': 'Non puoi scartare la carta appena presa — giocala o tienila',
  'err.CARD_DOES_NOT_FIT': 'Quella carta non corrisponde né per seme né per valore',
  'err.SUIT_REQUIRED': 'Indica il seme che prosegue',
  'err.MUST_ANSWER_DRAW_OR_TAKE': 'Rispondi con un sette, oppure prendi le carte',
  'err.NOTHING_TO_DRAW': 'Non è rimasto nulla da pescare',
  'err.PILE_EMPTY': 'La pila è vuota',
  'err.PILE_BLOCKED': "La pila è bloccata — in cima c'è un tre nero",
  'err.PILE_FROZEN': 'La pila è congelata — ti servono due carte naturali del valore della carta in cima',
  'err.TOP_CARD_UNUSABLE': 'Non puoi usare la carta in cima',
  'err.MELD_CLOSED': 'Quella combinazione è completa e chiusa',
  'err.MELD_TOO_SMALL': 'Una combinazione richiede più carte di così',
  'err.MELD_TOO_LARGE': 'Quella combinazione non può accogliere altre carte',
  'err.MELD_MIXED_RANKS': 'Tutte le carte di una combinazione devono avere lo stesso valore',
  'err.SEQUENCE_NO_WILDS': 'Una scala non può contenere jolly',
  'err.SEQUENCE_NEEDS_ONE_SUIT': 'Tutte le carte di una scala devono essere dello stesso seme',
  'err.RUN_NOT_CONSECUTIVE': 'Una scala deve essere consecutiva, senza salti',
  'err.NOT_ENOUGH_NATURALS': 'Una combinazione richiede più carte naturali che jolly',
  'err.RANK_ALREADY_MELDED': 'La tua coppia ha già una combinazione di quel valore',
  'err.NOT_YOUR_MELD': 'Quella combinazione è della coppia avversaria',
  'err.NO_SUCH_MELD': 'Quella combinazione non è sul tavolo',
  'err.CANNOT_MELD_THREE': 'I tre non si calano mai',
  'err.CANNOT_DISCARD_RED_THREE': 'Un tre rosso non si può scartare',
  'err.MUST_KEEP_A_CARD': 'Tieni almeno una carta — così non puoi svuotare la mano',
  'err.MUST_MELD_FIRST': "Cala prima l'apertura della tua coppia",
  'err.INITIAL_MELD_NOT_MET': 'Alla tua prima calata mancano ancora punti',
  'err.CANNOT_GO_OUT_YET': 'Alla tua coppia serve una canasta completa prima di poter chiudere',
  'err.NOTHING_TO_CALL': "Non c'è nessuna puntata da vedere",
  'err.CANNOT_CHECK': "Non puoi passare — c'è una puntata a cui rispondere",
  'err.CANNOT_RAISE': 'Qui non puoi rilanciare',
  'err.RAISE_TOO_SMALL': "Un rilancio deve valere almeno quanto l'ultimo",
  'err.NOT_ENOUGH_CHIPS': 'Non hai così tante fiches',
  'err.AMOUNT_REQUIRED': 'Indica quanto',
  'err.AMOUNT_NOT_A_NUMBER': "Quell'importo non è un numero",
  'err.SEAT_NOT_IN_HAND': 'Non sei in questa mano',
  'err.WRONG_RANK': 'Quella carta ha il valore sbagliato per questo',
  'err.MATCH_FULL': 'Il tavolo è pieno',
  'err.MATCH_ALREADY_STARTED': 'La partita è già iniziata',
  'err.TOO_FEW_PLAYERS': 'Non ci sono ancora abbastanza giocatori',
  'err.WRONG_PLAYER_COUNT': 'Questo gioco non si può giocare con quel numero di giocatori',
  'err.NOT_THE_HOST': "Solo l'organizzatore può farlo",
  'err.NO_LONGER_WAITING': 'Il tavolo non è più in attesa',
  'err.WAITING_ROOM_UNAVAILABLE': "La sala d'attesa non è disponibile",
  'err.SERVER_BUSY': 'Il server è pieno in questo momento — riprova tra poco',


  // --- rules the engine has always enforced and never stated ----------------
  // Every one of these was found by the guardrail rather than by review: the
  // validator could refuse a player for it, and no sentence anywhere said so.
  // See server/internal/zolikmod/ruleindex_test.go.
  'zolik.rules.section.layoff': 'Attaccare alle combinazioni',
  'zolik.rules.pickup.obligation':
    'Finché non sei calato, una carta presa dalla pila degli scarti deve essere usata nella combinazione con cui cali in questo turno.',
  'zolik.rules.pickup.noReturn':
    'Una carta presa dalla pila degli scarti non può essere riscartata nello stesso turno — giocala o tienila.',
  'zolik.rules.wilds.setLimit': 'Un gruppo non può contenere più jolly che carte naturali.',
  'zolik.rules.set.maxSize':
    'Un gruppo non può contenere più di {n} carte — un jolly sostituisce un seme mancante, non ne aggiunge a uno completo.',
  'zolik.rules.run.maxLength':
    "Una scala non può contenere più di {n} carte — l'asso in basso, i dodici valori sopra e l'asso in cima.",
  'zolik.rules.run.aceBridge':
    "L'asso sta sopra il re o sotto il due, mai a fare da ponte tra i due capi di una scala.",
  'zolik.rules.contracts.contribution':
    'Finché non sei calato, ogni combinazione che cali deve essere una di quelle che il contratto della smazzata richiede ancora.',
  'zolik.rules.layoff.afterDown':
    'Non puoi attaccare nulla alle combinazioni altrui finché non hai calato il tuo contratto.',
  'zolik.rules.layoff.runEnds': 'Una carta attaccata a una scala deve prolungarla a uno dei due capi.',
  'zolik.rules.jokers.swap':
    'Un jolly in una combinazione sul tavolo può essere riscattato con la carta esatta che rappresenta.',
  'zolik.rules.jokers.reclaim.on':
    'Un jolly riscattato dal tavolo va giocato in una combinazione nello stesso turno — non può restare in mano.',
  'zolik.rules.jokers.reclaim.off': 'Un jolly riscattato dal tavolo può restare in mano.',
  'zolik.rules.deck.reshuffle':
    'Quando il tallone finisce, la pila degli scarti viene mescolata e diventa il nuovo tallone; se sono vuoti entrambi, la smazzata finisce.',


  // --- what to do instead ---------------------------------------------------
  // The third layer of a refusal, after the reason and the rule. Sent by the
  // module because it is the only side that knows which card is owed and
  // which way out is on offer; see server/internal/zolikmod/remedy.go.
  'zolik.remedy.meldThePickup': 'Aggiungi {card} alla tua calata, oppure annulla la presa.',
  'zolik.remedy.discardSomethingElse': "Scarta un'altra carta, oppure gioca {card} in questo turno.",
  'zolik.remedy.discardNotAJoker': 'Scarta qualcosa che non sia un jolly.',
  'zolik.remedy.finishOrUndoLayDown': 'Completa la calata, oppure riprendila.',
  'zolik.remedy.needMorePoints': 'Ti servono altri {n} punti per poter calare.',
  'zolik.remedy.layACleanRun': 'Cala una scala senza jolly.',
  'zolik.remedy.playReclaimedJoker': 'Gioca {card} in una combinazione, oppure annulla la presa.',
  'zolik.remedy.goDownFirst': 'Cala prima le tue combinazioni.',
  'zolik.remedy.drawFirst': 'Pesca prima una carta.',
  'zolik.remedy.drawFromStock': 'Pesca dal tallone — la pila degli scarti si apre al round {n}.',
  'zolik.remedy.drawFromStockEmpty': 'Pesca invece dal tallone.',


  // --- keys whose meaning lives in their params -----------------------------
  // The shape fallback renders these with the number missing ("Stack", "Pot"),
  // which is why they earn a line where most keys do not.
  'header.contract': 'Richiede {sets} gruppi e {runs} scale',
  'header.contract.cleanRunOnly': 'Richiede una scala senza jolly',
  'header.round': 'Round {n}',
  'header.deck': 'Tallone',
  'header.target': 'Obiettivo',
  'header.suitInPlay': 'Seme in gioco',
  'seat.cards': 'Carte',
  'zolik.offer.meld': 'Cala',
  'prompt.pickupMustBeMelded':
    '{value} viene dalla pila degli scarti — deve finire nelle combinazioni con cui cali in questo turno.',
  'prompt.jokerMustBePlayed':
    '{value} viene dal tavolo — deve finire in una combinazione prima che tu possa chiudere il turno.',
  'prompt.initialMeld': "L'apertura della tua coppia deve raggiungere {n} punti.",
  'prompt.canastasNeeded': 'Alla tua coppia servono altre {n} canaste prima di poter chiudere.',
  'prompt.mustDrawOrAnswerSeven': 'Rispondi con un sette, oppure pesca {n} carte.',
  'prompt.chooseSuit': 'Scegli il seme che prosegue',
  'prompt.skipPending': 'Il tuo turno salta',
  'status.lastDeal': 'La squadra {team} ha fatto {value}',
  'status.teamScore': 'Squadra {team}: {value}',
  'canasta.offer.rank': 'Valore',
  'canasta.offer.sequence': 'Scala',
  'badge.naturalCanasta': 'Canasta pura',
  'badge.mixedCanasta': 'Canasta impura',
  'badge.samba': 'Samba',
  'badge.cleanRun': 'Scala pulita',
  'canasta.seat.teamScore': 'Punteggio squadra',
  'canasta.seat.canastas': 'Canaste',
  'holdem.header.pot': 'Piatto',
  'holdem.header.street': 'Giro',
  'holdem.header.hand': 'Mano',
  'holdem.header.handLimit': 'Mani in tutto',
  'holdem.header.blinds': 'Bui',
  'holdem.cost.call': 'per vedere',
  'holdem.cost.pot': 'nel piatto',
  'holdem.seat.stack': 'Stack',
  'holdem.seat.bet': 'Puntata',
  'holdem.prompt.yourAction': 'Tocca a te',
  'holdem.prompt.raiseTo': 'Rilancia a',
  'holdem.quick.halfPot': '½ Piatto',
  'holdem.quick.pot': 'Piatto',
  'holdem.quick.allIn': 'All-in',
  'zone.yourHand': 'La tua mano',
  'zone.opponentHand': 'La sua mano',
  'zone.drawPile': 'Tallone',
  'zone.discardPile': 'Pila degli scarti',
  'zone.melds': 'Combinazioni',
  'zone.teamMelds': 'Combinazioni della tua coppia',
  'zone.opponentMelds': 'Combinazioni della coppia avversaria',
  'zone.redThrees': 'Tre rossi',
  'zone.board': 'Board',
  'verb.drawFromDeck': 'Pesca',
  'verb.takeFromDiscard': 'Prendi dalla pila',


  // --- the why sheet's own furniture ---------------------------------------
  // The three layers a refusal is explained in. Labels, not sentences: what
  // goes under each is sent by the server or worded above.
  'why.reason': 'Perché no',
  'why.rule': 'La regola',
  'why.rules': 'Le regole',
  'why.remedy': 'Cosa puoi fare',
  'why.readTheRules': 'Leggi le regole complete →',
  'why.close': 'Chiudi',
  'why.open': 'perché',

  // --- marks on a particular card ------------------------------------------
  // A mark is a refusal that has not happened yet: the rule is enforced at the
  // discard, which is the last possible moment to hear about it, so the card
  // says so while there is still a turn left to act on it.
  'zolik.badge.owedToMeld':
    '{card} viene dalla pila degli scarti — deve finire nelle combinazioni con cui cali in questo turno.',
  'zolik.badge.jokerOwed':
    '{card} viene dal tavolo — deve finire in una combinazione prima che tu possa chiudere il turno.',

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
  'legal.terms': 'Condizioni',
  // The document's own name, used as the heading above it. Kept apart from
  // `legal.notice.terms`, which is the same words inflected to sit inside a
  // sentence — a heading in the instrumental case reads as a mistake in every
  // Slavic language.
  'legal.terms.title': "Condizioni d'uso",
  'legal.privacy.title': 'Informativa sulla privacy',
  'legal.privacy': 'Privacy',
  'legal.source': 'Codice sorgente',
  'legal.updated': 'Versione {version}',
  'legal.draft':
    'Bozza — non ancora in vigore. Nome, paese e indirizzo di contatto del gestore sono ancora da inserire.',
  'legal.notice.before': 'Giocando accetti le ',
  'legal.notice.terms': "condizioni d'uso",
  'legal.notice.between': ". Ciò che viene conservato su di te è indicato nell'",
  'legal.notice.privacy': 'informativa sulla privacy',
  'legal.notice.after': '.',
  // --- gin rummy -------------------------------------------------------------
  'err.UPCARD_DECLINED': 'Hai già passato su quella carta',
  'err.DEADWOOD_TOO_HIGH': 'Il tuo deadwood è troppo alto per bussare',
  'err.CARD_DOES_NOT_EXTEND_MELD': 'Quella carta non prolunga questa combinazione',
  'ginrummy.rules.setup': 'Preparazione',
  'ginrummy.rules.turn': 'Il tuo turno',
  'ginrummy.rules.melds': 'Combinazioni',
  'ginrummy.rules.knocking': 'Bussare',
  'ginrummy.rules.bigGin': 'Big gin',
  'ginrummy.rules.layoff': "L'attacco",
  'ginrummy.rules.deadHand': 'La mano morta',
  'ginrummy.rules.scoring': 'Il punteggio di una mano',
  'ginrummy.rules.match': 'Vincere la partita',
  'ginrummy.rules.lineBonuses': 'Bonus di conteggio',
  'ginrummy.rules.deck': 'Si gioca con un mazzo da {value} carte.',
  'ginrummy.rules.deal': 'A ogni giocatore vengono date {value} carte.',
  'ginrummy.rules.upcard': "Un'altra carta viene girata scoperta per iniziare la pila degli scarti.",
  'ginrummy.rules.drawDiscard':
    'Nel tuo turno pesca una carta — dal tallone o dalla pila degli scarti — e poi scartane una.',
  'ginrummy.rules.setsAndRuns':
    'Una combinazione è un gruppo di tre o quattro carte dello stesso valore, oppure una scala di tre o più carte dello stesso seme.',
  'ginrummy.rules.aceLow': "L'asso è sempre basso — non esiste la scala da donna ad asso.",
  'ginrummy.rules.knockLimit': 'Puoi bussare non appena il tuo deadwood scende a {n} o meno.',
  'ginrummy.rules.oklahoma': 'Il limite per bussare in questa mano è dato dal valore della carta scoperta.',
  'ginrummy.rules.gin': 'Deadwood zero è gin — la migliore bussata possibile.',
  'ginrummy.rules.bigGinBonus':
    'Undici carte tutte combinate, senza nemmeno scartare, è big gin, e vale altri {n} punti.',
  'ginrummy.rules.layoffDescription':
    'Dopo una bussata che non sia gin, il tuo avversario può attaccare il proprio deadwood alle tue combinazioni prima che le mani vengano confrontate.',
  'ginrummy.rules.deadHandDescription':
    'Se il tallone scende alle ultime due carte e nessuno ha bussato, la mano è morta — nessuno segna e lo stesso mazziere distribuisce di nuovo.',
  'ginrummy.rules.undercut':
    'Se il deadwood del tuo avversario non supera il tuo, ti taglia: segna la differenza, più {n}.',
  'ginrummy.rules.ginBonus': 'Il gin segna tutta la mano del tuo avversario, più {n}.',
  'ginrummy.rules.target': 'Il primo a superare {n} punti alla fine di una mano vince la partita.',
  'ginrummy.rules.shutout':
    'Il bonus di partita raddoppia a {n} se il perdente non ha segnato un solo punto.',
  'ginrummy.rules.box': 'Ogni mano vinta vale {n} punti alla fine della partita.',
  'ginrummy.rules.gameBonus': 'Vincere la partita vale altri {n} punti.',
  'ginrummy.fact.deadwood': '{value} di deadwood',
  'ginrummy.fact.discardCard': 'Scarta {value}',
  'ginrummy.fact.meldCards': 'Su {value}',
  'ginrummy.header.hand': 'Mano {n}',
  'ginrummy.round.kind': '{value}',
  'ginrummy.round.hand': 'Mano',
  'ginrummy.seat.score': '{value}',
  'ginrummy.seat.dealer': 'Mazziere',
  'ginrummy.status.knocked': '{playerId} ha bussato con {deadwood} di deadwood',
  'ginrummy.status.gin': '{playerId} ha fatto gin',
  'ginrummy.status.lastHand': 'Ultima mano: {winner} ({kind}, {delta} punti)',
  'ginrummy.offer.drawStock': 'Pesca dal tallone',
  'ginrummy.offer.drawDiscard': 'Pesca dalla pila degli scarti',
  'ginrummy.offer.takeUpcard': 'Prendi la carta scoperta',
  'ginrummy.offer.passUpcard': 'Passa',
  'ginrummy.offer.discard': 'Scarta',
  'ginrummy.offer.knock': 'Bussa',
  'ginrummy.offer.gin': 'Gin!',
  'ginrummy.offer.bigGin': 'Big gin!',
  'ginrummy.offer.layOff': 'Attacca',
  'ginrummy.offer.finishLayoff': 'Attacchi finiti',
  'ginrummy.zone.knockerHand': 'Mano che ha bussato',
  'ginrummy.zone.melds': 'Combinazioni',
  'ginrummy.prompt.upcardDecision': 'Prendi la carta scoperta, oppure passa',
  'ginrummy.prompt.yourTurnDraw': 'Pesca una carta',
  'ginrummy.prompt.yourTurnDiscard': 'Scarta — oppure bussa, se puoi',
  'ginrummy.prompt.layoff': 'Attacca il deadwood, oppure concludi',

  // --- rummy tiles -------------------------------------------------------------
  'err.TILE_NOT_IN_HAND': 'Quella tessera non è nella tua mano',
  'err.TILE_DOES_NOT_FIT': 'Lì non ci sta',
  'err.NO_SUCH_SET': 'Quella combinazione non è sul tavolo',
  'err.INITIAL_MELD_ONLY': 'Prima della tua apertura puoi riordinare solo le tue nuove combinazioni',
  'err.TABLE_NOT_VALID': 'Il tavolo non è ancora valido',
  'err.TRAY_NOT_EMPTY': 'Hai ancora tessere sciolte da piazzare',
  'err.NOTHING_PLAYED': 'Gioca almeno una tessera prima di chiudere il turno',
  'err.INITIAL_MELD_TOO_LOW': 'La tua apertura deve valere 30 punti o più',
  'err.NOT_A_RUN': 'Solo una scala può essere divisa',
  'err.BAD_SPLIT_POSITION': 'Questa scala non si divide in quel punto',
  'err.NO_JOKER_IN_SET': "In quella combinazione non c'è nessun jolly",
  'err.TILE_JOKER_SWAP_MISMATCH': 'Quella tessera non è ciò che il jolly rappresenta',
  'rummytiles.rules.setup': 'Preparazione',
  'rummytiles.rules.sets': 'Combinazioni',
  'rummytiles.rules.initialMeld': "L'apertura",
  'rummytiles.rules.turn': 'Il tuo turno',
  'rummytiles.rules.jokerTaking': 'Riprendere un jolly',
  'rummytiles.rules.ending': 'Chiudere un round',
  'rummytiles.rules.poolExhaustion': 'Se la riserva si esaurisce',
  'rummytiles.rules.match': 'Vincere la partita',
  'rummytiles.rules.tiles': 'Si gioca con {value} tessere.',
  'rummytiles.rules.dealCount': 'A ogni giocatore vengono date {value} tessere.',
  'rummytiles.rules.group':
    'Un gruppo è formato da tre o quattro tessere dello stesso numero, ciascuna di colore diverso.',
  'rummytiles.rules.run': 'Una scala è formata da tre o più numeri consecutivi dello stesso colore.',
  'rummytiles.rules.noWrap': "Il 13 non riprende dall'1.",
  'rummytiles.rules.joker': 'Un jolly sostituisce qualsiasi tessera.',
  'rummytiles.rules.initialMeldDescription':
    'Finché non hai calato {n} punti o più in un solo turno, dalla tua sola mano, non puoi toccare nulla di ciò che è già sul tavolo.',
  'rummytiles.rules.turnDescription':
    'Gioca almeno una tessera dalla tua mano, riordinando il tavolo liberamente, e chiudi con tutte le combinazioni sul tavolo valide.',
  'rummytiles.rules.noDiscard':
    'Non si scarta — se non riesci a completare un turno valido, peschi invece una tessera.',
  'rummytiles.rules.jokerTakingDescription':
    'Un jolly sul tavolo si può riprendere sostituendolo con la tessera che rappresenta, presa dalla tua mano — e va usato in una combinazione prima che il turno finisca.',
  'rummytiles.rules.goingOut':
    'Il primo giocatore rimasto senza tessere vince il round. Tutti gli altri segnano in negativo il valore di ciò che resta loro; il vincitore segna la somma di quanto hanno perso gli altri.',
  'rummytiles.rules.poolExhaustionLowestWins':
    'Se la riserva si esaurisce e nessuno può giocare, il round finisce e lo vince la mano di valore più basso.',
  'rummytiles.rules.poolExhaustionNoWinner':
    'Se la riserva si esaurisce e nessuno può giocare, il round finisce senza vincitore — ogni mano viene semplicemente conteggiata.',
  'rummytiles.rules.target': 'Il primo a superare {n} punti alla fine di un round vince la partita.',
  'rummytiles.rules.roundLimit': 'La partita finisce dopo {n} round — vince il punteggio più alto.',
  'rummytiles.fact.setCards': '{value}',
  'rummytiles.header.pool': 'Riserva {n}',
  'rummytiles.header.round': 'Round {n}',
  'rummytiles.round.kind': '{value}',
  'rummytiles.round.round': 'Round',
  'rummytiles.seat.score': '{value}',
  'rummytiles.seat.notOpened': 'Non aperto',
  'rummytiles.status.lastRound': 'Ultimo round: {winner} ({kind})',
  'rummytiles.badge.invalid': 'Non ancora valido',
  'rummytiles.zone.pool': 'Riserva',
  'rummytiles.zone.table': 'Tavolo',
  'rummytiles.zone.tray': 'Leggio',
  'rummytiles.offer.place': 'Piazza',
  'rummytiles.offer.addFromHand': 'Aggiungi',
  'rummytiles.offer.addFromTray': 'Aggiungi dal leggio',
  'rummytiles.offer.take': 'Prendi',
  'rummytiles.offer.split': 'Dividi',
  'rummytiles.offer.swapJoker': 'Scambia il jolly',
  'rummytiles.offer.resetTurn': 'Azzera il turno',
  'rummytiles.offer.commit': 'Fatto',
  'rummytiles.offer.draw': 'Pesca',
  'rummytiles.param.position': 'Dividi a',

  // --- blackjack -----------------------------------------------------------
  //
  // The house rules are worded twice over — on and off — because a rule that
  // is *not* in force is still something a player has to be told. "No
  // surrender at this table" is information; silence is a guess.
  'err.BET_BELOW_MINIMUM': 'È sotto il minimo del tavolo',
  'err.ALREADY_BET': 'La tua puntata è già sul tavolo',
  'err.INSURANCE_CLOSED': "Al momento non c'è nessuna assicurazione da prendere",
  'err.CANNOT_DOUBLE': 'Questa mano non si può raddoppiare',
  'err.CANNOT_SPLIT': 'Questa mano non si può dividere',
  'err.CANNOT_SURRENDER': 'Questa mano non si può abbandonare',

  'blackjack.rules.section.table': 'Il tavolo',
  'blackjack.rules.section.play': 'Giocare una mano',
  'blackjack.rules.section.dealer': 'Il banco',
  'blackjack.rules.section.end': 'Come finisce la partita',
  'blackjack.rules.goal':
    'Batti il banco senza superare ventuno. Chi supera perde subito, qualunque cosa faccia il banco dopo.',
  // Counts are their own phrase rather than a number glued to a noun: one
  // deck and six decks inflect differently, and Czech inflects them again.
  'blackjack.rules.decks': 'Mazzi nello shoe: {n}.',
  'blackjack.rules.stack': 'Ogni posto si siede con {n} fiches.',
  'blackjack.rules.minBet': 'Il minimo del tavolo è {n} fiches.',
  'blackjack.rules.faceUp':
    'Le carte dei giocatori si danno scoperte; il banco tiene una carta coperta finché non hanno giocato tutti.',
  'blackjack.rules.hitStand': 'Chiedi tutte le carte che vuoi, oppure stai su ciò che hai.',
  'blackjack.rules.aces': 'Un asso vale undici finché ci sta, e uno quando non ci sta più.',
  'blackjack.rules.blackjack': 'Un asso con una carta da dieci, nelle prime due carte, è blackjack.',
  'blackjack.rules.pays3to2': 'Il blackjack paga 3:2.',
  'blackjack.rules.pays6to5': 'Il blackjack paga 6:5.',
  'blackjack.rules.paysEven': 'Il blackjack paga alla pari.',
  'blackjack.rules.double':
    'Sulle tue prime due carte puoi raddoppiare la puntata e prendere esattamente una carta in più.',
  'blackjack.rules.doubleAfterSplit': 'Anche una mano nata da una divisione può essere raddoppiata.',
  'blackjack.rules.noDoubleAfterSplit': 'Una mano nata da una divisione non può essere raddoppiata.',
  'blackjack.rules.split':
    'Due carte dello stesso valore possono essere divise in mani a sé, ciascuna con la propria puntata — fino a {n} volte, per {hands} mani in tutto.',
  'blackjack.rules.noSplit': 'A questo tavolo le coppie non si dividono.',
  'blackjack.rules.splitAces':
    'Gli assi divisi ricevono una carta ciascuno e poi stanno, e il ventuno ottenuto così non è blackjack.',
  'blackjack.rules.surrender':
    'Puoi abbandonare la tua prima mano per metà della puntata, una volta che il banco ha controllato di non avere blackjack.',
  'blackjack.rules.noSurrender': 'A questo tavolo le mani non si possono abbandonare.',
  'blackjack.rules.dealerDraws': 'Il banco tira fino a diciassette e poi sta.',
  'blackjack.rules.hitsSoft17': 'Il banco tira su un diciassette formato con un asso.',
  'blackjack.rules.standsSoft17': 'Il banco sta su un diciassette formato con un asso.',
  'blackjack.rules.dealerPeeks':
    'Se mostra un asso o un dieci, il banco controlla di avere blackjack prima che giochi chiunque.',
  'blackjack.rules.insurance':
    'Contro un asso del banco puoi assicurarti per metà della puntata; paga 2:1 se il banco ha blackjack.',
  'blackjack.rules.noInsurance': "A questo tavolo non si offre l'assicurazione.",
  'blackjack.rules.rounds': 'Al tavolo si giocano {n} round.',
  'blackjack.rules.mostChipsWins': 'Chi ha più fiches alla fine vince la partita.',
  'blackjack.rules.bustedOut':
    'Il posto che non riesce più a coprire il minimo di {n} resta fuori per il resto della partita.',

  'blackjack.zone.dealer': 'Banco',
  'blackjack.zone.box': 'Mano',
  'blackjack.zone.yourBox': 'La tua mano',
  'blackjack.zone.shoe': 'Shoe',

  'blackjack.header.round': 'Round {n} di {of}',
  'blackjack.header.minBet': 'Minimo',
  'blackjack.header.decks': 'Mazzi',
  'blackjack.header.dealerTotal': 'Il banco mostra {n}',
  'blackjack.header.dealerSoftTotal': 'Il banco mostra {n} morbido',

  'blackjack.seat.stack': 'Fiches',
  'blackjack.seat.bet': 'Puntata',
  'blackjack.seat.insurance': 'Assicurazione',
  'blackjack.seat.total': 'Totale',
  'blackjack.seat.softTotal': 'Totale morbido',
  'blackjack.seat.out': 'Senza fiches',

  'blackjack.prompt.placeBet': 'Fai la tua puntata',
  'blackjack.prompt.insurance': 'Assicurazione?',
  'blackjack.prompt.yourMove': 'Tocca a te',
  'blackjack.prompt.waitingFor': 'In attesa di {playerId}',
  'blackjack.prompt.betAmount': 'Puntata',

  'blackjack.quick.doubleMin': '2× Minimo',
  'blackjack.quick.allIn': 'All-in',
  'blackjack.offer.bet': 'Punta',
  'blackjack.offer.hit': 'Carta',
  'blackjack.offer.stand': 'Sto',
  'blackjack.offer.double': 'Raddoppia',
  'blackjack.offer.split': 'Dividi',
  'blackjack.offer.surrender': 'Abbandona',
  'blackjack.offer.insure': 'Assicurati',
  'blackjack.offer.declineInsurance': 'Niente assicurazione',

  'blackjack.fact.tableMinimum': 'minimo',
  'blackjack.fact.insuranceCost': 'per assicurare',
  'blackjack.fact.extraStake': 'da puntare',
  'blackjack.fact.surrenderReturn': 'indietro',

  'blackjack.status.dealerBlackjack': 'Il banco aveva blackjack',
  'blackjack.status.dealerBust': 'Il banco ha sballato con {n}',
  'blackjack.status.dealerStands': 'Il banco sta a {n}',

  'blackjack.round.name': 'Round',
  'blackjack.round.dealerTotal': 'Banco {n}',
  'blackjack.round.dealerBust': 'Banco sballato ({n})',
  'blackjack.round.dealerBlackjack': 'Blackjack del banco',
  'blackjack.round.outcome.blackjack': 'Blackjack',
  'blackjack.round.outcome.win': 'Vinta',
  'blackjack.round.outcome.push': 'Pari',
  'blackjack.round.outcome.lose': 'Persa',
  'blackjack.round.outcome.bust': 'Sballato',
  'blackjack.round.outcome.surrender': 'Abbandonata',

  'blackjack.badge.inPlay': 'In gioco',
  'blackjack.badge.doubled': 'Raddoppiata',
  'blackjack.badge.split': 'Divisa',
  'blackjack.badge.blackjack': 'Blackjack',
  'blackjack.badge.bust': 'Sballato',
  'blackjack.badge.won': 'Vinta',
  'blackjack.badge.push': 'Pari',
  'blackjack.badge.lost': 'Persa',
  'blackjack.badge.surrendered': 'Abbandonata',

  'blackjack.unit.chips': 'fiches',

  // --- the settings screen --------------------------------------------------
  //
  // Hardcoded English until now, which was survivable while the only other
  // language was unreachable and faintly absurd once the language picker
  // itself lived on this screen: a player who cannot read "Settings" is
  // exactly the one who came here to change it.
  'settings.title': 'Impostazioni',
  'settings.signedInAs': 'Accesso effettuato come {username}',
  'settings.playingAsGuest': 'Stai giocando come {username} (ospite)',
  'settings.notSignedIn': 'Non hai effettuato l’accesso — accedi o continua come ospite per giocare online.',
  'settings.subtitle': 'Come appari tu, e come appare il tavolo',
  'settings.face.heading': 'La tua faccia al tavolo',
  'settings.face.account': 'Conservata con il tuo account, così ti segue su un altro dispositivo.',
  'settings.face.device': 'Conservata su questo dispositivo. Accedi per portarla con te.',
  'settings.skin.heading': 'Aspetto del tavolo',
  'settings.language.heading': 'Lingua',
  'settings.language.status': 'Conservata su questo dispositivo.',
  'settings.language.auto': 'Automatica',
  'settings.language.auto.now': 'Segue il tuo dispositivo — ora {language}',
  'settings.legal.heading': 'Le clausole',
  'settings.legal.status': 'Cosa hai accettato giocando, e cosa viene conservato su di te.',
  'settings.signIn': 'Accedi',
  'settings.back': 'Indietro',

  // A translated interface over an English notice is a worse lie than an
  // English interface, so the document says so itself rather than letting the
  // surrounding screen imply the text was written for this reader.
  'legal.untranslated':
    'Questo avviso non è ancora stato tradotto nella tua lingua. Il testo inglese qui sotto è la versione che fa fede.',

  // --- the navigation bar's screen titles ------------------------------------
  //
  // These sit in `app/_layout.tsx`, above every screen, and were the last
  // English left after the settings screen was translated — a German player
  // reading "Einstellungen" under a bar that said "Settings". A translated app
  // with an untranslated chrome bar looks like a bug, not like a limit.
  'nav.home': 'Zolik',
  'nav.emailSignIn': 'Accesso con e-mail',
  'nav.signingIn': 'Accesso in corso',
  'nav.usernameSignIn': 'Accesso con nome utente',
  'nav.legacyAccount': 'Account precedente',
  'nav.guest': 'Ospite',
  'nav.account': 'Account',
  'nav.games': 'Giochi',
  'nav.table': 'Il tuo tavolo',
  'nav.join': 'Unisciti a un tavolo',
  // The screen a shared invite link lands on while it resolves the code.
  'nav.joining': 'Ingresso in corso',
  'nav.rules': 'Regole',
  'nav.match': 'Partita',
  'nav.scoreTable': 'Tabella punteggi',
  'nav.stats': 'Statistiche',
  'nav.more': 'Altro',

  // --- the account menu behind the face in the corner ----------------------
  'menu.label': 'Menu account',
  'menu.signedIn': 'Accesso effettuato',
  'menu.notSignedIn': 'Accesso non effettuato',
  'menu.keepStats': 'per conservare le tue statistiche',
  'menu.signOut': 'Esci',
  'more.scoreTable': 'Tabella punti offline',
  'more.stats': 'Statistiche e classifica',
  'more.needsAccount': 'accedi per usare',
  'gate.title': 'Accedi per usare questo',
  'gate.body':
    'Le tabelle punti e le statistiche sono conservate con il tuo account, così ti seguono su un altro dispositivo. Un ospite non ha dove tenerle.',

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
  'error.generic': 'Non ha funzionato',
  'error.signIn': 'Accesso non riuscito',
  'error.login': 'Accesso non riuscito',
  'error.register': 'Registrazione non riuscita',
  'error.sendCode': 'Non è stato possibile inviare un codice',
  'error.badCode': 'Quel codice non ha funzionato',
  'error.rulesLoad': 'Non è stato possibile caricare le regole',
  'error.createFailed': 'Creazione non riuscita',
  'error.saveFailed': 'Salvataggio non riuscito',
  'error.exportFailed': 'Esportazione non riuscita',

  // --- the route that does not exist ----------------------------------------
  'notFound.title': 'Ops!',
  'notFound.message': 'Questa schermata non esiste.',
  'notFound.home': 'Vai alla schermata iniziale!',

  // --- signing in -----------------------------------------------------------
  'auth.login.subtitle': 'Conserva le tue statistiche su tutti i dispositivi',
  'auth.login.continueWithEmail': "Continua con l'e-mail",
  'auth.login.usernameInstead': 'Accedi invece con un nome utente',
  'auth.email.title': 'Accesso con e-mail',
  'auth.email.subtitle': 'Ti invieremo un codice usa e getta',
  'auth.email.address': 'Indirizzo e-mail',
  'auth.email.send': 'Invia il codice',
  'auth.email.codeTitle': 'Inserisci il codice',
  'auth.email.codePlaceholder': 'Codice a 6 cifre',
  'auth.email.differentAddress': 'Usa un altro indirizzo',
  'auth.email.sentTo': 'Inviato a {email}',
  'auth.email.continue': 'Continua',
  'auth.guest.title': 'Gioco da ospite',
  'auth.guest.subtitle': 'Nessun account richiesto',
  'auth.guest.displayName': 'Nome visualizzato',
  'auth.register.title': 'Crea un account',
  'auth.register.username': 'Nome utente',
  'auth.register.email': 'E-mail (facoltativa)',
  'auth.register.password': 'Password',
  'auth.username.createAccount': 'Crea un account con nome utente e password',
  'auth.callback.signedIn': 'Accesso effettuato.',

  // --- the account screen ---------------------------------------------------
  'account.signInPrompt': 'Accedi per gestire il tuo account.',
  'account.keepGames': 'Conserva queste partite',
  'account.signedInWith': 'Accesso effettuato con',
  'account.addMethod': 'Aggiungi un metodo di accesso',
  'account.usernameAndPassword': 'Nome utente e password',
  'account.faceAndTable': 'Faccia e aspetto del tavolo',
  'account.refresh': 'Aggiorna',
  'account.remove': 'Rimuovi',

  // --- the main menu --------------------------------------------------------
  'home.subtitle': 'Ramino continentale · {server}',
  'home.playingAs': 'Giochi come {name}',
  'home.signInPrompt': 'Accedi o continua da ospite per giocare online.',
  'home.statsAndLeaderboard': 'Statistiche e classifica',
  'home.play': 'Gioca',
  'home.offlineScoreTable': 'Segnapunti offline',
  'home.signInToKeepStats': 'Accedi per conservare le statistiche',
  'home.signOut': 'Esci',
  'home.continueAsGuest': 'Continua da ospite',
  // Someone playing as a guest, marked in a list of names.
  'home.guestSuffix': '(ospite)',

  // --- the waiting room on the main menu ------------------------------------
  //
  // Counts are whole phrases per count, not a number glued to a noun — the
  // same reason `countLabel` exists. One and many are the cases the interface
  // actually produces, so they are the cases that get their own wording.
  'waiting.checking': "Vediamo chi c'è…",
  'waiting.youAreWaiting': 'Stai aspettando di giocare',
  'waiting.pickedUp': 'Chiunque apra un tavolo può prenderti — non serve nessun codice da parte tua.',
  'waiting.othersOne': '1 altro giocatore sta aspettando',
  'waiting.othersMany': '{n} altri giocatori stanno aspettando',
  'waiting.oneWaiting': '1 giocatore sta aspettando di giocare',
  'waiting.manyWaiting': '{n} giocatori stanno aspettando di giocare',
  'waiting.adding': "Ti stiamo aggiungendo alla lista d'attesa…",
  'waiting.slowHint':
    "Se non finisce in qualche secondo, controlla che l'indirizzo del server qui sotto sia raggiungibile da questo dispositivo.",
  'waiting.serverBusyDetail':
    "Tentativo {n}. Al momento il server non accetta nuove connessioni alla sala d'attesa.",
  'waiting.reconnecting': 'Connessione persa — riconnessione…',
  'waiting.reconnectingDetail':
    'Tentativo {n}. Può succedere se la rete del tuo dispositivo è cambiata, o se il server è stato riavviato.',
  'waiting.tryAgain': 'Riprova ora',
  'waiting.makeAvailable': 'Rendimi disponibile a giocare',
  'waiting.stop': 'Smetti di aspettare',
  'waiting.noneYet':
    'Al momento non sta aspettando nessuno. Mettiti in lista e sarai il primo che chiunque vedrà.',
  'waiting.noOthersYet':
    'Non sta ancora aspettando nessun altro. Gli organizzatori ti vedono lo stesso e possono invitarti.',
  'waiting.server': 'Server',
  'waiting.none':
    'Al momento non sta aspettando nessuno. Chi si rende disponibile dal menu principale compare qui.',

  // --- landing on a shared invite link --------------------------------------
  'join.missingCode': 'A quel link manca il codice del tavolo.',
  'join.staleLink': 'Chiedi un link nuovo a chi ti ha invitato, oppure entra con il codice.',
  'join.enterCode': 'Inserisci un codice',
  'join.backToMenu': 'Torna al menu',
  'join.takingSeat': 'Ti stiamo facendo sedere…',
  'join.takingSeatAt': 'Ti stiamo facendo sedere a {game}…',

  // --- the lobby ------------------------------------------------------------
  'lobby.games.subtitle': 'Tutto ciò che questo server può ospitare',
  'lobby.games.bots': 'Bot',
  'lobby.games.playBot': 'Gioca contro un bot',
  'lobby.games.playBots': 'Gioca contro {n} bot',
  'lobby.games.openTable': 'Apri un tavolo',
  'lobby.games.players': '{n} giocatori',
  'lobby.games.playerRange': '{min}–{max} giocatori',
  'lobby.join.placeholder': "Codice o link d'invito",
  'lobby.join.needCode': 'Inserisci un codice, un link o un ID partita',
  'lobby.games.signInFirst': 'Accedi prima',
  'lobby.join.action': 'Entra',
  'lobby.join.waitingTitle': "In attesa dell'organizzatore",
  // Two whole sentences rather than one with a swapped noun: "a game of {game}"
  // does not survive a language that inflects the game's name after "of".
  'lobby.join.joinedGame': "Sei entrato in una partita di {game} — in attesa dell'inizio",
  'lobby.join.joinedTable': "Sei entrato al tavolo — in attesa dell'inizio",
  'lobby.table.addBot': 'Aggiungi un bot',
  'lobby.table.side': 'Squadra {n}',
  'lobby.table.shuffleSeats': 'Mescola i posti',
  'lobby.table.moveSeatUp': 'Sposta {name} su di un posto',
  'lobby.table.moveSeatDown': 'Sposta {name} giù di un posto',
  'lobby.table.start': 'Inizia',
  'lobby.table.waitingForHost': "In attesa che l'organizzatore inizi…",

  // --- inviting someone to a table ------------------------------------------
  'invite.heading': 'Invita giocatori',
  'invite.explain': 'Manda questo link. Chi lo apre arriva a questo tavolo — senza bisogno di account.',
  'invite.noAddress':
    'Per questo server non è configurato un indirizzo condivisibile, quindi usa il codice qui sotto.',
  'invite.readOutCode': 'Oppure detta il codice:',
  'invite.copy': 'Copia il link',
  'invite.share': 'Condividi il link',
  'invite.copied': 'Copiato!',
  'invite.shared': 'Condiviso',

  // --- the match screen -----------------------------------------------------
  'match.waitingForTable': 'In attesa del tavolo…',
  'match.waitingForPlayer': 'In attesa di un altro giocatore…',
  'match.nobodyWon': 'Non ha vinto nessuno.',
  'match.youWon': 'Hai vinto.',
  'match.finished': 'Questa partita è finita.',
  'match.inProgress': 'Partita in corso — è tutto connesso e funziona normalmente.',
  'match.connecting': 'Connessione…',
  'match.abandonedTitle': 'Tavolo messo da parte',
  'match.abandoned': 'Nessuno è tornato a questo tavolo, così è stato messo da parte. Le carte sono esattamente dove le hai lasciate.',
  'match.resume': 'Riprendi da dove eri rimasto',
  'match.resuming': 'Recupero del tavolo…',
  'match.controls': 'Comandi',
  'match.over': 'Partita finita',
  'match.settingUp': 'Preparazione…',
  'match.playAgain': 'Gioca ancora',
  'match.backToGames': 'Torna ai giochi',
  'match.table': 'Tavolo',
  'match.opponents': 'Avversari',
  // Marks which seat is the reader's own, in a list of seats.
  'match.youSuffix': '(tu)',
  // The winner line is composed from whole sentences rather than a name glued
  // to " won." — the verb agrees with the subject in most of these languages,
  // and a suffix cannot know that.
  'match.you': 'tu',
  'match.someoneWon': '{name} ha vinto.',
  'match.wonBy': 'Vinta da {names}.',
  'match.pausedFor': 'In pausa — in attesa che {name} si ricolleghi.',
  'match.results': 'Risultati',
  'match.players': 'Giocatori',
  'match.toPlay': 'al turno',

  // --- the offline score table ----------------------------------------------
  'scoring.namesHint': 'Nomi separati da virgole (4–8 giocatori)',
  'scoring.newSession': 'Nuova sessione',
  // A worked example, not a sentence: the names are placeholders a translator
  // may localise, the shape `name:score,` is what the parser needs.
  'scoring.scoresPlaceholder': 'Anna:120,Bruno:80,…',
  'scoring.saveRound': 'Salva la mano',
  'scoring.export': 'Esporta il segnapunti',
  'scoring.formatHint': 'Formato dei punteggi: Nome:100,Nome2:50',
  'scoring.nameCountError': 'Inserisci da 2 a 8 nomi separati da virgole',
  'scoring.session': 'Sessione: {id}',
  'scoring.players': 'Giocatori: {names}',
  'scoring.roundScores': 'Punteggi della mano {n}',
  'stats.loading': 'Caricamento…',
  // The figures could not be fetched. `{reason}` is whatever the server or the
  // network said, which is not ours to translate — the frame around it is.
  'stats.unavailable': '(non disponibile: {reason})',

  // --- statistics -----------------------------------------------------------
  'stats.title': 'Statistiche e classifica',
  'stats.yours': 'Le tue statistiche',
  'stats.leaderboard': 'Classifica',

  // --- a player's lifetime record, shown beside a finished match ------------
  'record.title': 'Il tuo score',
  'record.guest':
    'Stai giocando da ospite, quindi non viene tenuto alcuno score. Accedi e le partite che hai già giocato su questo dispositivo — questa compresa — resteranno legate al tuo account.',
  'record.signInToKeep': 'Accedi e conservali',
  'record.failed':
    'Non è stato possibile caricare il tuo score adesso. La partita è registrata senza problemi.',
  'record.loading': 'Caricamento…',
  'record.played': 'Giocate',
  'record.won': 'Vinte',
  'record.lost': 'Perse',
  'record.winRate': 'Percentuale di vittorie',
  'record.streak': 'Serie',
  'record.atThisGame': 'A questo gioco',
  // A streak is a count with a noun, so it is a whole phrase per count for
  // the same reason `countLabel` is: "1 win" / "2 wins" is an English rule,
  // and most of these languages do not share it.
  'record.streakNone': '—',
  'record.streakWinOne': '1 vittoria',
  'record.streakWinMany': '{n} vittorie',
  'record.streakLossOne': '1 sconfitta',
  'record.streakLossMany': '{n} sconfitte',

  // --- moving cards ---------------------------------------------------------
  'hand.dragHint': 'Trascina una carta lungo il ventaglio per riordinarla, o sul tavolo per giocarla',
  'hand.moveLeft': 'Sposta a sinistra',
  'hand.moveRight': 'Sposta a destra',
  'zone.collapseGroup': 'Comprimi questo gruppo',
  'zone.expandGroup': 'Mostra tutte le carte di questo gruppo',
  'zone.dropHere': 'Lascia qui',
  'offer.pickCards': 'scegli le carte per il posto che hai toccato',
  'offer.ambiguous': 'può andare in più di un posto — scegli sul tavolo',

  // --- the build footer -----------------------------------------------------
  'build.app': 'app',
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
  'option.pauseBetweenRounds': 'Pausa tra le mani',
  'choice.pauseBetweenRounds.1': 'Pausa',
  'choice.pauseBetweenRounds.0': 'Prosegui subito',
  'option.botSkill': 'Avversari',
  'choice.botSkill.0': 'Misti',
  'choice.botSkill.1': 'Facile',
  'choice.botSkill.2': 'Medio',
  'choice.botSkill.3': 'Difficile',
  'option.initialMeldMinimum': "Valore d'apertura",
  'choice.initialMeldMinimum.0': 'Nessuno',
  'option.discardDrawMinRound': 'Presa dagli scarti',
  'choice.discardDrawMinRound.0': 'Aperta',
  'choice.discardDrawMinRound.2': 'Dal round 2',
  'choice.discardDrawMinRound.3': 'Dal round 3',
  'option.requireCleanRun': 'Scala senza jolly',
  'choice.requireCleanRun.1': 'Obbligatoria',
  'choice.requireCleanRun.0': 'No',
  'option.jokerReclaimMustPlay': 'Jolly riscattato',
  'choice.jokerReclaimMustPlay.1': 'Da giocare nello stesso turno',
  'choice.jokerReclaimMustPlay.0': 'Si può tenere',
  'option.dealStarter': 'Primo di mano',
  'choice.dealStarter.0': 'A turno',
  'choice.dealStarter.1': 'Apre chi vince',
  'variation.prsi.classic': 'Classico',
  'option.handSize': 'Carte distribuite',
  'variation.canasta.classic': 'Classica',
  'variation.canasta.modern_american': 'Modern American',
  'variation.canasta.samba': 'Samba',
  'option.targetScore': 'Punteggio obiettivo',
  'option.canastasToGoOut': 'Canaste per chiudere',
  'variation.holdem.freezeout': 'Freezeout',
  'variation.holdem.timed': 'Numero fisso di mani',
  'option.startingStack': 'Fiches iniziali',
  'option.bigBlind': 'Buio grande',
  'option.handLimit': 'Mani',
  'choice.handLimit.0': 'Finché resta un solo posto',
  'variation.ginrummy.standard': 'Standard',
  'option.knockLimit': 'Limite per bussare',
  'choice.knockLimit.0': 'Oklahoma (lo fissa la carta scoperta)',
  'option.bigGin': 'Big gin',
  'choice.bigGin.0': 'No',
  'choice.bigGin.1': 'Sì (+25)',
  'option.lineBonuses': 'Bonus di conteggio',
  'choice.lineBonuses.1': 'Sì',
  'choice.lineBonuses.0': 'No',
  'variation.rummytiles.standard': 'Standard',
  'choice.targetScore.0': 'Nessuno',
  // Two games qualify a number the others mean plainly: Canasta's 500 is a
  // short game, Gin Rummy's and Rummy Tiles' 500 is just 500; Hold'em's
  // starting 200 is a short stack, Blackjack's 200 is just 200. Module-scoped
  // so the qualifier travels with the game that means it — a bare key would
  // hand the note to games it is false for. `scripts/check-server-labels.js`
  // is what finds these; both were shipping as English on a Czech screen.
  'choice.canasta.targetScore.500': '500 (breve)',
  'choice.holdem.startingStack.200': '200 (breve)',
  'option.roundLimit': 'Limite di round',
  'choice.roundLimit.0': 'Nessuno',
  'option.poolExhaustion': 'Se la riserva si esaurisce',
  'choice.poolExhaustion.1': 'Vince il round la mano più bassa',
  'choice.poolExhaustion.0': 'Nessuno vince il round',
  'variation.blackjack.single': 'Un mazzo',
  'option.minBet': 'Minimo del tavolo',
  'option.rounds': 'Round',
  'option.decks': 'Mazzi',
  'option.dealerHitsSoft17': 'Banco sul 17 morbido',
  'choice.dealerHitsSoft17.0': 'Sta',
  'choice.dealerHitsSoft17.1': 'Tira',
  'option.blackjackPays': 'Il blackjack paga',
  'choice.blackjackPays.100': 'Alla pari',
  'option.maxSplits': 'Divisione',
  'choice.maxSplits.0': 'Nessuna divisione',
  'choice.maxSplits.1': 'Una volta (due mani)',
  'choice.maxSplits.3': 'Tre volte (quattro mani)',
  'option.doubleAfterSplit': 'Raddoppio dopo divisione',
  'choice.doubleAfterSplit.1': 'Consentito',
  'choice.doubleAfterSplit.0': 'Non consentito',
  'option.surrender': 'Abbandono',
  'choice.surrender.0': 'No',
  'choice.surrender.1': 'Abbandono tardivo',
  'option.insurance': 'Assicurazione',
  'choice.insurance.1': 'Offerta',
  'choice.insurance.0': 'Non offerta',

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
  'verb.add': 'Aggiungi',
  'verb.bet': 'Punta',
  'verb.call': 'Vedo',
  'verb.check': 'Passo',
  'verb.commit': 'Fatto',
  'verb.continue': 'Continua',
  'verb.decline_insurance': 'Niente assicurazione',
  'verb.discard': 'Scarta',
  'verb.double': 'Raddoppia',
  'verb.draw': 'Pesca',
  'verb.finish_layoff': 'Attacchi finiti',
  'verb.fold': 'Lascio',
  'verb.hit': 'Carta',
  'verb.insure': 'Assicurati',
  'verb.knock': 'Bussa',
  'verb.lay_meld': 'Cala',
  'verb.lay_off': 'Attacca',
  'verb.pass': 'Passa',
  'verb.place': 'Piazza',
  'verb.play_card': 'Gioca',
  'verb.raise': 'Rilancio',
  'verb.reset_turn': 'Azzera il turno',
  'verb.split': 'Dividi',
  'verb.stand': 'Sto',
  'verb.surrender': 'Abbandona',
  'verb.swap_joker': 'Scambia il jolly',
  'verb.take': 'Prendi',
  'verb.take_pile': 'Prendi dalla pila',
  // Declared by the server and listed in `serverKeys.json`, but never worded.
  'verb.takePileFromHand': 'Prendi la pila in mano',
  'verb.takePileOntoMeld': 'Prendi la pila su una combinazione',
  'verb.takeTopForSequence': 'Prendere la carta in cima su una scala',
  'verb.undoDraw': 'Annulla la pescata',
  'verb.undoLayOff': "Annulla l'attacco",
  'verb.undoMeld': 'Annulla la combinazione',
  'verb.undoTakePile': 'Annulla la presa dalla pila',
  'verb.undoTurn': 'Annulla il turno',

  // --- suits, spelled out ---------------------------------------------------
  //
  // Read aloud by a screen reader and shown where a pip would not fit. The
  // pips themselves are drawn, not written, so these are the only place the
  // suit is ever a word.
  'suit.C': 'Fiori',
  'suit.D': 'Quadri',
  'suit.H': 'Cuori',
  'suit.S': 'Picche',

  // --- counters the board prints beside a number ----------------------------
  'canasta.seat.notOpened': 'Non aperto',
  'canasta.unit.points': 'punti',
  'ginrummy.unit.points': 'punti',
  'holdem.seat.dealer': 'Mazziere',
  'holdem.seat.folded': 'Fuori mano',
  'holdem.seat.allIn': 'All in',
  'holdem.seat.out': 'Eliminato',
  'holdem.unit.chips': 'fiches',
  'prsi.unit.cardsLeft': 'carte rimaste',
  'rummytiles.prompt.initialMeld': 'La tua prima calata deve valere {n} punti.',
  'rummytiles.unit.points': 'punti',
  'zolik.unit.penalty': 'penalità',
  'header.pileFrozen': 'Pila congelata',
  // The module-less form of `ginrummy.prompt.yourTurnDraw`: Žolíky and Canasta
  // send the prompt without a module prefix. Neither the Go constants nor
  // `serverKeys.json` list it — the sweep is what found it.
  'prompt.yourTurnDraw': 'Pesca una carta',
  'prompt.yourTurnMeld': 'Combina se puoi, poi scarta',
};
