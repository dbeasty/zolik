import type { Skin } from '@/src/skins/types';

/**
 * The card-room look: green felt under a light, dark glass panels with gold
 * trim, cards with proper faces and a patterned back. The default skin — it
 * is what the product looks like; `classic` is the plain fallback.
 *
 * Palette notes, so nobody has to re-derive them:
 * - The felt runs darker at the top and bottom than in the middle band, which
 *   is as close as two linear gradients get to a pool of lamplight.
 * - Gold (`accentButton`, `gold`, panel borders) is the trim colour. The live
 *   drop target stays *blue* (`accent`) on purpose: gold already means
 *   "selected / armed", and on a green felt a blue outline is the one colour
 *   in play that nothing else uses.
 * - Buttons are gold with near-black text (~10:1); the old blue-on-dark
 *   button reads as chrome, gold reads as a chip you're meant to touch.
 */
export const casino: Skin = {
  id: 'casino',
  label: 'Casino',
  colors: {
    bg: '#0a2417',
    surface: 'rgba(9, 26, 18, 0.82)',
    border: 'rgba(228, 198, 124, 0.26)',
    text: '#f4eedd',
    muted: '#a4b8a8',
    accent: '#63b3ff',
    accentDim: '#2c5f8f',
    accentButton: '#ecc772',
    onAccent: '#241905',
    danger: '#ff8a7a',
    success: '#6fe39a',
    gold: '#f0c75e',
    cardBg: '#fdfaf1',
    cardBorder: '#d9d0ba',
  },
  table: {
    background: ['#0b2c1b', '#175434', '#0a2415'],
    edge: 'rgba(0, 0, 0, 0.38)',
    sheen: 'rgba(255, 252, 235, 0.05)',
  },
  panel: {
    background: 'rgba(6, 21, 14, 0.66)',
    border: 'rgba(228, 198, 124, 0.22)',
    shadow: true,
    bevel: {
      highlight: 'rgba(240, 214, 148, 0.38)',
      shadow: 'rgba(0, 0, 0, 0.5)',
    },
  },
  card: {
    face: 'rich',
    faceGradient: ['#fffef8', '#f0e9d4'],
    ink: '#232c3b',
    red: '#bf2138',
    selectedFace: '#fff4cf',
    jokerFace: '#f9efd7',
    shadow: true,
    bevel: {
      highlight: '#fffef9',
      shadow: '#b9ad8e',
    },
    back: {
      colors: ['#8a2433', '#54121e'],
      frame: 'rgba(240, 199, 94, 0.85)',
      emblem: '#f0c75e',
    },
  },
  seats: {
    avatars: true,
  },
  deckStack: true,
  dropArmed: {
    borderStyle: 'dashed',
    borderColor: '#f0c75e',
    backgroundColor: 'rgba(240, 199, 94, 0.16)',
  },
};
