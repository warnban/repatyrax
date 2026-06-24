# TYRAX — CURSOR PROMPTS
## От нуля до MVP APK: пошаговое руководство

> Каждый промпт самодостаточен. Cursor не помнит предыдущий шаг — весь контекст уже внутри.
> Модель указана для каждого этапа. Используй Cursor Agent (Cmd+I), не чат.

---

## ПРО ПРОЦЕСС

**Перед первым этапом сделай одно действие:**
1. Распакуй архив `tyrax-starter.zip`
2. Открой папку `tyrax/` в Cursor как проект (File → Open Folder)
3. Убедись что в корне есть `.cursorrules` и `TYRAX_CONTEXT.md`
4. Всё — Cursor автоматически читает эти файлы при каждом запросе

**Порядок этапов строгий:**
- Этапы 1–3: backend (Go) — делать первым, Android зависит от API
- Этапы 4–9: Android (Kotlin/Compose)
- Этап 10: серверы и деплой
- Этап 11: сборка APK

---

## ЭТАП 0 — Подготовка окружения
### Модель: не нужна (делаешь руками)

Установи инструменты если нет:

**Go (backend):**
```bash
# Скачай с https://go.dev/dl/
# Проверь:
go version  # должно быть 1.22+
```

**Android Studio:**
```
Скачай с https://developer.android.com/studio
При установке выбери: Android SDK, Android Virtual Device
```

**Docker (для PostgreSQL локально):**
```bash
# https://docs.docker.com/get-docker/
docker --version
```

**Создай структуру проекта в Cursor:**
```
tyrax/                    ← уже есть из архива
├── tyrax-backend/        ← уже есть
├── tyrax-android/        ← уже есть (создадим через Android Studio)
└── tyrax-infra/          ← создай пустую папку
```

**Создай Android проект через Android Studio:**
```
File → New → New Project
Template: Empty Activity
Name: TYRAX
Package: com.tyrax
Language: Kotlin
Min SDK: API 26 (Android 8.0)
Build config: Kotlin DSL
Сохрани в папку tyrax/tyrax-android/
```

После создания — открой проект в Cursor и продолжай.

---

## ЭТАП 1 — Backend: БД + Авторизация
### Модель: `claude-opus-4` (критическая архитектура, делается один раз)

```
Read TYRAX_CONTEXT.md and .cursorrules before writing any code.

You are building the TYRAX VPN backend in Go. Project is in tyrax-backend/.

TASK: Set up the full database layer and authentication system.

--- STEP 1: Go module init ---
Initialize Go module:
  go mod init github.com/tyrax/tyrax-backend

Add dependencies:
  go get github.com/gofiber/fiber/v2
  go get github.com/golang-jwt/jwt/v5
  go get github.com/jackc/pgx/v5
  go get golang.org/x/crypto
  go get github.com/joho/godotenv

--- STEP 2: Database connection ---
Create internal/database/postgres.go:
- Connect via pgx/v5 pool using DATABASE_URL env var
- Pool config: MaxConns=25, MinConns=2, MaxConnLifetime=1h
- Ping on startup, fatal log if fails
- Export global Pool variable

--- STEP 3: Migrations runner ---
Create internal/database/migrate.go:
- Read and execute all .sql files from migrations/ folder in order
- Log each migration name as it runs

--- STEP 4: Repositories ---

Create internal/repository/user_repository.go:
```go
type UserRepository interface {
    Create(ctx, email, passwordHash string) (*model.User, error)
    FindByEmail(ctx, email string) (*model.User, error)
    FindByID(ctx, id string) (*model.User, error)
    FindByTelegramID(ctx, telegramID int64) (*model.User, error)
    UpdateTelegramID(ctx, userID string, telegramID int64) error
    UpdateSubscription(ctx, userID string, tier model.SubscriptionTier, endsAt time.Time) error
    UpdateParentSubscription(ctx, userID string, parentID *string) error
    CountDevices(ctx, userID string) (int, error)
    GetDeviceLimit(tier model.SubscriptionTier) int
    GetTrafficUsed(ctx, userID string) (int64, error)
    GetTrafficLimit(tier model.SubscriptionTier) int64
}
```

Create internal/repository/device_repository.go:
```go
type DeviceRepository interface {
    Create(ctx, userID, name, publicKey string) (*model.Device, error)
    ListByUser(ctx, userID string) ([]model.Device, error)
    Delete(ctx, deviceID, userID string) error
    FindByPublicKey(ctx, publicKey string) (*model.Device, error)
}
```

Create internal/repository/invite_repository.go:
```go
type InviteRepository interface {
    Create(ctx, ownerID, inviteeID string) (*model.Invite, error)
    FindPending(ctx, inviteeID string) (*model.Invite, error)
    Accept(ctx, inviteID string) error
    Delete(ctx, ownerID, inviteeID string) error
    CountByOwner(ctx, ownerID string) (int, error)
}
```

--- STEP 5: Auth Service ---
Create internal/service/auth_service.go with methods:

Register(ctx, email, password string) (*AuthResponse, error):
- Validate email format
- Check email not taken
- bcrypt password (cost 12)
- Create user with tier=FREE
- Return JWT

Login(ctx, email, password string) (*AuthResponse, error):
- Find user by email
- bcrypt compare
- Return JWT

GenerateTelegramToken(ctx, userID string) (string, error):
- Generate random 32-byte hex token
- Store in telegram_auth_tokens with 10min expiry
- Return token

HandleTelegramCallback(ctx, token string, telegramID int64) (*AuthResponse, error):
- Find valid (non-expired, non-used) token
- Mark token as used
- Link telegramID to user
- Return JWT

JWT claims must include: UserID, SubscriptionTier, ExpiresAt (24h)

AuthResponse struct:
```go
type AuthResponse struct {
    Token string          `json:"token"`
    User  *model.User    `json:"user"`
}
```

--- STEP 6: Auth Handlers ---
Create internal/handler/auth_handler.go:

POST /api/v1/auth/register
  Body: { "email": "...", "password": "..." }
  Response: { "status": "ok", "data": AuthResponse }
  Errors: "IDENTITY ALREADY EXISTS" (409), "INVALID REQUEST" (400)

POST /api/v1/auth/login
  Body: { "email": "...", "password": "..." }
  Response: { "status": "ok", "data": AuthResponse }
  Errors: "INVALID CREDENTIALS" (401)

GET /api/v1/auth/telegram-init [JWT required]
  Response: { "status": "ok", "data": { "token": "...", "bot_url": "https://t.me/tyraxvpnbot?start=TOKEN" } }

POST /api/v1/auth/telegram-callback [NO JWT - called by Telegram bot]
  Body: { "token": "...", "telegram_id": 123456 }
  Response: { "status": "ok" }

--- STEP 7: Update main.go ---
Wire everything together in cmd/server/main.go:
- Load .env with godotenv
- Connect DB
- Run migrations
- Init repositories and services
- Register routes
- Start Fiber server

--- STEP 8: DB schema update ---
Update migrations/001_init.sql to add:
- devices table: id, user_id, name, public_key, created_at
- subscription_invites table: id, owner_id, invitee_id, status (pending/accepted), created_at
- Add parent_subscription_id column to users table
- Add traffic_used column to users table (int8, default 0)

--- EXPECTED FILES ---
internal/database/postgres.go
internal/database/migrate.go
internal/repository/user_repository.go
internal/repository/user_repository_impl.go
internal/repository/device_repository.go
internal/repository/device_repository_impl.go
internal/repository/invite_repository.go
internal/repository/invite_repository_impl.go
internal/service/auth_service.go
internal/handler/auth_handler.go
migrations/001_init.sql (updated)
go.mod
go.sum

Run: go build ./... to verify no errors.
```

