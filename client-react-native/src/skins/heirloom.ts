import type { Skin } from '@/src/skins/types';

/**
 * A real deck on a real table.
 *
 * `casino` is a card *room* — green felt, gold trim, a look built out of the
 * furniture. This one is built out of the deck: ivory stock with the pips
 * where a printer would put them (`face: 'deluxe'`), court figures mirrored
 * about the waist, and a navy back with a gold frame. The table under it is
 * deliberately quieter than the casino's — a deep blue-slate baize with the
 * lamp hung low — because a busier felt competes with the cards, and here the
 * cards are the thing worth looking at.
 *
 * Palette notes, so nobody has to re-derive them:
 * - The baize is blue rather than green so the two felts are told apart at a
 *   glance in the switcher, and because red and black pips both sit well on
 *   it — a green felt pushes hearts towards brown.
 * - Ink is a true near-black (`#1b1b1f`) and the red a printer's vermilion
 *   (`#c8102e`, the red actually used on playing cards) rather than a screen
 *   red. On ivory stock a screen red reads as a warning light.
 * - `courtAccent` is the one colour a card carries beyond its ink: the gold a
 *   crown, a cap band and a jester's bells are drawn in.
 * - Gold is the trim, blue (`accent`) is the live drop target. Same division
 *   of labour as `casino`, and for the same reason — gold already means
 *   "armed", so a target needs a colour nothing else is using.
 */
export const heirloom: Skin = {
  id: 'heirloom',
  label: 'Heirloom',
  colors: {
    bg: '#0a1522',
    surface: 'rgba(10, 21, 34, 0.84)',
    border: 'rgba(214, 186, 122, 0.24)',
    text: '#f1ece0',
    muted: '#9aabc0',
    accent: '#6cc0ff',
    accentDim: '#2b5b86',
    accentButton: '#e3c178',
    onAccent: '#1d1604',
    danger: '#ff8f80',
    success: '#6ee2a4',
    gold: '#e8c579',
    cardBg: '#fbf7ec',
    cardBorder: '#cfc6ae',
  },
  table: {
    background: ['#0a1c2b', '#153b52', '#09161f'],
    edge: 'rgba(0, 0, 0, 0.42)',
    sheen: 'rgba(255, 250, 232, 0.045)',
    lamp: {
      inner: 'rgba(255, 243, 210, 0.11)',
      outer: 'rgba(0, 0, 0, 0.5)',
      cx: 0.5,
      cy: 0.36,
      r: 0.8,
    },
  },
  panel: {
    background: 'rgba(7, 17, 27, 0.7)',
    border: 'rgba(214, 186, 122, 0.2)',
    shadow: true,
    bevel: {
      highlight: 'rgba(232, 197, 121, 0.34)',
      shadow: 'rgba(0, 0, 0, 0.54)',
    },
  },
  card: {
    face: 'deluxe',
    // Barely a gradient: enough that the stock is not a flat fill, not enough
    // that a pip sits on a different colour at the top of the card than at
    // the bottom.
    faceGradient: ['#fffdf6', '#f2ecdb'],
    ink: '#1b1b1f',
    red: '#c8102e',
    courtAccent: '#c69a3f',
    selectedFace: '#fff6d8',
    jokerFace: '#fbf4e2',
    shadow: true,
    bevel: {
      highlight: '#fffef9',
      shadow: '#b3a98d',
    },
    back: {
      colors: ['#1f4a7a', '#12293f'],
      frame: 'rgba(232, 197, 121, 0.88)',
      emblem: '#e8c579',
    },
  },
  seats: {
    avatars: true,
  },
  deckStack: true,
  dropArmed: {
    borderStyle: 'dashed',
    borderColor: '#6cc0ff',
    backgroundColor: 'rgba(108, 192, 255, 0.16)',
  },
};
