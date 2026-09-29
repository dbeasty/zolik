import {
  cardSuit,
  cardText,
  displayRank,
  isCardCode,
  isTileCode,
  parseCard,
  tileText,
} from "@/src/lib/cards";

// What is left of this file after the bespoke Žolíky screen went.
//
// `autoOrganizeHand` and `moveCardToIndex` were tested here too — sorting a
// rummy hand into runs and sets, and reordering it by drag. Both were rules
// living in a client, and both went with the screen that needed them. Drawing
// a card is not: every game draws "TD" the same way.

describe("cards", () => {
  it("parses standard cards", () => {
    const d = parseCard("KH");
    expect(d.rank).toBe("K");
    expect(d.suit).toBe("H");
    expect(d.isRed).toBe(true);
    expect(d.isJoker).toBe(false);
  });

  it("parses ten", () => {
    expect(displayRank("TH")).toBe("10");
    expect(cardSuit("TH")).toBe("H");
  });

  it("parses joker", () => {
    const d = parseCard("JOKER1");
    expect(d.isJoker).toBe(true);
    expect(d.rank).toBe("JKR");
  });
});

describe("tiles in prose", () => {
  it("writes a tile as its colour and its number", () => {
    expect(tileText("7-R")).toBe("🔴7");
    expect(tileText("13-B")).toBe("🔵13");
    expect(tileText("1-O")).toBe("🟠1");
    expect(tileText("12-K")).toBe("⚫12");
  });

  it("knows a tile code only in its exact shape", () => {
    for (const v of ["7-R", "10-K", "13-O"]) expect(isTileCode(v)).toBe(true);
    for (const v of ["0-R", "14-B", "7-G", "7R", "7-r", "-R", "x7-R", "TH"]) {
      expect(isTileCode(v)).toBe(false);
    }
  });

  it("never takes a tile for a card, or a card for a tile", () => {
    expect(isCardCode("7-R")).toBe(false);
    expect(isTileCode("7H")).toBe(false);
    expect(cardText("7-R")).toBe("7-R");
    expect(tileText("7H")).toBe("7H");
  });
});
