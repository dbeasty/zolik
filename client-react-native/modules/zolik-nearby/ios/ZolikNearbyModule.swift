import ExpoModulesCore
import Zolikcore

/// The embedded game server, as JS sees it.
///
/// This is a thin layer: Go owns the host's lifetime and keeps one per
/// process (see server/mobile/zolikcore), so a JS reload that calls
/// startHost again gets the host that is already running.
public class ZolikNearbyModule: Module {
  public func definition() -> ModuleDefinition {
    Name("ZolikNearby")

    AsyncFunction("startHost") { (lan: Bool) -> [String: Any] in
      var error: NSError?
      guard let host = ZolikcoreStart(try Self.dataDir(), lan, &error) else {
        throw HostException(error?.localizedDescription ?? "the host did not start")
      }
      return Self.describe(host)
    }

    AsyncFunction("stopHost") {
      ZolikcoreCurrent()?.stop()
    }

    Function("hostStatus") { () -> [String: Any]? in
      guard let host = ZolikcoreCurrent() else { return nil }
      return Self.describe(host)
    }
  }

  private static func describe(_ host: ZolikcoreHost) -> [String: Any] {
    return [
      "port": host.port(),
      "baseUrl": host.baseURL(),
      "instanceId": host.instanceID(),
      "lan": host.lan(),
    ]
  }

  /// Application Support rather than Documents: the tables are the app's
  /// own state, not files the player manages, and Documents is visible in
  /// the Files app when file sharing is on.
  private static func dataDir() throws -> String {
    let base = try FileManager.default.url(
      for: .applicationSupportDirectory, in: .userDomainMask, appropriateFor: nil, create: true)
    return base.appendingPathComponent("zolik-host", isDirectory: true).path
  }
}

internal final class HostException: GenericException<String> {
  override var reason: String { "Could not start the offline table: \(param)" }
}
