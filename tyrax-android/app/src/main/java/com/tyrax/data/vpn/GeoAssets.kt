package com.tyrax.data.vpn

import android.content.Context
import android.util.Log
import java.io.File

/**
 * Materialises the bundled Xray geo databases (`geoip.dat`, `geosite.dat`) into a
 * stable on-disk directory that Xray-core reads via its asset-location env.
 *
 * Source: `geoip.dat` — v2fly/geoip release (contains `geoip:ru`); `geosite.dat` —
 * Loyalsoldier/v2ray-rules-dat release. Bump [GEO_ASSET_VERSION] whenever the bundled
 * `.dat` files under `assets/` are refreshed so devices re-copy them.
 *
 * With these present, [XrayConfigPatcher] can route `geoip:ru` traffic to `direct`
 * (real RU IP) even when the destination is reached by raw IP with no visible domain.
 */
object GeoAssets {

    private const val TAG = "TYRAX-GeoAssets"
    private const val GEO_ASSET_VERSION = 1
    private const val GEO_DIR = "geo"
    private const val MARKER = ".v"
    private val ASSET_FILES = listOf("geoip.dat", "geosite.dat")

    /**
     * Copies the bundled `.dat` files to `filesDir/geo` if missing or stale and returns
     * that directory's absolute path. On any failure returns [Context.getFilesDir]'s path
     * so the core still starts (routing simply falls back to the domain + per-app layers).
     */
    fun ensure(context: Context): String {
        return runCatching {
            val geoDir = File(context.filesDir, GEO_DIR).apply { mkdirs() }
            val marker = File(geoDir, MARKER)
            val fresh = marker.takeIf { it.exists() }?.readText()?.trim() == GEO_ASSET_VERSION.toString() &&
                ASSET_FILES.all { File(geoDir, it).exists() }

            if (!fresh) {
                ASSET_FILES.forEach { name ->
                    context.assets.open(name).use { input ->
                        File(geoDir, name).outputStream().use { output -> input.copyTo(output) }
                    }
                }
                marker.writeText(GEO_ASSET_VERSION.toString())
                Log.d(TAG, "geo assets materialised to ${geoDir.absolutePath} (v$GEO_ASSET_VERSION)")
            }
            geoDir.absolutePath
        }.getOrElse { e ->
            Log.e(TAG, "geo asset copy failed; falling back to filesDir", e)
            context.filesDir.absolutePath
        }
    }
}
