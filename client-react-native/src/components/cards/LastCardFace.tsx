import { useId } from 'react';

import type { Skin } from '@/src/skins/types';
import { Platform, StyleSheet, View } from 'react-native';
import Svg, { ClipPath, Defs, G, Path, Text as SvgText } from 'react-native-svg';

import {
  COLOURS,
  CREAM,
  DUSK,
  INKS,
  LAST_CARD_FONT,
  backdrop,
  duskStars,
  numeralCentre,
  parseLastCard,
  pinwheel,
  reverseGlyph,
  roundedRect,
  shapeBlock,
  shapePath,
  skipGlyph,
  type Colour,
  type LastCard,
  type Piece,
} from '@/src/components/cards/lastCardArt';
import {
  HEIRLOOM,
  hatching,
  heirloomBack,
  heirloomBackdrop,
  medallionRing,
  ornateFrame,
} from '@/src/components/cards/lastCardHeirloomArt';

/**
 * A Last Card face.
 *
 * One set of cards a skin, chosen by `CardView` from the skin's face style,
 * the way the French pack has a face a skin:
 *
 *  - `classic`, the condensed set (the classic skin): flat and quick to read —
 *    the colour's frame, its shape large with the number in it, an index in
 *    two corners, nothing behind.
 *  - `full`, the casino set: a panel carrying its colour's picture (a
 *    sunburst, the sea, a night sky, a honeycomb), a medallion in the colour's
 *    shape with the number or the action in it, an index on a cream tab.
 *  - `heirloom`, the heirloom set: engraved — ivory stock, a gilt double rule
 *    with corner scrolls, hatched shading, detailed pictures (a sun of
 *    straight and wavy rays over clouds, curling waves and gulls, a cratered
 *    moon over hills and pines, a honeycomb with honey and bees), ringed
 *    medallions and serif numerals.
 *  - `plain`, every set's small card — a phone's fanned hand, a compact
 *    board: the colour's frame and pale field, one big index down the left
 *    edge (all a closed hand shows of a card), and the shape in the corner.
 *
 * Drawn in a field 100 units wide and as tall as the card's own proportions
 * make it, so a circle stays round on every card size.
 */

/** On native the system face is already sans; a CSS stack means nothing there. */
const FONT = Platform.OS === 'web' ? LAST_CARD_FONT : undefined;

/** The heirloom set's type: an old-style serif, as an engraved card has. */
const SERIF = Platform.OS === 'web' ? "Georgia, 'Times New Roman', 'Iowan Old Style', serif" : 'Georgia';

/** Which set of cards a skin draws — see `CardView`. */
export type LastCardSet = 'classic' | 'casino' | 'heirloom';

/**
 * A skin's set of the pack, by the face style it already declares for the
 * French one: the classic skin's plain faces get the condensed set, the
 * heirloom skin's engraved deck the engraved set, and the rest the casino's.
 */
export function lastCardSetFor(skin: Skin): LastCardSet {
  switch (skin.card.face) {
    case 'plain':
      return 'classic';
    case 'vector':
      return 'heirloom';
  }
  return 'casino';
}

type Props = {
  card: string;
  width: number;
  height: number;
  variant: 'full' | 'plain' | 'classic' | 'heirloom';
  /** The plain index's size in pixels, from the layout metrics. */
  indexFont?: number;
  /** The colour a wild on the table was named, to draw on its face. */
  as?: string;
};

export function LastCardFace({ card, width, height, variant, indexFont, as }: Props) {
  const parsed = parseLastCard(card);
  const clip = useId().replace(/:/g, '');
  if (!parsed || width <= 0 || height <= 0) return null;
  const h = Math.round((height / width) * 1000) / 10;
  return (
    <View style={StyleSheet.absoluteFill} pointerEvents="none">
      <Svg width={width} height={height} viewBox={`0 0 100 ${h}`}>
        {variant === 'full' ? (
          <Full card={parsed} h={h} clipId={`lc${clip}`} named={namedColour(parsed, as)} />
        ) : variant === 'classic' ? (
          <Classic card={parsed} h={h} named={namedColour(parsed, as)} />
        ) : variant === 'heirloom' ? (
          <Heirloom card={parsed} h={h} clipId={`lc${clip}`} named={namedColour(parsed, as)} />
        ) : (
          <Plain card={parsed} h={h} font={indexFont ? (indexFont * 100) / width : 40} named={namedColour(parsed, as)} />
        )}
      </Svg>
    </View>
  );
}

