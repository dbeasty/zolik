import AppKit

/// The menu bar. The standard Mac menus, plus what the phones keep elsewhere:
/// Table for Play offline, Account for the account menu behind the face in
/// the header (which the Mac app does not show), and Go in View for moving
/// between screens. File › New Window opens a second table alongside the
/// first.
enum MenuBuilder {
  static func build(target: AppDelegate, account: NSMenu) -> NSMenu {
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

    let view = NSMenu(title: "View")
    view.addItem(item("Back", #selector(AppDelegate.goBack(_:)), "[", target))
    let home = item("Home", #selector(AppDelegate.goHome(_:)), "h", target)
    home.keyEquivalentModifierMask = [.command, .shift]
    view.addItem(home)
    view.addItem(.separator())
    view.addItem(item("Actual Size", #selector(AppDelegate.actualSize(_:)), "0", target))
    view.addItem(item("Zoom In", #selector(AppDelegate.zoomIn(_:)), "+", target))
    view.addItem(item("Zoom Out", #selector(AppDelegate.zoomOut(_:)), "-", target))
    view.addItem(.separator())
    view.addItem(item("Reload", #selector(AppDelegate.reloadPage(_:)), "r", target))
    let full = view.addItem(
      withTitle: "Enter Full Screen", action: #selector(NSWindow.toggleFullScreen(_:)), keyEquivalent: "f")
    full.keyEquivalentModifierMask = [.command, .control]
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
    help.addItem(item("Rules", #selector(AppDelegate.showRules(_:)), "", target))
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
