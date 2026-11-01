package dev.kyouji.tunnelgo

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.net.VpnService
import android.os.Build
import android.os.ParcelFileDescriptor
import android.content.Intent
import android.content.pm.ServiceInfo
import hev.htproxy.TProxyService
import dev.kyouji.tunnelgo.mobile.Mobile
import java.io.File
import java.util.concurrent.Executors

class TunnelVpnService : VpnService() {
    private val executor = Executors.newSingleThreadExecutor()
    @Volatile private var tun: ParcelFileDescriptor? = null
    @Volatile private var started = false

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_STOP -> stopTunnel()
            ACTION_START -> startTunnel(intent)
            else -> stopSelf(startId)
        }
        return START_NOT_STICKY
    }

    private fun startTunnel(intent: Intent) {
        if (started) return
        started = true
        createNotificationChannel()
        val notification = buildNotification()
        if (Build.VERSION.SDK_INT >= 34) {
            startForeground(NOTIFICATION_ID, notification, ServiceInfo.FOREGROUND_SERVICE_TYPE_SPECIAL_USE)
        } else {
            startForeground(NOTIFICATION_ID, notification)
        }

        executor.execute {
            try {
                Mobile.startSSH(
                    intent.getStringExtra(EXTRA_ADDRESS).orEmpty(),
                    intent.getStringExtra(EXTRA_USERNAME).orEmpty(),
                    intent.getStringExtra(EXTRA_PASSWORD).orEmpty(),
                    "",
                    "",
                    "",
                    intent.getStringExtra(EXTRA_PROXY).orEmpty(),
                    intent.getStringExtra(EXTRA_SNI).orEmpty(),
                    intent.getStringExtra(EXTRA_PAYLOAD).orEmpty(),
                    "",
                    intent.getBooleanExtra(EXTRA_TLS, false),
                    intent.getBooleanExtra(EXTRA_INSECURE_TLS, false),
                    intent.getBooleanExtra(EXTRA_INSECURE_SSH, false),
                    intent.getBooleanExtra(EXTRA_WAIT_RESPONSE, false),
                    SOCKS_PORT.toInt()
                )

                val builder = Builder()
                    .setSession("Tunnel-Go")
                    .setMtu(1500)
                    .addAddress("198.18.0.1", 30)
                    .addRoute("0.0.0.0", 0)
                    .addDnsServer("1.1.1.1")
                    .addDisallowedApplication(packageName)

                if (Build.VERSION.SDK_INT >= 29) {
                    builder.addAddress("fd00::1", 126)
                    builder.addRoute("::", 0)
                }

                val descriptor = builder.establish() ?: error("VPN interface creation was denied")
                tun = descriptor
                val config = File(filesDir, "hev-tunnel.yml")
                config.writeText(
                    "tunnel:\n" +
                    "  mtu: 1500\n" +
                    "  ipv4: 198.18.0.1\n" +
                    "  ipv6: 'fd00::1'\n" +
                    "socks5:\n" +
                    "  address: 127.0.0.1\n" +
                    "  port: $SOCKS_PORT\n" +
                    "  udp: 'udp'\n" +
                    "misc:\n" +
                    "  log-level: warn\n" +
                    "  task-stack-size: 24576\n" +
                    "  tcp-buffer-size: 4096\n" +
                    "  max-session-count: 1200\n"
                )
                check(TProxyService.TProxyStartService(config.absolutePath, descriptor.fd)) {
                    "HEV failed to start"
                }
            } catch (error: Throwable) {
                stopTunnel()
            }
        }
    }

    private fun stopTunnel() {
        if (started) {
            started = false
            runCatching { TProxyService.TProxyStopService() }
            runCatching { tun?.close() }
            tun = null
            runCatching { Mobile.stop() }
        }
        stopForeground(STOP_FOREGROUND_REMOVE)
        stopSelf()
    }

    override fun onRevoke() {
        stopTunnel()
        super.onRevoke()
    }

    override fun onDestroy() {
        stopTunnel()
        executor.shutdownNow()
        super.onDestroy()
    }

    private fun createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= 26) {
            val channel = NotificationChannel(CHANNEL_ID, "VPN connection", NotificationManager.IMPORTANCE_LOW)
            getSystemService(NotificationManager::class.java).createNotificationChannel(channel)
        }
    }

    private fun buildNotification(): Notification {
        val pending = PendingIntent.getActivity(
            this, 0, Intent(this, MainActivity::class.java),
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )
        val builder = if (Build.VERSION.SDK_INT >= 26) {
            Notification.Builder(this, CHANNEL_ID)
        } else {
            Notification.Builder(this)
        }
        return builder
            .setContentTitle("Tunnel-Go")
            .setContentText("Establishing SSH tunnel")
            .setSmallIcon(android.R.drawable.stat_sys_warning)
            .setContentIntent(pending)
            .setOngoing(true)
            .build()
    }

    companion object {
        const val ACTION_START = "dev.kyouji.tunnelgo.START"
        const val ACTION_STOP = "dev.kyouji.tunnelgo.STOP"
        const val EXTRA_ADDRESS = "address"
        const val EXTRA_USERNAME = "username"
        const val EXTRA_PASSWORD = "password"
        const val EXTRA_PROXY = "proxy"
        const val EXTRA_SNI = "sni"
        const val EXTRA_PAYLOAD = "payload"
        const val EXTRA_TLS = "tls"
        const val EXTRA_INSECURE_TLS = "insecure_tls"
        const val EXTRA_INSECURE_SSH = "insecure_ssh"
        const val EXTRA_WAIT_RESPONSE = "wait_response"
        private const val CHANNEL_ID = "tunnel_connection"
        private const val NOTIFICATION_ID = 41
        private const val SOCKS_PORT = 1080
    }
}
