# Specification: Tool Call M4 — `read_file` Tool

**Status:** Draft
**Version:** 1.0
**Created:** 2026-03-02
**Scope:** Read-only file reading tool with line numbering, offset/limit pagination, and binary detection

---

## 1. Purpose

Implement the `read_file` tool — the second concrete tool executor registered in the `Registry`. This tool reads file contents relative to the agent's workdir and returns them to the LLM with line-number prefixes. It is read-only with no side effects.

This milestone builds on the infrastructure established in M3:

- **`validatePath`** — reused as-is for path security (no modifications)
- **`RegisterAll`** — extended with one additional `r.Register(...)` call
- **`toolname.ReadFile`** — constant already declared in `internal/toolname/toolname.go`

No new infrastructure is introduced. This is a pure tool implementation milestone.

---

## 2. Design Decisions

The following decisions were made during planning and are binding for implementation:

1. **Output format is `N: content` (unpadded).** Each line is prefixed with its 1-indexed line number, a colon, and a space. Example: `1: package main`. No right-alignment padding. This matches the milestone example exactly.

2. **Default offset is 1 (first line). Default limit is 2000 lines.** When neither parameter is provided, the tool returns up to the first 2000 lines of the file.

3. **Binary files are rejected.** The first 512 bytes of the file are scanned for NUL (`\x00`) bytes. If any NUL byte is found, the tool returns an error result. The file is not returned.

4. **Offset past end-of-file is an error.** If the requested offset exceeds the total number of lines in the file, the tool returns an error result with a message indicating the offset exceeds the file length.

5. **Directory paths are rejected with a clear error.** If the resolved path is a directory (not a file), the tool returns an error with the message `path is a directory, not a file`. This provides a clear separation from `list_directory`.

6. **No file size hard limit.** The offset/limit parameters (default 2000 lines) keep output manageable. There is no byte-size cap on file reads.

7. **`call_agent` remains outside the registry.** Same as M2/M3.

8. **No new external dependencies.** Uses only Go stdlib.

---

## 3. Requirements

### 3.1 `read_file` Tool Definition

**Requirement 1.1:** Create a file `internal/tool/read_file.go`.

**Requirement 1.2:** Define an unexported function `readFileEntry() ToolEntry` that returns a `ToolEntry` with both `Definition` and `Execute` set to non-nil functions.

**Requirement 1.3:** The tool definition returned by the `Definition` function must have:
- `Name`: the value of `toolname.ReadFile` (i.e., `"read_file"`)
- `Description`: a clear description for the LLM explaining the tool reads file contents relative to the working directory and returns line-numbered output
- `Parameters`: three parameters:
  - `path`:
    - `Type`: `"string"`
    - `Description`: a description stating it is a relative path to the file to read
    - `Required`: `true`
  - `offset`:
    - `Type`: `"string"`
    - `Description`: a description stating it is the 1-indexed line number to start reading from, defaulting to 1
    - `Required`: `false`
  - `limit`:
    - `Type`: `"string"`
    - `Description`: a description stating it is the maximum number of lines to return, defaulting to 2000
    - `Required`: `false`

**Requirement 1.4:** The tool name in the definition must use `toolname.ReadFile`, not a hardcoded string.

**Requirement 1.5:** `ToolCall.Arguments` is `map[string]string`. The `offset` and `limit` parameters are strings that must be parsed to integers within the executor. This is consistent with the project's type system (see milestone notes: `ToolCall.Arguments is map[string]string`).

### 3.2 `read_file` Tool Executor

**Requirement 2.1:** The `Execute` function must have the signature: `func(ctx context.Context, call provider.ToolCall, ec ExecContext) provider.ToolResult`.

**Requirement 2.2:** Extract the `path` argument from `call.Arguments["path"]`.

**Requirement 2.3:** Call `validatePath(ec.Workdir, path)` to get the resolved absolute path. If `validatePath` returns an error, return a `ToolResult` with `CallID: call.ID`, `Content: err.Error()`, `IsError: true`.

**Requirement 2.4:** After path validation succeeds, check whether the resolved path is a directory using `os.Stat`. If `info.IsDir()` is true, return a `ToolResult` with `CallID: call.ID`, `Content: "path is a directory, not a file"`, `IsError: true`.

