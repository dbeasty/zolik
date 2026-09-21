package com.jokerless.zoliknearby

import android.annotation.SuppressLint
import android.bluetooth.BluetoothAdapter
import android.bluetooth.BluetoothDevice
import android.bluetooth.BluetoothGatt
import android.bluetooth.BluetoothGattCallback
import android.bluetooth.BluetoothGattCharacteristic
import android.bluetooth.BluetoothGattDescriptor
import android.bluetooth.BluetoothGattServer
import android.bluetooth.BluetoothGattServerCallback
import android.bluetooth.BluetoothGattService
import android.bluetooth.BluetoothManager
import android.bluetooth.BluetoothProfile
import android.bluetooth.le.AdvertiseCallback
import android.bluetooth.le.AdvertiseData
import android.bluetooth.le.AdvertiseSettings
import android.bluetooth.le.ScanCallback
import android.bluetooth.le.ScanFilter
import android.bluetooth.le.ScanResult
import android.bluetooth.le.ScanSettings
import android.content.Context
import android.os.Build
import android.os.Handler
import android.os.HandlerThread
import android.os.ParcelUuid
import com.jokerless.zolikcore.Tunnel
import com.jokerless.zolikcore.TunnelSink
import com.jokerless.zolikcore.Zolikcore
import java.io.ByteArrayOutputStream
import java.nio.ByteBuffer
import java.util.UUID

/**
 * Bluetooth for a table with no network at all. See NearbyBle.swift for the
 * design; this is the same protocol on Android.
 *
 * The phones never pair. Every characteristic is plain (no ENCRYPTED or MITM
 * permission) and nothing here calls createBond, so no pairing dialog ever
 * appears. The Go tunnel encrypts every message end to end.
 *
 * Every Bluetooth call and callback runs on one handler thread. That keeps
 * one operation in flight per link, which is what Android's GATT requires
 * and what keeps notifications from being dropped.
 */
internal object NearbyBleIds {
  val SERVICE: UUID = UUID.fromString("a54a0c00-3f5e-4d2a-9c47-5a6f6c696b01")
  val INFO: UUID = UUID.fromString("a54a0c01-3f5e-4d2a-9c47-5a6f6c696b01")
  val C2H: UUID = UUID.fromString("a54a0c02-3f5e-4d2a-9c47-5a6f6c696b01")
  val H2C: UUID = UUID.fromString("a54a0c03-3f5e-4d2a-9c47-5a6f6c696b01")
  val CCCD: UUID = UUID.fromString("00002902-0000-1000-8000-00805f9b34fb")
}

/** Puts length-prefixed messages back together from chunks. */
internal class Reassembler {
  private val buffer = ByteArrayOutputStream()

  fun feed(chunk: ByteArray): List<ByteArray> {
    buffer.write(chunk)
    val out = mutableListOf<ByteArray>()
    var bytes = buffer.toByteArray()
    var at = 0
    while (bytes.size - at >= 4) {
      val length = ByteBuffer.wrap(bytes, at, 4).int
      if (length < 0 || length > 8 shl 20) {
        // A length no tunnel message could have: the stream is corrupt.
        buffer.reset()
        return out
      }
      if (bytes.size - at - 4 < length) break
      out += bytes.copyOfRange(at + 4, at + 4 + length)
      at += 4 + length
    }
    buffer.reset()
    buffer.write(bytes, at, bytes.size - at)
    return out
  }
}

/**
 * The most a chunk can carry on a link with this MTU. An ATT value can never
 * exceed 512 bytes, whatever the MTU, and Android refuses a longer
 * notification or write outright: with the 517-byte MTU phones negotiate,
 * MTU − 3 alone would be 514.
 */
internal fun chunkSize(mtu: Int): Int = minOf(mtu - 3, 512)

