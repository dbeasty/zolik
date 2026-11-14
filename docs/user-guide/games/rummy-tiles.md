# Rummy Tiles

Rummy played with numbered tiles in four colours. The whole table belongs to
everyone, and on your turn you may rearrange it however you like, as long as
everything is valid when you finish.

- **Players:** 2–4
- **Variation:** Standard
- **Bots:** Easy, Medium, Hard
- **In-app rules:** the game's page › **Rules ›**

## Setup

The game uses 106 tiles: the numbers 1 to 13 in four colours (red, blue,
orange and black), twice over, plus jokers. Each player is dealt 14 tiles.
The rest form the pool.

## Sets

- A **group** is three or four tiles of one number, each a different colour.
- A **run** is three or more consecutive numbers in one colour. 13 doesn't
  run back around to 1.
- A **joker** stands for any tile.
- A tile is worth its number. A joker in a set is worth the tile it stands
  for. A joker left in your hand costs 30.

## The initial meld

Until you've laid 30 or more points in a single turn, from your own hand
alone, you may not touch anything already on the table.

## How a turn works

Play at least one tile from your hand, rearranging the table freely, and end
your turn with every set on the table valid. Splitting runs, moving tiles
between sets and adding to anyone's sets are all allowed. The table can be
**put back** to how it was at the start of your turn if you get in a muddle.

There's no discard. If you can't complete a valid turn, you **draw one tile**
instead.

A joker on the table may be taken by replacing it with the tile it stands
for, from your hand. It must be used in a set before your turn ends.

## Ending a round and scoring

The first player out of tiles wins the round. Everyone else scores the
negated value of what's left in their hand, and the winner scores the sum of
what everyone else lost.

If the pool runs dry and nobody can play, the round ends. By default the
lowest hand wins it. With **Pool exhaustion** set to "Nobody wins the round",
every hand is simply scored.

The first player to pass the **Target score** (200 by default) wins the match.
With a **Round limit** instead, the match ends after that many rounds and the
highest score wins. If you set both, whichever comes first ends the match. If
you set both to Off, the match has no end, so keep at least one of them on.

## Options

| Option | Choices | Default | What it does |
| --- | --- | --- | --- |
| **Target score** | Off / 200 / 300 / 500 | 200 | First to pass this score wins. Off uses the round limit instead. |
| **Round limit** | Off / 5 / 10 / 15 | Off | The match ends after this many rounds, and the highest score wins. Off uses the target score instead. |
| **Pool exhaustion** | Lowest hand wins the round / Nobody wins the round | Lowest hand wins the round | Who, if anyone, wins a round that ends with the pool dry. |

Also available: Pause between rounds, Opponents, When a player drops, Step
out, Hints.

## Tips

- Look for ways to break up the table's runs to slot in your tiles. The big
  plays often come from rearranging.
- Hold on to a joker only while it's useful. Stuck in your hand at the end,
  it costs 30.