**Requirement 2.5:** If `os.Stat` returns an error (e.g., permission denied after symlink resolution), return a `ToolResult` with `CallID: call.ID`, `Content: err.Error()`, `IsError: true`.

**Requirement 2.6:** Read the file contents using `os.ReadFile(resolvedPath)`. If `os.ReadFile` returns an error, return a `ToolResult` with `CallID: call.ID`, `Content: err.Error()`, `IsError: true`.

**Requirement 2.7 (Binary Detection):** Examine the first 512 bytes of the file content (or the entire content if shorter than 512 bytes) for the presence of any NUL byte (`\x00`). If a NUL byte is found, return a `ToolResult` with `CallID: call.ID`, `Content: "binary file detected"`, `IsError: true`. Binary detection occurs before any line splitting or offset/limit processing.

**Requirement 2.8 (Offset Parsing):** Extract the `offset` argument from `call.Arguments["offset"]`. If the key is absent or the value is an empty string, default to `1`. If present and non-empty, parse it as a base-10 integer using `strconv.Atoi`. If parsing fails, return a `ToolResult` with `CallID: call.ID`, `Content` containing a descriptive parse error message, `IsError: true`. If the parsed value is less than 1, return a `ToolResult` with `CallID: call.ID`, `Content: "offset must be >= 1"`, `IsError: true`.

**Requirement 2.9 (Limit Parsing):** Extract the `limit` argument from `call.Arguments["limit"]`. If the key is absent or the value is an empty string, default to `2000`. If present and non-empty, parse it as a base-10 integer using `strconv.Atoi`. If parsing fails, return a `ToolResult` with `CallID: call.ID`, `Content` containing a descriptive parse error message, `IsError: true`. If the parsed value is less than 1, return a `ToolResult` with `CallID: call.ID`, `Content: "limit must be >= 1"`, `IsError: true`.

**Requirement 2.10 (Line Splitting):** Split the file content into lines using `strings.Split(string(content), "\n")`. This preserves empty trailing elements if the file ends with a newline (the last element will be an empty string).

**Requirement 2.11 (Trailing Newline Handling):** If the file content ends with `"\n"`, the final empty element from the split must be removed before computing the total line count. A file with content `"a\nb\n"` has 2 lines, not 3. A file with content `"a\nb"` also has 2 lines.

**Requirement 2.12 (Offset Bounds Check):** After computing the total line count (post trailing-newline removal), if the offset exceeds the total line count, return a `ToolResult` with `CallID: call.ID`, `Content` in the format `"offset %d exceeds file length of %d lines"` (where the first value is the requested offset and the second is the total line count), `IsError: true`.

**Requirement 2.13 (Line Selection):** Select lines from `offset-1` (converting to 0-indexed) up to `offset-1+limit` (capped at the total line count). This is a standard Go slice operation: `lines[offset-1 : min(offset-1+limit, len(lines))]`.

**Requirement 2.14 (Output Formatting):** Build the output string by prefixing each selected line with its 1-indexed line number, a colon, and a space. The line number corresponds to the line's position in the original file, not its position in the output. Format: `fmt.Sprintf("%d: %s", lineNumber, lineContent)`. Lines are joined with `"\n"`.

**Requirement 2.15:** Return a `ToolResult` with `CallID: call.ID`, `Content: the formatted output`, `IsError: false`.

**Requirement 2.16 (Empty File):** An empty file (zero bytes) must return a `ToolResult` with `Content: ""` (empty string) and `IsError: false`. No offset bounds error is raised for empty files when offset is 1 (the default).

### 3.3 Registration in Registry

**Requirement 3.1:** Add `r.Register(toolname.ReadFile, readFileEntry())` to the `RegisterAll` function in `internal/tool/registry.go`.

**Requirement 3.2:** This is the only change to `registry.go`. No call-site changes are needed in `cmd/run.go` or `internal/tool/tool.go`.

---

## 4. Project Structure

After completion, the following files will be added or modified:

```
axe/
├── internal/
│   └── tool/
│       ├── read_file.go               # NEW: Definition, Execute, helper entry func
│       ├── read_file_test.go           # NEW: tests
│       ├── registry.go                 # MODIFIED: add one line to RegisterAll
│       ├── registry_test.go            # UNCHANGED (existing tests still pass)
│       ├── list_directory.go           # UNCHANGED
│       ├── list_directory_test.go      # UNCHANGED
│       ├── path_validation.go          # UNCHANGED
│       ├── path_validation_test.go     # UNCHANGED
│       ├── tool.go                     # UNCHANGED
│       └── tool_test.go               # UNCHANGED
├── go.mod                              # UNCHANGED
├── go.sum                              # UNCHANGED
└── ...                                 # all other files UNCHANGED
```

