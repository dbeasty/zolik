import AppKit
import WebKit
import Zolikcore

/// The window's content: a WKWebView showing the web client, which only draws
/// the screens and runs their JavaScript. It loads its pages from the core
/// through the `app:` scheme and sends every request it makes through the
/// bridge (bridge.js, Bridge.swift), so the web view itself never opens a
/// network connection. A content security policy on every page makes the
/// engine refuse anything else, whatever the page tries.
final class WebController: NSObject, WKNavigationDelegate, WKUIDelegate {
  static let scheme = "app"
  /// A test run's store is in memory, so it starts empty and leaves nothing
  /// behind, unless the run asks for one that survives a relaunch (a store
  /// named by ZOLIK_E2E_STORE_ID, which the run removes when it is done).
  private static let testStore: WKWebsiteDataStore = {
    #if DEBUG
      if #available(macOS 14.0, *), let raw = ProcessInfo.processInfo.environment["ZOLIK_E2E_STORE_ID"],
        let id = UUID(uuidString: raw)
      {
        return WKWebsiteDataStore(forIdentifier: id)
      }
    #endif
    return WKWebsiteDataStore.nonPersistent()
  }()
  static let origin = "app://jokerless"

  /// Which of the app's windows this page lives in: the one main window, or
  /// one game's. The page is told at load (`__ZOLIK_DESKTOP__.window`).
  struct Spec {
    enum Role: String { case main, game }
    var role: Role
    var matchId: String?
  }

  let webView: WKWebView
  let nearby: NearbyService
  private(set) var spec: Spec
  /// The seat the app holds (the offline table the player sits at), read
  /// before every page load so a window opened later finds the player seated.
  private let seat: () -> [String: Any]?
  /// The match ids that have a game window open, for the pages that list
  /// games (a game already open is shown, not resumed).
  var openGames: () -> [String] = { [] }
  private let bridge: Bridge
  private let schemeHandler = AppSchemeHandler()
  /// Called for console output and policy violations in an end-to-end run.
  var onLog: ((String) -> Void)? {
    didSet { bridge.onLog = onLog }
  }
  /// Called when the page says what the Account menu should hold.
  var onMenuState: (([String: Any]) -> Void)? {
    didSet { bridge.onMenuState = onMenuState }
  }
  /// The page asks the app to do something that is the app's to do: open a
  /// game window, show a screen in the main window, say what the title bar
  /// should carry, move the seat, count invites. See AppDelegate.appOp.
  var onAppOp: ((String, [String: Any]) -> Void)? {
    didSet { bridge.onAppOp = onAppOp }
  }
  /// Called when the game screen in front changes what it can show or hide.
  var onViewState: (([String: Any]?) -> Void)? {
    didSet { bridge.onViewState = onViewState }
  }

  var view: NSView { webView }

