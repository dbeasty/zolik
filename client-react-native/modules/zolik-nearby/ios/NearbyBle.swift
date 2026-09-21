import CoreBluetooth
import Foundation
import Zolikcore

/// Bluetooth for a table with no network at all.
///
/// The host is a GATT peripheral and guests are centrals. The phones never
/// pair: every characteristic is plain, so neither iOS nor Android ever
/// shows a pairing dialog. Security is the Go tunnel's job
/// (server/mobile/zolikcore/tunnel.go), which encrypts every message end to
/// end. This file only moves whole messages:
///
///   message on the air:  4-byte big-endian length | payload,
///                        cut into chunks the link's MTU allows
///
/// with one write or notification in flight at a time, so nothing is ever
/// dropped by a full queue.
enum NearbyBleIDs {
  static let service = CBUUID(string: "A54A0C00-3F5E-4D2A-9C47-5A6F6C696B01")
  static let info = CBUUID(string: "A54A0C01-3F5E-4D2A-9C47-5A6F6C696B01")
  static let c2h = CBUUID(string: "A54A0C02-3F5E-4D2A-9C47-5A6F6C696B01")
  static let h2c = CBUUID(string: "A54A0C03-3F5E-4D2A-9C47-5A6F6C696B01")
}

// MARK: - Host (peripheral)

/// Hands a tunnel's outgoing messages to the peripheral. Called on Go's
/// threads, so it only enqueues: blocking here would stall the tunnel.
final class TunnelSinkBridge: NSObject, ZolikcoreTunnelSinkProtocol {
  weak var host: NearbyBleHost?
  let central: CBCentral

  init(host: NearbyBleHost, central: CBCentral) {
    self.host = host
    self.central = central
  }

  func send(_ msg: Data?) {
    guard let msg else { return }
    host?.enqueue(msg, to: central)
  }
}

final class NearbyBleHost: NSObject, CBPeripheralManagerDelegate {
  private let queue = DispatchQueue(label: "zolik.ble.host")
  private var manager: CBPeripheralManager?
  private var h2c: CBMutableCharacteristic?
  private var info = Data()
  private var name = ""
  private var guests: [UUID: Guest] = [:]
  var onGuestsChanged: ((Int) -> Void)?

  private final class Guest {
    let central: CBCentral
    let tunnel: ZolikcoreTunnel
    let reassembler = Reassembler()
    var outbox: [Data] = []
    init(central: CBCentral, tunnel: ZolikcoreTunnel) {
      self.central = central
      self.tunnel = tunnel
    }
  }

  func start(name: String) {
    queue.async {
      self.name = name
      self.info = ZolikcoreCurrent()?.bleInfo(name) ?? Data()
      if self.manager == nil {
        self.manager = CBPeripheralManager(delegate: self, queue: self.queue)
      } else if self.manager?.state == .poweredOn {
        self.publish()
      }
    }
  }

  /// The check code of every guest that has finished its handshake, for the
  /// host to show beside what each guest's own screen says.
  func codes() -> [String] {
    queue.sync { guests.values.map { $0.tunnel.checkCode() }.filter { !$0.isEmpty }.sorted() }
  }

  func stop() {
    queue.async {
      self.manager?.stopAdvertising()
      self.manager?.removeAllServices()
      for g in self.guests.values { g.tunnel.close() }
      self.guests.removeAll()
      self.onGuestsChanged?(0)
    }
  }

  func peripheralManagerDidUpdateState(_ peripheral: CBPeripheralManager) {
    if peripheral.state == .poweredOn { publish() }
  }

  private func publish() {
    guard let manager else { return }
    manager.removeAllServices()
    let info = CBMutableCharacteristic(
      type: NearbyBleIDs.info, properties: [.read], value: nil, permissions: [.readable])
    let c2h = CBMutableCharacteristic(
      type: NearbyBleIDs.c2h, properties: [.write], value: nil, permissions: [.writeable])
    let h2c = CBMutableCharacteristic(
      type: NearbyBleIDs.h2c, properties: [.notify], value: nil, permissions: [.readable])
    self.h2c = h2c
    let service = CBMutableService(type: NearbyBleIDs.service, primary: true)
    service.characteristics = [info, c2h, h2c]
    manager.add(service)
    manager.startAdvertising([
      CBAdvertisementDataServiceUUIDsKey: [NearbyBleIDs.service],
      CBAdvertisementDataLocalNameKey: String(name.prefix(20)),
    ])
  }

