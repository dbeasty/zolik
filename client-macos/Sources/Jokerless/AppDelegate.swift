import AppKit
import WebKit
import Zolikcore
import UserNotifications

final class AppDelegate: NSObject, NSApplicationDelegate, NSMenuDelegate, NSMenuItemValidation, UNUserNotificationCenterDelegate {
  /// The one main window: home, the lobby, the account, rules. Always there;
  /// closing it hides it.
  private(set) var main: AppWindowController!
  /// One window per open match or replay, in the order they were opened.
  private(set) var games: [AppWindowController] = []
  /// Every window, the main one first.
  var windows: [AppWindowController] { [main].compactMap { $0 } + games }
  /// The window menu commands act on: the frontmost one, or the one an
  /// end-to-end run selected.
  private(set) var active: AppWindowController?
  private var e2e: E2EControl?
  private var activity: NSObjectProtocol?
  private var terminating = false
  /// Which offline table the player sits at, as the page described it. The
  /// app holds it so every window, whenever it opens, finds the player seated.
  private(set) var seat: [String: Any]?
  private var invites = (count: 0, newest: "")
  private let remembered = OpenGames()
  /// The menu bar's Account menu, rebuilt from the active page's state
  /// whenever it is about to open.
  let accountMenu = NSMenu(title: "Account")
  /// View and Help › Rules follow the game in front, so they are filled as
  /// they open too.
  let viewMenu = NSMenu(title: "View")
  let rulesMenu = NSMenu(title: "Rules")
  let helpMenu = NSMenu(title: "Help")

  func applicationDidFinishLaunching(_ notification: Notification) {
    AppConfig.configureCore()
    accountMenu.delegate = self
    viewMenu.delegate = self
    rulesMenu.delegate = self
    helpMenu.delegate = self
    NSApp.mainMenu = MenuBuilder.build(
      target: self, account: accountMenu, view: viewMenu, rules: rulesMenu, help: helpMenu)
    let first = makeController(role: .main, matchId: nil, kind: "match", path: "/")
    main = first
    active = first
    first.show()

    if let dir = AppConfig.e2eControlDir {
      // A test run must not be slowed by App Nap while its window sits
      // behind whatever the person at the Mac is doing.
      activity = ProcessInfo.processInfo.beginActivity(
        options: [.userInitiated, .idleSystemSleepDisabled], reason: "end-to-end run")
      e2e = E2EControl(dir: dir, app: self)
      e2e?.start()
    } else {
      NSApp.activate(ignoringOtherApps: true)
      UNUserNotificationCenter.current().delegate = self
    }
    first.web.load()
    // The games that were open when the app last quit come back with it.
    for entry in remembered.entries {
      openGame(key: entry.key, path: entry.path, activate: false)
    }
  }

  /// A window with its page wired to the app. Every window shares the player,
  /// the core and the seat.
  private func makeController(role: AppWindowController.Role, matchId: String?, kind: String, path: String)
    -> AppWindowController
  {
    let controller = AppWindowController(
      role: role, matchId: matchId, kind: kind, nearby: NearbyService.shared,
      cascadeFrom: active?.window ?? windows.last?.window, seat: { [weak self] in self?.seat })
    controller.web.openGames = { [weak self] in self?.openMatchIds ?? [] }
    controller.onActivate = { [weak self] c in
      self?.active = c
      self?.applyLabels()
      self?.updateBadge()
    }
    controller.onMenuChange = { [weak self] c in
      if c === self?.main { self?.applyLabels() }
    }
    controller.onInfoChange = { [weak self] _ in self?.updateBadge() }
    controller.onClose = { [weak self] c in self?.windowClosed(c) }
    controller.web.onAppOp = { [weak self, weak controller] op, body in
      guard let self, let controller else { return }
      self.appOp(op, body, from: controller)
    }
    return controller
  }

  /// The matches that have a window open: what the lists of games show as
  /// playing rather than as something to resume.
  var openMatchIds: [String] {
    games.compactMap { g in
      guard let key = g.gameKey, key.hasPrefix("match:") else { return nil }
      return String(key.dropFirst("match:".count))
    }
  }

  private func broadcastGames() {
    let ids = openMatchIds
    for w in windows { w.web.emitGames(ids) }
  }

  private func windowClosed(_ c: AppWindowController) {
    games.removeAll { $0 === c }
    defer { broadcastGames() }
    if active === c { active = games.last ?? main }
    // Closing a game's window is leaving its view, not the match; but it is
    // no longer one to bring back at the next launch.
    if !terminating, let key = c.gameKey { remembered.remove(key) }
    updateBadge()
  }

