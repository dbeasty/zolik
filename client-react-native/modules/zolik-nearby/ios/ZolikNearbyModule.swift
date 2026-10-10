import ExpoModulesCore
import Zolikcore

/// The embedded game server, and the local network around it, as JS sees it.
///
/// Go owns the host's lifetime and keeps one per process (see
/// server/mobile/zolikcore), so a JS reload that calls startHost again gets
/// the host that is already running. This side owns what Go cannot do on a
/// phone: advertising the table over Bonjour, finding other tables, and
/// reading the phone's own addresses.
public class ZolikNearbyModule: Module {
  private let bonjour = NearbyBonjour()
  private let bleHost = NearbyBleHost()
  private let bleGuest = NearbyBleGuest()
  private let thermal = NearbyThermal()

  public func definition() -> ModuleDefinition {
    Name("ZolikNearby")

    Events("onHostFound", "onHostLost", "onBleFound", "onBleMessage", "onBleClosed", "onBleGuests")

    OnCreate {
      self.bonjour.onFound = { [weak self] host in self?.sendEvent("onHostFound", host) }
      self.bonjour.onLost = { [weak self] name in self?.sendEvent("onHostLost", ["name": name]) }
      self.bleGuest.onFound = { [weak self] host in self?.sendEvent("onBleFound", host) }
      self.bleGuest.onMessage = { [weak self] linkId, data in
        self?.sendEvent("onBleMessage", ["linkId": linkId, "data": data.base64EncodedString()])
      }
      self.bleGuest.onClosed = { [weak self] linkId in self?.sendEvent("onBleClosed", ["linkId": linkId]) }
      self.bleHost.onGuestsChanged = { [weak self] n in self?.sendEvent("onBleGuests", ["count": n]) }
    }

    // Bluetooth, host side: advertise the table and serve guests through
    // the Go tunnel. The Go host must already be running.
    AsyncFunction("bleHostStart") { (name: String) in
      guard ZolikcoreCurrent() != nil else { throw HostException("no host is running") }
      self.bleHost.start(name: name)
    }

    AsyncFunction("bleHostStop") {
      self.bleHost.stop()
    }

    // Bluetooth, guest side.
    AsyncFunction("bleScanStart") {
      self.bleGuest.scan()
    }

    AsyncFunction("bleScanStop") {
      self.bleGuest.stopScan()
    }

    AsyncFunction("bleConnect") { (peripheralId: String, promise: Promise) in
      self.bleGuest.connect(peripheralId: peripheralId) { result in
        switch result {
        case .success(let (linkId, info)):
          promise.resolve(["linkId": linkId, "info": String(data: info, encoding: .utf8) ?? ""])
        case .failure(let e):
          promise.reject("BLE_CONNECT", e.localizedDescription)
        }
      }
    }

    AsyncFunction("bleSend") { (linkId: String, base64: String, promise: Promise) in
      guard let data = Data(base64Encoded: base64) else {
        promise.reject("BLE_SEND", "not base64")
        return
      }
      self.bleGuest.send(linkId: linkId, msg: data) { error in
        if let error { promise.reject("BLE_SEND", error.localizedDescription) } else { promise.resolve(nil) }
      }
    }

    AsyncFunction("bleDisconnect") { (linkId: String) in
      self.bleGuest.disconnect(linkId: linkId)
    }

    Function("bleGuestCodes") { () -> [String] in
      self.bleHost.codes()
    }

    // Randomness for the tunnel's ephemeral keys, from the system's secure
    // generator. JS has no crypto.getRandomValues under Hermes.
    Function("randomBytes") { (count: Int) -> String in
      var bytes = [UInt8](repeating: 0, count: max(0, min(count, 1024)))
      guard SecRandomCopyBytes(kSecRandomDefault, bytes.count, &bytes) == errSecSuccess else {
        throw HostException("the system random generator failed")
      }
      return Data(bytes).base64EncodedString()
    }

    AsyncFunction("bleReady") { (promise: Promise) in
      self.bleGuest.ready { promise.resolve($0) }
    }

    Function("bleState") { () -> String in
      self.bleGuest.state()
    }

    AsyncFunction("startHost") { () -> [String: Any] in
      var error: NSError?
      guard let host = ZolikcoreStart(try Self.dataDir(), &error) else {
        throw HostException(error?.localizedDescription ?? "the host did not start")
      }
      self.thermal.start()
      return Self.describe(host)
    }

    // The same host, for a phone that has been enrolled and has somebody
    // signed in: it additionally replicates that account's data and serves it
    // back, so the app can show a person their own things with no connection.
    AsyncFunction("startNode") { (credential: String, userHex: String, cloudBaseUrl: String) -> [String: Any] in
      var error: NSError?
      guard let host = ZolikcoreStartNode(try Self.dataDir(), credential, userHex, cloudBaseUrl, &error) else {
        throw HostException(error?.localizedDescription ?? "the host did not start")
      }
      self.thermal.start()
      return Self.describe(host)
    }

    // This install's node identity: what the cloud enrols, and the key half
    // it is enrolled by. The private half never leaves the phone.
    Function("nodeIdentity") { () -> [String: Any]? in
      guard let host = ZolikcoreCurrent() else { return nil }
      return ["nodeId": host.nodeID(), "publicKey": host.nodePublicKey()]
    }

    // Replicate now rather than at the next tick, for the moments where
    // waiting would be visible: coming back to the app, the network
    // returning, a match ending.
    AsyncFunction("syncNow") {
      guard let host = ZolikcoreCurrent() else { return }
      try host.syncNow()
    }

    // Whether this device is holding the signed-in account's data yet, so the
    // app can tell "nothing synced" from "you have never played".
    Function("replicaReady") { () -> Bool in
      ZolikcoreCurrent()?.replicaReady() ?? false
    }

    // Starts following a match that was begun somewhere else, so this device
    // can open it.
    AsyncFunction("followMatch") { (matchID: String) in
      guard let host = ZolikcoreCurrent() else { throw HostException("no host is running") }
      try host.followMatch(matchID)
    }

    AsyncFunction("stopHost") {
      self.bonjour.unpublish()
      self.bleHost.stop()
      ZolikcoreCurrent()?.stop()
    }

    Function("hostStatus") { () -> [String: Any]? in
      guard let host = ZolikcoreCurrent() else { return nil }
      return Self.describe(host)
    }

    // Lets the room in: the host listens on the network and says so over
    // Bonjour. Main queue, because NetService schedules on the run loop of
    // the thread that publishes it.
    AsyncFunction("openRoom") { (name: String) -> [String: Any] in
      guard let host = ZolikcoreCurrent() else { throw HostException("no host is running") }
      var port = 0
      try host.openLAN(&port)
      self.bonjour.publish(
        name: name, port: Int32(port),
        txt: ["v": String(ZolikcoreProtocolVersion), "id": host.instanceID(), "n": name])
      return ["port": port, "addresses": NearbyAddresses.ipv4()]
    }.runOnQueue(.main)

    AsyncFunction("closeRoom") {
      self.bonjour.unpublish()
      ZolikcoreCurrent()?.closeLAN()
    }.runOnQueue(.main)

    // The internet door: guests anywhere, through the cloud's relay. The
    // phone stays the server (zolikcore/relay.go).
    AsyncFunction("openRelay") { (name: String) -> [String: Any] in
      guard let host = ZolikcoreCurrent() else { throw HostException("no host is running") }
      _ = try host.openRelay(name)
      return Self.relay(host)
    }

    AsyncFunction("closeRelay") {
      ZolikcoreCurrent()?.closeRelay()
    }

    Function("relayStatus") { () -> [String: Any] in
      guard let host = ZolikcoreCurrent() else {
        return ["status": "off", "code": "", "url": "", "guests": 0]
      }
      return Self.relay(host)
    }

    Function("hostResumed") {
      ZolikcoreCurrent()?.resumed()
    }

    AsyncFunction("startBrowsing") {
      self.bonjour.browse()
    }.runOnQueue(.main)

    AsyncFunction("stopBrowsing") {
      self.bonjour.stopBrowsing()
    }.runOnQueue(.main)

    Function("localAddresses") { () -> [String] in
      NearbyAddresses.ipv4()
    }
  }

  private static func relay(_ host: ZolikcoreHost) -> [String: Any] {
    return [
      "status": host.relayStatus(),
      "code": host.relayCode(),
      "url": host.relayURL(),
      "guests": host.relayGuests(),
    ]
  }

  private static func describe(_ host: ZolikcoreHost) -> [String: Any] {
    return [
      "port": host.port(),
      "baseUrl": host.baseURL(),
      "instanceId": host.instanceID(),
      "lanPort": host.lanPort(),
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
