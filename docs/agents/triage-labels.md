# Triage label mapping

Use these labels with Matt Pocock Skills' canonical triage roles. The issue templates reference `bug`, `enhancement`, and `needs-triage`; create the complete default set in a new repository before relying on those templates. Setup may create missing defaults only when explicitly requested and must not change existing labels.

| Canonical role | GitHub label | Meaning |
|---|---|---|
| `needs-triage` | `needs-triage` | Maintainer evaluation needed |
| `needs-info` | `needs-info` | Waiting for reporter details |
| `ready-for-agent` | `ready-for-agent` | Requirements are sufficiently clear for agent work |
| `ready-for-human` | `ready-for-human` | Requires human implementation or judgment |
| `wontfix` | `wontfix` | Request will not be actioned |

The default issue category labels are `bug` and `enhancement`. Existing projects may map canonical roles to their established category/state labels; update this mapping and issue template frontmatter together. Applying `ready-for-agent` does not record maintainer approval; approval is an explicit comment on the issue.

Use at most one state label at a time and follow the installed triage skill's transition rules. Projects may change the right-hand label names to match existing vocabulary.
