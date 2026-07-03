package com.tyrax.data.local

import android.content.Context
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.booleanPreferencesKey
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringSetPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.map
import javax.inject.Inject
import javax.inject.Singleton

private val Context.splitDataStore: DataStore<Preferences>
        by preferencesDataStore(name = "tyrax_split_prefs")

/**
 * Persists the RU split-tunnel state:
 *  - [enabled]: master toggle (default ON) — RU services bypass the tunnel and exit
 *    over the phone's real network so they see a Russian IP.
 *  - [dynamicBypassDomains]: domains the self-healing [com.tyrax.data.vpn.SplitDiagnostics]
 *    loop found blocked-through-VPN and auto-added to the bypass set.
 */
@Singleton
class SplitTunnelPrefs @Inject constructor(
    @ApplicationContext private val context: Context,
) {
    companion object {
        private val SPLIT_ENABLED_KEY = booleanPreferencesKey("split_enabled")
        private val DYNAMIC_BYPASS_KEY = stringSetPreferencesKey("dynamic_bypass")
    }

    /** Defaults to true when unset — split-tunnel is on by default. */
    val enabled: Flow<Boolean> = context.splitDataStore.data.map { prefs ->
        prefs[SPLIT_ENABLED_KEY] ?: true
    }

    val dynamicBypassDomains: Flow<Set<String>> = context.splitDataStore.data.map { prefs ->
        prefs[DYNAMIC_BYPASS_KEY] ?: emptySet()
    }

    suspend fun setEnabled(value: Boolean) {
        context.splitDataStore.edit { prefs ->
            prefs[SPLIT_ENABLED_KEY] = value
        }
    }

    /** Adds a domain to the self-healing bypass set (idempotent). */
    suspend fun addDynamicBypass(domain: String) {
        context.splitDataStore.edit { prefs ->
            val current = prefs[DYNAMIC_BYPASS_KEY] ?: emptySet()
            prefs[DYNAMIC_BYPASS_KEY] = current + domain
        }
    }
}
