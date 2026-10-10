import AppKit

// No storyboard and no nib: the window, its menus and its web view are built
// in code (AppDelegate), which keeps the whole app buildable by `swift build`.
let app = NSApplication.shared
let delegate = AppDelegate()
app.delegate = delegate
app.setActivationPolicy(.regular)
app.run()
