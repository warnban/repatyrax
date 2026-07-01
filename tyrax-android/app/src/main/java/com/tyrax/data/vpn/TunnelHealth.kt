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
 * Liveness/throttle probe for the active Xray tunnel.
 *
 * [TyraxXrayVpnService] excludes our own package from the TUN, so a plain in-app
 * request would NOT traverse the tunnel. Instead we probe THROUGH the Xray SOCKS5
 * inbound (127.0.0.1:[socksPort]) — that exercises the real VLESS → node →
 * internet path. A success means the tunnel passes traffic; a failure or a very
 * slow response means the node is dead or being throttled by DPI.
 */
object TunnelHealth {

    private const val TAG = "TYRAX-Health"

    // A tiny "204 No Content" endpoint: minimal payload, widely reachable, fast.
    private const val PROBE_URL = "https://www.gstatic.com/generate_204"

    private const val TIMEOUT_SECONDS = 10L

    @Volatile
    private var client: OkHttpClient? = null

    private fun client(socksPort: Int): OkHttpClient =
        client ?: synchronized(this) {
            client ?: OkHttpClient.Builder()
                .proxy(Proxy(Proxy.Type.SOCKS, InetSocketAddress("127.0.0.1", socksPort)))
                .connectTimeout(TIMEOUT_SECONDS, TimeUnit.SECONDS)
                .readTimeout(TIMEOUT_SECONDS, TimeUnit.SECONDS)
                .callTimeout(TIMEOUT_SECONDS, TimeUnit.SECONDS)
                .retryOnConnectionFailure(false)
                .build()
                .also { client = it }
        }

    /** Probe result: [ok] reachability and the round-trip [elapsedMs] (Long.MAX on failure). */
    data class Result(val ok: Boolean, val elapsedMs: Long)

    suspend fun probe(socksPort: Int = 10808): Result = withContext(Dispatchers.IO) {
        val start = System.currentTimeMillis()
        try {
            val req = Request.Builder().url(PROBE_URL).get().build()
            client(socksPort).newCall(req).execute().use { resp ->
                val ok = resp.isSuccessful || resp.code == 204
                Result(ok, System.currentTimeMillis() - start)
            }
        } catch (e: Exception) {
            Log.d(TAG, "probe failed: ${e.message}")
            Result(false, Long.MAX_VALUE)
        }
    }
}
