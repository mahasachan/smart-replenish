# Smart Replenishment

A Go/PostgreSQL modular monolith that turns purchase history into replenishment
suggestions. Application code owns every side effect:

**Facts → Prediction → Decision → Policy → Action → Feedback**

The rule engine controls production. Jev can evaluate the same compact contexts in
shadow mode; its decisions are logged and never executed. No automatic purchases,
cart additions, outbound notifications, ML, Redis, or background infrastructure.

## Run locally

Requires Go 1.26+ and Docker with Compose. Install Node.js 20+ and npm to run the frontend.
Defaults bind PostgreSQL and HTTP to loopback.

Use `http://127.0.0.1:8080` for this service. On machines where another local API owns
IPv6 `localhost:8080`, `http://localhost:8080` can reach that other process and return its
404 instead. This project intentionally binds IPv4 loopback by default; set `HTTP_ADDR` to
another free address or port if needed.

```sh
cp backend/.env.example backend/.env
# Optional: edit backend/.env. Root run and seed commands load it automatically.

make db
make seed
make run
```

In a second terminal, start the simulation frontend:

```sh
make frontend-install
make frontend
```

Open the Vite URL printed in the terminal (usually `http://127.0.0.1:5173`). The frontend
proxies API calls to this backend. Create users, products, and dated purchases to build
scenarios, then evaluate them and compare production rules with Jev shadow decisions. Jev
remains optional; configure its key in `backend/.env` and restart the backend to enable it.

The server and seed command apply embedded migrations automatically. A migration ledger
and PostgreSQL advisory lock make repeated startup safe. PostgreSQL uses a named volume.
`docker compose stop` stops it without deleting data.

The seed is repeatable: it reuses one user, three products, and stable purchase keys.
Day 36 is anchored to the seed user's creation time. Milk was purchased on days
1/8/15/22/30, eggs on 1/9/17/25, and television once on day 1. On initial setup, milk
and eggs should be suggested; television is explicitly ineligible. As real time passes,
the estimates advance. Rerunning the seed does not reset user feedback or history.

```sh
curl -sS http://127.0.0.1:8080/healthz
curl -sS http://127.0.0.1:8080/users/00000000-0000-4000-8000-000000000001/replenishment
curl -sS -X POST http://127.0.0.1:8080/users/00000000-0000-4000-8000-000000000001/decisions/evaluate
curl -sS http://127.0.0.1:8080/users/00000000-0000-4000-8000-000000000001/suggestions
```

Use a returned suggestion ID to record a real impression and accept or dismiss:

```sh
SUGGESTION_ID='<id returned by the API>'
curl -sS -X POST "http://127.0.0.1:8080/suggestions/$SUGGESTION_ID/shown"
curl -sS -X POST "http://127.0.0.1:8080/suggestions/$SUGGESTION_ID/accept"
# For another pending suggestion:
# curl -sS -X POST "http://127.0.0.1:8080/suggestions/$SUGGESTION_ID/dismiss"

curl -sS 'http://127.0.0.1:8080/users/00000000-0000-4000-8000-000000000001/decision-history?limit=50&offset=0'
curl -sS http://127.0.0.1:8080/users/00000000-0000-4000-8000-000000000001/evaluation-metrics
```

Acceptance explicitly adds products to the shopping list, never to the cart. The suggested
list section remains a proposal until accepted. Repeating the same feedback is idempotent;
accepting an already dismissed suggestion returns 409. Acceptance/dismissal also ensures
one deduplicated impression. Reading suggestions alone records no impression.

More request examples: [docs/api.md](docs/api.md).
Architecture, schema, Mermaid flow and acceptance criteria: [docs/design.md](docs/design.md).
Repository workflow and contribution guidance: [AGENTS.md](AGENTS.md) and
[docs/engineering/development-workflow.md](docs/engineering/development-workflow.md).

## Jev shadow evaluation

Jev is available through TypeSafe directly or through OpenRouter. The OpenRouter path is
configured in this project by default:

```sh
# Set this in the shell that launches the server; keep the key private.
export OPENROUTER_API_KEY='your OpenRouter API key'
export JEV_MODEL='typesafe/jev-1.13'
make run
```

Alternatively, copy `backend/.env.example` to `backend/.env`, put your key in
`OPENROUTER_API_KEY`, and restart the server after changing the key. Root `make run` loads
those values automatically. You can also set `JEV_ENDPOINT`; its default is
`https://openrouter.ai/api/alpha/decisions`. The model ID is `typesafe/jev-1.13`.
OpenRouter's `~typesafe/jev-latest` alias is also available if you prefer the latest family
model. OpenRouter bills the request to your OpenRouter account.

