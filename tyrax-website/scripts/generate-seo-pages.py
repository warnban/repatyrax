#!/usr/bin/env python3
"""Generate SEO landing pages for tyrax.tech from a single template."""

from __future__ import annotations

import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent

FOOTER = (ROOT / "partials" / "seo-footer.html").read_text(encoding="utf-8")

NAV = """        <nav class="nav-desktop" aria-label="Основная навигация">
          <a href="/">Главная</a>
          <a href="/free.html">FREE</a>
          <a href="/vpn-windows.html">Windows</a>
          <a href="/vpn-android.html">Android</a>
          <a href="/guides.html">Инструкция</a>
        </nav>
        <div class="header-actions">
          <div class="nav-cta">
            <a href="/download/tyrax.apk" class="btn btn-primary btn-nav" data-dl="android" download>ANDROID</a>
            <a href="/download/windows/TYRAX-Setup.exe" class="btn btn-outline btn-nav" data-dl="windows" download>WINDOWS</a>
          </div>
          <button type="button" class="menu-toggle" id="menu-toggle" aria-expanded="false" aria-controls="mobile-nav" aria-label="Открыть меню">MENU</button>
        </div>"""

MOBILE_NAV = """    <nav class="mobile-nav" id="mobile-nav" aria-label="Мобильная навигация">
      <a href="/">Главная</a>
      <a href="/free.html">Бесплатный VPN</a>
      <a href="/vpn-windows.html">VPN Windows</a>
      <a href="/vpn-android.html">VPN Android</a>
      <a href="/obhod-blokirovok.html">Обход блокировок</a>
      <a href="/guides.html">Инструкция</a>
      <a href="/about.html">О сервисе</a>
      <a href="/contacts.html">Контакты</a>
      <div class="mobile-nav-cta">
        <a href="/download/tyrax.apk" class="btn btn-primary btn-block" data-dl="android" download>СКАЧАТЬ ANDROID</a>
        <a href="/download/windows/TYRAX-Setup.exe" class="btn btn-outline btn-block" data-dl="windows" download>СКАЧАТЬ WINDOWS</a>
      </div>
    </nav>"""

HEAD_BOOST = """  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link rel="preload" href="/css/main.css" as="style">
  <meta property="og:image" content="https://tyrax.tech/assets/og-image.png">
  <meta property="og:image:width" content="1200">
  <meta property="og:image:height" content="630">
  <meta property="og:locale" content="ru_RU">
  <meta property="og:site_name" content="TYRAX">
  <meta name="twitter:card" content="summary_large_image">
  <meta name="twitter:image" content="https://tyrax.tech/assets/og-image.png">"""


def faq_schema(items: list[dict]) -> str:
    entities = []
    for item in items:
        entities.append(
            {
                "@type": "Question",
                "name": item["q"],
                "acceptedAnswer": {"@type": "Answer", "text": item["a"]},
            }
        )
    return json.dumps(
        {"@context": "https://schema.org", "@type": "FAQPage", "mainEntity": entities},
        ensure_ascii=False,
        indent=2,
    )


def breadcrumb_schema(name: str, url: str) -> str:
    data = {
        "@context": "https://schema.org",
        "@type": "BreadcrumbList",
        "itemListElement": [
            {"@type": "ListItem", "position": 1, "name": "Главная", "item": "https://tyrax.tech/"},
            {"@type": "ListItem", "position": 2, "name": name, "item": url},
        ],
    }
    return json.dumps(data, ensure_ascii=False, indent=2)


def article_schema(title: str, url: str, description: str) -> str:
    data = {
        "@context": "https://schema.org",
        "@type": "Article",
        "headline": title,
        "description": description,
        "url": url,
        "author": {"@type": "Organization", "name": "TYRAX"},
        "publisher": {
            "@type": "Organization",
            "name": "TYRAX",
            "logo": {"@type": "ImageObject", "url": "https://tyrax.tech/assets/tyrax-icon.png"},
        },
        "datePublished": "2026-07-02",
        "dateModified": "2026-07-03",
        "inLanguage": "ru-RU",
    }
    return json.dumps(data, ensure_ascii=False, indent=2)


def feature_cards(features: list[dict]) -> str:
    blocks = []
    for f in features:
        blocks.append(
            f"""            <article class="feature-card">
              <h3>{f['title']}</h3>
              <p>{f['text']}</p>
            </article>"""
        )
    return "\n".join(blocks)


