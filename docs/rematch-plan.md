# Seat links and rematch — implementation plan

- **Deliverable A, seat links:** a link per player that brings *that person* back to *their seat*
  at an abandoned table, and tells them who they are before they sit down.
- **Deliverable B, rematch:** "Play again" for a finished table that has other humans at it. It
  opens the same table again and offers each of them their seat back.
- **Non-goals:** spectators; moving a seat to a different account while the game is on;
  carrying a rematch series score across tables (a follow-up, see §6).

---

## 0. What is true today

| Question | Answer today |
|---|---|
| What makes you "that seat"? | The seat id *is* the JWT subject (`match/handlers.go:781`). A guest's id lives only in `localStorage`/SecureStore (`SessionContext.tsx:55-57`). |
| Same browser, abandoned table? | That already works. Opening `/match/{id}` reconnects as the seat, and `canResume` turns on once every human is back (`match/presence.go:159-173`). |
| Different device, or cleared storage? | You can't get back. The page mints a **new** guest (`[matchId].tsx:120-129`), so the seat stays "away" and the table can never be resumed. |
| Played as a guest, signed in later? | Same problem. `ClaimGuest` rekeys stats and the circle but not `matches.players[].id` (`auth/accounts.go:306-335`). |
| Can the table's join link help? | No. `joinLocked` checks `status != lobby` **before** its "already seated" test (`match/manager.go:420-426`), so even a seated player gets `MATCH_ALREADY_STARTED`. |
| Rematch with humans? | No. "Play again" is client-side and only for bot tables (`[matchId].tsx:740-758, 839`). It also drops each bot's skill and persona. |
| Anything linking one match to the next? | No. `models.Match` has no previous or next field. |

---

## Part A — seat links

### A1. The link

`https://<origin>/seat/<token>`. The token is 128 random bits, and only its hash is stored:
`Player.SeatKeyHash` plus `SeatKeyIssuedAt`.

- **Minting:** `POST /matches/{id}/seats/{playerId}/link`. Any seated human at the table can mint
  one for any human seat, while the table is `suspended` or `abandoned`. Minting again replaces
  the old token, so a link sent to the wrong person can be killed.
- **Lifetime:** a token dies when the table leaves `suspended`/`abandoned` (resumed, completed or
  deleted). It also dies after the retention window.

### A2. Opening it — "you are Anna"

`GET /seat/{token}` is unauthenticated and returns a **preview only**: game, variation, the
seat's name and avatar, the other players, and who is present now. No hands, and no `?as=`.

The client route `app/seat/[token].tsx` shows:

> **You're Anna** 🦊 in Canasta (Samba) with Bob and Carol.
> Bob is at the table · Carol is away
> **[ This is me — take my seat ]**   *Not Anna?*

"Not Anna?" goes to the lobby and does nothing else. The name and avatar are the seat's own, so
the person recognises themselves. This is the "they know who they are" part.

### A3. Taking the seat

`POST /seat/{token}/claim` returns a **match-scoped access token**:

- `sub` is the seat id and a new claim is `scope: "match:<id>"`.
- The ws handler and the action routes accept it for that match only. Everything else
  (`/users/me`, notify, stats, other matches) refuses it.
- The client keeps it next to the session, keyed by match id (`seatTokens[matchId]`). The match
  screen uses it for that match's socket and falls back to the normal session everywhere else.

The phone keeps its own identity and the seat stays keyed as it was. Nobody else's identity is
handed out.

If the claimer's own session already *is* the seat, the call is a no-op and it just navigates.

Refusals, each with `err.<CODE>` in every locale and a regenerated `serverKeys.json`:

- `SEAT_LINK_EXPIRED`: the token was rotated, or the table moved on.
- `SEAT_IN_USE`: that seat has a live socket right now. We don't displace someone who is
  actually playing.

### A4. Getting links to people

On the abandoned banner (`[matchId].tsx:1135-1209`), under "Waiting for Anna, Carol", each away
player gets a row: **avatar · name · [Copy link] [Share]**. Share uses `navigator.share` or the
native share sheet, with the text "Anna, your seat at Canasta is waiting: <link>".

It's also pushed. If the away seat maps to someone reachable on `/ws/me` or push, "Nudge" sends
a `seat_waiting` notification that carries the same link:

- It reuses the notify Invite plumbing, with a new message type.
- Rate limit: once per seat per 10 minutes.
- It respects the recipient's "invites off" setting.
- It needs no circle edge: they sat at this table.

### A5. Small fixes that belong with it

