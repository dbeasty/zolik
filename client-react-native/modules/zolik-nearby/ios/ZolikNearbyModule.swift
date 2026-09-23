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
      return Self.describe(host)
    }

    // The same host, for a phone that has been enrolled and has somebody
    // signed in: it additionally replicates that account's data and serves it
    // back, so the app can show a person their own things with no connection.
    AsyncFunction("startNode") { (credential: String, userHex: String) -> [String: Any] in
      var error: NSError?
      guard let host = ZolikcoreStartNode(try Self.dataDir(), credential, userHex, &error) else {
        throw HostException(error?.localizedDescription ?? "the host did not start")
      }
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

/// Bonjour for `_zolik._tcp`: publishing this phone's table and finding the
/// others. NetService rather than the Network framework because the listener
/// belongs to Go: NWListener can only advertise a port it owns itself.
final class NearbyBonjour: NSObject, NetServiceDelegate, NetServiceBrowserDelegate {
  static let type = "_zolik._tcp."

  var onFound: (([String: Any]) -> Void)?
  var onLost: ((String) -> Void)?

  private var published: NetService?
  private var browser: NetServiceBrowser?
  /// Found but not yet resolved, kept alive until they answer.
  private var resolving: [NetService] = []

  func publish(name: String, port: Int32, txt: [String: String]) {
    unpublish()
    let service = NetService(domain: "local.", type: Self.type, name: name, port: port)
    service.setTXTRecord(NetService.data(fromTXTRecord: txt.mapValues { Data($0.utf8) }))
    service.delegate = self
    service.publish()
    published = service
  }

  func unpublish() {
    published?.stop()
    published = nil
  }

  func browse() {
    stopBrowsing()
    let b = NetServiceBrowser()
    b.delegate = self
    b.searchForServices(ofType: Self.type, inDomain: "local.")
    browser = b
  }

  func stopBrowsing() {
    browser?.stop()
    browser = nil
    resolving.forEach { $0.stop() }
    resolving.removeAll()
  }

  func netServiceBrowser(_ browser: NetServiceBrowser, didFind service: NetService, moreComing: Bool) {
    // Our own table is not somewhere to go.
    if let mine = published, mine.name == service.name { return }
    service.delegate = self
    resolving.append(service)
    service.resolve(withTimeout: 5)
  }

  func netServiceBrowser(_ browser: NetServiceBrowser, didRemove service: NetService, moreComing: Bool) {
    onLost?(service.name)
  }

  func netServiceDidResolveAddress(_ sender: NetService) {
    defer { resolving.removeAll { $0 === sender } }
    guard let address = Self.ipv4(of: sender) else { return }
    var fields: [String: String] = [:]
    if let data = sender.txtRecordData() {
      for (key, value) in NetService.dictionary(fromTXTRecord: data) {
        fields[key] = String(data: value, encoding: .utf8)
      }
    }
    onFound?([
      "name": sender.name,
      "address": address,
      "port": sender.port,
      "id": fields["id"] ?? "",
      "hostName": fields["n"] ?? sender.name,
      "protocol": Int(fields["v"] ?? "") ?? 0,
    ])
  }

  func netService(_ sender: NetService, didNotResolve errorDict: [String: NSNumber]) {
    resolving.removeAll { $0 === sender }
  }

  /// The first IPv4 address a resolved service answered with. IPv4 because
  /// every client on the other end (Android's OkHttp included) can dial it,
  /// which is not true of a link-local IPv6 address with a scope id.
  private static func ipv4(of service: NetService) -> String? {
    for data in service.addresses ?? [] {
      let found: String? = data.withUnsafeBytes { raw in
        guard let sa = raw.baseAddress?.assumingMemoryBound(to: sockaddr.self),
          sa.pointee.sa_family == sa_family_t(AF_INET)
        else { return nil }
        var host = [CChar](repeating: 0, count: Int(NI_MAXHOST))
        guard
          getnameinfo(sa, socklen_t(data.count), &host, socklen_t(host.count), nil, 0, NI_NUMERICHOST)
            == 0
        else { return nil }
        return String(cString: host)
      }
      if let found { return found }
    }
    return nil
  }
}

/// This phone's own IPv4 addresses on the networks a guest could share with
/// it: Wi-Fi, and the personal hotspot's bridge when it is sharing one.
enum NearbyAddresses {
  static func ipv4() -> [String] {
    var out: [String] = []
    var head: UnsafeMutablePointer<ifaddrs>?
    guard getifaddrs(&head) == 0, let first = head else { return out }
    defer { freeifaddrs(head) }
    for ptr in sequence(first: first, next: { $0.pointee.ifa_next }) {
      let ifa = ptr.pointee
      guard let sa = ifa.ifa_addr, sa.pointee.sa_family == sa_family_t(AF_INET) else { continue }
      let flags = Int32(ifa.ifa_flags)
      guard flags & IFF_UP != 0, flags & IFF_LOOPBACK == 0 else { continue }
      let name = String(cString: ifa.ifa_name)
      guard name.hasPrefix("en") || name.hasPrefix("bridge") else { continue }
      var host = [CChar](repeating: 0, count: Int(NI_MAXHOST))
      if getnameinfo(
        sa, socklen_t(sa.pointee.sa_len), &host, socklen_t(host.count), nil, 0, NI_NUMERICHOST) == 0
      {
        let address = String(cString: host)
        if !address.hasPrefix("169.254.") { out.append(address) }
      }
    }
    return out
  }
}