  // MARK: game windows

  /// Opens a game's window, or brings the one already showing that game
  /// forward: a second window on one match would displace the first's match
  /// socket.
  @discardableResult
  func openGame(key: String, path: String, activate: Bool = true) -> AppWindowController {
    if let existing = games.first(where: { $0.gameKey == key }) {
      if activate { raise(existing) }
      return existing
    }
    let parts = key.split(separator: ":", maxSplits: 1).map(String.init)
    let kind = parts.first ?? "match"
    let id = parts.count > 1 ? parts[1] : ""
    let c = makeController(role: .game, matchId: id, kind: kind, path: path)
    games.append(c)
    broadcastGames()
    remembered.add(key: key, path: path)
    c.web.load(path: path)
    if activate { raise(c) } else { c.show() }
    return c
  }

  private func raise(_ c: AppWindowController) {
    c.show()
    // A test run must not take the keyboard from the person at the Mac.
    if !AppConfig.isE2E {
      c.window?.makeKeyAndOrderFront(nil)
      NSApp.activate(ignoringOtherApps: true)
    }
    active = c
    applyLabels()
  }

  func showMain() {
    main.show()
    if AppConfig.isE2E {
      active = main
      applyLabels()
    } else {
      raise(main)
    }
  }

  /// What the pages ask of the app: see src/desktop/windows.tsx and seat.ts.
  private func appOp(_ op: String, _ body: [String: Any], from c: AppWindowController) {
    switch op {
    case "openGame":
      guard let id = body["matchId"] as? String, let path = body["path"] as? String else { return }
      openGame(key: "\(body["kind"] as? String ?? "match"):\(id)", path: path)
    case "toMain":
      // A game window asked for a screen that is not a game: the main window
      // shows it, and the game window closes if the player is leaving.
      showMain()
      if let path = body["path"] as? String { main.web.navigate(to: path) }
      if body["close"] as? Bool == true, c.role == .game { c.window?.close() }
    case "windowInfo":
      guard c.role == .game else { return }
      let before = c.gameKey
      c.apply(info: body)
      guard let key = c.gameKey else { return }
      if before != key { broadcastGames() }
      if let before, before != key { remembered.remove(before) }
      // A finished game, or a table on this Mac's own server, is not one to
      // bring back at the next launch.
      if c.info.finished || c.info.offline {
        remembered.remove(key)
      } else {
        let parts = key.split(separator: ":", maxSplits: 1).map(String.init)
        remembered.add(key: key, path: "/\(parts[0])/\(parts.count > 1 ? parts[1] : "")")
      }
    case "seat":
      seat = body["table"] as? [String: Any]
      for other in windows where other !== c { other.web.emitSeat(seat) }
    case "signedOut":
      for g in games { g.window?.close() }
      remembered.clear()
    case "invites":
      let count = body["count"] as? Int ?? 0
      let newest = body["newestId"] as? String ?? ""
      let isNew = !newest.isEmpty && newest != invites.newest
      invites = (count, newest)
      updateBadge()
      if isNew, !(main.window?.isKeyWindow ?? false) { notify(body["text"] as? String ?? "") }
    default:
      break
    }
  }

  // MARK: Dock badge and notifications

  /// The invites waiting while the main window is not in front, plus the
  /// games waiting on this player in windows that are not.
  var badgeCount: Int {
    let invitesWaiting = (main?.window?.isKeyWindow ?? false) ? 0 : invites.count
    return invitesWaiting + games.filter { $0.needsAttention }.count
  }

  func updateBadge() {
    let n = badgeCount
    NSApp.dockTile.badgeLabel = n > 0 ? String(n) : nil
    for g in games { g.refreshTitle() }
  }

  private func notify(_ text: String) {
    guard !AppConfig.isE2E, !text.isEmpty else { return }
    let center = UNUserNotificationCenter.current()
    center.requestAuthorization(options: [.alert]) { granted, _ in
      guard granted else { return }
      let content = UNMutableNotificationContent()
      content.title = "Jokerless"
      content.body = text
      center.add(UNNotificationRequest(identifier: UUID().uuidString, content: content, trigger: nil))
    }
  }

  /// Clicking a notification brings the main window forward, where the
  /// invite is.
  func userNotificationCenter(
    _ center: UNUserNotificationCenter, didReceive response: UNNotificationResponse,
    withCompletionHandler completionHandler: @escaping () -> Void
  ) {
    showMain()
    completionHandler()
  }

  // MARK: application

