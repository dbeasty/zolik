import type { ActionOffer } from '@/src/api/matchTypes';
import { NOTHING_FITS_HERE, sharedRefusal } from '@/src/api/matchTypes';

const refused = (id: string, whyNot?: string): ActionOffer =>
  ({ id, verb: 'lay_off', enabled: false, whyNot }) as ActionOffer;

describe('sharedRefusal', () => {
  it('prints the reason most members give', () => {
    expect(
      sharedRefusal([refused('a', 'WRONG_PHASE'), refused('b', 'WRONG_PHASE'), refused('c', 'MUST_MELD_FIRST')]),
    ).toBe('WRONG_PHASE');
  });

  // Match 6abc22d3b46a546c9d9bd1e4: six melds the four did not match, one
  // that refused it for the rule the player needed to hear.
  it('does not let "nothing fits here" outvote the one real reason', () => {
    const group = [
      ...['j', '10', '5', 'a', 'k', 'q'].map((id) => refused(id, NOTHING_FITS_HERE)),
      refused('4', 'MUST_KEEP_A_CARD'),
    ];
    expect(sharedRefusal(group)).toBe('MUST_KEEP_A_CARD');
  });

  it('still says "nothing fits here" when that is all there is', () => {
    expect(sharedRefusal([refused('a', NOTHING_FITS_HERE), refused('b', NOTHING_FITS_HERE)])).toBe(NOTHING_FITS_HERE);
  });

  it('ignores enabled members and members with no reason', () => {
    expect(sharedRefusal([{ ...refused('a', 'X'), enabled: true }, refused('b')])).toBeUndefined();
  });
});
