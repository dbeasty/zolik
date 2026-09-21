package com.jokerless.zoliknearby

import com.jokerless.zolikcore.Host
import com.jokerless.zolikcore.Zolikcore
import expo.modules.kotlin.exception.CodedException
import expo.modules.kotlin.modules.Module
import expo.modules.kotlin.modules.ModuleDefinition
import java.io.File

/**
 * The embedded game server, as JS sees it.
 *
 * This is a thin layer: Go owns the host's lifetime and keeps one per
 * process (see server/mobile/zolikcore), so a JS reload that calls startHost
 * again gets the host that is already running.
 */
class ZolikNearbyModule : Module() {
  override fun definition() = ModuleDefinition {
    Name("ZolikNearby")

    AsyncFunction("startHost") { lan: Boolean ->
      val context = appContext.reactContext ?: throw HostException("no application context")
      val dir = File(context.filesDir, "zolik-host")
      val host = try {
        Zolikcore.start(dir.absolutePath, lan)
      } catch (e: Exception) {
        throw HostException(e.message ?: "the host did not start")
      }
      describe(host)
    }

    AsyncFunction("stopHost") {
      Zolikcore.current()?.stop()
    }

    Function("hostStatus") {
      Zolikcore.current()?.let { describe(it) }
    }
  }

  private fun describe(host: Host): Map<String, Any> = mapOf(
    "port" to host.port(),
    "baseUrl" to host.baseURL(),
    "instanceId" to host.instanceID(),
    "lan" to host.lan(),
  )
}

internal class HostException(detail: String) :
  CodedException("Could not start the offline table: $detail")
