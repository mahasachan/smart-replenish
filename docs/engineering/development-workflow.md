# Development workflow

This document routes work between GitHub, Matt Pocock Skills, and Superpowers. It is policy, not a new skill system.

## Authority and durable state

- GitHub Issues are authoritative for requested behavior, acceptance criteria, scope decisions, and status.
- A chat request to turn an epic card into development work must result in one GitHub parent issue; the issue URL is the handoff back to the requester.
- A design conversation is not durable until its agreed decisions are recorded in the relevant issue or ADR.
- A pull request is evidence of implementation. Link the issue and report verification; humans decide whether to merge.
- Repository architecture guidance describes this project's conventions. It does not replace issue requirements.

## Responsibility boundary

| Concern | Owner | Expected output |
|---|---|---|
| Requirements, domain language, scope, issue refinement, ticket decomposition, triage | Matt Pocock Skills | Approved GitHub Issue(s) |
| Technical execution plan, worktree, implementation, TDD, debugging, verification, branch completion | Superpowers | Verified code change and pull request |
| Merge and material architectural decisions | Human | Approval / recorded decision |

Do not run two discovery or implementation-planning flows for the same work. For this workflow, do not use Matt's `implement` flow: implementation and TDD belong to Superpowers. Do not use Superpowers brainstorming to reopen requirements already settled in the issue. If an unresolved technical choice needs brainstorming, limit it to that choice; record any product or scope change on the issue and obtain human approval before proceeding.

Matt's `code-review` is the requirements-and-repository-standards review. Superpowers' code-review skill is an execution checkpoint, not a substitute for that final review. Fix findings and rerun relevant verification before branch completion.

## Choose the lightest workflow that fits

### Epic from chat

When the user gives an epic-level card in chat and asks to turn it into work:

1. Check for a supplied issue reference and search open GitHub Issues for an existing matching parent before creating anything. Reuse and refine a supplied or matching issue; do not create duplicate parent issues.
2. If no issue exists, use Matt `to-spec` to synthesize the card into one parent GitHub issue. If the requirements need discovery, use Matt `grill-with-docs` / triage to resolve the important questions, then update that same issue. Do not publish an intake issue and a second spec issue for the same epic.
3. Ensure the issue captures the problem, desired outcome, acceptance criteria, constraints, out-of-scope items, and unresolved questions. Do not invent decisions to make an incomplete epic look complete.
4. Return the issue URL and wait. Do not decompose or implement until a maintainer comments on the issue approving its current requirements and acceptance criteria.
5. A `ready-for-agent` label means the issue is sufficiently specified for agent work; it is not a substitute for the maintainer's approval comment. If scope or acceptance criteria materially change after approval, pause and request approval again.

If GitHub is unavailable or the agent lacks permission to publish, explain that issue creation is blocked and ask for access. Do not treat chat text or a local file as a created GitHub issue.

### Substantial feature

1. Use Matt discovery / `grill-with-docs` to clarify user need, domain terms, and constraints.
2. Use Matt `to-spec` to create the issue containing the approved behavior, acceptance criteria, relevant implementation/testing decisions, and explicit out-of-scope items.
3. A maintainer comments on the parent issue approving its current requirements and acceptance criteria before implementation or decomposition. A triage label alone is not approval.
4. Use Matt `to-tickets` only when the work benefits from multiple tickets. Prefer end-to-end, independently verifiable vertical slices; create dependency edges only when they truly gate work. A single clear ticket needs no decomposition.
5. Start a fresh implementation context for each ticket. Read the full issue and relevant parent context, including the approval comment. Use Superpowers worktree, implementation plan, TDD, and execution skills.
6. Verify against the issue, use Matt `code-review` against the issue and repository standards, resolve findings, then use Superpowers branch completion to prepare the PR.

### Small, clear task

Start from the existing GitHub Issue. Skip discovery, specification, and decomposition when the issue already defines the change and completion conditions. Implement with the appropriate Superpowers execution discipline, run relevant tests and verification, review against the issue, and open a linked PR.

### Bug

1. Use Matt `triage` to classify and clarify the report. Verify the claim when feasible; do not label an unverified hypothesis as a confirmed defect.
2. Use Matt `diagnosing-bugs` and Superpowers `systematic-debugging` to reproduce and identify the root cause before changing production code.
3. Add a regression test that fails for the reported behavior, then fix the cause using TDD.
4. Run verification-before-completion, review against the issue, and open a linked PR.

If reproduction is blocked by missing details or environment access, record what was tried and what is needed on the issue; do not guess at a fix.

## Pull request and issue completion

- Include `Closes #<issue>` only when this PR completes that issue. For child work, close the child issue and leave the parent open until its scope is complete.
- Summarize behavior changed, acceptance criteria addressed, tests/checks run and outcomes, operational or migration notes, and unresolved risks.
- Do not close issues, merge PRs, or make material architectural decisions without the project's configured human approval. Only a maintainer can approve and merge; agent-authored review is not human approval.

## Production change checks

Use the repository's actual build, test, lint, typecheck, migration, and security checks where applicable. Consider failure modes, authorization, data integrity, backwards compatibility, observability, rollback/recovery, and deployment sequencing when they matter to the change. Do not add infrastructure or checklist work unrelated to the issue.