  func peripheralManager(_ peripheral: CBPeripheralManager, didReceiveRead request: CBATTRequest) {
    guard request.characteristic.uuid == NearbyBleIDs.info else {
      peripheral.respond(to: request, withResult: .requestNotSupported)
      return
    }
    // Long reads arrive in pieces at increasing offsets.
    guard request.offset <= info.count else {
      peripheral.respond(to: request, withResult: .invalidOffset)
      return
    }
    request.value = info.subdata(in: request.offset..<info.count)
    peripheral.respond(to: request, withResult: .success)
  }

  func peripheralManager(_ peripheral: CBPeripheralManager, didReceiveWrite requests: [CBATTRequest]) {
    for request in requests where request.characteristic.uuid == NearbyBleIDs.c2h {
      guard let chunk = request.value, let guest = guestFor(request.central) else { continue }
      for msg in guest.reassembler.feed(chunk) {
        guest.tunnel.receive(msg)
      }
    }
    if let first = requests.first { peripheral.respond(to: first, withResult: .success) }
  }

  func peripheralManager(
    _ peripheral: CBPeripheralManager, central: CBCentral, didSubscribeTo characteristic: CBCharacteristic
  ) {
    _ = guestFor(central)
  }

  func peripheralManager(
    _ peripheral: CBPeripheralManager, central: CBCentral, didUnsubscribeFrom characteristic: CBCharacteristic
  ) {
    // Unsubscribing is how a central that disconnected shows up here.
    if let g = guests.removeValue(forKey: central.identifier) {
      g.tunnel.close()
      onGuestsChanged?(guests.count)
    }
  }

  /// The guest a central is, starting its tunnel the first time. Nil when the
  /// Go host has stopped underneath a write that was already in the air.
  private func guestFor(_ central: CBCentral) -> Guest? {
    if let g = guests[central.identifier] { return g }
    let bridge = TunnelSinkBridge(host: self, central: central)
    guard let tunnel = ZolikcoreCurrent()?.newTunnel(bridge, peer: central.identifier.uuidString) else {
      return nil
    }
    let g = Guest(central: central, tunnel: tunnel)
    guests[central.identifier] = g
    onGuestsChanged?(guests.count)
    return g
  }

  func enqueue(_ msg: Data, to central: CBCentral) {
    queue.async {
      guard let g = self.guests[central.identifier] else { return }
      g.outbox.append(contentsOf: frameChunks(msg, size: min(central.maximumUpdateValueLength, 512)))
      self.drain()
    }
  }

  /// Sends queued chunks until the stack's queue is full. The stack says when
  /// there is room again, and this runs again.
  private func drain() {
    guard let manager, let h2c else { return }
    for g in guests.values {
      while let chunk = g.outbox.first {
        if !manager.updateValue(chunk, for: h2c, onSubscribedCentrals: [g.central]) { return }
        g.outbox.removeFirst()
      }
    }
  }

  func peripheralManagerIsReady(toUpdateSubscribers peripheral: CBPeripheralManager) {
    drain()
  }
}

// MARK: - Guest (central)

final class NearbyBleGuest: NSObject, CBCentralManagerDelegate, CBPeripheralDelegate {
  private let queue = DispatchQueue(label: "zolik.ble.guest")
  private var manager: CBCentralManager?
  private var poweredOn: [() -> Void] = []
  private var seen: [UUID: CBPeripheral] = [:]
  private var links: [String: Link] = [:]

  var onFound: (([String: Any]) -> Void)?
  var onMessage: ((String, Data) -> Void)?
  var onClosed: ((String) -> Void)?

  private final class Link {
    let id: String
    let peripheral: CBPeripheral
    var c2h: CBCharacteristic?
    var h2c: CBCharacteristic?
    var info: CBCharacteristic?
    let reassembler = Reassembler()
    var outbox: [(Data, Bool)] = []  // chunk, last-of-message
    var writing = false
    var sendDone: [() -> Void] = []
    var connected: ((Result<String, Error>) -> Void)?
    init(id: String, peripheral: CBPeripheral) {
      self.id = id
      self.peripheral = peripheral
    }
  }

  private func whenOn(_ run: @escaping () -> Void) {
    queue.async {
      if self.manager == nil {
        self.manager = CBCentralManager(delegate: self, queue: self.queue)
      }
      if self.manager?.state == .poweredOn { run() } else { self.poweredOn.append(run) }
    }
  }

  private var settled: [(String) -> Void] = []

