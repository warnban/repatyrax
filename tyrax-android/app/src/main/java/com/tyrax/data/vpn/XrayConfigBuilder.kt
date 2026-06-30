package com.tyrax.data.vpn

import org.json.JSONArray
import org.json.JSONObject

/**
 * Builds the Xray-core JSON config consumed by libv2ray's CoreController.
 *
 * Topology: a local SOCKS5 inbound (127.0.0.1:[VlessConfig.socksPort]) feeds a
 * VLESS outbound secured with XTLS-Reality over TCP. A tun2socks bridge pumps
 * the VpnService TUN device into the SOCKS inbound; everything else exits via
 * the `freedom` (direct) outbound.
 *
 * Kept identical in spirit to the backend's GenerateVlessConfig so server and
 * client stay in lockstep, but the client owns the inbound so it can pin the
 * SOCKS port and flow.
 */
object XrayConfigBuilder {

    fun build(cfg: VlessConfig): String {
        val root = JSONObject()

        root.put("log", JSONObject().put("loglevel", "warning"))

        // ── Inbound: local SOCKS5 ────────────────────────────────────────────
        val socksInbound = JSONObject()
            .put("tag", "socks-in")
            .put("listen", "127.0.0.1")
            .put("port", cfg.socksPort)
            .put("protocol", "socks")
            .put("settings", JSONObject().put("auth", "noauth").put("udp", true))
            .put(
                "sniffing",
                JSONObject()
                    .put("enabled", true)
                    .put("destOverride", JSONArray(listOf("http", "tls"))),
            )
        root.put("inbounds", JSONArray(listOf(socksInbound)))

        // ── Outbound: VLESS + Reality ────────────────────────────────────────
        val user = JSONObject()
            .put("id", cfg.userUuid)
            .put("encryption", "none")
            .put("flow", cfg.flow) // "" → XHTTP profile; "xtls-rprx-vision" → stream-one

        val vnext = JSONObject()
            .put("address", cfg.nodeHost)
            .put("port", cfg.nodePort)
            .put("users", JSONArray(listOf(user)))

        // XHTTP defeats the behavioural-DPI layer by splitting the tunnel into
        // normal HTTP request/response transactions; xPaddingBytes normalises
        // packet sizes. TCP stays available as the legacy transport.
        val streamSettings = JSONObject()
            .put("network", cfg.network)
            .put("security", cfg.security)

        if (cfg.security == "tls") {
            // CDN profile: real TLS on a Cloudflare-proxied domain (hides origin IP).
            val sni = cfg.realitySNI.ifEmpty { cfg.nodeHost }
            streamSettings.put(
                "tlsSettings",
                JSONObject()
                    .put("serverName", sni)
                    .put("fingerprint", cfg.fingerprint)
                    .put("allowInsecure", false)
                    .put("alpn", JSONArray(listOf("h2", "http/1.1"))),
            )
        } else {
            streamSettings.put(
                "realitySettings",
                JSONObject()
                    .put("show", false)
                    .put("fingerprint", cfg.fingerprint)
                    .put("serverName", cfg.realitySNI)
                    .put("publicKey", cfg.realityPublicKey)
                    .put("shortId", cfg.realityShortId)
                    .put("spiderX", ""), // empty — must match the node's Reality config
            )
        }

        if (cfg.network == "xhttp") {
            // XTLS-Vision over XHTTP only works with single-connection stream-one.
            val mode = if (cfg.flow == "xtls-rprx-vision") "stream-one" else cfg.xhttpMode
            val xhttpSettings = JSONObject()
                .put("path", cfg.xhttpPath)
                .put("mode", mode)
                .put("extra", JSONObject().put("xPaddingBytes", cfg.xPaddingBytes))
            // For the CDN/TLS profile, the Host header must be the proxied domain.
            if (cfg.security == "tls") xhttpSettings.put("host", cfg.nodeHost)
            streamSettings.put("xhttpSettings", xhttpSettings)
        }

        val proxyOutbound = JSONObject()
            .put("tag", "proxy")
            .put("protocol", "vless")
            .put("settings", JSONObject().put("vnext", JSONArray(listOf(vnext))))
            .put("streamSettings", streamSettings)

        val directOutbound = JSONObject()
            .put("tag", "direct")
            .put("protocol", "freedom")
            .put("settings", JSONObject())

        val blockOutbound = JSONObject()
            .put("tag", "block")
            .put("protocol", "blackhole")
            .put("settings", JSONObject())

        root.put("outbounds", JSONArray(listOf(proxyOutbound, directOutbound, blockOutbound)))

        // Routing: keep private / LAN ranges direct so the tunnel never tries to
        // proxy local traffic. Explicit CIDRs are used instead of "geoip:private"
        // because geoip.dat is not bundled — referencing it would make the core
        // fail to load this config.
        val privateRoute = JSONObject()
            .put("type", "field")
            .put("outboundTag", "direct")
            .put(
                "ip",
                JSONArray(
                    listOf(
                        "10.0.0.0/8",
                        "100.64.0.0/10",
                        "127.0.0.0/8",
                        "169.254.0.0/16",
                        "172.16.0.0/12",
                        "192.168.0.0/16",
                        "::1/128",
                        "fc00::/7",
                        "fe80::/10",
                    ),
                ),
            )
        root.put(
            "routing",
            JSONObject()
                .put("domainStrategy", "AsIs")
                .put("rules", JSONArray(listOf(privateRoute))),
        )

        return root.toString()
    }
}
