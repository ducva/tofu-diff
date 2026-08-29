---
type: Domain Model
title: Plan model and loading
description: Shared plan structures and decoding rules for OpenTofu JSON and ZIP-wrapped protobuf plans.
resource: ../../internal/plan/domain/plan.go
tags: [opentofu, plans, protobuf, diff]
status: draft
stale_after: 2026-09-26T12:08:42+07:00
generated: { by: Codex, at: 2026-08-27T13:15:55+07:00 }
sources:
  - id: plan
    resource: ../../internal/plan/domain/plan.go
    title: Plan domain
  - id: ingestion
    resource: ../../internal/plan/ingestion/loader.go
    title: Plan ingestion
  - id: readme
    resource: ../../README.md
    title: Supported plan formats
---

# Plan model and loading

## Domain model

`Plan` contains OpenTofu's format version and a list of `ResourceChange` values. Each resource has identity fields plus a `Change` holding action names, before/after JSON attributes, unknown-after markers, and sensitivity maps. `AttributeDiff` is the presentation-neutral comparison shape used by both interfaces.[^plan]

`NormalizeAction` maps supported source action sequences into create, update, delete, replace, or no-op. Only OpenTofu's two ordered replacement pairs are accepted; unsupported actions fail domain validation.[^plan]

## Loading formats

The ingestion adapter consumes the selected source and checks for ZIP magic before attempting JSON, allowing the same application port to support `.json` exports and OpenTofu native binary plans.[^ingestion][^readme]

JSON input is unmarshaled into ingestion DTOs, translated to `domain.Plan`, and validated. Version diagnostics are emitted by the application layer rather than the decoder. Invalid format errors retain the actionable `tofu plan` and `tofu show -json` hint.[^ingestion]

## Native plan decoding

Native plans are ZIP archives containing a `tfplan` protobuf payload. The decoder walks protobuf fields with `protowire`, derives resource identity from the address, decodes dynamic values from msgpack, and sorts decoded resources by address for deterministic presentation.[^plan]

Msgpack extension types `0` and `12` are registered as unknown-value sentinels. Unknowns are detected recursively, recorded at their top-level attribute, and replaced with `null` only for JSON conversion. Sensitive protobuf paths are reduced to top-level attribute maps used by formatting code.[^plan]

## Attribute diff contract

`DiffAttributes` considers keys present in before, after, or unknown-after maps. It omits byte-identical attributes, emits `(known after apply)` for unknown results, applies sensitivity masking through `FormatValue`, and sorts results by attribute name. `FormatValue` truncates compact display strings to 120 characters; the TUI can use raw fields when it needs full multi-line values.[^plan]

Changes to OpenTofu schema interpretation or binary field numbers are high risk: verify them with representative files from `data/` and keep presentation concerns outside this package.

[^plan]: Source: [Plan domain](../../internal/plan/domain/plan.go).
[^ingestion]: Source: [Plan ingestion](../../internal/plan/ingestion/loader.go).
[^readme]: Source: [Supported plan formats](../../README.md).