/**
 * Heavy lettering: the outline is drawn first and wider, and the fill laid
 * over it, so the outline frames the figure instead of eating into it.
 */
function Lettering({
  x,
  y,
  size,
  fill,
  outline,
  underline,
  font = FONT,
  weight = '900',
  children,
}: {
  x: number;
  y: number;
  size: number;
  fill: string;
  outline?: string;
  underline?: boolean;
  font?: string;
  weight?: '700' | '900';
  children: string;
}) {
  const common = {
    x,
    y,
    fontSize: size,
    fontWeight: weight,
    fontFamily: font,
    textAnchor: 'middle' as const,
  };
  return (
    <>
      {outline ? (
        <SvgText {...common} fill={outline} stroke={outline} strokeWidth={size * 0.16} strokeLinejoin="round">
          {children}
        </SvgText>
      ) : null}
      <SvgText {...common} fill={fill}>
        {children}
      </SvgText>
      {underline ? (
        <Path
          d={`M${x - size * 0.2} ${y + size * 0.14}H${x + size * 0.2}`}
          stroke={outline ?? fill}
          strokeWidth={size * 0.08}
          strokeLinecap="round"
        />
      ) : null}
    </>
  );
}

function Pieces({ pieces }: { pieces: readonly Piece[] }) {
  return (
    <>
      {pieces.map((p, i) => (
        <Path
          key={i}
          d={p.d}
          fill={p.fill ?? 'none'}
          stroke={p.stroke}
          strokeWidth={p.sw}
          strokeLinecap="round"
          strokeLinejoin="round"
          opacity={p.opacity}
        />
      ))}
    </>
  );
}

/** The middle of a coloured card, or of a wild: number, glyph or emblem. */
function Emblem({ card, cx, cy, size, ink }: { card: LastCard; cx: number; cy: number; size: number; ink: string }) {
  switch (card.kind) {
    case 'skip':
      return <Pieces pieces={skipGlyph(cx, cy, size, ink)} />;
    case 'reverse':
      return <Pieces pieces={reverseGlyph(cx, cy, size, ink)} />;
    default:
      return null;
  }
}

/** The colour a wild was named, when it is one and was named one. */
function namedColour(card: LastCard, as?: string): Colour | undefined {
  return card.colour === null && as && (COLOURS as readonly string[]).includes(as) ? (as as Colour) : undefined;
}

function Full({ card, h, clipId, named }: { card: LastCard; h: number; clipId: string; named?: Colour }) {
  const cx = 50;
  const cy = h / 2;
  const panel = roundedRect(5, 5, 90, h - 10, 7);

  if (card.colour === null) {
    const four = card.kind === 'wildDrawFour';
    return (
      <>
        <Defs>
          <ClipPath id={clipId}>
            <Path d={panel} />
          </ClipPath>
        </Defs>
        <Path d={panel} fill={DUSK.field} />
        {named ? <Path d={panel} fill={INKS[named].main} opacity={0.28} /> : null}
        <G clipPath={`url(#${clipId})`}>
          <Pieces pieces={duskStars(h)} />
          <Path d={shapePath('C', cx, four ? cy - 8 : cy, 78)} fill={DUSK.glow} opacity={0.7} />
        </G>
        <Pieces pieces={pinwheel(cx, four ? cy - 8 : cy, four ? 50 : 64, named)} />
        {four ? (
          <Lettering x={cx} y={cy + 38} size={28} fill={CREAM} outline={DUSK.field}>
            +4
          </Lettering>
        ) : null}
        <Path d={panel} fill="none" stroke={named ? INKS[named].main : DUSK.glow} strokeWidth={named ? 4 : 2} />
        <WildTab card={card} />
        <G transform={`rotate(180 ${cx} ${cy})`}>
          <WildTab card={card} />
        </G>
      </>
    );
  }

  const ink = INKS[card.colour];
  const medal = 54;
  const ny = numeralCentre(card.colour, cy, medal);
  return (
    <>
      <Defs>
        <ClipPath id={clipId}>
          <Path d={panel} />
        </ClipPath>
      </Defs>
      <Path d={panel} fill={ink.pale} />
      <G clipPath={`url(#${clipId})`}>
        <Pieces pieces={backdrop(card.colour, h)} />
      </G>
      <Path d={panel} fill="none" stroke={ink.main} strokeWidth={2.2} />
      {/* A cream halo first, so the medallion stands clear of its picture. */}
      <Path d={shapePath(card.colour, cx, cy, medal + 10)} fill={CREAM} opacity={0.92} />
      <Path d={shapePath(card.colour, cx, cy, medal)} fill={ink.main} stroke={ink.deep} strokeWidth={2.4} />
      {card.kind === 'number' || card.kind === 'drawTwo' ? (
        <Lettering
          x={cx}
          y={ny + (card.kind === 'number' ? 12 : 9)}
          size={card.kind === 'number' ? 36 : 25}
          fill={CREAM}
          outline={ink.deep}
          underline={card.face === '6' || card.face === '9'}
        >
          {card.face}
        </Lettering>
      ) : (
        <Emblem card={card} cx={cx} cy={ny} size={30} ink={CREAM} />
      )}
      <ColourTab card={card} />
      <G transform={`rotate(180 ${cx} ${cy})`}>
        <ColourTab card={card} />
      </G>
    </>
  );
}