---

## 5. Edge Cases

### 5.1 Path Validation (reused from M3, no changes)

The `validatePath` function handles all path security edge cases. Refer to `docs/plans/014_list_directory_spec.md` section 5.1 for the full table.

### 5.2 Binary Detection

| Scenario | Behavior |
|----------|----------|
| Text file (no NUL bytes) | Normal operation. Content is returned. |
| Binary file (NUL in first 512 bytes) | Error: `binary file detected`. Content not returned. |
| File with NUL byte at position 513+ but not in first 512 | Treated as text. Content is returned. This is an acceptable trade-off — the heuristic matches Git's behavior. |
| Empty file (0 bytes) | No bytes to scan. Not binary. Returns empty content. |
| File with exactly 512 bytes, last byte is NUL | Binary detected. Error returned. |
| File shorter than 512 bytes with NUL | Binary detected. Error returned. |

### 5.3 Offset and Limit

| Scenario | Input | Behavior |
|----------|-------|----------|
| No offset, no limit | `{}` | Default offset=1, limit=2000. Returns up to first 2000 lines. |
| Offset=1, limit=10 | `{"offset":"1","limit":"10"}` | Returns lines 1–10. |
| Offset=50, limit=20 | `{"offset":"50","limit":"20"}` | Returns lines 50–69 (if file is long enough). |
| Offset past EOF | `{"offset":"500"}` on 100-line file | Error: `offset 500 exceeds file length of 100 lines`. |
| Offset=1 on empty file | `{"offset":"1"}` on 0-byte file | Returns empty content (no error). Special case: empty files skip bounds check. |
| Offset=2 on empty file | `{"offset":"2"}` on 0-byte file | Error: `offset 2 exceeds file length of 0 lines`. |
| Limit exceeds remaining lines | `{"offset":"90","limit":"2000"}` on 100-line file | Returns lines 90–100. No error. Limit is a cap, not exact. |
| Non-numeric offset | `{"offset":"abc"}` | Error: parse failure message. |
| Non-numeric limit | `{"limit":"xyz"}` | Error: parse failure message. |
| Offset=0 | `{"offset":"0"}` | Error: `offset must be >= 1`. |
| Negative offset | `{"offset":"-5"}` | Error: `offset must be >= 1`. |
| Limit=0 | `{"limit":"0"}` | Error: `limit must be >= 1`. |
| Negative limit | `{"limit":"-10"}` | Error: `limit must be >= 1`. |

### 5.4 File Content

| Scenario | Behavior |
|----------|----------|
| File with trailing newline (`"a\nb\n"`) | 2 lines: `1: a` and `2: b`. Trailing empty element removed. |
| File without trailing newline (`"a\nb"`) | 2 lines: `1: a` and `2: b`. |
| File with only newlines (`"\n\n\n"`) | 3 lines (after trailing removal): `1: `, `2: `, `3: `. Each line is empty but exists. |
| Single line, no newline (`"hello"`) | 1 line: `1: hello`. |
| Single line with newline (`"hello\n"`) | 1 line: `1: hello`. Trailing element removed. |
| Empty file (0 bytes) | Returns empty string. No line-number output. |
| Very long lines (>10KB per line) | No truncation. The line is returned as-is. |
| Windows-style line endings (`\r\n`) | `strings.Split` on `"\n"` produces lines ending with `\r`. The `\r` is included in the output. No special CRLF handling. |

### 5.5 Argument Handling

| Scenario | Behavior |
|----------|----------|
| `path` argument present | Normal operation. |
| `path` argument missing from `call.Arguments` | `call.Arguments["path"]` returns `""` → `validatePath` returns error: `path is required`. |
| `offset` key absent from map | Default offset=1. |
| `offset` key present with empty string value | Default offset=1. |
| `limit` key absent from map | Default limit=2000. |
| `limit` key present with empty string value | Default limit=2000. |

### 5.6 Directory vs File

