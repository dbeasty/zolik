import Foundation
import Zolikcore

/// The zolik-nearby module for the Mac: the same calls the iPhone app's
/// ZolikNearbyModule answers, built on the same Bonjour, Bluetooth and
/// thermal code (Sources/Jokerless/Shared) and the same core.
///
/// Calls the page makes synchronously on the phone (hostStatus, bleState and
/// the like) cannot be synchronous across the web view's bridge, so the page
/// keeps a copy of their answers. snapshot() is that copy; it rides along
/// with every reply and is pushed as a "state" event when something changes
/// on its own.
final class NearbyService {
  var emit: ((String, Any) -> Void)?

  private let bonjour = NearbyBonjour()
  private let bleHost = NearbyBleHost()
  private let bleGuest = NearbyBleGuest()
  private let thermal = NearbyThermal()
  private var ticker: Timer?

  init() {
    bonjour.onFound = { [weak self] host in self?.event("onHostFound", host) }
    bonjour.onLost = { [weak self] name in self?.event("onHostLost", ["name": name]) }
    bleGuest.onFound = { [weak self] host in self?.event("onBleFound", host) }
    bleGuest.onMessage = { [weak self] linkId, data in
      self?.event("onBleMessage", ["linkId": linkId, "data": data.base64EncodedString()])
    }
    bleGuest.onClosed = { [weak self] linkId in self?.event("onBleClosed", ["linkId": linkId]) }
    bleHost.onGuestsChanged = { [weak self] n in
      self?.event("onBleGuests", ["count": n])
      self?.pushState()
    }
  }

  struct Failure: LocalizedError {
    let errorDescription: String?
    init(_ message: String) { errorDescription = message }
  }

  /// Answers one call from the page, on the main thread.
  func call(_ method: String, _ args: [Any], done: @escaping (Result<Any?, Error>) -> Void) {
    func string(_ i: Int) -> String { args.count > i ? (args[i] as? String ?? "") : "" }
    func finish(_ value: Any?) { DispatchQueue.main.async { done(.success(value)) } }
    func fail(_ error: Error) { DispatchQueue.main.async { done(.failure(error)) } }
    func host() throws -> ZolikcoreHost {
      guard let h = ZolikcoreCurrent() else { throw Failure("no host is running") }
      return h
    }

    do {
      switch method {
      case "startHost":
        var error: NSError?
        guard let h = ZolikcoreStart(try Self.dataDir(), &error) else {
          throw Failure(error?.localizedDescription ?? "the offline table did not start")
        }
        thermal.start()
        startTicking()
        finish(Self.describe(h))
      case "startNode":
        var error: NSError?
        guard let h = ZolikcoreStartNode(try Self.dataDir(), string(0), string(1), string(2), &error) else {
          throw Failure(error?.localizedDescription ?? "the offline table did not start")
        }
        thermal.start()
        startTicking()
        finish(Self.describe(h))
      case "stopHost":
        shutdown()
        finish(nil)
      case "syncNow":
        // Replication can take a while; it must not hold up the window.
        DispatchQueue.global(qos: .userInitiated).async {
          do {
            try ZolikcoreCurrent()?.syncNow()
            finish(nil)
          } catch { fail(error) }
        }
      case "followMatch":
        let h = try host()
        try h.followMatch(string(0))
        finish(nil)
      case "openRoom":
        let h = try host()
        var port = 0
        try h.openLAN(&port)
        let name = string(0)
        bonjour.publish(
          name: name, port: Int32(port),
          txt: ["v": String(ZolikcoreProtocolVersion), "id": h.instanceID(), "n": name])
        finish(["port": port, "addresses": NearbyAddresses.ipv4()])
      case "closeRoom":
        bonjour.unpublish()
        ZolikcoreCurrent()?.closeLAN()
        finish(nil)
      case "openRelay":
        let h = try host()
        var error: NSError?
        _ = h.openRelay(string(0), error: &error)
        if let error { throw error }
        finish(Self.relay(h))
      case "closeRelay":
        ZolikcoreCurrent()?.closeRelay()
        finish(nil)
      case "hostResumed":
        ZolikcoreCurrent()?.resumed()
        finish(nil)
      case "startBrowsing":
        bonjour.browse()
        finish(nil)
      case "stopBrowsing":
        bonjour.stopBrowsing()
        finish(nil)
      case "bleHostStart":
        _ = try host()
        bleHost.start(name: string(0))
        finish(nil)
      case "bleHostStop":
        bleHost.stop()
        finish(nil)
      case "bleScanStart":
        bleGuest.scan()
        finish(nil)
      case "bleScanStop":
        bleGuest.stopScan()
        finish(nil)
      case "bleConnect":
        bleGuest.connect(peripheralId: string(0)) { result in
          switch result {
          case .success(let (linkId, info)):
            finish(["linkId": linkId, "info": String(data: info, encoding: .utf8) ?? ""])
          case .failure(let e): fail(e)
          }
        }
      case "bleSend":
        guard let data = Data(base64Encoded: string(1)) else { throw Failure("not base64") }
        bleGuest.send(linkId: string(0), msg: data) { error in
          if let error { fail(error) } else { finish(nil) }
        }
      case "bleDisconnect":
        bleGuest.disconnect(linkId: string(0))
        finish(nil)
      case "bleReady":
        bleGuest.ready { state in finish(state) }
      default:
        throw Failure("unknown call \(method)")
      }
    } catch {
      fail(error)
    }
  }