/** A coloured card's corner: its index and shape on a cream tab. */
function ColourTab({ card }: { card: LastCard }) {
  if (card.colour === null) return null;
  const ink = INKS[card.colour];
  const x = 9;
  const y = 9;
  const w = 19;
  const mid = x + w / 2;
  return (
    <>
      <Path d={roundedRect(x, y, w, 28, 4)} fill={CREAM} stroke={ink.main} strokeWidth={0.8} opacity={0.96} />
      {card.kind === 'number' || card.kind === 'drawTwo' ? (
        <SvgText
            fontFamily={FONT}
          x={mid}
          y={y + 15}
          fontSize={card.kind === 'number' ? 15 : 11}
          fontWeight="900"
          textAnchor="middle"
          fill={ink.deep}
        >
          {card.face}
        </SvgText>
      ) : (
        <Emblem card={card} cx={mid} cy={y + 10} size={14} ink={ink.deep} />
      )}
      <Path d={shapePath(card.colour, mid, y + 22, 7)} fill={ink.main} />
    </>
  );
}

/** A wild's corner: the four shapes, and the +4 where it has one. */
function WildTab({ card }: { card: LastCard }) {
  const x = 9;
  const y = 9;
  const w = 19;
  const mid = x + w / 2;
  const four = card.kind === 'wildDrawFour';
  return (
    <>
      <Path d={roundedRect(x, y, w, 28, 4)} fill={CREAM} opacity={0.96} />
      {four ? (
        <>
          <SvgText fontFamily={FONT} x={mid} y={y + 13} fontSize={11} fontWeight="900" textAnchor="middle" fill={DUSK.field}>
            +4
          </SvgText>
          <Pieces pieces={shapeBlock(mid, y + 21, 9)} />
        </>
      ) : (
        <Pieces pieces={shapeBlock(mid, y + 14, 14)} />
      )}
    </>
  );
}

/**
 * The classic set: condensed. The colour's frame, its shape large with the
 * number or action in it, an index in two corners — nothing behind, nothing
 * to read past.
 */
function Classic({ card, h, named }: { card: LastCard; h: number; named?: Colour }) {
  const cx = 50;
  const cy = h / 2;
  const frame = roundedRect(3, 3, 94, h - 6, 7);
  if (card.colour === null) {
    const four = card.kind === 'wildDrawFour';
    const label = four ? '+4' : 'W';
    return (
      <>
        <Path d={frame} fill={DUSK.field} stroke={named ? INKS[named].main : DUSK.glow} strokeWidth={named ? 6 : 3} />
        <Pieces pieces={pinwheel(cx, cy, 56, named)} />
        <ClassicIndex text={label} colour={CREAM} />
        <G transform={`rotate(180 ${cx} ${cy})`}>
          <ClassicIndex text={label} colour={CREAM} />
        </G>
      </>
    );
  }
  const ink = INKS[card.colour];
  const medal = 60;
  const ny = numeralCentre(card.colour, cy, medal);
  return (
    <>
      <Path d={frame} fill="#FFFFFF" stroke={ink.main} strokeWidth={5} />
      <Path d={shapePath(card.colour, cx, cy, medal)} fill={ink.main} />
      {card.kind === 'number' || card.kind === 'drawTwo' ? (
        <Lettering
          x={cx}
          y={ny + (card.kind === 'number' ? 13 : 9)}
          size={card.kind === 'number' ? 38 : 26}
          fill="#FFFFFF"
          underline={card.face === '6' || card.face === '9'}
        >
          {card.face}
        </Lettering>
      ) : (
        <Emblem card={card} cx={cx} cy={ny} size={32} ink="#FFFFFF" />
      )}
      <ClassicIndex card={card} />
      <G transform={`rotate(180 ${cx} ${cy})`}>
        <ClassicIndex card={card} />
      </G>
    </>
  );
}

