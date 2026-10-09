import { t } from '@/src/lib/i18n';
import type { Skin } from '@/src/skins/types';

/**
 * The high-contrast board (docs/accessibility-plan.md, Phase 5).
 *
 * Worn automatically when a player asks for contrast — Settings →
 * Accessibility → High contrast set to On, or left on Automatic on a device
 * whose system asks for more (`prefers-contrast: more` or forced colours on
 * the web, "Increase contrast" on iOS, high-text-contrast on Android; see
 * `useSkin`). It is also a skin like any other in the switcher, for someone
 * who simply likes it.
 *
 * What makes it this skin rather than a darker `classic`:
 * - Every text colour holds 7:1 against every surface it sits on (WCAG
 *   AAA), not the 4.5:1 the other skins are held to. `contrast.test.ts`
 *   checks it rather than this comment promising it.
 * - Nothing is textured, washed or lit: one flat black felt, no lamp, no
 *   sheen, no gradient on a face, no bevel, no shadow. A gradient is a
 *   background whose contrast with the text on it changes from top to
 *   bottom, and a shadow is an edge that is only implied.
 * - Edges are solid and stated: panels (and so melds) carry a white border,
 *   cards a black one on white stock, so a card is told from the felt by
 *   its fill and from its neighbour in a fan by its border.
 * - The plain face: one index, as large as the card takes. The face the
 *   phone gets, and here on every screen, because a large bold index is
 *   what low vision reads a card by.
 *
 * Colour and style only, as for every skin — nothing here can change a
 * size, so the board is laid out exactly as under any other.
 */
export const contrast: Skin = {
  id: 'contrast',
  // A getter so the switcher shows it in the current language — the other
  // skins' names are proper names of a look ("Casino") and stay as written,
  // but this one says what it does.
  get label() {
    return t('a11y.skin.highContrast');
  },
  colors: {
    bg: '#000000',
    surface: '#000000',
    border: '#d6d6d6',
    text: '#ffffff',
    muted: '#d6d6d6',
    accent: '#7cc4ff',
    accentDim: '#00336b',
    accentButton: '#ffd60a',
    onAccent: '#000000',
    danger: '#ff9a9a',
    success: '#6ff59a',
    gold: '#ffd60a',
    cardBg: '#ffffff',
    cardBorder: '#000000',
  },
  table: {
    background: ['#000000'],
  },
  panel: {
    background: '#000000',
    border: '#ffffff',
    shadow: false,
  },
  card: {
    face: 'plain',
    ink: '#000000',
    red: '#a3001b',
    fourColour: { diamonds: '#0030a8', clubs: '#005c1c', bells: '#6b3a00' },
    selectedFace: '#fff3a0',
    jokerFace: '#fff3a0',
    shadow: false,
    back: {
      // One colour twice: the type asks for a gradient's two stops, and this
      // skin has no gradients.
      colors: ['#00336b', '#00336b'],
      frame: '#ffffff',
      emblem: '#ffffff',
    },
  },
  seats: {
    avatars: false,
  },
  deckStack: false,
  dropArmed: {
    borderStyle: 'dashed',
    borderColor: '#ffd60a',
    backgroundColor: 'rgba(255, 214, 10, 0.2)',
  },
};