  /// What the page reads synchronously: hostStatus(), relayStatus(),
  /// localAddresses(), bleState(), bleGuestCodes(), nodeIdentity(),
  /// replicaReady().
  func snapshot() -> [String: Any] {
    let h = ZolikcoreCurrent()
    var out: [String: Any] = [
      "host": h.map { Self.describe($0) } ?? NSNull(),
      "relay": h.map { Self.relay($0) } ?? ["status": "off", "code": "", "url": "", "guests": 0],
      "addresses": NearbyAddresses.ipv4(),
      "bleState": bleGuest.state(),
      "bleCodes": bleHost.codes(),
      "replicaReady": h?.replicaReady() ?? false,
    ]
    if let h { out["identity"] = ["nodeId": h.nodeID(), "publicKey": h.nodePublicKey()] }
    return out
  }

  /// Ends the table properly: the room is told it closed, and the database
  /// is shut cleanly.
  func shutdown() {
    ticker?.invalidate()
    ticker = nil
    bonjour.unpublish()
    bleHost.stop()
    thermal.stop()
    ZolikcoreCurrent()?.stop()
    pushState()
  }

  private func event(_ name: String, _ payload: Any) {
    DispatchQueue.main.async { self.emit?("nearby", ["event": name, "payload": payload]) }
  }

  private func pushState() {
    DispatchQueue.main.async { self.emit?("state", self.snapshot()) }
  }

  /// While a table is up, the relay's guest count and the like change with
  /// nobody calling; the page's copy is refreshed every couple of seconds.
  private func startTicking() {
    guard ticker == nil else { return }
    ticker = Timer.scheduledTimer(withTimeInterval: 2, repeats: true) { [weak self] _ in self?.pushState() }
  }

  private static func relay(_ host: ZolikcoreHost) -> [String: Any] {
    ["status": host.relayStatus(), "code": host.relayCode(), "url": host.relayURL(), "guests": host.relayGuests()]
  }

  private static func describe(_ host: ZolikcoreHost) -> [String: Any] {
    ["port": host.port(), "baseUrl": host.baseURL(), "instanceId": host.instanceID(), "lanPort": host.lanPort()]
  }

  private static func dataDir() throws -> String {
    let dir = AppConfig.dataDir.appendingPathComponent("zolik-host", isDirectory: true)
    try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
    return dir.path
  }
}
