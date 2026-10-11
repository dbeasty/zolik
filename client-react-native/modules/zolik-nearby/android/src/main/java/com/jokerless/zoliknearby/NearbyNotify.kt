package com.jokerless.zoliknearby

import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.net.Uri
import android.os.Build
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import com.jokerless.zolikcore.Notifier

/**
 * The phone's own notification for a table it hosts: somebody sat down.
 *
 * A phone hosting a table has no internet to push through, so the embedded
 * server tells its owner itself (zolikcore.Notifier). Shown only while the
 * app is not in front; in front, the app's banner says it. A tap opens the
 * table through the app's own link scheme. Nothing is shown without the
 * notification permission the app asked for.
 */
internal class JoinNotifier(private val context: Context) : Notifier {
  companion object {
    @Volatile var inFront = true
    private const val CHANNEL = "table_events"
  }

  override fun notify(tag: String?, title: String?, body: String?, url: String?) {
    if (inFront) return
    val manager = NotificationManagerCompat.from(context)
    if (!manager.areNotificationsEnabled()) return
    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
      val system = context.getSystemService(NotificationManager::class.java)
      if (system.getNotificationChannel(CHANNEL) == null) {
        system.createNotificationChannel(
          NotificationChannel(CHANNEL, "Players at your table", NotificationManager.IMPORTANCE_HIGH),
        )
      }
    }
    val path = (url ?: "/").trimStart('/')
    val open = Intent(Intent.ACTION_VIEW, Uri.parse("clientreactnative://$path")).setPackage(context.packageName)
    val pending = PendingIntent.getActivity(
      context, (tag ?: path).hashCode(), open,
      PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
    )
    val notification = NotificationCompat.Builder(context, CHANNEL)
      .setSmallIcon(context.applicationInfo.icon)
      .setContentTitle(title ?: "")
      .setContentText(body ?: "")
      .setContentIntent(pending)
      .setAutoCancel(true)
      .setPriority(NotificationCompat.PRIORITY_HIGH)
      .build()
    try {
      manager.notify(tag ?: path, 0, notification)
    } catch (e: SecurityException) {
      // Permission withdrawn since the check above.
    }
  }
}
