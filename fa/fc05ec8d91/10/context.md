# Session Context

## User Prompts

### Prompt 1

You are working in /Users/jaronswab/go/src/github.com/jrswab/axe on branch ISS-24/allow-list-connections.

## Task
Gate the `url_fetch` tool with the `hostcheck` package using red/green TDD. This is Task 4 + 4b from the implementation plan.

## Context
- `internal/hostcheck/hostcheck.go` already exists with `CheckHost(ctx, hostname, allowlist, resolver) (net.IP, error)`, `IsAllowed()`, `IsPrivateIP()`, and a `Resolver` interface.
- `internal/tool/registry.go` `ExecContext` already has `Allowe...