/** A classic corner: the index printed straight on the card, and the shape. */
function ClassicIndex({ card, text, colour }: { card?: LastCard; text?: string; colour?: string }) {
  const x = 15;
  if (!card || card.colour === null) {
    return (
      <SvgText fontFamily={FONT} x={x} y={24} fontSize={15} fontWeight="900" textAnchor="middle" fill={colour}>
        {text}
      </SvgText>
    );
  }
  const ink = INKS[card.colour];
  return (
    <>
      {card.kind === 'number' || card.kind === 'drawTwo' ? (
        <SvgText fontFamily={FONT} x={x} y={23} fontSize={card.kind === 'number' ? 18 : 12} fontWeight="900" textAnchor="middle" fill={ink.deep}>
          {card.face}
        </SvgText>
      ) : (
        <Emblem card={card} cx={x} cy={17} size={15} ink={ink.deep} />
      )}
      <Path d={shapePath(card.colour, x, 31, 8)} fill={ink.main} />
    </>
  );
}

/**
 * The heirloom set: engraved. Ivory stock, a gilt double rule with corner
 * scrolls, the colour's picture in line and hatching, a ringed medallion
 * and serif numerals.
 */
function Heirloom({ card, h, clipId, named }: { card: LastCard; h: number; clipId: string; named?: Colour }) {
  const cx = 50;
  const cy = h / 2;
  const panel = roundedRect(9, 9, 82, h - 18, 3);
  if (card.colour === null) {
    const four = card.kind === 'wildDrawFour';
    return (
      <>
        <Path d={roundedRect(0, 0, 100, h, 6)} fill={HEIRLOOM.navy} />
        <Defs>
          <ClipPath id={clipId}>
            <Path d={panel} />
          </ClipPath>
        </Defs>
        <G clipPath={`url(#${clipId})`}>
          {named ? <Path d={panel} fill={INKS[named].main} opacity={0.3} /> : null}
          <Pieces pieces={[hatching(h, HEIRLOOM.navyLight, 2.6)]} />
          <Pieces pieces={duskStars(h)} />
        </G>
        <Pieces pieces={ornateFrame(h, named ? INKS[named].main : HEIRLOOM.giltLight)} />
        <Path d={shapePath('C', cx, four ? cy - 8 : cy, 62)} fill={HEIRLOOM.navy} stroke={HEIRLOOM.gilt} strokeWidth={1.2} />
        <Pieces pieces={pinwheel(cx, four ? cy - 8 : cy, four ? 46 : 56, named)} />
        {four ? (
          <Lettering x={cx} y={cy + 36} size={26} fill={HEIRLOOM.giltLight} outline={HEIRLOOM.navy} font={SERIF} weight="700">
            +4
          </Lettering>
        ) : null}
        <HeirloomIndex text={four ? '+4' : 'W'} ink={HEIRLOOM.giltLight} />
        <G transform={`rotate(180 ${cx} ${cy})`}>
          <HeirloomIndex text={four ? '+4' : 'W'} ink={HEIRLOOM.giltLight} />
        </G>
      </>
    );
  }
  const ink = INKS[card.colour];
  const medal = 46;
  const ny = numeralCentre(card.colour, cy, medal);
  return (
    <>
      <Path d={roundedRect(0, 0, 100, h, 6)} fill={HEIRLOOM.stock} />
      <Defs>
        <ClipPath id={clipId}>
          <Path d={panel} />
        </ClipPath>
      </Defs>
      <Path d={panel} fill={ink.pale} />
      <G clipPath={`url(#${clipId})`}>
        <Pieces pieces={[hatching(h, ink.tint)]} />
        <Pieces pieces={heirloomBackdrop(card.colour, h)} />
      </G>
      <Pieces pieces={ornateFrame(h, ink.deep)} />
      <Path d={shapePath(card.colour, cx, cy, medal + 16)} fill={HEIRLOOM.stock} opacity={0.94} />
      <Pieces pieces={medallionRing(card.colour, cx, cy, medal)} />
      <Path d={shapePath(card.colour, cx, cy, medal)} fill={ink.main} stroke={HEIRLOOM.gilt} strokeWidth={1.8} />
      {card.kind === 'number' || card.kind === 'drawTwo' ? (
        <Lettering
          x={cx}
          y={ny + (card.kind === 'number' ? 11 : 8)}
          size={card.kind === 'number' ? 32 : 22}
          fill={HEIRLOOM.stock}
          outline={ink.deep}
          font={SERIF}
          weight="700"
          underline={card.face === '6' || card.face === '9'}
        >
          {card.face}
        </Lettering>
      ) : (
        <Emblem card={card} cx={cx} cy={ny} size={26} ink={HEIRLOOM.stock} />
      )}
      <HeirloomIndex card={card} ink={ink.deep} />
      <G transform={`rotate(180 ${cx} ${cy})`}>
        <HeirloomIndex card={card} ink={ink.deep} />
      </G>
    </>
  );
}

