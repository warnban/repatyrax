# TYRAX — PAYMENTS INTEGRATION

> Этот документ описывает полную интеграцию платёжных систем для TYRAX.
> Cursor должен использовать его как единственный источник истины при работе с платежами.

---

## ОБЗОР ПЛАТЁЖНЫХ СИСТЕМ

| Система | Метод | Валюта | ID метода |
|---|---|---|---|
| **FreeKassa** | СБП (API) | RUB | `i=44` |
| **FreeKassa** | Карты РФ (API) | RUB | `i=36` |
| **Crypto Pay** | Крипта (через @CryptoBot) | BTC, ETH, USDT, TON, LTC и др. | — |

> ⚠️ FreeKassa интегрируется **только через API** (не SCI/форму).
> Параметр `i=44` — СБП API, `i=36` — Карты РФ API.

---

## 1. FREEKASSA API

### Общая информация

| Параметр | Значение |
|---|---|
| Base URL | `https://api.fk.life/v1/` |
| Формат | JSON |
| Метод запросов | POST |
| Аутентификация | `signature` в каждом запросе |

### Аутентификация — генерация подписи

Каждый запрос к API должен содержать поле `signature`.

**Алгоритм:**
1. Взять все параметры запроса (кроме самой `signature`)
2. Отсортировать ключи по алфавиту (`ksort`)
3. Соединить значения через `|`
4. Посчитать `HMAC-SHA256` от полученной строки с API-ключом

**Go реализация:**
```go
func generateSignature(params map[string]interface{}, apiKey string) string {
    // 1. Собрать ключи и отсортировать
    keys := make([]string, 0, len(params))
    for k := range params {
        keys = append(keys, k)
    }
    sort.Strings(keys)

    // 2. Соединить значения через |
    values := make([]string, 0, len(keys))
    for _, k := range keys {
        values = append(values, fmt.Sprintf("%v", params[k]))
    }
    message := strings.Join(values, "|")

    // 3. HMAC-SHA256
    mac := hmac.New(sha256.New, []byte(apiKey))
    mac.Write([]byte(message))
    return hex.EncodeToString(mac.Sum(nil))
}
```

**Обязательные поля в каждом запросе:**
```json
{
  "shopId": 12345,
  "nonce": 1719216000,
  "signature": "abc123..."
}
```
> `nonce` — уникальный ID запроса, должен **всегда быть больше предыдущего**. Используй `time.Now().UnixMilli()`.

---

### Создание заказа (платёжная ссылка)

**Endpoint:** `POST https://api.fk.life/v1/orders/create`

**Параметры:**

| Параметр | Обязательный | Тип | Описание |
|---|---|---|---|
| `shopId` | ✅ | integer | ID магазина |
| `nonce` | ✅ | integer | Unix timestamp в мс |
| `signature` | ✅ | string | HMAC-SHA256 подпись |
| `i` | ✅ | integer | **`44` — СБП, `36` — Карты РФ** |
| `email` | ✅ | string | Email покупателя |
| `ip` | ✅ | string | IP покупателя |
| `amount` | ✅ | numeric | Сумма в рублях |
| `currency` | ✅ | string | `"RUB"` |
| `paymentId` | ❌ | string | Твой внутренний ID заказа (рекомендуется) |
| `tel` | ❌ | string | Телефон (может требоваться для СБП) |
| `notification_url` | ❌ | string | Переопределить URL для вебхука (по запросу в поддержку) |
| `recurrent` | ❌ | string | `"Y"` для рекуррентных платежей |
| `recurrent_period` | ❌ | string | `"month"` / `"week"` / `"year"` / `"day"` |
| `recurrent_description` | ❌ | string | Описание, 10–200 символов |

**Go пример запроса:**
```go
func (f *FreeKassaClient) CreateOrder(ctx context.Context, req CreateOrderRequest) (*CreateOrderResponse, error) {
    params := map[string]interface{}{
        "shopId":    f.ShopID,
        "nonce":     time.Now().UnixMilli(),
        "i":         req.PaymentMethodID, // 44 = СБП, 36 = Карты РФ
        "email":     req.Email,
        "ip":        req.IP,
        "amount":    req.Amount,
        "currency":  "RUB",
        "paymentId": req.InternalOrderID,
    }
    params["signature"] = generateSignature(params, f.APIKey)

    // POST https://api.fk.life/v1/orders/create
    // body: JSON(params)
}
```

