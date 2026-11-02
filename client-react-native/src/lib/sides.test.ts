import type { Seat, Zone } from '@/src/api/matchTypes';
import { setLocale } from '@/src/lib/i18n';
import { isPartnerOfViewer, partnerText, partnersOf, sideMembers, spreadAreas } from '@/src/lib/sides';

afterEach(() => setLocale('en'));

const players = [
  { id: 'p1', name: 'Ana' },
  { id: 'p2', name: 'Bea' },
  { id: 'p3', name: 'Cal' },
  { id: 'p4', name: 'Dan' },
];

/** Four seats, partners opposite each other — what the server sends for a
 *  four-handed Canasta, sides and all. */
const partnered: Seat[] = [
  { playerId: 'p1', side: '0' },
  { playerId: 'p2', side: '1' },
  { playerId: 'p3', side: '0' },
  { playerId: 'p4', side: '1' },
];

/** The same table in a game with no sides: no `side` anywhere. */
const soloists: Seat[] = players.map((p) => ({ playerId: p.id }));

describe('partnersOf', () => {
  it('is everyone sharing a side, that seat excluded', () => {
    expect(partnersOf(partnered, 'p1')).toEqual(['p3']);
    expect(partnersOf(partnered, 'p4')).toEqual(['p2']);
  });

  it('is empty where the game has no sides', () => {
    expect(partnersOf(soloists, 'p1')).toEqual([]);
  });

  it('is empty for a player who is not at the table', () => {
    expect(partnersOf(partnered, 'nobody')).toEqual([]);
  });

  // Six seats are three sides of two, so a partner is one person and the other
  // four are opponents in two camps — the case a board that only knew "mine"
  // and "not mine" would flatten.
  it('keeps three sides apart', () => {
    const six: Seat[] = [
      { playerId: 'p1', side: '0' },
      { playerId: 'p2', side: '1' },
      { playerId: 'p3', side: '2' },
      { playerId: 'p4', side: '0' },
      { playerId: 'p5', side: '1' },
      { playerId: 'p6', side: '2' },
    ];
    expect(partnersOf(six, 'p2')).toEqual(['p5']);
    expect(partnersOf(six, 'p3')).toEqual(['p6']);
  });
});

describe('isPartnerOfViewer', () => {
  it('marks the viewer’s partner and nobody else', () => {
    expect(isPartnerOfViewer(partnered, 'p3', 'p1')).toBe(true);
    expect(isPartnerOfViewer(partnered, 'p2', 'p1')).toBe(false);
  });

  // The viewer's own tile is already the one drawn as theirs; marking it as
  // its own teammate would put "with you" under their own name.
  it('does not mark the viewer as their own partner', () => {
    expect(isPartnerOfViewer(partnered, 'p1', 'p1')).toBe(false);
  });
});

describe('partnerText', () => {
  it('names a seat’s partner', () => {
    expect(partnerText(partnered, players, 'p2', 'p1')).toBe('Dan');
  });

  it('calls the viewer "you" wherever they appear', () => {
    expect(partnerText(partnered, players, 'p3', 'p1')).toBe('you');
  });

  it('is empty where there is nobody to name', () => {
    expect(partnerText(soloists, players, 'p1', 'p1')).toBe('');
  });

  it('follows the active locale', () => {
    setLocale('cs');
    expect(partnerText(partnered, players, 'p3', 'p1')).toBe('ty');
  });
});

describe('spreadAreas', () => {
  const spread = (id: string, extra: Partial<Zone> = {}): Zone => ({ id, kind: 'spread', count: 0, ...extra });
  // Seating order, the other side first — the order a viewer on side 1 used
  // to see their opponents' melds above their own.
  const canasta = [
    spread('melds:t0', { side: '0' }),
    spread('redThrees:t0', { side: '0' }),
    spread('melds:t1', { side: '1' }),
    spread('redThrees:t1', { side: '1' }),
  ];

  it("gathers a side's zones and puts the viewer's side first", () => {
    const areas = spreadAreas(canasta, partnered, 'p2');
    expect(areas.map((a) => a.side)).toEqual(['1', '0']);
    expect(areas[0].zones.map((z) => z.id)).toEqual(['melds:t1', 'redThrees:t1']);
  });

  it('keeps seating order for a spectator', () => {
    expect(spreadAreas(canasta, partnered, 'watcher').map((a) => a.side)).toEqual(['0', '1']);
  });

  it("gathers a side's zones that did not arrive together", () => {
    const shuffled = [canasta[0], canasta[2], canasta[1]];
    const areas = spreadAreas(shuffled, partnered, 'p1');
    expect(areas[0].zones.map((z) => z.id)).toEqual(['melds:t0', 'redThrees:t0']);
  });

  it("leaves each player's own zone standing alone, the viewer's first", () => {
    const zones = [spread('m:p1', { ownerId: 'p1' }), spread('m:p2', { ownerId: 'p2' })];
    const areas = spreadAreas(zones, [{ playerId: 'p1' }, { playerId: 'p2' }], 'p2');
    expect(areas.map((a) => [a.side, a.zones[0].id])).toEqual([
      [undefined, 'm:p2'],
      [undefined, 'm:p1'],
    ]);
  });

  it('names who sits on a side', () => {
    expect(sideMembers(partnered, '1')).toEqual(['p2', 'p4']);
  });
});
