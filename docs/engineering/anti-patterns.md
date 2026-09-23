# Design anti-patterns to avoid

- **Premature abstraction:** do not generalize from one use case or speculative future requirements. Wait for demonstrated variation or a real seam.
- **Generic CRUD architecture everywhere:** model domain operations where their rules matter; do not force every operation through identical repositories/services/controllers.
- **God services:** keep unrelated responsibilities out of a single service. Split only along cohesive responsibilities with meaningful interfaces.
- **Unnecessary interfaces:** concrete implementations are fine when there is one stable implementation and no useful isolation seam.
- **Speculative abstractions:** do not add plugins, hooks, configuration, or extension points without a current consumer.
- **Pattern stacking:** avoid combining several patterns when one direct module/function is sufficient.
- **Architecture rewrites by adjacency:** do not rewrite surrounding code just because the requested change touches it.
- **Generic error swallowing:** preserve actionable context and handle failures at a boundary that can make a useful decision.
- **Infrastructure by fashion:** do not add distributed systems, caches, queues, or eventing to solve local synchronous problems.

When an existing design appears problematic, describe the concrete risk and propose the smallest change that addresses it. Do not silently expand issue scope.
