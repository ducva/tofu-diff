---
type: User Interface Architecture
title: Terminal interfaces
description: Responsibilities and behavior of tofu-diff's pipe-friendly renderer and interactive Bubble Tea TUI.
resource: ../../tui/tui.go
tags: [tui, rendering, bubble-tea, terminal]
status: draft
stale_after: 2026-09-26T12:08:42+07:00
generated: { by: Codex, at: 2026-08-27T13:15:55+07:00 }
sources:
  - id: tui
    resource: ../../tui/tui.go
    title: Interactive TUI
  - id: renderer
    resource: ../../render/render.go
    title: Plain-text renderer
  - id: readme
    resource: ../../README.md
    title: User-facing interface documentation
---

# Terminal interfaces

## Shared boundary

Both interfaces receive the already-loaded `plan.PlanFile` and omit no-op resources from meaningful output. They should share classification and attribute comparison through `plan` rather than implementing different domain rules.[^tui][^renderer]

## Plain-text renderer

`render.PlanRenderer` writes through an injected `io.Writer` wrapped in a buffered writer. It emits one block per changed resource, uses stable action labels, sorts create/delete attributes by key, and uses `plan.DiffAttributes` for update and replacement output. If nothing is printed, it emits the explicit up-to-date message.[^renderer]

This path is designed for redirected stdout, files, and shell pipelines. Avoid adding terminal control sequences or interactive assumptions to it.

## Interactive TUI

The Bubble Tea `Model` owns the filtered resource indices, expansion state, cursor, focused panel, search input, action filters, viewports, dimensions, action summary, and transient UI state. `Update` handles terminal resize and delegates keyboard input between search mode and normal mode; `View` composes the header, search/filter row, bordered panels, and footer.[^tui]

The left panel is the navigation surface. The right panel renders resource metadata and full attribute values. Values are converted to parallel plain and styled lines, JSON is pretty-printed, long content is wrapped, and an LCS calculation classifies unchanged, removed, and added lines for unified-diff rendering.[^tui]

User-facing controls and the automatic TTY/plain-text selection are summarized in the README. When controls change, update the footer and README together so discoverability matches behavior.[^readme]

## Change-placement rules

* Keep Bubble Tea state changes in `Update` or its handlers and visual composition in `View` helpers.
* Recompute viewport content after any state transition that changes selection, filters, dimensions, or displayed details.
* Preserve the plain-text path when expanding TUI-only behavior.
* Use ANSI-aware display widths for styled terminal content; byte length is not a reliable rendered width.

[^tui]: Source: [Interactive TUI](../../tui/tui.go).
[^renderer]: Source: [Plain-text renderer](../../render/render.go).
[^readme]: Source: [User-facing interface documentation](../../README.md).
