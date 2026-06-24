package com.tyrax.domain.model

data class Device(
    val id: String,
    val name: String,
    val publicKey: String? = null,
)

/** Returned when a device is provisioned: carries the connectable config + node list. */
data class DeviceConfig(
    val deviceId: String,
    val wireGuardConf: String?,
    val vlessConf: String?,
    val nodes: List<Node>,
)

/** A single resolved tunnel config for the current best node. */
data class VpnConfig(
    val protocol: String, // "wireguard" | "vless"
    val config: String,
)
