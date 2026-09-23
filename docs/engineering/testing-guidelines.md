# Testing guidelines

- Follow existing test layout, tools, and commands. Identify the highest useful test seam before adding a new one.
- Prefer tests of observable behavior and contracts over tests coupled to private implementation details.
- For bug fixes, write a regression test that demonstrates the reported failure before applying the fix when feasible.
- For behavior changes, cover the acceptance criteria and relevant boundary, failure, authorization, and data-integrity cases.
- Use unit tests for isolated rules, integration tests for meaningful component boundaries, and end-to-end tests for critical user journeys. Do not duplicate the same assertion at every layer without value.
- Keep tests deterministic: control time, randomness, external services, and concurrency as appropriate.
- A test passing is evidence for the cases it exercises, not proof of all production conditions. Report skipped checks and environmental limits honestly.
- Run focused tests while iterating, then the repository's relevant full verification commands before PR.