| Scenario | Behavior |
|----------|----------|
| Path points to a regular file | Normal operation. |
| Path points to a directory | Error: `path is a directory, not a file`. |
| Path points to a symlink to a file (within workdir) | `validatePath` resolves symlink. `os.Stat` on resolved path returns file info. Normal operation. |
| Path points to a symlink to a directory (within workdir) | `validatePath` resolves symlink. `os.Stat` reports directory. Error: `path is a directory, not a file`. |

---

## 6. Constraints

**Constraint 1:** No new external dependencies. `go.mod` must remain unchanged.

**Constraint 2:** No changes to `internal/provider/`, `internal/agent/`, or `internal/toolname/` packages.

**Constraint 3:** No changes to `cmd/run.go` or `internal/tool/tool.go`. The only modification outside the new file is adding one line to `RegisterAll` in `registry.go`.

**Constraint 4:** `call_agent` must NOT be registered in the registry. It remains special-cased.

**Constraint 5:** The tool must not write to the filesystem.

**Constraint 6:** Cross-platform compatibility: must build and run on Linux, macOS, and Windows. Path separator handling must use `filepath` (not `path`).

**Constraint 7:** `ToolCall.Arguments` is `map[string]string`. The `offset` and `limit` values must be parsed from strings within the executor using `strconv.Atoi`.

**Constraint 8:** No Windows CRLF normalization. Lines ending with `\r` are returned as-is.

**Constraint 9:** No file content caching. Each invocation reads the file from disk.

---

## 7. Testing Requirements

### 7.1 Test Conventions

Tests must follow the patterns established in M3:

- **Package-level tests:** Tests live in the same package (`package tool`)
- **Standard library only:** Use `testing` package. No test frameworks.
- **Temp directories:** Use `t.TempDir()` for filesystem isolation.
- **Descriptive names:** `TestFunctionName_Scenario` with underscores.
- **Test real code, not mocks.** Tests must call actual functions with real filesystem operations. Each test must fail if the code under test is deleted.
- **Red/green TDD:** Write failing tests first, then implement code to make them pass.
- **Run tests with:** `make test`

### 7.2 `internal/tool/read_file_test.go` Tests

**Test: `TestReadFile_FullFile`** — Create a tmpdir with a file containing known multi-line content (e.g., `"line1\nline2\nline3\n"`). Call the `Execute` function with `Arguments: {"path": "<filename>"}` (no offset/limit). Verify `IsError` is false. Verify `Content` equals `"1: line1\n2: line2\n3: line3"`. Verify `CallID` matches.

**Test: `TestReadFile_WithOffset`** — Create a file with 5+ lines. Call with `Arguments: {"path": "<filename>", "offset": "3"}`. Verify the output starts at line 3 with the correct line number prefix (e.g., `3: ...`).

**Test: `TestReadFile_WithLimit`** — Create a file with 10+ lines. Call with `Arguments: {"path": "<filename>", "limit": "3"}`. Verify exactly 3 lines are returned, starting from line 1.

**Test: `TestReadFile_WithOffsetAndLimit`** — Create a file with 10+ lines. Call with `Arguments: {"path": "<filename>", "offset": "4", "limit": "2"}`. Verify exactly 2 lines are returned: line 4 and line 5 with correct line-number prefixes.

**Test: `TestReadFile_OffsetPastEOF`** — Create a file with 5 lines. Call with `Arguments: {"path": "<filename>", "offset": "10"}`. Verify `IsError` is true. Verify `Content` contains `"offset 10 exceeds file length of 5 lines"`.

**Test: `TestReadFile_LimitExceedsRemaining`** — Create a file with 5 lines. Call with `Arguments: {"path": "<filename>", "offset": "4", "limit": "100"}`. Verify `IsError` is false. Verify exactly 2 lines are returned (lines 4 and 5).

**Test: `TestReadFile_EmptyFile`** — Create an empty file (0 bytes). Call with `Arguments: {"path": "<filename>"}`. Verify `IsError` is false. Verify `Content` is `""`.

**Test: `TestReadFile_EmptyFileWithOffsetTwo`** — Create an empty file. Call with `Arguments: {"path": "<filename>", "offset": "2"}`. Verify `IsError` is true. Verify `Content` contains `"offset 2 exceeds file length of 0 lines"`.

**Test: `TestReadFile_NonexistentFile`** — Call with `Arguments: {"path": "no_such_file.txt"}`. Verify `IsError` is true.