def faq_html(items: list[dict]) -> str:
    blocks = []
    for item in items:
        blocks.append(
            f"""            <div class="faq-item">
              <button class="faq-question" aria-expanded="false">{item['q']}</button>
              <div class="faq-answer"><div class="faq-answer-inner">{item['a']}</div></div>
            </div>"""
        )
    return "\n".join(blocks)


def seo_paragraphs(paragraphs: list[str]) -> str:
    return "\n".join(f"          <p>{p}</p>" for p in paragraphs)


def render(page: dict) -> str:
    canonical = f"https://tyrax.tech/{page['file']}"
    schema_blocks = [breadcrumb_schema(page["breadcrumb"], canonical)]
    if page.get("faq"):
        schema_blocks.append(faq_schema(page["faq"]))
    if page.get("article"):
        schema_blocks.append(article_schema(page["title"], canonical, page["description"]))

    schema_html = "\n".join(
        f'  <script type="application/ld+json">\n{block}\n  </script>' for block in schema_blocks
    )

    cta_primary = page.get("cta_primary", "")
    cta_secondary = page.get("cta_secondary", "")

    cta_html = ""
    if cta_primary or cta_secondary:
        parts = ['          <div class="hero-cta">']
        if cta_primary:
            parts.append(f'            <a href="{cta_primary["href"]}" class="btn btn-primary btn-lg" {cta_primary.get("attrs", "")}>{cta_primary["label"]}</a>')
        if cta_secondary:
            parts.append(f'            <a href="{cta_secondary["href"]}" class="btn btn-outline btn-lg" {cta_secondary.get("attrs", "")}>{cta_secondary["label"]}</a>')
        parts.append("          </div>")
        cta_html = "\n".join(parts)

    features_html = ""
    if page.get("features"):
        features_html = f"""
      <section>
        <div class="container">
          <p class="section-label">// {page.get('features_label', 'ВОЗМОЖНОСТИ')}</p>
          <h2 class="section-title">{page.get('features_title', 'ПОЧЕМУ TYRAX')}</h2>
          <div class="features-grid">
{feature_cards(page['features'])}
          </div>
        </div>
      </section>"""

    faq_section = ""
    if page.get("faq"):
        faq_section = f"""
      <section id="faq">
        <div class="container">
          <p class="section-label">// FAQ</p>
          <h2 class="section-title">{page.get('faq_title', 'ЧАСТЫЕ ВОПРОСЫ')}</h2>
          <div class="faq-list">
{faq_html(page['faq'])}
          </div>
        </div>
      </section>"""

    seo_section = ""
    if page.get("seo_paragraphs"):
        seo_section = f"""
      <section class="seo-content">
        <div class="container">
{seo_paragraphs(page['seo_paragraphs'])}
        </div>
      </section>"""

    return f"""<!DOCTYPE html>
<html lang="ru">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{page['title']}</title>
  <meta name="description" content="{page['description']}">
  <meta name="robots" content="index, follow, max-snippet:-1, max-image-preview:large">
  <link rel="canonical" href="{canonical}">
  <link rel="icon" href="/assets/favicon.ico" type="image/x-icon">
  <link rel="apple-touch-icon" href="/assets/tyrax-icon.png">
  <meta property="og:type" content="website">
  <meta property="og:url" content="{canonical}">
  <meta property="og:title" content="{page['og_title']}">
  <meta property="og:description" content="{page['og_description']}">
{HEAD_BOOST}
  <link rel="stylesheet" href="/css/main.css">
{schema_html}
</head>
<body class="scanlines">
  <a class="skip-link" href="#main-content">К содержимому</a>
  <canvas id="matrix-rain" aria-hidden="true"></canvas>
  <div class="page-wrap">
    <header class="site-header">
      <div class="container header-inner">
        <a href="/" class="logo">
          <img src="/assets/tyrax-icon.png" alt="TYRAX — VPN для Windows и Android" width="32" height="32">
          TYRAX
        </a>
{NAV}
      </div>
    </header>
{MOBILE_NAV}
    <main id="main-content">
      <nav class="breadcrumbs container" aria-label="Хлебные крошки">
        <a href="/">Главная</a><span aria-hidden="true"> / </span><span>{page['breadcrumb']}</span>
      </nav>
      <section class="hero">
        <div class="container">
          <span class="hero-badge">{page.get('badge', 'БЕЗ РАЗРЕШЕНИЯ')}</span>
          <h1>{page['h1']}</h1>
          <p class="hero-sub">{page['hero_sub']}</p>
{cta_html}
        </div>
      </section>
{features_html}
{faq_section}
{seo_section}
    </main>
{FOOTER}
  </div>
  <script src="/js/downloads.js" defer></script>
  <script src="/js/matrix-rain.js" defer></script>
  <script src="/js/main.js" defer></script>
</body>
</html>
"""


