import Foundation
import Zolikcore

/// Tells the embedded host how hot the phone is, so its bot governor stops
/// asking a throttled core for the dearest decisions (server/internal/botgov).
///
/// iOS reports heat as ProcessInfo.thermalState and posts a notification when
/// it changes; Low Power Mode is the person asking the phone to do less, and
/// counts as "serious". Levels as Go reads them: 0 nominal or fair, 1 serious,
/// 2 critical.
final class NearbyThermal {
  private var observers: [NSObjectProtocol] = []

  /// Starts watching, and reports the state now. Safe to call again: the
  /// host may be started more than once in a process's life.
  func start() {
    if observers.isEmpty {
      let center = NotificationCenter.default
      observers.append(center.addObserver(
        forName: ProcessInfo.thermalStateDidChangeNotification, object: nil, queue: nil
      ) { [weak self] _ in self?.report() })
      observers.append(center.addObserver(
        forName: Notification.Name.NSProcessInfoPowerStateDidChange, object: nil, queue: nil
      ) { [weak self] _ in self?.report() })
    }
    report()
  }

  func stop() {
    observers.forEach { NotificationCenter.default.removeObserver($0) }
    observers.removeAll()
  }

  static func level() -> Int {
    let info = ProcessInfo.processInfo
    switch info.thermalState {
    case .critical:
      return 2
    case .serious:
      return 1
    default:
      return info.isLowPowerModeEnabled ? 1 : 0
    }
  }

  private func report() {
    ZolikcoreCurrent()?.setThermalPressure(Self.level())
  }
}
