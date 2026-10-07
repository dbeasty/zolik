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

import { isLastCardCode, parseLastCard, SHAPE_GLYPH } from '@/src/components/cards/lastCardArt';
import { currentDeck, GERMAN_SUIT_GLYPHS, germanIndex } from '@/src/lib/deck';
import { t } from '@/src/lib/i18n';

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

/**
 * A Last Card's index as text: its number, "+2", or a sign for an action.
 * The face draws its own glyphs; this is for the places a card is a line of
 * type — a narrow index, a collapsed panel.
 */
const LAST_CARD_INDEX: Record<string, string> = { S: '⤼', R: '⟲', W: '✦' };

function parseLastCardDisplay(card: string): CardDisplay | null {
  const c = parseLastCard(card);
  if (!c) return null;
  return {
    rank: LAST_CARD_INDEX[c.face] ?? c.face,
    suitSymbol: c.colour ? SHAPE_GLYPH[c.colour] : '',
    suit: c.colour ?? 'W',
    isRed: false,
    isJoker: false,
  };
}

export function parseCard(card: string): CardDisplay {
  if (isLastCardCode(card)) {
    const lc = parseLastCardDisplay(card);
    if (lc) return lc;
  }
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
  if (isLastCardCode(card)) return parseLastCardDisplay(card)?.rank ?? card;
  if (card.startsWith("JOKER")) return "JKR";
  if (!card.length) return "?";
  if (card[0] === "T") return "10";
  return card[0];
}

export function cardSuit(card: string): string {
  if (isLastCardCode(card)) return parseLastCardDisplay(card)?.suit ?? "W";
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
  if (isLastCardCode(card)) return lastCardText(card);
  const c = parseCard(card);
  if (c.isJoker) return 'Joker';
  // The same code on a German-suited table is a different card to look at —
  // "QS" is the svršek of leaves, not the queen of spades — so it is named
  // the way the face on the table draws it.
  if (currentDeck() === 'german') return `${germanIndex(c.rank)}${GERMAN_SUIT_GLYPHS[c.suit] ?? ''}`;
  return `${c.rank}${c.suitSymbol}`;
}

/**
 * Whether a string is one of the server's card codes.
 *
 * Deliberately strict, because this decides whether an arbitrary value gets
 * rewritten on its way to a player: two or three characters, a known rank and
 * a known suit, or a joker. A player named "7H" keeps their name.
 */
export function isCardCode(value: string): boolean {
  // Last Card's codes count only at a Last Card table: "W" is a card there
  // and a perfectly good name anywhere else.
  if (currentDeck() === 'lastcard' && isLastCardCode(value)) return true;
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

/**
 * A Last Card in a sentence: its colour's shape and what is printed on it —
 * "◆7", "▲ Skip", "Wild Draw Four". The shape carries the colour the way ♠
 * carries a suit, so no locale has to name it; the actions are named in the
 * player's language.
 */
export function lastCardText(card: string): string {
  const c = parseLastCard(card);
  if (!c) return card;
  switch (c.kind) {
    case 'wild':
      return t('lastcard.card.wild');
    case 'wildDrawFour':
      return t('lastcard.card.wildDrawFour');
    case 'skip':
      return `${SHAPE_GLYPH[c.colour]} ${t('lastcard.card.skip')}`;
    case 'reverse':
      return `${SHAPE_GLYPH[c.colour]} ${t('lastcard.card.reverse')}`;
    case 'drawTwo':
      return `${SHAPE_GLYPH[c.colour]} ${t('lastcard.card.drawTwo')}`;
  }
  return `${SHAPE_GLYPH[c.colour]}${c.face}`;
}
