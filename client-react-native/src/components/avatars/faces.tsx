import { Circle, Ellipse, G, Path, Rect } from 'react-native-svg';

/**
 * The figures, drawn once each in a 100×100 box and clipped to the circle by
 * the caller.
 *
 * One flat ink colour per figure rather than shading: at 40 pixels a portrait
 * is read as a silhouette and nothing else survives, and a silhouette that
 * stays crisp at 26 (the collapsed rail) is worth more than one that looks
 * better only in the picker.
 *
 * People are head-and-shoulders; machines are a head filling more of the frame,
 * which is most of what makes the two read as different sorts of thing before
 * any detail lands.
 */

type Figure = { ink: string; face?: string };

/* ── people ─────────────────────────────────────────────────────────────── */

/**
 * The body every portrait sits on: clothes in the ink, a neck in the face
 * tone. Drawn before the head, so the head laps over the neck rather than
 * floating above a gap.
 */
function Body({ ink, face }: Figure) {
  return (
    <G>
      <Rect x="42" y="56" width="16" height="18" fill={face} />
      <Path d="M16 100c0-19 15-31 34-31s34 12 34 31z" fill={ink} />
    </G>
  );
}

/** The head itself, in the face tone the hair is drawn over. */
function Head({ face }: { face?: string }) {
  return (
    <G>
      <Circle cx="50" cy="42" r="21" fill={face} />
      {/* Ears, so a close-cropped head does not read as an egg. */}
      <Circle cx="28" cy="44" r="4.5" fill={face} />
      <Circle cx="72" cy="44" r="4.5" fill={face} />
    </G>
  );
}

export function PersonAmber({ ink, face }: Figure) {
  return (
    <G>
      <Body ink={ink} face={face} />
      <Head face={face} />
      {/* Short and swept, sitting well down the brow. */}
      <Path d="M29 42c0-15 9-24 21-24s21 9 21 24c-3-9-7-13-11-13-5 0-7 4-16 4-6 0-11 3-15 9z" fill={ink} />
    </G>
  );
}

export function PersonViolet({ ink, face }: Figure) {
  return (
    <G>
      <Body ink={ink} face={face} />
      <Head face={face} />
      {/* Long, falling past the jaw on both sides. */}
      <Path d="M27 46c0-18 10-28 23-28s23 10 23 28v22h-8V44c0-9-6-14-15-14s-15 5-15 14v24h-8z" fill={ink} />
      <Path d="M31 34c4-9 11-13 19-13s15 4 19 13c-5-4-11-6-19-6s-14 2-19 6z" fill={ink} />
    </G>
  );
}

export function PersonTeal({ ink, face }: Figure) {
  return (
    <G>
      <Body ink={ink} face={face} />
      {/* Tied back — the knot sits behind the head, so it goes first. */}
      <Circle cx="74" cy="32" r="9" fill={ink} />
      <Head face={face} />
      <Path d="M29 42c0-15 9-24 21-24s21 9 21 24c-4-10-11-14-21-14s-17 4-21 14z" fill={ink} />
    </G>
  );
}

export function PersonCoral({ ink, face }: Figure) {
  return (
    <G>
      <Body ink={ink} face={face} />
      {/* A full, round shape framing the whole head — drawn behind it, so the
          face stays clear and only the halo shows. */}
      <Circle cx="50" cy="38" r="27" fill={ink} />
      <Head face={face} />
      <Path d="M29 40c0-13 9-21 21-21s21 8 21 21c-4-8-11-12-21-12s-17 4-21 12z" fill={ink} />
    </G>
  );
}

export function PersonSlate({ ink, face }: Figure) {
  return (
    <G>
      <Body ink={ink} face={face} />
      <Head face={face} />
      {/* A beard, and very little on top — the one portrait whose silhouette
          is decided below the eyes rather than above them. */}
      <Path d="M30 44c1-6 3-10 6-13 3 5 8 8 14 8s11-3 14-8c3 3 5 7 6 13-3-7-10-11-20-11s-17 4-20 11z" fill={ink} />
      <Path d="M31 46c0 16 8 26 19 26s19-10 19-26c-2 9-9 13-19 13s-17-4-19-13z" fill={ink} />
    </G>
  );
}

export function PersonMoss({ ink, face }: Figure) {
  return (
    <G>
      <Body ink={ink} face={face} />
      <Head face={face} />
      {/* A brimmed hat — the one silhouette that is not about hair. */}
      <Path d="M20 33h60v6H20z" fill={ink} />
      <Path d="M32 33c0-12 8-19 18-19s18 7 18 19z" fill={ink} />
    </G>
  );
}

/* ── machines ───────────────────────────────────────────────────────────── */

/** The casing every machine's head is cut from, and the neck under it. */
function Casing({ ink, rx = 14 }: Figure & { rx?: number }) {
  return (
    <G>
      <Rect x="42" y="74" width="16" height="10" fill={ink} />
      <Path d="M20 88c0-9 13-14 30-14s30 5 30 14z" fill={ink} />
      <Rect x="22" y="24" width="56" height="52" rx={rx} fill={ink} />
    </G>
  );
}