/** An heirloom corner: a serif index and the shape, on a small cartouche. */
function HeirloomIndex({ card, text, ink }: { card?: LastCard; text?: string; ink: string }) {
  const x = 17;
  const plate = roundedRect(10.5, 11, 13, 25, 3);
  if (!card || card.colour === null) {
    return (
      <SvgText fontFamily={SERIF} x={x} y={25} fontSize={13} fontWeight="700" textAnchor="middle" fill={ink}>
        {text}
      </SvgText>
    );
  }
  return (
    <>
      <Path d={plate} fill={HEIRLOOM.stock} stroke={HEIRLOOM.gilt} strokeWidth={0.7} />
      {card.kind === 'number' || card.kind === 'drawTwo' ? (
        <SvgText fontFamily={SERIF} x={x} y={23} fontSize={card.kind === 'number' ? 14 : 9} fontWeight="700" textAnchor="middle" fill={ink}>
          {card.face}
        </SvgText>
      ) : (
        <Emblem card={card} cx={x} cy={18.5} size={11} ink={ink} />
      )}
      <Path d={shapePath(card.colour, x, 30, 6)} fill={INKS[card.colour].main} />
    </>
  );
}

/**
 * The plain face: frame, field, and one index sized for the strip a closed
 * hand shows. The index is left-aligned at the card's edge, because that
 * strip is all of the card a fanned hand lets you read.
 */
function Plain({ card, h, font, named }: { card: LastCard; h: number; font: number; named?: Colour }) {
  const frame = roundedRect(2.5, 2.5, 95, h - 5, 6);
  const left = 7;
  const mid = left + font * 0.42;
  if (card.colour === null) {
    const four = card.kind === 'wildDrawFour';
    return (
      <>
        <Path d={frame} fill={DUSK.field} stroke={named ? INKS[named].main : DUSK.glow} strokeWidth={named ? 6 : 3} />
        {named ? <Path d={frame} fill={INKS[named].main} opacity={0.28} /> : null}
        <Pieces pieces={duskStars(h)} />
        <Pieces pieces={pinwheel(64, h - 32, 46, named)} />
        {four ? (
          <>
            <SvgText fontFamily={FONT} x={left} y={font * 0.95 + 4} fontSize={font * 0.8} fontWeight="900" fill={CREAM}>
              +4
            </SvgText>
            <Pieces pieces={shapeBlock(mid, font * 0.95 + 4 + font * 0.55, font * 0.6)} />
          </>
        ) : (
          <Pieces pieces={shapeBlock(mid, font * 0.62 + 4, font * 0.95)} />
        )}
      </>
    );
  }
  const ink = INKS[card.colour];
  const base = font * 0.95 + 4;
  return (
    <>
      <Path d={frame} fill={ink.pale} stroke={ink.main} strokeWidth={4} />
      <Path d={shapePath(card.colour, 66, h - 34, 44)} fill={ink.tint} opacity={0.75} />
      <Path d={shapePath(card.colour, 66, h - 34, 26)} fill={ink.main} opacity={0.55} />
      {card.kind === 'number' || card.kind === 'drawTwo' ? (
        <SvgText
            fontFamily={FONT}
          x={left}
          y={base}
          fontSize={card.kind === 'number' ? font : font * 0.8}
          fontWeight="900"
          fill={ink.deep}
        >
          {card.face}
        </SvgText>
      ) : (
        <Emblem card={card} cx={mid} cy={font * 0.55 + 4} size={font * 0.95} ink={ink.deep} />
      )}
      {card.face === '6' || card.face === '9' ? (
        <Path
          d={`M${left + font * 0.06} ${base + font * 0.12}H${left + font * 0.5}`}
          stroke={ink.deep}
          strokeWidth={font * 0.07}
          strokeLinecap="round"
        />
      ) : null}
      <Path d={shapePath(card.colour, mid, base + font * 0.45, font * 0.5)} fill={ink.main} />
    </>
  );
}

