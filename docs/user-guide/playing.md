# Playing

This page covers what happens around the cards: finding a game, setting up
a table, inviting people, bots, and finding your way around the match
screen. For the rules of each game, see the [game pages](README.md#the-games).

## The games list

The main screen, **Jokerless**, lists every game under **Pick a game**. A
game's tile can carry badges:

- **Your turn**: a table of this game is waiting for you to move.
- **{n} games going**, or **Playing**: you have unfinished tables of this
  game.
- **{n} waiting**: players are in the waiting room for this game, looking
  for a table.
- **Resume** (**Show** in the Mac app): goes straight back to your latest
  unfinished table of this game.

On a wide screen, your **In progress** and **Finished** tables are listed
beside the games.

The account menu (your face in the top corner) holds everything else:
**My games**, **More**, **Game circle**, **Settings**, **Account** or
**Sign in**, **Sign out** and **About**.

### A game's page

Pressing a game opens its own page. It shows how many players the game takes
and a **Rules ›** link. Below that:

- **Play now**: deals at once, with bots in the other seats. It uses the
  settings you last chose for this game. **Settings ›** next to "Start your
  own" changes them first.
- **Open a table**: sets up a table for friends (see below).
- **Play offline**: hosts the table on this device, with no internet. This
  appears only in the iPhone, Android and Mac apps; see
  [Offline play](offline-and-mobile.md).
- **Learn to play**: a guided lesson. See
  [Getting started](getting-started.md#learn-to-play).
- **Previous games**: every table of this game you have stored, in progress
  or finished.
- **Resume**: back to your latest unfinished table of this game.
- **Join a table**: a box where you type a code or paste an invite link.
- **Tables you can join**: tables of this game that other players opened and
  that still have free seats ("{name}'s table", "2 of 4 seats taken"), with
  **Join**.
- The waiting room: **Make me available to play** puts you on a list that
  hosts of this game can see. "Anyone opening a table can pick you up — they
  don't need a code from you." Press **Stop waiting** to leave the list.
  While you're waiting, the games list shows "You're waiting to play {game}".

Solitaire is a one-player game, so its page has a single **Deal me in**
button instead.

## Setting up a table

**Settings ›** (for Play now) and **Open a table** both lead to the setup
screen, titled "{game} against bots" or "New {game} table". From top to
bottom:

1. **Rules ›**: the written rules, worked out for the variation and options
   currently selected.
2. **Ruleset**: the variation, such as Classic or Samba. A short summary of
   the variation appears under the choices. Picking a different variation
   resets the options to that variation's defaults.
3. **Bots** (when playing against bots): how many bot opponents to deal in.
4. The game's options, one row of choices each. Hover over, focus or
   long-press the **ⓘ** next to an option's name to see what it does.
5. **Deal me in** (against bots) or **Open table** (for friends). The button
   stays at the bottom of the screen however long the list gets.
6. **Learn with these settings**: a lesson for exactly what you've selected.

Your choices are remembered for each game, on this device, and **Play now**
reuses them next time.

### Options most games share

Each game page in this guide lists that game's own options. These are the
options many games have in common:

| Option | Choices | What it does |
| --- | --- | --- |
| **Opponents** | Mixed / Easy / Medium / Hard (and **AI** in Žolíky, Canasta and Poker) | How well the bots play. **Mixed** gives each bot seat its own strength. The default is Medium. |
| **When a player drops** | Wait for them / Bot after 1, 2, 4 or 10 minutes | How long the table waits for a player whose connection dropped before a bot plays for them. They take their seat back as soon as they return. The default is **Wait for them**. |
| **Step out** | Off / Allowed | Lets a player hand their seat to a bot while they step away, and take it back when they return. Off by default. |
| **Hints** | Allowed / Off | Lets players ask for a suggested move. Allowed by default. |
| **Pause between rounds** | Pause / Play straight on | Stop after each round to show the scores, and go on when everyone is ready. |
| **Discard pile** | Open to look through / Top card only | Whether players may look through everything in the discard pile, or only see its top card. |
| **Leave if away for** | 2 / 5 / 10 minutes / Never | Chip games where you can get up and leave (Poker cash tables, Blackjack, Siete y Medio): how long a disconnected player is sat out before they leave the table with their chips. |

House rules show up as options, never as hidden settings. If a table plays a
rule differently, it says so on the setup screen and in the written rules.

## The table lobby

After **Open table** you're the host of a table that hasn't started yet. The
**Your table** screen shows the seats as **Players ({n})** and an
**Invite players** panel.

**Inviting people**

- **Copy link** / **Share link**: "Send this link. Whoever opens it lands at
  this table — no account needed." Whoever opens it can play as a guest.
- **Or read out the code**: the short table code. A friend types it into
  **Join a table** on the games list, or on any game's page.
- **Waiting to play ({n})**: people in this game's waiting room. Press
  **Invite** next to a name to seat them at your table.
- **Game circle**: if you've built a game circle, its members are told when
  you open a table for friends (see [Settings](settings-and-accessibility.md#notifications)).

**Arranging the table** (host only)

- **Add a bot** adds one bot at the table's strength. The **Easy**,
  **Medium**, **Hard** and **AI** buttons next to it add one at that
  strength. AI appears only where a trained model is available.
- Press a bot's strength to change it ("{name}'s strength").
- Use the up and down buttons to move players between seats, or press
  **Shuffle the seats**. In partnership games the screen shows the sides,
  such as "Side 1: Anna + Ben".
- **Connect an AI agent**: lets an AI assistant such as Claude Code take a
  seat through the game server. Paste the command or config it gives you
  into the assistant. The key in it "can play at this table and nothing
  else. Keep it private."
- **Start** deals the cards. If a rematch is holding seats for players who
  haven't arrived yet, the button reads **Start without {names}**, and
  **Seat a bot instead** fills a held seat.

Everyone else sees "Waiting for the host to start…".

### Joining someone else's table

You can join a table in any of these ways:

- Open the invite link. You'll see "Taking a seat at {game}…".
- Type the code (or paste the link) into **Join a table** and press **Join**.
- Press **Join** under **Tables you can join** on a game's page.
- Accept an invite banner from someone in your game circle.

If a link has gone stale: "Ask whoever invited you for a fresh link, or join
with the code instead."

## Bots

Bots are computer players. They fill empty seats, stand in for players who
drop, and are always available for **Play now**.

- **Easy**, **Medium** and **Hard** are hand-written strategies of
  increasing strength.
- **AI** is a trained model. It's offered in Žolíky, Canasta and Poker.
- On the table, a bot's seat has a **BOT** badge. Hover over or long-press it
  to see its strength, for example "A bot, playing at Hard strength". The
  host can press it to change how well it plays.
- When the server is very busy, bots may play simpler moves for a while. The
  seat then shows **SIMPLER** ("Playing simpler moves while the server is
  busy").
- An AI assistant connected as a player shows **✦ AGENT**.

## The match screen

### Around the edges

The header shows the game and variation, and these controls:

- **Rules**: the written rules for this exact table.
- **Read table**: "Hear the whole table: whose turn, the scores, the piles
  and your hand". This is mostly for screen readers. The keyboard shortcut
  is **R**.
- **Step out**: only at tables where Step out is allowed. A bot plays your
  seat until you press **I'm back**.
- **Shortcuts** (web only): lists the keyboard shortcuts.
- The speaker button turns sound effects on or off.
- The look button cycles through the table looks (Heirloom, Casino, Classic,
  High contrast).
- Zoom out, zoom in and the zoom percentage (press the percentage to return
  to 100%). **Layout** lets you move and resize the board's panels.
- **Back to games** leaves the screen. Your seat stays yours, and the table
  waits in **My games**.

The **Players** strip shows every seat: name, face, score (press it for the
**Score breakdown**), whose turn it is, the dealer, and badges such as
**BOT**, **AWAY**, **BOT PLAYING** or **PAUSED**. Press your own seat to
**Change your name and face** for this table.

**Recent moves** lists what just happened. Your turn is announced as
"Your turn: …", and the status line explains what the table is waiting for.

### Your hand

Your cards are fanned along the bottom.

- **Select** a card by tapping or clicking it. Tap it again to **Deselect**.
- **Drag** a card along the fan to rearrange it, or onto the board to play
  it. On a touch screen, hold the card briefly before dragging.
- **Move left** and **Move right** shift the selected card along your hand.
- **Auto-arrange** sorts your hand. In rummy games it keeps likely melds
  together; in other games it sorts suit by suit.
- **Spread the cards out** and **Close the cards up** change how much the
  fan overlaps.
- **Float hand** keeps your hand and the controls at the top of the screen
  while you scroll a large table. Press it again to put them back in the
  page.
- A card the game has **marked** would break a rule if you went on (for
  example, a card you took from the pile and may not discard this turn).
  Press **Why is this card marked?** to find out why.

### Making a move

The **Controls** panel shows the moves the game allows right now, as
buttons, and the controls change with each game and each moment. You can
play in whichever way you find easiest:

- Select cards, then press a control such as a meld, discard or play
  button.
- Select cards, then tap the place on the board where they should go. Piles
  and melds say so: "Press to play the selected cards here."
- Tap a meld or pile first ("Press to aim at this, then pick the cards to
  put here."), then pick the cards.
- Drag the card to where it should go.

If a move could go to more than one place, the app asks "Play {cards}
where?" or "Which move for {cards}?". For an amount, such as a bet or a
raise, use the step buttons or type the number, then press the control.

A greyed-out control has a **why** link. It opens a sheet with **Why not**
(the reason), **The rule** (the sentence in the written rules behind it),
**What you can do** (the way forward) and **Read the full rules →**.

With **Confirm final moves** turned on in Settings, the app asks "Finish the
hand?" before a move that ends the hand, such as going out.

### Undo

Some games let you take a move back while it's still your turn. The undo
control "Takes back your last move", and on the keyboard it's **U**.

- **Žolíky** offers **Undo draw**, **Undo add**, **Undo meld** and
  **Undo turn**, for steps you haven't finished yet. A lay-down you start
  has to be finished, or taken back, before you discard.
- **Canasta** lets you undo the meld moves and wild swaps of the house rules
  (**Undo move**, **Undo swap**), latest first.
- In **Solitaire**, "Undo takes back a move, but never a card turned up or
  drawn".

An undo button only appears when the game allows one.

### Hints

When **Hints** is allowed at the table, the **Hint** button asks "what your
bot would play here" and shows "Suggested: …". It's a suggestion and never
plays for you. If the table has hints turned off, the button says so.

### Between rounds

When a round ends, a short flash announces it, for example **You took it**
or **Anna took it**, along with your new total ("now 42"). The **Results**
panel follows, with **Show score breakdown** for the line-by-line
arithmetic.

If the table pauses between rounds, press **Start the next round**. The
table goes on when everyone is ready. Switch on **Start the next round
automatically** to have it pressed for you after a few seconds, every round
of this game ("Next round in 5s").

### When the match ends

The flash says **You won**, **{name} won**, **Nobody won** or **Match over**
(and in Solitaire, **Not solved**). Under the final results:

- **Your record**: your played, won and lost totals, win rate and streak,
  overall and at this game. Guests are told that no record is kept.
- **Play again**: starts a rematch with the same players. Everyone else sees
  "{name} wants a rematch", with **Join the rematch** or **No thanks**.
- **Play this deal again**: the same cards again, at a new table.
- **Send this deal**: makes a link that lets a friend play exactly the same
  cards at a table of their own ("Can you beat me?"). After you've played a
  deal, **Everyone on this deal** lists the other people who played it.
- The other players at the table can be added to your game circle.
- **Back to games**.

A bot that played part of the match for someone means the game "doesn't
count towards their record". If somebody took over a seat partway through,
the game doesn't count on anyone's record.

## When someone drops

Connections drop. The table copes like this:

- **You're offline. Reconnecting…**: your own connection is down. Nothing
  is lost, and the table catches up when you're back.
- **Waiting for {name}.**: another player has dropped. If the table has a
  **When a player drops** time, the banner counts down ("A bot plays for
  them in …"). The host can **Let a bot play now** or
  **Keep waiting for them**.
- While a bot stands in, the seat shows **BOT PLAYING**. The player takes
  the seat back as soon as they return, and is told "A bot played for you
  while you were away. Your seat is yours again."
- The host can also send a player a fresh link back to their seat with
  **Invite back**: a table link for the device they played on, or a seat
  link that brings one person back to their own seat on any phone or
  browser.
- The host can **Give this seat to someone else**. Whoever opens that link
  plays the seat under their own name, and the original player can't come
  back to it.
- If everyone leaves, the table is eventually set aside: "Nobody came back to
  this table, so it was set aside. The cards are exactly where you left
  them." Press **Pick up where you left off** to bring it back.
- **Opened in another tab — this one has stopped syncing**: the same seat is
  open somewhere else. Press **Use this tab** to play here instead.

In Last Card, the **While a player is away** option decides whether an
absent player is dealt out of the next deals or a bot keeps playing for
them. At chip tables you can get up and leave ("Leave the table? Your chips
go with you and count as your result."), and a player away for longer than
**Leave if away for** is cashed out automatically.

## My games and replays

Account menu › **My games** ("Games you can pick back up, or clear away")
has two tabs:

- **In progress**: **Resume** a table, or **Delete** it. Deleting ends the
  game for everyone still seated and can't be undone.
- **Finished**: **Open** the final position, or **Replay** the game.

The replay steps through every move: **First move**, **Previous move**,
**Play** or **Pause**, **Next move** and **Last move**. You can also filter
to **Your moves** or **Rounds**, and **Mark this move** to come back to it
under **Marks**. Replays show every hand face up.

## Spectating

Anyone who opens the link of a game in progress without a seat at it sees
the table as a spectator: the board and the public cards, but nobody's
hand. A spectator can't make moves or ask for a rematch. There's no list of
games to watch. You need the link.

## More

Account menu › **More** has two extras for signed-in players:

- **Record a live game**: a scorecard for a game you're playing with real
  cards. Enter 2 to 8 names separated by commas, press
  **Start scorecard**, then add each round's scores (for example
  `Alice:120,Bob:80`) with **Save round**. **Export scorecard** shows the
  whole scorecard as text, ready to copy. Nothing recorded here counts towards your stats.
- **Stats & leaderboard**: see [Statistics](faq.md#statistics).