  init(nearby: NearbyService, spec: Spec, seat: @escaping () -> [String: Any]?) {
    self.spec = spec
    self.seat = seat
    let config = WKWebViewConfiguration()
    config.setURLSchemeHandler(schemeHandler, forURLScheme: Self.scheme)
    // Every window shares one store, so they share the signed-in player. A
    // test run's store is in memory: it starts empty and leaves nothing behind.
    config.websiteDataStore = AppConfig.isE2E ? Self.testStore : .default()
    config.preferences.javaScriptCanOpenWindowsAutomatically = false
    config.preferences.isElementFullscreenEnabled = true
    config.mediaTypesRequiringUserActionForPlayback = []

    let controller = config.userContentController

    webView = WKWebView(frame: .zero, configuration: config)
    webView.setValue(false, forKey: "drawsBackground")
    webView.allowsBackForwardNavigationGestures = true
    #if DEBUG
      if #available(macOS 13.3, *) { webView.isInspectable = true }
      // An end-to-end run's windows sit behind whatever is on the screen.
      // WebKit treats a covered window as hidden and stops animation frames,
      // so cards dealt there never finish appearing. Test runs only; a
      // shipped app behaves like any other window. (Key-value coding finds
      // WebKit's `_setWindowOcclusionDetectionEnabled:` for this key.)
      if AppConfig.isE2E, ProcessInfo.processInfo.environment["ZOLIK_E2E_OCCLUSION"] != "1",
        webView.responds(to: NSSelectorFromString("_setWindowOcclusionDetectionEnabled:")) {
        webView.setValue(false, forKey: "windowOcclusionDetectionEnabled")
      }
    #endif

    self.nearby = nearby
    bridge = Bridge(nearby: nearby)
    super.init()

    bridge.webView = webView
    controller.addScriptMessageHandler(bridge, contentWorld: .page, name: "zolik")
    webView.navigationDelegate = self
    webView.uiDelegate = self
    refreshBootScript()

    bridge.onNav = { [weak self] dir in
      if dir == "forward" { self?.goForward() } else { self?.goBack() }
    }
    // The header's arrows follow the window's history as it changes.
    navWatch = [
      webView.observe(\.canGoBack) { [weak self] _, _ in self?.emitNav() },
      webView.observe(\.canGoForward) { [weak self] _, _ in self?.emitNav() },
      webView.observe(\.url) { [weak self] _, _ in
        self?.settleForward()
        self?.emitNav()
      },
    ]
  }

  private var navWatch: [NSKeyValueObservation] = []

  private var navState: [String: Any] {
    ["canGoBack": canBack, "canGoForward": canForward]
  }

  /// The home screen is where Back stops: there is nowhere further back to go,
  /// whatever the window's history still holds.
  var atHome: Bool {
    guard let path = webView.url?.path else { return false }
    return path.isEmpty || path == "/" || path == "/index.html"
  }

  var canBack: Bool { !atHome && webView.canGoBack }

  /// Forward is for returning to where the player's own Back left. The page
  /// goes back by itself too (leaving the sign-in screen, say), which leaves
  /// WebKit with a forward entry nobody asked for; that is not offered.
  private var forwardIsTheirs = false
  private var arrowNav = false

  var canForward: Bool { forwardIsTheirs && webView.canGoForward }

  /// The window's address changed: by an arrow, Forward stays theirs; by
  /// anything else, whatever lies ahead is not.
  private func settleForward() {
    if !arrowNav { forwardIsTheirs = false }
    arrowNav = false
  }

  private func emitNav() {
    bridge.emit("nav", navState)
  }

  /// Back through the window's history; with nothing behind, the page's own
  /// rule (home). Forward can always return to where Back left.
  func goBack() {
    if atHome { return }
    if webView.canGoBack {
      arrowNav = true
      forwardIsTheirs = true
      webView.goBack()
    } else {
      command("back")
    }
  }

  func goForward() {
    if canForward {
      arrowNav = true
      webView.goForward()
    }
  }

  /// Lets go of the page when its window closes: the script handler would
  /// otherwise keep the bridge, and through it this controller, alive.
  func detach() {
    navWatch = []
    webView.configuration.userContentController.removeAllScriptMessageHandlers()
    nearby.remove(bridge)
    webView.stopLoading()
  }

  /// Shows or hides a part of the game in front: "hand", "table" or "log".
  func toggleView(_ part: String) {
    let quoted = (try? JSONSerialization.data(withJSONObject: [part])).flatMap { String(data: $0, encoding: .utf8) } ?? "[]"
    webView.evaluateJavaScript("window.__zolikView && window.__zolikView(\(quoted)[0])")
  }

  /// Runs one of the page's own commands (DesktopMenuBridge): "signOut", "back".
  func command(_ name: String) {
    let quoted = (try? JSONSerialization.data(withJSONObject: [name])).flatMap { String(data: $0, encoding: .utf8) } ?? "[]"
    webView.evaluateJavaScript("window.__zolikCommand && window.__zolikCommand(\(quoted)[0])")
  }

  func load(path: String = "/") {
    webView.load(URLRequest(url: URL(string: Self.origin + path)!))
  }

  func emitGames(_ ids: [String]) {
    bridge.emit("games", ["open": ids])
  }

