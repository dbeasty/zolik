package com.jokerless.zoliknearby

import android.content.Context
import android.net.nsd.NsdManager
import android.net.nsd.NsdServiceInfo
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

    Events("onHostFound", "onHostLost")

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

    AsyncFunction("stopHost") {
      nearby?.unpublish()
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
        @Suppress("DEPRECATION")
        val address = (info.host as? Inet4Address)?.hostAddress
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

  companion object {
    const val TYPE = "_zolik._tcp"
  }
}

internal class HostException(detail: String) :
  CodedException("Could not start the offline table: $detail")