internal fun frameChunks(msg: ByteArray, size: Int): List<ByteArray> {
  val framed = ByteBuffer.allocate(4 + msg.size).putInt(msg.size).put(msg).array()
  val step = maxOf(size, 20)
  return (framed.indices step step).map { framed.copyOfRange(it, minOf(it + step, framed.size)) }
}

/** About two seconds of a stack refusing the same chunk, at 10 ms a try. */
private const val MAX_REFUSALS = 200

internal class BleThread {
  private val thread = HandlerThread("zolik-ble").apply { start() }
  val handler = Handler(thread.looper)
  fun post(run: () -> Unit) { handler.post(run) }
}

// ---- Host (GATT server) ---------------------------------------------------

@SuppressLint("MissingPermission")
internal class NearbyBleHost(private val context: Context, private val onGuests: (Int) -> Unit) {
  private val ble = BleThread()
  private val manager = context.getSystemService(Context.BLUETOOTH_SERVICE) as BluetoothManager
  private var server: BluetoothGattServer? = null
  private var h2c: BluetoothGattCharacteristic? = null
  private var info = ByteArray(0)
  private val guests = mutableMapOf<String, Guest>()
  private var advertising: AdvertiseCallback? = null

  private inner class Guest(val device: BluetoothDevice, val tunnel: Tunnel) {
    val reassembler = Reassembler()
    val outbox = ArrayDeque<ByteArray>()
    var mtu = 23
    var notifying = false
    var refusals = 0
  }

  fun start(name: String) = ble.post {
    info = Zolikcore.current()?.bleInfo(name) ?: ByteArray(0)
    stopNow()
    val server = manager.openGattServer(context, callback) ?: return@post
    this.server = server
    val service = BluetoothGattService(NearbyBleIds.SERVICE, BluetoothGattService.SERVICE_TYPE_PRIMARY)
    service.addCharacteristic(
      BluetoothGattCharacteristic(
        NearbyBleIds.INFO,
        BluetoothGattCharacteristic.PROPERTY_READ,
        BluetoothGattCharacteristic.PERMISSION_READ,
      ),
    )
    service.addCharacteristic(
      BluetoothGattCharacteristic(
        NearbyBleIds.C2H,
        BluetoothGattCharacteristic.PROPERTY_WRITE,
        BluetoothGattCharacteristic.PERMISSION_WRITE,
      ),
    )
    val h2c = BluetoothGattCharacteristic(
      NearbyBleIds.H2C,
      BluetoothGattCharacteristic.PROPERTY_NOTIFY,
      BluetoothGattCharacteristic.PERMISSION_READ,
    )
    h2c.addDescriptor(
      BluetoothGattDescriptor(
        NearbyBleIds.CCCD,
        BluetoothGattDescriptor.PERMISSION_READ or BluetoothGattDescriptor.PERMISSION_WRITE,
      ),
    )
    service.addCharacteristic(h2c)
    this.h2c = h2c
    server.addService(service)
    advertise(name)
  }

  private fun advertise(name: String) {
    val advertiser = manager.adapter?.bluetoothLeAdvertiser ?: return
    val settings = AdvertiseSettings.Builder()
      .setAdvertiseMode(AdvertiseSettings.ADVERTISE_MODE_LOW_LATENCY)
      .setConnectable(true)
      .build()
    val data = AdvertiseData.Builder().addServiceUuid(ParcelUuid(NearbyBleIds.SERVICE)).build()
    // The table's name, in the scan response. A 128-bit UUID and its header
    // take 18 of the 31 bytes, which leaves 11 for the name.
    val nameBytes = name.toByteArray(Charsets.UTF_8).let { if (it.size > 11) it.copyOf(11) else it }
    val response = AdvertiseData.Builder()
      .addServiceData(ParcelUuid(NearbyBleIds.SERVICE), nameBytes)
      .build()
    val cb = object : AdvertiseCallback() {}
    advertising = cb
    advertiser.startAdvertising(settings, data, response, cb)
  }