**Ответ:**
```json
{
  "type": "success",
  "orderId": 123456,
  "orderHash": "bd4161db429848651499aabcb1d89330",
  "location": "https://pay.freekassa.net/form/123456/bd4161db429848651499aabcb1d89330"
}
```
> Поле `location` — ссылка для оплаты. Отправь её пользователю.

---

### Вебхук (уведомление об оплате)

FreeKassa делает `GET` или `POST` запрос на твой `notification_url` после успешной оплаты.

**Параметры вебхука:**

| Параметр | Описание |
|---|---|
| `MERCHANT_ID` | ID магазина |
| `AMOUNT` | Сумма платежа |
| `intid` | Номер операции FreeKassa |
| `MERCHANT_ORDER_ID` | Твой внутренний ID заказа |
| `P_EMAIL` | Email покупателя |
| `CUR_ID` | ID валюты/метода оплаты (`44` или `36`) |
| `SIGN` | Подпись для верификации |
| `payer_account` | Номер карты/телефона плательщика |

**Верификация подписи вебхука:**
```go
// Секретное слово 2 (отличается от API-ключа!)
func verifyWebhook(merchantID, amount, secret2, orderID, receivedSign string) bool {
    expected := md5(merchantID + ":" + amount + ":" + secret2 + ":" + orderID)
    return expected == receivedSign
}
```
> Используется `md5`, не SHA256. Секрет — **Секретное слово 2** из настроек магазина.

**Обязательный ответ:** если платёж принят — вернуть строку `YES`.

**Доверенные IP для вебхуков:**
```
168.119.157.136
168.119.60.227
178.154.197.79
51.250.54.238
```

---

### Статусы заказов

| Статус | Значение |
|---|---|
| `0` | Новый |
| `1` | Оплачен ✅ |
| `6` | Возврат |
| `8` | Ошибка |
| `9` | Отмена |

---

### Получение списка заказов

**Endpoint:** `POST https://api.fk.life/v1/orders`

Параметры: `shopId`, `nonce`, `signature` + фильтры `orderId`, `paymentId`, `orderStatus`, `dateFrom`, `dateTo`, `page`.

---

### Рекуррентные платежи (подписки)

Для автоматического списания за подписку добавь при создании заказа:
```json
{
  "recurrent": "Y",
  "recurrent_period": "month",
  "recurrent_description": "Подписка TYRAX SHADOW"
}
```
В ответе придёт:
```json
{
  "recurrent_order": {
    "id": 777,
    "pay_date_at": "2024-02-01 12:00:00"
  }
}
```
При повторном списании передай `recurrent_order_id: 777` вместо полного набора параметров.

---

### Конфигурация (env vars)

```env
FREEKASSA_SHOP_ID=<твой ID магазина>
FREEKASSA_API_KEY=<API ключ из личного кабинета>
FREEKASSA_SECRET_WORD_2=<Секретное слово 2 из настроек>
FREEKASSA_NOTIFICATION_URL=https://api.tyrax.app/webhooks/freekassa
```

---

## 2. CRYPTO PAY (CryptoBot)

### Общая информация

