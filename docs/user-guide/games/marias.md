# Mariáš

The great Czech trick-taking game. Each deal, one player (the declarer)
plays a game alone against the other two. Win units from them, and after the
last deal the player with the most units wins.

- **Players:** 3–4 (with four, the dealer sits each deal out)
- **Variations:** **Chosen trumps** (Volený mariáš) and **Auction**
  (Licitovaný mariáš)
- **Bots:** Easy, Medium, Hard
- **In-app rules:** the game's page › **Rules ›**. The rules are long, and
  the in-app version follows your exact table settings.

The English version of the app uses English names for the games and calls:
Game, Seven, Hundred, Betl, Durch, and "good" and "bad" for the answers. One
or two option descriptions still use the Czech words (hra, sedma, sto).

## The cards

32 German-suited cards in hearts, bells, acorns and leaves: 7, 8, 9, 10,
Unter, Ober, King and Ace.

- In Game, Seven and Hundred, the order is Ace, 10, King, Ober, Unter, 9, 8,
  7.
- In Betl and Durch, the order is Ace, King, Ober, Unter, 10, 9, 8, 7.
- Each Ace and 10 is worth 10, and the last trick another 10: 90 in all.

## The games

| Game | To win |
| --- | --- |
| **Game** | Take more points than both defenders together, counting marriages. |
| **Seven** | Win the last trick with the 7 of trumps. Announced with Game or Hundred, and settled separately. |
| **Hundred** | Reach 100 with card points and the first marriage announced. It replaces Game, and may be announced with Seven. |
| **Betl** | Take no trick. No trumps. |
| **Durch** | Take every trick. No trumps. |
| **Two Sevens** (Auction only) | Name trumps and a helper suit, win the trick before last with the helper suit's 7 and the last trick with the trump 7. |

## Chosen trumps (Volený)

1. The player left of the dealer is the **chooser**. They get 7 cards, and
   the others get 10 each.
2. The chooser names trumps by laying one of their first seven cards face
   down. With **Blind trumps** allowed, they may instead take a card unseen
   from the five they haven't looked at.
3. They then pick up the rest (12 cards in all), announce a game and lay two
   cards away to the talon. In Game, Seven and Hundred neither may be an Ace
   or a 10. The talon counts as the declarer's. The chooser may not pass.
4. In Game, Seven and Hundred, the trump card is turned face up so the others
   know trumps before they answer.
5. Each other player answers **good** (let the game stand) or **bad** (take
   it over with a higher game: Betl over Game, Seven or Hundred; Durch over
   any of those or over Betl). Whoever takes over picks up the talon, lays
   two away and becomes the declarer.
6. A defender may announce **Seven against** or **Hundred against**.

## Auction (Licitovaný)

Everyone gets 10 cards, and the other 2 lie face down as the talon. The
dealer bids first against the player on the dealer's left (the forehand).
When one of them passes, the third player takes their place. If nobody bids,
the forehand plays Seven.

The ladder, from low to high: Seven, Seven in hearts, Hundred, Hundred and
Seven, Hundred in hearts, Hundred and Seven in hearts, Betl, Durch, Two
Sevens, Two Sevens and Hundred, and both Two Sevens contracts with hearts as
trumps. The winner takes the talon and announces any contract at or above the
rung won. Defenders announce nothing of their own. A declarer held to plain
Seven may fold instead of playing, paying 6 to each defender.

## Doubling

Each part (the game, and each Seven) can be doubled on its own. The side that
didn't announce it doubles first, then the sides take turns doubling again
(called flek, re, tutti, boty, kalhoty in Czech), until a full circle passes
without a doubling. The **Doubling limit** option caps how many times.

## Play

- The declarer leads to the first trick in Chosen trumps. In Auction, the
  forehand leads, except in Betl and Durch, where the declarer does. Whoever
  wins a trick leads the next.
- **Follow suit** if you can, and **beat** the best card of that suit in the
  trick if you can. Once the trick has been trumped, any card of the suit led
  will do.
- If you can't follow, **play a trump**, and beat any trump already in the
  trick if you can. With neither, play anything.
- Whoever announced a Seven keeps the 7 of trumps for the last trick, unless
  no other card is legal.
- Play the Ober while holding the King of the same suit to announce a
  **marriage**: 20, or 40 in trumps. There are no marriages in Betl or Durch.
- Betl ends at the declarer's first trick, and Durch at their first trick
  lost.

## Scoring

- The game and each Seven are settled separately, between the declarer and
  each defender.
- Game 1, Seven 2, Hundred 4, Betl 15, Durch 30 (and Two Sevens 40) units,
  with the **Association** stakes. **Pub** stakes make Betl 5 and Durch 10.
- An announced Hundred pays its value again for every ten points past 100.
  A failed Hundred pays for every ten points short.
- A side that reaches 100 in Game without announcing it (a quiet Hundred)
  doubles the game, and is paid the doubled game again for every ten points
  past.
- An unannounced 7 of trumps that wins the last trick is worth 1. So is one
  beaten in the last trick, paid to the other side.
- Each doubling doubles that part. With hearts as trumps every payment is
  doubled (with **Hearts pay double** on).
- No deal pays more than 500 between the declarer and any one defender.

With four players, the dealer sits each deal out and neither pays nor is
paid, and the match is rounded up to a multiple of four deals.

## Options

| Option | Choices | Default | What it does |
| --- | --- | --- | --- |
| **Deals** | 9 / 12 / 18 / 24 | 12 | How many deals a match lasts. |
| **Stakes** | Association (betl 15, durch 30) / Pub (betl 5, durch 10) | Association | What each game is worth. |
| **Hearts pay double** | On / Off | On | With hearts as trumps, every payment is doubled. |
| **Doubling limit** | No limit / One doubling / Up to two doublings / Up to four doublings | Chosen trumps: No limit; Auction: Up to four doublings | How many times one game may be doubled. |
| **Blind trumps** | Allowed / Not allowed | Chosen trumps: Allowed; Auction: Not allowed | The chooser may take trumps from an unseen card. |
| **Card points** | Hidden until the end / Shown as you play | Shown as you play | Show each side's card points during the deal. |
| **Open betl and durch** | Played closed / Declarer's hand shown | Played closed | After the first trick of a Betl or Durch, the declarer's cards are laid face up. |

Also available: Opponents, When a player drops, Step out, Hints, Pause
between rounds.

## Tips

- Count the Aces and 10s. Ninety points are in play, and holding them decides
  Game.
- Keep the King with its Ober for a marriage: 40 in trumps is a big swing.
- Before announcing Seven, make sure you can keep the trump 7 safe to the
  end.
