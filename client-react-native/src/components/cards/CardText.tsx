import { Text, type TextProps } from 'react-native';

/**
 * Type printed on a card: a rank, a suit, an index.
 *
 * Pinned to the size the layout metrics give it, whatever the system's font
 * size. Every number a card's text is set in is derived from the card it sits
 * on — the plain face's index from the strip a closed hand shows, the corners
 * from the card's width (`src/lib/layout.ts`) — and the card's box does not
 * follow the system font size. Free scaling would grow a "10" past the strip
 * it is read in, under the next card, where it reads as an ace.
 *
 * Somebody who needs bigger cards has a setting for exactly that (Settings →
 * Accessibility → Card size), which grows the card *and* its type together.
 * Text off the card — panels, prose, every other screen — scales freely.
 *
 * Only native has a system font scale to refuse; on the web the browser's
 * zoom scales the card and its type as one, and this changes nothing.
 */
export function CardText(props: TextProps) {
  return <Text maxFontSizeMultiplier={1} {...props} />;
}
