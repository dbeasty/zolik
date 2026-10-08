import { useId } from 'react';
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

/**
 * A Last Card face.
 *
 * Two of them, chosen by `CardView` the way it chooses between a skin's faces:
 *
 *  - `full`, for the skins that print a whole card: a panel carrying its
 *    colour's picture (a sunburst, the sea, a night sky, a honeycomb), a
 *    medallion in the colour's own shape with the number or the action in
 *    it, and an index on a cream tab in two corners.
 *  - `plain`, for the classic skin and a phone's fanned hand: the colour's
 *    frame and pale field, one big index down the left edge — which is all a
 *    closed hand shows of a card — and the shape in the corner.
 *
 * Drawn in a field 100 units wide and as tall as the card's own proportions
 * make it, so a circle stays round on every card size.
 */

/** On native the system face is already sans; a CSS stack means nothing there. */
const FONT = Platform.OS === 'web' ? LAST_CARD_FONT : undefined;

type Props = {
  card: string;
  width: number;
  height: number;
  variant: 'full' | 'plain';
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
  children,
}: {
  x: number;
  y: number;
  size: number;
  fill: string;
  outline?: string;
  underline?: boolean;
  children: string;
}) {
  const common = {
    x,
    y,
    fontSize: size,
    fontWeight: '900' as const,
    fontFamily: FONT,
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

/** The back of a Last Card: dusk, the four shapes in a quiet repeat, and the pinwheel. */
export function LastCardBack({ width, height }: { width: number; height: number }) {
  const clip = useId().replace(/:/g, '');
  if (width <= 0 || height <= 0) return null;
  const h = Math.round((height / width) * 1000) / 10;
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
