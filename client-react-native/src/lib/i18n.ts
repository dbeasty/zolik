/**
 * Message keys and locale bundles.
 *
 * The server never sends a rendered sentence. It sends stable keys — engine
 * error codes (`NOT_YOUR_TURN`), option names, and structured facts like the
 * contract's sets/runs counts — and the wording lives here. That split is what
 * makes a Czech UI possible without touching the server, and it is the reason
 * Phase 1 shipped `whyNot` as a code rather than a message.
 *
 * Three rules this module keeps:
 *
 *  1. **A missing translation degrades, never blanks.** The lookup falls back
 *     locale → English → the caller's fallback → the key itself. A player
 *     seeing an untranslated English string is a bad day; a player seeing an
 *     empty control is a bug report.
 *  2. **Every key in one bundle exists in all of them.** Asserted by a test,
 *     not by review — a half-translated locale is how "mostly Czech with
 *     random English" ships.
 *  3. **No rule knowledge.** This file maps keys to words. It never decides
 *     what is legal, what a card is worth, or which profile is in play.
 */

export type Locale = 'en' | 'cs';

export const LOCALES: { id: Locale; label: string }[] = [
  { id: 'en', label: 'English' },
  { id: 'cs', label: 'Čeština' },
];

/**
 * Interpolation params. Values are substituted into `{name}` placeholders.
 */
export type Params = Record<string, string | number>;

