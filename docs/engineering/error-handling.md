# Error handling

Follow the framework and repository conventions. Make failures visible to the layer that can recover, retry safely, or explain the outcome.

- Distinguish expected domain outcomes from unexpected faults using the project's established result/error style.
- Do not catch and ignore failures. Add context when rethrowing or translating, while preserving the original cause where supported.
- At external boundaries, validate responses and handle timeouts, rate limits, and unavailable dependencies according to the operation's retry and user-impact requirements.
- Retry only when the operation is safe to retry or protected by idempotency; use bounded retries and avoid retry storms.
- Return user-safe messages; do not expose stack traces, secrets, or internal details in public responses.
- Ensure failures in background work are observable and have an actionable retry/dead-letter or operator path where required.
