package com.tyrax.data.vpn

import org.json.JSONArray
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class XrayConfigPatcherTest {

    private val rawConfig = """
        {
          "outbounds": [
            {"tag":"proxy","protocol":"vless","settings":{"vnext":[{"users":[{"id":"x"}]}]},"streamSettings":{"network":"tcp"}},
            {"tag":"direct","protocol":"freedom","settings":{}}
          ]
        }
    """.trimIndent()

    private fun rules(json: String): JSONArray =
        JSONObject(json).getJSONObject("routing").getJSONArray("rules")

    private fun indexOfDomainBypass(rules: JSONArray, domain: String): Int {
        for (i in 0 until rules.length()) {
            val r = rules.getJSONObject(i)
            if (r.optString("outboundTag") == "direct") {
                val d = r.optJSONArray("domain") ?: continue
                for (j in 0 until d.length()) {
                    if (d.getString(j) == "domain:$domain") return i
                }
            }
        }
        return -1
    }

    private fun indexOfGeoipRu(rules: JSONArray): Int {
        for (i in 0 until rules.length()) {
            val r = rules.getJSONObject(i)
            if (r.optString("outboundTag") == "direct") {
                val ip = r.optJSONArray("ip") ?: continue
                for (j in 0 until ip.length()) {
                    if (ip.getString(j) == "geoip:ru") return i
                }
            }
        }
        return -1
    }

    private fun indexOfProxy(rules: JSONArray): Int {
        for (i in 0 until rules.length()) {
            if (rules.getJSONObject(i).optString("outboundTag") == "proxy") return i
        }
        return -1
    }

    @Test
    fun `enabled emits domain and geoip bypass rules before the proxy catch-all`() {
        val out = XrayConfigPatcher.enhance(
            rawConfig,
            logDir = null,
            split = XrayConfigPatcher.SplitConfig(enabled = true, bypassDomains = listOf("ozon.ru", "vk.com")),
        )
        val rules = rules(out)

        val domainIdx = indexOfDomainBypass(rules, "ozon.ru")
        val geoipIdx = indexOfGeoipRu(rules)
        val proxyIdx = indexOfProxy(rules)

        assertTrue("domain bypass rule missing", domainIdx >= 0)
        assertTrue("geoip:ru rule missing", geoipIdx >= 0)
        assertTrue("proxy catch-all missing", proxyIdx >= 0)
        assertTrue("domain bypass must precede proxy", domainIdx < proxyIdx)
        assertTrue("geoip:ru must precede proxy", geoipIdx < proxyIdx)
        assertEquals("IPIfNonMatch", JSONObject(out).getJSONObject("routing").getString("domainStrategy"))
    }

    @Test
    fun `disabled emits no bypass rules and keeps AsIs strategy`() {
        val out = XrayConfigPatcher.enhance(
            rawConfig,
            logDir = null,
            split = XrayConfigPatcher.SplitConfig(enabled = false, bypassDomains = listOf("ozon.ru")),
        )
        val rules = rules(out)

        assertEquals(-1, indexOfDomainBypass(rules, "ozon.ru"))
        assertEquals(-1, indexOfGeoipRu(rules))
        assertTrue("proxy catch-all missing", indexOfProxy(rules) >= 0)
        assertFalse(
            "domainStrategy should not be IPIfNonMatch when disabled",
            JSONObject(out).getJSONObject("routing").getString("domainStrategy") == "IPIfNonMatch",
        )
    }
}
