package com.tyrax.data.local

import android.content.Context
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.intPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.map
import javax.inject.Inject
import javax.inject.Singleton

private val Context.updateDataStore: DataStore<Preferences>
        by preferencesDataStore(name = "tyrax_update_prefs")

/**
 * Remembers the highest update version_code the user dismissed with "ПОЗЖЕ" so the
 * banner does not nag on every launch. Mandatory updates ignore this value.
 */
@Singleton
class UpdatePrefs @Inject constructor(
    @ApplicationContext private val context: Context,
) {
    companion object {
        private val DISMISSED_VERSION_CODE_KEY = intPreferencesKey("dismissed_version_code")
    }

    val dismissedVersionCode: Flow<Int> = context.updateDataStore.data.map { prefs ->
        prefs[DISMISSED_VERSION_CODE_KEY] ?: 0
    }

    suspend fun setDismissed(versionCode: Int) {
        context.updateDataStore.edit { prefs ->
            prefs[DISMISSED_VERSION_CODE_KEY] = versionCode
        }
    }
}
