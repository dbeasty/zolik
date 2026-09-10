/**
 * Spanish. Rummy vocabulary: grupo for a set, escalera for a run, combinación for a meld, comodín for a joker.
 */

export const es: Record<string, string> = {
  // --- engine error codes (rules.RulesErrorCode) ---------------------------
  'err.NOT_YOUR_TURN': 'No es tu turno',
  'err.WRONG_PHASE': 'Ahora mismo no se puede',
  'err.MUST_DRAW_FIRST': 'Roba una carta antes de bajarte',
  'err.GAME_SUSPENDED': 'La partida está en pausa',
  'err.GAME_NOT_ACTIVE': 'La partida no está en marcha',
  // --- runtime refusals, which are not the rules saying no -----------------
  //
  // Both were reachable long before they were worded, and a player who met
  // one was shown the code in capitals or, worse, nothing at all: an action
  // sent while the socket was down simply vanished, which reads as a legal
  // move being silently refused.
  'err.MATCH_NOT_ACTIVE': 'La mesa está en pausa — se espera a que un jugador vuelva a conectarse',
  'err.NOT_CONNECTED': 'Sin conexión con la mesa — reconectando; inténtalo después',
  // Why the "start the next round" control is greyed out. Without wording the
  // shell printed the code itself, so a player agreeing to go on was answered
  // with ALREADY_READY in capitals.
  'err.ALREADY_READY': 'Ya estás listo',
  'err.NOT_BETWEEN_ROUNDS': 'La ronda sigue en juego',
  'err.NOT_AT_THIS_TABLE': 'No estás en esta mesa',
  // Bringing a table back that the sweeper set aside. Refusals a player
  // can actually provoke: a link to a table that is gone, one that was
  // already picked up in another tab, and a table with other people at it.
  'err.MATCH_MOVED_ON': 'La mesa ha cambiado — vuelve a cargar la página',
  'err.MATCH_NOT_ABANDONED': 'Esta mesa no está esperando a que la retomes',
  'err.MATCH_NOT_FOUND': 'Esta mesa ya no existe',
  'err.TABLE_HAS_OTHER_PLAYERS': 'Solo se puede retomar una mesa en la que todos los demás sean bots',
  'err.DISCARD_LOCKED': 'El montón de descarte está bloqueado por ahora',
  'err.DISCARD_PILE_EMPTY': 'El montón de descarte está vacío',
  'err.NO_CARDS_LEFT': 'No quedan cartas para robar',
  'err.ROUND_REQ_NOT_MET': 'Baja primero tu propia combinación inicial',
  'err.NEED_CLEAN_RUN': 'Necesitas una escalera sin comodines en la mesa para contar como bajado',
  'err.INCOMPLETE_INITIAL_MELD': 'Termina tu bajada, o deshazla, antes de descartar',
  'err.DISCARD_CARD_NOT_MELDED': 'La carta que cogiste debe entrar en tu combinación',
  'err.JOKER_DISCARD_FORBIDDEN': 'Un comodín no se puede descartar',
  'err.NOTHING_TO_UNDO': 'No hay nada que deshacer',
  'err.NO_JOKER_IN_MELD': 'No hay comodín en esta combinación',
  'err.JOKER_SWAP_MISMATCH': 'Esa carta no ocupa el lugar del comodín',
  'err.RECLAIMED_JOKER_NOT_MELDED':
    'El comodín que retiraste de la mesa debe jugarse en una combinación este turno',
  'err.RUN_TOO_LONG': 'Esa escalera ya tiene su longitud máxima',
  'err.WRONG_RUN_END': 'Esa carta alarga el otro extremo de la escalera',
  'err.INVALID_MELD': 'Ninguna carta de tu mano encaja aquí',
  'err.CARD_NOT_IN_HAND': 'Esa carta no está en tu mano',
  'err.MELD_BELOW_MINIMUM': 'A tus combinaciones aún les faltan puntos para poder bajarte',
  'err.MELD_NO_CONTRIBUTION': 'Esa combinación no avanza tu requisito',
  'err.TOO_MANY_WILDS': 'Demasiados comodines en esa combinación',
  'err.ADJACENT_WILDS': 'Dos comodines no pueden ir juntos',
  'err.ACE_BRIDGE': 'Un as no puede enlazar el rey con el dos',

  // --- contract phrasing ---------------------------------------------------
  // Built from the structured contract the server sends (sets/runs counts),
  // never from a deal-number lookup table.
  // Whole phrases per count rather than a number plus a pluralised noun:
  // Czech inflects the noun by count (skupina / skupiny / skupin) in a way no
  // "add an s" helper survives, and the counts that actually occur are 1-3.
  // A count outside that falls back to contract.sets.n.
  'contract.sets.1': 'Un grupo',
  'contract.sets.2': 'Dos grupos',
  'contract.sets.3': 'Tres grupos',
  'contract.sets.n': '{n} grupos',
  'contract.runs.1': 'Una escalera',
  'contract.runs.2': 'Dos escaleras',
  'contract.runs.3': 'Tres escaleras',
  'contract.runs.n': '{n} escaleras',
  'contract.any': 'Cualquier combinación válida',
  'contract.cleanRunOnly':
    'Cualquier mezcla de grupos y escaleras — al menos una escalera debe ir sin comodines',
  'contract.cleanRunSuffix': '{base} — una escalera debe ir sin comodines',

  // --- written rules ---------------------------------------------------------
  // Full-sentence keys a "see the rules" screen renders, one per module,
  // resolved server-side against the table's actual variation and options —
  // see internal/module/rules.go and each game's rules.go. Shared section
  // titles first, then one block per module in the order the four games were
  // added.
  'zolik.rules.section.goal': 'Objetivo',
  'zolik.rules.section.setup': 'Preparación',
  'zolik.rules.section.turn': 'Tu turno',
  'zolik.rules.section.melding': 'Bajarse',
  'zolik.rules.section.end': 'Cómo termina la partida',
  'zolik.rules.goal':
    'Sé el primero en vaciar tu mano bajando grupos y escaleras válidos, acumulando los menos puntos de penalización posibles en las cartas que aún tengas cuando otro se vaya.',
  'zolik.rules.deal': 'Cada jugador recibe {n} cartas.',
  'zolik.rules.meldShapes':
    'Un grupo son {set}+ cartas del mismo valor; una escalera son {run}+ cartas consecutivas del mismo palo.',
  'zolik.rules.turn.draw': 'En tu turno, roba una carta — del mazo o del montón de descarte.',
  'zolik.rules.pickup.topOnly': 'Solo puede cogerse la carta superior del montón de descarte.',
  'zolik.rules.pickup.anyFromPile':
    'Puede cogerse cualquier carta del montón de descarte, junto con todo lo que tenga encima.',
  'zolik.rules.pickup.locked': 'No se puede robar del montón de descarte hasta la ronda {n}.',
  'zolik.rules.pickup.open': 'El montón de descarte está abierto desde la primera ronda.',
  'zolik.rules.turn.discard': 'Termina tu turno descartando una carta.',
  'zolik.rules.jokers.restricted':
    'Un comodín nunca puede descartarse, salvo como la carta exacta que vacía tu mano.',
  'zolik.rules.lead.rotate': 'La mano avanza un asiento en cada reparto, sin importar quién ganó.',
  'zolik.rules.lead.winner': 'Quien se va sale primero en el siguiente reparto.',
  'zolik.rules.meldFloor.on':
    'Tu primera bajada debe sumar al menos {n} puntos naturales antes de que cuentes como bajado.',
  'zolik.rules.meldFloor.off': 'No hay un mínimo de puntos en tu primera bajada.',
  'zolik.rules.cleanRun.on':
    'Al menos una de tus escaleras debe ir completamente sin comodines para que cuentes como bajado.',
  'zolik.rules.cleanRun.off':
    'Tus escaleras pueden usar comodines libremente — ninguna tiene que ir sin ellos.',
  'zolik.rules.contracts.rotating':
    'La partida dura {n} repartos, y cada reparto exige su propia combinación de grupos y escaleras.',
  'zolik.rules.contracts.static':
    'Cada reparto exige la misma combinación: {sets} grupos y {runs} escaleras.',
  'zolik.rules.end.afterDeals': 'La partida termina tras {n} repartos.',
  'zolik.rules.end.atScore': 'Se sigue repartiendo hasta que alguien llega a {n} puntos — entonces se acaba.',

  'prsi.rules.section.goal': 'Objetivo',
  'prsi.rules.section.setup': 'Preparación',
  'prsi.rules.section.turn': 'Tu turno',
  'prsi.rules.section.special': 'Cartas especiales',
  'prsi.rules.section.end': 'Cómo termina la partida',
  'prsi.rules.goal': 'Sé el primero en jugar todas las cartas de tu mano.',
  'prsi.rules.deck': 'Se juega con una baraja de {value} cartas (del 7 en adelante).',
  'prsi.rules.deal': 'Cada jugador empieza con {n} cartas.',
  'prsi.rules.turn.match':
    'Juega una carta que coincida en palo o valor con la de encima — o roba si no puedes.',
  'prsi.rules.turn.draw': 'Robar termina tu turno sin jugar carta.',
  'prsi.rules.sevens':
    'Juega un 7 y el siguiente jugador roba dos cartas, salvo que responda con un 7 propio.',
  'prsi.rules.aces': 'Juega un as y el siguiente jugador pierde su turno.',
  'prsi.rules.queens': 'Juega una dama y di el palo que continúa.',
  'prsi.rules.end': 'La partida termina en cuanto una mano se queda vacía.',

  'canasta.rules.section.goal': 'Objetivo',
  'canasta.rules.section.setup': 'Preparación',
  'canasta.rules.section.melding': 'Bajarse',
  'canasta.rules.section.end': 'Cómo termina la partida',
  'canasta.rules.goal': 'Se juega por parejas; el primer bando en llegar a {n} puntos gana la partida.',
  'canasta.rules.deck': 'Se juega con {value} cartas — {decks} barajas más comodines.',
  'canasta.rules.deal': 'Cada jugador recibe {n} cartas.',
  'canasta.rules.drawCount': 'Robas {n} cartas al principio de tu turno.',
  'canasta.rules.redThrees':
    'Un tres rojo en tu mano se muestra al momento y puntúa como bonificación — salvo que tu bando no complete ninguna canasta, en cuyo caso cuenta en tu contra.',
  'canasta.rules.canasta': 'Una canasta es una combinación de {n} o más cartas del mismo valor.',
  'canasta.rules.sequences': 'Una combinación también puede ser una escalera: tres o más cartas del mismo palo seguidas, nunca con un comodín entre ellas.',
  'canasta.rules.samba': 'Una escalera de siete cartas es una samba y vale {n} puntos.',
  'canasta.rules.pileAlwaysFrozen': 'El montón de descarte está congelado toda la mano: para llevártelo tienes que casar su carta superior con dos cartas naturales de tu mano.',
  'canasta.rules.meldFloorBands':
    'Tu primera bajada debe alcanzar un mínimo de puntos que sube con tu marcador: {negative} por debajo de cero, {low} hasta 1500, {mid} hasta 3000, {high} más allá.',
  'canasta.rules.meldFloorBandsFive': 'Tu primera combinación debe alcanzar un mínimo de puntos que sube con tu puntuación: {negative} por debajo de cero, {low} hasta 1500, {mid} hasta 3000, {high} hasta 7000 y {top} por encima.',
  'canasta.rules.oneCanastaToGoOut': 'Una canasta completa basta para que tu bando se vaya.',
  'canasta.rules.twoCanastasToGoOut': 'Tu bando necesita dos canastas completas antes de poder irse.',
  'canasta.rules.end':
    'Se sigue repartiendo hasta que un bando pasa de {n} puntos — entonces la partida se acaba.',

  'holdem.rules.section.goal': 'Objetivo',
  'holdem.rules.section.setup': 'Preparación',
  'holdem.rules.section.betting': 'Las apuestas',
  'holdem.rules.section.end': 'Cómo termina la partida',
  'holdem.rules.goal':
    'Gana fichas teniendo la mejor mano en el showdown, o quedándote como único jugador en la mano.',
  'holdem.rules.stack': 'Cada asiento empieza con {n} fichas.',
  'holdem.rules.blinds': 'La ciega pequeña es {sb} y la grande {bb}, puestas antes de repartir.',
  'holdem.rules.streets': 'Se apuesta en cuatro rondas — antes del flop y tras el flop, el turn y el river.',
  'holdem.rules.showdown':
    'Quienes siguen en la mano muestran sus cartas; la mejor mano de cinco cartas se lleva el bote.',
  'holdem.rules.noLimit': 'Sin límite — cualquier apuesta puede llegar hasta la totalidad de tu pila.',
  'holdem.rules.lastPlayerStanding': 'Se juega hasta que un asiento tiene todas las fichas.',
  'holdem.rules.mostChipsWins': 'Quien tenga más fichas al detenerse el juego gana la partida.',
  'holdem.rules.handLimit': 'El juego se detiene tras {n} manos.',

  // --- header --------------------------------------------------------------
  'header.deal': 'Reparto {n}',
  'header.gameOf': 'Juego {n} de {total}',
  'header.gameOfWithContract': 'Juego {n} de {total}: {contract}',

  // --- meld preview --------------------------------------------------------
  'preview.validSet': 'Grupo válido',
  'preview.validRun': 'Escalera válida',
  'preview.validMeld': 'Combinación válida',
  'preview.notYet': 'Todavía no es combinación',
  'preview.points': '{shape} · {n} puntos',
  'preview.pointsWithLaid': '{shape} · {n} + {laid} ya bajados = {total} puntos',
  'preview.meetsFloor': '{line} (llega a {n} ✓)',
  'preview.needsFloor': '{line} (necesita {n} ✗)',
  'preview.becauseOf': '{line} — {reason}',

  // --- discard -------------------------------------------------------------
  // A discard with a meld staged lays that meld first; this is what the
  // player is told when the server refuses it and the whole move is rolled
  // back. {reason} is the engine's own words for the refusal.
  'discard.meldRejected': '{reason} — no se descartó nada, tus cartas siguen preparadas.',

  // --- what the current selection would send -------------------------------
  // The client's own reason a control is not ready, next to the engine's own
  // `err.*` reasons rather than a second kind of message — a control greyed
  // out for "not your turn" and one greyed out for "you picked two, this
  // takes one" should read as the same kind of thing, not one of them looking
  // broken. See `fits` in `src/lib/drops.ts`.
  'sel.tooMany.1': 'Elige solo una carta',
  'sel.tooMany.n': 'Elige como mucho {n} cartas',
  'sel.needMore': 'Elige {n} carta(s)',
  'sel.notThese': 'Esas cartas no pueden ir aquí',
  'sel.needsCompany': 'Esa carta necesita las de al lado',

  // --- what a module says happened -----------------------------------------
  // The first module-sent keys in this bundle, and deliberately few. Anything
  // a module sends is still legible without an entry here — `humanise` turns
  // `holdem.seat.stack` into "Stack" — so a key earns a line only when that
  // fallback loses something. These do: what each of them means lives in its
  // *params* (who won, how much, whose cards), and a key on its own has
  // nowhere to put them. Without the entry the player read "Winner", full
  // stop, at the end of a match they had just won.
  'status.winner': 'Ganada por {winners}',
  'holdem.status.pot': '{winners} gana {amount} con {hand}',
  'holdem.status.potUncontested': '{winners} gana {amount} — todos los demás se retiraron',
  'holdem.status.shown': '{playerId} mostró {value}',
  'holdem.prompt.waitingFor': 'Esperando a {playerId}',

  // How a rummy scoreboard was ordered. Both earn a line for the same reason
  // as the keys above: the number lives in the params, and `humanise` would
  // render "Deals Won" with nothing after it.
  'zolik.standing.dealsWon': 'Repartos ganados {n}',
  'zolik.standing.inHand': 'En la mano {n}',

  // --- rounds ---------------------------------------------------------------
  // What a round is called, and what one did. Each of these carries its number
  // in the params, so `humanise` alone would render a label with nothing after
  // it.
  // An instruction, not a heading. "Next round" named the thing rather than
  // asking for it, and sat above "Waiting for 1" — which reads as though the
  // player is the one waiting, when the table is waiting for them.
  'round.continue': 'Empezar la siguiente ronda',

  // --- the flash that announces a round or a match ending ------------------
  //
  // One line and one number, held for about two seconds. Worded generically on
  // purpose: which *kind* of ending it was is the module's own fact, printed
  // underneath, and a client that phrased "went gin" itself would be a client
  // that knows a game.
  'flash.roundWon': '{winners} se la llevó',
  'flash.roundWonYou': 'Te la llevaste',
  'flash.roundDrawn': 'No se la llevó nadie',
  'flash.matchOver': 'Partida terminada',
  'flash.matchWon': '{winners} gana',
  'flash.matchWonYou': 'Ganas tú',
  'flash.matchDrawn': 'No gana nadie',
  'flash.nowOn': 'ahora {total}',

  'zolik.round.deal': 'Reparto',
  'zolik.round.cleanRun': 'Una escalera debe ir sin comodines',
  'canasta.round.deal': 'Reparto',
  'canasta.round.concealed': 'Se fue en mano cerrada',
  'canasta.round.exhausted': 'Se acabó la baraja',
  'canasta.round.meldCards': 'Cartas bajadas {n}',
  'canasta.round.canastas': 'Canastas {n}',
  'canasta.round.redThrees': 'Treses rojos {n}',
  'canasta.round.goingOut': 'Irse {n}',
  'canasta.round.inHand': 'Pillado en la mano {n}',
  'holdem.round.hand': 'Mano',
  'holdem.round.pot': 'Bote {n}',
  'holdem.round.uncontested': 'Todos los demás se retiraron',
  'seat.ready': 'Listo',
  'zolik.seat.contractMet': 'Contrato cumplido',
  'results.you': '(tú)',



  // --- refusals the other three games raise ---------------------------------
  // Worded here for the same reason Žolíky's are: reasonText falls back to the
  // code, and OfferBar hands it the code as that fallback, so an unworded
  // refusal reaches a player as PILE_FROZEN in capitals. serverKeys.test.ts
  // now fails the build rather than letting the next one through.
  'err.SET_TOO_LARGE': 'Un grupo ya tiene los cuatro palos',
  'err.DISCARD_TAKEN_CARD_FORBIDDEN': 'No puedes descartar la carta que acabas de coger — júgala o quédatela',
  'err.CARD_DOES_NOT_FIT': 'Esa carta no coincide ni en palo ni en valor',
  'err.SUIT_REQUIRED': 'Di el palo que continúa',
  'err.MUST_ANSWER_DRAW_OR_TAKE': 'Responde con un siete, o llévate las cartas',
  'err.NOTHING_TO_DRAW': 'No queda nada que robar',
  'err.PILE_EMPTY': 'El montón está vacío',
  'err.PILE_BLOCKED': 'El montón está bloqueado — hay un tres negro encima',
  'err.PILE_FROZEN':
    'El montón está congelado — necesitas dos cartas naturales del valor de la carta superior',
  'err.TOP_CARD_UNUSABLE': 'No puedes usar la carta superior',
  'err.MELD_CLOSED': 'Esa combinación está completa y cerrada',
  'err.MELD_TOO_SMALL': 'Una combinación necesita más cartas que eso',
  'err.MELD_TOO_LARGE': 'Esa combinación no admite más cartas',
  'err.MELD_MIXED_RANKS': 'Todas las cartas de una combinación deben tener el mismo valor',
  'err.SEQUENCE_NO_WILDS': 'Una escalera no puede llevar comodines',
  'err.SEQUENCE_NEEDS_ONE_SUIT': 'Todas las cartas de una escalera deben ser del mismo palo',
  'err.RUN_NOT_CONSECUTIVE': 'Una escalera debe ir seguida, sin huecos',
  'err.NOT_ENOUGH_NATURALS': 'Una combinación necesita más cartas naturales que comodines',
  'err.RANK_ALREADY_MELDED': 'Tu bando ya tiene una combinación de ese valor',
  'err.NOT_YOUR_MELD': 'Esa combinación es del bando contrario',
  'err.NO_SUCH_MELD': 'Esa combinación no está en la mesa',
  'err.CANNOT_MELD_THREE': 'Los treses nunca se bajan',
  'err.CANNOT_DISCARD_RED_THREE': 'Un tres rojo no se puede descartar',
  'err.MUST_KEEP_A_CARD': 'Quédate al menos una carta — así no puedes vaciar tu mano',
  'err.MUST_MELD_FIRST': 'Baja primero la combinación inicial de tu bando',
  'err.INITIAL_MELD_NOT_MET': 'A tu primera bajada aún le faltan puntos',
  'err.CANNOT_GO_OUT_YET': 'Tu bando necesita una canasta completa antes de poder irse',
  'err.NOTHING_TO_CALL': 'No hay ninguna apuesta que igualar',
  'err.CANNOT_CHECK': 'No puedes pasar — hay una apuesta que responder',
  'err.CANNOT_RAISE': 'Aquí no puedes subir',
  'err.RAISE_TOO_SMALL': 'Una subida tiene que ser al menos igual a la anterior',
  'err.NOT_ENOUGH_CHIPS': 'No tienes tantas fichas',
  'err.AMOUNT_REQUIRED': 'Di cuánto',
  'err.AMOUNT_NOT_A_NUMBER': 'Esa cantidad no es un número',
  'err.SEAT_NOT_IN_HAND': 'No estás en esta mano',
  'err.WRONG_RANK': 'Esa carta tiene el valor equivocado para esto',
  'err.MATCH_FULL': 'La mesa está llena',
  'err.MATCH_ALREADY_STARTED': 'La partida ya ha empezado',
  'err.TOO_FEW_PLAYERS': 'Todavía no hay jugadores suficientes',
  'err.WRONG_PLAYER_COUNT': 'Este juego no puede jugarse con ese número de jugadores',
  'err.NOT_THE_HOST': 'Eso solo puede hacerlo el anfitrión',
  'err.NO_LONGER_WAITING': 'La mesa ya no está esperando',
  'err.WAITING_ROOM_UNAVAILABLE': 'La sala de espera no está disponible',
  'err.SERVER_BUSY': 'El servidor está lleno ahora mismo — inténtalo en un momento',


  // --- rules the engine has always enforced and never stated ----------------
  // Every one of these was found by the guardrail rather than by review: the
  // validator could refuse a player for it, and no sentence anywhere said so.
  // See server/internal/zolikmod/ruleindex_test.go.
  'zolik.rules.section.layoff': 'Añadir a combinaciones',
  'zolik.rules.pickup.obligation':
    'Antes de estar bajado, una carta cogida del montón de descarte debe usarse en la combinación con la que te bajas este turno.',
  'zolik.rules.pickup.noReturn':
    'Una carta cogida del montón de descarte no puede volver a descartarse en el mismo turno — júgala o quédatela.',
  'zolik.rules.wilds.setLimit': 'Un grupo no puede llevar más comodines que cartas naturales.',
  'zolik.rules.set.maxSize':
    'Un grupo no puede llevar más de {n} cartas — un comodín cubre un palo que falta, no rellena uno completo.',
  'zolik.rules.run.maxLength':
    'Una escalera no puede llevar más de {n} cartas — el as abajo, los doce valores por encima y el as arriba.',
  'zolik.rules.run.aceBridge':
    'El as se coloca sobre el rey o bajo el dos, nunca enlazando los dos extremos de una escalera.',
  'zolik.rules.contracts.contribution':
    'Hasta que estés bajado, cada combinación que bajes debe ser una que el contrato del reparto siga pidiendo.',
  'zolik.rules.layoff.afterDown':
    'No puedes añadir nada a las combinaciones ajenas hasta haber bajado tu propio contrato.',
  'zolik.rules.layoff.runEnds': 'Una carta añadida a una escalera debe continuarla por uno u otro extremo.',
  'zolik.rules.jokers.swap':
    'Un comodín de una combinación en la mesa puede recuperarse con la carta exacta a la que sustituye.',
  'zolik.rules.jokers.reclaim.on':
    'Un comodín recuperado de la mesa debe jugarse en una combinación en el mismo turno — no puede quedarse en la mano.',
  'zolik.rules.jokers.reclaim.off': 'Un comodín recuperado de la mesa puede quedarse en la mano.',
  'zolik.rules.deck.reshuffle':
    'Cuando se acaba el mazo, el montón de descarte se baraja y pasa a ser el nuevo mazo; si ambos están vacíos, el reparto termina.',


  // --- what to do instead ---------------------------------------------------
  // The third layer of a refusal, after the reason and the rule. Sent by the
  // module because it is the only side that knows which card is owed and
  // which way out is on offer; see server/internal/zolikmod/remedy.go.
  'zolik.remedy.meldThePickup': 'Añade {card} a tu bajada, o deshaz la recogida.',
  'zolik.remedy.discardSomethingElse': 'Descarta otra carta, o juega {card} este turno.',
  'zolik.remedy.discardNotAJoker': 'Descarta algo que no sea un comodín.',
  'zolik.remedy.finishOrUndoLayDown': 'Termina tu bajada, o recógela.',
  'zolik.remedy.needMorePoints': 'Te faltan {n} puntos para poder bajarte.',
  'zolik.remedy.layACleanRun': 'Baja una escalera sin ningún comodín.',
  'zolik.remedy.playReclaimedJoker': 'Juega {card} en una combinación, o deshaz su recuperación.',
  'zolik.remedy.goDownFirst': 'Baja primero tus propias combinaciones.',
  'zolik.remedy.drawFirst': 'Roba una carta primero.',
  'zolik.remedy.drawFromStock': 'Roba del mazo — el montón de descarte se abre en la ronda {n}.',
  'zolik.remedy.drawFromStockEmpty': 'Roba del mazo en su lugar.',


  // --- keys whose meaning lives in their params -----------------------------
  // The shape fallback renders these with the number missing ("Stack", "Pot"),
  // which is why they earn a line where most keys do not.
  'header.contract': 'Requiere {sets} grupos y {runs} escaleras',
  'header.contract.cleanRunOnly': 'Requiere una escalera sin comodines',
  'header.round': 'Ronda {n}',
  'header.deck': 'Mazo',
  'header.target': 'Objetivo',
  'header.suitInPlay': 'Palo en juego',
  'seat.cards': 'Cartas',
  'zolik.offer.meld': 'Bajar',
  'prompt.pickupMustBeMelded':
    '{value} vino del montón de descarte — tiene que entrar en las combinaciones con las que te bajas este turno.',
  'prompt.jokerMustBePlayed':
    '{value} vino de la mesa — tiene que entrar en una combinación antes de que puedas terminar tu turno.',
  'prompt.initialMeld': 'La combinación inicial de tu bando debe llegar a {n} puntos.',
  'prompt.canastasNeeded': 'A tu bando le faltan {n} canastas para poder irse.',
  'prompt.mustDrawOrAnswerSeven': 'Responde con un siete, o roba {n} cartas.',
  'prompt.chooseSuit': 'Elige el palo que continúa',
  'prompt.skipPending': 'Pierdes tu turno',
  'status.lastDeal': 'El equipo {team} anotó {value}',
  'status.teamScore': 'Equipo {team}: {value}',
  'canasta.offer.rank': 'Valor',
  'canasta.offer.sequence': 'Escalera',
  'badge.naturalCanasta': 'Canasta pura',
  'badge.mixedCanasta': 'Canasta impura',
  'badge.samba': 'Samba',
  'badge.cleanRun': 'Escalera limpia',
  'canasta.seat.teamScore': 'Puntos del equipo',
  'canasta.seat.canastas': 'Canastas',
  'holdem.header.pot': 'Bote',
  'holdem.header.street': 'Calle',
  'holdem.header.hand': 'Mano',
  'holdem.header.handLimit': 'Manos en total',
  'holdem.header.blinds': 'Ciegas',
  'holdem.cost.call': 'para igualar',
  'holdem.cost.pot': 'en el bote',
  'holdem.seat.stack': 'Pila',
  'holdem.seat.bet': 'Apuesta',
  'holdem.prompt.yourAction': 'Te toca',
  'holdem.prompt.raiseTo': 'Subir a',
  'holdem.quick.halfPot': '½ Bote',
  'holdem.quick.pot': 'Bote',
  'holdem.quick.allIn': 'All-in',
  'zone.yourHand': 'Tu mano',
  'zone.opponentHand': 'Su mano',
  'zone.drawPile': 'Mazo',
  'zone.discardPile': 'Montón de descarte',
  'zone.melds': 'Combinaciones',
  'zone.teamMelds': 'Combinaciones de tu bando',
  'zone.opponentMelds': 'Combinaciones del bando contrario',
  'zone.redThrees': 'Treses rojos',
  'zone.board': 'Mesa',
  'verb.drawFromDeck': 'Robar',
  'verb.takeFromDiscard': 'Coger del montón',


  // --- the why sheet's own furniture ---------------------------------------
  // The three layers a refusal is explained in. Labels, not sentences: what
  // goes under each is sent by the server or worded above.
  'why.reason': 'Por qué no',
  'why.rule': 'La regla',
  'why.rules': 'Las reglas',
  'why.remedy': 'Lo que puedes hacer',
  'why.readTheRules': 'Leer las reglas completas →',
  'why.close': 'Cerrar',
  'why.open': 'por qué',

  // --- marks on a particular card ------------------------------------------
  // A mark is a refusal that has not happened yet: the rule is enforced at the
  // discard, which is the last possible moment to hear about it, so the card
  // says so while there is still a turn left to act on it.
  'zolik.badge.owedToMeld':
    '{card} vino del montón de descarte — tiene que entrar en las combinaciones con las que te bajas este turno.',
  'zolik.badge.jokerOwed':
    '{card} vino de la mesa — tiene que entrar en una combinación antes de que puedas terminar tu turno.',

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
  'legal.terms': 'Condiciones',
  // The document's own name, used as the heading above it. Kept apart from
  // `legal.notice.terms`, which is the same words inflected to sit inside a
  // sentence — a heading in the instrumental case reads as a mistake in every
  // Slavic language.
  'legal.terms.title': 'Condiciones de uso',
  'legal.privacy.title': 'Aviso de privacidad',
  'legal.privacy': 'Privacidad',
  'legal.source': 'Código fuente',
  'legal.updated': 'Versión {version}',
  'legal.draft':
    'Borrador — todavía no está en vigor. Faltan por rellenar el nombre, el país y la dirección de contacto del operador.',
  'legal.notice.before': 'Al jugar aceptas las ',
  'legal.notice.terms': 'condiciones de uso',
  'legal.notice.between': '. Lo que se guarda sobre ti figura en el ',
  'legal.notice.privacy': 'aviso de privacidad',
  'legal.notice.after': '.',
  // --- gin rummy -------------------------------------------------------------
  'err.UPCARD_DECLINED': 'Ya pasaste de esa carta',
  'err.DEADWOOD_TOO_HIGH': 'Tu deadwood es demasiado alto para llamar',
  'err.CARD_DOES_NOT_EXTEND_MELD': 'Esa carta no alarga esta combinación',
  'ginrummy.rules.setup': 'Preparación',
  'ginrummy.rules.turn': 'Tu turno',
  'ginrummy.rules.melds': 'Combinaciones',
  'ginrummy.rules.knocking': 'Llamar',
  'ginrummy.rules.bigGin': 'Big gin',
  'ginrummy.rules.layoff': 'El arrime',
  'ginrummy.rules.deadHand': 'La mano muerta',
  'ginrummy.rules.scoring': 'Puntuar una mano',
  'ginrummy.rules.match': 'Ganar la partida',
  'ginrummy.rules.lineBonuses': 'Bonificaciones del recuento',
  'ginrummy.rules.deck': 'Se juega con una baraja de {value} cartas.',
  'ginrummy.rules.deal': 'Cada jugador recibe {value} cartas.',
  'ginrummy.rules.upcard': 'Se levanta una carta más para iniciar el montón de descarte.',
  'ginrummy.rules.drawDiscard':
    'En tu turno, roba una carta — del mazo o del montón de descarte — y luego descarta una.',
  'ginrummy.rules.setsAndRuns':
    'Una combinación es un grupo de tres o cuatro cartas de un mismo valor, o una escalera de tres o más del mismo palo.',
  'ginrummy.rules.aceLow': 'El as siempre va bajo — no existe la escalera de dama a as.',
  'ginrummy.rules.knockLimit': 'Puedes llamar en cuanto tu deadwood sea {n} o menos.',
  'ginrummy.rules.oklahoma': 'El límite para llamar en esta mano lo fija el valor de la carta levantada.',
  'ginrummy.rules.gin': 'Deadwood cero es gin — la mejor llamada posible.',
  'ginrummy.rules.bigGinBonus':
    'Once cartas combinadas sin descartar nada es big gin, y vale {n} puntos más.',
  'ginrummy.rules.layoffDescription':
    'Tras una llamada que no sea gin, tu rival puede arrimar su propio deadwood a tus combinaciones antes de comparar las manos.',
  'ginrummy.rules.deadHandDescription':
    'Si el mazo baja a sus dos últimas cartas y nadie ha llamado, la mano está muerta — nadie puntúa y reparte de nuevo el mismo repartidor.',
  'ginrummy.rules.undercut':
    'Si el deadwood de tu rival no supera al tuyo, te corta: anota la diferencia, más {n}.',
  'ginrummy.rules.ginBonus': 'El gin anota la mano entera de tu rival, más {n}.',
  'ginrummy.rules.target': 'El primero en pasar de {n} puntos al terminar una mano gana la partida.',
  'ginrummy.rules.shutout': 'La bonificación de partida se dobla a {n} si el perdedor no anotó ni un punto.',
  'ginrummy.rules.box': 'Cada mano que ganes vale {n} puntos al final de la partida.',
  'ginrummy.rules.gameBonus': 'Ganar la partida vale {n} puntos más.',
  'ginrummy.fact.deadwood': '{value} de deadwood',
  'ginrummy.fact.discardCard': 'Descartar {value}',
  'ginrummy.fact.meldCards': 'Sobre {value}',
  'ginrummy.header.hand': 'Mano {n}',
  'ginrummy.round.kind': '{value}',
  'ginrummy.round.hand': 'Mano',
  'ginrummy.seat.score': '{value}',
  'ginrummy.seat.dealer': 'Repartidor',
  'ginrummy.status.knocked': '{playerId} llamó con {deadwood} de deadwood',
  'ginrummy.status.gin': '{playerId} hizo gin',
  'ginrummy.status.lastHand': 'Última mano: {winner} ({kind}, {delta} puntos)',
  'ginrummy.offer.drawStock': 'Robar del mazo',
  'ginrummy.offer.drawDiscard': 'Robar del montón de descarte',
  'ginrummy.offer.takeUpcard': 'Coger la carta levantada',
  'ginrummy.offer.passUpcard': 'Pasar',
  'ginrummy.offer.discard': 'Descartar',
  'ginrummy.offer.knock': 'Llamar',
  'ginrummy.offer.gin': '¡Gin!',
  'ginrummy.offer.bigGin': '¡Big gin!',
  'ginrummy.offer.layOff': 'Arrimar',
  'ginrummy.offer.finishLayoff': 'Arrime terminado',
  'ginrummy.zone.knockerHand': 'Mano que llamó',
  'ginrummy.zone.melds': 'Combinaciones',
  'ginrummy.prompt.upcardDecision': 'Coge la carta levantada, o pasa',
  'ginrummy.prompt.yourTurnDraw': 'Roba una carta',
  'ginrummy.prompt.yourTurnDiscard': 'Descarta — o llama, si puedes',
  'ginrummy.prompt.layoff': 'Arrima deadwood, o termina',

  // --- rummy tiles -------------------------------------------------------------
  'err.TILE_NOT_IN_HAND': 'Esa ficha no está en tu mano',
  'err.TILE_DOES_NOT_FIT': 'Eso no encaja ahí',
  'err.NO_SUCH_SET': 'Esa combinación no está en la mesa',
  'err.INITIAL_MELD_ONLY':
    'Antes de tu primera bajada solo puedes reorganizar tus propias combinaciones nuevas',
  'err.TABLE_NOT_VALID': 'La mesa todavía no es válida',
  'err.TRAY_NOT_EMPTY': 'Aún te quedan fichas sueltas por colocar',
  'err.NOTHING_PLAYED': 'Coloca al menos una ficha antes de terminar tu turno',
  'err.INITIAL_MELD_TOO_LOW': 'Tu primera bajada tiene que valer 30 puntos o más',
  'err.NOT_A_RUN': 'Solo puede partirse una escalera',
  'err.BAD_SPLIT_POSITION': 'Ahí no es donde puede partirse esta escalera',
  'err.NO_JOKER_IN_SET': 'En esa combinación no hay ningún comodín',
  'err.TILE_JOKER_SWAP_MISMATCH': 'Esa ficha no es a lo que sustituye el comodín',
  'rummytiles.rules.setup': 'Preparación',
  'rummytiles.rules.sets': 'Combinaciones',
  'rummytiles.rules.initialMeld': 'La bajada inicial',
  'rummytiles.rules.turn': 'Tu turno',
  'rummytiles.rules.jokerTaking': 'Recuperar un comodín',
  'rummytiles.rules.ending': 'Terminar una ronda',
  'rummytiles.rules.poolExhaustion': 'Si se agota el pozo',
  'rummytiles.rules.match': 'Ganar la partida',
  'rummytiles.rules.tiles': 'Se juega con {value} fichas.',
  'rummytiles.rules.dealCount': 'Cada jugador recibe {value} fichas.',
  'rummytiles.rules.group':
    'Un grupo son tres o cuatro fichas del mismo número, cada una de un color distinto.',
  'rummytiles.rules.run': 'Una escalera son tres o más números consecutivos de un mismo color.',
  'rummytiles.rules.noWrap': 'El 13 no vuelve a enlazar con el 1.',
  'rummytiles.rules.joker': 'Un comodín sustituye a cualquier ficha.',
  'rummytiles.rules.initialMeldDescription':
    'Hasta que no hayas bajado {n} puntos o más en un solo turno, y solo desde tu propia mano, no puedes tocar nada de lo que ya está en la mesa.',
  'rummytiles.rules.turnDescription':
    'Coloca al menos una ficha de tu mano, reorganizando la mesa a tu gusto, y termina con todas las combinaciones de la mesa válidas.',
  'rummytiles.rules.noDiscard':
    'No hay descarte — si no puedes completar un turno válido, robas una ficha en su lugar.',
  'rummytiles.rules.jokerTakingDescription':
    'Un comodín de la mesa puede recuperarse sustituyéndolo por la ficha a la que representa, desde tu mano — y tiene que usarse en una combinación antes de que acabe tu turno.',
  'rummytiles.rules.goingOut':
    'El primer jugador que se queda sin fichas gana la ronda. Los demás anotan en negativo el valor de lo que les queda; el ganador anota la suma de lo que perdieron los demás.',
  'rummytiles.rules.poolExhaustionLowestWins':
    'Si el pozo se agota y nadie puede jugar, la ronda termina y la gana la mano de menor valor.',
  'rummytiles.rules.poolExhaustionNoWinner':
    'Si el pozo se agota y nadie puede jugar, la ronda termina sin ganador — simplemente se puntúa cada mano.',
  'rummytiles.rules.target': 'El primero en pasar de {n} puntos al terminar una ronda gana la partida.',
  'rummytiles.rules.roundLimit': 'La partida termina tras {n} rondas — gana la puntuación más alta.',
  'rummytiles.fact.setCards': '{value}',
  'rummytiles.header.pool': 'Pozo {n}',
  'rummytiles.header.round': 'Ronda {n}',
  'rummytiles.round.kind': '{value}',
  'rummytiles.round.round': 'Ronda',
  'rummytiles.seat.score': '{value}',
  'rummytiles.seat.notOpened': 'Sin abrir',
  'rummytiles.status.lastRound': 'Última ronda: {winner} ({kind})',
  'rummytiles.badge.invalid': 'Todavía no válido',
  'rummytiles.zone.pool': 'Pozo',
  'rummytiles.zone.table': 'Mesa',
  'rummytiles.zone.tray': 'Atril',
  'rummytiles.offer.place': 'Colocar',
  'rummytiles.offer.addFromHand': 'Añadir',
  'rummytiles.offer.addFromTray': 'Añadir del atril',
  'rummytiles.offer.take': 'Coger',
  'rummytiles.offer.split': 'Partir',
  'rummytiles.offer.swapJoker': 'Cambiar el comodín',
  'rummytiles.offer.resetTurn': 'Reiniciar el turno',
  'rummytiles.offer.commit': 'Hecho',
  'rummytiles.offer.draw': 'Robar',
  'rummytiles.param.position': 'Partir en',

  // --- blackjack -----------------------------------------------------------
  //
  // The house rules are worded twice over — on and off — because a rule that
  // is *not* in force is still something a player has to be told. "No
  // surrender at this table" is information; silence is a guess.
  'err.BET_BELOW_MINIMUM': 'Eso está por debajo del mínimo de la mesa',
  'err.ALREADY_BET': 'Tu apuesta ya está puesta',
  'err.INSURANCE_CLOSED': 'Ahora mismo no hay seguro que tomar',
  'err.CANNOT_DOUBLE': 'Esta mano no se puede doblar',
  'err.CANNOT_SPLIT': 'Esta mano no se puede separar',
  'err.CANNOT_SURRENDER': 'Esta mano no se puede abandonar',

  'blackjack.rules.section.table': 'La mesa',
  'blackjack.rules.section.play': 'Jugar una mano',
  'blackjack.rules.section.dealer': 'El crupier',
  'blackjack.rules.section.end': 'Cómo termina la partida',
  'blackjack.rules.goal':
    'Gana al crupier sin pasarte de veintiuno. Pasarte pierde al momento, haga lo que haga el crupier después.',
  // Counts are their own phrase rather than a number glued to a noun: one
  // deck and six decks inflect differently, and Czech inflects them again.
  'blackjack.rules.decks': 'Barajas en el zapato: {n}.',
  'blackjack.rules.stack': 'Cada asiento se sienta con {n} fichas.',
  'blackjack.rules.minBet': 'El mínimo de la mesa es de {n} fichas.',
  'blackjack.rules.faceUp':
    'Las cartas de los jugadores se reparten boca arriba; el crupier mantiene una carta tapada hasta que todos hayan jugado.',
  'blackjack.rules.hitStand': 'Pide tantas cartas como quieras, o plántate con lo que tengas.',
  'blackjack.rules.aces': 'Un as vale once mientras quepa, y uno cuando no.',
  'blackjack.rules.blackjack': 'Un as con una carta de valor diez, en las dos primeras cartas, es blackjack.',
  'blackjack.rules.pays3to2': 'El blackjack paga 3:2.',
  'blackjack.rules.pays6to5': 'El blackjack paga 6:5.',
  'blackjack.rules.paysEven': 'El blackjack paga a la par.',
  'blackjack.rules.double':
    'Con tus dos primeras cartas puedes doblar tu apuesta y recibir exactamente una carta más.',
  'blackjack.rules.doubleAfterSplit': 'Una mano surgida de una separación también puede doblarse.',
  'blackjack.rules.noDoubleAfterSplit': 'Una mano surgida de una separación no puede doblarse.',
  'blackjack.rules.split':
    'Dos cartas del mismo valor pueden separarse en manos propias, cada una con su apuesta — hasta {n} veces, para {hands} manos en total.',
  'blackjack.rules.noSplit': 'En esta mesa no se separan las parejas.',
  'blackjack.rules.splitAces':
    'Los ases separados reciben una carta cada uno y se plantan, y el veintiuno logrado así no es blackjack.',
  'blackjack.rules.surrender':
    'Puedes abandonar tu primera mano por la mitad de su apuesta, una vez que el crupier haya comprobado si tiene blackjack.',
  'blackjack.rules.noSurrender': 'En esta mesa no pueden abandonarse las manos.',
  'blackjack.rules.dealerDraws': 'El crupier pide hasta diecisiete y entonces se planta.',
  'blackjack.rules.hitsSoft17': 'El crupier pide con un diecisiete formado con un as.',
  'blackjack.rules.standsSoft17': 'El crupier se planta con un diecisiete formado con un as.',
  'blackjack.rules.dealerPeeks':
    'Si muestra un as o un diez, el crupier comprueba si tiene blackjack antes de que nadie juegue.',
  'blackjack.rules.insurance':
    'Frente a un as del crupier puedes asegurarte por la mitad de tu apuesta; paga 2:1 si el crupier tiene blackjack.',
  'blackjack.rules.noInsurance': 'En esta mesa no se ofrece seguro.',
  'blackjack.rules.rounds': 'En la mesa se juegan {n} rondas.',
  'blackjack.rules.mostChipsWins': 'Quien tenga más fichas al final gana la partida.',
  'blackjack.rules.bustedOut':
    'El asiento que ya no puede cubrir el mínimo de {n} queda fuera el resto de la partida.',

  'blackjack.zone.dealer': 'Crupier',
  'blackjack.zone.box': 'Mano',
  'blackjack.zone.yourBox': 'Tu mano',
  'blackjack.zone.shoe': 'Zapato',

  'blackjack.header.round': 'Ronda {n} de {of}',
  'blackjack.header.minBet': 'Mínimo',
  'blackjack.header.decks': 'Barajas',
  'blackjack.header.dealerTotal': 'El crupier muestra {n}',
  'blackjack.header.dealerSoftTotal': 'El crupier muestra {n} blando',

  'blackjack.seat.stack': 'Fichas',
  'blackjack.seat.bet': 'Apuesta',
  'blackjack.seat.insurance': 'Seguro',
  'blackjack.seat.total': 'Total',
  'blackjack.seat.softTotal': 'Total blando',
  'blackjack.seat.out': 'Sin fichas',

  'blackjack.prompt.placeBet': 'Haz tu apuesta',
  'blackjack.prompt.insurance': '¿Seguro?',
  'blackjack.prompt.yourMove': 'Te toca',
  'blackjack.prompt.waitingFor': 'Esperando a {playerId}',
  'blackjack.prompt.betAmount': 'Apuesta',

  'blackjack.quick.doubleMin': '2× Mínimo',
  'blackjack.quick.allIn': 'All-in',
  'blackjack.offer.bet': 'Apostar',
  'blackjack.offer.hit': 'Pedir',
  'blackjack.offer.stand': 'Plantarse',
  'blackjack.offer.double': 'Doblar',
  'blackjack.offer.split': 'Separar',
  'blackjack.offer.surrender': 'Abandonar',
  'blackjack.offer.insure': 'Tomar seguro',
  'blackjack.offer.declineInsurance': 'Sin seguro',

  'blackjack.fact.tableMinimum': 'mínimo',
  'blackjack.fact.insuranceCost': 'para asegurar',
  'blackjack.fact.extraStake': 'para apostar',
  'blackjack.fact.surrenderReturn': 'de vuelta',

  'blackjack.status.dealerBlackjack': 'El crupier tenía blackjack',
  'blackjack.status.dealerBust': 'El crupier se pasó con {n}',
  'blackjack.status.dealerStands': 'El crupier se planta en {n}',

  'blackjack.round.name': 'Ronda',
  'blackjack.round.dealerTotal': 'Crupier {n}',
  'blackjack.round.dealerBust': 'Crupier pasado ({n})',
  'blackjack.round.dealerBlackjack': 'Blackjack del crupier',
  'blackjack.round.outcome.blackjack': 'Blackjack',
  'blackjack.round.outcome.win': 'Ganada',
  'blackjack.round.outcome.push': 'Empate',
  'blackjack.round.outcome.lose': 'Perdida',
  'blackjack.round.outcome.bust': 'Pasado',
  'blackjack.round.outcome.surrender': 'Abandonada',

  'blackjack.badge.inPlay': 'En juego',
  'blackjack.badge.doubled': 'Doblada',
  'blackjack.badge.split': 'Separada',
  'blackjack.badge.blackjack': 'Blackjack',
  'blackjack.badge.bust': 'Pasado',
  'blackjack.badge.won': 'Ganada',
  'blackjack.badge.push': 'Empate',
  'blackjack.badge.lost': 'Perdida',
  'blackjack.badge.surrendered': 'Abandonada',

  'blackjack.unit.chips': 'fichas',

  // --- the settings screen --------------------------------------------------
  //
  // Hardcoded English until now, which was survivable while the only other
  // language was unreachable and faintly absurd once the language picker
  // itself lived on this screen: a player who cannot read "Settings" is
  // exactly the one who came here to change it.
  'settings.title': 'Ajustes',
  'settings.signedInAs': 'Sesión iniciada como {username}',
  'settings.playingAsGuest': 'Jugando como {username} (invitado)',
  'settings.notSignedIn':
    'No has iniciado sesión — inicia sesión o continúa como invitado para jugar online.',
  'settings.subtitle': 'Qué aspecto tienes tú y qué aspecto tiene la mesa',
  'settings.face.heading': 'Tu cara en la mesa',
  'settings.face.account': 'Se guarda con tu cuenta, así que te acompaña a otro dispositivo.',
  'settings.face.device': 'Se guarda en este dispositivo. Inicia sesión para llevarla contigo.',
  'settings.skin.heading': 'Aspecto de la mesa',
  'settings.language.heading': 'Idioma',
  'settings.language.status': 'Se guarda en este dispositivo.',
  'settings.language.auto': 'Automático',
  'settings.language.auto.now': 'Sigue a tu dispositivo — ahora {language}',
  'settings.legal.heading': 'La letra pequeña',
  'settings.legal.status': 'Lo que aceptaste al jugar, y lo que se guarda sobre ti.',
  'settings.signIn': 'Iniciar sesión',
  'settings.back': 'Volver',

  // A translated interface over an English notice is a worse lie than an
  // English interface, so the document says so itself rather than letting the
  // surrounding screen imply the text was written for this reader.
  'legal.untranslated':
    'Este aviso todavía no se ha traducido a tu idioma. El texto en inglés que aparece abajo es la versión aplicable.',

  // --- the navigation bar's screen titles ------------------------------------
  //
  // These sit in `app/_layout.tsx`, above every screen, and were the last
  // English left after the settings screen was translated — a German player
  // reading "Einstellungen" under a bar that said "Settings". A translated app
  // with an untranslated chrome bar looks like a bug, not like a limit.
  'nav.home': 'Zolik',
  'nav.emailSignIn': 'Acceso con correo',
  'nav.signingIn': 'Accediendo',
  'nav.usernameSignIn': 'Acceso con usuario',
  'nav.legacyAccount': 'Cuenta antigua',
  'nav.guest': 'Invitado',
  'nav.account': 'Cuenta',
  'nav.games': 'Juegos',
  'nav.table': 'Tu mesa',
  'nav.join': 'Unirse a una mesa',
  // The screen a shared invite link lands on while it resolves the code.
  'nav.joining': 'Uniéndose',
  'nav.rules': 'Reglas',
  'nav.match': 'Partida',
  'nav.scoreTable': 'Tabla de puntos',
  'nav.stats': 'Estadísticas',
  'nav.more': 'Más',

  // --- the account menu behind the face in the corner ----------------------
  'menu.label': 'Menú de cuenta',
  'menu.signedIn': 'Sesión iniciada',
  'menu.notSignedIn': 'Sin sesión iniciada',
  'menu.keepStats': 'para conservar tus estadísticas',
  'menu.signOut': 'Cerrar sesión',
  'more.scoreTable': 'Tabla de puntos sin conexión',
  'more.stats': 'Estadísticas y clasificación',
  'more.needsAccount': 'inicia sesión para usar',
  'gate.title': 'Inicia sesión para usar esto',
  'gate.body':
    'Las tablas de puntos y las estadísticas se guardan con tu cuenta, así te acompañan a otro dispositivo. Un invitado no tiene dónde guardarlas.',

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
  'error.generic': 'Eso no ha funcionado',
  'error.signIn': 'Error al iniciar sesión',
  'error.login': 'Error al iniciar sesión',
  'error.register': 'Error al registrarse',
  'error.sendCode': 'No se ha podido enviar un código',
  'error.badCode': 'Ese código no ha funcionado',
  'error.rulesLoad': 'No se han podido cargar las reglas',
  'error.createFailed': 'Error al crear',
  'error.saveFailed': 'Error al guardar',
  'error.exportFailed': 'Error al exportar',

  // --- the route that does not exist ----------------------------------------
  'notFound.title': '¡Vaya!',
  'notFound.message': 'Esta pantalla no existe.',
  'notFound.home': '¡Ir a la pantalla de inicio!',

  // --- signing in -----------------------------------------------------------
  'auth.login.subtitle': 'Conserva tus estadísticas en todos tus dispositivos',
  'auth.login.continueWithEmail': 'Continuar con el correo',
  'auth.login.usernameInstead': 'Iniciar sesión con un usuario en su lugar',
  'auth.email.title': 'Acceso con correo',
  'auth.email.subtitle': 'Te enviaremos un código de un solo uso',
  'auth.email.address': 'Dirección de correo',
  'auth.email.send': 'Enviar código',
  'auth.email.codeTitle': 'Introduce el código',
  'auth.email.codePlaceholder': 'Código de 6 dígitos',
  'auth.email.differentAddress': 'Usar otra dirección',
  'auth.email.sentTo': 'Enviado a {email}',
  'auth.email.continue': 'Continuar',
  'auth.guest.title': 'Juego como invitado',
  'auth.guest.subtitle': 'No hace falta cuenta',
  'auth.guest.displayName': 'Nombre visible',
  'auth.register.title': 'Crear cuenta',
  'auth.register.username': 'Usuario',
  'auth.register.email': 'Correo (opcional)',
  'auth.register.password': 'Contraseña',
  'auth.username.createAccount': 'Crear una cuenta con usuario y contraseña',
  'auth.callback.signedIn': 'Sesión iniciada.',

  // --- the account screen ---------------------------------------------------
  'account.signInPrompt': 'Inicia sesión para gestionar tu cuenta.',
  'account.keepGames': 'Conservar estas partidas',
  'account.signedInWith': 'Sesión iniciada con',
  'account.addMethod': 'Añadir un método de acceso',
  'account.usernameAndPassword': 'Usuario y contraseña',
  'account.faceAndTable': 'Cara y aspecto de la mesa',
  'account.refresh': 'Actualizar',
  'account.remove': 'Quitar',

  // --- the main menu --------------------------------------------------------
  'home.subtitle': 'Rummy continental · {server}',
  'home.playingAs': 'Juegas como {name}',
  'home.signInPrompt': 'Inicia sesión o continúa como invitado para jugar en línea.',
  'home.statsAndLeaderboard': 'Estadísticas y clasificación',
  'home.play': 'Jugar',
  'home.offlineScoreTable': 'Tabla de puntos sin conexión',
  'home.signInToKeepStats': 'Inicia sesión para conservar tus estadísticas',
  'home.signOut': 'Cerrar sesión',
  'home.continueAsGuest': 'Continuar como invitado',
  // Someone playing as a guest, marked in a list of names.
  'home.guestSuffix': '(invitado)',

  // --- the waiting room on the main menu ------------------------------------
  //
  // Counts are whole phrases per count, not a number glued to a noun — the
  // same reason `countLabel` exists. One and many are the cases the interface
  // actually produces, so they are the cases that get their own wording.
  'waiting.checking': 'Viendo quién anda por aquí…',
  'waiting.youAreWaiting': 'Estás esperando para jugar',
  'waiting.pickedUp': 'Cualquiera que abra una mesa puede recogerte — no necesita ningún código tuyo.',
  'waiting.othersOne': '1 jugador más también está esperando',
  'waiting.othersMany': '{n} jugadores más también están esperando',
  'waiting.oneWaiting': '1 jugador está esperando para jugar',
  'waiting.manyWaiting': '{n} jugadores están esperando para jugar',
  'waiting.adding': 'Añadiéndote a la lista de espera…',
  'waiting.slowHint':
    'Si esto no termina en unos segundos, comprueba que la dirección del servidor de abajo sea accesible desde este dispositivo.',
  'waiting.serverBusyDetail':
    'Intento {n}. El servidor no está aceptando nuevas conexiones a la sala de espera ahora mismo.',
  'waiting.reconnecting': 'Conexión perdida — reconectando…',
  'waiting.reconnectingDetail':
    'Intento {n}. Esto puede pasar si la red de tu dispositivo ha cambiado, o si el servidor se ha reiniciado.',
  'waiting.tryAgain': 'Reintentar ahora',
  'waiting.makeAvailable': 'Ponerme disponible para jugar',
  'waiting.stop': 'Dejar de esperar',
  'waiting.noneYet':
    'Ahora mismo no hay nadie esperando para jugar. Apúntate a la lista y serás el primero que vea cualquiera.',
  'waiting.noOthersYet': 'Todavía no espera nadie más. Los anfitriones te ven igualmente y pueden invitarte.',
  'waiting.server': 'Servidor',
  'waiting.none': 'Ahora mismo no espera nadie. Quien se ponga disponible en el menú principal aparece aquí.',

  // --- landing on a shared invite link --------------------------------------
  'join.missingCode': 'A ese enlace le falta el código de la mesa.',
  'join.staleLink': 'Pide un enlace nuevo a quien te invitó, o únete con el código en su lugar.',
  'join.enterCode': 'Introducir un código',
  'join.backToMenu': 'Volver al menú',
  'join.takingSeat': 'Tomando asiento…',
  'join.takingSeatAt': 'Tomando asiento en {game}…',

  // --- the lobby ------------------------------------------------------------
  'lobby.games.subtitle': 'Todo lo que este servidor puede alojar',
  'lobby.games.bots': 'Bots',
  'lobby.games.playBot': 'Jugar contra un bot',
  'lobby.games.playBots': 'Jugar contra {n} bots',
  'lobby.games.openTable': 'Abrir una mesa',
  'lobby.games.players': '{n} jugadores',
  'lobby.games.playerRange': '{min}–{max} jugadores',
  'lobby.join.placeholder': 'Código o enlace de invitación',
  'lobby.join.needCode': 'Introduce un código, un enlace o un ID de partida',
  'lobby.games.signInFirst': 'Inicia sesión primero',
  'lobby.join.action': 'Unirse',
  'lobby.join.waitingTitle': 'Esperando al anfitrión',
  // Two whole sentences rather than one with a swapped noun: "a game of {game}"
  // does not survive a language that inflects the game's name after "of".
  'lobby.join.joinedGame': 'Te has unido a una partida de {game} — esperando para empezar',
  'lobby.join.joinedTable': 'Te has unido a la mesa — esperando para empezar',
  'lobby.table.addBot': 'Añadir un bot',
  'lobby.table.start': 'Empezar',
  'lobby.table.waitingForHost': 'Esperando a que el anfitrión empiece…',

  // --- inviting someone to a table ------------------------------------------
  'invite.heading': 'Invitar jugadores',
  'invite.explain': 'Envía este enlace. Quien lo abra llega a esta mesa — sin necesidad de cuenta.',
  'invite.noAddress':
    'Este servidor no tiene configurada una dirección compartible, así que usa el código de abajo.',
  'invite.readOutCode': 'O dicta el código:',
  'invite.copy': 'Copiar enlace',
  'invite.share': 'Compartir enlace',
  'invite.copied': '¡Copiado!',
  'invite.shared': 'Compartido',

  // --- the match screen -----------------------------------------------------
  'match.waitingForTable': 'Esperando a la mesa…',
  'match.waitingForPlayer': 'Esperando a otro jugador…',
  'match.nobodyWon': 'No ha ganado nadie.',
  'match.youWon': 'Has ganado.',
  'match.finished': 'Esta partida ha terminado.',
  'match.inProgress': 'Partida en curso — todo está conectado y funcionando con normalidad.',
  'match.connecting': 'Conectando…',
  'match.abandonedTitle': 'Mesa apartada',
  'match.abandoned': 'Nadie volvió a esta mesa, así que se apartó. Las cartas están exactamente donde las dejaste.',
  'match.resume': 'Continuar donde lo dejaste',
  'match.resuming': 'Recuperando la mesa…',
  'match.controls': 'Controles',
  'match.over': 'Partida terminada',
  'match.settingUp': 'Preparando…',
  'match.playAgain': 'Jugar otra vez',
  'match.backToGames': 'Volver a los juegos',
  'match.table': 'Mesa',
  'match.opponents': 'Rivales',
  // Marks which seat is the reader's own, in a list of seats.
  'match.youSuffix': '(tú)',
  // The winner line is composed from whole sentences rather than a name glued
  // to " won." — the verb agrees with the subject in most of these languages,
  // and a suffix cannot know that.
  'match.you': 'tú',
  'match.someoneWon': '{name} ha ganado.',
  'match.wonBy': 'Ganada por {names}.',
  'match.pausedFor': 'En pausa — esperando a que {name} se reconecte.',
  'match.results': 'Resultados',
  'match.players': 'Jugadores',
  'match.toPlay': 'le toca',

  // --- the offline score table ----------------------------------------------
  'scoring.namesHint': 'Nombres separados por comas (4–8 jugadores)',
  'scoring.newSession': 'Nueva sesión',
  // A worked example, not a sentence: the names are placeholders a translator
  // may localise, the shape `name:score,` is what the parser needs.
  'scoring.scoresPlaceholder': 'Ana:120,Berto:80,…',
  'scoring.saveRound': 'Guardar ronda',
  'scoring.export': 'Exportar hoja de puntos',
  'scoring.formatHint': 'Formato de puntos: Nombre:100,Nombre2:50',
  'scoring.nameCountError': 'Introduce de 2 a 8 nombres separados por comas',
  'scoring.session': 'Sesión: {id}',
  'scoring.players': 'Jugadores: {names}',
  'scoring.roundScores': 'Puntos de la ronda {n}',
  'stats.loading': 'Cargando…',
  // The figures could not be fetched. `{reason}` is whatever the server or the
  // network said, which is not ours to translate — the frame around it is.
  'stats.unavailable': '(no disponible: {reason})',

  // --- statistics -----------------------------------------------------------
  'stats.title': 'Estadísticas y clasificación',
  'stats.yours': 'Tus estadísticas',
  'stats.leaderboard': 'Clasificación',

  // --- a player's lifetime record, shown beside a finished match ------------
  'record.title': 'Tu historial',
  'record.guest':
    'Estás jugando como invitado, así que no se guarda ningún historial. Inicia sesión y las partidas que ya has jugado en este dispositivo — esta incluida — quedarán en tu cuenta.',
  'record.signInToKeep': 'Iniciar sesión y conservarlas',
  'record.failed': 'Tu historial no se ha podido cargar ahora mismo. La partida está guardada sin problema.',
  'record.loading': 'Cargando…',
  'record.played': 'Jugadas',
  'record.won': 'Ganadas',
  'record.lost': 'Perdidas',
  'record.winRate': 'Porcentaje de victorias',
  'record.streak': 'Racha',
  'record.atThisGame': 'En este juego',
  // A streak is a count with a noun, so it is a whole phrase per count for
  // the same reason `countLabel` is: "1 win" / "2 wins" is an English rule,
  // and most of these languages do not share it.
  'record.streakNone': '—',
  'record.streakWinOne': '1 victoria',
  'record.streakWinMany': '{n} victorias',
  'record.streakLossOne': '1 derrota',
  'record.streakLossMany': '{n} derrotas',

  // --- moving cards ---------------------------------------------------------
  'hand.dragHint': 'Arrastra una carta por el abanico para reordenarla, o sobre la mesa para jugarla',
  'hand.moveLeft': 'Mover a la izquierda',
  'hand.moveRight': 'Mover a la derecha',
  'zone.collapseGroup': 'Contraer este grupo',
  'zone.expandGroup': 'Mostrar todas las cartas de este grupo',
  'zone.dropHere': 'Suelta aquí',
  'offer.pickCards': 'elige cartas para el sitio que has tocado',
  'offer.ambiguous': 'esto puede ir en más de un sitio — elige en la mesa',

  // --- the build footer -----------------------------------------------------
  'build.app': 'app',
  'build.server': 'servidor',
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
  'option.pauseBetweenRounds': 'Pausa entre rondas',
  'choice.pauseBetweenRounds.1': 'Pausa',
  'choice.pauseBetweenRounds.0': 'Seguir sin parar',
  'option.botSkill': 'Rivales',
  'choice.botSkill.0': 'Mezclados',
  'choice.botSkill.1': 'Fácil',
  'choice.botSkill.2': 'Medio',
  'choice.botSkill.3': 'Difícil',
  'option.initialMeldMinimum': 'Valor de apertura',
  'choice.initialMeldMinimum.0': 'Sin mínimo',
  'option.discardDrawMinRound': 'Robo del descarte',
  'choice.discardDrawMinRound.0': 'Abierto',
  'choice.discardDrawMinRound.2': 'Desde la ronda 2',
  'choice.discardDrawMinRound.3': 'Desde la ronda 3',
  'option.requireCleanRun': 'Escalera sin comodines',
  'choice.requireCleanRun.1': 'Obligatoria',
  'choice.requireCleanRun.0': 'No',
  'option.jokerReclaimMustPlay': 'Comodín recuperado',
  'choice.jokerReclaimMustPlay.1': 'Jugar en el mismo turno',
  'choice.jokerReclaimMustPlay.0': 'Se puede guardar',
  'option.dealStarter': 'Mano',
  'choice.dealStarter.0': 'Por turnos',
  'choice.dealStarter.1': 'Sale el ganador',
  'variation.prsi.classic': 'Clásico',
  'option.handSize': 'Cartas repartidas',
  'variation.canasta.classic': 'Clásica',
  'variation.canasta.modern_american': 'Modern American',
  'variation.canasta.samba': 'Samba',
  'option.targetScore': 'Puntuación objetivo',
  'option.canastasToGoOut': 'Canastas para salir',
  'variation.holdem.freezeout': 'Freezeout',
  'variation.holdem.timed': 'Número fijo de manos',
  'option.startingStack': 'Fichas iniciales',
  'option.bigBlind': 'Ciega grande',
  'option.handLimit': 'Manos',
  'choice.handLimit.0': 'Hasta que quede un solo asiento',
  'variation.ginrummy.standard': 'Estándar',
  'option.knockLimit': 'Límite para llamar',
  'choice.knockLimit.0': 'Oklahoma (lo fija la carta levantada)',
  'option.bigGin': 'Big gin',
  'choice.bigGin.0': 'No',
  'choice.bigGin.1': 'Sí (+25)',
  'option.lineBonuses': 'Bonificaciones del recuento',
  'choice.lineBonuses.1': 'Sí',
  'choice.lineBonuses.0': 'No',
  'variation.rummytiles.standard': 'Estándar',
  'choice.targetScore.0': 'Sin objetivo',
  // Two games qualify a number the others mean plainly: Canasta's 500 is a
  // short game, Gin Rummy's and Rummy Tiles' 500 is just 500; Hold'em's
  // starting 200 is a short stack, Blackjack's 200 is just 200. Module-scoped
  // so the qualifier travels with the game that means it — a bare key would
  // hand the note to games it is false for. `scripts/check-server-labels.js`
  // is what finds these; both were shipping as English on a Czech screen.
  'choice.canasta.targetScore.500': '500 (corta)',
  'choice.holdem.startingStack.200': '200 (corta)',
  'option.roundLimit': 'Límite de rondas',
  'choice.roundLimit.0': 'Sin límite',
  'option.poolExhaustion': 'Si se agota el pozo',
  'choice.poolExhaustion.1': 'Gana la ronda la mano más baja',
  'choice.poolExhaustion.0': 'Nadie gana la ronda',
  'variation.blackjack.single': 'Una baraja',
  'option.minBet': 'Mínimo de la mesa',
  'option.rounds': 'Rondas',
  'option.decks': 'Barajas',
  'option.dealerHitsSoft17': 'Crupier con 17 blando',
  'choice.dealerHitsSoft17.0': 'Se planta',
  'choice.dealerHitsSoft17.1': 'Pide',
  'option.blackjackPays': 'El blackjack paga',
  'choice.blackjackPays.100': 'A la par',
  'option.maxSplits': 'Separaciones',
  'choice.maxSplits.0': 'Sin separar',
  'choice.maxSplits.1': 'Una vez (dos manos)',
  'choice.maxSplits.3': 'Tres veces (cuatro manos)',
  'option.doubleAfterSplit': 'Doblar tras separar',
  'choice.doubleAfterSplit.1': 'Permitido',
  'choice.doubleAfterSplit.0': 'No permitido',
  'option.surrender': 'Abandono',
  'choice.surrender.0': 'No',
  'choice.surrender.1': 'Abandono tardío',
  'option.insurance': 'Seguro',
  'choice.insurance.1': 'Se ofrece',
  'choice.insurance.0': 'No se ofrece',

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
  'verb.add': 'Añadir',
  'verb.bet': 'Apostar',
  'verb.call': 'Igualar',
  'verb.check': 'Pasar',
  'verb.commit': 'Hecho',
  'verb.continue': 'Continuar',
  'verb.decline_insurance': 'Sin seguro',
  'verb.discard': 'Descartar',
  'verb.double': 'Doblar',
  'verb.draw': 'Robar',
  'verb.finish_layoff': 'Arrime terminado',
  'verb.fold': 'Retirarse',
  'verb.hit': 'Pedir',
  'verb.insure': 'Tomar seguro',
  'verb.knock': 'Llamar',
  'verb.lay_meld': 'Bajar',
  'verb.lay_off': 'Arrimar',
  'verb.pass': 'Pasar',
  'verb.place': 'Colocar',
  'verb.play_card': 'Jugar',
  'verb.raise': 'Subir',
  'verb.reset_turn': 'Reiniciar el turno',
  'verb.split': 'Separar',
  'verb.stand': 'Plantarse',
  'verb.surrender': 'Abandonar',
  'verb.swap_joker': 'Cambiar el comodín',
  'verb.take': 'Coger',
  'verb.take_pile': 'Coger del montón',
  // Declared by the server and listed in `serverKeys.json`, but never worded.
  'verb.takePileFromHand': 'Llevarte el montón a la mano',
  'verb.takePileOntoMeld': 'Llevar el montón a una combinación',
  'verb.takeTopForSequence': 'Llevar la carta superior a una escalera',
  'verb.undoDraw': 'Deshacer robo',
  'verb.undoLayOff': 'Deshacer arrime',
  'verb.undoMeld': 'Deshacer combinación',
  'verb.undoTakePile': 'Deshacer toma del montón',
  'verb.undoTurn': 'Deshacer turno',

  // --- suits, spelled out ---------------------------------------------------
  //
  // Read aloud by a screen reader and shown where a pip would not fit. The
  // pips themselves are drawn, not written, so these are the only place the
  // suit is ever a word.
  'suit.C': 'Tréboles',
  'suit.D': 'Diamantes',
  'suit.H': 'Corazones',
  'suit.S': 'Picas',

  // --- counters the board prints beside a number ----------------------------
  'canasta.seat.notOpened': 'Sin abrir',
  'canasta.unit.points': 'puntos',
  'ginrummy.unit.points': 'puntos',
  'holdem.seat.dealer': 'Repartidor',
  'holdem.seat.folded': 'Retirado',
  'holdem.seat.allIn': 'All in',
  'holdem.seat.out': 'Fuera',
  'holdem.unit.chips': 'fichas',
  'prsi.unit.cardsLeft': 'cartas restantes',
  'rummytiles.prompt.initialMeld': 'Tu primera bajada debe valer {n} puntos.',
  'rummytiles.unit.points': 'puntos',
  'zolik.unit.penalty': 'penalización',
  'header.pileFrozen': 'Montón congelado',
  // The module-less form of `ginrummy.prompt.yourTurnDraw`: Žolíky and Canasta
  // send the prompt without a module prefix. Neither the Go constants nor
  // `serverKeys.json` list it — the sweep is what found it.
  'prompt.yourTurnDraw': 'Roba una carta',
  'prompt.yourTurnMeld': 'Combina si puedes y luego descarta',
};
