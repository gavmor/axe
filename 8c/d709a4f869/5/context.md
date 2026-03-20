# Session Context

## User Prompts

### Prompt 1

You are working in /Users/jaronswab/go/src/github.com/jrswab/axe on branch ISS-24/allow-list-connections.

## Task
Two small changes:

### Fix 8: Add `allowed_hosts` to Scaffold template

In `internal/agent/agent.go`, find the `Scaffold()` function (around line 298-358). Add a commented-out `allowed_hosts` entry after the `tools` line. Find:

```go
# Tools this agent can use (optional)
# Valid: list_directory, read_file, write_file, edit_file, run_command, url_fetch, web_search
# tools = []
`...

