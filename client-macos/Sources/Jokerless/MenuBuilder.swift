import AppKit

/// The menu bar. Standard Mac menus, plus a Table menu for what the phone app
/// keeps on its Play offline screen.
enum MenuBuilder {
  static func build(target: AppDelegate) -> NSMenu {
    let main = NSMenu()

    let appMenu = NSMenu(title: "Jokerless")
    appMenu.addItem(
      withTitle: "About Jokerless", action: #selector(NSApplication.orderFrontStandardAboutPanel(_:)),
      keyEquivalent: "")
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

    let table = NSMenu(title: "Table")
    table.addItem(item("Start an offline table", #selector(AppDelegate.startOfflineTable(_:)), "n", target))
    table.addItem(item("Back to online play", #selector(AppDelegate.backOnline(_:)), "", target))
    table.addItem(.separator())
    table.addItem(item("Stats", #selector(AppDelegate.showStats(_:)), "", target))
    add(table, to: main)

    let view = NSMenu(title: "View")
    view.addItem(item("Reload", #selector(AppDelegate.reloadPage(_:)), "r", target))
    let full = view.addItem(
      withTitle: "Enter Full Screen", action: #selector(NSWindow.toggleFullScreen(_:)), keyEquivalent: "f")
    full.keyEquivalentModifierMask = [.command, .control]
    add(view, to: main)

    let window = NSMenu(title: "Window")
    window.addItem(withTitle: "Minimize", action: #selector(NSWindow.performMiniaturize(_:)), keyEquivalent: "m")
    window.addItem(withTitle: "Zoom", action: #selector(NSWindow.performZoom(_:)), keyEquivalent: "")
    add(window, to: main)
    NSApp.windowsMenu = window

    let help = NSMenu(title: "Help")
    help.addItem(item("Rules", #selector(AppDelegate.showRules(_:)), "", target))
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
