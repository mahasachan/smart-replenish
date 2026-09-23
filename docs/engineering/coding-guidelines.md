# Coding guidelines

Project-specific lint, formatter, language, and naming rules take precedence; document them here as they become known.

- Read nearby code and use its established language, naming, error, and module conventions.
- Make the smallest change that completely satisfies the issue's acceptance criteria.
- Prefer clear names and straightforward control flow over compressed or generic code.
- Validate untrusted input at the boundary; keep validation consistent with the existing stack.
- Keep secrets, credentials, and sensitive production data out of source control, logs, test fixtures, and issue/PR content.
- Avoid adding a production dependency unless the requirement justifies it; explain meaningful operational or security consequences in the PR.
- Do not leave dead code, debug output, or temporary scaffolding introduced by the change.
