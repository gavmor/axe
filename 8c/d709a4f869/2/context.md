# Session Context

## User Prompts

### Prompt 1

You are working in /Users/jaronswab/go/src/github.com/jrswab/axe on branch ISS-24/allow-list-connections.

## Task
Two quick fixes:

### Fix 1: Restore allowlist check in CheckRedirect

In `internal/tool/url_fetch.go`, the `CheckRedirect` callback currently only checks redirect count. Restore the allowlist-only check (no DNS) as a first line of defense. The `DialContext` handles the full DNS-level check.

Find the current `CheckRedirect` (around line 165-170):
```go
		CheckRedirect: func(redi...

