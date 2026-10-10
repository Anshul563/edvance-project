# Commerce Service

Owns the **commercial purchase domain**: carts, orders with immutable
price snapshots, coupons, purchases, and the boundaries to payment and
enrollment provisioning.

Port: `8089` · Database: `edvance_commerce` · Module:
`github.com/Anshul563/edvance-project/services/commerce-service`

## Responsibilities

- carts (one per user; live pricing, never stored prices)
- orders (snapshots, totals, lifecycle, human-friendly numbers)
- order items (immutable title/price/discount/currency per line)
- coupons (percentage basis points + fixed, caps, limits, windows)
- purchases (commercial ownership records)
- payment completion (trusted-result intake, no provider code)
- enrollment provisioning orchestration (interface + HTTP client)

## Non-responsibilities

Payment providers/webhooks (payment-service), enrollments and
learning state (learning-service), course data (course-service), users,
creators, videos, media, reviews, comments, recommendations, search.

## Money rules

Integers only, always minor units (`price_cents`, `discount_cents`,
`subtotal_cents`, `tax_cents`, `total_cents`, all `BIGINT`). No
`float64` anywhere near money. Percentages are basis points of a
percent: `1000 = 10%`, max `10000`. Discount formula:
`subtotal × value / 10000`, capped by `maximum_discount_cents`,
clamped to subtotal. Per-item discounts are distributed pro-rata
(floor shares, remainder to the priciest line) so lines always sum
exactly to the order discount.

## Environment

```env
APP_ENV=development
COMMERCE_SERVICE_PORT=8089
DATABASE_URL=postgres://postgres:postgres@localhost:5432/edvance_commerce?sslmode=disable
JWT_ACCESS_SECRET=   # must match auth-service (validate-only here)
JWT_ISSUER=edvance-auth
JWT_AUDIENCE=edvance-api
COURSE_SERVICE_URL=http://localhost:8086
LEARNING_SERVICE_URL=http://localhost:8087
LEARNING_SERVICE_INTERNAL_TOKEN=   # must match learning-service
COMMERCE_SERVICE_INTERNAL_TOKEN=   # must match payment-service
```

## Endpoints (all JWT-authed, strictly per-user)

```text
GET    /cart
POST   /cart/items                 {courseId}
DELETE /cart/items/:courseID
DELETE /cart                       (keeps the cart row)
POST   /orders                     {courseIds[], couponCode?}; optional Idempotency-Key
GET    /orders/:orderID            (owner only; foreign reads 404)
GET    /orders?page=&limit=&status=
POST   /coupons/validate           {code, courseIds[]} (never consumes)
GET    /purchases
GET    /purchases/:courseID
GET    /health  |  GET /ready
```

Through the gateway prefix with `/api/v1/commerce` (stripped):
`POST /api/v1/commerce/orders`, …

Errors use `{"error": {"code": "...", "message": "..."}}`. There is
deliberately **no** public payment-success endpoint: completion arrives
from the trusted payment-service integration calling `CompleteOrder`.
Internal paid and refund callbacks require the shared internal key and are
not exposed through the API Gateway.

## Cart rules

Courses must be published + public and not actively owned; duplicates
collapse to current state; removal of missing lines 404s; vanished
upstream courses are skipped (row kept); dead course-service fails the
view rather than showing stale prices.

## Order rules

Client sends course IDs + optional coupon code — never prices. The
service fetches live courses, enforces saleability/ownership, snapshots
title/price/currency per line, computes totals server-side (`tax = 0`
placeholder), mints `EDV-YYYYMMDD-XXXXXX` numbers (non-sequential),
and persists order + items + coupon consumption in one transaction.
Mixed-currency orders are rejected.

## Coupon rules

Active status, time window, usage limit, minimum order, currency match,
value bounds. Validation never consumes. Consumption is a conditional
`used_count + 1` inside order creation, so concurrent checkouts race
safely into `COUPON_USAGE_LIMIT_REACHED` instead of overshooting.

## Order lifecycle

```text
pending_payment → paid | failed | cancelled
paid → refunded | partially_refunded
partially_refunded → refunded
```

Enforced by `CanTransition` + compare-and-swap writes. `CompleteOrder`
is idempotent: paid orders replay (purchases fetched, provisioning
retried); anything non-pending refuses.

Checkout idempotency is scoped to the user. Commerce fingerprints the
course IDs and normalized coupon code: identical retries return the
original order, while reuse of a key with different request contents is
rejected. A full refund marks both order and purchase records refunded;
partial refunds retain active purchase records. Learning enrollments are
retained and are not automatically revoked by refunds.

## Purchase & provisioning

`CompleteOrder(orderID, paymentReference)` flips the order, creates one
purchase per line (unique, race-safe), then provisions enrollments with
`source = purchase` through the `EnrollmentProvisioner` interface
(HTTP client today, gRPC/events later). Provisioning failures are
logged, recorded per course, and retryable via repeat completion — the
paid order and purchases are never unwound.

## Local development

```bash
psql "$DATABASE_URL" -f migrations/001_create_commerce_tables.sql
psql "$DATABASE_URL" -f migrations/002_add_order_idempotency.sql
go run ./cmd/server
```

Course-service must run for pricing; coupons are seeded via SQL/Go
(no admin API in v1 by design).

## Testing

```bash
go test ./...
DATABASE_URL=postgres://.../edvance_commerce?... \
  go test -tags integration ./...
```

Unit tests use fake stores/clients; integration uses real PostgreSQL
(incl. coupon-race, cart-race, and payment-completion-race tests).