---

## ЭТАП 2 — Backend: VPN конфиги и ноды
### Модель: `gemini-2.5-pro` (большой контекст WireGuard + VLESS спецификации)

```
Read TYRAX_CONTEXT.md and .cursorrules before writing any code.

You are building VPN config generation for TYRAX backend (Go, Fiber).
Project: tyrax-backend/
Auth middleware exists at: internal/middleware/auth.go
User repository exists at: internal/repository/user_repository.go
DB models at: internal/model/

TASK: Build node management and VPN config generation.

--- STEP 1: Node model update ---
Ensure internal/model/user.go has:
```go
type Node struct {
    ID         string     `db:"id" json:"id"`
    Codename   string     `db:"codename" json:"codename"`   // "NL-01"
    Country    string     `db:"country" json:"country"`
    Host       string     `db:"host" json:"host"`
    Port       int        `db:"port" json:"port"`
    Protocol   string     `db:"protocol" json:"protocol"`   // "wireguard" | "vless" | "shadowsocks"
    Status     NodeStatus `db:"status" json:"status"`
    PublicKey  string     `db:"public_key" json:"public_key"` // WireGuard server public key
    XrayConfig string     `db:"xray_config" json:"-"`          // VLESS/Xray JSON config template
    PingMS     int        `db:"ping_ms" json:"ping_ms"`
    MinTier    string     `db:"min_tier" json:"min_tier"`     // all tiers get all nodes
}
```

--- STEP 2: Node Repository ---
Create internal/repository/node_repository.go:
```go
type NodeRepository interface {
    List(ctx context.Context) ([]model.Node, error)
    FindByID(ctx context.Context, id string) (*model.Node, error)
    UpdatePing(ctx context.Context, nodeID string, pingMS int) error
    GetBest(ctx context.Context) (*model.Node, error) // lowest ping, status=OPEN
}
```

--- STEP 3: WireGuard config generator ---
Create pkg/vpnconfig/wireguard.go:

Function: GenerateClientConfig(serverNode Node, clientPrivateKey, clientPublicKey, serverPublicKey, clientIP string) string

Returns a valid WireGuard .conf file as string:
```
[Interface]
PrivateKey = {clientPrivateKey}
Address = {clientIP}/32
DNS = 1.1.1.1, 8.8.8.8
MTU = 1420

[Peer]
PublicKey = {serverPublicKey}
Endpoint = {serverHost}:{serverPort}
AllowedIPs = 0.0.0.0/0, ::/0
PersistentKeepalive = 25
```

Function: GenerateKeypair() (privateKey, publicKey string, err error)
- Use golang.zx2c4.com/wireguard/wgctrl/wgtypes
- Generate new private key, derive public key
- Return both as base64 strings

--- STEP 4: VLESS config generator ---
Create pkg/vpnconfig/vless.go:

Function: GenerateVlessConfig(node Node, userUUID string) string

Returns Xray-core compatible JSON config that:
- Uses VLESS protocol with XTLS-Reality
- Sets outbound to node.Host:node.Port
- Embeds userUUID as client ID
- Includes inbound SOCKS5 on 127.0.0.1:10808

The config format must be valid Xray-core JSON (research the format).

--- STEP 5: Device + Config Service ---
Create internal/service/vpn_service.go:

AddDevice(ctx, userID, deviceName string) (*DeviceConfig, error):
- Check device count vs tier limit (use DeviceLimit from PAYMENTS.md)
- Generate WireGuard keypair
- Store device (userID, name, publicKey) in DB
- Pick best node (lowest ping, OPEN status)
- Generate WireGuard config for this device + node
- Return DeviceConfig { WireGuardConf, NodeList }

GetConfig(ctx, userID, devicePublicKey string) (*VPNConfig, error):
- Find device by publicKey
- Get best available node
- Return config for that node
- If node protocol=wireguard → WireGuard config
- If node protocol=vless → VLESS config

GetNodes(ctx) ([]NodeResponse, error):
- Return all nodes with status, ping, codename
- Do NOT expose host/port/keys to client

--- STEP 6: VPN Handlers ---
Create internal/handler/vpn_handler.go:

POST /api/v1/vpn/device [JWT required]
  Body: { "name": "My Phone" }
  Response: { "status": "ok", "data": { "device_id": "...", "wireguard_conf": "...", "nodes": [...] } }
  Errors: "DEVICE LIMIT REACHED" (403), "NODE UNAVAILABLE" (503)

GET /api/v1/vpn/config [JWT required]
  Query: ?device_public_key=BASE64KEY
  Response: { "status": "ok", "data": { "protocol": "wireguard|vless", "config": "..." } }

DELETE /api/v1/vpn/device/:deviceID [JWT required]
  Response: { "status": "ok" }

GET /api/v1/nodes [JWT required]
  Response: { "status": "ok", "data": [{ "codename": "NL-01", "country": "Netherlands", "status": "OPEN", "ping_ms": 12 }] }

GET /api/v1/vpn/split-domains [JWT required]
  Response: { "status": "ok", "data": { "domains": ["yandex.ru", "vk.com", ...], "updated_at": "..." } }
  Include hardcoded list of 50+ RU domains: yandex.ru, ya.ru, vk.com, vkontakte.ru, ok.ru, mail.ru,
  gosuslugi.ru, mos.ru, sberbank.ru, tinkoff.ru, vtb.ru, alfabank.ru, raiffeisen.ru,
  ozon.ru, wildberries.ru, avito.ru, hh.ru, kinopoisk.ru, ivi.ru, rutube.ru,
  2gis.ru, drom.ru, auto.ru, rbc.ru, kommersant.ru, ria.ru, lenta.ru, meduza.io

--- STEP 7: Wire into main.go ---
Register all new routes in cmd/server/main.go under the protected group.

--- EXPECTED FILES ---
internal/repository/node_repository.go
internal/repository/node_repository_impl.go
internal/service/vpn_service.go
internal/handler/vpn_handler.go
pkg/vpnconfig/wireguard.go
pkg/vpnconfig/vless.go

Run: go build ./... to verify.
```

---

## ЭТАП 3 — Backend: Платежи
### Модель: `claude-sonnet-4` (структурированные HTTP клиенты, ~50% дешевле)

