package com.tyrax.domain.model

/** A newer Android release the client should offer to install. */
data class UpdateInfo(
    val version: String,
    val versionCode: Int,
    val url: String,
    val mandatory: Boolean,
    val notes: String,
)
