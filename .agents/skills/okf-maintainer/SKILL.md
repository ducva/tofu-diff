---
name: okf-maintainer
description: Keep the tofu-diff repository's okf/ knowledge bundle aligned with durable changes to its Go architecture, OpenTofu plan model and decoding, terminal interfaces, requirements workflow, testing, or code-placement conventions, or when explicitly asked to add, refresh, validate, or repair OKF concepts.
---

# Maintain the OKF bundle

Update `okf/` in the same change when repository work changes knowledge that a
future maintainer or agent should rely on. Purely mechanical edits that preserve
behavior, ownership, and operating practice do not need an OKF rewrite.

## Maintenance contract

- Start from `okf/index.md` and update the smallest affected concept set.
- Follow and refresh `sources` against authoritative repository files. If a
  source and a concept disagree, correct the concept; do not rewrite source code
  merely to match OKF.
- Add a concept only for a distinct, durable unit of knowledge. Keep area
  `index.md` files useful for progressive disclosure.
- Use bundle-relative Markdown links for concept relationships and valid
  repository-relative paths for source material.
- Record meaningful bundle changes in `okf/log.md` under an ISO `YYYY-MM-DD`
  heading, newest first.

For an agent-authored content change, update `generated.by` and `generated.at`,
set `status: draft` until review, and choose a realistic `stale_after`. Never add
or refresh `verified` without an actual human or automated verification event.
Preserve older verification events when they are part of the record; a newer
`generated.at` and `draft` status signal that re-review is required.

## OKF v0.2 invariants

- Every concept Markdown file has parseable YAML frontmatter with a non-empty
  `type`.
- Only the bundle-root `index.md` may have frontmatter, containing
  `okf_version: "0.2"`; nested indexes have no frontmatter.
- `log.md` has no frontmatter and uses ISO date headings.
- Provenance entries have a `resource`; claim footnotes match `sources[].id`.
- Trust and lifecycle timestamps use ISO 8601 with an explicit UTC offset.
- Internal links and repository-relative source paths resolve.

Run `npm run okf:validate` after editing the bundle. The command validates the
format, metadata families, reserved files, source paths, links, and footnote
provenance; semantic accuracy still requires checking the cited sources.
