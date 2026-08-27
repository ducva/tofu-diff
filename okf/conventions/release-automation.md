---
type: Delivery Workflow
title: Release automation
description: How pushes to main become tested, cross-platform binary archives and GitHub releases.
resource: ../../.github/workflows/release.yml
tags: [github-actions, release, binaries, versioning]
status: draft
stale_after: 2026-11-27T12:17:53+07:00
generated: { by: Codex, at: 2026-08-27T12:17:53+07:00 }
sources:
  - id: workflow
    resource: ../../.github/workflows/release.yml
    title: GitHub release workflow
  - id: module
    resource: ../../go.mod
    title: Go module definition
---

# Release automation

Every push to `main` runs the Go test suite before publishing. A failed test or build prevents the release step from running.[^workflow]

The workflow uses the Go version declared by `go.mod`, disables CGO, and builds `tofu-diff` for Linux, macOS, and Windows on both AMD64 and ARM64. It packages each binary and publishes a SHA-256 checksum manifest with the archives.[^workflow][^module]

Each workflow run uses the tag `v0.0.<run-number>`. The release targets the pushed commit, uses GitHub-generated release notes, and is marked as the latest release. Re-running the same workflow run reuses its tag and replaces the uploaded assets rather than creating a duplicate release.[^workflow]

[^workflow]: Source: [GitHub release workflow](../../.github/workflows/release.yml).
[^module]: Source: [Go module definition](../../go.mod).