  /// The menu bar in the player's language: the words the main window's page
  /// gave, since both kinds of window speak the same one.
  private func applyLabels() {
    guard let labels = (main?.menuState ?? active?.menuState)?["labels"] as? [String: String], let bar = NSApp.mainMenu
    else { return }
    MenuBuilder.apply(labels, to: bar)
    main?.refreshToolbar(labels: labels)
    refreshViewMenu()
  }

  private func label(_ key: String, _ english: String) -> String {
    ((main?.menuState ?? active?.menuState)?["labels"] as? [String: String])?[key] ?? english
  }

  func select(_ controller: AppWindowController) {
    active = controller
    applyLabels()
    controller.window?.orderFront(nil)
  }

  /// Closing every window leaves the app, and the table this Mac hosts,
  /// running; quitting ends them.
  func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool { false }

  /// Clicking the Dock icon brings the main window back.
  func applicationShouldHandleReopen(_ sender: NSApplication, hasVisibleWindows flag: Bool) -> Bool {
    if !flag || main?.hidden == true { showMain() }
    return true
  }

  func applicationShouldTerminate(_ sender: NSApplication) -> NSApplication.TerminateReply {
    terminating = true
    return .terminateNow
  }

  func applicationWillTerminate(_ notification: Notification) {
    // The table lives in this process; quitting ends it for everyone, so it
    // is closed properly rather than dropped.
    ZolikcoreLeaveGuestTables()
    NearbyService.shared.shutdown()
  }

  /// Menu commands about the player's place in the app (Account, Table,
  /// Back, Forward, Home) act on the main window, wherever the focus is.
  private var web: WebController? { main?.web }
  /// Commands about a game (View parts) act on the game window in front.
  private var gameWeb: WebController? { active?.role == .game ? active?.web : nil }
  /// What zoom and Reload act on: the window in front.
  private var frontWeb: WebController? { active?.web }

  // MARK: menu actions

  /// File › New Game: the main window, at the game picker (the home screen).
  @objc func newGame(_ sender: Any?) {
    showMain()
    main.web.navigate(to: "/")
  }

  @objc func showMainWindow(_ sender: Any?) { showMain() }

  @objc func startOfflineTable(_ sender: Any?) { showMain(); web?.navigate(to: "/offline") }
  /// Back to online play: lets go of the table the player sits at (a host
  /// closes it; a guest leaves), in every window, and shows the home screen.
  @objc func backOnline(_ sender: Any?) {
    showMain()
    web?.command("leaveOffline")
    web?.navigate(to: "/")
  }
  @objc func goHome(_ sender: Any?) { showMain(); web?.navigate(to: "/") }
  /// Back and Forward walk the main window's own history, which holds every
  /// screen the page moved to, so one always undoes the other. With nothing
  /// behind, Back goes home (the page's own rule), and Forward can then
  /// return to where it left. A game window has neither: it is one game.
  @objc func goBack(_ sender: Any?) { web?.goBack() }
  @objc func goForward(_ sender: Any?) { web?.goForward() }
  @objc func showStats(_ sender: Any?) { showMain(); web?.navigate(to: "/stats") }
  @objc func showSettings(_ sender: Any?) { showMain(); web?.navigate(to: "/settings") }
  @objc func reloadPage(_ sender: Any?) { frontWeb?.webView.reload() }
  @objc func zoomIn(_ sender: Any?) { setZoom((frontWeb?.webView.pageZoom ?? 1) * 1.1) }
  @objc func zoomOut(_ sender: Any?) { setZoom((frontWeb?.webView.pageZoom ?? 1) / 1.1) }
  @objc func actualSize(_ sender: Any?) { setZoom(1) }
  @objc func openWebsite(_ sender: Any?) {
    if let url = URL(string: AppConfig.serverURL) { NSWorkspace.shared.open(url) }
  }

  private func setZoom(_ z: CGFloat) {
    frontWeb?.webView.pageZoom = min(2.5, max(0.5, z))
  }

  /// The standard About panel: this build's version, with its commit where
  /// macOS puts the build number, and the server's version and commit
  /// underneath, as the footer shows them on the phones.
  func aboutOptions() -> [NSApplication.AboutPanelOptionKey: Any] {
    var options: [NSApplication.AboutPanelOptionKey: Any] = [:]
    guard let about = main?.menuState?["about"] as? [String: Any] else { return options }
    if let v = about["version"] as? String { options[.applicationVersion] = v }
    if let c = about["commit"] as? String { options[.version] = c }
    if let server = about["server"] as? String {
      let style = NSMutableParagraphStyle()
      style.alignment = .center
      options[.credits] = NSAttributedString(
        string: server,
        attributes: [
          .font: NSFont.systemFont(ofSize: 11), .foregroundColor: NSColor.secondaryLabelColor, .paragraphStyle: style,
        ])
    }
    return options
  }

