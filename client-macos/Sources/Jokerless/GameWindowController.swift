import AppKit
import WebKit

/// One window, one web view: a game, a lobby, anything the app shows. The app
/// can have several open at once (File › New Window), each at its own table;
/// they share the signed-in player, the offline table this Mac hosts and the
/// core, because those belong to the app rather than to a window.
final class GameWindowController: NSWindowController, NSWindowDelegate {
  let web: WebController
  /// What the page last said the Account menu should hold, in its language.
  private(set) var menuState: [String: Any]?
  var onActivate: ((GameWindowController) -> Void)?
  var onClose: ((GameWindowController) -> Void)?
  private var titleWatch: NSKeyValueObservation?

  init(nearby: NearbyService, cascadeFrom previous: NSWindow?) {
    web = WebController(nearby: nearby)
    let window = NSWindow(
      contentRect: NSRect(x: 0, y: 0, width: 1280, height: 820),
      styleMask: [.titled, .closable, .miniaturizable, .resizable],
      backing: .buffered, defer: false)
    window.title = "Jokerless"
    window.minSize = NSSize(width: 720, height: 560)
    window.appearance = NSAppearance(named: .darkAqua)
    // The title bar takes the page's own header colour, so the two read as
    // one band; the page starts below it rather than under the window's
    // buttons and title.
    window.titlebarAppearsTransparent = true
    window.backgroundColor = NSColor(red: 0x1a / 255, green: 0x23 / 255, blue: 0x32 / 255, alpha: 1)
    window.titlebarSeparatorStyle = .none
    window.tabbingMode = .preferred
    window.isReleasedWhenClosed = false
    window.contentView = web.view
    super.init(window: window)
    window.delegate = self

    if let previous {
      window.setFrame(previous.frame, display: false)
      window.setFrameTopLeftPoint(window.cascadeTopLeft(from: NSPoint(x: previous.frame.minX, y: previous.frame.maxY)))
    } else {
      window.center()
      window.setFrameAutosaveName("JokerlessMain")
    }

    // The window is named after the screen it shows: the page sets
    // document.title per screen ("Match – Jokerless"), which also names the
    // window in the Window menu, so two tables are told apart there.
    titleWatch = web.webView.observe(\.title, options: [.new]) { [weak window] view, _ in
      let title = (view.title ?? "").trimmingCharacters(in: .whitespaces)
      window?.title = title.isEmpty ? "Jokerless" : title
    }
    web.onMenuState = { [weak self] state in
      guard let self else { return }
      self.menuState = state
    }
  }

  required init?(coder: NSCoder) { fatalError("not used") }

  func windowDidBecomeKey(_ notification: Notification) { onActivate?(self) }
  func windowDidBecomeMain(_ notification: Notification) { onActivate?(self) }

  func windowWillClose(_ notification: Notification) {
    titleWatch = nil
    web.detach()
    onClose?(self)
  }
}
