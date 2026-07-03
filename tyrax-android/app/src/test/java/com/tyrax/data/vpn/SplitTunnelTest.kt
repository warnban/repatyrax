package com.tyrax.data.vpn

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class SplitTunnelTest {

    @Test
    fun `RU_SPLIT_DOMAINS contains key RU services`() {
        val domains = SplitTunnel.RU_SPLIT_DOMAINS
        assertTrue("missing max.ru", domains.contains("max.ru"))
        assertTrue("missing sberbank.com", domains.contains("sberbank.com"))
        assertTrue("missing ozon.ru", domains.contains("ozon.ru"))
        assertTrue("missing wildberries.ru", domains.contains("wildberries.ru"))
        assertTrue("missing vk.com", domains.contains("vk.com"))
    }

    @Test
    fun `RU_SPLIT_DOMAINS has no duplicates`() {
        val domains = SplitTunnel.RU_SPLIT_DOMAINS
        assertEquals(domains.size, domains.distinct().size)
    }

    @Test
    fun `RU_BYPASS_APPS has no duplicates`() {
        val apps = SplitTunnel.RU_BYPASS_APPS
        assertEquals(apps.size, apps.distinct().size)
    }

    @Test
    fun `RU_BYPASS_APPS covers major RU apps`() {
        val apps = SplitTunnel.RU_BYPASS_APPS
        assertTrue("missing VK", apps.contains("com.vkontakte.android"))
        assertTrue("missing Ozon", apps.contains("ru.ozon.app.android"))
        assertTrue("missing a Sberbank package", apps.any { it.contains("sberbankmobile") })
    }
}
