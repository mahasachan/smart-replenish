# Architecture principles

These are defaults for decisions not already made by the codebase or an ADR.

## Prefer the simplest design that meets the requirement

Optimize for understandable behavior and safe change. Add a component, abstraction, dependency, or deployment unit only when it solves a concrete requirement visible in the issue or production constraints.

Do not introduce a broker, queue, microservice, distributed cache, CQRS, event bus, or framework layer speculatively. Prefer a direct function call and the existing datastore/runtime for a simple synchronous operation.

## Follow the repository before inventing conventions

Before changing design:

1. Inspect the repository structure and relevant code paths.
2. Search for a similar behavior and understand how it is used.
3. Read the architecture overview and relevant ADRs.
4. Follow established conventions unless they conflict with an approved requirement or a documented decision.

Avoid unrelated rewrites. Improve a nearby seam only when it materially enables the requested change.

## Keep responsibilities clear without purity projects

Keep business rules understandable and avoid coupling them unnecessarily to HTTP frameworks, persistence details, vendor SDKs, or message brokers. Introduce a boundary when it helps isolate change, test behavior, or meet a real integration need—not merely to satisfy a theoretical layer diagram.

Prefer cohesive modules with useful behavior behind clear interfaces. Avoid both a monolithic God service and a forest of one-line forwarding abstractions.

## Production concerns are requirements when relevant

For changes that touch production behavior, evaluate the actual risks: authorization, sensitive data, validation, concurrency, idempotency, transactional consistency, retries/timeouts, backward compatibility, migrations, observability, and rollback or recovery. Address the risks relevant to the change; do not add boilerplate controls without a failure mode they mitigate.

## Escalate architectural decisions

If a requested change conflicts with an ADR or requires a material architectural decision, explain the conflict and alternatives to the human. Record the approved choice as an ADR. Never silently rewrite historical decisions.