  /// Another window's seat change, for this page to follow.
  func emitSeat(_ table: Any?) {
    bridge.emit("seat", ["table": table ?? NSNull()])
  }

  /// A game window that moves on to another match (a rematch) is that
  /// match's window from then on; its next load says so.
  func setMatch(_ id: String?) { spec.matchId = id }

  /// Moves to another screen inside the app, the way a tap would. Only if
  /// the page is not ready to be asked does it load the screen afresh.
  func navigate(to path: String) {
    let quoted = (try? JSONSerialization.data(withJSONObject: [path])).flatMap { String(data: $0, encoding: .utf8) } ?? "[]"
    webView.evaluateJavaScript(
      "(function(p){ if (window.__zolikNavigate) { window.__zolikNavigate(p); return true } return false })(\(quoted)[0])"
    ) { [weak self] result, _ in
      guard let self, (result as? Bool) != true, let url = URL(string: Self.origin + path) else { return }
      self.webView.load(URLRequest(url: url))
    }
  }

  /// The answers the page reads synchronously on a phone (hostStatus and the
  /// like) must be there before its bundle runs, or a page loaded while a
  /// table is up would think there is none. So the boot script is rebuilt
  /// with the current ones before every page load.
  func refreshBootScript() {
    let controller = webView.configuration.userContentController
    controller.removeAllUserScripts()
    controller.addUserScript(
      WKUserScript(
        source: Self.bootScript(
          state: nearby.snapshot().merging(["nav": navState, "seat": seat() ?? NSNull(), "games": openGames()]) { a, _ in a }, spec: spec),
        injectionTime: .atDocumentStart, forMainFrameOnly: true))
    if AppConfig.isE2E, let e2e = Self.resource("e2e.js") {
      controller.addUserScript(WKUserScript(source: e2e, injectionTime: .atDocumentStart, forMainFrameOnly: true))
    }
  }

  /// The configuration the page reads before its bundle runs, then the
  /// bridge itself.
  private static func bootScript(state: [String: Any], spec: Spec) -> String {
    var window: [String: Any] = ["role": spec.role.rawValue]
    if let id = spec.matchId { window["matchId"] = id }
    let config: [String: Any] = [
      "window": window,
      "baseUrl": AppConfig.serverURL,
      "cloudUrl": AppConfig.serverURL,
      "platform": "mac",
      "version": AppConfig.version,
      "protocol": ZolikcoreProtocolVersion,
    ]
    let json = (try? JSONSerialization.data(withJSONObject: config)).flatMap { String(data: $0, encoding: .utf8) } ?? "{}"
    let stateJSON = (try? JSONSerialization.data(withJSONObject: state)).flatMap { String(data: $0, encoding: .utf8) } ?? "{}"
    return "window.__ZOLIK_DESKTOP__ = Object.freeze(\(json));\nwindow.__ZOLIK_DESKTOP_STATE__ = \(stateJSON);\n"
      + (resource("bridge.js") ?? "")
  }

  static func resource(_ name: String) -> String? {
    guard let url = Bundle.main.resourceURL?.appendingPathComponent(name) else { return nil }
    return try? String(contentsOf: url, encoding: .utf8)
  }

  // MARK: WKNavigationDelegate

  /// The app's own pages stay in the window. Anything else — a legal page,
  /// the source code, a sign-in provider — opens in the person's browser.
  func webView(
    _ webView: WKWebView, decidePolicyFor action: WKNavigationAction,
    decisionHandler: @escaping (WKNavigationActionPolicy) -> Void
  ) {
    guard let url = action.request.url else { return decisionHandler(.cancel) }
    if url.scheme == Self.scheme {
      if action.targetFrame?.isMainFrame ?? true {
        refreshBootScript()
        // A new page has no game in front until it says so.
        bridge.onViewState?(nil)
      }
      return decisionHandler(.allow)
    }
    if url.scheme == "about" || url.scheme == "blob" || url.scheme == "data" {
      return decisionHandler(.allow)
    }
    Self.openExternally(url)
    decisionHandler(.cancel)
  }

