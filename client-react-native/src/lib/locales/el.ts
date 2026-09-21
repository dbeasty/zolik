/**
 * Greek. Ρέμι vocabulary: ομάδα for a set, κέντα for a run, συνδυασμός for a meld, μπαλαντέρ for a joker.
 */

export const el: Record<string, string> = {
  // --- engine error codes (rules.RulesErrorCode) ---------------------------
  'err.NOT_YOUR_TURN': 'Δεν είναι η σειρά σου',
  'err.WRONG_PHASE': 'Όχι σε αυτό το σημείο του γύρου',
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
  // Bringing a table back that the sweeper set aside. Refusals a player
  // can actually provoke: a link to a table that is gone, one that was
  // already picked up in another tab, and a table with other people at it.
  'err.MATCH_MOVED_ON': 'Το τραπέζι προχώρησε — φόρτωσε ξανά τη σελίδα',
  'err.MATCH_NOT_ABANDONED': 'Αυτό το τραπέζι δεν περιμένει να συνεχιστεί',
  'err.MATCH_NOT_FOUND': 'Αυτό το τραπέζι δεν υπάρχει πια',
  'err.MATCH_DELETED': 'Ο οικοδεσπότης διέγραψε αυτό το τραπέζι',
  'err.TABLE_HAS_PLAYERS_AWAY': 'Όλοι πρέπει να είναι ξανά στο τραπέζι για να συνεχιστεί το παιχνίδι',
  'err.NOTHING_TO_REPLAY': 'Δεν υπάρχει ακόμη παιχνίδι για αναπαραγωγή εδώ',
  'err.REPLAY_UNAVAILABLE': 'Η αναπαραγωγή παιχνιδιών δεν είναι διαθέσιμη σε αυτόν τον διακομιστή',
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
  'prsi.remedy.matchOrDraw': 'Παίξε ένα φύλλο {suit} ή ένα που ταιριάζει με το {card} — αλλιώς τράβα.',
  'prsi.remedy.answerSevenOrTake': 'Απάντησε με δικό σου εφτάρι ή πάρε τα {n} φύλλα.',
  'prsi.remedy.playOrDraw': 'Δεν σε περιμένει καμία παράλειψη — παίξε ένα φύλλο {suit} ή τράβα.',
  'prsi.remedy.nameASuit': 'Πες ποιο χρώμα ακολουθεί τη ντάμα σου.',
  'prsi.remedy.nothingLeftToDraw': 'Δεν έμεινε τίποτα για τράβηγμα — παίξε φύλλο αν μπορείς.',

  'canasta.rules.section.goal': 'Στόχος',
  'canasta.rules.section.setup': 'Στήσιμο',
  'canasta.rules.section.melding': 'Κατέβασμα',
  'canasta.rules.section.end': 'Πώς τελειώνει ο αγώνας',
  'canasta.rules.section.turn': 'Η σειρά σου',
  'canasta.rules.goal':
    'Παίζεται σε ζευγάρια· η πρώτη πλευρά που φτάνει τους {n} πόντους κερδίζει τον αγώνα.',
  'canasta.rules.deck': 'Παίζεται με {value} φύλλα — {decks} τράπουλες συν μπαλαντέρ.',
  'canasta.rules.deal': 'Κάθε παίκτης παίρνει {n} φύλλα.',
  'canasta.rules.drawCount': 'Στην αρχή του γύρου σου τραβάς {n} φύλλα.',
  'canasta.rules.redThrees':
    'Ένα κόκκινο τριάρι στο χέρι σου φανερώνεται αμέσως και μετρά ως μπόνους — εκτός αν η πλευρά σου δεν ολοκληρώσει ποτέ καναστα, οπότε μετρά εναντίον σου.',
  'canasta.rules.canasta': 'Καναστα είναι ένας συνδυασμός {n} ή περισσότερων φύλλων ίδιας αξίας.',
  'canasta.rules.sequences': 'Ένας συνδυασμός μπορεί να είναι και σειρά: τρία ή περισσότερα φύλλα του ίδιου χρώματος στη σειρά, ποτέ με μπαλαντέρ ανάμεσά τους.',
  'canasta.rules.samba': 'Μια σειρά επτά φύλλων είναι σάμπα και αξίζει {n} πόντους.',
  'canasta.rules.blackThreesGoOut':
    'Το μαύρο τριάρι μπλοκάρει τον σωρό και αξίζει {n} πόντους. Τρία ή τέσσερα από αυτά μπορούν να κατέβουν κατευθείαν από το χέρι σου, ποτέ με μπαλαντέρ ανάμεσά τους, και μόνο ως η κίνηση με την οποία βγαίνει η πλευρά σου.',
  'canasta.rules.blackThreesNeverMeld':
    'Το μαύρο τριάρι δεν κατεβαίνει ποτέ. Όταν το πετάς μπλοκάρει τον σωρό, και αν μείνει στο χέρι σου στο τέλος της μοιρασιάς σού κοστίζει {n} πόντους.',
  'canasta.rules.pileAlwaysFrozen': 'Ο σωρός απόρριψης είναι παγωμένος όλη τη μοιρασιά: μπορείς να τον πάρεις μόνο ταιριάζοντας το πάνω φύλλο με δύο φυσικά φύλλα από το χέρι σου.',
  'canasta.rules.pileOntoMeld': 'Αν η πλευρά σου έχει ήδη έναν ημιτελή συνδυασμό της αξίας του πάνω φύλλου, μπορείς να πάρεις όλον τον σωρό απόρριψης και να προσθέσεις εκεί το φύλλο — δεν χρειάζεσαι ζευγάρι στο χέρι σου.',
  'canasta.rules.pileNoMeldCapture': 'Ένας συνδυασμός που βρίσκεται ήδη στο τραπέζι δεν μπορεί να πάρει τον σωρό απόρριψης: για να τον πάρεις πρέπει να ταιριάξεις το πάνω φύλλο με δύο φύλλα από το ίδιο σου το χέρι.',
  'canasta.rules.meldFloorBands':
    'Το πρώτο σου κατέβασμα πρέπει να φτάσει ένα ελάχιστο πόντων που ανεβαίνει με το σκορ σου: {negative} κάτω από το μηδέν, {low} έως {lowUpTo}, {mid} έως {midUpTo}, {high} πιο πάνω.',
  'canasta.rules.meldFloorBandsFive': 'Ο πρώτος σου συνδυασμός πρέπει να φτάσει ένα ελάχιστο πόντων που ανεβαίνει με το σκορ σου: {negative} κάτω από το μηδέν, {low} ως {lowUpTo}, {mid} ως {midUpTo}, {high} ως {highUpTo} και {top} πάνω από αυτό.',
  'canasta.rules.turn':
    'Ένας γύρος είναι μία κίνηση προς το χέρι σου — τράβηγμα από την τράπουλα ή πάρσιμο όλου του σωρού — μετά όσα μελντ θέλεις, και στο τέλος ένα φύλλο στον σωρό.',
  'canasta.rules.turnDiscard':
    "Ο γύρος τελειώνει με ένα φύλλο στον σωρό, άρα πρέπει πάντα να σου περισσεύει ένα γι' αυτό.",
  'canasta.rules.pileTopCard': 'Ο σωρός παίρνεται μόνο με κίνηση που χρησιμοποιεί αμέσως το πάνω φύλλο του.',
  'canasta.rules.pileBlocked':
    'Ένα μαύρο τρία στην κορυφή μπλοκάρει τον σωρό — κανείς δεν τον παίρνει ώσπου να θαφτεί — και στο χέρι κοστίζει {n}.',
  'canasta.rules.pileFrozenByWild':
    'Ένα θαμμένο μπαλαντέρ παγώνει τον σωρό για όλους: τότε το πάρσιμό του κοστίζει δύο φυσικά φύλλα από το χέρι σου, ίδιας αξίας με το πάνω φύλλο.',
  'canasta.rules.meldShape': 'Ένα μελντ είναι {n} ή περισσότερα φύλλα της ίδιας αξίας.',
  'canasta.rules.wildLimit':
    'Ένα μελντ μπορεί να έχει το πολύ {wilds} μπαλαντέρ και ποτέ λιγότερα από {naturals} φυσικά φύλλα.',
  'canasta.rules.wildRatio':
    'Ένα μελντ χρειάζεται {n} φυσικά φύλλα για κάθε μπαλαντέρ, και ποτέ πάνω από {wilds} μπαλαντέρ συνολικά.',
  'canasta.rules.oneMeldPerRank':
    'Η πλευρά σου κρατά ένα μελντ ανά αξία — τα επιπλέον φύλλα αυτής της αξίας μπαίνουν πάνω του.',
  'canasta.rules.meldsPerRankUnlimited': 'Η πλευρά σου μπορεί να έχει πολλά μελντ της ίδιας αξίας.',
  'canasta.rules.canastaCloses': 'Μια καναστα των {n} φύλλων είναι ολοκληρωμένη και δεν δέχεται άλλα.',
  'canasta.rules.meldsAreShared':
    'Τα μελντ ανήκουν στο ζευγάρι: τα επεκτείνει όποιος από τους δύο θέλει, και της άλλης πλευράς δεν τα αγγίζεις.',
  'canasta.rules.layOffAfterOpening':
    'Ώσπου η πλευρά σου να κάνει το πρώτο της μελντ, δεν μπορεί να προσθέσει φύλλα σε τίποτα στο τραπέζι.',
  'canasta.rules.goOutKeepsACard':
    'Πρέπει πάντα να μπορείς να ολοκληρώσεις τον γύρο σου, οπότε ποτέ μη βγάζεις όλο το χέρι εκτός αν αυτή είναι η κίνηση που σε βγάζει έξω.',
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
  'holdem.rules.checkOrCall':
    'Πάσο μπορείς να πεις μόνο όταν δεν χρωστάς τίποτα· αλλιώς πληρώνεις, ανεβάζεις ή τα παρατάς.',
  'holdem.rules.minRaise': 'Μια αύξηση πρέπει να είναι τουλάχιστον όση και η προηγούμενη.',
  'holdem.rules.allIn':
    'Ποτέ δεν βάζεις πάνω από το στοίβαγμά σου, και το ολα μέσα επιτρέπεται πάντα — ακόμη κι όταν είναι λιγότερο από πλήρη αύξηση.',
  'holdem.rules.foldedOut': 'Μόλις τα παρατήσεις, είσαι εκτός ώσπου να μοιραστεί το επόμενο χέρι.',
  'holdem.remedy.callOrFold': 'Χρωστάς {n} — πλήρωσε, ανέβασε ή τα παρατάς.',
  'holdem.remedy.checkOrRaise': 'Δεν χρωστιέται τίποτα — πάσο ή αύξηση.',
  'holdem.remedy.callAllInOrFold':
    'Το στοίβαγμά σου δεν φτάνει πάνω από το ποντάρισμα — πλήρωσε {n} ολα μέσα ή τα παρατάς.',
  'holdem.remedy.raiseAtLeast': 'Ανέβασε τουλάχιστον στα {n}.',
  'holdem.remedy.raiseAtMost': 'Ανέβασε το πολύ στα {n} — αυτό είναι όλο το στοίβαγμά σου.',
  'holdem.remedy.nameAnAmount': 'Πες σε πόσα ανεβάζεις — ανάμεσα σε {min} και {max}.',
  'holdem.remedy.waitForNextHand': 'Είσαι εκτός αυτού του χεριού — περίμενε το επόμενο μοίρασμα.',

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
  'zolik.seat.contractMet': 'Το συμβόλαιο ολοκληρώθηκε',
  'results.you': '(εσύ)',



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
  'err.NOTHING_TO_SKIP': 'Δεν υπάρχει σειρά για παράλειψη',
  'err.NOTHING_TO_DRAW': 'Δεν έμεινε τίποτα να τραβήξεις',
  'err.PILE_EMPTY': 'Ο σωρός είναι άδειος',
  'err.PILE_BLOCKED': 'Ο σωρός είναι μπλοκαρισμένος — πάνω βρίσκεται ένα μαύρο τριάρι',
  'err.PILE_FROZEN': 'Ο σωρός είναι παγωμένος — χρειάζεσαι δύο φυσικά φύλλα της αξίας του πάνω φύλλου',
  'err.MELD_CAPTURE_NOT_ALLOWED': 'Σε αυτό το παιχνίδι ένας συνδυασμός στο τραπέζι δεν μπορεί να πάρει τον σωρό — χρειάζεσαι δύο φύλλα από το χέρι',
  'err.CAPTURE_NEEDS_TWO_CARDS': 'Το πάρσιμο του σωρού κοστίζει δύο φύλλα από το χέρι σου',
  'err.TOP_CARD_UNUSABLE': 'Η πλευρά σου δεν μπορεί να χρησιμοποιήσει το πάνω φύλλο',
  'err.MELD_CLOSED': 'Αυτός ο συνδυασμός είναι πλήρης και κλειστός',
  'err.MELD_TOO_SMALL': 'Ένας συνδυασμός χρειάζεται περισσότερα φύλλα από αυτά',
  'err.MELD_TOO_LARGE': 'Αυτός ο συνδυασμός δεν χωρά άλλα φύλλα',
  'err.MELD_MIXED_RANKS': 'Κάθε φύλλο σε έναν συνδυασμό πρέπει να έχει την ίδια αξία',
  'err.SEQUENCE_NO_WILDS': 'Μια σειρά δεν μπορεί να έχει μπαλαντέρ',
  'err.SEQUENCE_NEEDS_ONE_SUIT': 'Όλα τα φύλλα μιας σειράς πρέπει να είναι στο ίδιο χρώμα',
  'err.RUN_NOT_CONSECUTIVE': 'Η σειρά πρέπει να είναι συνεχόμενη, χωρίς κενά',
  'err.NOT_ENOUGH_NATURALS': 'Ένας συνδυασμός χρειάζεται περισσότερα φυσικά φύλλα από μπαλαντέρ',
  'err.RANK_ALREADY_MELDED': 'Η πλευρά σου έχει ήδη συνδυασμό αυτής της αξίας',
  'err.NOT_YOUR_MELD': 'Αυτός ο συνδυασμός ανήκει στην αντίπαλη πλευρά',
  'err.NO_SUCH_MELD': 'Αυτός ο συνδυασμός δεν είναι στο τραπέζι',
  'err.CANNOT_MELD_THREE': 'Τα τριάρια δεν κατεβαίνουν ποτέ',
  'err.BLACK_THREE_GO_OUT_ONLY':
    'Τα μαύρα τριάρια κατεβαίνουν μόνο ως η κίνηση που αδειάζει το χέρι σου',
  'err.CANNOT_DISCARD_RED_THREE': 'Το κόκκινο τριάρι δεν πετιέται',
  'err.MUST_KEEP_A_CARD': 'Κράτα τουλάχιστον ένα φύλλο — έτσι δεν μπορείς να αδειάσεις το χέρι σου',
  'err.MUST_MELD_FIRST': 'Κατέβασε πρώτα το άνοιγμα της πλευράς σου',
  'err.UNDO_MELDS_FIRST': 'Πρώτα αναίρεσε όσα κατέβασες μετά τη λήψη του σωρού',
  'err.INITIAL_MELD_NOT_MET': 'Στο πρώτο σου κατέβασμα λείπουν ακόμα πόντοι',
  'err.CANNOT_GO_OUT_YET': 'Η πλευρά σου χρειάζεται ολοκληρωμένη καναστα πριν μπορέσει να βγει',
  'err.NOTHING_TO_CALL': 'Δεν υπάρχει στοίχημα για πάσο',
  'err.CANNOT_CHECK': 'Δεν μπορείς να τσεκάρεις — υπάρχει στοίχημα να απαντήσεις',
  'err.CANNOT_RAISE': 'Δεν μπορείς να ανεβάσεις — το στοίβαγμά σου δεν φτάνει πάνω από το ποντάρισμα',
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
  'err.BAD_SEATING': 'Αυτή η σειρά θέσεων δεν ταιριάζει με όσους είναι στο τραπέζι',
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
  'canasta.offer.sequence': 'Σειρά',
  'canasta.remedy.drawOrTakePile': 'Πρώτα τράβα από την τράπουλα ή πάρε τον σωρό, μετά κατεβάζεις.',
  'canasta.remedy.meldOrDiscard':
    'Έχεις ήδη τραβήξει — κατέβασε μελντ ή ρίξε ένα φύλλο για να κλείσεις τον γύρο.',
  'canasta.remedy.drawFromStock': "Τράβα από την τράπουλα αντί γι' αυτό.",
  'canasta.remedy.takePileInstead': "Η τράπουλα τελείωσε — πάρε τον σωρό αντί γι' αυτό.",
  'canasta.remedy.pileBlocked': 'Τράβα από την τράπουλα — το μαύρο τρία στην κορυφή κρατά τον σωρό κλειστό.',
  'canasta.remedy.pileFrozen':
    'Τράβα από την τράπουλα ή πάρε τον σωρό με δύο φυσικά φύλλα από το χέρι σου που ταιριάζουν με το {card}.',
  'canasta.remedy.topCardUnusable':
    'Τράβα από την τράπουλα — η πλευρά σου δεν έχει τι να κάνει με το {card} στην κορυφή.',
  'canasta.remedy.captureFromHand':
    'Πάρε τον σωρό με δύο φύλλα από το δικό σου χέρι που ταιριάζουν με το {card}.',
  'canasta.remedy.needTwoMatching':
    'Χρειάζεσαι δύο φύλλα από το χέρι που ταιριάζουν με το {card} — αλλιώς τράβα από την τράπουλα.',
  'canasta.remedy.needMorePoints': 'Στο πρώτο μελντ της πλευράς σου λείπουν {n} πόντοι για τους {floor}.',
  'canasta.remedy.openFirst': 'Κατέβασε πρώτα το πρώτο μελντ της πλευράς σου, μετά προσθέτεις.',
  'canasta.remedy.needCanastas': 'Η πλευρά σου χρειάζεται ακόμη {n} καναστες των {size} φύλλων για να βγει.',
  'canasta.remedy.keepACard': 'Κράτα ένα φύλλο για να το ρίξεις.',
  'canasta.remedy.undoMeldsFirst':
    'Αναίρεσε πρώτα τους συνδυασμούς που κατέβασες μετά τη λήψη του σωρού — μετά μπορεί να γυρίσει πίσω και ο ίδιος ο σωρός.',
  'canasta.remedy.layOffInstead': 'Πρόσθεσέ τα στο μελντ που έχει ήδη η πλευρά σου.',
  'canasta.remedy.meldClosed': 'Αυτό το μελντ ολοκληρώθηκε στα {n} φύλλα — ξεκίνα άλλο ή πρόσθεσε αλλού.',
  'canasta.remedy.discardNotARedThree': 'Ρίξε κάτι άλλο, όχι κόκκινο τρία.',
  'canasta.remedy.blackThreesOnTheWayOut':
    'Τα μαύρα τρία κατεβαίνουν μόνο με την κίνηση που αδειάζει το χέρι σου.',
  'canasta.remedy.ownMeldsOnly': 'Πρόσθετε μόνο στα μελντ της δικής σου πλευράς.',
  'badge.naturalCanasta': 'Καθαρή κανάστα',
  'badge.mixedCanasta': 'Μικτή κανάστα',
  'badge.samba': 'Σάμπα',
  'badge.cleanRun': 'Καθαρή σειρά',
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
  'holdem.quick.halfPot': '½ Πότ',
  'holdem.quick.pot': 'Πότ',
  'holdem.quick.allIn': 'All-in',
  'zone.yourHand': 'Το χέρι σου',
  'zone.opponentHand': 'Το χέρι του',
  'zone.drawPile': 'Τράπουλα',
  'zone.discardPile': 'Σωρός απόρριψης',
  'zone.melds': 'Συνδυασμοί',
  'zone.teamMelds': 'Συνδυασμοί της πλευράς σου',
  'zone.opponentMelds': 'Συνδυασμοί του αντιπάλου',
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
  // The short word for a link or a tab.
  'legal.terms': 'Όροι',
  // The document's own name, used as the heading above it. Kept apart from
  // `legal.notice.terms`, which is the same words inflected to sit inside a
  // sentence — a heading in the instrumental case reads as a mistake in every
  // Slavic language.
  'legal.terms.title': 'Όροι χρήσης',
  'legal.privacy.title': 'Ενημέρωση απορρήτου',
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
  'ginrummy.rules.upcardDance':
    'Πριν από το πρώτο τράβηγμα, το ανοιχτό φύλλο μπορεί να το πάρει πρώτα ο μη μοιράζων και μετά ο μοιράζων· αν το αρνηθούν κι οι δύο, ο μη μοιράζων τραβά από την τράπουλα.',
  'ginrummy.rules.knockOnDiscard':
    'Το χτύπημα αντικαθιστά το φύλλο που ρίχνεις, άρα γίνεται μόνο στο τέλος του γύρου σου.',
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
  'ginrummy.remedy.takeOrPassUpcard': 'Πάρε το ανοιχτό φύλλο ή πέρνα.',
  'ginrummy.remedy.drawFirst': 'Πρώτα τράβα ένα φύλλο — από την τράπουλα ή από τον σωρό.',
  'ginrummy.remedy.discardToEndTurn': 'Ρίξε ένα φύλλο για να κλείσεις τον γύρο σου.',
  'ginrummy.remedy.finishLayoff': 'Τίποτα άλλο δικό σου δεν ταιριάζει — τελείωσε το κατέβασμα.',
  'ginrummy.remedy.stockDrawForced': 'Το φύλλο αυτό το προσπεράσατε κι οι δύο — τράβα από την τράπουλα.',
  'ginrummy.remedy.drawElsewhere': 'Αυτός ο σωρός είναι άδειος — τράβα από τον άλλο.',
  'ginrummy.remedy.getDeadwoodDown':
    'Μπορείς να χτυπήσεις μόλις το νεκρό σου ξύλο πέσει στους {n} πόντους ή λιγότερο.',
  'ginrummy.zone.knockerHand': 'Το χέρι που χτύπησε',
  'ginrummy.zone.melds': 'Συνδυασμοί',
  'ginrummy.prompt.upcardDecision': 'Πάρε το ανοιχτό φύλλο, ή πες πάσο',
  'ginrummy.prompt.yourTurnDraw': 'Τράβα ένα φύλλο',
  'ginrummy.prompt.yourTurnDiscard': 'Πέταξε — ή χτύπα, αν μπορείς',
  'ginrummy.prompt.layoff': 'Προσάρτησε ντεντγουντ, ή τελείωσε',

  // --- rummy tiles -------------------------------------------------------------
  'err.TILE_NOT_IN_HAND': 'Αυτό το πλακίδιο δεν είναι στο χέρι σου',
  'err.TILE_DOES_NOT_FIT': "Αυτό το πλακάκι δεν ταιριάζει σ' αυτό το σετ",
  'err.NO_SUCH_SET': 'Αυτός ο συνδυασμός δεν είναι στο τραπέζι',
  'err.INITIAL_MELD_ONLY':
    'Πριν από το πρώτο σου κατέβασμα μπορείς να αναδιατάξεις μόνο τους δικούς σου νέους συνδυασμούς',
  'err.TABLE_NOT_VALID': 'Κάποιο σετ στο τραπέζι δεν είναι έγκυρη ομάδα ούτε σειρά',
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
  'rummytiles.remedy.emptyTheTray':
    'Τοποθέτησε τα {n} πλακάκια που είναι ακόμη στον δίσκο ή μηδένισε τον γύρο.',
  'rummytiles.remedy.playFromHandOrDraw':
    'Η αναδιάταξη μόνη της δεν είναι γύρος — βάλε τουλάχιστον ένα πλακάκι από το χέρι σου ή τράβα.',
  'rummytiles.remedy.fixOrReset':
    'Κάθε σετ στο τραπέζι πρέπει να είναι έγκυρη ομάδα ή σειρά — διόρθωσέ τα ή μηδένισε τον γύρο.',
  'rummytiles.remedy.needMorePoints': 'Στο πρώτο σου κατέβασμα λείπουν {n} πόντοι για τους {floor}.',
  'rummytiles.remedy.ownNewSetsOnly':
    "Ώσπου να κατεβάσεις τους πρώτους σου {n} πόντους, μπορείς να αναδιατάξεις μόνο τα σετ που έφτιαξες σ' αυτόν τον γύρο.",
  'rummytiles.remedy.startANewSet': "Βάλ' το σε νέο σετ αντί γι' αυτό.",
  'rummytiles.remedy.splitLeavesThree':
    'Χώρισε τη σειρά έτσι ώστε και τα δύο κομμάτια να κρατήσουν τουλάχιστον {n} πλακάκια.',
  'rummytiles.remedy.matchTheJoker': 'Αντάλλαξε τον μπαλαντέρ με ακριβώς το πλακάκι που αντιπροσωπεύει.',
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
  'blackjack.rules.roundOrder':
    'Ο γύρος έχει σειρά: στοιχήματα, μοίρασμα, ασφάλεια αν ο ντίλερ δείχνει άσο, και μετά κάθε θέση παίζει το χέρι της με τη σειρά.',
  'blackjack.rules.oneStakePerRound': 'Ένα στοίχημα ανά γύρο — μόλις μπει, δεν αλλάζει.',
  'blackjack.rules.stakeFromStack': 'Μπορείς να ποντάρεις μόνο μάρκες που πραγματικά έχεις.',
  'blackjack.remedy.putAStakeUp': 'Πρώτα ποντάρισε — {n} ή περισσότερα.',
  'blackjack.remedy.answerInsurance': 'Πρώτα πες ναι ή όχι στην ασφάλεια.',
  'blackjack.remedy.playThisHand': 'Παίξε το χέρι μπροστά σου — τράβα ή μείνε.',
  'blackjack.remedy.stakeIsUp': 'Το στοίχημά σου είναι ήδη μέσα — περίμενε το μοίρασμα.',
  'blackjack.remedy.stakeAtLeast': 'Ποντάρισε τουλάχιστον {n}.',
  'blackjack.remedy.stakeAtMost': 'Ποντάρισε το πολύ {n} — αυτά έχεις όλα κι όλα.',
  'blackjack.remedy.sayHowMuch': 'Πες πόσα ποντάρεις — {n} ή περισσότερα.',
  'blackjack.remedy.hitOrStand': 'Τράβα ή μείνε.',
  'blackjack.remedy.waitForNextRound': 'Είσαι εκτός αυτού του γύρου — περίμενε το επόμενο μοίρασμα.',

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

  'blackjack.quick.doubleMin': '2× Ελάχιστο',
  'blackjack.quick.allIn': 'All-in',
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
  'settings.signedInAs': 'Συνδέθηκες ως {username}',
  'settings.playingAsGuest': 'Παίζεις ως {username} (επισκέπτης)',
  'settings.notSignedIn': 'Δεν έχεις συνδεθεί — συνδέσου ή συνέχισε ως επισκέπτης για να παίξεις online.',
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
  'nav.home': 'Jokerless',
  'nav.emailSignIn': 'Σύνδεση με email',
  'nav.signingIn': 'Σύνδεση σε εξέλιξη',
  'nav.usernameSignIn': 'Σύνδεση με όνομα χρήστη',
  'nav.legacyAccount': 'Παλιός λογαριασμός',
  'nav.guest': 'Επισκέπτης',
  'nav.account': 'Λογαριασμός',
  'nav.games': 'Παιχνίδια',
  'nav.table': 'Το τραπέζι σου',
  'nav.join': 'Μπες σε τραπέζι',
  // The screen a shared invite link lands on while it resolves the code.
  'nav.joining': 'Σύνδεση σε τραπέζι',
  'nav.rules': 'Κανόνες',
  'nav.match': 'Αγώνας',
  'nav.scoreTable': 'Πίνακας σκορ',
  'nav.stats': 'Στατιστικά',
  'nav.more': 'Περισσότερα',
  'nav.myGames': 'Οι παρτίδες μου',
  'nav.about': 'Σχετικά',

  // --- the account menu behind the face in the corner ----------------------
  'menu.label': 'Μενού λογαριασμού',
  'menu.signedIn': 'Συνδεδεμένος',
  'menu.notSignedIn': 'Μη συνδεδεμένος',
  'menu.keepStats': 'για να κρατήσεις τα στατιστικά σου',
  'menu.signOut': 'Αποσύνδεση',
  'more.scoreTable': 'Πίνακας σκορ εκτός σύνδεσης',
  'more.stats': 'Στατιστικά και κατάταξη',
  'more.needsAccount': 'συνδέσου για χρήση',
  'gate.title': 'Συνδέσου για να το χρησιμοποιήσεις',
  'gate.body':
    'Οι πίνακες σκορ και τα στατιστικά φυλάσσονται με τον λογαριασμό σου, ώστε να σε ακολουθούν σε άλλη συσκευή. Ο επισκέπτης δεν έχει πού να τα φυλάξει.',

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
  'error.generic': 'Αυτό δεν πέτυχε',
  'error.signIn': 'Η σύνδεση απέτυχε',
  'error.login': 'Η σύνδεση απέτυχε',
  'error.register': 'Η εγγραφή απέτυχε',
  'error.sendCode': 'Δεν ήταν δυνατή η αποστολή κωδικού',
  'error.badCode': 'Αυτός ο κωδικός δεν λειτούργησε',
  'error.rulesLoad': 'Δεν ήταν δυνατή η φόρτωση των κανόνων',
  'error.createFailed': 'Η δημιουργία απέτυχε',
  'error.saveFailed': 'Η αποθήκευση απέτυχε',
  'error.exportFailed': 'Η εξαγωγή απέτυχε',

  // --- the route that does not exist ----------------------------------------
  'notFound.title': 'Ωχ!',
  'notFound.message': 'Αυτή η οθόνη δεν υπάρχει.',
  'notFound.home': 'Πήγαινε στην αρχική οθόνη!',

  // --- signing in -----------------------------------------------------------
  'auth.login.subtitle': 'Κράτα τα στατιστικά σου σε όλες τις συσκευές',
  'auth.login.continueWithEmail': 'Συνέχεια με email',
  'auth.login.usernameInstead': "Σύνδεση με όνομα χρήστη αντ' αυτού",
  'auth.email.title': 'Σύνδεση με email',
  'auth.email.subtitle': 'Θα σου στείλουμε έναν κωδικό μιας χρήσης',
  'auth.email.address': 'Διεύθυνση email',
  'auth.email.send': 'Αποστολή κωδικού',
  'auth.email.codeTitle': 'Βάλε τον κωδικό',
  'auth.email.codePlaceholder': 'Εξαψήφιος κωδικός',
  'auth.email.differentAddress': 'Χρήση άλλης διεύθυνσης',
  'auth.email.sentTo': 'Στάλθηκε στο {email}',
  'auth.email.continue': 'Συνέχεια',
  'auth.guest.title': 'Παιχνίδι ως επισκέπτης',
  'auth.guest.subtitle': 'Δεν χρειάζεται λογαριασμός',
  'auth.guest.displayName': 'Εμφανιζόμενο όνομα',
  'auth.register.title': 'Δημιουργία λογαριασμού',
  'auth.register.username': 'Όνομα χρήστη',
  'auth.register.email': 'Email (προαιρετικό)',
  'auth.register.password': 'Κωδικός πρόσβασης',
  'auth.username.createAccount': 'Δημιούργησε λογαριασμό με όνομα χρήστη και κωδικό',
  'auth.callback.signedIn': 'Συνδέθηκες.',

  // --- the account screen ---------------------------------------------------
  'account.signInPrompt': 'Συνδέσου για να διαχειριστείς τον λογαριασμό σου.',
  'account.keepGames': 'Κράτα αυτά τα παιχνίδια',
  'account.signedInWith': 'Συνδεδεμένος μέσω',
  'account.addMethod': 'Πρόσθεσε τρόπο σύνδεσης',
  'account.usernameAndPassword': 'Όνομα χρήστη και κωδικός',
  'account.faceAndTable': 'Πρόσωπο και όψη τραπεζιού',
  'account.refresh': 'Ανανέωση',
  'account.remove': 'Αφαίρεση',

  // --- the main menu --------------------------------------------------------
  'home.subtitle': 'Κοντινένταλ ρέμι · {server}',
  'home.playingAs': 'Παίζεις ως {name}',
  'home.signInPrompt': 'Συνδέσου ή συνέχισε ως επισκέπτης για να παίξεις online.',
  'home.statsAndLeaderboard': 'Στατιστικά και κατάταξη',
  'home.play': 'Παίξε',
  'home.offlineScoreTable': 'Πίνακας σκορ εκτός σύνδεσης',
  'home.signInToKeepStats': 'Συνδέσου για να κρατήσεις τα στατιστικά σου',
  'home.signOut': 'Αποσύνδεση',
  'home.continueAsGuest': 'Συνέχεια ως επισκέπτης',
  // Someone playing as a guest, marked in a list of names.
  'home.guestSuffix': '(επισκέπτης)',

  // --- the waiting room on the main menu ------------------------------------
  //
  // Counts are whole phrases per count, not a number glued to a noun — the
  // same reason `countLabel` exists. One and many are the cases the interface
  // actually produces, so they are the cases that get their own wording.
  'waiting.checking': 'Βλέπουμε ποιος είναι εδώ…',
  'waiting.youAreWaiting': 'Περιμένεις να παίξεις',
  'waiting.pickedUp': 'Όποιος ανοίξει τραπέζι μπορεί να σε πάρει — κανείς δεν χρειάζεται κωδικό από σένα.',
  'waiting.othersOne': 'Περιμένει και 1 ακόμα παίκτης',
  'waiting.othersMany': 'Περιμένουν και {n} ακόμα παίκτες',
  'waiting.oneWaiting': '1 παίκτης περιμένει να παίξει',
  'waiting.manyWaiting': '{n} παίκτες περιμένουν να παίξουν',
  'waiting.adding': 'Σε προσθέτουμε στη λίστα αναμονής…',
  'waiting.slowHint':
    'Αν αυτό δεν τελειώσει σε λίγα δευτερόλεπτα, έλεγξε αν η διεύθυνση του διακομιστή παρακάτω είναι προσβάσιμη από αυτή τη συσκευή.',
  'waiting.serverBusyDetail':
    'Προσπάθεια {n}. Ο διακομιστής δεν δέχεται αυτή τη στιγμή νέες συνδέσεις στην αίθουσα αναμονής.',
  'waiting.reconnecting': 'Χάθηκε η σύνδεση — επανασύνδεση…',
  'waiting.reconnectingDetail':
    'Προσπάθεια {n}. Μπορεί να συμβεί αν άλλαξε το δίκτυο της συσκευής σου ή αν ο διακομιστής επανεκκινήθηκε.',
  'waiting.tryAgain': 'Δοκίμασε ξανά τώρα',
  'waiting.makeAvailable': 'Κάνε με διαθέσιμο για παιχνίδι',
  'waiting.stop': 'Σταμάτα να περιμένεις',
  'waiting.noneYet':
    'Αυτή τη στιγμή δεν περιμένει κανείς να παίξει. Μπες στη λίστα και θα είσαι ο πρώτος που θα δει ο καθένας.',
  'waiting.noOthersYet':
    'Δεν περιμένει κανείς άλλος ακόμα. Οι οικοδεσπότες σε βλέπουν έτσι κι αλλιώς και μπορούν να σε καλέσουν.',
  'waiting.server': 'Διακομιστής',
  'waiting.none':
    'Αυτή τη στιγμή δεν περιμένει κανείς. Όποιος δηλώσει διαθέσιμος στο κύριο μενού εμφανίζεται εδώ.',

  // --- landing on a shared invite link --------------------------------------
  'join.missingCode': 'Σε αυτόν τον σύνδεσμο λείπει ο κωδικός του τραπεζιού.',
  'join.staleLink': 'Ζήτα φρέσκο σύνδεσμο από όποιον σε κάλεσε, ή μπες με τον κωδικό.',
  'join.enterCode': 'Βάλε κωδικό',
  'join.backToMenu': 'Πίσω στο μενού',
  'join.takingSeat': 'Παίρνουμε θέση…',
  'join.takingSeatAt': 'Παίρνουμε θέση στο {game}…',

  // --- the lobby ------------------------------------------------------------
  'lobby.games.subtitle': 'Ό,τι μπορεί να φιλοξενήσει αυτός ο διακομιστής',
  'lobby.games.bots': 'Μποτ',
  'lobby.games.setup': 'Ρυθμίσεις',
  'lobby.games.playBot': 'Παίξε εναντίον ενός μποτ',
  'lobby.games.playBots': 'Παίξε εναντίον {n} μποτ',
  'lobby.games.openTable': 'Άνοιξε τραπέζι',
  'lobby.games.players': '{n} παίκτες',
  'lobby.games.playerRange': '{min}–{max} παίκτες',
  'lobby.join.placeholder': 'Κωδικός ή σύνδεσμος πρόσκλησης',
  'lobby.join.needCode': 'Δώσε κωδικό, σύνδεσμο ή ταυτότητα αγώνα',
  'lobby.games.signInFirst': 'Συνδέσου πρώτα',
  'lobby.join.action': 'Μπες',
  'lobby.join.waitingTitle': 'Αναμονή για τον οικοδεσπότη',
  // Two whole sentences rather than one with a swapped noun: "a game of {game}"
  // does not survive a language that inflects the game's name after "of".
  'lobby.join.joinedGame': 'Μπήκες σε παιχνίδι {game} — αναμονή για την έναρξη',
  'lobby.join.joinedTable': 'Μπήκες στο τραπέζι — αναμονή για την έναρξη',
  'lobby.table.addBot': 'Πρόσθεσε μποτ',
  'lobby.table.side': 'Πλευρά {n}',
  'lobby.table.shuffleSeats': 'Ανακάτεψε τις θέσεις',
  'lobby.table.moveSeatUp': 'Μετακίνησε τον {name} μία θέση πάνω',
  'lobby.table.moveSeatDown': 'Μετακίνησε τον {name} μία θέση κάτω',
  'lobby.table.start': 'Ξεκίνα',
  'lobby.table.waitingForHost': 'Αναμονή να ξεκινήσει ο οικοδεσπότης…',

  // --- "my games": stored tables, resumed or deleted --------------------
  'mine.subtitle': 'Παρτίδες που μπορείς να συνεχίσεις ή να διαγράψεις',
  'mine.tabUnfinished': 'Σε εξέλιξη',
  'mine.tabFinished': 'Ολοκληρωμένες',
  'mine.emptyUnfinished': 'Καμία παρτίδα σε εξέλιξη αυτή τη στιγμή',
  'mine.emptyFinished': 'Καμία ολοκληρωμένη παρτίδα ακόμα',
  'mine.open': 'Άνοιγμα',
  'mine.resume': 'Συνέχεια',
  'mine.delete': 'Διαγραφή',
  'mine.deleteConfirmTitle': 'Διαγραφή αυτού του τραπεζιού;',
  'mine.deleteConfirmBody': 'Η παρτίδα τελειώνει για όλους τους υπόλοιπους παίκτες. Δεν αναιρείται.',
  'mine.deleteConfirm': 'Διαγραφή',
  'mine.deleteCancel': 'Άκυρο',
  'mine.viewAll': 'Δες όλες',
  'mine.status.lobby': 'Περιμένει να ξεκινήσει',
  'mine.status.active': 'Σε εξέλιξη',
  'mine.status.suspended': 'Σε παύση',
  'mine.status.abandoned': 'Στην άκρη',
  'mine.status.completed': 'Ολοκληρώθηκε',

  // --- inviting someone to a table ------------------------------------------
  'invite.heading': 'Κάλεσε παίκτες',
  'invite.explain':
    'Στείλε αυτόν τον σύνδεσμο. Όποιος τον ανοίξει προσγειώνεται σε αυτό το τραπέζι — χωρίς λογαριασμό.',
  'invite.noAddress':
    'Σε αυτόν τον διακομιστή δεν έχει ρυθμιστεί κοινοποιήσιμη διεύθυνση, οπότε χρησιμοποίησε τον κωδικό παρακάτω.',
  'invite.readOutCode': 'Ή υπαγόρευσε τον κωδικό:',
  'invite.copy': 'Αντιγραφή συνδέσμου',
  'invite.share': 'Κοινοποίηση συνδέσμου',
  'invite.copied': 'Αντιγράφηκε!',
  'invite.shared': 'Κοινοποιήθηκε',

  // --- the match screen -----------------------------------------------------
  'match.waitingForTable': 'Αναμονή για το τραπέζι…',
  'match.waitingForPlayer': 'Αναμονή για άλλον παίκτη…',
  'match.nobodyWon': 'Δεν κέρδισε κανείς.',
  'match.youWon': 'Κέρδισες.',
  'match.finished': 'Αυτός ο αγώνας τελείωσε.',
  'match.inProgress': 'Ο αγώνας είναι σε εξέλιξη — όλα είναι συνδεδεμένα και κυλούν κανονικά.',
  'match.connecting': 'Σύνδεση…',
  'match.abandonedTitle': 'Το τραπέζι μπήκε στην άκρη',
  'match.abandoned': 'Κανείς δεν επέστρεψε σε αυτό το τραπέζι, οπότε μπήκε στην άκρη. Τα φύλλα είναι ακριβώς εκεί που τα άφησες.',
  'match.resume': 'Συνέχισε από εκεί που έμεινες',
  'match.resuming': 'Επαναφορά τραπεζιού…',
  'match.abandonedWaitingFor': 'Αναμένεται να επιστρέψει στο τραπέζι: {names}.',
  'match.controls': 'Χειριστήρια',
  'match.over': 'Ο αγώνας τελείωσε',
  'match.settingUp': 'Ετοιμάζουμε…',
  'match.playAgain': 'Παίξε ξανά',
  'match.backToGames': 'Πίσω στα παιχνίδια',
  'match.table': 'Τραπέζι',
  'match.opponents': 'Αντίπαλοι',
  // Marks which seat is the reader's own, in a list of seats.
  'match.youSuffix': '(εσύ)',
  // The winner line is composed from whole sentences rather than a name glued
  // to " won." — the verb agrees with the subject in most of these languages,
  // and a suffix cannot know that.
  'match.you': 'εσύ',
  'match.yourTeam': 'Η ομάδα σου',
  'match.teammates': 'Ομάδα: {names}',
  'match.someoneWon': 'Ο {name} κέρδισε.',
  'match.wonBy': 'Κερδισμένο από {names}.',
  'match.pausedFor': 'Σε παύση — αναμονή να επανασυνδεθεί ο {name}.',
  'match.results': 'Αποτελέσματα',
  'match.players': 'Παίκτες',
  'match.toPlay': 'στη σειρά',

  // --- the offline score table ----------------------------------------------
  'scoring.namesHint': 'Ονόματα χωρισμένα με κόμμα (4–8 παίκτες)',
  'scoring.newSession': 'Νέα συνεδρία',
  // A worked example, not a sentence: the names are placeholders a translator
  // may localise, the shape `name:score,` is what the parser needs.
  'scoring.scoresPlaceholder': 'Άννα:120,Βασίλης:80,…',
  'scoring.saveRound': 'Αποθήκευση γύρου',
  'scoring.export': 'Εξαγωγή φύλλου σκορ',
  'scoring.formatHint': 'Μορφή σκορ: Όνομα:100,Όνομα2:50',
  'scoring.nameCountError': 'Δώσε 2–8 ονόματα παικτών χωρισμένα με κόμμα',
  'scoring.session': 'Συνεδρία: {id}',
  'scoring.players': 'Παίκτες: {names}',
  'scoring.roundScores': 'Σκορ του γύρου {n}',
  'stats.loading': 'Φόρτωση…',
  // The figures could not be fetched. `{reason}` is whatever the server or the
  // network said, which is not ours to translate — the frame around it is.
  'stats.unavailable': '(μη διαθέσιμο: {reason})',

  // --- statistics -----------------------------------------------------------
  'stats.title': 'Στατιστικά και κατάταξη',
  'stats.yours': 'Τα στατιστικά σου',
  'stats.leaderboard': 'Κατάταξη',

  // --- a player's lifetime record, shown beside a finished match ------------
  'record.title': 'Το ιστορικό σου',
  'record.guest':
    'Παίζεις ως επισκέπτης, οπότε δεν κρατιέται ιστορικό. Συνδέσου και τα παιχνίδια που έχεις ήδη παίξει σε αυτή τη συσκευή — μαζί με αυτό — θα μείνουν στον λογαριασμό σου.',
  'record.signInToKeep': 'Συνδέσου και κράτα τα',
  'record.failed': 'Το ιστορικό σου δεν φορτώθηκε αυτή τη στιγμή. Ο αγώνας έχει καταγραφεί με ασφάλεια.',
  'record.loading': 'Φόρτωση…',
  'record.played': 'Παιγμένα',
  'record.won': 'Κερδισμένα',
  'record.lost': 'Χαμένα',
  'record.winRate': 'Ποσοστό νικών',
  'record.streak': 'Σερί',
  'record.atThisGame': 'Σε αυτό το παιχνίδι',
  // A streak is a count with a noun, so it is a whole phrase per count for
  // the same reason `countLabel` is: "1 win" / "2 wins" is an English rule,
  // and most of these languages do not share it.
  'record.streakNone': '—',
  'record.streakWinOne': '1 νίκη',
  'record.streakWinMany': '{n} νίκες',
  'record.streakLossOne': '1 ήττα',
  'record.streakLossMany': '{n} ήττες',

  // --- moving cards ---------------------------------------------------------
  'hand.dragHint':
    'Σύρε ένα φύλλο κατά μήκος της βεντάλιας για να το αναδιατάξεις, ή στο τραπέζι για να το παίξεις',
  'hand.moveLeft': 'Αριστερά',
  'hand.moveRight': 'Δεξιά',
  'hand.openFan': 'Άνοιγμα των φύλλων',
  'hand.closeFan': 'Κλείσιμο των φύλλων',
  'zone.collapseGroup': 'Σύμπτυξη αυτής της ομάδας',
  'zone.expandGroup': 'Εμφάνιση όλων των φύλλων αυτής της ομάδας',
  'zone.dropHere': 'Άφησέ το εδώ',
  'offer.pickCards': 'διάλεξε φύλλα για το σημείο που άγγιξες',
  'offer.ambiguous': 'αυτό μπορεί να πάει σε περισσότερα από ένα σημεία — διάλεξε στο τραπέζι',

  // --- the build footer -----------------------------------------------------
  'build.app': 'εφαρμογή',
  'build.server': 'διακομιστής',
  'about.subtitle': 'Η έκδοση που παίζετε και τα ψιλά γράμματα.',
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
  'option.pauseBetweenRounds': 'Παύση ανάμεσα στους γύρους',
  'choice.pauseBetweenRounds.1': 'Παύση',
  'choice.pauseBetweenRounds.0': 'Συνέχισε κατευθείαν',
  'option.openDiscardPile': 'Σωρός απόρριψης',
  'choice.openDiscardPile.1': 'Ανοιχτός για έλεγχο',
  'choice.openDiscardPile.0': 'Μόνο το πάνω φύλλο',
  'option.botSkill': 'Αντίπαλοι',
  'choice.botSkill.0': 'Ανάμεικτοι',
  'choice.botSkill.1': 'Εύκολοι',
  'choice.botSkill.2': 'Μέτριοι',
  'choice.botSkill.3': 'Δύσκολοι',
  'option.initialMeldMinimum': 'Αξία ανοίγματος',
  'choice.initialMeldMinimum.0': 'Καμία',
  'option.discardDrawMinRound': 'Τράβηγμα από τον σωρό',
  'choice.discardDrawMinRound.0': 'Ανοιχτό',
  'choice.discardDrawMinRound.2': 'Από τον γύρο 2',
  'choice.discardDrawMinRound.3': 'Από τον γύρο 3',
  'option.requireCleanRun': 'Κέντα χωρίς μπαλαντέρ',
  'choice.requireCleanRun.1': 'Απαιτείται',
  'choice.requireCleanRun.0': 'Όχι',
  'option.jokerReclaimMustPlay': 'Εξαγορασμένος μπαλαντέρ',
  'choice.jokerReclaimMustPlay.1': 'Παίζεται τον ίδιο γύρο',
  'choice.jokerReclaimMustPlay.0': 'Μπορεί να κρατηθεί',
  'option.dealStarter': 'Ποιος ξεκινά',
  'choice.dealStarter.0': 'Εκ περιτροπής',
  'choice.dealStarter.1': 'Ξεκινά ο νικητής',
  'variation.prsi.classic': 'Κλασικό',
  'option.handSize': 'Φύλλα που μοιράζονται',
  'variation.canasta.classic': 'Κλασική',
  'variation.canasta.modern_american': 'Modern American',
  'variation.canasta.samba': 'Σάμπα',
  'option.targetScore': 'Σκορ στόχος',
  'option.canastasToGoOut': 'Καναστες για έξοδο',
  'variation.holdem.freezeout': 'Freezeout',
  'variation.holdem.timed': 'Σταθερός αριθμός μοιρασιών',
  'option.startingStack': 'Αρχικές μάρκες',
  'option.bigBlind': 'Μεγάλο τυφλό',
  'option.handLimit': 'Μοιρασιές',
  'choice.handLimit.0': 'Μέχρι να μείνει μία θέση',
  'variation.ginrummy.standard': 'Κανονικό',
  'option.knockLimit': 'Όριο χτυπήματος',
  'choice.knockLimit.0': 'Οκλαχόμα (το ορίζει το ανοιχτό φύλλο)',
  'option.bigGin': 'Μεγάλο τζιν',
  'choice.bigGin.0': 'Ανενεργό',
  'choice.bigGin.1': 'Ενεργό (+25)',
  'option.lineBonuses': 'Μπόνους στον απολογισμό',
  'choice.lineBonuses.1': 'Ενεργά',
  'choice.lineBonuses.0': 'Ανενεργά',
  'variation.rummytiles.standard': 'Κανονικό',
  'choice.targetScore.0': 'Κανένα',
  // Two games qualify a number the others mean plainly: Canasta's 500 is a
  // short game, Gin Rummy's and Rummy Tiles' 500 is just 500; Hold'em's
  // starting 200 is a short stack, Blackjack's 200 is just 200. Module-scoped
  // so the qualifier travels with the game that means it — a bare key would
  // hand the note to games it is false for. `scripts/check-server-labels.js`
  // is what finds these; both were shipping as English on a Czech screen.
  'choice.canasta.targetScore.500': '500 (σύντομη)',
  'choice.holdem.startingStack.200': '200 (σύντομη)',
  'option.roundLimit': 'Όριο γύρων',
  'choice.roundLimit.0': 'Κανένα',
  'option.poolExhaustion': 'Αν στερέψει το απόθεμα',
  'choice.poolExhaustion.1': 'Τον γύρο κερδίζει το χαμηλότερο χέρι',
  'choice.poolExhaustion.0': 'Τον γύρο δεν τον κερδίζει κανείς',
  'variation.blackjack.single': 'Μία τράπουλα',
  'option.minBet': 'Ελάχιστο τραπεζιού',
  'option.rounds': 'Γύροι',
  'option.decks': 'Τράπουλες',
  'option.dealerHitsSoft17': 'Ο ντίλερ στο μαλακό 17',
  'choice.dealerHitsSoft17.0': 'Μένει',
  'choice.dealerHitsSoft17.1': 'Τραβά',
  'option.blackjackPays': 'Το μπλακ τζακ πληρώνει',
  'choice.blackjackPays.100': 'Ένα προς ένα',
  'option.maxSplits': 'Χώρισμα',
  'choice.maxSplits.0': 'Χωρίς χώρισμα',
  'choice.maxSplits.1': 'Μία φορά (δύο χέρια)',
  'choice.maxSplits.3': 'Τρεις φορές (τέσσερα χέρια)',
  'option.doubleAfterSplit': 'Διπλασιασμός μετά το χώρισμα',
  'choice.doubleAfterSplit.1': 'Επιτρέπεται',
  'choice.doubleAfterSplit.0': 'Δεν επιτρέπεται',
  'option.surrender': 'Παράδοση',
  'choice.surrender.0': 'Ανενεργή',
  'choice.surrender.1': 'Όψιμη παράδοση',
  'option.insurance': 'Ασφάλεια',
  'choice.insurance.1': 'Προσφέρεται',
  'choice.insurance.0': 'Δεν προσφέρεται',

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
  'verb.add': 'Πρόσθεσε',
  'verb.bet': 'Ποντάρισε',
  'verb.call': 'Πάσο',
  'verb.check': 'Τσεκ',
  'verb.commit': 'Έτοιμο',
  'verb.continue': 'Συνέχεια',
  'verb.decline_insurance': 'Χωρίς ασφάλεια',
  'verb.discard': 'Πέταξε',
  'verb.double': 'Διπλασίασε',
  'verb.draw': 'Τράβα',
  'verb.finish_layoff': 'Τέλος προσάρτησης',
  'verb.fold': 'Πάσο (φολ)',
  'verb.hit': 'Φύλλο',
  'verb.insure': 'Πάρε ασφάλεια',
  'verb.knock': 'Χτύπα',
  'verb.lay_meld': 'Κατέβασε',
  'verb.lay_off': 'Προσάρτησε',
  'verb.pass': 'Πάσο',
  'verb.place': 'Τοποθέτησε',
  'verb.play_card': 'Παίξε',
  'verb.raise': 'Ανέβασε',
  'verb.reset_turn': 'Μηδένισε τη σειρά',
  'verb.split': 'Χώρισε',
  'verb.stand': 'Μένω',
  'verb.surrender': 'Παράδοση',
  'verb.swap_joker': 'Άλλαξε τον μπαλαντέρ',
  'verb.take': 'Πάρε',
  'verb.take_pile': 'Πάρε από τον σωρό',
  // Declared by the server and listed in `serverKeys.json`, but never worded.
  'verb.takePileFromHand': 'Πάρε τον σωρό στο χέρι',
  'verb.takePileOntoMeld': 'Πάρε τον σωρό σε συνδυασμό',
  'verb.takeTopForSequence': 'Πάρε το πάνω φύλλο σε μια σειρά',
  'verb.undoDraw': 'Ακύρωση τραβήγματος',
  'verb.undoLayOff': 'Ακύρωση προσάρτησης',
  'verb.undoMeld': 'Ακύρωση συνδυασμού',
  'verb.undoTakePile': 'Ακύρωση λήψης σωρού',
  'verb.undoTurn': 'Ακύρωση σειράς',

  // --- suits, spelled out ---------------------------------------------------
  //
  // Read aloud by a screen reader and shown where a pip would not fit. The
  // pips themselves are drawn, not written, so these are the only place the
  // suit is ever a word.
  'suit.C': 'Σπαθιά',
  'suit.D': 'Καρό',
  'suit.H': 'Κούπες',
  'suit.S': 'Μπαστούνια',

  // --- counters the board prints beside a number ----------------------------
  'canasta.seat.notOpened': 'Δεν άνοιξε',
  'canasta.unit.points': 'πόντοι',
  'ginrummy.unit.points': 'πόντοι',
  'holdem.seat.dealer': 'Μοιράζει',
  'holdem.seat.folded': 'Πάσο',
  'holdem.seat.allIn': 'Όλα μέσα',
  'holdem.seat.out': 'Εκτός',
  'holdem.unit.chips': 'μάρκες',
  'prsi.unit.cardsLeft': 'φύλλα απομένουν',
  'rummytiles.prompt.initialMeld': 'Το πρώτο σου κατέβασμα πρέπει να αξίζει {n} πόντους.',
  'rummytiles.unit.points': 'πόντοι',
  'zolik.unit.penalty': 'ποινή',
  'header.pileFrozen': 'Ο σωρός πάγωσε',
  // The module-less form of `ginrummy.prompt.yourTurnDraw`: Žolíky and Canasta
  // send the prompt without a module prefix. Neither the Go constants nor
  // `serverKeys.json` list it — the sweep is what found it.
  'prompt.yourTurnDraw': 'Τράβα ένα φύλλο',
  'prompt.yourTurnMeld': 'Κάνε συνδυασμό αν μπορείς και μετά ρίξε',

  // --- replay: stepping through a game that has stopped ---
  'nav.replay': 'Επανάληψη',
  'mine.replay': 'Επανάληψη',
  'replay.loading': 'Άνοιγμα της επανάληψης…',
  'replay.failed': 'Η επανάληψη δεν φορτώθηκε',
  'replay.openHands': 'όλα τα χέρια ανοιχτά',
  'replay.truncated': 'Το παιχνίδι αναπαράγεται ως την κίνηση {at} — οι κανόνες του άλλαξαν έκτοτε',
  'replay.position': '{at} από {of}',
  'replay.theDeal': 'Η μοιρασιά',
  'replay.round': 'Γύρος',
  'replay.track.all': 'Όλα',
  'replay.track.yours': 'Οι κινήσεις σου',
  'replay.track.rounds': 'Γύροι',
  'replay.track.marks': 'Σημάδια',
  'replay.play': 'Αναπαραγωγή',
  'replay.pause': 'Παύση',
  'replay.move': '{player} · {move}',

  // --- the showdown --------------------------------------------------------
  //
  // Hold'em stops after every hand with the cards on the table, and a player
  // may turn their own hand over before the next one is dealt. Most of these
  // earn a line for the usual reason — what they mean lives in their params —
  // and the rest because `humanise` would render "Shown Hand" or "Mucked" as
  // English that looks written rather than missing.
  'err.ALREADY_SHOWN': 'Το χέρι σου είναι ήδη ανοιχτό',
  'err.NOTHING_TO_SHOW': 'Δεν έχεις χέρι για να δείξεις',
  'holdem.offer.show': 'Δείξε το χέρι μου',
  'holdem.offer.finish': 'Τερματισμός αγώνα',
  'holdem.remedy.nothingToShow': 'Δεν έπαιξες σε αυτό το χέρι — περίμενε το επόμενο.',
  'holdem.rules.section.showdown': 'Μετά το χέρι',
  'holdem.rules.stopEveryHand': 'Το παιχνίδι σταματά μετά από κάθε χέρι, για να φαίνονται τα φύλλα.',
  'holdem.rules.revealEveryone': 'Στο φανέρωμα ανοίγουν όλα τα χέρια που πλήρωσαν.',
  'holdem.rules.revealWinners': 'Στο φανέρωμα ανοίγει μόνο το νικητήριο χέρι.',
  'holdem.rules.showYourOwn': 'Μπορείς πάντα να ανοίξεις το δικό σου χέρι, μέχρι να δεχτείς να συνεχίσετε.',
  'holdem.rules.showOnce': 'Ένα χέρι μπορεί να δειχτεί μόνο μία φορά.',
  'holdem.seat.won': 'Κέρδισε',
  'holdem.seat.mucked': 'Δεν έδειξε',
  'holdem.status.mucked': '{playerId} δεν έδειξε',
  'holdem.status.shownVoluntary': '{playerId} έδειξε το χέρι',
  'zone.shownHand': 'Ανοιχτό',
  'option.showdownReveal': 'Φανέρωμα στο showdown',
  'choice.showdownReveal.2': 'Όλοι όσοι πλήρωσαν',
  'choice.showdownReveal.1': 'Μόνο ο νικητής',
  'home.playOffline': 'Παιχνίδι εκτός σύνδεσης',
  'offline.title': 'Παιχνίδι εκτός σύνδεσης',
  'offline.subtitle': 'Χωρίς internet',
  'offline.body': 'Αυτό το τηλέφωνο φιλοξενεί μόνο του το τραπέζι, οπότε μπορείτε να παίξετε με bots παντού — ακόμη και σε λειτουργία πτήσης.',
  'offline.nameLabel': 'Το όνομά σας στο τραπέζι',
  'offline.start': 'Άνοιγμα τραπεζιού εκτός σύνδεσης',
  'offline.starting': 'Εκκίνηση…',
  'offline.active': 'Βρίσκεστε στο τραπέζι εκτός σύνδεσης αυτού του τηλεφώνου. Τα παιχνίδια που ξεκινάτε εδώ μένουν σε αυτό το τηλέφωνο.',
  'offline.chooseGame': 'Επιλέξτε παιχνίδι',
  'offline.backOnline': 'Πίσω στο online παιχνίδι',
  'offline.failed': 'Το τραπέζι εκτός σύνδεσης δεν ξεκίνησε: {reason}',
  'offline.nearbyTitle': 'Τραπέζια κοντά',
  'offline.nearbyLooking': 'Αναζήτηση τραπεζιών σε αυτό το δίκτυο…',
  'offline.nearbyNothing': 'Δεν βρέθηκαν ακόμη τραπέζια. Βεβαιωθείτε ότι είστε στο ίδιο Wi-Fi ή hotspot με τον οικοδεσπότη και ότι το Zolik μπορεί να χρησιμοποιεί το τοπικό δίκτυο.',
  'offline.joinThis': 'Συμμετοχή',
  'offline.byAddressTitle': 'Συμμετοχή με διεύθυνση',
  'offline.versionMismatch': 'Αυτό το τραπέζι τρέχει άλλη έκδοση του Zolik. Ενημερώστε την εφαρμογή και στα δύο τηλέφωνα.',
  'offline.roomTitle': 'Παίκτες κοντά',
  'offline.roomBody': 'Επιτρέψτε σε άλλα τηλέφωνα σε αυτό το Wi-Fi ή hotspot να καθίσουν στο τραπέζι σας. Χωρίς internet.',
  'offline.roomOpenButton': 'Πρόσκληση παικτών κοντά',
  'offline.roomOpen': 'Τα άλλα τηλέφωνα του δικτύου μπορούν τώρα να βρουν το τραπέζι σας. Αν δεν το βλέπουν, μπορούν να πληκτρολογήσουν τη διεύθυνση ή να σαρώσουν τον κωδικό.',
  'offline.roomClose': 'Διακοπή προσκλήσεων',
  'offline.guestActive': 'Βρίσκεστε σε τραπέζι που φιλοξενεί άλλο τηλέφωνο ({host}).',
  'offline.joinFailed': 'Δεν ήταν δυνατή η συμμετοχή σε αυτό το τραπέζι: {reason}',
  'invite.offlineExplain': 'Οι παίκτες αυτού του τραπεζιού εκτός σύνδεσης μπαίνουν με αυτόν τον κωδικό, από το «{menu}».',
  'invite.offlineCode': 'Κωδικός:',
  'offline.bleTitle': 'Μέσω Bluetooth',
  'offline.bleLook': 'Αναζήτηση τραπεζιών μέσω Bluetooth',
  'offline.bleLooking': 'Αναζήτηση τραπεζιών μέσω Bluetooth…',
  'offline.bleOff': 'Ενεργοποιήστε το Bluetooth για να βρείτε τραπέζια χωρίς Wi-Fi.',
  'offline.bleDenied': 'Επιτρέψτε στο Zolik να χρησιμοποιεί Bluetooth στις Ρυθμίσεις για να παίζετε χωρίς Wi-Fi.',
  'offline.bleInvite': 'Πρόσκληση μέσω Bluetooth',
  'offline.bleInviteBody': 'Για τηλέφωνα που δεν μοιράζονται Wi-Fi με το δικό σας. Χωρίς σύζευξη.',
  'offline.bleOpen': 'Τα κοντινά τηλέφωνα μπορούν να συνδεθούν μέσω Bluetooth. Συνδεδεμένα: {n}.',
  'offline.bleStop': 'Διακοπή Bluetooth',
  'offline.bleCheck': 'Κωδικός ελέγχου: {code}',
  'offline.guestBle': 'Βρίσκεστε μέσω Bluetooth σε τραπέζι που φιλοξενεί άλλο τηλέφωνο.',
  'offline.keyChanged': 'Αυτό δεν είναι το τηλέφωνο στο οποίο παίξατε πριν με αυτό το όνομα. Ρωτήστε τον οικοδεσπότη ή συνδεθείτε μέσω Wi-Fi.',
};
