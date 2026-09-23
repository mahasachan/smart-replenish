# Issue tracker: GitHub

GitHub Issues are the source of truth for requirements and work. Use `gh` for issue and pull request operations. Run `gh auth status` before writing. When the user asks to turn an epic card into work, create or reuse exactly one parent GitHub issue and return its URL. If GitHub access or repository context is missing, stop and report the blocker rather than substituting chat text or local files.

## Conventions

- Create/read/comment/edit issues with `gh issue create`, `gh issue view <number> --comments`, `gh issue comment`, and `gh issue edit`.
- Search for a matching open issue before creating a new parent with `gh issue list --state open --search "<keywords>" --json number,title,url`; refine an existing issue in place rather than publishing a duplicate spec issue.
- After creating/updating the parent issue, wait for an explicit maintainer comment approving the current scope and acceptance criteria. `ready-for-agent` is not approval. Material scope changes require renewed approval.
- Apply approved ticket labels using `gh issue edit <number> --add-label "<label>"`.
- Use GitHub child issues and native issue dependencies when available. If unavailable, include parent and blocker references in issue bodies.
- Link a PR to the issue it completes with `Closes #<number>`. A child ticket PR should not close its parent feature issue.
- Pull requests are implementation evidence, not an alternate request queue.