export function MachineSteel({ ink }: Figure) {
  return (
    <G>
      {/* Antenna, above the casing so the casing does not cover its stem. */}
      <Rect x="48" y="8" width="4" height="16" fill={ink} />
      <Circle cx="50" cy="8" r="6" fill={ink} />
      <Casing ink={ink} />
      {/* Cut-outs are painted in the wash behind, not in a second colour —
          one ink per figure, holes included. */}
      <Circle cx="38" cy="46" r="7" fill="rgba(255,255,255,0.92)" />
      <Circle cx="62" cy="46" r="7" fill="rgba(255,255,255,0.92)" />
      <Rect x="38" y="62" width="24" height="4" rx="2" fill="rgba(255,255,255,0.92)" />
    </G>
  );
}

export function MachineBrass({ ink }: Figure) {
  return (
    <G>
      <Casing ink={ink} rx={26} />
      {/* One wide lens across the whole face. */}
      <Rect x="31" y="40" width="38" height="14" rx="7" fill="rgba(255,255,255,0.92)" />
      <Circle cx="42" cy="47" r="4" fill={ink} />
      <Circle cx="58" cy="47" r="4" fill={ink} />
      <Rect x="42" y="63" width="16" height="3" rx="1.5" fill="rgba(255,255,255,0.92)" />
    </G>
  );
}

export function MachineCopper({ ink }: Figure) {
  return (
    <G>
      <Rect x="14" y="38" width="8" height="20" rx="4" fill={ink} />
      <Rect x="78" y="38" width="8" height="20" rx="4" fill={ink} />
      <Casing ink={ink} />
      {/* A single eye, centred. */}
      <Circle cx="50" cy="46" r="11" fill="rgba(255,255,255,0.92)" />
      <Circle cx="50" cy="46" r="4.5" fill={ink} />
      <Rect x="40" y="64" width="20" height="4" rx="2" fill="rgba(255,255,255,0.92)" />
    </G>
  );
}

export function MachineVerdigris({ ink }: Figure) {
  return (
    <G>
      <Rect x="48" y="8" width="4" height="16" fill={ink} />
      <Rect x="38" y="4" width="24" height="6" rx="3" fill={ink} />
      <Casing ink={ink} rx={8} />
      <Rect x="32" y="41" width="14" height="10" rx="3" fill="rgba(255,255,255,0.92)" />
      <Rect x="54" y="41" width="14" height="10" rx="3" fill="rgba(255,255,255,0.92)" />
      {/* Cooling vents where a mouth would be. */}
      <Rect x="38" y="61" width="24" height="3" fill="rgba(255,255,255,0.92)" />
      <Rect x="38" y="67" width="24" height="3" fill="rgba(255,255,255,0.92)" />
    </G>
  );
}

export function MachineGunmetal({ ink }: Figure) {
  return (
    <G>
      <Casing ink={ink} rx={14} />
      {/* Twin lenses set wide, under a visor slot. */}
      <Rect x="28" y="36" width="44" height="6" rx="3" fill="rgba(255,255,255,0.92)" />
      <Circle cx="37" cy="54" r="6" fill="rgba(255,255,255,0.92)" />
      <Circle cx="63" cy="54" r="6" fill="rgba(255,255,255,0.92)" />
      <Circle cx="50" cy="54" r="3" fill="rgba(255,255,255,0.92)" />
    </G>
  );
}

export function MachineOxide({ ink }: Figure) {
  return (
    <G>
      <Casing ink={ink} rx={20} />
      {/* Wide-set round eyes and a grille. */}
      <Ellipse cx="38" cy="45" rx="8" ry="9" fill="rgba(255,255,255,0.92)" />
      <Ellipse cx="62" cy="45" rx="8" ry="9" fill="rgba(255,255,255,0.92)" />
      <Rect x="34" y="62" width="32" height="8" rx="4" fill="rgba(255,255,255,0.92)" />
      <Rect x="44" y="62" width="3" height="8" fill={ink} />
      <Rect x="53" y="62" width="3" height="8" fill={ink} />
    </G>
  );
}

/** Every figure by slug — the one lookup `Avatar` needs. */
export const FIGURES: Record<string, (p: Figure) => React.JSX.Element> = {
  'p-amber': PersonAmber,
  'p-violet': PersonViolet,
  'p-teal': PersonTeal,
  'p-coral': PersonCoral,
  'p-slate': PersonSlate,
  'p-moss': PersonMoss,
  'm-steel': MachineSteel,
  'm-brass': MachineBrass,
  'm-copper': MachineCopper,
  'm-verdigris': MachineVerdigris,
  'm-gunmetal': MachineGunmetal,
  'm-oxide': MachineOxide,
};