  fun stop() = ble.post { stopNow() }

  /** Check codes of the guests that have finished their handshake. */
  fun codes(): List<String> {
    val latch = java.util.concurrent.CountDownLatch(1)
    var out = emptyList<String>()
    ble.post {
      out = guests.values.map { it.tunnel.checkCode() }.filter { it.isNotEmpty() }.sorted()
      latch.countDown()
    }
    latch.await(1, java.util.concurrent.TimeUnit.SECONDS)
    return out
  }

  private fun stopNow() {
    advertising?.let { manager.adapter?.bluetoothLeAdvertiser?.stopAdvertising(it) }
    advertising = null
    guests.values.forEach { it.tunnel.close() }
    guests.clear()
    // Hang up on every guest before closing the server. Closing it alone
    // leaves the LE link up: the guest sees only "services changed", thinks
    // it is still at the table, and never reconnects.
    server?.let { srv ->
      manager.getConnectedDevices(BluetoothProfile.GATT_SERVER).forEach { srv.cancelConnection(it) }
    }
    server?.close()
    server = null
    onGuests(0)
  }

  private fun guestFor(device: BluetoothDevice): Guest? {
    guests[device.address]?.let { return it }
    val host = Zolikcore.current() ?: return null
    val sink = object : TunnelSink {
      // Called on Go's threads: enqueue and return.
      override fun send(msg: ByteArray?) {
        if (msg != null) ble.post { enqueue(device.address, msg) }
      }
    }
    val g = Guest(device, host.newTunnel(sink, device.address))
    guests[device.address] = g
    onGuests(guests.size)
    return g
  }

  private fun enqueue(address: String, msg: ByteArray) {
    val g = guests[address] ?: return
    frameChunks(msg, chunkSize(g.mtu)).forEach { g.outbox.addLast(it) }
    pump(g)
  }

  private fun pump(g: Guest) {
    if (g.notifying) return
    val chunk = g.outbox.removeFirstOrNull() ?: return
    val server = server ?: return
    val c = h2c ?: return
    g.notifying = true
    val ok = if (Build.VERSION.SDK_INT >= 33) {
      server.notifyCharacteristicChanged(g.device, c, false, chunk) == BluetoothGatt.GATT_SUCCESS
    } else {
      @Suppress("DEPRECATION")
      c.value = chunk
      @Suppress("DEPRECATION")
      server.notifyCharacteristicChanged(g.device, c, false)
    }
    if (!ok) {
      g.outbox.addFirst(chunk)
      g.notifying = false
      // Busy: try the same chunk again shortly. A stack that keeps refusing
      // it is not busy, and retrying for ever would stall the table without
      // a word. Hang up instead, and the guest reconnects.
      if (++g.refusals > MAX_REFUSALS) {
        server.cancelConnection(g.device)
        return
      }
      ble.handler.postDelayed({ pump(g) }, 10)
    } else {
      g.refusals = 0
    }
  }

