# Session Context

## User Prompts

### Prompt 1

You are working in /Users/jaronswab/go/src/github.com/jrswab/axe on branch ISS-24/allow-list-connections.

## Task
Implement sub-agent AllowedHosts propagation in `internal/tool/tool.go` — Tasks 6a, 6b, 6c from the implementation plan.

## What to change

### Change 1: Add `AllowedHosts` to `ExecuteOptions` struct (line 28)

Add `AllowedHosts []string` field to the struct:
```go
type ExecuteOptions struct {
    AllowedAgents []string
    ParentModel   string
    Depth         int
    MaxDepth...

