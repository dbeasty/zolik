/**
 * Irish. Rummy vocabulary: tacar for a set, sraith for a run, cumasc for a meld, áilteoir for a joker, stoc and carn caite for the two piles.
 */

export const ga: Record<string, string> = {
  // --- engine error codes (rules.RulesErrorCode) ---------------------------
  'err.NOT_YOUR_TURN': 'Ní hé do sheal é',
  'err.WRONG_PHASE': 'Níl sé sin ar fáil faoi láthair',
  'err.MUST_DRAW_FIRST': 'Tarraing cárta sula leagann tú síos',
  'err.GAME_SUSPENDED': 'Tá an cluiche ar sos',
  'err.GAME_NOT_ACTIVE': 'Níl an cluiche ar siúl',
  // --- runtime refusals, which are not the rules saying no -----------------
  //
  // Both were reachable long before they were worded, and a player who met
  // one was shown the code in capitals or, worse, nothing at all: an action
  // sent while the socket was down simply vanished, which reads as a legal
  // move being silently refused.
  'err.MATCH_NOT_ACTIVE': 'Tá an bord ar sos — ag fanacht le himreoir ceangal a dhéanamh arís',
  'err.NOT_CONNECTED': 'Gan cheangal leis an mbord — ag athcheangal, bain triail eile as ansin',
  // Why the "start the next round" control is greyed out. Without wording the
  // shell printed the code itself, so a player agreeing to go on was answered
  // with ALREADY_READY in capitals.
  'err.ALREADY_READY': 'Tá tú réidh',
  'err.NOT_BETWEEN_ROUNDS': 'Tá an babhta fós á imirt',
  'err.NOT_AT_THIS_TABLE': 'Níl tú ag an mbord seo',
  'err.DISCARD_LOCKED': 'Tá an carn caite faoi ghlas go fóill',
  'err.DISCARD_PILE_EMPTY': 'Tá an carn caite folamh',
  'err.NO_CARDS_LEFT': 'Níl cárta ar bith fágtha le tarraingt',
  'err.ROUND_REQ_NOT_MET': 'Leag síos do chéad chumasc féin ar dtús',
  'err.NEED_CLEAN_RUN': 'Teastaíonn sraith gan áilteoir ar an mbord uait sula gcomhairtear leagtha síos thú',
  'err.INCOMPLETE_INITIAL_MELD': 'Críochnaigh do leagan síos, nó cealaigh é, sula gcaitheann tú cárta',
  'err.DISCARD_CARD_NOT_MELDED': 'Caithfidh an cárta a phioc tú dul isteach i do chumasc',
  'err.JOKER_DISCARD_FORBIDDEN': 'Ní féidir áilteoir a chaitheamh',
  'err.NOTHING_TO_UNDO': 'Níl aon rud le cealú',
  'err.NO_JOKER_IN_MELD': 'Níl áilteoir sa chumasc seo',
  'err.JOKER_SWAP_MISMATCH': 'Ní ghlacann an cárta sin áit an áilteora',
  'err.RECLAIMED_JOKER_NOT_MELDED':
    'Caithfear an t-áilteoir a tógadh den bhord a imirt i gcumasc sa seal seo',
  'err.RUN_TOO_LONG': 'Tá an tsraith sin ag a lánfhad cheana',
  'err.WRONG_RUN_END': 'Cuireann an cárta sin leis an gceann eile den tsraith',
  'err.INVALID_MELD': 'Níl cárta ar bith i do láimh a oireann anseo',
  'err.CARD_NOT_IN_HAND': 'Níl an cárta sin i do láimh',
  'err.MELD_BELOW_MINIMUM': 'Tá do chumaisc gann ar phointí fós chun dul síos',
  'err.MELD_NO_CONTRIBUTION': 'Ní chuireann an cumasc sin do riachtanas chun cinn',
  'err.TOO_MANY_WILDS': 'An iomarca áilteoirí sa chumasc sin',
  'err.ADJACENT_WILDS': 'Ní féidir le dhá áilteoir a bheith taobh le taobh',
  'err.ACE_BRIDGE': 'Ní féidir le hAonach an Rí agus an Dó a nascadh',

  // --- contract phrasing ---------------------------------------------------
  // Built from the structured contract the server sends (sets/runs counts),
  // never from a deal-number lookup table.
  // Whole phrases per count rather than a number plus a pluralised noun:
  // Czech inflects the noun by count (skupina / skupiny / skupin) in a way no
  // "add an s" helper survives, and the counts that actually occur are 1-3.
  // A count outside that falls back to contract.sets.n.
  'contract.sets.1': 'Tacar amháin',
  'contract.sets.2': 'Dhá thacar',
  'contract.sets.3': 'Trí thacar',
  'contract.sets.n': 'Tacair: {n}',
  'contract.runs.1': 'Sraith amháin',
  'contract.runs.2': 'Dhá shraith',
  'contract.runs.3': 'Trí shraith',
  'contract.runs.n': 'Sraitheanna: {n}',
  'contract.any': 'Cumasc bailí ar bith',
  'contract.cleanRunOnly':
    'Meascán ar bith de thacair agus de shraitheanna — caithfidh sraith amháin ar a laghad a bheith gan áilteoir',
  'contract.cleanRunSuffix': '{base} — caithfidh sraith amháin a bheith gan áilteoir',

  // --- written rules ---------------------------------------------------------
  // Full-sentence keys a "see the rules" screen renders, one per module,
  // resolved server-side against the table's actual variation and options —
  // see internal/module/rules.go and each game's rules.go. Shared section
  // titles first, then one block per module in the order the four games were
  // added.
  'zolik.rules.section.goal': 'Aidhm',
  'zolik.rules.section.setup': 'Socrú',
  'zolik.rules.section.turn': 'Do sheal',
  'zolik.rules.section.melding': 'Leagan síos',
  'zolik.rules.section.end': 'Conas a chríochnaíonn an cluiche',
  'zolik.rules.goal':
    'Bí ar an gcéad duine a fholmhaíonn a lámh trí thacair agus sraitheanna bailí a leagan síos, agus tú ag bailiú a laghad pointí pionóis agus is féidir sna cártaí atá fós agat nuair a théann duine eile amach.',
  'zolik.rules.deal': 'Faigheann gach imreoir {n} chárta.',
  'zolik.rules.meldShapes':
    'Is é atá i dtacar ná {set}+ chárta den luach céanna; is é atá i sraith ná {run}+ chárta as a chéile den dath céanna.',
  'zolik.rules.turn.draw': 'Ar do sheal, tarraing cárta amháin — ón stoc nó ón gcarn caite.',
  'zolik.rules.pickup.topOnly': 'Ní féidir ach an cárta uachtarach a thógáil ón gcarn caite.',
  'zolik.rules.pickup.anyFromPile':
    'Is féidir cárta ar bith a thógáil ón gcarn caite, mar aon le gach a bhfuil os a chionn.',
  'zolik.rules.pickup.locked': 'Ní féidir tarraingt ón gcarn caite roimh bhabhta {n}.',
  'zolik.rules.pickup.open': 'Tá an carn caite ar oscailt ón gcéad bhabhta.',
  'zolik.rules.turn.discard': 'Cuir deireadh le do sheal trí chárta amháin a chaitheamh.',
  'zolik.rules.jokers.restricted':
    'Ní féidir áilteoir a chaitheamh riamh, ach amháin más é go díreach an cárta a fholmhaíonn do lámh é.',
  'zolik.rules.lead.rotate': 'Bogann an chéad imirt suíochán amháin gach dáileadh, is cuma cé a bhuaigh.',
  'zolik.rules.lead.winner': 'Tosaíonn an té a théann amach an chéad dáileadh eile.',
  'zolik.rules.meldFloor.on':
    'Caithfidh do chéad leagan síos {n} phointe nádúrtha ar a laghad a bhaint amach sula mbeidh tú síos.',
  'zolik.rules.meldFloor.off': 'Níl aon íosluach pointí ar do chéad leagan síos.',
  'zolik.rules.cleanRun.on':
    'Caithfidh ceann amháin ar a laghad de do shraitheanna a bheith go hiomlán gan áilteoir sula gcomhairtear síos thú.',
  'zolik.rules.cleanRun.off':
    'Is féidir le do shraitheanna áilteoirí a úsáid gan srian — ní gá do cheann ar bith a bheith gan iad.',
  'zolik.rules.contracts.rotating':
    'Maireann an cluiche {n} dháileadh, agus éilíonn gach dáileadh a mheascán féin de thacair agus de shraitheanna.',
  'zolik.rules.contracts.static':
    'Éilíonn gach dáileadh an meascán céanna: {sets} thacar agus {runs} shraith.',
  'zolik.rules.end.afterDeals': 'Críochnaíonn an cluiche tar éis {n} dháileadh.',
  'zolik.rules.end.atScore':
    'Leantar ag dáileadh go dtí go sroicheann duine éigin {n} bpointe — ansin tá deireadh leis.',

  'prsi.rules.section.goal': 'Aidhm',
  'prsi.rules.section.setup': 'Socrú',
  'prsi.rules.section.turn': 'Do sheal',
  'prsi.rules.section.special': 'Cártaí speisialta',
  'prsi.rules.section.end': 'Conas a chríochnaíonn an cluiche',
  'prsi.rules.goal': 'Bí ar an gcéad duine a imríonn gach cárta as a lámh.',
  'prsi.rules.deck': 'Imrítear le paca {value} cárta (ón seacht aníos).',
  'prsi.rules.deal': 'Tosaíonn gach imreoir le {n} chárta.',
  'prsi.rules.turn.match':
    'Imir cárta a mheaitseálann dath nó luach an chárta uachtaraigh — nó tarraing, mura féidir leat.',
  'prsi.rules.turn.draw': 'Cuireann tarraingt deireadh le do sheal gan imirt.',
  'prsi.rules.sevens':
    'Imir 7 agus tarraingíonn an chéad imreoir eile dhá chárta, mura bhfreagraíonn sé le seacht dá chuid féin.',
  'prsi.rules.aces': 'Imir aonach agus scipeáiltear seal an chéad imreora eile.',
  'prsi.rules.queens': 'Imir banríon agus ainmnigh an dath a leanann.',
  'prsi.rules.end': 'Críochnaíonn an cluiche an nóiméad a bhíonn lámh duine éigin folamh.',

  'canasta.rules.section.goal': 'Aidhm',
  'canasta.rules.section.setup': 'Socrú',
  'canasta.rules.section.melding': 'Leagan síos',
  'canasta.rules.section.end': 'Conas a chríochnaíonn an cluiche',
  'canasta.rules.goal': 'Imrítear i mbeirteanna; buann an chéad taobh a shroicheann {n} bpointe an cluiche.',
  'canasta.rules.deck': 'Imrítear le {value} cárta — dhá phaca agus áilteoirí.',
  'canasta.rules.deal': 'Faigheann gach imreoir {n} chárta.',
  'canasta.rules.redThrees':
    'Taispeántar trí dhearg i do láimh láithreach agus faigheann tú bónas air — ach amháin mura gcríochnaíonn do thaobh canasta riamh, agus ansin comhairtear i do choinne é.',
  'canasta.rules.canasta': 'Is é atá i gcanasta ná cumasc de {n} chárta nó níos mó den luach céanna.',
  'canasta.rules.meldFloorBands':
    'Caithfidh do chéad leagan síos íosmhéid pointí a bhaint amach a ardaíonn le do scór: {negative} faoi bhun a náid, {low} suas go 1500, {mid} suas go 3000, {high} os a chionn sin.',
  'canasta.rules.oneCanastaToGoOut': 'Is leor canasta amháin críochnaithe chun go rachadh do thaobh amach.',
  'canasta.rules.twoCanastasToGoOut':
    'Teastaíonn dhá chanasta chríochnaithe ó do thaobh sula bhféadfaidh sé dul amach.',
  'canasta.rules.end':
    'Leantar ag dáileadh go dtí go dtéann taobh amháin thar {n} bpointe — ansin tá an cluiche thart.',

  'holdem.rules.section.goal': 'Aidhm',
  'holdem.rules.section.setup': 'Socrú',
  'holdem.rules.section.betting': 'Geallta',
  'holdem.rules.section.end': 'Conas a chríochnaíonn an cluiche',
  'holdem.rules.goal':
    'Buaigh sliseanna leis an lámh is fearr ag an taispeáint, nó tríd a bheith fágtha mar an t-aon imreoir amháin sa lámh.',
  'holdem.rules.stack': 'Tosaíonn gach suíochán le {n} slis.',
  'holdem.rules.blinds':
    'Is é {sb} an dallóg bheag agus {bb} an dallóg mhór, cuirtear iad sula ndáiltear na cártaí.',
  'holdem.rules.streets':
    'Cuirtear geall i gceithre bhabhta — roimh an flop, agus tar éis an flop, an turn agus an river.',
  'holdem.rules.showdown':
    'Nochtann na himreoirí atá fós sa lámh a gcuid cártaí; buann an lámh chúig chárta is fearr an pota.',
  'holdem.rules.noLimit':
    'Gan teorainn — is féidir le geall ar bith a bheith chomh mór le do chruach ar fad.',
  'holdem.rules.lastPlayerStanding': 'Imrítear go dtí go bhfuil gach slis ag suíochán amháin.',
  'holdem.rules.mostChipsWins':
    'An té a bhfuil an líon is mó slisní aige nuair a stopann an imirt, buann sé an cluiche.',
  'holdem.rules.handLimit': 'Stopann an imirt tar éis {n} lámh.',

  // --- header --------------------------------------------------------------
  'header.deal': 'Dáileadh {n}',
  'header.gameOf': 'Cluiche {n} as {total}',
  'header.gameOfWithContract': 'Cluiche {n} as {total}: {contract}',

  // --- meld preview --------------------------------------------------------
  'preview.validSet': 'Tacar bailí',
  'preview.validRun': 'Sraith bhailí',
  'preview.validMeld': 'Cumasc bailí',
  'preview.notYet': 'Ní cumasc é go fóill',
  'preview.points': '{shape} · {n} bpointe',
  'preview.pointsWithLaid': '{shape} · {n} + {laid} leagtha cheana = {total} pointe',
  'preview.meetsFloor': '{line} (sroicheann {n} ✓)',
  'preview.needsFloor': '{line} (teastaíonn {n} ✗)',
  'preview.becauseOf': '{line} — {reason}',

  // --- discard -------------------------------------------------------------
  // A discard with a meld staged lays that meld first; this is what the
  // player is told when the server refuses it and the whole move is rolled
  // back. {reason} is the engine's own words for the refusal.
  'discard.meldRejected': '{reason} — níor caitheadh aon rud, tá do chártaí fós ullamh.',

  // --- what the current selection would send -------------------------------
  // The client's own reason a control is not ready, next to the engine's own
  // `err.*` reasons rather than a second kind of message — a control greyed
  // out for "not your turn" and one greyed out for "you picked two, this
  // takes one" should read as the same kind of thing, not one of them looking
  // broken. See `fits` in `src/lib/drops.ts`.
  'sel.tooMany.1': 'Roghnaigh cárta amháin agus é sin amháin',
  'sel.tooMany.n': 'Roghnaigh {n} chárta ar a mhéad',
  'sel.needMore': 'Roghnaigh {n} chárta',
  'sel.notThese': 'Ní féidir leis na cártaí sin dul anseo',
  'sel.needsCompany': 'Teastaíonn na cinn in aice leis ón gcárta sin',

  // --- what a module says happened -----------------------------------------
  // The first module-sent keys in this bundle, and deliberately few. Anything
  // a module sends is still legible without an entry here — `humanise` turns
  // `holdem.seat.stack` into "Stack" — so a key earns a line only when that
  // fallback loses something. These do: what each of them means lives in its
  // *params* (who won, how much, whose cards), and a key on its own has
  // nowhere to put them. Without the entry the player read "Winner", full
  // stop, at the end of a match they had just won.
  'status.winner': 'Bainte ag {winners}',
  'holdem.status.pot': 'Bhuaigh {winners} {amount} le {hand}',
  'holdem.status.potUncontested': "Bhuaigh {winners} {amount} — d'fhill gach duine eile",
  'holdem.status.shown': 'Thaispeáin {playerId} {value}',
  'holdem.prompt.waitingFor': 'Ag fanacht le {playerId}',

  // How a rummy scoreboard was ordered. Both earn a line for the same reason
  // as the keys above: the number lives in the params, and `humanise` would
  // render "Deals Won" with nothing after it.
  'zolik.standing.dealsWon': 'Dáiltí buaite: {n}',
  'zolik.standing.inHand': 'Sa lámh: {n}',

  // --- rounds ---------------------------------------------------------------
  // What a round is called, and what one did. Each of these carries its number
  // in the params, so `humanise` alone would render a label with nothing after
  // it.
  // An instruction, not a heading. "Next round" named the thing rather than
  // asking for it, and sat above "Waiting for 1" — which reads as though the
  // player is the one waiting, when the table is waiting for them.
  'round.continue': 'Tosaigh an chéad bhabhta eile',

  // --- the flash that announces a round or a match ending ------------------
  //
  // One line and one number, held for about two seconds. Worded generically on
  // purpose: which *kind* of ending it was is the module's own fact, printed
  // underneath, and a client that phrased "went gin" itself would be a client
  // that knows a game.
  'flash.roundWon': 'Thóg {winners} é',
  'flash.roundWonYou': 'Thóg tusa é',
  'flash.roundDrawn': 'Níor thóg aon duine é',
  'flash.matchOver': 'Tá an cluiche thart',
  'flash.matchWon': 'Bhuaigh {winners}',
  'flash.matchWonYou': 'Bhuaigh tú',
  'flash.matchDrawn': 'Níor bhuaigh aon duine',
  'flash.nowOn': 'anois {total}',

  'zolik.round.deal': 'Dáileadh',
  'zolik.round.cleanRun': 'Caithfidh sraith amháin a bheith gan áilteoir',
  'canasta.round.deal': 'Dáileadh',
  'canasta.round.concealed': 'Chuaigh amach faoi cheilt',
  'canasta.round.exhausted': 'Chríochnaigh an paca',
  'canasta.round.meldCards': 'Cártaí leagtha: {n}',
  'canasta.round.canastas': 'Canastaí: {n}',
  'canasta.round.redThrees': 'Trínna dearga: {n}',
  'canasta.round.goingOut': 'Dul amach: {n}',
  'canasta.round.inHand': 'Fágtha sa lámh: {n}',
  'holdem.round.hand': 'Lámh',
  'holdem.round.pot': 'Pota {n}',
  'holdem.round.uncontested': "D'fhill gach duine eile",
  'seat.ready': 'Réidh',
  'results.you': '(tú)',



  // --- refusals the other three games raise ---------------------------------
  // Worded here for the same reason Žolíky's are: reasonText falls back to the
  // code, and OfferBar hands it the code as that fallback, so an unworded
  // refusal reaches a player as PILE_FROZEN in capitals. serverKeys.test.ts
  // now fails the build rather than letting the next one through.
  'err.SET_TOO_LARGE': 'Tá na ceithre dhath ag tacar cheana',
  'err.DISCARD_TAKEN_CARD_FORBIDDEN':
    'Ní féidir leat an cárta a thóg tú díreach anois a chaitheamh — imir é nó coinnigh é',
  'err.CARD_DOES_NOT_FIT': 'Ní mheaitseálann an cárta sin an dath ná an luach',
  'err.SUIT_REQUIRED': 'Ainmnigh an dath a leanann',
  'err.MUST_ANSWER_DRAW_OR_TAKE': 'Freagair le seacht, nó tóg na cártaí',
  'err.NOTHING_TO_DRAW': 'Níl aon rud fágtha le tarraingt',
  'err.PILE_EMPTY': 'Tá an carn folamh',
  'err.PILE_BLOCKED': 'Tá an carn dúnta — tá trí dhubh ar a bharr',
  'err.PILE_FROZEN': 'Tá an carn reoite — teastaíonn dhá chárta nádúrtha de luach an chárta uachtaraigh uait',
  'err.TOP_CARD_UNUSABLE': 'Ní féidir leat an cárta uachtarach a úsáid',
  'err.MELD_CLOSED': 'Tá an cumasc sin iomlán agus dúnta',
  'err.MELD_TOO_SMALL': 'Teastaíonn níos mó cártaí ná sin ó chumasc',
  'err.MELD_TOO_LARGE': 'Ní féidir leis an gcumasc sin níos mó cártaí a ghlacadh',
  'err.MELD_MIXED_RANKS': 'Caithfidh gach cárta i gcumasc a bheith den luach céanna',
  'err.NOT_ENOUGH_NATURALS': 'Teastaíonn níos mó cártaí nádúrtha ná áilteoirí ó chumasc',
  'err.RANK_ALREADY_MELDED': 'Tá cumasc den luach sin ag do thaobh cheana',
  'err.NOT_YOUR_MELD': 'Is leis an taobh eile an cumasc sin',
  'err.NO_SUCH_MELD': 'Níl an cumasc sin ar an mbord',
  'err.CANNOT_MELD_THREE': 'Ní leagtar trínna síos riamh',
  'err.CANNOT_DISCARD_RED_THREE': 'Ní féidir trí dhearg a chaitheamh',
  'err.MUST_KEEP_A_CARD': 'Coinnigh cárta amháin ar a laghad — ní féidir leat do lámh a fholmhú mar sin',
  'err.MUST_MELD_FIRST': 'Leag síos céad chumasc do thaobha ar dtús',
  'err.INITIAL_MELD_NOT_MET': 'Tá do chéad leagan síos gann ar phointí fós',
  'err.CANNOT_GO_OUT_YET': 'Teastaíonn canasta críochnaithe ó do thaobh sula bhféadfaidh sé dul amach',
  'err.NOTHING_TO_CALL': 'Níl aon gheall le glaoch',
  'err.CANNOT_CHECK': 'Ní féidir leat seiceáil — tá geall ann le freagairt',
  'err.CANNOT_RAISE': 'Ní féidir leat ardú anseo',
  'err.RAISE_TOO_SMALL': 'Caithfidh ardú a bheith chomh mór leis an gceann deireanach ar a laghad',
  'err.NOT_ENOUGH_CHIPS': 'Níl an oiread sin slisní agat',
  'err.AMOUNT_REQUIRED': 'Abair cé mhéad',
  'err.AMOUNT_NOT_A_NUMBER': 'Ní uimhir í an tsuim sin',
  'err.SEAT_NOT_IN_HAND': 'Níl tú sa lámh seo',
  'err.WRONG_RANK': 'Tá an luach mícheart ar an gcárta sin chuige seo',
  'err.MATCH_FULL': 'Tá an bord lán',
  'err.MATCH_ALREADY_STARTED': 'Tá an cluiche tosaithe cheana',
  'err.TOO_FEW_PLAYERS': 'Níl go leor imreoirí ann fós',
  'err.WRONG_PLAYER_COUNT': 'Ní féidir an cluiche seo a imirt leis an oiread sin imreoirí',
  'err.NOT_THE_HOST': 'Níl cead ach ag an óstach é sin a dhéanamh',
  'err.NO_LONGER_WAITING': 'Níl an bord ag fanacht a thuilleadh',
  'err.WAITING_ROOM_UNAVAILABLE': 'Níl an seomra feithimh ar fáil',
  'err.SERVER_BUSY': 'Tá an freastalaí lán faoi láthair — bain triail eile as i gceann nóiméid',


  // --- rules the engine has always enforced and never stated ----------------
  // Every one of these was found by the guardrail rather than by review: the
  // validator could refuse a player for it, and no sentence anywhere said so.
  // See server/internal/zolikmod/ruleindex_test.go.
  'zolik.rules.section.layoff': 'Cur le cumaisc',
  'zolik.rules.pickup.obligation':
    'Sula mbíonn tú síos, caithfear cárta a tógadh ón gcarn caite a úsáid sa chumasc a chuireann síos thú an seal seo.',
  'zolik.rules.pickup.noReturn':
    'Ní féidir cárta a thóg tú ón gcarn caite a chaitheamh arís sa seal céanna — imir é nó coinnigh é.',
  'zolik.rules.wilds.setLimit': 'Ní féidir níos mó áilteoirí ná cártaí nádúrtha a bheith i dtacar.',
  'zolik.rules.set.maxSize':
    'Ní féidir níos mó ná {n} chárta a bheith i dtacar — líonann áilteoir dath atá in easnamh, ní chuireann sé le ceann atá iomlán.',
  'zolik.rules.run.maxLength':
    'Ní féidir níos mó ná {n} chárta a bheith i sraith — an tAonach thíos, an dá luach dhéag os a chionn, agus an tAonach thuas.',
  'zolik.rules.run.aceBridge':
    'Suíonn an tAonach os cionn an Rí nó faoin Dó, riamh mar dhroichead idir dhá cheann sraithe.',
  'zolik.rules.contracts.contribution':
    'Go dtí go mbíonn tú síos, caithfidh gach cumasc a leagann tú a bheith ar cheann a éilíonn conradh an dáilte fós.',
  'zolik.rules.layoff.afterDown':
    'Ní féidir leat cur le cumaisc dhuine ar bith eile go dtí go leagann tú do chonradh féin.',
  'zolik.rules.layoff.runEnds':
    'Caithfidh cárta a chuirtear le sraith í a leanúint ag ceann amháin nó ag an gceann eile.',
  'zolik.rules.jokers.swap':
    'Is féidir áilteoir i gcumasc ar an mbord a cheannach ar ais leis an gcárta beacht a sheasann sé dó.',
  'zolik.rules.jokers.reclaim.on':
    'Caithfear áilteoir a ceannaíodh ar ais den bhord a imirt i gcumasc sa seal céanna — ní féidir é a choinneáil sa lámh.',
  'zolik.rules.jokers.reclaim.off': 'Is féidir áilteoir a ceannaíodh ar ais den bhord a choinneáil sa lámh.',
  'zolik.rules.deck.reshuffle':
    'Nuair a chríochnaíonn an stoc, suaitear an carn caite agus déantar an stoc nua de; má tá an dá cheann folamh, críochnaíonn an dáileadh.',


  // --- what to do instead ---------------------------------------------------
  // The third layer of a refusal, after the reason and the rule. Sent by the
  // module because it is the only side that knows which card is owed and
  // which way out is on offer; see server/internal/zolikmod/remedy.go.
  'zolik.remedy.meldThePickup': 'Cuir {card} le do leagan síos, nó cealaigh an piocadh.',
  'zolik.remedy.discardSomethingElse': 'Caith cárta eile, nó imir {card} an seal seo.',
  'zolik.remedy.discardNotAJoker': 'Caith rud éigin nach áilteoir é.',
  'zolik.remedy.finishOrUndoLayDown': 'Críochnaigh do leagan síos, nó tóg ar ais é.',
  'zolik.remedy.needMorePoints': 'Teastaíonn {n} phointe eile uait sula bhféadfaidh tú dul síos.',
  'zolik.remedy.layACleanRun': 'Leag síos sraith gan áilteoir inti.',
  'zolik.remedy.playReclaimedJoker': 'Imir {card} i gcumasc, nó cealaigh a thógáil.',
  'zolik.remedy.goDownFirst': 'Leag síos do chumaisc féin ar dtús.',
  'zolik.remedy.drawFirst': 'Tarraing cárta ar dtús.',
  'zolik.remedy.drawFromStock': 'Tarraing ón stoc — osclaíonn an carn caite i mbabhta {n}.',
  'zolik.remedy.drawFromStockEmpty': 'Tarraing ón stoc ina áit sin.',


  // --- keys whose meaning lives in their params -----------------------------
  // The shape fallback renders these with the number missing ("Stack", "Pot"),
  // which is why they earn a line where most keys do not.
  'header.contract': 'Teastaíonn {sets} thacar agus {runs} shraith',
  'header.contract.cleanRunOnly': 'Teastaíonn sraith gan áilteoir',
  'header.round': 'Babhta {n}',
  'header.deck': 'Stoc',
  'header.target': 'Sprioc',
  'header.suitInPlay': 'Dath sa chluiche',
  'seat.cards': 'Cártaí',
  'zolik.offer.meld': 'Leag síos',
  'prompt.pickupMustBeMelded':
    'Tháinig {value} ón gcarn caite — caithfidh sé dul isteach sna cumaisc a chuireann síos thú an seal seo.',
  'prompt.jokerMustBePlayed':
    'Tháinig {value} ón mbord — caithfidh sé dul isteach i gcumasc sula bhféadfaidh tú do sheal a chríochnú.',
  'prompt.initialMeld': 'Caithfidh céad chumasc do thaobha {n} bpointe a bhaint amach.',
  'prompt.canastasNeeded': 'Teastaíonn {n} chanasta eile ó do thaobh sula bhféadfaidh sé dul amach.',
  'prompt.mustDrawOrAnswerSeven': 'Freagair le seacht, nó tarraing {n} chárta.',
  'prompt.chooseSuit': 'Roghnaigh an dath a leanann',
  'prompt.skipPending': 'Scipeáiltear do sheal',
  'status.lastDeal': 'Fuair foireann {team} {value}',
  'status.teamScore': 'Foireann {team}: {value}',
  'canasta.offer.rank': 'Luach',
  'canasta.seat.teamScore': 'Scór na foirne',
  'canasta.seat.canastas': 'Canastaí',
  'holdem.header.pot': 'Pota',
  'holdem.header.street': 'Babhta',
  'holdem.header.hand': 'Lámh',
  'holdem.header.handLimit': 'Lámha ar fad',
  'holdem.header.blinds': 'Dallóga',
  'holdem.cost.call': 'chun glaoch',
  'holdem.cost.pot': 'sa phota',
  'holdem.seat.stack': 'Cruach',
  'holdem.seat.bet': 'Geall',
  'holdem.prompt.yourAction': 'Do sheal chun gnímh',
  'holdem.prompt.raiseTo': 'Ardaigh go',
  'zone.yourHand': 'Do lámh',
  'zone.opponentHand': 'A lámh',
  'zone.drawPile': 'Stoc',
  'zone.discardPile': 'Carn caite',
  'zone.melds': 'Cumaisc',
  'zone.teamMelds': 'Cumaisc do thaobha',
  'zone.redThrees': 'Trínna dearga',
  'zone.board': 'Bord',
  'verb.drawFromDeck': 'Tarraing',
  'verb.takeFromDiscard': 'Tóg ón gcarn',


  // --- the why sheet's own furniture ---------------------------------------
  // The three layers a refusal is explained in. Labels, not sentences: what
  // goes under each is sent by the server or worded above.
  'why.reason': 'Cén fáth nach bhfuil',
  'why.rule': 'An riail',
  'why.rules': 'Na rialacha',
  'why.remedy': 'Cad is féidir leat a dhéanamh',
  'why.readTheRules': 'Léigh na rialacha iomlána →',
  'why.close': 'Dún',
  'why.open': 'cén fáth',

  // --- marks on a particular card ------------------------------------------
  // A mark is a refusal that has not happened yet: the rule is enforced at the
  // discard, which is the last possible moment to hear about it, so the card
  // says so while there is still a turn left to act on it.
  'zolik.badge.owedToMeld':
    'Tháinig {card} ón gcarn caite — caithfidh sé dul isteach sna cumaisc a chuireann síos thú an seal seo.',
  'zolik.badge.jokerOwed':
    'Tháinig {card} ón mbord — caithfidh sé dul isteach i gcumasc sula bhféadfaidh tú do sheal a chríochnú.',

  // --- the legal notices ----------------------------------------------------
  // Only the furniture. The documents themselves are in `src/legal`, which is
  // a bundle of the same kind with a parity test of its own — prose that long
  // in a flat key map buries the keys this one exists for.
  //
  // The notice is five fragments rather than one sentence with two links glued
  // in, because Czech does not put the link where English does: "souhlasíš s
  // Podmínkami" inflects the noun the link is made of. Fragments let each
  // locale place and decline its own.
  'legal.terms': 'Téarmaí',
  'legal.privacy': 'Príobháideacht',
  'legal.source': 'Cód foinseach',
  'legal.updated': 'Leagan {version}',
  'legal.draft':
    'Dréacht — níl sé i bhfeidhm fós. Tá ainm, tír agus seoladh teagmhála an oibreora fós le líonadh isteach.',
  'legal.notice.before': 'Trí imirt aontaíonn tú leis na ',
  'legal.notice.terms': 'téarmaí úsáide',
  'legal.notice.between': '. Tá cur síos ar a stóráiltear fút san ',
  'legal.notice.privacy': 'fhógra príobháideachta',
  'legal.notice.after': '.',
  // --- gin rummy -------------------------------------------------------------
  'err.UPCARD_DECLINED': 'Lig tú an cárta sin thart cheana',
  'err.DEADWOOD_TOO_HIGH': 'Tá do deadwood ró-ard chun cnagadh',
  'err.CARD_DOES_NOT_EXTEND_MELD': 'Ní chuireann an cárta sin leis an gcumasc seo',
  'ginrummy.rules.setup': 'Socrú',
  'ginrummy.rules.turn': 'Do sheal',
  'ginrummy.rules.melds': 'Cumaisc',
  'ginrummy.rules.knocking': 'Cnagadh',
  'ginrummy.rules.bigGin': 'Big gin',
  'ginrummy.rules.layoff': 'An cur leis',
  'ginrummy.rules.deadHand': 'An lámh mharbh',
  'ginrummy.rules.scoring': 'Scóráil láimhe',
  'ginrummy.rules.match': 'An cluiche a bhuachan',
  'ginrummy.rules.lineBonuses': 'Bónais sa chuntas',
  'ginrummy.rules.deck': 'Imrítear le paca {value} cárta.',
  'ginrummy.rules.deal': 'Faigheann gach imreoir {value} chárta.',
  'ginrummy.rules.upcard': 'Iompaítear cárta amháin eile aghaidh suas chun an carn caite a thosú.',
  'ginrummy.rules.drawDiscard':
    'Ar do sheal, tarraing cárta amháin — ón stoc nó ón gcarn caite — agus caith ceann amháin ansin.',
  'ginrummy.rules.setsAndRuns':
    'Is é atá i gcumasc ná tacar de thrí nó ceithre chárta den luach céanna, nó sraith de thrí chárta nó níos mó den dath céanna.',
  'ginrummy.rules.aceLow': 'Bíonn an tAonach íseal i gcónaí — níl aon sraith ó Bhanríon go hAonach ann.',
  'ginrummy.rules.knockLimit': 'Is féidir leat cnagadh chomh luath is atá do deadwood {n} nó níos lú.',
  'ginrummy.rules.oklahoma': 'Socraítear teorainn chnagtha na láimhe seo de réir luach an chárta iompaithe.',
  'ginrummy.rules.gin': 'Is é gin é deadwood a náid — an cnagadh is fearr is féidir.',
  'ginrummy.rules.bigGinBonus':
    'Is é big gin é aon chárta déag ar fad i gcumaisc, gan aon chaitheamh, agus is fiú {n} bpointe eile é.',
  'ginrummy.rules.layoffDescription':
    'Tar éis cnagtha nach gin é, is féidir le do chéile comhraic a deadwood féin a chur le do chumaisc sula gcuirtear na lámha i gcomparáid.',
  'ginrummy.rules.deadHandDescription':
    'Má thiteann an stoc go dtí a dhá chárta dheireanacha gan aon duine a bheith tar éis cnagadh, tá an lámh marbh — ní scórálann aon duine, agus dáileann an dáileoir céanna arís.',
  'ginrummy.rules.undercut':
    'Mura bhfuil deadwood do chéile comhraic níos airde ná do cheannsa, gearrann sé fút: faigheann sé an difríocht, móide {n}.',
  'ginrummy.rules.ginBonus': 'Faigheann gin lámh iomlán do chéile comhraic, móide {n}.',
  'ginrummy.rules.target':
    'An chéad duine a théann thar {n} bpointe tar éis do lámh críochnú, buann sé an cluiche.',
  'ginrummy.rules.shutout':
    'Déantar bónas an chluiche a dhúbailt go {n} mura bhfuair an cailliúnaí pointe ar bith.',
  'ginrummy.rules.box': 'Is fiú {n} bpointe gach lámh a bhuaigh tú ag deireadh an chluiche.',
  'ginrummy.rules.gameBonus': 'Is fiú {n} bpointe eile an cluiche a bhuachan.',
  'ginrummy.fact.deadwood': 'deadwood {value}',
  'ginrummy.fact.discardCard': 'Caith {value}',
  'ginrummy.fact.meldCards': 'Ar {value}',
  'ginrummy.header.hand': 'Lámh {n}',
  'ginrummy.round.kind': '{value}',
  'ginrummy.round.hand': 'Lámh',
  'ginrummy.seat.score': '{value}',
  'ginrummy.seat.dealer': 'Dáileoir',
  'ginrummy.status.knocked': 'Chnag {playerId} le deadwood {deadwood}',
  'ginrummy.status.gin': 'Rinne {playerId} gin',
  'ginrummy.status.lastHand': 'An lámh dheireanach: {winner} ({kind}, {delta} pointe)',
  'ginrummy.offer.drawStock': 'Tarraing ón stoc',
  'ginrummy.offer.drawDiscard': 'Tarraing ón gcarn caite',
  'ginrummy.offer.takeUpcard': 'Tóg an cárta iompaithe',
  'ginrummy.offer.passUpcard': 'Lig thart',
  'ginrummy.offer.discard': 'Caith',
  'ginrummy.offer.knock': 'Cnag',
  'ginrummy.offer.gin': 'Gin!',
  'ginrummy.offer.bigGin': 'Big gin!',
  'ginrummy.offer.layOff': 'Cuir leis',
  'ginrummy.offer.finishLayoff': 'Cur leis críochnaithe',
  'ginrummy.zone.knockerHand': 'Lámh an chnagaire',
  'ginrummy.zone.melds': 'Cumaisc',
  'ginrummy.prompt.upcardDecision': 'Tóg an cárta iompaithe, nó lig thart é',
  'ginrummy.prompt.yourTurnDraw': 'Tarraing cárta',
  'ginrummy.prompt.yourTurnDiscard': 'Caith cárta — nó cnag, más féidir leat',
  'ginrummy.prompt.layoff': 'Cuir deadwood leis, nó críochnaigh',

  // --- rummy tiles -------------------------------------------------------------
  'err.TILE_NOT_IN_HAND': 'Níl an leacán sin i do láimh',
  'err.TILE_DOES_NOT_FIT': 'Ní oireann sé sin ansin',
  'err.NO_SUCH_SET': 'Níl an cumasc sin ar an mbord',
  'err.INITIAL_MELD_ONLY': 'Roimh do chéad leagan síos ní féidir leat ach do chumaisc nua féin a atheagrú',
  'err.TABLE_NOT_VALID': 'Níl an bord bailí fós',
  'err.TRAY_NOT_EMPTY': 'Tá leacáin scaoilte fós agat le cur síos',
  'err.NOTHING_PLAYED': 'Imir leacán amháin ar a laghad sula gcríochnaíonn tú do sheal',
  'err.INITIAL_MELD_TOO_LOW': 'Caithfidh do chéad leagan síos a bheith 30 pointe nó níos mó',
  'err.NOT_A_RUN': 'Ní féidir ach sraith a roinnt',
  'err.BAD_SPLIT_POSITION': 'Ní féidir an tsraith seo a roinnt ansin',
  'err.NO_JOKER_IN_SET': 'Níl áilteoir ar bith sa chumasc sin',
  'err.TILE_JOKER_SWAP_MISMATCH': 'Ní hé an leacán sin an rud a sheasann an t-áilteoir dó',
  'rummytiles.rules.setup': 'Socrú',
  'rummytiles.rules.sets': 'Cumaisc',
  'rummytiles.rules.initialMeld': 'An chéad leagan síos',
  'rummytiles.rules.turn': 'Do sheal',
  'rummytiles.rules.jokerTaking': 'Áilteoir a thógáil',
  'rummytiles.rules.ending': 'Babhta a chríochnú',
  'rummytiles.rules.poolExhaustion': 'Má thránn an linn',
  'rummytiles.rules.match': 'An cluiche a bhuachan',
  'rummytiles.rules.tiles': 'Imrítear le {value} leacán.',
  'rummytiles.rules.dealCount': 'Faigheann gach imreoir {value} leacán.',
  'rummytiles.rules.group':
    'Is é atá i ngrúpa ná trí nó ceithre leacán den uimhir chéanna, gach ceann acu de dhath éagsúil.',
  'rummytiles.rules.run': 'Is é atá i sraith ná trí uimhir as a chéile nó níos mó den dath céanna.',
  'rummytiles.rules.noWrap': 'Ní fhilleann 13 timpeall go dtí 1.',
  'rummytiles.rules.joker': 'Seasann áilteoir do leacán ar bith.',
  'rummytiles.rules.initialMeldDescription':
    'Go dtí go leagann tú {n} bpointe nó níos mó in aon seal amháin, ó do lámh féin amháin, níl cead agat baint le haon rud atá ar an mbord cheana.',
  'rummytiles.rules.turnDescription':
    'Imir leacán amháin ar a laghad as do lámh, atheagraigh an bord mar is mian leat, agus críochnaigh le gach cumasc ar an mbord bailí.',
  'rummytiles.rules.noDiscard':
    'Níl aon chaitheamh ann — mura féidir leat seal bailí a chur i gcrích, tarraingíonn tú leacán amháin ina áit.',
  'rummytiles.rules.jokerTakingDescription':
    'Is féidir áilteoir ar an mbord a thógáil trína mhalartú ar an leacán a sheasann sé dó, as do lámh — agus caithfear é a úsáid i gcumasc sula gcríochnaíonn do sheal.',
  'rummytiles.rules.goingOut':
    'Buann an chéad imreoir a bhíonn gan leacáin an babhta. Faigheann gach duine eile luach diúltach a bhfuil fágtha acu; faigheann an buaiteoir suim a chaill gach duine eile.',
  'rummytiles.rules.poolExhaustionLowestWins':
    'Má thránn an linn agus mura féidir le haon duine imirt, críochnaíonn an babhta agus buann an lámh is ísle é.',
  'rummytiles.rules.poolExhaustionNoWinner':
    'Má thránn an linn agus mura féidir le haon duine imirt, críochnaíonn an babhta gan bhuaiteoir — scóráiltear gach lámh gan a thuilleadh.',
  'rummytiles.rules.target':
    'An chéad duine a théann thar {n} bpointe tar éis do bhabhta críochnú, buann sé an cluiche.',
  'rummytiles.rules.roundLimit': 'Críochnaíonn an cluiche tar éis {n} bhabhta — buann an scór is airde.',
  'rummytiles.fact.setCards': '{value}',
  'rummytiles.header.pool': 'Linn {n}',
  'rummytiles.header.round': 'Babhta {n}',
  'rummytiles.round.kind': '{value}',
  'rummytiles.round.round': 'Babhta',
  'rummytiles.seat.score': '{value}',
  'rummytiles.seat.notOpened': 'Gan oscailt',
  'rummytiles.status.lastRound': 'An babhta deireanach: {winner} ({kind})',
  'rummytiles.badge.invalid': 'Níl sé bailí fós',
  'rummytiles.zone.pool': 'Linn',
  'rummytiles.zone.table': 'Bord',
  'rummytiles.zone.tray': 'Raca',
  'rummytiles.offer.place': 'Cuir síos',
  'rummytiles.offer.addFromHand': 'Cuir leis',
  'rummytiles.offer.addFromTray': 'Cuir leis ón raca',
  'rummytiles.offer.take': 'Tóg',
  'rummytiles.offer.split': 'Roinn',
  'rummytiles.offer.swapJoker': 'Malartaigh an t-áilteoir',
  'rummytiles.offer.resetTurn': 'Athshocraigh an seal',
  'rummytiles.offer.commit': 'Déanta',
  'rummytiles.offer.draw': 'Tarraing',
  'rummytiles.param.position': 'Roinn ag',

  // --- blackjack -----------------------------------------------------------
  //
  // The house rules are worded twice over — on and off — because a rule that
  // is *not* in force is still something a player has to be told. "No
  // surrender at this table" is information; silence is a guess.
  'err.BET_BELOW_MINIMUM': 'Tá sé sin faoi bhun íosmhéid an bhoird',
  'err.ALREADY_BET': 'Tá do gheall curtha cheana',
  'err.INSURANCE_CLOSED': 'Níl árachas le tógáil faoi láthair',
  'err.CANNOT_DOUBLE': 'Ní féidir an lámh seo a dhúbailt',
  'err.CANNOT_SPLIT': 'Ní féidir an lámh seo a roinnt',
  'err.CANNOT_SURRENDER': 'Ní féidir an lámh seo a ghéilleadh',

  'blackjack.rules.section.table': 'An bord',
  'blackjack.rules.section.play': 'Lámh a imirt',
  'blackjack.rules.section.dealer': 'An dáileoir',
  'blackjack.rules.section.end': 'Conas a chríochnaíonn an cluiche',
  'blackjack.rules.goal':
    'Buaigh ar an dáileoir gan dul thar fiche a haon. Cailleann dul thairis láithreach, is cuma cad a dhéanann an dáileoir ina dhiaidh sin.',
  // Counts are their own phrase rather than a number glued to a noun: one
  // deck and six decks inflect differently, and Czech inflects them again.
  'blackjack.rules.decks': 'Pacaí sa bhróig: {n}.',
  'blackjack.rules.stack': 'Suíonn gach suíochán síos le {n} slis.',
  'blackjack.rules.minBet': 'Is é {n} slis íosmhéid an bhoird.',
  'blackjack.rules.faceUp':
    'Dáiltear cártaí na n-imreoirí aghaidh suas; coinníonn an dáileoir cárta amháin síos go dtí go mbíonn gach duine tar éis imirt.',
  'blackjack.rules.hitStand': 'Tarraing an oiread cártaí is mian leat, nó seas ar a bhfuil agat.',
  'blackjack.rules.aces':
    'Comhairtear aonach mar aon déag fad is a oireann sé, agus mar a haon nuair nach n-oireann.',
  'blackjack.rules.blackjack': 'Is blackjack é aonach le cárta deich-luacha ar an gcéad dá chárta.',
  'blackjack.rules.pays3to2': 'Íocann blackjack 3:2.',
  'blackjack.rules.pays6to5': 'Íocann blackjack 6:5.',
  'blackjack.rules.paysEven': 'Íocann blackjack a chomhionann.',
  'blackjack.rules.double':
    'Ar do chéad dá chárta is féidir leat do gheall a dhúbailt agus cárta amháin eile go díreach a thógáil.',
  'blackjack.rules.doubleAfterSplit': 'Is féidir lámh a tháinig as roinnt a dhúbailt freisin.',
  'blackjack.rules.noDoubleAfterSplit': 'Ní féidir lámh a tháinig as roinnt a dhúbailt.',
  'blackjack.rules.split':
    'Is féidir dhá chárta den luach céanna a roinnt ina lámha féin, gach ceann lena gheall féin — suas le {n} uair, do {hands} lámh ar fad.',
  'blackjack.rules.noSplit': 'Ní roinntear péirí ag an mbord seo.',
  'blackjack.rules.splitAces':
    'Faigheann aonaigh roinnte cárta an ceann agus seasann siad ansin, agus ní blackjack é fiche a haon a dhéantar mar sin.',
  'blackjack.rules.surrender':
    'Is féidir leat do chéad lámh a ghéilleadh ar leath a ghill, chomh luath is a bheidh an dáileoir tar éis seiceáil an bhfuil blackjack aige.',
  'blackjack.rules.noSurrender': 'Ní féidir lámha a ghéilleadh ag an mbord seo.',
  'blackjack.rules.dealerDraws': 'Tarraingíonn an dáileoir go dtí a seacht déag agus seasann sé ansin.',
  'blackjack.rules.hitsSoft17': 'Tarraingíonn an dáileoir ar a seacht déag a dhéantar le haonach.',
  'blackjack.rules.standsSoft17': 'Seasann an dáileoir ar a seacht déag a dhéantar le haonach.',
  'blackjack.rules.dealerPeeks':
    'Agus aonach nó deich á thaispeáint aige, seiceálann an dáileoir an bhfuil blackjack aige sula n-imríonn aon duine.',
  'blackjack.rules.insurance':
    'In aghaidh aonach an dáileora is féidir leat árachas a thógáil ar leath do ghill; íocann sé 2:1 má tá blackjack ag an dáileoir.',
  'blackjack.rules.noInsurance': 'Ní chuirtear árachas ar fáil ag an mbord seo.',
  'blackjack.rules.rounds': 'Imríonn an bord {n} bhabhta.',
  'blackjack.rules.mostChipsWins':
    'An té a bhfuil an líon is mó slisní aige ag an deireadh, buann sé an cluiche.',
  'blackjack.rules.bustedOut':
    'Suíochán nach féidir leis íosmhéid {n} a chlúdach a thuilleadh, fanann sé amuigh don chuid eile den chluiche.',

  'blackjack.zone.dealer': 'Dáileoir',
  'blackjack.zone.box': 'Lámh',
  'blackjack.zone.yourBox': 'Do lámh',
  'blackjack.zone.shoe': 'Bróg',

  'blackjack.header.round': 'Babhta {n} as {of}',
  'blackjack.header.minBet': 'Íosmhéid',
  'blackjack.header.decks': 'Pacaí',
  'blackjack.header.dealerTotal': 'Taispeánann an dáileoir {n}',
  'blackjack.header.dealerSoftTotal': 'Taispeánann an dáileoir {n} bog',

  'blackjack.seat.stack': 'Slisní',
  'blackjack.seat.bet': 'Geall',
  'blackjack.seat.insurance': 'Árachas',
  'blackjack.seat.total': 'Iomlán',
  'blackjack.seat.softTotal': 'Iomlán bog',
  'blackjack.seat.out': 'Gan slisní',

  'blackjack.prompt.placeBet': 'Cuir do gheall',
  'blackjack.prompt.insurance': 'Árachas?',
  'blackjack.prompt.yourMove': 'Do sheal',
  'blackjack.prompt.waitingFor': 'Ag fanacht le {playerId}',
  'blackjack.prompt.betAmount': 'Geall',

  'blackjack.offer.bet': 'Cuir geall',
  'blackjack.offer.hit': 'Cárta',
  'blackjack.offer.stand': 'Seasaim',
  'blackjack.offer.double': 'Dúbail',
  'blackjack.offer.split': 'Roinn',
  'blackjack.offer.surrender': 'Géill',
  'blackjack.offer.insure': 'Tóg árachas',
  'blackjack.offer.declineInsurance': 'Gan árachas',

  'blackjack.fact.tableMinimum': 'íosmhéid',
  'blackjack.fact.insuranceCost': 'ar árachas',
  'blackjack.fact.extraStake': 'le cur',
  'blackjack.fact.surrenderReturn': 'ar ais',

  'blackjack.status.dealerBlackjack': 'Bhí blackjack ag an dáileoir',
  'blackjack.status.dealerBust': 'Chuaigh an dáileoir thar fóir le {n}',
  'blackjack.status.dealerStands': 'Seasann an dáileoir ar {n}',

  'blackjack.round.name': 'Babhta',
  'blackjack.round.dealerTotal': 'Dáileoir {n}',
  'blackjack.round.dealerBust': 'Dáileoir thar fóir ({n})',
  'blackjack.round.dealerBlackjack': 'Blackjack an dáileora',
  'blackjack.round.outcome.blackjack': 'Blackjack',
  'blackjack.round.outcome.win': 'Buaite',
  'blackjack.round.outcome.push': 'Cothrom',
  'blackjack.round.outcome.lose': 'Caillte',
  'blackjack.round.outcome.bust': 'Thar fóir',
  'blackjack.round.outcome.surrender': 'Géillte',

  'blackjack.badge.inPlay': 'Á imirt',
  'blackjack.badge.doubled': 'Dúbailte',
  'blackjack.badge.split': 'Roinnte',
  'blackjack.badge.blackjack': 'Blackjack',
  'blackjack.badge.bust': 'Thar fóir',
  'blackjack.badge.won': 'Buaite',
  'blackjack.badge.push': 'Cothrom',
  'blackjack.badge.lost': 'Caillte',
  'blackjack.badge.surrendered': 'Géillte',

  'blackjack.unit.chips': 'slis',

  // --- the settings screen --------------------------------------------------
  //
  // Hardcoded English until now, which was survivable while the only other
  // language was unreachable and faintly absurd once the language picker
  // itself lived on this screen: a player who cannot read "Settings" is
  // exactly the one who came here to change it.
  'settings.title': 'Socruithe',
  'settings.subtitle': 'An chuma atá ortsa, agus an chuma atá ar an mbord',
  'settings.face.heading': "D'aghaidh ag an mbord",
  'settings.face.account': 'Coinnítear le do chuntas í, mar sin leanann sí thú go gléas eile.',
  'settings.face.device': 'Coinnítear ar an ngléas seo í. Sínigh isteach chun í a thabhairt leat.',
  'settings.skin.heading': 'Cuma an bhoird',
  'settings.language.heading': 'Teanga',
  'settings.language.status': 'Coinnítear ar an ngléas seo í.',
  'settings.language.auto': 'Uathoibríoch',
  'settings.language.auto.now': 'Leanann sé do ghléas — {language} anois',
  'settings.legal.heading': 'An cló beag',
  'settings.legal.status': 'Cad ar aontaigh tú leis tríd an imirt, agus cad a stóráiltear fút.',
  'settings.signIn': 'Sínigh isteach',
  'settings.back': 'Ar ais',

  // A translated interface over an English notice is a worse lie than an
  // English interface, so the document says so itself rather than letting the
  // surrounding screen imply the text was written for this reader.
  'legal.untranslated':
    'Níl an fógra seo aistrithe go dtí do theanga fós. Is é an téacs Béarla thíos an leagan atá i bhfeidhm.',

  // --- the navigation bar's screen titles ------------------------------------
  //
  // These sit in `app/_layout.tsx`, above every screen, and were the last
  // English left after the settings screen was translated — a German player
  // reading "Einstellungen" under a bar that said "Settings". A translated app
  // with an untranslated chrome bar looks like a bug, not like a limit.
  'nav.home': 'Zolik',
  'nav.emailSignIn': 'Sínigh isteach le ríomhphost',
  'nav.signingIn': 'Ag síniú isteach',
  'nav.usernameSignIn': 'Sínigh isteach le hainm úsáideora',
  'nav.legacyAccount': 'Cuntas oidhreachta',
  'nav.guest': 'Aoi',
  'nav.account': 'Cuntas',
  'nav.games': 'Cluichí',
  'nav.table': 'Do bhord',
  'nav.join': 'Téigh isteach i mbord',
  // The screen a shared invite link lands on while it resolves the code.
  'nav.joining': 'Ag dul isteach',
  'nav.rules': 'Rialacha',
  'nav.match': 'Cluiche',
  'nav.scoreTable': 'Tábla scór',
  'nav.stats': 'Staitisticí',
};
