# Specification: Add MCP Support to README Features Section

**Status:** Draft
**Version:** 1.0
**Created:** 2026-03-12
**GitHub Issue:** https://github.com/jrswab/axe/issues/25
**Scope:** Documentation-only changes to `README.md`

---

## 1. Context & Constraints

### Associated Milestone

GitHub Issue #25 — "docs: add MCP support to README features section"

MCP support is fully implemented in the codebase (`internal/mcpclient/`,
`internal/envinterp/`, agent config `[[mcp_servers]]` TOML syntax, integration in
`cmd/run.go`). The README does not mention MCP anywhere. Users discovering Axe
miss this capability entirely.

### Relevant Codebase State

- **README.md** is 417 lines. The Features section (lines 23–35) lists 11 bullet
  points. MCP is absent.
- The Agent Configuration TOML example (lines 293–317) shows `tools`,
  `sub_agents`, `[sub_agents_config]`, `[memory]`, and `[params]` but does not
  include `[[mcp_servers]]`.
- The Tools section (lines 321–357) documents built-in tools, path security, and
  parallel execution. There is no mention of MCP tools.
- The "Parallel Execution" subsection (lines 353–357) is the last subsection
  under "## Tools". The next section is "## Skills" (line 359).
- `go.mod` lists 4 direct dependencies: `cobra`, `toml`, `mcp-go-sdk`, and
  `golang.org/x/net`. The README currently claims "two direct dependencies
  (cobra, toml)" which is outdated.
- MCP implementation details are fully specified in
  `docs/plans/027_mcp_tool_support_spec.md`. The README documentation must be
  consistent with that spec.

### Decisions Already Made

1. **Three targeted edits to README.md.** No other files are changed. No code
   changes.
2. **MCP bullet goes between "Built-in tools" and "Minimal dependencies"** in the
   Features list — this groups tool-related features together.
3. **The `[[mcp_servers]]` block is added to the existing Agent Configuration
   TOML example** — users see it alongside other config fields in one place.
4. **A new `### MCP Tools` subsection is added under `## Tools`** — placed after
   "### Parallel Execution" and before "## Skills". This keeps all tool
   documentation together.
5. **The dependency count in the "Minimal dependencies" bullet is updated** to
   reflect the actual `go.mod` state.

### Constraints

- **README is the first thing new users see.** Brevity matters. Each addition
  must earn its space.
- **No duplication of the full MCP spec.** The README provides enough to
  understand the capability and get started. Users who need edge cases or
  internals read `docs/plans/027_mcp_tool_support_spec.md`.
- **Consistency with existing README style.** Feature bullets use
  `**Bold label** — description` format. Subsections under Tools use `###`
  headings. TOML examples use fenced code blocks with `toml` language tag.
- **Factual accuracy.** Only `"sse"` and `"streamable-http"` transports are
  supported. Axe is an MCP client only. `${VAR}` is the only interpolation
  syntax. Built-in tools take precedence on name collision. MCP tools are
  controlled by `[[mcp_servers]]`, not the `tools` field.

---

## 2. Requirements

### Requirement 1: Add MCP Feature Bullet

Add a bullet point to the Features section (between the "Built-in tools" bullet
and the "Minimal dependencies" bullet) that communicates:

- Axe supports connecting to external MCP servers
- MCP servers provide additional tools to agents
- Two transport types are available: SSE and streamable-HTTP

The bullet must follow the existing format: `**Bold label** — description`.

### Requirement 2: Update Dependency Count

The "Minimal dependencies" bullet currently reads:

> two direct dependencies (cobra, toml); all LLM calls use the standard library

This is factually incorrect. `go.mod` has 4 direct `require` entries:
`github.com/BurntSushi/toml`, `github.com/modelcontextprotocol/go-sdk`,
`github.com/spf13/cobra`, and `golang.org/x/net`.

Update the bullet to accurately reflect the current dependency count and list.
The claim that "all LLM calls use the standard library" remains true and must be
preserved.

### Requirement 3: Add MCP Servers to Agent Configuration Example

Add a `[[mcp_servers]]` block to the existing Agent Configuration TOML example.
The block must show:

- The `name` field (string, required)
- The `url` field (string, required)
- The `transport` field (string, required — one of `"sse"` or
  `"streamable-http"`)
- The `headers` field (map, optional — demonstrating `${ENV_VAR}` interpolation)

Placement: after the `[memory]` block and before the `[params]` block. This
matches the field ordering in `internal/agent/agent.go` where `MCPServers` sits
between `SubAgentsConf`/`Memory` and `Params`.

