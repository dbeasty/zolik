import type { LegalDocument } from './types';

/**
 * Privacy Notice, Czech.
 *
 * Section ids, order, and `{placeholders}` match `privacy.en.ts` — see the
 * note there for where each claim is checked against the code.
 */
export const privacyCs: LegalDocument = {
  id: 'privacy',
  title: 'Zásady ochrany osobních údajů',
  version: '2026-08-31',
  sections: [
    {
      id: 'controller',
      heading: 'Kdo za zpracování odpovídá',
      body: [
        '{operator} provozuje play.limidus.com a určuje, jak se s níže popsanými údaji nakládá. Kontakt: {contact}.',
      ],
    },
    {
      id: 'no-tracking',
      heading: 'Žádná analytika, žádné reklamy, žádné sledování',
      body: [
        'Žolíky neobsahují žádnou analytickou službu, žádnou reklamní síť ani žádné sledovací pixely. Neprofilujeme tě, nesledujeme tě na jiných webech a tvoje údaje neprodáváme ani nesdílíme pro cizí účely.',
        'Následuje všechno, co hra opravdu ukládá, a proč.',
      ],
    },
    {
      id: 'what',
      heading: 'Co ukládáme a proč',
      body: [
        'Abychom tě poznali u stolu: tvoje jméno, zvolený obrázek, kdy účet vznikl, kdy byl naposledy k vidění, a tvoje nastavení hry.',
        'Pokud se přihlašuješ e-mailem: tvoji e-mailovou adresu a údaj, zda byla ověřena. Dokud je přihlašovací kód platný, ukládáme s ním i IP adresu, ze které o něj bylo požádáno, aby šlo odhalit opakované zneužití. Kód i tato adresa se automaticky smažou, jakmile kód vyprší — deset minut po odeslání.',
        'Pokud se přihlašuješ přes poskytovatele, například Google: identifikátor, který tvůj účet v Žolíkách spojuje s tímto poskytovatelem, a e-mailovou adresu, kterou nám poskytovatel předá. Tvoje heslo u něj nikdy nevidíme.',
        'U starších účtů se jménem a heslem: otisk hesla. Přečíst ho neumíme.',
        'Abychom mohli ukázat výsledky: výsledky tvých zápasů, tvoje statistiky a tvoje místo v žebříčku.',
        'Pokud hraješ jako host: náhodný identifikátor uložený ve tvém zařízení, aby sis mezi návštěvami udržel stejnou tvář. Na naší straně o tobě neuchováváme nic nad rámec toho, co stůl potřebuje, dokud u něj sedíš.',
      ],
    },
    {
      id: 'sharing',
      heading: 'Kdo další to vidí',
      body: [
        'Ostatní hráči vidí tvoje jméno a obrázek, když sedíš u stolu nebo čekáš na hru, a tvoje výsledky tam, kde je ukazuje žebříček.',
        'Jinak už jen služby nutné k provozu hry: poskytovatel přihlášení, kterého sis zvolil, e-mailová služba doručující přihlašovací kódy a náš hosting. Nikdo jiný. Tvoje údaje neprodáváme.',
      ],
    },
    {
      id: 'retention',
      heading: 'Jak dlouho je uchováváme',
      body: [
        'Přihlašovací kódy a IP adresa uložená spolu s nimi se automaticky mažou deset minut po odeslání kódu.',
        'Tvůj účet a tvoje výsledky uchováváme, dokud nepožádáš o jejich smazání. Napiš na {contact} a účet, přihlašovací identity k němu připojené i jeho výsledky odstraníme.',
      ],
    },
    {
      id: 'rights',
      heading: 'Tvoje práva',
      body: [
        'Můžeš požádat o kopii údajů, které o tobě máme, o jejich opravu nebo o jejich výmaz, případně proti jejich zpracování vznést námitku. Napiš na {contact}; odpovíme do jednoho měsíce.',
        'Pokud si myslíš, že s tvými údaji nakládáme nesprávně, můžeš podat stížnost u dozorového úřadu. V České republice je jím Úřad pro ochranu osobních údajů (uoou.cz).',
      ],
    },
  ],
};
