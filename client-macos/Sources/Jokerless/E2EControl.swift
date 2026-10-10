import AppKit
import WebKit

/// The end-to-end runner's hold on the app, in a debug build started with
/// ZOLIK_E2E_CONTROL=<dir>. There is no network in it, deliberately: the
/// runner drops numbered command files into the folder and the app answers
/// each with a result file.
///
///   cmd-<n>.js     JavaScript run in the page; its value (awaited) is the result
///   cmd-<n>.native one line: "menu <title>", "screenshot <path>", "state",
///                  "quit"
///   res-<n>.json   {"ok": true, "value": …} or {"ok": false, "error": "…"}
///   console.log    the page's console output and security policy violations
///
/// Inert in a release build: AppConfig.e2eControlDir is always nil there,
/// and the folder is never polled.
final class E2EControl {
  private let dir: URL
  private weak var app: AppDelegate?
  /// Commands run in the active window (`window select N` changes it).
  private var web: WebController? { app?.active?.web }
  private var logged = Set<ObjectIdentifier>()
  private var timer: Timer?
  private var busy = false
  private var next = 1
  private let log: FileHandle?

  init(dir: URL, app: AppDelegate) {
    self.dir = dir
    self.app = app
    try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
    let logURL = dir.appendingPathComponent("console.log")
    FileManager.default.createFile(atPath: logURL.path, contents: nil)
    log = try? FileHandle(forWritingTo: logURL)
    hookLogs()
  }

  /// Every window's console goes to console.log, prefixed with its number.
  private func hookLogs() {
    for (i, w) in (app?.windows ?? []).enumerated() where !logged.contains(ObjectIdentifier(w)) {
      logged.insert(ObjectIdentifier(w))
      let prefix = w.role == .main ? "" : "[game \(w.gameKey ?? String(i + 1))] "
      w.web.onLog = { [weak self] line in self?.append(prefix + line) }
    }
  }

  func start() {
    #if DEBUG
      append("app: started, server \(AppConfig.serverURL)")
      timer = Timer.scheduledTimer(withTimeInterval: 0.05, repeats: true) { [weak self] _ in self?.poll() }
    #endif
  }

  private func append(_ line: String) {
    log?.write((line.replacingOccurrences(of: "\n", with: "\\n") + "\n").data(using: .utf8)!)
  }

  private func poll() {
    guard !busy else { return }
    hookLogs()
    let js = dir.appendingPathComponent("cmd-\(next).js")
    let native = dir.appendingPathComponent("cmd-\(next).native")
    let n = next
    if let source = try? String(contentsOf: js, encoding: .utf8) {
      busy = true
      next += 1
      watchdog(n)
      runJS(source) { self.answer(n, $0) }
    } else if let line = try? String(contentsOf: native, encoding: .utf8) {
      busy = true
      next += 1
      watchdog(n)
      runNative(line.trimmingCharacters(in: .whitespacesAndNewlines)) { self.answer(n, $0) }
    }
  }

  private var answered = Set<Int>()

  private func answer(_ n: Int, _ result: Result<Any?, Error>) {
    // A command answers once: either itself, or the watchdog below.
    guard answered.insert(n).inserted else { return }
    var out: [String: Any]
    switch result {
    case .success(let value): out = ["ok": true, "value": value ?? NSNull()]
    case .failure(let error): out = ["ok": false, "error": "\(error)"]
    }
    if !JSONSerialization.isValidJSONObject(out) { out = ["ok": true, "value": "\(out["value"] ?? "")"] }
    let data = (try? JSONSerialization.data(withJSONObject: out)) ?? Data("{\"ok\":false}".utf8)
    let tmp = dir.appendingPathComponent("res-\(n).json.tmp")
    try? data.write(to: tmp)
    try? FileManager.default.moveItem(at: tmp, to: dir.appendingPathComponent("res-\(n).json"))
    busy = false
  }

  /// A command whose page went away mid-run never hears back from WebKit.
  /// It is failed after two minutes, so one lost command cannot stall a run.
  private func watchdog(_ n: Int) {
    DispatchQueue.main.asyncAfter(deadline: .now() + 120) {
      self.answer(n, .failure(E2EError("no answer in 120 s (did the page navigate mid-command?)")))
    }
  }

