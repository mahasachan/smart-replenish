# Design pattern decision rules

Patterns are tools for specific forces in the problem, not goals. First inspect existing solutions. Use the simplest option that meets current requirements.

| Situation | Prefer | Avoid |
|---|---|---|
| One straightforward synchronous operation | A direct function call | Queue, event bus, or mediator without a real need |
| Multiple external providers implement a shared business capability | An Adapter at each provider boundary | Vendor SDK types leaking through business logic |
| Behavior genuinely varies at runtime and more variants are expected or configured | Strategy | Turning a small, stable conditional into a class hierarchy |
| Persistence abstraction materially helps business rules, testing, or multiple storage implementations | A domain-specific persistence interface with useful operations | A generic `Repository<T>` by default |
| A database update and message publication must be reliably coordinated | Transactional Outbox, with delivery/idempotency behavior made explicit | Best-effort publish after commit when message loss causes inconsistency |
| Clients may retry a request whose repeated effects would be harmful | An idempotency key scoped to the operation and safely persisted | Blanket idempotency infrastructure for every endpoint |
| Work is non-critical and can happen after the response | An existing background worker/queue if latency or reliability requirements justify it | Introducing a distributed queue for work that is cheap and synchronous |
| A third-party API differs from the domain's useful contract | A narrow Adapter that translates at the boundary | Exposing vendor models in core rules |

## Pattern test

Before introducing a pattern, state the concrete variation, failure mode, or boundary it addresses. Identify its operational cost and how it will be tested. If there is no clear answer, keep the direct implementation. Do not stack Factory + Strategy + Repository + Mediator + Event Bus around a simple endpoint.
