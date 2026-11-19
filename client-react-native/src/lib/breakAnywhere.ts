import { Platform, type TextStyle } from 'react-native';

/**
 * Lets a line break inside a word when it has no space to break at.
 *
 * Player names are typed by players, and some are one long word: an email
 * address used as a username ("msft_davja@hotmail.com") has no space in it at
 * all. A browser will not break such a word, so in a narrow column it ran out
 * of its box and over the scores beside it. Native text already breaks a word
 * that cannot fit, so this only changes the web.
 *
 * Spread into a text style, never used to shorten the name itself: the name is
 * what the player chose, and a screen reader still reads it whole.
 */
export const breakAnywhere: TextStyle =
  Platform.OS === 'web' ? ({ overflowWrap: 'anywhere' } as unknown as TextStyle) : {};
