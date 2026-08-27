---
type: Domain Model
title: Plan model and loading
description: Shared plan structures and decoding rules for OpenTofu JSON and ZIP-wrapped protobuf plans.
resource: ../../plan/plan.go
tags: [opentofu, plans, protobuf, diff]
status: draft
stale_after: 2026-09-26T12:08:42+07:00
generated: { by: Codex, at: 2026-08-27T13:15:55+07:00 }
sources:
  - id: plan
    resource: ../../plan/plan.go
    title: Plan domain and loaders
  - id: readme
    resource: ../../README.md
    title: Supported plan formats
---

# Plan model and loading

## Domain model

`PlanFile` contains OpenTofu's format version and a list of `ResourceChange` values. Each resource has identity fields plus a `Change` holding action names, before/after JSON attributes, unknown-after markers, and sensitivity maps. `AttributeDiff` is the presentation-neutral comparison shape used by both interfaces.[^plan]

`Change.NormalizedAction` maps the source action list into create, update, delete, replace, or no-op. Two actions are treated as a replacement, and missing or unrecognized actions fall back to no-op.[^plan]

## Loading formats

`Load` opens a named file and delegates to `LoadReader`. The reader consumes all bytes and checks for ZIP magic before attempting JSON, allowing the same entry path to support `.json` exports and OpenTofu native binary plans.[^plan][^readme]

JSON input is unmarshaled directly into `PlanFile`. An unfamiliar `format_version` emits a warning without rejecting the plan. Input that is neither ZIP data nor JSON beginning with an object receives an actionable hint explaining the `tofu plan` and `tofu show -json` workflow.[^plan]

## Native plan decoding

Native plans are ZIP archives containing a `tfplan` protobuf payload. The decoder walks protobuf fields with `protowire`, derives resource identity from the address, decodes dynamic values from msgpack, and sorts decoded resources by address for deterministic presentation.[^plan]

Msgpack extension types `0` and `12` are registered as unknown-value sentinels. Unknowns are detected recursively, recorded at their top-level attribute, and replaced with `null` only for JSON conversion. Sensitive protobuf paths are reduced to top-level attribute maps used by formatting code.[^plan]

## Attribute diff contract

`DiffAttributes` considers keys present in before, after, or unknown-after maps. It omits byte-identical attributes, emits `(known after apply)` for unknown results, applies sensitivity masking through `FormatValue`, and sorts results by attribute name. `FormatValue` truncates compact display strings to 120 characters; the TUI can use raw fields when it needs full multi-line values.[^plan]

Changes to OpenTofu schema interpretation or binary field numbers are high risk: verify them with representative files from `data/` and keep presentation concerns outside this package.

[^plan]: Source: [Plan domain and loaders](../../plan/plan.go).
[^readme]: Source: [Supported plan formats](../../README.md).