- In `joinLocked`, move the "already seated" check above the status check. The ordinary
  `/join/<code>` link then puts a seated player back on a started or abandoned table instead of
  erroring.
- `ClaimGuest` should also rekey `players[].id` on the guest's unfinished matches. Whether this
  belongs here or in its own PR is question 2 in §5.

---

## Part B — rematch

### B1. Server: one route, idempotent

`POST /matches/{id}/rematch`. The caller must be a seated human, and the table must be
`completed` (`MATCH_NOT_COMPLETED` otherwise). It runs under the old table's lock:

1. If `old.NextMatchID` is already set, return it. Two people pressing at once get the same table.
2. Create the new match:
   - The same module, variation and options, and the same seat order.
   - Host is the presser. Bots are copied with their **skill and persona**.
   - `RematchOf = old.ID`.
3. **Reserve** a seat for every other human: `Reserved: [{playerId, name, avatar, state:
   waiting|joined|declined}]`.
   - Reserved seats count against capacity, so a stranger with the join code gets `MATCH_FULL`.
   - A reserved player joins without a code.
4. Set `old.NextMatchID` and broadcast the old table's state again.

`POST /matches/{new}/rematch/decline` marks the caller `declined` and frees the seat.

A bots-only table goes through the same route. With nothing reserved it starts straight away, and
the client's own `playAgain` goes away. One code path, and the bot fix comes with it.

### B2. Reaching the other players

There are two cases.

- **Still looking at the finished table.** The state message gains `rematch: {matchId, hostId,
  seats}`. The match-over banner changes from "Play again" to:
  > **Bob wants a rematch** — Carol joined
  > **[ Join rematch ]**  [ No thanks ]
- **Already left.** A `rematch_invite` goes on `/ws/me` and push. It uses the notify `Invite`
  payload with `rematchOf`, and the URL is `/join/<code>`. `InviteBanner` shows it like a table
  invite with the wording "Rematch with Bob". Like A4, it needs no circle edge.

Whichever way they arrive, accepting is the ordinary join. They land in `lobby/join`, and the
host's `lobby/table` shows the reserved seats with waiting, joined and declined chips.

### B3. Starting

The host starts when they choose. For seats still in `waiting` at start, the host picks one of:

- **[Fill with bot]**
- **[Start without]**, if the variation allows the smaller count
- keep waiting

When the lobby closes, `invite_revoked` fires for the reservations still outstanding.

### B4. Client

- `[matchId].tsx`: the rematch banner states and the Share row from A4.
- `lobby/table.tsx`: reserved-seat chips and the fill-with-bot choice.
- `InviteProvider` / `InviteBanner`: handle `rematch_invite` and `seat_waiting`.
- The TUI's lobby can come later.

---

## 3. Order of work (one PR each)

1. **A5 join-order fix.** Tiny, and it helps immediately.
2. **Server rematch (B1) + banner (B2, first case).** Covers the common case: everyone is still on
   the finished screen.
3. **Rematch notifications (B2, second case) + host start choices (B3).**
4. **Seat links (A1–A3)**, including the match-scoped token in auth and the ws handler.
5. **Seat link sharing + nudge (A4).**

Every PR that adds a refusal code adds `err.<CODE>` to all locales and regenerates
`serverKeys.json`.

## 4. Tests

- **Go:**
  - rematch is idempotent under concurrent presses;
  - reservations block strangers but admit the reserved player without a code;
  - bot skill and persona are carried over;
  - a scoped token is refused outside its match;
  - claim is refused while the seat's socket is live;
  - the join-order fix.
- **e2e (two browser contexts):**
  - finish a match, A presses rematch, B sees the banner and joins, host starts;
  - abandon a table, clear B's storage, open B's seat link, see "You're B", claim, and resume
    becomes available to both.

  Keep the default bot think time. Assert on what is visible (banner text, seat chips), not only
  on API state.

## 5. Decisions (settled 2026-09-29)

1. **Who may start a rematch?** Any seated human; the first press wins and hosts.
2. **Guest → account rekey (A5):** its own PR, because it touches live module state keyed by
   player id.
3. **Trust in A1.** Any seated human can mint a link for another seat, so Bob could open Anna's
   link himself and play both hands. That's acceptable among people who chose to play together.
   The mitigations are the `SEAT_IN_USE` refusal and link rotation. Accepted as it stands.

## 6. Follow-ups

- A rematch series score ("Bob 2 – 1 Anna"), by walking the `RematchOf` chain.
- TUI support.
