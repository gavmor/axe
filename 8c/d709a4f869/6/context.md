# Session Context

## User Prompts

### Prompt 1

You are working in /Users/jaronswab/go/src/github.com/jrswab/axe on branch ISS-24/allow-list-connections.

## Task
Two related fixes in `internal/tool/url_fetch.go`:

### Fix 5: Move timeout context before host check

Currently the host check on line 141 uses the unbounded parent `ctx`. DNS resolution inside `CheckHost` could hang indefinitely. Move the timeout context creation to before the host check.

**Current order (lines 140-147):**
```go
	// Host allowlist and private IP check.
	safeIP...

