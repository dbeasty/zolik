import { createContext, createElement, useContext, useEffect, type ReactNode } from 'react';

import type { CardDeck } from '@/src/api/matchTypes';
import { t } from '@/src/lib/i18n';

/**
 * Which pack the cards on screen belong to.
 *
 * Every game until Mariáš was dealt from the French pack, so a card code was
 * enough to draw a card: "QS" is the queen of spades. Mariáš and Prší are
 * dealt from the German-suited one, where the same code is the svršek of
 * leaves; Siete y Medio from the Spanish forty, where it is the caballo of
 * espadas. The server says which pack a match uses (`MatchState.deck`), the
 * match and replay screens put it here, and everything that draws or names a
 * card reads it — `CardView` for the face, `cardText` for a card in a
 * sentence.
 *
 * A pack, never a game: the shell learns that a Spanish deck exists, not that
 * Siete y Medio does.
 */
export type Deck = 'french' | CardDeck;

const DeckContext = createContext<Deck>('french');

/**
 * The pack sentences are written in.
 *
 * A module-level value as well as a context, because a card named inside a
 * sentence is rendered by `labels.ts`, which is plain functions called from a
 * dozen components that have no reason to know about decks. The provider
 * below keeps the two in step; only one match is ever on screen.
 */
let activeDeck: Deck = 'french';

export function currentDeck(): Deck {
  return activeDeck;
}

/** Sets the pack sentences are written in. The provider's job; exported for tests. */
export function setActiveDeck(deck: Deck): void {
  activeDeck = deck;
}

export function DeckProvider({ deck, children }: { deck: Deck | undefined; children: ReactNode }) {
  const value: Deck = deck ?? 'french';
  setActiveDeck(value);
  useEffect(() => {
    setActiveDeck(value);
    return () => setActiveDeck('french');
  }, [value]);
  return createElement(DeckContext.Provider, { value }, children);
}

export function useDeck(): Deck {
  return useContext(DeckContext);
}

/**
 * A German card's corner index for a rank as `parseCard` reports it: the
 * numbers as they are, and the spodek, svršek, král and eso by the letters
 * the player's language uses for them (U, O, K, A in English and German).
 */
export function germanIndex(rank: string): string {
  switch (rank) {
    case 'J':
      return t('card.german.J');
    case 'Q':
      return t('card.german.Q');
    case 'K':
      return t('card.german.K');
    case 'A':
      return t('card.german.A');
  }
  return rank;
}

/**
 * A Spanish card's corner index for a rank as `parseCard` reports it: the
 * numbers as they are, the ace as the 1 a baraja prints, and the sota, the
 * caballo and the rey by their Spanish initials.
 *
 * Fixed letters rather than localised ones, which is the opposite of
 * `germanIndex` and deliberate: a Czech player reads "Sv" on a svršek because
 * a Czech-printed German pack prints Sv, while every baraja española ever
 * printed says S, C and R whoever is holding it. Those three letters are part
 * of the pack, not of the reader's language.
 */
export function spanishIndex(rank: string): string {
  switch (rank) {
    case 'A':
      return '1';
    case 'J':
      return 'S';
    case 'Q':
      return 'C';
    case 'K':
      return 'R';
  }
  return rank;
}

/**
 * The Spanish suits as they read inside a sentence.
 *
 * The French glyphs, and not for want of trying: these are the four suits the
 * French ones descend from — copas became hearts, oros diamonds, bastos clubs,
 * espadas spades — and the forty-card pack maps onto the same codes for
 * exactly that reason. A coin and a cudgel have no character every platform
 * draws, and a tofu box in a sentence about a card is worse than the French
 * form of the same suit. The card *faces* draw the Spanish signs
 * (`spanishArt.ts`), which is where a player looks.
 */
export const SPANISH_SUIT_GLYPHS: Record<string, string> = {
  H: '♥',
  D: '♦',
  C: '♣',
  S: '♠',
};

/**
 * The German suits as they read inside a sentence. There are no Unicode suit
 * characters for bells, acorns and leaves, so these are the nearest pictures
 * every platform has.
 */
export const GERMAN_SUIT_GLYPHS: Record<string, string> = {
  H: '♥',
  D: '🔔',
  C: '🌰',
  S: '🍃',
};
