# Session Context

## User Prompts

### Prompt 1

You are working in the Go project at /Users/jaronswab/go/src/github.com/jrswab/axe.

Your task: Add 3 new test functions to `internal/tool/url_fetch_test.go`. These tests are the RED phase of TDD — they should FAIL right now because the production code doesn't have the timeout yet.

The production code in `internal/tool/url_fetch.go` currently has NO per-request timeout. It uses `ctx` directly in `http.NewRequestWithContext`. We are about to add a `context.WithTimeout` wrapper with a package-...

