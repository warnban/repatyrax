package com.tyrax.data.vpn

/**
 * Connection parameters for a VLESS + Reality node, as returned by the backend
 * `/vpn/device` response when `protocol == "vless"`.
 *
 * [flow] is empty for the XHTTP profile (Profile A) and "xtls-rprx-vision" for
 * the stream-one Vision profile (Profile B); it must match the server user's
 * flow. [network] selects the Xray stream transport: "xhttp" (default,
 * behavioural-DPI resistant) or "tcp" (legacy). XHTTP fields are ignored when
 * [network] == "tcp".
 */
data class VlessConfig(
    val nodeHost: String,
    val nodePort: Int,
    val userUuid: String,
    val realityPublicKey: String,
    val realitySNI: String,
    val realityShortId: String = "",
    val flow: String = "",
    /** Stream security: "reality" (direct) or "tls" (CDN-fronted). */
    val security: String = "reality",
    val network: String = "xhttp",
    val xhttpPath: String = "/api/v1/data",
    val xhttpMode: String = "auto",
    val xPaddingBytes: String = "100-1000",
    val fingerprint: String = "chrome",
    /** Local SOCKS5 inbound the tun2socks bridge proxies through. */
    val socksPort: Int = 10808,
)