### Requirement 4: Add MCP Tools Subsection

Add a new `### MCP Tools` subsection under the `## Tools` section. Placement:
after the "### Parallel Execution" subsection and before the `## Skills` section.

The subsection must communicate:

1. **What it is:** Agents can use tools from external MCP servers.
2. **How to configure it:** Declare servers in agent TOML with
   `[[mcp_servers]]`.
3. **A minimal TOML example** showing one MCP server declaration with all four
   fields (`name`, `url`, `transport`, `headers`).
4. **Startup behavior:** At startup, axe connects to each declared server,
   discovers available tools via `tools/list`, and makes them available to the
   LLM alongside built-in tools.
5. **Field reference table** with columns: Field, Required, Description. Rows:
   - `name` — Yes — Human-readable identifier for the server
   - `url` — Yes — MCP server endpoint URL
   - `transport` — Yes — `"sse"` or `"streamable-http"`
   - `headers` — No — HTTP headers; values support `${ENV_VAR}` interpolation
6. **Separation from `tools` field:** MCP tools are controlled entirely by
   `[[mcp_servers]]` — they are not listed in the `tools` field.
7. **Name collision rule:** If an MCP tool has the same name as an enabled
   built-in tool, the built-in takes precedence.

The subsection must NOT include:

- Internal implementation details (router, client, SDK)
- Edge cases covered in the MCP spec (JSON Schema flattening, sub-agent
  isolation, etc.)
- Transport protocol details beyond naming the two supported types
- Retry behavior, timeout inheritance, or connection failure semantics

### Requirement 5: No Other Changes

No files other than `README.md` are modified. No code changes. No new files
created (other than this spec and its eventual implementation doc).

---

## 3. Edge Cases

### 3.1 Dependency Count Accuracy

The dependency count must match `go.mod` at the time of implementation. If
dependencies have changed since this spec was written, the implementer must check
`go.mod` and use the actual count.

### 3.2 TOML Example Validity

The `[[mcp_servers]]` block added to the Agent Configuration example must be
valid TOML that axe can parse. Specifically:

- `[[mcp_servers]]` is the correct TOML array-of-tables syntax.
- `headers` uses inline table syntax: `{ Key = "Value" }`.
- `${ENV_VAR}` inside a TOML string is literal text (not TOML interpolation) —
  axe's `envinterp` package handles expansion at runtime.

### 3.3 Section Ordering

The MCP Tools subsection must appear:
- After "### Parallel Execution" (currently the last subsection under ## Tools)
- Before "## Skills" (the next top-level section)

If other subsections have been added to ## Tools since this spec was written, MCP
Tools goes after all existing subsections and before ## Skills.

### 3.4 Link to MCP

The subsection should include a link to the MCP website
(`https://modelcontextprotocol.io/`) so users unfamiliar with MCP can learn more.
This is an inline link, not a separate references section.

---

## 4. Acceptance Criteria

| Criterion | Verification |
|-----------|-------------|
| Features section includes an MCP bullet | Visual inspection of README.md |
| MCP bullet is between "Built-in tools" and "Minimal dependencies" | Visual inspection of line ordering |
| Dependency count in "Minimal dependencies" matches `go.mod` | Compare bullet text to `go.mod` direct requires |
| Agent Configuration TOML example includes `[[mcp_servers]]` | Visual inspection of the fenced code block |
| `[[mcp_servers]]` block shows all four fields | Visual inspection |
| `[[mcp_servers]]` block is between `[memory]` and `[params]` | Visual inspection of line ordering |
| MCP Tools subsection exists under ## Tools | Visual inspection |
| MCP Tools subsection is after ### Parallel Execution | Visual inspection of line ordering |
| MCP Tools subsection is before ## Skills | Visual inspection of line ordering |
| MCP Tools subsection includes a TOML example | Visual inspection |
| MCP Tools subsection includes a field reference table | Visual inspection |
| MCP Tools subsection states built-in precedence on collision | Text search for "precedence" or equivalent |
| MCP Tools subsection states MCP tools are not in `tools` field | Text search |
| MCP Tools subsection links to modelcontextprotocol.io | Text search for URL |
| No other files are modified | `git diff --stat` shows only README.md |
| README renders correctly as Markdown | Visual inspection or Markdown preview |

---

## 5. Out of Scope

- Changes to any file other than `README.md`
- Adding MCP examples to the `examples/` directory
- Adding a dedicated MCP configuration guide or tutorial
- Documenting MCP edge cases (sub-agent isolation, JSON Schema flattening, etc.)
- Documenting `--dry-run` MCP output format
- Documenting verbose MCP logging