```
Read TYRAX_CONTEXT.md, .cursorrules, and PAYMENTS.md before writing any code.

You are building the payments layer for TYRAX backend (Go, Fiber).
Project: tyrax-backend/
Existing: internal/model/, internal/repository/, internal/middleware/auth.go

PAYMENTS.md contains full FreeKassa and CryptoPay API documentation.
Read it carefully — use exact parameter names, signature algorithms, and webhook verification logic from that file.

TASK: Build complete payment integration.

--- STEP 1: Order model ---
Create/update internal/model/order.go:
```go
type PaymentMethod string
const (
    PaymentSBP    PaymentMethod = "SBP"      // FreeKassa i=44
    PaymentCardRF PaymentMethod = "CARD_RF"  // FreeKassa i=36
    PaymentCrypto PaymentMethod = "CRYPTO"   // CryptoPay
)

type OrderStatus string
const (
    OrderNew       OrderStatus = "NEW"
    OrderPaid      OrderStatus = "PAID"
    OrderCancelled OrderStatus = "CANCELLED"
    OrderRefunded  OrderStatus = "REFUNDED"
)

type Order struct {
    ID              string        `db:"id"`
    UserID          string        `db:"user_id"`
    Tier            string        `db:"tier"`            // CORE/SHADOW/DOMINION
    Months          int           `db:"months"`          // 1/3/6/12
    AmountRUB       float64       `db:"amount_rub"`
    PaymentMethod   PaymentMethod `db:"payment_method"`
    ExternalOrderID string        `db:"external_order_id"`
    Status          OrderStatus   `db:"status"`
    CreatedAt       time.Time     `db:"created_at"`
    PaidAt          *time.Time    `db:"paid_at"`
}
```

Add orders table to migrations (new migration file 002_orders.sql):
```sql
CREATE TABLE orders (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id          UUID NOT NULL REFERENCES users(id),
    tier             TEXT NOT NULL,
    months           INT NOT NULL DEFAULT 1,
    amount_rub       NUMERIC(10,2) NOT NULL,
    payment_method   TEXT NOT NULL,
    external_order_id TEXT,
    status           TEXT NOT NULL DEFAULT 'NEW',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    paid_at          TIMESTAMPTZ
);