  private val callback = object : BluetoothGattServerCallback() {
    override fun onConnectionStateChange(device: BluetoothDevice, status: Int, newState: Int) = ble.post {
      if (newState == BluetoothProfile.STATE_DISCONNECTED) {
        guests.remove(device.address)?.let {
          it.tunnel.close()
          onGuests(guests.size)
        }
      }
    }

    override fun onMtuChanged(device: BluetoothDevice, mtu: Int) = ble.post {
      guests[device.address]?.mtu = mtu
      pendingMtu[device.address] = mtu
    }

    override fun onCharacteristicReadRequest(
      device: BluetoothDevice, requestId: Int, offset: Int, characteristic: BluetoothGattCharacteristic,
    ) = ble.post {
      val server = server ?: return@post
      if (characteristic.uuid != NearbyBleIds.INFO || offset > info.size) {
        server.sendResponse(device, requestId, BluetoothGatt.GATT_INVALID_OFFSET, offset, null)
        return@post
      }
      server.sendResponse(device, requestId, BluetoothGatt.GATT_SUCCESS, offset, info.copyOfRange(offset, info.size))
    }

    override fun onCharacteristicWriteRequest(
      device: BluetoothDevice, requestId: Int, characteristic: BluetoothGattCharacteristic,
      preparedWrite: Boolean, responseNeeded: Boolean, offset: Int, value: ByteArray?,
    ) = ble.post {
      val server = server ?: return@post
      if (characteristic.uuid == NearbyBleIds.C2H && value != null) {
        guestFor(device)?.let { g ->
          pendingMtu.remove(device.address)?.let { g.mtu = it }
          g.reassembler.feed(value).forEach { g.tunnel.receive(it) }
        }
      }
      if (responseNeeded) server.sendResponse(device, requestId, BluetoothGatt.GATT_SUCCESS, offset, null)
    }

    override fun onDescriptorWriteRequest(
      device: BluetoothDevice, requestId: Int, descriptor: BluetoothGattDescriptor,
      preparedWrite: Boolean, responseNeeded: Boolean, offset: Int, value: ByteArray?,
    ) = ble.post {
      // A central turning notifications on is a guest arriving.
      if (descriptor.uuid == NearbyBleIds.CCCD) guestFor(device)?.let { g -> pendingMtu.remove(device.address)?.let { g.mtu = it } }
      if (responseNeeded) server?.sendResponse(device, requestId, BluetoothGatt.GATT_SUCCESS, offset, null)
    }

    override fun onNotificationSent(device: BluetoothDevice, status: Int) = ble.post {
      guests[device.address]?.let {
        it.notifying = false
        pump(it)
      }
    }
  }

  /** An MTU can be negotiated before the guest's first write creates it. */
  private val pendingMtu = mutableMapOf<String, Int>()
}

// ---- Guest (GATT client) --------------------------------------------------