  private func runJS(
    _ source: String, waited: TimeInterval = 0, done: @escaping (Result<Any?, Error>) -> Void
  ) {
    guard let webView = web?.webView else { return done(.failure(E2EError("no web view"))) }
    // A command sent while a page is loading would run in the old page, or
    // in none; it waits for the new one instead.
    if webView.isLoading || webView.url?.scheme != WebController.scheme {
      guard waited < 30 else { return done(.failure(E2EError("the page did not finish loading"))) }
      DispatchQueue.main.asyncAfter(deadline: .now() + 0.1) {
        self.runJS(source, waited: waited + 0.1, done: done)
      }
      return
    }
    // The command is the body of an async function; whatever it returns is
    // passed through JSON so only plain data comes back.
    let body = "const __v = await (async () => {\n\(source)\n})(); return JSON.stringify(__v === undefined ? null : __v);"
    webView.callAsyncJavaScript(body, arguments: [:], in: nil, in: .page) { result in
      switch result {
      case .success(let raw):
        if let s = raw as? String, let d = s.data(using: .utf8),
          let v = try? JSONSerialization.jsonObject(with: d, options: [.fragmentsAllowed])
        {
          done(.success(v))
        } else {
          done(.success(raw))
        }
      case .failure(let error):
        let info = (error as NSError).userInfo
        let message = info["WKJavaScriptExceptionMessage"] as? String ?? error.localizedDescription
        done(.failure(E2EError(message)))
      }
    }
  }

  private func runNative(_ line: String, done: @escaping (Result<Any?, Error>) -> Void) {
    let parts = line.split(separator: " ", maxSplits: 1).map(String.init)
    let arg = parts.count > 1 ? parts[1] : ""
    switch parts.first ?? "" {
    case "menu":
      done(app?.performMenuItem(titled: arg) == true ? .success(true) : .failure(E2EError("no menu item \(arg)")))
    case "screenshot":
      guard let webView = web?.webView else { return done(.failure(E2EError("no web view"))) }
      webView.takeSnapshot(with: nil) { image, error in
        guard let image, let tiff = image.tiffRepresentation, let rep = NSBitmapImageRep(data: tiff),
          let png = rep.representation(using: .png, properties: [:])
        else { return done(.failure(error ?? E2EError("no snapshot"))) }
        do {
          try png.write(to: URL(fileURLWithPath: arg))
          done(.success(arg))
        } catch { done(.failure(error)) }
      }
    case "window":
      handleWindow(arg, done: done)
    case "windows":
      done(.success((app?.windows ?? []).map { $0.window?.title ?? "" }))
    case "windowsinfo":
      done(.success((app?.windows ?? []).map { w in
        [
          "role": w.role == .main ? "main" : "game", "key": w.gameKey ?? "", "title": w.window?.title ?? "",
          "subtitle": w.window?.subtitle ?? "", "visible": w.window?.isVisible ?? false,
          "hidden": w.hidden,
        ] as [String: Any]
      }))
    case "toolbar":
      guard let main = app?.main else { return done(.failure(E2EError("no main window"))) }
      switch arg {
      case "state": done(.success(main.toolbarState()))
      case "back", "forward":
        done(main.pressToolbar(arg) ? .success(true) : .failure(E2EError("the \(arg) arrow is off")))
      default: done(.failure(E2EError("toolbar state|back|forward")))
      }
    case "drag":
      // A real mouse drag, as the window's own events: "drag x1 y1 x2 y2" in
      // the page's pixels. Goes through AppKit and WebKit's input path, not
      // a script's synthetic events.
      let n = arg.split(separator: " ").compactMap { Double($0) }
      guard n.count == 4, let c = app?.active, let window = c.window else {
        return done(.failure(E2EError("drag x1 y1 x2 y2")))
      }
      MouseDrag(window: window, view: c.web.webView, from: CGPoint(x: n[0], y: n[1]), to: CGPoint(x: n[2], y: n[3]))
        .run { done(.success(true)) }
    case "radiodrop":
      // The radio drops every loopback link the core holds.
      NearbyService.shared.dropLoopbackLinks()
      done(.success(true))
    case "badge":
      done(.success(NSApp.dockTile.badgeLabel ?? ""))
    case "dock":
      // A click on the Dock icon.
      done(.success(app?.applicationShouldHandleReopen(NSApp, hasVisibleWindows: false) ?? false))
    case "seat":
      done(.success(app?.seat ?? [:]))
    case "account":
      done(.success(app?.accountMenuTitles() ?? []))
    case "about":
      let o = app?.aboutOptions() ?? [:]
      done(.success([
        "version": o[.applicationVersion] as? String ?? "",
        "build": o[.version] as? String ?? "",
        "credits": (o[.credits] as? NSAttributedString)?.string ?? "",
      ]))
    case "menuitems":
      done(.success(app?.menuTitles(arg) ?? []))
    case "state":
      done(.success(web?.nearby.snapshot()))
    case "quit":
      done(.success(true))
      DispatchQueue.main.asyncAfter(deadline: .now() + 0.2) { NSApp.terminate(nil) }
    default:
      done(.failure(E2EError("unknown native command \(line)")))
    }
  }
}

