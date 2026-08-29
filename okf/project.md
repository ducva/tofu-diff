---
type: Software Repository
title: tofu-diff repository
description: Repository map for the Go terminal viewer that turns OpenTofu plans into interactive or plain-text diffs.
resource: ../README.md
tags: [tofu-diff, opentofu, go, terminal]
status: draft
stale_after: 2026-09-26T12:08:42+07:00
generated: { by: Codex, at: 2026-08-27T13:15:55+07:00 }
sources:
  - id: readme
    resource: ../README.md
    title: Project README
  - id: module
    resource: ../go.mod
    title: Go module definition
---

# tofu-diff repository

tofu-diff is a terminal-oriented OpenTofu plan viewer. It accepts JSON plans and native binary plan files, then presents resource changes through an interactive terminal UI or pipe-friendly text output.[^readme]

The repository is a single Go module, `github.com/ducva/tofu-diff`, with Bubble Tea and Lip Gloss providing the interactive interface and protobuf/msgpack packages supporting native plan decoding.[^module]

## Workspace layout

| Path | Responsibility |
| --- | --- |
| `cmd/tofu-diff/` | Canonical executable entry point. |
| `main.go` | Compatibility entry point preserving module-root `go install`. |
| `internal/cli/` | CLI argument handling, source selection, TTY dispatch, and top-level error handling. |
| `internal/plan/domain/` | Presentation-neutral plan model, validated actions, invariants, and attribute diff calculation. |
| `internal/plan/application/` | Inspect-plan use case plus decoder and presenter ports. |
| `internal/plan/ingestion/` | JSON/native format detection, decoding, and translation into the domain model. |
| `internal/presentation/text/` | Non-interactive output for pipes and redirected stdout. |
| `internal/presentation/tui/` | Bubble Tea state, keyboard interaction, filtering, layout, and unified diff presentation. |
| `internal/presentation/valuefmt/` | Shared compact and sensitivity-aware value formatting. |
| `.github/workflows/` | Continuous delivery workflows for testing, packaging, and publishing releases. |
| `okf/` | Progressive-disclosure repository knowledge and maintenance history. |

## Architectural map

Start with [runtime flow](/architecture/runtime-flow.md) to understand how the executable chooses an input and presentation mode. Continue to [plan model and loading](/plan-processing/plan-model-and-loading.md) for decoding and diff semantics, then [terminal interfaces](/interfaces/terminal-interfaces.md) for output behavior. For publishing behavior, read [release automation](/conventions/release-automation.md).

[^readme]: Source: [Project README](../README.md).
[^module]: Source: [Go module definition](../go.mod).
