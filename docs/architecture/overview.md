# Architecture overview

SmartReplenish is a local-development replenishment simulator. It estimates when a user
may need to buy a product again and creates suggestions, never purchases. The rule engine
controls suggestions; TypeSafe Jev can make shadow decisions for comparison only.

The repository has a Go service in `backend/` and a React/TypeScript simulation UI in
`frontend/`. The root Makefile coordinates both. The UI calls the existing REST API
through Vite's development proxy. The Go service is a modular monolith: PostgreSQL
persists facts, suggestions, feedback, and decision logs; its internal packages separate
prediction, decision, policy, action, storage, and HTTP handling.

The critical evaluation flow is **Facts → Prediction → Decision → Policy → Action →
Feedback**. Production rules and Jev receive the same compact decision context. The
provider response is logged for comparison and cannot create a suggestion. Provider
credentials stay in the backend environment. The server applies embedded SQL migrations
on startup and seed.

See the [root README](../../README.md) for local setup and verification commands,
[design](../design.md) for package boundaries and behavior, and [API examples](../api.md)
for requests. This MVP has no authentication, deployment, or recovery runbook; it must
not be exposed as a public service without those capabilities.