| Параметр | Значение |
|---|---|
| Base URL (mainnet) | `https://pay.crypt.bot/api/` |
| Base URL (testnet) | `https://testnet-pay.crypt.bot/api/` |
| Формат | JSON |
| Аутентификация | Header: `Crypto-Pay-API-Token: <TOKEN>` |
| Бот (mainnet) | [@CryptoBot](https://t.me/CryptoBot) |
| Бот (testnet) | [@CryptoTestnetBot](https://t.me/CryptoTestnetBot) |

### Получение API токена

1. Открыть [@CryptoBot](https://t.me/CryptoBot)
2. Перейти в **Crypto Pay → Create App**
3. Скопировать API Token
4. Вставить в env: `CRYPTO_PAY_TOKEN=<token>`

> Cursor: при генерации кода оставь placeholder `CRYPTO_PAY_TOKEN` и добавь комментарий с инструкцией выше.

---

### Поддерживаемые активы

`BTC`, `ETH`, `USDT`, `TON`, `LTC`, `BNB`, `TRX`, `USDC`

---

### Создание инвойса

**Endpoint:** `POST https://pay.crypt.bot/api/createInvoice`

**Headers:**
```
Crypto-Pay-API-Token: YOUR_TOKEN
Content-Type: application/json
```

**Параметры:**

| Параметр | Обязательный | Тип | Описание |
|---|---|---|---|
| `currency_type` | ❌ | string | `"crypto"` (default) или `"fiat"` |
| `asset` | ✅ если crypto | string | `"USDT"`, `"BTC"`, `"TON"` и т.д. |
| `fiat` | ✅ если fiat | string | `"RUB"`, `"USD"` и т.д. |
| `accepted_assets` | ❌ | string | Через запятую: `"USDT,TON,BTC"` |
| `amount` | ✅ | string | Сумма (float как строка: `"9.99"`) |
| `description` | ❌ | string | До 1024 символов |
| `payload` | ❌ | string | Твои данные (user_id, order_id) — до 4KB |
| `paid_btn_name` | ❌ | string | `"openBot"` / `"openChannel"` / `"callback"` / `"viewItem"` |
| `paid_btn_url` | ❌ | string | URL кнопки после оплаты |
| `allow_anonymous` | ❌ | boolean | Анонимная оплата |
| `expires_in` | ❌ | integer | Время жизни в секундах (1–2678400) |

**Go пример:**
```go
func (c *CryptoPayClient) CreateInvoice(ctx context.Context, req CryptoInvoiceRequest) (*Invoice, error) {
    body := map[string]interface{}{
        "currency_type":   "fiat",           // выставляем в рублях
        "fiat":            "RUB",
        "accepted_assets": "USDT,TON,BTC",   // пользователь выбирает сам
        "amount":          fmt.Sprintf("%.2f", req.AmountRUB),
        "description":     "TYRAX " + req.TierName,
        "payload":         req.UserID + "|" + req.OrderID,
        "paid_btn_name":   "openBot",
        "paid_btn_url":    "https://t.me/tyraxvpnbot",
        "expires_in":      3600, // 1 час
    }

    // POST https://pay.crypt.bot/api/createInvoice
    // Header: Crypto-Pay-API-Token: <TOKEN>
}
```

**Ответ:**
```json
{
  "ok": true,
  "result": {
    "invoice_id": 1234567,
    "status": "active",
    "hash": "abc123",
    "asset": "USDT",
    "amount": "9.99",
    "pay_url": "https://t.me/CryptoBot?start=IVxxxxxx",
    "bot_invoice_url": "https://t.me/CryptoBot?start=IVxxxxxx",
    "mini_app_invoice_url": "https://t.me/CryptoBot/app?startapp=IVxxxxxx",
    "web_app_invoice_url": "https://pay.crypt.bot/invoices/IVxxxxxx",
    "created_at": "2024-01-15T12:00:00Z",
    "expiration_date": "2024-01-15T13:00:00Z"
  }
}
```
> Отправь пользователю `bot_invoice_url` — откроет оплату в Telegram.

---

### Получение обновлений об оплате

Два способа:

#### 1. Вебхук (рекомендуется)

Настройка: @CryptoBot → Crypto Pay → My Apps → Webhooks → Enable → указать URL.

```
POST https://api.tyrax.app/webhooks/crypto-pay
Header: crypto-pay-api-signature: <HMAC-SHA256>
```

**Верификация подписи:**
```go
func verifyCryptoPayWebhook(token, body, receivedSignature string) bool {
    secret := sha256.Sum256([]byte(token))
    mac := hmac.New(sha256.New, secret[:])
    mac.Write([]byte(body))
    expected := hex.EncodeToString(mac.Sum(nil))
    return expected == receivedSignature
}
```

**Тело вебхука при оплате:**
```json
{
  "update_id": 123,
  "update_type": "invoice_paid",
  "request_date": "2024-01-15T12:05:00Z",
  "payload": {
    "invoice_id": 1234567,
    "status": "paid",
    "asset": "USDT",
    "amount": "9.99",
    "paid_amount": "9.99",
    "paid_asset": "USDT",
    "fee": "0.01",
    "usd_rate": "1.00",
    "payload": "user_123|order_456",
    "paid_at": "2024-01-15T12:04:55Z"
  }
}
```

#### 2. Поллинг (getInvoices)

```go
func (c *CryptoPayClient) GetInvoices(ctx context.Context, invoiceIDs []int64) ([]Invoice, error) {
    // GET https://pay.crypt.bot/api/getInvoices
    // Params: invoice_ids=123,456&status=paid
}
```

---

### Статусы инвойса

| Статус | Значение |
|---|---|
| `active` | Ожидает оплаты |
| `paid` | Оплачен ✅ |
| `expired` | Истёк срок |

---

### Конфигурация (env vars)

```env
CRYPTO_PAY_TOKEN=<токен из @CryptoBot → Crypto Pay → Create App>
CRYPTO_PAY_WEBHOOK_URL=https://api.tyrax.app/webhooks/crypto-pay
```

---

## 3. СТРУКТУРА BACKEND (Go)

### Файловая структура

```
tyrax-backend/
└── internal/
    ├── handler/
    │   └── payment.go          # HTTP handlers для платежей
    ├── service/
    │   └── payment.go          # Бизнес-логика
    ├── repository/
    │   └── order.go            # DB операции с заказами
    ├── model/
    │   └── order.go            # Модели Order, Payment
    └── pkg/
        ├── freekassa/
        │   └── client.go       # FreeKassa API клиент
        └── cryptopay/
            └── client.go       # Crypto Pay API клиент
```

---

### Модели (model/order.go)

```go
type PaymentMethod string
const (
    PaymentSBP      PaymentMethod = "SBP"        // FreeKassa i=44
    PaymentCardRF   PaymentMethod = "CARD_RF"     // FreeKassa i=36
    PaymentCrypto   PaymentMethod = "CRYPTO"      // CryptoPay
)

type OrderStatus string
const (
    OrderNew      OrderStatus = "NEW"
    OrderPaid     OrderStatus = "PAID"
    OrderCancelled OrderStatus = "CANCELLED"
    OrderRefunded OrderStatus = "REFUNDED"
)

type Order struct {
    ID              string        `db:"id"`
    UserID          string        `db:"user_id"`
    Tier            string        `db:"tier"`           // CORE / SHADOW / DOMINION
    AmountRUB       float64       `db:"amount_rub"`
    PaymentMethod   PaymentMethod `db:"payment_method"`
    ExternalOrderID string        `db:"external_order_id"` // ID в FreeKassa или CryptoPay
    Status          OrderStatus   `db:"status"`
    CreatedAt       time.Time     `db:"created_at"`
    PaidAt          *time.Time    `db:"paid_at"`
}
```

---

### API эндпоинты для платежей

```
POST /api/v1/payment/create          # создать заказ, вернуть ссылку
GET  /api/v1/payment/status/:orderId # статус заказа
POST /webhooks/freekassa             # вебхук от FreeKassa (публичный, без JWT)
POST /webhooks/crypto-pay            # вебхук от CryptoPay (публичный, без JWT)
```

---

### Обработчик создания платежа (handler/payment.go)

```go
type CreatePaymentRequest struct {
    Tier          string `json:"tier"`           // CORE / SHADOW / DOMINION
    PaymentMethod string `json:"payment_method"` // SBP / CARD_RF / CRYPTO
    Email         string `json:"email"`
    IP            string `json:"ip"`
}

type CreatePaymentResponse struct {
    OrderID    string `json:"order_id"`
    PaymentURL string `json:"payment_url"` // ссылка для оплаты
}

// POST /api/v1/payment/create
func CreatePayment(c *fiber.Ctx) error {
    userID := c.Locals("user_id").(string)
    var req CreatePaymentRequest
    if err := c.BodyParser(&req); err != nil {
        return fiber.NewError(400, "INVALID REQUEST")
    }
    // → paymentService.CreateOrder(ctx, userID, req)
    // ← { order_id, payment_url }
}
```

---

### Обработчики вебхуков

```go
// POST /webhooks/freekassa  (NO JWT)
func FreekassaWebhook(c *fiber.Ctx) error {
    // 1. Проверить IP (доверенные: 168.119.157.136 и т.д.)
    // 2. Верифицировать SIGN через md5
    // 3. Найти заказ по MERCHANT_ORDER_ID
    // 4. Обновить статус на PAID
    // 5. Активировать подписку пользователя
    // 6. Вернуть "YES"
    return c.SendString("YES")
}

// POST /webhooks/crypto-pay  (NO JWT)
func CryptoPayWebhook(c *fiber.Ctx) error {
    // 1. Верифицировать crypto-pay-api-signature header
    // 2. Проверить update_type == "invoice_paid"
    // 3. Распарсить payload → userID + orderID
    // 4. Обновить статус заказа на PAID
    // 5. Активировать подписку
    return c.SendStatus(200)
}
```

---

## 4. ЦЕНЫ ТАРИФОВ (СПРАВОЧНИК)

> Cursor: используй эти значения при генерации кода тарифов.

### Базовые цены (помесячно)

| Тариф | Цена/мес | Устройства | Трафик | Ноды | Особенности |
|---|---|---|---|---|---|
| **FREE** | 0 ₽ | 1 | 1 GB/мес | Все | Без рекламы, полный авторекконект |
| **CORE** | 199 ₽ | 2 | Безлимит | Все | — |
| **SHADOW** | 349 ₽ | до 5 | Безлимит | Все | Приоритет на серверах |
| **DOMINION** | 649 ₽ | до 10 своих + пригласить 3 аккаунта | Безлимит | Все | Приоритет #1, ранний доступ к нодам, Telegram поддержка |

### Скидки за период (только CORE, SHADOW, DOMINION)

| Период | Скидка | CORE | SHADOW | DOMINION |
|---|---|---|---|---|
| 1 месяц | 0% | 199 ₽ | 349 ₽ | 649 ₽ |
| 3 месяца | 10% | 537 ₽ (~179/мес) | 942 ₽ (~314/мес) | 1752 ₽ (~584/мес) |
| 6 месяцев | 15% | 1015 ₽ (~169/мес) | 1780 ₽ (~297/мес) | 3310 ₽ (~552/мес) |
| 12 месяцев | 20% | 1912 ₽ (~159/мес) | 3350 ₽ (~279/мес) | 6230 ₽ (~519/мес) |

### Расчёт скидки в коде

```go
func ApplyPeriodDiscount(basePrice float64, months int) float64 {
    switch months {
    case 3:
        return basePrice * float64(months) * 0.90
    case 6:
        return basePrice * float64(months) * 0.85
    case 12:
        return basePrice * float64(months) * 0.80
    default: // 1 month
        return basePrice
    }
}
```

### Логика устройств (для backend)

```go
// Лимиты устройств по тарифу
func DeviceLimit(tier SubscriptionTier) int {
    switch tier {
    case TierFree:     return 1
    case TierCore:     return 2
    case TierShadow:   return 5
    case TierDominion: return 10
    default:           return 1
    }
}

// Лимит трафика в байтах (0 = безлимит)
func TrafficLimit(tier SubscriptionTier) int64 {
    if tier == TierFree {
        return 1 * 1024 * 1024 * 1024 // 1 GB
    }
    return 0 // unlimited
}

// Приоритет на нодах (меньше = выше приоритет)
func RoutingPriority(tier SubscriptionTier) int {
    switch tier {
    case TierDominion: return 1
    case TierShadow:   return 2
    default:           return 3
    }
}
```

### DOMINION: приглашение аккаунтов

```
Поток приглашения:
1. Владелец DOMINION → POST /api/v1/subscription/invite { account_id: "xyz" }
2. Backend проверяет: владелец DOMINION? invited_count < 3? аккаунт существует?
3. Создаёт запись: subscription_invites (owner_id, invitee_id, status=pending)
4. Приглашённый получает пуш-уведомление
5. Принимает → POST /api/v1/subscription/invite/accept { invite_id: "abc" }
6. Backend: users SET parent_subscription_id = owner_id WHERE id = invitee_id
7. Приглашённый работает с лимитами DOMINION
8. Его устройства считаются в общем лимите 10 устройств владельца

Удаление приглашённого:
DELETE /api/v1/subscription/invite/:account_id
→ users SET parent_subscription_id = NULL WHERE id = invitee_id
→ аккаунт возвращается на свой тариф
```

---

## 5. ЧЕКЛИСТ ИНТЕГРАЦИИ

- [ ] Зарегистрировать магазин на [merchant.freekassa.net](https://merchant.freekassa.net)
- [ ] Получить `shopId`, `API Key`, `Секретное слово 2`
- [ ] Включить методы `i=44` (СБП) и `i=36` (Карты РФ) в настройках магазина
- [ ] Указать `notification_url` в настройках FreeKassa
- [ ] Создать приложение в @CryptoBot → Crypto Pay → получить токен
- [ ] Включить Webhooks в @CryptoBot → Crypto Pay → My Apps → Webhooks
- [ ] Занести все ключи в `.env` (никогда не коммитить в git)
- [ ] Добавить `.env` в `.gitignore`
- [ ] Настроить проверку IP для FreeKassa вебхуков
- [ ] Протестировать через тестовый режим FreeKassa и @CryptoTestnetBot

---

## 6. ВАЖНЫЕ ОГРАНИЧЕНИЯ

- FreeKassa `i=36` и `i=44` — **API-only**, не форма. Не использовать SCI.
- `nonce` в FreeKassa должен быть **строго возрастающим** — используй `time.UnixMilli()`.
- Вебхуки обоих систем должны быть на **публичных эндпоинтах без JWT**.
- Перед активацией подписки **всегда проверять подпись** вебхука.
- Не доверять сумме из вебхука — сверять с суммой из БД по `orderID`.
