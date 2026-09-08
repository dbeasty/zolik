/**
 * Portuguese (European spelling, post-Acordo Ortográfico). Serves every Portuguese-speaking region: localeDetect matches on the language subtag, so pt-BR resolves here too.
 */

export const pt: Record<string, string> = {
  // --- engine error codes (rules.RulesErrorCode) ---------------------------
  'err.NOT_YOUR_TURN': 'Não é a tua vez',
  'err.WRONG_PHASE': 'Neste momento não é possível',
  'err.MUST_DRAW_FIRST': 'Compra uma carta antes de baixar',
  'err.GAME_SUSPENDED': 'O jogo está em pausa',
  'err.GAME_NOT_ACTIVE': 'O jogo não está a decorrer',
  // --- runtime refusals, which are not the rules saying no -----------------
  //
  // Both were reachable long before they were worded, and a player who met
  // one was shown the code in capitals or, worse, nothing at all: an action
  // sent while the socket was down simply vanished, which reads as a legal
  // move being silently refused.
  'err.MATCH_NOT_ACTIVE': 'A mesa está em pausa — à espera que um jogador se ligue de novo',
  'err.NOT_CONNECTED': 'Sem ligação à mesa — a religar; tenta outra vez a seguir',
  // Why the "start the next round" control is greyed out. Without wording the
  // shell printed the code itself, so a player agreeing to go on was answered
  // with ALREADY_READY in capitals.
  'err.ALREADY_READY': 'Já estás pronto',
  'err.NOT_BETWEEN_ROUNDS': 'A ronda ainda está a decorrer',
  'err.NOT_AT_THIS_TABLE': 'Não estás nesta mesa',
  'err.DISCARD_LOCKED': 'O monte de descartes está bloqueado por agora',
  'err.DISCARD_PILE_EMPTY': 'O monte de descartes está vazio',
  'err.NO_CARDS_LEFT': 'Já não há cartas para comprar',
  'err.ROUND_REQ_NOT_MET': 'Baixa primeiro a tua própria abertura',
  'err.NEED_CLEAN_RUN': 'Precisas de uma sequência sem joker na mesa para contares como baixado',
  'err.INCOMPLETE_INITIAL_MELD': 'Termina a tua baixa, ou anula-a, antes de descartares',
  'err.DISCARD_CARD_NOT_MELDED': 'A carta que apanhaste tem de entrar na tua combinação',
  'err.JOKER_DISCARD_FORBIDDEN': 'Um joker não pode ser descartado',
  'err.NOTHING_TO_UNDO': 'Não há nada para anular',
  'err.NO_JOKER_IN_MELD': 'Não há joker nesta combinação',
  'err.JOKER_SWAP_MISMATCH': 'Essa carta não ocupa o lugar do joker',
  'err.RECLAIMED_JOKER_NOT_MELDED':
    'O joker que tiraste da mesa tem de ser jogado numa combinação nesta jogada',
  'err.RUN_TOO_LONG': 'Essa sequência já tem o comprimento máximo',
  'err.WRONG_RUN_END': 'Essa carta prolonga a outra ponta da sequência',
  'err.INVALID_MELD': 'Nenhuma carta da tua mão serve aqui',
  'err.CARD_NOT_IN_HAND': 'Essa carta não está na tua mão',
  'err.MELD_BELOW_MINIMUM': 'Às tuas combinações ainda faltam pontos para baixares',
  'err.MELD_NO_CONTRIBUTION': 'Essa combinação não faz avançar o teu requisito',
  'err.TOO_MANY_WILDS': 'Jokers a mais nessa combinação',
  'err.ADJACENT_WILDS': 'Dois jokers não podem ficar lado a lado',
  'err.ACE_BRIDGE': 'Um ás não pode ligar o rei ao dois',

  // --- contract phrasing ---------------------------------------------------
  // Built from the structured contract the server sends (sets/runs counts),
  // never from a deal-number lookup table.
  // Whole phrases per count rather than a number plus a pluralised noun:
  // Czech inflects the noun by count (skupina / skupiny / skupin) in a way no
  // "add an s" helper survives, and the counts that actually occur are 1-3.
  // A count outside that falls back to contract.sets.n.
  'contract.sets.1': 'Um grupo',
  'contract.sets.2': 'Dois grupos',
  'contract.sets.3': 'Três grupos',
  'contract.sets.n': '{n} grupos',
  'contract.runs.1': 'Uma sequência',
  'contract.runs.2': 'Duas sequências',
  'contract.runs.3': 'Três sequências',
  'contract.runs.n': '{n} sequências',
  'contract.any': 'Qualquer combinação válida',
  'contract.cleanRunOnly':
    'Qualquer mistura de grupos e sequências — pelo menos uma sequência tem de ser sem joker',
  'contract.cleanRunSuffix': '{base} — uma sequência tem de ser sem joker',

  // --- written rules ---------------------------------------------------------
  // Full-sentence keys a "see the rules" screen renders, one per module,
  // resolved server-side against the table's actual variation and options —
  // see internal/module/rules.go and each game's rules.go. Shared section
  // titles first, then one block per module in the order the four games were
  // added.
  'zolik.rules.section.goal': 'Objetivo',
  'zolik.rules.section.setup': 'Preparação',
  'zolik.rules.section.turn': 'A tua vez',
  'zolik.rules.section.melding': 'Baixar',
  'zolik.rules.section.end': 'Como acaba a partida',
  'zolik.rules.goal':
    'Sê o primeiro a esvaziar a mão baixando grupos e sequências válidos, juntando o menor número possível de pontos de penalização nas cartas que ainda tiveres quando outra pessoa sair.',
  'zolik.rules.deal': 'Cada jogador recebe {n} cartas.',
  'zolik.rules.meldShapes':
    'Um grupo são {set}+ cartas do mesmo valor; uma sequência são {run}+ cartas seguidas do mesmo naipe.',
  'zolik.rules.turn.draw': 'Na tua vez, compra uma carta — do baralho ou do monte de descartes.',
  'zolik.rules.pickup.topOnly': 'Só a carta do topo do monte de descartes pode ser tirada.',
  'zolik.rules.pickup.anyFromPile':
    'Qualquer carta do monte de descartes pode ser tirada, juntamente com tudo o que estiver por cima.',
  'zolik.rules.pickup.locked': 'Não se pode comprar do monte de descartes antes da ronda {n}.',
  'zolik.rules.pickup.open': 'O monte de descartes está aberto desde a primeira ronda.',
  'zolik.rules.turn.discard': 'Termina a tua vez descartando uma carta.',
  'zolik.rules.jokers.restricted':
    'Um joker nunca pode ser descartado, exceto se for exatamente a carta que te esvazia a mão.',
  'zolik.rules.lead.rotate': 'A mão avança um lugar em cada ronda, independentemente de quem ganhou.',
  'zolik.rules.lead.winner': 'Quem sai abre a ronda seguinte.',
  'zolik.rules.meldFloor.on':
    'A tua primeira baixa tem de somar pelo menos {n} pontos naturais para contares como baixado.',
  'zolik.rules.meldFloor.off': 'Não há mínimo de pontos na tua primeira baixa.',
  'zolik.rules.cleanRun.on':
    'Pelo menos uma das tuas sequências tem de ser totalmente sem jokers para contares como baixado.',
  'zolik.rules.cleanRun.off': 'As tuas sequências podem usar jokers à vontade — nenhuma tem de ser sem eles.',
  'zolik.rules.contracts.rotating':
    'A partida tem {n} rondas, e cada ronda exige a sua própria combinação de grupos e sequências.',
  'zolik.rules.contracts.static':
    'Todas as rondas exigem a mesma combinação: {sets} grupos e {runs} sequências.',
  'zolik.rules.end.afterDeals': 'A partida acaba ao fim de {n} rondas.',
  'zolik.rules.end.atScore': 'Continua a dar-se cartas até alguém chegar a {n} pontos — aí acaba.',

  'prsi.rules.section.goal': 'Objetivo',
  'prsi.rules.section.setup': 'Preparação',
  'prsi.rules.section.turn': 'A tua vez',
  'prsi.rules.section.special': 'Cartas especiais',
  'prsi.rules.section.end': 'Como acaba a partida',
  'prsi.rules.goal': 'Sê o primeiro a jogar todas as cartas da tua mão.',
  'prsi.rules.deck': 'Joga-se com um baralho de {value} cartas (do 7 para cima).',
  'prsi.rules.deal': 'Cada jogador começa com {n} cartas.',
  'prsi.rules.turn.match':
    'Joga uma carta que coincida no naipe ou no valor com a do topo — ou compra, se não puderes.',
  'prsi.rules.turn.draw': 'Comprar termina a tua vez sem jogares.',
  'prsi.rules.sevens':
    'Joga um 7 e o jogador seguinte compra duas cartas, a menos que responda com um 7 dele.',
  'prsi.rules.aces': 'Joga um ás e o jogador seguinte perde a vez.',
  'prsi.rules.queens': 'Joga uma dama e diz o naipe que continua.',
  'prsi.rules.end': 'A partida acaba no momento em que a mão de alguém fica vazia.',

  'canasta.rules.section.goal': 'Objetivo',
  'canasta.rules.section.setup': 'Preparação',
  'canasta.rules.section.melding': 'Baixar',
  'canasta.rules.section.end': 'Como acaba a partida',
  'canasta.rules.goal': 'Joga-se a pares; o primeiro lado a chegar a {n} pontos ganha a partida.',
  'canasta.rules.deck': 'Joga-se com {value} cartas — dois baralhos mais jokers.',
  'canasta.rules.deal': 'Cada jogador recebe {n} cartas.',
  'canasta.rules.redThrees':
    'Um três vermelho na tua mão é mostrado logo e conta como bónus — a não ser que o teu lado nunca complete uma canastra, e então conta contra ti.',
  'canasta.rules.canasta': 'Uma canastra é uma combinação de {n} ou mais cartas do mesmo valor.',
  'canasta.rules.meldFloorBands':
    'A tua primeira baixa tem de atingir um mínimo de pontos que sobe com a tua pontuação: {negative} abaixo de zero, {low} até 1500, {mid} até 3000, {high} acima disso.',
  'canasta.rules.oneCanastaToGoOut': 'Uma canastra completa chega para o teu lado sair.',
  'canasta.rules.twoCanastasToGoOut': 'O teu lado precisa de duas canastras completas antes de poder sair.',
  'canasta.rules.end': 'Continua a dar-se cartas até um lado passar os {n} pontos — aí a partida acaba.',

  'holdem.rules.section.goal': 'Objetivo',
  'holdem.rules.section.setup': 'Preparação',
  'holdem.rules.section.betting': 'Apostas',
  'holdem.rules.section.end': 'Como acaba a partida',
  'holdem.rules.goal': 'Ganha fichas tendo a melhor mão no showdown, ou ficando o único jogador na mão.',
  'holdem.rules.stack': 'Cada lugar começa com {n} fichas.',
  'holdem.rules.blinds': 'A small blind é {sb} e a big blind {bb}, postas antes de as cartas serem dadas.',
  'holdem.rules.streets': 'Aposta-se em quatro rondas — antes do flop e depois do flop, do turn e do river.',
  'holdem.rules.showdown':
    'Quem ainda estiver na mão mostra as cartas; a melhor mão de cinco cartas leva o pote.',
  'holdem.rules.noLimit': 'Sem limite — qualquer aposta pode ir até à totalidade do teu stack.',
  'holdem.rules.lastPlayerStanding': 'Joga-se até um lugar ter todas as fichas.',
  'holdem.rules.mostChipsWins': 'Quem tiver mais fichas quando o jogo parar ganha a partida.',
  'holdem.rules.handLimit': 'O jogo para ao fim de {n} mãos.',

  // --- header --------------------------------------------------------------
  'header.deal': 'Ronda {n}',
  'header.gameOf': 'Jogo {n} de {total}',
  'header.gameOfWithContract': 'Jogo {n} de {total}: {contract}',

  // --- meld preview --------------------------------------------------------
  'preview.validSet': 'Grupo válido',
  'preview.validRun': 'Sequência válida',
  'preview.validMeld': 'Combinação válida',
  'preview.notYet': 'Ainda não é combinação',
  'preview.points': '{shape} · {n} pontos',
  'preview.pointsWithLaid': '{shape} · {n} + {laid} já baixados = {total} pontos',
  'preview.meetsFloor': '{line} (chega a {n} ✓)',
  'preview.needsFloor': '{line} (precisa de {n} ✗)',
  'preview.becauseOf': '{line} — {reason}',

  // --- discard -------------------------------------------------------------
  // A discard with a meld staged lays that meld first; this is what the
  // player is told when the server refuses it and the whole move is rolled
  // back. {reason} is the engine's own words for the refusal.
  'discard.meldRejected': '{reason} — não se descartou nada, as tuas cartas continuam preparadas.',

  // --- what the current selection would send -------------------------------
  // The client's own reason a control is not ready, next to the engine's own
  // `err.*` reasons rather than a second kind of message — a control greyed
  // out for "not your turn" and one greyed out for "you picked two, this
  // takes one" should read as the same kind of thing, not one of them looking
  // broken. See `fits` in `src/lib/drops.ts`.
  'sel.tooMany.1': 'Escolhe apenas uma carta',
  'sel.tooMany.n': 'Escolhe no máximo {n} cartas',
  'sel.needMore': 'Escolhe {n} carta(s)',
  'sel.notThese': 'Essas cartas não podem ir aqui',
  'sel.needsCompany': 'Essa carta precisa das que estão ao lado',

  // --- what a module says happened -----------------------------------------
  // The first module-sent keys in this bundle, and deliberately few. Anything
  // a module sends is still legible without an entry here — `humanise` turns
  // `holdem.seat.stack` into "Stack" — so a key earns a line only when that
  // fallback loses something. These do: what each of them means lives in its
  // *params* (who won, how much, whose cards), and a key on its own has
  // nowhere to put them. Without the entry the player read "Winner", full
  // stop, at the end of a match they had just won.
  'status.winner': 'Ganho por {winners}',
  'holdem.status.pot': '{winners} ganha {amount} com {hand}',
  'holdem.status.potUncontested': '{winners} ganha {amount} — todos os outros desistiram',
  'holdem.status.shown': '{playerId} mostrou {value}',
  'holdem.prompt.waitingFor': 'À espera de {playerId}',

  // How a rummy scoreboard was ordered. Both earn a line for the same reason
  // as the keys above: the number lives in the params, and `humanise` would
  // render "Deals Won" with nothing after it.
  'zolik.standing.dealsWon': 'Rondas ganhas {n}',
  'zolik.standing.inHand': 'Na mão {n}',

  // --- rounds ---------------------------------------------------------------
  // What a round is called, and what one did. Each of these carries its number
  // in the params, so `humanise` alone would render a label with nothing after
  // it.
  // An instruction, not a heading. "Next round" named the thing rather than
  // asking for it, and sat above "Waiting for 1" — which reads as though the
  // player is the one waiting, when the table is waiting for them.
  'round.continue': 'Começar a ronda seguinte',

  // --- the flash that announces a round or a match ending ------------------
  //
  // One line and one number, held for about two seconds. Worded generically on
  // purpose: which *kind* of ending it was is the module's own fact, printed
  // underneath, and a client that phrased "went gin" itself would be a client
  // that knows a game.
  'flash.roundWon': '{winners} levou-a',
  'flash.roundWonYou': 'Levaste-a',
  'flash.roundDrawn': 'Ninguém a levou',
  'flash.matchOver': 'Partida terminada',
  'flash.matchWon': '{winners} ganha',
  'flash.matchWonYou': 'Ganhaste',
  'flash.matchDrawn': 'Ninguém ganha',
  'flash.nowOn': 'agora {total}',

  'zolik.round.deal': 'Ronda',
  'zolik.round.cleanRun': 'Uma sequência tem de ser sem joker',
  'canasta.round.deal': 'Ronda',
  'canasta.round.concealed': 'Saiu de mão fechada',
  'canasta.round.exhausted': 'O baralho acabou',
  'canasta.round.meldCards': 'Cartas baixadas {n}',
  'canasta.round.canastas': 'Canastras {n}',
  'canasta.round.redThrees': 'Treses vermelhos {n}',
  'canasta.round.goingOut': 'Saída {n}',
  'canasta.round.inHand': 'Apanhado na mão {n}',
  'holdem.round.hand': 'Mão',
  'holdem.round.pot': 'Pote {n}',
  'holdem.round.uncontested': 'Todos os outros desistiram',
  'seat.ready': 'Pronto',



  // --- refusals the other three games raise ---------------------------------
  // Worded here for the same reason Žolíky's are: reasonText falls back to the
  // code, and OfferBar hands it the code as that fallback, so an unworded
  // refusal reaches a player as PILE_FROZEN in capitals. serverKeys.test.ts
  // now fails the build rather than letting the next one through.
  'err.SET_TOO_LARGE': 'Um grupo já tem os quatro naipes',
  'err.DISCARD_TAKEN_CARD_FORBIDDEN':
    'Não podes descartar a carta que acabaste de tirar — joga-a ou fica com ela',
  'err.CARD_DOES_NOT_FIT': 'Essa carta não coincide nem no naipe nem no valor',
  'err.SUIT_REQUIRED': 'Diz o naipe que continua',
  'err.MUST_ANSWER_DRAW_OR_TAKE': 'Responde com um sete, ou leva as cartas',
  'err.NOTHING_TO_DRAW': 'Não resta nada para comprar',
  'err.PILE_EMPTY': 'O monte está vazio',
  'err.PILE_BLOCKED': 'O monte está bloqueado — está um três preto por cima',
  'err.PILE_FROZEN': 'O monte está congelado — precisas de duas cartas naturais do valor da carta do topo',
  'err.TOP_CARD_UNUSABLE': 'Não podes usar a carta do topo',
  'err.MELD_CLOSED': 'Essa combinação está completa e fechada',
  'err.MELD_TOO_SMALL': 'Uma combinação precisa de mais cartas do que isso',
  'err.MELD_TOO_LARGE': 'Essa combinação já não aceita mais cartas',
  'err.MELD_MIXED_RANKS': 'Todas as cartas de uma combinação têm de ter o mesmo valor',
  'err.NOT_ENOUGH_NATURALS': 'Uma combinação precisa de mais cartas naturais do que jokers',
  'err.RANK_ALREADY_MELDED': 'O teu lado já tem uma combinação desse valor',
  'err.NOT_YOUR_MELD': 'Essa combinação é do lado adversário',
  'err.NO_SUCH_MELD': 'Essa combinação não está na mesa',
  'err.CANNOT_MELD_THREE': 'Os treses nunca se baixam',
  'err.CANNOT_DISCARD_RED_THREE': 'Um três vermelho não pode ser descartado',
  'err.MUST_KEEP_A_CARD': 'Fica com pelo menos uma carta — assim não podes esvaziar a mão',
  'err.MUST_MELD_FIRST': 'Baixa primeiro a abertura do teu lado',
  'err.INITIAL_MELD_NOT_MET': 'À tua primeira baixa ainda faltam pontos',
  'err.CANNOT_GO_OUT_YET': 'O teu lado precisa de uma canastra completa antes de poder sair',
  'err.NOTHING_TO_CALL': 'Não há aposta para igualar',
  'err.CANNOT_CHECK': 'Não podes passar — há uma aposta para responder',
  'err.CANNOT_RAISE': 'Aqui não podes subir',
  'err.RAISE_TOO_SMALL': 'Uma subida tem de valer pelo menos o mesmo que a anterior',
  'err.NOT_ENOUGH_CHIPS': 'Não tens tantas fichas',
  'err.AMOUNT_REQUIRED': 'Diz quanto',
  'err.AMOUNT_NOT_A_NUMBER': 'Esse valor não é um número',
  'err.SEAT_NOT_IN_HAND': 'Não estás nesta mão',
  'err.WRONG_RANK': 'Essa carta tem o valor errado para isto',
  'err.MATCH_FULL': 'A mesa está cheia',
  'err.MATCH_ALREADY_STARTED': 'A partida já começou',
  'err.TOO_FEW_PLAYERS': 'Ainda não há jogadores suficientes',
  'err.WRONG_PLAYER_COUNT': 'Este jogo não se joga com esse número de jogadores',
  'err.NOT_THE_HOST': 'Só o anfitrião pode fazer isso',
  'err.NO_LONGER_WAITING': 'A mesa já não está à espera',
  'err.WAITING_ROOM_UNAVAILABLE': 'A sala de espera não está disponível',
  'err.SERVER_BUSY': 'O servidor está cheio neste momento — tenta daqui a pouco',


  // --- rules the engine has always enforced and never stated ----------------
  // Every one of these was found by the guardrail rather than by review: the
  // validator could refuse a player for it, and no sentence anywhere said so.
  // See server/internal/zolikmod/ruleindex_test.go.
  'zolik.rules.section.layoff': 'Juntar a combinações',
  'zolik.rules.pickup.obligation':
    'Antes de estares baixado, uma carta tirada do monte de descartes tem de ser usada na combinação com que baixas nesta jogada.',
  'zolik.rules.pickup.noReturn':
    'Uma carta tirada do monte de descartes não pode ser descartada outra vez na mesma jogada — joga-a ou fica com ela.',
  'zolik.rules.wilds.setLimit': 'Um grupo não pode ter mais jokers do que cartas naturais.',
  'zolik.rules.set.maxSize':
    'Um grupo não pode ter mais de {n} cartas — um joker substitui um naipe em falta, não acrescenta a um grupo completo.',
  'zolik.rules.run.maxLength':
    'Uma sequência não pode ter mais de {n} cartas — o ás em baixo, os doze valores acima e o ás em cima.',
  'zolik.rules.run.aceBridge':
    'O ás fica acima do rei ou abaixo do dois, nunca a ligar as duas pontas de uma sequência.',
  'zolik.rules.contracts.contribution':
    'Até estares baixado, cada combinação que baixares tem de ser uma que o contrato da ronda ainda peça.',
  'zolik.rules.layoff.afterDown':
    'Não podes juntar nada às combinações dos outros enquanto não baixares o teu próprio contrato.',
  'zolik.rules.layoff.runEnds': 'Uma carta juntada a uma sequência tem de a continuar numa das duas pontas.',
  'zolik.rules.jokers.swap':
    'Um joker numa combinação na mesa pode ser recomprado com a carta exata que representa.',
  'zolik.rules.jokers.reclaim.on':
    'Um joker recomprado da mesa tem de ser jogado numa combinação na mesma jogada — não pode ficar na mão.',
  'zolik.rules.jokers.reclaim.off': 'Um joker recomprado da mesa pode ficar na mão.',
  'zolik.rules.deck.reshuffle':
    'Quando o baralho acaba, o monte de descartes é baralhado e passa a ser o novo baralho; se ambos estiverem vazios, a ronda acaba.',


  // --- what to do instead ---------------------------------------------------
  // The third layer of a refusal, after the reason and the rule. Sent by the
  // module because it is the only side that knows which card is owed and
  // which way out is on offer; see server/internal/zolikmod/remedy.go.
  'zolik.remedy.meldThePickup': 'Junta {card} à tua baixa, ou anula a apanha.',
  'zolik.remedy.discardSomethingElse': 'Descarta outra carta, ou joga {card} nesta jogada.',
  'zolik.remedy.discardNotAJoker': 'Descarta outra coisa que não um joker.',
  'zolik.remedy.finishOrUndoLayDown': 'Termina a tua baixa, ou volta atrás.',
  'zolik.remedy.needMorePoints': 'Faltam-te {n} pontos para poderes baixar.',
  'zolik.remedy.layACleanRun': 'Baixa uma sequência sem nenhum joker.',
  'zolik.remedy.playReclaimedJoker': 'Joga {card} numa combinação, ou anula a recolha.',
  'zolik.remedy.goDownFirst': 'Baixa primeiro as tuas combinações.',
  'zolik.remedy.drawFirst': 'Compra primeiro uma carta.',
  'zolik.remedy.drawFromStock': 'Compra do baralho — o monte de descartes abre na ronda {n}.',
  'zolik.remedy.drawFromStockEmpty': 'Compra antes do baralho.',


  // --- keys whose meaning lives in their params -----------------------------
  // The shape fallback renders these with the number missing ("Stack", "Pot"),
  // which is why they earn a line where most keys do not.
  'header.contract': 'Precisa de {sets} grupos e {runs} sequências',
  'header.contract.cleanRunOnly': 'Precisa de uma sequência sem joker',
  'header.round': 'Ronda {n}',
  'header.deck': 'Baralho',
  'header.target': 'Objetivo',
  'header.suitInPlay': 'Naipe em jogo',
  'seat.cards': 'Cartas',
  'zolik.offer.meld': 'Baixar',
  'prompt.pickupMustBeMelded':
    '{value} veio do monte de descartes — tem de entrar nas combinações com que baixas nesta jogada.',
  'prompt.jokerMustBePlayed':
    '{value} veio da mesa — tem de entrar numa combinação antes de poderes terminar a tua vez.',
  'prompt.initialMeld': 'A abertura do teu lado tem de chegar a {n} pontos.',
  'prompt.canastasNeeded': 'Ao teu lado faltam {n} canastras para poder sair.',
  'prompt.mustDrawOrAnswerSeven': 'Responde com um sete, ou compra {n} cartas.',
  'prompt.chooseSuit': 'Escolhe o naipe que continua',
  'prompt.skipPending': 'Perdes a vez',
  'status.lastDeal': 'A equipa {team} fez {value}',
  'status.teamScore': 'Equipa {team}: {value}',
  'canasta.offer.rank': 'Valor',
  'canasta.seat.teamScore': 'Pontos da equipa',
  'canasta.seat.canastas': 'Canastras',
  'holdem.header.pot': 'Pote',
  'holdem.header.street': 'Ronda',
  'holdem.header.hand': 'Mão',
  'holdem.header.handLimit': 'Mãos no total',
  'holdem.header.blinds': 'Blinds',
  'holdem.cost.call': 'para igualar',
  'holdem.cost.pot': 'no pote',
  'holdem.seat.stack': 'Stack',
  'holdem.seat.bet': 'Aposta',
  'holdem.prompt.yourAction': 'É a tua vez',
  'holdem.prompt.raiseTo': 'Subir para',
  'zone.yourHand': 'A tua mão',
  'zone.opponentHand': 'A mão dele',
  'zone.drawPile': 'Baralho',
  'zone.discardPile': 'Monte de descartes',
  'zone.melds': 'Combinações',
  'zone.teamMelds': 'Combinações do teu lado',
  'zone.redThrees': 'Treses vermelhos',
  'zone.board': 'Mesa',
  'verb.drawFromDeck': 'Comprar',
  'verb.takeFromDiscard': 'Tirar do monte',


  // --- the why sheet's own furniture ---------------------------------------
  // The three layers a refusal is explained in. Labels, not sentences: what
  // goes under each is sent by the server or worded above.
  'why.reason': 'Porque não',
  'why.rule': 'A regra',
  'why.rules': 'As regras',
  'why.remedy': 'O que podes fazer',
  'why.readTheRules': 'Ler as regras completas →',
  'why.close': 'Fechar',
  'why.open': 'porquê',

  // --- marks on a particular card ------------------------------------------
  // A mark is a refusal that has not happened yet: the rule is enforced at the
  // discard, which is the last possible moment to hear about it, so the card
  // says so while there is still a turn left to act on it.
  'zolik.badge.owedToMeld':
    '{card} veio do monte de descartes — tem de entrar nas combinações com que baixas nesta jogada.',
  'zolik.badge.jokerOwed':
    '{card} veio da mesa — tem de entrar numa combinação antes de poderes terminar a tua vez.',

  // --- the legal notices ----------------------------------------------------
  // Only the furniture. The documents themselves are in `src/legal`, which is
  // a bundle of the same kind with a parity test of its own — prose that long
  // in a flat key map buries the keys this one exists for.
  //
  // The notice is five fragments rather than one sentence with two links glued
  // in, because Czech does not put the link where English does: "souhlasíš s
  // Podmínkami" inflects the noun the link is made of. Fragments let each
  // locale place and decline its own.
  'legal.terms': 'Termos',
  'legal.privacy': 'Privacidade',
  'legal.source': 'Código-fonte',
  'legal.updated': 'Versão {version}',
  'legal.draft':
    'Rascunho — ainda não em vigor. Falta preencher o nome, o país e o endereço de contacto do operador.',
  'legal.notice.before': 'Ao jogares, aceitas os ',
  'legal.notice.terms': 'termos de utilização',
  'legal.notice.between': '. O que é guardado sobre ti está no ',
  'legal.notice.privacy': 'aviso de privacidade',
  'legal.notice.after': '.',
  // --- gin rummy -------------------------------------------------------------
  'err.UPCARD_DECLINED': 'Já passaste a essa carta',
  'err.DEADWOOD_TOO_HIGH': 'O teu deadwood é alto demais para bateres',
  'err.CARD_DOES_NOT_EXTEND_MELD': 'Essa carta não prolonga esta combinação',
  'ginrummy.rules.setup': 'Preparação',
  'ginrummy.rules.turn': 'A tua vez',
  'ginrummy.rules.melds': 'Combinações',
  'ginrummy.rules.knocking': 'Bater',
  'ginrummy.rules.bigGin': 'Big gin',
  'ginrummy.rules.layoff': 'O encosto',
  'ginrummy.rules.deadHand': 'A mão morta',
  'ginrummy.rules.scoring': 'Pontuar uma mão',
  'ginrummy.rules.match': 'Ganhar a partida',
  'ginrummy.rules.lineBonuses': 'Bónus da contagem',
  'ginrummy.rules.deck': 'Joga-se com um baralho de {value} cartas.',
  'ginrummy.rules.deal': 'Cada jogador recebe {value} cartas.',
  'ginrummy.rules.upcard': 'Vira-se mais uma carta para começar o monte de descartes.',
  'ginrummy.rules.drawDiscard':
    'Na tua vez, compra uma carta — do baralho ou do monte de descartes — e depois descarta uma.',
  'ginrummy.rules.setsAndRuns':
    'Uma combinação é um grupo de três ou quatro cartas do mesmo valor, ou uma sequência de três ou mais no mesmo naipe.',
  'ginrummy.rules.aceLow': 'O ás é sempre baixo — não existe sequência de dama a ás.',
  'ginrummy.rules.knockLimit': 'Podes bater assim que o teu deadwood for {n} ou menos.',
  'ginrummy.rules.oklahoma': 'O limite para bater nesta mão é dado pelo valor da carta virada.',
  'ginrummy.rules.gin': 'Deadwood zero é gin — a melhor batida possível.',
  'ginrummy.rules.bigGinBonus':
    'Onze cartas todas combinadas, sem sequer descartar, é big gin, e vale mais {n} pontos.',
  'ginrummy.rules.layoffDescription':
    'Depois de uma batida que não seja gin, o teu adversário pode encostar o próprio deadwood às tuas combinações antes de as mãos serem comparadas.',
  'ginrummy.rules.deadHandDescription':
    'Se o baralho descer às duas últimas cartas e ninguém tiver batido, a mão está morta — ninguém pontua e o mesmo dador dá outra vez.',
  'ginrummy.rules.undercut':
    'Se o deadwood do teu adversário não for maior do que o teu, ele corta-te: marca a diferença, mais {n}.',
  'ginrummy.rules.ginBonus': 'O gin marca a mão inteira do teu adversário, mais {n}.',
  'ginrummy.rules.target': 'O primeiro a passar {n} pontos no fim de uma mão ganha a partida.',
  'ginrummy.rules.shutout':
    'O bónus de partida duplica para {n} se o perdedor não tiver marcado um único ponto.',
  'ginrummy.rules.box': 'Cada mão que ganhaste vale {n} pontos no fim da partida.',
  'ginrummy.rules.gameBonus': 'Ganhar a partida vale mais {n} pontos.',
  'ginrummy.fact.deadwood': '{value} de deadwood',
  'ginrummy.fact.discardCard': 'Descartar {value}',
  'ginrummy.fact.meldCards': 'Em {value}',
  'ginrummy.header.hand': 'Mão {n}',
  'ginrummy.round.kind': '{value}',
  'ginrummy.round.hand': 'Mão',
  'ginrummy.seat.score': '{value}',
  'ginrummy.seat.dealer': 'Dador',
  'ginrummy.status.knocked': '{playerId} bateu com {deadwood} de deadwood',
  'ginrummy.status.gin': '{playerId} fez gin',
  'ginrummy.status.lastHand': 'Última mão: {winner} ({kind}, {delta} pontos)',
  'ginrummy.offer.drawStock': 'Comprar do baralho',
  'ginrummy.offer.drawDiscard': 'Comprar do monte de descartes',
  'ginrummy.offer.takeUpcard': 'Tirar a carta virada',
  'ginrummy.offer.passUpcard': 'Passar',
  'ginrummy.offer.discard': 'Descartar',
  'ginrummy.offer.knock': 'Bater',
  'ginrummy.offer.gin': 'Gin!',
  'ginrummy.offer.bigGin': 'Big gin!',
  'ginrummy.offer.layOff': 'Encostar',
  'ginrummy.offer.finishLayoff': 'Acabei de encostar',
  'ginrummy.zone.knockerHand': 'Mão que bateu',
  'ginrummy.zone.melds': 'Combinações',
  'ginrummy.prompt.upcardDecision': 'Tira a carta virada, ou passa',
  'ginrummy.prompt.yourTurnDraw': 'Compra uma carta',
  'ginrummy.prompt.yourTurnDiscard': 'Descarta — ou bate, se puderes',
  'ginrummy.prompt.layoff': 'Encosta deadwood, ou termina',

  // --- rummy tiles -------------------------------------------------------------
  'err.TILE_NOT_IN_HAND': 'Essa peça não está na tua mão',
  'err.TILE_DOES_NOT_FIT': 'Isso não encaixa aí',
  'err.NO_SUCH_SET': 'Essa combinação não está na mesa',
  'err.INITIAL_MELD_ONLY':
    'Antes da tua primeira baixa só podes reorganizar as tuas próprias combinações novas',
  'err.TABLE_NOT_VALID': 'A mesa ainda não é válida',
  'err.TRAY_NOT_EMPTY': 'Ainda tens peças soltas por colocar',
  'err.NOTHING_PLAYED': 'Joga pelo menos uma peça antes de terminares a tua vez',
  'err.INITIAL_MELD_TOO_LOW': 'A tua primeira baixa tem de valer 30 pontos ou mais',
  'err.NOT_A_RUN': 'Só uma sequência pode ser dividida',
  'err.BAD_SPLIT_POSITION': 'Não é aí que esta sequência se pode dividir',
  'err.NO_JOKER_IN_SET': 'Não há nenhum joker nessa combinação',
  'err.TILE_JOKER_SWAP_MISMATCH': 'Essa peça não é o que o joker representa',
  'rummytiles.rules.setup': 'Preparação',
  'rummytiles.rules.sets': 'Combinações',
  'rummytiles.rules.initialMeld': 'A baixa inicial',
  'rummytiles.rules.turn': 'A tua vez',
  'rummytiles.rules.jokerTaking': 'Tirar um joker',
  'rummytiles.rules.ending': 'Terminar uma ronda',
  'rummytiles.rules.poolExhaustion': 'Se o monte se esgotar',
  'rummytiles.rules.match': 'Ganhar a partida',
  'rummytiles.rules.tiles': 'Joga-se com {value} peças.',
  'rummytiles.rules.dealCount': 'Cada jogador recebe {value} peças.',
  'rummytiles.rules.group': 'Um grupo são três ou quatro peças do mesmo número, cada uma de cor diferente.',
  'rummytiles.rules.run': 'Uma sequência são três ou mais números seguidos da mesma cor.',
  'rummytiles.rules.noWrap': 'O 13 não volta a ligar ao 1.',
  'rummytiles.rules.joker': 'Um joker representa qualquer peça.',
  'rummytiles.rules.initialMeldDescription':
    'Enquanto não tiveres baixado {n} pontos ou mais numa só jogada, apenas da tua própria mão, não podes mexer em nada do que já está na mesa.',
  'rummytiles.rules.turnDescription':
    'Joga pelo menos uma peça da tua mão, reorganizando a mesa à vontade, e termina com todas as combinações da mesa válidas.',
  'rummytiles.rules.noDiscard':
    'Não há descarte — se não conseguires completar uma jogada válida, compras uma peça em vez disso.',
  'rummytiles.rules.jokerTakingDescription':
    'Um joker na mesa pode ser tirado substituindo-o pela peça que representa, vinda da tua mão — e tem de ser usado numa combinação antes de a tua vez acabar.',
  'rummytiles.rules.goingOut':
    'O primeiro jogador a ficar sem peças ganha a ronda. Todos os outros marcam o valor negativo do que lhes resta; o vencedor marca a soma do que todos os outros perderam.',
  'rummytiles.rules.poolExhaustionLowestWins':
    'Se o monte se esgotar e ninguém puder jogar, a ronda acaba e ganha-a a mão de menor valor.',
  'rummytiles.rules.poolExhaustionNoWinner':
    'Se o monte se esgotar e ninguém puder jogar, a ronda acaba sem vencedor — cada mão é simplesmente contada.',
  'rummytiles.rules.target': 'O primeiro a passar {n} pontos no fim de uma ronda ganha a partida.',
  'rummytiles.rules.roundLimit': 'A partida acaba ao fim de {n} rondas — ganha a pontuação mais alta.',
  'rummytiles.fact.setCards': '{value}',
  'rummytiles.header.pool': 'Monte {n}',
  'rummytiles.header.round': 'Ronda {n}',
  'rummytiles.round.kind': '{value}',
  'rummytiles.round.round': 'Ronda',
  'rummytiles.seat.score': '{value}',
  'rummytiles.seat.notOpened': 'Sem abrir',
  'rummytiles.status.lastRound': 'Última ronda: {winner} ({kind})',
  'rummytiles.badge.invalid': 'Ainda não válido',
  'rummytiles.zone.pool': 'Monte',
  'rummytiles.zone.table': 'Mesa',
  'rummytiles.zone.tray': 'Suporte',
  'rummytiles.offer.place': 'Colocar',
  'rummytiles.offer.addFromHand': 'Juntar',
  'rummytiles.offer.addFromTray': 'Juntar do suporte',
  'rummytiles.offer.take': 'Tirar',
  'rummytiles.offer.split': 'Dividir',
  'rummytiles.offer.swapJoker': 'Trocar o joker',
  'rummytiles.offer.resetTurn': 'Reiniciar a jogada',
  'rummytiles.offer.commit': 'Pronto',
  'rummytiles.offer.draw': 'Comprar',
  'rummytiles.param.position': 'Dividir em',

  // --- blackjack -----------------------------------------------------------
  //
  // The house rules are worded twice over — on and off — because a rule that
  // is *not* in force is still something a player has to be told. "No
  // surrender at this table" is information; silence is a guess.
  'err.BET_BELOW_MINIMUM': 'Isso está abaixo do mínimo da mesa',
  'err.ALREADY_BET': 'A tua aposta já está feita',
  'err.INSURANCE_CLOSED': 'Neste momento não há seguro a tomar',
  'err.CANNOT_DOUBLE': 'Esta mão não pode ser dobrada',
  'err.CANNOT_SPLIT': 'Esta mão não pode ser dividida',
  'err.CANNOT_SURRENDER': 'Esta mão não pode ser desistida',

  'blackjack.rules.section.table': 'A mesa',
  'blackjack.rules.section.play': 'Jogar uma mão',
  'blackjack.rules.section.dealer': 'O dador',
  'blackjack.rules.section.end': 'Como acaba a partida',
  'blackjack.rules.goal':
    'Vence o dador sem passares dos vinte e um. Passar perde logo, faça o dador o que fizer a seguir.',
  // Counts are their own phrase rather than a number glued to a noun: one
  // deck and six decks inflect differently, and Czech inflects them again.
  'blackjack.rules.decks': 'Baralhos no sabot: {n}.',
  'blackjack.rules.stack': 'Cada lugar senta-se com {n} fichas.',
  'blackjack.rules.minBet': 'O mínimo da mesa é {n} fichas.',
  'blackjack.rules.faceUp':
    'As cartas dos jogadores são dadas viradas para cima; o dador mantém uma carta tapada até todos terem jogado.',
  'blackjack.rules.hitStand': 'Pede as cartas que quiseres, ou fica com o que tens.',
  'blackjack.rules.aces': 'Um ás vale onze enquanto isso couber, e um quando não couber.',
  'blackjack.rules.blackjack': 'Um ás com uma carta de valor dez, nas duas primeiras cartas, é um blackjack.',
  'blackjack.rules.pays3to2': 'Um blackjack paga 3:2.',
  'blackjack.rules.pays6to5': 'Um blackjack paga 6:5.',
  'blackjack.rules.paysEven': 'Um blackjack paga a par.',
  'blackjack.rules.double':
    'Nas tuas duas primeiras cartas podes dobrar a aposta e receber exatamente mais uma carta.',
  'blackjack.rules.doubleAfterSplit': 'Uma mão saída de uma divisão também pode ser dobrada.',
  'blackjack.rules.noDoubleAfterSplit': 'Uma mão saída de uma divisão não pode ser dobrada.',
  'blackjack.rules.split':
    'Duas cartas do mesmo valor podem ser divididas em mãos próprias, cada uma com a sua aposta — até {n} vezes, num total de {hands} mãos.',
  'blackjack.rules.noSplit': 'Nesta mesa os pares não se dividem.',
  'blackjack.rules.splitAces':
    'Ases divididos recebem uma carta cada e ficam-se por aí, e um vinte e um assim obtido não é blackjack.',
  'blackjack.rules.surrender':
    'Podes desistir da tua primeira mão por metade da aposta, depois de o dador ter verificado se tem blackjack.',
  'blackjack.rules.noSurrender': 'Nesta mesa não se pode desistir de mãos.',
  'blackjack.rules.dealerDraws': 'O dador pede até dezassete e depois fica.',
  'blackjack.rules.hitsSoft17': 'O dador pede num dezassete formado com um ás.',
  'blackjack.rules.standsSoft17': 'O dador fica num dezassete formado com um ás.',
  'blackjack.rules.dealerPeeks':
    'Mostrando um ás ou um dez, o dador verifica se tem blackjack antes de alguém jogar.',
  'blackjack.rules.insurance':
    'Contra um ás do dador podes segurar por metade da tua aposta; paga 2:1 se o dador tiver blackjack.',
  'blackjack.rules.noInsurance': 'Nesta mesa não se oferece seguro.',
  'blackjack.rules.rounds': 'A mesa joga {n} rondas.',
  'blackjack.rules.mostChipsWins': 'Quem tiver mais fichas no fim ganha a partida.',
  'blackjack.rules.bustedOut':
    'Um lugar que já não consiga cobrir o mínimo de {n} fica de fora o resto da partida.',

  'blackjack.zone.dealer': 'Dador',
  'blackjack.zone.box': 'Mão',
  'blackjack.zone.yourBox': 'A tua mão',
  'blackjack.zone.shoe': 'Sabot',

  'blackjack.header.round': 'Ronda {n} de {of}',
  'blackjack.header.minBet': 'Mínimo',
  'blackjack.header.decks': 'Baralhos',
  'blackjack.header.dealerTotal': 'O dador mostra {n}',
  'blackjack.header.dealerSoftTotal': 'O dador mostra {n} suave',

  'blackjack.seat.stack': 'Fichas',
  'blackjack.seat.bet': 'Aposta',
  'blackjack.seat.insurance': 'Seguro',
  'blackjack.seat.total': 'Total',
  'blackjack.seat.softTotal': 'Total suave',
  'blackjack.seat.out': 'Sem fichas',

  'blackjack.prompt.placeBet': 'Faz a tua aposta',
  'blackjack.prompt.insurance': 'Seguro?',
  'blackjack.prompt.yourMove': 'É a tua vez',
  'blackjack.prompt.waitingFor': 'À espera de {playerId}',
  'blackjack.prompt.betAmount': 'Aposta',

  'blackjack.offer.bet': 'Apostar',
  'blackjack.offer.hit': 'Carta',
  'blackjack.offer.stand': 'Ficar',
  'blackjack.offer.double': 'Dobrar',
  'blackjack.offer.split': 'Dividir',
  'blackjack.offer.surrender': 'Desistir',
  'blackjack.offer.insure': 'Fazer seguro',
  'blackjack.offer.declineInsurance': 'Sem seguro',

  'blackjack.fact.tableMinimum': 'mínimo',
  'blackjack.fact.insuranceCost': 'para segurar',
  'blackjack.fact.extraStake': 'a apostar',
  'blackjack.fact.surrenderReturn': 'de volta',

  'blackjack.status.dealerBlackjack': 'O dador tinha blackjack',
  'blackjack.status.dealerBust': 'O dador rebentou com {n}',
  'blackjack.status.dealerStands': 'O dador fica em {n}',

  'blackjack.round.name': 'Ronda',
  'blackjack.round.dealerTotal': 'Dador {n}',
  'blackjack.round.dealerBust': 'Dador rebentou ({n})',
  'blackjack.round.dealerBlackjack': 'Blackjack do dador',
  'blackjack.round.outcome.blackjack': 'Blackjack',
  'blackjack.round.outcome.win': 'Ganha',
  'blackjack.round.outcome.push': 'Empate',
  'blackjack.round.outcome.lose': 'Perdida',
  'blackjack.round.outcome.bust': 'Rebentou',
  'blackjack.round.outcome.surrender': 'Desistiu',

  'blackjack.badge.inPlay': 'Em jogo',
  'blackjack.badge.doubled': 'Dobrada',
  'blackjack.badge.split': 'Dividida',
  'blackjack.badge.blackjack': 'Blackjack',
  'blackjack.badge.bust': 'Rebentou',
  'blackjack.badge.won': 'Ganha',
  'blackjack.badge.push': 'Empate',
  'blackjack.badge.lost': 'Perdida',
  'blackjack.badge.surrendered': 'Desistiu',

  'blackjack.unit.chips': 'fichas',

  // --- the settings screen --------------------------------------------------
  //
  // Hardcoded English until now, which was survivable while the only other
  // language was unreachable and faintly absurd once the language picker
  // itself lived on this screen: a player who cannot read "Settings" is
  // exactly the one who came here to change it.
  'settings.title': 'Definições',
  'settings.subtitle': 'O aspeto que tens tu, e o que a mesa tem',
  'settings.face.heading': 'A tua cara à mesa',
  'settings.face.account': 'Guardada com a tua conta, por isso segue-te para outro dispositivo.',
  'settings.face.device': 'Guardada neste dispositivo. Inicia sessão para a levares contigo.',
  'settings.skin.heading': 'Aspeto da mesa',
  'settings.language.heading': 'Idioma',
  'settings.language.status': 'Guardado neste dispositivo.',
  'settings.language.auto': 'Automático',
  'settings.language.auto.now': 'Segue o teu dispositivo — agora {language}',
  'settings.legal.heading': 'As letras pequenas',
  'settings.legal.status': 'O que aceitaste ao jogar, e o que é guardado sobre ti.',
  'settings.signIn': 'Iniciar sessão',
  'settings.back': 'Voltar',

  // A translated interface over an English notice is a worse lie than an
  // English interface, so the document says so itself rather than letting the
  // surrounding screen imply the text was written for this reader.
  'legal.untranslated':
    'Este aviso ainda não foi traduzido para o teu idioma. O texto em inglês abaixo é a versão que vale.',

  // --- the navigation bar's screen titles ------------------------------------
  //
  // These sit in `app/_layout.tsx`, above every screen, and were the last
  // English left after the settings screen was translated — a German player
  // reading "Einstellungen" under a bar that said "Settings". A translated app
  // with an untranslated chrome bar looks like a bug, not like a limit.
  'nav.home': 'Zolik',
  'nav.emailSignIn': 'Entrada por e-mail',
  'nav.signingIn': 'A entrar',
  'nav.usernameSignIn': 'Entrada com nome de utilizador',
  'nav.legacyAccount': 'Conta antiga',
  'nav.guest': 'Convidado',
  'nav.account': 'Conta',
  'nav.games': 'Jogos',
  'nav.table': 'A tua mesa',
  'nav.join': 'Juntar-te a uma mesa',
  // The screen a shared invite link lands on while it resolves the code.
  'nav.joining': 'A entrar na mesa',
  'nav.rules': 'Regras',
  'nav.match': 'Partida',
  'nav.scoreTable': 'Tabela de pontos',
  'nav.stats': 'Estatísticas',
};
