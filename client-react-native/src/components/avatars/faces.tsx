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

type Figure = { ink: string };

/* ── people ─────────────────────────────────────────────────────────────── */

/** Shoulders every portrait sits on, so the six read as one set. */
function Shoulders({ ink }: Figure) {
  return <Path d="M18 100c0-18 14-30 32-30s32 12 32 30z" fill={ink} />;
}

export function PersonAmber({ ink }: Figure) {
  return (
    <G>
      <Circle cx="50" cy="42" r="20" fill={ink} />
      {/* A short crop, sitting on the crown. */}
      <Path d="M30 40c0-14 9-22 20-22s20 8 20 22c-4-8-11-11-20-11s-16 3-20 11z" fill={ink} />
      <Shoulders ink={ink} />
    </G>
  );
}

export function PersonViolet({ ink }: Figure) {
  return (
    <G>
      <Circle cx="50" cy="43" r="19" fill={ink} />
      {/* Long, falling past the jaw on both sides. */}
      <Path d="M28 44c0-17 10-26 22-26s22 9 22 26v22h-7V44c0-9-6-14-15-14s-15 5-15 14v22h-7z" fill={ink} />
      <Shoulders ink={ink} />
    </G>
  );
}

export function PersonTeal({ ink }: Figure) {
  return (
    <G>
      <Circle cx="50" cy="42" r="20" fill={ink} />
      {/* Tied back — a small knot behind the crown. */}
      <Circle cx="72" cy="34" r="8" fill={ink} />
      <Path d="M30 40c0-14 9-22 20-22s20 8 20 22c-4-8-11-11-20-11s-16 3-20 11z" fill={ink} />
      <Shoulders ink={ink} />
    </G>
  );
}

export function PersonCoral({ ink }: Figure) {
  return (
    <G>
      <Circle cx="50" cy="44" r="19" fill={ink} />
      {/* A round, full shape framing the whole head. */}
      <Circle cx="50" cy="36" r="24" fill={ink} />
      <Shoulders ink={ink} />
    </G>
  );
}

export function PersonSlate({ ink }: Figure) {
  return (
    <G>
      <Circle cx="50" cy="42" r="20" fill={ink} />
      {/* Bare crown, with the sides kept. */}
      <Path d="M29 46c0-6 2-11 5-14 2 6 8 9 16 9s14-3 16-9c3 3 5 8 5 14-3-9-11-13-21-13s-18 4-21 13z" fill={ink} />
      <Shoulders ink={ink} />
    </G>
  );
}

export function PersonMoss({ ink }: Figure) {
  return (
    <G>
      <Circle cx="50" cy="43" r="19" fill={ink} />
      {/* A brimmed hat — the one silhouette that is not about hair. */}
      <Path d="M22 34h56v5H22z" fill={ink} />
      <Path d="M34 34c0-11 7-17 16-17s16 6 16 17z" fill={ink} />
      <Shoulders ink={ink} />
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
