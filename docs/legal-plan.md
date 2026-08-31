# Legal notices — plan

**Status: implemented.** Sections 1–5 and 7 are built; the documents themselves
live in `client-react-native/src/legal/` and are the source of truth for their
wording — §8 below is kept only as the record of what was drafted and why, not
as a second copy to maintain. What is still open is §6: four facts about the
operator, and the account-deletion endpoint the privacy notice's promise leans
on. Until the four facts are filled in, both screens mark themselves a draft.

Goal: publish a disclaimer that limits liability, and a statement about data,
reachable from inside the game. You are right that a game needs one — the
exposure is not gambling losses, it is the ordinary "this service ate my
account / my stats / my evening" claim, plus the EU transparency duty that
attaches the moment you store an email address for a Czech player.

**I am not a lawyer and this plan is not legal advice.** Everything below is
engineering: what is factually true about this codebase, where the wording
belongs, and what has to change so the wording is not a lie. The final text
should get a lawyer's pass before it goes on play.limidus.com.

## 1. The one thing to fix before writing a word

The request said "we do not collect the data". As written that is false, and a
false privacy statement is strictly worse than no statement: under the GDPR the
transparency information itself must be accurate, so an untrue notice is its own
violation, and in a dispute a demonstrably false statement of fact undermines
the very document you were relying on for protection.

Zolik collects a fair amount. What it genuinely does *not* do is the thing
players actually care about — no analytics SDK, no ad network, no tracking
pixels, no selling anything to anyone. A search for `analytics|telemetry|
sentry|gtag|posthog|mixpanel` across the server and the client returns nothing.
That is a strong, true, unusual claim. Make *that* claim, precisely, instead of
a blanket denial that any reader can disprove by signing in with Google.

## 2. What is true today (the facts the wording must match)

| Data | Where | Why | Evidence |
| --- | --- | --- | --- |
| Username, email, `emailVerified`, `passwordHash` (legacy accounts only), `authProvider`, `avatarUrl`, `createdAt`, `lastSeenAt`, preferences | Mongo (or kdb) `users` | The account exists | `server/internal/models/models.go:33` |
| Identity `(provider, subject)` pairs from Google / OIDC | `identities` | Sign-in | `server/internal/identity/providers.go` |
| Email address handed to an SMTP sender for one-time codes | outbound mail | Email sign-in | `server/internal/auth/mailer.go` |
| Client IP, best-effort, "for abuse investigation only" | request-scoped | Abuse | `server/internal/auth/handlers.go:562` |
| Match results, stats, leaderboard entries | `stats` | The feature is called Stats & leaderboard | `server/internal/stats/` |
| Username shown to other players while waiting | live, not stored | The waiting room lists who is around | `client-react-native/app/index.tsx` (`WaitingStatusCard`) |
| A persistent guest id on the device | device storage | A returning guest keeps their face | `src/context/SessionContext.tsx` (`loadGuestId`) |
| **Nothing** for analytics, ads, tracking, or profiling | — | — | no such dependency exists |

Two more facts that shape the text:

- **No real money anywhere.** No payment code, no purchases, no wagering. The
  Hold'em module deals play chips. Say so explicitly — "virtual chips have no
  monetary value and cannot be exchanged for anything" — because a poker table
  invites the gambling question even when the answer is no.
- **Usernames and table names are user-supplied and visible to other players.**
  That is the only user-generated content surface, and it is the one the "not
  responsible for what users do" clause actually needs to cover.

## 3. What to publish

Three pieces, not one. Mixing them produces a document that is too long to read
and too vague to rely on.

1. **Terms of Use & Disclaimer** — service provided "as is"; no warranty of
   availability, accuracy, or fitness; liability limited to the extent the law
   allows; no responsibility for other players' conduct or for the names they
   choose; no guarantee that accounts, stats, or match history survive; free
   service, may change or stop at any time; not a gambling service.
2. **Privacy Notice** — the table in §2, in prose: what is stored, why, who it
   is shared with (identity provider, mail sender, hosting — nobody else), how
   long, and how to get it deleted.
3. **A one-line notice at the point of entry** — on the guest screen and the
   sign-in screen: "By playing you agree to the Terms. See how your data is
   handled." Two links, no checkbox. A checkbox is a bigger claim (it asserts
   affirmative consent you would then have to record) and is not needed for a
   free game with no payment.

## 4. Where it lives in the code

New route, following the shape `app/rules.tsx` already has — a static content
screen reached from elsewhere:

- `client-react-native/app/legal/terms.tsx`
- `client-react-native/app/legal/privacy.tsx`
- Register both in `app/_layout.tsx` alongside `<Stack.Screen name="rules" …>`,
  titles `'Terms'` and `'Privacy'`.
- Render with `Screen` + `ScrollView` and the existing `shared` styles; no new
  visual vocabulary. Per the skins rule, this is plain text on the surface
  colour — nothing here is skinned.