extension E2EControl {
  /// `window` describes the active window; `window select N` (1-based; the
  /// main window is 1, then the game windows in the order they opened)
  /// switches to one; `window main` selects the main window, showing it if
  /// it was hidden; `window close N` closes one the way its close button
  /// does (the main window hides).
  fileprivate func handleWindow(_ arg: String, done: @escaping (Result<Any?, Error>) -> Void) {
    guard let app else { return done(.failure(E2EError("no app"))) }
    let parts = arg.split(separator: " ").map(String.init)
    switch parts.first ?? "" {
    case "select", "close":
      guard parts.count > 1, let n = Int(parts[1]), n >= 1, n <= app.windows.count else {
        return done(.failure(E2EError("no window \(arg)")))
      }
      let target = app.windows[n - 1]
      if parts[0] == "close" {
        target.window?.performClose(nil)
        return done(.success(app.windows.count))
      }
      app.select(target)
      done(.success(n))
    case "main":
      app.showMain()
      done(.success(true))
    default:
      guard let c = app.active, let w = c.window else { return done(.failure(E2EError("no window"))) }
      let content = w.contentLayoutRect
      let view = c.web.webView.frame
      done(.success([
        "title": w.title, "subtitle": w.subtitle, "visible": w.isVisible, "count": app.windows.count,
        "role": c.role == .main ? "main" : "game", "key": c.gameKey ?? "", "hidden": c.hidden,
        "width": w.frame.width, "height": w.frame.height,
        // Where the page sits: entirely below the title bar when its top
        // edge is no higher than the content area's.
        "pageTop": view.maxY, "contentTop": content.maxY,
        "zoom": c.web.webView.pageZoom,
        "tabs": w.tabbedWindows?.count ?? 0,
      ]))
    }
  }
}

struct E2EError: Error, CustomStringConvertible {
  let description: String
  init(_ d: String) { description = d }
}


/// Presses, drags and releases the left mouse button in a window with real
/// AppKit events, spaced out the way a hand does it.
final class MouseDrag {
  private let window: NSWindow
  private let view: NSView
  private let from: CGPoint
  private let to: CGPoint
  private var keep: MouseDrag?

  init(window: NSWindow, view: NSView, from: CGPoint, to: CGPoint) {
    self.window = window
    self.view = view
    self.from = from
    self.to = to
  }

  private func event(_ type: NSEvent.EventType, _ p: CGPoint) -> NSEvent? {
    // The page's pixels are the web view's own (it is flipped).
    let inView = view.isFlipped ? p : CGPoint(x: p.x, y: view.bounds.height - p.y)
    let inWindow = view.convert(inView, to: nil)
    return NSEvent.mouseEvent(
      with: type, location: inWindow, modifierFlags: [], timestamp: ProcessInfo.processInfo.systemUptime,
      windowNumber: window.windowNumber, context: nil, eventNumber: 0, clickCount: 1,
      pressure: type == .leftMouseUp ? 0 : 1)
  }

  func run(done: @escaping () -> Void) {
    keep = self
    // Real events reach a page only through a window that is in front and
    // key, as for a person; the test takes the keyboard for the drag.
    NSApp.activate(ignoringOtherApps: true)
    window.makeKeyAndOrderFront(nil)
    window.makeFirstResponder(view)
    // A person's pointer arrives first and rests on the card (a tooltip comes
    // up, the card lifts) before the button goes down.
    let rest = DispatchTime.now()
    if let hover = event(.mouseMoved, CGPoint(x: from.x - 30, y: from.y + 20)) { NSApp.postEvent(hover, atStart: false) }
    if let hover = event(.mouseMoved, from) { NSApp.postEvent(hover, atStart: false) }
    Thread.sleep(forTimeInterval: 1.2)
    _ = rest
    if let down = event(.leftMouseDown, from) { NSApp.postEvent(down, atStart: false) }
    var step = 0
    let steps = 24
    // A pause after the press, then steady movement: the drag gesture needs
    // to see time pass.
    Timer.scheduledTimer(withTimeInterval: 0.2, repeats: false) { _ in
      Timer.scheduledTimer(withTimeInterval: 0.02, repeats: true) { timer in
        step += 1
        let t = Double(step) / Double(steps)
        let p = CGPoint(x: self.from.x + (self.to.x - self.from.x) * t, y: self.from.y + (self.to.y - self.from.y) * t)
        if let drag = self.event(.leftMouseDragged, p) { NSApp.postEvent(drag, atStart: false) }
        if step >= steps {
          timer.invalidate()
          Timer.scheduledTimer(withTimeInterval: 0.25, repeats: false) { _ in
            if let up = self.event(.leftMouseUp, self.to) { NSApp.postEvent(up, atStart: false) }
            Timer.scheduledTimer(withTimeInterval: 0.5, repeats: false) { _ in
              done()
              self.keep = nil
            }
          }
        }
      }
    }
  }
}
