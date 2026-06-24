package com.tyrax.domain.repository

import com.tyrax.domain.model.DeviceConfig
import com.tyrax.domain.model.InviteRecord
import com.tyrax.domain.model.Node
import com.tyrax.domain.model.Subscription
import com.tyrax.domain.model.UserDevice
import com.tyrax.domain.model.VpnConfig
import com.tyrax.domain.model.VpnState
import kotlinx.coroutines.flow.StateFlow

interface VpnRepository {
    suspend fun getNodes(): Result<List<Node>>
    suspend fun getBestNode(): Result<Node>
    suspend fun getDeviceConfig(devicePublicKey: String): Result<VpnConfig>
    suspend fun addDevice(name: String): Result<DeviceConfig>
    suspend fun getDevices(): Result<List<UserDevice>>
    suspend fun deleteDevice(id: String): Result<Unit>
    suspend fun getSplitDomains(): Result<List<String>>
    suspend fun getSubscription(): Result<Subscription>
    suspend fun getInvites(): Result<List<InviteRecord>>
    suspend fun sendInvite(accountId: String): Result<Unit>
    suspend fun removeInvite(accountId: String): Result<Unit>
    val vpnState: StateFlow<VpnState>
}
