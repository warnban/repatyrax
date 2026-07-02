namespace Tyrax.Data;

/// <summary>
/// Built-in RU split-tunnel list used when <c>/vpn/split-domains</c> is empty or
/// unreachable. Matches the backend's default set.
/// </summary>
public static class SplitTunnelDefaults
{
    public static readonly IReadOnlyList<string> RuDomains = new[]
    {
        "yandex.ru", "ya.ru", "vk.com", "vkontakte.ru", "ok.ru", "mail.ru",
        "gosuslugi.ru", "mos.ru", "sberbank.ru", "tinkoff.ru", "vtb.ru", "alfabank.ru", "raiffeisen.ru",
        "ozon.ru", "wildberries.ru", "avito.ru", "hh.ru", "kinopoisk.ru", "ivi.ru", "rutube.ru",
        "2gis.ru", "drom.ru", "auto.ru", "rbc.ru", "kommersant.ru", "ria.ru", "lenta.ru", "meduza.io",
    };
}
