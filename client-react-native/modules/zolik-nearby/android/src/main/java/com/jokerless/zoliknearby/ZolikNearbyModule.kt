package com.jokerless.zoliknearby

import android.content.Context
import android.net.nsd.NsdManager
import android.net.nsd.NsdServiceInfo
import android.os.Build
import android.os.Handler
import android.os.Looper
import com.jokerless.zolikcore.Host
import com.jokerless.zolikcore.Zolikcore
import expo.modules.kotlin.exception.CodedException
import expo.modules.kotlin.modules.Module
import expo.modules.kotlin.modules.ModuleDefinition
import java.io.File
import java.net.Inet4Address
import java.net.NetworkInterface

/**
 * The embedded game server, and the local network around it, as JS sees it.
 *
 * Go owns the host's lifetime and keeps one per process (see
 * server/mobile/zolikcore), so a JS reload that calls startHost again gets
 * the host that is already running. This side owns what Go cannot do on a
 * phone: advertising the table over NSD, finding other tables, and reading
 * the phone's own addresses. Go's own interface listing is refused by
 * Android's netlink restrictions.
 */
class ZolikNearbyModule : Module() {
  private var nearby: NearbyNsd? = null
  private var bleHost: NearbyBleHost? = null
  private var bleGuest: NearbyBleGuest? = null

  private fun context(): android.content.Context =
    appContext.reactContext ?: throw HostException("no application context")

  private fun host(): NearbyBleHost =
    bleHost ?: NearbyBleHost(context()) { sendEvent("onBleGuests", mapOf("count" to it)) }.also { bleHost = it }

  private fun guest(): NearbyBleGuest =
    bleGuest ?: NearbyBleGuest(
      context(),
      onFound = { sendEvent("onBleFound", it) },
      onMessage = { id, data ->
        sendEvent("onBleMessage", mapOf("linkId" to id, "data" to android.util.Base64.encodeToString(data, android.util.Base64.NO_WRAP)))
      },
      onClosed = { sendEvent("onBleClosed", mapOf("linkId" to it)) },
    ).also { bleGuest = it }

  private fun nsd(): NearbyNsd {
    nearby?.let { return it }
    val context = appContext.reactContext ?: throw HostException("no application context")
    return NearbyNsd(
      context,
      onFound = { sendEvent("onHostFound", it) },
      onLost = { sendEvent("onHostLost", mapOf("name" to it)) },
    ).also { nearby = it }
  }

