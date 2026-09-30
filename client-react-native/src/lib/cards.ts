/**
 * How a card string is drawn.
 *
 * Presentation only, and that is the whole file now. It used to also hold
 * `autoOrganizeHand`, `contractLabel`, `rulesSummaryLines` and
 * `profileDisplayName` — sorting a rummy hand, naming a rummy contract,
 * re-typing each rummy profile's constants — every one of which was a rule
 * living in a client. They went with the screen that needed them.
 *
 * What is left knows that "TD" is the ten of diamonds and that diamonds are
 * red. That is a fact about a deck, not about a game: Žolíky, Canasta and
 * Hold'em all draw the same card the same way, which is why one CardView can
 * serve all of them.
 */

export type CardDisplay = {
  rank: string;
  suitSymbol: string;
  suit: string;
  isRed: boolean;
  isJoker: boolean;
};

const SUIT_SYMBOLS: Record<string, string> = {
  H: "♥",
  D: "♦",
  C: "♣",
  S: "♠",
};

export function parseCard(card: string): CardDisplay {
  if (card.startsWith("JOKER")) {
    return {
      rank: "JKR",
      suitSymbol: "★",
      suit: "",
      isRed: false,
      isJoker: true,
    };
  }
  const suit = cardSuit(card);
  return {
    rank: displayRank(card),
    suitSymbol: SUIT_SYMBOLS[suit] ?? "?",
    suit,
    isRed: suit === "H" || suit === "D",
    isJoker: false,
  };
}

export function displayRank(card: string): string {
  if (card.startsWith("JOKER")) return "JKR";
  if (!card.length) return "?";
  if (card[0] === "T") return "10";
  return card[0];
}

export function cardSuit(card: string): string {
  if (card.length < 2) return "S";
  // "TD" is a ten, so its suit is the second character rather than the last —
  // which is the same thing for a two-character card and matters only because
  // a rank could one day be two characters.
  if (card[0] === "T") return card[1];
  return card[card.length - 1];
}

/**
 * A card as it should read inside a sentence — "J♠", "10♦", "Joker".
 *
 * Wherever a card ends up in prose rather than on a card face: the rule that
 * says which card you owe your lay-down, the remedy that names it, the mark
 * on the card itself. Those all travel as the server's own card codes, and
 * "JS came off the discard pile" is a sentence about a card nobody at the
 * table can see.
 */
export function cardText(card: string): string {
  if (!isCardCode(card)) return card;
  const c = parseCard(card);
  return c.isJoker ? "Joker" : `${c.rank}${c.suitSymbol}`;
}

/**
 * Whether a string is one of the server's card codes.
 *
 * Deliberately strict, because this decides whether an arbitrary value gets
 * rewritten on its way to a player: two or three characters, a known rank and
 * a known suit, or a joker. A player named "7H" keeps their name.
 */
export function isCardCode(value: string): boolean {
  if (value.startsWith("JOKER")) return true;
  if (value.length < 2 || value.length > 2) return false;
  const [rank, suit] = [value[0], value[1]];
  return "A23456789TJQK".includes(rank) && "HDCS".includes(suit);
}

/**
 * The four Rummy Tiles colours, each as a mark that carries its colour in the
 * glyph itself. A tile in prose is a number and a colour, and a coloured mark
 * is to a tile what ♠ is to a card: no locale has to word it, and a run reads
 * as one — "🔴10 🔴11 🔴12".
 */
const TILE_COLOURS: Record<string, string> = {
  R: "🔴",
  B: "🔵",
  O: "🟠",
  K: "⚫",
};

const TILE_CODE = /^(1[0-3]|[1-9])-[RBOK]$/;

/**
 * Whether a string is one of the server's tile codes: a number from 1 to 13,
 * a hyphen and a colour, "7-R". As strict as `isCardCode`, and for the same
 * reason. The hyphen keeps the two notations apart, so no value is both. The
 * two jokers share the cards' `JOKER1`/`JOKER2` spelling, so `isCardCode`
 * already covers them.
 */
export function isTileCode(value: string): boolean {
  return TILE_CODE.test(value);
}

/** A tile as it should read inside a sentence — "🔴7", "⚫12". */
export function tileText(tile: string): string {
  if (!isTileCode(tile)) return tile;
  const [n, colour] = tile.split("-");
  return `${TILE_COLOURS[colour]}${n}`;
}
