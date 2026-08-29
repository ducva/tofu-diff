---
type: System Architecture
title: Runtime flow
description: How tofu-diff moves a file or stdin plan through the shared plan domain into TUI or plain-text output.
resource: ../../main.go
tags: [architecture, cli, data-flow, dependencies]
status: draft
stale_after: 2026-09-26T12:08:42+07:00
generated: { by: Codex, at: 2026-08-27T13:15:55+07:00 }
sources:
  - id: entrypoint
    resource: ../../internal/cli/run.go
    title: CLI entry point
  - id: plan
    resource: ../../internal/plan/domain/plan.go
    title: Plan domain and loaders
  - id: ingestion
    resource: ../../internal/plan/ingestion/loader.go
    title: Plan ingestion adapter
  - id: renderer
    resource: ../../internal/presentation/text/presenter.go
    title: Plain-text renderer
  - id: tui
    resource: ../../internal/presentation/tui/model.go
    title: Interactive TUI
---

# Runtime flow

## Composition boundary

`internal/cli/run.go` is the composition boundary. It owns command-line parsing, chooses a path argument or piped stdin, selects a presenter, and translates top-level failures into stderr messages and exit codes.[^entrypoint]

The dependency direction is intentionally inward toward the plan domain:

```text
cli ─> application ─> domain
 │          │
 │          └─> ingestion ─> domain
 └─> text/tui presenters ─> domain
```

The domain package imports no ingestion or presentation packages. The application layer defines decoder and presenter ports, and both interfaces consume the same `domain.Plan`, `domain.ResourceChange`, and `domain.DiffAttributes` behavior.[^plan][^ingestion][^renderer][^tui]

## Input-to-output sequence

1. The CLI accepts a plan path when one is supplied; otherwise it accepts stdin only when stdin is a pipe. Missing input is an error.[^entrypoint]
2. The `InspectPlan` application service calls the ingestion decoder through its input port.[^ingestion]
3. The ingestion adapter detects JSON or native binary input, translates it, and validates the domain plan.[^ingestion][^plan]
4. If stdout is a terminal, the CLI selects the Bubble Tea presenter; otherwise it selects the pipe-friendly text presenter.[^entrypoint][^renderer][^tui]
5. Decoder and presentation errors are reported at the CLI boundary. A deferred recovery guard converts unexpected panics into exit code `1`.[^entrypoint]

## Change-placement rules

* Put normalized semantics, validation, and reusable attribute comparison in `internal/plan/domain/`.
* Put JSON, ZIP, protobuf, and msgpack translation in `internal/plan/ingestion/`.
* Put use-case coordination and I/O ports in `internal/plan/application/`.
* Put non-interactive formatting in `internal/presentation/text/` and interactive behavior in `internal/presentation/tui/`.
* Keep `internal/cli/` focused on process wiring rather than domain or rendering algorithms.

[^entrypoint]: Source: [CLI adapter](../../internal/cli/run.go).
[^plan]: Source: [Plan domain](../../internal/plan/domain/plan.go).
[^ingestion]: Source: [Plan ingestion adapter](../../internal/plan/ingestion/loader.go).
[^renderer]: Source: [Plain-text presenter](../../internal/presentation/text/presenter.go).
[^tui]: Source: [Interactive TUI](../../internal/presentation/tui/model.go).
