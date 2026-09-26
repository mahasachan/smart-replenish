# REST examples

Base URL: `http://127.0.0.1:8080`. JSON responses; errors use `{"error":"..."}`.
Invalid input returns 400, missing resources 404, conflicting feedback/idempotency/SKU 409.
The local MVP has no authentication. IDs in URLs are not authorization.

## Create facts

```sh
curl -sS -X POST http://127.0.0.1:8080/users \
  -H 'Content-Type: application/json' \
  -d '{"timezone":"Asia/Bangkok","suggestion_enabled":true,"notification_enabled":false}'

curl -sS -X POST http://127.0.0.1:8080/products \
  -H 'Content-Type: application/json' \
  -d '{"sku":"water-001","name":"Drinking water","category":"water","brand":"Example","unit":"pack","replenishable_score":0.97,"available":true}'
```

All quantities are whole units, money is integer minor units. Allowed purchase sources:
`app`, `receipt`, `POS`, `imported_invoice`. Timestamps use RFC 3339 and cannot be in the future.
The following request records one purchase against the seeded user and milk product:

```sh
PURCHASED_AT=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
curl -sS -X POST http://127.0.0.1:8080/purchases \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: demo-milk-purchase-1' \
  -d "{\"user_id\":\"00000000-0000-4000-8000-000000000001\",\"purchased_at\":\"$PURCHASED_AT\",\"source\":\"app\",\"total_amount\":13000,\"items\":[{\"product_id\":\"00000000-0000-4000-8000-000000000101\",\"quantity\":2,\"unit_price\":6500}]}"
```

To test a retry, resend the identical body, including timestamp, and the same key. Use a
new key for a different purchase. Reusing a key with changed content returns 409.

## Preferences, availability and current shopping state

```sh
curl -sS -X PATCH http://127.0.0.1:8080/users/00000000-0000-4000-8000-000000000001 \
  -H 'Content-Type: application/json' -d '{"suggestion_enabled":false}'

curl -sS -X PATCH http://127.0.0.1:8080/products/00000000-0000-4000-8000-000000000101 \
  -H 'Content-Type: application/json' -d '{"available":false}'

curl -sS -X PUT http://127.0.0.1:8080/users/00000000-0000-4000-8000-000000000001/products/00000000-0000-4000-8000-000000000101/state \
  -H 'Content-Type: application/json' -d '{"in_cart":true,"in_list":false}'
```

PUT replaces both membership flags; omitted flags are false. Membership changes create
corresponding events. The application never adds products to the cart on its own.
User PATCH supports `timezone`, `suggestion_enabled`, and `notification_enabled`.
Product PATCH supports `available` and `replenishable_score`.

Record a product view:

```sh
curl -sS -X POST http://127.0.0.1:8080/users/00000000-0000-4000-8000-000000000001/events \
  -H 'Content-Type: application/json' \
  -d '{"product_id":"00000000-0000-4000-8000-000000000101","event_type":"PRODUCT_VIEWED"}'
```

Only views are accepted through this generic endpoint. Purchase, cart/list changes and
suggestion feedback have dedicated endpoints that generate their own trustworthy events.
The schema reserves `NOTIFICATION_OPENED`; notification delivery is not implemented.

## Evaluate and inspect

```sh
curl -sS http://127.0.0.1:8080/users/00000000-0000-4000-8000-000000000001/replenishment
curl -sS -X POST http://127.0.0.1:8080/users/00000000-0000-4000-8000-000000000001/decisions/evaluate
curl -sS 'http://127.0.0.1:8080/users/00000000-0000-4000-8000-000000000001/suggestions?limit=20&offset=0'
curl -sS 'http://127.0.0.1:8080/users/00000000-0000-4000-8000-000000000001/decision-history?limit=20&offset=0'
curl -sS http://127.0.0.1:8080/users/00000000-0000-4000-8000-000000000001/evaluation-metrics
```

Evaluation creates a log for every purchased product, including ineligible products whose
policy outcome is DO_NOTHING. Repeating evaluation creates new audit records but pending
suggestions, membership checks and cooldowns prevent duplicate proposals. Rules currently
choose SUGGEST_NOW or WAIT; other bounded actions are implemented in policy and execution
for future production engines. Configuring Jev only enables shadow evaluation.

## Feedback

```sh
SUGGESTION_ID='<returned suggestion UUID>'
curl -sS -X POST "http://127.0.0.1:8080/suggestions/$SUGGESTION_ID/shown"
curl -sS -X POST "http://127.0.0.1:8080/suggestions/$SUGGESTION_ID/accept"
# Or dismiss a pending suggestion:
# curl -sS -X POST "http://127.0.0.1:8080/suggestions/$SUGGESTION_ID/dismiss"
```

Feedback applies to every item in a bundle and preserves each decision reference.
A pending suggestion becomes fulfilled when all its products have been purchased after
its creation. A fulfilled or oppositely resolved suggestion rejects new accept/dismiss
feedback. Same-resolution repeats are idempotent.
