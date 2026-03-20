# Session Context

## User Prompts

### Prompt 1

You are working in /Users/jaronswab/go/src/github.com/jrswab/axe on branch ISS-24/allow-list-connections.

## Task
Fix two security/correctness issues in `internal/tool/url_fetch.go`:

1. **DNS rebinding vulnerability**: The resolved IP from `CheckHost` is discarded (line 140 uses `_`). The HTTP client then does its own DNS resolution, creating a TOCTOU gap. Fix by capturing the resolved IP and pinning the connection to it via a custom `DialContext`.

2. **Missing Timeout on http.Client**: Ad...

