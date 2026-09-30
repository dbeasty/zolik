/**
 * The name offered to a guest who has not said what to call themselves.
 *
 * The field on the guest screen used to open with "Player" in it, and most
 * people leave a prefilled field alone — which is how a table ends up seating
 * Player, Player and Player, and why the only way to follow whose turn it is
 * becomes counting seats. A default name has to identify before it is anything
 * else.
 *
 * Numbering would identify — Player 1, Player 2 — and is not what we do: a
 * number is a queue ticket, a poor thing to be called for an evening of cards.
 * So a name is drawn from a roster instead, in the same spirit as the bot
 * personas the server seats (Rookie Rita, Master Miroslav) but deliberately a
 * different shape — those are adjective-plus-forename, these are
 * adjective-plus-creature — so a guest is never taken for an opponent nobody
 * is sitting at.
 *
 * The roster is English and not translated, as the avatar labels in
 * `components/avatars/catalogue.ts` are not. These are proper names, the thing
 * a locale bundle helps with least, and the suggestion is sitting in an
 * editable field on the screen that makes it.
 *
 * The server keeps a near-copy of this roster (`server/internal/auth/
 * guestname.go`) for clients that send no name at all — the SSH host, mainly.
 * The two lists are allowed to drift: nothing compares them, and a guest named
 * from either is equally well named. That is the opposite of the rule for
 * `serverKeys.json`, where drift reaches a player as SCREAMING_SNAKE.
 */

const ADJECTIVES = [
  'Amber', 'Arctic', 'Autumn', 'Bold', 'Brave', 'Brisk', 'Bright', 'Calm',
  'Canny', 'Cheery', 'Clever', 'Copper', 'Coral', 'Crimson', 'Curious', 'Daring',
  'Dusty', 'Eager', 'Emerald', 'Fearless', 'Fleet', 'Frosty', 'Gentle', 'Golden',
  'Grand', 'Happy', 'Hasty', 'Honest', 'Humble', 'Indigo', 'Ivory', 'Jolly',
  'Keen', 'Kindly', 'Lively', 'Lucky', 'Merry', 'Mighty', 'Nimble', 'Noble',
  'Patient', 'Plucky', 'Polite', 'Proud', 'Quick', 'Quiet', 'Rapid', 'Restless',
  'Royal', 'Ruby', 'Rusty', 'Scarlet', 'Sharp', 'Silver', 'Sly', 'Snowy',
  'Solid', 'Spry', 'Steady', 'Stormy', 'Sunny', 'Swift', 'Velvet', 'Witty',
] as const;

const NOUNS = [
  'Otter', 'Badger', 'Falcon', 'Heron', 'Marten', 'Lynx', 'Sparrow', 'Magpie',
  'Raven', 'Robin', 'Fox', 'Hare', 'Stag', 'Ibex', 'Bison', 'Boar',
  'Wolf', 'Owl', 'Crane', 'Swallow', 'Finch', 'Kestrel', 'Osprey', 'Puffin',
  'Curlew', 'Beaver', 'Weasel', 'Ferret', 'Mole', 'Dormouse', 'Squirrel', 'Hedgehog',
  'Chamois', 'Lark', 'Wren', 'Starling', 'Kite', 'Harrier', 'Pike', 'Perch',
  'Trout', 'Salmon', 'Newt', 'Frog', 'Turtle', 'Bee', 'Moth', 'Cricket',
  'Beetle', 'Firefly', 'Dragonfly', 'Comet', 'Ember', 'Lantern', 'Compass', 'Anchor',
  'Beacon', 'Pebble', 'Willow', 'Aspen', 'Birch', 'Juniper', 'Thistle', 'Clover',
] as const;

/** How many names the roster can make. 64 x 64, and the reason no suffix is
 *  needed: two guests at a six-seat table collide about one table in 270. */
export const GUEST_NAME_COUNT = ADJECTIVES.length * NOUNS.length;

/**
 * A number from a string. The same idiom `avatarFor` hashes an id with, kept
 * identical on purpose — a face and a name derived from one id by one method
 * are two views of the same coin toss rather than two unrelated ones.
 */
function hash(s: string): number {
  let h = 0;
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) >>> 0;
  return h;
}

/**
 * The name to offer.
 *
 * Given this device's guest id it is the *same* name every time, which is what
 * a returning guest wants: the person who played as Nimble Heron last week and
 * signed out is offered Nimble Heron again, not a stranger's name. With no id
 * — a first run, before the server has issued one — one is drawn at random,
 * which is still 4096 ways to not be called Player.
 *
 * Dividing out the first list's length before the second index keeps the two
 * halves independent; two plain moduli of the same number would move together,
 * because the lengths are equal.
 */
export function guestNameFor(seed?: string | null): string {
  const n = seed ? hash(seed) : Math.floor(Math.random() * GUEST_NAME_COUNT);
  const adjective = ADJECTIVES[n % ADJECTIVES.length]!;
  const noun = NOUNS[Math.floor(n / ADJECTIVES.length) % NOUNS.length]!;
  return `${adjective} ${noun}`;
}