/**
 * The back of a Last Card, in the skin's set: classic is dusk and the four
 * shapes; casino adds the shapes in a quiet repeat and the colour wheel;
 * heirloom is navy and gilt filigree, a rosette and a ribboned wordmark.
 */
export function LastCardBack({ width, height, set = 'casino' }: { width: number; height: number; set?: LastCardSet }) {
  const clip = useId().replace(/:/g, '');
  if (width <= 0 || height <= 0) return null;
  const h = Math.round((height / width) * 1000) / 10;
  if (set === 'heirloom') {
    const { pieces, ribbonY } = heirloomBack(h);
    return (
      <View style={[StyleSheet.absoluteFill, styles.back]} pointerEvents="none">
        <Svg width={width} height={height} viewBox={`0 0 100 ${h}`}>
          <Pieces pieces={pieces} />
          <SvgText
            fontFamily={SERIF}
            x={50}
            y={ribbonY + 2.6}
            fontSize={7.5}
            fontWeight="700"
            textAnchor="middle"
            fill={HEIRLOOM.giltLight}
            letterSpacing={0.6}
          >
            Last Card
          </SvgText>
        </Svg>
      </View>
    );
  }
  if (set === 'classic') {
    return (
      <View style={[StyleSheet.absoluteFill, styles.back]} pointerEvents="none">
        <Svg width={width} height={height} viewBox={`0 0 100 ${h}`}>
          <Path d={roundedRect(0, 0, 100, h, 8)} fill={DUSK.field} />
          <Path d={roundedRect(5, 5, 90, h - 10, 5)} fill="none" stroke={DUSK.glow} strokeWidth={2} />
          <Pieces pieces={shapeBlock(50, h / 2, 40)} />
        </Svg>
      </View>
    );
  }
  const panel = roundedRect(4, 4, 92, h - 8, 6);
  const repeat: Piece[] = [];
  const order = ['C', 'T', 'V', 'A'] as const;
  let k = 0;
  for (let y = 10; y < h; y += 16) {
    for (let x = (y / 16) % 2 < 1 ? 10 : 18; x < 100; x += 16) {
      const c = order[k++ % 4];
      repeat.push({ d: shapePath(c, x, y, 7), fill: INKS[c].main, opacity: 0.32 });
    }
  }
  return (
    <View style={[StyleSheet.absoluteFill, styles.back]} pointerEvents="none">
      <Svg width={width} height={height} viewBox={`0 0 100 ${h}`}>
        <Defs>
          <ClipPath id={`lcb${clip}`}>
            <Path d={panel} />
          </ClipPath>
        </Defs>
        <Path d={roundedRect(0, 0, 100, h, 8)} fill={DUSK.field} />
        <G clipPath={`url(#lcb${clip})`}>
          <Pieces pieces={repeat} />
        </G>
        <Path d={panel} fill="none" stroke={DUSK.glow} strokeWidth={2} />
        <Path d={shapePath('C', 50, h / 2, 50)} fill={DUSK.field} stroke={DUSK.glow} strokeWidth={1.5} />
        <Pieces pieces={pinwheel(50, h / 2, 40)} />
      </Svg>
    </View>
  );
}

const styles = StyleSheet.create({
  back: { borderRadius: 6, overflow: 'hidden' },
});
