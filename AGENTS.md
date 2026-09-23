# Repository instructions

GitHub Issues are the source of truth for requirements, acceptance criteria, and work status. Pull requests must link the issue they complete.

When asked in chat to turn an epic card into work, create or reuse exactly one GitHub parent issue, return its URL, and wait for a maintainer approval comment on the current requirements and acceptance criteria. A `ready-for-agent` label is not approval. Never substitute chat text for a GitHub issue when GitHub is unavailable; report the blocker.

## Workflow ownership

- Use Matt Pocock Skills for requirement discovery, domain understanding, issue refinement, ticket decomposition, triage, and issue-based code review.
- Use Superpowers for engineering execution: worktrees, implementation planning, TDD, debugging, verification, and branch completion.
- `docs/engineering/development-workflow.md` is the canonical routing policy. Read it before non-trivial work.
- Do not repeat discovery or planning. An approved issue is the requirements brief; only reopen its scope when implementation reveals a real ambiguity, and record the resolution on the issue.

## Engineering decisions

Before significant design or implementation, inspect existing code and read relevant documentation in `docs/engineering/`, especially `architecture-principles.md`, `design-patterns.md`, and `anti-patterns.md`, plus relevant files in `docs/architecture/` and `docs/adr/`. Follow established conventions and choose the simplest design that meets the issue's acceptance criteria.

An architecture overview, domain glossary, or ADR may be a starter template. Do not infer project facts from placeholders. Record significant decisions in `docs/adr/`.

## Completion

Verify changes using the repository's documented commands. Do not claim checks passed unless they ran. Have a human review and merge the pull request.