  override fun definition() = ModuleDefinition {
    Name("ZolikNearby")

    Events("onHostFound", "onHostLost", "onBleFound", "onBleMessage", "onBleClosed", "onBleGuests")

    // Bluetooth, host side: advertise the table and serve guests through the
    // Go tunnel. The Go host must already be running.
    AsyncFunction("bleHostStart") { name: String ->
      Zolikcore.current() ?: throw HostException("no host is running")
      host().start(name)
    }

    AsyncFunction("bleHostStop") {
      bleHost?.stop()
    }

    // Bluetooth, guest side.
    AsyncFunction("bleScanStart") {
      guest().scan()
    }

    AsyncFunction("bleScanStop") {
      bleGuest?.stopScan()
    }

    AsyncFunction("bleConnect") { peripheralId: String, promise: expo.modules.kotlin.Promise ->
      guest().connect(peripheralId) { result ->
        result.fold(
          { (linkId, info) -> promise.resolve(mapOf("linkId" to linkId, "info" to String(info, Charsets.UTF_8))) },
          { promise.reject("BLE_CONNECT", it.message ?: "could not connect", it) },
        )
      }
    }

    AsyncFunction("bleSend") { linkId: String, base64: String, promise: expo.modules.kotlin.Promise ->
      val data = try {
        android.util.Base64.decode(base64, android.util.Base64.NO_WRAP)
      } catch (e: IllegalArgumentException) {
        promise.reject("BLE_SEND", "not base64", e)
        return@AsyncFunction
      }
      guest().send(linkId, data) { err ->
        if (err != null) promise.reject("BLE_SEND", err.message ?: "send failed", err) else promise.resolve(null)
      }
    }

    AsyncFunction("bleDisconnect") { linkId: String ->
      bleGuest?.disconnect(linkId)
    }

    Function("bleGuestCodes") { ->
      bleHost?.codes() ?: emptyList<String>()
    }

    // Randomness for the tunnel's ephemeral keys, from the system's secure
    // generator. JS has no crypto.getRandomValues under Hermes.
    Function("randomBytes") { count: Int ->
      val bytes = ByteArray(count.coerceIn(0, 1024))
      java.security.SecureRandom().nextBytes(bytes)
      android.util.Base64.encodeToString(bytes, android.util.Base64.NO_WRAP)
    }

    // Android knows its radio's state at once; iOS may have to ask first.
    AsyncFunction("bleReady") {
      guest().state()
    }

    Function("bleState") {
      guest().state()
    }

    AsyncFunction("startHost") {
      val context = appContext.reactContext ?: throw HostException("no application context")
      val dir = File(context.filesDir, "zolik-host")
      val host = try {
        Zolikcore.start(dir.absolutePath)
      } catch (e: Exception) {
        throw HostException(e.message ?: "the host did not start")
      }
      describe(host)
    }

    // The same host, for a phone that has been enrolled and has somebody
    // signed in: it additionally replicates that account's data and serves it
    // back, so the app can show a person their own things with no connection.
    AsyncFunction("startNode") { credential: String, userHex: String ->
      val context = appContext.reactContext ?: throw HostException("no application context")
      val dir = File(context.filesDir, "zolik-host")
      val host = try {
        Zolikcore.startNode(dir.absolutePath, credential, userHex)
      } catch (e: Exception) {
        throw HostException(e.message ?: "the host did not start")
      }
      describe(host)
    }

    // This install's node identity: what the cloud enrols, and the key half
    // it is enrolled by. The private half never leaves the phone.
    Function("nodeIdentity") {
      Zolikcore.current()?.let { mapOf("nodeId" to it.nodeID(), "publicKey" to it.nodePublicKey()) }
    }

    // Replicate now rather than at the next tick, for the moments where
    // waiting would be visible: coming back to the app, the network
    // returning, a match ending.
    AsyncFunction("syncNow") {
      Zolikcore.current()?.syncNow()
    }

    // Whether this device is holding the signed-in account's data yet, so the
    // app can tell "nothing synced" from "you have never played".
    Function("replicaReady") {
      Zolikcore.current()?.replicaReady() ?: false
    }

    // Starts following a match that was begun somewhere else, so this device
    // can open it.
    AsyncFunction("followMatch") { matchId: String ->
      val host = Zolikcore.current() ?: throw HostException("no host is running")
      host.followMatch(matchId)
    }

    AsyncFunction("stopHost") {
      nearby?.unpublish()
      bleHost?.stop()
      Zolikcore.current()?.stop()
    }

    Function("hostStatus") {
      Zolikcore.current()?.let { describe(it) }
    }

    AsyncFunction("openRoom") { name: String ->
      val host = Zolikcore.current() ?: throw HostException("no host is running")
      val port = try {
        host.openLAN()
      } catch (e: Exception) {
        throw HostException(e.message ?: "could not open the table to the room")
      }
      nsd().publish(
        name, port.toInt(),
        mapOf("v" to Zolikcore.ProtocolVersion.toString(), "id" to host.instanceID(), "n" to name),
      )
      mapOf("port" to port, "addresses" to ipv4Addresses())
    }

    AsyncFunction("closeRoom") {
      nearby?.unpublish()
      Zolikcore.current()?.closeLAN()
    }

    AsyncFunction("startBrowsing") {
      nsd().browse()
    }

    AsyncFunction("stopBrowsing") {
      nearby?.stopBrowsing()
    }

    Function("localAddresses") {
      ipv4Addresses()
    }

    OnDestroy {
      nearby?.unpublish()
      nearby?.stopBrowsing()
      bleHost?.stop()
      bleGuest?.stopScan()
    }
  }

  private fun describe(host: Host): Map<String, Any> = mapOf(
    "port" to host.port(),
    "baseUrl" to host.baseURL(),
    "instanceId" to host.instanceID(),
    "lanPort" to host.lanPort(),
  )

  /** Wi-Fi and hotspot addresses a guest in the room could dial. */
  private fun ipv4Addresses(): List<String> =
    NetworkInterface.getNetworkInterfaces()?.toList().orEmpty()
      .filter { it.isUp && !it.isLoopback }
      .flatMap { it.inetAddresses.toList() }
      .filterIsInstance<Inet4Address>()
      .filter { it.isSiteLocalAddress }
      .map { it.hostAddress ?: "" }
      .filter { it.isNotEmpty() }
}

