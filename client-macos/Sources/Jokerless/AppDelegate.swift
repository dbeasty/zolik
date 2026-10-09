import AppKit
import WebKit
import Zolikcore

final class AppDelegate: NSObject, NSApplicationDelegate, NSMenuDelegate, NSMenuItemValidation {
  private(set) var windows: [GameWindowController] = []
  /// The window menu commands act on: the frontmost one, or the one an
  /// end-to-end run selected.
  private(set) var active: GameWindowController?
  private var e2e: E2EControl?
  private var activity: NSObjectProtocol?
  /// The menu bar's Account menu, rebuilt from the active page's state
  /// whenever it is about to open.
  let accountMenu = NSMenu(title: "Account")
  /// View and Help › Rules follow the game in front, so they are filled as
  /// they open too.
  let viewMenu = NSMenu(title: "View")
  let rulesMenu = NSMenu(title: "Rules")

  func applicationDidFinishLaunching(_ notification: Notification) {
    AppConfig.configureCore()
    accountMenu.delegate = self
    viewMenu.delegate = self
    rulesMenu.delegate = self
    NSApp.mainMenu = MenuBuilder.build(target: self, account: accountMenu, view: viewMenu, rules: rulesMenu)
    let first = openWindow()

    if let dir = AppConfig.e2eControlDir {
      // A test run must not be slowed by App Nap while its window sits
      // behind whatever the person at the Mac is doing.
      activity = ProcessInfo.processInfo.beginActivity(
        options: [.userInitiated, .idleSystemSleepDisabled], reason: "end-to-end run")
      e2e = E2EControl(dir: dir, app: self)
      e2e?.start()
    } else {
      NSApp.activate(ignoringOtherApps: true)
    }
    first.web.load()
  }

  /// A new window at the home screen. Every window shares the player, the
  /// core and any offline table this Mac hosts.
  @discardableResult
  func openWindow() -> GameWindowController {
    let controller = GameWindowController(
      nearby: NearbyService.shared, cascadeFrom: active?.window ?? windows.last?.window)
    controller.onActivate = { [weak self] c in self?.active = c }
    controller.onClose = { [weak self] c in
      guard let self else { return }
      self.windows.removeAll { $0 === c }
      if self.active === c { self.active = self.windows.last }
    }
    windows.append(controller)
    active = controller
    if AppConfig.isE2E {
      controller.window?.orderFront(nil)
    } else {
      controller.showWindow(nil)
    }
    return controller
  }

  func select(_ controller: GameWindowController) {
    active = controller
    controller.window?.orderFront(nil)
  }

  func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool { true }

  func applicationWillTerminate(_ notification: Notification) {
    // The table lives in this process; quitting ends it for everyone, so it
    // is closed properly rather than dropped.
    NearbyService.shared.shutdown()
  }

  private var web: WebController? { active?.web }

  // MARK: menu actions

  @objc func newWindow(_ sender: Any?) {
    openWindow().web.load()
  }

  @objc func startOfflineTable(_ sender: Any?) { web?.navigate(to: "/offline") }
  @objc func backOnline(_ sender: Any?) { web?.navigate(to: "/") }
  @objc func goHome(_ sender: Any?) { web?.navigate(to: "/") }
  /// Back and Forward walk the window's own history, which holds every
  /// screen the page moved to, so one always undoes the other. With nothing
  /// behind, Back goes home (the page's own rule), and Forward can then
  /// return to where it left.
  @objc func goBack(_ sender: Any?) {
    guard let view = web?.webView else { return }
    if view.canGoBack { view.goBack() } else { web?.command("back") }
  }
  @objc func goForward(_ sender: Any?) { web?.webView.goForward() }
  @objc func showStats(_ sender: Any?) { web?.navigate(to: "/stats") }
  @objc func showSettings(_ sender: Any?) { web?.navigate(to: "/settings") }
  @objc func reloadPage(_ sender: Any?) { web?.webView.reload() }
  @objc func zoomIn(_ sender: Any?) { setZoom((web?.webView.pageZoom ?? 1) * 1.1) }
  @objc func zoomOut(_ sender: Any?) { setZoom((web?.webView.pageZoom ?? 1) / 1.1) }
  @objc func actualSize(_ sender: Any?) { setZoom(1) }
  @objc func openWebsite(_ sender: Any?) {
    if let url = URL(string: AppConfig.serverURL) { NSWorkspace.shared.open(url) }
  }

  private func setZoom(_ z: CGFloat) {
    web?.webView.pageZoom = min(2.5, max(0.5, z))
  }

  /// About, with this app's and the server's versions as the page words them.
  @objc func showAbout(_ sender: Any?) {
    var options: [NSApplication.AboutPanelOptionKey: Any] = [:]
    if let versions = active?.menuState?["versions"] as? String {
      options[.credits] = NSAttributedString(
        string: versions,
        attributes: [.font: NSFont.systemFont(ofSize: 11), .foregroundColor: NSColor.secondaryLabelColor])
    }
    NSApp.orderFrontStandardAboutPanel(options: options)
    NSApp.activate(ignoringOtherApps: true)
  }

  @objc func showMore(_ sender: Any?) {
    let path = (active?.menuState?["more"] as? [String: Any])?["path"] as? String ?? "/more"
    web?.navigate(to: path)
  }

  @objc private func accountItem(_ sender: NSMenuItem) {
    guard let item = sender.representedObject as? [String: Any] else { return }
    if let path = item["path"] as? String {
      web?.navigate(to: path)
    } else if let command = item["command"] as? String {
      web?.command(command)
    }
  }

  // MARK: View and Help › Rules

