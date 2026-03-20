# Session Context

## User Prompts

### Prompt 1

You are working in /Users/jaronswab/go/src/github.com/jrswab/axe on branch ISS-24/allow-list-connections.

## Task
Change the sub-agent AllowedHosts inheritance logic from `len() == 0` to `== nil` so that an explicit `allowed_hosts = []` clears the parent's list. Use red/green TDD.

### Step 1: Write a test FIRST

In `internal/tool/tool_test.go` (or whatever test file exists for tool.go — check if it exists first; if not, create it), add a test that verifies:
- When `cfg.AllowedHosts` is `nil...

