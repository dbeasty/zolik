/**
 * Every face a player can wear, and the rule for which one they get.
 *
 * Deliberately *not* in the shell. `src/lib/__tests__/shell.test.ts` greps the
 * match screen's files for any one game's vocabulary, and a roster of names is
 * exactly the kind of list that eventually acquires one. So the shell imports
 * `avatarFor` and renders whatever it is handed, and the names live here.
 *
 * The identity a face is derived from is the **player id**, never the name.
 * The initial-circle this replaces hashed the name, which meant renaming
 * yourself changed your face mid-match — a small thing that quietly said the
 * face was decoration rather than identity.
 */

export type AvatarKind = 'person' | 'machine';

export type AvatarSpec = {
  /** Stable slug. Travels on the wire and is stored in a preference. */
  id: string;
  kind: AvatarKind;
  /** Shown under the swatch in the picker, and read out by a screen reader. */
  label: string;
  /**
   * The two-stop wash behind the figure, and the ink the figure is drawn in.
   * Held here rather than in the skin because a face is not a look: two
   * players must be told apart under every skin, so these colours are the
   * one palette on the board a skin does not get to repaint.
   */
  palette: { from: string; to: string; ink: string };
};

/**
 * The people. Six is enough that a full table rarely collides and few enough
 * that the picker fits a phone without scrolling.
 */
const PEOPLE: readonly AvatarSpec[] = [
  { id: 'p-amber', kind: 'person', label: 'Amber', palette: { from: '#e8a33d', to: '#b8722f', ink: '#3a2410' } },
  { id: 'p-violet', kind: 'person', label: 'Violet', palette: { from: '#9b7ad6', to: '#6a4aa8', ink: '#241a3a' } },
  { id: 'p-teal', kind: 'person', label: 'Teal', palette: { from: '#4fb8a0', to: '#2f7d6c', ink: '#0f2b26' } },
  { id: 'p-coral', kind: 'person', label: 'Coral', palette: { from: '#e88070', to: '#b84f5e', ink: '#3a1620' } },
  { id: 'p-slate', kind: 'person', label: 'Slate', palette: { from: '#7d93a8', to: '#4a5f75', ink: '#16212e' } },
  { id: 'p-moss', kind: 'person', label: 'Moss', palette: { from: '#8fae5a', to: '#5a7d3f', ink: '#1d2a12' } },
];

/**
 * The machines, one per metal. An opponent nobody is sitting at gets one of
 * these, so "who am I actually playing" is answered by looking rather than by
 * reading a badge.
 */
const MACHINES: readonly AvatarSpec[] = [
  { id: 'm-steel', kind: 'machine', label: 'Steel', palette: { from: '#b9c6d4', to: '#6c7f92', ink: '#1b2530' } },
  { id: 'm-brass', kind: 'machine', label: 'Brass', palette: { from: '#e0c070', to: '#a3823a', ink: '#33260c' } },
  { id: 'm-copper', kind: 'machine', label: 'Copper', palette: { from: '#e0906a', to: '#a55a38', ink: '#331a10' } },
  { id: 'm-verdigris', kind: 'machine', label: 'Verdigris', palette: { from: '#7fc9b8', to: '#3f8a7a', ink: '#0e2b26' } },
  { id: 'm-gunmetal', kind: 'machine', label: 'Gunmetal', palette: { from: '#8b93a3', to: '#4b5462', ink: '#151a22' } },
  { id: 'm-oxide', kind: 'machine', label: 'Oxide', palette: { from: '#d1836f', to: '#8f4534', ink: '#2e130e' } },
];

/** The whole roster, people first — the order the picker lays them out in. */
export const AVATARS: readonly AvatarSpec[] = [...PEOPLE, ...MACHINES];

export function avatarById(id: string | null | undefined): AvatarSpec | undefined {
  return AVATARS.find((a) => a.id === id);
}

/** The faces a player of this sort may be given, and may choose between. */
export function choicesFor(isAI: boolean): readonly AvatarSpec[] {
  return isAI ? MACHINES : PEOPLE;
}

/**
 * A number from a string, stable everywhere and cheap. Same idiom the seat
 * tile's initial-circle used before it had faces to pick from.
 */
function hash(s: string): number {
  let h = 0;
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) >>> 0;
  return h;
}

/**
 * The face a player wears.
 *
 * Their own choice, if they made one and it names a face this build knows.
 * Otherwise one derived from their id — which every client derives
 * identically, so a player who has never picked still looks the same to
 * everybody at the table without a single byte being stored for them.
 *
 * A seat nobody is sitting at always gets a machine, whatever it asked for.
 * That is the one place the choice is overruled, and it is worth overruling:
 * an opponent that looks human and is not is the sort of thing a player
 * should never have to find out by being told.
 */
export function avatarFor(playerId: string, isAI: boolean, chosen?: string): AvatarSpec {
  const pool = choicesFor(isAI);
  const named = avatarById(chosen);
  if (named && named.kind === (isAI ? 'machine' : 'person')) return named;
  return pool[hash(playerId) % pool.length]!;
}