  /// Show/Hide Hand, Table or Log in the game in front.
  @objc func toggleViewPart(_ sender: NSMenuItem) {
    guard let part = MenuBuilder.viewTags[sender.tag] else { return }
    web?.toggleView(part)
  }

  @objc private func openRules(_ sender: NSMenuItem) {
    if let path = sender.representedObject as? String { web?.navigate(to: path) }
  }

  /// Whether a View part is showing in the game in front: true, false, or nil
  /// when there is no game in front or this game has no such part.
  private func viewPart(_ part: String) -> Bool? {
    active?.viewState?[part] as? Bool
  }

  func validateMenuItem(_ item: NSMenuItem) -> Bool {
    if item.action == #selector(goForward(_:)) { return web?.webView.canGoForward ?? false }
    if item.action == #selector(toggleViewPart(_:)), let part = MenuBuilder.viewTags[item.tag] {
      return viewPart(part) != nil
    }
    return true
  }

  private func refreshViewMenu() {
    for item in viewMenu.items {
      guard let part = MenuBuilder.viewTags[item.tag], let name = MenuBuilder.viewNames[part] else { continue }
      item.title = viewPart(part) == false ? "Show \(name)" : "Hide \(name)"
    }
  }

  /// The rules of the game in front first, then every game by name.
  private func refreshRulesMenu() {
    rulesMenu.removeAllItems()
    let games = active?.menuState?["rules"] as? [[String: Any]] ?? []
    var current: String?
    if let target = active?.viewState?["rules"] as? [String: Any], let id = target["moduleId"] as? String {
      var q = URLComponents()
      q.queryItems = [URLQueryItem(name: "moduleId", value: id)]
      if let v = target["variation"] as? String, !v.isEmpty { q.queryItems?.append(URLQueryItem(name: "variation", value: v)) }
      if let o = target["options"], let d = try? JSONSerialization.data(withJSONObject: o), let s = String(data: d, encoding: .utf8) {
        q.queryItems?.append(URLQueryItem(name: "options", value: s))
      }
      current = id
      let label = games.first { ($0["path"] as? String)?.hasSuffix("moduleId=\(id)") == true }?["label"] as? String ?? id
      let item = NSMenuItem(title: label, action: #selector(openRules(_:)), keyEquivalent: "/")
      item.keyEquivalentModifierMask = [.command, .option]
      item.target = self
      item.representedObject = "/rules?" + (q.percentEncodedQuery ?? "moduleId=\(id)")
      item.state = .on
      rulesMenu.addItem(item)
      rulesMenu.addItem(.separator())
    }
    for game in games {
      guard let label = game["label"] as? String, let path = game["path"] as? String else { continue }
      if let current, path.hasSuffix("moduleId=\(current)") { continue }
      let item = NSMenuItem(title: label, action: #selector(openRules(_:)), keyEquivalent: "")
      item.target = self
      item.representedObject = path
      rulesMenu.addItem(item)
    }
    if rulesMenu.items.isEmpty {
      let none = NSMenuItem(title: "Rules", action: nil, keyEquivalent: "")
      none.isEnabled = false
      rulesMenu.addItem(none)
    }
  }

  /// A menu's items as it would show them now, for tests.
  func menuTitles(_ which: String) -> [String] {
    let menu: NSMenu
    switch which {
    case "View": refreshViewMenu(); menu = viewMenu
    case "Rules": refreshRulesMenu(); menu = rulesMenu
    default: menuNeedsUpdate(accountMenu); menu = accountMenu
    }
    return menu.items.map { item in
      if item.isSeparatorItem { return "—" }
      let enabled = item.action == nil ? item.isEnabled : validateMenuItem(item)
      return (enabled ? "" : "(disabled) ") + (item.state == .on ? "✓ " : "") + item.title
    }
  }

  // MARK: Account menu

  /// The Account menu holds what the account menu behind the face holds on
  /// the phones, in the player's language, for the frontmost window's page.
  func menuNeedsUpdate(_ menu: NSMenu) {
    if menu === viewMenu { return refreshViewMenu() }
    if menu === rulesMenu { return refreshRulesMenu() }
    guard menu === accountMenu else { return }
    menu.removeAllItems()
    let state = active?.menuState ?? [:]
    let who = NSMenuItem(title: state["who"] as? String ?? "Jokerless", action: nil, keyEquivalent: "")
    who.isEnabled = false
    menu.addItem(who)
    menu.addItem(.separator())
    for entry in state["account"] as? [[String: Any]] ?? [] {
      if entry["separator"] as? Bool == true {
        if menu.items.last?.isSeparatorItem == false { menu.addItem(.separator()) }
        continue
      }
      let item = NSMenuItem(
        title: entry["label"] as? String ?? "", action: #selector(accountItem(_:)), keyEquivalent: "")
      item.target = self
      item.representedObject = entry
      menu.addItem(item)
    }
  }

  /// The Account menu's items as the menu would show them now, for tests.
  func accountMenuTitles() -> [String] {
    menuNeedsUpdate(accountMenu)
    return accountMenu.items.map { $0.isSeparatorItem ? "—" : $0.title }
  }

  /// Menu items by title, for the end-to-end runner.
  func performMenuItem(titled title: String) -> Bool {
    menuNeedsUpdate(accountMenu)
    refreshViewMenu()
    refreshRulesMenu()
    func find(_ menu: NSMenu) -> NSMenuItem? {
      for item in menu.items {
        if item.title == title { return item }
        if let sub = item.submenu, let hit = find(sub) { return hit }
      }
      return nil
    }
    guard let menu = NSApp.mainMenu, let item = find(menu), let action = item.action else { return false }
    if !validateMenuItem(item) { return false }
    return NSApp.sendAction(action, to: item.target, from: item)
  }
}