  func centralManagerDidUpdateState(_ central: CBCentralManager) {
    if central.state != .unknown && central.state != .resetting {
      let waiting = settled
      settled = []
      let s = state()
      waiting.forEach { $0(s) }
    }
    guard central.state == .poweredOn else { return }
    let waiting = poweredOn
    poweredOn = []
    waiting.forEach { $0() }
  }

  /// The radio's state once iOS knows it. Creating the manager is what asks
  /// the player for Bluetooth, so this runs on a tap, never on arrival.
  func ready(_ done: @escaping (String) -> Void) {
    queue.async {
      if self.manager == nil {
        self.manager = CBCentralManager(delegate: self, queue: self.queue)
      }
      if let m = self.manager, m.state != .unknown, m.state != .resetting {
        done(self.state())
        return
      }
      self.settled.append(done)
      // iOS answers at once, or once the player has answered the prompt.
      // Past that, say what is known rather than wait for ever.
      self.queue.asyncAfter(deadline: .now() + 30) {
        guard !self.settled.isEmpty else { return }
        let waiting = self.settled
        self.settled = []
        waiting.forEach { $0(self.state()) }
      }
    }
  }

  func scan() {
    whenOn {
      self.manager?.scanForPeripherals(
        withServices: [NearbyBleIDs.service], options: [CBCentralManagerScanOptionAllowDuplicatesKey: false])
    }
  }

  func stopScan() {
    queue.async { self.manager?.stopScan() }
  }

  func centralManager(
    _ central: CBCentralManager, didDiscover peripheral: CBPeripheral, advertisementData: [String: Any],
    rssi RSSI: NSNumber
  ) {
    seen[peripheral.identifier] = peripheral
    var name = advertisementData[CBAdvertisementDataLocalNameKey] as? String
    if let data = (advertisementData[CBAdvertisementDataServiceDataKey] as? [CBUUID: Data])?[NearbyBleIDs.service] {
      name = String(data: data, encoding: .utf8) ?? name
    }
    onFound?(["peripheralId": peripheral.identifier.uuidString, "name": name ?? "", "rssi": RSSI.intValue])
  }

  func connect(peripheralId: String, done: @escaping (Result<(String, Data), Error>) -> Void) {
    whenOn {
      guard let uuid = UUID(uuidString: peripheralId),
        let peripheral = self.seen[uuid] ?? self.manager?.retrievePeripherals(withIdentifiers: [uuid]).first
      else {
        done(.failure(BleError("that table is out of range")))
        return
      }
      let link = Link(id: UUID().uuidString, peripheral: peripheral)
      self.links[link.id] = link
      peripheral.delegate = self
      link.connected = { result in
        switch result {
        case .success:
          // The info characteristic, read now: it says which table this is
          // before a single tunnel message is spent.
          guard let info = link.info else {
            done(.failure(BleError("not a Zolik table")))
            return
          }
          self.pendingInfo[link.id] = { data in done(.success((link.id, data))) }
          peripheral.readValue(for: info)
        case .failure(let e):
          self.links.removeValue(forKey: link.id)
          done(.failure(e))
        }
      }
      self.manager?.connect(peripheral)
      // A connect never times out on its own in CoreBluetooth.
      self.queue.asyncAfter(deadline: .now() + 10) {
        if link.connected != nil, link.c2h == nil {
          self.manager?.cancelPeripheralConnection(peripheral)
          link.connected?(.failure(BleError("the table did not answer")))
          link.connected = nil
        }
      }
    }
  }

  private var pendingInfo: [String: (Data) -> Void] = [:]

  func centralManager(_ central: CBCentralManager, didConnect peripheral: CBPeripheral) {
    peripheral.discoverServices([NearbyBleIDs.service])
  }

  func centralManager(_ central: CBCentralManager, didFailToConnect peripheral: CBPeripheral, error: Error?) {
    for link in links.values where link.peripheral === peripheral {
      link.connected?(.failure(error ?? BleError("could not connect")))
      link.connected = nil
    }
  }

  func centralManager(
    _ central: CBCentralManager, didDisconnectPeripheral peripheral: CBPeripheral, error: Error?
  ) {
    for (id, link) in links where link.peripheral === peripheral {
      links.removeValue(forKey: id)
      link.connected?(.failure(error ?? BleError("disconnected")))
      link.connected = nil
      onClosed?(id)
    }
  }

  func peripheral(_ peripheral: CBPeripheral, didDiscoverServices error: Error?) {
    guard let service = peripheral.services?.first(where: { $0.uuid == NearbyBleIDs.service }) else {
      fail(peripheral, "not a Zolik table")
      return
    }
    peripheral.discoverCharacteristics([NearbyBleIDs.info, NearbyBleIDs.c2h, NearbyBleIDs.h2c], for: service)
  }