CREATE TABLE subscription_invites (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    owner_id    UUID NOT NULL REFERENCES users(id),
    invitee_id  UUID NOT NULL REFERENCES users(id),
    status      TEXT NOT NULL DEFAULT 'pending',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

--- STEP 2: Pricing logic ---
Create internal/service/pricing_service.go:

Tier prices (monthly base):
- CORE: 199 RUB
- SHADOW: 349 RUB
- DOMINION: 649 RUB

```go
func CalculatePrice(tier string, months int) float64 {
    base := map[string]float64{"CORE": 199, "SHADOW": 349, "DOMINION": 649}[tier]
    discount := map[int]float64{1: 1.0, 3: 0.90, 6: 0.85, 12: 0.80}[months]
    return math.Round(base * float64(months) * discount)
}
```

--- STEP 3: FreeKassa client ---
Create pkg/freekassa/client.go following EXACTLY the API spec in PAYMENTS.md:

Config (from env):
- FREEKASSA_SHOP_ID
- FREEKASSA_API_KEY
- FREEKASSA_SECRET_WORD_2

Methods:
- CreateOrder(ctx, shopID int, paymentMethodID int, email, ip string, amount float64, internalOrderID string) (*CreateOrderResponse, error)
  → POST https://api.fk.life/v1/orders/create
  → Signature: HMAC-SHA256 of sorted values joined by |
  → Returns location (payment URL)

- VerifyWebhook(merchantID, amount, orderID, receivedSign string) bool
  → md5(merchantID + ":" + amount + ":" + SECRET_WORD_2 + ":" + orderID)

Trusted IPs for webhook: 168.119.157.136, 168.119.60.227, 178.154.197.79, 51.250.54.238

--- STEP 4: CryptoPay client ---
Create pkg/cryptopay/client.go following EXACTLY the API spec in PAYMENTS.md:

Config (from env):
- CRYPTO_PAY_TOKEN

Methods:
- CreateInvoice(ctx, amountRUB float64, tier, userID, orderID string) (*Invoice, error)
  → POST https://pay.crypt.bot/api/createInvoice
  → Header: Crypto-Pay-API-Token: TOKEN
  → currency_type=fiat, fiat=RUB, accepted_assets=USDT,TON,BTC
  → payload = userID + "|" + orderID
  → expires_in = 3600
  → paid_btn_name = openBot, paid_btn_url = https://t.me/tyraxvpnbot
  → Returns bot_invoice_url

- VerifyWebhook(token, body, signature string) bool
  → sha256(token) → HMAC-SHA256(body) → compare with header crypto-pay-api-signature

--- STEP 5: Payment Service ---
Create internal/service/payment_service.go:

CreateOrder(ctx, userID, tier, paymentMethod string, months int, email, ip string) (*CreateOrderResult, error):
- Calculate price via CalculatePrice
- Create Order in DB (status=NEW)
- If paymentMethod=SBP → FreeKassa i=44
- If paymentMethod=CARD_RF → FreeKassa i=36
- If paymentMethod=CRYPTO → CryptoPay createInvoice
- Update order with external_order_id
- Return { order_id, payment_url }

HandleFreekassaWebhook(ctx, params map[string]string) error:
- Verify IP is trusted
- Verify SIGN via VerifyWebhook
- Find order by MERCHANT_ORDER_ID
- If already PAID → skip (idempotency)
- Mark order PAID, set paid_at
- Activate subscription: update user tier + subscription_end (now + months)

HandleCryptoPayWebhook(ctx, body, signature string) error:
- Verify signature
- Check update_type == "invoice_paid"
- Parse payload → userID + orderID
- Mark order PAID
- Activate subscription

ActivateSubscription(ctx, userID, tier string, months int) error:
- Calculate subscription_end = NOW() + months
- UPDATE users SET subscription_tier=tier, subscription_end=... WHERE id=userID

--- STEP 6: Subscription invite service ---
Create internal/service/invite_service.go:

SendInvite(ctx, ownerID, inviteeAccountID string) error:
- Check owner has DOMINION tier
- Check owner's current invite count < 3
- Check invitee exists
- Create pending invite record

AcceptInvite(ctx, inviteeID, inviteID string) error:
- Find pending invite for this invitee
- Set users.parent_subscription_id = owner_id
- Mark invite accepted

RemoveInvite(ctx, ownerID, inviteeID string) error:
- Delete invite record
- Set users.parent_subscription_id = NULL for invitee

--- STEP 7: Handlers ---
Create internal/handler/payment_handler.go:

POST /api/v1/payment/create [JWT]
  Body: { "tier": "CORE", "payment_method": "SBP", "months": 3, "email": "...", "ip": "..." }
  Response: { "status": "ok", "data": { "order_id": "...", "payment_url": "...", "amount_rub": 537 } }

GET /api/v1/payment/status/:orderID [JWT]
  Response: { "status": "ok", "data": { "order_status": "NEW|PAID", "tier": "CORE" } }

GET /api/v1/subscription [JWT]
  Response: { "status": "ok", "data": { "tier": "SHADOW", "ends_at": "...", "devices_count": 2, "devices_limit": 5 } }

POST /webhooks/freekassa [PUBLIC - no JWT]
POST /webhooks/crypto-pay [PUBLIC - no JWT]

POST /api/v1/subscription/invite [JWT - DOMINION only]
  Body: { "account_id": "tyrax-id-of-invitee" }

DELETE /api/v1/subscription/invite/:accountID [JWT]

POST /api/v1/subscription/invite/accept [JWT]
  Body: { "invite_id": "..." }

POST /api/v1/subscription/invite/leave [JWT]

--- ENV VARS TO ADD to .env.example ---
FREEKASSA_SHOP_ID=
FREEKASSA_API_KEY=
FREEKASSA_SECRET_WORD_2=
CRYPTO_PAY_TOKEN=

Run: go build ./... to verify.
```

---

## ЭТАП 4 — Android: Тема + Навигация + Онбординг
### Модель: `claude-sonnet-4`

```
Read TYRAX_CONTEXT.md and .cursorrules before writing any code.

You are building the Android app for TYRAX VPN.
Package: com.tyrax
Language: Kotlin
UI: Jetpack Compose
Min SDK: 26
Project location: tyrax-android/

TASK: Set up theme, navigation, and onboarding screens.

--- STEP 1: Dependencies ---
Add to app/build.gradle.kts:
```kotlin
dependencies {
    // Compose
    implementation(platform("androidx.compose:compose-bom:2024.05.00"))
    implementation("androidx.compose.ui:ui")
    implementation("androidx.compose.material3:material3")
    implementation("androidx.compose.ui:ui-tooling-preview")
    implementation("androidx.activity:activity-compose:1.9.0")

    // Navigation
    implementation("androidx.navigation:navigation-compose:2.7.7")

    // Hilt
    implementation("com.google.dagger:hilt-android:2.51")
    kapt("com.google.dagger:hilt-android-compiler:2.51")
    implementation("androidx.hilt:hilt-navigation-compose:1.2.0")

    // DataStore
    implementation("androidx.datastore:datastore-preferences:1.1.1")

    // Retrofit
    implementation("com.squareup.retrofit2:retrofit:2.11.0")
    implementation("com.squareup.retrofit2:converter-gson:2.11.0")
    implementation("com.squareup.okhttp3:logging-interceptor:4.12.0")

    // Coroutines
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.8.0")
    implementation("androidx.lifecycle:lifecycle-viewmodel-compose:2.8.1")
    implementation("androidx.lifecycle:lifecycle-runtime-compose:2.8.1")
}
```

--- STEP 2: TyraxTheme ---
Create presentation/theme/TyraxTheme.kt:

Colors:
```kotlin
object TyraxColors {
    val Black = Color(0xFF000000)
    val White = Color(0xFFFFFFFF)
    val Red = Color(0xFFFF1E1E)
    val DarkGray = Color(0xFF111111)
    val MidGray = Color(0xFF1A1A1A)
    val SubText = Color(0xFF555555)
    val Success = Color(0xFFFF1E1E) // Red = success in TYRAX world
}
```

Typography — all uppercase feel, tight letterSpacing:
- Display: FontWeight.Black, 48sp, letterSpacing=4sp
- Headline: FontWeight.Black, 20sp, letterSpacing=3sp
- Label: FontWeight.Bold, 11sp, letterSpacing=2.5sp, color=SubText
- Body: FontWeight.Normal, 14sp, letterSpacing=0.5sp
- Accent: FontWeight.Black, 13sp, letterSpacing=2sp, color=Red

MaterialTheme colorScheme: all backgrounds Black, surfaces DarkGray, primary Red.
Set status bar transparent with white icons in MainActivity.

--- STEP 3: Navigation ---
Create presentation/navigation/TyraxNavGraph.kt with routes:
```kotlin
sealed class Screen(val route: String) {
    object Splash : Screen("splash")
    object Onboarding : Screen("onboarding")
    object Login : Screen("login")
    object Register : Screen("register")
    object Main : Screen("main")
    object Nodes : Screen("nodes")
    object Subscription : Screen("subscription")
    object Devices : Screen("devices")
    object Settings : Screen("settings")
}
```

NavHost with startDestination = Splash.
Logic: Splash checks DataStore for JWT token → if exists go to Main, else go to Onboarding.

--- STEP 4: Splash Screen ---
Create presentation/screens/splash/SplashScreen.kt:

Black screen, TYRAX wordmark centered (Text with Display style).
After 1.2 seconds:
- Glitch animation: rapid alpha flicker (3 frames, 50ms each)
- Navigate based on JWT presence

No LaunchedEffect delay visible to user — instant feel.

--- STEP 5: Onboarding ---
Create presentation/screens/onboarding/OnboardingScreen.kt:

3 slides, horizontal swipe (HorizontalPager):

Slide 1:
  Top: small label "SYSTEM ALERT"
  Center: giant text "ОНИ РЕШАЮТ," then new line "ЧТО ТЕБЕ" then "МОЖНО ВИДЕТЬ"
  Each word appears with 200ms staggered fade-in

Slide 2:
  Center: "МЫ ЭТО УБИРАЕМ"
  Subtext below: "без объяснений. без разрешения."

Slide 3:
  Center huge: "БЕЗ РАЗРЕШЕНИЯ"
  Below: TyraxButton("ENTER", onClick = navigateToLogin)
  Very small text below button: "уже есть аккаунт" → navigateToLogin

Navigation indicators: 3 thin lines (not dots), active one is Red.
All text: White on Black. No images, no illustrations.

--- STEP 6: Reusable Components ---
Create presentation/components/TyraxButton.kt:
- Sharp corners (0.dp radius)
- Filled variant: Red background, White text
- Outline variant: Red border, transparent background, Red text
- Loading state: text replaced with animated "..." dots

Create presentation/components/GlitchText.kt:
- Composable that takes text and triggers brief glitch on composition
- Glitch = 3 rapid alpha flickers over 150ms, then stable

Create presentation/components/StatusBadge.kt:
- For node status: OPEN (white border), MONITORED (dim), HEAVILY RESTRICTED (red)

--- EXPECTED FILES ---
app/src/main/java/com/tyrax/
├── presentation/
│   ├── theme/TyraxTheme.kt
│   ├── navigation/TyraxNavGraph.kt
│   ├── screens/
│   │   ├── splash/SplashScreen.kt
│   │   └── onboarding/OnboardingScreen.kt
│   └── components/
│       ├── TyraxButton.kt
│       ├── GlitchText.kt
│       └── StatusBadge.kt
```

---

## ЭТАП 5 — Android: Auth экраны
### Модель: `claude-sonnet-4`

```
Read TYRAX_CONTEXT.md and .cursorrules before writing any code.

Package: com.tyrax | Kotlin + Jetpack Compose + Hilt + Retrofit
TyraxTheme, TyraxColors, TyraxTypography already exist at presentation/theme/TyraxTheme.kt
TyraxButton exists at presentation/components/TyraxButton.kt
Navigation graph exists at presentation/navigation/TyraxNavGraph.kt

TASK: Build auth screens, API client, and JWT storage.

--- STEP 1: API Service ---
Create data/remote/TyraxApiService.kt (Retrofit interface):

Base URL from BuildConfig or const: https://api.tyrax.app/api/v1/
(Use http://10.0.2.2:8080/api/v1/ for emulator local dev — add comment)

```kotlin
interface TyraxApiService {
    @POST("auth/register")
    suspend fun register(@Body req: RegisterRequest): ApiResponse<AuthData>

    @POST("auth/login")
    suspend fun login(@Body req: LoginRequest): ApiResponse<AuthData>

    @GET("auth/telegram-init")
    suspend fun telegramInit(): ApiResponse<TelegramInitData>

    @GET("nodes")
    suspend fun getNodes(): ApiResponse<List<NodeDto>>

    @GET("vpn/config")
    suspend fun getVpnConfig(@Query("device_public_key") pubKey: String): ApiResponse<VpnConfigDto>

    @POST("vpn/device")
    suspend fun addDevice(@Body req: AddDeviceRequest): ApiResponse<DeviceConfigDto>

    @DELETE("vpn/device/{id}")
    suspend fun deleteDevice(@Path("id") deviceId: String): ApiResponse<Unit>

    @GET("subscription")
    suspend fun getSubscription(): ApiResponse<SubscriptionDto>

    @POST("payment/create")
    suspend fun createPayment(@Body req: CreatePaymentRequest): ApiResponse<PaymentResultDto>

    @GET("payment/status/{orderId}")
    suspend fun getPaymentStatus(@Path("orderId") orderId: String): ApiResponse<PaymentStatusDto>
}
```

Create data/remote/ApiResponse.kt:
```kotlin
data class ApiResponse<T>(
    val status: String,
    val data: T? = null,
    val message: String? = null
)
```

Create data/remote/AuthInterceptor.kt:
- OkHttp interceptor that adds "Authorization: Bearer TOKEN" header
- Token read from DataStore synchronously (runBlocking — acceptable for interceptor)
- Skip auth header for /auth/register and /auth/login endpoints

--- STEP 2: DataStore ---
Create data/local/TokenStore.kt:
```kotlin
class TokenStore(private val dataStore: DataStore<Preferences>) {
    val token: Flow<String?> // reads "jwt_token" key
    suspend fun saveToken(token: String)
    suspend fun clearToken()
    val isLoggedIn: Flow<Boolean>
}
```

--- STEP 3: Auth Repository ---
Create data/repository/AuthRepositoryImpl.kt implementing domain/repository/AuthRepository.kt:

```kotlin
interface AuthRepository {
    suspend fun login(email: String, password: String): Result<AuthData>
    suspend fun register(email: String, password: String): Result<AuthData>
    suspend fun getTelegramInitUrl(): Result<String>
    suspend fun saveToken(token: String)
    suspend fun logout()
    val isLoggedIn: Flow<Boolean>
}
```

--- STEP 4: Use Cases ---
Create domain/usecase/LoginUseCase.kt
Create domain/usecase/RegisterUseCase.kt
Each wraps repository call, returns Result<AuthData>

--- STEP 5: AuthViewModel ---
Create presentation/screens/auth/AuthViewModel.kt:

```kotlin
sealed class AuthUiState {
    object Idle : AuthUiState()
    object Loading : AuthUiState()
    data class Success(val token: String) : AuthUiState()
    data class Error(val message: String) : AuthUiState()
}

sealed class AuthUiEvent {
    object NavigateToMain : AuthUiEvent()
}
```

Methods: login(email, password), register(email, password)
On success: save token → emit NavigateToMain event

--- STEP 6: Login Screen ---
Create presentation/screens/auth/LoginScreen.kt:

Layout (Black background, full screen):
- Top: small label "IDENTITY VERIFICATION"
- Input field "IDENTITY" (email) — custom dark style, no rounded corners, bottom border only (Red when focused)
- Input field "ACCESS CODE" (password) — same style, obscured
- TyraxButton("ENTER", filled=true) — full width
- Below button: thin divider with "or"
- TyraxButton("ENTER VIA TELEGRAM", filled=false)
- Bottom: "нет аккаунта? REGISTER" → navigateToRegister

Telegram auth flow:
1. User taps "ENTER VIA TELEGRAM"
2. ViewModel calls getTelegramInitUrl() → gets URL like t.me/tyraxvpnbot?start=TOKEN
3. Open URL via Intent (opens Telegram app)
4. App goes to background
5. When user returns → ViewModel polls /auth/status for 30 seconds (every 2s) looking for JWT
6. On JWT received → save and navigate to Main

Error messages must use TYRAX vocabulary:
- Wrong password → "INVALID CREDENTIALS"
- Network error → "CONNECTION FAILED. RETRY."
- Server error → "SYSTEM ERROR. NODE OFFLINE."

--- STEP 7: Register Screen ---
Create presentation/screens/auth/RegisterScreen.kt:

Same style as Login but with:
- Email field
- Password field
- Confirm password field (validate match client-side)
- TyraxButton("REGISTER")
- "уже есть аккаунт? ENTER" → navigateToLogin

--- STEP 8: Hilt Module ---
Create di/NetworkModule.kt providing:
- OkHttpClient (with AuthInterceptor + logging)
- Retrofit
- TyraxApiService
- TokenStore (singleton)
- AuthRepository

--- EXPECTED FILES ---
data/remote/TyraxApiService.kt
data/remote/ApiResponse.kt
data/remote/AuthInterceptor.kt
data/local/TokenStore.kt
data/repository/AuthRepositoryImpl.kt
domain/repository/AuthRepository.kt
domain/usecase/LoginUseCase.kt
domain/usecase/RegisterUseCase.kt
presentation/screens/auth/AuthViewModel.kt
presentation/screens/auth/LoginScreen.kt
presentation/screens/auth/RegisterScreen.kt
di/NetworkModule.kt
```

---

## ЭТАП 6 — Android: VPN ядро
### Модель: `claude-opus-4` (системный уровень, максимальный риск багов)

```
Read TYRAX_CONTEXT.md and .cursorrules before writing any code.

Package: com.tyrax | Kotlin | Min SDK 26
API service exists at: data/remote/TyraxApiService.kt
Token store at: data/local/TokenStore.kt

TASK: Build the VPN engine — VpnService, WireGuard integration, auto-reconnect, network handoff.

--- STEP 1: WireGuard dependency ---
Add to app/build.gradle.kts:
```kotlin
implementation("com.wireguard.android:tunnel:1.0.20230706")
```

Add to AndroidManifest.xml:
```xml
<uses-permission android:name="android.permission.INTERNET"/>
<uses-permission android:name="android.permission.FOREGROUND_SERVICE"/>
<uses-permission android:name="android.permission.FOREGROUND_SERVICE_SPECIAL_USE"/>
<uses-permission android:name="android.permission.CHANGE_NETWORK_STATE"/>
<uses-permission android:name="android.permission.ACCESS_NETWORK_STATE"/>

<service
    android:name=".data.vpn.TyraxVpnService"
    android:exported="false"
    android:foregroundServiceType="specialUse"
    android:permission="android.permission.BIND_VPN_SERVICE">
    <intent-filter>
        <action android:name="android.net.VpnService"/>
    </intent-filter>
</service>
```

--- STEP 2: VPN State ---
Create domain/model/VpnState.kt:
```kotlin
sealed class VpnState {
    object Disconnected : VpnState()
    object Connecting : VpnState()
    data class Connected(
        val nodeCodename: String,
        val protocol: String,
        val pingMs: Int,
        val bytesIn: Long = 0,
        val bytesOut: Long = 0,
    ) : VpnState()
    data class Error(val message: String) : VpnState()
    object Reconnecting : VpnState()
}
```

Create domain/model/Node.kt, Device.kt matching API DTOs.

--- STEP 3: TyraxVpnService ---
Create data/vpn/TyraxVpnService.kt extending android.net.VpnService:

State management:
- companion object with StateFlow<VpnState> (shared across app)
- companion object with method connect(context, wireGuardConf: String, nodeCodename: String)
- companion object with method disconnect(context)

connect() flow:
1. Set state = Connecting
2. Parse WireGuard config string into WireGuard tunnel config
3. Build VPN interface via Builder:
   - addAddress(clientIP, 32)
   - addRoute("0.0.0.0", 0) — all traffic
   - addDnsServer("1.1.1.1")
   - addDnsServer("8.8.8.8")
   - setMtu(1420)
   - setSession("TYRAX")
   - Apply split tunnel: EXCLUDE RU domains (addDisallowedApplication or route exclusions)
4. establish() → get ParcelFileDescriptor
5. Start WireGuard tunnel via wireguard-android library
6. Set state = Connected(nodeCodename, "wireguard", ping)
7. Start monitoring coroutine

Split tunnel RU domains — apply as excluded routes (bypass VPN):
Hardcode the same domain list from /api/v1/vpn/split-domains endpoint.
Also try to fetch fresh list from API on connect, update if successful.

Foreground notification:
- Show persistent notification: "TYRAX — ACCESS GRANTED · NL-01"
- Notification channel: "TYRAX_VPN"
- Tap opens app

--- STEP 4: Auto-reconnect monitor ---
Inside TyraxVpnService, start a coroutine that runs while connected:

```kotlin
private fun startMonitor() {
    monitorJob = serviceScope.launch {
        while (isActive) {
            delay(30_000) // check every 30s
            val ping = measurePing(currentNodeHost)
            if (ping > 2000 || ping == -1L) {
                // Node degraded — try to switch
                _state.value = VpnState.Reconnecting
                val betterNode = fetchBestNode()
                if (betterNode != null) {
                    reconnectToNode(betterNode)
                } else {
                    // All nodes failed — wait and retry
                    delay(10_000)
                    reconnectToNode(currentNode)
                }
            }
        }
    }
}

private suspend fun measurePing(host: String): Long {
    // ICMP ping via InetAddress.getByName(host).isReachable(timeout)
    // Return RTT in ms, or -1 if unreachable
}
```

--- STEP 5: NetworkCallback (Wi-Fi ↔ LTE handoff) ---
Register ConnectivityManager.NetworkCallback in TyraxVpnService.onStartCommand:

```kotlin
private val networkCallback = object : ConnectivityManager.NetworkCallback() {
    override fun onAvailable(network: Network) {
        // Network changed — if VPN was connected, reconnect to same node
        if (_state.value is VpnState.Connected || _state.value is VpnState.Reconnecting) {
            serviceScope.launch {
                delay(500) // brief wait for network to stabilize
                reconnectToNode(currentNode)
            }
        }
    }

    override fun onLost(network: Network) {
        if (_state.value is VpnState.Connected) {
            _state.value = VpnState.Reconnecting
        }
    }
}
```

Register with: connectivityManager.registerDefaultNetworkCallback(networkCallback)
Unregister in onDestroy.

--- STEP 6: VPN Repository and Use Cases ---
Create domain/repository/VpnRepository.kt:
```kotlin
interface VpnRepository {
    suspend fun getNodes(): Result<List<Node>>
    suspend fun getBestNode(): Result<Node>
    suspend fun getDeviceConfig(devicePublicKey: String): Result<VpnConfig>
    suspend fun addDevice(name: String): Result<DeviceConfig>
    suspend fun getSplitDomains(): Result<List<String>>
    val vpnState: StateFlow<VpnState>
}
```

Create domain/usecase/ConnectToNodeUseCase.kt:
- Gets device config from repository
- Calls TyraxVpnService.connect(context, wireGuardConf, nodeCodename)

Create domain/usecase/DisconnectUseCase.kt:
- Calls TyraxVpnService.disconnect(context)

Create domain/usecase/GetBestNodeUseCase.kt:
- Calls repository.getBestNode()

--- EXPECTED FILES ---
data/vpn/TyraxVpnService.kt
domain/model/VpnState.kt
domain/model/Node.kt
domain/model/Device.kt
domain/repository/VpnRepository.kt
data/repository/VpnRepositoryImpl.kt
domain/usecase/ConnectToNodeUseCase.kt
domain/usecase/DisconnectUseCase.kt
domain/usecase/GetBestNodeUseCase.kt
di/VpnModule.kt
```

---

## ЭТАП 7 — Android: Главный экран + Ноды
### Модель: `claude-sonnet-4`

```
Read TYRAX_CONTEXT.md and .cursorrules before writing any code.

Package: com.tyrax | Kotlin + Compose
VpnState exists at: domain/model/VpnState.kt
TyraxVpnService exists at: data/vpn/TyraxVpnService.kt
ConnectToNodeUseCase, DisconnectUseCase exist in domain/usecase/
TyraxTheme, TyraxColors, TyraxTypography, TyraxButton exist in presentation/

TASK: Build the main screen and nodes screen.

--- STEP 1: MainViewModel ---
Create presentation/screens/main/MainViewModel.kt:

UiState:
```kotlin
data class MainUiState(
    val vpnState: VpnState = VpnState.Disconnected,
    val currentNode: String = "",
    val pingMs: Int = 0,
    val trafficIn: Long = 0,
    val trafficOut: Long = 0,
)
```

- Collect TyraxVpnService.state as StateFlow → update uiState
- connect(): launch ConnectToNodeUseCase
- disconnect(): launch DisconnectUseCase

--- STEP 2: MainScreen ---
Create presentation/screens/main/MainScreen.kt:

Layout — full black screen, 3 zones:

TOP ZONE (status):
```
STATUS
[ACCESS GRANTED / OUTSIDE SYSTEM / BREACHING NETWORK… / RECONNECTING…]
```
- "STATUS" in Label style (gray, uppercase, letter-spaced)
- Status text: Display style
  - DISCONNECTED → "OUTSIDE SYSTEM" in SubText color
  - CONNECTING → "BREACHING NETWORK…" in White, animated ellipsis
  - CONNECTED → "ACCESS GRANTED" in Red
  - RECONNECTING → "RECONNECTING…" in White, pulsing

CENTER ZONE (main button):
Square button 200×200dp, centered:
- DISCONNECTED: White border 1.5dp, text "ENTER" in White Display style
- CONNECTING: Red border pulsing (alpha animation 0.4→1.0, 600ms loop), text "BREACHING…"
- CONNECTED: Red border solid, text "DISCONNECT" in Red

When connecting → show glitch animation overlay (rapid alpha flicker on status text).

BOTTOM ZONE (info + nav):
- When connected: "NODE: NL-01 · OPEN" then "12ms" in Red accent style
- When disconnected: "NODE: —"
- Horizontal row: "NODES" | "CONTROL" (tappable labels in SubText color)
- "NODES" → navigateToNodes
- "CONTROL" → navigateToSubscription

NO hamburger menu, NO bottom bar, NO top app bar.

Connection animation sequence (when ENTERING):
1. Status flickers to "BREACHING NETWORK…"
2. Button border turns Red and pulses
3. After connect → status glitches to "NODE ACQUIRED"
4. 500ms later → glitches to "ACCESS GRANTED"

--- STEP 3: NodesScreen ---
Create presentation/screens/nodes/NodesScreen.kt + NodesViewModel.kt:

NodesViewModel:
- Load nodes from GetNodesUseCase on init
- UiState: loading, nodes list, error

NodesScreen layout:
- Full black, no top bar (just back arrow in top-left corner as Text "<")
- Title: "NODES" in Headline style
- LazyColumn of NodeCard composables

NodeCard:
```
NL-01                    OPEN
Netherlands              12ms
```
- Codename in Headline style (White)
- Country in Label style (SubText)
- Status badge (right side): OPEN = white text, MONITORED = dim, HEAVILY RESTRICTED = red
- Ping in Accent style (Red)
- Sharp divider between cards (0.5dp, MidGray)
- No selection — tap does nothing (nodes are auto-selected by system)

Empty state: "NO NODES AVAILABLE" centered in SubText style.

--- EXPECTED FILES ---
presentation/screens/main/MainScreen.kt
presentation/screens/main/MainViewModel.kt
presentation/screens/nodes/NodesScreen.kt
presentation/screens/nodes/NodesViewModel.kt
domain/usecase/GetNodesUseCase.kt
```

---

## ЭТАП 8 — Android: Подписки + Устройства + Настройки
### Модель: `gpt-4.1` (UI-фокус, дешевле)

```
Read TYRAX_CONTEXT.md, .cursorrules, and PAYMENTS.md before writing any code.

Package: com.tyrax | Kotlin + Compose
TyraxTheme, TyraxColors, TyraxButton exist in presentation/
TyraxApiService exists at data/remote/TyraxApiService.kt
Tiers: FREE (0₽, 1 device, 3GB), CORE (199₽, 2 dev), SHADOW (349₽, 5 dev), DOMINION (649₽, 10 dev + invite 3)
Discounts: 3mo=10%, 6mo=15%, 12mo=20%

TASK: Build subscription, devices, and settings screens.

--- STEP 1: SubscriptionScreen ---
Create presentation/screens/subscription/SubscriptionScreen.kt + SubscriptionViewModel.kt:

SubscriptionViewModel:
- Load current subscription from API
- createPayment(tier, method, months): call API → get payment_url → open in browser/WebView
- pollPaymentStatus(orderId): poll every 3s for 5min, stop when PAID
- On PAID → show "ACCESS UNLOCKED" animation → pop back to main

SubscriptionScreen layout:
Full black screen, scroll.

Header:
- "CONTROL" title in Headline
- Current tier badge: "CURRENT: SHADOW" in Red accent

Period selector (horizontal chips):
[1 МЕС] [3 МЕС -10%] [6 МЕС -15%] [12 МЕС -20%]
Active chip: Red background. Inactive: Red border.

Three tier cards (CORE, SHADOW, DOMINION):
Each card:
- Tier name in big Headline (Red if current, White otherwise)
- Price line: "349 ₽ / МЕС" (or discounted price if period selected)
- 3-4 feature lines in Body style:
  CORE: "2 УСТРОЙСТВА · БЕЗЛИМИТ · ВСЕ НОДЫ"
  SHADOW: "5 УСТРОЙСТВ · ПРИОРИТЕТ · ВСЕ НОДЫ"
  DOMINION: "10 УСТРОЙСТВ · ПРИОРИТЕТ #1 · ПРИГЛАСИТЬ 3 АККАУНТА · ПОДДЕРЖКА"
- TyraxButton("UNLOCK", filled=true) — disabled if already this tier
- Sharp border around card, 1dp White (Red if current)

Payment method selector (below cards):
[СБП] [КАРТА РФ] [КРИПТА]
After selecting tier + period + method → button "ПЕРЕЙТИ К ОПЛАТЕ"
Opens payment_url in CustomTabsIntent (browser).

--- STEP 2: DevicesScreen ---
Create presentation/screens/devices/DevicesScreen.kt + DevicesViewModel.kt:

Header: "MY DEVICES" | count badge "2/5"

List of devices:
Each row:
- Device name (e.g. "My Phone") in Body White
- Added date in Label SubText
- Red "×" button to delete (confirm dialog: "REMOVE DEVICE?" [CONFIRM] [CANCEL])

Bottom: TyraxButton("ADD THIS DEVICE", filled=false)
- Calls addDevice(name="Device N") API
- Saves returned WireGuard config to DataStore
- Shows "DEVICE ADDED" in status

For DOMINION: separate section "INVITED ACCOUNTS"
- List invited accounts with remove button
- TyraxButton("INVITE ACCOUNT", outline) → dialog: enter account ID → send invite

--- STEP 3: SettingsScreen ---
Create presentation/screens/settings/SettingsScreen.kt:

Minimal. No cluttering.

Rows (thin separator between each):
1. "IDENTITY" — shows email/telegram handle in SubText
2. "SUBSCRIPTION" — shows tier name → navigateToSubscription
3. "DEVICES" — shows count → navigateToDevices
4. "EXIT SYSTEM" — logout confirmation → clears token → navigate to Onboarding

Each row: full width tap target, Label on left, SubText value on right, no icons.

--- STEP 4: Traffic counter for FREE tier ---
In MainScreen: if tier == FREE, show below node info:
"TRAFFIC: 1.2 GB / 3 GB"
Thin progress line (Red fill, MidGray track) showing usage.

--- EXPECTED FILES ---
presentation/screens/subscription/SubscriptionScreen.kt
presentation/screens/subscription/SubscriptionViewModel.kt
presentation/screens/devices/DevicesScreen.kt
presentation/screens/devices/DevicesViewModel.kt
presentation/screens/settings/SettingsScreen.kt
domain/usecase/GetSubscriptionUseCase.kt
domain/usecase/CreatePaymentUseCase.kt
domain/usecase/AddDeviceUseCase.kt
domain/usecase/DeleteDeviceUseCase.kt
domain/usecase/InviteAccountUseCase.kt
```

---

## ЭТАП 9 — Финальная полировка Android + Backend
### Модель: `claude-sonnet-4`

```
Read TYRAX_CONTEXT.md and .cursorrules before writing any code.

TASK: Final polish — error handling, ProGuard, logging, strings cleanup.

--- Android ---

1. Error handling: all network errors must show TYRAX-branded messages.
   Create presentation/utils/ErrorMapper.kt:
   - 401 → "ACCESS DENIED. RE-ENTER."
   - 403 → "LEVEL INSUFFICIENT."
   - 503 → "NODE UNAVAILABLE. SWITCHING..."
   - No network → "CONNECTION LOST."
   - Timeout → "SIGNAL LOST. RETRYING..."

2. Verify ALL user-facing strings are in res/values/strings.xml (no hardcoded Russian/English in Compose files).

3. ProGuard — update proguard-rules.pro:
   - Keep WireGuard classes
   - Keep Retrofit interfaces
   - Keep model data classes (Gson serialization)
   - Keep Hilt generated classes

4. Add -keepattributes to avoid stripping annotations.

5. AndroidManifest: verify INTERNET, FOREGROUND_SERVICE, VPN_SERVICE permissions.

6. Set versionCode=1, versionName="1.0.0" in build.gradle.kts.

--- Backend ---

1. Add slog structured logging to all handlers:
   - Log request method, path, user_id, status_code, duration
   - Log webhook events with amounts and order IDs

2. Add .env.example file with all required vars and comments.

3. Add basic rate limiting middleware (Fiber built-in):
   - /auth/* endpoints: max 10 req/min per IP
   - /webhooks/*: unlimited (external services)
   - Everything else: 100 req/min per user

4. Dockerfile for backend:
   FROM golang:1.22-alpine AS builder
   ... multi-stage build ...
   Final image: scratch or alpine, ~20MB

5. Update docker-compose.yml to include health checks.

--- EXPECTED CHANGES ---
- proguard-rules.pro (updated)
- res/values/strings.xml (complete, no missing strings)
- presentation/utils/ErrorMapper.kt (new)
- tyrax-backend/internal/middleware/ratelimit.go (new)
- tyrax-backend/Dockerfile (new)
- tyrax-backend/docker-compose.yml (updated)
- tyrax-backend/.env.example (new)
```

---

## ЭТАП 10 — Деплой серверов (руками, не Cursor)
### Модель: не нужна (терминал)

```bash
# --- На своём компьютере ---

# 1. Купи 4 сервера на aeza.net
# Локации: NL (backend), NL (нода), DE (нода), FI (нода)
# Тариф: Shared NLs-2 / DEs-2 / HELs-2 (2vCPU, 4GB, 60GB NVMe)
# Оплата: карта Мир или СБП

# 2. Для каждого сервера получишь IP и root пароль по email

# --- На сервере BACKEND (NL) ---
ssh root@<BACKEND_IP>

apt update && apt install -y docker.io docker-compose-v2 nginx certbot python3-certbot-nginx git

# Клонируй репо
git clone https://github.com/ТЫ/tyrax-backend /opt/tyrax-backend
cd /opt/tyrax-backend

# Создай .env
cp .env.example .env
nano .env  # заполни все переменные

# Запусти
docker compose up -d

# SSL
certbot --nginx -d api.tyrax.app  # замени на свой домен

# --- На каждой VPN ноде (NL/DE/FI) ---
ssh root@<NODE_IP>

apt update && apt install -y wireguard

# Установи Marzban (управление VLESS/Xray)
sudo bash -c "$(curl -sL https://github.com/Gozargah/Marzban/raw/master/script.sh)"
marzban install

# WireGuard ключи
wg genkey | tee /etc/wireguard/server_private.key | wg pubkey > /etc/wireguard/server_public.key
cat /etc/wireguard/server_public.key  # → скопируй в backend DB как public_key для этой ноды

# WireGuard конфиг /etc/wireguard/wg0.conf
[Interface]
PrivateKey = <server_private_key>
Address = 10.0.0.1/24
ListenPort = 51820
PostUp = iptables -A FORWARD -i wg0 -j ACCEPT; iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
PostDown = iptables -D FORWARD -i wg0 -j ACCEPT; iptables -t nat -D POSTROUTING -o eth0 -j MASQUERADE

systemctl enable wg-quick@wg0
systemctl start wg-quick@wg0

# Добавь ноду в backend DB
# INSERT INTO nodes (codename, country, host, port, protocol, status, public_key)
# VALUES ('NL-01', 'Netherlands', '1.2.3.4', 51820, 'wireguard', 'OPEN', '<public_key>');
```

---

## ЭТАП 11 — Сборка MVP APK
### Модель: не нужна (Android Studio)

```
В Android Studio:

1. Смени BASE_URL в data/remote/TyraxApiService.kt:
   С: http://10.0.2.2:8080/api/v1/   (эмулятор)
   На: https://api.tyrax.app/api/v1/  (прод)

2. Создай keystore для подписи:
   Build → Generate Signed Bundle/APK → APK
   Create new keystore:
     Path: ~/tyrax-release.jks
     Password: придумай и запомни
     Alias: tyrax
   Заполни данные

3. Выбери:
   Build Variant: release
   Signature Versions: V1 + V2

4. Нажми Finish

5. APK будет: app/release/app-release.apk

Размер нормального APK: 15-35 MB
Если больше 50MB — что-то не так с ProGuard

6. Установи на телефон:
   adb install app/release/app-release.apk
   Или скопируй на телефон и открой файл

7. Проверь:
   ✓ Регистрация работает
   ✓ VPN подключается
   ✓ Смена Wi-Fi → LTE не рвёт соединение
   ✓ Яндекс открывается при включённом VPN
   ✓ Платёжная ссылка открывается
```

---

## ЧЕКЛИСТ ПЕРЕД ПЕРВЫМ ПОЛЬЗОВАТЕЛЕМ

- [ ] Backend задеплоен, API отвечает на https://api.tyrax.app/api/v1/nodes
- [ ] Все 3 VPN ноды добавлены в БД со статусом OPEN
- [ ] WireGuard работает на нодах (проверь вручную с тестовым конфигом)
- [ ] FreeKassa: shop зарегистрирован, i=44 и i=36 включены, webhook URL указан
- [ ] CryptoPay: приложение создано в @CryptoBot, webhook включён
- [ ] APK установлен и протестирован на реальном телефоне
- [ ] Telegram бот создан через @BotFather, токен вписан в .env
- [ ] Домен привязан к backend серверу, SSL работает
