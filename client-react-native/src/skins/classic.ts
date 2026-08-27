import type { Skin } from '@/src/skins/types';

/**
 * The original look, kept exactly: flat dark panels, plain card faces, no
 * decoration. This skin is the compatibility baseline — `src/theme.ts` builds
 * its static exports from this palette, so every screen outside the match
 * (lobby, auth, rules) is unchanged by the skin system existing.
 */
export const classic: Skin = {
  id: 'classic',
  label: 'Classic',
  colors: {
    bg: '#0f1419',
    surface: '#1a2332',
    border: '#2d3a4f',
    text: '#e8eef5',
    muted: '#8b9cb3',
    accent: '#3d8bfd',
    accentDim: '#2563c4',
    // Primary buttons sit on a very dark background, where a mid blue reads as
    // "there is a button here" rather than "this button is live". They use this
    // lighter blue instead, with dark text (`onAccent`) — white on a blue this
    // light is under 3:1, dark text on it is ~9:1. Disabled is the same fill at
    // 0.4 opacity, so enabled/disabled stays an obvious brightness step.
    accentButton: '#7ab8ff',
    onAccent: '#0b1220',
    danger: '#f87171',
    success: '#4ade80',
    gold: '#fbbf24',
    cardBg: '#f8fafc',
    cardBorder: '#cbd5e1',
  },
  table: {
    background: ['#0f1419'],
  },
  panel: {
    background: '#1a2332',
    border: '#2d3a4f',
    shadow: false,
  },
  card: {
    face: 'plain',
    ink: '#1e293b',
    red: '#dc2626',
    selectedFace: '#fffbeb',
    jokerFace: '#fef3c7',
    shadow: false,
    back: {
      colors: ['#2563c4', '#2563c4'],
      frame: '#2d3a4f',
      emblem: '#e8eef5',
    },
  },
  seats: {
    avatars: false,
  },
  deckStack: false,
  dropArmed: {
    borderStyle: 'dashed',
    borderColor: '#fbbf24',
    backgroundColor: 'rgba(251, 191, 36, 0.14)',
  },
};
