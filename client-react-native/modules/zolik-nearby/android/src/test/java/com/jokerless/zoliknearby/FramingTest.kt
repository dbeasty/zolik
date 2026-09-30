package com.jokerless.zoliknearby

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/** The Kotlin twin of ios/FramingCheck: the on-air framing, with no device. */
class FramingTest {
  @Test
  fun roundTripsAtEveryChunkSizeWhateverTheSlicing() {
    val msgs = listOf(ByteArray(0), byteArrayOf(1), ByteArray(1000) { 7 }, ByteArray(5000) { (it % 251).toByte() })
    for (size in listOf(20, 23, 100, 182, 512)) {
      val stream = msgs.flatMap { frameChunks(it, size) }
      stream.forEach { assertTrue("chunk of ${it.size} over $size", it.size <= maxOf(size, 20)) }
      val all = stream.fold(ByteArray(0)) { a, b -> a + b }
      val r = Reassembler()
      val out = mutableListOf<ByteArray>()
      var at = 0
      while (at < all.size) {
        out += r.feed(all.copyOfRange(at, minOf(at + 7, all.size)))
        at += 7
      }
      assertEquals("size $size", msgs.map { it.toList() }, out.map { it.toList() })
    }
  }

  @Test
  fun aChunkNeverExceedsTheLargestAttributeValue() {
    assertEquals(20, chunkSize(23))
    assertEquals(182, chunkSize(185))
    assertEquals(512, chunkSize(517))
    frameChunks(ByteArray(5000), chunkSize(517)).forEach { assertTrue(it.size <= 512) }
  }

  @Test
  fun aCorruptLengthResetsRatherThanWaitingForever() {
    val r = Reassembler()
    assertTrue(r.feed(byteArrayOf(0x7f, -1, -1, -1, 1, 2)).isEmpty())
    assertEquals(listOf(listOf<Byte>(9, 9)), r.feed(frameChunks(byteArrayOf(9, 9), 100)[0]).map { it.toList() })
  }
}
