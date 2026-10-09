import AppKit
import WebKit
import Zolikcore

/// The page's only way out. bridge.js turns fetch, WebSocket and the
/// zolik-nearby module into messages; this answers them, handing network
/// requests to the core (which dials only Jokerless servers, this Mac and the
/// local network) and the nearby calls to NearbyService.
///
/// Messages are `{op, …}` objects posted to `window.webkit.messageHandlers.
/// zolik`; the reply resolves the page's promise. Things that happen later —
/// socket traffic, tables found nearby, Bluetooth — come back as events
/// through `window.__zolikDesktop.event(name, payload)`.
final class Bridge: NSObject, WKScriptMessageHandlerWithReply {
  weak var webView: WKWebView?
  var onLog: ((String) -> Void)?
  var onMenuState: (([String: Any]) -> Void)?
  /// The match screen's parts and rules target, or nil when no game is in front.
  var onViewState: (([String: Any]?) -> Void)?
  private let nearby: NearbyService
  private lazy var sink = NetSinkBridge(bridge: self)
  /// Fetches waiting for the core's answer, by the page's request id.
  private var pending: [Int64: (Any?, String?) -> Void] = [:]

  init(nearby: NearbyService) {
    self.nearby = nearby
    super.init()
    nearby.add(self)
  }

  func userContentController(
    _ controller: WKUserContentController, didReceive message: WKScriptMessage,
    replyHandler: @escaping (Any?, String?) -> Void
  ) {
    // Only the app's own page may use the bridge.
    guard message.frameInfo.isMainFrame, message.frameInfo.securityOrigin.protocol == WebController.scheme,
      let body = message.body as? [String: Any], let op = body["op"] as? String
    else {
      replyHandler(nil, "not allowed")
      return
    }
    switch op {
    case "fetch":
      let id = Self.int64(body["id"])
      let headers = (body["headers"] as? [String: String]) ?? [:]
      let headersJSON = (try? JSONSerialization.data(withJSONObject: headers)).flatMap {
        String(data: $0, encoding: .utf8)
      }
      let payload = (body["body"] as? String).flatMap { Data(base64Encoded: $0) }
      pending[id] = replyHandler
      ZolikcoreNetFetch(
        sink, id, body["method"] as? String ?? "GET", body["url"] as? String ?? "", headersJSON, payload)
    case "ws.open":
      ZolikcoreNetSocketOpen(sink, Self.int64(body["id"]), body["url"] as? String ?? "")
      replyHandler(true, nil)
    case "ws.send":
      var error: NSError?
      let ok = ZolikcoreNetSocketSend(Self.int64(body["id"]), body["data"] as? String ?? "", &error)
      replyHandler(ok, ok ? nil : error?.localizedDescription ?? "the socket is closed")
    case "ws.close":
      ZolikcoreNetSocketClose(
        Self.int64(body["id"]), (body["code"] as? Int) ?? 1000, body["reason"] as? String ?? "")
      replyHandler(true, nil)
    case "nearby":
      let method = body["method"] as? String ?? ""
      let args = body["args"] as? [Any] ?? []
      nearby.call(method, args) { [weak self] result in
        let state = self?.nearby.snapshot() ?? [:]
        switch result {
        case .success(let value): replyHandler(["value": value ?? NSNull(), "state": state], nil)
        case .failure(let error): replyHandler(nil, error.localizedDescription)
        }
      }
    case "open":
      if let raw = body["url"] as? String, let url = URL(string: raw) { WebController.openExternally(url) }
      replyHandler(true, nil)
    case "view":
      onViewState?(body["state"] as? [String: Any])
      replyHandler(true, nil)
    case "menu":
      onMenuState?(body["state"] as? [String: Any] ?? [:])
      replyHandler(true, nil)
    case "log":
      onLog?(body["line"] as? String ?? "")
      replyHandler(true, nil)
    default:
      replyHandler(nil, "unknown operation \(op)")
    }
  }

  fileprivate func fetchDone(_ id: Int64, _ status: Int, _ headersJSON: String?, _ body: Data?, _ err: String?) {
    guard let reply = pending.removeValue(forKey: id) else { return }
    if let err, !err.isEmpty {
      reply(nil, err)
      return
    }
    let headers =
      (headersJSON?.data(using: .utf8)).flatMap { try? JSONSerialization.jsonObject(with: $0) } ?? [:]
    reply(["status": status, "headers": headers, "body": (body ?? Data()).base64EncodedString()], nil)
  }

  func emit(_ name: String, _ payload: Any) {
    guard let webView,
      let data = try? JSONSerialization.data(withJSONObject: ["name": name, "payload": payload]),
      let json = String(data: data, encoding: .utf8)
    else { return }
    webView.evaluateJavaScript("window.__zolikDesktop && window.__zolikDesktop.event(\(json))")
  }

  static func int64(_ v: Any?) -> Int64 {
    if let n = v as? NSNumber { return n.int64Value }
    return 0
  }
}

/// The core's callbacks arrive on Go's threads; the page lives on the main one.
private final class NetSinkBridge: NSObject, ZolikcoreNetSinkProtocol {
  weak var bridge: Bridge?
  init(bridge: Bridge) { self.bridge = bridge }

  func fetchDone(_ id_: Int64, status: Int, headersJSON: String?, body: Data?, errMsg: String?) {
    DispatchQueue.main.async { self.bridge?.fetchDone(id_, status, headersJSON, body, errMsg) }
  }

  func socketOpened(_ id_: Int64) {
    DispatchQueue.main.async { self.bridge?.emit("ws", ["id": id_, "type": "open"]) }
  }

  func socketMessage(_ id_: Int64, data: String?) {
    DispatchQueue.main.async { self.bridge?.emit("ws", ["id": id_, "type": "message", "data": data ?? ""]) }
  }

  func socketClosed(_ id_: Int64, code: Int, reason: String?) {
    DispatchQueue.main.async {
      self.bridge?.emit("ws", ["id": id_, "type": "close", "code": code, "reason": reason ?? ""])
    }
  }
}
