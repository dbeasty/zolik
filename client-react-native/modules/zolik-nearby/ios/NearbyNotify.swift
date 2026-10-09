import UIKit
import UserNotifications
import Zolikcore

/// The phone's own notification for a table it hosts: somebody sat down.
///
/// A phone hosting a table has no internet to push through, so the embedded
/// server tells its owner itself (zolikcore.Notifier). Shown only while the
/// app is not in front; in front, the app's banner says it. Uses the
/// permission the app asked for through expo-notifications, and nothing at
/// all without it.
final class JoinNotifier: NSObject, ZolikcoreNotifierProtocol {
  func notify(_ tag: String?, title: String?, body: String?, url: String?) {
    DispatchQueue.main.async {
      guard UIApplication.shared.applicationState != .active else { return }
      let content = UNMutableNotificationContent()
      content.title = title ?? ""
      content.body = body ?? ""
      content.sound = .default
      // expo-notifications reads a notification's data from "body", so a
      // tap is routed like a push's (src/notify/push.native.ts).
      content.userInfo = ["body": ["type": "table_joined_local", "url": url ?? ""]]
      let request = UNNotificationRequest(identifier: tag ?? UUID().uuidString, content: content, trigger: nil)
      UNUserNotificationCenter.current().add(request)
    }
  }
}

/// Keeps a hosting phone's server answering for the short while iOS allows
/// after the app leaves the screen.
///
/// iOS suspends an app the moment it goes to the background, and with it the
/// embedded server: a guest arriving a second after the host locked the phone
/// would find nobody there, and the host would never be told. While the room
/// is open, the module asks for the background time iOS grants (about thirty
/// seconds), which is when a late arrival is still caught and announced. The
/// time is handed back as soon as the app returns, or when iOS calls it in.
final class HostBackgroundGrace {
  private var task: UIBackgroundTaskIdentifier = .invalid

  func enterBackground() {
    guard task == .invalid, let host = ZolikcoreCurrent(), host.lanPort() > 0 else { return }
    task = UIApplication.shared.beginBackgroundTask(withName: "zolik-host") { [weak self] in
      self?.end()
    }
  }

  func enterForeground() {
    end()
  }

  private func end() {
    guard task != .invalid else { return }
    UIApplication.shared.endBackgroundTask(task)
    task = .invalid
  }
}