@SuppressLint("MissingPermission")
internal class NearbyBleGuest(
  private val context: Context,
  private val onFound: (Map<String, Any>) -> Unit,
  private val onMessage: (String, ByteArray) -> Unit,
  private val onClosed: (String) -> Unit,
) {
  private val ble = BleThread()
  private val manager = context.getSystemService(Context.BLUETOOTH_SERVICE) as BluetoothManager
  private var scanning: ScanCallback? = null
  private val links = mutableMapOf<String, Link>()

  private inner class Link(val id: String) {
    var gatt: BluetoothGatt? = null
    var c2h: BluetoothGattCharacteristic? = null
    var mtu = 23
    val reassembler = Reassembler()
    val outbox = ArrayDeque<Pair<ByteArray, (() -> Unit)?>>()
    var writing = false
    var refusals = 0
    var connected: ((Result<ByteArray>) -> Unit)? = null
  }

  fun scan() = ble.post {
    if (scanning != null) return@post
    val scanner = manager.adapter?.bluetoothLeScanner ?: return@post
    val cb = object : ScanCallback() {
      override fun onScanResult(callbackType: Int, result: ScanResult) {
        val record = result.scanRecord
        val name = record?.serviceData?.get(ParcelUuid(NearbyBleIds.SERVICE))?.let { String(it, Charsets.UTF_8) }
          ?: record?.deviceName ?: ""
        onFound(mapOf("peripheralId" to result.device.address, "name" to name, "rssi" to result.rssi))
      }
    }
    scanning = cb
    scanner.startScan(
      listOf(ScanFilter.Builder().setServiceUuid(ParcelUuid(NearbyBleIds.SERVICE)).build()),
      ScanSettings.Builder().setScanMode(ScanSettings.SCAN_MODE_LOW_LATENCY).build(),
      cb,
    )
  }

  fun stopScan() = ble.post {
    scanning?.let { manager.adapter?.bluetoothLeScanner?.stopScan(it) }
    scanning = null
  }

  fun connect(address: String, done: (Result<Pair<String, ByteArray>>) -> Unit) = ble.post {
    if (Build.VERSION.SDK_INT < 26) {
      done(Result.failure(IllegalStateException("Bluetooth tables need Android 8 or later")))
      return@post
    }
    val device = runCatching { manager.adapter.getRemoteDevice(address) }.getOrNull()
    if (device == null) {
      done(Result.failure(IllegalStateException("that table is out of range")))
      return@post
    }
    val link = Link(UUID.randomUUID().toString())
    links[link.id] = link
    link.connected = { r -> done(r.map { link.id to it }) }
    // TRANSPORT_LE and autoConnect=false: a direct, unbonded LE connection.
    link.gatt = device.connectGatt(context, false, gattCallback(link), BluetoothDevice.TRANSPORT_LE, 0, ble.handler)
    ble.handler.postDelayed({
      link.connected?.let {
        link.connected = null
        link.gatt?.close()
        links.remove(link.id)
        it(Result.failure(IllegalStateException("the table did not answer")))
      }
    }, 10_000)
  }

  private fun gattCallback(link: Link) = object : BluetoothGattCallback() {
    override fun onConnectionStateChange(gatt: BluetoothGatt, status: Int, newState: Int) {
      if (newState == BluetoothProfile.STATE_CONNECTED) {
        gatt.requestMtu(517)
      } else if (newState == BluetoothProfile.STATE_DISCONNECTED) {
        gatt.close()
        if (links.remove(link.id) != null) {
          link.connected?.invoke(Result.failure(IllegalStateException("disconnected")))
          link.connected = null
          onClosed(link.id)
        }
      }
    }

    override fun onMtuChanged(gatt: BluetoothGatt, mtu: Int, status: Int) {
      if (status == BluetoothGatt.GATT_SUCCESS) link.mtu = mtu
      gatt.discoverServices()
    }

    override fun onServicesDiscovered(gatt: BluetoothGatt, status: Int) {
      val service = gatt.getService(NearbyBleIds.SERVICE)
      val h2c = service?.getCharacteristic(NearbyBleIds.H2C)
      link.c2h = service?.getCharacteristic(NearbyBleIds.C2H)
      if (h2c == null || link.c2h == null) {
        fail(link, "not a Zolik table")
        return
      }
      gatt.setCharacteristicNotification(h2c, true)
      val cccd = h2c.getDescriptor(NearbyBleIds.CCCD)
      if (Build.VERSION.SDK_INT >= 33) {
        gatt.writeDescriptor(cccd, BluetoothGattDescriptor.ENABLE_NOTIFICATION_VALUE)
      } else {
        @Suppress("DEPRECATION")
        cccd.value = BluetoothGattDescriptor.ENABLE_NOTIFICATION_VALUE
        @Suppress("DEPRECATION")
        gatt.writeDescriptor(cccd)
      }
    }

    override fun onDescriptorWrite(gatt: BluetoothGatt, descriptor: BluetoothGattDescriptor, status: Int) {
      // Notifications are on: read the info characteristic, which says which
      // table this is before a single tunnel message is spent.
      val info = gatt.getService(NearbyBleIds.SERVICE)?.getCharacteristic(NearbyBleIds.INFO)
      if (status != BluetoothGatt.GATT_SUCCESS || info == null || !gatt.readCharacteristic(info)) {
        fail(link, "not a Zolik table")
      }
    }

    @Deprecated("pre-33 read callback")
    override fun onCharacteristicRead(gatt: BluetoothGatt, c: BluetoothGattCharacteristic, status: Int) {
      @Suppress("DEPRECATION")
      read(c, c.value ?: ByteArray(0), status)
    }

    override fun onCharacteristicRead(gatt: BluetoothGatt, c: BluetoothGattCharacteristic, value: ByteArray, status: Int) {
      read(c, value, status)
    }

    private fun read(c: BluetoothGattCharacteristic, value: ByteArray, status: Int) {
      if (c.uuid != NearbyBleIds.INFO) return
      val done = link.connected ?: return
      link.connected = null
      if (status == BluetoothGatt.GATT_SUCCESS) done(Result.success(value)) else fail(link, "could not read the table")
    }

    @Deprecated("pre-33 notification callback")
    override fun onCharacteristicChanged(gatt: BluetoothGatt, c: BluetoothGattCharacteristic) {
      @Suppress("DEPRECATION")
      changed(c.value ?: return)
    }

    override fun onCharacteristicChanged(gatt: BluetoothGatt, c: BluetoothGattCharacteristic, value: ByteArray) {
      changed(value)
    }

    // The host's service went away under a live link: its table closed, or
    // it restarted Bluetooth. Nothing on this link can work any more, so drop
    // it, and reconnection finds the table again if it comes back.
    override fun onServiceChanged(gatt: BluetoothGatt) {
      gatt.disconnect()
    }

    private fun changed(value: ByteArray) {
      link.reassembler.feed(value).forEach { onMessage(link.id, it) }
    }

    override fun onCharacteristicWrite(gatt: BluetoothGatt, c: BluetoothGattCharacteristic, status: Int) {
      link.writing = false
      if (status != BluetoothGatt.GATT_SUCCESS) {
        gatt.disconnect()
        return
      }
      link.outbox.removeFirstOrNull()?.second?.invoke()
      pump(link)
    }
  }

  fun send(linkId: String, msg: ByteArray, done: (Throwable?) -> Unit) = ble.post {
    val link = links[linkId]
    if (link == null) {
      done(IllegalStateException("the link to the table is closed"))
      return@post
    }
    val chunks = frameChunks(msg, chunkSize(link.mtu))
    chunks.forEachIndexed { i, c -> link.outbox.addLast(c to if (i == chunks.lastIndex) ({ done(null) }) else null) }
    pump(link)
  }

  /** One write at a time, each acknowledged before the next: lossless, in order. */
  private fun pump(link: Link) {
    if (link.writing) return
    val (chunk, _) = link.outbox.firstOrNull() ?: return
    val gatt = link.gatt ?: return
    val c = link.c2h ?: return
    link.writing = true
    val ok = if (Build.VERSION.SDK_INT >= 33) {
      gatt.writeCharacteristic(c, chunk, BluetoothGattCharacteristic.WRITE_TYPE_DEFAULT) == BluetoothGatt.GATT_SUCCESS
    } else {
      @Suppress("DEPRECATION")
      c.writeType = BluetoothGattCharacteristic.WRITE_TYPE_DEFAULT
      @Suppress("DEPRECATION")
      c.value = chunk
      @Suppress("DEPRECATION")
      gatt.writeCharacteristic(c)
    }
    if (!ok) {
      link.writing = false
      // As on the host: busy is retried, a refusal that lasts is a dead link.
      if (++link.refusals > MAX_REFUSALS) {
        gatt.disconnect()
        return
      }
      ble.handler.postDelayed({ pump(link) }, 10)
    } else {
      link.refusals = 0
    }
  }

  fun disconnect(linkId: String) = ble.post { links[linkId]?.gatt?.disconnect() }

  private fun fail(link: Link, why: String) {
    link.connected?.invoke(Result.failure(IllegalStateException(why)))
    link.connected = null
    link.gatt?.disconnect()
  }

  /**
   * Whether Bluetooth play is possible here. Android 8 is the floor: before
   * it, a GATT client's callbacks cannot be put on this module's thread, and
   * every link would need its own locking to match.
   */
  fun state(): String {
    if (Build.VERSION.SDK_INT < 26) return "unsupported"
    val adapter: BluetoothAdapter = manager.adapter ?: return "unsupported"
    return if (adapter.isEnabled) "on" else "off"
  }
}
