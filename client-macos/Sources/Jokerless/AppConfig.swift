import Foundation
import Zolikcore

/// Where the app plays online, and the few switches a development build has.
enum AppConfig {
  /// The server for online play. A release always uses the one in Info.plist;
  /// a debug build can be pointed at a local server for testing.
  static var serverURL: String {
    #if DEBUG
      if let env = ProcessInfo.processInfo.environment["ZOLIK_BASE_URL"], !env.isEmpty {
        return env.trimmingCharacters(in: CharacterSet(charactersIn: "/"))
      }
    #endif
    return (Bundle.main.object(forInfoDictionaryKey: "JokerlessServer") as? String) ?? "https://jokerless.com"
  }

  /// The folder the end-to-end runner talks to the app through, in a debug
  /// build started with ZOLIK_E2E_CONTROL. Always nil in a release.
  static var e2eControlDir: URL? {
    #if DEBUG
      if let dir = ProcessInfo.processInfo.environment["ZOLIK_E2E_CONTROL"], !dir.isEmpty {
        return URL(fileURLWithPath: dir, isDirectory: true)
      }
    #endif
    return nil
  }

  static var isE2E: Bool { e2eControlDir != nil }

  /// Where the app keeps its own state: the offline tables' database and the
  /// web view's storage. Overridable in a debug build so a test run starts
  /// from nothing and leaves the player's own data alone.
  static var dataDir: URL {
    #if DEBUG
      if let dir = ProcessInfo.processInfo.environment["ZOLIK_DATA_DIR"], !dir.isEmpty {
        return URL(fileURLWithPath: dir, isDirectory: true)
      }
    #endif
    let base = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0]
    return base.appendingPathComponent("Jokerless", isDirectory: true)
  }

  static var version: String {
    (Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String) ?? "0.0.0-dev"
  }

  /// Tells the core where the web client and the cloud are, before anything
  /// asks it for either.
  static func configureCore() {
    if let web = Bundle.main.resourceURL?.appendingPathComponent("web", isDirectory: true) {
      ZolikcoreSetWebRoot(web.path)
    }
    var error: NSError?
    if !ZolikcoreSetCloudBaseURL(serverURL, &error) {
      NSLog("Jokerless: the server address %@ was refused: %@", serverURL, error?.localizedDescription ?? "")
    }
  }
}
