import AppKit

/// The menu bar. The standard Mac menus, plus what the phones keep elsewhere:
/// Table for Play offline, Account for the account menu behind the face in
/// the header (which the Mac app does not show), the game's own parts and
/// Back/Forward in View, and every game's rules and the legal notices in Help.
/// File › New Window opens a second table alongside the first.
///
/// Every item carries a key (its identifier, `menu.<key>`); AppDelegate
/// retitles them from the page's words, so the menu bar speaks the player's
/// language. The English here is only what shows before the page has said.
enum MenuBuilder {
  /// Tags of View's show/hide items, whose titles follow what is showing.
  static let viewTags: [Int: String] = [101: "hand", 102: "table", 103: "log"]
  /// Help's legal items are rebuilt from the page; they carry this tag.
  static let legalTag = 200

  static func build(target: AppDelegate, account: NSMenu, view: NSMenu, rules: NSMenu, help: NSMenu) -> NSMenu {
    let main = NSMenu()

    let appMenu = NSMenu(title: "Jokerless")
    appMenu.addItem(item("about", "About Jokerless", #selector(AppDelegate.showAbout(_:)), "", target))
    appMenu.addItem(.separator())
    appMenu.addItem(item("settings", "Settings…", #selector(AppDelegate.showSettings(_:)), ",", target))
    appMenu.addItem(.separator())
    appMenu.addItem(item("hideApp", "Hide Jokerless", #selector(NSApplication.hide(_:)), "h", nil))
    let others = item("hideOthers", "Hide Others", #selector(NSApplication.hideOtherApplications(_:)), "h", nil)
    others.keyEquivalentModifierMask = [.command, .option]
    appMenu.addItem(others)
    appMenu.addItem(item("showAll", "Show All", #selector(NSApplication.unhideAllApplications(_:)), "", nil))
    appMenu.addItem(.separator())
    appMenu.addItem(item("quit", "Quit Jokerless", #selector(NSApplication.terminate(_:)), "q", nil))
    add(appMenu, key: nil, to: main)

    let file = NSMenu(title: "File")
    file.addItem(item("newWindow", "New Window", #selector(AppDelegate.newWindow(_:)), "n", target))
    file.addItem(.separator())
    file.addItem(item("closeWindow", "Close Window", #selector(NSWindow.performClose(_:)), "w", nil))
    add(file, key: "file", to: main)

    // Without an Edit menu, ⌘C/⌘V/⌘A do nothing in the web view's fields.
    let edit = NSMenu(title: "Edit")
    edit.addItem(item("undo", "Undo", Selector(("undo:")), "z", nil))
    let redo = item("redo", "Redo", Selector(("redo:")), "z", nil)
    redo.keyEquivalentModifierMask = [.command, .shift]
    edit.addItem(redo)
    edit.addItem(.separator())
    edit.addItem(item("cut", "Cut", #selector(NSText.cut(_:)), "x", nil))
    edit.addItem(item("copy", "Copy", #selector(NSText.copy(_:)), "c", nil))
    edit.addItem(item("paste", "Paste", #selector(NSText.paste(_:)), "v", nil))
    edit.addItem(item("selectAll", "Select All", #selector(NSText.selectAll(_:)), "a", nil))
    add(edit, key: "edit", to: main)

    // Show/Hide Hand, Table and Log first: the game's own parts, titled and
    // enabled by AppDelegate from what the game in front says it has.
    for (tag, key) in [(101, "1"), (102, "2"), (103, "3")] {
      let part = item(nil, "Hide", #selector(AppDelegate.toggleViewPart(_:)), key, target)
      part.keyEquivalentModifierMask = [.command, .option]
      part.tag = tag
      view.addItem(part)
    }
    view.addItem(.separator())
    // Both ways, always: whatever Back leaves, Forward returns to.
    view.addItem(item("back", "Back", #selector(AppDelegate.goBack(_:)), "[", target))
    view.addItem(item("forward", "Forward", #selector(AppDelegate.goForward(_:)), "]", target))
    let home = item("home", "Home", #selector(AppDelegate.goHome(_:)), "h", target)
    home.keyEquivalentModifierMask = [.command, .shift]
    view.addItem(home)
    view.addItem(.separator())
    view.addItem(item("actualSize", "Actual Size", #selector(AppDelegate.actualSize(_:)), "0", target))
    view.addItem(item("zoomIn", "Zoom In", #selector(AppDelegate.zoomIn(_:)), "+", target))
    view.addItem(item("zoomOut", "Zoom Out", #selector(AppDelegate.zoomOut(_:)), "-", target))
    view.addItem(.separator())
    view.addItem(item("reload", "Reload", #selector(AppDelegate.reloadPage(_:)), "r", target))
    // macOS adds Enter Full Screen to the View menu by itself.
    add(view, key: "view", to: main)

    let table = NSMenu(title: "Table")
    let offline = item("startOffline", "Start an offline table", #selector(AppDelegate.startOfflineTable(_:)), "n", target)
    offline.keyEquivalentModifierMask = [.command, .shift]
    table.addItem(offline)
    table.addItem(item("backOnline", "Back to online play", #selector(AppDelegate.backOnline(_:)), "", target))
    table.addItem(.separator())
    table.addItem(item("stats", "Stats", #selector(AppDelegate.showStats(_:)), "", target))
    add(table, key: "table", to: main)

    // Filled by AppDelegate.menuNeedsUpdate from the page, in its language.
    add(account, key: "account", to: main)

    let window = NSMenu(title: "Window")
    window.addItem(item("minimize", "Minimize", #selector(NSWindow.performMiniaturize(_:)), "m", nil))
    window.addItem(item("zoom", "Zoom", #selector(NSWindow.performZoom(_:)), "", nil))
    window.addItem(.separator())
    window.addItem(item("bringAllToFront", "Bring All to Front", #selector(NSApplication.arrangeInFront(_:)), "", nil))
    add(window, key: "window", to: main)
    NSApp.windowsMenu = window

    // Every game's rules, the one in front first (AppDelegate fills it).
    let rulesHolder = NSMenuItem(title: "Rules", action: nil, keyEquivalent: "")
    rulesHolder.identifier = NSUserInterfaceItemIdentifier("menu.rules")
    rulesHolder.submenu = rules
    help.addItem(rulesHolder)
    help.addItem(item("more", "More", #selector(AppDelegate.showMore(_:)), "", target))
    help.addItem(item("website", "Jokerless Website", #selector(AppDelegate.openWebsite(_:)), "", target))
    // Then the notices the phones keep in their footer (Terms, Privacy,
    // Accessibility, Source), added by AppDelegate under `legalTag`.
    add(help, key: "help", to: main)
    NSApp.helpMenu = help

    return main
  }

  private static func item(
    _ key: String?, _ title: String, _ action: Selector, _ keyEquivalent: String, _ target: AnyObject?
  ) -> NSMenuItem {
    let i = NSMenuItem(title: title, action: action, keyEquivalent: keyEquivalent)
    i.target = target
    if let key { i.identifier = NSUserInterfaceItemIdentifier("menu.\(key)") }
    return i
  }

  private static func add(_ menu: NSMenu, key: String?, to main: NSMenu) {
    let holder = NSMenuItem(title: menu.title, action: nil, keyEquivalent: "")
    if let key { holder.identifier = NSUserInterfaceItemIdentifier("menu.\(key)") }
    holder.submenu = menu
    main.addItem(holder)
  }

  /// Retitles every keyed item (and the menu a holder opens) from `labels`.
  static func apply(_ labels: [String: String], to menu: NSMenu) {
    for item in menu.items {
      if let id = item.identifier?.rawValue, id.hasPrefix("menu."), let title = labels[String(id.dropFirst(5))] {
        item.title = title
        item.submenu?.title = title
      }
      if let sub = item.submenu { apply(labels, to: sub) }
    }
  }
}
