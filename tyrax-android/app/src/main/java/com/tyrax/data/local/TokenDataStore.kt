package com.tyrax.data.local

import android.content.Context
import android.provider.Settings
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.map
import javax.inject.Inject
import javax.inject.Singleton

private val Context.tyraxDataStore: DataStore<Preferences>
        by preferencesDataStore(name = "tyrax_prefs")

@Singleton
class TokenDataStore @Inject constructor(
    @ApplicationContext private val context: Context,
) {
    companion object {
        private val JWT_TOKEN_KEY  = stringPreferencesKey("jwt_token")
        private val EMAIL_KEY      = stringPreferencesKey("user_email")
        private val DEVICE_NAME_KEY = stringPreferencesKey("device_name")
    }

    val hasToken: Flow<Boolean> = context.tyraxDataStore.data.map { prefs ->
        prefs[JWT_TOKEN_KEY].orEmpty().isNotBlank()
    }

    val token: Flow<String?> = context.tyraxDataStore.data.map { prefs ->
        prefs[JWT_TOKEN_KEY]
    }

    val email: Flow<String?> = context.tyraxDataStore.data.map { prefs ->
        prefs[EMAIL_KEY]
    }

    val deviceName: Flow<String?> = context.tyraxDataStore.data.map { prefs ->
        prefs[DEVICE_NAME_KEY]
    }

    suspend fun saveToken(token: String) {
        context.tyraxDataStore.edit { prefs ->
            prefs[JWT_TOKEN_KEY] = token
        }
    }

    suspend fun clearToken() {
        context.tyraxDataStore.edit { prefs ->
            prefs.remove(JWT_TOKEN_KEY)
            prefs.remove(EMAIL_KEY)
            // Intentionally keep DEVICE_NAME_KEY — the physical device is the same after logout.
        }
    }

    suspend fun saveEmail(email: String) {
        context.tyraxDataStore.edit { prefs ->
            prefs[EMAIL_KEY] = email
        }
    }

    /**
     * Returns the persisted device name, creating and saving it on first call.
     * Format: "android-<first 8 chars of ANDROID_ID>".
     * Survives logout; only changes if the device is factory-reset (ANDROID_ID changes).
     */
    suspend fun getOrCreateDeviceName(): String {
        val stored = context.tyraxDataStore.data.first()[DEVICE_NAME_KEY]
        if (!stored.isNullOrBlank()) return stored

        val androidId = Settings.Secure.getString(
            context.contentResolver,
            Settings.Secure.ANDROID_ID,
        ).orEmpty().take(8).ifBlank { "unknown" }
        val name = "android-$androidId"

        context.tyraxDataStore.edit { prefs -> prefs[DEVICE_NAME_KEY] = name }
        return name
    }
}