**Test: `TestReadFile_BinaryFileRejected`** — Create a file containing NUL bytes within the first 512 bytes. Call `Execute`. Verify `IsError` is true. Verify `Content` contains `"binary file detected"`.

**Test: `TestReadFile_DirectoryRejected`** — Create a subdirectory. Call with `Arguments: {"path": "<dirname>"}`. Verify `IsError` is true. Verify `Content` contains `"path is a directory, not a file"`.

**Test: `TestReadFile_AbsolutePathRejected`** — Call with `Arguments: {"path": "/etc/passwd"}`. Verify `IsError` is true. Verify `Content` mentions absolute paths.

**Test: `TestReadFile_ParentTraversalRejected`** — Call with `Arguments: {"path": "../../etc/passwd"}`. Verify `IsError` is true. Verify `Content` mentions path escaping.

**Test: `TestReadFile_SymlinkEscapeRejected`** — Create a symlink inside tmpdir pointing outside. Call with the symlink as path. Verify `IsError` is true.

**Test: `TestReadFile_MissingPathArgument`** — Call with empty `Arguments` map. Verify `IsError` is true. Verify `Content` mentions path required.

**Test: `TestReadFile_InvalidOffset`** — Call with `Arguments: {"path": "<filename>", "offset": "abc"}`. Verify `IsError` is true. Verify `Content` describes a parse error.

**Test: `TestReadFile_ZeroOffset`** — Call with `Arguments: {"path": "<filename>", "offset": "0"}`. Verify `IsError` is true. Verify `Content` contains `"offset must be >= 1"`.

**Test: `TestReadFile_InvalidLimit`** — Call with `Arguments: {"path": "<filename>", "limit": "xyz"}`. Verify `IsError` is true. Verify `Content` describes a parse error.

**Test: `TestReadFile_ZeroLimit`** — Call with `Arguments: {"path": "<filename>", "limit": "0"}`. Verify `IsError` is true. Verify `Content` contains `"limit must be >= 1"`.

**Test: `TestReadFile_TrailingNewline`** — Create a file with content `"a\nb\n"`. Call `Execute`. Verify output is `"1: a\n2: b"` (2 lines, trailing empty element removed).

**Test: `TestReadFile_NoTrailingNewline`** — Create a file with content `"a\nb"`. Call `Execute`. Verify output is `"1: a\n2: b"` (2 lines).

**Test: `TestReadFile_CallIDPassthrough`** — Call with a specific `call.ID`. Verify the returned `ToolResult.CallID` matches.

**Test: `TestReadFile_NestedPath`** — Create a file at `sub/deep/file.txt` inside tmpdir. Call with `Arguments: {"path": "sub/deep/file.txt"}`. Verify `IsError` is false and content is correct.

### 7.3 `RegisterAll` Tests (additions to existing)

**Test: `TestRegisterAll_RegistersReadFile`** — Call `NewRegistry()`, then `RegisterAll(r)`. Verify `r.Has(toolname.ReadFile)` returns true.

**Test: `TestRegisterAll_ResolvesReadFile`** — Call `RegisterAll(r)`, then `r.Resolve([]string{toolname.ReadFile})`. Verify the returned tool has `Name` equal to `toolname.ReadFile` and has `path`, `offset`, and `limit` parameters.

### 7.4 Existing Tests

All existing tests must continue to pass without modification:

- `internal/tool/tool_test.go`
- `internal/tool/registry_test.go`
- `internal/tool/list_directory_test.go`
- `internal/tool/path_validation_test.go`
- All other test files in the project.

### 7.5 Running Tests

All tests must pass when run with:

```bash
make test
```

---

## 8. Acceptance Criteria

