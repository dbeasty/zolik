import { FACES, PRINTED, type Face, type FaceNode } from '@/src/cards/vector/faces';
import { parseCard } from '@/src/lib/cards';
import { SKINS } from '@/src/skins';

/**
 * The deck is generated art, so what is worth testing is not how a card looks
 * — no assertion can tell a good king from a bad one — but that the deck
 * *covers the game*. A missing face is a blank card at a real table, and it
 * would appear only when somebody was dealt that exact card.
 *
 * Every card code the server can send, built the way the server builds them:
 * a rank character and a suit, plus the joker. `parseCard` turns those into
 * what `VectorFace` looks a face up by, so this walks the same path a card
 * takes on its way to the screen rather than re-deriving the keys.
 */
const RANK_CODES = ['A', '2', '3', '4', '5', '6', '7', '8', '9', 'T', 'J', 'Q', 'K'];
const SUIT_CODES = ['H', 'D', 'C', 'S'];

const DECK = [...RANK_CODES.flatMap((r) => SUIT_CODES.map((s) => `${r}${s}`)), 'JOKER'];

const keyFor = (code: string) => {
  const d = parseCard(code);
  return d.isJoker ? 'JKR' : `${d.rank}${d.suit}`;
};

const paths = (face: Face): { d: string; c: string }[] => {
  const out: { d: string; c: string }[] = [];
  (function walk(nodes: readonly FaceNode[]) {
    for (const n of nodes) {
      if ('d' in n) out.push(n);
      else walk(n.k);
    }
  })(face.nodes);
  return out;
};

describe('the vector deck', () => {
  it('has a face for every card the game can deal', () => {
    const missing = DECK.filter((code) => !FACES[keyFor(code)]);
    expect(missing).toEqual([]);
    expect(DECK).toHaveLength(53);
  });

  it('has no faces the game will never ask for', () => {
    const dealt = new Set(DECK.map(keyFor));
    expect(Object.keys(FACES).filter((k) => !dealt.has(k))).toEqual([]);
  });

  it('draws every card, and draws it in the five inks a skin can name', () => {
    // Named per card rather than asserted in a loop of anonymous numbers: a
    // failure here should say *which* card lost its line work.
    const blank = Object.entries(FACES).filter(([, f]) => paths(f).length === 0);
    expect(blank.map(([k]) => k)).toEqual([]);
    const strange = Object.entries(FACES).flatMap(([k, f]) =>
      paths(f)
        .map((p) => p.c)
        .filter((c) => !(c in PRINTED))
        .map((c) => `${k}: ${c}`),
    );
    expect(strange).toEqual([]);
  });

  it('is drawn to the card, so one face fits every card size', () => {
    for (const face of Object.values(FACES)) expect(face.viewBox).toBe('0 0 63 88');
  });

  it('carries no stock of its own, so the skin underneath shows through', () => {
    // The art's white rounded rect is dropped at generation time (see
    // `scripts/gen-card-faces.js`). If one came back, a selected card would
    // stop looking selected — the tint is painted by `CardView`, under this.
    for (const [key, face] of Object.entries(FACES)) {
      const stock = paths(face).filter((p) => p.c === 'stock');
      // The jokers' masks are genuinely white; nothing else is.
      if (key !== 'JKR') expect(`${key}: ${stock.length}`).toBe(`${key}: 0`);
    }
  });
});

describe('a skin using it', () => {
  it('names all five inks, or none of them', () => {
    for (const skin of SKINS) {
      const palette = skin.card.cardPalette;
      if (!palette) continue;
      expect(`${skin.id}: ${Object.keys(palette).sort().join(',')}`).toBe(
        `${skin.id}: ${Object.keys(PRINTED).sort().join(',')}`,
      );
    }
  });

  it('keeps the deck legible: ink and red are not the stock they sit on', () => {
    for (const skin of SKINS) {
      if (skin.card.face !== 'vector') continue;
      const palette = skin.card.cardPalette ?? PRINTED;
      const invisible = (['ink', 'red', 'gold', 'navy'] as const).filter(
        (token) => palette[token] === palette.stock,
      );
      expect(`${skin.id}: ${invisible.join(',')}`).toBe(`${skin.id}: `);
    }
  });
});
