package com.jokerless.zoliknearby

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.os.Build
import android.os.PowerManager
import com.jokerless.zolikcore.Zolikcore

/**
 * Tells the embedded host how hot the phone is, so its bot governor stops
 * asking a throttled core for the dearest decisions (server/internal/botgov).
 *
 * Android reports heat as PowerManager's thermal status (API 29+) and calls a
 * listener when it changes; Battery Saver is the person asking the phone to do
 * less, and counts as "serious". Levels as Go reads them: 0 nominal or fair,
 * 1 serious, 2 critical.
 */
class NearbyThermal(private val context: Context) {
  private val power = context.getSystemService(Context.POWER_SERVICE) as PowerManager
  private var thermalListener: PowerManager.OnThermalStatusChangedListener? = null
  private var saverReceiver: BroadcastReceiver? = null

  /** Starts watching, and reports the state now. Safe to call again. */
  fun start() {
    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q && thermalListener == null) {
      val listener = PowerManager.OnThermalStatusChangedListener { report() }
      power.addThermalStatusListener(context.mainExecutor, listener)
      thermalListener = listener
    }
    if (saverReceiver == null) {
      val receiver = object : BroadcastReceiver() {
        override fun onReceive(c: Context?, i: Intent?) = report()
      }
      context.registerReceiver(receiver, IntentFilter(PowerManager.ACTION_POWER_SAVE_MODE_CHANGED))
      saverReceiver = receiver
    }
    report()
  }

  fun stop() {
    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
      thermalListener?.let { power.removeThermalStatusListener(it) }
    }
    thermalListener = null
    saverReceiver?.let { runCatching { context.unregisterReceiver(it) } }
    saverReceiver = null
  }

  fun level(): Long {
    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
      when (power.currentThermalStatus) {
        PowerManager.THERMAL_STATUS_CRITICAL,
        PowerManager.THERMAL_STATUS_EMERGENCY,
        PowerManager.THERMAL_STATUS_SHUTDOWN -> return 2
        PowerManager.THERMAL_STATUS_SEVERE -> return 1
      }
    }
    return if (power.isPowerSaveMode) 1 else 0
  }

  private fun report() {
    Zolikcore.current()?.setThermalPressure(level())
  }
}
