# Session Context

## User Prompts

### Prompt 1

You are working in /Users/jaronswab/go/src/github.com/jrswab/axe on branch ISS-24/allow-list-connections.

## Task
Create the `internal/hostcheck/` package using red/green TDD. Write the tests FIRST, then write the implementation to make them pass.

## Step 1: Write tests first — `internal/hostcheck/hostcheck_test.go`

Create the file with these test cases:

### Tests for `IsAllowed(hostname string, allowlist []string) bool`:
- Table-driven tests:
  - Empty allowlist returns `true` for any ho...

