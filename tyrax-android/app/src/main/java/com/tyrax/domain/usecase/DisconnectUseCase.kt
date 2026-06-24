package com.tyrax.domain.usecase

import android.content.Context
import com.tyrax.data.vpn.TyraxVpnManager
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject

class DisconnectUseCase @Inject constructor(
    @ApplicationContext private val context: Context,
) {
    operator fun invoke() {
        TyraxVpnManager.disconnect(context)
    }
}
