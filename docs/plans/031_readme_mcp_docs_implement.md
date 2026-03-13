# Implementation: Add MCP Support to README Features Section

**Spec:** `docs/plans/031_readme_mcp_docs_spec.md`

---

## 1. Context Summary

MCP support is fully implemented in the codebase but the README — the first thing
new users see — does not mention it anywhere. This implementation adds MCP
documentation to three locations in `README.md`: the Features bullet list, the
Agent Configuration TOML example, and a new MCP Tools subsection under Tools. No
code changes; only `README.md` is modified. The dependency count bullet is also
corrected to match the current `go.mod`.

---

## 2. Implementation Checklist

### Task 1: Add MCP feature bullet and update dependency count

**File:** `README.md` (lines 34–35, Features section)

- [x] After line 34 (`- **Built-in tools** — file operations ...`), insert a new bullet:
  ```
  - **MCP tool support** — connect to external MCP servers for additional tools via SSE or streamable-HTTP transport
  ```
- [x] Replace line 35:
  ```
  - **Minimal dependencies** — two direct dependencies (cobra, toml); all LLM calls use the standard library
  ```
  with:
  ```
  - **Minimal dependencies** — four direct dependencies (cobra, toml, mcp-go-sdk, x/net); all LLM calls use the standard library
  ```

**Verify:** The Features section now has 12 bullets. The MCP bullet sits between
"Built-in tools" and "Minimal dependencies". The dependency count says "four" and
lists all four.

---

### Task 2: Add `[[mcp_servers]]` to Agent Configuration TOML example

**File:** `README.md` (lines 309–313, inside the fenced TOML code block)

- [x] After line 312 (`max_entries = 100   # warn when exceeded`) and before line 314 (`[params]`), insert:
  ```toml

  [[mcp_servers]]
  name = "my-tools"
  url = "https://my-mcp-server.example.com/sse"
  transport = "sse"
  headers = { Authorization = "Bearer ${MY_TOKEN}" }

  ```
  (blank line before `[[mcp_servers]]`, blank line after `headers` line, so it
  visually separates from `[memory]` above and `[params]` below)

**Verify:** The TOML example now shows `[[mcp_servers]]` between `[memory]` and
`[params]`. All four fields (`name`, `url`, `transport`, `headers`) are present.
The `headers` value demonstrates `${ENV_VAR}` interpolation.

---

### Task 3: Add `### MCP Tools` subsection

**File:** `README.md` (after line 357, between "### Parallel Execution" and "## Skills")

- [x] After line 357 (`with \`parallel = false\` in \`[sub_agents_config]\`.`) and before line 359 (`## Skills`), insert the following subsection:

  ````markdown

  ### MCP Tools

  Agents can use tools from external [MCP](https://modelcontextprotocol.io/)
  servers. Declare servers in the agent TOML with `[[mcp_servers]]`:

  ```toml
  [[mcp_servers]]
  name = "my-tools"
  url = "https://my-mcp-server.example.com/sse"
  transport = "sse"
  headers = { Authorization = "Bearer ${MY_TOKEN}" }
  ```

  At startup, axe connects to each declared server, discovers available tools via
  `tools/list`, and makes them available to the LLM alongside built-in tools.

  | Field | Required | Description |
  |---|---|---|
  | `name` | Yes | Human-readable identifier for the server |
  | `url` | Yes | MCP server endpoint URL |
  | `transport` | Yes | `"sse"` or `"streamable-http"` |
  | `headers` | No | HTTP headers; values support `${ENV_VAR}` interpolation |

  MCP tools are controlled entirely by `[[mcp_servers]]` — they are not listed in
  the `tools` field. If an MCP tool has the same name as an enabled built-in tool,
  the built-in takes precedence.

  ````

**Verify:** The `### MCP Tools` subsection appears after "### Parallel Execution"
and before "## Skills". It contains a TOML example, a field reference table, the
`tools` field separation note, the name collision rule, and a link to
`https://modelcontextprotocol.io/`.

---

### Task 4: Final verification

- [x] Run `git diff --stat` and confirm only `README.md` is modified.
- [x] Visually confirm the Markdown renders correctly (no broken fences, tables, or links).
- [x] Confirm the dependency count in the "Minimal dependencies" bullet matches the direct `require` block in `go.mod`.
- [x] Confirm no other sections of the README were accidentally altered.
