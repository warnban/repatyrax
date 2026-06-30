package com.tyrax.data.vpn

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Intent
import android.content.pm.ServiceInfo
import android.net.ConnectivityManager
import android.net.VpnService
import android.os.Build
import android.os.ParcelFileDescriptor
import android.util.Log
import androidx.core.app.NotificationCompat
import androidx.core.app.ServiceCompat
import com.tyrax.MainActivity
import com.tyrax.R
import com.tyrax.domain.model.VpnState
import com.v2ray.ang.service.TProxyService
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.launch
import libv2ray.CoreCallbackHandler
import libv2ray.CoreController
import libv2ray.Libv2ray
import java.io.File

/**
 * VLESS + Reality data plane: libv2ray (SOCKS inbound) + hev-socks5-tunnel (TUN bridge).
 *
 * Startup order matters:
 *   1. Start Xray while NO VPN is active — outbound dials to the node use the physical
 *      network and cannot loop through a TUN that does not exist yet.
 *   2. Establish the TUN with addDisallowedApplication(self) so our process stays off it.
 *   3. Start hev to pump TUN packets into the already-listening SOCKS inbound.
 */
class TyraxXrayVpnService : VpnService() {

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)

    private var tunInterface: ParcelFileDescriptor? = null
    private var coreController: CoreController? = null
    private var tun2socksRunning: Boolean = false
    private var codename: String = "—"

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        Log.d(TAG, "onStartCommand action=${intent?.action} flags=$flags startId=$startId")
        when (intent?.action) {
            ACTION_DISCONNECT -> {
                stopTunnel()
                return START_NOT_STICKY
            }
            ACTION_CONNECT -> {
                val configJson = intent.getStringExtra(EXTRA_CONFIG_JSON)
                val socksPort = intent.getIntExtra(EXTRA_SOCKS_PORT, DEFAULT_SOCKS_PORT)
                codename = intent.getStringExtra(EXTRA_CODENAME) ?: "—"
                Log.d(TAG, "onStartCommand, config length=${configJson?.length ?: 0}")
                Log.d(TAG, "ACTION_CONNECT codename=$codename socksPort=$socksPort")
                if (configJson.isNullOrBlank()) {
                    fail("INVALID CONFIG")
                    return START_NOT_STICKY
                }
                startForegroundNotification()
                scope.launch { startTunnel(configJson, socksPort) }
            }
        }
        return START_STICKY
    }

    private fun startTunnel(configJson: String, socksPort: Int) {
        try {
            val logDir = (getExternalFilesDir(null) ?: filesDir).absolutePath
            runCatching { File(logDir, "xray_access.log").delete() }
            runCatching { File(logDir, "xray_error.log").delete() }
            val patchedConfig = XrayConfigPatcher.enhance(configJson, logDir)
            runCatching { File(logDir, "xray_config.json").writeText(patchedConfig) }

            // 1. Xray first — no TUN yet, node dial cannot loop.
            Libv2ray.initCoreEnv(filesDir.absolutePath, "")
            val controller = Libv2ray.newCoreController(CoreCallback())
            coreController = controller
            Log.d(TAG, "startLoop() before TUN, configLen=${patchedConfig.length}")
            controller.startLoop(patchedConfig, 0)
            if (!waitForXray(controller)) {
                fail("XRAY CORE FAILED TO START")
                stopTunnel()
                return
            }

            // 2. VPN interface — our package is excluded from the tunnel.
            val tun = establishTun() ?: run {
                fail("TUN ESTABLISH FAILED")
                stopTunnel()
                return
            }
            tunInterface = tun
            bindUnderlyingNetwork()

            // 3. hev bridge: TUN fd → SOCKS5 on loopback.
            Log.d(TAG, "startTun2Socks() hev, socksPort=$socksPort tunFd=${tun.fd}")
            startTun2Socks(tun.fd, socksPort)

            Log.d(TAG, "tunnel UP -> Connected node=$codename")
            VpnStateBus.state.value = VpnState.Connected(
                nodeCodename = codename,
                protocol = "vless",
                pingMs = 0,
            )
        } catch (e: Exception) {
            Log.e(TAG, "startTunnel failed", e)
            fail(e.message ?: "CONNECTION FAILED. NODE UNAVAILABLE.")
            stopTunnel()
        }
    }

    private fun establishTun(): ParcelFileDescriptor? {
        val builder = Builder()
            .setSession("TYRAX")
            .setMtu(TUN_MTU)
            .addAddress(TUN_ADDRESS, TUN_PREFIX)
            .addDnsServer(TUN_DNS)
            .addRoute("0.0.0.0", 0)

        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            builder.setMetered(false)
        }

        try {
            builder.addDisallowedApplication(packageName)
            Log.d(TAG, "addDisallowedApplication OK: $packageName")
        } catch (e: Exception) {
            Log.e(TAG, "addDisallowedApplication FAILED", e)
        }

        return builder.establish()
    }

    /**
     * One-shot bind to the physical network (Wi‑Fi/LTE). Avoids NetworkCallback —
     * that path crashed on some OEM builds in 0726-E/F.
     */
    private fun bindUnderlyingNetwork() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.LOLLIPOP_MR1) return
        val cm = getSystemService(ConnectivityManager::class.java) ?: return
        val network = cm.activeNetwork ?: run {
            Log.w(TAG, "bindUnderlyingNetwork: no active network")
            return
        }
        try {
            val ok = setUnderlyingNetworks(arrayOf(network))
            Log.d(TAG, "setUnderlyingNetworks($network) ok=$ok")
        } catch (e: Exception) {
            Log.e(TAG, "setUnderlyingNetworks failed", e)
        }
    }

    private fun startTun2Socks(tunFd: Int, socksPort: Int) {
        if (tun2socksRunning) {
            Log.w(TAG, "hev already running, stopping first")
            runCatching { TProxyService.TProxyStopService() }
            tun2socksRunning = false
        }
        val configPath = writeHevConfig(socksPort)
        TProxyService.TProxyStartService(configPath, tunFd)
        tun2socksRunning = true
    }

    private fun waitForXray(controller: CoreController): Boolean {
        repeat(30) {
            if (controller.isRunning) {
                Thread.sleep(200)
                Log.d(TAG, "xray core running")
                return true
            }
            Thread.sleep(100)
        }
        Log.e(TAG, "xray core did not start within timeout")
        return false
    }

    /** hev YAML layout matches v2rayNG TProxyService.buildConfig(). */
    private fun writeHevConfig(socksPort: Int): String {
        val yaml = buildString {
            appendLine("tunnel:")
            appendLine("  mtu: $TUN_MTU")
            appendLine("  ipv4: $TUN_ADDRESS")
            appendLine("socks5:")
            appendLine("  port: $socksPort")
            appendLine("  address: 127.0.0.1")
            appendLine("  udp: 'udp'")
            appendLine("misc:")
            appendLine("  log-level: warn")
            appendLine("  tcp-read-write-timeout: 300000")
            appendLine("  udp-read-write-timeout: 60000")
        }
        val file = File(filesDir, "hev-tunnel.yaml")
        file.writeText(yaml)
        return file.absolutePath
    }

    private fun stopTunnel() {
        scope.launch {
            if (tun2socksRunning) {
                runCatching { TProxyService.TProxyStopService() }
                tun2socksRunning = false
            }

            runCatching { coreController?.takeIf { it.isRunning }?.stopLoop() }
            coreController = null

            runCatching { tunInterface?.close() }
            tunInterface = null

            VpnStateBus.state.value = VpnState.Disconnected
            stopSelfSafely()
        }
    }

    private fun fail(message: String) {
        VpnStateBus.state.value = VpnState.Error(message)
    }

    override fun onRevoke() {
        stopTunnel()
        super.onRevoke()
    }

    override fun onDestroy() {
        scope.cancel()
        if (tun2socksRunning) {
            runCatching { TProxyService.TProxyStopService() }
            tun2socksRunning = false
        }
        runCatching { coreController?.takeIf { it.isRunning }?.stopLoop() }
        runCatching { tunInterface?.close() }
        super.onDestroy()
    }

    private fun stopSelfSafely() {
        ServiceCompat.stopForeground(this, ServiceCompat.STOP_FOREGROUND_REMOVE)
        stopSelf()
    }

    private inner class CoreCallback : CoreCallbackHandler {
        override fun startup(): Long {
            Log.d(TAG, "xray callback: startup")
            return 0
        }

        override fun shutdown(): Long {
            Log.d(TAG, "xray callback: shutdown")
            return 0
        }

        override fun onEmitStatus(l: Long, s: String?): Long {
            if (!s.isNullOrBlank()) Log.d(TAG, "xray: $s")
            return 0
        }
    }

    private fun startForegroundNotification() {
        val nm = getSystemService(NotificationManager::class.java)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            nm.createNotificationChannel(
                NotificationChannel(CHANNEL_ID, "TYRAX PROTOCOL", NotificationManager.IMPORTANCE_LOW),
            )
        }

        val pending = PendingIntent.getActivity(
            this, 0,
            Intent(this, MainActivity::class.java),
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )

        val notification = NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle("TYRAX")
            .setContentText("TUNNEL ACTIVE")
            .setSmallIcon(R.mipmap.ic_launcher)
            .setOngoing(true)
            .setContentIntent(pending)
            .build()

        val type = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) {
            ServiceInfo.FOREGROUND_SERVICE_TYPE_SPECIAL_USE
        } else {
            0
        }
        ServiceCompat.startForeground(this, NOTIF_ID, notification, type)
    }

    companion object {
        const val ACTION_CONNECT = "com.tyrax.xray.CONNECT"
        const val ACTION_DISCONNECT = "com.tyrax.xray.DISCONNECT"
        const val EXTRA_CONFIG_JSON = "xray_config"
        const val EXTRA_SOCKS_PORT = "socks_port"
        const val EXTRA_CODENAME = "codename"

        private const val TAG = "TYRAX-XraySvc"
        private const val CHANNEL_ID = "tyrax_protocol"
        private const val NOTIF_ID = 0x7A
        private const val DEFAULT_SOCKS_PORT = 10808

        private const val TUN_MTU = 1500
        private const val TUN_ADDRESS = "10.10.0.2"
        private const val TUN_DNS = "1.1.1.1"
        private const val TUN_PREFIX = 30
    }
}
