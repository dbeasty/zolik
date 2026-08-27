import { StyleSheet } from 'react-native';

import { classic } from '@/src/skins/classic';

/**
 * The static palette, kept for every screen *outside* the match: lobby,
 * auth, rules, stats. Those screens have one look, and it is the classic
 * skin's — the values live in `src/skins/classic.ts` now, so the skin
 * system and this file can never disagree about what "classic" means.
 *
 * The match screen and everything on the board read `useSkin()` instead,
 * which serves the same shape per active skin.
 */
export const colors = classic.colors;

// The exact spot a dragged card will land, drawn identically everywhere a
// card can be dropped — the hand's own reorder gap and every meld or zone on
// the board — so "this is where it goes" reads the same regardless of what
// kind of space it is. A wash rather than a solid fill: filled in solid it
// reads as a card already sitting there, which is the one thing it is not.
// (Skinned components use `skin.dropArmed`, same contract per skin.)
export const dropArmed = classic.dropArmed;

/**
 * The layer a card being carried is lifted onto, and everything it is drawn
 * inside of with it.
 *
 * A dragged card stays a child of the hand it came from for the whole
 * gesture — it has to, because moving its node loses the pointer
 * gesture-handler captured — so it is painted inside the hand's own box, and
 * the panels that come after the hand on the board paint over it. A card
 * carried down onto a meld went *behind* the meld, sliced in half by its
 * edge.
 *
 * `zIndex` on the card alone cannot fix that. Every `View` establishes its
 * own stacking context — on react-native-web literally, via a base style of
 * `position: relative; z-index: 0`, and on iOS and Android by the same
 * siblings-only rule — so a `zIndex` ranks a card against its siblings and
 * against nothing else. Getting the card above the board means raising every
 * box between it and the board, each one over *its* siblings. Hence one
 * number applied at every link in that chain, and only while a card is
 * actually in flight.
 */
export const dragLayer = { zIndex: 100 } as const;

export const shared = StyleSheet.create({
  screen: {
    flex: 1,
    backgroundColor: colors.bg,
    padding: 16,
  },
  title: {
    fontSize: 22,
    fontWeight: '700',
    color: colors.text,
    marginBottom: 8,
  },
  subtitle: {
    fontSize: 14,
    color: colors.muted,
    marginBottom: 16,
  },
  card: {
    backgroundColor: colors.surface,
    borderRadius: 12,
    borderWidth: 1,
    borderColor: colors.border,
    padding: 16,
    marginBottom: 12,
  },
  button: {
    backgroundColor: colors.accentButton,
    paddingVertical: 14,
    paddingHorizontal: 20,
    borderRadius: 10,
    marginBottom: 10,
    alignItems: 'center',
  },
  buttonSecondary: {
    backgroundColor: colors.surface,
    borderWidth: 1,
    borderColor: colors.border,
  },
  buttonText: {
    color: colors.onAccent,
    fontSize: 16,
    fontWeight: '600',
  },
  buttonTextSecondary: {
    color: colors.text,
  },
  input: {
    backgroundColor: colors.surface,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 10,
    padding: 12,
    color: colors.text,
    fontSize: 16,
    marginBottom: 12,
  },
  error: {
    color: colors.danger,
    marginBottom: 8,
  },
  status: {
    color: colors.muted,
    fontSize: 13,
    marginTop: 8,
  },
  // A more prominent callout for rule violations (invalid meld, joker
  // discard, etc.) than plain `error` text — sits right under the turn/phase
  // status line so the "why" is impossible to miss.
  errorBanner: {
    flexDirection: 'row',
    alignItems: 'flex-start',
    gap: 6,
    backgroundColor: 'rgba(248, 113, 113, 0.12)',
    borderWidth: 1,
    borderColor: colors.danger,
    borderRadius: 8,
    paddingVertical: 8,
    paddingHorizontal: 10,
    marginTop: 8,
  },
  errorBannerIcon: {
    color: colors.danger,
    fontSize: 15,
    lineHeight: 18,
  },
  errorBannerText: {
    color: colors.danger,
    fontSize: 13,
    lineHeight: 18,
    flex: 1,
  },
});
