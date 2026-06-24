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

    /** Mirrors the backend `/vpn/split-domains` seed list. */
    val RU_SPLIT_DOMAINS: List<String> = listOf(
        "yandex.ru", "ya.ru", "vk.com", "vkontakte.ru", "ok.ru", "mail.ru",
        "gosuslugi.ru", "mos.ru", "sberbank.ru", "tinkoff.ru", "vtb.ru", "alfabank.ru", "raiffeisen.ru",
        "ozon.ru", "wildberries.ru", "avito.ru", "hh.ru", "kinopoisk.ru", "ivi.ru", "rutube.ru",
        "2gis.ru", "drom.ru", "auto.ru", "rbc.ru", "kommersant.ru", "ria.ru", "lenta.ru", "meduza.io",
    )

    /**
     * RU apps routed directly (outside the tunnel). These cover the banks / gov / marketplaces
     * whose domains must bypass TYRAX. Apps not installed are skipped safely at apply-time.
     */
    val RU_BYPASS_APPS: List<String> = listOf(
        "ru.yandex.searchplugin",        // Yandex
        "com.vkontakte.android",         // VK
        "ru.ok.android",                 // OK
        "ru.mail.mailapp",               // Mail.ru
        "ru.rostel",                     // Gosuslugi
        "ru.gosuslugi.structure",        // Gosuslugi (alt)
        "ru.sberbankmobile",             // Sberbank Online
        "com.idamob.tinkoff.android",    // T-Bank / Tinkoff
        "ru.vtb24.mobilebanking.android",// VTB
        "ru.alfabank.mobile.android",    // Alfa-Bank
        "ru.raiffeisennews",             // Raiffeisen
        "ru.ozon.app.android",           // Ozon
        "com.wildberries.ru",            // Wildberries
        "ru.avito.app",                  // Avito
        "ru.hh.android",                 // hh.ru
        "ru.kinopoisk",                  // Kinopoisk
        "ru.ivi.client",                 // ivi
        "ru.rutube.app",                 // Rutube
        "ru.dublgis.dgismobile",         // 2GIS
    )
}
