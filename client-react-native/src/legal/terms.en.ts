import type { LegalDocument } from './types';

/**
 * Terms of Use, English — the reference document. Every other locale is
 * checked against this one's section ids and placeholders.
 *
 * Written to be read, not to be impressive: a notice nobody finishes protects
 * nobody. Each clause states one thing, and the clauses a player is most
 * likely to care about (it is free, it is not gambling, results can vanish)
 * come before the ones only a court will read.
 *
 * `{operator}`, `{country}` and `{contact}` are filled from `OPERATOR` in
 * `./index.ts`. Until those are real values the screen marks itself a draft —
 * see `operatorIsNamed`. `{source}` is filled from `SOURCE_URL` in
 * `src/config.ts` and is never a draft: it always has a true answer.
 */
export const termsEn: LegalDocument = {
  id: 'terms',
  title: 'Terms of Use',
  version: '2026-09-01',
  sections: [
    {
      id: 'who',
      heading: 'Who runs Žolíky',
      body: [
        'Žolíky is a free online card game operated by {operator} ("we", "us"). By playing you accept these terms. If you do not accept them, do not play.',
      ],
    },
    {
      id: 'as-is',
      heading: 'The game is provided as is',
      body: [
        'We give no warranty that Žolíky will be available, uninterrupted, or free of errors, or that its rules are implemented without mistakes.',
        'We may change, suspend, or discontinue any part of the game, and may remove inactive accounts, at any time and without notice.',
      ],
    },
    {
      id: 'no-money',
      heading: 'No real money',
      body: [
        'Žolíky involves no wagering, no purchases, and no prizes. It is not a gambling service.',
        'Chips, scores, and standings are virtual. They have no monetary value and cannot be exchanged for money or for anything else.',
      ],
    },
    {
      id: 'account',
      heading: 'Your account is yours to look after',
      body: [
        'You are responsible for what happens under your account, and for the display name and picture you choose.',
        'Do not choose a name that is offensive, that impersonates someone else, or that infringes anyone’s rights. We may change or remove such a name, and may close the account behind it.',
      ],
    },
    {
      id: 'others',
      heading: 'Other players',
      body: [
        'Other players are not under our control. We are not responsible for how they behave, for the names they choose, or for anything they cause you. Table invitations and join codes are yours to share carefully.',
      ],
    },
    {
      id: 'results',
      heading: 'Results and history may be lost',
      body: [
        'Match results, statistics, and standings are provided for entertainment. They may be reset, corrected, or lost, and nothing in these terms entitles you to have them preserved.',
      ],
    },
    {
      id: 'liability',
      heading: 'Liability',
      body: [
        'To the fullest extent permitted by law, we are not liable for any loss or damage arising from your use of Žolíky, including lost data, lost progress, lost time, or the game being unavailable.',
        'Nothing in these terms limits liability that cannot lawfully be limited — including liability for death or personal injury caused by negligence, for fraud, or for intent and gross negligence. If you are a consumer, your mandatory statutory rights are unaffected.',
      ],
    },
    {
      id: 'source',
      heading: 'Žolíky is free software',
      body: [
        'Žolíky is licensed under the GNU Affero General Public License, version 3. You are free to use it, study it, share it, and change it, on the terms that licence sets out — including the warranty disclaimer above, which is its wording as much as ours.',
        'The complete source of the version running here is at {source}. Section 13 of that licence is why this paragraph exists: because you reach Žolíky over a network rather than by installing it, the source has to be offered to you here, on the screen, and not only to whoever downloads a copy.',
        'If you run a modified Žolíky and let other people play it over a network, that licence requires you to offer them your source in the same way.',
      ],
    },
    {
      id: 'law',
      heading: 'Law and contact',
      body: [
        'These terms are governed by the law of {country}. If you are a consumer, you keep the protection of the mandatory rules of the country you live in.',
        'Questions about these terms: {contact}.',
      ],
    },
  ],
};
