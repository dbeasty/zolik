import Foundation

// Shared by the iPhone module and the Mac app (client-macos), which compiles
// this file as it is. Keep it free of Expo and of UIKit.

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

  func netServiceBrowser(_ browser: NetServiceBrowser, didNotSearch errorDict: [String: NSNumber]) {
    NSLog("NearbyBonjour: browsing failed: %@", errorDict)
  }

  func netService(_ sender: NetService, didNotPublish errorDict: [String: NSNumber]) {
    NSLog("NearbyBonjour: publishing %@ failed: %@", sender.name, errorDict)
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
    NSLog("NearbyBonjour: %@ did not resolve: %@", sender.name, errorDict)
    resolving.removeAll { $0 === sender }
  }

  /// The IPv4 address to reach a resolved service at. IPv4 because every
  /// client on the other end (Android's OkHttp included) can dial it, which
  /// is not true of a link-local IPv6 address with a scope id.
  ///
  /// A host answers with an address per interface, and a Mac often has more
  /// than the Wi-Fi: a bridge for virtual machines or Docker answers too,
  /// and is unreachable from anywhere else. So the address on the same /24 as
  /// one of this device's own networks wins, and the first one otherwise.
  private static func ipv4(of service: NetService) -> String? {
    var all: [String] = []
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
      if let found, !all.contains(found) { all.append(found) }
    }
    return preferred(all, mine: NearbyAddresses.ipv4())
  }

  /// The first of `addresses` sharing a /24 with `mine`, taken in the order
  /// of `mine` (Wi-Fi before bridges), or the first address at all.
  static func preferred(_ addresses: [String], mine: [String]) -> String? {
    func net(_ a: String) -> String { a.split(separator: ".").prefix(3).joined(separator: ".") }
    for own in mine {
      if let hit = addresses.first(where: { net($0) == net(own) }) { return hit }
    }
    return addresses.first
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