Entry points, in order of how likely a player is to use them:

- `src/components/BuildFooter.tsx` — add a third line, `Terms · Privacy`, as
  two `Pressable`s routing to the screens. The footer is already on the main
  menu and is already the "boring metadata" slot, so this costs no layout.
  Rename it or leave the name; `BuildFooter` growing a legal line is honest
  enough given it is the footer.
- `app/auth/guest.tsx` and `app/auth/login.tsx` — the one-line notice above the
  submit button.
- `app/settings.tsx` — a link, for the player who goes looking later.

On web the same routes serve as `/legal/terms` and `/legal/privacy` from the
Expo build behind nginx; `+html.tsx` needs no change. Those two URLs are also
what Apple and Google will ask for as the privacy-policy link if the RN client
is ever submitted to a store — worth having them stable now.

Out of scope unless you want it: `client-tui` and `client-defold` get no notice.
The TUI is reached over SSH by people you know; the Defold client is not shipped.

## 5. Wording lives in i18n, not in JSX

Two locales are enforced by a test (`src/lib/i18n.test.ts` asserts every bundle
has exactly the English key set, no blanks, matching placeholders), so English-
only legal text would fail CI the moment it is added to a bundle — and shipping
Czech players an English-only privacy notice is a bad idea anyway.

But 40 paragraphs of prose do not belong in the flat `i18n.ts` key map, which is
built for short interface strings. Proposal:

- New `client-react-native/src/legal/` with `terms.en.ts`, `terms.cs.ts`,
  `privacy.en.ts`, `privacy.cs.ts`, each exporting `{ version, updated,
  sections: { id, heading, body[] }[] }`.
- A `src/legal/index.ts` picking the document by the current locale, falling
  back to English exactly as `i18n.t` does.
- A `src/legal/legal.test.ts` mirroring the i18n parity test: same section ids
  in every locale, no empty bodies, same `version` string. This is the same
  failure mode the i18n test exists to catch — a half-translated document that
  ships because the fallback quietly hides it.
- Only the *short* strings go in `i18n.ts`: `legal.terms`, `legal.privacy`,
  `legal.notice`, `legal.updated`.

No server work, so `serverKeys.json` does not move: these are client-authored
strings, not server message keys, and nothing here is passed through a variable
into a server key. No regeneration, no CI wording check.

Keep a `version` on each document (e.g. `2026-08-31`). It costs nothing now and
is the only way to answer "which terms did they play under" later.

## 6. Gaps that must close for the privacy notice to be truthful

These are the items where the honest text cannot be written until code changes.
Each is small; none blocks publishing the Terms, only the Privacy notice.

1. **No way to delete an account.** The route table at
   `server/internal/auth/handlers.go:102-137` has no delete and no export. A
   privacy notice that says "contact us to delete your account" is fine as an
   interim answer *only if* someone can actually do it by hand — decide which,
   and if it is by hand, name a real email address in the notice.
   Recommended follow-up: `DELETE /auth/account`, removing the user, their
   identities, sessions, and either deleting or anonymising stats rows.
2. **Retention is undefined.** Today nothing expires: guest accounts, sessions,
   and stats live forever. Either say "kept until you ask us to delete it",
   which is true, or add expiry. Do not write a number the code does not honour.
3. ~~**IP handling.**~~ *Resolved.* It is persisted, and it expires on its own:
   `StartSignIn` writes the request IP onto the `LoginCode` document
   (`internal/auth/email.go:105`), and a TTL index on `expiresAt`
   (`internal/db/mongo.go:121`) deletes the code and the address with it ten
   minutes after the code is sent (`codeTTL`). The privacy notice says exactly
   that, and the e2e spec asserts the sentence is on screen — so shortening or
   lengthening that TTL without revisiting the notice turns a test red.
