/**
 * Polish. Remik vocabulary: grupa for a set, sekwens for a run, układ for a meld, talia for the stock. Counted phrases beyond three fall back to a genitive-plural form, which is why contract.sets.n reads "Grupy: {n}" rather than gluing a number to a noun.
 */

export const pl: Record<string, string> = {
  // --- engine error codes (rules.RulesErrorCode) ---------------------------
  'err.NOT_YOUR_TURN': 'To nie twoja kolej',
  'err.WRONG_PHASE': 'W tej chwili niedostępne',
  'err.MUST_DRAW_FIRST': 'Dobierz kartę, zanim wyłożysz',
  'err.GAME_SUSPENDED': 'Gra jest wstrzymana',
  'err.GAME_NOT_ACTIVE': 'Gra nie jest w toku',
  // --- runtime refusals, which are not the rules saying no -----------------
  //
  // Both were reachable long before they were worded, and a player who met
  // one was shown the code in capitals or, worse, nothing at all: an action
  // sent while the socket was down simply vanished, which reads as a legal
  // move being silently refused.
  'err.MATCH_NOT_ACTIVE': 'Stół jest wstrzymany — czekamy, aż gracz wróci',
  'err.NOT_CONNECTED': 'Brak połączenia ze stołem — trwa łączenie, spróbuj potem ponownie',
  // Why the "start the next round" control is greyed out. Without wording the
  // shell printed the code itself, so a player agreeing to go on was answered
  // with ALREADY_READY in capitals.
  'err.ALREADY_READY': 'Jesteś gotowy',
  'err.NOT_BETWEEN_ROUNDS': 'Runda wciąż trwa',
  'err.NOT_AT_THIS_TABLE': 'Nie siedzisz przy tym stole',
  // Bringing a table back that the sweeper set aside. Refusals a player
  // can actually provoke: a link to a table that is gone, one that was
  // already picked up in another tab, and a table with other people at it.
  'err.MATCH_MOVED_ON': 'Stół poszedł dalej — odśwież stronę',
  'err.MATCH_NOT_ABANDONED': 'Ten stół nie czeka na wznowienie',
  'err.MATCH_NOT_FOUND': 'Ten stół już nie istnieje',
  'err.TABLE_HAS_OTHER_PLAYERS': 'Wznowić można tylko stół, przy którym wszyscy pozostali to boty',
  'err.DISCARD_LOCKED': 'Stos odrzuconych jest na razie zablokowany',
  'err.DISCARD_PILE_EMPTY': 'Stos odrzuconych jest pusty',
  'err.NO_CARDS_LEFT': 'Nie ma już kart do dobrania',
  'err.ROUND_REQ_NOT_MET': 'Najpierw wyłóż własne otwarcie',
  'err.NEED_CLEAN_RUN': 'Potrzebujesz na stole sekwensu bez jokera, żeby liczyć się jako wyłożony',
  'err.INCOMPLETE_INITIAL_MELD': 'Dokończ wykładanie albo je cofnij, zanim odrzucisz kartę',
  'err.DISCARD_CARD_NOT_MELDED': 'Podniesiona karta musi trafić do twojego układu',
  'err.JOKER_DISCARD_FORBIDDEN': 'Jokera nie można odrzucić',
  'err.NOTHING_TO_UNDO': 'Nie ma czego cofać',
  'err.NO_JOKER_IN_MELD': 'W tym układzie nie ma jokera',
  'err.JOKER_SWAP_MISMATCH': 'Ta karta nie zajmuje miejsca jokera',
  'err.RECLAIMED_JOKER_NOT_MELDED': 'Joker zdjęty ze stołu musi zostać zagrany w układ w tej kolejce',
  'err.RUN_TOO_LONG': 'Ten sekwens ma już pełną długość',
  'err.WRONG_RUN_END': 'Ta karta przedłuża drugi koniec sekwensu',
  'err.INVALID_MELD': 'Żadna karta z twojej ręki tu nie pasuje',
  'err.CARD_NOT_IN_HAND': 'Nie masz tej karty na ręce',
  'err.MELD_BELOW_MINIMUM': 'Twoim układom wciąż brakuje punktów do wyłożenia',
  'err.MELD_NO_CONTRIBUTION': 'Ten układ nie realizuje twojego wymogu',
  'err.TOO_MANY_WILDS': 'Za dużo jokerów w tym układzie',
  'err.ADJACENT_WILDS': 'Dwa jokery nie mogą leżeć obok siebie',
  'err.ACE_BRIDGE': 'As nie może łączyć króla z dwójką',

  // --- contract phrasing ---------------------------------------------------
  // Built from the structured contract the server sends (sets/runs counts),
  // never from a deal-number lookup table.
  // Whole phrases per count rather than a number plus a pluralised noun:
  // Czech inflects the noun by count (skupina / skupiny / skupin) in a way no
  // "add an s" helper survives, and the counts that actually occur are 1-3.
  // A count outside that falls back to contract.sets.n.
  'contract.sets.1': 'Jedna grupa',
  'contract.sets.2': 'Dwie grupy',
  'contract.sets.3': 'Trzy grupy',
  'contract.sets.n': 'Grupy: {n}',
  'contract.runs.1': 'Jeden sekwens',
  'contract.runs.2': 'Dwa sekwensy',
  'contract.runs.3': 'Trzy sekwensy',
  'contract.runs.n': 'Sekwensy: {n}',
  'contract.any': 'Dowolny poprawny układ',
  'contract.cleanRunOnly':
    'Dowolne połączenie grup i sekwensów — co najmniej jeden sekwens musi być bez jokera',
  'contract.cleanRunSuffix': '{base} — jeden sekwens musi być bez jokera',

  // --- written rules ---------------------------------------------------------
  // Full-sentence keys a "see the rules" screen renders, one per module,
  // resolved server-side against the table's actual variation and options —
  // see internal/module/rules.go and each game's rules.go. Shared section
  // titles first, then one block per module in the order the four games were
  // added.
  'zolik.rules.section.goal': 'Cel',
  'zolik.rules.section.setup': 'Przygotowanie',
  'zolik.rules.section.turn': 'Twoja kolej',
  'zolik.rules.section.melding': 'Wykładanie',
  'zolik.rules.section.end': 'Jak kończy się mecz',
  'zolik.rules.goal':
    'Bądź pierwszym, który opróżni rękę, wykładając poprawne grupy i sekwensy — i zbierz przy tym jak najmniej punktów karnych w kartach, które trzymasz, gdy ktoś inny wychodzi.',
  'zolik.rules.deal': 'Każdy gracz dostaje {n} kart.',
  'zolik.rules.meldShapes':
    'Grupa to {set}+ kart tej samej wartości; sekwens to {run}+ kolejnych kart w tym samym kolorze.',
  'zolik.rules.turn.draw': 'W swojej kolejce dobierz jedną kartę — z talii albo ze stosu odrzuconych.',
  'zolik.rules.pickup.topOnly': 'Ze stosu odrzuconych można wziąć tylko wierzchnią kartę.',
  'zolik.rules.pickup.anyFromPile':
    'Ze stosu odrzuconych można wziąć dowolną kartę wraz ze wszystkim, co leży nad nią.',
  'zolik.rules.pickup.locked': 'Ze stosu odrzuconych nie wolno dobierać przed rundą {n}.',
  'zolik.rules.pickup.open': 'Stos odrzuconych jest otwarty od pierwszej rundy.',
  'zolik.rules.turn.discard': 'Zakończ kolejkę, odrzucając jedną kartę.',
  'zolik.rules.jokers.restricted':
    'Jokera nigdy nie wolno odrzucić — chyba że jest dokładnie tą kartą, która opróżnia ci rękę.',
  'zolik.rules.lead.rotate':
    'Wyjście przesuwa się o jedno miejsce co rozdanie, niezależnie od tego, kto wygrał.',
  'zolik.rules.lead.winner': 'Kto wychodzi, ten zaczyna następne rozdanie.',
  'zolik.rules.meldFloor.on':
    'Twoje pierwsze wyłożenie musi dać łącznie co najmniej {n} punktów naturalnych, żebyś był wyłożony.',
  'zolik.rules.meldFloor.off': 'Pierwsze wyłożenie nie ma minimalnej wartości punktowej.',
  'zolik.rules.cleanRun.on':
    'Co najmniej jeden z twoich sekwensów musi być całkowicie bez jokera, żebyś liczył się jako wyłożony.',
  'zolik.rules.cleanRun.off':
    'Twoje sekwensy mogą swobodnie korzystać z jokerów — żaden nie musi być bez nich.',
  'zolik.rules.contracts.rotating':
    'Mecz trwa {n} rozdań, a każde rozdanie wymaga własnego zestawu grup i sekwensów.',
  'zolik.rules.contracts.static':
    'Każde rozdanie wymaga tego samego zestawu: {sets} grup i {runs} sekwensów.',
  'zolik.rules.end.afterDeals': 'Mecz kończy się po {n} rozdaniach.',
  'zolik.rules.end.atScore': 'Rozdaje się dalej, aż ktoś osiągnie {n} punktów — wtedy koniec.',

  'prsi.rules.section.goal': 'Cel',
  'prsi.rules.section.setup': 'Przygotowanie',
  'prsi.rules.section.turn': 'Twoja kolej',
  'prsi.rules.section.special': 'Karty specjalne',
  'prsi.rules.section.end': 'Jak kończy się mecz',
  'prsi.rules.goal': 'Bądź pierwszym, który zagra wszystkie karty z ręki.',
  'prsi.rules.deck': 'Gra się talią {value} kart (od siódemki wzwyż).',
  'prsi.rules.deal': 'Każdy gracz zaczyna z {n} kartami.',
  'prsi.rules.turn.match':
    'Zagraj kartę zgodną z kolorem albo wartością wierzchniej karty — albo dobierz, jeśli nie możesz.',
  'prsi.rules.turn.draw': 'Dobranie kończy twoją kolejkę bez zagrania.',
  'prsi.rules.sevens': 'Zagraj 7, a następny gracz dobiera dwie karty — chyba że odpowie własną siódemką.',
  'prsi.rules.aces': 'Zagraj asa, a następny gracz traci kolejkę.',
  'prsi.rules.queens': 'Zagraj damę i podaj kolor, który obowiązuje dalej.',
  'prsi.rules.end': 'Mecz kończy się w chwili, gdy czyjaś ręka jest pusta.',

  'canasta.rules.section.goal': 'Cel',
  'canasta.rules.section.setup': 'Przygotowanie',
  'canasta.rules.section.melding': 'Wykładanie',
  'canasta.rules.section.end': 'Jak kończy się mecz',
  'canasta.rules.goal': 'Gra się w parach; pierwsza strona, która osiągnie {n} punktów, wygrywa mecz.',
  'canasta.rules.deck': 'Gra się {value} kartami — {decks} talie plus jokery.',
  'canasta.rules.deal': 'Każdy gracz dostaje {n} kart.',
  'canasta.rules.drawCount': 'Na początku tury dobierasz {n} karty.',
  'canasta.rules.redThrees':
    'Czerwoną trójkę z ręki pokazuje się od razu i liczy jako premia — chyba że twoja strona nigdy nie skompletuje canasty, wtedy liczy się przeciwko tobie.',
  'canasta.rules.canasta': 'Canasta to układ {n} lub więcej kart tej samej wartości.',
  'canasta.rules.sequences': 'Układ może być też sekwensem: trzy lub więcej kart tego samego koloru po kolei, nigdy z jokerem w środku.',
  'canasta.rules.samba': 'Sekwens z siedmiu kart to samba warta {n} punktów.',
  'canasta.rules.pileAlwaysFrozen': 'Stos odrzuconych jest zamrożony przez całe rozdanie: aby go wziąć, musisz dołożyć do wierzchniej karty dwie naturalne karty z ręki.',
  'canasta.rules.meldFloorBands':
    'Twoje pierwsze wyłożenie musi osiągnąć minimum punktowe rosnące wraz z wynikiem: {negative} poniżej zera, {low} do 1500, {mid} do 3000, {high} powyżej.',
  'canasta.rules.meldFloorBandsFive': 'Twój pierwszy układ musi osiągnąć minimum punktowe rosnące wraz z wynikiem: {negative} poniżej zera, {low} do 1500, {mid} do 3000, {high} do 7000, powyżej {top}.',
  'canasta.rules.oneCanastaToGoOut': 'Jedna skompletowana canasta wystarczy, by twoja strona mogła wyjść.',
  'canasta.rules.twoCanastasToGoOut':
    'Twoja strona potrzebuje dwóch skompletowanych canast, zanim będzie mogła wyjść.',
  'canasta.rules.end': 'Rozdaje się dalej, aż jedna strona przekroczy {n} punktów — wtedy mecz się kończy.',

  'holdem.rules.section.goal': 'Cel',
  'holdem.rules.section.setup': 'Przygotowanie',
  'holdem.rules.section.betting': 'Licytacja',
  'holdem.rules.section.end': 'Jak kończy się mecz',
  'holdem.rules.goal':
    'Wygrywaj żetony, mając najlepszy układ przy odkryciu kart albo zostając jedynym graczem w rozdaniu.',
  'holdem.rules.stack': 'Każde miejsce zaczyna z {n} żetonami.',
  'holdem.rules.blinds': 'Mała ciemna to {sb}, duża ciemna {bb}; obie wpłaca się przed rozdaniem kart.',
  'holdem.rules.streets': 'Licytuje się w czterech rundach — przed flopem oraz po flopie, turnie i riverze.',
  'holdem.rules.showdown':
    'Gracze wciąż w rozdaniu odkrywają karty; najlepszy pięciokartowy układ bierze pulę.',
  'holdem.rules.noLimit': 'Bez limitu — każdy zakład może sięgać całego twojego stosu.',
  'holdem.rules.lastPlayerStanding': 'Gra się, aż jedno miejsce zgromadzi wszystkie żetony.',
  'holdem.rules.mostChipsWins': 'Kto ma najwięcej żetonów, gdy gra się kończy, wygrywa mecz.',
  'holdem.rules.handLimit': 'Gra kończy się po {n} rozdaniach.',

  // --- header --------------------------------------------------------------
  'header.deal': 'Rozdanie {n}',
  'header.gameOf': 'Gra {n} z {total}',
  'header.gameOfWithContract': 'Gra {n} z {total}: {contract}',

  // --- meld preview --------------------------------------------------------
  'preview.validSet': 'Poprawna grupa',
  'preview.validRun': 'Poprawny sekwens',
  'preview.validMeld': 'Poprawny układ',
  'preview.notYet': 'To jeszcze nie układ',
  'preview.points': '{shape} · {n} pkt',
  'preview.pointsWithLaid': '{shape} · {n} + {laid} już wyłożone = {total} pkt',
  'preview.meetsFloor': '{line} (osiąga {n} ✓)',
  'preview.needsFloor': '{line} (wymaga {n} ✗)',
  'preview.becauseOf': '{line} — {reason}',

  // --- discard -------------------------------------------------------------
  // A discard with a meld staged lays that meld first; this is what the
  // player is told when the server refuses it and the whole move is rolled
  // back. {reason} is the engine's own words for the refusal.
  'discard.meldRejected': '{reason} — nic nie odrzucono, twoje karty wciąż czekają.',

  // --- what the current selection would send -------------------------------
  // The client's own reason a control is not ready, next to the engine's own
  // `err.*` reasons rather than a second kind of message — a control greyed
  // out for "not your turn" and one greyed out for "you picked two, this
  // takes one" should read as the same kind of thing, not one of them looking
  // broken. See `fits` in `src/lib/drops.ts`.
  'sel.tooMany.1': 'Wybierz tylko jedną kartę',
  'sel.tooMany.n': 'Wybierz najwyżej {n} kart',
  'sel.needMore': 'Wybierz karty: {n}',
  'sel.notThese': 'Te karty nie mogą tu trafić',
  'sel.needsCompany': 'Ta karta potrzebuje sąsiednich',

  // --- what a module says happened -----------------------------------------
  // The first module-sent keys in this bundle, and deliberately few. Anything
  // a module sends is still legible without an entry here — `humanise` turns
  // `holdem.seat.stack` into "Stack" — so a key earns a line only when that
  // fallback loses something. These do: what each of them means lives in its
  // *params* (who won, how much, whose cards), and a key on its own has
  // nowhere to put them. Without the entry the player read "Winner", full
  // stop, at the end of a match they had just won.
  'status.winner': 'Wygrywa {winners}',
  'holdem.status.pot': '{winners} wygrywa {amount} układem {hand}',
  'holdem.status.potUncontested': '{winners} wygrywa {amount} — wszyscy inni spasowali',
  'holdem.status.shown': '{playerId} pokazał {value}',
  'holdem.prompt.waitingFor': 'Czekamy na {playerId}',

  // How a rummy scoreboard was ordered. Both earn a line for the same reason
  // as the keys above: the number lives in the params, and `humanise` would
  // render "Deals Won" with nothing after it.
  'zolik.standing.dealsWon': 'Wygrane rozdania: {n}',
  'zolik.standing.inHand': 'Na ręce: {n}',

  // --- rounds ---------------------------------------------------------------
  // What a round is called, and what one did. Each of these carries its number
  // in the params, so `humanise` alone would render a label with nothing after
  // it.
  // An instruction, not a heading. "Next round" named the thing rather than
  // asking for it, and sat above "Waiting for 1" — which reads as though the
  // player is the one waiting, when the table is waiting for them.
  'round.continue': 'Zacznij następną rundę',

  // --- the flash that announces a round or a match ending ------------------
  //
  // One line and one number, held for about two seconds. Worded generically on
  // purpose: which *kind* of ending it was is the module's own fact, printed
  // underneath, and a client that phrased "went gin" itself would be a client
  // that knows a game.
  'flash.roundWon': '{winners} bierze rundę',
  'flash.roundWonYou': 'Bierzesz rundę',
  'flash.roundDrawn': 'Nikt nie bierze rundy',
  'flash.matchOver': 'Koniec meczu',
  'flash.matchWon': '{winners} wygrywa',
  'flash.matchWonYou': 'Wygrywasz',
  'flash.matchDrawn': 'Nikt nie wygrywa',
  'flash.nowOn': 'teraz {total}',

  'zolik.round.deal': 'Rozdanie',
  'zolik.round.cleanRun': 'Jeden sekwens musi być bez jokera',
  'canasta.round.deal': 'Rozdanie',
  'canasta.round.concealed': 'Wyjście z zakrytą ręką',
  'canasta.round.exhausted': 'Talia się skończyła',
  'canasta.round.meldCards': 'Wyłożone karty: {n}',
  'canasta.round.canastas': 'Canasty: {n}',
  'canasta.round.redThrees': 'Czerwone trójki: {n}',
  'canasta.round.goingOut': 'Wyjście: {n}',
  'canasta.round.inHand': 'Zostało na ręce: {n}',
  'holdem.round.hand': 'Rozdanie',
  'holdem.round.pot': 'Pula {n}',
  'holdem.round.uncontested': 'Wszyscy inni spasowali',
  'seat.ready': 'Gotowy',
  'zolik.seat.contractMet': 'Kontrakt spełniony',
  'results.you': '(ty)',



  // --- refusals the other three games raise ---------------------------------
  // Worded here for the same reason Žolíky's are: reasonText falls back to the
  // code, and OfferBar hands it the code as that fallback, so an unworded
  // refusal reaches a player as PILE_FROZEN in capitals. serverKeys.test.ts
  // now fails the build rather than letting the next one through.
  'err.SET_TOO_LARGE': 'Grupa ma już wszystkie cztery kolory',
  'err.DISCARD_TAKEN_CARD_FORBIDDEN':
    'Nie możesz odrzucić karty, którą właśnie wziąłeś — zagraj ją albo zatrzymaj',
  'err.CARD_DOES_NOT_FIT': 'Ta karta nie zgadza się ani kolorem, ani wartością',
  'err.SUIT_REQUIRED': 'Podaj kolor, który obowiązuje dalej',
  'err.MUST_ANSWER_DRAW_OR_TAKE': 'Odpowiedz siódemką albo weź karty',
  'err.NOTHING_TO_DRAW': 'Nie ma już czego dobierać',
  'err.PILE_EMPTY': 'Stos jest pusty',
  'err.PILE_BLOCKED': 'Stos jest zablokowany — na wierzchu leży czarna trójka',
  'err.PILE_FROZEN': 'Stos jest zamrożony — potrzebujesz dwóch naturalnych kart o wartości wierzchniej karty',
  'err.TOP_CARD_UNUSABLE': 'Nie możesz użyć wierzchniej karty',
  'err.MELD_CLOSED': 'Ten układ jest kompletny i zamknięty',
  'err.MELD_TOO_SMALL': 'Układ potrzebuje więcej kart',
  'err.MELD_TOO_LARGE': 'Ten układ nie przyjmie już żadnej karty',
  'err.MELD_MIXED_RANKS': 'Wszystkie karty w układzie muszą mieć tę samą wartość',
  'err.SEQUENCE_NO_WILDS': 'Sekwens nie może zawierać jokerów',
  'err.SEQUENCE_NEEDS_ONE_SUIT': 'Wszystkie karty sekwensu muszą być w tym samym kolorze',
  'err.RUN_NOT_CONSECUTIVE': 'Sekwens musi iść po kolei, bez przerw',
  'err.NOT_ENOUGH_NATURALS': 'Układ potrzebuje więcej kart naturalnych niż jokerów',
  'err.RANK_ALREADY_MELDED': 'Twoja strona ma już układ tej wartości',
  'err.NOT_YOUR_MELD': 'Ten układ należy do przeciwnej strony',
  'err.NO_SUCH_MELD': 'Tego układu nie ma na stole',
  'err.CANNOT_MELD_THREE': 'Trójek nigdy się nie wykłada',
  'err.CANNOT_DISCARD_RED_THREE': 'Czerwonej trójki nie można odrzucić',
  'err.MUST_KEEP_A_CARD': 'Zatrzymaj co najmniej jedną kartę — tak nie opróżnisz ręki',
  'err.MUST_MELD_FIRST': 'Najpierw wyłóż otwarcie swojej strony',
  'err.INITIAL_MELD_NOT_MET': 'Twojemu pierwszemu wyłożeniu wciąż brakuje punktów',
  'err.CANNOT_GO_OUT_YET': 'Twoja strona potrzebuje skompletowanej canasty, zanim będzie mogła wyjść',
  'err.NOTHING_TO_CALL': 'Nie ma zakładu do sprawdzenia',
  'err.CANNOT_CHECK': 'Nie możesz czekać — jest zakład do odpowiedzi',
  'err.CANNOT_RAISE': 'Tutaj nie możesz podbić',
  'err.RAISE_TOO_SMALL': 'Podbicie musi być co najmniej równe poprzedniemu',
  'err.NOT_ENOUGH_CHIPS': 'Nie masz tylu żetonów',
  'err.AMOUNT_REQUIRED': 'Podaj ile',
  'err.AMOUNT_NOT_A_NUMBER': 'Ta kwota nie jest liczbą',
  'err.SEAT_NOT_IN_HAND': 'Nie bierzesz udziału w tym rozdaniu',
  'err.WRONG_RANK': 'Ta karta ma do tego złą wartość',
  'err.MATCH_FULL': 'Stół jest pełny',
  'err.MATCH_ALREADY_STARTED': 'Mecz już się rozpoczął',
  'err.TOO_FEW_PLAYERS': 'Jeszcze za mało graczy',
  'err.WRONG_PLAYER_COUNT': 'W tę grę nie da się grać w takim składzie',
  'err.NOT_THE_HOST': 'Może to zrobić tylko gospodarz',
  'err.BAD_SEATING': 'Ta kolejność miejsc nie zgadza się z tym, kto jest przy stole',
  'err.NO_LONGER_WAITING': 'Stół już nie czeka',
  'err.WAITING_ROOM_UNAVAILABLE': 'Poczekalnia jest niedostępna',
  'err.SERVER_BUSY': 'Serwer jest w tej chwili przeciążony — spróbuj za moment',


  // --- rules the engine has always enforced and never stated ----------------
  // Every one of these was found by the guardrail rather than by review: the
  // validator could refuse a player for it, and no sentence anywhere said so.
  // See server/internal/zolikmod/ruleindex_test.go.
  'zolik.rules.section.layoff': 'Dokładanie do układów',
  'zolik.rules.pickup.obligation':
    'Zanim będziesz wyłożony, karta wzięta ze stosu odrzuconych musi trafić do układu, którym wykładasz się w tej kolejce.',
  'zolik.rules.pickup.noReturn':
    'Karty wziętej ze stosu odrzuconych nie wolno odrzucić w tej samej kolejce — zagraj ją albo zatrzymaj.',
  'zolik.rules.wilds.setLimit': 'Grupa nie może zawierać więcej jokerów niż kart naturalnych.',
  'zolik.rules.set.maxSize':
    'Grupa nie może liczyć więcej niż {n} kart — joker zastępuje brakujący kolor, nie dopełnia kompletnej grupy.',
  'zolik.rules.run.maxLength':
    'Sekwens nie może liczyć więcej niż {n} kart — as na dole, dwanaście wartości nad nim i as na górze.',
  'zolik.rules.run.aceBridge':
    'As stoi nad królem albo pod dwójką, nigdy jako pomost między końcami sekwensu.',
  'zolik.rules.contracts.contribution':
    'Dopóki nie jesteś wyłożony, każdy wykładany układ musi być tym, którego kontrakt rozdania jeszcze wymaga.',
  'zolik.rules.layoff.afterDown':
    'Nie możesz dokładać do cudzych układów, dopóki nie wyłożysz własnego kontraktu.',
  'zolik.rules.layoff.runEnds':
    'Karta dokładana do sekwensu musi przedłużać go z jednego albo z drugiego końca.',
  'zolik.rules.jokers.swap': 'Jokera w układzie na stole można wykupić dokładnie tą kartą, którą zastępuje.',
  'zolik.rules.jokers.reclaim.on':
    'Joker wykupiony ze stołu musi zostać zagrany w układ w tej samej kolejce — nie może zostać na ręce.',
  'zolik.rules.jokers.reclaim.off': 'Jokera wykupionego ze stołu można zatrzymać na ręce.',
  'zolik.rules.deck.reshuffle':
    'Gdy talia się skończy, stos odrzuconych zostaje przetasowany i staje się nową talią; jeśli oba są puste, rozdanie się kończy.',


  // --- what to do instead ---------------------------------------------------
  // The third layer of a refusal, after the reason and the rule. Sent by the
  // module because it is the only side that knows which card is owed and
  // which way out is on offer; see server/internal/zolikmod/remedy.go.
  'zolik.remedy.meldThePickup': 'Dodaj {card} do wykładanego układu albo cofnij podniesienie.',
  'zolik.remedy.discardSomethingElse': 'Odrzuć inną kartę albo zagraj {card} w tej kolejce.',
  'zolik.remedy.discardNotAJoker': 'Odrzuć coś innego niż jokera.',
  'zolik.remedy.finishOrUndoLayDown': 'Dokończ wykładanie albo je cofnij.',
  'zolik.remedy.needMorePoints': 'Brakuje ci jeszcze {n} punktów, żeby móc się wyłożyć.',
  'zolik.remedy.layACleanRun': 'Wyłóż sekwens bez jokera.',
  'zolik.remedy.playReclaimedJoker': 'Zagraj {card} w układ albo cofnij zdjęcie.',
  'zolik.remedy.goDownFirst': 'Najpierw wyłóż własne układy.',
  'zolik.remedy.drawFirst': 'Najpierw dobierz kartę.',
  'zolik.remedy.drawFromStock': 'Dobierz z talii — stos odrzuconych otwiera się w rundzie {n}.',
  'zolik.remedy.drawFromStockEmpty': 'Dobierz zamiast tego z talii.',


  // --- keys whose meaning lives in their params -----------------------------
  // The shape fallback renders these with the number missing ("Stack", "Pot"),
  // which is why they earn a line where most keys do not.
  'header.contract': 'Wymaga {sets} grup i {runs} sekwensów',
  'header.contract.cleanRunOnly': 'Wymaga sekwensu bez jokera',
  'header.round': 'Runda {n}',
  'header.deck': 'Talia',
  'header.target': 'Cel',
  'header.suitInPlay': 'Kolor w grze',
  'seat.cards': 'Karty',
  'zolik.offer.meld': 'Wyłóż',
  'prompt.pickupMustBeMelded':
    '{value} pochodzi ze stosu odrzuconych — musi trafić do układów, którymi wykładasz się w tej kolejce.',
  'prompt.jokerMustBePlayed': '{value} pochodzi ze stołu — musi trafić do układu, zanim zakończysz kolejkę.',
  'prompt.initialMeld': 'Otwarcie twojej strony musi osiągnąć {n} punktów.',
  'prompt.canastasNeeded': 'Twojej stronie brakuje jeszcze {n} canast, żeby móc wyjść.',
  'prompt.mustDrawOrAnswerSeven': 'Odpowiedz siódemką albo dobierz {n} kart.',
  'prompt.chooseSuit': 'Wybierz kolor, który obowiązuje dalej',
  'prompt.skipPending': 'Tracisz kolejkę',
  'status.lastDeal': 'Drużyna {team} zdobyła {value}',
  'status.teamScore': 'Drużyna {team}: {value}',
  'canasta.offer.rank': 'Wartość',
  'canasta.offer.sequence': 'Sekwens',
  'badge.naturalCanasta': 'Czysta canasta',
  'badge.mixedCanasta': 'Brudna canasta',
  'badge.samba': 'Samba',
  'badge.cleanRun': 'Czysty sekwens',
  'canasta.seat.teamScore': 'Wynik drużyny',
  'canasta.seat.canastas': 'Canasty',
  'holdem.header.pot': 'Pula',
  'holdem.header.street': 'Ulica',
  'holdem.header.hand': 'Rozdanie',
  'holdem.header.handLimit': 'Rozdań łącznie',
  'holdem.header.blinds': 'Ciemne',
  'holdem.cost.call': 'do sprawdzenia',
  'holdem.cost.pot': 'w puli',
  'holdem.seat.stack': 'Stos',
  'holdem.seat.bet': 'Zakład',
  'holdem.prompt.yourAction': 'Twój ruch',
  'holdem.prompt.raiseTo': 'Podbij do',
  'holdem.quick.halfPot': '½ Pula',
  'holdem.quick.pot': 'Pula',
  'holdem.quick.allIn': 'All-in',
  'zone.yourHand': 'Twoja ręka',
  'zone.opponentHand': 'Jego ręka',
  'zone.drawPile': 'Talia',
  'zone.discardPile': 'Stos odrzuconych',
  'zone.melds': 'Układy',
  'zone.teamMelds': 'Układy twojej strony',
  'zone.opponentMelds': 'Układy przeciwnej strony',
  'zone.redThrees': 'Czerwone trójki',
  'zone.board': 'Stół',
  'verb.drawFromDeck': 'Dobierz',
  'verb.takeFromDiscard': 'Weź ze stosu',


  // --- the why sheet's own furniture ---------------------------------------
  // The three layers a refusal is explained in. Labels, not sentences: what
  // goes under each is sent by the server or worded above.
  'why.reason': 'Dlaczego nie',
  'why.rule': 'Zasada',
  'why.rules': 'Zasady',
  'why.remedy': 'Co możesz zrobić',
  'why.readTheRules': 'Przeczytaj pełne zasady →',
  'why.close': 'Zamknij',
  'why.open': 'dlaczego',

  // --- marks on a particular card ------------------------------------------
  // A mark is a refusal that has not happened yet: the rule is enforced at the
  // discard, which is the last possible moment to hear about it, so the card
  // says so while there is still a turn left to act on it.
  'zolik.badge.owedToMeld':
    '{card} pochodzi ze stosu odrzuconych — musi trafić do układów, którymi wykładasz się w tej kolejce.',
  'zolik.badge.jokerOwed': '{card} pochodzi ze stołu — musi trafić do układu, zanim zakończysz kolejkę.',

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
  'legal.terms': 'Warunki',
  // The document's own name, used as the heading above it. Kept apart from
  // `legal.notice.terms`, which is the same words inflected to sit inside a
  // sentence — a heading in the instrumental case reads as a mistake in every
  // Slavic language.
  'legal.terms.title': 'Warunki korzystania',
  'legal.privacy.title': 'Informacja o prywatności',
  'legal.privacy': 'Prywatność',
  'legal.source': 'Kod źródłowy',
  'legal.updated': 'Wersja {version}',
  'legal.draft':
    'Projekt — jeszcze nieobowiązujący. Nazwa, kraj i adres kontaktowy operatora pozostają do uzupełnienia.',
  'legal.notice.before': 'Grając, akceptujesz ',
  'legal.notice.terms': 'warunki korzystania',
  'legal.notice.between': '. To, co jest o tobie przechowywane, opisuje ',
  'legal.notice.privacy': 'informacja o prywatności',
  'legal.notice.after': '.',
  // --- gin rummy -------------------------------------------------------------
  'err.UPCARD_DECLINED': 'Już zrezygnowałeś z tej karty',
  'err.DEADWOOD_TOO_HIGH': 'Twój deadwood jest za wysoki, żeby pukać',
  'err.CARD_DOES_NOT_EXTEND_MELD': 'Ta karta nie przedłuża tego układu',
  'ginrummy.rules.setup': 'Przygotowanie',
  'ginrummy.rules.turn': 'Twoja kolej',
  'ginrummy.rules.melds': 'Układy',
  'ginrummy.rules.knocking': 'Pukanie',
  'ginrummy.rules.bigGin': 'Big gin',
  'ginrummy.rules.layoff': 'Dokładanie',
  'ginrummy.rules.deadHand': 'Martwe rozdanie',
  'ginrummy.rules.scoring': 'Punktacja rozdania',
  'ginrummy.rules.match': 'Wygranie meczu',
  'ginrummy.rules.lineBonuses': 'Premie w podliczeniu',
  'ginrummy.rules.deck': 'Gra się talią {value} kart.',
  'ginrummy.rules.deal': 'Każdy gracz dostaje {value} kart.',
  'ginrummy.rules.upcard': 'Jeszcze jedną kartę odkrywa się, by rozpocząć stos odrzuconych.',
  'ginrummy.rules.drawDiscard':
    'W swojej kolejce dobierz jedną kartę — z talii albo ze stosu odrzuconych — a potem jedną odrzuć.',
  'ginrummy.rules.setsAndRuns':
    'Układ to grupa trzech lub czterech kart tej samej wartości albo sekwens trzech lub więcej kart w jednym kolorze.',
  'ginrummy.rules.aceLow': 'As jest zawsze niski — nie ma sekwensu od damy do asa.',
  'ginrummy.rules.knockLimit': 'Możesz zapukać, gdy twój deadwood wynosi {n} lub mniej.',
  'ginrummy.rules.oklahoma': 'Limit pukania w tym rozdaniu wyznacza wartość odkrytej karty.',
  'ginrummy.rules.gin': 'Zerowy deadwood to gin — najlepsze możliwe zapukanie.',
  'ginrummy.rules.bigGinBonus':
    'Jedenaście kart w układach, bez odrzucania czegokolwiek, to big gin, wart dodatkowe {n} punktów.',
  'ginrummy.rules.layoffDescription':
    'Po zapukaniu, które nie jest ginem, przeciwnik może dołożyć własny deadwood do twoich układów, zanim ręce zostaną porównane.',
  'ginrummy.rules.deadHandDescription':
    'Jeśli w talii zostaną ostatnie dwie karty, a nikt nie zapukał, rozdanie jest martwe — nikt nie punktuje, a ten sam rozdający rozdaje ponownie.',
  'ginrummy.rules.undercut':
    'Jeśli deadwood przeciwnika nie jest wyższy od twojego, podcina cię: zapisuje różnicę plus {n}.',
  'ginrummy.rules.ginBonus': 'Gin daje całą rękę przeciwnika plus {n}.',
  'ginrummy.rules.target': 'Kto pierwszy przekroczy {n} punktów po zakończeniu rozdania, wygrywa mecz.',
  'ginrummy.rules.shutout':
    'Premia meczowa podwaja się do {n}, jeśli przegrany nie zdobył ani jednego punktu.',
  'ginrummy.rules.box': 'Każde wygrane rozdanie jest warte {n} punktów na koniec meczu.',
  'ginrummy.rules.gameBonus': 'Wygranie meczu daje dodatkowe {n} punktów.',
  'ginrummy.fact.deadwood': 'deadwood {value}',
  'ginrummy.fact.discardCard': 'Odrzuć {value}',
  'ginrummy.fact.meldCards': 'Do {value}',
  'ginrummy.header.hand': 'Rozdanie {n}',
  'ginrummy.round.kind': '{value}',
  'ginrummy.round.hand': 'Rozdanie',
  'ginrummy.seat.score': '{value}',
  'ginrummy.seat.dealer': 'Rozdający',
  'ginrummy.status.knocked': '{playerId} zapukał z deadwoodem {deadwood}',
  'ginrummy.status.gin': '{playerId} zrobił gina',
  'ginrummy.status.lastHand': 'Ostatnie rozdanie: {winner} ({kind}, {delta} pkt)',
  'ginrummy.offer.drawStock': 'Dobierz z talii',
  'ginrummy.offer.drawDiscard': 'Dobierz ze stosu odrzuconych',
  'ginrummy.offer.takeUpcard': 'Weź odkrytą kartę',
  'ginrummy.offer.passUpcard': 'Pas',
  'ginrummy.offer.discard': 'Odrzuć',
  'ginrummy.offer.knock': 'Zapukaj',
  'ginrummy.offer.gin': 'Gin!',
  'ginrummy.offer.bigGin': 'Big gin!',
  'ginrummy.offer.layOff': 'Dołóż',
  'ginrummy.offer.finishLayoff': 'Koniec dokładania',
  'ginrummy.zone.knockerHand': 'Ręka pukającego',
  'ginrummy.zone.melds': 'Układy',
  'ginrummy.prompt.upcardDecision': 'Weź odkrytą kartę albo spasuj',
  'ginrummy.prompt.yourTurnDraw': 'Dobierz kartę',
  'ginrummy.prompt.yourTurnDiscard': 'Odrzuć — albo zapukaj, jeśli możesz',
  'ginrummy.prompt.layoff': 'Dołóż deadwood albo zakończ',

  // --- rummy tiles -------------------------------------------------------------
  'err.TILE_NOT_IN_HAND': 'Nie masz tej kostki na ręce',
  'err.TILE_DOES_NOT_FIT': 'To tam nie pasuje',
  'err.NO_SUCH_SET': 'Tego układu nie ma na stole',
  'err.INITIAL_MELD_ONLY': 'Przed pierwszym wyłożeniem możesz przestawiać tylko własne nowe układy',
  'err.TABLE_NOT_VALID': 'Stół nie jest jeszcze poprawny',
  'err.TRAY_NOT_EMPTY': 'Masz jeszcze luźne kostki do ułożenia',
  'err.NOTHING_PLAYED': 'Zagraj co najmniej jedną kostkę, zanim zakończysz kolejkę',
  'err.INITIAL_MELD_TOO_LOW': 'Twoje pierwsze wyłożenie musi być warte co najmniej 30 punktów',
  'err.NOT_A_RUN': 'Podzielić można tylko sekwens',
  'err.BAD_SPLIT_POSITION': 'W tym miejscu tego sekwensu nie da się podzielić',
  'err.NO_JOKER_IN_SET': 'W tym układzie nie ma jokera',
  'err.TILE_JOKER_SWAP_MISMATCH': 'Ta kostka nie jest tym, co zastępuje joker',
  'rummytiles.rules.setup': 'Przygotowanie',
  'rummytiles.rules.sets': 'Układy',
  'rummytiles.rules.initialMeld': 'Pierwsze wyłożenie',
  'rummytiles.rules.turn': 'Twoja kolej',
  'rummytiles.rules.jokerTaking': 'Zabranie jokera',
  'rummytiles.rules.ending': 'Zakończenie rundy',
  'rummytiles.rules.poolExhaustion': 'Gdy pula się wyczerpie',
  'rummytiles.rules.match': 'Wygranie meczu',
  'rummytiles.rules.tiles': 'Gra się {value} kostkami.',
  'rummytiles.rules.dealCount': 'Każdy gracz dostaje {value} kostek.',
  'rummytiles.rules.group': 'Grupa to trzy albo cztery kostki z tą samą liczbą, każda w innym kolorze.',
  'rummytiles.rules.run': 'Sekwens to trzy albo więcej kolejnych liczb w jednym kolorze.',
  'rummytiles.rules.noWrap': 'Po 13 nie wraca się do 1.',
  'rummytiles.rules.joker': 'Joker zastępuje dowolną kostkę.',
  'rummytiles.rules.initialMeldDescription':
    'Dopóki nie wyłożysz {n} lub więcej punktów w jednej kolejce, wyłącznie z własnej ręki, nie wolno ci ruszać niczego, co już leży na stole.',
  'rummytiles.rules.turnDescription':
    'Zagraj co najmniej jedną kostkę z ręki, dowolnie przestawiając stół, i zakończ tak, by każdy układ na stole był poprawny.',
  'rummytiles.rules.noDiscard':
    'Nie ma odrzucania — jeśli nie możesz wykonać poprawnej kolejki, dobierasz jedną kostkę.',
  'rummytiles.rules.jokerTakingDescription':
    'Jokera ze stołu można zabrać, zastępując go kostką, którą reprezentuje, wziętą z ręki — i trzeba go użyć w układzie przed końcem kolejki.',
  'rummytiles.rules.goingOut':
    'Rundę wygrywa pierwszy gracz, któremu skończą się kostki. Wszyscy pozostali zapisują ujemną wartość tego, co im zostało; zwycięzca zapisuje sumę strat wszystkich innych.',
  'rummytiles.rules.poolExhaustionLowestWins':
    'Jeśli pula się wyczerpie i nikt nie może zagrać, runda się kończy, a wygrywa ją najniższa wartość ręki.',
  'rummytiles.rules.poolExhaustionNoWinner':
    'Jeśli pula się wyczerpie i nikt nie może zagrać, runda kończy się bez zwycięzcy — każda ręka zostaje po prostu podliczona.',
  'rummytiles.rules.target': 'Kto pierwszy przekroczy {n} punktów po zakończeniu rundy, wygrywa mecz.',
  'rummytiles.rules.roundLimit': 'Mecz kończy się po {n} rundach — wygrywa najwyższy wynik.',
  'rummytiles.fact.setCards': '{value}',
  'rummytiles.header.pool': 'Pula {n}',
  'rummytiles.header.round': 'Runda {n}',
  'rummytiles.round.kind': '{value}',
  'rummytiles.round.round': 'Runda',
  'rummytiles.seat.score': '{value}',
  'rummytiles.seat.notOpened': 'Bez otwarcia',
  'rummytiles.status.lastRound': 'Ostatnia runda: {winner} ({kind})',
  'rummytiles.badge.invalid': 'Jeszcze niepoprawne',
  'rummytiles.zone.pool': 'Pula',
  'rummytiles.zone.table': 'Stół',
  'rummytiles.zone.tray': 'Stojak',
  'rummytiles.offer.place': 'Połóż',
  'rummytiles.offer.addFromHand': 'Dodaj',
  'rummytiles.offer.addFromTray': 'Dodaj ze stojaka',
  'rummytiles.offer.take': 'Weź',
  'rummytiles.offer.split': 'Podziel',
  'rummytiles.offer.swapJoker': 'Wymień jokera',
  'rummytiles.offer.resetTurn': 'Cofnij kolejkę',
  'rummytiles.offer.commit': 'Gotowe',
  'rummytiles.offer.draw': 'Dobierz',
  'rummytiles.param.position': 'Podziel przy',

  // --- blackjack -----------------------------------------------------------
  //
  // The house rules are worded twice over — on and off — because a rule that
  // is *not* in force is still something a player has to be told. "No
  // surrender at this table" is information; silence is a guess.
  'err.BET_BELOW_MINIMUM': 'To poniżej minimum stołu',
  'err.ALREADY_BET': 'Twoja stawka już stoi',
  'err.INSURANCE_CLOSED': 'W tej chwili nie ma ubezpieczenia do wzięcia',
  'err.CANNOT_DOUBLE': 'Tego rozdania nie można podwoić',
  'err.CANNOT_SPLIT': 'Tego rozdania nie można podzielić',
  'err.CANNOT_SURRENDER': 'Tego rozdania nie można oddać',

  'blackjack.rules.section.table': 'Stół',
  'blackjack.rules.section.play': 'Rozgrywka',
  'blackjack.rules.section.dealer': 'Krupier',
  'blackjack.rules.section.end': 'Jak kończy się mecz',
  'blackjack.rules.goal':
    'Pokonaj krupiera, nie przekraczając dwudziestu jeden. Przekroczenie oznacza natychmiastową przegraną, cokolwiek krupier zrobi później.',
  // Counts are their own phrase rather than a number glued to a noun: one
  // deck and six decks inflect differently, and Czech inflects them again.
  'blackjack.rules.decks': 'Talie w butze: {n}.',
  'blackjack.rules.stack': 'Każde miejsce siada z {n} żetonami.',
  'blackjack.rules.minBet': 'Minimum stołu to {n} żetonów.',
  'blackjack.rules.faceUp':
    'Karty graczy rozdaje się odkryte; krupier trzyma jedną kartę zakrytą, dopóki wszyscy nie zagrają.',
  'blackjack.rules.hitStand': 'Dobierz tyle kart, ile chcesz, albo zostań przy tym, co masz.',
  'blackjack.rules.aces': 'As liczy się jako jedenaście, dopóki to się mieści, a poza tym jako jeden.',
  'blackjack.rules.blackjack': 'As z kartą o wartości dziesięć, na dwóch pierwszych kartach, to blackjack.',
  'blackjack.rules.pays3to2': 'Blackjack płaci 3:2.',
  'blackjack.rules.pays6to5': 'Blackjack płaci 6:5.',
  'blackjack.rules.paysEven': 'Blackjack płaci jeden do jednego.',
  'blackjack.rules.double':
    'Na dwóch pierwszych kartach możesz podwoić stawkę i dobrać dokładnie jedną kartę.',
  'blackjack.rules.doubleAfterSplit': 'Rozdanie powstałe z podziału też można podwoić.',
  'blackjack.rules.noDoubleAfterSplit': 'Rozdania powstałego z podziału nie można podwoić.',
  'blackjack.rules.split':
    'Dwie karty o tej samej wartości można podzielić na osobne rozdania, każde z własną stawką — do {n} razy, łącznie na {hands} rozdań.',
  'blackjack.rules.noSplit': 'Przy tym stole par się nie dzieli.',
  'blackjack.rules.splitAces':
    'Podzielone asy dostają po jednej karcie i zostają, a dwadzieścia jeden uzyskane w ten sposób nie jest blackjackiem.',
  'blackjack.rules.surrender':
    'Możesz oddać pierwsze rozdanie za połowę stawki, gdy krupier sprawdzi już, czy ma blackjacka.',
  'blackjack.rules.noSurrender': 'Przy tym stole nie można oddawać rozdań.',
  'blackjack.rules.dealerDraws': 'Krupier dobiera do siedemnastu, a potem zostaje.',
  'blackjack.rules.hitsSoft17': 'Krupier dobiera przy siedemnastu utworzonych z asem.',
  'blackjack.rules.standsSoft17': 'Krupier zostaje przy siedemnastu utworzonych z asem.',
  'blackjack.rules.dealerPeeks':
    'Pokazując asa albo dziesiątkę, krupier sprawdza blackjacka, zanim ktokolwiek zagra.',
  'blackjack.rules.insurance':
    'Przeciw asowi krupiera możesz się ubezpieczyć za połowę stawki; płaci 2:1, jeśli krupier ma blackjacka.',
  'blackjack.rules.noInsurance': 'Przy tym stole nie oferuje się ubezpieczenia.',
  'blackjack.rules.rounds': 'Przy stole rozgrywa się {n} rund.',
  'blackjack.rules.mostChipsWins': 'Kto na koniec ma najwięcej żetonów, wygrywa mecz.',
  'blackjack.rules.bustedOut':
    'Miejsce, które nie jest już w stanie pokryć minimum {n}, pauzuje do końca meczu.',

  'blackjack.zone.dealer': 'Krupier',
  'blackjack.zone.box': 'Rozdanie',
  'blackjack.zone.yourBox': 'Twoje rozdanie',
  'blackjack.zone.shoe': 'But',

  'blackjack.header.round': 'Runda {n} z {of}',
  'blackjack.header.minBet': 'Minimum',
  'blackjack.header.decks': 'Talie',
  'blackjack.header.dealerTotal': 'Krupier pokazuje {n}',
  'blackjack.header.dealerSoftTotal': 'Krupier pokazuje miękkie {n}',

  'blackjack.seat.stack': 'Żetony',
  'blackjack.seat.bet': 'Stawka',
  'blackjack.seat.insurance': 'Ubezpieczenie',
  'blackjack.seat.total': 'Suma',
  'blackjack.seat.softTotal': 'Suma miękka',
  'blackjack.seat.out': 'Bez żetonów',

  'blackjack.prompt.placeBet': 'Postaw stawkę',
  'blackjack.prompt.insurance': 'Ubezpieczenie?',
  'blackjack.prompt.yourMove': 'Twój ruch',
  'blackjack.prompt.waitingFor': 'Czekamy na {playerId}',
  'blackjack.prompt.betAmount': 'Stawka',

  'blackjack.quick.doubleMin': '2× Minimum',
  'blackjack.quick.allIn': 'All-in',
  'blackjack.offer.bet': 'Postaw',
  'blackjack.offer.hit': 'Dobierz',
  'blackjack.offer.stand': 'Zostaję',
  'blackjack.offer.double': 'Podwój',
  'blackjack.offer.split': 'Podziel',
  'blackjack.offer.surrender': 'Oddaj',
  'blackjack.offer.insure': 'Ubezpiecz',
  'blackjack.offer.declineInsurance': 'Bez ubezpieczenia',

  'blackjack.fact.tableMinimum': 'minimum',
  'blackjack.fact.insuranceCost': 'za ubezpieczenie',
  'blackjack.fact.extraStake': 'do postawienia',
  'blackjack.fact.surrenderReturn': 'zwrotu',

  'blackjack.status.dealerBlackjack': 'Krupier miał blackjacka',
  'blackjack.status.dealerBust': 'Krupier przebił z {n}',
  'blackjack.status.dealerStands': 'Krupier zostaje przy {n}',

  'blackjack.round.name': 'Runda',
  'blackjack.round.dealerTotal': 'Krupier {n}',
  'blackjack.round.dealerBust': 'Krupier przebił ({n})',
  'blackjack.round.dealerBlackjack': 'Blackjack krupiera',
  'blackjack.round.outcome.blackjack': 'Blackjack',
  'blackjack.round.outcome.win': 'Wygrana',
  'blackjack.round.outcome.push': 'Remis',
  'blackjack.round.outcome.lose': 'Przegrana',
  'blackjack.round.outcome.bust': 'Przebicie',
  'blackjack.round.outcome.surrender': 'Oddane',

  'blackjack.badge.inPlay': 'W grze',
  'blackjack.badge.doubled': 'Podwojone',
  'blackjack.badge.split': 'Podzielone',
  'blackjack.badge.blackjack': 'Blackjack',
  'blackjack.badge.bust': 'Przebicie',
  'blackjack.badge.won': 'Wygrana',
  'blackjack.badge.push': 'Remis',
  'blackjack.badge.lost': 'Przegrana',
  'blackjack.badge.surrendered': 'Oddane',

  'blackjack.unit.chips': 'żetonów',

  // --- the settings screen --------------------------------------------------
  //
  // Hardcoded English until now, which was survivable while the only other
  // language was unreachable and faintly absurd once the language picker
  // itself lived on this screen: a player who cannot read "Settings" is
  // exactly the one who came here to change it.
  'settings.title': 'Ustawienia',
  'settings.signedInAs': 'Zalogowano jako {username}',
  'settings.playingAsGuest': 'Grasz jako {username} (gość)',
  'settings.notSignedIn': 'Nie jesteś zalogowany — zaloguj się lub kontynuuj jako gość, aby grać online.',
  'settings.subtitle': 'Jak wyglądasz ty i jak wygląda stół',
  'settings.face.heading': 'Twoja twarz przy stole',
  'settings.face.account': 'Zapisane przy koncie, więc pójdzie z tobą na inne urządzenie.',
  'settings.face.device': 'Zapisane na tym urządzeniu. Zaloguj się, by zabrać je ze sobą.',
  'settings.skin.heading': 'Wygląd stołu',
  'settings.language.heading': 'Język',
  'settings.language.status': 'Zapisane na tym urządzeniu.',
  'settings.language.auto': 'Automatycznie',
  'settings.language.auto.now': 'Zgodnie z urządzeniem — teraz {language}',
  'settings.legal.heading': 'Drobny druk',
  'settings.legal.status': 'Na co zgodziłeś się, grając, i co jest o tobie przechowywane.',
  'settings.signIn': 'Zaloguj się',
  'settings.back': 'Wstecz',

  // A translated interface over an English notice is a worse lie than an
  // English interface, so the document says so itself rather than letting the
  // surrounding screen imply the text was written for this reader.
  'legal.untranslated':
    'Tej informacji nie przetłumaczono jeszcze na twój język. Obowiązuje angielski tekst poniżej.',

  // --- the navigation bar's screen titles ------------------------------------
  //
  // These sit in `app/_layout.tsx`, above every screen, and were the last
  // English left after the settings screen was translated — a German player
  // reading "Einstellungen" under a bar that said "Settings". A translated app
  // with an untranslated chrome bar looks like a bug, not like a limit.
  'nav.home': 'Zolik',
  'nav.emailSignIn': 'Logowanie e-mailem',
  'nav.signingIn': 'Logowanie',
  'nav.usernameSignIn': 'Logowanie nazwą użytkownika',
  'nav.legacyAccount': 'Stare konto',
  'nav.guest': 'Gość',
  'nav.account': 'Konto',
  'nav.games': 'Gry',
  'nav.table': 'Twój stół',
  'nav.join': 'Dołącz do stołu',
  // The screen a shared invite link lands on while it resolves the code.
  'nav.joining': 'Dołączanie',
  'nav.rules': 'Zasady',
  'nav.match': 'Mecz',
  'nav.scoreTable': 'Tabela punktów',
  'nav.stats': 'Statystyki',
  'nav.more': 'Więcej',
  'nav.about': 'O aplikacji',

  // --- the account menu behind the face in the corner ----------------------
  'menu.label': 'Menu konta',
  'menu.signedIn': 'Zalogowany',
  'menu.notSignedIn': 'Niezalogowany',
  'menu.keepStats': 'by zachować swoje statystyki',
  'menu.signOut': 'Wyloguj się',
  'more.scoreTable': 'Tabela wyników offline',
  'more.stats': 'Statystyki i ranking',
  'more.needsAccount': 'zaloguj się, by użyć',
  'gate.title': 'Zaloguj się, by tego użyć',
  'gate.body':
    'Tabele wyników i statystyki są zapisywane przy twoim koncie, więc idą z tobą na inne urządzenie. Gość nie ma gdzie ich trzymać.',

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
  'error.generic': 'To się nie udało',
  'error.signIn': 'Logowanie nie powiodło się',
  'error.login': 'Logowanie nie powiodło się',
  'error.register': 'Rejestracja nie powiodła się',
  'error.sendCode': 'Nie udało się wysłać kodu',
  'error.badCode': 'Ten kod nie zadziałał',
  'error.rulesLoad': 'Nie udało się wczytać zasad',
  'error.createFailed': 'Tworzenie nie powiodło się',
  'error.saveFailed': 'Zapis nie powiódł się',
  'error.exportFailed': 'Eksport nie powiódł się',

  // --- the route that does not exist ----------------------------------------
  'notFound.title': 'Ups!',
  'notFound.message': 'Ten ekran nie istnieje.',
  'notFound.home': 'Przejdź do ekranu głównego!',

  // --- signing in -----------------------------------------------------------
  'auth.login.subtitle': 'Zachowaj statystyki na wszystkich urządzeniach',
  'auth.login.continueWithEmail': 'Kontynuuj e-mailem',
  'auth.login.usernameInstead': 'Zaloguj się zamiast tego nazwą użytkownika',
  'auth.email.title': 'Logowanie e-mailem',
  'auth.email.subtitle': 'Wyślemy ci jednorazowy kod',
  'auth.email.address': 'Adres e-mail',
  'auth.email.send': 'Wyślij kod',
  'auth.email.codeTitle': 'Wpisz kod',
  'auth.email.codePlaceholder': 'Kod sześciocyfrowy',
  'auth.email.differentAddress': 'Użyj innego adresu',
  'auth.email.sentTo': 'Wysłano na {email}',
  'auth.email.continue': 'Dalej',
  'auth.guest.title': 'Gra jako gość',
  'auth.guest.subtitle': 'Konto niepotrzebne',
  'auth.guest.displayName': 'Nazwa wyświetlana',
  'auth.register.title': 'Utwórz konto',
  'auth.register.username': 'Nazwa użytkownika',
  'auth.register.email': 'E-mail (opcjonalnie)',
  'auth.register.password': 'Hasło',
  'auth.username.createAccount': 'Utwórz konto z nazwą użytkownika i hasłem',
  'auth.callback.signedIn': 'Zalogowano.',

  // --- the account screen ---------------------------------------------------
  'account.signInPrompt': 'Zaloguj się, aby zarządzać kontem.',
  'account.keepGames': 'Zachowaj te partie',
  'account.signedInWith': 'Zalogowano przez',
  'account.addMethod': 'Dodaj sposób logowania',
  'account.usernameAndPassword': 'Nazwa użytkownika i hasło',
  'account.faceAndTable': 'Twarz i wygląd stołu',
  'account.refresh': 'Odśwież',
  'account.remove': 'Usuń',

  // --- the main menu --------------------------------------------------------
  'home.subtitle': 'Remik kontynentalny · {server}',
  'home.playingAs': 'Grasz jako {name}',
  'home.signInPrompt': 'Zaloguj się albo graj dalej jako gość, żeby grać online.',
  'home.statsAndLeaderboard': 'Statystyki i ranking',
  'home.play': 'Graj',
  'home.offlineScoreTable': 'Tabela punktów offline',
  'home.signInToKeepStats': 'Zaloguj się, by zachować statystyki',
  'home.signOut': 'Wyloguj się',
  'home.continueAsGuest': 'Kontynuuj jako gość',
  // Someone playing as a guest, marked in a list of names.
  'home.guestSuffix': '(gość)',

  // --- the waiting room on the main menu ------------------------------------
  //
  // Counts are whole phrases per count, not a number glued to a noun — the
  // same reason `countLabel` exists. One and many are the cases the interface
  // actually produces, so they are the cases that get their own wording.
  'waiting.checking': 'Sprawdzamy, kto jest w pobliżu…',
  'waiting.youAreWaiting': 'Czekasz na grę',
  'waiting.pickedUp': 'Każdy, kto otwiera stół, może cię zabrać — nikt nie potrzebuje od ciebie kodu.',
  'waiting.othersOne': 'Czeka też 1 inny gracz',
  'waiting.othersMany': 'Czeka też innych graczy: {n}',
  'waiting.oneWaiting': '1 gracz czeka na grę',
  'waiting.manyWaiting': 'Graczy czekających na grę: {n}',
  'waiting.adding': 'Dodajemy cię do listy oczekujących…',
  'waiting.slowHint':
    'Jeśli nie skończy się to w kilka sekund, sprawdź, czy adres serwera poniżej jest osiągalny z tego urządzenia.',
  'waiting.serverBusyDetail': 'Próba {n}. Serwer w tej chwili nie przyjmuje nowych połączeń z poczekalnią.',
  'waiting.reconnecting': 'Utracono połączenie — łączymy ponownie…',
  'waiting.reconnectingDetail':
    'Próba {n}. Może się tak zdarzyć, gdy zmieniła się sieć twojego urządzenia albo serwer został zrestartowany.',
  'waiting.tryAgain': 'Spróbuj teraz ponownie',
  'waiting.makeAvailable': 'Zgłoś mnie jako gotowego do gry',
  'waiting.stop': 'Przestań czekać',
  'waiting.noneYet':
    'W tej chwili nikt nie czeka na grę. Zapisz się na listę, a będziesz pierwszym, kogo ktokolwiek zobaczy.',
  'waiting.noOthersYet': 'Nikt inny jeszcze nie czeka. Gospodarze i tak cię widzą i mogą cię zaprosić.',
  'waiting.server': 'Serwer',
  'waiting.none': 'W tej chwili nikt nie czeka. Kto zgłosi się w menu głównym, pojawi się tutaj.',

  // --- landing on a shared invite link --------------------------------------
  'join.missingCode': 'Temu linkowi brakuje kodu stołu.',
  'join.staleLink': 'Poproś osobę, która cię zaprosiła, o świeży link, albo dołącz kodem.',
  'join.enterCode': 'Wpisz kod',
  'join.backToMenu': 'Powrót do menu',
  'join.takingSeat': 'Zajmujemy miejsce…',
  'join.takingSeatAt': 'Zajmujemy miejsce przy: {game}…',

  // --- the lobby ------------------------------------------------------------
  'lobby.games.subtitle': 'Wszystko, co ten serwer potrafi udostępnić',
  'lobby.games.bots': 'Boty',
  'lobby.games.playBot': 'Zagraj z botem',
  'lobby.games.playBots': 'Zagraj z botami: {n}',
  'lobby.games.openTable': 'Otwórz stół',
  'lobby.games.players': 'Graczy: {n}',
  'lobby.games.playerRange': 'Graczy: {min}–{max}',
  'lobby.join.placeholder': 'Kod dołączenia albo link z zaproszeniem',
  'lobby.join.needCode': 'Podaj kod, link albo identyfikator meczu',
  'lobby.games.signInFirst': 'Najpierw się zaloguj',
  'lobby.join.action': 'Dołącz',
  'lobby.join.waitingTitle': 'Czekanie na gospodarza',
  // Two whole sentences rather than one with a swapped noun: "a game of {game}"
  // does not survive a language that inflects the game's name after "of".
  'lobby.join.joinedGame': 'Dołączono do gry {game} — czekamy na start',
  'lobby.join.joinedTable': 'Dołączono do stołu — czekamy na start',
  'lobby.table.addBot': 'Dodaj bota',
  'lobby.table.side': 'Strona {n}',
  'lobby.table.shuffleSeats': 'Potasuj miejsca',
  'lobby.table.moveSeatUp': 'Przesuń {name} o miejsce w górę',
  'lobby.table.moveSeatDown': 'Przesuń {name} o miejsce w dół',
  'lobby.table.start': 'Start',
  'lobby.table.waitingForHost': 'Czekamy, aż gospodarz zacznie…',

  // --- inviting someone to a table ------------------------------------------
  'invite.heading': 'Zaproś graczy',
  'invite.explain': 'Wyślij ten link. Kto go otworzy, trafi do tego stołu — konto niepotrzebne.',
  'invite.noAddress': 'Ten serwer nie ma skonfigurowanego adresu do udostępniania, więc użyj kodu poniżej.',
  'invite.readOutCode': 'Albo podyktuj kod:',
  'invite.copy': 'Kopiuj link',
  'invite.share': 'Udostępnij link',
  'invite.copied': 'Skopiowano!',
  'invite.shared': 'Udostępniono',

  // --- the match screen -----------------------------------------------------
  'match.waitingForTable': 'Czekamy na stół…',
  'match.waitingForPlayer': 'Czekamy na innego gracza…',
  'match.nobodyWon': 'Nikt nie wygrał.',
  'match.youWon': 'Wygrałeś.',
  'match.finished': 'Ten mecz się zakończył.',
  'match.inProgress': 'Mecz w toku — wszystko jest połączone i działa normalnie.',
  'match.connecting': 'Łączenie…',
  'match.abandonedTitle': 'Stół odłożony',
  'match.abandoned': 'Nikt nie wrócił do tego stołu, więc został odłożony. Karty są dokładnie tam, gdzie je zostawiłeś.',
  'match.resume': 'Wróć tam, gdzie skończyłeś',
  'match.resuming': 'Przywracanie stołu…',
  'match.controls': 'Sterowanie',
  'match.over': 'Koniec meczu',
  'match.settingUp': 'Przygotowujemy…',
  'match.playAgain': 'Zagraj jeszcze raz',
  'match.backToGames': 'Powrót do gier',
  'match.table': 'Stół',
  'match.opponents': 'Przeciwnicy',
  // Marks which seat is the reader's own, in a list of seats.
  'match.youSuffix': '(ty)',
  // The winner line is composed from whole sentences rather than a name glued
  // to " won." — the verb agrees with the subject in most of these languages,
  // and a suffix cannot know that.
  'match.you': 'ty',
  'match.someoneWon': '{name} wygrywa.',
  'match.wonBy': 'Wygrywa {names}.',
  'match.pausedFor': 'Wstrzymane — czekamy, aż {name} połączy się ponownie.',
  'match.results': 'Wyniki',
  'match.players': 'Gracze',
  'match.toPlay': 'na ruchu',

  // --- the offline score table ----------------------------------------------
  'scoring.namesHint': 'Imiona oddzielone przecinkami (4–8 graczy)',
  'scoring.newSession': 'Nowa sesja',
  // A worked example, not a sentence: the names are placeholders a translator
  // may localise, the shape `name:score,` is what the parser needs.
  'scoring.scoresPlaceholder': 'Ala:120,Bartek:80,…',
  'scoring.saveRound': 'Zapisz rundę',
  'scoring.export': 'Eksportuj kartę wyników',
  'scoring.formatHint': 'Format punktów: Imię:100,Imię2:50',
  'scoring.nameCountError': 'Podaj 2–8 imion graczy oddzielonych przecinkami',
  'scoring.session': 'Sesja: {id}',
  'scoring.players': 'Gracze: {names}',
  'scoring.roundScores': 'Punkty rundy {n}',
  'stats.loading': 'Wczytywanie…',
  // The figures could not be fetched. `{reason}` is whatever the server or the
  // network said, which is not ours to translate — the frame around it is.
  'stats.unavailable': '(niedostępne: {reason})',

  // --- statistics -----------------------------------------------------------
  'stats.title': 'Statystyki i ranking',
  'stats.yours': 'Twoje statystyki',
  'stats.leaderboard': 'Ranking',

  // --- a player's lifetime record, shown beside a finished match ------------
  'record.title': 'Twój bilans',
  'record.guest':
    'Grasz jako gość, więc bilans nie jest prowadzony. Zaloguj się, a partie rozegrane już na tym urządzeniu — łącznie z tą — zostaną przypisane do twojego konta.',
  'record.signInToKeep': 'Zaloguj się i zachowaj je',
  'record.failed': 'Nie udało się teraz wczytać twojego bilansu. Mecz jest bezpiecznie zapisany.',
  'record.loading': 'Wczytywanie…',
  'record.played': 'Rozegrane',
  'record.won': 'Wygrane',
  'record.lost': 'Przegrane',
  'record.winRate': 'Skuteczność',
  'record.streak': 'Seria',
  'record.atThisGame': 'W tej grze',
  // A streak is a count with a noun, so it is a whole phrase per count for
  // the same reason `countLabel` is: "1 win" / "2 wins" is an English rule,
  // and most of these languages do not share it.
  'record.streakNone': '—',
  'record.streakWinOne': '1 wygrana',
  'record.streakWinMany': 'Wygrane: {n}',
  'record.streakLossOne': '1 przegrana',
  'record.streakLossMany': 'Przegrane: {n}',

  // --- moving cards ---------------------------------------------------------
  'hand.dragHint': 'Przeciągnij kartę wzdłuż wachlarza, żeby ją przestawić, albo na stół, żeby ją zagrać',
  'hand.moveLeft': 'W lewo',
  'hand.moveRight': 'W prawo',
  'zone.collapseGroup': 'Zwiń tę grupę',
  'zone.expandGroup': 'Pokaż wszystkie karty w tej grupie',
  'zone.dropHere': 'Upuść tutaj',
  'offer.pickCards': 'wybierz karty dla miejsca, które dotknąłeś',
  'offer.ambiguous': 'to pasuje w więcej niż jedno miejsce — wybierz na stole',

  // --- the build footer -----------------------------------------------------
  'build.app': 'aplikacja',
  'build.server': 'serwer',
  'about.subtitle': 'Wersja, w którą grasz, i drobny druk.',
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
  'option.pauseBetweenRounds': 'Przerwa między rundami',
  'choice.pauseBetweenRounds.1': 'Przerwa',
  'choice.pauseBetweenRounds.0': 'Graj dalej bez przerwy',
  'option.botSkill': 'Przeciwnicy',
  'choice.botSkill.0': 'Mieszani',
  'choice.botSkill.1': 'Łatwi',
  'choice.botSkill.2': 'Średni',
  'choice.botSkill.3': 'Trudni',
  'option.initialMeldMinimum': 'Wartość otwarcia',
  'choice.initialMeldMinimum.0': 'Bez minimum',
  'option.discardDrawMinRound': 'Branie ze stosu odrzuconych',
  'choice.discardDrawMinRound.0': 'Otwarte',
  'choice.discardDrawMinRound.2': 'Od rundy 2',
  'choice.discardDrawMinRound.3': 'Od rundy 3',
  'option.requireCleanRun': 'Sekwens bez jokera',
  'choice.requireCleanRun.1': 'Wymagany',
  'choice.requireCleanRun.0': 'Nie',
  'option.jokerReclaimMustPlay': 'Wykupiony joker',
  'choice.jokerReclaimMustPlay.1': 'Zagrać w tej samej kolejce',
  'choice.jokerReclaimMustPlay.0': 'Można zatrzymać',
  'option.dealStarter': 'Kto zaczyna',
  'choice.dealStarter.0': 'Po kolei',
  'choice.dealStarter.1': 'Zaczyna zwycięzca',
  'variation.prsi.classic': 'Klasyczne',
  'option.handSize': 'Rozdawane karty',
  'variation.canasta.classic': 'Klasyczna',
  'variation.canasta.modern_american': 'Modern American',
  'variation.canasta.samba': 'Samba',
  'option.targetScore': 'Wynik docelowy',
  'option.canastasToGoOut': 'Canasty do wyjścia',
  'variation.holdem.freezeout': 'Freezeout',
  'variation.holdem.timed': 'Stała liczba rozdań',
  'option.startingStack': 'Żetony na start',
  'option.bigBlind': 'Duża ciemna',
  'option.handLimit': 'Rozdania',
  'choice.handLimit.0': 'Aż zostanie jedno miejsce',
  'variation.ginrummy.standard': 'Standardowy',
  'option.knockLimit': 'Limit pukania',
  'choice.knockLimit.0': 'Oklahoma (wyznacza go odkryta karta)',
  'option.bigGin': 'Big gin',
  'choice.bigGin.0': 'Nie',
  'choice.bigGin.1': 'Tak (+25)',
  'option.lineBonuses': 'Premie w podliczeniu',
  'choice.lineBonuses.1': 'Tak',
  'choice.lineBonuses.0': 'Nie',
  'variation.rummytiles.standard': 'Standardowe',
  'choice.targetScore.0': 'Bez celu',
  // Two games qualify a number the others mean plainly: Canasta's 500 is a
  // short game, Gin Rummy's and Rummy Tiles' 500 is just 500; Hold'em's
  // starting 200 is a short stack, Blackjack's 200 is just 200. Module-scoped
  // so the qualifier travels with the game that means it — a bare key would
  // hand the note to games it is false for. `scripts/check-server-labels.js`
  // is what finds these; both were shipping as English on a Czech screen.
  'choice.canasta.targetScore.500': '500 (krótka)',
  'choice.holdem.startingStack.200': '200 (krótka)',
  'option.roundLimit': 'Limit rund',
  'choice.roundLimit.0': 'Bez limitu',
  'option.poolExhaustion': 'Gdy pula się wyczerpie',
  'choice.poolExhaustion.1': 'Rundę wygrywa najniższa ręka',
  'choice.poolExhaustion.0': 'Nikt nie wygrywa rundy',
  'variation.blackjack.single': 'Jedna talia',
  'option.minBet': 'Minimum stołu',
  'option.rounds': 'Rundy',
  'option.decks': 'Talie',
  'option.dealerHitsSoft17': 'Krupier przy miękkiej 17',
  'choice.dealerHitsSoft17.0': 'Zostaje',
  'choice.dealerHitsSoft17.1': 'Dobiera',
  'option.blackjackPays': 'Blackjack płaci',
  'choice.blackjackPays.100': 'Jeden do jednego',
  'option.maxSplits': 'Dzielenie',
  'choice.maxSplits.0': 'Bez dzielenia',
  'choice.maxSplits.1': 'Raz (dwa rozdania)',
  'choice.maxSplits.3': 'Trzy razy (cztery rozdania)',
  'option.doubleAfterSplit': 'Podwojenie po podziale',
  'choice.doubleAfterSplit.1': 'Dozwolone',
  'choice.doubleAfterSplit.0': 'Niedozwolone',
  'option.surrender': 'Oddanie',
  'choice.surrender.0': 'Nie',
  'choice.surrender.1': 'Późne oddanie',
  'option.insurance': 'Ubezpieczenie',
  'choice.insurance.1': 'Oferowane',
  'choice.insurance.0': 'Nieoferowane',

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
  'verb.bet': 'Postaw',
  'verb.call': 'Sprawdzam',
  'verb.check': 'Czekam',
  'verb.commit': 'Gotowe',
  'verb.continue': 'Dalej',
  'verb.decline_insurance': 'Bez ubezpieczenia',
  'verb.discard': 'Odrzuć',
  'verb.double': 'Podwój',
  'verb.draw': 'Dobierz',
  'verb.finish_layoff': 'Koniec dokładania',
  'verb.fold': 'Pasuję',
  'verb.hit': 'Dobierz',
  'verb.insure': 'Ubezpiecz',
  'verb.knock': 'Zapukaj',
  'verb.lay_meld': 'Wyłóż',
  'verb.lay_off': 'Dołóż',
  'verb.pass': 'Pas',
  'verb.place': 'Połóż',
  'verb.play_card': 'Zagraj',
  'verb.raise': 'Podbijam',
  'verb.reset_turn': 'Cofnij kolejkę',
  'verb.split': 'Podziel',
  'verb.stand': 'Zostaję',
  'verb.surrender': 'Oddaj',
  'verb.swap_joker': 'Wymień jokera',
  'verb.take': 'Weź',
  'verb.take_pile': 'Weź ze stosu',
  // Declared by the server and listed in `serverKeys.json`, but never worded.
  'verb.takePileFromHand': 'Weź stos do ręki',
  'verb.takePileOntoMeld': 'Weź stos na układ',
  'verb.takeTopForSequence': 'Weź wierzchnią kartę do sekwensu',
  'verb.undoDraw': 'Cofnij dobranie',
  'verb.undoLayOff': 'Cofnij dołożenie',
  'verb.undoMeld': 'Cofnij układ',
  'verb.undoTakePile': 'Cofnij wzięcie ze stosu',
  'verb.undoTurn': 'Cofnij kolejkę',

  // --- suits, spelled out ---------------------------------------------------
  //
  // Read aloud by a screen reader and shown where a pip would not fit. The
  // pips themselves are drawn, not written, so these are the only place the
  // suit is ever a word.
  'suit.C': 'Trefl',
  'suit.D': 'Karo',
  'suit.H': 'Kier',
  'suit.S': 'Pik',

  // --- counters the board prints beside a number ----------------------------
  'canasta.seat.notOpened': 'Bez otwarcia',
  'canasta.unit.points': 'pkt',
  'ginrummy.unit.points': 'pkt',
  'holdem.seat.dealer': 'Rozdający',
  'holdem.seat.folded': 'Spasował',
  'holdem.seat.allIn': 'All in',
  'holdem.seat.out': 'Odpadł',
  'holdem.unit.chips': 'żetonów',
  'prsi.unit.cardsLeft': 'kart zostało',
  'rummytiles.prompt.initialMeld': 'Twoje pierwsze wyłożenie musi być warte {n} punktów.',
  'rummytiles.unit.points': 'pkt',
  'zolik.unit.penalty': 'punkty karne',
  'header.pileFrozen': 'Stos zamrożony',
  // The module-less form of `ginrummy.prompt.yourTurnDraw`: Žolíky and Canasta
  // send the prompt without a module prefix. Neither the Go constants nor
  // `serverKeys.json` list it — the sweep is what found it.
  'prompt.yourTurnDraw': 'Dobierz kartę',
  'prompt.yourTurnMeld': 'Wyłóż, jeśli możesz, potem odrzuć',
};
