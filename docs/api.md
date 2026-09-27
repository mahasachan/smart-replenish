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
  -H 'Content-Type: application/json' -d '{"in_cart":true,"in_list":false,"cart_quantity":2}'

curl -sS http://127.0.0.1:8080/users/00000000-0000-4000-8000-000000000001/cart
curl -sS 'http://127.0.0.1:8080/products?limit=100&offset=0'
```

PUT replaces the membership; omitted flags are false. `cart_quantity` must be 1–999 for a
cart item (it defaults to 1) and 0 otherwise. The `auto_added` marker is kept while the
item stays in the cart and cleared when it leaves. Membership changes create corresponding
events. GET cart returns the products that are in the cart or on the list. The application
adds a product to the cart itself only after the user confirms a running-low question.
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
policy outcome is DO_NOTHING. Repeating evaluation creates new audit records, but pending
questions, membership checks, cooldowns and running-low snoozes prevent duplicates. Rules
choose ASK_IF_RUNNING_LOW or WAIT. The other bounded actions are implemented in policy and
execution for future production engines. Configuring Jev only enables shadow evaluation.

## Running-low answers

```sh
QUESTION_ID='<returned ASK_IF_RUNNING_LOW suggestion UUID>'
curl -sS -X POST "http://127.0.0.1:8080/suggestions/$QUESTION_ID/shown"
curl -sS -X POST "http://127.0.0.1:8080/suggestions/$QUESTION_ID/answer" -d '{"running_low":true}'
```

`running_low` is required. `true` accepts the question, adds the product to the cart with
its median purchase quantity and marks it `auto_added`. `false` dismisses the question and
snoozes that product for half its median repurchase interval (minimum one day). Answering a
question that is not `ASK_IF_RUNNING_LOW` returns 400. Answering is idempotent, and the
opposite answer returns 409. `accept` and `dismiss` on a question behave the same as
`true` and `false`. Evaluation metrics include `running_low` precision and counts.

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
