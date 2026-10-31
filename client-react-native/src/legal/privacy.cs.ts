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
  version: '2026-10-31',
  sections: [
    {
      id: 'controller',
      heading: 'Kdo za zpracování odpovídá',
      body: [
        '{operator} provozuje Jokerless (jokerless.com, jokerless.org, play.limidus.com a aplikace Jokerless) a určuje, jak se s níže popsanými údaji nakládá. Kontakt: {contact}.',
      ],
    },
    {
      id: 'no-tracking',
      heading: 'Žádná analytika, žádné reklamy, žádné sledování',
      body: [
        'Jokerless neobsahuje žádnou analytickou službu, žádnou reklamní síť ani žádné sledovací pixely. Neprofilujeme tě, nesledujeme tě na jiných webech a tvoje údaje neprodáváme ani nesdílíme pro cizí účely.',
        'Následuje všechno, co hra opravdu ukládá, a proč.',
      ],
    },
    {
      id: 'what',
      heading: 'Co ukládáme a proč',
      body: [
        'Abychom tě poznali u stolu: tvoje jméno, zvolený obrázek, kdy účet vznikl, kdy byl naposledy k vidění, a tvoje nastavení hry.',
        'Pokud se přihlašuješ e-mailem: tvoji e-mailovou adresu a údaj, zda byla ověřena. Dokud je přihlašovací kód platný, ukládáme s ním i IP adresu, ze které o něj bylo požádáno, aby šlo odhalit opakované zneužití. Kód i tato adresa se automaticky smažou, jakmile kód vyprší — deset minut po odeslání.',
        'Pokud se přihlašuješ přes poskytovatele, například Google: identifikátor, který tvůj účet v Jokerless spojuje s tímto poskytovatelem, a e-mailovou adresu, kterou nám poskytovatel předá. Tvoje heslo u něj nikdy nevidíme.',
        'U starších účtů se jménem a heslem: otisk hesla. Přečíst ho neumíme.',
        'Abychom mohli ukázat výsledky: výsledky tvých zápasů, tvoje statistiky a tvoje místo v žebříčku.',
        'Tvoje hry: tahy zahrané u každého stolu a to, jak hra skončila, aby šla dohrát, přehrát a započítat. Dohrané hry bez jmen a identifikátorů účtů používáme také k trénování počítačových soupeřů.',
        'Tvůj herní kruh: hráče, které sis přidal, hráče, kteří si přidali tebe, a to, jestli chceš vědět o jejich stolech.',
        'Časové pásmo, které hlásí tvoje zařízení, abychom zhruba věděli, ze kterých zemí hráči jsou. Tvoji polohu neukládáme.',
        'Pokud zapneš oznámení: token od služby oznámení ve tvém telefonu nebo prohlížeči, abychom ti mohli dát vědět o pozvánkách ke stolu. Smaže se, když oznámení vypneš, odhlásíš se nebo smažeš účet.',
        'Hra s lidmi poblíž přes Wi-Fi nebo Bluetooth probíhá mezi vašimi zařízeními; hru si nechává telefon, který stůl hostí. K nám se dostane jen za hráče, kteří souhlasí s jejím uložením do svého účtu, a místa ostatních přijdou bez jmen.',
        'Pokud hraješ jako host: náhodný identifikátor uložený ve tvém zařízení, aby sis mezi návštěvami udržel stejnou tvář. Na naší straně o tobě neuchováváme nic nad rámec toho, co stůl potřebuje, dokud u něj sedíš.',
      ],
    },
    {
      id: 'sharing',
      heading: 'Kdo další to vidí',
      body: [
        'Ostatní hráči vidí tvoje jméno a obrázek, když sedíš u stolu nebo čekáš na hru, a tvoje výsledky tam, kde je ukazuje žebříček.',
        'Jinak už jen služby nutné k provozu hry: poskytovatel přihlášení, kterého sis zvolil, e-mailová služba doručující přihlašovací kódy, náš hosting a, pokud zapneš oznámení, služba, která je doručuje (Apple, Google nebo výrobce tvého prohlížeče a Expo, které je předává Applu a Googlu). Nikdo jiný. Tvoje údaje neprodáváme.',
      ],
    },
    {
      id: 'retention',
      heading: 'Jak dlouho je uchováváme',
      body: [
        'Přihlašovací kódy a IP adresa uložená spolu s nimi se automaticky mažou deset minut po odeslání kódu.',
        'Tvůj účet a tvoje výsledky uchováváme, dokud účet nesmažeš. Můžeš to udělat přímo na obrazovce Účet, nebo napiš na {contact} a uděláme to za tebe. Smaže se účet, přihlašovací identity k němu připojené, tvoje statistiky, herní kruh, zařízení a tabulky skóre.',
        'Odehrané hry se s účtem nemažou, protože patří i do historie ostatních hráčů. Tvoje místo v nich se přejmenuje na „Player 1“, „Player 2“ a tak dál a už nevede k tobě.',
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
