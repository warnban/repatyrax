package com.tyrax.domain.model

data class Device(
    val id: String,
    val name: String,
    val publicKey: String? = null,
)

/** Returned when a device is provisioned: carries the connectable config + node list. */
data class DeviceConfig(
    val deviceId: String,
    val protocol: String?,        // "wireguard" | "vless"
    val wireGuardConf: String?,
    val vlessConf: String?,
    // Structured VLESS + Reality params (present when protocol == "vless").
    val uuid: String? = null,
    val nodeHost: String? = null,
    val nodePort: Int? = null,
    val realityPublicKey: String? = null,
    val realitySni: String? = null,
    val realityShortId: String? = null,
    // Transport / anti-DPI params (RU 2026); present when protocol == "vless".
    val security: String? = null,
    val network: String? = null,
    val flow: String? = null,
    val xhttpPath: String? = null,
    val xhttpMode: String? = null,
    val xPaddingBytes: String? = null,
    val fingerprint: String? = null,
    val nodes: List<Node>,
)

/** A single resolved tunnel config for the current best node. */
data class VpnConfig(
    val protocol: String, // "wireguard" | "vless"
    val config: String,
)
