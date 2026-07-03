package com.tyrax.domain.usecase

import com.tyrax.BuildConfig
import com.tyrax.data.local.UpdatePrefs
import com.tyrax.data.remote.AndroidUpdateDto
import com.tyrax.data.remote.TyraxApiService
import kotlinx.coroutines.flow.first
import javax.inject.Inject

/**
 * Checks the backend Android manifest and decides whether to surface the update banner.
 * Returns null when up-to-date, unreachable, or a non-mandatory update the user dismissed.
 */
class CheckUpdateUseCase @Inject constructor(
    private val api: TyraxApiService,
    private val updatePrefs: UpdatePrefs,
) {
    suspend operator fun invoke(): com.tyrax.domain.model.UpdateInfo? {
        val dto = runCatching { api.getAndroidLatest(MANIFEST_URL) }.getOrNull() ?: return null
        val dismissed = updatePrefs.dismissedVersionCode.first()
        return resolveUpdate(dto, BuildConfig.VERSION_CODE, dismissed)
    }

    companion object {
        const val MANIFEST_URL = "https://api.tyrax.tech/download/android/latest.json"

        /**
         * Pure decision: offer the update iff its code is newer than [currentCode] and either
         * it is [AndroidUpdateDto.mandatory] or it has not been dismissed at/above its code.
         */
        fun resolveUpdate(
            dto: AndroidUpdateDto,
            currentCode: Int,
            dismissedCode: Int,
        ): com.tyrax.domain.model.UpdateInfo? {
            if (dto.versionCode <= currentCode) return null
            if (!dto.mandatory && dto.versionCode <= dismissedCode) return null
            return com.tyrax.domain.model.UpdateInfo(
                version = dto.version,
                versionCode = dto.versionCode,
                url = dto.url,
                mandatory = dto.mandatory,
                notes = dto.notes,
            )
        }
    }
}
