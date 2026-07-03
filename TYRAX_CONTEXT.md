# TYRAX — PROJECT CONTEXT

> Read this file before touching any code. It is the single source of truth for the entire project.

---

## WHAT IS TYRAX

TYRAX is not a VPN.
TYRAX is an access protocol.

It removes restrictions. It doesn't ask permission.
The product is an Android application that connects users to the open internet — silently, automatically, without effort.

**Core promise:** the user forgets the VPN is on. It just works.

---

## BRAND DNA

**Slogan:** БЕЗ РАЗРЕШЕНИЯ / WITHOUT PERMISSION

**Positioning:** Aggressive. Dominant. With a sharp sexual undercurrent.
Think: something that takes what it wants. No apology. No explanation.
The brand has edge — provocative, but never cheap.

**Emotional core:** control · dominance · entry by force

**What TYRAX is NOT:**
- Not friendly
- Not safe/cozy
- Not corporate
- Not "privacy-first" in a boring way

**Visual identity:**
- Colors: `#000000` · `#FFFFFF` · `#FF1E1E`
- Style: military terminal · glitch · digital fracture · minimal
- NO gradients, NO rounded soft shapes, NO emoji in UI
- Typography: bold, uppercase, condensed — like a classified document

**UX Language — always use TYRAX vocabulary:**

| Generic | TYRAX |
|---|---|
| Connected | ACCESS GRANTED |
| Disconnected | OUTSIDE SYSTEM |
| VPN / Tunnel | PROTOCOL / TUNNEL |
| Server | NODE |
| Protection | NO RESTRICTIONS |
| Connecting... | BREACHING NETWORK… |
| Settings | CONTROL |
| Account | IDENTITY |
| Subscribe | ENTER |
| Free trial | TEST ACCESS |

---

## PRODUCT OVERVIEW

### Core Features

1. **Auto Node Selection** — pings all available nodes on launch, picks best by RTT + packet loss
2. **Auto Protocol Fallback** — WireGuard → VLESS/Reality → Shadowsocks → OpenVPN/TCP
3. **Silent Reconnect** — monitors connection every 30s, switches nodes without user interaction
4. **Network Handoff** — seamless Wi-Fi ↔ LTE transition via Android NetworkCallback
5. **Split Tunneling (RU services)** — RU domains (Yandex, VK, banks, Gosuslugi) bypass tunnel; everything else goes through TYRAX
6. **Split domain list** updates from server silently (push config)
7. **Auth:** login/password OR Telegram Bot deep link flow

### Subscription Tiers

| Tier | Price | Devices | Traffic | Nodes | Priority |
|---|---|---|---|---|---|
| **FREE** | 0 ₽ | 1 | 1 GB/month | All | No |
| **CORE** | 199 ₽/mo | 2 | Unlimited | All | No |
| **SHADOW** | 349 ₽/mo | Up to 5 | Unlimited | All | Yes |
| **DOMINION** | 649 ₽/mo | Up to 10 own + invite 3 accounts | Unlimited | All | #1 Priority |

### Discount Periods (CORE, SHADOW, DOMINION only)

| Period | Discount |
|---|---|
| 3 months | 10% |
| 6 months | 15% |
| 12 months | 20% |

### Device Logic

- Each device gets a unique WireGuard keypair issued by the backend
- Backend counts active devices per account; refuses new config if limit exceeded
- User sees "My Devices" list in app with option to remove any device
- DOMINION "invite" flow: owner sends invite by account ID → invitee accepts → their account inherits DOMINION limits
- Invited accounts count toward owner's 10-device total
- Owner can remove invited account at any time; invitee can leave at any time
- Invited accounts have `parent_subscription_id` field in DB

### FREE Tier Rules

- No ads — ever (across all tiers)
- Full auto-reconnect and auto-protocol switching — same as paid
- All nodes accessible
- Only restriction: 1 GB/month traffic cap and 1 device

---

## TECH STACK

### Backend
- Language: **Go**
- Framework: **Fiber** (fast, minimal)
- DB: **PostgreSQL** (users, subscriptions, node configs)
- Auth: **JWT** + Telegram Bot API (deep link OAuth flow)
- VPN config: generates WireGuard `.conf` and VLESS/Xray configs per user
- Deployment: Docker + docker-compose

### Android Client
- Language: **Kotlin**
- UI: **Jetpack Compose** (dark terminal theme)
- VPN layer: **WireGuard Android** library + Xray-core (local SOCKS proxy)
- Architecture: **MVVM + Clean Architecture**
  - `domain/` — use cases, models
  - `data/` — repositories, API clients, VPN engine
  - `presentation/` — Compose screens, ViewModels
- Network: **Retrofit + OkHttp**
- DI: **Hilt**
- Storage: **DataStore** (preferences), **Room** (node cache)

### Infrastructure
- VPS providers: Hetzner / Vultr
- Locations: NL, DE, FI (start), expand later
- Panel: **Marzban** or **3x-ui** (VLESS/Xray management)
- WireGuard: native kernel module on Ubuntu 22.04
- Reverse proxy: **Nginx**

---

## PROJECT STRUCTURE

```
tyrax/
├── TYRAX_CONTEXT.md          ← you are here
├── .cursorrules              ← code style for Cursor AI
├── tyrax-backend/            ← Go API server
├── tyrax-android/            ← Android application
└── tyrax-infra/              ← server configs, docker-compose
```

---

## SCREENS & FLOWS

### Onboarding (3 slides)
```
Slide 1: "ОНИ РЕШАЮТ, ЧТО ТЕБЕ МОЖНО ВИДЕТЬ"
Slide 2: "МЫ ЭТО УБИРАЕМ"
Slide 3: "БЕЗ РАЗРЕШЕНИЯ" → [ENTER]
```

### Main Screen
- Large status indicator: `STATUS: OUTSIDE SYSTEM` / `STATUS: ACCESS GRANTED`
- Single giant button: `ENTER` / `DISCONNECT`
- Current node tag: `NODE: NL-01 · OPEN`
- Minimal. No clutter. Maximum tension.

### Connection Animation
```
BREACHING NETWORK…
[glitch progress bar]
NODE ACQUIRED
ACCESS GRANTED
```

### Nodes Screen
- List of nodes with status badges: `OPEN` · `MONITORED` · `HEAVILY RESTRICTED`
- Ping display
- No flags — codenames only (NL-01, DE-02, FI-01)

### Subscription Screen
- Three tiers: CORE / SHADOW / DOMINION
- Framed as "levels of access", not "features"
- CTA: "UNLOCK" not "Buy"

---

## RULES FOR AI (CURSOR)

When generating code for this project:

1. **No friendly UI copy** — always use TYRAX vocabulary (see table above)
2. **Compose UI:** dark background `#000000`, text `#FFFFFF`, accent `#FF1E1E`. No Material3 default colors.
3. **No placeholder lorem ipsum** — use TYRAX-style copy even in mocks
4. **Architecture first** — always follow MVVM + Clean Architecture layers
5. **No hardcoded strings in UI** — use `strings.xml` (Android) or constants file
6. **Backend routes** use kebab-case: `/api/v1/auth/telegram-callback`
7. **Error messages** stay on brand: "CONNECTION FAILED. NODE UNAVAILABLE." not "Error 500"
8. **Comments in English** (code), **UI copy can be RU or EN** depending on context
