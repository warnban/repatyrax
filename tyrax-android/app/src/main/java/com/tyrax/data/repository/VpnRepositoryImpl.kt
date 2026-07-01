package com.tyrax.data.repository

import com.tyrax.data.remote.AddDeviceRequest
import com.tyrax.data.remote.VpnConnectRequest
import com.tyrax.data.remote.DeviceConfigDto
import com.tyrax.data.remote.InviteDto
import com.tyrax.data.remote.NodeDto
import com.tyrax.data.remote.SendInviteRequest
import com.tyrax.data.remote.SubscriptionDto
import com.tyrax.data.remote.TyraxApiService
import com.tyrax.data.remote.UserDeviceDto
import com.tyrax.data.remote.VpnConfigDto
import com.tyrax.data.vpn.SplitTunnel
import com.tyrax.data.vpn.TyraxVpnManager
import com.tyrax.domain.model.DeviceConfig
import com.tyrax.domain.model.InviteRecord
import com.tyrax.domain.model.Node
import com.tyrax.domain.model.NodeStatus
import com.tyrax.domain.model.Subscription
import com.tyrax.domain.model.UserDevice
import com.tyrax.domain.model.VpnConfig
import com.tyrax.domain.model.VpnState
import android.util.Log
import com.tyrax.domain.repository.VpnRepository
import kotlinx.coroutines.flow.StateFlow
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class VpnRepositoryImpl @Inject constructor(
    private val api: TyraxApiService,
) : VpnRepository {

    override suspend fun getNodes(): Result<List<Node>> = runCatching {
        val resp = api.getNodes()
        resp.data?.map { it.toDomain() } ?: error("NODE UNAVAILABLE")
    }.mapApiError()

    override suspend fun getBestNode(): Result<Node> = runCatching {
        val resp = api.getNodes()
        val nodes = resp.data?.map { it.toDomain() } ?: error("NODE UNAVAILABLE")
        nodes.filter { it.status == NodeStatus.OPEN }
            .minByOrNull { it.pingMs }
            ?: error("NODE UNAVAILABLE")
    }.mapApiError()

    override suspend fun getDeviceConfig(devicePublicKey: String): Result<VpnConfig> = runCatching {
        val resp = api.getVpnConfig(devicePublicKey)
        resp.data?.toDomain() ?: error("NODE UNAVAILABLE")
    }.mapApiError()

    override suspend fun connect(name: String, codename: String): Result<VpnConfig> = runCatching {
        val resp = api.connectVpn(VpnConnectRequest(name = name, codename = codename))
        Log.d(
            "TYRAX-Repo",
            "connectVpn resp: status=${resp.status} protocol=${resp.data?.protocol} " +
                "configLen=${resp.data?.config?.length ?: 0}",
        )
        resp.data?.toDomain() ?: error(resp.message ?: "NODE UNAVAILABLE")
    }.mapApiError()

    override suspend fun addDevice(name: String): Result<DeviceConfig> = runCatching {
        val resp = api.addDevice(AddDeviceRequest(name))
        Log.d("TYRAX-Repo", "addDevice resp: status=${resp.status} message=${resp.message} " +
            "data=${resp.data != null} protocol=${resp.data?.protocol} " +
            "uuid=${resp.data?.uuid} host=${resp.data?.nodeHost} port=${resp.data?.nodePort} " +
            "pubKey=${resp.data?.realityPublicKey?.take(16)} sni=${resp.data?.realitySni}")
        resp.data?.toDomain() ?: error(resp.message ?: "DEVICE LIMIT REACHED")
    }.mapApiError()

    override suspend fun getDevices(): Result<List<UserDevice>> = runCatching {
        api.getDevices().data?.map { it.toDomain() } ?: error("DEVICE LIST UNAVAILABLE")
    }.mapApiError()

    override suspend fun deleteDevice(id: String): Result<Unit> = runCatching {
        api.deleteDevice(id)
        Unit
    }.mapApiError()

    override suspend fun getSplitDomains(): Result<List<String>> = runCatching {
        val resp = api.getSplitDomains()
        resp.data?.domains?.takeIf { it.isNotEmpty() } ?: SplitTunnel.RU_SPLIT_DOMAINS
    }.recover { SplitTunnel.RU_SPLIT_DOMAINS }

    override suspend fun getSubscription(): Result<Subscription> = runCatching {
        api.getSubscription().data?.toDomain() ?: error("SUBSCRIPTION UNAVAILABLE")
    }.mapApiError()

    override suspend fun getInvites(): Result<List<InviteRecord>> = runCatching {
        api.getInvites().data?.map { it.toDomain() } ?: emptyList()
    }.mapApiError()

    override suspend fun sendInvite(accountId: String): Result<Unit> = runCatching {
        api.sendInvite(SendInviteRequest(accountId))
        Unit
    }.mapApiError()

    override suspend fun removeInvite(accountId: String): Result<Unit> = runCatching {
        api.removeInvite(accountId)
        Unit
    }.mapApiError()

    override val vpnState: StateFlow<VpnState> = TyraxVpnManager.state
}

private fun NodeDto.toDomain() = Node(
    id = id,
    codename = codename,
    country = country,
    status = NodeStatus.from(status),
    pingMs = pingMs,
    realitySni = realitySni,
)

private fun VpnConfigDto.toDomain() = VpnConfig(
    protocol = protocol,
    config = config,
)

private fun DeviceConfigDto.toDomain() = DeviceConfig(
    deviceId = deviceId,
    protocol = protocol,
    wireGuardConf = wireguardConf,
    vlessConf = vlessConf,
    uuid = uuid,
    nodeHost = nodeHost,
    nodePort = nodePort,
    realityPublicKey = realityPublicKey,
    realitySni = realitySni,
    realityShortId = realityShortId,
    security = security,
    network = network,
    flow = flow,
    xhttpPath = xhttpPath,
    xhttpMode = xhttpMode,
    xPaddingBytes = xPaddingBytes,
    fingerprint = fingerprint,
    nodes = nodes.map { it.toDomain() },
)

private fun UserDeviceDto.toDomain() = UserDevice(id = id, name = name, createdAt = createdAt)

private fun SubscriptionDto.toDomain() = Subscription(
    tier = tier,
    endsAt = endsAt,
    devicesCount = devicesCount,
    devicesLimit = devicesLimit,
    usedBytes = trafficUsedBytes,
    limitBytes = trafficLimitBytes,
    unlimited = unlimited,
    blockedUntil = blockedUntil,
)

private fun InviteDto.toDomain() = InviteRecord(id = id, inviteeId = inviteeId, status = status)

// Map network / HTTP exceptions to TYRAX-branded messages.
private fun <T> Result<T>.mapApiError(): Result<T> = onFailure { e ->
    val message = when {
        e.message?.contains("403") == true -> "DEVICE LIMIT REACHED"
        e.message?.contains("503") == true -> "NODE UNAVAILABLE"
        e.message?.contains("Unable to resolve") == true ||
        e.message?.contains("failed to connect") == true -> "CONNECTION FAILED. RETRY."
        else -> e.message ?: "SYSTEM ERROR. NODE OFFLINE."
    }
    return Result.failure(Exception(message))
}
