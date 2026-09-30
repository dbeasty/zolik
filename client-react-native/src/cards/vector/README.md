# The card deck

The faces on every card in this client are **Vectorized Playing Cards 3.0** by
Chris Aguilar — an engraved deck of the pattern that has been in print since
the 1880s — not artwork this repository draws.

    Vector Playing Cards 3.0
    https://totalnonsense.com/open-source-vector-playing-cards/
    Copyright 2011,2019 - Chris Aguilar - conjurenation@gmail.com
    Licensed under: LGPL 3.0 - https://www.gnu.org/licenses/lgpl-3.0.html

## Why this directory exists at all

Everything in here is **LGPL 3.0, not Zolik's AGPL**. That is the whole reason
the art and the file generated from it sit in their own directory instead of
next to the components that draw them: the licence boundary is a directory you
can point at, rather than a fact somebody has to remember.

Combining the two is fine — LGPL 3.0 is compatible with AGPL 3.0, and the
combined work goes out under the AGPL. The obligation LGPL adds is that anyone
receiving the program can replace this artwork with their own and still have a
working program. Because Zolik is AGPL and publishes its complete source, that
is already true: the SVGs are right here, `art/` is the modifiable form, and
one command rebuilds the client's copy.

## What is in here

| | |
|---|---|
| `art/` | The deck, 55 SVGs, as vendored. The *source form* — edit these. |
| `faces.ts` | Generated from `art/`. Do not edit; see below. |
| `NOTICE.txt` | Aguilar's own licence file and his attribution instructions, verbatim. |
| `LICENSE.LGPL-3.0.txt` | The LGPL, verbatim. |

`art/` holds 55 files but the client ships 53 faces: the deck has three jokers
and the game deals one. `j01` and `j02` are kept so switching joker is a
one-line change in the generator rather than a trip back to the source deck.

The SVGs are the published deck run through `svgo --multipass -p 1`. That
rounds coordinates to one decimal, which takes the deck from 3.4 MB to 824 KB
and is indistinguishable from the original at every size a card is ever drawn —
checked at 150px, 78px and 52px before it was chosen.

## Regenerating

    node scripts/gen-card-faces.js

Run it by hand after changing anything in `art/`, and commit the result.
Nothing runs it automatically: it is a build step nobody needs on the critical
path of a change that has nothing to do with cards.

`scripts/gen-card-faces.js` explains what it does to the art and why — briefly:
it strips each card's white stock (the skin draws that), turns the deck's five
flat colours into named tokens a skin can restate, and emits the paths as data
so no XML is parsed at runtime.

## Changing how the deck looks

Not here. A skin says what the deck's five inks are, in `cardPalette` — see
`src/skins/heirloom.ts` for one that restates the deck in ivory and gold, and
`src/skins/types.ts` for what a skin is and is not allowed to decide. A skin
that says nothing gets the colours the deck was printed in.
