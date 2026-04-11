# Implementation Plan - Microkernel Language Alignment

This plan aligns the existing documentation in `docs/plans` with the microkernel and plugin architecture described in `@docs/design/Golang Microkernel and Plugin Implementation.md`.

## User Alignment Confirmation
- Use "Core System" for the orchestrator/kernel.
- Use "Shared Protocol" for `pkg/protocol`.
- Use "Plugin Registry" for the tool registry.
- Use "Plugin Components" for tools (especially Wasm tools).
- Align milestone descriptions to reflect a deliberate microkernel design.

## Proposed Changes

### 1. Update `docs/plans/000_milestones.md`
- Rename M3 from "Single Agent Run" to "Core System Orchestration".
- Update M3 description to mention the "Core Orchestration Loop".
- Update M4 to mention "Provider Plugin Interface".
- Rename "Future" section to "M9+ Extensibility".
- Update "Plugin system for custom tools" to "Wasm Plugin Microkernel Implementation".

### 2. Update `docs/plans/000_tool_call_milestones.md`
- Rename M2 from "Tool Registry" to "Plugin Registry".
- Update M2 description to use "Registry Pattern" terminology.
- Update M3-M7 to refer to "Built-in Plugin Components".

### 3. Update `docs/plans/003_single_agent_run_spec.md`
- Rename "Purpose" section to reflect "Core System Orchestration".
- Rename "internal/provider" requirements to "Shared Protocol (pkg/protocol)".
- Update terminology: "Agent loop" -> "Core conversation loop".

### 4. Update `docs/plans/013_tool_registry_spec.md`
- Rename from "Tool Registry" to "Plugin Registry".
- Update description to mention the "Microkernel Registry Pattern".
- Align `ToolEntry` language with plugin implementation details.

## Verification
- Review updated files for consistency with the design document.
- Ensure no functional changes are implied, only terminology alignment.
