package com.tyrax.presentation.utils

import android.content.Context
import androidx.annotation.StringRes
import com.tyrax.R
import retrofit2.HttpException
import java.io.IOException
import java.net.ConnectException
import java.net.SocketTimeoutException
import java.net.UnknownHostException

/**
 * Single source of truth for translating raw failures into TYRAX-branded copy.
 * Returns string resource IDs so the actual text stays in strings.xml.
 *
 * Cold, direct tone — never apologetic (see brand rules).
 */
object ErrorMapper {

    /** Map an HTTP status code to a branded message resource. */
    @StringRes
    fun resForHttpCode(code: Int): Int = when (code) {
        401 -> R.string.error_access_denied_reenter
        403 -> R.string.error_level_insufficient
        503 -> R.string.error_node_switching
        else -> R.string.error_system
    }

    /** Map any throwable (network or HTTP) to a branded message resource. */
    @StringRes
    fun resForThrowable(throwable: Throwable): Int = when (throwable) {
        is HttpException        -> resForHttpCode(throwable.code())
        is SocketTimeoutException -> R.string.error_signal_lost
        is UnknownHostException,
        is ConnectException     -> R.string.error_connection_lost
        is IOException          -> R.string.error_connection_lost
        else                    -> resForMessage(throwable.message)
    }

    /**
     * Fallback for failures that only carry a message string (the existing
     * repository layer wraps HTTP errors as plain exceptions).
     */
    @StringRes
    private fun resForMessage(message: String?): Int = when {
        message == null                          -> R.string.error_system
        message.contains("401")                  -> R.string.error_access_denied_reenter
        message.contains("403")                  -> R.string.error_level_insufficient
        message.contains("503")                  -> R.string.error_node_switching
        message.contains("timeout", true)        -> R.string.error_signal_lost
        message.contains("Unable to resolve")    -> R.string.error_connection_lost
        message.contains("failed to connect")    -> R.string.error_connection_lost
        else                                     -> R.string.error_system
    }

    /** Resolve a throwable straight to its branded message text. */
    fun message(context: Context, throwable: Throwable): String =
        context.getString(resForThrowable(throwable))

    /** Resolve an HTTP code straight to its branded message text. */
    fun message(context: Context, httpCode: Int): String =
        context.getString(resForHttpCode(httpCode))
}
