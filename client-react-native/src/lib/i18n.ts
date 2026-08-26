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

  // --- rules panel ---------------------------------------------------------
  'rules.variation': 'Variation',
  'rules.dealSize': 'Deal size',
  'rules.minSetSize': 'Minimum set size',
  'rules.minRunSize': 'Minimum run length',
  'rules.toGoDown': 'To go down',
  'rules.meldFloor': 'Meld value floor',
  'rules.discardPickup': 'Discard pickup',
  'rules.jokers': 'Jokers',
  'rules.matchEnds': 'Match ends',
  'rules.cards': '{n} cards',
  'rules.floorOff': 'Off',
  'rules.floorPoints': '{n}+ points on your first meld(s)',
  'rules.pickupTopOnly': 'Top card only',
  'rules.pickupAnyFromPile': 'Any card in the pile (and everything stacked above it)',
  'rules.pickupLocked': '{scope}, locked until round {n}',
  'rules.jokersRestricted': 'Can never be discarded, except as the card that ends your hand',
  'rules.jokersFree': 'Can be discarded freely',
  'rules.endsAfterDeals': 'After {n} deals',
  'rules.endsAtScore': 'First to {n} points',
  'rules.endsOnDeal': 'When a deal ends',

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

  // --- what the current selection would send -------------------------------
  // The client's own reason a control is not ready, next to the engine's own
  // `err.*` reasons rather than a second kind of message — a control greyed
  // out for "not your turn" and one greyed out for "you picked two, this
  // takes one" should read as the same kind of thing, not one of them looking
  // broken. See `fits` in `src/lib/drops.ts`.
  'sel.tooMany.1': 'Select just one card',
  'sel.tooMany.n': 'Select at most {n} cards',
  'sel.needMore': 'Select {n} card(s)',
  'sel.notThese': "Those cards can't go here",

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

  'rules.variation': 'Varianta',
  'rules.dealSize': 'Počet rozdaných karet',
  'rules.minSetSize': 'Nejmenší skupina',
  'rules.minRunSize': 'Nejkratší postupka',
  'rules.toGoDown': 'Pro vyložení',
  'rules.meldFloor': 'Minimální hodnota',
  'rules.discardPickup': 'Braní z odhazovacího balíčku',
  'rules.jokers': 'Žolíci',
  'rules.matchEnds': 'Konec zápasu',
  'rules.cards': '{n} karet',
  'rules.floorOff': 'Vypnuto',
  'rules.floorPoints': '{n}+ bodů za první kombinace',
  'rules.pickupTopOnly': 'Pouze vrchní karta',
  'rules.pickupAnyFromPile': 'Libovolná karta z balíčku (a vše nad ní)',
  'rules.pickupLocked': '{scope}, zamčeno do kola {n}',
  'rules.jokersRestricted': 'Nelze odhodit, kromě karty, která ukončí hru',
  'rules.jokersFree': 'Lze odhodit kdykoli',
  'rules.endsAfterDeals': 'Po {n} rozdáních',
  'rules.endsAtScore': 'První na {n} bodů',
  'rules.endsOnDeal': 'Po konci rozdání',

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

  'sel.tooMany.1': 'Vyber jen jednu kartu',
  'sel.tooMany.n': 'Vyber nejvýš {n} karet',
  'sel.needMore': 'Vyber {n} kartu/karty',
  'sel.notThese': 'Tyhle karty sem nepatří',

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
