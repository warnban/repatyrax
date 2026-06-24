package com.tyrax.data.remote

data class ApiResponse<T>(
    val status: String,
    val data: T? = null,
    val message: String? = null,
)