  func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) { emitNav() }

  func webView(_ webView: WKWebView, webContentProcessDidTerminate: WKWebView) {
    // The page's process can be ended by the system under memory pressure.
    // The tables live in this process, not that one, so a reload loses
    // nothing but the screen.
    webView.reload()
  }

  // MARK: WKUIDelegate

  /// window.open and target=_blank links: always the person's browser.
  func webView(
    _ webView: WKWebView, createWebViewWith configuration: WKWebViewConfiguration,
    for action: WKNavigationAction, windowFeatures: WKWindowFeatures
  ) -> WKWebView? {
    if let url = action.request.url { Self.openExternally(url) }
    return nil
  }

  func webView(
    _ webView: WKWebView, runJavaScriptAlertPanelWithMessage message: String,
    initiatedByFrame frame: WKFrameInfo, completionHandler: @escaping () -> Void
  ) {
    let alert = NSAlert()
    alert.messageText = message
    alert.runModal()
    completionHandler()
  }

  func webView(
    _ webView: WKWebView, runJavaScriptConfirmPanelWithMessage message: String,
    initiatedByFrame frame: WKFrameInfo, completionHandler: @escaping (Bool) -> Void
  ) {
    let alert = NSAlert()
    alert.messageText = message
    alert.addButton(withTitle: "OK")
    alert.addButton(withTitle: "Cancel")
    completionHandler(alert.runModal() == .alertFirstButtonReturn)
  }

  static func openExternally(_ url: URL) {
    guard let scheme = url.scheme?.lowercased(), ["http", "https", "mailto"].contains(scheme) else {
      NSLog("Jokerless: not opening %@", url.absoluteString)
      return
    }
    NSWorkspace.shared.open(url)
  }
}

/// Serves the app's own pages from the core, in process. Every page carries
/// the content security policy that keeps the web view off the network.
final class AppSchemeHandler: NSObject, WKURLSchemeHandler {
  /// Scripts, styles, fonts and images come from the app; connections only
  /// to the app itself, which in practice means none, since the bridge
  /// carries every request. `unsafe-inline` because Expo's static export
  /// inlines its boot script and styles.
  static let policy = [
    "default-src 'self'",
    "script-src 'self' 'unsafe-inline'",
    "style-src 'self' 'unsafe-inline'",
    "img-src 'self' data: blob:",
    "font-src 'self' data:",
    "media-src 'self' data: blob:",
    "connect-src 'self'",
    "worker-src 'self' blob:",
    "frame-src 'none'",
    "object-src 'none'",
    "base-uri 'self'",
    "form-action 'none'",
  ].joined(separator: "; ")

  /// Tasks WebKit still wants an answer for; one it stopped is never answered.
  private var active = Set<ObjectIdentifier>()
  private let queue = DispatchQueue(label: "jokerless.assets", qos: .userInitiated)

  func webView(_ webView: WKWebView, start task: WKURLSchemeTask) {
    let id = ObjectIdentifier(task)
    guard let url = task.request.url else {
      task.didFailWithError(URLError(.badURL))
      return
    }
    let path = url.path.isEmpty ? "/" : url.path
    active.insert(id)
    queue.async {
      let answer = ZolikcoreWebAsset(path)
      DispatchQueue.main.async {
        guard self.active.remove(id) != nil else { return }
        let status = answer?.status ?? 404
        let headers = [
          "Content-Type": answer?.contentType.isEmpty == false ? answer!.contentType : "application/octet-stream",
          "Content-Security-Policy": Self.policy,
          "Cache-Control": "no-cache",
          "X-Content-Type-Options": "nosniff",
        ]
        let response = HTTPURLResponse(url: url, statusCode: status, httpVersion: "HTTP/1.1", headerFields: headers)!
        task.didReceive(response)
        task.didReceive(answer?.body ?? Data())
        task.didFinish()
      }
    }
  }

  func webView(_ webView: WKWebView, stop task: WKURLSchemeTask) {
    active.remove(ObjectIdentifier(task))
  }
}
