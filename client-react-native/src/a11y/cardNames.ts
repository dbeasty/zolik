import { parseLastCard } from '@/src/components/cards/lastCardArt';
import { isCardCode, isTileCode } from '@/src/lib/cards';
import { currentDeck } from '@/src/lib/deck';
import { t } from '@/src/lib/i18n';

/**
 * A card as a screen reader should say it: "King of Hearts", "Over of
 * Leaves", "Teal Skip", "Red 7".
 *
 * `cardText` is for a card inside a sentence a sighted player reads — "K♥" —
 * and a glyph is the right thing to print there. Spoken, the same glyph is
 * read as "black heart suit" or skipped altogether, and the raw code the
 * board used to hand VoiceOver ("K, H") is worse. So this is its own
 * function, worded by the bundle: word order differs by language ("srdcový
 * král"), which is why the whole phrase is a template rather than two words
 * glued together.
 *
 * Anything that is not a card code comes back unchanged, the same contract
 * `cardText` keeps — a face-down back or a player's name is not rewritten.
 */
export function cardSpokenName(card: string): string {
  if (!card) return '';
  const deck = currentDeck();

  if (isTileCode(card)) {
    const [n, colour] = card.split('-');
    return t('a11y.tile.spoken', { colour: t(`a11y.tile.colour.${colour}`), n });
  }

  if (deck === 'lastcard') {
    const lc = parseLastCard(card);
    if (lc) {
      switch (lc.kind) {
        case 'wild':
          return t('lastcard.card.wild');
        case 'wildDrawFour':
          return t('lastcard.card.wildDrawFour');
        case 'skip':
        case 'reverse':
        case 'drawTwo':
          return t('a11y.lastcard.spoken', {
            colour: t(`lastcard.colourName.${lc.colour}`),
            face: t(`lastcard.card.${lc.kind}`),
          });
        default:
          return t('a11y.lastcard.spoken', {
            colour: t(`lastcard.colourName.${lc.colour}`),
            face: lc.face,
          });
      }
    }
  }

  if (!isCardCode(card)) return card;
  if (card.startsWith('JOKER')) return t('a11y.card.joker');

  const rank = card[0];
  const suit = card[1];
  if (deck === 'german') {
    const rankKey = rank === 'J' || rank === 'Q' ? `a11y.rank.german.${rank}` : `a11y.rank.${rank}`;
    return t('a11y.card.spoken', { rank: t(rankKey), suit: t(`suit.german.${suit}`) });
  }
  return t('a11y.card.spoken', { rank: t(`a11y.rank.${rank}`), suit: t(`suit.${suit}`) });
}

/** Several cards as one phrase: "Five of Hearts, Six of Hearts, Seven of Hearts". */
export function cardsSpoken(cards: readonly string[]): string {
  return cards.map(cardSpokenName).join(', ');
}