// The English bundle is the reference: every other locale is checked against
// its key set. Keys are namespaced by what they describe, so an unfamiliar one
// is at least placeable.
const en: Record<string, string> = {
  // --- engine error codes (rules.RulesErrorCode) ---------------------------
  'err.NOT_YOUR_TURN': "It's not your turn",
  'err.WRONG_PHASE': 'Not available right now',
  'err.MUST_DRAW_FIRST': 'Draw a card before melding',
  'err.GAME_SUSPENDED': 'The game is paused',
  'err.GAME_NOT_ACTIVE': 'The game is not running',
  'err.DISCARD_LOCKED': 'The discard pile is locked for now',
  'err.DISCARD_PILE_EMPTY': 'The discard pile is empty',
  'err.NO_CARDS_LEFT': 'No cards left to draw',
  'err.ROUND_REQ_NOT_MET': 'Lay your own initial meld first',
  'err.NEED_CLEAN_RUN': 'You need a joker-free run on the table before you count as down',
  'err.INCOMPLETE_INITIAL_MELD': 'Finish your lay-down, or undo it, before you discard',
  'err.DISCARD_CARD_NOT_MELDED': 'The card you picked up must go into your meld',
  'err.JOKER_DISCARD_FORBIDDEN': "A joker can't be discarded",
  'err.NOTHING_TO_UNDO': 'Nothing to undo',
  'err.NO_JOKER_IN_MELD': 'No joker in this meld',
  'err.JOKER_SWAP_MISMATCH': "That card doesn't take the joker's place",
  'err.BREAKS_CLEAN_RUN': 'That run has to stay joker-free',
  'err.WRONG_RUN_END': 'That card extends the other end of the run',
  'err.INVALID_MELD': 'No card in your hand fits here',
  'err.CARD_NOT_IN_HAND': 'That card is not in your hand',
  'err.MELD_BELOW_MINIMUM': 'Your melds are still short of the points needed to go down',
  'err.MELD_NO_CONTRIBUTION': "That meld doesn't advance your requirement",
  'err.TOO_MANY_WILDS': 'Too many wild cards in that meld',
  'err.ADJACENT_WILDS': 'Two wild cards cannot sit next to each other',
  'err.ACE_BRIDGE': 'An ace cannot bridge king and two',

  // --- contract phrasing ---------------------------------------------------
  // Built from the structured contract the server sends (sets/runs counts),
  // never from a deal-number lookup table.
  // Whole phrases per count rather than a number plus a pluralised noun:
  // Czech inflects the noun by count (skupina / skupiny / skupin) in a way no
  // "add an s" helper survives, and the counts that actually occur are 1-3.
  // A count outside that falls back to contract.sets.n.
  'contract.sets.1': 'One set',
  'contract.sets.2': 'Two sets',
  'contract.sets.3': 'Three sets',
  'contract.sets.n': '{n} sets',
  'contract.runs.1': 'One run',
  'contract.runs.2': 'Two runs',
  'contract.runs.3': 'Three runs',
  'contract.runs.n': '{n} runs',
  'contract.any': 'Any valid meld',
  'contract.cleanRunOnly': 'Any mix of sets and runs — at least one run must be joker-free',
  'contract.cleanRunSuffix': '{base} — one run must be joker-free',

  // --- written rules ---------------------------------------------------------
  // Full-sentence keys a "see the rules" screen renders, one per module,
  // resolved server-side against the table's actual variation and options —
  // see internal/module/rules.go and each game's rules.go. Shared section
  // titles first, then one block per module in the order the four games were
  // added.
  'zolik.rules.section.goal': 'Goal',
  'zolik.rules.section.setup': 'Setup',
  'zolik.rules.section.turn': 'Your turn',
  'zolik.rules.section.melding': 'Laying down',
  'zolik.rules.section.end': 'How the match ends',
  'zolik.rules.goal':
    "Be the first to empty your hand by laying down valid sets and runs, while scoring as few penalty points as possible in the cards you're still holding when someone else goes out.",
  'zolik.rules.deal': 'Each player is dealt {n} cards.',
  'zolik.rules.meldShapes':
    'A set is {set}+ cards of the same rank; a run is {run}+ consecutive cards of the same suit.',
  'zolik.rules.turn.draw': 'On your turn, draw one card — from the stock, or from the discard pile.',
  'zolik.rules.pickup.topOnly': 'Only the top card of the discard pile may be taken.',
  'zolik.rules.pickup.anyFromPile':
    'Any card in the discard pile may be taken, along with everything stacked above it.',
  'zolik.rules.pickup.locked': "The discard pile can't be drawn from until round {n}.",
  'zolik.rules.pickup.open': 'The discard pile is open from the first round.',
  'zolik.rules.turn.discard': 'End your turn by discarding one card.',
  'zolik.rules.jokers.restricted':
    'A joker can never be discarded, except as the exact card that empties your hand.',
  'zolik.rules.meldFloor.on':
    "Your first meld (or melds) must total at least {n} natural points before you're down.",
  'zolik.rules.meldFloor.off': "There's no minimum point value on your first meld.",
  'zolik.rules.cleanRun.on':
    'At least one of your runs must be completely joker-free before you count as down.',
  'zolik.rules.cleanRun.off': "Your runs may use jokers freely — no run has to be joker-free.",
  'zolik.rules.contracts.rotating':
    'The match is {n} deals long, and each deal requires its own combination of sets and runs.',
  'zolik.rules.contracts.static': 'Every deal requires the same combination: {sets} sets and {runs} runs.',
  'zolik.rules.end.afterDeals': 'The match ends after {n} deals.',
  'zolik.rules.end.atScore': "The match keeps redealing until someone reaches {n} points — then it's over.",

  'prsi.rules.section.goal': 'Goal',
  'prsi.rules.section.setup': 'Setup',
  'prsi.rules.section.turn': 'Your turn',
  'prsi.rules.section.special': 'Special cards',
  'prsi.rules.section.end': 'How the match ends',
  'prsi.rules.goal': 'Be the first to play every card in your hand.',
  'prsi.rules.deck': 'Played with a {value}-card deck (7 and up).',
  'prsi.rules.deal': 'Each player starts with {n} cards.',
  'prsi.rules.turn.match':
    "Play a card that matches the top card's suit or rank — or draw if you can't.",
  'prsi.rules.turn.draw': 'Drawing ends your turn without a play.',
  'prsi.rules.sevens':
    'Play a 7 and the next player draws two cards, unless they can answer with a 7 of their own.',
  'prsi.rules.aces': "Play an ace and the next player's turn is skipped.",
  'prsi.rules.queens': 'Play a queen and name the suit that continues.',
  'prsi.rules.end': "The match ends the moment someone's hand is empty.",

  'canasta.rules.section.goal': 'Goal',
  'canasta.rules.section.setup': 'Setup',
  'canasta.rules.section.melding': 'Melding',
  'canasta.rules.section.end': 'How the match ends',
  'canasta.rules.goal':
    'Play in partnerships; the first side to reach {n} points wins the match.',
  'canasta.rules.deck': 'Played with {value} cards — two decks plus jokers.',
  'canasta.rules.deal': 'Each player is dealt {n} cards.',
  'canasta.rules.redThrees':
    "A red three in your hand is shown immediately and scores as a bonus — unless your side never completes a canasta, when it counts against you instead.",
  'canasta.rules.canasta': 'A canasta is a meld of {n} or more cards of the same rank.',
  'canasta.rules.meldFloorBands':
    'Your first meld must reach a point minimum that rises with your score: {negative} below zero, {low} up to 1500, {mid} up to 3000, {high} beyond that.',
  'canasta.rules.oneCanastaToGoOut': 'One completed canasta is enough for your side to go out.',
  'canasta.rules.twoCanastasToGoOut':
    'Your side needs two completed canastas before it may go out.',
  'canasta.rules.end':
    'The deal keeps being redealt until one side passes {n} points — then the match is over.',

  'holdem.rules.section.goal': 'Goal',
  'holdem.rules.section.setup': 'Setup',
  'holdem.rules.section.betting': 'Betting',
  'holdem.rules.section.end': 'How the match ends',
  'holdem.rules.goal':
    'Win chips by having the best hand at showdown, or by being the only player left in the hand.',
  'holdem.rules.stack': 'Every seat starts with {n} chips.',
  'holdem.rules.blinds':
    'The small blind is {sb} and the big blind is {bb}, posted before the cards are dealt.',
  'holdem.rules.streets':
    'Betting happens in four rounds — before the flop, and after the flop, turn and river are dealt.',
  'holdem.rules.showdown':
    'Players still in the hand reveal their cards; the best five-card hand wins the pot.',
  'holdem.rules.noLimit':
    'No-limit betting — any bet may be for any amount up to your whole stack.',
  'holdem.rules.lastPlayerStanding': 'The match plays until one seat holds every chip.',
  'holdem.rules.mostChipsWins': 'Whoever holds the most chips when play stops wins the match.',
  'holdem.rules.handLimit': 'Play stops after {n} hands.',

  // --- header --------------------------------------------------------------
  'header.deal': 'Deal {n}',
  'header.gameOf': 'Game {n} of {total}',
  'header.gameOfWithContract': 'Game {n} of {total}: {contract}',

  // --- meld preview --------------------------------------------------------
  'preview.validSet': 'Valid set',
  'preview.validRun': 'Valid run',
  'preview.validMeld': 'Valid meld',
  'preview.notYet': 'Not a meld yet',
  'preview.points': '{shape} · {n} points',
  'preview.pointsWithLaid': '{shape} · {n} + {laid} already laid = {total} points',
  'preview.meetsFloor': '{line} (meets {n} ✓)',
  'preview.needsFloor': '{line} (needs {n} ✗)',
  'preview.becauseOf': '{line} — {reason}',

  // --- discard -------------------------------------------------------------
  // A discard with a meld staged lays that meld first; this is what the
  // player is told when the server refuses it and the whole move is rolled
  // back. {reason} is the engine's own words for the refusal.
  'discard.meldRejected': '{reason} — nothing was discarded, your cards are still staged.',

  // --- what a module says happened -----------------------------------------
  // The first module-sent keys in this bundle, and deliberately few. Anything
  // a module sends is still legible without an entry here — `humanise` turns
  // `holdem.seat.stack` into "Stack" — so a key earns a line only when that
  // fallback loses something. These do: what each of them means lives in its
  // *params* (who won, how much, whose cards), and a key on its own has
  // nowhere to put them. Without the entry the player read "Winner", full
  // stop, at the end of a match they had just won.
  'status.winner': 'Won by {winners}',
  'holdem.status.pot': '{winners} won {amount} with {hand}',
  'holdem.status.potUncontested': '{winners} won {amount} — everyone else folded',
  'holdem.status.shown': '{playerId} showed {value}',
  'holdem.prompt.waitingFor': 'Waiting for {playerId}',

};

