// Draws the background of the Mac disk image's window, at one size per screen
// density, for scripts/macos-dmg.sh:
//
//   swift scripts/macos-dmg-background.swift <out-1x.png> <out-2x.png>
//
// An arrow from where the app's icon sits to where the Applications link sits
// (the positions in macos-dmg.sh), with no words: one image for every language.
import AppKit

let width = 600.0
let height = 400.0
// Finder measures from the top; these are the icons' centres, and the arrow
// stops short of the two 128-point icons.
let iconY = 170.0
let arrowFrom = 245.0
let arrowTo = 355.0

func draw(scale: CGFloat, to path: String) {
  let rep = NSBitmapImageRep(
    bitmapDataPlanes: nil, pixelsWide: Int(width * scale), pixelsHigh: Int(height * scale),
    bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false,
    colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0)!
  rep.size = NSSize(width: width, height: height)
  NSGraphicsContext.saveGraphicsState()
  NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: rep)

  let bounds = NSRect(x: 0, y: 0, width: width, height: height)
  NSGradient(
    starting: NSColor(srgbRed: 0.96, green: 0.96, blue: 0.95, alpha: 1),
    ending: NSColor(srgbRed: 0.86, green: 0.87, blue: 0.85, alpha: 1))!
    .draw(in: bounds, angle: -90)

  let y = height - iconY
  let arrow = NSBezierPath()
  arrow.lineWidth = 7
  arrow.lineCapStyle = .round
  arrow.lineJoinStyle = .round
  arrow.move(to: NSPoint(x: arrowFrom, y: y))
  arrow.line(to: NSPoint(x: arrowTo, y: y))
  arrow.move(to: NSPoint(x: arrowTo - 20, y: y + 20))
  arrow.line(to: NSPoint(x: arrowTo, y: y))
  arrow.line(to: NSPoint(x: arrowTo - 20, y: y - 20))
  NSColor(srgbRed: 0.45, green: 0.47, blue: 0.45, alpha: 1).setStroke()
  arrow.stroke()

  NSGraphicsContext.restoreGraphicsState()
  try! rep.representation(using: .png, properties: [:])!.write(to: URL(fileURLWithPath: path))
}

draw(scale: 1, to: CommandLine.arguments[1])
draw(scale: 2, to: CommandLine.arguments[2])
