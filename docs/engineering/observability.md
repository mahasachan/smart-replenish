# Observability

Use the logging, metrics, tracing, and alerting already present in the project. Add signals when they help detect or diagnose an important production outcome.

- Prefer structured logs with useful operation/entity identifiers and outcome; avoid secrets, credentials, and unnecessary personal data.
- Instrument critical user and system outcomes, dependency failures, latency, and queue/backlog behavior where those concepts exist.
- Avoid high-cardinality metric labels and noisy per-request logs without operational value.
- For a new asynchronous or failure-prone flow, identify how an operator will detect failure and safely recover it.
- Do not introduce a new observability vendor or stack for a single change without an explicit requirement.
