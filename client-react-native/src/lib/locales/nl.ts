/**
 * Dutch. Rummy vocabulary: groep for a set, reeks for a run, combinatie for a meld, trekstapel and aflegstapel for the two piles.
 */

export const nl: Record<string, string> = {
  // --- engine error codes (rules.RulesErrorCode) ---------------------------
  'err.NOT_YOUR_TURN': 'Je bent niet aan de beurt',
  'err.WRONG_PHASE': 'Op dit moment niet mogelijk',
  'err.MUST_DRAW_FIRST': 'Pak eerst een kaart voordat je legt',
  'err.GAME_SUSPENDED': 'Het spel is gepauzeerd',
  'err.GAME_NOT_ACTIVE': 'Het spel loopt niet',
  // --- runtime refusals, which are not the rules saying no -----------------
  //
  // Both were reachable long before they were worded, and a player who met
  // one was shown the code in capitals or, worse, nothing at all: an action
  // sent while the socket was down simply vanished, which reads as a legal
  // move being silently refused.
  'err.MATCH_NOT_ACTIVE': 'De tafel is gepauzeerd — er wordt gewacht tot een speler terugkomt',
  'err.NOT_CONNECTED': 'Geen verbinding met de tafel — opnieuw verbinden, probeer het daarna nog eens',
  // Why the "start the next round" control is greyed out. Without wording the
  // shell printed the code itself, so a player agreeing to go on was answered
  // with ALREADY_READY in capitals.
  'err.ALREADY_READY': 'Je bent al klaar',
  'err.NOT_BETWEEN_ROUNDS': 'De ronde is nog bezig',
  'err.NOT_AT_THIS_TABLE': 'Je zit niet aan deze tafel',
  // Bringing a table back that the sweeper set aside. Refusals a player
  // can actually provoke: a link to a table that is gone, one that was
  // already picked up in another tab, and a table with other people at it.
  'err.MATCH_MOVED_ON': 'De tafel is verdergegaan — laad de pagina opnieuw',
  'err.MATCH_NOT_ABANDONED': 'Deze tafel wacht niet om hervat te worden',
  'err.MATCH_NOT_FOUND': 'Deze tafel bestaat niet meer',
  'err.TABLE_HAS_OTHER_PLAYERS': 'Alleen een tafel waar alle anderen bots zijn kan worden hervat',
  'err.DISCARD_LOCKED': 'De aflegstapel is voorlopig op slot',
  'err.DISCARD_PILE_EMPTY': 'De aflegstapel is leeg',
  'err.NO_CARDS_LEFT': 'Er zijn geen kaarten meer om te pakken',
  'err.ROUND_REQ_NOT_MET': 'Leg eerst je eigen openingsleg',
  'err.NEED_CLEAN_RUN': 'Je hebt een jokervrije reeks op tafel nodig om als uitgelegd te tellen',
  'err.INCOMPLETE_INITIAL_MELD': 'Maak je leg af, of draai hem terug, voordat je aflegt',
  'err.DISCARD_CARD_NOT_MELDED': 'De kaart die je pakte moet in je combinatie',
  'err.JOKER_DISCARD_FORBIDDEN': 'Een joker mag niet afgelegd worden',
  'err.NOTHING_TO_UNDO': 'Er is niets om ongedaan te maken',
  'err.NO_JOKER_IN_MELD': 'Geen joker in deze combinatie',
  'err.JOKER_SWAP_MISMATCH': 'Die kaart neemt de plaats van de joker niet in',
  'err.RECLAIMED_JOKER_NOT_MELDED':
    'De joker die je van tafel nam moet deze beurt in een combinatie gespeeld worden',
  'err.RUN_TOO_LONG': 'Die reeks heeft haar volledige lengte al',
  'err.WRONG_RUN_END': 'Die kaart verlengt het andere uiteinde van de reeks',
  'err.INVALID_MELD': 'Geen enkele kaart in je hand past hier',
  'err.CARD_NOT_IN_HAND': 'Die kaart zit niet in je hand',
  'err.MELD_BELOW_MINIMUM': 'Je combinaties komen nog punten tekort om uit te leggen',
  'err.MELD_NO_CONTRIBUTION': 'Die combinatie brengt je opdracht niet verder',
  'err.TOO_MANY_WILDS': 'Te veel jokers in die combinatie',
  'err.ADJACENT_WILDS': 'Twee jokers mogen niet naast elkaar liggen',
  'err.ACE_BRIDGE': 'Een aas kan heer en twee niet overbruggen',

  // --- contract phrasing ---------------------------------------------------
  // Built from the structured contract the server sends (sets/runs counts),
  // never from a deal-number lookup table.
  // Whole phrases per count rather than a number plus a pluralised noun:
  // Czech inflects the noun by count (skupina / skupiny / skupin) in a way no
  // "add an s" helper survives, and the counts that actually occur are 1-3.
  // A count outside that falls back to contract.sets.n.
  'contract.sets.1': 'Eén groep',
  'contract.sets.2': 'Twee groepen',
  'contract.sets.3': 'Drie groepen',
  'contract.sets.n': '{n} groepen',
  'contract.runs.1': 'Eén reeks',
  'contract.runs.2': 'Twee reeksen',
  'contract.runs.3': 'Drie reeksen',
  'contract.runs.n': '{n} reeksen',
  'contract.any': 'Elke geldige combinatie',
  'contract.cleanRunOnly': 'Elke mix van groepen en reeksen — minstens één reeks moet jokervrij zijn',
  'contract.cleanRunSuffix': '{base} — één reeks moet jokervrij zijn',

  // --- written rules ---------------------------------------------------------
  // Full-sentence keys a "see the rules" screen renders, one per module,
  // resolved server-side against the table's actual variation and options —
  // see internal/module/rules.go and each game's rules.go. Shared section
  // titles first, then one block per module in the order the four games were
  // added.
  'zolik.rules.section.goal': 'Doel',
  'zolik.rules.section.setup': 'Opzet',
  'zolik.rules.section.turn': 'Jouw beurt',
  'zolik.rules.section.melding': 'Uitleggen',
  'zolik.rules.section.end': 'Hoe de partij eindigt',
  'zolik.rules.goal':
    'Wees de eerste die zijn hand leegt door geldige groepen en reeksen te leggen, met zo min mogelijk strafpunten in de kaarten die je nog vasthoudt wanneer iemand anders uitgaat.',
  'zolik.rules.deal': 'Elke speler krijgt {n} kaarten.',
  'zolik.rules.meldShapes':
    'Een groep bestaat uit {set}+ kaarten van dezelfde waarde; een reeks uit {run}+ opeenvolgende kaarten van dezelfde kleur.',
  'zolik.rules.turn.draw': 'Pak in jouw beurt één kaart — van de trekstapel of van de aflegstapel.',
  'zolik.rules.pickup.topOnly': 'Alleen de bovenste kaart van de aflegstapel mag gepakt worden.',
  'zolik.rules.pickup.anyFromPile':
    'Elke kaart uit de aflegstapel mag gepakt worden, samen met alles wat erboven ligt.',
  'zolik.rules.pickup.locked': 'Van de aflegstapel mag pas vanaf ronde {n} gepakt worden.',
  'zolik.rules.pickup.open': 'De aflegstapel is vanaf de eerste ronde open.',
  'zolik.rules.turn.discard': 'Beëindig je beurt door één kaart af te leggen.',
  'zolik.rules.jokers.restricted':
    'Een joker mag nooit afgelegd worden, behalve als precies de kaart die je hand leegt.',
  'zolik.rules.lead.rotate': 'De voorhand schuift elke ronde één plaats op, ongeacht wie won.',
  'zolik.rules.lead.winner': 'Wie uitgaat, begint de volgende ronde.',
  'zolik.rules.meldFloor.on':
    'Je eerste leg moet minstens {n} natuurlijke punten opleveren voordat je uitgelegd bent.',
  'zolik.rules.meldFloor.off': 'Er geldt geen minimum aantal punten voor je eerste leg.',
  'zolik.rules.cleanRun.on':
    'Minstens één van je reeksen moet volledig jokervrij zijn voordat je als uitgelegd telt.',
  'zolik.rules.cleanRun.off': 'Je reeksen mogen jokers vrij gebruiken — geen enkele hoeft jokervrij te zijn.',
  'zolik.rules.contracts.rotating':
    'De partij duurt {n} rondes, en elke ronde vraagt haar eigen combinatie van groepen en reeksen.',
  'zolik.rules.contracts.static': 'Elke ronde vraagt dezelfde combinatie: {sets} groepen en {runs} reeksen.',
  'zolik.rules.end.afterDeals': 'De partij eindigt na {n} rondes.',
  'zolik.rules.end.atScore': 'Er wordt doorgedeeld tot iemand {n} punten haalt — dan is het voorbij.',

  'prsi.rules.section.goal': 'Doel',
  'prsi.rules.section.setup': 'Opzet',
  'prsi.rules.section.turn': 'Jouw beurt',
  'prsi.rules.section.special': 'Bijzondere kaarten',
  'prsi.rules.section.end': 'Hoe de partij eindigt',
  'prsi.rules.goal': 'Wees de eerste die elke kaart uit zijn hand speelt.',
  'prsi.rules.deck': 'Gespeeld met een spel van {value} kaarten (vanaf de 7).',
  'prsi.rules.deal': 'Elke speler begint met {n} kaarten.',
  'prsi.rules.turn.match':
    'Speel een kaart die past bij de kleur of de waarde van de bovenste kaart — of pak er een als dat niet lukt.',
  'prsi.rules.turn.draw': 'Pakken beëindigt je beurt zonder te spelen.',
  'prsi.rules.sevens':
    'Speel een 7 en de volgende speler pakt twee kaarten, tenzij die met een eigen 7 antwoordt.',
  'prsi.rules.aces': 'Speel een aas en de beurt van de volgende speler wordt overgeslagen.',
  'prsi.rules.queens': 'Speel een vrouw en noem de kleur die verdergaat.',
  'prsi.rules.end': 'De partij eindigt op het moment dat iemands hand leeg is.',

  'canasta.rules.section.goal': 'Doel',
  'canasta.rules.section.setup': 'Opzet',
  'canasta.rules.section.melding': 'Uitleggen',
  'canasta.rules.section.end': 'Hoe de partij eindigt',
  'canasta.rules.goal':
    'Er wordt in koppels gespeeld; de eerste partij die {n} punten haalt, wint de wedstrijd.',
  'canasta.rules.deck': 'Gespeeld met {value} kaarten — {decks} spellen plus jokers.',
  'canasta.rules.deal': 'Elke speler krijgt {n} kaarten.',
  'canasta.rules.drawCount': 'Je pakt {n} kaarten aan het begin van je beurt.',
  'canasta.rules.redThrees':
    'Een rode drie in je hand wordt meteen getoond en telt als bonus — tenzij jouw partij nooit een canasta afmaakt, dan telt hij tegen je.',
  'canasta.rules.canasta': 'Een canasta is een combinatie van {n} of meer kaarten van dezelfde waarde.',
  'canasta.rules.sequences': 'Een combinatie kan ook een reeks zijn: drie of meer kaarten van dezelfde kleur op volgorde, nooit met een wilde kaart ertussen.',
  'canasta.rules.samba': 'Een reeks van zeven kaarten is een samba en levert {n} punten op.',
  'canasta.rules.blackThreesGoOut':
    'Een zwarte drie blokkeert de stapel en is {n} punten waard. Drie of vier ervan mogen rechtstreeks uit de hand worden gelegd, nooit met een joker ertussen, en alleen als de zet waarmee jouw partij uitgaat.',
  'canasta.rules.blackThreesNeverMeld':
    'Een zwarte drie wordt nooit gelegd. Afgelegd blokkeert hij de stapel, en blijft hij aan het eind van het spel in je hand, dan kost hij {n} punten.',
  'canasta.rules.pileAlwaysFrozen': 'De aflegstapel is het hele spel bevroren: je kunt hem alleen nemen door de bovenste kaart te combineren met twee natuurlijke kaarten uit je hand.',
  'canasta.rules.pileOntoMeld': 'Heeft jouw partij al een onafgemaakte combinatie van de waarde van de bovenste kaart, dan mag je de hele aflegstapel nemen en die kaart eraan toevoegen — een paar in je hand is niet nodig.',
  'canasta.rules.pileNoMeldCapture': 'Een combinatie die al op tafel ligt kan de aflegstapel niet nemen: daarvoor moet je de bovenste kaart combineren met twee kaarten uit je eigen hand.',
  'canasta.rules.meldFloorBands':
    'Je eerste leg moet een puntenminimum halen dat met je stand meestijgt: {negative} onder nul, {low} tot 1500, {mid} tot 3000, {high} daarboven.',
  'canasta.rules.meldFloorBandsFive': 'Je eerste combinatie moet een puntenminimum halen dat met je score meestijgt: {negative} onder nul, {low} tot 1500, {mid} tot 3000, {high} tot 7000 en {top} daarboven.',
  'canasta.rules.oneCanastaToGoOut': 'Eén afgemaakte canasta is genoeg om jouw partij te laten uitgaan.',
  'canasta.rules.twoCanastasToGoOut':
    "Jouw partij heeft twee afgemaakte canasta's nodig voordat ze mag uitgaan.",
  'canasta.rules.end':
    'Er wordt doorgedeeld tot één partij {n} punten passeert — dan is de wedstrijd voorbij.',

  'holdem.rules.section.goal': 'Doel',
  'holdem.rules.section.setup': 'Opzet',
  'holdem.rules.section.betting': 'Inzetten',
  'holdem.rules.section.end': 'Hoe de partij eindigt',
  'holdem.rules.goal':
    'Win fiches met de beste hand bij de showdown, of door als enige speler over te blijven.',
  'holdem.rules.stack': 'Elke plek begint met {n} fiches.',
  'holdem.rules.blinds': 'De small blind is {sb} en de big blind {bb}, ingelegd voordat er gedeeld wordt.',
  'holdem.rules.streets':
    'Er wordt in vier rondes ingezet — vóór de flop en na de flop, de turn en de river.',
  'holdem.rules.showdown':
    'Wie nog in het spel zit, laat zijn kaarten zien; de beste hand van vijf kaarten wint de pot.',
  'holdem.rules.noLimit': 'No-limit — elke inzet mag oplopen tot je hele stapel.',
  'holdem.rules.lastPlayerStanding': 'Er wordt gespeeld tot één plek alle fiches heeft.',
  'holdem.rules.mostChipsWins': 'Wie de meeste fiches heeft als het spel stopt, wint de partij.',
  'holdem.rules.handLimit': 'Het spel stopt na {n} handen.',

  // --- header --------------------------------------------------------------
  'header.deal': 'Ronde {n}',
  'header.gameOf': 'Spel {n} van {total}',
  'header.gameOfWithContract': 'Spel {n} van {total}: {contract}',

  // --- meld preview --------------------------------------------------------
  'preview.validSet': 'Geldige groep',
  'preview.validRun': 'Geldige reeks',
  'preview.validMeld': 'Geldige combinatie',
  'preview.notYet': 'Nog geen combinatie',
  'preview.points': '{shape} · {n} punten',
  'preview.pointsWithLaid': '{shape} · {n} + {laid} al gelegd = {total} punten',
  'preview.meetsFloor': '{line} (haalt {n} ✓)',
  'preview.needsFloor': '{line} (heeft {n} nodig ✗)',
  'preview.becauseOf': '{line} — {reason}',

  // --- discard -------------------------------------------------------------
  // A discard with a meld staged lays that meld first; this is what the
  // player is told when the server refuses it and the whole move is rolled
  // back. {reason} is the engine's own words for the refusal.
  'discard.meldRejected': '{reason} — er is niets afgelegd, je kaarten liggen nog klaar.',

  // --- what the current selection would send -------------------------------
  // The client's own reason a control is not ready, next to the engine's own
  // `err.*` reasons rather than a second kind of message — a control greyed
  // out for "not your turn" and one greyed out for "you picked two, this
  // takes one" should read as the same kind of thing, not one of them looking
  // broken. See `fits` in `src/lib/drops.ts`.
  'sel.tooMany.1': 'Kies maar één kaart',
  'sel.tooMany.n': 'Kies hoogstens {n} kaarten',
  'sel.needMore': 'Kies {n} kaart(en)',
  'sel.notThese': 'Die kaarten kunnen hier niet heen',
  'sel.needsCompany': 'Die kaart heeft de kaarten ernaast nodig',

  // --- what a module says happened -----------------------------------------
  // The first module-sent keys in this bundle, and deliberately few. Anything
  // a module sends is still legible without an entry here — `humanise` turns
  // `holdem.seat.stack` into "Stack" — so a key earns a line only when that
  // fallback loses something. These do: what each of them means lives in its
  // *params* (who won, how much, whose cards), and a key on its own has
  // nowhere to put them. Without the entry the player read "Winner", full
  // stop, at the end of a match they had just won.
  'status.winner': 'Gewonnen door {winners}',
  'holdem.status.pot': '{winners} wint {amount} met {hand}',
  'holdem.status.potUncontested': '{winners} wint {amount} — alle anderen pasten',
  'holdem.status.shown': '{playerId} liet {value} zien',
  'holdem.prompt.waitingFor': 'Wachten op {playerId}',

  // How a rummy scoreboard was ordered. Both earn a line for the same reason
  // as the keys above: the number lives in the params, and `humanise` would
  // render "Deals Won" with nothing after it.
  'zolik.standing.dealsWon': 'Rondes gewonnen {n}',
  'zolik.standing.inHand': 'In de hand {n}',

  // --- rounds ---------------------------------------------------------------
  // What a round is called, and what one did. Each of these carries its number
  // in the params, so `humanise` alone would render a label with nothing after
  // it.
  // An instruction, not a heading. "Next round" named the thing rather than
  // asking for it, and sat above "Waiting for 1" — which reads as though the
  // player is the one waiting, when the table is waiting for them.
  'round.continue': 'Start de volgende ronde',

  // --- the flash that announces a round or a match ending ------------------
  //
  // One line and one number, held for about two seconds. Worded generically on
  // purpose: which *kind* of ending it was is the module's own fact, printed
  // underneath, and a client that phrased "went gin" itself would be a client
  // that knows a game.
  'flash.roundWon': '{winners} pakte hem',
  'flash.roundWonYou': 'Jij pakte hem',
  'flash.roundDrawn': 'Niemand pakte hem',
  'flash.matchOver': 'Partij afgelopen',
  'flash.matchWon': '{winners} wint',
  'flash.matchWonYou': 'Jij wint',
  'flash.matchDrawn': 'Niemand wint',
  'flash.nowOn': 'nu {total}',

  'zolik.round.deal': 'Ronde',
  'zolik.round.cleanRun': 'Eén reeks moet jokervrij zijn',
  'canasta.round.deal': 'Ronde',
  'canasta.round.concealed': 'Verdekt uitgegaan',
  'canasta.round.exhausted': 'Het spel raakte op',
  'canasta.round.meldCards': 'Gelegde kaarten {n}',
  'canasta.round.canastas': "Canasta's {n}",
  'canasta.round.redThrees': 'Rode drieën {n}',
  'canasta.round.goingOut': 'Uitgaan {n}',
  'canasta.round.inHand': 'In de hand betrapt {n}',
  'holdem.round.hand': 'Hand',
  'holdem.round.pot': 'Pot {n}',
  'holdem.round.uncontested': 'Alle anderen pasten',
  'seat.ready': 'Klaar',
  'zolik.seat.contractMet': 'Contract gehaald',
  'results.you': '(jij)',



  // --- refusals the other three games raise ---------------------------------
  // Worded here for the same reason Žolíky's are: reasonText falls back to the
  // code, and OfferBar hands it the code as that fallback, so an unworded
  // refusal reaches a player as PILE_FROZEN in capitals. serverKeys.test.ts
  // now fails the build rather than letting the next one through.
  'err.SET_TOO_LARGE': 'Een groep heeft al alle vier de kleuren',
  'err.DISCARD_TAKEN_CARD_FORBIDDEN':
    'Je mag de kaart die je net pakte niet afleggen — speel hem of houd hem',
  'err.CARD_DOES_NOT_FIT': 'Die kaart past niet qua kleur en niet qua waarde',
  'err.SUIT_REQUIRED': 'Noem de kleur die verdergaat',
  'err.MUST_ANSWER_DRAW_OR_TAKE': 'Antwoord met een zeven, of neem de kaarten',
  'err.NOTHING_TO_DRAW': 'Er is niets meer om te pakken',
  'err.PILE_EMPTY': 'De stapel is leeg',
  'err.PILE_BLOCKED': 'De stapel is geblokkeerd — er ligt een zwarte drie bovenop',
  'err.PILE_FROZEN':
    'De stapel is bevroren — je hebt twee natuurlijke kaarten van de waarde van de bovenste kaart nodig',
  'err.MELD_CAPTURE_NOT_ALLOWED': 'In dit spel kan een combinatie op tafel de stapel niet nemen — je hebt twee kaarten uit je hand nodig',
  'err.TOP_CARD_UNUSABLE': 'Je kunt de bovenste kaart niet gebruiken',
  'err.MELD_CLOSED': 'Die combinatie is compleet en gesloten',
  'err.MELD_TOO_SMALL': 'Een combinatie heeft meer kaarten nodig dan dat',
  'err.MELD_TOO_LARGE': 'Die combinatie kan er geen kaarten meer bij hebben',
  'err.MELD_MIXED_RANKS': 'Elke kaart in een combinatie moet dezelfde waarde hebben',
  'err.SEQUENCE_NO_WILDS': 'Een reeks mag geen wilde kaarten bevatten',
  'err.SEQUENCE_NEEDS_ONE_SUIT': 'Alle kaarten in een reeks moeten dezelfde kleur hebben',
  'err.RUN_NOT_CONSECUTIVE': 'Een reeks moet op volgorde lopen, zonder gaten',
  'err.NOT_ENOUGH_NATURALS': 'Een combinatie heeft meer natuurlijke dan wilde kaarten nodig',
  'err.RANK_ALREADY_MELDED': 'Jouw partij heeft al een combinatie van die waarde',
  'err.NOT_YOUR_MELD': 'Die combinatie is van de tegenpartij',
  'err.NO_SUCH_MELD': 'Die combinatie ligt niet op tafel',
  'err.CANNOT_MELD_THREE': 'Drieën worden nooit gelegd',
  'err.BLACK_THREE_GO_OUT_ONLY':
    'Zwarte drieën worden alleen gelegd als de zet die je hand leegmaakt',
  'err.CANNOT_DISCARD_RED_THREE': 'Een rode drie mag niet afgelegd worden',
  'err.MUST_KEEP_A_CARD': 'Houd minstens één kaart — zo kun je je hand niet legen',
  'err.MUST_MELD_FIRST': 'Leg eerst de openingsleg van jouw partij',
  'err.INITIAL_MELD_NOT_MET': 'Je eerste leg komt nog punten tekort',
  'err.CANNOT_GO_OUT_YET': 'Jouw partij heeft een afgemaakte canasta nodig voordat ze kan uitgaan',
  'err.NOTHING_TO_CALL': 'Er is geen inzet om mee te gaan',
  'err.CANNOT_CHECK': 'Je kunt niet checken — er staat een inzet',
  'err.CANNOT_RAISE': 'Hier kun je niet verhogen',
  'err.RAISE_TOO_SMALL': 'Een verhoging moet minstens zo groot zijn als de vorige',
  'err.NOT_ENOUGH_CHIPS': 'Zoveel fiches heb je niet',
  'err.AMOUNT_REQUIRED': 'Zeg hoeveel',
  'err.AMOUNT_NOT_A_NUMBER': 'Dat bedrag is geen getal',
  'err.SEAT_NOT_IN_HAND': 'Je zit niet in deze hand',
  'err.WRONG_RANK': 'Die kaart heeft hiervoor de verkeerde waarde',
  'err.MATCH_FULL': 'De tafel is vol',
  'err.MATCH_ALREADY_STARTED': 'De partij is al begonnen',
  'err.TOO_FEW_PLAYERS': 'Nog niet genoeg spelers',
  'err.WRONG_PLAYER_COUNT': 'Dit spel kan niet met zoveel spelers gespeeld worden',
  'err.NOT_THE_HOST': 'Dat kan alleen de gastheer',
  'err.BAD_SEATING': 'Die volgorde past niet bij wie er aan tafel zit',
  'err.NO_LONGER_WAITING': 'De tafel wacht niet meer',
  'err.WAITING_ROOM_UNAVAILABLE': 'De wachtruimte is niet beschikbaar',
  'err.SERVER_BUSY': 'De server zit nu vol — probeer het zo nog eens',


  // --- rules the engine has always enforced and never stated ----------------
  // Every one of these was found by the guardrail rather than by review: the
  // validator could refuse a player for it, and no sentence anywhere said so.
  // See server/internal/zolikmod/ruleindex_test.go.
  'zolik.rules.section.layoff': 'Aanleggen bij combinaties',
  'zolik.rules.pickup.obligation':
    'Zolang je niet uitgelegd bent, moet een kaart die je van de aflegstapel neemt gebruikt worden in de combinatie waarmee je deze beurt uitlegt.',
  'zolik.rules.pickup.noReturn':
    'Een kaart die je van de aflegstapel nam, mag in dezelfde beurt niet opnieuw afgelegd worden — speel hem of houd hem.',
  'zolik.rules.wilds.setLimit': 'Een groep mag niet meer jokers bevatten dan natuurlijke kaarten.',
  'zolik.rules.set.maxSize':
    'Een groep mag niet meer dan {n} kaarten bevatten — een joker vervangt een ontbrekende kleur, hij vult een volledige groep niet aan.',
  'zolik.rules.run.maxLength':
    'Een reeks mag niet meer dan {n} kaarten bevatten — de aas onderaan, de twaalf waarden erboven en de aas bovenaan.',
  'zolik.rules.run.aceBridge':
    'Een aas staat boven de heer of onder de twee, nooit als brug tussen de twee uiteinden van een reeks.',
  'zolik.rules.contracts.contribution':
    'Zolang je niet uitgelegd bent, moet elke combinatie die je legt er een zijn die de opdracht van de ronde nog vraagt.',
  'zolik.rules.layoff.afterDown':
    'Je mag niets aanleggen bij andermans combinaties zolang je je eigen opdracht niet gelegd hebt.',
  'zolik.rules.layoff.runEnds':
    'Een kaart die je bij een reeks legt, moet die aan het ene of het andere uiteinde voortzetten.',
  'zolik.rules.jokers.swap':
    'Een joker in een combinatie op tafel mag teruggekocht worden met precies de kaart waarvoor hij staat.',
  'zolik.rules.jokers.reclaim.on':
    'Een van tafel teruggekochte joker moet dezelfde beurt in een combinatie gespeeld worden — hij mag niet in de hand blijven.',
  'zolik.rules.jokers.reclaim.off': 'Een van tafel teruggekochte joker mag in de hand blijven.',
  'zolik.rules.deck.reshuffle':
    'Als de trekstapel op is, wordt de aflegstapel geschud en de nieuwe trekstapel; zijn ze allebei leeg, dan eindigt de ronde.',


  // --- what to do instead ---------------------------------------------------
  // The third layer of a refusal, after the reason and the rule. Sent by the
  // module because it is the only side that knows which card is owed and
  // which way out is on offer; see server/internal/zolikmod/remedy.go.
  'zolik.remedy.meldThePickup': 'Voeg {card} toe aan je leg, of draai het pakken terug.',
  'zolik.remedy.discardSomethingElse': 'Leg een andere kaart af, of speel {card} deze beurt.',
  'zolik.remedy.discardNotAJoker': 'Leg iets anders af dan een joker.',
  'zolik.remedy.finishOrUndoLayDown': 'Maak je leg af, of neem hem terug.',
  'zolik.remedy.needMorePoints': 'Je hebt nog {n} punten nodig voordat je kunt uitleggen.',
  'zolik.remedy.layACleanRun': 'Leg een reeks zonder joker erin.',
  'zolik.remedy.playReclaimedJoker': 'Speel {card} in een combinatie, of draai het pakken terug.',
  'zolik.remedy.goDownFirst': 'Leg eerst je eigen combinaties.',
  'zolik.remedy.drawFirst': 'Pak eerst een kaart.',
  'zolik.remedy.drawFromStock': 'Pak van de trekstapel — de aflegstapel gaat open in ronde {n}.',
  'zolik.remedy.drawFromStockEmpty': 'Pak in plaats daarvan van de trekstapel.',


  // --- keys whose meaning lives in their params -----------------------------
  // The shape fallback renders these with the number missing ("Stack", "Pot"),
  // which is why they earn a line where most keys do not.
  'header.contract': 'Vraagt {sets} groepen en {runs} reeksen',
  'header.contract.cleanRunOnly': 'Vraagt een jokervrije reeks',
  'header.round': 'Ronde {n}',
  'header.deck': 'Trekstapel',
  'header.target': 'Doel',
  'header.suitInPlay': 'Kleur in het spel',
  'seat.cards': 'Kaarten',
  'zolik.offer.meld': 'Leggen',
  'prompt.pickupMustBeMelded':
    '{value} komt van de aflegstapel — die kaart moet in de combinaties waarmee je deze beurt uitlegt.',
  'prompt.jokerMustBePlayed':
    '{value} komt van tafel — die kaart moet in een combinatie voordat je je beurt kunt beëindigen.',
  'prompt.initialMeld': 'De openingsleg van jouw partij moet {n} punten halen.',
  'prompt.canastasNeeded': "Jouw partij heeft nog {n} canasta's nodig voordat ze kan uitgaan.",
  'prompt.mustDrawOrAnswerSeven': 'Antwoord met een zeven, of pak {n} kaarten.',
  'prompt.chooseSuit': 'Kies de kleur die verdergaat',
  'prompt.skipPending': 'Je beurt wordt overgeslagen',
  'status.lastDeal': 'Team {team} scoorde {value}',
  'status.teamScore': 'Team {team}: {value}',
  'canasta.offer.rank': 'Waarde',
  'canasta.offer.sequence': 'Reeks',
  'badge.naturalCanasta': 'Zuivere canasta',
  'badge.mixedCanasta': 'Onzuivere canasta',
  'badge.samba': 'Samba',
  'badge.cleanRun': 'Zuivere reeks',
  'canasta.seat.teamScore': 'Teamstand',
  'canasta.seat.canastas': "Canasta's",
  'holdem.header.pot': 'Pot',
  'holdem.header.street': 'Street',
  'holdem.header.hand': 'Hand',
  'holdem.header.handLimit': 'Handen in totaal',
  'holdem.header.blinds': 'Blinds',
  'holdem.cost.call': 'om mee te gaan',
  'holdem.cost.pot': 'in de pot',
  'holdem.seat.stack': 'Stapel',
  'holdem.seat.bet': 'Inzet',
  'holdem.prompt.yourAction': 'Jij bent',
  'holdem.prompt.raiseTo': 'Verhogen naar',
  'holdem.quick.halfPot': '½ Pot',
  'holdem.quick.pot': 'Pot',
  'holdem.quick.allIn': 'All-in',
  'zone.yourHand': 'Jouw hand',
  'zone.opponentHand': 'Zijn hand',
  'zone.drawPile': 'Trekstapel',
  'zone.discardPile': 'Aflegstapel',
  'zone.melds': 'Combinaties',
  'zone.teamMelds': 'Combinaties van jouw partij',
  'zone.opponentMelds': 'Combinaties van de tegenpartij',
  'zone.redThrees': 'Rode drieën',
  'zone.board': 'Board',
  'verb.drawFromDeck': 'Pakken',
  'verb.takeFromDiscard': 'Van de stapel nemen',


  // --- the why sheet's own furniture ---------------------------------------
  // The three layers a refusal is explained in. Labels, not sentences: what
  // goes under each is sent by the server or worded above.
  'why.reason': 'Waarom niet',
  'why.rule': 'De regel',
  'why.rules': 'De regels',
  'why.remedy': 'Wat je kunt doen',
  'why.readTheRules': 'Lees de volledige regels →',
  'why.close': 'Sluiten',
  'why.open': 'waarom',

  // --- marks on a particular card ------------------------------------------
  // A mark is a refusal that has not happened yet: the rule is enforced at the
  // discard, which is the last possible moment to hear about it, so the card
  // says so while there is still a turn left to act on it.
  'zolik.badge.owedToMeld':
    '{card} komt van de aflegstapel — die kaart moet in de combinaties waarmee je deze beurt uitlegt.',
  'zolik.badge.jokerOwed':
    '{card} komt van tafel — die kaart moet in een combinatie voordat je je beurt kunt beëindigen.',

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
  'legal.terms': 'Voorwaarden',
  // The document's own name, used as the heading above it. Kept apart from
  // `legal.notice.terms`, which is the same words inflected to sit inside a
  // sentence — a heading in the instrumental case reads as a mistake in every
  // Slavic language.
  'legal.terms.title': 'Gebruiksvoorwaarden',
  'legal.privacy.title': 'Privacyverklaring',
  'legal.privacy': 'Privacy',
  'legal.source': 'Broncode',
  'legal.updated': 'Versie {version}',
  'legal.draft':
    'Concept — nog niet van kracht. De naam, het land en het contactadres van de exploitant moeten nog ingevuld worden.',
  'legal.notice.before': 'Door te spelen ga je akkoord met de ',
  'legal.notice.terms': 'gebruiksvoorwaarden',
  'legal.notice.between': '. Wat er over je bewaard wordt, staat in de ',
  'legal.notice.privacy': 'privacyverklaring',
  'legal.notice.after': '.',
  // --- gin rummy -------------------------------------------------------------
  'err.UPCARD_DECLINED': 'Die kaart heb je al laten liggen',
  'err.DEADWOOD_TOO_HIGH': 'Je deadwood is te hoog om te kloppen',
  'err.CARD_DOES_NOT_EXTEND_MELD': 'Die kaart verlengt deze combinatie niet',
  'ginrummy.rules.setup': 'Opzet',
  'ginrummy.rules.turn': 'Jouw beurt',
  'ginrummy.rules.melds': 'Combinaties',
  'ginrummy.rules.knocking': 'Kloppen',
  'ginrummy.rules.bigGin': 'Big gin',
  'ginrummy.rules.layoff': 'Het aanleggen',
  'ginrummy.rules.deadHand': 'De dode hand',
  'ginrummy.rules.scoring': 'Een hand tellen',
  'ginrummy.rules.match': 'De partij winnen',
  'ginrummy.rules.lineBonuses': 'Bonussen bij het optellen',
  'ginrummy.rules.deck': 'Gespeeld met een spel van {value} kaarten.',
  'ginrummy.rules.deal': 'Elke speler krijgt {value} kaarten.',
  'ginrummy.rules.upcard': 'Er wordt nog één kaart open gelegd om de aflegstapel te beginnen.',
  'ginrummy.rules.drawDiscard':
    'Pak in jouw beurt één kaart — van de trekstapel of van de aflegstapel — en leg er daarna één af.',
  'ginrummy.rules.setsAndRuns':
    'Een combinatie is een groep van drie of vier kaarten van één waarde, of een reeks van drie of meer kaarten in één kleur.',
  'ginrummy.rules.aceLow': 'De aas telt altijd laag — een reeks van vrouw tot en met aas bestaat niet.',
  'ginrummy.rules.knockLimit': 'Je mag kloppen zodra je deadwood {n} of minder is.',
  'ginrummy.rules.oklahoma': 'De klopgrens van deze hand wordt bepaald door de waarde van de open kaart.',
  'ginrummy.rules.gin': 'Nul deadwood is gin — de best mogelijke klop.',
  'ginrummy.rules.bigGinBonus':
    'Elf kaarten die allemaal in combinaties passen, zonder ook maar af te leggen, is big gin en levert nog eens {n} punten op.',
  'ginrummy.rules.layoffDescription':
    'Na een klop die geen gin is, mag je tegenstander zijn eigen deadwood bij jouw combinaties aanleggen voordat de handen vergeleken worden.',
  'ginrummy.rules.deadHandDescription':
    'Zakt de trekstapel tot zijn laatste twee kaarten zonder dat iemand geklopt heeft, dan is de hand dood — niemand scoort, en dezelfde deler deelt opnieuw.',
  'ginrummy.rules.undercut':
    'Is het deadwood van je tegenstander niet hoger dan het jouwe, dan snijdt hij je af: hij scoort het verschil, plus {n}.',
  'ginrummy.rules.ginBonus': 'Gin levert de hele hand van je tegenstander op, plus {n}.',
  'ginrummy.rules.target': 'Wie na afloop van een hand als eerste {n} punten passeert, wint de partij.',
  'ginrummy.rules.shutout': 'De partijbonus verdubbelt naar {n} als de verliezer geen enkel punt scoorde.',
  'ginrummy.rules.box': 'Elke gewonnen hand is aan het eind van de partij {n} punten waard.',
  'ginrummy.rules.gameBonus': 'De partij winnen levert nog eens {n} punten op.',
  'ginrummy.fact.deadwood': '{value} deadwood',
  'ginrummy.fact.discardCard': '{value} afleggen',
  'ginrummy.fact.meldCards': 'Bij {value}',
  'ginrummy.header.hand': 'Hand {n}',
  'ginrummy.round.kind': '{value}',
  'ginrummy.round.hand': 'Hand',
  'ginrummy.seat.score': '{value}',
  'ginrummy.seat.dealer': 'Deler',
  'ginrummy.status.knocked': '{playerId} klopte met {deadwood} deadwood',
  'ginrummy.status.gin': '{playerId} ging gin',
  'ginrummy.status.lastHand': 'Laatste hand: {winner} ({kind}, {delta} punten)',
  'ginrummy.offer.drawStock': 'Van de trekstapel pakken',
  'ginrummy.offer.drawDiscard': 'Van de aflegstapel pakken',
  'ginrummy.offer.takeUpcard': 'De open kaart nemen',
  'ginrummy.offer.passUpcard': 'Passen',
  'ginrummy.offer.discard': 'Afleggen',
  'ginrummy.offer.knock': 'Kloppen',
  'ginrummy.offer.gin': 'Gin!',
  'ginrummy.offer.bigGin': 'Big gin!',
  'ginrummy.offer.layOff': 'Aanleggen',
  'ginrummy.offer.finishLayoff': 'Klaar met aanleggen',
  'ginrummy.zone.knockerHand': 'Geklopte hand',
  'ginrummy.zone.melds': 'Combinaties',
  'ginrummy.prompt.upcardDecision': 'Neem de open kaart, of pas',
  'ginrummy.prompt.yourTurnDraw': 'Pak een kaart',
  'ginrummy.prompt.yourTurnDiscard': 'Leg af — of klop, als het kan',
  'ginrummy.prompt.layoff': 'Leg deadwood aan, of rond af',

  // --- rummy tiles -------------------------------------------------------------
  'err.TILE_NOT_IN_HAND': 'Die steen zit niet in je hand',
  'err.TILE_DOES_NOT_FIT': 'Dat past daar niet',
  'err.NO_SUCH_SET': 'Die combinatie ligt niet op tafel',
  'err.INITIAL_MELD_ONLY': 'Vóór je eerste leg mag je alleen je eigen nieuwe combinaties herschikken',
  'err.TABLE_NOT_VALID': 'De tafel is nog niet geldig',
  'err.TRAY_NOT_EMPTY': 'Je hebt nog losse stenen te plaatsen',
  'err.NOTHING_PLAYED': 'Speel minstens één steen voordat je je beurt beëindigt',
  'err.INITIAL_MELD_TOO_LOW': 'Je eerste leg moet 30 punten of meer waard zijn',
  'err.NOT_A_RUN': 'Alleen een reeks kan gesplitst worden',
  'err.BAD_SPLIT_POSITION': 'Daar kan deze reeks niet gesplitst worden',
  'err.NO_JOKER_IN_SET': 'Er zit geen joker in die combinatie',
  'err.TILE_JOKER_SWAP_MISMATCH': 'Die steen is niet waar de joker voor staat',
  'rummytiles.rules.setup': 'Opzet',
  'rummytiles.rules.sets': 'Combinaties',
  'rummytiles.rules.initialMeld': 'De openingsleg',
  'rummytiles.rules.turn': 'Jouw beurt',
  'rummytiles.rules.jokerTaking': 'Een joker nemen',
  'rummytiles.rules.ending': 'Een ronde beëindigen',
  'rummytiles.rules.poolExhaustion': 'Als de voorraad opraakt',
  'rummytiles.rules.match': 'De partij winnen',
  'rummytiles.rules.tiles': 'Gespeeld met {value} stenen.',
  'rummytiles.rules.dealCount': 'Elke speler krijgt {value} stenen.',
  'rummytiles.rules.group':
    'Een groep bestaat uit drie of vier stenen met hetzelfde getal, elk in een andere kleur.',
  'rummytiles.rules.run': 'Een reeks bestaat uit drie of meer opeenvolgende getallen in één kleur.',
  'rummytiles.rules.noWrap': 'Na de 13 begint het niet opnieuw bij de 1.',
  'rummytiles.rules.joker': 'Een joker staat voor elke willekeurige steen.',
  'rummytiles.rules.initialMeldDescription':
    'Zolang je niet in één beurt {n} punten of meer hebt gelegd, uitsluitend uit je eigen hand, mag je niets aanraken van wat al op tafel ligt.',
  'rummytiles.rules.turnDescription':
    'Speel minstens één steen uit je hand, herschik de tafel naar believen, en eindig met elke combinatie op tafel geldig.',
  'rummytiles.rules.noDiscard':
    'Er wordt niet afgelegd — kun je geen geldige beurt afmaken, dan pak je in plaats daarvan één steen.',
  'rummytiles.rules.jokerTakingDescription':
    'Een joker op tafel mag je nemen door hem te vervangen door de steen waarvoor hij staat, uit je hand — en die joker moet vóór het eind van je beurt in een combinatie gebruikt worden.',
  'rummytiles.rules.goingOut':
    'De eerste speler zonder stenen wint de ronde. Alle anderen scoren de negatieve waarde van wat ze overhouden; de winnaar scoort de som van wat alle anderen verloren.',
  'rummytiles.rules.poolExhaustionLowestWins':
    'Raakt de voorraad op en kan niemand meer spelen, dan eindigt de ronde en wint de laagste handwaarde.',
  'rummytiles.rules.poolExhaustionNoWinner':
    'Raakt de voorraad op en kan niemand meer spelen, dan eindigt de ronde zonder winnaar — elke hand wordt gewoon geteld.',
  'rummytiles.rules.target': 'Wie na afloop van een ronde als eerste {n} punten passeert, wint de partij.',
  'rummytiles.rules.roundLimit': 'De partij eindigt na {n} rondes — de hoogste score wint.',
  'rummytiles.fact.setCards': '{value}',
  'rummytiles.header.pool': 'Voorraad {n}',
  'rummytiles.header.round': 'Ronde {n}',
  'rummytiles.round.kind': '{value}',
  'rummytiles.round.round': 'Ronde',
  'rummytiles.seat.score': '{value}',
  'rummytiles.seat.notOpened': 'Niet geopend',
  'rummytiles.status.lastRound': 'Laatste ronde: {winner} ({kind})',
  'rummytiles.badge.invalid': 'Nog niet geldig',
  'rummytiles.zone.pool': 'Voorraad',
  'rummytiles.zone.table': 'Tafel',
  'rummytiles.zone.tray': 'Plankje',
  'rummytiles.offer.place': 'Plaatsen',
  'rummytiles.offer.addFromHand': 'Toevoegen',
  'rummytiles.offer.addFromTray': 'Van het plankje toevoegen',
  'rummytiles.offer.take': 'Nemen',
  'rummytiles.offer.split': 'Splitsen',
  'rummytiles.offer.swapJoker': 'Joker ruilen',
  'rummytiles.offer.resetTurn': 'Beurt herstellen',
  'rummytiles.offer.commit': 'Klaar',
  'rummytiles.offer.draw': 'Pakken',
  'rummytiles.param.position': 'Splitsen bij',

  // --- blackjack -----------------------------------------------------------
  //
  // The house rules are worded twice over — on and off — because a rule that
  // is *not* in force is still something a player has to be told. "No
  // surrender at this table" is information; silence is a guess.
  'err.BET_BELOW_MINIMUM': 'Dat is onder het tafelminimum',
  'err.ALREADY_BET': 'Je inzet staat al',
  'err.INSURANCE_CLOSED': 'Er is nu geen verzekering te nemen',
  'err.CANNOT_DOUBLE': 'Deze hand kan niet verdubbeld worden',
  'err.CANNOT_SPLIT': 'Deze hand kan niet gesplitst worden',
  'err.CANNOT_SURRENDER': 'Deze hand kan niet opgegeven worden',

  'blackjack.rules.section.table': 'De tafel',
  'blackjack.rules.section.play': 'Een hand spelen',
  'blackjack.rules.section.dealer': 'De dealer',
  'blackjack.rules.section.end': 'Hoe de partij eindigt',
  'blackjack.rules.goal':
    'Versla de dealer zonder boven de eenentwintig te komen. Erboven komen verliest meteen, wat de dealer daarna ook doet.',
  // Counts are their own phrase rather than a number glued to a noun: one
  // deck and six decks inflect differently, and Czech inflects them again.
  'blackjack.rules.decks': 'Spellen in de schoen: {n}.',
  'blackjack.rules.stack': 'Elke plek gaat zitten met {n} fiches.',
  'blackjack.rules.minBet': 'Het tafelminimum is {n} fiches.',
  'blackjack.rules.faceUp':
    'De kaarten van de spelers worden open gedeeld; de dealer houdt één kaart dicht tot iedereen gespeeld heeft.',
  'blackjack.rules.hitStand': 'Neem zoveel kaarten als je wilt, of blijf staan op wat je hebt.',
  'blackjack.rules.aces': 'Een aas telt voor elf zolang dat past, en anders voor één.',
  'blackjack.rules.blackjack':
    'Een aas met een tienwaardige kaart, op de eerste twee kaarten, is een blackjack.',
  'blackjack.rules.pays3to2': 'Een blackjack betaalt 3:2.',
  'blackjack.rules.pays6to5': 'Een blackjack betaalt 6:5.',
  'blackjack.rules.paysEven': 'Een blackjack betaalt gelijk uit.',
  'blackjack.rules.double':
    'Op je eerste twee kaarten mag je je inzet verdubbelen en precies één kaart bij nemen.',
  'blackjack.rules.doubleAfterSplit': 'Een hand die uit een split komt, mag ook verdubbeld worden.',
  'blackjack.rules.noDoubleAfterSplit': 'Een hand die uit een split komt, mag niet verdubbeld worden.',
  'blackjack.rules.split':
    'Twee kaarten van dezelfde waarde mogen gesplitst worden in eigen handen, elk met een eigen inzet — tot {n} keer, voor {hands} handen in totaal.',
  'blackjack.rules.noSplit': 'Aan deze tafel worden paren niet gesplitst.',
  'blackjack.rules.splitAces':
    'Gesplitste azen krijgen elk één kaart en blijven dan staan, en eenentwintig die zo ontstaat is geen blackjack.',
  'blackjack.rules.surrender':
    'Je mag je eerste hand opgeven voor de helft van de inzet, zodra de dealer op blackjack gecontroleerd heeft.',
  'blackjack.rules.noSurrender': 'Aan deze tafel kunnen handen niet opgegeven worden.',
  'blackjack.rules.dealerDraws': 'De dealer neemt kaarten tot zeventien en blijft dan staan.',
  'blackjack.rules.hitsSoft17': 'De dealer neemt een kaart bij een zeventien die met een aas gevormd is.',
  'blackjack.rules.standsSoft17': 'De dealer blijft staan op een zeventien die met een aas gevormd is.',
  'blackjack.rules.dealerPeeks':
    'Bij een aas of een tien controleert de dealer op blackjack voordat er iemand speelt.',
  'blackjack.rules.insurance':
    'Tegen een aas van de dealer mag je je verzekeren voor de helft van je inzet; dat betaalt 2:1 als de dealer blackjack heeft.',
  'blackjack.rules.noInsurance': 'Aan deze tafel wordt geen verzekering aangeboden.',
  'blackjack.rules.rounds': 'Aan de tafel worden {n} rondes gespeeld.',
  'blackjack.rules.mostChipsWins': 'Wie aan het eind de meeste fiches heeft, wint de partij.',
  'blackjack.rules.bustedOut':
    'Een plek die het minimum van {n} niet meer kan opbrengen, zit de rest van de partij uit.',

  'blackjack.zone.dealer': 'Dealer',
  'blackjack.zone.box': 'Hand',
  'blackjack.zone.yourBox': 'Jouw hand',
  'blackjack.zone.shoe': 'Schoen',

  'blackjack.header.round': 'Ronde {n} van {of}',
  'blackjack.header.minBet': 'Minimum',
  'blackjack.header.decks': 'Spellen',
  'blackjack.header.dealerTotal': 'Dealer toont {n}',
  'blackjack.header.dealerSoftTotal': 'Dealer toont zacht {n}',

  'blackjack.seat.stack': 'Fiches',
  'blackjack.seat.bet': 'Inzet',
  'blackjack.seat.insurance': 'Verzekering',
  'blackjack.seat.total': 'Totaal',
  'blackjack.seat.softTotal': 'Zacht totaal',
  'blackjack.seat.out': 'Geen fiches meer',

  'blackjack.prompt.placeBet': 'Plaats je inzet',
  'blackjack.prompt.insurance': 'Verzekering?',
  'blackjack.prompt.yourMove': 'Jij bent',
  'blackjack.prompt.waitingFor': 'Wachten op {playerId}',
  'blackjack.prompt.betAmount': 'Inzet',

  'blackjack.quick.doubleMin': '2× Minimum',
  'blackjack.quick.allIn': 'All-in',
  'blackjack.offer.bet': 'Inzetten',
  'blackjack.offer.hit': 'Kaart',
  'blackjack.offer.stand': 'Passen',
  'blackjack.offer.double': 'Verdubbelen',
  'blackjack.offer.split': 'Splitsen',
  'blackjack.offer.surrender': 'Opgeven',
  'blackjack.offer.insure': 'Verzekering nemen',
  'blackjack.offer.declineInsurance': 'Geen verzekering',

  'blackjack.fact.tableMinimum': 'minimum',
  'blackjack.fact.insuranceCost': 'om te verzekeren',
  'blackjack.fact.extraStake': 'in te zetten',
  'blackjack.fact.surrenderReturn': 'terug',

  'blackjack.status.dealerBlackjack': 'De dealer had blackjack',
  'blackjack.status.dealerBust': 'De dealer ging kapot met {n}',
  'blackjack.status.dealerStands': 'De dealer blijft staan op {n}',

  'blackjack.round.name': 'Ronde',
  'blackjack.round.dealerTotal': 'Dealer {n}',
  'blackjack.round.dealerBust': 'Dealer kapot ({n})',
  'blackjack.round.dealerBlackjack': 'Blackjack voor de dealer',
  'blackjack.round.outcome.blackjack': 'Blackjack',
  'blackjack.round.outcome.win': 'Gewonnen',
  'blackjack.round.outcome.push': 'Gelijk',
  'blackjack.round.outcome.lose': 'Verloren',
  'blackjack.round.outcome.bust': 'Kapot',
  'blackjack.round.outcome.surrender': 'Opgegeven',

  'blackjack.badge.inPlay': 'In het spel',
  'blackjack.badge.doubled': 'Verdubbeld',
  'blackjack.badge.split': 'Gesplitst',
  'blackjack.badge.blackjack': 'Blackjack',
  'blackjack.badge.bust': 'Kapot',
  'blackjack.badge.won': 'Gewonnen',
  'blackjack.badge.push': 'Gelijk',
  'blackjack.badge.lost': 'Verloren',
  'blackjack.badge.surrendered': 'Opgegeven',

  'blackjack.unit.chips': 'fiches',

  // --- the settings screen --------------------------------------------------
  //
  // Hardcoded English until now, which was survivable while the only other
  // language was unreachable and faintly absurd once the language picker
  // itself lived on this screen: a player who cannot read "Settings" is
  // exactly the one who came here to change it.
  'settings.title': 'Instellingen',
  'settings.signedInAs': 'Aangemeld als {username}',
  'settings.playingAsGuest': 'Je speelt als {username} (gast)',
  'settings.notSignedIn': 'Niet aangemeld — meld je aan of ga verder als gast om online te spelen.',
  'settings.subtitle': 'Hoe jij eruitziet, en hoe de tafel',
  'settings.face.heading': 'Jouw gezicht aan tafel',
  'settings.face.account': 'Bewaard bij je account, zodat het met je meegaat naar een ander apparaat.',
  'settings.face.device': 'Bewaard op dit apparaat. Meld je aan om het mee te nemen.',
  'settings.skin.heading': 'Uiterlijk van de tafel',
  'settings.language.heading': 'Taal',
  'settings.language.status': 'Bewaard op dit apparaat.',
  'settings.language.auto': 'Automatisch',
  'settings.language.auto.now': 'Volgt je apparaat — nu {language}',
  'settings.legal.heading': 'De kleine lettertjes',
  'settings.legal.status': 'Waar je door te spelen mee akkoord ging, en wat er over je bewaard wordt.',
  'settings.signIn': 'Aanmelden',
  'settings.back': 'Terug',

  // A translated interface over an English notice is a worse lie than an
  // English interface, so the document says so itself rather than letting the
  // surrounding screen imply the text was written for this reader.
  'legal.untranslated':
    'Deze kennisgeving is nog niet in jouw taal vertaald. De Engelse tekst hieronder is de versie die geldt.',

  // --- the navigation bar's screen titles ------------------------------------
  //
  // These sit in `app/_layout.tsx`, above every screen, and were the last
  // English left after the settings screen was translated — a German player
  // reading "Einstellungen" under a bar that said "Settings". A translated app
  // with an untranslated chrome bar looks like a bug, not like a limit.
  'nav.home': 'Zolik',
  'nav.emailSignIn': 'Aanmelden met e-mail',
  'nav.signingIn': 'Bezig met aanmelden',
  'nav.usernameSignIn': 'Aanmelden met gebruikersnaam',
  'nav.legacyAccount': 'Oud account',
  'nav.guest': 'Gast',
  'nav.account': 'Account',
  'nav.games': 'Spellen',
  'nav.table': 'Jouw tafel',
  'nav.join': 'Aan een tafel deelnemen',
  // The screen a shared invite link lands on while it resolves the code.
  'nav.joining': 'Bezig met deelnemen',
  'nav.rules': 'Regels',
  'nav.match': 'Partij',
  'nav.scoreTable': 'Scoretabel',
  'nav.stats': 'Statistieken',
  'nav.more': 'Meer',
  'nav.about': 'Over',

  // --- the account menu behind the face in the corner ----------------------
  'menu.label': 'Accountmenu',
  'menu.signedIn': 'Aangemeld',
  'menu.notSignedIn': 'Niet aangemeld',
  'menu.keepStats': 'om je statistieken te bewaren',
  'menu.signOut': 'Afmelden',
  'more.scoreTable': 'Offline scoretabel',
  'more.stats': 'Statistieken en ranglijst',
  'more.needsAccount': 'meld je aan om te gebruiken',
  'gate.title': 'Meld je aan om dit te gebruiken',
  'gate.body':
    'Scoretabellen en statistieken worden bij je account bewaard, zodat ze met je meegaan naar een ander apparaat. Een gast heeft er geen plek voor.',

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
  'error.generic': 'Dat is niet gelukt',
  'error.signIn': 'Aanmelden mislukt',
  'error.login': 'Aanmelden mislukt',
  'error.register': 'Registreren mislukt',
  'error.sendCode': 'Kon geen code versturen',
  'error.badCode': 'Die code werkte niet',
  'error.rulesLoad': 'Kon de regels niet laden',
  'error.createFailed': 'Aanmaken mislukt',
  'error.saveFailed': 'Opslaan mislukt',
  'error.exportFailed': 'Exporteren mislukt',

  // --- the route that does not exist ----------------------------------------
  'notFound.title': 'Oeps!',
  'notFound.message': 'Dit scherm bestaat niet.',
  'notFound.home': 'Ga naar het beginscherm!',

  // --- signing in -----------------------------------------------------------
  'auth.login.subtitle': 'Houd je statistieken op al je apparaten',
  'auth.login.continueWithEmail': 'Doorgaan met e-mail',
  'auth.login.usernameInstead': 'In plaats daarvan met een gebruikersnaam aanmelden',
  'auth.email.title': 'Aanmelden met e-mail',
  'auth.email.subtitle': 'We mailen je een eenmalige code',
  'auth.email.address': 'E-mailadres',
  'auth.email.send': 'Code versturen',
  'auth.email.codeTitle': 'Voer de code in',
  'auth.email.codePlaceholder': 'Code van 6 cijfers',
  'auth.email.differentAddress': 'Ander adres gebruiken',
  'auth.email.sentTo': 'Verstuurd naar {email}',
  'auth.email.continue': 'Doorgaan',
  'auth.guest.title': 'Spelen als gast',
  'auth.guest.subtitle': 'Geen account nodig',
  'auth.guest.displayName': 'Weergavenaam',
  'auth.register.title': 'Account aanmaken',
  'auth.register.username': 'Gebruikersnaam',
  'auth.register.email': 'E-mail (optioneel)',
  'auth.register.password': 'Wachtwoord',
  'auth.username.createAccount': 'Een account met gebruikersnaam en wachtwoord aanmaken',
  'auth.callback.signedIn': 'Aangemeld.',

  // --- the account screen ---------------------------------------------------
  'account.signInPrompt': 'Meld je aan om je account te beheren.',
  'account.keepGames': 'Deze partijen behouden',
  'account.signedInWith': 'Aangemeld met',
  'account.addMethod': 'Een aanmeldmethode toevoegen',
  'account.usernameAndPassword': 'Gebruikersnaam en wachtwoord',
  'account.faceAndTable': 'Gezicht en uiterlijk van de tafel',
  'account.refresh': 'Vernieuwen',
  'account.remove': 'Verwijderen',

  // --- the main menu --------------------------------------------------------
  'home.subtitle': 'Continentale rummy · {server}',
  'home.playingAs': 'Je speelt als {name}',
  'home.signInPrompt': 'Meld je aan of ga verder als gast om online te spelen.',
  'home.statsAndLeaderboard': 'Statistieken en ranglijst',
  'home.play': 'Spelen',
  'home.offlineScoreTable': 'Scoretabel offline',
  'home.signInToKeepStats': 'Meld je aan om je statistieken te bewaren',
  'home.signOut': 'Afmelden',
  'home.continueAsGuest': 'Doorgaan als gast',
  // Someone playing as a guest, marked in a list of names.
  'home.guestSuffix': '(gast)',

  // --- the waiting room on the main menu ------------------------------------
  //
  // Counts are whole phrases per count, not a number glued to a noun — the
  // same reason `countLabel` exists. One and many are the cases the interface
  // actually produces, so they are the cases that get their own wording.
  'waiting.checking': 'Kijken wie er is…',
  'waiting.youAreWaiting': 'Je wacht om te spelen',
  'waiting.pickedUp': 'Iedereen die een tafel opent kan je oppikken — daar is geen code van jou voor nodig.',
  'waiting.othersOne': '1 andere speler wacht ook',
  'waiting.othersMany': '{n} andere spelers wachten ook',
  'waiting.oneWaiting': '1 speler wacht om te spelen',
  'waiting.manyWaiting': '{n} spelers wachten om te spelen',
  'waiting.adding': 'Je wordt aan de wachtlijst toegevoegd…',
  'waiting.slowHint':
    'Als dit niet binnen een paar seconden klaar is, controleer dan of het serveradres hieronder vanaf dit apparaat bereikbaar is.',
  'waiting.serverBusyDetail':
    'Poging {n}. De server neemt op dit moment geen nieuwe verbindingen met de wachtruimte aan.',
  'waiting.reconnecting': 'Verbinding verbroken — opnieuw verbinden…',
  'waiting.reconnectingDetail':
    'Poging {n}. Dit kan gebeuren als het netwerk van je apparaat is veranderd, of als de server opnieuw is gestart.',
  'waiting.tryAgain': 'Nu opnieuw proberen',
  'waiting.makeAvailable': 'Mij beschikbaar stellen om te spelen',
  'waiting.stop': 'Stoppen met wachten',
  'waiting.noneYet':
    'Er wacht op dit moment niemand om te spelen. Zet jezelf op de lijst, dan ben jij de eerste die iemand ziet.',
  'waiting.noOthersYet': 'Er wacht nog niemand anders. Gastheren zien je toch en kunnen je uitnodigen.',
  'waiting.server': 'Server',
  'waiting.none':
    'Er wacht op dit moment niemand. Wie zich in het hoofdmenu beschikbaar stelt, verschijnt hier.',

  // --- landing on a shared invite link --------------------------------------
  'join.missingCode': 'Aan die link ontbreekt de tafelcode.',
  'join.staleLink': 'Vraag degene die je uitnodigde om een nieuwe link, of doe mee met de code.',
  'join.enterCode': 'Een code invoeren',
  'join.backToMenu': 'Terug naar het menu',
  'join.takingSeat': 'Plaatsnemen…',
  'join.takingSeatAt': 'Plaatsnemen bij {game}…',

  // --- the lobby ------------------------------------------------------------
  'lobby.games.subtitle': 'Alles wat deze server kan aanbieden',
  'lobby.games.bots': 'Bots',
  'lobby.games.playBot': 'Tegen een bot spelen',
  'lobby.games.playBots': 'Tegen {n} bots spelen',
  'lobby.games.openTable': 'Een tafel openen',
  'lobby.games.players': '{n} spelers',
  'lobby.games.playerRange': '{min}–{max} spelers',
  'lobby.join.placeholder': 'Deelnamecode of uitnodigingslink',
  'lobby.join.needCode': 'Voer een code, een link of een match-ID in',
  'lobby.games.signInFirst': 'Meld je eerst aan',
  'lobby.join.action': 'Meedoen',
  'lobby.join.waitingTitle': 'Wachten op de gastheer',
  // Two whole sentences rather than one with a swapped noun: "a game of {game}"
  // does not survive a language that inflects the game's name after "of".
  'lobby.join.joinedGame': 'Je doet mee aan een partij {game} — wachten op de start',
  'lobby.join.joinedTable': 'Je zit aan tafel — wachten op de start',
  'lobby.table.addBot': 'Een bot toevoegen',
  'lobby.table.side': 'Kant {n}',
  'lobby.table.shuffleSeats': 'Plaatsen schudden',
  'lobby.table.moveSeatUp': '{name} een plaats omhoog',
  'lobby.table.moveSeatDown': '{name} een plaats omlaag',
  'lobby.table.start': 'Starten',
  'lobby.table.waitingForHost': 'Wachten tot de gastheer start…',

  // --- inviting someone to a table ------------------------------------------
  'invite.heading': 'Spelers uitnodigen',
  'invite.explain': 'Stuur deze link. Wie hem opent komt aan deze tafel terecht — zonder account.',
  'invite.noAddress': 'Voor deze server is geen deelbaar adres ingesteld, gebruik dus de code hieronder.',
  'invite.readOutCode': 'Of lees de code voor:',
  'invite.copy': 'Link kopiëren',
  'invite.share': 'Link delen',
  'invite.copied': 'Gekopieerd!',
  'invite.shared': 'Gedeeld',

  // --- the match screen -----------------------------------------------------
  'match.waitingForTable': 'Wachten op de tafel…',
  'match.waitingForPlayer': 'Wachten op een andere speler…',
  'match.nobodyWon': 'Niemand heeft gewonnen.',
  'match.youWon': 'Je hebt gewonnen.',
  'match.finished': 'Deze partij is afgelopen.',
  'match.inProgress': 'Partij bezig — alles is verbonden en loopt normaal.',
  'match.connecting': 'Verbinden…',
  'match.abandonedTitle': 'Tafel opzijgezet',
  'match.abandoned': 'Niemand kwam terug naar deze tafel, dus is hij opzijgezet. De kaarten liggen precies waar je ze achterliet.',
  'match.resume': 'Ga verder waar je gebleven was',
  'match.resuming': 'Tafel wordt teruggehaald…',
  'match.controls': 'Bediening',
  'match.over': 'Partij afgelopen',
  'match.settingUp': 'Klaarzetten…',
  'match.playAgain': 'Nog een keer spelen',
  'match.backToGames': 'Terug naar de spellen',
  'match.table': 'Tafel',
  'match.opponents': 'Tegenstanders',
  // Marks which seat is the reader's own, in a list of seats.
  'match.youSuffix': '(jij)',
  // The winner line is composed from whole sentences rather than a name glued
  // to " won." — the verb agrees with the subject in most of these languages,
  // and a suffix cannot know that.
  'match.you': 'jij',
  'match.someoneWon': '{name} heeft gewonnen.',
  'match.wonBy': 'Gewonnen door {names}.',
  'match.pausedFor': 'Gepauzeerd — wachten tot {name} opnieuw verbindt.',
  'match.results': 'Uitslag',
  'match.players': 'Spelers',
  'match.toPlay': 'aan zet',

  // --- the offline score table ----------------------------------------------
  'scoring.namesHint': "Namen gescheiden door komma's (4–8 spelers)",
  'scoring.newSession': 'Nieuwe sessie',
  // A worked example, not a sentence: the names are placeholders a translator
  // may localise, the shape `name:score,` is what the parser needs.
  'scoring.scoresPlaceholder': 'Anna:120,Bram:80,…',
  'scoring.saveRound': 'Ronde opslaan',
  'scoring.export': 'Scorekaart exporteren',
  'scoring.formatHint': 'Puntenformaat: Naam:100,Naam2:50',
  'scoring.nameCountError': "Voer 2–8 spelersnamen in, gescheiden door komma's",
  'scoring.session': 'Sessie: {id}',
  'scoring.players': 'Spelers: {names}',
  'scoring.roundScores': 'Punten van ronde {n}',
  'stats.loading': 'Laden…',
  // The figures could not be fetched. `{reason}` is whatever the server or the
  // network said, which is not ours to translate — the frame around it is.
  'stats.unavailable': '(niet beschikbaar: {reason})',

  // --- statistics -----------------------------------------------------------
  'stats.title': 'Statistieken en ranglijst',
  'stats.yours': 'Jouw statistieken',
  'stats.leaderboard': 'Ranglijst',

  // --- a player's lifetime record, shown beside a finished match ------------
  'record.title': 'Jouw balans',
  'record.guest':
    'Je speelt als gast, dus er wordt geen balans bijgehouden. Meld je aan en de partijen die je op dit apparaat al hebt gespeeld — deze inbegrepen — worden bij je account bewaard.',
  'record.signInToKeep': 'Aanmelden en bewaren',
  'record.failed': 'Je balans kon nu niet geladen worden. De partij is netjes vastgelegd.',
  'record.loading': 'Laden…',
  'record.played': 'Gespeeld',
  'record.won': 'Gewonnen',
  'record.lost': 'Verloren',
  'record.winRate': 'Winstpercentage',
  'record.streak': 'Reeks',
  'record.atThisGame': 'Bij dit spel',
  // A streak is a count with a noun, so it is a whole phrase per count for
  // the same reason `countLabel` is: "1 win" / "2 wins" is an English rule,
  // and most of these languages do not share it.
  'record.streakNone': '—',
  'record.streakWinOne': '1 overwinning',
  'record.streakWinMany': '{n} overwinningen',
  'record.streakLossOne': '1 nederlaag',
  'record.streakLossMany': '{n} nederlagen',

  // --- moving cards ---------------------------------------------------------
  'hand.dragHint': 'Sleep een kaart langs de waaier om hem te verplaatsen, of naar het bord om hem te spelen',
  'hand.moveLeft': 'Naar links',
  'hand.moveRight': 'Naar rechts',
  'zone.collapseGroup': 'Deze groep inklappen',
  'zone.expandGroup': 'Alle kaarten in deze groep tonen',
  'zone.dropHere': 'Hier neerleggen',
  'offer.pickCards': 'kies kaarten voor de plek die je aantikte',
  'offer.ambiguous': 'dit kan op meer dan één plek — kies op het bord',

  // --- the build footer -----------------------------------------------------
  'build.app': 'app',
  'build.server': 'server',
  'about.subtitle': 'De versie waarmee je speelt, en de kleine lettertjes.',
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
  'option.pauseBetweenRounds': 'Pauze tussen rondes',
  'choice.pauseBetweenRounds.1': 'Pauze',
  'choice.pauseBetweenRounds.0': 'Meteen door',
  'option.openDiscardPile': 'Aflegstapel',
  'choice.openDiscardPile.1': 'Mag doorgekeken worden',
  'choice.openDiscardPile.0': 'Alleen de bovenste kaart',
  'option.botSkill': 'Tegenstanders',
  'choice.botSkill.0': 'Gemengd',
  'choice.botSkill.1': 'Makkelijk',
  'choice.botSkill.2': 'Gemiddeld',
  'choice.botSkill.3': 'Moeilijk',
  'option.initialMeldMinimum': 'Openingswaarde',
  'choice.initialMeldMinimum.0': 'Geen',
  'option.discardDrawMinRound': 'Pakken van de aflegstapel',
  'choice.discardDrawMinRound.0': 'Open',
  'choice.discardDrawMinRound.2': 'Vanaf ronde 2',
  'choice.discardDrawMinRound.3': 'Vanaf ronde 3',
  'option.requireCleanRun': 'Jokervrije reeks',
  'choice.requireCleanRun.1': 'Verplicht',
  'choice.requireCleanRun.0': 'Nee',
  'option.jokerReclaimMustPlay': 'Teruggekochte joker',
  'choice.jokerReclaimMustPlay.1': 'Zelfde beurt spelen',
  'choice.jokerReclaimMustPlay.0': 'Mag je houden',
  'option.dealStarter': 'Voorhand',
  'choice.dealStarter.0': 'Om de beurt',
  'choice.dealStarter.1': 'Winnaar begint',
  'variation.prsi.classic': 'Klassiek',
  'option.handSize': 'Gedeelde kaarten',
  'variation.canasta.classic': 'Klassiek',
  'variation.canasta.modern_american': 'Modern American',
  'variation.canasta.samba': 'Samba',
  'option.targetScore': 'Doelscore',
  'option.canastasToGoOut': "Canasta's om uit te gaan",
  'variation.holdem.freezeout': 'Freezeout',
  'variation.holdem.timed': 'Vast aantal handen',
  'option.startingStack': 'Startfiches',
  'option.bigBlind': 'Big blind',
  'option.handLimit': 'Handen',
  'choice.handLimit.0': 'Tot er één plek over is',
  'variation.ginrummy.standard': 'Standaard',
  'option.knockLimit': 'Klopgrens',
  'choice.knockLimit.0': 'Oklahoma (de open kaart bepaalt hem)',
  'option.bigGin': 'Big gin',
  'choice.bigGin.0': 'Uit',
  'choice.bigGin.1': 'Aan (+25)',
  'option.lineBonuses': 'Bonussen bij het optellen',
  'choice.lineBonuses.1': 'Aan',
  'choice.lineBonuses.0': 'Uit',
  'variation.rummytiles.standard': 'Standaard',
  'choice.targetScore.0': 'Geen',
  // Two games qualify a number the others mean plainly: Canasta's 500 is a
  // short game, Gin Rummy's and Rummy Tiles' 500 is just 500; Hold'em's
  // starting 200 is a short stack, Blackjack's 200 is just 200. Module-scoped
  // so the qualifier travels with the game that means it — a bare key would
  // hand the note to games it is false for. `scripts/check-server-labels.js`
  // is what finds these; both were shipping as English on a Czech screen.
  'choice.canasta.targetScore.500': '500 (kort)',
  'choice.holdem.startingStack.200': '200 (kort)',
  'option.roundLimit': 'Rondelimiet',
  'choice.roundLimit.0': 'Geen',
  'option.poolExhaustion': 'Als de voorraad opraakt',
  'choice.poolExhaustion.1': 'Laagste hand wint de ronde',
  'choice.poolExhaustion.0': 'Niemand wint de ronde',
  'variation.blackjack.single': 'Eén spel',
  'option.minBet': 'Tafelminimum',
  'option.rounds': 'Rondes',
  'option.decks': 'Spellen',
  'option.dealerHitsSoft17': 'Dealer bij zachte 17',
  'choice.dealerHitsSoft17.0': 'Blijft staan',
  'choice.dealerHitsSoft17.1': 'Neemt kaart',
  'option.blackjackPays': 'Blackjack betaalt',
  'choice.blackjackPays.100': 'Gelijk uit',
  'option.maxSplits': 'Splitsen',
  'choice.maxSplits.0': 'Niet splitsen',
  'choice.maxSplits.1': 'Eén keer (twee handen)',
  'choice.maxSplits.3': 'Drie keer (vier handen)',
  'option.doubleAfterSplit': 'Verdubbelen na splitsen',
  'choice.doubleAfterSplit.1': 'Toegestaan',
  'choice.doubleAfterSplit.0': 'Niet toegestaan',
  'option.surrender': 'Opgeven',
  'choice.surrender.0': 'Uit',
  'choice.surrender.1': 'Laat opgeven',
  'option.insurance': 'Verzekering',
  'choice.insurance.1': 'Aangeboden',
  'choice.insurance.0': 'Niet aangeboden',

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
  'verb.add': 'Toevoegen',
  'verb.bet': 'Inzetten',
  'verb.call': 'Meegaan',
  'verb.check': 'Checken',
  'verb.commit': 'Klaar',
  'verb.continue': 'Doorgaan',
  'verb.decline_insurance': 'Geen verzekering',
  'verb.discard': 'Afleggen',
  'verb.double': 'Verdubbelen',
  'verb.draw': 'Pakken',
  'verb.finish_layoff': 'Klaar met aanleggen',
  'verb.fold': 'Passen',
  'verb.hit': 'Kaart',
  'verb.insure': 'Verzekering nemen',
  'verb.knock': 'Kloppen',
  'verb.lay_meld': 'Leggen',
  'verb.lay_off': 'Aanleggen',
  'verb.pass': 'Passen',
  'verb.place': 'Plaatsen',
  'verb.play_card': 'Speel',
  'verb.raise': 'Verhogen',
  'verb.reset_turn': 'Beurt herstellen',
  'verb.split': 'Splitsen',
  'verb.stand': 'Passen',
  'verb.surrender': 'Opgeven',
  'verb.swap_joker': 'Joker ruilen',
  'verb.take': 'Nemen',
  'verb.take_pile': 'Van de stapel nemen',
  // Declared by the server and listed in `serverKeys.json`, but never worded.
  'verb.takePileFromHand': 'Neem de stapel in je hand',
  'verb.takePileOntoMeld': 'Neem de stapel op een combinatie',
  'verb.takeTopForSequence': 'De bovenste kaart op een reeks nemen',
  'verb.undoDraw': 'Pakken ongedaan maken',
  'verb.undoLayOff': 'Aanleggen ongedaan maken',
  'verb.undoMeld': 'Combinatie ongedaan maken',
  'verb.undoTakePile': 'Stapel nemen ongedaan maken',
  'verb.undoTurn': 'Beurt ongedaan maken',

  // --- suits, spelled out ---------------------------------------------------
  //
  // Read aloud by a screen reader and shown where a pip would not fit. The
  // pips themselves are drawn, not written, so these are the only place the
  // suit is ever a word.
  'suit.C': 'Klaveren',
  'suit.D': 'Ruiten',
  'suit.H': 'Harten',
  'suit.S': 'Schoppen',

  // --- counters the board prints beside a number ----------------------------
  'canasta.seat.notOpened': 'Niet geopend',
  'canasta.unit.points': 'punten',
  'ginrummy.unit.points': 'punten',
  'holdem.seat.dealer': 'Deler',
  'holdem.seat.folded': 'Gepast',
  'holdem.seat.allIn': 'All-in',
  'holdem.seat.out': 'Uit',
  'holdem.unit.chips': 'fiches',
  'prsi.unit.cardsLeft': 'kaarten over',
  'rummytiles.prompt.initialMeld': 'Je eerste leg moet {n} punten waard zijn.',
  'rummytiles.unit.points': 'punten',
  'zolik.unit.penalty': 'strafpunten',
  'header.pileFrozen': 'Stapel bevroren',
  // The module-less form of `ginrummy.prompt.yourTurnDraw`: Žolíky and Canasta
  // send the prompt without a module prefix. Neither the Go constants nor
  // `serverKeys.json` list it — the sweep is what found it.
  'prompt.yourTurnDraw': 'Pak een kaart',
  'prompt.yourTurnMeld': 'Leg af als je kunt, gooi dan weg',
};
