# SEO Overhaul Design — tyrax.tech

**Date:** 2026-07-03  
**Status:** Implemented

## Goal

Raise tyrax.tech from ~5/10 to production-grade SEO: fix P0 issues, expand semantic coverage (VPN/proxy clusters), optimize for AI search citations.

## Approach (recommended)

**Static HTML expansion** — 15 new landing/blog pages generated from Python template, unified footer/nav, schema per page. No build pipeline required; deploy via existing `deploy-website.sh`.

Alternatives rejected:
- SPA/Next.js migration — too much scope, hurts time-to-ship
- Keyword footer stuffing — spam risk, removed

## Changes

### P0 fixes
- Remove fake `aggregateRating` JSON-LD
- Remove `footer-keywords` spam block
- H1/title alignment on homepage
- OG image 1200×630
- FAQ schema synced with visible FAQ

### New pages (15)
Platform: vpn-windows, vpn-android, vpn-iphone, vpn-mac  
Intent: obhod-blokirovok, no-logs  
Tech: vless-vpn, wireguard-vpn, split-tunnel, socks5-proxy, vless-proxy  
Blog: blog hub + 3 articles

### GEO / AI
- `llms.txt` + `llms-full.txt`
- robots.txt allows GPTBot, ClaudeBot, PerplexityBot, Google-Extended
- Quotable FAQ blocks, Article/HowTo schema

### Infra
- `nginx-tyrax.tech.conf`: www→apex, security headers, static cache
- sitemap.xml: 24 URLs

### Performance
- Preload CSS/OG image
- Matrix rain disabled on mobile
- font-display=swap via Google Fonts URL

## Post-deploy checklist
1. Apply nginx config on VPS, reload nginx
2. Submit sitemap in Google Search Console + Яндекс.Вебмастер
3. Request indexing for new URLs
4. Monitor Core Web Vitals in PageSpeed Insights
