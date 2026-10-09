import AppKit
import WebKit

/// One window of the app. There are two kinds:
///
/// - the **main window**, exactly one, which holds every screen but a game
///   (home, the game picker, setup, the waiting room, My games, the account,
///   rules, settings) and has Back and Forward in its title bar. Closing it
///   hides it; the app, and any table this Mac hosts, keep running;
/// - a **game window**, one per open match or replay, which holds that game
///   and nothing else. Closing it closes the view only: the player stays
///   seated and the match is under In progress.
///
/// Every window shows the same signed-in player, the same core and the same
/// seat, because those belong to the app rather than to a window.
final class AppWindowController: NSWindowController, NSWindowDelegate, NSToolbarDelegate {
  enum Role { case main, game }

  let role: Role
  let web: WebController
  /// A game window's key: `match:<id>` or `replay:<id>`. Nil for the main window.
  private(set) var gameKey: String?
  /// What the page last said the Account menu should hold, in its language.
  private(set) var menuState: [String: Any]?
  /// What the game in front can show or hide, and whose rules; nil off a game.
  private(set) var viewState: [String: Any]?
  /// The game window's own words: title, status, whether it is finished,
  /// whether it is waiting on this player, and whether it is a table on this
  /// Mac's own server (offline) rather than a stored game.
  private(set) var info = Info()
  struct Info {
    var title = ""
    var subtitle = ""
    var finished = false
    var yourTurn = false
    var offline = false
  }

  var onActivate: ((AppWindowController) -> Void)?
  var onClose: ((AppWindowController) -> Void)?
  /// The page has new words for the menu bar, or a new account state.
  var onMenuChange: ((AppWindowController) -> Void)?
  /// The game window's title, subtitle or attention changed.
  var onInfoChange: ((AppWindowController) -> Void)?
  private var titleWatch: NSKeyValueObservation?
  private var navWatch: [NSKeyValueObservation] = []
  private var toolbarItems: [String: NSToolbarItem] = [:]
  /// Hidden, not closed: the main window comes back when asked for.
  private(set) var hidden = false

  init(
    role: Role, matchId: String?, kind: String, nearby: NearbyService, cascadeFrom previous: NSWindow?,
    seat: @escaping () -> [String: Any]?
  ) {
    self.role = role
    if let matchId { gameKey = "\(kind):\(matchId)" }
    web = WebController(
      nearby: nearby, spec: .init(role: role == .main ? .main : .game, matchId: matchId), seat: seat)
    let window = NSWindow(
      contentRect: NSRect(x: 0, y: 0, width: role == .main ? 1280 : 1100, height: role == .main ? 820 : 780),
      styleMask: [.titled, .closable, .miniaturizable, .resizable],
      backing: .buffered, defer: false)
    window.title = "Jokerless"
    window.minSize = NSSize(width: role == .main ? 720 : 640, height: 560)
    window.appearance = NSAppearance(named: .darkAqua)
    // The title bar takes the page's own header colour, so the two read as
    // one band; the page starts below it rather than under the window's
    // buttons and title.
    window.titlebarAppearsTransparent = true
    window.backgroundColor = NSColor(red: 0x1a / 255, green: 0x23 / 255, blue: 0x32 / 255, alpha: 1)
    window.titlebarSeparatorStyle = .none
    // One main window and a window per game, never tabs of one another.
    window.tabbingMode = .disallowed
    window.isReleasedWhenClosed = false
    window.contentView = web.view
    super.init(window: window)
    window.delegate = self

    if let previous {
      window.setFrame(previous.frame, display: false)
      window.setFrameTopLeftPoint(window.cascadeTopLeft(from: NSPoint(x: previous.frame.minX, y: previous.frame.maxY)))
    } else {
      window.center()
      if role == .main { window.setFrameAutosaveName("JokerlessMain") }
    }

    if role == .main { installToolbar(on: window) }

    // The window is named after the screen it shows: the page sets
    // document.title per screen, which also names the window in the Window
    // menu. A game window is named by the game itself (see `apply`).
    titleWatch = web.webView.observe(\.title, options: [.new]) { [weak self] view, _ in
      guard let self else { return }
      if self.role == .game, !self.info.title.isEmpty { return }
      let title = (view.title ?? "").trimmingCharacters(in: .whitespaces)
      self.window?.title = title.isEmpty ? "Jokerless" : title
    }
    web.onMenuState = { [weak self] state in
      guard let self else { return }
      self.menuState = state
      self.onMenuChange?(self)
    }
    web.onViewState = { [weak self] state in self?.viewState = state }
    navWatch = [
      web.webView.observe(\.canGoBack) { [weak self] _, _ in self?.refreshToolbar() },
      web.webView.observe(\.canGoForward) { [weak self] _, _ in self?.refreshToolbar() },
      web.webView.observe(\.url) { [weak self] _, _ in self?.refreshToolbar() },
    ]
  }

  required init?(coder: NSCoder) { fatalError("not used") }

  // MARK: game window state