PAGES = [
    {
        "file": "vpn-windows.html",
        "breadcrumb": "VPN для Windows",
        "title": "VPN для Windows — скачать TYRAX | Бесплатно 1 ГБ, без рекламы",
        "description": "Скачать VPN для Windows 10/11. TYRAX — нативный клиент с автовыбором сервера, WireGuard/VLESS, 1 ГБ бесплатно без рекламы. VPN для компьютера и ноутбука.",
        "og_title": "VPN для Windows — TYRAX",
        "og_description": "Нативный VPN-клиент для Windows. Автодиагностика, фоновый туннель, 1 ГБ FREE.",
        "h1": "VPN ДЛЯ WINDOWS",
        "hero_sub": "Скачать VPN для компьютера за 2 минуты. Нативный клиент TYRAX работает в фоне: автозапуск, системный трей, автовыбор сервера и протокола при блокировках.",
        "cta_primary": {"href": "/download/windows/TYRAX-Setup.exe", "label": "СКАЧАТЬ ДЛЯ WINDOWS", "attrs": 'data-dl="windows" download rel="noopener"'},
        "cta_secondary": {"href": "/free.html", "label": "БЕСПЛАТНЫЙ VPN 1 ГБ"},
        "features_title": "VPN ДЛЯ КОМПЬЮТЕРА — ВОЗМОЖНОСТИ",
        "features": [
            {"title": "Фоновый сервис", "text": "Туннель работает без открытого окна. Автозапуск с Windows — ENTER один раз, протокол всегда активен."},
            {"title": "Автовыбор сервера", "text": "Пинг всех нод при старте. Лучший сервер по задержке и потерям. Смена без вашего участия."},
            {"title": "Fallback протоколов", "text": "WireGuard → VLESS/Reality → Shadowsocks → OpenVPN/TCP. Автопереключение при DPI-блокировках."},
            {"title": "Split Tunnel RU", "text": "Яндекс, VK, банки, Госуслуги — напрямую. Остальной трафик через TYRAX."},
        ],
        "faq": [
            {"q": "Как скачать VPN для Windows бесплатно?", "a": "Скачайте TYRAX-Setup.exe с tyrax.tech, установите клиент, зарегистрируйтесь — тариф FREE даёт 1 ГБ в месяц без рекламы."},
            {"q": "Работает ли VPN на Windows 10 и 11?", "a": "Да. TYRAX поддерживает Windows 10 и 11 x64. Клиент использует системный VPN-интерфейс и фоновый сервис."},
            {"q": "Какой VPN лучше для компьютера?", "a": "TYRAX выделяется автодиагностикой и автопротоколом — не нужно вручную выбирать сервер или протокол при блокировках."},
        ],
        "seo_paragraphs": [
            "Ищете надёжный <strong>VPN для Windows</strong>? TYRAX — нативный клиент для компьютера и ноутбука с полной автоматизацией. Это не расширение браузера, а системный туннель.",
            "Скачать <strong>VPN для компа</strong> можно напрямую с tyrax.tech — без Microsoft Store. <a href=\"/free.html\">Бесплатный VPN</a> — 1 ГБ/мес. Для iPhone смотрите <a href=\"/vpn-iphone.html\">VPN для iPhone</a>.",
        ],
    },
    {
        "file": "vpn-android.html",
        "breadcrumb": "VPN для Android",
        "title": "VPN для Android — скачать APK TYRAX | Бесплатно 1 ГБ",
        "description": "Скачать VPN для Android (APK). TYRAX — автодиагностика, Wi-Fi/LTE handoff, split tunnel RU, 1 ГБ бесплатно без рекламы.",
        "og_title": "VPN для Android — TYRAX APK",
        "og_description": "VPN для телефона с автовыбором сервера. Прямая загрузка APK.",
        "h1": "VPN ДЛЯ ANDROID",
        "hero_sub": "VPN для телефона с терминальным интерфейсом. Один экран — один статус. Автопереключение Wi-Fi ↔ LTE, split tunnel для RU-сервисов.",
        "cta_primary": {"href": "/download/tyrax.apk", "label": "СКАЧАТЬ APK", "attrs": 'data-dl="android" download rel="noopener"'},
        "cta_secondary": {"href": "/guides.html#android", "label": "ИНСТРУКЦИЯ"},
        "features_title": "VPN ДЛЯ ТЕЛЕФОНА",
        "features": [
            {"title": "VpnService + Xray", "text": "Полноценный системный VPN, не прокси в браузере. Весь трафик приложений через туннель."},
            {"title": "Network Handoff", "text": "Бесшовный переход Wi-Fi ↔ LTE без разрыва. Мониторинг каждые 30 секунд."},
            {"title": "APK с сайта", "text": "Прямая загрузка с tyrax.tech — без Google Play. Android 8.0+, ARM64/ARM32."},
            {"title": "FREE 1 ГБ", "text": "Бесплатный тариф с полной автоматизацией и всеми серверами. Без рекламы."},
        ],
        "faq": [
            {"q": "Как скачать VPN на Android?", "a": "Скачайте tyrax.apk с tyrax.tech, разрешите установку из неизвестных источников, войдите через Telegram или email и нажмите ENTER."},
            {"q": "Есть ли бесплатный VPN для телефона?", "a": "Да. TYRAX FREE — 1 ГБ/мес, все серверы, без рекламы. Постоянный тариф, не trial."},
            {"q": "Работает ли VPN на мобильном интернете?", "a": "Да. TYRAX автоматически переключается между Wi-Fi и LTE без разрыва соединения."},
        ],
        "seo_paragraphs": [
            "<strong>VPN для Android</strong> от TYRAX — это VPN для телефона с полной автоматизацией. Скачать APK можно с официального сайта.",
            "Нужен <strong>VPN для телефона бесплатно</strong>? Тариф FREE — <a href=\"/free.html\">1 ГБ без рекламы</a>. Для ПК — <a href=\"/vpn-windows.html\">VPN для Windows</a>.",
        ],
    },
    {
        "file": "vpn-iphone.html",
        "breadcrumb": "VPN для iPhone",
        "title": "VPN для iPhone и iPad — TYRAX + Happ | VLESS Reality",
        "description": "VPN для iPhone через Happ и подписку TYRAX. VLESS Reality XHTTP, инструкция подключения iOS 15+. Обход блокировок на iPhone.",
        "og_title": "VPN для iPhone — TYRAX",
        "og_description": "Подключение через Happ + подписка TYRAX. VLESS Reality для iOS.",
        "h1": "VPN ДЛЯ IPHONE И IPAD",
        "hero_sub": "Нативного клиента TYRAX для iOS пока нет — используйте Happ с подпиской TYRAX. VLESS Reality XHTTP, импорт через Telegram-бот.",
        "cta_primary": {"href": "/guides.html#ios", "label": "ИНСТРУКЦИЯ + HAPP"},
        "cta_secondary": {"href": "/obhod-blokirovok.html", "label": "ОБХОД БЛОКИРОВОК"},
        "features": [
            {"title": "Happ + подписка", "text": "Установите Happ из App Store, получите URL подписки TYRAX в Telegram-боте, импортируйте одним тапом."},
            {"title": "VLESS Reality", "text": "Современный протокол, устойчивый к DPI. XHTTP transport для стабильности на iOS."},
            {"title": "Один аккаунт", "text": "Тот же аккаунт TYRAX, что на Windows и Android. Единая подписка на все устройства по тарифу."},
        ],
        "faq": [
            {"q": "Как установить VPN на iPhone?", "a": "Установите Happ, получите подписку TYRAX в Telegram-боте, импортируйте URL в Happ и подключитесь. Подробно — на странице инструкции."},
            {"q": "Есть ли TYRAX в App Store?", "a": "Нативное приложение TYRAX для iOS в разработке. Сейчас используйте Happ с подпиской TYRAX — полный функционал протокола."},
        ],
        "seo_paragraphs": [
            "<strong>VPN для iPhone</strong> через TYRAX: Happ + VLESS Reality. <a href=\"/guides.html#ios\">Пошаговая инструкция</a>.",
            "Для Mac — <a href=\"/vpn-mac.html\">VPN для Mac</a>. Для Android — <a href=\"/vpn-android.html\">скачать APK</a>.",
        ],
    },
    {
        "file": "vpn-mac.html",
        "breadcrumb": "VPN для Mac",
        "title": "VPN для Mac — TYRAX + Happ | macOS 12+",
        "description": "VPN для Mac через Happ и подписку TYRAX. DMG или App Store, VLESS Reality. Инструкция подключения macOS.",
        "og_title": "VPN для Mac — TYRAX",
        "og_description": "Подключение macOS через Happ + подписка TYRAX.",
        "h1": "VPN ДЛЯ MAC",
        "hero_sub": "VPN для MacBook и iMac через Happ. Та же подписка TYRAX, что на iPhone. VLESS Reality XHTTP.",
        "cta_primary": {"href": "/guides.html#mac", "label": "ИНСТРУКЦИЯ + HAPP"},
        "cta_secondary": {"href": "/vpn-iphone.html", "label": "VPN ДЛЯ IPHONE"},
        "features": [
            {"title": "Happ для macOS", "text": "DMG или App Store. Импорт подписки TYRAX из Telegram-бота."},
            {"title": "Единая подписка", "text": "Один аккаунт TYRAX на iPhone, Mac, Windows, Android (по лимиту тарифа)."},
        ],
        "faq": [
            {"q": "Как подключить VPN на Mac?", "a": "Установите Happ, получите подписку в Telegram-боте TYRAX, импортируйте и подключитесь. Инструкция на guides.html#mac."},
        ],
        "seo_paragraphs": [
            "<strong>VPN для Mac</strong> — через Happ + TYRAX. <a href=\"/guides.html#mac\">Инструкция</a>.",
        ],
    },
    {
        "file": "obhod-blokirovok.html",
        "breadcrumb": "Обход блокировок",
        "title": "VPN для обхода блокировок — TYRAX | Telegram, YouTube",
        "description": "VPN для обхода блокировок сайтов и мессенджеров. TYRAX автоматически переключает протокол при DPI. Telegram, YouTube, Discord.",
        "og_title": "Обход блокировок — TYRAX VPN",
        "og_description": "Автопротокол при блокировках. WireGuard → VLESS/Reality.",
        "h1": "VPN ДЛЯ ОБХОДА БЛОКИРОВОК",
        "hero_sub": "TYRAX обнаруживает блокировки и throttling, автоматически переключает протокол и сервер. Telegram, YouTube, зарубежные сервисы — без ручных настроек.",
        "cta_primary": {"href": "/download/tyrax.apk", "label": "СКАЧАТЬ ANDROID", "attrs": 'data-dl="android" download'},
        "cta_secondary": {"href": "/download/windows/TYRAX-Setup.exe", "label": "СКАЧАТЬ WINDOWS", "attrs": 'data-dl="windows" download'},
        "features_title": "КАК TYRAX ОБХОДИТ БЛОКИРОВКИ",
        "features": [
            {"title": "Автодиагностика DPI", "text": "Обнаружение блокировок и деградации канала. Переключение протокола без участия пользователя."},
            {"title": "VLESS/Reality", "text": "Протоколы, устойчивые к активному DPI в РФ. Fallback-цепочка из 4 протоколов."},
            {"title": "Split Tunnel RU", "text": "Российские сервисы работают напрямую — меньше нагрузка, стабильнее банки и Госуслуги."},
        ],
        "faq": [
            {"q": "Какой VPN лучше для обхода блокировок?", "a": "TYRAX автоматически переключает протокол при блокировках — не нужно вручную менять настройки. Поддерживает VLESS/Reality, WireGuard, Shadowsocks."},
            {"q": "Работает ли VPN для Telegram?", "a": "Да. TYRAX обеспечивает доступ к Telegram и другим заблокированным сервисам через туннель с автопротоколом."},
        ],
        "seo_paragraphs": [
            "<strong>VPN для обхода блокировок</strong> в 2026 — TYRAX с автодиагностикой и fallback протоколов.",
            "Скачать: <a href=\"/vpn-windows.html\">Windows</a>, <a href=\"/vpn-android.html\">Android</a>. <a href=\"/free.html\">1 ГБ бесплатно</a>.",
        ],
    },
    {
        "file": "no-logs.html",
        "breadcrumb": "No Logs",
        "title": "VPN без логов — политика No Logs TYRAX",
        "description": "TYRAX — VPN без логов активности. Минимальный сбор данных, no-logs политика. Что мы не храним и что нужно для работы сервиса.",
        "og_title": "No Logs — TYRAX",
        "og_description": "VPN без логов трафика. Минимальный сбор данных.",
        "h1": "VPN БЕЗ ЛОГОВ",
        "hero_sub": "TYRAX не ведёт логи посещённых сайтов, содержимого трафика и DNS-запросов. Храним только минимум для аккаунта и биллинга.",
        "cta_primary": {"href": "/privacy.html", "label": "ПОЛИТИКА КОНФИДЕНЦИАЛЬНОСТИ"},
        "features": [
            {"title": "Не логируем трафик", "text": "Содержимое соединений, посещённые URL и DNS-запросы не записываются и не анализируются."},
            {"title": "Минимум данных", "text": "Email/Telegram ID, тариф, объём трафика для лимита FREE — только для работы сервиса."},
            {"title": "No ads tracking", "text": "Рекламы нет — нет трекинга для рекламодателей на всех тарифах."},
        ],
        "faq": [
            {"q": "Ведёт ли TYRAX логи?", "a": "Нет логов активности и содержимого трафика. Подробности — в политике конфиденциальности."},
        ],
        "seo_paragraphs": [
            "<strong>VPN без логов</strong> — принцип TYRAX. Полная политика: <a href=\"/privacy.html\">privacy.html</a>.",
        ],
    },
    {
        "file": "vless-vpn.html",
        "breadcrumb": "VLESS VPN",
        "title": "VLESS VPN — TYRAX | Reality, XHTTP, обход DPI",
        "description": "VLESS VPN в TYRAX: Reality, XHTTP transport, автопереключение при блокировках. Часть fallback-цепочки WireGuard → VLESS → Shadowsocks.",
        "og_title": "VLESS VPN — TYRAX",
        "og_description": "VLESS/Reality с автодиагностикой. Обход DPI.",
        "h1": "VLESS VPN",
        "hero_sub": "TYRAX использует VLESS с Reality и XHTTP для устойчивости к DPI. Протокол включается автоматически при блокировке WireGuard.",
        "cta_primary": {"href": "/download/tyrax.apk", "label": "СКАЧАТЬ TYRAX", "attrs": 'data-dl="android" download'},
        "features": [
            {"title": "Reality", "text": "Маскировка под легитимный TLS-трафик. Устойчивость к активному DPI."},
            {"title": "XHTTP", "text": "Transport для iOS через Happ и стабильной работы на мобильных сетях."},
            {"title": "Auto fallback", "text": "Клиент сам переключается на VLESS, если WireGuard заблокирован."},
        ],
        "seo_paragraphs": [
            "<strong>VLESS VPN</strong> в TYRAX — автоматически, без ручной настройки. Также: <a href=\"/wireguard-vpn.html\">WireGuard</a>, <a href=\"/vless-proxy.html\">VLESS прокси</a>.",
        ],
    },
    {
        "file": "wireguard-vpn.html",
        "breadcrumb": "WireGuard VPN",
        "title": "WireGuard VPN — TYRAX | Быстрый протокол по умолчанию",
        "description": "WireGuard VPN в TYRAX — протокол по умолчанию. Быстрое соединение, автопереключение на VLESS при блокировках.",
        "og_title": "WireGuard VPN — TYRAX",
        "og_description": "WireGuard как основной протокол TYRAX.",
        "h1": "WIREGUARD VPN",
        "hero_sub": "TYRAX начинает с WireGuard — быстрый и лёгкий протокол. При блокировках автоматически переключается на VLESS/Reality.",
        "cta_primary": {"href": "/download/windows/TYRAX-Setup.exe", "label": "СКАЧАТЬ WINDOWS", "attrs": 'data-dl="windows" download'},
        "seo_paragraphs": [
            "<strong>WireGuard VPN</strong> — первый протокол в цепочке TYRAX. Подробнее: <a href=\"/vless-vpn.html\">VLESS VPN</a>.",
        ],
    },
    {
        "file": "split-tunnel.html",
        "breadcrumb": "Split Tunnel",
        "title": "Split Tunnel VPN — TYRAX | RU-сервисы напрямую",
        "description": "Split Tunnel в TYRAX: Яндекс, VK, банки, Госуслуги — напрямую. Остальной трафик через VPN. Список обновляется с сервера.",
        "og_title": "Split Tunnel — TYRAX",
        "og_description": "Split Tunnel RU — российские сервисы без VPN.",
        "h1": "SPLIT TUNNEL VPN",
        "hero_sub": "Российские домены идут напрямую — банки, Госуслуги, Яндекс работают стабильно. Зарубежные сервисы — через TYRAX.",
        "cta_primary": {"href": "/", "label": "СКАЧАТЬ TYRAX"},
        "features": [
            {"title": "RU bypass list", "text": "Яндекс, VK, Сбер, Тинькофф, Госуслуги — без туннеля."},
            {"title": "Server sync", "text": "Список доменов обновляется с сервера автоматически."},
        ],
        "seo_paragraphs": [
            "<strong>Split tunnel VPN</strong> для России — уникальная функция TYRAX.",
        ],
    },
    {
        "file": "socks5-proxy.html",
        "breadcrumb": "SOCKS5 прокси",
        "title": "SOCKS5 прокси vs VPN — TYRAX протокол доступа",
        "description": "Чем VPN TYRAX лучше SOCKS5 прокси: системный туннель, шифрование всего трафика, автопротокол. Когда нужен VPN, а не прокси.",
        "og_title": "SOCKS5 прокси — TYRAX",
        "og_description": "VPN vs SOCKS5 — полный системный туннель.",
        "h1": "SOCKS5 ПРОКСИ И VPN",
        "hero_sub": "SOCKS5 прокси шифрует только приложения с ручной настройкой. TYRAX — системный VPN-туннель для всего трафика с автодиагностикой.",
        "cta_primary": {"href": "/free.html", "label": "ПОПРОБОВАТЬ FREE"},
        "seo_paragraphs": [
            "Ищете <strong>SOCKS5 прокси</strong>? TYRAX — полноценный VPN, а не точечный прокси. <a href=\"/vless-proxy.html\">VLESS прокси</a> vs системный туннель.",
        ],
    },
    {
        "file": "vless-proxy.html",
        "breadcrumb": "VLESS прокси",
        "title": "VLESS прокси — TYRAX | Reality, подписка, Happ",
        "description": "VLESS прокси через подписку TYRAX. Reality XHTTP для iOS/Mac в Happ. Отличие от SOCKS5 и полноценного VPN-туннеля.",
        "og_title": "VLESS прокси — TYRAX",
        "og_description": "VLESS через подписку TYRAX + Happ.",
        "h1": "VLESS ПРОКСИ",
        "hero_sub": "VLESS в TYRAX — часть протокола доступа. На iOS/Mac через Happ импортируется как подписка с VLESS Reality endpoints.",
        "cta_primary": {"href": "/vless-vpn.html", "label": "VLESS VPN"},
        "seo_paragraphs": [
            "<strong>VLESS прокси</strong> и <a href=\"/vless-vpn.html\">VLESS VPN</a> в экосистеме TYRAX.",
        ],
    },
    {
        "file": "blog.html",
        "breadcrumb": "Блог",
        "title": "Блог TYRAX — VPN, обход блокировок, инструкции",
        "description": "Статьи о VPN: как скачать, бесплатные VPN без рекламы, что делать если VPN не работает. Блог TYRAX.",
        "og_title": "Блог TYRAX",
        "og_description": "VPN-статьи и инструкции.",
        "h1": "БЛОГ TYRAX",
        "hero_sub": "Практические статьи о VPN, обходе блокировок и настройке TYRAX на всех платформах.",
        "features_label": "СТАТЬИ",
        "features_title": "ПОСЛЕДНИЕ МАТЕРИАЛЫ",
        "features": [
            {"title": "Как скачать VPN на Windows", "text": "Пошаговая инструкция установки TYRAX на компьютер. → /blog-kak-skachat-vpn-windows.html"},
            {"title": "Лучший бесплатный VPN 2026", "text": "Сравнение free VPN: реклама, лимиты, серверы. Почему TYRAX FREE — 1 ГБ без рекламы. → /blog-luchshiy-besplatnyy-vpn.html"},
            {"title": "VPN не работает — что делать", "text": "Диагностика: блокировки, лимит трафика, смена протокола. → /blog-vpn-ne-rabotaet.html"},
        ],
        "seo_paragraphs": [
            "Блог TYRAX — <strong>VPN</strong>, <strong>прокси</strong>, обход блокировок, инструкции.",
        ],
    },
    {
        "file": "blog-kak-skachat-vpn-windows.html",
        "breadcrumb": "Как скачать VPN на Windows",
        "title": "Как скачать VPN на Windows — инструкция TYRAX 2026",
        "description": "Как скачать и установить VPN на Windows 10/11. TYRAX: загрузка, установка, регистрация, тариф FREE 1 ГБ.",
        "og_title": "Как скачать VPN на Windows",
        "og_description": "Инструкция TYRAX для Windows.",
        "h1": "КАК СКАЧАТЬ VPN НА WINDOWS",
        "hero_sub": "Пошаговая инструкция: скачать TYRAX-Setup.exe, установить, войти, нажать ENTER. VPN для компьютера за 2 минуты.",
        "cta_primary": {"href": "/download/windows/TYRAX-Setup.exe", "label": "СКАЧАТЬ TYRAX", "attrs": 'data-dl="windows" download'},
        "article": True,
        "features": [
            {"title": "Шаг 1: Скачать", "text": "Перейдите на tyrax.tech/vpn-windows.html и скачайте TYRAX-Setup.exe."},
            {"title": "Шаг 2: Установить", "text": "Запустите installer, следуйте мастеру установки. Разрешите VPN-профиль Windows."},
            {"title": "Шаг 3: Войти", "text": "Telegram или email — создайте аккаунт в клиенте."},
            {"title": "Шаг 4: ENTER", "text": "Нажмите ENTER — протокол выберет лучший сервер автоматически."},
        ],
        "seo_paragraphs": [
            "Это полноценный <strong>VPN для Windows</strong>, не расширение. <a href=\"/free.html\">1 ГБ бесплатно</a>.",
        ],
    },
    {
        "file": "blog-luchshiy-besplatnyy-vpn.html",
        "breadcrumb": "Лучший бесплатный VPN 2026",
        "title": "Лучший бесплатный VPN 2026 — без рекламы | TYRAX",
        "description": "Как выбрать бесплатный VPN в 2026: реклама, лимиты, серверы. TYRAX FREE — 1 ГБ/мес, все серверы, zero ads.",
        "og_title": "Лучший бесплатный VPN 2026",
        "og_description": "TYRAX FREE — 1 ГБ без рекламы.",
        "h1": "ЛУЧШИЙ БЕСПЛАТНЫЙ VPN 2026",
        "hero_sub": "Большинство free VPN показывают рекламу или продают данные. TYRAX FREE — 1 ГБ каждый месяц, все серверы, полная автоматизация, ноль рекламы.",
        "cta_primary": {"href": "/free.html", "label": "TYRAX FREE"},
        "article": True,
        "features": [
            {"title": "Без рекламы", "text": "TYRAX не показывает баннеры и не продаёт данные рекламодателям на FREE и платных тарифах."},
            {"title": "Все серверы", "text": "FREE не ограничен медленными нодами — тот же пул, что у платных пользователей."},
            {"title": "1 ГБ/мес навсегда", "text": "Не trial на 7 дней — постоянный бесплатный тариф с ежемесячным обновлением лимита."},
        ],
        "seo_paragraphs": [
            "<strong>Бесплатный VPN без рекламы</strong> — редкость. TYRAX FREE — наш ответ.",
        ],
    },
    {
        "file": "blog-vpn-ne-rabotaet.html",
        "breadcrumb": "VPN не работает",
        "title": "VPN не работает — что делать | TYRAX",
        "description": "VPN не подключается или медленный: проверка тарифа, лимита FREE 1 ГБ, блокировок, автопротокола TYRAX.",
        "og_title": "VPN не работает — что делать",
        "og_description": "Диагностика проблем VPN TYRAX.",
        "h1": "VPN НЕ РАБОТАЕТ — ЧТО ДЕЛАТЬ",
        "hero_sub": "TYRAX автоматически диагностирует сеть и переключает протокол. Если проблема остаётся — проверьте тариф, лимит трафика и обратитесь в поддержку.",
        "cta_primary": {"href": "/contacts.html", "label": "ПОДДЕРЖКА"},
        "article": True,
        "features": [
            {"title": "Проверьте лимит FREE", "text": "Тариф FREE — 1 ГБ/мес. При исчерпании лимита подключение недоступно до обновления."},
            {"title": "Дождитесь автопротокола", "text": "TYRAX переключит WireGuard → VLESS → Shadowsocks при блокировках. Подождите 30–60 секунд."},
            {"title": "Напишите в поддержку", "text": "support@tyrax.tech или @tyrax_support в Telegram."},
        ],
        "seo_paragraphs": [
            "Если <strong>VPN не работает</strong>, TYRAX сначала пробует автодиагностику. <a href=\"/obhod-blokirovok.html\">Обход блокировок</a>.",
        ],
    },
]


def main() -> None:
    for page in PAGES:
        path = ROOT / page["file"]
        path.write_text(render(page), encoding="utf-8")
        print(f"Wrote {path.name}")


if __name__ == "__main__":
    main()