// Czech. Present to prove the seam is real rather than theoretical: if a
// second locale can be added without touching the server or any rule logic,
// the message-key split did its job.
const cs: Record<string, string> = {
  'err.NOT_YOUR_TURN': 'Nejsi na řadě',
  'err.WRONG_PHASE': 'Teď to nejde',
  'err.MUST_DRAW_FIRST': 'Nejdřív si lízni kartu',
  'err.GAME_SUSPENDED': 'Hra je pozastavena',
  'err.GAME_NOT_ACTIVE': 'Hra neběží',
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
  'err.BREAKS_CLEAN_RUN': 'Tato postupka musí zůstat bez žolíka',
  'err.WRONG_RUN_END': 'Tato karta patří na druhý konec postupky',
  'err.INVALID_MELD': 'Žádná karta v ruce sem nepasuje',
  'err.CARD_NOT_IN_HAND': 'Tuto kartu v ruce nemáš',
  'err.MELD_BELOW_MINIMUM': 'Tvé kombinace zatím nemají dost bodů na vyložení',
  'err.MELD_NO_CONTRIBUTION': 'Tato kombinace ti nepomůže splnit požadavek',
  'err.TOO_MANY_WILDS': 'Příliš mnoho žolíků v kombinaci',
  'err.ADJACENT_WILDS': 'Dva žolíci nemohou být vedle sebe',
  'err.ACE_BRIDGE': 'Eso nemůže spojit krále a dvojku',

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
  'zolik.rules.meldFloor.on':
    'Tvoje první kombinace (nebo kombinace) musí dohromady mít aspoň {n} přirozených bodů, než jsi dole.',
  'zolik.rules.meldFloor.off': 'Na první kombinaci není žádná minimální bodová hranice.',
  'zolik.rules.cleanRun.on':
    'Než se počítáš jako dole, musíš mít na stole aspoň jednu postupku úplně bez žolíka.',
  'zolik.rules.cleanRun.off': 'Postupky můžou žolíky používat volně — žádná nemusí být bez žolíka.',
  'zolik.rules.contracts.rotating':
    'Zápas má {n} rozdání a každé vyžaduje jinou kombinaci skupin a postupek.',
  'zolik.rules.contracts.static': 'Každé rozdání vyžaduje stejnou kombinaci: {sets} skupin a {runs} postupek.',
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
  'prsi.rules.sevens':
    'Zahraješ-li sedmu, další hráč líže dvě karty, pokud nemá vlastní sedmu k odpovědi.',
  'prsi.rules.aces': 'Zahraješ-li eso, další hráč je vynechán.',
  'prsi.rules.queens': 'Zahraješ-li dámu, urči barvu, kterou se hraje dál.',
  'prsi.rules.end': 'Hra končí ve chvíli, kdy má někdo prázdnou ruku.',

  'canasta.rules.section.goal': 'Cíl',
  'canasta.rules.section.setup': 'Příprava',
  'canasta.rules.section.melding': 'Vykládání',
  'canasta.rules.section.end': 'Konec zápasu',
  'canasta.rules.goal':
    'Hraje se ve dvojicích; zápas vyhrává strana, která první dosáhne {n} bodů.',
  'canasta.rules.deck': 'Hraje se s {value} kartami — dva balíčky plus žolíci.',
  'canasta.rules.deal': 'Každý hráč dostane {n} karet.',
  'canasta.rules.redThrees':
    'Červenou trojku v ruce hned ukážeš a započítá se jako bonus — pokud ale tvá strana nedokončí ani jednu kanastu, počítá se naopak proti vám.',
  'canasta.rules.canasta': 'Kanasta je kombinace {n} a více karet stejné hodnoty.',
  'canasta.rules.meldFloorBands':
    'Minimální hodnota první kombinace roste s vaším skóre: {negative} pod nulou, {low} do 1500, {mid} do 3000, {high} nad tím.',
  'canasta.rules.oneCanastaToGoOut': 'Vaší straně stačí k ukončení hry jedna dokončená kanasta.',
  'canasta.rules.twoCanastasToGoOut':
    'Vaše strana potřebuje k ukončení hry dvě dokončené kanasty.',
  'canasta.rules.end':
    'Rozdává se dál, dokud jedna strana nepřekročí {n} bodů — pak zápas končí.',

  'holdem.rules.section.goal': 'Cíl',
  'holdem.rules.section.setup': 'Příprava',
  'holdem.rules.section.betting': 'Sázení',
  'holdem.rules.section.end': 'Konec zápasu',
  'holdem.rules.goal':
    'Vyhrávej žetony nejlepší kombinací u vyhodnocení, nebo tím, že v kole zůstaneš jako jediný.',
  'holdem.rules.stack': 'Každé místo začíná s {n} žetony.',
  'holdem.rules.blinds':
    'Malý blind je {sb} a velký blind {bb}, vkládají se ještě před rozdáním karet.',
  'holdem.rules.streets':
    'Sázelo se ve čtyřech kolech — před flopem a po rozdání flopu, turnu a riveru.',
  'holdem.rules.showdown':
    'Hráči, kteří zůstali ve hře, odkryjí karty; bank bere nejlepší kombinace z pěti karet.',
  'holdem.rules.noLimit':
    'Sázení bez limitu — vsadit lze libovolnou částku až do výše celého stacku.',
  'holdem.rules.lastPlayerStanding': 'Hraje se, dokud jedno místo nezíská všechny žetony.',
  'holdem.rules.mostChipsWins': 'Zápas vyhrává ten, kdo má na konci nejvíc žetonů.',
  'holdem.rules.handLimit': 'Hraje se {n} rozdání.',

  'header.deal': 'Rozdání {n}',
  'header.gameOf': 'Hra {n} ze {total}',
  'header.gameOfWithContract': 'Hra {n} ze {total}: {contract}',

  'preview.validSet': 'Platná skupina',
  'preview.validRun': 'Platná postupka',
  'preview.validMeld': 'Platná kombinace',
  'preview.notYet': 'Zatím není kombinace',
  'preview.points': '{shape} · {n} bodů',
  'preview.pointsWithLaid': '{shape} · {n} + {laid} již vyloženo = {total} bodů',
  'preview.meetsFloor': '{line} (splňuje {n} ✓)',
  'preview.needsFloor': '{line} (potřebuje {n} ✗)',
  'preview.becauseOf': '{line} — {reason}',

  'discard.meldRejected': '{reason} — nic se neodhodilo, karty máš pořád připravené.',

  // Wording that names no verb, because a Czech verb agrees with the gender of
  // whoever did it and a player id carries no gender. "Vítěz: Anna" is right
  // where "Vyhrál Anna" is wrong.
  'status.winner': 'Vítěz: {winners}',
  'holdem.status.pot': 'Bank {amount} bere {winners} — {hand}',
  'holdem.status.potUncontested': 'Bank {amount} bere {winners} — ostatní složili',
  'holdem.status.shown': 'Karty hráče {playerId}: {value}',
  'holdem.prompt.waitingFor': 'Čeká se na hráče {playerId}',

};

