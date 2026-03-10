# Session Context

## User Prompts

### Prompt 1

You are working in the Go project at /Users/jaronswab/go/src/github.com/jrswab/axe.

Your task: Make exactly 2 changes to `internal/tool/url_fetch.go` to add a 15-second per-request HTTP timeout. This is the GREEN phase of TDD — the tests already exist and are waiting for this code.

### Change 1: Add the timeout variable

After the existing line:
```go
const maxReadBytes = 10000
```
(line 14)

Add this line:
```go
var urlFetchTimeout = 15 * time.Second
```

Also add `"time"` to the import bl...

