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
  /// One per app: the core hosts one table per process, and every window
  /// sees the same one. Events go to every window's page.
  static let shared = NearbyService()

  private final class WeakBridge {
    weak var bridge: Bridge?
    init(_ b: Bridge) { bridge = b }
  }
  private var bridges: [WeakBridge] = []

  func add(_ bridge: Bridge) {
    bridges.removeAll { $0.bridge == nil }
    bridges.append(WeakBridge(bridge))
  }

  func remove(_ bridge: Bridge) {
    bridges.removeAll { $0.bridge == nil || $0.bridge === bridge }
  }

  private func emit(_ name: String, _ payload: Any) {
    for b in bridges { b.bridge?.emit(name, payload) }
  }

  private let bonjour = NearbyBonjour()
  private let bleHost = NearbyBleHost()
  private let bleGuest = NearbyBleGuest()
  private let thermal = NearbyThermal()
  private var ticker: Timer?

  init() {
    bonjour.onFound = { [weak self] host in self?.event("onHostFound", host) }
    bonjour.onLost = { [weak self] name in self?.event("onHostLost", ["name": name]) }
    bleGuest.onFound = { [weak self] host in
      self?.event("onBleFound", host)
      if let id = host["peripheralId"] as? String { self?.sighted(id) }
    }
    bleGuest.onMessage = { [weak self] linkId, data in
      // A link the core holds goes to the core; the page's own links, to the page.
      if self?.coreHolds(linkId) == true {
        ZolikcoreBleReceive(linkId, data)
        return
      }
      self?.event("onBleMessage", ["linkId": linkId, "data": data.base64EncodedString()])
    }
    bleGuest.onClosed = { [weak self] linkId in
      if self?.coreHolds(linkId) == true {
        self?.coreReleases(linkId)
        ZolikcoreBleClosed(linkId)
        return
      }
      self?.event("onBleClosed", ["linkId": linkId])
    }
    bleHost.onGuestsChanged = { [weak self] n in
      self?.event("onBleGuests", ["count": n])
      self?.pushState()
    }
  }

  // MARK: Bluetooth guests held by the core

  /// Links the core holds, and the peripheral each table was last reached at
  /// (only a first guess: a guest has no stable Bluetooth address).
  private let coreLock = NSLock()
  private var coreLinks = Set<String>()
  private var radioHints: [String: String] = [:]
  private var sightings: [UUID: (String) -> Void] = [:]
  private lazy var coreSource = CoreBleSource(service: self)

  fileprivate func coreHolds(_ linkId: String) -> Bool {
    coreLock.lock()
    defer { coreLock.unlock() }
    return coreLinks.contains(linkId)
  }

  fileprivate func coreTakes(_ linkId: String, instance: String, peripheral: String) {
    coreLock.lock()
    coreLinks.insert(linkId)
    radioHints[instance] = peripheral
    coreLock.unlock()
  }

  fileprivate func coreReleases(_ linkId: String) {
    coreLock.lock()
    coreLinks.remove(linkId)
    coreLock.unlock()
  }

  private func sighted(_ peripheral: String) {
    coreLock.lock()
    let watchers = Array(sightings.values)
    coreLock.unlock()
    for w in watchers { w(peripheral) }
  }

  /// Opens a link to the table with this instance id, finding it again if it
  /// must: the peripheral it was last at first, then whatever a scan turns up
  /// that says it is that table. Blocks; called from the core's goroutines.
  fileprivate func openLink(instance: String) -> String? {
    coreLock.lock()
    let hint = radioHints[instance]
    coreLock.unlock()
    var tried = Set<String>()
    if let hint {
      tried.insert(hint)
      if let link = connectBlocking(hint, instance) { return link }
    }
    let wake = DispatchSemaphore(value: 0)
    var found: [String] = []
    let watcher = UUID()
    coreLock.lock()
    sightings[watcher] = { id in
      self.coreLock.lock()
      found.append(id)
      self.coreLock.unlock()
      wake.signal()
    }
    coreLock.unlock()
    bleGuest.scan()
    defer {
      bleGuest.stopScan()
      coreLock.lock()
      sightings[watcher] = nil
      coreLock.unlock()
    }
    let deadline = Date().addingTimeInterval(8)
    while Date() < deadline {
      _ = wake.wait(timeout: .now() + 0.25)
      coreLock.lock()
      let next = found
      found = []
      coreLock.unlock()
      for id in next where tried.insert(id).inserted {
        if let link = connectBlocking(id, instance) { return link }
      }
    }
    return nil
  }

  private func connectBlocking(_ peripheral: String, _ instance: String) -> String? {
    let done = DispatchSemaphore(value: 0)
    var out: (String, Data)?
    radioConnect(peripheral) { result in
      if case .success(let v) = result { out = v }
      done.signal()
    }
    done.wait()
    guard let (link, info) = out else { return nil }
    let id = ((try? JSONSerialization.jsonObject(with: info)) as? [String: Any])?["id"] as? String
    guard id == instance else {
      radioDisconnect(link)
      return nil
    }
    coreTakes(link, instance: instance, peripheral: peripheral)
    return link
  }

  fileprivate func radioSend(_ linkId: String, _ msg: Data) -> Error? {
    let done = DispatchSemaphore(value: 0)
    var failure: Error?
    radioWrite(linkId, msg) { error in
      failure = error
      done.signal()
    }
    done.wait()
    return failure
  }

  fileprivate func radioHangUp(_ linkId: String) {
    radioDisconnect(linkId)
    coreReleases(linkId)
  }

  // The radio, or in an end-to-end run a stand-in for it: "e2e-loopback"
  // is a table reached through this process's own host, so the core's whole
  // guest path (this Swift, the gomobile bridge, the tunnel) runs without a
  // second device. Never in a release build.
  private static let loopbackPeripheral = "e2e-loopback"
  private var loopbackLinks: [String: ZolikcoreTunnel] = [:]

  fileprivate func radioConnect(_ peripheral: String, _ done: @escaping (Result<(String, Data), Error>) -> Void) {
    #if DEBUG
      if AppConfig.isE2E, peripheral == Self.loopbackPeripheral {
        guard let host = ZolikcoreCurrent() else { return done(.failure(Failure("no host is running"))) }
        let id = "loopback-" + UUID().uuidString
        let tunnel = host.newTunnel(LoopbackSink(linkId: id), peer: id)
        coreLock.lock()
        loopbackLinks[id] = tunnel
        coreLock.unlock()
        let info = try? JSONSerialization.data(withJSONObject: ["id": host.instanceID(), "v": ZolikcoreProtocolVersion])
        return done(.success((id, info ?? Data())))
      }
    #endif
    bleGuest.connect(peripheralId: peripheral, done: done)
  }

  /// For tests: the loopback radio loses every link at once.
  func dropLoopbackLinks() {
    #if DEBUG
      coreLock.lock()
      let ids = Array(loopbackLinks.keys)
      coreLock.unlock()
      for id in ids { radioDisconnect(id) }
    #endif
  }

  fileprivate func radioDisconnect(_ linkId: String) {
    #if DEBUG
      coreLock.lock()
      let fake = loopbackLinks.removeValue(forKey: linkId)
      coreLock.unlock()
      if let fake {
        fake.close()
        ZolikcoreBleClosed(linkId)
        return
      }
    #endif
    bleGuest.disconnect(linkId: linkId)
  }

  fileprivate func radioWrite(_ linkId: String, _ msg: Data, _ done: @escaping (Error?) -> Void) {
    #if DEBUG
      coreLock.lock()
      let fake = loopbackLinks[linkId]
      coreLock.unlock()
      if let fake {
        fake.receive(msg)
        return done(nil)
      }
    #endif
    bleGuest.send(linkId: linkId, msg: msg, done: done)
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
      case "relayJoin":
        // A table across the internet: the core holds the tunnel and serves
        // the table on this machine, so any window can use its address.
        let (code, instance, pinned) = (string(0), string(1), string(2))
        DispatchQueue.global(qos: .userInitiated).async {
          var error: NSError?
          guard let table = ZolikcoreJoinRelay(code, instance, pinned, &error) else {
            fail(Failure(error?.localizedDescription ?? "could not reach the table"))
            return
          }
          finish([
            "baseUrl": table.baseURL(), "instanceId": table.instanceID(), "hostKey": table.hostKey(),
            "checkCode": table.checkCode(),
          ])
        }
      case "bleOpen":
        // The first link to a table the core is about to hold: connects, and
        // says which table it is, before anything is spent on the handshake.
        let peripheral = string(0)
        radioConnect(peripheral) { [weak self] result in
          switch result {
          case .success(let (linkId, info)):
            let parsed = (try? JSONSerialization.jsonObject(with: info)) as? [String: Any]
            guard let instance = parsed?["id"] as? String else {
              self?.radioDisconnect(linkId)
              fail(Failure("not a Zolik table"))
              return
            }
            self?.coreTakes(linkId, instance: instance, peripheral: peripheral)
            finish(["linkId": linkId, "instanceId": instance, "v": parsed?["v"] ?? 0])
          case .failure(let e): fail(e)
          }
        }
      case "bleJoin":
        let (linkId, instance, pinned) = (string(0), string(1), string(2))
        DispatchQueue.global(qos: .userInitiated).async { [weak self] in
          var error: NSError?
          guard let table = ZolikcoreJoinBle(self?.coreSource, instance, linkId, pinned, &error) else {
            self?.radioDisconnect(linkId)
            self?.coreReleases(linkId)
            fail(Failure(error?.localizedDescription ?? "could not reach the table"))
            return
          }
          finish([
            "baseUrl": table.baseURL(), "instanceId": table.instanceID(), "hostKey": table.hostKey(),
            "checkCode": table.checkCode(),
          ])
        }
      case "bleLeaveLink":
        radioDisconnect(string(0))
        coreReleases(string(0))
        finish(nil)
      case "guestLeave":
        ZolikcoreLeaveGuestTable(string(0))
        finish(nil)
      case "guestStatus":
        let raw = ZolikcoreGuestStatus(string(0))
        let parsed = (raw.data(using: .utf8)).flatMap { try? JSONSerialization.jsonObject(with: $0) }
        finish(parsed)
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
    DispatchQueue.main.async { self.emit("nearby", ["event": name, "payload": payload]) }
  }

  private func pushState() {
    DispatchQueue.main.async { self.emit("state", self.snapshot()) }
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

/// The core's way to a guest's Bluetooth radio (server/mobile/zolikcore
/// BleSource): it asks for links and sends messages through them, and what
/// the radio hears comes back through ZolikcoreBleReceive and BleClosed.
private final class CoreBleSource: NSObject, ZolikcoreBleSourceProtocol {
  private unowned let service: NearbyService
  init(service: NearbyService) { self.service = service }

  func connect(_ instanceID: String?, error: NSErrorPointer) -> String {
    if let link = service.openLink(instance: instanceID ?? "") { return link }
    error?.pointee = NSError(
      domain: "Jokerless", code: 1, userInfo: [NSLocalizedDescriptionKey: "the table is out of range"])
    return ""
  }

  func send(_ linkID: String?, msg: Data?) throws {
    if let failure = service.radioSend(linkID ?? "", msg ?? Data()) { throw failure }
  }

  func disconnect(_ linkID: String?) { service.radioHangUp(linkID ?? "") }
}

#if DEBUG
  /// A loopback link's way back to the core: the host tunnel's messages go to
  /// the guest the way a radio's would.
  private final class LoopbackSink: NSObject, ZolikcoreTunnelSinkProtocol {
    let linkId: String
    init(linkId: String) { self.linkId = linkId }
    func send(_ msg: Data?) {
      let copy = msg ?? Data()
      DispatchQueue.global().async { ZolikcoreBleReceive(self.linkId, copy) }
    }
  }
#endif
