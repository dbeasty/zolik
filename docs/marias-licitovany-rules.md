# Mariáš licitovaný — draft rules (for sign-off)

This is step 4's step 0: the rules [marias-plan.md](marias-plan.md) will build Licitovaný to,
written down before any code. It is a variation of the `marias` module, sharing the deck, the
play, the doubling round and the settlement machinery with Volený
([marias-rules.md](marias-rules.md)). This document covers only what differs.

**Status:** draft, not yet built. There are seven questions for you at the end (§8).

## Sources

- **[ČSM-L]** Český svaz mariáše, *Pravidla soutěžního licitovaného mariáše*, in force from
  1.1.2010 ([talon.cz PDF](https://www.talon.cz/pravidla/mari%C3%A1%C5%A1_pravidla_licitovan%C3%BD_2014.pdf)).
  This is the association's tournament sheet: the bidding ladder, the tariff, and the
  licitovaný-specific rulings.
- **[ČSM-O]** Český svaz mariáše, *Obecná pravidla hry mariáš* (2009)
  ([mariasnik.cz PDF](http://www.mariasnik.cz/public/rules/obecna-pravidla-mariase.pdf)).
  These are the general rules the tournament sheet builds on: the auction procedure (Čl. VII/3),
  "dvě sedmy" (Čl. IV/9), and how sto is scored (Čl. V/6-7). Where the two disagree, ČSM-L wins
  (ČSM-L II/1).

## 1. What is different from Volený

| | Volený (shipped) | Licitovaný |
|---|---|---|
| Deal | Chooser gets 7, names trumps, then 5 more | 10 each; **the talon is dealt face down** to the middle (ČSM-O VII/3) |
| Who declares | The player left of the dealer (forhont) | **Whoever wins the auction** |
| Trumps named | From the first seven, before the talon | By the declarer, **after** taking the talon, as part of the contract |
| Games | hra, sedma, sto, betl, durch | The **12-step ladder** (§3), which adds **dvě sedmy**. There is no plain hra; the lowest contract is sedma |
| Take-over (špatná) | Yes | No; the auction replaces it |
| Proti | Sedma and sto proti | **None** (ČSM-L II/23) |
| Doubling | Unlimited by default | **At most four** (flek, re, tutti, boty) (ČSM-L IV) |
| First lead | The declarer (always the forhont there) | **The forhont**, even when someone else declares; the declarer leads only in betl and durch (ČSM-O II/6) |
| Bluffing | Sedma needs the 7 | Any contract above sedma **need not match the cards** (ČSM-L II/16) |
| Folding | None | **Omyl**: a declarer held to plain sedma may fold instead of playing (§5) |

## 2. The deal

Each player gets 10 cards. The deal goes 5-5-5, then 2 to the talon, then 5-5-5; digitally only
the counts matter. The talon lies face down, and nobody sees it until the auction is won.

## 3. The ladder

Bids name a rung. The contract finally announced may be any rung **at or above** the one won
(ČSM-O VII/3). "Červená" means hearts are trumps, which doubles every payment for a trump game.

| Rung | Contract | Its parts |
|---|---|---|
| 1 | sedma | hra + sedma |
| 2 | sedma červená | hra + sedma, hearts |
| 3 | sto | sto |
| 4 | sto a sedma | sto + sedma |
| 5 | sto červených | sto, hearts |
| 6 | sto a sedma červených | sto + sedma, hearts |
| 7 | betl | betl |
| 8 | durch | durch |
| 9 | dvě sedmy | dvě sedmy |
| 10 | dvě sedmy a sto | dvě sedmy + sto |
| 11 | dvě sedmy, červená trumf | dvě sedmy, hearts |
| 12 | dvě sedmy, červená trumf, a sto | dvě sedmy + sto, hearts |

**Dvě sedmy** (ČSM-O IV/9): the declarer names trumps and a *helper suit*. They must win the
second-to-last trick with the helper suit's 7 and the last trick with the trump 7. It fails if
either seven is beaten, or if the trump 7 has to be played early. Both announced sevens are kept
for their tricks, as the sedma is in Volený (ČSM-O IV/11).

## 4. The auction (ČSM-O VII/3)

The three seats are the **forhont** (left of the dealer), the **middle** player, and the
**zadák** (the dealer, who got the last cards).

1. **The zadák bids against the forhont.** The zadák names a rung, or passes. Rungs may be
   skipped.
2. The forhont answers **"mám"** (holds, meaning they will play that rung too) or passes. Holding
   is enough: at equal rungs the forhont wins.
3. The zadák then names a higher rung or passes, and so on, until one of them passes.
4. **The middle player then takes the place of whoever passed.**
   - If the zadák passed, the middle bids against the forhont.
   - If the forhont passed, the middle holds against the zadák and, like the forhont, needs only
     to match.
5. The last player left wins at the last rung bid or held.
6. **If nobody bids, the forhont is the declarer at rung 1 (sedma).** *Pinned by us:* the sheet
   implies it (the forhont is the default holder) but does not spell it out.

The winner takes the talon (12 cards) and announces the contract. That means:
- a rung at or above the one won;
- the trump suit, or the helper suit too for dvě sedmy.

They then lay two cards away. As in Volený, there is no eso or 10 in the talon in a trump game,
and no announced seven (ČSM-L II/17; ČSM-L IV, renonc 19-20).

## 5. Sevens, bluffing and omyl (ČSM-L II/16-17)

- **Plain sedma** (rungs 1-2) cannot be played without the trump 7 in hand.
- **Above sedma the contract need not match the cards.** A player may announce sto with no
  marriage, or sto a sedma without the seven, and simply lose those parts.
- **Omyl:** a declarer whose contract is plain sedma (rung 1) and who does not want to play may
  fold before play. They pay the omyl tariff, which is 6 units to each defender (ČSM-L I, "omyl
  1,20" on a 0,20 base). The rule reads this as the flekked hra plus the flekked sedma: 1×2 + 2×2
  = 6.
- A player who won at **sedma červená** without the heart 7 must announce something higher
  (ČSM-L II/17). That is the same rule as the first bullet, applied to rung 2.

## 6. Doubling and play

- The doubling round works as in Volený: each part separately, the sides alternating, starting
  with the defender after the declarer. There are **no proti announcements**, and **at most four
  doublings** per part (ČSM-L IV: "flek nad rámec posledního platného fleku (čtvrtého)").
- **The forhont leads to the first trick**, except in betl and durch, where the declarer leads
  (ČSM-O II/6).
- Follow, beat, trump; marriages; betl and durch ending early: all exactly as Volený.

## 7. Scoring

The tariff in units (ČSM-L I, on its 0,20 base):

| Part | Units |
|---|---|
| hra | 1 |
| sedma | 2 |
| sto | 4 |
| betl | 15 |
| durch | 30 |
| dvě sedmy | 40 |
| tichá sedma | 1 |
| omyl | 6 |

- **Payment:** each part is paid between the declarer and each defender. Hearts double every
  trump-game part. Each doubling doubles its part.
- **Quiet results** (ČSM-L I):
  - *Tiché sto* can happen only in a sedma contract, on its hra part.
  - *Tichá sedma* can happen only in a sto contract. It is paid only when the seven wins the last
    trick, or is **beaten** in it.
  - A sto a sedma contract has no tichá sedma.
- **Sto** (ČSM-O V/6):
  - Made: the sto tariff, **plus the sto tariff again for every 10 points over 100**.
  - Failed: the sto tariff **for every 10 points short of 100, plus for every 10 points of the
    defenders' marriages**.

  This is linear, not doubling (see §8, question 1).
- **Tiché sto** (ČSM-O V/7) doubles the hra, and adds the doubled hra again for every 10 points
  past 100. Every marriage counts toward it.
- **Limit:** no deal pays more than 500 units to or from any one player (ČSM-O V/9).

## 8. Questions for you

1. **How sto scales, in both variations.** The association's general rules scale sto
   **linearly**:
   - Made: +4 for each 10 over. 110 pays 8, and 120 pays 12.
   - Failed: 4 for each 10 short, plus 4 for each 10 of the defenders' marriages. 80 pays 8.

   What shipped in Volený is **Pagat's doubling** (110 pays 8, 120 pays 16), plus the **flat** 4
   for a failure that you signed off. I had not found ČSM-O then; it does answer the open
   question. **Recommendation:** use the ČSM linear rule in Licitovaný, and change Volený to
   match. That is a small change to `settle.go` with new tests.
2. **Which marriage counts toward an announced sto.** ČSM (volený sheet, ČSM-O VII/1) says **the
   first one announced**; we count the **best** one. **Recommendation:** first announced, in both
   variations.
3. **Dvě sedmy has no hra part.** It is a contract on its own (40), with points not counted
   unless "a sto" is added. That is my reading of ČSM-O IV/9-10. Agree?
4. **Nobody bids** means the forhont declares at sedma (§4, point 6). Agree?
5. **Bluffing above sedma is allowed** (§5), as the sheet says. The alternative is to require the
   cards for every part. Agree to allow it?
6. **The 500-unit limit** (§7). It applies to Licitovaný. Should it also apply to Volený, which
   has no cap today?
7. **Left out:** the renonc penalties, the ložená-hra rulings, pauzírovaný four-seat tables, and
   Čl. III (a defender showing four "pomocné" in dvě sedmy). These are tournament bookkeeping, as
   in Volený. Agree?

## 9. Options (Licitovaný defaults)

| Option | Default | Note |
|---|---|---|
| `deals` | 12 | as Volený |
| `tariff` | Association | betl 15, durch 30; the pub table also applies |
| `flekLimit` | **4** | the sheet's limit; "no limit" stays available |
| `redDoubles` | on, fixed | part of the ladder here, so not offered as an option |
| `zLidu` | n/a | there is no trump card to choose from |
| `showCardPoints`, `botSkill`, pause | as Volený | |
