package com.tyrax.data.vpn

import org.json.JSONArray
import org.json.JSONObject

/**
 * Patches the raw Xray JSON from POST /vpn/connect before [CoreController.startLoop].
 *
 * Topology (proven DoH ~200 ms through the live VLESS link):
 *   UDP :53 → dns-out → DoH via proxy (TCP, not raw UDP over VLESS)
 *   App TCP to resolved real IPs → proxy (no FakeDNS)
 *   TLS/HTTP sniffing on so Chrome SNI still reaches the node when apps connect by IP.
 *
 * Transport-agnostic: the proxy outbound's `streamSettings` (network/xhttpSettings)
 * and the VLESS `flow` arrive fully formed from the backend generator and are left
 * untouched here, so the XHTTP + Reality (+ optional Vision) profile survives intact.
 * Only DNS, routing and packetEncoding are normalised below.
 */
object XrayConfigPatcher {

    /** Excludes 10.0.0.0/8 — TUN lives at 10.10.0.x and must not bypass the tunnel. */
    private val PRIVATE_CIDRS = listOf(
        "127.0.0.0/8",
        "169.254.0.0/16",
        "172.16.0.0/12",
        "192.168.0.0/16",
        "::1/128",
        "fc00::/7",
        "fe80::/10",
    )

    fun enhance(rawConfigJson: String, logDir: String? = null): String {
        val root = JSONObject(rawConfigJson)

        if (logDir != null) root.put("log", buildLog(logDir))
        root.remove("fakedns")
        root.put("dns", buildDns())

        enhanceInbounds(root.optJSONArray("inbounds"))

        val outbounds = root.optJSONArray("outbounds") ?: JSONArray().also { root.put("outbounds", it) }
        setPacketEncoding(outbounds)
        tuneXhttpMux(outbounds)
        ensureDnsOutbound(outbounds)

        root.put(
            "routing",
            JSONObject()
                .put("domainStrategy", "AsIs")
                .put("rules", buildRoutingRules()),
        )

        return root.toString()
    }

    private fun buildLog(logDir: String): JSONObject =
        JSONObject()
            .put("loglevel", "warning")
            .put("access", "$logDir/xray_access.log")
            .put("error", "$logDir/xray_error.log")

    private fun buildDns(): JSONObject =
        JSONObject()
            .put(
                "servers",
                JSONArray()
                    .put("https://1.1.1.1/dns-query")
                    .put("https://8.8.8.8/dns-query"),
            )
            .put("queryStrategy", "UseIPv4")

    private fun enhanceInbounds(inbounds: JSONArray?) {
        if (inbounds == null) return
        for (i in 0 until inbounds.length()) {
            val inbound = inbounds.optJSONObject(i) ?: continue
            if (inbound.optString("protocol") != "socks") continue
            inbound.optJSONObject("settings")?.put("udp", true)
            inbound.put(
                "sniffing",
                JSONObject()
                    .put("enabled", true)
                    .put("destOverride", JSONArray(listOf("http", "tls")))
                    .put("routeOnly", false),
            )
        }
    }

    private fun setPacketEncoding(outbounds: JSONArray) {
        for (i in 0 until outbounds.length()) {
            val outbound = outbounds.optJSONObject(i) ?: continue
            if (outbound.optString("protocol") != "vless") continue
            val vnext = outbound.optJSONObject("settings")?.optJSONArray("vnext") ?: continue
            for (j in 0 until vnext.length()) {
                val users = vnext.optJSONObject(j)?.optJSONArray("users") ?: continue
                for (k in 0 until users.length()) {
                    users.optJSONObject(k)?.put("packetEncoding", "xudp")
                }
            }
        }
    }

    /**
     * Forces XHTTP to reuse a SINGLE multiplexed connection to the node instead of
     * spawning dozens. On mobile carriers, many parallel TLS connections to one
     * foreign datacenter IP (a) self-congest the radio link and (b) fingerprint as
     * a VPN, so the carrier throttles them to a crawl (observed: 70+ conns collapsing
     * to cwnd=1 with heavy retransmit). One persistent H2 connection carrying all
     * streams looks like a browser talking to apple.com and survives.
     */
    private fun tuneXhttpMux(outbounds: JSONArray) {
        for (i in 0 until outbounds.length()) {
            val outbound = outbounds.optJSONObject(i) ?: continue
            val stream = outbound.optJSONObject("streamSettings") ?: continue
            if (stream.optString("network") != "xhttp") continue
            val xhttp = stream.optJSONObject("xhttpSettings") ?: continue
            val extra = xhttp.optJSONObject("extra") ?: JSONObject().also { xhttp.put("extra", it) }
            extra.put(
                "xmux",
                JSONObject()
                    .put("maxConcurrency", 0)
                    .put("maxConnections", 1)
                    .put("cMaxReuseTimes", 0)
                    .put("hMaxRequestTimes", "1000-5000")
                    .put("hMaxReusableSecs", "1800-3000")
                    .put("hKeepAlivePeriod", 0),
            )
        }
    }

    private fun ensureDnsOutbound(outbounds: JSONArray) {
        for (i in 0 until outbounds.length()) {
            if (outbounds.optJSONObject(i)?.optString("tag") == "dns-out") return
        }
        outbounds.put(
            JSONObject()
                .put("tag", "dns-out")
                .put("protocol", "dns"),
        )
    }

    private fun buildRoutingRules(): JSONArray {
        val rules = JSONArray()

        rules.put(
            JSONObject()
                .put("type", "field")
                .put("outboundTag", "dns-out")
                .put("port", "53"),
        )

        rules.put(
            JSONObject()
                .put("type", "field")
                .put("outboundTag", "direct")
                .put("ip", JSONArray(PRIVATE_CIDRS)),
        )

        rules.put(
            JSONObject()
                .put("type", "field")
                .put("outboundTag", "proxy")
                .put("network", "tcp,udp"),
        )

        return rules
    }
}
