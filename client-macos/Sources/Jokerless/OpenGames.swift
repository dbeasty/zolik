import Foundation

/// The games that have a window open, remembered so the app brings them back
/// at the next launch. Online games only (a table on this Mac's own server
/// ends with the app), and never a finished one: the page drops a game from
/// here as soon as it reports the match over.
final class OpenGames {
  struct Entry: Codable, Equatable {
    var key: String
    var path: String
  }

  private(set) var entries: [Entry]
  private let file: URL

  init(file: URL = AppConfig.dataDir.appendingPathComponent("windows.json")) {
    self.file = file
    entries = (try? Data(contentsOf: file)).flatMap { try? JSONDecoder().decode([Entry].self, from: $0) } ?? []
  }

  func add(key: String, path: String) {
    if let i = entries.firstIndex(where: { $0.key == key }) {
      entries[i].path = path
    } else {
      entries.append(Entry(key: key, path: path))
    }
    save()
  }

  func remove(_ key: String) {
    entries.removeAll { $0.key == key }
    save()
  }

  func clear() {
    entries = []
    save()
  }

  private func save() {
    try? FileManager.default.createDirectory(at: file.deletingLastPathComponent(), withIntermediateDirectories: true)
    if let data = try? JSONEncoder().encode(entries) { try? data.write(to: file, options: .atomic) }
  }
}
