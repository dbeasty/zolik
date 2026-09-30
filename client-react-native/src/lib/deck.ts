import { createContext, createElement, useContext, useEffect, type ReactNode } from 'react';

import type { CardDeck } from '@/src/api/matchTypes';
import { t } from '@/src/lib/i18n';

/**
 * Which pack the cards on screen belong to.
 *
 * Every game until Mariáš was dealt from the French pack, so a card code was
 * enough to draw a card: "QS" is the queen of spades. Mariáš and Prší are
 * dealt from the German-suited one, where the same code is the svršek of
 * leaves. The server says which pack a match uses (`MatchState.deck`), the
 * match and replay screens put it here, and everything that draws or names a
 * card reads it — `CardView` for the face, `cardText` for a card in a
 * sentence.
 *
 * A pack, never a game: the shell learns that a German deck exists, not that
 * Mariáš does.
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
