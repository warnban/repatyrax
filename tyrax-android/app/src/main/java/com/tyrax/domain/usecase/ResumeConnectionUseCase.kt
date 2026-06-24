package com.tyrax.domain.usecase

import android.content.Context
import com.tyrax.data.vpn.TyraxVpnManager
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject

/**
 * Resumes a pending tunnel connection after the user grants VPN consent
 * (i.e. after the system dialog raised by [VpnState.NeedsPermission] returns OK).
 */
class ResumeConnectionUseCase @Inject constructor(
    @ApplicationContext private val context: Context,
) {
    operator fun invoke() = TyraxVpnManager.retryAfterPermission(context)
}
