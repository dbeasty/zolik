import AppKit
import WebKit
import Zolikcore

final class AppDelegate: NSObject, NSApplicationDelegate {
  private var window: NSWindow!
  private var web: WebController!
  private var e2e: E2EControl?
  private var activity: NSObjectProtocol?

  func applicationDidFinishLaunching(_ notification: Notification) {
    AppConfig.configureCore()
    web = WebController()

    window = NSWindow(
      contentRect: NSRect(x: 0, y: 0, width: 1280, height: 820),
      styleMask: [.titled, .closable, .miniaturizable, .resizable, .fullSizeContentView],
      backing: .buffered, defer: false)
    window.title = "Jokerless"
    window.minSize = NSSize(width: 720, height: 560)
    window.appearance = NSAppearance(named: .darkAqua)
    window.backgroundColor = NSColor(red: 0x0f / 255, green: 0x14 / 255, blue: 0x19 / 255, alpha: 1)
    window.titlebarAppearsTransparent = true
    window.tabbingMode = .disallowed
    window.contentView = web.view
    window.center()
    window.setFrameAutosaveName("JokerlessMain")
    window.makeKeyAndOrderFront(nil)

    NSApp.mainMenu = MenuBuilder.build(target: self)

    if let dir = AppConfig.e2eControlDir {
      // A test run must not be slowed by App Nap while its window sits
      // behind whatever the person at the Mac is doing.
      activity = ProcessInfo.processInfo.beginActivity(
        options: [.userInitiated, .idleSystemSleepDisabled], reason: "end-to-end run")
      e2e = E2EControl(dir: dir, web: web, app: self)
      e2e?.start()
    } else {
      NSApp.activate(ignoringOtherApps: true)
    }
    web.load()
  }

  func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool { true }

  func applicationWillTerminate(_ notification: Notification) {
    // The table lives in this process; quitting ends it for everyone, so it
    // is closed properly rather than dropped.
    web.nearby.shutdown()
  }

  // MARK: menu actions

  @objc func startOfflineTable(_ sender: Any?) { web.navigate(to: "/offline") }
  @objc func backOnline(_ sender: Any?) { web.navigate(to: "/") }
  @objc func showRules(_ sender: Any?) { web.navigate(to: "/rules") }
  @objc func showStats(_ sender: Any?) { web.navigate(to: "/stats") }
  @objc func showSettings(_ sender: Any?) { web.navigate(to: "/settings") }
  @objc func reloadPage(_ sender: Any?) { web.webView.reload() }
  @objc func openWebsite(_ sender: Any?) {
    if let url = URL(string: AppConfig.serverURL) { NSWorkspace.shared.open(url) }
  }

  /// Menu items by title, for the end-to-end runner.
  func performMenuItem(titled title: String) -> Bool {
    func find(_ menu: NSMenu) -> NSMenuItem? {
      for item in menu.items {
        if item.title == title { return item }
        if let sub = item.submenu, let hit = find(sub) { return hit }
      }
      return nil
    }
    guard let menu = NSApp.mainMenu, let item = find(menu), let action = item.action else { return false }
    return NSApp.sendAction(action, to: item.target, from: item)
  }
}
