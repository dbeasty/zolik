import AppKit

/// The menu bar. The standard Mac menus, plus what the phones keep elsewhere:
/// Table for Play offline, Account for the account menu behind the face in
/// the header (which the Mac app does not show), and Go in View for moving
/// between screens. File › New Window opens a second table alongside the
/// first.
enum MenuBuilder {
  /// Tags of View's show/hide items, whose titles follow what is showing.
  static let viewTags: [Int: String] = [101: "hand", 102: "table", 103: "log"]
  static let viewNames: [String: String] = ["hand": "Hand", "table": "Table", "log": "Log"]

  static func build(target: AppDelegate, account: NSMenu, view: NSMenu, rules: NSMenu) -> NSMenu {
    let main = NSMenu()

    let appMenu = NSMenu(title: "Jokerless")
    appMenu.addItem(item("About Jokerless", #selector(AppDelegate.showAbout(_:)), "", target))
    appMenu.addItem(.separator())
    appMenu.addItem(item("Settings…", #selector(AppDelegate.showSettings(_:)), ",", target))
    appMenu.addItem(.separator())
    appMenu.addItem(withTitle: "Hide Jokerless", action: #selector(NSApplication.hide(_:)), keyEquivalent: "h")
    let others = appMenu.addItem(
      withTitle: "Hide Others", action: #selector(NSApplication.hideOtherApplications(_:)), keyEquivalent: "h")
    others.keyEquivalentModifierMask = [.command, .option]
    appMenu.addItem(
      withTitle: "Show All", action: #selector(NSApplication.unhideAllApplications(_:)), keyEquivalent: "")
    appMenu.addItem(.separator())
    appMenu.addItem(withTitle: "Quit Jokerless", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q")
    add(appMenu, to: main)

    let file = NSMenu(title: "File")
    file.addItem(item("New Window", #selector(AppDelegate.newWindow(_:)), "n", target))
    file.addItem(.separator())
    file.addItem(withTitle: "Close Window", action: #selector(NSWindow.performClose(_:)), keyEquivalent: "w")
    add(file, to: main)

    // Without an Edit menu, ⌘C/⌘V/⌘A do nothing in the web view's fields.
    let edit = NSMenu(title: "Edit")
    edit.addItem(withTitle: "Undo", action: Selector(("undo:")), keyEquivalent: "z")
    let redo = edit.addItem(withTitle: "Redo", action: Selector(("redo:")), keyEquivalent: "z")
    redo.keyEquivalentModifierMask = [.command, .shift]
    edit.addItem(.separator())
    edit.addItem(withTitle: "Cut", action: #selector(NSText.cut(_:)), keyEquivalent: "x")
    edit.addItem(withTitle: "Copy", action: #selector(NSText.copy(_:)), keyEquivalent: "c")
    edit.addItem(withTitle: "Paste", action: #selector(NSText.paste(_:)), keyEquivalent: "v")
    edit.addItem(withTitle: "Select All", action: #selector(NSText.selectAll(_:)), keyEquivalent: "a")
    add(edit, to: main)

    // Show/Hide Hand, Table and Log first: the game's own parts, titled and
    // enabled by AppDelegate from what the game in front says it has.
    for (tag, key) in [(101, "1"), (102, "2"), (103, "3")] {
      let part = item("Hide", #selector(AppDelegate.toggleViewPart(_:)), key, target)
      part.keyEquivalentModifierMask = [.command, .option]
      part.tag = tag
      view.addItem(part)
    }
    view.addItem(.separator())
    // Both ways, always: whatever Back leaves, Forward returns to.
    view.addItem(item("Back", #selector(AppDelegate.goBack(_:)), "[", target))
    view.addItem(item("Forward", #selector(AppDelegate.goForward(_:)), "]", target))
    let home = item("Home", #selector(AppDelegate.goHome(_:)), "h", target)
    home.keyEquivalentModifierMask = [.command, .shift]
    view.addItem(home)
    view.addItem(.separator())
    view.addItem(item("Actual Size", #selector(AppDelegate.actualSize(_:)), "0", target))
    view.addItem(item("Zoom In", #selector(AppDelegate.zoomIn(_:)), "+", target))
    view.addItem(item("Zoom Out", #selector(AppDelegate.zoomOut(_:)), "-", target))
    view.addItem(.separator())
    view.addItem(item("Reload", #selector(AppDelegate.reloadPage(_:)), "r", target))
    // macOS adds Enter Full Screen to a menu titled View by itself.
    add(view, to: main)

    let table = NSMenu(title: "Table")
    let offline = item("Start an offline table", #selector(AppDelegate.startOfflineTable(_:)), "n", target)
    offline.keyEquivalentModifierMask = [.command, .shift]
    table.addItem(offline)
    table.addItem(item("Back to online play", #selector(AppDelegate.backOnline(_:)), "", target))
    table.addItem(.separator())
    table.addItem(item("Stats", #selector(AppDelegate.showStats(_:)), "", target))
    add(table, to: main)

    // Filled by AppDelegate.menuNeedsUpdate from the page, in its language.
    add(account, to: main)

    let window = NSMenu(title: "Window")
    window.addItem(withTitle: "Minimize", action: #selector(NSWindow.performMiniaturize(_:)), keyEquivalent: "m")
    window.addItem(withTitle: "Zoom", action: #selector(NSWindow.performZoom(_:)), keyEquivalent: "")
    window.addItem(.separator())
    window.addItem(
      withTitle: "Bring All to Front", action: #selector(NSApplication.arrangeInFront(_:)), keyEquivalent: "")
    add(window, to: main)
    NSApp.windowsMenu = window

    let help = NSMenu(title: "Help")
    // Every game's rules, the one in front first (AppDelegate fills it).
    let rulesHolder = NSMenuItem(title: "Rules", action: nil, keyEquivalent: "")
    rulesHolder.submenu = rules
    help.addItem(rulesHolder)
    help.addItem(item("More", #selector(AppDelegate.showMore(_:)), "", target))
    help.addItem(item("Jokerless Website", #selector(AppDelegate.openWebsite(_:)), "", target))
    add(help, to: main)
    NSApp.helpMenu = help

    return main
  }

  private static func item(_ title: String, _ action: Selector, _ key: String, _ target: AnyObject) -> NSMenuItem {
    let i = NSMenuItem(title: title, action: action, keyEquivalent: key)
    i.target = target
    return i
  }

  private static func add(_ menu: NSMenu, to main: NSMenu) {
    let holder = NSMenuItem(title: menu.title, action: nil, keyEquivalent: "")
    holder.submenu = menu
    main.addItem(holder)
  }
}
