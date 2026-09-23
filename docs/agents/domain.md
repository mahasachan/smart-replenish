# Domain documentation

Use GitHub Issues for change-specific requirements. Use `CONTEXT.md` for a stable project glossary and domain rules when the project has accumulated terms that need to be shared across issues. Create or expand it when discovery resolves real domain ambiguity; do not invent a glossary upfront.

Read `docs/architecture/overview.md` and the ADRs relevant to the work. For a large monorepo, a root `CONTEXT-MAP.md` may point to context documents near each domain; add this only when a single glossary no longer works.

Use established domain terms in issue text, code, tests, and reviews. If a requested term conflicts with the glossary, raise the discrepancy and resolve it during discovery.
