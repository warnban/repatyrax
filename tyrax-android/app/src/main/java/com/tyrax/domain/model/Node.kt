package com.tyrax.domain.model

data class Node(
    val id: String,
    val codename: String,
    val country: String,
    val status: NodeStatus,
    val pingMs: Int,
    val realitySni: String? = null,
)

enum class NodeStatus {
    OPEN,
    MONITORED,
    HEAVILY_RESTRICTED,
    UNKNOWN;

    companion object {
        fun from(raw: String): NodeStatus = when (raw.uppercase()) {
            "OPEN" -> OPEN
            "MONITORED" -> MONITORED
            "HEAVILY_RESTRICTED" -> HEAVILY_RESTRICTED
            else -> UNKNOWN
        }
    }
}
