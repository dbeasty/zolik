// swift-tools-version:5.9
//
// The Mac app: a native window around the web client, with the game core
// (server/mobile/zolikcore) linked in. Built by scripts/build-macos.sh, which
// produces build/Zolikcore.xcframework first and wraps the binary in an .app.
//
// Sources/Jokerless/Shared holds links to the iPhone module's Bonjour,
// Bluetooth, framing and thermal code, so both apps compile the same files.
import PackageDescription

let package = Package(
  name: "Jokerless",
  platforms: [.macOS(.v13)],
  products: [.executable(name: "Jokerless", targets: ["Jokerless"])],
  targets: [
    .binaryTarget(name: "Zolikcore", path: "build/Zolikcore.xcframework"),
    .executableTarget(
      name: "Jokerless",
      dependencies: ["Zolikcore"],
      path: "Sources/Jokerless",
      linkerSettings: [
        .linkedFramework("WebKit"),
        .linkedFramework("CoreBluetooth"),
        .linkedFramework("Security"),
        .linkedLibrary("resolv"),
      ]
    ),
  ]
)
