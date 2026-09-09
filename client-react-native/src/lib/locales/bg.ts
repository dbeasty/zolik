/**
 * Bulgarian. Cyrillic script. Rummy vocabulary: група for a set, поредица for a run, комбинация for a meld, жокер for a joker.
 */

export const bg: Record<string, string> = {
  // --- engine error codes (rules.RulesErrorCode) ---------------------------
  'err.NOT_YOUR_TURN': 'Не е твой ред',
  'err.WRONG_PHASE': 'В момента не е възможно',
  'err.MUST_DRAW_FIRST': 'Изтегли карта, преди да свалиш',
  'err.GAME_SUSPENDED': 'Играта е на пауза',
  'err.GAME_NOT_ACTIVE': 'Играта не тече',
  // --- runtime refusals, which are not the rules saying no -----------------
  //
  // Both were reachable long before they were worded, and a player who met
  // one was shown the code in capitals or, worse, nothing at all: an action
  // sent while the socket was down simply vanished, which reads as a legal
  // move being silently refused.
  'err.MATCH_NOT_ACTIVE': 'Масата е на пауза — чака се играч да се свърже отново',
  'err.NOT_CONNECTED': 'Няма връзка с масата — свързваме се отново, после опитай пак',
  // Why the "start the next round" control is greyed out. Without wording the
  // shell printed the code itself, so a player agreeing to go on was answered
  // with ALREADY_READY in capitals.
  'err.ALREADY_READY': 'Готов си',
  'err.NOT_BETWEEN_ROUNDS': 'Рундът още се играе',
  'err.NOT_AT_THIS_TABLE': 'Не си на тази маса',
  // Bringing a table back that the sweeper set aside. Refusals a player
  // can actually provoke: a link to a table that is gone, one that was
  // already picked up in another tab, and a table with other people at it.
  'err.MATCH_MOVED_ON': 'Масата продължи напред — презареди страницата',
  'err.MATCH_NOT_ABANDONED': 'Тази маса не чака да бъде подновена',
  'err.MATCH_NOT_FOUND': 'Тази маса вече не съществува',
  'err.TABLE_HAS_OTHER_PLAYERS': 'Може да се поднови само маса, на която всички останали са ботове',
  'err.DISCARD_LOCKED': 'Купчината за изхвърляне засега е заключена',
  'err.DISCARD_PILE_EMPTY': 'Купчината за изхвърляне е празна',
  'err.NO_CARDS_LEFT': 'Няма повече карти за теглене',
  'err.ROUND_REQ_NOT_MET': 'Първо свали собственото си отваряне',
  'err.NEED_CLEAN_RUN': 'Нужна ти е поредица без жокер на масата, за да се броиш за свалил',
  'err.INCOMPLETE_INITIAL_MELD': 'Довърши свалянето или го върни, преди да изхвърлиш',
  'err.DISCARD_CARD_NOT_MELDED': 'Картата, която взе, трябва да влезе в комбинацията ти',
  'err.JOKER_DISCARD_FORBIDDEN': 'Жокер не може да се изхвърля',
  'err.NOTHING_TO_UNDO': 'Няма какво да се връща',
  'err.NO_JOKER_IN_MELD': 'В тази комбинация няма жокер',
  'err.JOKER_SWAP_MISMATCH': 'Тази карта не заема мястото на жокера',
  'err.RECLAIMED_JOKER_NOT_MELDED': 'Жокерът, взет от масата, трябва да се изиграе в комбинация този ход',
  'err.RUN_TOO_LONG': 'Тази поредица вече е с пълна дължина',
  'err.WRONG_RUN_END': 'Тази карта удължава другия край на поредицата',
  'err.INVALID_MELD': 'Никоя карта в ръката ти не пасва тук',
  'err.CARD_NOT_IN_HAND': 'Тази карта не е в ръката ти',
  'err.MELD_BELOW_MINIMUM': 'На комбинациите ти още им липсват точки, за да свалиш',
  'err.MELD_NO_CONTRIBUTION': 'Тази комбинация не придвижва изискването ти',
  'err.TOO_MANY_WILDS': 'Твърде много жокери в тази комбинация',
  'err.ADJACENT_WILDS': 'Два жокера не могат да стоят един до друг',
  'err.ACE_BRIDGE': 'Асо не може да свързва поп и двойка',

  // --- contract phrasing ---------------------------------------------------
  // Built from the structured contract the server sends (sets/runs counts),
  // never from a deal-number lookup table.
  // Whole phrases per count rather than a number plus a pluralised noun:
  // Czech inflects the noun by count (skupina / skupiny / skupin) in a way no
  // "add an s" helper survives, and the counts that actually occur are 1-3.
  // A count outside that falls back to contract.sets.n.
  'contract.sets.1': 'Една група',
  'contract.sets.2': 'Две групи',
  'contract.sets.3': 'Три групи',
  'contract.sets.n': 'Групи: {n}',
  'contract.runs.1': 'Една поредица',
  'contract.runs.2': 'Две поредици',
  'contract.runs.3': 'Три поредици',
  'contract.runs.n': 'Поредици: {n}',
  'contract.any': 'Всяка валидна комбинация',
  'contract.cleanRunOnly': 'Всякаква смес от групи и поредици — поне една поредица трябва да е без жокер',
  'contract.cleanRunSuffix': '{base} — една поредица трябва да е без жокер',

  // --- written rules ---------------------------------------------------------
  // Full-sentence keys a "see the rules" screen renders, one per module,
  // resolved server-side against the table's actual variation and options —
  // see internal/module/rules.go and each game's rules.go. Shared section
  // titles first, then one block per module in the order the four games were
  // added.
  'zolik.rules.section.goal': 'Цел',
  'zolik.rules.section.setup': 'Подготовка',
  'zolik.rules.section.turn': 'Твоят ход',
  'zolik.rules.section.melding': 'Сваляне',
  'zolik.rules.section.end': 'Как свършва мачът',
  'zolik.rules.goal':
    'Бъди първият, който изпразва ръката си, сваляйки валидни групи и поредици, като събереш възможно най-малко наказателни точки в картите, които още държиш, когато някой друг излезе.',
  'zolik.rules.deal': 'Всеки играч получава {n} карти.',
  'zolik.rules.meldShapes':
    'Групата е {set}+ карти с еднаква стойност; поредицата е {run}+ последователни карти от една боя.',
  'zolik.rules.turn.draw': 'На своя ход изтегли една карта — от тестето или от купчината за изхвърляне.',
  'zolik.rules.pickup.topOnly': 'Може да се взема само горната карта от купчината за изхвърляне.',
  'zolik.rules.pickup.anyFromPile':
    'Може да се вземе всяка карта от купчината за изхвърляне заедно с всичко над нея.',
  'zolik.rules.pickup.locked': 'От купчината за изхвърляне не може да се тегли преди рунд {n}.',
  'zolik.rules.pickup.open': 'Купчината за изхвърляне е отворена от първия рунд.',
  'zolik.rules.turn.discard': 'Завърши хода си, като изхвърлиш една карта.',
  'zolik.rules.jokers.restricted':
    'Жокер никога не може да се изхвърля, освен ако не е точно картата, която изпразва ръката ти.',
  'zolik.rules.lead.rotate':
    'Първият ход се измества с едно място при всяко раздаване, независимо кой е спечелил.',
  'zolik.rules.lead.winner': 'Който излезе, започва следващото раздаване.',
  'zolik.rules.meldFloor.on':
    'Първото ти сваляне трябва да е поне {n} естествени точки, за да се смяташ за свалил.',
  'zolik.rules.meldFloor.off': 'Няма минимална стойност в точки за първото ти сваляне.',
  'zolik.rules.cleanRun.on':
    'Поне една от поредиците ти трябва да е напълно без жокер, за да се броиш за свалил.',
  'zolik.rules.cleanRun.off':
    'Поредиците ти могат да ползват жокери свободно — нито една не трябва да е без тях.',
  'zolik.rules.contracts.rotating':
    'Мачът е дълъг {n} раздавания и всяко раздаване изисква своя комбинация от групи и поредици.',
  'zolik.rules.contracts.static':
    'Всяко раздаване изисква една и съща комбинация: {sets} групи и {runs} поредици.',
  'zolik.rules.end.afterDeals': 'Мачът свършва след {n} раздавания.',
  'zolik.rules.end.atScore': 'Раздава се, докато някой стигне {n} точки — тогава свършва.',

  'prsi.rules.section.goal': 'Цел',
  'prsi.rules.section.setup': 'Подготовка',
  'prsi.rules.section.turn': 'Твоят ход',
  'prsi.rules.section.special': 'Специални карти',
  'prsi.rules.section.end': 'Как свършва мачът',
  'prsi.rules.goal': 'Бъди първият, който изиграе всяка карта от ръката си.',
  'prsi.rules.deck': 'Играе се с тесте от {value} карти (от 7 нагоре).',
  'prsi.rules.deal': 'Всеки играч започва с {n} карти.',
  'prsi.rules.turn.match':
    'Изиграй карта, която съвпада по боя или стойност с горната — или тегли, ако не можеш.',
  'prsi.rules.turn.draw': 'Тегленето приключва хода ти без изиграване.',
  'prsi.rules.sevens': 'Изиграй 7 и следващият играч тегли две карти, освен ако не отговори със своя 7.',
  'prsi.rules.aces': 'Изиграй асо и ходът на следващия играч се пропуска.',
  'prsi.rules.queens': 'Изиграй дама и назови боята, която продължава.',
  'prsi.rules.end': 'Мачът свършва в мига, в който нечия ръка е празна.',

  'canasta.rules.section.goal': 'Цел',
  'canasta.rules.section.setup': 'Подготовка',
  'canasta.rules.section.melding': 'Сваляне',
  'canasta.rules.section.end': 'Как свършва мачът',
  'canasta.rules.goal': 'Играе се по двойки; първата страна, стигнала {n} точки, печели мача.',
  'canasta.rules.deck': 'Играе се с {value} карти — две тестета плюс жокери.',
  'canasta.rules.deal': 'Всеки играч получава {n} карти.',
  'canasta.rules.redThrees':
    'Червена тройка в ръката ти се показва веднага и носи бонус — освен ако страната ти никога не завърши канаста, тогава се брои срещу теб.',
  'canasta.rules.canasta': 'Канаста е комбинация от {n} или повече карти с еднаква стойност.',
  'canasta.rules.meldFloorBands':
    'Първото ти сваляне трябва да достигне минимум точки, който расте с резултата ти: {negative} под нулата, {low} до 1500, {mid} до 3000, {high} над това.',
  'canasta.rules.oneCanastaToGoOut': 'Една завършена канаста стига, за да излезе страната ти.',
  'canasta.rules.twoCanastasToGoOut':
    'Страната ти се нуждае от две завършени канасти, преди да може да излезе.',
  'canasta.rules.end': 'Раздава се, докато една страна не мине {n} точки — тогава мачът свършва.',

  'holdem.rules.section.goal': 'Цел',
  'holdem.rules.section.setup': 'Подготовка',
  'holdem.rules.section.betting': 'Залагане',
  'holdem.rules.section.end': 'Как свършва мачът',
  'holdem.rules.goal':
    'Печели чипове с най-добрата ръка на разкриването или като останеш единственият в раздаването.',
  'holdem.rules.stack': 'Всяко място започва с {n} чипа.',
  'holdem.rules.blinds': 'Малкият блайнд е {sb}, а големият {bb}, залагат се преди раздаването на картите.',
  'holdem.rules.streets': 'Залага се в четири кръга — преди флопа и след флопа, търна и ривъра.',
  'holdem.rules.showdown':
    'Останалите в раздаването разкриват картите си; най-добрата ръка от пет карти взема пота.',
  'holdem.rules.noLimit': 'Без лимит — всеки залог може да е до целия ти стек.',
  'holdem.rules.lastPlayerStanding': 'Играе се, докато едно място не държи всички чипове.',
  'holdem.rules.mostChipsWins': 'Който има най-много чипове при спиране на играта, печели мача.',
  'holdem.rules.handLimit': 'Играта спира след {n} раздавания.',

  // --- header --------------------------------------------------------------
  'header.deal': 'Раздаване {n}',
  'header.gameOf': 'Игра {n} от {total}',
  'header.gameOfWithContract': 'Игра {n} от {total}: {contract}',

  // --- meld preview --------------------------------------------------------
  'preview.validSet': 'Валидна група',
  'preview.validRun': 'Валидна поредица',
  'preview.validMeld': 'Валидна комбинация',
  'preview.notYet': 'Още не е комбинация',
  'preview.points': '{shape} · {n} точки',
  'preview.pointsWithLaid': '{shape} · {n} + {laid} вече свалени = {total} точки',
  'preview.meetsFloor': '{line} (достига {n} ✓)',
  'preview.needsFloor': '{line} (нужни са {n} ✗)',
  'preview.becauseOf': '{line} — {reason}',

  // --- discard -------------------------------------------------------------
  // A discard with a meld staged lays that meld first; this is what the
  // player is told when the server refuses it and the whole move is rolled
  // back. {reason} is the engine's own words for the refusal.
  'discard.meldRejected': '{reason} — нищо не беше изхвърлено, картите ти още чакат.',

  // --- what the current selection would send -------------------------------
  // The client's own reason a control is not ready, next to the engine's own
  // `err.*` reasons rather than a second kind of message — a control greyed
  // out for "not your turn" and one greyed out for "you picked two, this
  // takes one" should read as the same kind of thing, not one of them looking
  // broken. See `fits` in `src/lib/drops.ts`.
  'sel.tooMany.1': 'Избери само една карта',
  'sel.tooMany.n': 'Избери най-много {n} карти',
  'sel.needMore': 'Избери карти: {n}',
  'sel.notThese': 'Тези карти не могат да отидат тук',
  'sel.needsCompany': 'Тази карта се нуждае от съседните',

  // --- what a module says happened -----------------------------------------
  // The first module-sent keys in this bundle, and deliberately few. Anything
  // a module sends is still legible without an entry here — `humanise` turns
  // `holdem.seat.stack` into "Stack" — so a key earns a line only when that
  // fallback loses something. These do: what each of them means lives in its
  // *params* (who won, how much, whose cards), and a key on its own has
  // nowhere to put them. Without the entry the player read "Winner", full
  // stop, at the end of a match they had just won.
  'status.winner': 'Спечелено от {winners}',
  'holdem.status.pot': '{winners} печели {amount} с {hand}',
  'holdem.status.potUncontested': '{winners} печели {amount} — всички други се отказаха',
  'holdem.status.shown': '{playerId} показа {value}',
  'holdem.prompt.waitingFor': 'Чака се {playerId}',

  // How a rummy scoreboard was ordered. Both earn a line for the same reason
  // as the keys above: the number lives in the params, and `humanise` would
  // render "Deals Won" with nothing after it.
  'zolik.standing.dealsWon': 'Спечелени раздавания: {n}',
  'zolik.standing.inHand': 'В ръката: {n}',

  // --- rounds ---------------------------------------------------------------
  // What a round is called, and what one did. Each of these carries its number
  // in the params, so `humanise` alone would render a label with nothing after
  // it.
  // An instruction, not a heading. "Next round" named the thing rather than
  // asking for it, and sat above "Waiting for 1" — which reads as though the
  // player is the one waiting, when the table is waiting for them.
  'round.continue': 'Започни следващия рунд',

  // --- the flash that announces a round or a match ending ------------------
  //
  // One line and one number, held for about two seconds. Worded generically on
  // purpose: which *kind* of ending it was is the module's own fact, printed
  // underneath, and a client that phrased "went gin" itself would be a client
  // that knows a game.
  'flash.roundWon': '{winners} го взе',
  'flash.roundWonYou': 'Ти го взе',
  'flash.roundDrawn': 'Никой не го взе',
  'flash.matchOver': 'Мачът приключи',
  'flash.matchWon': '{winners} печели',
  'flash.matchWonYou': 'Ти печелиш',
  'flash.matchDrawn': 'Никой не печели',
  'flash.nowOn': 'сега {total}',

  'zolik.round.deal': 'Раздаване',
  'zolik.round.cleanRun': 'Една поредица трябва да е без жокер',
  'canasta.round.deal': 'Раздаване',
  'canasta.round.concealed': 'Излезе скрито',
  'canasta.round.exhausted': 'Тестето свърши',
  'canasta.round.meldCards': 'Свалени карти: {n}',
  'canasta.round.canastas': 'Канасти: {n}',
  'canasta.round.redThrees': 'Червени тройки: {n}',
  'canasta.round.goingOut': 'Излизане: {n}',
  'canasta.round.inHand': 'Останали в ръката: {n}',
  'holdem.round.hand': 'Раздаване',
  'holdem.round.pot': 'Пот {n}',
  'holdem.round.uncontested': 'Всички други се отказаха',
  'seat.ready': 'Готов',
  'results.you': '(ти)',



  // --- refusals the other three games raise ---------------------------------
  // Worded here for the same reason Žolíky's are: reasonText falls back to the
  // code, and OfferBar hands it the code as that fallback, so an unworded
  // refusal reaches a player as PILE_FROZEN in capitals. serverKeys.test.ts
  // now fails the build rather than letting the next one through.
  'err.SET_TOO_LARGE': 'Групата вече има и четирите бои',
  'err.DISCARD_TAKEN_CARD_FORBIDDEN':
    'Не можеш да изхвърлиш картата, която току-що взе — изиграй я или я задръж',
  'err.CARD_DOES_NOT_FIT': 'Тази карта не съвпада нито по боя, нито по стойност',
  'err.SUIT_REQUIRED': 'Назови боята, която продължава',
  'err.MUST_ANSWER_DRAW_OR_TAKE': 'Отговори със седмица или вземи картите',
  'err.NOTHING_TO_DRAW': 'Не е останало нищо за теглене',
  'err.PILE_EMPTY': 'Купчината е празна',
  'err.PILE_BLOCKED': 'Купчината е блокирана — отгоре има черна тройка',
  'err.PILE_FROZEN': 'Купчината е замразена — нужни са ти две естествени карти със стойността на горната',
  'err.TOP_CARD_UNUSABLE': 'Не можеш да използваш горната карта',
  'err.MELD_CLOSED': 'Тази комбинация е пълна и затворена',
  'err.MELD_TOO_SMALL': 'Комбинацията се нуждае от повече карти',
  'err.MELD_TOO_LARGE': 'Тази комбинация не може да поеме повече карти',
  'err.MELD_MIXED_RANKS': 'Всяка карта в комбинация трябва да е с еднаква стойност',
  'err.NOT_ENOUGH_NATURALS': 'Комбинацията се нуждае от повече естествени карти, отколкото жокери',
  'err.RANK_ALREADY_MELDED': 'Страната ти вече има комбинация с тази стойност',
  'err.NOT_YOUR_MELD': 'Тази комбинация е на противниковата страна',
  'err.NO_SUCH_MELD': 'Тази комбинация не е на масата',
  'err.CANNOT_MELD_THREE': 'Тройки никога не се свалят',
  'err.CANNOT_DISCARD_RED_THREE': 'Червена тройка не може да се изхвърля',
  'err.MUST_KEEP_A_CARD': 'Задръж поне една карта — така не можеш да изпразниш ръката си',
  'err.MUST_MELD_FIRST': 'Първо свали отварянето на страната си',
  'err.INITIAL_MELD_NOT_MET': 'На първото ти сваляне още му липсват точки',
  'err.CANNOT_GO_OUT_YET': 'Страната ти се нуждае от завършена канаста, преди да може да излезе',
  'err.NOTHING_TO_CALL': 'Няма залог за плащане',
  'err.CANNOT_CHECK': 'Не можеш да чекнеш — има залог, на който да отговориш',
  'err.CANNOT_RAISE': 'Тук не можеш да вдигаш',
  'err.RAISE_TOO_SMALL': 'Вдигането трябва да е поне колкото предишното',
  'err.NOT_ENOUGH_CHIPS': 'Нямаш толкова чипове',
  'err.AMOUNT_REQUIRED': 'Кажи колко',
  'err.AMOUNT_NOT_A_NUMBER': 'Тази сума не е число',
  'err.SEAT_NOT_IN_HAND': 'Не участваш в това раздаване',
  'err.WRONG_RANK': 'Тази карта е с грешна стойност за това',
  'err.MATCH_FULL': 'Масата е пълна',
  'err.MATCH_ALREADY_STARTED': 'Мачът вече е започнал',
  'err.TOO_FEW_PLAYERS': 'Още няма достатъчно играчи',
  'err.WRONG_PLAYER_COUNT': 'Тази игра не може да се играе с толкова играчи',
  'err.NOT_THE_HOST': 'Това може само домакинът',
  'err.NO_LONGER_WAITING': 'Масата вече не чака',
  'err.WAITING_ROOM_UNAVAILABLE': 'Чакалнята не е достъпна',
  'err.SERVER_BUSY': 'Сървърът е пълен в момента — опитай пак след малко',


  // --- rules the engine has always enforced and never stated ----------------
  // Every one of these was found by the guardrail rather than by review: the
  // validator could refuse a player for it, and no sentence anywhere said so.
  // See server/internal/zolikmod/ruleindex_test.go.
  'zolik.rules.section.layoff': 'Добавяне към комбинации',
  'zolik.rules.pickup.obligation':
    'Докато не си свалил, карта, взета от купчината за изхвърляне, трябва да се използва в комбинацията, с която сваляш този ход.',
  'zolik.rules.pickup.noReturn':
    'Карта, взета от купчината за изхвърляне, не може да се изхвърли отново в същия ход — изиграй я или я задръж.',
  'zolik.rules.wilds.setLimit': 'Групата не може да съдържа повече жокери, отколкото естествени карти.',
  'zolik.rules.set.maxSize':
    'Групата не може да съдържа повече от {n} карти — жокерът замества липсваща боя, не допълва пълна група.',
  'zolik.rules.run.maxLength':
    'Поредицата не може да съдържа повече от {n} карти — асото долу, дванадесетте стойности над него и асото горе.',
  'zolik.rules.run.aceBridge':
    'Асото стои над попа или под двойката, никога като мост между двата края на поредица.',
  'zolik.rules.contracts.contribution':
    'Докато не си свалил, всяка сваляна комбинация трябва да е такава, каквато договорът на раздаването още изисква.',
  'zolik.rules.layoff.afterDown':
    'Не можеш да добавяш към чужди комбинации, докато не свалиш собствения си договор.',
  'zolik.rules.layoff.runEnds':
    'Карта, добавена към поредица, трябва да я продължи в единия или другия край.',
  'zolik.rules.jokers.swap':
    'Жокер в комбинация на масата може да бъде откупен с точно картата, която замества.',
  'zolik.rules.jokers.reclaim.on':
    'Жокер, откупен от масата, трябва да се изиграе в комбинация в същия ход — не може да остане в ръката.',
  'zolik.rules.jokers.reclaim.off': 'Жокер, откупен от масата, може да остане в ръката.',
  'zolik.rules.deck.reshuffle':
    'Когато тестето свърши, купчината за изхвърляне се разбърква и става новото тесте; ако и двете са празни, раздаването приключва.',


  // --- what to do instead ---------------------------------------------------
  // The third layer of a refusal, after the reason and the rule. Sent by the
  // module because it is the only side that knows which card is owed and
  // which way out is on offer; see server/internal/zolikmod/remedy.go.
  'zolik.remedy.meldThePickup': 'Добави {card} към свалянето си или върни вземането.',
  'zolik.remedy.discardSomethingElse': 'Изхвърли друга карта или изиграй {card} този ход.',
  'zolik.remedy.discardNotAJoker': 'Изхвърли нещо друго, не жокер.',
  'zolik.remedy.finishOrUndoLayDown': 'Довърши свалянето си или го вземи обратно.',
  'zolik.remedy.needMorePoints': 'Нужни са ти още {n} точки, за да можеш да свалиш.',
  'zolik.remedy.layACleanRun': 'Свали поредица без жокер в нея.',
  'zolik.remedy.playReclaimedJoker': 'Изиграй {card} в комбинация или върни вземането.',
  'zolik.remedy.goDownFirst': 'Първо свали собствените си комбинации.',
  'zolik.remedy.drawFirst': 'Първо изтегли карта.',
  'zolik.remedy.drawFromStock': 'Тегли от тестето — купчината за изхвърляне се отваря в рунд {n}.',
  'zolik.remedy.drawFromStockEmpty': 'Тегли от тестето вместо това.',


  // --- keys whose meaning lives in their params -----------------------------
  // The shape fallback renders these with the number missing ("Stack", "Pot"),
  // which is why they earn a line where most keys do not.
  'header.contract': 'Изисква {sets} групи и {runs} поредици',
  'header.contract.cleanRunOnly': 'Изисква поредица без жокер',
  'header.round': 'Рунд {n}',
  'header.deck': 'Тесте',
  'header.target': 'Цел',
  'header.suitInPlay': 'Боя в игра',
  'seat.cards': 'Карти',
  'zolik.offer.meld': 'Свали',
  'prompt.pickupMustBeMelded':
    '{value} дойде от купчината за изхвърляне — трябва да влезе в комбинациите, с които сваляш този ход.',
  'prompt.jokerMustBePlayed':
    '{value} дойде от масата — трябва да влезе в комбинация, преди да можеш да завършиш хода си.',
  'prompt.initialMeld': 'Отварянето на страната ти трябва да достигне {n} точки.',
  'prompt.canastasNeeded': 'На страната ти липсват още {n} канасти, преди да може да излезе.',
  'prompt.mustDrawOrAnswerSeven': 'Отговори със седмица или изтегли {n} карти.',
  'prompt.chooseSuit': 'Избери боята, която продължава',
  'prompt.skipPending': 'Ходът ти се пропуска',
  'status.lastDeal': 'Отбор {team} направи {value}',
  'status.teamScore': 'Отбор {team}: {value}',
  'canasta.offer.rank': 'Стойност',
  'canasta.seat.teamScore': 'Точки на отбора',
  'canasta.seat.canastas': 'Канасти',
  'holdem.header.pot': 'Пот',
  'holdem.header.street': 'Улица',
  'holdem.header.hand': 'Раздаване',
  'holdem.header.handLimit': 'Раздавания общо',
  'holdem.header.blinds': 'Блайндове',
  'holdem.cost.call': 'за плащане',
  'holdem.cost.pot': 'в пота',
  'holdem.seat.stack': 'Стек',
  'holdem.seat.bet': 'Залог',
  'holdem.prompt.yourAction': 'Твой ред е',
  'holdem.prompt.raiseTo': 'Вдигни до',
  'holdem.quick.halfPot': '½ Пот',
  'holdem.quick.pot': 'Пот',
  'holdem.quick.allIn': 'All-in',
  'zone.yourHand': 'Твоята ръка',
  'zone.opponentHand': 'Ръката на противника',
  'zone.drawPile': 'Тесте',
  'zone.discardPile': 'Изхвърлени',
  'zone.melds': 'Комбинации',
  'zone.teamMelds': 'Комбинации на твоята страна',
  'zone.redThrees': 'Червени тройки',
  'zone.board': 'Маса',
  'verb.drawFromDeck': 'Тегли',
  'verb.takeFromDiscard': 'Вземи от купчината',


  // --- the why sheet's own furniture ---------------------------------------
  // The three layers a refusal is explained in. Labels, not sentences: what
  // goes under each is sent by the server or worded above.
  'why.reason': 'Защо не',
  'why.rule': 'Правилото',
  'why.rules': 'Правилата',
  'why.remedy': 'Какво можеш да направиш',
  'why.readTheRules': 'Прочети пълните правила →',
  'why.close': 'Затвори',
  'why.open': 'защо',

  // --- marks on a particular card ------------------------------------------
  // A mark is a refusal that has not happened yet: the rule is enforced at the
  // discard, which is the last possible moment to hear about it, so the card
  // says so while there is still a turn left to act on it.
  'zolik.badge.owedToMeld':
    '{card} дойде от купчината за изхвърляне — трябва да влезе в комбинациите, с които сваляш този ход.',
  'zolik.badge.jokerOwed':
    '{card} дойде от масата — трябва да влезе в комбинация, преди да можеш да завършиш хода си.',

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
  'legal.terms': 'Условия',
  // The document's own name, used as the heading above it. Kept apart from
  // `legal.notice.terms`, which is the same words inflected to sit inside a
  // sentence — a heading in the instrumental case reads as a mistake in every
  // Slavic language.
  'legal.terms.title': 'Условия за ползване',
  'legal.privacy.title': 'Уведомление за поверителност',
  'legal.privacy': 'Поверителност',
  'legal.source': 'Изходен код',
  'legal.updated': 'Версия {version}',
  'legal.draft':
    'Проект — още не е в сила. Името, държавата и адресът за връзка на оператора предстои да бъдат попълнени.',
  'legal.notice.before': 'С играта приемаш ',
  'legal.notice.terms': 'условията за ползване',
  'legal.notice.between': '. Какво се съхранява за теб, е описано в ',
  'legal.notice.privacy': 'уведомлението за поверителност',
  'legal.notice.after': '.',
  // --- gin rummy -------------------------------------------------------------
  'err.UPCARD_DECLINED': 'Вече отказа тази карта',
  'err.DEADWOOD_TOO_HIGH': 'Твоят дедуд е твърде висок, за да чукаш',
  'err.CARD_DOES_NOT_EXTEND_MELD': 'Тази карта не удължава тази комбинация',
  'ginrummy.rules.setup': 'Подготовка',
  'ginrummy.rules.turn': 'Твоят ход',
  'ginrummy.rules.melds': 'Комбинации',
  'ginrummy.rules.knocking': 'Чукане',
  'ginrummy.rules.bigGin': 'Голям джин',
  'ginrummy.rules.layoff': 'Прикачването',
  'ginrummy.rules.deadHand': 'Мъртвото раздаване',
  'ginrummy.rules.scoring': 'Точкуване на раздаване',
  'ginrummy.rules.match': 'Спечелване на мача',
  'ginrummy.rules.lineBonuses': 'Бонуси в равносметката',
  'ginrummy.rules.deck': 'Играе се с тесте от {value} карти.',
  'ginrummy.rules.deal': 'Всеки играч получава {value} карти.',
  'ginrummy.rules.upcard': 'Още една карта се обръща с лице нагоре, за да започне купчината за изхвърляне.',
  'ginrummy.rules.drawDiscard':
    'На своя ход изтегли една карта — от тестето или от купчината за изхвърляне — и после изхвърли една.',
  'ginrummy.rules.setsAndRuns':
    'Комбинацията е група от три или четири карти с една стойност, или поредица от три и повече карти в една боя.',
  'ginrummy.rules.aceLow': 'Асото винаги е ниско — няма поредица от дама до асо.',
  'ginrummy.rules.knockLimit': 'Можеш да чукнеш, щом дедудът ти е {n} или по-малко.',
  'ginrummy.rules.oklahoma':
    'Границата за чукане в това раздаване се определя от стойността на обърнатата карта.',
  'ginrummy.rules.gin': 'Нулев дедуд е джин — най-доброто възможно чукане.',
  'ginrummy.rules.bigGinBonus':
    'Единайсет карти изцяло в комбинации, без никакво изхвърляне, е голям джин и носи още {n} точки.',
  'ginrummy.rules.layoffDescription':
    'След чукане, което не е джин, противникът ти може да прикачи собствения си дедуд към твоите комбинации, преди ръцете да бъдат сравнени.',
  'ginrummy.rules.deadHandDescription':
    'Ако тестето спадне до последните си две карти и никой не е чукнал, раздаването е мъртво — никой не точкува и същият раздаващ раздава отново.',
  'ginrummy.rules.undercut':
    'Ако дедудът на противника ти не е по-висок от твоя, той те подрязва: печели разликата плюс {n}.',
  'ginrummy.rules.ginBonus': 'Джинът носи цялата ръка на противника ти плюс {n}.',
  'ginrummy.rules.target': 'Първият, който мине {n} точки след края на раздаване, печели мача.',
  'ginrummy.rules.shutout': 'Бонусът за мача се удвоява до {n}, ако губещият не е спечелил нито една точка.',
  'ginrummy.rules.box': 'Всяко спечелено раздаване струва {n} точки в края на мача.',
  'ginrummy.rules.gameBonus': 'Спечелването на мача носи още {n} точки.',
  'ginrummy.fact.deadwood': 'дедуд {value}',
  'ginrummy.fact.discardCard': 'Изхвърли {value}',
  'ginrummy.fact.meldCards': 'Към {value}',
  'ginrummy.header.hand': 'Раздаване {n}',
  'ginrummy.round.kind': '{value}',
  'ginrummy.round.hand': 'Раздаване',
  'ginrummy.seat.score': '{value}',
  'ginrummy.seat.dealer': 'Раздаващ',
  'ginrummy.status.knocked': '{playerId} чукна с дедуд {deadwood}',
  'ginrummy.status.gin': '{playerId} направи джин',
  'ginrummy.status.lastHand': 'Последно раздаване: {winner} ({kind}, {delta} точки)',
  'ginrummy.offer.drawStock': 'Тегли от тестето',
  'ginrummy.offer.drawDiscard': 'Тегли от купчината за изхвърляне',
  'ginrummy.offer.takeUpcard': 'Вземи обърнатата карта',
  'ginrummy.offer.passUpcard': 'Пас',
  'ginrummy.offer.discard': 'Изхвърли',
  'ginrummy.offer.knock': 'Чукни',
  'ginrummy.offer.gin': 'Джин!',
  'ginrummy.offer.bigGin': 'Голям джин!',
  'ginrummy.offer.layOff': 'Прикачи',
  'ginrummy.offer.finishLayoff': 'Готово с прикачването',
  'ginrummy.zone.knockerHand': 'Ръката на чукналия',
  'ginrummy.zone.melds': 'Комбинации',
  'ginrummy.prompt.upcardDecision': 'Вземи обърнатата карта или пасувай',
  'ginrummy.prompt.yourTurnDraw': 'Изтегли карта',
  'ginrummy.prompt.yourTurnDiscard': 'Изхвърли — или чукни, ако можеш',
  'ginrummy.prompt.layoff': 'Прикачи дедуд или приключи',

  // --- rummy tiles -------------------------------------------------------------
  'err.TILE_NOT_IN_HAND': 'Това плочка не е в ръката ти',
  'err.TILE_DOES_NOT_FIT': 'Това не пасва там',
  'err.NO_SUCH_SET': 'Тази комбинация не е на масата',
  'err.INITIAL_MELD_ONLY':
    'Преди първото си сваляне можеш да пренареждаш само собствените си нови комбинации',
  'err.TABLE_NOT_VALID': 'Масата още не е валидна',
  'err.TRAY_NOT_EMPTY': 'Още имаш свободни плочки за поставяне',
  'err.NOTHING_PLAYED': 'Изиграй поне една плочка, преди да завършиш хода си',
  'err.INITIAL_MELD_TOO_LOW': 'Първото ти сваляне трябва да струва 30 точки или повече',
  'err.NOT_A_RUN': 'Само поредица може да се разделя',
  'err.BAD_SPLIT_POSITION': 'На това място тази поредица не може да се раздели',
  'err.NO_JOKER_IN_SET': 'В тази комбинация няма жокер',
  'err.TILE_JOKER_SWAP_MISMATCH': 'Тази плочка не е това, което жокерът замества',
  'rummytiles.rules.setup': 'Подготовка',
  'rummytiles.rules.sets': 'Комбинации',
  'rummytiles.rules.initialMeld': 'Първото сваляне',
  'rummytiles.rules.turn': 'Твоят ход',
  'rummytiles.rules.jokerTaking': 'Вземане на жокер',
  'rummytiles.rules.ending': 'Край на рунд',
  'rummytiles.rules.poolExhaustion': 'Ако запасът свърши',
  'rummytiles.rules.match': 'Спечелване на мача',
  'rummytiles.rules.tiles': 'Играе се с {value} плочки.',
  'rummytiles.rules.dealCount': 'Всеки играч получава {value} плочки.',
  'rummytiles.rules.group': 'Групата е три или четири плочки с едно и също число, всяка в различен цвят.',
  'rummytiles.rules.run': 'Поредицата е три или повече последователни числа в един цвят.',
  'rummytiles.rules.noWrap': '13 не се връща обратно към 1.',
  'rummytiles.rules.joker': 'Жокерът замества всяка плочка.',
  'rummytiles.rules.initialMeldDescription':
    'Докато не свалиш {n} или повече точки в един-единствен ход, само от собствената си ръка, не можеш да пипаш нищо, което вече е на масата.',
  'rummytiles.rules.turnDescription':
    'Изиграй поне една плочка от ръката си, преподреждай масата свободно и завърши с всяка комбинация на масата валидна.',
  'rummytiles.rules.noDiscard':
    'Няма изхвърляне — ако не можеш да завършиш валиден ход, теглиш една плочка вместо това.',
  'rummytiles.rules.jokerTakingDescription':
    'Жокер на масата може да се вземе, като го замениш с плочката, която замества, от твоята ръка — и трябва да се използва в комбинация, преди ходът ти да свърши.',
  'rummytiles.rules.goingOut':
    'Първият играч без плочки печели рунда. Всички останали получават отрицателната стойност на това, което им е останало; победителят получава сбора от загубите на всички останали.',
  'rummytiles.rules.poolExhaustionLowestWins':
    'Ако запасът свърши и никой не може да играе, рундът приключва и го печели най-ниската стойност на ръка.',
  'rummytiles.rules.poolExhaustionNoWinner':
    'Ако запасът свърши и никой не може да играе, рундът приключва без победител — всяка ръка просто се точкува.',
  'rummytiles.rules.target': 'Първият, който мине {n} точки след края на рунд, печели мача.',
  'rummytiles.rules.roundLimit': 'Мачът свършва след {n} рунда — печели най-високият резултат.',
  'rummytiles.fact.setCards': '{value}',
  'rummytiles.header.pool': 'Запас {n}',
  'rummytiles.header.round': 'Рунд {n}',
  'rummytiles.round.kind': '{value}',
  'rummytiles.round.round': 'Рунд',
  'rummytiles.seat.score': '{value}',
  'rummytiles.seat.notOpened': 'Не е отворил',
  'rummytiles.status.lastRound': 'Последен рунд: {winner} ({kind})',
  'rummytiles.badge.invalid': 'Още не е валидно',
  'rummytiles.zone.pool': 'Запас',
  'rummytiles.zone.table': 'Маса',
  'rummytiles.zone.tray': 'Поставка',
  'rummytiles.offer.place': 'Постави',
  'rummytiles.offer.addFromHand': 'Добави',
  'rummytiles.offer.addFromTray': 'Добави от поставката',
  'rummytiles.offer.take': 'Вземи',
  'rummytiles.offer.split': 'Раздели',
  'rummytiles.offer.swapJoker': 'Смени жокера',
  'rummytiles.offer.resetTurn': 'Върни хода',
  'rummytiles.offer.commit': 'Готово',
  'rummytiles.offer.draw': 'Тегли',
  'rummytiles.param.position': 'Раздели при',

  // --- blackjack -----------------------------------------------------------
  //
  // The house rules are worded twice over — on and off — because a rule that
  // is *not* in force is still something a player has to be told. "No
  // surrender at this table" is information; silence is a guess.
  'err.BET_BELOW_MINIMUM': 'Това е под минимума на масата',
  'err.ALREADY_BET': 'Залогът ти вече е направен',
  'err.INSURANCE_CLOSED': 'В момента няма застраховка за вземане',
  'err.CANNOT_DOUBLE': 'Тази ръка не може да се удвоява',
  'err.CANNOT_SPLIT': 'Тази ръка не може да се разделя',
  'err.CANNOT_SURRENDER': 'Тази ръка не може да се предава',

  'blackjack.rules.section.table': 'Масата',
  'blackjack.rules.section.play': 'Изиграване на ръка',
  'blackjack.rules.section.dealer': 'Крупието',
  'blackjack.rules.section.end': 'Как свършва мачът',
  'blackjack.rules.goal':
    'Победи крупието, без да минеш двайсет и едно. Минаването губи веднага, каквото и да направи крупието после.',
  // Counts are their own phrase rather than a number glued to a noun: one
  // deck and six decks inflect differently, and Czech inflects them again.
  'blackjack.rules.decks': 'Тестета в шуто: {n}.',
  'blackjack.rules.stack': 'Всяко място сяда с {n} чипа.',
  'blackjack.rules.minBet': 'Минимумът на масата е {n} чипа.',
  'blackjack.rules.faceUp':
    'Картите на играчите се раздават с лице нагоре; крупието държи една карта покрита, докато всички изиграят.',
  'blackjack.rules.hitStand': 'Тегли колкото карти искаш или остани с това, което имаш.',
  'blackjack.rules.aces':
    'Асото се брои за единайсет, докато това се побира, и за едно, когато не се побира.',
  'blackjack.rules.blackjack': 'Асо с карта със стойност десет, на първите две карти, е блекджек.',
  'blackjack.rules.pays3to2': 'Блекджекът плаща 3:2.',
  'blackjack.rules.pays6to5': 'Блекджекът плаща 6:5.',
  'blackjack.rules.paysEven': 'Блекджекът плаща едно към едно.',
  'blackjack.rules.double':
    'На първите си две карти можеш да удвоиш залога и да вземеш точно още една карта.',
  'blackjack.rules.doubleAfterSplit': 'Ръка, получена от разделяне, също може да се удвои.',
  'blackjack.rules.noDoubleAfterSplit': 'Ръка, получена от разделяне, не може да се удвоява.',
  'blackjack.rules.split':
    'Две карти с еднаква стойност може да се разделят на самостоятелни ръце, всяка със свой залог — до {n} пъти, за общо {hands} ръце.',
  'blackjack.rules.noSplit': 'На тази маса двойки не се разделят.',
  'blackjack.rules.splitAces':
    'Разделените аса получават по една карта и след това остават, а двайсет и едно, направено така, не е блекджек.',
  'blackjack.rules.surrender':
    'Можеш да се откажеш от първата си ръка срещу половината от залога, след като крупието е проверило за блекджек.',
  'blackjack.rules.noSurrender': 'На тази маса не може да се предават ръце.',
  'blackjack.rules.dealerDraws': 'Крупието тегли до седемнайсет и след това остава.',
  'blackjack.rules.hitsSoft17': 'Крупието тегли и при седемнайсет, направени с асо.',
  'blackjack.rules.standsSoft17': 'Крупието остава при седемнайсет, направени с асо.',
  'blackjack.rules.dealerPeeks':
    'С асо или десетка отгоре крупието проверява за блекджек, преди някой да е изиграл.',
  'blackjack.rules.insurance':
    'Срещу асо на крупието можеш да се застраховаш за половината от залога си; плаща 2:1, ако крупието има блекджек.',
  'blackjack.rules.noInsurance': 'На тази маса не се предлага застраховка.',
  'blackjack.rules.rounds': 'На масата се играят {n} рунда.',
  'blackjack.rules.mostChipsWins': 'Който има най-много чипове накрая, печели мача.',
  'blackjack.rules.bustedOut':
    'Място, което вече не може да покрие минимума от {n}, стои настрана до края на мача.',

  'blackjack.zone.dealer': 'Крупие',
  'blackjack.zone.box': 'Ръка',
  'blackjack.zone.yourBox': 'Твоята ръка',
  'blackjack.zone.shoe': 'Шу',

  'blackjack.header.round': 'Рунд {n} от {of}',
  'blackjack.header.minBet': 'Минимум',
  'blackjack.header.decks': 'Тестета',
  'blackjack.header.dealerTotal': 'Крупието показва {n}',
  'blackjack.header.dealerSoftTotal': 'Крупието показва меки {n}',

  'blackjack.seat.stack': 'Чипове',
  'blackjack.seat.bet': 'Залог',
  'blackjack.seat.insurance': 'Застраховка',
  'blackjack.seat.total': 'Общо',
  'blackjack.seat.softTotal': 'Мека сума',
  'blackjack.seat.out': 'Без чипове',

  'blackjack.prompt.placeBet': 'Направи залога си',
  'blackjack.prompt.insurance': 'Застраховка?',
  'blackjack.prompt.yourMove': 'Твой ред е',
  'blackjack.prompt.waitingFor': 'Чака се {playerId}',
  'blackjack.prompt.betAmount': 'Залог',

  'blackjack.quick.doubleMin': '2× Минимум',
  'blackjack.quick.allIn': 'All-in',
  'blackjack.offer.bet': 'Заложи',
  'blackjack.offer.hit': 'Карта',
  'blackjack.offer.stand': 'Оставам',
  'blackjack.offer.double': 'Удвои',
  'blackjack.offer.split': 'Раздели',
  'blackjack.offer.surrender': 'Откажи се',
  'blackjack.offer.insure': 'Застраховай се',
  'blackjack.offer.declineInsurance': 'Без застраховка',

  'blackjack.fact.tableMinimum': 'минимум',
  'blackjack.fact.insuranceCost': 'за застраховка',
  'blackjack.fact.extraStake': 'за залагане',
  'blackjack.fact.surrenderReturn': 'обратно',

  'blackjack.status.dealerBlackjack': 'Крупието имаше блекджек',
  'blackjack.status.dealerBust': 'Крупието изгоря с {n}',
  'blackjack.status.dealerStands': 'Крупието остава на {n}',

  'blackjack.round.name': 'Рунд',
  'blackjack.round.dealerTotal': 'Крупие {n}',
  'blackjack.round.dealerBust': 'Крупието изгоря ({n})',
  'blackjack.round.dealerBlackjack': 'Блекджек на крупието',
  'blackjack.round.outcome.blackjack': 'Блекджек',
  'blackjack.round.outcome.win': 'Спечелена',
  'blackjack.round.outcome.push': 'Равенство',
  'blackjack.round.outcome.lose': 'Загубена',
  'blackjack.round.outcome.bust': 'Изгоряла',
  'blackjack.round.outcome.surrender': 'Предадена',

  'blackjack.badge.inPlay': 'В игра',
  'blackjack.badge.doubled': 'Удвоена',
  'blackjack.badge.split': 'Разделена',
  'blackjack.badge.blackjack': 'Блекджек',
  'blackjack.badge.bust': 'Изгоряла',
  'blackjack.badge.won': 'Спечелена',
  'blackjack.badge.push': 'Равенство',
  'blackjack.badge.lost': 'Загубена',
  'blackjack.badge.surrendered': 'Предадена',

  'blackjack.unit.chips': 'чипа',

  // --- the settings screen --------------------------------------------------
  //
  // Hardcoded English until now, which was survivable while the only other
  // language was unreachable and faintly absurd once the language picker
  // itself lived on this screen: a player who cannot read "Settings" is
  // exactly the one who came here to change it.
  'settings.title': 'Настройки',
  'settings.signedInAs': 'Влязъл си като {username}',
  'settings.playingAsGuest': 'Играеш като {username} (гост)',
  'settings.notSignedIn': 'Не си влязъл — влез или продължи като гост, за да играеш онлайн.',
  'settings.subtitle': 'Как изглеждаш ти и как изглежда масата',
  'settings.face.heading': 'Твоето лице на масата',
  'settings.face.account': 'Пази се в акаунта ти, така че те следва и на друго устройство.',
  'settings.face.device': 'Пази се на това устройство. Влез в профила си, за да го носиш със себе си.',
  'settings.skin.heading': 'Вид на масата',
  'settings.language.heading': 'Език',
  'settings.language.status': 'Пази се на това устройство.',
  'settings.language.auto': 'Автоматично',
  'settings.language.auto.now': 'Следва устройството ти — сега {language}',
  'settings.legal.heading': 'Дребният шрифт',
  'settings.legal.status': 'С какво се съгласи, като играеш, и какво се съхранява за теб.',
  'settings.signIn': 'Влез',
  'settings.back': 'Назад',

  // A translated interface over an English notice is a worse lie than an
  // English interface, so the document says so itself rather than letting the
  // surrounding screen imply the text was written for this reader.
  'legal.untranslated':
    'Това уведомление още не е преведено на твоя език. Валидна е английската версия по-долу.',

  // --- the navigation bar's screen titles ------------------------------------
  //
  // These sit in `app/_layout.tsx`, above every screen, and were the last
  // English left after the settings screen was translated — a German player
  // reading "Einstellungen" under a bar that said "Settings". A translated app
  // with an untranslated chrome bar looks like a bug, not like a limit.
  'nav.home': 'Zolik',
  'nav.emailSignIn': 'Вход с имейл',
  'nav.signingIn': 'Влизане',
  'nav.usernameSignIn': 'Вход с потребителско име',
  'nav.legacyAccount': 'Стар акаунт',
  'nav.guest': 'Гост',
  'nav.account': 'Акаунт',
  'nav.games': 'Игри',
  'nav.table': 'Твоята маса',
  'nav.join': 'Присъедини се към маса',
  // The screen a shared invite link lands on while it resolves the code.
  'nav.joining': 'Присъединяване',
  'nav.rules': 'Правила',
  'nav.match': 'Мач',
  'nav.scoreTable': 'Таблица с точки',
  'nav.stats': 'Статистика',
  'nav.more': 'Още',

  // --- the account menu behind the face in the corner ----------------------
  'menu.label': 'Меню на профила',
  'menu.signedIn': 'Влязъл',
  'menu.notSignedIn': 'Не си влязъл',
  'menu.keepStats': 'за да пазиш статистиките си',
  'menu.signOut': 'Изход',
  'more.scoreTable': 'Офлайн таблица с точки',
  'more.stats': 'Статистики и класация',
  'more.needsAccount': 'влез, за да ползваш',
  'gate.title': 'Влез, за да ползваш това',
  'gate.body':
    'Таблиците с точки и статистиките се пазят с профила ти, за да те следват и на друго устройство. Гостът няма къде да ги пази.',

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
  'error.generic': 'Това не се получи',
  'error.signIn': 'Влизането не бе успешно',
  'error.login': 'Влизането не бе успешно',
  'error.register': 'Регистрацията не бе успешна',
  'error.sendCode': 'Кодът не можа да бъде изпратен',
  'error.badCode': 'Този код не сработи',
  'error.rulesLoad': 'Правилата не можаха да се заредят',
  'error.createFailed': 'Създаването не бе успешно',
  'error.saveFailed': 'Запазването не бе успешно',
  'error.exportFailed': 'Експортът не бе успешен',

  // --- the route that does not exist ----------------------------------------
  'notFound.title': 'Опа!',
  'notFound.message': 'Този екран не съществува.',
  'notFound.home': 'Към началния екран!',

  // --- signing in -----------------------------------------------------------
  'auth.login.subtitle': 'Запази статистиката си на всички устройства',
  'auth.login.continueWithEmail': 'Продължи с имейл',
  'auth.login.usernameInstead': 'Влез вместо това с потребителско име',
  'auth.email.title': 'Влизане с имейл',
  'auth.email.subtitle': 'Ще ти изпратим еднократен код',
  'auth.email.address': 'Имейл адрес',
  'auth.email.send': 'Изпрати код',
  'auth.email.codeTitle': 'Въведи кода',
  'auth.email.codePlaceholder': 'Шестцифрен код',
  'auth.email.differentAddress': 'Използвай друг адрес',
  'auth.email.sentTo': 'Изпратено до {email}',
  'auth.email.continue': 'Продължи',
  'auth.guest.title': 'Игра като гост',
  'auth.guest.subtitle': 'Не е нужен акаунт',
  'auth.guest.displayName': 'Показвано име',
  'auth.register.title': 'Създай акаунт',
  'auth.register.username': 'Потребителско име',
  'auth.register.email': 'Имейл (по желание)',
  'auth.register.password': 'Парола',
  'auth.username.createAccount': 'Създай акаунт с потребителско име и парола',
  'auth.callback.signedIn': 'Влязохте.',

  // --- the account screen ---------------------------------------------------
  'account.signInPrompt': 'Влез, за да управляваш акаунта си.',
  'account.keepGames': 'Запази тези игри',
  'account.signedInWith': 'Влязохте чрез',
  'account.addMethod': 'Добави начин за влизане',
  'account.usernameAndPassword': 'Потребителско име и парола',
  'account.faceAndTable': 'Лице и вид на масата',
  'account.refresh': 'Опресни',
  'account.remove': 'Премахни',

  // --- the main menu --------------------------------------------------------
  'home.subtitle': 'Континентален реми · {server}',
  'home.playingAs': 'Играеш като {name}',
  'home.signInPrompt': 'Влез или продължи като гост, за да играеш онлайн.',
  'home.statsAndLeaderboard': 'Статистика и класация',
  'home.play': 'Играй',
  'home.offlineScoreTable': 'Таблица с точки офлайн',
  'home.signInToKeepStats': 'Влез, за да запазиш статистиката си',
  'home.signOut': 'Излез',
  'home.continueAsGuest': 'Продължи като гост',
  // Someone playing as a guest, marked in a list of names.
  'home.guestSuffix': '(гост)',

  // --- the waiting room on the main menu ------------------------------------
  //
  // Counts are whole phrases per count, not a number glued to a noun — the
  // same reason `countLabel` exists. One and many are the cases the interface
  // actually produces, so they are the cases that get their own wording.
  'waiting.checking': 'Гледаме кой е наоколо…',
  'waiting.youAreWaiting': 'Чакаш да играеш',
  'waiting.pickedUp': 'Всеки, който отвори маса, може да те вземе — никой няма нужда от код от теб.',
  'waiting.othersOne': 'Чака и още 1 играч',
  'waiting.othersMany': 'Чакат и още {n} играчи',
  'waiting.oneWaiting': '1 играч чака да играе',
  'waiting.manyWaiting': '{n} играчи чакат да играят',
  'waiting.adding': 'Добавяме те към списъка на чакащите…',
  'waiting.slowHint':
    'Ако това не приключи за няколко секунди, провери дали адресът на сървъра по-долу е достъпен от това устройство.',
  'waiting.serverBusyDetail': 'Опит {n}. В момента сървърът не приема нови връзки към чакалнята.',
  'waiting.reconnecting': 'Връзката прекъсна — свързваме се отново…',
  'waiting.reconnectingDetail':
    'Опит {n}. Това може да стане, ако мрежата на устройството ти се е сменила или сървърът е рестартирал.',
  'waiting.tryAgain': 'Опитай пак сега',
  'waiting.makeAvailable': 'Отбележи ме като готов за игра',
  'waiting.stop': 'Спри да чакаш',
  'waiting.noneYet':
    'В момента никой не чака да играе. Запиши се в списъка и ще си първият, когото някой вижда.',
  'waiting.noOthersYet': 'Още никой друг не чака. Домакините пак те виждат и могат да те поканят.',
  'waiting.server': 'Сървър',
  'waiting.none': 'В момента никой не чака. Който се отбележи като готов в главното меню, се появява тук.',

  // --- landing on a shared invite link --------------------------------------
  'join.missingCode': 'На тази връзка ѝ липсва кодът на масата.',
  'join.staleLink': 'Поискай нов линк от този, който те покани, или се присъедини с кода.',
  'join.enterCode': 'Въведи код',
  'join.backToMenu': 'Обратно към менюто',
  'join.takingSeat': 'Заемаме място…',
  'join.takingSeatAt': 'Заемаме място на {game}…',

  // --- the lobby ------------------------------------------------------------
  'lobby.games.subtitle': 'Всичко, което този сървър може да предложи',
  'lobby.games.bots': 'Ботове',
  'lobby.games.playBot': 'Играй срещу бот',
  'lobby.games.playBots': 'Играй срещу {n} бота',
  'lobby.games.openTable': 'Отвори маса',
  'lobby.games.players': 'Играчи: {n}',
  'lobby.games.playerRange': 'Играчи: {min}–{max}',
  'lobby.join.placeholder': 'Код за присъединяване или линк с покана',
  'lobby.join.needCode': 'Въведи код за присъединяване, връзка или ID на мач',
  'lobby.games.signInFirst': 'Първо влез',
  'lobby.join.action': 'Присъедини се',
  'lobby.join.waitingTitle': 'Чакаме домакина',
  // Two whole sentences rather than one with a swapped noun: "a game of {game}"
  // does not survive a language that inflects the game's name after "of".
  'lobby.join.joinedGame': 'Присъедини се към игра на {game} — чакаме старта',
  'lobby.join.joinedTable': 'Присъедини се към масата — чакаме старта',
  'lobby.table.addBot': 'Добави бот',
  'lobby.table.start': 'Започни',
  'lobby.table.waitingForHost': 'Чакаме домакинът да започне…',

  // --- inviting someone to a table ------------------------------------------
  'invite.heading': 'Покани играчи',
  'invite.explain': 'Изпрати този линк. Който го отвори, попада на тази маса — без нужда от акаунт.',
  'invite.noAddress': 'На този сървър не е зададен адрес за споделяне, затова използвай кода по-долу.',
  'invite.readOutCode': 'Или продиктувай кода:',
  'invite.copy': 'Копирай линка',
  'invite.share': 'Сподели линка',
  'invite.copied': 'Копирано!',
  'invite.shared': 'Споделено',

  // --- the match screen -----------------------------------------------------
  'match.waitingForTable': 'Чакаме масата…',
  'match.waitingForPlayer': 'Чакаме друг играч…',
  'match.nobodyWon': 'Никой не спечели.',
  'match.youWon': 'Ти спечели.',
  'match.finished': 'Този мач приключи.',
  'match.inProgress': 'Мачът тече — всичко е свързано и работи нормално.',
  'match.connecting': 'Свързване…',
  'match.abandonedTitle': 'Масата е оставена настрана',
  'match.abandoned': 'Никой не се върна на тази маса, затова тя беше оставена настрана. Картите са точно там, където ги остави.',
  'match.resume': 'Продължи оттам, докъдето стигна',
  'match.resuming': 'Масата се възстановява…',
  'match.controls': 'Управление',
  'match.over': 'Краят на мача',
  'match.settingUp': 'Подготвяме…',
  'match.playAgain': 'Играй отново',
  'match.backToGames': 'Обратно към игрите',
  'match.table': 'Маса',
  'match.opponents': 'Съперници',
  // Marks which seat is the reader's own, in a list of seats.
  'match.youSuffix': '(ти)',
  // The winner line is composed from whole sentences rather than a name glued
  // to " won." — the verb agrees with the subject in most of these languages,
  // and a suffix cannot know that.
  'match.you': 'ти',
  'match.someoneWon': '{name} спечели.',
  'match.wonBy': 'Спечелено от {names}.',
  'match.pausedFor': 'На пауза — чакаме {name} да се свърже отново.',
  'match.results': 'Резултати',
  'match.players': 'Играчи',
  'match.toPlay': 'на ход',

  // --- the offline score table ----------------------------------------------
  'scoring.namesHint': 'Имена, разделени със запетаи (4–8 играчи)',
  'scoring.newSession': 'Нова сесия',
  // A worked example, not a sentence: the names are placeholders a translator
  // may localise, the shape `name:score,` is what the parser needs.
  'scoring.scoresPlaceholder': 'Ана:120,Борис:80,…',
  'scoring.saveRound': 'Запази рунда',
  'scoring.export': 'Експортирай таблицата',
  'scoring.formatHint': 'Формат на точките: Име:100,Име2:50',
  'scoring.nameCountError': 'Въведи 2–8 имена на играчи, разделени със запетаи',
  'scoring.session': 'Сесия: {id}',
  'scoring.players': 'Играчи: {names}',
  'scoring.roundScores': 'Точки за рунд {n}',
  'stats.loading': 'Зарежда се…',
  // The figures could not be fetched. `{reason}` is whatever the server or the
  // network said, which is not ours to translate — the frame around it is.
  'stats.unavailable': '(недостъпно: {reason})',

  // --- statistics -----------------------------------------------------------
  'stats.title': 'Статистика и класация',
  'stats.yours': 'Твоята статистика',
  'stats.leaderboard': 'Класация',

  // --- a player's lifetime record, shown beside a finished match ------------
  'record.title': 'Твоят баланс',
  'record.guest':
    'Играеш като гост, затова не се води баланс. Влез и игрите, които вече си изиграл на това устройство — включително тази — ще останат към акаунта ти.',
  'record.signInToKeep': 'Влез и ги запази',
  'record.failed': 'Балансът ти не можа да се зареди в момента. Мачът е записан надеждно.',
  'record.loading': 'Зарежда се…',
  'record.played': 'Изиграни',
  'record.won': 'Спечелени',
  'record.lost': 'Загубени',
  'record.winRate': 'Процент победи',
  'record.streak': 'Серия',
  'record.atThisGame': 'В тази игра',
  // A streak is a count with a noun, so it is a whole phrase per count for
  // the same reason `countLabel` is: "1 win" / "2 wins" is an English rule,
  // and most of these languages do not share it.
  'record.streakNone': '—',
  'record.streakWinOne': '1 победа',
  'record.streakWinMany': '{n} победи',
  'record.streakLossOne': '1 загуба',
  'record.streakLossMany': '{n} загуби',

  // --- moving cards ---------------------------------------------------------
  'hand.dragHint': 'Влачи карта по ветрилото, за да я пренаредиш, или върху масата, за да я изиграеш',
  'hand.moveLeft': 'Наляво',
  'hand.moveRight': 'Надясно',
  'zone.collapseGroup': 'Свий тази група',
  'zone.expandGroup': 'Покажи всички карти в тази група',
  'zone.dropHere': 'Пусни тук',
  'offer.pickCards': 'избери карти за мястото, което докосна',
  'offer.ambiguous': 'това може да отиде на повече от едно място — избери на масата',

  // --- the build footer -----------------------------------------------------
  'build.app': 'приложение',
  'build.server': 'сървър',
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
  'option.pauseBetweenRounds': 'Пауза между рундовете',
  'choice.pauseBetweenRounds.1': 'Пауза',
  'choice.pauseBetweenRounds.0': 'Продължавай веднага',
  'option.botSkill': 'Съперници',
  'choice.botSkill.0': 'Смесени',
  'choice.botSkill.1': 'Лесни',
  'choice.botSkill.2': 'Средни',
  'choice.botSkill.3': 'Трудни',
  'option.initialMeldMinimum': 'Стойност за отваряне',
  'choice.initialMeldMinimum.0': 'Без',
  'option.discardDrawMinRound': 'Вземане от купчината',
  'choice.discardDrawMinRound.0': 'Отворено',
  'choice.discardDrawMinRound.2': 'От рунд 2',
  'choice.discardDrawMinRound.3': 'От рунд 3',
  'option.requireCleanRun': 'Поредица без жокер',
  'choice.requireCleanRun.1': 'Задължителна',
  'choice.requireCleanRun.0': 'Не',
  'option.jokerReclaimMustPlay': 'Откупен жокер',
  'choice.jokerReclaimMustPlay.1': 'Да се изиграе в същия ход',
  'choice.jokerReclaimMustPlay.0': 'Може да се задържи',
  'option.dealStarter': 'Кой започва',
  'choice.dealStarter.0': 'По ред',
  'choice.dealStarter.1': 'Започва победителят',
  'variation.prsi.classic': 'Класически',
  'option.handSize': 'Раздадени карти',
  'variation.canasta.classic': 'Класическа',
  'variation.canasta.modern_american': 'Modern American',
  'option.targetScore': 'Целеви резултат',
  'option.canastasToGoOut': 'Канасти за излизане',
  'variation.holdem.freezeout': 'Freezeout',
  'variation.holdem.timed': 'Фиксиран брой раздавания',
  'option.startingStack': 'Начални чипове',
  'option.bigBlind': 'Голям блайнд',
  'option.handLimit': 'Раздавания',
  'choice.handLimit.0': 'Докато остане едно място',
  'variation.ginrummy.standard': 'Стандартен',
  'option.knockLimit': 'Граница за чукане',
  'choice.knockLimit.0': 'Оклахома (определя я обърнатата карта)',
  'option.bigGin': 'Голям джин',
  'choice.bigGin.0': 'Изкл.',
  'choice.bigGin.1': 'Вкл. (+25)',
  'option.lineBonuses': 'Бонуси в равносметката',
  'choice.lineBonuses.1': 'Вкл.',
  'choice.lineBonuses.0': 'Изкл.',
  'variation.rummytiles.standard': 'Стандартен',
  'choice.targetScore.0': 'Без',
  // Two games qualify a number the others mean plainly: Canasta's 500 is a
  // short game, Gin Rummy's and Rummy Tiles' 500 is just 500; Hold'em's
  // starting 200 is a short stack, Blackjack's 200 is just 200. Module-scoped
  // so the qualifier travels with the game that means it — a bare key would
  // hand the note to games it is false for. `scripts/check-server-labels.js`
  // is what finds these; both were shipping as English on a Czech screen.
  'choice.canasta.targetScore.500': '500 (кратка)',
  'choice.holdem.startingStack.200': '200 (кратка)',
  'option.roundLimit': 'Лимит на рундове',
  'choice.roundLimit.0': 'Без',
  'option.poolExhaustion': 'Ако запасът свърши',
  'choice.poolExhaustion.1': 'Рундът се печели от най-ниската ръка',
  'choice.poolExhaustion.0': 'Никой не печели рунда',
  'variation.blackjack.single': 'Едно тесте',
  'option.minBet': 'Минимум на масата',
  'option.rounds': 'Рундове',
  'option.decks': 'Тестета',
  'option.dealerHitsSoft17': 'Крупието при мека 17',
  'choice.dealerHitsSoft17.0': 'Остава',
  'choice.dealerHitsSoft17.1': 'Тегли',
  'option.blackjackPays': 'Блекджекът плаща',
  'choice.blackjackPays.100': 'Едно към едно',
  'option.maxSplits': 'Разделяне',
  'choice.maxSplits.0': 'Без разделяне',
  'choice.maxSplits.1': 'Веднъж (две ръце)',
  'choice.maxSplits.3': 'Три пъти (четири ръце)',
  'option.doubleAfterSplit': 'Удвояване след разделяне',
  'choice.doubleAfterSplit.1': 'Разрешено',
  'choice.doubleAfterSplit.0': 'Забранено',
  'option.surrender': 'Отказ',
  'choice.surrender.0': 'Изкл.',
  'choice.surrender.1': 'Късен отказ',
  'option.insurance': 'Застраховка',
  'choice.insurance.1': 'Предлага се',
  'choice.insurance.0': 'Не се предлага',

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
  'verb.add': 'Добави',
  'verb.bet': 'Заложи',
  'verb.call': 'Плащам',
  'verb.check': 'Чек',
  'verb.commit': 'Готово',
  'verb.continue': 'Продължи',
  'verb.decline_insurance': 'Без застраховка',
  'verb.discard': 'Изхвърли',
  'verb.double': 'Удвои',
  'verb.draw': 'Тегли',
  'verb.finish_layoff': 'Готово с прикачването',
  'verb.fold': 'Пас на ръката',
  'verb.hit': 'Карта',
  'verb.insure': 'Застраховай се',
  'verb.knock': 'Чукни',
  'verb.lay_meld': 'Свали',
  'verb.lay_off': 'Прикачи',
  'verb.pass': 'Пас',
  'verb.place': 'Постави',
  'verb.play_card': 'Изиграй',
  'verb.raise': 'Вдигам',
  'verb.reset_turn': 'Върни хода',
  'verb.split': 'Раздели',
  'verb.stand': 'Оставам',
  'verb.surrender': 'Откажи се',
  'verb.swap_joker': 'Смени жокера',
  'verb.take': 'Вземи',
  'verb.take_pile': 'Вземи от купчината',
  // Declared by the server and listed in `serverKeys.json`, but never worded.
  'verb.takePileFromHand': 'Вземи купчината в ръката',
  'verb.takePileOntoMeld': 'Вземи купчината върху комбинация',
  'verb.undoDraw': 'Върни тегленето',
  'verb.undoLayOff': 'Върни прикачването',
  'verb.undoMeld': 'Върни комбинацията',
  'verb.undoTurn': 'Върни хода',

  // --- suits, spelled out ---------------------------------------------------
  //
  // Read aloud by a screen reader and shown where a pip would not fit. The
  // pips themselves are drawn, not written, so these are the only place the
  // suit is ever a word.
  'suit.C': 'Спатии',
  'suit.D': 'Каро',
  'suit.H': 'Купи',
  'suit.S': 'Пики',

  // --- counters the board prints beside a number ----------------------------
  'canasta.seat.notOpened': 'Не е отворил',
  'canasta.unit.points': 'точки',
  'ginrummy.unit.points': 'точки',
  'holdem.seat.dealer': 'Раздаващ',
  'holdem.unit.chips': 'чипа',
  'prsi.unit.cardsLeft': 'останали карти',
  'rummytiles.prompt.initialMeld': 'Първото ти сваляне трябва да струва {n} точки.',
  'rummytiles.unit.points': 'точки',
  'zolik.unit.penalty': 'наказание',
  'header.pileFrozen': 'Купчината е замразена',
  // The module-less form of `ginrummy.prompt.yourTurnDraw`: Žolíky and Canasta
  // send the prompt without a module prefix. Neither the Go constants nor
  // `serverKeys.json` list it — the sweep is what found it.
  'prompt.yourTurnDraw': 'Изтегли карта',
};
