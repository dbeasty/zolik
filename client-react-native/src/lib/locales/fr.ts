/**
 * French. Rami vocabulary: groupe for a set, suite for a run, combinaison for a meld, pioche and défausse for the two piles.
 */

export const fr: Record<string, string> = {
  // --- engine error codes (rules.RulesErrorCode) ---------------------------
  'err.NOT_YOUR_TURN': "Ce n'est pas ton tour",
  'err.WRONG_PHASE': 'Indisponible pour le moment',
  'err.MUST_DRAW_FIRST': 'Pioche une carte avant de poser',
  'err.GAME_SUSPENDED': 'La partie est en pause',
  'err.GAME_NOT_ACTIVE': "La partie n'est pas en cours",
  // --- runtime refusals, which are not the rules saying no -----------------
  //
  // Both were reachable long before they were worded, and a player who met
  // one was shown the code in capitals or, worse, nothing at all: an action
  // sent while the socket was down simply vanished, which reads as a legal
  // move being silently refused.
  'err.MATCH_NOT_ACTIVE': "La table est en pause — on attend qu'un joueur se reconnecte",
  'err.NOT_CONNECTED': 'Pas connecté à la table — reconnexion, puis réessaie',
  // Why the "start the next round" control is greyed out. Without wording the
  // shell printed the code itself, so a player agreeing to go on was answered
  // with ALREADY_READY in capitals.
  'err.ALREADY_READY': 'Tu es prêt',
  'err.NOT_BETWEEN_ROUNDS': 'La manche est encore en cours',
  'err.NOT_AT_THIS_TABLE': "Tu n'es pas à cette table",
  // Bringing a table back that the sweeper set aside. Refusals a player
  // can actually provoke: a link to a table that is gone, one that was
  // already picked up in another tab, and a table with other people at it.
  'err.MATCH_MOVED_ON': 'La table a changé — recharge la page',
  'err.MATCH_NOT_ABANDONED': 'Cette table n’attend pas d’être reprise',
  'err.MATCH_NOT_FOUND': 'Cette table n’existe plus',
  'err.TABLE_HAS_OTHER_PLAYERS': 'Seule une table où tous les autres sont des bots peut être reprise',
  'err.DISCARD_LOCKED': "La défausse est verrouillée pour l'instant",
  'err.DISCARD_PILE_EMPTY': 'La défausse est vide',
  'err.NO_CARDS_LEFT': 'Plus de cartes à piocher',
  'err.ROUND_REQ_NOT_MET': "Pose d'abord ta propre ouverture",
  'err.NEED_CLEAN_RUN': 'Il te faut une suite sans joker sur la table pour être considéré comme posé',
  'err.INCOMPLETE_INITIAL_MELD': 'Termine ta pose, ou annule-la, avant de défausser',
  'err.DISCARD_CARD_NOT_MELDED': 'La carte ramassée doit entrer dans ta combinaison',
  'err.JOKER_DISCARD_FORBIDDEN': 'Un joker ne peut pas être défaussé',
  'err.NOTHING_TO_UNDO': 'Rien à annuler',
  'err.NO_JOKER_IN_MELD': 'Aucun joker dans cette combinaison',
  'err.JOKER_SWAP_MISMATCH': 'Cette carte ne prend pas la place du joker',
  'err.RECLAIMED_JOKER_NOT_MELDED':
    'Le joker repris sur la table doit être joué dans une combinaison ce tour-ci',
  'err.RUN_TOO_LONG': 'Cette suite est déjà à sa longueur maximale',
  'err.WRONG_RUN_END': "Cette carte prolonge l'autre bout de la suite",
  'err.INVALID_MELD': 'Aucune carte de ta main ne convient ici',
  'err.CARD_NOT_IN_HAND': "Cette carte n'est pas dans ta main",
  'err.MELD_BELOW_MINIMUM': "Tes combinaisons n'atteignent pas encore les points requis pour poser",
  'err.MELD_NO_CONTRIBUTION': 'Cette combinaison ne fait pas avancer ton contrat',
  'err.TOO_MANY_WILDS': 'Trop de jokers dans cette combinaison',
  'err.ADJACENT_WILDS': 'Deux jokers ne peuvent pas se suivre',
  'err.ACE_BRIDGE': 'Un as ne peut pas relier le roi et le deux',

  // --- contract phrasing ---------------------------------------------------
  // Built from the structured contract the server sends (sets/runs counts),
  // never from a deal-number lookup table.
  // Whole phrases per count rather than a number plus a pluralised noun:
  // Czech inflects the noun by count (skupina / skupiny / skupin) in a way no
  // "add an s" helper survives, and the counts that actually occur are 1-3.
  // A count outside that falls back to contract.sets.n.
  'contract.sets.1': 'Un groupe',
  'contract.sets.2': 'Deux groupes',
  'contract.sets.3': 'Trois groupes',
  'contract.sets.n': '{n} groupes',
  'contract.runs.1': 'Une suite',
  'contract.runs.2': 'Deux suites',
  'contract.runs.3': 'Trois suites',
  'contract.runs.n': '{n} suites',
  'contract.any': 'Toute combinaison valide',
  'contract.cleanRunOnly': 'Tout mélange de groupes et de suites — au moins une suite doit être sans joker',
  'contract.cleanRunSuffix': '{base} — une suite doit être sans joker',

  // --- written rules ---------------------------------------------------------
  // Full-sentence keys a "see the rules" screen renders, one per module,
  // resolved server-side against the table's actual variation and options —
  // see internal/module/rules.go and each game's rules.go. Shared section
  // titles first, then one block per module in the order the four games were
  // added.
  'zolik.rules.section.goal': 'But',
  'zolik.rules.section.setup': 'Mise en place',
  'zolik.rules.section.turn': 'Ton tour',
  'zolik.rules.section.melding': 'La pose',
  'zolik.rules.section.end': 'Fin de la partie',
  'zolik.rules.goal':
    "Sois le premier à vider ta main en posant des groupes et des suites valides, en gardant le moins de points de pénalité possible dans les cartes qui te restent quand quelqu'un d'autre termine.",
  'zolik.rules.deal': 'Chaque joueur reçoit {n} cartes.',
  'zolik.rules.meldShapes':
    'Un groupe réunit {set}+ cartes de même rang ; une suite réunit {run}+ cartes consécutives de même couleur.',
  'zolik.rules.turn.draw': 'À ton tour, pioche une carte — dans la pioche ou dans la défausse.',
  'zolik.rules.pickup.topOnly': 'Seule la carte du dessus de la défausse peut être prise.',
  'zolik.rules.pickup.anyFromPile':
    "N'importe quelle carte de la défausse peut être prise, avec tout ce qui la recouvre.",
  'zolik.rules.pickup.locked': 'On ne peut pas piocher dans la défausse avant la manche {n}.',
  'zolik.rules.pickup.open': 'La défausse est ouverte dès la première manche.',
  'zolik.rules.turn.discard': 'Termine ton tour en défaussant une carte.',
  'zolik.rules.jokers.restricted':
    "Un joker ne peut jamais être défaussé, sauf s'il est exactement la carte qui vide ta main.",
  'zolik.rules.lead.rotate': "L'entame tourne d'un siège à chaque donne, quel que soit le vainqueur.",
  'zolik.rules.lead.winner': 'Celui qui termine entame la donne suivante.',
  'zolik.rules.meldFloor.on':
    'Ta première pose doit totaliser au moins {n} points naturels pour que tu sois posé.',
  'zolik.rules.meldFloor.off': "Aucun minimum de points n'est exigé sur ta première pose.",
  'zolik.rules.cleanRun.on':
    'Au moins une de tes suites doit être entièrement sans joker pour que tu comptes comme posé.',
  'zolik.rules.cleanRun.off':
    "Tes suites peuvent utiliser les jokers librement — aucune n'a besoin d'être sans joker.",
  'zolik.rules.contracts.rotating':
    'La partie dure {n} donnes, et chaque donne exige sa propre combinaison de groupes et de suites.',
  'zolik.rules.contracts.static': 'Chaque donne exige la même combinaison : {sets} groupes et {runs} suites.',
  'zolik.rules.end.afterDeals': "La partie s'achève après {n} donnes.",
  'zolik.rules.end.atScore':
    "On redonne jusqu'à ce que quelqu'un atteigne {n} points — la partie est alors terminée.",

  'prsi.rules.section.goal': 'But',
  'prsi.rules.section.setup': 'Mise en place',
  'prsi.rules.section.turn': 'Ton tour',
  'prsi.rules.section.special': 'Cartes spéciales',
  'prsi.rules.section.end': 'Fin de la partie',
  'prsi.rules.goal': 'Sois le premier à jouer toutes les cartes de ta main.',
  'prsi.rules.deck': 'Se joue avec un jeu de {value} cartes (à partir du 7).',
  'prsi.rules.deal': 'Chaque joueur commence avec {n} cartes.',
  'prsi.rules.turn.match':
    'Joue une carte de la même couleur ou du même rang que celle du dessus — ou pioche si tu ne peux pas.',
  'prsi.rules.turn.draw': 'Piocher met fin à ton tour sans jouer de carte.',
  'prsi.rules.sevens':
    "Joue un 7 et le joueur suivant pioche deux cartes, à moins qu'il ne réponde par un 7.",
  'prsi.rules.aces': 'Joue un as et le tour du joueur suivant est sauté.',
  'prsi.rules.queens': 'Joue une dame et annonce la couleur qui continue.',
  'prsi.rules.end': "La partie s'arrête dès qu'une main est vide.",

  'canasta.rules.section.goal': 'But',
  'canasta.rules.section.setup': 'Mise en place',
  'canasta.rules.section.melding': 'La pose',
  'canasta.rules.section.end': 'Fin de la partie',
  'canasta.rules.goal': 'On joue en équipes ; le premier camp à atteindre {n} points remporte la partie.',
  'canasta.rules.deck': 'Se joue avec {value} cartes — {decks} jeux plus les jokers.',
  'canasta.rules.deal': 'Chaque joueur reçoit {n} cartes.',
  'canasta.rules.drawCount': 'Tu pioches {n} cartes au début de ton tour.',
  'canasta.rules.redThrees':
    'Un trois rouge dans ta main est montré aussitôt et compte en bonus — sauf si ton camp ne réalise jamais de canasta, auquel cas il compte contre toi.',
  'canasta.rules.canasta': 'Une canasta est une combinaison de {n} cartes ou plus de même rang.',
  'canasta.rules.sequences': 'Une combinaison peut aussi être une séquence : trois cartes ou plus de la même couleur qui se suivent, jamais avec un joker parmi elles.',
  'canasta.rules.samba': 'Une séquence de sept cartes est un samba, qui vaut {n} points.',
  'canasta.rules.pileAlwaysFrozen': 'La défausse est gelée toute la donne : pour la prendre, tu dois associer sa carte du dessus à deux cartes naturelles de ta main.',
  'canasta.rules.meldFloorBands':
    "Ta première pose doit atteindre un minimum de points qui monte avec ton score : {negative} en dessous de zéro, {low} jusqu'à 1500, {mid} jusqu'à 3000, {high} au-delà.",
  'canasta.rules.meldFloorBandsFive': "Ta première combinaison doit atteindre un minimum de points qui monte avec ton score : {negative} sous zéro, {low} jusqu'à 1500, {mid} jusqu'à 3000, {high} jusqu'à 7000, {top} au-delà.",
  'canasta.rules.oneCanastaToGoOut': 'Une canasta terminée suffit à ton camp pour sortir.',
  'canasta.rules.twoCanastasToGoOut': 'Ton camp a besoin de deux canastas terminées avant de pouvoir sortir.',
  'canasta.rules.end': "On redonne jusqu'à ce qu'un camp dépasse {n} points — la partie est alors terminée.",

  'holdem.rules.section.goal': 'But',
  'holdem.rules.section.setup': 'Mise en place',
  'holdem.rules.section.betting': 'Les enchères',
  'holdem.rules.section.end': 'Fin de la partie',
  'holdem.rules.goal':
    "Gagne des jetons en ayant la meilleure main à l'abattage, ou en restant seul en lice.",
  'holdem.rules.stack': 'Chaque siège débute avec {n} jetons.',
  'holdem.rules.blinds': 'La petite blinde est de {sb} et la grosse de {bb}, posées avant la distribution.',
  'holdem.rules.streets': 'On mise en quatre tours — avant le flop, puis après le flop, le turn et la river.',
  'holdem.rules.showdown':
    'Ceux qui restent en lice dévoilent leurs cartes ; la meilleure main de cinq cartes emporte le pot.',
  'holdem.rules.noLimit': "Mise sans limite — toute mise peut aller jusqu'à la totalité de ton tapis.",
  'holdem.rules.lastPlayerStanding': "On joue jusqu'à ce qu'un siège détienne tous les jetons.",
  'holdem.rules.mostChipsWins': "Celui qui détient le plus de jetons à l'arrêt du jeu remporte la partie.",
  'holdem.rules.handLimit': "Le jeu s'arrête après {n} mains.",

  // --- header --------------------------------------------------------------
  'header.deal': 'Donne {n}',
  'header.gameOf': 'Partie {n} sur {total}',
  'header.gameOfWithContract': 'Partie {n} sur {total} : {contract}',

  // --- meld preview --------------------------------------------------------
  'preview.validSet': 'Groupe valide',
  'preview.validRun': 'Suite valide',
  'preview.validMeld': 'Combinaison valide',
  'preview.notYet': 'Pas encore une combinaison',
  'preview.points': '{shape} · {n} points',
  'preview.pointsWithLaid': '{shape} · {n} + {laid} déjà posés = {total} points',
  'preview.meetsFloor': '{line} (atteint {n} ✓)',
  'preview.needsFloor': '{line} (exige {n} ✗)',
  'preview.becauseOf': '{line} — {reason}',

  // --- discard -------------------------------------------------------------
  // A discard with a meld staged lays that meld first; this is what the
  // player is told when the server refuses it and the whole move is rolled
  // back. {reason} is the engine's own words for the refusal.
  'discard.meldRejected': "{reason} — rien n'a été défaussé, tes cartes sont toujours en attente.",

  // --- what the current selection would send -------------------------------
  // The client's own reason a control is not ready, next to the engine's own
  // `err.*` reasons rather than a second kind of message — a control greyed
  // out for "not your turn" and one greyed out for "you picked two, this
  // takes one" should read as the same kind of thing, not one of them looking
  // broken. See `fits` in `src/lib/drops.ts`.
  'sel.tooMany.1': "Ne choisis qu'une seule carte",
  'sel.tooMany.n': 'Choisis au plus {n} cartes',
  'sel.needMore': 'Choisis {n} carte(s)',
  'sel.notThese': 'Ces cartes ne peuvent pas aller là',
  'sel.needsCompany': "Cette carte a besoin de celles qui l'entourent",

  // --- what a module says happened -----------------------------------------
  // The first module-sent keys in this bundle, and deliberately few. Anything
  // a module sends is still legible without an entry here — `humanise` turns
  // `holdem.seat.stack` into "Stack" — so a key earns a line only when that
  // fallback loses something. These do: what each of them means lives in its
  // *params* (who won, how much, whose cards), and a key on its own has
  // nowhere to put them. Without the entry the player read "Winner", full
  // stop, at the end of a match they had just won.
  'status.winner': 'Gagné par {winners}',
  'holdem.status.pot': '{winners} gagne {amount} avec {hand}',
  'holdem.status.potUncontested': '{winners} gagne {amount} — tous les autres se sont couchés',
  'holdem.status.shown': '{playerId} a montré {value}',
  'holdem.prompt.waitingFor': 'En attente de {playerId}',

  // How a rummy scoreboard was ordered. Both earn a line for the same reason
  // as the keys above: the number lives in the params, and `humanise` would
  // render "Deals Won" with nothing after it.
  'zolik.standing.dealsWon': 'Donnes gagnées {n}',
  'zolik.standing.inHand': 'En main {n}',

  // --- rounds ---------------------------------------------------------------
  // What a round is called, and what one did. Each of these carries its number
  // in the params, so `humanise` alone would render a label with nothing after
  // it.
  // An instruction, not a heading. "Next round" named the thing rather than
  // asking for it, and sat above "Waiting for 1" — which reads as though the
  // player is the one waiting, when the table is waiting for them.
  'round.continue': 'Commencer la manche suivante',

  // --- the flash that announces a round or a match ending ------------------
  //
  // One line and one number, held for about two seconds. Worded generically on
  // purpose: which *kind* of ending it was is the module's own fact, printed
  // underneath, and a client that phrased "went gin" itself would be a client
  // that knows a game.
  'flash.roundWon': "{winners} l'emporte",
  'flash.roundWonYou': "Tu l'emportes",
  'flash.roundDrawn': "Personne ne l'emporte",
  'flash.matchOver': 'Partie terminée',
  'flash.matchWon': '{winners} gagne',
  'flash.matchWonYou': 'Tu gagnes',
  'flash.matchDrawn': 'Personne ne gagne',
  'flash.nowOn': 'maintenant {total}',

  'zolik.round.deal': 'Donne',
  'zolik.round.cleanRun': 'Une suite doit être sans joker',
  'canasta.round.deal': 'Donne',
  'canasta.round.concealed': 'Sortie en main fermée',
  'canasta.round.exhausted': "Le jeu s'est épuisé",
  'canasta.round.meldCards': 'Cartes posées {n}',
  'canasta.round.canastas': 'Canastas {n}',
  'canasta.round.redThrees': 'Trois rouges {n}',
  'canasta.round.goingOut': 'Sortie {n}',
  'canasta.round.inHand': 'Pris en main {n}',
  'holdem.round.hand': 'Main',
  'holdem.round.pot': 'Pot {n}',
  'holdem.round.uncontested': 'Tous les autres se sont couchés',
  'seat.ready': 'Prêt',
  'zolik.seat.contractMet': 'Contrat rempli',
  'results.you': '(toi)',



  // --- refusals the other three games raise ---------------------------------
  // Worded here for the same reason Žolíky's are: reasonText falls back to the
  // code, and OfferBar hands it the code as that fallback, so an unworded
  // refusal reaches a player as PILE_FROZEN in capitals. serverKeys.test.ts
  // now fails the build rather than letting the next one through.
  'err.SET_TOO_LARGE': 'Un groupe contient déjà les quatre couleurs',
  'err.DISCARD_TAKEN_CARD_FORBIDDEN':
    'Tu ne peux pas défausser la carte que tu viens de prendre — joue-la ou garde-la',
  'err.CARD_DOES_NOT_FIT': 'Cette carte ne correspond ni à la couleur ni au rang',
  'err.SUIT_REQUIRED': 'Annonce la couleur qui continue',
  'err.MUST_ANSWER_DRAW_OR_TAKE': 'Réponds par un sept, ou prends les cartes',
  'err.NOTHING_TO_DRAW': "Il n'y a plus rien à piocher",
  'err.PILE_EMPTY': 'La pile est vide',
  'err.PILE_BLOCKED': 'La pile est bloquée — un trois noir est sur le dessus',
  'err.PILE_FROZEN': 'La pile est gelée — il te faut deux cartes naturelles du rang de la carte du dessus',
  'err.TOP_CARD_UNUSABLE': 'Tu ne peux pas utiliser la carte du dessus',
  'err.MELD_CLOSED': 'Cette combinaison est complète et fermée',
  'err.MELD_TOO_SMALL': 'Une combinaison exige plus de cartes que cela',
  'err.MELD_TOO_LARGE': 'Cette combinaison ne peut plus accueillir de cartes',
  'err.MELD_MIXED_RANKS': "Toutes les cartes d'une combinaison doivent être du même rang",
  'err.SEQUENCE_NO_WILDS': 'Une séquence ne peut pas contenir de jokers',
  'err.SEQUENCE_NEEDS_ONE_SUIT': "Toutes les cartes d'une séquence doivent être de la même couleur",
  'err.RUN_NOT_CONSECUTIVE': 'Une séquence doit se suivre sans trou',
  'err.NOT_ENOUGH_NATURALS': 'Une combinaison exige plus de cartes naturelles que de jokers',
  'err.RANK_ALREADY_MELDED': 'Ton camp a déjà une combinaison de ce rang',
  'err.NOT_YOUR_MELD': 'Cette combinaison appartient au camp adverse',
  'err.NO_SUCH_MELD': "Cette combinaison n'est pas sur la table",
  'err.CANNOT_MELD_THREE': 'Les trois ne se posent jamais',
  'err.CANNOT_DISCARD_RED_THREE': 'Un trois rouge ne peut pas être défaussé',
  'err.MUST_KEEP_A_CARD': 'Garde au moins une carte — tu ne peux pas vider ta main ainsi',
  'err.MUST_MELD_FIRST': "Pose d'abord l'ouverture de ton camp",
  'err.INITIAL_MELD_NOT_MET': "Ta première pose n'atteint pas encore les points requis",
  'err.CANNOT_GO_OUT_YET': "Ton camp a besoin d'une canasta terminée avant de pouvoir sortir",
  'err.NOTHING_TO_CALL': "Il n'y a aucune mise à suivre",
  'err.CANNOT_CHECK': 'Tu ne peux pas checker — il y a une mise à répondre',
  'err.CANNOT_RAISE': 'Tu ne peux pas relancer ici',
  'err.RAISE_TOO_SMALL': 'Une relance doit valoir au moins la précédente',
  'err.NOT_ENOUGH_CHIPS': "Tu n'as pas autant de jetons",
  'err.AMOUNT_REQUIRED': 'Indique combien',
  'err.AMOUNT_NOT_A_NUMBER': "Ce montant n'est pas un nombre",
  'err.SEAT_NOT_IN_HAND': 'Tu ne participes pas à cette main',
  'err.WRONG_RANK': "Cette carte n'a pas le bon rang pour cela",
  'err.MATCH_FULL': 'La table est complète',
  'err.MATCH_ALREADY_STARTED': 'La partie a déjà commencé',
  'err.TOO_FEW_PLAYERS': 'Pas encore assez de joueurs',
  'err.WRONG_PLAYER_COUNT': 'Ce jeu ne peut pas se jouer à ce nombre de joueurs',
  'err.NOT_THE_HOST': "Seul l'hôte peut faire cela",
  'err.NO_LONGER_WAITING': "La table n'attend plus",
  'err.WAITING_ROOM_UNAVAILABLE': "La salle d'attente n'est pas disponible",
  'err.SERVER_BUSY': 'Le serveur est saturé — réessaie dans un instant',


  // --- rules the engine has always enforced and never stated ----------------
  // Every one of these was found by the guardrail rather than by review: the
  // validator could refuse a player for it, and no sentence anywhere said so.
  // See server/internal/zolikmod/ruleindex_test.go.
  'zolik.rules.section.layoff': 'Ajouter aux combinaisons',
  'zolik.rules.pickup.obligation':
    "Avant d'être posé, une carte prise dans la défausse doit servir dans la combinaison avec laquelle tu te poses ce tour-ci.",
  'zolik.rules.pickup.noReturn':
    'Une carte prise dans la défausse ne peut pas y retourner le même tour — joue-la ou garde-la.',
  'zolik.rules.wilds.setLimit': 'Un groupe ne peut pas contenir plus de jokers que de cartes naturelles.',
  'zolik.rules.set.maxSize':
    "Un groupe ne peut pas dépasser {n} cartes — un joker remplace une couleur manquante, il n'en rajoute pas à un groupe complet.",
  'zolik.rules.run.maxLength':
    "Une suite ne peut pas dépasser {n} cartes — l'as en bas, les douze rangs au-dessus, et l'as en haut.",
  'zolik.rules.run.aceBridge':
    "Un as se place au-dessus du roi ou en dessous du deux, jamais en pont entre les deux bouts d'une suite.",
  'zolik.rules.contracts.contribution':
    "Tant que tu n'es pas posé, chaque combinaison posée doit être une de celles que le contrat de la donne réclame encore.",
  'zolik.rules.layoff.afterDown':
    "Tu ne peux rien ajouter aux combinaisons d'autrui tant que tu n'as pas posé ton propre contrat.",
  'zolik.rules.layoff.runEnds': "Une carte ajoutée à une suite doit la prolonger à l'un ou l'autre bout.",
  'zolik.rules.jokers.swap':
    "Un joker posé sur la table peut être racheté avec la carte exacte qu'il représente.",
  'zolik.rules.jokers.reclaim.on':
    'Un joker racheté sur la table doit être joué dans une combinaison le même tour — il ne peut pas rester en main.',
  'zolik.rules.jokers.reclaim.off': 'Un joker racheté sur la table peut être gardé en main.',
  'zolik.rules.deck.reshuffle':
    "Quand la pioche s'épuise, la défausse est mélangée et devient la nouvelle pioche ; si les deux sont vides, la donne s'arrête.",


  // --- what to do instead ---------------------------------------------------
  // The third layer of a refusal, after the reason and the rule. Sent by the
  // module because it is the only side that knows which card is owed and
  // which way out is on offer; see server/internal/zolikmod/remedy.go.
  'zolik.remedy.meldThePickup': 'Ajoute {card} à ta pose, ou annule le ramassage.',
  'zolik.remedy.discardSomethingElse': 'Défausse une autre carte, ou joue {card} ce tour-ci.',
  'zolik.remedy.discardNotAJoker': "Défausse autre chose qu'un joker.",
  'zolik.remedy.finishOrUndoLayDown': 'Termine ta pose, ou reprends-la.',
  'zolik.remedy.needMorePoints': 'Il te faut {n} points de plus pour pouvoir poser.',
  'zolik.remedy.layACleanRun': 'Pose une suite sans aucun joker.',
  'zolik.remedy.playReclaimedJoker': 'Joue {card} dans une combinaison, ou annule sa reprise.',
  'zolik.remedy.goDownFirst': "Pose d'abord tes propres combinaisons.",
  'zolik.remedy.drawFirst': "Pioche d'abord une carte.",
  'zolik.remedy.drawFromStock': "Pioche dans la pioche — la défausse s'ouvre à la manche {n}.",
  'zolik.remedy.drawFromStockEmpty': 'Pioche plutôt dans la pioche.',


  // --- keys whose meaning lives in their params -----------------------------
  // The shape fallback renders these with the number missing ("Stack", "Pot"),
  // which is why they earn a line where most keys do not.
  'header.contract': 'Exige {sets} groupes et {runs} suites',
  'header.contract.cleanRunOnly': 'Exige une suite sans joker',
  'header.round': 'Manche {n}',
  'header.deck': 'Pioche',
  'header.target': 'Objectif',
  'header.suitInPlay': 'Couleur en jeu',
  'seat.cards': 'Cartes',
  'zolik.offer.meld': 'Poser',
  'prompt.pickupMustBeMelded':
    '{value} vient de la défausse — cette carte doit entrer dans les combinaisons avec lesquelles tu te poses ce tour-ci.',
  'prompt.jokerMustBePlayed':
    '{value} vient de la table — cette carte doit entrer dans une combinaison avant que tu puisses finir ton tour.',
  'prompt.initialMeld': "L'ouverture de ton camp doit atteindre {n} points.",
  'prompt.canastasNeeded': "Il manque {n} canastas à ton camp avant qu'il puisse sortir.",
  'prompt.mustDrawOrAnswerSeven': 'Réponds par un sept, ou pioche {n} cartes.',
  'prompt.chooseSuit': 'Choisis la couleur qui continue',
  'prompt.skipPending': 'Ton tour est sauté',
  'status.lastDeal': "L'équipe {team} a marqué {value}",
  'status.teamScore': 'Équipe {team} : {value}',
  'canasta.offer.rank': 'Rang',
  'canasta.offer.sequence': 'Séquence',
  'badge.naturalCanasta': 'Canasta pure',
  'badge.mixedCanasta': 'Canasta mixte',
  'badge.samba': 'Samba',
  'badge.cleanRun': 'Suite pure',
  'canasta.seat.teamScore': "Score de l'équipe",
  'canasta.seat.canastas': 'Canastas',
  'holdem.header.pot': 'Pot',
  'holdem.header.street': 'Tour',
  'holdem.header.hand': 'Main',
  'holdem.header.handLimit': 'Mains au total',
  'holdem.header.blinds': 'Blindes',
  'holdem.cost.call': 'pour suivre',
  'holdem.cost.pot': 'dans le pot',
  'holdem.seat.stack': 'Tapis',
  'holdem.seat.bet': 'Mise',
  'holdem.prompt.yourAction': 'À toi de parler',
  'holdem.prompt.raiseTo': 'Relancer à',
  'holdem.quick.halfPot': '½ Pot',
  'holdem.quick.pot': 'Pot',
  'holdem.quick.allIn': 'All-in',
  'zone.yourHand': 'Ta main',
  'zone.opponentHand': 'Sa main',
  'zone.drawPile': 'Pioche',
  'zone.discardPile': 'Défausse',
  'zone.melds': 'Combinaisons',
  'zone.teamMelds': 'Combinaisons de ton camp',
  'zone.opponentMelds': 'Combinaisons du camp adverse',
  'zone.redThrees': 'Trois rouges',
  'zone.board': 'Tableau',
  'verb.drawFromDeck': 'Piocher',
  'verb.takeFromDiscard': 'Prendre dans la défausse',


  // --- the why sheet's own furniture ---------------------------------------
  // The three layers a refusal is explained in. Labels, not sentences: what
  // goes under each is sent by the server or worded above.
  'why.reason': 'Pourquoi pas',
  'why.rule': 'La règle',
  'why.rules': 'Les règles',
  'why.remedy': 'Ce que tu peux faire',
  'why.readTheRules': 'Lire les règles complètes →',
  'why.close': 'Fermer',
  'why.open': 'pourquoi',

  // --- marks on a particular card ------------------------------------------
  // A mark is a refusal that has not happened yet: the rule is enforced at the
  // discard, which is the last possible moment to hear about it, so the card
  // says so while there is still a turn left to act on it.
  'zolik.badge.owedToMeld':
    '{card} vient de la défausse — cette carte doit entrer dans les combinaisons avec lesquelles tu te poses ce tour-ci.',
  'zolik.badge.jokerOwed':
    '{card} vient de la table — cette carte doit entrer dans une combinaison avant que tu puisses finir ton tour.',

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
  'legal.terms': 'Conditions',
  // The document's own name, used as the heading above it. Kept apart from
  // `legal.notice.terms`, which is the same words inflected to sit inside a
  // sentence — a heading in the instrumental case reads as a mistake in every
  // Slavic language.
  'legal.terms.title': "Conditions d'utilisation",
  'legal.privacy.title': 'Politique de confidentialité',
  'legal.privacy': 'Confidentialité',
  'legal.source': 'Code source',
  'legal.updated': 'Version {version}',
  'legal.draft':
    "Projet — pas encore en vigueur. Le nom, le pays et l'adresse de contact de l'exploitant restent à renseigner.",
  'legal.notice.before': 'En jouant, tu acceptes les ',
  'legal.notice.terms': "conditions d'utilisation",
  'legal.notice.between': '. Ce qui est conservé à ton sujet figure dans la ',
  'legal.notice.privacy': 'politique de confidentialité',
  'legal.notice.after': '.',
  // --- gin rummy -------------------------------------------------------------
  'err.UPCARD_DECLINED': 'Tu as déjà passé sur cette carte',
  'err.DEADWOOD_TOO_HIGH': 'Ton deadwood est trop élevé pour frapper',
  'err.CARD_DOES_NOT_EXTEND_MELD': 'Cette carte ne prolonge pas cette combinaison',
  'ginrummy.rules.setup': 'Mise en place',
  'ginrummy.rules.turn': 'Ton tour',
  'ginrummy.rules.melds': 'Combinaisons',
  'ginrummy.rules.knocking': 'Frapper',
  'ginrummy.rules.bigGin': 'Big gin',
  'ginrummy.rules.layoff': 'Le report',
  'ginrummy.rules.deadHand': 'La main morte',
  'ginrummy.rules.scoring': "Le décompte d'une main",
  'ginrummy.rules.match': 'Remporter la partie',
  'ginrummy.rules.lineBonuses': 'Bonus de décompte',
  'ginrummy.rules.deck': 'Se joue avec un jeu de {value} cartes.',
  'ginrummy.rules.deal': 'Chaque joueur reçoit {value} cartes.',
  'ginrummy.rules.upcard': 'Une carte de plus est retournée face visible pour amorcer la défausse.',
  'ginrummy.rules.drawDiscard':
    'À ton tour, pioche une carte — dans la pioche ou dans la défausse — puis défausse-en une.',
  'ginrummy.rules.setsAndRuns':
    "Une combinaison est un groupe de trois ou quatre cartes d'un même rang, ou une suite d'au moins trois cartes d'une même couleur.",
  'ginrummy.rules.aceLow': "L'as est toujours bas — il n'existe pas de suite allant de la dame à l'as.",
  'ginrummy.rules.knockLimit': 'Tu peux frapper dès que ton deadwood tombe à {n} ou moins.',
  'ginrummy.rules.oklahoma':
    'La limite pour frapper dans cette main est fixée par la valeur de la carte retournée.',
  'ginrummy.rules.gin': "Un deadwood nul, c'est gin — le meilleur coup possible.",
  'ginrummy.rules.bigGinBonus':
    "Onze cartes toutes combinées, sans même défausser, c'est big gin, qui vaut {n} points de plus.",
  'ginrummy.rules.layoffDescription':
    "Après une frappe qui n'est pas gin, ton adversaire peut reporter son propre deadwood sur tes combinaisons avant que les mains soient comparées.",
  'ginrummy.rules.deadHandDescription':
    "S'il ne reste que deux cartes dans la pioche et que personne n'a frappé, la main est morte — personne ne marque, et le même donneur redonne.",
  'ginrummy.rules.undercut':
    "Si le deadwood de ton adversaire n'est pas supérieur au tien, il te contre : il marque la différence, plus {n}.",
  'ginrummy.rules.ginBonus': 'Gin rapporte toute la main de ton adversaire, plus {n}.',
  'ginrummy.rules.target': "Le premier à dépasser {n} points à la fin d'une main remporte la partie.",
  'ginrummy.rules.shutout': "Le bonus de partie double à {n} si le perdant n'a pas marqué le moindre point.",
  'ginrummy.rules.box': 'Chaque main gagnée vaut {n} points à la fin de la partie.',
  'ginrummy.rules.gameBonus': 'Remporter la partie rapporte {n} points de plus.',
  'ginrummy.fact.deadwood': '{value} de deadwood',
  'ginrummy.fact.discardCard': 'Défausser {value}',
  'ginrummy.fact.meldCards': 'Sur {value}',
  'ginrummy.header.hand': 'Main {n}',
  'ginrummy.round.kind': '{value}',
  'ginrummy.round.hand': 'Main',
  'ginrummy.seat.score': '{value}',
  'ginrummy.seat.dealer': 'Donneur',
  'ginrummy.status.knocked': '{playerId} a frappé avec {deadwood} de deadwood',
  'ginrummy.status.gin': '{playerId} a fait gin',
  'ginrummy.status.lastHand': 'Dernière main : {winner} ({kind}, {delta} points)',
  'ginrummy.offer.drawStock': 'Piocher dans la pioche',
  'ginrummy.offer.drawDiscard': 'Piocher dans la défausse',
  'ginrummy.offer.takeUpcard': 'Prendre la carte retournée',
  'ginrummy.offer.passUpcard': 'Passer',
  'ginrummy.offer.discard': 'Défausser',
  'ginrummy.offer.knock': 'Frapper',
  'ginrummy.offer.gin': 'Gin !',
  'ginrummy.offer.bigGin': 'Big gin !',
  'ginrummy.offer.layOff': 'Reporter',
  'ginrummy.offer.finishLayoff': 'Report terminé',
  'ginrummy.zone.knockerHand': 'Main frappée',
  'ginrummy.zone.melds': 'Combinaisons',
  'ginrummy.prompt.upcardDecision': 'Prends la carte retournée, ou passe',
  'ginrummy.prompt.yourTurnDraw': 'Pioche une carte',
  'ginrummy.prompt.yourTurnDiscard': 'Défausse — ou frappe, si tu le peux',
  'ginrummy.prompt.layoff': 'Reporte ton deadwood, ou termine',

  // --- rummy tiles -------------------------------------------------------------
  'err.TILE_NOT_IN_HAND': "Cette tuile n'est pas dans ta main",
  'err.TILE_DOES_NOT_FIT': 'Cela ne va pas là',
  'err.NO_SUCH_SET': "Cette combinaison n'est pas sur la table",
  'err.INITIAL_MELD_ONLY':
    'Avant ta première pose, tu ne peux réarranger que tes propres nouvelles combinaisons',
  'err.TABLE_NOT_VALID': "La table n'est pas encore valide",
  'err.TRAY_NOT_EMPTY': 'Il te reste des tuiles à placer',
  'err.NOTHING_PLAYED': 'Pose au moins une tuile avant de terminer ton tour',
  'err.INITIAL_MELD_TOO_LOW': 'Ta première pose doit valoir au moins 30 points',
  'err.NOT_A_RUN': 'Seule une suite peut être coupée',
  'err.BAD_SPLIT_POSITION': 'Cette suite ne peut pas être coupée à cet endroit',
  'err.NO_JOKER_IN_SET': "Il n'y a pas de joker dans cette combinaison",
  'err.TILE_JOKER_SWAP_MISMATCH': "Cette tuile n'est pas ce que le joker représente",
  'rummytiles.rules.setup': 'Mise en place',
  'rummytiles.rules.sets': 'Combinaisons',
  'rummytiles.rules.initialMeld': 'La première pose',
  'rummytiles.rules.turn': 'Ton tour',
  'rummytiles.rules.jokerTaking': 'Reprendre un joker',
  'rummytiles.rules.ending': 'Terminer une manche',
  'rummytiles.rules.poolExhaustion': "Si la réserve s'épuise",
  'rummytiles.rules.match': 'Remporter la partie',
  'rummytiles.rules.tiles': 'Se joue avec {value} tuiles.',
  'rummytiles.rules.dealCount': 'Chaque joueur reçoit {value} tuiles.',
  'rummytiles.rules.group':
    "Un groupe réunit trois ou quatre tuiles d'un même chiffre, chacune d'une couleur différente.",
  'rummytiles.rules.run': "Une suite réunit au moins trois chiffres consécutifs d'une même couleur.",
  'rummytiles.rules.noWrap': 'Le 13 ne reboucle pas sur le 1.',
  'rummytiles.rules.joker': "Un joker représente n'importe quelle tuile.",
  'rummytiles.rules.initialMeldDescription':
    "Tant que tu n'as pas posé {n} points ou plus en un seul tour, depuis ta seule main, tu ne peux toucher à rien de ce qui est déjà sur la table.",
  'rummytiles.rules.turnDescription':
    'Pose au moins une tuile de ta main, réarrange la table librement, et termine avec toutes les combinaisons de la table valides.',
  'rummytiles.rules.noDiscard':
    "Il n'y a pas de défausse — si tu ne peux pas terminer un tour valide, tu pioches une tuile à la place.",
  'rummytiles.rules.jokerTakingDescription':
    "Un joker posé sur la table peut être repris en le remplaçant par la tuile qu'il représente, prise dans ta main — et il doit servir dans une combinaison avant la fin de ton tour.",
  'rummytiles.rules.goingOut':
    'Le premier joueur sans tuiles gagne la manche. Tous les autres marquent la valeur négative de ce qui leur reste ; le gagnant marque la somme de ce que les autres ont perdu.',
  'rummytiles.rules.poolExhaustionLowestWins':
    "Si la réserve s'épuise et que personne ne peut jouer, la manche s'arrête et la main de plus faible valeur l'emporte.",
  'rummytiles.rules.poolExhaustionNoWinner':
    "Si la réserve s'épuise et que personne ne peut jouer, la manche s'arrête sans vainqueur — chaque main est simplement décomptée.",
  'rummytiles.rules.target': "Le premier à dépasser {n} points à la fin d'une manche remporte la partie.",
  'rummytiles.rules.roundLimit': "La partie s'achève après {n} manches — le plus haut score l'emporte.",
  'rummytiles.fact.setCards': '{value}',
  'rummytiles.header.pool': 'Réserve {n}',
  'rummytiles.header.round': 'Manche {n}',
  'rummytiles.round.kind': '{value}',
  'rummytiles.round.round': 'Manche',
  'rummytiles.seat.score': '{value}',
  'rummytiles.seat.notOpened': 'Pas ouvert',
  'rummytiles.status.lastRound': 'Dernière manche : {winner} ({kind})',
  'rummytiles.badge.invalid': 'Pas encore valide',
  'rummytiles.zone.pool': 'Réserve',
  'rummytiles.zone.table': 'Table',
  'rummytiles.zone.tray': 'Chevalet',
  'rummytiles.offer.place': 'Placer',
  'rummytiles.offer.addFromHand': 'Ajouter',
  'rummytiles.offer.addFromTray': 'Ajouter du chevalet',
  'rummytiles.offer.take': 'Prendre',
  'rummytiles.offer.split': 'Couper',
  'rummytiles.offer.swapJoker': 'Échanger le joker',
  'rummytiles.offer.resetTurn': 'Recommencer le tour',
  'rummytiles.offer.commit': 'Terminé',
  'rummytiles.offer.draw': 'Piocher',
  'rummytiles.param.position': 'Couper à',

  // --- blackjack -----------------------------------------------------------
  //
  // The house rules are worded twice over — on and off — because a rule that
  // is *not* in force is still something a player has to be told. "No
  // surrender at this table" is information; silence is a guess.
  'err.BET_BELOW_MINIMUM': "C'est en dessous du minimum de la table",
  'err.ALREADY_BET': 'Ta mise est déjà engagée',
  'err.INSURANCE_CLOSED': "Il n'y a pas d'assurance à prendre en ce moment",
  'err.CANNOT_DOUBLE': 'Cette main ne peut pas être doublée',
  'err.CANNOT_SPLIT': 'Cette main ne peut pas être séparée',
  'err.CANNOT_SURRENDER': 'Cette main ne peut pas être abandonnée',

  'blackjack.rules.section.table': 'La table',
  'blackjack.rules.section.play': 'Jouer une main',
  'blackjack.rules.section.dealer': 'Le croupier',
  'blackjack.rules.section.end': 'Fin de la partie',
  'blackjack.rules.goal':
    'Bats le croupier sans dépasser vingt et un. Dépasser fait perdre aussitôt, quoi que fasse le croupier ensuite.',
  // Counts are their own phrase rather than a number glued to a noun: one
  // deck and six decks inflect differently, and Czech inflects them again.
  'blackjack.rules.decks': 'Jeux dans le sabot : {n}.',
  'blackjack.rules.stack': "Chaque siège s'installe avec {n} jetons.",
  'blackjack.rules.minBet': 'Le minimum de la table est de {n} jetons.',
  'blackjack.rules.faceUp':
    "Les cartes des joueurs sont données face visible ; le croupier garde une carte cachée jusqu'à ce que tout le monde ait joué.",
  'blackjack.rules.hitStand': 'Tire autant de cartes que tu veux, ou reste sur ce que tu as.',
  'blackjack.rules.aces': 'Un as vaut onze tant que cela passe, et un quand cela ne passe plus.',
  'blackjack.rules.blackjack':
    "Un as accompagné d'une carte à dix points, sur les deux premières cartes, fait un blackjack.",
  'blackjack.rules.pays3to2': 'Un blackjack paie 3:2.',
  'blackjack.rules.pays6to5': 'Un blackjack paie 6:5.',
  'blackjack.rules.paysEven': 'Un blackjack paie à égalité.',
  'blackjack.rules.double':
    'Sur tes deux premières cartes, tu peux doubler ta mise et prendre exactement une carte de plus.',
  'blackjack.rules.doubleAfterSplit': "Une main issue d'une séparation peut aussi être doublée.",
  'blackjack.rules.noDoubleAfterSplit': "Une main issue d'une séparation ne peut pas être doublée.",
  'blackjack.rules.split':
    "Deux cartes de même valeur peuvent être séparées en mains distinctes, chacune avec sa mise — jusqu'à {n} fois, pour {hands} mains au total.",
  'blackjack.rules.noSplit': 'Les paires ne se séparent pas à cette table.',
  'blackjack.rules.splitAces':
    "Des as séparés reçoivent une carte chacun puis restent, et vingt et un obtenu ainsi n'est pas un blackjack.",
  'blackjack.rules.surrender':
    "Tu peux abandonner ta première main contre la moitié de sa mise, une fois que le croupier a vérifié s'il avait blackjack.",
  'blackjack.rules.noSurrender': 'On ne peut pas abandonner une main à cette table.',
  'blackjack.rules.dealerDraws': "Le croupier tire jusqu'à dix-sept, puis reste.",
  'blackjack.rules.hitsSoft17': 'Le croupier tire sur un dix-sept comptant un as.',
  'blackjack.rules.standsSoft17': 'Le croupier reste sur un dix-sept comptant un as.',
  'blackjack.rules.dealerPeeks':
    "S'il montre un as ou une carte à dix, le croupier vérifie s'il a blackjack avant que quiconque joue.",
  'blackjack.rules.insurance':
    "Face à un as du croupier, tu peux t'assurer pour la moitié de ta mise ; cela paie 2:1 si le croupier a blackjack.",
  'blackjack.rules.noInsurance': "L'assurance n'est pas proposée à cette table.",
  'blackjack.rules.rounds': 'La table joue {n} manches.',
  'blackjack.rules.mostChipsWins': 'Celui qui détient le plus de jetons à la fin remporte la partie.',
  'blackjack.rules.bustedOut':
    "Un siège qui ne peut plus couvrir le minimum de {n} reste hors jeu jusqu'à la fin de la partie.",

  'blackjack.zone.dealer': 'Croupier',
  'blackjack.zone.box': 'Main',
  'blackjack.zone.yourBox': 'Ta main',
  'blackjack.zone.shoe': 'Sabot',

  'blackjack.header.round': 'Manche {n} sur {of}',
  'blackjack.header.minBet': 'Minimum',
  'blackjack.header.decks': 'Jeux',
  'blackjack.header.dealerTotal': 'Le croupier montre {n}',
  'blackjack.header.dealerSoftTotal': 'Le croupier montre {n} souple',

  'blackjack.seat.stack': 'Jetons',
  'blackjack.seat.bet': 'Mise',
  'blackjack.seat.insurance': 'Assurance',
  'blackjack.seat.total': 'Total',
  'blackjack.seat.softTotal': 'Total souple',
  'blackjack.seat.out': 'Plus de jetons',

  'blackjack.prompt.placeBet': 'Place ta mise',
  'blackjack.prompt.insurance': 'Assurance ?',
  'blackjack.prompt.yourMove': 'À toi de jouer',
  'blackjack.prompt.waitingFor': 'En attente de {playerId}',
  'blackjack.prompt.betAmount': 'Mise',

  'blackjack.quick.doubleMin': '2× Minimum',
  'blackjack.quick.allIn': 'All-in',
  'blackjack.offer.bet': 'Miser',
  'blackjack.offer.hit': 'Carte',
  'blackjack.offer.stand': 'Rester',
  'blackjack.offer.double': 'Doubler',
  'blackjack.offer.split': 'Séparer',
  'blackjack.offer.surrender': 'Abandonner',
  'blackjack.offer.insure': "Prendre l'assurance",
  'blackjack.offer.declineInsurance': "Pas d'assurance",

  'blackjack.fact.tableMinimum': 'minimum',
  'blackjack.fact.insuranceCost': 'pour assurer',
  'blackjack.fact.extraStake': 'à miser',
  'blackjack.fact.surrenderReturn': 'rendus',

  'blackjack.status.dealerBlackjack': 'Le croupier avait blackjack',
  'blackjack.status.dealerBust': 'Le croupier a sauté à {n}',
  'blackjack.status.dealerStands': 'Le croupier reste à {n}',

  'blackjack.round.name': 'Manche',
  'blackjack.round.dealerTotal': 'Croupier {n}',
  'blackjack.round.dealerBust': 'Croupier sauté ({n})',
  'blackjack.round.dealerBlackjack': 'Blackjack du croupier',
  'blackjack.round.outcome.blackjack': 'Blackjack',
  'blackjack.round.outcome.win': 'Gagné',
  'blackjack.round.outcome.push': 'Égalité',
  'blackjack.round.outcome.lose': 'Perdu',
  'blackjack.round.outcome.bust': 'Sauté',
  'blackjack.round.outcome.surrender': 'Abandonné',

  'blackjack.badge.inPlay': 'En jeu',
  'blackjack.badge.doubled': 'Doublé',
  'blackjack.badge.split': 'Séparé',
  'blackjack.badge.blackjack': 'Blackjack',
  'blackjack.badge.bust': 'Sauté',
  'blackjack.badge.won': 'Gagné',
  'blackjack.badge.push': 'Égalité',
  'blackjack.badge.lost': 'Perdu',
  'blackjack.badge.surrendered': 'Abandonné',

  'blackjack.unit.chips': 'jetons',

  // --- the settings screen --------------------------------------------------
  //
  // Hardcoded English until now, which was survivable while the only other
  // language was unreachable and faintly absurd once the language picker
  // itself lived on this screen: a player who cannot read "Settings" is
  // exactly the one who came here to change it.
  'settings.title': 'Réglages',
  'settings.signedInAs': 'Connecté en tant que {username}',
  'settings.playingAsGuest': 'Tu joues en tant que {username} (invité)',
  'settings.notSignedIn': "Non connecté — connecte-toi ou continue en tant qu'invité pour jouer en ligne.",
  'settings.subtitle': "De quoi tu as l'air, et de quoi la table a l'air",
  'settings.face.heading': 'Ton visage à la table',
  'settings.face.account': 'Conservé avec ton compte, il te suit sur un autre appareil.',
  'settings.face.device': "Conservé sur cet appareil. Connecte-toi pour l'emporter avec toi.",
  'settings.skin.heading': 'Aspect de la table',
  'settings.language.heading': 'Langue',
  'settings.language.status': 'Conservé sur cet appareil.',
  'settings.language.auto': 'Automatique',
  'settings.language.auto.now': 'Suit ton appareil — actuellement {language}',
  'settings.legal.heading': 'Les petits caractères',
  'settings.legal.status': 'Ce que tu acceptes en jouant, et ce qui est conservé à ton sujet.',
  'settings.signIn': 'Se connecter',
  'settings.back': 'Retour',

  // A translated interface over an English notice is a worse lie than an
  // English interface, so the document says so itself rather than letting the
  // surrounding screen imply the text was written for this reader.
  'legal.untranslated':
    "Cet avis n'a pas encore été traduit dans ta langue. Le texte anglais ci-dessous est la version qui fait foi.",

  // --- the navigation bar's screen titles ------------------------------------
  //
  // These sit in `app/_layout.tsx`, above every screen, and were the last
  // English left after the settings screen was translated — a German player
  // reading "Einstellungen" under a bar that said "Settings". A translated app
  // with an untranslated chrome bar looks like a bug, not like a limit.
  'nav.home': 'Zolik',
  'nav.emailSignIn': 'Connexion par e-mail',
  'nav.signingIn': 'Connexion en cours',
  'nav.usernameSignIn': 'Connexion par identifiant',
  'nav.legacyAccount': 'Ancien compte',
  'nav.guest': 'Invité',
  'nav.account': 'Compte',
  'nav.games': 'Jeux',
  'nav.table': 'Ta table',
  'nav.join': 'Rejoindre une table',
  // The screen a shared invite link lands on while it resolves the code.
  'nav.joining': 'Connexion à la table',
  'nav.rules': 'Règles',
  'nav.match': 'Partie',
  'nav.scoreTable': 'Tableau des scores',
  'nav.stats': 'Statistiques',
  'nav.more': 'Plus',

  // --- the account menu behind the face in the corner ----------------------
  'menu.label': 'Menu du compte',
  'menu.signedIn': 'Connecté',
  'menu.notSignedIn': 'Non connecté',
  'menu.keepStats': 'pour garder tes statistiques',
  'menu.signOut': 'Se déconnecter',
  'more.scoreTable': 'Feuille de score hors ligne',
  'more.stats': 'Statistiques et classement',
  'more.needsAccount': 'connecte-toi pour utiliser',
  'gate.title': 'Connecte-toi pour utiliser ceci',
  'gate.body':
    "Les feuilles de score et les statistiques sont conservées avec ton compte, elles te suivent donc sur un autre appareil. Un invité n'a nulle part où les garder.",

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
  'error.generic': "Ça n'a pas marché",
  'error.signIn': 'Échec de la connexion',
  'error.login': 'Échec de la connexion',
  'error.register': "Échec de l'inscription",
  'error.sendCode': "Impossible d'envoyer un code",
  'error.badCode': "Ce code n'a pas fonctionné",
  'error.rulesLoad': 'Impossible de charger les règles',
  'error.createFailed': 'Échec de la création',
  'error.saveFailed': "Échec de l'enregistrement",
  'error.exportFailed': "Échec de l'export",

  // --- the route that does not exist ----------------------------------------
  'notFound.title': 'Oups !',
  'notFound.message': "Cet écran n'existe pas.",
  'notFound.home': "Aller à l'accueil !",

  // --- signing in -----------------------------------------------------------
  'auth.login.subtitle': 'Garde tes statistiques sur tous tes appareils',
  'auth.login.continueWithEmail': "Continuer avec l'e-mail",
  'auth.login.usernameInstead': 'Se connecter avec un identifiant à la place',
  'auth.email.title': 'Connexion par e-mail',
  'auth.email.subtitle': "Nous t'enverrons un code à usage unique",
  'auth.email.address': 'Adresse e-mail',
  'auth.email.send': 'Envoyer le code',
  'auth.email.codeTitle': 'Saisis le code',
  'auth.email.codePlaceholder': 'Code à 6 chiffres',
  'auth.email.differentAddress': 'Utiliser une autre adresse',
  'auth.email.sentTo': 'Envoyé à {email}',
  'auth.email.continue': 'Continuer',
  'auth.guest.title': 'Jouer en invité',
  'auth.guest.subtitle': 'Aucun compte requis',
  'auth.guest.displayName': 'Nom affiché',
  'auth.register.title': 'Créer un compte',
  'auth.register.username': 'Identifiant',
  'auth.register.email': 'E-mail (facultatif)',
  'auth.register.password': 'Mot de passe',
  'auth.username.createAccount': 'Créer un compte avec identifiant et mot de passe',
  'auth.callback.signedIn': 'Connecté.',

  // --- the account screen ---------------------------------------------------
  'account.signInPrompt': 'Connecte-toi pour gérer ton compte.',
  'account.keepGames': 'Garder ces parties',
  'account.signedInWith': 'Connecté avec',
  'account.addMethod': 'Ajouter une méthode de connexion',
  'account.usernameAndPassword': 'Identifiant et mot de passe',
  'account.faceAndTable': 'Visage et aspect de la table',
  'account.refresh': 'Actualiser',
  'account.remove': 'Retirer',

  // --- the main menu --------------------------------------------------------
  'home.subtitle': 'Rami continental · {server}',
  'home.playingAs': 'Tu joues en tant que {name}',
  'home.signInPrompt': 'Connecte-toi ou continue en invité pour jouer en ligne.',
  'home.statsAndLeaderboard': 'Statistiques et classement',
  'home.play': 'Jouer',
  'home.offlineScoreTable': 'Tableau des scores hors ligne',
  'home.signInToKeepStats': 'Se connecter pour garder ses statistiques',
  'home.signOut': 'Se déconnecter',
  'home.continueAsGuest': 'Continuer en invité',
  // Someone playing as a guest, marked in a list of names.
  'home.guestSuffix': '(invité)',

  // --- the waiting room on the main menu ------------------------------------
  //
  // Counts are whole phrases per count, not a number glued to a noun — the
  // same reason `countLabel` exists. One and many are the cases the interface
  // actually produces, so they are the cases that get their own wording.
  'waiting.checking': 'On regarde qui est là…',
  'waiting.youAreWaiting': 'Tu attends de jouer',
  'waiting.pickedUp': "Quiconque ouvre une table peut te prendre — personne n'a besoin d'un code de ta part.",
  'waiting.othersOne': '1 autre joueur attend aussi',
  'waiting.othersMany': '{n} autres joueurs attendent aussi',
  'waiting.oneWaiting': '1 joueur attend de jouer',
  'waiting.manyWaiting': '{n} joueurs attendent de jouer',
  'waiting.adding': "On t'ajoute à la liste d'attente…",
  'waiting.slowHint':
    "Si cela ne se termine pas en quelques secondes, vérifie que l'adresse du serveur ci-dessous est joignable depuis cet appareil.",
  'waiting.serverBusyDetail':
    "Tentative {n}. Le serveur n'accepte pas de nouvelles connexions à la salle d'attente pour l'instant.",
  'waiting.reconnecting': 'Connexion perdue — reconnexion…',
  'waiting.reconnectingDetail':
    'Tentative {n}. Cela peut arriver si le réseau de ton appareil a changé, ou si le serveur a redémarré.',
  'waiting.tryAgain': 'Réessayer maintenant',
  'waiting.makeAvailable': 'Me rendre disponible pour jouer',
  'waiting.stop': "Arrêter d'attendre",
  'waiting.noneYet':
    "Personne n'attend de jouer pour l'instant. Inscris-toi sur la liste et tu seras le premier que l'on verra.",
  'waiting.noOthersYet':
    "Personne d'autre n'attend encore. Les hôtes te voient quand même et peuvent t'inviter.",
  'waiting.server': 'Serveur',
  'waiting.none':
    "Personne n'attend pour l'instant. Quiconque se rend disponible depuis le menu principal apparaît ici.",

  // --- landing on a shared invite link --------------------------------------
  'join.missingCode': 'Il manque à ce lien le code de la table.',
  'join.staleLink':
    "Demande un nouveau lien à la personne qui t'a invité, ou rejoins avec le code à la place.",
  'join.enterCode': 'Saisir un code',
  'join.backToMenu': 'Retour au menu',
  'join.takingSeat': 'On te place…',
  'join.takingSeatAt': 'On te place à {game}…',

  // --- the lobby ------------------------------------------------------------
  'lobby.games.subtitle': 'Tout ce que ce serveur peut héberger',
  'lobby.games.bots': 'Bots',
  'lobby.games.playBot': 'Jouer contre un bot',
  'lobby.games.playBots': 'Jouer contre {n} bots',
  'lobby.games.openTable': 'Ouvrir une table',
  'lobby.games.players': '{n} joueurs',
  'lobby.games.playerRange': '{min} à {max} joueurs',
  'lobby.join.placeholder': "Code ou lien d'invitation",
  'lobby.join.needCode': 'Saisis un code, un lien ou un identifiant de partie',
  'lobby.games.signInFirst': "Connecte-toi d'abord",
  'lobby.join.action': 'Rejoindre',
  'lobby.join.waitingTitle': "En attente de l'hôte",
  // Two whole sentences rather than one with a swapped noun: "a game of {game}"
  // does not survive a language that inflects the game's name after "of".
  'lobby.join.joinedGame': 'Tu as rejoint une partie de {game} — en attente du départ',
  'lobby.join.joinedTable': 'Tu as rejoint la table — en attente du départ',
  'lobby.table.addBot': 'Ajouter un bot',
  'lobby.table.side': 'Camp {n}',
  'lobby.table.shuffleSeats': 'Mélanger les places',
  'lobby.table.moveSeatUp': 'Monter {name} d’une place',
  'lobby.table.moveSeatDown': 'Descendre {name} d’une place',
  'lobby.table.start': 'Démarrer',
  'lobby.table.waitingForHost': "En attente du départ donné par l'hôte…",

  // --- inviting someone to a table ------------------------------------------
  'invite.heading': 'Inviter des joueurs',
  'invite.explain': "Envoie ce lien. Celui qui l'ouvre arrive à cette table — sans compte.",
  'invite.noAddress':
    "Aucune adresse partageable n'est configurée pour ce serveur, utilise donc le code ci-dessous.",
  'invite.readOutCode': 'Ou dicte le code :',
  'invite.copy': 'Copier le lien',
  'invite.share': 'Partager le lien',
  'invite.copied': 'Copié !',
  'invite.shared': 'Partagé',

  // --- the match screen -----------------------------------------------------
  'match.waitingForTable': 'En attente de la table…',
  'match.waitingForPlayer': "En attente d'un autre joueur…",
  'match.nobodyWon': "Personne n'a gagné.",
  'match.youWon': 'Tu as gagné.',
  'match.finished': 'Cette partie est terminée.',
  'match.inProgress': 'Partie en cours — tout est connecté et fonctionne normalement.',
  'match.connecting': 'Connexion…',
  'match.abandonedTitle': 'Table mise de côté',
  'match.abandoned': 'Personne n’est revenu à cette table, elle a donc été mise de côté. Les cartes sont exactement là où tu les as laissées.',
  'match.resume': 'Reprendre où tu t’es arrêté',
  'match.resuming': 'Reprise de la table…',
  'match.controls': 'Commandes',
  'match.over': 'Partie terminée',
  'match.settingUp': 'Préparation…',
  'match.playAgain': 'Rejouer',
  'match.backToGames': 'Retour aux jeux',
  'match.table': 'Table',
  'match.opponents': 'Adversaires',
  // Marks which seat is the reader's own, in a list of seats.
  'match.youSuffix': '(toi)',
  // The winner line is composed from whole sentences rather than a name glued
  // to " won." — the verb agrees with the subject in most of these languages,
  // and a suffix cannot know that.
  'match.you': 'toi',
  'match.someoneWon': '{name} a gagné.',
  'match.wonBy': 'Gagné par {names}.',
  'match.pausedFor': 'En pause — en attente de la reconnexion de {name}.',
  'match.results': 'Résultats',
  'match.players': 'Joueurs',
  'match.toPlay': 'à jouer',

  // --- the offline score table ----------------------------------------------
  'scoring.namesHint': 'Noms séparés par des virgules (4 à 8 joueurs)',
  'scoring.newSession': 'Nouvelle session',
  // A worked example, not a sentence: the names are placeholders a translator
  // may localise, the shape `name:score,` is what the parser needs.
  'scoring.scoresPlaceholder': 'Alice:120,Bruno:80,…',
  'scoring.saveRound': 'Enregistrer la manche',
  'scoring.export': 'Exporter la feuille de score',
  'scoring.formatHint': 'Format des scores : Nom:100,Nom2:50',
  'scoring.nameCountError': 'Saisis de 2 à 8 noms de joueurs séparés par des virgules',
  'scoring.session': 'Session : {id}',
  'scoring.players': 'Joueurs : {names}',
  'scoring.roundScores': 'Scores de la manche {n}',
  'stats.loading': 'Chargement…',
  // The figures could not be fetched. `{reason}` is whatever the server or the
  // network said, which is not ours to translate — the frame around it is.
  'stats.unavailable': '(indisponible : {reason})',

  // --- statistics -----------------------------------------------------------
  'stats.title': 'Statistiques et classement',
  'stats.yours': 'Tes statistiques',
  'stats.leaderboard': 'Classement',

  // --- a player's lifetime record, shown beside a finished match ------------
  'record.title': 'Ton bilan',
  'record.guest':
    "Tu joues en invité, donc aucun bilan n'est conservé. Connecte-toi et les parties que tu as déjà jouées sur cet appareil — celle-ci comprise — seront rattachées à ton compte.",
  'record.signInToKeep': 'Se connecter et les garder',
  'record.failed': "Ton bilan n'a pas pu être chargé pour l'instant. La partie est bien enregistrée.",
  'record.loading': 'Chargement…',
  'record.played': 'Jouées',
  'record.won': 'Gagnées',
  'record.lost': 'Perdues',
  'record.winRate': 'Taux de victoire',
  'record.streak': 'Série',
  'record.atThisGame': 'À ce jeu',
  // A streak is a count with a noun, so it is a whole phrase per count for
  // the same reason `countLabel` is: "1 win" / "2 wins" is an English rule,
  // and most of these languages do not share it.
  'record.streakNone': '—',
  'record.streakWinOne': '1 victoire',
  'record.streakWinMany': '{n} victoires',
  'record.streakLossOne': '1 défaite',
  'record.streakLossMany': '{n} défaites',

  // --- moving cards ---------------------------------------------------------
  'hand.dragHint':
    "Fais glisser une carte le long de l'éventail pour la déplacer, ou sur le tableau pour la jouer",
  'hand.moveLeft': 'Vers la gauche',
  'hand.moveRight': 'Vers la droite',
  'zone.collapseGroup': 'Replier ce groupe',
  'zone.expandGroup': 'Afficher toutes les cartes de ce groupe',
  'zone.dropHere': 'Déposer ici',
  'offer.pickCards': "choisis des cartes pour l'endroit que tu as touché",
  'offer.ambiguous': 'cela peut aller à plusieurs endroits — choisis sur le tableau',

  // --- the build footer -----------------------------------------------------
  'build.app': 'app',
  'build.server': 'serveur',
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
  'option.pauseBetweenRounds': 'Pause entre les manches',
  'choice.pauseBetweenRounds.1': 'Pause',
  'choice.pauseBetweenRounds.0': 'Enchaîner directement',
  'option.botSkill': 'Adversaires',
  'choice.botSkill.0': 'Mélangés',
  'choice.botSkill.1': 'Facile',
  'choice.botSkill.2': 'Moyen',
  'choice.botSkill.3': 'Difficile',
  'option.initialMeldMinimum': "Valeur d'ouverture",
  'choice.initialMeldMinimum.0': 'Aucune',
  'option.discardDrawMinRound': 'Prise dans la défausse',
  'choice.discardDrawMinRound.0': 'Ouverte',
  'choice.discardDrawMinRound.2': 'Dès la manche 2',
  'choice.discardDrawMinRound.3': 'Dès la manche 3',
  'option.requireCleanRun': 'Suite sans joker',
  'choice.requireCleanRun.1': 'Exigée',
  'choice.requireCleanRun.0': 'Non',
  'option.jokerReclaimMustPlay': 'Joker racheté',
  'choice.jokerReclaimMustPlay.1': 'À jouer le même tour',
  'choice.jokerReclaimMustPlay.0': 'Peut être gardé',
  'option.dealStarter': 'Entame',
  'choice.dealStarter.0': 'À tour de rôle',
  'choice.dealStarter.1': 'Le vainqueur entame',
  'variation.prsi.classic': 'Classique',
  'option.handSize': 'Cartes distribuées',
  'variation.canasta.classic': 'Classique',
  'variation.canasta.modern_american': 'Modern American',
  'variation.canasta.samba': 'Samba',
  'option.targetScore': 'Score cible',
  'option.canastasToGoOut': 'Canastas pour sortir',
  'variation.holdem.freezeout': 'Freezeout',
  'variation.holdem.timed': 'Nombre de mains fixe',
  'option.startingStack': 'Tapis de départ',
  'option.bigBlind': 'Grosse blinde',
  'option.handLimit': 'Mains',
  'choice.handLimit.0': "Jusqu'à ce qu'il ne reste qu'un siège",
  'variation.ginrummy.standard': 'Standard',
  'option.knockLimit': 'Limite pour frapper',
  'choice.knockLimit.0': 'Oklahoma (la carte retournée la fixe)',
  'option.bigGin': 'Big gin',
  'choice.bigGin.0': 'Non',
  'choice.bigGin.1': 'Oui (+25)',
  'option.lineBonuses': 'Bonus de décompte',
  'choice.lineBonuses.1': 'Oui',
  'choice.lineBonuses.0': 'Non',
  'variation.rummytiles.standard': 'Standard',
  'choice.targetScore.0': 'Aucun',
  // Two games qualify a number the others mean plainly: Canasta's 500 is a
  // short game, Gin Rummy's and Rummy Tiles' 500 is just 500; Hold'em's
  // starting 200 is a short stack, Blackjack's 200 is just 200. Module-scoped
  // so the qualifier travels with the game that means it — a bare key would
  // hand the note to games it is false for. `scripts/check-server-labels.js`
  // is what finds these; both were shipping as English on a Czech screen.
  'choice.canasta.targetScore.500': '500 (courte)',
  'choice.holdem.startingStack.200': '200 (courte)',
  'option.roundLimit': 'Limite de manches',
  'choice.roundLimit.0': 'Aucune',
  'option.poolExhaustion': 'Réserve épuisée',
  'choice.poolExhaustion.1': 'La main la plus faible gagne la manche',
  'choice.poolExhaustion.0': 'Personne ne gagne la manche',
  'variation.blackjack.single': 'Un seul jeu',
  'option.minBet': 'Minimum de la table',
  'option.rounds': 'Manches',
  'option.decks': 'Jeux',
  'option.dealerHitsSoft17': 'Croupier sur 17 souple',
  'choice.dealerHitsSoft17.0': 'Reste',
  'choice.dealerHitsSoft17.1': 'Tire',
  'option.blackjackPays': 'Le blackjack paie',
  'choice.blackjackPays.100': 'À égalité',
  'option.maxSplits': 'Séparation',
  'choice.maxSplits.0': 'Pas de séparation',
  'choice.maxSplits.1': 'Une fois (deux mains)',
  'choice.maxSplits.3': 'Trois fois (quatre mains)',
  'option.doubleAfterSplit': 'Doubler après séparation',
  'choice.doubleAfterSplit.1': 'Autorisé',
  'choice.doubleAfterSplit.0': 'Interdit',
  'option.surrender': 'Abandon',
  'choice.surrender.0': 'Non',
  'choice.surrender.1': 'Abandon tardif',
  'option.insurance': 'Assurance',
  'choice.insurance.1': 'Proposée',
  'choice.insurance.0': 'Non proposée',

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
  'verb.add': 'Ajouter',
  'verb.bet': 'Miser',
  'verb.call': 'Suivre',
  'verb.check': 'Parole',
  'verb.commit': 'Terminé',
  'verb.continue': 'Continuer',
  'verb.decline_insurance': "Pas d'assurance",
  'verb.discard': 'Défausser',
  'verb.double': 'Doubler',
  'verb.draw': 'Piocher',
  'verb.finish_layoff': 'Report terminé',
  'verb.fold': 'Se coucher',
  'verb.hit': 'Carte',
  'verb.insure': "Prendre l'assurance",
  'verb.knock': 'Frapper',
  'verb.lay_meld': 'Poser',
  'verb.lay_off': 'Reporter',
  'verb.pass': 'Passer',
  'verb.place': 'Placer',
  'verb.play_card': 'Jouer',
  'verb.raise': 'Relancer',
  'verb.reset_turn': 'Recommencer le tour',
  'verb.split': 'Séparer',
  'verb.stand': 'Rester',
  'verb.surrender': 'Abandonner',
  'verb.swap_joker': 'Échanger le joker',
  'verb.take': 'Prendre',
  'verb.take_pile': 'Prendre dans la défausse',
  // Declared by the server and listed in `serverKeys.json`, but never worded.
  'verb.takePileFromHand': 'Prendre la pile en main',
  'verb.takePileOntoMeld': 'Prendre la pile sur une combinaison',
  'verb.takeTopForSequence': 'Prendre la carte du dessus sur une séquence',
  'verb.undoDraw': 'Annuler la pioche',
  'verb.undoLayOff': 'Annuler le report',
  'verb.undoMeld': 'Annuler la combinaison',
  'verb.undoTakePile': 'Annuler la prise de la défausse',
  'verb.undoTurn': 'Annuler le tour',

  // --- suits, spelled out ---------------------------------------------------
  //
  // Read aloud by a screen reader and shown where a pip would not fit. The
  // pips themselves are drawn, not written, so these are the only place the
  // suit is ever a word.
  'suit.C': 'Trèfles',
  'suit.D': 'Carreaux',
  'suit.H': 'Cœurs',
  'suit.S': 'Piques',

  // --- counters the board prints beside a number ----------------------------
  'canasta.seat.notOpened': 'Pas ouvert',
  'canasta.unit.points': 'points',
  'ginrummy.unit.points': 'points',
  'holdem.seat.dealer': 'Donneur',
  'holdem.seat.folded': 'Couché',
  'holdem.seat.allIn': 'Tapis',
  'holdem.seat.out': 'Éliminé',
  'holdem.unit.chips': 'jetons',
  'prsi.unit.cardsLeft': 'cartes restantes',
  'rummytiles.prompt.initialMeld': 'Ta première pose doit valoir {n} points.',
  'rummytiles.unit.points': 'points',
  'zolik.unit.penalty': 'pénalité',
  'header.pileFrozen': 'Pile gelée',
  // The module-less form of `ginrummy.prompt.yourTurnDraw`: Žolíky and Canasta
  // send the prompt without a module prefix. Neither the Go constants nor
  // `serverKeys.json` list it — the sweep is what found it.
  'prompt.yourTurnDraw': 'Pioche une carte',
  'prompt.yourTurnMeld': 'Combine si tu peux, puis défausse',
};
