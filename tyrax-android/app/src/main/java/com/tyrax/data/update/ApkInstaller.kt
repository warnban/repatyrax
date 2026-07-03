package com.tyrax.data.update

import android.content.Context
import android.content.Intent
import android.net.Uri
import android.os.Build
import android.provider.Settings
import android.util.Log
import androidx.core.content.FileProvider
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import okhttp3.OkHttpClient
import okhttp3.Request
import java.io.File
import java.util.concurrent.TimeUnit

/**
 * Downloads a signed TYRAX APK and hands it to the system package installer.
 *
 * Flow: stream the APK to cacheDir → (if needed) route the user to enable
 * "install unknown apps" for TYRAX → fire ACTION_VIEW with a FileProvider content URI.
 * The OS installer then shows the standard update confirmation.
 */
object ApkInstaller {

    private const val TAG = "TYRAX-Installer"
    private const val APK_NAME = "tyrax-update.apk"
    private const val TIMEOUT_SECONDS = 120L

    sealed class Result {
        object Success : Result()
        object NeedsUnknownSourcesPermission : Result()
        data class Error(val message: String) : Result()
    }

    private val client: OkHttpClient by lazy {
        OkHttpClient.Builder()
            .connectTimeout(30, TimeUnit.SECONDS)
            .readTimeout(TIMEOUT_SECONDS, TimeUnit.SECONDS)
            .callTimeout(TIMEOUT_SECONDS, TimeUnit.SECONDS)
            .build()
    }

    /**
     * Downloads [url] and launches the installer. Returns a [Result]; on
     * [Result.NeedsUnknownSourcesPermission] the caller has been sent to settings and
     * should retry after the user grants the permission.
     */
    suspend fun downloadAndInstall(context: Context, url: String): Result = withContext(Dispatchers.IO) {
        // Android O+ gates sideload installs behind a per-app "unknown sources" grant.
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O &&
            !context.packageManager.canRequestPackageInstalls()
        ) {
            runCatching {
                val intent = Intent(Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES)
                    .setData(Uri.parse("package:${context.packageName}"))
                    .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                context.startActivity(intent)
            }
            return@withContext Result.NeedsUnknownSourcesPermission
        }

        val apk = runCatching { download(context, url) }.getOrElse { e ->
            Log.e(TAG, "download failed", e)
            return@withContext Result.Error("ОБНОВЛЕНИЕ НЕ УДАЛОСЬ. ПОВТОРИТЕ.")
        }

        runCatching {
            val uri = FileProvider.getUriForFile(context, "${context.packageName}.fileprovider", apk)
            val intent = Intent(Intent.ACTION_VIEW)
                .setDataAndType(uri, "application/vnd.android.package-archive")
                .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_GRANT_READ_URI_PERMISSION)
            context.startActivity(intent)
            Result.Success
        }.getOrElse { e ->
            Log.e(TAG, "install intent failed", e)
            Result.Error("ОБНОВЛЕНИЕ НЕ УДАЛОСЬ. ПОВТОРИТЕ.")
        }
    }

    private fun download(context: Context, url: String): File {
        val target = File(context.cacheDir, APK_NAME).apply { if (exists()) delete() }
        val req = Request.Builder().url(url).get().build()
        client.newCall(req).execute().use { resp ->
            if (!resp.isSuccessful) error("HTTP ${resp.code}")
            val body = resp.body ?: error("empty body")
            target.outputStream().use { out -> body.byteStream().copyTo(out) }
        }
        return target
    }
}