4. **Named operator and contact address.** There is no LICENSE, no operator
   name, no contact anywhere in the repo. A disclaimer with no identifiable
   party behind it is weak, and the GDPR notice requires naming the controller.
   This is the one input only you can supply: legal name (or "a private
   individual operating play.limidus.com"), country, and a contact email.
5. **Sub-processors.** The notice should name the categories actually in play:
   the identity providers you enable, the mail sender configured in
   `internal/app/config.go`, and your hosting. Confirm which are live in prod.

## 7. Tests

- `src/legal/legal.test.ts` — parity, as in §5.
- `e2e/tests/legal.spec.ts` — from the main menu, click the footer's Terms
  link, assert a known heading is *on screen*; same for Privacy; assert the
  guest screen shows the notice line. Asserting the rendered text rather than
  the route is the point: a link that navigates to a blank screen would pass a
  routing test.
- Baseline any flake against unmodified `main` before blaming this branch.

## 8. Draft text — superseded by the code

The wording below is what was drafted here first. It now lives, with its Czech
translation, in `client-react-native/src/legal/{terms,privacy}.{en,cs}.ts`,
which is where to change it — two copies of a legal document is how a document
ends up saying two things. It is left here as the record of the reasoning.

Non-lawyer draft. Short on purpose; a notice nobody reads protects nobody.

### Terms of Use

> Zolik is a free online card game operated by **[OPERATOR]** ("we"). By playing
> you accept these terms.
>
> **The game is provided as is.** We give no warranty that it will be
> available, uninterrupted, error-free, or that the rules are implemented
> without mistakes. We may change, suspend, or discontinue any part of it, and
> may delete inactive accounts, at any time and without notice.
>
> **No real money.** Zolik involves no wagering, no purchases, and no prizes.
> Chips, scores, and standings are virtual, have no monetary value, and cannot
> be exchanged for money or anything else.
>
> **Your account is yours to look after.** You are responsible for what happens
> under your account and for the name and picture you choose. Do not choose a
> name that is offensive, impersonates someone, or infringes anyone's rights.
>
> **Other players.** We do not control how other players behave and are not
> responsible for their conduct, their names, or anything they cause you.
>
> **Results and history may be lost.** Match results, statistics, and standings
> are provided for entertainment. They may be reset, corrected, or lost, and
> nothing here entitles you to their preservation.
>
> **Liability.** To the fullest extent permitted by law, we are not liable for
> any loss or damage arising from your use of Zolik, including lost data, lost
> progress, or unavailability. Nothing in these terms limits liability that
> cannot lawfully be limited — including for death, personal injury, fraud, or
> intent and gross negligence — and, if you are a consumer, your mandatory
> statutory rights are unaffected.
>
> **Law.** These terms are governed by the law of [COUNTRY]. If you are a
> consumer, you keep the protection of the mandatory rules of your own country
> of residence.
>
> Version [DATE]. Questions: [CONTACT EMAIL].

### Privacy Notice

> **[OPERATOR]** operates play.limidus.com and decides how the data described
> here is used. Contact: [CONTACT EMAIL].
>
> **We run no analytics, no advertising, and no tracking.** Zolik contains no
> analytics SDK, no ad network, and no tracking pixels. We do not profile you,
> and we do not sell or share your data for anyone else's purposes.
>
> **What we store, and why**
> - *To let you sign in and be recognised at a table:* your username, your
>   avatar choice, when the account was created, when it was last seen, and your
>   game preferences.
> - *If you sign in by email:* your email address and whether it is verified.
>   Codes are sent through our mail provider.
> - *If you sign in with a provider such as Google:* an identifier linking your
>   account to that provider, and the email address it gives us. We never see
>   your password with that provider.
> - *On older username accounts:* a hashed password. We cannot read it.
> - *To show results:* your match results, statistics, and leaderboard position.
> - *If you play as a guest:* a random identifier stored on your device so you
>   keep the same face between visits. No account is created on our side beyond
>   what a table needs.
> - *For abuse investigation:* your IP address may be seen by the server when
>   you sign in. [Confirm before publishing whether it is stored, and for how long.]
>
> **Who else sees it.** Other players see your username and avatar while you are
> at a table or waiting to play. Beyond that, only the services needed to run
> the game: the identity provider you chose to sign in with, our mail provider,
> and our hosting provider. Nobody else.
>
> **How long.** We keep your account and results until you ask us to delete
> them. [Update once retention or a delete endpoint exists.]
>
> **Your rights.** You can ask for a copy of your data, ask us to correct it, or
> ask us to delete your account and everything attached to it. Write to
> [CONTACT EMAIL] and we will act within one month. You may also complain to
> your national data protection authority (in the Czech Republic, the Úřad pro
> ochranu osobních údajů).
>
> Version [DATE].

## 9. Order of work

1. ~~Legal text in `src/legal/` (en + cs) with the parity test.~~ Done.
2. ~~The two screens, the `_layout.tsx` entries, and the footer link.~~ Done,
   plus a link from Settings.
3. ~~The notice line on `auth/guest.tsx` and `auth/login.tsx`.~~ Done.
4. ~~`e2e/tests/legal.spec.ts`.~~ Done.
5. **You supply**, in `OPERATOR` in `src/legal/index.ts`: legal name, country,
   and contact email. Flip the assertion in `legal.test.ts` that currently pins
   `operatorIsNamed()` to `false`, and delete the draft-banner e2e case. The
   banner then disappears from both screens on its own.
6. Confirm which identity providers and which mail sender are live in
   production, and correct §sharing of the privacy notice if it is not what it
   says.
7. Follow-up PR: `DELETE /auth/account` plus the stats anonymisation, then
   tighten the retention and deletion paragraphs from "write to us" to the real
   behaviour.
8. Lawyer's pass on the final wording before the draft banner comes off.
