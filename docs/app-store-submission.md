# App Store submission: Jokerless for iPhone and iPad

What App Store Connect asks for, answered from what the app actually does.
Bundle id `com.jokerless.app`, team Limidus Corp (`U763YTDQ4R`). The first
version is iPhone and iPad, with no push notifications (see "Later").

## Building and uploading

```sh
ZOLIK_IOS_STORE=1 scripts/build-ios.sh                    # .ipa in client-react-native/ios/build/store
ZOLIK_IOS_STORE=1 ZOLIK_IOS_UPLOAD=1 scripts/build-ios.sh # build and upload to App Store Connect
```

Build from a pinned zolik+kdb pair at a released commit, the same as a server
release (`cutting a production release` in the team notes): the app talks to
https://jokerless.com, so it should match what production runs. The version
is `VERSION`; the build number is the UTC time of the build. Uploading needs
the app record below to exist first, and Xcode signed in to the team.

## App record

| Field | Value |
| --- | --- |
| Name | Jokerless (fallback if taken: Jokerless Card Games) |
| Subtitle (30) | Card games with friends & bots |
| Bundle ID | com.jokerless.app |
| SKU | jokerless-ios |
| Primary language | English (U.S.) |
| Primary category | Games › Card |
| Secondary category | Games › Board |
| Price | Free, no in-app purchases |
| Copyright | 2026 Limidus Corp |
| Support URL | https://jokerless.com/about |
| Marketing URL | https://jokerless.com |
| Privacy Policy URL | https://jokerless.com/legal/privacy |

Add Czech as a second localisation: the app, the rules and both legal notices
are written in Czech too.

### Promotional text (170)

Play Canasta, Poker, Gin Rummy, Mariáš and twelve more with friends nearby or online, or against computer opponents. No ads, no purchases.

### Keywords (100)

`canasta,rummy,poker,blackjack,solitaire,gin,card game,multiplayer,offline,bluetooth,friends,mariáš`

### Description

Jokerless is a table for classic card games. Sit down with friends online, with the people in the room when there is no internet, or with computer opponents who play every game properly.

SIXTEEN GAMES
Canasta (with Samba), Hold'em, Omaha and Five-Card Draw poker, Blackjack, Gin Rummy, Rummy Tiles, Last Card, Solitaire, Žolíky, Mariáš, Prší, Sedma, Šnaps, Lóra, Ferbl, Oko bere and Siete y Medio. Each one has its rules written out in the app, with the house rules your family plays by as table options.

PLAY WITH PEOPLE NEARBY, NO INTERNET NEEDED
One phone hosts the table over Wi-Fi or Bluetooth and everyone else joins from theirs. iPhones and Android phones can share a table, and someone without the app can join from a browser on the same Wi-Fi.

PLAY ONLINE
Open a table and share its link, or pick up a game from your game circle. Leave a game and come back to it later on any device.

COMPUTER OPPONENTS
Fill empty seats with bots at the strength you want, from a gentle first game to opponents trained on real play. Ask for a hint when you are stuck.

FAIR AND QUIET
No ads, no purchases, no chips to buy and nothing to win. Jokerless is not a gambling service. It contains no analytics or tracking, and you can delete your account from inside the app.

Free software under the GNU AGPL: the source is at https://github.com/dbeasty/zolik.

## Age rating

Answer the questionnaire as follows. Apple works out the rating from the
answers, so the rating itself is theirs.

- Simulated gambling: **Frequent/Intense**. Poker, Blackjack, Siete y Medio and Oko bere are betting games played for chips. No real money and no prizes; say so where the form allows.
- Everything else (violence, sexual content, profanity, drugs, horror, medical): **None**.
- Unrestricted web access: **No**.
- User-generated content: players choose a display name that other players at the same table see. There is no chat or messaging, and no photos or free text beyond the name.
- Contests, loot boxes, in-app purchases: **No**.

## App Privacy ("nutrition label")

Tracking: **No**, for every type. Nothing is shared with advertisers or data brokers.

| Data type | Collected | Linked to the user | Purpose |
| --- | --- | --- | --- |
| Contact Info › Email Address | Yes, for email sign-in | Yes | App Functionality |
| Identifiers › User ID | Yes, the account id | Yes | App Functionality |
| User Content › Gameplay Content | Yes: moves, results, game history | Yes | App Functionality; Analytics (finished games train the computer opponents, with no names or account ids) |
| Other Data | Game circle (who you added); device time zone | Yes | App Functionality; Analytics (time zone, for rough country counts) |

Not collected in this version: location (the time zone is not a location
reading), contacts, photos, health, financial info, browsing history,
purchases, diagnostics or crash data (there is no crash reporting SDK), and
device identifiers (no push in this version).

Guests are covered by the same answers: a guest's games are kept under a random
id the device holds.

## Review notes

Paste into App Review Information › Notes:

> You can play without an account: choose any game on the first screen and
> "Play against bots", or open the account menu (top right) › Sign in and
> choose "Play as guest".
>
> Sign-in is by a one-time code sent to an email address, which a reviewer
> cannot receive. Please use the demo account below: Sign in › "Sign in with a
> username instead".
>
> Account deletion: account menu (top right) › Account › Delete account ›
> Delete account. It deletes the account on every device; games already played
> stay in the other players' history with the seat renamed.
>
> Play offline (on the first screen) needs no internet: "Start an offline
> table" plays against bots in airplane mode. Playing together without
> internet needs two devices: on one, Play offline › Start an offline table
> and let players nearby join; on the other, Play offline › Tables nearby ›
> Join. The app asks for Local Network and Bluetooth permission only then.
>
> No real money is involved anywhere. The betting games (poker, blackjack)
> use chips that cannot be bought, sold or won.

Demo account: create a username/password account on production for the
reviewers (Sign in › "Sign in with a username instead" › "Create a
username/password account"), play one
game with it so the history is not empty, and enter its username and password
under Sign-In Information. It has to be created by hand: the app's own test
endpoints are off in production.

## Screenshots

Required sizes, portrait:

- iPhone 6.9" (1320 × 2868), e.g. iPhone 17 Pro Max simulator
- iPad 13" (2064 × 2752), e.g. iPad Pro 13-inch simulator

Suggested set (up to ten each): the game picker; a Canasta table mid-game;
poker; Mariáš; the nearby-play host screen; the rules screen; the game circle.

## Before submitting

- [ ] App record created with the values above
- [ ] DSA trader status declared (App Store Connect › Business), or the app is not offered in the EU
- [ ] Demo account created and entered under Sign-In Information
- [ ] Build uploaded and processed; export compliance already answered in the build (`ITSAppUsesNonExemptEncryption = false`: HTTPS only)
- [ ] Screenshots for iPhone 6.9" and iPad 13"
- [ ] Age rating questionnaire, App Privacy answers
- [ ] TestFlight internal test on a real iPhone and iPad before "Submit for Review"

## Later

Push notifications for table invites need an EAS project (`eas init`, which
writes `extra.eas.projectId`) and an APNs key registered with Expo
(`eas credentials`). Until then `push.native.ts` reports notifications as
unavailable and never asks for the permission. When push ships, add Identifiers
› Device ID (App Functionality, linked) to App Privacy.