/**
 * NSD for `_zolik._tcp`: publishing this phone's table and finding the
 * others.
 *
 * Resolution is serialised. Before API 34, NsdManager fails a second
 * resolveService while one is in flight (FAILURE_ALREADY_ACTIVE), and a room
 * with three tables in it finds all three at once.
 */
internal class NearbyNsd(
  context: Context,
  private val onFound: (Map<String, Any>) -> Unit,
  private val onLost: (String) -> Unit,
) {
  private val manager = context.getSystemService(Context.NSD_SERVICE) as NsdManager
  private val main = Handler(Looper.getMainLooper())
  private var registration: NsdManager.RegistrationListener? = null
  private var registeredName: String? = null
  private var discovery: NsdManager.DiscoveryListener? = null
  private val queue = ArrayDeque<NsdServiceInfo>()
  private var resolving = false

  fun publish(name: String, port: Int, txt: Map<String, String>) {
    unpublish()
    val info = NsdServiceInfo().apply {
      serviceName = name
      serviceType = TYPE
      setPort(port)
      txt.forEach { (k, v) -> setAttribute(k, v) }
    }
    val listener = object : NsdManager.RegistrationListener {
      // NSD renames on a clash ("Name (2)"), and browsing has to recognise
      // the name it actually got to skip our own table.
      override fun onServiceRegistered(info: NsdServiceInfo) { registeredName = info.serviceName }
      override fun onRegistrationFailed(info: NsdServiceInfo, code: Int) {}
      override fun onServiceUnregistered(info: NsdServiceInfo) {}
      override fun onUnregistrationFailed(info: NsdServiceInfo, code: Int) {}
    }
    registration = listener
    manager.registerService(info, NsdManager.PROTOCOL_DNS_SD, listener)
  }

  fun unpublish() {
    registration?.let { runCatching { manager.unregisterService(it) } }
    registration = null
    registeredName = null
  }

  fun browse() {
    stopBrowsing()
    val listener = object : NsdManager.DiscoveryListener {
      override fun onDiscoveryStarted(type: String) {}
      override fun onDiscoveryStopped(type: String) {}
      override fun onStartDiscoveryFailed(type: String, code: Int) {}
      override fun onStopDiscoveryFailed(type: String, code: Int) {}
      override fun onServiceFound(info: NsdServiceInfo) {
        if (info.serviceName == registeredName) return
        main.post { queue.addLast(info); next() }
      }
      override fun onServiceLost(info: NsdServiceInfo) {
        main.post { onLost(info.serviceName) }
      }
    }
    discovery = listener
    manager.discoverServices(TYPE, NsdManager.PROTOCOL_DNS_SD, listener)
  }

  fun stopBrowsing() {
    discovery?.let { runCatching { manager.stopServiceDiscovery(it) } }
    discovery = null
    main.post { queue.clear() }
  }

  private fun next() {
    if (resolving) return
    val info = queue.removeFirstOrNull() ?: return
    resolving = true
    @Suppress("DEPRECATION")
    manager.resolveService(info, object : NsdManager.ResolveListener {
      override fun onResolveFailed(info: NsdServiceInfo, code: Int) {
        main.post { resolving = false; next() }
      }
      override fun onServiceResolved(info: NsdServiceInfo) {
        val address = ipv4Of(info)
        val txt = info.attributes.mapValues { (_, v) -> v?.let { String(it) } ?: "" }
        main.post {
          resolving = false
          if (address != null) {
            onFound(
              mapOf(
                "name" to info.serviceName,
                "address" to address,
                "port" to info.port,
                "id" to (txt["id"] ?: ""),
                "hostName" to (txt["n"] ?: info.serviceName),
                "protocol" to (txt["v"]?.toIntOrNull() ?: 0),
              ),
            )
          }
          next()
        }
      }
    })
  }

  /**
   * The host's IPv4 address. `host` is whichever address the resolver saw
   * first, and on the newer mDNS stack that is often an IPv6 one — the table
   * was then dropped as unreachable, so a room found it only some of the time.
   * API 34 lists every address; the first IPv4 among them is the one to use.
   */
  private fun ipv4Of(info: NsdServiceInfo): String? {
    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) {
      info.hostAddresses.firstOrNull { it is Inet4Address }?.let { return it.hostAddress }
    }
    @Suppress("DEPRECATION")
    return (info.host as? Inet4Address)?.hostAddress
  }

  companion object {
    const val TYPE = "_zolik._tcp"
  }
}

internal class HostException(detail: String) :
  CodedException("Could not start the offline table: $detail")
