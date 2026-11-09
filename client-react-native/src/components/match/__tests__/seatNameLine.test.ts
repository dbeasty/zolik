import { seatNameNeedsOwnLine } from '../SeatStrip';

describe('where a seat tile puts the name', () => {
  it('past four seats, always under the face', () => {
    expect(seatNameNeedsOwnLine(5, 5000, 0)).toBe(true);
  });

  it('under the face at four seats in a 1100 point window, where it came down to "P…"', () => {
    expect(seatNameNeedsOwnLine(4, 1013, 94)).toBe(true);
  });

  it('beside the face when the share has room for it', () => {
    expect(seatNameNeedsOwnLine(2, 1013, 94)).toBe(false);
    expect(seatNameNeedsOwnLine(4, 1800, 94)).toBe(false);
  });

  it('beside it before the strip has been measured', () => {
    expect(seatNameNeedsOwnLine(4, 0, 94)).toBe(false);
  });
});
