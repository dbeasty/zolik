import Foundation

// The on-air framing both platforms share: each message is a 4-byte
// big-endian length and then its bytes, cut into chunks the link allows.
// Kept free of CoreBluetooth so it can be tested on its own. The Kotlin twin
// is in android/.../NearbyBle.kt, with tests in android/src/test.

/// Puts length-prefixed messages back together from chunks.
final class Reassembler {
  private var buffer = Data()

  /// Feeds a chunk, returning every message it completed.
  func feed(_ chunk: Data) -> [Data] {
    buffer.append(chunk)
    var out: [Data] = []
    while buffer.count >= 4 {
      let b = [UInt8](buffer.prefix(4))
      let length = Int(b[0]) << 24 | Int(b[1]) << 16 | Int(b[2]) << 8 | Int(b[3])
      // A length no tunnel message could have means the stream is corrupt.
      if length > 8 << 20 {
        buffer.removeAll()
        return out
      }
      guard buffer.count >= 4 + length else { break }
      out.append(buffer.subdata(in: buffer.startIndex + 4..<buffer.startIndex + 4 + length))
      buffer.removeSubrange(buffer.startIndex..<buffer.startIndex + 4 + length)
    }
    return out
  }
}

func frameChunks(_ msg: Data, size: Int) -> [Data] {
  var framed = Data()
  var n = UInt32(msg.count).bigEndian
  framed.append(Data(bytes: &n, count: 4))
  framed.append(msg)
  let step = max(size, 20)
  return stride(from: 0, to: framed.count, by: step).map {
    framed.subdata(in: $0..<min($0 + step, framed.count))
  }
}

