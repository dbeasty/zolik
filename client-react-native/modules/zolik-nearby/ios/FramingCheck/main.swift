// Checks NearbyFraming.swift on its own, with no device: see
// scripts/test-nearby-framing.sh. Not part of the pod (its podspec only
// takes the Swift files beside it).

import Foundation

func check(_ cond: Bool, _ what: String) { if !cond { print("FAIL: \(what)"); exit(1) } }

// Round trip across every chunk size, with messages delivered in odd slices.
for size in [20, 23, 100, 182, 512] {
  let msgs = [Data(), Data([1]), Data(repeating: 7, count: 1000), Data((0..<5000).map { UInt8($0 % 251) })]
  var stream: [Data] = []
  for m in msgs { stream.append(contentsOf: frameChunks(m, size: size)) }
  for c in stream { check(c.count <= max(size, 20), "chunk of \(c.count) over \(size)") }
  let r = Reassembler()
  var out: [Data] = []
  // Re-slice the whole stream into 7-byte pieces: chunk boundaries must not matter.
  let all = stream.reduce(Data(), +)
  var at = 0
  while at < all.count { out += r.feed(all.subdata(in: at..<min(at + 7, all.count))); at += 7 }
  check(out == msgs, "size \(size): got \(out.map { $0.count }) want \(msgs.map { $0.count })")
}
// A length no message could have resets rather than waiting for ever.
let r = Reassembler()
check(r.feed(Data([0x7f, 0xff, 0xff, 0xff, 1, 2])).isEmpty, "corrupt length")
check(r.feed(frameChunks(Data([9, 9]), size: 100)[0]) == [Data([9, 9])], "recovers after a corrupt length")
print("framing ok")
