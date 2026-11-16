import type { LegalDocument } from './types';

/**
 * Privacy Notice, English — the reference document.
 *
 * Every sentence here is checked against the code, because a privacy notice
 * is the one document where a comfortable generality is a lie:
 *
 *  - the account fields are `models.User` (`server/internal/models/models.go`)
 *  - the provider link is an `identities` document (`internal/identity`)
 *  - the sign-in code, and the request IP stored beside it, are
 *    `models.LoginCode` (`internal/auth/email.go`), retired by a TTL index on
 *    `expiresAt` (`internal/db/mongo.go`) ten minutes after the code is sent
 *  - the guest identifier is kept on the device (`src/context/SessionContext.tsx`)
 *    and recorded server-side: guest sessions (`internal/auth/session.go`,
 *    refresh TTL in `handlers.go`), seats and move logs in matches
 *    (`internal/match`, windows in `internal/app/config.go`), match results
 *    with no expiry (`internal/stats`, moved to an account by `claim.go`), and
 *    the circle, friend code and push devices (`internal/notify`)
 *  - the time zone is `User.TimeZone`, reported on a session refresh
 *  - the game circle and notification devices are `internal/notify`
 *  - training reads finished games through `internal/learnexport`, which
 *    carries no user id or name
 *  - a game played offline reaches the cloud only through the consent list
 *    in `internal/match/localsaves.go`
 *  - the "no analytics" claim is true because no such dependency exists —
 *    check before weakening it, and check before shipping one
 *
 * If any of those change, this document changes in the same commit.
 */
export const privacyEn: LegalDocument = {
  id: 'privacy',
  title: 'Privacy Notice',
  version: '2026-11-15',
  sections: [
    {
      id: 'controller',
      heading: 'Who is responsible',
      body: [
        '{operator} operates Jokerless (jokerless.com, jokerless.org, play.limidus.com and the Jokerless apps) and decides how the data described here is used. Contact: {contact}.',
      ],
    },
    {
      id: 'no-tracking',
      heading: 'No analytics, no ads, no tracking',
      body: [
        'Jokerless contains no analytics service, no advertising network, and no tracking pixels. We do not profile you, we do not follow you across other sites, and we do not sell or share your data for anyone else’s purposes.',
        'What follows is everything the game does store, and why.',
      ],
    },
    {
      id: 'what',
      heading: 'What we store, and why',
      body: [
        'To recognise you at a table: your display name, your avatar choice, when the account was created, when it was last seen, and your game preferences.',
        'If you sign in by email: your email address and whether it has been verified. While a sign-in code is outstanding we also store the IP address it was requested from, so that repeated abuse can be spotted. The code and that address are deleted automatically when the code expires, ten minutes after it is sent.',
        'If you sign in with a provider such as Google: an identifier linking your Jokerless account to that provider, and the email address the provider gives us. We never see your password with that provider.',
        'On older username accounts: a hashed password. We cannot read it.',
        'To show results: your match results, your statistics, and your position on the leaderboard.',
        'Your games: the moves played at each table and how each game ended, so a game can be resumed, replayed and counted. Finished games, with no names or account identifiers in them, are also used to train the computer opponents.',
        'Your game circle: the players you added, the players who added you, and whether you want to hear about their tables.',
        'The time zone your device reports, so we can see roughly which countries players come from. We do not store your location.',
        'If you turn on notifications: a token from your phone\'s or browser\'s notification service, so we can tell you about table invites. It is removed when you turn notifications off, sign out, or delete your account.',
        'Playing with people nearby, over Wi-Fi or Bluetooth, happens between your devices; the phone hosting the table keeps that game. It reaches us only for the players who agree to save it to their account, and the others\' seats arrive without their names.',
        'If you play as a guest, there is no account, but we do keep some things. A random guest identifier is kept on your device and sent to us, so you are the same player between visits. On our side we store your guest sessions (that identifier and your guest display name, until 30 days after you last play). We also store the games you play with that identifier, your name and your avatar in them. Your results are kept, so the games can be added to an account if you sign in later. If you use them, we also keep your game circle (the players you added or who added you, and your friend code) and a notification token. Recent and finished tables are listed under My games.',
      ],
    },
    {
      id: 'sharing',
      heading: 'Who else sees it',
      body: [
        'Other players see your display name and avatar while you are at a table or waiting to play, and see your results where the leaderboard shows them.',
        'Beyond that, only the services needed to run the game: the identity provider you chose to sign in with, the mail service that delivers sign-in codes, our hosting provider, and, if you turn on notifications, the service that carries them (Apple, Google or your browser\'s maker, and Expo, which passes them on to Apple and Google). Nobody else. We do not sell your data.',
      ],
    },
    {
      id: 'retention',
      heading: 'How long we keep it',
      body: [
        'Sign-in codes, and the IP address stored with them, are deleted automatically ten minutes after the code is sent.',
        'Your account and your results are kept until you delete the account. You can do that yourself on the Account screen, or write to {contact} and we will do it for you. It removes the account, the sign-in identities attached to it, your statistics, your game circle, your devices and your score tables.',
        'Games you played are not deleted with your account, because they are in the other players\' history too. Your seat in them is renamed "Player 1", "Player 2" and so on, and no longer leads to you.',
        'Guest play: guest sessions end 30 days after they were last used. Tables, with their moves, are deleted 24 hours after they were opened if they never started, 30 days after they were abandoned, and 90 days after they finished. Results of finished games, the game circle and notification devices are kept until you sign in, which moves them to your account, or until you ask us to delete them. You can remove people from your circle and turn off notifications yourself. Write to {contact} for the rest, with the part of your guest link before the dot (on the Account screen) so we can find it; never send the whole link, which is what lets someone play as you.',
      ],
    },
    {
      id: 'rights',
      heading: 'Your rights',
      body: [
        'You can ask for a copy of the data we hold about you, ask us to correct it, ask us to delete it, or object to our using it. Write to {contact}; we will answer within one month.',
        'If you believe we have handled your data wrongly, you can complain to your national data protection authority. In the Czech Republic that is the Úřad pro ochranu osobních údajů (uoou.cz).',
      ],
    },
  ],
};
