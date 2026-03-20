# Session Context

## User Prompts

### Prompt 1

You are working in /Users/jaronswab/go/src/github.com/jrswab/axe on branch ISS-24/allow-list-connections.

## Task
Add `AllowedHosts []string` to the `ExecContext` struct in `internal/tool/registry.go`.

## Step 1: Read the current `ExecContext` struct

Read `internal/tool/registry.go` and find the `ExecContext` struct (around line 13). It currently has:
```go
type ExecContext struct {
    Workdir string
    Stderr  io.Writer
    Verbose bool
}
```

## Step 2: Add the field

Add `AllowedHosts...

