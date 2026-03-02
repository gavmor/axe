# Implementation: Tool Call M4 — `read_file` Tool

**Spec:** `docs/plans/015_read_file_spec.md`
**Status:** Not started

---

## Phase 1 — Red: Write Failing Tests

Write all tests first. Every test must fail (compile error or test failure) before any implementation code exists.

- [x] Create `internal/tool/read_file_test.go` with `package tool` declaration and imports (`context`, `os`, `path/filepath`, `strings`, `testing`, `github.com/jrswab/axe/internal/provider`)
- [x] Write `TestReadFile_FullFile` — file with `"line1\nline2\nline3\n"`, no offset/limit, expect `"1: line1\n2: line2\n3: line3"` (Req 2.10–2.15)
- [x] Write `TestReadFile_WithOffset` — 5+ line file, offset=3, expect output starting at `"3: ..."` (Req 2.8, 2.13, 2.14)
- [x] Write `TestReadFile_WithLimit` — 10+ line file, limit=3, expect exactly 3 lines from line 1 (Req 2.9, 2.13)
- [x] Write `TestReadFile_WithOffsetAndLimit` — 10+ line file, offset=4 limit=2, expect lines 4–5 (Req 2.8, 2.9, 2.13)
- [x] Write `TestReadFile_OffsetPastEOF` — 5-line file, offset=10, expect error `"offset 10 exceeds file length of 5 lines"` (Req 2.12)
- [x] Write `TestReadFile_LimitExceedsRemaining` — 5-line file, offset=4 limit=100, expect 2 lines returned (lines 4–5) (Req 2.13)
- [x] Write `TestReadFile_EmptyFile` — 0-byte file, expect empty content, no error (Req 2.16)
- [x] Write `TestReadFile_EmptyFileWithOffsetTwo` — 0-byte file, offset=2, expect error `"offset 2 exceeds file length of 0 lines"` (Req 2.12, 2.16)
- [x] Write `TestReadFile_NonexistentFile` — path `"no_such_file.txt"`, expect error (Req 2.3)
- [x] Write `TestReadFile_BinaryFileRejected` — file with NUL bytes in first 512 bytes, expect error `"binary file detected"` (Req 2.7)
- [x] Write `TestReadFile_DirectoryRejected` — path to subdirectory, expect error `"path is a directory, not a file"` (Req 2.4)
- [x] Write `TestReadFile_AbsolutePathRejected` — path `"/etc/passwd"`, expect error mentioning absolute paths (Req 2.3)
- [x] Write `TestReadFile_ParentTraversalRejected` — path `"../../etc/passwd"`, expect error mentioning path escaping (Req 2.3)
- [x] Write `TestReadFile_SymlinkEscapeRejected` — symlink inside tmpdir pointing outside, expect error (Req 2.3)
- [x] Write `TestReadFile_MissingPathArgument` — empty Arguments map, expect error mentioning path required (Req 2.2, 2.3)
- [x] Write `TestReadFile_InvalidOffset` — offset `"abc"`, expect error with parse failure (Req 2.8)
- [x] Write `TestReadFile_ZeroOffset` — offset `"0"`, expect error `"offset must be >= 1"` (Req 2.8)
- [x] Write `TestReadFile_InvalidLimit` — limit `"xyz"`, expect error with parse failure (Req 2.9)
- [x] Write `TestReadFile_ZeroLimit` — limit `"0"`, expect error `"limit must be >= 1"` (Req 2.9)
- [x] Write `TestReadFile_TrailingNewline` — content `"a\nb\n"`, expect `"1: a\n2: b"` (Req 2.11)
- [x] Write `TestReadFile_NoTrailingNewline` — content `"a\nb"`, expect `"1: a\n2: b"` (Req 2.11)
- [x] Write `TestReadFile_CallIDPassthrough` — specific call.ID, verify ToolResult.CallID matches (Req 2.1)
- [x] Write `TestReadFile_NestedPath` — file at `sub/deep/file.txt`, expect success (Req 2.3)
- [x] Add `TestRegisterAll_RegistersReadFile` to `registry_test.go` — verify `r.Has(toolname.ReadFile)` after `RegisterAll` (Req 3.1)
- [x] Add `TestRegisterAll_ResolvesReadFile` to `registry_test.go` — verify resolved tool has name `toolname.ReadFile` and `path`, `offset`, `limit` parameters (Req 3.1)
- [x] Verify all new tests fail: `make test` must show compile errors or test failures for the new tests (TDD red phase)

---

## Phase 2 — Green: Implement `read_file`

Write the minimum code to make all tests pass.

- [x] Create `internal/tool/read_file.go` with `package tool` declaration and imports (`context`, `bytes`, `fmt`, `os`, `strconv`, `strings`, `github.com/jrswab/axe/internal/provider`, `github.com/jrswab/axe/internal/toolname`)
- [x] Implement `readFileEntry() ToolEntry` — returns `ToolEntry{Definition: readFileDefinition, Execute: readFileExecute}` (Req 1.2)
- [x] Implement `readFileDefinition() provider.Tool` — name from `toolname.ReadFile`, description, three parameters: `path` (required), `offset` (optional), `limit` (optional) (Req 1.3, 1.4)
- [x] Implement `readFileExecute` — path extraction and `validatePath` call with error handling (Req 2.2, 2.3)
- [x] Implement `readFileExecute` — `os.Stat` check: directory rejection with `"path is a directory, not a file"`, stat error propagation (Req 2.4, 2.5)
- [x] Implement `readFileExecute` — `os.ReadFile` with error handling (Req 2.6)
- [x] Implement `readFileExecute` — binary detection: scan first 512 bytes for NUL, error `"binary file detected"` (Req 2.7)
- [x] Implement `readFileExecute` — offset parsing: default 1, `strconv.Atoi`, `< 1` check (Req 2.8)
- [x] Implement `readFileExecute` — limit parsing: default 2000, `strconv.Atoi`, `< 1` check (Req 2.9)
- [x] Implement `readFileExecute` — empty file early return: 0 bytes returns `Content: ""`, `IsError: false` (Req 2.16)
- [x] Implement `readFileExecute` — line splitting with `strings.Split` and trailing newline removal (Req 2.10, 2.11)
- [x] Implement `readFileExecute` — offset bounds check: error `"offset %d exceeds file length of %d lines"` (Req 2.12)
- [x] Implement `readFileExecute` — line selection: `lines[offset-1 : min(offset-1+limit, len(lines))]` (Req 2.13)
- [x] Implement `readFileExecute` — output formatting: `fmt.Sprintf("%d: %s", lineNumber, lineContent)` joined with `"\n"` (Req 2.14, 2.15)
- [x] Register in `RegisterAll`: add `r.Register(toolname.ReadFile, readFileEntry())` to `internal/tool/registry.go` (Req 3.1)

---

## Phase 3 — Verify

- [x] Run `make test` — all tests pass (new and existing)
- [x] Run `go build ./internal/tool/` — compiles without errors
- [x] Verify no changes to `go.mod` or `go.sum`
- [x] Verify no changes to `cmd/run.go` or `internal/tool/tool.go`
- [x] Verify no changes to `internal/provider/`, `internal/agent/`, or `internal/toolname/`
- [x] Verify `registry.go` diff is exactly one added line in `RegisterAll`
