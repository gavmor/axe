# Session Context

## User Prompts

### Prompt 1

You are implementing Track A of a refactoring in the axe project at /Users/jaronswab/go/src/github.com/jrswab/axe.

## Context

The file `internal/tool/tool.go` has inline logic at lines 149-154 for computing effective allowed hosts during sub-agent delegation:

```go
// Compute effective allowed hosts: sub-agent's own list wins (even if empty),
// else inherit parent's when not explicitly set (nil).
effectiveAllowedHosts := cfg.AllowedHosts
if effectiveAllowedHosts == nil {
    effectiveAllo...

