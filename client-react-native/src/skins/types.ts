/**
 * What a UI skin is allowed to decide.
 *
 * A skin is a *look*, and only a look: every colour on the board, whether a
 * card face is drawn plainly or with the full two-corner-and-a-pip treatment,
 * whether panels cast shadows, whether the draw pile is a flat box or a
 * little stack of card backs. It decides nothing about layout, nothing about
 * what is on screen, and nothing about the game — the shell stays as
 * game-blind under a skin as it was before skins existed, and every testID
 * and drop-target measurement is identical under every skin.
 *
 * That last part is a rule, not an accident: a skin may change colour,
 * border style, fill, shadow and decoration, but never the *size* of
 * anything a drag is hit-tested against. The no-size-change-on-highlight
 * discipline (`Panel`, `ZoneView`, `CardView`) extends to skins whole.
 */

/** The palette every component reads. Same keys as the original `theme.colors`. */
export type SkinColors = {
  bg: string;
  surface: string;
  border: string;
  text: string;
  muted: string;
  accent: string;
  accentDim: string;
  accentButton: string;
  onAccent: string;
  danger: string;
  success: string;
  gold: string;
  cardBg: string;
  cardBorder: string;
};

export type Skin = {
  id: string;
  /** Shown on the skin switcher. */
  label: string;
  colors: SkinColors;
  /**
   * The surface behind the whole board. One colour renders flat; two or more
   * render as a vertical gradient — the felt. `edge`, when present, is washed
   * in from the left and right sides so the middle of the table reads as
   * sitting under the light.
   */
  table: {
    background: string[];
    edge?: string;
    /**
     * A diagonal wash of light across the felt, as if the lamp sat just off
     * the top-left corner. Colour only — painted behind everything, catches
     * nothing.
     */
    sheen?: string;
  };
  panel: {
    background: string;
    border: string;
    /** Panels lift off the felt with a soft shadow. */
    shadow: boolean;
    /**
     * Fake depth on the panel's own border: the lit edges (top and left)
     * take `highlight`, the shaded ones (bottom and right) take `shadow`.
     * Border *colours* only — the width never changes, and a panel lit up
     * as a drop target keeps its plain drop colours instead (per-side
     * colours would beat the highlight's all-side one).
     */
    bevel?: { highlight: string; shadow: string };
    // No typography knobs: a wider or taller title is a *layout* change (two
    // spread panels stop fitting one phone row), and a skin may never change
    // a size — see the file comment. Spaced-capitals titles were tried and
    // reverted for exactly that.
  };
  card: {
    /**
     * 'plain' is the original card: rank top-left, one pip centered.
     * 'rich' is the full treatment: indices in two corners (the second one
     * upside down, the way a real card reads from either end), a large
     * centre pip, and a medallion for the court cards.
     */
    face: 'plain' | 'rich';
    /** Top-to-bottom wash across a rich face; ignored for 'plain'. */
    faceGradient?: [string, string];
    ink: string;
    red: string;
    selectedFace: string;
    jokerFace: string;
    /** Cards cast a small shadow, and the one being dragged casts a bigger one. */
    shadow: boolean;
    /**
     * Fake thickness on the card's own 2px border: lit top/left edges,
     * shaded bottom/right ones — the way a stacked paper card catches the
     * lamp. Colours only, never a width, and a selected card keeps its
     * solid selection border instead. Also switches on the card back's
     * gloss, so face and back agree about the light.
     */
    bevel?: { highlight: string; shadow: string };
    /** The face-down side, for draw piles (and any future flight animation). */
    back: {
      /** Corner-to-corner gradient. */
      colors: [string, string];
      frame: string;
      emblem: string;
    };
  };
  seats: {
    /** Each seat gets an initial-circle avatar before the name. */
    avatars: boolean;
  };
  /** The face-down stack is drawn as a small 3D pile of card backs. */
  deckStack: boolean;
  /**
   * The exact spot a dragged card will land — same contract as the original
   * `theme.dropArmed`: colour, style and fill only, never a size.
   */
  dropArmed: {
    borderStyle: 'dashed';
    borderColor: string;
    backgroundColor: string;
  };
};