| Criterion | Verification |
|-----------|-------------|
| `internal/tool/read_file.go` exists and compiles | `go build ./internal/tool/` succeeds |
| `RegisterAll` registers `read_file` | `TestRegisterAll_RegistersReadFile` passes |
| Tool definition has correct name | `toolname.ReadFile` constant used, not hardcoded string |
| Tool definition has 3 parameters | `path` (required), `offset` (optional), `limit` (optional) |
| Read full file with line numbers | `TestReadFile_FullFile` passes |
| Read with offset | `TestReadFile_WithOffset` passes |
| Read with limit | `TestReadFile_WithLimit` passes |
| Read with offset and limit | `TestReadFile_WithOffsetAndLimit` passes |
| Offset past EOF returns error | `TestReadFile_OffsetPastEOF` passes |
| Limit exceeding remaining lines returns partial | `TestReadFile_LimitExceedsRemaining` passes |
| Empty file returns empty content | `TestReadFile_EmptyFile` passes |
| Empty file with offset=2 returns error | `TestReadFile_EmptyFileWithOffsetTwo` passes |
| Nonexistent file returns error | `TestReadFile_NonexistentFile` passes |
| Binary file rejected | `TestReadFile_BinaryFileRejected` passes |
| Directory path rejected | `TestReadFile_DirectoryRejected` passes |
| Absolute path rejected | `TestReadFile_AbsolutePathRejected` passes |
| Parent traversal rejected | `TestReadFile_ParentTraversalRejected` passes |
| Symlink escape rejected | `TestReadFile_SymlinkEscapeRejected` passes |
| Missing path argument rejected | `TestReadFile_MissingPathArgument` passes |
| Invalid offset rejected | `TestReadFile_InvalidOffset` passes |
| Zero offset rejected | `TestReadFile_ZeroOffset` passes |
| Invalid limit rejected | `TestReadFile_InvalidLimit` passes |
| Zero limit rejected | `TestReadFile_ZeroLimit` passes |
| Trailing newline handled correctly | `TestReadFile_TrailingNewline` passes |
| No trailing newline handled correctly | `TestReadFile_NoTrailingNewline` passes |
| CallID propagated | `TestReadFile_CallIDPassthrough` passes |
| Nested path works | `TestReadFile_NestedPath` passes |
| All existing tests pass | `make test` exits 0 |

---

## 9. Out of Scope

The following items are explicitly **not** included in this milestone:

1. Other tool executors (`write_file`, `edit_file`, `run_command`) — M5–M7 scope
2. File metadata in output (size, permissions, modification time)
3. Windows CRLF normalization
4. File content caching
5. Syntax highlighting or language detection
6. Glob/wildcard patterns for reading multiple files
7. Line-number right-alignment or padding
8. Output truncation by byte count (only line-based limit)
9. File size hard limits
10. Changes to `--dry-run`, `--json`, or `--verbose` output — M8 scope
11. Changes to `internal/agent/`, `internal/provider/`, or `internal/toolname/` packages
12. Changes to `cmd/run.go` or `internal/tool/tool.go`

---

## 10. References

- Milestone Definition: `docs/plans/000_tool_call_milestones.md` (M4 section, lines 62–73)
- M3 Spec: `docs/plans/014_list_directory_spec.md`
- Existing `list_directory` implementation: `internal/tool/list_directory.go`
- `validatePath` function: `internal/tool/path_validation.go`
- `RegisterAll` function: `internal/tool/registry.go` (bottom of file)
- `toolname.ReadFile` constant: `internal/toolname/toolname.go:9`
- `provider.Tool` type: `internal/provider/provider.go:34`
- `provider.ToolCall` type: `internal/provider/provider.go:41`
- `provider.ToolResult` type: `internal/provider/provider.go:48`
- `ToolEntry` type: `internal/tool/registry.go:20`
- `ExecContext` type: `internal/tool/registry.go:13`

---

## 11. Notes

- **Binary detection uses the same heuristic as Git.** Scanning the first 512 bytes for NUL is the standard approach used by `git diff` and HTTP `Content-Type` sniffing. It is not perfect (some binary files have no NUL in the first 512 bytes), but it catches the vast majority of cases with zero false positives on text files.
- **`strings.Split` on `"\n"` produces a trailing empty element for files ending in `"\n"`.** This is handled explicitly by removing the last element when the file ends with a newline, ensuring consistent line counts. The implementation must check `len(content) > 0 && content[len(content)-1] == '\n'` (on the byte slice) before splitting.
- **Line numbers in output are absolute.** When using offset=50, the first line of output is prefixed with `50:`, not `1:`. This allows the LLM to reference exact line numbers in subsequent tool calls (e.g., `edit_file`).
- **The `RegisterAll` pattern continues to scale.** M4 adds one line. M5–M7 will each add one line. No call-site changes needed.
- **`os.ReadFile` reads the entire file into memory.** For very large files, this could use significant memory. The lack of a hard file size limit is a deliberate decision — the offset/limit mechanism provides adequate protection for typical use cases. If this becomes a problem in practice, a byte-size limit can be added in a future milestone.
