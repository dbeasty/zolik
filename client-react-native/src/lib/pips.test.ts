import { isCourt, pipsFor } from '@/src/lib/pips';

const NUMBERED = ['2', '3', '4', '5', '6', '7', '8', '9', '10'];

describe('pipsFor', () => {
  it('draws as many pips as the rank names', () => {
    for (const rank of NUMBERED) {
      expect(pipsFor(rank)).toHaveLength(Number(rank));
    }
  });

  // An ace, a court and a joker are each drawn as their own piece of art, and
  // the caller tells them apart by getting nothing back rather than by
  // carrying a second list of which ranks are special.
  it('gives no pips to the ranks that are drawn some other way', () => {
    for (const rank of ['A', 'J', 'Q', 'K', 'JKR']) {
      expect(pipsFor(rank)).toHaveLength(0);
    }
  });

  it('keeps every pip inside the field', () => {
    for (const rank of NUMBERED) {
      for (const p of pipsFor(rank)) {
        expect(p.x).toBeGreaterThan(0);
        expect(p.x).toBeLessThan(1);
        expect(p.y).toBeGreaterThan(0);
        expect(p.y).toBeLessThan(1);
      }
    }
  });

  // Rule 1: two columns and a spine, nothing anywhere else. A pip at an
  // arbitrary x is how a hand-tuned layout drifts into a spreadsheet.
  it('puts every pip on one of three columns', () => {
    const columns = new Set(NUMBERED.flatMap((r) => pipsFor(r).map((p) => p.x)));
    expect([...columns].sort((a, b) => a - b)).toHaveLength(3);
  });

  // Rule 2: a real card reads from either end, so the bottom half is the top
  // half upside down. Checked as a property of every layout rather than
  // spot-checked on one, because it is the rule that makes them read as real.
  //
  // The seven is the one printed exception, and it is a real one rather than
  // an oversight here: seven is odd, its odd pip rides the spine *above* the
  // waist (see the test below), and so it has no partner. Every other rank
  // either has an even count or puts its odd pip exactly on the waist, where
  // it is its own mirror.
  it.each(NUMBERED.filter((r) => r !== '7'))('mirrors the %s top to bottom', (rank) => {
    const pips = pipsFor(rank);
    const unmirrored = pips.filter(
      (p) => !pips.some((q) => Math.abs(q.x - p.x) < 1e-9 && Math.abs(q.y - (1 - p.y)) < 1e-9),
    );
    expect(unmirrored).toEqual([]);
  });

  it('turns the bottom half over and leaves the waist alone', () => {
    for (const rank of NUMBERED) {
      for (const p of pipsFor(rank)) {
        expect({ y: p.y, flip: p.flip }).toEqual({ y: p.y, flip: p.y > 0.5 });
      }
    }
    // The three's middle pip sits exactly on the waist and stays upright.
    expect(pipsFor('3').filter((p) => p.flip)).toHaveLength(1);
  });

  // The seven's odd pip is the one every "six plus a centre pip" gets wrong.
  it('rides the seventh pip high on the spine rather than at the centre', () => {
    const seven = pipsFor('7');
    const six = pipsFor('6');
    const odd = seven.filter((p) => !six.some((q) => q.x === p.x && q.y === p.y));
    expect(odd).toHaveLength(1);
    expect(odd[0].x).toBe(0.5);
    expect(odd[0].y).toBeLessThan(0.5);
    expect(odd[0].y).toBeGreaterThan(Math.min(...six.map((p) => p.y)));
  });

  it('stacks the nine and the ten four to a column', () => {
    for (const rank of ['9', '10']) {
      const left = pipsFor(rank).filter((p) => p.x < 0.5);
      expect(left).toHaveLength(4);
    }
  });
});

describe('isCourt', () => {
  it('is the three ranks that wear a figure', () => {
    expect(['J', 'Q', 'K'].every(isCourt)).toBe(true);
    expect(['A', '10', '2', 'JKR'].some(isCourt)).toBe(false);
  });
});
