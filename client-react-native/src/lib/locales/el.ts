/**
 * Greek. Ρέμι vocabulary: ομάδα for a set, κέντα for a run, συνδυασμός for a meld, μπαλαντέρ for a joker.
 */

export const el: Record<string, string> = {
  // --- engine error codes (rules.RulesErrorCode) ---------------------------
  'err.NOT_YOUR_TURN': 'Δεν είναι η σειρά σου',
  'err.WRONG_PHASE': 'Δεν γίνεται αυτή τη στιγμή',
  'err.MUST_DRAW_FIRST': 'Τράβα ένα φύλλο πριν κατεβάσεις',
  'err.GAME_SUSPENDED': 'Το παιχνίδι είναι σε παύση',
  'err.GAME_NOT_ACTIVE': 'Το παιχνίδι δεν τρέχει',
  // --- runtime refusals, which are not the rules saying no -----------------
  //
  // Both were reachable long before they were worded, and a player who met
  // one was shown the code in capitals or, worse, nothing at all: an action
  // sent while the socket was down simply vanished, which reads as a legal
  // move being silently refused.
  'err.MATCH_NOT_ACTIVE': 'Το τραπέζι είναι σε παύση — περιμένουμε να επανασυνδεθεί ένας παίκτης',
  'err.NOT_CONNECTED': 'Δεν υπάρχει σύνδεση με το τραπέζι — γίνεται επανασύνδεση, δοκίμασε μετά',
  // Why the "start the next round" control is greyed out. Without wording the
  // shell printed the code itself, so a player agreeing to go on was answered
  // with ALREADY_READY in capitals.
  'err.ALREADY_READY': 'Είσαι έτοιμος',
  'err.NOT_BETWEEN_ROUNDS': 'Ο γύρος παίζεται ακόμα',
  'err.NOT_AT_THIS_TABLE': 'Δεν είσαι σε αυτό το τραπέζι',
  'err.DISCARD_LOCKED': 'Ο σωρός απόρριψης είναι προς το παρόν κλειδωμένος',
  'err.DISCARD_PILE_EMPTY': 'Ο σωρός απόρριψης είναι άδειος',
  'err.NO_CARDS_LEFT': 'Δεν έμειναν φύλλα για τράβηγμα',
  'err.ROUND_REQ_NOT_MET': 'Κατέβασε πρώτα το δικό σου άνοιγμα',
  'err.NEED_CLEAN_RUN': 'Χρειάζεσαι μια κέντα χωρίς μπαλαντέρ στο τραπέζι για να μετρήσεις ως κατεβασμένος',
  'err.INCOMPLETE_INITIAL_MELD': 'Ολοκλήρωσε το κατέβασμα, ή ανέβασέ το πίσω, πριν πετάξεις',
  'err.DISCARD_CARD_NOT_MELDED': 'Το φύλλο που πήρες πρέπει να μπει στον συνδυασμό σου',
  'err.JOKER_DISCARD_FORBIDDEN': 'Ο μπαλαντέρ δεν πετιέται',
  'err.NOTHING_TO_UNDO': 'Δεν υπάρχει τίποτα να αναιρεθεί',
  'err.NO_JOKER_IN_MELD': 'Δεν υπάρχει μπαλαντέρ σε αυτόν τον συνδυασμό',
  'err.JOKER_SWAP_MISMATCH': 'Αυτό το φύλλο δεν παίρνει τη θέση του μπαλαντέρ',
  'err.RECLAIMED_JOKER_NOT_MELDED':
    'Ο μπαλαντέρ που πήρες από το τραπέζι πρέπει να παιχτεί σε συνδυασμό αυτόν τον γύρο',
  'err.RUN_TOO_LONG': 'Αυτή η κέντα έχει ήδη το πλήρες μήκος της',
  'err.WRONG_RUN_END': 'Αυτό το φύλλο επεκτείνει την άλλη άκρη της κέντας',
  'err.INVALID_MELD': 'Κανένα φύλλο στο χέρι σου δεν ταιριάζει εδώ',
  'err.CARD_NOT_IN_HAND': 'Αυτό το φύλλο δεν είναι στο χέρι σου',
  'err.MELD_BELOW_MINIMUM': 'Στους συνδυασμούς σου λείπουν ακόμα πόντοι για να κατεβάσεις',
  'err.MELD_NO_CONTRIBUTION': 'Αυτός ο συνδυασμός δεν προχωρά την απαίτησή σου',
  'err.TOO_MANY_WILDS': 'Πάρα πολλοί μπαλαντέρ σε αυτόν τον συνδυασμό',
  'err.ADJACENT_WILDS': 'Δύο μπαλαντέρ δεν μπορούν να είναι δίπλα-δίπλα',
  'err.ACE_BRIDGE': 'Ο άσος δεν μπορεί να γεφυρώσει ρήγα και δυάρι',

  // --- contract phrasing ---------------------------------------------------
  // Built from the structured contract the server sends (sets/runs counts),
  // never from a deal-number lookup table.
  // Whole phrases per count rather than a number plus a pluralised noun:
  // Czech inflects the noun by count (skupina / skupiny / skupin) in a way no
  // "add an s" helper survives, and the counts that actually occur are 1-3.
  // A count outside that falls back to contract.sets.n.
  'contract.sets.1': 'Μία ομάδα',
  'contract.sets.2': 'Δύο ομάδες',
  'contract.sets.3': 'Τρεις ομάδες',
  'contract.sets.n': '{n} ομάδες',
  'contract.runs.1': 'Μία κέντα',
  'contract.runs.2': 'Δύο κέντες',
  'contract.runs.3': 'Τρεις κέντες',
  'contract.runs.n': '{n} κέντες',
  'contract.any': 'Οποιοσδήποτε έγκυρος συνδυασμός',
  'contract.cleanRunOnly':
    'Οποιοσδήποτε συνδυασμός ομάδων και κεντών — τουλάχιστον μία κέντα πρέπει να είναι χωρίς μπαλαντέρ',
  'contract.cleanRunSuffix': '{base} — μία κέντα πρέπει να είναι χωρίς μπαλαντέρ',

  // --- written rules ---------------------------------------------------------
  // Full-sentence keys a "see the rules" screen renders, one per module,
  // resolved server-side against the table's actual variation and options —
  // see internal/module/rules.go and each game's rules.go. Shared section
  // titles first, then one block per module in the order the four games were
  // added.
  'zolik.rules.section.goal': 'Στόχος',
  'zolik.rules.section.setup': 'Στήσιμο',
  'zolik.rules.section.turn': 'Η σειρά σου',
  'zolik.rules.section.melding': 'Κατέβασμα',
  'zolik.rules.section.end': 'Πώς τελειώνει ο αγώνας',
  'zolik.rules.goal':
    'Γίνε ο πρώτος που θα αδειάσει το χέρι του κατεβάζοντας έγκυρες ομάδες και κέντες, μαζεύοντας όσο το δυνατόν λιγότερους πόντους ποινής στα φύλλα που κρατάς ακόμα όταν βγει κάποιος άλλος.',
  'zolik.rules.deal': 'Κάθε παίκτης παίρνει {n} φύλλα.',
  'zolik.rules.meldShapes':
    'Η ομάδα είναι {set}+ φύλλα ίδιας αξίας· η κέντα είναι {run}+ συνεχόμενα φύλλα ίδιου χρώματος.',
  'zolik.rules.turn.draw': 'Στη σειρά σου τράβα ένα φύλλο — από την τράπουλα ή από τον σωρό απόρριψης.',
  'zolik.rules.pickup.topOnly': 'Μόνο το πάνω φύλλο του σωρού απόρριψης μπορεί να παρθεί.',
  'zolik.rules.pickup.anyFromPile':
    'Οποιοδήποτε φύλλο του σωρού απόρριψης μπορεί να παρθεί, μαζί με ό,τι βρίσκεται πάνω του.',
  'zolik.rules.pickup.locked': 'Από τον σωρό απόρριψης δεν τραβάς πριν από τον γύρο {n}.',
  'zolik.rules.pickup.open': 'Ο σωρός απόρριψης είναι ανοιχτός από τον πρώτο γύρο.',
  'zolik.rules.turn.discard': 'Τέλειωσε τη σειρά σου πετώντας ένα φύλλο.',
  'zolik.rules.jokers.restricted':
    'Ο μπαλαντέρ δεν πετιέται ποτέ, εκτός αν είναι ακριβώς το φύλλο που αδειάζει το χέρι σου.',
  'zolik.rules.lead.rotate':
    'Το πρώτο παίξιμο μετακινείται μία θέση σε κάθε μοιρασιά, ανεξάρτητα από το ποιος κέρδισε.',
  'zolik.rules.lead.winner': 'Όποιος βγει παίζει πρώτος στην επόμενη μοιρασιά.',
  'zolik.rules.meldFloor.on':
    'Το πρώτο σου κατέβασμα πρέπει να φτάνει τουλάχιστον {n} φυσικούς πόντους για να είσαι κατεβασμένος.',
  'zolik.rules.meldFloor.off': 'Δεν υπάρχει ελάχιστη αξία πόντων στο πρώτο σου κατέβασμα.',
  'zolik.rules.cleanRun.on':
    'Τουλάχιστον μία από τις κέντες σου πρέπει να είναι εντελώς χωρίς μπαλαντέρ για να μετράς ως κατεβασμένος.',
  'zolik.rules.cleanRun.off':
    'Οι κέντες σου μπορούν να χρησιμοποιούν μπαλαντέρ ελεύθερα — καμία δεν χρειάζεται να είναι χωρίς.',
  'zolik.rules.contracts.rotating':
    'Ο αγώνας κρατά {n} μοιρασιές, και κάθε μοιρασιά απαιτεί τον δικό της συνδυασμό ομάδων και κεντών.',
  'zolik.rules.contracts.static':
    'Κάθε μοιρασιά απαιτεί τον ίδιο συνδυασμό: {sets} ομάδες και {runs} κέντες.',
  'zolik.rules.end.afterDeals': 'Ο αγώνας τελειώνει μετά από {n} μοιρασιές.',
  'zolik.rules.end.atScore': 'Μοιράζεται ξανά μέχρι κάποιος να φτάσει τους {n} πόντους — τότε τελειώνει.',

  'prsi.rules.section.goal': 'Στόχος',
  'prsi.rules.section.setup': 'Στήσιμο',
  'prsi.rules.section.turn': 'Η σειρά σου',
  'prsi.rules.section.special': 'Ειδικά φύλλα',
  'prsi.rules.section.end': 'Πώς τελειώνει ο αγώνας',
  'prsi.rules.goal': 'Γίνε ο πρώτος που θα παίξει όλα τα φύλλα του χεριού του.',
  'prsi.rules.deck': 'Παίζεται με τράπουλα {value} φύλλων (από το 7 και πάνω).',
  'prsi.rules.deal': 'Κάθε παίκτης ξεκινά με {n} φύλλα.',
  'prsi.rules.turn.match':
    'Παίξε ένα φύλλο που ταιριάζει στο χρώμα ή στην αξία του πάνω φύλλου — ή τράβα, αν δεν μπορείς.',
  'prsi.rules.turn.draw': 'Το τράβηγμα τελειώνει τη σειρά σου χωρίς παίξιμο.',
  'prsi.rules.sevens':
    'Παίξε ένα 7 και ο επόμενος παίκτης τραβά δύο φύλλα, εκτός αν απαντήσει με δικό του 7.',
  'prsi.rules.aces': 'Παίξε άσο και η σειρά του επόμενου παίκτη προσπερνιέται.',
  'prsi.rules.queens': 'Παίξε ντάμα και πες το χρώμα που συνεχίζει.',
  'prsi.rules.end': 'Ο αγώνας τελειώνει τη στιγμή που το χέρι κάποιου αδειάσει.',

  'canasta.rules.section.goal': 'Στόχος',
  'canasta.rules.section.setup': 'Στήσιμο',
  'canasta.rules.section.melding': 'Κατέβασμα',
  'canasta.rules.section.end': 'Πώς τελειώνει ο αγώνας',
  'canasta.rules.goal':
    'Παίζεται σε ζευγάρια· η πρώτη πλευρά που φτάνει τους {n} πόντους κερδίζει τον αγώνα.',
  'canasta.rules.deck': 'Παίζεται με {value} φύλλα — δύο τράπουλες συν μπαλαντέρ.',
  'canasta.rules.deal': 'Κάθε παίκτης παίρνει {n} φύλλα.',
  'canasta.rules.redThrees':
    'Ένα κόκκινο τριάρι στο χέρι σου φανερώνεται αμέσως και μετρά ως μπόνους — εκτός αν η πλευρά σου δεν ολοκληρώσει ποτέ καναστα, οπότε μετρά εναντίον σου.',
  'canasta.rules.canasta': 'Καναστα είναι ένας συνδυασμός {n} ή περισσότερων φύλλων ίδιας αξίας.',
  'canasta.rules.meldFloorBands':
    'Το πρώτο σου κατέβασμα πρέπει να φτάσει ένα ελάχιστο πόντων που ανεβαίνει με το σκορ σου: {negative} κάτω από το μηδέν, {low} έως 1500, {mid} έως 3000, {high} πιο πάνω.',
  'canasta.rules.oneCanastaToGoOut': 'Μία ολοκληρωμένη καναστα αρκεί για να βγει η πλευρά σου.',
  'canasta.rules.twoCanastasToGoOut':
    'Η πλευρά σου χρειάζεται δύο ολοκληρωμένες καναστες πριν μπορέσει να βγει.',
  'canasta.rules.end':
    'Μοιράζεται ξανά μέχρι μια πλευρά να ξεπεράσει τους {n} πόντους — τότε ο αγώνας τελειώνει.',

  'holdem.rules.section.goal': 'Στόχος',
  'holdem.rules.section.setup': 'Στήσιμο',
  'holdem.rules.section.betting': 'Στοιχηματισμός',
  'holdem.rules.section.end': 'Πώς τελειώνει ο αγώνας',
  'holdem.rules.goal':
    'Κέρδισε μάρκες έχοντας το καλύτερο χέρι στο φανέρωμα, ή μένοντας ο μόνος παίκτης στη μοιρασιά.',
  'holdem.rules.stack': 'Κάθε θέση ξεκινά με {n} μάρκες.',
  'holdem.rules.blinds': 'Το μικρό τυφλό είναι {sb} και το μεγάλο {bb}, μπαίνουν πριν μοιραστούν τα φύλλα.',
  'holdem.rules.streets':
    'Ο στοιχηματισμός γίνεται σε τέσσερις γύρους — πριν το φλοπ και μετά το φλοπ, το τερν και το ρίβερ.',
  'holdem.rules.showdown':
    'Όσοι είναι ακόμα στη μοιρασιά δείχνουν τα φύλλα τους· το καλύτερο πεντάφυλλο χέρι παίρνει το πότ.',
  'holdem.rules.noLimit': 'Χωρίς όριο — κάθε στοίχημα μπορεί να φτάσει ως ολόκληρο το στακ σου.',
  'holdem.rules.lastPlayerStanding': 'Παίζεται μέχρι μία θέση να κρατά όλες τις μάρκες.',
  'holdem.rules.mostChipsWins':
    'Όποιος κρατά τις περισσότερες μάρκες όταν σταματήσει το παιχνίδι κερδίζει τον αγώνα.',
  'holdem.rules.handLimit': 'Το παιχνίδι σταματά μετά από {n} μοιρασιές.',

  // --- header --------------------------------------------------------------
  'header.deal': 'Μοιρασιά {n}',
  'header.gameOf': 'Παιχνίδι {n} από {total}',
  'header.gameOfWithContract': 'Παιχνίδι {n} από {total}: {contract}',

  // --- meld preview --------------------------------------------------------
  'preview.validSet': 'Έγκυρη ομάδα',
  'preview.validRun': 'Έγκυρη κέντα',
  'preview.validMeld': 'Έγκυρος συνδυασμός',
  'preview.notYet': 'Δεν είναι ακόμα συνδυασμός',
  'preview.points': '{shape} · {n} πόντοι',
  'preview.pointsWithLaid': '{shape} · {n} + {laid} ήδη κατεβασμένοι = {total} πόντοι',
  'preview.meetsFloor': '{line} (φτάνει τους {n} ✓)',
  'preview.needsFloor': '{line} (χρειάζεται {n} ✗)',
  'preview.becauseOf': '{line} — {reason}',

  // --- discard -------------------------------------------------------------
  // A discard with a meld staged lays that meld first; this is what the
  // player is told when the server refuses it and the whole move is rolled
  // back. {reason} is the engine's own words for the refusal.
  'discard.meldRejected': '{reason} — δεν πετάχτηκε τίποτα, τα φύλλα σου είναι ακόμα έτοιμα.',

  // --- what the current selection would send -------------------------------
  // The client's own reason a control is not ready, next to the engine's own
  // `err.*` reasons rather than a second kind of message — a control greyed
  // out for "not your turn" and one greyed out for "you picked two, this
  // takes one" should read as the same kind of thing, not one of them looking
  // broken. See `fits` in `src/lib/drops.ts`.
  'sel.tooMany.1': 'Διάλεξε μόνο ένα φύλλο',
  'sel.tooMany.n': 'Διάλεξε το πολύ {n} φύλλα',
  'sel.needMore': 'Διάλεξε {n} φύλλο/α',
  'sel.notThese': 'Αυτά τα φύλλα δεν μπορούν να πάνε εδώ',
  'sel.needsCompany': 'Αυτό το φύλλο χρειάζεται τα διπλανά του',

  // --- what a module says happened -----------------------------------------
  // The first module-sent keys in this bundle, and deliberately few. Anything
  // a module sends is still legible without an entry here — `humanise` turns
  // `holdem.seat.stack` into "Stack" — so a key earns a line only when that
  // fallback loses something. These do: what each of them means lives in its
  // *params* (who won, how much, whose cards), and a key on its own has
  // nowhere to put them. Without the entry the player read "Winner", full
  // stop, at the end of a match they had just won.
  'status.winner': 'Κερδίστηκε από {winners}',
  'holdem.status.pot': 'Ο {winners} κέρδισε {amount} με {hand}',
  'holdem.status.potUncontested': 'Ο {winners} κέρδισε {amount} — όλοι οι άλλοι πάσαραν',
  'holdem.status.shown': 'Ο {playerId} έδειξε {value}',
  'holdem.prompt.waitingFor': 'Αναμονή για {playerId}',

  // How a rummy scoreboard was ordered. Both earn a line for the same reason
  // as the keys above: the number lives in the params, and `humanise` would
  // render "Deals Won" with nothing after it.
  'zolik.standing.dealsWon': 'Κερδισμένες μοιρασιές {n}',
  'zolik.standing.inHand': 'Στο χέρι {n}',

  // --- rounds ---------------------------------------------------------------
  // What a round is called, and what one did. Each of these carries its number
  // in the params, so `humanise` alone would render a label with nothing after
  // it.
  // An instruction, not a heading. "Next round" named the thing rather than
  // asking for it, and sat above "Waiting for 1" — which reads as though the
  // player is the one waiting, when the table is waiting for them.
  'round.continue': 'Ξεκίνα τον επόμενο γύρο',

  // --- the flash that announces a round or a match ending ------------------
  //
  // One line and one number, held for about two seconds. Worded generically on
  // purpose: which *kind* of ending it was is the module's own fact, printed
  // underneath, and a client that phrased "went gin" itself would be a client
  // that knows a game.
  'flash.roundWon': 'Ο {winners} τον πήρε',
  'flash.roundWonYou': 'Τον πήρες',
  'flash.roundDrawn': 'Δεν τον πήρε κανείς',
  'flash.matchOver': 'Ο αγώνας τελείωσε',
  'flash.matchWon': 'Ο {winners} κέρδισε',
  'flash.matchWonYou': 'Κέρδισες',
  'flash.matchDrawn': 'Δεν κέρδισε κανείς',
  'flash.nowOn': 'τώρα {total}',

  'zolik.round.deal': 'Μοιρασιά',
  'zolik.round.cleanRun': 'Μία κέντα πρέπει να είναι χωρίς μπαλαντέρ',
  'canasta.round.deal': 'Μοιρασιά',
  'canasta.round.concealed': 'Βγήκε κρυφά',
  'canasta.round.exhausted': 'Η τράπουλα τελείωσε',
  'canasta.round.meldCards': 'Κατεβασμένα φύλλα {n}',
  'canasta.round.canastas': 'Καναστες {n}',
  'canasta.round.redThrees': 'Κόκκινα τριάρια {n}',
  'canasta.round.goingOut': 'Βγήκε {n}',
  'canasta.round.inHand': 'Έμειναν στο χέρι {n}',
  'holdem.round.hand': 'Μοιρασιά',
  'holdem.round.pot': 'Πότ {n}',
  'holdem.round.uncontested': 'Όλοι οι άλλοι πάσαραν',
  'seat.ready': 'Έτοιμος',



  // --- refusals the other three games raise ---------------------------------
  // Worded here for the same reason Žolíky's are: reasonText falls back to the
  // code, and OfferBar hands it the code as that fallback, so an unworded
  // refusal reaches a player as PILE_FROZEN in capitals. serverKeys.test.ts
  // now fails the build rather than letting the next one through.
  'err.SET_TOO_LARGE': 'Η ομάδα έχει ήδη και τα τέσσερα χρώματα',
  'err.DISCARD_TAKEN_CARD_FORBIDDEN':
    "Δεν μπορείς να πετάξεις το φύλλο που μόλις πήρες — παίξ' το ή κράτα το",
  'err.CARD_DOES_NOT_FIT': 'Αυτό το φύλλο δεν ταιριάζει ούτε στο χρώμα ούτε στην αξία',
  'err.SUIT_REQUIRED': 'Πες το χρώμα που συνεχίζει',
  'err.MUST_ANSWER_DRAW_OR_TAKE': 'Απάντησε με εφτάρι, ή πάρε τα φύλλα',
  'err.NOTHING_TO_DRAW': 'Δεν έμεινε τίποτα να τραβήξεις',
  'err.PILE_EMPTY': 'Ο σωρός είναι άδειος',
  'err.PILE_BLOCKED': 'Ο σωρός είναι μπλοκαρισμένος — πάνω βρίσκεται ένα μαύρο τριάρι',
  'err.PILE_FROZEN': 'Ο σωρός είναι παγωμένος — χρειάζεσαι δύο φυσικά φύλλα της αξίας του πάνω φύλλου',
  'err.TOP_CARD_UNUSABLE': 'Δεν μπορείς να χρησιμοποιήσεις το πάνω φύλλο',
  'err.MELD_CLOSED': 'Αυτός ο συνδυασμός είναι πλήρης και κλειστός',
  'err.MELD_TOO_SMALL': 'Ένας συνδυασμός χρειάζεται περισσότερα φύλλα από αυτά',
  'err.MELD_TOO_LARGE': 'Αυτός ο συνδυασμός δεν χωρά άλλα φύλλα',
  'err.MELD_MIXED_RANKS': 'Κάθε φύλλο σε έναν συνδυασμό πρέπει να έχει την ίδια αξία',
  'err.NOT_ENOUGH_NATURALS': 'Ένας συνδυασμός χρειάζεται περισσότερα φυσικά φύλλα από μπαλαντέρ',
  'err.RANK_ALREADY_MELDED': 'Η πλευρά σου έχει ήδη συνδυασμό αυτής της αξίας',
  'err.NOT_YOUR_MELD': 'Αυτός ο συνδυασμός ανήκει στην αντίπαλη πλευρά',
  'err.NO_SUCH_MELD': 'Αυτός ο συνδυασμός δεν είναι στο τραπέζι',
  'err.CANNOT_MELD_THREE': 'Τα τριάρια δεν κατεβαίνουν ποτέ',
  'err.CANNOT_DISCARD_RED_THREE': 'Το κόκκινο τριάρι δεν πετιέται',
  'err.MUST_KEEP_A_CARD': 'Κράτα τουλάχιστον ένα φύλλο — έτσι δεν μπορείς να αδειάσεις το χέρι σου',
  'err.MUST_MELD_FIRST': 'Κατέβασε πρώτα το άνοιγμα της πλευράς σου',
  'err.INITIAL_MELD_NOT_MET': 'Στο πρώτο σου κατέβασμα λείπουν ακόμα πόντοι',
  'err.CANNOT_GO_OUT_YET': 'Η πλευρά σου χρειάζεται ολοκληρωμένη καναστα πριν μπορέσει να βγει',
  'err.NOTHING_TO_CALL': 'Δεν υπάρχει στοίχημα για πάσο',
  'err.CANNOT_CHECK': 'Δεν μπορείς να τσεκάρεις — υπάρχει στοίχημα να απαντήσεις',
  'err.CANNOT_RAISE': 'Εδώ δεν μπορείς να ανεβάσεις',
  'err.RAISE_TOO_SMALL': 'Το ανέβασμα πρέπει να είναι τουλάχιστον όσο το προηγούμενο',
  'err.NOT_ENOUGH_CHIPS': 'Δεν έχεις τόσες μάρκες',
  'err.AMOUNT_REQUIRED': 'Πες πόσο',
  'err.AMOUNT_NOT_A_NUMBER': 'Αυτό το ποσό δεν είναι αριθμός',
  'err.SEAT_NOT_IN_HAND': 'Δεν είσαι σε αυτή τη μοιρασιά',
  'err.WRONG_RANK': "Αυτό το φύλλο έχει λάθος αξία γι' αυτό",
  'err.MATCH_FULL': 'Το τραπέζι είναι γεμάτο',
  'err.MATCH_ALREADY_STARTED': 'Ο αγώνας έχει ήδη αρχίσει',
  'err.TOO_FEW_PLAYERS': 'Δεν υπάρχουν ακόμα αρκετοί παίκτες',
  'err.WRONG_PLAYER_COUNT': 'Αυτό το παιχνίδι δεν παίζεται με τόσους παίκτες',
  'err.NOT_THE_HOST': 'Αυτό μπορεί να το κάνει μόνο ο οικοδεσπότης',
  'err.NO_LONGER_WAITING': 'Το τραπέζι δεν περιμένει πια',
  'err.WAITING_ROOM_UNAVAILABLE': 'Η αίθουσα αναμονής δεν είναι διαθέσιμη',
  'err.SERVER_BUSY': 'Ο διακομιστής είναι γεμάτος αυτή τη στιγμή — δοκίμασε ξανά σε λίγο',


  // --- rules the engine has always enforced and never stated ----------------
  // Every one of these was found by the guardrail rather than by review: the
  // validator could refuse a player for it, and no sentence anywhere said so.
  // See server/internal/zolikmod/ruleindex_test.go.
  'zolik.rules.section.layoff': 'Προσθήκη σε συνδυασμούς',
  'zolik.rules.pickup.obligation':
    'Πριν κατεβάσεις, ένα φύλλο που πάρθηκε από τον σωρό απόρριψης πρέπει να χρησιμοποιηθεί στον συνδυασμό με τον οποίο κατεβαίνεις αυτόν τον γύρο.',
  'zolik.rules.pickup.noReturn':
    "Ένα φύλλο που πήρες από τον σωρό απόρριψης δεν ξαναπετιέται στον ίδιο γύρο — παίξ' το ή κράτα το.",
  'zolik.rules.wilds.setLimit': 'Μια ομάδα δεν μπορεί να έχει περισσότερους μπαλαντέρ από φυσικά φύλλα.',
  'zolik.rules.set.maxSize':
    'Μια ομάδα δεν μπορεί να έχει πάνω από {n} φύλλα — ο μπαλαντέρ καλύπτει ένα χρώμα που λείπει, δεν γεμίζει μια πλήρη ομάδα.',
  'zolik.rules.run.maxLength':
    'Μια κέντα δεν μπορεί να έχει πάνω από {n} φύλλα — ο άσος κάτω, οι δώδεκα αξίες από πάνω και ο άσος στην κορυφή.',
  'zolik.rules.run.aceBridge':
    'Ο άσος κάθεται πάνω από τον ρήγα ή κάτω από το δυάρι, ποτέ ως γέφυρα ανάμεσα στις δύο άκρες μιας κέντας.',
  'zolik.rules.contracts.contribution':
    'Μέχρι να κατεβάσεις, κάθε συνδυασμός που κατεβάζεις πρέπει να είναι από αυτούς που ζητά ακόμα το συμβόλαιο της μοιρασιάς.',
  'zolik.rules.layoff.afterDown':
    'Δεν μπορείς να προσθέσεις σε ξένους συνδυασμούς μέχρι να κατεβάσεις το δικό σου συμβόλαιο.',
  'zolik.rules.layoff.runEnds':
    'Ένα φύλλο που προστίθεται σε κέντα πρέπει να τη συνεχίζει στη μία ή στην άλλη άκρη.',
  'zolik.rules.jokers.swap':
    'Ένας μπαλαντέρ σε συνδυασμό στο τραπέζι μπορεί να εξαγοραστεί με ακριβώς το φύλλο που αντιπροσωπεύει.',
  'zolik.rules.jokers.reclaim.on':
    'Ένας μπαλαντέρ που εξαγοράστηκε από το τραπέζι πρέπει να παιχτεί σε συνδυασμό τον ίδιο γύρο — δεν μένει στο χέρι.',
  'zolik.rules.jokers.reclaim.off':
    'Ένας μπαλαντέρ που εξαγοράστηκε από το τραπέζι μπορεί να μείνει στο χέρι.',
  'zolik.rules.deck.reshuffle':
    'Όταν τελειώσει η τράπουλα, ο σωρός απόρριψης ανακατεύεται και γίνεται η νέα τράπουλα· αν και τα δύο είναι άδεια, η μοιρασιά τελειώνει.',


  // --- what to do instead ---------------------------------------------------
  // The third layer of a refusal, after the reason and the rule. Sent by the
  // module because it is the only side that knows which card is owed and
  // which way out is on offer; see server/internal/zolikmod/remedy.go.
  'zolik.remedy.meldThePickup': 'Πρόσθεσε το {card} στο κατέβασμά σου, ή ακύρωσε το πάρσιμο.',
  'zolik.remedy.discardSomethingElse': 'Πέταξε άλλο φύλλο, ή παίξε το {card} αυτόν τον γύρο.',
  'zolik.remedy.discardNotAJoker': 'Πέταξε κάτι άλλο εκτός από μπαλαντέρ.',
  'zolik.remedy.finishOrUndoLayDown': "Ολοκλήρωσε το κατέβασμά σου, ή πάρ' το πίσω.",
  'zolik.remedy.needMorePoints': 'Χρειάζεσαι {n} πόντους ακόμα για να μπορέσεις να κατεβάσεις.',
  'zolik.remedy.layACleanRun': 'Κατέβασε μια κέντα χωρίς μπαλαντέρ μέσα.',
  'zolik.remedy.playReclaimedJoker': 'Παίξε το {card} σε συνδυασμό, ή ακύρωσε το πάρσιμο.',
  'zolik.remedy.goDownFirst': 'Κατέβασε πρώτα τους δικούς σου συνδυασμούς.',
  'zolik.remedy.drawFirst': 'Τράβα πρώτα ένα φύλλο.',
  'zolik.remedy.drawFromStock': 'Τράβα από την τράπουλα — ο σωρός απόρριψης ανοίγει στον γύρο {n}.',
  'zolik.remedy.drawFromStockEmpty': "Τράβα από την τράπουλα αντ' αυτού.",


  // --- keys whose meaning lives in their params -----------------------------
  // The shape fallback renders these with the number missing ("Stack", "Pot"),
  // which is why they earn a line where most keys do not.
  'header.contract': 'Χρειάζεται {sets} ομάδες και {runs} κέντες',
  'header.contract.cleanRunOnly': 'Χρειάζεται κέντα χωρίς μπαλαντέρ',
  'header.round': 'Γύρος {n}',
  'header.deck': 'Τράπουλα',
  'header.target': 'Στόχος',
  'header.suitInPlay': 'Χρώμα σε ισχύ',
  'seat.cards': 'Φύλλα',
  'zolik.offer.meld': 'Κατέβασε',
  'prompt.pickupMustBeMelded':
    'Το {value} ήρθε από τον σωρό απόρριψης — πρέπει να μπει στους συνδυασμούς με τους οποίους κατεβαίνεις αυτόν τον γύρο.',
  'prompt.jokerMustBePlayed':
    'Το {value} ήρθε από το τραπέζι — πρέπει να μπει σε συνδυασμό πριν μπορέσεις να τελειώσεις τη σειρά σου.',
  'prompt.initialMeld': 'Το άνοιγμα της πλευράς σου πρέπει να φτάσει τους {n} πόντους.',
  'prompt.canastasNeeded': 'Στην πλευρά σου λείπουν ακόμα {n} καναστες πριν μπορέσει να βγει.',
  'prompt.mustDrawOrAnswerSeven': 'Απάντησε με εφτάρι, ή τράβα {n} φύλλα.',
  'prompt.chooseSuit': 'Διάλεξε το χρώμα που συνεχίζει',
  'prompt.skipPending': 'Η σειρά σου προσπερνιέται',
  'status.lastDeal': 'Η ομάδα {team} έκανε {value}',
  'status.teamScore': 'Ομάδα {team}: {value}',
  'canasta.offer.rank': 'Αξία',
  'canasta.seat.teamScore': 'Σκορ ομάδας',
  'canasta.seat.canastas': 'Καναστες',
  'holdem.header.pot': 'Πότ',
  'holdem.header.street': 'Γύρος',
  'holdem.header.hand': 'Μοιρασιά',
  'holdem.header.handLimit': 'Μοιρασιές συνολικά',
  'holdem.header.blinds': 'Τυφλά',
  'holdem.cost.call': 'για πάσο',
  'holdem.cost.pot': 'στο πότ',
  'holdem.seat.stack': 'Στακ',
  'holdem.seat.bet': 'Στοίχημα',
  'holdem.prompt.yourAction': 'Σειρά σου',
  'holdem.prompt.raiseTo': 'Ανέβασε σε',
  'zone.yourHand': 'Το χέρι σου',
  'zone.opponentHand': 'Το χέρι του',
  'zone.drawPile': 'Τράπουλα',
  'zone.discardPile': 'Σωρός απόρριψης',
  'zone.melds': 'Συνδυασμοί',
  'zone.teamMelds': 'Συνδυασμοί της πλευράς σου',
  'zone.redThrees': 'Κόκκινα τριάρια',
  'zone.board': 'Τραπέζι',
  'verb.drawFromDeck': 'Τράβα',
  'verb.takeFromDiscard': 'Πάρε από τον σωρό',


  // --- the why sheet's own furniture ---------------------------------------
  // The three layers a refusal is explained in. Labels, not sentences: what
  // goes under each is sent by the server or worded above.
  'why.reason': 'Γιατί όχι',
  'why.rule': 'Ο κανόνας',
  'why.rules': 'Οι κανόνες',
  'why.remedy': 'Τι μπορείς να κάνεις',
  'why.readTheRules': 'Διάβασε τους πλήρεις κανόνες →',
  'why.close': 'Κλείσιμο',
  'why.open': 'γιατί',

  // --- marks on a particular card ------------------------------------------
  // A mark is a refusal that has not happened yet: the rule is enforced at the
  // discard, which is the last possible moment to hear about it, so the card
  // says so while there is still a turn left to act on it.
  'zolik.badge.owedToMeld':
    'Το {card} ήρθε από τον σωρό απόρριψης — πρέπει να μπει στους συνδυασμούς με τους οποίους κατεβαίνεις αυτόν τον γύρο.',
  'zolik.badge.jokerOwed':
    'Το {card} ήρθε από το τραπέζι — πρέπει να μπει σε συνδυασμό πριν μπορέσεις να τελειώσεις τη σειρά σου.',

  // --- the legal notices ----------------------------------------------------
  // Only the furniture. The documents themselves are in `src/legal`, which is
  // a bundle of the same kind with a parity test of its own — prose that long
  // in a flat key map buries the keys this one exists for.
  //
  // The notice is five fragments rather than one sentence with two links glued
  // in, because Czech does not put the link where English does: "souhlasíš s
  // Podmínkami" inflects the noun the link is made of. Fragments let each
  // locale place and decline its own.
  'legal.terms': 'Όροι',
  'legal.privacy': 'Απόρρητο',
  'legal.source': 'Πηγαίος κώδικας',
  'legal.updated': 'Έκδοση {version}',
  'legal.draft':
    'Προσχέδιο — δεν ισχύει ακόμα. Το όνομα, η χώρα και η διεύθυνση επικοινωνίας του φορέα εκμετάλλευσης μένει να συμπληρωθούν.',
  'legal.notice.before': 'Παίζοντας αποδέχεσαι τους ',
  'legal.notice.terms': 'όρους χρήσης',
  'legal.notice.between': '. Τι αποθηκεύεται για σένα περιγράφεται στην ',
  'legal.notice.privacy': 'ενημέρωση απορρήτου',
  'legal.notice.after': '.',
  // --- gin rummy -------------------------------------------------------------
  'err.UPCARD_DECLINED': 'Έχεις ήδη πει πάσο σε αυτό το φύλλο',
  'err.DEADWOOD_TOO_HIGH': 'Το ντεντγουντ σου είναι πολύ ψηλό για χτύπημα',
  'err.CARD_DOES_NOT_EXTEND_MELD': 'Αυτό το φύλλο δεν επεκτείνει αυτόν τον συνδυασμό',
  'ginrummy.rules.setup': 'Στήσιμο',
  'ginrummy.rules.turn': 'Η σειρά σου',
  'ginrummy.rules.melds': 'Συνδυασμοί',
  'ginrummy.rules.knocking': 'Χτύπημα',
  'ginrummy.rules.bigGin': 'Μεγάλο τζιν',
  'ginrummy.rules.layoff': 'Η προσάρτηση',
  'ginrummy.rules.deadHand': 'Η νεκρή μοιρασιά',
  'ginrummy.rules.scoring': 'Βαθμολόγηση μοιρασιάς',
  'ginrummy.rules.match': 'Κερδίζοντας τον αγώνα',
  'ginrummy.rules.lineBonuses': 'Μπόνους στον απολογισμό',
  'ginrummy.rules.deck': 'Παίζεται με τράπουλα {value} φύλλων.',
  'ginrummy.rules.deal': 'Κάθε παίκτης παίρνει {value} φύλλα.',
  'ginrummy.rules.upcard': 'Άλλο ένα φύλλο γυρίζει ανοιχτό για να ξεκινήσει ο σωρός απόρριψης.',
  'ginrummy.rules.drawDiscard':
    'Στη σειρά σου τράβα ένα φύλλο — από την τράπουλα ή τον σωρό απόρριψης — και μετά πέταξε ένα.',
  'ginrummy.rules.setsAndRuns':
    'Συνδυασμός είναι μια ομάδα τριών ή τεσσάρων φύλλων μιας αξίας, ή μια κέντα τριών ή περισσότερων στο ίδιο χρώμα.',
  'ginrummy.rules.aceLow': 'Ο άσος είναι πάντα χαμηλά — δεν υπάρχει κέντα από ντάμα ως άσο.',
  'ginrummy.rules.knockLimit': 'Μπορείς να χτυπήσεις μόλις το ντεντγουντ σου γίνει {n} ή λιγότερο.',
  'ginrummy.rules.oklahoma':
    'Το όριο χτυπήματος σε αυτή τη μοιρασιά ορίζεται από την αξία του ανοιχτού φύλλου.',
  'ginrummy.rules.gin': 'Μηδενικό ντεντγουντ είναι τζιν — το καλύτερο δυνατό χτύπημα.',
  'ginrummy.rules.bigGinBonus':
    'Έντεκα φύλλα όλα σε συνδυασμούς, χωρίς καθόλου πέταμα, είναι μεγάλο τζιν και αξίζει άλλους {n} πόντους.',
  'ginrummy.rules.layoffDescription':
    'Μετά από χτύπημα που δεν είναι τζιν, ο αντίπαλός σου μπορεί να προσαρτήσει το δικό του ντεντγουντ στους συνδυασμούς σου πριν συγκριθούν τα χέρια.',
  'ginrummy.rules.deadHandDescription':
    'Αν η τράπουλα πέσει στα τελευταία δύο φύλλα και κανείς δεν έχει χτυπήσει, η μοιρασιά είναι νεκρή — κανείς δεν βαθμολογείται και ο ίδιος μοιράζει ξανά.',
  'ginrummy.rules.undercut':
    'Αν το ντεντγουντ του αντιπάλου σου δεν είναι μεγαλύτερο από το δικό σου, σε κόβει: παίρνει τη διαφορά, συν {n}.',
  'ginrummy.rules.ginBonus': 'Το τζιν παίρνει ολόκληρο το χέρι του αντιπάλου σου, συν {n}.',
  'ginrummy.rules.target':
    'Ο πρώτος που θα ξεπεράσει τους {n} πόντους στο τέλος μιας μοιρασιάς κερδίζει τον αγώνα.',
  'ginrummy.rules.shutout':
    'Το μπόνους του αγώνα διπλασιάζεται στους {n} αν ο ηττημένος δεν πήρε ούτε έναν πόντο.',
  'ginrummy.rules.box': 'Κάθε μοιρασιά που κέρδισες αξίζει {n} πόντους στο τέλος του αγώνα.',
  'ginrummy.rules.gameBonus': 'Η νίκη στον αγώνα αξίζει άλλους {n} πόντους.',
  'ginrummy.fact.deadwood': 'ντεντγουντ {value}',
  'ginrummy.fact.discardCard': 'Πέταξε {value}',
  'ginrummy.fact.meldCards': 'Στο {value}',
  'ginrummy.header.hand': 'Μοιρασιά {n}',
  'ginrummy.round.kind': '{value}',
  'ginrummy.round.hand': 'Μοιρασιά',
  'ginrummy.seat.score': '{value}',
  'ginrummy.seat.dealer': 'Μοιράζει',
  'ginrummy.status.knocked': 'Ο {playerId} χτύπησε με ντεντγουντ {deadwood}',
  'ginrummy.status.gin': 'Ο {playerId} έκανε τζιν',
  'ginrummy.status.lastHand': 'Τελευταία μοιρασιά: {winner} ({kind}, {delta} πόντοι)',
  'ginrummy.offer.drawStock': 'Τράβα από την τράπουλα',
  'ginrummy.offer.drawDiscard': 'Τράβα από τον σωρό απόρριψης',
  'ginrummy.offer.takeUpcard': 'Πάρε το ανοιχτό φύλλο',
  'ginrummy.offer.passUpcard': 'Πάσο',
  'ginrummy.offer.discard': 'Πέταξε',
  'ginrummy.offer.knock': 'Χτύπα',
  'ginrummy.offer.gin': 'Τζιν!',
  'ginrummy.offer.bigGin': 'Μεγάλο τζιν!',
  'ginrummy.offer.layOff': 'Προσάρτησε',
  'ginrummy.offer.finishLayoff': 'Τέλος προσάρτησης',
  'ginrummy.zone.knockerHand': 'Το χέρι που χτύπησε',
  'ginrummy.zone.melds': 'Συνδυασμοί',
  'ginrummy.prompt.upcardDecision': 'Πάρε το ανοιχτό φύλλο, ή πες πάσο',
  'ginrummy.prompt.yourTurnDraw': 'Τράβα ένα φύλλο',
  'ginrummy.prompt.yourTurnDiscard': 'Πέταξε — ή χτύπα, αν μπορείς',
  'ginrummy.prompt.layoff': 'Προσάρτησε ντεντγουντ, ή τελείωσε',

  // --- rummy tiles -------------------------------------------------------------
  'err.TILE_NOT_IN_HAND': 'Αυτό το πλακίδιο δεν είναι στο χέρι σου',
  'err.TILE_DOES_NOT_FIT': 'Αυτό δεν χωράει εκεί',
  'err.NO_SUCH_SET': 'Αυτός ο συνδυασμός δεν είναι στο τραπέζι',
  'err.INITIAL_MELD_ONLY':
    'Πριν από το πρώτο σου κατέβασμα μπορείς να αναδιατάξεις μόνο τους δικούς σου νέους συνδυασμούς',
  'err.TABLE_NOT_VALID': 'Το τραπέζι δεν είναι ακόμα έγκυρο',
  'err.TRAY_NOT_EMPTY': 'Έχεις ακόμα ασύνδετα πλακίδια να τοποθετήσεις',
  'err.NOTHING_PLAYED': 'Παίξε τουλάχιστον ένα πλακίδιο πριν τελειώσεις τη σειρά σου',
  'err.INITIAL_MELD_TOO_LOW': 'Το πρώτο σου κατέβασμα πρέπει να αξίζει 30 πόντους ή περισσότερους',
  'err.NOT_A_RUN': 'Μόνο μια κέντα μπορεί να χωριστεί',
  'err.BAD_SPLIT_POSITION': 'Εκεί δεν χωρίζεται αυτή η κέντα',
  'err.NO_JOKER_IN_SET': 'Δεν υπάρχει μπαλαντέρ σε αυτόν τον συνδυασμό',
  'err.TILE_JOKER_SWAP_MISMATCH': 'Αυτό το πλακίδιο δεν είναι αυτό που αντιπροσωπεύει ο μπαλαντέρ',
  'rummytiles.rules.setup': 'Στήσιμο',
  'rummytiles.rules.sets': 'Συνδυασμοί',
  'rummytiles.rules.initialMeld': 'Το πρώτο κατέβασμα',
  'rummytiles.rules.turn': 'Η σειρά σου',
  'rummytiles.rules.jokerTaking': 'Πάρσιμο μπαλαντέρ',
  'rummytiles.rules.ending': 'Τέλος γύρου',
  'rummytiles.rules.poolExhaustion': 'Αν στερέψει το απόθεμα',
  'rummytiles.rules.match': 'Κερδίζοντας τον αγώνα',
  'rummytiles.rules.tiles': 'Παίζεται με {value} πλακίδια.',
  'rummytiles.rules.dealCount': 'Κάθε παίκτης παίρνει {value} πλακίδια.',
  'rummytiles.rules.group':
    'Ομάδα είναι τρία ή τέσσερα πλακίδια με τον ίδιο αριθμό, το καθένα σε διαφορετικό χρώμα.',
  'rummytiles.rules.run': 'Κέντα είναι τρεις ή περισσότεροι διαδοχικοί αριθμοί στο ίδιο χρώμα.',
  'rummytiles.rules.noWrap': 'Το 13 δεν γυρίζει πίσω στο 1.',
  'rummytiles.rules.joker': 'Ο μπαλαντέρ αντιπροσωπεύει οποιοδήποτε πλακίδιο.',
  'rummytiles.rules.initialMeldDescription':
    'Μέχρι να κατεβάσεις {n} ή περισσότερους πόντους σε μία μόνο σειρά, μόνο από το δικό σου χέρι, δεν μπορείς να αγγίξεις τίποτα από όσα βρίσκονται ήδη στο τραπέζι.',
  'rummytiles.rules.turnDescription':
    'Παίξε τουλάχιστον ένα πλακίδιο από το χέρι σου, αναδιατάσσοντας ελεύθερα το τραπέζι, και τελείωσε με κάθε συνδυασμό στο τραπέζι έγκυρο.',
  'rummytiles.rules.noDiscard':
    "Δεν υπάρχει πέταμα — αν δεν μπορείς να ολοκληρώσεις έγκυρη σειρά, τραβάς ένα πλακίδιο αντ' αυτού.",
  'rummytiles.rules.jokerTakingDescription':
    'Μπορείς να πάρεις έναν μπαλαντέρ από το τραπέζι αντικαθιστώντας τον με το πλακίδιο που αντιπροσωπεύει, από το χέρι σου — και πρέπει να χρησιμοποιηθεί σε συνδυασμό πριν τελειώσει η σειρά σου.',
  'rummytiles.rules.goingOut':
    'Ο πρώτος παίκτης που μένει χωρίς πλακίδια κερδίζει τον γύρο. Όλοι οι άλλοι παίρνουν την αρνητική αξία όσων τους έμειναν· ο νικητής παίρνει το άθροισμα όσων έχασαν όλοι οι άλλοι.',
  'rummytiles.rules.poolExhaustionLowestWins':
    'Αν στερέψει το απόθεμα και κανείς δεν μπορεί να παίξει, ο γύρος τελειώνει και τον κερδίζει το χέρι με τη χαμηλότερη αξία.',
  'rummytiles.rules.poolExhaustionNoWinner':
    'Αν στερέψει το απόθεμα και κανείς δεν μπορεί να παίξει, ο γύρος τελειώνει χωρίς νικητή — κάθε χέρι απλώς βαθμολογείται.',
  'rummytiles.rules.target':
    'Ο πρώτος που θα ξεπεράσει τους {n} πόντους στο τέλος ενός γύρου κερδίζει τον αγώνα.',
  'rummytiles.rules.roundLimit': 'Ο αγώνας τελειώνει μετά από {n} γύρους — κερδίζει το υψηλότερο σκορ.',
  'rummytiles.fact.setCards': '{value}',
  'rummytiles.header.pool': 'Απόθεμα {n}',
  'rummytiles.header.round': 'Γύρος {n}',
  'rummytiles.round.kind': '{value}',
  'rummytiles.round.round': 'Γύρος',
  'rummytiles.seat.score': '{value}',
  'rummytiles.seat.notOpened': 'Δεν άνοιξε',
  'rummytiles.status.lastRound': 'Τελευταίος γύρος: {winner} ({kind})',
  'rummytiles.badge.invalid': 'Δεν είναι ακόμα έγκυρο',
  'rummytiles.zone.pool': 'Απόθεμα',
  'rummytiles.zone.table': 'Τραπέζι',
  'rummytiles.zone.tray': 'Βάση',
  'rummytiles.offer.place': 'Τοποθέτησε',
  'rummytiles.offer.addFromHand': 'Πρόσθεσε',
  'rummytiles.offer.addFromTray': 'Πρόσθεσε από τη βάση',
  'rummytiles.offer.take': 'Πάρε',
  'rummytiles.offer.split': 'Χώρισε',
  'rummytiles.offer.swapJoker': 'Άλλαξε τον μπαλαντέρ',
  'rummytiles.offer.resetTurn': 'Μηδένισε τη σειρά',
  'rummytiles.offer.commit': 'Έτοιμο',
  'rummytiles.offer.draw': 'Τράβα',
  'rummytiles.param.position': 'Χώρισε στο',

  // --- blackjack -----------------------------------------------------------
  //
  // The house rules are worded twice over — on and off — because a rule that
  // is *not* in force is still something a player has to be told. "No
  // surrender at this table" is information; silence is a guess.
  'err.BET_BELOW_MINIMUM': 'Αυτό είναι κάτω από το ελάχιστο του τραπεζιού',
  'err.ALREADY_BET': 'Το ποντάρισμά σου είναι ήδη κατεβασμένο',
  'err.INSURANCE_CLOSED': 'Δεν υπάρχει ασφάλεια για να πάρεις αυτή τη στιγμή',
  'err.CANNOT_DOUBLE': 'Αυτό το χέρι δεν διπλασιάζεται',
  'err.CANNOT_SPLIT': 'Αυτό το χέρι δεν χωρίζεται',
  'err.CANNOT_SURRENDER': 'Αυτό το χέρι δεν παραδίδεται',

  'blackjack.rules.section.table': 'Το τραπέζι',
  'blackjack.rules.section.play': 'Παίζοντας ένα χέρι',
  'blackjack.rules.section.dealer': 'Ο ντίλερ',
  'blackjack.rules.section.end': 'Πώς τελειώνει ο αγώνας',
  'blackjack.rules.goal':
    'Νίκησε τον ντίλερ χωρίς να ξεπεράσεις το είκοσι ένα. Το ξεπέρασμα χάνει αμέσως, ό,τι κι αν κάνει ο ντίλερ μετά.',
  // Counts are their own phrase rather than a number glued to a noun: one
  // deck and six decks inflect differently, and Czech inflects them again.
  'blackjack.rules.decks': 'Τράπουλες στο παπούτσι: {n}.',
  'blackjack.rules.stack': 'Κάθε θέση κάθεται με {n} μάρκες.',
  'blackjack.rules.minBet': 'Το ελάχιστο του τραπεζιού είναι {n} μάρκες.',
  'blackjack.rules.faceUp':
    'Τα φύλλα των παικτών μοιράζονται ανοιχτά· ο ντίλερ κρατά ένα φύλλο κλειστό μέχρι να παίξουν όλοι.',
  'blackjack.rules.hitStand': 'Τράβα όσα φύλλα θέλεις, ή μείνε σε αυτό που έχεις.',
  'blackjack.rules.aces': 'Ο άσος μετρά έντεκα όσο αυτό χωράει, και ένα όταν δεν χωράει.',
  'blackjack.rules.blackjack': 'Άσος με φύλλο αξίας δέκα, στα δύο πρώτα φύλλα, είναι μπλακ τζακ.',
  'blackjack.rules.pays3to2': 'Το μπλακ τζακ πληρώνει 3:2.',
  'blackjack.rules.pays6to5': 'Το μπλακ τζακ πληρώνει 6:5.',
  'blackjack.rules.paysEven': 'Το μπλακ τζακ πληρώνει ένα προς ένα.',
  'blackjack.rules.double':
    'Στα δύο πρώτα σου φύλλα μπορείς να διπλασιάσεις το ποντάρισμα και να πάρεις ακριβώς ένα ακόμα φύλλο.',
  'blackjack.rules.doubleAfterSplit': 'Ένα χέρι που προήλθε από χώρισμα μπορεί επίσης να διπλασιαστεί.',
  'blackjack.rules.noDoubleAfterSplit': 'Ένα χέρι που προήλθε από χώρισμα δεν διπλασιάζεται.',
  'blackjack.rules.split':
    'Δύο φύλλα ίδιας αξίας μπορούν να χωριστούν σε ξεχωριστά χέρια, καθένα με δικό του ποντάρισμα — έως {n} φορές, για {hands} χέρια συνολικά.',
  'blackjack.rules.noSplit': 'Σε αυτό το τραπέζι τα ζευγάρια δεν χωρίζονται.',
  'blackjack.rules.splitAces':
    'Οι χωρισμένοι άσοι παίρνουν από ένα φύλλο και μετά μένουν, και το είκοσι ένα που γίνεται έτσι δεν είναι μπλακ τζακ.',
  'blackjack.rules.surrender':
    'Μπορείς να παραδώσεις το πρώτο σου χέρι για το μισό ποντάρισμα, αφού ο ντίλερ ελέγξει για μπλακ τζακ.',
  'blackjack.rules.noSurrender': 'Σε αυτό το τραπέζι τα χέρια δεν παραδίδονται.',
  'blackjack.rules.dealerDraws': 'Ο ντίλερ τραβά ως το δεκαεπτά και μετά μένει.',
  'blackjack.rules.hitsSoft17': 'Ο ντίλερ τραβά σε δεκαεπτά που έγινε με άσο.',
  'blackjack.rules.standsSoft17': 'Ο ντίλερ μένει σε δεκαεπτά που έγινε με άσο.',
  'blackjack.rules.dealerPeeks':
    'Δείχνοντας άσο ή δεκάρι, ο ντίλερ ελέγχει για μπλακ τζακ πριν παίξει οποιοσδήποτε.',
  'blackjack.rules.insurance':
    'Απέναντι σε άσο του ντίλερ μπορείς να ασφαλιστείς για το μισό ποντάρισμά σου· πληρώνει 2:1 αν ο ντίλερ έχει μπλακ τζακ.',
  'blackjack.rules.noInsurance': 'Σε αυτό το τραπέζι δεν προσφέρεται ασφάλεια.',
  'blackjack.rules.rounds': 'Το τραπέζι παίζει {n} γύρους.',
  'blackjack.rules.mostChipsWins': 'Όποιος κρατά τις περισσότερες μάρκες στο τέλος κερδίζει τον αγώνα.',
  'blackjack.rules.bustedOut':
    'Μια θέση που δεν μπορεί πια να καλύψει το ελάχιστο των {n} μένει εκτός για το υπόλοιπο του αγώνα.',

  'blackjack.zone.dealer': 'Ντίλερ',
  'blackjack.zone.box': 'Χέρι',
  'blackjack.zone.yourBox': 'Το χέρι σου',
  'blackjack.zone.shoe': 'Παπούτσι',

  'blackjack.header.round': 'Γύρος {n} από {of}',
  'blackjack.header.minBet': 'Ελάχιστο',
  'blackjack.header.decks': 'Τράπουλες',
  'blackjack.header.dealerTotal': 'Ο ντίλερ δείχνει {n}',
  'blackjack.header.dealerSoftTotal': 'Ο ντίλερ δείχνει μαλακό {n}',

  'blackjack.seat.stack': 'Μάρκες',
  'blackjack.seat.bet': 'Ποντάρισμα',
  'blackjack.seat.insurance': 'Ασφάλεια',
  'blackjack.seat.total': 'Σύνολο',
  'blackjack.seat.softTotal': 'Μαλακό σύνολο',
  'blackjack.seat.out': 'Χωρίς μάρκες',

  'blackjack.prompt.placeBet': 'Κάνε το ποντάρισμά σου',
  'blackjack.prompt.insurance': 'Ασφάλεια;',
  'blackjack.prompt.yourMove': 'Σειρά σου',
  'blackjack.prompt.waitingFor': 'Αναμονή για {playerId}',
  'blackjack.prompt.betAmount': 'Ποντάρισμα',

  'blackjack.offer.bet': 'Ποντάρισε',
  'blackjack.offer.hit': 'Φύλλο',
  'blackjack.offer.stand': 'Μένω',
  'blackjack.offer.double': 'Διπλασίασε',
  'blackjack.offer.split': 'Χώρισε',
  'blackjack.offer.surrender': 'Παράδοση',
  'blackjack.offer.insure': 'Πάρε ασφάλεια',
  'blackjack.offer.declineInsurance': 'Χωρίς ασφάλεια',

  'blackjack.fact.tableMinimum': 'ελάχιστο',
  'blackjack.fact.insuranceCost': 'για ασφάλεια',
  'blackjack.fact.extraStake': 'για ποντάρισμα',
  'blackjack.fact.surrenderReturn': 'πίσω',

  'blackjack.status.dealerBlackjack': 'Ο ντίλερ είχε μπλακ τζακ',
  'blackjack.status.dealerBust': 'Ο ντίλερ κάηκε με {n}',
  'blackjack.status.dealerStands': 'Ο ντίλερ μένει στο {n}',

  'blackjack.round.name': 'Γύρος',
  'blackjack.round.dealerTotal': 'Ντίλερ {n}',
  'blackjack.round.dealerBust': 'Ο ντίλερ κάηκε ({n})',
  'blackjack.round.dealerBlackjack': 'Μπλακ τζακ του ντίλερ',
  'blackjack.round.outcome.blackjack': 'Μπλακ τζακ',
  'blackjack.round.outcome.win': 'Κερδισμένο',
  'blackjack.round.outcome.push': 'Ισοπαλία',
  'blackjack.round.outcome.lose': 'Χαμένο',
  'blackjack.round.outcome.bust': 'Κάηκε',
  'blackjack.round.outcome.surrender': 'Παραδόθηκε',

  'blackjack.badge.inPlay': 'Σε παιχνίδι',
  'blackjack.badge.doubled': 'Διπλασιασμένο',
  'blackjack.badge.split': 'Χωρισμένο',
  'blackjack.badge.blackjack': 'Μπλακ τζακ',
  'blackjack.badge.bust': 'Κάηκε',
  'blackjack.badge.won': 'Κερδισμένο',
  'blackjack.badge.push': 'Ισοπαλία',
  'blackjack.badge.lost': 'Χαμένο',
  'blackjack.badge.surrendered': 'Παραδόθηκε',

  'blackjack.unit.chips': 'μάρκες',

  // --- the settings screen --------------------------------------------------
  //
  // Hardcoded English until now, which was survivable while the only other
  // language was unreachable and faintly absurd once the language picker
  // itself lived on this screen: a player who cannot read "Settings" is
  // exactly the one who came here to change it.
  'settings.title': 'Ρυθμίσεις',
  'settings.subtitle': 'Πώς φαίνεσαι εσύ, και πώς το τραπέζι',
  'settings.face.heading': 'Το πρόσωπό σου στο τραπέζι',
  'settings.face.account': 'Φυλάσσεται με τον λογαριασμό σου, οπότε σε ακολουθεί και σε άλλη συσκευή.',
  'settings.face.device': 'Φυλάσσεται σε αυτή τη συσκευή. Συνδέσου για να το πάρεις μαζί σου.',
  'settings.skin.heading': 'Όψη του τραπεζιού',
  'settings.language.heading': 'Γλώσσα',
  'settings.language.status': 'Φυλάσσεται σε αυτή τη συσκευή.',
  'settings.language.auto': 'Αυτόματα',
  'settings.language.auto.now': 'Ακολουθεί τη συσκευή σου — τώρα {language}',
  'settings.legal.heading': 'Τα ψιλά γράμματα',
  'settings.legal.status': 'Τι αποδέχτηκες παίζοντας, και τι αποθηκεύεται για σένα.',
  'settings.signIn': 'Σύνδεση',
  'settings.back': 'Πίσω',

  // A translated interface over an English notice is a worse lie than an
  // English interface, so the document says so itself rather than letting the
  // surrounding screen imply the text was written for this reader.
  'legal.untranslated':
    'Αυτή η ενημέρωση δεν έχει μεταφραστεί ακόμα στη γλώσσα σου. Ισχύει το αγγλικό κείμενο παρακάτω.',

  // --- the navigation bar's screen titles ------------------------------------
  //
  // These sit in `app/_layout.tsx`, above every screen, and were the last
  // English left after the settings screen was translated — a German player
  // reading "Einstellungen" under a bar that said "Settings". A translated app
  // with an untranslated chrome bar looks like a bug, not like a limit.
  'nav.home': 'Zolik',
  'nav.emailSignIn': 'Σύνδεση με email',
  'nav.signingIn': 'Σύνδεση σε εξέλιξη',
  'nav.usernameSignIn': 'Σύνδεση με όνομα χρήστη',
  'nav.legacyAccount': 'Παλιός λογαριασμός',
  'nav.guest': 'Επισκέπτης',
  'nav.account': 'Λογαριασμός',
  'nav.games': 'Παιχνίδια',
  'nav.table': 'Το τραπέζι σου',
  'nav.join': 'Μπες σε τραπέζι',
  'nav.rules': 'Κανόνες',
  'nav.match': 'Αγώνας',
  'nav.scoreTable': 'Πίνακας σκορ',
  'nav.stats': 'Στατιστικά',
};