  /// Takes what a game page says about its window. Returns true if its match
  /// changed (a rematch is the same window at a new game).
  @discardableResult
  func apply(info raw: [String: Any]) -> Bool {
    var changedKey = false
    if let id = raw["matchId"] as? String, !id.isEmpty {
      let kind = raw["kind"] as? String ?? gameKey?.split(separator: ":").first.map(String.init) ?? "match"
      let key = "\(kind):\(id)"
      if key != gameKey {
        gameKey = key
        web.setMatch(id)
        changedKey = true
      }
    }
    if let title = raw["title"] as? String {
      info.title = title
      info.subtitle = raw["subtitle"] as? String ?? ""
      info.finished = raw["finished"] as? Bool ?? false
      info.yourTurn = raw["yourTurn"] as? Bool ?? false
      info.offline = raw["offline"] as? Bool ?? info.offline
      renderTitle()
    }
    onInfoChange?(self)
    return changedKey
  }

  func refreshTitle() { renderTitle() }

  /// Waiting on this player, in a window that is not the one in front.
  var needsAttention: Bool { info.yourTurn && !(window?.isKeyWindow ?? false) }

  private func renderTitle() {
    guard let window, role == .game, !info.title.isEmpty else { return }
    window.title = (needsAttention ? "• " : "") + info.title
    window.subtitle = info.subtitle
  }

  // MARK: toolbar (main window): Back and Forward

  private func installToolbar(on window: NSWindow) {
    let toolbar = NSToolbar(identifier: "JokerlessMain")
    toolbar.delegate = self
    toolbar.displayMode = .iconOnly
    toolbar.allowsUserCustomization = false
    window.toolbar = toolbar
    window.toolbarStyle = .unified
  }

  func toolbarAllowedItemIdentifiers(_ toolbar: NSToolbar) -> [NSToolbarItem.Identifier] {
    [.init("back"), .init("forward"), .flexibleSpace]
  }

  func toolbarDefaultItemIdentifiers(_ toolbar: NSToolbar) -> [NSToolbarItem.Identifier] {
    [.init("back"), .init("forward"), .flexibleSpace]
  }

  func toolbar(
    _ toolbar: NSToolbar, itemForItemIdentifier id: NSToolbarItem.Identifier, willBeInsertedIntoToolbar: Bool
  ) -> NSToolbarItem? {
    guard id.rawValue == "back" || id.rawValue == "forward" else { return nil }
    let back = id.rawValue == "back"
    let item = NSToolbarItem(itemIdentifier: id)
    item.image = NSImage(systemSymbolName: back ? "chevron.left" : "chevron.right", accessibilityDescription: id.rawValue)
    item.label = back ? "Back" : "Forward"
    item.toolTip = item.label
    item.isBordered = true
    item.target = self
    item.action = back ? #selector(toolbarBack(_:)) : #selector(toolbarForward(_:))
    toolbarItems[id.rawValue] = item
    item.isEnabled = back ? web.canBack : web.canForward
    return item
  }

  /// The toolbar re-validates its items on every event; without this it would
  /// enable Back again at home.
  func validateToolbarItem(_ item: NSToolbarItem) -> Bool {
    switch item.itemIdentifier.rawValue {
    case "back": return web.canBack
    case "forward": return web.canForward
    default: return true
    }
  }

  @objc private func toolbarBack(_ sender: Any?) { web.goBack() }
  @objc private func toolbarForward(_ sender: Any?) { web.goForward() }

  /// Retitles the arrows in the player's language and enables each only
  /// where there is somewhere to go.
  func refreshToolbar(labels: [String: String]? = nil) {
    for (id, item) in toolbarItems {
      item.isEnabled = id == "back" ? web.canBack : web.canForward
      if let word = labels?[id] {
        item.label = word
        item.toolTip = word
      }
    }
  }

  /// The arrows as the title bar shows them, for tests.
  func toolbarState() -> [String: Bool] {
    var out: [String: Bool] = [:]
    for (id, item) in toolbarItems { out[id] = item.isEnabled }
    return out
  }

  func pressToolbar(_ id: String) -> Bool {
    guard let item = toolbarItems[id], item.isEnabled, let action = item.action else { return false }
    return NSApp.sendAction(action, to: item.target, from: item)
  }

  // MARK: window delegate

  func windowDidBecomeKey(_ notification: Notification) {
    renderTitle()
    onActivate?(self)
    onInfoChange?(self)
  }

  func windowDidBecomeMain(_ notification: Notification) { onActivate?(self) }

  /// The main window hides rather than closes: it is the app's one window
  /// that is always there (as Mail's viewer is), and the Dock icon or ⌘1
  /// brings it back.
  func windowShouldClose(_ sender: NSWindow) -> Bool {
    guard role == .main else { return true }
    hidden = true
    sender.orderOut(nil)
    onInfoChange?(self)
    return false
  }

  func show() {
    hidden = false
    if AppConfig.isE2E {
      window?.orderFront(nil)
    } else {
      showWindow(nil)
    }
  }

  func windowWillClose(_ notification: Notification) {
    titleWatch = nil
    navWatch = []
    web.detach()
    onClose?(self)
  }
}
