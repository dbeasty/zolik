import type { LegalDocument } from './types';

/**
 * Terms of Use, Czech.
 *
 * Section ids, order, and `{placeholders}` match `terms.en.ts` exactly —
 * asserted by `legal.test.ts`, because a legal document that is half
 * translated is the one kind of half-translated screen a fallback cannot make
 * acceptable: an English clause served to a Czech reader is a clause they were
 * never shown.
 */
export const termsCs: LegalDocument = {
  id: 'terms',
  title: 'Podmínky použití',
  version: '2026-09-01',
  sections: [
    {
      id: 'who',
      heading: 'Kdo Žolíky provozuje',
      body: [
        'Žolíky jsou bezplatná online karetní hra, kterou provozuje {operator} („my“). Hraním tyto podmínky přijímáš. Pokud je nepřijímáš, nehraj.',
      ],
    },
    {
      id: 'as-is',
      heading: 'Hra je poskytována, jak stojí a leží',
      body: [
        'Neposkytujeme žádnou záruku, že Žolíky budou dostupné, bez výpadků nebo bez chyb, ani že jsou pravidla naprogramována bez omylů.',
        'Kteroukoli část hry můžeme kdykoli a bez předchozího upozornění změnit, pozastavit nebo ukončit a můžeme odstranit neaktivní účty.',
      ],
    },
    {
      id: 'no-money',
      heading: 'Žádné skutečné peníze',
      body: [
        'V Žolíkách se nesází, nic se nekupuje a nejsou v nich žádné výhry. Nejde o hazardní hru.',
        'Žetony, body a pořadí jsou virtuální. Nemají žádnou peněžní hodnotu a nelze je směnit za peníze ani za nic jiného.',
      ],
    },
    {
      id: 'account',
      heading: 'Za svůj účet si odpovídáš sám',
      body: [
        'Odpovídáš za to, co se pod tvým účtem děje, i za jméno a obrázek, které si zvolíš.',
        'Nevol jméno, které je urážlivé, vydává se za někoho jiného nebo zasahuje do práv jiných. Takové jméno můžeme změnit nebo odstranit a účet, který je nese, můžeme zrušit.',
      ],
    },
    {
      id: 'others',
      heading: 'Ostatní hráči',
      body: [
        'Ostatní hráče neřídíme. Neodpovídáme za to, jak se chovají, jaká jména si volí, ani za to, co ti způsobí. S pozvánkami ke stolu a s kódy pro připojení nakládej obezřetně.',
      ],
    },
    {
      id: 'results',
      heading: 'Výsledky a historie se mohou ztratit',
      body: [
        'Výsledky zápasů, statistiky a pořadí slouží pro zábavu. Mohou být vynulovány, opraveny nebo ztraceny; z těchto podmínek neplyne nárok na jejich uchování.',
      ],
    },
    {
      id: 'liability',
      heading: 'Odpovědnost za škodu',
      body: [
        'V nejširším rozsahu, který právo připouští, neodpovídáme za žádnou újmu vzniklou používáním Žolíků, včetně ztráty dat, ztráty postupu, ztráty času nebo nedostupnosti hry.',
        'Nic v těchto podmínkách neomezuje odpovědnost, kterou omezit nelze — zejména za újmu na životě a na zdraví, za podvod a za úmysl a hrubou nedbalost. Jsi-li spotřebitel, tvá zákonná práva tím nejsou dotčena.',
      ],
    },
    {
      id: 'source',
      heading: 'Žolíky jsou svobodný software',
      body: [
        'Žolíky jsou licencovány pod GNU Affero General Public License, verze 3. Můžeš je volně používat, zkoumat, šířit i měnit za podmínek, které tato licence stanoví — včetně výše uvedeného vyloučení záruk, které je stejně tak jejím zněním jako naším.',
        'Úplný zdrojový kód verze, která tu běží, najdeš na {source}. Tento odstavec existuje kvůli článku 13 té licence: protože se k Žolíkům dostáváš po síti, a ne tak, že by sis je instaloval, musí ti být zdrojový kód nabídnut přímo tady na obrazovce, a ne jen tomu, kdo si stáhne kopii.',
        'Pokud provozuješ upravené Žolíky a necháš na nich hrát další lidi po síti, tatáž licence po tobě žádá, abys jim svůj zdrojový kód nabídl stejným způsobem.',
      ],
    },
    {
      id: 'law',
      heading: 'Rozhodné právo a kontakt',
      body: [
        'Tyto podmínky se řídí právem státu {country}. Jsi-li spotřebitel, zůstává ti ochrana podle kogentních předpisů země, ve které bydlíš.',
        'Dotazy k podmínkám: {contact}.',
      ],
    },
  ],
};
