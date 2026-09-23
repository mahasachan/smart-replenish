# GitHub-native AI development workflow

A reusable repository template for production software teams using Cursor, Claude Code, Codex, OpenCode, or a mix of them. GitHub Issues are the durable source of truth; a small repository policy routes requirements through Matt Pocock Skills and engineering execution through Superpowers.

## Start a new project

1. In the source repository's GitHub settings, enable **Template repository**. Create a new repository from it and clone that repository.
2. Run `scripts/setup-ai.sh` for a read-only project readiness check. Use `--agent cursor`, `--agent claude`, `--agent codex`, or `--agent opencode` to select an agent explicitly; use multiple `--agent` flags for a team using more than one.
3. Run `scripts/setup-ai.sh --install --agent <agent>` to invoke documented Matt Pocock Skills installers where supported and print agent-native Superpowers installation steps. Select `setup-matt-pocock-skills` in the installer. Verify plugins inside the agent; the script deliberately does not claim it can inspect every vendor's plugin state.
4. If GitHub labels are missing, run `scripts/setup-ai.sh --setup-labels` to create missing defaults. This is explicit and additive; existing labels are never renamed or overwritten. For custom existing-project labels, edit `docs/agents/triage-labels.md` and issue template frontmatter to match.
5. Replace the starter architecture overview with verified project facts. Add the project's real setup, build, test, lint, typecheck, migration, deployment, and recovery commands to its documentation and CI.
6. Enable Issues and Actions, configure branch protection and required checks, and confirm the PR template/issue linking convention with maintainers.

The template provides engineering defaults, not an application architecture or a guarantee of production readiness. Each project must document its real runtime, data, security, operational, and release constraints.

## Adopt in an existing repository

Do not replace existing instructions or architecture documentation wholesale. Review `AGENTS.md`, `CLAUDE.md`, `.claude/`, Cursor/Codex configuration, GitHub templates, CI, and ADRs; preserve project-specific rules and route them to the shared instructions without duplicating the workflow. If the repository already has a root `CLAUDE.md`, keep it as the Claude entry point and merge or import the shared policy there; do not add `.claude/CLAUDE.md` as a competing second entry point. Keep one canonical workflow document. Preserve existing issue labels and map Matt's triage roles in `docs/agents/triage-labels.md`. Keep project-specific ADRs and adapt the overview to observed architecture. Add files selectively, then run `scripts/setup-ai.sh` and the repository's real verification checks. `scripts/check-template.sh` is only for this template source, not for validating an adopted project.

## Workflow at a glance

- **Epic from chat:** ask the agent to turn your epic card into work → it checks for an existing issue and creates or reuses one GitHub parent issue → it returns the URL and waits for your approval comment. No child tickets or implementation before approval. A `ready-for-agent` label is not approval.
- **Substantial feature:** Matt discovery → approved GitHub issue/spec → optional vertical-slice child issues → fresh context per ticket → Superpowers worktree/plan/TDD/verification → Matt issue-based review → Superpowers branch completion → linked PR.
- **Clear small task:** existing issue → implement/test/verify/review → linked PR. Skip unnecessary discovery and decomposition.
- **Bug:** triage and root-cause diagnosis → reproduce → regression test → fix → verification → review → linked PR.

See [`docs/engineering/development-workflow.md`](docs/engineering/development-workflow.md) for the routing contract and responsibility boundaries. Agent entry points are intentionally thin and all point to one shared policy.

## Agent instruction loading

| Agent | Active project instructions in this template |
|---|---|
| Claude Code | `.claude/CLAUDE.md` imports the root `AGENTS.md` |
| Cursor | Root `AGENTS.md` plus `.cursor/rules/development-workflow.mdc` |
| Codex | Root `AGENTS.md`; `.codex/README.md` is setup documentation only |
| OpenCode | Root `AGENTS.md` |

`.agents/README.md` explains that `.agents/` may hold installed skills but is not a universal rules directory. Do not create separate copies of the full workflow in these agent-specific locations. `scripts/setup-ai.sh` detects CLI commands as candidates; the committed template directories do not count as proof that an agent is installed.

## Repository map

- `.github/ISSUE_TEMPLATE/` and `.github/pull_request_template.md`: issue and change evidence shapes.
- `.claude/`, `.cursor/`, `.codex/`, and `.agents/`: thin agent entry point or setup documentation; see the loading table above.
- `docs/engineering/`: project design defaults and workflow.
- `docs/architecture/`: verified project architecture, filled in by maintainers.
- `docs/adr/`: approved project-specific architectural decisions.
- `docs/agents/`: Matt Pocock Skills issue-tracker and domain-doc configuration.
- `scripts/setup-ai.sh`: readiness checks by default; explicit interactive installation guidance with `--install`.

## Setup and checks

```bash
scripts/setup-ai.sh
scripts/check-template.sh
```

`setup-ai.sh` never installs in its default mode or edits agent configuration. It distinguishes local checks from plugin checks that require agent-side verification. `--install` is explicit and interactive; `--setup-labels` only adds missing default labels after confirming the GitHub remote and authentication.

`setup-ai.sh` exits `0` with `READY` only when its checks pass; exits `2` with `NEEDS CONFIRMATION` when agent-side verification remains; and exits `1` with `BLOCKED` when required GitHub or repository prerequisites are missing. A ready result only describes what the script checks; it cannot prove plugin activation inside every vendor application.

`scripts/check-template.sh` validates the published template structure, including its single Claude entry point and Cursor rule. It intentionally rejects a root `CLAUDE.md` in the template source. For existing repositories, run `setup-ai.sh` instead and preserve the existing Claude entry point.

### Agent routing smoke test

After installing skills, test in each agent used by the team:

1. Give the agent a substantial epic card. It should create/reuse one parent issue, return the URL, and stop for your approval comment.
2. Add an approval comment. The agent may then create useful vertical-slice child issues; it should not repeat settled requirements discovery.
3. Give it a clear small issue. It should skip unnecessary specification and ticket decomposition.
4. Give it a bug issue. It should reproduce and investigate root cause before modifying production code.

Repeat with OpenCode, Claude Code, Codex, and Cursor as applicable. Record any agent that repeats discovery, duplicates the parent issue, or starts work before approval, then adjust the shared routing policy or the smallest relevant agent entry point.

## Upstream systems

- [Matt Pocock Skills](https://github.com/mattpocock/skills)
- [Superpowers](https://github.com/obra/superpowers)

Install each in the agents you actually use and keep them updated using their documented mechanisms. The repository stores routing policy and project context, not copied forks of those skill libraries.
