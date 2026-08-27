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
    resource: ../../main.go
    title: CLI entry point
  - id: plan
    resource: ../../plan/plan.go
    title: Plan domain and loaders
  - id: renderer
    resource: ../../render/render.go
    title: Plain-text renderer
  - id: tui
    resource: ../../tui/tui.go
    title: Interactive TUI
---

# Runtime flow

## Composition boundary

`main.go` is the composition root. It owns command-line parsing, chooses a path argument or piped stdin, delegates decoding to `plan`, and translates top-level failures into stderr messages and exit codes.[^entrypoint]

The dependency direction is intentionally inward toward the plan domain:

```text
main
 ├─> plan
 ├─> render ─> plan
 └─> tui ────> plan
```

The `plan` package does not import either presentation package. Both output modes consume the same `plan.PlanFile`, `plan.ResourceChange`, and `plan.DiffAttributes` behavior.[^plan][^renderer][^tui]

## Input-to-output sequence

1. The CLI accepts a plan path when one is supplied; otherwise it accepts stdin only when stdin is a pipe. Missing input is an error.[^entrypoint]
2. `plan.Load` opens a path, while `plan.LoadReader` reads either source and selects JSON or native binary decoding.[^plan]
3. If stdout is a terminal, `main` constructs the Bubble Tea model and runs it in the alternate screen.[^entrypoint]
4. If stdout is redirected or piped, `main` invokes the plain-text renderer so output remains non-interactive and pipe-friendly.[^entrypoint][^renderer]
5. Loader, TUI, and rendering errors are reported at the entry point. A deferred recovery guard converts unexpected panics while processing a plan into exit code `1`.[^entrypoint]

## Change-placement rules

* Put schema, decoding, action normalization, and reusable attribute comparison in `plan/`.
* Put non-interactive formatting in `render/`; it should write to its injected `io.Writer`.
* Put interactive state and terminal event handling in `tui/`.
* Keep `main.go` focused on wiring and process-level concerns rather than domain or presentation algorithms.

[^entrypoint]: Source: [CLI entry point](../../main.go).
[^plan]: Source: [Plan domain and loaders](../../plan/plan.go).
[^renderer]: Source: [Plain-text renderer](../../render/render.go).
[^tui]: Source: [Interactive TUI](../../tui/tui.go).