export const BUNDLES: Record<Locale, Record<string, string>> = { en, cs };

let currentLocale: Locale = 'en';

export function setLocale(locale: Locale) {
  currentLocale = BUNDLES[locale] ? locale : 'en';
}

export function getLocale(): Locale {
  return currentLocale;
}

/**
 * Looks up a key and substitutes `{name}` placeholders.
 *
 * Falls back locale → English → `fallback` → the key itself, so an
 * untranslated string degrades to a readable one rather than to nothing.
 */
export function t(key: string, params?: Params, fallback?: string): string {
  return interpolate(messageTemplate(key) ?? fallback ?? key, params);
}

/**
 * The wording a key would use, or undefined for a key no bundle knows.
 *
 * Exported so a caller can tell "this key has words of its own" from "this key
 * is about to be rendered by its own shape". The difference matters to
 * whoever is composing a line out of more than the key alone — see `factText`,
 * where a phrase that already places its own values must not have another one
 * appended after it.
 */
export function messageTemplate(key: string): string | undefined {
  return BUNDLES[currentLocale]?.[key] ?? BUNDLES.en[key];
}

function interpolate(template: string, params?: Params): string {
  if (!params) return template;
  return template.replace(/\{(\w+)\}/g, (whole, name: string) =>
    name in params ? String(params[name]) : whole,
  );
}

/**
 * Wording for an engine error code. The codes are the server's stable
 * vocabulary; only the phrasing is ours. An unrecognised code — a newer
 * server than this build — falls back rather than printing SCREAMING_SNAKE at
 * the player.
 */
export function reasonText(code: string | undefined, fallback = ''): string {
  if (!code) return fallback;
  return t(`err.${code}`, undefined, fallback);
}

/**
 * "One set", "Two runs" — the whole phrase, looked up by count.
 *
 * Not a number glued to a pluralised noun: Czech inflects the noun by count
 * in a way no "add an s" helper survives, so the phrase is what the bundle
 * owns. Counts beyond the enumerated ones fall back to a generic form.
 */
export function countLabel(n: number, noun: 'sets' | 'runs'): string {
  const exact = `contract.${noun}.${n}`;
  if (BUNDLES.en[exact]) return t(exact);
  return t(`contract.${noun}.n`, { n });
}