Jev uses OpenRouter's dedicated Decisions API, not `/api/v1/chat/completions`. The request
uses the same `state` and typed `choice` question shape as TypeSafe's System One API. See
the [OpenRouter Jev model listing](https://openrouter.ai/typesafe) and its
[Decisions API reference](https://openrouter.ai/docs/api/api-reference/alphadecisions/submit-a-decisions-request).

For a direct TypeSafe key instead, set `TYPESAFE_API_KEY` and optionally
`TYPESAFE_MODEL=jev-latest`; leave `OPENROUTER_API_KEY` empty. If both are set, OpenRouter
takes precedence. Without either key, the application works entirely with rules.

Only the six predefined action names are valid; application code validates the response
enum, confidence and distribution. OpenRouter's reported cost and token usage and the
actual returned model are logged. Prediction confidence, selected-option probability, and
provider confidence are distinct.

Production commits first. Shadow calls have a 3-second per-request deadline and an
8-second total provider budget; persistence has a separate bounded deadline. Failures are
logged when possible, and the response reports omitted comparisons. No automatic retries
or background workers. Requests can therefore take several seconds when shadow is enabled.
Jev receives aggregate state, never the raw purchase or event history.

No live provider call is part of the test suite. Mock HTTP contract tests verify the
adapter without spending API credits or requiring credentials.

## Tests

```sh
make test        # Go tests; integration tests skip without TEST_DATABASE_URL
make check       # Go vet, race-enabled tests, and frontend production build/typecheck
make integration # real PostgreSQL, isolated temporary schemas, race detector
```

Or run everything together:

```sh
cd backend
TEST_DATABASE_URL='postgres://smart:smart@localhost:55432/smartreplenish?sslmode=disable' go test -race ./...
```

Integration tests create and drop uniquely named schemas. The database role needs schema
creation permissions. They do not reset application tables. Coverage includes migrations,
HTTP evaluation/feedback, transaction rollback, purchase idempotency, concurrent evaluation,
policy suppression, bundles, exact-context shadow pairing, and shadow-provider failure.

## Main behavior

- Purchase occasions are distinct local calendar dates. At least three occasions and a
  replenishability score of 0.5 are required for an eligible estimate.
- Median repurchase interval is the prediction baseline. Mean and population variance are
  diagnostic fields. Remaining days may be negative when a product is overdue.
- Regularity = `1 / (1 + mean(abs(interval - median)) / median)`.
- Confidence = `min(1, interval_count / 4) * regularity * replenishable_score`.
  This is an explainable heuristic, not a calibrated probability.
- Rules suggest when remaining days <= 2 and confidence >= 0.7. Other candidates wait.
- Policy blocks disabled suggestions, unavailable products, existing cart/list membership,
  ineligible products, a 12-hour product cooldown, and existing pending suggestions.
- A user row lock serializes purchase, preference, membership, evaluation, and feedback
  changes. Shared product locks protect availability during production evaluation. Local
  rules execute inside that transaction; no remote provider call holds its locks.
- Decision logs distinguish selected, permitted, and executed actions. Suggestion and
  production logs commit together. Failed transactions leave no partial suggestions.
- Money is integer minor units in one application currency. Quantity is whole SKU units.
  `total_amount` must equal the item totals; tax/discount fields are not modeled yet.
- Purchases use an `Idempotency-Key`; changed content with the same key returns 409.
  Purchases within the last 24 hours clear purchased items from current cart/list state;
  older imported receipts preserve current membership.

## Evaluation and limits

Metrics are item-level descriptive observations. Acceptance/dismissal use unique shown
item decisions; conversion uses only mature 24h/3d/7d windows. A purchase is attributed to
the most recent preceding impression for that product within seven days, using purchase
time. Duplicate receipts and repeated feedback do not inflate counts. Late imported
receipts can change metrics. Shadow actions have agreement/coverage/error/latency metrics,
not conversion claims. Dismissal and purchases after no action are only proxies.

This is a local-development MVP. There is no authentication or tenant authorization; add
those before public exposure. `/healthz` reports process liveness. The server verifies the
database on startup. Lists accept `limit` (1–100) and `offset` (0–100000), request bodies are
limited to 1 MiB, purchases to 100 items, evaluations to 200 previously purchased products.
The fact loader currently reads a user's complete history from PostgreSQL; the provider
still receives only aggregates. Larger histories need database aggregation and retention
work before deployment. Notification fatigue, true unnecessary-suggestion labels, stable
A/B assignment, fractional quantities and calibrated confidence remain future work.
