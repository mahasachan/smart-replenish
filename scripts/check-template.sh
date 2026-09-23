#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
required_files=(
  AGENTS.md
  .claude/CLAUDE.md
  .cursor/rules/development-workflow.mdc
  .codex/README.md
  .agents/README.md
  README.md
  docs/engineering/development-workflow.md
  docs/engineering/architecture-principles.md
  docs/engineering/design-patterns.md
  docs/engineering/anti-patterns.md
  docs/engineering/coding-guidelines.md
  docs/engineering/testing-guidelines.md
  docs/engineering/error-handling.md
  docs/engineering/observability.md
  docs/architecture/overview.md
  docs/adr/README.md
  docs/adr/template.md
  docs/agents/issue-tracker.md
  docs/agents/triage-labels.md
  docs/agents/domain.md
  .github/ISSUE_TEMPLATE/feature.md
  .github/ISSUE_TEMPLATE/bug.md
  .github/ISSUE_TEMPLATE/task.md
  .github/pull_request_template.md
  .github/workflows/template-checks.yml
  scripts/setup-ai.sh
)

for file in "${required_files[@]}"; do
  if [[ ! -f "$root/$file" ]]; then
    printf 'Missing required template file: %s\n' "$file" >&2
    exit 1
  fi
done

for file in scripts/setup-ai.sh scripts/check-template.sh; do
  if [[ ! -x "$root/$file" ]]; then
    printf 'Script must be executable: %s\n' "$file" >&2
    exit 1
  fi
done

if [[ -f "$root/CLAUDE.md" ]]; then
  printf 'Remove root CLAUDE.md; this template uses only .claude/CLAUDE.md.\n' >&2
  exit 1
fi
if ! grep -Fxq '@../AGENTS.md' "$root/.claude/CLAUDE.md"; then
  printf '.claude/CLAUDE.md must import the shared AGENTS.md.\n' >&2
  exit 1
fi
if ! grep -Fq 'alwaysApply: true' "$root/.cursor/rules/development-workflow.mdc" || \
   ! grep -Fq 'AGENTS.md' "$root/.cursor/rules/development-workflow.mdc"; then
  printf 'Cursor workflow rule must be always-applied and point to AGENTS.md.\n' >&2
  exit 1
fi

if ! grep -Fq 'create or reuse exactly one GitHub parent issue' "$root/AGENTS.md"; then
  printf 'AGENTS.md must require one durable GitHub parent issue for epic intake.\n' >&2
  exit 1
fi
if ! grep -Fq 'it is not a substitute for the maintainer' "$root/docs/engineering/development-workflow.md"; then
  printf 'Development workflow must distinguish triage readiness from human approval.\n' >&2
  exit 1
fi
if ! grep -Fq 'Closes #' "$root/.github/pull_request_template.md"; then
  printf 'Pull request template must link the issue it completes.\n' >&2
  exit 1
fi

bash -n "$root/scripts/setup-ai.sh"
bash -n "$root/scripts/check-template.sh"
printf 'Template files present; shell scripts parse successfully.\n'
