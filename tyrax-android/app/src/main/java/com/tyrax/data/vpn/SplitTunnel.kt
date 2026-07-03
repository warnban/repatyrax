package com.tyrax.data.vpn

/**
 * RU split-tunnel data.
 *
 * NOTE ON MECHANISM:
 * Android's [android.net.VpnService.Builder] cannot exclude traffic by *domain* — there is no
 * public route-exclusion API, and domains aren't known at route-install time. The only
 * first-class bypass mechanism is per-application exclusion via `addDisallowedApplication`.
 *
 * So TYRAX bypasses the tunnel for known RU apps (which themselves talk to the RU domains below),
 * and keeps [RU_SPLIT_DOMAINS] for DNS-level handling and parity with the
 * `/api/v1/vpn/split-domains` server list. The server list takes precedence when reachable.
 */
object SplitTunnel {

    /** Mirrors the backend `/vpn/split-domains` seed list (kept in lockstep). */
    val RU_SPLIT_DOMAINS: List<String> = listOf(
        "yandex.ru", "ya.ru", "yandex.net", "dzen.ru", "vk.com", "vkontakte.ru", "ok.ru", "mail.ru",
        "max.ru", "oneme.ru",
        "gosuslugi.ru", "nalog.gov.ru", "gostech.ru", "mos.ru",
        "sberbank.ru", "sberbank.com", "sber.ru", "tinkoff.ru", "tbank.ru", "vtb.ru", "alfabank.ru", "raiffeisen.ru",
        "ozon.ru", "ozon.com", "wildberries.ru", "wildberries.com", "megamarket.ru", "aliexpress.ru",
        "mvideo.ru", "dns-shop.ru", "citilink.ru",
        "avito.ru", "hh.ru", "kinopoisk.ru", "ivi.ru", "rutube.ru",
        "2gis.ru", "gismeteo.ru", "drom.ru", "auto.ru", "rbc.ru", "kommersant.ru", "ria.ru", "lenta.ru", "meduza.io",
    )

    /**
     * RU apps routed directly (outside the tunnel). These cover the messengers / banks / gov /
     * marketplaces whose backends geo-block foreign IPs and must bypass TYRAX. Apps not installed
     * are skipped safely at apply-time, so over-listing candidate package names is harmless.
     */
    val RU_BYPASS_APPS: List<String> = listOf(
        "ru.yandex.searchplugin",        // Yandex
        "ru.yandex.taxi",                // Yandex Go
        "ru.yandex.market",              // Yandex Market
        "com.vkontakte.android",         // VK
        "ru.ok.android",                 // OK
        "ru.mail.mailapp",               // Mail.ru
        "ru.oneme.app",                  // MAX messenger (candidate)
        "ru.vk.max",                     // MAX messenger (alt candidate)
        "ru.rostel",                     // Gosuslugi
        "ru.gosuslugi.structure",        // Gosuslugi (alt)
        "ru.sberbankmobile",             // Sberbank Online
        "com.sberbankmobile",            // Sberbank (alt package)
        "ru.sberbankmobile.push",        // Sberbank push
        "com.idamob.tinkoff.android",    // T-Bank / Tinkoff
        "ru.vtb24.mobilebanking.android",// VTB
        "ru.alfabank.mobile.android",    // Alfa-Bank
        "ru.raiffeisennews",             // Raiffeisen
        "ru.ozon.app.android",           // Ozon
        "com.wildberries.ru",            // Wildberries
        "ru.wildberries.wbservices",     // Wildberries services
        "ru.megamarket.mobile",          // Megamarket
        "com.aliexpress.ru",             // AliExpress RU
        "ru.avito.app",                  // Avito
        "ru.hh.android",                 // hh.ru
        "ru.kinopoisk",                  // Kinopoisk
        "ru.ivi.client",                 // ivi
        "ru.rutube.app",                 // Rutube
        "ru.dublgis.dgismobile",         // 2GIS
        "ru.megafon.mlk",                // MegaFon
        "ru.beeline.services",           // Beeline
        "ru.mts.mtstv",                  // MTS TV
    )
}
