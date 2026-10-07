# Commerce Service Architecture

## Boundary

```text
Course Service (current pricing)
      ↓  reads only: course identity, saleability, live price
Commerce Service
      ├── Cart (relationships only, live prices)
      ├── Order (immutable snapshots + totals)
      ├── Coupon (validation + concurrency-safe consumption)
      └── Purchase (commercial ownership)
          │
          ▼  trusted payment result (future payment-service)
Payment Service (future)
      │  provider communication (Razorpay/Stripe)
      ▼  webhooks, verification
Commerce Service (CompleteOrder)
      │
      ▼  EnrollmentProvisioner (HTTP today, gRPC/events later)
Learning Service
      │
      ▼  Enrollment (source = purchase)
```

Clearly stated:

```text
course-service owns current course pricing.
commerce-service owns historical order pricing snapshots.
payment-service owns payment-provider communication.
learning-service owns enrollment and learning access.
```

## Money flow

```text
client sends: course IDs + coupon code
server computes: live prices → subtotal → discount → tax(0) → total
server freezes: per-line snapshots (title/price/discount/currency)
later price changes: never touch history
```

All values are integer minor units end to end (`BIGINT` columns, `int64`
code, basis-point percentages). No `float64` exists in the money path.

## Why snapshots, not references

An order must mean the same thing forever. Storing only `course_id`
would let later price edits rewrite what a customer paid — a support,
audit, and legal hazard. Snapshots cost a few columns and remove the
entire class of bug.

## Transaction boundaries

```text
order creation:      order + items + coupon increment + usage row (1 tx)
payment completion:  order flip + purchases (1 tx)
provisioning:        AFTER commit, per course, failures recorded
```

Provisioning deliberately sits outside the money transaction: a
learning-service outage must delay enrollment, never unwind payment.
Repeat completion replays state and retries provisioning — the system
converges instead of duplicating.

## Concurrency design

- Coupon limits: conditional `used_count + 1` under the order tx.
  100 racers for 1 slot → exactly 1 winner (tested).
- Payment completion: order row lock + terminal-state short-circuit.
  4 concurrent completions → 1 paid order, 1 purchase (tested).
- Cart adds: unique constraint → collapse to current state (tested).
- Enrollment creation: unique constraint → return existing (tested).

## What is intentionally missing

- Payment providers, webhooks, refunds execution (placeholders only).
- Coupon/order admin APIs (seeded via SQL; admin console later).
- Real enrollment provisioning target: the HTTP provisioner exists
  and is mock-tested, but learning-service does not yet expose the
  internal enroll endpoint it calls — provisioning failures are
  therefore the *expected* live behavior until that route lands, and
  the paid order is always preserved for retry.
- Events/message bus: repeat-completion is the retry mechanism until
  one exists.
