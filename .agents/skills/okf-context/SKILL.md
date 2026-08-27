---
name: okf-context
description: Consult the tofu-diff OKF bundle before planning, diagnosing, reviewing, or modifying its Go CLI, OpenTofu JSON or binary plan decoding, plain-text renderer, Bubble Tea TUI, requirements, or SPDD workflow; start at okf/index.md and follow only the relevant concepts and sources.
---

# OKF repository context

The `okf/` bundle is the repository's progressive-disclosure knowledge map. Use
it to find the relevant system boundaries and authoritative sources without
loading unrelated documentation.

Before substantive repository work, start at `okf/index.md`, follow the narrowest
relevant area index, and read the concepts that cover the task. A concept's
`sources` identify the code or documentation to inspect when current behavior
matters.

OKF trust and lifecycle metadata is meaningful:

- Source code and current canonical documentation outrank a conflicting OKF
  concept.
- Treat `draft`, unverified, or stale concepts as navigation and hypotheses
  until their sources confirm them.
- If the sources reveal drift, continue from the authoritative source and use
  `okf-maintainer` to correct the bundle as part of the same change when the
  task includes repository edits.

Reading OKF does not broaden the requested task or authorize unrelated changes.
