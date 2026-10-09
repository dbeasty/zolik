import { cardSuit } from '@/src/lib/cards';
import type { Skin } from '@/src/skins/types';

/**
 * The four-colour deck, as a skin seen through one card's suit.
 *
 * Every face this client draws — the plain index, the rich corners, the
 * deluxe pips, the engraved vector deck — already colours a card from two
 * inks: `card.ink` for a black suit and `card.red` for a red one. So the
 * four-colour deck is not a fifth way of drawing a card; it is those same
 * faces handed a skin in which, *for this card*, the red is blue (a diamond)
 * or the black is green (a club). Nothing that draws a face has to know the
 * setting exists, and a face added later gets it for free.
 *
 * The engraved deck's own palette is restated the same way, so a club's line
 * work is green on every face rather than only on the ones that read `ink`.
 *
 * Cached per skin and suit: the returned object is what `CardView` memoises
 * its styles on, so it has to be the same object on every render rather than
 * an equal one.
 */
const cache = new WeakMap<Skin, Map<string, Skin>>();

export function fourColourSkin(skin: Skin, suit: string): Skin {
  if (suit !== 'D' && suit !== 'C') return skin;
  let bySuit = cache.get(skin);
  if (!bySuit) {
    bySuit = new Map();
    cache.set(skin, bySuit);
  }
  const hit = bySuit.get(suit);
  if (hit) return hit;
  const inks = skin.card.fourColour;
  const swap = suit === 'D' ? { red: inks.diamonds } : { ink: inks.clubs };
  const derived: Skin = {
    ...skin,
    card: {
      ...skin.card,
      ...swap,
      // A skin that draws the engraved deck as printed (no palette of its
      // own) has nothing here to swap; only heirloom draws that deck today,
      // and it restates the palette.
      cardPalette: skin.card.cardPalette ? { ...skin.card.cardPalette, ...swap } : undefined,
    },
  };
  bySuit.set(suit, derived);
  return derived;
}

/**
 * The colour a German card's index is printed in under the four-colour deck.
 *
 * The German pack is four-coloured already — red hearts, gold bells, brown
 * acorns, green leaves — but its *indices* are printed in each suit's dark
 * line colour, and two of those (bells and acorns) are both brown. So the
 * setting gives the bells an amber of their own and the acorns the skin's
 * black ink; hearts keep the skin's red and leaves the clubs' green. The
 * drawn suit marks are untouched: they are pictures, and already differ.
 *
 * `undefined` for a suit whose index keeps its usual colour.
 */
export function germanFourColourInk(skin: Skin, suit: string): string | undefined {
  switch (suit) {
    case 'H':
      return skin.card.red;
    case 'D':
      return skin.card.fourColour.bells;
    case 'C':
      return skin.card.ink;
    case 'S':
      return skin.card.fourColour.clubs;
    default:
      return undefined;
  }
}

/**
 * Which of the four-colour deck's two extra inks a card takes, if either —
 * for text that names a card outside its face (a panel's digest), which has
 * to pick its own colours for the surface it is on.
 */
export function fourColourInkOf(card: string): 'diamonds' | 'clubs' | undefined {
  const s = cardSuit(card);
  return s === 'D' ? 'diamonds' : s === 'C' ? 'clubs' : undefined;
}
