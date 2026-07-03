package com.tyrax.data.vpn

import android.util.Log
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import okhttp3.OkHttpClient
import okhttp3.Request
import java.net.InetSocketAddress
import java.net.Proxy
import java.util.concurrent.TimeUnit

/**
 * Self-healing detector for RU services that are blocked when routed through the
 * foreign tunnel.
 *
 * For each marker it compares two paths:
 *  - DIRECT: a plain request from this process. Because [TyraxXrayVpnService] excludes
 *    the TYRAX package from the TUN, this exits over the phone's real (Russian) network.
 *  - PROXY: a request through the Xray SOCKS5 inbound (127.0.0.1:socksPort) — i.e. the
 *    real VLESS → foreign node path.
 *
 * "Blocked-through-VPN" = DIRECT works but PROXY fails. Those domains should bypass the
 * tunnel, so the caller adds them to the dynamic bypass set and they exit via RU next time.
 */
object SplitDiagnostics {

    private const val TAG = "TYRAX-SplitDiag"
    private const val TIMEOUT_SECONDS = 6L

    /** Small, representative RU marker set (geo-blocked backends). */
    val MARKERS: List<String> = listOf(
        "vk.com", "ozon.ru", "sberbank.ru", "max.ru", "wildberries.ru",
    )

    @Volatile private var directClient: OkHttpClient? = null
    @Volatile private var proxyClient: OkHttpClient? = null
    @Volatile private var proxyPort: Int = -1

    private fun baseBuilder() = OkHttpClient.Builder()
        .connectTimeout(TIMEOUT_SECONDS, TimeUnit.SECONDS)
        .readTimeout(TIMEOUT_SECONDS, TimeUnit.SECONDS)
        .callTimeout(TIMEOUT_SECONDS, TimeUnit.SECONDS)
        .retryOnConnectionFailure(false)

    private fun direct(): OkHttpClient =
        directClient ?: synchronized(this) {
            directClient ?: baseBuilder().build().also { directClient = it }
        }

    private fun proxy(socksPort: Int): OkHttpClient =
        proxyClient?.takeIf { proxyPort == socksPort } ?: synchronized(this) {
            baseBuilder()
                .proxy(Proxy(Proxy.Type.SOCKS, InetSocketAddress("127.0.0.1", socksPort)))
                .build()
                .also { proxyClient = it; proxyPort = socksPort }
        }

    private fun reachable(client: OkHttpClient, domain: String): Boolean = try {
        val req = Request.Builder().url("https://$domain/").get().build()
        client.newCall(req).execute().use { resp ->
            // Any HTTP response (even 403/redirect) proves the host is reachable on this path.
            resp.code in 200..599
        }
    } catch (e: Exception) {
        Log.d(TAG, "reachable($domain) failed: ${e.message}")
        false
    }

    /**
     * Probes [markers] not already in [alreadyBypassed] and returns those detected as
     * blocked-through-VPN (direct OK, proxy not OK). Never touches the tunnel.
     */
    suspend fun probeOnce(
        socksPort: Int,
        markers: List<String> = MARKERS,
        alreadyBypassed: Set<String> = emptySet(),
    ): List<String> = withContext(Dispatchers.IO) {
        val blocked = mutableListOf<String>()
        for (domain in markers) {
            if (alreadyBypassed.contains(domain)) continue
            val proxyOk = reachable(proxy(socksPort), domain)
            if (proxyOk) continue // reachable through the tunnel — no bypass needed
            val directOk = reachable(direct(), domain)
            if (directOk) {
                Log.d(TAG, "blocked-through-VPN detected: $domain (direct ok, proxy blocked)")
                blocked += domain
            }
        }
        blocked
    }
}