  func peripheral(_ peripheral: CBPeripheral, didDiscoverCharacteristicsFor service: CBService, error: Error?) {
    guard let link = links.values.first(where: { $0.peripheral === peripheral }) else { return }
    for c in service.characteristics ?? [] {
      switch c.uuid {
      case NearbyBleIDs.info: link.info = c
      case NearbyBleIDs.c2h: link.c2h = c
      case NearbyBleIDs.h2c: link.h2c = c
      default: break
      }
    }
    guard let h2c = link.h2c, link.c2h != nil else {
      fail(peripheral, "not a Zolik table")
      return
    }
    peripheral.setNotifyValue(true, for: h2c)
  }

  func peripheral(
    _ peripheral: CBPeripheral, didUpdateNotificationStateFor characteristic: CBCharacteristic, error: Error?
  ) {
    guard let link = links.values.first(where: { $0.peripheral === peripheral }) else { return }
    if let error {
      link.connected?(.failure(error))
    } else {
      link.connected?(.success(link.id))
    }
    link.connected = nil
  }

  /// The host's service went away under a live link: its table closed, or
  /// it restarted Bluetooth. Drop the link, and reconnection finds the table
  /// again if it comes back.
  func peripheral(_ peripheral: CBPeripheral, didModifyServices invalidatedServices: [CBService]) {
    if invalidatedServices.contains(where: { $0.uuid == NearbyBleIDs.service }) {
      manager?.cancelPeripheralConnection(peripheral)
    }
  }

  func peripheral(_ peripheral: CBPeripheral, didUpdateValueFor characteristic: CBCharacteristic, error: Error?) {
    guard let link = links.values.first(where: { $0.peripheral === peripheral }),
      let value = characteristic.value
    else { return }
    if characteristic.uuid == NearbyBleIDs.info {
      pendingInfo.removeValue(forKey: link.id)?(value)
      return
    }
    for msg in link.reassembler.feed(value) {
      onMessage?(link.id, msg)
    }
  }

  func send(linkId: String, msg: Data, done: @escaping (Error?) -> Void) {
    queue.async {
      guard let link = self.links[linkId] else {
        done(BleError("the link to the table is closed"))
        return
      }
      let size = link.peripheral.maximumWriteValueLength(for: .withResponse)
      let chunks = frameChunks(msg, size: min(size, 512))
      for (i, c) in chunks.enumerated() { link.outbox.append((c, i == chunks.count - 1)) }
      link.sendDone.append { done(nil) }
      self.pump(link)
    }
  }

  /// Writes the next chunk once the previous one is acknowledged: writes with
  /// response are what make the guest-to-host direction lossless.
  private func pump(_ link: Link) {
    guard !link.writing, let c2h = link.c2h, !link.outbox.isEmpty else { return }
    link.writing = true
    link.peripheral.writeValue(link.outbox[0].0, for: c2h, type: .withResponse)
  }

  func peripheral(_ peripheral: CBPeripheral, didWriteValueFor characteristic: CBCharacteristic, error: Error?) {
    guard let link = links.values.first(where: { $0.peripheral === peripheral }) else { return }
    link.writing = false
    if error != nil {
      disconnect(linkId: link.id)
      return
    }
    let (_, last) = link.outbox.removeFirst()
    if last, !link.sendDone.isEmpty { link.sendDone.removeFirst()() }
    pump(link)
  }

  func disconnect(linkId: String) {
    queue.async {
      guard let link = self.links[linkId] else { return }
      self.manager?.cancelPeripheralConnection(link.peripheral)
    }
  }

  private func fail(_ peripheral: CBPeripheral, _ why: String) {
    for link in links.values where link.peripheral === peripheral {
      link.connected?(.failure(BleError(why)))
      link.connected = nil
    }
    manager?.cancelPeripheralConnection(peripheral)
  }

  /// Whether Bluetooth can be used at all, for the screen to say why not.
  func state() -> String {
    switch CBManager.authorization {
    case .denied, .restricted: return "unauthorized"
    default: break
    }
    switch manager?.state {
    case .poweredOn: return "on"
    case .poweredOff: return "off"
    case .unsupported: return "unsupported"
    case .unauthorized: return "unauthorized"
    default: return "unknown"
    }
  }
}

struct BleError: LocalizedError {
  let message: String
  init(_ message: String) { self.message = message }
  var errorDescription: String? { message }
}
