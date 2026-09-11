/**
 * Czech. The first translation, and the one the fallback machinery was built for — see `countLabel`, which exists because Czech inflects a noun by its count in a way no "add an s" helper survives.
 */

export const cs: Record<string, string> = {
  // --- engine error codes (rules.RulesErrorCode) ---------------------------
  'err.NOT_YOUR_TURN': 'Nejsi na řadě',
  'err.WRONG_PHASE': 'Teď to nejde',
  'err.MUST_DRAW_FIRST': 'Nejdřív si lízni kartu',
  'err.GAME_SUSPENDED': 'Hra je pozastavena',
  'err.GAME_NOT_ACTIVE': 'Hra neběží',
  // --- runtime refusals, which are not the rules saying no -----------------
  //
  // Both were reachable long before they were worded, and a player who met
  // one was shown the code in capitals or, worse, nothing at all: an action
  // sent while the socket was down simply vanished, which reads as a legal
  // move being silently refused.
  'err.MATCH_NOT_ACTIVE': 'Stůl je pozastavený — čeká se na návrat hráče',
  'err.NOT_CONNECTED': 'Nejsi připojen ke stolu — připojuji znovu, pak to zkus',
  // Why the "start the next round" control is greyed out. Without wording the
  // shell printed the code itself, so a player agreeing to go on was answered
  // with ALREADY_READY in capitals.
  'err.ALREADY_READY': 'Potvrzeno',
  'err.NOT_BETWEEN_ROUNDS': 'Kolo ještě běží',
  'err.NOT_AT_THIS_TABLE': 'Nejsi u tohoto stolu',
  // Bringing a table back that the sweeper set aside. Refusals a player
  // can actually provoke: a link to a table that is gone, one that was
  // already picked up in another tab, and a table with other people at it.
  'err.MATCH_MOVED_ON': 'Stůl se mezitím posunul — načti stránku znovu',
  'err.MATCH_NOT_ABANDONED': 'Tenhle stůl na obnovení nečeká',
  'err.MATCH_NOT_FOUND': 'Tenhle stůl už neexistuje',
  'err.TABLE_HAS_OTHER_PLAYERS': 'Obnovit jde jen stůl, kde jsou všichni ostatní boti',
  'err.DISCARD_LOCKED': 'Odhazovací balíček je zatím zamčený',
  'err.DISCARD_PILE_EMPTY': 'Odhazovací balíček je prázdný',
  'err.NO_CARDS_LEFT': 'Už nezbývají žádné karty',
  'err.ROUND_REQ_NOT_MET': 'Nejdřív vylož vlastní první kombinaci',
  'err.NEED_CLEAN_RUN': 'Než budeš dole, musíš mít na stole čistou postupku bez žolíka',
  'err.INCOMPLETE_INITIAL_MELD': 'Dokonči výklad, nebo ho vrať zpět, než odhodíš',
  'err.DISCARD_CARD_NOT_MELDED': 'Vzatá karta musí jít do tvé kombinace',
  'err.JOKER_DISCARD_FORBIDDEN': 'Žolíka nelze odhodit',
  'err.NOTHING_TO_UNDO': 'Není co vrátit',
  'err.NO_JOKER_IN_MELD': 'V této kombinaci není žolík',
  'err.JOKER_SWAP_MISMATCH': 'Tato karta nenahradí žolíka',
  'err.RECLAIMED_JOKER_NOT_MELDED': 'Žolíka vzatého ze stolu musíš v tomto tahu zahrát do kombinace',
  'err.RUN_TOO_LONG': 'Postupka už je na plné délce',
  'err.WRONG_RUN_END': 'Tato karta patří na druhý konec postupky',
  'err.INVALID_MELD': 'Žádná karta v ruce sem nepasuje',
  'err.CARD_NOT_IN_HAND': 'Tuto kartu v ruce nemáš',
  'err.MELD_BELOW_MINIMUM': 'Tvé kombinace zatím nemají dost bodů na vyložení',
  'err.MELD_NO_CONTRIBUTION': 'Tato kombinace ti nepomůže splnit požadavek',
  'err.TOO_MANY_WILDS': 'Příliš mnoho žolíků v kombinaci',
  'err.ADJACENT_WILDS': 'Dva žolíci nemohou být vedle sebe',
  'err.ACE_BRIDGE': 'Eso nemůže spojit krále a dvojku',

  // --- contract phrasing ---------------------------------------------------
  // Built from the structured contract the server sends (sets/runs counts),
  // never from a deal-number lookup table.
  // Whole phrases per count rather than a number plus a pluralised noun:
  // Czech inflects the noun by count (skupina / skupiny / skupin) in a way no
  // "add an s" helper survives, and the counts that actually occur are 1-3.
  // A count outside that falls back to contract.sets.n.
  'contract.sets.1': 'Jedna skupina',
  'contract.sets.2': 'Dvě skupiny',
  'contract.sets.3': 'Tři skupiny',
  'contract.sets.n': '{n} skupin',
  'contract.runs.1': 'Jedna postupka',
  'contract.runs.2': 'Dvě postupky',
  'contract.runs.3': 'Tři postupky',
  'contract.runs.n': '{n} postupek',
  'contract.any': 'Jakákoli platná kombinace',
  'contract.cleanRunOnly': 'Libovolná směs skupin a postupek — alespoň jedna postupka bez žolíka',
  'contract.cleanRunSuffix': '{base} — jedna postupka bez žolíka',

  // --- written rules ---------------------------------------------------------
  // Full-sentence keys a "see the rules" screen renders, one per module,
  // resolved server-side against the table's actual variation and options —
  // see internal/module/rules.go and each game's rules.go. Shared section
  // titles first, then one block per module in the order the four games were
  // added.
  'zolik.rules.section.goal': 'Cíl',
  'zolik.rules.section.setup': 'Příprava',
  'zolik.rules.section.turn': 'Tvůj tah',
  'zolik.rules.section.melding': 'Vykládání',
  'zolik.rules.section.end': 'Konec zápasu',
  'zolik.rules.goal':
    'Jako první se zbav všech karet vykládáním platných skupin a postupek — a snaž se přitom mít v ruce co nejméně trestných bodů, až někdo jiný skončí kolo.',
  'zolik.rules.deal': 'Každý hráč dostane {n} karet.',
  'zolik.rules.meldShapes':
    'Skupina je {set}+ karet stejné hodnoty; postupka je {run}+ karet stejné barvy jdoucích po sobě.',
  'zolik.rules.turn.draw': 'Na svém tahu si lízni jednu kartu — z balíčku, nebo z odhazovacího balíčku.',
  'zolik.rules.pickup.topOnly': 'Z odhazovacího balíčku lze vzít jen vrchní kartu.',
  'zolik.rules.pickup.anyFromPile':
    'Z odhazovacího balíčku lze vzít libovolnou kartu i se vším, co je na ní navrch.',
  'zolik.rules.pickup.locked': 'Z odhazovacího balíčku nelze brát karty až do kola {n}.',
  'zolik.rules.pickup.open': 'Odhazovací balíček je otevřený už od prvního kola.',
  'zolik.rules.turn.discard': 'Tah ukonči odhozením jedné karty.',
  'zolik.rules.jokers.restricted':
    'Žolíka nelze odhodit, kromě situace, kdy jím právě zbavíš ruku poslední karty.',
  'zolik.rules.lead.rotate':
    'Kdo je na řadě jako první, se každé kolo posouvá o jedno místo dál — bez ohledu na to, kdo vyhrál.',
  'zolik.rules.lead.winner': 'Další kolo začíná ten, kdo se právě zbavil karet.',
  'zolik.rules.meldFloor.on':
    'Tvoje první kombinace (nebo kombinace) musí dohromady mít aspoň {n} přirozených bodů, než jsi dole.',
  'zolik.rules.meldFloor.off': 'Na první kombinaci není žádná minimální bodová hranice.',
  'zolik.rules.cleanRun.on':
    'Než se počítáš jako dole, musíš mít na stole aspoň jednu postupku úplně bez žolíka.',
  'zolik.rules.cleanRun.off': 'Postupky můžou žolíky používat volně — žádná nemusí být bez žolíka.',
  'zolik.rules.contracts.rotating':
    'Zápas má {n} rozdání a každé vyžaduje jinou kombinaci skupin a postupek.',
  'zolik.rules.contracts.static':
    'Každé rozdání vyžaduje stejnou kombinaci: {sets} skupin a {runs} postupek.',
  'zolik.rules.end.afterDeals': 'Zápas končí po {n} rozdáních.',
  'zolik.rules.end.atScore': 'Rozdává se dál, dokud někdo nedosáhne {n} bodů — pak zápas končí.',

  'prsi.rules.section.goal': 'Cíl',
  'prsi.rules.section.setup': 'Příprava',
  'prsi.rules.section.turn': 'Tvůj tah',
  'prsi.rules.section.special': 'Speciální karty',
  'prsi.rules.section.end': 'Konec hry',
  'prsi.rules.goal': 'Jako první se zbav všech karet z ruky.',
  'prsi.rules.deck': 'Hraje se s balíčkem {value} karet (od sedmy výš).',
  'prsi.rules.deal': 'Každý hráč začíná s {n} kartami.',
  'prsi.rules.turn.match':
    'Zahraj kartu, která odpovídá barvou nebo hodnotou vrchní kartě — pokud nemůžeš, lízni si.',
  'prsi.rules.turn.draw': 'Lízání ukončí tah bez zahrání karty.',
  'prsi.rules.sevens': 'Zahraješ-li sedmu, další hráč líže dvě karty, pokud nemá vlastní sedmu k odpovědi.',
  'prsi.rules.aces': 'Zahraješ-li eso, další hráč je vynechán.',
  'prsi.rules.queens': 'Zahraješ-li dámu, urči barvu, kterou se hraje dál.',
  'prsi.rules.end': 'Hra končí ve chvíli, kdy má někdo prázdnou ruku.',

  'canasta.rules.section.goal': 'Cíl',
  'canasta.rules.section.setup': 'Příprava',
  'canasta.rules.section.melding': 'Vykládání',
  'canasta.rules.section.end': 'Konec zápasu',
  'canasta.rules.goal': 'Hraje se ve dvojicích; zápas vyhrává strana, která první dosáhne {n} bodů.',
  'canasta.rules.deck': 'Hraje se s {value} kartami — {decks} balíčky plus žolíci.',
  'canasta.rules.deal': 'Každý hráč dostane {n} karet.',
  'canasta.rules.drawCount': 'Na začátku tahu si líznete {n} karty.',
  'canasta.rules.redThrees':
    'Červenou trojku v ruce hned ukážeš a započítá se jako bonus — pokud ale tvá strana nedokončí ani jednu kanastu, počítá se naopak proti vám.',
  'canasta.rules.canasta': 'Kanasta je kombinace {n} a více karet stejné hodnoty.',
  'canasta.rules.sequences': 'Kombinace může být i postupka: tři a více karet stejné barvy za sebou, nikdy se žolíkem mezi nimi.',
  'canasta.rules.samba': 'Postupka ze sedmi karet je samba a má hodnotu {n} bodů.',
  'canasta.rules.pileAlwaysFrozen': 'Odhazovací balíček je zamrzlý po celé rozdání: vzít si ho můžete jen tak, že k vrchní kartě přiložíte dvě přirozené karty z ruky.',
  'canasta.rules.meldFloorBands':
    'Minimální hodnota první kombinace roste s vaším skóre: {negative} pod nulou, {low} do 1500, {mid} do 3000, {high} nad tím.',
  'canasta.rules.meldFloorBandsFive': 'Vaše první kombinace musí dosáhnout bodového minima, které roste s vaším skóre: {negative} pod nulou, {low} do 1500, {mid} do 3000, {high} do 7000 a {top} nad tím.',
  'canasta.rules.oneCanastaToGoOut': 'Vaší straně stačí k ukončení hry jedna dokončená kanasta.',
  'canasta.rules.twoCanastasToGoOut': 'Vaše strana potřebuje k ukončení hry dvě dokončené kanasty.',
  'canasta.rules.end': 'Rozdává se dál, dokud jedna strana nepřekročí {n} bodů — pak zápas končí.',

  'holdem.rules.section.goal': 'Cíl',
  'holdem.rules.section.setup': 'Příprava',
  'holdem.rules.section.betting': 'Sázení',
  'holdem.rules.section.end': 'Konec zápasu',
  'holdem.rules.goal':
    'Vyhrávej žetony nejlepší kombinací u vyhodnocení, nebo tím, že v kole zůstaneš jako jediný.',
  'holdem.rules.stack': 'Každé místo začíná s {n} žetony.',
  'holdem.rules.blinds': 'Malý blind je {sb} a velký blind {bb}, vkládají se ještě před rozdáním karet.',
  'holdem.rules.streets': 'Sázelo se ve čtyřech kolech — před flopem a po rozdání flopu, turnu a riveru.',
  'holdem.rules.showdown':
    'Hráči, kteří zůstali ve hře, odkryjí karty; bank bere nejlepší kombinace z pěti karet.',
  'holdem.rules.noLimit': 'Sázení bez limitu — vsadit lze libovolnou částku až do výše celého stacku.',
  'holdem.rules.lastPlayerStanding': 'Hraje se, dokud jedno místo nezíská všechny žetony.',
  'holdem.rules.mostChipsWins': 'Zápas vyhrává ten, kdo má na konci nejvíc žetonů.',
  'holdem.rules.handLimit': 'Hraje se {n} rozdání.',

  // --- header --------------------------------------------------------------
  'header.deal': 'Rozdání {n}',
  'header.gameOf': 'Hra {n} ze {total}',
  'header.gameOfWithContract': 'Hra {n} ze {total}: {contract}',

  // --- meld preview --------------------------------------------------------
  'preview.validSet': 'Platná skupina',
  'preview.validRun': 'Platná postupka',
  'preview.validMeld': 'Platná kombinace',
  'preview.notYet': 'Zatím není kombinace',
  'preview.points': '{shape} · {n} bodů',
  'preview.pointsWithLaid': '{shape} · {n} + {laid} již vyloženo = {total} bodů',
  'preview.meetsFloor': '{line} (splňuje {n} ✓)',
  'preview.needsFloor': '{line} (potřebuje {n} ✗)',
  'preview.becauseOf': '{line} — {reason}',

  // --- discard -------------------------------------------------------------
  // A discard with a meld staged lays that meld first; this is what the
  // player is told when the server refuses it and the whole move is rolled
  // back. {reason} is the engine's own words for the refusal.
  'discard.meldRejected': '{reason} — nic se neodhodilo, karty máš pořád připravené.',

  // --- what the current selection would send -------------------------------
  // The client's own reason a control is not ready, next to the engine's own
  // `err.*` reasons rather than a second kind of message — a control greyed
  // out for "not your turn" and one greyed out for "you picked two, this
  // takes one" should read as the same kind of thing, not one of them looking
  // broken. See `fits` in `src/lib/drops.ts`.
  'sel.tooMany.1': 'Vyber jen jednu kartu',
  'sel.tooMany.n': 'Vyber nejvýš {n} karet',
  'sel.needMore': 'Vyber {n} kartu/karty',
  'sel.notThese': 'Tyhle karty sem nepatří',
  'sel.needsCompany': 'Tahle karta potřebuje ty vedle sebe',

  // --- what a module says happened -----------------------------------------
  // The first module-sent keys in this bundle, and deliberately few. Anything
  // a module sends is still legible without an entry here — `humanise` turns
  // `holdem.seat.stack` into "Stack" — so a key earns a line only when that
  // fallback loses something. These do: what each of them means lives in its
  // *params* (who won, how much, whose cards), and a key on its own has
  // nowhere to put them. Without the entry the player read "Winner", full
  // stop, at the end of a match they had just won.
  'status.winner': 'Vítěz: {winners}',
  'holdem.status.pot': 'Bank {amount} bere {winners} — {hand}',
  'holdem.status.potUncontested': 'Bank {amount} bere {winners} — ostatní složili',
  'holdem.status.shown': 'Karty hráče {playerId}: {value}',
  'holdem.prompt.waitingFor': 'Čeká se na hráče {playerId}',

  // How a rummy scoreboard was ordered. Both earn a line for the same reason
  // as the keys above: the number lives in the params, and `humanise` would
  // render "Deals Won" with nothing after it.
  'zolik.standing.dealsWon': 'Vyhraných kol {n}',
  'zolik.standing.inHand': 'V ruce {n}',

  // --- rounds ---------------------------------------------------------------
  // What a round is called, and what one did. Each of these carries its number
  // in the params, so `humanise` alone would render a label with nothing after
  // it.
  // An instruction, not a heading. "Next round" named the thing rather than
  // asking for it, and sat above "Waiting for 1" — which reads as though the
  // player is the one waiting, when the table is waiting for them.
  'round.continue': 'Začít další kolo',

  // --- the flash that announces a round or a match ending ------------------
  //
  // One line and one number, held for about two seconds. Worded generically on
  // purpose: which *kind* of ending it was is the module's own fact, printed
  // underneath, and a client that phrased "went gin" itself would be a client
  // that knows a game.
  'flash.roundWon': 'Kolo bere {winners}',
  'flash.roundWonYou': 'Kolo bereš ty',
  'flash.roundDrawn': 'Kolo nikdo nebere',
  'flash.matchOver': 'Konec zápasu',
  'flash.matchWon': 'Zápas bere {winners}',
  'flash.matchWonYou': 'Zápas bereš ty',
  'flash.matchDrawn': 'Nikdo nevyhrál',
  'flash.nowOn': 'nyní {total}',

  'zolik.round.deal': 'Rozdání',
  'zolik.round.cleanRun': 'Jedna postupka bez žolíka',
  'canasta.round.deal': 'Rozdání',
  'canasta.round.concealed': 'Vyložení naráz',
  'canasta.round.exhausted': 'Došel balíček',
  'canasta.round.meldCards': 'Vyložené karty {n}',
  'canasta.round.canastas': 'Kanasty {n}',
  'canasta.round.redThrees': 'Červené trojky {n}',
  'canasta.round.goingOut': 'Za ukončení {n}',
  'canasta.round.inHand': 'Zbylo v ruce {n}',
  'holdem.round.hand': 'Hra',
  'holdem.round.pot': 'Bank {n}',
  'holdem.round.uncontested': 'Ostatní složili',
  'seat.ready': 'Připraven',
  'zolik.seat.contractMet': 'Závazek splněn',
  'results.you': '(ty)',



  // --- refusals the other three games raise ---------------------------------
  // Worded here for the same reason Žolíky's are: reasonText falls back to the
  // code, and OfferBar hands it the code as that fallback, so an unworded
  // refusal reaches a player as PILE_FROZEN in capitals. serverKeys.test.ts
  // now fails the build rather than letting the next one through.
  'err.SET_TOO_LARGE': 'Skupina už má všechny čtyři barvy',
  'err.DISCARD_TAKEN_CARD_FORBIDDEN':
    'Kartu, kterou sis právě vzal, nemůžeš odhodit — zahraj ji, nebo si ji nech',
  'err.CARD_DOES_NOT_FIT': 'Tato karta nesedí barvou ani hodnotou',
  'err.SUIT_REQUIRED': 'Řekni, jaká barva se hraje dál',
  'err.MUST_ANSWER_DRAW_OR_TAKE': 'Odpověz sedmičkou, nebo si karty vezmi',
  'err.NOTHING_TO_DRAW': 'Není co líznout',
  'err.PILE_EMPTY': 'Balíček je prázdný',
  'err.PILE_BLOCKED': 'Balíček je zablokovaný — nahoře leží černá trojka',
  'err.PILE_FROZEN': 'Balíček je zmrazený — potřebuješ dvě přirozené karty hodnoty vrchní karty',
  'err.TOP_CARD_UNUSABLE': 'Vrchní kartu použít nemůžeš',
  'err.MELD_CLOSED': 'Tato kombinace je hotová a uzavřená',
  'err.MELD_TOO_SMALL': 'Kombinace potřebuje víc karet',
  'err.MELD_TOO_LARGE': 'Do této kombinace už další karty nejdou',
  'err.MELD_MIXED_RANKS': 'Všechny karty v kombinaci musí mít stejnou hodnotu',
  'err.SEQUENCE_NO_WILDS': 'Postupka nesmí obsahovat žolíky',
  'err.SEQUENCE_NEEDS_ONE_SUIT': 'Všechny karty postupky musí být stejné barvy',
  'err.RUN_NOT_CONSECUTIVE': 'Postupka musí jít popořadě, bez mezer',
  'err.NOT_ENOUGH_NATURALS': 'Kombinace potřebuje víc přirozených karet než žolíků',
  'err.RANK_ALREADY_MELDED': 'Tvoje strana už kombinaci této hodnoty má',
  'err.NOT_YOUR_MELD': 'Tato kombinace patří druhé straně',
  'err.NO_SUCH_MELD': 'Taková kombinace na stole není',
  'err.CANNOT_MELD_THREE': 'Trojky se nevykládají',
  'err.CANNOT_DISCARD_RED_THREE': 'Červená trojka se nedá odhodit',
  'err.MUST_KEEP_A_CARD': 'Nech si aspoň jednu kartu — takhle ruku vyprázdnit nemůžeš',
  'err.MUST_MELD_FIRST': 'Než to uděláš, vylož první kombinaci své strany',
  'err.INITIAL_MELD_NOT_MET': 'První kombinaci ještě chybí body',
  'err.CANNOT_GO_OUT_YET': 'Tvoje strana potřebuje hotovou canastu, než může vyjít',
  'err.NOTHING_TO_CALL': 'Není co dorovnat',
  'err.CANNOT_CHECK': 'Nemůžeš čekat — je tu sázka k dorovnání',
  'err.CANNOT_RAISE': 'Tady zvýšit nemůžeš',
  'err.RAISE_TOO_SMALL': 'Zvýšení musí být aspoň o poslední sázku',
  'err.NOT_ENOUGH_CHIPS': 'Tolik žetonů nemáš',
  'err.AMOUNT_REQUIRED': 'Zadej kolik',
  'err.AMOUNT_NOT_A_NUMBER': 'Tato částka není číslo',
  'err.SEAT_NOT_IN_HAND': 'V tomto rozdání nehraješ',
  'err.WRONG_RANK': 'Tato karta má pro tohle špatnou hodnotu',
  'err.MATCH_FULL': 'Stůl je plný',
  'err.MATCH_ALREADY_STARTED': 'Zápas už začal',
  'err.TOO_FEW_PLAYERS': 'Zatím je málo hráčů',
  'err.WRONG_PLAYER_COUNT': 'Tuto hru nelze hrát s tímto počtem hráčů',
  'err.NOT_THE_HOST': 'To může udělat jen zakladatel stolu',
  'err.BAD_SEATING': 'Toto rozsazení neodpovídá tomu, kdo je u stolu',
  'err.NO_LONGER_WAITING': 'Stůl už nečeká',
  'err.WAITING_ROOM_UNAVAILABLE': 'Čekárna není dostupná',
  'err.SERVER_BUSY': 'Server je právě plný — zkuste to za chvíli',


  // --- rules the engine has always enforced and never stated ----------------
  // Every one of these was found by the guardrail rather than by review: the
  // validator could refuse a player for it, and no sentence anywhere said so.
  // See server/internal/zolikmod/ruleindex_test.go.
  'zolik.rules.section.layoff': 'Přikládání ke kombinacím',
  'zolik.rules.pickup.obligation':
    'Dokud nejsi dole, musí karta vzatá z odhazovacího balíčku být použita v kombinaci, se kterou v tomto tahu jdeš dolů.',
  'zolik.rules.pickup.noReturn':
    'Kartu vzatou z odhazovacího balíčku nemůžeš ve stejném tahu odhodit — zahraj ji, nebo si ji nech.',
  'zolik.rules.wilds.setLimit': 'Skupina nesmí mít víc žolíků než přirozených karet.',
  'zolik.rules.set.maxSize':
    'Skupina nesmí mít víc než {n} karty — žolík doplní chybějící barvu, nenafukuje plnou.',
  'zolik.rules.run.maxLength':
    'Postupka je nejvýš {n} karet dlouhá — eso dole, dvanáct hodnot nad ním a eso nahoře.',
  'zolik.rules.run.aceBridge': 'Eso leží nad králem nebo pod dvojkou, nikdy nespojuje oba konce postupky.',
  'zolik.rules.contracts.contribution':
    'Dokud nejsi dole, musí každá vyložená kombinace být taková, jakou zadání rozdání ještě potřebuje.',
  'zolik.rules.layoff.afterDown': 'Ke kombinacím nemůžeš přikládat, dokud nevyložíš vlastní zadání.',
  'zolik.rules.layoff.runEnds': 'Karta přiložená k postupce ji musí prodloužit na jednom nebo druhém konci.',
  'zolik.rules.jokers.swap':
    'Žolíka v kombinaci na stole můžeš vyměnit za přesně tu kartu, kterou zastupuje.',
  'zolik.rules.jokers.reclaim.on':
    'Žolíka vzatého ze stolu musíš ve stejném tahu zahrát do kombinace — nesmí ti zůstat v ruce.',
  'zolik.rules.jokers.reclaim.off': 'Žolíka vzatého ze stolu si můžeš nechat v ruce.',
  'zolik.rules.deck.reshuffle':
    'Když dojde lízací balíček, odhazovací balíček se zamíchá a stane se novým lízacím; pokud jsou prázdné oba, rozdání končí.',


  // --- what to do instead ---------------------------------------------------
  // The third layer of a refusal, after the reason and the rule. Sent by the
  // module because it is the only side that knows which card is owed and
  // which way out is on offer; see server/internal/zolikmod/remedy.go.
  'zolik.remedy.meldThePickup': 'Přidej {card} do výkladu, nebo vrať vzetí zpět.',
  'zolik.remedy.discardSomethingElse': 'Odhoď jinou kartu, nebo {card} v tomto tahu zahraj.',
  'zolik.remedy.discardNotAJoker': 'Odhoď něco jiného než žolíka.',
  'zolik.remedy.finishOrUndoLayDown': 'Dokonči výklad, nebo ho vrať zpět.',
  'zolik.remedy.needMorePoints': 'Než půjdeš dolů, chybí ti ještě {n} bodů.',
  'zolik.remedy.layACleanRun': 'Vylož postupku bez žolíka.',
  'zolik.remedy.playReclaimedJoker': 'Zahraj {card} do kombinace, nebo vrať vzetí zpět.',
  'zolik.remedy.goDownFirst': 'Nejdřív vylož vlastní kombinace.',
  'zolik.remedy.drawFirst': 'Nejdřív si lízni kartu.',
  'zolik.remedy.drawFromStock': 'Lízni si z balíčku — odhazovací balíček se otevře v kole {n}.',
  'zolik.remedy.drawFromStockEmpty': 'Lízni si místo toho z balíčku.',


  // --- keys whose meaning lives in their params -----------------------------
  // The shape fallback renders these with the number missing ("Stack", "Pot"),
  // which is why they earn a line where most keys do not.
  'header.contract': 'Potřebuje {sets} skupiny a {runs} postupky',
  'header.contract.cleanRunOnly': 'Potřebuje postupku bez žolíka',
  'header.round': 'Kolo {n}',
  'header.deck': 'Balíček',
  'header.target': 'Cíl',
  'header.suitInPlay': 'Hraje se',
  'seat.cards': 'Karet',
  'zolik.offer.meld': 'Kombinace',
  'prompt.pickupMustBeMelded':
    '{value} je z odhazovacího balíčku — musí jít do kombinací, se kterými v tomto tahu jdeš dolů.',
  'prompt.jokerMustBePlayed': '{value} je ze stolu — než ukončíš tah, musí jít do kombinace.',
  'prompt.initialMeld': 'První kombinace tvé strany musí mít {n} bodů.',
  'prompt.canastasNeeded': 'Tvé straně chybí ještě {n} canasty, než může vyjít.',
  'prompt.mustDrawOrAnswerSeven': 'Odpověz sedmičkou, nebo si lízni {n} karty.',
  'prompt.chooseSuit': 'Vyber barvu, která se hraje dál',
  'prompt.skipPending': 'Tvůj tah se přeskakuje',
  'status.lastDeal': 'Tým {team} získal {value}',
  'status.teamScore': 'Tým {team}: {value}',
  'canasta.offer.rank': 'Hodnota',
  'canasta.offer.sequence': 'Postupka',
  'badge.naturalCanasta': 'Čistá kanasta',
  'badge.mixedCanasta': 'Nečistá kanasta',
  'badge.samba': 'Samba',
  'badge.cleanRun': 'Čistá postupka',
  'canasta.seat.teamScore': 'Skóre týmu',
  'canasta.seat.canastas': 'Canasty',
  'holdem.header.pot': 'Bank',
  'holdem.header.street': 'Fáze',
  'holdem.header.hand': 'Rozdání',
  'holdem.header.handLimit': 'Rozdání celkem',
  'holdem.header.blinds': 'Blindy',
  'holdem.cost.call': 'k dorovnání',
  'holdem.cost.pot': 'v banku',
  'holdem.seat.stack': 'Žetony',
  'holdem.seat.bet': 'Sázka',
  'holdem.prompt.yourAction': 'Jsi na tahu',
  'holdem.prompt.raiseTo': 'Zvýšit na',
  'holdem.quick.halfPot': '½ Bank',
  'holdem.quick.pot': 'Bank',
  'holdem.quick.allIn': 'All-in',
  'zone.yourHand': 'Tvoje karty',
  'zone.opponentHand': 'Karty soupeře',
  'zone.drawPile': 'Lízací balíček',
  'zone.discardPile': 'Odhazovací balíček',
  'zone.melds': 'Kombinace',
  'zone.teamMelds': 'Kombinace tvé strany',
  'zone.opponentMelds': 'Kombinace soupeře',
  'zone.redThrees': 'Červené trojky',
  'zone.board': 'Stůl',
  'verb.drawFromDeck': 'Líznout',
  'verb.takeFromDiscard': 'Vzít z balíčku',


  // --- the why sheet's own furniture ---------------------------------------
  // The three layers a refusal is explained in. Labels, not sentences: what
  // goes under each is sent by the server or worded above.
  'why.reason': 'Proč ne',
  'why.rule': 'Pravidlo',
  'why.rules': 'Pravidla',
  'why.remedy': 'Co s tím můžeš udělat',
  'why.readTheRules': 'Zobrazit celá pravidla →',
  'why.close': 'Zavřít',
  'why.open': 'proč',

  // --- marks on a particular card ------------------------------------------
  // A mark is a refusal that has not happened yet: the rule is enforced at the
  // discard, which is the last possible moment to hear about it, so the card
  // says so while there is still a turn left to act on it.
  'zolik.badge.owedToMeld':
    '{card} je z odhazovacího balíčku — musí jít do kombinací, se kterými v tomto tahu jdeš dolů.',
  'zolik.badge.jokerOwed': '{card} je ze stolu — než ukončíš tah, musí jít do kombinace.',

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
  'legal.terms': 'Podmínky',
  // The document's own name, used as the heading above it. Kept apart from
  // `legal.notice.terms`, which is the same words inflected to sit inside a
  // sentence — a heading in the instrumental case reads as a mistake in every
  // Slavic language.
  'legal.terms.title': 'Podmínky použití',
  'legal.privacy.title': 'Zásady ochrany osobních údajů',
  'legal.privacy': 'Soukromí',
  'legal.source': 'Zdrojový kód',
  'legal.updated': 'Verze {version}',
  'legal.draft': 'Návrh — zatím neplatí. Jméno provozovatele, stát a kontaktní adresa se teprve doplní.',
  'legal.notice.before': 'Hraním souhlasíš s ',
  'legal.notice.terms': 'Podmínkami použití',
  'legal.notice.between': '. Co o tobě ukládáme, popisují ',
  'legal.notice.privacy': 'Zásady ochrany osobních údajů',
  'legal.notice.after': '.',
  // --- gin rummy -------------------------------------------------------------
  'err.UPCARD_DECLINED': 'Tuhle kartu jsi už nechal',
  'err.DEADWOOD_TOO_HIGH': 'Máš příliš mnoho mrtvých bodů na klepnutí',
  'err.CARD_DOES_NOT_EXTEND_MELD': 'Tahle karta se do kombinace nehodí',
  'ginrummy.rules.setup': 'Příprava',
  'ginrummy.rules.turn': 'Tvůj tah',
  'ginrummy.rules.melds': 'Kombinace',
  'ginrummy.rules.knocking': 'Klepnutí',
  'ginrummy.rules.bigGin': 'Velký gin',
  'ginrummy.rules.layoff': 'Přiložení',
  'ginrummy.rules.deadHand': 'Mrtvé kolo',
  'ginrummy.rules.scoring': 'Vyhodnocení kola',
  'ginrummy.rules.match': 'Výhra partie',
  'ginrummy.rules.lineBonuses': 'Bonusy za partii',
  'ginrummy.rules.deck': 'Hraje se s balíčkem {value} karet.',
  'ginrummy.rules.deal': 'Každý hráč dostane {value} karet.',
  'ginrummy.rules.upcard': 'Jedna další karta se otočí lícem nahoru a založí odhazovací balíček.',
  'ginrummy.rules.drawDiscard':
    'Ve svém tahu líznu jednu kartu — z balíčku nebo z odhazovacího balíčku — a jednu odhodíš.',
  'ginrummy.rules.setsAndRuns':
    'Kombinace je trojice nebo čtveřice karet stejné hodnoty, nebo postupka tří a více karet stejné barvy.',
  'ginrummy.rules.aceLow': 'Eso je vždy nejnižší karta — postupka od Q přes K až po A neexistuje.',
  'ginrummy.rules.knockLimit': 'Klepnout můžeš, jakmile máš {n} nebo méně mrtvých bodů.',
  'ginrummy.rules.oklahoma': 'Limit pro klepnutí v tomto kole určuje hodnota otočené karty.',
  'ginrummy.rules.gin': 'Nula mrtvých bodů je gin — nejlepší možné klepnutí.',
  'ginrummy.rules.bigGinBonus':
    'Jedenáct karet spojených bez jakéhokoli odhozu je velký gin, za který je bonus {n} bodů navíc.',
  'ginrummy.rules.layoffDescription':
    'Po klepnutí, které není gin, může soupeř přiložit své mrtvé karty na tvoje kombinace, než se ruce porovnají.',
  'ginrummy.rules.deadHandDescription':
    'Pokud balíčku zbydou poslední dvě karty a nikdo neklepnul, je kolo mrtvé — nikdo nebodoval a rozdává stejný hráč znovu.',
  'ginrummy.rules.undercut':
    'Pokud soupeř nemá víc mrtvých bodů než ty, podklepne tě: získá rozdíl plus {n}.',
  'ginrummy.rules.ginBonus': 'Gin bodově znamená celou soupeřovu ruku plus {n}.',
  'ginrummy.rules.target': 'Partii vyhrává první hráč, který po skončení kola překročí {n} bodů.',
  'ginrummy.rules.shutout':
    'Bonus za partii se zdvojnásobí na {n}, pokud poražený za celou partii nezískal ani bod.',
  'ginrummy.rules.box': 'Každé vyhrané kolo je na konci partie navíc {n} bodů.',
  'ginrummy.rules.gameBonus': 'Výhra celé partie je navíc {n} bodů.',
  'ginrummy.fact.deadwood': '{value} mrtvých bodů',
  'ginrummy.fact.discardCard': 'Odhodit {value}',
  'ginrummy.fact.meldCards': 'Na {value}',
  'ginrummy.header.hand': 'Kolo {n}',
  'ginrummy.round.kind': '{value}',
  'ginrummy.round.hand': 'Kolo',
  'ginrummy.seat.score': '{value}',
  'ginrummy.seat.dealer': 'Rozdávající',
  'ginrummy.status.knocked': '{playerId} klepnul s {deadwood} mrtvými body',
  'ginrummy.status.gin': '{playerId} má gin',
  'ginrummy.status.lastHand': 'Poslední kolo: {winner} ({kind}, {delta} bodů)',
  'ginrummy.offer.drawStock': 'Líznout z balíčku',
  'ginrummy.offer.drawDiscard': 'Líznout z odhazovacího balíčku',
  'ginrummy.offer.takeUpcard': 'Vzít otočenou kartu',
  'ginrummy.offer.passUpcard': 'Nechat',
  'ginrummy.offer.discard': 'Odhodit',
  'ginrummy.offer.knock': 'Klepnout',
  'ginrummy.offer.gin': 'Gin!',
  'ginrummy.offer.bigGin': 'Velký gin!',
  'ginrummy.offer.layOff': 'Přiložit',
  'ginrummy.offer.finishLayoff': 'Hotovo s přikládáním',
  'ginrummy.zone.knockerHand': 'Odklepnutá ruka',
  'ginrummy.zone.melds': 'Kombinace',
  'ginrummy.prompt.upcardDecision': 'Vezmi otočenou kartu, nebo ji nechej',
  'ginrummy.prompt.yourTurnDraw': 'Lízni kartu',
  'ginrummy.prompt.yourTurnDiscard': 'Odhoď — nebo klepni, pokud můžeš',
  'ginrummy.prompt.layoff': 'Přilož mrtvé karty, nebo skonči',

  // --- rummy tiles -------------------------------------------------------------
  'err.TILE_NOT_IN_HAND': 'Tenhle kámen nemáš v ruce',
  'err.TILE_DOES_NOT_FIT': 'Tohle sem nepatří',
  'err.NO_SUCH_SET': 'Tahle kombinace není na stole',
  'err.INITIAL_MELD_ONLY': 'Před svým prvním vyložením smíš upravovat jen vlastní nové kombinace',
  'err.TABLE_NOT_VALID': 'Stůl zatím není v pořádku',
  'err.TRAY_NOT_EMPTY': 'Ještě máš volné kameny k umístění',
  'err.NOTHING_PLAYED': 'Než skončíš tah, musíš položit aspoň jeden kámen',
  'err.INITIAL_MELD_TOO_LOW': 'Tvoje první vyložení musí mít hodnotu aspoň 30 bodů',
  'err.NOT_A_RUN': 'Rozdělit lze jen řadu',
  'err.BAD_SPLIT_POSITION': 'Na tomhle místě se řada rozdělit nedá',
  'err.NO_JOKER_IN_SET': 'V téhle kombinaci není žádný žolík',
  'err.TILE_JOKER_SWAP_MISMATCH': 'Tenhle kámen neodpovídá tomu, co žolík zastupuje',
  'rummytiles.rules.setup': 'Příprava',
  'rummytiles.rules.sets': 'Kombinace',
  'rummytiles.rules.initialMeld': 'Vstupní vyložení',
  'rummytiles.rules.turn': 'Tvůj tah',
  'rummytiles.rules.jokerTaking': 'Braní žolíka',
  'rummytiles.rules.ending': 'Konec kola',
  'rummytiles.rules.poolExhaustion': 'Když dojde banka',
  'rummytiles.rules.match': 'Výhra partie',
  'rummytiles.rules.tiles': 'Hraje se s {value} kameny.',
  'rummytiles.rules.dealCount': 'Každý hráč dostane {value} kamenů.',
  'rummytiles.rules.group': 'Skupina jsou tři nebo čtyři kameny stejného čísla, každý jiné barvy.',
  'rummytiles.rules.run': 'Řada jsou tři a více po sobě jdoucích čísel jedné barvy.',
  'rummytiles.rules.noWrap': '13 nenavazuje zpátky na 1.',
  'rummytiles.rules.joker': 'Žolík zastupuje libovolný kámen.',
  'rummytiles.rules.initialMeldDescription':
    'Dokud v jednom tahu nepoložíš {n} a více bodů výhradně z vlastní ruky, nesmíš sahat na nic, co už je na stole.',
  'rummytiles.rules.turnDescription':
    'Polož aspoň jeden kámen z ruky, stůl si volně uprav podle potřeby a skonči s tím, že každá kombinace na stole je platná.',
  'rummytiles.rules.noDiscard':
    'Neexistuje odhoz — pokud tah nedokážeš dokončit platně, místo toho lízneš kámen.',
  'rummytiles.rules.jokerTakingDescription':
    'Žolíka na stole smíš vzít, když ho nahradíš kamenem, který zastupuje, z vlastní ruky — a musíš ho ještě tentýž tah použít v nějaké kombinaci.',
  'rummytiles.rules.goingOut':
    'Kolo vyhrává první hráč, kterému dojdou kameny. Ostatním se počítá záporná hodnota toho, co jim zbylo v ruce; vítěz získá součet toho, co ostatní prohráli.',
  'rummytiles.rules.poolExhaustionLowestWins':
    'Když dojde banka a nikdo nemůže hrát, kolo končí a vyhrává ten s nejnižší hodnotou v ruce.',
  'rummytiles.rules.poolExhaustionNoWinner':
    'Když dojde banka a nikdo nemůže hrát, kolo končí bez vítěze — každému se prostě spočítá jeho ruka.',
  'rummytiles.rules.target': 'Partii vyhrává první hráč, který po skončení kola překročí {n} bodů.',
  'rummytiles.rules.roundLimit': 'Partie končí po {n} kolech — vyhrává nejvyšší skóre.',
  'rummytiles.fact.setCards': '{value}',
  'rummytiles.header.pool': 'Banka {n}',
  'rummytiles.header.round': 'Kolo {n}',
  'rummytiles.round.kind': '{value}',
  'rummytiles.round.round': 'Kolo',
  'rummytiles.seat.score': '{value}',
  'rummytiles.seat.notOpened': 'Ještě nevyložil',
  'rummytiles.status.lastRound': 'Poslední kolo: {winner} ({kind})',
  'rummytiles.badge.invalid': 'Zatím neplatné',
  'rummytiles.zone.pool': 'Banka',
  'rummytiles.zone.table': 'Stůl',
  'rummytiles.zone.tray': 'Odkladiště',
  'rummytiles.offer.place': 'Položit',
  'rummytiles.offer.addFromHand': 'Přidat',
  'rummytiles.offer.addFromTray': 'Přidat z odkladiště',
  'rummytiles.offer.take': 'Vzít',
  'rummytiles.offer.split': 'Rozdělit',
  'rummytiles.offer.swapJoker': 'Vyměnit žolíka',
  'rummytiles.offer.resetTurn': 'Vrátit tah',
  'rummytiles.offer.commit': 'Hotovo',
  'rummytiles.offer.draw': 'Líznout',
  'rummytiles.param.position': 'Rozdělit na',

  // --- blackjack -----------------------------------------------------------
  //
  // The house rules are worded twice over — on and off — because a rule that
  // is *not* in force is still something a player has to be told. "No
  // surrender at this table" is information; silence is a guess.
  'err.BET_BELOW_MINIMUM': 'Sázka je pod minimem stolu',
  'err.ALREADY_BET': 'Sázka už je vsazená',
  'err.INSURANCE_CLOSED': 'Pojištění teď nabídnout nelze',
  'err.CANNOT_DOUBLE': 'Tuhle ruku nelze zdvojit',
  'err.CANNOT_SPLIT': 'Tuhle ruku nelze rozdělit',
  'err.CANNOT_SURRENDER': 'Tuhle ruku nelze vzdát',

  'blackjack.rules.section.table': 'Stůl',
  'blackjack.rules.section.play': 'Hra s rukou',
  'blackjack.rules.section.dealer': 'Krupiér',
  'blackjack.rules.section.end': 'Konec zápasu',
  'blackjack.rules.goal':
    'Poraz krupiéra a nepřetáhni přes jednadvacet. Kdo přetáhne, prohrává hned, ať krupiér udělá potom cokoli.',
  // Counts are their own phrase rather than a number glued to a noun: one
  // deck and six decks inflect differently, and Czech inflects them again.
  'blackjack.rules.decks': 'Balíčků v botě: {n}.',
  'blackjack.rules.stack': 'Každé místo začíná s {n} žetony.',
  'blackjack.rules.minBet': 'Minimální sázka u stolu je {n} žetonů.',
  'blackjack.rules.faceUp':
    'Karty hráčů se rozdávají lícem nahoru; krupiér má jednu kartu skrytou, dokud všichni nedohrají.',
  'blackjack.rules.hitStand': 'Můžeš si brát další karty, nebo zůstat stát na tom, co máš.',
  'blackjack.rules.aces': 'Eso platí jedenáct, dokud se vejde, jinak jedna.',
  'blackjack.rules.blackjack': 'Eso s desítkovou kartou na prvních dvou kartách je blackjack.',
  'blackjack.rules.pays3to2': 'Blackjack se platí 3:2.',
  'blackjack.rules.pays6to5': 'Blackjack se platí 6:5.',
  'blackjack.rules.paysEven': 'Blackjack se platí jedna ku jedné.',
  'blackjack.rules.double': 'Na prvních dvou kartách můžeš zdvojit sázku a vzít si přesně jednu kartu.',
  'blackjack.rules.doubleAfterSplit': 'Zdvojit lze i ruku, která vznikla rozdělením.',
  'blackjack.rules.noDoubleAfterSplit': 'Ruku, která vznikla rozdělením, zdvojit nelze.',
  'blackjack.rules.split':
    'Dvě karty stejné hodnoty můžeš rozdělit na samostatné ruce, každou s vlastní sázkou — až {n}×, celkem na {hands} ruce.',
  'blackjack.rules.noSplit': 'U tohoto stolu se páry nedělí.',
  'blackjack.rules.splitAces':
    'Rozdělená esa dostanou po jedné kartě a stojí; jednadvacet z nich není blackjack.',
  'blackjack.rules.surrender':
    'První ruku můžeš vzdát za polovinu sázky, jakmile krupiér zkontroluje blackjack.',
  'blackjack.rules.noSurrender': 'U tohoto stolu se ruce vzdát nedají.',
  'blackjack.rules.dealerDraws': 'Krupiér dobírá do sedmnácti a pak stojí.',
  'blackjack.rules.hitsSoft17': 'Na sedmnáctku s esem si krupiér ještě bere.',
  'blackjack.rules.standsSoft17': 'Na sedmnáctce s esem krupiér stojí.',
  'blackjack.rules.dealerPeeks':
    'S esem nebo desítkou nahoře se krupiér podívá na blackjack dřív, než kdokoli hraje.',
  'blackjack.rules.insurance':
    'Proti krupiérovu esu se můžeš pojistit za polovinu sázky; při krupiérově blackjacku platí 2:1.',
  'blackjack.rules.noInsurance': 'Pojištění se u tohoto stolu nenabízí.',
  'blackjack.rules.rounds': 'Hraje se {n} kol.',
  'blackjack.rules.mostChipsWins': 'Zápas vyhrává ten, kdo má na konci nejvíc žetonů.',
  'blackjack.rules.bustedOut': 'Místo, které už nepokryje minimum {n} žetonů, zbytek zápasu nehraje.',

  'blackjack.zone.dealer': 'Krupiér',
  'blackjack.zone.box': 'Ruka',
  'blackjack.zone.yourBox': 'Tvoje ruka',
  'blackjack.zone.shoe': 'Bota',

  'blackjack.header.round': 'Kolo {n} z {of}',
  'blackjack.header.minBet': 'Minimum',
  'blackjack.header.decks': 'Balíčky',
  'blackjack.header.dealerTotal': 'Krupiér ukazuje {n}',
  'blackjack.header.dealerSoftTotal': 'Krupiér ukazuje měkkých {n}',

  'blackjack.seat.stack': 'Žetony',
  'blackjack.seat.bet': 'Sázka',
  'blackjack.seat.insurance': 'Pojištění',
  'blackjack.seat.total': 'Celkem',
  'blackjack.seat.softTotal': 'Měkkých',
  'blackjack.seat.out': 'Bez žetonů',

  'blackjack.prompt.placeBet': 'Vsaď si',
  'blackjack.prompt.insurance': 'Pojistíš se?',
  'blackjack.prompt.yourMove': 'Jsi na tahu',
  'blackjack.prompt.waitingFor': 'Čeká se na hráče {playerId}',
  'blackjack.prompt.betAmount': 'Sázka',

  'blackjack.quick.doubleMin': '2× Minimum',
  'blackjack.quick.allIn': 'All-in',
  'blackjack.offer.bet': 'Vsadit',
  'blackjack.offer.hit': 'Další kartu',
  'blackjack.offer.stand': 'Stát',
  'blackjack.offer.double': 'Zdvojit',
  'blackjack.offer.split': 'Rozdělit',
  'blackjack.offer.surrender': 'Vzdát',
  'blackjack.offer.insure': 'Pojistit se',
  'blackjack.offer.declineInsurance': 'Bez pojištění',

  'blackjack.fact.tableMinimum': 'minimum',
  'blackjack.fact.insuranceCost': 'za pojištění',
  'blackjack.fact.extraStake': 'k vsazení',
  'blackjack.fact.surrenderReturn': 'zpět',

  'blackjack.status.dealerBlackjack': 'Krupiér měl blackjack',
  'blackjack.status.dealerBust': 'Krupiér přetáhl na {n}',
  'blackjack.status.dealerStands': 'Krupiér stojí na {n}',

  'blackjack.round.name': 'Kolo',
  'blackjack.round.dealerTotal': 'Krupiér {n}',
  'blackjack.round.dealerBust': 'Krupiér přetáhl ({n})',
  'blackjack.round.dealerBlackjack': 'Krupiérův blackjack',
  'blackjack.round.outcome.blackjack': 'Blackjack',
  'blackjack.round.outcome.win': 'Výhra',
  'blackjack.round.outcome.push': 'Shoda',
  'blackjack.round.outcome.lose': 'Prohra',
  'blackjack.round.outcome.bust': 'Přetažení',
  'blackjack.round.outcome.surrender': 'Vzdáno',

  'blackjack.badge.inPlay': 'Na tahu',
  'blackjack.badge.doubled': 'Zdvojeno',
  'blackjack.badge.split': 'Rozděleno',
  'blackjack.badge.blackjack': 'Blackjack',
  'blackjack.badge.bust': 'Přetaženo',
  'blackjack.badge.won': 'Výhra',
  'blackjack.badge.push': 'Shoda',
  'blackjack.badge.lost': 'Prohra',
  'blackjack.badge.surrendered': 'Vzdáno',

  'blackjack.unit.chips': 'žetonů',

  // --- the settings screen --------------------------------------------------
  //
  // Hardcoded English until now, which was survivable while the only other
  // language was unreachable and faintly absurd once the language picker
  // itself lived on this screen: a player who cannot read "Settings" is
  // exactly the one who came here to change it.
  'settings.title': 'Nastavení',
  'settings.signedInAs': 'Přihlášen jako {username}',
  'settings.playingAsGuest': 'Hraješ jako {username} (host)',
  'settings.notSignedIn': 'Nejsi přihlášen — přihlas se nebo pokračuj jako host a hraj online.',
  'settings.subtitle': 'Jak vypadáš ty a jak stůl',
  'settings.face.heading': 'Tvoje tvář u stolu',
  'settings.face.account': 'Uloženo k tvému účtu, takže tě doprovodí i na jiné zařízení.',
  'settings.face.device': 'Uloženo v tomto zařízení. Přihlas se, aby šlo s tebou.',
  'settings.skin.heading': 'Vzhled stolu',
  'settings.language.heading': 'Jazyk',
  'settings.language.status': 'Uloženo v tomto zařízení.',
  'settings.language.auto': 'Automaticky',
  'settings.language.auto.now': 'Podle zařízení — nyní {language}',
  'settings.legal.heading': 'Drobným písmem',
  'settings.legal.status': 'S čím jsi hrou souhlasil a co se o tobě ukládá.',
  'settings.signIn': 'Přihlásit se',
  'settings.back': 'Zpět',

  // A translated interface over an English notice is a worse lie than an
  // English interface, so the document says so itself rather than letting the
  // surrounding screen imply the text was written for this reader.
  'legal.untranslated': 'Toto oznámení zatím nebylo přeloženo do tvého jazyka. Platí anglické znění níže.',

  // --- the navigation bar's screen titles ------------------------------------
  //
  // These sit in `app/_layout.tsx`, above every screen, and were the last
  // English left after the settings screen was translated — a German player
  // reading "Einstellungen" under a bar that said "Settings". A translated app
  // with an untranslated chrome bar looks like a bug, not like a limit.
  'nav.home': 'Zolik',
  'nav.emailSignIn': 'Přihlášení e-mailem',
  'nav.signingIn': 'Přihlašování',
  'nav.usernameSignIn': 'Přihlášení jménem',
  'nav.legacyAccount': 'Starý účet',
  'nav.guest': 'Host',
  'nav.account': 'Účet',
  'nav.games': 'Hry',
  'nav.table': 'Tvůj stůl',
  'nav.join': 'Připojit se ke stolu',
  // The screen a shared invite link lands on while it resolves the code.
  'nav.joining': 'Připojování',
  'nav.rules': 'Pravidla',
  'nav.match': 'Zápas',
  'nav.scoreTable': 'Tabulka skóre',
  'nav.stats': 'Statistiky',
  'nav.more': 'Další',

  // --- the account menu behind the face in the corner ----------------------
  'menu.label': 'Nabídka účtu',
  'menu.signedIn': 'Přihlášen',
  'menu.notSignedIn': 'Nepřihlášen',
  'menu.keepStats': 'aby ti zůstaly statistiky',
  'menu.signOut': 'Odhlásit se',
  'more.scoreTable': 'Offline tabulka skóre',
  'more.stats': 'Statistiky a žebříček',
  'more.needsAccount': 'přihlas se',
  'gate.title': 'Přihlas se, ať to můžeš použít',
  'gate.body':
    'Tabulky skóre a statistiky se ukládají k tvému účtu, takže jdou s tebou i na jiné zařízení. Host je nemá kam uložit.',

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
  'error.generic': 'To se nepovedlo',
  'error.signIn': 'Přihlášení selhalo',
  'error.login': 'Přihlášení selhalo',
  'error.register': 'Registrace selhala',
  'error.sendCode': 'Kód se nepodařilo odeslat',
  'error.badCode': 'Tenhle kód nefungoval',
  'error.rulesLoad': 'Pravidla se nepodařilo načíst',
  'error.createFailed': 'Vytvoření selhalo',
  'error.saveFailed': 'Uložení selhalo',
  'error.exportFailed': 'Export selhal',

  // --- the route that does not exist ----------------------------------------
  'notFound.title': 'Jejda!',
  'notFound.message': 'Tahle obrazovka neexistuje.',
  'notFound.home': 'Zpátky na úvod!',

  // --- signing in -----------------------------------------------------------
  'auth.login.subtitle': 'Měj svoje statistiky na všech zařízeních',
  'auth.login.continueWithEmail': 'Pokračovat e-mailem',
  'auth.login.usernameInstead': 'Přihlásit se raději jménem',
  'auth.email.title': 'Přihlášení e-mailem',
  'auth.email.subtitle': 'Pošleme ti jednorázový kód',
  'auth.email.address': 'E-mailová adresa',
  'auth.email.send': 'Poslat kód',
  'auth.email.codeTitle': 'Zadej kód',
  'auth.email.codePlaceholder': 'Šestimístný kód',
  'auth.email.differentAddress': 'Použít jinou adresu',
  'auth.email.sentTo': 'Odesláno na {email}',
  'auth.email.continue': 'Pokračovat',
  'auth.guest.title': 'Hra jako host',
  'auth.guest.subtitle': 'Účet není potřeba',
  'auth.guest.displayName': 'Zobrazované jméno',
  'auth.register.title': 'Vytvořit účet',
  'auth.register.username': 'Uživatelské jméno',
  'auth.register.email': 'E-mail (nepovinný)',
  'auth.register.password': 'Heslo',
  'auth.username.createAccount': 'Vytvořit účet se jménem a heslem',
  'auth.callback.signedIn': 'Přihlášeno.',

  // --- the account screen ---------------------------------------------------
  'account.signInPrompt': 'Přihlas se, ať můžeš spravovat účet.',
  'account.keepGames': 'Zachovat tyhle hry',
  'account.signedInWith': 'Přihlášen přes',
  'account.addMethod': 'Přidat způsob přihlášení',
  'account.usernameAndPassword': 'Jméno a heslo',
  'account.faceAndTable': 'Tvář a vzhled stolu',
  'account.refresh': 'Obnovit',
  'account.remove': 'Odebrat',

  // --- the main menu --------------------------------------------------------
  'home.subtitle': 'Kontinentální žolíky · {server}',
  'home.playingAs': 'Hraješ jako {name}',
  'home.signInPrompt': 'Přihlas se nebo pokračuj jako host, ať můžeš hrát online.',
  'home.statsAndLeaderboard': 'Statistiky a žebříček',
  'home.play': 'Hrát',
  'home.offlineScoreTable': 'Offline tabulka skóre',
  'home.signInToKeepStats': 'Přihlas se a zachovej si statistiky',
  'home.signOut': 'Odhlásit se',
  'home.continueAsGuest': 'Pokračovat jako host',
  // Someone playing as a guest, marked in a list of names.
  'home.guestSuffix': '(host)',

  // --- the waiting room on the main menu ------------------------------------
  //
  // Counts are whole phrases per count, not a number glued to a noun — the
  // same reason `countLabel` exists. One and many are the cases the interface
  // actually produces, so they are the cases that get their own wording.
  'waiting.checking': 'Koukáme, kdo je poblíž…',
  'waiting.youAreWaiting': 'Čekáš na hru',
  'waiting.pickedUp': 'Kdokoli, kdo otevře stůl, si tě může vzít — kód od tebe nikdo nepotřebuje.',
  'waiting.othersOne': 'Čeká i 1 další hráč',
  'waiting.othersMany': 'Čekají i další hráči: {n}',
  'waiting.oneWaiting': '1 hráč čeká na hru',
  'waiting.manyWaiting': 'Hráčů čekajících na hru: {n}',
  'waiting.adding': 'Přidáváme tě na čekací listinu…',
  'waiting.slowHint':
    'Pokud to za pár vteřin neskončí, zkontroluj, jestli je adresa serveru níže z tohohle zařízení dostupná.',
  'waiting.serverBusyDetail': 'Pokus {n}. Server právě nepřijímá nová spojení do čekárny.',
  'waiting.reconnecting': 'Spojení ztraceno — připojujeme znovu…',
  'waiting.reconnectingDetail':
    'Pokus {n}. Může se to stát, když se změnila síť tvého zařízení nebo se restartoval server.',
  'waiting.tryAgain': 'Zkusit hned znovu',
  'waiting.makeAvailable': 'Nabídnout se ke hře',
  'waiting.stop': 'Přestat čekat',
  'waiting.noneYet': 'Právě teď nikdo nečeká na hru. Zapiš se na listinu a budeš první, koho kdokoli uvidí.',
  'waiting.noOthersYet': 'Zatím nečeká nikdo další. Hostitelé tě stejně vidí a můžou tě pozvat.',
  'waiting.server': 'Server',
  'waiting.none': 'Právě teď nikdo nečeká. Kdo se nabídne v hlavní nabídce, objeví se tady.',

  // --- landing on a shared invite link --------------------------------------
  'join.missingCode': 'Tomuhle odkazu chybí kód stolu.',
  'join.staleLink': 'Požádej toho, kdo tě pozval, o nový odkaz, nebo se připoj kódem.',
  'join.enterCode': 'Zadat kód',
  'join.backToMenu': 'Zpátky do nabídky',
  'join.takingSeat': 'Sedáme si ke stolu…',
  'join.takingSeatAt': 'Sedáme si ke stolu {game}…',

  // --- the lobby ------------------------------------------------------------
  'lobby.games.subtitle': 'Všechno, co tenhle server umí nabídnout',
  'lobby.games.bots': 'Boti',
  'lobby.games.playBot': 'Hrát proti botovi',
  'lobby.games.playBots': 'Hrát proti botům: {n}',
  'lobby.games.openTable': 'Otevřít stůl',
  'lobby.games.players': 'Hráčů: {n}',
  'lobby.games.playerRange': 'Hráčů: {min}–{max}',
  'lobby.join.placeholder': 'Kód nebo odkaz s pozvánkou',
  'lobby.join.needCode': 'Zadej kód, odkaz nebo ID zápasu',
  'lobby.games.signInFirst': 'Nejdřív se přihlas',
  'lobby.join.action': 'Připojit se',
  'lobby.join.waitingTitle': 'Čekání na hostitele',
  // Two whole sentences rather than one with a swapped noun: "a game of {game}"
  // does not survive a language that inflects the game's name after "of".
  'lobby.join.joinedGame': 'Připojeno ke hře {game} — čeká se na start',
  'lobby.join.joinedTable': 'Připojeno ke stolu — čeká se na start',
  'lobby.table.addBot': 'Přidat bota',
  'lobby.table.side': 'Strana {n}',
  'lobby.table.shuffleSeats': 'Zamíchat místa',
  'lobby.table.moveSeatUp': 'Posunout {name} o místo nahoru',
  'lobby.table.moveSeatDown': 'Posunout {name} o místo dolů',
  'lobby.table.start': 'Začít',
  'lobby.table.waitingForHost': 'Čekáme, až hostitel začne…',

  // --- inviting someone to a table ------------------------------------------
  'invite.heading': 'Pozvat hráče',
  'invite.explain': 'Pošli tenhle odkaz. Kdo ho otevře, přistane u tohohle stolu — účet není potřeba.',
  'invite.noAddress': 'Tenhle server nemá nastavenou sdílitelnou adresu, použij tedy kód níže.',
  'invite.readOutCode': 'Nebo nadiktuj kód:',
  'invite.copy': 'Kopírovat odkaz',
  'invite.share': 'Sdílet odkaz',
  'invite.copied': 'Zkopírováno!',
  'invite.shared': 'Sdíleno',

  // --- the match screen -----------------------------------------------------
  'match.waitingForTable': 'Čekáme na stůl…',
  'match.waitingForPlayer': 'Čekáme na dalšího hráče…',
  'match.nobodyWon': 'Nikdo nevyhrál.',
  'match.youWon': 'Vyhrál jsi.',
  'match.finished': 'Tenhle zápas skončil.',
  'match.inProgress': 'Zápas běží — všechno je připojené a funguje normálně.',
  'match.connecting': 'Připojuji…',
  'match.abandonedTitle': 'Stůl odložen',
  'match.abandoned': 'K tomuhle stolu se nikdo nevrátil, tak byl odložen. Karty jsou přesně tam, kde jsi je nechal.',
  'match.resume': 'Pokračovat tam, kde jsi skončil',
  'match.resuming': 'Obnovuji stůl…',
  'match.controls': 'Ovládání',
  'match.over': 'Konec zápasu',
  'match.settingUp': 'Připravujeme…',
  'match.playAgain': 'Hrát znovu',
  'match.backToGames': 'Zpátky ke hrám',
  'match.table': 'Stůl',
  'match.opponents': 'Soupeři',
  // Marks which seat is the reader's own, in a list of seats.
  'match.youSuffix': '(ty)',
  // The winner line is composed from whole sentences rather than a name glued
  // to " won." — the verb agrees with the subject in most of these languages,
  // and a suffix cannot know that.
  'match.you': 'ty',
  'match.someoneWon': '{name} vyhrává.',
  'match.wonBy': 'Vyhrává {names}.',
  'match.pausedFor': 'Pozastaveno — čekáme, až se {name} znovu připojí.',
  'match.results': 'Výsledky',
  'match.players': 'Hráči',
  'match.toPlay': 'na tahu',

  // --- the offline score table ----------------------------------------------
  'scoring.namesHint': 'Jména oddělená čárkami (4–8 hráčů)',
  'scoring.newSession': 'Nová relace',
  // A worked example, not a sentence: the names are placeholders a translator
  // may localise, the shape `name:score,` is what the parser needs.
  'scoring.scoresPlaceholder': 'Alena:120,Bohuš:80,…',
  'scoring.saveRound': 'Uložit kolo',
  'scoring.export': 'Exportovat zápis',
  'scoring.formatHint': 'Formát bodů: Jméno:100,Jméno2:50',
  'scoring.nameCountError': 'Zadej 2–8 jmen hráčů oddělených čárkami',
  'scoring.session': 'Relace: {id}',
  'scoring.players': 'Hráči: {names}',
  'scoring.roundScores': 'Body kola {n}',
  'stats.loading': 'Načítání…',
  // The figures could not be fetched. `{reason}` is whatever the server or the
  // network said, which is not ours to translate — the frame around it is.
  'stats.unavailable': '(nedostupné: {reason})',

  // --- statistics -----------------------------------------------------------
  'stats.title': 'Statistiky a žebříček',
  'stats.yours': 'Tvoje statistiky',
  'stats.leaderboard': 'Žebříček',

  // --- a player's lifetime record, shown beside a finished match ------------
  'record.title': 'Tvoje bilance',
  'record.guest':
    'Hraješ jako host, takže se žádná bilance nevede. Přihlas se a hry, které jsi na tomhle zařízení už odehrál — včetně téhle — zůstanou u tvého účtu.',
  'record.signInToKeep': 'Přihlásit se a zachovat je',
  'record.failed': 'Tvoji bilanci se teď nepodařilo načíst. Zápas je bezpečně zaznamenaný.',
  'record.loading': 'Načítání…',
  'record.played': 'Odehráno',
  'record.won': 'Výhry',
  'record.lost': 'Prohry',
  'record.winRate': 'Úspěšnost',
  'record.streak': 'Série',
  'record.atThisGame': 'V téhle hře',
  // A streak is a count with a noun, so it is a whole phrase per count for
  // the same reason `countLabel` is: "1 win" / "2 wins" is an English rule,
  // and most of these languages do not share it.
  'record.streakNone': '—',
  'record.streakWinOne': '1 výhra',
  'record.streakWinMany': 'Výhry: {n}',
  'record.streakLossOne': '1 prohra',
  'record.streakLossMany': 'Prohry: {n}',

  // --- moving cards ---------------------------------------------------------
  'hand.dragHint': 'Táhni kartu podél vějíře, když ji chceš přerovnat, nebo na stůl, když ji chceš zahrát',
  'hand.moveLeft': 'Doleva',
  'hand.moveRight': 'Doprava',
  'zone.collapseGroup': 'Sbalit tuhle skupinu',
  'zone.expandGroup': 'Ukázat všechny karty téhle skupiny',
  'zone.dropHere': 'Polož sem',
  'offer.pickCards': 'vyber karty pro místo, na které jsi klepl',
  'offer.ambiguous': 'tohle patří na víc míst — vyber na stole',

  // --- the build footer -----------------------------------------------------
  'build.app': 'aplikace',
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
  'option.pauseBetweenRounds': 'Pauza mezi koly',
  'choice.pauseBetweenRounds.1': 'Pauza',
  'choice.pauseBetweenRounds.0': 'Hrát rovnou dál',
  'option.botSkill': 'Soupeři',
  'choice.botSkill.0': 'Smíšení',
  'choice.botSkill.1': 'Lehcí',
  'choice.botSkill.2': 'Střední',
  'choice.botSkill.3': 'Těžcí',
  'option.initialMeldMinimum': 'Hodnota otevření',
  'choice.initialMeldMinimum.0': 'Bez minima',
  'option.discardDrawMinRound': 'Braní z odhazovacího balíčku',
  'choice.discardDrawMinRound.0': 'Otevřené',
  'choice.discardDrawMinRound.2': 'Od kola 2',
  'choice.discardDrawMinRound.3': 'Od kola 3',
  'option.requireCleanRun': 'Postupka bez žolíka',
  'choice.requireCleanRun.1': 'Povinná',
  'choice.requireCleanRun.0': 'Ne',
  'option.jokerReclaimMustPlay': 'Vykoupený žolík',
  'choice.jokerReclaimMustPlay.1': 'Zahrát ve stejném tahu',
  'choice.jokerReclaimMustPlay.0': 'Může zůstat v ruce',
  'option.dealStarter': 'Kdo vynáší',
  'choice.dealStarter.0': 'Po řadě',
  'choice.dealStarter.1': 'Vynáší vítěz',
  'variation.prsi.classic': 'Klasické',
  'option.handSize': 'Rozdané karty',
  'variation.canasta.classic': 'Klasická',
  'variation.canasta.modern_american': 'Modern American',
  'variation.canasta.samba': 'Samba',
  'option.targetScore': 'Cílové skóre',
  'option.canastasToGoOut': 'Canasty k vyjití',
  'variation.holdem.freezeout': 'Freezeout',
  'variation.holdem.timed': 'Pevný počet rozdání',
  'option.startingStack': 'Startovní žetony',
  'option.bigBlind': 'Velký blind',
  'option.handLimit': 'Rozdání',
  'choice.handLimit.0': 'Dokud nezbude jedno místo',
  'variation.ginrummy.standard': 'Standardní',
  'option.knockLimit': 'Limit klepnutí',
  'choice.knockLimit.0': 'Oklahoma (určí ho otočená karta)',
  'option.bigGin': 'Big gin',
  'choice.bigGin.0': 'Ne',
  'choice.bigGin.1': 'Ano (+25)',
  'option.lineBonuses': 'Bonusy v zúčtování',
  'choice.lineBonuses.1': 'Ano',
  'choice.lineBonuses.0': 'Ne',
  'variation.rummytiles.standard': 'Standardní',
  'choice.targetScore.0': 'Bez cíle',
  // Two games qualify a number the others mean plainly: Canasta's 500 is a
  // short game, Gin Rummy's and Rummy Tiles' 500 is just 500; Hold'em's
  // starting 200 is a short stack, Blackjack's 200 is just 200. Module-scoped
  // so the qualifier travels with the game that means it — a bare key would
  // hand the note to games it is false for. `scripts/check-server-labels.js`
  // is what finds these; both were shipping as English on a Czech screen.
  'choice.canasta.targetScore.500': '500 (krátká)',
  'choice.holdem.startingStack.200': '200 (krátká)',
  'option.roundLimit': 'Limit kol',
  'choice.roundLimit.0': 'Bez limitu',
  'option.poolExhaustion': 'Když se banka vyčerpá',
  'choice.poolExhaustion.1': 'Kolo vyhrává nejnižší ruka',
  'choice.poolExhaustion.0': 'Kolo nevyhrává nikdo',
  'variation.blackjack.single': 'Jeden balíček',
  'option.minBet': 'Minimum stolu',
  'option.rounds': 'Kola',
  'option.decks': 'Balíčky',
  'option.dealerHitsSoft17': 'Krupiér při měkké 17',
  'choice.dealerHitsSoft17.0': 'Zůstává',
  'choice.dealerHitsSoft17.1': 'Táhne',
  'option.blackjackPays': 'Blackjack platí',
  'choice.blackjackPays.100': 'Jedna ku jedné',
  'option.maxSplits': 'Rozdělení',
  'choice.maxSplits.0': 'Bez rozdělení',
  'choice.maxSplits.1': 'Jednou (dvě rozdání)',
  'choice.maxSplits.3': 'Třikrát (čtyři rozdání)',
  'option.doubleAfterSplit': 'Zdvojení po rozdělení',
  'choice.doubleAfterSplit.1': 'Povoleno',
  'choice.doubleAfterSplit.0': 'Nepovoleno',
  'option.surrender': 'Vzdání',
  'choice.surrender.0': 'Ne',
  'choice.surrender.1': 'Pozdní vzdání',
  'option.insurance': 'Pojištění',
  'choice.insurance.1': 'Nabízeno',
  'choice.insurance.0': 'Nenabízeno',

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
  'verb.add': 'Přidat',
  'verb.bet': 'Vsadit',
  'verb.call': 'Dorovnat',
  'verb.check': 'Čekám',
  'verb.commit': 'Hotovo',
  'verb.continue': 'Pokračovat',
  'verb.decline_insurance': 'Bez pojištění',
  'verb.discard': 'Odhodit',
  'verb.double': 'Zdvojit',
  'verb.draw': 'Líznout',
  'verb.finish_layoff': 'Hotovo s přikládáním',
  'verb.fold': 'Složit',
  'verb.hit': 'Další kartu',
  'verb.insure': 'Pojistit se',
  'verb.knock': 'Klepnout',
  'verb.lay_meld': 'Kombinace',
  'verb.lay_off': 'Přiložit',
  'verb.pass': 'Nechat',
  'verb.place': 'Položit',
  'verb.play_card': 'Zahrát',
  'verb.raise': 'Zvýšit',
  'verb.reset_turn': 'Vrátit tah',
  'verb.split': 'Rozdělit',
  'verb.stand': 'Stát',
  'verb.surrender': 'Vzdát',
  'verb.swap_joker': 'Vyměnit žolíka',
  'verb.take': 'Vzít',
  'verb.take_pile': 'Vzít z balíčku',
  // Declared by the server and listed in `serverKeys.json`, but never worded.
  'verb.takePileFromHand': 'Vzít kupu do ruky',
  'verb.takePileOntoMeld': 'Vzít kupu na kombinaci',
  'verb.takeTopForSequence': 'Vzít vrchní kartu do postupky',
  'verb.undoDraw': 'Vrátit líznutí',
  'verb.undoLayOff': 'Vrátit přiložení',
  'verb.undoMeld': 'Vrátit kombinaci',
  'verb.undoTakePile': 'Vrátit vzetí balíčku',
  'verb.undoTurn': 'Vrátit tah',

  // --- suits, spelled out ---------------------------------------------------
  //
  // Read aloud by a screen reader and shown where a pip would not fit. The
  // pips themselves are drawn, not written, so these are the only place the
  // suit is ever a word.
  'suit.C': 'Kříže',
  'suit.D': 'Káry',
  'suit.H': 'Srdce',
  'suit.S': 'Piky',

  // --- counters the board prints beside a number ----------------------------
  'canasta.seat.notOpened': 'Ještě nevyložil',
  'canasta.unit.points': 'bodů',
  'ginrummy.unit.points': 'bodů',
  'holdem.seat.dealer': 'Rozdávající',
  'holdem.seat.folded': 'Složil',
  'holdem.seat.allIn': 'All in',
  'holdem.seat.out': 'Vyřazen',
  'holdem.unit.chips': 'žetonů',
  'prsi.unit.cardsLeft': 'zbývá karet',
  'rummytiles.prompt.initialMeld': 'Tvoje první vyložení musí mít hodnotu {n} bodů.',
  'rummytiles.unit.points': 'bodů',
  'zolik.unit.penalty': 'trestné body',
  'header.pileFrozen': 'Kupa zamrzlá',
  // The module-less form of `ginrummy.prompt.yourTurnDraw`: Žolíky and Canasta
  // send the prompt without a module prefix. Neither the Go constants nor
  // `serverKeys.json` list it — the sweep is what found it.
  'prompt.yourTurnDraw': 'Lízni kartu',
  'prompt.yourTurnMeld': 'Vyložte, pokud můžete, pak odhoďte',
};