  @objc func showAbout(_ sender: Any?) {
    NSApp.orderFrontStandardAboutPanel(options: aboutOptions())
    NSApp.activate(ignoringOtherApps: true)
  }

  @objc private func openLegal(_ sender: NSMenuItem) {
    guard let entry = sender.representedObject as? [String: Any] else { return }
    if let path = entry["path"] as? String {
      showMain()
      web?.navigate(to: path)
    } else if let raw = entry["url"] as? String, let url = URL(string: raw) {
      WebController.openExternally(url)
    }
  }

  /// Help's notices, after its own items: Terms, Privacy, Accessibility, Source.
  private func refreshHelpMenu() {
    for item in helpMenu.items where item.tag == MenuBuilder.legalTag { helpMenu.removeItem(item) }
    let legal = main?.menuState?["legal"] as? [[String: Any]] ?? []
    guard !legal.isEmpty else { return }
    let rule = NSMenuItem.separator()
    rule.tag = MenuBuilder.legalTag
    helpMenu.addItem(rule)
    for entry in legal {
      let item = NSMenuItem(title: entry["label"] as? String ?? "", action: #selector(openLegal(_:)), keyEquivalent: "")
      item.target = self
      item.tag = MenuBuilder.legalTag
      item.representedObject = entry
      helpMenu.addItem(item)
    }
  }

  @objc func showMore(_ sender: Any?) {
    let path = (main?.menuState?["more"] as? [String: Any])?["path"] as? String ?? "/more"
    showMain()
    web?.navigate(to: path)
  }

  @objc private func accountItem(_ sender: NSMenuItem) {
    guard let item = sender.representedObject as? [String: Any] else { return }
    if let path = item["path"] as? String {
      showMain()
      web?.navigate(to: path)
    } else if let command = item["command"] as? String {
      web?.command(command)
    }
  }

  // MARK: View and Help › Rules

  /// Show/Hide Hand, Table or Log in the game in front.
  @objc func toggleViewPart(_ sender: NSMenuItem) {
    guard let part = MenuBuilder.viewTags[sender.tag] else { return }
    gameWeb?.toggleView(part)
  }

  @objc private func openRules(_ sender: NSMenuItem) {
    if let path = sender.representedObject as? String {
      showMain()
      web?.navigate(to: path)
    }
  }

  /// Whether a View part is showing in the game in front: true, false, or nil
  /// when there is no game in front or this game has no such part.
  private func viewPart(_ part: String) -> Bool? {
    (active?.role == .game ? active?.viewState : nil)?[part] as? Bool
  }

  func validateMenuItem(_ item: NSMenuItem) -> Bool {
    // A game window is one game: it has no history to walk.
    if item.action == #selector(goBack(_:)) { return active?.role != .game }
    if item.action == #selector(goForward(_:)) { return active?.role != .game && (web?.webView.canGoForward ?? false) }
    if item.action == #selector(toggleViewPart(_:)), let part = MenuBuilder.viewTags[item.tag] {
      return viewPart(part) != nil
    }
    return true
  }

  private func refreshViewMenu() {
    for item in viewMenu.items {
      guard let part = MenuBuilder.viewTags[item.tag] else { continue }
      let Part = part.prefix(1).uppercased() + part.dropFirst()
      item.title = viewPart(part) == false
        ? label("show\(Part)", "Show \(Part)") : label("hide\(Part)", "Hide \(Part)")
    }
  }

  /// The rules of the game in front first, then every game by name.
  private func refreshRulesMenu() {
    rulesMenu.removeAllItems()
    let games = main?.menuState?["rules"] as? [[String: Any]] ?? []
    var current: String?
    if active?.role == .game, let target = active?.viewState?["rules"] as? [String: Any], let id = target["moduleId"] as? String {
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
    case "Help": refreshHelpMenu(); menu = helpMenu
    case "bar": menu = NSApp.mainMenu ?? accountMenu
    case "File", "Edit", "Table", "Window":
      let id = "menu.\(which.lowercased())"
      menu = NSApp.mainMenu?.items.first { $0.identifier?.rawValue == id }?.submenu ?? accountMenu
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
    if menu === helpMenu { return refreshHelpMenu() }
    guard menu === accountMenu else { return }
    menu.removeAllItems()
    let state = main?.menuState ?? [:]
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
    refreshHelpMenu()
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
