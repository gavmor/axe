# Session Context

## User Prompts

### Prompt 1

You are working in /Users/jaronswab/go/src/github.com/jrswab/axe on branch ISS-24/allow-list-connections.

## Task
The `TestExecuteCallAgent_AllowedHostsInheritance` test in `internal/tool/tool_test.go` doesn't actually verify the nil-vs-empty distinction. It only checks that the sub-agent ran successfully, which would pass even with the old `len() == 0` logic.

## Problem
The test needs to verify that:
1. When `cfg.AllowedHosts` is nil → the parent's `AllowedHosts` is inherited into the sub-...

