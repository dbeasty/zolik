/**
 * German. Rummy vocabulary follows the Rommé tradition — Satz for a set, Folge for a run, Auslage for a meld, Talon for the stock — because those are the words a German player already has for this game.
 */

export const de: Record<string, string> = {
  // --- engine error codes (rules.RulesErrorCode) ---------------------------
  'err.NOT_YOUR_TURN': 'Du bist nicht an der Reihe',
  'err.WRONG_PHASE': 'Gerade nicht möglich',
  'err.MUST_DRAW_FIRST': 'Zieh eine Karte, bevor du auslegst',
  'err.GAME_SUSPENDED': 'Das Spiel pausiert',
  'err.GAME_NOT_ACTIVE': 'Das Spiel läuft nicht',
  // --- runtime refusals, which are not the rules saying no -----------------
  //
  // Both were reachable long before they were worded, and a player who met
  // one was shown the code in capitals or, worse, nothing at all: an action
  // sent while the socket was down simply vanished, which reads as a legal
  // move being silently refused.
  'err.MATCH_NOT_ACTIVE': 'Der Tisch pausiert — es wird auf einen Spieler gewartet',
  'err.NOT_CONNECTED': 'Keine Verbindung zum Tisch — verbinde neu und versuch es dann noch einmal',
  // Why the "start the next round" control is greyed out. Without wording the
  // shell printed the code itself, so a player agreeing to go on was answered
  // with ALREADY_READY in capitals.
  'err.ALREADY_READY': 'Du bist bereit',
  'err.NOT_BETWEEN_ROUNDS': 'Die Runde läuft noch',
  'err.NOT_AT_THIS_TABLE': 'Du sitzt nicht an diesem Tisch',
  'err.DISCARD_LOCKED': 'Der Ablagestapel ist vorerst gesperrt',
  'err.DISCARD_PILE_EMPTY': 'Der Ablagestapel ist leer',
  'err.NO_CARDS_LEFT': 'Keine Karten mehr zum Ziehen',
  'err.ROUND_REQ_NOT_MET': 'Leg zuerst deine eigene Erstauslage',
  'err.NEED_CLEAN_RUN': 'Du brauchst eine jokerfreie Folge auf dem Tisch, bevor du als ausgelegt giltst',
  'err.INCOMPLETE_INITIAL_MELD': 'Beende deine Auslage oder nimm sie zurück, bevor du ablegst',
  'err.DISCARD_CARD_NOT_MELDED': 'Die aufgenommene Karte muss in deine Auslage',
  'err.JOKER_DISCARD_FORBIDDEN': 'Ein Joker kann nicht abgelegt werden',
  'err.NOTHING_TO_UNDO': 'Nichts rückgängig zu machen',
  'err.NO_JOKER_IN_MELD': 'Kein Joker in dieser Auslage',
  'err.JOKER_SWAP_MISMATCH': 'Diese Karte tritt nicht an die Stelle des Jokers',
  'err.RECLAIMED_JOKER_NOT_MELDED':
    'Der vom Tisch genommene Joker muss in dieser Runde in eine Auslage gespielt werden',
  'err.RUN_TOO_LONG': 'Diese Folge hat bereits ihre volle Länge',
  'err.WRONG_RUN_END': 'Diese Karte verlängert das andere Ende der Folge',
  'err.INVALID_MELD': 'Keine Karte auf deiner Hand passt hierher',
  'err.CARD_NOT_IN_HAND': 'Diese Karte ist nicht auf deiner Hand',
  'err.MELD_BELOW_MINIMUM': 'Deiner Auslage fehlen noch Punkte zum Auslegen',
  'err.MELD_NO_CONTRIBUTION': 'Diese Auslage bringt deine Anforderung nicht voran',
  'err.TOO_MANY_WILDS': 'Zu viele Joker in dieser Auslage',
  'err.ADJACENT_WILDS': 'Zwei Joker dürfen nicht nebeneinander liegen',
  'err.ACE_BRIDGE': 'Ein Ass kann König und Zwei nicht verbinden',

  // --- contract phrasing ---------------------------------------------------
  // Built from the structured contract the server sends (sets/runs counts),
  // never from a deal-number lookup table.
  // Whole phrases per count rather than a number plus a pluralised noun:
  // Czech inflects the noun by count (skupina / skupiny / skupin) in a way no
  // "add an s" helper survives, and the counts that actually occur are 1-3.
  // A count outside that falls back to contract.sets.n.
  'contract.sets.1': 'Ein Satz',
  'contract.sets.2': 'Zwei Sätze',
  'contract.sets.3': 'Drei Sätze',
  'contract.sets.n': '{n} Sätze',
  'contract.runs.1': 'Eine Folge',
  'contract.runs.2': 'Zwei Folgen',
  'contract.runs.3': 'Drei Folgen',
  'contract.runs.n': '{n} Folgen',
  'contract.any': 'Jede gültige Auslage',
  'contract.cleanRunOnly':
    'Beliebige Mischung aus Sätzen und Folgen — mindestens eine Folge muss jokerfrei sein',
  'contract.cleanRunSuffix': '{base} — eine Folge muss jokerfrei sein',

  // --- written rules ---------------------------------------------------------
  // Full-sentence keys a "see the rules" screen renders, one per module,
  // resolved server-side against the table's actual variation and options —
  // see internal/module/rules.go and each game's rules.go. Shared section
  // titles first, then one block per module in the order the four games were
  // added.
  'zolik.rules.section.goal': 'Ziel',
  'zolik.rules.section.setup': 'Vorbereitung',
  'zolik.rules.section.turn': 'Dein Zug',
  'zolik.rules.section.melding': 'Auslegen',
  'zolik.rules.section.end': 'Wie die Partie endet',
  'zolik.rules.goal':
    'Leere als Erster deine Hand, indem du gültige Sätze und Folgen auslegst — und sammle dabei möglichst wenige Strafpunkte in den Karten, die du noch hältst, wenn jemand anderes hinausgeht.',
  'zolik.rules.deal': 'Jeder Spieler erhält {n} Karten.',
  'zolik.rules.meldShapes':
    'Ein Satz besteht aus {set}+ Karten desselben Werts, eine Folge aus {run}+ aufeinanderfolgenden Karten derselben Farbe.',
  'zolik.rules.turn.draw': 'Zieh in deinem Zug eine Karte — vom Talon oder vom Ablagestapel.',
  'zolik.rules.pickup.topOnly': 'Nur die oberste Karte des Ablagestapels darf genommen werden.',
  'zolik.rules.pickup.anyFromPile':
    'Jede Karte des Ablagestapels darf genommen werden, zusammen mit allem, was darüber liegt.',
  'zolik.rules.pickup.locked': 'Vom Ablagestapel darf erst ab Runde {n} gezogen werden.',
  'zolik.rules.pickup.open': 'Der Ablagestapel ist von der ersten Runde an offen.',
  'zolik.rules.turn.discard': 'Beende deinen Zug, indem du eine Karte ablegst.',
  'zolik.rules.jokers.restricted':
    'Ein Joker darf nie abgelegt werden — außer als genau die Karte, die deine Hand leert.',
  'zolik.rules.lead.rotate':
    'Die Vorhand wandert pro Gabe einen Platz weiter, unabhängig davon, wer gewonnen hat.',
  'zolik.rules.lead.winner': 'Wer hinausgeht, hat in der nächsten Gabe die Vorhand.',
  'zolik.rules.meldFloor.on':
    'Deine erste Auslage (oder Auslagen) muss mindestens {n} natürliche Punkte ergeben, bevor du ausgelegt bist.',
  'zolik.rules.meldFloor.off': 'Für die erste Auslage gibt es keinen Mindestwert.',
  'zolik.rules.cleanRun.on':
    'Mindestens eine deiner Folgen muss völlig jokerfrei sein, bevor du als ausgelegt giltst.',
  'zolik.rules.cleanRun.off': 'Deine Folgen dürfen Joker frei verwenden — keine Folge muss jokerfrei sein.',
  'zolik.rules.contracts.rotating':
    'Die Partie geht über {n} Gaben, und jede Gabe verlangt ihre eigene Kombination aus Sätzen und Folgen.',
  'zolik.rules.contracts.static': 'Jede Gabe verlangt dieselbe Kombination: {sets} Sätze und {runs} Folgen.',
  'zolik.rules.end.afterDeals': 'Die Partie endet nach {n} Gaben.',
  'zolik.rules.end.atScore': 'Es wird weiter gegeben, bis jemand {n} Punkte erreicht — dann ist Schluss.',

  'prsi.rules.section.goal': 'Ziel',
  'prsi.rules.section.setup': 'Vorbereitung',
  'prsi.rules.section.turn': 'Dein Zug',
  'prsi.rules.section.special': 'Sonderkarten',
  'prsi.rules.section.end': 'Wie die Partie endet',
  'prsi.rules.goal': 'Spiel als Erster jede Karte deiner Hand aus.',
  'prsi.rules.deck': 'Gespielt mit einem Blatt aus {value} Karten (ab der 7).',
  'prsi.rules.deal': 'Jeder Spieler beginnt mit {n} Karten.',
  'prsi.rules.turn.match':
    'Spiel eine Karte, die Farbe oder Wert der obersten Karte trifft — oder zieh, wenn du nicht kannst.',
  'prsi.rules.turn.draw': 'Ziehen beendet deinen Zug ohne Ausspielen.',
  'prsi.rules.sevens':
    'Spielst du eine 7, zieht der nächste Spieler zwei Karten — außer er kontert mit einer eigenen 7.',
  'prsi.rules.aces': 'Spielst du ein Ass, wird der nächste Spieler übersprungen.',
  'prsi.rules.queens': 'Spiel eine Dame und nenne die Farbe, die weitergeht.',
  'prsi.rules.end': 'Die Partie endet in dem Moment, in dem jemandes Hand leer ist.',

  'canasta.rules.section.goal': 'Ziel',
  'canasta.rules.section.setup': 'Vorbereitung',
  'canasta.rules.section.melding': 'Auslegen',
  'canasta.rules.section.end': 'Wie die Partie endet',
  'canasta.rules.goal':
    'Gespielt wird in Partnerschaften; die erste Seite mit {n} Punkten gewinnt die Partie.',
  'canasta.rules.deck': 'Gespielt mit {value} Karten — zwei Blätter plus Joker.',
  'canasta.rules.deal': 'Jeder Spieler erhält {n} Karten.',
  'canasta.rules.redThrees':
    'Eine rote Drei auf deiner Hand wird sofort aufgedeckt und zählt als Bonus — außer deine Seite bringt nie eine Canasta zustande, dann zählt sie gegen dich.',
  'canasta.rules.canasta': 'Eine Canasta ist eine Auslage aus {n} oder mehr Karten desselben Werts.',
  'canasta.rules.meldFloorBands':
    'Deine erste Auslage muss einen Mindestwert erreichen, der mit deinem Punktestand steigt: {negative} unter null, {low} bis 1500, {mid} bis 3000, {high} darüber.',
  'canasta.rules.oneCanastaToGoOut': 'Eine fertige Canasta genügt, damit deine Seite hinausgehen darf.',
  'canasta.rules.twoCanastasToGoOut':
    'Deine Seite braucht zwei fertige Canastas, bevor sie hinausgehen darf.',
  'canasta.rules.end':
    'Es wird weiter gegeben, bis eine Seite {n} Punkte überschreitet — dann ist die Partie vorbei.',

  'holdem.rules.section.goal': 'Ziel',
  'holdem.rules.section.setup': 'Vorbereitung',
  'holdem.rules.section.betting': 'Setzen',
  'holdem.rules.section.end': 'Wie die Partie endet',
  'holdem.rules.goal':
    'Gewinne Chips mit dem besten Blatt im Showdown — oder indem du als Einziger im Spiel bleibst.',
  'holdem.rules.stack': 'Jeder Platz beginnt mit {n} Chips.',
  'holdem.rules.blinds':
    'Der Small Blind beträgt {sb}, der Big Blind {bb}; beide werden vor dem Geben gesetzt.',
  'holdem.rules.streets': 'Gesetzt wird in vier Runden — vor dem Flop sowie nach Flop, Turn und River.',
  'holdem.rules.showdown':
    'Wer noch im Spiel ist, deckt auf; das beste Blatt aus fünf Karten gewinnt den Pot.',
  'holdem.rules.noLimit': 'No-Limit — jeder Einsatz darf bis zur Höhe deines gesamten Stacks gehen.',
  'holdem.rules.lastPlayerStanding': 'Gespielt wird, bis ein Platz alle Chips hält.',
  'holdem.rules.mostChipsWins': 'Wer bei Spielende die meisten Chips hält, gewinnt die Partie.',
  'holdem.rules.handLimit': 'Nach {n} Blättern ist Schluss.',

  // --- header --------------------------------------------------------------
  'header.deal': 'Gabe {n}',
  'header.gameOf': 'Spiel {n} von {total}',
  'header.gameOfWithContract': 'Spiel {n} von {total}: {contract}',

  // --- meld preview --------------------------------------------------------
  'preview.validSet': 'Gültiger Satz',
  'preview.validRun': 'Gültige Folge',
  'preview.validMeld': 'Gültige Auslage',
  'preview.notYet': 'Noch keine Auslage',
  'preview.points': '{shape} · {n} Punkte',
  'preview.pointsWithLaid': '{shape} · {n} + {laid} bereits ausgelegt = {total} Punkte',
  'preview.meetsFloor': '{line} (erreicht {n} ✓)',
  'preview.needsFloor': '{line} (braucht {n} ✗)',
  'preview.becauseOf': '{line} — {reason}',

  // --- discard -------------------------------------------------------------
  // A discard with a meld staged lays that meld first; this is what the
  // player is told when the server refuses it and the whole move is rolled
  // back. {reason} is the engine's own words for the refusal.
  'discard.meldRejected': '{reason} — es wurde nichts abgelegt, deine Karten liegen weiterhin bereit.',

  // --- what the current selection would send -------------------------------
  // The client's own reason a control is not ready, next to the engine's own
  // `err.*` reasons rather than a second kind of message — a control greyed
  // out for "not your turn" and one greyed out for "you picked two, this
  // takes one" should read as the same kind of thing, not one of them looking
  // broken. See `fits` in `src/lib/drops.ts`.
  'sel.tooMany.1': 'Wähle nur eine Karte',
  'sel.tooMany.n': 'Wähle höchstens {n} Karten',
  'sel.needMore': 'Wähle {n} Karte(n)',
  'sel.notThese': 'Diese Karten können hier nicht hin',
  'sel.needsCompany': 'Diese Karte braucht die Karten daneben',

  // --- what a module says happened -----------------------------------------
  // The first module-sent keys in this bundle, and deliberately few. Anything
  // a module sends is still legible without an entry here — `humanise` turns
  // `holdem.seat.stack` into "Stack" — so a key earns a line only when that
  // fallback loses something. These do: what each of them means lives in its
  // *params* (who won, how much, whose cards), and a key on its own has
  // nowhere to put them. Without the entry the player read "Winner", full
  // stop, at the end of a match they had just won.
  'status.winner': 'Gewonnen von {winners}',
  'holdem.status.pot': '{winners} gewinnt {amount} mit {hand}',
  'holdem.status.potUncontested': '{winners} gewinnt {amount} — alle anderen sind ausgestiegen',
  'holdem.status.shown': '{playerId} zeigte {value}',
  'holdem.prompt.waitingFor': 'Warten auf {playerId}',

  // How a rummy scoreboard was ordered. Both earn a line for the same reason
  // as the keys above: the number lives in the params, and `humanise` would
  // render "Deals Won" with nothing after it.
  'zolik.standing.dealsWon': 'Gewonnene Gaben {n}',
  'zolik.standing.inHand': 'Auf der Hand {n}',

  // --- rounds ---------------------------------------------------------------
  // What a round is called, and what one did. Each of these carries its number
  // in the params, so `humanise` alone would render a label with nothing after
  // it.
  // An instruction, not a heading. "Next round" named the thing rather than
  // asking for it, and sat above "Waiting for 1" — which reads as though the
  // player is the one waiting, when the table is waiting for them.
  'round.continue': 'Nächste Runde starten',

  // --- the flash that announces a round or a match ending ------------------
  //
  // One line and one number, held for about two seconds. Worded generically on
  // purpose: which *kind* of ending it was is the module's own fact, printed
  // underneath, and a client that phrased "went gin" itself would be a client
  // that knows a game.
  'flash.roundWon': '{winners} hat sie geholt',
  'flash.roundWonYou': 'Du hast sie geholt',
  'flash.roundDrawn': 'Niemand hat sie geholt',
  'flash.matchOver': 'Partie beendet',
  'flash.matchWon': '{winners} gewinnt',
  'flash.matchWonYou': 'Du gewinnst',
  'flash.matchDrawn': 'Niemand gewinnt',
  'flash.nowOn': 'jetzt {total}',

  'zolik.round.deal': 'Gabe',
  'zolik.round.cleanRun': 'Eine Folge muss jokerfrei sein',
  'canasta.round.deal': 'Gabe',
  'canasta.round.concealed': 'Verdeckt hinausgegangen',
  'canasta.round.exhausted': 'Das Blatt ging aus',
  'canasta.round.meldCards': 'Ausgelegte Karten {n}',
  'canasta.round.canastas': 'Canastas {n}',
  'canasta.round.redThrees': 'Rote Dreien {n}',
  'canasta.round.goingOut': 'Hinausgehen {n}',
  'canasta.round.inHand': 'Auf der Hand erwischt {n}',
  'holdem.round.hand': 'Blatt',
  'holdem.round.pot': 'Pot {n}',
  'holdem.round.uncontested': 'Alle anderen sind ausgestiegen',
  'seat.ready': 'Bereit',
  'results.you': '(du)',



  // --- refusals the other three games raise ---------------------------------
  // Worded here for the same reason Žolíky's are: reasonText falls back to the
  // code, and OfferBar hands it the code as that fallback, so an unworded
  // refusal reaches a player as PILE_FROZEN in capitals. serverKeys.test.ts
  // now fails the build rather than letting the next one through.
  'err.SET_TOO_LARGE': 'Ein Satz hat bereits alle vier Farben',
  'err.DISCARD_TAKEN_CARD_FORBIDDEN':
    'Du kannst die eben genommene Karte nicht ablegen — spiel sie aus oder behalte sie',
  'err.CARD_DOES_NOT_FIT': 'Diese Karte passt weder in Farbe noch im Wert',
  'err.SUIT_REQUIRED': 'Nenne die Farbe, die weitergeht',
  'err.MUST_ANSWER_DRAW_OR_TAKE': 'Kontere mit einer Sieben oder nimm die Karten',
  'err.NOTHING_TO_DRAW': 'Es ist nichts mehr zum Ziehen da',
  'err.PILE_EMPTY': 'Der Stapel ist leer',
  'err.PILE_BLOCKED': 'Der Stapel ist blockiert — oben liegt eine schwarze Drei',
  'err.PILE_FROZEN':
    'Der Stapel ist eingefroren — du brauchst zwei natürliche Karten im Wert der obersten Karte',
  'err.TOP_CARD_UNUSABLE': 'Du kannst die oberste Karte nicht verwenden',
  'err.MELD_CLOSED': 'Diese Auslage ist vollständig und geschlossen',
  'err.MELD_TOO_SMALL': 'Eine Auslage braucht mehr Karten als das',
  'err.MELD_TOO_LARGE': 'Diese Auslage kann keine weiteren Karten aufnehmen',
  'err.MELD_MIXED_RANKS': 'Jede Karte einer Auslage muss denselben Wert haben',
  'err.NOT_ENOUGH_NATURALS': 'Eine Auslage braucht mehr natürliche als wilde Karten',
  'err.RANK_ALREADY_MELDED': 'Deine Seite hat bereits eine Auslage dieses Werts',
  'err.NOT_YOUR_MELD': 'Diese Auslage gehört der anderen Seite',
  'err.NO_SUCH_MELD': 'Diese Auslage liegt nicht auf dem Tisch',
  'err.CANNOT_MELD_THREE': 'Dreien werden nie ausgelegt',
  'err.CANNOT_DISCARD_RED_THREE': 'Eine rote Drei kann nicht abgelegt werden',
  'err.MUST_KEEP_A_CARD': 'Behalte mindestens eine Karte — so kannst du deine Hand nicht leeren',
  'err.MUST_MELD_FIRST': 'Leg zuerst die Erstauslage deiner Seite',
  'err.INITIAL_MELD_NOT_MET': 'Deiner ersten Auslage fehlen noch Punkte',
  'err.CANNOT_GO_OUT_YET': 'Deine Seite braucht eine fertige Canasta, bevor sie hinausgehen kann',
  'err.NOTHING_TO_CALL': 'Es gibt keinen Einsatz mitzugehen',
  'err.CANNOT_CHECK': 'Du kannst nicht schieben — es steht ein Einsatz',
  'err.CANNOT_RAISE': 'Hier kannst du nicht erhöhen',
  'err.RAISE_TOO_SMALL': 'Eine Erhöhung muss mindestens so hoch sein wie die letzte',
  'err.NOT_ENOUGH_CHIPS': 'So viele Chips hast du nicht',
  'err.AMOUNT_REQUIRED': 'Sag, wie viel',
  'err.AMOUNT_NOT_A_NUMBER': 'Dieser Betrag ist keine Zahl',
  'err.SEAT_NOT_IN_HAND': 'Du bist an diesem Blatt nicht beteiligt',
  'err.WRONG_RANK': 'Diese Karte hat dafür den falschen Wert',
  'err.MATCH_FULL': 'Der Tisch ist voll',
  'err.MATCH_ALREADY_STARTED': 'Die Partie hat bereits begonnen',
  'err.TOO_FEW_PLAYERS': 'Noch nicht genug Spieler',
  'err.WRONG_PLAYER_COUNT': 'Dieses Spiel lässt sich mit so vielen Spielern nicht spielen',
  'err.NOT_THE_HOST': 'Das kann nur der Gastgeber',
  'err.NO_LONGER_WAITING': 'Der Tisch wartet nicht mehr',
  'err.WAITING_ROOM_UNAVAILABLE': 'Der Warteraum ist nicht verfügbar',
  'err.SERVER_BUSY': 'Der Server ist gerade voll — versuch es gleich noch einmal',


  // --- rules the engine has always enforced and never stated ----------------
  // Every one of these was found by the guardrail rather than by review: the
  // validator could refuse a player for it, and no sentence anywhere said so.
  // See server/internal/zolikmod/ruleindex_test.go.
  'zolik.rules.section.layoff': 'Anlegen',
  'zolik.rules.pickup.obligation':
    'Bevor du ausgelegt bist, muss eine vom Ablagestapel genommene Karte in die Auslage, mit der du in diesem Zug herauskommst.',
  'zolik.rules.pickup.noReturn':
    'Eine vom Ablagestapel genommene Karte darf im selben Zug nicht wieder abgelegt werden — spiel sie aus oder behalte sie.',
  'zolik.rules.wilds.setLimit': 'Ein Satz darf nicht mehr Joker als natürliche Karten enthalten.',
  'zolik.rules.set.maxSize':
    'Ein Satz darf höchstens {n} Karten enthalten — ein Joker ersetzt eine fehlende Farbe, er füllt keine vollständige auf.',
  'zolik.rules.run.maxLength':
    'Eine Folge darf höchstens {n} Karten enthalten — das Ass unten, die zwölf Werte darüber und das Ass oben.',
  'zolik.rules.run.aceBridge':
    'Ein Ass steht über dem König oder unter der Zwei, nie als Brücke zwischen beiden Enden einer Folge.',
  'zolik.rules.contracts.contribution':
    'Bis du ausgelegt bist, muss jede Auslage eine sein, die der Kontrakt der Gabe noch verlangt.',
  'zolik.rules.layoff.afterDown':
    'Du darfst an fremde Auslagen erst anlegen, wenn du deinen eigenen Kontrakt gelegt hast.',
  'zolik.rules.layoff.runEnds':
    'Eine an eine Folge angelegte Karte muss diese an einem der beiden Enden fortsetzen.',
  'zolik.rules.jokers.swap':
    'Ein Joker in einer Auslage auf dem Tisch darf gegen genau die Karte eingetauscht werden, für die er steht.',
  'zolik.rules.jokers.reclaim.on':
    'Ein vom Tisch zurückgekaufter Joker muss im selben Zug in eine Auslage gespielt werden — er darf nicht auf der Hand bleiben.',
  'zolik.rules.jokers.reclaim.off': 'Ein vom Tisch zurückgekaufter Joker darf auf der Hand bleiben.',
  'zolik.rules.deck.reshuffle':
    'Geht der Talon aus, wird der Ablagestapel gemischt und zum neuen Talon; sind beide leer, endet die Gabe.',


  // --- what to do instead ---------------------------------------------------
  // The third layer of a refusal, after the reason and the rule. Sent by the
  // module because it is the only side that knows which card is owed and
  // which way out is on offer; see server/internal/zolikmod/remedy.go.
  'zolik.remedy.meldThePickup': 'Füge {card} deiner Auslage hinzu oder mach das Aufnehmen rückgängig.',
  'zolik.remedy.discardSomethingElse': 'Leg eine andere Karte ab oder spiel {card} in diesem Zug.',
  'zolik.remedy.discardNotAJoker': 'Leg etwas anderes als einen Joker ab.',
  'zolik.remedy.finishOrUndoLayDown': 'Beende deine Auslage oder nimm sie zurück.',
  'zolik.remedy.needMorePoints': 'Dir fehlen noch {n} Punkte, um auslegen zu können.',
  'zolik.remedy.layACleanRun': 'Leg eine Folge ohne Joker.',
  'zolik.remedy.playReclaimedJoker': 'Spiel {card} in eine Auslage oder mach das Nehmen rückgängig.',
  'zolik.remedy.goDownFirst': 'Leg zuerst deine eigenen Auslagen.',
  'zolik.remedy.drawFirst': 'Zieh zuerst eine Karte.',
  'zolik.remedy.drawFromStock': 'Zieh vom Talon — der Ablagestapel öffnet sich in Runde {n}.',
  'zolik.remedy.drawFromStockEmpty': 'Zieh stattdessen vom Talon.',


  // --- keys whose meaning lives in their params -----------------------------
  // The shape fallback renders these with the number missing ("Stack", "Pot"),
  // which is why they earn a line where most keys do not.
  'header.contract': 'Braucht {sets} Sätze und {runs} Folgen',
  'header.contract.cleanRunOnly': 'Braucht eine jokerfreie Folge',
  'header.round': 'Runde {n}',
  'header.deck': 'Talon',
  'header.target': 'Ziel',
  'header.suitInPlay': 'Farbe im Spiel',
  'seat.cards': 'Karten',
  'zolik.offer.meld': 'Auslegen',
  'prompt.pickupMustBeMelded':
    '{value} kam vom Ablagestapel — die Karte muss in die Auslage, mit der du in diesem Zug herauskommst.',
  'prompt.jokerMustBePlayed':
    '{value} kam vom Tisch — die Karte muss in eine Auslage, bevor du deinen Zug beenden kannst.',
  'prompt.initialMeld': 'Die Erstauslage deiner Seite muss {n} Punkte erreichen.',
  'prompt.canastasNeeded': 'Deiner Seite fehlen noch {n} Canastas, bevor sie hinausgehen kann.',
  'prompt.mustDrawOrAnswerSeven': 'Kontere mit einer Sieben oder zieh {n} Karten.',
  'prompt.chooseSuit': 'Wähle die Farbe, die weitergeht',
  'prompt.skipPending': 'Dein Zug wird übersprungen',
  'status.lastDeal': 'Team {team} erzielte {value}',
  'status.teamScore': 'Team {team}: {value}',
  'canasta.offer.rank': 'Wert',
  'canasta.seat.teamScore': 'Teamstand',
  'canasta.seat.canastas': 'Canastas',
  'holdem.header.pot': 'Pot',
  'holdem.header.street': 'Street',
  'holdem.header.hand': 'Blatt',
  'holdem.header.handLimit': 'Blätter insgesamt',
  'holdem.header.blinds': 'Blinds',
  'holdem.cost.call': 'zum Mitgehen',
  'holdem.cost.pot': 'im Pot',
  'holdem.seat.stack': 'Stack',
  'holdem.seat.bet': 'Einsatz',
  'holdem.prompt.yourAction': 'Du bist dran',
  'holdem.prompt.raiseTo': 'Erhöhen auf',
  'holdem.quick.halfPot': '½ Pot',
  'holdem.quick.pot': 'Pot',
  'holdem.quick.allIn': 'All-in',
  'zone.yourHand': 'Deine Hand',
  'zone.opponentHand': 'Seine Hand',
  'zone.drawPile': 'Talon',
  'zone.discardPile': 'Ablagestapel',
  'zone.melds': 'Auslagen',
  'zone.teamMelds': 'Auslagen deiner Seite',
  'zone.redThrees': 'Rote Dreien',
  'zone.board': 'Board',
  'verb.drawFromDeck': 'Ziehen',
  'verb.takeFromDiscard': 'Vom Stapel nehmen',


  // --- the why sheet's own furniture ---------------------------------------
  // The three layers a refusal is explained in. Labels, not sentences: what
  // goes under each is sent by the server or worded above.
  'why.reason': 'Warum nicht',
  'why.rule': 'Die Regel',
  'why.rules': 'Die Regeln',
  'why.remedy': 'Was du tun kannst',
  'why.readTheRules': 'Vollständige Regeln lesen →',
  'why.close': 'Schließen',
  'why.open': 'warum',

  // --- marks on a particular card ------------------------------------------
  // A mark is a refusal that has not happened yet: the rule is enforced at the
  // discard, which is the last possible moment to hear about it, so the card
  // says so while there is still a turn left to act on it.
  'zolik.badge.owedToMeld':
    '{card} kam vom Ablagestapel — die Karte muss in die Auslage, mit der du in diesem Zug herauskommst.',
  'zolik.badge.jokerOwed':
    '{card} kam vom Tisch — die Karte muss in eine Auslage, bevor du deinen Zug beenden kannst.',

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
  'legal.terms': 'Bedingungen',
  // The document's own name, used as the heading above it. Kept apart from
  // `legal.notice.terms`, which is the same words inflected to sit inside a
  // sentence — a heading in the instrumental case reads as a mistake in every
  // Slavic language.
  'legal.terms.title': 'Nutzungsbedingungen',
  'legal.privacy.title': 'Datenschutzerklärung',
  'legal.privacy': 'Datenschutz',
  'legal.source': 'Quellcode',
  'legal.updated': 'Fassung {version}',
  'legal.draft':
    'Entwurf — noch nicht in Kraft. Name, Land und Kontaktadresse des Betreibers sind noch einzutragen.',
  'legal.notice.before': 'Mit dem Spielen stimmst du den ',
  'legal.notice.terms': 'Nutzungsbedingungen',
  'legal.notice.between': ' zu. Was über dich gespeichert wird, steht in der ',
  'legal.notice.privacy': 'Datenschutzerklärung',
  'legal.notice.after': '.',
  // --- gin rummy -------------------------------------------------------------
  'err.UPCARD_DECLINED': 'Diese Karte hast du bereits abgelehnt',
  'err.DEADWOOD_TOO_HIGH': 'Dein Deadwood ist zu hoch zum Klopfen',
  'err.CARD_DOES_NOT_EXTEND_MELD': 'Diese Karte verlängert diese Auslage nicht',
  'ginrummy.rules.setup': 'Vorbereitung',
  'ginrummy.rules.turn': 'Dein Zug',
  'ginrummy.rules.melds': 'Auslagen',
  'ginrummy.rules.knocking': 'Klopfen',
  'ginrummy.rules.bigGin': 'Big Gin',
  'ginrummy.rules.layoff': 'Das Anlegen',
  'ginrummy.rules.deadHand': 'Das tote Blatt',
  'ginrummy.rules.scoring': 'Wertung eines Blatts',
  'ginrummy.rules.match': 'Die Partie gewinnen',
  'ginrummy.rules.lineBonuses': 'Boni in der Abrechnung',
  'ginrummy.rules.deck': 'Gespielt mit einem Blatt aus {value} Karten.',
  'ginrummy.rules.deal': 'Jeder Spieler erhält {value} Karten.',
  'ginrummy.rules.upcard': 'Eine weitere Karte wird aufgedeckt und beginnt den Ablagestapel.',
  'ginrummy.rules.drawDiscard':
    'Zieh in deinem Zug eine Karte — vom Talon oder vom Ablagestapel — und leg dann eine ab.',
  'ginrummy.rules.setsAndRuns':
    'Eine Auslage ist ein Satz aus drei oder vier Karten eines Werts oder eine Folge aus drei oder mehr Karten einer Farbe.',
  'ginrummy.rules.aceLow': 'Das Ass zählt immer niedrig — es gibt keine Folge von Dame bis Ass.',
  'ginrummy.rules.knockLimit': 'Klopfen darfst du, sobald dein Deadwood {n} oder weniger beträgt.',
  'ginrummy.rules.oklahoma': 'Die Klopfgrenze dieses Blatts ergibt sich aus dem Wert der aufgedeckten Karte.',
  'ginrummy.rules.gin': 'Null Deadwood ist Gin — das bestmögliche Klopfen.',
  'ginrummy.rules.bigGinBonus':
    'Elf Karten in Auslagen, ganz ohne Ablage, sind Big Gin und bringen weitere {n} Punkte.',
  'ginrummy.rules.layoffDescription':
    'Nach einem Klopfen, das kein Gin ist, darf dein Gegner sein eigenes Deadwood an deine Auslagen anlegen, bevor die Blätter verglichen werden.',
  'ginrummy.rules.deadHandDescription':
    'Sinkt der Talon auf seine letzten zwei Karten, ohne dass jemand geklopft hat, ist das Blatt tot — niemand punktet, und derselbe Geber gibt erneut.',
  'ginrummy.rules.undercut':
    'Ist das Deadwood deines Gegners nicht höher als deines, unterbietet er dich: er erhält die Differenz plus {n}.',
  'ginrummy.rules.ginBonus': 'Gin bringt das ganze Blatt deines Gegners plus {n}.',
  'ginrummy.rules.target':
    'Wer nach Ende eines Blatts als Erster {n} Punkte überschreitet, gewinnt die Partie.',
  'ginrummy.rules.shutout':
    'Der Partiebonus verdoppelt sich auf {n}, wenn der Verlierer keinen einzigen Punkt erzielt hat.',
  'ginrummy.rules.box': 'Jedes gewonnene Blatt ist am Ende der Partie {n} Punkte wert.',
  'ginrummy.rules.gameBonus': 'Der Sieg in der Partie bringt weitere {n} Punkte.',
  'ginrummy.fact.deadwood': '{value} Deadwood',
  'ginrummy.fact.discardCard': '{value} ablegen',
  'ginrummy.fact.meldCards': 'An {value}',
  'ginrummy.header.hand': 'Blatt {n}',
  'ginrummy.round.kind': '{value}',
  'ginrummy.round.hand': 'Blatt',
  'ginrummy.seat.score': '{value}',
  'ginrummy.seat.dealer': 'Geber',
  'ginrummy.status.knocked': '{playerId} klopfte mit {deadwood} Deadwood',
  'ginrummy.status.gin': '{playerId} ging Gin',
  'ginrummy.status.lastHand': 'Letztes Blatt: {winner} ({kind}, {delta} Punkte)',
  'ginrummy.offer.drawStock': 'Vom Talon ziehen',
  'ginrummy.offer.drawDiscard': 'Vom Ablagestapel ziehen',
  'ginrummy.offer.takeUpcard': 'Die offene Karte nehmen',
  'ginrummy.offer.passUpcard': 'Passen',
  'ginrummy.offer.discard': 'Ablegen',
  'ginrummy.offer.knock': 'Klopfen',
  'ginrummy.offer.gin': 'Gin!',
  'ginrummy.offer.bigGin': 'Big Gin!',
  'ginrummy.offer.layOff': 'Anlegen',
  'ginrummy.offer.finishLayoff': 'Anlegen beendet',
  'ginrummy.zone.knockerHand': 'Geklopftes Blatt',
  'ginrummy.zone.melds': 'Auslagen',
  'ginrummy.prompt.upcardDecision': 'Nimm die offene Karte oder passe',
  'ginrummy.prompt.yourTurnDraw': 'Zieh eine Karte',
  'ginrummy.prompt.yourTurnDiscard': 'Leg ab — oder klopfe, wenn du kannst',
  'ginrummy.prompt.layoff': 'Leg Deadwood an oder beende',

  // --- rummy tiles -------------------------------------------------------------
  'err.TILE_NOT_IN_HAND': 'Dieser Stein ist nicht auf deiner Hand',
  'err.TILE_DOES_NOT_FIT': 'Das passt dort nicht',
  'err.NO_SUCH_SET': 'Diese Gruppe liegt nicht auf dem Tisch',
  'err.INITIAL_MELD_ONLY': 'Vor deiner Erstauslage darfst du nur deine eigenen neuen Gruppen umlegen',
  'err.TABLE_NOT_VALID': 'Der Tisch ist noch nicht gültig',
  'err.TRAY_NOT_EMPTY': 'Du hast noch lose Steine zu platzieren',
  'err.NOTHING_PLAYED': 'Leg mindestens einen Stein, bevor du deinen Zug beendest',
  'err.INITIAL_MELD_TOO_LOW': 'Deine Erstauslage muss 30 Punkte oder mehr wert sein',
  'err.NOT_A_RUN': 'Nur eine Folge lässt sich teilen',
  'err.BAD_SPLIT_POSITION': 'An dieser Stelle lässt sich die Folge nicht teilen',
  'err.NO_JOKER_IN_SET': 'In dieser Gruppe ist kein Joker',
  'err.TILE_JOKER_SWAP_MISMATCH': 'Dieser Stein ist nicht das, wofür der Joker steht',
  'rummytiles.rules.setup': 'Vorbereitung',
  'rummytiles.rules.sets': 'Gruppen und Folgen',
  'rummytiles.rules.initialMeld': 'Die Erstauslage',
  'rummytiles.rules.turn': 'Dein Zug',
  'rummytiles.rules.jokerTaking': 'Einen Joker nehmen',
  'rummytiles.rules.ending': 'Eine Runde beenden',
  'rummytiles.rules.poolExhaustion': 'Wenn der Vorrat leer ist',
  'rummytiles.rules.match': 'Die Partie gewinnen',
  'rummytiles.rules.tiles': 'Gespielt mit {value} Steinen.',
  'rummytiles.rules.dealCount': 'Jeder Spieler erhält {value} Steine.',
  'rummytiles.rules.group':
    'Eine Gruppe besteht aus drei oder vier Steinen derselben Zahl in jeweils verschiedenen Farben.',
  'rummytiles.rules.run': 'Eine Folge besteht aus drei oder mehr aufeinanderfolgenden Zahlen in einer Farbe.',
  'rummytiles.rules.noWrap': 'Auf die 13 folgt nicht wieder die 1.',
  'rummytiles.rules.joker': 'Ein Joker steht für jeden beliebigen Stein.',
  'rummytiles.rules.initialMeldDescription':
    'Solange du nicht in einem einzigen Zug {n} oder mehr Punkte allein aus deiner Hand ausgelegt hast, darfst du nichts anrühren, was bereits auf dem Tisch liegt.',
  'rummytiles.rules.turnDescription':
    'Leg mindestens einen Stein aus deiner Hand, ordne den Tisch dabei frei um, und beende deinen Zug so, dass jede Gruppe auf dem Tisch gültig ist.',
  'rummytiles.rules.noDiscard':
    'Es wird nicht abgelegt — kannst du keinen gültigen Zug beenden, ziehst du stattdessen einen Stein.',
  'rummytiles.rules.jokerTakingDescription':
    'Einen Joker auf dem Tisch darfst du nehmen, indem du ihn durch den Stein ersetzt, für den er steht — und du musst ihn noch in diesem Zug in einer Gruppe verwenden.',
  'rummytiles.rules.goingOut':
    'Wer als Erster keine Steine mehr hat, gewinnt die Runde. Alle anderen erhalten den negierten Wert dessen, was ihnen bleibt; der Gewinner erhält die Summe aller Verluste.',
  'rummytiles.rules.poolExhaustionLowestWins':
    'Ist der Vorrat leer und niemand kann spielen, endet die Runde, und der niedrigste Handwert gewinnt sie.',
  'rummytiles.rules.poolExhaustionNoWinner':
    'Ist der Vorrat leer und niemand kann spielen, endet die Runde ohne Sieger — jede Hand wird einfach gewertet.',
  'rummytiles.rules.target':
    'Wer nach Ende einer Runde als Erster {n} Punkte überschreitet, gewinnt die Partie.',
  'rummytiles.rules.roundLimit': 'Die Partie endet nach {n} Runden — die höchste Punktzahl gewinnt.',
  'rummytiles.fact.setCards': '{value}',
  'rummytiles.header.pool': 'Vorrat {n}',
  'rummytiles.header.round': 'Runde {n}',
  'rummytiles.round.kind': '{value}',
  'rummytiles.round.round': 'Runde',
  'rummytiles.seat.score': '{value}',
  'rummytiles.seat.notOpened': 'Nicht eröffnet',
  'rummytiles.status.lastRound': 'Letzte Runde: {winner} ({kind})',
  'rummytiles.badge.invalid': 'Noch nicht gültig',
  'rummytiles.zone.pool': 'Vorrat',
  'rummytiles.zone.table': 'Tisch',
  'rummytiles.zone.tray': 'Bank',
  'rummytiles.offer.place': 'Platzieren',
  'rummytiles.offer.addFromHand': 'Hinzufügen',
  'rummytiles.offer.addFromTray': 'Von der Bank hinzufügen',
  'rummytiles.offer.take': 'Nehmen',
  'rummytiles.offer.split': 'Teilen',
  'rummytiles.offer.swapJoker': 'Joker tauschen',
  'rummytiles.offer.resetTurn': 'Zug zurücksetzen',
  'rummytiles.offer.commit': 'Fertig',
  'rummytiles.offer.draw': 'Ziehen',
  'rummytiles.param.position': 'Teilen bei',

  // --- blackjack -----------------------------------------------------------
  //
  // The house rules are worded twice over — on and off — because a rule that
  // is *not* in force is still something a player has to be told. "No
  // surrender at this table" is information; silence is a guess.
  'err.BET_BELOW_MINIMUM': 'Das liegt unter dem Tischminimum',
  'err.ALREADY_BET': 'Dein Einsatz steht bereits',
  'err.INSURANCE_CLOSED': 'Gerade gibt es keine Versicherung zu nehmen',
  'err.CANNOT_DOUBLE': 'Dieses Blatt kann nicht verdoppelt werden',
  'err.CANNOT_SPLIT': 'Dieses Blatt kann nicht geteilt werden',
  'err.CANNOT_SURRENDER': 'Dieses Blatt kann nicht aufgegeben werden',

  'blackjack.rules.section.table': 'Der Tisch',
  'blackjack.rules.section.play': 'Ein Blatt spielen',
  'blackjack.rules.section.dealer': 'Der Geber',
  'blackjack.rules.section.end': 'Wie die Partie endet',
  'blackjack.rules.goal':
    'Schlag den Geber, ohne über einundzwanzig zu kommen. Wer darüber kommt, verliert sofort — ganz gleich, was der Geber danach tut.',
  // Counts are their own phrase rather than a number glued to a noun: one
  // deck and six decks inflect differently, and Czech inflects them again.
  'blackjack.rules.decks': 'Blätter im Schlitten: {n}.',
  'blackjack.rules.stack': 'Jeder Platz setzt sich mit {n} Chips hin.',
  'blackjack.rules.minBet': 'Das Tischminimum beträgt {n} Chips.',
  'blackjack.rules.faceUp':
    'Die Spielerkarten werden offen gegeben; der Geber hält eine Karte verdeckt, bis alle gespielt haben.',
  'blackjack.rules.hitStand': 'Zieh so viele Karten, wie du möchtest, oder bleib bei dem, was du hast.',
  'blackjack.rules.aces': 'Ein Ass zählt elf, solange das passt, und sonst eins.',
  'blackjack.rules.blackjack':
    'Ein Ass mit einer Zehnerkarte auf den ersten beiden Karten ist ein Blackjack.',
  'blackjack.rules.pays3to2': 'Ein Blackjack zahlt 3:2.',
  'blackjack.rules.pays6to5': 'Ein Blackjack zahlt 6:5.',
  'blackjack.rules.paysEven': 'Ein Blackjack zahlt eins zu eins.',
  'blackjack.rules.double':
    'Auf deinen ersten beiden Karten darfst du deinen Einsatz verdoppeln und genau eine weitere Karte nehmen.',
  'blackjack.rules.doubleAfterSplit': 'Auch ein Blatt aus einem Split darf verdoppelt werden.',
  'blackjack.rules.noDoubleAfterSplit': 'Ein Blatt aus einem Split darf nicht verdoppelt werden.',
  'blackjack.rules.split':
    'Zwei Karten gleichen Werts dürfen in eigene Blätter geteilt werden, jedes mit eigenem Einsatz — bis zu {n} Mal, für insgesamt {hands} Blätter.',
  'blackjack.rules.noSplit': 'An diesem Tisch werden Paare nicht geteilt.',
  'blackjack.rules.splitAces':
    'Geteilte Asse erhalten je eine Karte und bleiben dann stehen; einundzwanzig auf diesem Weg ist kein Blackjack.',
  'blackjack.rules.surrender':
    'Du darfst dein erstes Blatt für die Hälfte des Einsatzes aufgeben, sobald der Geber auf Blackjack geprüft hat.',
  'blackjack.rules.noSurrender': 'An diesem Tisch können Blätter nicht aufgegeben werden.',
  'blackjack.rules.dealerDraws': 'Der Geber zieht bis siebzehn und bleibt dann stehen.',
  'blackjack.rules.hitsSoft17': 'Der Geber zieht auch bei einer Siebzehn mit Ass.',
  'blackjack.rules.standsSoft17': 'Der Geber bleibt bei einer Siebzehn mit Ass stehen.',
  'blackjack.rules.dealerPeeks':
    'Zeigt der Geber ein Ass oder eine Zehn, prüft er auf Blackjack, bevor jemand spielt.',
  'blackjack.rules.insurance':
    'Gegen ein Geberass darfst du dich für die Hälfte deines Einsatzes versichern; das zahlt 2:1, wenn der Geber Blackjack hat.',
  'blackjack.rules.noInsurance': 'An diesem Tisch wird keine Versicherung angeboten.',
  'blackjack.rules.rounds': 'Am Tisch werden {n} Runden gespielt.',
  'blackjack.rules.mostChipsWins': 'Wer am Ende die meisten Chips hält, gewinnt die Partie.',
  'blackjack.rules.bustedOut':
    'Ein Platz, der das Minimum von {n} nicht mehr aufbringt, setzt für den Rest der Partie aus.',

  'blackjack.zone.dealer': 'Geber',
  'blackjack.zone.box': 'Blatt',
  'blackjack.zone.yourBox': 'Dein Blatt',
  'blackjack.zone.shoe': 'Schlitten',

  'blackjack.header.round': 'Runde {n} von {of}',
  'blackjack.header.minBet': 'Minimum',
  'blackjack.header.decks': 'Blätter',
  'blackjack.header.dealerTotal': 'Geber zeigt {n}',
  'blackjack.header.dealerSoftTotal': 'Geber zeigt weich {n}',

  'blackjack.seat.stack': 'Chips',
  'blackjack.seat.bet': 'Einsatz',
  'blackjack.seat.insurance': 'Versicherung',
  'blackjack.seat.total': 'Gesamt',
  'blackjack.seat.softTotal': 'Weiche Summe',
  'blackjack.seat.out': 'Keine Chips mehr',

  'blackjack.prompt.placeBet': 'Setz deinen Einsatz',
  'blackjack.prompt.insurance': 'Versicherung?',
  'blackjack.prompt.yourMove': 'Du bist dran',
  'blackjack.prompt.waitingFor': 'Warten auf {playerId}',
  'blackjack.prompt.betAmount': 'Einsatz',

  'blackjack.quick.doubleMin': '2× Minimum',
  'blackjack.quick.allIn': 'All-in',
  'blackjack.offer.bet': 'Setzen',
  'blackjack.offer.hit': 'Karte',
  'blackjack.offer.stand': 'Stehen bleiben',
  'blackjack.offer.double': 'Verdoppeln',
  'blackjack.offer.split': 'Teilen',
  'blackjack.offer.surrender': 'Aufgeben',
  'blackjack.offer.insure': 'Versichern',
  'blackjack.offer.declineInsurance': 'Keine Versicherung',

  'blackjack.fact.tableMinimum': 'Minimum',
  'blackjack.fact.insuranceCost': 'zum Versichern',
  'blackjack.fact.extraStake': 'als Einsatz',
  'blackjack.fact.surrenderReturn': 'zurück',

  'blackjack.status.dealerBlackjack': 'Der Geber hatte Blackjack',
  'blackjack.status.dealerBust': 'Der Geber hat sich mit {n} überkauft',
  'blackjack.status.dealerStands': 'Der Geber bleibt bei {n} stehen',

  'blackjack.round.name': 'Runde',
  'blackjack.round.dealerTotal': 'Geber {n}',
  'blackjack.round.dealerBust': 'Geber überkauft ({n})',
  'blackjack.round.dealerBlackjack': 'Geber-Blackjack',
  'blackjack.round.outcome.blackjack': 'Blackjack',
  'blackjack.round.outcome.win': 'Gewonnen',
  'blackjack.round.outcome.push': 'Unentschieden',
  'blackjack.round.outcome.lose': 'Verloren',
  'blackjack.round.outcome.bust': 'Überkauft',
  'blackjack.round.outcome.surrender': 'Aufgegeben',

  'blackjack.badge.inPlay': 'Im Spiel',
  'blackjack.badge.doubled': 'Verdoppelt',
  'blackjack.badge.split': 'Geteilt',
  'blackjack.badge.blackjack': 'Blackjack',
  'blackjack.badge.bust': 'Überkauft',
  'blackjack.badge.won': 'Gewonnen',
  'blackjack.badge.push': 'Unentschieden',
  'blackjack.badge.lost': 'Verloren',
  'blackjack.badge.surrendered': 'Aufgegeben',

  'blackjack.unit.chips': 'Chips',

  // --- the settings screen --------------------------------------------------
  //
  // Hardcoded English until now, which was survivable while the only other
  // language was unreachable and faintly absurd once the language picker
  // itself lived on this screen: a player who cannot read "Settings" is
  // exactly the one who came here to change it.
  'settings.title': 'Einstellungen',
  'settings.signedInAs': 'Angemeldet als {username}',
  'settings.playingAsGuest': 'Du spielst als {username} (Gast)',
  'settings.notSignedIn':
    'Nicht angemeldet — melde dich an oder spiele als Gast weiter, um online zu spielen.',
  'settings.subtitle': 'Wie du aussiehst — und wie der Tisch',
  'settings.face.heading': 'Dein Gesicht am Tisch',
  'settings.face.account': 'Wird bei deinem Konto gespeichert und folgt dir auf andere Geräte.',
  'settings.face.device': 'Wird auf diesem Gerät gespeichert. Melde dich an, um es mitzunehmen.',
  'settings.skin.heading': 'Aussehen des Tisches',
  'settings.language.heading': 'Sprache',
  'settings.language.status': 'Wird auf diesem Gerät gespeichert.',
  'settings.language.auto': 'Automatisch',
  'settings.language.auto.now': 'Folgt deinem Gerät — derzeit {language}',
  'settings.legal.heading': 'Das Kleingedruckte',
  'settings.legal.status': 'Wozu du dich mit dem Spielen bereit erklärst und was über dich gespeichert wird.',
  'settings.signIn': 'Anmelden',
  'settings.back': 'Zurück',

  // A translated interface over an English notice is a worse lie than an
  // English interface, so the document says so itself rather than letting the
  // surrounding screen imply the text was written for this reader.
  'legal.untranslated':
    'Dieser Hinweis wurde noch nicht in deine Sprache übersetzt. Maßgeblich ist die englische Fassung unten.',

  // --- the navigation bar's screen titles ------------------------------------
  //
  // These sit in `app/_layout.tsx`, above every screen, and were the last
  // English left after the settings screen was translated — a German player
  // reading "Einstellungen" under a bar that said "Settings". A translated app
  // with an untranslated chrome bar looks like a bug, not like a limit.
  'nav.home': 'Zolik',
  'nav.emailSignIn': 'Anmeldung per E-Mail',
  'nav.signingIn': 'Anmeldung läuft',
  'nav.usernameSignIn': 'Anmeldung mit Benutzername',
  'nav.legacyAccount': 'Altes Konto',
  'nav.guest': 'Gast',
  'nav.account': 'Konto',
  'nav.games': 'Spiele',
  'nav.table': 'Dein Tisch',
  'nav.join': 'Einem Tisch beitreten',
  // The screen a shared invite link lands on while it resolves the code.
  'nav.joining': 'Beitreten läuft',
  'nav.rules': 'Regeln',
  'nav.match': 'Partie',
  'nav.scoreTable': 'Punktetabelle',
  'nav.stats': 'Statistik',
  'nav.more': 'Mehr',

  // --- the account menu behind the face in the corner ----------------------
  'menu.label': 'Kontomenü',
  'menu.signedIn': 'Angemeldet',
  'menu.notSignedIn': 'Nicht angemeldet',
  'menu.keepStats': 'um deine Statistik zu behalten',
  'menu.signOut': 'Abmelden',
  'more.scoreTable': 'Offline-Punktetabelle',
  'more.stats': 'Statistik und Rangliste',
  'more.needsAccount': 'zum Nutzen anmelden',
  'gate.title': 'Melde dich an, um das zu nutzen',
  'gate.body':
    'Punktetabellen und Statistiken werden bei deinem Konto gespeichert und folgen dir auf ein anderes Gerät. Ein Gast hat keinen Ort dafür.',

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
  'error.generic': 'Das hat nicht geklappt',
  'error.signIn': 'Anmeldung fehlgeschlagen',
  'error.login': 'Anmeldung fehlgeschlagen',
  'error.register': 'Registrierung fehlgeschlagen',
  'error.sendCode': 'Code konnte nicht gesendet werden',
  'error.badCode': 'Dieser Code hat nicht funktioniert',
  'error.rulesLoad': 'Die Regeln konnten nicht geladen werden',
  'error.createFailed': 'Anlegen fehlgeschlagen',
  'error.saveFailed': 'Speichern fehlgeschlagen',
  'error.exportFailed': 'Export fehlgeschlagen',

  // --- the route that does not exist ----------------------------------------
  'notFound.title': 'Hoppla!',
  'notFound.message': 'Diese Seite gibt es nicht.',
  'notFound.home': 'Zur Startseite!',

  // --- signing in -----------------------------------------------------------
  'auth.login.subtitle': 'Behalte deine Statistiken auf allen Geräten',
  'auth.login.continueWithEmail': 'Mit E-Mail fortfahren',
  'auth.login.usernameInstead': 'Stattdessen mit Benutzernamen anmelden',
  'auth.email.title': 'Mit E-Mail anmelden',
  'auth.email.subtitle': 'Wir schicken dir einen Einmalcode per E-Mail',
  'auth.email.address': 'E-Mail-Adresse',
  'auth.email.send': 'Code senden',
  'auth.email.codeTitle': 'Code eingeben',
  'auth.email.codePlaceholder': '6-stelliger Code',
  'auth.email.differentAddress': 'Andere Adresse verwenden',
  'auth.email.sentTo': 'Gesendet an {email}',
  'auth.email.continue': 'Weiter',
  'auth.guest.title': 'Als Gast spielen',
  'auth.guest.subtitle': 'Kein Konto nötig',
  'auth.guest.displayName': 'Anzeigename',
  'auth.register.title': 'Konto erstellen',
  'auth.register.username': 'Benutzername',
  'auth.register.email': 'E-Mail (optional)',
  'auth.register.password': 'Passwort',
  'auth.username.createAccount': 'Konto mit Benutzername und Passwort erstellen',
  'auth.callback.signedIn': 'Angemeldet.',

  // --- the account screen ---------------------------------------------------
  'account.signInPrompt': 'Melde dich an, um dein Konto zu verwalten.',
  'account.keepGames': 'Diese Spiele behalten',
  'account.signedInWith': 'Angemeldet mit',
  'account.addMethod': 'Anmeldemethode hinzufügen',
  'account.usernameAndPassword': 'Benutzername und Passwort',
  'account.faceAndTable': 'Gesicht und Tischlook',
  'account.refresh': 'Aktualisieren',
  'account.remove': 'Entfernen',

  // --- the main menu --------------------------------------------------------
  'home.subtitle': 'Kontinental-Rommé · {server}',
  'home.playingAs': 'Du spielst als {name}',
  'home.signInPrompt': 'Melde dich an oder spiel als Gast weiter, um online zu spielen.',
  'home.statsAndLeaderboard': 'Statistik & Rangliste',
  'home.play': 'Spielen',
  'home.offlineScoreTable': 'Offline-Punktetabelle',
  'home.signInToKeepStats': 'Anmelden und Statistik behalten',
  'home.signOut': 'Abmelden',
  'home.continueAsGuest': 'Als Gast fortfahren',
  // Someone playing as a guest, marked in a list of names.
  'home.guestSuffix': '(Gast)',

  // --- the waiting room on the main menu ------------------------------------
  //
  // Counts are whole phrases per count, not a number glued to a noun — the
  // same reason `countLabel` exists. One and many are the cases the interface
  // actually produces, so they are the cases that get their own wording.
  'waiting.checking': 'Schauen, wer da ist …',
  'waiting.youAreWaiting': 'Du wartest aufs Spielen',
  'waiting.pickedUp':
    'Wer einen Tisch eröffnet, kann dich dazunehmen — einen Code von dir braucht dafür niemand.',
  'waiting.othersOne': '1 weiterer Spieler wartet ebenfalls',
  'waiting.othersMany': '{n} weitere Spieler warten ebenfalls',
  'waiting.oneWaiting': '1 Spieler wartet aufs Spielen',
  'waiting.manyWaiting': '{n} Spieler warten aufs Spielen',
  'waiting.adding': 'Du wirst zur Warteliste hinzugefügt …',
  'waiting.slowHint':
    'Wenn das nicht in ein paar Sekunden fertig ist, prüfe, ob die Serveradresse unten von diesem Gerät aus erreichbar ist.',
  'waiting.serverBusyDetail': 'Versuch {n}. Der Server nimmt gerade keine neuen Warteraum-Verbindungen an.',
  'waiting.reconnecting': 'Verbindung verloren — neu verbinden …',
  'waiting.reconnectingDetail':
    'Versuch {n}. Das kann passieren, wenn sich das Netzwerk deines Geräts geändert hat oder der Server neu gestartet wurde.',
  'waiting.tryAgain': 'Jetzt erneut versuchen',
  'waiting.makeAvailable': 'Mich zum Spielen bereitstellen',
  'waiting.stop': 'Warten beenden',
  'waiting.noneYet':
    'Gerade wartet niemand aufs Spielen. Setz dich auf die Liste, dann bist du der Erste, den jemand sieht.',
  'waiting.noOthersYet': 'Sonst wartet noch niemand. Gastgeber sehen dich trotzdem und können dich einladen.',
  'waiting.server': 'Server',
  'waiting.none': 'Gerade wartet niemand. Wer sich im Hauptmenü bereitstellt, taucht hier auf.',

  // --- landing on a shared invite link --------------------------------------
  'join.missingCode': 'Diesem Link fehlt der Tischcode.',
  'join.staleLink': 'Bitte um einen frischen Link, oder tritt stattdessen mit dem Code bei.',
  'join.enterCode': 'Code eingeben',
  'join.backToMenu': 'Zurück zum Menü',
  'join.takingSeat': 'Platz nehmen …',
  'join.takingSeatAt': 'Platz nehmen bei {game} …',

  // --- the lobby ------------------------------------------------------------
  'lobby.games.subtitle': 'Alles, was dieser Server anbieten kann',
  'lobby.games.bots': 'Bots',
  'lobby.games.playBot': 'Gegen einen Bot spielen',
  'lobby.games.playBots': 'Gegen {n} Bots spielen',
  'lobby.games.openTable': 'Tisch eröffnen',
  'lobby.games.players': '{n} Spieler',
  'lobby.games.playerRange': '{min}–{max} Spieler',
  'lobby.join.placeholder': 'Beitrittscode oder Einladungslink',
  'lobby.join.needCode': 'Gib einen Beitrittscode, einen Link oder eine Partie-ID ein',
  'lobby.games.signInFirst': 'Melde dich zuerst an',
  'lobby.join.action': 'Beitreten',
  'lobby.join.waitingTitle': 'Warten auf den Gastgeber',
  // Two whole sentences rather than one with a swapped noun: "a game of {game}"
  // does not survive a language that inflects the game's name after "of".
  'lobby.join.joinedGame': 'Du bist einer Partie {game} beigetreten — Start steht noch aus',
  'lobby.join.joinedTable': 'Du bist dem Tisch beigetreten — Start steht noch aus',
  'lobby.table.addBot': 'Bot hinzufügen',
  'lobby.table.start': 'Starten',
  'lobby.table.waitingForHost': 'Warten, bis der Gastgeber startet …',

  // --- inviting someone to a table ------------------------------------------
  'invite.heading': 'Spieler einladen',
  'invite.explain': 'Schick diesen Link. Wer ihn öffnet, landet an diesem Tisch — ganz ohne Konto.',
  'invite.noAddress': 'Für diesen Server ist keine teilbare Adresse hinterlegt, nutze also den Code unten.',
  'invite.readOutCode': 'Oder sag den Code an:',
  'invite.copy': 'Link kopieren',
  'invite.share': 'Link teilen',
  'invite.copied': 'Kopiert!',
  'invite.shared': 'Geteilt',

  // --- the match screen -----------------------------------------------------
  'match.waitingForTable': 'Warten auf den Tisch …',
  'match.waitingForPlayer': 'Warten auf einen anderen Spieler …',
  'match.nobodyWon': 'Niemand hat gewonnen.',
  'match.youWon': 'Du hast gewonnen.',
  'match.finished': 'Diese Partie ist beendet.',
  'match.inProgress': 'Partie läuft — alles ist verbunden und läuft normal.',
  'match.controls': 'Steuerung',
  'match.over': 'Partie beendet',
  'match.settingUp': 'Wird vorbereitet …',
  'match.playAgain': 'Nochmal spielen',
  'match.backToGames': 'Zurück zu den Spielen',
  'match.table': 'Tisch',
  'match.opponents': 'Gegner',
  // Marks which seat is the reader's own, in a list of seats.
  'match.youSuffix': '(du)',
  // The winner line is composed from whole sentences rather than a name glued
  // to " won." — the verb agrees with the subject in most of these languages,
  // and a suffix cannot know that.
  'match.you': 'du',
  'match.someoneWon': '{name} hat gewonnen.',
  'match.wonBy': 'Gewonnen von {names}.',
  'match.pausedFor': 'Pausiert — warten, bis {name} sich neu verbindet.',
  'match.results': 'Ergebnisse',
  'match.players': 'Spieler',
  'match.toPlay': 'am Zug',

  // --- the offline score table ----------------------------------------------
  'scoring.namesHint': 'Namen mit Komma getrennt (4–8 Spieler)',
  'scoring.newSession': 'Neue Sitzung',
  // A worked example, not a sentence: the names are placeholders a translator
  // may localise, the shape `name:score,` is what the parser needs.
  'scoring.scoresPlaceholder': 'Anna:120,Bernd:80,…',
  'scoring.saveRound': 'Runde speichern',
  'scoring.export': 'Punktezettel exportieren',
  'scoring.formatHint': 'Format der Punkte: Name:100,Name2:50',
  'scoring.nameCountError': 'Gib 2–8 durch Komma getrennte Spielernamen ein',
  'scoring.session': 'Sitzung: {id}',
  'scoring.players': 'Spieler: {names}',
  'scoring.roundScores': 'Punkte für Runde {n}',
  'stats.loading': 'Wird geladen …',
  // The figures could not be fetched. `{reason}` is whatever the server or the
  // network said, which is not ours to translate — the frame around it is.
  'stats.unavailable': '(nicht verfügbar: {reason})',

  // --- statistics -----------------------------------------------------------
  'stats.title': 'Statistik & Rangliste',
  'stats.yours': 'Deine Statistik',
  'stats.leaderboard': 'Rangliste',

  // --- a player's lifetime record, shown beside a finished match ------------
  'record.title': 'Deine Bilanz',
  'record.guest':
    'Du spielst als Gast, deshalb wird keine Bilanz geführt. Melde dich an, und die Partien, die du auf diesem Gerät schon gespielt hast — auch diese hier — werden deinem Konto zugeordnet.',
  'record.signInToKeep': 'Anmelden und behalten',
  'record.failed': 'Deine Bilanz konnte gerade nicht geladen werden. Die Partie ist sicher gespeichert.',
  'record.loading': 'Wird geladen …',
  'record.played': 'Gespielt',
  'record.won': 'Gewonnen',
  'record.lost': 'Verloren',
  'record.winRate': 'Siegquote',
  'record.streak': 'Serie',
  'record.atThisGame': 'In diesem Spiel',
  // A streak is a count with a noun, so it is a whole phrase per count for
  // the same reason `countLabel` is: "1 win" / "2 wins" is an English rule,
  // and most of these languages do not share it.
  'record.streakNone': '—',
  'record.streakWinOne': '1 Sieg',
  'record.streakWinMany': '{n} Siege',
  'record.streakLossOne': '1 Niederlage',
  'record.streakLossMany': '{n} Niederlagen',

  // --- moving cards ---------------------------------------------------------
  'hand.dragHint':
    'Zieh eine Karte entlang des Fächers, um sie umzusortieren, oder auf das Board, um sie auszuspielen',
  'hand.moveLeft': 'Nach links',
  'hand.moveRight': 'Nach rechts',
  'zone.collapseGroup': 'Diese Gruppe einklappen',
  'zone.expandGroup': 'Alle Karten dieser Gruppe zeigen',
  'zone.dropHere': 'Hier ablegen',
  'offer.pickCards': 'wähl Karten für die Stelle, die du angetippt hast',
  'offer.ambiguous': 'das passt an mehrere Stellen — wähl auf dem Board',

  // --- the build footer -----------------------------------------------------
  'build.app': 'App',
  'build.server': 'Server',
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
  'option.pauseBetweenRounds': 'Pause zwischen Runden',
  'choice.pauseBetweenRounds.1': 'Pause',
  'choice.pauseBetweenRounds.0': 'Ohne Pause weiter',
  'option.botSkill': 'Gegner',
  'choice.botSkill.0': 'Gemischt',
  'choice.botSkill.1': 'Leicht',
  'choice.botSkill.2': 'Mittel',
  'choice.botSkill.3': 'Schwer',
  'option.initialMeldMinimum': 'Auslagewert',
  'choice.initialMeldMinimum.0': 'Aus',
  'option.discardDrawMinRound': 'Aufnahme vom Ablagestapel',
  'choice.discardDrawMinRound.0': 'Offen',
  'choice.discardDrawMinRound.2': 'Ab Runde 2',
  'choice.discardDrawMinRound.3': 'Ab Runde 3',
  'option.requireCleanRun': 'Jokerfreie Folge',
  'choice.requireCleanRun.1': 'Erforderlich',
  'choice.requireCleanRun.0': 'Aus',
  'option.jokerReclaimMustPlay': 'Zurückgekaufter Joker',
  'choice.jokerReclaimMustPlay.1': 'Im selben Zug spielen',
  'choice.jokerReclaimMustPlay.0': 'Darf behalten werden',
  'option.dealStarter': 'Vorhand',
  'choice.dealStarter.0': 'Reihum',
  'choice.dealStarter.1': 'Sieger beginnt',
  'variation.prsi.classic': 'Klassisch',
  'option.handSize': 'Ausgeteilte Karten',
  'variation.canasta.classic': 'Klassisch',
  'variation.canasta.modern_american': 'Modern American',
  'option.targetScore': 'Zielpunktzahl',
  'option.canastasToGoOut': 'Canastas zum Hinausgehen',
  'variation.holdem.freezeout': 'Freezeout',
  'variation.holdem.timed': 'Feste Blattzahl',
  'option.startingStack': 'Startchips',
  'option.bigBlind': 'Big Blind',
  'option.handLimit': 'Blätter',
  'choice.handLimit.0': 'Bis nur ein Platz übrig ist',
  'variation.ginrummy.standard': 'Standard',
  'option.knockLimit': 'Klopfgrenze',
  'choice.knockLimit.0': 'Oklahoma (die offene Karte bestimmt sie)',
  'option.bigGin': 'Big Gin',
  'choice.bigGin.0': 'Aus',
  'choice.bigGin.1': 'An (+25)',
  'option.lineBonuses': 'Boni in der Abrechnung',
  'choice.lineBonuses.1': 'An',
  'choice.lineBonuses.0': 'Aus',
  'variation.rummytiles.standard': 'Standard',
  'choice.targetScore.0': 'Aus',
  // Two games qualify a number the others mean plainly: Canasta's 500 is a
  // short game, Gin Rummy's and Rummy Tiles' 500 is just 500; Hold'em's
  // starting 200 is a short stack, Blackjack's 200 is just 200. Module-scoped
  // so the qualifier travels with the game that means it — a bare key would
  // hand the note to games it is false for. `scripts/check-server-labels.js`
  // is what finds these; both were shipping as English on a Czech screen.
  'choice.canasta.targetScore.500': '500 (kurz)',
  'choice.holdem.startingStack.200': '200 (kurz)',
  'option.roundLimit': 'Rundenlimit',
  'choice.roundLimit.0': 'Aus',
  'option.poolExhaustion': 'Wenn der Vorrat leer ist',
  'choice.poolExhaustion.1': 'Niedrigste Hand gewinnt die Runde',
  'choice.poolExhaustion.0': 'Niemand gewinnt die Runde',
  'variation.blackjack.single': 'Ein Blatt',
  'option.minBet': 'Tischminimum',
  'option.rounds': 'Runden',
  'option.decks': 'Blätter',
  'option.dealerHitsSoft17': 'Geber bei weicher 17',
  'choice.dealerHitsSoft17.0': 'Bleibt stehen',
  'choice.dealerHitsSoft17.1': 'Zieht',
  'option.blackjackPays': 'Blackjack zahlt',
  'choice.blackjackPays.100': 'Eins zu eins',
  'option.maxSplits': 'Teilen',
  'choice.maxSplits.0': 'Kein Teilen',
  'choice.maxSplits.1': 'Einmal (zwei Blätter)',
  'choice.maxSplits.3': 'Dreimal (vier Blätter)',
  'option.doubleAfterSplit': 'Verdoppeln nach Teilen',
  'choice.doubleAfterSplit.1': 'Erlaubt',
  'choice.doubleAfterSplit.0': 'Nicht erlaubt',
  'option.surrender': 'Aufgeben',
  'choice.surrender.0': 'Aus',
  'choice.surrender.1': 'Spätes Aufgeben',
  'option.insurance': 'Versicherung',
  'choice.insurance.1': 'Angeboten',
  'choice.insurance.0': 'Nicht angeboten',

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
  'verb.add': 'Hinzufügen',
  'verb.bet': 'Setzen',
  'verb.call': 'Mitgehen',
  'verb.check': 'Schieben',
  'verb.commit': 'Fertig',
  'verb.continue': 'Weiter',
  'verb.decline_insurance': 'Keine Versicherung',
  'verb.discard': 'Ablegen',
  'verb.double': 'Verdoppeln',
  'verb.draw': 'Ziehen',
  'verb.finish_layoff': 'Anlegen beendet',
  'verb.fold': 'Aussteigen',
  'verb.hit': 'Karte',
  'verb.insure': 'Versichern',
  'verb.knock': 'Klopfen',
  'verb.lay_meld': 'Auslegen',
  'verb.lay_off': 'Anlegen',
  'verb.pass': 'Passen',
  'verb.place': 'Platzieren',
  'verb.play_card': 'Ausspielen',
  'verb.raise': 'Erhöhen',
  'verb.reset_turn': 'Zug zurücksetzen',
  'verb.split': 'Teilen',
  'verb.stand': 'Stehen bleiben',
  'verb.surrender': 'Aufgeben',
  'verb.swap_joker': 'Joker tauschen',
  'verb.take': 'Nehmen',
  'verb.take_pile': 'Vom Stapel nehmen',
  // Declared by the server and listed in `serverKeys.json`, but never worded.
  'verb.takePileFromHand': 'Stapel auf die Hand nehmen',
  'verb.takePileOntoMeld': 'Stapel an eine Auslage nehmen',
  'verb.undoDraw': 'Ziehen rückgängig',
  'verb.undoLayOff': 'Anlegen rückgängig',
  'verb.undoMeld': 'Auslage rückgängig',
  'verb.undoTurn': 'Zug rückgängig',

  // --- suits, spelled out ---------------------------------------------------
  //
  // Read aloud by a screen reader and shown where a pip would not fit. The
  // pips themselves are drawn, not written, so these are the only place the
  // suit is ever a word.
  'suit.C': 'Kreuz',
  'suit.D': 'Karo',
  'suit.H': 'Herz',
  'suit.S': 'Pik',

  // --- counters the board prints beside a number ----------------------------
  'canasta.seat.notOpened': 'Nicht eröffnet',
  'canasta.unit.points': 'Punkte',
  'ginrummy.unit.points': 'Punkte',
  'holdem.seat.dealer': 'Geber',
  'holdem.unit.chips': 'Chips',
  'prsi.unit.cardsLeft': 'Karten übrig',
  'rummytiles.prompt.initialMeld': 'Deine erste Auslage muss {n} Punkte wert sein.',
  'rummytiles.unit.points': 'Punkte',
  'zolik.unit.penalty': 'Strafpunkte',
  'header.pileFrozen': 'Stapel eingefroren',
  // The module-less form of `ginrummy.prompt.yourTurnDraw`: Žolíky and Canasta
  // send the prompt without a module prefix. Neither the Go constants nor
  // `serverKeys.json` list it — the sweep is what found it.
  'prompt.yourTurnDraw': 'Zieh eine Karte',
};
